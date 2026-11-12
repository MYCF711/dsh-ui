// official_trigger.go —— 官方 `SettingsRoot` 的**设置触发器按钮**（`sidebar.settings`
// 那一行里 `flex:1` 的那一格）。
//
// # 官方出处（逐字）
//
// `ui-settings-general/src/client/SettingsRoot.tsx:222-245`：
//
//	<div className={clsx(SettingsRoot_module_css_default.triggerRow, !wide && …railRow)}>
//	  {renderSlot('settings.launcher', {…}, {
//	     fallback: (
//	       <Tooltip disabled={open} label={t('trigger')} …>
//	         <button type="button"
//	                 className={clsx(SettingsRoot_module_css_default.trigger, !wide && …rail)}
//	                 aria-label={t('trigger')}
//	                 aria-haspopup="dialog"
//	                 aria-expanded={open}
//	                 onClick={() => { actions.open(); }}>
//	           {renderSlot('settings.trigger', { wide })}
//	         </button>
//	       </Tooltip>
//	     ),
//	  })}
//	  <ConnectionIndicator … />
//	  <DesktopUpdateIndicator … />
//	</div>
//
// 也就是：**设置按钮是 `settings.launcher` 的 fallback**——有账号启动器时
// 由它当设置入口（官方 `AccountMenu` 的 trigger 打开同一个设置面板），
// 没有启动器时才画这个齿轮按钮。本仓与官方同一条判据：`settings.launcher`
// 槽上**没有任何占用方**时才画（`env.Has`）。
//
// # 几何（`SettingsRoot.module.css`，逐条）
//
//	.trigger{box-sizing:border-box;border-radius:var(--dsw-radius-md);cursor:pointer;
//	         width:auto;min-width:0;height:42px;color:var(--dsw-alias-label-primary);
//	         background:0 0;border:none;flex:1;align-items:center;gap:8px;margin:0;
//	         padding:0 10px 0 8px;font-family:inherit;font-size:14px;line-height:22px;
//	         display:flex;overflow:hidden}
//	.trigger:hover{background:var(--dsw-alias-interactive-bg-hover)}
//	.triggerLabel{white-space:nowrap;overflow:hidden}
//	.trigger.rail{flex:none;justify-content:center;gap:0;width:36px;height:36px;margin:0;padding:0}
//
// 本仓只实现宽态（`wide`）：收起轨（`railRow` / `.rail` 36×36）还没有对应的
// 侧栏收起态，不画一个没有出路的变体。
//
// # 为什么是 fallback，而不是"再加一个设置按钮"
//
// 产品形状里 dsnotifications 的账号启动器一直占着 `settings.launcher`，
// 所以**产品里看不到这个按钮**——看到的是启动器（同样打开设置面板）。
// 这不是"没实现"，是官方的替换语义：`fallback` 只在没有占用方时出现。
// 无头/最小插件集（没有 dsnotifications）时它才画，正是本文件要保证的那条。
package dssettings

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// slotSettingsLauncher 是官方 `settings.launcher`（`ui-settings-general/index.ts:237`
// 声明、`ui-settings-account/index.ts:316` 注册 AccountMenu）。
//
// 为什么写成字面量而不是 import dsnotifications：插件之间不互相 import
// （`dsnotifications/launcher.go` 的 `SlotSettingsLauncher` 是同一个字符串）。
const slotSettingsLauncher = pluginapi.Slot("settings.launcher")

// zhSettingsTrigger 是官方 `t('trigger')` 的 zh 文案（`settings` 语言域，逐字）。
const zhSettingsTrigger = "设置"

// drawSettingsTrigger 画官方设置触发器（fallback 语义）。
//
// 行容器由壳提供（`uikernel.drawSidebarSkeleton` 的 `SettingsRoot.triggerRow`
// 那个 Row），本函数只画行里的这一格：`flex:1` 让它吃掉左侧剩余空间，
// 把连接指示推到行右缘（官方 `[设置按钮(flex:1)] [连接指示]` 的排法）。
func (p *Plugin) drawSettingsTrigger(f ui.C, env *pluginapi.Env) {
	// 官方 `settings.launcher` 有占用方（账号启动器）时，设置入口在它身上；
	// 这里再画一枚就是同一个窗口里两个"设置"入口。
	if env.Has != nil && env.Has(slotSettingsLauncher) {
		return
	}
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	id := env.Tree.ID(env.Feat, "settings-trigger")
	row := ui.Row(f).Height(TriggerHeight).Grow(1).
		PadSides(0, TriggerPadRight, 0, TriggerPadLeft).
		Radius(t.Radii.MD).Items(ui.Center).Gap(TriggerGap).
		Hit(id, zhSettingsTrigger).
		// `.trigger:hover{background:var(--dsw-alias-interactive-bg-hover)}`。
		Hover(ui.Look{}.Background(t.Hover))
	row.Children(func() {
		// 宽态图标 16（`chrome.tsx:22-29` 的 `wide ? 16 : 18`）。
		ui.Icon(f, settingsTriggerIcon()).Size(TriggerIconSize, TriggerIconSize).Color(t.LabelPrimary)
		// `.triggerLabel{white-space:nowrap;overflow:hidden}`：单行省略。
		ui.Text(f, zhSettingsTrigger).FontSize(TriggerFontSize).LineHeight(TriggerLineHeight).
			Color(t.LabelPrimary).Ellipsis()
	})
	if row.Clicked() {
		// 与 Shift+F2 同一个真实入口（`actions.open()`）。
		p.Open()
	}
}
