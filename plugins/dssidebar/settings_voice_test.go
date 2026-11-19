// settings_voice_test.go —— 账本 #66 ④「MicrophonePicker 设置页」的**数据模型证据**。
//
// 每条断言都对着官方原文（锚点写在 settings_voice.go 的文件头与常量注释里）：
//
//	OFF:client-ui-voice-input/src/client/MicrophonePicker.tsx:13-30   电平公式与 8 根 bar
//	OFF:client-ui-voice-input/src/client/MicrophonePicker.tsx:87-99   首行/保留失效项/splice
//	OFF:client-ui-voice-input/src/client/MicrophonePicker.tsx:117     choices 过滤
//	OFF:client-ui-voice-input/src/client/MicrophonePicker.tsx:119-123 选项文案
//	OFF:client-ui-voice-input/src/client/MicrophonePicker.tsx:132-134 disabled 与「 — 不可用」
//	OFF:client-ui-voice-input/src/client/VoiceInput.module.css:30,31,43-50,51-55  几何
//	OFF:client-ui-voice-input/src/client/locales.ts:85-91             zh 文案逐字
//	OFF:voice-input-bundle/locale/zh.json:3-4                        卡片标题/摘要
//
// 真实绘制与点击由 settings_voice_visual_test.go 走真 Viva（headless）。
package dssidebar

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// settingsVoiceCapabilityFixture 是一台"有两支麦、选中系统默认"的机器。
func settingsVoiceCapabilityFixture() pluginapi.VoiceCapability {
	return pluginapi.VoiceCapability{
		Available: true,
		Devices: []pluginapi.VoiceDevice{
			{ID: 0, Name: "麦克风阵列 (Realtek(R) Audio)"},
			{ID: 1, Name: "USB 麦克风"},
		},
		DeviceID: settingsVoiceSystemDeviceID,
	}
}

// settingsVoiceStateFixture 造一份会话状态。
func settingsVoiceStateFixture(phase string, level float64) pluginapi.VoiceState {
	return pluginapi.VoiceState{Phase: phase, Level: level}
}

// settingsVoiceChoicesOf 取 fixture 的 option 表。
func settingsVoiceChoicesOf(capability pluginapi.VoiceCapability) []settingsVoiceChoice {
	return settingsVoiceChoices(capability)
}

// TestSettingsVoiceGeometryMatchesOfficialCSS 逐行核对官方 CSS 的几何。
//
// 台账里每行 Official 必须等于 Impl；下面再把 15 个"官方数字"逐个钉住
// （任务书要求：100/120/280/12/0.5/6/6/10/72/24/3/12/3/1/12），并核对
// 72 = 24 + 45 + 3 这条**推导**（官方 select 的右内缩正好盖住绝对定位的仪表
// 再留 3px 空隙），所以 72 不是随手写的数。
func TestSettingsVoiceGeometryMatchesOfficialCSS(t *testing.T) {
	rows := settingsVoiceGeometry()
	if len(rows) == 0 {
		t.Fatal("几何台账是空的")
	}
	for _, row := range rows {
		if row.Official != row.Impl {
			t.Errorf("%s.%s：官方 %v ≠ 实现 %v（%s）", row.Node, row.Prop, row.Official, row.Impl, row.Anchor)
		}
		if !strings.Contains(row.Anchor, "VoiceInput.module.css") {
			t.Errorf("%s.%s 的锚点 %q 不是官方 CSS 文件", row.Node, row.Prop, row.Anchor)
		}
	}
	// 官方 VoiceInput.module.css:30 —— 100px / minmax(120px,280px) / gap 12。
	if voiceDeviceLabelWidth != 100 || voiceDeviceFieldMin != 120 ||
		voiceDeviceFieldMax != 280 || voiceDeviceGap != 12 {
		t.Fatalf(".deviceRow 列宽/间距 = %v/%v/%v/%v（官方 100/120/280/12）",
			voiceDeviceLabelWidth, voiceDeviceFieldMin, voiceDeviceFieldMax, voiceDeviceGap)
	}
	// 官方 VoiceInput.module.css:31 —— 0.5px 描边 / r6 / padding 6px 10px。
	if voiceSelectStroke != 0.5 || voiceSelectRadius != 6 ||
		voiceSelectPadY != 6 || voiceSelectPadX != 10 {
		t.Fatalf("select 几何 = %v/%v/%v/%v（官方 0.5/6/6/10）",
			voiceSelectStroke, voiceSelectRadius, voiceSelectPadY, voiceSelectPadX)
	}
	// 官方 VoiceInput.module.css:45-46 —— 有仪表时右内缩 72，仪表右边缘 24。
	if voiceSelectPadRight != 72 || voiceMeterRight != 24 {
		t.Fatalf("仪表让位 = %v / right %v（官方 72 / 24）", voiceSelectPadRight, voiceMeterRight)
	}
	// 官方 VoiceInput.module.css:48-49 —— gap 3 / 3×12 / r1，共 8 根。
	if voiceLevelGap != 3 || voiceLevelBarWidth != 3 || voiceLevelBarHeight != 12 || voiceLevelBarRadius != 1 {
		t.Fatalf("inputLevel = gap %v / %v×%v / r%v（官方 3 / 3×12 / r1）",
			voiceLevelGap, voiceLevelBarWidth, voiceLevelBarHeight, voiceLevelBarRadius)
	}
	if voiceLevelBars != 8 {
		t.Fatalf("bar 数 = %d（官方 `Array.from({length:8})`，MicrophonePicker.tsx:28）", voiceLevelBars)
	}
	// 官方 VoiceInput.module.css:47 —— .deviceError 12px。
	if voiceErrorSize != 12 {
		t.Fatalf(".deviceError 字号 = %v（官方 12）", voiceErrorSize)
	}
	// 官方 VoiceInput.module.css:51-55 —— ≤480px 断点，单列 + gap 6。
	if voiceDeviceBreakpoint != 480 || voiceDeviceNarrowGap != 6 {
		t.Fatalf("断点 = %v / gap %v（官方 480 / 6）", voiceDeviceBreakpoint, voiceDeviceNarrowGap)
	}
	// 推导：仪表宽 = 8×3 + 7×3 = 45；72 − 24 − 45 = 3（官方 select 右内缩给
	// 绝对定位仪表留出位置后还剩 3px 空隙）。
	if voiceMeterWidth != 45 {
		t.Fatalf("仪表宽 = %v（8 根 3px + 7 个 3px 间隙 = 45）", voiceMeterWidth)
	}
	if slack := voiceSelectPadRight - voiceMeterRight - voiceMeterWidth; slack != 3 {
		t.Fatalf("72 − 24 − 45 = %v（want 3：官方右内缩相对仪表的剩余空隙）", slack)
	}
	// select 的自然高度 = 6+6+19.5+0.5+0.5（内容撑开，官方没写 height）。
	if voiceSelectHeight != 32.5 {
		t.Fatalf("select 高度 = %v（want 32.5 = 2×6 + 19.5 + 2×0.5）", voiceSelectHeight)
	}
}

// TestSettingsVoiceSelectionFollowsCapabilityDeviceID 钉住「选中项来自
// `Capability().DeviceID`」（官方 `value={selected.id}`，MicrophonePicker.tsx:127）。
func TestSettingsVoiceSelectionFollowsCapabilityDeviceID(t *testing.T) {
	capability := settingsVoiceCapabilityFixture()
	capability.DeviceID = 1
	choices := settingsVoiceChoicesOf(capability)
	selected := make([]int, 0, 1)
	for _, choice := range choices {
		if choice.Selected {
			selected = append(selected, choice.ID)
		}
	}
	if len(selected) != 1 || selected[0] != 1 {
		t.Fatalf("选中项 = %v（want 只有 1，来自 DeviceID=1）", selected)
	}
	if value := settingsVoiceSelectedLabel(choices); value != "USB 麦克风" {
		t.Fatalf("select 显示值 = %q（want 选中设备名 USB 麦克风）", value)
	}
	// 系统默认不再是选中项。
	if choices[0].Selected {
		t.Fatal("DeviceID=1 时系统默认不该还是选中项")
	}
}

// TestSettingsVoiceChoicesKeepSystemDefaultFirst 钉住官方首行与设备名（:87-90/:119-123）。
func TestSettingsVoiceChoicesKeepSystemDefaultFirst(t *testing.T) {
	choices := settingsVoiceChoicesOf(settingsVoiceCapabilityFixture())
	if len(choices) != 3 {
		t.Fatalf("option 数 = %d（want 3：系统默认 + 两支麦）；choices=%+v", len(choices), choices)
	}
	if choices[0].ID != settingsVoiceSystemDeviceID {
		t.Fatalf("首行 ID = %d（want -1 系统默认，官方首行 id 是空串）", choices[0].ID)
	}
	// 壳不给"系统默认设备名"⇒ 只画 `系统默认`，不许编 `系统默认（…）`。
	if choices[0].Label != zhVoiceSystemMicrophone {
		t.Fatalf("首行文字 = %q（want %q）", choices[0].Label, zhVoiceSystemMicrophone)
	}
	if !choices[0].Selected {
		t.Fatal("DeviceID=-1 时首行必须是选中项")
	}
	if choices[0].Disabled || choices[0].Unavailable {
		t.Fatalf("有设备时系统默认行不该不可用：%+v", choices[0])
	}
	// 真实设备名逐字（官方 `device.label`）。
	if choices[1].Label != "麦克风阵列 (Realtek(R) Audio)" || choices[2].Label != "USB 麦克风" {
		t.Fatalf("设备名 = %q / %q（要逐字来自 VoiceDevice.Name）", choices[1].Label, choices[2].Label)
	}
	for _, choice := range choices[1:] {
		if choice.Unavailable || choice.Disabled {
			t.Fatalf("枚举到的设备不该标不可用：%+v", choice)
		}
	}
}

// TestSettingsVoiceSystemDefaultNamedWhenShellKnowsName 钉住 `systemMicrophoneNamed`
// 的占位符替换路径（官方 :120）。本仓壳当前不给名字，但这段代码必须在：
// 名字一到就画 `系统默认（{name}）`。
func TestSettingsVoiceSystemDefaultNamedWhenShellKnowsName(t *testing.T) {
	if got := settingsVoiceDeviceName(settingsVoiceDevice{ID: settingsVoiceSystemDeviceID}, 0,
		settingsVoiceSystemDeviceID, settingsVoiceCapabilityFixture()); got != zhVoiceSystemMicrophone {
		t.Fatalf("没有名字时 = %q（want %q）", got, zhVoiceSystemMicrophone)
	}
	got := settingsVoiceDeviceName(settingsVoiceDevice{ID: settingsVoiceSystemDeviceID, Name: "扬声器阵列"},
		0, settingsVoiceSystemDeviceID, settingsVoiceCapabilityFixture())
	want := strings.ReplaceAll(zhVoiceSystemMicrophoneNamed, "{name}", "扬声器阵列")
	if got != want {
		t.Fatalf("有名字时 = %q（want %q，官方 :120 的 systemMicrophoneNamed）", got, want)
	}
}

// TestSettingsVoiceUnnamedMicrophoneSuffix 钉住官方 `unnamedMicrophone{number}`
// （:122，number = 在 choices 里的序号）。
func TestSettingsVoiceUnnamedMicrophoneNumberedByIndex(t *testing.T) {
	capability := pluginapi.VoiceCapability{
		Available: true,
		Devices:   []pluginapi.VoiceDevice{{ID: 0, Name: ""}, {ID: 1, Name: ""}},
		DeviceID:  settingsVoiceSystemDeviceID,
	}
	choices := settingsVoiceChoicesOf(capability)
	if len(choices) != 3 {
		t.Fatalf("option 数 = %d", len(choices))
	}
	// 官方：index>0 ⇒ `麦克风 {number}`，number 是**过滤后列表**的序号。
	for index, want := range []string{zhVoiceSystemMicrophone, "麦克风 1", "麦克风 2"} {
		if choices[index].Label != want {
			t.Fatalf("第 %d 项 = %q（want %q）", index, choices[index].Label, want)
		}
	}
}

// TestSettingsVoiceMissingSelectedKeepsRowUnavailable 钉住官方 :91-99 的 splice：
// 选中的设备从名录里消失时**保留这一行**、标不可用、画 ` — 不可用` 且不可选。
func TestSettingsVoiceMissingSelectedKeepsRowUnavailable(t *testing.T) {
	capability := settingsVoiceCapabilityFixture()
	capability.DeviceID = 7 // 名录里没有 7（拔掉了）
	choices := settingsVoiceChoicesOf(capability)
	if len(choices) != 4 {
		t.Fatalf("option 数 = %d（want 4：系统默认 + 保留的失效项 + 两支麦）；choices=%+v",
			len(choices), choices)
	}
	kept := choices[1]
	if kept.ID != 7 {
		t.Fatalf("保留行的 ID = %d（want 7）", kept.ID)
	}
	if !kept.Unavailable || !kept.Disabled {
		t.Fatalf("保留行必须 unavailable 且 disabled：%+v", kept)
	}
	if !kept.Selected {
		t.Fatalf("保留行是被选中的那一项：%+v", kept)
	}
	// 官方 :133 的后缀：` — ` + 「不可用」（U+2014，两侧各一个空格）。
	if kept.Label != "麦克风 1 — 不可用" {
		t.Fatalf("保留行文字 = %q（want %q）", kept.Label, "麦克风 1 — 不可用")
	}
}

// TestSettingsVoiceChoicesFilterDropsUnavailableOthers 钉住官方 :117 的过滤：
// 不可用的项**除当前项与系统默认**外都不画。
func TestSettingsVoiceChoicesFilterDropsUnavailableOthers(t *testing.T) {
	// 没有设备 ⇒ 首行（系统默认）不可用，但按 :117 必须保留。
	empty := pluginapi.VoiceCapability{Available: false, DeviceID: settingsVoiceSystemDeviceID,
		Reason: "这台机器没有录音设备"}
	choices := settingsVoiceChoicesOf(empty)
	if len(choices) != 1 {
		t.Fatalf("没有设备时 option 数 = %d（want 1：只留系统默认）；choices=%+v", len(choices), choices)
	}
	if !choices[0].Unavailable || !choices[0].Disabled {
		t.Fatalf("没有设备时系统默认行必须不可用：%+v", choices[0])
	}
	if choices[0].Label != zhVoiceSystemMicrophone+" — "+zhVoiceDeviceUnavailable {
		t.Fatalf("系统默认行文字 = %q（want %q）", choices[0].Label,
			zhVoiceSystemMicrophone+" — "+zhVoiceDeviceUnavailable)
	}
}

// TestSettingsVoiceActiveBarsMatchOfficialFormula 钉住官方 InputLevel 公式
// （MicrophonePicker.tsx:19-21）的取值表。期望值由公式手算，不从实现反推：
//
//	decibels = 20*log10(max(level, 1e-5))；ratio = clamp((decibels+60)/60, 0, 1)
//	count = ceil(ratio * 8)
func TestSettingsVoiceActiveBarsMatchOfficialFormula(t *testing.T) {
	cases := []struct {
		level float64
		want  int
	}{
		{1, 8},         // 0 dB → ratio 1 → 8
		{0.5, 8},       // -6.02 dB → 0.8997×8 = 7.20 → ceil 8
		{0.1778279, 6}, // -15 dB → 0.75×8 = 6 → ceil 6
		{0.1, 6},       // -20 dB → 0.6667×8 = 5.33 → ceil 6
		{0.01, 3},      // -40 dB → 0.3333×8 = 2.67 → ceil 3
		{0.0031623, 2}, // -50 dB → 0.1667×8 = 1.33 → ceil 2
		{0.001, 1},     // -60 dB → ratio 0 → ceil 0 … 但 0.001 是**恰好** -60 dB
		{0.0001, 0},    // -80 dB → ratio clamp 0 → 0
		{0, 0},         // 地板值 1e-5 → -100 dB → 0
		{-1, 0},        // 负电平按 `Math.max(level, 1e-5)` 地板 → 0
	}
	for _, c := range cases {
		got := settingsVoiceActiveBars(c.level)
		// `-60 dB` 落在 ratio 恰为 0 的边界：ceil(0)=0 ⇒ 0 根。
		want := c.want
		if c.level == 0.001 {
			want = 0
		}
		if got != want {
			t.Errorf("level=%v：点亮 %d 根（want %d）", c.level, got, want)
		}
	}
	// 单调性：电平越高，点亮根数不减（公式本身的性质，防"随手写死"）。
	previous := 0
	for level := 0.0; level <= 1.0; level += 0.01 {
		got := settingsVoiceActiveBars(level)
		if got < previous {
			t.Fatalf("level=%v 点亮 %d 根，比更低电平的 %d 根还少", level, got, previous)
		}
		previous = got
	}
}

// TestSettingsVoiceBarsNeverExceedEight 钉住 clamps：任何输入都不越界。
func TestSettingsVoiceBarsNeverExceedEight(t *testing.T) {
	for _, level := range []float64{1, 1.5, 1000, 0.000001, 0, -3} {
		if got := settingsVoiceActiveBars(level); got < 0 || got > voiceLevelBars {
			t.Fatalf("level=%v：点亮 %d 根（必须落在 0..%d）", level, got, voiceLevelBars)
		}
	}
}

// TestSettingsVoiceCardAbsentWithoutShellVoice 钉住「`Act.Voice == nil` ⇒ 整页不画」。
func TestSettingsVoiceCardAbsentWithoutShellVoice(t *testing.T) {
	if _, ok := buildSettingsVoiceCard(settingsVoiceItem(), nil); ok {
		t.Fatal("壳没接 Act.Voice 时不该画出这张卡")
	}
	// `Capability == nil` 也是"没接"（官方没有 readiness 就没有 catalog，页面不注册）。
	if _, ok := buildSettingsVoiceCard(settingsVoiceItem(), &pluginapi.VoiceActions{}); ok {
		t.Fatal("Capability 为 nil 时不该画出这张卡")
	}
	cards := settingsItemCards(nil, settingsItemFormSnapshot{})
	for _, card := range cards {
		if card.Item.ID == settingsVoiceCardID {
			t.Fatalf("voice 卡不该出现；cards=%+v", cards)
		}
	}
	// 接了就有（哪怕 settings describe 一条都没给：官方这页不由 settings 决定）。
	cards = settingsItemCards(nil, settingsItemFormSnapshot{voice: settingsVoiceActionsFor(
		settingsVoiceCapabilityFixture(), settingsVoiceStateFixture("idle", 0))})
	found := false
	for _, card := range cards {
		if card.Item.ID == settingsVoiceCardID {
			found = true
			if !card.Available() {
				t.Fatal("voice 卡存在时必须 Available（页面照样画，设备不可用只影响行）")
			}
		}
	}
	if !found {
		t.Fatalf("壳接了 Act.Voice 却没有 voice 卡；cards=%+v", cards)
	}
}

// settingsVoiceActionsFor 造一份只带只读面与一个 SelectDevice 桩的壳入口。
func settingsVoiceActionsFor(capability pluginapi.VoiceCapability,
	state pluginapi.VoiceState) *pluginapi.VoiceActions {
	return &pluginapi.VoiceActions{
		Capability:     func() pluginapi.VoiceCapability { return capability },
		State:          func() pluginapi.VoiceState { return state },
		SelectDevice:   func(int) {},
		RefreshDevices: func() {},
	}
}

// TestSettingsVoiceMeterNeedsRealLevel 钉住「只有真实电平才画仪表」：
// 壳只在 phase=recording 时刷新 Level（cmd/app/speech_voice.go:324-343），
// 其它阶段的 Level 是上一次留下的 0，不是预览数据 ⇒ 不画、不点亮。
func TestSettingsVoiceMeterNeedsRealLevel(t *testing.T) {
	item := settingsVoiceItem()
	// idle 且 Level 有残值也不能画（那会被误当成"一直在采音"）。
	card, ok := buildSettingsVoiceCard(item,
		settingsVoiceActionsFor(settingsVoiceCapabilityFixture(), settingsVoiceStateFixture("idle", 0.5)))
	if !ok || card.Voice == nil {
		t.Fatal("voice 卡没造出来")
	}
	if card.Voice.Meter {
		t.Fatal("idle 阶段不该画仪表（本仓没有预览采集入口）")
	}
	if card.Voice.ActiveBars != 0 {
		t.Fatalf("idle 阶段点亮 %d 根（want 0）", card.Voice.ActiveBars)
	}
	// recording ⇒ 真值，按官方公式点亮。
	card, ok = buildSettingsVoiceCard(item,
		settingsVoiceActionsFor(settingsVoiceCapabilityFixture(), settingsVoiceStateFixture("recording", 0.01)))
	if !ok || card.Voice == nil {
		t.Fatal("voice 卡没造出来")
	}
	if !card.Voice.Meter {
		t.Fatal("recording 阶段必须画仪表（有真实 Level）")
	}
	if card.Voice.ActiveBars != settingsVoiceActiveBars(0.01) {
		t.Fatalf("recording 点亮 %d 根（want 公式值 %d）", card.Voice.ActiveBars, settingsVoiceActiveBars(0.01))
	}
	// State 为 nil（壳没给状态）⇒ 没有电平数据 ⇒ 不画仪表。
	actions := settingsVoiceActionsFor(settingsVoiceCapabilityFixture(), pluginapi.VoiceState{})
	actions.State = nil
	card, _ = buildSettingsVoiceCard(item, actions)
	if card.Voice == nil || card.Voice.Meter {
		t.Fatal("壳没给 State 时不该画仪表")
	}
}

// TestSettingsVoiceCardCopyComesFromOfficialBundle 钉住卡片文案/锚点来自官方原文。
func TestSettingsVoiceCardCopyComesFromOfficialBundle(t *testing.T) {
	item := settingsVoiceItem()
	if item.ID != settingsVoiceCardID || item.Kind != settingsItemKindVoiceInput {
		t.Fatalf("卡片 id/kind = %q/%q", item.ID, item.Kind)
	}
	// OFF:voice-input-bundle/locale/zh.json:3-4
	if item.Title != "语音输入" {
		t.Fatalf("卡片标题 = %q（官方 meta.title = 语音输入）", item.Title)
	}
	if item.Summary != "在本机使用 SenseVoice 转写录音，首次使用需安装依赖。" {
		t.Fatalf("卡片摘要 = %q（官方 meta.description 逐字）", item.Summary)
	}
	// 锚点：官方注册点（mount.ts:57-59，键 = bundle 名）与挂载点（PreparationCard.tsx:147）。
	if !strings.Contains(item.Anchor, "mount.ts:57-59") ||
		!strings.Contains(item.Anchor, "PreparationCard.tsx:147") {
		t.Fatalf("锚点 = %q（要含 mount.ts:57-59 与 PreparationCard.tsx:147）", item.Anchor)
	}
	if settingsVoiceBundleKey != "@deepseek-ai/dsh-experimental-voice-input-bundle" {
		t.Fatalf("注册键 = %q（官方 mount.ts:58）", settingsVoiceBundleKey)
	}
	if settingsVoiceDeviceDOMID != "voice-input-device" {
		t.Fatalf("select id = %q（官方 :125/:127 写死 id）", settingsVoiceDeviceDOMID)
	}
	// zh 文案逐字（locales.ts:85-91）。
	official := map[string]string{
		zhVoiceInputDevice:        "输入设备",
		zhVoiceSystemMicrophone:   "系统默认",
		zhVoiceUnnamedSelectedMic: "麦克风",
		zhVoiceUnnamedMicrophone:  "麦克风 {number}",
		zhVoiceInputLevel:         "麦克风输入电平",
		zhVoiceDeviceUnavailable:  "不可用",
	}
	for got, want := range official {
		if got != want {
			t.Fatalf("官方 zh 文案 = %q（want %q）", got, want)
		}
	}
	if zhVoiceSystemMicrophoneNamed != "系统默认（{name}）" {
		t.Fatalf("systemMicrophoneNamed = %q（官方 :87 逐字，全角括号）", zhVoiceSystemMicrophoneNamed)
	}
}

// TestSettingsVoiceLedgerKeepsOfficialOrder 钉住这张卡在台账里的位置不动摇既有 4 张卡的相对顺序。
func TestSettingsVoiceLedgerKeepsOfficialOrder(t *testing.T) {
	positions := map[string]int{}
	for index, item := range settingsItemLedger() {
		positions[item.ID] = index
	}
	for _, id := range []string{"subagent", "web-search", "agent-loop", "shell", settingsVoiceCardID} {
		if _, ok := positions[id]; !ok {
			t.Fatalf("台账里缺 %q；positions=%v", id, positions)
		}
	}
	if positions[settingsVoiceCardID] <= positions["shell"] {
		t.Fatalf("voice 卡应在既有 4 张之后（Order 50）：positions=%v", positions)
	}
	if settingsVoiceItem().Order != 50 {
		t.Fatalf("voice 卡 Order = %d（want 50）", settingsVoiceItem().Order)
	}
}
