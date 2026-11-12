//go:build darwin || windows || linux

// reasoning_visual_test.go —— 思考块真实绘制与命中：折叠预览、点击展开、
// <signature> 只从预览消失、真实时长出现/未知隐藏。
//
// 这是 docs/conversation-gap.md 第 12 项的视觉证据：两个窗口尺寸各在
// 展开前/展开后留一张真实像素图（REASONING_CAPTURE_DIR 非空时落到
// plugins/dsconversation/testdata/reasoning-ui，否则仍实画到临时目录）。
package dsconversation

import (
	"fmt"
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
	reasoningShotSession = "sess-reasoning-ui"
	reasoningUserLine    = "先检查一下仓库，再看流式输出。"
	// reasoningRaw 同时含签名、开头段落、中间段落与「最近完成段落」：
	// 收起行只许出现最后一段的第一行；展开必须逐字还原这一整段原文。
	reasoningRaw = "<signature>c2lnbmF0dXJlLWJsb2Nr</signature>\n" +
		"开头第一段：先看目录结构，确认仓库基线与测试基线。\n\n" +
		"中间一段：把会话加载路径和流式路径对齐。\n\n" +
		"最近完成段落第一行\n最近段落还在写"
	reasoningBaseTime = int64(1762222444000)
)

// reasoningShotData 造一条 user → thinking → assistant 的真实形状快照：
// 两条消息都有 Transport 时间真值，间隔 7 秒。
func reasoningShotData() pluginapi.Snapshot {
	msgs := []apiclient.Message{
		{Seq: 1, Role: "user", Text: reasoningUserLine},
		{Seq: 2, Role: "thinking", Text: reasoningRaw},
		{Seq: 3, Role: "assistant", Text: "基线是绿的，可以开工。"},
	}
	data := pluginapi.Snapshot{Cur: reasoningShotSession, Msgs: msgs, Total: len(msgs)}
	data.MessageMeta = map[string]pubapi.MessageMetadata{
		reasoningKey(data, 0): {Time: reasoningBaseTime},
		reasoningKey(data, 1): {Time: reasoningBaseTime + 7000},
		reasoningKey(data, 2): {Time: reasoningBaseTime + 9000},
	}
	return data
}

type reasoningHarness struct {
	t           *testing.T
	p           *Plugin
	data        pluginapi.Snapshot
	tree        *gfx.Tree
	feat        gfx.Scope
	session     *headless.Session
	invalidator *app.Invalidator
}

func openReasoningHarness(t *testing.T, width, height int, data pluginapi.Snapshot) *reasoningHarness {
	t.Helper()
	p := newStreamingMarkdownTestPlugin()
	theme := p.theme
	tree := &gfx.Tree{}
	feat := tree.Feature("reasoning-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "reasoning-headless", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, current ui.Size) {
		bounds := ui.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: current.Width,
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			p.draw(f, env)
		})
		p.afterDraw(canvas, env)
	}
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult { return view.Wheel(event) }
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := &reasoningHarness{t: t, p: p, data: data, tree: tree, feat: feat, session: session, invalidator: &invalidator}
	h.pump()
	h.pump()
	return h
}

func (h *reasoningHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *reasoningHarness) close() { h.session.Close() }

// reasoningHit 找某条思考行的真实命中区（Paint 之后由 HitTest 登记）。
func reasoningHit(snapshot app.Snapshot, id gfx.ID) (gfx.Hit, bool) {
	for _, hit := range snapshot.Hits {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func (h *reasoningHarness) screenshot(dir, name string, width, height int) {
	h.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%dx%d.png", name, width, height))
	if err := h.session.Screenshot(path); err != nil {
		h.t.Fatal(err)
	}
	picture, err := h.session.Image()
	if err != nil {
		h.t.Fatal(err)
	}
	if picture.Bounds().Dx() != width || picture.Bounds().Dy() != height {
		h.t.Fatalf("截图尺寸=%v，期望 %dx%d", picture.Bounds(), width, height)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		h.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	h.t.Logf("REASONING_CAPTURE path=%s size=%dx%d bytes=%d", path, width, height, info.Size())
}

// TestReasoningUIPaintHitTestEvidence 在 1182×832 与 1920×1032 两个尺寸上
// 走同一条真实路径：Paint → HitTest 命中思考行 → 点击 → 再 Paint。
func TestReasoningUIPaintHitTestEvidence(t *testing.T) {
	dir := os.Getenv("REASONING_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, size := range []struct {
		name          string
		width, height int
	}{{"1182x832", 1182, 832}, {"1920x1032", 1920, 1032}} {
		t.Run(size.name, func(t *testing.T) { runReasoningEvidence(t, size.width, size.height, dir) })
	}
}

func runReasoningEvidence(t *testing.T, width, height int, dir string) {
	t.Helper()
	data := reasoningShotData()
	h := openReasoningHarness(t, width, height, data)
	defer h.close()
	key := messageRenderKey(&pluginapi.Env{Data: data}, data.Msgs[1], 1)

	// —— 收起态：最近完成段落 + 真实 7 秒；开头段落与签名都不许出现。
	if !h.session.HasText("思考") {
		t.Fatalf("思考行没画出来：%q", h.session.Texts())
	}
	if !h.session.HasText("想了 7 秒") {
		t.Fatalf("真实起止时间戳没有显示「想了 7 秒」：%q", h.session.Texts())
	}
	if !h.session.HasText("最近完成段落第一行") {
		t.Fatalf("折叠预览没有取最近完成段落：%q", h.session.Texts())
	}
	if h.session.HasText("开头第一段") {
		t.Fatalf("折叠预览仍在画原始文本开头段落：%q", h.session.Texts())
	}
	if h.session.HasText("signature") || h.session.HasText("c2lnbmF0dXJl") {
		t.Fatalf("折叠预览没清理 <signature>：%q", h.session.Texts())
	}
	h.screenshot(dir, "reasoning-collapsed", width, height)

	// —— 真实命中：找 think-<MessageKey>，按命中区中心点击。
	id := h.tree.ID(h.feat, "think-"+key)
	hit, ok := reasoningHit(h.session.Snapshot(), id)
	if !ok {
		t.Fatalf("思考行没有命中区 id=%v；hits=%+v", id, h.session.Snapshot().Hits)
	}
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	if err := h.session.Press(x, y); err != nil {
		t.Fatal(err)
	}
	if err := h.session.Release(x, y); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if !h.p.expanded[key] {
		t.Fatalf("点击思考行没有展开（key=%q）：%q", key, h.session.Texts())
	}

	// —— 展开态：全文逐字还原，开头段落/签名/未完成段落都在。
	if !h.session.HasText("开头第一段") {
		t.Fatalf("展开后丢了开头段落：%q", h.session.Texts())
	}
	if !h.session.HasText("中间一段") {
		t.Fatalf("展开后丢了中间段落：%q", h.session.Texts())
	}
	if !h.session.HasText("<signature>c2lnbmF0dXJlLWJsb2Nr</signature>") {
		t.Fatalf("展开后丢了 <signature> 原文（清理只允许发生在预览里）：%q", h.session.Texts())
	}
	if !h.session.HasText("最近段落还在写") {
		t.Fatalf("展开后丢了最后一段内容：%q", h.session.Texts())
	}
	h.screenshot(dir, "reasoning-expanded", width, height)
}

// TestReasoningUIHidesUnknownAndRunningDurations 证明时长只在有真值时出现：
// 没有 MessageMeta、或这一段仍在跑（Busy 尾块），一个字都不许编。
func TestReasoningUIHidesUnknownAndRunningDurations(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"1182x832", 1182, 832}, {"1920x1032", 1920, 1032}} {
		t.Run(size.name, func(t *testing.T) {
			t.Run("没有时间真值", func(t *testing.T) {
				data := reasoningShotData()
				data.MessageMeta = nil
				h := openReasoningHarness(t, size.width, size.height, data)
				defer h.close()
				if h.session.HasText("想了 ") {
					t.Fatalf("没有时间真值却画了时长：%q", h.session.Texts())
				}
				if !h.session.HasText("最近完成段落第一行") {
					t.Fatalf("未知时长只许隐藏时长，预览仍要在：%q", h.session.Texts())
				}
			})
			t.Run("运行中的尾块", func(t *testing.T) {
				data := reasoningShotData()
				data.Msgs = data.Msgs[:2]
				data.Total = len(data.Msgs)
				data.Busy = true
				h := openReasoningHarness(t, size.width, size.height, data)
				defer h.close()
				if h.session.HasText("想了 ") {
					t.Fatalf("运行中不许伪造「Thought for Ns」：%q", h.session.Texts())
				}
			})
		})
	}
}

// TestReasoningOnlyAssistantStillPaints 是「reasoning-only assistant 不消失」的
// 真实绘制钉子：宿主给的只有思考的回答，经 ReasoningOnlyThinking 翻成
// thinking 之后仍有一行思考块（带真实时长），不是空气泡。
func TestReasoningOnlyAssistantStillPaints(t *testing.T) {
	think, ok := apiclient.ReasoningOnlyThinking(apiclient.Message{
		Role: "assistant", Reasoning: "只有思考的回答：先确认真实基线。",
	})
	if !ok {
		t.Fatal("reasoning-only assistant 没有翻成 thinking")
	}
	msgs := []apiclient.Message{
		{Seq: 1, Role: "user", Text: "开始。"},
		think,
	}
	data := pluginapi.Snapshot{Cur: "sess-reasoning-only", Msgs: msgs, Total: len(msgs)}
	data.MessageMeta = map[string]pubapi.MessageMetadata{
		reasoningKey(data, 0): {Time: reasoningBaseTime},
		reasoningKey(data, 1): {Time: reasoningBaseTime + 4000},
	}
	h := openReasoningHarness(t, 1182, 832, data)
	defer h.close()
	if !h.session.HasText("思考") {
		t.Fatalf("reasoning-only assistant 消失了：%q", h.session.Texts())
	}
	if !h.session.HasText("想了 4 秒") {
		t.Fatalf("reasoning-only 的真实时长没显示：%q", h.session.Texts())
	}
	if !h.session.HasText("只有思考的回答") {
		t.Fatalf("reasoning-only 的折叠预览没画正文：%q", h.session.Texts())
	}
}
