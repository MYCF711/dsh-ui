// markdown_image.go —— markdown 正文图片的**壳侧取数**（`#18` (b)）。
//
// # 官方那一条到底做什么
//
//	OFF:dist/assets/index-Ciz7DZxv.js
//	  function aK(...)      正文图片入口：可解析的目的地才画 `<img>`，否则只画 alt 文本
//	  function XW(e,t,r)    ① 绝对 http(s) 原样当 src（浏览器自己取）
//	                       ② 否则交给 `pathImages.resolve` / `fileImages.resolve`
//	  function Gb(e)        拒绝表：javascript / vbscript / data / blob 一律退成空串
//	  function qW(e)        白名单：只放 http: / https:
//	  function Vp(e,t)      给 `…/api/file` 的 URL 补 `load=<序号>`（只用来破缓存）
//	OFF:dsh-api-session-controller/lib/index.js:3240,3323
//	                        `GET /api/file?path=…`：认证后按 `ctx.fs` 读**字节**，
//	                        Content-Type 由 `mime.lookup(displayPath)` 决定（不看路径白名单）
//	OFF:dsh-client-ui-chat/lib/client.js 的 `fileImages.resolve`
//	                        `resolveWorkspacePath(cwd, path)` → 绝对路径 → `api/file?path=`
//
// 也就是说：**官方正文图片只有两条取数来源** —— 浏览器直接发 HTTP（URL 档），
// 或宿主的一条字节路由（路径档）。本仓没有浏览器层，两条都归这里：
//
//	URL 档   自己发 GET（带超时、卡字节上限、只收 image/*）
//	路径档   交给宿主 `workspaceFiles.readBytes`（真字节；路径解析在宿主侧）
//
// # 为什么这一层必须存在（而不是"等宿主给一条 URL"）
//
// 官方的 `<img src>` 就是一次 HTTP GET。本仓要复刻它，缺的从来不是协议方法，
// 而是"谁来发这个 GET"。把这件事放在壳里，是因为只有壳同时具备两样东西：
// `m.cli`（apiclient，宿主认证）与 `internal/imgdecode`（解码）。插件两条都拿不到。
//
// # 与官方逐条对齐的边界
//
//	只认 http/https      其余 scheme 一律不取（官方 qW）
//	空/控制字符/空 path  不取（官方 Fp）
//	路径必须落到绝对     相对路径用会话 cwd 拼（官方 resolveWorkspacePath）
//	0 字节算失败         不许把"没内容"当成功（同 copy_image.go 的判据）
//	上限 50 MiB          官方 `imageLimits.maxImageBytes` 默认值 52428800
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

const (
	// markdownImageMaxBytes 是单张正文图片的字节上限。
	//
	// 取值依据是官方宿主自己的图片上限默认值：`maxImageBytes: config.maxImageBytes ?? 52428800`
	// （`OFF:dsh-attachment-local` / `dsh-base` 的图片限额装配），也就是 50 MiB。
	// 用官方同一个数，是为了"官方能显示的图本仓也能显示"，不自己发明更紧的上限。
	markdownImageMaxBytes int64 = 52_428_800

	// markdownImageFetchTimeout 是一次 URL 取数的总时限。
	//
	// 为什么比 `fileSaveTimeout`（5 s）宽松：正文图片**没有重试入口**（官方也没有），
	// 一次过早超时就等于这张图永远变成 alt 文本；而这条 HTTP 跑在插件起的
	// goroutine 里，不占 UI 线程，拖久一点也不卡界面。
	markdownImageFetchTimeout = 30 * time.Second

	// markdownImageReadTimeout 是读一个本地工作区文件的时限：走宿主回环 HTTP 的
	// `workspaceFiles.readBytes`，与保存链同一个量级（`fileSaveTimeout`）。
	markdownImageReadTimeout = 10 * time.Second

	// markdownImageMaxRedirects 是 URL 取数允许的跳转次数（官方浏览器同样有上限，
	// 这里取一个保守值，避免被一个跳转环拖到超时）。
	markdownImageMaxRedirects = 5
)

// markdownImageHTTPClient 是 URL 取数用的客户端（测试按下它换掉）。
//
// 显式设置 Timeout 而不是用 http.DefaultClient：默认客户端没有总时限，
// 一个不响应的地址会让 goroutine 永远挂着。
var markdownImageHTTPClient = &http.Client{
	Timeout: markdownImageFetchTimeout,
	CheckRedirect: func(_ *http.Request, via []*http.Request) error {
		if len(via) >= markdownImageMaxRedirects {
			return fmt.Errorf("重定向超过 %d 次", markdownImageMaxRedirects)
		}
		return nil
	},
}

// markdownImageBytes 实现 `pluginapi.Actions.MarkdownImage`。
//
// 返回 ok=false 的每一档都对应官方的"取不到"：scheme 不认识、路径落不到绝对、
// 网络失败、非 2xx、非图片类型、超过上限、0 字节、壳没有 apiclient。
// 界面拿到 false 就画官方失败态（`图片无法预览 · alt||目的地`），不重试、不弹窗。
func (m *model) markdownImageBytes(sessionID, cwd, destination string) (pluginapi.MarkdownImageBytes, bool) {
	ref := mdlite.ParseMarkdownImage(destination)
	switch ref.Kind {
	case mdlite.MarkdownImageURL:
		body, mediaType, err := fetchMarkdownImageURL(ref.Destination)
		if err != nil || len(body) == 0 {
			return pluginapi.MarkdownImageBytes{}, false
		}
		return pluginapi.MarkdownImageBytes{Bytes: body, MIME: mediaType}, true
	case mdlite.MarkdownImagePath:
		return m.readMarkdownImageFile(sessionID, cwd, ref.Destination)
	default:
		return pluginapi.MarkdownImageBytes{}, false
	}
}

// readMarkdownImageFile 走宿主 `workspaceFiles.readBytes` 读一张本地图。
func (m *model) readMarkdownImageFile(sessionID, cwd, destination string) (pluginapi.MarkdownImageBytes, bool) {
	if m == nil || m.cli == nil || sessionID == "" {
		return pluginapi.MarkdownImageBytes{}, false
	}
	path := resolveMarkdownImagePath(cwd, destination)
	if !isAbsoluteMarkdownImagePath(path) {
		// 官方 `fileMediaUrl` 只给绝对路径开 URL：cwd 未知时相对路径根本取不到。
		return pluginapi.MarkdownImageBytes{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), markdownImageReadTimeout)
	defer cancel()
	content, err := m.cli.WorkspaceFileReadBytes(ctx, sessionID, path)
	if err != nil || content == nil || len(content.Data) == 0 {
		return pluginapi.MarkdownImageBytes{}, false
	}
	return pluginapi.MarkdownImageBytes{Bytes: content.Data, MIME: markdownImageMediaType(path)}, true
}

// fetchMarkdownImageURL 取一张 http(s) 图片的字节。
//
// 三道闸，都是为了让"不是图"的东西尽快失败、且失败得**有界**：
//
//	状态码     非 2xx 直接失败（官方 `<img>` 同样不画非 2xx）
//	类型       Content-Type 有值且不是 image/* 直接失败（省下一次完整下载）
//	字节上限   用 LimitReader 多读 1 字节判断越界，超了失败（官方同样的 50 MiB 上限）
func fetchMarkdownImageURL(destination string) ([]byte, string, error) {
	return fetchMarkdownImageURLWithin(destination, markdownImageMaxBytes)
}

// fetchMarkdownImageURLWithin 是带显式上限的那一版（上限是参数，测试才能用小数字验越界）。
func fetchMarkdownImageURLWithin(destination string, maxBytes int64) ([]byte, string, error) {
	request, err := http.NewRequest(http.MethodGet, destination, nil)
	if err != nil {
		return nil, "", err
	}
	// 只要图：省得把一个写着图片地址、实际回 HTML 的端点整页拉下来。
	request.Header.Set("Accept", "image/*")
	response, err := markdownImageHTTPClient.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, "", fmt.Errorf("HTTP %d", response.StatusCode)
	}
	mediaType := markdownImageMediaTypeOfHeader(response.Header.Get("Content-Type"))
	if mediaType != "" && !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return nil, mediaType, fmt.Errorf("不是图片：%s", mediaType)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, mediaType, err
	}
	if int64(len(body)) > maxBytes {
		return nil, mediaType, errors.New("图片超过字节上限")
	}
	return body, mediaType, nil
}

// markdownImageMediaTypeOfHeader 取 Content-Type 的裸类型（去掉 `;charset=…`）。
func markdownImageMediaTypeOfHeader(header string) string {
	if header == "" {
		return ""
	}
	if index := strings.IndexByte(header, ';'); index >= 0 {
		header = header[:index]
	}
	return strings.ToLower(strings.TrimSpace(header))
}

// markdownImageMediaType 按扩展名猜一张本地图的媒体类型（猜不到就空串）。
//
// 它不是判据：真正的判据是插件侧能不能解码出位图。这里只为 shell 的返回值完整，
// 与官方 `mime.lookup(displayPath)` 同一个信息来源（扩展名）。
func markdownImageMediaType(path string) string {
	if extension := filepath.Ext(path); extension != "" {
		return markdownImageMediaTypeOfHeader(mime.TypeByExtension(extension))
	}
	return ""
}

// resolveMarkdownImagePath 是官方 `resolveWorkspacePath(cwd, path)` 的逐行移植
// （`OFF:dsh-util-workspace-path/lib/index.js` 的同名函数）。
//
//	isAbsoluteWorkspacePath(path)  → 原样返回（`/a`、`C:/a`、`\\unc`）
//	cwd 为空                       → 原样返回（相对路径就此落不到绝对，→ 取不到）
//	否则                           → cwd（去尾分隔符）+ 分隔符 + path（去前导 `./` 与分隔符）
//
// 几点必须照抄，否则官方能显示的图本仓读不到：
//
//	分隔符跟着 cwd 的拼写走（`D:\w` 用 `\`，`/w` 用 `/`）——Windows 上混拼也能读，
//	但"逐行对齐"比"碰巧能读"好排错；
//	`../a.png` **不去前导点**（官方正则 `^(?:\.?[/\\])+` 匹配不到 `../`），
//	原样拼给宿主，由宿主的工作区解析决定能不能出界；
//	前导 `./` 只去一次还是多次由那个 `+` 决定——官方是**多次**，这里也循环去。
func resolveMarkdownImagePath(cwd, path string) string {
	if path == "" {
		return ""
	}
	if isAbsoluteMarkdownImagePath(path) {
		return path
	}
	if cwd == "" {
		return path
	}
	separator := "/"
	if isWindowsStyleMarkdownPath(cwd) && strings.Contains(cwd, `\`) {
		separator = `\`
	}
	root := strings.TrimRight(cwd, `/\`)
	trimmed := path
	for {
		if strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, `.\`) {
			trimmed = trimmed[2:]
			continue
		}
		if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, `\`) {
			trimmed = trimmed[1:]
			continue
		}
		break
	}
	return root + separator + trimmed
}

// isAbsoluteMarkdownImagePath 是官方 `isAbsoluteWorkspacePath`：POSIX `/a`、
// Windows 盘符 `C:/a`、UNC `\\server\share`。
func isAbsoluteMarkdownImagePath(path string) bool {
	return strings.HasPrefix(path, "/") || isWindowsStyleMarkdownPath(path)
}

// isWindowsStyleMarkdownPath 是官方 `isWindowsStylePath`：盘符 + 分隔符，或 UNC 前缀。
func isWindowsStyleMarkdownPath(value string) bool {
	if len(value) >= 3 {
		c := value[0]
		if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) && value[1] == ':' &&
			(value[2] == '/' || value[2] == '\\') {
			return true
		}
	}
	return strings.HasPrefix(value, `\\`)
}
