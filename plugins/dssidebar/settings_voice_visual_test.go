//go:build darwin || windows || linux

// settings_voice_visual_test.go —— 账本 #66 ④「MicrophonePicker 设置页」的**真实绘制**证据。
//
// 走真 Viva：真 Paint（经真 Page + 真命中区）→ 真 HitTest → 真 Click → 再 Paint。
// 判据不是"图里有东西"，而是官方原文那几条：
//
//   - 卡片与详情页的文案：`语音输入`（voice-input-bundle/locale/zh.json:3）、
//     官方摘要、`输入设备`、`系统默认`、真实设备名（locales.ts:85-91 / MicrophonePicker.tsx:119-123）；
//   - 几何：select 是 0.5px 描边 / r6 / padding 6px 10px 的自然高 32.5，字段宽
//     落在 minmax(120px,280px) 里（VoiceInput.module.css:30-31）；8 根 bar 是 3×12、
//     gap 3、r1，仪表右边缘距 select 右边 24px（css:46-49）——全部用**绘制原语**核对；
//   - 交互：点开 select 选另一支麦 ⇒ 真的调用壳的 `SelectDevice(id)`；
//   - 不可用行 ` — 不可用` 且**选不动**（官方 `disabled={device.unavailable}`，:132）；
//   - 电平：`phase=recording` 且给定 Level ⇒ 点亮根数 == 官方公式；没有真实电平 ⇒
//     一根 bar 都不画（本仓没有预览采集，见 settings_voice.go 文件头 ①）；
//   - `Act.Voice == nil` ⇒ 整页不画；
//   - ≤480px 断点 ⇒ 官方单列版式（label 与控件上下叠、gap 6，css:51-53）。
package dssidebar

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// settingsVoiceHarness 是一帧可点、可读文字、可数原语的「插件」页（只接 Act.Voice）。
type settingsVoiceHarness struct {
	t           *testing.T
	p           *Plugin
	theme       *pluginapi.Theme
	tree        *gfx.Tree
	feat        gfx.Scope
	session     *headless.Session
	invalidator *app.Invalidator
	width       int

	mu         sync.Mutex
	capability pluginapi.VoiceCapability
	state      pluginapi.VoiceState
	wire       bool
	selects    []int
	refreshes  int
	// pageCard 非 nil 时 Paint 直接画这一页表单（见 settingsVoiceHarnessOptions.card）。
	pageCard *settingsItemCardState
}

// settingsVoiceHarnessOptions 是 harness 的入口参数。
type settingsVoiceHarnessOptions struct {
	width      int
	capability pluginapi.VoiceCapability
	state      pluginapi.VoiceState
	// unwired 为真 ⇒ `Act.Voice == nil`（老壳/最小装配）。
	unwired bool
	// card 非 nil 时**直接画这一页表单**（不经过卡片/详情两层），用来钉住
	// "有数据才画"的绘制路径——例如官方 `.deviceError`（本仓没有数据源，
	// 见下文的说明）。
	card *settingsItemCardState
}

func openSettingsVoiceHarness(t *testing.T, options settingsVoiceHarnessOptions) *settingsVoiceHarness {
	t.Helper()
	width := options.width
	if width <= 0 {
		width = 960
	}
	h := &settingsVoiceHarness{
		t: t, width: width, wire: !options.unwired,
		capability: options.capability, state: options.state, pageCard: options.card,
	}
	h.p = &Plugin{}
	kernel := uikernel.New("settings-voice")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, h.p); err != nil {
		t.Fatal(err)
	}
	h.theme = kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	act := pluginapi.Actions{
		// 这一页**不依赖** settings.describe（官方来自 `plugins.bundle.config` 槽）：
		// 这里故意给 nil，证明卡片的在场只由 Act.Voice 决定。
		SettingsDescribe: func() *apiclient.SettingsDescribeResult { return nil },
		PluginBundles: func() ([]apiclient.PluginBundle, bool, bool, string) {
			return nil, true, false, ""
		},
	}
	if h.wire {
		act.Voice = &pluginapi.VoiceActions{
			Capability:     h.capabilityNow,
			State:          h.stateNow,
			SelectDevice:   h.selectDevice,
			RefreshDevices: h.refreshDevices,
		}
	}
	h.tree = &gfx.Tree{}
	h.feat = h.tree.Feature("settings-voice")
	pageTheme := dscore.VivaTheme(h.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "settings-voice", Width: width, Height: 1100,
		Tree: h.tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: h.theme, Tree: h.tree, Feat: h.feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
			Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, MainWidth: size.Width},
			Act:      act, Has: kernel.Has,
		}
		canvas.Fill(bounds, h.theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			if h.pageCard != nil {
				h.p.drawSettingsItemForm(f, env, h.theme, *h.pageCard)
				return
			}
			h.p.drawPluginManagerPage(f, env)
		})
	}
	config.OnKey = func(ev gfx.KeyEvent) gfx.EventResult { return gfx.EventResult{} }
	h.invalidator = &app.Invalidator{}
	config.Invalidator = h.invalidator
	config.TextInput = ui.Fields(h.p.Fields()...)
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	h.session = session
	h.pump()
	h.pump()
	return h
}

func (h *settingsVoiceHarness) capabilityNow() pluginapi.VoiceCapability {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.capability
}

func (h *settingsVoiceHarness) stateNow() pluginapi.VoiceState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

// selectDevice 与真壳同一个语义：只影响下一次 Begin，并立刻改选中项后重画
// （cmd/app/speech_voice.go:461-476 的 voiceSelectDevice + Invalidate）。
func (h *settingsVoiceHarness) selectDevice(id int) {
	h.mu.Lock()
	h.selects = append(h.selects, id)
	h.capability.DeviceID = id
	h.mu.Unlock()
	h.invalidator.Invalidate()
}

func (h *settingsVoiceHarness) refreshDevices() {
	h.mu.Lock()
	h.refreshes++
	h.mu.Unlock()
	h.invalidator.Invalidate()
}

func (h *settingsVoiceHarness) setState(state pluginapi.VoiceState) {
	h.mu.Lock()
	h.state = state
	h.mu.Unlock()
	h.invalidator.Invalidate()
}

func (h *settingsVoiceHarness) selectCalls() []int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]int(nil), h.selects...)
}

func (h *settingsVoiceHarness) refreshCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.refreshes
}

func (h *settingsVoiceHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *settingsVoiceHarness) close() { h.session.Close() }

func (h *settingsVoiceHarness) hitFor(name string) gfx.Hit {
	h.t.Helper()
	id := h.tree.ID(h.feat, name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit
		}
	}
	h.t.Fatalf("没有命中区 %q（控件没画出来或没挂 Hit）", name)
	return gfx.Hit{}
}

func (h *settingsVoiceHarness) hasHit(name string) bool {
	id := h.tree.ID(h.feat, name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return true
		}
	}
	return false
}

func (h *settingsVoiceHarness) clickAt(bounds gfx.Rect, label string) {
	h.t.Helper()
	x, y := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
	snapshot := h.session.Snapshot()
	if x < 0 || y < 0 || x >= float32(snapshot.Width) || y >= float32(snapshot.Height) {
		h.t.Fatalf("%s 的中心 (%v,%v) 在窗口 %dx%d 之外", label, x, y, snapshot.Width, snapshot.Height)
	}
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

func (h *settingsVoiceHarness) click(name string) {
	h.t.Helper()
	h.clickAt(h.hitFor(name).Bounds, name)
}

// clickText 点某段画出来的文字（viva 菜单行没有 tree id，只能按文字定位）。
func (h *settingsVoiceHarness) clickText(label string) {
	h.t.Helper()
	for _, text := range h.session.Snapshot().Texts {
		if text.Value == label {
			h.clickAt(text.Bounds, label)
			return
		}
	}
	h.t.Fatalf("没有画文字 %q；texts=%q", label, h.session.Texts())
}

// clickTextOutside 点某段文字里**落在 exclude 之外**的那一次出现。
//
// 为什么需要它：不可用行的文字会**同时**出现在 select 的当前值里（官方原生 select
// 把 option 文字画在收起态）和展开的菜单行里；只按文字找会点到 select 自己，
// 那一按只是把菜单关掉，于是"不可用项选不动"变成一条永远为真的假断言。
func (h *settingsVoiceHarness) clickTextOutside(label string, exclude gfx.Rect) {
	h.t.Helper()
	for _, text := range h.session.Snapshot().Texts {
		if text.Value != label {
			continue
		}
		bounds := text.Bounds
		if bounds.X >= exclude.X-1 && bounds.X+bounds.Width <= exclude.X+exclude.Width+1 &&
			bounds.Y >= exclude.Y-1 && bounds.Y+bounds.Height <= exclude.Y+exclude.Height+1 {
			continue
		}
		h.clickAt(bounds, label)
		return
	}
	h.t.Fatalf("没有画在 select 之外的 %q（菜单没展开？）；texts=%q", label, h.session.Texts())
}

// textBounds 取某段画出来的文字的矩形（label 没有命中区，只能从 Texts 里拿）。
func (h *settingsVoiceHarness) textBounds(value string) (gfx.Rect, bool) {
	for _, text := range h.session.Snapshot().Texts {
		if text.Value == value {
			return text.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

func (h *settingsVoiceHarness) hasText(want string) bool { return h.session.HasText(want) }

func (h *settingsVoiceHarness) openVoiceCard() {
	h.t.Helper()
	h.click("settings-item-card-" + settingsVoiceCardID)
	if h.p.itemSelected != settingsVoiceCardID {
		h.t.Fatalf("点卡片后 itemSelected=%q，want %q", h.p.itemSelected, settingsVoiceCardID)
	}
}

func (h *settingsVoiceHarness) capture(name string) {
	dir := os.Getenv("SETTINGS_ITEMS_CAPTURE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := h.session.Screenshot(filepath.Join(dir, name)); err != nil {
		h.t.Fatal(err)
	}
}

// voiceMeterBars 从绘制原语里挑出「3×12、r1」的 bar（官方 .inputLevel>span），
// 并限定在 select 的矩形里（不把别处的 3×12 圆角块算进来）。
func voiceMeterBars(snapshot app.Snapshot, within gfx.Rect) []app.Primitive {
	bars := make([]app.Primitive, 0, voiceLevelBars)
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.RectanglePrimitive {
			continue
		}
		bounds := primitive.Bounds
		if bounds.Width != voiceLevelBarWidth || bounds.Height != voiceLevelBarHeight ||
			primitive.Radius != voiceLevelBarRadius {
			continue
		}
		if bounds.X < within.X-1 || bounds.X+bounds.Width > within.X+within.Width+1 ||
			bounds.Y < within.Y-1 || bounds.Y+bounds.Height > within.Y+within.Height+1 {
			continue
		}
		bars = append(bars, primitive)
	}
	return bars
}

// TestSettingsVoicePageDrawsOfficialPicker 是「这一页真的画出来了」的主证据。
func TestSettingsVoicePageDrawsOfficialPicker(t *testing.T) {
	h := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{
		capability: settingsVoiceCapabilityFixture(),
		state:      settingsVoiceStateFixture("idle", 0),
	})
	defer h.close()

	// 卡片：标题/摘要逐字来自官方 bundle 元数据。
	if !h.hasText(zhVoiceCardTitle) || !h.hasText(zhVoiceCardSummary) {
		t.Fatalf("插件页缺 #66 卡片文案；texts=%q", h.session.Texts())
	}
	h.openVoiceCard()
	for _, want := range []string{zhVoiceInputDevice, zhVoiceSystemMicrophone} {
		if !h.hasText(want) {
			t.Fatalf("详情页缺官方文案 %q；texts=%q", want, h.session.Texts())
		}
	}
	// 真实设备名逐字（官方 device.label）：在**展开的菜单**里（官方 select 收起时
	// 只显示选中项，MicrophonePicker.tsx:127 的 value）。
	h.click(settingsVoiceDeviceDOMID)
	for _, want := range []string{"麦克风阵列 (Realtek(R) Audio)", "USB 麦克风"} {
		if !h.hasText(want) {
			t.Fatalf("下拉里缺设备名 %q；texts=%q", want, h.session.Texts())
		}
	}
	h.capture("settings-voice-menu.png")
	// select 是真的（有命中区），几何照官方：高 = 2×6 + 19.5 + 2×0.5 = 32.5，
	// 宽落在 minmax(120px,280px) 里（css:30-31）。判据用**官方字面量**，不用实现里的
	// 常量——否则把常量改坏时断言跟着变，等于没断言。
	selectHit := h.hitFor(settingsVoiceDeviceDOMID)
	if got := selectHit.Bounds.Height; got < 31.5 || got > 33.5 {
		t.Fatalf("select 高度 = %v（官方 2×6 + 19.5 + 2×0.5 = 32.5，css:31）", got)
	}
	if selectHit.Bounds.Width < 119 || selectHit.Bounds.Width > 281 {
		t.Fatalf("select 宽度 = %v（官方 minmax(120px,280px)，css:30）", selectHit.Bounds.Width)
	}
	// 官方这一页没有保存按钮（选设备立即生效，MicrophonePicker.tsx:128-131）。
	if h.hasHit("settings-item-save-" + settingsVoiceCardID) {
		t.Fatal("语音输入页不该画保存按钮（官方 selectMicrophone 立即生效）")
	}
	// 打开页面即重新枚举一次设备（官方 :104-111 的 refresh()）。
	if h.refreshCount() == 0 {
		t.Fatal("进入页面没有触发 RefreshDevices（官方挂载即 refresh）")
	}
	h.capture("settings-voice-detail.png")
}

// TestSettingsVoiceAbsentWithoutShellVoice 钉住「`Act.Voice == nil` ⇒ 整页不画」。
func TestSettingsVoiceAbsentWithoutShellVoice(t *testing.T) {
	h := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{
		capability: settingsVoiceCapabilityFixture(),
		unwired:    true,
	})
	defer h.close()
	if h.hasText(zhVoiceCardTitle) {
		t.Fatalf("壳没接 Act.Voice 却画了 #66 卡片；texts=%q", h.session.Texts())
	}
	if h.hasHit("settings-item-card-" + settingsVoiceCardID) {
		t.Fatal("壳没接 Act.Voice 却挂了卡片命中区")
	}
	if h.hasHit(settingsVoiceDeviceDOMID) {
		t.Fatal("壳没接 Act.Voice 却画了麦克风 select")
	}
}

// TestSettingsVoiceSelectDeviceCallsShell 是「换设备真的调壳」的证据。
func TestSettingsVoiceSelectDeviceCallsShell(t *testing.T) {
	h := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{
		capability: settingsVoiceCapabilityFixture(),
		state:      settingsVoiceStateFixture("idle", 0),
	})
	defer h.close()
	h.openVoiceCard()
	h.click(settingsVoiceDeviceDOMID)
	if !h.hasText("USB 麦克风") {
		t.Fatalf("菜单没展开；texts=%q", h.session.Texts())
	}
	h.clickText("USB 麦克风")
	calls := h.selectCalls()
	if len(calls) != 1 || calls[0] != 1 {
		t.Fatalf("SelectDevice 调用 = %v（want [1]）", calls)
	}
	// 壳改选中项后，select 上显示的就是新设备名。
	selectHit := h.hitFor(settingsVoiceDeviceDOMID)
	if selectHit.Label != "USB 麦克风" {
		t.Fatalf("select 命中区 label = %q（want 新选中项 USB 麦克风）", selectHit.Label)
	}
	h.capture("settings-voice-selected.png")
}

// TestSettingsVoiceUnavailableRowIsDisabled 钉住「` — 不可用` + 不可选」。
//
// 场景：选中的设备（ID=7）已从名录里消失 ⇒ 官方保留该行并标不可用（:91-99/:133），
// 菜单里这一行画出来但选不动（`disabled={device.unavailable}`，:132）。
func TestSettingsVoiceUnavailableRowIsDisabled(t *testing.T) {
	capability := settingsVoiceCapabilityFixture()
	capability.Devices = []pluginapi.VoiceDevice{{ID: 1, Name: "USB 麦克风"}}
	capability.DeviceID = 7
	h := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{
		capability: capability,
		state:      settingsVoiceStateFixture("idle", 0),
	})
	defer h.close()
	h.openVoiceCard()
	if !h.hasText("麦克风 1 — 不可用") {
		t.Fatalf("失效的当前项没有 ` — 不可用`；texts=%q", h.session.Texts())
	}
	h.click(settingsVoiceDeviceDOMID)
	if !h.hasText("麦克风 1 — 不可用") {
		t.Fatalf("菜单里没有失效行；texts=%q", h.session.Texts())
	}
	// 点**菜单里**那一行（不是 select 自己显示的那份），官方 option 是 disabled。
	h.clickTextOutside("麦克风 1 — 不可用", h.hitFor(settingsVoiceDeviceDOMID).Bounds)
	if calls := h.selectCalls(); len(calls) != 0 {
		t.Fatalf("点了不可用项却调了 SelectDevice：%v（官方 option 是 disabled）", calls)
	}
	h.capture("settings-voice-unavailable.png")
}

// TestSettingsVoiceMeterUsesRealLevelAndOfficialGeometry 是电平条的**真绘制**证据：
//
//	几何：8 根 3×12、r1 的 bar，节距 6（= 3 + gap 3），右边缘距 select 右边 24px，
//	      垂直居中（css:46/48-49）；
//	颜色：点亮的用 `--dsw-alias-state-success-primary`，其余用 `--dsw-alias-border-l1`（css:49-50）；
//	语义：只有 phase=recording（真实电平）才画；idle 时一根都不画。
func TestSettingsVoiceMeterUsesRealLevelAndOfficialGeometry(t *testing.T) {
	h := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{
		capability: settingsVoiceCapabilityFixture(),
		state:      settingsVoiceStateFixture("recording", 0.01),
	})
	defer h.close()
	h.openVoiceCard()
	selectHit := h.hitFor(settingsVoiceDeviceDOMID)
	bars := voiceMeterBars(h.session.Snapshot(), selectHit.Bounds)
	if len(bars) != 8 {
		t.Fatalf("仪表画了 %d 根 bar（官方 8 根）；select=%+v", len(bars), selectHit.Bounds)
	}
	// 节距 = 3 + 3 = 6（官方 css:48-49）；右边缘距 select 右边 24（css:46）。
	// 判据一律写**官方字面量**，避免"断言跟着实现常量走"的假绿。
	right := selectHit.Bounds.X + selectHit.Bounds.Width
	for index := 1; index < len(bars); index++ {
		step := bars[index].Bounds.X - bars[index-1].Bounds.X
		if step < 5 || step > 7 {
			t.Fatalf("第 %d 根与上一根的水平间隔 = %v（官方 3px bar + 3px gap = 6）", index, step)
		}
	}
	last := bars[len(bars)-1]
	if got := right - (last.Bounds.X + last.Bounds.Width); got < 23 || got > 25 {
		t.Fatalf("仪表右边缘距 select 右边 = %v（官方 .deviceMeter{right:24px}，css:46）", got)
	}
	// 垂直居中：bar 中心 = select 中心。
	barCenter := last.Bounds.Y + last.Bounds.Height/2
	selectCenter := selectHit.Bounds.Y + selectHit.Bounds.Height/2
	if barCenter < selectCenter-1 || barCenter > selectCenter+1 {
		t.Fatalf("仪表中心 y = %v（select 中心 %v，官方 top:50%% + translateY(-50%%)）", barCenter, selectCenter)
	}
	// 点亮的根数 == 官方公式手算值（0.01 → -40dB → ceil(0.3333×8) = 3 根）。
	active := 0
	for _, bar := range bars {
		if bar.Color == h.theme.SuccessPrimary {
			active++
		} else if bar.Color != h.theme.BorderL1 {
			t.Fatalf("bar 颜色 %+v 既不是 success-primary 也不是 border-l1（css:49-50）", bar.Color)
		}
	}
	if active != 3 {
		t.Fatalf("点亮 %d 根（level=0.01 的官方公式值 = 3）", active)
	}
	h.capture("settings-voice-meter.png")

	// idle：没有真实电平 ⇒ 一根 bar 都不画（不做空闲动画、不编 0 值预览）。
	h.setState(settingsVoiceStateFixture("idle", 0.01))
	h.pump()
	h.pump()
	selectHit = h.hitFor(settingsVoiceDeviceDOMID)
	if bars := voiceMeterBars(h.session.Snapshot(), selectHit.Bounds); len(bars) != 0 {
		t.Fatalf("idle 阶段画了 %d 根 bar（本仓没有预览采集，必须 0）", len(bars))
	}
}

// TestSettingsVoiceMeterReservesOfficialSeventyTwoPixelPadding 钉住
// `.deviceField select.deviceSelectMeter{padding-right:72px}`（css:45）：**有仪表时**
// 选中项文字必须让出右侧 72px 再截断，没有仪表时只让 10px（select 的 padding-inline）。
//
// 判据用一段长得必然要截断的设备名量出来，而不是读常量：
// 有仪表 ⇒ 文字可用宽 ≤ 280 − 10 − 72 = 198；无仪表 ⇒ ≈ 280 − 10 − 10 = 260。
func TestSettingsVoiceMeterReservesOfficialSeventyTwoPixelPadding(t *testing.T) {
	longName := "超长名字的虚拟录音设备（用来验证 72px 让位规则的第十七个字符之后必然截断）"
	capability := pluginapi.VoiceCapability{
		Available: true,
		Devices:   []pluginapi.VoiceDevice{{ID: 0, Name: longName}},
		DeviceID:  0,
	}
	h := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{
		capability: capability,
		state:      settingsVoiceStateFixture("recording", 0.5),
	})
	defer h.close()
	h.openVoiceCard()
	selectHit := h.hitFor(settingsVoiceDeviceDOMID)
	withMeter := voiceTextWidth(h.session.Snapshot(), selectHit.Bounds)
	if withMeter <= 0 {
		t.Fatalf("select 里没找到选中项文字；texts=%q", h.session.Texts())
	}
	if withMeter > 200 {
		t.Fatalf("有仪表时选中项文字宽 %v（该让出 72px：≤ 280−10−72 = 198）", withMeter)
	}
	// 关掉仪表（idle ⇒ 没有真实电平）：右内缩回到 select 的 padding-inline 10px。
	h.setState(settingsVoiceStateFixture("idle", 0))
	h.pump()
	h.pump()
	selectHit = h.hitFor(settingsVoiceDeviceDOMID)
	withoutMeter := voiceTextWidth(h.session.Snapshot(), selectHit.Bounds)
	if withoutMeter <= withMeter {
		t.Fatalf("无仪表时文字宽 %v 不大于有仪表时的 %v（72px 让位规则没生效）",
			withoutMeter, withMeter)
	}
	if withoutMeter > 262 {
		t.Fatalf("无仪表时文字宽 %v（该只让 padding-inline 10px：≤ 280−10−10 = 260）", withoutMeter)
	}
}

// voiceTextWidth 返回落在 rect 里的最宽文字（select 收起态的当前值）。
func voiceTextWidth(snapshot app.Snapshot, rect gfx.Rect) float32 {
	width := float32(0)
	for _, text := range snapshot.Texts {
		bounds := text.Bounds
		if bounds.Y < rect.Y-2 || bounds.Y+bounds.Height > rect.Y+rect.Height+2 ||
			bounds.X < rect.X-2 || bounds.X >= rect.X+rect.Width {
			continue
		}
		if bounds.Width > width {
			width = bounds.Width
		}
	}
	return width
}

// TestSettingsVoiceDeviceErrorRowKeepsOfficialColumn 钉住官方 `.deviceError`
// （MicrophonePicker.tsx:138 / css:47）的版式：它在 `.deviceRow` 网格的**第 2 列**，
// 也就是从 100px 列 + 12px gap 之后起画；≤480px 断点改到第 1 列（css:54）。
//
// **为什么这条要手造状态**：官方那一行来自"预览采集失败"（`:49-52`），而本仓
// `VoiceActions` 没有预览入口（账本 #66 已登记的差异）⇒ 生产路径永远不产生它。
// 这条测试钉的是"有数据时按官方版式画"，不是"假装壳会报错"。
func TestSettingsVoiceDeviceErrorRowKeepsOfficialColumn(t *testing.T) {
	// 官方 `deviceMissing`（locales.ts:92）：麦克风不可用，请选择其他输入设备。
	const message = "麦克风不可用，请选择其他输入设备"
	voice := &settingsVoiceState{
		Choices:     settingsVoiceChoices(settingsVoiceCapabilityFixture()),
		Selected:    settingsVoiceSystemDeviceID,
		Value:       zhVoiceSystemMicrophone,
		Selectable:  true,
		DeviceError: message,
	}
	card := settingsItemCardState{Item: settingsVoiceItem(), Known: true, Writable: true, Voice: voice}

	wide := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{width: 960, card: &card})
	defer wide.close()
	errorBounds, ok := wide.textBounds(message)
	if !ok {
		t.Fatalf(".deviceError 那一行没画出来；texts=%q", wide.session.Texts())
	}
	selectHit := wide.hitFor(settingsVoiceDeviceDOMID)
	// 官方 `.deviceError` 与 `.deviceField` 同在网格第 2 列 ⇒ 左边缘对齐控件左边缘。
	if delta := errorBounds.X - selectHit.Bounds.X; delta < -1 || delta > 1 {
		t.Fatalf(".deviceError 起点 %v 没对齐控件左边缘 %v（官方 grid-column:2 ⇒ 第 2 列靠左，css:47）；error=%+v",
			errorBounds.X, selectHit.Bounds.X, errorBounds)
	}
	// 它必须在控件**下面**（网格第二行），不是同一行。
	if errorBounds.Y < selectHit.Bounds.Y+selectHit.Bounds.Height-1 {
		t.Fatalf(".deviceError 不在控件下方（error=%+v select=%+v）", errorBounds, selectHit.Bounds)
	}
	wide.capture("settings-voice-error.png")

	narrow := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{width: 420, card: &card})
	defer narrow.close()
	errorBounds, ok = narrow.textBounds(message)
	if !ok {
		t.Fatalf("窄版 .deviceError 那一行没画出来；texts=%q", narrow.session.Texts())
	}
	selectHit = narrow.hitFor(settingsVoiceDeviceDOMID)
	// 断点内 `grid-column:1`（css:54）⇒ 与控件同列靠左。
	if delta := errorBounds.X - selectHit.Bounds.X; delta < -1 || delta > 1 {
		t.Fatalf("窄版 .deviceError 起点 %v 没对齐控件 %v（官方断点内 grid-column:1，css:54）",
			errorBounds.X, selectHit.Bounds.X)
	}
}

// TestSettingsVoiceNarrowBreakpointStacks 钉住官方 ≤480px 断点（css:51-55）：
// 宽版 label 与控件同一行、label 占 100px 列；窄版单列上下叠、gap 6。
func TestSettingsVoiceNarrowBreakpointStacks(t *testing.T) {
	wide := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{
		width:      960,
		capability: settingsVoiceCapabilityFixture(),
		state:      settingsVoiceStateFixture("idle", 0),
	})
	defer wide.close()
	wide.openVoiceCard()
	label, ok := wide.textBounds(zhVoiceInputDevice)
	if !ok {
		t.Fatalf("宽版没有 `%s` 文字", zhVoiceInputDevice)
	}
	selectHit := wide.hitFor(settingsVoiceDeviceDOMID)
	// 官方 100px 列 + 12px gap = 112；字面量判据（不跟实现常量走）。
	if delta := selectHit.Bounds.X - label.X; delta < 110 || delta > 114 {
		t.Fatalf("宽版：控件左边缘距 label 左边缘 %v（官方 100 + 12 = 112，css:30）", delta)
	}
	if selectHit.Bounds.Y > label.Y+label.Height {
		t.Fatalf("宽版：控件跑到 label 下一行了（select.Y=%v label=%+v）", selectHit.Bounds.Y, label)
	}

	narrow := openSettingsVoiceHarness(t, settingsVoiceHarnessOptions{
		width:      420,
		capability: settingsVoiceCapabilityFixture(),
		state:      settingsVoiceStateFixture("idle", 0),
	})
	defer narrow.close()
	narrow.openVoiceCard()
	label, ok = narrow.textBounds(zhVoiceInputDevice)
	if !ok {
		t.Fatalf("窄版没有 `%s` 文字", zhVoiceInputDevice)
	}
	selectHit = narrow.hitFor(settingsVoiceDeviceDOMID)
	if selectHit.Bounds.Y < label.Y+label.Height-1 {
		t.Fatalf("窄版：控件没有落到 label 下一行（select.Y=%v label=%+v）",
			selectHit.Bounds.Y, label)
	}
	if gap := selectHit.Bounds.Y - (label.Y + label.Height); gap > 14 {
		t.Fatalf("窄版：label 与控件的行距 %v 太大（官方 480 断点 gap:6px，css:52）", gap)
	}
	if selectHit.Bounds.Width < 160 {
		t.Fatalf("窄版：控件宽 %v（官方单列 minmax(0,1fr)，应占满一行）", selectHit.Bounds.Width)
	}
	narrow.capture("settings-voice-narrow.png")
}
