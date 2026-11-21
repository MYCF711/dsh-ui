// contact.go —— 官方 `contactUs`（AccountMenu 的「意见反馈」）的本仓等价物。
//
// # 官方事实（rc.5 随包 src/**，逐行核对）
//
//	OFF:dsh-client-ui-settings-account/src/client/AccountMenu.tsx:76
//	  菜单第三行之前的固定一行：`{ id: 'contact', label: t('contactUs'),
//	  icon: <IconPaperPlaneOutlineMedium size={16} /> }`；顺序是 settings → contact →
//	  (signout | signin)（:74-78）。点它先 `setOpen(false)` 再 `contactUs()`（:83）。
//	OFF:.../src/client/locales.ts:52
//	  zh 文案逐字：`contactUs: '意见反馈'`。
//	OFF:.../src/client/index.ts:186-214
//	  contactUs 采样 { uid, version, width, height, pixelRatio }（:189-194），
//	  其中 uid = 账号资料 ready 时的 `profile.value.id`，否则 null；version =
//	  `process.env.DSH_CLIENT_VERSION`。设备信息取 `globalThis.dshDesktop?.deviceInfo`：
//	  无桥（纯浏览器）直接用 `navigator.userAgent`（:201-204），有桥就 await 它，
//	  失败给空串（:205-213，原生信息可选，问卷照开）。最后
//	  `window.open(contactUrl(...), '_blank', 'noopener,noreferrer')`。
//	OFF:.../src/client/contact-url.ts:11-40
//	  纯函数：`new URL(config.contactFormUrl)` → 计算设备串 → 四个字段
//	  {uid, source, harness_version, device_info} 各自 `hide_<name>=1` +
//	  （有值时）`prefill_<name>` → 再从配置 URL 里剔除六个不支持的字段。
//	OFF:.../src/contact-config.ts:12-19
//	  配置默认值（逐字，本仓直接采用）：
//	    contactFormUrl = https://trtgsjkv6r.feishu.cn/share/base/form/shrcnlCoGElW7MQznGy9r3YYXcg?hide_uid=1&hide_device_info=1&hide_harness_version=1
//	    contactSource  = ''
//	  官方由宿主经 `__DSH_CONTACT_CONFIG__` 注入覆盖；本仓没有浏览器 bootstrap 全局，
//	  所以只用这两个默认值（不自己发明配置通道）。
//
// # 本仓的边界
//
//   - 「桥」这一半：官方桌面端 `deviceInfo()` 是 Electron 主进程的原生实现
//     （`Desktop readDeviceInfo()`：platform/os/app_arch/cpu/memory_gib 的
//     `name=value` 串，`; ` 分隔，缺的字段直接省掉）。本仓是原生 Go 应用，
//     等价物就是 `readContactDeviceInfo()`（`contact_device_*.go`）——**同一格式**、
//     同一来源类别，不是编一个 userAgent。
//   - 屏幕三件套（`screen.width/height` 与 `devicePixelRatio`）是**浏览器**度量，
//     原生应用没有对应读口：传零，于是 `screen_resolution` 不出现（官方自己的判据
//     就是 `width > 0 && height > 0` 才写，contact-url.ts:24）。
//   - 「打开」这一半：官方在浏览器里 `window.open`；本仓用现成的
//     `Env.Act.OpenExternal`（只开明确的 http/https，见 pluginapi/plugin.go:1727）。
//
// # 为什么 URL 是手拼而不是 `url.Values.Encode()`
//
// 官方那串是 `URLSearchParams.set()` 的产物，语义与 Go 的 `url.Values` 有三处不同：
// ① 键序是**插入序**（已存在的键原地更新、新键追加），`url.Values.Encode()` 排序；
// ② 空格序列化成 `+`（两者一致），但 `*` 不转义、`~` 转义成 `%7E`（Go 的
// `QueryEscape` 恰好相反：`~` 不转义、`*` 转义）；
// ③ `set` 会删掉重复键。所以这里按官方语义实现一条极小的有序查询串（`contactQuery`），
// 编码器逐字节照 WHATWG 的 application/x-www-form-urlencoded 序列化器。
package dsnotifications

import (
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

const (
	// contactFormURL 是官方 contact-config.ts:13 的默认问卷地址（**逐字**，125 字符）。
	//
	// 它自己带的三个 `hide_*` 是有意留着的：官方默认配置就是这么发的，
	// 本仓不重排、不删减（下面的循环只会把同名键原地刷新成 `hide_…=1`）。
	contactFormURL = "https://trtgsjkv6r.feishu.cn/share/base/form/shrcnlCoGElW7MQznGy9r3YYXcg?hide_uid=1&hide_device_info=1&hide_harness_version=1"
	// contactSource 是官方 contact-config.ts:14 的默认值（空串 = 问卷还不认 Harness 这个来源）。
	// 空值仍然写 `hide_source=1`（官方循环对每个字段都先 hide 再考虑 prefill）。
	contactSource = ""
	// zhAccountContact 是官方 locales.ts:52 的 `contactUs: '意见反馈'`（逐字）。
	zhAccountContact = "意见反馈"
)

// contactUnsupportedFields 是官方 contact-url.ts:35 明令从配置 URL 里剔除的字段
// （prefill_ 与 hide_ 两半都删）。默认地址里一个都没有，但配置通道理论上能带进来。
var contactUnsupportedFields = []string{
	"os_version", "device_brand", "app_locale", "screen_resolution", "app_version", "device_model",
}

// contactContext 是官方 contactUs 采样、再交给 contactUrl 的上下文（index.ts:189-194）。
type contactContext struct {
	// UID 是账号资料 ready 时的 Platform 用户 id；未登录 / 资料未知 = ""（官方 null）。
	UID string
	// Version 是客户端版本（官方 `process.env.DSH_CLIENT_VERSION`）。
	Version string
	// DeviceInfo 是设备描述（官方桌面端 `dshDesktop.deviceInfo()`，无桥时 userAgent）。
	DeviceInfo string
	// Width / Height / PixelRatio 是浏览器 screen 三件套；本仓无浏览器度量，传零。
	Width      float64
	Height     float64
	PixelRatio float64
}

// contactURL 逐条实现官方 contact-url.ts:11-40。
//
// 入参是「配置里的问卷地址」而不是常量，因为官方这个函数就是收 config 的
// （`contactUrl(config, context)`），剔除不支持字段那段只有能换地址才可验证。
func contactURL(formURL string, context contactContext) string {
	base, err := url.Parse(formURL)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		// 常量写错（编译期不可能）/ 配置非法：交空串，绝不交一条坏链接。
		return ""
	}
	query := parseContactQuery(base.RawQuery)
	// `Number.isFinite(pixelRatio) ? pixelRatio : 1`（contact-url.ts:20）。
	ratio := 1.0
	if !math.IsNaN(context.PixelRatio) && !math.IsInf(context.PixelRatio, 0) {
		ratio = context.PixelRatio
	}
	deviceInfo := make([]string, 0, 2)
	if context.DeviceInfo != "" {
		deviceInfo = append(deviceInfo, context.DeviceInfo)
	}
	// `if (width > 0 && height > 0) deviceInfo.push(...)`（:24）——NaN 比较恒假，同效。
	width := math.Round(context.Width * ratio)
	height := math.Round(context.Height * ratio)
	if width > 0 && height > 0 {
		deviceInfo = append(deviceInfo, "screen_resolution="+jsInteger(width)+"x"+jsInteger(height))
	}
	fields := [][2]string{
		{"uid", context.UID},
		{"source", contactSource},
		{"harness_version", context.Version},
		{"device_info", strings.Join(deviceInfo, "; ")}, // `filter(Boolean).join('; ')`（:27）
	}
	for _, field := range fields {
		name, value := field[0], field[1]
		query.set("hide_"+name, "1") // 官方先把每个字段都设为隐藏（:30）
		query.del("prefill_" + name) // 再清掉配置里可能带的同名 prefill（:31）
		if value != "" {
			query.set("prefill_"+name, value) // 有值才预填（:32）
		}
	}
	for _, name := range contactUnsupportedFields {
		query.del("prefill_" + name)
		query.del("hide_" + name)
	}
	base.RawQuery = query.encode()
	return base.String()
}

// openContactUs 是官方 `contactUs` 的动作面：采样 → 拼 URL → 交给系统浏览器。
//
// 官方没有错误面（`window.open` 被拦截也只是没开）；本仓 `OpenExternal` 会返回错误，
// 按 pluginapi 的约定「打不开要如实提示」，不静默吞掉。
func (p *Plugin) openContactUs(env *pluginapi.Env) {
	destination := p.contactDestination(env)
	if destination == "" {
		return
	}
	if err := env.Act.OpenExternal(destination); err != nil && env.Act.Notify != nil {
		env.Act.Notify("打开链接失败: " + err.Error())
	}
}

// contactDestination 采样 uid / 版本 / 设备信息并拼出问卷 URL；无法打开时回 ""。
//
// 官方先把账号与构建信息采好再 await 设备信息（index.ts:187-189 的注释说明：
// 等原生信息期间读到的 profile 不能替换这次点击报的 uid）。本仓设备信息是同步读，
// 顺序天然一致，但仍按「先 uid、后设备」写，保持与官方同样的语义边界。
func (p *Plugin) contactDestination(env *pluginapi.Env) string {
	if env == nil || env.Act.OpenExternal == nil {
		return ""
	}
	uid := ""
	if snapshot := p.accountSnapshot(env); snapshot.Profile != nil &&
		snapshot.Profile.Status == "ready" && snapshot.Profile.Value != nil &&
		snapshot.Profile.Value.ID != nil {
		uid = *snapshot.Profile.Value.ID
	}
	// 官方 `process.env.DSH_CLIENT_VERSION` 就是 build 注入的版本号；
	// 本仓同值从壳的宿主事实里取（cmd/app/settings_panel_wiring.go 的 ClientVersion）。
	version := ""
	if env.Act.SettingsHostFacts != nil {
		version = env.Act.SettingsHostFacts().ClientVersion
	}
	return contactURL(contactFormURL, contactContext{
		UID: uid, Version: version, DeviceInfo: p.contactDevice(),
	})
}

// contactDevice 取设备描述；测试用 `Plugin.deviceInfo` 替身（真实实现见
// contact_device_*.go）。替身是为了让「交出去的 URL」能被逐字断言，
// 同时保证测试不会去碰真实硬件读数。
func (p *Plugin) contactDevice() string {
	if p.deviceInfo != nil {
		return p.deviceInfo()
	}
	return readContactDeviceInfo()
}

// contactQuery 是官方 `URLSearchParams` 的插入序子集：set / delete / encode。
type contactQuery struct {
	params []contactParam
}

// contactParam 是一条 name=value（保留出现顺序）。
type contactParam struct {
	name  string
	value string
}

// parseContactQuery 按出现顺序解析已有查询串（`a=1&b=2`）。
func parseContactQuery(raw string) contactQuery {
	var query contactQuery
	if raw == "" {
		return query
	}
	for _, piece := range strings.Split(raw, "&") {
		if piece == "" {
			continue
		}
		name, value, _ := strings.Cut(piece, "=")
		query.set(contactQueryUnescape(name), contactQueryUnescape(value))
	}
	return query
}

// index 找同名键；不存在回 -1。
func (q *contactQuery) index(name string) int {
	for i := range q.params {
		if q.params[i].name == name {
			return i
		}
	}
	return -1
}

// set 与 URLSearchParams.set 同语义：同名键（含重复）全部删掉，值写到**原位置**，
// 原本没有就追加到末尾。
func (q *contactQuery) set(name, value string) {
	first := q.index(name)
	if first < 0 {
		q.params = append(q.params, contactParam{name: name, value: value})
		return
	}
	q.params[first].value = value
	kept := q.params[:0]
	seen := false
	for _, param := range q.params {
		if param.name == name {
			if seen {
				continue
			}
			seen = true
		}
		kept = append(kept, param)
	}
	q.params = kept
}

// del 与 URLSearchParams.delete 同语义：删掉所有同名键，其余顺序不变。
func (q *contactQuery) del(name string) {
	kept := q.params[:0]
	for _, param := range q.params {
		if param.name == name {
			continue
		}
		kept = append(kept, param)
	}
	q.params = kept
}

// encode 是 URLSearchParams 的序列化：`name=value` 以 `&` 连接，键与值都走
// application/x-www-form-urlencoded 序列化器（空格 → `+`）。
func (q *contactQuery) encode() string {
	var builder strings.Builder
	for i, param := range q.params {
		if i > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(contactQueryEscape(param.name))
		builder.WriteByte('=')
		builder.WriteString(contactQueryEscape(param.value))
	}
	return builder.String()
}

// contactQueryEscape 是 WHATWG application/x-www-form-urlencoded 序列化器：
// 空格 → `+`；`*` `-` `.` 数字字母 `_` 原样；其余按 UTF-8 逐字节 `%XX`（大写十六进制）。
//
// 与 Go 的 url.QueryEscape 有两处分歧，必须自己写：`*`（Go 转义 / JS 不转义）、
// `~`（Go 不转义 / JS 转义成 %7E）。
func contactQueryEscape(value string) string {
	const hex = "0123456789ABCDEF"
	var builder strings.Builder
	for i := 0; i < len(value); i++ {
		byteValue := value[i]
		switch {
		case byteValue == ' ':
			builder.WriteByte('+')
		case byteValue >= 'a' && byteValue <= 'z',
			byteValue >= 'A' && byteValue <= 'Z',
			byteValue >= '0' && byteValue <= '9',
			byteValue == '*', byteValue == '-', byteValue == '.', byteValue == '_':
			builder.WriteByte(byteValue)
		default:
			builder.WriteByte('%')
			builder.WriteByte(hex[byteValue>>4])
			builder.WriteByte(hex[byteValue&0x0f])
		}
	}
	return builder.String()
}

// contactQueryUnescape 是 URLSearchParams 的解析面：`+` 还原成空格，`%XX` 还原字节。
func contactQueryUnescape(value string) string {
	value = strings.ReplaceAll(value, "+", " ")
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

// jsInteger 是 `URLSearchParams` 里 `String(Math.round(x))` 的整数形态（无小数点、无指数）。
func jsInteger(value float64) string {
	return strconv.FormatInt(int64(value), 10)
}
