//go:build windows || darwin || linux

package dsconversation

// feedback_visual_test.go —— 反馈按钮与弹窗的**一帧真布局**验收。
//
// 数据面是假的宿主，但交互走的是真实路径：
//
//	点 👍 → 打开弹窗 → 选分类 → 提交 → 壳回成功 → 按钮变「取消标记」+ toast
//	点「取消标记」→ delete → 回到未标记
//
// 断言只依据本帧画布：命中区几何、文案、点击后的状态。证据图默认落临时目录；
// FEEDBACK_CAPTURE_DIR 非空时落到指定目录（验收留档用）。

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const feedbackShotSession = "sess-feedback-shot"

// fakeFeedbackHost 是一个最小宿主：items 就是 messageFeedback.list 的结果，
// put/delete 同步改它并把终态留给 FeedbackTakeResult。
type fakeFeedbackHost struct {
	mu      sync.Mutex
	items   map[string]pluginapi.FeedbackItem
	results []pluginapi.FeedbackResult
	puts    int
	deletes int
}

func newFakeFeedbackHost() *fakeFeedbackHost {
	return &fakeFeedbackHost{items: map[string]pluginapi.FeedbackItem{}}
}

func (h *fakeFeedbackHost) actions(copy, retry bool) pluginapi.Actions {
	act := pluginapi.Actions{
		MessageFeedback: func(session string) (map[string]pluginapi.FeedbackItem, bool) {
			h.mu.Lock()
			defer h.mu.Unlock()
			out := map[string]pluginapi.FeedbackItem{}
			for k, v := range h.items {
				out[k] = v
			}
			return out, true
		},
		MessageFeedbackPut: func(session, id, rating, note, category string) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.puts++
			h.items[id] = pluginapi.FeedbackItem{
				MessageID: id, Rating: rating, Note: note, Category: category, Version: "v-" + id,
			}
			h.results = append(h.results, pluginapi.FeedbackResult{MessageID: id, OK: true})
		},
		MessageFeedbackDelete: func(session, id string) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.deletes++
			delete(h.items, id)
			h.results = append(h.results, pluginapi.FeedbackResult{MessageID: id, OK: true})
		},
		FeedbackTakeResult: func(session string) (pluginapi.FeedbackResult, bool) {
			h.mu.Lock()
			defer h.mu.Unlock()
			if len(h.results) == 0 {
				return pluginapi.FeedbackResult{}, false
			}
			out := h.results[0]
			h.results = h.results[1:]
			return out, true
		},
	}
	if copy {
		act.Copy = func(string) {}
	}
	if retry {
		act.Retry = func() {}
	}
	return act
}

func feedbackShotData() pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur: feedbackShotSession,
		Msgs: []apiclient.Message{
			{Seq: 1, Role: "user", Text: "把反馈按钮放到消息动作行里。"},
			{Seq: 2, Role: "assistant", Text: "已经加在这条消息下面了：复制 · 👍 · 👎 · 重试。"},
		},
	}
}

// containsText 报告画布上有没有这段文字（viva 的 Texts 是每段绘制文本）。
func containsText(texts []string, want string) bool {
	for _, text := range texts {
		if text == want || strings.Contains(text, want) {
			return true
		}
	}
	return false
}

func TestMessageFeedbackDialogAndRecord(t *testing.T) {
	dir := os.Getenv("FEEDBACK_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	host := newFakeFeedbackHost()
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	feat := tree.Feature("feedback-headless")
	data := feedbackShotData()
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "feedback-headless", Width: 900, Height: 620, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
			Act: host.actions(true, true),
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillWidth().Children(func() {
				ui.Column(f).Width(size.Width - 24).Children(func() { p.draw(f, env) })
				p.drawFeedbackOverlay(f, env)
			})
		})
		p.afterDraw(canvas, env)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}

	// ── 未记录态：两个按钮都在，且与「复制」同一排同一基线 ──────────────
	upHit, ok := session.Hit(zhFeedbackLike)
	if !ok {
		t.Fatalf("动作行里没有👍（命中区缺失）；画面文本=%q", session.Texts())
	}
	downHit, ok := session.Hit(zhFeedbackDislike)
	if !ok {
		t.Fatal("动作行里没有👎（命中区缺失）")
	}
	copyHit, ok := session.Hit("复制")
	if !ok {
		t.Fatal("复制按钮不在这一排，无法验证对齐")
	}
	for name, hit := range map[string]gfx.Hit{"👍": upHit, "👎": downHit, "复制": copyHit} {
		if hit.Bounds.Height != feedbackHitSize {
			t.Errorf("%s 命中区高 = %v，期望 %d", name, hit.Bounds.Height, feedbackHitSize)
		}
	}
	center := func(r gfx.Rect) float32 { return r.Y + r.Height/2 }
	if delta := center(upHit.Bounds) - center(copyHit.Bounds); delta > 0.5 || delta < -0.5 {
		t.Errorf("👍 与「复制」不在同一基线：中心差 %v px", delta)
	}
	if !(copyHit.Bounds.X < upHit.Bounds.X && upHit.Bounds.X < downHit.Bounds.X) {
		t.Errorf("次序不对：复制 x=%v，👍 x=%v，👎 x=%v", copyHit.Bounds.X, upHit.Bounds.X, downHit.Bounds.X)
	}

	// ── 点 👍 打开弹窗（官方：未记录的一侧先收集分类/详情）──────────────
	if err := session.ClickLabel(zhFeedbackLike); err != nil {
		t.Fatalf("点👍失败: %v", err)
	}
	if p.feedbackDialog == nil {
		t.Fatal("点未记录的👍应当打开反馈弹窗")
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	// 弹窗内容：标题、分类、详情、提交（标题/正文是文本，不是命中区）。
	texts := session.Texts()
	for _, want := range []string{zhFeedbackDialogTitle, "任务结果", "稳定性和速度", zhFeedbackHint, zhFeedbackSubmit} {
		if !containsText(texts, want) {
			t.Fatalf("弹窗缺少 %q；画面文本=%q", want, texts)
		}
	}
	if _, ok := session.Hit(zhFeedbackLike); ok {
		t.Fatal("弹窗打开时下层按钮不应再可点（模态）")
	}

	// 选一个分类、提交。
	if err := session.ClickLabel("其他"); err != nil {
		t.Fatalf("点分类失败: %v", err)
	}
	if p.feedbackDialog == nil || p.feedbackDialog.Category != apiclient.FeedbackCategoryOther {
		t.Fatalf("分类没选中：%+v", p.feedbackDialog)
	}
	if err := session.ClickLabel(zhFeedbackSubmit); err != nil {
		t.Fatalf("点提交失败: %v", err)
	}
	host.mu.Lock()
	puts := host.puts
	host.mu.Unlock()
	if puts != 1 {
		t.Fatalf("提交没有调用 messageFeedback.put（puts=%d）", puts)
	}
	// 壳回成功 → 下一帧关弹窗、按钮进已记录态、toast 出现。
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if p.feedbackDialog != nil {
		t.Fatal("提交成功后弹窗应关闭")
	}
	if _, ok := session.Hit(zhFeedbackRetract); !ok {
		t.Fatalf("提交成功后按钮没进入「取消标记」态；文本=%q", session.Texts())
	}
	if !containsText(session.Texts(), zhFeedbackRecorded) {
		t.Fatalf("提交成功后没有「%s」toast；文本=%q", zhFeedbackRecorded, session.Texts())
	}
	restPath := filepath.Join(dir, "feedback-recorded.png")
	if err := session.Screenshot(restPath); err != nil {
		t.Fatal(err)
	}

	// ── 点已记录的评分 = 撤回 ─────────────────────────────────────────
	if err := session.ClickLabel(zhFeedbackRetract); err != nil {
		t.Fatalf("点撤回失败: %v", err)
	}
	host.mu.Lock()
	deletes := host.deletes
	host.mu.Unlock()
	if deletes != 1 {
		t.Fatalf("撤回没有调用 messageFeedback.delete（deletes=%d）", deletes)
	}
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if _, ok := session.Hit(zhFeedbackLike); !ok {
		t.Fatalf("撤回后没回到未记录态；文本=%q", session.Texts())
	}
	revertedPath := filepath.Join(dir, "feedback-reverted.png")
	if err := session.Screenshot(revertedPath); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ name, path string }{
		{"recorded", restPath}, {"reverted", revertedPath},
	} {
		info, err := os.Stat(item.path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("%s 截图为空 path=%s info=%v err=%v", item.name, item.path, info, err)
		}
	}
	t.Logf("FEEDBACK_CAPTURE recorded=%s reverted=%s puts=%d deletes=%d up_hit=%+v",
		restPath, revertedPath, puts, deletes, upHit.Bounds)
}
