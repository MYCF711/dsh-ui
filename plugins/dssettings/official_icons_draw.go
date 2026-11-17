// official_icons_draw.go —— 设置面官方 sprite 的 Go/Viva 投影。
//
// 路径来源（均为官方原样的 d 属性，不是手绘近似）：
//
//	assets/official/icons/settings/{settings,close-outline}.jsx
//	assets/official/icons/composer/{model-data,chevron-down}.jsx
//	assets/official/icons/tools/agent-preset.jsx
//	assets/official/icons/workspace/search.jsx
//	dsh-web-frontend/dist/assets/index-ghOpC9uV.js 的 user/personalization/
//	light/dark/followsystem-outline sprite（rc.2）。
//
// Medium 使用官方 1.3 描边，Regular 使用 1；显示尺寸由调用点按 TSX 给定。
package dssettings

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	officialRegularStroke float32 = 1
	officialMediumStroke  float32 = 1.3
)

type officialIconPart struct {
	d    string
	fill bool
}

// officialVectorIcon 只逐命令转写官方 path；fill 层使用调用点的 currentColor。
func officialVectorIcon(stroke float32, tint gfx.Color, parts ...officialIconPart) gfx.Icon {
	icon := gfx.Icon{ViewBox: 16, Stroke: stroke}
	for _, part := range parts {
		commands, ok := dsassets.ParsePath(part.d)
		if !ok {
			continue
		}
		if part.fill {
			icon.Layers = append(icon.Layers, gfx.IconLayer{Fill: tint, Commands: commands})
		} else {
			icon.Commands = append(icon.Commands, commands...)
		}
	}
	return icon
}

const (
	officialUserHeadPath = `M8 8.25C9.51878 8.25 10.75 7.01878 10.75 5.5C10.75 3.98122 9.51878 2.75 8 2.75C6.48122 2.75 5.25 3.98122 5.25 5.5C5.25 7.01878 6.48122 8.25 8 8.25Z`
	officialUserBodyPath = `M2.5 14.5C2.5 11.5 5.25 10.25 8 10.25C10.75 10.25 13.5 11.5 13.5 14.5`
	officialClosePathA   = `M2.5 2.5L13.5 13.5`
	officialClosePathB   = `M13.5 2.5L2.5 13.5`
	officialSearchPathA  = `M6.58727 11.8586C9.55061 11.8586 11.9529 9.45637 11.9529 6.49304C11.9529 3.5297 9.55061 1.12744 6.58727 1.12744C3.62394 1.12744 1.22168 3.5297 1.22168 6.49304C1.22168 9.45637 3.62394 11.8586 6.58727 11.8586Z`
	officialSearchPathB  = `M10.2991 10.3933L14.7783 14.8725`
	officialLightRing    = `M8.00007 11.8117C10.1052 11.8117 11.8117 10.1052 11.8117 8.00007C11.8117 5.89499 10.1052 4.18848 8.00007 4.18848C5.89499 4.18848 4.18848 5.89499 4.18848 8.00007C4.18848 10.1052 5.89499 11.8117 8.00007 11.8117Z`
	officialDarkPath     = `M14.1127 8.70663C14.2576 8.60602 14.4627 8.71355 14.4386 8.88834C14.2901 9.96567 13.8731 10.9912 13.2229 11.8692C12.479 12.8735 11.4613 13.6421 10.2917 14.0829C9.1222 14.5236 7.85038 14.6179 6.62865 14.3543C5.40692 14.0907 4.28709 13.4805 3.40332 12.5967C2.51955 11.7129 1.90931 10.5931 1.64572 9.37135C1.38212 8.14962 1.47635 6.87779 1.91711 5.70825C2.35787 4.5387 3.12647 3.52103 4.13083 2.77714C5.00878 2.12689 6.03433 1.70994 7.11166 1.5614C7.28645 1.5373 7.39397 1.74238 7.29337 1.88734C6.68703 2.76099 6.37885 3.81241 6.42313 4.88345C6.47392 6.11194 6.98471 7.27645 7.85413 8.14587C8.72355 9.01529 9.88805 9.52608 11.1166 9.57687C12.1876 9.62114 13.239 9.31296 14.1127 8.70663Z`
	officialSystemPathA  = `M13.5 2.5H2.5C1.94772 2.5 1.5 2.94772 1.5 3.5V11.5C1.5 12.0523 1.94772 12.5 2.5 12.5H13.5C14.0523 12.5 14.5 12.0523 14.5 11.5V3.5C14.5 2.94772 14.0523 2.5 13.5 2.5Z`
	officialSystemPathB  = `M5 14.5H11`
	officialRightUpArrow = `M11.7256 2.77441C12.5538 2.77469 13.2256 3.44616 13.2256 4.27441V10.1416H12.2256V4.27441C12.2256 3.99844 12.0015 3.77469 11.7256 3.77441H5.7207V2.77441H11.7256Z`
	officialRightUpStem  = `M2.77441 13.2255L12.3756 3.62427`
	officialArchivePathA = `M13.5 2.5H2.5C1.94772 2.5 1.5 2.94772 1.5 3.5V4.5C1.5 5.05228 1.94772 5.5 2.5 5.5H13.5C14.0523 5.5 14.5 5.05228 14.5 4.5V3.5C14.5 2.94772 14.0523 2.5 13.5 2.5Z`
	officialArchivePathB = `M2.5 5.5V13.5C2.5 13.7652 2.60536 14.0196 2.79289 14.2071C2.98043 14.3946 3.23478 14.5 3.5 14.5H12.5C12.7652 14.5 13.0196 14.3946 13.2071 14.2071C13.3946 14.0196 13.5 13.7652 13.5 13.5V5.5`
	officialArchivePathC = `M6.5 9.5H9.5`
)

var officialLightRays = []string{
	`M13.3899 8H15.1499`, `M11.8115 11.8115L13.0556 13.0556`,
	`M8 13.3901V15.1501`, `M4.18868 11.8115L2.94458 13.0556`,
	`M2.6101 8H0.850098`, `M4.18868 4.18856L2.94458 2.94446`,
	`M8 2.6101V0.850098`, `M11.8115 4.18856L13.0556 2.94446`,
}

func closeOutlineIcon() gfx.Icon {
	return officialVectorIcon(officialRegularStroke, gfx.Color{},
		officialIconPart{d: officialClosePathA}, officialIconPart{d: officialClosePathB})
}

func searchOutlineIcon() gfx.Icon {
	return officialVectorIcon(officialRegularStroke, gfx.Color{},
		officialIconPart{d: officialSearchPathA}, officialIconPart{d: officialSearchPathB})
}

func chevronOutlineIcon() gfx.Icon {
	commands, _ := dsassets.ChevronDownOutline()
	return gfx.Icon{ViewBox: 16, Stroke: officialRegularStroke, Commands: commands}
}

func userOutlineIcon() gfx.Icon {
	return officialVectorIcon(officialMediumStroke, gfx.Color{},
		officialIconPart{d: officialUserHeadPath}, officialIconPart{d: officialUserBodyPath})
}

func archiveOutlineIcon() gfx.Icon {
	return officialVectorIcon(officialMediumStroke, gfx.Color{}, officialIconPart{d: officialArchivePathA},
		officialIconPart{d: officialArchivePathB}, officialIconPart{d: officialArchivePathC})
}

func rightUpOutlineIcon(tint gfx.Color) gfx.Icon {
	return officialVectorIcon(officialRegularStroke, tint,
		officialIconPart{d: officialRightUpArrow, fill: true}, officialIconPart{d: officialRightUpStem})
}

func appearanceOutlineIcon(value string) gfx.Icon {
	parts := []officialIconPart{}
	switch value {
	case "light":
		parts = append(parts, officialIconPart{d: officialLightRing})
		for _, ray := range officialLightRays {
			parts = append(parts, officialIconPart{d: ray})
		}
	case "dark":
		parts = append(parts, officialIconPart{d: officialDarkPath})
	default:
		parts = append(parts, officialIconPart{d: officialSystemPathA}, officialIconPart{d: officialSystemPathB})
	}
	return officialVectorIcon(officialMediumStroke, gfx.Color{}, parts...)
}

// drawSectionIcon 画 SettingsRoot.tsx:35-42 的 16px Medium 导航图标。
// personalization 的三条 path 与三个 circle 都来自同一官方 sprite；circle 用
// Viva 原生圆/描边表达，避免拿贝塞尔近似冒充官方圆。
func drawSectionIcon(f ui.C, name string, tint ui.Color) {
	if name == IconPersonalization {
		drawPersonalizationIcon(f, tint)
		return
	}
	var icon gfx.Icon
	switch name {
	case IconUser:
		icon = userOutlineIcon()
	case IconData:
		icon = modelDataOutlineIcon(tint)
	case IconAgentPreset:
		icon = agentPresetOutlineIcon(tint)
	case IconArchive:
		icon = archiveOutlineIcon()
	default:
		icon = settingsTriggerIcon()
		icon.Stroke = officialMediumStroke
	}
	ui.Icon(f, icon).Size(NavCellIconSize, NavCellIconSize).Color(tint)
}

func drawPersonalizationIcon(f ui.C, tint ui.Color) {
	ui.Row(f).Size(NavCellIconSize, NavCellIconSize).Children(func() {
		ui.Icon(f, personalizationOutlineIcon(tint)).Absolute().Inset(0).Color(tint)
		for _, circle := range [][2]float32{{4.97065, 7.66401}, {10.6596, 12.4091}, {10.6596, 3.59101}} {
			const radius float32 = 1.35151
			ui.Row(f).Absolute().Left(circle[0]-radius).Top(circle[1]-radius).
				Size(2*radius, 2*radius).Radius(radius).Border(officialMediumStroke, tint)
		}
	})
}
