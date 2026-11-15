package dsmarket

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// plugin.go —— 插件的注册面。
//
// # 官方这条包的座位
//
// 官方 `dsh-client-ui-plugin-market` 是 **Plugins 页里的一个 tab**：
//
//	OFF:../dsh-client-ui-plugin-market/src/client/index.ts:55-57
//	     ctx.slots.register({name:'plugins.tab', id:'discover', label: () => t('title'), …},
//	                        DiscoverSection)
//
// 本仓的 Plugins 页（`plugins/dssidebar/plugins_page.go`）是**页面属主**，所以
// tab 条由它画、tab 体由本插件交出去。两边不互相 import（仓规：插件之间只经
// 服务键，见 `pluginapi.KeyMarketDiscover` 的注释），于是本插件交出两个能力：
//
//	ui.market.detail   → *Page      （详情页本体，`#30` C 块）
//	ui.market.discover → *Discover  （「发现」tab 本体：索引源 + 搜索 + 筛选 + 列表）
//
// `#30` 的 A 块（tab 骨架 + 索引源增删）与 B 块（搜索 + 筛选工具栏）落地时，
// `plugins/dssidebar` 的 `plugins.tab` 台账就多了 `discover` 这一项，页面上才
// 真的出现 tab 条与这一页——在那之前这里只是"能力已登记、还没人来取"，
// 不是"登记了但画不到的座位"（那种先例见 `plugins/dsgoal/official_seat.go` 文件头）。
//
// # 数据从哪来
//
// 本插件**不持连接、不发请求**：详情读走 `env.Act.MarketEntryDetail`（C 块），
// 索引源与列表走 `env.Act.Market*`（A/B 块，壳的状态机在
// `cmd/app/market_discover.go`）。插件只做投影与绘制。

// KeyMarketDetail / KeyMarketDiscover 是插件契约里的两个服务键（值分别是
// `*Page` 与 `*Discover`）。键本体定义在 `pluginapi`，这里只是给本包一个短名字。
const (
	KeyMarketDetail   = pluginapi.KeyMarketDetail
	KeyMarketDiscover = pluginapi.KeyMarketDiscover
)

// Plugin 是 `ds-market` 插件。
type Plugin struct {
	theme    *pluginapi.Theme
	page     *Page
	discover *Discover
}

// ID 实现 pluginapi.Plugin。
func (p *Plugin) ID() string { return "ds-market" }

// Provides 交出详情页与「发现」tab 两个能力。
func (p *Plugin) Provides() []string {
	return []string{KeyMarketDetail, KeyMarketDiscover}
}

// Requires 要主题（画东西的令牌）与 Plugins 页的 tab 贡献口。
//
// 后者是官方 `plugins.tab` 的注册面（`pluginapi.KeyPluginsTabs`）：没有它，"发现"
// 这一页就没有地方落脚——装配失败并点名缺哪个键，好过静默少一个 tab。
func (p *Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPluginsTabs}
}

// Register 取主题、造两个对象、交服务键。这里不画东西（仓规：Register 只注册）。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	theme, _ := ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.theme = theme
	p.page = NewPage(theme)
	p.discover = NewDiscover(theme, p.page)
	ctx.Provide(KeyMarketDetail, p.page)
	ctx.Provide(KeyMarketDiscover, p.discover)
	// 把自己登记进 Plugins 页的 `plugins.tab` 台账（官方 `slots.register` 的等价物）。
	if tabs, ok := ctx.Inject(pluginapi.KeyPluginsTabs).(pluginapi.PluginsTabs); ok && tabs != nil {
		tabs.RegisterPluginsTab(p.discover)
	}
	return nil
}

// Fields 交出发「发现」tab 的两个真输入框（搜索框、索引源 URL）。
//
// 不实现这个接口就收不到键盘——搜索框会是一个点了打不出字的空框
// （`pluginapi.Fields` 的注释写着同一条）。
func (p *Plugin) Fields() []*ui.Field {
	if p.discover == nil {
		return nil
	}
	return p.discover.Fields()
}

// Page 返回详情页本体（与 KeyMarketDetail 提供的是同一个对象）。
func (p *Plugin) Page() *Page { return p.page }

// Discover 返回「发现」tab 的本体（与 KeyMarketDiscover 提供的是同一个对象）。
func (p *Plugin) Discover() *Discover { return p.discover }
