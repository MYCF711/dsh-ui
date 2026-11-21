// changes_menu.go —— 范围菜单的**面与行**（`ui.MenuItems.Unpainted` 的调用方）。
//
// # 为什么要自己画面
//
// 官方范围菜单是 `primitives.Menu`（`OFF:lib/client.js:6436-6479`）：
// `portal: true` / `align: "start"` / `dense` / `selectedId`。它的面在官方 CSS 里
// 只写这三个令牌（`ui-model-selection/src/client/ModelSelect.module.css:109-117`
// 是同一套 primitives `Menu` 面，原文注释写明"照 Menu 原语复刻"）：
//
//	border: 0;                                    ← 面自己**没有** border
//	--dsw-elevation-stroke-color: border-l1;      ← 那圈 0.5px 描边是阴影第一层画的
//	box-shadow: var(--dsw-elevation-prominent);
//
// Viva v0.6.5 的 `ui.Menu` 默认面把阴影写死成 `gfx.ShadowMD`
// （`viva@v0.6.5/components/dropdown_menu.go:813`），而 `gfx.ShadowMD` 是 Tailwind
//
//	`0 4px 6px -1px .10 + 0 2px 4px -2px .10`（`viva@v0.6.5/gfx/canvas.go:123`），
//
// 与官方 `--dsw-elevation-prominent`（0 0 0 .5px 描边 + 0 3px 8px .04 + 0 0 20px .05，
// `dscore/shadow.go` 头部逐行抄的 `gradient-shadow-text.css:36-37`）是两套东西。
//
// v0.6.4 之前没有出口，只能"保留浮层、阴影偏一档"（旧注释里那句已过期）。v0.6.5 给了
// 整套出口：`MenuItems.Unpainted()`（`ui/menu.go:377`）让菜单**只声明行为不画面**，
// `DrawPanel`（`:382`）画面、`DrawItem`（`:389`）画行；`ui/menu.go:369-376` 原文写明
// hits / focus / keyboard / dismissal / submenu **仍是菜单自己的**，应用自画的菜单
// 也不可能表现出不同行为。
//
// # 面与行的读数现在只有一份
//
// 实现搬到了 `plugins/dscore/menu.go`（`Menu` 原语的外观定义处）：终端 shell 菜单
// （`plugins/dsterminal/guide.go`）用同一套面与行，两处不能各写一份。这个文件保留
// 原来的三个名字（`drawScopeMenuPanel` / `drawScopeMenuItem` / `scopeMenuCheckIcon`）
// 与常量，供本包的调用点与判据继续用，**内部一律转发**，不在这里再写一遍绘图。
package dsforge

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方菜单行/面的几何（`ui-model-selection/src/client/ModelSelect.module.css`）。
// 定义在 `plugins/dscore/menu.go`，这里给本包判据留同名的本地常量。
const (
	// `.cell{padding:0 8px}`（:348）——标签左缘、勾的右缘都用这一档。
	scopeMenuRowPadX = dscore.MenuRowPadX
	// `.cell{font-size:13px;line-height:20px}`（:353-354）。
	scopeMenuRowFont = dscore.MenuRowFont
	scopeMenuRowLine = dscore.MenuRowLine
	// 选中项的勾：primitives 的 `IconCheckOutlineRegular`，sprite `size:16`。
	scopeMenuCheckSize = dscore.MenuCheckSize
	// 分隔线的几何沿用菜单自己那一档（`viva@v0.6.5/components/dropdown_menu.go:974-978`）。
	scopeMenuSeparatorTop    = dscore.MenuSeparatorTop
	scopeMenuSeparatorHeight = dscore.MenuSeparatorHeight
)

// drawScopeMenuPanel 画官方菜单面：`--dsw-elevation-prominent` + `border-l1` + 面材质。
//
// 实现与出处见 `plugins/dscore/menu.go` 的 `PaintMenuPanel`。
func drawScopeMenuPanel(canvas *gfx.Canvas, bounds gfx.Rect, t *pluginapi.Theme) {
	dscore.PaintMenuPanel(canvas, bounds, t)
}

// drawScopeMenuItem 画范围菜单的一行：分隔线、悬停底、标签、选中勾。
//
// 实现与出处见 `plugins/dscore/menu.go` 的 `PaintMenuItem`。
func drawScopeMenuItem(parts ui.MenuItemParts, t *pluginapi.Theme) {
	dscore.PaintMenuItem(parts, t)
}

// scopeMenuCheckIcon 是官方 `IconCheckOutlineRegular`（16 viewBox、1 描边）。
func scopeMenuCheckIcon() (gfx.Icon, bool) {
	return dscore.MenuCheckIcon()
}
