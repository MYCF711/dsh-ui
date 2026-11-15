package dsmarket

import "github.com/MYCF711/dsh-ui/pluginapi"

// plugin.go —— 插件的注册面。
//
// # 为什么本插件今天**不注册任何槽位**
//
// 官方这条详情页的座位是 `plugins.tab`（id `discover`）里的内容：
//
//	OFF:../dsh-client-ui-plugin-market/src/client/index.ts:55-57
//	     ctx.slots.register({name:'plugins.tab', id:'discover', …}, DiscoverSection)
//
// 也就是说它是 **Plugins 页里的一个 tab**，而本仓的 Plugins 页
// （`plugins/dssidebar/plugins_page.go`）今天**没有 tab 贡献方机制**，它自己的文件头
// 就写着："本仓列表有多 tab（`plugins.tab` 贡献方）；本仓没有贡献方，按官方
// 「没有贡献方就不画 tab 条」的规则整段不画"。那条框架是 `#30` 的 **A 块**，
// 而 A 块卡在宿主 `pluginMarket.listIndexes` / `setIndexes` 的形状上，还没做。
//
// 所以本块只交**可画的那一半**：把详情页做成一个能画的对象（`Page`），用服务键
// `ui.market.detail` 交出去。A 块落地时只要：Inject 这个键 → `Open(entry, url)` →
// 在它的 tab 里调 `Draw`/`AfterDraw`。**不注册一个永远 dispatch 不到的座位**：
// 仓里对"登记了但画不到"的座位有明确先例（`plugins/dsgoal/official_seat.go`
// 文件头），那是不报错、界面空白的最难查的一类故障。

// KeyMarketDetail 是本插件提供的服务键，值是 `*Page`（详情页本体）。
//
// 谁消费：`#30` 的 A 块（Discover tab）。在那之前没有任何 Inject ——
// 这**不是**"登记了但画不到的座位"，只是一个还没人来取的能力；插件本身
// 不占槽位、不画东西、不影响装配。
const KeyMarketDetail = "ui.market.detail"

// Plugin 是 `ds-market` 插件。
type Plugin struct {
	theme *pluginapi.Theme
	page  *Page
}

// ID 实现 pluginapi.Plugin。
func (p *Plugin) ID() string { return "ds-market" }

// Provides 交出详情页本体（见 KeyMarketDetail）。
func (p *Plugin) Provides() []string { return []string{KeyMarketDetail} }

// Requires 只要主题：本页一个请求都不发，详情读由壳通过 `Actions.MarketEntryDetail` 交进来。
func (p *Plugin) Requires() []string { return []string{pluginapi.KeyTheme} }

// Register 取主题、造页面、交服务键。这里不画东西（仓规：Register 只注册）。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	theme, _ := ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.theme = theme
	p.page = NewPage(theme)
	ctx.Provide(KeyMarketDetail, p.page)
	return nil
}

// Page 返回详情页本体（与 KeyMarketDetail 提供的是同一个对象）。
func (p *Plugin) Page() *Page { return p.page }
