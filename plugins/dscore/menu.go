// menu.go —— primitives `Menu` 的**面**与**行**（官方读数只有这一份定义）。
//
// # 为什么要有这一层
//
// 官方所有下拉菜单都是同一个原语 `@deepseek-ai/dsh-client-ui-primitives` 的 `Menu`
// （`OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalGuide.tsx:62-77`、
//
//	`OFF:dsh-client-ui-model-selection/src/client/ModelSelect.module.css:113-116`
//	的注释原文「Surface tokens match the Menu primitive card (ui-primitives
//	Menu.module.css) so every dropdown reads as the same material」）。
//
// 那个包在 rc.5 的解包树里**不存在**（338 个包逐个数过，只有它的消费方），所以面的
// 令牌只能从**消费方复刻它的那段 CSS** 取；行几何同理。两个已知的确凿读数：
//
//	面（ModelSelect.module.css:109-117，原文注释写明是照 Menu 原语复刻）：
//	  border: 0;
//	  --dsw-elevation-stroke-color: var(--dsw-alias-border-l1);   ← 那圈 0.5px 描边
//	  box-shadow: var(--dsw-elevation-prominent);
//	  padding: 4px;
//	行（ModelSelect.module.css:340-361 的 `.cell`）：
//	  height: 34px; padding: 0 8px; border-radius: var(--dsw-radius-md);
//	  color: var(--dsw-alias-label-primary); font-size: 13px; line-height: 20px;
//	  .cell:hover / .cell:focus-visible → background: var(--dsw-alias-interactive-bg-hover);
//
// # 为什么不让调用方各画一遍
//
// 面与行是同一个原语的外观。以前 `plugins/dsforge` 自己画了一份（范围菜单），
// `plugins/dsterminal` 又手绘了一份（shell 菜单，还因为手绘而占布局）。现在两处
// 都走这里，**读数只有一份**，将来补行高也只改这里。
//
// # 刻意不做的两件事（没有官方读数就不编）
//
//   - **行高**：`Menu` 原语的每行高度取不到（`ModelSelect` 的 `.cell{height:34px}`
//     是它自己复刻的行，不是原语的；`primitives/Menu.module.css` 不在 rc.5 树内）。
//     所以不调 `ui.MenuItems.RowHeight`，行高交给菜单按主题布，本层只在行盒里
//     居中一段 20 的行。
//   - **勾的位置**：原语 `selectedId` 的勾画在哪一侧同样没有 CSS 读数。本层沿用
//     仓库既有那一档（行右缘、贴 8 的内边距），两个菜单共用同一个画笔，至少不会
//     互相矛盾；真正的读数补到时只改这一处。
package dscore

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方菜单行/面的几何（出处在文件头）。
const (
	// MenuRowPadX 是 `.cell{padding:0 8px}`（ModelSelect.module.css:348）：标签左缘、
	// 勾的右缘都用这一档。
	MenuRowPadX float32 = 8
	// MenuRowFont / MenuRowLine 是 `.cell{font-size:13px;line-height:20px}`（:353-354）。
	MenuRowFont float32 = 13
	MenuRowLine float32 = 20
	// MenuCheckSize 是原语 `selectedId` 那枚 `IconCheckOutlineRegular`：sprite `size:16`
	// （`dsh-web-frontend/dist/assets/index-ghOpC9uV.js` 的 ICON_REGULAR_STROKE 同段）。
	MenuCheckSize float32 = 16
	// 分隔线的几何沿用菜单自己那一档（官方 `.cell` 不给分隔线读数）：
	// `viva@v0.6.5/components/dropdown_menu.go:974-978` 是 `Fill(y+4, h=1, theme.Border)`。
	MenuSeparatorTop    float32 = 4
	MenuSeparatorHeight float32 = 1
)

// PaintMenuPanel 画官方菜单面：`--dsw-elevation-prominent` + `border-l1` + 面材质。
//
// 面材质用 `--dsw-specific-menu`（`pluginapi.Theme.Menu`，`design-platform.css:290,414`
// 声明、`:425/:429` 亮暗两档覆盖）——它是官方唯一叫"菜单材质"的令牌。
//
// 顺序与 viva 自己的 `paintPanel` 一致（`components/dialog.go:470-477`）：先阴影、
// 再填面。`PaintElevationOverlay` 一次画描边 + 两层柔光，所以填面放在它之后，
// 把落进面内的那点柔光盖住，只留面外那一圈。
func PaintMenuPanel(canvas *gfx.Canvas, bounds gfx.Rect, t *pluginapi.Theme) {
	if canvas == nil || t == nil {
		return
	}
	PaintElevationOverlay(canvas, bounds, t.Radii.MD, t.Shadows.Prominent, t.BorderL1)
	canvas.Rounded(bounds, t.Radii.MD, t.Menu)
}

// PaintMenuItem 画菜单的一行：分隔线、悬停底、标签、选中勾。
//
// 颜色里凡是菜单自己也有语义的（`Foreground`、`Active`）都用菜单给的，不另判一遍
// ——`pluginapi` 里 `ui.MenuItemParts` 就是为此存在；几何与字号用官方读数。
func PaintMenuItem(parts ui.MenuItemParts, t *pluginapi.Theme) {
	canvas := parts.Canvas
	if canvas == nil || t == nil {
		return
	}
	row := parts.Rect
	if parts.Separator {
		canvas.Fill(gfx.Rect{
			X: row.X, Y: row.Y + MenuSeparatorTop,
			Width: row.Width, Height: MenuSeparatorHeight,
		}, parts.Foreground)
		return
	}
	// 行高由菜单按主题算，这里只把 20 的行盒在其中居中；行比行盒矮时贴顶，不越界。
	lineY := row.Y + max((row.Height-MenuRowLine)/2, 0)
	if parts.Active {
		// `.cell:hover{background:var(--dsw-alias-interactive-bg-hover)}`（:361），
		// 圆角同 `.cell`（`--dsw-radius-md`）。
		canvas.Rounded(row, t.Radii.MD, t.Hover)
	}
	chosen := parts.Checked || parts.Selected
	labelWidth := max(row.Width-2*MenuRowPadX, 0)
	if chosen {
		labelWidth = max(labelWidth-MenuCheckSize-MenuRowPadX, 0)
	}
	label := parts.Label
	if canvas.TextWidthWeight(label, MenuRowFont, gfx.WeightRegular) > labelWidth {
		label = canvas.FitLine(label, MenuRowFont, gfx.WeightRegular, labelWidth)
	}
	canvas.Label(label, gfx.Rect{
		X: row.X + MenuRowPadX, Y: lineY, Width: labelWidth, Height: MenuRowLine,
	}, gfx.TextStyle{Size: MenuRowFont, Weight: gfx.WeightRegular, Color: parts.Foreground})
	if !chosen {
		return
	}
	// 勾是官方素材，不是手画的：`IconCheckOutlineRegular` 的 path 原文在 dsassets。
	if icon, ok := MenuCheckIcon(); ok {
		canvas.DrawIcon(icon, gfx.Rect{
			X: row.X + row.Width - MenuRowPadX - MenuCheckSize, Y: lineY,
			Width: MenuCheckSize, Height: MenuCheckSize,
		}, parts.Foreground)
	}
}

// MenuCheckIcon 是官方 `IconCheckOutlineRegular`（16 viewBox、1 描边）。
//
// 原语 `Menu` 用它在 `selectedId` 那一行打勾；本仓的 forge 范围菜单与终端 shell
// 菜单都用同一枚（`plugins/dsterminal/icons.go` 的 `terminalCheckIcon` 同源）。
func MenuCheckIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.CheckOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}
