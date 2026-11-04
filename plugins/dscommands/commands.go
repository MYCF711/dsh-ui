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
	"strconv"
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

// shortModel 去掉渠道前缀（`group/deepseek-v4.1-flash1017` → `deepseek-v4.1-flash1017`）。
//
// 命令面板一行放不下全名；而且渠道（group/streamlake/…）对用户选模型没帮助——
// 他关心的是"哪个模型"，渠道只该出现在提示里（Hint 里已经给了 OwnedBy）。
func shortModel(id string) string {
	if i := strings.Index(id, "/"); i >= 0 && i+1 < len(id) {
		return id[i+1:]
	}
	return id
}

// disambiguate 处理**不同渠道有同名模型**的情况。
//
// 实测：`group/deepseek-v4.1-flash-1017` 和 `streamlake/deepseek-v4.1-flash-1017`
// 去掉渠道前缀之后一模一样，命令面板里会出现**两条看着完全相同的「模型：…」**。
// 用户点哪个都对，但他没法知道自己在点哪个——所以重名的保留完整 id。
//
// 只有重名的才变长：绝大多数模型名是唯一的，全保留渠道会让面板变吵。
func disambiguate(ids []string) []string {
	short := make([]string, len(ids))
	count := map[string]int{}
	for i, id := range ids {
		short[i] = shortModel(id)
		count[short[i]]++
	}
	for i := range short {
		if count[short[i]] > 1 {
			short[i] = ids[i] // 重名：退回完整 id（带渠道）
		}
	}
	return short
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
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
	// 模型：每个模型一条命令。**上限 8 条**——网关上有几十个模型，
	// 全塞进命令面板会把"打断当前 turn"这种常用命令挤到看不见的地方。
	// 超出的部分用户可以在宿主侧改默认模型，这里不做分页（命令面板不是列表页）。
	if a.ModelsList != nil && a.SetModel != nil {
		set := a.SetModel
		if models := a.ModelsList(); len(models) > 0 {
			const maxModels = 8
			shown := models
			if len(shown) > maxModels {
				shown = shown[:maxModels]
			}
			labels := disambiguate(func() []string {
				ids := make([]string, len(shown))
				for i, m := range shown {
					ids[i] = m.ID
				}
				return ids
			}())
			for i, m := range shown {
				m := m // 闭包捕获（同下面权限命令的坑）
				out = append(out, Command{
					"模型：" + labels[i],
					m.OwnedBy,
					func() { set(m.ID) },
				})
			}
			// 超出的部分**不画成命令**：`Action` 为 nil 的命令点下去什么都不会发生，
			// 画出来就是在骗人。改成写进"恢复默认"的提示里——用户看得到还有多少，
			// 而且那一条本来就可点。
			hint := "回到宿主默认模型"
			if len(models) > maxModels {
				hint = "回到宿主默认模型（另有 " + itoa(len(models)-maxModels) + " 个未列出）"
			}
			out = append(out, Command{"模型：恢复默认", hint, func() { set("") }})
		}
	}

	// 布局宽度：**原地加减**而不是"打开设置面板"。
	//
	// 为什么不做设置面板：宿主的 `config.get/set` 是通用 KV，但**真正被消费的键
	// 只有两个**（`ui.sidebarWidth` / `ui.rightbarWidth`）。一个只放得下两个数字的
	// 面板是空壳，而"宽一点/窄一点"本身已经够表达意图。
	//
	// **拖动分隔线才是主路径**（dsshell 支持），这里给的是没有鼠标时的手段。
	if a.ConfigGet != nil && a.ConfigSet != nil {
		bump := func(key string, delta, fallback int) func() {
			return func() {
				cur := fallback
				if v, ok := a.ConfigGet()[key]; ok {
					if n, err := strconv.Atoi(v); err == nil && n > 0 {
						cur = n
					}
				}
				next := cur + delta
				// 和 dsshell 的拖动限制对齐：太窄放不下内容、太宽会把中栏挤没。
				if next < 180 {
					next = 180
				}
				if next > 600 {
					next = 600
				}
				a.ConfigSet(map[string]string{key: strconv.Itoa(next)})
				if a.Notify != nil {
					a.Notify(key + " = " + strconv.Itoa(next))
				}
			}
		}
		out = append(out,
			Command{"侧栏宽一点", "当前值 +40（也可以直接拖分隔线）", bump("ui.sidebarWidth", 40, 264)},
			Command{"侧栏窄一点", "当前值 -40", bump("ui.sidebarWidth", -40, 264)},
			Command{"右栏宽一点", "当前值 +40（也可以直接拖分隔线）", bump("ui.rightbarWidth", 40, 420)},
			Command{"右栏窄一点", "当前值 -40", bump("ui.rightbarWidth", -40, 420)},
		)
	}

	// 审批策略：四条命令，**值都来自宿主的白名单**（deny/ask/allow/auto）。
	//
	// 为什么不做成"一个循环切换"：用户没法从命令名看出下一个是什么。
	// 四条明写，点哪个就是哪个。第五条的"恢复默认"要单独给——
	// 没有它，用户设过一次就再也回不到宿主的 `--policy` 了。
	if a.SetPermission != nil {
		for _, c := range []struct{ label, hint, value string }{
			{"权限：每次都问我", "写类工具每次都要点允许（ask）", "ask"},
			{"权限：只读", "写类工具一律拒绝（deny）", "deny"},
			{"权限：自动允许", "写类工具不问直接跑（allow）", "allow"},
			{"权限：按模式自动", "由当前模式决定（auto）", "auto"},
			{"权限：恢复默认", "回到宿主的 --policy，不留覆盖", ""},
		} {
			c := c // 闭包捕获：不这么做四条命令都会设成最后一个值
			set := a.SetPermission
			out = append(out, Command{c.label, c.hint, func() { set(c.value) }})
		}
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
