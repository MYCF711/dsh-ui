package dsassets

import "github.com/vivaui/viva/gfx"

// 侧栏与工作区图标。路径逐字来自 assets/official/icons/{sidebar,workspace}/ 的
// 官方 JSX；rc.2 新增的两枚复合图标来自 dsh-web-frontend/dist/assets/
// index-ghOpC9uV.js:129 的 sprite（sha256 061de8ce…f8bb8）。这些素材不作为
// 新 ID 加入 All()：与 ChevronDownOutline 一样，它们是按需接线的未登记素材，
// 不改变既有 golden 的 15 条基线。

func buildSidebarIcon(stroke float32, fill gfx.Color, elements ...Element) (gfx.Icon, bool) {
	return Build(Asset{ViewBox: 16, Size: 16, StrokeWidth: stroke, Elements: elements}, fill)
}

// SidebarNewChatMedium 是 SidebarRoot.tsx:393 的 IconNewChatOutlineMedium。
// 三条 path 与 assets/official/icons/sidebar/new-chat.jsx:1-3 相同，Medium 仅改 stroke-width=1.3。
func SidebarNewChatMedium(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1.3, fill,
		Element{Kind: PathElement, D: `M2.37091 11.2501C1.58745 9.89288 1.32067 8.29835 1.61969 6.76006C1.91872 5.22177 2.76342 3.8433 3.99826 2.87846C5.2331 1.91362 6.77494 1.42737 8.33988 1.50925C9.90482 1.59113 11.3875 2.23562 12.5149 3.32406C13.6425 4.41269 14.3387 5.87206 14.4754 7.4334C14.612 8.99474 14.18 10.5529 13.2587 11.8209C12.3375 13.0888 10.9891 13.9813 9.46194 14.3337C8.18691 14.628 6.85895 14.5294 5.64989 14.0605C5.1712 13.8748 4.76962 13.4932 4.26534 13.3967C3.67413 13.2835 2.95257 13.5598 2.03794 14.3337`, Stroke: true},
		Element{Kind: PathElement, D: `M8 5V11`, Stroke: true},
		Element{Kind: PathElement, D: `M5 8H11`, Stroke: true},
	)
}

// SidebarPluginPinwheel 是 PluginsPanelIcon.tsx:14 的 IconPluginPinwheelOutlineRegular。
// 官方片段：assets/official/icons/sidebar/plugin-pinwheel.jsx:1-4。
func SidebarPluginPinwheel(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M7.84457 5.06199C11.6605 4.93876 14.7962 6.14848 14.8484 7.76397C14.8875 8.97461 13.1838 10.0696 10.7215 10.5942`, Stroke: true},
		Element{Kind: PathElement, D: `M5.12742 8.07731C5.00419 4.26138 6.21391 1.12568 7.8294 1.07351C9.04004 1.03441 10.135 2.73808 10.6596 5.20037`, Stroke: true},
		Element{Kind: PathElement, D: `M8.02457 10.6802C4.20865 10.8034 1.07294 9.5937 1.02077 7.97821C0.981678 6.76758 2.68535 5.67262 5.14763 5.14798`, Stroke: true},
		Element{Kind: PathElement, D: `M10.7476 7.89535C10.8708 11.7113 9.66109 14.847 8.0456 14.8991C6.83496 14.9382 5.74 13.2346 5.21536 10.7723`, Stroke: true},
	)
}

// SidebarClock 是 TaskManagerIcon.tsx:15 的 IconClockOutlineRegular。
// 官方片段：assets/official/icons/sidebar/clock.jsx:1-2。
func SidebarClock(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M8 14C11.3137 14 14 11.3137 14 8C14 4.68629 11.3137 2 8 2C4.68629 2 2 4.68629 2 8C2 11.3137 4.68629 14 8 14Z`, Stroke: true},
		Element{Kind: PathElement, D: `M8 4.31V8.46L11 10.08`, Stroke: true},
	)
}

// WorkspaceSearch 是 WorkspaceBrowser.tsx:1248 的 IconSearchOutlineRegular。
// 官方片段：assets/official/icons/workspace/search.jsx:1-2。
func WorkspaceSearch(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M6.58727 11.8586C9.55061 11.8586 11.9529 9.45637 11.9529 6.49304C11.9529 3.5297 9.55061 1.12744 6.58727 1.12744C3.62394 1.12744 1.22168 3.5297 1.22168 6.49304C1.22168 9.45637 3.62394 11.8586 6.58727 11.8586Z`, Stroke: true},
		Element{Kind: PathElement, D: `M10.2991 10.3933L14.7783 14.8725`, Stroke: true},
	)
}

// WorkspaceProjectAdd 是 WorkspaceBrowser.tsx:1310 的 IconProjectAddOutlineRegular。
// 官方片段：assets/official/icons/workspace/project-add.jsx:1-3（首条填充、后两条描边）。
func WorkspaceProjectAdd(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M5.54492 2.06738C5.91034 2.06754 6.26318 2.20149 6.53711 2.44336L7.94043 3.68164V4.7998C7.71462 4.74105 7.50367 4.63139 7.32617 4.47461L5.87598 3.19238C5.78477 3.11185 5.66658 3.06754 5.54492 3.06738H2.94922C2.67322 3.06738 2.44946 3.29145 2.44922 3.56738V12.4326C2.44927 12.7087 2.67311 12.9326 2.94922 12.9326H12.9326C13.2086 12.9325 13.4326 12.7086 13.4326 12.4326V8.53613H14.4326V12.4326C14.4326 13.2609 13.7609 13.9325 12.9326 13.9326H2.94922C2.12083 13.9326 1.44927 13.261 1.44922 12.4326V3.56738C1.44946 2.73916 2.12094 2.06738 2.94922 2.06738H5.54492Z`, Fill: true},
		Element{Kind: PathElement, D: `M9.75977 4.50208H14.5509`, Stroke: true},
		Element{Kind: PathElement, D: `M12.1492 6.89758L12.1492 2.10642`, Stroke: true},
	)
}

// WorkspaceFolder 是 Rows.tsx:314-318 的 folder-close/folder-open。
// 官方片段：assets/official/icons/workspace/folder-{close,open}.jsx。
func WorkspaceFolder(fill gfx.Color, open bool) (gfx.Icon, bool) {
	if !open {
		return buildSidebarIcon(1, fill,
			Element{Kind: PathElement, D: `M1.50439 3.11059C1.50439 2.55831 1.95211 2.1106 2.50439 2.1106H5.43389C5.67773 2.1106 5.91318 2.19969 6.09593 2.36113L7.71649 3.79265C7.89924 3.95409 8.1347 4.04319 8.3785 4.04319H13.4958C14.0481 4.04319 14.4958 4.4909 14.4958 5.04319V12.8894C14.4958 13.4417 14.0481 13.8894 13.4958 13.8894H2.50439C1.95211 13.8894 1.50439 13.4417 1.50439 12.8894V4.04319V3.11059Z`, Stroke: true},
			Element{Kind: PathElement, D: `M3.63501 7.66614H12.3647`, Stroke: true},
		)
	}
	faint := fill
	faint.Alpha *= 0.16 // folder-open.jsx:1 的 opacity="0.16"
	paths := []string{
		`M2.55912 7.93683C2.67584 7.49906 3.0723 7.19446 3.52536 7.19446H13.6491C14.3061 7.19446 14.7846 7.81725 14.6153 8.45209L13.4411 12.856C13.3244 13.2938 12.9279 13.5984 12.4748 13.5984H2.35113C1.69411 13.5984 1.21562 12.9756 1.38489 12.3407L2.55912 7.93683Z`,
		`M13.6491 6.69446C14.6346 6.69453 15.3522 7.62895 15.0983 8.58118L13.9245 12.9845C13.7494 13.6412 13.1539 14.0988 12.4743 14.0988H2.35126C1.36574 14.0988 0.648153 13.1643 0.902044 12.212L2.07587 7.80774C2.25102 7.15128 2.84567 6.69455 3.52509 6.69446H13.6491ZM3.52509 7.69446C3.29865 7.69455 3.10004 7.84674 3.04169 8.06555L1.86786 12.4698C1.78345 12.7872 2.02285 13.0988 2.35126 13.0988H12.4743C12.7007 13.0988 12.8992 12.9463 12.9577 12.7277L14.1325 8.32336C14.2171 8.00598 13.9776 7.69453 13.6491 7.69446H3.52509Z`,
		`M4.7666 1.90137C5.13227 1.90144 5.48571 2.03525 5.75977 2.27734L7.27246 3.61328C7.36379 3.69382 7.48174 3.73828 7.60352 3.73828H12.3994C13.2276 3.73841 13.8993 4.41005 13.8994 5.23828V6.7168C13.8183 6.70327 13.735 6.69436 13.6494 6.69434H12.8994V5.23828C12.8993 4.96233 12.6754 4.73841 12.3994 4.73828H7.60352C7.23781 4.73828 6.88446 4.60438 6.61035 4.3623L5.09766 3.02637C5.00636 2.94576 4.88838 2.90144 4.7666 2.90137H2.0498C1.77366 2.90137 1.5498 3.12523 1.5498 3.40137V9.78223L0.902344 12.2119C0.648452 13.1642 1.36604 14.0986 2.35156 14.0986H2.0498C1.2214 14.0986 0.549838 13.427 0.549805 12.5986V3.40137C0.549805 2.57294 1.22138 1.90137 2.0498 1.90137H4.7666Z`,
	}
	icon := gfx.Icon{ViewBox: 16}
	for i, d := range paths {
		commands, ok := ParsePath(d)
		if !ok {
			return gfx.Icon{}, false
		}
		color := fill
		if i == 0 {
			color = faint
		}
		icon.Layers = append(icon.Layers, gfx.IconLayer{Fill: color, Commands: commands})
	}
	return icon, true
}

// WorkspaceCloseFill 是 WorkspaceBrowser.tsx:1277 的 IconCloseFillRegular。
// rc.2 sprite close-fill（index-ghOpC9uV.js:129，字节偏移 674573）。
func WorkspaceCloseFill(fill gfx.Color) (gfx.Icon, bool) {
	return buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M3.5 3.5L12.5 12.5`, Stroke: true},
		Element{Kind: PathElement, D: `M12.5 3.5L3.5 12.5`, Stroke: true},
	)
}

// OfficialRing / OfficialRoundedRect 是复合 SVG 里不能塞进 gfx.Icon 的精确图元。
// 调用方用 Viva 的圆/圆角矩形绘制，不用贝塞尔近似冒充。
type OfficialRing struct {
	CX, CY, Radius, Stroke float32
}

type OfficialRoundedRect struct {
	X, Y, Width, Height, Radius, Stroke float32
}

// WorkspaceSlidersTwo 是 WorkspaceBrowser.tsx:198 的 IconSlidersTwoOutlineRegular。
// rc.2 sprite sliders-two-outline（index-ghOpC9uV.js:129，字节偏移 733979）。
func WorkspaceSlidersTwo(fill gfx.Color) (gfx.Icon, []OfficialRing, bool) {
	icon, ok := buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M2.3 5h5.85M12.05 5h1.65`, Stroke: true, Cap: gfx.CapRound, CapSet: true},
		Element{Kind: PathElement, D: `M2.3 11h1.65M7.85 11h5.85`, Stroke: true, Cap: gfx.CapRound, CapSet: true},
	)
	return icon, []OfficialRing{{CX: 9.95, CY: 5, Radius: 1.45, Stroke: 1}, {CX: 5.75, CY: 11, Radius: 1.45, Stroke: 1}}, ok
}

// SidebarAppsAdd 是 More 行的 IconAppsAddOutlineRegular。
// rc.2 sprite apps-add-outline（index-ghOpC9uV.js:129，紧随 sliders-two-outline）。
func SidebarAppsAdd(fill gfx.Color) (gfx.Icon, []OfficialRoundedRect, bool) {
	icon, ok := buildSidebarIcon(1, fill,
		Element{Kind: PathElement, D: `M11.75 1.75v5M9.25 4.25h5`, Stroke: true},
	)
	rects := []OfficialRoundedRect{
		{X: 2.25, Y: 2.25, Width: 4, Height: 4, Radius: 1, Stroke: 1},
		{X: 2.25, Y: 9.75, Width: 4, Height: 4, Radius: 1, Stroke: 1},
		{X: 9.75, Y: 9.75, Width: 4, Height: 4, Radius: 1, Stroke: 1},
	}
	return icon, rects, ok
}
