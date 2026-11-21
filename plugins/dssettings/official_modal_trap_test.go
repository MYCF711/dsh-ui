//go:build darwin || windows || linux

// official_modal_trap_test.go —— 设置模态的**键盘焦点必须留在面板里**。
//
// 官方 `useModalLayer(panel, true, onClose)` 的语义（结构树 §2.2）里有一条是
// **root inert**：对话框打开时，背后的界面既不能点、也不能被 Tab 走到。本仓的
// `ModalLayer`（official_modal.go）只在插件自己的键处理里数 Tab —— 而 viva 的
// Tab 走 `Router` 的焦点遍历，**根本不进 `kernel.HandleKey`**。所以真正把焦点
// 关在面板里的是渲染器的 focus scope + trap（与右键菜单
// `dscontextmenu/context_menu.go:328-336` 同一套）。
//
// 判据只有两条，都是真键盘：
//
//	① Tab 的落点必须落在面板卡片里（不是遮罩背后的侧栏行）；
//	② 遮罩本身不是 Tab 停靠点（整窗的关闭面不该被键盘选中）。
//
// 2026-11-22 修之前实测：设置打开后 Tab 1..14 全在遮罩背后（新建会话 / 侧栏行 /
// 删除 × ……），面板里的「关闭」要到第 40 多次才轮得到。
package dssettings

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// modalCardRect 是本帧面板卡片的矩形（与绘制用的是同一个求解器）。
func modalCardRect(t *testing.T, width, height float32) gfx.Rect {
	t.Helper()
	layout, err := LayoutSettingsPanel(width, height, PanelOverlayTop)
	if err != nil {
		t.Fatalf("布局求解失败：%v", err)
	}
	return gfx.Rect{X: layout.X, Y: layout.Y, Width: layout.Width, Height: layout.Height}
}

func TestSettingsModalTrapsKeyboardFocus(t *testing.T) {
	const width, height = 1180, 800
	h := openOfficialHarness(t, width, height)
	defer h.close()
	if !h.p.Open() {
		t.Fatal("设置打不开")
	}
	h.pump()
	h.pump()

	card := modalCardRect(t, width, height)
	hitByID := func(id gfx.ID) (gfx.Hit, bool) {
		for _, hit := range h.session.Snapshot().Hits {
			if hit.ID == id {
				return hit, true
			}
		}
		return gfx.Hit{}, false
	}
	snap := h.session.Snapshot()
	var mask gfx.ID
	for _, hit := range snap.Hits {
		if hit.Backdrop {
			mask = hit.ID
			break
		}
	}

	// ① Tab 全程留在卡片里。连按 16 次，每一次的焦点都必须在卡片矩形内。
	for i := 0; i < 16; i++ {
		if err := h.session.Keys("tab"); err != nil {
			t.Fatalf("第 %d 次 Tab：%v", i+1, err)
		}
		focused := h.session.Snapshot().Focused
		if focused.IsZero() {
			t.Fatalf("第 %d 次 Tab 之后没有焦点（焦点掉出面板就是键盘用户走丢）", i+1)
		}
		hit, ok := hitByID(focused)
		if !ok {
			t.Fatalf("第 %d 次 Tab 的焦点 %v 不是本帧命中区", i+1, focused)
		}
		center := gfx.Point{X: hit.Bounds.X + hit.Bounds.Width/2, Y: hit.Bounds.Y + hit.Bounds.Height/2}
		if center.X < card.X || center.Y < card.Y ||
			center.X >= card.X+card.Width || center.Y >= card.Y+card.Height {
			t.Fatalf("第 %d 次 Tab 的焦点 %q 在面板卡片外面：焦点 %+v，卡片 %+v\n"+
				"模态打开时背景必须是 inert（官方 useModalLayer 的 root inert）",
				i+1, hit.Label, hit.Bounds, card)
		}
		// ② 遮罩不是 Tab 停靠点。
		if !mask.IsZero() && focused == mask {
			t.Fatalf("第 %d 次 Tab 落在遮罩上：整窗关闭面不该是 Tab 停靠点（SkipFocus）", i+1)
		}
	}
}
