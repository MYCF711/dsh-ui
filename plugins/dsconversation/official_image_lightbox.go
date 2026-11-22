// official_image_lightbox.go —— 正文图片的「查看大图」灯箱（`#18` (a)）。
//
// # 官方形状（rc.5，逐条 file:line / 产物 byte 偏移）
//
// 触发点（`OFF:dist/assets/index-Ciz7DZxv.js` byte 978669 的 `lK`，正文 `![alt](目的地)`）：
//
//	<button type="button" className={Lt.imageButton} title={labels.open}
//	        aria-label={alt ? `${labels.open}: ${alt}` : labels.open}
//	        onClick={() => setOpen(true)}>
//	  <img className={Lt.image} … />
//	</button>
//	{open && <Jb src={src} alt={alt} labels={labels} onClose={() => setOpen(false)} … />}
//
//	._imageButton_13jw0_471{display:inline-block;max-width:100%;padding:0;border:0;
//	                        border-radius:var(--dsw-radius-sm);background:transparent;
//	                        vertical-align:middle;cursor:zoom-in}
//	（`OFF:dist/assets/index-CSAdOBx6.css` byte 46122）
//
// 灯箱本体（同 js byte 969168 的 `Jb`，`createPortal` 到 `document.body`）：
//
//	<div role="dialog" aria-modal="true" aria-label={labels.dialog}
//	     className={styles.backdrop}>                    ← ._backdrop_19zw2_2
//	  <div className={styles.stage}>                     ← ._stage_19zw2_9
//	    <div className={styles.mask} aria-hidden="true" onMouseDown={onClose}/>
//	    <img className={styles.image} src={src} alt={alt} {...contextMenuProps}/>
//	    <button className={styles.close} aria-label={labels.close} onClick={onClose}>
//	      <IconCloseOutlineRegular size={16}/>
//	    </button>
//	  </div>
//	</div>
//
// 几何与材质（全部取自 `index-CSAdOBx6.css`，byte 是规则起点）：
//
//	._backdrop_19zw2_2  (byte 70484) position:fixed;inset:0;z-index:1000
//	._stage_19zw2_9     (byte 70539) position:absolute;inset:var(--dsh-frame-chrome-top,0px) 0 0;
//	                                 display:flex;align-items:center;justify-content:center;padding:40px
//	._mask_19zw2_19     (byte 70684) position:absolute;inset:0;
//	                                 background:var(--dsw-alias-bg-mask-1);
//	                                 backdrop-filter:var(--dsw-mask-blur)
//	._image_19zw2_26    (byte 70801) position:relative;max-width:min(100%,1600px);max-height:100%;
//	                                 object-fit:contain;border-radius:var(--dsw-radius-lg);
//	                                 background:var(--dsw-specific-input-major);border:0;
//	                                 box-shadow:var(--dsw-elevation-prominent)
//	._close_19zw2_37    (byte 71027) position:absolute;top:20px;right:20px;z-index:1;display:grid;
//	                                 place-items:center;width:36px;height:36px;
//	                                 border:.5px solid var(--dsw-alias-border-l2-darkmode-thin);
//	                                 border-radius:999px;corner-shape:round;
//	                                 background:var(--dsw-specific-input-major);
//	                                 color:var(--dsw-alias-label-primary);cursor:pointer
//
// 令牌取值（`OFF:dsh-client-ui-theme/src/styles/`）：
//
//	--dsw-alias-bg-mask-1            亮 rgba(0,0,0,.24) / 暗 rgba(0,0,0,.5)   design-platform.css:184 / :336
//	--dsw-alias-border-l2-darkmode-thin 亮 rgba(0,0,0,.1) / 暗 rgba(255,255,255,.06)  :196 / :348
//	--dsw-mask-blur = none                                                  gradient-shadow-text.css:19
//	--dsw-elevation-stroke-color 默认 = --dsw-alias-border-l4               gradient-shadow-text.css:16
//	--dsw-radius-lg = 16px                                                  base.css
//
// 文案（`OFF:dsh-client-ui-chat/src/client/locale.ts:75-79` 的 zh 段，逐字）：
// `image.open` 查看大图 / `image.dialog` 图片预览 / `image.close` 关闭图片预览。
//
// # 关闭方式与键盘（官方 `Jb` 的 useEffect，同 byte 969168）
//
//	Esc             window keydown **capture** 阶段：`d.key === 'Escape'` →
//	                `d.stopPropagation()` + onClose（所以下层快捷键收不到这个键）
//	遮罩 mousedown  `onMouseDown={onClose}` 挂在 `._mask` 上——**只**遮罩，图片没有
//	                处理函数（点图不关：图在 mask 之上，事件不冒泡到 mask）
//	关闭按钮 click  `._close` 的 onClick
//	Tab             `d.key === 'Tab'` → preventDefault + `closeRef.current.focus()`
//	                （焦点锁在这 36px 按钮上；没有别的可聚焦元素）
//
// **没有**方向键翻图、没有缩放/手势、没有轮播：官方这把键位表里只有 Escape 与 Tab。
//
// 焦点：打开时 `closeRef.current?.focus()`，卸载时把焦点还给打开前的元素
// （`previousRef.current?.focus()`）。`role="dialog"` / `aria-modal="true"` /
// `aria-label={image.dialog}` 三件套照写。
//
// # 本仓怎么画（映射，不是重设计）
//
//	viva 没有 portal；本仓的 body 级等价物是 `pluginapi.SlotShellOverlay`，
//	它的盒子**正好**是官方 `_stage` 的 `inset: var(--dsh-frame-chrome-top,0px) 0 0`
//	（壳在 Windows 上把 `env.Bounds` 定成扣掉 40px caption 的内容区，
//	overlay 层铺满它；见 `cmd/app/main.go` 的 `contentBounds` 与
//	`internal/uikernel/shell_skeleton.go` 的 `AppFrame.overlayLayer`）——
//	所以遮罩、图片居中、关闭按钮的 20px 内缩都不需要任何换算。
//
//	官方              本仓
//	._mask            全窗 `Absolute().Inset(0).Background(mask 色)`
//	._image           等比 fit 到 `min(100%,1600px) × 100%` 的居中盒子 + radius 16
//	                  + input-major 底 + `--dsw-elevation-prominent`
//	._close           36×36 圆、右/上 20、`.5px` thin 描边、`ScheduleCloseOutline`
//	                  （官方 `IconCloseOutlineRegular` 素材，16px）
//	role=dialog 一族  `RoleGroup` + `SkipFocus` 的遮罩 + 焦点 scope/TrapFocus
//	                  （viva 没有 RoleDialog，见 dsstatusbar 的同一处取舍）
//	Esc               `ui.Shortcut(f, 0, "escape")` = 页面级 capture，先于
//	                  `kernel.HandleKey`（`cmd/app/main.go` 的 `cfg.OnKey` 先把键
//	                  送 `ui.Page`），等价官方 capture + stopPropagation
//	Tab               焦点 scope + `TrapFocus` + `SetInitialFocus(关闭按钮)`，
//	                  等价官方 preventDefault + focus(close)
//
// 两处如实登记的差异（都不改行为，只记录）：
//
//	① `cursor:zoom-in`：viva 的 `gfx.PointerCursor` 没有 zoom-in 这一档
//	   （Default/Text/Hand/Crosshair/Wait/Resize*/NotAllowed/Grabbing/Hidden/Image），
//	   这里用 `PointerCursorHand`（可点的手型）——形态不同，语义相同。
//	② `contextMenuProps`：官方把图片的右键菜单属性**同时**传给正文图与灯箱图。
//	   本仓正文图这一层还没有图片右键菜单（`plugins/dsfileactions` 的 image 菜单
//	   没有接到正文图上），灯箱不单独造一份——两处一致地没有，而不是只有一处有。
package dsconversation

import (
	"math"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方几何常量（读数见文件头；改这里等于改灯箱，逐条有出处）。
const (
	// ImageLightboxStagePadding 是 `._stage_19zw2_9{padding:40px}`。
	ImageLightboxStagePadding float32 = 40
	// ImageLightboxImageMaxWidth 是 `._image_19zw2_26{max-width:min(100%,1600px)}` 的 1600。
	ImageLightboxImageMaxWidth float32 = 1600
	// ImageLightboxImageRadius 是 `._image_19zw2_26{border-radius:var(--dsw-radius-lg)}` = 16。
	ImageLightboxImageRadius float32 = 16
	// ImageLightboxCloseSize 是 `._close_19zw2_37{width:36px;height:36px}`。
	ImageLightboxCloseSize float32 = 36
	// ImageLightboxCloseInset 是 `._close_19zw2_37{top:20px;right:20px}`。
	ImageLightboxCloseInset float32 = 20
	// ImageLightboxCloseBorder 是 `._close_19zw2_37{border:.5px solid …}`。
	ImageLightboxCloseBorder float32 = 0.5
	// ImageLightboxCloseIcon 是官方 `<IconCloseOutlineRegular size={16}/>`。
	ImageLightboxCloseIcon float32 = 16
)

// 官方文案（`dsh-client-ui-chat/src/client/locale.ts:75-79` 的 zh 段，逐字）。
const (
	// ImageLightboxOpen 是 `image.open`（正文图的悬停提示 / 可访问名）。
	ImageLightboxOpen = "查看大图"
	// ImageLightboxDialog 是 `image.dialog`（`role=dialog` 的 aria-label）。
	ImageLightboxDialog = "图片预览"
	// ImageLightboxClose 是 `image.close`（关闭按钮的 aria-label）。
	ImageLightboxClose = "关闭图片预览"
)

// 交互 id 的名字（一律经 `env.Tree.ID(env.Feat, …)` 生成，不写死数字）。
const (
	// lightboxBackdropName 是覆盖全窗的遮罩（官方 `._mask` 的 onMouseDown）。
	lightboxBackdropName = "image-lightbox-backdrop"
	// lightboxCloseName 是关闭按钮（官方 `._close`）。
	lightboxCloseName = "image-lightbox-close"
	// lightboxScopeName 是模态焦点 scope（官方 dialog 的焦点语义）。
	lightboxScopeName = "image-lightbox-modal"
	// lightboxImageHitName 是灯箱里那张图自己的命中区：**只用于吃掉点击**。
	//
	// 为什么需要它：官方点图不关闭（图在 `._mask` 之上、`img` 没有 handler）。
	// 本仓遮罩是全窗命中区，如果不给图一个命中区，点图会落到遮罩上直接关掉——
	// 那就比官方多一种"手一滑就关"的行为。
	lightboxImageHitName = "image-lightbox-image"
)

// imageLightboxMaskColor 是 `--dsw-alias-bg-mask-1`（亮 .24 / 暗 .5，两处出处见文件头）。
//
// `--dsw-mask-blur` 是 `none`，所以官方这层**没有**背景模糊：只有纯色压暗。
func imageLightboxMaskColor(t *pluginapi.Theme) ui.Color {
	alpha := float32(0.24)
	if t != nil && t.Dark {
		alpha = 0.5
	}
	return ui.RGBA(0, 0, 0, alpha)
}

// imageLightboxCloseBorderColor 是 `--dsw-alias-border-l2-darkmode-thin`。
//
// 它**不等于** `--dsw-alias-border-l2`：亮色两者都是 .1，暗色这里 .06、l2 是 .12
// （design-platform.css:196/348 vs :197/349），所以不能拿 BorderL2 顶替。
func imageLightboxCloseBorderColor(t *pluginapi.Theme) ui.Color {
	if t != nil && t.Dark {
		return ui.RGBA(255, 255, 255, 0.06)
	}
	return ui.RGBA(0, 0, 0, 0.1)
}

// markdownLightbox 是灯箱的开合态（官方 `{open && <Jb …/>}`：关着就没有这个组件）。
type markdownLightbox struct {
	// session / cwd 是打开时那张图所在的位置：切会话/切目录之后官方那段 DOM
	// 已经卸载，灯箱自然跟着消失，这里用同一条判据主动关掉。
	session string
	cwd     string
	// destination 是作者写在正文里的目的地（官方 `src`）。
	destination string
	// alt 是官方 `<img alt>`：灯箱图沿用正文那个 alt，不另取名。
	alt string
	// bitmap 是打开那一刻的位图（官方灯箱图用的是同一个 `src`，不重新取数）。
	bitmap gfx.Bitmap
}

// openMarkdownLightbox 记下要放大的那张图。
//
// 只在 `MarkdownImageReady` 时调用：字节没到/失败时官方根本没有 `<img>` 可点
// （那时正文画的是 alt 文本），所以不存在"点开一个空灯箱"的路径。
func (v *OfficialTranscriptView) openMarkdownLightbox(env *pluginapi.Env, destination, alt string,
	image mdlite.MarkdownImage) {
	if v == nil || env == nil || image.State != mdlite.MarkdownImageReady {
		return
	}
	if image.Bitmap.Width <= 0 || image.Bitmap.Height <= 0 {
		return
	}
	v.lightbox = &markdownLightbox{
		session:     sessionIDOf(env),
		cwd:         cwdOf(env),
		destination: destination,
		alt:         alt,
		bitmap:      image.Bitmap,
	}
}

// closeMarkdownLightbox 关灯箱（三种官方关闭方式都走这一条）。
func (v *OfficialTranscriptView) closeMarkdownLightbox() {
	if v == nil {
		return
	}
	v.lightbox = nil
}

// markdownLightboxVisible 报告这一帧要不要画灯箱，并顺手关掉"已经不属于这一帧"的。
//
// 判据只有一条：打开时的 (session, cwd) 还是当前这一帧的。官方的灯箱挂在
// 那条消息的组件树上，会话一换组件就卸载——同一条语义，不用另一套判断。
func (v *OfficialTranscriptView) markdownLightboxVisible(env *pluginapi.Env) bool {
	if v == nil || v.lightbox == nil {
		return false
	}
	if env == nil || v.lightbox.session != sessionIDOf(env) || v.lightbox.cwd != cwdOf(env) {
		v.lightbox = nil
		return false
	}
	return true
}

// markdownLightboxOpen 报告灯箱此刻是否开着（`HandleKey` 与测试读它）。
func (v *OfficialTranscriptView) markdownLightboxOpen() bool {
	return v != nil && v.lightbox != nil
}

// lightboxImageFit 按官方的两条 max-* 规则算灯箱里那张图的显示尺寸（逻辑像素）。
//
//	._image_19zw2_26{max-width:min(100%,1600px);max-height:100%;object-fit:contain}
//	._stage_19zw2_9 {padding:40px}
//
// 100% 是 stage 的**内容盒**：宽 = bounds.Width-80、高 = bounds.Height-80。
// `width/height:auto` + `object-fit:contain` ⇒ 从不放大，只在两个上限里等比缩。
// 与正文图的 `mdlite.FitMarkdownImage` 同一条推导，只是上限换成 1600/窗口高。
func lightboxImageFit(bitmap gfx.Bitmap, bounds gfx.Rect) (float32, float32) {
	if bitmap.Width <= 0 || bitmap.Height <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return 0, 0
	}
	maxWidth := bounds.Width - 2*ImageLightboxStagePadding
	maxHeight := bounds.Height - 2*ImageLightboxStagePadding
	if maxWidth <= 0 || maxHeight <= 0 {
		return 0, 0
	}
	if maxWidth > ImageLightboxImageMaxWidth {
		maxWidth = ImageLightboxImageMaxWidth
	}
	width, height := float32(bitmap.Width), float32(bitmap.Height)
	scale := float32(1)
	if width > maxWidth {
		scale = maxWidth / width
	}
	if height*scale > maxHeight {
		scale = maxHeight / height
	}
	return float32(math.Round(float64(width * scale))), float32(math.Round(float64(height * scale)))
}

// drawMarkdownLightbox 是 `shell.overlay` 上的灯箱（官方 portal 到 body 的那一段）。
//
// 一条硬规矩：**只有用户点了正文图才会走到这里**（`v.lightbox` 只由
// `drawMarkdownImage` 的 `Clicked()` 写）。这里不读任何状态去"顺带弹一个"。
func (v *OfficialTranscriptView) drawMarkdownLightbox(f ui.C, env *pluginapi.Env) {
	if env == nil || !v.markdownLightboxVisible(env) {
		return
	}
	t := v.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	if t == nil || env.Tree == nil {
		return
	}
	bounds := env.Bounds
	lightbox := v.lightbox

	// ① 模态层：焦点 scope + 陷阱 + 打开时把焦点放进关闭按钮、关掉时还给原处。
	//
	// 官方是 `role="dialog" aria-modal="true"` + `closeRef.current?.focus()` +
	// Tab 锁在关闭按钮 + 卸载时 `previousRef.current?.focus()`。viva 没有
	// RoleDialog（`gfx.Role` 一览里只有 Group 能表达"成组的非动作区域"，
	// 与 dsstatusbar 的统计弹窗同一处取舍），焦点四件事由这一层声明兑现。
	scope := env.Tree.ID(env.Feat, lightboxScopeName)
	closeID := env.Tree.ID(env.Feat, lightboxCloseName)
	backdropID := env.Tree.ID(env.Feat, lightboxBackdropName)
	imageID := env.Tree.ID(env.Feat, lightboxImageHitName)

	layer := ui.Row(f).Absolute().Inset(0)
	layer = layer.Draw(func(d *ui.DrawContext) {
		d.Canvas.PushFocusScope(scope)
		d.Canvas.TrapFocus(scope)
		d.Canvas.ReturnFocus(scope)
		d.Canvas.SetInitialFocus(closeID)
	}).DrawOver(func(d *ui.DrawContext) { d.Canvas.PopFocusScope() })

	layer.Children(func() {
		// ② 遮罩：`._mask`（`inset:0` + bg-mask-1）。点它 = 官方的 onMouseDown=onClose。
		//
		// `SkipFocus`：它是整窗的关闭面，不该成为 Tab 的停靠点（与设置遮罩同一处）；
		// `Backdrop`：它盖住窗口但不挡窗口拖动。
		mask := ui.Row(f).Absolute().Inset(0).Background(imageLightboxMaskColor(t)).
			HitWith(backdropID, ImageLightboxDialog, gfx.HitOptions{
				Role: gfx.RoleGroup, SkipFocus: true, Backdrop: true,
			})
		if mask.Clicked() {
			v.closeMarkdownLightbox()
			return
		}

		// ③ Esc：官方 capture 阶段的 `keydown` + `stopPropagation()`。
		//
		// `ui.Shortcut` 是页面级声明，由 `ui.Page` 在 `kernel.HandleKey` **之前**
		// 路由（`cmd/app/main.go` 的 `cfg.OnKey`），所以这一下既关灯箱，也不会
		// 顺带把下层（停止当前 turn / 拒绝审批）的动作带出来——与官方
		// stopPropagation 一致。
		if ui.Shortcut(f, 0, "escape") {
			v.closeMarkdownLightbox()
			return
		}

		// ④ 图本身：`._image`（等比 fit + radius 16 + input-major 底 + prominent 阴影）。
		//
		// 阴影画在盒子的 `Draw` 钩子里（= 子元素之前），所以盒子**不能** `.Clip()`
		// ——viva 先上 Clip 再跑 Draw 钩子，带 Clip 会把外扩的那圈阴影整层裁掉
		// （`dscore.PaintElevation` 的注释同一句）。圆角由 `ui.Image` 自己的
		// `Radius` 兑现（`base/paint.go` 的 `kindImage` 分支把它传给
		// `DrawBitmapShaded`），不需要裁剪。
		width, height := lightboxImageFit(lightbox.bitmap, bounds)
		if width > 0 && height > 0 {
			left := (bounds.Width - width) / 2
			top := (bounds.Height - height) / 2
			frame := ui.Row(f).Absolute().Left(left).Top(top).Size(width, height).
				Radius(ImageLightboxImageRadius).Background(t.InputMajor).
				Draw(func(d *ui.DrawContext) {
					dscore.PaintElevation(d, ImageLightboxImageRadius, t.Shadows.Prominent, t.BorderL4)
				}).
				// 吃掉点击：官方点图不关（`img` 没有 handler，事件不冒泡到 `._mask`）。
				HitWith(imageID, lightbox.alt, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
			frame.Children(func() {
				ui.Image(f, lightbox.bitmap).Size(width, height).Radius(ImageLightboxImageRadius)
			})
		}

		// ⑤ 关闭按钮：`._close`（36×36 圆、右上 20、.5px thin 描边、16px 关闭叉）。
		//
		// 素材是官方 `IconCloseOutlineRegular`（`dsassets.ScheduleCloseOutline`
		// 就是那两条 path：`M2.5 2.5L13.5 13.5` / `M13.5 2.5L2.5 13.5`，viewBox 16、
		// Regular 描边 1），显示 16px —— 不手画。
		closeButton := ui.Row(f).
			Absolute().
			Left(bounds.Width-ImageLightboxCloseInset-ImageLightboxCloseSize).
			Top(ImageLightboxCloseInset).
			Size(ImageLightboxCloseSize, ImageLightboxCloseSize).
			Radius(ImageLightboxCloseSize/2).
			Border(ImageLightboxCloseBorder, imageLightboxCloseBorderColor(t)).
			Background(t.InputMajor).
			Items(ui.Center).Justify(ui.Center).
			HitWith(closeID, ImageLightboxClose, gfx.HitOptions{
				Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
			})
		closeButton.Children(func() {
			if icon, ok := dsassets.ScheduleCloseOutline(t.LabelPrimary); ok {
				ui.Icon(f, icon).Size(ImageLightboxCloseIcon, ImageLightboxCloseIcon).Color(t.LabelPrimary)
			}
		})
		if closeButton.Clicked() {
			v.closeMarkdownLightbox()
		}
	})
}

// drawImageLightbox 是 `shell.overlay` 的 painter：把灯箱画在整窗浮层上。
//
// 没有渲染器（老路径/没画过官方 transcript）或灯箱关着时**一个像素都不画**——
// 这一条是"不许新增非用户触发的浮层"的机器判据：没点过图，这一帧的 overlay
// 里就没有这块画。
func (p *Plugin) drawImageLightbox(f ui.C, env *pluginapi.Env) {
	if p == nil || p.officialTranscript == nil || env == nil {
		return
	}
	p.officialTranscript.drawMarkdownLightbox(f, env)
}

// closeImageLightbox 关灯箱，返回"这一下有没有真的关掉一个"。
//
// `HandleKey` 的 Esc 兜底用它：灯箱开着时 Esc 只关灯箱，不落到
// 「拒绝审批 / 停止当前 turn」那些下层动作（官方 `stopPropagation()` 同义）。
// 正常路径上 Esc 由页面级 `ui.Shortcut` 接（在 `kernel.HandleKey` 之前），
// 这里只在页面没接住时兜底（例如某个 viva 浮层正握着键盘）。
func (p *Plugin) closeImageLightbox() bool {
	if p == nil || p.officialTranscript == nil || !p.officialTranscript.markdownLightboxOpen() {
		return false
	}
	p.officialTranscript.closeMarkdownLightbox()
	return true
}
