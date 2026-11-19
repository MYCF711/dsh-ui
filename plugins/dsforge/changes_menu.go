// changes_menu.go —— 范围菜单的**面与行**（`ui.MenuItems.Unpainted` 的调用方）。
//
// # 为什么要自己画面
//
// 官方范围菜单是 `primitives.Menu`（`OFF:lib/client.js:6436-6479`）：
// `portal: true` / `align: "start"` / `dense` / `selectedId`。它的面在官方 CSS 里
// 只写这三个令牌（`ui-model-selection/src/client/ModelSelect.module.css:107-118`
// 是同一套 primitives `Menu` 面）：
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
// 也不可能表现出不同行为。所以这里把面与行接过来，阴影用 `dscore.PaintElevationOverlay`
// 的官方令牌。
//
// # 行几何的出处
//
// 行的读数取 primitives `Menu` 面里那一档 `.cell`（同一份 CSS）：
//
//	.cell{height:34px;padding:0 8px;border-radius:var(--dsw-radius-md);
//	      color:var(--dsw-alias-label-primary);font-size:13px;line-height:20px}
//	.cell:hover{background:var(--dsw-alias-interactive-bg-hover)}
//
// # 这个 painter 覆盖哪些行
//
// 只覆盖 `ScopeMenu` 实际声明出来的三种：普通项、选中项（打勾）、分隔线。
// `MenuItemParts` 还带 `Description`（第二行说明）、`Shortcut`/`Keys`、
// `Expanded`（子菜单）与 `Icon`——本菜单一个都没有，而那几档的行内几何/字号在
// 官方可用产物里没有读数，所以**不预先编**：真要用时按官方读数补同一处，
// 不在这里猜一个位置。
//
// 行**高度**不让调用方给：`ui.Menu` 按主题的控制高度布行（`RowHeight` 只用于
// 官方能取到读数的档位）。官方 `Menu dense` 的每行高度在 rc.2 的可用产物里**没有
// 读数**（`_denseList_70kcw_209 ._item_70kcw_173{min-height:30px;padding-block:4px}`
// 属于入口菜单那套组件，不是 primitives `Menu` 的 dense 行；primitives 包不在 rc.2
// 解包树内），所以**不调** `RowHeight`，也不自造 30/24（账本 #64 ⑧ 的行高半）。
package dsforge

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方菜单行/面的几何（`ui-model-selection/src/client/ModelSelect.module.css`）。
const (
	// `.cell{padding:0 8px}`（:347）——标签左缘、勾的右缘都用这一档。
	scopeMenuRowPadX float32 = 8
	// `.cell{font-size:13px;line-height:20px}`（:353-354）。
	scopeMenuRowFont float32 = 13
	scopeMenuRowLine float32 = 20
	// 选中项的勾：primitives 的 `IconCheckOutlineRegular`，sprite `size:16`
	// （`ICON_REGULAR_STROKE` 同行，`dsh-web-frontend/dist/assets/index-ghOpC9uV.js`），
	// 与 dsterminal 的 shell 菜单同一枚（`plugins/dsterminal/icons.go:48-56`）。
	scopeMenuCheckSize float32 = 16
	// 分隔线的几何沿用菜单自己那一档（官方菜单面 CSS 不给分隔线读数）：
	// `viva@v0.6.5/components/dropdown_menu.go:974-978` 是 `Fill(y+4, h=1, theme.Border)`。
	scopeMenuSeparatorTop    float32 = 4
	scopeMenuSeparatorHeight float32 = 1
)

// drawScopeMenuPanel 画官方菜单面：`--dsw-elevation-prominent` + `border-l1` + 面材质。
//
// 面材质用 `--dsw-specific-menu`（`pluginapi.Theme.Menu`，`design-platform.css:290,414`
// 声明、`:425/:429` 亮暗两档覆盖）——它就叫"菜单材质"，与手绘的 shell 菜单
// （`plugins/dsterminal/guide.go:358-363` 的 `Background(t.Menu)`）同一档。
//
// 顺序与 viva 自己的 `paintPanel` 一致（`components/dialog.go:470-477`）：先阴影、
// 再填面。`PaintElevationOverlay` 一次画描边 + 两层柔光，所以填面放在它之后，
// 把落进面内的那点柔光盖住，只留面外那一圈。
func drawScopeMenuPanel(canvas *gfx.Canvas, bounds gfx.Rect, t *pluginapi.Theme) {
	if canvas == nil || t == nil {
		return
	}
	dscore.PaintElevationOverlay(canvas, bounds, t.Radii.MD, t.Shadows.Prominent, t.BorderL1)
	canvas.Rounded(bounds, t.Radii.MD, t.Menu)
}

// drawScopeMenuItem 画范围菜单的一行：分隔线、悬停底、标签、选中勾。
//
// 颜色里凡是菜单自己也有语义的（`Foreground`、`Active`）都用菜单给的，不另判一遍
// ——`ui/menu.go:345-360` 的 `MenuItemParts` 就是为此存在；几何与字号用官方读数。
func drawScopeMenuItem(parts ui.MenuItemParts, t *pluginapi.Theme) {
	canvas := parts.Canvas
	if canvas == nil || t == nil {
		return
	}
	row := parts.Rect
	if parts.Separator {
		canvas.Fill(gfx.Rect{
			X: row.X, Y: row.Y + scopeMenuSeparatorTop,
			Width: row.Width, Height: scopeMenuSeparatorHeight,
		}, parts.Foreground)
		return
	}
	// 行高由菜单按主题算，这里只把 20 的行盒在其中居中；行比行盒矮时贴顶，不越界。
	lineY := row.Y + max((row.Height-scopeMenuRowLine)/2, 0)
	if parts.Active {
		// `.cell:hover{background:var(--dsw-alias-interactive-bg-hover)}`（:348-352），
		// 圆角同 `.cell`（`--dsw-radius-md`）。
		canvas.Rounded(row, t.Radii.MD, t.Hover)
	}
	chosen := parts.Checked || parts.Selected
	labelWidth := max(row.Width-2*scopeMenuRowPadX, 0)
	if chosen {
		labelWidth = max(labelWidth-scopeMenuCheckSize-scopeMenuRowPadX, 0)
	}
	label := parts.Label
	if canvas.TextWidthWeight(label, scopeMenuRowFont, gfx.WeightRegular) > labelWidth {
		label = canvas.FitLine(label, scopeMenuRowFont, gfx.WeightRegular, labelWidth)
	}
	canvas.Label(label, gfx.Rect{
		X: row.X + scopeMenuRowPadX, Y: lineY, Width: labelWidth, Height: scopeMenuRowLine,
	}, gfx.TextStyle{Size: scopeMenuRowFont, Weight: gfx.WeightRegular, Color: parts.Foreground})
	if !chosen {
		return
	}
	// 勾是官方素材，不是手画的：`IconCheckOutlineRegular` 的 path 原文在 dsassets。
	if icon, ok := scopeMenuCheckIcon(); ok {
		canvas.DrawIcon(icon, gfx.Rect{
			X: row.X + row.Width - scopeMenuRowPadX - scopeMenuCheckSize, Y: lineY,
			Width: scopeMenuCheckSize, Height: scopeMenuCheckSize,
		}, parts.Foreground)
	}
}

// scopeMenuCheckIcon 是官方 `IconCheckOutlineRegular`（16 viewBox、1 描边）。
//
// 与 `plugins/dsterminal/icons.go:48-56` 的 `terminalCheckIcon` 同一枚素材：
// 官方 `primitives.Menu` 用它在 `selectedId` 那一行打勾。
func scopeMenuCheckIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.CheckOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}
