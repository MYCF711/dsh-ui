package uikernel

// shell_skeleton.go —— 首屏 shell 的**结构骨架**（rc.2 官方结构树 §2 的落地）。
//
// 它做三件事：
//
//	① 求几何：SolveColumns 是官方 `computeColumns` 的逐行移植
//	   （ui-layout/src/client/columns.ts:55-64），含 rc.2 的新常量
//	   SIDEBAR_DEFAULT=280 / SIDEBAR_DRAWER_BREAKPOINT=768 /
//	   SIDEBAR_AUTO_COLLAPSE=1024 / centeredSidebar 半边落点。
//	② 建结构：BuildFirstScreen 按官方节点名建出一棵**可断言的节点树**
//	   （AppFrame.frame → sidebarCol / centerCol / rightbarCol / bottomRow /
//	   overlayLayer / leadingSeat → SidebarRoot / ConversationRoot / InputBar /
//	   SidebarRight），每个节点带官方槽名与这一帧的矩形。
//	③ 画骨架：DrawFirstScreen 用 flex 真的把这棵树摆出来，并把每个官方槽
//	   dispatch 给内核（槽里有没有占用方由插件决定，空槽不占位）。
//
// 与旧实现（plugins/dsshell）的关系：
//
//	dsshell 的三栏求解是同一份官方算法的旧版，但它把「自造 12px 栏内边距、
//	自造拖拽 180..400、bottom 兜底 24」带进了产品（结构树 §7 记为「多造」）。
//	本骨架按官方裸列 + 官方槽名重建；旧内容插件（dssidebar/dstopbar/…）还没
//	迁到官方槽位，所以提供 `BridgeLegacy` 兼容桥：桥模式下用旧槽名把内容画
//	进相同容器，并沿用旧 12px 内边距（official_geometry.go 的三行 deviation）。
//
// 三件事不做（留给后续任务）：
//
//	transcript 渲染 / 设置表单 / 插件面板内容；骨架只负责「容器存在、
//	几何正确、槽位可挂」。

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

// Columns 是官方 computeColumns 的结果（像素）。
type Columns struct {
	Sidebar  float32
	Center   float32
	Rightbar float32
}

// SolveColumns 是官方 `computeColumns` 的逐行移植（ui-layout/src/client/columns.ts:55-64）：
//
//	s = sidebar == 0 ? collapsedWidth
//	               : centeredSidebar ? viewport/2
//	               : clampWidth(sidebar, SIDEBAR_MIN, SIDEBAR_MAX)
//	available = viewport - s - CENTER_MIN
//	r = rightbar == 0 || available < RIGHTBAR_MIN
//	    ? 0
//	    : min(available, clampWidth(rightbar, RIGHTBAR_MIN, viewport * RIGHTBAR_MAX_RATIO))
//	center = max(0, viewport - s - r)
//
// 右栏「放不下就整列消失」是官方的硬规则：宁可让右栏轨道为 0，也不把中栏
// 压到 CENTER_MIN 以下（只有右栏整个不出现时才允许中栏继续变窄）。
func SolveColumns(viewport, sidebar, rightbar, collapsedWidth float32, centeredSidebar bool) Columns {
	if viewport <= 0 {
		return Columns{}
	}
	var s float32
	switch {
	case sidebar <= 0:
		s = collapsedWidth
	case centeredSidebar:
		s = viewport / 2
	default:
		s = clampWidthOff(sidebar, SidebarMin, SidebarMax)
	}
	available := viewport - s - CenterMin
	var r float32
	if rightbar > 0 && available >= RightbarMin {
		r = minFloat32(available, clampWidthOff(rightbar, RightbarMin, viewport*RightbarMaxRatio))
	}
	center := viewport - s - r
	if center < 0 {
		center = 0
	}
	return Columns{Sidebar: s, Center: center, Rightbar: r}
}

// clampWidthOff 对照官方 `clampWidth`：min(max(round(px), min), max)。
// 取整是官方原样（Math.round），三列之和因此能与视口严格相等。
func clampWidthOff(px, lo, hi float32) float32 {
	if px < lo {
		px = lo
	}
	if px > hi {
		px = hi
	}
	return roundOff(px)
}

func roundOff(v float32) float32 {
	if v < 0 {
		return float32(-int32(-v + 0.5))
	}
	return float32(int32(v + 0.5))
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// ShellPhase 是官方 ConversationRoot 的三值 phase（contract/snapshot.ts:26-34）。
// 骨架只区分 hero（空白首屏）与其余（有会话）；settling/active 的内容由后续任务画。
type ShellPhase string

const (
	PhaseHero     ShellPhase = "hero"
	PhaseSettling ShellPhase = "settling"
	PhaseActive   ShellPhase = "active"
)

// ShellOptions 是一帧的输入（全部来自壳的真实状态，不许骨架自己猜）。
type ShellOptions struct {
	// Windows 决定是否扣 40px 标题栏（W0：只由 L0 扣一次）。
	Windows bool
	// Tabs 表示当前有 view tab（>1 个 view）；只有这时 header 才是 76。
	Tabs bool
	// Blank 表示无会话标题的空白 header（40px 行）。
	Blank bool
	// Phase 是官方三值 phase；hero 时主区画 Hero 结构而不是 transcript。
	Phase ShellPhase
	// SidebarPreferred 是左栏宽度偏好；<=0 表示「没有占用方」（用 CollapsedWidth）。
	SidebarPreferred float32
	// CollapsedWidth 是收起后的轨道宽（Web 默认 56，Windows/drawer 0）。
	CollapsedWidth float32
	// CenteredSidebar 打开时左栏落在框架中点（官方 centeredSidebar 参数）。
	CenteredSidebar bool
	// RightbarOn 表示右栏这一帧是否出现。
	RightbarOn bool
	// RightbarPreferred 是右栏宽度偏好；<=0 时用官方首开比例 45%。
	RightbarPreferred float32
	// BottomHeight 是 shell.bottom 占用方声明的高度；0 = 空内容不预留。
	BottomHeight float32
	// PanellistRows 是左栏导航行数（0 = 还没插件注册，整块不占高）。
	PanellistRows int
	// BridgeLegacy 打开旧插件兼容桥（见文件头）。
	BridgeLegacy bool
	// BridgeLegacyComposer 是**中栏 composer 那一段**的兼容桥开关。
	//
	// 为什么要单独一条：composer 是第一个迁到官方座位的部件（dscomposer 同时
	// 注册了 `main.composer`（旧三栏）与 `conversation.composer`（官方 chain）），
	// 而中栏其余部件（dstopbar 的 header、dsconversation 的 transcript）今天仍走
	// 旧槽。整列一个开关的话，composer 永远只能跟着旧路径走——官方座位就永远
	// 不会被 dispatch（GP 报告 §5 第 1 条）。
	//
	// 判据由壳给出（`cmd/app/shell.go` 的 shellOptionsFor）：
	//
	//	官方 composer 座位有占用方 → false（走官方座位，旧槽这一帧不 dispatch）
	//	官方座位没人、旧槽有人 → true（纯旧插件集：退回 main.composer）
	//
	// 两个座位**同一帧只有一个活**，所以不会双画。
	BridgeLegacyComposer bool
}

// ShellLayout 是一帧的几何结果；节点树与绘制都从这里取数，避免两处各算一遍。
type ShellLayout struct {
	Options ShellOptions
	Columns Columns

	Frame     ui.Rect // 整窗
	Caption   float32 // Windows 标题栏高度（只扣一次）
	Content   ui.Rect // 扣掉标题栏后的内容区
	BodyRow   ui.Rect // 三列所在行（扣掉 bottomRow 之后）
	BottomRow ui.Rect // 横跨三列的底部整行

	SidebarCol  ui.Rect
	CenterCol   ui.Rect
	RightbarCol ui.Rect

	HeaderH      float32
	Header       ui.Rect
	TitleRow     ui.Rect
	TabsRow      ui.Rect
	Body         ui.Rect
	ScrollBody   ui.Rect
	Column       ui.Rect // ChatView.column（正文宽轴）
	ComposerSeat ui.Rect
	ComposerRoot ui.Rect
	ComposerCard ui.Rect
	ComposerRow  ui.Rect
	ComposerDock ui.Rect

	SidebarRoot    ui.Rect
	SidebarLogoRow ui.Rect
	NewSession     ui.Rect
	ScrollRegion   ui.Rect
	PanelList      ui.Rect
	RegionArea     ui.Rect
	FootArea       ui.Rect

	RightPanel ui.Rect

	HeroFrame        ui.Rect
	HeroStack        ui.Rect
	HeroHeadline     ui.Rect
	HeroWorkspaceRow ui.Rect
	HeroCard         ui.Rect

	Overlay     ui.Rect
	LeadingSeat ui.Rect
}

// ComputeShellLayout 是纯函数：输入视口与壳状态，输出这一帧的结构几何。
// 没有任何跨帧状态，resize 之后不会沿用上一帧的列宽。
func ComputeShellLayout(viewportW, viewportH float32, opts ShellOptions) ShellLayout {
	var out ShellLayout
	out.Options = opts
	out.Frame = ui.Rect{X: 0, Y: 0, Width: viewportW, Height: viewportH}
	if viewportW < 0 {
		viewportW = 0
	}
	if viewportH < 0 {
		viewportH = 0
	}
	if opts.Windows {
		out.Caption = CaptionHeight
	}
	if out.Caption > viewportH {
		out.Caption = viewportH
	}
	out.Content = ui.Rect{Y: out.Caption, Width: viewportW, Height: viewportH - out.Caption}

	bottom := opts.BottomHeight
	if bottom < 0 {
		bottom = 0
	}
	if bottom > out.Content.Height {
		bottom = out.Content.Height
	}
	out.BodyRow = ui.Rect{Y: out.Caption, Width: viewportW, Height: out.Content.Height - bottom}
	out.BottomRow = ui.Rect{Y: out.Caption + out.BodyRow.Height, Width: viewportW, Height: bottom}

	var sidebarPref, rightbarPref float32
	if opts.SidebarPreferred > 0 {
		sidebarPref = opts.SidebarPreferred
	}
	if opts.RightbarOn {
		rightbarPref = opts.RightbarPreferred
		if rightbarPref <= 0 {
			rightbarPref = out.BodyRow.Width * RightbarDefaultRatio
		}
	}
	out.Columns = SolveColumns(out.BodyRow.Width, sidebarPref, rightbarPref, opts.CollapsedWidth, opts.CenteredSidebar)
	out.SidebarCol = ui.Rect{X: 0, Y: out.BodyRow.Y, Width: out.Columns.Sidebar, Height: out.BodyRow.Height}
	out.CenterCol = ui.Rect{X: out.Columns.Sidebar, Y: out.BodyRow.Y, Width: out.Columns.Center, Height: out.BodyRow.Height}
	out.RightbarCol = ui.Rect{X: out.Columns.Sidebar + out.Columns.Center, Y: out.BodyRow.Y, Width: out.Columns.Rightbar, Height: out.BodyRow.Height}

	// ── 左栏：SidebarRoot padding 6 / 8（rc.2），logoRow → newSession → scrollRegion → footArea
	if out.SidebarCol.Width > 0 {
		out.SidebarRoot = ui.Rect{
			X: out.SidebarCol.X + SidebarRootPadX, Y: out.SidebarCol.Y + SidebarRootPadY,
			Width: out.SidebarCol.Width - 2*SidebarRootPadX, Height: out.SidebarCol.Height - 2*SidebarRootPadY,
		}
		logoH := SidebarLogoRowHeight
		if opts.Windows {
			logoH = SidebarLogoRowHeightWin
		}
		out.SidebarLogoRow = ui.Rect{X: out.SidebarRoot.X, Y: out.SidebarRoot.Y, Width: out.SidebarRoot.Width, Height: logoH}
		out.NewSession = ui.Rect{
			X: out.SidebarRoot.X + SidebarNewSessionMargin, Y: out.SidebarLogoRow.Y + out.SidebarLogoRow.Height,
			Width: out.SidebarRoot.Width - 2*SidebarNewSessionMargin, Height: SidebarNewSessionHeight,
		}
		scrollY := out.NewSession.Y + out.NewSession.Height + SidebarScrollRegionTop
		scrollH := out.SidebarRoot.Y + out.SidebarRoot.Height - scrollY
		if scrollH < 0 {
			scrollH = 0
		}
		out.ScrollRegion = ui.Rect{X: out.SidebarRoot.X, Y: scrollY, Width: out.SidebarRoot.Width, Height: scrollH}
		listH := float32(0)
		if opts.PanellistRows > 0 {
			listH = float32(opts.PanellistRows)*SidebarPanelRowMinH + float32(opts.PanellistRows-1)*SidebarPanelListGap + SidebarPanelListBottom
		}
		if listH > out.ScrollRegion.Height {
			listH = out.ScrollRegion.Height
		}
		// 官方 scrollArea 的网格行：8px / panelList / 2px / regionArea。
		out.PanelList = ui.Rect{X: out.SidebarRoot.X, Y: out.ScrollRegion.Y + 8, Width: out.ScrollRegion.Width, Height: listH}
		regionY := out.PanelList.Y + out.PanelList.Height + 2
		regionH := out.ScrollRegion.Y + out.ScrollRegion.Height - regionY
		if regionH < 0 {
			regionH = 0
		}
		out.RegionArea = ui.Rect{X: out.SidebarRoot.X, Y: regionY, Width: out.ScrollRegion.Width, Height: regionH}
		out.FootArea = ui.Rect{X: out.SidebarRoot.X, Y: out.SidebarRoot.Y + out.SidebarRoot.Height, Width: out.SidebarRoot.Width}
	}

	// ── 中栏：header → body(scrollBody + composerSeat)
	center := out.CenterCol
	switch {
	case opts.Blank:
		out.HeaderH = HeaderBlankHeight
	case opts.Tabs:
		out.HeaderH = HeaderMinHeightTabs
	default:
		out.HeaderH = HeaderPadTop + TitleRowMinHeight + HeaderPadBottomNone
	}
	if out.HeaderH > center.Height {
		out.HeaderH = center.Height
	}
	out.Header = ui.Rect{X: center.X, Y: center.Y, Width: center.Width, Height: out.HeaderH}
	out.TitleRow = ui.Rect{
		X: out.Header.X + HeaderPadLeft, Y: out.Header.Y + HeaderPadTop,
		Width: maxFloat32(out.Header.Width-HeaderPadLeft-HeaderPadRight, 0), Height: TitleRowMinHeight,
	}
	if opts.Tabs {
		out.TabsRow = ui.Rect{
			X: out.TitleRow.X + TabsPaddingLeft, Y: out.TitleRow.Y + out.TitleRow.Height + TabsMarginTop,
			Width: maxFloat32(out.TitleRow.Width-TabsPaddingLeft, 0), Height: ViewTabsMinH,
		}
	}
	out.Body = ui.Rect{X: center.X, Y: center.Y + out.HeaderH, Width: center.Width, Height: center.Height - out.HeaderH}

	composerH := ComposerSeatHeight
	if composerH > out.Body.Height {
		composerH = out.Body.Height
	}
	out.ComposerSeat = ui.Rect{X: out.Body.X, Y: out.Body.Y + out.Body.Height - composerH, Width: out.Body.Width, Height: composerH}
	out.ScrollBody = ui.Rect{X: out.Body.X, Y: out.Body.Y, Width: out.Body.Width, Height: out.Body.Height - composerH}
	if out.ScrollBody.Height < 0 {
		out.ScrollBody.Height = 0
	}
	// 正文宽轴：clamp(680, 主列宽×.64, 920)，滚动区两侧 16 + clearance 16。
	innerW := out.ScrollBody.Width - BodyScrollMarginRight - 2*(ChatScrollPad+ComposerSideClearance)
	if innerW < 0 {
		innerW = 0
	}
	colW := contentWidth(out.Body.Width)
	if colW > innerW {
		colW = innerW
	}
	out.Column = ui.Rect{
		X: out.ScrollBody.X + ChatScrollPad + ComposerSideClearance + (innerW-colW)/2,
		Y: out.ScrollBody.Y + ChatScrollPad, Width: colW, Height: maxFloat32(out.ScrollBody.Height-2*ChatScrollPad, 0),
	}
	out.ComposerRoot = ui.Rect{
		X: out.ComposerSeat.X + ComposerSideClearance, Y: out.ComposerSeat.Y,
		Width: maxFloat32(out.ComposerSeat.Width-2*ComposerSideClearance, 0), Height: out.ComposerSeat.Height,
	}
	cardW := composerCardWidth(out.Body.Width)
	if cardW > out.ComposerRoot.Width {
		cardW = out.ComposerRoot.Width
	}
	cardH := ComposerCardPadTop + ComposerInputMinH + ComposerCardGap + ComposerRowHeight
	out.ComposerCard = ui.Rect{
		X: out.ComposerRoot.X + (out.ComposerRoot.Width-cardW)/2, Y: out.ComposerRoot.Y,
		Width: cardW, Height: cardH,
	}
	out.ComposerRow = ui.Rect{X: out.ComposerCard.X, Y: out.ComposerCard.Y + ComposerCardPadTop + ComposerInputMinH + ComposerCardGap, Width: cardW, Height: ComposerRowHeight}
	out.ComposerDock = ui.Rect{
		X: out.ComposerRoot.X, Y: out.ComposerCard.Y + out.ComposerCard.Height,
		Width: out.ComposerRoot.Width, Height: maxFloat32(out.ComposerRoot.Y+out.ComposerRoot.Height-ComposerPadBottom-(out.ComposerCard.Y+out.ComposerCard.Height), 0),
	}

	// ── Hero：官方 stack 居中，宽度上限 = composer 卡片宽。
	if opts.Phase == PhaseHero {
		out.HeroFrame = out.Body
		stackW := composerCardWidth(out.Body.Width)
		if limit := out.Body.Width - 2*HeroPadX; stackW > limit {
			stackW = limit
		}
		if stackW < 0 {
			stackW = 0
		}
		heroCardH := ComposerCardPadTop + ComposerInputHeroH + ComposerCardGap + ComposerRowHeight
		stackH := HeroHeadlineLineH + HeroStackGap + HeroWorkspaceMinH + HeroStackGap + heroCardH
		stackX := out.HeroFrame.X + HeroPadX + (maxFloat32(out.HeroFrame.Width-2*HeroPadX, 0)-stackW)/2
		stackY := out.HeroFrame.Y + (out.HeroFrame.Height-stackH)/2
		if stackY < out.HeroFrame.Y {
			stackY = out.HeroFrame.Y
		}
		out.HeroStack = ui.Rect{X: stackX, Y: stackY, Width: stackW, Height: stackH}
		out.HeroHeadline = ui.Rect{X: stackX, Y: stackY, Width: stackW, Height: HeroHeadlineLineH}
		out.HeroWorkspaceRow = ui.Rect{X: stackX, Y: styleAdd(stackY, HeroHeadlineLineH+HeroStackGap), Width: stackW, Height: HeroWorkspaceMinH}
		out.HeroCard = ui.Rect{X: stackX, Y: styleAdd(out.HeroWorkspaceRow.Y, HeroWorkspaceMinH+HeroStackGap), Width: stackW, Height: heroCardH}
	}

	// ── 右栏：面板绝对锚定框架右缘，占满整列。
	out.RightPanel = out.RightbarCol
	out.Overlay = out.Frame
	seatLeft := LeadingSeatLeft
	if opts.SidebarPreferred <= 0 && opts.CollapsedWidth <= 0 {
		seatLeft = LeadingSeatLeftD
	}
	out.LeadingSeat = ui.Rect{X: seatLeft, Y: LeadingSeatTop}
	return out
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func styleAdd(a, b float32) float32 { return a + b }

// ── 节点树 ──────────────────────────────────────────────────────────

// SkeletonNode 是首屏骨架里的一个节点。
type SkeletonNode struct {
	// Name 是官方节点名（`AppFrame.frame` / `ConversationRoot.header` …），也是
	// SlotSpec.Host 引用它的键。
	Name string
	// Slot 是挂在这个节点上的官方槽（"" = 纯容器）。
	Slot pluginapi.Slot
	// Parent 是父节点名（根为 ""）。
	Parent string
	// Rect 是这一帧的矩形；零宽/零高 = 结构存在但当前没有内容（官方空槽不占位）。
	Rect ui.Rect
}

// Visible 表示这个节点这一帧是否真的占面积（零面积 = 声明存在、不占位）。
func (n SkeletonNode) Visible() bool { return n.Rect.Width > 0 && n.Rect.Height > 0 }

// SkeletonTree 是一帧的骨架节点树。
type SkeletonTree struct {
	layout ShellLayout
	nodes  []SkeletonNode
	index  map[string]int
}

// Nodes 返回节点（父节点在子节点之前）。
func (t *SkeletonTree) Nodes() []SkeletonNode { return t.nodes }

// Layout 返回这一帧的几何。
func (t *SkeletonTree) Layout() ShellLayout { return t.layout }

// Node 按名字取节点。
func (t *SkeletonTree) Node(name string) (SkeletonNode, bool) {
	if i, ok := t.index[name]; ok {
		return t.nodes[i], true
	}
	return SkeletonNode{}, false
}

// NodesForSlot 返回挂着某个官方槽的节点（正常应恰好一个）。
func (t *SkeletonTree) NodesForSlot(slot pluginapi.Slot) []SkeletonNode {
	var out []SkeletonNode
	for _, n := range t.nodes {
		if n.Slot == slot {
			out = append(out, n)
		}
	}
	return out
}

// BuildFirstScreen 按官方结构树建出首屏节点树（纯结构，不画）。
//
// 节点顺序 = 绘制顺序 = 官方 DOM 顺序：先左栏（sidebarCol → SidebarRoot 各带），
// 再中栏（centerCol → ConversationRoot header/body → ChatView → InputBar），
// 再右栏，最后 bottomRow / overlayLayer / leadingSeat。
func BuildFirstScreen(layout ShellLayout) *SkeletonTree {
	t := &SkeletonTree{layout: layout, index: map[string]int{}}
	add := func(name string, parent string, slot pluginapi.Slot, rect ui.Rect) {
		if _, dup := t.index[name]; dup {
			panic("uikernel: 骨架节点重复: " + name)
		}
		t.index[name] = len(t.nodes)
		t.nodes = append(t.nodes, SkeletonNode{Name: name, Parent: parent, Slot: slot, Rect: rect})
	}

	// 窗口框架
	add("AppFrame.frame", "", OffRoot, layout.Frame)
	if layout.Options.Windows {
		add("AppFrame.titlebar", "AppFrame.frame", "", ui.Rect{Width: layout.Frame.Width, Height: layout.Caption})
	}
	add("AppFrame.sidebarCol", "AppFrame.frame", OffSidebar, layout.SidebarCol)
	add("AppFrame.centerCol", "AppFrame.frame", "", layout.CenterCol)
	add("AppFrame.mainPanel", "AppFrame.centerCol", OffMain, layout.CenterCol)
	add("AppFrame.rightbarCol", "AppFrame.frame", pluginapi.SlotRightbar, layout.RightbarCol)
	add("AppFrame.bottomRow", "AppFrame.frame", pluginapi.SlotShellBottom, layout.BottomRow)
	add("AppFrame.overlayLayer", "AppFrame.frame", pluginapi.SlotShellOverlay, layout.Overlay)
	add("AppFrame.leadingSeat", "AppFrame.frame", pluginapi.SlotShellLeading, layout.LeadingSeat)

	// 左栏（SidebarRoot）
	add("SidebarRoot.root", "AppFrame.sidebarCol", "", layout.SidebarRoot)
	add("SidebarRoot.logoRow", "SidebarRoot.root", "", layout.SidebarLogoRow)
	markRect := ui.Rect{X: layout.SidebarLogoRow.X + SidebarLogoRowPadLeft, Y: layout.SidebarLogoRow.Y + SidebarLogoRowPadTop}
	add("SidebarRoot.logoRow.mark", "SidebarRoot.logoRow", OffSidebarBrandMark, markRect)
	add("SidebarRoot.logoRow.name", "SidebarRoot.logoRow", OffSidebarBrandName, markRect)
	add("SidebarRoot.logoRow.toggleBadge", "SidebarRoot.logoRow", OffSidebarToggleBadge, ui.Rect{X: layout.SidebarLogoRow.X + layout.SidebarLogoRow.Width, Y: layout.SidebarLogoRow.Y})
	add("SidebarRoot.newSession", "SidebarRoot.root", "", layout.NewSession)
	add("SidebarRoot.scrollRegion", "SidebarRoot.root", "", layout.ScrollRegion)
	add("SidebarRoot.panelList", "SidebarRoot.scrollRegion", OffSidebarPanelList, layout.PanelList)
	add("SidebarRoot.panelListActions", "SidebarRoot.panelList", OffSidebarPanelActions, ui.Rect{X: layout.PanelList.X, Y: layout.PanelList.Y})
	add("SidebarRoot.regionArea", "SidebarRoot.scrollRegion", OffSidebarWorkspaces, layout.RegionArea)
	add("SidebarRoot.footArea", "SidebarRoot.root", "", layout.FootArea)
	add("SidebarRoot.footArea.actions", "SidebarRoot.footArea", OffSidebarFooterActions, ui.Rect{X: layout.FootArea.X, Y: layout.FootArea.Y})
	add("SidebarRoot.footArea.settings", "SidebarRoot.footArea", OffSidebarSettings, ui.Rect{X: layout.FootArea.X, Y: layout.FootArea.Y})

	// 中栏（ConversationRoot）
	conversation := layout.CenterCol
	add("ConversationRoot.root", "AppFrame.mainPanel", OffMainConversation, conversation)
	{
		add("ConversationRoot.header", "ConversationRoot.root", OffConversationHeader, layout.Header)
		leadingRect := ui.Rect{X: layout.Header.X + HeaderPadLeft, Y: layout.TitleRow.Y, Height: TitleRowMinHeight}
		add("ConversationHeader.leading", "ConversationRoot.header", OffHeaderLeading, leadingRect)
		add("ConversationHeader.titleRow", "ConversationRoot.header", OffSessionHeader, layout.TitleRow)
		add("ConversationHeader.titleCluster", "ConversationHeader.titleRow", "", layout.TitleRow)
		add("ConversationHeader.lineage", "ConversationHeader.titleCluster", OffSessionHeaderLineage,
			ui.Rect{X: layout.TitleRow.X, Y: layout.TitleRow.Y + CrumbPadY, Height: CrumbLineH})
		crumbW := minFloat32(2*CrumbMaxWidth+CrumbsGap, layout.TitleRow.Width)
		add("ConversationHeader.crumbs", "ConversationHeader.titleCluster", "",
			ui.Rect{X: layout.TitleRow.X, Y: layout.TitleRow.Y + (TitleRowMinHeight-CrumbLineH)/2, Width: crumbW, Height: CrumbLineH})
		add("ConversationHeader.crumb", "ConversationHeader.crumbs", "",
			ui.Rect{X: layout.TitleRow.X, Y: layout.TitleRow.Y + (TitleRowMinHeight-CrumbLineH)/2, Height: CrumbLineH})
		add("ConversationHeader.actions", "ConversationHeader.titleCluster", OffSessionHeaderActions,
			ui.Rect{X: layout.TitleRow.X + layout.TitleRow.Width, Y: layout.TitleRow.Y, Height: TitleRowMinHeight})
		add("ConversationHeader.utilities", "ConversationHeader.titleRow", OffSessionHeaderUtilities,
			ui.Rect{X: layout.TitleRow.X + layout.TitleRow.Width, Y: layout.TitleRow.Y, Height: TitleRowMinHeight})
		add("ConversationHeader.corner", "ConversationHeader.titleRow", OffSessionHeaderCorner,
			ui.Rect{X: layout.TitleRow.X + layout.TitleRow.Width + HeaderCornerMarginRight, Y: layout.TitleRow.Y, Height: TitleRowMinHeight})
		add("ConversationHeader.tabs", "ConversationRoot.header", OffSessionHeaderViewAction, layout.TabsRow)
		add("ConversationHeader.viewTabs", "ConversationHeader.tabs", "",
			ui.Rect{X: layout.TabsRow.X, Y: layout.TabsRow.Y, Width: layout.TabsRow.Width, Height: ViewTabsMinH})
		add("ConversationHeader.tab", "ConversationHeader.viewTabs", "", ui.Rect{X: layout.TabsRow.X, Y: layout.TabsRow.Y, Height: TabMinHeight})
	}

	add("ConversationRoot.body", "ConversationRoot.root", OffConvContent, layout.Body)
	add("ConversationRoot.scrollBody", "ConversationRoot.body", "", layout.ScrollBody)
	add("ChatView.frame", "ConversationRoot.scrollBody", OffConversationSession, layout.ScrollBody)
	add("ChatView.scroll", "ChatView.frame", "", layout.ScrollBody)
	add("ChatView.column", "ChatView.scroll", OffConvView, layout.Column)
	add("ChatView.flow", "ChatView.column", OffUserTextFactory, layout.Column)
	add("ChatView.flow.markdown", "ChatView.flow", OffUserTextNode, ui.Rect{X: layout.Column.X, Y: layout.Column.Y})
	add("ChatView.flow.images", "ChatView.flow", OffImagesContentPlant, ui.Rect{X: layout.Column.X, Y: layout.Column.Y})

	add("ConversationRoot.composerSeat", "ConversationRoot.body", OffConvComposer, layout.ComposerSeat)
	add("ConversationRoot.composerStack", "ConversationRoot.composerSeat", "", layout.ComposerSeat)
	add("InputBar.root", "ConversationRoot.composerStack", OffConvComposerBar, layout.ComposerRoot)
	add("InputBar.card", "InputBar.root", "", layout.ComposerCard)
	add("InputBar.input", "InputBar.card", "",
		ui.Rect{X: layout.ComposerCard.X, Y: layout.ComposerCard.Y + ComposerCardPadTop, Width: layout.ComposerCard.Width, Height: ComposerInputMinH})
	add("InputBar.row", "InputBar.card", "", layout.ComposerRow)
	add("InputBar.row.left", "InputBar.row", OffInputLeft, ui.Rect{X: layout.ComposerRow.X + ComposerRowPadX, Y: layout.ComposerRow.Y + ComposerRowPadTop, Height: ComposerRowHeight - ComposerRowPadTop - ComposerRowPadBot})
	add("InputBar.row.plan", "InputBar.row", OffInputPlan, ui.Rect{X: layout.ComposerRow.X + ComposerRowPadX, Y: layout.ComposerRow.Y + ComposerRowPadTop, Height: ComposerRowHeight - ComposerRowPadTop - ComposerRowPadBot})
	add("InputBar.row.permission", "InputBar.row", OffInputPermission, ui.Rect{X: layout.ComposerRow.X + ComposerRowPadX, Y: layout.ComposerRow.Y + ComposerRowPadTop, Height: ComposerRowHeight - ComposerRowPadTop - ComposerRowPadBot})
	add("InputBar.row.right", "InputBar.row", OffInputRight,
		ui.Rect{X: layout.ComposerRow.X + layout.ComposerRow.Width - ComposerRowPadX, Y: layout.ComposerRow.Y + ComposerRowPadTop, Height: ComposerRowHeight - ComposerRowPadTop - ComposerRowPadBot})
	add("InputBar.row.model", "InputBar.row", OffInputModel,
		ui.Rect{X: layout.ComposerRow.X + layout.ComposerRow.Width - ComposerRowPadX, Y: layout.ComposerRow.Y + ComposerRowPadTop, Height: ComposerRowHeight - ComposerRowPadTop - ComposerRowPadBot})
	add("InputBar.row.activity", "InputBar.row", OffInputActivity,
		ui.Rect{X: layout.ComposerRow.X + layout.ComposerRow.Width - ComposerRowPadX, Y: layout.ComposerRow.Y + ComposerRowPadTop, Height: ComposerRowHeight - ComposerRowPadTop - ComposerRowPadBot})
	add("InputBar.attachments", "InputBar.card", OffInputAttachments,
		ui.Rect{X: layout.ComposerCard.X, Y: layout.ComposerCard.Y + ComposerCardPadTop, Width: layout.ComposerCard.Width})
	add("InputBar.overlayLayer", "InputBar.card", OffInputOverlay, layout.ComposerCard)
	add("InputBar.dockTop", "InputBar.root", OffInputDock, ui.Rect{X: layout.ComposerRoot.X, Y: layout.ComposerRoot.Y, Width: layout.ComposerRoot.Width})
	add("InputBar.dock", "InputBar.root", OffConvComposerDock, layout.ComposerDock)

	// Hero（hero 阶段才有面积；节点恒在，空槽不占位）
	{
		add("Hero.frame", "ConversationRoot.body", OffHeroFrame, layout.HeroFrame)
		add("Hero.stack", "Hero.frame", "", layout.HeroStack)
		add("Hero.brandMark", "Hero.stack", OffHeroBrandMark, ui.Rect{X: layout.HeroHeadline.X, Y: layout.HeroHeadline.Y})
		add("Hero.headline", "Hero.stack", "",
			ui.Rect{X: layout.HeroHeadline.X, Y: layout.HeroHeadline.Y, Width: layout.HeroHeadline.Width, Height: HeroHeadlineLineH})
		add("Hero.workspace", "Hero.stack", OffHeroWorkspace, layout.HeroWorkspaceRow)
		add("Hero.agentPreset", "Hero.stack", OffHeroAgentPreset, ui.Rect{X: layout.HeroWorkspaceRow.X, Y: layout.HeroWorkspaceRow.Y})
		add("Hero.options", "Hero.stack", OffHeroOptions, ui.Rect{X: layout.HeroWorkspaceRow.X + layout.HeroWorkspaceRow.Width, Y: layout.HeroWorkspaceRow.Y})
	}

	// 右栏（SidebarRight / DockKit）
	add("SidebarRight.panel", "AppFrame.rightbarCol", OffRightbarSession, layout.RightPanel)
	add("SidebarRight.tabStrip", "SidebarRight.panel", OffRightPaneTab,
		ui.Rect{X: layout.RightPanel.X, Y: layout.RightPanel.Y, Width: layout.RightPanel.Width})
	add("SidebarRight.tabStrip.title", "SidebarRight.tabStrip", OffRightPaneTabTitle,
		ui.Rect{X: layout.RightPanel.X, Y: layout.RightPanel.Y})
	add("SidebarRight.guide", "SidebarRight.panel", OffRightGuide, layout.RightPanel)
	add("SidebarRight.guide.actions", "SidebarRight.guide", OffRightGuideActions, ui.Rect{X: layout.RightPanel.X, Y: layout.RightPanel.Y})
	add("SidebarRight.guide.entry", "SidebarRight.guide", OffRightGuideEntry, ui.Rect{X: layout.RightPanel.X, Y: layout.RightPanel.Y})
	add("SidebarRight.tabMenu", "SidebarRight.tabStrip", OffRightTabMenuItem,
		ui.Rect{X: layout.RightPanel.X, Y: layout.RightPanel.Y})
	return t
}

// RectForHost 取某个槽的实现节点矩形；没登记返回零值 + false。
func (t *SkeletonTree) RectForHost(host string) (ui.Rect, bool) {
	n, ok := t.Node(host)
	return n.Rect, ok
}

// ── 绘制 ────────────────────────────────────────────────────────────

// FirstScreen 画首屏骨架，返回这一帧的布局（调用方要拿列宽去填 env.Geometry）。
//
// dispatch 由壳提供（一般是 kernel.DrawSlot 的闭包）；骨架不直接认识内核。
// 空槽也会 dispatch：正常实现里没有 painter 就什么都不画，这样「每个官方槽
// 都有一个可挂节点」与「挂上去真的会出现」是同一套代码路径。
func FirstScreen(f ui.C, env *pluginapi.Env, opts ShellOptions, dispatch func(pluginapi.Slot)) ShellLayout {
	return FirstScreenWithMeasure(f, env, opts, dispatch, nil)
}

// MeasureFunc 让调用方（测试 / 证据脚本）拿到**实际布出来**的容器矩形，
// 用来核对「布局表算的」与「flex 真画的」是同一份几何。
type MeasureFunc func(name string, box ui.Box)

// FirstScreenWithMeasure 是 FirstScreen 的可测量版本（measure 为 nil 时等价）。
func FirstScreenWithMeasure(f ui.C, env *pluginapi.Env, opts ShellOptions, dispatch func(pluginapi.Slot), measure MeasureFunc) ShellLayout {
	bounds := env.Bounds
	if env.Geometry.Width > 0 {
		bounds.Width = env.Geometry.Width
	}
	if env.Geometry.Height > 0 {
		bounds.Height = env.Geometry.Height
	}
	layout := ComputeShellLayout(bounds.Width, bounds.Height, opts)

	note := func(name string, box ui.Box) {
		if measure != nil {
			measure(name, box)
		}
	}
	has := func(slot pluginapi.Slot) bool {
		if env.Has == nil {
			return false
		}
		return env.Has(slot)
	}
	var bg, sidebarFill ui.Color
	if env.Theme != nil {
		bg, sidebarFill = env.Theme.BgBase, env.Theme.SidebarFill
	}

	root := ui.Column(f).FillWidth().FillHeight().PadSides(layout.Caption, 0, 0, 0)
	note("AppFrame.frame", root)
	if env.Theme != nil {
		root = root.Background(bg)
	}
	root.Children(func() {
		dispatch(OffRoot)
		// 三列所在行
		ui.Row(f).FillWidth().Grow(1).Children(func() {
			// 左栏
			if layout.Columns.Sidebar > 0 {
				col := ui.Column(f).Width(layout.Columns.Sidebar).Shrink(0).FillHeight()
				note("AppFrame.sidebarCol", col)
				if env.Theme != nil {
					col = col.Background(sidebarFill)
				}
				col.Children(func() {
					dispatch(OffSidebar)
					if has(OffSidebar) {
						return // 官方：注册在这里 = 整列替换
					}
					drawSidebarSkeleton(f, env, layout, dispatch, measure)
				})
			}
			// 中栏
			centerBox := ui.Column(f).Width(layout.Columns.Center).Grow(1).FillHeight()
			note("AppFrame.centerCol", centerBox)
			centerBox.Children(func() {
				if has(OffMain) {
					dispatch(OffMain) // keyed 面板占用方自画整块
					return
				}
				dispatch(OffMainConversation)
				if has(OffMainConversation) {
					return
				}
				dispatch(OffMain)
				drawConversationSkeleton(f, env, layout, dispatch, measure)
			})
			// 右栏
			if layout.Columns.Rightbar > 0 {
				rightBox := ui.Column(f).Width(layout.Columns.Rightbar).FillHeight()
				note("AppFrame.rightbarCol", rightBox)
				rightBox.Children(func() {
					dispatch(pluginapi.SlotRightbar)
					if has(pluginapi.SlotRightbar) {
						return
					}
					drawRightbarSkeleton(f, env, layout, dispatch, measure)
				})
			}
		})
		// 底部整行：占用方自己声明高度，空内容不预留。
		if layout.BottomRow.Height > 0 {
			bottomBox := ui.Column(f).FillWidth().Height(layout.BottomRow.Height)
			note("AppFrame.bottomRow", bottomBox)
			bottomBox.Children(func() {
				if env.Theme != nil {
					ui.Row(f).FillWidth().FillHeight().Background(bg).Children(func() { dispatch(pluginapi.SlotShellBottom) })
					return
				}
				dispatch(pluginapi.SlotShellBottom)
			})
		}
		// 整框浮层：绝对定位，覆盖整个框架（官方 .overlayLayer）。
		if has(pluginapi.SlotShellOverlay) {
			overlay := ui.Row(f).Absolute().Left(0).Top(0).Width(layout.Frame.Width).Height(layout.Frame.Height)
			note("AppFrame.overlayLayer", overlay)
			overlay.Children(func() { dispatch(pluginapi.SlotShellOverlay) })
		}
		// 窗口 chrome 左上座位（绝对定位，top 11 / left 88）。
		if has(pluginapi.SlotShellLeading) {
			ui.Row(f).Absolute().Left(layout.LeadingSeat.X).Top(layout.LeadingSeat.Y).Children(func() {
				dispatch(pluginapi.SlotShellLeading)
			})
		}
	})
	return layout
}

// drawSidebarSkeleton 画 SidebarRoot 的带结构（logoRow / newSession / scrollRegion / footArea）。
func drawSidebarSkeleton(f ui.C, env *pluginapi.Env, layout ShellLayout, dispatch func(pluginapi.Slot), measure MeasureFunc) {
	note := func(name string, box ui.Box) {
		if measure != nil {
			measure(name, box)
		}
	}
	if layout.Options.BridgeLegacy {
		drawLegacySidebar(f, env, layout, dispatch, measure)
		return
	}
	padX := SidebarRootPadX
	ui.Column(f).FillWidth().FillHeight().PadSides(SidebarRootPadY, padX, SidebarRootPadY, padX).Children(func() {
		// logoRow：mark / name / toggle.badge 三个单座
		ui.Row(f).FillWidth().Height(layout.SidebarLogoRow.Height).Gap(SidebarLogoRowGap).Children(func() {
			dispatch(OffSidebarBrandMark)
			dispatch(OffSidebarBrandName)
			dispatch(OffSidebarToggleBadge)
		})
		// newSession（官方 38px 圆角按钮；骨架只留结构框）
		ui.Row(f).FillWidth().Height(SidebarNewSessionHeight).MarginX(SidebarNewSessionMargin).Children(func() {})
		// scrollRegion：panelList + regionArea
		ui.Column(f).FillWidth().Grow(1).Children(func() {
			if layout.PanelList.Height > 0 {
				ui.Column(f).FillWidth().Height(layout.PanelList.Height).Gap(SidebarPanelListGap).Children(func() {
					dispatch(OffSidebarPanelList)
					dispatch(OffSidebarPanelActions)
				})
			}
			ui.Column(f).FillWidth().Grow(1).Children(func() { dispatch(OffSidebarWorkspaces) })
		})
		// footArea：footer 动作（list）+ 设置行（single）
		//
		// 设置行是官方 `SettingsRoot.triggerRow` 的**一行**：
		// `[设置按钮(flex:1)] [连接指示] [更新指示]`。容器必须是一个 Row，
		// 否则同一槽上的几个占用方会各画一行（同一列里上下堆叠）——
		// 「连接指示在设置按钮下面一行」就是这么来的（HF 报告 §剩余差距 1）。
		// Row 的子项由各插件自己画，壳不认任何具体部件。
		ui.Column(f).FillWidth().Children(func() {
			dispatch(OffSidebarFooterActions)
			settingsRow := ui.Row(f).FillWidth().
				MarginSides(SettingsTriggerRowMarginY, -SettingsTriggerRowInset,
					SettingsTriggerRowMarginY, -SettingsTriggerRowInset).
				Items(ui.Center).Gap(SettingsTriggerRowGap)
			settingsRow.Children(func() { dispatch(OffSidebarSettings) })
			// 必须在 Children 之后报：布局跑完才量得到行高（顺序反了报的是 0）。
			note("SettingsRoot.triggerRow", settingsRow)
		})
	})
}

// drawLegacySidebar 是左栏的旧插件兼容路径：dssidebar 挂在
// sidebar.header / sidebar.list / sidebar.footer 三个旧槽上，按 dsshell 时代的
// 12px 内联 padding + gap 8 排版（official_geometry.go 的 deviation 行）。
func drawLegacySidebar(f ui.C, env *pluginapi.Env, layout ShellLayout, dispatch func(pluginapi.Slot), measure MeasureFunc) {
	note := func(name string, box ui.Box) {
		if measure != nil {
			measure(name, box)
		}
	}
	ui.Column(f).FillWidth().FillHeight().
		PadSides(SidebarRootPadY, LegacySidebarPadX, SidebarRootPadY, LegacySidebarPadX).Gap(8).Children(func() {
		dispatch(pluginapi.SlotSidebarHeader)
		ui.Column(f).FillWidth().Grow(1).MinHeight(0).Children(func() { dispatch(pluginapi.SlotSidebarList) })
		if env.Has != nil && env.Has(pluginapi.SlotSidebarFooter) {
			dispatch(pluginapi.SlotSidebarFooter)
		}
		// 设置行在**两种档位下都是同一行**（官方 footArea 的顺序就是
		// 「footerActions 在上、settings 在下」）。兼容桥里 `sidebar.settings`
		// 的官方座位不画，但它的**过渡桥**（dsnotifications 的账号启动器、
		// dsstatusbar 的连接指示）都注册在这个槽上，由这一行统一摆：
		// `[启动器(flex:1)] [连接指示]`，与官方几何一致。
		//
		// 这一行必须放在 footer 之后：官方 `.footArea{flex-direction:column}`
		// 里 settingsArea 是最后一行。
		settingsRow := ui.Row(f).FillWidth().
			MarginSides(SettingsTriggerRowMarginY, -SettingsTriggerRowInset,
				SettingsTriggerRowMarginY, -SettingsTriggerRowInset).
			Items(ui.Center).Gap(SettingsTriggerRowGap)
		settingsRow.Children(func() { dispatch(OffSidebarSettings) })
		// 必须在 Children 之后报：布局跑完才量得到行高（顺序反了报的是 0）。
		note("SettingsRoot.triggerRow (legacy bridge)", settingsRow)
	})
}

// drawConversationSkeleton 画 ConversationRoot：header（titleRow + tabs）+ body（scrollBody + composerSeat）。
func drawConversationSkeleton(f ui.C, env *pluginapi.Env, layout ShellLayout, dispatch func(pluginapi.Slot), measure MeasureFunc) {
	note := func(name string, box ui.Box) {
		if measure != nil {
			measure(name, box)
		}
	}
	if layout.Options.BridgeLegacy {
		drawLegacyConversation(f, env, layout, dispatch, measure)
		return
	}
	// header
	if !layout.Options.Blank && layout.Header.Height > 0 {
		headerBox := ui.Column(f).FillWidth().Height(layout.Header.Height).
			PadSides(HeaderPadTop, HeaderPadRight, 0, HeaderPadLeft)
		note("ConversationRoot.header", headerBox)
		headerBox.Children(func() {
			dispatch(OffConversationHeader)
			// title row：leading(single) + title cluster（lineage/actions/utilities/corner）
			ui.Row(f).FillWidth().Height(TitleRowMinHeight).Children(func() {
				dispatch(OffHeaderLeading)
				ui.Row(f).FillWidth().Height(TitleRowMinHeight).Children(func() {
					dispatch(OffSessionHeader)
					dispatch(OffSessionHeaderLineage)
					dispatch(OffSessionHeaderActions)
					dispatch(OffSessionHeaderUtilities)
					dispatch(OffSessionHeaderCorner)
				})
			})
			// tabs 行（只有 tabs.length>1 才出现）
			if layout.Options.Tabs && layout.TabsRow.Height > 0 {
				ui.Row(f).FillWidth().Height(ViewTabsMinH).MarginSides(TabsMarginTop, 0, 0, TabsPaddingLeft).Children(func() {
					dispatch(OffSessionHeaderViewAction)
				})
			}
		})
	}
	// hero 阶段：主区是 Hero（官方 ConversationRoot[data-phase=hero]），
	// 没有 transcript 滚动区与 active composer seat。
	if layout.Options.Phase == PhaseHero {
		dispatch(OffConvContent)
		drawHeroSkeleton(f, env, layout, dispatch, measure)
		return
	}
	// body
	dispatch(OffConvContent)
	bodyBox := ui.Column(f).FillWidth().Grow(1)
	note("ConversationRoot.body", bodyBox)
	bodyBox.Children(func() {
		// 滚动区（transcript 容器）
		scrollBox := ui.Row(f).FillWidth().Grow(1)
		note("ConversationRoot.scrollBody", scrollBox)
		scrollBox.Children(func() {
			ui.Column(f).FillWidth().FillHeight().PadSides(ChatScrollPad, ChatScrollPad+ComposerSideClearance+BodyScrollMarginRight, ChatScrollPad, ChatScrollPad+ComposerSideClearance).Children(func() {
				ui.Column(f).Width(layout.Column.Width).FillHeight().Children(func() {
					dispatch(OffConversationSession)
					dispatch(OffConvView)
					dispatch(OffUserTextFactory)
					dispatch(OffUserTextNode)
					dispatch(OffImagesContentPlant)
				})
			})
		})
		// composer seat：官方 `conversation.composer` 是 chain 槽，占用方接管
		// 整个 composer 区（见 drawComposerSeat 的注释）。
		drawComposerSeat(f, env, layout, dispatch, measure, ComposerSideClearance)
	})
}

// drawHeroSkeleton 画空白会话的 Hero：官方 HeroShell.root 居中，
// stack 宽度上限 = composer 卡片宽，三块依次是标题（鱼标 + 标题 + 预览徽标）、
// 工作区行、composer 卡片。
func drawHeroSkeleton(f ui.C, env *pluginapi.Env, layout ShellLayout, dispatch func(pluginapi.Slot), measure MeasureFunc) {
	note := func(name string, box ui.Box) {
		if measure != nil {
			measure(name, box)
		}
	}
	heroBox := ui.Column(f).FillWidth().Grow(1).PadX(HeroPadX)
	note("Hero.frame", heroBox)
	heroBox.Items(ui.Center).Children(func() {
		stack := ui.Column(f).Width(layout.HeroStack.Width).Gap(HeroStackGap)
		note("Hero.stack", stack)
		stack.Children(func() {
			dispatch(OffHeroFrame)
			// 标题行：鱼标（brand mark）+ 标题 + 预览徽标
			ui.Row(f).FillWidth().Height(HeroHeadlineLineH).Items(ui.Center).Gap(HeroHeadlineGapCol).Children(func() {
				dispatch(OffHeroBrandMark)
			})
			// 工作区行：workspace / agentPreset / options
			ui.Row(f).FillWidth().Height(HeroWorkspaceMinH).Gap(HeroWorkspaceGap).Children(func() {
				dispatch(OffHeroWorkspace)
				dispatch(OffHeroAgentPreset)
				dispatch(OffHeroOptions)
			})
			// Hero 内嵌 composer：同一个 InputBar 卡片
			ui.Row(f).FillWidth().Height(layout.HeroCard.Height).Children(func() {
				ui.Column(f).FillWidth().FillHeight().PadSides(ComposerCardPadTop, 0, 0, 0).Gap(ComposerCardGap).Children(func() {
					dispatch(OffConvComposerBar)
					dispatch(OffInputAttachments)
					ui.Column(f).FillWidth().Height(ComposerInputHeroH).Children(func() { dispatch(OffInputOverlay) })
					ui.Row(f).FillWidth().Height(ComposerRowHeight).PadSides(ComposerRowPadTop, ComposerRowPadX, ComposerRowPadBot, ComposerRowPadX).Children(func() {
						dispatch(OffInputLeft)
						dispatch(OffInputPlan)
						dispatch(OffInputPermission)
						dispatch(OffInputRight)
						dispatch(OffInputModel)
						dispatch(OffInputActivity)
					})
				})
			})
			dispatch(OffConvComposerDock)
			dispatch(OffInputDock)
		})
	})
}

// drawComposerSkeleton 画 InputBar：附件 rail / 卡片（输入区 + 控件行）/ dock。
func drawComposerSkeleton(f ui.C, env *pluginapi.Env, layout ShellLayout, dispatch func(pluginapi.Slot), measure MeasureFunc) {
	ui.Row(f).FillWidth().Height(layout.ComposerRoot.Height).
		PadSides(0, ComposerSideClearance, ComposerPadBottom, ComposerSideClearance).Children(func() {
		ui.Column(f).FillWidth().FillHeight().Children(func() {
			dispatch(OffConvComposerBar)
			// dock：卡片上方的整行条目（官方 conversation.input.dock）
			dispatch(OffInputDock)
			// 卡片
			cardRow := ui.Row(f).FillWidth().Height(layout.ComposerCard.Height).Justify(ui.Center)
			cardRow.Children(func() {
				cardBox := ui.Column(f).Width(layout.ComposerCard.Width).FillHeight().PadSides(ComposerCardPadTop, 0, 0, 0).Gap(ComposerCardGap)
				if measure != nil {
					measure("InputBar.card", cardBox)
				}
				cardBox.Children(func() {
					dispatch(OffInputAttachments)
					// 输入区
					ui.Column(f).FillWidth().Height(ComposerInputMinH).Children(func() { dispatch(OffInputOverlay) })
					// 控件行：左组 / 右组
					ui.Row(f).FillWidth().Height(ComposerRowHeight).PadSides(ComposerRowPadTop, ComposerRowPadX, ComposerRowPadBot, ComposerRowPadX).Children(func() {
						dispatch(OffInputLeft)
						dispatch(OffInputPlan)
						dispatch(OffInputPermission)
						dispatch(OffInputRight)
						dispatch(OffInputModel)
						dispatch(OffInputActivity)
					})
				})
			})
			// dock：卡片下方环境条目
			dispatch(OffConvComposerDock)
		})
	})
}

// drawLegacyConversation 是「旧插件还没迁走」时的兼容路径：中栏按 dsshell
// 时代的 12px 内边距 + gap 8 摆 header / body 两个旧槽，composer 那一段再按
// `BridgeLegacyComposer` 二选一（官方座位 / 旧 main.composer）。
//
// 这是官方结构树 §7 记的「多造」之一（自造栏内边距），所以 official_geometry.go
// 里对应行是 deviation；等 dstopbar / dsconversation 迁到 conversation.* 官方槽位后
// 整段删除。composer 先迁走（它是第一个有官方座位占用方的部件），所以座位画在
// 12px 内边距**外面**：官方 composer 的侧边距是 16（相对正文列），不是 12+16。
func drawLegacyConversation(f ui.C, env *pluginapi.Env, layout ShellLayout, dispatch func(pluginapi.Slot), measure MeasureFunc) {
	ui.Column(f).FillWidth().FillHeight().Children(func() {
		// header + 正文：旧三栏的 12px 栏内边距（遗留桥）。
		ui.Column(f).FillWidth().Grow(1).MinHeight(0).Pad(LegacyColumnInset).Gap(8).Children(func() {
			if env.Has != nil && env.Has(pluginapi.SlotMainHeader) {
				dispatch(pluginapi.SlotMainHeader)
			}
			ui.Column(f).FillWidth().Grow(1).MinHeight(0).Children(func() { dispatch(pluginapi.SlotMainBody) })
		})
		// composer：官方座位。它画在 12px 内边距**外面**（上面那个 Grow 列才有 padding），
		// 所以侧边距直接给官方的 16——官方 `InputBar.root` 的 clearance 是相对正文列的，
		// 不是"再叠一层 12"。
		drawComposerSeat(f, env, layout, dispatch, measure, ComposerSideClearance)
	})
}

// drawComposerSeat 画 composer 座位（官方 `ConversationRoot.composerSeat`）。
//
// 官方语义（`ConversationContent.tsx:147-186`）：`conversation.composer` 是 **chain**
// 槽，`renderSlotChain(..., {fallback: composerBar, overlay: true})` ——
//
//	fallback = composerStack（InputBar 卡片 + dock）
//	overlay  = 接管项（审批卡 / 问题条），画在 fallback 之上
//
// 所以**有占用方就由它接管整个 composer 区**：骨架不再画一份 InputBar（否则卡片
// 双画、命中区重叠）。没有占用方时才画骨架（空壳结构，给没有 composer 插件的
// 宿主/单测用）。
//
// 高度：没有占用方时用几何表的静止高度（`ComposerSeatHeight` = 卡片 98 + dock 26
// + 底边距 4 = 128）；有占用方时**不钉死**——InputBar.root 的高度由卡片 + dock 的
// 内容决定，钉死等于把同一份算术写两遍，字号/行高一变就漂。
func drawComposerSeat(f ui.C, env *pluginapi.Env, layout ShellLayout, dispatch func(pluginapi.Slot), measure MeasureFunc, sidePad float32) {
	// 纯旧插件集：官方座位没人、旧槽有人 → 走兼容桥（与官方座位二选一）。
	if layout.Options.BridgeLegacyComposer {
		dispatch(pluginapi.SlotMainComposer)
		return
	}
	if sidePad < 0 {
		sidePad = 0
	}
	occupied := env.Has != nil && env.Has(OffConvComposer)
	// 空壳：座位按几何表的静止高度，**自己**画官方 InputBar 骨架（含它自己的
	// 侧边距与底边距），所以这里不再补 padding。
	if !occupied {
		if layout.ComposerSeat.Height <= 0 {
			return
		}
		seat := ui.Column(f).FillWidth().Height(layout.ComposerSeat.Height)
		if measure != nil {
			measure("ConversationRoot.composerSeat", seat)
		}
		seat.Children(func() {
			dispatch(OffConvComposer)
			drawComposerSkeleton(f, env, layout, dispatch, measure)
		})
		return
	}
	// 有占用方：高度内容驱动；侧边距由座位给（官方 InputBar.root 的左右
	// clearance），底边距 4 归 InputBar.root 自己（`padding:0 16 4`）。
	seat := ui.Column(f).FillWidth().PadX(sidePad)
	if measure != nil {
		measure("ConversationRoot.composerSeat", seat)
	}
	seat.Children(func() { dispatch(OffConvComposer) })
}

// drawRightbarSkeleton 画右栏默认组合：面板 → tab strip / guide / tab menu。
func drawRightbarSkeleton(f ui.C, env *pluginapi.Env, layout ShellLayout, dispatch func(pluginapi.Slot), measure MeasureFunc) {
	ui.Column(f).FillWidth().FillHeight().Children(func() {
		dispatch(OffRightbarSession)
		ui.Row(f).FillWidth().Children(func() { dispatch(OffRightPaneTab) })
		ui.Row(f).FillWidth().Children(func() { dispatch(OffRightPaneTabTitle) })
		ui.Row(f).FillWidth().Grow(1).Children(func() {
			dispatch(OffRightGuide)
			dispatch(OffRightGuideActions)
			dispatch(OffRightGuideEntry)
		})
		ui.Row(f).FillWidth().Children(func() { dispatch(OffRightTabMenuItem) })
	})
}
