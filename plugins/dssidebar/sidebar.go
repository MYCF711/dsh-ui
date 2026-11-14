// Package dssidebar 是「左栏会话列表」这个官方部件的移植。
//
// 它是**第一个**依赖根本插件的组件：Requires ui.theme + ui.primitives，
// 自己一行色号都不写。官方的对应物是 `@deepseek-ai/dsh-client-ui-session`
// （会话列表）+ `dsh-client-ui-layout` 里那根 280px 的侧栏列。
//
// 它自己持有两块界面状态，不往壳里塞：
//   - 过滤词（搜索框内容）
//   - 哪些工作区分组被折叠了
//
// 这样壳只提供「有哪些会话」，侧栏怎么分组、怎么排序、展开还是收起，
// 都是这个插件自己的事——换一个更聪明的侧栏不需要动壳一行。
package dssidebar

import (
	"flag"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方几何常量。每一个都能在 docs/official-specs.md 里找到出处：
//
//	.wnFoaW_sessionRow{height:32px;gap:0}          ← sessionRowHeight
//	.wnFoaW_projectRow{height:34px}                ← groupRowHeight
//	.wnFoaW_sessionRow .wnFoaW_title{margin:0 6px 0 4px} ← titleMarginRight/Left
//	.wnFoaW_slot{width:16px;height:20px}           ← slotWidth/slotHeight
//	.wnFoaW_title{font-size:14px;line-height:20px} ← titleLineHeight
//	.wnFoaW_time{font-size:10px;line-height:16px}  ← timeLineHeight
//	.GAkraW_flatList>*+*{margin-top:2px}           ← rowGap
//	.GAkraW_groupSection+.GAkraW_groupSection{margin-top:4px} ← groupGap
//	.GAkraW_list{padding-bottom:16px}              ← listBottomPadding
//	.GAkraW_searchExpanded{height:30px}            ← searchHeight
//	.wtg9ga_root{--dsh-sidebar-inline-padding:12px} ← sidebarInline
//	--dsh-workspace-indent: depth * 12px           ← indentStep
//
// 搜索结果卡（官方 `Rows.module.css`，类名前缀 `wnFoaW`；只有搜索态才用）：
//
//	.wnFoaW_searchResultRow{…;min-height:48px;flex-direction:column;padding:4px 8px} ← searchResultHeight
//	.wnFoaW_searchResultTitle{…;margin-left:4px;font-size:14px;line-height:20px}    ← searchTitleLineHeight
//	.wnFoaW_searchResultMeta{…;gap:6px;margin-left:20px}                            ← searchMetaMarginLeft/Gap
//	.wnFoaW_searchResultWorkspace{font-size:12px;line-height:17px;max-width:40%}    ← searchMetaLineHeight/Percent
//	.wnFoaW_searchResultSnippet{font-size:12px;line-height:17px;flex:1}             ← 同上
//	.GAkraW_searchTree>[role=treeitem]+[role=treeitem]{margin-top:2px}              ← rowGap（复用）
//	.GAkraW_empty{padding:16px 12px;font-size:13px}                                 ← emptyStatePadY/X
const (
	sessionRowHeight  = 32
	groupRowHeight    = 34
	rowGap            = 2
	groupGap          = 4
	listBottomPadding = 16
	titleLineHeight   = 20
	timeLineHeight    = 16
	slotWidth         = 16
	slotHeight        = 20
	titleMarginLeft   = 4
	titleMarginRight  = 6
	searchHeight      = 30
	sidebarInline     = 12
	indentStep        = 12

	// 官方 searchResultRow 那一档。
	searchResultHeight    = 48
	searchTitleLineHeight = 20
	searchMetaLineHeight  = 17
	searchMetaMarginLeft  = 20
	searchMetaGap         = 6
	searchWorkspaceMaxPct = 40
	emptyStatePadX        = 12
	emptyStatePadY        = 16
)

// 官方兜底文案（`dsh-client-ui-workspace` 的 zh 字典原文）。
//
// 卡片上的空值**必须**用这两条，而不是我们分组态那套自造词：官方 JSX 写死了
// `result.title || t("session.untitled")` 与 `result.workspace || t("group.ungrouped")`。
const (
	officialUntitled    = "未命名"   // t("session.untitled")
	officialUngrouped   = "未分组"   // t("group.ungrouped")
	officialSearchEmpty = "无匹配会话" // t("search.noMatches")
)

// 两个调试 flag 注册在**本插件**里，不在 cmd/app：
//   - 侧栏的过滤词只有插件自己知道，壳不该为了截一张图去碰另一层的状态；
//   - 名字都带 sidebar- 前缀，和壳的 -search（会话内搜索）不冲突。
//
// -sidebar-filter 走的是和用户打字**完全相同**的路径（Field.SetText → drawList
// 读 p.search.Text()），不是另开一条"假装过滤"的分支。
var (
	flagFilter = flag.String("sidebar-filter", "",
		"调试：启动时把这段文字填进侧栏搜索框（走与用户输入相同的过滤路径）")
	flagDebug = flag.Bool("sidebar-debug", false,
		"调试：侧栏数据变化时打印一行快照（会话数/过滤词/当前会话/选中行），用于真机取证")
)

// Plugin 实现 pluginapi.Plugin + Panels + 字段登记。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	search         ui.Field
	searchExpanded bool
	frameBoxes     frameBoxes
	// scrollBox 是本帧的会话列表滚动区。v0.6.0 的滚动状态由声明方持有
	// （没有 ID→ScrollState 查表），测试与内部要按 Box 取状态，所以记下它。
	scrollBox   ui.Box
	activePanel string
	// pendingDelete 是"已经点过一次 ×、等第二次确认"的会话 id。
	//
	// 删除是**破坏性**的（宿主那边是从磁盘上删），所以做成两次点击。
	// 为什么不弹确认框：这个界面里没有对话框，为了一个确认去造一个组件
	// 不比"同一行再点一次"更好——而且两次点的都是同一个位置，误触的概率更低。
	pendingDelete string
	// extensionRoutes 只接收当前实际存在、带 Draw 的扩展面板；主题鱼/压缩没有硬编码入口。
	extensionRoutes []panelRoute
	// lastOrder / lastCur / act 是绘制期留下的快照，按键时要用：
	// "下一个会话"是哪个只有分组和过滤之后才知道，壳不该替它算。
	lastOrder []string
	lastCur   string
	act       pluginapi.Actions
	// CollapseAll 是调试开关：Register 时把所有分组置为收起。
	// 只为"无头截图验证折叠态"存在，正常运行一直是 false。
	CollapseAll bool
	// collapsed 是"每个工作区分组自己收没收"；allCollapsed 是"全部收起"
	// 那个按钮的状态。两者分开：按钮不该把已经有状态的组翻一半。
	collapsed    map[string]bool
	allCollapsed bool
	// known 是最近一帧出现过的组键。"全部收起"要落到每一组上，否则
	// 点开某一组时其余组会一起冒出来（它们只是没被标记过，并不是真的收起）。
	known []string
	width float32
	// debug 打开时，列表数据一变就打印一行快照（-sidebar-debug）。
	// lastSig 是上一次打印过的指纹，用来避免每帧刷屏。
	debug   bool
	lastSig string

	// ── 官方任务管理页的本地状态（schedule_page.go）──
	//
	// 这些都是**界面自己的状态**，不进壳：筛选词、选中任务、详情 tab、
	// 删除上膛、记录展开。官方的 TaskManagerPage 同样把它们放在组件本地。
	schedSearch       ui.Field
	schedFilter       string // active(默认) / paused / ended / all
	schedSelected     string
	schedTab          string // rule / records
	schedDeleteArm    bool   // 删除按钮第一次点击后的"上膛"态
	schedExpanded     map[string]bool
	schedActionNotice string
	schedNoticeUntil  time.Time
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-sidebar" }

// Provides 实现 pluginapi.Plugin：侧栏目前不对外提供能力。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin：没有根本依赖插件它什么都画不了。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	p.collapsed = map[string]bool{}
	if p.CollapseAll {
		p.allCollapsed = true
	}
	// 调试入口：预填搜索词。放在 Register 而不是 drawList 里，是为了不每帧
	// 覆盖用户正在敲的内容（否则搜索框永远只能显示这一个词）。
	if *flagFilter != "" {
		p.search.SetText(*flagFilter)
		p.searchExpanded = true
	}
	p.debug = *flagDebug
	p.width = 280 // 官方 cols.sidebar = 280（不含分隔线）
	return nil
}

// Fields 让壳把搜索框登记进 cfg.TextInput（不登记就收不到键盘）。
//
// 任务管理页的搜索框同样要登记：它只在那个面板可见，但注册是静态的，
// 焦点由壳按"点到的输入框"分配，多一个未挂载的 Field 不会抢焦点。
func (p *Plugin) Fields() []*ui.Field { return []*ui.Field{&p.search, &p.schedSearch} }

// Width 返回侧栏占的宽度。
func (p *Plugin) Width() float32 { return p.width }

// Panels 实现 pluginapi.Panels：品牌/导航、工作区列表、footer 与真实主区路由。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "header", Slot: pluginapi.SlotSidebarHeader, Order: 10, Draw: p.drawHeader},
		{ID: "list", Slot: pluginapi.SlotSidebarList, Order: 20, Draw: p.drawWorkspace},
		{ID: "footer", Slot: pluginapi.SlotSidebarFooter, Order: 30, Draw: p.drawFooter},
		{ID: "active-panel", Slot: pluginapi.SlotShellOverlay, Order: 50, Draw: p.drawActivePanel},
	}
}

// drawFooter 画左栏底部那一行：会话总数。
//
// 官方侧栏 footer（`SidebarRoot.module.css`）只给了这些：
//
//	.wtg9ga_footArea{flex-direction:column;flex:none;display:flex}
//	.wtg9ga_settingsArea,.wtg9ga_footerActions{flex:none;width:100%;min-width:0}
//	.wtg9ga_footerActions{display:flex}
//	.wtg9ga_iconButton{border-radius:var(--dsw-radius-sm);width:28px;height:28px;…}
//
// **没有高度、没有字号**（里面装的是 28px 图标按钮，高度由按钮撑）。所以这里
// 行高取按钮那一档 28，字号沿用我们原来的一档（官方未给，不另编一个）。
//
// # 为什么这一行不再写「连接状态」
//
// 这里原来画的是 `env.Data.Status`（壳的动作状态串，成功 load 一次就变成
// 「已连接 · N 条（总 M）」）。两处不对：
//
//  1. **错层**：官方连接指示的家是 `sidebar.settings`（结构树 §4 S8 原话
//     「连接指示由 settings 的连接状态驱动，不能移到底部自创 statusbar」），
//     今天由 dsstatusbar 的 `connection.go` 画在那里；
//  2. **编事实**：`Status` 是"上一次动作的文案"，不是连接层结论——壳没订阅成功时
//     它照样可以写着「已连接 · N 条」。连接三态现在从 `Snapshot.Link` 读。
//
// 所以这一行只留"会话总数"这一个它自己知道的事实。
func (p *Plugin) drawFooter(f ui.C, env *pluginapi.Env) {
	t := p.theme
	ui.Row(f).FillWidth().Height(28).Items(ui.Center).Gap(6).Children(func() {
		ui.Text(f, itoa(len(env.Data.Sessions))+" 个会话").
			FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		ui.Spacer(f)
	})
}

// drawList 画会话列表：有过滤词走**搜索结果卡**，没有才走分组树。
func (p *Plugin) drawList(f ui.C, env *pluginapi.Env) {
	q := normalizeQuery(p.search.Text())
	if q != "" {
		// 官方搜索态是另一棵平铺的树（`SearchResults` + `searchResultRow`），
		// 不是"分组列表里少了几行"：所以这里整个换掉绘制路径，连分组头都不画。
		p.drawSearch(f, env, q)
		return
	}
	t := p.theme

	groups, order := groupByWorkspace(env.Data.Sessions, q)
	p.known = append(p.known[:0], order...)
	p.lastCur = env.Data.Cur
	p.act = env.Act
	p.lastOrder = p.lastOrder[:0]
	for _, key := range order {
		if !p.groupOpen(key) {
			continue // 收起的分组里的会话不参与键盘导航
		}
		for i := range groups[key] {
			p.lastOrder = append(p.lastOrder, groups[key][i].ID)
		}
	}

	// 滚动区宽度：官方 `.wtg9ga_regionArea{margin-left:-4px;
	// margin-right:calc(-1 * var(--dsh-sidebar-inline-padding));padding-left:4px}`。
	// 净效果是**右侧那 12px 内边距被吃掉**：列表右缘贴到侧栏右缘（x=280），
	// 而不是停在 268。行行背景因此宽 268 而不是 256——差 12px，超过 8px 的
	// 几何门槛，是这次按图量出来的。
	//
	// 左侧 -4 + padding-left 4 相抵，保持 x=12 不动。
	//
	// 宽度仍然必须钉死（ScrollArea 里用 FillWidth 时内容宽度算不出来，
	// 标题会被压成竖排，或者整个内容列撑到窗口宽——都踩过）。
	inner := p.width - sidebarInline
	p.logSnapshot(env, q, groups, order)
	p.scrollBox = ui.Scroll(f).Width(inner).Grow(1).MinHeight(120).Clip()
	p.scrollBox.Children(func() {
		// 外层 Gap(0)：行间 2、组间 4 是两档，不能用一个统一的 gap 冒充
		// （official-specs §6 遗留第 2 条）。
		ui.Column(f).Width(inner).Gap(0).PadSides(0, 0, listBottomPadding, 0).Children(func() {
			if len(order) == 0 {
				// 走到这里只可能是"宿主一个会话都没有"：有过滤词时上面已经分流到
				// 搜索结果卡（那条路有官方的「无匹配会话」空态）。
				ui.Text(f, "没有会话").FontSize(t.Fonts.Caption).Color(t.LabelCaption).Pad(8)
				return
			}
			for gi, key := range order {
				sessions := groups[key]
				open := p.groupOpen(key)
				if gi > 0 {
					// `.GAkraW_groupSection+.GAkraW_groupSection{margin-top:4px}`
					ui.Row(f).Width(inner).Height(groupGap)
				}
				gid := env.Tree.ID(env.Feat, "g-"+key)
				ui.Column(f).Width(inner).Gap(rowGap).Children(func() {
					if p.prims.SectionHeader(f, gid, displayWorkspace(key), len(sessions), open).Clicked() {
						p.toggleGroup(key)
					}
					if !open {
						return
					}
					for i := range sessions {
						p.drawSession(f, env, sessions[i])
					}
				})
			}
		})
	})
}

// drawSearch 画官方搜索态：一棵**平铺**的结果树（没有工作区分组头、没有折叠）。
//
// 官方 `SearchResults` 的容器就是同一个 `.GAkraW_list`（padding-bottom:16px），
// 结果之间是 `.GAkraW_searchTree>[role=treeitem]+[role=treeitem]{margin-top:2px}`
// ——和会话行的行距同一档，所以复用 rowGap。
//
// 键盘顺序 = 这一步画出来的卡片顺序（`lastOrder`），与分组态共用同一个字段，
// HandleKey 和点击因此不需要为搜索态写第二套分支。
func (p *Plugin) drawSearch(f ui.C, env *pluginapi.Env, q string) {
	t := p.theme
	results := searchResults(env.Data.Sessions, q)
	p.lastCur = env.Data.Cur
	p.act = env.Act
	p.lastOrder = p.lastOrder[:0]
	for i := range results {
		p.lastOrder = append(p.lastOrder, results[i].ID)
	}
	p.logSearchSnapshot(env, q, results)

	// 与分组态同一套宽度口径（侧栏内宽 + 吃掉右侧 12px 内边距），见 drawList。
	inner := p.width - sidebarInline
	p.scrollBox = ui.Scroll(f).Width(inner).Grow(1).MinHeight(120).Clip()
	p.scrollBox.Children(func() {
		ui.Column(f).Width(inner).Gap(0).PadSides(0, 0, listBottomPadding, 0).Children(func() {
			if len(results) == 0 {
				// 官方 .GAkraW_empty{color:var(--dsw-alias-label-tertiary);padding:16px 12px;font-size:13px}
				// 配 `search.noMatches`「无匹配会话」。
				ui.Text(f, officialSearchEmpty).FontSize(t.Fonts.Secondary).Color(t.LabelTertiary).
					PadSides(emptyStatePadY, emptyStatePadX, emptyStatePadY, emptyStatePadX)
				return
			}
			ui.Column(f).Width(inner).Gap(rowGap).Children(func() {
				for i := range results {
					p.drawSearchCard(f, env, results[i])
				}
			})
		})
	})
}

// searchCardBoxes 是画一张结果卡时真正用到的矩形。
//
// 生产路径丢弃返回值（`p.drawSearchCard(...)` 当语句用，不分配）；测试拿它量
// 卡片内部几何：标题行 20、元信息行 17、元信息左缩进 20、workspace 上限 40%。
// 没有 snippet 真值时 Snippet 是零值盒子，所以另给一个 HasSnippet 布尔，
// 测试不需要去碰空盒子的 Rect()。
type searchCardBoxes struct {
	Card       ui.Box
	Heading    ui.Box
	Title      ui.Box
	Dot        ui.Box
	Meta       ui.Box
	Workspace  ui.Box
	Snippet    ui.Box
	HasSnippet bool
}

// drawSearchCard 画一条官方 `searchResultRow`：两行、48px 的结果卡。
//
//	button.searchResultRow              ← 卡片本体（命中区域 = 整张卡）
//	  div.searchResultHeading           ← 标题行：活动点槽 + 标题
//	  div.searchResultMeta              ← 元信息行：workspace +（有真值才有的）snippet
//
// 容器复用 dscore 的 `ListRow` 图元（悬停/选中/命中与分组行走同一条路）：
// 它给的是 radius-md(12)、padding 0 8、垂直居中，而官方 searchResultRow 是
// radius-lg(16)、padding 4px 8px、align-items:stretch——圆角差 4px、内容顶距差
// 约 1.5px，都在本轮 8px 门槛内；要对齐就得改 plugins/dscore 的图元（不在本轮
// 允许改动的路径里），所以如实记在 docs/visual-search-result.md 的缺口里。
//
// 第二行的时间**不画**：官方 searchResultRow 里没有时间元素，卡片只有
// 标题 + workspace + snippet 两行；分组行的时间不受影响。
func (p *Plugin) drawSearchCard(f ui.C, env *pluginapi.Env, r searchResult) searchCardBoxes {
	t := p.theme
	active := r.ID == env.Data.Cur
	// 与分组态同一套 ID（`s-<会话ID>`）：点击/键盘/选中都不需要第二条通路。
	id := env.Tree.ID(env.Feat, "s-"+r.ID)
	var out searchCardBoxes
	card := p.prims.ListRow(f, id, active, searchResultHeight, func() {
		ui.Column(f).FillWidth().Children(func() {
			out.Heading = ui.Row(f).FillWidth().Height(searchTitleLineHeight).Items(ui.Center).Gap(0).
				Children(func() {
					// 图标槽 16×20（官方 .wnFoaW_slot）。里面只放当前会话的正向活动真值：
					// 规则与分组行完全一致，别的卡片一律留空。
					ui.Row(f).Width(slotWidth).Height(slotHeight).Shrink(0).
						Items(ui.Center).Justify(ui.Center).Children(func() {
						if active && (env.Data.Busy || env.Data.Stream != "") {
							out.Dot = p.prims.Dot(f, t.Info, 6)
						}
					})
					out.Title = ui.Text(f, r.Title).FontSize(t.Fonts.Body).
						LineHeight(searchTitleLineHeight).Ellipsis().Grow(1).Color(t.LabelPrimary).
						MarginSides(0, titleMarginRight, 0, titleMarginLeft)
				})
			out.Meta = ui.Row(f).FillWidth().Height(searchMetaLineHeight).Items(ui.Center).
				Gap(searchMetaGap).MarginSides(0, 0, 0, searchMetaMarginLeft).Children(func() {
				out.Workspace = ui.Text(f, r.Workspace).FontSize(t.Fonts.Caption).
					LineHeight(searchMetaLineHeight).Color(t.LabelTertiary).
					MaxWidthPercent(searchWorkspaceCap(r.HasSnippet)).Shrink(0).Ellipsis()
				// snippet 是有真值才有的元素（官方 `result.snippet !== void 0`）：
				// 拿不到真值就整段不画，不留占位、不拼摘要。
				if r.HasSnippet {
					out.HasSnippet = true
					out.Snippet = ui.Text(f, r.Snippet).FontSize(t.Fonts.Caption).
						LineHeight(searchMetaLineHeight).Color(t.LabelSecondary).Grow(1).Ellipsis()
				}
			})
		})
	})
	out.Card = card
	if card.Clicked() && env.Act.OpenSession != nil {
		p.activePanel = ""
		env.Act.OpenSession(r.ID)
	}
	return out
}

// drawSession 画一条会话行：缩进 + 图标槽 + 标题 + 相对时间。
//
// 数值全部来自官方 `.wnFoaW_sessionRow`（见 prims.ListRow 的注释）：
//   - 行高 32、radius md(12)、左右内边距 8；
//   - 标题 14px/**20px**、正文色（官方对未选中行**不降级**，一直是 label-primary）；
//   - 标题 `margin:0 6px 0 4px`：左边离图标槽 4、右边离时间 6；
//   - 时间是 10px/**16px**、`--dsw-alias-label-tertiary`；
//   - 行首有一个 16×20 的图标槽 `.wnFoaW_slot`；
//   - 会话行要挂在项目行下面，官方用 `padding-inline-start:calc(8px + indent)`，
//     indent 由 JS 给 `depth * 12px`，所以这里补 12px 缩进。
//
// 图标槽里只放一个**当前行有真值**的活动点：
//
//   - 宿主 Session 没有逐会话的"运行中/未读/错误"字段，所以**别的行一律留空**，
//     绝不按标题/时间猜状态；
//   - 当前会话（active）这一行，用壳的两个真值做并集：`Busy`（有 turn 在跑）
//     与 `Stream`（正在出正文，Done 时清空）。任一非空就点一个 6px 的点。
//
// 为什么两个都要：Busy 在真机上会晚到/早退（壳在 prompt 返回后才置 true），
// 单看它会漏；Stream 只在有正文增量时非空，单看它会漏掉"只剩思考"的阶段。
// 两者都是壳给的真值，并集只会**漏报**，不会把没跑的东西画成在跑。
//
// 这个点表示的是"我现在看的这个会话正在生成"，不是"别的会话在跑"。
func (p *Plugin) drawSession(f ui.C, env *pluginapi.Env, s apiclient.Session) {
	t := p.theme
	active := s.ID == env.Data.Cur
	// 删除入口只在壳接了 DeleteSession 时才画：老壳没这条线时，
	// 画一个点了没反应的东西，比不画更让人以为"删掉了"。
	canDelete := env.Act.DeleteSession != nil
	confirming := canDelete && p.pendingDelete == s.ID
	title := strings.TrimSpace(s.Title)
	if title == "" {
		title = "（无标题）"
	}
	id := env.Tree.ID(env.Feat, "s-"+s.ID)

	// 单行：标题在左、相对时间在右（官方侧栏就是一行，两行会把列表撑到看不了几屏）。
	// 时间是 Shrink(0)：官方 `.wnFoaW_time{flex:none}`，不许被压缩——
	// 不写的话标题一长就把「分钟前」挤出右边界，看起来像被切掉半个字。
	row := p.prims.ListRow(f, id, active, sessionRowHeight, func() {
		// 缩进与图标槽都必须 Shrink(0)：标题长到被省略号截断时，flex 会把
		// 可收缩的子元素一起压——实测一个很长的标题能把 16px 的图标槽压成 0，
		// 于是这一行的标题比别的行左移 16px，整列对不齐。
		ui.Row(f).Width(indentStep).Height(1).Shrink(0) // 官方 depth*12 的缩进
		ui.Row(f).Width(slotWidth).Height(slotHeight).Shrink(0).
			Items(ui.Center).Justify(ui.Center).Children(func() {
			// 只有当前会话、且壳给了真值（有 turn 在跑 / 正文真在流）时才点：
			// 其他行没有任何逐会话真值，留空是唯一诚实的画法。
			if active && (env.Data.Busy || env.Data.Stream != "") {
				p.prims.Dot(f, t.Info, 6)
			}
		})
		ui.Text(f, title).FontSize(t.Fonts.Body).LineHeight(titleLineHeight).
			Ellipsis().Grow(1).Color(t.LabelPrimary).
			MarginSides(0, titleMarginRight, 0, titleMarginLeft)
		// 时间位在确认态下换成「确认?」：**同一个位置**，不新增宽度。
		// 换成弹窗或者把整行变色的话，行宽会跳，列表会跟着抖一下。
		if stamp := relativeTime(s.UpdatedAt); stamp != "" || confirming {
			text, color := stamp, t.LabelTertiary
			if confirming {
				text, color = "确认?", t.Error
			}
			ui.Text(f, text).FontSize(10).LineHeight(timeLineHeight).
				Color(color).Shrink(0)
		}
		if canDelete {
			p.drawDeleteBox(f, env, s, confirming)
		}
	})
	if row.Clicked() && env.Act.OpenSession != nil {
		// 打开别的会话就取消"待确认删除"：用户已经把注意力挪走了，
		// 留着一个待确认状态，下一次误点会直接删掉。
		p.pendingDelete = ""
		p.activePanel = ""
		env.Act.OpenSession(s.ID)
	}
}

// drawDeleteBox 画行尾那个 ×，并处理"两次点击"。
//
// 嵌套命中是**安全的**：事件目标是这个 × 时，外层行自己的 `Clicked()` 是 false
// （实测过，见 sidebar_test.go 的 TestDeleteBoxDoesNotAlsoOpenTheSession）。
// 所以点删除不会顺手把会话切过去。
func (p *Plugin) drawDeleteBox(f ui.C, env *pluginapi.Env, s apiclient.Session, confirming bool) {
	t := p.theme
	color := t.LabelCaption
	label := "×"
	if confirming {
		color = t.Error
	}
	box := ui.Row(f).Width(16).Height(16).Shrink(0).
		Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, "del-"+s.ID), "删除这个会话").
		Children(func() {
			ui.Text(f, label).FontSize(t.Fonts.Tiny).Color(color)
		})
	if box.Clicked() {
		if confirming {
			del := env.Act.DeleteSession
			p.pendingDelete = ""
			del(s.ID)
			return
		}
		p.pendingDelete = s.ID
		if env.Act.Notify != nil {
			env.Act.Notify("再点一次就删掉这个会话")
		}
	}
}

// searchResult 是一条搜索结果卡的数据面，逐项对齐官方 `SearchResultItem` 的
// `props.result`（`{id,title,workspace,snippet,…}`）：渲染层只认这里，不自己回头
// 去猜标题或工作区。
//
// snippet 的契约缺口（本轮查证，不许自己造）：
//
//   - 官方那条真值来自 **Host 全文检索结果页**（`deriveSearchResults` 里只有
//     `content.items` 带 `snippet`）；同一批本地元信息命中（标题/工作区名）
//     一个都没有这个字段，官方 JSX 也是 `result.snippet !== void 0` 才画。
//   - 我们的 `apiclient.Session` 只有 `ID/Title/Cwd/UpdatedAt/Mode`；
//     `pluginapi.Snapshot` 里有 `Sessions/Cur/Search/Status/Msgs/…`，
//     没有逐会话片段；`../DSH-GO/docs/transport-contract.md` 与两份
//     `testdata/api.snapshot.json` 里 grep 不到任何 `snippet`。
//   - 所以 **HasSnippet 永远是 false**：卡片不画片段行，也不拿标题/cwd 拼一个
//     假摘要。宿主将来提供这个字段时，这里是唯一的接入点（在 searchResults 里填）。
type searchResult struct {
	ID         string
	Title      string // 真值；空 → 官方 session.untitled「未命名」
	Workspace  string // 由真实 cwd 推导；空 → 官方 group.ungrouped「未分组」
	Snippet    string // 宿主真值；HasSnippet 为 true 才画
	HasSnippet bool
	UpdatedAt  string // 只给 -sidebar-debug 的取证日志用
}

// searchResults 把宿主快照变成搜索结果卡：过滤 → 按最近更新降序 → 映射成卡片数据。
//
// 过滤条件与分组态共用 matchesQuery，避免"列表里有、搜索里没有"这种两条路漂移。
// 排序与分组态共用 newerSession：官方搜索结果也是按 recency 排的
// （`deriveSearchResults` 里 `orderByRecency`）。
func searchResults(sessions []apiclient.Session, q string) []searchResult {
	matched := make([]apiclient.Session, 0, len(sessions))
	for _, s := range sessions {
		if matchesQuery(s, q) {
			matched = append(matched, s)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool { return newerSession(matched[i], matched[j]) })
	out := make([]searchResult, 0, len(matched))
	for _, s := range matched {
		out = append(out, searchResult{
			ID:        s.ID,
			Title:     cardTitle(s.Title),
			Workspace: cardWorkspace(workspaceKey(s.Cwd)),
			UpdatedAt: s.UpdatedAt,
			// Snippet / HasSnippet 故意留零值：宿主契约里没有这条真值，见类型注释。
		})
	}
	return out
}

// matchesQuery 是唯一的命中判据：标题或真实 cwd 包含过滤词（大小写不敏感）。
//
// 官方本地命中也是"标题 或 工作区名"（`sessionTitle(summary).includes(q) ||
// labelOf(summary).includes(q)`），工作区名由 cwd 推导，所以这里是等价的。
func matchesQuery(s apiclient.Session, q string) bool {
	return q == "" || strings.Contains(strings.ToLower(s.Title+" "+s.Cwd), q)
}

// cardTitle 是官方 `result.title || t("session.untitled")`。
func cardTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return officialUntitled
	}
	return title
}

// cardWorkspace 是官方 `result.workspace || t("group.ungrouped")`。
//
// 注意它与分组头用的 displayWorkspace 兜底词**不同**：分组头那套是分组态自己
// 的措辞（本轮不回退、不动），卡片这一档照官方字典原文来。
func cardWorkspace(key string) string {
	if key == "" {
		return officialUngrouped
	}
	return displayWorkspace(key)
}

// searchWorkspaceCap 是工作区名在元信息行里的宽度上限（百分比）。
//
// 官方两条规则：
//
//	.wnFoaW_searchResultWorkspace{max-width:40%;flex:none}
//	.wnFoaW_searchResultWorkspace:only-child{max-width:100%}
//
// 也就是说：**这一行只有工作区名时**（本地命中没有 snippet，正是我们唯一的情形）
// 它可以吃满整行；和 snippet 并排时才缩到 40%。
func searchWorkspaceCap(hasSnippet bool) float32 {
	if hasSnippet {
		return searchWorkspaceMaxPct
	}
	return 100
}

// groupByWorkspace 按 cwd 分组；组内、组间都按最近更新降序。
//
// 返回 (组 → 会话, 组键的显示顺序)。空 cwd 归到「未指定工作区」。
//
// 组内排序是这次补的：官方列表按 `updatedAt` 从新到旧排，不是按宿主返回的顺序。
// 不排的话，新发过消息的会话会停在列表中间——"最近用过"这件事在侧栏里就看不出来。
func groupByWorkspace(sessions []apiclient.Session, q string) (map[string][]apiclient.Session, []string) {
	groups := map[string][]apiclient.Session{}
	for _, s := range sessions {
		if !matchesQuery(s, q) {
			continue
		}
		key := workspaceKey(s.Cwd)
		groups[key] = append(groups[key], s)
	}
	order := make([]string, 0, len(groups))
	for k, list := range groups {
		sort.SliceStable(list, func(i, j int) bool { return newerSession(list[i], list[j]) })
		groups[k] = list
		order = append(order, k)
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := groups[order[i]][0], groups[order[j]][0]
		ta, oka := parseUpdated(a.UpdatedAt)
		tb, okb := parseUpdated(b.UpdatedAt)
		switch {
		case oka && okb && !ta.Equal(tb):
			return ta.After(tb)
		case a.UpdatedAt != b.UpdatedAt:
			return a.UpdatedAt > b.UpdatedAt // 解析不了时退回字典序（RFC3339 同格式可用）
		default:
			return order[i] < order[j]
		}
	})
	return groups, order
}

// newerSession 是组内排序的比较器：更新的在前；同一时刻按 ID 稳定。
func newerSession(a, b apiclient.Session) bool {
	ta, oka := parseUpdated(a.UpdatedAt)
	tb, okb := parseUpdated(b.UpdatedAt)
	switch {
	case oka && okb:
		if !ta.Equal(tb) {
			return ta.After(tb)
		}
	case a.UpdatedAt != b.UpdatedAt:
		return a.UpdatedAt > b.UpdatedAt
	}
	return a.ID < b.ID
}

// parseUpdated 解析宿主给的 updatedAt。
//
// 真宿主给 RFC3339（可能带毫秒）；解析不了就返回 false，让调用方退回字符串比较——
// 宁可排序退化，也不要把整个列表按"解析失败 1970 年"排成一团。
func parseUpdated(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if tm, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return tm, true
	}
	if tm, err := time.Parse(time.RFC3339, s); err == nil {
		return tm, true
	}
	return time.Time{}, false
}

// logSnapshot 在 -sidebar-debug 下把这一帧侧栏看到的真值打印出来。
//
// 它证明的是"侧栏画的是壳递进来的真实快照"：原始会话数、过滤词、组、每行的
// id/标题/时间、当前会话、Busy。只在内容变化时打印一次，所以正常使用不会刷屏。
func (p *Plugin) logSnapshot(env *pluginapi.Env, q string,
	groups map[string][]apiclient.Session, order []string) {
	if !p.debug {
		return
	}
	rows := 0
	var b strings.Builder
	fmt.Fprintf(&b, "会话=%d 过滤=%q 组=%d 当前=%q busy=%v 流=%d 全部收起=%v",
		len(env.Data.Sessions), q, len(order), env.Data.Cur, env.Data.Busy,
		len([]rune(env.Data.Stream)), p.allCollapsed)
	for _, key := range order {
		fmt.Fprintf(&b, " |组 %q", key)
		if !p.groupOpen(key) {
			b.WriteString("(收起)")
			continue
		}
		for _, s := range groups[key] {
			rows++
			fmt.Fprintf(&b, " |行 %q 标题=%q 时间=%q 选中=%v",
				s.ID, s.Title, s.UpdatedAt, s.ID == env.Data.Cur)
		}
	}
	if rows == 0 {
		b.WriteString(" |（无可见行）")
	}
	sig := b.String()
	if sig == p.lastSig {
		return
	}
	p.lastSig = sig
	log.Printf("ds-sidebar: 可见行=%d %s", rows, sig)
}

// logSearchSnapshot 在 -sidebar-debug 下把搜索这一帧的真值打印出来。
//
// 它证明的是"结果卡上的字都来自宿主快照"：每条结果的原样 ID、标题、由真实 cwd
// 推导的工作区、时间，以及**片段有没有真值**（没有就是 none，不是空串）。
func (p *Plugin) logSearchSnapshot(env *pluginapi.Env, q string, results []searchResult) {
	if !p.debug {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "搜索=%q 结果=%d 会话=%d 当前=%q busy=%v 流=%d",
		q, len(results), len(env.Data.Sessions), env.Data.Cur, env.Data.Busy,
		len([]rune(env.Data.Stream)))
	for _, r := range results {
		snippet := "none" // 宿主没有这条真值：画面上整段不画
		if r.HasSnippet {
			snippet = fmt.Sprintf("%q", r.Snippet)
		}
		fmt.Fprintf(&b, " |卡 %q 标题=%q 工作区=%q 时间=%q 片段=%s 选中=%v",
			r.ID, r.Title, r.Workspace, r.UpdatedAt, snippet, r.ID == env.Data.Cur)
	}
	if len(results) == 0 {
		b.WriteString(" |（无匹配）")
	}
	sig := b.String()
	if sig == p.lastSig {
		return
	}
	p.lastSig = sig
	log.Printf("ds-sidebar: %s", sig)
}

// workspaceKey 是分组的键：cwd 去掉尾部分隔符。
func workspaceKey(cwd string) string {
	c := strings.TrimSpace(cwd)
	trimmed := strings.TrimRight(c, `/\`)
	if trimmed == "" && c != "" {
		return c[:1] // POSIX / 与 Windows \ 根目录是真实 cwd，不是“未指定工作区”
	}
	return trimmed
}

// displayWorkspace 把 cwd 变成人能读的一行：只取最后一段，空值给个兜底名。
//
// 不显示全路径是有意的：官方侧栏也是短名，全路径会挤爆 280px。
func displayWorkspace(key string) string {
	if key == "" {
		return "未指定工作区"
	}
	base := path.Base(strings.ReplaceAll(key, `\`, "/"))
	if base == "." || base == "/" || base == "" {
		return key
	}
	return base
}

// relativeTime 把宿主 RFC3339 updatedAt 变成「刚刚 / 12 分钟前 / 昨天 / 3 天前」。
// 空串、解析失败和未来值都表示没有可展示的真值：整段时间隐藏，
// 不回显坏值、不截片段，也不把未来值误画成「刚刚」。
func relativeTime(s string) string {
	return relativeTimeAt(s, time.Now())
}

// relativeTimeAt 把时钟作为参数，保证边界行为可以复现测试。
func relativeTimeAt(s string, now time.Time) string {
	tm, ok := parseUpdated(s)
	if !ok {
		return ""
	}
	d := now.Sub(tm)
	switch {
	case d < 0:
		return ""
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return itoa(int(d.Minutes())) + " 分钟前"
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + " 小时前"
	case d < 48*time.Hour:
		return "昨天"
	case d < 7*24*time.Hour:
		return itoa(int(d.Hours()/24)) + " 天前"
	default:
		return tm.Format("01-02")
	}
}

func cond[T any](ok bool, a, b T) T {
	if ok {
		return a
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 && i > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg && i > 0 {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// groupOpen 表示某个工作区分组现在是不是展开的。
//
// 两个状态（每组自己的 collapsed、全局的 allCollapsed）在这里合成一个判断，
// 免得绘制、键盘导航、调试日志三处各写一遍、哪天改漏一处。
func (p *Plugin) groupOpen(key string) bool {
	return !p.collapsed[key] && !p.allCollapsed
}

// toggleGroup 是"点了这个组头"：只改这一组，并退出全局收起状态。
//
// 全局收起时所有组都是收起的，用户点某组显然是想看它——所以这一下要能直接
// 展开，而不是"先退出全局、再点一次"。
func (p *Plugin) toggleGroup(key string) {
	open := p.groupOpen(key)
	p.materializeAllCollapsed()
	p.collapsed[key] = open // 原来展开 → 收起；原来收起 → 展开
}

// toggleAll 是"全部收起 / 展开"按钮。
//
// 它把状态**落到每一组**上，而不是只翻一个全局布尔：否则"全部收起"之后再
// 点开某一组，其余组会因为各自的 collapsed 还是 false 而一起展开——看起来像
// "按钮只对第一下有效"。
func (p *Plugin) toggleAll() {
	p.materializeAllCollapsed()
	// collapsed=true 表示"收起"，所以把"现在是不是展开"原样写进去就是翻转。
	nowOpen := p.groupOpen(p.knownFirst())
	for _, k := range p.known {
		p.collapsed[k] = nowOpen
	}
}

// materializeAllCollapsed 把启动调试开关（CollapseAll / allCollapsed）
// 落成"每个组都收起"，之后所有操作只看每组自己的状态。
func (p *Plugin) materializeAllCollapsed() {
	if !p.allCollapsed {
		return
	}
	p.allCollapsed = false
	for _, k := range p.known {
		p.collapsed[k] = true
	}
}

// knownFirst 取一个已有组来推算"现在是收还是开"，空列表时按"当前是收起"处理。
func (p *Plugin) knownFirst() string {
	if len(p.known) == 0 {
		return ""
	}
	return p.known[0]
}

// normalizeQuery 把搜索框里的内容变成比较用的键：去首尾空白、转小写。
//
// 大小写不敏感是有意的：会话标题会中英混排，用户不会记得当时用的哪种大小写。
func normalizeQuery(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// HandleKey 实现 pluginapi.KeyHandler：↑↓ 上下切会话，Home/End 跳到首尾。
//
// 为什么放在侧栏插件里而不是壳里：'下一个会话'取决于分组、过滤和折叠状态，
// 只有这个插件知道。壳只负责把按键转发过来。
func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	if ev.Modifiers != 0 {
		return false // 带修饰键的是别的意图（留给别的插件）
	}
	switch ev.Key {
	case gfx.KeyEscape:
		if p.searchExpanded {
			p.search.SetText("")
			p.searchExpanded = false
			return true
		}
	case gfx.KeyUp:
		return p.move(-1)
	case gfx.KeyDown:
		return p.move(1)
	case gfx.KeyHome:
		return p.moveTo(0)
	case gfx.KeyEnd:
		return p.moveTo(len(p.lastOrder) - 1)
	}
	return false
}

// move 在当前可见顺序里移动 delta 个。
func (p *Plugin) move(delta int) bool {
	idx := -1
	for i, id := range p.lastOrder {
		if id == p.lastCur {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	return p.moveTo(idx + delta)
}

// moveTo 打开第 i 个；越界就不处理（让别的插件或系统拿这个键）。
func (p *Plugin) moveTo(i int) bool {
	if i < 0 || i >= len(p.lastOrder) || p.act.OpenSession == nil {
		return false
	}
	p.activePanel = ""
	p.act.OpenSession(p.lastOrder[i])
	return true
}

var (
	_ pluginapi.Plugin     = (*Plugin)(nil)
	_ pluginapi.Panels     = (*Plugin)(nil)
	_ pluginapi.KeyHandler = (*Plugin)(nil)
)
