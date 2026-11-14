// stats_view.go —— 官方 rc.2 `dsh-experimental-client-ui-stats` 的会话「统计」视图。
//
// # 它是什么
//
// 官方包只做一件事：往 `conversation.view` 注册一个 `{id:'stats', order:40}`
// 的视图（`OFF:dsh-experimental-client-ui-stats/src/client/index.ts:74-81`），
// 页面本体是一张**逐轮 / 逐 step 的性能表**：上下文、输出、缓存命中、首 token、
// 模型用时、输出速度、工具用时、总用时条，外加一行全量/窗口合计。
//
// # 数据从哪来（不另建采集层）
//
// 官方 `stats-fold.ts` 是对 Session 事件窗口做纯投影：step/start 记开始，
// assistant/text|reasoning 记首 token，assistant/message 记 message 时间与 usage，
// step/end 记结束，tool/call 与 tool/result 配对算工具耗时，turn/start|end 定轮界。
// 本仓 `pluginapi.Snapshot.Frames` 就是同一份原始 journal（`session.history` 的
// frames 与 WS 推送同形），所以这里逐条照抄同一套折叠，**不读 Msgs 凑数**。
//
// 合计行的「全量」值只用宿主 whole-log 投影 `sessionStats` + `tokenUsage`
// （`Snapshot.SessionStats` / `TokenUsage`）；两者缺一时退回「本窗口合计」并在
// 标题里如实写出来——绝不把 tail 窗口冒充全量。
//
// # 与官方的差距（如实登记，不假装）
//
//   - Code-Mode 内层 dispatch 的合并、compaction 事务行、非 step 事件行
//     （system-prompt / tool-set / retry / approval / prune…）没有画：本仓
//     `Snapshot.Frames` 里有这些帧，但这一版只做官方默认行（turns + steps + 合计）。
//   - 官方工具栏（context delta / show events / context basis 三个开关）没有；
//     默认口径 = 官方默认：轮行取最后一步的上下文（latest）、事件行隐藏。
//   - 官方 `useProjection` 的 whole-session 合计在本仓有等价投影；窗口合计由本文件
//     折叠出来，二者不混用。
package dsstatusbar

import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 统计表列宽逐值来自官方 `stats.module.css:49-59`（11 列，合计 100%）。
const (
	statsColScopePct  float32 = 13
	statsColPromptPct float32 = 15
	statsColDidPct    float32 = 13.5
	statsColCtxPct    float32 = 5.5
	statsColOutPct    float32 = 5.5
	statsColCachePct  float32 = 6.5
	statsColTTFTPct   float32 = 6
	statsColModelPct  float32 = 6
	statsColTPSPct    float32 = 7.5
	statsColToolPct   float32 = 6
	statsColTimePct   float32 = 15.5

	statsRowHeight     float32 = 30
	statsTurnRowHeight float32 = 34
	statsHeaderHeight  float32 = 26
	statsTablePadX     float32 = 8
	statsScrollPadX    float32 = 20
	// `.root { padding-bottom: calc(var(--dsh-composer-height,152px) + 8px) }`
	statsScrollPadBottom float32 = 160
	statsBarTurnHeight   float32 = 14
	statsBarStepHeight   float32 = 6
	statsBarRadius       float32 = 2
	statsStepIndentLeft  float32 = 22
)

// 官方 `stats-format.ts` 的占位符：没有记录就是破折号，不是 0。
const statsDash = "—"

// ── wire 形状 ──────────────────────────────────────────────────────────────

// statsUsage 是 assistant/message 的 usage（四个桶都可能缺席，指针保真）。
type statsUsage struct {
	InputTokens      *int64 `json:"inputTokens"`
	OutputTokens     *int64 `json:"outputTokens"`
	CacheReadTokens  *int64 `json:"cacheReadTokens"`
	CacheWriteTokens *int64 `json:"cacheWriteTokens"`
}

// statsEventData 是统计折叠认的字段并集；其余字段一律忽略。
type statsEventData struct {
	Turn    *int64      `json:"turn"`
	Step    *int64      `json:"step"`
	Usage   *statsUsage `json:"usage"`
	CallID  string      `json:"callId"`
	Name    string      `json:"name"`
	Text    string      `json:"text"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Message *struct {
		Role       string `json:"role"`
		ToolCallID string `json:"toolCallId"`
		IsError    bool   `json:"isError"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
	Source *struct {
		Kind string `json:"kind"`
	} `json:"source"`
	Reason *struct {
		Kind string `json:"kind"`
	} `json:"reason"`
}

// statsCall 是一次工具调用的配对状态（只留统计需要的 time/name/error）。
type statsCall struct {
	CallID    string
	Name      string
	StartTime int64
	EndTime   int64
	HasStart  bool
	HasEnd    bool
	Error     bool
}

// statsStep 是一个 step/start..step/end 之间的折叠状态。
type statsStep struct {
	Turn int64
	Step int64

	StartTime      int64
	EndTime        int64
	FirstTokenTime int64
	MessageTime    int64
	HasStart       bool
	HasEnd         bool
	HasFirstToken  bool
	HasMessage     bool

	Usage       *statsUsage
	Calls       []*statsCall
	Answer      string
	Interrupted bool
}

// statsTurn 是一轮及其 steps（按 step 号升序）。
type statsTurn struct {
	Turn         int64
	Steps        []*statsStep
	Prompt       string
	PromptSource string
}

// statsModel 是整张表的折叠结果。
type statsModel struct {
	Turns []*statsTurn
}

// statsByKey 找/建一轮；turn 号缺失时沿用上一轮，仍没有就按出现顺序补。
func (m *statsModel) turn(n *int64) *statsTurn {
	if n != nil && *n > 0 {
		for _, t := range m.Turns {
			if t.Turn == *n {
				return t
			}
		}
		t := &statsTurn{Turn: *n}
		m.Turns = append(m.Turns, t)
		return t
	}
	if len(m.Turns) > 0 {
		return m.Turns[len(m.Turns)-1]
	}
	t := &statsTurn{Turn: 1}
	m.Turns = append(m.Turns, t)
	return t
}

// step 找/建一个 step。step 号缺失时沿用同轮最后一个，仍没有记 1。
func (t *statsTurn) step(n *int64) *statsStep {
	if n != nil && *n > 0 {
		for _, s := range t.Steps {
			if s.Step == *n {
				return s
			}
		}
		s := &statsStep{Turn: t.Turn, Step: *n}
		t.Steps = append(t.Steps, s)
		return s
	}
	if len(t.Steps) > 0 {
		return t.Steps[len(t.Steps)-1]
	}
	s := &statsStep{Turn: t.Turn, Step: 1}
	t.Steps = append(t.Steps, s)
	return s
}

// foldStats 把原始帧折成统计模型（官方 stats-fold.ts 的逐事件移植）。
func foldStats(frames []apiclient.Frame) *statsModel {
	m := &statsModel{}
	for _, fr := range frames {
		var d statsEventData
		if len(fr.Data) > 0 {
			_ = json.Unmarshal(fr.Data, &d)
		}
		switch fr.Type {
		case "turn/start":
			m.turn(d.Turn)
		case "user/message":
			t := m.turn(d.Turn)
			if t.Prompt == "" {
				t.Prompt = eventText(d.Content)
			}
			if d.Source != nil {
				t.PromptSource = d.Source.Kind
			}
		case "step/start":
			t := m.turn(d.Turn)
			s := t.step(d.Step)
			if fr.Time > 0 {
				s.StartTime, s.HasStart = fr.Time, true
			}
		case "assistant/text", "assistant/reasoning":
			t := m.turn(d.Turn)
			s := t.step(d.Step)
			if fr.Time > 0 && !s.HasFirstToken {
				s.FirstTokenTime, s.HasFirstToken = fr.Time, true
			}
		case "assistant/message":
			t := m.turn(d.Turn)
			s := t.step(d.Step)
			if fr.Time > 0 {
				s.MessageTime, s.HasMessage = fr.Time, true
			}
			s.Usage = d.Usage
			if d.Message != nil {
				s.Answer = eventText(d.Message.Content)
			}
		case "step/end":
			t := m.turn(d.Turn)
			s := t.step(d.Step)
			if fr.Time > 0 {
				s.EndTime, s.HasEnd = fr.Time, true
			}
		case "tool/call":
			t := m.turn(d.Turn)
			s := t.step(d.Step)
			s.Calls = append(s.Calls, &statsCall{
				CallID: d.CallID, Name: d.Name,
				StartTime: fr.Time, HasStart: fr.Time > 0,
			})
		case "tool/result":
			t := m.turn(d.Turn)
			s := t.step(d.Step)
			id := d.CallID
			error := false
			if d.Message != nil {
				if id == "" {
					id = d.Message.ToolCallID
				}
				error = d.Message.IsError
			}
			var call *statsCall
			for i := len(s.Calls) - 1; i >= 0; i-- {
				if s.Calls[i].CallID == id && !s.Calls[i].HasEnd {
					call = s.Calls[i]
					break
				}
			}
			if call == nil {
				call = &statsCall{CallID: id}
				s.Calls = append(s.Calls, call)
			}
			if fr.Time > 0 {
				call.EndTime, call.HasEnd = fr.Time, true
			}
			call.Error = call.Error || error
		case "turn/end":
			t := m.turn(d.Turn)
			kind := ""
			if d.Reason != nil {
				kind = d.Reason.Kind
			}
			if kind == "interrupted" || kind == "aborted" || kind == "error" {
				if len(t.Steps) > 0 {
					t.Steps[len(t.Steps)-1].Interrupted = true
				}
			}
		}
	}
	// 同轮内按 step 号升序（事件顺序理论上有序，但窗口重建可能乱序）。
	for _, t := range m.Turns {
		sort.SliceStable(t.Steps, func(i, j int) bool { return t.Steps[i].Step < t.Steps[j].Step })
	}
	sort.SliceStable(m.Turns, func(i, j int) bool { return m.Turns[i].Turn < m.Turns[j].Turn })
	return m
}

// eventText 拼一段 content 里的 text 块（保持原顺序）。
func eventText(blocks []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) string {
	var b strings.Builder
	for _, block := range blocks {
		if block.Type == "" || block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

// ── 指标（official stats-usage.ts / stats-fold.ts 的同名公式）─────────────────

// usageReported 报告 usage 是否至少有一个桶（官方 reported()）。
func usageReported(u *statsUsage) bool {
	return u != nil && (u.InputTokens != nil || u.OutputTokens != nil ||
		u.CacheReadTokens != nil || u.CacheWriteTokens != nil)
}

func tokenOrNil(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// contextTokens = 输入 + 缓存读 + 缓存写（三项不重叠）。
func contextTokens(u *statsUsage) (int64, bool) {
	if !usageReported(u) {
		return 0, false
	}
	return tokenOrNil(u.InputTokens) + tokenOrNil(u.CacheReadTokens) + tokenOrNil(u.CacheWriteTokens), true
}

func outputTokens(u *statsUsage) (int64, bool) {
	if !usageReported(u) || u.OutputTokens == nil {
		return 0, false
	}
	return *u.OutputTokens, true
}

func cacheReadTokens(u *statsUsage) (int64, bool) {
	if u == nil || u.CacheReadTokens == nil {
		return 0, false
	}
	return *u.CacheReadTokens, true
}

func cacheWriteTokens(u *statsUsage) (int64, bool) {
	if u == nil || u.CacheWriteTokens == nil {
		return 0, false
	}
	return *u.CacheWriteTokens, true
}

// cacheHitRate = 缓存读 ÷ context（官方 cacheHitRate，夹到 [0,1]）。
func cacheHitRate(u *statsUsage) (float64, bool) {
	ctx, ok := contextTokens(u)
	if !ok || ctx <= 0 {
		return 0, false
	}
	read, ok := cacheReadTokens(u)
	if !ok {
		return 0, false
	}
	return math.Min(1, float64(read)/float64(ctx)), true
}

// stepMetrics 是官方 `stepMetrics` 的 Go 版：所有时间都只在两端都有真值时可算。
type stepMetrics struct {
	Turn        int64
	Step        int64
	Context     int64
	HasCtx      bool
	Output      int64
	HasOutput   bool
	CacheHit    float64
	HasCache    bool
	TTFT        int64
	HasTTFT     bool
	Model       int64
	HasModel    bool
	Decode      int64
	HasDecode   bool
	TPS         float64
	HasTPS      bool
	Tool        int64
	HasTool     bool
	Wall        int64
	HasWall     bool
	Tools       []toolTally
	Calls       []*statsCall
	Answer      string
	Running     bool
	Interrupted bool
}

type toolTally struct {
	Name  string
	Count int
}

func measureStep(s *statsStep) stepMetrics {
	m := stepMetrics{Turn: s.Turn, Step: s.Step}
	if ctx, ok := contextTokens(s.Usage); ok {
		m.Context, m.HasCtx = ctx, true
	}
	if out, ok := outputTokens(s.Usage); ok {
		m.Output, m.HasOutput = out, true
	}
	if rate, ok := cacheHitRate(s.Usage); ok {
		m.CacheHit, m.HasCache = rate, true
	}
	if s.HasStart && s.HasFirstToken && s.FirstTokenTime >= s.StartTime {
		m.TTFT, m.HasTTFT = s.FirstTokenTime-s.StartTime, true
	}
	if s.HasStart && s.HasMessage && s.MessageTime >= s.StartTime {
		m.Model, m.HasModel = s.MessageTime-s.StartTime, true
	}
	if s.HasFirstToken && s.HasMessage && s.MessageTime >= s.FirstTokenTime {
		m.Decode, m.HasDecode = s.MessageTime-s.FirstTokenTime, true
	}
	if m.HasDecode && m.HasOutput && m.Decode > 0 && m.Output > 0 {
		m.TPS, m.HasTPS = float64(m.Output)/(float64(m.Decode)/1000), true
	}
	if s.HasStart && s.HasEnd && s.EndTime >= s.StartTime {
		m.Wall, m.HasWall = s.EndTime-s.StartTime, true
	}
	paired := 0
	var toolSum int64
	counts := map[string]int{}
	for _, c := range s.Calls {
		if c.Name != "" {
			counts[c.Name]++
		}
		if c.HasStart && c.HasEnd && c.EndTime >= c.StartTime {
			toolSum += c.EndTime - c.StartTime
			paired++
		}
	}
	if paired > 0 {
		m.Tool, m.HasTool = toolSum, true
	}
	for name, count := range counts {
		m.Tools = append(m.Tools, toolTally{Name: name, Count: count})
	}
	sort.Slice(m.Tools, func(i, j int) bool {
		if m.Tools[i].Count != m.Tools[j].Count {
			return m.Tools[i].Count > m.Tools[j].Count
		}
		return m.Tools[i].Name < m.Tools[j].Name
	})
	m.Calls = append([]*statsCall(nil), s.Calls...)
	m.Answer = s.Answer
	m.Running = !s.HasEnd
	m.Interrupted = s.Interrupted
	return m
}

// turnMetrics 是官方 `aggregateTurn` 的 Go 版（只取表格要显示的字段）。
type turnMetrics struct {
	Turn         int64
	Prompt       string
	PromptSource string
	Steps        []stepMetrics
	Context      int64
	HasContext   bool
	Output       int64
	HasOutput    bool
	CacheHit     float64
	HasCache     bool
	TTFT         int64
	HasTTFT      bool
	Model        int64
	HasModel     bool
	TPS          float64
	HasTPS       bool
	Tool         int64
	HasTool      bool
	Wall         int64
	HasWall      bool
	Tools        []toolTally
	Running      bool
	Answer       string
}

func measureTurn(t *statsTurn) turnMetrics {
	tm := turnMetrics{Turn: t.Turn, Prompt: t.Prompt, PromptSource: t.PromptSource}
	var outSum, ttftSum, modelSum, decodeSum, toolSum int64
	var ctxN, outN, ttftN, modelN, decodeN, toolN int
	lastCtx := int64(0)
	hasLastCtx := false
	var cacheRead, cacheCtx int64
	var cacheMeasured bool
	tools := map[string]int{}
	for _, s := range t.Steps {
		m := measureStep(s)
		tm.Steps = append(tm.Steps, m)
		if m.HasCtx {
			lastCtx, hasLastCtx = m.Context, true
			ctxN++
		}
		if m.HasOutput {
			outSum += m.Output
			outN++
		}
		if m.HasTTFT {
			ttftSum += m.TTFT
			ttftN++
		}
		if m.HasModel {
			modelSum += m.Model
			modelN++
		}
		if m.HasDecode {
			decodeSum += m.Decode
			decodeN++
		}
		if m.HasTool {
			toolSum += m.Tool
			toolN++
		}
		if read, ok := cacheReadTokens(s.Usage); ok {
			if ctx, ok2 := contextTokens(s.Usage); ok2 {
				cacheRead += read
				cacheCtx += ctx
				cacheMeasured = true
			}
		}
		for _, item := range m.Tools {
			tools[item.Name] += item.Count
		}
		if m.Answer != "" {
			tm.Answer = m.Answer
		}
	}
	if hasLastCtx {
		tm.Context, tm.HasContext = lastCtx, true
	}
	if outN > 0 {
		tm.Output, tm.HasOutput = outSum, true
	}
	if ttftN > 0 {
		tm.TTFT, tm.HasTTFT = ttftSum, true
	}
	if modelN > 0 {
		tm.Model, tm.HasModel = modelSum, true
	}
	if decodeN > 0 && decodeSum > 0 && outSum > 0 {
		tm.TPS, tm.HasTPS = float64(outSum)/(float64(decodeSum)/1000), true
	}
	if toolN > 0 {
		tm.Tool, tm.HasTool = toolSum, true
	}
	if cacheMeasured && cacheCtx > 0 {
		tm.CacheHit, tm.HasCache = math.Min(1, float64(cacheRead)/float64(cacheCtx)), true
	}
	if len(t.Steps) > 0 {
		first, last := t.Steps[0], t.Steps[len(t.Steps)-1]
		if first.HasStart && last.HasEnd && last.EndTime >= first.StartTime {
			tm.Wall, tm.HasWall = last.EndTime-first.StartTime, true
		}
		tm.Running = !last.HasEnd
	}
	for name, count := range tools {
		tm.Tools = append(tm.Tools, toolTally{Name: name, Count: count})
	}
	sort.Slice(tm.Tools, func(i, j int) bool {
		if tm.Tools[i].Count != tm.Tools[j].Count {
			return tm.Tools[i].Count > tm.Tools[j].Count
		}
		return tm.Tools[i].Name < tm.Tools[j].Name
	})
	return tm
}

// ── 格式化（official stats-format.ts 逐函数移植）────────────────────────────

func statsFormatMs(v int64, ok bool) string {
	if !ok {
		return statsDash
	}
	switch {
	case v >= 60_000:
		return statsFixed(float64(v)/60_000, 1) + " min"
	case v >= 1_000:
		return statsFixed(float64(v)/1_000, 2) + " s"
	default:
		return itoa(int(math.Round(float64(v)))) + " ms"
	}
}

func statsFormatTokensShort(v int64, ok bool) string {
	if !ok {
		return statsDash
	}
	if v >= 10_000_000 {
		return statsFixed(float64(v)/1_000_000, 1) + "M"
	}
	if v >= 1_000 {
		return statsFixed(float64(v)/1_000, 1) + "K"
	}
	return statsFixed(float64(v)/1_000, 2) + "K"
}

func statsFormatTps(v float64, ok bool) string {
	if !ok {
		return statsDash
	}
	return statsFixed(v, 1) + " tok/s"
}

func statsFormatPercent(v float64, ok bool) string {
	if !ok {
		return statsDash
	}
	return statsFixed(v*100, 2) + "%"
}

// statsFixed 复刻 JS `Number.prototype.toFixed(digits)` 的**固定小数位数**：
// 官方 formatMs / formatTokensShort / formatPercent 都用它，整数也必须补零
// （`75.00%`、`1.00 s`、`0.50K`），不能走 compactDecimal 那种"整数不补 .0"。
func statsFixed(v float64, digits int) string {
	if math.IsNaN(v) || v < 0 {
		v = 0
	}
	scale := math.Pow10(digits)
	n := int64(math.Round(v * scale))
	unit := int64(scale)
	whole := n / unit
	if digits == 0 {
		return itoa64(whole)
	}
	frac := n % unit
	text := itoa64(frac)
	for len(text) < digits {
		text = "0" + text
	}
	return itoa64(whole) + "." + text
}

// statsToolSummary 是「执行内容」列的官方口径缩略：`名字 ×次数 · …`。
func statsToolSummary(tools []toolTally, answer string) string {
	if len(tools) == 0 {
		if answer != "" {
			return oneLineText(answer, 40)
		}
		return ""
	}
	parts := make([]string, 0, len(tools))
	for i, item := range tools {
		if i == 3 {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, item.Name+" ×"+itoa(item.Count))
	}
	return strings.Join(parts, " · ")
}

func oneLineText(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ── 绘制 ───────────────────────────────────────────────────────────────────

// statsOpenKey 是"这一轮是否展开"的稳定身份（会话 + 轮号）。
func statsOpenKey(session string, turn int64) string {
	return session + "/" + itoa64(turn)
}

// statsTurnOpen 是官方默认 `newest` 口径：只有最新一轮默认展开，用户点过以点击为准。
func (p *Plugin) statsTurnOpen(session string, turn int64, last bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.statsOpen == nil {
		p.statsOpen = map[string]bool{}
	}
	if open, ok := p.statsOpen[statsOpenKey(session, turn)]; ok {
		return open
	}
	return last
}

func (p *Plugin) toggleStatsTurn(session string, turn int64, last bool) {
	p.mu.Lock()
	if p.statsOpen == nil {
		p.statsOpen = map[string]bool{}
	}
	p.statsOpen[statsOpenKey(session, turn)] = !p.statsTurnOpenLocked(session, turn, last)
	p.mu.Unlock()
}

// statsTurnOpenLocked 在锁内重复判定（toggle 不能在持锁时再调带锁的公开方法）。
func (p *Plugin) statsTurnOpenLocked(session string, turn int64, last bool) bool {
	if open, ok := p.statsOpen[statsOpenKey(session, turn)]; ok {
		return open
	}
	return last
}

// drawStatsView 是 `conversation.view` 的 stats 条目。
func (p *Plugin) drawStatsView(f ui.C, env *pluginapi.Env) {
	if f == nil || env == nil || p.theme == nil {
		return
	}
	t := p.theme
	model := foldStats(env.Data.Frames)
	root := ui.Column(f).FillWidth().FillHeight()
	if len(model.Turns) == 0 {
		root.Children(func() {
			ui.Row(f).WidthPercent(100).PadSides(24, 20, 24, 20).Children(func() {
				ui.Text(f, "当前窗口里还没有可以测量的步骤。").FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
			})
		})
		return
	}
	turns := make([]turnMetrics, 0, len(model.Turns))
	for _, turn := range model.Turns {
		turns = append(turns, measureTurn(turn))
	}
	root.Children(func() {
		// 滚动区 padding 逐值来自官方 `.scroll { padding: 0 20px 16px }`。
		area := ui.Scroll(f.Key("stats-scroll-"+env.Data.Cur)).WidthPercent(100).Grow(1).Clip().
			PadSides(0, statsScrollPadX, statsScrollPadBottom, statsScrollPadX)
		area.Children(func() {
			ui.Column(f).WidthPercent(100).Children(func() {
				p.drawStatsHeader(f, t)
				last := len(turns) - 1
				var turnScale int64
				for _, turn := range turns {
					if turn.HasWall && turn.Wall > turnScale {
						turnScale = turn.Wall
					}
				}
				for i := range turns {
					p.drawStatsTurn(f, env, turns[i], i == last, turnScale)
				}
				p.drawStatsTotals(f, env, turns)
			})
		})
	})
}

// drawStatsHeader 是官方表头：11 列，左三列文本左对齐，数值列同（官方 CSS 也是 left）。
func (p *Plugin) drawStatsHeader(f ui.C, t *pluginapi.Theme) {
	ui.Row(f).WidthPercent(100).Height(statsHeaderHeight).Children(func() {
		for _, cell := range []struct {
			W    float32
			Text string
		}{
			{statsColScopePct, "轮次 / 步骤"},
			{statsColPromptPct, "触发"},
			{statsColDidPct, "执行内容"},
			{statsColCtxPct, "上下文"},
			{statsColOutPct, "输出"},
			{statsColCachePct, "缓存命中"},
			{statsColTTFTPct, "首 token"},
			{statsColModelPct, "模型用时"},
			{statsColTPSPct, "输出速度"},
			{statsColToolPct, "工具用时"},
			{statsColTimePct, "总用时"},
		} {
			ui.Row(f).WidthPercent(cell.W).Height(statsHeaderHeight).PadSides(4, statsTablePadX, 4, statsTablePadX).
				Items(ui.Center).Children(func() {
				ui.Text(f, cell.Text).FontSize(t.Fonts.Tiny).Color(t.LabelTertiary).Ellipsis()
			})
		}
	})
}

// drawStatsTurn 画一轮：轮行 + （展开时）逐 step 行。
func (p *Plugin) drawStatsTurn(f ui.C, env *pluginapi.Env, turn turnMetrics, last bool, turnScale int64) {
	t := p.theme
	open := p.statsTurnOpen(env.Data.Cur, turn.Turn, last)
	did := statsToolSummary(turn.Tools, turn.Answer)
	row := ui.Row(f).WidthPercent(100).Height(statsTurnRowHeight).Gap(0).
		Hit(p.statsHitID(env, "stats-turn-"+itoa64(turn.Turn)), "第 "+itoa64(turn.Turn)+" 轮").Children(func() {
		p.statsCell(f, statsColScopePct, "第 "+itoa64(turn.Turn)+" 轮", t.LabelPrimary, true)
		p.statsCell(f, statsColPromptPct, statsTurnTrigger(turn), t.LabelSecondary, false)
		p.statsCell(f, statsColDidPct, did, t.LabelSecondary, false)
		p.statsMetricCell(f, statsColCtxPct, statsFormatTokensShort(turn.Context, turn.HasContext))
		p.statsMetricCell(f, statsColOutPct, statsFormatTokensShort(turn.Output, turn.HasOutput))
		p.statsMetricCell(f, statsColCachePct, statsFormatPercent(turn.CacheHit, turn.HasCache))
		p.statsMetricCell(f, statsColTTFTPct, statsFormatMs(turn.TTFT, turn.HasTTFT))
		p.statsMetricCell(f, statsColModelPct, statsFormatMs(turn.Model, turn.HasModel))
		p.statsMetricCell(f, statsColTPSPct, statsFormatTps(turn.TPS, turn.HasTPS))
		p.statsMetricCell(f, statsColToolPct, statsFormatMs(turn.Tool, turn.HasTool))
		p.statsBarCell(f, statsColTimePct, statsBarTurnHeight, statsTimeBar{
			Wall: turn.Wall, HasWall: turn.HasWall,
			TTFT: turn.TTFT, HasTTFT: turn.HasTTFT,
			Model: turn.Model, HasModel: turn.HasModel,
			Tool: turn.Tool, HasTool: turn.HasTool,
			FillShare: statsBarShare(turn.Wall, turnScale), Running: turn.Running,
		})
	})
	if row.Clicked() {
		p.toggleStatsTurn(env.Data.Cur, turn.Turn, last)
		if env.Act.Invalidate != nil {
			env.Act.Invalidate()
		}
	}
	if !open {
		return
	}
	var stepScale int64
	for _, step := range turn.Steps {
		if step.HasWall && step.Wall > stepScale {
			stepScale = step.Wall
		}
	}
	for _, step := range turn.Steps {
		p.drawStatsStep(f, step, stepScale)
	}
}

func (p *Plugin) drawStatsStep(f ui.C, step stepMetrics, stepScale int64) {
	t := p.theme
	did := statsToolSummary(step.Tools, step.Answer)
	ui.Row(f).WidthPercent(100).Height(statsRowHeight).Children(func() {
		// `.stepCell` 左内边距 22 + inset 2px 线（官方 stats.module.css:173-181）。
		ui.Row(f).WidthPercent(statsColScopePct).Height(statsRowHeight).
			PadSides(0, statsTablePadX, 0, statsStepIndentLeft).Items(ui.Center).Children(func() {
			ui.Text(f, "第 "+itoa64(step.Step)+" 步").FontSize(t.Fonts.Tiny).Color(t.LabelSecondary).Ellipsis()
		})
		p.statsCell(f, statsColPromptPct, "", t.LabelSecondary, false)
		p.statsCell(f, statsColDidPct, did, t.LabelSecondary, false)
		p.statsMetricCell(f, statsColCtxPct, statsFormatTokensShort(step.Context, step.HasCtx))
		p.statsMetricCell(f, statsColOutPct, statsFormatTokensShort(step.Output, step.HasOutput))
		p.statsMetricCell(f, statsColCachePct, statsFormatPercent(step.CacheHit, step.HasCache))
		p.statsMetricCell(f, statsColTTFTPct, statsFormatMs(step.TTFT, step.HasTTFT))
		p.statsMetricCell(f, statsColModelPct, statsFormatMs(step.Model, step.HasModel))
		p.statsMetricCell(f, statsColTPSPct, statsFormatTps(step.TPS, step.HasTPS))
		p.statsMetricCell(f, statsColToolPct, statsFormatMs(step.Tool, step.HasTool))
		p.statsBarCell(f, statsColTimePct, statsBarStepHeight, statsTimeBar{
			Wall: step.Wall, HasWall: step.HasWall,
			TTFT: step.TTFT, HasTTFT: step.HasTTFT,
			Model: step.Model, HasModel: step.HasModel,
			Tool: step.Tool, HasTool: step.HasTool,
			FillShare: statsBarShare(step.Wall, stepScale), Running: step.Running,
		})
	})
}

// statsCell 画一个左对齐文本单元格（三列文本列共用）。
func (p *Plugin) statsCell(f ui.C, width float32, text string, color ui.Color, medium bool) {
	ui.Row(f).WidthPercent(width).Height(statsRowHeight).PadSides(0, statsTablePadX, 0, statsTablePadX).
		Items(ui.Center).Children(func() {
		txt := ui.Text(f, text).FontSize(p.theme.Fonts.Tiny).Color(color).Ellipsis()
		if medium {
			txt.Weight(gfx.WeightMedium)
		}
	})
}

// statsMetricCell 画一个数值列；没有真值就是破折号（不写 0）。
func (p *Plugin) statsMetricCell(f ui.C, width float32, text string) {
	ui.Row(f).WidthPercent(width).Height(statsRowHeight).PadSides(0, statsTablePadX, 0, statsTablePadX).
		Items(ui.Center).Children(func() {
		ui.Text(f, text).FontSize(p.theme.Fonts.Tiny).Color(p.theme.LabelSecondary).Ellipsis()
	})
}

// statsTimeBar 是一行总用时条需要的真值（缺项就不画那一段，不写 0 宽假段）。
type statsTimeBar struct {
	Wall      int64
	HasWall   bool
	TTFT      int64
	HasTTFT   bool
	Model     int64
	HasModel  bool
	Tool      int64
	HasTool   bool
	FillShare float32
	Running   bool
}

// statsBarShare 是官方 `barShare`：平方根刻度，组内最长者填满。
func statsBarShare(total, scale int64) float32 {
	if scale <= 0 {
		return 1
	}
	if total <= 0 {
		return 0
	}
	return float32(math.Min(1, math.Sqrt(float64(total)/float64(scale))))
}

// statsBarCell 画总用时条（官方 `.barTurn` 14px / `.barStep` 6px；段色见 CSS）。
//
// 颜色映射（官方 token → 本仓 Theme 的现有语义色）：
//
//	--dsw-alias-state-business-primary（TTFT） → Theme.Info
//	--dsw-alias-state-business-primary（decode）→ Theme.Link
//	--dsw-alias-state-warn-*（tool）           → Theme.Warn
//	--dsw-alias-bg-layer-2（其余）              → Theme.BgLayer2
func (p *Plugin) statsBarCell(f ui.C, width float32, height float32, bar statsTimeBar) {
	t := p.theme
	ui.Row(f).WidthPercent(width).Height(statsRowHeight).PadSides(0, statsTablePadX, 0, statsTablePadX).
		Items(ui.Center).Gap(6).Children(func() {
		track := ui.Row(f).Grow(1).Height(height).Radius(statsBarRadius).Background(t.BgLayer2).Clip()
		if bar.HasWall && bar.Wall > 0 && bar.FillShare > 0 {
			track.Children(func() {
				fill := ui.Row(f).WidthPercent(bar.FillShare * 100).Height(height)
				fill.Children(func() {
					remaining := float64(bar.Wall)
					add := func(v int64, ok bool, color ui.Color) {
						if !ok || v <= 0 || remaining <= 0 {
							return
						}
						share := math.Min(1, float64(v)/float64(bar.Wall))
						ui.Row(f).WidthPercent(float32(share * 100)).Height(height).Background(color)
						remaining -= float64(v)
					}
					add(bar.TTFT, bar.HasTTFT, t.Info)
					if bar.HasTTFT && bar.HasModel && bar.Model > bar.TTFT {
						add(bar.Model-bar.TTFT, true, t.Link)
					}
					add(bar.Tool, bar.HasTool, t.Warn)
					if remaining > 0 {
						ui.Row(f).WidthPercent(float32(remaining / float64(bar.Wall) * 100)).Height(height).Background(t.BorderL3)
					}
				})
			})
		}
		text := statsDash
		if bar.HasWall && bar.Wall > 0 {
			text = statsFormatMs(bar.Wall, true)
			if bar.Running {
				text = "≥ " + text
			}
		}
		ui.Text(f, text).FontSize(t.Fonts.Tiny).Color(t.LabelTertiary).Ellipsis()
	})
}

// drawStatsTotals 画官方表尾合计行：有 whole-log 投影就用全量，否则本窗口合计。
func (p *Plugin) drawStatsTotals(f ui.C, env *pluginapi.Env, turns []turnMetrics) {
	t := p.theme
	label := "本窗口合计"
	var turnCount, stepCount int64
	var ctx, output int64
	var cacheHit float64
	var ttft, model, tool int64
	var hasCtx, hasOutput, hasCache, hasTTFT, hasModel, hasTool bool
	var wall int64
	var hasWall bool
	var cacheRead, cacheCtx int64
	var cacheMeasured bool
	for _, turn := range turns {
		turnCount++
		for _, step := range turn.Steps {
			stepCount++
			if step.HasCtx {
				ctx += step.Context
				hasCtx = true
			}
			if step.HasOutput {
				output += step.Output
				hasOutput = true
			}
			if step.HasTTFT {
				ttft += step.TTFT
				hasTTFT = true
			}
			if step.HasModel {
				model += step.Model
				hasModel = true
			}
			if step.HasTool {
				tool += step.Tool
				hasTool = true
			}
		}
		if turn.HasWall {
			wall += turn.Wall
			hasWall = true
		}
	}
	// 窗口的缓存率按 token 加权（官方 turnCache 的合计版）：rate×context 反推缓存读。
	for _, turn := range turns {
		for _, step := range turn.Steps {
			if step.HasCache && step.HasCtx && step.Context > 0 {
				cacheRead += int64(step.CacheHit * float64(step.Context))
				cacheCtx += step.Context
				cacheMeasured = true
			}
		}
	}
	if cacheMeasured && cacheCtx > 0 {
		cacheHit, hasCache = math.Min(1, float64(cacheRead)/float64(cacheCtx)), true
	}
	if stats := env.Data.SessionStats; stats != nil {
		label = "全量合计"
		turnCount, stepCount = stats.Turns, stats.Steps
		ttft, hasTTFT = stats.TTFTMs, stats.TTFTMs > 0
		model, hasModel = stats.LLMMs, stats.LLMMs > 0
		tool, hasTool = stats.ToolMs, stats.ToolMs > 0
		wall, hasWall = stats.LLMMs+stats.ToolMs, stats.LLMMs+stats.ToolMs > 0
		// 全量行的 token 只来自 whole-log `tokenUsage`；窗口折叠值不能混进来。
		ctx, hasCtx, output, hasOutput, cacheHit, hasCache = 0, false, 0, false, 0, false
		if usage := env.Data.TokenUsage; usage != nil {
			ctx = usage.BilledInputTokens()
			hasCtx = true
			output = usage.OutputTokens
			hasOutput = true
			if ctx > 0 {
				cacheHit = float64(usage.CacheReadTokens) / float64(ctx)
				hasCache = true
			}
		}
	}
	if env.Data.SessionStats == nil {
		// 窗口模型用时可以从 step 求和；TTFT 也是逐 step 可加。
		if hasModel && hasTool {
			wall, hasWall = model+tool, true
		}
	}
	ui.Row(f).WidthPercent(100).Height(statsTurnRowHeight).Children(func() {
		p.statsCell(f, statsColScopePct, label, t.LabelPrimary, true)
		p.statsCell(f, statsColPromptPct, itoa64(turnCount)+" 轮 · "+itoa64(stepCount)+" 步", t.LabelTertiary, false)
		p.statsCell(f, statsColDidPct, "", t.LabelSecondary, false)
		p.statsMetricCell(f, statsColCtxPct, statsFormatTokensShort(ctx, hasCtx))
		p.statsMetricCell(f, statsColOutPct, statsFormatTokensShort(output, hasOutput))
		p.statsMetricCell(f, statsColCachePct, statsFormatPercent(cacheHit, hasCache))
		p.statsMetricCell(f, statsColTTFTPct, statsFormatMs(ttft, hasTTFT))
		p.statsMetricCell(f, statsColModelPct, statsFormatMs(model, hasModel))
		p.statsMetricCell(f, statsColTPSPct, statsFormatTps(statsTotalTPS(env, turns), statsHasTPS(env, turns)))
		p.statsMetricCell(f, statsColToolPct, statsFormatMs(tool, hasTool))
		p.statsBarCell(f, statsColTimePct, statsBarTurnHeight, statsTimeBar{
			Wall: wall, HasWall: hasWall, TTFT: ttft, HasTTFT: hasTTFT,
			Model: model, HasModel: hasModel, Tool: tool, HasTool: hasTool,
			FillShare: statsBarShare(wall, wall),
		})
	})
}

// statsTotalTPS 是合计行的输出速度：优先 whole-log decode 投影，否则逐 step 相加。
func statsTotalTPS(env *pluginapi.Env, turns []turnMetrics) float64 {
	if stats := env.Data.SessionStats; stats != nil && stats.DecodeMs > 0 && stats.DecodeTokens > 0 {
		return float64(stats.DecodeTokens) / (float64(stats.DecodeMs) / 1000)
	}
	var output, decode int64
	for _, turn := range turns {
		for _, step := range turn.Steps {
			if step.HasOutput {
				output += step.Output
			}
			if step.HasDecode {
				decode += step.Decode
			}
		}
	}
	if decode > 0 && output > 0 {
		return float64(output) / (float64(decode) / 1000)
	}
	return 0
}

func statsHasTPS(env *pluginapi.Env, turns []turnMetrics) bool {
	if stats := env.Data.SessionStats; stats != nil && stats.DecodeMs > 0 && stats.DecodeTokens > 0 {
		return true
	}
	var output, decode int64
	for _, turn := range turns {
		for _, step := range turn.Steps {
			if step.HasOutput {
				output += step.Output
			}
			if step.HasDecode {
				decode += step.Decode
			}
		}
	}
	return decode > 0 && output > 0
}

// statsTurnTrigger 是「触发」列：人提示词显示正文，自动触发显示来源标签。
//
// 文案逐字来自官方 `stats` zh 字典 `trigger.*`（OFF:locales.ts:13-24）。
func statsTurnTrigger(turn turnMetrics) string {
	switch turn.PromptSource {
	case "", "user":
		return oneLineText(turn.Prompt, 60)
	case "agent-message":
		return "智能体消息"
	case "team-message":
		return "团队消息"
	case "schedule":
		return "定时触发"
	case "webhook":
		return "Webhook"
	case "tool-jobs":
		return "后台任务"
	case "cordis-host-runner":
		return "插件触发"
	case "subagent-settled":
		return "子代理完成"
	case "goal":
		return "目标轮次"
	default:
		if turn.Prompt != "" {
			return oneLineText(turn.Prompt, 60)
		}
		return "自动触发"
	}
}

// statsHitID 造稳定交互 id（没有 Tree 时返回零值 = 不接交互）。
func (p *Plugin) statsHitID(env *pluginapi.Env, name string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, name)
}
