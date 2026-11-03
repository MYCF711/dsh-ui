// Package dscommands 是一个**命令面板**：Ctrl+K 打开，输入过滤，回车执行。
//
// 为什么它值得先做：官方把"能做什么"放进了可搜的命令面（`dsh-client-ui-commands`），
// 而我们的动作散落在各处——切主题要重启、开右栏要记快捷键、新建会话要点侧栏的 +。
// 这些能力**都已经存在**（`Actions` 里有），缺的是"一个能查、能执行的地方"。
//
// 面板里只列**真实可执行**的命令：每条都绑到一个非 nil 的 Action。
// Action 为 nil 就不列——画出来点了没反应就是死按钮。
package dscommands

import (
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Command 是一条命令。Action 为 nil 表示"这个能力壳还没接"，那就不出现在面板里。
type Command struct {
	Name   string
	Hint   string
	Action func()
}

// Plugin 持有面板状态。
type Plugin struct {
	theme  *pluginapi.Theme
	prims  pluginapi.Primitives
	act    pluginapi.Actions
	shown  bool
	query  []rune // 输入过滤（只有一个字符串，不做多字段）
	cursor int    // 当前选中项
}

func (*Plugin) ID() string { return "ds-commands" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	// Actions 不在 Context 里（它是每帧从 Env 给的），所以命令表在 draw 时现取。
	return nil
}

// commands 按当前接到的 Actions 生成命令表。**每次打开都重新生成**，
// 这样壳后来接上的能力会立刻出现，接不上的永远不出现。
func (p *Plugin) commands() []Command {
	a := p.act // 由 draw 每帧刷新（见 draw 开头）
	var out []Command
	if a.NewSession != nil {
		out = append(out, Command{"新建会话", "开一个新会话", a.NewSession})
	}
	if a.ToggleRightbar != nil {
		out = append(out, Command{"开关右栏", "显示或收起轨迹栏", a.ToggleRightbar})
	}
	if a.Retry != nil {
		out = append(out, Command{"重试上一条", "重发最近一条用户消息", a.Retry})
	}
	if a.Cancel != nil {
		out = append(out, Command{"打断当前 turn", "停止正在跑的这一轮", a.Cancel})
	}
	return out
}

// filtered 按输入过滤（大小写不敏感、匹配名字或提示）。
func (p *Plugin) filtered() []Command {
	all := p.commands()
	q := strings.TrimSpace(strings.ToLower(string(p.query)))
	if q == "" {
		return all
	}
	var out []Command
	for _, c := range all {
		if strings.Contains(strings.ToLower(c.Name), q) || strings.Contains(strings.ToLower(c.Hint), q) {
			out = append(out, c)
		}
	}
	return out
}

func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:    "command-panel",
		Slot:  pluginapi.SlotShellOverlay,
		Order: 96, // 比快捷键面板（95）再高一档
		Draw:  p.draw,
	}}
}

// HandleKey 实现 pluginapi.KeyHandler（返回 bool）。
//
// Ctrl+K 开关；开着时 ↑↓ 选、Enter 执行、Esc 关。
// **Ctrl+K 只在 Ctrl 按下且 key 是 K 时吃**，其余一律放行。
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	// 用 F2 而不是 Ctrl+K：viva 的 OnKey **不送字母键**（字母走文本输入框），
	// 所以 Ctrl+K 这种组合在这层拿不到。键枚举只有 F1-F12/方向/Enter/Esc 等。
	if ev.Key == ui.KeyF2 {
		p.shown = !p.shown
		p.query = nil
		p.cursor = 0
		// 借 Notify 的副作用请求重绘（宿主实现是 setStatus：写状态栏 + Invalidate）。
		// pluginapi 目前没有显式的 repaint 动作，这是插件唯一能请求重绘的公开入口。
		if p.shown && p.act.Notify != nil {
			p.act.Notify("命令面板已打开 · F2 或 Esc 关闭")
		}
		return true
	}
	if !p.shown {
		return false
	}
	switch ev.Key {
	case ui.KeyEscape:
		p.shown = false
		return true
	case ui.KeyDown:
		if n := len(p.filtered()); n > 0 {
			p.cursor = (p.cursor + 1) % n
		}
		return true
	case ui.KeyUp:
		if n := len(p.filtered()); n > 0 {
			p.cursor = (p.cursor + n - 1) % n
		}
		return true
	case ui.KeyEnter:
		cs := p.filtered()
		if p.cursor >= 0 && p.cursor < len(cs) && cs[p.cursor].Action != nil {
			cs[p.cursor].Action()
			p.shown = false
		}
		return true
	}
	return false
}

// draw 用**和 dsdemo 完全相同的结构**画（Row + Absolute + Top/Right 一类），
// 因为那是本仓唯一被真机证明能出现在 shell.overlay 上的写法。
// 面板贴右上角，宽度固定、高度随条目数。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	if !p.shown {
		return
	}
	t := p.theme
	p.act = env.Act // 每帧刷新：壳可能中途接上新能力
	cs := p.filtered()
	w := float32(360)

	// **必须用 Column 而不是 Row**：Row 会把所有子元素排成一行——我第一版
	// 就是用 Row，结果面板只有一行高、下面几条全被压没了。Column 让高度由
	// 内容 pref 决定（只给 Top/Right 时宽高就是内容 pref，见 dsshortcuts 注释）。
	f.Column().Absolute().Top(56).Right(24).Width(w).
		Radius(t.Radii.Panel).Background(t.InputMajor).Border(1, t.BorderL2).
		PadSides(14, 14, 12, 14).Gap(4).Children(func() {
		f.Text("命令").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		q := string(p.query)
		if q == "" {
			q = "输入筛选…"
		}
		f.Text(q).FontSize(13).Color(t.LabelSecondary)
		if len(cs) == 0 {
			f.Text("没有匹配的命令").FontSize(13).Color(t.LabelTertiary)
		}
		for i, c := range cs {
			col := t.LabelPrimary
			if i == p.cursor {
				col = t.Info // 选中项用 Info 色，和官方下拉的高亮一致
			}
			f.Row().FillWidth().Items(flex.Center).Gap(8).Children(func() {
				f.Text(c.Name).FontSize(13).Color(col)
				f.Text(c.Hint).FontSize(11).Color(t.LabelTertiary).Grow(1)
			})
		}
	})
}
