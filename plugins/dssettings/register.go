package dssettings

import "github.com/MYCF711/dsh-ui/pluginapi"

// ServiceKey 是"打开设置浮层"的可选能力。
//
// 为什么要它：这个插件自己只认识键盘（Shift+F2）。将来壳或别的插件想加一个
// 菜单项 / 命令面板里的「设置」行，不用 import 本包（插件之间不互相 import），
// 只要 `ctx.Inject(ServiceKey).(func() bool)` 调一下就行——nil 就是"没装设置"，
// 那边就别画入口。
//
// 现在**没有任何人 Inject 它**（命令面板在 plugins/dscommands，这一轮不允许改）。
// 留着的理由是它零成本，且是"给 F2 面板加一行设置"的唯一不碰契约的接法；
// 真接上之前，它是空转接口——注释里写清楚，免得下一个人以为有人在用。
const ServiceKey = "ui.settings.open"

func init() {
	pluginapi.Register("ds-settings", func() pluginapi.Plugin { return &Plugin{} })
}
