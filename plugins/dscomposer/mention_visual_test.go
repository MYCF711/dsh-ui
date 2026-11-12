//go:build darwin || windows || linux

package dscomposer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestMentionMenuHeadlessCapture 用真实 Viva 文本测量 + 光栅化验收 `@` 候选菜单。
//
// 为什么要有这条：单测能证明"候选算出来了"，证明不了**它画在屏幕上**。
// 这个仓里"算了但没画"已经发生过好几次（SlotMainHeader、open-in-app 的按钮），
// 所以每一块新界面都得有一帧真实像素。
//
// # 判据是怎么定下来的（第一版定错了）
//
// 第一版判"卡片被菜单顶上去了"（比较卡片的 Y）。跑出来两张图的卡片 Y **一模一样**——
// 一开始看着像 bug，其实是布局使然：中栏那一列是 `Spacer()` + composer，
// 菜单多占的高度由 Spacer 吸收，**卡片始终贴着底部**。
// 所以"卡片 Y 变了"根本不是这条链路该有的期望，拿它当判据只会得到一个假的失败。
//
// 正确判据是**内容顶端**：把这一帧画出来的所有图元的最低 Y 取出来，
// 菜单开着时它必须比关着时高一截。这个量对"菜单画在哪"敏感，对 Spacer 不敏感。
func TestMentionMenuHeadlessCapture(t *testing.T) {
	dir := os.Getenv("COMPOSER_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	const cwd = `D:\ws`
	fsList := func(string) []pluginapi.FSEntry {
		return []pluginapi.FSEntry{
			{Name: "dsopen", IsDir: true},
			{Name: "dsquestions", IsDir: true},
			{Name: "dsstatusbar", IsDir: true},
			{Name: "main.go"},
		}
	}

	// shot 画一帧，返回：卡片矩形、这一帧画出来的内容顶端、候选行数。
	shot := func(t *testing.T, name, draft string, act pluginapi.Actions) (ui.Rect, float32, int) {
		t.Helper()
		theme := dscore.Dark()
		p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
		if draft != "" {
			p.field.SetText(draft)
		}
		data := pluginapi.Snapshot{Cur: "s-001", Model: "deepseek-v4-pro", Permission: "ask", CWD: cwd}
		if act.Send == nil {
			act.Send = func(string) {}
			act.Cancel = func() {}
		}
		tree := &gfx.Tree{}
		feat := tree.Feature("composer-mention-" + name)
		var view flex.View
		var invalidator app.Invalidator
		config := app.Config{
			Title: "composer-mention-" + name, Width: 1182, Height: 832, Tree: tree,
			WindowMode: app.WindowFrameless, Invalidator: &invalidator,
		}
		config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
			bounds := ui.Rect{Width: size.Width, Height: size.Height}
			canvas.Fill(bounds, theme.BgBase)
			env := &pluginapi.Env{
				Theme: theme, Data: data, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
				Bounds: bounds, MainWidth: 902,
				Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: 902},
			}
			view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
				f.Column().Width(902-24).FillHeight().PadSides(0, 0, 12, 0).Children(func() {
					f.Spacer()
					p.draw(f, env)
				})
			})
			p.afterDraw(canvas, env)
		}
		session, err := headless.Open(config, nil, headless.Options{Scale: 1})
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}

		var card app.Primitive
		found := false
		for _, primitive := range session.Snapshot().Primitives {
			if primitive.Color != theme.InputMajor || primitive.Radius != 28 {
				continue
			}
			area := primitive.Bounds.Width * primitive.Bounds.Height
			if !found || area > card.Bounds.Width*card.Bounds.Height {
				card, found = primitive, true
			}
		}
		if !found {
			t.Fatal("真实像素帧里找不到 R28 composer 卡片")
		}

		// 内容顶端：只看**落在卡片横向范围内**的图元。
		// 不这么框的话会取到铺满整窗的那层底（它的 Y 恒为 0），对着它比什么都比不出来——
		// 第一版的判据就是这么废掉的。
		contentTop := float32(1 << 30)
		for _, primitive := range session.Snapshot().Primitives {
			b := primitive.Bounds
			if b.X < card.Bounds.X-1 || b.X+b.Width > card.Bounds.X+card.Bounds.Width+1 {
				continue
			}
			if primitive.Color == theme.BgBase {
				continue
			}
			if b.Y < contentTop {
				contentTop = b.Y
			}
		}
		if contentTop == float32(1<<30) {
			t.Fatal("卡片横向范围内什么都没画出来")
		}

		path := filepath.Join(dir, fmt.Sprintf("mention-%s.png", name))
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("截图文件无效 path=%s info=%v err=%v", path, info, err)
		}
		t.Logf("MENTION_VISUAL path=%s bytes=%d card={x:%v y:%v w:%v h:%v} contentTop=%v rows=%d",
			path, info.Size(), card.Bounds.X, card.Bounds.Y, card.Bounds.Width, card.Bounds.Height,
			contentTop, len(p.mentionRows))
		return card.Bounds, contentTop, len(p.mentionRows)
	}

	t.Run("菜单开着", func(t *testing.T) {
		cardOpen, topOpen, rows := shot(t, "open", "@ds", pluginapi.Actions{FSList: fsList})
		if rows == 0 {
			t.Fatal("草稿是 @ds、目录里有 dsopen/dsquestions，菜单却一行都没有")
		}

		// 对照：同样的草稿，但壳没接 FSList（老宿主）→ 不该有菜单。
		cardClosed, topClosed, rows2 := shot(t, "closed", "@ds", pluginapi.Actions{})
		if rows2 != 0 {
			t.Fatalf("壳没接 FSList 时不该有候选，得到 %d 行", rows2)
		}

		if !(topOpen < topClosed) {
			t.Errorf("菜单开着时内容顶端应当更高：开着 y=%v、没开 y=%v", topOpen, topClosed)
		}
		// 差值必须够大，否则"高了 1px"也算过——那说明画的不是一块菜单。
		if d := topClosed - topOpen; d < 24 {
			t.Errorf("菜单只占了 %vpx（至少要有一行的高度），基本等于没画", d)
		}
		// 卡片贴底：菜单只能往上长，不能把卡片顶出屏幕。
		if cardOpen.Y+cardOpen.Height != cardClosed.Y+cardClosed.Height {
			t.Errorf("卡片底沿被菜单挪动了：%v → %v（它应当贴着底部不动）",
				cardClosed.Y+cardClosed.Height, cardOpen.Y+cardOpen.Height)
		}
		// 菜单和卡片同宽：官方那套是"同一个宽度的一列"，
		// 宽窄不一致看起来像两个不相干的东西。
		if cardOpen.Width != cardClosed.Width {
			t.Errorf("菜单把卡片宽度改了：%v → %v", cardClosed.Width, cardOpen.Width)
		}
	})

	t.Run("敲完之后不再弹", func(t *testing.T) {
		// `@main.go 然后呢`：token 已经被空格结束，用户开始写别的话了。
		// 这时还弹菜单，Enter 就会被吃掉——消息发不出去。
		_, _, rows := shot(t, "done", "@main.go 然后呢", pluginapi.Actions{FSList: fsList})
		if rows != 0 {
			t.Errorf("token 已经被空格结束，不该再弹菜单，得到 %d 行", rows)
		}
	})
}
