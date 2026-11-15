// plugins_tabs.go —— 官方 Plugins 页的**多 tab 骨架**（`#30` 的 A 块）。
//
// # 官方形状（唯一权威）
//
//	OFF:../dsh-client-ui-plugin-manager/src/client/PluginManagerPage.tsx:1725-1750
//	     页面页头里画 tab 条：第一项永远是页面自带的 `installed`，其后是
//	     `plugins.tab` 的贡献方（`SegmentedTabs`，`size="sm"`）。
//	OFF:PluginManagerPage.tsx:1577-1580
//	     选中的贡献方若离开台账就回落到 `installed`（"stays there when it returns"）。
//	OFF:PluginManagerPage.tsx:1750-1760
//	     贡献方的内容画在页头与 tab 条**下面**的 tabpanel 里。
//	OFF:../dsh-client-ui-plugin-manager/src/client/slot-contract.ts:135-141
//	     `plugins.tab` 是 list 槽：`label` 是 tab 名、`order` 是位置。
//	OFF:../dsh-client-ui-plugin-market/src/client/index.ts:55-57
//	     唯一的贡献方就是「发现」（id `discover`，label = `pluginMarket.title`）。
//
// # 本仓怎么落
//
// 本仓没有动态槽位台账（`ui.slots` 那一套），所以按仓里已冻结的先例
// （`plugins/dssettings/official_plugins.go` 的 `PluginsTab` 台账）用**静态台账**
// 表达同一件事：页面自带的 `installed` 不进台账，贡献方进台账；台账为空就
// **不画 tab 条**（官方"没有贡献方就不画"的规则），只有一项贡献方时画两格。
//
// 贡献方的本体由 `ds-market` 通过服务键 `pluginapi.KeyMarketDiscover` 交出来
// （插件之间不互相 import，见 pluginapi 那两条键的注释）；本文件只认下面这个
// **最小接口**，所以测试里可以塞一个假座位。
package dssidebar

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// zhPluginInstalledTab 是 tab 条第一格的文案：官方 'installedTab'
// （`OFF:../dsh-client-ui-plugin-manager/src/client/locales.ts:39`）。
//
// 放在本文件而不是 plugins_page.go 的文案块里：那一块是按列对齐的官方 zh 字典，
// 插一行会把整块重排成一片纯格式 diff（R02：改动范围只收到本任务内）。
const zhPluginInstalledTab = "已安装"

// Plugins 页的 tab id。官方 `installed` 是页面自带那一项
// （`PluginManagerPage.tsx:907` 的 `INSTALLED_TAB`），`discover` 是市场插件
// 注册 id（`client/index.ts:56`）。
const (
	// pluginsTabInstalled 是页面自带的那一页（已安装插件）。
	pluginsTabInstalled = "installed"
	// pluginsTabDiscover 是插件市场贡献的「发现」页。
	pluginsTabDiscover = "discover"
)

// PluginsPageTab 是 `plugins.tab` 台账的一项（官方 `PluginTab`，`tab-ledger.ts:7-10`）。
type PluginsPageTab struct {
	// ID 是注册 id。
	ID string
	// Order 是排序键（官方 `entry.options.order`，升序）。
	Order int
	// Label 是 tab 文案（官方 `resolveSlotLabel` 的结果）。
	Label string
}

// marketTabSeat 是「发现」tab 贡献方的最小接口。
//
// 它就是契约里的 `pluginapi.PluginsTab`（官方那边贡献方是 React 组件
// `DiscoverSection`；本仓是一个能画的对象）。三个方法就是页面真正用到的那三件：
// 叫什么、画什么、详情页开着没有（详情页那一帧页面要不要让出自己那层内距）。
// 这里单独起一个内名只为让测试塞假座位时不必依赖契约包的类型名。
type marketTabSeat = pluginapi.PluginsTab

// tabs 是 `plugins.tab` 台账（按 id 去重，登记顺序即绘制顺序）。
//
// 官方台账是槽位注册表的投影（`tab-ledger.ts:20-33`）；本仓没有动态槽位，
// 所以由页面自己持有一份，贡献方通过 `RegisterPluginsTab` 登记。
type pluginsTabLedger struct {
	tabs []PluginsPageTab
}

// RegisterPluginsTab 实现 `pluginapi.PluginsTabs`：登记一个 tab 贡献方。
//
// 同一 id 后登记的那次覆盖前一次（官方槽位同样按 id 去重）。
func (p *Plugin) RegisterPluginsTab(tab pluginapi.PluginsTab) {
	if tab == nil {
		return
	}
	label := tab.Label()
	if label == "" {
		return
	}
	id := pluginsTabDiscover
	// 同一 id 覆盖：把旧的那一项换掉（标签与座位一起换）。
	p.marketSeat = tab
	for i, existing := range p.pluginsTabLedger.tabs {
		if existing.ID == id {
			p.pluginsTabLedger.tabs[i].Label = label
			return
		}
	}
	// order 0：官方 `client/index.ts:55-57` 注册时没给 order（默认 0），
	// 而页面自带的 `installed` 永远排第一，所以这里 0 与官方一致。
	p.pluginsTabLedger.tabs = append(p.pluginsTabLedger.tabs, PluginsPageTab{ID: id, Order: 0, Label: label})
}

// pluginTabs 是**当前生效**的 `plugins.tab` 台账（页面自带那一项不在里面）。
//
// 今天只有一个贡献方（「发现」）；它不在（老壳/最小装配）时台账为空，
// 页面按官方规则不画 tab 条。
func (p *Plugin) pluginTabs() []PluginsPageTab {
	if p.marketSeat == nil {
		return nil
	}
	return p.pluginsTabLedger.tabs
}

// pluginTabIDs 是 tab 条的完整 id 名单：页面自带的 `installed` 在前，贡献方在后。
func (p *Plugin) pluginTabIDs() []string {
	tabs := p.pluginTabs()
	if len(tabs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(tabs)+1)
	ids = append(ids, pluginsTabInstalled)
	for _, tab := range tabs {
		ids = append(ids, tab.ID)
	}
	return ids
}

// activePluginTab 是**这一帧**真正生效的 tab。
//
// 官方判据（`PluginManagerPage.tsx:1577-1580`）：选中的贡献方不在台账里就落回
// `installed`——本仓同样判，并且顺手把状态写回去，免得下一帧再算一遍。
func (p *Plugin) activePluginTab() string {
	ids := p.pluginTabIDs()
	if len(ids) == 0 {
		p.pluginTab = pluginsTabInstalled
		return pluginsTabInstalled
	}
	for _, id := range ids {
		if id == p.pluginTab {
			return p.pluginTab
		}
	}
	p.pluginTab = pluginsTabInstalled
	return pluginsTabInstalled
}

// drawPluginTabBar 画官方 tab 条（`SegmentedTabs`，`PluginManagerPage.tsx:1732-1749`）。
//
// 只有一个 tab（也就是没有贡献方）时**整段不画**——与官方"`tabs.length === 0`
// 就不画 tab 条"同一条规则（`PluginManagerPage.tsx:1732`）。
func (p *Plugin) drawPluginTabBar(f ui.C) {
	ids := p.pluginTabIDs()
	if len(ids) <= 1 {
		return
	}
	labels := map[string]string{pluginsTabInstalled: zhPluginInstalledTab}
	for _, tab := range p.pluginTabs() {
		labels[tab.ID] = tab.Label
	}
	ui.Tabs(f, &p.pluginTab, ids, func(id string) string {
		if label, ok := labels[id]; ok {
			return label
		}
		return id
	})
}

// drawDiscoverTab 画「发现」tab 的整页：官方页头 + tab 条（带页内距）+ 贡献方本体。
//
// 页内距只加在**页头与 tab 条**上：贡献方（`Discover` / 详情页 `Page`）自带
// 官方 `.page` 那一层内距（见 `dsmarket/detail.go` 的 `pagePadX` 注释），再加一层
// 就是双份——那不是官方几何。
func (p *Plugin) drawDiscoverTab(f ui.C, env *pluginapi.Env) {
	pagePadding := pluginPagePadX(env)
	ui.Column(f).FillWidth().FillHeight().Children(func() {
		ui.Column(f).FillWidth().PadSides(0, pagePadding, 0, pagePadding).Children(func() {
			ui.Column(f).FillWidth().MaxWidth(pluginPageMaxWidth).Children(func() {
				p.drawPluginHeading(f)
				p.drawPluginTabBar(f)
			})
		})
		if p.marketSeat != nil {
			p.marketSeat.Draw(f, env)
		}
	})
}
