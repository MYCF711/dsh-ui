// icons.go —— 本包用到的官方 sprite 符号（rc.2 `dsh-client-ui-primitives` 的 sprite 表）。
//
//   - layers-outline（16×16，`IconLayersOutlineRegular size=14`）：JobListAction 触发器的
//     空闲态图标（`OFF:dsh-client-ui-jobs/src/client/JobListAction.tsx:549-552`）；
//   - stop-fill（16×16，`IconStopFillRegular size=10`）：运行中行停止钮（`JobListAction.tsx:280-287`）。
//
// 素材来源：官方 rc.2 的 sprite 表在 `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`
// 的 `"layers-outline":{size:16,svg:...}` / `"stop-fill":{size:16,svg:...}` 段；
// **viewBox 一律官方原值 16**，显示尺寸由调用方按官方 `size` 传（layers 14 / stop 10）。
//
// 注意：`layers-outline` 的原路径用的是 SVG `a`（圆弧）命令，而 viva 的 gfx.Icon
// 只有 Move/Line/Quad/Cubic——这里的曲线是把圆弧按**标准 arc→cubic（每段 ≤90°）**
// 机械换算出来的，换算脚本与输入在 `tmp/GO8-RC2/_gp2/svg2gfx.txt`。
package dsjobs

import "github.com/vivaui/viva/gfx"

// layersIcon 是官方 `IconLayersOutlineRegular`（3 段描边：顶层圆角菱形 + 两条层线）。
func layersIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(8.48137, 2.30437),
			gfx.CubicTo(8.17561, 2.16491, 7.82439, 2.16491, 7.51863, 2.30437),
			gfx.LineTo(2.54797, 4.56637),
			gfx.CubicTo(2.3377, 4.65911, 2.20202, 4.86726, 2.20202, 5.09707),
			gfx.CubicTo(2.20202, 5.32689, 2.3377, 5.53503, 2.54797, 5.62777),
			gfx.LineTo(7.52437, 7.8956),
			gfx.CubicTo(7.83016, 8.03509, 8.18141, 8.03509, 8.4872, 7.8956),
			gfx.LineTo(13.4636, 5.6336),
			gfx.CubicTo(13.6739, 5.54086, 13.8096, 5.33271, 13.8096, 5.1029),
			gfx.CubicTo(13.8096, 4.87309, 13.6739, 4.66494, 13.4636, 4.5722),
			gfx.ClosePath(),
			gfx.MoveTo(2.19997, 8),
			gfx.CubicTo(2.19941, 8.22655, 2.3308, 8.43268, 2.5364, 8.52783),
			gfx.LineTo(7.52437, 10.7956),
			gfx.CubicTo(7.82851, 10.9333, 8.17724, 10.9333, 8.48137, 10.7956),
			gfx.LineTo(13.4578, 8.53357),
			gfx.CubicTo(13.6676, 8.4393, 13.8018, 8.22998, 13.8, 8),
			gfx.MoveTo(2.19997, 10.9),
			gfx.CubicTo(2.19941, 11.1265, 2.3308, 11.3327, 2.5364, 11.4278),
			gfx.LineTo(7.52437, 13.6956),
			gfx.CubicTo(7.82851, 13.8334, 8.17724, 13.8334, 8.48137, 13.6956),
			gfx.LineTo(13.4578, 11.4336),
			gfx.CubicTo(13.6676, 11.3393, 13.8019, 11.13, 13.8, 10.9),
		},
	}
}

// stopIcon 是官方 `IconStopFillRegular`（实心圆角方块；Stroke=0 表示按填充画）。
func stopIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(12.5, 2.5),
			gfx.LineTo(3.5, 2.5),
			gfx.CubicTo(2.94772, 2.5, 2.5, 2.94772, 2.5, 3.5),
			gfx.LineTo(2.5, 12.5),
			gfx.CubicTo(2.5, 13.0523, 2.94772, 13.5, 3.5, 13.5),
			gfx.LineTo(12.5, 13.5),
			gfx.CubicTo(13.0523, 13.5, 13.5, 13.0523, 13.5, 12.5),
			gfx.LineTo(13.5, 3.5),
			gfx.CubicTo(13.5, 2.94772, 13.0523, 2.5, 12.5, 2.5),
			gfx.ClosePath(),
		},
	}
}
