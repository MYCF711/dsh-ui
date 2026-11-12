// icons.go —— 连接指示用到的官方 sprite 符号（rc.2 `dsh-client-ui-primitives` 的 sprite 表）。
//
// # 官方出处（逐字）
//
// `ui-settings-general/src/client/SettingsRoot.tsx` 的 `triggerRow` 里渲染
// `ConnectionIndicator`（primitives）。它的两个状态图标是：
//
//	recovered    -> `Cs` = `M("check-outline","regular")`   -> `IconCheckOutlineRegular`
//	disconnected -> `Jw` = `M("refresh-outline","regular")` -> `IconRefreshOutlineRegular`
//
// 两者在 rc.2 都是 sprite 表的一条（`M(name,weight)` + `<use href="#dsw-icon-...">`），
// 生成器为 `svg({width:size,height:size,"data-icon":"dsw-icon-<name>-<weight>"})`：
//
//	name             viewBox       weight regular    dist 位置（dsh-web-frontend/dist/assets/index-ghOpC9uV.js）
//	check-outline    0 0 16 16     1（stroke-width） 第 129 行 第 10644 列
//	refresh-outline  0 0 16 16     1（stroke-width） 第 129 行 第 14896 列
//
// dist 文件 1,093,231 B，sha256 061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8；
// sprite svg(1) 原串 sha256：
//
//	check-outline   41e32b03bef4130a030a4f4827a13e2e474e9c0fe8f9fe42dc7ca9ba437fc291
//	refresh-outline 06a5745f7fe0ceddf5ecfd9dfbd80f78e3927e395eacd9d9a20f0b66fb71eadb
//
// `check-outline` 素材库内有同物：assets/icons/composer/check.jsx
// （sha256 3b47a8c21d5bf626867f2d6b45a6ff8b2f7362123cfa76de5a395a2b2916bb8c），
// 其 d 与 rc.2 sprite 逐字节相同；`refresh-outline` 库内暂无，本文件按 sprite 原文转写。
//
// viewBox 一律官方 16；显示尺寸由调用方给官方的 14（._icon_16cyo_71{width:14px;height:14px}）。
package dsstatusbar

import "github.com/vivaui/viva/gfx"

// connectionIconSize 是官方 ._icon_16cyo_71{width:14px;height:14px}。
const connectionIconSize float32 = 14

// checkOutlineIcon 是官方 sprite `check-outline`（成功态：已连接 / 已恢复）。
func checkOutlineIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			// 第 1 段（官方 path #1）
			gfx.MoveTo(2.25, 8.5),
			gfx.LineTo(5.49732, 11.7473),
			gfx.CubicTo(5.90519, 12.1552, 6.57263, 12.1344, 6.95426, 11.7018),
			gfx.LineTo(13.75, 4),
		},
	}
}

// refreshOutlineIcon 是官方 sprite `refresh-outline`（异常态：断开 / 正在重试）。
func refreshOutlineIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			// 第 1 段（官方 path #1）
			gfx.MoveTo(14.5001, 8),
			gfx.CubicTo(14.5, 9.28552, 14.1188, 10.5422, 13.4045, 11.611),
			gfx.CubicTo(12.6903, 12.6799, 11.6752, 13.5129, 10.4875, 14.0049),
			gfx.CubicTo(9.29982, 14.4968, 7.99295, 14.6255, 6.73212, 14.3747),
			gfx.CubicTo(5.4713, 14.124, 4.31314, 13.505, 3.4041, 12.596),
			gfx.CubicTo(2.49514, 11.687, 1.87614, 10.5288, 1.62537, 9.26798),
			gfx.CubicTo(1.37459, 8.00716, 1.50331, 6.70028, 1.99525, 5.51261),
			gfx.CubicTo(2.48719, 4.32494, 3.32025, 3.30981, 4.3891, 2.59557),
			gfx.CubicTo(5.45795, 1.88134, 6.71458, 1.50008, 8.0001, 1.5),
			gfx.CubicTo(9.9001, 1.5, 11.7001, 2.3, 13.0001, 3.6),
			gfx.LineTo(14.5001, 5.1),
			// 第 2 段（官方 path #2）
			gfx.MoveTo(14.4999, 1.5),
			gfx.LineTo(14.4999, 5.1),
			gfx.LineTo(10.8999, 5.1),
		},
	}
}

// loadingArcIcon 是官方 `connecting` 态的加载指示（primitives 的 `Spinner`,
// `state="ongoing"`）。
//
// # 官方出处（逐字）
//
// `dsh-web-frontend/dist/assets/index-BXV-oxvX.js` 的 spinner 组件：
//
//	<svg width={14} height={14} viewBox="0 0 24 24" aria-hidden="true">
//	  <g class="spinnerMotion">
//	    <circle class="spinnerTrack" cx="12" cy="12" r="9.5"/>
//	    <circle class="spinnerArc"   cx="12" cy="12" r="9.5"/>
//	  </g>
//	</svg>
//
// CSS（`index-BNyAYEM7.css` 的 `_spinnerTrack_1i3xo_47` / `_spinnerArc_1i3xo_48`）：
//
//	.spinnerTrack,.spinnerArc{fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round}
//	.spinnerTrack{opacity:.25}
//	.spinnerArc{stroke-dasharray:12 150}          ← 周长 2π·9.5 ≈ 59.69，取 12（≈72°）
//	@media (prefers-reduced-motion:reduce){ .spinnerMotion,.spinnerArc{animation:none} }
//	.spinnerArc（减动效档）{stroke-dasharray:18 150;stroke-dashoffset:-3}
//
// # 本仓的转写（derived，登记在报告里）
//
// `ui.Icon` 只有 path 命令、没有 `<circle>`/逐 path opacity，所以：
//
//   - **只画 arc 那一段**（72° 圆弧，从正上方 -90° 顺时针），不画 25% 的 track——
//     画成 100% 不透明会变成"一个实心圆环"，比缺一段更错；
//   - 不转（官方 `spinnerMotion` 是 1.5s 旋转动画；无头截图拍不到动画，
//     真机每帧重绘只会让整窗脏区变大）；
//   - 减动效档的 `stroke-dasharray:18` 也不做——本仓没有 media query，
//     固定取常规档的 12。
//
// 72° 圆弧的三次贝塞尔近似：k = 4/3·tan(θ/4) = 4/3·tan18° = 0.43321，
// 半径 9.5 → 控制点偏移 4.1155。
func loadingArcIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 24,
		Stroke:  2,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(12, 2.5),
			gfx.CubicTo(16.1155, 2.5, 19.7645, 5.1517, 21.0371, 9.0641),
		},
	}
}
