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
	"github.com/vivaui/viva/flex"
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

	search flex.Field
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
	}
	p.debug = *flagDebug
	p.width = 280 // 官方 cols.sidebar = 280（不含分隔线）
	return nil
}

// Fields 让壳把搜索框登记进 cfg.TextInput（不登记就收不到键盘）。
func (p *Plugin) Fields() []*flex.Field { return []*flex.Field{&p.search} }

// Width 返回侧栏占的宽度。
func (p *Plugin) Width() float32 { return p.width }

// Panels 实现 pluginapi.Panels：头部、列表两块。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "header", Slot: pluginapi.SlotSidebarHeader, Order: 10, Draw: p.drawHeader},
		{ID: "list", Slot: pluginapi.SlotSidebarList, Order: 20, Draw: p.drawList},
		{ID: "footer", Slot: pluginapi.SlotSidebarFooter, Order: 30, Draw: p.drawFooter},
	}
}

// drawHeader 画「新会话 + 搜索」。
//
// 搜索框几何照官方会话浏览器的**展开态**（`WorkspaceBrowser.module.css`）：
//
//	.GAkraW_searchExpanded{border:.5px solid var(--dsw-alias-border-l4);
//	  border-radius:var(--dsw-radius-md);width:calc(100% + 4px);height:30px;
//	  color:var(--dsw-alias-label-caption);background:0 0;margin-inline:-2px;
//	  padding:0 4px 0 0}
//	.GAkraW_searchInput{font-size:13px;line-height:18px;
//	  color:var(--dsw-alias-label-primary)}
//	.GAkraW_searchInput::placeholder{color:var(--dsw-alias-label-tertiary)}
//
// 原来是 viva 的 `.Small()`（28px 高、无描边），和官方不是一回事。
//
// 宽度 `calc(100% + 4px)` + `margin-inline:-2px` 也要照抄：官方这个框比内容列
// **左右各宽 2px**，所以它的左右边缘和下面的会话行（行宽 = 侧栏内宽 + 12）
// 不是同一根线——不抄的话搜索框会比官方窄 4px，肉眼看不出来，但按图量就对不上。
func (p *Plugin) drawHeader(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	f.Row().FillWidth().Height(38).Items(flex.Center).Gap(8).Children(func() {
		if p.prims.CircleButton(f, env.Tree.ID(env.Feat, "new"), "+", false) {
			if env.Act.NewSession != nil {
				env.Act.NewSession()
			}
		}
		f.Spacer()
		if f.Button(env.Tree.ID(env.Feat, "collapse"), "全部收起").Small().Ghost().Clicked() {
			// 一键收起/展开所有分组。必须用**独立**的 allCollapsed 开关：
			// 早先是 for k := range p.collapsed { 翻转 }，只翻已经点过的组，
			// 一个都没点过时按下去什么都不会发生（等于死按钮）。
			p.toggleAll()
		}
	})
	// 搜索框宽 = 侧栏内宽 256 + 4 = 260（官方 width:calc(100% + 4px)）。
	inner := p.width - 2*sidebarInline
	f.Field(&p.search, env.Tree.ID(env.Feat, "search"), "搜索", "过滤会话…").
		Width(inner+4).Height(searchHeight).Radius(t.Radii.MD).Border(0.5, t.BorderL4).
		MarginSides(0, -2, 0, -2).
		FontSize(t.Fonts.Secondary)
}

// drawFooter 画左栏底部那一行：会话总数 + 连接状态。
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
// 数据全部来自快照，不自己发请求。状态文字太长就省略——底部这行只用来"扫一眼"。
func (p *Plugin) drawFooter(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	f.Row().FillWidth().Height(28).Items(flex.Center).Gap(6).Children(func() {
		f.Text(itoa(len(env.Data.Sessions)) + " 个会话").
			FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		f.Spacer()
		if env.Data.Status != "" {
			f.Text(env.Data.Status).FontSize(t.Fonts.Tiny).
				Color(t.LabelTertiary).Ellipsis().MaxWidth(p.width - 120)
		}
	})
}

// drawList 画分好组的会话列表。
func (p *Plugin) drawList(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	q := normalizeQuery(p.search.Text())

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
	scrID := env.Tree.ID(env.Feat, "sidebar-scroll")
	p.logSnapshot(env, q, groups, order)
	f.ScrollArea(scrID).Width(inner).Grow(1).MinHeight(120).Clip().Children(func() {
		// 外层 Gap(0)：行间 2、组间 4 是两档，不能用一个统一的 gap 冒充
		// （official-specs §6 遗留第 2 条）。
		f.Column().Width(inner).Gap(0).PadSides(0, 0, listBottomPadding, 0).Children(func() {
			if len(order) == 0 {
				msg := "没有会话"
				if q != "" {
					msg = "没有匹配的会话"
				}
				f.Text(msg).FontSize(t.Fonts.Caption).Color(t.LabelCaption).Pad(8)
				return
			}
			for gi, key := range order {
				sessions := groups[key]
				open := p.groupOpen(key)
				if gi > 0 {
					// `.GAkraW_groupSection+.GAkraW_groupSection{margin-top:4px}`
					f.Row().Width(inner).Height(groupGap)
				}
				gid := env.Tree.ID(env.Feat, "g-"+key)
				f.Column().Width(inner).Gap(rowGap).Children(func() {
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
func (p *Plugin) drawSession(f *flex.Frame, env *pluginapi.Env, s apiclient.Session) {
	t := p.theme
	active := s.ID == env.Data.Cur
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
		f.Row().Width(indentStep).Height(1).Shrink(0) // 官方 depth*12 的缩进
		f.Row().Width(slotWidth).Height(slotHeight).Shrink(0).
			Items(flex.Center).Justify(flex.Center).Children(func() {
			// 只有当前会话、且壳给了真值（有 turn 在跑 / 正文真在流）时才点：
			// 其他行没有任何逐会话真值，留空是唯一诚实的画法。
			if active && (env.Data.Busy || env.Data.Stream != "") {
				p.prims.Dot(f, t.Info, 6)
			}
		})
		f.Text(title).FontSize(t.Fonts.Body).LineHeight(titleLineHeight).
			Ellipsis().Grow(1).Color(t.LabelPrimary).
			MarginSides(0, titleMarginRight, 0, titleMarginLeft)
		if stamp := relativeTime(s.UpdatedAt); stamp != "" {
			f.Text(stamp).FontSize(10).LineHeight(timeLineHeight).
				Color(t.LabelTertiary).Shrink(0)
		}
	})
	if row.Clicked() && env.Act.OpenSession != nil {
		env.Act.OpenSession(s.ID)
	}
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
		if q != "" && !strings.Contains(strings.ToLower(s.Title+" "+s.Cwd), q) {
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
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	if ev.Modifiers != 0 {
		return false // 带修饰键的是别的意图（留给别的插件）
	}
	switch ev.Key {
	case ui.KeyUp:
		return p.move(-1)
	case ui.KeyDown:
		return p.move(1)
	case ui.KeyHome:
		return p.moveTo(0)
	case ui.KeyEnd:
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
	p.act.OpenSession(p.lastOrder[i])
	return true
}

var (
	_ pluginapi.Plugin     = (*Plugin)(nil)
	_ pluginapi.Panels     = (*Plugin)(nil)
	_ pluginapi.KeyHandler = (*Plugin)(nil)
)
