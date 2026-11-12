package uikernel

// official_classes.go —— 首屏结构涉及到的**官方 CSS Module 类名表**（rc.2 口径）。
//
// 为什么类名也要进契约：结构相同不只是「有那个容器」，还包括**用官方那个选择器
// 语义**。rc.2 有三处真变（H2-shell-tree-reanchor.md §3），实现里必须用新形态：
//
//	① `.crumbSubagent` **整个类消失**（ConversationRoot.module.css rc.2 全文 0 命中），
//	   父子血缘只剩 `conversation.session.header.lineage` + 当前层级 `.crumb`。
//	② 右栏 pane 闪烁从 `.flashEven/.flashOdd` 两个**类**改成
//	   `[data-sidebar-right-flash='odd'/'even']` **属性选择器**
//	   （SidebarRight.module.css:261-268；注意 @keyframes 名仍叫 flashOdd/flashEven，
//	   那是动画名不是类名，别混）。
//	③ `.headerActions` 的 gap 8px → **10px**（几何表里有专门一行）。
//
// 本文件只登记「结构骨架要用/要对拍」的类，不追求官方 66 包全量；
// 未登记的类由各自的后续任务补，但**不许**把已删除的类再写回来。

// ClassSpec 是一个官方 CSS Module 类。
type ClassSpec struct {
	// Name 是去哈希后的官方类名（rc.2 的 CSS Module 已是可读类名）。
	Name string
	// Module 是官方 rc.2 模块路径（`ui-layout/src/client/AppFrame.module.css`）。
	Module string
	// Line 是 `.Name {` 所在行。
	Line int
	// Note 记这个类在结构里的角色或容易搞错的地方。
	Note string
}

// RemovedClass 是 rc.2 **没有**的类名；实现里出现即视为结构回流（回归）。
type RemovedClass struct {
	Name string
	// WasRC1 表示 rc.1 还存在（rc.2 删的）；false 表示 rc.1 就已经删了。
	WasRC1 bool
	// Evidence 是「rc.2 里没有」的取证方式。
	Evidence string
	// Replacement 是 rc.2 的替代结构。
	Replacement string
}

// SidebarRightFlashAttr 是 rc.2 右栏 pane 闪烁的**属性**（不是类）。
const (
	SidebarRightFlashAttr = "data-sidebar-right-flash"
	SidebarRightFlashOdd  = "odd"
	SidebarRightFlashEven = "even"
)

var officialClasses = []ClassSpec{
	// AppFrame（ui-layout）
	{"frame", "ui-layout/src/client/AppFrame.module.css", 1, "网格 frame：minmax(0,1fr) auto；Windows 下 padding-top 40"},
	{"sidebarCol", "ui-layout/src/client/AppFrame.module.css", 59, "左列：sidebar-fill 底 + 0.5px border-l3（Windows 无边框）"},
	{"centerCol", "ui-layout/src/client/AppFrame.module.css", 68, "中列：裸列，flex column"},
	{"mainPanel", "ui-layout/src/client/AppFrame.module.css", 77, "main keyed 槽的容器（flex:1, min-height:0）"},
	{"rightbarCol", "ui-layout/src/client/AppFrame.module.css", 264, "右列：轨道，overflow:visible，占用方面板绝对锚定右缘"},
	{"bottomRow", "ui-layout/src/client/AppFrame.module.css", 355, "横跨三列的底部整行；空内容不预留"},
	{"overlayLayer", "ui-layout/src/client/AppFrame.module.css", 363, "整框浮层：absolute inset 0, z-index 20, pointer-events:none"},
	{"leadingSeat", "ui-layout/src/client/AppFrame.module.css", 204, "窗口 chrome 左上座位：top 11 / left 88"},
	{"handle", "ui-layout/src/client/AppFrame.module.css", 220, "列宽拖拽命中带：8px、margin-left -4px"},
	{"drawerMask", "ui-layout/src/client/AppFrame.module.css", 310, "抽屉遮罩：z-index 70"},

	// ConversationRoot（ui-conversation）
	{"root", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 1, "ConversationRoot：flex column，height 100%，bg-base"},
	{"header", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 23, "grid auto minmax(0,1fr)；有 tabs 时 min-height 76"},
	{"headerBlank", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 41, "空白 header：40px 行，无下边框"},
	{"titleRow", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 65, "标题行：min-height 30，grid-column 2"},
	{"titleCluster", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 91, "标题簇：gap 10 + 左侧渐隐 mask"},
	{"headerLeading", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 111, "header 最左（全局导航）"},
	{"crumbs", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 120, "面包屑行：gap 4"},
	{"crumbSeg", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 129, "一段面包屑：gap 4"},
	{"crumbSep", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 136, "分隔符：14/20 caption"},
	{"crumb", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 143, "一级面包屑：max-width 220, padding 4/8, 14/20"},
	{"crumbCurrent", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 165, "当前层级：label-primary / 500"},
	{"headerActions", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 171, "标题旁动作：**gap 10px**（rc.1 是 8px）"},
	{"headerUtilities", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 184, "工具区：gap 8 + margin-left 20；:empty 不显示"},
	{"headerCorner", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 212, "右栏展开：margin-left 8 / margin-right -16"},
	{"tabs", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 225, "view tab 行：gap 16/20、margin-top 10、padding-left 8"},
	{"viewTabs", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 246, "tab 列表：gap 32、min-height 25"},
	{"tab", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 260, "tab：min-height 35、13/16 500"},
	{"tabActive", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 291, "当前 tab"},
	{"tabLabel", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 314, "tab 文本：max-width 88"},
	{"body", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 458, "正文轴：--dsh-chat-content-width 与 composer 宽度都挂这里"},
	{"scrollBody", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 485, "滚动体：margin-right 2, scrollbar-gutter stable"},
	{"composerSeat", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 430, "composer 座：active 时 sticky bottom + z-index 9"},
	{"composerStack", "ui-conversation/src/client/skeleton/ConversationRoot.module.css", 419, "composer 链栈：gap 6"},

	// HeroShell
	{"root", "ui-conversation/src/client/skeleton/HeroShell.module.css", 5, "Hero：居中，padding 0 24"},
	{"stack", "ui-conversation/src/client/skeleton/HeroShell.module.css", 15, "Hero stack：gap 12，max-width = composer 卡片宽"},
	{"headline", "ui-conversation/src/client/skeleton/HeroShell.module.css", 29, "标题行：26/32 500，列 gap 10 行 gap 12"},
	{"titleGroup", "ui-conversation/src/client/skeleton/HeroShell.module.css", 51, "标题组：列 gap 7 行 gap 4"},
	{"previewBadge", "ui-conversation/src/client/skeleton/HeroShell.module.css", 62, "预览徽标：12/18 500、padding 1/7、999px"},
	{"fish", "ui-conversation/src/client/skeleton/HeroShell.module.css", 90, "Hero 鱼标：34px、transform-origin 50% 60%"},
	{"workspaceRow", "ui-conversation/src/client/skeleton/HeroShell.module.css", 139, "工作区行：padding-left 8、z-index 10"},
	{"workspace", "ui-conversation/src/client/skeleton/HeroShell.module.css", 152, "工作区 pill：min-height 28、max-width 360、13/20 500"},

	// InputBar
	{"root", "ui-conversation/src/client/skeleton/InputBar.module.css", 2, "InputBar 根：padding 0 clearance 4"},
	{"dock", "ui-conversation/src/client/skeleton/InputBar.module.css", 13, "dock：gap 12、padding-top 4、居中"},
	{"card", "ui-conversation/src/client/skeleton/InputBar.module.css", 54, "输入卡：radius-panel 28、padding-top 8、gap 12"},
	{"scroll", "ui-conversation/src/client/skeleton/InputBar.module.css", 145, "输入滚动区：max-height 336、margin-right 4"},
	{"input", "ui-conversation/src/client/skeleton/InputBar.module.css", 186, "输入区：min-height 36（hero 52）、padding 4/8/0/14"},
	{"placeholder", "ui-conversation/src/client/skeleton/InputBar.module.css", 235, "占位文本：与 input 同 padding"},
	{"row", "ui-conversation/src/client/skeleton/InputBar.module.css", 273, "控件行：gap 12、padding 2/8/6"},
	{"primary", "ui-conversation/src/client/skeleton/InputBar.module.css", 398, "发送/停止主钮：34×34 圆形"},
	{"sendingSpinner", "ui-conversation/src/client/skeleton/InputBar.module.css", 442, "sending 旋转：800ms linear"},

	// ChatView
	{"frame", "ui-chat/src/client/chat/ChatView.module.css", 1, "ChatView frame：flex column, min-height 0"},
	{"root", "ui-chat/src/client/chat/ChatView.module.css", 11, "ChatView root：overflow-y clip"},
	{"scroll", "ui-chat/src/client/chat/ChatView.module.css", 22, "transcript 滚动区：padding 16 / clearance+16"},
	{"column", "ui-chat/src/client/chat/ChatView.module.css", 69, "正文列：max-width = chat content width，居中"},
	{"flowItem", "ui-chat/src/client/chat/ChatView.module.css", 102, "流节点：min-width 0"},
	{"turnSpacer", "ui-chat/src/client/chat/ChatView.module.css", 108, "尾部留白：height 0"},

	// ui-sidebar
	{"root", "ui-sidebar/src/client/SidebarRoot.module.css", 9, "SidebarRoot：padding 6/8（rc.2；rc.1 是 6/12）、14px"},
	{"topStrip", "ui-sidebar/src/client/SidebarRoot.module.css", 187, "macOS 顶条：52 高"},
	{"logoRow", "ui-sidebar/src/client/SidebarRoot.module.css", 231, "品牌行：Web 60 / Windows 40，gap 8"},
	{"brand", "ui-sidebar/src/client/SidebarRoot.module.css", 258, "品牌按钮：flex 1"},
	{"brandIdentity", "ui-sidebar/src/client/SidebarRoot.module.css", 279, "mark+name 共同基线：gap 5、24 高"},
	{"brandMark", "ui-sidebar/src/client/SidebarRoot.module.css", 287, "鱼标座位"},
	{"brandName", "ui-sidebar/src/client/SidebarRoot.module.css", 294, "wordmark 座位：18/24 600、letter-spacing .04em"},
	{"newSession", "ui-sidebar/src/client/SidebarRoot.module.css", 420, "新建会话：高 38、radius-md、padding 8/12"},
	{"scrollRegion", "ui-sidebar/src/client/SidebarRoot.module.css", 533, "滚动区：margin-top 4、flex 1"},
	{"panelList", "ui-sidebar/src/client/SidebarRoot.module.css", 673, "导航列表：gap 2、margin-bottom 8"},
	{"panelRow", "ui-sidebar/src/client/SidebarRoot.module.css", 685, "导航行：min-height 32（rc.2；rc.1 是 36）、padding 5/9"},
	{"regionArea", "ui-sidebar/src/client/SidebarRoot.module.css", 766, "工作区/会话树区域"},
	{"footArea", "ui-sidebar/src/client/SidebarRoot.module.css", 781, "底部座位：footer 动作在上、设置在下的容器"},

	// ui-sidebar-right
	{"session", "ui-sidebar-right/src/client/shell/SidebarRight.module.css", 23, "每会话 surface：display:contents"},
	{"panel", "ui-sidebar-right/src/client/shell/SidebarRight.module.css", 31, "右栏面板：absolute 右缘、dock layer 10 / float 60"},
	{"iconButton", "ui-sidebar-right/src/client/shell/SidebarRight.module.css", 134, "面板控制钮：28×28、padding 6"},
	{"collapseGlyph", "ui-sidebar-right/src/client/shell/SidebarRight.module.css", 162, "收起图标：scaleX(-1)"},
	{"unavailable", "ui-sidebar-right/src/client/shell/SidebarRight.module.css", 179, "不可用提示"},
	{"tabTitle", "ui-sidebar-right/src/client/shell/SidebarRight.module.css", 187, "tab 标题：display:contents"},
	{"tabBody", "ui-sidebar-right/src/client/shell/SidebarRight.module.css", 191, "tab 内容：flex column, height 100%"},
}

// removedClasses 是 rc.2 里**不存在**的类名。实现里出现就是结构回流。
var removedClasses = []RemovedClass{
	{
		Name: "crumbSubagent", WasRC1: true,
		Evidence:    "ui-conversation/src/client/skeleton/ConversationRoot.module.css 全文 grep crumbSubagent = 0 命中（rc.1 有该类）",
		Replacement: "conversation.session.header.lineage 单槽 + .crumb（当前层级），没有独立 subagent 类",
	},
	{
		Name: "flashEven", WasRC1: true,
		Evidence:    "ui-sidebar-right/src/client/shell/SidebarRight.module.css rc.2 无 `.flashEven` 类；只有 `[data-sidebar-right-flash='even']` 属性选择器与 @keyframes flashEven（动画名）",
		Replacement: "[data-sidebar-right-flash='even']（属性选择器）",
	},
	{
		Name: "flashOdd", WasRC1: true,
		Evidence:    "同上；rc.2 用 `[data-sidebar-right-flash='odd']`",
		Replacement: "[data-sidebar-right-flash='odd']（属性选择器）",
	},
	{
		Name: "bubble", WasRC1: false,
		Evidence:    "ui-chat/src/client/chat/MessageItem.module.css rc.2 只有注释提到 bubble，无 `.bubble` 类（rc.1 已删）",
		Replacement: ".userRow / .userStack（气泡外观由 userStack 承担）",
	},
}

// OfficialClasses 返回结构骨架对拍用的官方类表副本。
func OfficialClasses() []ClassSpec {
	return append([]ClassSpec(nil), officialClasses...)
}

// RemovedClasses 返回 rc.2 中已不存在的类名副本。
func RemovedClasses() []RemovedClass {
	return append([]RemovedClass(nil), removedClasses...)
}
