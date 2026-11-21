// file_actions.go —— 官方 `dsh-client-ui-file-actions` 的**壳侧**实现（#18）。
//
// # 分工（与插件那边划清）
//
//	plugins/dsfileactions   菜单项（哪个 target、什么顺序、什么文案）→ 只调 Actions
//	internal/apiclient      workspaceFiles.readBytes（原始字节）
//	本文件                  真正写盘的那一步：原生另存对话框 / 系统下载目录
//	copy_image.go           「复制图片」写剪贴板那一步（同一条读字节链的另一端）
//
// 为什么写盘在壳里而不是插件里：插件拿不到 `*apiclient.Client`，也拿不到 viva 的
// `app`（R2/R7）；原生另存对话框还必须在 UI 线程上开，而插件的菜单回调正好在那一桢。
//
// # 与官方的差异（照实登记，见 docs/official-file-actions-evidence-rc2.md §4）
//
//  1. 官方 `desktop.save` 是 Electron 主进程能力，`FileSaveResult` 有四个状态
//     （cancelled / requested / saved+path）。本仓只有 `saved` 与 `cancelled`：
//     没有浏览器那一档"requested（开始下载）"，因为我们不是浏览器。
//  2. 官方成功后会弹一条带「在文件夹显示」动作的 toast。本仓不画浮动条
//     （context menu 服务注释里已登记同一条理由），反馈落在**状态栏**。
//  3. 「在文件夹显示」本仓没接：`session.openWorkspacePath({action:'reveal'})`
//     宿主未实现（grep 为空），不做只有一半的取巧路径（`Actions.OpenFile` 打开
//     目录而不是选中文件，语义不同）。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vivaui/viva/app"
)

// fileStatTimeout 是一次 `workspaceFiles.stat` 探测的上限（见 workspaceFileStat）。
//
// 比 fileSaveTimeout 短：这条探测发生在**菜单 resolve 的那一帧**上（官方是 async，
// 本仓的 Resolve 是同步签名），卡住就是整帧卡住。本地宿主一次 stat 是毫秒级，
// 1 秒足够；超时按"不可用"处理，也就是把动作置灰——宁可"可能是灰的"，
// 也不能让用户点开菜单时界面顿一下。
const fileStatTimeout = time.Second

// fileSaveTimeout 是一次 readBytes 的上限。
//
// 写入本身不受它管（另存对话框由用户决定停留多久）。本地宿主一次读通常是毫秒级，
// 5 秒是"宿主卡住时别把界面吊死"的一条线。
const fileSaveTimeout = 5 * time.Second

// 反馈文案逐字来自官方 zh 字典（OFF:ui-file-actions/src/client/locales.ts:5-16）：
//
//	menu.saved        '已保存'
//	menu.downloaded   '下载完成'
//	menu.saveFailed   '无法保存文件，请检查源文件和目标位置后重试'
const (
	zhFileSaved      = "已保存"
	zhFileDownloaded = "下载完成"
	zhFileSaveFailed = "无法保存文件，请检查源文件和目标位置后重试"
)

// 下面三个是**替换点**：产品路径上它们分别是 viva 的原生对话框、os.WriteFile 与
// os.UserHomeDir；测试按下它们，就能在无头环境里验完整个决策链（而不是只验分支）。
var (
	fileSaveDialog = func(suggestedName string) (string, error) {
		return app.SaveFile(app.SaveFileOptions{SuggestedName: suggestedName})
	}
	fileWriteBytes = os.WriteFile
	fileHomeDir    = os.UserHomeDir
)

// saveWorkspaceFile 实现 `pluginapi.Actions.SaveWorkspaceFile`。
//
// 调用方是插件的菜单回调（UI 线程）。整条链路同步：
//
//	workspaceFiles.readBytes → （saveAs: 原生对话框 / download: 下载目录）→ 写盘 → 状态栏
//
// 任何一步失败都**写状态栏**并把原因带上，不静默成功（R3 的同一精神：不给假成功）。
// 用户取消对话框不是错误：不写状态栏（官方 `status:'cancelled'` 也不弹 toast）。
func (m *model) saveWorkspaceFile(sessionID, path, name, mode string) {
	if m.cli == nil {
		m.setStatus(zhFileSaveFailed + "（壳没有 apiclient）")
		return
	}
	if sessionID == "" || path == "" {
		// 官方 file target 的 (sessionId, path) 都是必填；缺了就是调用方写错了，
		// 这时候去猜"当前会话"会写错工作区。
		m.setStatus(zhFileSaveFailed + "（缺会话或路径）")
		return
	}
	suggested := suggestedFileName(name, path)
	ctx, cancel := context.WithTimeout(context.Background(), fileSaveTimeout)
	defer cancel()
	content, err := m.cli.WorkspaceFileReadBytes(ctx, sessionID, path)
	if err != nil {
		m.setStatus(zhFileSaveFailed + "：" + err.Error())
		return
	}
	var target string
	switch mode {
	case "saveAs":
		target, err = fileSaveDialog(suggested)
		if errors.Is(err, app.ErrCanceled) {
			return
		}
		if err != nil {
			m.setStatus(zhFileSaveFailed + "：" + err.Error())
			return
		}
	case "download":
		target, err = downloadTarget(suggested)
		if err != nil {
			m.setStatus(zhFileSaveFailed + "：" + err.Error())
			return
		}
	default:
		m.setStatus(zhFileSaveFailed + "（不认识的保存模式 " + mode + "）")
		return
	}
	if err := fileWriteBytes(target, content.Data, 0o600); err != nil {
		m.setStatus(zhFileSaveFailed + "：" + err.Error())
		return
	}
	// 官方成功时弹 toast（"已保存"）并附一个「在文件夹显示」的后续动作；本仓没有那
	// 个动作，所以把**落点**直接写在状态栏——不然"存哪了"就没人回答得出来。
	// 官方文案本身一个字不改（已保存 / 下载完成），冒号后面是落点。
	if mode == "saveAs" {
		m.setStatus(zhFileSaved + "：" + target)
		return
	}
	m.setStatus(zhFileDownloaded + "：" + target)
}

// workspaceFileStat 实现 `pluginapi.Actions.WorkspaceFileStat`：一次**有上限**的
// `workspaceFiles.stat`，把结果压成"能不能用"这一个布尔。
//
// 官方 file 菜单的 resolve 就是这条判据
// （OFF:ui-file-actions/src/client/context-menus.tsx:50-60）：
//
//	try { absolutePath = (await stat(sessionId, path)).value.absolutePath } catch { /* 置灰 */ }
//	const available = absolutePath !== undefined
//
// 返回 false 的三种来源都是**官方的同一档**（`available = false`）：
//
//	宿主报错（not-found / not-regular-file / 权限）   → 动作置灰，不是隐藏
//	客户端解析不出 absolutePath（宿主没给）           → 同上
//	超时 / 没有 apiclient                            → 同上（本仓的额外一档，已登记）
//
// 它**不**读字节、也不缓存：菜单每次打开都重新问一次（官方同样每次 resolve 都 stat），
// 所以"文件刚被删掉"在下一次右键里就反映出来。
func (m *model) workspaceFileStat(sessionID, path string) bool {
	if m.cli == nil || sessionID == "" || path == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), fileStatTimeout)
	defer cancel()
	if _, err := m.cli.WorkspaceFileStatOf(ctx, sessionID, path); err != nil {
		return false
	}
	return true
}

// suggestedFileName 定"建议文件名"。
//
// 优先级：官方 `FileActionSource.name`（图片 target 的 name 字段）> 路径最后一段。
// 两个都可能为空（path 以 / 结尾之类），那时给 "download"——官方 files.ts:53 的
// 兜底就是 `source.name ?? 'image'`（对图片而言）。
func suggestedFileName(name, path string) string {
	if v := strings.TrimSpace(name); v != "" {
		return v
	}
	trimmed := strings.TrimRight(strings.ReplaceAll(path, "\\", "/"), "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		trimmed = trimmed[i+1:]
	}
	if trimmed = strings.TrimSpace(trimmed); trimmed != "" {
		return trimmed
	}
	return "download"
}

// downloadTarget 是 `download` 模式的落点：系统下载目录 + 不覆盖同名文件。
//
// 官方 `desktop.save({mode:'download'})` 同样不问位置。本仓不发明"下载"的另一种
// 语义（复制到剪贴板之类），就是"放到下载目录"。
func downloadTarget(name string) (string, error) {
	home, err := fileHomeDir()
	if err != nil {
		return "", fmt.Errorf("找不到用户目录：%w", err)
	}
	dir := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("下载目录建不出来：%w", err)
	}
	return uniquePath(dir, name), nil
}

// uniquePath 返回 dir/name；已存在就在扩展名前加 " (2)"、" (3)"…（不覆盖用户已有文件）。
func uniquePath(dir, name string) string {
	base := name
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem, ext = base, ""
	}
	candidate := filepath.Join(dir, base)
	for i := 2; ; i++ {
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		} else if err != nil {
			// 探测失败也返回候选：真写不进去时 writeFileBytes 会报真实错误，
			// 这里不猜"是不是存在"。
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
	}
}
