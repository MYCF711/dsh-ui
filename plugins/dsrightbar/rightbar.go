// Package dsrightbar 是「右栏工具调用轨迹」的移植。
//
// 官方对应物是 `dsh-client-ui-plan` / `dsh-client-ui-goal` 那一列（任务板 + 树状图）。
// 我们的宿主侧 P0 还没做 plan/goal，所以这一版只画**真的有的数据**：
// 当前会话里发生过的工具调用，按发生顺序排成一条竖向时间线——点、线、点，
// 一眼看出「跑到第几步、哪一步炸了」。
//
// 克制的地方：没有工具调用时只画一句提示，**不画一条空线**；
// 「耗时」「token」这类宿主还没给的数字一律不显示。
package dsrightbar

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar/viewapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// maxEntries 是时间线最多画多少条。
//
// 真实会话里有 3000+ 次工具调用（实测一个 31 MB 的会话 3020 次）。
// 全画出来既没人看得过来，也会把这一列变成几千个节点。只画最近的，
// 并在顶上如实写出「更早的还有多少条」——截断必须让人看得见。
const maxEntries = 80

const shellColumnInset = 12

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
	views viewapi.Store
	// fullscreen 是右栏自己的 per-session mode；它不伪造壳的 ShowRightbar。
	fullscreen map[string]bool
	// viewData 在 main.body 的 chat 前后两个 painter 之间保存一帧原始快照。
	// 这是公共 Actions 尚无 SelectConversationView 时的最小本地适配。
	viewData *pluginapi.Snapshot
	// viewOffset 保存被覆盖前的 env.ViewportOffset（通常 nil），Order=30 恢复。
	// v0.6.0 没有 ID→ScrollState 查表，所以"把正文推到视口外"改成给一个偏移覆盖，
	// 由 dsconversation 自己在虚拟化时采用（见 pluginapi.Env.ViewportOffset）。
	viewOffset       *float32
	trajectoryOffset float32
	// turnOpen 记「哪一组 turn 被用户手动收/开过」。
	//
	// 只在本机内存里、按会话隔开：切回同一会话还是原样，进程重启就回到默认。
	// 不写 ConfigSet 是刻意的——这条是**看的时候的临时视图状态**，不是配置，
	// 写盘会让「我上次收起过」变成跨机器、跨安装的持久事实。
	turnOpen map[string]bool

	// WP-D1：主区与右栏各有真实输入框，避免同屏时共享 Field.id 抢焦点。
	// 查询、展开与选择都只作用于当前已加载 Snapshot，不触发任何历史请求。
	searchField     ui.Field
	mainSearchField ui.Field
	stepOpen        map[string]bool
	selectedTurn    map[string]string
	selectedStep    map[string]string
	lastSearch      map[string]string
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-rightbar" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives, viewapi.ServiceKey}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	p.views = ctx.Inject(viewapi.ServiceKey).(viewapi.Store)
	if p.fullscreen == nil {
		p.fullscreen = map[string]bool{}
	}
	if p.turnOpen == nil {
		p.turnOpen = map[string]bool{}
	}
	return nil
}

// RightTabTimeline 是本包注册的右栏 tab **类型** id（官方 tab 类型的 `id`）。
//
// 它同时是 keyed 座位 `sidebar.right.pane.tab` 的 key 与名录上的身份。
const RightTabTimeline = "timeline"

// Panels 实现 pluginapi.Panels。
//
// 四块画，两个座位：
//
//	sidebar.right.pane.tab（keyed）  本栏 tab 类型的**体**（key = RightTabTimeline）
//	rightbar（root single）          右栏面板本身：画 dockkit tab 条 + 派发选中那一个体
//	conversation.view（keyed）       轨迹视图（key = 'trajectory'，与 dstopbar 的页签同一名录）
//	main.body（旧槽 ×2）             三栏兼容桥：官方座位那帧不派发，桥那帧照旧工作
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		// 右栏 tab 类型 `timeline` 的体。壳投影名录时读它的 Key/Label/Order。
		{ID: "timeline-body", Slot: pluginapi.SlotRightPaneTab, Key: RightTabTimeline,
			Label: "轨迹", Order: 10, Draw: p.drawTimelineBody},
		// 右栏面板：官方 `rightbar` 座位（ui-sidebar-right 的 RightbarRoot 对应物）。
		{ID: "timeline", Slot: pluginapi.SlotRightbar, Order: 10, Draw: p.draw},
		// 官方 `conversation.view` 的 trajectory 条目：名录上的"轨迹"页签就是这条。
		{ID: "trajectory-seat", Slot: pluginapi.SlotConvView, Key: pluginapi.ViewTrajectory,
			Label: "轨迹", Order: 10, Draw: p.drawTrajectorySeat},
		{ID: "view-chat-gate", Slot: pluginapi.SlotMainBody, Order: 10, Draw: p.beginTrajectoryView},
		{ID: "trajectory-view", Slot: pluginapi.SlotMainBody, Order: 30, Draw: p.drawTrajectoryView},
	}
}

// draw 画整条右栏：PanelChrome + 分隔线 + 时间线（可滚）。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	t := p.theme
	full := p.fullscreen[env.Data.Cur]
	root := ui.Column(f).WidthPercent(100).FillHeight()
	if full {
		h := env.Bounds.Height - env.Geometry.BottomHeight
		if h <= 0 {
			h = env.Bounds.Height
		}
		// 官方 fullscreen 是 100vw、锚定右缘。相对本槽内容轨道回退壳的 12px inset，
		// 不读取或修改 cmd/app，也不猜全局窗口坐标。
		root = ui.Column(f).Absolute().Top(-shellColumnInset).Right(-shellColumnInset).
			Width(env.Bounds.Width).Height(h).Background(t.BgBase).Clip().Pad(shellColumnInset).
			Hit(env.Tree.ID(env.Feat, "rightbar-fullscreen"), "右栏全屏面板")
	}
	root.Children(func() {
		// 右栏是**选择器**：登记了 tab 类型就画官方 dockkit 的 `.tabStrip`，
		// 面板体只派发选中那一个 key（`uikernel.RightTabs` 做的名录投影）。
		//
		// 名录为空、或壳这一帧没接 keyed 派发（`Env.DrawKeyed == nil`：最小装配与
		// 单测环境）时退回 rc.2 之前的行为——直接画时间线。那是过渡桥，与
		// `uikernel.ShellOptions.BridgeLegacy` 同一性质，不是终态。
		if tabs, active := p.rightTabs(env); len(tabs) > 0 && env.DrawKeyed != nil {
			selectTab := func(string) {}
			if env.Act.SelectRightTab != nil {
				selectTab = env.Act.SelectRightTab
			}
			uikernel.DrawRightTabs(f, env, tabs, active, selectTab)
			ui.Row(f).FillWidth().Grow(1).Children(func() {
				env.DrawKeyed(f, pluginapi.SlotRightPaneTab, active)
			})
			return
		}
		p.drawTimeline(f, env, env.Data, "rightbar-scroll-", true)
	})
}

// rightTabs 取这一帧的右栏 tab 名录与激活项。
//
// 名录由壳投影（`uikernel.RightTabs`）后经 `Env.RightTabs` 交进来；本包**不查表**、
// 不排序、不算激活，只把壳说的画出来。名录取不到时返回空——调用点据此走过渡桥。
func (p *Plugin) rightTabs(env *pluginapi.Env) ([]pluginapi.Tab, string) {
	if env == nil || len(env.RightTabs) == 0 {
		return nil, ""
	}
	active := env.ActiveRightTab
	if active == "" {
		if tab, ok := uikernel.ResolveActiveRightTab(env.RightTabs, ""); ok {
			active = tab.Key
		}
	}
	return env.RightTabs, active
}

// drawTimelineBody 是 `sidebar.right.pane.tab` 上 key = timeline 的那块体。
//
// 与旧路径共用同一个 `drawTimeline`：右栏（chrome=true）与主区轨迹（chrome=false）
// 共用同一组真实工具消息，这一层只决定挂在哪。
func (p *Plugin) drawTimelineBody(f ui.C, env *pluginapi.Env) {
	p.drawTimeline(f, env, env.Data, "rightbar-scroll-", true)
}

// drawTrajectorySeat 是官方 `conversation.view` 上 key = trajectory 的那块体。
//
// 与旧三栏的 `drawTrajectoryView` 是**同一份内容的两条挂法**，互斥：
//
//	官方档：壳按 key 只派发选中的那一个 view，本函数直接画，不需要"把 chat 推到视口外"
//	旧三栏：主区正文由 dsconversation 画，只能靠 beginTrajectoryView 的视口偏移让位
//
// 两条都留着是因为档位由壳一帧一个值决定（cmd/app 的 resolve()）；哪一帧活哪一条，
// 见 conversation.go 的座位注释。
func (p *Plugin) drawTrajectorySeat(f ui.C, env *pluginapi.Env) {
	if p.views == nil || p.views.Active(env.Data.Cur) != viewapi.Trajectory {
		return
	}
	ui.Column(f).FillWidth().FillHeight().
		Hit(env.Tree.ID(env.Feat, "main-trajectory"), "主区轨迹").Children(func() {
		p.drawTimeline(f, env, env.Data, "main-trajectory-scroll-", false)
	})
}

// drawTimeline 同时服务右栏与主区 trajectory；两种容器共享同一组真实工具消息。
func (p *Plugin) drawTimeline(f ui.C, env *pluginapi.Env, data pluginapi.Snapshot, scrollKey string, chrome bool) {
	t := p.theme
	groups := groupTurns(data.Msgs)
	total := 0
	for i := range groups {
		total += len(groups[i].calls)
	}

	ui.Row(f).WidthPercent(100).MinHeight(28).Items(ui.Center).Gap(6).Children(func() {
		ui.Text(f, "轨迹").FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).Color(t.LabelTertiary)
		ui.Spacer(f)
		if total > 0 {
			ui.Text(f, itoa(total)+" 次").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
		if chrome {
			p.drawFullscreenButton(f, env)
			if env.Act.ToggleRightbar != nil {
				collapse := p.chromeButton(f, env, "collapse-rightbar", "收起右侧边栏", rightbarCollapseIcon)
				if collapse.Clicked() {
					env.Act.ToggleRightbar()
				}
			}
		}
	})
	p.prims.Divider(f)
	p.drawTrajectoryWorkspace(f, env, data, scrollKey, groups, total)
}

// drawTurnGroup 画一组 turn：可点的组头 + （展开时）这一组的工具条目。
//
// 折叠只影响**这一帧画不画条目**，不改 Snapshot、不改消息：收起时数据仍在，
// 只是没铺开。组头永远带条数，所以收起也不会让人以为"这次只有下面这些"。
func (p *Plugin) drawTurnGroup(f ui.C, env *pluginapi.Env, session, scope string, g turnGroup, last bool) {
	t := p.theme
	open := p.groupOpen(session, g.key, last)
	title := "第 " + itoa(g.turn) + " 轮 · " + itoa(len(g.calls)) + " 次工具"

	header := ui.Row(f).WidthPercent(100).MinHeight(24).Radius(t.Radii.SM).PadX(2).
		Items(ui.Center).Gap(6).Hover(ui.Look{}.Background(t.Hover)).
		// 标签就是看得见的那行字：读屏和 -keys/无头点击都按它找，
		// 不另起一个用户看不见的名字。
		Hit(env.Tree.ID(env.Feat, scope+"turn-"+g.key), title).Children(func() {
		arrow := "▸"
		if open {
			arrow = "▾"
		}
		ui.Text(f, arrow).FontSize(t.Fonts.Caption).Color(t.LabelCaption)
		ui.Text(f, title).FontSize(t.Fonts.Caption).Color(t.LabelSecondary).Ellipsis().Grow(1)
	})
	if header.Clicked() {
		p.toggleGroup(session, g.key, last)
	}
	if !open {
		return
	}
	if g.skip > 0 {
		ui.Text(f, "… 前 "+itoa(g.skip)+" 次未显示").FontSize(t.Fonts.Tiny).Color(t.LabelCaption).PadY(2)
	}
	for i := g.skip; i < len(g.calls); i++ {
		p.drawEntry(f, g.calls[i], i == len(g.calls)-1)
	}
}

// groupOpen 决定这一组现在是不是展开。
//
// 默认**只展开最后一组**（正在发生的那一轮），更早的收成一行组头——这正是
// 分组要解决的问题：一长串历史工具调用不该把当前这一轮挤到看不见的地方。
// 用户点过之后以他点的为准（同一会话内一直有效）。
func (p *Plugin) groupOpen(session, key string, last bool) bool {
	if open, ok := p.turnOpen[session+"/"+key]; ok {
		return open
	}
	return last
}

// toggleGroup 是"点了这一组的组头"：原来开就收，原来收就开。
func (p *Plugin) toggleGroup(session, key string, last bool) {
	if p.turnOpen == nil {
		p.turnOpen = map[string]bool{}
	}
	p.turnOpen[session+"/"+key] = !p.groupOpen(session, key, last)
}

// turnGroup 是一次 turn 里的全部工具调用。
type turnGroup struct {
	// key 是这一组的身份，拿来做折叠状态的下标；同一个会话里稳定。
	key string
	// turn 是这一组在**当前这份消息列表**里的序号（1 起）。
	//
	// 为什么用视图内的序号而不是宿主的 turn 数值：session.load 的消息投影里
	// 没有 turnId/turn（只有 WS 流式帧有），而首屏看到的绝大多数条目都来自 load。
	// 按用户消息切出来的序号两边都成立，宿主数值只在流式帧里出现、基数（0 还是 1）
	// 没有契约保证——拿它直接当"第 N 轮"就是在编。
	turn int
	// hostKey 是宿主真的给了的 turn 身份（Turn 优先、TurnID 次之）；空 = 没给。
	hostKey string
	calls   []apiclient.Message
	// indices 与 calls 对齐，保留消息在当前已加载 Snapshot.Msgs 里的原始下标。
	// 搜索过滤后仍用同一个 MessageKey/Time，绝不拿过滤后的下标冒充身份。
	indices []int
	// totalCalls 仅在搜索过滤时用于组头「命中/总数」；零值兼容旧测试构造的组。
	totalCalls int
	// skip 是组内被 maxEntries 砍掉的前段条数（条目数上限是全局的，
	// 一组自己就超过上限时只能组内切，切了就要说出来）。
	skip int
}

// groupTurns 把消息按 turn 切开。
//
// 切法三层，优先级从硬到软：
//
//  1. **用户消息**是 turn 的真实边界（宿主数据里一轮一定从 user 开始），
//     它同时给出界面上的"第 N 轮"序号——这是 load 投影里唯一稳定的轮次信息。
//  2. 宿主在流式帧里给了 turn（`Turn` 数值优先，`TurnID` 次之）时用它做**切分**
//     的依据：同一轮里 turn 变了就另起一组，turn 没变就并进去。
//  3. 两者都没有（load 投影、旧数据）就只能靠 1，也就是"两条用户消息之间算一轮"。
func groupTurns(msgs []apiclient.Message) []turnGroup {
	groups := make([]turnGroup, 0, len(msgs)/4)
	users, split, cur := 0, 0, -1
	for i := range msgs {
		m := msgs[i]
		if m.Role == "user" {
			users++
			split = 0
			cur = -1
			continue
		}
		if m.Role != "tool" {
			continue
		}
		host := messageTurnKey(m)
		if cur >= 0 && host != "" && groups[cur].hostKey != "" && groups[cur].hostKey != host {
			// 没有新的用户消息，但宿主说这是另一轮（load 窗口从某轮中间开始时
			// 会出现）：照样切开，序号在同一条用户消息下加一位后缀。
			split++
			cur = -1
		}
		if cur < 0 {
			g := turnGroup{key: "u" + itoa(users) + ":" + itoa(split), hostKey: host, turn: len(groups) + 1}
			groups = append(groups, g)
			cur = len(groups) - 1
		}
		if groups[cur].hostKey == "" {
			groups[cur].hostKey = host
		}
		groups[cur].calls = append(groups[cur].calls, m)
		groups[cur].indices = append(groups[cur].indices, i)
		groups[cur].totalCalls = len(groups[cur].calls)
	}
	return groups
}

// messageTurnKey 取宿主给这一条的工具轮次身份；空串 = 宿主没给，不许自己编。
func messageTurnKey(m apiclient.Message) string {
	if m.Turn != nil {
		return "n" + itoa(int(*m.Turn))
	}
	if m.TurnID != "" {
		return "id:" + m.TurnID
	}
	return ""
}

// clipGroups 把要画的条数压到 max 以内，返回留下的组和"更早的还有多少条"。
//
// 从最早的一整组开始丢，而不是把第一组切一半：组头的「K 次工具」讲的是这一轮
// 一共有几次，切一半就会报一个比真实值小的数（和 "更早的 N 次未显示" 重复计数）。
// 只有最后一组自己就超过上限时才组内切，切了多少记在 skip 里、在展开时写明。
func clipGroups(groups []turnGroup, max int) ([]turnGroup, int) {
	total := 0
	for i := range groups {
		total += len(groups[i].calls)
	}
	if total <= max {
		return groups, 0
	}
	cut, i := 0, 0
	for i < len(groups)-1 && total-cut > max {
		cut += len(groups[i].calls)
		i++
	}
	kept := append([]turnGroup(nil), groups[i:]...)
	if len(kept) == 1 {
		if n := len(kept[0].calls) - kept[0].skip; n > max {
			kept[0].skip += n - max
		}
	}
	shown := 0
	for i := range kept {
		shown += len(kept[i].calls) - kept[i].skip
	}
	return kept, total - shown
}

func (p *Plugin) drawFullscreenButton(f ui.C, env *pluginapi.Env) {
	full := p.fullscreen[env.Data.Cur]
	label, icon := "右栏全屏", rightbarFullscreenIcon
	if full {
		label, icon = "退出全屏", rightbarRestoreIcon
	}
	button := p.chromeButton(f, env, "fullscreen-rightbar", label, icon)
	if button.Clicked() {
		if p.fullscreen == nil {
			p.fullscreen = map[string]bool{}
		}
		p.fullscreen[env.Data.Cur] = !full
	}
}

func (p *Plugin) chromeButton(f ui.C, env *pluginapi.Env, name, label string, icon gfx.Icon) ui.Box {
	t := p.theme
	return ui.Row(f).Width(28).Height(28).Radius(t.Radii.SM).
		Items(ui.Center).Justify(ui.Center).Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, name), label).Children(func() {
		ui.Icon(f, icon).Width(16).Height(16).Color(t.LabelSecondary)
	})
}

// beginTrajectoryView 是 main.body 的 Order=10 适配器。公共 Snapshot/Actions 还没有
// view 字段，因此只在 chat painter 执行期间把原始消息投到视口外，Order=30 立即恢复。
// 这样 dsconversation 不生成 chat 文本/Markdown 槽位，也不会画空投影的“没有匹配”。
// composer、rightbar、statusbar 和下一帧仍读原始快照；原消息切片从不被改写。
func (p *Plugin) beginTrajectoryView(_ ui.C, env *pluginapi.Env) {
	p.viewData = nil
	p.viewOffset = nil
	if p.views == nil || p.views.Active(env.Data.Cur) != viewapi.Trajectory {
		return
	}
	data := env.Data
	p.viewData = &data
	p.viewOffset = env.ViewportOffset
	if len(env.Data.Msgs) == 0 {
		// 零值只用于通过 chat 的正常列表分支；下面的视口投影保证它不会被读取或实画。
		env.Data.Msgs = []apiclient.Message{{}}
	}
	env.Data.Stream = ""
	env.Data.Busy = true
	// 偏移给一个远大于任何内容高度的值：dsconversation 的视口虚拟化于是把每条
	// 消息都当成"在视口外"，只留 Slot，不生成文本/Markdown 槽位。
	p.trajectoryOffset = 1e9
	env.ViewportOffset = &p.trajectoryOffset
}

// drawTrajectoryView 是 main.body 的 Order=30 真实轨迹 painter。
func (p *Plugin) drawTrajectoryView(f ui.C, env *pluginapi.Env) {
	if p.viewData == nil {
		return
	}
	data := *p.viewData
	p.viewData = nil
	env.Data = data
	env.ViewportOffset = p.viewOffset
	p.viewOffset = nil
	ui.Column(f).Absolute().Left(0).Top(0).WidthPercent(100).HeightPercent(100).
		Background(p.theme.BgBase).Clip().
		Hit(env.Tree.ID(env.Feat, "main-trajectory"), "主区轨迹").Children(func() {
		p.drawTimeline(f, env, data, "main-trajectory-scroll-", false)
	})
}

var rightbarCollapseIcon = gfx.Icon{
	ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
	Commands: []gfx.PathCommand{
		gfx.MoveTo(5, 3), gfx.LineTo(19, 3), gfx.QuadTo(21, 3, 21, 5),
		gfx.LineTo(21, 19), gfx.QuadTo(21, 21, 19, 21), gfx.LineTo(5, 21),
		gfx.QuadTo(3, 21, 3, 19), gfx.LineTo(3, 5), gfx.QuadTo(3, 3, 5, 3), gfx.ClosePath(),
		gfx.MoveTo(15, 3), gfx.LineTo(15, 21),
	},
}

var rightbarFullscreenIcon = gfx.Icon{
	ViewBox: 16, Stroke: 1, Cap: gfx.CapRound, Join: gfx.JoinRound,
	Commands: []gfx.PathCommand{
		gfx.MoveTo(6, 2), gfx.LineTo(2, 2), gfx.LineTo(2, 6),
		gfx.MoveTo(10, 2), gfx.LineTo(14, 2), gfx.LineTo(14, 6),
		gfx.MoveTo(6, 14), gfx.LineTo(2, 14), gfx.LineTo(2, 10),
		gfx.MoveTo(10, 14), gfx.LineTo(14, 14), gfx.LineTo(14, 10),
	},
}

var rightbarRestoreIcon = gfx.Icon{
	ViewBox: 16, Stroke: 1, Cap: gfx.CapRound, Join: gfx.JoinRound,
	Commands: []gfx.PathCommand{
		gfx.MoveTo(2, 6), gfx.LineTo(6, 6), gfx.LineTo(6, 2),
		gfx.MoveTo(14, 6), gfx.LineTo(10, 6), gfx.LineTo(10, 2),
		gfx.MoveTo(2, 10), gfx.LineTo(6, 10), gfx.LineTo(6, 14),
		gfx.MoveTo(14, 10), gfx.LineTo(10, 10), gfx.LineTo(10, 14),
	},
}

// drawEntry 画时间线上的一条：左边「点 + 竖线」，右边两行文字。
//
// 竖线的画法：gutter 是一个竖着的 Column，里面先放一个点，再放一个
// `Width(2) + Grow(1)` 的方块——Grow(1) 在 Column 里就是「吃掉剩下的高度」，
// 于是它正好从点下面连到下一条的入口。最后一条不画线（线不能凭空延长）。
func (p *Plugin) drawEntry(f ui.C, msg apiclient.Message, last bool) {
	t := p.theme
	name, detail, failed := describe(msg)

	// 行高用 MinHeight + PadY 撑，不要写 Height(...).Shrink(0)：
	// 那组合会让整行塌成 0 高、一根线都不画（踩过，见 docs/plugin-api.md §7）。
	ui.Row(f).WidthPercent(100).Gap(8).PadY(4).Children(func() {
		ui.Column(f).Width(14).Items(ui.Center).Children(func() {
			p.prims.Dot(f, dotColor(t, failed, msg.View), 6)
			if !last {
				ui.Column(f).Width(2).Grow(1).Background(t.BorderL2)
			}
		})

		ui.Column(f).Grow(1).Gap(2).Children(func() {
			ui.Row(f).WidthPercent(100).Gap(6).Items(ui.Center).Children(func() {
				ui.Text(f, name).Mono().FontSize(t.Fonts.Caption).
					Color(t.LabelSecondary).Ellipsis().Grow(1)
				if failed {
					p.prims.Badge(f, "error", t.Error)
				}
			})
			if s := oneLine(detail, 72); s != "" {
				ui.Text(f, s).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).Ellipsis()
			}
		})
	})
}

// describe 决定一条的标题、摘要与「是不是失败」。
//
// 摘要优先级按显示契约来：View 的声明 > Summary > Args。
// 但 View.Title 通常就等于工具名，原样拼上去只会得到「bash / bash」这种废话，
// 所以**标题与工具名相同时视为没提供新信息**，继续往下取。
func describe(msg apiclient.Message) (name, detail string, failed bool) {
	name = msg.Tool
	if name == "" {
		name = "工具"
	}

	var viewTitle, viewBadge string
	if v := msg.View; v != nil {
		viewTitle, viewBadge = v.Title, v.Badge
	}
	failed = viewBadge == "error"

	switch {
	case viewTitle != "" && viewTitle != msg.Tool:
		detail = viewTitle
	case msg.Summary != "":
		detail = msg.Summary
	default:
		detail = msg.Args
	}
	return name, detail, failed
}

// dotColor 决定圆点颜色：失败红、声明 ok 绿、没声明灰。
func dotColor(t *pluginapi.Theme, failed bool, view *apiclient.ViewSpec) ui.Color {
	if failed {
		return t.Error
	}
	if view != nil && view.Badge == "ok" {
		return t.Success
	}
	return t.LabelDimmed
}

// oneLine 压成一行并截断（这一列只有两百多像素，长命令不能铺开）。
func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// itoa 让这个包不依赖 strconv（与 ds-sidebar / ds-statusbar 保持一致）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 && i > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)
