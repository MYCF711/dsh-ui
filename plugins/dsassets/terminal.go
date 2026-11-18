package dsassets

import "github.com/vivaui/viva/gfx"

// 终端素材。两条 path 逐字来自 rc.2 web 前端的 `PluginArtworkTerminal`
// （`dsh-web-frontend/dist/assets/index-ghOpC9uV.js` 的 `xK` 组件）：
//
//	<svg width={size} height={size} viewBox="0 0 36 36" fill="none">
//	  <path d="M10 11L16.606 17.606C16.6841 17.6841 16.6841 17.8107 16.606 17.8888L10 24.4948"
//	        stroke="#679EFE" stroke-width="3.5"/>
//	  <path d="M20.1211 24.4946H26.8685" stroke="#679EFE" stroke-width="3.5"/>
//	</svg>
//
// 与 `MarketPluginArtworkDefault` 一样按需接线，不加入 `All()` 的 golden 基线。
// 官方这枚是**固定色**（不是 currentColor），所以调用方要用同一个 #679EFE 着色；
// 本包只导出 path/viewBox/stroke，颜色由使用处按官方值给。
func PluginArtworkTerminal() (gfx.Icon, bool) {
	left, ok := ParsePath(`M10 11L16.606 17.606C16.6841 17.6841 16.6841 17.8107 16.606 17.8888L10 24.4948`)
	if !ok {
		return gfx.Icon{}, false
	}
	right, ok := ParsePath(`M20.1211 24.4946H26.8685`)
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 36, Stroke: 3.5, Commands: append(left, right...)}, true
}

// TerminalArtworkStroke 是官方 `PluginArtworkTerminal` 的笔画色 `#679EFE`
// （`stroke="#679EFE"`，见上）。
func TerminalArtworkStroke() gfx.Color { return gfx.RGB24(0x679efe) }
