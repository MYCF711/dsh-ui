// toggle.go —— TeamModeToggle：模型/推理菜单里的 Team 开关
// （官方 `TeamModeToggle.tsx` + `TeamModeToggle.module.css`）。
//
// # 守卫（官方，顺序也一样）
//
//	addressedChild = useSession(s => s.subagent !== null)        → null  (:30-32)
//	listedChild    = useSessions(s => s.byId[sessionId]?.origin === 'subagent') → null
//	mode === undefined（投影缺失）                                → null  (:43)
//
// # 行为
//
//   - **不做乐观更新**：`checked={mode.enabled}` 是宿主值，点完只是 `pending`；
//     请求 resolve 成功才变（Store.Select 的注释里写了）。
//   - 关闭且 `modelSelection.lastUsed !== null` → 不直接关，先请求停用确认
//     （确认框因此在 shell.overlay）。
//   - 禁用条件：`locked || pending || modelSelection === undefined`。
//
// # 几何（§4.2）
//
//	.row   gap 16; min-height 34; padding 0 8; radius md; 13px/20px
//	.label inline-flex; gap 4
//	.infoButton 20×20; color label-caption; z-index 1
package dsagentteam

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

const (
	toggleMinHeight = 34 // .row min-height:34px
	togglePadH      = 8  // .row padding:0 8px
	toggleGap       = 16 // .row gap:16px
	labelGap        = 4  // .label gap:4px
	infoButtonSize  = 20 // .infoButton 20×20
	infoIconSize    = 11 // IconInfoOutlineRegular size=11
	toggleFontSize  = 13
	toggleLine      = 20
	infoTooltipMax  = 300 // Tooltip maxWidth=300
)

// drawToggle 画开关行。slot ② 的 painter。
func (p *Plugin) drawToggle(f ui.C, env *pluginapi.Env) {
	view := p.team.View()
	// 子会话（按地址或按 origin）**不渲染**开关：Team 模式只能从根会话改。
	if view.SubagentChild || view.ListedAsSubagent {
		return
	}
	// 投影缺失 = 官方 `mode === undefined`。
	if !view.ProjectionKnown {
		return
	}
	t := p.themeFor(env)
	pending := p.team.Pending()

	ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.Between).Gap(toggleGap).
		MinHeight(toggleMinHeight).PadSides(0, togglePadH, 0, togglePadH).
		Radius(t.Radii.MD).Children(func() {
		// .label：文案 + 信息按钮（信息按钮有自己的命中区，挡在 Switch 的整行命中面上）。
		ui.Row(f).Items(ui.Center).Gap(labelGap).Children(func() {
			ui.Text(f, zhModeLabel).FontSize(toggleFontSize).LineHeight(toggleLine).
				Color(t.LabelPrimary)
			infoID := env.Tree.ID(env.Feat, "team-mode-info")
			btn := ui.Row(f).Size(infoButtonSize, infoButtonSize).Center().Radius(4).
				Hit(infoID, zhModeInfoLabel)
			if btn.Clicked() {
				// Tooltip(openOnClick)：点一下展开官方整段说明。
				p.team.ToggleInfo()
			}
			btn.Children(func() {
				ui.Icon(f, infoIcon()).Size(infoIconSize, infoIconSize).Color(t.LabelCaption)
			})
		})
		// Switch：checked 是宿主值（非乐观），pending/locked/投影缺失时禁用。
		// v0.6.0 的 Switch 绑定 *bool（点击写回），所以先每帧重置成宿主值。
		p.toggleValue = view.Projection.Enabled
		sw := ui.Switch(f, &p.toggleValue, "").Label(zhModeLabel).
			Disabled(view.Locked || pending || !view.ModelSelectionKnown)
		if sw.Clicked() {
			p.team.Select(!view.Projection.Enabled)
		}
	})

	if p.team.InfoOpen() {
		// Tooltip(side=top, delayMs=300, maxWidth=300, openOnClick)：
		// 本仓没有 portal tooltip，就地画在开关下方（宽度上限 300，可换行）。
		ui.Text(f, zhModeDescription).FillWidth().MaxWidth(infoTooltipMax).
			FontSize(toggleFontSize).LineHeight(toggleLine).Color(t.LabelSecondary)
	}
	if failure := p.team.Failure(); failure != "" {
		// 官方把 mode.failed 交给 reportFailure（壳的状态栏/提示）。
		// 本仓没有那个通道，就地画一行错误——比"点了没反应"诚实。
		ui.Text(f, failure).FillWidth().FontSize(t.Fonts.Caption).Color(t.Error)
	}
}

// toggleVisible 是开关的可见性判据（守卫本身要能被单测直接钉住）。
func toggleVisible(view View) bool {
	if view.SubagentChild || view.ListedAsSubagent {
		return false
	}
	return view.ProjectionKnown
}

var _ = ui.Center
var _ = ui.Color{}
