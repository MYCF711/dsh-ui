package dsnotifications

import "github.com/MYCF711/dsh-ui/pluginapi"

// 自己在 init 里报到：壳不需要知道有这个插件（见 pluginapi.Register 的注释）。
//
// 装配顺序由 Requires 拓扑排序决定：本插件只要 ui.theme（ds-core 提供），
// 所以它一定在 ds-core 之后；数据面（ui.apiclient / ui.account.notifications）
// 是**可选**注入，缺了就是空态，不会让整棵装配树失败。
func init() {
	pluginapi.Register("ds-account-notifications", func() pluginapi.Plugin { return &Plugin{} })
}
