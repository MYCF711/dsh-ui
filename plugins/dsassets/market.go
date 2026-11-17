package dsassets

import "github.com/vivaui/viva/gfx"

// 市场面板的官方素材。前四组路径逐字来自 rc.2 web 前端
// dsh-web-frontend/dist/assets/index-ghOpC9uV.js:129 的 sprite 表；
// PluginArtworkDefault 来自同一文件的 zK 组件。它们与 ChevronDownOutline 一样
// 按需接线，不加入 All() 的既有 15 条 golden 基线。

func buildMarketStroke(fill gfx.Color, elements ...Element) (gfx.Icon, bool) {
	return Build(Asset{ViewBox: 16, Size: 16, StrokeWidth: regularStroke, Elements: elements}, fill)
}

// MarketRefreshOutline 是 IconRefreshOutlineRegular（DiscoverSection.tsx:542，size=14）。
func MarketRefreshOutline(fill gfx.Color) (gfx.Icon, bool) {
	return buildMarketStroke(fill,
		Element{Kind: PathElement, D: `M14.5001 8C14.5 9.28552 14.1188 10.5422 13.4045 11.611C12.6903 12.6799 11.6752 13.5129 10.4875 14.0049C9.29982 14.4968 7.99295 14.6255 6.73212 14.3747C5.4713 14.124 4.31314 13.505 3.4041 12.596C2.49514 11.687 1.87614 10.5288 1.62537 9.26798C1.37459 8.00716 1.50331 6.70028 1.99525 5.51261C2.48719 4.32494 3.32025 3.30981 4.3891 2.59557C5.45795 1.88134 6.71458 1.50008 8.0001 1.5C9.9001 1.5 11.7001 2.3 13.0001 3.6L14.5001 5.1`, Stroke: true},
		Element{Kind: PathElement, D: `M14.4999 1.5V5.1H10.8999`, Stroke: true},
	)
}

// MarketWarningOutline 是 IconWarningOutlineRegular（DiscoverSection.tsx:256,349，size=14）。
func MarketWarningOutline(fill gfx.Color) (gfx.Icon, bool) {
	return buildMarketStroke(fill,
		Element{Kind: PathElement, D: `M8 14.5C11.5899 14.5 14.5 11.5899 14.5 8C14.5 4.41015 11.5899 1.5 8 1.5C4.41015 1.5 1.5 4.41015 1.5 8C1.5 11.5899 4.41015 14.5 8 14.5Z`, Stroke: true},
		Element{Kind: PathElement, D: `M8 4.29199V9.79199`, Stroke: true},
		Element{Kind: PathElement, D: `M8 10.708V11.708`, Stroke: true},
	)
}

// MarketTrashOutline 是 IconTrashOutlineRegular（DiscoverSection.tsx:331，size=13）。
func MarketTrashOutline(fill gfx.Color) (gfx.Icon, bool) {
	return buildMarketStroke(fill,
		Element{Kind: PathElement, D: `M1.28149 3.88831H14.7187`, Stroke: true},
		Element{Kind: PathElement, D: `M5.41602 3.88833V2.47962C5.41602 2.29282 5.52492 2.11366 5.71876 1.98157C5.9126 1.84948 6.17551 1.77527 6.44964 1.77527H9.55053C9.82466 1.77527 10.0876 1.84948 10.2814 1.98157C10.4753 2.11366 10.5842 2.29282 10.5842 2.47962V3.88833`, Stroke: true},
		Element{Kind: PathElement, D: `M2.57349 3.88831L3.19366 13.2943C3.21937 13.5502 3.33952 13.7872 3.53065 13.9593C3.72178 14.1313 3.97016 14.2259 4.22729 14.2246H11.7728C12.0299 14.2259 12.2783 14.1313 12.4694 13.9593C12.6605 13.7872 12.7807 13.5502 12.8064 13.2943L13.4266 3.88831`, Stroke: true},
		Element{Kind: PathElement, D: `M6.44946 6.98926V11.1238`, Stroke: true},
		Element{Kind: PathElement, D: `M9.55054 6.98926V11.1238`, Stroke: true},
	)
}

// MarketPlusOutline 是 IconPlusOutlineRegular（DiscoverSection.tsx:342,618，size=13）。
func MarketPlusOutline(fill gfx.Color) (gfx.Icon, bool) {
	return buildMarketStroke(fill,
		Element{Kind: PathElement, D: `M8 2V14`, Stroke: true},
		Element{Kind: PathElement, D: `M2 8H14`, Stroke: true},
	)
}

// MarketPluginArtworkDefault 是 PluginArtworkDefault 的 36×36 viewBox、原始 path 与
// userSpaceOnUse 线性渐变（#54ECE7 → #658EFF）。卡片与详情在 48px 框内画 36px，
// 空态直接画 48px（DiscoverSection.tsx:27-28,47,615）。
func MarketPluginArtworkDefault() (gfx.Icon, bool) {
	commands, ok := ParsePath(`M24.6294 8.63696C26.2862 8.63705 27.6294 9.98016 27.6294 11.637V12.7825C27.6292 14.4391 26.2861 15.7824 24.6294 15.7825H23.4839C22.6519 15.7825 21.5454 16.3459 21.5454 17.1778V18.1573C21.5454 18.7757 22.216 19.1966 22.8345 19.1965H24.6294C26.2861 19.1965 27.6292 20.5398 27.6294 22.1965V23.9924C27.6292 25.6491 26.2861 26.9924 24.6294 26.9924H22.8345C21.1778 26.9924 19.8347 25.649 19.8345 23.9924V22.1965C19.8345 21.7148 19.4957 21.2327 19.014 21.2327H16.4792C15.7975 21.2327 15.3374 22.0061 15.3374 22.6877V23.8333C15.3373 25.49 13.9942 26.8333 12.3374 26.8333H11.1919C9.53509 26.8333 8.19198 25.49 8.19189 23.8333V22.6877C8.19189 21.0309 9.53504 19.6877 11.1919 19.6877H12.3374C13.1964 19.6877 14.3999 19.0959 14.3999 18.2369V16.0185C14.3999 14.9518 15.2646 14.0872 16.3312 14.0872H19.435C20.0596 14.0872 20.484 13.4071 20.4839 12.7825V11.637C20.4839 9.98011 21.827 8.63696 23.4839 8.63696H24.6294ZM13.4116 11.2468C13.4116 12.6882 12.2431 13.8567 10.8018 13.8567C9.36037 13.8567 8.19189 12.6882 8.19189 11.2468C8.19189 9.80544 9.36037 8.63696 10.8018 8.63696C12.2431 8.63696 13.4116 9.80544 13.4116 11.2468Z`)
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{
		ViewBox: 36,
		Layers: []gfx.IconLayer{{
			Fill:     gfx.RGB24(0x54ece7),
			Commands: commands,
			Gradient: &gfx.LinearGradient{
				Start: gfx.Point{X: 15.1481, Y: 9.94188},
				End:   gfx.Point{X: 15.1481, Y: 13.6375},
				From:  gfx.RGB24(0x54ece7),
				To:    gfx.RGB24(0x658eff),
			},
		}},
	}, true
}
