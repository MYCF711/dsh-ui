// discover.go —— 官方「发现」tab（`#30` 的 A 块骨架 + B 块搜索/筛选工具栏）。
//
// # 官方结构（唯一权威）
//
//	OFF:../dsh-client-ui-plugin-market/src/client/index.ts:55-57
//	     ctx.slots.register({name:'plugins.tab', id:'discover', label: t('title')}, DiscoverSection)
//	OFF:DiscoverSection.tsx:492-560   Browser 的工具栏：索引菜单 → 搜索框 → 标签过滤 →
//	                                  排序 → 刷新；下面是分类胶囊行
//	OFF:DiscoverSection.tsx:561-570   失败行（`readFailed` + 重试）
//	OFF:DiscoverSection.tsx:571-600   骨架 / 空态（`emptyIndex` / `noMatches`）/ 卡片 / 显示更多
//	OFF:DiscoverSection.tsx:150-165   EntryCard：48px 图标框 + 标题 + 标签 + 说明 + meta
//	OFF:DiscoverSection.tsx:318-350   SourcesDialog：索引源列表 + 输入框 + 添加 + 信任提示
//	OFF:DiscoverSection.tsx:585-605   没有任何索引源时的空态（`emptyTitle` + 「添加索引源」）
//	OFF:DiscoverSection.module.css:3-124 几何（每一处常量下面标了行号）
//
// # 数据 → 容器 → 渲染
//
//	pluginMarket.listIndexes / search / setIndexes（宿主真实现）
//	  → 壳的状态机（cmd/app/market_discover.go，照官方 market-store.ts 移植）
//	  → env.Act.Market*（本文件的唯一数据来源）
//	  → 本文件按官方结构画工具栏 / 分类行 / 卡片；点卡片进 C 块的详情页（Page）
//
// # 按仓规 HIDE 的三处（缺真实能力，不画假入口）
//
//  1. **安装 / 更新 / 在已安装中查看**（官方 `EntryAction`，:102-135）：走
//     `pluginManager.installBundle` / `updateBundle`，宿主这两个是结构化 501。
//     卡片右侧那一列留空，不拿别的动作冒充。
//  2. **远程图标图**（官方 `.image` 加载 `entry.icon` 的 URL / data URL）：本仓没有
//     图片加载器，所以缺省与有 URL 时都先画官方 `PluginArtworkDefault` 原始 path +
//     渐变；48px 框内为 36px（DiscoverSection.tsx:27-28,44-51）。
//  3. **详情页相对图片**：宿主没有 `GET /api/plugin-market/detail-file`
//     （C 块已登记，见 docs/official-plugin-market-detail-evidence-rc2.md §4）。
//
// 刷新、搜索、过滤、移除、添加、警告、返回与默认插件图全部复用官方 sprite/path；
// 不保留文字字形或首字兜底。
package dsmarket

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 官方几何（DiscoverSection.module.css，行号逐条标注）────────────────────

const (
	// `.section{display:flex;flex-direction:column;gap:16px}`（:4-8）。
	discoverSectionGap float32 = 16
	// `.toolbar{display:flex;flex-wrap:wrap;align-items:center;gap:8px}`（:10-15）。
	discoverToolbarGap float32 = 8
	// `.marketButton{max-width:240px;margin-left:-8px;font-weight:500}`（:17-21）。
	discoverMarketMax  float32 = 240
	discoverMarketPull float32 = -8
	// `.search{flex:1;min-width:120px}`（:29-32）。
	discoverSearchMin float32 = 120
	// `.categories{display:flex;flex-wrap:wrap;align-items:center;gap:6px}`（:34-39）。
	discoverCategoriesGap float32 = 6
	// `.muted{margin:0;font-size:13px;line-height:20px}`（:41-47）。
	discoverMutedSize float32 = 13
	discoverMutedLine float32 = 20
	// `.error{display:inline-flex;align-items:center;gap:6px;margin:0;font-size:13px;line-height:20px}`（:62-70）。
	discoverErrorSize float32 = 13
	discoverErrorLine float32 = 20
	// `.failure{display:flex;align-items:center;gap:10px}`（:72-76）。
	discoverFailureGap float32 = 10
	// `.cards{display:flex;flex-direction:column;gap:2px}`（:97-104）。
	discoverCardsGap float32 = 2
	// `.card{display:flex;align-items:center;gap:14px;margin:0 -8px;padding:8px;
	//   border-radius:radius-xl}`（:106-114）。
	discoverCardGap  float32 = 14
	discoverCardPull float32 = -8
	discoverCardPad  float32 = 8
	// `.icon{width:48px;height:48px;border:0.5px solid border-l3;border-radius:radius-lg}`（:122-132）。
	discoverIconSize   float32 = 48
	discoverIconBorder float32 = 0.5
	// 框内官方 PluginArtworkDefault 为 36px（DiscoverSection.tsx:27-28）。
	marketArtworkSize float32 = 36
	// `.description{-webkit-line-clamp:1;font-size:13px;line-height:18px}`（:188-196）。
	discoverDescSize float32 = 13
	discoverDescLine float32 = 18
	// `.empty{padding:64px 0;gap:8px}`（:216-223）；`.emptyTitle{font-size:14px;line-height:20px;margin-top:8px}`（:225-230）。
	discoverEmptyPad  float32 = 64
	discoverEmptyGap  float32 = 8
	discoverEmptySize float32 = 14
	discoverEmptyLine float32 = 20
	// `.dialogBody{display:flex;flex-direction:column;gap:12px}`（:364-369）。
	discoverDialogGap float32 = 12
	// `.dialog{width:min(480px, calc(100vw - 32px))}`（:360-362）。
	discoverDialogWidth float32 = 480
	// `.sourceRow{gap:8px;padding:8px 0;border-bottom:0.5px solid border-l2}`（:379-393）。
	discoverSourceGap float32 = 8
	// `.notice{padding:10px 12px;border-radius:radius-md;font-size:12px;line-height:18px}`（:78-89）。
	discoverNoticeSize float32 = 12
	discoverNoticeLine float32 = 18

	// ui-primitives/src/Input.module.css:1-9,16-23,26-34。
	marketInputHeight float32 = 32
	marketInputPadX   float32 = 8
	marketInputGap    float32 = 6
	marketInputIcon   float32 = 16
	marketInputGlyph  float32 = 14
	marketInputFont   float32 = 14
	marketInputLine   float32 = 22
	marketInputBorder float32 = 0.5
	// ui-primitives/src/Pill.module.css:1-14,26-30。
	marketPillHeight float32 = 24
	marketPillPadX   float32 = 8
	marketPillFont   float32 = 12
	marketPillLine   float32 = 18
	marketPillRadius float32 = 999
	// ui-primitives/src/Button.module.css:28-34,56-61,80-86。
	marketButtonHeight float32 = 28
	marketButtonPadX   float32 = 10
	marketButtonGap    float32 = 4
	marketButtonFont   float32 = 12
	marketButtonLine   float32 = 18
	marketButtonIcon   float32 = 16
	marketButtonBorder float32 = 0.5
	// `.emptyTitle{margin-top:8px}` / `.empty > button{margin-top:8px}`（:225-234）。
	discoverEmptyExtraGap float32 = 8
	// `.skeletonBar{height:12px;margin:4px 0}` 与 144/60%、280/85%（:421-429）。
	marketSkeletonBarHeight float32 = 12
	marketSkeletonBarMargin float32 = 4
	marketSkeletonTitleMax  float32 = 144
	marketSkeletonDescMax   float32 = 280
	// Tab 体自己的内距：官方 `.page{padding:0 clamp(24px,4vw,48px) 48px}` 在 Plugins 页那
	// 一层，本仓 plugins 页把这一层交给**本 tab** 自己（见 Page.Draw 的同一条约定），
	// 所以这里复用 detail.go 的 `pagePadX` / `pageBottomPad` / `pageMaxWidth`，不另抄一份。
)

// ── 组件 ────────────────────────────────────────────────────────────────

// Discover 是「发现」tab 的本体：索引源 + 搜索栏 + 结果列表 + 索引源对话框。
//
// 生命周期由 `*Plugin` 管（与详情页 Page 同一个插件、同一个 Register）。
// 它**不持有数据源**：所有状态都从 `env.Act.Market*` 读，所有动作都交给壳。
type Discover struct {
	theme *pluginapi.Theme
	page  *Page
	// language 是详情读与日期用的 BCP-47（本仓界面只有中文）。
	language string

	// search 是工具栏搜索框（必须由 Plugin.Fields() 交出去，否则收不到键盘）。
	search ui.Field
	// draft 是索引源对话框里的 URL 输入框。
	draft ui.Field

	// query 是用户选定的 query（文本随输入框每帧同步）。
	query Query
	// selected 是当前索引源 URL（与壳里的 listing.URL 一致）。
	selected string

	// 本帧从壳读到的快照（sync 里填）。
	indexes            []apiclient.MarketIndexSummary
	indexesLoaded      bool
	indexesUnavailable bool
	listing            pluginapi.MarketListing
	hasListing         bool

	// editing 是索引源对话框开着没有；saving 是写回在飞；failure 是官方四档。
	editing       bool
	saving        bool
	sourceFailure apiclient.MarketSourceFailureKind
	// wasSaving 用来识别"写回刚刚成功"（壳只给状态，不给事件）。
	wasSaving bool

	// menus 是三个下拉各自的开关；同一时刻只允许一个开着（官方 `OpenMenu`）。
	menuMarket bool
	menuFilter bool
	menuSort   bool

	// opened 是点开的那条条目（非 nil = 详情页）。detailURL 是它所属的索引源，
	// 详情读要用（官方 `listing.url`）。
	detailURL  string
	openedName string
	// entries 是本帧的结果投影缓存（同一次结果只投影一次）。
	entriesKey  entriesCacheKey
	entriesList []Entry
}

// entriesCacheKey 是结果投影的键：同一份结果文档 + 同一 URL 就没必要重投影。
type entriesCacheKey struct {
	url       string
	fetchedAt int64
	total     int
	length    int
}

// NewDiscover 造一个「发现」tab。theme 来自 `ui.theme`；page 是 C 块的详情页。
func NewDiscover(theme *pluginapi.Theme, page *Page) *Discover {
	d := &Discover{theme: theme, page: page, language: "zh-CN", query: InitialQuery()}
	if page != nil {
		page.SetLanguage(d.language)
		page.SetBack(func() { d.closeDetail() })
	}
	return d
}

// Label 是 tab 的名字（官方 `t('title')`，`client/index.ts:56`）。
func (d *Discover) Label() string { return zhTabTitle }

// Fields 交出两个真输入框（搜索框与索引源 URL）。
func (d *Discover) Fields() []*ui.Field {
	return []*ui.Field{&d.search, &d.draft}
}

// DetailOpen 报告详情页开着没有（Plugins 页据此决定要不要收掉自己那层页内距）。
func (d *Discover) DetailOpen() bool { return d.openedName != "" }

// Open 打开一条条目的详情页（供测试与壳的无鼠标入口用）。
func (d *Discover) Open(entry Entry, marketURL string) {
	d.openedName = entry.Name
	d.detailURL = marketURL
	if d.page != nil {
		d.page.Open(entry, marketURL)
	}
}

// CloseDetail 关掉详情页，回到列表。
func (d *Discover) closeDetail() {
	d.openedName = ""
	d.detailURL = ""
	if d.page != nil {
		d.page.Close()
	}
}

// Draw 画这个 tab（列表或详情页）。
//
// 壳没接数据面（`env.Act.MarketIndexes == nil`）时**整块不画**：官方同样要求
// `remote.pluginMarket` 存在才注册这个 tab。这里画一句真实原因，避免"空白页"。
func (d *Discover) Draw(f ui.C, env *pluginapi.Env) {
	if d == nil || d.theme == nil || env == nil {
		return
	}
	if env.Act.MarketIndexes == nil || env.Act.MarketListing == nil {
		ui.Text(f, "当前宿主未提供插件市场数据面").FontSize(discoverMutedSize).
			LineHeight(discoverMutedLine).Color(d.theme.LabelTertiary).FillWidth()
		return
	}
	d.sync(env)
	if d.openedName != "" && d.page != nil {
		d.page.Draw(f, env)
		return
	}
	d.drawList(f, env)
}

// sync 读一次壳的状态并把它折叠进界面状态。
//
// 三件事：
//  1. 索引源清单与列表状态（壳每帧给的都是缓存，读起来很便宜）；
//  2. **壳裁剪过的 query 要写回界面**（官方 declaredQuery 在控制器里就地改 query，
//     本仓分居壳/插件两处，所以要这一趟，否则工具栏上会挂着一个"没生效"的过滤）；
//  3. 写回索引源的 saving 变化：从 true 落回 false 且没有失败档 = 成功 → 清空草稿。
func (d *Discover) sync(env *pluginapi.Env) {
	indexes, loaded, unavailable, _ := env.Act.MarketIndexes()
	d.indexes, d.indexesLoaded, d.indexesUnavailable = indexes, loaded, unavailable

	listing, has := env.Act.MarketListing()
	if !has {
		d.hasListing = false
		d.listing = pluginapi.MarketListing{}
	} else {
		d.hasListing = true
		d.listing = listing
		if d.selected != listing.URL {
			// 壳换了索引（首屏自动选中 / 刚添加完）→ 本地 query 回到官方初始值。
			d.selected = listing.URL
			d.query = InitialQuery()
			d.search.SetText("")
		}
		if listing.Answered != nil {
			applied := QueryFromWire(listing.Query)
			// 只回写**过滤与排序**，而且要等壳真的处理到我们这段文本之后——
			// 文本是用户正在打的字，壳那边可能还在 250ms 去抖里；那期间回写
			// 会把"打字刚切过去的 relevance"打回 index。
			if applied.Text == d.query.Text {
				d.query.Category = applied.Category
				d.query.Labels = applied.Labels
				d.query.Sort = applied.Sort
			}
		}
	}
	// 文本不在 sync 里跟随输入框：`withText` 的判据是"**之前**的文本空不空"
	// （官方 presentation.ts:24-34），先在这里把文本写过去会把那次切换吃掉。
	// 文本只在输入框 Changed 的那一帧经 `WithText` 更新（见 drawToolbar）。

	saving, kind, _ := env.Act.MarketSourcesState()
	if d.wasSaving && !saving && kind == "" {
		// 写回成功：官方清空草稿（`saveSources` 里 `adding ? {draft: ''} : {}`）。
		d.draft.SetText("")
	}
	d.wasSaving = saving
	d.saving = saving
	d.sourceFailure = kind
}

// ── 列表 ────────────────────────────────────────────────────────────────

// drawList 画整个列表页（工具栏 + 分类行 + 失败行 + 结果）。
func (d *Discover) drawList(f ui.C, env *pluginapi.Env) {
	ui.Column(f).FillWidth().FillHeight().Children(func() {
		scroll := ui.Scroll(f.Key("market-discover")).FillWidth().Grow(1).Clip()
		scroll.Children(func() {
			ui.Column(f).FillWidth().PadSides(0, pagePadX(env), pageBottomPad, pagePadX(env)).Children(func() {
				ui.Column(f).FillWidth().MaxWidth(pageMaxWidth).Gap(discoverSectionGap).Children(func() {
					switch {
					case d.indexesUnavailable:
						d.drawSourcesFailure(f, env)
					case !d.indexesLoaded:
						d.drawLoading(f)
					case len(d.indexes) == 0:
						d.drawEmpty(f, env)
					case !d.hasListing:
						d.drawLoading(f)
					default:
						d.drawToolbar(f, env)
						d.drawCategories(f, env)
						d.drawListing(f, env)
					}
				})
			})
		})
	})
	// 对话框永远挂在最外层（它是 overlay，不参与上面的列布局）。
	d.drawSourcesDialog(f, env)
}

// drawLoading 画官方无字骨架（ListSkeleton，:398-412）：四行 48px 框 +
// 12px 横条，横条上下 margin 4，宽度 min(144,60%) / min(280,85%)。
func (d *Discover) drawLoading(f ui.C) {
	t := d.theme
	ui.Column(f).FillWidth().Gap(discoverCardsGap).Children(func() {
		for i := 0; i < 4; i++ {
			ui.Row(f).FillWidth().Items(ui.Center).Gap(discoverCardGap).
				MarginSides(0, discoverCardPull, 0, discoverCardPull).
				PadSides(discoverCardPad, discoverCardPad, discoverCardPad, discoverCardPad).
				Radius(t.Radii.XL).Children(func() {
				ui.Row(f).Width(discoverIconSize).Height(discoverIconSize).Shrink(0).
					Radius(t.Radii.LG).Background(t.BgSkeleton)
				ui.Column(f).Grow(1).MinWidth(0).Children(func() {
					ui.Row(f).WidthPercent(0.60).MaxWidth(marketSkeletonTitleMax).
						Height(marketSkeletonBarHeight).MarginSides(marketSkeletonBarMargin, 0, marketSkeletonBarMargin, 0).
						Radius(t.Radii.XS).Background(t.BgSkeleton)
					ui.Row(f).WidthPercent(0.85).MaxWidth(marketSkeletonDescMax).
						Height(marketSkeletonBarHeight).MarginSides(marketSkeletonBarMargin, 0, marketSkeletonBarMargin, 0).
						Radius(t.Radii.XS).Background(t.BgSkeleton)
				})
			})
		}
	})
}

// drawSourcesFailure 画"无法读取索引源" + 重试（官方 `state.status === 'error'`，:602-612）。
func (d *Discover) drawSourcesFailure(f ui.C, env *pluginapi.Env) {
	t := d.theme
	ui.Row(f).FillWidth().Items(ui.Center).Gap(discoverFailureGap).Children(func() {
		ui.Text(f, zhSourcesFailed).FontSize(discoverErrorSize).LineHeight(discoverErrorLine).
			Color(t.Error)
		if env.Act.MarketIndexesReload != nil {
			button := d.smallButton(f, marketButtonOutline).Hit(d.id(env, "market-indexes-retry"), zhRetry)
			button.Children(func() { marketButtonLabel(f, t, zhRetry, marketButtonOutline) })
			if button.Clicked() {
				env.Act.MarketIndexesReload()
			}
		}
	})
}

// drawEmpty 画"还没有插件索引" + 「添加索引源」（官方 :585-605）。
func (d *Discover) drawEmpty(f ui.C, env *pluginapi.Env) {
	t := d.theme
	ui.Column(f).FillWidth().Items(ui.Center).Gap(discoverEmptyGap).
		PadSides(discoverEmptyPad, 0, discoverEmptyPad, 0).Children(func() {
		drawMarketArtwork(f, discoverIconSize)
		ui.Text(f, zhEmptyTitle).MarginSides(discoverEmptyExtraGap, 0, 0, 0).
			FontSize(discoverEmptySize).LineHeight(discoverEmptyLine).
			Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		ui.Wrap(f, zhEmptyDescription).FontSize(discoverMutedSize).LineHeight(discoverMutedLine).
			Color(t.LabelTertiary).TextCenter()
		if env.Act.MarketSetIndexes != nil {
			button := d.smallButton(f, marketButtonPrimary).
				Hit(d.id(env, "market-add-first-source"), zhAddFirstSource).MarginSides(discoverEmptyExtraGap, 0, 0, 0)
			button.Children(func() {
				drawMarketPlusButtonIcon(f, t.ButtonPrimaryFg)
				marketButtonLabel(f, t, zhAddFirstSource, marketButtonPrimary)
			})
			if button.Clicked() {
				d.editing = true
			}
		}
	})
}

// drawToolbar 是官方 `.toolbar`（:492-560）：索引菜单 → 搜索框 → 标签过滤 → 排序 → 刷新。
func (d *Discover) drawToolbar(f ui.C, env *pluginapi.Env) {
	ui.Row(f).FillWidth().Items(ui.Center).Gap(discoverToolbarGap).Children(func() {
		d.drawMarketMenu(f, env)
		t := d.theme
		ui.Row(f).Grow(1).MinWidth(discoverSearchMin).Height(marketInputHeight).
			Radius(t.Radii.MD).Border(marketInputBorder, t.BorderL4).Background(t.BgLayer1).
			Items(ui.Center).Children(func() {
			search := ui.TextFieldBase(f, &d.search, zhSearch, zhSearch).
				Grow(1).Height(marketInputHeight).FontSize(marketInputFont).LineHeight(marketInputLine).
				Color(t.LabelPrimary).
				PadSides(0, marketInputPadX, 0, marketInputPadX+marketInputIcon+marketInputGap)
			if icon, ok := dsassets.WorkspaceSearch(t.LabelTertiary); ok {
				ui.Icon(f, icon).Absolute().Left(marketInputPadX+(marketInputIcon-marketInputGlyph)/2).
					Top((marketInputHeight-marketInputGlyph)/2).
					Size(marketInputGlyph, marketInputGlyph).Color(t.LabelTertiary)
			}
			if search.Changed() {
				d.query = WithText(d.query, d.search.Text())
				d.searchNow(env)
			}
		})
		labels := d.labelDeclarations()
		if len(labels) > 0 {
			d.drawFilterMenu(f, env, labels)
		}
		d.drawSortMenu(f, env, d.sortKeyDeclarations())
		d.drawRefresh(f, env)
	})
}

// drawMarketMenu 画索引切换菜单（官方 `.marketButton` + `Menu`，:493-511）。
func (d *Discover) drawMarketMenu(f ui.C, env *pluginapi.Env) {
	t := d.theme
	label := d.marketLabel()
	box := d.smallButton(f, marketButtonGhost).
		Hit(d.id(env, "market-menu"), label).MaxWidth(discoverMarketMax).MarginSides(0, 0, 0, discoverMarketPull)
	box.Children(func() {
		ui.Text(f, label).Shrink(1).FontSize(marketButtonFont).LineHeight(marketButtonLine).
			Weight(gfx.WeightMedium).Color(t.LabelPrimary).Ellipsis()
		d.drawChevron(f, t)
	})
	if box.Clicked() {
		d.menuMarket = !d.menuMarket
		d.menuFilter, d.menuSort = false, false
	}
	open := d.menuMarket
	ui.Menu(f, box, &open, func(items *ui.MenuItems) {
		for _, index := range d.indexes {
			if items.Item(index.PickLabel()).Chosen(index.URL == d.selected).Clicked() {
				if env.Act.MarketSelectIndex != nil {
					env.Act.MarketSelectIndex(index.URL)
				}
			}
		}
		// 官方 `footer: [{id:'manage', label: t('manageSources')}]`（:504-507）。
		if items.Item(zhManageSources).Clicked() {
			d.editing = true
		}
	})
	d.menuMarket = open
}

// drawFilterMenu 画标签过滤菜单（官方 `Menu` + `items`，:513-533）。
func (d *Discover) drawFilterMenu(f ui.C, env *pluginapi.Env, labels []Declaration) {
	t := d.theme
	label := FilterMenuLabel(len(d.query.Labels))
	box := d.smallButton(f, marketButtonGhost).Hit(d.id(env, "market-filter"), label)
	box.Children(func() {
		drawMarketSlidersIcon(f, t.LabelPrimary, 13)
		marketButtonLabel(f, t, label, marketButtonGhost)
	})
	if box.Clicked() {
		d.menuFilter = !d.menuFilter
		d.menuMarket, d.menuSort = false, false
	}
	open := d.menuFilter
	ui.Menu(f, box, &open, func(items *ui.MenuItems) {
		for _, declaration := range labels {
			if items.Item(declaration.Title).Chosen(containsString(d.query.Labels, declaration.ID)).Clicked() {
				d.query.Labels = ToggleLabel(d.query.Labels, declaration.ID)
				d.searchNow(env)
			}
		}
	})
	d.menuFilter = open
}

// drawSortMenu 画排序菜单（官方 `Menu` + `sortOptions` + 方向切换，:535-556）。
func (d *Discover) drawSortMenu(f ui.C, env *pluginapi.Env, sortKeys []Declaration) {
	t := d.theme
	label := SortMenuLabel(d.query, sortKeys)
	box := d.smallButton(f, marketButtonGhost).Hit(d.id(env, "market-sort"), label)
	box.Children(func() {
		marketButtonLabel(f, t, label, marketButtonGhost)
		d.drawChevron(f, t)
	})
	if box.Clicked() {
		d.menuSort = !d.menuSort
		d.menuMarket, d.menuFilter = false, false
	}
	open := d.menuSort
	ui.Menu(f, box, &open, func(items *ui.MenuItems) {
		if d.chooseSortItems(items, sortKeys) {
			d.searchNow(env)
		}
	})
	d.menuSort = open
}

// chooseSortItems 声明排序菜单并处理选择；返回 true = query 变了要重读。
func (d *Discover) chooseSortItems(items *ui.MenuItems, sortKeys []Declaration) bool {
	current := SortMenuID(d.query.Sort)
	for _, option := range SortMenuOptions(d.query, sortKeys) {
		if !items.Item(option.Label).Chosen(option.ID == current).Clicked() {
			continue
		}
		switch option.ID {
		case apiclient.MarketSortIndex, apiclient.MarketSortRelevance:
			d.query.Sort = apiclient.MarketSort{Kind: option.ID}
		case apiclient.MarketSortPublished:
			d.query.Sort = apiclient.MarketSort{Kind: apiclient.MarketSortPublished, Direction: "desc"}
		default:
			for _, key := range sortKeys {
				if "key:"+key.ID != option.ID {
					continue
				}
				direction := key.Order
				if direction != "asc" && direction != "desc" {
					direction = "desc"
				}
				d.query.Sort = apiclient.MarketSort{Kind: apiclient.MarketSortKey, ID: key.ID, Direction: direction}
			}
		}
		return true
	}
	// 方向切换：官方在定向排序时把 `switchToAscending/Descending` 放在页脚。
	if Directed(d.query.Sort) {
		flipped := zhSwitchToAscending
		if d.query.Sort.Direction == "asc" {
			flipped = zhSwitchToDescending
		}
		if items.Item(flipped).Clicked() {
			direction := "desc"
			if d.query.Sort.Direction == "desc" {
				direction = "asc"
			}
			d.query.Sort = apiclient.MarketSort{Kind: d.query.Sort.Kind, ID: d.query.Sort.ID, Direction: direction}
			return true
		}
	}
	return false
}

// drawRefresh 画刷新按钮（官方 `Button` + 刷新图标，:557-563）。
//
// 官方是纯图标 small ghost 按钮（aria-label `refresh`），图标 14px，
// 外层 icon 槽 16px（Button.module.css:78-84）。
func (d *Discover) drawRefresh(f ui.C, env *pluginapi.Env) {
	if env.Act.MarketRefresh == nil {
		return
	}
	busy := d.listing.Status == pluginapi.MarketListingLoading
	button := d.smallButton(f, marketButtonGhost).Hit(d.id(env, "market-refresh"), zhRefresh).Disabled(busy)
	if busy {
		button = button.Opacity(0.4) // Button.module.css:18-21
	}
	button.Children(func() {
		if icon, ok := dsassets.MarketRefreshOutline(d.theme.LabelPrimary); ok {
			drawMarketButtonIcon(f, icon, 14, d.theme.LabelPrimary)
		}
	})
	if button.Clicked() && !busy {
		env.Act.MarketRefresh()
	}
}

// drawCategories 画分类胶囊行（官方 `.categories` + `Pill`，:565-583）。
func (d *Discover) drawCategories(f ui.C, env *pluginapi.Env) {
	categories := d.categoryDeclarations()
	if len(categories) == 0 {
		return
	}
	ui.Row(f).FillWidth().Items(ui.Center).Gap(discoverCategoriesGap).Children(func() {
		if d.drawPill(f, env, "market-category-all", zhAllCategories, d.query.Category == "") {
			d.query.Category = ""
			d.searchNow(env)
		}
		for _, category := range categories {
			active := d.query.Category == category.ID
			if d.drawPill(f, env, "market-category-"+category.ID, category.Title, active) {
				d.query.Category = category.ID
				d.searchNow(env)
			}
		}
	})
}

// drawPill 画一枚可点的分类胶囊（官方 `Pill`：`.categories` 下的圆角小按钮）。
//
// 官方 ui-primitives/src/Pill.module.css:1-14：24px 高、0 8px、12/18、999px。
// 选中态是 ghost-active-fill + 1px inset border（同文件 :26-30）。
func (d *Discover) drawPill(f ui.C, env *pluginapi.Env, key, label string, active bool) bool {
	t := d.theme
	bg, fg := t.BgLayer2, t.LabelSecondary
	box := ui.Row(f).Height(marketPillHeight).Shrink(0).Radius(marketPillRadius).Items(ui.Center).
		PadSides(0, marketPillPadX, 0, marketPillPadX).Background(bg).Hit(d.id(env, key), label)
	if active {
		bg, fg = t.NavActive, t.LabelPrimary
		box = box.Background(bg).Border(1, t.ButtonGhostActiveBorder)
	} else {
		box = box.Hover(ui.Look{}.Background(t.Hover))
	}
	box.Children(func() {
		ui.Text(f, label).FontSize(marketPillFont).LineHeight(marketPillLine).Color(fg)
	})
	return box.Clicked()
}

// drawListing 画失败行 + 结果（官方 `listing.failure` / 骨架 / 空 / 卡片 / 显示更多）。
func (d *Discover) drawListing(f ui.C, env *pluginapi.Env) {
	t := d.theme
	if d.listing.Status == pluginapi.MarketListingError {
		ui.Row(f).FillWidth().Items(ui.Center).Gap(discoverFailureGap).Children(func() {
			ui.Text(f, zhReadFailed).FontSize(discoverErrorSize).LineHeight(discoverErrorLine).Color(t.Error)
			if env.Act.MarketRetry != nil {
				button := d.smallButton(f, marketButtonOutline).Hit(d.id(env, "market-listing-retry"), zhRetry)
				button.Children(func() { marketButtonLabel(f, t, zhRetry, marketButtonOutline) })
				if button.Clicked() {
					env.Act.MarketRetry()
				}
			}
		})
	}
	value := d.listing.Value
	if value == nil {
		if d.listing.Status == pluginapi.MarketListingLoading {
			d.drawLoading(f)
		}
		return
	}
	if value.Total == 0 {
		// 官方按"结果回答的那个 query 有没有过滤"分两句（:568-570）。
		message := zhNoMatches
		if d.listing.Answered != nil && Unfiltered(QueryFromWire(*d.listing.Answered)) {
			message = zhEmptyIndex
		}
		ui.Text(f, message).FillWidth().FontSize(discoverMutedSize).LineHeight(discoverMutedLine).
			Color(t.LabelTertiary)
		return
	}
	entries := d.projectEntries(value)
	ui.Column(f).FillWidth().Gap(discoverCardsGap).Children(func() {
		for i := range entries {
			d.drawCard(f, env, entries[i])
		}
	})
	if value.NextOffset != nil && env.Act.MarketMore != nil {
		disabled := d.listing.Status != pluginapi.MarketListingReady
		more := d.smallButton(f, marketButtonOutline).
			Hit(d.id(env, "market-more"), zhShowMore).Self(ui.Center).Disabled(disabled)
		if disabled {
			more = more.Opacity(0.4)
		}
		more.Children(func() { marketButtonLabel(f, t, zhShowMore, marketButtonOutline) })
		if more.Clicked() && !disabled {
			env.Act.MarketMore()
		}
	}
}

// projectEntries 把当前结果投影成 Entry（同一份结果只投影一次）。
func (d *Discover) projectEntries(value *apiclient.MarketSearchResult) []Entry {
	key := entriesCacheKey{url: value.URL, fetchedAt: value.FetchedAt, total: value.Total, length: len(value.Entries)}
	if d.entriesKey == key && d.entriesList != nil {
		return d.entriesList
	}
	d.entriesKey = key
	d.entriesList = EntriesFromResult(*value)
	return d.entriesList
}

// drawCard 画一条结果卡（官方 `EntryCard`，:150-165）。
//
// 官方卡片右侧的 `EntryAction`（安装/更新/在已安装中查看）**不画**：宿主
// installBundle/updateBundle 是结构化 501（见文件头第 1 条）。
func (d *Discover) drawCard(f ui.C, env *pluginapi.Env, entry Entry) {
	t := d.theme
	ui.Row(f).FillWidth().Items(ui.Center).Gap(discoverCardGap).
		MarginSides(0, discoverCardPull, 0, discoverCardPull).
		PadSides(discoverCardPad, discoverCardPad, discoverCardPad, discoverCardPad).
		Radius(t.Radii.XL).Children(func() {
		// `.icon`：48px 框 + 0.5px 描边，框内是 36px 官方默认插件图。
		drawMarketIconFrame(f, t)
		ui.Column(f).Grow(1).MinWidth(0).Gap(mainGap).Children(func() {
			ui.Row(f).FillWidth().Items(ui.Center).Gap(titleRowGap).Children(func() {
				title := ui.Text(f, entry.TitleText()).Shrink(1).FontSize(strongSize).
					LineHeight(strongLine).Weight(gfx.WeightMedium).Color(t.LabelPrimary).
					Ellipsis().Hit(d.id(env, "market-entry-"+entry.Name), entry.TitleText())
				if title.Clicked() {
					d.Open(entry, d.selected)
				}
				for _, label := range entry.LabelTitles {
					drawTagPill(f, t, Tag{Text: label, Tone: TagNeutral})
				}
			})
			if entry.Description != "" {
				ui.Text(f, entry.Description).FillWidth().FontSize(discoverDescSize).
					LineHeight(discoverDescLine).Color(t.LabelTertiary).Ellipsis()
			}
			if meta := EntryMeta(entry, d.language); meta != "" {
				ui.Text(f, meta).FillWidth().FontSize(metaSize).LineHeight(metaLine).
					Color(t.LabelTertiary).Ellipsis()
			}
		})
	})
}

// drawChevron 画官方 `IconChevronDownOutline`，glyph 12px，外层 icon 槽 16px。
func (d *Discover) drawChevron(f ui.C, t *pluginapi.Theme) {
	commands, ok := dsassets.ChevronDownOutline()
	if !ok || len(commands) == 0 {
		return
	}
	icon := gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}
	drawMarketButtonIcon(f, icon, 12, t.LabelPrimary)
}

// ── 官方 small Button / 图标素材 ───────────────────────────────────────

type marketButtonKind uint8

const (
	marketButtonGhost marketButtonKind = iota
	marketButtonOutline
	marketButtonPrimary
)

// smallButton 是 primitives 的 size="sm"：28px 高、0 10px、4px gap、radius-sm。
func (d *Discover) smallButton(f ui.C, kind marketButtonKind) ui.Box {
	t := d.theme
	box := ui.Row(f).Shrink(0).Height(marketButtonHeight).Items(ui.Center).Justify(ui.Center).
		Gap(marketButtonGap).PadSides(0, marketButtonPadX, 0, marketButtonPadX).
		Radius(t.Radii.SM)
	switch kind {
	case marketButtonPrimary:
		box = box.Background(t.ButtonPrimaryFill)
	case marketButtonOutline:
		box = box.Border(marketButtonBorder, t.BorderL3).Hover(ui.Look{}.Background(t.Hover))
	default:
		box = box.Hover(ui.Look{}.Background(t.Hover))
	}
	return box
}

func marketButtonLabel(f ui.C, t *pluginapi.Theme, label string, kind marketButtonKind) {
	color := t.LabelPrimary
	if kind == marketButtonPrimary {
		color = t.ButtonPrimaryFg
	}
	ui.Text(f, label).FontSize(marketButtonFont).LineHeight(marketButtonLine).Color(color)
}

// drawMarketButtonIcon 画 Button.module.css:78-84 的 16×16 icon 槽。
func drawMarketButtonIcon(f ui.C, icon gfx.Icon, glyphSize float32, color gfx.Color) {
	ui.Row(f).Width(marketButtonIcon).Height(marketButtonIcon).Shrink(0).
		Items(ui.Center).Justify(ui.Center).Children(func() {
		ui.Icon(f, icon).Size(glyphSize, glyphSize).Color(color)
	})
}

func drawMarketPlusButtonIcon(f ui.C, color gfx.Color) {
	if icon, ok := dsassets.MarketPlusOutline(color); ok {
		drawMarketButtonIcon(f, icon, 13, color)
	}
}

// drawMarketSlidersIcon 复用官方两条 path + 两枚圆环，不用文字代替过滤图标。
func drawMarketSlidersIcon(f ui.C, color gfx.Color, size float32) {
	icon, rings, ok := dsassets.WorkspaceSlidersTwo(color)
	if !ok {
		return
	}
	inset := (marketButtonIcon - size) / 2
	scale := size / 16
	ui.Row(f).Width(marketButtonIcon).Height(marketButtonIcon).Shrink(0).Children(func() {
		ui.Icon(f, icon).Absolute().Left(inset).Top(inset).Size(size, size).Color(color)
		for _, ring := range rings {
			d := 2 * ring.Radius * scale
			ui.Row(f).Absolute().Left(inset+(ring.CX-ring.Radius)*scale).Top(inset+(ring.CY-ring.Radius)*scale).
				Width(d).Height(d).Radius(gfx.RadiusFull).Border(ring.Stroke*scale, color)
		}
	})
}

func drawMarketArtwork(f ui.C, size float32) {
	if icon, ok := dsassets.MarketPluginArtworkDefault(); ok {
		ui.Icon(f, icon).Size(size, size).Color(gfx.Color{Alpha: 1})
	}
}

func drawMarketIconFrame(f ui.C, t *pluginapi.Theme) {
	ui.Row(f).Width(discoverIconSize).Height(discoverIconSize).Shrink(0).
		Radius(t.Radii.LG).Border(discoverIconBorder, t.BorderL3).
		Items(ui.Center).Justify(ui.Center).Children(func() {
		drawMarketArtwork(f, marketArtworkSize)
	})
}

// ── 索引源对话框（官方 SourcesDialog，:318-350）──────────────────────────

// drawSourcesDialog 画索引源对话框：已添加的源 + 新增表单 + 失败行 + 信任提示。
func (d *Discover) drawSourcesDialog(f ui.C, env *pluginapi.Env) {
	if !d.editing {
		return
	}
	t := d.theme
	ui.ModalWith(f, &d.editing, zhSourcesTitle, ui.ModalSpec{
		Description: zhSourcesDescription,
		Width:       discoverDialogWidth,
	}, func() {
		ui.Column(f).FillWidth().Gap(discoverDialogGap).Children(func() {
			if len(d.indexes) > 0 {
				ui.Column(f).FillWidth().Children(func() {
					for _, index := range d.indexes {
						d.drawSourceRow(f, env, index)
					}
				})
			}
			d.drawSourceForm(f, env)
			if d.sourceFailure != "" {
				ui.Text(f, sourceFailureText(d.sourceFailure)).FillWidth().
					FontSize(discoverErrorSize).LineHeight(discoverErrorLine).Color(t.Error)
			}
			drawMarketNotice(f, t, zhSourcesTrust)
		})
	}, nil)
}

// drawSourceRow 画一行已添加的索引源（官方 `.sourceRow`：标题 + URL + 错误 + 移除）。
func (d *Discover) drawSourceRow(f ui.C, env *pluginapi.Env, index apiclient.MarketIndexSummary) {
	t := d.theme
	ui.Row(f).FillWidth().Items(ui.Center).Gap(discoverSourceGap).
		PadSides(discoverSourceGap, 0, discoverSourceGap, 0).
		Border(versionBorderW, t.BorderL2).Children(func() {
		ui.Column(f).Grow(1).MinWidth(0).Gap(mainGap).Children(func() {
			ui.Text(f, index.PickLabel()).FillWidth().FontSize(strongSize).LineHeight(strongLine).
				Weight(gfx.WeightMedium).Color(t.LabelPrimary).Ellipsis()
			ui.Text(f, index.URL).FillWidth().FontSize(metaSize).LineHeight(metaLine).
				Color(t.LabelTertiary).Ellipsis()
			if index.Error != "" {
				ui.Text(f, index.Error).FillWidth().FontSize(discoverNoticeSize).
					LineHeight(discoverNoticeLine).Color(t.Error).Ellipsis()
			}
		})
		if env.Act.MarketSetIndexes != nil {
			// 官方这一枚是 13px trash 图标，aria-label 为 removeSourceNamed。
			named := fillTemplate(zhRemoveSourceNamed, map[string]string{"url": index.URL})
			button := d.smallButton(f, marketButtonGhost).
				Hit(d.id(env, "market-source-remove-"+index.URL), named).Disabled(d.saving)
			if d.saving {
				button = button.Opacity(0.4)
			}
			button.Children(func() {
				if icon, ok := dsassets.MarketTrashOutline(t.LabelPrimary); ok {
					drawMarketButtonIcon(f, icon, 13, t.LabelPrimary)
				}
			})
			if button.Clicked() && !d.saving {
				env.Act.MarketSetIndexes(d.sourceURLsExcept(index.URL))
			}
		}
	})
}

// drawSourceForm 画新增索引源的输入框与「添加」（官方 `.sourceForm`）。
func (d *Discover) drawSourceForm(f ui.C, env *pluginapi.Env) {
	t := d.theme
	ui.Row(f).FillWidth().Items(ui.Center).Gap(discoverToolbarGap).Children(func() {
		ui.Row(f).Grow(1).MinWidth(0).Height(marketInputHeight).Radius(t.Radii.MD).
			Border(marketInputBorder, t.BorderL4).Background(t.BgLayer1).Items(ui.Center).Children(func() {
			ui.TextFieldBase(f, &d.draft, zhSourceURL, zhSourcePlaceholder).
				Grow(1).Height(marketInputHeight).FontSize(marketInputFont).LineHeight(marketInputLine).
				Color(t.LabelPrimary).PadSides(0, marketInputPadX, 0, marketInputPadX)
		})
		if env.Act.MarketSetIndexes == nil {
			return
		}
		disabled := d.saving || strings.TrimSpace(d.draft.Text()) == ""
		add := d.smallButton(f, marketButtonPrimary).Hit(d.id(env, "market-source-add"), zhAddSource).Disabled(disabled)
		if disabled {
			add = add.Opacity(0.4)
		}
		add.Children(func() {
			drawMarketPlusButtonIcon(f, t.ButtonPrimaryFg)
			marketButtonLabel(f, t, zhAddSource, marketButtonPrimary)
		})
		if add.Clicked() && !disabled {
			urls := d.sourceURLsExcept("")
			urls = append(urls, strings.TrimSpace(d.draft.Text()))
			env.Act.MarketSetIndexes(urls)
		}
	})
}

// sourceURLsExcept 返回现有索引源（去掉 next 那个），追加新源时用它拼列表。
func (d *Discover) sourceURLsExcept(skip string) []string {
	urls := make([]string, 0, len(d.indexes)+1)
	for _, index := range d.indexes {
		if index.URL == skip {
			continue
		}
		urls = append(urls, index.URL)
	}
	return urls
}

// sourceFailureText 取官方四档之一的 zh 文案（`OFF:DiscoverSection.tsx:67-76`）。
//
// duplicate / unavailable 两档宿主产不出（见 internal/apiclient 的
// MarketSourceFailureOf），所以这里按 other 的兜底句画——**不编**第八种文案。
func sourceFailureText(kind apiclient.MarketSourceFailureKind) string {
	switch kind {
	case apiclient.MarketSourceInvalid:
		return zhSourceInvalid
	}
	return zhSourceFailed
}

// ── 动作 ────────────────────────────────────────────────────────────────

// searchNow 把当前 query 交给壳（壳内 250ms 去抖，插件这边不自己计时）。
func (d *Discover) searchNow(env *pluginapi.Env) {
	if env.Act.MarketSearch == nil {
		return
	}
	env.Act.MarketSearch(d.query.Wire(), d.query.Text)
}

// marketLabel 是索引菜单触发器上那一句（官方 `indexLabel`，`OFF:presentation.ts:106-115`）。
func (d *Discover) marketLabel() string {
	for _, index := range d.indexes {
		if index.URL == d.selected {
			return index.PickLabel()
		}
	}
	if d.selected == "" {
		return ""
	}
	// 索引行还没读到（刚选中就画了一帧）：退回 URL 的 host，与官方同一个兜底。
	return apiclient.MarketIndexSummary{URL: d.selected}.PickLabel()
}

// categoryDeclarations / labelDeclarations / sortKeyDeclarations 读当前索引的声明表。
//
// 声明表随结果回来（`MarketSearchResult.metadata`）；还没结果时为空——官方同样
// 在 `value === null` 时把三组声明当空（`DiscoverSection.tsx:459-464`）。
func (d *Discover) categoryDeclarations() []Declaration {
	if d.listing.Value == nil {
		return nil
	}
	return Categories(d.listing.Value.Metadata)
}

func (d *Discover) labelDeclarations() []Declaration {
	if d.listing.Value == nil {
		return nil
	}
	return Labels(d.listing.Value.Metadata)
}

func (d *Discover) sortKeyDeclarations() []Declaration {
	if d.listing.Value == nil {
		return nil
	}
	return SortKeys(d.listing.Value.Metadata)
}

// id 造一个交互 id（env.Tree 为 nil 时返回零 id，与 detail.go 同一写法）。
func (d *Discover) id(env *pluginapi.Env, name string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, name)
}
