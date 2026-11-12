package dsagentteam

import "github.com/MYCF711/dsh-ui/pluginapi"

// 自己在 init 里报到（同 dsnotifications）。
//
// 注意：本插件**不** Require `ui.apiclient`——远端写是可选注入，缺了时
// 开关点了会如实报错（不假装保存成功），而不是让整棵装配树装配失败。
func init() {
	pluginapi.Register("ds-agent-team", func() pluginapi.Plugin { return &Plugin{} })
}
