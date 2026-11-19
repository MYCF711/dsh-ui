// settings_voice.go —— 账本 #66 ④「MicrophonePicker 设置页」在本仓的**数据模型**。
//
// # 官方这一页挂在哪（逐行）
//
//	OFF:client-ui-voice-input/src/client/mount.ts:57-59
//	  ctx.slots.inject('plugins.bundle.config', () => ctx.slots.register({
//	    name: 'plugins.bundle.config',
//	    key: '@deepseek-ai/dsh-experimental-voice-input-bundle', locale: NS,
//	    inject: () => actions,
//	  }, VoicePreparation))
//	OFF:client-ui-voice-input/src/client/PreparationCard.tsx:147
//	  {catalog && <div className={css.preferences}>
//	    <MicrophonePicker {...props} />
//	    …（识别服务 / 识别语言 / 计费说明）
//
// 也就是**不是** `plugins.item` 那种「一串 SettingsValueField」的页，而是
// `plugins.bundle.config` 槽里的一页，正文的第一格就是麦克风选择器。本仓没有
// bundles 组（宿主 `listBundles` 只给 profile bundle，见 plugins_page.go:426-428），
// 所以这张卡落在「内置插件配置」组的末尾、`Kind = "voice-input"` 走专用绘制
// （形态分发在 settings_items_draw.go 的 settingsItemCards）。**顺序与分组是
// 本仓落点差异**，已登记；标题/摘要/文案/几何全部是官方原文。
//
// # 结构（OFF:MicrophonePicker.tsx:124-139，逐元素）
//
//	.deviceRow                      grid:100px minmax(120px,280px)，gap 12
//	  <label htmlFor="voice-input-device">{t('inputDevice')}</label>
//	  .deviceField                  position:relative（本仓没有 absolute 叠层，见下）
//	    <select id="voice-input-device">  0.5px 边框 / r6 / padding 6px 10px / 透明底
//	      …options（列表见 settingsVoiceChoices）
//	    .deviceMeter（仅在**有真实电平**时） 绝对定位 right:24px、垂直居中
//	      .inputLevel                 8 根 3×12 的 bar，gap 3、r1
//	  .deviceError（role=alert）      命中失败时的一行，grid-column:2
//
// # 与本仓壳的三条**必须登记**的差异（不伪造）
//
//	① **没有实时预览**。官方页面打开期间自己开一路 `createRecording().preview()`
//	   （MicrophonePicker.tsx:38-130）采音，所以 8 根 bar 一直在跳；本仓
//	   `pluginapi.VoiceActions`（pluginapi/plugin.go:2392-2427）没有预览入口，
//	   壳只在真正听写（`phase=recording`）时刷新 `VoiceState.Level`
//	   （cmd/app/speech_voice.go:324-343 的 voiceRecordLoop）。所以本仓的
//	   `.deviceMeter` **只在 phase==recording 时**出现并按官方公式点亮；没有真实
//	   数据时一根都不点亮，不做空闲动画、不编 0 值预览冒充。
//	② **`.deviceError` 生产路径恒不出现**。官方那一行是预览采集失败
//	   （`failureText(failure, t)`，:49-52/:138）；本仓没有预览就没有这条失败源。
//	   元素与几何照官方实现（有数据就画），数据面不存在，不在别处找句子冒充。
//	③ **`.deviceMeter` 用真 absolute 叠层**（viva `ui.Box.Absolute()`，与官方
//	   `position:absolute;top:50%;right:24px` 同义），不是流内挤占：所以
//	   `select` 的右内缩仍是官方的 72px，文字不会被仪表挤压。
//	④ **没有下拉箭头**。官方是原生 `<select>`，右侧那个小三角是**浏览器 UA 画的**，
//	   包里既没有它的 CSS 也没有素材（`VoiceInput.module.css:31` 只有
//	   `background:transparent`）。按仓规「缺官方证据不自造图标」，本仓不手绘冒充：
//	   收起态只画文字（`.deviceMeter{right:24px}` 与
//	   `.deviceSelectMeter{padding-right:72px}` 正好给箭头留出右侧 24px 位置，
//	   需要画时位置是确定的，见 ③）。**这是可见差异，已登记**。
//
// # 「系统默认」这一项
//
// 官方首行是 `{id:”, label:defaultLabel}`（:87），本仓 `VoiceDevice.ID == -1`
// 就是 `WAVE_MAPPER`（pluginapi/plugin.go:2436-2440 的注释与
// cmd/app/speech_voice.go:77-92 的初始化），语义等价 ⇒ 列表首行是 ID=-1。
// 官方的 `label` 是"系统默认设备的名字"（从 `enumerateDevices` 的 default 那条
// 经 groupId 反查）；本仓壳不提供这个名字（`audio.Devices()` 只回物理设备名，
// internal/audio/capture_windows.go:100-118），所以只画 `系统默认`，
// **不自造** `系统默认（…）`；`systemMicrophoneNamed` 的替换代码在，等壳给名字。
package dssidebar

import (
	"math"
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// ── 官方标识与文案（逐字，勿改）────────────────────────────────────

const (
	// settingsVoiceCardID 是这张卡的 id。官方这条 UI 包的命名空间是
	// `NS = 'voice-input'`（OFF:locales.ts:4），卡片 id 取同一名字。
	settingsVoiceCardID = "voice-input"
	// settingsVoiceBundleKey 是 `plugins.bundle.config` 的注册键
	// （OFF:mount.ts:58 的 key），也是本仓卡片 Anchor 的另一半。
	settingsVoiceBundleKey = "@deepseek-ai/dsh-experimental-voice-input-bundle"
	// settingsVoiceDeviceDOMID 是官方**写死**的 select id（OFF:MicrophonePicker.tsx:125/127），
	// 本仓当稳定交互 id 与 `htmlFor` 的等价物用。
	settingsVoiceDeviceDOMID = "voice-input-device"

	// settingsVoicePhaseRecording 是 `VoiceState.Phase` 里"正在采集"那一档
	// （与官方 phase 字面量同名，见 pluginapi/plugin.go 的 VoiceState 注释）。
	settingsVoicePhaseRecording = "recording"
	// settingsVoiceSystemDeviceID 是"系统默认"那一项的 ID：官方是空串 `''`，
	// 本仓 VoiceDevice.ID 的约定是 -1（pluginapi/plugin.go:2436-2440）。
	settingsVoiceSystemDeviceID = -1
	// settingsVoiceLevelFloor 是官方 `Math.max(amplitude, 0.00001)` 的下限
	// （OFF:MicrophonePicker.tsx:19）。
	settingsVoiceLevelFloor = 0.00001
)

// 官方 zh 文案（OFF:locales.ts:85-91，**逐字**；`{name}` / `{number}` 是占位符）。
const (
	zhVoiceInputDevice           = "输入设备"         // 'inputDevice' :85
	zhVoiceSystemMicrophone      = "系统默认"         // 'systemMicrophone' :86
	zhVoiceSystemMicrophoneNamed = "系统默认（{name}）" // 'systemMicrophoneNamed' :87
	zhVoiceUnnamedSelectedMic    = "麦克风"          // 'unnamedSelectedMicrophone' :88
	zhVoiceUnnamedMicrophone     = "麦克风 {number}" // 'unnamedMicrophone' :89
	zhVoiceInputLevel            = "麦克风输入电平"      // 'inputLevel' :90
	zhVoiceDeviceUnavailable     = "不可用"          // 'deviceUnavailable' :91
)

// 卡片标题与摘要：官方这一页的 bundle 元数据
// （OFF:voice-input-bundle/locale/zh.json:3-4 的 meta.title / meta.description）。
// 本仓卡片标题/摘要一律来自官方原文，不自造。
const (
	zhVoiceCardTitle   = "语音输入"
	zhVoiceCardSummary = "在本机使用 SenseVoice 转写录音，首次使用需安装依赖。"
)

// ── 几何（官方 VoiceInput.module.css 逐行；测试逐行核对）────────────

const (
	// `.deviceRow{grid-template-columns:100px minmax(120px,280px)}`（css:30）。
	voiceDeviceLabelWidth float32 = 100 // 第一列固定 100px
	voiceDeviceFieldMin   float32 = 120 // 第二列 minmax(120px, …) 的下界
	voiceDeviceFieldMax   float32 = 280 // 第二列 … 280px) 的上界
	// `.deviceRow{gap:12px}`（css:30）。
	voiceDeviceGap float32 = 12
	// `@media (max-width:480px){.deviceRow{grid-template-columns:minmax(0,1fr);gap:6px}}`（css:51-53）。
	voiceDeviceBreakpoint float32 = 480
	voiceDeviceNarrowGap  float32 = 6

	// `select{border:0.5px solid var(--dsw-alias-border-l1);border-radius:6px;
	//         padding:6px 10px;background:transparent;color:inherit;font:inherit}`（css:31）。
	voiceSelectStroke float32 = 0.5
	voiceSelectRadius float32 = 6
	voiceSelectPadY   float32 = 6
	voiceSelectPadX   float32 = 10
	// `font:inherit` ← `.preferences{font-size:13px}`（css:29）；官方没给 line-height，
	// 按本产品全局 1.5（13 × 1.5 = 19.5）—— 与 settings_items_draw.go 的
	// itemFieldInputFont/Line 同一口径。
	voiceSelectFont float32 = 13
	voiceSelectLine float32 = 19.5
	// select 的自然高度 = 上下 padding + 一行字 + 上下描边 = 6+6+19.5+0.5+0.5。
	// 官方没写 height（内容撑开），本仓显式钉住这个值，absolute 仪表才好垂直居中。
	voiceSelectHeight float32 = 2*voiceSelectPadY + voiceSelectLine + 2*voiceSelectStroke

	// `.deviceField select.deviceSelectMeter{padding-right:72px}`（css:45）：
	// 有仪表时给右侧 72px 让位。
	voiceSelectPadRight float32 = 72
	// `.deviceMeter{position:absolute;top:50%;right:24px;transform:translateY(-50%)}`（css:46）。
	voiceMeterRight float32 = 24

	// `.inputLevel{display:inline-flex;align-items:center;gap:3px}`（css:48）。
	voiceLevelGap float32 = 3
	// `.inputLevel>span{width:3px;height:12px;border-radius:1px}`（css:49）。
	voiceLevelBarWidth  float32 = 3
	voiceLevelBarHeight float32 = 12
	voiceLevelBarRadius float32 = 1
	// `Array.from({length:8}, …)`（OFF:MicrophonePicker.tsx:28）。
	voiceLevelBars int = 8

	// `.deviceError{grid-column:2;…;font-size:12px}`（css:47）。
	voiceErrorSize float32 = 12
	voiceErrorLine float32 = 18
)

// voiceGeoRow 是一行几何事实（与 itemGeoRow / webGeoRow 同构）。
type voiceGeoRow struct {
	Node     string
	Prop     string
	Official float32
	Impl     float32
	Anchor   string
}

// settingsVoiceGeometry 是这一页的几何台账：每一行的 Official 必须等于 Impl，
// 锚点一律是官方 CSS 原文的行号（`VoiceInput.module.css`）。
func settingsVoiceGeometry() []voiceGeoRow {
	const css = "client-ui-voice-input/src/client/VoiceInput.module.css"
	return []voiceGeoRow{
		{"deviceRow", "grid-template-columns[0]", 100, voiceDeviceLabelWidth, css + ":30"},
		{"deviceRow", "grid-template-columns[1] min", 120, voiceDeviceFieldMin, css + ":30"},
		{"deviceRow", "grid-template-columns[1] max", 280, voiceDeviceFieldMax, css + ":30"},
		{"deviceRow", "gap", 12, voiceDeviceGap, css + ":30"},
		{"select", "border-width", 0.5, voiceSelectStroke, css + ":31"},
		{"select", "border-radius", 6, voiceSelectRadius, css + ":31"},
		{"select", "padding-block", 6, voiceSelectPadY, css + ":31"},
		{"select", "padding-inline", 10, voiceSelectPadX, css + ":31"},
		{"deviceField select.deviceSelectMeter", "padding-right", 72, voiceSelectPadRight, css + ":45"},
		{"deviceMeter", "right", 24, voiceMeterRight, css + ":46"},
		{"inputLevel", "gap", 3, voiceLevelGap, css + ":48"},
		{"inputLevel>span", "width", 3, voiceLevelBarWidth, css + ":49"},
		{"inputLevel>span", "height", 12, voiceLevelBarHeight, css + ":49"},
		{"inputLevel>span", "border-radius", 1, voiceLevelBarRadius, css + ":49"},
		{"deviceError", "font-size", 12, voiceErrorSize, css + ":47"},
		{"deviceRow@media(max-width)", "breakpoint", 480, voiceDeviceBreakpoint, css + ":51-55"},
		{"deviceRow@media(max-width)", "gap", 6, voiceDeviceNarrowGap, css + ":51-53"},
	}
}

// voiceMeterWidth 是 8 根 bar 加 7 个间隙的宽度（css:48-49：8×3 + 7×3 = 45）。
const voiceMeterWidth float32 = float32(voiceLevelBars)*voiceLevelBarWidth +
	float32(voiceLevelBars-1)*voiceLevelGap

// ── 选项（官方 choices / deviceName / 过滤）────────────────────────

// settingsVoiceDevice 是一只**候选**设备（官方 `DeviceChoice`，OFF:MicrophonePicker.tsx:11）。
type settingsVoiceDevice struct {
	ID   int
	Name string
	// unavailable 对应当前系统默认行"没有设备可枚举"（官方 `inputs.length === 0`，
	// :87）与"选中项已失效但仍保留该行"（:96）。
	unavailable bool
}

// settingsVoiceChoice 是一个 <option>（官方 `choices.map(...)`，:132-134）。
type settingsVoiceChoice struct {
	// ID 是 option 的值；-1 = 系统默认。
	ID int
	// Label 是 option 上画出来的文字（已含 ` — 不可用` 后缀）。
	Label string
	// Unavailable 表示这一项当前不可用；Disabled 是官方 `disabled={device.unavailable}`。
	Unavailable bool
	Disabled    bool
	// Selected 表示 value 等于它的那一条（官方 `value={selected.id}`，:127）。
	Selected bool
}

// settingsVoiceChoices 造这一帧的 option 表，逐条照官方：
//
//	:87   首行永远是系统默认（`{id:'', label: defaultLabel, unavailable: inputs.length === 0}`）
//	:88-90 其余是枚举出来的设备
//	:91-99 选中项在清单里不存在时**保留该行**并标 unavailable（splice 到第 1 位之后）
//	:117  `choices = devices.filter(d => !d.unavailable || d.id === selected.id || d.id === '')`
func settingsVoiceChoices(capability pluginapi.VoiceCapability) []settingsVoiceChoice {
	devices := []settingsVoiceDevice{{
		ID:   settingsVoiceSystemDeviceID,
		Name: settingsVoiceDefaultDeviceName(capability),
		// 官方这一条就是"系统里有没有输入设备"：`inputs.length === 0`（:87）。
		unavailable: len(capability.Devices) == 0,
	}}
	present := make(map[int]bool, len(capability.Devices))
	for _, device := range capability.Devices {
		devices = append(devices, settingsVoiceDevice{ID: device.ID, Name: device.Name})
		present[device.ID] = true
	}
	selected := capability.DeviceID
	if selected != settingsVoiceSystemDeviceID && !present[selected] {
		// 官方把失效的当前项插在 `Math.max(1, previous.findIndex(...))`——前一轮
		// 找不到时是 1。本仓不缓存上一轮的名录（壳的名录就是真值），所以这里
		// 与"前一轮没有它"同一条路径：插在第 1 位之后，名字留空 → 画"麦克风 N"。
		devices = append(devices, settingsVoiceDevice{})
		copy(devices[2:], devices[1:])
		devices[1] = settingsVoiceDevice{ID: selected, unavailable: true}
	}
	choices := make([]settingsVoiceChoice, 0, len(devices))
	for index, device := range devices {
		if device.unavailable && device.ID != selected && device.ID != settingsVoiceSystemDeviceID {
			continue
		}
		label := settingsVoiceDeviceName(device, index, selected, capability)
		if device.unavailable {
			// 官方 `{deviceName(device, index)}{device.unavailable ? ` — ${t('deviceUnavailable')}` : ''}`
			// （:133）：破折号是 U+2014，两侧各一个空格。
			label += " — " + zhVoiceDeviceUnavailable
		}
		choices = append(choices, settingsVoiceChoice{
			ID: device.ID, Label: label,
			Unavailable: device.unavailable, Disabled: device.unavailable,
			Selected: device.ID == selected,
		})
	}
	return choices
}

// settingsVoiceDefaultDeviceName 是官方首行的 `defaultLabel`（:85）：
// `physicalDefault?.label || activeDefault?.label || system?.label || ”`。
//
// 本仓壳只回物理设备名录（`audio.Devices()`），没有"系统默认是哪一只"这一栏，
// 所以这里恒为空串 ⇒ 首行画 `系统默认`（:120 的 `device.label ? named : system`）。
// 壳将来若给名字，这里不用改：`named` 分支已经在。
func settingsVoiceDefaultDeviceName(pluginapi.VoiceCapability) string { return "" }

// settingsVoiceDeviceName 是官方 `deviceName(device, index)`（:119-123）：
//
//	if device.id === ''            → device.label ? systemMicrophoneNamed{name} : systemMicrophone
//	return device.label || (device.id === selected.id ? selected.label : '')
//	     || (index > 0 ? unnamedMicrophone{number=index} : unnamedSelectedMicrophone)
func settingsVoiceDeviceName(device settingsVoiceDevice, index, selected int, capability pluginapi.VoiceCapability) string {
	if device.ID == settingsVoiceSystemDeviceID {
		if device.Name != "" {
			return strings.ReplaceAll(zhVoiceSystemMicrophoneNamed, "{name}", device.Name)
		}
		return zhVoiceSystemMicrophone
	}
	if device.Name != "" {
		return device.Name
	}
	if device.ID == selected {
		// 官方这里是 `selected.label`：当前选中项上一轮记下的名字。
		if name := settingsVoiceNameOf(capability, device.ID); name != "" {
			return name
		}
	}
	if index > 0 {
		return strings.ReplaceAll(zhVoiceUnnamedMicrophone, "{number}", strconv.Itoa(index))
	}
	return zhVoiceUnnamedSelectedMic
}

// settingsVoiceNameOf 在名录里查一只设备的名字（壳没给就是空串）。
func settingsVoiceNameOf(capability pluginapi.VoiceCapability, id int) string {
	for _, device := range capability.Devices {
		if device.ID == id {
			return device.Name
		}
	}
	return ""
}

// settingsVoiceSelectedLabel 取 select 上显示的文字（官方 `value={selected.id}` 的
// 选中项 label；找不到就退回首行，与官方"value 一定在 choices 里"同一条不变量）。
func settingsVoiceSelectedLabel(choices []settingsVoiceChoice) string {
	for _, choice := range choices {
		if choice.Selected {
			return choice.Label
		}
	}
	if len(choices) > 0 {
		return choices[0].Label
	}
	return zhVoiceSystemMicrophone
}

// ── 电平（官方 InputLevel 的公式）────────────────────────────────

// settingsVoiceActiveBars 是官方 InputLevel 的**逐字**公式
// （OFF:MicrophonePicker.tsx:19-21）：
//
//	decibels = 20 * Math.log10(Math.max(recording.amplitude(), 0.00001))
//	count    = Math.ceil(Math.max(0, Math.min(1, (decibels + 60) / 60)) * bars.length)
//	第 i 根标 data-active = String(i < count)
//
// 返回值就是"点亮几根"（0..8）。
func settingsVoiceActiveBars(level float64) int {
	amplitude := level
	if !(amplitude > settingsVoiceLevelFloor) {
		// 官方是 `Math.max(amplitude, 0.00001)`；NaN 走进来也按地板值算（JS 的
		// Math.max(NaN, x) 是 NaN，但记录器不会给 NaN——这里取地板而不是编结果）。
		amplitude = settingsVoiceLevelFloor
	}
	decibels := 20 * math.Log10(amplitude)
	ratio := (decibels + 60) / 60
	if ratio < 0 || math.IsNaN(ratio) {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	count := int(math.Ceil(ratio * float64(voiceLevelBars)))
	if count < 0 {
		count = 0
	}
	if count > voiceLevelBars {
		count = voiceLevelBars
	}
	return count
}

// ── 这一帧的呈现状态 ────────────────────────────────────────────

// settingsVoiceState 是这一页这一帧的状态（`settingsItemCardState.Voice`）。
type settingsVoiceState struct {
	// Choices 是 <option> 表（顺序即官方 choices 顺序）。
	Choices []settingsVoiceChoice
	// Selected 是当前选中项的 ID（官方 `value={selected.id}`）。
	Selected int
	// Value 是 select 上显示的当前值文字。
	Value string
	// Meter 表示**有真实电平数据**：壳只在 `phase=recording` 时刷新 Level
	// （cmd/app/speech_voice.go:324-343），所以只有那时才画 `.deviceMeter`
	// 并给 select 挂 `.deviceSelectMeter`（官方 :127/:136 的 `preview &&`）。
	Meter bool
	// ActiveBars 是 8 根 bar 里点亮几根（`settingsVoiceActiveBars`）。
	ActiveBars int
	// Level 是这一帧读到的真实电平（0..1，壳的 RMS）。
	Level float64
	// Phase 是 `VoiceState.Phase`（五档），只用于报告与测试。
	Phase string
	// DeviceError 是 `.deviceError` 那一行（官方 :138，`role="alert"`）。
	//
	// **本仓生产路径恒为空**：官方那条来自预览采集失败，而本仓
	// `VoiceActions` 没有预览入口（见文件头 ②）。绘制路径照官方实现，
	// 有数据就画，不在这里编句子。
	DeviceError string
	// Selectable 表示壳接了 `SelectDevice`；false 时 select 不挂命中区
	// （不画一个点了没反应的控件）。
	Selectable bool
	// Refreshable 表示壳接了 `RefreshDevices`（官方页面打开即重新枚举一次，
	// MicrophonePicker.tsx:104-111 的 refresh()）。
	Refreshable bool
	// Available / Reason 是 `VoiceCapability` 的原值（不做二次判定，也不吞掉）。
	Available bool
	Reason    string
}

// settingsVoiceItem 是这张卡的台账条目。
//
// 标题/摘要是官方 bundle 元数据（voice-input-bundle/locale/zh.json:3-4）；
// Anchor 是官方两处注册/挂载点 —— `plugins.bundle.config` 的注册
// （mount.ts:57-59，键 = settingsVoiceBundleKey）与 `<MicrophonePicker>` 的调用
// （PreparationCard.tsx:147）。
func settingsVoiceItem() settingsItem {
	return settingsItem{
		ID:      settingsVoiceCardID,
		Order:   50,
		Kind:    settingsItemKindVoiceInput,
		Title:   zhVoiceCardTitle,
		Summary: zhVoiceCardSummary,
		Anchor:  "client-ui-voice-input/src/client/mount.ts:57-59 · PreparationCard.tsx:147",
	}
}

// buildSettingsVoiceCard 把 `Act.Voice` 投影成这一页的卡片状态。
//
// 返回 false = **这一帧没有这张卡**：壳整条没接（`Act.Voice == nil`）或没给
// `Capability`。官方同一件事：`plugins.bundle.config` 的占用方要 `inject` 到
// `remote.speech` 与那套 actions，没有它们这一页根本不注册
// （mount.ts:19/:74-75）。所以这里不画空壳、不画死控件。
func buildSettingsVoiceCard(item settingsItem, act *pluginapi.VoiceActions) (settingsItemCardState, bool) {
	if act == nil || act.Capability == nil {
		return settingsItemCardState{}, false
	}
	capability := act.Capability()
	state := settingsVoiceState{
		Selected:    capability.DeviceID,
		Available:   capability.Available,
		Reason:      capability.Reason,
		Selectable:  act.SelectDevice != nil,
		Refreshable: act.RefreshDevices != nil,
	}
	state.Choices = settingsVoiceChoices(capability)
	state.Value = settingsVoiceSelectedLabel(state.Choices)
	if act.State != nil {
		voice := act.State()
		state.Phase = voice.Phase
		state.Level = voice.Level
		// 只有 recording 阶段壳在刷新电平（cmd/app/speech_voice.go:324-343）：
		// 其它阶段的 Level 是上一次留下的 0，不是"预览数据"，不画成仪表。
		state.Meter = voice.Phase == settingsVoicePhaseRecording
		if state.Meter {
			state.ActiveBars = settingsVoiceActiveBars(voice.Level)
		}
	}
	return settingsItemCardState{
		Item:     item,
		Known:    true,
		Writable: act.SelectDevice != nil,
		Voice:    &state,
	}, true
}
