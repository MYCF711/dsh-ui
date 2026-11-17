package dssidebar

// icons.go —— 本包为 More 菜单新接的图标（**不是手画的**）。
//
// # pin-fill（官方 `IconPinFillRegular`）
//
// 官方用处：面板行尾的「取消固定」按钮（`<IconPinFillRegular size={14}/>`，
// OFF:dsh-client-ui-sidebar/src/client/SidebarRoot.tsx:113-120 的 `.panelUnpin`）。
//
// 取法：rc.2 安装态的图标 sprite 表（不是 compoent 源码——primitives 的图标是运行时
// 由 sprite 字符串生成 `<svg>`）：
//
//	dist: /dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-ghOpC9uV.js
//	      整文件 1093231 B，sha256 061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8
//	sprite `"pin-fill"` 起于字节 730374；`svg(1)` 片段 sha256
//	      51c6115cc935c69bfad1836e0a51ad5b44cf58b595944bd088d1df2e7a95cc07
//
// 形状：viewBox `0 0 16 16`、regular 档 stroke-width 1。第一条 path **同时**填充与
// 描边（`stroke-linejoin="round"`），第二条只填充。所以这里建一个"描边一条 + 两个
// 填充层"的 icon；填充层的颜色必须由调用方传（viva 的 tint 只对层贡献 alpha，
// 见 `gfx.IconLayer`），描边的颜色由 `.Color()` 给。

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// pinFillHeadPath / pinFillTailPath 是 sprite `"pin-fill"` 的两条 `d` 原文
// （逐字节，未重排）。
const (
	pinFillHeadPath = `M9.96976 1.70572L13.1554 3.93629L10.9019 8.12317L11.5158 11.605L10.7192 12.7427L2.52767 7.00693L3.3243 5.86922L6.80612 5.25528L9.96976 1.70572Z`
	pinFillTailPath = `M6.05285 9.47511C6.27284 9.16094 6.70586 9.08458 7.02003 9.30457C7.3342 9.52455 7.41055 9.95757 7.19057 10.2717L3.98587 14.4708L3.21223 13.9291L6.05285 9.47511Z`
)

// pinFillIcon 造官方 `IconPinFillRegular`；color 是**填充层**的颜色（描边色由
// 调用方 `.Color()` 给，本包两个调用点都给同一个 caption 色）。
//
// 返回 false 表示路径解析失败（与 dsassets 的约定一致：调用方自己决定不画，
// 不许拿近似形状顶上）。
func pinFillIcon(color gfx.Color) (gfx.Icon, bool) {
	head, ok := dsassets.ParsePath(pinFillHeadPath)
	if !ok {
		return gfx.Icon{}, false
	}
	tail, ok := dsassets.ParsePath(pinFillTailPath)
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{
		ViewBox:  16,
		Stroke:   1,
		Join:     gfx.JoinRound,
		Commands: head,
		Layers: []gfx.IconLayer{
			{Fill: color, Commands: head},
			{Fill: color, Commands: tail},
		},
	}, true
}

// sidebarPanelIcon 返回面板注册方自己的官方图标；扩展面板没有官方素材时不画。
func sidebarPanelIcon(id string, color gfx.Color) (gfx.Icon, bool) {
	switch id {
	case "plugins":
		return dsassets.SidebarPluginPinwheel(color)
	case "schedules":
		return dsassets.SidebarClock(color)
	default:
		return gfx.Icon{}, false
	}
}

// drawSlidersTwoIcon 用官方 path + 两枚圆环组成 IconSlidersTwoOutlineRegular。
// 圆环由 Viva 的圆形 Border 画出，未做贝塞尔近似。
func drawSlidersTwoIcon(f ui.C, color gfx.Color, size float32) {
	icon, rings, ok := dsassets.WorkspaceSlidersTwo(color)
	if !ok {
		return
	}
	scale := size / 16
	ui.Row(f).Width(size).Height(size).Children(func() {
		ui.Icon(f, icon).Size(size, size).Color(color)
		for _, ring := range rings {
			d := 2 * ring.Radius * scale
			ui.Row(f).Absolute().Left((ring.CX-ring.Radius)*scale).Top((ring.CY-ring.Radius)*scale).
				Width(d).Height(d).Radius(gfx.RadiusFull).Border(ring.Stroke*scale, color)
		}
	})
}

// drawAppsAddIcon 用官方 path + 三枚圆角矩形组成 More 行图标。
func drawAppsAddIcon(f ui.C, color gfx.Color, size float32) {
	icon, rects, ok := dsassets.SidebarAppsAdd(color)
	if !ok {
		return
	}
	scale := size / 16
	ui.Row(f).Width(size).Height(size).Children(func() {
		ui.Icon(f, icon).Size(size, size).Color(color)
		for _, rect := range rects {
			ui.Row(f).Absolute().Left(rect.X*scale).Top(rect.Y*scale).
				Width(rect.Width*scale).Height(rect.Height*scale).
				Radius(rect.Radius*scale).Border(rect.Stroke*scale, color)
		}
	})
}
