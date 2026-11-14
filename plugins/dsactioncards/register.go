package dsactioncards

import "github.com/MYCF711/dsh-ui/pluginapi"

// 官方包名 `dsh-client-ui-action-cards` → 本仓插件 id `ds-actioncards`（沿用 ds* 前缀）。
//
// 壳接线的唯一一步是 `cmd/app/main.go` 里加一行 blank import；数据面由
// `SetSource` + `Refresh` 注入（见 cards.go 包注释「它要壳注入什么」）。
func init() { pluginapi.Register("ds-actioncards", func() pluginapi.Plugin { return &Plugin{} }) }
