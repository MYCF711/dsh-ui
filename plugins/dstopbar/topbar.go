// Package dstopbar 是「主区顶部条」的移植。
//
// 官方对应物：`dsh-client-ui-layout` 的顶栏 + `dsh-client-ui-model-selection`
// （模型选择）+ `dsh-client-ui-permission-presets`（权限状态）。
//
// 顶栏回答的是「我现在在看什么」：会话标题、它属于哪个工作区、这条会话有多少条、
// 以及（宿主给了才有）模式徽标。
//
// 一条硬规矩：**契约里没有的字段一律不画**。模型名（§12 H4）与审批策略（§12 H5）
// 现在由 `pluginapi.Snapshot.Model / Permission` 带进来，但宿主的约定是
// 「空串 = 不知道」，所以空串时整段不出现——顶栏上多一个看起来很专业的假徽标，
// 比空着糟糕得多。
package dstopbar

import (
	"path"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar/viewapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 顶栏几何全部来自官方 `ui-conversation` 的 ConversationRoot.module.css
// （安装态 asar 内，模块 hash an3WzG；结构见同包 ConversationSessionHeader：
// titleRow > titleCluster > nav.crumbs，右侧 headerActions / utilities / corner）：
//
//	.an3WzG_header{box-sizing:border-box;border-bottom:.5px solid var(--dsw-alias-border-l3);
//	               grid-template-columns:auto minmax(0,1fr);min-height:76px;
//	               padding:10px 28px 0 20px;display:grid}
//	.an3WzG_header:where(:not(:has(.an3WzG_tabs))){min-height:0;padding-bottom:10px}
//	.an3WzG_headerBlank{border-bottom:none;min-height:0;padding-bottom:0}
//	.an3WzG_titleRow{min-height:30px;align-items:center;gap:0;display:flex}
//	.an3WzG_titleCluster{flex:1;align-items:center;gap:10px;min-width:0;display:flex}
//	.an3WzG_crumbs{align-items:center;gap:4px;min-width:0;display:flex;overflow:hidden}
//	.an3WzG_crumb{max-width:220px;border-radius:var(--dsw-radius-md);padding:4px 8px;
//	              font-size:14px;line-height:20px;color:var(--dsw-alias-label-tertiary)}
//	.an3WzG_crumbCurrent{color:var(--dsw-alias-label-primary);font-weight:500}
//	.an3WzG_crumbSep{color:var(--dsw-alias-label-caption);font-size:14px;line-height:20px}
//	.an3WzG_headerUtilities{flex:none;align-items:center;gap:8px;margin-left:20px}
//	.an3WzG_headerCorner{flex:none;align-items:center;margin-left:8px;margin-right:-16px}
//	html:not([data-platform=darwin]) .an3WzG_headerSessionless{padding-top:0}
//	html:not([data-platform=darwin]) .an3WzG_headerSessionless .an3WzG_titleRow{min-height:0}
//
// 三种状态（都与官方一一对应）：
//
//	有会话且有内容  10 + 30 + 10 = 50，底下一条 0.5px 的 border-l3
//	有会话但空白    10 + 30 +  0 = 40，没有下划线（headerBlank）
//	一个会话都没有   0 +  0 + 10 = 10（headerSessionless：上内边距 0、titleRow 塌成 0）
//
// 那根 `margin-right:-16px` 是给右内边距 28 用的：28-16 = 12，控件右边离列右边 12px。
// 它不是"粘到窗口边缘"的意思，照抄时必须连同 28 的右内边距一起抄。
//
// 坐标口径：壳给中栏加 columnPad=12（dsshell 的跨插件契约，见那里的注释），
// 所以本槽位的原点比官方裸列多 12px。按窗口坐标量这一行整体 +12px；
// 扣掉壳内边距后，行高/内边距/间距与官方逐项相等（量测见 tmp/v1-nav-*.png）。
const (
	// headerPad* 是官方 header 的 `padding:10px 28px 0 20px`（无 tabs 时下内边距 10）。
	headerPadTop    = 10
	headerPadRight  = 28
	headerPadBottom = 10
	headerPadLeft   = 20
	// headerRule 是官方 header 的 `border-bottom:.5px solid border-l3`。
	headerRule = 0.5

	titleRowHeight     = 30
	activeHeaderHeight = 76
	tabsHeight         = 25
	tabsMarginTop      = 10
	tabsGap            = 20
	tabRuleHeight      = 2
	titleClusterGap    = 10
	crumbsGap          = 4
	crumbMaxWidth      = 220
	crumbPadX          = 8
	crumbPadY          = 4
	cornerMarginLX     = 8
	cornerMarginRX     = -16
)

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	// headerBoxes 保存本帧关键容器几何，供官方 76/40/10px 高度回归测试使用。
	headerBoxes headerBoxes
	// views 只保存用户明确选择过的会话视图；缺省永远是 chat。
	views map[string]string
}

type headerBoxes struct {
	Root, Body, TitleRow, Tabs, ChatTab, TrajectoryTab, Corner, Divider ui.Box
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-topbar" }

// Provides 实现 pluginapi.Plugin。view store 是两个插件间的最小适配，
// 不需要改动公共 pluginapi，也不伪装成宿主 Transport 数据。
func (*Plugin) Provides() []string { return []string{viewapi.ServiceKey} }

// Requires 实现 pluginapi.Plugin。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	if p.views == nil {
		p.views = map[string]string{}
	}
	ctx.Provide(viewapi.ServiceKey, p)
	return nil
}

// Active 实现 viewapi.Store。未知/空会话不继承其他会话的选择。
func (p *Plugin) Active(sessionID string) string {
	if view := p.views[sessionID]; viewapi.Known(view) {
		return view
	}
	return viewapi.Chat
}

// Select 实现 viewapi.Store。只有两个有真实 painter 的官方视图可选。
func (p *Plugin) Select(sessionID, view string) {
	if sessionID == "" || !viewapi.Known(view) {
		return
	}
	if p.views == nil {
		p.views = map[string]string{}
	}
	p.views[sessionID] = view
}

// Panels 实现 pluginapi.Panels。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "bar", Slot: pluginapi.SlotMainHeader, Order: 10, Draw: p.draw},
	}
}

// draw 画官方的主区导航行：`面包屑` + 右侧工具区 + 最右角控件。
//
// 结构逐项照官方 ConversationSessionHeader：
//
//	titleRow ─ titleCluster ─ nav.crumbs ─ crumb / crumbSep / crumb(当前)
//	         └ headerUtilities … headerCorner
//
// 三条硬规矩：
//  1. **只画真实数据**：工作区名来自 cwd、crumb 来自会话标题、条数来自 Total、
//     模型/审批来自宿主。任何一项拿不到就整段不出现（不补占位文案）。
//  2. **不做死按钮**：官方祖先 crumb 是 `<button onClick=open(id)>`；
//     我们没有"跳到另一个会话"的 Action，所以祖先 crumb 画成纯文本，
//     不给它 HoverBackground、也不给 Hit id —— 点上去什么都不发生，
//     而不是点上去像坏了一样。
//  3. **没有会话时不画空标题**：官方 `headerSessionless .titleRow{min-height:0}`，
//     titleRow 塌成 0，只剩 header 的 10px 下内边距。
//
// 高度用 MinHeight（行）+ PadSides（内边距）撑，不用 Height + Shrink 组合：
// 后者在 viva 里会让整行塌成 0 高（见 docs/plugin-api.md §7）。
// 宽度用百分比：槽位里 f.Bounds().Width 拿到的是**整窗**宽，不是本列宽。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	t := p.theme

	title, cwd, _ := titleOf(env)
	hasSession := env.Data.Cur != "" && title != ""
	active := len(env.Data.Msgs) > 0 || env.Data.Stream != "" || env.Data.Busy

	p.headerBoxes = headerBoxes{}
	p.headerBoxes.Root = ui.Column(f).WidthPercent(100).Children(func() {
		if !hasSession {
			// 官方 sessionless：上内边距 0、titleRow 高 0、没有下划线，只剩 10px。
			p.headerBoxes.Body = ui.Row(f).WidthPercent(100).Height(headerPadBottom)
			return
		}
		if !active {
			// 官方 blank Session：隐藏标题、actions、utilities、tabs 与分割线，
			// 但保留 headerCorner（右栏展开入口）。高度仍是 10 + 30。
			p.headerBoxes.Body = ui.Column(f).WidthPercent(100).
				PadSides(headerPadTop, headerPadRight, 0, headerPadLeft).
				Children(func() {
					p.headerBoxes.TitleRow = ui.Row(f).WidthPercent(100).MinHeight(titleRowHeight).
						Items(ui.Center).Children(func() {
						ui.Spacer(f)
						p.drawCorner(f, env)
					})
				})
			return
		}

		p.headerBoxes.Body = ui.Column(f).WidthPercent(100).MinHeight(activeHeaderHeight).
			PadSides(headerPadTop, headerPadRight, 0, headerPadLeft).
			Children(func() {
				p.headerBoxes.TitleRow = ui.Row(f).WidthPercent(100).MinHeight(titleRowHeight).
					Items(ui.Center).Gap(0).Children(func() {
					p.drawTitleCluster(f, title, cwd)
					p.drawCorner(f, env)
				})
				p.drawTabs(f, env)
			})

		// 官方 header 的下划线：`.5px solid border-l3`，横跨整列（含左右内边距）。
		p.headerBoxes.Divider = ui.Row(f).WidthPercent(100).Height(headerRule).Background(t.BorderL3)
	})
}

// drawTitleCluster 画 nav.crumbs：`工作区 / 会话标题`。
func (p *Plugin) drawTitleCluster(f ui.C, title, cwd string) {
	t := p.theme
	ui.Row(f).Grow(1).Items(ui.Center).Gap(titleClusterGap).MinWidth(0).Children(func() {
		ui.Row(f).Items(ui.Center).Gap(crumbsGap).MinWidth(0).Children(func() {
			// 祖先：工作区（官方祖先 crumb 是按钮，我们无对应 Action → 纯文本）
			if ws := workspaceOf(cwd); ws != "" {
				p.crumb(f, ws, false)
				ui.Text(f, "/").FontSize(t.Fonts.Body).LineHeight(t.Fonts.SecondaryLine).
					Color(t.LabelCaption)
			}
			// 当前：会话标题（官方 crumbCurrent：primary + 500）
			p.crumb(f, title, true)
		})
	})
}

// drawTabs 画官方 ViewTabs：只有 chat / trajectory 两个有真实 painter 的页签。
// 点击只改当前会话的本地 view store，不 OpenSession、不 reload、不 Cancel、不写 draft。
func (p *Plugin) drawTabs(f ui.C, env *pluginapi.Env) {
	p.headerBoxes.Tabs = ui.Row(f).WidthPercent(100).Height(tabsHeight).
		MarginSides(tabsMarginTop, 0, 0, 0).Gap(tabsGap).Children(func() {
		p.headerBoxes.ChatTab = p.drawTab(f, env, viewapi.Chat, "对话")
		p.headerBoxes.TrajectoryTab = p.drawTab(f, env, viewapi.Trajectory, "轨迹")
	})
}

func (p *Plugin) drawTab(f ui.C, env *pluginapi.Env, view, label string) ui.Box {
	t := p.theme
	active := p.Active(env.Data.Cur) == view
	box := ui.Column(f).Height(tabsHeight).
		Hit(env.Tree.ID(env.Feat, "view-"+view), label).Children(func() {
		ui.Row(f).Grow(1).Items(ui.Center).Children(func() {
			text := ui.Text(f, label).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).
				MaxLines(1).Ellipsis()
			if active {
				text.Color(t.LabelPrimary).Weight(gfx.WeightMedium)
			} else {
				text.Color(t.LabelTertiary)
			}
		})
		rule := ui.Row(f).WidthPercent(100).Height(tabRuleHeight)
		if active {
			rule.Background(t.BorderL4)
		}
	})
	if box.Clicked() {
		p.Select(env.Data.Cur, view)
	}
	return box
}

// drawCorner 画官方 headerCorner：最右角的那一个控件（右栏开合）。
func (p *Plugin) drawCorner(f ui.C, env *pluginapi.Env) {
	t := p.theme
	ui.Row(f).Items(ui.Center).MarginSides(0, cornerMarginRX, 0, cornerMarginLX).
		Children(func() {
			// slot 不存在时开关没有可见效果，必须整颗隐藏；blank Session 也只在
			// action 与 slot 都真实存在时保留官方 corner 入口。
			available := !env.Data.ShowRightbar && env.Act.ToggleRightbar != nil &&
				env.Has != nil && env.Has(pluginapi.SlotRightbar)
			if !available {
				return
			}
			// 官方 ExpandButton 只在收起时出现；展开态由 ds-rightbar 的 PanelChrome 收起。
			p.headerBoxes.Corner = ui.Row(f).Width(28).Height(28).Radius(t.Radii.SM).
				Items(ui.Center).Justify(ui.Center).Hover(ui.Look{}.Background(t.Hover)).
				Hit(env.Tree.ID(env.Feat, "toggle-rightbar"), "打开右侧边栏").Children(func() {
				ui.Icon(f, rightbarPanelIcon).Width(15).Height(15).Color(t.LabelSecondary)
			})
			if p.headerBoxes.Corner.Clicked() {
				env.Act.ToggleRightbar()
			}
		})
}

// rightbarPanelIcon 与官方 ExpandButton 的镜像 PanelLeft 同属 panel-right 形状：
// 24 viewBox、2px stroke、圆角外框 + 右侧栏分隔线。
var rightbarPanelIcon = gfx.Icon{
	ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
	Commands: []gfx.PathCommand{
		gfx.MoveTo(5, 3), gfx.LineTo(19, 3), gfx.QuadTo(21, 3, 21, 5),
		gfx.LineTo(21, 19), gfx.QuadTo(21, 21, 19, 21), gfx.LineTo(5, 21),
		gfx.QuadTo(3, 21, 3, 19), gfx.LineTo(3, 5), gfx.QuadTo(3, 3, 5, 3), gfx.ClosePath(),
		gfx.MoveTo(15, 3), gfx.LineTo(15, 21),
	},
}

// crumb 画一枚官方面包屑。
//
// 官方：`padding:4px 8px;border-radius:radius-md;max-width:220px;
// font-size:14px;line-height:20px;color:label-tertiary`，
// 当前项 `color:label-primary;font-weight:500`，超长省略。
func (p *Plugin) crumb(f ui.C, text string, current bool) {
	t := p.theme
	b := ui.Text(f, text).
		FontSize(t.Fonts.Body).LineHeight(t.Fonts.SecondaryLine).
		MaxWidth(crumbMaxWidth).MaxLines(1).Ellipsis().
		PadSides(crumbPadY, crumbPadX, crumbPadY, crumbPadX).
		Radius(t.Radii.MD)
	if current {
		b.Color(t.LabelPrimary).Weight(gfx.WeightMedium)
		return
	}
	// 祖先 crumb 没有可用的跳转 Action：保持官方底色/字号，但不给悬停反馈。
	b.Color(t.LabelTertiary)
}

// titleOf 从快照里找出当前会话的标题 / cwd / mode。
//
// 顺序：会话标题 → cwd 最后一段 → 空串。空串由调用方翻译成「未选择会话」，
// 这里不编占位文案（编了就没法区分"没有会话"和"会话标题刚好叫这个"）。
func titleOf(env *pluginapi.Env) (title, cwd, mode string) {
	if env.Data.Cur == "" {
		return "", "", ""
	}
	for _, s := range env.Data.Sessions {
		if s.ID != env.Data.Cur {
			continue
		}
		title = strings.TrimSpace(s.Title)
		cwd = s.Cwd
		mode = strings.TrimSpace(s.Mode)
		if title == "" {
			title = workspaceOf(cwd) // 没标题就退到工作区名，总比空白强
		}
		return title, cwd, mode
	}
	return "", "", ""
}

// workspaceOf 把 cwd 变成短名（顶栏放不下全路径）。
func workspaceOf(cwd string) string {
	c := strings.TrimRight(strings.TrimSpace(cwd), `/\`)
	if c == "" {
		return ""
	}
	base := path.Base(strings.ReplaceAll(c, `\`, "/"))
	if base == "." || base == "/" {
		return c
	}
	return base
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)
