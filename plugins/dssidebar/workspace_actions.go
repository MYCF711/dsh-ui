// workspace_actions.go —— #62 工作区改名/删除（官方挂 contextMenu 的两个动作）。
//
// # 官方证据
//
//	OFF:ui-workspace/src/client/session-actions/context-menus.tsx:39-68
//	  workspace 菜单贡献：rename(order 100) / delete(order 300, destructive)
//	OFF:ui-workspace/src/client/rows/WorkspaceDialogs.tsx:1-141
//	  两个模态：rename.workspace.title「重命名工作区」+ field.workspaceName
//	  「工作区名称」+ rename「重命名」；delete.workspace「删除工作区」+
//	  delete.desc「将把“{name}”从工作区列表中移除。文件夹与会话记录会保留，
//	  其会话将显示在“未分组”下。」+ delete.pending「正在删除工作区…」。
//	OFF:ui-workspace/src/client/rows/Rows.tsx:308-336
//	  工作区行右键（getTriggerProps）与行尾省略号（contextMenu.button）
//
// # 本仓边界
//
//   - 数据面：`Snapshot.Workspaces`（壳缓存 `workspace.registry`）；
//   - 写面：`Actions.WorkspaceRename/Delete`（壳同步写宿主，返回四字段错误）；
//   - 渲染：菜单由 ds-contextmenu 的 `ui.ContextMenu` / `ui.Menu` 画；对话框
//     是本文件画的模态（没有官方 primitives 的 Modal 源码，几何取本仓既有
//     模态一档；文案逐字用官方中文）。
//   - 分组仍按 cwd（官方按 registry.SessionIDs）。因此**只对 registry 里真实
//     存在的工作区**显示菜单；没有 registry 的组保持原样。这条差异写在
//     docs/official-parity.md #62 行的"还剩什么"里。
package dssidebar

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 zh 字典原文（`OFF:ui-workspace/src/client/locales.ts:50-60,99-100`）。
const (
	zhWorkspaceRename      = "重命名"
	zhWorkspaceDelete      = "删除工作区"
	zhWorkspaceRenameTitle = "重命名工作区"
	zhWorkspaceName        = "工作区名称"
	zhWorkspaceCancel      = "取消"
	zhWorkspaceDeleting    = "正在删除工作区…"
	zhSessionCopyID        = "复制 Session ID"
)

const (
	workspaceActionRename = "rename"
	workspaceActionDelete = "delete"

	workspaceDialogWidth  = 420
	workspaceDialogPad    = 20
	workspaceDialogGap    = 12
	workspaceDialogTitle  = 16
	workspaceDialogTopPct = 0.18
)

// workspaceActionRequest 是一次已打开的工作区对话框请求。
type workspaceActionRequest struct {
	kind         string // rename / delete
	workspaceID  string
	displayTitle string
	// storedTitle 是 registry 里的原标题：官方 rename 对"没改过"直接禁用。
	storedTitle string
}

// registerContextMenus 登记 #10 的两份真实业务贡献（菜单是数据面，渲染归服务）。
func (p *Plugin) registerContextMenus(ctx pluginapi.Context) error {
	if p.menus == nil {
		value := ctx.Inject(pluginapi.KeyContextMenus)
		if value == nil {
			return nil // 单测直接构造 Plugin：没有服务就不登记，也不报错
		}
		menus, ok := value.(pluginapi.ContextMenus)
		if !ok {
			return nil
		}
		p.menus = menus
	}
	if p.menus == nil {
		return nil
	}

	unregisterWorkspace, err := p.menus.RegisterContribution(pluginapi.ContextMenuContribution{
		ID: "workspace.rename", Target: "workspace", Order: 100,
		Resolve: func(target pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			if p.act.WorkspaceRename == nil {
				return nil
			}
			return []pluginapi.ContextMenuAction{{
				ID: "rename", Label: zhWorkspaceRename,
				Run: func() { p.requestWorkspaceAction(workspaceActionRename, target) },
			}}
		},
	})
	if err != nil {
		return err
	}
	ctx.Effect(unregisterWorkspace)

	unregisterDelete, err := p.menus.RegisterContribution(pluginapi.ContextMenuContribution{
		ID: "workspace.delete", Target: "workspace", Order: 300,
		Resolve: func(target pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			if p.act.WorkspaceDelete == nil {
				return nil
			}
			return []pluginapi.ContextMenuAction{{
				ID: "delete", Label: zhWorkspaceDelete, Danger: true, SeparatorBefore: true,
				Run: func() { p.requestWorkspaceAction(workspaceActionDelete, target) },
			}}
		},
	})
	if err != nil {
		return err
	}
	ctx.Effect(unregisterDelete)

	// session 菜单：官方 `menu.sessionId`「复制 Session ID」（order 450，分隔线前）。
	// 它让 `contextMenu.button` 在会话行有一个真实消费方；其余会话动作
	// （置顶/归档/分叉/改名）本仓尚无对应写面，不画死按钮。
	unregisterSession, err := p.menus.RegisterContribution(pluginapi.ContextMenuContribution{
		ID: "session.copy-id", Target: "session", Order: 450,
		Resolve: func(target pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			if p.act.Copy == nil {
				return nil
			}
			return []pluginapi.ContextMenuAction{{
				ID: "copy", Label: zhSessionCopyID, SeparatorBefore: true,
				Run: func() { p.act.Copy(target.ID) },
			}}
		},
	})
	if err != nil {
		return err
	}
	ctx.Effect(unregisterSession)
	return nil
}

// workspaceMenuTarget 把 cwd 组键映射成官方 workspace target。
//
// 匹配规则：registry 的 path 与组键规范化后相等。匹配不到 = 这条组没有
// 真实 registry 身份 → 不显示改名/删除（不对不存在的工作区做写操作）。
func workspaceMenuTarget(workspaces []pluginapi.Workspace, key string) (pluginapi.ContextMenuTarget, bool) {
	for _, workspace := range workspaces {
		if sameWorkspacePath(workspace.Path, key) {
			return pluginapi.ContextMenuTarget{
				Kind:         "workspace",
				ID:           workspace.WorkspaceID,
				DisplayTitle: workspaceTitle(workspace, key),
				Path:         workspace.Path,
			}, true
		}
	}
	return pluginapi.ContextMenuTarget{}, false
}

// workspaceTargetLabel 取组头/菜单文案：registry 标题优先，空时退回 cwd 基名。
func workspaceTargetLabel(target pluginapi.ContextMenuTarget, fallback string) string {
	if strings.TrimSpace(target.DisplayTitle) != "" {
		return target.DisplayTitle
	}
	return fallback
}

func workspaceTitle(workspace pluginapi.Workspace, key string) string {
	if title := strings.TrimSpace(workspace.Title); title != "" {
		return title
	}
	return displayWorkspace(key)
}

// sameWorkspacePath 规范化比较：Windows 大小写不敏感、斜杠统一、去尾分隔符。
func sameWorkspacePath(a, b string) bool {
	return normalizeWorkspacePath(a) == normalizeWorkspacePath(b)
}

func normalizeWorkspacePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.ReplaceAll(path, "/", `\`)
	path = strings.TrimRight(path, `\`)
	// `C:\` 这种根路径 trim 后只剩 `C:`，保留根语义。
	if len(path) == 2 && path[1] == ':' {
		path += `\`
	}
	return strings.ToLower(path)
}

// requestWorkspaceAction 打开对应对话框（重命名会预填当前标题）。
func (p *Plugin) requestWorkspaceAction(kind string, target pluginapi.ContextMenuTarget) {
	request := workspaceActionRequest{
		kind:         kind,
		workspaceID:  target.ID,
		displayTitle: target.DisplayTitle,
		storedTitle:  target.DisplayTitle,
	}
	p.wsMu.Lock()
	p.wsAction = request
	p.wsBusy = false
	p.wsErr = ""
	p.wsMu.Unlock()
	if kind == workspaceActionRename {
		p.wsRename.SetText(target.DisplayTitle)
	}
	if p.act.Invalidate != nil {
		p.act.Invalidate()
	}
}

// closeWorkspaceAction 关闭对话框；写操作在途时不允许关（官方 renaming/deleting 同义）。
func (p *Plugin) closeWorkspaceAction() {
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	if p.wsBusy {
		return
	}
	p.wsAction = workspaceActionRequest{}
	p.wsErr = ""
}

// workspaceActionSnapshot 给绘制线程一份一致的状态（含字段文本由调用方读）。
func (p *Plugin) workspaceActionSnapshot() (workspaceActionRequest, bool, string) {
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	return p.wsAction, p.wsBusy, p.wsErr
}

// drawWorkspaceDialogs 是 shell.overlay 的 painter：只在一个请求存在时画模态。
func (p *Plugin) drawWorkspaceDialogs(f ui.C, env *pluginapi.Env) {
	request, busy, errText := p.workspaceActionSnapshot()
	if request.kind == "" {
		return
	}
	t := p.theme
	width := float32(workspaceDialogWidth)
	if env.Bounds.Width > 0 && width > env.Bounds.Width-24 {
		width = env.Bounds.Width - 24
	}
	if width < 260 {
		width = 260
	}
	left := (env.Bounds.Width - width) / 2
	top := env.Bounds.Height * workspaceDialogTopPct
	if top < 0 {
		top = 0
	}

	mask := t.BgBase
	mask.Alpha = 0.55
	backdrop := ui.Row(f).Absolute().Inset(0).Background(mask).
		Hit(env.Tree.ID(env.Feat, "ws-dialog-backdrop"), zhWorkspaceCancel)

	title := zhWorkspaceRenameTitle
	if request.kind == workspaceActionDelete {
		title = zhWorkspaceDelete
	}
	card := ui.Column(f).Absolute().Left(left).Top(top).Width(width).
		Hit(env.Tree.ID(env.Feat, "ws-dialog"), title).
		Radius(t.Radii.Panel).Background(t.InputMajor).Border(1, t.BorderL2).
		Shadow(gfx.ShadowMD).Pad(workspaceDialogPad).Gap(workspaceDialogGap)
	card.Children(func() {
		ui.Row(f).FillWidth().Items(ui.Center).Children(func() {
			ui.Text(f, title).FontSize(workspaceDialogTitle).Weight(500).
				Color(t.LabelPrimary).Grow(1)
			if ui.Button(f, "×").Ghost().Small().Disabled(busy).Clicked() {
				p.closeWorkspaceAction()
			}
		})

		switch request.kind {
		case workspaceActionRename:
			p.drawWorkspaceRenameBody(f, env, request, busy, errText)
		case workspaceActionDelete:
			p.drawWorkspaceDeleteBody(f, env, request, busy, errText)
		}
	})

	if backdrop.Clicked() && !busy {
		p.closeWorkspaceAction()
	}
}

func (p *Plugin) drawWorkspaceRenameBody(f ui.C, env *pluginapi.Env,
	request workspaceActionRequest, busy bool, errText string) {
	t := p.theme
	draft := strings.TrimSpace(p.wsRename.Text())
	duplicate := false
	for _, workspace := range env.Data.Workspaces {
		if workspace.WorkspaceID != request.workspaceID &&
			strings.TrimSpace(workspace.Title) == draft && draft != "" {
			duplicate = true
			break
		}
	}
	blocked := busy || draft == "" || draft == request.storedTitle || duplicate

	ui.TextFieldID(f, env.Tree.ID(env.Feat, "ws-rename-input"), &p.wsRename,
		zhWorkspaceName, request.storedTitle).FillWidth()
	if duplicate {
		ui.Text(f, "已存在名为“"+draft+"”的工作区。").FontSize(t.Fonts.Caption).Color(t.Error)
	}
	if errText != "" {
		ui.Text(f, errText).FontSize(t.Fonts.Caption).Color(t.Error)
	}
	ui.Row(f).FillWidth().Justify(ui.End).Items(ui.Center).Gap(8).Children(func() {
		if ui.Button(f, zhWorkspaceCancel).Outline().Small().Disabled(busy).Clicked() {
			p.closeWorkspaceAction()
		}
		if ui.Button(f, zhWorkspaceRename).Small().Disabled(blocked).Clicked() {
			p.startWorkspaceRename(env.Act, draft)
		}
	})
}

func (p *Plugin) drawWorkspaceDeleteBody(f ui.C, env *pluginapi.Env,
	request workspaceActionRequest, busy bool, errText string) {
	t := p.theme
	ui.Text(f, "将把“"+request.displayTitle+"”从工作区列表中移除。文件夹与会话记录会保留，其会话将显示在“未分组”下。").
		FontSize(t.Fonts.Body).Color(t.LabelPrimary)
	if busy {
		ui.Text(f, zhWorkspaceDeleting).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
	}
	if errText != "" {
		ui.Text(f, errText).FontSize(t.Fonts.Caption).Color(t.Error)
	}
	ui.Row(f).FillWidth().Justify(ui.End).Items(ui.Center).Gap(8).Children(func() {
		if ui.Button(f, zhWorkspaceCancel).Outline().Small().Disabled(busy).Clicked() {
			p.closeWorkspaceAction()
		}
		if ui.Button(f, zhWorkspaceDelete).Destructive().Small().Disabled(busy).Clicked() {
			p.startWorkspaceDelete(env.Act)
		}
	})
}

func (p *Plugin) startWorkspaceRename(act pluginapi.Actions, title string) {
	p.wsMu.Lock()
	request := p.wsAction
	p.wsBusy = true
	p.wsErr = ""
	p.wsMu.Unlock()
	rename := act.WorkspaceRename
	go func() {
		var err error
		if rename != nil {
			err = rename(request.workspaceID, title)
		}
		p.finishWorkspaceAction(act, err)
	}()
}

func (p *Plugin) startWorkspaceDelete(act pluginapi.Actions) {
	p.wsMu.Lock()
	request := p.wsAction
	p.wsBusy = true
	p.wsErr = ""
	p.wsMu.Unlock()
	remove := act.WorkspaceDelete
	go func() {
		var err error
		if remove != nil {
			err = remove(request.workspaceID)
		}
		p.finishWorkspaceAction(act, err)
	}()
}

// finishWorkspaceAction 统一收尾：成功清空对话框，失败把四字段错误留在框里。
func (p *Plugin) finishWorkspaceAction(act pluginapi.Actions, err error) {
	p.wsMu.Lock()
	p.wsBusy = false
	if err != nil {
		p.wsErr = err.Error()
	} else {
		p.wsAction = workspaceActionRequest{}
		p.wsErr = ""
	}
	p.wsMu.Unlock()
	if act.Invalidate != nil {
		act.Invalidate()
	}
}
