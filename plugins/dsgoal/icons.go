// icons.go —— 目标条那枚 glyph：官方 sprite 里的 `goal-outline`。
//
// # 官方出处（逐字，rc.2）
//
// `dsh-client-ui-goal/src/client/GoalBar.tsx:137`：
//
//	<span className={css.goalGlyph}><IconGoalOutlineRegular size={14} /></span>
//
// `GoalBar.module.css`：
//
//	.Gc24Bq_goalGlyph{color:var(--dsw-alias-label-tertiary);flex:none;display:inline-flex}
//
// `IconGoalOutlineRegular` 在 rc.2 是 sprite 表里的一条（`M(name,weight)` + `<use>`）：
//
//	name          goal-outline
//	weight        regular = 1（`p7={regular:1,…}`）→ `stroke-width="1"`
//	viewBox       0 0 16 16（**不是 14**；14 是显示尺寸 `size={14}`）
//	sprite 偏移   `dsh-web-frontend/dist/assets/index-ghOpC9uV.js` 第 129 行第 53572 列
//	              （文件 1,093,231 B，sha256 `061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8`）
//	sprite sha256  `svg(1)` 原串 `0fe6266990c7ce022cc1245145ffab032cba1db98736b743222f33d945f63485`
//	4 段 path    与素材库 `assets/icons/tools/goal.jsx`
//	              （sha256 `aca46bf4068391abddb67c70b4af3c84c12a46144e587de4361ca22f14584ea0`）**逐字节相同**
//
// 所以这枚图标的正确写法就是「同一个符号、同一个 viewBox、按官方 size 缩放」——
// 不是自造的近似图形。原文件的「官方无此 glyph，用 4px 圆点顶」的判断是错的：
// 官方 GoalBar 用的就是 `IconGoalOutlineRegular`（见上引 GoalBar.tsx:137）。
//
// # 为什么 viewBox 是 16 而显示 14
//
// 官方 `<use>` 的 svg 是 `width={size} height={size}` + `viewBox="0 0 16 16"`，
// 也就是「16 个单点的图形缩到 14px 画」。这里保持同一组坐标，由调用方
// `f.Icon(…).Size(goalGlyphSize, goalGlyphSize)` 给 14——**不要**把 path 里的
// 数字整体缩放成 14，那会让描边宽度（view 单位 1）跟着错。
package dsgoal

import "github.com/vivaui/viva/ui"

// goalGlyphSize 是官方 `IconGoalOutlineRegular size={14}`（GoalBar.tsx:137）。
const goalGlyphSize float32 = 14

// goalOutlineIcon 是官方 sprite `goal-outline`（regular，4 段描边）。
//
// Stroke=1 与 Cap/Join 的零值（butt/miter）就是官方 svg 的默认：
// sprite 的 `<svg>` 只写了 `fill="none" stroke-width="${e}"`，
// 没有 linecap/linejoin，所以**不要**在这里改成 Round（那是另一枚图标的样子）。
func goalOutlineIcon() ui.Icon {
	return ui.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []ui.PathCommand{
			// 第 1 段（官方 path #1）：外圈缺口圆环
			ui.MoveTo(14.5001, 8),
			ui.CubicTo(14.5, 9.28552, 14.1188, 10.5422, 13.4045, 11.611),
			ui.CubicTo(12.6903, 12.6799, 11.6752, 13.5129, 10.4875, 14.0049),
			ui.CubicTo(9.29982, 14.4968, 7.99295, 14.6255, 6.73212, 14.3747),
			ui.CubicTo(5.4713, 14.124, 4.31314, 13.505, 3.4041, 12.596),
			ui.CubicTo(2.49514, 11.687, 1.87614, 10.5288, 1.62537, 9.26798),
			ui.CubicTo(1.37459, 8.00716, 1.50331, 6.70028, 1.99525, 5.51261),
			ui.CubicTo(2.48719, 4.32494, 3.32025, 3.30981, 4.3891, 2.59557),
			ui.CubicTo(5.45795, 1.88134, 6.71458, 1.50008, 8.0001, 1.5),
			// 第 2 段（官方 path #2）：内圈缺口圆环
			ui.MoveTo(11.5, 8),
			ui.CubicTo(11.5001, 8.69227, 11.2948, 9.36901, 10.9102, 9.94463),
			ui.CubicTo(10.5257, 10.5202, 9.97901, 10.9689, 9.33944, 11.2338),
			ui.CubicTo(8.69986, 11.4987, 7.99609, 11.5681, 7.31712, 11.433),
			ui.CubicTo(6.63816, 11.2979, 6.01449, 10.9645, 5.52501, 10.475),
			ui.CubicTo(5.03548, 9.98552, 4.70209, 9.36185, 4.56702, 8.68289),
			ui.CubicTo(4.43195, 8.00392, 4.50127, 7.30015, 4.76619, 6.66057),
			ui.CubicTo(5.03112, 6.021, 5.47976, 5.47436, 6.05538, 5.08978),
			ui.CubicTo(6.631, 4.70519, 7.30774, 4.49995, 8.00001, 4.5),
			// 第 3 段（官方 path #3）：中心到右上的指针
			ui.MoveTo(8.00024, 7.99976),
			ui.LineTo(11.2, 4.80005),
			// 第 4 段（官方 path #4）：箭头头部（闭合多边形）
			ui.MoveTo(12.4719, 5.62245),
			ui.CubicTo(12.4246, 5.66972, 12.3569, 5.69025, 12.2913, 5.67715),
			ui.LineTo(10.7814, 5.37555),
			ui.CubicTo(10.7022, 5.35972, 10.6402, 5.29781, 10.6244, 5.2186),
			ui.LineTo(10.3228, 3.70866),
			ui.CubicTo(10.3097, 3.6431, 10.3302, 3.57533, 10.3775, 3.52806),
			ui.LineTo(12.1826, 1.723),
			ui.CubicTo(12.2863, 1.61929, 12.4627, 1.65879, 12.5122, 1.79684),
			ui.LineTo(12.9271, 2.95225),
			ui.CubicTo(12.9472, 3.00847, 12.9915, 3.05272, 13.0477, 3.07291),
			ui.LineTo(14.2031, 3.48774),
			ui.CubicTo(14.3412, 3.5373, 14.3807, 3.71368, 14.277, 3.81739),
			ui.LineTo(12.4719, 5.62245),
			ui.ClosePath(),
		},
	}
}
