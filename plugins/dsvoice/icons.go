package dsvoice

// icons.go —— 三枚图标的**官方原始几何**（不是本仓手画的）。
//
// # 取自哪里
//
// 官方 `@deepseek-ai/dsh-client-ui-primitives` 的图标是**精灵表**：组件按
// `M("名字", 档位)` 生成一个 `<use href="#id">`，真正的几何在 web 前端的
// 精灵定义里（`dsh-web-frontend/dist/assets/index-ghOpC9uV.js` 的
// `"microphone-outline":{size:16,svg:e=>…}` 那一段）。官方客户端包
// （`dsh-experimental-client-ui-voice-input/lib/client.js:3560-3600`）引用的
// 就是 `IconMicrophoneOutlineRegular` / `IconCloseOutlineRegular` /
// `IconStopFillRegular` 三枚，尺寸 18 / 14 / 14。
//
// 本仓的素材库（`plugins/dsassets`）里没有这三枚，所以这里把它们按**官方 path
// 原文**登记成本包私有图元——不是重画，也不是"看起来差不多"：
//
//	microphone-outline regular：
//	  <rect x="5" y="1.5" width="6" height="9" rx="3">（stroke-width 1）
//	  <path d="M2.35 8.675C3.075 11.3 5.2 13.125 8 13.125C10.8 13.125 12.925 11.3 13.65 8.675M8 13.125V15">
//	close-outline regular：
//	  <path d="M2.5 2.5L13.5 13.5"><path d="M13.5 2.5L2.5 13.5">（stroke-width 1）
//	stop-fill regular：
//	  <path d="M12.5 2.5H3.5C2.94772 2.5 2.5 2.94772 2.5 3.5V12.5C2.5 13.0523 2.94772 13.5 3.5 13.5H12.5C13.0523 13.5 13.5 13.0523 13.5 12.5V3.5C13.5 2.94772 13.0523 2.5 12.5 2.5Z" fill="currentColor">
//
// # 麦克风的胶囊为什么要单独画
//
// 官方那枚 `rect rx=3` 的圆角是**椭圆弧**，而 viva 的路径词汇只有
// M/L/Q/C/Z（`viva/gfx/path.go:9-14`）。拿三次贝塞尔去凑就是"看起来差不多"，
// 本仓已经为停止键踩过这个坑（`plugins/dsassets/registry.go:129-131`）。
// 所以胶囊用 viva 的**原生圆角矩形**（`Box.Radius + Box.Border`）画——
// 那条 `<rect>` 的几何原样搬过来（x/y/width/height/rx 乘显示缩放），
// 括号与支架走官方 path，两者叠在同一个 16×16 的坐标系里。

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方精灵表的 viewBox（三枚都是 16）。
const iconViewBox = 16

// 官方 Regular 档描边宽度（`dsh-web-frontend` 精灵模板里的 `${e}`，
// 与 `plugins/dsassets/registry.go` 的 `regularStroke` 同值）。
const iconStroke = 1

// microphoneCapsule 是官方 `<rect x=5 y=1.5 width=6 height=9 rx=3>`（regular 档）。
//
// 注意：同一个精灵里 `stroke-width===1.3`（medium 档）时 rect 是
// `x=5.15 y=1.65 width=5.7 height=8.7 rx=2.85`——**两个档位的几何不同**，
// 不是同一份缩放。官方 voice-input 用的是 Regular，所以取前者。
var microphoneCapsule = struct{ X, Y, W, H, RX float32 }{X: 5, Y: 1.5, W: 6, H: 9, RX: 3}

// microphoneBracketPath 是官方那条括号 + 支架（M/C/V，viva 全套支持）。
var microphoneBracketPath = []gfx.PathCommand{
	gfx.MoveTo(2.35, 8.675),
	gfx.CubicTo(3.075, 11.3, 5.2, 13.125, 8, 13.125),
	gfx.CubicTo(10.8, 13.125, 12.925, 11.3, 13.65, 8.675),
	gfx.MoveTo(8, 13.125),
	gfx.LineTo(8, 15),
}

// closePath 是官方 `close-outline` 的两条斜线（M/L）。
var closePath = []gfx.PathCommand{
	gfx.MoveTo(2.5, 2.5), gfx.LineTo(13.5, 13.5),
	gfx.MoveTo(13.5, 2.5), gfx.LineTo(2.5, 13.5),
}

// stopPath 是官方 `stop-fill` 的圆角方块（M/L/H/V/C/Z，全在 viva 词汇里）。
var stopPath = []gfx.PathCommand{
	gfx.MoveTo(12.5, 2.5), gfx.LineTo(3.5, 2.5),
	gfx.CubicTo(2.94772, 2.5, 2.5, 2.94772, 2.5, 3.5),
	gfx.LineTo(2.5, 12.5),
	gfx.CubicTo(2.5, 13.0523, 2.94772, 13.5, 3.5, 13.5),
	gfx.LineTo(12.5, 13.5),
	gfx.CubicTo(13.0523, 13.5, 13.5, 13.0523, 13.5, 12.5),
	gfx.LineTo(13.5, 3.5),
	gfx.CubicTo(13.5, 2.94772, 13.0523, 2.5, 12.5, 2.5),
	gfx.ClosePath(),
}

var (
	bracketIcon = gfx.Icon{ViewBox: iconViewBox, Stroke: iconStroke, Commands: microphoneBracketPath}
	closeIcon   = gfx.Icon{ViewBox: iconViewBox, Stroke: iconStroke, Commands: closePath}
	// stopIcon 是**填充**图元：viva 用 Stroke==0 表示填充（gfx/icon.go:32-33），
	// 没有单独的 Fill 字段。
	stopIcon = gfx.Icon{ViewBox: iconViewBox, Commands: stopPath}
)

// drawMicrophone 在 size×size 的方框里画官方那枚麦克风：胶囊（原生圆角矩形
// + 描边）+ 括号支架（官方 path）。
//
// 为什么用 Stack + Absolute 而不是一行居中叠两个：`<rect>` 与 `<path>` 在同一个
// 16×16 坐标系里**各自有确定位置**（胶囊在 x=5..11，括号在 x=2.35..13.65），
// 用 flex 居中对齐只能对齐中心，对不出这两个不同的包围盒。
func drawMicrophone(f ui.C, size float32, color ui.Color) {
	scale := size / iconViewBox
	stack := ui.Stack(f).Size(size, size)
	stack.Children(func() {
		ui.Row(f).Absolute().
			Left(microphoneCapsule.X*scale).Top(microphoneCapsule.Y*scale).
			Size(microphoneCapsule.W*scale, microphoneCapsule.H*scale).
			Radius(microphoneCapsule.RX*scale).
			Border(iconStroke*scale, color)
		ui.Icon(f, bracketIcon).Absolute().Left(0).Top(0).Size(size, size).Color(color)
	})
}

// drawClose 画官方那枚叉（14px 显示）。
func drawClose(f ui.C, size float32, color ui.Color) {
	ui.Icon(f, closeIcon).Size(size, size).Color(color)
}

// drawStop 画官方那枚实心圆角方块（14px 显示）。
func drawStop(f ui.C, size float32, color ui.Color) {
	ui.Icon(f, stopIcon).Size(size, size).Color(color)
}
