// plugins_page.go —— 官方 `dsh-client-ui-plugin-manager` 的**管理页**
// （OFF:`src/client/PluginManagerPage.tsx`；官方描述 "the **Plugins** entry of the
// sidebar and the management page it opens in the main column"）。
//
// # 数据 → 容器 → 渲染
//
//	pluginManager.listBundles（壳缓存，Actions.PluginBundles）
//	  → 本文件按官方结构投影成「标题行 / 已安装插件分组 / 卡片」
//	  → 详情页再读 pluginManager.readBundleDetail（Actions.PluginBundleDetail）
//
//	两条真实写动作：setBundleEnabled（卡片与详情的开关）、removeBundle（详情的「卸载」）
//
// # 文案与几何的出处
//
// 每一个字都来自官方 zh 字典（OFF:`src/client/locales.ts:9-...`，键名写在常量注释里），
// 每一个尺寸都来自官方 CSS（OFF:`src/client/PluginManagerPage.module.css`，行号标注在各处）。
// 图标位置沿用本仓既有的**文字字形**约定（`‹`/`▾`）：本仓没有官方 sprite 导出，
// 按仓规不许手绘冒充官方图标；卡片的 48px 图标框里画标题首字是 **derived**
// 兜底（官方是 `PluginArtworkDefault` 精灵图或 manifest 的 icon data URL，
// 本仓宿主两样都不产出），已在报告里登记。
//
// # 为什么不画「添加插件」与「更新」
//
// 官方页面的主入口是 AddPluginMenu → InstallDialog（npm/Git/本地目录 + 检查 +
// pnpm 日志 + 进度 + 取消，一整台状态机，OFF:`PluginManagerPage.tsx:1034-1535`）。
// 本仓宿主只有本地目录一种安装源，且 `installBundle` 对 JS/Cordis 形态是**结构化
// 失败**（HOST:`cmd/dshd/serve_plugins_official.go:232-254` 的 `officialUnsupportedInstall`）；
// 更新同理（registry/git 下载源不存在，`listBundles` 恒不产出 `update`）。
// **缺真实能力就 HIDE + 登记**，不画点了没反应或注定失败的按钮。
//
// # 与官方的已知差异（照实登记）
//
//	· 官方用 Modal 做卸载确认；本仓按仓规用「同一按钮再点一次 + 行内确认条」
//	  （与侧栏删除会话、任务页删除同一先例）。
//	· 官方「详细介绍」走 `DetailDocument` 的 Markdown 渲染（含代码块/图片）；
//	  本仓这一版按**纯文本**画真实 markdown 原文（内容不假，样式不同）。
//	· 官方列表有多 tab（`plugins.tab` 贡献方）与官方插件配置卡片（`plugins.item`）；
//	  本仓没有任何贡献方，按官方「没有贡献方就不画 tab 条」的规则整段不画。
package dssidebar

import (
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 官方 zh 文案（OFF:src/client/locales.ts 的 zh 段，逐字，勿改）────────────

const (
	zhPluginTitle      = "插件"                           // 'title'
	zhPluginIntro      = "安装、启用和配置插件"                   // 'intro'
	zhPluginLoading    = "正在读取插件…"                      // 'loading'
	zhPluginError      = "可能由于网络问题，无法读取全部插件"            // 'error'
	zhPluginUnavail    = "本部署没有可管理的 profile，无法安装或启停插件。" // 'unavailable'
	zhPluginRetry      = "重试"                           // 'retry'
	zhPluginEmpty      = "还没有安装任何插件。"                   // 'empty'
	zhPluginBundlesTtl = "已安装插件"                        // 'bundlesTitle'
	zhPluginMoreTitle  = "实验性插件"                        // 'moreTitle'（本仓无官方 optional bundle → 不画）
	zhPluginStatusProb = "异常"                           // 'statusProblem'
	zhPluginStatusOff  = "未运行"                          // 'statusNotRunning'
	zhPluginVersionTpl = "v{version}"                   // 'versionTag'
	zhPluginSourceTtl  = "来源信息"                         // 'sourceTitle'
	zhPluginSourceSpec = "代码来源"                         // 'sourceSpec'
	zhPluginSourceVer  = "当前版本"                         // 'sourceVersion'
	zhPluginSourceIn   = "内置"                           // 'sourceBuiltIn'
	zhPluginPartsLabel = "包含的组件"                        // 'partsLabel'
	zhPluginPartsEmpty = "这个插件包不包含任何组件。"                // 'partsEmpty'
	zhPluginBackToList = "返回插件"                         // 'backToList'
	zhPluginCrumbRoot  = "插件"                           // 'crumbRoot'
	zhPluginUninstall  = "卸载"                           // 'uninstall'
	zhPluginReasonLbl  = "原因"                           // 'reasonLabel'
	zhPluginDetailTtl  = "详细介绍"                         // 'detailTitle'
	zhPluginDetailLoad = "正在读取详细介绍"                     // 'detailLoading'
	zhPluginDetailFail = "无法读取详细介绍"                     // 'detailFailed'
	zhPluginCancel     = "取消"                           // 'cancel'
	zhPluginClose      = "关闭"                           // 'close'

	zhPluginConfirmTtl = "卸载「{name}」？"   // 'confirmUninstallTitle'
	zhPluginConfirmDes = "卸载后它提供的功能会消失。" // 'confirmUninstallDescription'
	zhPluginConfirmOk  = "卸载"            // 'confirmUninstall'

	// 动作失败文案（OFF 同文件 'failedEnable' / 'failedDisable' / 'failedUninstall'）。
	zhPluginFailEnable  = "启用失败：{reason}"
	zhPluginFailDisable = "停用失败：{reason}"
	zhPluginFailRemove  = "卸载失败：{reason}"

	// 管理性失败的兜底（OFF:`presentation.ts` 的 CODE_KEYS → 'reasonOperationError'）。
	zhPluginReasonOp = "Host 报告了一个错误"
)

// pluginReasonText 把官方 `ManagementError['code']` 翻成那一句官方中文
// （OFF:`presentation.ts:31-63` 的 CODE_KEYS 表 + `locales.ts` 的 reason* 段）。
//
// 未知 code 回官方兜底 `reasonOperationError`；`operation-error` 按官方规则优先
// 回宿主诊断原文（`presentation.ts:managementText`）。
func pluginReasonText(code, diagnostic string) string {
	switch code {
	case "management-required":
		return "插件管理所需，不能停用或卸载"
	case "unaddressable":
		return "当前 profile 的 patch 无法唯一定位这一项"
	case "unknown-plugin":
		return "找不到该插件"
	case "invalid-spec":
		return "请输入有效的包名或地址"
	case "already-installed":
		return "这个包已经安装；要安装其他版本，请先卸载它"
	case "ambiguous-install":
		return "无法从依赖变更中确定安装了哪一个包"
	case "not-bundle":
		return "这个包没有声明组合包，不能作为插件管理"
	case "not-removable":
		return "这个包不属于当前 profile，或者是插件管理所需的组件"
	case "stop-profile":
		return "这个 profile 没有启用 HMR，正在使用的包要停止后用 dsh plugin 卸载"
	case "bundle-in-use":
		return "其他配置仍在使用这个组合包的组件，请先停用它们"
	case "stale-approval":
		return "待允许的安装脚本列表已变化，请重新安装以刷新"
	case "incompatible-version":
		return "这个插件与当前 DSH 版本不兼容，运行它可能导致崩溃或数据丢失。"
	case "unsupported-platform":
		return "此插件不支持运行 DSH 的主机操作系统"
	case "unexpected-package":
		return "插件来源安装了与所请求不同的包或版本，已撤销这次安装。请向插件发布者或插件索引维护者报告此问题。"
	case "operation-error":
		if strings.TrimSpace(diagnostic) != "" {
			return diagnostic
		}
		return zhPluginReasonOp
	case "":
		return zhPluginReasonOp
	default:
		return zhPluginReasonOp
	}
}

// pluginStatusOf 是官方 `packageStatus`（OFF:`PluginManagerPage.tsx:434-438`）的逐行移植。
//
// 本仓宿主不产出 patch rows（`rows: []`），所以 `rows.every(...)` 对空数组为真：
// 一个"启用着但宿主报了错"的 bundle 官方判成 not-running（而不是 problem）。
func pluginStatusOf(b apiclient.PluginBundle) string {
	rowsWithoutEntry := true
	for _, row := range b.Rows {
		if row.EntryID != "" {
			rowsWithoutEntry = false
			break
		}
	}
	if b.Enabled && b.Error != nil && rowsWithoutEntry {
		return "not-running"
	}
	failedRow := false
	for _, row := range b.Rows {
		if row.Meta != nil && row.Meta.Error != "" {
			failedRow = true
			break
		}
	}
	if b.Error != nil || (b.Enabled && failedRow) {
		return "problem"
	}
	if b.Enabled {
		return "running"
	}
	return "disabled"
}

// pluginStatusTag 是官方 `STATUS_TAG_KEYS`：running/disabled 不带标签，
// not-running → 「未运行」，problem → 「异常」。
func pluginStatusTag(status string) string {
	switch status {
	case "not-running":
		return zhPluginStatusOff
	case "problem":
		return zhPluginStatusProb
	}
	return ""
}

// pluginVersionTag 是官方 `versionTag`（'v{version}'）；没有版本就整枚不画。
func pluginVersionTag(b apiclient.PluginBundle) string {
	if strings.TrimSpace(b.Version) == "" {
		return ""
	}
	return strings.ReplaceAll(zhPluginVersionTpl, "{version}", b.Version)
}

// pluginSortedBundles 按官方清单顺序（宿主给什么画什么）。
//
// 官方对 `mine` 分组不做排序（`listed.filter(pkg => !pkg.official)`）；这里保留
// 宿主顺序，只把 nil 归一成空切片便于测试。
func pluginSortedBundles(bundles []apiclient.PluginBundle) []apiclient.PluginBundle {
	out := make([]apiclient.PluginBundle, len(bundles))
	copy(out, bundles)
	return out
}

// ── 官方几何（OFF:PluginManagerPage.module.css）──────────────────────────────

const (
	pluginPageMaxWidth    = 960 // .page > * {max-width:960px}
	pluginPageTopInset    = 28  // --plugin-page-top-inset:28px（.pageHead padding-top）
	pluginTitleSize       = 20  // .pageTitle{font-size:20px;line-height:28px}
	pluginTitleLine       = 28
	pluginIntroGap        = 4 // .pageIntro{margin:4px 0 0}；13px/20px
	pluginIntroSize       = 13
	pluginIntroLine       = 20
	pluginBodyGap         = 32 // .page > .pageBody{gap:32px}；.detailSections{gap:32px}
	pluginGroupGap        = 8  // .group{gap:8px}
	pluginGroupTitleSize  = 14 // .groupTitle{font-size:14px;line-height:22px;font-weight:500}
	pluginGroupTitleLine  = 22
	pluginCardsGap        = 2  // .cards{gap:2px}
	pluginCardInset       = 8  // .cardHead{padding:8px}（--card-head-inset）
	pluginCardIconSize    = 48 // .cardIcon{width:48px;height:48px}（--card-icon-size）
	pluginCardHeadGap     = 14 // .cardHead{gap:14px}（--card-head-gap）
	pluginCardTitleSize   = 14 // .cardTitle{font-size:14px;line-height:20px;font-weight:500}
	pluginCardTitleLine   = 20
	pluginCardDescSize    = 13 // .cardDesc{font-size:13px;line-height:18px}
	pluginCardDescLine    = 18
	pluginCardMainGap     = 4  // .cardMain{gap:4px}
	pluginTitleRowGap     = 8  // .titleRow{gap:8px}
	pluginTagHeight       = 18 // .statusTag{height:18px;padding:0 7px;font-size:10px}
	pluginTagPadX         = 7
	pluginTagFont         = 10
	pluginStatusSize      = 13 // .status{font-size:13px;line-height:20px}
	pluginStatusLine      = 20
	pluginFailureGap      = 10 // .failure{gap:10px}
	pluginSectionGap      = 12 // .detailSection{gap:12px}
	pluginSectionTitleSze = 14 // .sectionTitle{font-size:14px;line-height:20px;font-weight:500}
	pluginSectionTitleLn  = 20
	pluginDetailMainTop   = 20 // .detailMain{margin-top:20px}
	pluginDetailMainGap   = 8  // .detailMain{gap:8px}
	pluginDetailDescSize  = 14 // .detailDesc{font-size:14px;line-height:22px}
	pluginDetailDescLine  = 22
	pluginDetailNameSize  = 12 // .detailName{font-size:12px;line-height:18px}
	pluginDetailNameLine  = 18
	pluginCrumbFontSize   = 12.5 // .crumb{font-size:12.5px}
	pluginCrumbGap        = 6    // .crumb{gap:6px}
	pluginFactMinHeight   = 44   // .facts div{min-height:44px;padding:8px 2px}
	pluginFactFontSize    = 13   // .facts dt/dd{font-size:13px;line-height:20px}
	pluginFactLine        = 20
	pluginSwitchWidth     = 36 // 官方 Switch 36×20（本仓 dssettings 的 ItemSwitch 同一几何）
	pluginSwitchHeight    = 20
	pluginSwitchKnob      = 16
	pluginSwitchPad       = 2
)

// ── 本地状态（Plugin 上的字段，见 sidebar.go）──────────────────────────────

// pluginBusy 报告一个 bundle 是否有一个"在飞"的开关请求。
//
// 判据是**收敛**而不是计时：点完记下想要的值，直到宿主给的 `enabled` 等于它
// 才清掉。这样既不会连点，也不会在请求失败后永远卡在禁用态
// （失败由 `PluginActionError` 带回来的 name 清掉）。
func (p *Plugin) pluginBusy(name string, current bool) bool {
	want, ok := p.pluginWant[name]
	if !ok {
		return false
	}
	if want == current {
		delete(p.pluginWant, name)
		return false
	}
	return true
}

// drawPluginManagerPage 是「插件」主面板：官方 PluginManagerPage 的落地。
//
// 入口是侧栏 panelRoutes 的 plugins 路由（官方 `sidebar.panellist` 的 id `plugins`、
// label `插件`、order 0）。`env.Act.PluginBundles == nil` = 老壳/无头装配：
// 按「宿主没给就整段不画」的规则只画标题与一句真实原因，不画死控件。
func (p *Plugin) drawPluginManagerPage(f ui.C, env *pluginapi.Env) {
	t := p.theme
	if env.Act.PluginBundles == nil {
		ui.Column(f).FillWidth().Gap(8).Children(func() {
			ui.Text(f, zhPluginTitle).FontSize(pluginTitleSize).LineHeight(pluginTitleLine).
				Weight(gfx.WeightMedium).Color(t.LabelPrimary)
			ui.Text(f, "当前宿主未提供插件管理面").FontSize(t.Fonts.Body).LineHeight(22).Color(t.LabelTertiary)
		})
		return
	}
	bundles, loaded, unavailable, errText := env.Act.PluginBundles()

	// 页面动作失败（启停/卸载）：官方用 toast，本仓用页面顶部一行提示，文案仍是
	// 官方那一句，不附宿主原文（原文由壳写状态栏）。停留 8 秒（与任务页同一先例）。
	if env.Act.PluginActionError != nil {
		if action, name, ok := env.Act.PluginActionError(); ok {
			delete(p.pluginWant, name)
			p.pluginNotice = map[string]string{
				"enable":    strings.ReplaceAll(zhPluginFailEnable, "{reason}", zhPluginReasonOp),
				"disable":   strings.ReplaceAll(zhPluginFailDisable, "{reason}", zhPluginReasonOp),
				"uninstall": strings.ReplaceAll(zhPluginFailRemove, "{reason}", zhPluginReasonOp),
			}[action]
			p.pluginNoticeUntil = time.Now().Add(8 * time.Second)
		}
	}
	if p.pluginNotice != "" && time.Now().After(p.pluginNoticeUntil) {
		p.pluginNotice = ""
	}

	// 选中的 bundle 从清单里消失（卸载/刷新后）→ 关闭详情（官方也是"离开清单
	// 就回到卡片"）。只有 loaded 时才判，避免"还没拉到"被当成"不存在"。
	if p.pluginSelected != "" && loaded {
		if _, ok := findPluginBundle(bundles, p.pluginSelected); !ok {
			p.closePluginDetail()
		}
	}

	ui.Column(f).FillWidth().FillHeight().Items(ui.Center).Children(func() {
		ui.Column(f).FillWidth().MaxWidth(pluginPageMaxWidth).FillHeight().Children(func() {
			if p.pluginSelected != "" {
				p.drawPluginDetail(f, env)
				return
			}
			p.drawPluginHeading(f)
			if p.pluginNotice != "" {
				ui.Text(f, p.pluginNotice).FontSize(t.Fonts.Secondary).LineHeight(pluginStatusLine).
					Color(t.Error).MarginSides(0, 0, 8, 0)
			}
			scroll := ui.Scroll(f).FillWidth().Grow(1).MinHeight(0)
			scroll.Children(func() {
				ui.Column(f).FillWidth().Gap(pluginGroupGap).Children(func() {
					p.drawPluginBody(f, env, bundles, loaded, unavailable, errText)
				})
			})
		})
	})
}

// drawPluginHeading 画官方 `.pageHead`：标题 + 简介。
//
// 「添加插件」按钮**不画**（见文件头：本仓没有 registry/git 安装源，install 是
// 结构化失败或 501）。官方那枚按钮的位置留在右侧空白，不拿别的动作冒充它。
func (p *Plugin) drawPluginHeading(f ui.C) {
	t := p.theme
	ui.Column(f).FillWidth().Gap(pluginIntroGap).
		MarginSides(pluginPageTopInset, 0, pluginBodyGap, 0).Children(func() {
		ui.Text(f, zhPluginTitle).FontSize(pluginTitleSize).LineHeight(pluginTitleLine).
			Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		ui.Text(f, zhPluginIntro).FontSize(pluginIntroSize).LineHeight(pluginIntroLine).
			Color(t.LabelSecondary)
	})
}

// drawPluginBody 画列表页主体：不可用 / 加载中 / 失败 / 空 / 分组卡片。
func (p *Plugin) drawPluginBody(f ui.C, env *pluginapi.Env, bundles []apiclient.PluginBundle, loaded, unavailable bool, errText string) {
	t := p.theme
	switch {
	case unavailable:
		// 官方 `unavailable`：本部署没有可管理的 profile。**不画重试**——
		// 重试一个不存在的方法永远同一个 501。
		ui.Text(f, zhPluginUnavail).FontSize(pluginStatusSize).LineHeight(pluginStatusLine).
			Color(t.LabelTertiary).FillWidth()
		return
	case !loaded && errText == "":
		ui.Text(f, zhPluginLoading).FontSize(pluginStatusSize).LineHeight(pluginStatusLine).
			Color(t.LabelTertiary).FillWidth()
		return
	case !loaded:
		p.drawPluginFailure(f, env, errText)
		return
	}

	list := pluginSortedBundles(bundles)
	if len(list) == 0 {
		ui.Text(f, zhPluginEmpty).FontSize(pluginStatusSize).LineHeight(pluginStatusLine).
			Color(t.LabelTertiary).FillWidth()
	}
	if len(list) > 0 {
		// 官方 `renderGroup('bundles', t('bundlesTitle'), mine.map(packageCard))`。
		// 本仓宿主 listBundles 只有 profile bundle（official=false），所以
		// 「内置插件配置」与「实验性插件」两组没有数据源，整组不画（官方也是
		// 空组即 null）。
		ui.Column(f).FillWidth().Gap(pluginGroupGap).Children(func() {
			ui.Text(f, zhPluginBundlesTtl).FontSize(pluginGroupTitleSize).
				LineHeight(pluginGroupTitleLine).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
			ui.Column(f).FillWidth().Gap(pluginCardsGap).Children(func() {
				for _, b := range list {
					b := b
					p.drawPluginCard(f, env, b)
				}
			})
		})
	}
	// 刷新失败但保住了旧卡片：在分组之后画官方失败行 + 重试（官方同一分支：
	// "A failed package read trails the groups it left incomplete"）。
	if errText != "" {
		p.drawPluginFailure(f, env, errText)
	}
}

// drawPluginFailure 画官方 `.failure`（错误行 + 重试）。
func (p *Plugin) drawPluginFailure(f ui.C, env *pluginapi.Env, errText string) {
	t := p.theme
	ui.Row(f).FillWidth().Items(ui.Center).Gap(pluginFailureGap).Children(func() {
		text := zhPluginError
		if errText != "" {
			text = text + "：" + errText
		}
		ui.Text(f, text).Grow(1).FontSize(pluginStatusSize).LineHeight(pluginStatusLine).Color(t.Error)
		if env.Act.PluginBundlesRefresh == nil {
			return
		}
		btn := ui.Row(f).Height(28).Radius(t.Radii.MD).PadSides(0, 10, 0, 10).
			Items(ui.Center).Justify(ui.Center).Background(t.BgLayer3).
			Hit(env.Tree.ID(env.Feat, "plugin-retry"), zhPluginRetry).Children(func() {
			ui.Text(f, zhPluginRetry).FontSize(13).LineHeight(20).Color(t.LabelPrimary)
		})
		if btn.Clicked() {
			env.Act.PluginBundlesRefresh()
		}
	})
}

// drawPluginCard 画官方 `.card`：图标框 + 标题/标签 + 简介 + 行尾开关。
func (p *Plugin) drawPluginCard(f ui.C, env *pluginapi.Env, b apiclient.PluginBundle) {
	t := p.theme
	status := pluginStatusOf(b)
	busy := p.pluginBusy(b.Name, b.Enabled)
	id := env.Tree.ID(env.Feat, "plugin-card-"+b.Name)

	ui.Column(f).FillWidth().Children(func() {
		ui.Row(f).FillWidth().Items(ui.Center).Gap(pluginCardHeadGap).
			PadSides(pluginCardInset, pluginCardInset, pluginCardInset, pluginCardInset).
			Radius(t.Radii.LG).Hover(ui.Look{}.Background(t.Hover)).Children(func() {
			p.drawPluginArtwork(f, t, b)
			// `.cardMain{gap:4px}`：标题行（标题 + 标签）+ 简介。
			ui.Column(f).Grow(1).MinWidth(0).Gap(pluginCardMainGap).Children(func() {
				ui.Row(f).FillWidth().Items(ui.Center).Gap(pluginTitleRowGap).Children(func() {
					open := ui.Text(f, b.Title()).FontSize(pluginCardTitleSize).
						LineHeight(pluginCardTitleLine).Weight(gfx.WeightMedium).
						Color(t.LabelPrimary).Ellipsis().Grow(1).
						Hit(id, b.Title())
					if open.Clicked() {
						p.openPluginDetail(b.Name)
					}
					if tag := pluginVersionTag(b); tag != "" {
						p.drawPluginTag(f, t, tag, false)
					}
					if tag := pluginStatusTag(status); tag != "" {
						p.drawPluginTag(f, t, tag, true)
					}
				})
				if desc := b.DescriptionText(); desc != "" {
					ui.Text(f, desc).FillWidth().FontSize(pluginCardDescSize).
						LineHeight(pluginCardDescLine).Color(t.LabelTertiary).Ellipsis()
				}
			})
			// `.cardEnd{gap:8px}`：在飞时占位进度点，然后开关。
			ui.Row(f).Items(ui.Center).Gap(8).Children(func() {
				if busy {
					ui.Row(f).Size(pluginSwitchPad*7, pluginSwitchPad*7).Radius(pluginSwitchPad * 3).
						Background(t.BrandPrimary)
				}
				if p.drawPluginSwitch(f, env, "plugin-toggle-"+b.Name, b.Enabled, b.Title(), busy) {
					p.togglePluginBundle(env, b)
				}
			})
		})
		// 管理性错误（官方 `.card > .reason`：左缘与标题对齐）。
		if b.Error != nil {
			ui.Text(f, zhPluginReasonLbl+"："+pluginReasonText(b.Error.Code, b.Error.Diagnostic)).
				FillWidth().FontSize(pluginStatusSize).LineHeight(pluginStatusLine).Color(t.Error).
				PadSides(0, pluginCardInset, pluginCardInset, pluginCardInset+pluginCardIconSize+pluginCardHeadGap)
		}
	})
}

// drawPluginArtwork 画官方 `.cardIcon`（48×48 的 0.5px 边框框）。
//
// 框内是 **derived** 兜底：官方是 `PluginArtworkDefault` 精灵图或 manifest 的
// icon（data URL）；本仓宿主两样都不产出，按仓规不手绘官方图标，用标题首字
// 占位并登记（与 dssettings 用 `▾`/`⌕` 文字字形同一条先例）。
func (p *Plugin) drawPluginArtwork(f ui.C, t *pluginapi.Theme, b apiclient.PluginBundle) {
	glyph := "?"
	if title := b.Title(); title != "" {
		glyph = string([]rune(title)[0])
	}
	ui.Row(f).Size(pluginCardIconSize, pluginCardIconSize).Shrink(0).
		Radius(t.Radii.LG).Border(0.5, t.BorderL3).
		Items(ui.Center).Justify(ui.Center).Children(func() {
		ui.Text(f, glyph).FontSize(18).LineHeight(24).Color(t.LabelSecondary)
	})
}

// drawPluginTag 画官方 `Tag`（18px 高、10px 字、胶囊）。
//
// danger=true 用错误色（官方 tone="danger" 的 statusTag）。
func (p *Plugin) drawPluginTag(f ui.C, t *pluginapi.Theme, label string, danger bool) {
	bg, fg := t.BgLayer3, t.LabelSecondary
	if danger {
		// 官方 danger tag 是 error-* 三件套底；本仓 Theme 只有纯 Error 色，
		// 所以底沿用 BgLayer3、字用 Error（登记为差异，不拿纯色降透明度凑）。
		fg = t.Error
	}
	ui.Row(f).Height(pluginTagHeight).Shrink(0).Radius(pluginTagHeight/2).
		PadSides(0, pluginTagPadX, 0, pluginTagPadX).Items(ui.Center).Justify(ui.Center).
		Background(bg).Children(func() {
		ui.Text(f, label).FontSize(pluginTagFont).LineHeight(pluginTagHeight).Color(fg)
	})
}

// drawPluginSwitch 画官方 Switch（36×20，旋钮 16×16；本仓 dssettings 的
// `ItemSwitch` 同一几何）。返回是否被点击。
func (p *Plugin) drawPluginSwitch(f ui.C, env *pluginapi.Env, key string, on bool, label string, disabled bool) bool {
	t := p.theme
	bg := t.BorderL3
	if on {
		bg = t.BrandPrimary
	}
	id := env.Tree.ID(env.Feat, key)
	row := ui.Row(f).Width(pluginSwitchWidth).Height(pluginSwitchHeight).Shrink(0).
		Radius(pluginSwitchHeight/2).Background(bg).
		PadSides(pluginSwitchPad, pluginSwitchPad, pluginSwitchPad, pluginSwitchPad).
		Justify(ui.End).Hit(id, "启用 "+label).Children(func() {
		ui.Row(f).Size(pluginSwitchKnob, pluginSwitchKnob).Radius(pluginSwitchKnob / 2).
			Background(t.BgLayer2)
	})
	if disabled {
		return false
	}
	return row.Clicked()
}

// openPluginDetail / closePluginDetail 是详情页的本地状态迁移。
func (p *Plugin) openPluginDetail(name string) {
	p.pluginSelected = name
	p.pluginRemoveArm = ""
}

// ClosePluginDetail 关闭详情页（测试与壳的无鼠标入口用）。
func (p *Plugin) ClosePluginDetail() { p.closePluginDetail() }

func (p *Plugin) closePluginDetail() {
	p.pluginSelected = ""
	p.pluginRemoveArm = ""
}

// findPluginBundle 在清单里找一条 bundle。
func findPluginBundle(bundles []apiclient.PluginBundle, name string) (apiclient.PluginBundle, bool) {
	for _, b := range bundles {
		if b.Name == name {
			return b, true
		}
	}
	return apiclient.PluginBundle{}, false
}

// drawPluginDetail 画官方详情页：面包屑 + 标题/版本/状态 + 名称 + 简介 +
// 动作（更新/卸载/开关）+ 三段（详细介绍 / 包含的组件 / 来源信息）。
func (p *Plugin) drawPluginDetail(f ui.C, env *pluginapi.Env) {
	t := p.theme
	bundles, _, _, _ := env.Act.PluginBundles()
	b, ok := findPluginBundle(bundles, p.pluginSelected)
	if !ok {
		// 清单里已经没有了：回列表（调用方每帧也会清理）。
		p.closePluginDetail()
		return
	}
	status := pluginStatusOf(b)
	busy := p.pluginBusy(b.Name, b.Enabled)

	scroll := ui.Scroll(f).FillWidth().Grow(1).MinHeight(0)
	scroll.Children(func() {
		ui.Column(f).FillWidth().Children(func() {
			// `.detailTop{padding-top:28px}` + `.crumb`（12.5px，chevron 旋转 90°）。
			crumb := ui.Row(f).Items(ui.Center).Gap(pluginCrumbGap).
				MarginSides(pluginPageTopInset, 0, 0, 0).
				Hit(env.Tree.ID(env.Feat, "plugin-back"), zhPluginBackToList).Children(func() {
				ui.Text(f, "‹").FontSize(14).LineHeight(18).Color(t.LabelTertiary)
				ui.Text(f, zhPluginCrumbRoot).FontSize(pluginCrumbFontSize).LineHeight(18).Color(t.LabelTertiary)
			})
			if crumb.Clicked() {
				p.closePluginDetail()
				return
			}
			// `.detailMain{margin-top:20px;gap:8px}`。
			ui.Column(f).FillWidth().Gap(pluginDetailMainGap).
				MarginSides(pluginDetailMainTop, 0, pluginBodyGap, 0).Children(func() {
				ui.Row(f).FillWidth().Items(ui.Center).Gap(pluginTitleRowGap).Children(func() {
					ui.Text(f, b.Title()).FontSize(pluginTitleSize).LineHeight(pluginTitleLine).
						Weight(gfx.WeightMedium).Color(t.LabelPrimary).Ellipsis()
					if tag := pluginVersionTag(b); tag != "" {
						p.drawPluginTag(f, t, tag, false)
					}
					if tag := pluginStatusTag(status); tag != "" {
						p.drawPluginTag(f, t, tag, true)
					}
				})
				ui.Text(f, b.Name).FontSize(pluginDetailNameSize).LineHeight(pluginDetailNameLine).
					Color(t.LabelTertiary)
				if desc := b.DescriptionText(); desc != "" {
					ui.Text(f, desc).FillWidth().FontSize(pluginDetailDescSize).
						LineHeight(pluginDetailDescLine).Color(t.LabelSecondary)
				}
				if b.Error != nil {
					ui.Text(f, zhPluginReasonLbl+"："+pluginReasonText(b.Error.Code, b.Error.Diagnostic)).
						FillWidth().FontSize(pluginStatusSize).LineHeight(pluginStatusLine).Color(t.Error)
				} else if b.ReadOnlyReason != "" {
					ui.Text(f, pluginReasonText(b.ReadOnlyReason, "")).
						FillWidth().FontSize(pluginStatusSize).LineHeight(pluginStatusLine).Color(t.LabelTertiary)
				}
				// 动作行：卸载（破坏性，二次点击确认）+ 开关。
				ui.Row(f).FillWidth().Items(ui.Center).Gap(pluginTitleRowGap).Children(func() {
					p.drawPluginUninstall(f, env, b, busy)
					ui.Spacer(f)
					if p.drawPluginSwitch(f, env, "plugin-detail-toggle-"+b.Name, b.Enabled, b.Title(), busy) {
						p.togglePluginBundle(env, b)
					}
				})
				if p.pluginRemoveArm == b.Name {
					// 官方确认文案（OFF locates 'confirmUninstallTitle'/…Description）。
					ui.Column(f).FillWidth().Gap(2).Children(func() {
						ui.Text(f, strings.ReplaceAll(zhPluginConfirmTtl, "{name}", b.Title())).
							FontSize(pluginDetailDescSize).LineHeight(pluginDetailDescLine).Color(t.LabelPrimary)
						ui.Text(f, zhPluginConfirmDes).FontSize(pluginStatusSize).
							LineHeight(pluginStatusLine).Color(t.LabelTertiary)
					})
				}
			})
			// `.detailSections{gap:32px}`：详细介绍 / 包含的组件 / 来源信息。
			ui.Column(f).FillWidth().Gap(pluginBodyGap).Children(func() {
				p.drawPluginDocument(f, env, b)
				p.drawPluginParts(f, t, b)
				p.drawPluginSource(f, t, b)
			})
		})
	})
}

// togglePluginBundle 记下期望值并发出真实启停请求（不乐观更新：
// `enabled` 仍画宿主值，直到壳重拉清单后收敛）。
func (p *Plugin) togglePluginBundle(env *pluginapi.Env, b apiclient.PluginBundle) {
	if env.Act.PluginSetBundleEnabled == nil {
		return
	}
	if p.pluginWant == nil {
		p.pluginWant = map[string]bool{}
	}
	want := !b.Enabled
	p.pluginWant[b.Name] = want
	env.Act.PluginSetBundleEnabled(b.Name, want)
}

// drawPluginUninstall 画官方「卸载」（danger 文字按钮）。
//
// 官方 disabled 判据：`busy || !removable || readOnlyReason !== undefined`。
// 本仓按仓规把官方 Modal 换成"同一按钮再点一次"（差异已登记）。
func (p *Plugin) drawPluginUninstall(f ui.C, env *pluginapi.Env, b apiclient.PluginBundle, busy bool) {
	t := p.theme
	disabled := busy || !b.Removable || b.ReadOnlyReason != ""
	label := zhPluginUninstall
	if p.pluginRemoveArm == b.Name {
		label = zhPluginConfirmOk
	}
	color := t.Error
	if disabled {
		color = t.LabelTertiary
	}
	hit := ui.Row(f).Height(28).Radius(t.Radii.MD).PadSides(0, 10, 0, 10).
		Items(ui.Center).Justify(ui.Center).
		Hit(env.Tree.ID(env.Feat, "plugin-uninstall"), zhPluginUninstall).Children(func() {
		ui.Text(f, label).FontSize(13).LineHeight(20).Color(color)
	})
	if disabled {
		return
	}
	if hit.Clicked() {
		if env.Act.PluginRemoveBundle == nil {
			return // 壳没接 → 不画死按钮（上层已经只画了文字，这里直接忽略点击）
		}
		if p.pluginRemoveArm != b.Name {
			// 第一次点只是"上膛"。
			p.pluginRemoveArm = b.Name
		} else {
			env.Act.PluginRemoveBundle(b.Name)
			p.pluginRemoveArm = ""
		}
	}
}

// drawPluginDocument 画官方「详细介绍」段（BundleDetailSection → DetailDocument）。
//
// 本仓按**纯文本**画宿主返回的真实 markdown 原文（差异已登记）；加载/失败态
// 用官方文案 + 重试。
func (p *Plugin) drawPluginDocument(f ui.C, env *pluginapi.Env, b apiclient.PluginBundle) {
	t := p.theme
	if env.Act.PluginBundleDetail == nil {
		return
	}
	detail, loaded, errText := env.Act.PluginBundleDetail(b.Name)
	ui.Column(f).FillWidth().Gap(pluginSectionGap).Children(func() {
		ui.Text(f, zhPluginDetailTtl).FontSize(pluginSectionTitleSze).
			LineHeight(pluginSectionTitleLn).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		switch {
		case errText != "":
			ui.Row(f).FillWidth().Items(ui.Center).Gap(pluginFailureGap).Children(func() {
				ui.Text(f, zhPluginDetailFail+"："+errText).Grow(1).
					FontSize(pluginStatusSize).LineHeight(pluginStatusLine).Color(t.Error)
				if env.Act.PluginBundleDetailRefresh != nil {
					btn := ui.Row(f).Height(28).Radius(t.Radii.MD).PadSides(0, 10, 0, 10).
						Items(ui.Center).Justify(ui.Center).Background(t.BgLayer3).
						Hit(env.Tree.ID(env.Feat, "plugin-detail-retry"), zhPluginRetry).Children(func() {
						ui.Text(f, zhPluginRetry).FontSize(13).LineHeight(20).Color(t.LabelPrimary)
					})
					if btn.Clicked() {
						env.Act.PluginBundleDetailRefresh(b.Name)
					}
				}
			})
		case !loaded:
			ui.Text(f, zhPluginDetailLoad).FontSize(pluginStatusSize).LineHeight(pluginStatusLine).
				Color(t.LabelTertiary)
		case detail.Found():
			// 逐行画真实 markdown 原文：空行保留一个行高的间距，标题/正文都不改写。
			for _, line := range strings.Split(strings.ReplaceAll(detail.Markdown, "\r\n", "\n"), "\n") {
				if strings.TrimSpace(line) == "" {
					ui.Spacer(f).Height(pluginStatusLine / 2)
					continue
				}
				ui.Wrap(f, line).FillWidth().FontSize(pluginStatusSize).
					LineHeight(pluginStatusLine).Color(t.LabelSecondary)
			}
		}
	})
}

// drawPluginParts 画官方「包含的组件」段（RowSection）。
//
// 本仓宿主 bundle 没有 patch rows（`rows: []`），官方此时画 `partsEmpty`；
// 这不是缺数据，是真实模型（HOST:`serve_plugins_official.go:150-152`）。
func (p *Plugin) drawPluginParts(f ui.C, t *pluginapi.Theme, b apiclient.PluginBundle) {
	if len(b.Rows) == 0 {
		ui.Column(f).FillWidth().Gap(pluginSectionGap).Children(func() {
			ui.Text(f, zhPluginPartsLabel).FontSize(pluginSectionTitleSze).
				LineHeight(pluginSectionTitleLn).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
			ui.Text(f, zhPluginPartsEmpty).FontSize(pluginStatusSize).
				LineHeight(pluginStatusLine).Color(t.LabelTertiary)
		})
		return
	}
	// 有真实 rows 时才逐行画（本仓当前到不了这一支；官方行形状见 OFF:R ElsSection）。
	ui.Column(f).FillWidth().Gap(pluginSectionGap).Children(func() {
		ui.Text(f, zhPluginPartsLabel).FontSize(pluginSectionTitleSze).
			LineHeight(pluginSectionTitleLn).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		for _, row := range pluginRowsSorted(b.Rows) {
			ui.Text(f, row.RowID+" · "+row.ModuleName).FontSize(pluginStatusSize).
				LineHeight(pluginStatusLine).Color(t.LabelSecondary)
		}
	})
}

// pluginRowsSorted 是 rows 的保序副本（官方按 bundle patch 声明顺序，**不排序**；
// 本仓保留宿主给的顺序，只把 nil 归一）。
func pluginRowsSorted(rows []apiclient.PluginBundleRow) []apiclient.PluginBundleRow {
	out := make([]apiclient.PluginBundleRow, len(rows))
	copy(out, rows)
	return out
}

// drawPluginSource 画官方「来源信息」段（SourceSection）。
func (p *Plugin) drawPluginSource(f ui.C, t *pluginapi.Theme, b apiclient.PluginBundle) {
	ui.Column(f).FillWidth().Gap(pluginSectionGap).Children(func() {
		ui.Text(f, zhPluginSourceTtl).FontSize(pluginSectionTitleSze).
			LineHeight(pluginSectionTitleLn).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		spec := b.Source
		if spec == "" {
			spec = zhPluginSourceIn
		}
		pluginFactRow(f, t, zhPluginSourceSpec, spec)
		if b.Version != "" {
			pluginFactRow(f, t, zhPluginSourceVer, b.Version)
		}
	})
}

// pluginFactRow 画官方 `.facts` 的一行（label 左 / 值右，底部 0.5px 细线）。
func pluginFactRow(f ui.C, t *pluginapi.Theme, label, value string) {
	ui.Row(f).FillWidth().MinHeight(pluginFactMinHeight).Items(ui.Center).
		Justify(ui.Between).Gap(16).PadSides(8, 2, 8, 2).
		Border(0, t.BorderL2).Children(func() {
		ui.Text(f, label).Shrink(0).FontSize(pluginFactFontSize).LineHeight(pluginFactLine).
			Color(t.LabelSecondary)
		ui.Text(f, value).Grow(1).FontSize(pluginFactFontSize).LineHeight(pluginFactLine).
			Color(t.LabelPrimary).Ellipsis()
	})
}
