// Package dsattachment 补官方 `dsh-client-ui-attachment` 缺的那半：**输入框上方的附件 rail
// 与系统拖入入口**（官方 `files: OFF:src/client/ComposerAttachments.tsx` 那一个部件）。
//
// # 为什么必须补
//
// 本仓 `conversation.input.attachments` 这个 rail 槽**有座位、有派发，却没有 `Panels()`
// 注册者**（座位 `internal/uikernel/official_slots.go:137`/`:342`，派发
// `plugins/dscomposer/composer.go:283`，骨架 `internal/uikernel/shell_skeleton.go:549,893,927`）。
// 官方那条「空 rail 不占位」的规则在本仓就表现成**永久看不见附件能力**——不是官方没有，
// 是没有任何人往那个座位上画东西。这个包就是那个注册者。
//
// 归属：官方「接收拖入」属于本包（`OFF:src/client/drop-events.ts`），**不是**
// `dsh-client-ui-file-drag`（那个是"把文件从应用拖出去"，Viva 没有 OS 拖源，已判 HIDE）。
//
// # 它照抄的是哪一份官方源（几何/文案的唯一权威）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-attachment
//
//	OFF:src/client/ComposerAttachments.tsx          168 行，rail 的挂载条件与三态
//	OFF:src/client/ComposerAttachments.module.css   rail 的外框/缩略图/进度条
//	OFF:src/FileCard.tsx                           84 行，一张文件卡的全部结构
//	OFF:src/FileCard.module.css                    它的全部几何值
//	OFF:src/AttachmentRail.tsx                     横向 rail 与两端翻页箭头
//	OFF:src/AttachmentRail.module.css              箭头与 item 的几何
//	OFF:src/client/drop-events.ts                  拖入的 enter/leave/drop 状态机
//	OFF:src/DropOverlay.tsx + .module.css          拖拽悬停层（本包**没有**画，见下）
//
// 文案不在这个包里：官方 rail 的全部字符串来自 conversation 命名空间
// `OFF:dsh-client-ui-conversation/src/client/locales.ts` 的 zh 段（`:34-69`），
// 本文件逐个标了行号。**不许新写中文**。
//
// # 它要壳注入什么（**接线照这三条来**）
//
//  1. `SetSink(AttachmentSink)` —— 数据面。nil = 没有上传能力：rail 与拖入都**整块不画**，
//     `HandleFileDrop` 也不消费事件（画一个拖不进来的东西比不画更坏）。
//     sink 由壳用 `*apiclient.Client` 包一层实现（`AttachmentPut` / `SessionAttach`），
//     **必须在 `Load` 之后注入**——那之前 apiclient 还没拿到宿主地址。与
//     `plugins/dsdirpicker` 的 `SetSource`、`plugins/dsagentteam` 的 `AttachShellTier`
//     同一个时序与理由（插件拿不到 `*apiclient.Client`，界面线 R2/R7）。
//
//  2. `app.Config.OnFileDrop` 转发给本包的 `HandleFileDrop`：
//
//     cfg.OnFileDrop = func(ev gfx.FileDropEvent) gfx.EventResult {
//     return dsattachment.HandleFileDrop(ev)
//     }
//
//     返回值里 `Handled` = 这次拖入被接受了（viva 拿它给系统看"这里收文件"），
//     `Repaint` = 请重画一帧（rail 上新出现一张卡）。**空 `Paths` 一律不消费**。
//
//  3. `AttachReady(ctx)` —— 发消息**之前**调一次：把 rail 里已就绪的附件按会话挂上去
//     （宿主 `session.attachment`）。官方是在提交那一步做的（`OFF:service.ts:216-294`
//     的 `submitSession`：add 只做上传与 staging，提交时才把引用交给会话），
//     本仓没有"提交"这一层，所以把这一步交给壳，位置就在 `session.prompt` 之前。
//
// # 宿主三态与失败原文
//
// 上传走 `attachment.put`（`internal/apiclient/attachments.go`）。宿主拒绝时
// （非绝对路径 / 目录 / 符号链接 / **0 字节** / 超过 8 MiB）回的是一句中文明文
// （`HOST:cmd/dshd/attachments.go:129-134`），本包**原样显示**，不改写、不翻译、不编。
//
// # 官方三态（`FileCardState`，`OFF:FileCard.tsx:18-19`）
//
//	uploading → meta 行 = `file.uploading`「上传中…」           + 28px 图标槽换成 spinner + 2px 进度条
//	ready     → meta 行 = `[扩展名大写, fileSizeText(bytes)]`   + 图标槽（本包用扩展名文字，见下）
//	error     → meta 行 = 宿主错误原文                          + 边框转 error 色
//
// # 没搬的官方细节（照实登记，不假装搬了）
//
//   - **图片缩略图 / ImageLightbox**：官方 `ComposerAttachments.tsx:87-154` 的图片分支要
//     `attachment.previewUrl`（浏览器 `File` 给的对象 URL）与 `<img>`。本仓有字节
//     （`AttachmentGet`）但没有图片解码/位图绘制链路，**按任务约束不自行画图**，
//     所以图片也走文字卡，`image.*` 那组文案一律不用。
//   - **拖拽悬停层（DropOverlay）**：官方是整窗 `position:fixed` + `backdrop-filter: blur(10px)`
//   - `--dsw-alias-bg-mask-drop` 底色 + 115×84 SVG 插画（`OFF:DropOverlay.module.css:3-14`、
//     `OFF:DropOverlay.tsx:43-77`）。`pluginapi.Theme` 没有这个底色令牌，viva 也没有 blur，
//     手画 SVG 违反仓规，插画没有 Go 素材 —— **四样都缺，所以一层都不画**
//     （与 `plugins/dsdirpicker` 对 Modal 遮罩的处理同一条先例：没有官方 CSS 值就不自造压色）。
//     另外 viva 的 `OnFileDrop` 文档写明「On Windows only the drop is reported」
//     （`app/app.go:150-156`），而本仓只做 Windows 稳定版 —— 悬停层在这个平台上本来就到不了。
//     `dropTitle`/`dropBlocked` 因此**没有可见落点**（读到了，但不画）。
//   - **重试**：官方失败卡整张变成重试按钮（`OFF:FileCard.tsx:50-56` + `onRetry`）。
//     本包只保留右侧 ×（删除后重新拖入即可），不做重试入口。
//   - **多选文件 / 「选文件」按钮**：`app.OpenFiles` 回的 `FileRef` **只有
//     Name/Size/MIME/IsDir/Read，没有绝对路径**，而 `attachment.put` 要的是 `path`
//     ——这条路喂不了宿主，所以本包不提供按钮（官方桌面端也没有它自己的"选文件"，
//     入口是拖入 + 系统对话框）。
//   - **`role="group"` 的无障碍名**（`OFF:AttachmentRail.tsx:151-153` 的
//     `aria-label={labels.group}` = `attachment.pending`「待发送附件」）：viva 的
//     Box 没有 group 语义，这个字符串**只在 aria 里出现**，所以不画成可见文字。
package dsattachment

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 文案：逐字来自官方 conversation zh 字典，行号是 OFF:locales.ts ──────────────
//
// 界面上一旦出现这几条以外的字样，就不是官方那个 rail 了。
const (
	// attachment.dropBlocked（:39）；accepting=false 时官方只画这一句标题。
	zhDropBlocked = "当前无法添加文件或图片"
	// attachment.dropTitle（:37）。
	zhDropTitle = "文件或图片拖动到此处即可添加"
	// file.uploading（:63）：三态里的 uploading 那一行。
	zhFileUploading = "上传中…"
	// file.uploadFailed（:64）：**没有宿主原文时**的回退文案（官方文件卡只有这一句）。
	zhFileUploadFailed = "上传失败，点击重试"
	// file.remove（:62）：× 按钮的无障碍名，{name} 由文件名替换。
	zhFileRemove = "移除文件 {name}"
	// file.label（:69）：官方 `attachment.file.name || t('file.label')`（ComposerAttachments.tsx:73）。
	zhFileLabel = "文件"
	// attachment.scrollLeft（:35）/ attachment.scrollRight（:36）：翻页箭头的无障碍名。
	zhScrollLeft  = "向左滚动附件"
	zhScrollRight = "向右滚动附件"
)

// `attachment.dropDesc`（:38「图片限制：最多 {count} 张，每张 {size}」）**没有搬**：
// 官方只在拿到 `dropLimits` 时才渲染它（`OFF:client/labels.ts:45-49` 的
// `limits === undefined ? undefined : t('attachment.dropDesc', limits)`），
// 而宿主没有任何"条数/单张上限"事实（限额写在 `HOST:cmd/dshd/attachments.go:30` 的
// 8 MiB 总量上，是另一个语义），编一个数字出来就是假话。

// ── 几何：逐值来自官方 CSS，行号逐个标了 ────────────────────────────────────

// rail 外框：`OFF:ComposerAttachments.module.css:1-7`
//
//	.rail { min-width:0; padding:2px 10px 0; margin-bottom:-6px }
const (
	railPadTop     float32 = 2  // padding-top
	railPadX       float32 = 10 // padding-left/right
	railPullBottom float32 = -6 // margin-bottom（卡片 flex gap 是 12，缩略图贴得更近，回拉 6）
)

// 横向 rail 与一张 item：`OFF:AttachmentRail.module.css:6-9`（gap:10px）、`:25-28`（height:64px）
const (
	itemGap    float32 = 10
	itemHeight float32 = 64
)

// 一张文件卡：`OFF:FileCard.module.css:1-14`
//
//	.card { display:inline-flex; align-items:center; gap:10px; width:240px; height:64px;
//	        padding:0 12px; border:0.5px solid; border-radius:var(--dsw-radius-lg) }
const (
	cardWidth  float32 = 240
	cardHeight float32 = 64
	cardPadX   float32 = 12
	cardGap    float32 = 10
	cardBorder float32 = 0.5
	// 圆角用主题的 Radii.LG（官方 `var(--dsw-radius-lg)`），不写字面量。
)

// 图标槽与 spinner：`OFF:FileCard.module.css:20-27`（28×28）、`:29-37`（20×20 + 2px 边）
const (
	iconSize    float32 = 28
	spinnerSize float32 = 20
	spinnerRing float32 = 2
	// 图标槽里放扩展名文字时的字号：官方这里是 SVG sprite（FileTypeIcon），
	// 本仓没有素材，用文字符号顶替（仓规：不许手画 SVG）。28px 槽只放得下 4 个字符。
	glyphFontSize   float32 = 10
	glyphMaxLetters         = 4
)

// 卡内两行文字：`OFF:FileCard.module.css:43-49`（.body padding:8px 0）、
// `:61-69`（.name 14px/22px/500）、`:71-78`（.meta 12px/15px）
const (
	bodyPadY     float32 = 8
	nameFontSize float32 = 14
	nameLineH    float32 = 22
	nameWeight   float32 = 500
	metaFontSize float32 = 12
	metaLineH    float32 = 15
)

// 删除按钮：`OFF:FileCard.module.css:84-102`
//
//	.remove { position:absolute; top:6px; right:6px; width:18px; height:18px; border-radius:50% }
const (
	removeTop   float32 = 6
	removeRight float32 = 6
	removeSize  float32 = 18
	// 官方 `IconCloseFillRegular size={12}`（FileCard.tsx:69）。图标没有 Go 素材 → 文字「×」。
	removeGlyphSize float32 = 12
)

// 进度条：`OFF:FileCard.module.css:142-151`（track 左右各 12、底 5、高 2、圆角 1）、
// `:153-160`（bar 默认宽度 35%，有进度时按比例）
const (
	progressInsetX       float32 = 12
	progressBottom       float32 = 5
	progressHeight       float32 = 2
	progressRadius       float32 = 1
	progressWidthPercent float32 = 35
)

// 翻页箭头：`OFF:AttachmentRail.module.css:90-108`（24×24、圆角 999、panel 阴影）、
// `:116-123`（left:4px / right:4px）
const (
	arrowSize  float32 = 24
	arrowInset float32 = 4
	// 官方 `IconChevronLeft/RightOutlineRegular`；图标没有素材 → 文字「‹」「›」。
	arrowGlyphSize float32 = 14
)

// ── 数据面注入（照 plugins/dsdirpicker 的 SetSource 写法，不发明新机制）──────

// AttachmentSink 是壳注入的**上传能力**；nil = 没有上传能力，rail 与拖入都不画。
//
// 四条方法一一对应宿主四条真通道（证据在 `internal/apiclient/attachments.go` 与
// `prompt_parts.go` 的文件头）：
//
//	Put   → `attachment.put`（要绝对路径；0 字节/超限回 400 原文）
//	Attach→ `session.attachment`（只承载图片；旧路径保留，见 AttachReady 的注释）
//	Upload→ `attachment.get` 取回已暂存字节 + `fileUploads.upload` 换官方回执
//	        （`session.prompt` 的 `{type:'file', receiptId}` 用它；这是官方主路径）
//	CurrentSession 是壳那一侧的"当前会话 id"；空 = 还没有会话 → 不能 Attach/Upload。
//
// 为什么是本地接口而不是 `pluginapi` 的公共契约：公共契约要动 `pluginapi/plugin.go`，
// 那是另一条线的事；插件侧只需要"壳能塞进来一个实现"。等公共契约落定，
// 这个接口直接删掉、换成 `pluginapi` 的类型即可（与 `dsdirpicker.DirSource` 同一条理由）。
type AttachmentSink interface {
	Put(ctx context.Context, path string) (*apiclient.Attachment, error)
	Attach(ctx context.Context, sessionID string, a apiclient.Attachment) error
	Upload(ctx context.Context, sessionID string, a apiclient.Attachment) (*apiclient.FileUploadReceipt, error)
	CurrentSession() string
}

var (
	injectMu sync.RWMutex
	// injected 是壳塞进来的上传能力（nil = 没有）。
	injected AttachmentSink
	// instance 是注册起来的那个插件本体：`HandleFileDrop`/`AttachReady` 是包级函数
	// （壳要从 `app.Config.OnFileDrop` 转发），它们需要同一份 rail 状态。
	instance *Plugin
)

// SetSink 由壳注入上传能力（**必须在 Load 之后**）。
//
// 传 nil = 收回能力（rail 从此整块不画、拖入也不消费）——老壳 / 没有 apiclient 时就是这样。
func SetSink(s AttachmentSink) {
	injectMu.Lock()
	injected = s
	injectMu.Unlock()
}

// currentSink 取当前注入的上传能力（每帧都重新问，壳中途注入/收回都立刻生效）。
func currentSink() AttachmentSink {
	injectMu.RLock()
	defer injectMu.RUnlock()
	return injected
}

// bindInstance 由 Register 调用：让包级的 `HandleFileDrop` / `AttachReady` 找到本体。
func bindInstance(p *Plugin) {
	injectMu.Lock()
	instance = p
	injectMu.Unlock()
}

// currentInstance 取插件本体（还没 Register 过就是 nil）。
func currentInstance() *Plugin {
	injectMu.RLock()
	defer injectMu.RUnlock()
	return instance
}

// ── 三态（官方 `FileCardState`，OFF:FileCard.tsx:18-19）──────────────────────

// state 是一张卡的显示态，取值与官方三态逐一对应。
type state uint8

const (
	// stateUploading ← 'uploading'：上传在飞（官方 `upload === undefined || status === 'uploading'`）。
	stateUploading state = iota
	// stateReady ← 'ready'：宿主已经收下，meta 行换成扩展名 + 字节数。
	stateReady
	// stateError ← 'error'：失败，meta 行换宿主原文，边框转 error 色。
	stateError
)

// key 返回官方那三态的字符串（日志/测试用，与 `FileCardState` 同字面量）。
func (s state) key() string {
	switch s {
	case stateReady:
		return "ready"
	case stateError:
		return "error"
	default:
		return "uploading"
	}
}

// item 是 rail 上的一张卡。
//
// Key 是**本地的稳定身份**（= hit id 的后缀），从加入那一刻起不变：
// 上传中它代表一次待上传的路径，ready 之后它仍然是同一张卡的 id。
// 这一点与官方不同（官方用浏览器侧的 `attachment.id`），但界面行为一致，
// 而且它让"同一路径拖两次"仍然得到两张卡（与官方一致：`id` 是每次 add 新生成的）。
type item struct {
	Key      string
	Name     string
	Path     string // 本地绝对路径（拖入时系统给的；`Put` 的唯一入参）
	Bytes    int64
	State    state
	Progress float64 // 0..1；负数 = 不知道进度（官方 upload.total === undefined）
	ErrText  string  // 宿主错误**原文**（空 = 宿主没给文字）
	// Att contains 宿主回来的那份元数据（ready 之后才有；`AttachReady` 用它）。
	Att apiclient.Attachment
}

// ── 插件本体 ────────────────────────────────────────────────────────────────

// railBoxes 是**这一帧**画出来的几何（viva 的 Box 只在本次 paint 内有效，
// 测试必须在 draw 之后立刻抄；跨帧读会 panic）。
type railBoxes struct {
	Rail   ui.Box
	Cards  []ui.Box
	Remove []ui.Box
	Arrows []ui.Box
}

// Plugin 是附件 rail 本体。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	// run 执行一次宿主调用。
	//
	// 默认 nil = `go fn()`：`attachment.put` 背后是 HTTP，在拖入回调里同步等一次
	// 磁盘读 + 哈希会把窗口卡住。测试把它换成同步执行，拖入 → 状态落定在同一线程，
	// 断言才是确定的（与 `plugins/dsdirpicker` 的 `Plugin.run` 同一个思路）。
	run func(fn func())

	mu    sync.Mutex
	act   pluginapi.Actions
	items []item
	// nextKey 是本地身份计数器（单调递增，永不复用）。
	nextKey uint64
	// dragActive 是拖拽悬停态（官方 `dragActive`，drop-events.ts:50-70）。
	dragActive bool
	// visible 是 AfterDraw 量出来的"一页放得下几张卡"（0 = 还没量过 → 全放）。
	visible int
	// page 是当前页（官方靠横向滚动，viva 没有横向滚动容器 → 等价的翻页窗口）。
	page int

	// boxes 是这一帧的几何，draw 按它的尺寸决定翻页窗口。
	boxes railBoxes
}

// ID 实现 pluginapi.Plugin（官方包名 dsh-client-ui-attachment 的 kebab 化）。
func (*Plugin) ID() string { return "ds-attachment" }

// Provides 实现 pluginapi.Plugin：附件 rail 不对外提供服务键。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin：没有令牌画不了颜色/字号。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin：只做注入，不画东西。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	theme, _ := ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	prims, _ := ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	p.theme, p.prims = theme, prims
	// 包级 `HandleFileDrop` / `AttachReady` 与壳绘制的是同一个本体。
	bindInstance(p)
	return nil
}

// Panels 实现 pluginapi.Panels：rail 挂官方 `conversation.input.attachments` 座位。
//
// Order 0：这个槽是 single 且目前只有本插件注册（官方也是"一个 rail 占满"）。
// 槽位名来自 `internal/uikernel/official_slots.go:137`，不自己拼字符串。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:        "attachments",
		Slot:      uikernel.OffInputAttachments,
		Order:     0,
		Draw:      p.draw,
		AfterDraw: p.afterDraw,
	}}
}

// ── 拖入（官方 drop-events.ts 的状态机，只认 Paths）────────────────────────

// HandleFileDrop 由壳在 `app.Config.OnFileDrop` 里转发；返回是否消费了事件。
//
// 壳接线（**接线照这个来**）：
//
//	cfg.OnFileDrop = func(ev gfx.FileDropEvent) gfx.EventResult { return dsattachment.HandleFileDrop(ev) }
//
// 行为（逐条对应官方 `OFF:client/drop-events.ts`）：
//
//   - 没有 sink 或没有本体 → **不消费**（`:71-81` 的 `if (canAcceptDrop)` 反面：
//     不接受拖入时官方连 preventDefault 都不做，浏览器会自己打开这个文件）。
//   - `over` → 接受这次拖拽（官方 `:50-62` 的 preventDefault + dropEffect='copy'）。
//   - `leave` → 清悬停态（官方 `:63-70`）。
//   - `drop` → 先清悬停态，再对 `ev.Paths` 里**每个**绝对路径调一次 `Put`
//     （官方 `:71-80`：`reset()` 之后才 `onAddFiles(files)`）。
//   - **空 `Paths` 什么都不做**（官方 `files.length > 0` 那道门）：`ev.Files` 没有路径
//     （viva `gfx/input.go:249-258`：FileDropFile 只有 Name/Size/MIME/IsDir/Read），
//     而 `attachment.put` 要的是宿主可读的绝对路径 —— 拿 Files 喂 Put 就是编一条路径。
func HandleFileDrop(ev gfx.FileDropEvent) gfx.EventResult {
	p := currentInstance()
	sink := currentSink()
	if p == nil || sink == nil {
		return gfx.EventResult{}
	}
	switch ev.Phase {
	case gfx.FileDropOver:
		p.setDragActive(true)
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.FileDropLeave:
		p.setDragActive(false)
		return gfx.EventResult{Repaint: true}
	case gfx.FileDropDrop:
		p.setDragActive(false)
		paths := absolutePaths(ev.Paths)
		if len(paths) == 0 {
			return gfx.EventResult{Repaint: true}
		}
		p.addPaths(sink, paths)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// absolutePaths 只剔掉空串/纯空白项。
//
// **不在这里判"是不是绝对路径、是不是目录"**：那也是宿主的判据
// （`HOST:cmd/dshd/attachments.go:97-122`），它的中文原文要原样显示给用户；
// 提前过滤会让用户看到"拖进去什么都没发生"，比看到一句真话差得多。
func absolutePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		out = append(out, path)
	}
	return out
}

// setDragActive 落下悬停态（只影响内部状态；悬停层没有画，见包注释）。
func (p *Plugin) setDragActive(active bool) {
	p.mu.Lock()
	p.dragActive = active
	p.mu.Unlock()
}

// addPaths 按官方顺序把路径加进 rail，然后每个路径各发一次上传。
func (p *Plugin) addPaths(sink AttachmentSink, paths []string) {
	type job struct{ key, path string }
	jobs := make([]job, 0, len(paths))
	p.mu.Lock()
	for _, path := range paths {
		p.nextKey++
		it := item{
			Key:      "att-" + strconv.FormatUint(p.nextKey, 10),
			Name:     baseName(path),
			Path:     path,
			State:    stateUploading,
			Progress: -1, // 官方 upload.total 未知 → 进度条走不定态宽度
		}
		p.items = append(p.items, it)
		jobs = append(jobs, job{key: it.Key, path: path})
	}
	p.mu.Unlock()
	p.invalidate()
	for _, j := range jobs {
		p.upload(sink, j.key, j.path)
	}
}

// upload 传一个路径（**一个路径一次 Put**，官方每个 File 各一次）。
func (p *Plugin) upload(sink AttachmentSink, key, path string) {
	p.dispatch(func() {
		att, err := sink.Put(context.Background(), path)
		p.mu.Lock()
		if it := p.itemLocked(key); it != nil {
			switch {
			case err != nil:
				it.State = stateError
				it.Progress = -1
				it.ErrText = failureText(err)
			case att == nil:
				// sink 回了 (nil, nil)：没有元数据就没有可发的附件，按失败处理。
				// **不编中文**：ErrText 留空，meta 行回退官方通用文案。
				it.State = stateError
				it.Progress = -1
				it.ErrText = ""
			default:
				it.State = stateReady
				it.Progress = 1
				it.Att = *att
				if att.Name != "" {
					it.Name = att.Name
				}
				if att.ByteSize > 0 {
					it.Bytes = att.ByteSize
				}
			}
		}
		p.mu.Unlock()
		p.invalidate()
	})
}

// remove 把一张卡从 rail 移除（官方 `onRemoveAttachment(id)`）。
//
// 纯本地动作：官方那边 `attachment.id` 也从草稿表里删，宿主侧已上传的对象不回收
// （官方没有"删除附件对象"的通道，本仓也没有）——所以这里不假装通知宿主。
func (p *Plugin) remove(key string) {
	p.mu.Lock()
	kept := p.items[:0]
	for _, it := range p.items {
		if it.Key != key {
			kept = append(kept, it)
		}
	}
	p.items = kept
	p.clampPageLocked()
	p.mu.Unlock()
	p.invalidate()
}

// AttachReady 把 rail 里**已就绪**的附件交给当前会话（宿主 `session.attachment`）。
//
// 壳在 `session.prompt` 之前调一次（官方在提交那一步做同一件事，
// `OFF:service.ts:216-294`）。没有 sink / 没有当前会话 / 没有就绪附件时返回 nil
// ——"无事可做"不是错误。
//
// 失败返回第一个错误（宿主原文在 `*apiclient.APIError.Message` 里）：
// 调用方决定是拦住发送还是只提示。**任一附件失败就停**：继续挂后面的会让
// "这次发送带了几张图"变得不可知。
func AttachReady(ctx context.Context) error {
	p := currentInstance()
	sink := currentSink()
	if p == nil || sink == nil {
		return nil
	}
	session := strings.TrimSpace(sink.CurrentSession())
	if session == "" {
		return nil
	}
	for _, it := range p.readyItems() {
		if err := sink.Attach(ctx, session, it.Att); err != nil {
			return err
		}
	}
	return nil
}

// SendBatch 是一次发送要引用的已就绪附件。
//
// 官方 `conversation.sendSession` 在提交时把每张卡序列化成 `{type:'file', receiptId}`
// 放进 `session.prompt.content`（`OFF:service.ts:246-303`）。本仓没有"提交"层，
// 所以壳在 `session.prompt` 之前调 `PreparePrompt` 拿到这一批，成功后 `Commit` 清掉
// 已发送的卡；失败则什么都不丢（用户重试时同一批还在 rail 上）。
//
// keys 是**快照时刻**的本地身份：发送期间用户新拖进来的卡不在批次里，不会被误删。
type SendBatch struct {
	parts []apiclient.PromptPart
	keys  []string
}

// Parts 返回按 rail 顺序排好的附件 part（附件在前、文本由壳追加在最后）。
func (b *SendBatch) Parts() []apiclient.PromptPart {
	if b == nil {
		return nil
	}
	return append([]apiclient.PromptPart(nil), b.parts...)
}

// Empty 报告这一批有没有附件（nil 也算空）。
func (b *SendBatch) Empty() bool { return b == nil || len(b.parts) == 0 }

// Commit 把这一批已经从 rail 发出去的卡删掉（只删快照里那几张）。
func (b *SendBatch) Commit() {
	if b == nil || len(b.keys) == 0 {
		return
	}
	p := currentInstance()
	if p == nil {
		return
	}
	p.mu.Lock()
	keep := p.items[:0]
	for _, it := range p.items {
		remove := false
		for _, key := range b.keys {
			if it.Key == key {
				remove = true
				break
			}
		}
		if !remove {
			keep = append(keep, it)
		}
	}
	p.items = keep
	p.clampPageLocked()
	p.mu.Unlock()
	p.invalidate()
}

// HasItems 报告 rail 上有没有任何条目（含上传中）。
//
// 壳用它决定"空文本能不能发"：官方 `sendSession` 允许附件-only 消息
// （`content = uploaded + (text === ” ? [] : text)`，OFF:service.ts:302），
// 所以 composer 的"空文本不提交"只在没有附件时才成立。
func HasItems() bool {
	p := currentInstance()
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.items) > 0
}

// PreparePrompt 把 rail 里**全部已就绪**的附件换成官方上传回执。
//
// 与 AttachReady 的分工：AttachReady 走旧的 `session.attachment`（只登记、不绑定消息），
// 官方 rc.2 的 `session.prompt` 现在认 `{type:'file', receiptId}`，所以产品发送路径改走
// 这一条；AttachReady 保留是给没有 PromptContentPart 的老宿主/旧测试用的兼容桥。
//
// 三种"不做"都是正常返回 nil：
//   - 没有 sink / 没有插件本体（无上传能力）；
//   - 没有当前会话（壳还没选会话）；
//   - rail 里没有已就绪附件。
//
// 任一上传失败就整批失败并返回错误（宿主原文在 `*apiclient.APIError.Message`）：
// 继续上传后面的会让"这次到底带了几份附件"变得不可知，与官方"提交前必须全部 ready"
// 同一条判据。**有卡仍在上传中**也失败（官方 `uploadFor` 在非 ready 时直接 throw）。
func PreparePrompt(ctx context.Context) (*SendBatch, error) {
	p := currentInstance()
	sink := currentSink()
	if p == nil || sink == nil {
		return nil, nil
	}
	session := strings.TrimSpace(sink.CurrentSession())
	if session == "" {
		return nil, nil
	}
	p.mu.Lock()
	items := append([]item(nil), p.items...)
	p.mu.Unlock()
	if len(items) == 0 {
		return nil, nil
	}
	for _, it := range items {
		if it.State == stateReady {
			continue
		}
		if it.State == stateError && it.ErrText != "" {
			return nil, errors.New(it.ErrText)
		}
		return nil, errors.New(zhFileUploading)
	}
	batch := &SendBatch{}
	for _, it := range items {
		if it.State != stateReady || it.Att.ID == "" {
			continue
		}
		receipt, err := sink.Upload(ctx, session, it.Att)
		if err != nil {
			return nil, err
		}
		if receipt == nil || receipt.ReceiptID == "" {
			return nil, errors.New("fileUploads.upload 没有返回回执")
		}
		batch.parts = append(batch.parts, apiclient.PromptPart{
			Type:      "file",
			ReceiptID: receipt.ReceiptID,
		})
		batch.keys = append(batch.keys, it.Key)
	}
	if len(batch.parts) == 0 {
		return nil, nil
	}
	return batch, nil
}

// readyItems 抄一份已就绪的附件（按 rail 顺序）。
func (p *Plugin) readyItems() []item {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]item, 0, len(p.items))
	for _, it := range p.items {
		if it.State == stateReady && it.Att.ID != "" {
			out = append(out, it)
		}
	}
	return out
}

// itemLocked 按 key 找卡（在锁内调用）。
func (p *Plugin) itemLocked(key string) *item {
	for i := range p.items {
		if p.items[i].Key == key {
			return &p.items[i]
		}
	}
	return nil
}

// dispatch 执行一次宿主调用（默认丢给 goroutine，见 Plugin.run 的注释）。
func (p *Plugin) dispatch(fn func()) {
	if p.run != nil {
		p.run(fn)
		return
	}
	go fn()
}

// invalidate 请壳重绘一帧（异步上传落地之后必须调，否则画面停在"上传中…"）。
func (p *Plugin) invalidate() {
	p.mu.Lock()
	fn := p.act.Invalidate
	p.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// ── 绘制 ────────────────────────────────────────────────────────────────────

// draw 画 rail。**没有 sink 或 0 条附件就一个节点都不画**。
//
// 官方判据只有一句：`{railItems.length > 0 && <div className={css.rail}>}`
// （`OFF:client/ComposerAttachments.tsx:62`）——这就是"空 rail 不占位"。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	t := p.theme
	if t == nil {
		return
	}
	p.mu.Lock()
	p.act = env.Act
	items := append([]item(nil), p.items...)
	visible, page := p.visible, p.page
	p.mu.Unlock()

	// 上一帧的几何作废：这一帧一个节点都不画时 boxes 必须是空的，
	// 测试据此断言"零节点"（同 dsdirpicker 的做法）。
	p.boxes = railBoxes{}
	if currentSink() == nil || len(items) == 0 {
		return
	}

	per := visible
	if per <= 0 || per > len(items) {
		per = len(items)
	}
	pages := (len(items) + per - 1) / per
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	p.mu.Lock()
	p.page = page
	p.mu.Unlock()
	start := page * per
	end := start + per
	if end > len(items) {
		end = len(items)
	}

	// `.rail`（ComposerAttachments.module.css:1-7）：min-width:0; padding:2px 10px 0; margin-bottom:-6px
	rail := ui.Row(f).FillWidth().Items(ui.Center).
		PadSides(railPadTop, railPadX, 0, railPadX).
		Gap(itemGap).
		MarginBottom(railPullBottom)
	p.boxes.Rail = rail
	rail.Children(func() {
		for _, it := range items[start:end] {
			p.drawCard(f, env, t, it)
		}
		// 官方靠横向滚动 + 两端箭头翻页（OFF:AttachmentRail.tsx:138-167）；
		// viva 没有横向滚动容器，所以这里用**等价的翻页窗口**：只在放不下时出现箭头，
		// 保证任何一张卡都点得到（不能出现"看不见也删不掉"的卡）。
		if pages > 1 {
			p.drawArrow(f, env, t, -1, page > 0)
			p.drawArrow(f, env, t, 1, page < pages-1)
		}
	})
}

// drawArrow 画一枚翻页箭头（OFF:AttachmentRail.module.css:90-123 的几何）。
//
// `enabled=false` 时**不画**：官方那枚箭头只在 `edges.left/right` 为真时才渲染
// （OFF:AttachmentRail.tsx:138,159），画一个永远点不动的箭头不是官方形状。
func (p *Plugin) drawArrow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, direction int8, enabled bool) {
	if !enabled {
		return
	}
	glyph, label, id, at := "‹", zhScrollLeft, "attachments.scroll-left", arrowInset
	if direction > 0 {
		glyph, label, id = "›", zhScrollRight, "attachments.scroll-right"
	}
	arrow := ui.Row(f).Absolute().Top((itemHeight-arrowSize)/2).Size(arrowSize, arrowSize).
		Radius(arrowSize/2).Background(t.InputMajor).Shadow(gfx.ShadowMD).
		Items(ui.Center).Justify(ui.Center).
		Hit(p.hitID(env, id), label)
	if direction > 0 {
		arrow = arrow.Right(at)
	} else {
		arrow = arrow.Left(at)
	}
	p.boxes.Arrows = append(p.boxes.Arrows, arrow)
	arrow.Children(func() {
		ui.Text(f, glyph).FontSize(arrowGlyphSize).Color(t.LabelSecondary)
	})
	if arrow.Clicked() {
		p.stepPage(direction)
	}
}

// stepPage 翻一页（-1 上一页 / +1 下一页），落到 [0, 页数-1]。
func (p *Plugin) stepPage(direction int8) {
	delta := 1
	if direction < 0 {
		delta = -1
	}
	p.mu.Lock()
	pages := pagesLocked(len(p.items), p.visible)
	next := p.page + delta
	if next < 0 {
		next = 0
	}
	if next > pages-1 {
		next = pages - 1
	}
	if next < 0 {
		next = 0
	}
	p.page = next
	p.mu.Unlock()
	p.invalidate()
}

// pagesLocked 算总页数（per<=0 或放得下全部 = 1 页；没有附件 = 0 页）。
//
// 不自己加锁：调用方已经持有 p.mu（或只是拿纯数值算），这样避免重复加锁。
func pagesLocked(total, per int) int {
	if total <= 0 {
		return 0
	}
	if per <= 0 || per > total {
		per = total
	}
	return (total + per - 1) / per
}

// drawCard 画一张文件卡（官方 `OFF:FileCard.tsx:40-79` 的结构）。
func (p *Plugin) drawCard(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, it item) {
	// `.card`：240×64、0.5px 描边（失败转 error 色）、--dsw-radius-lg、--dsw-specific-input-major 底
	border := t.BorderL2
	if it.State == stateError {
		border = t.Error // .failed { border-color: var(--dsw-alias-state-error-primary) }（:16-18）
	}
	card := ui.Row(f).Width(cardWidth).Height(cardHeight).Items(ui.Center).
		Gap(cardGap).PadSides(0, cardPadX, 0, cardPadX).
		Radius(t.Radii.LG).Border(cardBorder, border).Background(t.InputMajor)
	p.boxes.Cards = append(p.boxes.Cards, card)

	card.Children(func() {
		p.drawIcon(f, t, it)
		// `.body`：flex:1 + column + padding 8px 0
		body := ui.Column(f).Grow(1).MinWidth(0).PadY(bodyPadY)
		body.Children(func() {
			ui.Text(f, displayName(it)).FontSize(nameFontSize).LineHeight(nameLineH).
				Weight(nameWeight).Color(t.LabelPrimary).Ellipsis()
			ui.Text(f, metaText(it)).FontSize(metaFontSize).LineHeight(metaLineH).
				Color(metaColor(t, it.State)).Ellipsis()
		})
		p.drawRemove(f, env, t, it)
		if it.State == stateUploading {
			p.drawProgress(f, t, it)
		}
	})
}

// drawIcon 画 28×28 图标槽。
//
// 官方：uploading 时是 `.spinner`（20×20、2px 边框、旋转），其余是 `FileTypeIcon`
// （`OFF:FileCard.tsx:45-49`）。本仓没有图标素材、也没有渲染循环（转不起来），
// 所以：上传中画静态的 20×20 圈，就绪/失败画扩展名的文字符号（仓规：不许手画 SVG）。
func (p *Plugin) drawIcon(f ui.C, t *pluginapi.Theme, it item) {
	icon := ui.Row(f).Size(iconSize, iconSize).Items(ui.Center).Justify(ui.Center)
	icon.Children(func() {
		if it.State == stateUploading {
			ui.Row(f).Size(spinnerSize, spinnerSize).Radius(spinnerSize/2).
				Border(spinnerRing, t.LabelTertiary)
			return
		}
		glyph := glyphOf(it.Name)
		if glyph == "" {
			return
		}
		ui.Text(f, glyph).FontSize(glyphFontSize).Color(t.LabelTertiary)
	})
}

// drawRemove 画右侧 ×（官方 `.remove`：absolute、top 6、right 6、18×18、圆形）。
//
// 官方默认 `opacity:0`、hover 才出现（`:99-112`）。viva 没有 per-box 的 hover 状态，
// 而"看不见的按钮"等于没有按钮 —— 所以这里始终可见（只改可见性，不改几何）。
func (p *Plugin) drawRemove(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, it item) {
	remove := ui.Row(f).Absolute().Top(removeTop).Right(removeRight).
		Size(removeSize, removeSize).Radius(removeSize/2).
		Background(t.ButtonPrimaryFill).Items(ui.Center).Justify(ui.Center).
		Hit(p.hitID(env, "attachments.remove."+it.Key), removeLabel(it.Name))
	p.boxes.Remove = append(p.boxes.Remove, remove)
	remove.Children(func() {
		ui.Text(f, "×").FontSize(removeGlyphSize).Color(t.ButtonPrimaryFg)
	})
	if remove.Clicked() {
		p.remove(it.Key)
	}
}

// drawProgress 画 2px 进度条（官方 `.progressTrack` / `.progressBar`）。
//
// 官方只有 `status === 'uploading'` 才画它（`OFF:FileCard.tsx:71-78`）。
// `attachment.put` 是一次请求、宿主不回进度，所以进度通常未知 → 用官方
// `.progressBar` 的默认宽度 35%（不清零、也不假装 100%）。
func (p *Plugin) drawProgress(f ui.C, t *pluginapi.Theme, it item) {
	// --dsw-specific-file-upload-track 在 pluginapi.Theme 里没有对应字段，
	// 取最轻的一档描边色（BorderL2）——**不新造颜色**。
	track := ui.Row(f).Absolute().Left(progressInsetX).Right(progressInsetX).
		Bottom(progressBottom).Height(progressHeight).Radius(progressRadius).
		Background(t.BorderL2)
	track.Children(func() {
		width := progressWidthPercent
		if it.Progress >= 0 {
			width = float32(clamp01(it.Progress) * 100)
		}
		ui.Row(f).WidthPercent(width).FillHeight().Radius(progressRadius).
			Background(t.BrandPrimary) // --dsw-alias-brand-primary
	})
}

// afterDraw 量这一帧的 rail 宽度，算出"一页放得下几张"给下一帧用。
//
// 为什么要 AfterDraw：`f.Bounds()` 在槽位里给的是**整窗**宽（`pluginapi.Env` 的
// Geometry 注释实测 1180），rail 的真实宽度只有布局之后才知道；而 `ui.Box` 不跨帧。
// 所以本帧量、存数值、请壳重画一帧（只在页宽真的变了时才请求，避免自激）。
func (p *Plugin) afterDraw(_ *gfx.Canvas, env *pluginapi.Env) {
	if !p.boxes.Rail.Valid() {
		return
	}
	width := p.boxes.Rail.Rect().Width
	per := int((width + itemGap) / (cardWidth + itemGap))
	if per < 1 {
		per = 1
	}
	p.mu.Lock()
	changed := per != p.visible
	p.visible = per
	p.mu.Unlock()
	if changed && env != nil && env.Act.Invalidate != nil {
		env.Act.Invalidate()
	}
}

// clampPageLocked 在移除后把页码收回合法范围（在锁内调用）。
func (p *Plugin) clampPageLocked() {
	pages := pagesLocked(len(p.items), p.visible)
	if pages <= 0 {
		p.page = 0
		return
	}
	if p.page >= pages {
		p.page = pages - 1
	}
	if p.page < 0 {
		p.page = 0
	}
}

// hitID 造一个稳定的交互 id（官方：插件不许写死数字，见 dsdirpicker 的同名方法）。
func (p *Plugin) hitID(env *pluginapi.Env, name string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, name)
}

// ── 文案与格式化（官方 labels.ts / primitives 的同名函数）──────────────────

// displayName 是卡片标题：官方 `attachment.file.name || t('file.label')`
// （OFF:ComposerAttachments.tsx:73 与 FileCard.tsx:53 的 `name`）。
func displayName(it item) string {
	if it.Name == "" {
		return zhFileLabel
	}
	return it.Name
}

// metaText 是卡片第二行，逐态对应官方 `FileCard.tsx:34-38`：
//
//	uploading → labels.uploading                 = file.uploading「上传中…」
//	error     → labels.failed                     = file.uploadFailed，但**先显示宿主原文**
//	ready     → [extension, fileSizeText(bytes)].filter(≠'').join(' ')
//
// 失败态为什么优先宿主原文：官方把宿主的失败原因放在 `uploadFailureText(upload.reason)`
// 里显示（`OFF:ComposerAttachments.tsx:92-94`，走 Tooltip）；本仓没有 Tooltip 层，
// 而宿主原文（"附件不能是 0 bytes"这类）是用户唯一能据此动作的信息。
// 宿主没给文字时才回退官方那句通用文案——**任何时候都不自己编中文**。
func metaText(it item) string {
	switch it.State {
	case stateUploading:
		return zhFileUploading
	case stateError:
		if it.ErrText != "" {
			return it.ErrText
		}
		return zhFileUploadFailed
	default:
		parts := make([]string, 0, 2)
		if ext := extensionText(it.Name); ext != "" {
			parts = append(parts, ext)
		}
		if size := fileSizeText(it.Bytes); size != "" {
			parts = append(parts, size)
		}
		return strings.Join(parts, " ")
	}
}

// metaColor 是第二行的颜色：`.meta` 是 label-tertiary，`.metaFailed` 是 error（:71-82）。
func metaColor(t *pluginapi.Theme, s state) ui.Color {
	if s == stateError {
		return t.Error
	}
	return t.LabelTertiary
}

// removeLabel 套官方 `file.remove` 模板（「移除文件 {name}」）。
func removeLabel(name string) string {
	return strings.ReplaceAll(zhFileRemove, "{name}", displayName(item{Name: name}))
}

// glyphOf 是图标槽里的文字符号：扩展名前 4 个字符（官方这里是 SVG sprite）。
func glyphOf(name string) string {
	ext := extensionText(name)
	runes := []rune(ext)
	if len(runes) > glyphMaxLetters {
		runes = runes[:glyphMaxLetters]
	}
	return string(runes)
}

// extensionText 等价官方的 `fileExtension(name).toUpperCase().slice(0, 8)`
// （`OFF:FileCard.tsx:33`；`fileExtension` 就是 basename 最后一个点之后那段）。
func extensionText(name string) string {
	base := baseName(name)
	dot := strings.LastIndex(base, ".")
	if dot < 0 {
		return ""
	}
	ext := strings.ToUpper(base[dot+1:])
	runes := []rune(ext)
	if len(runes) > 8 {
		runes = runes[:8]
	}
	return string(runes)
}

// baseName 取路径最后一段（Windows 与 POSIX 分隔符都认）。
func baseName(path string) string {
	trimmed := strings.TrimRight(path, `/\`)
	if i := strings.LastIndexAny(trimmed, `/\`); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}

// fileSizeText 是官方 primitives 的 `fileSizeText` 逐行照抄：
//
//	if (bytes < 1024) return `${bytes}B`
//	const kb = bytes/1024; if (kb < 1024) return `${kb<10 ? kb.toFixed(1) : Math.round(kb)}KB`
//	const mb = kb/1024;    if (mb < 1024) return `${mb<10 ? mb.toFixed(1) : Math.round(mb)}MB`
//	const gb = mb/1024;    return `${gb<10 ? gb.toFixed(1) : Math.round(gb)}GB`
//
// **单位与 `B` 之间没有空格**，小数点只有一档（<10 才带）——界面上要一模一样。
func fileSizeText(bytes int64) string {
	if bytes < 1024 {
		return strconv.FormatInt(bytes, 10) + "B"
	}
	kb := float64(bytes) / 1024
	if kb < 1024 {
		return sizeUnitText(kb, "KB")
	}
	mb := kb / 1024
	if mb < 1024 {
		return sizeUnitText(mb, "MB")
	}
	return sizeUnitText(mb/1024, "GB")
}

// sizeUnitText 是 `<10` 取一位小数、否则取整的那一档（JS `toFixed(1)` / `Math.round`）。
func sizeUnitText(value float64, unit string) string {
	if value < 10 {
		return strconv.FormatFloat(math.Round(value*10)/10, 'f', 1, 64) + unit
	}
	return strconv.FormatInt(int64(math.Round(value)), 10) + unit
}

// clamp01 把比例收进 [0,1]（官方 `.progressBar` 的 `Math.min(1, Math.max(0, progress))`）。
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// failureText 取宿主错误的**原文**（`*apiclient.APIError.Message` 就是宿主那句中文）。
//
// 三种情况分得很清，因为"编一句话"和"漏一句话"在这里代价不同：
//
//	是 APIError 且有 message → 原样用（宿主的真话，例如「附件不能是 0 bytes」）
//	是 APIError 但没有 message → **返回空**，交给 meta 行回退官方通用文案
//	                            （把 code 印给用户看等于说了一句他听不懂的话）
//	不是 APIError（网络/编码错误）→ err.Error()，那是本进程的真实失败原因
//
// 与 `plugins/dsdirpicker` 的同名函数同一条规则：有原文就用原文，
// **绝不翻译成自己编的"上传失败，请重试"**。
func failureText(err error) string {
	if err == nil {
		return ""
	}
	var api *apiclient.APIError
	if errors.As(err, &api) {
		return strings.TrimSpace(api.Message)
	}
	return strings.TrimSpace(err.Error())
}
