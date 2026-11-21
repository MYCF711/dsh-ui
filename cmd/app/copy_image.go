// copy_image.go —— 官方「复制图片」的**壳侧**实现（`#18` ①）。
//
// # 官方那一条到底做什么
//
//	OFF:dsh-client-ui-file-actions/src/client/context-menus.tsx:44-47   菜单项 → copy(source)
//	OFF:dsh-client-ui-file-actions/src/client/context-menus.tsx:69-72   file 族：id copyImage、separatorBefore
//	OFF:dsh-client-ui-file-actions/src/client/context-menus.tsx:99      image 族：id copy
//	OFF:dsh-client-ui-file-actions/src/client/files.ts:65-89           copyImage：读字节 → 非 PNG 过 canvas → clipboard.write
//
// 成功后一条 `ctx.contextMenus.notify(t('menu.copied'), true)`（"已复制"，浮条）；
// 失败由菜单项的 `failureLabel: t('menu.copyFailed')`（"无法复制，请检查内容是否可用后重试"）
// 显示。**本仓没有浮条服务**（差异 ④），两种反馈都落状态栏——与保存链同一口径。
//
// # 分工（与 file_actions.go 完全同构）
//
//	plugins/dsfileactions   菜单项（哪个 target、什么顺序、什么文案）→ 只调 Actions
//	internal/apiclient      workspaceFiles.readBytes（原始字节）
//	internal/imgdecode      非 PNG → PNG（官方 canvas 的等价物）
//	本文件                  真的写剪贴板那一步：viva 的 app.SetClipboardImage
//
// # 为什么不用 Clipboard"清空再写"这类花活
//
// viva 的写入口**语义就是一个复制动作**：整块替换剪贴板内容（连同文本一起），
// 平台侧还同时提供注册 "PNG" 格式与 CF_DIB 两种口味，让"只认位图的"阅读器也能粘。
// 本仓不做任何附加动作（不放文本、不留旧内容），与官方 `ClipboardItem` 一致。
package main

import (
	"context"

	"github.com/MYCF711/dsh-ui/internal/imgdecode"
	"github.com/vivaui/viva/app"
)

// 反馈文案逐字来自官方 zh 字典（OFF:ui-file-actions/src/client/locales.ts:8,12）：
//
//	menu.copied      '已复制'
//	menu.copyFailed  '无法复制，请检查内容是否可用后重试'
//
// 失败时冒号后面补原因——官方靠浮条的 failureLabel 只给一句，本仓写状态栏，
// 不带上原因就变成"不知道为什么失败"（保存链同一条处理，见 file_actions.go:95）。
const (
	zhImageCopied     = "已复制"
	zhImageCopyFailed = "无法复制，请检查内容是否可用后重试"
)

// clipboardSetImage 是**替换点**：产品路径是 viva 的 app.SetClipboardImage。
//
// 测试按下它，就能在无头环境里验完"读字节 → 转码 → 交给剪贴板写入口"整条链，
// **不去碰用户真实的剪贴板**（真剪贴板是全局状态，测试写它就是打扰正在用机器的人）。
var clipboardSetImage = func(png []byte) error { return app.SetClipboardImage(png) }

// copyWorkspaceImage 实现 `pluginapi.Actions.CopyWorkspaceImage`。
//
// 调用方是插件的菜单回调（UI 线程）。整条链路同步：
//
//	workspaceFiles.readBytes → imgdecode.ToPNG → app.SetClipboardImage → 状态栏
//
// 任何一步失败都**写状态栏**并带上原因，不静默成功、也不静默失败（R3 同一精神）。
// 只有一种情况什么都不写：`CopyWorkspaceImage` 压根没被画出来（壳没接线时插件不画），
// 所以这里没有"用户取消"这一档——复制动作本身没有可取消的东西。
func (m *model) copyWorkspaceImage(sessionID, path string) {
	if m.cli == nil {
		m.setStatus(zhImageCopyFailed + "（壳没有 apiclient）")
		return
	}
	if sessionID == "" || path == "" {
		// 官方两族的 copy 都建在 (sessionId, path) 之上（image 族缺 path 时退到
		// `read(source.url)`，本仓没有 URL→字节链，所以给不出动作）。
		// 走到这里说明调用方写错了：去猜"当前会话"会读到别的工作区。
		m.setStatus(zhImageCopyFailed + "（缺会话或路径）")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), fileSaveTimeout)
	defer cancel()
	content, err := m.cli.WorkspaceFileReadBytes(ctx, sessionID, path)
	if err != nil {
		m.setStatus(zhImageCopyFailed + "：" + err.Error())
		return
	}
	if content == nil || len(content.Data) == 0 {
		// 宿主不会登记 0 字节文件（`HOST:cmd/dshd/attachments.go` 的同一判据），
		// 所以 0 字节只可能是链上出了问题。把它当成功会让用户"粘不出东西"，
		// 比报错更坏（attachment_bytes.go 的 0 字节判据同一条理由）。
		m.setStatus(zhImageCopyFailed + "（宿主返回 0 字节）")
		return
	}
	png, err := imgdecode.ToPNG(content.Data)
	if err != nil {
		m.setStatus(zhImageCopyFailed + "：" + err.Error())
		return
	}
	if err := clipboardSetImage(png); err != nil {
		m.setStatus(zhImageCopyFailed + "：" + err.Error())
		return
	}
	// 文案与官方一字不差（menu.copied）。官方那条 notify 不带后续动作，
	// 所以这里也不需要补"在哪儿"。
	m.setStatus(zhImageCopied)
}
