package dscomposer

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 本文件的矢量全部使用 16×16 viewBox。发送/停止路径逐点来自官方 InputBar；
// 四种 preset 图标来自官方 primitives 的 PermissionIcon*Regular。
// 宿主额外暴露的 ask/deny 是审批 policy、不是官方 preset，官方没有同名图标；
// 这两枚只复用官方盾牌轮廓，分别用问号和斜线表达真值，不冒称 preset 图标。

var primarySendIcon = gfx.Icon{
	ViewBox: 16,
	Commands: []gfx.PathCommand{
		gfx.MoveTo(8.3125, 0.980183),
		gfx.CubicTo(8.66767, 1.0531, 8.97902, 1.20418, 9.2627, 1.43233),
		gfx.CubicTo(9.48724, 1.61297, 9.73029, 1.85793, 9.97949, 2.10714),
		gfx.LineTo(14.707, 6.83468),
		gfx.LineTo(13.293, 8.24874),
		gfx.LineTo(9, 3.95577),
		gfx.LineTo(9, 15.0417),
		gfx.LineTo(7, 15.0417),
		gfx.LineTo(7, 3.95577),
		gfx.LineTo(2.70703, 8.24874),
		gfx.LineTo(1.29297, 6.83468),
		gfx.LineTo(6.02051, 2.10714),
		gfx.CubicTo(6.26971, 1.85793, 6.51277, 1.61297, 6.7373, 1.43233),
		gfx.CubicTo(6.97662, 1.23986, 7.28445, 1.04402, 7.6875, 0.980183),
		gfx.CubicTo(7.8973, 0.947006, 8.1031, 0.95516, 8.3125, 0.980183),
		gfx.ClosePath(),
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
var addPlusIcon = gfx.Icon{
	ViewBox: 16, Stroke: 1, Join: gfx.JoinRound,
	Commands: []gfx.PathCommand{
		gfx.MoveTo(8, 3.5), gfx.LineTo(8, 12.5),
		gfx.MoveTo(3.5, 8), gfx.LineTo(12.5, 8),
	},
}

var primaryStopIcon = gfx.Icon{
	ViewBox: 16,
	Commands: []gfx.PathCommand{
		// 官方 <rect x=3 y=3 width=10 height=10 rx=3> 的等价闭合路径。
		gfx.MoveTo(6, 3), gfx.LineTo(10, 3),
		gfx.QuadTo(13, 3, 13, 6), gfx.LineTo(13, 10),
		gfx.QuadTo(13, 13, 10, 13), gfx.LineTo(6, 13),
		gfx.QuadTo(3, 13, 3, 10), gfx.LineTo(3, 6),
		gfx.QuadTo(3, 3, 6, 3), gfx.ClosePath(),
	},
}

func permissionShield() []gfx.PathCommand {
	return []gfx.PathCommand{
		gfx.MoveTo(6.59624, 2.14853),
		gfx.CubicTo(7.50155, 1.80917, 8.49914, 1.80919, 9.40444, 2.14859),
		gfx.LineTo(13.9245, 3.84317), gfx.LineTo(13.9245, 7.11961),
		gfx.CubicTo(13.9245, 11.6089, 10.5565, 13.5975, 8.00035, 14.5779),
		gfx.CubicTo(5.44423, 13.5975, 2.07544, 11.6089, 2.07544, 7.11961),
		gfx.LineTo(2.07544, 3.84317), gfx.LineTo(6.59624, 2.14853), gfx.ClosePath(),
	}
}

func permissionIconWith(extra ...gfx.PathCommand) gfx.Icon {
	commands := permissionShield()
	commands = append(commands, extra...)
	return gfx.Icon{ViewBox: 16, Stroke: 1, Join: gfx.JoinRound, Commands: commands}
}

func permissionReadOnlyIcon() gfx.Icon {
	return permissionIconWith(
		gfx.MoveTo(5.08545, 8.13775), gfx.LineTo(7.18455, 10.2368),
		gfx.CubicTo(7.26636, 10.3187, 7.4003, 10.3142, 7.47649, 10.2271),
		gfx.LineTo(11.5148, 5.61194),
	)
}

func permissionFullAccessIcon() gfx.Icon {
	return permissionIconWith(
		gfx.MoveTo(8, 4.39209), gfx.LineTo(8, 9.89209),
		gfx.MoveTo(8, 10.8081), gfx.LineTo(8, 11.8081),
	)
}

func permissionAutoReviewIcon() gfx.Icon {
	icon := permissionIconWith(
		gfx.MoveTo(5.75, 10.15), gfx.LineTo(8, 4.95), gfx.LineTo(10.25, 10.15),
		gfx.MoveTo(6.43, 8.59), gfx.LineTo(9.57, 8.59),
	)
	icon.Cap = gfx.CapRound
	return icon
}

func permissionAskIcon() gfx.Icon {
	icon := permissionIconWith(
		gfx.MoveTo(6.35, 6.05),
		gfx.CubicTo(6.48, 4.86, 7.18, 4.2, 8.08, 4.2),
		gfx.CubicTo(9.15, 4.2, 9.82, 4.88, 9.82, 5.86),
		gfx.CubicTo(9.82, 6.92, 9.12, 7.39, 8.48, 7.91),
		gfx.CubicTo(8.08, 8.23, 8, 8.57, 8, 9.12),
		gfx.MoveTo(8, 11.2), gfx.LineTo(8, 11.21),
	)
	icon.Cap = gfx.CapRound
	return icon
}

func permissionDenyIcon() gfx.Icon {
	icon := permissionIconWith(gfx.MoveTo(5.25, 11.15), gfx.LineTo(10.75, 5.65))
	icon.Cap = gfx.CapRound
	return icon
}

// permissionWorkspaceWriteIcon 的官方源同时包含 currentColor 填充和描边。
// viva 的 Layers 表达填充、Commands 表达描边；填充色由调用方传 theme 色，
// 因而不在插件里写死色号，深浅主题也不会串色。
func permissionWorkspaceWriteIcon(color ui.Color) gfx.Icon {
	shield := []gfx.PathCommand{
		gfx.MoveTo(6.4209, 1.68067),
		gfx.CubicTo(7.43922, 1.299, 8.56177, 1.29898, 9.58008, 1.68067),
		gfx.LineTo(14.0996, 3.375),
		gfx.CubicTo(14.2946, 3.44811, 14.4236, 3.63455, 14.4238, 3.84278),
		gfx.LineTo(14.4238, 6.89063),
		gfx.CubicTo(14.1115, 6.71853, 13.7761, 6.58312, 13.4238, 6.48926),
		gfx.LineTo(13.4238, 4.18946), gfx.LineTo(9.22852, 2.61621),
		gfx.CubicTo(8.43657, 2.31947, 7.56341, 2.31939, 6.77148, 2.61621),
		gfx.LineTo(2.5752, 4.18946), gfx.LineTo(2.5752, 7.11914),
		gfx.CubicTo(2.5752, 11.1796, 5.52369, 13.056, 8, 14.0391),
		gfx.CubicTo(8.27653, 13.9293, 8.55827, 13.8067, 8.8418, 13.6729),
		gfx.CubicTo(9.07101, 13.9468, 9.33228, 14.1929, 9.62012, 14.4053),
		gfx.CubicTo(9.12409, 14.6579, 8.63578, 14.8696, 8.17871, 15.0449),
		gfx.CubicTo(8.0637, 15.0889, 7.93628, 15.0889, 7.82129, 15.0449),
		gfx.CubicTo(5.22011, 14.0472, 1.5752, 11.9381, 1.5752, 7.11914),
		gfx.LineTo(1.5752, 3.84278),
		gfx.CubicTo(1.57541, 3.63469, 1.70463, 3.44821, 1.89941, 3.375),
		gfx.LineTo(6.4209, 1.68067), gfx.ClosePath(),
	}
	pencil := []gfx.PathCommand{
		gfx.MoveTo(10.0317, 13.2229),
		gfx.CubicTo(10.263, 13.3929, 10.4943, 13.563, 10.7256, 13.733),
		gfx.CubicTo(10.7932, 13.6482, 10.8608, 13.5634, 10.9284, 13.4786),
		gfx.CubicTo(12.1455, 11.9522, 13.3626, 10.4258, 14.5798, 8.89935),
		gfx.CubicTo(14.6474, 8.81455, 14.715, 8.72975, 14.7826, 8.64495),
		gfx.CubicTo(14.4143, 8.37419, 14.046, 8.10344, 13.6777, 7.83268),
		gfx.CubicTo(13.6169, 7.92252, 13.5562, 8.01236, 13.4954, 8.10219),
		gfx.CubicTo(12.4016, 9.71926, 11.3078, 11.3363, 10.214, 12.9534),
		gfx.CubicTo(10.1532, 13.0432, 10.0924, 13.1331, 10.0317, 13.2229), gfx.ClosePath(),
	}
	underline := []gfx.PathCommand{
		gfx.MoveTo(12.6516, 12.6696),
		gfx.CubicTo(12.6516, 12.925, 12.6516, 13.1804, 12.6516, 13.4359),
		gfx.CubicTo(12.6952, 13.4378, 12.7387, 13.4398, 12.7823, 13.4417),
		gfx.CubicTo(13.5663, 13.4768, 14.3504, 13.5118, 15.1345, 13.5469),
		gfx.CubicTo(15.178, 13.5488, 15.2216, 13.5508, 15.2651, 13.5527),
		gfx.CubicTo(15.2651, 13.2194, 15.2651, 12.8861, 15.2651, 12.5527),
		gfx.CubicTo(15.2216, 12.5547, 15.178, 12.5566, 15.1345, 12.5586),
		gfx.CubicTo(14.3504, 12.5936, 13.5663, 12.6287, 12.7823, 12.6637),
		gfx.CubicTo(12.7387, 12.6657, 12.6952, 12.6676, 12.6516, 12.6696), gfx.ClosePath(),
	}
	return gfx.Icon{
		ViewBox: 16, Stroke: 1,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(5.26392, 6.60339), gfx.LineTo(10.7361, 6.60339),
			gfx.MoveTo(5.26392, 9.86902), gfx.LineTo(8.32833, 9.86902),
		},
		Layers: []gfx.IconLayer{{Fill: color, Commands: shield}, {Fill: color, Commands: pencil}, {Fill: color, Commands: underline}},
	}
}

// permissionIconFor 只给明确知道的值配图标。未知 policy 保留原文但不猜图标。
func permissionIconFor(value string, color ui.Color) (gfx.Icon, bool) {
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
		return gfx.Icon{}, false
	}
}
