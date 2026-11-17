//go:build darwin || windows || linux

// voice_test.go —— 听写控件的**真实像素**判据（viva headless 驱动）。
//
// 判据不是"画出来了"，而是官方几何与官方文案：
//
//	① 挂在 `conversation.input.activity`（官方 mount.ts 的唯一视线注册）；
//	② idle + 不可用 → **什么都不画**（本仓没有"去安装模型"这条路可走）；
//	③ idle + 可用 → 28px 宽的触发钮，里面是官方 `<rect x=5 y=1.5 w=6 h=9 rx=3>`
//	   胶囊（18px 显示下 6.75×10.125、圆角 3.375）；
//	④ recording → 80 根波形柱 + 32px 停止圆钮，柱高按官方公式
//	   `1 + min(1, level*5) * 17` 换算到 24px 高；
//	⑤ requesting / transcribing / feedback 三段文案逐字取官方 zh 词典。
package dsvoice

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// voiceStub 是 `VoiceActions` 的可控替身。
type voiceStub struct {
	capability pluginapi.VoiceCapability
	state      pluginapi.VoiceState
	began      int
	stopped    int
	cancelled  int
	retried    int
	devices    []int
}

func (v *voiceStub) actions() *pluginapi.VoiceActions {
	return &pluginapi.VoiceActions{
		Capability: func() pluginapi.VoiceCapability { return v.capability },
		State:      func() pluginapi.VoiceState { return v.state },
		Begin:      func() { v.began++ },
		Stop:       func() { v.stopped++ },
		Cancel:     func() { v.cancelled++ },
		Retry:      func() { v.retried++ },
		Insert:     func() {},
		SelectDevice: func(id int) {
			v.devices = append(v.devices, id)
		},
		RefreshDevices: func() {},
	}
}

// paintVoice 把控件画一帧，交回 headless 快照。
//
// 宽度给 640：官方控件行扣掉取消/停止两枚 32px 圆钮与两段 12px 间距之后，
// 波形还有 552px 可占——这样"波形铺满剩余宽度"才测得出来。
func paintVoice(t *testing.T, stub *voiceStub) (*headless.Session, *app.Driver) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme}
	tree := &gfx.Tree{}
	feat := tree.Feature("voice-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "voice-input", Width: 640, Height: 120, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Act: pluginapi.Actions{Voice: stub.actions()},
			Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Row(f).FillWidth().Height(34).Pad(8).Children(func() {
				p.draw(f, env)
			})
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	return session, session.Driver()
}

func availableStub(phase string, level float64) *voiceStub {
	return &voiceStub{
		capability: pluginapi.VoiceCapability{
			Available:  true,
			Providers:  []pluginapi.VoiceProvider{{ID: "MS-2052-80-DESK", Name: "MS-2052-80-DESK", Language: "zh-CN"}},
			ProviderID: "MS-2052-80-DESK",
			Language:   "zh-CN",
			Devices:    []pluginapi.VoiceDevice{{ID: 0, Name: "麦克风 (花再 Halo PixelBar)"}},
			DeviceID:   -1,
			MaxSeconds: 60,
		},
		state: pluginapi.VoiceState{Phase: phase, Level: level},
	}
}

func TestPanelsRegistersTheOfficialActivitySeat(t *testing.T) {
	p := &Plugin{}
	panels := p.Panels()
	if len(panels) != 1 {
		t.Fatalf("panels = %d，期望 1", len(panels))
	}
	if panels[0].Slot != uikernel.OffInputActivity {
		t.Fatalf("slot = %v，期望 conversation.input.activity", panels[0].Slot)
	}
	if string(uikernel.OffInputActivity) != "conversation.input.activity" {
		t.Fatalf("槽名字面量 = %q", uikernel.OffInputActivity)
	}
}

// TestPluginOccupiesTheActivitySeatInAKernel：不只是"Panels() 声明的槽对"，
// 而是**装进真内核之后**那个槽上真的有占用方——dscomposer 的
// `composerDispatchTo` 判的就是 `env.Has(OffInputActivity)`（`composer.go:427`），
// 声明对而装配没走到，界面上会静默回退成它自己的只读活动 chip。
func TestPluginOccupiesTheActivitySeatInAKernel(t *testing.T) {
	kernel := uikernel.New("voice-seat")
	kernel.Root().Provide(pluginapi.KeyTheme, dscore.Dark())
	if err := kernel.Load(&Plugin{}); err != nil {
		t.Fatalf("装配 dsvoice 失败：%v", err)
	}
	if !kernel.Has(uikernel.OffInputActivity) {
		t.Fatal("conversation.input.activity 上没有占用方：composer 会回退画它自己的活动 chip")
	}
	if got := kernel.Painters(uikernel.OffInputActivity); len(got) != 1 || got[0].ID != "voice-input" {
		t.Fatalf("座位上的 painter = %+v", got)
	}
}

func TestUnavailableDrawsNoEntryAtAll(t *testing.T) {
	stub := &voiceStub{capability: pluginapi.VoiceCapability{Reason: "这台机器没有录音设备"}}
	session, driver := paintVoice(t, stub)
	// 只有底色与容器，没有图标三角形、没有圆钮矩形。
	for _, primitive := range session.Driver().Snapshot().Primitives {
		if primitive.Kind == app.TrianglePrimitive {
			t.Fatalf("不可用时不该画任何矢量图元：%+v", primitive.Bounds)
		}
		if primitive.Radius > 0 && primitive.Bounds.Width <= roundButton &&
			primitive.Bounds.Height <= roundButton {
			t.Fatalf("不可用时不该画圆钮：%+v", primitive.Bounds)
		}
	}
	_ = driver
}

func TestTriggerPaintsOfficialMicGlyphAndCallsBegin(t *testing.T) {
	stub := availableStub("idle", 0)
	session, driver := paintVoice(t, stub)

	ink, ok := inkBounds(session.Driver().Snapshot().Primitives)
	if !ok {
		t.Fatal("idle 态一枚矢量图元都没有：麦克风图标没画出来")
	}
	// 官方麦克风在 16 viewBox 里的墨迹：x 2.35-0.5 .. 13.65+0.5，y 1.5-0.5 .. 15+0.5。
	// 18px 显示时约 12.4 × 15.2 —— 只判"落在 18×18 内且不是芝麻大"，
	// 因为胶囊是原生圆角矩形、括号是 path，两者合成一个包围盒。
	if ink.Width > 20 || ink.Height > 20 {
		t.Fatalf("图标墨迹 %.2f×%.2f 超出 18px 显示框：%+v", ink.Width, ink.Height, ink)
	}
	if ink.Width < 10 || ink.Height < 12 {
		t.Fatalf("图标墨迹 %.2f×%.2f 太小：%+v", ink.Width, ink.Height, ink)
	}

	cx := ink.X + ink.Width/2
	cy := ink.Y + ink.Height/2
	if _, hit := driver.HitTest(cx, cy); !hit {
		t.Fatalf("(%.1f,%.1f) 没有命中目标：按钮不可点", cx, cy)
	}
	if err := driver.Click(cx, cy); err != nil {
		t.Fatal(err)
	}
	if stub.began != 1 {
		t.Fatalf("Begin 调用 %d 次，期望 1", stub.began)
	}
	_ = session
}

func TestRecordingRowPaintsWaveformStopAndCancel(t *testing.T) {
	stub := availableStub("recording", 1)
	session, _ := paintVoice(t, stub)
	snapshot := session.Driver().Snapshot()

	// 两枚 32px 圆钮（取消 + 停止），底色 --dsw-specific-selector。
	rounds := 0
	for _, p := range snapshot.Primitives {
		if p.Kind != app.RectanglePrimitive {
			continue
		}
		if near(p.Bounds.Width, roundButton) && near(p.Bounds.Height, roundButton) &&
			near(p.Radius, float32(gfx.RadiusFull)) {
			rounds++
		}
	}
	if rounds < 2 {
		t.Fatalf("圆钮 %d 枚，期望 ≥2（取消 + 停止）", rounds)
	}

	// 波形：80 根柱（节距 8 / 线宽 3）；满电平那根半高 = 1+17 = 18，
	// 换算到 24px 高是 2*18*24/40 = 21.6；静音那根压到柱宽 3。
	bars := 0
	tallest := float32(0)
	thin := 0
	for _, p := range snapshot.Primitives {
		if p.Kind != app.RectanglePrimitive {
			continue
		}
		if near(p.Bounds.Width, waveformBarWidth) && near(p.Radius, waveformBarWidth/2) {
			bars++
			if p.Bounds.Height > tallest {
				tallest = p.Bounds.Height
			}
			if near(p.Bounds.Height, waveformBarWidth) {
				thin++
			}
		}
	}
	if bars < waveformBars {
		t.Fatalf("波形柱 %d 根，官方是 %d 根", bars, waveformBars)
	}
	wantTall := 2 * (waveformHalfBase + waveformHalfRange) * waveformHeight / waveformViewBoxH
	// 栅格化会取整到像素，容差 1px。
	if tallest < wantTall-1 || tallest > wantTall+1 {
		t.Fatalf("最高波形柱 = %.3f，官方公式算出来是 %.3f", tallest, wantTall)
	}
	if thin == 0 {
		t.Fatal("静音柱应该压在柱宽 3px，一根都没有")
	}

	// 文字不该出现：录音态画的是波形，不是状态文案。
	for _, text := range snapshot.Texts {
		if text.Value == "识别中…" || text.Value == "请允许使用麦克风…" {
			t.Fatalf("录音态不该画状态文案 %q", text.Value)
		}
	}
}

func TestPhaseTextsAreOfficialChinese(t *testing.T) {
	cases := []struct{ phase, message, want string }{
		{"requesting", "", "请允许使用麦克风…"},
		{"transcribing", "", "识别中…"},
		{"feedback", "未识别到语音", "未识别到语音"},
		{"feedback", "语音识别失败：audioBase64 不是合法 base64", "语音识别失败：audioBase64 不是合法 base64"},
	}
	for _, c := range cases {
		stub := availableStub(c.phase, 0)
		stub.state.Message = c.message
		session, _ := paintVoice(t, stub)
		found := false
		for _, text := range session.Driver().Snapshot().Texts {
			if text.Value == c.want {
				found = true
				break
			}
		}
		if !found {
			var got []string
			for _, text := range session.Driver().Snapshot().Texts {
				got = append(got, text.Value)
			}
			t.Fatalf("phase=%q 找不到文案 %q，实际画了 %q", c.phase, c.want, got)
		}
	}
}

// TestFeedbackPaintsRetryButton：feedback 态要有一枚**重新录音**的圆钮（官方
// VoiceInput.tsx:178-181），不是只有一行字。
func TestFeedbackPaintsRetryButton(t *testing.T) {
	stub := availableStub("feedback", 0)
	stub.state.Message = "未识别到语音"
	session, _ := paintVoice(t, stub)
	// feedback 态：取消圆钮 + 重试圆钮，没有停止圆钮。
	rounds := 0
	for _, p := range session.Driver().Snapshot().Primitives {
		if p.Kind == app.RectanglePrimitive && near(p.Bounds.Width, roundButton) &&
			near(p.Bounds.Height, roundButton) && near(p.Radius, float32(gfx.RadiusFull)) {
			rounds++
		}
	}
	if rounds != 2 {
		t.Fatalf("feedback 态圆钮 %d 枚，期望 2（取消 + 重新录音）", rounds)
	}
}

func near(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= 0.05
}

// inkBounds 求这一帧所有矢量三角形的墨迹包围盒。
//
// 为什么不用 RectPrimitive：viva 把**描边**（Border/Stroke）与 path 图标都
// 三角化成 TrianglePrimitive，它们的 Bounds 是零；真实位置在 Points 里。
// dsgoal 的视觉测试（`plugins/dsgoal/goal_visual_test.go:96-124`）同一套算法。
func inkBounds(primitives []app.Primitive) (gfx.Rect, bool) {
	var out gfx.Rect
	found := false
	for _, p := range primitives {
		if p.Kind != app.TrianglePrimitive {
			continue
		}
		for _, pt := range p.Points {
			if !found {
				out = gfx.Rect{X: pt.X, Y: pt.Y}
				found = true
				continue
			}
			if pt.X < out.X {
				out.Width += out.X - pt.X
				out.X = pt.X
			}
			if pt.Y < out.Y {
				out.Height += out.Y - pt.Y
				out.Y = pt.Y
			}
			if pt.X > out.X+out.Width {
				out.Width = pt.X - out.X
			}
			if pt.Y > out.Y+out.Height {
				out.Height = pt.Y - out.Y
			}
		}
	}
	return out, found
}

func countTriangles(primitives []app.Primitive) int {
	n := 0
	for _, p := range primitives {
		if p.Kind == app.TrianglePrimitive {
			n++
		}
	}
	return n
}
