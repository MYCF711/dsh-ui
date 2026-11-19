package dsassets

import "github.com/vivaui/viva/gfx"

// 本文件是**素材登记表**：一枚素材 = 一小组官方图元 + 它的出处 + 它能不能被
// viva 精确画出来。ID 与 `testdata/official/<ID>.jsx` 同名，golden 测试按 ID 对表。

// ElementKind 是官方 SVG 里的一种绘制图元。
type ElementKind uint8

const (
	// PathElement 是 `<path d=…>`：viva 能逐命令精确表达
	// （M/L/H/V/C/S/A/Z 及相对形式；弧在解析时换算成三次贝塞尔，见 arc.go）。
	PathElement ElementKind = iota
	// RectElement 是 `<rect>`：`rx>0` 时圆角是四条四分之一椭圆弧，
	// 换算成三次贝塞尔后误差有数学上界（每段 < 0.03% of r），见 shapes.go。
	RectElement
	// CircleElement 是 `<circle>` / `<ellipse>`：同样换算成四段三次贝塞尔。
	CircleElement
)

// Element 是一个官方图元的原样描述（属性取自官方 JSX，未做换算）。
type Element struct {
	Kind   ElementKind
	D      string // Kind == PathElement 时的 `d` 原文
	Fill   bool   // 官方 fill="currentColor"
	Stroke bool   // 官方 stroke="currentColor"
	// Cap/Join 只在官方的 path **明确写了** stroke-linecap / stroke-linejoin 时才置位
	// （同时把 CapSet/JoinSet 置 true）。gfx.CapButt 与 gfx.JoinMiter 的零值就是官方的
	// 默认值，所以"没写"必须和"写了 miter"区分开——否则一枚图标里盾牌写了 round、
	// 对勾没写，会被误判成"两处要求互相矛盾"而不画。
	Cap     gfx.StrokeCap
	CapSet  bool
	Join    gfx.StrokeJoin
	JoinSet bool
	// Rect 专属。RX/RY 是圆角半径；**RY 为 0 表示官方没写**，按 SVG 1.1 §9.2 取 RX
	// （官方现有两枚 rect 都只写了 rx，所以这里绝大多数情况是"取 RX"）。
	X, Y, Width, Height, RX, RY float32
	// Circle 专属。`<circle>` 用 R；`<ellipse>` 用 RX/RY（任一 >0 即按椭圆处理，
	// 此时 R 不参与）。官方现有两枚都是 `<circle r>`。
	CX, CY, R float32
}

// Asset 是一枚可复用素材。
type Asset struct {
	// ID 同时是 `testdata/official/` 里的文件名（不含扩展名）。
	ID string
	// Title 是文档里用的中文名。
	Title string
	// Source 是出处。
	Source Source
	// ViewBox 是官方 viewBox 的宽（官方这套图标都是正方形 16）。
	ViewBox float32
	// Size 是官方组件的默认显示尺寸（px）。
	Size float32
	// StrokeWidth 是官方 Regular 档的描边宽度（viewBox 单位）。填充素材为 0。
	StrokeWidth float32
	// Elements 按官方 JSX 顺序排列。
	Elements []Element
	// Unsupported 非空表示这枚素材**不能被精确画出来**，内容是原因。
	// Build 对它返回 false，调用方不许自己找替代形状（那正是本包要消灭的事）。
	//
	// 2026-11-19 起本表**没有一条**是 Unsupported：`<rect rx>` / `<circle>` 需要的
	// 椭圆弧已经能换算成三次贝塞尔（arc.go），15 条全部可画。字段保留是因为
	// "说不出来的不可画"仍要能被登记——`registry_test.go` 会双向核对声明与实现。
	Unsupported string
}

// Buildable 表示这条素材能逐命令精确表达。
func (a Asset) Buildable() bool { return a.Unsupported == "" }

// 素材传输时用的出处常量，避免 14 条记录里重复写包名/文件/哈希（写错一处就没人查得出来）。
var (
	srcComposer = Source{
		Package: "@deepseek-ai/dsh-client-ui-conversation",
		File:    "dsh-client-ui-conversation/lib/client.js",
		SHA256:  "fd8ee6c74845d017c2d0e2e6fb62fc655c42b084b1b014da08e1ef7355e3e984",
	}
	srcPrimitives = Source{
		Package: "@deepseek-ai/dsh-client-ui-primitives",
		File:    "dsh-web-frontend/dist/assets/index-BXV-oxvX.js",
		SHA256:  "0f3fc6b857e4f8bd4d69619ce1010240cdaaa443abf0826532a24ef8f6e32ef0",
	}
)

// srcComposerSymbol 返回 conversation 里某个行内图元的出处。
func srcComposerSymbol(symbol string) Source {
	s := srcComposer
	s.Symbol = symbol
	return s
}

// srcPrimitivesSymbol 返回 primitives 里某个符号的出处（包/文件相同，符号不同）。
func srcPrimitivesSymbol(symbol string) Source {
	s := srcPrimitives
	s.Symbol = symbol
	return s
}

// 官方 Regular 档的描边宽度（`ICON_REGULAR_STROKE`）。
const regularStroke = 1

// 官方 Medium 档的描边宽度（web 前端 chunk 里的 `K=1.3`，与 `MT=1` 的 Regular 档并列）。
//
// 为什么单独一个常量：加号这枚官方用的是 **Medium**（`IconPlusOutlineMedium`），
// 14px 显示下 1.3 的描边才是官方的粗细；拿 Regular 的 1 去画会明显偏细。
const mediumStroke = 1.3

// 素材表。**这里是全仓唯一的官方矢量来源**：dscomposer / dsconversation 只准
// 从 `Build` 拿图标，不再自己写坐标。
var (
	// ComposerSend 是输入框发送键的箭头（填充）。
	ComposerSend = Asset{
		ID: "composer-send", Title: "发送键箭头",
		Source:  srcComposerSymbol("InputBar 主按钮内联 path"),
		ViewBox: 16, Size: 16,
		Elements: []Element{{Kind: PathElement, D: composerSendPath, Fill: true}},
	}

	// ComposerStop 是输入框运行态的停止方块。官方是
	// `<rect x=3 y=3 width=10 height=10 rx=3 fill="currentColor">`，rx=3 的圆角
	// 是真正的四分之一圆弧。
	//
	// **能精确画**：`Build` 把它换算成"四段四分之一弧 + 四条直边"的路径（10 条命令），
	// 每段径向误差 < 0.03% of r（`TestRoundedRectCornersAreExactWithinCriterion`）。
	// 而 `dscomposer` 走的是另一条同样精确的路——Viva 原生圆角矩形
	// （`mustOfficialNativeRect`），几何取自本登记项、不补画路径。
	//
	// 历史：2026-11-04 之前用二次贝塞尔凑角，实测角部最多偏 0.182 个 viewBox 单位
	// （16px 下约 0.18px）；那是"看起来差不多"，与这里的弧→贝塞尔换算不是一回事——
	// 后者每段误差 2.7e-4·r（16px 下 0.0043px），差两个数量级。
	ComposerStop = Asset{
		ID: "composer-stop", Title: "停止键方块",
		Source:  srcComposerSymbol("InputBar 主按钮内联 rect"),
		ViewBox: 16, Size: 16,
		Elements: []Element{{Kind: RectElement, X: 3, Y: 3, Width: 10, Height: 10, RX: 3, Fill: true}},
	}

	// SendOutline 是图标集里的发送箭头（机尾填充 + 机身前段描边），14px。
	SendOutline = Asset{
		ID: "send-outline", Title: "发送图标（图标集）",
		Source:  srcPrimitivesSymbol("IconSendOutlineRegular"),
		ViewBox: 16, Size: 14, StrokeWidth: regularStroke,
		Elements: []Element{
			{Kind: PathElement, D: sendOutlineHeadPath, Fill: true},
			{Kind: PathElement, D: sendOutlineStemPath, Stroke: true},
		},
	}

	// Paperclip 是附件回形针（纯描边）。
	Paperclip = Asset{
		ID: "paperclip", Title: "附件回形针",
		Source:  srcPrimitivesSymbol("IconPaperclipOutlineRegular"),
		ViewBox: 16, Size: 16, StrokeWidth: regularStroke,
		Elements: []Element{{Kind: PathElement, D: paperclipPath, Stroke: true}},
	}

	// ComposerAdd 是输入框左侧那枚**命令入口**：官方 `.GRgSLq_add`（28×28 圆形）
	// 里画的 `<IconPlusOutlineMedium size={14}/>`。
	//
	// 注意它是 Medium 档（描边 1.3），不是 Regular；官方在 14px 显示下用 1.3 的
	// 线宽，拿 1 去画会明显偏细。素材本身纯描边、无填充，颜色由调用方 `.Color()` 给。
	ComposerAdd = Asset{
		ID: "composer-add", Title: "命令入口加号",
		Source:  srcPrimitivesSymbol("IconPlusOutlineMedium"),
		ViewBox: 16, Size: 14, StrokeWidth: mediumStroke,
		Elements: []Element{
			{Kind: PathElement, D: composerAddVerticalPath, Stroke: true},
			{Kind: PathElement, D: composerAddHorizontalPath, Stroke: true},
		},
	}

	// Copy 是复制图标：前面那张纸是 `<rect rx=2>`（描边），后面那张是填充路径。
	//
	// **能精确画**：rect 的四个圆角换算成四段四分之一椭圆弧（10 条命令），
	// 与填充那条路径合成一个 `Stroke=1` + 一个填充层的 gfx.Icon。
	Copy = Asset{
		ID: "copy", Title: "复制",
		Source:  srcPrimitivesSymbol("IconCopyOutlineRegular"),
		ViewBox: 16, Size: 16, StrokeWidth: regularStroke,
		Elements: []Element{
			{Kind: RectElement, X: 1.52075, Y: 4.07373, Width: 10.3932, Height: 10.3932, RX: 2, Stroke: true},
			{Kind: PathElement, D: copyBackPath, Fill: true},
		},
	}

	// Branch 是分支图标：两条支线（描边）+ 两枚端点节点 `<circle r=1.56962>`（描边）。
	//
	// **能精确画**：圆换算成四段四分之一圆弧（每枚 6 条命令：M + 4C + Z），
	// 与两条支线合成一条描边命令流（24 条命令）。语义与官方 `branch.jsx` 逐项一致：
	// 圆心 (12.4502, 3.3079) / (12.4502, 12.6921)、半径 1.56962、viewBox 16、描边 1。
	Branch = Asset{
		ID: "branch", Title: "分支",
		Source:  srcPrimitivesSymbol("IconBranchOutlineRegular"),
		ViewBox: 16, Size: 16, StrokeWidth: regularStroke,
		Elements: []Element{
			{Kind: PathElement, D: branchUpperPath, Stroke: true},
			{Kind: PathElement, D: branchLowerPath, Stroke: true},
			{Kind: CircleElement, CX: 12.4502, CY: 3.3079, R: 1.56962, Stroke: true},
			{Kind: CircleElement, CX: 12.4502, CY: 12.6921, R: 1.56962, Stroke: true},
		},
	}

	// ThumbUpOutline / ThumbUpFill / ThumbDownOutline / ThumbDownFill：
	// 官方反馈按钮的四种状态。描边与填充用同一条 `d`，只有 fill 有无之差。
	ThumbUpOutline   = thumbAsset("thumb-up-outline", "👍 未评分", "IconLikeOutlineRegular", thumbUpPath, false)
	ThumbUpFill      = thumbAsset("thumb-up-fill", "👍 已评分", "IconLikeFillRegular", thumbUpPath, true)
	ThumbDownOutline = thumbAsset("thumb-down-outline", "👎 未评分", "IconDislikeOutlineRegular", thumbDownPath, false)
	ThumbDownFill    = thumbAsset("thumb-down-fill", "👎 已评分", "IconDislikeFillRegular", thumbDownPath, true)

	// PermissionReadOnly 是权限「只读」档：盾牌 + 对勾。
	PermissionReadOnly = permissionAsset("permission-read-only", "权限·只读",
		"PermissionIconReadOnlyRegular",
		Element{Kind: PathElement, D: permissionCheckPath, Stroke: true},
		// 官方对勾的那条 path 没写 linecap/linejoin（默认 butt/miter），
		// 盾牌那条写了 linejoin="round"。同一枚图标只有一个 Join，取 round
		// 才不会把盾牌的转角画成尖角；对勾只有一个 C，miter 与 round 无差别。
		Element{Kind: PathElement, D: permissionShieldPath, Stroke: true, Join: gfx.JoinRound, JoinSet: true},
	)

	// PermissionFullAccess 是权限「完全访问」档：盾牌 + 感叹号。
	PermissionFullAccess = permissionAsset("permission-full-access", "权限·完全访问",
		"PermissionIconFullAccessRegular",
		Element{Kind: PathElement, D: permissionShieldPath, Stroke: true, Join: gfx.JoinRound, JoinSet: true},
		Element{Kind: PathElement, D: permissionFullAccessStemA, Stroke: true},
		Element{Kind: PathElement, D: permissionFullAccessStemB, Stroke: true},
	)

	// PermissionAutoReview 是权限「自动审查」档：盾牌 + A 字（官方这条写了圆帽圆接）。
	PermissionAutoReview = permissionAsset("permission-auto-review", "权限·自动审查",
		"PermissionIconAutoReviewRegular",
		Element{Kind: PathElement, D: permissionShieldPath, Stroke: true, Join: gfx.JoinRound, JoinSet: true},
		Element{Kind: PathElement, D: permissionAutoReviewPath, Stroke: true, Cap: gfx.CapRound, CapSet: true, Join: gfx.JoinRound, JoinSet: true},
	)

	// PermissionWorkspaceWrite 是权限「工作区写入」档：填充盾牌 + 两行文字 + 铅笔 + 下划线。
	PermissionWorkspaceWrite = permissionAsset("permission-workspace-write", "权限·工作区写入",
		"PermissionIconWorkspaceWriteRegular",
		Element{Kind: PathElement, D: permissionWorkspaceShieldPath, Fill: true},
		Element{Kind: PathElement, D: permissionWorkspaceLineA, Stroke: true},
		Element{Kind: PathElement, D: permissionWorkspaceLineB, Stroke: true},
		Element{Kind: PathElement, D: permissionWorkspacePencilPath, Fill: true},
		Element{Kind: PathElement, D: permissionWorkspaceUnderlinePath, Fill: true},
	)
)

// thumbAsset 造一枚拇指素材：描边档 StrokeWidth=1，填充档额外带 fill。
func thumbAsset(id, title, symbol, d string, filled bool) Asset {
	return Asset{
		ID: id, Title: title,
		Source:  srcPrimitivesSymbol(symbol),
		ViewBox: 16, Size: 16,
		StrokeWidth: regularStroke,
		Elements:    []Element{{Kind: PathElement, D: d, Fill: filled, Stroke: true}},
	}
}

// permissionAsset 造一枚权限素材（四档共用 16 viewBox 与 Regular 描边宽度）。
func permissionAsset(id, title, symbol string, elements ...Element) Asset {
	return Asset{
		ID: id, Title: title,
		Source:  srcPrimitivesSymbol(symbol),
		ViewBox: 16, Size: 16,
		StrokeWidth: regularStroke,
		Elements:    elements,
	}
}

// All 返回全部素材（顺序与文档表格一致）。测试用它保证"表里有几条、文档就写几条"。
func All() []Asset {
	return []Asset{
		ComposerSend,
		ComposerStop,
		SendOutline,
		Paperclip,
		ComposerAdd,
		Copy,
		Branch,
		ThumbUpOutline,
		ThumbUpFill,
		ThumbDownOutline,
		ThumbDownFill,
		PermissionReadOnly,
		PermissionWorkspaceWrite,
		PermissionFullAccess,
		PermissionAutoReview,
	}
}

// Lookup 按 ID 找素材。
func Lookup(id string) (Asset, bool) {
	for _, a := range All() {
		if a.ID == id {
			return a, true
		}
	}
	return Asset{}, false
}
