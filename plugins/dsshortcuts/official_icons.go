// official_icons.go —— 本面板用到的官方素材，**逐字节取自官方**，不自画。
//
// 关闭叉（16×16 sprite `close-outline`，Regular 描边）：
//
//	素材   assets/official/icons/settings/close-outline.jsx:1-2（两条 path，d 原样）
//	用点   dsh-client-ui-shortcuts/src/client/Reference.tsx:165
//	       `<IconCloseOutlineRegular size={14} />`（头部关闭按钮）
//	       Reference.module.css:80-95 `.close{width:28px;height:28px;padding:6px}`
//	       —— 显示 14、容器 28、padding 6，三个数各归各位，别互相套。
//
// viewBox 一律用官方原值 16；显示尺寸由调用点按官方 `size` 传。
package dsshortcuts

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// 官方 sprite 的两条 path（d 属性原样，未改坐标）。
const (
	officialClosePathA = `M2.5 2.5L13.5 13.5`
	officialClosePathB = `M13.5 2.5L2.5 13.5`

	// officialSpriteViewBox 是 16×16 sprite 的 viewBox。
	officialSpriteViewBox float32 = 16
	// officialRegularStroke 是官方 Regular 档描边宽度（Medium 才是 1.3）。
	officialRegularStroke float32 = 1
)

// closeOutlineIcon 是官方 `IconCloseOutlineRegular`。
func closeOutlineIcon() gfx.Icon {
	icon := gfx.Icon{ViewBox: officialSpriteViewBox, Stroke: officialRegularStroke}
	for _, d := range []string{officialClosePathA, officialClosePathB} {
		commands, ok := dsassets.ParsePath(d)
		if !ok {
			continue
		}
		icon.Commands = append(icon.Commands, commands...)
	}
	return icon
}
