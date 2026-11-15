package dsmarket

import "github.com/MYCF711/dsh-ui/pluginapi"

// register.go —— 把 `ds-market` 登记进内置插件源。
//
// 为什么单独一个文件：`plugins/dsgoal` / `plugins/dsplan` 等 34 个插件都是这个形状
// （`init` 里只做登记，工厂返回零值插件），壳的 `cmd/app/main.go` 用空导入把它带进来。
// 装配顺序由 `Requires` 决定，不靠 init 顺序。
func init() { pluginapi.Register("ds-market", func() pluginapi.Plugin { return &Plugin{} }) }
