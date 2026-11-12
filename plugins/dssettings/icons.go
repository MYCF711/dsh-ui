// icons.go —— 设置面用到的官方 sprite 符号（rc.2 `dsh-client-ui-primitives` 的 sprite 表）。
//
// # 官方出处（逐字）
//
// `ui-settings-general/src/client/SettingsRoot.tsx` 的 `triggerRow` 里，设置触发器
// 的图标是 `IconSettingsOutlineMedium`（宽态 16 / 收起轨 18，
// `ui-settings-general/src/client/chrome.tsx:22-29`）；本文件转写的是**regular**
// 那一枚（`IconSettingsOutlineRegular`，dist sprite 表里的同名 symbol）：
//
//	name                     viewBox          weight  dist 位置（dsh-web-frontend/dist/assets/index-BXV-oxvX.js）
//	settings-outline         0 0 16 16        1       sprite 表 `IconSettingsOutlineRegular`
//
// 素材库同物：`$DSH_HOME/dsh-ui-official/assets/icons/settings/settings.jsx`
// （sha256 3558d09746b62933c13eb1933fdc062e1cf327b4591e10cf4c40d668ef5a88cf，
// `LIBRARY-MANIFEST.json` 的 entries[30]：viewBox 0 0 16 16、默认 16px、
// symbol `IconSettingsOutlineRegular`、target `plugins/dssettings/settings.go`）。
// 本文件的 path 命令由该 JSX 的 `d` 属性**逐点转写**（M/L/C/Z 四类，无近似）。
//
// 两条边界：
//
//	① 只转写**用得到的那一枚**；没被调用的 symbol 不进仓（避免"登记了但没人画"）。
//	② 显示尺寸由调用方给官方的 16（宽态 `.trigger` 的图标 16；收起轨 18 走
//	   另一个组件分支，本仓还没有收起轨）。
package dssettings

import "github.com/vivaui/viva/ui"

// settingsTriggerIcon 是官方 `IconSettingsOutlineRegular`（16 viewBox、stroke 1）。
func settingsTriggerIcon() ui.Icon {
	return ui.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []ui.PathCommand{
			// 第 1 段（官方 path #1）
			ui.MoveTo(8, 9.75012),
			ui.CubicTo(8.9665, 9.75012, 9.75, 8.96662, 9.75, 8.00012),
			ui.CubicTo(9.75, 7.03362, 8.9665, 6.25012, 8, 6.25012),
			ui.CubicTo(7.0335, 6.25012, 6.25, 7.03362, 6.25, 8.00012),
			ui.CubicTo(6.25, 8.96662, 7.0335, 9.75012, 8, 9.75012),
			ui.ClosePath(),
			// 第 2 段（官方 path #2）
			ui.MoveTo(13.0107, 7.79377),
			ui.CubicTo(12.9505, 7.89401, 12.9205, 7.94413, 12.9205, 7.99951),
			ui.CubicTo(12.9205, 8.0549, 12.9505, 8.10502, 13.0106, 8.20528),
			ui.LineTo(13.9849, 9.83006),
			ui.CubicTo(14.045, 9.93029, 14.0751, 9.9804, 14.0751, 10.0358),
			ui.CubicTo(14.0751, 10.0911, 14.045, 10.1413, 13.9849, 10.2415),
			ui.LineTo(13.0037, 11.8777),
			ui.CubicTo(12.9468, 11.9726, 12.9184, 12.0201, 12.8725, 12.0461),
			ui.CubicTo(12.8267, 12.072, 12.7713, 12.072, 12.6607, 12.072),
			ui.LineTo(10.6704, 12.072),
			ui.CubicTo(10.5598, 12.072, 10.5045, 12.072, 10.4586, 12.098),
			ui.CubicTo(10.4128, 12.1239, 10.3843, 12.1714, 10.3274, 12.2662),
			ui.LineTo(9.33825, 13.9142),
			ui.CubicTo(9.28133, 14.009, 9.25287, 14.0564, 9.20703, 14.0823),
			ui.CubicTo(9.16118, 14.1083, 9.10588, 14.1083, 8.99529, 14.1083),
			ui.LineTo(7.00486, 14.1083),
			ui.CubicTo(6.89426, 14.1083, 6.83896, 14.1083, 6.79312, 14.0823),
			ui.CubicTo(6.74727, 14.0564, 6.71881, 14.009, 6.6619, 13.9142),
			ui.LineTo(5.67273, 12.2662),
			ui.CubicTo(5.61581, 12.1714, 5.58735, 12.1239, 5.54151, 12.098),
			ui.CubicTo(5.49566, 12.072, 5.44036, 12.072, 5.32977, 12.072),
			ui.LineTo(3.33945, 12.072),
			ui.CubicTo(3.2288, 12.072, 3.17347, 12.072, 3.12761, 12.0461),
			ui.CubicTo(3.08176, 12.0201, 3.0533, 11.9726, 2.9964, 11.8777),
			ui.LineTo(2.0152, 10.2415),
			ui.CubicTo(1.9551, 10.1413, 1.92505, 10.0911, 1.92505, 10.0358),
			ui.CubicTo(1.92505, 9.9804, 1.9551, 9.93029, 2.0152, 9.83006),
			ui.LineTo(2.98951, 8.20528),
			ui.CubicTo(3.04963, 8.10502, 3.07969, 8.0549, 3.07969, 7.99951),
			ui.CubicTo(3.07968, 7.94413, 3.04961, 7.89401, 2.98946, 7.79377),
			ui.LineTo(2.01529, 6.17011),
			ui.CubicTo(1.95514, 6.06987, 1.92507, 6.01975, 1.92507, 5.96437),
			ui.CubicTo(1.92506, 5.90899, 1.95512, 5.85886, 2.01524, 5.7586),
			ui.LineTo(2.9964, 4.1224),
			ui.CubicTo(3.0533, 4.0275, 3.08176, 3.98005, 3.12761, 3.95408),
			ui.CubicTo(3.17347, 3.92811, 3.2288, 3.92811, 3.33945, 3.92811),
			ui.LineTo(5.32977, 3.92811),
			ui.CubicTo(5.44036, 3.92811, 5.49566, 3.92811, 5.54151, 3.90216),
			ui.CubicTo(5.58735, 3.87621, 5.61581, 3.82879, 5.67273, 3.73397),
			ui.LineTo(6.6619, 2.08599),
			ui.CubicTo(6.71881, 1.99116, 6.74727, 1.94375, 6.79312, 1.9178),
			ui.CubicTo(6.83896, 1.89185, 6.89426, 1.89185, 7.00486, 1.89185),
			ui.LineTo(8.99529, 1.89185),
			ui.CubicTo(9.10588, 1.89185, 9.16118, 1.89185, 9.20703, 1.9178),
			ui.CubicTo(9.25287, 1.94375, 9.28133, 1.99116, 9.33825, 2.08599),
			ui.LineTo(10.3274, 3.73397),
			ui.CubicTo(10.3843, 3.82879, 10.4128, 3.87621, 10.4586, 3.90216),
			ui.CubicTo(10.5045, 3.92811, 10.5598, 3.92811, 10.6704, 3.92811),
			ui.LineTo(12.6607, 3.92811),
			ui.CubicTo(12.7713, 3.92811, 12.8267, 3.92811, 12.8725, 3.95408),
			ui.CubicTo(12.9184, 3.98005, 12.9468, 4.0275, 13.0037, 4.1224),
			ui.LineTo(13.9849, 5.7586),
			ui.CubicTo(14.045, 5.85886, 14.0751, 5.90899, 14.0751, 5.96437),
			ui.CubicTo(14.0751, 6.01975, 14.045, 6.06987, 13.9849, 6.17011),
			ui.LineTo(13.0107, 7.79377),
			ui.ClosePath(),
		},
	}
}
