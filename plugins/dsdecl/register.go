// Package dsdecl 是「运行时声明式插件」的装配者：它自己一行都不画，
// 只把声明目录（DSH_UI_DECL_DIR，默认 <用户配置目录>/dsh-ui/ui-plugins.d）
// 里 *.json 声明出来的部件挂到槽位上。
//
// 分工要看清，不然会找错地方：
//
//	internal/uidecl   JSON 的形状、校验、模板变量、怎么画（纯逻辑，能单测）
//	plugins/dsdecl    什么时候扫目录、扫到的东西怎么挂上去（就是本包）
//
// 为什么它必须存在：扫描这件事得有个人来做，而这个人不能是壳——
// 壳一旦知道「有个目录要扫」，第三方就有理由去改壳。所以扫描被包成一个
// 普通插件，和侧栏、状态栏走同一条装配路径。壳那边只多一行 blank import，
// 且**这一行之后再也不用加**：以后所有声明式部件都是往目录里放文件。
package dsdecl

import "github.com/MYCF711/dsh-ui/pluginapi"

// 自己在 init 里报到：壳不需要知道有这个插件（见 pluginapi.Register 的注释）。
func init() {
	pluginapi.Register("ds-decl", func() pluginapi.Plugin { return &Plugin{} })
}
