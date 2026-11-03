// Package dstopbar 是「主区顶部条」的移植。
//
// 官方对应物：`dsh-client-ui-layout` 的顶栏 + `dsh-client-ui-model-selection`
// （模型选择器）+ `dsh-client-ui-permission-presets`（权限状态）。
//
// 顶栏是「我现在在看什么、以什么身份看」的答案，所以只放三样东西：
// 会话标题（正在看什么）、工作区（在哪个目录里）、权限与模型（以什么跑）。
// 一行放不下就省略中间，绝不换行——顶栏变高会把消息区推下去。
package dstopbar

import (
	"path"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
)

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

// draw 画一行：标题 · 工作区 …… 权限 / 模型。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme

	title, cwd, mode := titleOf(env)
	if title == "" {
		return // 没选会话就不占位置：空顶栏比假顶栏诚实
	}

	f.Column().FillWidth().Gap(6).Children(func() {
		f.Row().FillWidth().Height(28).Gap(8).Items(flex.Center).Children(func() {
			f.Text(title).FontSize(t.Fonts.Title).Bold().
				Color(t.LabelPrimary).MaxLines(1).Ellipsis()
			if ws := workspaceOf(cwd); ws != "" {
				f.Text("·").FontSize(t.Fonts.Caption).Color(t.LabelDimmed)
				f.Text(ws).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Ellipsis()
			}
			f.Spacer()
			if mode != "" {
				p.prims.Badge(f, mode, t.LabelSecondary)
			}
			p.prims.Badge(f, "只读会话", t.LabelCaption)
		})
		p.prims.Divider(f)
	})
}

// titleOf 从快照里找出当前会话的标题 / cwd / mode。
//
// 找不到就返回空串（不编一个「未命名会话」去充数）。
func titleOf(env *pluginapi.Env) (title, cwd, mode string) {
	if env.Data.Cur == "" {
		return "", "", ""
	}
	for _, s := range env.Data.Sessions {
		if s.ID != env.Data.Cur {
			continue
		}
		return strings.TrimSpace(s.Title), s.Cwd, s.Mode
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
