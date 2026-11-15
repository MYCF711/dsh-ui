// Package panelapi 是 ds-sidebar 与「面板导航」消费方之间的最小契约。
//
// # 官方事实（rc.2 随包 src/**）
//
// `pluginNavigation` 不是宿主能力，是 **plugin-manager 的客户端服务**：
//
//	OFF:dsh-client-ui-plugin-manager/src/client/index.ts:153-166
//	  `ctx.reflect.provide('pluginNavigation', { openList, openBundle })`
//	OFF:dsh-client-ui-entry-customization/src/client/index.ts:32-46
//	  `ctx.inject(['pluginNavigation'], …)` 只在该服务**存在时**多画一行
//	  「浏览更多插件」（`browsable`），点了调 `openList()`
//
// 也就是说：这一行不是"宿主有没有能力"，而是"插件管理面在不在"。本仓的插件管理面
// 就在 ds-sidebar 里（`plugins/dssidebar/plugins_page.go`，`panelRoutes()` 的
// `plugins` 路由），所以由 ds-sidebar 提供这条服务，消费方按**存在与否**决定画不画，
// 与官方同一个判据。
//
// 只暴露 `OpenList`：官方 entry-customization 只调 `openList()`（`openBundle` 是
// 插件管理面自己用的，没有任何消费方），本仓不预先发明第二个方法。
package panelapi

// ServiceKey 是面板导航能力的注入键。
//
// 名字沿用官方服务名（`pluginNavigation`），前缀与本仓其它能力键一致（`ui.*`）：
// 谁提供、谁消费都可以从键名直接对上官方那份 index.ts。
const ServiceKey = "ui.pluginNavigation"

// Navigation 是面板导航服务的形状。
//
// 它是**能力契约**而不是面板清单：清单归 ds-sidebar 自己（`panelRoutes()`），
// 消费方只需要"能不能打开插件主列表"这一个动作。
type Navigation interface {
	// OpenList 打开插件主列表（官方 `pluginNavigation.openList()`）。
	//
	// 空实现是允许的（那时这一行等于点了没反应）——所以消费方仍要按
	// `Inject(ServiceKey)` 的**存在性**决定画不画，而不是按"服务在但它是空的"。
	OpenList()
}
