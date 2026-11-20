// official_account_avatar.go —— 官方 `AccountAvatarLoader` 的 Go 复刻
// （`dsh-client-ui-settings-account/src/client/account-avatar.ts:11-92` +
// `AccountAvatar.tsx:11-19`）。
//
// # 官方语义（逐条对应，不许自造）
//
//	account-avatar.ts:24-33  setUrl：空/缺失 URL → 清空；URL 未变 → 不动
//	account-avatar.ts:36-38  retry：**只**重试 status==='failed' 的那一次
//	account-avatar.ts:44-48  failed：同一张图渲染失败 → detach + status='failed'
//	account-avatar.ts:50-54  reset：释放图但不发布（调用方随新账号帧一起发）
//	account-avatar.ts:62-77  load：先发 loading，再 load；成功 ready，失败 failed
//	account-avatar.ts:66      `image.referrerPolicy = 'no-referrer'` → 请求不带 Referer
//	AccountAvatar.tsx:15-18  只有 status==='ready' 才画 `<img>`，否则画
//	                         `IconUserOutlineMedium size={16}`
//	AccountAvatar.module.css:1 `object-fit: cover` + 100%×100% + 50% 圆角
//
// # 本仓为什么是"加载器 + 位图"而不是 `<img>` 元素
//
// viva 没有 HTML `<img>`；`ui.Image(c, gfx.Bitmap)` 拿的就是解码后的位图
// （`viva@v0.6.4/ui/elements.go:53` → `gfx.Bitmap{Pix,Width,Height,Stride}`）。
// 所以这里做的是同一件事的两个等价步骤：**下载 + 解码**（官方交给浏览器），
// 然后按官方三态（loading / ready / failed）决定画图还是画兜底图标。
//
// 三条"不编"：
//  1. 没拿到 avatarUrl（profile 未就绪 / Platform 给了 null）→ 不清空已有图、
//     也不画假图，保持官方兜底图标；
//  2. 解码失败 → status='failed'，画官方兜底图标（官方 onError 走的也是这条）；
//  3. 卸载（Dispose 交付）→ 释放位图，后续回调一律丢弃（account-avatar.ts:57-60）。
package dssettings

import (
	"context"
	"errors"
	"image"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/MYCF711/dsh-ui/internal/imgdecode"
	"github.com/vivaui/viva/gfx"
)

// AccountAvatarStatus 是官方 `AccountAvatarSnapshot['status']` 的三态。
type AccountAvatarStatus string

const (
	// AccountAvatarLoading 是 `{url, status:'loading'}`（官方 :75）。
	AccountAvatarLoading AccountAvatarStatus = "loading"
	// AccountAvatarReady 是 `{url, status:'ready'}`（官方 :73）。
	AccountAvatarReady AccountAvatarStatus = "ready"
	// AccountAvatarFailed 是 `{url, status:'failed'}`（官方 :74 / :47）。
	AccountAvatarFailed AccountAvatarStatus = "failed"
)

// AccountAvatarMaxBytes 是一张头像的最大接受体积。
//
// 官方没有这个上限（浏览器自己扛），但 Go 侧必须给：远端给一个无限流会
// 把内存吃干。8 MiB 足够任何平台头像（官方 OnboardingIllustration 的 webp
// 也只有几十 KB），超限按加载失败处理——**如实失败**，不是静默当成功。
const AccountAvatarMaxBytes = 8 << 20

// AccountAvatarSnapshot 是官方 `AccountAvatarSnapshot`（account-avatar.ts:4-9）。
type AccountAvatarSnapshot struct {
	// URL 是产出这个状态的图片地址。
	URL string
	// Status 是加载结果；只有 Ready 才画图。
	Status AccountAvatarStatus
	// Bitmap 是解码后的位图（仅 Ready 非零）。
	Bitmap gfx.Bitmap
}

// accountAvatarLoader 是官方 `AccountAvatarLoader` 的值类型版本。
//
// 与官方一样是**插件生命周期内唯一一份**：侧栏启动器与设置页共读同一张图，
// 所以两次 `setUrl` 同一个 URL 只会下一份（account-avatar.ts:27）。
type accountAvatarLoader struct {
	mu sync.Mutex
	// snapshot 是最后一次发布的状态；nil = 没有图（官方 undefined）。
	snapshot *AccountAvatarSnapshot
	// url 是正在持有点的 URL（官方 `this.snapshot?.url`）。
	url string
	// generation 每换一次 URL +1：旧请求回来时对不上就丢弃
	// （官方用 `this.image !== image` 做同一件事）。
	generation uint64
	// disposed 对应官方 `this.disposed`：置真后一切回调丢弃。
	disposed bool
	// client 是取图用的 HTTP 客户端（nil → DefaultAvatarHTTPClient）。
	client *http.Client
	// publish 是发布口；官方是 `publish({...snapshot, avatar: value})`。
	publish func(AccountAvatarSnapshot)
	// invalidate 是"有新图了，重画"的壳入口。
	invalidate func()
	// loading 是还在飞的 fetch 数（测试与 Dispose 用，不参与渲染）。
	loading int
}

// DefaultAvatarHTTPClient 是取头像用的客户端。
//
// Referer 由官方 `referrerPolicy='no-referrer'` 决定：请求**不带** Referer
// （某些图床按 Referer 防盗链，带了反而 403）。这里显式不设该头，
// Go 的 http.Client 默认就不发 Referer——所以只需不额外加。
var DefaultAvatarHTTPClient = &http.Client{}

// setURL 是官方 `setUrl`（account-avatar.ts:24-33）。
//
//   - disposed → 直接返回（:25）；
//   - URL 没变（含"都为空"）→ 不动（:27）；
//   - 源为空 → 清空（:28-31）；否则开始加载（:32）。
//
// 返回 true 表示**状态被改动**，调用方据此请求重画。
func (l *accountAvatarLoader) setURL(url string) bool {
	l.mu.Lock()
	if l.disposed {
		l.mu.Unlock()
		return false
	}
	source := strings.TrimSpace(url)
	current := ""
	if l.snapshot != nil {
		current = l.snapshot.URL
	}
	if current == source {
		l.mu.Unlock()
		return false
	}
	if source == "" {
		l.detachLocked()
		l.publishLocked(nil)
		l.mu.Unlock()
		return true
	}
	l.mu.Unlock()
	l.load(source)
	return true
}

// retry 是官方 `retry`（account-avatar.ts:36-38）：只重试失败的那一次。
func (l *accountAvatarLoader) retry() bool {
	l.mu.Lock()
	if l.disposed || l.snapshot == nil || l.snapshot.Status != AccountAvatarFailed {
		l.mu.Unlock()
		return false
	}
	url := l.snapshot.URL
	l.mu.Unlock()
	l.load(url)
	return true
}

// reset 是官方 `reset`（account-avatar.ts:50-54）：释放图但不发布。
//
// 官方在**每个账号帧**都调它，然后随新帧一起 `publish({..., avatar: undefined})`；
// 本仓的等价动作由调用方在同步账号快照后直接调 `setURL`，所以 reset
// 只保留给"账号被吊销"那一条（index.ts:111-121 的 revocation 分支）。
func (l *accountAvatarLoader) reset() {
	l.mu.Lock()
	l.detachLocked()
	l.snapshot = nil
	l.url = ""
	l.generation++
	l.mu.Unlock()
}

// dispose 是官方 `dispose`（account-avatar.ts:56-60）。
func (l *accountAvatarLoader) dispose() {
	l.mu.Lock()
	l.disposed = true
	l.detachLocked()
	l.snapshot = nil
	l.url = ""
	l.generation++
	l.mu.Unlock()
}

// snapshotValue 取当前状态（nil = 没有图）。
func (l *accountAvatarLoader) snapshotValue() *AccountAvatarSnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.snapshot == nil {
		return nil
	}
	copied := *l.snapshot
	return &copied
}

// loadingCount 是还在飞的 fetch 数（测试等它收敛，不参与渲染）。
func (l *accountAvatarLoader) loadingCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loading
}

// load 是官方 `load`（account-avatar.ts:62-77）：先发 loading，再取图并解码。
func (l *accountAvatarLoader) load(url string) {
	l.mu.Lock()
	if l.disposed {
		l.mu.Unlock()
		return
	}
	l.generation++
	generation := l.generation
	l.loading++
	l.publishLocked(&AccountAvatarSnapshot{URL: url, Status: AccountAvatarLoading})
	l.mu.Unlock()

	go func() {
		bitmap, err := l.fetch(url)
		l.mu.Lock()
		l.loading--
		if l.disposed || l.generation != generation {
			// 官方同一位置：`this.image !== image` → 丢弃（:68）
			l.mu.Unlock()
			return
		}
		if err != nil {
			l.publishLocked(&AccountAvatarSnapshot{URL: url, Status: AccountAvatarFailed})
		} else {
			l.publishLocked(&AccountAvatarSnapshot{URL: url, Status: AccountAvatarReady, Bitmap: bitmap})
		}
		l.mu.Unlock()
	}()
}

// fetch 取图并解码成 viva 的 gfx.Bitmap（BGRA 像素，Alpha 在前）。
//
// 为什么先解码再交位图：viva 的 `ui.Image` 只吃 `gfx.Bitmap`
// （viva@v0.6.4/ui/elements.go:53 / gfx.Bitmap{Pix,Width,Height,Stride}），
// 原样喂 png/jpeg 字节什么都不会发生。
func (l *accountAvatarLoader) fetch(url string) (gfx.Bitmap, error) {
	scheme := strings.ToLower(url)
	// 只走 http(s)：file:// 之类会把"远端图"变成"读本地磁盘"，官方浏览器
	// 侧也一样拒绝（<img src> 到 file 只在同源 file 页面里允许）。
	if !strings.HasPrefix(scheme, "http://") && !strings.HasPrefix(scheme, "https://") {
		return gfx.Bitmap{}, errAvatarScheme
	}
	client := l.client
	if client == nil {
		client = DefaultAvatarHTTPClient
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return gfx.Bitmap{}, err
	}
	// 官方 `referrerPolicy = 'no-referrer'`：不带 Referer。
	request.Header.Del("Referer")
	response, err := client.Do(request)
	if err != nil {
		return gfx.Bitmap{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return gfx.Bitmap{}, errAvatarStatus
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, AccountAvatarMaxBytes+1))
	if err != nil {
		return gfx.Bitmap{}, err
	}
	if len(body) > AccountAvatarMaxBytes {
		return gfx.Bitmap{}, errAvatarTooLarge
	}
	return DecodeAccountAvatar(body)
}

// DecodeAccountAvatar 把图片字节解码成 viva 位图（导出版，测试直接用）。
//
// 只接受 image/png、image/jpeg、image/gif（浏览器 `<img>` 对这三类都能解）；
// 不认识的格式**如实失败**，返回零位图。
//
// 实现在 `internal/imgdecode`：解码格式集合与 BGRA 像素布局是**同一个事实**，
// 消息区图片面（`#18`）用的是同一份，不许各写一份再慢慢漂。
func DecodeAccountAvatar(body []byte) (gfx.Bitmap, error) {
	bitmap, err := imgdecode.Decode(body)
	if errors.Is(err, imgdecode.ErrEmpty) {
		return gfx.Bitmap{}, errAvatarEmpty
	}
	return bitmap, err
}

// BitmapFromImage 把 `image.Image` 转成 viva 的 BGRA 位图。
//
// gfx.Bitmap 的 Pix 顺序是 B,G,R,A（见 viva gfx/bitmap_test.go 的夹具：
// `0,0,255,255` 是纯蓝像素），Stride = Width*4。
func BitmapFromImage(source image.Image) (gfx.Bitmap, error) {
	bitmap, err := imgdecode.Bitmap(source)
	if errors.Is(err, imgdecode.ErrEmpty) {
		return gfx.Bitmap{}, errAvatarEmpty
	}
	return bitmap, err
}

// detachLocked 是官方 `detach`（account-avatar.ts:84-91）：释放位图、掐断回调。
func (l *accountAvatarLoader) detachLocked() {
	l.generation++ // 相当于官方 `image.onload = null; image.onerror = null`
}

// publishLocked 发布一帧并请壳重画（官方 `publish` → 快照 listener）。
func (l *accountAvatarLoader) publishLocked(snapshot *AccountAvatarSnapshot) {
	l.snapshot = snapshot
	if snapshot != nil {
		l.url = snapshot.URL
	} else {
		l.url = ""
	}
	publish := l.publish
	invalidate := l.invalidate
	if snapshot == nil {
		if publish != nil {
			publish(AccountAvatarSnapshot{})
		}
	} else if publish != nil {
		publish(*snapshot)
	}
	if invalidate != nil {
		invalidate()
	}
}

// 头像加载的失败原因（只用于日志与测试断言，界面一律显示官方兜底图标）。
var (
	// errAvatarScheme 是"不是 http(s)"——不读本地磁盘。
	errAvatarScheme = errors.New("avatar: 只接受 http/https")
	// errAvatarStatus 是 HTTP 非 2xx（官方 onerror）。
	errAvatarStatus = errors.New("avatar: 非 2xx 响应")
	// errAvatarTooLarge 是超过 AccountAvatarMaxBytes。
	errAvatarTooLarge = errors.New("avatar: 超过体积上限")
	// errAvatarEmpty 是解码后 0×0。
	errAvatarEmpty = errors.New("avatar: 空位图")
)
