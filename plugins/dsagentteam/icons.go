// icons.go —— 本包用到的官方 sprite 符号（rc.2 `dsh-client-ui-primitives` 的 sprite 表）。
//
//   - users-outline（16×16，`IconUsersOutlineRegular size=14`）：TeamCommandRow 的图标
//   - info-outline（**14×14**，`IconInfoOutlineRegular size=11`）：TeamModeToggle 的 info 按钮
//   - checklist-outline（16×16，`IconChecklistOutlineRegular size=14`）：TeamBoard 的 lead
//   - chevron-down-outline（16×16，`IconChevronDownOutlineRegular size=14`）：任务行折叠箭头
//
// 素材来源与 sha256 见 `$DSH_HOME/dsh-ui-official/assets/icons/agent-team/*.jsx` 与
// `structure/official-assets-reuse.md §7`。**viewBox 一律官方原值**，
// 显示尺寸由调用方按官方 `size` 传（11 / 14）——`info-outline` 是 14 不是 16，
// 套 16 会让它在 11px 下缩一号（H1 报告 §3.3 点名过的坑）。
package dsagentteam

import "github.com/vivaui/viva/ui"

// usersIcon 是官方 `IconUsersOutlineRegular`（4 段描边）。
func usersIcon() ui.Icon {
	return ui.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     ui.CapRound,
		Join:    ui.JoinRound,
		Commands: []ui.PathCommand{
			ui.MoveTo(6.18, 8.2275),
			ui.CubicTo(7.5620898, 8.2275, 8.6825, 7.1070898, 8.6825, 5.725),
			ui.CubicTo(8.6825, 4.3429102, 7.5620898, 3.2225, 6.18, 3.2225),
			ui.CubicTo(4.7979102, 3.2225, 3.6775, 4.3429102, 3.6775, 5.725),
			ui.CubicTo(3.6775, 7.1070898, 4.7979102, 8.2275, 6.18, 8.2275),
			ui.ClosePath(),
			ui.MoveTo(1.63, 13.915),
			ui.CubicTo(1.63, 11.185, 3.905, 10.0475, 6.18, 10.0475),
			ui.CubicTo(8.455, 10.0475, 10.73, 11.185, 10.73, 13.915),
			ui.MoveTo(10.275, 3.359),
			ui.CubicTo(11.3215, 3.7685, 12.0495, 4.6785, 12.0495, 5.725),
			ui.CubicTo(12.0495, 6.7715, 11.3215, 7.6815, 10.275, 8.091),
			ui.MoveTo(12.004, 10.366),
			ui.CubicTo(13.369, 11.003, 14.37, 12.186, 14.37, 13.915),
		},
	}
}

// infoIcon 是官方 `IconInfoOutlineRegular`（viewBox=14，三段填充）。
func infoIcon() ui.Icon {
	return ui.Icon{
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
			ui.MoveTo(7.6127, 3.18921), ui.LineTo(7.6127, 4.55986),
			ui.LineTo(6.38735, 4.55986), ui.LineTo(6.38735, 3.18921),
			ui.ClosePath(),
			ui.MoveTo(7.6127, 5.68921), ui.LineTo(7.6127, 10.8109),
			ui.LineTo(6.38735, 10.8109), ui.LineTo(6.38735, 5.68921),
			ui.ClosePath(),
		},
	}
}

// checklistIcon 是官方 `IconChecklistOutlineRegular`（两环 + 两条线）。
func checklistIcon() ui.Icon {
	return ui.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     ui.CapRound,
		Join:    ui.JoinRound,
		Commands: []ui.PathCommand{
			ui.MoveTo(3.75, 6.25),
			ui.CubicTo(4.7165, 6.25, 5.5, 5.4665, 5.5, 4.5),
			ui.CubicTo(5.5, 3.5335, 4.7165, 2.75, 3.75, 2.75),
			ui.CubicTo(2.7835, 2.75, 2, 3.5335, 2, 4.5),
			ui.CubicTo(2, 5.4665, 2.7835, 6.25, 3.75, 6.25),
			ui.ClosePath(),
			ui.MoveTo(7.5, 4.5), ui.LineTo(13.5, 4.5),
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

// chevronDownIcon 是官方 `IconChevronDownOutlineRegular`。
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
