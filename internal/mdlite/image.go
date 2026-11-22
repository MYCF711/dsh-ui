package mdlite

// markdown 正文里的图片（`#18` (b)）：目的地判定、三态缓存、显示尺寸。
//
// # 官方怎么画的（rc.5，坐标都在 `OFF:` 前缀的 rc.5 前端包里）
//
//	dist/assets/index-Ciz7DZxv.js
//	  case"image":return ik(e.url,e.alt??"",t,r)          // mdast 的 image 节点入口
//	  function aK(...)                                    // 三步判定 + 不可取就当 alt 文本
//	  function XW(e,t,r)                                  // ① 绝对 http(s) 直出；② 路径交给 fileImages
//	  function Gb(e)                                      // 拒绝表：javascript/vbscript/data/blob
//	  function qW(e)                                      // 只放 http:/https:
//	  function Fp(e)                                      // 路径识别（/、./、../、盘符）+ 行号片段
//	  function lK(...)                                    // <img loading=lazy decoding=async>；onError → alt 文本
//	dist/assets/index-CSAdOBx6.css
//	  ._image_13jw0_300{display:block;width:auto;max-width:100%;height:auto;
//	                    border-radius:var(--dsw-radius-md);
//	                    background:var(--dsw-alias-bg-base);object-fit:contain}
//	  ._imageButton_13jw0_471 ._image_13jw0_300{max-width:min(100%,640px);max-height:360px}
//	  ._imageAlt_13jw0_300{color:var(--dsw-alias-label-tertiary);font-style:italic}
//	dsh-client-ui-chat/lib/client.js（zh 词典，`image.*`）
//	  open=查看大图  loading=加载图片…  failed=图片无法预览  dialog=图片预览  close=关闭图片预览
//
// 官方三态**没有额外 chrome**：
//
//	取数中   `<img>` 自己没画出来，浏览器显示 alt 文本 —— 不是转圈、不是骨架
//	取不到   `src === undefined` → `<span class=imageAlt>` 只有 alt（不进失败态）
//	失败     `<img onError>` → `<span class=imageAlt>` 内容是 `图片无法预览 · alt||目的地`
//
// # 本仓为什么需要这一层
//
// 本仓没有浏览器的 `<img>`：字节要自己取（宿主 `workspaceFiles.readBytes` / http）、
// 自己解（`internal/imgdecode`）、自己画（`ui.Image(gfx.Bitmap)`）。所以「URL→字节」
// 是本仓多写的一段代码，**不是用户代价**：官方那边同样要发请求、同样要解码，
// 只是浏览器替它做了。
//
// 这一层只放**与绘制无关的纯事实**，两处渲染器共用：
//
//	destination 判定   官方 Gb/qW/Fp 的等价物
//	三态缓存           一张图只取一次，取回来 Invalidate 重画
//	显示尺寸           官方两条 max-* 规则 + 「从不放大」
//
// 不认识的 scheme 一律**如实退成 alt 文本**（`MarkdownImageUnresolved`），
// 不猜、不改写、不把 `data:` 当文件路径读。

import (
	"hash/fnv"
	"math"
	"strings"
	"sync"

	"github.com/MYCF711/dsh-ui/internal/imgdecode"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
)

// Markdown 正文图片的两条官方上限（`.imageButton .image` 规则，chat 走的就是这条）。
//
// 为什么把 640/360 抄进常量：官方 chat 的 markdown 图片**一定**套着
// `_imageButton_13jw0_471` 那层（`fileImages` 能力在 chat 里恒有），所以用户在
// 官方 chat 里看到的就是"封顶 640×360 的方图"。本仓不做灯箱（`#18` (a)），
// 但**几何照它有**——不然一张 4000px 的截图会把整列正文顶到天边。
const (
	// MarkdownImageMaxWidth 是 `max-width:min(100%,640px)` 里的那个 640。
	MarkdownImageMaxWidth float32 = 640
	// MarkdownImageMaxHeight 是 `max-height:360px`。
	MarkdownImageMaxHeight float32 = 360
)

// MarkdownImageKind 是目的地判定后的三档。
type MarkdownImageKind uint8

const (
	// MarkdownImageRefused 是"官方也不会取"的目的地：空串、控制字符、
	// `data:` / `javascript:` / `vbscript:` / `blob:` / 其它 scheme（`file:`、`ftp:` …）。
	//
	// 依据是官方 `Gb`（`OFF:dist/assets/index-Ciz7DZxv.js`）：它把
	// javascript/vbscript/data/blob 与**所有不是 http(s) 的 scheme** 都退成空串，
	// 于是 `XW` 拿不到 URL → `src === undefined` → 只有 alt 文本。
	MarkdownImageRefused MarkdownImageKind = iota
	// MarkdownImageURL 是绝对 http(s) 地址（官方 `qW` 放行的那一类）。
	MarkdownImageURL
	// MarkdownImagePath 是路径：`/abs`、`\\unc`、`C:\…`、`./x`、`../x`、以及裸相对 `x.png`。
	//
	// 裸相对路径也算：官方 `Fp("x.png")` 过完 `!Kb && URL.canParse` 的正反两道
	// 判定后返回 `{path:'x.png'}`，再由 `fileImages.resolve` 用**会话 cwd** 拼成绝对路径。
	MarkdownImagePath
)

// MarkdownImageRef 是一个 markdown 图片目的地判定后的结论。
type MarkdownImageRef struct {
	// Kind 是这一档。
	Kind MarkdownImageKind
	// Destination 是目的地**原文**（不改写、不转义）。
	Destination string
}

// ParseMarkdownImage 判定一个 markdown 图片目的地（官方 `Gb` + `qW` + `Fp` 的等价物）。
//
// 判定顺序照官方，不给"聪明"的例外：
//
//	空串 / 含 NUL 与控制字符（官方 Fp 的 `/[\u0000-\u001f\u007f]/`）→ Refused
//	盘符开头 `C:` + 分隔符（官方 Kb 的三条之一）                  → Path
//	以 `/` 或 `\\` 开头                                           → Path
//	以 `./` 或 `../` 开头                                         → Path
//	有 scheme：http/https → URL；其余一律 Refused
//	其余（含裸相对名）                                            → Path
func ParseMarkdownImage(destination string) MarkdownImageRef {
	if destination == "" || hasControlChars(destination) {
		return MarkdownImageRef{Kind: MarkdownImageRefused, Destination: destination}
	}
	if scheme, rest, ok := splitScheme(destination); ok {
		switch scheme {
		case "http", "https":
			// 官方 `qW` 要求 `new URL` 能解析；`http:` 后面什么都没有就不是地址。
			if rest == "" {
				return MarkdownImageRef{Kind: MarkdownImageRefused, Destination: destination}
			}
			return MarkdownImageRef{Kind: MarkdownImageURL, Destination: destination}
		default:
			// `javascript:` / `vbscript:` / `data:` / `blob:` / `file:` / `ftp:` / `mailto:` …
			// 官方一个都不取（Gb 显式拒四个，qW 只放两个）。
			return MarkdownImageRef{Kind: MarkdownImageRefused, Destination: destination}
		}
	}
	// 盘符不算 scheme（官方 Kb：`/^(?:[\\/]|\.{1,2}[\\/]|[a-z]:[\\/])/i`）。
	if isWindowsDrivePath(destination) {
		return MarkdownImageRef{Kind: MarkdownImagePath, Destination: destination}
	}
	return MarkdownImageRef{Kind: MarkdownImagePath, Destination: destination}
}

// splitScheme 把 `scheme:rest` 拆开；盘符（`C:` + 分隔符）不算 scheme。
func splitScheme(destination string) (scheme, rest string, ok bool) {
	colon := strings.IndexByte(destination, ':')
	if colon <= 0 {
		return "", "", false
	}
	// scheme 只允许 RFC 3986 的字符；出现 `/`、`\`、空格说明不是 scheme 而是路径里的冒号。
	for i := 0; i < colon; i++ {
		c := destination[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9', c == '+', c == '-', c == '.':
		default:
			return "", "", false
		}
	}
	if isWindowsDrivePath(destination) {
		return "", "", false
	}
	return strings.ToLower(destination[:colon]), destination[colon+1:], true
}

// isWindowsDrivePath 报告 `C:\x` / `C:/x` 这种盘符路径（官方 Kb 的第三条）。
func isWindowsDrivePath(destination string) bool {
	if len(destination) < 3 {
		return false
	}
	c := destination[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		return false
	}
	if destination[1] != ':' {
		return false
	}
	return destination[2] == '/' || destination[2] == '\\'
}

// isAbsolutePath 报告一个路径是不是绝对（官方 `isAbsoluteWorkspacePath`：
// POSIX `/a`、Windows 盘符 `C:/a`、UNC `\server\share`）。相对路径要靠 cwd 才落地。
func isAbsolutePath(path string) bool {
	if strings.HasPrefix(path, "/") {
		return true
	}
	return isWindowsDrivePath(path) || strings.HasPrefix(path, `\\`)
}

// hasControlChars 是官方 `Fp` 的 `/[\u0000-\u001f\u007f]/` 判定。
func hasControlChars(destination string) bool {
	for i := 0; i < len(destination); i++ {
		if destination[i] < 0x20 || destination[i] == 0x7f {
			return true
		}
	}
	return false
}

// MarkdownImageState 是一张图的四态。
//
// 为什么是四态而不是三态：官方把"取不到地址"和"取到了但加载失败"画成**两种不同**
// 的文案（前者只有 alt，后者 `图片无法预览 · alt||目的地`），合并成一态就必然
// 给其中一种加多余的失败文案。
type MarkdownImageState uint8

const (
	// MarkdownImageUnresolved 是"没有可取的地址/没有取数能力"（官方 `src === undefined`）。
	MarkdownImageUnresolved MarkdownImageState = iota
	// MarkdownImagePending 是"字节还没回来"（官方 `<img>` 还没解出来那一瞬）。
	MarkdownImagePending
	// MarkdownImageReady 是有位图可画。
	MarkdownImageReady
	// MarkdownImageFailed 是取字节或解码失败（官方 `<img onError>`）。
	MarkdownImageFailed
)

// MarkdownImage 是一张图当帧的三态快照。
type MarkdownImage struct {
	// State 是当帧状态。
	State MarkdownImageState
	// Bitmap 只在 State == MarkdownImageReady 时有意义。
	Bitmap gfx.Bitmap
}

// MarkdownImageAltText 是官方的 `alt || destination`（`OFF:lK`：`t||r`）。
//
// 官方两处兜底都用它：alt 为空时把**目的地原文**当文本，而不是画空。
func MarkdownImageAltText(alt, destination string) string {
	if alt != "" {
		return alt
	}
	return destination
}

// MarkdownImageFailedText 是官方失败态文案：“ `${labels.failed} · ${alt||destination}` “。
//
// `image.failed` 的 zh 字面量是「图片无法预览」（`dsh-client-ui-chat/lib/client.js`
// 的 zh 段），分隔符是官方的 ` · `（空格 + 中点 + 空格），一处都不许改。
func MarkdownImageFailedText(alt, destination string) string {
	return "图片无法预览 · " + MarkdownImageAltText(alt, destination)
}

// FitMarkdownImage 按官方的两条 max-* 规则算显示尺寸，返回宽高（逻辑像素）。
//
// 规则逐条（`._image_13jw0_300` 的两条 CSS）：
//
//	max-width:min(100%, 640px)   宽度封顶 = 列宽与 640 的较小者
//	max-height:360px             高度封顶 360
//	width:auto / height:auto     先按原始像素尺寸，**从不放大**
//	object-fit:contain           压的时候保持比例
//
// column <= 0 表示"列宽未知"：这时只按 640/360 封顶（宁可多给横向空间，
// 也不把图按 0 宽画没）。
func FitMarkdownImage(bitmap gfx.Bitmap, column float32) (float32, float32) {
	if bitmap.Width <= 0 || bitmap.Height <= 0 {
		return 0, 0
	}
	width := float32(bitmap.Width)
	height := float32(bitmap.Height)
	maxWidth := MarkdownImageMaxWidth
	if column > 0 && column < maxWidth {
		maxWidth = column
	}
	if width > maxWidth {
		height = height * maxWidth / width
		width = maxWidth
	}
	if height > MarkdownImageMaxHeight {
		width = width * MarkdownImageMaxHeight / height
		height = MarkdownImageMaxHeight
	}
	// 两条封顶只会把像素压小；舍入到整像素，避免半像素描边。
	return float32(math.Round(float64(width))), float32(math.Round(float64(height)))
}

// MarkdownImageLoader 取一张 markdown 图片的**原始字节**。
//
// 同步、阻塞、必须在 goroutine 里调：绘制帧里不许等 HTTP（同消息区图片面那条规矩，
// 见 `plugins/dsconversation/official_message_image.go` 文件头）。
//
//	ok=false  取不到（网络失败 / 非 2xx / 空字节 / 宿主拒绝）→ 官方失败态
//	ok=true   字节原样返回，解码失败也归失败态（不拿零位图冒充成功）
type MarkdownImageLoader func(sessionID, cwd, destination string) ([]byte, bool)

// markdownImageEntry 是一张图的缓存项。
type markdownImageEntry struct {
	state  MarkdownImageState
	bitmap gfx.Bitmap
}

// MarkdownImageResolver 是 markdown 正文图片的缓存与取数调度。
//
// 为什么必须有缓存：`MarkdownStyle.Image` 每帧、每张图都要问一次（官方文档原话：
// "It is asked on every Layout, and more than once for one image"），而取字节是 HTTP。
// 缓存把"每帧一次请求"降成"每张图一次请求"，也让取到之后不再重取。
//
// 并发：`Lookup` 在绘制 goroutine 上调，取数在插件起的 goroutine 里跑；
// 两者只经锁交换状态。`redraw` 在取数完成时调一次（壳的 `Invalidate`）。
type MarkdownImageResolver struct {
	mu         sync.Mutex
	entries    map[string]markdownImageEntry
	inflight   map[string]bool
	load       MarkdownImageLoader
	redraw     func()
	generation uint64
}

// NewMarkdownImageResolver 造一个空缓存。
func NewMarkdownImageResolver() *MarkdownImageResolver {
	return &MarkdownImageResolver{
		entries:  map[string]markdownImageEntry{},
		inflight: map[string]bool{},
	}
}

// Use 装上当帧的取数入口与重画回调（每帧可重复调用，同一份实现重复装是幂等的）。
//
// load 为 nil = 壳没接这条能力 → 所有图都停在 `MarkdownImageUnresolved`（官方画 alt），
// **不画假图、也不假装失败**。这与消息区图片面"壳没接就不画成功态"同一条判据。
func (r *MarkdownImageResolver) Use(load MarkdownImageLoader, redraw func()) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.load = load
	r.redraw = redraw
}

// Generation 报告缓存内容的代号：每有一张图换状态就 +1。
//
// 用途：外层若按"文本 + 宽度"缓存了排版结果（`plugins/dsconversation/conversation.go`
// 的 `layoutMarkdownWithFiles` 就是），必须把代号并进缓存键——否则图片取回来之后
// 排版缓存不失效，图永远不出现。
func (r *MarkdownImageResolver) Generation() uint64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.generation
}

// Lookup 取一张图当帧的状态，并在需要时起一次后台取数。
//
//	allowPath=false 只认绝对 http(s)（官方流式期间就是这么办的：`streaming` 时
//	                `aK` 根本不调 `fileImages.resolve`，路径图先显示 alt，定稿后才出图）
//
// 同一张图的并发 `Lookup` 只会起**一个**请求（`inflight` 去重）。
func (r *MarkdownImageResolver) Lookup(sessionID, cwd, destination string, allowPath bool) MarkdownImage {
	ref := ParseMarkdownImage(destination)
	if ref.Kind == MarkdownImageRefused || (ref.Kind == MarkdownImagePath && !allowPath) {
		return MarkdownImage{State: MarkdownImageUnresolved}
	}
	// cwd 未知 + 相对路径：官方也取不到——`fileMediaUrl` 只给**绝对**路径开 URL
	// （`OFF:dsh-util-workspace-path` 的 `/^https?:/ && isAbsoluteWorkspacePath(path)`），
	// 所以这一档必须停在"取不到地址"，不许当成失败去画「图片无法预览」。
	if ref.Kind == MarkdownImagePath && cwd == "" && !isAbsolutePath(ref.Destination) {
		return MarkdownImage{State: MarkdownImageUnresolved}
	}
	if r == nil {
		return MarkdownImage{State: MarkdownImageUnresolved}
	}
	key := markdownImageKey(sessionID, cwd, ref)
	r.mu.Lock()
	entry, ok := r.entries[key]
	if !ok {
		entry = markdownImageEntry{state: MarkdownImagePending}
		r.entries[key] = entry
	}
	load, redraw := r.load, r.redraw
	if entry.state == MarkdownImageReady || entry.state == MarkdownImageFailed {
		r.mu.Unlock()
		return MarkdownImage{State: entry.state, Bitmap: entry.bitmap}
	}
	if load == nil {
		// 没有取数入口：如实停在"取不到地址"这一档（官方画 alt）。
		r.mu.Unlock()
		return MarkdownImage{State: MarkdownImageUnresolved}
	}
	if r.inflight[key] {
		r.mu.Unlock()
		return MarkdownImage{State: MarkdownImagePending}
	}
	r.inflight[key] = true
	r.mu.Unlock()

	go r.fetch(key, sessionID, cwd, ref, load, redraw)
	return MarkdownImage{State: MarkdownImagePending}
}

// fetch 在后台取一次字节并解码（**只在 goroutine 里跑**）。
func (r *MarkdownImageResolver) fetch(key, sessionID, cwd string, ref MarkdownImageRef, load MarkdownImageLoader, redraw func()) {
	entry := markdownImageEntry{state: MarkdownImageFailed}
	if body, ok := load(sessionID, cwd, ref.Destination); ok && len(body) > 0 {
		if bitmap, err := imgdecode.Decode(body); err == nil && bitmap.Width > 0 && bitmap.Height > 0 {
			bitmap.ID = markdownImageBitmapID(key)
			bitmap.Version = 1
			entry = markdownImageEntry{state: MarkdownImageReady, bitmap: bitmap}
		}
	}
	r.mu.Lock()
	delete(r.inflight, key)
	r.entries[key] = entry
	r.generation++
	r.mu.Unlock()
	if redraw != nil {
		redraw()
	}
}

// markdownImageKey 是缓存键：同一目的地在不同会话/工作区下是**不同的图**。
//
// 为什么 cwd 进键：相对路径（`./shot.png`）的含义由会话 cwd 决定，切了工作区
// 就换了图；不带 cwd 会把上一个工作区的图顶到新会话里。
func markdownImageKey(sessionID, cwd string, ref MarkdownImageRef) string {
	return sessionID + "\x00" + cwd + "\x00" + ref.Destination
}

// markdownImageBitmapID 由缓存键派生稳定位图 id（viva 的纹理缓存按 ID+Version 认图，
// 见 `MarkdownStyle.Image` 的文档）。同一张图每帧都用同一个 ID，像素不会再上传。
func markdownImageBitmapID(key string) uint64 {
	if key == "" {
		return 0
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(key))
	// 0 在 viva 里表示"没有 id"，避开它。
	if id := hash.Sum64(); id != 0 {
		return id
	}
	return 1
}

// MarkdownInlineRun 是一段连续的同质行内内容：文字，或者一张图。
//
// 为什么需要它：本仓主区的正文渲染器（`plugins/dsconversation` 的
// `OfficialTranscriptView.drawMarkdown`）把整段 spans 拍平成一行文字，
// 图片只能退成 alt 文本。要在正文里出图，就得先把 spans 按**原文顺序**切成
// "文字段 / 图片段"，再各画各的。
type MarkdownInlineRun struct {
	// Text 是这段文字；Image 非空时它是官方 `alt`（也可能是空串）。
	Text string
	// Image 是图片目的地原文；空 = 这是文字段。
	Image string
	// Mono 是"这一段全是行内代码"（官方 MarkdownText 的 Mono span）。
	Mono bool
}

// SplitMarkdownInline 把一段 spans 切成文字段与图片段，保持原文顺序。
//
// 没有图片时**必须**返回恰好一段、且 Text 与旧的 `spansText` 逐字节相同：
// 那条路径是既有正文渲染的唯一路径，切分不许改动它的任何一个像素。
func SplitMarkdownInline(spans []markdown.Span) []MarkdownInlineRun {
	hasImage := false
	for _, span := range spans {
		if span.Image != "" {
			hasImage = true
			break
		}
	}
	if !hasImage {
		var text strings.Builder
		for _, span := range spans {
			text.WriteString(span.Text)
		}
		return []MarkdownInlineRun{{Text: text.String(), Mono: allMonoSpans(spans)}}
	}
	runs := make([]MarkdownInlineRun, 0, len(spans))
	var pending strings.Builder
	pendingMono := true
	pendingAny := false
	flush := func() {
		if !pendingAny {
			return
		}
		runs = append(runs, MarkdownInlineRun{Text: pending.String(), Mono: pendingMono})
		pending.Reset()
		pendingMono = true
		pendingAny = false
	}
	for _, span := range spans {
		if span.Image != "" {
			flush()
			runs = append(runs, MarkdownInlineRun{Text: span.Text, Image: span.Image})
			continue
		}
		pending.WriteString(span.Text)
		pendingAny = true
		if span.Style&markdown.Mono == 0 {
			pendingMono = false
		}
	}
	flush()
	return runs
}

// allMonoSpans 报告一段 spans 是不是**整段行内代码**（`spansText` 的旧判据）。
func allMonoSpans(spans []markdown.Span) bool {
	if len(spans) == 0 {
		return false
	}
	mono := false
	for _, span := range spans {
		if span.Text == "" || span.Text == markdown.Newline {
			continue
		}
		if span.Style&markdown.Mono == 0 {
			return false
		}
		mono = true
	}
	return mono
}
