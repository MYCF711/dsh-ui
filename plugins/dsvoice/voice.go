// Package dsvoice 是官方 `dsh-experimental-client-ui-voice-input` 的本仓实现：
// composer 控件行里的**麦克风听写**。
//
// # 官方挂在哪里（逐字）
//
// `OFF:dsh-experimental-client-ui-voice-input/src/client/mount.ts` 里只有三处注册，
// 视线这一条是本包的全部：
//
//	ctx.slots.inject('conversation.input.activity', () => ctx.slots.register({
//	  name: 'conversation.input.activity', locale: NS, inject: () => actions,
//	}, VoiceInput))
//
// 也就是官方 `ui-conversation/src/client/contract/slots.ts:331` 的
// `conversation.input.activity`（single / session）——输入卡片控件行里
// "活动 pill" 那一个座位。本仓这个座位**早就存在且会被派发**
// （`internal/uikernel/official_slots.go:141`，由 `plugins/dscomposer` 的
// `drawControls` 转发，见 `plugins/dscomposer/composer.go:427-431`），
// 一直没人占。本包就是那个占用方。
//
// # 两态一套结构（与官方同形）
//
// 官方 `VoiceInput.tsx:147-182` 只有一个组件两个出口：
//
//	idle      → 28px 宽的麦克风按钮（<IconMicrophoneOutlineRegular size=18>）
//	其它四档  → `.captureRow`：取消圆钮 + （波形 | 状态文案）+ 停止圆钮 + 重试圆钮
//
// 五档 phase 是 `idle / requesting / recording / transcribing / feedback`
// （`VoiceInput.tsx:58`），与 `cmd/app/speech_voice.go` 的状态机逐字对应。
//
// # 数据与动作从哪来
//
// 插件不碰宿主、不碰麦克风：`Env` 明确不给客户端与模型（`pluginapi/plugin.go`
// 的 Env 注释），所以采集/识别全在壳里（`cmd/app/speech_voice.go`），
// 本包只读 `Act.Voice.Capability()/State()` 并调那六条动作。
package dsvoice

import (
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 几何**全部**取自官方 `VoiceInput.module.css`（本文件逐条标行）与
// `VoiceInput.tsx` 里的 size 实参。没有一个数是自己定的。
const (
	// `.trigger{width:28px}`（VoiceInput.module.css:1）。
	triggerWidth float32 = 28
	// `<IconMicrophoneOutlineRegular size={18}>`（VoiceInput.tsx:157）。
	triggerIcon float32 = 18
	// `.roundButton{width:32px;height:32px;border-radius:50%}`（css:4）。
	roundButton float32 = 32
	// `<IconCloseOutlineRegular size={14}>` / `<IconStopFillRegular size={14}>`
	// / `<IconMicrophoneOutlineRegular size={18}>`（VoiceInput.tsx:172,176,180）。
	roundCloseIcon float32 = 14
	roundStopIcon  float32 = 14
	// `.captureRow{gap:12px;min-height:34px}`（css:3）。
	captureGap       float32 = 12
	captureMinHeight float32 = 34
	// `.waveform{height:24px;min-width:24px}`（css:8）。
	waveformHeight   float32 = 24
	waveformMinWidth float32 = 24
	// `.activityMessage{gap:8px;font-size:12px}`（css:9）。
	messageGap  float32 = 8
	messageFont float32 = 12
	// StateDot 在官方是 `<StateDot state="ongoing">`（VoiceInput.tsx:174），
	// 一颗 6px 的点（primitives 的 dot 默认档）。
	stateDotSize float32 = 6

	// 波形是 `<svg viewBox="0 0 640 40">` 里的 80 根 `line`（Waveform.tsx:29-31）：
	// x1=x2=index*8+4、strokeWidth=3、线帽圆。高度按
	// `1 + min(1, level*5) * 17` 算半高（Waveform.tsx:20），所以满幅 36/40。
	waveformViewBoxW  float32 = 640
	waveformViewBoxH  float32 = 40
	waveformBars      int     = 80
	waveformBarPitch  float32 = 8
	waveformBarOffset float32 = 4
	waveformBarWidth  float32 = 3
	waveformHalfBase  float32 = 1
	waveformHalfRange float32 = 17
	waveformGain      float64 = 5
	// 基线不透明度 `0.25 + index/120`（Waveform.tsx:31）。
	waveformBaseAlpha float32 = 0.25
	waveformAlphaStep float32 = 1.0 / 120

	// 本包在座位里的顺序。single 槽只有一个占用方，顺序不影响位置；
	// 给 10 只是与 dscomposer 自己的输入卡片同档，便于日志里按 Order 读。
	voiceOrder = 10

	// 每一帧往波形里推进一格电平。
	// 官方是 rAF 里每 50ms 推一次（Waveform.tsx:14）；本仓壳在录音期间按
	// `voiceLevelTick=50ms` 触发 Invalidate，所以"一帧一格"与官方同频。
)

// Plugin 是听写控件。
type Plugin struct {
	theme *pluginapi.Theme
	// levels 是波形延迟线：新电平进队首，老电平往右挪（官方同样把新值
	// 赋给第 0 根柱、旧的依次后移，Waveform.tsx:17-21）。
	levels [waveformBars]float64
	// phase 用来判"新的一次录音"，跨次把波形清空。
	phase string
}

// ID 实现 pluginapi.Plugin。
func (p *Plugin) ID() string { return "ds-voice" }

// Provides 实现 pluginapi.Plugin。本包不提供可注入能力。
func (p *Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
func (p *Plugin) Requires() []string { return nil }

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	if v := ctx.Inject(pluginapi.KeyTheme); v != nil {
		if theme, ok := v.(*pluginapi.Theme); ok {
			p.theme = theme
		}
	}
	return nil
}

// Panels 把听写挂到官方 `conversation.input.activity` 座位。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:   "voice-input",
		Slot: uikernel.OffInputActivity,
		// 座位由 dscomposer 派发（`composer.go:427-431`）：注册了就有、没注册就
		// 走它自己的只读活动 chip 回退，不会双画。
		Order: voiceOrder,
		Draw:  p.draw,
	}}
}

// draw 按官方两个出口分派。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	act := env.Act.Voice
	if act == nil || act.Capability == nil || act.State == nil {
		return // 壳没接整条能力：不画（画一个点了没反应的按钮更坏）
	}
	if p.theme == nil {
		return
	}
	st := act.State()
	if st.Phase == "" || st.Phase == "idle" {
		p.phase = "idle"
		capability := act.Capability()
		// 不可用就**不画入口**：官方这时画一个按钮，点开是"你需要先安装模型"的
		// 引导对话框（`VoiceInput.tsx:152-165`），而本仓宿主用的是 Windows
		// **已安装**的识别器，没有"安装"这件事可做（见账本 #66 行）。
		if !capability.Available {
			return
		}
		p.drawTrigger(f, env, act)
		return
	}
	if st.Phase == "recording" && p.phase != "recording" {
		p.levels = [waveformBars]float64{} // 新的一次录音：波形从基线起
	}
	p.phase = st.Phase
	p.pushLevel(st.Level)
	p.drawCaptureRow(f, env, st, act)
}

// pushLevel 把本次电平推进延迟线。
func (p *Plugin) pushLevel(level float64) {
	copy(p.levels[1:], p.levels[:len(p.levels)-1])
	p.levels[0] = level
}

// drawTrigger 是 idle 态的麦克风按钮（`VoiceInput.tsx:152-166`）。
func (p *Plugin) drawTrigger(f ui.C, env *pluginapi.Env, act *pluginapi.VoiceActions) {
	t := p.theme
	id := env.Tree.ID(env.Feat, "voice-start")
	button := ui.Row(f).Size(triggerWidth, triggerWidth).Radius(t.Radii.MD).
		Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover))
	clicked := button.HitWith(id, "开始录音", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true}).
		Children(func() { drawMicrophone(f, triggerIcon, t.LabelSecondary) }).Clicked()
	if clicked {
		act.Begin()
	}
}

// drawCaptureRow 是展开态那一行（`VoiceInput.tsx:167-181`）。
//
// 宽度用 `Grow(1)` 而不是官方的 `width:100%`：这一行落在 composer 控件行里，
// 右边还有模型 chip 与主按钮；`100%` 会把它们挤出卡片。flex-grow 在同一个
// flex 容器里表达的是同一件事（占满剩余宽度）。
func (p *Plugin) drawCaptureRow(f ui.C, env *pluginapi.Env, st pluginapi.VoiceState, act *pluginapi.VoiceActions) {
	t := p.theme
	row := ui.Row(f).Items(ui.Center).Gap(captureGap).Height(captureMinHeight).Grow(1).Clip()
	row.Children(func() {
		p.roundButton(f, env, "voice-cancel", "取消", func() { drawClose(f, roundCloseIcon, t.LabelSecondary) }, act.Cancel)

		if st.Phase == "recording" {
			p.drawWaveform(f, t)
		} else {
			p.drawMessage(f, t, st)
		}

		if st.Phase == "recording" {
			p.roundButton(f, env, "voice-stop", "停止并识别", func() { drawStop(f, roundStopIcon, t.LabelSecondary) }, act.Stop)
		}
		if st.Phase == "feedback" {
			p.roundButton(f, env, "voice-retry", "重新录音", func() { drawMicrophone(f, triggerIcon, t.LabelSecondary) }, act.Retry)
		}
	})
}

// roundButton 是官方的 `.roundButton`（32×32 圆钮）：底色 `--dsw-specific-selector`，
// 悬停 `--dsw-alias-interactive-bg-hover-solid`（css:4-7）。
func (p *Plugin) roundButton(f ui.C, env *pluginapi.Env, name, label string, glyph func(), on func()) {
	t := p.theme
	id := env.Tree.ID(env.Feat, name)
	button := ui.Row(f).Size(roundButton, roundButton).Radius(gfx.RadiusFull).
		Background(t.BgLayer3).Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover))
	clicked := button.HitWith(id, label, gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true}).
		Children(glyph).Clicked()
	if clicked && on != nil {
		on()
	}
}

// drawMessage 是 `.activityMessage`：状态点 + 一行字（css:9，VoiceInput.tsx:173-176）。
//
// 三种文案逐字取官方 zh 词典（`locales.ts:100-102`）：
//
//	requesting   → "请允许使用麦克风…"
//	transcribing → "识别中…"
//	feedback     → 这一次失败/空结果的原文（`empty`="未识别到语音"，
//	               `failed`="语音识别失败：{message}"，由壳拼好）
//
// 官方在 requesting/transcribing 前面加一颗 `StateDot state="ongoing"`
// （会呼吸的点）。本仓画的是同一颗点的**静帧**——动画需要逐帧重绘，
// 而这一行的重绘节奏由壳的电平 tick 决定，录音态之外没有 tick。
func (p *Plugin) drawMessage(f ui.C, t *pluginapi.Theme, st pluginapi.VoiceState) {
	text := st.Message
	switch st.Phase {
	case "requesting":
		text = "请允许使用麦克风…"
	case "transcribing":
		text = "识别中…"
	}
	if text == "" {
		return
	}
	line := ui.Row(f).Items(ui.Center).Gap(messageGap).Grow(1).Clip()
	line.Children(func() {
		if st.Phase == "requesting" || st.Phase == "transcribing" {
			ui.Row(f).Size(stateDotSize, stateDotSize).Radius(stateDotSize / 2).
				Background(t.LabelTertiary)
		}
		ui.Text(f, text).FontSize(messageFont).Color(t.LabelSecondary)
	})
}

// drawWaveform 画官方那条 80 根柱的波形（Waveform.tsx:29-31 + css:8）。
//
// 每根柱：x = index*8+4（viewBox 单位）、宽 3、圆帽；半高
// `1 + min(1, level*5) * 17`，也就是静音时是 2px 的基线、最响 36px。
// 水平方向官方靠 `preserveAspectRatio="none"` 拉伸，所以这里把 viewBox 坐标
// 按实际宽度等比缩放；垂直方向 viewBox 40 → 24px。
func (p *Plugin) drawWaveform(f ui.C, t *pluginapi.Theme) {
	box := ui.Row(f).Height(waveformHeight).Grow(1).Clip()
	box.Children(func() {
		// 官方的 80 根柱是 `<svg viewBox="0 0 640 40" preserveAspectRatio="none">`
		// 里 x=index*8+4 的 `line`：**节距 8、线宽 3**，横向靠 SVG 拉伸铺满。
		// 这里用 80 个等宽格（每格 100/80 %）承载，柱子在格里居中、宽度固定 3px ——
		// 640 宽时 viewBox 单位正好 1px，所以 3px 就是官方的 3 个单位宽。
		for i := 0; i < waveformBars; i++ {
			level := p.levels[i]
			half := waveformHalfBase + float32(minFloat(1, level*waveformGain))*waveformHalfRange
			// 垂直缩放：viewBox 40 → 24px。
			height := 2 * half * waveformHeight / waveformViewBoxH
			if height < waveformBarWidth {
				height = waveformBarWidth
			}
			alpha := waveformBaseAlpha + waveformAlphaStep*float32(i)
			color := t.LabelSecondary
			color.Alpha *= alpha
			cell := ui.Row(f).WidthPercent(100 / float32(waveformBars)).
				Height(waveformHeight).Items(ui.Center).Justify(ui.Center)
			cell.Children(func() {
				ui.Row(f).Width(waveformBarWidth).Height(height).
					Radius(waveformBarWidth / 2).Background(color)
			})
		}
	})
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
