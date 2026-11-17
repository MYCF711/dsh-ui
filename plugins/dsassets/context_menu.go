package dsassets

import "github.com/vivaui/viva/gfx"

// 右键菜单素材全部来自 rc.2 图标 sprite
// dsh-web-frontend/dist/assets/index-ghOpC9uV.js:129。
// copy 的圆角 rect 与 save 的 SVG a 命令由 `viva icons` 编译为三次曲线；
// 其余 path 原文由 Build/ParsePath 无损展开，不手画替代形状。

// ContextMenuButtonIcon 是 Button.tsx:49 的 IconEllipsisOutlineRegular。
func ContextMenuButtonIcon(fill gfx.Color) (gfx.Icon, bool) {
	return Build(Asset{ViewBox: 16, Size: 16, Elements: []Element{
		{Kind: PathElement, D: `M3 9C3.55228 9 4 8.55228 4 8C4 7.44772 3.55228 7 3 7C2.44772 7 2 7.44772 2 8C2 8.55228 2.44772 9 3 9Z`, Fill: true},
		{Kind: PathElement, D: `M8 9C8.55228 9 9 8.55228 9 8C9 7.44772 8.55228 7 8 7C7.44772 7 7 7.44772 7 8C7 8.55228 7.44772 9 8 9Z`, Fill: true},
		{Kind: PathElement, D: `M13 9C13.5523 9 14 8.55228 14 8C14 7.44772 13.5523 7 13 7C12.4477 7 12 7.44772 12 8C12 8.55228 12.4477 9 13 9Z`, Fill: true},
	}}, fill)
}

// ContextMenuIcon 按官方 contextMenu.icon 的 key 取对应素材。
// 映射证据：workspace/src/client/index.ts:317-320、
// file-actions/src/client/index.ts:33-34。
func ContextMenuIcon(key string, fill gfx.Color) (gfx.Icon, bool) {
	switch key {
	case "workspace.rename":
		return PlanReviewEdit(fill) // IconEditOutlineRegular
	case "workspace.delete":
		return MarketTrashOutline(fill) // IconTrashOutlineRegular
	case "workspace.copy", "open-in-app.copy", "file-actions.copy":
		return contextMenuCopy(fill)
	case "file-actions.download":
		return Build(Asset{ViewBox: 16, Size: 14, StrokeWidth: regularStroke, Elements: []Element{
			{Kind: PathElement, D: `M8 1.95317V10.0469`, Stroke: true},
			{Kind: PathElement, D: `M4.25 6.29688L8 10.0469L11.75 6.29688`, Stroke: true},
			{Kind: PathElement, D: `M1.5 10.0469V13.158C1.5 13.3937 1.60536 13.6198 1.79289 13.7865C1.98043 13.9532 2.23478 14.0469 2.5 14.0469H13.5C13.7652 14.0469 14.0196 13.9532 14.2071 13.7865C14.3946 13.6198 14.5 13.3937 14.5 13.158V10.0469`, Stroke: true},
		}}, fill)
	case "file-actions.save":
		return contextMenuSave(), true
	default:
		return gfx.Icon{}, false
	}
}

func contextMenuCopy(fill gfx.Color) (gfx.Icon, bool) {
	back, ok := ParsePath(copyBackPath)
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  regularStroke,
		// 官方 <rect x=1.52075 y=4.07373 width=10.3932 height=10.3932 rx=2>。
		Commands: []gfx.PathCommand{
			gfx.MoveTo(9.91395, 4.07373),
			gfx.CubicTo(11.0185, 4.07373, 11.9139, 4.96916, 11.9139, 6.07373),
			gfx.LineTo(11.9139, 12.4669),
			gfx.CubicTo(11.9139, 13.5715, 11.0185, 14.4669, 9.91395, 14.4669),
			gfx.LineTo(3.52075, 14.4669),
			gfx.CubicTo(2.41618, 14.4669, 1.52075, 13.5715, 1.52075, 12.4669),
			gfx.LineTo(1.52075, 6.07373),
			gfx.CubicTo(1.52075, 4.96916, 2.41618, 4.07373, 3.52075, 4.07373),
			gfx.ClosePath(),
		},
		Layers: []gfx.IconLayer{{Fill: fill, Commands: back}},
	}, true
}

func contextMenuSave() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  regularStroke,
		Join:    gfx.JoinRound, // 官方 svg stroke-linejoin="round"；linecap 保持默认 butt。
		Commands: []gfx.PathCommand{
			gfx.MoveTo(2.5, 1.5),
			gfx.LineTo(10.5, 1.5),
			gfx.LineTo(13.5, 4.5),
			gfx.LineTo(13.5, 13.5),
			gfx.CubicTo(13.5, 14.0523, 13.0523, 14.5, 12.5, 14.5),
			gfx.LineTo(2.5, 14.5),
			gfx.CubicTo(1.94772, 14.5, 1.5, 14.0523, 1.5, 13.5),
			gfx.LineTo(1.5, 2.5),
			gfx.CubicTo(1.5, 1.94772, 1.94772, 1.5, 2.5, 1.5),
			gfx.ClosePath(),
			gfx.MoveTo(4.5, 1.5),
			gfx.LineTo(4.5, 5.5),
			gfx.LineTo(10.5, 5.5),
			gfx.LineTo(10.5, 1.5),
			gfx.MoveTo(4, 14.5),
			gfx.LineTo(4, 8.5),
			gfx.LineTo(11, 8.5),
			gfx.LineTo(11, 14.5),
		},
	}
}
