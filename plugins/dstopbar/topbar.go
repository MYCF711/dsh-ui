// Package dstopbar 是「主区顶部条」的移植。
//
// 官方对应物：`dsh-client-ui-layout` 的顶栏 + `dsh-client-ui-model-selection`
// （模型选择）+ `dsh-client-ui-permission-presets`（权限状态）。
//
// 顶栏回答的是「我现在在看什么」：会话标题、它属于哪个工作区、这条会话有多少条、
// 以及（宿主给了才有）模式徽标。
//
// 一条硬规矩：**契约里没有的字段一律不画**。模型名、token 用量、权限状态现在
// 都不在 `pluginapi.Snapshot` 里，所以这里一个都不出现——顶栏上多一个看起来
// 很专业的假徽标，比空着糟糕得多。
package dstopbar

import (
	"path"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
)

// barHeight 是顶栏高度。固定值：顶栏一变高，消息区就被推下去，滚动位置会跳。
const barHeight = 40

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-topbar" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

// Panels 实现 pluginapi.Panels。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "bar", Slot: pluginapi.SlotMainHeader, Order: 10, Draw: p.draw},
	}
}

// draw 画一行：标题 · 工作区 …… 共 N 条 · 模式徽标，底下一根分割线。
//
// 高度用 MinHeight + PadY 撑，不用 Height + Shrink 组合：
// 后者在 viva 里会让整行塌成 0 高（踩过，见 docs/plugin-api.md §7）。
// 宽度用百分比：槽位里 f.Bounds().Width 拿到的是**整窗**宽，不是本列宽。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme

	title, cwd, mode := titleOf(env)
	hasSession := env.Data.Cur != "" && title != ""

	f.Column().WidthPercent(100).Children(func() {
		f.Row().WidthPercent(100).MinHeight(barHeight).PadY(8).
			Items(flex.Center).Gap(8).Children(func() {
			// 标题：有会话就是标题，没有就明说没有（不编，也不整条消失——
			// 顶栏消失会让下面内容整体上移，看起来像"界面跳了一下"）。
			if hasSession {
				f.Text(title).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary).
					MaxLines(1).Ellipsis()
				if ws := workspaceOf(cwd); ws != "" {
					f.Text("·").FontSize(t.Fonts.Caption).Color(t.LabelDimmed)
					f.Text(ws).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Ellipsis()
				}
			} else {
				f.Text("未选择会话").FontSize(t.Fonts.Secondary).Color(t.LabelCaption)
			}

			f.Spacer()
			// 右栏开合：官方也能收右栏，这里给一个明显的入口（状态从快照读，不改本地）。
			// 用文字而不是三角：▸/▾ 在这个字号下几乎分不出来（实测两张图看着一样）。
			label := "轨迹 关"
			if env.Data.ShowRightbar {
				label = "轨迹 开"
			}
			if env.Act.ToggleRightbar != nil &&
				f.Button(env.Tree.ID(env.Feat, "toggle-rightbar"), label).Ghost().Small().Clicked() {
				env.Act.ToggleRightbar()
			}

			// 右：条数。只在真的加载过消息时显示，0 条不显示「共 0 条」。
			if hasSession && env.Data.Total > 0 {
				f.Text("共 " + itoa(env.Data.Total) + " 条").
					FontSize(t.Fonts.Caption).Color(t.LabelCaption)
			}
			// 模式徽标：宿主给了才画。
			if mode != "" {
				p.prims.Badge(f, mode, t.LabelTertiary)
			}
		})
		p.prims.Divider(f)
	})
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

// itoa 避免为一个数字 import strconv；这个包只依赖 flex + 契约。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
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

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)
