package dscomposer

import "github.com/MYCF711/dsh-ui/pluginapi"

// 自己在 init 里报到：壳不需要知道有这个插件（见 pluginapi.Register 的注释）。
func init() {
	pluginapi.Register("ds-composer", func() pluginapi.Plugin { return &Plugin{} })
}
