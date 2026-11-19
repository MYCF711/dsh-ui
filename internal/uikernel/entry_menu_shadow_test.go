package uikernel

// entry_menu_shadow_test.go —— 钉住区域 More 菜单面的阴影是**官方
// `--dsw-elevation-prominent`**，不是 Viva 的 `gfx.ShadowMD` 预设档（#64 的 ⑧）。
//
// 官方依据（`entry_menu.go` 文件头本来就写着，代码却用了预设档）：
// primitives 的 MenuSurface = padding 4 / radius md / **elevation-prominent**
// （`ModelSelect.module.css:107-135`，与模型菜单面同一档）。
//
// `--dsw-elevation-prominent`（gradient-shadow-text.css:36-37）=
// 0.5px 描边 + `0 3px 8px 0 rgba(0,0,0,.04)` + `0 0 20px 0 rgba(0,0,0,.05)`。
// `gfx.ShadowMD` 是 Tailwind 的 `0 4px 6px -1px .10` + `0 2px 4px -2px .10`：
// 位移/模糊/负扩展/透明度没有一项对，而且**没有描边层**。
//
// 判据双向：① 画布上出现 prominent 的两层柔光；② 不出现 shadow-md 的两层。
// ② 让"改回 ShadowMD"这种回退立刻红。
//
// 用 `renderContainer`（真 Canvas + 测量函数）而不是 `newSkeletonEnv`：后者是给
// dispatch/props 那几条断言用的最小装配，画不出菜单面（实测画布上零矩形）。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// TestEntryMoreMenuPaintsOfficialProminentElevation 钉区域 More 菜单面的阴影档。
func TestEntryMoreMenuPaintsOfficialProminentElevation(t *testing.T) {
	theme := menuTheme()
	bounds := gfx.Rect{Width: 1182, Height: 832}
	env := containerEnv(bounds.Width, bounds.Height)
	env.Theme = theme

	dispatched := 0
	canvas := renderContainer(t, bounds.Width, bounds.Height, env, func(f ui.C) {
		DrawEntryMoreMenu(f, env, EntryMenuOptions{
			Open: true, Slot: OffSidebarPanelActions,
			Region:  pluginapi.EntryRegionSidebar,
			Entries: entryMenuEntries(),
			Anchor:  gfx.Rect{X: 12, Y: 300, Width: 256, Height: 36},
			Bounds:  bounds,
			Theme:   theme,
			Dispatch: func(pluginapi.Slot) {
				dispatched++
			},
		})
	})

	if dispatched != 1 {
		t.Fatalf("座位 dispatch 次数 = %d，want 1（菜单面没走到绘制分支就断言不了阴影）", dispatched)
	}
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("菜单该画面，却一个矩形都没有")
	}

	shadows := canvas.Shadows()
	if len(shadows) == 0 {
		t.Fatal("菜单面一个阴影层都没画（应有两层 prominent 柔光 + 0.5px 描边）")
	}

	for i, layer := range theme.Shadows.Prominent.Glows {
		if layer.Alpha <= 0 {
			t.Fatalf("主题的 Prominent 第 %d 层没有透明度：%+v（令牌没接上）", i, layer)
		}
		if !hasGlow(shadows, layer) {
			t.Fatalf("画布上没有官方 prominent 第 %d 层 %+v；实际阴影 %+v",
				i, layer, styleList(shadows))
		}
	}
	if hasShadowMDPreset(shadows) {
		t.Fatalf("菜单面用了 Tailwind shadow-md 预设档；官方是 elevation-prominent：%+v",
			styleList(shadows))
	}
}
