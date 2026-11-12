package dscomposer

import "github.com/vivaui/viva/ui"

// 本文件的矢量全部使用 16×16 viewBox。发送/停止路径逐点来自官方 InputBar；
// 四种 preset 图标来自官方 primitives 的 PermissionIcon*Regular。
// 宿主额外暴露的 ask/deny 是审批 policy、不是官方 preset，官方没有同名图标；
// 这两枚只复用官方盾牌轮廓，分别用问号和斜线表达真值，不冒称 preset 图标。

var primarySendIcon = ui.Icon{
	ViewBox: 16,
	Commands: []ui.PathCommand{
		ui.MoveTo(8.3125, 0.980183),
		ui.CubicTo(8.66767, 1.0531, 8.97902, 1.20418, 9.2627, 1.43233),
		ui.CubicTo(9.48724, 1.61297, 9.73029, 1.85793, 9.97949, 2.10714),
		ui.LineTo(14.707, 6.83468),
		ui.LineTo(13.293, 8.24874),
		ui.LineTo(9, 3.95577),
		ui.LineTo(9, 15.0417),
		ui.LineTo(7, 15.0417),
		ui.LineTo(7, 3.95577),
		ui.LineTo(2.70703, 8.24874),
		ui.LineTo(1.29297, 6.83468),
		ui.LineTo(6.02051, 2.10714),
		ui.CubicTo(6.26971, 1.85793, 6.51277, 1.61297, 6.7373, 1.43233),
		ui.CubicTo(6.97662, 1.23986, 7.28445, 1.04402, 7.6875, 0.980183),
		ui.CubicTo(7.8973, 0.947006, 8.1031, 0.95516, 8.3125, 0.980183),
		ui.ClosePath(),
	},
}

// addPlusIcon 是控件行左端「加号」入口的图标（官方 `IconPlusOutlineMedium
// size={14}`，`InputBar.tsx:517`）。
//
// **derived**：rc.2 随包的 `dsh-client-ui-primitives` 没有 `src/` 可锚
// （GT 报告 §8 的未取证第 1 条），所以这里不是逐点抄来的路径，而是按
// 16 viewBox 画的等宽十字（官方图标名明确是 "Outline Medium"，即描边式）。
// 几何（14×14 显示、1px 描边、圆头）来自上面那行 size 与 `.pill svg` 的
// 同类规则；**不得**把它当成官方逐字节素材（official-assets-reuse 里记为 derived）。
var addPlusIcon = ui.Icon{
	ViewBox: 16, Stroke: 1, Join: ui.JoinRound,
	Commands: []ui.PathCommand{
		ui.MoveTo(8, 3.5), ui.LineTo(8, 12.5),
		ui.MoveTo(3.5, 8), ui.LineTo(12.5, 8),
	},
}

var primaryStopIcon = ui.Icon{
	ViewBox: 16,
	Commands: []ui.PathCommand{
		// 官方 <rect x=3 y=3 width=10 height=10 rx=3> 的等价闭合路径。
		ui.MoveTo(6, 3), ui.LineTo(10, 3),
		ui.QuadTo(13, 3, 13, 6), ui.LineTo(13, 10),
		ui.QuadTo(13, 13, 10, 13), ui.LineTo(6, 13),
		ui.QuadTo(3, 13, 3, 10), ui.LineTo(3, 6),
		ui.QuadTo(3, 3, 6, 3), ui.ClosePath(),
	},
}

func permissionShield() []ui.PathCommand {
	return []ui.PathCommand{
		ui.MoveTo(6.59624, 2.14853),
		ui.CubicTo(7.50155, 1.80917, 8.49914, 1.80919, 9.40444, 2.14859),
		ui.LineTo(13.9245, 3.84317), ui.LineTo(13.9245, 7.11961),
		ui.CubicTo(13.9245, 11.6089, 10.5565, 13.5975, 8.00035, 14.5779),
		ui.CubicTo(5.44423, 13.5975, 2.07544, 11.6089, 2.07544, 7.11961),
		ui.LineTo(2.07544, 3.84317), ui.LineTo(6.59624, 2.14853), ui.ClosePath(),
	}
}

func permissionIconWith(extra ...ui.PathCommand) ui.Icon {
	commands := permissionShield()
	commands = append(commands, extra...)
	return ui.Icon{ViewBox: 16, Stroke: 1, Join: ui.JoinRound, Commands: commands}
}

func permissionReadOnlyIcon() ui.Icon {
	return permissionIconWith(
		ui.MoveTo(5.08545, 8.13775), ui.LineTo(7.18455, 10.2368),
		ui.CubicTo(7.26636, 10.3187, 7.4003, 10.3142, 7.47649, 10.2271),
		ui.LineTo(11.5148, 5.61194),
	)
}

func permissionFullAccessIcon() ui.Icon {
	return permissionIconWith(
		ui.MoveTo(8, 4.39209), ui.LineTo(8, 9.89209),
		ui.MoveTo(8, 10.8081), ui.LineTo(8, 11.8081),
	)
}

func permissionAutoReviewIcon() ui.Icon {
	icon := permissionIconWith(
		ui.MoveTo(5.75, 10.15), ui.LineTo(8, 4.95), ui.LineTo(10.25, 10.15),
		ui.MoveTo(6.43, 8.59), ui.LineTo(9.57, 8.59),
	)
	icon.Cap = ui.CapRound
	return icon
}

func permissionAskIcon() ui.Icon {
	icon := permissionIconWith(
		ui.MoveTo(6.35, 6.05),
		ui.CubicTo(6.48, 4.86, 7.18, 4.2, 8.08, 4.2),
		ui.CubicTo(9.15, 4.2, 9.82, 4.88, 9.82, 5.86),
		ui.CubicTo(9.82, 6.92, 9.12, 7.39, 8.48, 7.91),
		ui.CubicTo(8.08, 8.23, 8, 8.57, 8, 9.12),
		ui.MoveTo(8, 11.2), ui.LineTo(8, 11.21),
	)
	icon.Cap = ui.CapRound
	return icon
}

func permissionDenyIcon() ui.Icon {
	icon := permissionIconWith(ui.MoveTo(5.25, 11.15), ui.LineTo(10.75, 5.65))
	icon.Cap = ui.CapRound
	return icon
}

// permissionWorkspaceWriteIcon 的官方源同时包含 currentColor 填充和描边。
// viva 的 Layers 表达填充、Commands 表达描边；填充色由调用方传 theme 色，
// 因而不在插件里写死色号，深浅主题也不会串色。
func permissionWorkspaceWriteIcon(color ui.Color) ui.Icon {
	shield := []ui.PathCommand{
		ui.MoveTo(6.4209, 1.68067),
		ui.CubicTo(7.43922, 1.299, 8.56177, 1.29898, 9.58008, 1.68067),
		ui.LineTo(14.0996, 3.375),
		ui.CubicTo(14.2946, 3.44811, 14.4236, 3.63455, 14.4238, 3.84278),
		ui.LineTo(14.4238, 6.89063),
		ui.CubicTo(14.1115, 6.71853, 13.7761, 6.58312, 13.4238, 6.48926),
		ui.LineTo(13.4238, 4.18946), ui.LineTo(9.22852, 2.61621),
		ui.CubicTo(8.43657, 2.31947, 7.56341, 2.31939, 6.77148, 2.61621),
		ui.LineTo(2.5752, 4.18946), ui.LineTo(2.5752, 7.11914),
		ui.CubicTo(2.5752, 11.1796, 5.52369, 13.056, 8, 14.0391),
		ui.CubicTo(8.27653, 13.9293, 8.55827, 13.8067, 8.8418, 13.6729),
		ui.CubicTo(9.07101, 13.9468, 9.33228, 14.1929, 9.62012, 14.4053),
		ui.CubicTo(9.12409, 14.6579, 8.63578, 14.8696, 8.17871, 15.0449),
		ui.CubicTo(8.0637, 15.0889, 7.93628, 15.0889, 7.82129, 15.0449),
		ui.CubicTo(5.22011, 14.0472, 1.5752, 11.9381, 1.5752, 7.11914),
		ui.LineTo(1.5752, 3.84278),
		ui.CubicTo(1.57541, 3.63469, 1.70463, 3.44821, 1.89941, 3.375),
		ui.LineTo(6.4209, 1.68067), ui.ClosePath(),
	}
	pencil := []ui.PathCommand{
		ui.MoveTo(10.0317, 13.2229),
		ui.CubicTo(10.263, 13.3929, 10.4943, 13.563, 10.7256, 13.733),
		ui.CubicTo(10.7932, 13.6482, 10.8608, 13.5634, 10.9284, 13.4786),
		ui.CubicTo(12.1455, 11.9522, 13.3626, 10.4258, 14.5798, 8.89935),
		ui.CubicTo(14.6474, 8.81455, 14.715, 8.72975, 14.7826, 8.64495),
		ui.CubicTo(14.4143, 8.37419, 14.046, 8.10344, 13.6777, 7.83268),
		ui.CubicTo(13.6169, 7.92252, 13.5562, 8.01236, 13.4954, 8.10219),
		ui.CubicTo(12.4016, 9.71926, 11.3078, 11.3363, 10.214, 12.9534),
		ui.CubicTo(10.1532, 13.0432, 10.0924, 13.1331, 10.0317, 13.2229), ui.ClosePath(),
	}
	underline := []ui.PathCommand{
		ui.MoveTo(12.6516, 12.6696),
		ui.CubicTo(12.6516, 12.925, 12.6516, 13.1804, 12.6516, 13.4359),
		ui.CubicTo(12.6952, 13.4378, 12.7387, 13.4398, 12.7823, 13.4417),
		ui.CubicTo(13.5663, 13.4768, 14.3504, 13.5118, 15.1345, 13.5469),
		ui.CubicTo(15.178, 13.5488, 15.2216, 13.5508, 15.2651, 13.5527),
		ui.CubicTo(15.2651, 13.2194, 15.2651, 12.8861, 15.2651, 12.5527),
		ui.CubicTo(15.2216, 12.5547, 15.178, 12.5566, 15.1345, 12.5586),
		ui.CubicTo(14.3504, 12.5936, 13.5663, 12.6287, 12.7823, 12.6637),
		ui.CubicTo(12.7387, 12.6657, 12.6952, 12.6676, 12.6516, 12.6696), ui.ClosePath(),
	}
	return ui.Icon{
		ViewBox: 16, Stroke: 1,
		Commands: []ui.PathCommand{
			ui.MoveTo(5.26392, 6.60339), ui.LineTo(10.7361, 6.60339),
			ui.MoveTo(5.26392, 9.86902), ui.LineTo(8.32833, 9.86902),
		},
		Layers: []ui.IconLayer{{Fill: color, Commands: shield}, {Fill: color, Commands: pencil}, {Fill: color, Commands: underline}},
	}
}

// permissionIconFor 只给明确知道的值配图标。未知 policy 保留原文但不猜图标。
func permissionIconFor(value string, color ui.Color) (ui.Icon, bool) {
	switch value {
	case "read-only":
		return permissionReadOnlyIcon(), true
	case "workspace-write":
		return permissionWorkspaceWriteIcon(color), true
	case "danger-full-access", "allow":
		return permissionFullAccessIcon(), true
	case "auto":
		return permissionAutoReviewIcon(), true
	case "ask":
		return permissionAskIcon(), true
	case "deny":
		return permissionDenyIcon(), true
	default:
		return ui.Icon{}, false
	}
}
