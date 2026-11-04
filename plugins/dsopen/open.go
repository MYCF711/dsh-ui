// Package dsopen 在会话头部放一个「在工作目录里打开」的按钮。
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
//
// # 它必须走 OpenFile，不能走 OpenExternal
//
// `Actions.OpenExternal` 只打开明确的 http/https/mailto 目的地
// （`app.OpenURL` 会拒绝其余形状），把 `D:\ws` 这种裸 Windows 路径交给它，
// 真机上就是**点一次失败一次**：Viva 返回
// `app: external URL must use http, https, or mailto`，状态栏里出现一条报错，
// 文件管理器一个字都不会打开。
//
// 所以本地目录走 `Actions.OpenFile` + 显式 `file:` 链接，由壳的
// `filePathFromLink` 还原成绝对路径，目录再交给平台文件管理器。
package dsopen

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
)

// 按钮落在会话头部的哪一格：这几个数与 `plugins/dstopbar` 的 header 几何同源
// （官方 ConversationRoot.module.css，逐项抄在那里）：
//
//	header            padding:10px 28px 0 20px
//	titleRow          min-height:30px
//	headerUtilities   在 headerCorner 左边，gap:8px
//	headerCorner      28×28、margin-right:-16px
//
// 再加中栏的跨插件内边距 columnPad=12（`plugins/dsshell` 的契约）。
//
// **这不是"抄一份常量"那么简单**：Absolute 的坐标系是槽位容器（中栏列）的
// 边距盒，不是 header 的内容盒，所以两者必须一起算进来。ds-topbar 换成别的
// 头部实现时，这一组数要跟着它走——`open_header_test.go` 会在几何不符时报红。
const (
	columnPad         = 12
	headerPadTop      = 10
	headerPadRight    = 28
	titleRowHeight    = 30
	cornerWidth       = 28
	cornerNegativePad = 16
	utilitiesGap      = 8
	buttonWidth       = 56
	buttonHeight      = 24
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
	destination, ok := openDestination(env)
	if !ok {
		return
	}
	t := p.theme
	// **绝对定位，不是再声明一行**。
	//
	// 槽位拿到的是 dsshell 的中栏列（`f.Column()…Gap(8)`）。在它里面声明
	// `f.Row()` 会变成 header 后面的**另一个子项**：按钮确实画出来了，但它
	// 占掉 24+8 的流高、把正文整体往下推，而且落在 header 的分隔线之外
	// （旧实现实测 y=96.5，而标题行是 22…52——看起来就像"按钮不见了"）。
	//
	// Absolute 不参与布局流：标题行、页签、正文的几何一位都不动，
	// 绘制顺序仍在 header 之后，所以点击命中归按钮。
	box := f.Row().Absolute().
		Top(columnPad+headerPadTop+(titleRowHeight-buttonHeight)/2).
		Right(utilitiesRight(env)).
		Width(buttonWidth).Height(buttonHeight).
		Items(flex.Center).Gap(6).PadSides(8, 8, 4, 4).
		Radius(t.Radii.MD).Background(t.InputMajor).
		Hit(env.Tree.ID(env.Feat, "open-in-app"), "在工作目录里打开")
	if box.Clicked() {
		// 失败走壳的既有状态栏（`openFile` 写一句"打开文件/目录失败: …"）——
		// 这里不自己弹窗、也不改口成别的动作：一处动作失败该只有一处提示。
		_ = env.Act.OpenFile(destination)
	}
	box.Children(func() {
		f.Text("打开").FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
	})
}

// openDestination 是"这一次点击该打开哪里"的唯一判据。
//
// 三条同时成立才画按钮：
//   - 壳接了 `OpenFile`（本地路径唯一正确的出口）；
//   - `Snapshot.CWD` 是**绝对**路径（相对路径拼出来的 file 链接会被壳判成非绝对，
//     点下去只会得到一条错）；
//   - 链接造得出来（首尾空白这类形状在 fileURL 里就被拒）。
//
// 缺 CWD 只是其中一种：不知道打开哪，就不该摆一个点了没反应的按钮。
func openDestination(env *pluginapi.Env) (string, bool) {
	if env.Act.OpenFile == nil || !absoluteCWD(env.Data.CWD) {
		return "", false
	}
	destination, err := fileURL(env.Data.CWD)
	if err != nil {
		return "", false
	}
	return destination, true
}

// utilitiesRight 是按钮右缘到中栏右边缘的距离。
//
// 官方 headerUtilities 是 headerCorner 的 flex 前兄弟、gap 8，而 corner 的
// margin-right:-16 让它伸出标题行右缘 16px。于是：
//
//	corner 在场：按钮右缘 = 列右 - 40 - (28 + 8 - 16) = 列右 - 60
//	corner 不在：按钮就贴着标题行右缘 = 列右 - 40
func utilitiesRight(env *pluginapi.Env) float32 {
	right := float32(columnPad + headerPadRight)
	if rightbarCornerVisible(env) {
		right += cornerWidth + utilitiesGap - cornerNegativePad
	}
	return right
}

// rightbarCornerVisible 复刻 dstopbar.drawCorner 的可用条件：右栏槽存在、
// 壳接了开合动作、且右栏当前是收起的（展开态由右栏自己的 PanelChrome 收起）。
//
// 为什么这里要跟着算：corner 在流里占 28+8，它一消失，官方 utilities 会右移。
// 按钮不跟着走的话，右栏一开一合就会离右边多出 20px 的空隙。
func rightbarCornerVisible(env *pluginapi.Env) bool {
	return !env.Data.ShowRightbar && env.Act.ToggleRightbar != nil &&
		env.Has != nil && env.Has(pluginapi.SlotRightbar)
}
