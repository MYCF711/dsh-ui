package dsassets

import "github.com/vivaui/viva/gfx"

// 自动化任务详情的官方操作图标。path 逐字来自 rc.2
// dsh-web-frontend/dist/assets/index-ghOpC9uV.js:129 的 sprite 表：
// play-outline / pause-outline / close-outline / ellipsis-outline。
// 与 SidebarClock 一样按需接线，不加入 All() 的既有 golden 基线。

// SchedulePlayOutline 是 TaskDetail.tsx:333 的 IconPlayOutlineRegular。
func SchedulePlayOutline(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M8 14.5C11.5899 14.5 14.5 11.5899 14.5 8C14.5 4.41015 11.5899 1.5 8 1.5C4.41015 1.5 1.5 4.41015 1.5 8C1.5 11.5899 4.41015 14.5 8 14.5Z`, Stroke: true},
		Element{Kind: PathElement, D: `M10.3329 7.91346C10.3996 7.95195 10.3996 8.04818 10.3329 8.08667L6.78304 10.1362C6.71638 10.1747 6.63304 10.1266 6.63304 10.0496L6.63304 5.95055C6.63304 5.87357 6.71638 5.82546 6.78304 5.86395L10.3329 7.91346Z`, Stroke: true},
	)
}

// SchedulePauseOutline 是 TaskDetail.tsx:333 的 IconPauseOutlineRegular。
func SchedulePauseOutline(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M8 14.5C11.5899 14.5 14.5 11.5899 14.5 8C14.5 4.41015 11.5899 1.5 8 1.5C4.41015 1.5 1.5 4.41015 1.5 8C1.5 11.5899 4.41015 14.5 8 14.5Z`, Stroke: true},
		Element{Kind: PathElement, D: `M6.5 5V11`, Stroke: true},
		Element{Kind: PathElement, D: `M9.5 5V11`, Stroke: true},
	)
}

// ScheduleCloseOutline 是 TaskDetail.tsx:355-357 的 IconCloseOutlineRegular；
// 同一路径也已存于 assets/official/icons/settings/close-outline.jsx:1-2。
func ScheduleCloseOutline(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M2.5 2.5L13.5 13.5`, Stroke: true},
		Element{Kind: PathElement, D: `M13.5 2.5L2.5 13.5`, Stroke: true},
	)
}

// ScheduleEllipsisOutline 是 TaskDetail.tsx:337-340 的 IconEllipsisOutlineRegular。
// 三个圆点都是 fill=currentColor，不能改成描边圆或文字省略号。
func ScheduleEllipsisOutline(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M3 9C3.55228 9 4 8.55228 4 8C4 7.44772 3.55228 7 3 7C2.44772 7 2 7.44772 2 8C2 8.55228 2.44772 9 3 9Z`, Fill: true},
		Element{Kind: PathElement, D: `M8 9C8.55228 9 9 8.55228 9 8C9 7.44772 8.55228 7 8 7C7.44772 7 7 7.44772 7 8C7 8.55228 7.44772 9 8 9Z`, Fill: true},
		Element{Kind: PathElement, D: `M13 9C13.5523 9 14 8.55228 14 8C14 7.44772 13.5523 7 13 7C12.4477 7 12 7.44772 12 8C12 8.55228 12.4477 9 13 9Z`, Fill: true},
	)
}
