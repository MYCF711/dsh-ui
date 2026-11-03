package dssidebar

import "github.com/MYCF711/dsh-ui/pluginapi"

// 侧栏需要壳持有同一个实例（壳要读它的 Fields / Width），
// 所以这里登记的是「占位」：真正的实例由壳 appendPlugin 注入，
// 这个工厂只保证 id 出现在可用列表里，正常不会被用到。
func init() {
	pluginapi.Register("ds-sidebar", func() pluginapi.Plugin { return &Plugin{} })
}
