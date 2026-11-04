// Package dsopen 在会话头部放一个「在文件管理器里打开」的按钮。
//
// 官方 `dsh-client-ui-open-in-app`：*Web "Open In..." controls: the Session-header
// split button opening the workspace directory in an installed application…*
//
// # 这一版只做"在工作目录里打开"
//
// 官方的 split button 还有第二个动作：用某个具体应用打开某个文件。
// 那需要"这台机器装了哪些应用"的知识，而宿主没有这个信息源
// （`OpenExternal` 只是把路径交给系统默认程序）。
//
// **画一个只能做一件事的按钮，比画一个假装有两个动作的分裂按钮好。**
//
// # 为什么值得有
//
// 界面里看到的路径（侧栏文件树、写过的文件）全都是**相对的**。
// 想真去看那个文件，用户得自己去文件管理器里一层层点。
// 这个按钮把那几步变成一次点击。
package dsopen

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
)

type Plugin struct {
	theme *pluginapi.Theme
}

func (*Plugin) ID() string { return "ds-open" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string { return []string{pluginapi.KeyTheme} }

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	return nil
}

func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:   "open-in-app",
		Slot: pluginapi.SlotMainHeader,
		// Order 90：靠右。会话头左边的位置该留给标题/分支这些"说明性"的东西，
		// 这是个**动作**，动作放右边是通例。
		Order: 90,
		Draw:  p.draw,
	}}
}

func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	// 没有工作目录就没有"打开哪"这个问题；没有 OpenExternal 就没有打开的手段。
	if env.Data.CWD == "" || env.Act.OpenExternal == nil {
		return
	}
	t := p.theme
	// **必须给宽度**： 把整行交给插件， 默认撑满，
	// 于是按钮会变成一条横贯整行的色带（实测过）。56 够放下两个字。
	row := f.Row().Width(56).Height(24).Items(flex.Center).Gap(6).PadSides(8, 8, 4, 4).
		Radius(t.Radii.MD).Background(t.InputMajor).
		Hit(env.Tree.ID(env.Feat, "open-in-app"), "在工作目录里打开")
	if row.Clicked() {
		// 失败走状态栏（`OpenExternal` 的错由壳统一报）——
		// 这里不自己弹窗：一处动作失败该只有一处提示。
		_ = env.Act.OpenExternal(env.Data.CWD)
	}
	row.Children(func() {
		f.Text("打开").FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
	})
}
