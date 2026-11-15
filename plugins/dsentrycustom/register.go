package dsentrycustom

import "github.com/MYCF711/dsh-ui/pluginapi"

// 登记进内置插件表：壳的 `pluginapi.Builtin()` 会为它造一个实例。
//
// 与 dssidebar / dstopbar 不同，本包没有需要壳持有的外部实例（不提供 Field、
// 不提供 Width），所以这里就是唯一的实例来源。
func init() {
	pluginapi.Register("ds-entry-customization", func() pluginapi.Plugin { return &Plugin{} })
}
