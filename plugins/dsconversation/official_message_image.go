// official_message_image.go —— 官方 `MessageImage` 的 Go 复刻（#18 ②③ 图片面）。
//
// # 官方要什么（逐条抄原文，坐标都在 `OFF:` 前缀的 rc.2 随包 src 里）
//
//	MessageImage.tsx:36-48   singleFit：单图长边 240、比例夹到 [0.25, 4]、
//	                         超出的部分由 object-fit:cover 裁掉、绝不放大过原始尺寸；
//	                         裁切锚点按极端比例取 `center top` / `left center` / `center`
//	MessageImage.tsx:96-98   没有尺寸真值时单图按 240×240 方裁
//	MessageImage.tsx:118-132 失败态：整块换成 `.error` 按钮，点一下重试（`request`）
//	MessageImage.tsx:140-150 loading 态：`.loading` 文案（缩略图档是转圈图标）
//	MessageImage.tsx:152     成功态：`<img src alt style={objectPosition}>`
//	ImageGallery:174         variant = thumbnail ? thumbnail : (compact || n>1) ? tile : single
//	UserContent.tsx:18-31    用户消息里**每张图各自一个单图 gallery**，
//	                         `align:end`、`compact: attachments.length > 1`
//
// 也就是说：**只有一张图时官方画的是 `singleFit` 的大图，不是 64×64 瓦片**；
// 多张附件才降级成 64×64 tile。旧实现一律画 64×64 空槽，这一点同样是错的。
//
// # 几何（MessageImage.module.css，逐条注释在常量上）
//
//	.frame            border .5px border-l2-darkmode-thin / radius xl /
//	                  background interactive-bg-hover / cursor zoom-in /
//	                  display grid + place-items center / overflow hidden
//	.frame[tile]      64×64（min 也是 64）
//	.frame img        object-fit cover / 100%×100%
//	.loading .error   12px/18px、label-tertiary
//	.error            border .5px border-l2 / radius md / bg hover-danger /
//	                  cursor pointer / max-width 240 / padding 10px 12px
//	.error[tile]      radius xl / 64×64 / padding 4
//
// # 本仓怎么替代 `<img>`
//
// 官方 `<img src>` 的字节由浏览器取（受授权的 URL）。本仓走
// `pluginapi.Actions.AttachmentGet`（宿主 `attachment.get`，真字节）→
// `internal/imgdecode` 解码 → `ui.Image(c, gfx.Bitmap)`。
//
// **取数是异步的**：绘制帧里不许等 HTTP（同 `writeDraft` 那条规矩）。所以第一帧
// 画官方 loading 态、后台 goroutine 取字节、回来 `Invalidate()` 重画——与官方
// `useEffect` 里 `load(attachment).then(setLoaded).catch(setError)` 的三态同构。
//
// # 成功态是可点的（灯箱），并可作为拖出源
//
// 官方点图片开**原图灯箱**（`MessageImage.tsx:135-145` 的 `<button onClick>` +
// `:152` 的 `ImageLightbox`）。本仓的灯箱实现在 `official_image_lightbox.go`（与正文
// markdown 图共用同一个官方组件映射），这里只负责"成功态是按钮 + 点一下打开"。
//
// 同一张图还是 `#19` 的**拖出源**（官方 `:152` 的 `DraggableImage`）：`#19` 那道
// `StartFileDrag` 的接线在 `official_message_image_drag.go`。灯箱与拖出是同一个
// 命中区的两种手势——点一下是开灯箱，按下后越过 4px 是拖出，互不吞掉。
//
// # 失败态仍然只有重试
//
// 官方失败态整块换成 `.error` 按钮（点一下重试），**没有**灯箱可开、**没有**东西可拖：
// 那时候连字节都没取到。本仓照抄这一条（`drawMessageImageError`）。
package dsconversation

import (
	"sync"

	"github.com/MYCF711/dsh-ui/internal/imgdecode"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 `props.variant`（MessageImage.tsx:74）的三档。
const (
	// MessageImageVariantSingle 是"只有这一张图"：按 singleFit 画大图。
	MessageImageVariantSingle = "single"
	// MessageImageVariantTile 是"多张里的一张"或 `compact`：固定 64×64 方裁。
	MessageImageVariantTile = "tile"
	// MessageImageVariantThumbnail 是附件列表行里的不裁切缩略图（48×48，contain）。
	MessageImageVariantThumbnail = "thumbnail"
)

// 官方图片文案，逐字取 `OFF:dsh-client-ui-conversation/src/client/locales.ts`
// 的 zh 词典（`tmp/dsh-client-ui-conversation.js:16102-16113` 同一批字面量）。
const (
	// MessageImageLabelFallback 是 `image.label`（无名图片的兜底名）。
	MessageImageLabelFallback = "图片"
	// MessageImageOpen 是 `image.openOriginal`（悬停提示「查看原图」）。
	MessageImageOpen = "查看原图"
	// MessageImageOpenLabelSuffix 是 `image.openOriginalLabel` 的模板
	// `{label}，点击查看原图` 的固定后半段。
	MessageImageOpenLabelSuffix = "，点击查看原图"
	// MessageImageLoading 是 `image.loading`。
	MessageImageLoading = "图片加载中…"
	// MessageImageLoadFailed 是 `image.loadFailed`（失败态的文案，也是重试按钮的名字）。
	MessageImageLoadFailed = "图片加载失败，点击重试"
	// imageDragFailedText 是官方 `file-drag.failed`（`OFF:dsh-client-ui-file-drag/src/client/locales.ts`
	// 的 zh 段，逐字「拖放失败，请重试」）。拖出失败落状态栏时用它，原因接在冒号后。
	imageDragFailedText = "拖放失败，请重试"
)

// messageImageAnchor 是 `object-position` 的三档里本仓用得到的两档 + 居中。
type messageImageAnchor int

const (
	// anchorCenter 是 `object-position: center`。
	anchorCenter messageImageAnchor = iota
	// anchorCenterTop 是 `object-position: center top`（极高的图保住顶部）。
	anchorCenterTop
	// anchorLeftCenter 是 `object-position: left center`（极宽的图保住左边）。
	anchorLeftCenter
)

// singleFit 是官方 `singleFit`（MessageImage.tsx:36-48）的逐行移植。
//
// 返回显示框的宽高与裁切锚点。**不许**改成"直接按原始尺寸画"：官方这条规则是
// 刻意压住超大图的（长边 240、比例夹 [0.25,4]、不放大），也是 `#18` 里
// 「超大图」那一档的官方表现。
func singleFit(width, height int) (float32, float32, messageImageAnchor) {
	if width <= 0 || height <= 0 {
		// 官方没有尺寸真值时按 240×240 方裁（MessageImage.tsx:96-98）。
		return ImageSingleMaxEdge, ImageSingleMaxEdge, anchorCenter
	}
	natural := float64(width) / float64(height)
	ratio := natural
	if ratio > ImageSingleRatioMax {
		ratio = ImageSingleRatioMax
	}
	if ratio < ImageSingleRatioMin {
		ratio = ImageSingleRatioMin
	}
	var boxW, boxH float64
	if ratio >= 1 {
		boxW, boxH = float64(ImageSingleMaxEdge), float64(ImageSingleMaxEdge)/ratio
	} else {
		boxW, boxH = float64(ImageSingleMaxEdge)*ratio, float64(ImageSingleMaxEdge)
	}
	scale := 1.0
	if v := float64(width) / boxW; v < scale {
		scale = v
	}
	if v := float64(height) / boxH; v < scale {
		scale = v
	}
	outW := boxW * scale
	outH := boxH * scale
	if outW < 1 {
		outW = 1
	}
	if outH < 1 {
		outH = 1
	}
	anchor := anchorCenter
	switch {
	case natural < ImageSingleRatioMin:
		anchor = anchorCenterTop
	case natural > ImageSingleRatioMax:
		anchor = anchorLeftCenter
	}
	return float32(roundHalfUp(outW)), float32(roundHalfUp(outH)), anchor
}

// imageCoverCrop 把位图裁成 boxW×boxH 的比例（等价于 `object-fit: cover`）。
//
// 为什么必须自己裁：viva 的 `ui.Image` 是"缩放到填满元素矩形"
// （`viva@0.6.5/ui/frame.go:628` 的注释：`scaled to fill the element's rectangle`），
// 也就是 CSS 的 `object-fit: fill`——直接交给它会把极端比例的图**拉变形**。
// 官方是 `cover`：保持比例、多出来的部分裁掉，锚点由 `object-position` 定。
func imageCoverCrop(source gfx.Bitmap, boxW, boxH float32, anchor messageImageAnchor) gfx.Bitmap {
	if source.Width <= 0 || source.Height <= 0 || boxW <= 0 || boxH <= 0 {
		return gfx.Bitmap{}
	}
	sourceAspect := float64(source.Width) / float64(source.Height)
	boxAspect := float64(boxW) / float64(boxH)
	x0, y0 := 0, 0
	cropW, cropH := source.Width, source.Height
	if sourceAspect > boxAspect {
		// 源更宽：保住高度，横向裁。
		cropW = int(roundHalfUp(float64(source.Height) * boxAspect))
		if cropW < 1 {
			cropW = 1
		}
		if cropW > source.Width {
			cropW = source.Width
		}
		switch anchor {
		case anchorLeftCenter:
			x0 = 0
		default:
			x0 = (source.Width - cropW) / 2
		}
	} else if sourceAspect < boxAspect {
		// 源更高：保住宽度，纵向裁。
		cropH = int(roundHalfUp(float64(source.Width) / boxAspect))
		if cropH < 1 {
			cropH = 1
		}
		if cropH > source.Height {
			cropH = source.Height
		}
		switch anchor {
		case anchorCenterTop:
			y0 = 0
		default:
			y0 = (source.Height - cropH) / 2
		}
	}
	stride := cropW * 4
	pixels := make([]byte, stride*cropH)
	for y := 0; y < cropH; y++ {
		srcRow := (y0 + y) * source.Stride
		dstRow := y * stride
		copy(pixels[dstRow:dstRow+stride], source.Pix[srcRow+x0*4:srcRow+x0*4+stride])
	}
	return gfx.Bitmap{Pix: pixels, Width: cropW, Height: cropH, Stride: stride}
}

// roundHalfUp 是 JS `Math.round` 的等价物（Go 的 math.Round 也是 half-away-from-zero，
// 这里等价于官方那一次 round；写成局部函数是为了不引 math 包进绘制路径）。
func roundHalfUp(v float64) float64 {
	if v < 0 {
		return -roundHalfUp(-v)
	}
	return float64(int64(v + 0.5))
}

// ── 三态与缓存 ────────────────────────────────────────────────────────

// messageImageLoadState 对应官方 `loaded / error / 未定` 三态（MessageImage.tsx:81-83）。
type messageImageLoadState int

const (
	// messageImagePending 是"字节还没回来"（官方 `src === null`）。
	messageImagePending messageImageLoadState = iota
	// messageImageReady 是解码成功、可以画。
	messageImageReady
	// messageImageFailed 是取字节或解码失败（官方 `error === true`）。
	messageImageFailed
)

// messageImageCropKey 是裁切缓存键：同一张图在不同框尺寸下要各留一份。
type messageImageCropKey struct {
	width  int
	height int
	anchor messageImageAnchor
}

// messageImageEntry 是一张附件图的三态 + 位图 + 裁切缓存。
type messageImageEntry struct {
	state  messageImageLoadState
	bitmap gfx.Bitmap
	// width / height 是**原始像素尺寸**：宿主 `attachment.get` 给的优先，
	// 没有时才退到投影里的尺寸（官方 `dimensionsOf`，MessageImage.tsx:51-56）。
	width  int
	height int
	crops  map[messageImageCropKey]gfx.Bitmap
	// raw 是宿主给的**原始字节**（未解码）与它的 MIME / 文件名。
	//
	// 为什么缓存要留着它们：`#19` ③ 的拖出要把这张图写成临时文件再交给 OLE，
	// 而拖出的时刻**不允许**再走一次 HTTP——`OnPointer` 跑在窗口线程上，
	// 一次 8 MiB 的 `attachment.get` 能把界面卡住（同 `writeDraft` 那条规矩）。
	// 所以字节在**加载那一趟**就留下来，拖出时直接落盘。
	//
	// 与官方对齐：官方浏览器通道拿的是同一个 `File`（`FileTransfers.active.file`），
	// 桌面通道 `readSource` 也是从同一个 Blob 取字节——都不是拖出时重新取。
	raw   []byte
	mime  string
	fname string
}

// messageImageCache 是附件图的内存缓存。
//
// 为什么要有缓存：绘制每帧都会问"这张图好了吗"，而取字节是 HTTP。缓存把
// "每帧一次请求"降成"每张图一次请求"，也让加载成功后不再重取——
// 与官方 `load.peek?.(attachment)`（同步拿缓存 URL，MessageImage.tsx:82,107）同一意图。
type messageImageCache struct {
	mu      sync.Mutex
	entries map[string]*messageImageEntry
	// inflight 防止同一张图被并发的两帧各起一个请求。
	inflight map[string]bool
}

// newMessageImageCache 造一个空缓存。
func newMessageImageCache() *messageImageCache {
	return &messageImageCache{entries: map[string]*messageImageEntry{}, inflight: map[string]bool{}}
}

// lookup 取一张图的三态快照，并报告"要不要现在起一次加载"。
//
// retry=true 时把失败态重置回 pending——这是官方 `request`（`setAttempt(a=>a+1)`，
// MessageImage.tsx:85-88）的等价物。
func (c *messageImageCache) lookup(id string, retry bool) (messageImageEntry, bool) {
	if c == nil || id == "" {
		return messageImageEntry{state: messageImagePending}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[id]
	if entry == nil {
		entry = &messageImageEntry{state: messageImagePending}
		c.entries[id] = entry
	}
	if retry && entry.state == messageImageFailed {
		entry.state = messageImagePending
		entry.bitmap = gfx.Bitmap{}
		entry.crops = nil
	}
	// 快照要**完整**：`raw` 那三个字段是拖出（`#19` ③）要的，漏拷就会变成
	// "图在屏上、却被告知还没加载好"——`dragImageOut` 就是按 `raw` 判的。
	snapshot := messageImageEntry{state: entry.state, bitmap: entry.bitmap,
		width: entry.width, height: entry.height, crops: entry.crops,
		raw: entry.raw, mime: entry.mime, fname: entry.fname}
	if entry.state != messageImagePending || c.inflight[id] {
		return snapshot, false
	}
	c.inflight[id] = true
	return snapshot, true
}

// store 落一次加载结果。
func (c *messageImageCache) store(id string, entry messageImageEntry) {
	if c == nil || id == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.inflight, id)
	c.entries[id] = &entry
}

// cropFor 取（或算一次并记住）这张图在给定框里的裁切结果。
//
// 只在 Ready 时调用；缓存键包含框尺寸与锚点，所以单图/瓦片两种画法互不干扰。
func (c *messageImageCache) cropFor(id string, boxW, boxH float32, anchor messageImageAnchor) (gfx.Bitmap, bool) {
	if c == nil || id == "" {
		return gfx.Bitmap{}, false
	}
	key := messageImageCropKey{width: int(boxW), height: int(boxH), anchor: anchor}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[id]
	if entry == nil || entry.state != messageImageReady || entry.bitmap.Width == 0 {
		return gfx.Bitmap{}, false
	}
	if entry.crops == nil {
		entry.crops = map[messageImageCropKey]gfx.Bitmap{}
	}
	if crop, ok := entry.crops[key]; ok {
		return crop, crop.Width > 0
	}
	crop := imageCoverCrop(entry.bitmap, boxW, boxH, anchor)
	entry.crops[key] = crop
	return crop, crop.Width > 0
}

// loadMessageImage 在后台取一张附件的字节并解码（**必须在 goroutine 里调**）。
//
// 官方这一步是浏览器 `<img src>`：拿 URL、解字节、onload/onerror。本仓把三段
// 摊开：壳给的 `AttachmentGet` 取字节 → `imgdecode` 解码 → 缓存三态。
// 任何失败都落 `messageImageFailed`（**不是**"画个空框当成功"）。
func (c *messageImageCache) loadMessageImage(id string, get func(string) (pluginapi.AttachmentBlob, bool), redraw func()) {
	entry := messageImageEntry{state: messageImageFailed}
	if get != nil {
		if blob, ok := get(id); ok && len(blob.Bytes) > 0 {
			entry.width, entry.height = blob.Width, blob.Height
			entry.raw, entry.mime, entry.fname = blob.Bytes, blob.MIME, blob.Name
			if bitmap, err := imgdecode.Decode(blob.Bytes); err == nil {
				entry.state = messageImageReady
				entry.bitmap = bitmap
				// 宿主没报尺寸时退到位图真实尺寸（官方 `dimensionsOf` 的兜底同理）。
				if entry.width <= 0 || entry.height <= 0 {
					entry.width, entry.height = bitmap.Width, bitmap.Height
				}
			}
		}
	}
	c.store(id, entry)
	if redraw != nil {
		redraw()
	}
}

// ── 绘制 ──────────────────────────────────────────────────────────────

// messageImageLabels 是这一处图片面用到的官方文案（`MessageImageLabels` 的子集）。
//
// 官方 `lightbox` 那一组（`image.dialog` / `image.close`）不在这里：它归灯箱自己的
// 文案常量（见 `official_image_lightbox.go`，与正文图共用同一份）。
type messageImageLabels struct {
	image      string
	open       string
	loading    string
	loadFailed string
}

// officialMessageImageLabels 是 zh 词典里那一组（`locales.ts` 的 `image.*`）。
func officialMessageImageLabels() messageImageLabels {
	return messageImageLabels{
		image:      MessageImageLabelFallback,
		open:       MessageImageOpen,
		loading:    MessageImageLoading,
		loadFailed: MessageImageLoadFailed,
	}
}

// messageImageVariantFor 是官方 `ImageGallery` 那一行变体判定
// （MessageImage.tsx:174）：`thumbnail` > (`compact` || 多图 → tile) > `single`。
//
// 用户消息的调用点是 `UserContent.tsx:18-31`：每张图各自成组、`compact` 取
// `attachments.length > 1`。所以"只有一张图"才会走到 single 的大图档。
func messageImageVariantFor(imageCount int, compact bool) string {
	if compact || imageCount > 1 {
		return MessageImageVariantTile
	}
	return MessageImageVariantSingle
}

// drawMessageImage 按官方三态画一张附件图。
//
// `retryHit` 是失败态重试按钮的交互 id，`openHit` 是成功态「点开灯箱」的交互 id；
// 两者形状完全不同（官方失败态是 `.error` 按钮重新加载，成功态是 `.frame` 按钮开
// `ImageLightbox`），所以是**两个** id，不许合并：一个 id 上的两次点击会打架。
//
// 零值 id 就那一档不给点击（官方同理：没有交互身份就没有按钮）。
func (v *OfficialTranscriptView) drawMessageImage(f ui.C, env *pluginapi.Env, a TranscriptAttachment,
	variant string, retryHit, openHit gfx.ID) {
	t := v.Theme
	labels := officialMessageImageLabels()

	entry, needLoad := v.messageImages.lookup(a.ID, false)
	if a.ID == "" {
		// 没有 attachmentId 就没有取数入口（官方不会出现这一档：附件 ref 必带 id）。
		// 如实画 loading 文案，**不**画失败态——失败态会诱导用户点一个永远失败的重试。
		entry = messageImageEntry{state: messageImagePending}
		needLoad = false
	}
	if needLoad && env != nil {
		// 绘制帧里不许等 HTTP：起 goroutine，回来 Invalidate 重画
		// （对官方 useLayoutEffect 的 load().then/.catch 三态，MessageImage.tsx:103-110）。
		go v.messageImages.loadMessageImage(a.ID, env.Act.AttachmentGet, env.Act.Invalidate)
	}

	// 框尺寸：单图走 singleFit，瓦片固定 64，缩略图固定 48。
	boxW, boxH := float32(ImageTileSize), float32(ImageTileSize)
	anchor := anchorCenter
	switch variant {
	case MessageImageVariantSingle:
		width, height := a.Width, a.Height
		if entry.width > 0 && entry.height > 0 {
			width, height = entry.width, entry.height
		}
		boxW, boxH, anchor = singleFit(width, height)
	case MessageImageVariantThumbnail:
		boxW, boxH = ImageThumbnailSize, ImageThumbnailSize
	}

	switch entry.state {
	case messageImageFailed:
		v.drawMessageImageError(f, env, a, variant, boxW, boxH, labels, retryHit)
	case messageImageReady:
		if crop, ok := v.messageImages.cropFor(a.ID, boxW, boxH, anchor); ok {
			// `.frame`：描边 .5、radius xl、底 interactive-bg-hover、居中、裁掉溢出。
			// `overflow:hidden` 由 `.Clip()` 表达：圆角框要把图裁在框里
			// （官方 `.frame{...overflow:hidden}`，MessageImage.module.css:5-13）。
			label := messageImageDisplayLabel(a, labels)
			frame := ui.Row(f).Size(boxW, boxH).Radius(t.Radii.XL).Border(0.5, t.BorderL2).
				Background(t.Hover).Items(ui.Center).Justify(ui.Center).Clip()
			// 成功态的可点性：官方 `.frame` 是 `<button onClick={()=>setOpen(true)}>`
			// （MessageImage.tsx:135-145），`title = labels.open`（「查看原图」）、
			// `aria-label = labels.openNamed(label)`。没有 id 时不给假承诺。
			clickable := !openHit.IsZero()
			if clickable {
				frame = frame.HitWith(openHit, messageImageOpenLabel(label), gfx.HitOptions{
					Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
				})
				// `#19` ③ 的拖出源：本帧这张图是**可拖出**的，壳的 OnPointer 用它把
				// "按下 + 越过阈值"变成一次真 OS 拖拽（见 official_message_image_drag.go）。
				v.noteImageDragSource(openHit, a.ID)
			}
			frame.Children(func() {
				ui.Image(f, crop).Size(boxW, boxH)
			})
			if clickable && frame.Clicked() {
				// 与 markdown 图同一条：只在位图真的在时才开（上面已判 Ready）。
				v.openAttachmentLightbox(env, a.ID, label, entry)
			}
			return
		}
		// 位图在、裁切却失败（0×0）：按失败态画，不画一个空白框。
		v.drawMessageImageError(f, env, a, variant, boxW, boxH, labels, retryHit)
	default:
		// `.frame` + `.loading`：12/18、label-tertiary、居中。
		ui.Row(f).Size(boxW, boxH).Radius(t.Radii.XL).Border(0.5, t.BorderL2).
			Background(t.Hover).Items(ui.Center).Justify(ui.Center).Clip().Children(func() {
			ui.Text(f, labels.loading).FontSize(ImageStateFontSize).
				LineHeight(ImageStateLineHeight).Color(t.LabelTertiary).Ellipsis()
		})
	}
}

// messageImageDisplayLabel 是官方那个 `label`：
//
//	('attachment' in image ? image.label : undefined) ?? preview?.name
//	  ?? attachment?.name ?? labels.image      （MessageImage.tsx:113-114）
//
// 本仓的消息附件只有"已登记的 attachment"这一档（没有 composer 的本地 preview），
// 所以退化链就是 附件名 → `image.label`（「图片」）。
func messageImageDisplayLabel(a TranscriptAttachment, labels messageImageLabels) string {
	if a.Name != "" {
		return a.Name
	}
	return labels.image
}

// messageImageOpenLabel 是官方 `labels.openNamed(label)`
// = 模板 `image.openOriginalLabel`「{label}，点击查看原图」。
func messageImageOpenLabel(label string) string {
	return label + MessageImageOpenLabelSuffix
}

// drawMessageImageError 画官方失败态（`.error`，MessageImage.tsx:119-132）。
//
// 形状逐条：
//
//	generic   border .5 l2 / radius md / bg hover-danger / max-width 240 / padding 10 12 /
//	          12-18 label-tertiary / 文案 = loadFailed / 可点重试
//	tile      radius xl / 64×64 / padding 4 / 图标 = IconRefreshOutlineRegular（无文案）
//	thumbnail radius lg / 48×48 / padding 2 / 图标同上
//
// 重试是官方那一句 `onClick: request`（同处 :126）：点一下重新加载同一条附件。
func (v *OfficialTranscriptView) drawMessageImageError(f ui.C, env *pluginapi.Env, a TranscriptAttachment, variant string, boxW, boxH float32, labels messageImageLabels, hit gfx.ID) {
	t := v.Theme
	radius := t.Radii.MD
	padX, padY := ImageErrorPadX, ImageErrorPadY
	switch variant {
	case MessageImageVariantTile:
		radius = t.Radii.XL
		padX, padY = ImageErrorTilePad, ImageErrorTilePad
	case MessageImageVariantThumbnail:
		radius = t.Radii.LG
		padX, padY = ImageErrorThumbPad, ImageErrorThumbPad
	}
	row := ui.Row(f).Size(boxW, boxH).Radius(radius).Border(0.5, t.BorderL2).
		Background(t.HoverDanger).Items(ui.Center).Justify(ui.Center).
		PadSides(padY, padX, padY, padX).Clip()
	clickable := !hit.IsZero() && a.ID != ""
	if clickable {
		row = row.Hit(hit, labels.loadFailed)
	}
	row.Children(func() {
		if variant == MessageImageVariantTile || variant == MessageImageVariantThumbnail {
			if icon, ok := dsassets.MarketRefreshOutline(t.LabelTertiary); ok {
				ui.Icon(f, icon).Size(ImageErrorIconSize, ImageErrorIconSize).Color(t.LabelTertiary)
			}
			return
		}
		ui.Text(f, labels.loadFailed).FontSize(ImageStateFontSize).
			LineHeight(ImageStateLineHeight).Color(t.LabelTertiary).Ellipsis()
	})
	if clickable && row.Clicked() && env != nil {
		// 官方 `request`：把失败态重置回 pending 并重取一次。
		if _, need := v.messageImages.lookup(a.ID, true); need {
			go v.messageImages.loadMessageImage(a.ID, env.Act.AttachmentGet, env.Act.Invalidate)
		}
	}
}
