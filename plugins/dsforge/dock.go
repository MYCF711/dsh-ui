// dock.go —— `conversation.input.dock` 上的**本地变更卡**（官方 `LocalChangesCard`）。
//
// # 座位说明（为什么不是官方那一格）
//
// 官方这个座位上是 `WorktreeProgress`（mount.ts:392-394），本地变更卡在
// **变更请求面板**（`ForgeTab.tsx:219`）里。而那张面板的正文是变更请求列表，
// 本仓 CR 内容整块 HIDE（G1：`changeRequestStates` 形状与数据源都不符）。
//
// 于是这张卡只剩两个去处：
//
//	① 塞进 ForgeTab —— 但 ForgeTab 一画就要先画"本会话还没有变更请求"那句
//	   （`changes.empty`），而这句话在本仓**是假的**：读不到 ≠ 没有。
//	② 放到这个**已存在、今天空着**的 dock 座位（`OffInputDock` 目前没有占用方）。
//
// 选 ②：卡片本体（组件结构、文案、几何）与官方逐条一致，只有**座位**不同。
// 这条差异登记在账本 #64 行，等 CR 线（#64b）解开后搬回 ForgeTab。
//
// # 官方形状
//
//	OFF:…/src/client/LocalChangesCard.tsx:40-83
//	OFF:…/src/client/ChangesTab.module.css:768-900
//	  .local{border:.5px l2;radius:md;bg:layer1}
//	  .localHead{gap:8px;padding:8px 12px 2px 6px}
//	  .localToggle{min-height:26px;padding:0 6px;gap:6px;color:label-primary;weight:500}
//	  .localRefs{margin-left:auto;12px code;color:label-tertiary}
//	  .localRow{gap:8px;padding:4px 10px 10px 12px;font-size:12px}
//	  .localView{min-height:28px;gap:8px;margin-left:-6px;padding:0 6px;color:label-secondary}
//	  .localFiles{padding:4px 6px 8px;border-top:.5px l2}
//	  .localFile{min-height:28px;padding:0 6px;gap:8px;font-size:12px}
package dsforge

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 LocalChangesCard 的中文文案（locales.ts:72-77，逐字）。
const (
	copyLocalTitle  = "本地变更"
	copyLocalRefs   = "%s → %s"
	copyLocalFiles  = "%s 个文件"
	copyLocalView   = "查看本会话的变更：%d 个文件，新增 %d 行，删除 %d 行"
	copyDiffHeadRef = "HEAD"
)

// 官方 `.local*` 的几何（ChangesTab.module.css:768-900）。
const (
	localBorder      float32 = 0.5 // 0.5px 描边
	localHeadPadTop  float32 = 8
	localHeadPadR    float32 = 12
	localHeadPadB    float32 = 2
	localHeadPadL    float32 = 6
	localHeadGap     float32 = 8
	localToggleH     float32 = 26
	localTogglePadX  float32 = 6
	localToggleGap   float32 = 6
	localRowPadTop   float32 = 4
	localRowPadR     float32 = 10
	localRowPadB     float32 = 10
	localRowPadL     float32 = 12
	localRowGap      float32 = 8
	localViewH       float32 = 28
	localViewGap     float32 = 8
	localViewPadX    float32 = 6
	localFilesPadTop float32 = 4
	localFilesPadX   float32 = 6
	localFilesPadB   float32 = 8
	localFileH       float32 = 28
	localFont        float32 = 12
	localLine        float32 = 18
	localTitleFont   float32 = 13
	localTitleLine   float32 = 20
)

// drawLocalCard 画本地变更卡。**没有数据就整块不画**（官方同样只在
// `local !== null` 时渲染）——画一张"0 个文件"的空卡等于断言"这个会话没有改动"。
func (p *Plugin) drawLocalCard(f ui.C, env *pluginapi.Env) {
	view := forgeView(env)
	if view == nil || !view.Local.Known {
		return
	}
	if env.Act.Forge == nil {
		// 没有择开视图的动作时，卡片本身就是个死链：官方 `view()` 打不开就不该有这张卡。
		return
	}
	t := p.themeOf(env)
	local := view.Local
	session := view.SessionID
	expanded := p.open.localExpanded(session)

	ui.Row(f).FillWidth().Justify(ui.Center).Children(func() {
		ui.Column(f).MaxWidth(localCardMaxWidth).Shrink(1).
			Border(localBorder, t.BorderL2).Radius(t.Radii.MD).
			Background(t.BgLayer1).Children(func() {
			// ── .localHead ──
			ui.Row(f).FillWidth().Gap(localHeadGap).Items(ui.Center).
				PadSides(localHeadPadTop, localHeadPadR, localHeadPadB, localHeadPadL).Children(func() {
				toggle := ui.Row(f).Height(localToggleH).Shrink(0).Gap(localToggleGap).
					PadSides(0, localTogglePadX, 0, localTogglePadX).Items(ui.Center).
					Radius(t.Radii.SM).Hover(ui.Look{}.Background(t.Hover)).
					Hit(env.Tree.ID(env.Feat, "forge-local-expand"), copyLocalTitle)
				toggle.Children(func() {
					ui.Text(f, copyLocalTitle).FontSize(localTitleFont).LineHeight(localTitleLine).
						Weight(gfx.WeightMedium).Color(t.LabelPrimary)
					if icon, ok := chevronDownIcon(); ok {
						ui.Icon(f, icon).Size(chevronSize, chevronSize).Color(t.LabelTertiary)
					}
				})
				if toggle.Clicked() {
					p.open.toggleLocal(session)
				}
				// `.localRefs`（margin-left:auto）：代码体 12px、第三级色。
				ui.Spacer(f).Grow(1)
				base := local.Base
				if base == "" {
					base = copyDiffHeadRef
				}
				ui.Text(f, sprintf(copyLocalRefs, copyDiffHeadRef, base)).
					FontSize(localFont).LineHeight(localLine).Color(t.LabelTertiary).
					Shrink(1).Ellipsis()
			})

			// ── .localRow ──
			ui.Row(f).FillWidth().Gap(localRowGap).Items(ui.Center).
				PadSides(localRowPadTop, localRowPadR, localRowPadB, localRowPadL).Children(func() {
				viewBox := ui.Row(f).Height(localViewH).Grow(1).Shrink(1).Gap(localViewGap).
					PadSides(0, localViewPadX, 0, localViewPadX).Items(ui.Center).
					Radius(t.Radii.SM).Hover(ui.Look{}.Background(t.Hover)).
					Hit(env.Tree.ID(env.Feat, "forge-local-view"), copyLocalTitle)
				viewBox.Children(func() {
					ui.Text(f, localCountsText(local)).FontSize(localFont).LineHeight(localLine).
						Color(t.LabelSecondary)
					ui.Text(f, sprintf(copyLocalFiles, itoa(local.Files))).FontSize(localFont).
						LineHeight(localLine).Color(t.LabelSecondary).Ellipsis()
				})
				if viewBox.Clicked() {
					// 官方 `view()`：打开变更视图（本地那一半），范围回到 branch。
					env.Act.SelectRightTab(changesTabKey)
					env.Act.Forge.LoadScope(apiclient.ForgeScopeBranch)
				}
			})

			// ── .localFiles（展开态）──
			if expanded {
				if diff := forgeDiffOf(env); diff != nil && diff.Scope == apiclient.ForgeScopeBranch && len(diff.Files) > 0 {
					ui.Column(f).FillWidth().
						PadSides(localFilesPadTop, localFilesPadX, localFilesPadB, localFilesPadX).
						Border(localBorder, t.BorderL2).Children(func() {
						for _, file := range diff.Files {
							p.drawLocalFileRow(f, env, file)
						}
					})
				}
			}
		})
	})
}

// drawLocalFileRow 画官方 `.localFile`：路径 + 该文件的增删数。
func (p *Plugin) drawLocalFileRow(f ui.C, env *pluginapi.Env, file apiclient.ForgeDiffFile) {
	t := p.themeOf(env)
	row := ui.Row(f).FillWidth().Height(localFileH).Gap(localRowGap).Items(ui.Center).
		PadSides(0, localFilesPadX, 0, localFilesPadX).Radius(t.Radii.SM).
		Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, "forge-local-file:"+file.Path), file.Path)
	row.Children(func() {
		ui.Text(f, file.Path).FontSize(localFont).LineHeight(localLine).
			Color(t.LabelPrimary).Grow(1).Ellipsis()
		ui.Text(f, fileCountsText(file)).FontSize(localFont).LineHeight(localLine).
			Color(t.LabelTertiary)
	})
	if row.Clicked() && env.Act.Forge != nil {
		// 官方 `view(path)`：打开变更视图并滚到该文件。本仓没有"滚到"的地址参数，
		// 只做到"打开视图 + 选中这个文件"（差异登记在账本行）。
		env.Act.SelectRightTab(changesTabKey)
		env.Act.Forge.LoadScope(apiclient.ForgeScopeBranch)
		p.open.selectFile(env.Data.Cur, file.Path)
	}
}

// localCardMaxWidth 是官方 `--dsh-chat-content-width` 的一档：卡片不许横穿整窗，
// 与 composer 同宽（官方那张卡在面板里，本仓放在 dock 上，取内容宽避免过宽）。
const localCardMaxWidth float32 = 720

// localCountsText 是官方 `<Counts>`（+a −d，代码体）。
func localCountsText(local pluginapi.ForgeLocalSummary) string {
	return "+" + itoa(local.Additions) + " −" + itoa(local.Deletions)
}

// fileCountsText 是单个文件的增删（官方 `<Counts file>`）。
func fileCountsText(file apiclient.ForgeDiffFile) string {
	return "+" + itoa(file.Additions) + " −" + itoa(file.Deletions)
}
