package main

// 官方 rc.5 `dsh-deepseek-chat-import` 的**壳侧**动作：选文件 → 校验上限 →
// 上传导入 → 刷新列表。
//
// # 为什么整条流程都在壳里
//
// 官方这条能力由浏览器插件自己完成（`dsh-deepseek-chat-import/src/client/
// ImportSessions.tsx` 的 `injected.importArchive`）：它用 `<input type="file">`
// 拿 File 对象、用自己的 fetch 上传、完了调 `ctx.sessions.refresh()`。
// 本仓没有浏览器：文件对话框、HTTP 客户端、会话列表缓存三者都归壳
// （与 Copy / OpenFile / DesktopUpdateOpen 同一条分工）。所以插件只画那颗按钮，
// 点下去交给这个文件。
//
// # 文案逐字取自官方客户端字典（lib/client.js 的 zh 字典）
//
//	已导入 {count} 个会话到工作区 {name}。 / {count} 个会话无法导入。
//	正在上传 {percent}% / 正在导入… / 正在导入 {done}/{total}
//	error.workspace-not-found: 所选工作区已不存在，请重新选择。
//	error.busy: 已有一个导入正在进行，请稍后再试。
//	…其余见 chatImportFailureText
//
// # 目标工作区
//
// 官方对话框让用户挑工作区，默认值是"当前会话所在的工作区"。本仓这一版没有
// 那个模态框（见返回值说明），所以**只能**用官方默认规则：当前会话所在的工作区。
// 没有当前会话、或它不属于任何工作区时如实拒绝，不猜一个工作区写进去。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vivaui/viva/app"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// chatImportTimeout 是一次导入的**总**超时（含上传与宿主写盘）。
//
// 官方客户端只用 AbortSignal（用户取消才停），不给固定上限；这里给一个宽上限
// 是为了不让一次卡住的导入把 goroutine 永久挂住：512 MiB 的上限在大文件上
// 也要给足时间，所以是 30 分钟而不是 30 秒。
const chatImportTimeout = 30 * time.Minute

// importDeepSeekSessions 是侧栏那颗导入按钮的动作。
//
// 全程在后台 goroutine：文件对话框是模态的，HTTP 上传可能几分钟，都不能占着
// 绘制线程（Actions 的约定见 `pluginapi.Actions.PickExportArchive` 邻域注释）。
func (m *model) importDeepSeekSessions() {
	if m == nil || m.cli == nil {
		return
	}
	sessionID := m.cur
	go func() {
		path, err := m.pickExportArchive()
		if err != nil {
			return // pickExportArchive 自己写过状态栏了
		}
		if path == "" {
			return // 用户取消，什么都不做（官方同样不提示）
		}
		workspaceID, workspaceTitle := m.importTargetWorkspace(sessionID)
		if workspaceID == "" {
			m.setStatus("导入 DeepSeek 会话需要先打开一个属于某个工作区的会话")
			return
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			m.setStatus("读不到导出文件: " + statErr.Error())
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), chatImportTimeout)
		defer cancel()

		limits, limitsErr := m.cli.ChatImportLimits(ctx)
		if limitsErr != nil {
			m.setStatus("导入失败：" + chatImportFailureText(limitsErr))
			return
		}
		if info.Size() > limits.MaxUploadBytes {
			m.setStatus(fmt.Sprintf("文件过大：最多 %s。", fileSizeText(limits.MaxUploadBytes)))
			return
		}

		lastPhase := ""
		result, importErr := m.cli.ChatImportArchive(ctx, workspaceID, path, func(progress apiclient.ChatImportProgress) {
			// 进度按"阶段/百分比/计数"去重：上传阶段每个 Read 都会回调，
			// 不去重会把状态栏刷成几千次重绘。
			switch progress.Phase {
			case "uploading":
				percent := int(progress.Fraction * 100)
				if lastPhase == fmt.Sprintf("uploading%d", percent) {
					return
				}
				lastPhase = fmt.Sprintf("uploading%d", percent)
				m.setStatus(fmt.Sprintf("正在上传 %d%%", percent))
			case "reading":
				if lastPhase == "reading" {
					return
				}
				lastPhase = "reading"
				m.setStatus("正在导入…")
			case "importing":
				key := fmt.Sprintf("importing%d/%d", progress.Done, progress.Total)
				if lastPhase == key {
					return
				}
				lastPhase = key
				if progress.Total > 0 {
					m.setStatus(fmt.Sprintf("正在导入 %d/%d", progress.Done, progress.Total))
				}
			}
		})
		if importErr != nil {
			m.setStatus("导入失败：" + chatImportFailureText(importErr))
			return
		}
		// 官方 `finally { await ctx.sessions.refresh() }`：**无论成败**都刷新
		// （中途失败可能已经写进了一部分会话）。
		m.refreshList()
		name := workspaceTitle
		if name == "" {
			name = workspaceID
		}
		m.setStatus(fmt.Sprintf("已导入 %d 个会话到工作区 %s。", len(result.SessionIDs), name))
		if len(result.Skipped) > 0 {
			m.setStatus(fmt.Sprintf("已导入 %d 个会话到工作区 %s；%d 个会话无法导入。",
				len(result.SessionIDs), name, len(result.Skipped)))
		}
		if result.OmittedByLimit > 0 {
			m.setStatus(fmt.Sprintf("已导入 %d 个会话到工作区 %s；%d 个较早的会话超出导入上限，未导入。",
				len(result.SessionIDs), name, result.OmittedByLimit))
		}
	}()
}

// pickExportArchive 弹系统文件对话框选一个导出 ZIP。
//
// 过滤器只给 zip：官方 `<input>` 的 accept 是 `.zip,application/zip`
// （ImportSessions.tsx），给用户选一个别的类型再报错是把校验推给用户去撞墙。
//
// 线程模型：Viva 的 Windows 实现自己在对话框里做 `runtime.LockOSThread` + STA
// COM 初始化（`vivaui/viva internal/platform/windows/system_windows.go:751`
// 的 `withSTA`），并把主窗口句柄当 owner（`:910` 的 `dialogOwner`），
// 所以在后台 goroutine 里直接调它不需要额外调度。
func (m *model) pickExportArchive() (string, error) {
	paths, err := app.OpenFiles(app.OpenFileOptions{
		Title: "选择 DeepSeek 导出文件（deepseek_data-*.zip）",
		Filters: []app.FileFilter{
			{Name: "DeepSeek 导出", Pattern: "*.zip"},
			{Name: "所有文件", Pattern: "*"},
		},
	})
	if err != nil {
		if errors.Is(err, app.ErrCanceled) {
			return "", nil
		}
		m.setStatus("打开文件对话框失败: " + err.Error())
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	return strings.TrimSpace(paths[0]), nil
}

// importTargetWorkspace 是官方对话框里"工作区"字段的默认值：
// 当前会话所在的工作区（ImportSessions.tsx 用 `workspaces.find(…sessionIds…)`）。
//
// 返回空串表示当前会话不属于任何工作区——这时不猜。
func (m *model) importTargetWorkspace(sessionID string) (string, string) {
	if sessionID == "" {
		return "", ""
	}
	for _, workspace := range m.workspaces {
		for _, member := range workspace.SessionIDs {
			if member == sessionID {
				return workspace.WorkspaceID, workspace.Title
			}
		}
	}
	return "", ""
}

// chatImportFailureText 把一次导入失败翻成官方字典里的那句中文
// （lib/client.js 的 zh.error.*）。原因码来自宿主 outcome 行。
func chatImportFailureText(err error) string {
	switch apiclient.ChatImportFailureReason(err) {
	case "too-large":
		return "文件过大，超过了宿主的上传上限。"
	case "unreadable-archive":
		return "该文件不是可读取的 ZIP 压缩包。"
	case "missing-manifest":
		return "这不是 DeepSeek 导出数据：缺少 manifest.json。"
	case "missing-index":
		return "这不是 DeepSeek 导出数据：缺少 chat_session_index.json，或该文件无法读取。"
	case "unsupported-version":
		return "不支持该导出格式版本。"
	case "no-conversations":
		return "导出数据中没有可导入的会话。"
	case "workspace-not-found":
		return "所选工作区已不存在，请重新选择。"
	case "busy":
		return "已有一个导入正在进行，请稍后再试。"
	}
	return err.Error()
}

// fileSizeText 是官方 `fileSizeText` 的同义实现（`error.too-large` 要用它拼上限）。
func fileSizeText(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	divisor, exponent := int64(unit), 0
	for value := bytes / unit; value >= unit && exponent < 3; value /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(divisor), "KMGTPE"[exponent])
}
