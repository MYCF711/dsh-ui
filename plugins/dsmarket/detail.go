package dsmarket

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 官方几何 ────────────────────────────────────────────────────────────────
//
// 每一条都来自官方 CSS（`OFF:../dsh-client-ui-plugin-market/src/client/DiscoverSection.module.css`），
// 行号写在注释里。**不要**在这里发明数值：官方没有的就写进"缺口"，不拿近似值顶上。
const (
	// `.detail{display:flex;flex-direction:column;gap:16px}`（:236-240）。
	detailGap float32 = 16
	// `.crumb{display:inline-flex;align-items:center;gap:6px;font-size:12.5px}`（:242-270）。
	crumbSize float32 = 12.5
	crumbGap  float32 = 6
	// `.detailHead{display:flex;align-items:center;gap:14px}`（:271-275）。
	headGap float32 = 14
	// `.detailTitle{margin:0;font-size:20px;line-height:28px;font-weight:500}`（:277-282）。
	detailTitleSize float32 = 20
	detailTitleLine float32 = 28
	// `.detailDescription{font-size:14px;line-height:22px;color:label-secondary}`（:284-290）。
	detailDescSize float32 = 14
	detailDescLine float32 = 22
	// `.detailDocument{margin-top:16px}`（:299-305）；`.detailDocument:empty{display:none}`（:303-305）。
	documentTop float32 = 16
	// `.detailSection{display:flex;flex-direction:column;gap:12px;margin-top:16px}`（:292-297）。
	sectionTop float32 = 16
	sectionGap float32 = 12
	// `.sectionTitle{margin:0;font-size:14px;line-height:20px;font-weight:500}`（:307-312）。
	sectionTitleSize float32 = 14
	sectionTitleLine float32 = 20
	// `.versions{display:flex;flex-direction:column;list-style:none;margin:0;padding:0}`（:368-377）
	// `.versionRow{display:flex;align-items:center;gap:8px;padding:8px 0;
	//   border-bottom:0.5px solid border-l2}`（:379-385）。
	versionRowGap  float32 = 8
	versionRowPadY float32 = 8
	versionBorderW float32 = 0.5
	// `.strong{font-size:14px;line-height:20px;font-weight:500}`（:55-60）。
	strongSize float32 = 14
	strongLine float32 = 20
	// `.meta{font-size:12px;line-height:18px;color:label-tertiary}`（:41-53）。
	metaSize float32 = 12
	metaLine float32 = 18
	// `.main{display:flex;flex:1;flex-direction:column;gap:2px;min-width:0}`（:42-48）。
	mainGap float32 = 2
	// `.titleRow{display:flex;flex-wrap:wrap;align-items:center;gap:8px;min-width:0}`（:50-56）。
	titleRowGap float32 = 8
	// `.notice{padding:10px 12px;border-radius:radius-md;background:warn-tertiary;
	//   font-size:12px;line-height:18px;color:label-secondary;gap:8px}`（:78-95）。
	noticePadY float32 = 10
	noticePadX float32 = 12
	noticeSize float32 = 12
	noticeLine float32 = 18
	// `.facts{margin:0}`；`.facts div{display:flex;align-items:center;justify-content:space-between;
	//   gap:16px;min-height:44px;padding:8px 2px;border-bottom:0.5px solid border-l2}`（:314-342）。
	factMinHeight float32 = 44
	factPadY      float32 = 8
	factPadX      float32 = 2
	factGap       float32 = 16
	// `.facts dt/dd{font-size:13px;line-height:20px}`（:328-342）。
	factSize float32 = 13
	factLine float32 = 20
	// `.facts code{padding:2px 6px;border-radius:4px;background:bg-module-platform;
	//   font-family:code;font-size:12px}`（:344-352）。
	factCodePadY   float32 = 2
	factCodePadX   float32 = 6
	factCodeRadius float32 = 4
	factCodeSize   float32 = 12
	// `.tags{display:flex;flex-wrap:wrap;justify-content:flex-end;gap:6px}`（:353-360）。
	tagsGap float32 = 6

	// Tag 的度量：官方 `Tag` 在 `@deepseek-ai/dsh-client-ui-primitives` 里，而那份包
	// **随包不发 CSS**（本机 rc.2 的 node_modules 里没有它），所以取本仓已经在用的
	// 同一份官方度量——`OFF:../dsh-client-ui-plugin-manager/src/client/PluginManagerPage.module.css:332-337`
	// 的 `.statusTag{height:18px;padding:0 7px;font-size:10px}`（与 `plugins/dssidebar`
	// 的 `drawPluginTag` 同一个来源）。
	tagHeight float32 = 18
	tagPadX   float32 = 7
	tagFont   float32 = 10

	// 页面内距与最大宽度：详情页在官方是 **Plugins 页**里的一个 tab，页面容器就是
	// `.page`（`OFF:../dsh-client-ui-plugin-manager/src/client/PluginManagerPage.module.css`
	// 的 `.page{padding:0 clamp(24px,4vw,48px) 48px}` 与 `.page > *{max-width:960px}`）。
	// 这两个数沿用 `plugins/dssidebar` 已冻结的同一组（`pluginPagePadMinX` 等）。
	pagePadMinX   float32 = 24
	pagePadMaxX   float32 = 48
	pagePadVw     float32 = 0.04
	pageBottomPad float32 = 48
	pageMaxWidth  float32 = 960
	// 首帧还没量到宽度时的估算值：详情页最窄也在 320 以上。
	pageFallbackWidth float32 = 640
)

// ReadRequest 是一次详情读的身份：官方 `(url, name, version, language)`
// （`OFF:../dsh-plugin-market/lib/typert.host.js:17-24`）。
//
// 语言不在这个结构里：它是界面当前语言，由 `Page.language` 交给壳
// （`pluginapi.Actions.MarketEntryDetail` 的第四个参数）。
type ReadRequest struct {
	// URL 是这条条目所在的**索引源** URL（官方第一个位置参数）。
	URL string
	// Name 是包名，Version 是这一版的版本号。
	Name    string
	Version string
}

// Page 是 Discover tab 里**一条市场条目的详情页**。
//
// 生命周期由打开它的那一层（A 块的 Discover tab）管：`Open` → 每帧 `Draw` /
// `AfterDraw` → `Close`。本页不发请求、不持连接，只把 entry + 详情读结果画出来。
type Page struct {
	theme *pluginapi.Theme

	entry     Entry
	marketURL string
	open      bool
	language  string
	// back 是面包屑的返回动作；nil = 没有返回目标（不画面包屑，见 drawCrumb）。
	back func()

	// 详情文档的排版缓存：`MarkdownView.Layout` 很贵（要量每一行），正文与宽度
	// 都没变就复用同一份——与 `plugins/dsplan/preview.go` 同一条判据。
	doc docCache
	// widthSeen 是上一帧量到的真实内容宽度（holder 的 rect 宽）。
	widthSeen float32

	// boxes 是本帧的几何，**只在同一帧内有效**：Draw 里写、AfterDraw 里读。
	boxes pageBoxes
}

type pageBoxes struct {
	scroll ui.Box
	holder ui.Box
}

// NewPage 造一页详情。theme 来自 `ui.theme`（ds-core）。
//
// 详情读**不在**这里注入：它走 `env.Act.MarketEntryDetail`
// （壳侧数据面，与 `PluginBundleDetail` 同一个形状）。理由：插件不该持有数据源，
// 也就不可能在 Draw 里偷偷发请求。
func NewPage(theme *pluginapi.Theme) *Page {
	return &Page{theme: theme, language: "zh-CN"}
}

// Open 打开一条条目的详情页。marketURL 是它所在的索引源（详情读的第一个参数）。
func (p *Page) Open(entry Entry, marketURL string) {
	p.entry = entry
	p.marketURL = marketURL
	p.open = true
	p.doc = docCache{}
}

// Close 关掉详情页（回到列表由调用方负责）。
func (p *Page) Close() {
	p.open = false
	p.entry = Entry{}
	p.marketURL = ""
	p.doc = docCache{}
}

// Opened 报告是否有一页开着，并返回它的条目（测试与壳的无鼠标入口用）。
func (p *Page) Opened() (Entry, bool) { return p.entry, p.open }

// SetBack 设置面包屑的返回动作。
func (p *Page) SetBack(back func()) { p.back = back }

// SetLanguage 设置详情读与日期用的语言（官方 BCP-47）。
func (p *Page) SetLanguage(language string) {
	if language != "" {
		p.language = language
	}
}

// Draw 画整页（官方 `EntryDetail`，`OFF:…/DiscoverSection.tsx:265-355`）。
func (p *Page) Draw(f ui.C, env *pluginapi.Env) {
	p.boxes = pageBoxes{}
	if p == nil || env == nil || p.theme == nil || !p.open {
		return
	}
	version, hasVersion := PresentedVersion(p.entry)
	// 文档高度必须在**排版之前**知道：它下面还有版本列表与 facts 两段。
	// 这与官方不同（官方是 DOM 自己流），所以这里按本仓 dsplan 的既定做法先量后画。
	docHeight := p.layoutDocument(env, p.contentWidth(env), version, hasVersion)

	ui.Column(f).FillWidth().FillHeight().Children(func() {
		scroll := ui.Scroll(f.Key("market-detail")).FillWidth().Grow(1).Clip()
		p.boxes.scroll = scroll
		scroll.Children(func() {
			ui.Column(f).FillWidth().PadSides(0, pagePadX(env), pageBottomPad, pagePadX(env)).Children(func() {
				ui.Column(f).FillWidth().MaxWidth(pageMaxWidth).
					Gap(detailGap).Children(func() {
					p.drawCrumb(f, env)
					p.drawHead(f, env, version, hasVersion)
					p.drawDescription(f)
					p.drawNotice(f, version, hasVersion)
					p.drawDocument(f, env, docHeight, version, hasVersion)
					p.drawVersions(f, env)
					p.drawFacts(f, env, version, hasVersion)
				})
			})
		})
	})
}

// drawCrumb 画面包屑（官方回到 Discover 列表的那一行）。
//
// 没有返回目标时**不画**：官方那枚按钮的作用是"回到列表"，而本仓的列表
// （A 块的 Discover tab）还不存在——画一个点了没反应的按钮比不画更糟
// （同 `plugins/dssidebar/PluginOpenDetail` 返回 false 时不许画入口的理由）。
//
// 图标用文字字形 `‹`：官方是 `IconChevronDownOutlineRegular` 旋转 90°
// （`.crumbIcon{transform:rotate(90deg)}`，:267-269），本仓没有官方 sprite 导出，
// 按仓规不手绘冒充（与 dssidebar 的 `‹`/`▾` 同一条先例，已在证据文档登记）。
func (p *Page) drawCrumb(f ui.C, env *pluginapi.Env) {
	t := p.theme
	label := zhBackToList
	if p.back == nil {
		// 留一行同高的空位：官方那一行永远在，占位让下面的头/说明/版本与官方
		// 处在同一个纵向位置（否则"有没有返回目标"会改变整页的相对高度）。
		ui.Row(f).Height(crumbSize + 6)
		return
	}
	id := gfx.ID{}
	if env.Tree != nil {
		id = env.Tree.ID(env.Feat, "market-detail-back")
	}
	row := ui.Row(f).Shrink(0).Items(ui.Center).Gap(crumbGap).Hit(id, label).Children(func() {
		ui.Text(f, "‹").FontSize(crumbSize).LineHeight(crumbSize + 4).Color(t.LabelTertiary)
		ui.Text(f, label).FontSize(crumbSize).LineHeight(crumbSize + 4).Color(t.LabelTertiary)
	})
	if row.Clicked() {
		p.back()
	}
}

// drawHead 画官方 `.detailHead`：标题 + 版本 Tag（+ 包名），右侧是条目动作。
//
// 右侧的 `EntryAction`（安装/更新/在已安装中查看）**不画**：它走
// `pluginManager.installBundle` / `updateBundle`，本仓这两个是结构化 501
// （见包注释第 1 条）。位置留空，不拿别的动作冒充。
func (p *Page) drawHead(f ui.C, env *pluginapi.Env, version Version, hasVersion bool) {
	t := p.theme
	title := p.entry.TitleText()
	ui.Row(f).FillWidth().Items(ui.Center).Gap(headGap).Children(func() {
		ui.Column(f).Grow(1).MinWidth(0).Gap(mainGap).Children(func() {
			ui.Row(f).FillWidth().Items(ui.Center).Gap(titleRowGap).Children(func() {
				ui.Text(f, title).Shrink(1).FontSize(detailTitleSize).LineHeight(detailTitleLine).
					Weight(gfx.WeightMedium).Color(t.LabelPrimary).Ellipsis()
				if hasVersion {
					p.drawTag(f, Tag{Text: version.Version, Tone: TagNeutral})
				}
			})
			// 官方 `{title === entry.name ? null : <span className={css.meta}>{entry.name}</span>}`。
			if title != p.entry.Name && p.entry.Name != "" {
				ui.Text(f, p.entry.Name).FillWidth().FontSize(metaSize).LineHeight(metaLine).
					Color(t.LabelTertiary).Ellipsis()
			}
		})
	})
}

// drawDescription 画官方 `.detailDescription`（没有说明就整行不画）。
func (p *Page) drawDescription(f ui.C) {
	if p.entry.Description == "" {
		return
	}
	ui.Wrap(f, p.entry.Description).FillWidth().FontSize(detailDescSize).
		LineHeight(detailDescLine).Color(p.theme.LabelSecondary)
}

// drawNotice 画官方 `.notice`：当前 DSH 与这一版不兼容时的提示条。
//
// 官方那条左侧有一枚 `IconWarningOutlineRegular`（:329-336 的 JSX）。本仓没有
// 这枚 sprite 的导出（`plugins/dsassets` 的注册表里没有 warning 三角），按仓规
// **不手绘冒充**，只画文字——已在证据文档登记。
func (p *Page) drawNotice(f ui.C, version Version, hasVersion bool) {
	if !hasVersion || version.Compatibility.Compatible {
		return
	}
	t := p.theme
	text := zhExempted
	if !version.Compatibility.Exempted {
		text = fillTemplate(zhIncompatible, map[string]string{
			"runtime": version.Compatibility.RuntimeVersion,
			"range":   requirementsRange(version.Compatibility.Requirements),
		})
	}
	ui.Wrap(f, text).FillWidth().FontSize(noticeSize).LineHeight(noticeLine).
		Color(t.LabelSecondary).PadSides(noticePadY, noticePadX, noticePadY, noticePadX).
		Radius(t.Radii.MD).Background(t.WarnTertiary)
}

// drawVersions 画官方版本列表（`.detailSection` + `.sectionTitle` + `ul.versions`）。
func (p *Page) drawVersions(f ui.C, env *pluginapi.Env) {
	if len(p.entry.Versions) == 0 {
		// 官方 `entry.versions` 至少有一条（`presentation.ts:74-77` 的注释：
		// "The Host lists at least one version"）。一条都没有说明数据没到，
		// 整段不画，而不是画一个"版本"空标题。
		return
	}
	t := p.theme
	ui.Column(f).FillWidth().PadSides(sectionTop, 0, 0, 0).Gap(sectionGap).Children(func() {
		ui.Text(f, zhVersionsTitle).FillWidth().FontSize(sectionTitleSize).
			LineHeight(sectionTitleLine).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		ui.Column(f).FillWidth().Children(func() {
			for _, row := range VersionRows(p.entry, p.language) {
				p.drawVersionRow(f, row)
			}
		})
	})
}

// drawVersionRow 画一条版本行（官方 `.versionRow`，没有行尾的安装/更新按钮）。
func (p *Page) drawVersionRow(f ui.C, row VersionRow) {
	t := p.theme
	ui.Row(f).FillWidth().Items(ui.Center).Gap(versionRowGap).
		PadSides(versionRowPadY, 0, versionRowPadY, 0).
		Border(versionBorderW, t.BorderL2).Children(func() {
		ui.Column(f).Grow(1).MinWidth(0).Gap(mainGap).Children(func() {
			ui.Row(f).FillWidth().Items(ui.Center).Gap(titleRowGap).Children(func() {
				ui.Text(f, row.Version).Shrink(1).FontSize(strongSize).LineHeight(strongLine).
					Weight(gfx.WeightMedium).Color(t.LabelPrimary).Ellipsis()
				for _, tag := range row.Tags {
					p.drawTag(f, tag)
				}
			})
			if row.Meta != "" {
				ui.Text(f, row.Meta).FillWidth().FontSize(metaSize).LineHeight(metaLine).
					Color(t.LabelTertiary).Ellipsis()
			}
		})
	})
}

// drawTag 画官方 `Tag`（胶囊）。
//
// 底/字的取色：官方 tone 用的是四套 `--dsw-alias-tag-*` 令牌，本仓 Theme 只有
// 状态容器色里成功/警告两对，所以：
//
//	success → SuccessTertiary 底 + SuccessPrimary 字（官方 success 容器那一对）
//	warning → WarnTertiary 底 + WarnLabel 字（官方 warning 容器那一对）
//	info    → BgLayer3 底 + Info 字（没有 info 容器令牌，登记差异）
//	neutral → BgLayer3 底 + LabelSecondary 字（同上）
//
// 不拿纯色降透明度凑：那不是官方那个色（同 dssidebar 的 `drawPluginTag` 注释）。
func (p *Page) drawTag(f ui.C, tag Tag) {
	t := p.theme
	bg, fg := t.BgLayer3, t.LabelSecondary
	switch tag.Tone {
	case TagSuccess:
		bg, fg = t.SuccessTertiary, t.SuccessPrimary
	case TagWarning:
		bg, fg = t.WarnTertiary, t.WarnLabel
	case TagInfo:
		fg = t.Info
	}
	ui.Row(f).Height(tagHeight).Shrink(0).Radius(tagHeight/2).
		PadSides(0, tagPadX, 0, tagPadX).Items(ui.Center).Justify(ui.Center).
		Background(bg).Children(func() {
		ui.Text(f, tag.Text).FontSize(tagFont).LineHeight(tagHeight).Color(fg)
	})
}

// drawFacts 画官方 facts 分区（`.detailSection` + `dl.facts`）。
//
// 行顺序与官方 `EntryDetail` 一致（`DiscoverSection.tsx:305-345`）：作者 → 主页 →
// 源代码 → 来源事实（来自 presentation.sourceFacts）→ 发布时间 → 分类 → 标签 →
// 插件索引。**缺席的半行不画**：本仓宿主没有那条事实时不给空值占位
// （官方也是 `=== undefined ? null : …`，同一判据）。
func (p *Page) drawFacts(f ui.C, env *pluginapi.Env, version Version, hasVersion bool) {
	if !hasVersion {
		return
	}
	t := p.theme
	ui.Column(f).FillWidth().PadSides(sectionTop, 0, 0, 0).Gap(sectionGap).Children(func() {
		ui.Text(f, zhFactSource).FillWidth().FontSize(sectionTitleSize).
			LineHeight(sectionTitleLine).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		ui.Column(f).FillWidth().Children(func() {
			if p.entry.Author != "" {
				factTextRow(f, t, zhFactAuthor, p.entry.Author)
			}
			if p.entry.Homepage != "" {
				p.factLinkRow(f, env, "homepage", zhFactHomepage, p.entry.Homepage)
			}
			if p.entry.Repository != nil && p.entry.Repository.URL != "" {
				value := p.entry.Repository.URL
				if dir := p.entry.Repository.Directory; dir != "" {
					value += " " + dir
				}
				p.factLinkRow(f, env, "source-code", zhFactSourceCode, value)
			}
			for _, fact := range SourceFacts(p.entry.Name, version) {
				factCodeRow(f, t, factLabel(fact.Kind), fact.Value)
			}
			if published := FormatDate(version.PublishTimestamp, p.language); published != "" {
				factTextRow(f, t, zhFactPublished, published)
			}
			// 分类 / 标签 / 索引名三行要官方 `metadata.categories|labels` 的**声明标题**
			// 与 `listing.title`；本仓索引文档不解析 `metadata.*`（B 块的宿主缺口），
			// 所以数据没到就不画（不拿 id 冒充标题）。
			if p.entry.CategoryTitle != "" {
				factTextRow(f, t, zhFactCategory, p.entry.CategoryTitle)
			}
			if len(p.entry.LabelTitles) > 0 {
				factTagsRow(f, t, zhFactLabels, p.entry.LabelTitles)
			}
			if p.entry.MarketTitle != "" {
				factTextRow(f, t, zhFactMarket, p.entry.MarketTitle)
			}
		})
	})
}

// factLabel 把来源事实的种类映射到官方 zh 标签。
func factLabel(kind SourceFactKind) string {
	switch kind {
	case FactPackage:
		return zhFactPackage
	case FactRegistry:
		return zhFactRegistry
	case FactRepository:
		return zhFactRepository
	case FactCommit:
		return zhFactCommit
	case FactDirectory:
		return zhFactDirectory
	}
	return ""
}

// factRow 是 `.facts` 一行的外壳：label 左、值右、底部 0.5px 细线。
func factRow(f ui.C, t *pluginapi.Theme, label string, body func()) {
	ui.Row(f).FillWidth().MinHeight(factMinHeight).Items(ui.Center).
		Justify(ui.Between).Gap(factGap).PadSides(factPadY, factPadX, factPadY, factPadX).
		Border(versionBorderW, t.BorderL2).Children(func() {
		ui.Text(f, label).Shrink(0).FontSize(factSize).LineHeight(factLine).Color(t.LabelSecondary)
		body()
	})
}

// factTextRow 画一行右对齐的纯文本事实。
func factTextRow(f ui.C, t *pluginapi.Theme, label, value string) {
	factRow(f, t, label, func() {
		ui.Text(f, value).Grow(1).FontSize(factSize).LineHeight(factLine).
			Color(t.LabelPrimary).TextEnd().Ellipsis()
	})
}

// factCodeRow 画一行等宽事实（官方 `.facts code` 的小胶囊）。
func factCodeRow(f ui.C, t *pluginapi.Theme, label, value string) {
	factRow(f, t, label, func() {
		ui.Row(f).Shrink(0).Radius(factCodeRadius).
			PadSides(factCodePadY, factCodePadX, factCodePadY, factCodePadX).
			Background(t.InlineCode).Children(func() {
			ui.Text(f, value).FontSize(factCodeSize).LineHeight(factCodeSize + 4).Color(t.LabelPrimary)
		})
	})
}

// factTagsRow 画一行小标签（官方 `.facts dd.tags`）。
func factTagsRow(f ui.C, t *pluginapi.Theme, label string, values []string) {
	factRow(f, t, label, func() {
		ui.Row(f).Grow(1).Justify(ui.End).Gap(tagsGap).Children(func() {
			for _, value := range values {
				ui.Row(f).Height(tagHeight).Shrink(0).Radius(tagHeight/2).
					PadSides(0, tagPadX, 0, tagPadX).Items(ui.Center).Justify(ui.Center).
					Background(t.BgLayer3).Children(func() {
					ui.Text(f, value).FontSize(tagFont).LineHeight(tagHeight).Color(t.LabelSecondary)
				})
			}
		})
	})
}

// factLinkRow 画一行可点击的外部链接（主页 / 源代码）。
//
// 官方这两行是 `<a href target="_blank" rel="noopener noreferrer">`；本仓没有浏览器，
// 走壳的"用系统默认程序打开"（`env.Act.OpenExternal`）。壳没接时**不画**这枚链接
// （点了没反应比没有更糟），退回纯文本。
func (p *Page) factLinkRow(f ui.C, env *pluginapi.Env, key, label, value string) {
	t := p.theme
	if env.Act.OpenExternal == nil {
		factTextRow(f, t, label, value)
		return
	}
	factRow(f, t, label, func() {
		id := gfx.ID{}
		if env.Tree != nil {
			id = env.Tree.ID(env.Feat, "market-fact-link-"+key)
		}
		link := ui.Text(f, value).Grow(1).FontSize(factSize).LineHeight(factLine).
			Color(t.Link).TextEnd().Ellipsis().Hit(id, value)
		if link.Clicked() {
			_ = env.Act.OpenExternal(value)
		}
	})
}

// contentWidth 是正文可排版的宽度：优先用上一帧量到的真实宽度。
func (p *Page) contentWidth(env *pluginapi.Env) float32 {
	if p.widthSeen > 0 {
		return p.widthSeen
	}
	width := env.MainWidth
	if width <= 0 {
		width = env.Bounds.Width
	}
	if width <= 0 {
		return pageFallbackWidth - 2*pagePadMinX
	}
	if width > pageMaxWidth {
		width = pageMaxWidth
	}
	inner := width - 2*pagePadX(env)
	if inner <= 0 {
		return pageFallbackWidth - 2*pagePadMinX
	}
	return inner
}

// pagePadX 复刻官方 `.page` 的左右内距 `clamp(24px, 4vw, 48px)`。
//
// ⚠ 本页**自带**这一层内距（它是官方 Plugins 页里的 tab 体，而 `.page` 的内距
// 是那一层的）。A 块把它挂进自己的页面容器时**不要**再套一层 `.page` 内距，
// 否则左右会双份。
func pagePadX(env *pluginapi.Env) float32 {
	width := env.Bounds.Width
	if width <= 0 {
		width = env.MainWidth
	}
	value := width * pagePadVw
	if value < pagePadMinX {
		return pagePadMinX
	}
	if value > pagePadMaxX {
		return pagePadMaxX
	}
	return value
}
