// skillrow.go —— 官方 `SkillRow.tsx`（173 行）的 Go 实现。
//
// # 官方渲染什么
//
// 一条 `skill` 工具调用画成**一行摘要 + 可展开的说明卡**：
//
//	[icon] 加载技能 · 名字/错误首行                          ← .row（24px，点它展开）
//	┌────────────────────────────────────────────────┐
//	│ 说明（CSS 转大写）                              │  ← .instructionsHeader
//	├────────────────────────────────────────────────┤
//	│ <pre>技能正文</pre>                             │  ← .instructions
//	└────────────────────────────────────────────────┘
//	( 查看 )                                          ← .inspectButton（悬停才显形）
//
// # 什么时候出现
//
// 每当消息流里有一条 `tool.name === "skill"` 的调用。**两棵树**：
//
//	phase === 'preparing'  → 只有 leading 图标 + 隐藏状态文案 + 闪烁标题
//	否则                   → 完整行（四态 running/ok/error/stopped）
//
// # 数据从哪来
//
// **只**从这一条 durable 的 call/result slice 推（官方注释：
// *"Derive display state without consulting the live skill catalog"*）。
// 一次 RPC 都不发——所以宿主缺 `skills/list` **不影响**这一行（见报告「缺口」§1）。
//
// # 有哪些交互
//
//	① 点 `.row`（仅 expandable = 有说明正文时）→ 展开/收起
//	② 键盘 Enter / 空格（仅 expandable）→ 同上
//	③ 点 `.inspectButton`（仅宿主注入了 `inspect` 时）→ 打开该 skill 的资源预览
//
// ①② 的判据是 `expandable = output !== null`——**没有说明正文就不给按钮语义**
// （不是"点了没反应"，是**根本没有 role=button / tabIndex / onClick**）。
package dsskill

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dstool"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 官方 block 的 Go 形态 ───────────────────────────────────────────

// SkillCall 是官方 `block.call`（只暴露行渲染用得到的 `argsRaw`）。
type SkillCall struct {
	// ArgsRaw 是官方 `call.argsRaw`：调用参数的**原文**（流式时可能是被截断的 JSON 前缀）。
	ArgsRaw string
}

// SkillCallError 是官方 `block.error`。
type SkillCallError struct {
	// Name 是错误名（官方 `error.name`）。
	Name string
	// Code 是错误码（官方 `error.code`）；`"interrupted"` 是**中止**的判据。
	Code string
}

// SkillResultItem 是官方 `block.content[]` 的一项。
type SkillResultItem struct {
	// Type 是官方 `item.type`；`"text"` 走 `Text`，其余走 `Raw` 的 JSON 序列化。
	Type string
	// Text 是 `Type == "text"` 时的正文。
	Text string
	// Raw 是这一项的**原始 JSON**（官方 `JSON.stringify(item, null, 2)` 的输入）。
	//
	// 为什么存原始字节而不是 `any`：`JSON.stringify` 保留对象的**插入顺序**，
	// 而 Go 的 `map[string]any` 序列化会按键排序——存成 `any` 再 marshal
	// 就悄悄改了官方输出的字节序。`json.Indent` 只在原文上加缩进，不动顺序。
	Raw json.RawMessage
}

// SkillBlock 是官方 `StartedToolCallViewProps['block']` 的 Go 形态。
//
// 官方用 `'kind' in block` 判别「已落定 / 未落定」，Go 用 `Settled`。
// **两个形态的字段不重叠**（官方就是这么写的：`settled ? block.call?.argsRaw : block.argsRaw`），
// 所以合并成一个结构体不会互相污染。
type SkillBlock struct {
	// Settled 是官方 `'kind' in block`：true = 调用已落定。
	Settled bool
	// CallID 是官方 `block.callId`（名字兜底用）。
	CallID string
	// ArgsRaw 是官方 `block.argsRaw`（**只在未落定时**读）。
	ArgsRaw string
	// Call 是官方 `block.call`（落定时读它的 `argsRaw`）。
	Call *SkillCall
	// Content 是官方 `block.content`。
	Content []SkillResultItem
	// Error 是官方 `block.error`。
	Error *SkillCallError
	// IsError 是官方 `block.isError`。
	IsError bool
}

// ── 官方 view model ────────────────────────────────────────────────

// skillRowModel 是官方 `SkillRowModel`（`SkillRow.tsx:15-20`）。
type skillRowModel struct {
	// Name 是 skill 名（`skillName` 的结果）。
	Name string
	// Output 是说明正文；nil = 官方 `null`（**没有正文 = 不可展开**）。
	Output *string
	// ErrorSummary 是错误态的首行摘要；nil = 官方 `null`。
	ErrorSummary *string
	// State 是四态。
	State SkillRowState
}

// expandable 是官方 `const expandable = model.output !== null`（`SkillRow.tsx:116`）。
func (m skillRowModel) expandable() bool { return m.Output != nil }

// open 是官方 `const open = expanded && expandable`（`SkillRow.tsx:117`）。
func (m skillRowModel) open(expanded bool) bool { return expanded && m.expandable() }

// firstLine 是官方 `firstLine`（`SkillRow.tsx:23-26`）。
//
// 只切 `'\n'`（**不是** `\r\n`）：官方的 `indexOf('\n')` 就是这样。
func firstLine(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i]
	}
	return text
}

// skillName 是官方 `skillName`（`SkillRow.tsx:29-41`）。
//
// 三条事实：
//
//	① 参数是 JSON **对象**且 `name` 是非空字符串 → 取 `firstLine(name)`。
//	② JSON 解析失败（流式截断）→ **不换目录查询**，退回原文首行。
//	③ `argsRaw === ''` → 用 `callId`（**不是**空串：空标题会让整行看起来坏了）。
func skillName(argsRaw, callID string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(argsRaw), &parsed); err == nil && parsed != nil {
		if raw, ok := parsed["name"]; ok {
			if name, ok := raw.(string); ok && name != "" {
				return firstLine(name)
			}
		}
	}
	if argsRaw == "" {
		return callID
	}
	return firstLine(argsRaw)
}

// resultText 是官方 `resultText`（`SkillRow.tsx:45-55`）。
//
// 三条事实：
//
//	① 未落定（`!'kind' in block`）→ nil（没有正文）。
//	② 逐项拼：`type === 'text'` 取 `text`，否则 `JSON.stringify(item, null, 2)`。
//	③ 一项都没有但**有 error** → 用 `"<name>: <code>"` 兜底（否则错误态就是空卡）。
//	   最后 `parts.join('\n') || null`：**拼接结果是空串也算 null**。
func resultText(b SkillBlock) *string {
	if !b.Settled {
		return nil
	}
	parts := make([]string, 0, len(b.Content))
	for _, item := range b.Content {
		if item.Type == "text" {
			parts = append(parts, item.Text)
			continue
		}
		parts = append(parts, jsonStringifyIndent2(item.Raw))
	}
	if len(parts) == 0 && b.Error != nil {
		parts = append(parts, b.Error.Name+": "+b.Error.Code)
	}
	joined := strings.Join(parts, "\n")
	if joined == "" {
		return nil
	}
	return &joined
}

// jsonStringifyIndent2 是官方 `JSON.stringify(raw, null, 2)` 的 Go 等价。
//
// 用 `json.Indent`（**不是** `json.MarshalIndent`）：前者只改空白、保留原始
// 键顺序；后者要先反序列化成 `map` 再序列化，键会被排序。官方输出是前者。
//
// 空 `Raw`（官方 `undefined`）→ `"null"`：`JSON.stringify(undefined)` 返回
// `undefined`，但它在**数组元素位置**会被序列化成 `null`，而官方这里正是
// 在数组里逐项拼。取 `"null"` 与实际输出一致。
func jsonStringifyIndent2(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "null"
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		// 不是合法 JSON：原样返回（官方 `stringify` 不会失败，但这里是外部数据，
		// 宁可显示原文也不能吞掉内容或 panic）。
		return string(raw)
	}
	return buf.String()
}

// deriveState 是官方 `skillRowModel` 里的状态三目（`SkillRow.tsx:61-65`）。
//
// 判据顺序**不能换**：`interrupted` 先于 `isError`——一个被用户中止的调用
// 在宿主眼里也是 `isError`，先判 `isError` 会把「已中止」画成「失败」。
func deriveState(b SkillBlock) SkillRowState {
	if !b.Settled {
		return SkillStateRunning
	}
	if b.Error != nil && b.Error.Code == "interrupted" {
		return SkillStateStopped
	}
	if b.IsError {
		return SkillStateError
	}
	return SkillStateOK
}

// skillRowModelOf 是官方 `skillRowModel`（`SkillRow.tsx:58-73`）。
func skillRowModelOf(b SkillBlock) skillRowModel {
	argsRaw := b.ArgsRaw
	if b.Settled {
		argsRaw = ""
		if b.Call != nil {
			argsRaw = b.Call.ArgsRaw
		}
	}
	state := deriveState(b)
	output := resultText(b)
	model := skillRowModel{
		Name:   skillName(argsRaw, b.CallID),
		Output: output,
		State:  state,
	}
	if state == SkillStateError && output != nil {
		summary := firstLine(*output)
		model.ErrorSummary = &summary
	}
	return model
}

// blockFromCard 把消息流交给工具卡的 `dstool.Card` 映射成官方 block。
//
// # 三处**有损**（都是"我们这一侧还没有那个字段"，不是编数据）
//
//	① 官方 `block.content[]` —— 我们的 `Card` 没有对应字段。
//	   最接近的真值是 `Card.View.Body`（宿主声明的结构化结果正文）。
//	   **没有它就没有正文** → 行不可展开（官方 `output === null` 的同一语义），
//	   绝不拿 `Card.Args` 或工具名凑一段"说明"。
//	② 官方 `block.error{name, code}` —— 没有对应字段。
//	   中止/失败判据用 `Card.Status`（消息流从显式结果、原始终局或活动 Turn
//	   算出的**真值**，见 `plugins/dstool/status.go`），
//	   所以 `stopped` / `error` 两态与官方同义。
//	③ 官方 `block.call.argsRaw` —— 没有对应字段，用 `Card.Args`（同一条消息）。
//	   `CardID` 未填时名字兜底退到 `Card.Seq` 的十进制串。
//
// ①②③ 全部登记在报告「缺口」§2；`dsconversation.drawTool` 填上
// `Card.CallID` / 结果块之后，本函数是把它们接上去的**唯一**改动点。
func blockFromCard(c dstool.Card) SkillBlock {
	callID := c.CallID
	if callID == "" {
		callID = strconv.Itoa(c.Seq)
	}
	block := SkillBlock{
		Settled: true, // 消息流只把已落定的工具消息交给工具卡
		CallID:  callID,
		ArgsRaw: c.Args,
		Call:    &SkillCall{ArgsRaw: c.Args},
	}
	if c.View != nil && c.View.Body != "" {
		block.Content = []SkillResultItem{{Type: "text", Text: c.View.Body}}
	}
	switch c.Status {
	case dstool.StatusError:
		block.IsError = true
		block.Error = &SkillCallError{Name: "skill", Code: "error"}
	case dstool.StatusStopped:
		block.IsError = true
		block.Error = &SkillCallError{Name: "skill", Code: "interrupted"}
	}
	return block
}

// ── 展开态（官方 `useState(false)` 的 Go 形态）────────────────────────

// skillExpandState 是每条卡自己的展开位（官方 `const [expanded, setExpanded] = useState(false)`）。
//
// 为什么要一个 map：viva 是**立即模式**，没有组件实例可以存 state；
// 官方的 `useState` 在 Go 侧对应「按卡的渲染键存一个 bool」。
// 键用 `dstool.Card.Seq`（消息流按消息身份分配的会话局部 int）——
// 同参交错调用、跨会话同序号都不会共享（见 `plugins/dstool/dstool.go` 的 `Card.Seq`）。
type skillExpandState struct {
	// expanded 是官方 `expanded`。**注意**它可以在不可展开时被置 true
	// （官方 `toggleExpand` 从不看 `expandable`），只是 `open = expanded && expandable`
	// 把它挡住了，所以界面上看不出来。未来 output 出现时它会**自动展开**——
	// 这是官方的真行为，实现不许"顺手修掉"。
	expanded map[int]bool
}

// newSkillExpandState 造一个空的展开态表。
func newSkillExpandState() *skillExpandState {
	return &skillExpandState{expanded: map[int]bool{}}
}

// toggle 是官方 `setExpanded(value => !value)`（`SkillRow.tsx:121-123`）。
func (s *skillExpandState) toggle(seq int) { s.expanded[seq] = !s.expanded[seq] }

// expandedFor 读一条卡的 `expanded`。
func (s *skillExpandState) expandedFor(seq int) bool { return s.expanded[seq] }

// ── leading 的三态（官方 `disclosureLeading`）────────────────────────

// skillLeading 是官方 `disclosureLeading`（`SkillRow.tsx:76-86`）的三个返回值。
//
// 三态而不是"两个 bool"：`open` 时 chevron **只有一枚**（没有 iconIdle），
// `!open && !expandable` 时只有图标（**没有** chevronHover）。
// 用两个 bool 表达会漏掉"open 时 iconIdle 不存在"这条官方事实。
type skillLeading int

const (
	// leadingChevronOnly 是 `open` 分支：leading 里只有 chevron。
	leadingChevronOnly skillLeading = iota
	// leadingIconOnly 是 `!expandable` 分支：只有 skill 图标，没有 chevron。
	leadingIconOnly
	// leadingIconWithChevron 是可展开的静止态：图标淡出 + chevron 淡入（两者叠放）。
	leadingIconWithChevron
)

// disclosureLeading 是官方 `disclosureLeading`。
func disclosureLeading(open, expandable bool) skillLeading {
	if open {
		return leadingChevronOnly
	}
	if !expandable {
		return leadingIconOnly
	}
	return leadingIconWithChevron
}

// stateStatus 是官方 `stateStatus`（`SkillRow.tsx:89-95`）：
// 只有 running 与 error 有**读屏**状态文案（ok / stopped 都没有）。
func stateStatus(state SkillRowState, lang string) (string, bool) {
	switch state {
	case SkillStateRunning:
		return SkillText(lang, KeyRowRunning), true
	case SkillStateError:
		return SkillText(lang, KeyRowFailed), true
	}
	return "", false
}

// summaryOf 是官方
// `model.state === 'stopped' ? t('row.stopped') : model.errorSummary ?? model.name`
// （`SkillRow.tsx:120`）。
func summaryOf(model skillRowModel, lang string) string {
	if model.State == SkillStateStopped {
		return SkillText(lang, KeyRowStopped)
	}
	if model.ErrorSummary != nil {
		return *model.ErrorSummary
	}
	return model.Name
}

// ── 绘制 ────────────────────────────────────────────────────────────

// skillContentWidth 是主区内容可用宽度（与 `plugins/dstool` 的 `contentWidth` 同源）。
func skillContentWidth(env *pluginapi.Env) float32 {
	w := env.MainWidth - 40
	if w < 200 {
		w = 200
	}
	return w
}

// drawSkillRow 画一条**已落定**的 skill 行，返回本帧高度（视口虚拟化用）。
func (p *Plugin) drawSkillRow(f ui.C, env *pluginapi.Env, c dstool.Card) float32 {
	t := env.Theme
	block := blockFromCard(c)
	model := skillRowModelOf(block)
	expandable := model.expandable()
	open := model.open(p.expand.expandedFor(c.Seq))
	status, hasStatus := stateStatus(model.State, p.lang)
	summary := summaryOf(model, p.lang)
	leading := disclosureLeading(open, expandable)

	// 展开体的高度由子节点绘制时算出（官方靠浏览器排版，本仓立即模式只能自报）。
	var bodyHeight float32

	// .card{display:flex;flex-direction:column}
	//
	// 为什么要给卡本身一个命中区：官方的 `.card:hover .inspectButton{opacity:1}`
	// 挂的是**整张卡**的悬停（不是按钮自己的）。viva 的 `StateFrom(container)`
	// 跟的是 container 的**悬停状态**，而悬停状态由命中区算——没有命中区的
	// 盒子永远不会"被悬停"，`.inspectButton` 就永远停在 `opacity:0`（点得到、
	// 看不见）。所以这里给卡一个 `RoleGroup` 的命中区：参与悬停，但没有按钮
	// 语义（`SkipTab` 不让它进 Tab 序），与官方那个普通 `div` 同形。
	card := ui.Column(f).FillWidth().Gap(0).
		HitWith(env.Tree.ID(env.Feat, "skill-card-"+strconv.Itoa(c.Seq)), "",
			gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true})
	card.Children(func() {
		// .row{position:relative;overflow:hidden;display:flex;align-items:center;
		//   height:calc(24px + delta);min-width:0;color:label-tertiary;
		//   transition:color 100ms ease}
		row := ui.Row(f).FillWidth().Height(SkillRowHeight).Items(ui.Center).Gap(0).
			Color(t.LabelTertiary)
		if expandable {
			// 官方只有 expandable 时才挂 role=button / tabIndex=0 / onClick /
			// onKeyDown / cursor:pointer（`SkillRow.tsx:129-135` + CSS:26）。
			row = row.HitWith(
				env.Tree.ID(env.Feat, "skill-row-"+strconv.Itoa(c.Seq)),
				SkillText(p.lang, KeyRowTitle),
				gfx.HitOptions{
					Role:       gfx.RoleButton,
					Cursor:     gfx.PointerCursorHand,
					Expanded:   open,
					Expandable: expandable,
				},
			)
		}
		// `@media (hover:hover){.row:hover{color:label-secondary}}`（CSS:19-23）
		row = row.Hover(ui.Look{}.Color(t.LabelSecondary))
		row.Children(func() {
			p.drawLeading(f, row, leading, t.LabelTertiary)

			if hasStatus {
				// 官方 `span.visuallyHidden`（`SkillRow.tsx:145`）：
				// position:absolute;width:1px;height:1px;overflow:hidden;
				// clip:rect(0 0 0 0);white-space:nowrap —— 只给读屏，不占位。
				ui.Text(f, status).Width(SkillVisuallyHiddenWidth).
					Height(SkillVisuallyHiddenHeight).Absolute().Opacity(0)
			}

			// TextShimmer（`SkillRow.tsx:146`）：active={running} 才闪。
			// 本仓没有 shimmer 动画原语（见报告「缺口」§4），结构照官方搭：
			// 一个 flex 行里放 title / separator / summary 三个兄弟。
			shimmer := ui.Row(f).Items(ui.Center).Gap(0).Grow(1)
			shimmer.Children(func() {
				// .title{flex:none;font-size:13px;line-height:calc(24px + delta)}
				ui.Text(f, SkillText(p.lang, KeyRowTitle)).
					FontSize(SkillTitleFontSize).LineHeight(SkillTitleLineHeight).
					Color(t.LabelTertiary)
				// .separator{flex:none;width:2px;height:2px;border-radius:1px;
				//   margin:0 8px;background:label-caption}
				ui.Row(f).Width(SkillSeparatorWidth).Height(SkillSeparatorHeight).
					Radius(SkillSeparatorRadius).
					MarginX(SkillSeparatorMarginInline).
					Background(t.LabelCaption)
				// .summary{flex:1 1 auto;min-width:0;overflow:hidden;
				//   text-overflow:ellipsis;white-space:nowrap;font-size:13px;line-height:24px}
				summaryColor := t.LabelTertiary
				switch model.State {
				case SkillStateError:
					summaryColor = t.Error // .errorSummary
				case SkillStateStopped:
					summaryColor = t.WarnLabel // .stoppedSummary
				}
				ui.Text(f, summary).
					FontSize(SkillSummaryFontSize).LineHeight(SkillSummaryLineHeight).
					Color(summaryColor).MaxWidth(skillContentWidth(env)).
					MaxLines(1).Ellipsis()
			})
		})
		if expandable {
			// 官方有两个入口：`onClick: toggleExpand`（`SkillRow.tsx:133`）与
			// `onKeyDown: toggleFromKeyboard`（`SkillRow.tsx:124-128`，
			// 只认 Enter / 空格）。
			//
			// viva 把这两条**收在一个 API 里**：`Canvas.Clicked(id)` 在
			// 「指针点在这个命中区上」和「焦点在它上面时按 Enter/空格」
			// 两种事件里都返回真（`app/painter.go` 的 `deliverClick` 由
			// 焦点路由器的 `result.Activated` 触发）。所以这里**不写**
			// `Box.OnKey`——那个 API 只给 TextField 用（调它会 panic：
			// `ui: OnKey is for a TextField`）。
			//
			// 官方 `preventDefault()` 在本仓没有对应物：`Clicked` 是**读**
			// 一个已经发生的激活，不是可取消的 DOM 事件；空格不会滚动页面
			// 因为画面根本不是文档流。
			if row.Clicked() {
				p.expand.toggle(c.Seq)
			}
		}

		if !open {
			return
		}
		// .bodyWrap{display:flex;flex-direction:column}（`SkillRow.tsx:158`）
		body := ui.Column(f).FillWidth().Gap(0)
		body.Children(func() {
			bodyHeight = p.drawInstructionsCard(f, card, env, t, model)
			if p.inspect != nil {
				bodyHeight += p.drawInspectButton(f, card, t, c.Seq, env, model.Name)
			}
		})
	})

	if !open {
		return SkillRowHeight
	}
	return SkillRowHeight + bodyHeight
}

// drawLeading 画官方 `.leading` 的三态（`SkillRow.tsx:76-86`）。
//
// `row` 是 `.row` 那个盒子：官方的悬停换图是 `.row:hover .iconIdle{opacity:0}`
// 与 `.row:hover .chevronHover{opacity:1}`——**状态来自行**，不是来自图标自己。
// viva 对应 `StateFrom(row)`（"what is inside a row changes with the row"）。
func (p *Plugin) drawLeading(f ui.C, row ui.Box, leading skillLeading, tint ui.Color) {
	// .leading{position:relative;flex:none;width:16;height:16;display:inline-flex;
	//   align-items:center;justify-content:center;margin-right:6px;color:inherit}
	box := ui.Row(f).Width(SkillLeadingWidth).Height(SkillLeadingHeight).
		MarginRight(SkillLeadingMarginRight).Items(ui.Center).Justify(ui.Center)
	box.Children(func() {
		switch leading {
		case leadingChevronOnly:
			ui.Icon(f, chevronDownIcon()).Size(SkillChevronSize, SkillChevronSize).Color(tint)
		case leadingIconOnly:
			drawSkillGlyph(f, tint, SkillGlyphSize)
		case leadingIconWithChevron:
			// 两枚**叠放**：`.iconIdle{display:inline-flex;opacity:1}` +
			// `.chevronHover{position:absolute;inset:0;margin:auto;opacity:0}`。
			// 静止态看到的是图标；悬停时图标淡出、chevron 淡入。
			ui.Stack(f).Width(SkillLeadingWidth).Height(SkillLeadingHeight).Children(func() {
				iconIdle := ui.Row(f).Width(SkillLeadingIconSize).Height(SkillLeadingIconSize).
					Opacity(SkillIconIdleOpacity).
					Hover(ui.Look{}.Opacity(SkillRowHoverIconIdleOpacity)).
					StateFrom(row)
				iconIdle.Children(func() { drawSkillGlyph(f, tint, SkillGlyphSize) })

				ui.Icon(f, chevronDownIcon()).
					Size(SkillChevronSize, SkillChevronSize).Color(tint).
					Opacity(SkillChevronHoverOpacity).
					Hover(ui.Look{}.Opacity(SkillRowHoverChevronOpacity)).
					StateFrom(row)
			})
		}
	})
}

// drawInstructionsCard 画官方 `.instructionsCard`（`SkillRow.tsx:159-162`），返回高度。
func (p *Plugin) drawInstructionsCard(f ui.C, card ui.Box, env *pluginapi.Env, t *pluginapi.Theme, model skillRowModel) float32 {
	output := ""
	if model.Output != nil {
		output = *model.Output
	}
	// .instructionsCard{display:flex;flex-direction:column;max-height:260px;
	//   margin:4px 0 4px 4px;overflow:hidden;border:0.5px solid border-l1;
	//   border-radius:radius-lg;background:markdown-code-block}
	box := ui.Column(f).FillWidth().
		MarginTop(SkillInstructionsCardMarginTop).
		MarginBottom(SkillInstructionsCardMarginBottom).
		MarginLeft(SkillInstructionsCardMarginLeft).
		Border(SkillInstructionsCardBorderWidth, t.BorderL1).
		Radius(SkillInstructionsCardRadius).
		Background(t.CodeBlock).
		MaxHeight(SkillInstructionsCardMaxHeight)
	box.Children(func() {
		// .instructionsHeader{flex:none;padding:8px 12px;
		//   border-bottom:0.5px solid border-l2;background:markdown-code-block-banner;
		//   font-size:11px;font-weight:500;line-height:16px;color:label-caption;
		//   text-transform:uppercase;letter-spacing:.04em}
		//
		// `text-transform:uppercase` 是**可见文案变换**：官方字典里是小写混合
		// （`说明` / `Instructions`），显示成大写的责任在样式层。本仓没有
		// `text-transform` 原语，用 ASCII 大写变换在绘制期做等价变换
		// （中文没有大小写，`说明` 不变；英文变 `INSTRUCTIONS`——与官方一致）。
		ui.Text(f, uppercaseTypeTransform(SkillText(p.lang, KeyRowInstructions))).
			PadSides(SkillInstructionsHeaderPadTop, SkillInstructionsHeaderPadRight,
				SkillInstructionsHeaderPadBottom, SkillInstructionsHeaderPadLeft).
			FontSize(SkillInstructionsHeaderFontSize).
			Weight(SkillInstructionsHeaderWeight).
			LineHeight(SkillInstructionsHeaderLineHeight).
			Color(t.LabelCaption).
			Background(t.CodeBanner)
		// `border-bottom` 的等价物：viva 没有单边 Border，画一条 0.5px 的分隔盒。
		ui.Row(f).FillWidth().Height(SkillInstructionsHeaderBorderBottom).Background(t.BorderL2)
		// .instructions{min-height:0;margin:0;padding:10px 12px 12px;overflow:auto;
		//   white-space:pre-wrap;overflow-wrap:anywhere;
		//   font:--dsw-font-markdown-code-block-small;color:label-secondary}
		instructionsColor := t.LabelSecondary
		if model.State == SkillStateError {
			instructionsColor = t.Error // .instructions[data-error]
		}
		ui.Paragraph(f, output, 0).
			PadSides(SkillInstructionsPadTop, SkillInstructionsPadRight,
				SkillInstructionsPadBottom, SkillInstructionsPadLeft).
			Mono().
			FontSize(SkillInstructionsFontSize).
			LineHeight(SkillInstructionsLineHeight).
			Color(instructionsColor)
	})
	// 高度估算：header(8+16+8) + 0.5 分隔 + 正文(10 + 行数×19 + 12)，整体被 260 夹住。
	header := SkillInstructionsHeaderPadTop + SkillInstructionsHeaderLineHeight +
		SkillInstructionsHeaderPadBottom + SkillInstructionsHeaderBorderBottom
	innerWidth := skillContentWidth(env) - SkillInstructionsCardMarginLeft -
		SkillInstructionsPadLeft - SkillInstructionsPadRight
	body := SkillInstructionsPadTop + SkillInstructionsPadBottom +
		float32(skillLineCount(output, innerWidth))*SkillInstructionsLineHeight
	cardHeight := header + body
	if cardHeight > SkillInstructionsCardMaxHeight {
		cardHeight = SkillInstructionsCardMaxHeight
	}
	return SkillInstructionsCardMarginTop + cardHeight + SkillInstructionsCardMarginBottom
}

// drawInspectButton 画官方 `.inspectButton`（`SkillRow.tsx:163-168`），返回高度。
//
// `card` 是**外层** `.card`：官方的显形条件是 `.card:hover .inspectButton{opacity:1}`
// ——悬停的是整张卡，不是按钮自己（`StateFrom(card)`）。
func (p *Plugin) drawInspectButton(f ui.C, card ui.Box, t *pluginapi.Theme, seq int, env *pluginapi.Env, name string) float32 {
	// .inspectButton{display:inline-flex;align-self:flex-start;align-items:center;
	//   gap:4px;margin:4px 0 2px 4px;padding:2px 8px;border:0.5px solid border-l4;
	//   border-radius:999px;corner-shape:round;background:bg-base;color:label-secondary;
	//   font-size:11px;line-height:16px;cursor:pointer;opacity:0;
	//   transition:opacity 100ms ease}
	label := SkillText(p.lang, KeyRowInspect)
	button := ui.Row(f).Items(ui.Center).Gap(SkillInspectGap).
		MarginSides(SkillInspectMarginTop, SkillInspectMarginRight,
			SkillInspectMarginBottom, SkillInspectMarginLeft).
		PadSides(SkillInspectPadTop, SkillInspectPadRight,
			SkillInspectPadBottom, SkillInspectPadLeft).
		Border(SkillInspectBorderWidth, t.BorderL4).
		Radius(SkillInspectRadius).
		Background(t.BgBase).
		Color(t.LabelSecondary).
		Opacity(SkillInspectOpacity).
		// `@media (hover:hover){.card:hover .inspectButton{opacity:1}}`（CSS:185-187）
		Hover(ui.Look{}.Opacity(SkillCardHoverInspectOpacity).
			// `.inspectButton:hover{background:hover-solid;color:label-primary}`（CSS:189-192）
			Background(hoverSolidToken(t)).Color(t.LabelPrimary)).
		StateFrom(card).
		HitWith(
			env.Tree.ID(env.Feat, "skill-inspect-"+strconv.Itoa(seq)),
			label,
			gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand},
		)
	button.Children(func() {
		ui.Icon(f, inspectIcon()).Size(SkillInspectSize, SkillInspectSize).Color(t.LabelSecondary)
		ui.Text(f, label).
			FontSize(SkillInspectFontSize).LineHeight(SkillInspectLineHeight).
			Color(t.LabelSecondary)
	})
	if button.Clicked() && p.inspect != nil {
		p.inspect(name)
	}
	// 高 = 行高 16 + 上下内边距 2+2 + 上下边框 0.5+0.5 = 21；外加 margin 4+2。
	return SkillInspectLineHeight + 2*SkillInspectPadTop + 2*SkillInspectBorderWidth +
		SkillInspectMarginTop + SkillInspectMarginBottom
}

// drawPreparing 画官方 **preparing 态**（`SkillRow.tsx:103-109`），返回本帧高度。
//
// 这棵树比 started 态少得多：没有 separator、没有 summary、没有展开体，
// 且 `data-state` **写死** `"preparing"`（不取四态）。单独一条路径是因为
// 「preparing 时 summary 不出现」是一条会被实现忘掉的官方事实。
func (p *Plugin) drawPreparing(f ui.C, env *pluginapi.Env, c dstool.Card) float32 {
	t := env.Theme
	// div.card[data-tool="skill"][data-state="preparing"]
	card := ui.Column(f).FillWidth().Gap(0)
	card.Children(func() {
		// div.row（preparing 态没有 data-expandable，也没有 role/onClick）
		row := ui.Row(f).FillWidth().Height(SkillRowHeight).Items(ui.Center).Gap(0).
			Color(t.LabelTertiary)
		row.Children(func() {
			// span.leading > IconSkillOutlineRegular(size=14)
			ui.Row(f).Width(SkillLeadingWidth).Height(SkillLeadingHeight).
				MarginRight(SkillLeadingMarginRight).Items(ui.Center).Justify(ui.Center).
				Children(func() { drawSkillGlyph(f, t.LabelTertiary, SkillGlyphSize) })
			// span.visuallyHidden > t('row.preparing')
			ui.Text(f, SkillText(p.lang, KeyRowPreparing)).
				Width(SkillVisuallyHiddenWidth).Height(SkillVisuallyHiddenHeight).
				Absolute().Opacity(0)
			// TextShimmer.active > span.title
			ui.Text(f, SkillText(p.lang, KeyRowTitle)).
				FontSize(SkillTitleFontSize).LineHeight(SkillTitleLineHeight).
				Color(t.LabelTertiary)
		})
	})
	return SkillRowHeight
}

// uppercaseTypeTransform 是官方 `text-transform:uppercase` 的**等价变换**。
//
// 为什么不直接 `strings.ToUpper`：那个函数做的是 Unicode 大小写映射
// （土耳其语 `i` → `İ`、希腊语终位 sigma 等），而 CSS `text-transform`
// 只对**有大小写的字母**生效。官方这 8 种字典里所有需要变形的字符
// 都是 ASCII 小写字母，所以「只把 ASCII 小写转大写」与官方结果逐字节相同，
// 且不会碰到任何非 ASCII 字符。
func uppercaseTypeTransform(text string) string {
	needs := false
	for i := 0; i < len(text); i++ {
		if c := text[i]; c >= 'a' && c <= 'z' {
			needs = true
			break
		}
	}
	if !needs {
		return text
	}
	out := make([]byte, len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

// skillLineCount 估算一段 pre-wrap 文本在给定宽度下的行数（**只用于高度估算**）。
//
// 为什么要估：`Service.Height` 在**排版之前**被调用（视口虚拟化），
// 拿不到真实排版结果。估小了会把还在视口里的消息判成"在视口外"直接不画，
// 所以宁可估大。
func skillLineCount(text string, width float32) int {
	if text == "" || width <= 0 {
		return 1
	}
	// 等宽字：11px 的代码字宽约 6.6px（11 × 0.6）。
	perLine := int(width / (SkillInstructionsFontSize * 0.6))
	if perLine < 8 {
		perLine = 8
	}
	lines := 0
	for _, para := range strings.Split(text, "\n") {
		if para == "" {
			lines++
			continue
		}
		runes := 0
		for range para {
			runes++
		}
		n := (runes + perLine - 1) / perLine
		if n < 1 {
			n = 1
		}
		lines += n
	}
	return lines
}
