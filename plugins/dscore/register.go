package dscore

import "github.com/MYCF711/dsh-ui/pluginapi"

// 自己在 init 里报到。浅色/深色的选择留在壳的命令行参数里（-theme），
// 这里给的是默认（深色），因为 register 只负责"有这个插件"。
func init() {
	pluginapi.Register("ds-core", func() pluginapi.Plugin { return &Plugin{Dark: true} })
}
