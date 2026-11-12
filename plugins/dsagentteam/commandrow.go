// commandrow.go —— TeamCommandRow：历史 `/team` 命令回执
// （官方 `TeamCommandRow.tsx` + `.module.css`）。
//
// # 谁决定"画不画"
//
// 两条守卫，都在本文件：
//
//	① 槽位绑定 key：只有 `node.command === 'team'` 才画（官方由 ui-chat 的 key 过滤，
//	   本仓 chat 还没有命令节点类型，所以这里再守一次）；
//	② 可展开条件：`body !== '' && (失败 || 含换行)`。
//
// 历史 `/team on|off` 的**隐藏**不归本包：官方明写由 ui-chat 的可见性策略完成
// （README：“Chat visibility policy hides only historical commands named `team`”），
// 本包只负责剩下回执的渲染。
//
// # summary 规则（官方 TeamCommandRow.tsx:15-28）
//
//	无 outcome        → t('command.running')
//	success           → outcome.text || t('command.done')
//	error             → 取 outcome.text **第一行非空** → t('command.failedReason',{reason})
//	                    取不到则 t('command.failed')
//
// # 几何（§4.2 TeamCommandRow.module.css）
//
//	.root      flex-column
//	.title     font-weight:400
//	.separator 2×2; margin 0 8px; radius 1; background label-caption
//	.summary   13px; line-height 24; nowrap+ellipsis; data-error → state-error
//	.body      <pre> max-height 260; margin 4px 0 4px 4px; padding 12px 16px; radius lg;
//	           border .5px; code-block 底; 等宽；pre-wrap
package dsagentteam

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

const (
	separatorSize   = 2   // .separator 2×2
	separatorMargin = 8   // .separator margin:0 8px
	summarySize     = 13  // .summary font-size:13px
	summaryLine     = 24  // .summary line-height:calc(24px + delta)
	bodyMaxHeight   = 260 // .body max-height:260px
	bodyMarginTop   = 4
	bodyMarginLeft  = 4
	bodyPadV        = 12
	bodyPadH        = 16
	commandIconSize = 14 // IconUsersOutlineRegular size=14
)

// commandSummary 是官方 `commandSummary`（三条分支逐字）。
func commandSummary(node *CommandNode) string {
	if node == nil || node.Outcome == nil {
		return zhCommandRunning
	}
	switch node.Outcome.Kind {
	case "success":
		if node.Outcome.Text != "" {
			return node.Outcome.Text
		}
		return zhCommandDone
	case "error":
		reason := firstNonEmptyLine(node.Outcome.Text)
		if reason != "" {
			return formatMessage(zhCommandFailedWhy, reason)
		}
		return zhCommandFailed
	}
	// 官方 `assertNever(outcome.kind)`：闭集之外的状态不存在。
	return zhCommandFailed
}

// firstNonEmptyLine 取第一行**非空**文本（官方 `outcome.text?.trim().split(/\r?\n/)[0]`）。
func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line != "" {
			return line
		}
	}
	return ""
}

// commandExpandable 是官方可展开条件。
func commandExpandable(node *CommandNode) bool {
	if node == nil || node.Outcome == nil {
		return false
	}
	body := node.Outcome.Text
	if body == "" {
		return false
	}
	return node.Outcome.Kind == "error" || strings.Contains(body, "\n")
}

// DrawCommandRow 画一行命令回执。**导出**给父级（将来的 chat 命令节点渲染器）直接调用：
// 它需要的只是一个 node 与一个展开态回调，与所在容器无关。
func DrawCommandRow(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme, node *CommandNode, expanded bool, onToggle func()) {
	if node == nil {
		return
	}
	running := node.Outcome == nil
	failed := node.Outcome != nil && node.Outcome.Kind == "error"
	summary := commandSummary(node)
	expandable := commandExpandable(node)
	open := expanded && expandable

	summaryColor := t.LabelSecondary
	if running {
		// 官方 collapsedContent 用 TextShimmer：执行中的摘要比正文淡一层。
		// viva 没有 shimmer 动画，这里用第三级文字色表达"进行中"。
		summaryColor = t.LabelTertiary
	}
	if failed {
		summaryColor = t.Error
	}

	rowID := env.Tree.ID(env.Feat, "team-command-row")
	f.Column().FillWidth().Children(func() {
		// DisclosureRow：icon + title + separator + summary；整行可点（expandOnRowClick）。
		row := f.Row().FillWidth().Items(flex.Center).Hit(rowID, zhCommandTitle)
		if expandable && row.Clicked() && onToggle != nil {
			onToggle()
		}
		row.Children(func() {
			f.Icon(usersIcon()).Size(commandIconSize, commandIconSize).Color(t.LabelTertiary)
			// .title{font-weight:400}——默认就是 400，这里显式写出来防上游改默认。
			f.Text(zhCommandTitle).FontSize(summarySize).LineHeight(summaryLine).
				Weight(400).Color(t.LabelPrimary)
			// .separator：2×2 圆点，margin 0 8px。
			f.Row().Width(separatorSize).Height(separatorSize).Radius(separatorSize/2).
				Background(t.LabelCaption).MarginSides(0, separatorMargin, 0, separatorMargin)
			text := f.Text(summary).FontSize(summarySize).LineHeight(summaryLine).
				Color(summaryColor).Grow(1)
			if !open {
				text.Ellipsis()
			}
		})
		if open {
			// `<pre>`：官方 max-height 260、margin 4 0 4 4、padding 12 16、radius lg、
			// 0.5px 描边、代码块底色、等宽、pre-wrap。
			f.Column().FillWidth().MaxHeight(bodyMaxHeight).
				MarginSides(bodyMarginTop, 0, bodyMarginTop, bodyMarginLeft).
				PadSides(bodyPadV, bodyPadH, bodyPadV, bodyPadH).
				Radius(t.Radii.LG).Background(t.CodeBlock).Border(0.5, t.BorderL1).
				Children(func() {
					f.Text(node.Outcome.Text).FillWidth().FontSize(t.Fonts.Code).
						LineHeight(t.Fonts.CodeLine).Mono().Color(t.LabelSecondary)
				})
		}
	})
}

// drawCommand 是 slot ④ 的 painter：从 Store 取当前节点，其余交给 DrawCommandRow。
func (p *Plugin) drawCommand(f *flex.Frame, env *pluginapi.Env) {
	node := p.team.Command()
	if node == nil || node.Command != CommandKey {
		return
	}
	t := p.themeFor(env)
	DrawCommandRow(f, env, t, node, p.team.CommandExpanded(), p.team.ToggleCommandExpanded)
}

// 编译期确认我们用到的令牌字段存在（改主题结构时这里先红）。
var _ = ui.ShadowMD
