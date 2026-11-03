// register.go 把自己报到插件注册表：blank import 即接入（见 cmd/app/main.go 的
// `_ "…/plugins/dsshortcuts"`）。这是第三方插件的第三种接入方式——不碰壳、不改内核。
package dsshortcuts

import "github.com/MYCF711/dsh-ui/pluginapi"

func init() { pluginapi.Register("ds-shortcuts", func() pluginapi.Plugin { return &Plugin{} }) }
