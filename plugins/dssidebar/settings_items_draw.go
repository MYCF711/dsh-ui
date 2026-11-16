// settings_items_draw.go —— 官方 4 条「插件设置页」的**绘制**（#39 / #45 / #46，本轮 3 条）。
//
// # 几何 / 文案的权威源（每一条都能核对）
//
// 页面结构（卡片 / 详情 / 分组）：
//
//	OFF:dsh-client-ui-plugin-manager/src/client/PluginManagerPage.tsx:584-595（ItemCard）
//	OFF:…/PluginManagerPage.tsx:635-666（ItemDetail：面包屑 + 标题 + 一句话 + 表单段）
//	OFF:…/PluginManagerPage.tsx:1664-1680（renderGroup：`.group` + `.groupTitle` + `.cards`）
//	OFF:…/PluginManagerPage.tsx:1855-1857（basic / extensions / bundles 三组的先后）
//	OFF:…/PluginManagerPage.module.css（.groupTitle 14/22、.cards gap 2、.cardHead gap 14
//	    padding 8、.detailTitle 20/28、.detailDesc 14/22、.detailSections gap 32）
//
// 共享表单框架 `SettingsForm` / `SettingsValueField`（primitives，**不是独立发布的包**）：
// 官方把 primitives 打进 web 前端 dist chunk，本仓从交付物里取原文：
//
//	CSS  OFF:dsh-web-frontend/dist/assets/index-CSAdOBx6.css
//	     `._form_ft7vz_3` / `._readOnly_ft7vz_8` / `._footer_ft7vz_16` / `._failed_ft7vz_23`
//	     / `._save_ft7vz_32`（表单框架）
//	     `._field_181ew_3` / `._head_181ew_14` / `._label_181ew_20` / `._labelGroup_181ew_29`
//	     / `._helpButton_181ew_41` / `._help_181ew_41` / `._badges_181ew_88` / `._reset_181ew_94`
//	     / `._input_181ew_115` / `._invalid_181ew_141` / `._hint_181ew_148`（字段）
//	     `._tag_brmue_4`（Tag，overridden 角标用 tonal=neutral）
//	JS   OFF:dsh-web-frontend/dist/assets/index-ghOpC9uV.js（SettingsForm 的 DOM 结构：
//	     readOnly 说明 → 子节点 → footer{ failed → save }；字段 = head{label+help} +
//	     badges{Tag+reset} + input + (invalid|hint) + help）
//
// section 标题与小节内间距（#46）：
//
//	OFF:dsh-client-ui-settings-subagent/src/client/SubagentCard.module.css:1-2
//	     `.section{padding:16px 0}` / `.heading{font-size:13px;font-weight:600;line-height:1.5}`
//	OFF:…/SubagentLimitsFields.module.css:1（`.limits` 在官方是两列网格；本仓用单列，
//	     见下面的差异登记）
//
// # 已登记的差异（缺真实能力/缺素材，不造假）
//
//	① **帮助折行按钮不画**：官方是 `<button class=helpButton>`（24×24，里面是
//	   `IconQuestionCircle`）。本机官方素材库里没有这枚图标（asar 的
//	   `dsh-web-frontend/dist` 只内联了图元，本仓 `plugins/dsassets` 没收录它），
//	   按仓规不手绘冒充。帮助**正文**照画（官方 `.help` 的版式），常显不折叠。
//	② **官方 depth 帮助里的 `<table>`** 压成两行文字（"0 禁用子智能体" /
//	   "1 仅允许主 Agent 创建子智能体"），内容逐字、版式不同。
//	③ **卡片图标**沿用本仓既有的文字字形兜底（官方 `itemArtwork(id)` 是精灵图；
//	   本仓宿主不产出，与 plugins_page.go 的 bundle 卡同一先例）。
//	④ **两列网格**（官方 `.limits{grid-template-columns:repeat(auto-fit,minmax(min(220px,100%),1fr))}`）
//	   本仓按单列画：当前页宽（960 上限）下两列会把 34px 输入框挤到 220px 以下，
//	   而 Han 字标签会截断；这是版式选择，已登记为差异而不是 aligned。
package dssidebar

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 几何常量（官方值逐行见 settingsItemGeometry 台账，测试逐行核对）─────

const (
	// 字段（primitives `._*_181ew_*`）。
	itemFieldPadY       float32 = 12   // .field{padding:12px 0}
	itemFieldGap        float32 = 6    // .field{gap:6px}
	itemFieldHeadGap    float32 = 8    // .head{gap:8px}
	itemFieldLabelSize  float32 = 13   // .label{font-size:13px}
	itemFieldLabelLine  float32 = 19.5 // .label{line-height:1.5}
	itemFieldBadgeGap   float32 = 8    // .badges{gap:8px}
	itemFieldResetSize  float32 = 12   // .reset{font-size:12px}
	itemFieldResetLine  float32 = 18   // .reset{line-height:1.5}
	itemFieldInputH     float32 = 34   // .input{height:34px}
	itemFieldInputPadX  float32 = 12   // .input{padding:0 12px}
	itemFieldInputFont  float32 = 13   // .input{font-size:13px}
	itemFieldInputLine  float32 = 19.5 // .input{line-height:1.5}
	itemFieldStrokeW    float32 = 0.5  // .input{border:.5px solid}
	itemFieldMsgSize    float32 = 12   // .hint / .invalid{font-size:12px}
	itemFieldMsgLine    float32 = 18   // .hint / .invalid{line-height:1.5}
	itemFieldHelpPadTop float32 = 10   // .help{padding:10px 0 0}
	itemFieldHelpSize   float32 = 12   // .help{font-size:12px}
	itemFieldHelpLine   float32 = 19.2 // .help{line-height:1.6}
	// Tag（overridden 角标）：`._tag_brmue_4{padding:1px 8px;font-size:11px;line-height:17px}`。
	itemTagPadX  float32 = 8
	itemTagPadY  float32 = 1
	itemTagFont  float32 = 11
	itemTagLine  float32 = 17
	itemTagPadYx float32 = 1 // 上下内缩（与 itemTagPadY 同值，命名分开读得清）

	// 表单框架（primitives `._*_ft7vz_*`）。
	itemFormReadOnlyGap float32 = 12 // .readOnly{margin:0 0 12px}
	itemFormFooterPadT  float32 = 16 // .footer{padding-top:16px}
	itemFormFooterGap   float32 = 8  // .footer{gap:8px}
	itemFormFailedSize  float32 = 12 // .failed{font-size:12px}
	itemFormFailedLine  float32 = 18 // .failed{line-height:1.5}
	itemFormSavePadY    float32 = 5  // .save{padding:5px 14px}
	itemFormSavePadX    float32 = 14
	itemFormSaveFont    float32 = 13 // .save{font-size:13px}
	itemFormSaveLine    float32 = 19.5

	// section（#46 `SubagentCard.module.css`）。
	itemSectionPadY    float32 = 16 // .section{padding:16px 0}
	itemSectionHeadSze float32 = 13 // .heading{font-size:13px}
	itemSectionHeadLn  float32 = 19.5

	// 详情页（PluginManagerPage.module.css 既有常量在 plugins_page.go 里，这里只补
	// `.detailSection{gap:12px}`——复用 pluginSectionGap）。
)

// itemGeoRow 是一行几何事实（与本仓 dssettings 的 GeoRow 同构，字段少几项：
// 这里没有 derived 公式/偏差智商，差异登记在文件头）。
type itemGeoRow struct {
	Node     string
	Prop     string
	Official float32
	Impl     float32
	Anchor   string
}

// settingsItemGeometry 是这一批的几何台账：每一行的 Official 与 Impl 必须相等。
//
// 锚点一律是**交付物原文**（CSS 规则名 + 选择器里的 class 后缀），不是截图估的。
func settingsItemGeometry() []itemGeoRow {
	const prim = "dsh-web-frontend/dist/assets/index-CSAdOBx6.css" // primitives 打包落点
	return []itemGeoRow{
		{"SettingsValueField.field", "padding-block", itemFieldPadY, itemFieldPadY, prim + " ._field_181ew_3"},
		{"SettingsValueField.field", "gap", itemFieldGap, itemFieldGap, prim + " ._field_181ew_3"},
		{"SettingsValueField.head", "gap", itemFieldHeadGap, itemFieldHeadGap, prim + " ._head_181ew_14"},
		{"SettingsValueField.label", "font-size", itemFieldLabelSize, itemFieldLabelSize, prim + " ._label_181ew_20"},
		{"SettingsValueField.badges", "gap", itemFieldBadgeGap, itemFieldBadgeGap, prim + " ._badges_181ew_88"},
		{"SettingsValueField.reset", "font-size", itemFieldResetSize, itemFieldResetSize, prim + " ._reset_181ew_94"},
		{"SettingsValueField.input", "height", itemFieldInputH, itemFieldInputH, prim + " ._input_181ew_115"},
		{"SettingsValueField.input", "padding-inline", itemFieldInputPadX, itemFieldInputPadX, prim + " ._input_181ew_115"},
		{"SettingsValueField.input", "font-size", itemFieldInputFont, itemFieldInputFont, prim + " ._input_181ew_115"},
		{"SettingsValueField.input", "border-width", itemFieldStrokeW, itemFieldStrokeW, prim + " ._input_181ew_115"},
		{"SettingsValueField.hint", "font-size", itemFieldMsgSize, itemFieldMsgSize, prim + " ._hint_181ew_148"},
		{"SettingsValueField.help", "padding-top", itemFieldHelpPadTop, itemFieldHelpPadTop, prim + " ._help_181ew_41"},
		{"SettingsValueField.help", "font-size", itemFieldHelpSize, itemFieldHelpSize, prim + " ._help_181ew_41"},
		{"SettingsValueField.help", "line-height", itemFieldHelpLine, itemFieldHelpLine, prim + " ._help_181ew_41"},
		{"Tag.overridden", "padding-inline", itemTagPadX, itemTagPadX, prim + " ._tag_brmue_4"},
		{"Tag.overridden", "font-size", itemTagFont, itemTagFont, prim + " ._tag_brmue_4"},
		{"Tag.overridden", "line-height", itemTagLine, itemTagLine, prim + " ._tag_brmue_4"},
		{"SettingsForm.footer", "padding-top", itemFormFooterPadT, itemFormFooterPadT, prim + " ._footer_ft7vz_16"},
		{"SettingsForm.footer", "gap", itemFormFooterGap, itemFormFooterGap, prim + " ._footer_ft7vz_16"},
		{"SettingsForm.failed", "font-size", itemFormFailedSize, itemFormFailedSize, prim + " ._failed_ft7vz_23"},
		{"SettingsForm.readOnly", "margin-bottom", itemFormReadOnlyGap, itemFormReadOnlyGap, prim + " ._readOnly_ft7vz_8"},
		{"SettingsForm.save", "padding-block", itemFormSavePadY, itemFormSavePadY, prim + " ._save_ft7vz_32"},
		{"SettingsForm.save", "padding-inline", itemFormSavePadX, itemFormSavePadX, prim + " ._save_ft7vz_32"},
		{"SettingsForm.save", "font-size", itemFormSaveFont, itemFormSaveFont, prim + " ._save_ft7vz_32"},
		{"SubagentCard.section", "padding-block", itemSectionPadY, itemSectionPadY, "ui-settings-subagent/src/client/SubagentCard.module.css .GZFzRG_section"},
		{"SubagentCard.heading", "font-size", itemSectionHeadSze, itemSectionHeadSze, "ui-settings-subagent/src/client/SubagentCard.module.css .GZFzRG_heading"},
	}
}

// ── 输入框登记 ─────────────────────────────────────────────────────

// settingsItemFields 是这几页表单的输入框。
//
// 为什么是固定槽位而不是按 Item 动态建：`Fields()` 是静态契约（壳只在装配时读一次）。
// 与任务页搜索框同一先例：多几个未挂载的 Field 不会抢焦点。
// mirror 记"上一次写进输入框的文本"，用来区分"用户改的"与"我们同步进去的"
// （见 syncField 的注释）——只有绘制线程读写它。
type settingsItemFields struct {
	agentLoopParallel ui.Field
	shellTimeout      ui.Field
	shellMaxOutput    ui.Field
	subagentDepth     ui.Field
	subagentActive    ui.Field
	// #47「网页搜索与获取」的四个输入框。
	webCount   ui.Field
	webBaseURL ui.Field
	webMaxUses ui.Field
	webKey     ui.Field
	// #63 目标语言输入框；provider 是官方 select，不占文本槽。
	cotTargetLanguage ui.Field
	mirror            map[string]string
}

// all 返回全部输入框（Fields() 用）。
func (f *settingsItemFields) all() []*ui.Field {
	// 密钥框是官方 `<input type="password">`（OFF:WebSearchCard.tsx:96-103 经
	// primitives `SettingsSecretField`）：viva 的等价物是 `Field.Secure`——显示圆点、
	// 不进无障碍树。**每帧设一次**（幂等），这样无论从哪条路径进到这一页都已经打开。
	f.webKey.Secure = true
	return []*ui.Field{
		&f.agentLoopParallel, &f.shellTimeout, &f.shellMaxOutput,
		&f.subagentDepth, &f.subagentActive,
		&f.webCount, &f.webBaseURL, &f.webMaxUses, &f.webKey,
		&f.cotTargetLanguage,
	}
}

// slot 按 (命名空间, 字段) 取输入框；没有就是 nil（那时这一格画静态文本）。
func (f *settingsItemFields) slot(ns, name string) *ui.Field {
	switch ns + "." + name {
	case settingsNSAgentLoop + "." + settingsFieldMaxParallelToolCalls:
		return &f.agentLoopParallel
	case settingsNSPwshSandbox + "." + settingsFieldShellTimeout, settingsNSBashSandbox + "." + settingsFieldShellTimeout:
		return &f.shellTimeout
	case settingsNSPwshSandbox + "." + settingsFieldShellMaxOutput, settingsNSBashSandbox + "." + settingsFieldShellMaxOutput:
		return &f.shellMaxOutput
	case settingsNSSubagent + "." + settingsFieldMaxDepth:
		return &f.subagentDepth
	case settingsNSSubagent + "." + settingsFieldMaxActiveSubagents:
		return &f.subagentActive
	case settingsNSWebAPI + "." + settingsFieldWebCount:
		return &f.webCount
	case settingsNSWebSearch + "." + settingsFieldLegacyBaseURL:
		return &f.webBaseURL
	case settingsNSWebSearch + "." + settingsFieldLegacyMaxUses:
		return &f.webMaxUses
	case settingsNSWeb + "." + settingsFieldLegacySecret:
		return &f.webKey
	case settingsNSCotTranslation + "." + settingsFieldCotTargetLanguage:
		return &f.cotTargetLanguage
	default:
		return nil
	}
}

// syncField 把输入框的文本与"该画的值"对齐，并报告这一帧用户是否改了它。
//
// 为什么需要 mirror 而不是每帧 SetText(显示值)：viva 的输入框是**活编辑器**，
// 每帧按显示值回写会把用户正在敲的字擦掉。mirror 记的是"我们上次写进去的文本"：
//
//	field.Text() != mirror  → 这一次是**别处**改的（用户敲的），交回去 stage；
//	field.Text() == mirror  → 没有别人的改动，若与应显示值不同就同步（describe 刷新、
//	                          打开页面、reset 三种情况）。
func (f *settingsItemFields) syncField(key string, field *ui.Field, text string) bool {
	if f.mirror == nil {
		f.mirror = map[string]string{}
	}
	if field == nil {
		return false
	}
	last, seen := f.mirror[key]
	current := field.Text()
	if seen && current != last {
		f.mirror[key] = current
		return true
	}
	if current != text {
		field.SetText(text)
		f.mirror[key] = text
		return false
	}
	f.mirror[key] = current
	return false
}

// forgetMirror 在打开/关闭一页时清掉镜像：新的一页从"没有历史"开始。
func (f *settingsItemFields) forgetMirror() { f.mirror = nil }

// ── 卡片 / 分组 ─────────────────────────────────────────────────────

// settingsItemCardsFor 是这一帧的卡片清单（壳没接 describe 时一张都没有）。
//
// `SettingsDescribe == nil` 是老壳/最小装配：那时**不画任何卡片**——官方 whileServed
// 在拿不到"服务了哪些命名空间"时同样不挂载任何一页，绝不按名字猜着画。
func (p *Plugin) settingsItemCardsFor(env *pluginapi.Env) []settingsItemCardState {
	if env == nil || env.Act.SettingsDescribe == nil {
		return nil
	}
	snapshot := p.itemFormSnapshot()
	// #47 那一页的两个额外数据面：provider 清单与凭据状态。
	// 两者都**同步返回壳缓存**；壳没接时对应字段保持 nil/false，页面自己决定
	// 不画菜单/不画密钥控件（不是画一个空清单）。
	if env.Act.WebProviderList != nil {
		snapshot.providerWired = true
		snapshot.providers = env.Act.WebProviderList()
	}
	if env.Act.CredentialStatus != nil {
		snapshot.credentialWired = true
		snapshot.credentials = env.Act.CredentialStatus([]string{
			settingsWebKeyRefFromDescribe(env.Act.SettingsDescribe()),
		})
	}
	return settingsItemCards(env.Act.SettingsDescribe(), snapshot)
}

// settingsWebKeyRefFromDescribe 取这一帧密钥控件指向的引用（`apiKeyEnv` 或默认）。
func settingsWebKeyRefFromDescribe(describe *apiclient.SettingsDescribeResult) string {
	view, ok := describe.Section(settingsNSWebSearch)
	return settingsWebKeyRef(view, ok)
}

// settingsItemCards 返回这一帧要画的卡片（官方 whileServed 未满足的**整张不出现**）。
func settingsItemCards(describe *apiclient.SettingsDescribeResult, fallback settingsItemFormSnapshot) []settingsItemCardState {
	if describe == nil {
		return nil
	}
	ledger := settingsItemLedger()
	out := make([]settingsItemCardState, 0, len(ledger))
	for _, item := range ledger {
		var card settingsItemCardState
		if item.Kind == settingsItemKindWebSearch {
			card = buildSettingsItemWebCard(item, describe, fallback)
		} else {
			card = buildSettingsItemCard(item, describe, fallback.drafts, fallback.write)
		}
		if !card.Available() {
			continue
		}
		out = append(out, card)
	}
	return out
}

// settingsItemFormSnapshot 是一帧的草稿快照（锁内拷贝，绘制期间不再持锁）。
type settingsItemFormSnapshot struct {
	drafts   map[string]settingsItemDraft
	baseline map[string]int
	write    settingsItemWrite
	// providers 是官方 `web.listProviders` 的壳缓存（nil = 还没拉到 / 壳没接）。
	providers *apiclient.WebProviderList
	// providerWired 表示壳接了这条能力；false 时页面不画"重试"，也不说"清单读失败"。
	providerWired bool
	// credentials 是凭据域按 ref 的答案（**缺键 = 不知道**）。
	credentials map[string]apiclient.CredentialInfo
	// credentialWired 表示壳接了凭据域；false 时密钥控件整个不画。
	credentialWired bool
}

// itemFormSnapshot 取这一帧的草稿快照。
func (p *Plugin) itemFormSnapshot() settingsItemFormSnapshot {
	form := p.ensureSettingsItemForm()
	drafts, baseline, write := form.snapshot()
	return settingsItemFormSnapshot{drafts: drafts, baseline: baseline, write: write}
}

// settingsItemCardByID 按 id 取这一帧的卡片状态（详情页用）。
//
// 取不到 = 宿主这一帧不再服务那条命名空间（或壳把自己的 describe 丢了）：
// 调用方必须退回列表，而不是画一页没有数据的表单。
func (p *Plugin) settingsItemCardByID(env *pluginapi.Env, id string) (settingsItemCardState, bool) {
	for _, card := range p.settingsItemCardsFor(env) {
		if card.Item.ID == id {
			return card, true
		}
	}
	return settingsItemCardState{}, false
}

// drawSettingsItemsGroup 画官方 `basic` 组：标题 + 卡片列。
//
// 位置在「已安装插件」之前（官方 1855-1857 的组序）。
func (p *Plugin) drawSettingsItemsGroup(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, cards []settingsItemCardState) {
	ui.Column(f).FillWidth().Gap(pluginGroupGap).Children(func() {
		ui.Text(f, settingsItemsGroupTitle).FontSize(pluginGroupTitleSize).
			LineHeight(pluginGroupTitleLine).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		ui.Column(f).FillWidth().Gap(pluginCardsGap).Children(func() {
			for _, card := range cards {
				card := card
				p.drawSettingsItemCard(f, env, t, card)
			}
		})
	})
}

// drawSettingsItemCard 画一张官方插件配置卡（`.card > .cardHead`）。
func (p *Plugin) drawSettingsItemCard(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card settingsItemCardState) {
	ui.Row(f).FillWidth().Items(ui.Center).Gap(pluginCardHeadGap).
		PadSides(pluginCardInset, pluginCardInset, pluginCardInset, pluginCardInset).
		Radius(t.Radii.LG).Hover(ui.Look{}.Background(t.Hover)).Children(func() {
		p.drawSettingsItemArtwork(f, t, card.Item.Title)
		ui.Column(f).Grow(1).MinWidth(0).Gap(pluginCardMainGap).Children(func() {
			open := ui.Text(f, card.Item.Title).FontSize(pluginCardTitleSize).
				LineHeight(pluginCardTitleLine).Weight(gfx.WeightMedium).
				Color(t.LabelPrimary).Ellipsis().Grow(1).
				Hit(env.Tree.ID(env.Feat, "settings-item-card-"+card.Item.ID), card.Item.Title)
			if open.Clicked() {
				p.openSettingsItem(card.Item.ID)
			}
			if summary := card.Item.Summary; summary != "" {
				ui.Text(f, summary).FillWidth().FontSize(pluginCardDescSize).
					LineHeight(pluginCardDescLine).Color(t.LabelTertiary).Ellipsis()
			}
		})
	})
}

// drawSettingsItemArtwork 画卡片图标框（48×48；框内是 derived 字形，见文件头 ③）。
func (p *Plugin) drawSettingsItemArtwork(f ui.C, t *pluginapi.Theme, title string) {
	glyph := "?"
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		glyph = string([]rune(trimmed)[0])
	}
	ui.Row(f).Size(pluginCardIconSize, pluginCardIconSize).Shrink(0).
		Radius(t.Radii.LG).Border(0.5, t.BorderL3).
		Items(ui.Center).Justify(ui.Center).Children(func() {
		ui.Text(f, glyph).FontSize(18).LineHeight(24).Color(t.LabelSecondary)
	})
}

// ── 详情页 ─────────────────────────────────────────────────────────

// openSettingsItem / closeSettingsItem 是卡片 ↔ 详情页的状态迁移。
//
// 打开/关闭都清草稿（官方每页一个 `SettingsFormModel`，离开页面即丢草稿）。
func (p *Plugin) openSettingsItem(id string) {
	p.itemSelected = id
	p.ensureSettingsItemForm().discard()
	p.itemFields.forgetMirror()
}

// CloseSettingsItemDetail 关闭官方插件配置详情页（测试与壳的无鼠标入口用）。
func (p *Plugin) CloseSettingsItemDetail() { p.closeSettingsItem() }

func (p *Plugin) closeSettingsItem() {
	if p.itemSelected == "" {
		return
	}
	p.itemSelected = ""
	p.ensureSettingsItemForm().discard()
	p.itemFields.forgetMirror()
}

// drawSettingsItemDetail 画 `ItemDetail`：面包屑 + 标题 + 一句话 + 表单段。
func (p *Plugin) drawSettingsItemDetail(f ui.C, env *pluginapi.Env, card settingsItemCardState) {
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	scroll := ui.Scroll(f).FillWidth().Grow(1).MinHeight(0)
	scroll.Children(func() {
		ui.Column(f).FillWidth().Children(func() {
			crumb := ui.Row(f).Items(ui.Center).Gap(pluginCrumbGap).
				MarginSides(pluginPageTopInset, 0, 0, 0).
				Hit(env.Tree.ID(env.Feat, "settings-item-back"), zhPluginBackToList).Children(func() {
				ui.Text(f, "‹").FontSize(14).LineHeight(18).Color(t.LabelTertiary)
				ui.Text(f, zhPluginCrumbRoot).FontSize(pluginCrumbFontSize).LineHeight(18).Color(t.LabelTertiary)
			})
			if crumb.Clicked() {
				p.closeSettingsItem()
				return
			}
			// `.detailMain{margin-top:20px;gap:8px}`：标题行 + 一句话。
			ui.Column(f).FillWidth().Gap(pluginDetailMainGap).
				MarginSides(pluginDetailMainTop, 0, pluginBodyGap, 0).Children(func() {
				ui.Text(f, card.Item.Title).FontSize(pluginTitleSize).LineHeight(pluginTitleLine).
					Weight(gfx.WeightMedium).Color(t.LabelPrimary)
				if card.Item.Summary != "" {
					ui.Text(f, card.Item.Summary).FillWidth().FontSize(pluginDetailDescSize).
						LineHeight(pluginDetailDescLine).Color(t.LabelSecondary)
				}
			})
			// `.detailSections{gap:32px}` → 一段 `.detailSection{gap:12px}` 装表单。
			ui.Column(f).FillWidth().Gap(pluginBodyGap).Children(func() {
				ui.Column(f).FillWidth().Gap(pluginSectionGap).Children(func() {
					p.drawSettingsItemForm(f, env, t, card)
				})
			})
		})
	})
}

// drawSettingsItemForm 画共享表单框架：readOnly 说明 → 各 section → footer(save)。
func (p *Plugin) drawSettingsItemForm(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card settingsItemCardState) {
	ui.Column(f).FillWidth().Children(func() {
		if !card.Writable {
			// `._readOnly_ft7vz_8{margin:0 0 12px;font-size:12px;line-height:1.5}`
			ui.Text(f, zhFormReadOnly).FillWidth().FontSize(itemFormFailedSize).
				LineHeight(itemFormFailedLine).Color(t.LabelTertiary).
				MarginSides(0, 0, itemFormReadOnlyGap, 0)
		}
		if card.Cot != nil {
			p.drawCotTranslationNotices(f, t, card)
		}
		if card.Web != nil {
			// #47 那一页：provider 菜单 + 灰面板，不是"section → 字段"形状。
			p.drawSettingsWebForm(f, env, t, card)
		} else {
			for index, section := range card.Sections {
				section := section
				if section.Title != "" {
					// `SubagentCard.module.css .section{padding:16px 0}` + `.heading`。
					ui.Column(f).FillWidth().PadSides(itemSectionPadY, 0, itemSectionPadY, 0).Children(func() {
						ui.Text(f, section.Title).FontSize(itemSectionHeadSze).
							LineHeight(itemSectionHeadLn).Weight(gfx.WeightSemibold).
							Color(t.LabelPrimary)
					})
				}
				for _, field := range section.Fields {
					field := field
					// `.field+.field{border-top:.5px solid …}`：同一个容器里除第一个之外都带顶线。
					p.drawSettingsItemField(f, env, t, card, section, field, index == 0 && fieldIndex(section, field) == 0)
				}
			}
		}
		p.drawSettingsItemFormFooter(f, env, t, card)
	})
}

// fieldIndex 返回字段在自己 section 里的序号（画 `.field+.field` 的顶线用）。
func fieldIndex(section settingsItemSectionState, target settingsItemFieldState) int {
	for index, field := range section.Fields {
		if field.Spec.Name == target.Spec.Name {
			return index
		}
	}
	return 0
}

// drawSettingsItemField 画一个 `SettingsValueField`。
func (p *Plugin) drawSettingsItemField(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, section settingsItemSectionState, field settingsItemFieldState, first bool) {
	ui.Column(f).FillWidth().Gap(itemFieldGap).
		PadSides(itemFieldPadY, 0, itemFieldPadY, 0).Children(func() {
		if !first {
			// `.field+.field{border-top:.5px solid var(--dsw-alias-border-l2)}`
			ui.Column(f).FillWidth().Children(func() {
				ui.Divider(f)
			})
		}
		// `.head{gap:8px}`：label(.labelGroup) + badges(已覆盖 + 恢复默认)。
		ui.Row(f).FillWidth().Items(ui.Center).Gap(itemFieldHeadGap).Children(func() {
			ui.Column(f).Grow(1).MinWidth(0).Children(func() {
				ui.Text(f, field.Spec.Label).FontSize(itemFieldLabelSize).
					LineHeight(itemFieldLabelLine).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
			})
			if field.Overridden {
				ui.Row(f).Items(ui.Center).Gap(itemFieldBadgeGap).Children(func() {
					p.drawSettingsItemOverriddenTag(f, t)
					if p.drawSettingsItemReset(f, env, t, card, section, field) {
						// reset 只是 stage（官方 `resetField` 也是 staged）：
						// 发请求要等 footer 的「保存」。
						p.ensureSettingsItemForm().resetField(section.NS, field.Spec.Name, field.Default)
						p.itemFields.syncField(section.NS+"."+field.Spec.Name,
							p.itemFields.slot(section.NS, field.Spec.Name), field.Default)
					}
				})
			}
		})
		p.drawSettingsItemInput(f, env, t, card, section, field)
		// (invalid | hint)：官方 `aria-describedby` 同时指这两处，画的是二选一。
		switch {
		case field.Invalid:
			ui.Text(f, field.Spec.Invalid).FillWidth().FontSize(itemFieldMsgSize).
				LineHeight(itemFieldMsgLine).Color(t.Error)
		case field.Spec.Hint != "":
			ui.Text(f, field.Spec.Hint).FillWidth().FontSize(itemFieldMsgSize).
				LineHeight(itemFieldMsgLine).Color(t.LabelTertiary)
		case field.Note != "":
			ui.Text(f, field.Note).FillWidth().FontSize(itemFieldMsgSize).
				LineHeight(itemFieldMsgLine).Color(t.LabelTertiary)
		}
		if len(field.Spec.HelpLines) > 0 {
			// `.help{padding:10px 0 0;font-size:12px;line-height:1.6}`
			ui.Column(f).FillWidth().PadSides(itemFieldHelpPadTop, 0, 0, 0).Children(func() {
				for _, line := range field.Spec.HelpLines {
					ui.Wrap(f, line).FillWidth().FontSize(itemFieldHelpSize).
						LineHeight(itemFieldHelpLine).Color(t.LabelSecondary)
				}
			})
		}
	})
}

// drawSettingsItemOverriddenTag 画 primitives 的 `Tag tone=neutral`（「已覆盖」）。
func (p *Plugin) drawSettingsItemOverriddenTag(f ui.C, t *pluginapi.Theme) {
	p.drawSettingsItemTag(f, t, zhFormOverridden, true)
}

// drawSettingsItemTag 画 primitives 的 `Tag`（`._tag_brmue_4`：11px / line-height 17 /
// padding-inline 8 / 圆角 = 高度一半）。
//
// `emphasis` 对应官方的 tone：neutral（当前项/已覆盖/已配置）用 layer-3 底 + 次要字，
// quiet（未配置）只留次要字与描边。官方这两种 tone 的取色原文在
// `dsh-web-frontend/dist/assets/index-CSAdOBx6.css` 的 `._tag_brmue_4[data-tone=...]`。
func (p *Plugin) drawSettingsItemTag(f ui.C, t *pluginapi.Theme, label string, emphasis bool) {
	height := itemTagLine + 2*itemTagPadY
	row := ui.Row(f).Height(height).Shrink(0).Radius(height/2).
		PadSides(itemTagPadYx, itemTagPadX, itemTagPadYx, itemTagPadX).
		Items(ui.Center).Justify(ui.Center)
	if emphasis {
		row = row.Background(t.BgLayer3)
	} else {
		row = row.Border(itemFieldStrokeW, t.BorderL3)
	}
	row.Children(func() {
		ui.Text(f, label).FontSize(itemTagFont).LineHeight(itemTagLine).Color(t.LabelSecondary)
	})
}

// drawSettingsItemReset 画 `.reset`（12px 次要色文字按钮）。返回是否被点。
func (p *Plugin) drawSettingsItemReset(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, section settingsItemSectionState, field settingsItemFieldState) bool {
	disabled := !section.Writable || card.Saving
	color := t.LabelSecondary
	if disabled {
		color = t.LabelTertiary
	}
	box := ui.Row(f).Height(itemFieldResetLine).Shrink(0).PadSides(0, 0, 0, 0).
		Items(ui.Center).Hit(env.Tree.ID(env.Feat, "settings-item-reset-"+field.Spec.DOMID), zhFormReset).
		Children(func() {
			ui.Text(f, zhFormReset).FontSize(itemFieldResetSize).
				LineHeight(itemFieldResetLine).Color(color)
		})
	if disabled {
		return false
	}
	return box.Clicked()
}

// drawSettingsItemInput 画 `.input`（34px 圆角框；有输入框槽位时是真编辑器）。
func (p *Plugin) drawSettingsItemInput(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, section settingsItemSectionState, field settingsItemFieldState) {
	if field.Spec.Kind == settingsItemFieldProvider {
		p.drawCotProviderInput(f, env, t, card, section, field)
		return
	}
	borderColor := t.BorderL4
	if field.Invalid {
		// `.input[aria-invalid=true]{border-color:state-error-primary}`
		borderColor = t.Error
	}
	key := section.NS + "." + field.Spec.Name
	slot := p.itemFields.slot(section.NS, field.Spec.Name)
	disabled := field.Disabled || card.Saving
	ui.Row(f).FillWidth().Height(itemFieldInputH).Radius(t.Radii.MD).
		Border(itemFieldStrokeW, borderColor).Background(t.BgLayer3).
		Items(ui.Center).PadSides(0, itemFieldInputPadX, 0, itemFieldInputPadX).Children(func() {
		if slot == nil {
			// 没有输入框槽位：画真实值（只读文本），不画一个点了没反应的输入框。
			ui.Text(f, field.Text).FontSize(itemFieldInputFont).LineHeight(itemFieldInputLine).
				Color(t.LabelPrimary).Ellipsis()
			return
		}
		// 输入框本体：`ui.TextFieldID` 只登记编辑器（焦点由壳按点到的框分配），
		// 它**不产生命中区**——所以这里不拿它的 Box 判点击（那会是一个"编译过、
		// 永远点不着"的接收者，cmd/app 的静态守卫就是这么抓的）。
		ui.TextFieldID(f, env.Tree.ID(env.Feat, field.Spec.DOMID), slot,
			field.Spec.Label, "").
			Grow(1).Height(itemFieldInputH).FontSize(itemFieldInputFont).
			Disabled(disabled)
		if disabled {
			return
		}
		if p.itemFields.syncField(key, slot, field.Text) {
			// 用户改了：stage 一份草稿（官方 `edit(name, text)` → `stage`）。
			p.ensureSettingsItemForm().stage(section.NS, field.Spec.Name,
				settingsItemDraft{Text: slot.Text()}, section.Revision)
		}
	})
}

// drawSettingsItemFormFooter 画 `._footer_ft7vz_16`：失败原因 + 保存按钮。
func (p *Plugin) drawSettingsItemFormFooter(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card settingsItemCardState) {
	ui.Row(f).FillWidth().Items(ui.Center).Gap(itemFormFooterGap).
		PadSides(itemFormFooterPadT, 0, 0, 0).Children(func() {
		if card.Failed {
			// `._failed_ft7vz_23{flex:1;font-size:12px;line-height:1.5;color:label-error}`
			ui.Text(f, zhFormSaveFailed).Grow(1).FontSize(itemFormFailedSize).
				LineHeight(itemFormFailedLine).Color(t.Error)
		} else {
			ui.Spacer(f)
		}
		save := p.drawSettingsItemSave(f, env, t, card)
		if save {
			p.startSettingsItemSave(env.Act, card)
		}
	})
}

// drawSettingsItemSave 画 `.save` 按钮（`background:label-primary;color:bg-layer-3`）。
func (p *Plugin) drawSettingsItemSave(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card settingsItemCardState) bool {
	label := zhFormSave
	if card.Saving {
		label = zhFormSaving
	}
	color := t.BgLayer3
	if !card.CanSave() {
		color = t.BgLayer3
	}
	box := ui.Row(f).Shrink(0).Radius(t.Radii.MD).Background(t.LabelPrimary).
		PadSides(itemFormSavePadY, itemFormSavePadX, itemFormSavePadY, itemFormSavePadX).
		Items(ui.Center).Justify(ui.Center).
		Hit(env.Tree.ID(env.Feat, "settings-item-save-"+card.Item.ID), label).Children(func() {
		ui.Text(f, label).FontSize(itemFormSaveFont).LineHeight(itemFormSaveLine).Color(color)
	})
	box = box.Disabled(!card.CanSave())
	if !card.CanSave() {
		return false
	}
	return box.Clicked()
}

// startSettingsItemSave 把保存交给 goroutine（绘制线程不能等 HTTP），并立刻置 saving。
func (p *Plugin) startSettingsItemSave(act pluginapi.Actions, card settingsItemCardState) {
	form := p.ensureSettingsItemForm()
	snapshot := p.itemFormSnapshot()
	form.setWrite(settingsItemWrite{Saving: true})
	if card.Web != nil {
		// #47：三段 mutate（api → legacy → selection）+ 可选的凭据写。
		go p.applySettingsItemWebSave(act, card.Web.plan, snapshot.baseline, form, snapshot.drafts)
	} else {
		go p.applySettingsItemSave(act, card.Plan, snapshot.baseline, form, snapshot.drafts)
	}
	if act.Invalidate != nil {
		act.Invalidate()
	}
}
