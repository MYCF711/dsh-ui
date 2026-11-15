// 本文件补的是官方拖入体验的另一半：**文件拖到窗口上方时的那层悬停提示**
// （官方 `dsh-client-ui-attachment` 的 `DropOverlay`）。rail.go 的包注释里曾把它
// 列为「一层都不画」，理由第四条是「viva 的 `OnFileDrop` 在 Windows 只报 drop」——
// **那条论据是错的**，本文件按实测改正（见下「为什么现在能画」）。
//
// # 它照抄的是哪一份官方源（几何/文案/底色的唯一权威）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-attachment
//
//	OFF:src/DropOverlay.tsx:24-40        结构：mask → wrap → title（+ 可选 desc）
//	OFF:src/DropOverlay.tsx:43-62        115×84 SVG 插画（**没画**，见「缺什么」）
//	OFF:src/DropOverlay.tsx:64-77        拒绝态插画（同上，**没画**）
//	OFF:src/DropOverlay.module.css:3-14  .mask：固定、铺满、z-index 1000、内容居中、
//	                                     `pointer-events:none`、底色 + `blur(10px)` + fade-in 160ms
//	OFF:src/DropOverlay.module.css:27-35 .wrap：flex column / items-center / margin-top -3% / padding 0 40px
//	                                        / 文字色 = `--dsw-alias-label-primary`
//	OFF:src/DropOverlay.module.css:37-45 .illustration（115×84）、.title（margin-top 16 + `--dsw-font-l-20`）
//	OFF:src/DropOverlay.module.css:47-52 .desc（`--dsw-font-s-14` + `--dsw-alias-label-tertiary`）
//
// 底色不是猜的，也不是从 `pluginapi.Theme` 里借位：
//
//	OFF:dsh-client-ui-theme/src/styles/design-platform.css:188   --dsw-alias-bg-mask-drop: rgba(255, 255, 255, 0.7)   （亮）
//	OFF:…design-platform.css:340                                 --dsw-alias-bg-mask-drop: rgba(39, 39, 48, 0.7)    （暗）
//
// 文案同样不是新写的：`attachment.dropTitle` / `attachment.dropBlocked` 逐字来自
// `OFF:dsh-client-ui-conversation/src/client/locales.ts`（zh 段），常量在 rail.go 里。
//
// # 为什么现在能画（rail.go 那条论据错在哪）
//
// viva v0.6.4 的 `app.Config.OnFileDrop` 文档写「On Windows only the drop is reported」
// （`app/app.go:154-159`），但那是**提权进程的兜底路径**才成立：
//
//	非提权：窗口注册真 OLE `IDropTarget`（`internal/platform/windows/drop_windows.go:263`
//	        `registerDropTarget`），`DragEnter/DragOver/DragLeave/Drop` 四个回调都在
//	        （`:174-222`），`DragEnter`/`DragOver` 报 `FileDropOver`（`:179`、`:189`）、
//	        `DragLeave` 报 `FileDropLeave`（`:198`）—— 悬停态拿得到。
//	提权：  `acceptFiles()` 走 `WM_DROPFILES`（`:238`、`:310` `handleDropFiles`），
//	        只有 drop 一趟 —— 文档说的是这一条。
//
// 也就是说：**常规（非提权）Windows 进程上，悬停层到得了**。剩下三样（插画/blur/fade）
// 仍然缺，所以只画「灰色压层 + 官方邀请文案」这一层，不画插画、不做模糊与过渡。
//
// # 缺什么（照实登记，不假装画了）
//
//   - **115×84 插画**（`OFF:DropOverlay.tsx:43-77`）：官方是内联 SVG（4 块彩色图形 +
//     白色描边 + clipPath）。仓规不许手画 SVG（先例：rail.go 的 `drawIcon` 用扩展名文字
//     顶替 `FileTypeIcon`），`assets/official/**` 里也没有这幅插画 → 不画。
//   - **`backdrop-filter: blur(10px)`**（`OFF:DropOverlay.module.css:12`）：viva 的画布
//     没有背景模糊 → 不画。
//   - **`animation: fade-in 160ms`**（`:13-19`）与 `prefers-reduced-motion` 分支（`:21-25`）：
//     viva 的画布没有过渡 → 不画。
//   - 因为插画缺席，跟着插画走的几何也一并没有意义：`.title` 的 `margin-top:16px`
//     （`:42-45`，那是插画与标题之间的间距）、`.wrap` 的 `margin-top:-3%`（`:31`，
//     那是为了把「插画+标题」这一坨整体上提一点）。本文件只保留「整层居中」，
//     不给缺席的插画留空洞。
//   - **`attachment.dropDesc`**（`:38`「图片限制：最多 {count} 张，每张 {size}」，`limits`
//     取自官方 `dropLimits`）：宿主没有「张数/单张上限」这个事实（限额写在
//     `HOST:cmd/dshd/attachments.go:30` 的 8 MiB 总量上，是另一个语义），编一个数字就是
//     假话 → 不画（与 rail.go 同一条登记）。
//   - **`attachment.dropBlocked`**（`:39`）：官方在 `!canAcceptDrop` 时用它当标题
//     （`OFF:client/labels.ts` 的 `dropOverlayLabels(t, canAcceptDrop, …)`）。本包在没有
//     sink 时**连 `FileDropOver` 都不消费**（`drop-events.ts:50-62` 与 `:71-81` 同一道门：
//     不接受就 `preventDefault` 都不做），悬停态根本置不起来 → 这个字符串仍然没有可见
//     落点，本文件不画它（常量保留在 rail.go，理由也留在那里）。
package dsattachment

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// `.mask` 的底色：官方 `--dsw-alias-bg-mask-drop`，两档都是 70% 不透明
//
//	OFF:dsh-client-ui-theme/src/styles/design-platform.css:188  rgba(255, 255, 255, 0.7)
//	OFF:…/design-platform.css:340                               rgba(39, 39, 48, 0.7)
const dropMaskAlpha float32 = 0.7

// `.title` 的字体：`--dsw-font-l-20` = `500 20px/28px`
//
//	OFF:dsh-client-ui-theme/src/styles/gradient-shadow-text.css:192-196
const (
	dropTitleFontSize float32 = 20 // font-size
	dropTitleLineH    float32 = 28 // line-height
	dropTitleWeight   float32 = 500
)

// dropMaskColor 取官方 `--dsw-alias-bg-mask-drop` 的当档取值（亮/暗各一个原值）。
//
// 这里**用的是官方原值**，不是「找一档近似色」——`pluginapi.Theme` 没有这个令牌
// （rail.go 的 `drawProgress` 那种「借最轻一档描边色」的做法是给"官方本来就没有值"用的，
// 而这里官方值就在上面两行）。两档的分界就是主题自己的 `Dark`。
func dropMaskColor(t *pluginapi.Theme) ui.Color {
	if t.Dark {
		return gfx.RGBA(39, 39, 48, dropMaskAlpha)
	}
	return gfx.RGBA(255, 255, 255, dropMaskAlpha)
}

// dropTitleText 是这一帧该显示的标题。
//
// 官方 `dropOverlayLabels(t, disabled, limits)` 只有两种标题：接受 → `attachment.dropTitle`，
// 拒绝 → `attachment.dropBlocked`。本包只画得出「接受」那一种（拒绝态根本进不了这里，
// 见文件头「缺什么」最后一条），所以这里恒返回 dropTitle。
func dropTitleText() string { return zhDropTitle }

// drawDropOverlay 画官方 `DropOverlay` 的「压层 + 标题」。没有悬停态就**一个节点都不画**
// （官方 `{dragActive && <DropOverlay …/>}`，`OFF:client/ComposerAttachments.tsx:56-61`）。
func (p *Plugin) drawDropOverlay(f ui.C, env *pluginapi.Env) {
	t := p.theme
	if t == nil {
		return
	}
	// 上一帧的几何作废：测试据此断言"零节点"（与 rail.go 的 p.boxes 同一条规矩）。
	p.overlayBox = ui.Box{}
	if !p.dragActiveNow() {
		return
	}
	// `.mask`：铺满整个框架、内容居中、**不吃指针**（官方 `pointer-events:none`：
	// 这层是纯装饰，拖放的接受/拒绝由 `OnFileDrop` 那条原生通道自己判，所以这里
	// 一个 `Hit` 都不声明）。
	mask := ui.Row(f).FillWidth().FillHeight().Background(dropMaskColor(t)).
		Items(ui.Center).Justify(ui.Center)
	p.overlayBox = mask
	mask.Children(func() {
		// `.wrap`：column / items-center / 居中。`.illustration` 缺席，所以这里只有标题。
		ui.Column(f).Items(ui.Center).Children(func() {
			ui.Text(f, dropTitleText()).
				FontSize(dropTitleFontSize).LineHeight(dropTitleLineH).Weight(dropTitleWeight).
				Color(t.LabelPrimary)
		})
	})
}

// dragActiveNow 读悬停态（写方在 `HandleFileDrop` 里，走同一把锁）。
func (p *Plugin) dragActiveNow() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dragActive
}
