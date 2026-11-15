package dsbrowser

import "testing"

// TestParseBrowserAddressFollowsTheOfficialAllowlist 逐条对着
// OFF:dsh-client-ui-sidebar-browser/src/client/browser/url.ts:22-44 的分支表。
//
// 每条都标了它在官方那边是哪一行；改判据时这张表会先红。
func TestParseBrowserAddressFollowsTheOfficialAllowlist(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		origin string
		ok     bool
		reason addressFailure
		href   string
		title  string
		kind   targetKind
		anchor string // 官方 url.ts 的行号，写进失败信息里
	}{
		// :24 空串（只含空白也算空）
		{name: "空串", input: "", reason: failEmpty, anchor: "url.ts:24"},
		{name: "纯空白", input: "   \t ", reason: failEmpty, anchor: "url.ts:24"},
		// :26 没有显式 scheme 就补 https://
		{name: "裸域名补 https", input: "example.com", ok: true,
			href: "https://example.com/", title: "example.com", kind: kindHTTPS, anchor: "url.ts:26"},
		{name: "裸域名带路径", input: "example.com/a/b", ok: true,
			href: "https://example.com/a/b", title: "example.com", kind: kindHTTPS, anchor: "url.ts:26"},
		// 显式 scheme 保留
		{name: "显式 http", input: "http://example.com/x", ok: true,
			href: "http://example.com/x", title: "example.com", kind: kindHTTP, anchor: "url.ts:30"},
		{name: "显式 https 大写", input: "HTTPS://Example.COM/A", ok: true,
			href: "https://example.com/A", title: "example.com", kind: kindHTTPS, anchor: "url.ts:30"},
		// :26 的负向前瞻：`host:8080/path` 不是 scheme
		{name: "端口不是 scheme", input: "example.com:8080/a", ok: true,
			href: "https://example.com:8080/a", title: "example.com", kind: kindHTTPS, anchor: "url.ts:26"},
		{name: "端口结尾不是 scheme", input: "example.com:8080", ok: true,
			href: "https://example.com:8080/", title: "example.com", kind: kindHTTPS, anchor: "url.ts:26"},
		// 默认端口去掉（Go 的 net/url 不做这件事，是本仓补的三条之一）
		{name: "https 默认端口去掉", input: "https://example.com:443/x", ok: true,
			href: "https://example.com/x", title: "example.com", kind: kindHTTPS, anchor: "url.ts:26"},
		{name: "http 默认端口去掉", input: "http://example.com:80/", ok: true,
			href: "http://example.com/", title: "example.com", kind: kindHTTP, anchor: "url.ts:26"},
		// :30-37 只认 http/https
		{name: "file 被拒", input: "file:///etc/passwd", reason: failProtocol, anchor: "url.ts:37"},
		{name: "javascript 被拒", input: "javascript:alert(1)", reason: failProtocol, anchor: "url.ts:37"},
		{name: "data 被拒", input: "data:text/html,x", reason: failProtocol, anchor: "url.ts:37"},
		{name: "ftp 被拒", input: "ftp://example.com/", reason: failProtocol, anchor: "url.ts:37"},
		// :29 凭据
		{name: "用户名被拒", input: "https://user@example.com/", reason: failCredentials, anchor: "url.ts:29"},
		{name: "口令被拒", input: "https://user:secret@example.com/", reason: failCredentials, anchor: "url.ts:29"},
		// :28 解析不出
		{name: "只有 scheme", input: "https://", reason: failInvalid, anchor: "url.ts:28"},
		{name: "非法方括号", input: "https://[::1/", reason: failInvalid, anchor: "url.ts:28"},
		// :32-34 应用自身 origin
		{name: "应用自身被拒", input: "https://app.example/", origin: "https://app.example",
			reason: failApplicationOrigin, anchor: "url.ts:32"},
		{name: "同主机不同端口可开", input: "https://app.example:8443/", origin: "https://app.example",
			ok: true, href: "https://app.example:8443/", title: "app.example", kind: kindHTTPS, anchor: "url.ts:32"},
		{name: "origin 是 null 时不拦", input: "https://app.example/", origin: "null",
			ok: true, href: "https://app.example/", title: "app.example", kind: kindHTTPS, anchor: "url.ts:33"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseBrowserAddress(c.input, c.origin)
			if got.OK != c.ok {
				t.Fatalf("%s: OK=%v want %v (%+v)", c.anchor, got.OK, c.ok, got)
			}
			if !c.ok {
				if got.Reason != c.reason {
					t.Fatalf("%s: reason=%q want %q", c.anchor, got.Reason, c.reason)
				}
				return
			}
			if got.Target.URL != c.href {
				t.Fatalf("%s: href=%q want %q", c.anchor, got.Target.URL, c.href)
			}
			if got.Target.Title != c.title {
				t.Fatalf("%s: title=%q want %q", c.anchor, got.Target.Title, c.title)
			}
			if got.Target.Kind != c.kind {
				t.Fatalf("%s: kind=%v want %v", c.anchor, got.Target.Kind, c.kind)
			}
		})
	}
}

// TestParseBrowserAddressBoundsTheLength 是 url.ts:5 的 16 KiB 上限。
//
// 为什么它重要：地址会被持久化成导航状态，没有上限就是"用户粘一份 10MB 文本
// 进去，状态与日志一起被撑爆"。
func TestParseBrowserAddressBoundsTheLength(t *testing.T) {
	// 刚好在上限内：长度 16384 的 https 地址仍然接受。
	pad := make([]byte, 0, maxBrowserURLLength)
	pad = append(pad, []byte("https://example.com/")...)
	for len(pad) < maxBrowserURLLength {
		pad = append(pad, 'a')
	}
	if got := parseBrowserAddress(string(pad), ""); !got.OK {
		t.Fatalf("恰好 %d 字节应当接受，却得到 %q", maxBrowserURLLength, got.Reason)
	}
	// 超一个字节就拒。
	over := append(pad, 'a')
	if got := parseBrowserAddress(string(over), ""); got.OK || got.Reason != failInvalid {
		t.Fatalf("超长地址应当 %q，却得到 OK=%v reason=%q", failInvalid, got.OK, got.Reason)
	}
}

// TestHasExplicitSchemeMatchesTheOfficialRegex 单独钉 url.ts:26 的正则
// `/^[A-Za-z][A-Za-z\d+.-]*:(?!\d+(?:[/?#]|$))/` —— Go 的 RE2 没有负向前瞻，
// 所以这是手写的，必须单独证明它与正则同判。
func TestHasExplicitSchemeMatchesTheOfficialRegex(t *testing.T) {
	cases := map[string]bool{
		"https://a":       true,
		"http://a":        true,
		"file:///x":       true,
		"mailto:a":        true,
		"a+b-c.d:":        true,
		"example.com":     false,
		"example.com:80":  false, // 冒号后是纯数字直到结尾 → 不是 scheme
		"example.com:80/": false, // 冒号后是数字 + "/" → 不是 scheme
		"a:1?x":           false, // 数字 + "?" → 不是 scheme
		"a:1#x":           false, // 数字 + "#" → 不是 scheme
		"a:1x":            true,  // 数字后面是别的字符 → 仍是 scheme
		"1abc:":           false, // 首字符必须是字母
		":":               false,
		"":                false,
	}
	for input, want := range cases {
		if got := hasExplicitScheme(input); got != want {
			t.Fatalf("hasExplicitScheme(%q)=%v want %v", input, got, want)
		}
	}
}
