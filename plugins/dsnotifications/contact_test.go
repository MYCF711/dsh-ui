package dsnotifications

import (
	"errors"
	"math"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// contactBase 是官方默认问卷地址（contact-config.ts:13）拼上四个字段的 hide 之后的样子。
const contactBase = "https://trtgsjkv6r.feishu.cn/share/base/form/shrcnlCoGElW7MQznGy9r3YYXcg"

// TestContactURLIsTheOfficialQuery 逐字节钉住 URL：字段顺序、hide/prefill 配对、
// 值走 WHATWG 表单序列化（空格→`+`、`=`→`%3D`、`;`→`%3B`）。
//
// 变异点：改默认地址 / 少写一个 hide_ / 把 source 的 hide 漏掉 / 把 prefill 写成
// 未编码的值 / 把键序换成 Go 的 url.Values.Encode()（排序）→ 全红。
func TestContactURLIsTheOfficialQuery(t *testing.T) {
	got := contactURL(contactFormURL, contactContext{
		UID:        "u-42",
		Version:    "9.8.7-go.1",
		DeviceInfo: "platform=win32; os=10.0.22631; app_arch=x64",
	})
	want := contactBase + "?hide_uid=1&hide_device_info=1&hide_harness_version=1" +
		"&prefill_uid=u-42&hide_source=1&prefill_harness_version=9.8.7-go.1" +
		"&prefill_device_info=platform%3Dwin32%3B+os%3D10.0.22631%3B+app_arch%3Dx64"
	if got != want {
		t.Fatalf("问卷 URL 与官方不一致\n got = %s\nwant = %s", got, want)
	}
}

// TestContactURLAlwaysHidesEveryContextField 覆盖官方循环的「先 hide、再考虑 prefill」：
// 四个字段一个都不能漏，空值仍然要 hide（`contactSource` 默认就是空串）。
func TestContactURLAlwaysHidesEveryContextField(t *testing.T) {
	got := contactURL(contactFormURL, contactContext{})
	want := contactBase + "?hide_uid=1&hide_device_info=1&hide_harness_version=1&hide_source=1"
	if got != want {
		t.Fatalf("空上下文 URL = %s", got)
	}
	for _, field := range []string{"uid", "source", "harness_version", "device_info"} {
		if !strings.Contains(got, "hide_"+field+"=1") {
			t.Fatalf("缺 hide_%s：%s", field, got)
		}
		if strings.Contains(got, "prefill_"+field) {
			t.Fatalf("空值不该预填 prefill_%s：%s", field, got)
		}
	}
}

// TestContactURLPrefillsOnlyPresentFields 钉住「有值才预填」，且键序是**出现顺序**
// （uid 的 prefill 在 source 的 hide 之前——这是官方 URLSearchParams 的插入序）。
func TestContactURLPrefillsOnlyPresentFields(t *testing.T) {
	got := contactURL(contactFormURL, contactContext{UID: "u-42"})
	want := contactBase + "?hide_uid=1&hide_device_info=1&hide_harness_version=1&prefill_uid=u-42&hide_source=1"
	if got != want {
		t.Fatalf("只带 uid 的 URL = %s", got)
	}
}

// TestContactURLDropsUnsupportedConfiguredFields 覆盖官方 contact-url.ts:35 那段：
// 配置地址里带了不支持的字段（prefill_/hide_ 两半）就必须删掉。
//
// 默认地址里没有这些键，所以这条只能靠「可换地址」的入参验证（官方那个函数也是收 config 的）。
func TestContactURLDropsUnsupportedConfiguredFields(t *testing.T) {
	configured := contactBase + "?hide_uid=1&prefill_os_version=10&prefill_uid=stale&hide_screen_resolution=1&prefill_device_model=x"
	got := contactURL(configured, contactContext{})
	for _, banned := range contactUnsupportedFields {
		if strings.Contains(got, banned+"=") {
			t.Fatalf("不支持的字段 %s 没被剔除：%s", banned, got)
		}
	}
	if strings.Contains(got, "prefill_uid") {
		t.Fatalf("空 uid 时配置里的陈旧 prefill_uid 没被删掉：%s", got)
	}
	if !strings.Contains(got, "hide_uid=1") {
		t.Fatalf("hide_uid 被误删：%s", got)
	}
}

// TestContactURLScreenResolutionIsPhysical 覆盖官方 `width*ratio` 那段：
// 只有宽高都 > 0 才写 screen_resolution，且是「CSS 宽 × 像素比」取整后的物理像素。
func TestContactURLScreenResolutionIsPhysical(t *testing.T) {
	got := contactURL(contactFormURL, contactContext{
		DeviceInfo: "platform=win32", Width: 1280, Height: 720, PixelRatio: 1.5,
	})
	if !strings.Contains(got, "prefill_device_info=platform%3Dwin32%3B+screen_resolution%3D1920x1080") {
		t.Fatalf("screen_resolution 不是 1280×1.5 / 720×1.5：%s", got)
	}
	// 没有浏览器屏幕度量时（本仓真实情况）不写这一段：官方判据就是宽高都 > 0。
	none := contactURL(contactFormURL, contactContext{})
	if strings.Contains(none, "screen_resolution") || strings.Contains(none, "prefill_device_info") {
		t.Fatalf("没有屏幕度量却写了 device_info：%s", none)
	}
	// pixelRatio 非有限值回退 1（官方 `Number.isFinite(...) ? ... : 1`）。
	// 注意：0 是**有限值**，官方会真的按 0 乘（宽变 0 → 不写 screen_resolution），
	// 这不是「缺失」，所以这里用 NaN 才测得到那条回退分支。
	fallback := contactURL(contactFormURL, contactContext{Width: 800, Height: 600, PixelRatio: math.NaN()})
	if !strings.Contains(fallback, "screen_resolution%3D800x600") {
		t.Fatalf("像素比非有限值时没回退 1：%s", fallback)
	}
	zero := contactURL(contactFormURL, contactContext{Width: 800, Height: 600, PixelRatio: 0})
	if strings.Contains(zero, "screen_resolution") {
		t.Fatalf("像素比 0 时官方会算出 0 宽，不该写 screen_resolution：%s", zero)
	}
}

// TestContactURLRefusesNonHTTPSForm 覆盖「只发 https」这条：配置坏了就交空串。
func TestContactURLRefusesNonHTTPSForm(t *testing.T) {
	for _, form := range []string{"", "http://example.com/form", "javascript:alert(1)", "/relative"} {
		if got := contactURL(form, contactContext{UID: "u"}); got != "" {
			t.Fatalf("非法问卷地址 %q 仍拼出 URL：%s", form, got)
		}
	}
}

// TestContactQueryEscapeMatchesURLSearchParams 钉住编码器与 JS 的分歧点：
// `*` 不转义、`~` 转成 `%7E`（Go 的 url.QueryEscape 恰好相反），空格是 `+`。
func TestContactQueryEscapeMatchesURLSearchParams(t *testing.T) {
	cases := map[string]string{
		"platform=win32; os=10.0": "platform%3Dwin32%3B+os%3D10.0",
		"a*b":                     "a*b",
		"a~b":                     "a%7Eb",
		"a+b":                     "a%2Bb",
		"100%":                    "100%25",
		"-_.~":                    "-_.%7E",
		"中":                       "%E4%B8%AD",
		"a/b":                     "a%2Fb",
	}
	for in, want := range cases {
		if got := contactQueryEscape(in); got != want {
			t.Fatalf("转义 %q = %q，官方应为 %q", in, got, want)
		}
	}
}

// TestContactMenuRowIsTheOfficialCopyAndOrder 钉住文案与位置：
// 官方 locales.ts:52 的 `contactUs: '意见反馈'`，顺序 settings → contact → (登录|退出)。
func TestContactMenuRowIsTheOfficialCopyAndOrder(t *testing.T) {
	signedOut := accountMenuEntries(AccountState{SignedIn: false}, false, true)
	wantSignedOut := []string{"settings", "contact", "signin"}
	if got := entryIDs(signedOut); strings.Join(got, ",") != strings.Join(wantSignedOut, ",") {
		t.Fatalf("未登录菜单顺序 = %v", got)
	}
	if signedOut[1].label != "意见反馈" {
		t.Fatalf("contact 行文案 = %q（官方 zh 是「意见反馈」）", signedOut[1].label)
	}
	signedIn := accountMenuEntries(AccountState{SignedIn: true}, false, true)
	if got := entryIDs(signedIn); strings.Join(got, ",") != "settings,contact,signout" {
		t.Fatalf("已登录菜单顺序 = %v", got)
	}
	// 壳没接 OpenExternal 时一行都不画（画一行点了没反应的菜单比不画更坏）。
	for _, entries := range [][]accountMenuEntry{
		accountMenuEntries(AccountState{}, false, false),
		accountMenuEntries(AccountState{SignedIn: true}, false, false),
	} {
		for _, entry := range entries {
			if entry.id == "contact" {
				t.Fatalf("没接 OpenExternal 却画了反馈行：%+v", entries)
			}
		}
	}
	// addApiKey 仍未接线，继续不画。
	for _, entry := range accountMenuEntries(AccountState{}, false, true) {
		if entry.label == "添加 API Key" {
			t.Fatalf("未接线的动作不该画：%+v", entry)
		}
	}
}

// TestContactMenuHandsTheOfficialURLToOpenExternal 是「对外跳转」那条的验收：
// 点击必须把**官方形状的 URL** 交给壳的 OpenExternal，且测试里用替身接住——
// 真去开浏览器会在这个测试里被看见（替身是唯一出口）。
func TestContactMenuHandsTheOfficialURLToOpenExternal(t *testing.T) {
	p := newTestPlugin(nil)
	p.deviceInfo = func() string { return "platform=win32; os=10.0.22631; app_arch=x64" }
	var opened []string
	env := &pluginapi.Env{Act: pluginapi.Actions{
		OpenExternal: func(destination string) error {
			opened = append(opened, destination)
			return nil
		},
		AccountSnapshot: func() apiclient.AccountSnapshot {
			id := "u-42"
			return apiclient.AccountSnapshot{Profile: &apiclient.AccountProfile{
				Status: "ready", Value: &apiclient.AccountProfileValue{ID: &id},
			}}
		},
		SettingsHostFacts: func() pluginapi.SettingsHostFacts {
			return pluginapi.SettingsHostFacts{ClientVersion: "9.8.7-go.1"}
		},
	}}
	p.chooseAccountMenu("contact", env)
	if len(opened) != 1 {
		t.Fatalf("OpenExternal 调用次数 = %d（要恰好一次）", len(opened))
	}
	want := contactBase + "?hide_uid=1&hide_device_info=1&hide_harness_version=1" +
		"&prefill_uid=u-42&hide_source=1&prefill_harness_version=9.8.7-go.1" +
		"&prefill_device_info=platform%3Dwin32%3B+os%3D10.0.22631%3B+app_arch%3Dx64"
	if opened[0] != want {
		t.Fatalf("交出去的 URL 与官方不一致\n got = %s\nwant = %s", opened[0], want)
	}
	// 菜单点完要关（官方 onSelect 先 setOpen(false)）。
	p.accountUIMu.Lock()
	open := p.accountMenuOpen != nil && *p.accountMenuOpen
	p.accountUIMu.Unlock()
	if open {
		t.Fatal("点完反馈菜单没关")
	}
}

// TestContactMenuWithoutBridgeStaysSilent 覆盖三种「没地方开」：
// 空 env、没 OpenExternal、账号/版本都未知。都不许 panic、不许交 URL。
func TestContactMenuWithoutBridgeStaysSilent(t *testing.T) {
	p := newTestPlugin(nil)
	p.deviceInfo = func() string { return "" }
	if destination := p.contactDestination(nil); destination != "" {
		t.Fatalf("空 env 仍拼出 URL：%s", destination)
	}
	if destination := p.contactDestination(&pluginapi.Env{}); destination != "" {
		t.Fatalf("没 OpenExternal 仍拼出 URL：%s", destination)
	}
	var calls int
	env := &pluginapi.Env{Act: pluginapi.Actions{OpenExternal: func(string) error { calls++; return nil }}}
	p.chooseAccountMenu("contact", env)
	if calls != 1 {
		t.Fatalf("接了 OpenExternal 却没交 URL：%d", calls)
	}
	// 资料不是 ready / id 为 null / 壳没给宿主事实 → 三个 prefill 各自缺席，hide 仍在。
	env.Act.AccountSnapshot = func() apiclient.AccountSnapshot {
		return apiclient.AccountSnapshot{Profile: &apiclient.AccountProfile{Status: "failed"}}
	}
	got := p.contactDestination(env)
	if strings.Contains(got, "prefill_uid") || strings.Contains(got, "prefill_harness_version") ||
		strings.Contains(got, "prefill_device_info") {
		t.Fatalf("未知事实被预填：%s", got)
	}
	if !strings.Contains(got, "hide_uid=1&hide_device_info=1&hide_harness_version=1&hide_source=1") {
		t.Fatalf("hide 四件套不全：%s", got)
	}
}

// TestContactOpenFailureIsReported 钉住「打不开要如实提示」，不静默吞错。
func TestContactOpenFailureIsReported(t *testing.T) {
	p := newTestPlugin(nil)
	p.deviceInfo = func() string { return "" }
	var notes []string
	env := &pluginapi.Env{Act: pluginapi.Actions{
		OpenExternal: func(string) error { return errors.New("refused") },
		Notify:       func(msg string) { notes = append(notes, msg) },
	}}
	p.chooseAccountMenu("contact", env)
	if len(notes) != 1 || !strings.Contains(notes[0], "打开链接失败") {
		t.Fatalf("打开失败没有如实提示：%v", notes)
	}
}

// TestReadContactDeviceInfoFollowsOfficialShape 校验真实设备描述：
// 字段顺序必须是官方那五个（platform → os → app_arch → cpu → memory_gib），
// 必填项必须在、缺的项必须整段省略（不是空值占位）。
func TestReadContactDeviceInfoFollowsOfficialShape(t *testing.T) {
	info := readContactDeviceInfo()
	if strings.HasPrefix(info, "platform=") == false {
		t.Fatalf("第一项必须是 platform=：%q", info)
	}
	order := map[string]int{"platform": 0, "os": 1, "app_arch": 2, "cpu": 3, "memory_gib": 4}
	seen := map[string]bool{}
	values := map[string]string{}
	last := -1
	fields := strings.Split(info, "; ")
	for _, field := range fields {
		name, value, found := strings.Cut(field, "=")
		if !found {
			t.Fatalf("字段不是 name=value：%q", field)
		}
		index, known := order[name]
		if !known {
			t.Fatalf("出现官方没有的字段 %q：%s", name, info)
		}
		if seen[name] {
			t.Fatalf("字段 %q 重复：%s", name, info)
		}
		if index <= last {
			t.Fatalf("字段顺序不符（%q 在 %d）：%s", name, last, info)
		}
		if value == "" {
			t.Fatalf("字段 %q 是空值（官方是整段省略）：%s", name, info)
		}
		seen[name] = true
		values[name] = value
		last = index
	}
	if !seen["platform"] || !seen["app_arch"] {
		t.Fatalf("必填项缺失：%s", info)
	}
	if value, ok := values["memory_gib"]; ok {
		if matched, _ := regexp.MatchString(`^\d+\.\d$`, value); !matched {
			t.Fatalf("memory_gib 不是一位小数：%q", value)
		}
	}
	if runtime.GOOS == "windows" {
		if version, ok := values["os"]; ok {
			if matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+$`, version); !matched {
				t.Fatalf("os 不是 major.minor.build：%q", version)
			}
		}
		if values["platform"] != "win32" {
			t.Fatalf("Windows 上 platform 应是 process.platform 的 win32：%q", values["platform"])
		}
	}
}

// TestContactPlatformAndArchNamesMatchProcessNames 钉住两处命名映射：
// Electron 的 process.platform / process.arch 与我们 GOOS/GOARCH 不是同一套字面量。
func TestContactPlatformAndArchNamesMatchProcessNames(t *testing.T) {
	if runtime.GOOS == "windows" && contactPlatformName() != "win32" {
		t.Fatalf("GOOS windows 应映射成 win32，得到 %q", contactPlatformName())
	}
	if runtime.GOARCH == "amd64" && contactArchName() != "x64" {
		t.Fatalf("GOARCH amd64 应映射成 x64，得到 %q", contactArchName())
	}
	if runtime.GOARCH == "386" && contactArchName() != "ia32" {
		t.Fatalf("GOARCH 386 应映射成 ia32，得到 %q", contactArchName())
	}
}

// entryIDs 取菜单项 id 序列（断言顺序用）。
func entryIDs(entries []accountMenuEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.id)
	}
	return out
}
