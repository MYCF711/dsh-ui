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
// 那个包（`dsh-client-ui-primitives`）在 rc.5 的解包树里**不出包**（338 个包逐个数过，
// `ls | grep -i primitiv` 为空，只有它的消费方），但它的 **CSS module 已完整编译进**
// web 产物：`dsh-web-frontend/dist/assets/index-CSAdOBx6.css`（92252 字节、整文件一行、
// sha256 `d487cca8d8a97f1e0cc5eca02343528c873e544eb7ec3a5b323a0f035553551d`）。
// 原语的 class hash 是 `70kcw`（行/分隔线/勾）、面壳是 `1w4el`，下文一律给该文件的
// **字节偏移**（`dd if=… bs=1 skip=<byte> count=180` 可复现；`grep -b` 同样能定位）。
// 消费侧的复刻读数仍然有效（`ModelSelect.module.css:109-117` 原文注释写明
// 「Surface tokens match the Menu primitive card (ui-primitives Menu.module.css)」）：
//
//	面壳（byte 19790/19861 的 `._surface_1w4el_1` + `._material_1w4el_21`）：
//	  border-radius: var(--dsw-radius-lg);                     ← 16，不是 md
//	  --dsw-elevation-stroke-color: var(--dsw-alias-border-l1);
//	  box-shadow: var(--dsw-elevation-prominent);
//	  background: var(--dsw-menu-surface-fill) + backdrop-filter
//	列表容器（byte 11530 的 `._list_70kcw_7`）：padding: 4px;
//	行（byte 15091 的 `._item_70kcw_173`）：
//	  min-height: 34px; padding: 6px 8px; border-radius: var(--dsw-radius-md);
//	  font-size: 13px; line-height: 20px; color: var(--dsw-alias-label-primary);
//	  :hover → background: var(--dsw-alias-interactive-bg-hover)（byte 15381）
//	dense 档（byte 15603）：min-height: 30px; padding-block: 4px
//
// # 为什么不让调用方各画一遍
//
// 面与行是同一个原语的外观。以前 `plugins/dsforge` 自己画了一份（范围菜单），
// `plugins/dsterminal` 又手绘了一份（shell 菜单，还因为手绘而占布局）。现在两处
// 都走这里，**读数只有一份**，将来补行高也只改这里。
//
// # 刻意不做的两件事
//
//   - **行高（⑧-a）**：读数**取得到**（base `._item{min-height:34px;padding:6px 8px}`
//     byte 15091、dense `._denseList ._item{min-height:30px;padding-block:4px}` byte 15603；
//     forge 范围菜单是 `dense` ⇒ 目标 30），但**改不动**：`ui.MenuItems.RowHeight`
//     只写 `rowSize`，而 `viva@v0.6.12-ow/ui/menu_paint.go` 的 `rowExtent` 只在
//     `GetContext()` 为真（右键菜单）时才读 `GetRowSize()`；普通 dropdown 走
//     `components.DropdownMenuItemHeight`，`DropdownMenuOptions` 没有行高字段。
//     所以不调 `RowHeight`、不自造 30/24，行高交给菜单按主题布，本层只在行盒里
//     居中一段 20 的行。
//   - **勾的位置**：原语 `selectedId` 的勾画在哪一侧读不到确定值（`._check_70kcw_265`
//     只给尺寸 14，不给 `order`/`margin-inline`）。能确定的只有顺序：`._itemLabel_70kcw_276
//     {flex:1}`（byte 16654）让标签占满剩余宽度、`._shortcut{margin-inline-start:auto}`
//     （byte 16753）把它推到行尾，所以勾在标签之后。本层沿用仓库既有那一档（行右缘、
//     贴 8 的内边距），两个菜单共用同一个画笔，至少不会互相矛盾；真正的读数补到时
//     只改这一处。
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
	// MenuCheckSize 是原语 `selectedId` 那枚 `IconCheckOutlineRegular` 的**渲染尺寸**：
	// `._itemIcon_70kcw_233 svg,._check_70kcw_265{width:14px;height:14px}`
	// （`index-CSAdOBx6.css` byte 16474）。注意 viewBox 与渲染尺寸不同：素材仍是
	// `viewBox 16`（`MenuCheckIcon`），画出来是 14。
	MenuCheckSize float32 = 14
	// 分隔线：`._separator_70kcw_157{height:.5px;margin:3px 8px;
	// background:var(--dsw-alias-border-l2)}`（同一份 CSS 的 byte 17672）。
	//
	// 竖向的 `3px` 是**外边距**，不是「距行顶多少」：官方这条线的 margin box 高
	// 6.5（3 + .5 + 3），线在盒里居中。Viva 的槽高是写死的 9
	// （`components/dropdown_menu.go` 的 `menuSeparatorHeight`，账本 #64 ⑧-e，
	// 上游没给出口），所以这里按「线在自己的 margin box 里居中」铺：
	// 槽高回到官方的 6.5 时，结果就是逐像素的「行顶 +3」。
	MenuSeparatorHeight float32 = 0.5
	// MenuSeparatorInset 是 `margin:3px 8px` 的**横向**那 8：左右各内缩 8。
	MenuSeparatorInset float32 = 8
	// MenuRowGap 是 `._item_70kcw_173{display:flex;align-items:center;gap:6px}`
	// （byte 15091）的 6：标签与勾之间的间距。
	MenuRowGap float32 = 6
)

// MenuSurfaceRadius 是菜单面的圆角：`._surface_1w4el_1,._backing_1w4el_2{
// border-radius:var(--dsw-radius-lg)}`（byte 19622；`--dsw-radius-lg: 16px` 见
// `dsh-client-ui-theme/src/styles/base.css:21`）。
//
// **不是 md**：只有 compact 档才是 `._compact_1w4el_6{border-radius:var(--dsw-radius-md)}`
// （byte 19693）。行自己的悬停底仍是 md（`._item_70kcw_173`，byte 15091），两者别混。
const MenuSurfaceRadius float32 = 16

// PaintMenuPanel 画官方菜单面：`--dsw-elevation-prominent` + `border-l1` + 面材质。
//
// 面材质用 `--dsw-specific-menu`（`pluginapi.Theme.Menu`，`design-platform.css:318`/`:469`
// 声明、`:480`/`:484` darwin 覆盖——旧账写的 `:290,414` / `:425/:429` 已过期约 28–55 行）
// ——它是官方唯一叫"菜单材质"的令牌。
//
// 顺序与 viva 自己的 `paintPanel` 一致（`components/dialog.go:470-477`）：先阴影、
// 再填面。`PaintElevationOverlay` 一次画描边 + 两层柔光，所以填面放在它之后，
// 把落进面内的那点柔光盖住，只留面外那一圈。
func PaintMenuPanel(canvas *gfx.Canvas, bounds gfx.Rect, t *pluginapi.Theme) {
	if canvas == nil || t == nil {
		return
	}
	PaintElevationOverlay(canvas, bounds, MenuSurfaceRadius, t.Shadows.Prominent, t.BorderL1)
	canvas.Rounded(bounds, MenuSurfaceRadius, t.Menu)
}

// PaintMenuItem 画菜单的一行：分隔线、悬停底、标签、选中勾。
//
// 颜色里凡是菜单自己也有语义的（行字色 `Foreground`、悬停态 `Active`）都用菜单给的，
// 不另判一遍——`pluginapi` 里 `ui.MenuItemParts` 就是为此存在；几何与字号用官方读数。
//
// **例外是分隔线**：官方 `._separator_70kcw_157` 自己写死
// `background:var(--dsw-alias-border-l2)`，不看行的字色，所以那一条用 `t.BorderL2`。
func PaintMenuItem(parts ui.MenuItemParts, t *pluginapi.Theme) {
	canvas := parts.Canvas
	if canvas == nil || t == nil {
		return
	}
	row := parts.Rect
	if parts.Separator {
		// 官方 `._separator_70kcw_157`：高 .5、左右各内缩 8、颜色是
		// `--dsw-alias-border-l2`（**不是**菜单行给的字色 `Foreground`），
		// 竖向按「线在自己的 margin box 里居中」铺（见 `MenuSeparatorHeight` 注释）。
		canvas.Fill(gfx.Rect{
			X:      row.X + MenuSeparatorInset,
			Y:      row.Y + (row.Height-MenuSeparatorHeight)/2,
			Width:  max(row.Width-2*MenuSeparatorInset, 0),
			Height: MenuSeparatorHeight,
		}, t.BorderL2)
		return
	}
	// 行高由菜单按主题算，这里只把 20 的行盒在其中居中；行比行盒矮时贴顶，不越界。
	lineY := row.Y + max((row.Height-MenuRowLine)/2, 0)
	if parts.Active {
		// `._item_70kcw_173:hover{background:var(--dsw-alias-interactive-bg-hover)}`
		// （byte 15381），圆角同 `._item`（`--dsw-radius-md`，byte 15091）。
		canvas.Rounded(row, t.Radii.MD, t.Hover)
	}
	chosen := parts.Checked || parts.Selected
	labelWidth := max(row.Width-2*MenuRowPadX, 0)
	checkY := lineY
	if chosen {
		// 官方 `.item{display:flex;align-items:center;gap:6px}`（byte 15091）：
		// 标签与勾之间隔 `MenuRowGap`，勾在行里**竖直居中**（不是与 20 的行盒顶对齐）。
		labelWidth = max(labelWidth-MenuCheckSize-MenuRowGap, 0)
		checkY = lineY + max((MenuRowLine-MenuCheckSize)/2, 0)
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
	// 渲染 14×14（`._check_70kcw_265`），viewBox 仍是 16。
	if icon, ok := MenuCheckIcon(); ok {
		canvas.DrawIcon(icon, gfx.Rect{
			X: row.X + row.Width - MenuRowPadX - MenuCheckSize, Y: checkY,
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
