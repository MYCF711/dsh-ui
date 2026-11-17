package dsrightbar

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 完整账本只消费 Snapshot.Frames。官方依据：
// TrajectoryView.tsx:141-163（Turn/Assistant/SubCalls 三套折叠与 50 条分页）、
// TrajectoryToolbar.tsx:58-143（时长/实际时间/全局折叠/搜索）、
// TrajectoryTimeline.tsx:240-386,428-575（三轨、缩放、拖选、Esc）、
// TrajectoryTable.tsx:178-244,993-1027,3002-3600（分栏与多页 Inspector）。

const (
	ledgerHistoryPage = 50
	ledgerOverscan    = 100.0
	// TrajectoryTable.module.css:129-161：表头/内容行均 30px；:753-757：折叠摘要 20px。
	ledgerTurnHeight    = 30.0
	ledgerCellHeight    = 30.0
	ledgerSummaryHeight = 20.0
	// TrajectoryGroupHeader.module.css:3-10：Step 分组头 36px、左右 20px。
	ledgerGroupHeight = 36.0
	ledgerGroupPadX   = 20.0
	// TrajectoryTable.module.css:153-161：行间 border-spacing 为 0，不叠自造 2px gap。
	ledgerRowGap = 0.0
)

type ledgerRange struct {
	Start float64
	End   float64
}

type ledgerSpan struct {
	Record *ledgerRecord
	Start  float64
	End    float64
	Lane   int
}

type ledgerTimeline struct {
	Spans []*ledgerSpan
	Start float64
	End   float64
	Mode  string
}

type ledgerDrag struct {
	Active       bool
	Pan          bool
	ID           gfx.ID
	RecordID     string
	AnchorX      float64
	AnchorTime   float64
	AnchorStart  float64
	DomainStart  float64
	DomainLength float64
	Moved        bool
}

type ledgerViewState struct {
	collapsedTurns     map[int]bool
	collapsedAssistant map[string]bool
	collapsedTools     map[string]bool
	actualTime         bool
	selectedRecord     string
	selectedRequest    string
	tabs               map[string]string
	recentTabs         []string
	timelineFocus      map[string]bool
	lastClick          map[string]time.Time
	historyLimit       int
	selection          *ledgerRange
	draft              *ledgerRange
	viewport           *ledgerRange
	drag               ledgerDrag
	timelineRect       gfx.Rect
	timelineFull       ledgerRange
	timelineMode       string
	detailsWidth       float32
	resizing           bool
	resizeX            float32
	resizeWidth        float32
}

func (p *Plugin) ledgerState(session string) *ledgerViewState {
	if p.ledgerStates == nil {
		p.ledgerStates = map[string]*ledgerViewState{}
	}
	s := p.ledgerStates[session]
	if s == nil {
		s = &ledgerViewState{
			collapsedTurns: map[int]bool{}, collapsedAssistant: map[string]bool{},
			collapsedTools: map[string]bool{}, tabs: map[string]string{}, lastClick: map[string]time.Time{},
			historyLimit: ledgerHistoryPage,
		}
		p.ledgerStates[session] = s
	}
	p.ledgerSession = session
	return s
}

func (p *Plugin) loadLedgerDuration(env *pluginapi.Env) {
	if p.ledgerDurationLoaded {
		return
	}
	p.ledgerDurationLoaded = true
	if env.Act.ConfigGet == nil {
		return
	}
	p.ledgerDuration = env.Act.ConfigGet()["dsh.trajectory.duration"] == "true"
}

func (p *Plugin) setLedgerDuration(env *pluginapi.Env, value bool) {
	p.ledgerDuration = value
	if env.Act.ConfigSet != nil {
		env.Act.ConfigSet(map[string]string{"dsh.trajectory.duration": strconv.FormatBool(value)})
	}
}

func (p *Plugin) drawLedger(f ui.C, env *pluginapi.Env, data pluginapi.Snapshot, scope string, chrome bool) {
	p.initWPD1()
	p.loadLedgerDuration(env)
	s := p.ledgerState(data.Cur)
	model := foldLedger(data.Frames)
	window := windowLedger(model, s.historyLimit)
	query := strings.TrimSpace(p.trajectoryField(scope).Text())
	mode := ledgerTimelineMode(p.ledgerDuration, s.actualTime)
	timeline := deriveLedgerTimeline(window, mode)
	normalizeLedgerViewport(s, timeline)
	p.drawLedgerHeader(f, env, data, model, chrome)
	p.drawLedgerToolbar(f, env, s, window, scope, query)
	p.drawLedgerTimeline(f, env, s, timeline, query)
	matches := ledgerMatchSet(window, query)
	activeRange := s.selection
	if s.draft != nil {
		activeRange = s.draft
	}
	focus := ledgerFocusSet(timeline, activeRange)
	s.timelineFocus = focus
	rows := buildLedgerRows(window, matches, focus, query != "")
	if query == "" { // 官方搜索直接过滤 allRecords，绕过但保留三套折叠状态。
		rows = collapseLedgerRows(rows, s)
	}
	p.drawLedgerSplit(f, env, data, s, model, window, rows, scope, query)
}

func (p *Plugin) drawLedgerHeader(f ui.C, env *pluginapi.Env, data pluginapi.Snapshot, model *ledgerModel, chrome bool) {
	t := p.theme
	ui.Row(f).WidthPercent(100).MinHeight(28).Items(ui.Center).Gap(6).Children(func() {
		ui.Text(f, "轨迹").FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).Color(t.LabelTertiary)
		ui.Spacer(f)
		if count := ledgerContentRecordCount(model); count > 0 {
			ui.Text(f, itoa(count)+" 条").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
		if chrome {
			p.drawFullscreenButton(f, env)
			if env.Act.ToggleRightbar != nil {
				if p.chromeButton(f, env, "collapse-rightbar", "收起右侧边栏", rightbarCollapseIcon).Clicked() {
					env.Act.ToggleRightbar()
				}
			}
		}
	})
	p.prims.Divider(f)
}

func ledgerTimelineMode(actualDuration, actualTime bool) string {
	if actualDuration {
		if actualTime {
			return "actual"
		}
		return "duration"
	}
	if actualTime {
		return "time"
	}
	return "sequence"
}

func (p *Plugin) drawLedgerToolbar(f ui.C, env *pluginapi.Env, s *ledgerViewState, model *ledgerModel, scope, query string) {
	t := p.theme
	field := p.trajectoryField(scope)
	ui.Row(f).WidthPercent(100).MinHeight(32).Gap(8).PadSides(4, 6, 0, 6).Items(ui.Center).Children(func() {
		drawLedgerAction(f, env, "ledger-duration", "时长", func() {
			p.setLedgerDuration(env, !p.ledgerDuration)
			s.selection, s.viewport = nil, nil
		}, true)
		drawLedgerAction(f, env, "ledger-actual-time", "实际时间", func() {
			s.actualTime = !s.actualTime
			s.selection, s.viewport = nil, nil
		}, s.actualTime)
		turnIDs, assistantIDs, toolIDs := ledgerCollapsibleIDs(model)
		drawLedgerAction(f, env, "ledger-turns", "轮次", func() {
			toggleLedgerTurns(s, turnIDs)
		}, allLedgerTurnsCollapsed(s, turnIDs))
		drawLedgerAction(f, env, "ledger-calls", "调用", func() {
			toggleLedgerAssistants(s, assistantIDs)
		}, allLedgerAssistantsCollapsed(s, assistantIDs))
		drawLedgerAction(f, env, "ledger-subcalls", "子调用", func() {
			toggleLedgerTools(s, toolIDs)
		}, allLedgerToolsCollapsed(s, toolIDs))
	})
	ui.Row(f).WidthPercent(100).MinHeight(32).Gap(4).PadSides(0, 6, 4, 6).Items(ui.Center).Children(func() {
		ui.TextFieldID(f, env.Tree.ID(env.Feat, "trajectory-search-"+scope+env.Data.Cur), field, "搜索轨迹", "搜索").
			Grow(1).Height(22).FontSize(12)
		if query != "" {
			ui.Text(f, itoa(len(ledgerMatchSet(model, query)))+" 命中").FontSize(12).Color(t.LabelCaption)
		}
		if field.Text() != "" {
			drawLedgerAction(f, env, "ledger-clear-search", "清除轨迹搜索", func() { field.SetText("") }, false)
		}
		if s.selection != nil {
			drawLedgerAction(f, env, "ledger-clear-range", "清除时间范围", func() { s.selection = nil }, false)
		}
	})
}

// drawLedgerAction 是官方 toolbar 的小按钮；active 只改底色，不把开关状态伪装成动作结果。
func drawLedgerAction(f ui.C, env *pluginapi.Env, name, label string, fn func(), active bool) {
	t := env.Theme
	// TrajectoryToolbar.module.css:33-46：20px 高、左右 7px、12px 字。
	box := ui.Row(f).MinHeight(20).Radius(t.Radii.SM).PadX(7).Items(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, name), label)
	if active {
		box = box.Background(t.BgLayer2)
	}
	box.Children(func() { ui.Text(f, label).FontSize(12).Color(t.LabelTertiary) })
	if box.Clicked() {
		fn()
	}
}

func drawLedgerDetailTab(f ui.C, env *pluginapi.Env, name, label string, fn func(), active bool) {
	t := env.Theme
	box := ui.Row(f).MinHeight(34).PadX(9).Items(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, name), label)
	box.Children(func() {
		text := ui.Text(f, label).FontSize(13).Color(t.LabelTertiary)
		if active {
			text.Color(t.Link)
		}
	})
	if box.Clicked() {
		fn()
	}
}

func windowLedger(model *ledgerModel, limit int) *ledgerModel {
	if model == nil || limit <= 0 || len(model.Records) <= limit {
		return model
	}
	start := len(model.Records) - limit
	allowed := map[*ledgerRecord]bool{}
	for _, r := range model.Records[start:] {
		allowed[r] = true
	}
	out := &ledgerModel{Requests: model.Requests}
	for _, turn := range model.Turns {
		var dst *ledgerTurn
		for _, r := range turn.Records {
			if !allowed[r] {
				continue
			}
			if dst == nil {
				dst = &ledgerTurn{Turn: turn.Turn, Title: turn.Title}
				out.Turns = append(out.Turns, dst)
			}
			dst.Records = append(dst.Records, r)
			out.Records = append(out.Records, r)
		}
	}
	return out
}

func deriveLedgerTimeline(model *ledgerModel, mode string) *ledgerTimeline {
	tl := &ledgerTimeline{Mode: mode}
	if model == nil {
		return tl
	}
	if mode == "sequence" {
		position := 0.0
		for _, r := range model.Records {
			if r.RequestOnly {
				continue
			}
			tl.Spans = append(tl.Spans, &ledgerSpan{Record: r, Start: position, End: position + 1, Lane: ledgerLane(r.Kind)})
			position++
		}
		tl.End = position
		if tl.End == 0 {
			tl.End = 1
		}
		return tl
	}
	type rawSpan struct {
		span *ledgerSpan
		off  float64
	}
	var raw []*rawSpan
	for _, r := range model.Records {
		if r.RequestOnly || r.StartedAt == nil {
			continue
		}
		start := float64(*r.StartedAt)
		end := start
		if r.DurationMS != nil && *r.DurationMS > 0 {
			end += float64(*r.DurationMS)
		}
		raw = append(raw, &rawSpan{span: &ledgerSpan{Record: r, Start: start, End: end, Lane: ledgerLane(r.Kind)}})
	}
	compress := mode == "duration"
	removed := 0.0
	covered := 0.0
	hasCovered := false
	for _, entry := range raw {
		s := entry.span
		if compress && hasCovered && s.Start > covered {
			removed += s.Start - covered
		}
		entry.off = removed
		if !hasCovered || s.End > covered {
			covered = s.End
		}
		hasCovered = true
	}
	first := true
	for _, entry := range raw {
		s := entry.span
		s.Start -= entry.off
		if mode == "time" {
			s.End = s.Start
		} else {
			s.End -= entry.off
		}
		if first || s.Start < tl.Start {
			tl.Start = s.Start
		}
		if first || s.End > tl.End {
			tl.End = s.End
		}
		first = false
		tl.Spans = append(tl.Spans, s)
	}
	if tl.End <= tl.Start {
		tl.End = tl.Start + 1
	}
	return tl
}

func ledgerLane(kind ledgerKind) int {
	if kind == ledgerTool || kind == ledgerSubtool {
		return 2
	}
	if kind == ledgerMessage || kind == ledgerCompacted {
		return 1
	}
	return 0
}

func normalizeLedgerViewport(s *ledgerViewState, tl *ledgerTimeline) {
	s.timelineFull = ledgerRange{Start: tl.Start, End: tl.End}
	s.timelineMode = tl.Mode
	if s.viewport != nil && (s.viewport.End < tl.Start || s.viewport.Start > tl.End) {
		s.viewport = nil
	}
	if s.selection != nil && (s.selection.End < tl.Start || s.selection.Start > tl.End) {
		s.selection = nil
	}
}

func (p *Plugin) drawLedgerTimeline(f ui.C, env *pluginapi.Env, s *ledgerViewState, tl *ledgerTimeline, query string) {
	t := p.theme
	width := f.Bounds().Width
	if width <= 0 {
		width = 400
	}
	// TrajectoryTimeline.module.css:14-20：44px 标签轨 + 50px 高 plot。
	trackWidth := width - 44
	if trackWidth < 1 {
		trackWidth = 1
	}
	s.timelineRect = gfx.Rect{X: f.Bounds().X + 44, Y: f.Bounds().Y, Width: trackWidth, Height: 50}
	domain := s.timelineFull
	if s.viewport != nil {
		domain = *s.viewport
		for _, span := range tl.Spans {
			if span.Record.ID != s.selectedRecord || (span.Start >= domain.Start && span.End <= domain.End) {
				continue
			}
			length := domain.End - domain.Start
			if span.Start < domain.Start {
				domain.Start, domain.End = span.Start, span.Start+length
			} else if span.End > domain.End {
				domain.Start, domain.End = span.End-length, span.End
			}
			s.viewport = &domain
		}
	}
	if domain.End <= domain.Start {
		domain.End = domain.Start + 1
	}
	matches := map[string]bool{}
	if query != "" {
		matches = ledgerMatchSet(&ledgerModel{Records: spansLedgerRecords(tl.Spans)}, query)
	}
	ui.Row(f).WidthPercent(100).Height(50).Clip().Children(func() {
		ui.Column(f).Width(44).Height(50).Background(t.BgLayer2).Children(func() {
			// :31-51：右 3px，三条标签分别 top 7/21/35，10px/1。
			for i, label := range []string{"输入", "助手", "工具"} {
				ui.Row(f).Absolute().Right(3).Top(float32(7 + i*14)).Height(8).
					Items(ui.Center).Justify(ui.End).Children(func() {
					ui.Text(f, label).FontSize(10).LineHeight(8).Color(t.LabelCaption)
				})
			}
		})
		trackID := env.Tree.ID(env.Feat, "ledger-timeline-track")
		ui.Row(f).Grow(1).Height(50).Background(t.BgLayer2).Clip().Items(ui.Center).Justify(ui.Center).
			Hit(trackID, "时间线概览；水平拖动可聚焦事件").Children(func() {
			if len(tl.Spans) == 0 {
				ui.Text(f, "无计时数据").FontSize(13).Color(t.LabelCaption)
			}
			p.handleLedgerTimelinePointer(env, s, tl, domain, trackID, "")
			for _, span := range tl.Spans {
				if span.End < domain.Start || span.Start > domain.End {
					continue
				}
				left := float32((span.Start - domain.Start) / (domain.End - domain.Start) * float64(trackWidth))
				spanWidth := float32((span.End - span.Start) / (domain.End - domain.Start) * float64(trackWidth))
				minimumWidth := float32(2)
				if tl.Mode == "time" {
					minimumWidth = 8
				}
				if spanWidth < minimumWidth {
					spanWidth = minimumWidth
				}
				if left < 0 {
					spanWidth += left
					left = 0
				}
				if spanWidth < 1 || left >= trackWidth {
					continue
				}
				if left+spanWidth > trackWidth {
					spanWidth = trackWidth - left
				}
				color := ledgerKindColor(t, span.Record.Kind)
				if query != "" && !matches[span.Record.ID] {
					color = t.LabelDimmed
				}
				selected := s.selection != nil && span.Start <= s.selection.End && span.End >= s.selection.Start
				id := env.Tree.ID(env.Feat, "ledger-timeline-"+span.Record.ID)
				// :120-127,155-169：lanes top 7、lane 间距 14、span 高 8、radius 1。
				box := ui.Row(f).Absolute().Left(left).Top(float32(7+span.Lane*14)).Width(spanWidth).Height(8).
					Background(color).Radius(1).Clip().Hit(id, ledgerTimelineLabel(span.Record))
				if selected || s.selectedRecord == span.Record.ID {
					box = box.Background(t.Link)
				}
				if portion, ok := ledgerTTFTPortion(span.Record); ok {
					firstWidth := spanWidth * portion
					box.Children(func() {
						ui.Row(f).Absolute().Left(0).Top(0).Width(firstWidth).Height(8).Background(t.WarnLabel)
						ui.Row(f).Absolute().Left(firstWidth).Top(0).Width(spanWidth - firstWidth).Height(8).Background(t.Info)
					})
				}
				p.handleLedgerTimelinePointer(env, s, tl, domain, id, span.Record.ID)
			}
		})
	})
}

func spansLedgerRecords(spans []*ledgerSpan) []*ledgerRecord {
	out := make([]*ledgerRecord, 0, len(spans))
	for _, s := range spans {
		out = append(out, s.Record)
	}
	return out
}

func ledgerKindColor(t *pluginapi.Theme, kind ledgerKind) ui.Color {
	// TrajectoryTable.module.css:180-239 与 TrajectoryTimeline.module.css:181-232。
	switch kind {
	case ledgerMessage, ledgerCompacted:
		return t.Info
	case ledgerTool, ledgerSubtool:
		return t.WarnLabel
	case ledgerUser:
		return t.Link
	case ledgerContext:
		return t.SuccessPrimary
	default:
		return t.LabelCaption
	}
}

func ledgerTTFTPortion(r *ledgerRecord) (float32, bool) {
	if r.Kind != ledgerMessage || r.StartedAt == nil || r.FirstTokenAt == nil || r.CompletedAt == nil {
		return 0, false
	}
	start, first, completed := *r.StartedAt, *r.FirstTokenAt, *r.CompletedAt
	if first < start || completed < first || completed == first {
		return 0, false
	}
	ttft, decode := first-start, completed-first
	return float32(float64(ttft) / float64(ttft+decode)), true
}

func ledgerTimelineLabel(r *ledgerRecord) string {
	label := ledgerRecordLabel(r)
	if r.StartedAt != nil {
		label += " · 开始 " + formatLedgerTimestamp(*r.StartedAt)
	}
	if r.DurationMS != nil {
		label += " · 总时长 " + formatLedgerMS(*r.DurationMS)
	}
	if r.StartedAt != nil && r.FirstTokenAt != nil && r.CompletedAt != nil {
		label += " · 首 token " + formatLedgerMS(maxLedger(0, *r.FirstTokenAt-*r.StartedAt)) +
			" · 解码 " + formatLedgerMS(maxLedger(0, *r.CompletedAt-*r.FirstTokenAt))
	}
	return label
}

func (p *Plugin) handleLedgerTimelinePointer(env *pluginapi.Env, s *ledgerViewState, tl *ledgerTimeline, domain ledgerRange, id gfx.ID, recordID string) {
	c := env.Canvas
	if c == nil {
		return
	}
	ev, ok := c.PointerOn(id)
	if !ok {
		return
	}
	x := float64(ev.X)
	fraction := (x - float64(s.timelineRect.X)) / math.Max(1, float64(s.timelineRect.Width))
	fraction = math.Max(0, math.Min(1, fraction))
	at := domain.Start + fraction*(domain.End-domain.Start)
	switch ev.Kind {
	case gfx.PointerDown:
		if ev.Button != gfx.PointerButtonPrimary && ev.Button != gfx.PointerButtonSecondary && ev.Button != gfx.PointerButtonNone {
			return
		}
		pan := ev.Button == gfx.PointerButtonSecondary && s.viewport != nil
		s.drag = ledgerDrag{Active: true, Pan: pan, ID: id, RecordID: recordID, AnchorX: x,
			AnchorTime: at, AnchorStart: domain.Start, DomainStart: domain.Start, DomainLength: domain.End - domain.Start}
		s.draft = nil
		c.Consume()
	case gfx.PointerMove:
		if !s.drag.Active || s.drag.ID != id {
			return
		}
		if math.Abs(x-s.drag.AnchorX) >= 3 {
			s.drag.Moved = true
		}
		if s.drag.Pan {
			delta := (x - s.drag.AnchorX) / math.Max(1, float64(s.timelineRect.Width)) * s.drag.DomainLength
			start := clampLedger(s.drag.AnchorStart-delta, tl.Start, math.Max(tl.Start, tl.End-s.drag.DomainLength))
			s.viewport = &ledgerRange{Start: start, End: start + s.drag.DomainLength}
		} else {
			s.draft = &ledgerRange{Start: math.Min(s.drag.AnchorTime, at), End: math.Max(s.drag.AnchorTime, at)}
		}
		c.Consume()
	case gfx.PointerUp:
		if !s.drag.Active || s.drag.ID != id {
			return
		}
		drag := s.drag
		s.drag = ledgerDrag{}
		s.draft = nil
		if drag.Pan {
			if !drag.Moved {
				s.selection = nil
			}
			c.Consume()
			return
		}
		if !drag.Moved && drag.RecordID != "" {
			s.selectedRecord, s.selectedRequest, s.selection = drag.RecordID, "", nil
			c.Consume()
			return
		}
		r := ledgerRange{Start: math.Min(drag.AnchorTime, at), End: math.Max(drag.AnchorTime, at)}
		minimum := math.Min(domain.End-domain.Start, (tl.End-tl.Start)/math.Max(1, float64(len(tl.Spans))))
		if r.End-r.Start < minimum {
			center := (r.Start + r.End) / 2
			r.Start = clampLedger(center-minimum/2, tl.Start, math.Max(tl.Start, tl.End-minimum))
			r.End = r.Start + minimum
		}
		s.selection = &r
		c.Consume()
	}
}

func clampLedger(v, low, high float64) float64 {
	return math.Max(low, math.Min(high, v))
}

func ledgerMatchSet(model *ledgerModel, query string) map[string]bool {
	out := map[string]bool{}
	q := strings.ToLower(strings.TrimSpace(query))
	if model == nil {
		return out
	}
	terms := strings.Fields(q)
	for _, r := range model.Records {
		text := strings.ToLower(ledgerSearchText(r))
		matched := true
		for _, term := range terms { // 官方 search-index.ts:119-132：空白分词后逐项 AND。
			if !strings.Contains(text, term) {
				matched = false
				break
			}
		}
		if matched {
			out[r.ID] = true
		}
	}
	return out
}

func ledgerFocusSet(tl *ledgerTimeline, selection *ledgerRange) map[string]bool {
	out := map[string]bool{}
	if selection == nil || tl == nil {
		return out
	}
	for _, span := range tl.Spans {
		if span.Start <= selection.End && span.End >= selection.Start {
			out[span.Record.ID] = true
		}
	}
	return out
}

type ledgerRowKind uint8

const (
	ledgerRowTurn ledgerRowKind = iota
	ledgerRowGroup
	ledgerRowRecord
	ledgerRowSummary
	ledgerRowHistory
	ledgerRowRequest
)

type ledgerUIRow struct {
	Kind        ledgerRowKind
	Turn        *ledgerTurn
	Group       string
	Record      *ledgerRecord
	Request     *ledgerRequest
	Text        string
	Height      float32
	Dimmed      bool
	Summary     string
	SummaryID   string
	SummaryTurn int
}

func buildLedgerRows(model *ledgerModel, matches, focus map[string]bool, filtered bool) []ledgerUIRow {
	if model == nil {
		return nil
	}
	var rows []ledgerUIRow
	for _, turn := range model.Turns {
		turnRows := make([]ledgerUIRow, 0, len(turn.Records)+2)
		lastGroup := ""
		groupRow := -1
		for _, r := range turn.Records {
			if filtered && !matches[r.ID] {
				continue
			}
			if r.RequestOnly {
				// 官方 requestOnly 是 9px 分隔锚点，不是可选择的 assistant 内容行。
				turnRows = append(turnRows, ledgerUIRow{Kind: ledgerRowRequest, Record: r, Request: r.Request, Height: 9})
				lastGroup, groupRow = "", -1
				continue
			}
			if r.Group != lastGroup {
				turnRows = append(turnRows, ledgerUIRow{Kind: ledgerRowGroup, Group: r.Group, Record: r, Height: ledgerGroupHeight})
				groupRow = len(turnRows) - 1
				lastGroup = r.Group
			}
			if groupRow >= 0 && turnRows[groupRow].Request == nil {
				turnRows[groupRow].Request = r.Request
			}
			turnRows = append(turnRows, ledgerUIRow{Kind: ledgerRowRecord, Record: r, Height: ledgerCellHeight,
				Dimmed: len(focus) > 0 && !focus[r.ID]})
		}
		if len(turnRows) == 0 {
			continue
		}
		rows = append(rows, ledgerUIRow{Kind: ledgerRowTurn, Turn: turn, Height: ledgerTurnHeight})
		rows = append(rows, turnRows...)
	}
	return rows
}

func collapseLedgerRows(rows []ledgerUIRow, s *ledgerViewState) []ledgerUIRow {
	// Turn 折叠保留 requestOnly/system 与首条内容（TrajectoryTable.tsx:639-671）。
	var turnCollapsed []ledgerUIRow
	for i := 0; i < len(rows); i++ {
		if rows[i].Kind != ledgerRowTurn || rows[i].Turn == nil || !s.collapsedTurns[rows[i].Turn.Turn] {
			turnCollapsed = append(turnCollapsed, rows[i])
			continue
		}
		turn := rows[i].Turn
		turnCollapsed = append(turnCollapsed, rows[i])
		content := 0
		first := -1
		for j := i + 1; j < len(rows) && rows[j].Kind != ledgerRowTurn; j++ {
			if rows[j].Kind != ledgerRowRecord || rows[j].Record.RequestOnly || rows[j].Record.Kind == ledgerSystem {
				continue
			}
			content++
			if first < 0 {
				first = j
			}
		}
		if content <= 1 {
			continue
		}
		toolCount := 0
		for j := i + 1; j < len(rows) && rows[j].Kind != ledgerRowTurn; j++ {
			r := rows[j]
			if r.Kind == ledgerRowRecord && !r.Record.RequestOnly && r.Record.Kind != ledgerSystem && j != first {
				if r.Record.Kind == ledgerTool || r.Record.Kind == ledgerSubtool {
					toolCount++
				}
				continue
			}
			turnCollapsed = append(turnCollapsed, r)
			if j == first {
				turnCollapsed = append(turnCollapsed, ledgerUIRow{Kind: ledgerRowSummary, SummaryTurn: turn.Turn,
					Text: fmt.Sprintf("已收起轮次 · %d 个步骤 · %d 个工具调用", content-1, toolCount), Height: ledgerSummaryHeight})
			}
		}
		for i+1 < len(rows) && rows[i+1].Kind != ledgerRowTurn {
			i++
		}
	}
	// Assistant/SubCalls 折叠只跳过自己的后代，不吃掉下一条平级记录。
	out := make([]ledgerUIRow, 0, len(turnCollapsed))
	for i := 0; i < len(turnCollapsed); i++ {
		r := turnCollapsed[i]
		out = append(out, r)
		if r.Kind != ledgerRowRecord {
			continue
		}
		id := ""
		assistant := r.Record.Kind == ledgerMessage && s.collapsedAssistant[r.Record.ID]
		tool := r.Record.Kind == ledgerTool && s.collapsedTools[r.Record.ID]
		if assistant {
			id = r.Record.ID
		} else if tool {
			id = r.Record.ID
		} else {
			continue
		}
		skipped := 0
		for i+1 < len(turnCollapsed) {
			next := turnCollapsed[i+1]
			if next.Kind != ledgerRowRecord {
				break
			}
			if assistant && next.Record.AssistantID != id {
				break
			}
			if tool && (next.Record.Kind != ledgerSubtool || !ledgerDescendant(next.Record, id)) {
				break
			}
			skipped++
			i++
		}
		if skipped > 0 {
			label := "已收起调用"
			if tool {
				label = "已收起子调用"
			}
			out = append(out, ledgerUIRow{Kind: ledgerRowSummary, SummaryID: id, Text: fmt.Sprintf("%s · %d 项", label, skipped), Height: ledgerSummaryHeight})
		}
	}
	return out
}

func ledgerDescendant(r *ledgerRecord, parent string) bool {
	// 投影已保证 ParentID 链无环；Depth 同时阻止异常输入造成无限循环。
	for depth := 0; r != nil && depth <= 256; depth++ {
		if r.ParentID == parent {
			return true
		}
		if r.ParentID == "" {
			return false
		}
		// UI 只需要判断直接/间接父子；fold 阶段 ParentID 已指向真实父记录。
		return true
	}
	return false
}

func (p *Plugin) drawLedgerSplit(f ui.C, env *pluginapi.Env, data pluginapi.Snapshot, s *ledgerViewState, model, window *ledgerModel, rows []ledgerUIRow, scope, query string) {
	width := f.Bounds().Width
	if width <= 0 {
		width = 500
	}
	selected := selectedLedgerRecord(model, s.selectedRecord)
	request := model.Requests[s.selectedRequest]
	details := selected != nil || request != nil
	listWidth := width
	overlay := details && width <= 760
	if details && !overlay {
		listWidth = ledgerListWidth(width, s.detailsWidth)
	}
	ui.Row(f).WidthPercent(100).Grow(1).MinHeight(100).Children(func() {
		ui.Column(f).Width(listWidth).FillHeight().Children(func() {
			p.drawLedgerHistory(f, env, data, s, model, window)
			if len(rows) == 0 {
				message := "本会话还没有轨迹记录"
				if query != "" {
					message = "没有匹配「" + query + "」的记录"
				} else if s.selection != nil {
					message = "时间范围内没有记录"
				}
				ui.Column(f).Grow(1).Justify(ui.Center).Items(ui.Center).Children(func() {
					ui.Text(f, message).FontSize(p.theme.Fonts.Secondary).Color(p.theme.LabelTertiary)
				})
				return
			}
			p.drawLedgerRows(f, env, data, s, rows, scope)
		})
		if details && overlay {
			overlayWidth := float32(math.Min(420, float64(width)*0.92))
			ui.Column(f).Absolute().Right(0).Top(0).Width(overlayWidth).HeightPercent(100).Children(func() {
				p.drawLedgerDetails(f, env, s, model, selected, request, overlayWidth)
			})
		} else if details {
			p.drawLedgerDetails(f, env, s, model, selected, request, width-listWidth)
		}
	})
}

func (p *Plugin) drawLedgerHistory(f ui.C, env *pluginapi.Env, data pluginapi.Snapshot, s *ledgerViewState, model, window *ledgerModel) {
	resident := len(model.Records) > len(window.Records)
	h := data.History
	if !resident && !h.Partial && !h.HasMore && !h.Loading && h.Error == "" {
		return
	}
	label := "加载更早的历史"
	enabled := env.Act.LoadOlder != nil || resident
	switch {
	case h.Loading:
		label, enabled = "正在加载更早的历史…", false
	case h.Error != "":
		label = "重新加载"
	case !resident && !h.HasMore:
		label, enabled = "已到会话开头", false
	}
	t := p.theme
	box := ui.Row(f).WidthPercent(100).MinHeight(28).Items(ui.Center).Justify(ui.Center)
	if enabled {
		box = box.Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, "ledger-load-older"), label)
	}
	box.Children(func() { ui.Text(f, label).FontSize(t.Fonts.Caption).Color(t.LabelCaption) })
	if enabled && box.Clicked() {
		if resident {
			s.historyLimit += ledgerHistoryPage
		} else if env.Act.LoadOlder != nil {
			s.historyLimit += ledgerHistoryPage
			env.Act.LoadOlder()
		}
	}
}

func (p *Plugin) drawLedgerRows(f ui.C, env *pluginapi.Env, data pluginapi.Snapshot, s *ledgerViewState, rows []ledgerUIRow, scope string) {
	area := ui.Scroll(f.Key("ledger-rows-" + scope + data.Cur)).WidthPercent(100).Grow(1).MinHeight(80).Gap(ledgerRowGap).Clip()
	state := area.ScrollState()
	viewport, offset := state.Viewport, state.Offset
	if viewport <= 0 {
		viewport = f.Bounds().Height
		if viewport < 120 {
			viewport = 120
		}
	}
	area.Children(func() {
		ui.Column(f).WidthPercent(100).Gap(ledgerRowGap).Children(func() {
			top := float32(0)
			for i := range rows {
				row := rows[i]
				if top+row.Height < offset-ledgerOverscan || top > offset+viewport+ledgerOverscan {
					ui.Slot(f).Height(row.Height)
				} else {
					p.drawLedgerRow(f, env, data, s, row, scope)
				}
				top += row.Height + ledgerRowGap
			}
		})
	})
}

func (p *Plugin) drawLedgerRow(f ui.C, env *pluginapi.Env, data pluginapi.Snapshot, s *ledgerViewState, row ledgerUIRow, scope string) {
	t := p.theme
	switch row.Kind {
	case ledgerRowTurn:
		open := !s.collapsedTurns[row.Turn.Turn]
		title := ledgerTurnTitle(row.Turn)
		if title == "" {
			title = "轮次之间"
		}
		label := "收起" + title
		if !open {
			label = "展开" + title
		}
		box := ui.Row(f).WidthPercent(100).MinHeight(ledgerTurnHeight).Gap(6).PadX(8).Items(ui.Center).
			Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, scope+"ledger-turn-"+itoa(row.Turn.Turn)), label)
		box.Children(func() {
			ui.Text(f, ledgerArrow(open)).FontSize(12).Color(t.LabelCaption)
			ui.Text(f, title).FontSize(12).Weight(500).Color(t.LabelTertiary)
		})
		if box.Clicked() {
			s.collapsedTurns[row.Turn.Turn] = !s.collapsedTurns[row.Turn.Turn]
		}
	case ledgerRowGroup:
		ui.Row(f).WidthPercent(100).MinHeight(ledgerGroupHeight).Items(ui.Center).PadX(ledgerGroupPadX).Gap(24).Children(func() {
			ui.Text(f, ledgerGroupLabel(row.Group)).FontSize(13).Color(t.LabelPrimary)
			ui.Spacer(f)
			if row.Request != nil {
				r := row.Request
				label := "请求 #" + itoa(r.Number)
				if r.Compaction {
					label += " · 压缩"
				}
				ledgerActionNoopGuard(f, env, "ledger-request-"+r.ID, label, func() {
					s.selectedRequest, s.selectedRecord = r.ID, ""
				}, s.selectedRequest == r.ID)
			}
		})
	case ledgerRowRequest:
		if row.Request != nil {
			r := row.Request
			label := "请求 #" + itoa(r.Number)
			box := ui.Row(f).WidthPercent(100).Height(9).Items(ui.Center).Justify(ui.Center).
				Hit(env.Tree.ID(env.Feat, scope+"ledger-request-only-"+r.ID), label)
			box.Children(func() { ui.Text(f, label).FontSize(t.Fonts.Tiny).Color(t.LabelCaption) })
			if box.Clicked() {
				s.selectedRequest, s.selectedRecord = r.ID, ""
			}
		}
	case ledgerRowRecord:
		p.drawLedgerRecord(f, env, s, row.Record, scope, row.Dimmed)
	case ledgerRowSummary:
		label := row.Text
		box := ui.Row(f).WidthPercent(100).MinHeight(ledgerSummaryHeight).Items(ui.Center).PadX(8).
			Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, scope+"ledger-summary-"+row.SummaryID+itoa(row.SummaryTurn)), label)
		box.Children(func() { ui.Text(f, label).FontSize(12).LineHeight(16).Color(t.LabelSecondary) })
		if box.Clicked() {
			if row.SummaryTurn != 0 {
				s.collapsedTurns[row.SummaryTurn] = false
			}
			delete(s.collapsedAssistant, row.SummaryID)
			delete(s.collapsedTools, row.SummaryID)
		}
	}
}

func ledgerActionNoopGuard(f ui.C, env *pluginapi.Env, name, label string, fn func(), active bool) {
	drawLedgerAction(f, env, name, label, fn, active)
}

func ledgerTurnTitle(turn *ledgerTurn) string {
	if turn == nil {
		return ""
	}
	if turn.Turn > 0 {
		return "第 " + itoa(turn.Turn) + " 轮"
	}
	return turn.Title
}

func ledgerGroupLabel(group string) string {
	if group == "Message" {
		return "消息"
	}
	if strings.HasPrefix(group, "Step ") {
		return "第 " + strings.TrimPrefix(group, "Step ") + " 步"
	}
	if strings.HasPrefix(group, "Compaction ") {
		return "压缩 " + strings.TrimPrefix(group, "Compaction ")
	}
	return group
}

func ledgerArrow(open bool) string {
	if open {
		return "▾"
	}
	return "▸"
}

func (p *Plugin) drawLedgerRecord(f ui.C, env *pluginapi.Env, s *ledgerViewState, r *ledgerRecord, scope string, dimmed bool) {
	t := p.theme
	selected := s.selectedRecord == r.ID
	label := ledgerRecordLabel(r)
	// TrajectoryTable.module.css:153-161,458-496：30px 单行、左 20/右 8、12px 文本。
	box := ui.Row(f).WidthPercent(100).MinHeight(ledgerCellHeight).Gap(4).PadSides(0, 8, 0, 20).
		Items(ui.Center).Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, scope+"ledger-record-"+r.ID), label)
	if selected {
		box = box.Background(t.BgLayer2)
	}
	textColor := t.LabelPrimary
	if dimmed {
		textColor = t.LabelDimmed
	}
	box.Children(func() {
		ui.Text(f, "#"+itoa(r.Index)).Width(24).FontSize(12).Color(t.LabelTertiary)
		ui.Text(f, ledgerKindLabel(r.Kind)).Width(44).FontSize(10).LineHeight(16).Weight(650).Color(ledgerKindColor(t, r.Kind))
		ui.Text(f, ledgerRecordText(r)).FontSize(12).LineHeight(18).Color(textColor).Ellipsis().Grow(1)
		preview := r.Preview
		if preview == r.Text {
			preview = ""
		}
		if preview == "" && (r.Kind == ledgerTool || r.Kind == ledgerSubtool) {
			preview = r.Output
		}
		if preview != "" {
			ui.Text(f, oneLine(preview, 60)).FontSize(12).LineHeight(18).Color(t.LabelSecondary).Ellipsis().Grow(1)
		}
		if r.IsError {
			p.prims.Badge(f, "失败", t.Error)
		}
		if r.DurationMS != nil {
			ui.Text(f, formatLedgerMS(*r.DurationMS)).Width(71).FontSize(12).Color(t.LabelTertiary)
		} else {
			ui.Text(f, "—").Width(71).FontSize(12).Color(t.LabelTertiary)
		}
	})
	if box.Clicked() {
		if r.Kind == ledgerMessage && ledgerDoubleClick(s, "assistant:"+r.ID) && ledgerHasAssistantCalls(r) {
			s.collapsedAssistant[r.ID] = !s.collapsedAssistant[r.ID]
		} else {
			if s.selection != nil && !s.timelineFocus[r.ID] {
				s.selection = nil
			}
			s.selectedRecord, s.selectedRequest = r.ID, ""
		}
	}
}

func ledgerDoubleClick(s *ledgerViewState, key string) bool {
	now := time.Now()
	previous := s.lastClick[key]
	s.lastClick[key] = now
	return !previous.IsZero() && now.Sub(previous) <= 350*time.Millisecond
}

func ledgerHasAssistantCalls(r *ledgerRecord) bool { return r != nil && r.Kind == ledgerMessage }

func ledgerKindLabel(kind ledgerKind) string {
	switch kind {
	case ledgerSystem:
		return "系统"
	case ledgerUser:
		return "用户"
	case ledgerContext:
		return "上下文"
	case ledgerCompacted:
		return "已压缩"
	case ledgerMessage:
		return "助手"
	case ledgerTool:
		return "工具"
	case ledgerSubtool:
		return "子工具"
	default:
		return string(kind)
	}
}

func ledgerRecordText(r *ledgerRecord) string {
	if r.Kind == ledgerSystem {
		switch r.Text {
		case "Initial system prompt":
			return "初始系统提示词"
		case "Tools updated":
			return "工具已更新"
		case "System prompt and tools updated":
			return "系统提示词与工具已更新"
		case "System prompt updated":
			return "系统提示词已更新"
		}
	}
	if r.Kind == ledgerCompacted {
		switch r.Text {
		case "Compacting":
			return "正在压缩"
		case "Compacted":
			return "压缩完成"
		case ledgerCompactionInterrupted:
			return "压缩已中断"
		}
	}
	if r.Text != "" {
		return r.Text
	}
	if r.Preview != "" {
		return oneLine(r.Preview, 100)
	}
	if r.RequestOnly {
		return "请求"
	}
	return "无内容"
}

func ledgerRecordLabel(r *ledgerRecord) string {
	return fmt.Sprintf("#%d %s · %s", r.Index, ledgerKindLabel(r.Kind), ledgerRecordText(r))
}

func ledgerContentRecordCount(model *ledgerModel) int {
	count := 0
	if model != nil {
		for _, record := range model.Records {
			if !record.RequestOnly {
				count++
			}
		}
	}
	return count
}

func selectedLedgerRecord(model *ledgerModel, id string) *ledgerRecord {
	if model == nil || id == "" {
		return nil
	}
	for _, r := range model.Records {
		if r.ID == id {
			return r
		}
	}
	return nil
}

func ledgerListWidth(width, details float32) float32 {
	if details <= 0 {
		details = float32(math.Max(320, math.Min(440, float64(width)*0.38)))
	}
	minDetails := float32(160)
	if width >= 600 {
		minDetails = 320
	}
	details = float32(math.Max(float64(minDetails), math.Min(720, float64(details))))
	minList := float32(140)
	if width >= 600 {
		minList = 280
	}
	if details > width-minList {
		details = width - minList
	}
	if details < 100 {
		details = width / 2
	}
	return width - details
}

func (p *Plugin) drawLedgerDetails(f ui.C, env *pluginapi.Env, s *ledgerViewState, model *ledgerModel, record *ledgerRecord, request *ledgerRequest, width float32) {
	t := p.theme
	key := s.selectedRecord
	if request != nil {
		key = "request:" + request.ID
	}
	tabs := ledgerDetailTabs(record, request)
	active := s.tabs[key]
	if !ledgerTabAllowed(active, tabs) {
		active = tabs[0].ID
		for i := len(s.recentTabs) - 1; i >= 0; i-- {
			if ledgerTabAllowed(s.recentTabs[i], tabs) {
				active = s.recentTabs[i]
				break
			}
		}
		s.tabs[key] = active
	}
	ui.Column(f).Width(width).FillHeight().Background(t.BgLayer1).Clip().Children(func() {
		// TrajectoryTable.module.css:756-769：详情头 42px，左 12/右 8。
		ui.Row(f).WidthPercent(100).MinHeight(42).Items(ui.Center).Gap(8).PadSides(0, 8, 0, 12).Children(func() {
			ui.Text(f, "事件详情").FontSize(13).Color(t.LabelSecondary)
			ui.Spacer(f)
			close := ui.Row(f).Width(28).Height(28).Items(ui.Center).Justify(ui.Center).Radius(t.Radii.SM).
				Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, "ledger-close-details"), "关闭详情")
			close.Children(func() { ui.Text(f, "×").FontSize(18).LineHeight(18).Color(t.LabelSecondary) })
			if close.Clicked() {
				s.selectedRecord, s.selectedRequest = "", ""
			}
		})
		// :986-1017：tabs 34px、左右 8、gap 1；每页左右 9、13px 字。
		ui.Row(f).WidthPercent(100).MinHeight(34).Gap(1).PadX(8).Items(ui.Center).Children(func() {
			for _, tab := range tabs {
				tab := tab
				drawLedgerDetailTab(f, env, "ledger-tab-"+key+"-"+tab.ID, tab.Label, func() {
					s.tabs[key] = tab.ID
					s.recentTabs = append(s.recentTabs, tab.ID)
					if len(s.recentTabs) > 16 {
						s.recentTabs = s.recentTabs[len(s.recentTabs)-16:]
					}
				}, active == tab.ID)
			}
		})
		p.drawLedgerResizeHandle(f, env, s, width)
		area := ui.Scroll(f.Key("ledger-details-" + key)).WidthPercent(100).Grow(1).Clip()
		area.Children(func() {
			// :1047-1062,1463-1473：正文左右 14、上 8/下 12；载荷 12/19。
			ui.Column(f).WidthPercent(100).PadSides(8, 14, 12, 14).Gap(6).Children(func() {
				if env.Act.Copy != nil {
					drawLedgerAction(f, env, "ledger-copy-detail", "复制", func() { env.Act.Copy(ledgerDetailText(record, request, active, model)) }, false)
				}
				ui.Wrap(f, ledgerDetailText(record, request, active, model)).Mono().FontSize(12).LineHeight(19).Color(t.LabelSecondary)
			})
		})
		ui.Row(f).Absolute().Left(0).Top(0).Width(0.5).HeightPercent(100).Background(t.BorderL2)
	})
}

type ledgerTab struct{ ID, Label string }

func ledgerDetailTabs(record *ledgerRecord, request *ledgerRequest) []ledgerTab {
	if request != nil {
		tabs := []ledgerTab{{"overview", "概述"}}
		if request.Options != "" {
			tabs = append(tabs, ledgerTab{"options", "选项"})
		}
		return append(tabs, ledgerTab{"usage", "用量"}, ledgerTab{"timing", "计时"})
	}
	if record == nil {
		return []ledgerTab{{"overview", "概述"}}
	}
	if record.Kind == ledgerSystem {
		tabs := []ledgerTab{}
		if record.PreviousPrompt != "" || record.Diff != "" {
			tabs = append(tabs, ledgerTab{"diff", "差异"})
		}
		tabs = append(tabs, ledgerTab{"system-prompt", "系统提示词"})
		if record.Tools != "" {
			tabs = append(tabs, ledgerTab{"tools", "工具"})
		}
		return tabs
	}
	if record.Kind == ledgerCompacted {
		return []ledgerTab{{"overview", "概述"}, {"raw", "原始输出"}}
	}
	if record.Kind == ledgerUser || record.Kind == ledgerContext || record.Kind == ledgerMessage {
		tabs := []ledgerTab{{"overview", "概述"}, {"rendered", "预览"}, {"raw", "原始内容"}}
		if record.Source != "" {
			tabs = append(tabs, ledgerTab{"source", "来源"})
		}
		return tabs
	}
	tabs := []ledgerTab{{"overview", "概述"}}
	if record.Input != "" {
		label := "参数"
		if _, _, ok := ledgerCodeProgram(record); ok {
			label = "代码"
		}
		tabs = append(tabs, ledgerTab{"input", label})
	}
	if record.Output != "" || record.CompletedAt != nil {
		tabs = append(tabs, ledgerTab{"output", "结果"})
	}
	return append(tabs, ledgerTab{"schema", "Schema"}, ledgerTab{"timing", "计时"})
}

func ledgerTabAllowed(id string, tabs []ledgerTab) bool {
	for _, tab := range tabs {
		if tab.ID == id {
			return true
		}
	}
	return false
}

func ledgerDetailText(record *ledgerRecord, request *ledgerRequest, tab string, model *ledgerModel) string {
	if request != nil {
		switch tab {
		case "options":
			return valueOr(request.Options, "未记录选项")
		case "usage":
			return "本次请求\n" + ledgerRequestUsageText(request.Usage) + "\n\n会话累计\n" + ledgerRequestUsageText(request.CumulativeUsage)
		case "timing":
			if record := ledgerRecordForRequest(model, request.ID); record != nil {
				return ledgerTimingText(record.StartedAt, record.CompletedAt, record.FirstTokenAt, record.Usage)
			}
			return ledgerTimingText(request.StartedAt, request.CompletedAt, nil, request.Usage)
		default:
			return ledgerRequestOverview(request, model)
		}
	}
	if record == nil {
		return "无内容"
	}
	switch tab {
	case "rendered":
		return valueOr(firstNonempty(record.Output, record.Input, record.Thinking, record.Preview), "无内容")
	case "raw":
		if record.Kind == ledgerCompacted {
			return valueOr(firstNonempty(record.Raw, record.Output), "无内容")
		}
		return valueOr(firstNonempty(record.Raw, record.Output, record.Input, record.Thinking), "无内容")
	case "source":
		return valueOr(record.Source, "未记录来源")
	case "input":
		if code, description, ok := ledgerCodeProgram(record); ok {
			return "描述\n" + description + "\n\n代码\n" + ledgerNumberedCode(code) + "\n\n原始 JSON\n" + record.Input
		}
		return valueOr(record.Input, "未捕获参数")
	case "output":
		fallback := "未捕获结果"
		if record.CompletedAt != nil {
			fallback = "无输出"
		}
		return "渲染结果\n" + valueOr(record.Output, fallback) +
			"\n\n原始结果\n原始结果仅在实时工具调用中可用"
	case "schema":
		return valueOr(record.Schema, "Schema 不可用")
	case "timing":
		return ledgerTimingText(record.StartedAt, record.CompletedAt, record.FirstTokenAt, record.Usage)
	case "diff":
		return ledgerCombinedDiff(record)
	case "system-prompt":
		return valueOr(ledgerSystemPromptText(record), "本次请求没有系统提示词")
	case "tools":
		return valueOr(record.Tools, "本次请求没有工具")
	default:
		return ledgerRecordOverview(record, model)
	}
}

func ledgerCodeProgram(r *ledgerRecord) (code, description string, ok bool) {
	if r == nil || (r.Text != "run_code" && !strings.HasPrefix(r.Text, "run_code ")) {
		return "", "", false
	}
	var input struct {
		Code        any `json:"code"`
		Description any `json:"description"`
	}
	if json.Unmarshal([]byte(r.Input), &input) != nil {
		return "", "", false
	}
	code, codeOK := input.Code.(string)
	if !codeOK {
		return "", "", false
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal([]byte(r.Input), &fields)
	if _, exists := fields["description"]; exists {
		value, ok := input.Description.(string)
		if !ok {
			return "", "", false
		}
		description = value
	}
	if description == "" {
		for _, line := range strings.Split(code, "\n") {
			if strings.TrimSpace(line) != "" {
				description = strings.TrimSpace(line)
				break
			}
		}
	}
	return code, description, true
}

func ledgerNumberedCode(code string) string {
	lines := strings.Split(code, "\n")
	for i := range lines {
		lines[i] = fmt.Sprintf("%4d  %s", i+1, lines[i])
	}
	return strings.Join(lines, "\n")
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func ledgerRecordForRequest(model *ledgerModel, id string) *ledgerRecord {
	if model == nil {
		return nil
	}
	var anchor *ledgerRecord
	for _, record := range model.Records {
		if record.Request == nil || record.Request.ID != id {
			continue
		}
		if !record.RequestOnly {
			return record
		}
		anchor = record // attempt-only 请求也用真实 stream timing，不回退成“首 token 不可用”。
	}
	return anchor
}

func ledgerRequestUsageText(usage *ledgerUsage) string {
	if usage == nil {
		return "未报告用量"
	}
	value := func(p *int64) int64 {
		if p == nil {
			return 0
		}
		return *p
	}
	var sections []string
	if usage.Input != nil || usage.CacheRead != nil || usage.CacheWrite != nil {
		var b strings.Builder
		fmt.Fprintf(&b, "输入\n%d tok", value(usage.Input)+value(usage.CacheRead)+value(usage.CacheWrite))
		if usage.CacheRead != nil {
			fmt.Fprintf(&b, "\n  缓存读取 %d tok", *usage.CacheRead)
		}
		if usage.CacheWrite != nil {
			fmt.Fprintf(&b, "\n  缓存写入 %d tok", *usage.CacheWrite)
		}
		if usage.Input != nil {
			fmt.Fprintf(&b, "\n  其他 %d tok", *usage.Input)
		}
		sections = append(sections, b.String())
	}
	if usage.Output != nil || usage.Reasoning != nil {
		var b strings.Builder
		if usage.Output != nil {
			fmt.Fprintf(&b, "输出\n%d tok", *usage.Output)
		}
		if usage.Reasoning != nil {
			fmt.Fprintf(&b, "  推理 %d tok", *usage.Reasoning)
			if usage.Output != nil {
				content := *usage.Output - *usage.Reasoning
				if content < 0 {
					content = 0
				}
				fmt.Fprintf(&b, "\n  内容 %d tok", content)
			}
		}
		sections = append(sections, b.String())
	}
	if len(sections) == 0 {
		return "未报告用量"
	}
	return strings.Join(sections, "\n")
}

func ledgerSystemPromptText(record *ledgerRecord) string {
	if record == nil {
		return ""
	}
	var prompt struct {
		System string `json:"system"`
	}
	if json.Unmarshal([]byte(record.Prompt), &prompt) == nil && prompt.System != "" {
		return prompt.System
	}
	return record.Prompt
}

func ledgerCombinedDiff(record *ledgerRecord) string {
	if record == nil {
		return ""
	}
	var previous, current struct {
		System string          `json:"system"`
		Tools  json.RawMessage `json:"tools"`
	}
	_ = json.Unmarshal([]byte(record.PreviousPrompt), &previous)
	_ = json.Unmarshal([]byte(record.Prompt), &current)
	systemDiff := record.Diff
	if systemDiff == "" {
		systemDiff = ledgerDiff(previous.System, current.System)
	}
	toolsDiff := ledgerDiff(string(previous.Tools), string(current.Tools))
	var sections []string
	if systemDiff != "" {
		sections = append(sections, "系统提示词\n"+systemDiff)
	}
	if toolsDiff != "" {
		sections = append(sections, "工具\n"+toolsDiff)
	}
	if len(sections) == 0 {
		return "没有系统提示词或工具差异"
	}
	return strings.Join(sections, "\n\n")
}

func ledgerRequestOverview(r *ledgerRequest, model *ledgerModel) string {
	var b strings.Builder
	writeLedgerField(&b, "请求", "#"+itoa(r.Number))
	writeLedgerField(&b, "状态", ledgerStatusLabel(r.Status))
	writeLedgerField(&b, "用途", valueOr(r.Purpose, "助手"))
	writeLedgerField(&b, "提供方", r.Provider)
	writeLedgerField(&b, "模型", r.Model)
	writeLedgerField(&b, "Turn / Step", itoa(r.Turn)+" / "+itoa(r.Step))
	tools, subtools := 0, 0
	if model != nil {
		for _, record := range model.Records {
			if record.AssistantID != ledgerAssistantID(r.Turn, r.Step) {
				continue
			}
			if record.Kind == ledgerTool {
				tools++
			} else if record.Kind == ledgerSubtool {
				subtools++
			}
		}
	}
	writeLedgerField(&b, "工具调用", itoa(tools))
	if subtools > 0 {
		writeLedgerField(&b, "子工具调用", itoa(subtools))
	}
	writeLedgerField(&b, "错误代码", r.ErrorCode)
	if r.ErrorCode == "AUTH" {
		writeLedgerField(&b, "错误", "API 密钥无效")
	} else if r.ErrorCode == ledgerCompactionInterrupted {
		writeLedgerField(&b, "错误", "压缩已中断")
	} else {
		writeLedgerField(&b, "错误", r.Error)
	}
	if r.Retry != nil {
		writeLedgerField(&b, "重试", strconv.FormatInt(*r.Retry, 10))
	}
	if r.MaxRetries != nil {
		writeLedgerField(&b, "最大重试", strconv.FormatInt(*r.MaxRetries, 10))
	}
	if r.RetryDelayMS != nil {
		writeLedgerField(&b, "重试延迟", formatLedgerMS(*r.RetryDelayMS))
	}
	if r.ResultSeq != 0 {
		writeLedgerField(&b, "结果 seq", itoa(r.ResultSeq))
	}
	return b.String()
}

func ledgerRecordOverview(r *ledgerRecord, model *ledgerModel) string {
	var b strings.Builder
	writeLedgerField(&b, "记录", "#"+itoa(r.Index))
	writeLedgerField(&b, "类型", ledgerKindLabel(r.Kind))
	writeLedgerField(&b, "状态", ledgerRecordStatus(r))
	writeLedgerField(&b, "Turn / Step", itoa(r.Turn)+" / "+itoa(r.Step))
	writeLedgerField(&b, "内容", ledgerRecordText(r))
	writeLedgerField(&b, "调用 ID", r.ID)
	writeLedgerField(&b, "父调用", r.ParentID)
	writeLedgerField(&b, "助手调用", r.AssistantID)
	if r.Request != nil {
		writeLedgerField(&b, "请求", "#"+itoa(r.Request.Number))
		writeLedgerField(&b, "提供方", r.Request.Provider)
		writeLedgerField(&b, "模型", r.Request.Model)
	}
	if r.Usage != nil {
		writeLedgerField(&b, "用量", ledgerUsageJSON(r.Usage))
	}
	return b.String()
}

func writeLedgerField(b *strings.Builder, name, value string) {
	if value == "" {
		return
	}
	b.WriteString(name)
	b.WriteString("\n")
	b.WriteString(value)
	b.WriteString("\n\n")
}

func ledgerStatusLabel(status string) string {
	switch status {
	case "complete":
		return "已完成"
	case "running":
		return "等待中"
	case "error":
		return "失败"
	default:
		return status
	}
}

func ledgerRecordStatus(r *ledgerRecord) string {
	if r.IsError {
		return "失败"
	}
	if (r.Kind == ledgerTool || r.Kind == ledgerSubtool || r.Kind == ledgerMessage) && r.CompletedAt == nil {
		return "等待中"
	}
	if r.Kind == ledgerCompacted && r.CompletedAt == nil {
		return "等待中"
	}
	return "已完成"
}

func ledgerTimingText(started, completed, first *int64, usage *ledgerUsage) string {
	var b strings.Builder
	if started == nil {
		writeLedgerField(&b, "开始时间", "不可用")
		writeLedgerField(&b, "总时长", "未记录")
	} else {
		writeLedgerField(&b, "开始时间", formatLedgerTimestamp(*started))
		if completed == nil {
			writeLedgerField(&b, "总时长", "等待中")
		} else {
			writeLedgerField(&b, "总时长", formatLedgerMS(maxLedger(0, *completed-*started)))
		}
	}
	if started == nil || first == nil {
		writeLedgerField(&b, "首 token 延迟", "首 token 时间不可用")
	} else {
		writeLedgerField(&b, "首 token 延迟", formatLedgerMS(maxLedger(0, *first-*started)))
	}
	if first == nil {
		writeLedgerField(&b, "生成", "首 token 时间不可用")
	} else if completed == nil {
		writeLedgerField(&b, "生成", "等待中")
	} else {
		writeLedgerField(&b, "生成", formatLedgerMS(maxLedger(0, *completed-*first)))
	}
	if usage == nil || usage.Output == nil {
		writeLedgerField(&b, "吞吐量", "用量不可用")
	} else if first == nil || completed == nil || *completed <= *first {
		writeLedgerField(&b, "吞吐量", "时长过短")
	} else {
		rate := float64(*usage.Output) / (float64(*completed-*first) / 1000)
		writeLedgerField(&b, "吞吐量", fmt.Sprintf("%.1f tok/s", rate))
	}
	return b.String()
}

func formatLedgerTimestamp(ms int64) string {
	return time.UnixMilli(ms).Local().Format("2006-01-02 15:04:05.000")
}

func formatLedgerMS(ms int64) string {
	if ms < 1000 {
		return itoa(int(ms)) + " 毫秒"
	}
	places := 1
	if ms < 10000 {
		places = 2
	}
	return fmt.Sprintf("%.*f 秒", places, float64(ms)/1000)
}

func maxLedger(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (p *Plugin) drawLedgerResizeHandle(f ui.C, env *pluginapi.Env, s *ledgerViewState, width float32) {
	id := env.Tree.ID(env.Feat, "ledger-details-resize")
	// TrajectoryTable.module.css:779-789：左侧 -4px 起的 8px 垂直命中带，不画横条。
	ui.Row(f).Absolute().Left(-4).Top(0).Width(8).HeightPercent(100).
		HitWith(id, "调整事件详情宽度", gfx.HitOptions{Cursor: gfx.PointerCursorResizeHorizontal})
	c := env.Canvas
	if c == nil {
		return
	}
	ev, ok := c.PointerOn(id)
	if !ok {
		return
	}
	switch ev.Kind {
	case gfx.PointerDown:
		if ev.Button != gfx.PointerButtonPrimary && ev.Button != gfx.PointerButtonNone {
			return
		}
		s.resizing, s.resizeX, s.resizeWidth = true, ev.X, width
		c.Consume()
	case gfx.PointerMove:
		if s.resizing {
			s.detailsWidth = s.resizeWidth - (ev.X - s.resizeX)
			c.Consume()
		}
	case gfx.PointerUp:
		s.resizing = false
		c.Consume()
	}
}

func ledgerCollapsibleIDs(model *ledgerModel) (turns []int, assistants, tools []string) {
	if model == nil {
		return
	}
	for _, turn := range model.Turns {
		count := 0
		for _, r := range turn.Records {
			if !r.RequestOnly && r.Kind != ledgerSystem {
				count++
			}
		}
		if turn.Turn != 0 && count > 1 {
			turns = append(turns, turn.Turn)
		}
	}
	for i, r := range model.Records {
		if r.Kind == ledgerMessage && i+1 < len(model.Records) {
			next := model.Records[i+1]
			if next.AssistantID == r.ID && (next.Kind == ledgerTool || next.Kind == ledgerSubtool) {
				assistants = append(assistants, r.ID)
			}
		}
		if r.Kind == ledgerTool && i+1 < len(model.Records) && model.Records[i+1].Kind == ledgerSubtool && model.Records[i+1].ParentID == r.ID {
			tools = append(tools, r.ID)
		}
	}
	return
}

func allLedgerTurnsCollapsed(s *ledgerViewState, ids []int) bool {
	if len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		if !s.collapsedTurns[id] {
			return false
		}
	}
	return true
}

func toggleLedgerTurns(s *ledgerViewState, ids []int) {
	next := !allLedgerTurnsCollapsed(s, ids)
	for _, id := range ids {
		s.collapsedTurns[id] = next
	}
}

func allLedgerAssistantsCollapsed(s *ledgerViewState, ids []string) bool {
	return allLedgerIDsCollapsed(s.collapsedAssistant, ids)
}

func toggleLedgerAssistants(s *ledgerViewState, ids []string) {
	toggleLedgerIDs(s.collapsedAssistant, ids)
}

func allLedgerToolsCollapsed(s *ledgerViewState, ids []string) bool {
	return allLedgerIDsCollapsed(s.collapsedTools, ids)
}

func toggleLedgerTools(s *ledgerViewState, ids []string) {
	toggleLedgerIDs(s.collapsedTools, ids)
}

func allLedgerIDsCollapsed(state map[string]bool, ids []string) bool {
	if len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		if !state[id] {
			return false
		}
	}
	return true
}

func toggleLedgerIDs(state map[string]bool, ids []string) {
	next := !allLedgerIDsCollapsed(state, ids)
	for _, id := range ids {
		state[id] = next
	}
}

// HandleScroll 实现官方 wheel 缩放（TrajectoryTimeline.tsx:355-386）。
func (p *Plugin) HandleScroll(ev gfx.ScrollEvent) gfx.EventResult {
	s := p.ledgerStates[p.ledgerSession]
	if s == nil || s.timelineRect.Width <= 0 {
		return gfx.EventResult{}
	}
	x, y := float32(ev.X), float32(ev.Y)
	r := s.timelineRect
	if x < r.X || x > r.X+r.Width || y < r.Y || y > r.Y+r.Height {
		return gfx.EventResult{}
	}
	full := s.timelineFull
	fullDuration := math.Max(1, full.End-full.Start)
	domain := full
	if s.viewport != nil {
		domain = *s.viewport
	}
	domainDuration := math.Max(1, domain.End-domain.Start)
	minimum := 20.0
	if s.timelineMode == "sequence" {
		minimum = 4
	}
	minimum = math.Min(minimum, fullDuration)
	nextDuration := math.Min(fullDuration, math.Max(minimum, domainDuration*math.Exp(float64(ev.DeltaY)*0.0015)))
	if nextDuration >= fullDuration*0.999 {
		s.viewport = nil
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	fraction := clampLedger(float64((x-r.X)/r.Width), 0, 1)
	anchor := domain.Start + fraction*domainDuration
	start := clampLedger(anchor-fraction*nextDuration, full.Start, math.Max(full.Start, full.End-nextDuration))
	s.viewport = &ledgerRange{Start: start, End: start + nextDuration}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// HandleKey 只消费确实存在的 timeline 选择；其他 Esc 仍交给对话/审批。
func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	if ev.Key != gfx.KeyEscape || ev.Modifiers != 0 {
		return false
	}
	s := p.ledgerStates[p.ledgerSession]
	if s == nil || s.selection == nil {
		return false
	}
	s.selection = nil
	return true
}

var _ pubapi.ScrollHandler = (*Plugin)(nil)
var _ pluginapi.KeyHandler = (*Plugin)(nil)
