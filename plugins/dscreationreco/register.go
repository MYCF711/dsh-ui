package dscreationreco

import "github.com/MYCF711/dsh-ui/pluginapi"

// 官方包名 `dsh-client-ui-creation-recommendations` → 本仓插件 id `ds-creationreco`。
//
// 壳接线只有一步：`cmd/app/main.go` 加一行 blank import（本包**不需要**任何
// SetSource 注入——官方的 `inject: ['locale', 'actionCards']` 里没有 remote，
// 这张卡的数据全部在 props 上，见包注释「数据面」）。
func init() { pluginapi.Register(ServiceID, func() pluginapi.Plugin { return &Plugin{} }) }
