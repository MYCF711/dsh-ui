// Package dsshell 是框架插件（L3）：三栏几何 + 底部整行 + 浮层。
//
// 它把「窗口里怎么分栏」从壳里搬了出来——壳只画一个 shell.root 槽，
// 这个插件在 root 里切出侧栏 / 中栏 / 右栏 / 底部整行，然后把 frame 交给
// 下一层（sidebar.*  /  main.* / rightbar / shell.bottom）。
//
// 三栏宽度**不是拍脑袋的比例**，是官方 `dsh-client-ui-layout` 的
// `computeColumns` 逐行移植（asar 里 `AppFrame.js` 的原函数）：
//
//	function computeColumns(viewport, sidebar, rightbar, collapsedWidth = 56) {
//	    const s = sidebar === 0 ? collapsedWidth : clampWidth(sidebar, 264, 420);
//	    const available = viewport - s - 400;
//	    const r = rightbar === 0 || available < 300 ? 0
//	            : Math.min(available, clampWidth(rightbar, 300, viewport * .7));
//	    return { sidebar: s, center: Math.max(0, viewport - s - r), rightbar: r };
//	}
//
// 三条规则值得单独记住（以前都踩过）：
//
//	侧栏   264–420 定宽、不参与压缩；没占用方时整列不占位
//	中栏   至少 400，**只有右栏整个不出现时**才允许被压到 400 以下
//	右栏   首次打开占视口 45%；放不下（available < 300）时官方是**整列消失**，
//	       不是压到 300 硬塞——硬塞会把中栏挤到 400 以下
//	底部   横跨三列的整行，占用方自己声明高度，三列按它的高度变矮
//
// # 分栏可拖拽
//
// 官方那套比例是**默认值**，不是铁律：真界面上侧栏和右栏的分隔线可以拖。
// 这里在官方求解之上加一层「用户选定宽度」覆盖（`UserSidebarWidth` /
// `UserRightbarWidth`，>0 时压过壳给的 Geometry）：
//
//	拖拽把手   分隔线两侧各 4px 的抓取带（共 8px，不占布局，绝对定位叠在列上），
//	           悬停时画 2px 描边线、拖动时画 3px 品牌色线，指针是 SizeWE
//	拖动限制   侧栏 180–400、右栏 300–视口 40%（官方默认值 264–420 / 45%
//	           是**算默认**时的口径，用户明确拖到的位置不该被它弹回去）
//	几何真值   拖动后本插件把 env.Geometry 校正成**这一帧真实画出来的**三列，
//	           否则下游（侧栏的活动面板、composer 的排版宽）还按旧值摆
//
// 事件走 viva 的 canvas.PointerOn：按下时指针被捕获，拖动可以离开抓取带，
// Consume 让运行时立刻重绘一帧——所以宽度是跟着指针实时变的，不是松手才变。
//
// 插件不需要知道自己挂在第几层：它只认「父级把 f 递给我」。
package dsshell

import (
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// 官方 ui-layout 的常量（asar：`@deepseek-ai/dsh-client-ui-layout`）。
const (
	sidebarMin = 264 // 官方 clampWidth(sidebar, 264, 420) 的下界
	sidebarMax = 420 // 同上界
	// sidebarRail 是官方「收起后保留 56px 控制栏」的轨道宽度。
	// 这一版没有折叠功能，solveColumns 里由调用方传 collapsedWidth；
	// 传 0 表示「没有占用方就整列不占位」（本插件既有约定）。
	sidebarRail = 56

	mainMin = 400 // 官方中栏下限：minmax(400px, 1fr)
	// rightbarMin / rightbarMaxFrac 是官方右栏的两端：clampWidth(rightbar, 300, viewport*.7)。
	rightbarMin      = 300
	rightbarMaxFrac  = 0.7
	rightbarOpenFrac = 0.45 // 官方首次打开：layoutInfo.rightbar ?? viewport * .45
)

// 用户拖出来的宽度**不受官方默认值那两条夹取范围约束**：
// 264–420 是官方算默认宽度时用的，用户明确拖到 180 就该是 180。
//
// 上下限的出处与理由：
//
//	侧栏 180  再窄就放不下会话行的图标 + 标题（官方折叠态是 56，那是**收起**，不是拖窄）
//	侧栏 400  与官方上界 420 同量级，留一点余量，避免拖动值被 layout 二次夹掉
//	右栏 300  与官方下限一致：再窄面板里的表格/代码就开始横向溢出
//	右栏 40%  面板的合理上限。官方首次打开是 45%，比这里还宽——两者只在「用户
//	          第一次拖右栏之前」冲突，拖拽时的上限取 max(40%, 按下时的宽度)，
//	          所以从 45% 往右拖不会突然掉到 40%（见 dragCeiling）
const (
	userSidebarMin = 180
	userSidebarMax = 400

	userRightbarMin     = 300
	userRightbarMaxFrac = 0.40
)

// 拖拽把手的尺寸。
//
// handleWidth 是**命中带**的宽度，不占布局：抓取带绝对定位叠在列上，
// 所以三列宽度仍然严格等于 solveColumns 的结果，不会因为这个功能各少 8px。
// 8px 与 viva 自己的 flex.Splitter 一致。
//
// 线宽：平时不画线（官方两栏之间本来就没有分隔线，只靠底色差），
// 悬停 2px / 拖动 3px——这就是「至少视觉提示」的那一档。
const (
	handleWidth     = 8
	handleLineHover = 2
	handleLineDrag  = 3
)

// 把手的两种身份。用字符串而不是 bool，是因为一次只能拖一根：
// dragState.kind 记住「现在拖的是哪根」，另一根的事件必须忽略
// （否则拖动中鼠标划过另一根把手会把那一列也改掉）。
const (
	handleSidebar  = "sidebar"
	handleRightbar = "rightbar"
)

// 宿主 `ui-config.json` 的键名。宿主是**纯 key/value 存储、不解释**（见
// internal/apiclient/config.go），所以键名和值的格式完全由界面这边定：
// 写成像素整数（"360"），别写 "360px"——下次读回来还得自己拆。
const (
	cfgKeySidebar  = "ui.sidebarWidth"
	cfgKeyRightbar = "ui.rightbarWidth"

	// configWaitFrames 是愿意等壳把配置拿回来的帧数（约 10 秒 @60fps）。
	// 壳在第一帧之后才异步 config.get，通常几十毫秒就到；等这么久是兜底。
	// 之后不再每帧问——壳真没存过任何键时 map 恒空，每帧拷一份不值得。
	configWaitFrames = 600
)

// columnPad 是中栏/右栏由壳加的内边距。
//
// 官方 AppFrame 的 centerCol / rightbarCol 都是**裸列**（CSS 里没有 padding，
// 内容 inset 归占用方：ChatView scroll 32、composer clearance 16、右栏面板自画）。
// 我们这里保留 12，是因为它已经是**跨插件契约**：
// `plugins/dscomposer/composer.go` 的 `shellMainPad = 12` 就是按「12 + 4 = 官方 16」
// 校过的第二个出处，单方面把这里改成 0，composer 的净边距会从 16 掉到 4。
// 切到官方裸列模型要 dsshell + dscomposer + dsconversation + dstopbar 同一次改，
// 见 V4A 报告的「缺口」一节；到时候只改这个常量一处即可。
const columnPad = 12

// columns 是官方 computeColumns 的结果（像素）。
type columns struct {
	sidebar  float32
	center   float32
	rightbar float32
}

// clampWidth 对照官方 `clampWidth(px, min, max)`：
//
//	return Math.min(max, Math.max(min, Math.round(px)));
//
// 官方多一次 Math.round。这里保留取整：窗口宽度是整数，差值取整后
// 三列之和才能与视口严格相等（否则 0.5px 的余量会随机分给某一列）。
func clampWidth(px, lo, hi float32) float32 {
	if px < lo {
		px = lo
	}
	if px > hi {
		px = hi
	}
	return float32(round32(px))
}

// solveColumns 是官方 computeColumns 的逐行移植。
//
// 参数：
//
//	viewport      整窗宽（客户区，不是含标题栏的整窗）
//	sidebarPref   侧栏偏好宽；<=0 表示「没有占用方」，此时用 collapsedWidth
//	rightbarPref  右栏偏好宽（首次打开用 viewport*0.45）；<=0 表示没有偏好
//	rightbarOn    右栏这一帧要不要出现（占用方在 + 壳说展开）
//	collapsedWidth 侧栏折叠后保留的轨道宽；0 = 整列不占位
//
// 右栏的三条下落顺序照官方写死：
//  1. 没有占用方/没收起 → 0；
//  2. 扣掉侧栏和中栏下限后 available < 300 → 0（整列消失，不是压窄）；
//  3. 否则 min(available, clamp(pref, 300, viewport*0.7))。
func solveColumns(viewport, sidebarPref, rightbarPref float32, rightbarOn bool, collapsedWidth float32) columns {
	return solveColumnsLimits(viewport, sidebarPref, rightbarPref, rightbarOn, collapsedWidth, sidebarMin, sidebarMax)
}

// solveColumnsLimits 是 solveColumns 去掉「侧栏两端写死」的那一版。
//
// 为什么要多一个函数而不是就地改常量：官方口径（264–420）要留给**默认宽度**，
// 它被 TestSolveColumnsMatchesOfficial 逐条对拍着，不能因为加了拖拽就变；
// 用户拖出来的宽度走另一组端点（userSidebarMin/Max）。
// 右栏不需要这一层：它的下限 300 与官方一致，上限只在拖拽时收紧（40%），
// 落盘的值不会在 layout 里被二次夹紧——否则窗口一大一小就把用户的选择改掉了。
func solveColumnsLimits(viewport, sidebarPref, rightbarPref float32, rightbarOn bool, collapsedWidth, sideLo, sideHi float32) columns {
	if viewport <= 0 {
		return columns{}
	}
	var s float32
	if sidebarPref <= 0 {
		s = collapsedWidth
	} else {
		s = clampWidth(sidebarPref, sideLo, sideHi)
	}
	var r float32
	if rightbarOn && rightbarPref > 0 {
		if available := viewport - s - mainMin; available >= rightbarMin {
			r = min(available, clampWidth(rightbarPref, rightbarMin, viewport*rightbarMaxFrac))
		}
	}
	c := viewport - s - r
	if c < 0 {
		c = 0
	}
	return columns{sidebar: s, center: c, rightbar: r}
}

// round32 把像素值取整（官方 clampWidth 里的 Math.round）。
// 自己写是为了不引 math 包：这里只处理 0..1e4 的正数，加 0.5 再截断即可。
func round32(v float32) float32 {
	if v < 0 {
		return float32(-int32(-v + 0.5))
	}
	return float32(int32(v + 0.5))
}

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme pluginapi.Theme
	prims pluginapi.Primitives

	// SidebarWidth 侧栏默认宽度（官方默认 280）。
	// 壳在 Geometry 里给了宽度就以壳为准，这里只是「壳没说」时的兜底。
	SidebarWidth float32
	// RightbarWidth 右栏宽度；<=0 时按官方首次打开的口径算（视口 45%）。
	RightbarWidth float32

	// UserSidebarWidth / UserRightbarWidth 是**用户拖出来的**宽度（0 = 没拖过）。
	//
	// 语义（这三条就是持久化的接口约定）：
	//  1. >0 时压过壳给的 Geometry——用户拖过的宽度不能再被「默认值」盖回去；
	//  2. 壳启动时把读回的 ui-config.json 值（ui.sidebarWidth / ui.rightbarWidth）
	//     填进这两个字段，就是「重启后宽度保留」；
	//  3. 拖拽松手时插件会写回这两个字段，并把它们通过 Actions.ConfigSet 落盘。
	//
	// 两条灌值路径都留着：**Actions.ConfigGet**（pluginapi 的跨仓契约，壳接线后
	// 插件自己读，见 readConfig）和**壳直接填字段**（壳在装配前就知道配置时更省事）。
	// 谁先到算谁的：两个字段一旦 >0 就不再被配置覆盖——用户拖过的手感优先。
	UserSidebarWidth  float32
	UserRightbarWidth float32

	// configPoll 是已经问过几次 Actions.ConfigGet。
	// 壳在第一帧之后才异步把配置拿回来，所以这里要等几帧；
	// 但壳真没配过任何键时 map 永远是空的，不能无限每帧拷一份 map。
	configPoll int
	// configLoaded 表示已经处理过一份非空配置（或者两列都由用户定过了）。
	configLoaded bool

	// dragging 是一次进行中的拖拽。零值 = 没在拖。
	dragging dragState
}

// dragState 记录拖拽起点：宽度一律按「起点宽度 + 指针位移」算，
// 而不是按上一帧的宽度累加——后者会把取整误差和丢帧越积越大。
type dragState struct {
	active bool
	kind   string  // handleSidebar / handleRightbar
	startX float32 // 按下时的指针 x
	startW float32 // 按下时那一列的宽度
	// moved 表示这一次拖拽真的改过宽度。没改过（按下即松手）就不写配置：
	// 一次 config.set 是一次 HTTP，不该为"点了一下分隔线"发出去。
	moved bool
}

// handle 是一根拖拽把手在**这一帧**的位置。
type handle struct {
	kind string
	id   ui.ID
	// x 是命中带左缘的绝对坐标（三列都从 x=0 起算，所以就是列边界减半个带宽）。
	x float32
	// width 是它调的那一列当前宽；0 = 这一列不在，把手不出现。
	width float32
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-shell" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = *ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	if p.SidebarWidth <= 0 {
		p.SidebarWidth = 280
	}
	return nil
}

// Panels 实现 pluginapi.Panels。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "shell", Slot: pluginapi.SlotShellRoot, Order: 10, Draw: p.draw, Children: p.children},
	}
}

// layout 把「这一帧的三栏该多宽」算成官方 computeColumns 的结果。
//
// 输入优先级：**用户拖过的宽度**（User*Width，>0 才算）
// → 壳给的 Geometry（它知道侧栏偏好、右栏开合、白名单里有没有占用方）
// → 插件自己的默认值 → 官方比例。绝不拿 f.Bounds() 当列宽：槽位里那是整窗宽。
func (p *Plugin) layout(env *pluginapi.Env) columns {
	vw := viewport(env)
	if vw <= 0 {
		// 连视口都不知道时不编宽度：返回全 0，让各列按「没有」处理。
		return columns{}
	}

	// 侧栏：没有 sidebar.* 占用方就整列不占位（本插件的既有约定）。
	// 有占用方时，用户拖过的宽度优先——否则拖动结果会被壳的 280 盖掉。
	var sidebarPref float32
	if hasSidebar(env) {
		sidebarPref = p.UserSidebarWidth
		if sidebarPref <= 0 {
			sidebarPref = env.Geometry.SidebarWidth
			if sidebarPref <= 0 {
				sidebarPref = p.SidebarWidth
			}
		}
	}
	sideLo, sideHi := float32(sidebarMin), float32(sidebarMax)
	if p.UserSidebarWidth > 0 {
		sideLo, sideHi = userSidebarMin, userSidebarMax
	}

	rightbarOn := env.Data.ShowRightbar && env.Has(pluginapi.SlotRightbar)
	rightbarPref := p.UserRightbarWidth
	if rightbarPref <= 0 {
		rightbarPref = env.Geometry.RightWidth
		if rightbarPref <= 0 {
			rightbarPref = p.RightbarWidth
		}
		if rightbarPref <= 0 {
			rightbarPref = vw * rightbarOpenFrac // 官方：首次打开占视口 45%
		}
	}

	return solveColumnsLimits(vw, sidebarPref, rightbarPref, rightbarOn, 0, sideLo, sideHi)
}

// viewport 取这一帧的整窗宽：壳给的 Geometry.Width 优先，退到 Bounds。
func viewport(env *pluginapi.Env) float32 {
	if env.Geometry.Width > 0 {
		return env.Geometry.Width
	}
	if env.Bounds.Width > 0 {
		return env.Bounds.Width
	}
	return 0
}

// hasSidebar 判断这一帧有没有侧栏占用方（三个 sidebar.* 槽任意一个有东西）。
func hasSidebar(env *pluginapi.Env) bool {
	return env.Has(pluginapi.SlotSidebarHeader) || env.Has(pluginapi.SlotSidebarList) ||
		env.Has(pluginapi.SlotSidebarFooter)
}

// syncGeometry 把 env 里那三个「壳说」的几何字段校正成**本帧真正画出来的**。
//
// 为什么插件要动这些字段：壳算 Geometry 的口径和这里不完全一样，两处已经分叉过两次——
//
//  1. 中栏宽：壳里另有一份 `buildGeometry`（cmd/app/main.go），早先按
//     「右栏压到 300 硬塞」算，官方是「放不下就让右栏整列消失」。两者在
//     视口 < ~984（侧栏 280 时）分叉：壳说主区 380，这里按官方画 680，
//     下游若照 380 排版，内容就缩在左半边。
//  2. 三列宽：用户拖过分隔线之后，壳那份（侧栏 280、右栏 45%）就是**旧值**了。
//     侧栏的「活动面板」浮层按 Geometry.SidebarWidth 摆左缘、按
//     Geometry.MainWidth 定宽（plugins/dssidebar/frame.go），不校正的话
//     拖完侧栏那块面板会停在新侧栏的中间。
//
// 所以：这一帧画了什么，就把 Geometry 写成什么。壳的输入只在这一帧开始时读一次，
// 不存在跨帧累积（env 是每帧新建的）。
func syncGeometry(env *pluginapi.Env, cols columns) {
	if cols.center <= 0 {
		// 视口未知：不覆盖壳的值。
		return
	}
	env.MainWidth = cols.center
	env.Geometry.MainWidth = cols.center
	env.Geometry.SidebarWidth = cols.sidebar
	env.Geometry.RightWidth = cols.rightbar
}

// handles 算这一帧两根把手的位置。width 为 0 表示那一列不在，把手不画也不响应。
//
// 左边把手骑在「侧栏右缘」上，右边把手骑在「右栏左缘」上：各向外探 4px，
// 8px 的抓取带里 4px 压在列内、4px 压在列外。列外那 4px 分别落在中栏的
// 12px 内边距和右栏的 12px 内边距里，所以不会盖住任何真内容。
func (p *Plugin) handles(env *pluginapi.Env, cols columns) [2]handle {
	var out [2]handle
	if env.Tree == nil {
		// 测试里有时不建交互树：这一帧没有交互 id，就当没有把手（不画、不响应）。
		return out
	}
	if cols.sidebar > 0 {
		out[0] = handle{
			kind:  handleSidebar,
			id:    env.Tree.ID(env.Feat, "layout-handle-sidebar"),
			x:     cols.sidebar - handleWidth/2,
			width: cols.sidebar,
		}
	}
	if cols.rightbar > 0 {
		right := viewport(env) - cols.rightbar
		out[1] = handle{
			kind:  handleRightbar,
			id:    env.Tree.ID(env.Feat, "layout-handle-rightbar"),
			x:     right - handleWidth/2,
			width: cols.rightbar,
		}
	}
	return out
}

// dragHandles 处理两根把手上的指针事件，返回这一帧宽度有没有被改动。
//
// viva 的指针模型（ui.Canvas.PointerOn + app/painter.go）：
//
//	PointerDown  只送到「指针正下方」的命中区
//	PointerMove  按下之后指针被**捕获**，事件一直送到按下的那个 id，指针离开抓取带
//	             也照样送——所以在窗口任何位置拖动都能继续改宽，不会"一拖出线就断"
//	PointerUp    同样送到按下的 id
//
// Consume 的两层作用：告诉运行时这一下不是普通点击（不要再去派发 click），
// 并且让运行时立刻重绘一帧——宽度因此是跟手的，不是松手才跳过去。
func (p *Plugin) dragHandles(env *pluginapi.Env, cols columns) bool {
	c := env.Canvas
	if c == nil {
		return false
	}
	changed := false
	for _, h := range p.handles(env, cols) {
		if h.width <= 0 {
			continue
		}
		ev, ok := c.PointerOn(h.id)
		if !ok {
			continue
		}
		switch ev.Kind {
		case ui.PointerDown:
			// 只认主键。右键/中键落在分隔线上不应该开始拖。
			if ev.Button != ui.PointerButtonPrimary && ev.Button != ui.PointerButtonNone {
				continue
			}
			// 已经在拖另一根：不抢。真机上「按住左键再按右键」会送来第二个
			// PointerDown，放过去的话正在拖的那一列会定格、另一列突然开始跟手。
			if p.dragging.active {
				continue
			}
			p.dragging = dragState{active: true, kind: h.kind, startX: ev.X, startW: h.width}
			c.Consume()
		case ui.PointerMove:
			if !p.dragging.active || p.dragging.kind != h.kind {
				continue
			}
			if p.dragTo(env, ev.X) {
				changed = true
			}
			c.Consume()
		case ui.PointerUp:
			if !p.dragging.active || p.dragging.kind != h.kind {
				continue
			}
			p.dragging.active = false
			// 松手才落盘：拖动过程中每个 move 都写一次 config.set，一次拖拽就是
			// 几十次 HTTP 往返，宿主那边也是一串合并写。松手写一次就够——
			// 「拖动时实时改宽」靠的是内存里的宽度，跟落盘无关。
			p.persistWidths(env)
			c.Consume()
		}
	}
	return changed
}

// dragTo 把拖拽起点 + 指针位移折算成新宽度，写进 User*Width。
// 返回 true 表示宽度真的变了（变了才需要重算一帧几何）。
func (p *Plugin) dragTo(env *pluginapi.Env, x float32) bool {
	d := p.dragging
	switch d.kind {
	case handleSidebar:
		w := clampUser(d.startW+(x-d.startX), userSidebarMin, userSidebarMax)
		if w == p.UserSidebarWidth {
			return false
		}
		p.UserSidebarWidth = w
		p.dragging.moved = true
		return true
	case handleRightbar:
		// 右栏在右边：往左拖是**变宽**，所以位移取反。
		w := clampUser(d.startW-(x-d.startX), userRightbarMin, rightbarDragCeiling(viewport(env), d.startW))
		if w == p.UserRightbarWidth {
			return false
		}
		p.UserRightbarWidth = w
		p.dragging.moved = true
		return true
	}
	return false
}

// readConfig 从壳那儿把落盘的宽度拿回来（`config.get` → ui-config.json）。
//
// 时机：壳是第一帧之后才异步取的（它的缓存里有 `Actions.ConfigGet` 的注释），
// 所以这里不是"启动读一次"，而是**前 configWaitFrames 帧里每帧问一次**。
// 三种停下：拿到过一份非空配置 / 两列都已经被用户定过 / 等够帧数。
//
// 为什么要"两列都定过就停"：下班回家把侧栏拖宽了、右栏没碰——这时还得让右栏
// 接受落盘的值，所以不能"任一列 >0 就停"（第一版就是这么写的，测试当场抓到）。
func (p *Plugin) readConfig(env *pluginapi.Env) {
	if env.Act.ConfigGet == nil || p.configLoaded || p.configPoll >= configWaitFrames {
		return
	}
	if p.UserSidebarWidth > 0 && p.UserRightbarWidth > 0 {
		p.configLoaded = true
		return
	}
	p.configPoll++
	cfg := env.Act.ConfigGet()
	if len(cfg) == 0 {
		// 壳还没拿到，或者宿主那边根本没存过：下一帧再看（最多 configWaitFrames 帧）。
		return
	}
	p.configLoaded = true
	p.applyConfig(cfg)
}

// applyConfig 把配置灌进两个字段，**只填还空着的**：
// 用户在配置到达之前就拖过的那一列，以用户的为准。
func (p *Plugin) applyConfig(cfg map[string]string) {
	if p.UserSidebarWidth <= 0 {
		if v, ok := parseWidth(cfg[cfgKeySidebar]); ok {
			p.UserSidebarWidth = clampUser(v, userSidebarMin, userSidebarMax)
		}
	}
	if p.UserRightbarWidth <= 0 {
		if v, ok := parseWidth(cfg[cfgKeyRightbar]); ok {
			// **只抬下限 300**，上限不在这里夹：solveColumns 里还有官方 70% 和
			// available（中栏 ≥400）两道约束，存量值不该因为"这里多了一条规则"被改掉。
			p.UserRightbarWidth = max(v, userRightbarMin)
		}
	}
}

// persistWidths 松手时把两栏宽度写进宿主 ui-config.json。
//
// 没拖过（moved=false）什么都不写：一次 config.set 是一次 HTTP，
// "点了一下分隔线没动"不该产生它。
func (p *Plugin) persistWidths(env *pluginapi.Env) {
	if !p.dragging.moved || env.Act.ConfigSet == nil {
		return
	}
	kv := make(map[string]string, 2)
	if p.UserSidebarWidth > 0 {
		kv[cfgKeySidebar] = formatWidth(p.UserSidebarWidth)
	}
	if p.UserRightbarWidth > 0 {
		kv[cfgKeyRightbar] = formatWidth(p.UserRightbarWidth)
	}
	if len(kv) == 0 {
		return
	}
	env.Act.ConfigSet(kv)
}

// parseWidth 解析落盘的像素值；<=0 / 空串 / 非数字都算"没配置"。
func parseWidth(s string) (float32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 32)
	if err != nil || v <= 0 {
		return 0, false
	}
	return float32(v), true
}

// formatWidth 把像素宽度写成整数字符串（配置里存 "360"，不存 "360.000000"）。
func formatWidth(v float32) string {
	return strconv.FormatFloat(float64(round32(v)), 'f', 0, 32)
}

// rightbarDragCeiling 是右栏拖动时的上限：视口 40%。
//
// 唯一一段例外：官方首次打开就占 45%，比 40% 还宽。若按下时就已经比上限宽，
// 上限抬到按下时的宽度——否则从 45% 往右拖一下，右栏会立刻塌到 40%，
// 看着像 bug。代价是这种起手状态只能收窄、不能加宽，这也正是期望行为。
func rightbarDragCeiling(vw, startW float32) float32 {
	ceiling := vw * userRightbarMaxFrac
	if startW > ceiling {
		return startW
	}
	if ceiling < userRightbarMin {
		return userRightbarMin
	}
	return ceiling
}

// clampUser 把用户拖出来的宽度夹进 [lo, hi] 并取整。
//
// 取整是为了落盘干净（ui.sidebarWidth 写 "328" 而不是 "328.00000000004"），
// 也保证三列之和与视口严格相等——与官方 clampWidth 的 Math.round 同一个理由。
func clampUser(px, lo, hi float32) float32 {
	if hi < lo {
		hi = lo
	}
	return clampWidth(px, lo, hi)
}

// drawHandles 把两根把手叠在分隔线上（绝对定位，不占布局）。
//
// 平时**不画线**：官方侧栏与主区之间本来就没有分隔线（只靠底色差），
// 平白加一条 1px 会让首屏和官方基线截图对不上。悬停才出现 2px 线，
// 拖动时 3px 且换成品牌色——加上 SizeWE 指针，这就是"这里能拖"的全部提示。
func (p *Plugin) drawHandles(f *flex.Frame, env *pluginapi.Env, cols columns) {
	c := env.Canvas
	for _, h := range p.handles(env, cols) {
		if h.width <= 0 {
			continue
		}
		var line float32
		var color ui.Color
		switch {
		case p.dragging.active && p.dragging.kind == h.kind:
			line, color = handleLineDrag, p.theme.Link
		case c != nil && (c.Hovered() == h.id || c.Pressed() == h.id):
			line, color = handleLineHover, p.theme.BorderL4
		}
		f.Column().Absolute().Left(h.x).Top(0).Width(handleWidth).FillHeight().
			Items(flex.Center).
			HitWith(h.id, handleLabel(h.kind), ui.HitOptions{
				Role:   ui.RoleSlider,
				Cursor: ui.PointerCursorResizeHorizontal,
			}).
			Children(func() {
				if line <= 0 {
					return
				}
				// 线宽显式给，不用 FillWidth：绝对盒子自身的 pref 是 0，
				// 子节点写 FillWidth 会量出 0 宽（见 shortcuts.go 的同一条注释）。
				f.Column().Width(line).FillHeight().Background(color)
			})
	}
}

// handleLabel 是把手给无障碍/读屏用的名字（也是 E2E 找它的标签）。
func handleLabel(kind string) string {
	if kind == handleRightbar {
		return "拖动调整右栏宽度"
	}
	return "拖动调整侧栏宽度"
}

// draw 只画底色：分栏几何在 children 里。
//
// 底色的画法照官方 AppFrame：`.mP3x-q_frame{background:var(--dsw-alias-bg-base)}`
// 铺满整框。这里直接落在画布上而不是开一个 Box——开 Box 会在 root 列里
// 多占一个孩子，把后面的行挤出视口（踩过：底部整行跑到窗口外面）。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	env.Canvas.Fill(env.Bounds, p.theme.BgBase)
}

// children 把 root 切成「上：三列」+「下：底部整行」两行。
//
// 官方 AppFrame 是 grid：
//
//	.mP3x-q_frame{grid-template-rows:minmax(0,1fr) auto;display:grid}
//	.mP3x-q_bottomRow{grid-column:1/-1;background:var(--dsw-alias-bg-base)}
//
// 也就是**底部条横跨三列整行**，且三列因为它的高度而变矮。
// 这里用等价的 Column(主体 Grow(1) + 底部定高) 表达，
// 而不是把底部条绝对定位——绝对定位的元素不参与父级布局，
// 先前那一版状态栏因此既没从 x=0 开始，也没把三列压矮。
func (p *Plugin) children(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	// 先补配置：壳可能是第一帧之后才把 ui-config.json 拿回来的，
	// 而这一帧的三列宽度马上就要用到它（必须在 layout 之前）。
	p.readConfig(env)
	// 一帧只算一次：三列宽度必须来自同一次求解，
	// 否则右栏被判定「放不下」而中栏还按有右栏算，中间会空一条或被挤出窗口。
	cols := p.layout(env)
	// 拖拽要在**画之前**处理：这一帧按下/拖动之后宽度已经变了，
	// 就用新宽度重算一次——否则拖动时画面永远慢一帧（看起来像鼠标和边线脱节）。
	// 把手位置用的是上一帧的几何，但那正是 PointerOn 里 target 的来处，一致。
	if p.dragHandles(env, cols) {
		cols = p.layout(env)
	}
	syncGeometry(env, cols)
	f.Column().FillWidth().FillHeight().Children(func() {
		f.Row().FillWidth().Grow(1).Children(func() {
			p.drawSidebar(f, env, drawSlot, cols)
			p.drawMain(f, env, drawSlot, cols)
			p.drawRightbar(f, env, drawSlot, cols)
			// 把手最后声明：绝对定位、画在列之后，命中区因此盖在列内容上面
			// （列右缘 4px 内那点位置归拖拽，不归悬停行）。
			p.drawHandles(f, env, cols)
		})
		p.drawBottom(f, env, drawSlot)
		// 浮层放**最后**画：viva 按绘制顺序叠，靠后的盖在靠前的上面。
		// 这一步漏了整整一轮——`ds-demo` 装配日志里明明有，界面上却什么都没有，
		// 因为没人画 shell.overlay 这个槽。
		//
		// 官方对应 `.mP3x-q_overlayLayer{position:absolute;inset:0;pointer-events:none}`：
		// 覆盖整个框架的绝对定位层。这里同样从 (0,0) 铺满整窗，而不是铺满某一列。
		if env.Has(pluginapi.SlotShellOverlay) {
			f.Row().Absolute().Left(0).Top(0).
				Width(env.Bounds.Width).Height(env.Bounds.Height).
				Children(func() {
					drawSlot(pluginapi.SlotShellOverlay)
				})
		}
	})
}

// drawSidebar 左栏：只有挂了 sidebar.* 的插件才出现。
func (p *Plugin) drawSidebar(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot), cols columns) {
	if cols.sidebar <= 0 {
		return
	}
	// 内边距照官方侧栏根：`.wtg9ga_root{padding:6px var(--dsh-sidebar-inline-padding)}`，
	// 其中 `--dsh-sidebar-inline-padding:12px`。所以上下 6、左右 12，不是四边 12。
	//
	// Shrink(0) 是必须的：侧栏是**定宽**（官方 264–420，默认 280），不参与压缩。
	// 不写它的话，中间栏 + 右栏一起超宽时 flex 会把侧栏压到 239，
	// 而侧栏内部的滚动区是按 280 算的（280-24=256），于是每一行都往中栏里
	// 探出 41px——悬停/选中底色会横穿到会话正文上（这次截图里量出来的：
	// 行背景 x=20…276，而侧栏列只到 x=247）。
	f.Column().Width(cols.sidebar).Shrink(0).FillHeight().PadX(12).PadY(6).Gap(8).
		Background(p.theme.SidebarFill).Children(func() {
		drawSlot(pluginapi.SlotSidebarHeader)
		p.prims.Divider(f)
		f.Column().Grow(1).MinHeight(120).Children(func() {
			drawSlot(pluginapi.SlotSidebarList)
		})
		if env.Has(pluginapi.SlotSidebarFooter) {
			p.prims.Divider(f)
			drawSlot(pluginapi.SlotSidebarFooter)
		}
	})
}

// drawMain 中栏：宽度必须钉死（百分比在槽位里按整窗解析，内容会跑到窗口外）。
//
// 宽度用 solveColumns 的 center，而不是「窗体减一遍侧栏右栏」——
// 官方只在没有右栏时才允许中栏低于 400，这条规则只写在求解器里。
// Grow(1) 留着当安全网：三列之和按构造等于视口，正常情况下它不起作用。
func (p *Plugin) drawMain(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot), cols columns) {
	w := cols.center
	if w <= 0 {
		w = env.MainWidth // 视口未知时的兜底（壳算好的主区宽）
	}
	f.Column().Width(w).Grow(1).FillHeight().Pad(columnPad).Gap(8).Children(func() {
		if env.Has(pluginapi.SlotMainHeader) {
			drawSlot(pluginapi.SlotMainHeader)
		}
		// body 是可伸展轨道：composer 的位置由主区几何决定，
		// 不能依赖空态/消息流占用方自己记得 Grow。
		f.Column().FillWidth().Grow(1).MinHeight(0).Children(func() {
			drawSlot(pluginapi.SlotMainBody)
		})
		drawSlot(pluginapi.SlotMainComposer)
	})
}

// drawRightbar 右栏：没有占用方、壳说收起、或**扣完中栏下限放不下**时，
// 整列不占位（官方 computeColumns 的 available < 300 → 右栏宽度 0）。
//
// 放不下时不是压到 300 硬塞：那样中栏会被挤到 400 以下，
// 而官方中栏只有在右栏整个不出现时才允许低于 400。
//
// 底色这里画 SidebarFill，和官方有一处**已知差异**：官方 `.mP3x-q_rightbarCol`
// 在 Windows 上不设背景（透出 frame 的 bg-base），面板底色归 dockkit pane 自己画。
// 我们的右栏占用方（plugins/dsrightbar）没有自己的面板框，直接画在列里，
// 所以这层底色只能由列来给——否则右栏和主区同色、边界消失。
// 真正的面板底色（pane 的 CSS）没在打包产物里找到，等 V4B 定右栏面板形态时一起收。
func (p *Plugin) drawRightbar(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot), cols columns) {
	if cols.rightbar <= 0 {
		return
	}
	f.Column().Width(cols.rightbar).FillHeight().Pad(columnPad).Gap(8).
		Background(p.theme.SidebarFill).Children(func() {
		drawSlot(pluginapi.SlotRightbar)
	})
}

// drawBottom 底部整行：占用方自己声明高度；没占用方就一行都不占。
//
// 几何照官方 `.mP3x-q_bottomRow{background:var(--dsw-alias-bg-base);grid-column:1/-1}`：
// **横跨整行**（从窗口左边缘到右边缘，不是从中栏开始），底色 bg-base，
// 高度由占用方决定（官方 slot 文档：「Its occupant controls its own height
// and visibility; the columns shrink by that height」）。
//
// 用 Column 不是 Row：底部槽里通常是「一条分隔线 + 一行内容」，
// 竖着排。用 Row 会让分隔线和内容并排，背景只剩右半截。
func (p *Plugin) drawBottom(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if !env.Has(pluginapi.SlotShellBottom) {
		return
	}
	h := env.Geometry.BottomHeight
	if h <= 0 {
		h = 24
	}
	f.Column().FillWidth().Height(h).Background(p.theme.BgBase).Children(func() {
		drawSlot(pluginapi.SlotShellBottom)
	})
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)

// _ 保证 draw / children 被引用（调试期临时）。
var _ = func() bool {
	println("DBG dsshell funcs wired")
	return true
}
