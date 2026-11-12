//go:build windows || darwin || linux

package dsconversation

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// 这条无头会话回答三个问题：按钮在不在一排里、点一下画面真的变了没有、
// 以及记录到底写进了哪个文件的哪个键。
//
// 证据图默认落在临时目录；FEEDBACK_CAPTURE_DIR 非空时落到指定目录（验收留档用）。

const (
	feedbackShotSession = "sess-feedback-shot"
	// feedbackShotIndex 是助手消息在快照里的下标——它就是配置键里的 msgIndex。
	feedbackShotIndex = 1
)

// withRowActions 在配置通道之上补两个空的 复制/重试：它们是壳接线的真实动作，
// 给空实现只为让它们真的画出来（没有 Action 时按钮本来就不画，那样就证明不了"同一排"）。
func withRowActions(act pluginapi.Actions) pluginapi.Actions {
	act.Copy = func(string) {}
	act.Retry = func() {}
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

// brightest 返回一块矩形里最亮的像素（只取亮度，用来比较两处的前景色是否同一档）。
func brightest(picture *image.RGBA, bounds gfx.Rect) (luma int, x, y int) {
	luma = -1
	for py := int(bounds.Y); py < int(bounds.Y+bounds.Height); py++ {
		for px := int(bounds.X); px < int(bounds.X+bounds.Width); px++ {
			if px < picture.Rect.Min.X || px >= picture.Rect.Max.X || py < picture.Rect.Min.Y || py >= picture.Rect.Max.Y {
				continue
			}
			c := picture.RGBAAt(px, py)
			l := (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
			if l > luma {
				luma, x, y = l, px, py
			}
		}
	}
	return luma, x, y
}

// changedPixels 数两张图有多少像素不同（用来证明"点了之后画面真的变了"）。
func changedPixels(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	changed := 0
	for y := a.Rect.Min.Y; y < a.Rect.Max.Y; y++ {
		for x := a.Rect.Min.X; x < a.Rect.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				changed++
			}
		}
	}
	return changed
}

func TestMessageFeedbackInActionRow(t *testing.T) {
	dir := os.Getenv("FEEDBACK_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 配置通道用假壳实现（ConfigGet/ConfigSet）：这条测试绝不碰真实用户的 ui-config.json。
	cfg := newFakeConfig(nil)

	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	feat := tree.Feature("feedback-headless")
	data := feedbackShotData()
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var (
		invalidator app.Invalidator
	)
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
			// 复制/重试 是壳接线的真实动作；这里给两个空实现，只为让它们真的画出来
			// （没有 Actor 时按钮本来就不画，那样就证明不了"同一排"）。
			Act: withRowActions(cfg.actions()),
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			p.draw(f, env)
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

	upHit, ok := session.Hit("好的回答")
	if !ok {
		t.Fatalf("动作行里没有👍（命中区缺失）；画面文本=%q", session.Texts())
	}
	downHit, ok := session.Hit("有问题的回答")
	if !ok {
		t.Fatal("动作行里没有👎（命中区缺失）")
	}
	copyHit, ok := session.Hit("复制")
	if !ok {
		t.Fatal("复制按钮不在这一排，无法验证对齐")
	}

	// ── 对齐：同一排、同一盒高、同一基线 ─────────────────────────────
	// 官方 .Hyy-Wa_action 是 28×28；本排的 Ghost/Small 按钮盒高也是 28
	// （viva ControlHeight 32 - 4），所以两者底边和中心必须落在同一像素行上。
	const wantHit = 28
	for name, hit := range map[string]gfx.Hit{"👍": upHit, "👎": downHit, "复制": copyHit} {
		if hit.Bounds.Height != wantHit {
			t.Errorf("%s 命中区高 = %v，期望 %d（官方 .Hyy-Wa_action / viva Small 按钮）", name, hit.Bounds.Height, wantHit)
		}
	}
	center := func(r gfx.Rect) float32 { return r.Y + r.Height/2 }
	if delta := center(upHit.Bounds) - center(copyHit.Bounds); delta > 0.5 || delta < -0.5 {
		t.Errorf("👍 与「复制」不在同一基线：中心差 %v px（👍 中心 %v，复制中心 %v）",
			delta, center(upHit.Bounds), center(copyHit.Bounds))
	}
	if delta := center(downHit.Bounds) - center(copyHit.Bounds); delta > 0.5 || delta < -0.5 {
		t.Errorf("👎 与「复制」不在同一基线：中心差 %v px", delta)
	}
	// 次序：官方 IconActions 是 copy → feedback → branch。这里只有两个固定点
	// （复制在最左、👍 在复制右边、👎 在👍右边），够证明"夹在中间"。
	if !(copyHit.Bounds.X < upHit.Bounds.X && upHit.Bounds.X < downHit.Bounds.X) {
		t.Errorf("次序不对：复制 x=%v，👍 x=%v，👎 x=%v", copyHit.Bounds.X, upHit.Bounds.X, downHit.Bounds.X)
	}

	// 与同排文字按钮没有额外间隙：相邻命中区之间不留 width gap
	// （Ghost 按钮自带 10px 内边距，所以这里只要求不出现额外的空隙 > 1px）。
	if gap := upHit.Bounds.X - (copyHit.Bounds.X + copyHit.Bounds.Width); gap > 1 {
		t.Errorf("复制 与 👍 之间多出 %v px 空隙", gap)
	}

	restPath := filepath.Join(dir, "feedback.png")
	if err := session.Screenshot(restPath); err != nil {
		t.Fatal(err)
	}
	restPicture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}

	// ── 颜色：图标和同排文字同一档前景色 ────────────────────────────
	// 官方图标是 label-tertiary，但本排的「复制/重试」是 Ghost 按钮（= 主题 Foreground
	// = LabelPrimary）。同一排混两种深浅会一眼看出不对，所以取同一档，这里验证它。
	iconLuma, _, _ := brightest(restPicture, upHit.Bounds)
	textLuma, _, _ := brightest(restPicture, copyHit.Bounds)
	if iconLuma < 200 {
		t.Errorf("👍 最亮像素亮度 %d 太低：图标没画出来或颜色不对", iconLuma)
	}
	if diff := iconLuma - textLuma; diff > 8 || diff < -8 {
		t.Errorf("👍 亮度 %d 与「复制」文字亮度 %d 不是同一档前景色", iconLuma, textLuma)
	}

	// ── 点击：变实心、并留下本机记录 ───────────────────────────────
	if err := session.ClickLabel("好的回答"); err != nil {
		t.Fatalf("点👍失败: %v", err)
	}
	// 指针移开再截图：官方的说法是"已记录的评分在指针离开之后依然可见"，
	// 所以证据图里不能带着悬停底色，否则分不清是哪一种状态在起作用。
	if err := session.Move(820, 560); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	activeHit, ok := session.Hit("取消标记")
	if !ok {
		t.Fatalf("点了👍之后没有进入已记录态（命中区标签没变成「取消标记」）；文本=%q", session.Texts())
	}
	if activeHit.Bounds != upHit.Bounds {
		t.Errorf("切换状态改变了命中区几何：%+v → %+v", upHit.Bounds, activeHit.Bounds)
	}
	if _, still := session.Hit("好的回答"); still {
		t.Error("👍 已记录后又出现了「好的回答」标签，两个状态同时存在")
	}

	// 记录必须经壳的配置通道写出去（键就是宿主 ui-config.json 里那个扁平键）。
	key := "feedback." + feedbackShotSession + "." + itoa(feedbackShotIndex)
	if got := cfg.store[key]; got != "up" {
		t.Fatalf("配置通道里 %s = %q，期望 up；整表=%v", key, got, cfg.store)
	}
	if got := p.feedbackRating(feedbackShotSession, feedbackShotIndex); got != "up" {
		t.Fatalf("内存态 = %q，期望 up", got)
	}

	activePath := filepath.Join(dir, "feedback-active.png")
	if err := session.Screenshot(activePath); err != nil {
		t.Fatal(err)
	}
	activePicture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	changed := changedPixels(restPicture, activePicture)
	if changed <= 0 {
		t.Fatalf("点击前后两张图完全相同（changed=%d）：状态没有真的画出来", changed)
	}
	// 只允许很小的改动范围：一次点赞只该换掉拇指那一小块。
	if changed > 1200 {
		t.Errorf("点击改了 %d 个像素，超出「只换图标」的范围（不应有布局跳动）", changed)
	}
	// 实心版比空心版更"重"：图标区域内的亮像素数必须增加。
	inkCount := func(picture *image.RGBA, bounds gfx.Rect) int {
		n := 0
		for py := int(bounds.Y); py < int(bounds.Y+bounds.Height); py++ {
			for px := int(bounds.X); px < int(bounds.X+bounds.Width); px++ {
				if l, _, _ := brightest(picture, gfx.Rect{X: float32(px), Y: float32(py), Width: 1, Height: 1}); l > 120 {
					n++
				}
			}
		}
		return n
	}
	if before, after := inkCount(restPicture, upHit.Bounds), inkCount(activePicture, upHit.Bounds); after <= before {
		t.Errorf("点击后拇指没有变实心：亮像素 %d → %d", before, after)
	}

	// ── 再点一次 = 撤回（官方：点击已记录的评分取消它）────────────────
	if err := session.ClickLabel("取消标记"); err != nil {
		t.Fatalf("点已记录的👍失败: %v", err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if _, ok := session.Hit("好的回答"); !ok {
		t.Error("撤回之后没有回到未记录态")
	}
	// 撤回写的是空串墓碑（ConfigSet 只有合并语义，没有删除动作）；
	// 读回来必须当成"没记录"，不能拿它冒充一条评分。
	if got, exists := cfg.store[key]; !exists || got != "" {
		t.Errorf("撤回后配置里 %s = %q（exists=%v），期望空串墓碑", key, got, exists)
	}

	// ── 证据：三张图都有、尺寸对、非空 ──────────────────────────────
	// 第三张把"撤回后"也留档：证明撤回不是把按钮删掉，而是回到空心状态。
	revertedPath := filepath.Join(dir, "feedback-reverted.png")
	if err := session.Screenshot(revertedPath); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		path string
		want image.Rectangle
	}{
		{"rest", restPath, image.Rect(0, 0, 900, 620)},
		{"active", activePath, image.Rect(0, 0, 900, 620)},
		{"reverted", revertedPath, image.Rect(0, 0, 900, 620)},
	} {
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if picture.Bounds() != item.want {
			t.Fatalf("%s 截图尺寸 = %v，期望 %v", item.name, picture.Bounds(), item.want)
		}
		info, err := os.Stat(item.path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("%s 截图为空 path=%s info=%v err=%v", item.name, item.path, info, err)
		}
	}
	t.Logf("FEEDBACK_CAPTURE rest=%s active=%s reverted=%s click_changed_px=%d up_hit=%+v copy_hit=%+v config_key=%s",
		restPath, activePath, revertedPath, changed, upHit.Bounds, copyHit.Bounds, key)
}
