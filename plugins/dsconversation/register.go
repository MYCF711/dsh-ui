package dsconversation

import "github.com/MYCF711/dsh-ui/pluginapi"

// 消息流的搜索开关由命令行注入，所以壳会自己 append 一个实例；
// 这里的工厂只为让它出现在"可用插件"列表里。
func init() {
	pluginapi.Register("ds-conversation", func() pluginapi.Plugin { return &Plugin{} })
}
