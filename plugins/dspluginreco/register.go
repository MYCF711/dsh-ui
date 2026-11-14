package dspluginreco

import "github.com/MYCF711/dsh-ui/pluginapi"

// 官方包名 `dsh-client-ui-plugin-recommendations` → 本仓插件 id `ds-pluginreco`。
//
// 壳接线的两步：`cmd/app/main.go` 加一行 blank import；`cmd/app/official_panels_wiring.go`
// 里调 `SetSource` / `SetOnOpenDetails`（与 ds-actioncards 的 `SetSource` 同一时序与理由）。
func init() { pluginapi.Register(ServiceID, func() pluginapi.Plugin { return &Plugin{} }) }
