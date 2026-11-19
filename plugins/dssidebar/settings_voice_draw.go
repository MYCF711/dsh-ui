// settings_voice_draw.go —— 账本 #66 ④「MicrophonePicker 设置页」的**绘制**。
//
// 结构逐元素照官方 OFF:client-ui-voice-input/src/client/MicrophonePicker.tsx:124-139：
//
//	<div .deviceRow>                        grid 100px / minmax(120px,280px)，gap 12
//	  <label htmlFor="voice-input-device">  输入设备
//	  <span .deviceField>                   position:relative（min-width:0）
//	    <select id="voice-input-device">    0.5px 边框 / r6 / padding 6px 10px / 透明底
//	    <span .deviceMeter>（有真实电平时）  绝对定位 right:24 / 垂直居中
//	      <span .inputLevel>                8 根 3×12 bar，gap 3 / r1
//	  <span .deviceError>（有数据时）        grid-column:2 / 12px / state-error-primary
//
// 差异（缺真实能力，不造假）与文件头 settings_voice.go 的三条一致：
// 预览采集没有（仪表只在 recording 时画真值）、`.deviceError` 无数据源、
// 仪表用真 absolute 叠层（与官方 `position:absolute` 同义，不用流内挤占）。
//
// 断点：官方 `@media (max-width:480px)`（css:51-55）把 `.deviceRow` 改成单列、
// gap 6、`.deviceError{grid-column:1}`。本仓按**整窗宽**判（官方媒体查询也是视口宽），
// 没有布局引擎的响应式，所以这一个判断是显式的（settingsVoiceNarrow）。
package dssidebar

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

// drawSettingsVoiceForm 画这一页的正文（官方 `.preferences` 里的第一格）。
func (p *Plugin) drawSettingsVoiceForm(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card settingsItemCardState) {
	voice := card.Voice
	if voice == nil {
		return
	}
	// 官方页面挂载即重新枚举设备（MicrophonePicker.tsx:40-116 的 refresh()）。
	p.settingsVoiceEnumerateOnce(env, voice)
	narrow := settingsVoiceNarrow(env)
	ui.Column(f).FillWidth().Children(func() {
		if narrow {
			// `@media (max-width:480px){.deviceRow{grid-template-columns:minmax(0,1fr);gap:6px}}`
			ui.Column(f).FillWidth().Gap(voiceDeviceNarrowGap).Children(func() {
				p.drawSettingsVoiceLabel(f, t, true)
				p.drawSettingsVoiceField(f, env, t, voice, true)
				p.drawSettingsVoiceError(f, t, voice)
			})
		} else {
			// `.deviceRow{display:grid;grid-template-columns:100px minmax(120px,280px);
			//            align-items:center;gap:12px}`（css:30）
			ui.Row(f).FillWidth().Items(ui.Center).Gap(voiceDeviceGap).Children(func() {
				p.drawSettingsVoiceLabel(f, t, false)
				p.drawSettingsVoiceField(f, env, t, voice, false)
			})
		}
	})
}

// drawSettingsVoiceError 画官方 `.deviceError`（MicrophonePicker.tsx:138，`role="alert"`）。
//
// 官方它是 `.deviceRow` 网格里的 `grid-column:2`（css:47）⇒ 占的正是第 2 列、也就是
// `.deviceField` 那一格。本仓没有网格，所以把它画在**字段列里面**（紧贴 select 下方、
// 宽度与 select 同列），几何等价；断点内 `grid-column:1`（css:54）⇒ 整行宽，
// 调用处（窄版分支）也放在同一列里。
//
// 数据面：官方那一行是"预览采集失败"，本仓 `VoiceActions` 没有预览入口，
// 所以生产路径恒为空串（`voice.DeviceError` 见 settings_voice.go）；有数据就画，
// 不在别处找句子冒充。
func (p *Plugin) drawSettingsVoiceError(f ui.C, t *pluginapi.Theme, voice *settingsVoiceState) {
	if voice.DeviceError == "" {
		return
	}
	ui.Column(f).FillWidth().Children(func() {
		ui.Text(f, voice.DeviceError).FillWidth().FontSize(voiceErrorSize).
			LineHeight(voiceErrorLine).Color(t.Error)
	})
}

// drawSettingsVoiceLabel 画 `<label htmlFor="voice-input-device">{t('inputDevice')}</label>`
// （OFF:MicrophonePicker.tsx:125）：宽版占 100px 那一列，窄版整行。
func (p *Plugin) drawSettingsVoiceLabel(f ui.C, t *pluginapi.Theme, narrow bool) {
	box := ui.Row(f).Shrink(0)
	if narrow {
		box = ui.Row(f).FillWidth()
	} else {
		box = ui.Row(f).Width(voiceDeviceLabelWidth).Shrink(0)
	}
	box.Children(func() {
		ui.Text(f, zhVoiceInputDevice).FontSize(voiceSelectFont).LineHeight(voiceSelectLine).
			Color(t.LabelPrimary)
	})
}

// drawSettingsVoiceField 画 `.deviceField{position:relative;display:block;min-width:0}`
// （css:43）：宽版 = `minmax(120px,280px)` 那一列，窄版占满。
//
// 宽版里 `.deviceError` 与它在同一列（官方 `grid-column:2`），间隔用
// `.deviceRow{gap:12px}` 那一行距（网格行间距），所以字段 Column 的 gap 就是它。
func (p *Plugin) drawSettingsVoiceField(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	voice *settingsVoiceState, narrow bool) {
	var field ui.Box
	if narrow {
		field = ui.Column(f).FillWidth()
	} else {
		field = ui.Column(f).Grow(1).MinWidth(voiceDeviceFieldMin).MaxWidth(voiceDeviceFieldMax).
			Gap(voiceDeviceGap)
	}
	field.Children(func() {
		p.drawSettingsVoiceSelect(f, env, t, voice)
		if !narrow {
			p.drawSettingsVoiceError(f, t, voice)
		}
	})
}

// drawSettingsVoiceSelect 画 select + 它的选项菜单。
//
// 官方是原生 `<select>`（OFF:MicrophonePicker.tsx:127-135）。viva 没有原生 select，
// 本仓对"下拉"的既有做法是「自绘 `.selector` 盒子 + `ui.Menu`」（#47 provider 菜单，
// settings_items_web_draw.go:200-252），这里沿用同一套：盒子按官方 select 的几何画，
// 菜单行用 `.Disabled()` 表达官方 `disabled={device.unavailable}`（:132）。
func (p *Plugin) drawSettingsVoiceSelect(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, voice *settingsVoiceState) {
	if p.itemVoiceMenus == nil {
		p.itemVoiceMenus = map[string]bool{}
	}
	open := p.itemVoiceMenus[settingsVoiceDeviceDOMID] && voice.Selectable
	box, clicked := p.drawSettingsVoiceSelectorBox(f, env, t, voice)
	if voice.Selectable && clicked {
		open = !open
	}
	if voice.Selectable && env.Act.Voice != nil {
		selectDevice := env.Act.Voice.SelectDevice
		ui.Menu(f, box, &open, func(items *ui.MenuItems) {
			for _, choice := range voice.Choices {
				choice := choice
				// 官方 `disabled={device.unavailable}`：不可用的项画出来但选不动。
				if items.Item(choice.Label).Chosen(choice.Selected).Disabled(choice.Disabled).Clicked() &&
					!choice.Disabled {
					selectDevice(choice.ID)
				}
			}
		})
	}
	// 菜单自己会把 open 置回 false（选中/点外面）；写回映射表，下一帧才是真的关着。
	p.itemVoiceMenus[settingsVoiceDeviceDOMID] = open
}

// drawSettingsVoiceSelectorBox 画 `.selector`（官方 select 的几何），并报告它被点没被点。
//
// 命中区与 `.Clicked()` **写在同一个函数里**：`cmd/app` 的静态守卫（V-06-4 判据②）
// 要求 `.Clicked()` 与 `.Hit()` 成对出现，否则那个接收者被判成"永远点不着"
// —— 与 settings_items_web_draw.go:225-252 同一写法。
func (p *Plugin) drawSettingsVoiceSelectorBox(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	voice *settingsVoiceState) (ui.Box, bool) {
	// 有仪表时 `.deviceField select.deviceSelectMeter{padding-right:72px}`（css:45），
	// 否则就是 select 自己的 `padding:6px 10px`（css:31）。
	padRight := voiceSelectPadX
	if voice.Meter {
		padRight = voiceSelectPadRight
	}
	var box ui.Box
	var clicked bool
	ui.Stack(f).FillWidth().Children(func() {
		row := ui.Row(f).FillWidth().Height(voiceSelectHeight).Radius(voiceSelectRadius).
			Border(voiceSelectStroke, t.BorderL1).Items(ui.Center).
			PadSides(voiceSelectPadY, padRight, voiceSelectPadY, voiceSelectPadX)
		drawValue := func() {
			ui.Text(f, voice.Value).FontSize(voiceSelectFont).LineHeight(voiceSelectLine).
				Color(t.LabelPrimary).Grow(1).MinWidth(0).Ellipsis()
		}
		if voice.Selectable {
			box = row.Hit(env.Tree.ID(env.Feat, settingsVoiceDeviceDOMID), voice.Value)
			box.Children(drawValue)
			clicked = box.Clicked()
		} else {
			// 壳没接 `SelectDevice`：不挂命中区（画一个点了没反应的控件更坏）。
			box = row
			box.Children(drawValue)
		}
		if voice.Meter {
			// `.deviceMeter{position:absolute;top:50%;right:24px;display:inline-flex;
			//              transform:translateY(-50%);pointer-events:none}`（css:46）。
			// `top:50%` + `translateY(-50%)` = 垂直居中，等价于
			// (盒子高 - 仪表高) / 2；盒子高是显式常量（见 settings_voice.go）。
			top := (voiceSelectHeight - voiceLevelBarHeight) / 2
			ui.Row(f).Absolute().Right(voiceMeterRight).Top(top).Children(func() {
				p.drawSettingsVoiceLevel(f, t, voice.ActiveBars)
			})
		}
	})
	return box, clicked
}

// drawSettingsVoiceLevel 画官方 `.inputLevel`（8 根 bar）：
// `.inputLevel{gap:3px}` / `>span{width:3px;height:12px;border-radius:1px;
// background:var(--dsw-alias-border-l1)}` / `[data-active='true']` 用 state-success-primary
// （css:48-50）。点亮根数由 settingsVoiceActiveBars 按官方公式算。
func (p *Plugin) drawSettingsVoiceLevel(f ui.C, t *pluginapi.Theme, active int) {
	ui.Row(f).Gap(voiceLevelGap).Items(ui.Center).Children(func() {
		for index := 0; index < voiceLevelBars; index++ {
			color := t.BorderL1
			if index < active {
				color = t.SuccessPrimary
			}
			ui.Row(f).Width(voiceLevelBarWidth).Height(voiceLevelBarHeight).
				Radius(voiceLevelBarRadius).Background(color)
		}
	})
}

// settingsVoiceNarrow 报告这一帧要不要走官方 `@media (max-width:480px)` 那一档。
//
// 官方那条媒体查询看**视口**宽，本仓看整窗宽（`Geometry.Width`）；都拿不到时
// 退 `MainWidth` / `Bounds`，全部为 0（无几何的最小装配）时不走窄版。
func settingsVoiceNarrow(env *pluginapi.Env) bool {
	if env == nil {
		return false
	}
	width := env.Geometry.Width
	if width <= 0 {
		width = env.MainWidth
	}
	if width <= 0 {
		width = env.Bounds.Width
	}
	return width > 0 && width <= voiceDeviceBreakpoint
}

// settingsVoiceEnumerateOnce 实现官方"页面挂载即重新枚举一次设备"的效果
// （MicrophonePicker.tsx:40-116 的 refresh()；它的 effect 依赖 `selected.id`，
// 所以**选中项一变也重新枚举**）。同一选中项只枚举一次，避免逐帧重入。
func (p *Plugin) settingsVoiceEnumerateOnce(env *pluginapi.Env, voice *settingsVoiceState) {
	if !voice.Refreshable || env.Act.Voice == nil {
		return
	}
	if p.voiceEnumerated && p.voiceEnumeratedFor == voice.Selected {
		return
	}
	p.voiceEnumerated = true
	p.voiceEnumeratedFor = voice.Selected
	env.Act.Voice.RefreshDevices()
}
