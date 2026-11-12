package dsshell

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// TestShellGeometryAndRealSlots 不只测宽度求解：把真实 flex 树布局出来，
// 逐容器检查主区、右栏、composer 与底部整行的矩形。
// 三栏外框来自官方 computeColumns；slot 保留现有跨插件 12px inset 契约。
func TestShellGeometryAndRealSlots(t *testing.T) {
	cases := []struct {
		name              string
		width, height     float32
		mainWidth, rightX float32
		rightWidth        float32
	}{
		{name: "1920x1032", width: 1920, height: 1032, mainWidth: 776, rightX: 1056, rightWidth: 864},
		{name: "1182x832", width: 1182, height: 832, mainWidth: 400, rightX: 680, rightWidth: 502},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			theme := dscore.Dark()
			p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}, SidebarWidth: 280}
			tree := &gfx.Tree{}
			feat := tree.Feature("shell-geometry-test")
			canvas := &gfx.Canvas{}
			canvas.SetTree(tree)
			canvas.SetMeasurer(func(value string, size, weight float32) float32 {
				return float32(len([]rune(value))) * size / 2
			})
			bounds := ui.Rect{Width: tc.width, Height: tc.height}
			rightPref := tc.width * rightbarOpenFrac
			env := &pluginapi.Env{
				Theme: theme,
				Data:  pluginapi.Snapshot{ShowRightbar: true},
				Tree:  tree, Feat: feat, Canvas: canvas, Bounds: bounds,
				Geometry: pluginapi.Geometry{
					Width: tc.width, Height: tc.height, SidebarWidth: 280,
					RightWidth: rightPref, MainWidth: tc.mainWidth, BottomHeight: 24,
				},
				MainWidth: tc.mainWidth,
				Has:       func(slot pluginapi.Slot) bool { return slot != pluginapi.SlotShellOverlay },
			}

			boxes := map[pluginapi.Slot]ui.Box{}
			var view ui.Page
			view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f ui.C) {
				p.draw(f, env)
				p.children(f, env, func(slot pluginapi.Slot) {
					switch slot {
					case pluginapi.SlotMainHeader:
						boxes[slot] = ui.Slot(f).FillWidth().Height(40)
					case pluginapi.SlotMainBody:
						// 故意用一个自身不 Grow 的占用方：壳必须给它可伸展的区域，
						// 否则空态会把 composer 提到顶部。
						boxes[slot] = ui.Slot(f).FillWidth()
					case pluginapi.SlotMainComposer:
						boxes[slot] = ui.Slot(f).FillWidth().Height(80)
					case pluginapi.SlotRightbar:
						boxes[slot] = ui.Slot(f).FillWidth().FillHeight()
					case pluginapi.SlotShellBottom:
						boxes[slot] = ui.Slot(f).FillWidth().FillHeight()
					default:
						boxes[slot] = ui.Slot(f).FillWidth()
					}
				})
			})

			assertRect(t, "main.header", boxes[pluginapi.SlotMainHeader].Rect(), ui.Rect{X: 280 + columnPad, Y: columnPad, Width: tc.mainWidth - 2*columnPad, Height: 40})
			assertRect(t, "main.body", boxes[pluginapi.SlotMainBody].Rect(), ui.Rect{X: 280 + columnPad, Y: columnPad + 40 + 8, Width: tc.mainWidth - 2*columnPad})
			assertRect(t, "rightbar", boxes[pluginapi.SlotRightbar].Rect(), ui.Rect{X: tc.rightX + columnPad, Y: columnPad, Width: tc.rightWidth - 2*columnPad, Height: tc.height - 24 - 2*columnPad})
			assertRect(t, "main.composer", boxes[pluginapi.SlotMainComposer].Rect(), ui.Rect{X: 280 + columnPad, Y: tc.height - 24 - columnPad - 80, Width: tc.mainWidth - 2*columnPad, Height: 80})
			assertRect(t, "shell.bottom", boxes[pluginapi.SlotShellBottom].Rect(), ui.Rect{Y: tc.height - 24, Width: tc.width, Height: 24})

			wantRight := ui.Rect{X: tc.rightX, Width: tc.rightWidth, Height: tc.height - 24}
			found := false
			for _, r := range canvas.Rectangles() {
				if r.Bounds == wantRight && r.Color == theme.SidebarFill {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("右栏外框应按当前主题 SidebarFill 实画 %+v；rectangles=%+v", wantRight, canvas.Rectangles())
			}
			t.Logf("V4A_GEOMETRY viewport=%.0fx%.0f sidebar={x:0,width:280} main={x:280,width:%.0f} right={x:%.0f,width:%.0f} bodyHeight=%.0f bottom=%+v composer=%+v",
				tc.width, tc.height, tc.mainWidth, tc.rightX, tc.rightWidth, tc.height-24,
				boxes[pluginapi.SlotShellBottom].Rect(), boxes[pluginapi.SlotMainComposer].Rect())
		})
	}
}

// TestShellDropsRightTrackWhenItCannotFit 用真实 flex 树验证缩放边界，
// 防止只改求解器却仍在 drawRightbar 里硬塞 300px。
func TestShellDropsRightTrackWhenItCannotFit(t *testing.T) {
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}, SidebarWidth: 280}
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 2 })
	bounds := ui.Rect{Width: 960, Height: 832}
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{ShowRightbar: true}, Tree: tree,
		Feat: tree.Feature("shell-narrow-test"), Canvas: canvas, Bounds: bounds,
		Geometry:  pluginapi.Geometry{Width: 960, Height: 832, SidebarWidth: 280, RightWidth: 300, MainWidth: 320, BottomHeight: 24},
		MainWidth: 320,
		Has:       func(slot pluginapi.Slot) bool { return slot != pluginapi.SlotShellOverlay },
	}
	rightDrawn := false
	var main ui.Box
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f ui.C) {
		p.children(f, env, func(slot pluginapi.Slot) {
			switch slot {
			case pluginapi.SlotRightbar:
				rightDrawn = true
			case pluginapi.SlotMainBody:
				main = ui.Slot(f).FillWidth()
			case pluginapi.SlotShellBottom:
				ui.Slot(f).FillWidth().FillHeight()
			default:
				ui.Slot(f).FillWidth()
			}
		})
	})
	if rightDrawn {
		t.Fatal("960 宽时官方 available=280<300，右栏必须整列消失")
	}
	if got := main.Rect(); got.X != 280+columnPad || got.Width != 680-2*columnPad {
		t.Fatalf("右栏消失后主区外框应为 680px（slot 扣双侧 inset），得到 %+v", got)
	}
}

func assertRect(t *testing.T, name string, got, want ui.Rect) {
	t.Helper()
	const tolerance = 0.01
	if abs32(got.X-want.X) > tolerance || abs32(got.Y-want.Y) > tolerance ||
		abs32(got.Width-want.Width) > tolerance || abs32(got.Height-want.Height) > tolerance {
		t.Fatalf("%s 几何 = %+v，期望 %+v", name, got, want)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
