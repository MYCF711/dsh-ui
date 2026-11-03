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
	"path"
	"sort"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
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
	collapsed   map[string]bool
	collapseAll bool
	width       float32
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
		p.collapseAll = true
	}
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
	}
}

// drawHeader 画「新会话 + 搜索」。
func (p *Plugin) drawHeader(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	f.Row().FillWidth().Height(40).Items(flex.Center).Gap(8).Children(func() {
		if f.Button(env.Tree.ID(env.Feat, "new"), "新会话").Small().Clicked() {
			if env.Act.NewSession != nil {
				env.Act.NewSession()
			}
		}
		f.Spacer()
		if f.Button(env.Tree.ID(env.Feat, "collapse"), "全部收起").Small().Ghost().Clicked() {
			// 一键把当前所有分组收起来：分组多的时候比一个个点快。
			for k, v := range p.collapsed {
				p.collapsed[k] = !v
			}
		}
	})
	f.Field(&p.search, env.Tree.ID(env.Feat, "search"), "搜索", "过滤会话…").Small().
		FillWidth().FontSize(t.Fonts.Caption)
}

// drawList 画分好组的会话列表。
func (p *Plugin) drawList(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	q := strings.ToLower(strings.TrimSpace(p.search.Text()))

	groups, order := groupByWorkspace(env.Data.Sessions, q)
	p.lastCur = env.Data.Cur
	p.act = env.Act
	p.lastOrder = p.lastOrder[:0]
	for _, key := range order {
		if p.collapsed[key] || p.collapseAll {
			continue // 收起的分组里的会话不参与键盘导航
		}
		for i := range groups[key] {
			p.lastOrder = append(p.lastOrder, groups[key][i].ID)
		}
	}

	// 滚动区宽度要留出 20px 给滚动条 + 容器内边距，否则最后一行的背景
	// 会盖住滚动条（踩过一次，视觉上像"行被切了"）。
	// 宽度必须钉死：ScrollArea 里用 FillWidth 时内容宽度算不出来，
	// 标题会被压成竖排（第一次踩），或者整个内容列撑到窗口宽（第二次踩）。
	inner := p.width - 24 // 扣掉左右各 12 的列内边距
	scrID := env.Tree.ID(env.Feat, "sidebar-scroll")
	f.ScrollArea(scrID).Width(inner).Grow(1).MinHeight(120).Gap(2).Clip().Children(func() {
		f.Column().Width(inner).Gap(2).Children(func() {
			if len(order) == 0 {
				f.Text("没有匹配的会话").FontSize(t.Fonts.Caption).Color(t.LabelCaption).Pad(8)
				return
			}
			for _, key := range order {
				sessions := groups[key]
				open := !p.collapsed[key] && !p.collapseAll
				gid := env.Tree.ID(env.Feat, "g-"+key)
				if p.prims.SectionHeader(f, gid, displayWorkspace(key), len(sessions), open).Clicked() {
					p.collapsed[key] = open // 点一下翻转
				}
				if !open {
					continue
				}
				for i := range sessions {
					p.drawSession(f, env, sessions[i])
				}
			}
		})
	})
}

// drawSession 画一条会话行：标题 + 相对时间，选中态带一个圆点。
func (p *Plugin) drawSession(f *flex.Frame, env *pluginapi.Env, s apiclient.Session) {
	t := p.theme
	active := s.ID == env.Data.Cur
	title := s.Title
	if strings.TrimSpace(title) == "" {
		title = "（无标题）"
	}
	id := env.Tree.ID(env.Feat, "s-"+s.ID)

	row := p.prims.ListRow(f, id, active, 34, func() {
		f.Column().Grow(1).Gap(1).Children(func() {
			f.Text(title).FontSize(t.Fonts.Secondary).Ellipsis().
				Color(cond(active, t.LabelPrimary, t.LabelSecondary))
			f.Text(relativeTime(s.UpdatedAt)).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		})
		if active {
			p.prims.Dot(f, t.Link, 6)
		}
	})
	if row.Clicked() && env.Act.OpenSession != nil {
		env.Act.OpenSession(s.ID)
	}
}

// groupByWorkspace 按 cwd 分组，组和组内都按最近更新降序。
//
// 返回 (组 → 会话, 组键的显示顺序)。空 cwd 归到「未指定工作区」。
func groupByWorkspace(sessions []apiclient.Session, q string) (map[string][]apiclient.Session, []string) {
	groups := map[string][]apiclient.Session{}
	var newest = map[string]string{}
	for _, s := range sessions {
		if q != "" && !strings.Contains(strings.ToLower(s.Title+" "+s.Cwd), q) {
			continue
		}
		key := workspaceKey(s.Cwd)
		groups[key] = append(groups[key], s)
		if s.UpdatedAt > newest[key] {
			newest[key] = s.UpdatedAt
		}
	}
	order := make([]string, 0, len(groups))
	for k := range groups {
		order = append(order, k)
	}
	sort.Slice(order, func(i, j int) bool {
		if newest[order[i]] != newest[order[j]] {
			return newest[order[i]] > newest[order[j]] // RFC3339 字典序 == 时间序
		}
		return order[i] < order[j]
	})
	return groups, order
}

// workspaceKey 是分组的键：cwd 去掉尾部分隔符。
func workspaceKey(cwd string) string {
	c := strings.TrimSpace(cwd)
	c = strings.TrimRight(c, `/\`)
	return c
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

// relativeTime 把 RFC3339 变成「刚刚 / 12 分钟前 / 昨天 / 3 天前」。
//
// 解析不了就原样返回——宁可显示一个丑时间，也不要空着或显示 1970。
func relativeTime(s string) string {
	if s == "" {
		return ""
	}
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		if len(s) >= 16 {
			return s[5:16] // 2016-11 这种就用原串的月日时分
		}
		return s
	}
	d := time.Since(tm)
	switch {
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

// 编译期断言：这个插件确实满足契约，漏实现会在这里炸，而不是在装配时。
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
