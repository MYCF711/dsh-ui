// Package dsfileactions 是官方 `dsh-client-ui-file-actions` 的 Go 版：给
// `file` / `image` 两个 context menu target 挂**原始字节**的两条动作。
//
// # 结构证据（唯一权威）
//
//	OFF:ui-file-actions/src/client/context-menus.tsx:49-84   file 菜单（order 200）
//	OFF:ui-file-actions/src/client/context-menus.tsx:85-104  image 菜单（order 100）
//	OFF:ui-file-actions/src/client/locales.ts:5-16           zh 字典（本包只用其中三条）
//	OFF:ui-file-actions/src/client/files.ts:70-103           save 的两种模式语义
//	OFF:ui-file-actions/src/types.ts:20-25                   'download' | 'saveAs' 与四态结果
//	OFF:ui-context-menu/src/client/types.ts:30,33            file / image target 的字段
//
// # 数据面（宿主是真的，不是注释）
//
//	HOST:internal/transport/handlers_workspace_official.go:70,997   workspaceFiles.readBytes
//	HOST:cmd/dshd/serve_workspace_api.go:570,1208                   生产接线 + 真实现
//	本仓客户端：internal/apiclient/workspace_files.go（base64 → []byte）
//	壳侧动作：pluginapi.Actions.SaveWorkspaceFile → cmd/app/file_actions.go
//
// # 本包**不画任何常驻面**
//
// 官方那份包也是纯基础设施（两个菜单贡献 + 两个图标槽），没有自己的可见区域。
// 但本仓的菜单贡献 Resolve 签名只有 target（官方那边 body 能拿 React context 的
// 服务），而动作要用**每帧传入**的 `env.Act`——所以这里挂一个什么都不画的
// painter 到 overlay，只为每帧刷新 `act` 一份（不建 Box、不占位、没有命中区）。
//
// # 三处照实登记的缺口（见 docs/official-file-actions-evidence-rc2.md）
//
//  1. **「复制图片」不画**：官方 `copyImage` 走 `navigator.clipboard.write(ClipboardItem)`
//     （OFF:files.ts:105-138）。viva v0.6.4 只导出 `ClipboardImagePNG`（**读**）与
//     `SetClipboardText`；图片的**写**只有测试 Driver 上有。`SetClipboardFiles` 是
//     "复制文件"，语义不同，不许冒充。
//  2. **`canSaveAs` 恒为 true**，所以 file target 上「下载」只在图片上出现——这正是
//     官方的判据 `mediaType.startsWith('image/') || !actions.canSaveAs`
//     （OFF:context-menus.tsx:68-80）在 canSaveAs=true 下的结果，不是我们的简化。
//  3. **image target 只在有 (sessionId, path) 时给动作**：官方没有 path 时可以靠
//     `source.url` 走 fetch（OFF:files.ts:41-52）。本仓没有浏览器那一层，取不到
//     URL 的原始字节，所以宁可不画动作（返回 nil = 菜单不弹），也不画一个必失败的项。
package dsfileactions

import (
	"mime"
	"path"
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// ServiceID 是本插件的装配 id（壳与测试按它查表）。
const ServiceID = "ds-fileactions"

// 官方两个贡献的 id 与 order（OFF:context-menus.tsx:49,85）。
const (
	fileContributionID   = "file-actions.file"
	fileContributionOrd  = 200
	imageContributionID  = "file-actions.image"
	imageContributionOrd = 100
)

// 官方动作 id（OFF:context-menus.tsx:71,76,82）。
const (
	actionCopyImage = "copyImage"
	actionDownload  = "download"
	actionSaveAs    = "saveAs"
)

// 文案：逐字来自 OFF:locales.ts:9-10 的 zh 字典。
//
//	menu.copyImage  '复制图片'  ← 不画（见包注释缺口 1）
//	menu.download   '下载'
//	menu.saveAs     '另存为…'
const (
	zhDownload = "下载"
	zhSaveAs   = "另存为…"
)

// Plugin 只有注册表与一份"本帧的 Actions"。
type Plugin struct {
	// menus 是 context menu 服务；nil（单测直接构造 / 没有该服务）时什么都不登记。
	menus pluginapi.ContextMenus
	// act 是每帧由 refreshPainter 刷新的壳动作面。
	//
	// 不加锁：Painter.Draw 与菜单 Resolve 都在 UI 线程上执行（同一桢），
	// 加锁只会掩盖"有人从 goroutine 里读它"这种真 bug。
	act pluginapi.Actions
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return ServiceID }

// Provides 实现 pluginapi.Plugin：本包不提供服务。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin：只要主题。
//
// **不把 `KeyContextMenus` 写进 Requires**：Requires 是硬依赖（`uikernel.Load` 里
// 缺键直接报错），而用户在插件管理页可以停用 `ds-contextmenu` —— 那时整棵插件树
// 不该因为"少了一张卡片的菜单"而起不来。菜单服务改为**尽力注入**：生产装配里它
// 总在（`docs/official-file-actions-evidence-rc2.md` §5 有装配级测试钉住），缺了
// 就打一行日志，不静默。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	if value := ctx.Inject(pluginapi.KeyContextMenus); value != nil {
		if menus, ok := value.(pluginapi.ContextMenus); ok {
			p.menus = menus
		}
	}
	if p.menus == nil {
		// 装配顺序问题（ds-contextmenu 还没 Provide）或它被停用：这一族菜单本次
		// 不存在。**响亮地**说出来——静默不登记会让"右键没反应"变成不可查的故障。
		ctx.Logf("没有 context menu 服务：file/image 两条菜单本次不登记")
		return nil
	}
	return p.registerMenus(ctx)
}

// Panels 只挂一个**不画东西**的 painter，用来取每帧的 Actions（见包注释）。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:    "file-actions-act",
		Slot:  pluginapi.SlotShellOverlay,
		Order: 900,
		Draw: func(_ ui.C, env *pluginapi.Env) {
			if env == nil {
				return
			}
			p.act = env.Act
		},
	}}
}

// registerMenus 登记官方那两份贡献。
func (p *Plugin) registerMenus(ctx pluginapi.Context) error {
	if p.menus == nil {
		return nil
	}
	unregisterFile, err := p.menus.RegisterContribution(pluginapi.ContextMenuContribution{
		ID:     fileContributionID,
		Target: pluginapi.TargetFile,
		Order:  fileContributionOrd,
		Resolve: func(target pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			return p.resolveFile(target)
		},
	})
	if err != nil {
		return err
	}
	ctx.Effect(unregisterFile)

	unregisterImage, err := p.menus.RegisterContribution(pluginapi.ContextMenuContribution{
		ID:     imageContributionID,
		Target: pluginapi.TargetImage,
		Order:  imageContributionOrd,
		Resolve: func(target pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			return p.resolveImage(target)
		},
	})
	if err != nil {
		return err
	}
	ctx.Effect(unregisterImage)
	return nil
}

// resolveFile 是官方 file 菜单（OFF:context-menus.tsx:49-84）的判据投影。
//
// 官方那三段展开条件的原样：
//
//	...mediaType.startsWith('image/') ? [copyImage(separatorBefore)] : [],
//	...mediaType.startsWith('image/') || !actions.canSaveAs ? [download] : [],
//	...actions.canSaveAs ? [saveAs] : [],
//
// 本仓：copyImage 不画（缺口 1），canSaveAs 恒 true（缺口 2）→ 只剩下
// 「图片：下载 + 另存为」与「非图片：另存为」。
func (p *Plugin) resolveFile(target pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
	if p.act.SaveWorkspaceFile == nil || target.SessionID == "" || target.Path == "" {
		// 缺壳动作、缺会话身份或缺失路径 → 一个动作都不给（菜单不弹）。
		// 官方在 stat 失败时是把动作置灰（disabled），本仓不画死按钮。
		return nil
	}
	name := fileName(target.Path)
	var out []pluginapi.ContextMenuAction
	if strings.HasPrefix(mediaTypeOf(target.Path), "image/") {
		out = append(out, p.saveAction(target, actionDownload, zhDownload, pluginapi.FileDownload, name))
	}
	out = append(out, p.saveAction(target, actionSaveAs, zhSaveAs, pluginapi.FileSaveAs, name))
	return out
}

// resolveImage 是官方 image 菜单（OFF:context-menus.tsx:85-104）的判据投影。
//
// 官方那段的三项：copyImage、download、canSaveAs 时的 saveAs。本仓少 copyImage；
// 且没有 (sessionId, path) 就整段不给（缺口 3）。
func (p *Plugin) resolveImage(target pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
	if p.act.SaveWorkspaceFile == nil || target.SessionID == "" || target.Path == "" {
		return nil
	}
	name := strings.TrimSpace(target.Name)
	if name == "" {
		name = fileName(target.Path)
	}
	return []pluginapi.ContextMenuAction{
		p.saveAction(target, actionDownload, zhDownload, pluginapi.FileDownload, name),
		p.saveAction(target, actionSaveAs, zhSaveAs, pluginapi.FileSaveAs, name),
	}
}

// saveAction 造一条"把原始字节保存出去"的菜单项。
//
// 动作本体全在壳里（读字节 + 开对话框/写下载目录 + 反馈），插件只交出
// (sessionId, path, name, mode) 四个身份字段——菜单打开后用户可能已经切走会话，
// 所以这里用的是**捕获的 target**，不读"当前会话"。
func (p *Plugin) saveAction(target pluginapi.ContextMenuTarget, id, label, mode, name string) pluginapi.ContextMenuAction {
	sessionID, relativePath := target.SessionID, target.Path
	icon := "file-actions.save"
	if id == actionDownload {
		icon = "file-actions.download"
	}
	return pluginapi.ContextMenuAction{
		ID:    id,
		Label: label,
		Icon:  icon,
		Run: func() {
			if p.act.SaveWorkspaceFile == nil {
				return
			}
			p.act.SaveWorkspaceFile(sessionID, relativePath, name, mode)
		},
	}
}

// fileName 取路径最后一段（官方 `path.slice(Math.max(lastIndexOf('/'), lastIndexOf('\\'))+1)`，
// OFF:context-menus.tsx:63-64）。两种分隔符都要认——Windows 上 path 可能是反斜杠。
func fileName(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	return p
}

// mediaTypeOf 按扩展名判 MIME（官方用 `mime.getType(path)`，OFF:context-menus.tsx:59）。
//
// 用标准库：它自带常见类型表，Windows 上还会查注册表；查不到返回空串——
// 空串**不是** image/，于是非图片分支（只有「另存为」）是安全的兜底。
func mediaTypeOf(p string) string {
	ext := strings.ToLower(path.Ext(strings.ReplaceAll(p, `\`, "/")))
	if ext == "" {
		return ""
	}
	return mime.TypeByExtension(ext)
}
