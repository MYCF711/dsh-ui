package dscordis

import "github.com/MYCF711/dsh-ui/pluginapi"

// 自己在 init 里报到（同 dsnotifications / dsagentteam / dsjobs）。
//
// 注意：本插件**不** Require `ui.apiclient`，也不 Require 任何 Cordis 远端——
// 数据面是 `Snapshot.Cordis`，动作面是 `Actions.Cordis*`，两者都可以缺席。
// 缺席时的表现是**整块不画**（而不是装配失败）：宿主没有 dynamicCordisRunner
// 时壳根本不填 `Snapshot.Cordis`，面板就没有存在的理由（`pluginapi/cordis.go` 文件头）。
func init() {
	pluginapi.Register("ds-cordis", func() pluginapi.Plugin { return &Plugin{} })
}
