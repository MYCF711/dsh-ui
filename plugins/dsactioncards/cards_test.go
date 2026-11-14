package dsactioncards

// 无头测试：真的画一帧（ui.NewPage + gfx.Canvas），点真的 Hit id，断言画面文字与几何。
// 不 mock 绘制层、不绕过按钮直接调方法——点击走的是产品同一条路径。
//
// 官方几何/文案的证据全部在 cards.go 的常量注释里（OFF:… 的行号）。
//
// 一个 viva 的硬约束：`ui.Box` 只在**本次 paint** 内有效，跨帧再读会 panic。
// 所以每次 draw 完立刻把矩形抄成纯数值（snap），断言只对数值做。

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 假数据源：记录每一次真实调用 ──────────────────────────────────────────

type fakeAct struct {
	session string
	callID  string
	verb    string
	payload map[string]json.RawMessage
}

type fakeSource struct {
	view    *apiclient.ActionCardView
	viewErr error

	// raw 是 Act 的成功响应；err 非 nil 时 Act 失败（验"宿主错误原文"那条）。
	raw json.RawMessage
	err error

	views  []string
	acteds []fakeAct
}

func (f *fakeSource) View(_ context.Context, sessionID string) (*apiclient.ActionCardView, error) {
	f.views = append(f.views, sessionID)
	if f.viewErr != nil {
		return nil, f.viewErr
	}
	return f.view, nil
}

func (f *fakeSource) Act(_ context.Context, sessionID, callID, verb string, payload map[string]json.RawMessage) (json.RawMessage, error) {
	f.acteds = append(f.acteds, fakeAct{session: sessionID, callID: callID, verb: verb, payload: payload})
	if f.err != nil {
		return nil, f.err
	}
	return f.raw, nil
}

func (f *fakeSource) acted() []fakeAct { return append([]fakeAct(nil), f.acteds...) }

// ── 夹具 ─────────────────────────────────────────────────────────────────

// fixture 是一个设备齐全的无头环境：真主题、真 Tree、真 Canvas。
//
// `run` 换成同步执行：产品路径把宿主调用丢给 goroutine，测试必须在同一线程看到
// 结算结果，否则断言就是撞运气（同 dsdirpicker / dssettings 的思路）。
func fixture(t *testing.T, src CardSource) (*Plugin, *pluginapi.Env, *gfx.Tree, *gfx.Canvas) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("actioncards-test")
	p := &Plugin{theme: theme, run: func(fn func()) { fn() }}
	env := &pluginapi.Env{
		Theme: theme,
		Tree:  tree,
		Feat:  feat,
		Data:  pluginapi.Snapshot{Cur: "s1"},
	}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	SetSource(src)
	t.Cleanup(func() { SetSource(nil) })
	return p, env, tree, canvas
}

// ── 几何快照（纯数值，可跨帧比较）────────────────────────────────────────

type cardSnap struct {
	Card, Body, Title, Brief, Toggle, Footer, Dismiss gfx.Rect
	Statuss                                           []gfx.Rect
}

type rowSnap struct {
	Row, Icon, Title, State, CardTitle, Reason gfx.Rect
}

func rectOf(box ui.Box) gfx.Rect {
	if !box.Valid() {
		return gfx.Rect{}
	}
	return box.Rect()
}

func (p *Plugin) snapCard() cardSnap {
	out := cardSnap{
		Card:    rectOf(p.cardBoxes.Card),
		Body:    rectOf(p.cardBoxes.Body),
		Title:   rectOf(p.cardBoxes.Title),
		Brief:   rectOf(p.cardBoxes.Brief),
		Toggle:  rectOf(p.cardBoxes.Toggle),
		Footer:  rectOf(p.cardBoxes.Footer),
		Dismiss: rectOf(p.cardBoxes.Dismiss),
	}
	for _, box := range p.cardBoxes.Statuss {
		out.Statuss = append(out.Statuss, rectOf(box))
	}
	return out
}

func (p *Plugin) snapRow() rowSnap {
	return rowSnap{
		Row:       rectOf(p.rowBoxes.Row),
		Icon:      rectOf(p.rowBoxes.Icon),
		Title:     rectOf(p.rowBoxes.Title),
		State:     rectOf(p.rowBoxes.State),
		CardTitle: rectOf(p.rowBoxes.CardTitle),
		Reason:    rectOf(p.rowBoxes.Reason),
	}
}

// drawCardOnce / drawRowOnce 画一帧并抄下几何。
//
// **先 Reset**：canvas 的图元是累加的，不 Reset 的话"上一帧画过、这一帧不画"的
// 节点会留在画面上，断言就成了假的。Reset 不动事件态，放在 BeginEventPass 之后也安全。
func drawCardOnce(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, w, h float32) cardSnap {
	t.Helper()
	canvas.Reset()
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: w, Height: h}, func(f ui.C) { p.drawCardTail(f, env) })
	return p.snapCard()
}

func drawRowOnce(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, w, h float32) rowSnap {
	t.Helper()
	canvas.Reset()
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: w, Height: h}, func(f ui.C) { p.drawProposeRow(f, env) })
	return p.snapRow()
}

// clickCard 走一次真实的点击事件通道：BeginEventPass → 画一帧 → EndEventPass。
func clickCard(t *testing.T, p *Plugin, env *pluginapi.Env, tree *gfx.Tree, canvas *gfx.Canvas, w, h float32, id string) cardSnap {
	t.Helper()
	canvas.BeginEventPass(tree.ID(env.Feat, id), gfx.ClickInfo{})
	out := drawCardOnce(t, p, env, canvas, w, h)
	canvas.EndEventPass()
	return out
}

func clickRow(t *testing.T, p *Plugin, env *pluginapi.Env, tree *gfx.Tree, canvas *gfx.Canvas, w, h float32, id string) rowSnap {
	t.Helper()
	canvas.BeginEventPass(tree.ID(env.Feat, id), gfx.ClickInfo{})
	out := drawRowOnce(t, p, env, canvas, w, h)
	canvas.EndEventPass()
	return out
}

// ── 文字断言 ─────────────────────────────────────────────────────────────

func textValues(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func textOf(t *testing.T, canvas *gfx.Canvas, value string) gfx.Text {
	t.Helper()
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text
		}
	}
	t.Fatalf("画面缺少 %q；实际文字=%v", value, textValues(canvas))
	return gfx.Text{}
}

// cardView 是一份只有一张卡、还没决定的官方投影（形状照
// HOST:internal/actioncards/service.go:111-133）。
func cardView() *apiclient.ActionCardView {
	return &apiclient.ActionCardView{
		Proposals: map[string]apiclient.ActionCardProposal{
			"call-1": {
				CallID: "call-1",
				Turn:   3,
				Action: "create-package",
				Title:  "新建一个包",
				Brief:  "按这份说明创建一个新的包。",
				Details: map[string]json.RawMessage{
					"name": json.RawMessage(`"demo"`),
				},
			},
		},
		Decisions: map[string]apiclient.ActionCardDecision{},
	}
}

// ── 1. 没有 source → 零节点 ──────────────────────────────────────────────

func TestNoSourceDrawsNothing(t *testing.T) {
	p, env, _, canvas := fixture(t, nil)
	snap := drawCardOnce(t, p, env, canvas, 800, 600)
	if len(canvas.Texts()) != 0 {
		t.Fatalf("没有 source 时不该画任何文字，实际 %v", textValues(canvas))
	}
	if snap.Card.Width != 0 || snap.Card.Height != 0 {
		t.Fatalf("没有 source 时卡片框不该有面积：%+v", snap.Card)
	}
	row := drawRowOnce(t, p, env, canvas, 800, 600)
	if row.Row.Width != 0 || row.Row.Height != 0 {
		t.Fatalf("没有 source 时过程行不该有面积：%+v", row.Row)
	}
}

// ── 2. 投影为空 → 零节点 ────────────────────────────────────────────────

func TestEmptyViewDrawsNothing(t *testing.T) {
	src := &fakeSource{view: &apiclient.ActionCardView{
		Proposals: map[string]apiclient.ActionCardProposal{},
		Decisions: map[string]apiclient.ActionCardDecision{},
	}}
	p, env, _, canvas := fixture(t, src)
	if err := p.Refresh(context.Background(), "s1"); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	snap := drawCardOnce(t, p, env, canvas, 800, 600)
	if len(canvas.Texts()) != 0 {
		t.Fatalf("投影为空时不该画任何文字，实际 %v", textValues(canvas))
	}
	if snap.Card.Width != 0 || snap.Card.Height != 0 {
		t.Fatalf("投影为空时卡片框不该有面积：%+v", snap.Card)
	}
}

// ── 3. 卡片框几何 = 官方值 ──────────────────────────────────────────────

func TestCardGeometryMatchesOfficial(t *testing.T) {
	src := &fakeSource{view: cardView()}
	p, env, _, canvas := fixture(t, src)
	if err := p.Refresh(context.Background(), "s1"); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	got := drawCardOnce(t, p, env, canvas, 800, 600)

	// 常量本身先钉住：改常量必须对着官方 CSS 行号改（下面再验真画出来的几何）。
	if cardMarginY != 16 || cardBodyPadLeft != 20 || cardBodyPadTop != 12 ||
		cardTitleFontSize != 16 || cardTitleWeight != 500 {
		t.Fatalf("几何常量与官方 CSS 不一致：margin=%v bodyPadLeft=%v bodyPadTop=%v titleSize=%v titleWeight=%v",
			cardMarginY, cardBodyPadLeft, cardBodyPadTop, cardTitleFontSize, cardTitleWeight)
	}
	if cardTitleLineH != 24 || 16*1.5 != cardTitleLineH {
		t.Fatalf("标题行高常量=%v，want 16×1.5=24（Card.module.css:3）", cardTitleLineH)
	}

	// `.card{ margin:16px 0 }`（OFF:src/client/Card.module.css:1）→ 卡片顶边 16。
	if got.Card.Y != 16 {
		t.Fatalf("卡片 top=%v，want 16（Card.module.css:1 的 margin:16px 0）", got.Card.Y)
	}
	// `.body{ padding:12px 20px 20px }`（OFF:src/client/Card.module.css:2）
	// → 正文左内边距 20、上内边距 12。
	// 0.5px 边框在 padding 外侧（`.card{border:0.5px}` + `.body{padding:...}`），
	// 所以正文里的第一行相对卡片框的位移 = 边框 + 内边距（量标题这一行，最准）。
	if d := got.Title.X - got.Card.X; d != cardBodyPadLeft+cardBorder {
		t.Fatalf("正文左内边距=%v，want 20+0.5（Card.module.css:1-2）", d)
	}
	if d := got.Title.Y - got.Card.Y; d != cardBodyPadTop+cardBorder {
		t.Fatalf("正文上内边距=%v，want 12+0.5（Card.module.css:1-2）", d)
	}
	// `.title{ font-size:16px; font-weight:500; line-height:1.5 }`（Card.module.css:3）
	// → 标题字号 16、字重 500。
	title := textOf(t, canvas, "新建一个包")
	if title.Style.Size != 16 {
		t.Fatalf("标题字号=%v，want 16（Card.module.css:3）", title.Style.Size)
	}
	if title.Style.Weight != 500 {
		t.Fatalf("标题字重=%v，want 500（Card.module.css:3）", title.Style.Weight)
	}
	// 边框圆角取官方 `var(--dsw-radius-lg)`（Card.module.css:1）→ 主题 Radii.LG。
	if radii := p.theme.Radii.LG; radii != 16 {
		t.Fatalf("主题 Radii.LG=%v，官方 --dsw-radius-lg 是 16（Card.module.css:1）", radii)
	}
}

// ── 4. 文案逐字（官方 zh 字典原文）──────────────────────────────────────

func TestCopyIsVerbatimFromOfficialLocale(t *testing.T) {
	// 全部逐字抄自 OFF:src/client/locales.ts:15-22 的 zh 字典（不是重写的文案）。
	cases := map[string]struct{ got, want string }{
		"expandBrief":   {zhExpandBrief, "展开"},
		"collapseBrief": {zhCollapseBrief, "收起"},
		"dismiss":       {zhDismiss, "暂不需要"},
		"dismissed":     {zhDismissed, "已忽略"},
		"replaced":      {zhReplaced, "已替换"},
		"unavailable":   {zhUnavailable, "暂时无法执行，请到插件页面查看"},
		"transport":     {zhTransport, "暂时无法确认结果，请重新连接并查看卡片状态后再重试"},
		"row.title":     {zhRowTitle, "操作卡片"},
		"row.preparing": {zhRowPreparing, "正在准备卡片"},
		"row.running":   {zhRowRunning, "正在提议"},
		"row.ok":        {zhRowOK, "已提议"},
		"row.error":     {zhRowError, "未显示"},
		"row.stopped":   {zhRowStopped, "已中断"},
		"row.inspect":   {zhRowInspect, "查看调用"},
	}
	for key, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s=%q，官方 zh 字典原文=%q", key, tc.got, tc.want)
		}
	}

	// 四个行状态的渲染文案（`t(\`row.${state}\`)`，OFF:ProposeRow.tsx:38）。
	states := map[string]string{
		rowPreparing: "正在准备卡片",
		rowRunning:   "正在提议",
		rowOK:        "已提议",
		rowStopped:   "已中断",
	}
	for state, want := range states {
		if got := proposeStateText(state); got != want {
			t.Errorf("proposeStateText(%q)=%q，官方 row.%s=%q", state, got, state, want)
		}
	}
	if got := proposeStateText("something-else"); got != zhRowError {
		t.Errorf("未知状态应回落到 row.error=%q，实际 %q", zhRowError, got)
	}
}

// ── 5. 点「暂不需要」→ Act 恰好一次，参数逐字 ────────────────────────────

func TestDismissCallsActOnceWithExactArguments(t *testing.T) {
	src := &fakeSource{
		view: cardView(),
		raw:  json.RawMessage(`{"status":"settled","decision":{"callId":"call-1","action":"create-package","phase":"dismissed"}}`),
	}
	p, env, tree, canvas := fixture(t, src)
	if err := p.Refresh(context.Background(), "s1"); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	drawCardOnce(t, p, env, canvas, 800, 600) // 先立起一帧（Hit id 在这一帧注册）
	clickCard(t, p, env, tree, canvas, 800, 600, "dismiss")

	acted := src.acted()
	if len(acted) != 1 {
		t.Fatalf("Act 应恰好被调 1 次，实际 %d 次：%+v", len(acted), acted)
	}
	got := acted[0]
	if got.session != "s1" {
		t.Errorf("Act sessionId=%q，want %q", got.session, "s1")
	}
	if got.callID != "call-1" {
		t.Errorf("Act callId=%q，want %q", got.callID, "call-1")
	}
	if verbDismiss != "dismiss" {
		t.Errorf("verbDismiss=%q，want 官方共享动词 %q（HOST:internal/actioncards/service.go:45）", verbDismiss, "dismiss")
	}
	if got.verb != "dismiss" {
		t.Errorf("Act verb=%q，want %q", got.verb, "dismiss")
	}
	if got.payload != nil {
		t.Errorf("dismiss 的 payload 应为 nil（官方 acceptsUndefined），实际 %+v", got.payload)
	}

	// 宿主回的 settled.decision 是**真值**：下一帧必须显示「已忽略」（官方 dismissed）。
	drawCardOnce(t, p, env, canvas, 800, 600)
	textOf(t, canvas, zhDismissed)
}

// ── 6. Act 回 refused → 显示宿主 reason 原文 ────────────────────────────

func TestRefusedShowsHostReasonVerbatim(t *testing.T) {
	const reason = "宿主说：这个动作需要插件授权"
	src := &fakeSource{
		view: cardView(),
		raw:  json.RawMessage(`{"status":"refused","reason":"宿主说：这个动作需要插件授权"}`),
	}
	p, env, tree, canvas := fixture(t, src)
	if err := p.Refresh(context.Background(), "s1"); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	drawCardOnce(t, p, env, canvas, 800, 600)
	clickCard(t, p, env, tree, canvas, 800, 600, "dismiss")

	snap := drawCardOnce(t, p, env, canvas, 800, 600)
	textOf(t, canvas, zhUnavailable) // 官方 unavailable 文案
	textOf(t, canvas, reason)        // 宿主 reason **原文**
	if snap.Card.Width == 0 || snap.Card.Height == 0 {
		t.Fatalf("refused 之后卡片不该消失：%+v", snap.Card)
	}
}

// ── 7. Act 报错 → 显示错误原文，卡片不消失 ──────────────────────────────

func TestActErrorShowsMessageAndKeepsCard(t *testing.T) {
	src := &fakeSource{
		view: cardView(),
		err:  &apiclient.APIError{Code: "internal", Message: "宿主内部错误：卡片运行时不可用"},
	}
	p, env, tree, canvas := fixture(t, src)
	if err := p.Refresh(context.Background(), "s1"); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	drawCardOnce(t, p, env, canvas, 800, 600)
	clickCard(t, p, env, tree, canvas, 800, 600, "dismiss")

	snap := drawCardOnce(t, p, env, canvas, 800, 600)
	textOf(t, canvas, "宿主内部错误：卡片运行时不可用") // 宿主错误原文（APIError.Message）
	if snap.Card.Width == 0 || snap.Card.Height == 0 {
		t.Fatalf("报错之后卡片不该消失：%+v", snap.Card)
	}
}

// ── 8. 过程行：从 journal 原始帧折叠五态 ────────────────────────────────

func toolCallFrame(seq int, callID, arguments string) apiclient.Frame {
	data, _ := json.Marshal(map[string]any{
		"name":      proposeActionName,
		"callId":    callID,
		"arguments": arguments,
	})
	return apiclient.Frame{Seq: seq, Type: "tool/call", Data: data}
}

func toolResultFrame(seq int, callID string, isError bool, text, code string) apiclient.Frame {
	message := map[string]any{
		"toolCallId": callID,
		"isError":    isError,
		"content":    []map[string]any{{"type": "text", "text": text}},
	}
	data := map[string]any{"message": message}
	if code != "" {
		data["error"] = map[string]any{"code": code}
	}
	raw, _ := json.Marshal(data)
	return apiclient.Frame{Seq: seq, Type: "tool/result", Data: raw}
}

func TestProposeRowFoldsStatesFromFrames(t *testing.T) {
	cases := []struct {
		name  string
		state string
		frame []apiclient.Frame
		want  string
	}{
		{"参数还没到齐 → preparing", rowPreparing, []apiclient.Frame{
			toolCallFrame(1, "call-9", ""),
		}, "正在准备卡片"},
		{"调用发出、还没有结果 → running", rowRunning, []apiclient.Frame{
			toolCallFrame(1, "call-9", `{"title":"新建一个包"}`),
		}, "正在提议"},
		{"成功结果 → ok", rowOK, []apiclient.Frame{
			toolCallFrame(1, "call-9", `{"title":"新建一个包"}`),
			toolResultFrame(2, "call-9", false, "卡片已显示", ""),
		}, "已提议"},
		{"失败结果 → error", rowError, []apiclient.Frame{
			toolCallFrame(1, "call-9", `{"title":"新建一个包"}`),
			toolResultFrame(2, "call-9", true, "details 校验失败", ""),
		}, "未显示"},
		{"interrupted 码 → stopped", rowStopped, []apiclient.Frame{
			toolCallFrame(1, "call-9", `{"title":"新建一个包"}`),
			toolResultFrame(2, "call-9", true, "调用被打断", "interrupted"),
		}, "已中断"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := &fakeSource{view: cardView()}
			p, env, _, canvas := fixture(t, src)
			env.Data.Frames = tc.frame
			call, ok := proposeCallOf(tc.frame)
			if !ok {
				t.Fatal("应能折叠出一次 propose_action 调用")
			}
			if call.State != tc.state {
				t.Fatalf("状态=%q，want %q", call.State, tc.state)
			}
			drawRowOnce(t, p, env, canvas, 800, 600)
			textOf(t, canvas, zhRowTitle) // row.title
			textOf(t, canvas, tc.want)
		})
	}
}

// TestProposeRowExpandsReason 覆盖失败行的展开：点行 → 显示宿主结果原文。
func TestProposeRowExpandsReason(t *testing.T) {
	const reason = "details 校验失败：details.name 不是字符串"
	frames := []apiclient.Frame{
		toolCallFrame(1, "call-9", `{"title":"新建一个包"}`),
		toolResultFrame(2, "call-9", true, reason, ""),
	}
	p, env, tree, canvas := fixture(t, &fakeSource{view: cardView()})
	env.Data.Frames = frames

	before := drawRowOnce(t, p, env, canvas, 800, 600)
	if hasText(canvas, reason) {
		t.Fatal("未展开时不该显示结果正文")
	}
	if before.Reason.Width != 0 {
		t.Fatalf("未展开时不该有 reason 区：%+v", before.Reason)
	}
	after := clickRow(t, p, env, tree, canvas, 800, 600, "propose-row")
	textOf(t, canvas, reason)
	if after.Reason.Width == 0 || after.Reason.Height == 0 {
		t.Fatalf("展开后应有 reason 区：%+v", after.Reason)
	}
	// `.reason{ padding:12px }`（OFF:ProposeRow.module.css:5）：常量必须来自官方。
	if reasonPad != 12 || reasonMarginY != 8 || reasonFontSize != 12 {
		t.Fatalf("reason 几何常量与 ProposeRow.module.css:5 不一致：pad=%v marginY=%v font=%v",
			reasonPad, reasonMarginY, reasonFontSize)
	}
}
