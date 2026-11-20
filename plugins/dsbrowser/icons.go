// icons.go —— 右栏浏览器工具条的官方图标（路径原文来自官方打包产物）。
//
// # 出处不是"照着截图画"，是官方产物里的原文
//
// 官方这六枚图标都从 `@deepseek-ai/dsh-client-ui-primitives` 导出
// （`OFF:dsh-client-ui-sidebar-browser/src/client/view/BrowserBody.tsx:3-10`
// 与 `src/client/view/BrowserTitle.tsx:3`），而 primitives 的 `src/**`
// **不随包发**（`ls dsh-client-ui-primitives` 只有 lib）。所以图元原文取自
// web 前端的打包产物：
//
//	OFF:dsh-web-frontend/dist/assets/index-ghOpC9uV.js
//	  "chevron-left-outline":{size:14,…}   ← IconChevronLeftOutlineRegular
//	  "chevron-right-outline":{size:14,…}  ← IconChevronRightOutlineRegular
//	  "refresh-outline":{size:16,…}        ← IconRefreshOutlineRegular
//	  "link-outline":{size:16,…}           ← IconLinkOutlineRegular
//	  "right-up-outline":{size:16,…}       ← IconRightUpOutlineRegular
//	  "globe-outline":{size:14,…}          ← IconGlobeOutlineRegular
//
// 同一份 bundle 里 `ICON_REGULAR_STROKE` 的取值是 `RR`、`RR=1`，与本仓
// `plugins/dsassets/registry.go:101` 的 `regularStroke = 1` 是同一个数，所以
// 这里把 `d` 原文交给 dsassets 的路径编译器（`dsassets.Build`），不重新描坐标。
//
// # 为什么放插件里，不进 dsassets 的登记表
//
// `dsassets.All()` 的条数与 `docs/official-assets.md` 的表逐条对账
// （`plugins/dsassets/golden_test.go` 守着）：往里加六条就得同步改那张文档表，
// 而这六枚只服务本插件。出处照样逐条写在下面，可回查。
package dsbrowser

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// 六枚图标的 `d` 原文（官方 bundle 逐字，未改一个字符）。
const (
	// pathChevronLeft 是官方 IconChevronLeftOutlineRegular 的单条描边路径。
	pathChevronLeft = "M10 4L6.70711 7.29289C6.31658 7.68342 6.31658 8.31658 6.70711 8.70711L10 12"
	// pathChevronRight 是官方 IconChevronRightOutlineRegular。
	pathChevronRight = "M6 12L9.29289 8.70711C9.68342 8.31658 9.68342 7.68342 9.29289 7.29289L6 4"
	// pathRefresh 是官方 IconRefreshOutlineRegular 的主体（圆环缺口）。
	pathRefresh = "M14.5001 8C14.5 9.28552 14.1188 10.5422 13.4045 11.611C12.6903 12.6799 11.6752 13.5129 " +
		"10.4875 14.0049C9.29982 14.4968 7.99295 14.6255 6.73212 14.3747C5.4713 14.124 4.31314 13.505 " +
		"3.4041 12.596C2.49514 11.687 1.87614 10.5288 1.62537 9.26798C1.37459 8.00716 1.50331 6.70028 " +
		"1.99525 5.51261C2.48719 4.32494 3.32025 3.30981 4.3891 2.59557C5.45795 1.88134 6.71458 1.50008 " +
		"8.0001 1.5C9.9001 1.5 11.7001 2.3 13.0001 3.6L14.5001 5.1"
	// pathRefreshHead 是官方 IconRefreshOutlineRegular 的箭头（第二条 path）。
	pathRefreshHead = "M14.4999 1.5V5.1H10.8999"
	// pathLinkA 是官方 IconLinkOutlineRegular 的第一段链环。
	pathLinkA = "M6.59961 9.40051C6.82779 9.6334 7.10015 9.81842 7.40074 9.94472C7.70132 10.071 8.02409 10.1361 " +
		"8.35013 10.1361C8.67618 10.1361 8.99894 10.071 9.29953 9.94472C9.60011 9.81842 9.87247 9.6334 " +
		"10.1007 9.40051L12.9015 6.59967C13.3658 6.13541 13.6266 5.50572 13.6266 4.84915C13.6266 4.19258 " +
		"13.3658 3.56289 12.9015 3.09863C12.4372 2.63436 11.8075 2.37354 11.151 2.37354C10.4944 2.37354 " +
		"9.86472 2.63436 9.40045 3.09863L9.05034 3.44873"
	// pathLinkB 是官方 IconLinkOutlineRegular 的第二段链环。
	pathLinkB = "M9.40051 6.59959C9.17233 6.3667 8.89997 6.18169 8.59939 6.05538C8.2988 5.92907 7.97603 5.86401 " +
		"7.64999 5.86401C7.32395 5.86401 7.00118 5.92907 6.70059 6.05538C6.40001 6.18169 6.12765 6.3667 " +
		"5.89946 6.59959L3.09863 9.40043C2.63436 9.8647 2.37354 10.4944 2.37354 11.151C2.37354 11.8075 " +
		"2.63436 12.4372 3.09863 12.9015C3.56289 13.3657 4.19258 13.6266 4.84915 13.6266C5.50572 13.6266 " +
		"6.13541 13.3657 6.59967 12.9015L6.94978 12.5514"
	// pathRightUpArrow 是官方 IconRightUpOutlineRegular 的填充箭头头部。
	pathRightUpArrow = "M11.7256 2.77441C12.5538 2.77469 13.2256 3.44616 13.2256 4.27441V10.1416H12.2256V4.27441" +
		"C12.2256 3.99844 12.0015 3.77469 11.7256 3.77441H5.7207V2.77441H11.7256Z"
	// pathRightUpStem 是官方 IconRightUpOutlineRegular 的斜线。
	pathRightUpStem = "M2.77441 13.2255L12.3756 3.62427"
	// pathGlobeRing 是官方 IconGlobeOutlineRegular 的外圆。
	pathGlobeRing = "M7.99986 14.0887C11.3626 14.0887 14.0886 11.3627 14.0886 7.99998C14.0886 4.63727 " +
		"11.3626 1.91125 7.99986 1.91125C4.63715 1.91125 1.91113 4.63727 1.91113 7.99998C1.91113 11.3627 " +
		"4.63715 14.0887 7.99986 14.0887Z"
	// pathGlobeEquator 是官方 IconGlobeOutlineRegular 的赤道线（带 stroke-linecap="square"）。
	// **逐字未改**，是全文件两份派生几何的唯一来源：
	//
	//	pathGlobeEquatorSquareCap  两端各外推半描边（芯片：整枚 butt）
	pathGlobeEquator = "M2.34619 8H13.6538"
	// pathGlobeEquatorSquareCap 是赤道那一笔的**方帽等价几何**（芯片用）。
	//
	// SVG 1.1 F.11（stroke-linecap="square"）：方帽把描边沿端点处路径方向**正向**
	// 延长半个描边宽。本图描边宽 1（`ICON_REGULAR_STROKE`，见 `globeStrokeWidth`），
	// 所以两端各外推 0.5 个 viewBox 单位：
	//
	//	左端 2.34619 - 0.5 = 1.84619
	//	右端 13.6538 + 0.5 = 14.1538
	//
	// 这是**精确等价**（butt 端头 + 外推的半截描边 = square 端头），不是近似、
	// 不是手绘：`icons_globe_test.go` 用 `globeSquareCapExtend` 逐端钉住这份推导。
	pathGlobeEquatorSquareCap = "M1.84619 8H14.1538"
	// pathGlobeMeridianWest 是官方 IconGlobeOutlineRegular 的西经线。
	pathGlobeMeridianWest = "M7.99976 14.0889C9.23509 14.0889 10.1743 11.3629 10.1743 8.00006C10.1743 4.63739 " +
		"9.23509 1.91138 7.99976 1.91138"
	// pathGlobeMeridianEast 是官方 IconGlobeOutlineRegular 的东经线。
	pathGlobeMeridianEast = "M7.99973 14.0889C6.76445 14.0889 5.8252 11.3629 5.8252 8.00006C5.8252 4.63739 " +
		"6.76445 1.91138 7.99973 1.91138"
)

// 官方组件的默认显示尺寸（bundle 里每条图标的 `size`）。
// 外部打开是例外：BrowserBody.tsx:101 显式传 size={14}，不是默认的 16。
const (
	iconSizeChevron  float32 = 14
	iconSizeTool     float32 = 16
	iconSizeExternal float32 = 14
	iconSizeTitle    float32 = 14
)

// 地球图标的描边宽度与"半描边"（方帽等价几何的外推量）。
const (
	// globeStrokeWidth 是官方 Regular 档描边宽：bundle 里 `ICON_REGULAR_STROKE = 1`
	// （与 plugins/dsassets/registry.go 的 regularStroke 同一个数）。
	globeStrokeWidth float32 = 1
	// globeSquareCapExtend 是 square 端头比 butt 端头多出来的长度 = strokeWidth/2
	// （SVG 1.1 F.11 "square ... extend by half the stroke width"）。
	//
	// 两处都要把它落成墨迹：芯片那一枚单图标只有一个 Cap，所以**两端**都写进坐标
	// （pathGlobeEquatorSquareCap）；标题行那一枚直接用官方原文的 pathGlobeEquator，
	// 两端各由 `gfx.CapSquare` 真画 —— 上游合并 vivaui/viva#358 之后两端都会长，
	// 原来「只写远端」的绕法已删。
	globeSquareCapExtend = globeStrokeWidth / 2
)

// 出处：六枚图标同包同文件，只有符号不同。
func iconSource(symbol string) dsassets.Source {
	return dsassets.Source{
		Package: "@deepseek-ai/dsh-client-ui-primitives",
		File:    "dsh-web-frontend/dist/assets/index-ghOpC9uV.js",
		Symbol:  symbol,
	}
}

// strokeAsset 造一枚纯描边素材（官方正则档描边 = 1）。
func strokeAsset(symbol string, size float32, paths ...string) dsassets.Asset {
	elements := make([]dsassets.Element, 0, len(paths))
	for _, d := range paths {
		elements = append(elements, dsassets.Element{Kind: dsassets.PathElement, D: d, Stroke: true})
	}
	return dsassets.Asset{
		ID: symbol, Title: symbol, Source: iconSource(symbol),
		ViewBox: 16, Size: size, StrokeWidth: 1, Elements: elements,
	}
}

// iconChevronLeft 是官方 IconChevronLeftOutlineRegular（工具条「后退」）。
func iconChevronLeft(color gfx.Color) gfx.Icon {
	return buildIcon(strokeAsset("IconChevronLeftOutlineRegular", iconSizeChevron, pathChevronLeft), color)
}

// iconChevronRight 是官方 IconChevronRightOutlineRegular（工具条「前进」）。
func iconChevronRight(color gfx.Color) gfx.Icon {
	return buildIcon(strokeAsset("IconChevronRightOutlineRegular", iconSizeChevron, pathChevronRight), color)
}

// iconRefresh 是官方 IconRefreshOutlineRegular（工具条「刷新」）。
func iconRefresh(color gfx.Color) gfx.Icon {
	return buildIcon(strokeAsset("IconRefreshOutlineRegular", iconSizeTool, pathRefresh, pathRefreshHead), color)
}

// iconLink 是官方 IconLinkOutlineRegular（地址框里的「前往」）。
func iconLink(color gfx.Color) gfx.Icon {
	return buildIcon(strokeAsset("IconLinkOutlineRegular", iconSizeTool, pathLinkA, pathLinkB), color)
}

// iconRightUp 是官方 IconRightUpOutlineRegular（「在系统浏览器中打开」）。
//
// 它是一枚**填充 + 描边混用**的图标：箭头头部是 `fill="currentColor"`，
// 斜杆是 stroke。`dsassets.Build` 对混用的处理是"描边走 Commands、填充分层进
// Layers"，正是这里要的。
func iconRightUp(color gfx.Color) gfx.Icon {
	return buildIcon(dsassets.Asset{
		ID: "IconRightUpOutlineRegular", Title: "IconRightUpOutlineRegular",
		Source:  iconSource("IconRightUpOutlineRegular"),
		ViewBox: 16, Size: iconSizeExternal, StrokeWidth: 1,
		Elements: []dsassets.Element{
			{Kind: dsassets.PathElement, D: pathRightUpArrow, Fill: true},
			{Kind: dsassets.PathElement, D: pathRightUpStem, Stroke: true},
		},
	}, color)
}

// iconGlobe 是**tab 芯片**用的官方 IconGlobeOutlineRegular。
//
// 赤道线在官方是 `stroke-linecap="square"`，其余三条没写（= butt）。芯片只拿到
// 一枚 `gfx.Icon`（`pluginapi.Painter.IconImage`，由内核
// `internal/uikernel/view_tabs.go` 用 `ui.Icon` 画），而 viva 一枚 icon 只有一个
// `Cap`（OFF:viva@v0.6.5/gfx/icon.go:29-31）——叠不了第二枚。
//
// 所以这一枚把赤道换成**方帽等价几何** `pathGlobeEquatorSquareCap`
// （两端各外推 `globeSquareCapExtend` = 0.5，推导见该常量的注释），整枚图标保持
// butt：外推后的 butt 端头落下的墨迹与官方 square 端头逐像素相同。外圆是闭合路径
// （`Z`，没有端头），两条经线仍是官方原文与 butt。
//
// 标题行不走这一枚：那边用 `ui.Stack` 叠两枚真图标（iconGlobeOutline +
// iconGlobeEquator）。两处必须视觉一致，`icons_globe_test.go` 钉住。
func iconGlobe(color gfx.Color) gfx.Icon {
	return buildIcon(strokeAsset("IconGlobeOutlineRegular", iconSizeTitle,
		pathGlobeRing, pathGlobeEquatorSquareCap, pathGlobeMeridianWest, pathGlobeMeridianEast), color)
}

// iconGlobeOutline 是标题行地球的"外圆 + 两条经线"那一枚（官方未写 linecap = butt）。
//
// 它与 iconGlobeEquator 在 `ui.Stack` 里同槽叠起来，就是官方那一枚 svg：
//
//	OFF:dsh-client-ui-sidebar-browser/src/client/view/BrowserTitle.tsx:3
//	  <IconGlobeOutlineRegular size={14} />
//
// 三条路径都是**官方原文**（pathGlobeRing 闭合、两条经线 open，Cap 取零值 butt）。
func iconGlobeOutline(color gfx.Color) gfx.Icon {
	return buildIcon(strokeAsset("IconGlobeOutlineRegular", iconSizeTitle,
		pathGlobeRing, pathGlobeMeridianWest, pathGlobeMeridianEast), color)
}

// iconGlobeEquator 是标题行地球的**赤道那一笔**，`Cap = gfx.CapSquare`。
//
// 几何就是官方原文的 `pathGlobeEquator`，两端各由 `gfx.CapSquare` 外推 0.5，
// 合起来正好是官方的墨迹 [1.84619, 14.1538]。
//
// 2026-11-20：上游已合并我们对 square cap 的修复（vivaui/viva#358）——
// `gfx/path.go` 的末端不再传 `negate(outLast)`，两端都长。所以原来那套
// 「远端写进坐标、起点交给 CapSquare」的绕法不再需要，已换回官方原文路径。
//
// 为什么要在 Build 之后手写 Cap：`dsassets.Build` 的 Cap 映射只认 Round / Butt
// （plugins/dsassets/icon.go:44-49），`CapSquare` 没有分支，声明了也会被当成
// "没写"。`gfx.Icon.Cap` 是导出字段（OFF:viva@v0.6.5/gfx/icon.go:30），
// 所以在这里补上，不需要改 dsassets（也不该为一个用例改素材编译器）。
func iconGlobeEquator(color gfx.Color) gfx.Icon {
	icon := buildIcon(strokeAsset("IconGlobeOutlineRegular", iconSizeTitle, pathGlobeEquator), color)
	icon.Cap = gfx.CapSquare
	return icon
}

// buildIcon 把素材编译成 gfx.Icon；编不出来时返回零值（不画），
// 绝不拿近似形状顶上（dsassets 的规矩）。
func buildIcon(asset dsassets.Asset, color gfx.Color) gfx.Icon {
	icon, ok := dsassets.Build(asset, color)
	if !ok {
		return gfx.Icon{}
	}
	return icon
}
