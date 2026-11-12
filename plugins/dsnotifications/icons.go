// icons.go —— 本插件用到的 4 个官方素材，**逐字节取自官方**，不自画。
//
//   - 通知铃（14×14，内联 SVG）：`ui-account-notifications/src/client/AccountNotifications.tsx:148-151`
//     落库 `$DSH_HOME/dsh-ui-official/assets/icons/account-notifications/notice-bell.jsx`
//     （原始块 sha256 9a070d99…）
//   - 关闭（16×16 sprite `close-outline` regular）：`dsh-client-ui-primitives`
//     `IconCloseOutlineRegular size={10}`；path 数据与 rc.1 相同（素材主库 §7）
//   - 勾选清单（16×16 sprite `checklist-outline`）：`ui-agent-team` TeamBoard 的 lead 图标（size 14）
//   - 信息（**14×14** sprite `info-outline` regular）：`ui-agent-team` TeamModeToggle 的 info 按钮
//     （size 11）。**viewBox 是 14 不是 16**，套 16 会让它在 11px 下缩一号（H1 报告点名提醒）
//   - 下折箭头（16×16 sprite `chevron-down-outline`）：TeamBoard 的 chevron
//
// viewBox 一律用官方原值；**显示尺寸**由调用方按官方 `size` 传（14 / 10 / 11），
// 两者不能混（这正是 H1 §3.3 说的"易套错"的地方）。
package dsnotifications

import "github.com/vivaui/viva/ui"

// bellViewBox 是官方内联铃图标的 viewBox（14，不是 16）。
const bellViewBox = 14

// bellBodyPath 是官方第一段 path（铃身，fill + stroke 双画）。
func bellBodyPath() []ui.PathCommand {
	return []ui.PathCommand{
		ui.MoveTo(3.6355, 6.93969),
		ui.CubicTo(3.6355, 4.8398, 5.33779, 3.1375, 7.43768, 3.1375),
		ui.CubicTo(9.53757, 3.1375, 11.23987, 4.8398, 11.23987, 6.93968),
		ui.LineTo(11.23987, 11.363),
		ui.LineTo(3.6355, 11.363),
		ui.LineTo(3.6355, 6.93969),
		ui.ClosePath(),
	}
}

// bellLinePath 是官方第二段 path（底边 + 铃舌 + 顶部竖线，只有描边）。
func bellLinePath() []ui.PathCommand {
	return []ui.PathCommand{
		ui.MoveTo(1.641, 11.3645), ui.LineTo(12.36, 11.3645),
		ui.MoveTo(5.053, 13.3095), ui.LineTo(8.9479, 13.3095),
		ui.MoveTo(7.4375, 4.27422), ui.LineTo(7.4375, 2.70137912),
	}
}

// noticeBellIcon 画官方铃图标：铃身用 currentColor 的 0.2 填充 + 0.875 描边，
// 横/舌/顶线只描边。官方是 `fillOpacity="0.2"`、`strokeWidth="0.875"`。
//
// 为什么按 theme 色造：viva 的 `IconLayer.Fill` 是**自带颜色**（tint 只贡献 alpha），
// 所以"currentColor 的 20%"只能在造图标时把当帧品牌色填进去。
func noticeBellIcon(brand ui.Color) ui.Icon {
	fill := brand
	fill.Alpha = 0.2
	return ui.Icon{
		ViewBox:  bellViewBox,
		Stroke:   0.875,
		Cap:      ui.CapRound,
		Join:     ui.JoinRound,
		Commands: append(bellBodyPath(), bellLinePath()...),
		Layers: []ui.IconLayer{{
			Fill:     fill,
			Commands: bellBodyPath(),
		}},
	}
}

// closeIcon 是官方 `IconCloseOutlineRegular`（16×16 sprite，default size 10）。
func closeIcon() ui.Icon {
	return ui.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     ui.CapRound,
		Join:    ui.JoinRound,
		Commands: []ui.PathCommand{
			ui.MoveTo(2.5, 2.5), ui.LineTo(13.5, 13.5),
			ui.MoveTo(13.5, 2.5), ui.LineTo(2.5, 13.5),
		},
	}
}

// checklistIcon 是官方 `IconChecklistOutlineRegular`（16×16 sprite，TeamBoard size 14）。
func checklistIcon() ui.Icon {
	return ui.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     ui.CapRound,
		Join:    ui.JoinRound,
		Commands: []ui.PathCommand{
			// 上环
			ui.MoveTo(3.75, 6.25),
			ui.CubicTo(4.7165, 6.25, 5.5, 5.4665, 5.5, 4.5),
			ui.CubicTo(5.5, 3.5335, 4.7165, 2.75, 3.75, 2.75),
			ui.CubicTo(2.7835, 2.75, 2, 3.5335, 2, 4.5),
			ui.CubicTo(2, 5.4665, 2.7835, 6.25, 3.75, 6.25),
			ui.ClosePath(),
			ui.MoveTo(7.5, 4.5), ui.LineTo(13.5, 4.5),
			// 下环
			ui.MoveTo(3.75, 13.25),
			ui.CubicTo(4.7165, 13.25, 5.5, 12.4665, 5.5, 11.5),
			ui.CubicTo(5.5, 10.5335, 4.7165, 9.75, 3.75, 9.75),
			ui.CubicTo(2.7835, 9.75, 2, 10.5335, 2, 11.5),
			ui.CubicTo(2, 12.4665, 2.7835, 13.25, 3.75, 13.25),
			ui.ClosePath(),
			ui.MoveTo(7.5, 11.5), ui.LineTo(13.5, 11.5),
		},
	}
}

// infoIcon 是官方 `IconInfoOutlineRegular`（见文件头：viewBox=14）。
func infoIcon() ui.Icon {
	return ui.Icon{
		// Stroke=0 = 用填充模式画（官方这三段 path 都是 fill=currentColor）。
		ViewBox: 14,
		Commands: []ui.PathCommand{
			ui.MoveTo(12.5757, 7.00012),
			ui.CubicTo(12.5757, 3.92085, 10.0794, 1.42463, 7.00012, 1.42456),
			ui.CubicTo(3.9208, 1.42456, 1.42456, 3.9208, 1.42456, 7.00012),
			ui.CubicTo(1.42463, 10.0794, 3.92085, 12.5757, 7.00012, 12.5757),
			ui.CubicTo(10.0793, 12.5756, 12.5756, 10.0793, 12.5757, 7.00012),
			ui.ClosePath(),
			ui.MoveTo(13.8002, 7.00012),
			ui.CubicTo(13.8001, 10.7559, 10.7559, 13.8001, 7.00012, 13.8002),
			ui.CubicTo(3.2443, 13.8002, 0.199291, 10.7559, 0.199219, 7.00012),
			ui.CubicTo(0.199219, 3.24426, 3.24426, 0.199219, 7.00012, 0.199219),
			ui.CubicTo(10.7559, 0.199291, 13.8002, 3.2443, 13.8002, 7.00012),
			ui.ClosePath(),
			// i 的点
			ui.MoveTo(7.6127, 3.18921), ui.LineTo(7.6127, 4.55986),
			ui.LineTo(6.38735, 4.55986), ui.LineTo(6.38735, 3.18921),
			ui.ClosePath(),
			// i 的竖
			ui.MoveTo(7.6127, 5.68921), ui.LineTo(7.6127, 10.8109),
			ui.LineTo(6.38735, 10.8109), ui.LineTo(6.38735, 5.68921),
			ui.ClosePath(),
		},
	}
}

// chevronDownIcon 是官方 `IconChevronDownOutlineRegular`（16×16 sprite）。
func chevronDownIcon() ui.Icon {
	return ui.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     ui.CapRound,
		Join:    ui.JoinRound,
		Commands: []ui.PathCommand{
			ui.MoveTo(4, 6),
			ui.LineTo(7.29289, 9.29289),
			ui.CubicTo(7.68342, 9.68342, 8.31658, 9.68342, 8.70711, 9.29289),
			ui.LineTo(12, 6),
		},
	}
}
