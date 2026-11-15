// url.go —— 地址栏的 HTTP(S) 白名单解析。
//
// # 逐条照着官方搬
//
//	OFF:dsh-client-ui-sidebar-browser/src/client/browser/url.ts:1-45
//
// 官方是一个 45 行的纯函数 `parseBrowserAddress(input, applicationOrigin)`，
// 没有任何宿主方法、没有任何载具依赖。这里搬的就是它，判据一条不少：
//
//	src/client/browser/url.ts:5    MAX_BROWSER_URL_LENGTH = 16 * 1024（地址长度上限）
//	src/client/browser/url.ts:24   trim 之后空串                       → 'empty'
//	src/client/browser/url.ts:25   超过 16384                          → 'invalid'
//	src/client/browser/url.ts:26   显式 scheme 判据（见 hasExplicitScheme） → 没有就补 https://
//	src/client/browser/url.ts:28   new URL() 抛错                      → 'invalid'
//	src/client/browser/url.ts:29   用户名或密码非空                    → 'credentials'
//	src/client/browser/url.ts:30-37 http:/https: 之外                  → 'protocol'
//	src/client/browser/url.ts:32-34 等于应用自身 origin               → 'application-origin'
//	src/client/browser/url.ts:40   title = url.hostname
//
// # 与官方**故意不同**的一处（如实登记）
//
// 官方用浏览器的 `URL` 类做规范化（dot-segment 消解 `a/b/../c` → `a/c`、
// 百分号编码、`?` 前空 query 的归一）。Go 的 `net/url` 没有这套算法，这里
// 只补了三条**在地址栏里真会看见**的：
//
//	① 空 path 补 "/"（官方 https://a → href "https://a/"）
//	② host 小写（官方 URL 一律小写域名）
//	③ 默认端口去掉（https:443 / http:80）
//
// dot-segment 与百分号编码**不补**：补一半比不补更容易让人以为"和官方一样"。
// 影响面只有"地址栏里显示成什么样"，不影响白名单判定（判定只看 scheme /
// 凭据 / origin），所以不构成安全差异。这条差异写在
// docs/official-parity.md 的 #50 行里。
package dsbrowser

import (
	"net/url"
	"strings"
)

// maxBrowserURLLength 界住持久化的导航状态（官方同值）。
const maxBrowserURLLength = 16 * 1024

// targetKind 是官方 `BrowserTarget.kind`（url.ts:9-11）。
type targetKind uint8

const (
	kindHTTPS targetKind = iota
	kindHTTP
)

// browserTarget 是官方 `BrowserTarget`（url.ts:9-11）。
type browserTarget struct {
	Kind  targetKind
	URL   string
	Title string
}

// addressFailure 是官方 `BrowserAddressFailure`（url.ts:14）。
type addressFailure string

const (
	failEmpty             addressFailure = "empty"
	failInvalid           addressFailure = "invalid"
	failProtocol          addressFailure = "protocol"
	failCredentials       addressFailure = "credentials"
	failApplicationOrigin addressFailure = "application-origin"
)

// addressResult 是官方 `BrowserAddressResult`（url.ts:17-19）。
type addressResult struct {
	OK     bool
	Target browserTarget
	Reason addressFailure
}

// parseBrowserAddress 把一个地址栏输入解析成固定协议白名单里的目标。
//
// applicationOrigin 是"应用自己的 origin"：官方传 `window.location.origin`，
// 并拒绝在嵌入浏览器里打开应用自身（url.ts:32-34）。**本仓是原生应用，
// 没有 document origin**，所以调用方一律传空串；参数留着是为了让这条判据
// 有可测的入口（测试直接喂它），不是死参数。
func parseBrowserAddress(input, applicationOrigin string) addressResult {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return addressResult{Reason: failEmpty}
	}
	if len(trimmed) > maxBrowserURLLength {
		return addressResult{Reason: failInvalid}
	}
	candidate := trimmed
	if !hasExplicitScheme(trimmed) {
		candidate = "https://" + trimmed
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return addressResult{Reason: failInvalid}
	}
	if parsed.User != nil {
		password, hasPassword := parsed.User.Password()
		if parsed.User.Username() != "" || (hasPassword && password != "") {
			return addressResult{Reason: failCredentials}
		}
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && scheme != "http" {
		return addressResult{Reason: failProtocol}
	}
	host := normalizeHost(parsed, scheme)
	if applicationOrigin != "" && applicationOrigin != "null" {
		if origin, err := url.Parse(applicationOrigin); err == nil {
			if scheme == strings.ToLower(origin.Scheme) && host == strings.ToLower(origin.Host) {
				return addressResult{Reason: failApplicationOrigin}
			}
		}
	}
	href := buildHref(scheme, host, parsed)
	if href == "" {
		return addressResult{Reason: failInvalid}
	}
	kind := kindHTTPS
	if scheme == "http" {
		kind = kindHTTP
	}
	return addressResult{OK: true, Target: browserTarget{
		Kind: kind, URL: href, Title: titleHost(parsed),
	}}
}

// hasExplicitScheme 逐字实现官方 url.ts:26 的正则
//
//	/^[A-Za-z][A-Za-z\d+.-]*:(?!\d+(?:[/?#]|$))/
//
// Go 的 RE2 **没有负向前瞻**，所以拆成两步：先匹配 scheme 前缀，再看冒号后面
// 是不是"纯数字 + ( / ? # 或行尾)"——那是 `host:8080/path` 的形状，不是 scheme。
func hasExplicitScheme(value string) bool {
	if value == "" || !isSchemeStart(value[0]) {
		return false
	}
	index := 1
	for index < len(value) && isSchemeRest(value[index]) {
		index++
	}
	if index >= len(value) || value[index] != ':' {
		return false
	}
	rest := value[index+1:]
	digits := 0
	for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return true
	}
	if digits == len(rest) {
		return false
	}
	switch rest[digits] {
	case '/', '?', '#':
		return false
	}
	return true
}

func isSchemeStart(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

func isSchemeRest(b byte) bool {
	return isSchemeStart(b) || b >= '0' && b <= '9' || b == '+' || b == '.' || b == '-'
}

// normalizeHost 返回 `scheme://` 之后那一段（含端口，默认端口已去掉、域名已小写）。
func normalizeHost(parsed *url.URL, scheme string) string {
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if scheme == "https" && port == "443" || scheme == "http" && port == "80" {
		port = ""
	}
	if port != "" {
		return host + ":" + port
	}
	return host
}

// titleHost 是官方 `title`：`url.hostname`（**不含端口**，IPv6 保留方括号）。
func titleHost(parsed *url.URL) string {
	host := strings.ToLower(parsed.Hostname())
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

// buildHref 拼出官方 `url.href` 的形状。
func buildHref(scheme, host string, parsed *url.URL) string {
	if host == "" {
		return ""
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	var builder strings.Builder
	builder.WriteString(scheme)
	builder.WriteString("://")
	builder.WriteString(host)
	builder.WriteString(path)
	if parsed.ForceQuery || parsed.RawQuery != "" {
		builder.WriteByte('?')
		builder.WriteString(parsed.RawQuery)
	}
	if parsed.Fragment != "" {
		builder.WriteByte('#')
		builder.WriteString(parsed.EscapedFragment())
	}
	return builder.String()
}

// sameTarget 判断两次导航是不是同一个地址（官方 `loadUrl` 对同一地址走 reload，
// url.ts 的调用方 `IframeImpl.loadUrl`：`current?.url === target.url ? reload : navigate`）。
func sameTarget(a, b browserTarget) bool { return a.URL == b.URL }
