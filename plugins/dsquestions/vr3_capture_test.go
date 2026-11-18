//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计（dsquestions）：**空态 / 选中态 / 真回传 /
// 提交中 / 禁用 / 溢出与滚动**。
//
// 前两轮只看过静态外观，这一轮全部用**真指针**（`session.Move` / `session.Click`）
// 打在真命中区上，再读下一帧的真像素与真命中区。逐条对着官方
// `QuestionComposer.module.css`（`tmp/asar-css/149__QuestionComposer.module.css.mjs.css`）
// 与 `ui-user-questions/client.js` 的具体值。
//
// 关键官方依据（行号见每次断言旁边的注释）：
//
//	.card{max-height:min(60vh,520px);max-width:var(--dsh-chat-content-width);
//	      border-radius:radius-xl;background:specific-input-major;padding:0 0 10px}
//	.frame{padding:6px … 10px}
//	.header{padding:20px 16px 0 24px}  .eyebrow{font-size:11px;line-height:16px}
//	.title{font-size:16px;line-height:22px}
//	.options{gap:1px;margin:8px 0 0;padding:4px 12px}
//	.option{min-height:40px;padding:8px 12px 8px 8px;gap:8px;border:1px solid transparent}
//	.option:hover:not(:disabled),.optionSelected{background:interactive-bg-hover}
//	.optionSelected{border-color:border-l2}      ← 只有单选（client.js:1093 逐字：
//	                     `selected && question.multiSelect !== true && optionSelected`）
//	.optionLabel{font-size:14px;line-height:24px}  .description{font-size:14px;line-height:24px}
//	.number{width:20px;height:20px;font-size:12px;line-height:18px;margin-top:2px}
//	.checkbox:before{width:14px;height:14px;border:.5px solid border-l4}
//	.checkboxChecked:before{border-color:label-primary;background:label-primary}
//	.footer{margin-top:12px;padding:0 10px 0 18px;gap:12px}
//	.pager{gap:6px}  .progress{font-size:14px;line-height:24px;padding:0 4px}
//	.iconButton{width:24px;height:24px;border-radius:999px}
//	.body{overflow-y:auto;min-height:0;flex:auto}
//
// 截图落 `$VR3_CAPTURE_DIR`。
package dsquestions

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const (
	vr3QuestionsTopOffset    float32 = 120
	vr3QuestionsBelowHeight  float32 = 98
	vr3QuestionsBelowID              = "vr3-below-marker"
	vr3QuestionsBelowLabel           = "vr3-below"
	vr3QuestionsMainWidth    float32 = 902
	vr3QuestionsWindowWidth          = 1182
	vr3QuestionsWindowHeight         = 832
)

// vr3AnswerSpy 是一次真实 `Actions.AnswerQuestion` 调用的记录（回传的唯一证据）。
type vr3AnswerSpy struct {
	mu   sync.Mutex
	call []vr3AnswerCall
}

type vr3AnswerCall struct {
	callID string
	answer apiclient.QuestionAnswer
}

func (s *vr3AnswerSpy) record(callID string, answer apiclient.QuestionAnswer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.call = append(s.call, vr3AnswerCall{callID: callID, answer: answer})
}

func (s *vr3AnswerSpy) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.call)
}

func (s *vr3AnswerSpy) last() (vr3AnswerCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.call) == 0 {
		return vr3AnswerCall{}, false
	}
	return s.call[len(s.call)-1], true
}

// vr3QuestionsHarness 是一帧可换数据、可点、可滚的官方座位
// （`conversation.composer`，official 档位）。
type vr3QuestionsHarness struct {
	t           *testing.T
	p           *Plugin
	session     *headless.Session
	invalidator app.Invalidator
	tree        *gfx.Tree
	feat        gfx.Scope
	theme       *pluginapi.Theme
	data        pluginapi.Snapshot
	sent        []string
	spy         *vr3AnswerSpy
	// hold 非 nil 时 `AnswerQuestion` 阻塞在它上面，用来拍「提交中」那一帧。
	hold chan struct{}
}

func vr3QuestionsDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("VR3_CAPTURE_DIR")
	if base == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

// vr3AskMessage 造一条真实的 `ask_user_question` 工具消息（与宿主投影同形）。
func vr3AskMessage(t *testing.T, questions []Question) apiclient.Message {
	t.Helper()
	payload := map[string]any{"questions": questions}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return apiclient.Message{Role: "tool", Tool: askTool, CallID: "call-vr3", Args: string(raw)}
}

func vr3OpenQuestions(t *testing.T, width, height int, msgs []apiclient.Message) *vr3QuestionsHarness {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true) // official 档位：`composerSeatLive()` 真
	kernel := uikernel.New("vr3-questions")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	p := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatalf("装配 dsquestions 失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("vr3-questions")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})

	h := &vr3QuestionsHarness{
		t: t, p: p, tree: tree, feat: feat, theme: theme,
		spy:  &vr3AnswerSpy{},
		data: pluginapi.Snapshot{Cur: "s-vr3", Msgs: msgs},
	}
	config := app.Config{
		Title: "vr3-questions", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &h.invalidator,
	}
	// 官方 `.body{overflow-y:auto}` 的真滚动：把滚轮交给页自己的 ScrollArea。
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult { return view.Wheel(event) }
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: h.data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			MainWidth: vr3QuestionsMainWidth,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height,
				SidebarWidth: 280, MainWidth: vr3QuestionsMainWidth,
			},
			Has: kernel.Has,
			Act: pluginapi.Actions{
				Send: func(text string) { h.sent = append(h.sent, text) },
				AnswerQuestion: func(callID string, answer apiclient.QuestionAnswer) (bool, error) {
					if h.hold != nil {
						<-h.hold
					}
					h.spy.record(callID, answer)
					return true, nil
				},
				Invalidate: func() { h.invalidator.Invalidate() },
			},
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillWidth().FillHeight().Children(func() {
				ui.Row(f).FillWidth().Height(vr3QuestionsTopOffset)
				ui.Column(f).Width(vr3QuestionsMainWidth).Children(func() {
					for _, painter := range kernel.Painters(uikernel.OffConvComposer) {
						if painter.Draw != nil {
							painter.Draw(f, env)
						}
					}
					// 下方占位：卡（加载/禁用/隐藏）占多少版面，用它的 y 量。
					ui.Row(f).FillWidth().Height(vr3QuestionsBelowHeight).Radius(28).
						Background(theme.BgLayer2).
						Hit(tree.ID(feat, vr3QuestionsBelowID), vr3QuestionsBelowLabel)
				})
			})
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	h.session = session
	h.pump()
	h.pump()
	return h
}

func (h *vr3QuestionsHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

// settle 画到条件成立（有界），用于异步回传（`AnswerQuestion` 跑在 goroutine 里）。
func (h *vr3QuestionsHarness) settle(cond func() bool) {
	h.t.Helper()
	for i := 0; i < 400; i++ {
		if cond() {
			return
		}
		h.pump()
	}
	if !cond() {
		h.t.Fatalf("等待条件超时；texts=%q", h.session.Texts())
	}
}

// hit 取某个交互 id 的真命中区（没画出来就是失败）。
func (h *vr3QuestionsHarness) hit(name string) gfx.Hit {
	h.t.Helper()
	id := h.tree.ID(h.feat, name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit
		}
	}
	h.t.Fatalf("没有命中区 %q（元素没画出来或没挂 Hit）；hits=%d", name, len(h.session.Snapshot().Hits))
	return gfx.Hit{}
}

func (h *vr3QuestionsHarness) hasHit(name string) bool {
	id := h.tree.ID(h.feat, name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return true
		}
	}
	return false
}

// click 走**真指针**：先把指针移进去（hover 态），再按下抬起，然后画两帧
// （viva 的点击在下一帧才派发到元素上，选中态再晚一帧才在像素上出现）。
func (h *vr3QuestionsHarness) click(name string) {
	h.t.Helper()
	hit := h.hit(name)
	x := hit.Bounds.X + hit.Bounds.Width/2
	y := hit.Bounds.Y + hit.Bounds.Height/2
	if err := h.session.Move(x, y); err != nil {
		h.t.Fatal(err)
	}
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

// hover 只移动指针（不点），用来验 hover 态。
func (h *vr3QuestionsHarness) hover(name string) {
	h.t.Helper()
	hit := h.hit(name)
	if err := h.session.Move(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

// moveAway 把指针移出所有元素（清掉 hover 态，否则 hover 背景会污染选中态断言）。
func (h *vr3QuestionsHarness) moveAway() {
	h.t.Helper()
	if err := h.session.Move(2, 2); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

func (h *vr3QuestionsHarness) below() gfx.Rect {
	h.t.Helper()
	id := h.tree.ID(h.feat, vr3QuestionsBelowID)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit.Bounds
		}
	}
	h.t.Fatalf("没有下方占位命中区")
	return gfx.Rect{}
}

// card 返回本帧提问卡的矩形（`.card`：radius-xl + specific-input-major 底）。
func (h *vr3QuestionsHarness) card() (gfx.Rect, bool) {
	for _, primitive := range h.session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive {
			continue
		}
		if primitive.Color == h.theme.InputMajor && primitive.Radius == h.theme.Radii.XL &&
			primitive.Bounds.Width > 100 && primitive.Bounds.Height > 60 {
			return primitive.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// rectAt 找"覆盖 point 且颜色 = color"的矩形（背景态断言用）。
func (h *vr3QuestionsHarness) rectAt(point gfx.Point, color gfx.Color) (gfx.Rect, bool) {
	for _, primitive := range h.session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Color != color {
			continue
		}
		b := primitive.Bounds
		if point.X >= b.X && point.X <= b.X+b.Width && point.Y >= b.Y && point.Y <= b.Y+b.Height {
			return b, true
		}
	}
	return gfx.Rect{}, false
}

func (h *vr3QuestionsHarness) label(value string) (app.Label, bool) {
	for _, label := range h.session.Snapshot().Labels {
		if label.Value == value {
			return label, true
		}
	}
	return app.Label{}, false
}

func (h *vr3QuestionsHarness) capture(name string) string {
	h.t.Helper()
	path := filepath.Join(vr3QuestionsDir(h.t), name)
	if err := h.session.Screenshot(path); err != nil {
		h.t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		h.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	h.t.Logf("VR3_QUESTIONS_CAPTURE path=%s bytes=%d", path, info.Size())
	return path
}

// bodyDiagnostic 判"提问卡正文是否真的进了像素"，返回空串 = 正常。
//
// 为什么单列一条：`.card` 的正文挂在 `ui.Scroll(...).Grow(1).MinHeight(0)` 上，
// 而它的祖先链（card 的 Column、内层 Clip Column）都是**内容自适应高度**。
// viva 里 ScrollArea 的 flex basis 是 0（不是内容高），于是在内容自适应的父级里
// `Grow(1)` 拿不到任何余量 → 正文高度 0 → 被 `.Clip()` 全裁掉：
//
//	Texts（裁前）有全部正文，Labels（真画出来的）一条都没有，选项连命中区都没有。
//
// 官方 `.card{max-height:min(60vh,520px)}` + `.body{flex:auto;min-height:0;overflow-y:auto}`
// 的语义是"正文按内容高，超了就滚动"，不是"正文消失"。
func (h *vr3QuestionsHarness) bodyDiagnostic(card gfx.Rect) string {
	snapshot := h.session.Snapshot()
	drawn := map[string]bool{}
	for _, label := range snapshot.Labels {
		drawn[label.Value] = true
	}
	painted := map[string]bool{}
	for _, text := range snapshot.Texts {
		painted[text.Value] = true
	}
	missing := make([]string, 0, 3)
	for _, want := range []string{
		"要不要现在就把 DSH 的宿主换成 Go 版本？", "现在就换", "一次性替换，旧 Node 宿主退役",
	} {
		if painted[want] && !drawn[want] {
			missing = append(missing, want)
		}
	}
	if !h.hasHit("question-option-0") {
		missing = append(missing, "question-option-0 命中区")
	}
	if len(missing) == 0 {
		return ""
	}
	return "提问卡正文整块没进像素（被裁成 0 高）：" + strings.Join(missing, " / ") +
		"。\n  卡矩形=" + fmtRect(card) + "（官方 `.card{max-height:min(60vh,520px)}` 下，正文该占满卡内剩余高度并可滚动）" +
		"\n  已画文字=" + itoa(len(snapshot.Labels)) + " 条，裁前文字=" + itoa(len(snapshot.Texts)) + " 条" +
		"\n  根因形状：`ui.Scroll(...).Grow(1).MinHeight(0)` 的父链是内容自适应高度；viva 里 ScrollArea 的 flex basis = 0，" +
		"最小复现（36 高 header + Scroll(3×40) + 48 高 footer，列 MaxHeight 499）实测 scrollH=0、cardH=84；" +
		"把同一个 Scroll 换成 `.MaxHeight(300)` 实测 scrollH=120、cardH=204。"
}

func fmtRect(r gfx.Rect) string {
	return "x=" + fmtFloat(r.X) + " y=" + fmtFloat(r.Y) + " w=" + fmtFloat(r.Width) + " h=" + fmtFloat(r.Height)
}

func fmtFloat(v float32) string {
	return itoa(int(math.Round(float64(v))))
}

// vr3QuestionsClose 是像素级比较（描边亚像素对齐，容差按调用点给）。
func vr3QuestionsClose(got, want, tolerance float32) bool {
	return math.Abs(float64(got-want)) <= float64(tolerance)
}

// vr3SingleQuestion 是一条典型的单选题（3 个选项，带描述）。
func vr3SingleQuestion() []Question {
	return []Question{{
		ID: "q1", Header: "构建", Question: "要不要现在就把 DSH 的宿主换成 Go 版本？",
		Detail: "换掉宿主不影响界面插件。",
		Options: []Option{
			{Label: "现在就换", Description: "一次性替换，旧 Node 宿主退役"},
			{Label: "先小范围试", Description: "只在一个会话上验证"},
			{Label: "暂时不动", Description: "保持现状，等下一版"},
			// 第 4 条：超长描述，用来钉官方 `.description{max-height:calc(2lh + 2px)}` 的两行截断。
			{Label: "先看文档", Description: strings.Repeat("这是一段很长的选项描述用来验两行截断", 8)},
		},
	}}
}

// TestVR3QuestionsCardStates 覆盖空态、单选选中态、真回传、多选勾选、自定义输入。
func TestVR3QuestionsCardStates(t *testing.T) {
	// ── ① 空态：没有待答问题 ───────────────────────────────────────────────
	// 官方是 selector 路由（`ui-user-questions/src/client/index.ts:383-395`
	// `select: pendingInteraction instanceof PendingQuestion ? … : null`）：
	// 没有 pending 就**不接管**，卡片与它的 padding 一起不存在。
	empty := vr3OpenQuestions(t, vr3QuestionsWindowWidth, vr3QuestionsWindowHeight, nil)
	if _, ok := empty.card(); ok {
		t.Fatal("没有待答问题却画出了提问卡")
	}
	if empty.session.HasText(questionSkipText) || empty.session.HasText("1 / 1") {
		t.Fatalf("空态画出了卡片内容；texts=%q", empty.session.Texts())
	}
	emptyBelow := empty.below()
	if !vr3QuestionsClose(emptyBelow.Y, vr3QuestionsTopOffset, 0.01) {
		t.Fatalf("空态下方占位 y=%.2f，want %.2f（不许留卡片占位）", emptyBelow.Y, vr3QuestionsTopOffset)
	}
	empty.capture("questions-empty-1182x832.png")
	empty.session.Close()

	// ── ② 单选题：官方几何逐条 ────────────────────────────────────────────
	h := vr3OpenQuestions(t, vr3QuestionsWindowWidth, vr3QuestionsWindowHeight,
		[]apiclient.Message{vr3AskMessage(t, vr3SingleQuestion())})
	defer h.session.Close()

	card, ok := h.card()
	if !ok {
		t.Fatalf("没画出提问卡；texts=%q", h.session.Texts())
	}
	// `.frame{padding:6px … 10px}`：卡顶 = 座位顶 + 6。
	if !vr3QuestionsClose(card.Y, vr3QuestionsTopOffset+questionFramePadTop, 0.5) {
		t.Errorf("卡顶 y=%.2f，want %.2f（`.frame{padding:6px …}`）", card.Y, vr3QuestionsTopOffset+questionFramePadTop)
	}
	// `.card{max-width:var(--dsh-chat-content-width)}` = clamp(680, 902×.64, 920) = 680。
	if !vr3QuestionsClose(card.Width, 680, 0.5) {
		t.Errorf("卡宽 %.2f，want 680（`--dsh-chat-content-width` = clamp(680, 主列×0.64, 920)）", card.Width)
	}
	if !vr3QuestionsClose(card.X, (vr3QuestionsMainWidth-680)/2, 0.5) {
		t.Errorf("卡 x=%.2f，want %.2f（主列内居中）", card.X, (vr3QuestionsMainWidth-680)/2)
	}

	// `.eyebrow{font-size:11px}` + `.header{padding:20px 16px 0 24px}`。
	eyebrow, ok := h.label("构建")
	if !ok {
		t.Fatalf("没画出 eyebrow（官方 `.eyebrow` 画 question.header）；labels=%v", h.session.Texts())
	}
	if eyebrow.FontSize != questionEyebrowFont {
		t.Errorf("eyebrow 字号 %.1f，want %.1f（`.eyebrow{font-size:11px}`）", eyebrow.FontSize, questionEyebrowFont)
	}
	if !vr3QuestionsClose(eyebrow.Bounds.X, card.X+questionHeaderPadLeft, 0.5) {
		t.Errorf("eyebrow x=%.2f，want %.2f（`.header{padding-left:24px}`）",
			eyebrow.Bounds.X, card.X+questionHeaderPadLeft)
	}
	// `.progress{font-size:14px;line-height:24px}` + `.footer{margin-top:12px;padding:0 10px 0 18px}`。
	progress, ok := h.label("1 / 1")
	if !ok {
		t.Fatalf("没画出进度 %q；labels=%v", "1 / 1", h.session.Texts())
	}
	if progress.FontSize != questionProgressFont {
		t.Errorf("进度字号 %.1f，want %.1f（`.progress{font-size:14px}`）", progress.FontSize, questionProgressFont)
	}
	// `.iconButton{width:24px;height:24px;border-radius:999px}`。
	prev := h.hit("question-prev")
	if !vr3QuestionsClose(prev.Bounds.Width, questionIconButtonSize, 0.5) ||
		!vr3QuestionsClose(prev.Bounds.Height, questionIconButtonSize, 0.5) {
		t.Errorf("上一题按钮 %.2fx%.2f，want %.0fx%.0f（`.iconButton{width:24px;height:24px}`）",
			prev.Bounds.Width, prev.Bounds.Height, questionIconButtonSize, questionIconButtonSize)
	}
	// 未作答时「提交」是禁用态：官方 `disabled: busy || !answered || (last && !canSubmit)`，
	// 实现用的是 `opacity:.4`（planReviewDisabled），所以文字 alpha = 0.4。
	submitText, ok := h.label(questionSubmitText)
	if !ok {
		t.Fatalf("没画出提交按钮文案 %q；labels=%v", questionSubmitText, h.session.Texts())
	}
	if alpha := submitText.Color.Alpha; math.Abs(float64(alpha-0.4)) > 0.02 {
		t.Errorf("未作答时提交按钮文字 alpha=%.2f，want 0.4（禁用态 `.option:disabled`/Button disabled 的 opacity）", alpha)
	}
	// ── 卡的**正文**（title/detail/options/custom 都在 `ui.Scroll` 里）必须真的画出来 ──
	// 这是本文件最重要的一条：正文一旦被裁成 0 高，后面的每一条都对不上，
	// 所以这里 fail-fast，把判据一次说清（不要后面十几条报同一个根因）。
	// 先留一张图：这张图就是"正文被裁掉"的样子（只剩 header 的 eyebrow + footer）。
	h.capture("questions-body-collapsed-1182x832.png")
	if msg := h.bodyDiagnostic(card); msg != "" {
		t.Fatalf("%s", msg)
	}

	// `.title{font-size:16px;line-height:22px}`。
	title, ok := h.label("要不要现在就把 DSH 的宿主换成 Go 版本？")
	if !ok {
		t.Fatalf("没画出题目标题；labels=%v", h.session.Texts())
	}
	if title.FontSize != questionTitleFont {
		t.Errorf("标题字号 %.1f，want %.1f（`.title{font-size:16px}`）", title.FontSize, questionTitleFont)
	}

	// `.options{padding:4px 12px;gap:1px;margin:8px 0 0}`。
	opt0 := h.hit("question-option-0")
	opt1 := h.hit("question-option-1")
	if !vr3QuestionsClose(opt0.Bounds.X, card.X+questionOptionsPadX, 0.5) {
		t.Errorf("第一个选项 x=%.2f，want %.2f（`.options{padding:0 12px}`）",
			opt0.Bounds.X, card.X+questionOptionsPadX)
	}
	if !vr3QuestionsClose(opt0.Bounds.Width, card.Width-2*questionOptionsPadX, 0.5) {
		t.Errorf("选项宽 %.2f，want %.2f（卡宽 - 2×12）", opt0.Bounds.Width, card.Width-2*questionOptionsPadX)
	}
	// `.option{min-height:40px}`。
	if opt0.Bounds.Height < questionOptionMinHeight {
		t.Errorf("选项高 %.2f < 官方 min-height %.1f", opt0.Bounds.Height, questionOptionMinHeight)
	}
	// `.options{gap:1px}`。
	if gap := opt1.Bounds.Y - (opt0.Bounds.Y + opt0.Bounds.Height); !vr3QuestionsClose(gap, questionOptionsGap, 0.5) {
		t.Errorf("选项间距 %.2f，want %.1f（`.options{gap:1px}`）", gap, questionOptionsGap)
	}
	// `.optionLabel{font-size:14px;line-height:24px}`。
	optLabel, ok := h.label("现在就换")
	if !ok {
		t.Fatalf("没画出选项文案；labels=%v", h.session.Texts())
	}
	if optLabel.FontSize != questionOptionLabelFont {
		t.Errorf("选项字号 %.1f，want %.1f（`.optionLabel{font-size:14px}`）", optLabel.FontSize, questionOptionLabelFont)
	}
	// rc.2 `.description{font-size:13px;font-weight:400;line-height:20px;
	//   max-height:calc(2lh + 2px);display:block;overflow:hidden}`（asar 61568466 附近）。
	desc, ok := h.label("一次性替换，旧 Node 宿主退役")
	if !ok {
		t.Fatalf("没画出选项描述；labels=%v", h.session.Texts())
	}
	if desc.FontSize != questionOptionDescFont || questionOptionDescFont != 13 {
		t.Errorf("选项描述字号 %.1f，want 13（rc.2 `.description{font-size:13px;line-height:20px}`）", desc.FontSize)
	}
	// `.description{…max-height:calc(2lh + 2px);overflow:hidden}`：描述最多两行。
	//
	// 平台说明（已登记为 derived）：rc.2 把 [label+badge] 与 description 放在同一个
	// `optionCopy{flex-wrap:wrap;align-items:baseline;gap:0 6px}` 里，放得下就并排；
	// viva v0.6.4 的 flex **没有容器换行**（`flagWrap` 只服务文本 maxLines），
	// 这一半几何在本仓不可表达。这里断言官方有明确数值、且可表达的那一半：
	// 超长描述必须被压到两行，而不是写多长就长多高。
	longOpt := h.hit("question-option-3")
	descLimit := questionOptionLabelLine + 2*questionOptionDescLine + 2*questionOptionPadTop + 1
	if longOpt.Bounds.Height > descLimit {
		t.Errorf("超长描述把选项撑到 %.2f 高，want ≤ %.2f（`.description{max-height:calc(2lh + 2px)}`："+
			"24 标签行 + 2×20 描述行 + 上下各 8 内边距）", longOpt.Bounds.Height, descLimit)
	}

	// `.number{width:20px;height:20px;border-radius:radius-xs;background:bg-overlay}`。
	if _, ok := h.rectAt(gfx.Point{X: opt0.Bounds.X + questionOptionPadX + questionIndicatorSeat/2,
		Y: opt0.Bounds.Y + questionOptionPadTop + questionIndicatorMarginTop + questionIndicatorSeat/2},
		h.theme.BgOverlay); !ok {
		t.Error("选项序号底（20×20 radius-xs bg-overlay）没画出来")
	}
	h.moveAway()
	h.capture("questions-single-1182x832.png")

	// ── ③ hover 态：官方 `.option:hover:not(:disabled){background:interactive-bg-hover}` ──
	h.hover("question-option-2")
	opt2 := h.hit("question-option-2")
	if _, ok := h.rectAt(gfx.Point{X: opt2.Bounds.X + opt2.Bounds.Width - 8, Y: opt2.Bounds.Y + opt2.Bounds.Height/2},
		h.theme.Hover); !ok {
		t.Error("悬停选项没有出现 interactive-bg-hover 背景（官方 `.option:hover:not(:disabled)`）")
	}
	h.moveAway()
	if _, ok := h.rectAt(gfx.Point{X: opt2.Bounds.X + opt2.Bounds.Width - 8, Y: opt2.Bounds.Y + opt2.Bounds.Height/2},
		h.theme.Hover); ok {
		t.Error("指针移开后 hover 背景没有消失")
	}

	// ── ④ 真点击单选：选中态 + 真回传 ────────────────────────────────────
	h.click("question-option-0")
	opt0 = h.hit("question-option-0")
	if _, ok := h.rectAt(gfx.Point{X: opt0.Bounds.X + 4, Y: opt0.Bounds.Y + opt0.Bounds.Height/2},
		h.theme.Hover); !ok {
		t.Error("单选点中之后没有 optionSelected 背景（官方 `.optionSelected{background:interactive-bg-hover}`）")
	}
	h.capture("questions-single-selected-1182x832.png")
	// 作答后提交按钮解除禁用（alpha 回到 1）。
	h.moveAway()
	submitText, ok = h.label(questionSubmitText)
	if !ok {
		t.Fatalf("选中之后提交按钮不见了；labels=%v", h.session.Texts())
	}
	if alpha := submitText.Color.Alpha; alpha < 0.95 {
		t.Errorf("已作答后提交按钮文字 alpha=%.2f，want ≈1（官方 disabled 条件里有 `!answered`）", alpha)
	}
	// 真回传：点「提交」必须真的发出 `userQuestions.answer`，且带选中的那一项。
	h.hold = nil
	h.click("question-submit")
	h.settle(func() bool { return h.spy.count() > 0 })
	call, _ := h.spy.last()
	if call.callID != "call-vr3" {
		t.Errorf("回传 callID=%q，want %q（宿主用 CallID 定位那次工具调用）", call.callID, "call-vr3")
	}
	if len(call.answer.Answers) != 1 || call.answer.Answers[0].ID != "q1" {
		t.Fatalf("回传 answers=%+v，want 一条 id=q1", call.answer.Answers)
	}
	if got := call.answer.Answers[0].Selected; len(got) != 1 || got[0] != "现在就换" {
		t.Errorf("回传 selected=%v，want [现在就换]（点的是第一个选项）", got)
	}
	// 提交成功后卡片整块消失（不再等这个问题）。
	h.settle(func() bool { return !h.hasHit("question-submit") })
	h.pump()
	if h.session.HasText(questionSkipText) {
		t.Errorf("提交成功后卡片还挂着；texts=%q", h.session.Texts())
	}
	if back := h.below(); !vr3QuestionsClose(back.Y, vr3QuestionsTopOffset, 0.01) {
		t.Errorf("提交后下方占位 y=%.2f，want %.2f（卡片不许留空白）", back.Y, vr3QuestionsTopOffset)
	}
	h.capture("questions-answered-1182x832.png")

	// ── ⑤ 多选：勾选态在 checkbox 上，选项行**不该**有 optionSelected 背景 ──
	multi := vr3OpenQuestions(t, vr3QuestionsWindowWidth, vr3QuestionsWindowHeight, []apiclient.Message{
		vr3AskMessage(t, []Question{{
			ID: "q-multi", Header: "范围", Question: "要一起改哪些目录？", MultiSelect: true,
			Options: []Option{{Label: "plugins"}, {Label: "internal"}, {Label: "cmd"}},
		}}),
	})
	defer multi.session.Close()
	multi.moveAway()
	multi.click("question-option-0")
	// 指针还压在这一行上（hover 底色是官方 `@media (hover:hover)` 的合法结果），
	// 先移开再判"多选有没有多画 optionSelected 底色"。
	multi.moveAway()
	opt0 = multi.hit("question-option-0")
	// `.checkboxChecked:before{border-color:label-primary;background:label-primary}` + 14×14。
	checkPoint := gfx.Point{
		X: opt0.Bounds.X + questionOptionPadX + questionIndicatorSeat/2,
		Y: opt0.Bounds.Y + questionOptionPadTop + questionIndicatorMarginTop + questionIndicatorSeat/2,
	}
	checkedRect, ok := multi.rectAt(checkPoint, multi.theme.LabelPrimary)
	if !ok {
		t.Fatalf("多选勾上之后 checkbox 没画成 label-primary 底；texts=%q", multi.session.Texts())
	}
	if !vr3QuestionsClose(checkedRect.Width, questionCheckboxSize, 0.5) ||
		!vr3QuestionsClose(checkedRect.Height, questionCheckboxSize, 0.5) {
		t.Errorf("勾选框 %.2fx%.2f，want %.0fx%.0f（`.checkbox:before{width:14px;height:14px}`）",
			checkedRect.Width, checkedRect.Height, questionCheckboxSize, questionCheckboxSize)
	}
	// 官方 `client.js:1093`：`selected && question.multiSelect !== true && optionSelected`
	// ——多选**不加** optionSelected 底色，只有 checkbox 变。这条钉住"没多画一层"。
	if rect, ok := multi.rectAt(gfx.Point{X: opt0.Bounds.X + 4, Y: opt0.Bounds.Y + opt0.Bounds.Height/2},
		multi.theme.Hover); ok {
		t.Errorf("多选选中却给选项行加了 optionSelected 背景 %+v（官方仅单选，client.js:1093）", rect)
	}
	multi.capture("questions-multi-checked-1182x832.png")
	// 再点一次取消勾选。
	multi.click("question-option-0")
	opt0 = multi.hit("question-option-0")
	if _, ok := multi.rectAt(checkPoint, multi.theme.LabelPrimary); ok {
		t.Error("再点一次没有取消勾选（官方 aria-checked 取反复位）")
	}
	multi.capture("questions-multi-unchecked-1182x832.png")

	// ── ⑥ 自定义输入：草稿有内容 → 那一行变成 customRowActive ────────────
	// 官方 `.customRow:focus-within:not(.customRowYielded),.customRowActive{
	//   background:interactive-bg-hover;border-color:border-l2}`。
	multi.p.custom.SetText("全部目录都改")
	multi.pump()
	multi.pump()
	customText, ok := multi.label("全部目录都改")
	if !ok {
		t.Fatalf("自定义输入的内容没画出来；labels=%v", multi.session.Texts())
	}
	if _, ok := multi.rectAt(gfx.Point{X: customText.Bounds.X + customText.Bounds.Width/2,
		Y: customText.Bounds.Y + customText.Bounds.Height/2}, multi.theme.Hover); !ok {
		t.Error("自定义输入有内容时 customRowActive 背景没画（官方 `.customRowActive{background:interactive-bg-hover}`）")
	}
	multi.capture("questions-custom-active-1182x832.png")
}

// TestVR3QuestionsOverflowAndBusy 覆盖选项溢出裁切、真滚动、长选项换行，以及
// 「提交中」的禁用帧。
func TestVR3QuestionsOverflowAndBusy(t *testing.T) {
	many := make([]Option, 0, 12)
	for i := 0; i < 12; i++ {
		many = append(many, Option{Label: "选项 " + itoa(i) + " 号", Description: "第 " + itoa(i) + " 个候选"})
	}
	h := vr3OpenQuestions(t, vr3QuestionsWindowWidth, vr3QuestionsWindowHeight, []apiclient.Message{
		vr3AskMessage(t, []Question{{ID: "q-many", Question: "选一个候选", Options: many}}),
	})
	defer h.session.Close()

	card, ok := h.card()
	if !ok {
		t.Fatalf("没画出提问卡；texts=%q", h.session.Texts())
	}
	// `.card{max-height:min(60vh,520px)}`：832×0.6 = 499.2 < 520 → 内容溢出时卡该被顶到 499.2 并滚动。
	wantHeight := float32(vr3QuestionsWindowHeight) * questionCardViewportRatio
	if msg := h.bodyDiagnostic(card); msg != "" {
		t.Fatalf("12 个选项时：%s\n  卡高=%.2f，官方上限 %.2f（`.card{max-height:min(60vh,520px)}`）",
			msg, card.Height, wantHeight)
	}
	if !vr3QuestionsClose(card.Height, wantHeight, 0.5) {
		t.Errorf("12 个选项时卡高 %.2f，want %.2f（`.card{max-height:min(60vh,520px)}`）",
			card.Height, wantHeight)
	}
	// 溢出部分必须**被裁掉**：任何一个已画文字都不许整行落在卡底之下。
	cardBottom := card.Y + card.Height
	for _, label := range h.session.Snapshot().Labels {
		if label.Bounds.Y >= cardBottom {
			t.Errorf("文字 %q（y=%.2f）整行画到卡底 %.2f 之下（`.card{overflow:hidden}`）",
				label.Value, label.Bounds.Y, cardBottom)
		}
	}
	// 溢出真的发生了：最后一项**没有**被画出来（`Labels` 只装真画出来的文字，
	// `Texts` 是裁前的，不能用来判可见性），而第一项画出来了。
	if _, ok := h.label("选项 0 号"); !ok {
		t.Fatalf("视口里连第一项都没画出来；labels=%d", len(h.session.Snapshot().Labels))
	}
	if _, ok := h.label("选项 11 号"); ok {
		t.Fatalf("12 个选项全塞进卡里了（最后一项也画出来了），滚动用例不成立")
	}
	h.capture("questions-many-clipped-1182x832.png")

	// `.body{overflow-y:auto}`：真滚轮必须真把内容滚上来。
	firstBefore, ok := h.label("选项 0 号")
	if !ok {
		t.Fatal("滚动前没有画出的第一项")
	}
	// 先滚一行：第一项**还在视口里**，只是上移（证明滚动真的发生在正文里）。
	scrollBy := func(dy float32) {
		if err := h.session.Scroll(gfx.ScrollEvent{
			X: card.X + card.Width/2, Y: card.Y + card.Height/2, DeltaY: dy,
			Viewport: gfx.Size{Width: vr3QuestionsWindowWidth, Height: vr3QuestionsWindowHeight},
		}); err != nil {
			t.Fatal(err)
		}
		h.pump()
		h.pump()
	}
	// 判据挑中间那一项（选项 0 号贴着视口顶，滚一行就整行出去了，不适合判"上移"）。
	middleBefore, ok := h.label("选项 2 号")
	if !ok {
		t.Fatalf("滚动前没画出中间的选项 2 号；labels=%d", len(h.session.Snapshot().Labels))
	}
	scrollBy(80)
	middleAfter, ok := h.label("选项 2 号")
	if !ok {
		t.Fatalf("滚一行就把中间的选项 2 号裁掉了；labels=%d", len(h.session.Snapshot().Labels))
	}
	if !(middleAfter.Bounds.Y < middleBefore.Bounds.Y-1) {
		t.Errorf("滚轮没有滚动卡内选项：选项 2 号 y %.2f → %.2f（`.body{overflow-y:auto}`）",
			middleBefore.Bounds.Y, middleAfter.Bounds.Y)
	}
	_ = firstBefore
	// 再滚到底：最后一项必须**真被滚出来**（滚动范围覆盖整段内容，不是只裁不滚）。
	scrollBy(1200)
	if _, ok := h.label("选项 11 号"); !ok {
		t.Errorf("滚到底后最后一项仍没被画出来（滚动范围不足或裁切过早）；labels=%d",
			len(h.session.Snapshot().Labels))
	}
	h.capture("questions-many-scrolled-1182x832.png")

	// ── 长选项文案：官方 `.optionLine{flex-wrap:wrap}` 且 `.optionLabel` 没有
	// text-overflow —— 官方是**换行**，不是省略号。
	long := vr3OpenQuestions(t, vr3QuestionsWindowWidth, vr3QuestionsWindowHeight, []apiclient.Message{
		vr3AskMessage(t, []Question{{
			ID: "q-long", Question: "这一条特别长", Options: []Option{{
				Label: strings.Repeat("很长的选项文案不能省略只能换行", 4),
			}},
		}}),
	})
	defer long.session.Close()
	longOpt := long.hit("question-option-0")
	if longOpt.Bounds.Height <= questionOptionMinHeight {
		t.Errorf("长选项高 %.2f，want > %.1f（官方 `.optionLine{flex-wrap:wrap}` 换行撑高）",
			longOpt.Bounds.Height, questionOptionMinHeight)
	}
	for _, label := range long.session.Snapshot().Labels {
		if strings.HasSuffix(label.Value, "…") && strings.Contains(label.Value, "很长的选项文案") {
			t.Errorf("长选项被省略成 %q（官方 `.optionLabel` 无 text-overflow，靠 flex-wrap 换行）", label.Value)
		}
	}
	long.capture("questions-long-label-1182x832.png")

	// ── 「提交中」：busy 态文案 + 选项点不动 ──────────────────────────────
	hold := make(chan struct{})
	busy := vr3OpenQuestions(t, vr3QuestionsWindowWidth, vr3QuestionsWindowHeight, []apiclient.Message{
		vr3AskMessage(t, vr3SingleQuestion()),
	})
	defer busy.session.Close()
	busy.hold = hold
	busy.moveAway()
	busy.click("question-option-0")
	busy.click("question-submit")
	// `busy` 在 `AnswerQuestion` 返回前一直是真：这一帧的提交按钮文案是 questionSubmittingText。
	busy.settle(func() bool { return busy.session.HasText(questionSubmittingText) })
	if !busy.session.HasText(questionSubmittingText) {
		t.Fatalf("提交中没画 %q；texts=%q", questionSubmittingText, busy.session.Texts())
	}
	busy.capture("questions-submitting-1182x832.png")
	// 提交中点别的选项必须**没有回传**（官方 `.option:disabled{cursor:default}` + 不挂 onClick）。
	sentBefore := len(busy.sent)
	busy.click("question-option-2")
	if len(busy.sent) != sentBefore {
		t.Errorf("提交中点了选项还回传了 %v 次（应当禁用）", len(busy.sent)-sentBefore)
	}
	if _, ok := busy.rectAt(gfx.Point{
		X: busy.hit("question-option-2").Bounds.X + 4,
		Y: busy.hit("question-option-2").Bounds.Y + busy.hit("question-option-2").Bounds.Height/2,
	}, busy.theme.Hover); ok {
		t.Error("提交中选项仍画成选中态（busy 时不处理点击，也不该换选中项）")
	}
	close(hold)
	busy.settle(func() bool { return busy.spy.count() > 0 })
	busy.settle(func() bool { return !busy.hasHit("question-submit") })
}
