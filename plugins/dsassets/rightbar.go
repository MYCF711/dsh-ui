package dsassets

import "github.com/vivaui/viva/gfx"

// 右栏与计划卡使用的官方矢量原件。路径逐字来自 rc.2，不重新描形。
//
// panel-left：assets/official/icons/sidebar/panel-left.jsx:1-2；
// 镜像规则：dsh-client-ui-sidebar-right/src/client/shell/SidebarRight.module.css:161-164。
// fullscreen：dsh-client-ui-sidebar-right/lib/client.js 的 FullscreenGlyph /
// ExitFullscreenGlyph（文件字节偏移 47818 / 48672；src/client/shell/SidebarRight.tsx:253-269）。
// markdown：dsh-web-frontend/dist/assets/index-ghOpC9uV.js 的 FileTypeIcon，
// 公共底板 _Z（偏移 883961）、折角 S7（884691）、markdown 分支（890768）。

const rightbarPanelLeftFrame = `M13.5 1.5H2.5C1.94772 1.5 1.5 1.94772 1.5 2.5V13.5C1.5 14.0523 1.94772 14.5 2.5 14.5H13.5C14.0523 14.5 14.5 14.0523 14.5 13.5V2.5C14.5 1.94772 14.0523 1.5 13.5 1.5Z`
const rightbarPanelLeftDivider = `M5.5 1.5V14.5`

// RightbarPanelLeft 复用 IconPanelLeftOutlineRegular，并执行官方 scaleX(-1)。
func RightbarPanelLeft() (gfx.Icon, bool) {
	commands := make([]gfx.PathCommand, 0, 32)
	for _, d := range []string{rightbarPanelLeftFrame, rightbarPanelLeftDivider} {
		parsed, ok := ParsePath(d)
		if !ok {
			return gfx.Icon{}, false
		}
		commands = append(commands, mirrorRightbarX(parsed, 16)...)
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}

func mirrorRightbarX(commands []gfx.PathCommand, viewBox float32) []gfx.PathCommand {
	out := make([]gfx.PathCommand, len(commands))
	for i, command := range commands {
		out[i] = command
		if command.Kind == gfx.PathCommandClose {
			continue
		}
		out[i].Point.X = viewBox - command.Point.X
		out[i].Control1.X = viewBox - command.Control1.X
		out[i].Control2.X = viewBox - command.Control2.X
	}
	return out
}

const rightbarFullscreenPath = `M2.33203 10.4054V13.1681C2.33229 13.444 2.55605 13.6681 2.83203 13.6681H5.49512V14.6681H2.83203C2.00376 14.6681 1.33229 13.9963 1.33203 13.1681V10.4054H2.33203ZM14.6689 13.1681C14.6687 13.996 13.9968 14.6676 13.1689 14.6681H10.4951V13.6681H13.1689C13.4445 13.6676 13.6687 13.4437 13.6689 13.1681V10.4054H14.6689V13.1681ZM13.1689 1.33118C13.9969 1.33163 14.6688 2.00315 14.6689 2.83118V5.4054H13.6689V2.83118C13.6688 2.55544 13.4446 2.33162 13.1689 2.33118H10.4951V1.33118H13.1689ZM5.49512 2.33118H2.83203C2.55598 2.33118 2.33218 2.55516 2.33203 2.83118V5.4054H1.33203V2.83118C1.33218 2.00288 2.00369 1.33118 2.83203 1.33118H5.49512V2.33118Z`

// RightbarFullscreen 是右栏 PanelChrome 的进入全屏图标。
func RightbarFullscreen() (gfx.Icon, bool) {
	commands, ok := ParsePath(rightbarFullscreenPath)
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Commands: commands}, true
}

// RightbarExitFullscreen 是右栏 PanelChrome 的退出全屏图标。
func RightbarExitFullscreen() (gfx.Icon, bool) {
	first, ok := ParsePath(`M9 2.5V6C9 6.26522 9.10536 6.51957 9.29289 6.70711C9.48043 6.89464 9.73478 7 10 7H13.5`)
	if !ok {
		return gfx.Icon{}, false
	}
	second, ok := ParsePath(`M7 13.5V10C7 9.73478 6.89464 9.48043 6.70711 9.29289C6.51957 9.10536 6.26522 9 6 9H2.5`)
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: append(first, second...)}, true
}

const fileTypeBasePath = `M8.48924 28H19.5108C21.6479 28 22.7165 28 23.5594 27.6509C24.6833 27.1853 25.5762 26.2924 26.0417 25.1685C26.3909 24.3256 26.3909 23.257 26.3909 21.1199V8.79443C26.3909 8.32877 26.3909 8.09593 26.3471 7.87507C26.2887 7.58058 26.173 7.30042 26.0067 7.05048C25.882 6.86303 25.7177 6.69799 25.3893 6.36792L20.0611 1.01354C19.7304 0.681235 19.5651 0.515081 19.3769 0.38885C19.126 0.220541 18.8443 0.103463 18.5481 0.0443412C18.3259 0 18.0915 0 17.6226 0H8.48924C6.35209 0 5.28351 0 4.4406 0.349145C3.31672 0.814671 2.4238 1.70759 1.95828 2.83147C1.60913 3.67438 1.60913 4.74296 1.60913 6.88011V21.1199C1.60913 23.257 1.60913 24.3256 1.95828 25.1685C2.4238 26.2924 3.31672 27.1853 4.4406 27.6509C5.28351 28 6.35209 28 8.48924 28Z`
const fileTypeFoldPath = `M26.3909 7.37445L19.0525 0V3.77445C19.0525 4.89271 19.0525 5.45184 19.2352 5.89289C19.4788 6.48096 19.946 6.94818 20.5341 7.19176C20.9751 7.37445 21.5342 7.37445 22.6525 7.37445H26.3909Z`
const fileTypeMarkdownPath = `M8.7588 19.5V14.6H9.8998L11.9298 17.932H11.3278L13.3018 14.6H14.4428L14.4568 19.5H13.1828L13.1688 16.539H13.3858L11.9088 19.017H11.2928L9.7738 16.539H10.0398V19.5H8.7588ZM15.4375 19.5V14.6H17.7545C18.2958 14.6 18.7718 14.7003 19.1825 14.901C19.5932 15.1017 19.9128 15.384 20.1415 15.748C20.3748 16.112 20.4915 16.546 20.4915 17.05C20.4915 17.5493 20.3748 17.9833 20.1415 18.352C19.9128 18.716 19.5932 18.9983 19.1825 19.199C18.7718 19.3997 18.2958 19.5 17.7545 19.5H15.4375ZM16.8235 18.394H17.6985C17.9785 18.394 18.2212 18.3427 18.4265 18.24C18.6365 18.1327 18.7998 17.9787 18.9165 17.778C19.0332 17.5727 19.0915 17.33 19.0915 17.05C19.0915 16.7653 19.0332 16.5227 18.9165 16.322C18.7998 16.1213 18.6365 15.9697 18.4265 15.867C18.2212 15.7597 17.9785 15.706 17.6985 15.706H16.8235V18.394Z`

// FileTypeMarkdown 构造 FileTypeIcon(kind="markdown") 的三层原件。
// markTransform 逐字执行官方 mu="translate(14 16) scale(1.22) translate(-14 -16)"。
func FileTypeMarkdown(base, mark gfx.Color) (gfx.Icon, bool) {
	basePath, ok := ParsePath(fileTypeBasePath)
	if !ok {
		return gfx.Icon{}, false
	}
	foldPath, ok := ParsePath(fileTypeFoldPath)
	if !ok {
		return gfx.Icon{}, false
	}
	markPath, ok := ParsePath(fileTypeMarkdownPath)
	if !ok {
		return gfx.Icon{}, false
	}
	foldColor := mark
	foldColor.Alpha *= 0.7
	for i := range markPath {
		if markPath[i].Kind == gfx.PathCommandClose {
			continue
		}
		markPath[i].Point = scaleFileTypeMark(markPath[i].Point)
		markPath[i].Control1 = scaleFileTypeMark(markPath[i].Control1)
		markPath[i].Control2 = scaleFileTypeMark(markPath[i].Control2)
	}
	return gfx.Icon{ViewBox: 28, Layers: []gfx.IconLayer{
		{Fill: base, Commands: basePath},
		{Fill: foldColor, Commands: foldPath},
		{Fill: mark, Commands: markPath},
	}}, true
}

func scaleFileTypeMark(point gfx.Point) gfx.Point {
	// translate(14 16) scale(1.22) translate(-14 -16)
	return gfx.Point{X: 14 + (point.X-14)*1.22, Y: 16 + (point.Y-16)*1.22}
}
