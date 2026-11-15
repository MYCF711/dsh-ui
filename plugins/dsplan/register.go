package dsplan

import "github.com/MYCF711/dsh-ui/pluginapi"

// 官方包名 `dsh-client-ui-plan` → 本仓插件 id `ds-plan`。
//
// 壳接线的唯一一步：`cmd/app/main.go` 加一行 blank import。本包**没有**
// `SetSource` 之类的注入口——数据全在 `env.Data.Frames`（原始 journal 帧）里，
// 两个动作全在 `pluginapi.Actions` 上（`SelectRightTab` / `ToggleRightbar` /
// `OpenExternal`），不需要壳再转一手。缺哪个动作时本包按"不画/不响应"处理，
// 见 cards.go 的 `open` 与 preview.go 的 `openLink`。
func init() { pluginapi.Register(ServiceID, func() pluginapi.Plugin { return &Plugin{} }) }
