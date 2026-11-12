package dsrightbar

import (
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Fields 让壳把两个 Trajectory 搜索框登记给真实文本输入系统。主区与右栏可以
// 同屏出现，因此它们不是同一个 flex.Field；查询是否一致不影响同一份 Snapshot。
func (p *Plugin) Fields() []*ui.Field {
	return []*ui.Field{&p.searchField, &p.mainSearchField}
}

func (p *Plugin) initWPD1() {
	if p.stepOpen == nil {
		p.stepOpen = map[string]bool{}
	}
	if p.selectedTurn == nil {
		p.selectedTurn = map[string]string{}
	}
	if p.selectedStep == nil {
		p.selectedStep = map[string]string{}
	}
	if p.lastSearch == nil {
		p.lastSearch = map[string]string{}
	}
	if p.turnOpen == nil {
		p.turnOpen = map[string]bool{}
	}
}

func (p *Plugin) trajectoryField(scrollKey string) *ui.Field {
	if strings.HasPrefix(scrollKey, "main-") {
		return &p.mainSearchField
	}
	return &p.searchField
}

// selectionState 是同包交互测试读取的真实选择，不是另一份测试模型。
func (p *Plugin) selectionState(session string) (turnKey, stepKey string) {
	return p.selectedTurn[session], p.selectedStep[session]
}

// drawTrajectoryWorkspace 在既有 A-TRAJ 分组之上增加当前窗口搜索、选择、Inspect
// 和可见行虚拟化。它只读 Snapshot；没有 loadOlder、完整工具树或耗时伪投影。
func (p *Plugin) drawTrajectoryWorkspace(f *flex.Frame, env *pluginapi.Env, data pluginapi.Snapshot, scrollKey string, groups []turnGroup, total int) {
	p.initWPD1()
	t := p.theme
	field := p.trajectoryField(scrollKey)
	rawQuery := field.Text()
	query := strings.TrimSpace(rawQuery)
	filtered, matches := filterTrajectoryGroups(data, groups, query)

	f.Row().WidthPercent(100).MinHeight(30).Gap(6).Items(ui.Center).Children(func() {
		f.Field(field, env.Tree.ID(env.Feat, "trajectory-search-"+scrollKey+data.Cur), "搜索当前窗口", "Turn / Step / 工具 / 参数 / 结果 / 错误").
			Grow(1).FontSize(t.Fonts.Caption)
		if query != "" {
			f.Text(itoa(matches) + " / " + itoa(total) + " Step").FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		}
		if rawQuery != "" {
			if f.Button(env.Tree.ID(env.Feat, "trajectory-search-clear-"+scrollKey+data.Cur), "清除轨迹搜索").Ghost().Small().Clicked() {
				field.SetText("")
			}
		}
	})
	f.Row().WidthPercent(100).MinHeight(18).Items(ui.Center).Children(func() {
		f.Text("仅当前已加载窗口").FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
	})

	if total == 0 {
		msg := "本会话还没有工具调用"
		if data.Cur == "" {
			msg = "先选一个会话"
		}
		f.Column().WidthPercent(100).Grow(1).Justify(ui.Center).Items(ui.Center).PadX(12).Children(func() {
			f.Text(msg).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).
				Color(t.LabelTertiary).TextCenter()
		})
		return
	}
	if query != "" && matches == 0 {
		f.Column().WidthPercent(100).Grow(1).Justify(ui.Center).Items(ui.Center).PadX(12).Children(func() {
			f.Text("没有匹配「" + query + "」的 Step").FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).
				Color(t.LabelTertiary).TextCenter()
		})
		return
	}

	shown, hidden := clipGroups(filtered, maxEntries)
	scrID := env.Tree.ID(env.Feat, scrollKey+data.Cur)
	var scrollState *ui.ScrollState
	if env.Scroll != nil {
		scrollState = env.Scroll(scrID)
	}
	searchScope := data.Cur + "\x00" + scrollKey
	if previous, ok := p.lastSearch[searchScope]; !ok || previous != query {
		p.lastSearch[searchScope] = query
		if scrollState != nil {
			scrollState.ScrollTo(0)
		}
	}
	rows := p.buildTrajectoryRows(data, shown, hidden, query)
	viewport := float32(0)
	offset := float32(0)
	if scrollState != nil {
		viewport, offset = scrollState.Viewport, scrollState.Offset
	}
	if viewport <= 0 {
		viewport = f.Bounds().Height - 150
		if viewport < 120 {
			viewport = 120
		}
	}
	const overscan = 84.0 // 两条收起 Step；不把整批 80/1000 条都建为文本节点。

	f.ScrollArea(scrID).WidthPercent(100).Grow(1).MinHeight(80).Gap(2).Clip().Children(func() {
		f.Column().WidthPercent(100).Gap(2).Children(func() {
			top := float32(0)
			for i := range rows {
				row := &rows[i]
				if top+row.height < offset-overscan || top > offset+viewport+overscan {
					f.Slot().Height(row.height)
				} else {
					p.drawTrajectoryRow(f, env, data, scrollKey, query, *row)
				}
				top += row.height + 2
			}
		})
	})
}

// filterTrajectoryGroups 对当前已加载的全部 Step 做大小写不敏感包含查询。
// TrimSpace 后的空查询恢复全部；Turn/Turn error 命中时保留该 Turn 的全部 Step，
// Step/工具/参数/结果命中时只保留对应 Step。最近 80 条展示裁剪发生在搜索之后。
func filterTrajectoryGroups(data pluginapi.Snapshot, groups []turnGroup, query string) ([]turnGroup, int) {
	total := 0
	for i := range groups {
		total += len(groups[i].calls)
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return groups, total
	}
	out := make([]turnGroup, 0, len(groups))
	matches := 0
	for gi := range groups {
		g := groups[gi]
		originalTotal := g.totalCalls
		if originalTotal == 0 {
			originalTotal = len(g.calls)
		}
		filtered := g
		filtered.totalCalls = originalTotal
		filtered.skip = 0
		if strings.Contains(trajectoryTurnSearchText(data, g), q) {
			filtered.calls = append([]apiclient.Message(nil), g.calls...)
			filtered.indices = append([]int(nil), g.indices...)
			matches += len(filtered.calls)
			out = append(out, filtered)
			continue
		}
		filtered.calls = nil
		filtered.indices = nil
		for i, msg := range g.calls {
			index := trajectoryMessageIndex(g, i)
			if strings.Contains(trajectoryStepSearchText(data, msg, index), q) {
				filtered.calls = append(filtered.calls, msg)
				filtered.indices = append(filtered.indices, index)
			}
		}
		if len(filtered.calls) > 0 {
			matches += len(filtered.calls)
			out = append(out, filtered)
		}
	}
	return out, matches
}

func trajectoryTurnSearchText(data pluginapi.Snapshot, g turnGroup) string {
	var b strings.Builder
	writeTrajectorySearch(&b, "第 "+itoa(g.turn)+" 轮", "Turn "+itoa(g.turn), g.hostKey)
	for _, msg := range g.calls {
		writeTrajectorySearch(&b, msg.TurnID)
		if msg.Turn != nil {
			writeTrajectorySearch(&b, "Turn "+strconv.FormatInt(*msg.Turn, 10), strconv.FormatInt(*msg.Turn, 10))
		}
	}
	for _, event := range data.TurnEvents {
		if !trajectoryEventInTurn(event, g) {
			continue
		}
		writeTrajectorySearch(&b, event.TurnID, event.Kind, event.Error)
		if event.Turn != nil {
			writeTrajectorySearch(&b, strconv.FormatInt(*event.Turn, 10))
		}
	}
	return strings.ToLower(b.String())
}

func trajectoryEventInTurn(event pubapi.TurnEvent, g turnGroup) bool {
	for _, msg := range g.calls {
		if event.TurnID != "" && event.TurnID == msg.TurnID {
			return true
		}
		if event.Turn != nil && msg.Turn != nil && *event.Turn == *msg.Turn {
			return true
		}
	}
	return false
}

func trajectoryStepSearchText(data pluginapi.Snapshot, msg apiclient.Message, index int) string {
	var b strings.Builder
	writeTrajectorySearch(&b, msg.Role, msg.Tool, msg.Args, msg.Summary, msg.Text, msg.CallID, msg.TurnID, msg.MessageID)
	if msg.Step != nil {
		writeTrajectorySearch(&b, "Step "+strconv.FormatInt(*msg.Step, 10), strconv.FormatInt(*msg.Step, 10))
	}
	if msg.Turn != nil {
		writeTrajectorySearch(&b, "Turn "+strconv.FormatInt(*msg.Turn, 10), strconv.FormatInt(*msg.Turn, 10))
	}
	if msg.View != nil {
		v := msg.View
		writeTrajectorySearch(&b, v.Kind, v.Title, v.Badge, v.Body)
		for _, field := range v.Fields {
			writeTrajectorySearch(&b, field.Key, field.Value)
		}
		writeTrajectorySearch(&b, v.Columns...)
		for _, row := range v.Rows {
			writeTrajectorySearch(&b, row...)
		}
	}
	if meta, ok := data.MessageMeta[pubapi.MessageKey(data.Cur, msg, index)]; ok && meta.Time > 0 {
		writeTrajectorySearch(&b, strconv.FormatInt(meta.Time, 10))
	}
	return strings.ToLower(b.String())
}

func writeTrajectorySearch(b *strings.Builder, values ...string) {
	for _, value := range values {
		if value == "" {
			continue
		}
		b.WriteString(value)
		b.WriteByte('\x00')
	}
}

func trajectoryMessageIndex(g turnGroup, callIndex int) int {
	if callIndex >= 0 && callIndex < len(g.indices) {
		return g.indices[callIndex]
	}
	return callIndex
}

func trajectoryRecordKey(session string, g turnGroup, callIndex int) string {
	return pubapi.MessageKey(session, g.calls[callIndex], trajectoryMessageIndex(g, callIndex))
}

type trajectoryRowKind uint8

const (
	trajectoryGlobalSkip trajectoryRowKind = iota
	trajectoryTurnRow
	trajectoryGroupSkip
	trajectoryStepRow
	trajectoryInspectRow
)

type trajectoryRow struct {
	kind       trajectoryRowKind
	group      turnGroup
	groupLast  bool
	callIndex  int
	lastStep   bool
	hidden     int
	height     float32
	inspect    []trajectoryInspectField
	selection  string
	messageKey string
}

const (
	trajectoryTurnHeight = 26.0
	trajectorySkipHeight = 20.0
	trajectoryStepHeight = 44.0
)

func (p *Plugin) buildTrajectoryRows(data pluginapi.Snapshot, groups []turnGroup, hidden int, query string) []trajectoryRow {
	rows := make([]trajectoryRow, 0, maxEntries+len(groups)*2+4)
	if hidden > 0 {
		rows = append(rows, trajectoryRow{kind: trajectoryGlobalSkip, hidden: hidden, height: trajectorySkipHeight})
	}
	for gi := range groups {
		g := groups[gi]
		lastGroup := gi == len(groups)-1
		rows = append(rows, trajectoryRow{kind: trajectoryTurnRow, group: g, groupLast: lastGroup, height: trajectoryTurnHeight})
		if !p.groupOpen(data.Cur, g.key, lastGroup) {
			continue
		}
		if g.skip > 0 {
			rows = append(rows, trajectoryRow{kind: trajectoryGroupSkip, group: g, hidden: g.skip, height: trajectorySkipHeight})
		}
		for i := g.skip; i < len(g.calls); i++ {
			key := trajectoryRecordKey(data.Cur, g, i)
			rows = append(rows, trajectoryRow{
				kind: trajectoryStepRow, group: g, groupLast: lastGroup, callIndex: i,
				lastStep: i == len(g.calls)-1, height: trajectoryStepHeight, messageKey: key,
			})
			if p.stepOpen[key] {
				fields := trajectoryInspectFields(data, g.calls[i], trajectoryMessageIndex(g, i))
				rows = append(rows, trajectoryRow{
					kind: trajectoryInspectRow, group: g, callIndex: i, messageKey: key,
					height: estimateTrajectoryInspectHeight(fields), inspect: fields,
				})
			}
		}
	}
	_ = query // 查询只影响过滤/文案；行身份始终来自原始消息。
	return rows
}

func (p *Plugin) drawTrajectoryRow(f *flex.Frame, env *pluginapi.Env, data pluginapi.Snapshot, scope, query string, row trajectoryRow) {
	t := p.theme
	switch row.kind {
	case trajectoryGlobalSkip:
		text := "… 更早的 " + itoa(row.hidden) + " 次未显示"
		if query != "" {
			text = "… 更早的 " + itoa(row.hidden) + " 个匹配 Step 未显示"
		}
		f.Text(text).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).PadY(2)
	case trajectoryGroupSkip:
		f.Text("… 前 " + itoa(row.hidden) + " 次未显示").FontSize(t.Fonts.Tiny).Color(t.LabelCaption).PadY(2)
	case trajectoryTurnRow:
		p.drawTrajectoryTurnRow(f, env, data, scope, row.group, row.groupLast, query)
	case trajectoryStepRow:
		p.drawTrajectoryStepRow(f, env, data, scope, row.group, row.callIndex, row.lastStep)
	case trajectoryInspectRow:
		p.drawTrajectoryInspector(f, row.inspect)
	}
}

func (p *Plugin) drawTrajectoryTurnRow(f *flex.Frame, env *pluginapi.Env, data pluginapi.Snapshot, scope string, g turnGroup, last bool, query string) {
	t := p.theme
	open := p.groupOpen(data.Cur, g.key, last)
	total := g.totalCalls
	if total == 0 {
		total = len(g.calls)
	}
	title := "第 " + itoa(g.turn) + " 轮 · " + itoa(len(g.calls)) + " 次工具"
	if query != "" && len(g.calls) != total {
		title = "第 " + itoa(g.turn) + " 轮 · " + itoa(len(g.calls)) + "/" + itoa(total) + " 次工具"
	}
	selected := p.selectedTurn[data.Cur] == g.key
	header := f.Row().WidthPercent(100).MinHeight(24).Radius(t.Radii.SM).PadX(2).
		Items(ui.Center).Gap(6).Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, scope+"turn-"+g.key), title)
	if selected {
		header = header.Background(t.BgLayer2)
	}
	header.Children(func() {
		arrow := "▸"
		if open {
			arrow = "▾"
		}
		f.Text(arrow).FontSize(t.Fonts.Caption).Color(t.LabelCaption)
		f.Text(title).FontSize(t.Fonts.Caption).Color(t.LabelSecondary).Ellipsis().Grow(1)
	})
	if header.Clicked() {
		p.toggleGroup(data.Cur, g.key, last)
		p.selectedTurn[data.Cur] = g.key
		if len(g.calls) > 0 {
			p.selectedStep[data.Cur] = trajectoryRecordKey(data.Cur, g, 0)
		} else {
			p.selectedStep[data.Cur] = ""
		}
	}
}

func (p *Plugin) drawTrajectoryStepRow(f *flex.Frame, env *pluginapi.Env, data pluginapi.Snapshot, scope string, g turnGroup, callIndex int, last bool) {
	t := p.theme
	msg := g.calls[callIndex]
	name, detail, failed := describe(msg)
	key := trajectoryRecordKey(data.Cur, g, callIndex)
	open := p.stepOpen[key]
	label := "Turn " + itoa(g.turn) + " · "
	if msg.Step != nil {
		label += "Step " + strconv.FormatInt(*msg.Step, 10) + " · "
	}
	label += name
	row := f.Row().WidthPercent(100).Gap(8).PadY(4).Radius(t.Radii.SM).
		Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, scope+"step-"+key), label)
	if p.selectedStep[data.Cur] == key {
		row = row.Background(t.BgLayer2)
	}
	row.Children(func() {
		f.Column().Width(14).Items(ui.Center).Children(func() {
			p.prims.Dot(f, dotColor(t, failed, msg.View), 6)
			if !last {
				f.Column().Width(2).Grow(1).Background(t.BorderL2)
			}
		})
		f.Column().Grow(1).Gap(2).Children(func() {
			f.Row().WidthPercent(100).Gap(6).Items(ui.Center).Children(func() {
				arrow := "▸"
				if open {
					arrow = "▾"
				}
				f.Text(arrow).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
				f.Text(name).Mono().FontSize(t.Fonts.Caption).Color(t.LabelSecondary).Ellipsis().Grow(1)
				if msg.Step != nil {
					f.Text("Step " + strconv.FormatInt(*msg.Step, 10)).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
				}
				if failed {
					p.prims.Badge(f, "error", t.Error)
				}
			})
			if s := oneLine(detail, 72); s != "" {
				f.Text(s).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).Ellipsis()
			}
		})
	})
	if row.Clicked() {
		p.selectedTurn[data.Cur] = g.key
		p.selectedStep[data.Cur] = key
		p.stepOpen[key] = !open
	}
}

type trajectoryInspectField struct {
	name  string
	value string
}

// trajectoryInspectFields 只暴露当前共享数据里确实存在的原值。当前 Message 没有
// parentCallId/isError/error 字段，因此这些标签整段隐藏；View.Badge 不冒充 isError，
// TurnEvents 的 Turn 级 error 也不冒充某一个 Step 的 error。
func trajectoryInspectFields(data pluginapi.Snapshot, msg apiclient.Message, index int) []trajectoryInspectField {
	fields := make([]trajectoryInspectField, 0, 8)
	add := func(name, value string) {
		if value != "" {
			fields = append(fields, trajectoryInspectField{name: name, value: value})
		}
	}
	if msg.Step != nil {
		add("step", strconv.FormatInt(*msg.Step, 10))
	}
	add("turnId", msg.TurnID)
	if msg.Turn != nil {
		add("turn", strconv.FormatInt(*msg.Turn, 10))
	}
	add("args", msg.Args)
	result := msg.Summary
	if result == "" {
		result = msg.Text
	}
	add("result", result)
	add("callId", msg.CallID)
	if meta, ok := data.MessageMeta[pubapi.MessageKey(data.Cur, msg, index)]; ok && meta.Time > 0 {
		add("time", strconv.FormatInt(meta.Time, 10))
	}
	if msg.View != nil {
		add("view.body", msg.View.Body)
	}
	return fields
}

func (p *Plugin) drawTrajectoryInspector(f *flex.Frame, fields []trajectoryInspectField) {
	t := p.theme
	f.Column().WidthPercent(100).PadX(22).PadY(4).Gap(4).Children(func() {
		f.Text("Inspect").FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
		for _, field := range fields {
			f.Text(field.name).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			f.Wrap(field.value).Mono().FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
		}
	})
}

func estimateTrajectoryInspectHeight(fields []trajectoryInspectField) float32 {
	height := float32(28)
	for _, field := range fields {
		height += 16 + float32(estimateTrajectoryLines(field.value, 48))*14 + 6
	}
	return height + 8
}

func estimateTrajectoryLines(value string, widthRunes int) int {
	if widthRunes < 1 {
		widthRunes = 1
	}
	lines := 0
	for _, part := range strings.Split(value, "\n") {
		n := (len([]rune(part)) + widthRunes - 1) / widthRunes
		if n < 1 {
			n = 1
		}
		lines += n
	}
	if lines < 1 {
		return 1
	}
	return lines
}
