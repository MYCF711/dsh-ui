//go:build darwin || windows || linux

// cot_translation_visual_test.go —— #63 思考译文的**真实绘制 + 真实点击**证据。
//
// 单测（cot_translation_test.go）只看状态机；这里走真 Viva：展开思考 → 看到
// 「查看译文」→ 点它 → pump 到译文出现 → 再点「查看原文」→ 回到原文。
// 失败态与"宿主没接就不画"两条也各走一遍真实绘制。
//
// 为什么每一步都 pump 到条件成立而不是"pump 两帧再看"：请求的推进是**帧驱动**的
// （每帧最多一次迁移，见 cotAdvance），固定帧数会在慢机器上假红。
package dsconversation

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const (
	cotShotSession = "sess-cot-translation"
	cotOriginText  = "先看目录结构。\n\n再对齐会话加载路径。"
	cotTranslated  = "T(先看目录结构。)\n\nT(再对齐会话加载路径。)"
)

// cotShotData 造一条 user → thinking → assistant 的真实形状快照。
func cotShotData() pluginapi.Snapshot {
	msgs := []apiclient.Message{
		{Seq: 1, Role: "user", Text: "看一下仓库。"},
		{Seq: 2, Role: "thinking", Text: cotOriginText},
		{Seq: 3, Role: "assistant", Text: "看完了。"},
	}
	data := pluginapi.Snapshot{Cur: cotShotSession, Msgs: msgs, Total: len(msgs)}
	base := int64(1762222444000)
	data.MessageMeta = map[string]pubapi.MessageMetadata{
		pubapi.MessageKey(data.Cur, msgs[0], 0): {Time: base},
		pubapi.MessageKey(data.Cur, msgs[1], 1): {Time: base + 5000},
		pubapi.MessageKey(data.Cur, msgs[2], 2): {Time: base + 7000},
	}
	return data
}

// cotHarness 是带真 Act 的无头窗口（与 reasoningHarness 同一个装配方式）。
type cotHarness struct {
	t           *testing.T
	p           *Plugin
	data        pluginapi.Snapshot
	tree        *gfx.Tree
	feat        gfx.Scope
	session     *headless.Session
	invalidator *app.Invalidator
	act         pluginapi.Actions
	// canvas 是最近一帧的绘制画布（Paint 里写）：要重排 Markdown 的测试用它，
	// 因为 gfx.Canvas 只有壳会造。
	canvas *gfx.Canvas
}

func openCotHarness(t *testing.T, width, height int, data pluginapi.Snapshot, act pluginapi.Actions) *cotHarness {
	t.Helper()
	p := newStreamingMarkdownTestPlugin()
	theme := p.theme
	tree := &gfx.Tree{}
	feat := tree.Feature("cot-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "cot-headless", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	var lastCanvas *gfx.Canvas
	config.Paint = func(canvas *gfx.Canvas, current ui.Size) {
		bounds := gfx.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas, Act: act,
			Bounds: bounds, MainWidth: current.Width,
		}
		lastCanvas = canvas
		view.Draw(canvas, bounds, func(f ui.C) { p.draw(f, env) })
		p.afterDraw(canvas, env)
	}
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult { return view.Wheel(event) }
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &cotHarness{t: t, p: p, data: data, tree: tree, feat: feat, session: session, invalidator: &invalidator, act: act, canvas: lastCanvas}
	h.pump()
	h.pump()
	return h
}

func (h *cotHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

// pumpUntil 每帧推进一次直到条件成立；不成立就把当前所有文本贴出来。
func (h *cotHarness) pumpUntil(what string, cond func() bool) {
	h.t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		h.pump()
	}
	h.t.Fatalf("等不到「%s」；当前文本=%q", what, h.session.Texts())
}

func (h *cotHarness) close() { h.session.Close() }

// hasLine 判"某一整行文本被画出来"。
//
// 必须按整行而不是子串：折叠行标题是「思考 · 想了 5 秒 · 先看目录结构。」，
// 用 HasText（子串）判正文那一段永远为真，测试就会假绿。
func (h *cotHarness) hasLine(want string) bool {
	for _, value := range h.session.Texts() {
		if value == want {
			return true
		}
	}
	return false
}

// click 按标签找命中区并点它（Button 的命中标签就是按钮文案，见 viva
// ui/elements.go:98-102 —— `c.f.Button(id, label)`）。
func (h *cotHarness) click(label string) {
	h.t.Helper()
	for _, hit := range h.session.Snapshot().Hits {
		if hit.Label != label {
			continue
		}
		x := hit.Bounds.X + hit.Bounds.Width/2
		y := hit.Bounds.Y + hit.Bounds.Height/2
		if err := h.session.Press(x, y); err != nil {
			h.t.Fatal(err)
		}
		if err := h.session.Release(x, y); err != nil {
			h.t.Fatal(err)
		}
		h.pump()
		return
	}
	h.t.Fatalf("找不到「%s」的命中区；hits=%+v texts=%q", label, h.session.Snapshot().Hits, h.session.Texts())
}

func (h *cotHarness) screenshot(dir, name string, width, height int) {
	h.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%dx%d.png", name, width, height))
	if err := h.session.Screenshot(path); err != nil {
		h.t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		h.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	h.t.Logf("COT_CAPTURE path=%s size=%dx%d bytes=%d", path, width, height, info.Size())
}

// cotAutoTranslator 是"立即回包"的假宿主：发起时就把终态放进 results，
// 下一帧的 take 就能拿到。failFirst 让**第一次**请求失败，用来走失败态与重试。
type cotAutoTranslator struct {
	calls     []cotCall
	pending   map[string]cotCall
	results   map[string]pluginapi.CotTranslationResult
	seq       int
	failFirst bool
	failed    bool
}

func newCotAutoTranslator() *cotAutoTranslator {
	return &cotAutoTranslator{
		pending: map[string]cotCall{},
		results: map[string]pluginapi.CotTranslationResult{},
	}
}

func (a *cotAutoTranslator) actions() pluginapi.Actions {
	return pluginapi.Actions{
		CotTranslationLimits: func() (apiclient.CotTranslationLimits, bool) {
			return apiclient.CotTranslationLimits{
				MaxTextChars: 100000,
				Preferences: apiclient.CotTranslationPreferences{
					Provider:       apiclient.CotTranslationProviderOfficial,
					TargetLanguage: "auto",
				},
				AvailableProviders: []string{apiclient.CotTranslationProviderOfficial},
			}, true
		},
		CotTranslationTranslate: func(sessionID, text, targetLanguage, provider string) string {
			a.seq++
			job := fmt.Sprintf("auto-%d", a.seq)
			call := cotCall{sessionID: sessionID, text: text, targetLanguage: targetLanguage, provider: provider}
			a.calls = append(a.calls, call)
			if a.failFirst && !a.failed {
				a.failed = true
				a.results[job] = pluginapi.CotTranslationResult{Code: "not_implemented", Message: "宿主没有 cotTranslation.translate"}
				return job
			}
			a.results[job] = pluginapi.CotTranslationResult{Text: "T(" + text + ")", OK: true}
			return job
		},
		CotTranslationResult: func(job string) (pluginapi.CotTranslationResult, bool) {
			result, ok := a.results[job]
			if ok {
				delete(a.results, job)
			}
			return result, ok
		},
	}
}

// expandThinking 点开那条思考块（与 reasoning_visual_test.go 同一个命中 id）。
func (h *cotHarness) expandThinking(index int) {
	h.t.Helper()
	key := messageRenderKey(&pluginapi.Env{Data: h.data}, h.data.Msgs[index], index)
	id := h.tree.ID(h.feat, "think-"+key)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID != id {
			continue
		}
		x := hit.Bounds.X + hit.Bounds.Width/2
		y := hit.Bounds.Y + hit.Bounds.Height/2
		if err := h.session.Press(x, y); err != nil {
			h.t.Fatal(err)
		}
		if err := h.session.Release(x, y); err != nil {
			h.t.Fatal(err)
		}
		h.pump()
		return
	}
	h.t.Fatalf("思考行没有命中区 id=%v；hits=%+v", id, h.session.Snapshot().Hits)
}

// TestCotTranslationViewSwitches 走完整交互：展开 → 译文 → 原文，并留两张图。
//
// 两个窗口尺寸都跑：译文与原文是**同一处正文**的两态，宽度变化时工具栏不许把
// 正文挤没（这是"两态共用一条折行"的唯一可观察后果）。
func TestCotTranslationViewSwitches(t *testing.T) {
	dir := os.Getenv("COT_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, size := range []struct {
		name          string
		width, height int
	}{{"1182x832", 1182, 832}, {"1920x1032", 1920, 1032}} {
		t.Run(size.name, func(t *testing.T) { runCotViewEvidence(t, size.width, size.height, dir) })
	}
}

func runCotViewEvidence(t *testing.T, width, height int, dir string) {
	translator := newCotAutoTranslator()
	h := openCotHarness(t, width, height, cotShotData(), translator.actions())
	defer h.close()

	h.expandThinking(1)
	// 展开后正文立刻在（还没翻完的段仍是原文——官方 emit() 的同一规则，
	// 所以这一刻正文可能是「译文段 + 原文段」的混合，两种形态都算画出来了）。
	if !h.hasLine("再对齐会话加载路径。") && !h.hasLine("T(再对齐会话加载路径。)") {
		t.Fatalf("展开态没画出思考正文：%q", h.session.Texts())
	}
	h.pumpUntil("查看原文按钮", func() bool { return h.session.HasText(cotLabelShowOriginal) })
	if len(translator.calls) == 0 {
		t.Fatal("画了译文入口却一个请求都没发")
	}
	// 请求参数：`auto` 在**客户端**解析成界面语言（官方 TranslationBody 同一句），
	// provider 沿用 limits 的偏好，sessionId 是当前会话。
	if first := translator.calls[0]; first.targetLanguage != cotUILanguage {
		t.Fatalf("targetLanguage=%q，期望 %q", first.targetLanguage, cotUILanguage)
	} else if first.sessionID != cotShotSession {
		t.Fatalf("sessionID=%q，期望 %q", first.sessionID, cotShotSession)
	} else if first.provider != apiclient.CotTranslationProviderOfficial {
		t.Fatalf("provider=%q，期望沿用 limits 的偏好", first.provider)
	}

	// pump 到两段都翻完（帧驱动：每帧最多一次迁移）。
	h.pumpUntil("译文出现", func() bool {
		return h.hasLine("T(先看目录结构。)") && h.hasLine("T(再对齐会话加载路径。)")
	})
	if h.hasLine("再对齐会话加载路径。") {
		t.Fatalf("译文态仍在画原文：%q", h.session.Texts())
	}
	h.screenshot(dir, "cot-translated", width, height)

	// 点是「查看原文」（默认就在看译文，官方按钮文案随之切换）。
	h.click(cotLabelShowOriginal)
	h.pumpUntil("原文回来", func() bool { return h.hasLine("再对齐会话加载路径。") })
	if h.hasLine("T(再对齐会话加载路径。)") {
		t.Fatalf("原文态仍在画译文：%q", h.session.Texts())
	}
	h.screenshot(dir, "cot-original", width, height)
}

// TestCotTranslationFailureShowsOriginalAndRetry 走失败态：文案 + 重试真的能再翻。
func TestCotTranslationFailureShowsOriginalAndRetry(t *testing.T) {
	translator := newCotAutoTranslator()
	translator.failFirst = true
	h := openCotHarness(t, 1182, 832, cotShotData(), translator.actions())
	defer h.close()

	h.expandThinking(1)
	h.pumpUntil("失败文案", func() bool { return h.session.HasText(cotLabelFailed) })
	// 失败态必须显示原文（官方 showOriginal = original || failed）。
	if !h.hasLine("再对齐会话加载路径。") {
		t.Fatalf("失败态没有显示原文：%q", h.session.Texts())
	}
	if h.hasLine("T(再对齐会话加载路径。)") {
		t.Fatalf("失败态却显示了译文：%q", h.session.Texts())
	}
	h.pumpUntil("重试按钮", func() bool { return h.session.HasText(cotLabelRetry) })

	// 点重试：这次翻译成功，失败文案消失、译文出现。
	h.click(cotLabelRetry)
	h.pumpUntil("重试后的译文", func() bool {
		return h.hasLine("T(先看目录结构。)") && h.hasLine("T(再对齐会话加载路径。)")
	})
	if h.session.HasText(cotLabelFailed) {
		t.Fatalf("重试成功后仍画着失败文案：%q", h.session.Texts())
	}
}

// TestCotTranslationAbsentWithoutHost 钉住"宿主没接就不画入口"（回归守卫：
// 没有 cotTranslation 的宿主上，思考块外观必须与本功能上线前完全一致）。
func TestCotTranslationAbsentWithoutHost(t *testing.T) {
	h := openCotHarness(t, 1182, 832, cotShotData(), pluginapi.Actions{})
	defer h.close()
	h.expandThinking(1)
	// 展开后正文照常，但五个译文文案一个都不许出现。
	if !h.hasLine("再对齐会话加载路径。") {
		t.Fatalf("展开态没画出思考正文：%q", h.session.Texts())
	}
	for _, label := range []string{cotLabelShowTranslation, cotLabelShowOriginal, cotLabelRetry, cotLabelFailed, cotLabelTranslating} {
		if h.session.HasText(label) {
			t.Fatalf("宿主没接 cotTranslation 却画了「%s」：%q", label, h.session.Texts())
		}
	}
}

// TestCotTranslationShowsEntryWhenProviderUnavailable 钉住官方那条语义：
// **provider 不在可用清单里时入口照样在**，只是多一句 `providerUnavailable`
// （`OFF:TranslationSettings.tsx:58-60`）。
//
// 旧版本这里断言的是"不画入口"——那是把设置项缺失误当成功能缺失，
// 用户会看到一个整块消失、也没有原因的思考块。现版本把"推进状态机"交给
// 宿主真值：provider 不可用时请求照发（宿主会拒），界面得到 failed + 重试，
// 与官方"选不到可用服务 → 翻译失败"同一条路；这里只钉入口与文案。
func TestCotTranslationShowsEntryWhenProviderUnavailable(t *testing.T) {
	translator := newCotAutoTranslator()
	act := translator.actions()
	act.CotTranslationLimits = func() (apiclient.CotTranslationLimits, bool) {
		return apiclient.CotTranslationLimits{
			MaxTextChars: 100000,
			Preferences: apiclient.CotTranslationPreferences{
				Provider:       apiclient.CotTranslationProviderBing,
				TargetLanguage: "auto",
			},
			AvailableProviders: []string{apiclient.CotTranslationProviderOfficial},
		}, true
	}
	h := openCotHarness(t, 1182, 832, cotShotData(), act)
	defer h.close()
	h.expandThinking(1)
	for i := 0; i < 5; i++ {
		h.pump()
	}
	// 入口照常画（官方同一个座位），并且带上官方那句不可用说明。
	// 默认视图是译文，所以按钮文案是「查看原文」（`showOriginal` 为假 → t('original')）。
	if !h.session.HasText(cotLabelShowOriginal) {
		t.Fatalf("provider 不可用时入口不该消失（官方 providerUnavailable 只是加一句话）：%q", h.session.Texts())
	}
	if !h.session.HasText(cotLabelProviderUnavailable) {
		t.Fatalf("provider 不可用没画官方那句说明：%q", h.session.Texts())
	}
}

// heldCotTranslator 是"请求永不完成"的 translator：用来停在 pending 态。
type heldCotTranslator struct{ cotAutoTranslator }

func (a *heldCotTranslator) actions() pluginapi.Actions {
	return pluginapi.Actions{
		CotTranslationLimits: a.cotAutoTranslator.actions().CotTranslationLimits,
		CotTranslationTranslate: func(sessionID, text, targetLanguage, provider string) string {
			a.calls = append(a.calls, cotCall{sessionID: sessionID, text: text,
				targetLanguage: targetLanguage, provider: provider})
			return fmt.Sprintf("held-%d", len(a.calls))
		},
		CotTranslationResult: func(string) (pluginapi.CotTranslationResult, bool) {
			return pluginapi.CotTranslationResult{}, false
		},
	}
}

// 官方那枚 loading 图标的形状常量，全部由官方 path / 官方描边算得，不是量的：
//
//   - 圆弧半径 6.5、viewBox 16、Regular 描边 1（`dsassets.LoadingOutline`，
//     sprite 原文 `"loading-outline":{size:16,...}` + 权重表 `{regular:1}`）；
//   - 显示尺寸 14（`OFF:TranslationBody.tsx:55` 的 `size={14}`）。
//
// 按 14px 画：外半径 = (6.5+0.5)×14/16 = 6.125，内半径 = (6.5-0.5)×14/16 = 5.25。
const (
	cotSpinnerRadius     = 6.5
	cotSpinnerStrokeHalf = 0.5
	cotSpinnerViewBox    = 16
	// cotSpinnerOfficialSize 是官方那处调用点的显示尺寸（`OFF:TranslationBody.tsx:55`
	// 的 `size={14}`）。**写死在这里、不从被测常量反算**：否则把 `cotSpinnerSize`
	// 改成 16，期望值会跟着变，这条就测不出尺寸错了。
	cotSpinnerOfficialSize = 14
	cotSpinnerOuter        = (cotSpinnerRadius + cotSpinnerStrokeHalf) * cotSpinnerOfficialSize / cotSpinnerViewBox
	cotSpinnerInner        = (cotSpinnerRadius - cotSpinnerStrokeHalf) * cotSpinnerOfficialSize / cotSpinnerViewBox
	// cotSpinnerTol 是抗锯齿羽化与浮点误差的余量（实测端点外扩 ≤ 0.05px）。
	cotSpinnerTol = 0.25
)

// TestCotTranslationPendingDrawsSpinner 钉住官方 pending 形状：一枚**官方 14px
// loading 图标**（`OFF:TranslationBody.tsx:55` 的 `IconLoadingOutlineRegular
// size={14}`），既不是一行文字（改版前画的是「正在翻译思考内容」），
// 也不是 Viva 那枚 16px 八辐条 `ui.Spinner`。
//
// 判据是几何，不是"画了点什么"：
//   - 那簇描边图元的包围盒宽 = 外半径 + 外半径/√2（缺口在正东，所以最右点是
//     45° 那个端点）、高 = 2×外半径；
//   - 每个顶点到底圆圆心的距离都落在 [内半径, 外半径]；
//   - 角度覆盖 270°，缺口 90° 且落在正东；
//   - 同一包围盒里**没有矩形图元**——Viva 的旧 spinner 是 8 根小矩形辐条，
//     这条让退回去必然红。
func TestCotTranslationPendingDrawsSpinner(t *testing.T) {
	translator := &heldCotTranslator{}
	act := translator.actions()
	h := openCotHarness(t, 1182, 832, cotShotData(), act)
	defer h.close()
	h.expandThinking(1)
	for i := 0; i < 5; i++ {
		h.pump()
	}
	if cotSpinnerSize != cotSpinnerOfficialSize {
		t.Fatalf("图标显示尺寸=%v，官方那处是 %v", cotSpinnerSize, float32(cotSpinnerOfficialSize))
	}
	// pending 期间官方不画翻译文案，只画 spinner。
	if h.session.HasText(cotLabelFailed) {
		t.Fatalf("还在翻译却画了失败文案：%q", h.session.Texts())
	}
	_, angles, ok := cotSpinnerRing(h.session.Snapshot())
	if !ok {
		t.Fatalf("pending 没有画出官方 loading 图标的圆环（%v）", h.session.Snapshot().Primitives)
	}
	// 缺口：正东 90° 里不许有顶点。
	for _, angle := range angles {
		if angle > -40 && angle < 40 {
			t.Fatalf("正东缺口里出现了顶点（%.2f°），官方那 90° 是空的", angle)
		}
	}
	// 覆盖 270°：从 45° 到 315°，每 30° 一格都得有顶点（缺口那 90° 上面已判过）。
	for want := 45.0; want <= 315; want += 30 {
		found := false
		for _, angle := range angles {
			if math.Abs(normalizeAngle(angle)-want) < 15 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%.0f° 附近没有顶点，说明圆弧没画全（角度集合 %v）", want, angles)
		}
	}
}

// normalizeAngle 把 atan2 的 (-180,180] 折算到 [0,360)。
func normalizeAngle(angle float64) float64 {
	for angle < 0 {
		angle += 360
	}
	return angle
}

// cotSpinnerRing 在 snapshot 里找官方 loading 图标的圆环。
//
// 返回圆心与全部顶点相对圆心的角度（度）。判据同 `TestCotTranslationPendingDrawsSpinner`：
// 把描边三角按包围盒邻接聚成一簇，取最大的一簇，按官方外半径反推圆心，
// 再要求"所有顶点都在 [内半径, 外半径] 的环里"。找不到就 ok=false。
func cotSpinnerRing(snapshot app.Snapshot) (gfx.Point, []float64, bool) {
	type box struct{ minX, minY, maxX, maxY float32 }
	var boxes []box
	var points [][3]gfx.Point
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.TrianglePrimitive {
			continue
		}
		b := box{minX: 1e9, minY: 1e9, maxX: -1e9, maxY: -1e9}
		for _, point := range primitive.Points {
			b.minX = min32(b.minX, point.X)
			b.minY = min32(b.minY, point.Y)
			b.maxX = max32(b.maxX, point.X)
			b.maxY = max32(b.maxY, point.Y)
		}
		boxes = append(boxes, b)
		points = append(points, primitive.Points)
	}
	parent := make([]int, len(boxes))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			if a.minX <= b.maxX+1.2 && b.minX <= a.maxX+1.2 &&
				a.minY <= b.maxY+1.2 && b.minY <= a.maxY+1.2 {
				parent[find(i)] = find(j)
			}
		}
	}
	groups := map[int][]int{}
	for i := range boxes {
		root := find(i)
		groups[root] = append(groups[root], i)
	}
	best := -1
	for root, members := range groups {
		if best < 0 || len(members) > len(groups[best]) {
			best = root
		}
	}
	if best < 0 {
		return gfx.Point{}, nil, false
	}
	members := groups[best]
	minX, minY, maxX, maxY := float32(1e9), float32(1e9), float32(-1e9), float32(-1e9)
	for _, index := range members {
		b := boxes[index]
		minX = min32(minX, b.minX)
		minY = min32(minY, b.minY)
		maxX = max32(maxX, b.maxX)
		maxY = max32(maxY, b.maxY)
	}
	// 官方圆弧：最左点 = 圆心 - 外半径（正西那个端点）；上下 = 圆心 ∓ 外半径；
	// 最右点 = 圆心 + 外半径/√2（45° 那个外沿端点）。
	wantWidth := float64(cotSpinnerOuter) + float64(cotSpinnerOuter)/math.Sqrt2
	wantHeight := 2 * float64(cotSpinnerOuter)
	if math.Abs(float64(maxX-minX)-wantWidth) > cotSpinnerTol ||
		math.Abs(float64(maxY-minY)-wantHeight) > cotSpinnerTol {
		return gfx.Point{}, nil, false
	}
	center := gfx.Point{X: minX + cotSpinnerOuter, Y: (minY + maxY) / 2}
	var angles []float64
	for _, index := range members {
		for _, point := range points[index] {
			r := math.Hypot(float64(point.X-center.X), float64(point.Y-center.Y))
			if r < float64(cotSpinnerInner)-cotSpinnerTol || r > float64(cotSpinnerOuter)+cotSpinnerTol {
				return gfx.Point{}, nil, false
			}
			angles = append(angles, math.Atan2(float64(point.Y-center.Y), float64(point.X-center.X))*180/math.Pi)
		}
	}
	// 旧实现（Viva ui.Spinner）在同一块地方画的是 8 根小矩形辐条：有矩形就不是官方形状。
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.RectanglePrimitive {
			continue
		}
		b := primitive.Bounds
		if b.X >= minX-cotSpinnerTol && b.Y >= minY-cotSpinnerTol &&
			b.X+b.Width <= maxX+cotSpinnerTol && b.Y+b.Height <= maxY+cotSpinnerTol {
			return gfx.Point{}, nil, false
		}
	}
	return center, angles, true
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
