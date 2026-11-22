// official_message_image_drag.go —— 附件图（`MessageImage`）的**拖出**（`#19` ③）。
//
// # 官方那一层是什么（先核，再写）
//
// `MessageImage.tsx:152` 成功态画的是 `<DraggableImage fileDrag src alt name />`。
// `DraggableImage` 本身只是 `<img draggable>` 加一串手势（`OFF:dist/assets/index-Ciz7DZxv.js`
// byte 931341 的 `xb` = `forwardRef` → `{...rest} src draggable:!0`），真正干活的是
// `dsh-client-ui-file-drag` 的 `FileTransfers.prepare`（`OFF:…/src/client/transfers.ts:49-87`）：
//
//	① `source.resourceAddress !== undefined` → 向桌面桥**借**那个已经存在的原生文件
//	② 否则读字节 → `prepareNative(desktop, {kind:'bytes', …})`
//
// **关键读数（本轮核出来的）**：`MessageImage` 传给 `DraggableImage` 的 props 是
// `{ fileDrag, src, alt, name, style }`——**没有 `resourceAddress`、没有 `original`**
// （`OFF:…/dsh-client-ui-attachment/src/MessageImage.tsx:152`）。而 `DraggableImage`
// 把 source 定成 `original === undefined ? {kind:'url', url:src, name, mediaType:"",
// resourceAddress} : {kind:'blob', …}`。所以消息附件图这一档 **`resourceAddress` 恒为
// undefined ⇒ 永远走 ②**，而 ② 的产物就是「桌面壳替我们写一个临时文件」
// （`OFF:…/dsh-client-ui-file-drag/README.md`「Desktop … owns **temporary native
// exports**」）。借文件那条 ① 只有侧栏文档预览那类**本来就带资源地址**的源才走。
//
// 结论：**消息附件图的拖出，官方本身就是「先落一个临时文件」**。账本 `#19` ③ 把
// 这条记成「必须先落临时文件 ⇒ 多出代价」是把另一档的成本记到了这一档头上——
// 对本源而言落临时文件是**复刻**，不是新增代价。
//
// # viva 那一侧的契约
//
//	// viva@app/drag_files.go:62（本仓 go.mod 的 replace → MYCF711/viva v0.6.12-ow）
//	func StartFileDrag(paths []string, allowed FileDragEffect) (FileDragEffect, error)
//
// 逐条约束（都写在 `viva@internal/platform/windows/drag_windows.go:389-456` 的注释里）：
//
//   - 收的是**绝对路径列表**，最多 `MaxDragFiles`（4096）个；相对路径直接报错
//   - **文件在松手那一刻必须还在原处**：「The files are read while the drop happens,
//     so they must still exist where they were when the user lets go.」
//   - **阻塞**：内部跑 `DoDragDrop`，一直不返回直到松手 / Esc，返回目标真正接受的 effect
//   - 必须在**窗口线程**上、且在指针事件的处理函数里调用（`getCurrentThread.Call()`
//     与 `state.application.thread` 比对，不一致就报错）——所以发起点在壳的
//     `app.Config.OnPointer`（页面绘制之外），与 `plugins/dsfiles/drag_files.go` 同一处
//
// 所以本文件要做的只有三件：**把字节写成带正经文件名的临时文件 → 交给 StartFileDrag →
// 拖完立刻删掉**。没有「生命周期管理」可言：`StartFileDrag` 是同步阻塞的，临时文件的
// 寿命就是一个 defer 的作用域，比官方的 prepare/start/release/cancel 四段状态机还短。
//
// # 为什么临时文件得放在一个专属目录里
//
// `CF_HDROP` 交给目标的是**路径**，资源管理器显示的是**路径的文件名**。所以文件必须
// 叫用户认得的名字（`shot.png`），不能叫 `blob` 或随机串——而同一个目录里放第二张图时
// 名字会撞。官方桌面桥的临时导出同理是按请求独立命名的，这里用
// `os.MkdirTemp` 一人一目录，目录整个删掉即可。
//
// # 文件名（`resolveDragFileName` 的移植，`OFF:…/dsh-client-ui-file-drag/lib/client.js`）
//
// 官方的规则逐条是：显式 `outputExtension` 优先 → 声明 MIME 的默认扩展名（当原名的后缀
// 与声明 MIME **不兼容**、或原名根本没有后缀时）→ 否则保留原名后缀；然后清掉
// `<>:"/\|?*` 与控制字符、掐掉首尾的点与空格、Windows 保留设备名前面加 `_`、
// 最后按 **UTF-8 字节**把主体截到 255 减去后缀（后缀最多 32 字节）。
// 官方有一张完整的 MIME→扩展名表；本拖出源只可能是图片，所以这张表只覆盖图片
// MIME（见 `imageDragExtension`），表里没有的类型**保持调用方原名**——这正是官方
// `preferred === null` 那一支的行为，不是简化。
//
// # 与官方的两处登记差异
//
//	① 失败反馈：官方是 `shell.overlay` 上的 `Toast`（`src/client/feedback.tsx:23-26`，
//	   文案 `file-drag.failed`=「拖放失败，请重试」）。本仓没有浮条服务，落状态栏
//	   （`Actions.Notify`），与 `#19` ② 记的既存差异同一条。**不新增弹窗。**
//	② 视觉反馈：官方浏览器给的是 `draggable` 的原生拖影（`<img>` 的缩略图幽灵）。
//	   viva 的 OLE 拖出源没有 `IDragSourceHelper`，所以没有拖影，只有系统给的效果
//	   光标（复制/移动）。这是 Win32 从资源管理器拖文件时**本来就有**的那一套反馈。
//
// # 与灯箱的关系（同一块命中区，两种手势）
//
// 官方 `.frame` 是 `<button onClick>` 里套 `DraggableImage`，所以「点一下开灯箱」与
// 「按下拖出去」共用一个命中区；官方用 `onClickCapture` 里那个 `dragging` 标志
// （`xb` 的 `c.current`）保证**拖过之后不再开灯箱**。本仓同一条判据靠机制达成：
// 发起拖出的那次 `PointerMove` 被壳判为已消费（不投递给页面），松手又被
// `DoDragDrop` 的模态循环吃掉，页面根本收不到一次完整的按下-松开 ⇒ 不会产生 `Clicked()`。
// `TestImageDragDoesNotOpenTheLightbox` 钉住这一条。
package dsconversation

import (
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// imageDragThreshold 是"按下"变成"拖出"的位移阈值（像素，逻辑坐标）。
//
// 与 `plugins/dsfiles/drag_files.go` 的 `fileDragThreshold` 同一个数、同一个理由：
// 没有阈值，点一下图就等于开始拖拽，「点开灯箱」会全部失效。官方那边这个阈值由
// 浏览器的 `draggable` 给（Chromium 是系统 drag threshold），本仓自己拿一个 4px。
const imageDragThreshold = 4

// imageDragEffects 是图片拖出提供的下场效果：复制与移动。
//
// 官方浏览器通道写死 `effectAllowed='copy'`（`transfers.ts:148`），桌面桥按
// `effectAllowed` 给。这里给"复制 + 移动"由目标按修饰键选——与从资源管理器
// 自己拖一张图出来一致；链接不给：`CF_HDROP` 里没有 shell link，给了是空承诺。
const imageDragEffects = app.FileDragCopy | app.FileDragMove

// startImageFileDrag 是发起 OS 拖出的唯一出口。
//
// 测试替换它（真实拖拽没法在单测里按鼠标，而且它会阻塞到松手），真机走
// `app.StartFileDrag`。与 `plugins/dsfiles` 的同名变量同一形状。
var startImageFileDrag = app.StartFileDrag

// imageDragTempPrefix 是临时目录前缀（`os.MkdirTemp` 会在后面接随机串）。
const imageDragTempPrefix = "dsh-go-image-drag-"

// imageDragPress 是一次按下、还没变成拖出的状态。零值 = 没在按。
type imageDragPress struct {
	pressing     bool
	attachmentID string  // 被按下的那张附件（`attachment.get` 的身份）
	x, y         float32 // 按下时的逻辑坐标，判阈值用
}

// moved 报告指针是否已经离开按下点足够远，也就是"按下"该变成"拖出"了。
func (press imageDragPress) moved(x, y float32) bool {
	return math.Hypot(float64(x-press.x), float64(y-press.y)) >= imageDragThreshold
}

// noteImageDragSource 把本帧这张图登记成可拖出源（`openHit` → 附件 id）。
//
// 只由 `drawMessageImage` 的成功态调用：loading / 失败态没有字节，拖出去只会是一个
// 空文件或者一次必然失败，所以那时**不登记**（壳也就不会把它当拖出源）。
func (v *OfficialTranscriptView) noteImageDragSource(openHit gfx.ID, attachmentID string) {
	if v == nil || openHit.IsZero() || attachmentID == "" {
		return
	}
	if v.imageDragSources == nil {
		v.imageDragSources = map[gfx.ID]string{}
	}
	v.imageDragSources[openHit] = attachmentID
}

// imageDragSourceFor 是壳的指针事件要的那一问：这个交互身份是不是一张可拖出的图。
func (v *OfficialTranscriptView) imageDragSourceFor(hit gfx.ID) (string, bool) {
	if v == nil || hit.IsZero() || len(v.imageDragSources) == 0 {
		return "", false
	}
	attachmentID, ok := v.imageDragSources[hit]
	return attachmentID, ok && attachmentID != ""
}

// OnPointer 是壳的 `app.Config.OnPointer` 转进来的指针事件，返回是否消费
// （消费 = 要求重画）。
//
// 它**必须**跑在窗口线程上、且在页面绘制之外：`StartFileDrag` 会在里面阻塞到用户松手
// （`viva@internal/platform/windows/drag_windows.go:399` 的那三条前置条件）。
// 不认识的身份（不是可拖出的图）、非主键、以及没按下的移动都直接放行，页面的
// 点击/灯箱/悬停照旧走运行时那条路。
func (p *Plugin) OnPointer(event gfx.PointerEvent) bool {
	view := p.officialTranscript
	if view == nil {
		return false
	}
	switch event.Kind {
	case gfx.PointerDown:
		// 按下先记下来，**不消费**：页面还要靠这一下把 `Clicked()` 置起来
		// （点一下 = 开灯箱，官方 `.frame` 的 onClick）。
		if event.Button != gfx.PointerButtonPrimary {
			return false
		}
		attachmentID, ok := view.imageDragSourceFor(event.Target)
		if !ok {
			return false
		}
		p.imagePress = imageDragPress{pressing: true, attachmentID: attachmentID,
			x: event.X, y: event.Y}
		p.imageDragErr = ""
		return false
	case gfx.PointerMove:
		if !p.imagePress.pressing || !p.imagePress.moved(event.X, event.Y) {
			return false
		}
		attachmentID := p.imagePress.attachmentID
		p.imagePress = imageDragPress{}
		// 拖出期间窗口线程在这里面（Viva 的 DoDragDrop 自己跑模态循环）。
		if err := p.dragImageOut(attachmentID); err != nil {
			// 平台不支持拖出 / 拿不到窗口 / 写临时文件失败：写清楚，不假装成功。
			p.imageDragErr = err.Error()
			p.notifyImageDragFailure()
			return true
		}
		return true
	case gfx.PointerUp, gfx.PointerExit:
		p.imagePress = imageDragPress{}
	}
	return false
}

// dragImageOut 把一张附件图写成临时文件、交给 OLE 拖出、拖完删掉。
//
// 同步阻塞到用户松手（`StartFileDrag` 的契约），所以**只能**从 `OnPointer` 里调。
func (p *Plugin) dragImageOut(attachmentID string) error {
	view := p.officialTranscript
	entry, _ := view.messageImages.lookup(attachmentID, false)
	// 只有解码成功、而且字节还在的那一张能拖：加载中 / 失败的拖出去是空文件。
	if entry.state != messageImageReady || len(entry.raw) == 0 {
		return errImageDragNotReady
	}
	directory, err := os.MkdirTemp("", imageDragTempPrefix)
	if err != nil {
		return err
	}
	// 拖完立刻删。`StartFileDrag` 是阻塞的，它返回时目标已经把文件读完了
	// （`drag_windows.go` 的注释：「The files are read while the drop happens」），
	// 所以这一次删除不会把还在被读的文件抽走。
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, resolveImageDragFileName(entry.fname, entry.mime))
	if err := os.WriteFile(path, entry.raw, 0o600); err != nil {
		return err
	}
	_, err = startImageFileDrag([]string{path}, imageDragEffects)
	return err
}

// notifyImageDragFailure 把拖出失败写进状态栏（**不弹窗**）。
//
// 官方这一处是 `shell.overlay` 上的 Toast（文案 `file-drag.failed`「拖放失败，请重试」）；
// 本仓没有浮条服务，落状态栏是既有差异（`#19` ②），原因接在文案后面。
func (p *Plugin) notifyImageDragFailure() {
	if p == nil || p.lastAct.Notify == nil || p.imageDragErr == "" {
		return
	}
	p.lastAct.Notify(imageDragFailedText + "：" + p.imageDragErr)
}

// errImageDragNotReady 覆盖不了"拖一张还没加载好的图"。
var errImageDragNotReady = errImageDragNotReadyError{}

// errImageDragNotReadyError 是那个错误的类型（写成类型是为了不被 fmt 当成动词）。
type errImageDragNotReadyError struct{}

func (errImageDragNotReadyError) Error() string {
	return "图片还没加载好，无法拖出"
}

// ── 文件名（官方 `resolveDragFileName` 的移植）─────────────────────────────

// imageDragExtension 是**图片** MIME → 默认扩展名。
//
// 官方用的是一张完整的 mime 表；本拖出源只可能是消息附件里的图片（`MessageImage`
// 只画图片附件），所以这张表只覆盖它能出现的几种。表里没有的类型让
// `resolveImageDragFileName` 走官方 `preferred === null` 那一支（保持原名），
// 不是"随便给一个扩展名"。
func imageDragExtension(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg", "image/pjpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/bmp":
		return "bmp"
	case "image/svg+xml":
		return "svg"
	}
	return ""
}

// imageDragCompatibleSuffix 报告 name 的后缀是否与 mediaType 兼容
// （官方 `getAllExtensions(declared).has(suffix)`）。
//
// 只在"这张表认识这个 MIME"时有意义：表里没有的类型官方也是 `compatible === false`，
// 但那时的 `preferred` 同样是 null，所以两个分支都不改后缀——见调用处的注释。
func imageDragCompatibleSuffix(mediaType, suffix string) bool {
	preferred := imageDragExtension(mediaType)
	if preferred == "" || suffix == "" {
		return false
	}
	suffix = strings.ToLower(suffix)
	// jpg / jpeg 是同一个 MIME 的两个常用后缀，官方那张表两个都收。
	if preferred == "jpg" {
		return suffix == "jpg" || suffix == "jpeg" || suffix == "jpe"
	}
	return suffix == preferred
}

// imageDragSanitize 照官方那条正则清理文件名：
//
//	name.replace(/[<>:"/\\|?*\x00-\x1f]/gu, '_').replace(/^[. ]+|[. ]+$/gu, "") || 'file'
func imageDragSanitize(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			b.WriteRune('_')
			continue
		}
		if r <= 0x1f {
			b.WriteRune('_')
			continue
		}
		b.WriteRune(r)
	}
	clean := strings.Trim(b.String(), ". ")
	if clean == "" {
		return "file"
	}
	return clean
}

// imageDragReservedDevice 是 Windows 的保留设备名（官方同一条正则，大小写不敏感）。
//
//	/^(con|prn|aux|nul|com[1-9¹²³]|lpt[1-9¹²³])(?:\.|$)/iu
func imageDragReservedDevice(name string) bool {
	lower := strings.ToLower(name)
	head, _, _ := strings.Cut(lower, ".")
	switch head {
	case "con", "prn", "aux", "nul":
		return true
	}
	if len(head) == 4 && (strings.HasPrefix(head, "com") || strings.HasPrefix(head, "lpt")) {
		digit := head[3]
		return (digit >= '1' && digit <= '9') || digit == '¹' || digit == '²' || digit == '³'
	}
	return false
}

// imageDragMaxNameBytes / imageDragMaxSuffixBytes 是官方那两个数：
// 主体截到 255 - 后缀字节，后缀自己最多 32 字节（`transfers.ts` 同款 `truncate`）。
const (
	imageDragMaxNameBytes   = 255
	imageDragMaxSuffixBytes = 32
)

// imageDragTruncate 按 **UTF-8 字节**截断，且不切断一个字符（官方那个 `truncate`
// 逐字符累加 `TextEncoder().encode(c).length`，超了就停）。
func imageDragTruncate(value string, bytes int) string {
	if bytes <= 0 {
		return ""
	}
	used := 0
	var b strings.Builder
	for _, r := range value {
		size := len(string(r))
		if used+size > bytes {
			break
		}
		b.WriteRune(r)
		used += size
	}
	return b.String()
}

// resolveImageDragFileName 是官方 `resolveDragFileName` 的逐条移植（对着源写，不重设计）。
//
//	显式 outputExtension 优先（本仓拖出没有这一档，故不设参数）
//	原名的后缀  后缀为空、或与声明 MIME 不兼容 → 换成 MIME 的默认扩展名
//	            否则保留原名后缀
//	然后        清理非法字符 → Windows 保留名加 `_` → 按字节截到 255（后缀≤32）
//
// 举例（本仓真会遇到的几档）：
//
//	shot.png  + image/png   → shot.png   （后缀兼容，不动）
//	blob      + image/png   → blob.png   （没后缀，补 MIME 的）
//	shot.jpg  + image/png   → shot.png   （后缀与声明 MIME 不兼容，换成声明的那一个）
//	CON       + image/png   → _CON.png
func resolveImageDragFileName(name, mediaType string) string {
	if name == "" {
		name = "file"
	}
	clean := imageDragSanitize(name)
	dot := strings.LastIndex(clean, ".")
	suffix := ""
	if dot >= 0 {
		suffix = clean[dot+1:]
	}
	preferred := imageDragExtension(mediaType)
	compatible := imageDragCompatibleSuffix(mediaType, suffix)
	extension := suffix
	if suffix == "" || (preferred != "" && !compatible) {
		extension = preferred
	}
	if extension != "" {
		if dot < 0 {
			clean = clean + "." + extension
		} else {
			clean = clean[:dot] + "." + extension
		}
	}
	if imageDragReservedDevice(clean) {
		clean = "_" + clean
	}
	finalDot := strings.LastIndex(clean, ".")
	tail := ""
	if finalDot >= 0 {
		tail = clean[finalDot:]
	}
	tail = imageDragTruncate(tail, imageDragMaxSuffixBytes)
	body := clean
	if finalDot >= 0 {
		body = clean[:finalDot]
	}
	return imageDragTruncate(body, imageDragMaxNameBytes-len(tail)) + tail
}
