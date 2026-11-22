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
//
// 它是 `PaintMenuItemRuns` 的单段形态（一段 = 行自己的 `Label`，13px、行墨色、可压缩）：
// 行内容拆成多段时（forge 基准分支菜单的名字 + 「当前分支」注记）走那个函数，画笔仍只有
// 这一份。
func PaintMenuItem(parts ui.MenuItemParts, t *pluginapi.Theme) {
	PaintMenuItemRuns(parts, t, []MenuRun{{Text: parts.Label, Size: MenuRowFont, Flex: true}})
}

// MenuRun 是菜单行里的一段文字。
//
// 原语 `Menu` 的行内容可以是一个节点而不是一句话，forge 的基准分支菜单就用到了这一点：
// `WorktreeToggle.tsx:69-72` 把当前分支那一行拆成 `<span class=option>` 里的两段
// `optionName` 与 `optionNote`。所以行画笔要能画若干段，而不是只画一个 `Label`。
type MenuRun struct {
	// Text 是这一段的文字。
	Text string
	// Size 是这一段的字号（官方 `.optionName`/`.optionNote` 都是 12px，
	// `WorktreeToggle.module.css:99-110`）。
	Size float32
	// Mono 走等宽面：官方 `.optionName{font-family:var(--ds-font-family-code)}`。
	Mono bool
	// Color 是这一段的墨色；零值 = 用行自己的 `Foreground`。
	Color gfx.Color
	// Flex 表示这一段可压缩并省略：官方 `.optionName{min-width:0;overflow:hidden;
	// text-overflow:ellipsis}`。false = `flex:none`（`.optionNote` 不缩、不吃剩余宽度）。
	Flex bool
}

// 各段之间的间距：官方 `.option{display:inline-flex;align-items:baseline;gap:8px}`
// （`WorktreeToggle.module.css:99-106`）。
//
// **基线不在这里模拟**：官方是 `align-items:baseline`，本层把各段放进同一个行盒顶对齐。
// forge 基准菜单那两段同为 12px，同号就是同一条基线，所以这一档没有差；跨字号的行
// 按顶对齐画，读数补到之前不假装自己做了基线对齐。
const MenuRunGap float32 = 8

// 菜单分组标题（官方原语 `._label_70kcw_214`）：
//
//	._label_70kcw_214{padding:6px 8px;font-size:11px;line-height:15px;
//	                   color:var(--dsw-alias-label-tertiary)}
//
// 出处在 `index-CSAdOBx6.css`（class hash `70kcw`，文件头已给该 CSS 的 sha256）；
// `dense` 档只把 `padding-block` 改成 3（`._denseList_70kcw_209 ._label_70kcw_214`）。
const (
	MenuLabelPadX float32 = 8
	MenuLabelPadY float32 = 6
	MenuLabelFont float32 = 11
	MenuLabelLine float32 = 15
)

// PaintMenuLabel 画一行菜单分组标题（`MenuItemParts.GroupLabel` 为真的行）。
//
// 颜色用 `t.LabelTertiary`（官方 `--dsw-alias-label-tertiary`），**不用**菜单给的那一档：
// 原语 `MenuItemPaint.Foreground` 在这一档传的是 viva 主题的 `MutedForeground`，
// 而官方分组标题的令牌出自 primitives 自己的 CSS。与 `PaintMenuItem` 里分隔线用
// `t.BorderL2` 是同一条理由。
func PaintMenuLabel(parts ui.MenuItemParts, t *pluginapi.Theme) {
	canvas := parts.Canvas
	if canvas == nil || t == nil {
		return
	}
	row := parts.Rect
	lineY := row.Y + MenuLabelPadY
	width := max(row.Width-2*MenuLabelPadX, 0)
	label := parts.Label
	style := gfx.TextStyle{Size: MenuLabelFont, Weight: gfx.WeightRegular, Color: t.LabelTertiary}
	if canvas.TextWidthStyle(label, style) > width {
		label = fitRun(canvas, label, style, width)
	}
	canvas.Label(label, gfx.Rect{
		X: row.X + MenuLabelPadX, Y: lineY, Width: width, Height: MenuLabelLine,
	}, style)
}

// PaintMenuItemRuns 是 `PaintMenuItem` 的多段版：行里的文字由若干段组成。
//
// 面、悬停底、内边距、选中勾、分隔线与行自己的墨色都仍是这一处读数；调用方只说
// "这一行有哪几段、各自什么字号/字体/墨色、哪段可压缩"。
func PaintMenuItemRuns(parts ui.MenuItemParts, t *pluginapi.Theme, runs []MenuRun) {
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
	avail := max(row.Width-2*MenuRowPadX, 0)
	if chosen {
		// 官方 `.item{display:flex;align-items:center;gap:6px}`（byte 15091）：
		// 标签与勾之间隔 `MenuRowGap`，勾在行里**竖直居中**（不是与 20 的行盒顶对齐）。
		avail = max(avail-MenuCheckSize-MenuRowGap, 0)
	}
	x := row.X + MenuRowPadX
	gap := MenuRunGap * float32(max(len(runs)-1, 0))
	// `flex:none` 的段先量好，剩下的宽度才归可压缩的段（官方 `.optionName` 是唯一
	// 会缩的那一段：`.option{min-width:0}` 里它 `min-width:0`，`.optionNote{flex:none}`）。
	fixed := float32(0)
	flexCount := 0
	for _, run := range runs {
		if run.Flex {
			flexCount++
			continue
		}
		fixed += canvas.TextWidthStyle(run.Text, runStyle(run, parts.Foreground))
	}
	flexEach := float32(0)
	if flexCount > 0 {
		flexEach = max(avail-fixed-gap, 0) / float32(flexCount)
	}
	for _, run := range runs {
		style := runStyle(run, parts.Foreground)
		text := run.Text
		natural := canvas.TextWidthStyle(text, style)
		// 画的**盒子**：可压缩段占它分到的宽度（左对齐下盒子比字宽只影响省略号），
		// `flex:none` 的段占自己的自然宽。前进量用**字**的宽度，所以下一段紧贴上一段
		// 的字尾（官方 `.option{gap:8px}` 是字与字之间隔 8，不是盒子之间）。
		box := natural
		advance := natural
		if run.Flex {
			box = flexEach
			if natural <= flexEach {
				advance = natural
			} else {
				// 官方 `.optionName` 的 `text-overflow:ellipsis`：放不下才缩。
				text = fitRun(canvas, text, style, flexEach)
				advance = flexEach
			}
		}
		if text != "" {
			canvas.Label(text, gfx.Rect{X: x, Y: lineY, Width: box, Height: MenuRowLine}, style)
		}
		x += advance + MenuRunGap
	}
	if !chosen {
		return
	}
	// 勾是官方素材，不是手画的：`IconCheckOutlineRegular` 的 path 原文在 dsassets。
	// 渲染 14×14（`._check_70kcw_265`），viewBox 仍是 16。
	checkY := lineY + max((MenuRowLine-MenuCheckSize)/2, 0)
	if icon, ok := MenuCheckIcon(); ok {
		canvas.DrawIcon(icon, gfx.Rect{
			X: row.X + row.Width - MenuRowPadX - MenuCheckSize, Y: checkY,
			Width: MenuCheckSize, Height: MenuCheckSize,
		}, parts.Foreground)
	}
}

// MenuSurfaceExtra 是菜单面宽里**行内容之外**的那一圈。
//
// Viva 算菜单宽（`viva@v0.6.12-ow/components/dropdown_menu.go:693-721` 的
// `menuItemWidth`）是：行左内边距 8 + 行文字宽 + 行右内边距（单选/复选行 32，
// 普通行 8）+ 2×菜单内边距 4。它只量**声明的标签**，不知道调用方在行里多画了几段。
//
// 多段行（forge 基准菜单的「名字 + 当前分支」）要自己抬 `MenuItems.WidthRange` 的
// 下界，否则 Viva 会给一个只够画名字的宽度，名字被省略号截掉。这里给的是那份公式里
// 与内容无关的那一段（8 + 32 + 2×4 = 48，按单选/复选行算，它与官方菜单一样最多只有
// 一条选中行；多留的右列只会让行更宽松，不会截断内容）。
const MenuSurfaceExtra float32 = 48

// MenuRunsWidth 是这些段并排画出来需要的宽度（段之间隔 MenuRunGap）。
//
// 用来给 `MenuItems.WidthRange` 算下界，读数与 `PaintMenuItemRuns` 用的是同一份。
func MenuRunsWidth(canvas *gfx.Canvas, runs []MenuRun) float32 {
	if canvas == nil || len(runs) == 0 {
		return 0
	}
	width := MenuRunGap * float32(len(runs)-1)
	for _, run := range runs {
		width += canvas.TextWidthStyle(run.Text, runStyle(run, gfx.Color{}))
	}
	return width
}

// runStyle 把一段的文字样式拼出来：段自己的字号/字体/墨色，缺省用行的那一档
// （`fallback` 就是行的 `Foreground`；量宽度时不需要墨色，传零值即可）。
func runStyle(run MenuRun, fallback gfx.Color) gfx.TextStyle {
	size := run.Size
	if size <= 0 {
		size = MenuRowFont
	}
	color := run.Color
	if color == (gfx.Color{}) {
		color = fallback
	}
	return gfx.TextStyle{Size: size, Weight: gfx.WeightRegular, Color: color, Mono: run.Mono}
}

// fitRun 是 `gfx.FitLine` 的**样式版**：等宽面用 `TextWidthStyle` 量，二分找最长的
// 能放下的前缀再加省略号。非等宽面直接走 `gfx.FitLine`，与既有行画笔逐像素一致。
func fitRun(canvas *gfx.Canvas, text string, style gfx.TextStyle, width float32) string {
	if width <= 0 {
		return ""
	}
	if !style.Mono {
		return canvas.FitLine(text, style.Size, style.Weight, width)
	}
	if canvas.TextWidthStyle(text, style) <= width {
		return text
	}
	room := width - canvas.TextWidthStyle("…", style)
	if room <= 0 {
		return "…"
	}
	runes := []rune(text)
	fits := 0
	for low, high := 0, len(runes); low < high; {
		middle := (low + high + 1) / 2
		if canvas.TextWidthStyle(string(runes[:middle]), style) <= room {
			low = middle
		} else {
			high = middle - 1
		}
		fits = low
	}
	return string(runes[:fits]) + "…"
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
