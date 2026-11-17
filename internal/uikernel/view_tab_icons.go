package uikernel

import "github.com/vivaui/viva/gfx"

// 右栏芯片控件使用官方 rc.2 sprite 原路径，不用 + / × 字符近似。
// 来源：dsh-web-frontend/dist/assets/index-ghOpC9uV.js:129 的
// close-fill / plus-outline，viewBox=16、regular stroke=1；dockkit 调用处均显式 size={14}。
// sprite 未声明 stroke-linecap / stroke-linejoin，因此 Cap / Join 保持零值。

// rightTabCloseIcon 是官方 IconCloseFillRegular。
func rightTabCloseIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(3.5, 3.5), gfx.LineTo(12.5, 12.5),
			gfx.MoveTo(12.5, 3.5), gfx.LineTo(3.5, 12.5),
		},
	}
}

// rightTabAddIcon 是官方 IconPlusOutlineRegular。
func rightTabAddIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(8, 2), gfx.LineTo(8, 14),
			gfx.MoveTo(2, 8), gfx.LineTo(14, 8),
		},
	}
}
