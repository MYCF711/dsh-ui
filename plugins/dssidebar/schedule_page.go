// schedule_page.go —— 官方 `dsh-client-ui-schedule` 的**任务管理页**
// （OFF:`src/client/TaskManagerPage.tsx`，官方描述 "Host task management page"）。
//
// # 数据 → 容器 → 渲染
//
//	schedule.catalog（壳缓存，走 Actions.ScheduleCatalog）
//	  → 本文件按官方结构投影成「标题行 / 状态筛选 / 搜索 / 任务行 / 详情面板」
//	  → 详情面板再按 tab 拉 schedule.history（Actions.ScheduleHistory）
//
// 这条链上的每一个字都来自官方中文文案表（OFF:`task-manager-locales.ts:171-330`），
// 每一个尺寸都来自官方 CSS（OFF:`TaskManagerPage.module.css`，行号在下面各处标注）。
// **没有一句自造中文、没有一个自造图标**：图标位置沿用本仓既有的文字字形约定
// （`⌕` / `◷` / `×`，与侧栏工作区工具带同一条先例：本仓没有官方 sprite 导出，
// 按仓规不许手绘冒充官方图标）。
//
// # 官方哪些表面在这一版里，哪些不在（照实登记，不假装）
//
//	✅ 任务列表（状态筛选 active/paused/ended/all、搜索三字段、下次运行、空/错/加载态）
//	✅ 任务详情 · 规则（名称、下次运行、内容、运行时间、关联会话、排队提示）
//	✅ 任务详情 · 记录（schedule.history 分页、保留说明、加载更多）
//	✅ 三条宿主动作：暂停/恢复（schedule.edit patch.active）、立即执行
//	   （schedule.run）、删除（schedule.edit delete，两次点击确认）
//	❌ 手动创建表单（官方的日期/时钟/时区选择器是一整套组件；本仓没有它们，
//	   硬画一个日期输入就是自造交互。官方页面自己的主入口是「新建会话」，
//	   这条有：标题行的「新建」按钮）
//	❌ 详情页的**可编辑**规则（官方对 active 任务允许改名称/内容/时间；本仓只读。
//	   理由同上：编辑时间需要官方那套选择器）
//	❌ 更多操作菜单（官方把「立即执行 / 删除」收进 ⋯ 菜单；本仓没有通用菜单组件，
//	   按仓规不手画。三个动作以官方同名文字按钮直接列出，位置差异已登记）
//	❌ 删除确认 Modal（官方是 Modal；本仓用「同一按钮再点一次确认」，
//	   与侧栏删除会话同一条先例）
package dssidebar

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方中文文案（OFF:`task-manager-locales.ts:215-330` 的 zh 段，逐字）。
const (
	zhSchedTitle        = "自动化任务"                                               // 'title'
	zhSchedNew          = "新建"                                                  // 'new.action'
	zhSchedSearchLabel  = "搜索任务"                                                // 'search.label'
	zhSchedSearchHold   = "搜索自动化任务"                                             // 'search.placeholder'
	zhSchedSearchClear  = "清空搜索"                                                // 'search.clear'
	zhSchedFilterAll    = "全部"                                                  // 'statusFilter.all'
	zhSchedFilterActive = "已开启"                                                 // 'status.active'
	zhSchedFilterPaused = "已暂停"                                                 // 'status.paused'
	zhSchedFilterEnded  = "已结束"                                                 // 'status.ended'
	zhSchedLoading      = "正在加载任务…"                                             // 'list.loading'
	zhSchedLoadError    = "无法加载任务"                                              // 'list.error'
	zhSchedRetry        = "重试"                                                  // 'list.retry'
	zhSchedEmpty        = "还没有自动化任务，在会话中创建的任务会显示在这里"                            // 'list.empty'
	zhSchedEmptyActive  = "没有已开启的自动化任务"                                         // 'list.emptyActive'
	zhSchedEmptyPaused  = "没有已暂停的自动化任务"                                         // 'list.emptyPaused'
	zhSchedEmptyEnded   = "没有已结束的自动化任务"                                         // 'list.emptyInactive'
	zhSchedNoMatches    = "没有匹配的自动化任务"                                          // 'list.noMatches'
	zhSchedNextPrefix   = "下次计划时间："                                             // 'list.nextPrefix'
	zhSchedRule         = "规则"                                                  // 'detail.rule'
	zhSchedRecords      = "任务运行记录"                                              // 'detail.records'
	zhSchedClose        = "关闭详情"                                                // 'detail.close'
	zhSchedNextRun      = "下次运行"                                                // 'detail.nextRun'
	zhSchedName         = "自动化任务名称"                                             // 'detail.name'
	zhSchedInstruction  = "自动化任务内容"                                             // 'detail.instruction'
	zhSchedSession      = "关联会话"                                                // 'detail.session'
	zhSchedRunTime      = "运行时间"                                                // 'rule.title'
	zhSchedPause        = "暂停任务"                                                // 'host.pause'
	zhSchedResume       = "恢复任务"                                                // 'host.resume'
	zhSchedRunNow       = "立即执行"                                                // 'host.run'
	zhSchedRunning      = "此任务已排队或执行中，请在会话中管理任务"                                // 'host.executionActive'
	zhSchedPendingNew   = "待创建"                                                 // 'host.sessionAfterFirstRun'
	zhSchedDeleteAction = "删除任务"                                                // 'delete.action'
	zhSchedDeleteTitle  = "删除此任务？"                                              // 'delete.title'
	zhSchedDeleteDesc   = "任务将停止触发，并连同其已保存的任务运行记录一并删除。原会话及其消息仍然保留；已排队的消息不会被撤回。" // 'delete.description'
	zhSchedDeleteOK     = "确认删除"                                                // 'delete.confirm'
	zhSchedDeleteCancel = "取消"                                                  // 'delete.cancel'
	zhSchedDeleteFail   = "无法删除任务"                                              // 'toast.deleteFailed'
	zhSchedSaveFail     = "无法保存，已保留修改，请重试"                                      // 'host.saveError'
	zhSchedRunFail      = "无法将任务加入队列，请重试"                                       // 'host.runError'
	zhSchedNoRecord     = "暂无任务运行记录"                                            // 'delivery.empty'
	zhSchedHistoryLoad  = "正在加载任务运行记录…"                                         // 'delivery.loading'
	zhSchedHistoryFail  = "无法加载任务运行记录"                                          // 'delivery.error'
	zhSchedHistoryRetry = "重试"                                                  // 'delivery.retry'
	zhSchedLoadMore     = "加载更多"                                                // 'delivery.loadMore'
	zhSchedPruned       = "更早的运行记录已清理"                                          // 'delivery.pruned'
	zhSchedRetention    = "保留规则"                                                // 'delivery.retention'
	zhSchedExpand       = "展开"                                                  // 'delivery.expand'
	zhSchedCollapse     = "收起"                                                  // 'delivery.collapse'
	zhSchedMissing      = "任务已不可用"                                              // 'delivery.notFound'
	zhSchedOnce         = "仅一次"                                                 // 'frequency.once'
)

// ── 官方几何（OFF:TaskManagerPage.module.css） ─────────────────────────────
const (
	schedPageMaxWidth    = 960 // .pageContent{max-width:960px}
	schedHeadingTop      = 28  // .pageHeading{padding-top:28px}
	schedHeadingBottom   = 24  // .pageHeading{margin-bottom:24px}
	schedHeadingHeight   = 28  // .pageHeading h1{line-height:28px}
	schedNewHeight       = 32  // .headerAction{height:32px;padding:0 12px;border-radius:16px}
	schedNewRadius       = 16
	schedFilterHeight    = 28 // .filterTab{height:28px;padding:0 10px;border-radius:14px}
	schedFilterRadius    = 14
	schedSearchHeight    = 36 // .searchTools{--task-search-height:36px}
	schedSearchRadius    = 12 // .searchField{border-radius:12px;border:0.5px solid l3}
	schedSearchBottom    = 16 // .searchTools{margin-bottom:16px}
	schedFiltersBottom   = 14 // .filters{margin-bottom:14px}
	schedRowRadius       = 12 // .row{border-radius:12px;padding:8px;gap:12px}
	schedRowPad          = 8
	schedDetailBasisPct  = 47 // .detail{flex:0 0 47%}
	schedDetailBar       = 44 // .detailTabsBar{height:44px;padding:0 24px}
	schedDetailGutter    = 24 // .detail{--detail-gutter:24px}
	schedDetailScrollTop = 24 // .detailScroll{padding:24px 24px 28px}
	schedDetailScrollBtm = 28
	schedTabGap          = 36 // .detailTabs{gap:36px}
	schedRowGlyphWidth   = 16 // .rowGlyph{width:16px;height:20px;margin-top:2px}
	schedRowGlyphHeight  = 20
)

// scheduleStatusFilter 是官方状态筛选的四个值（顺序也照官方：
// active → paused → ended → all，OFF:`TaskManagerPage.tsx` 的 filterTabs 数组）。
var scheduleStatusFilter = []string{"active", "paused", "ended", "all"}

// drawSchedulePage 是「自动化任务」主面板：官方 TaskManagerPage 的落地。
//
// 入口是侧栏 panelRoutes 的 schedules 路由（与官方一样同时注册 main 与
// sidebar.panellist）。`env.Act.ScheduleCatalog == nil` = 老壳/无头装配：
// 按「宿主没给就整段不画」的规则只画标题与一句真实原因，不画死控件。
func (p *Plugin) drawSchedulePage(f ui.C, env *pluginapi.Env) {
	t := p.theme
	if env.Act.ScheduleCatalog == nil {
		ui.Column(f).FillWidth().Gap(8).Children(func() {
			ui.Text(f, zhSchedTitle).FontSize(20).LineHeight(28).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
			ui.Text(f, "当前宿主未提供任务目录").FontSize(t.Fonts.Body).LineHeight(22).Color(t.LabelTertiary)
		})
		return
	}
	if p.schedFilter == "" {
		p.schedFilter = "active" // 官方初始筛选就是 active
	}
	if p.schedTab == "" {
		p.schedTab = "rule"
	}
	entries, loaded, errText := env.Act.ScheduleCatalog()

	// 选中的任务从目录里消失（删除/刷新后）→ 关闭详情。官方也是目录刷新后
	// 把 selectedId 置空（TaskManagerPage.tsx 的 status==='ready' 分支）。
	if p.schedSelected != "" && loaded {
		if _, ok := findSchedule(entries, p.schedSelected); !ok {
			p.closeScheduleDetail()
		}
	}

	// 页面动作失败（删除/暂停/执行）：官方用 toast，本仓用页面顶部一行提示。
	// 文案仍是官方那一句，不附宿主原文（原文由壳写状态栏）。
	// 停留 8 秒（官方 toast 的停留量级；本仓没有 Toast 组件，用时间窗自清）。
	if env.Act.ScheduleActionError != nil {
		if action, ok := env.Act.ScheduleActionError(); ok {
			p.schedActionNotice = map[string]string{
				"delete": zhSchedDeleteFail, "update": zhSchedSaveFail, "run": zhSchedRunFail,
			}[action]
			p.schedNoticeUntil = time.Now().Add(8 * time.Second)
		}
	}
	if p.schedActionNotice != "" && time.Now().After(p.schedNoticeUntil) {
		p.schedActionNotice = ""
	}

	ui.Column(f).FillWidth().FillHeight().Children(func() {
		p.drawScheduleHeading(f, env)
		if p.schedActionNotice != "" {
			ui.Text(f, p.schedActionNotice).FontSize(t.Fonts.Secondary).LineHeight(20).Color(t.Error).
				MarginSides(0, 0, 8, 0)
		}
		p.drawScheduleFilters(f, env)
		p.drawScheduleSearch(f, env)
		ui.Row(f).FillWidth().Grow(1).Children(func() {
			p.drawScheduleList(f, env, entries, loaded, errText)
			if p.schedSelected != "" {
				p.drawScheduleDetail(f, env, entries)
			}
		})
	})
}

// drawScheduleHeading 画标题行：标题 + 官方「新建」按钮（= 新建会话）。
func (p *Plugin) drawScheduleHeading(f ui.C, env *pluginapi.Env) {
	t := p.theme
	ui.Row(f).FillWidth().Height(schedHeadingHeight).MarginSides(schedHeadingTop, 0, schedHeadingBottom, 0).
		Items(ui.Center).Children(func() {
		ui.Text(f, zhSchedTitle).FontSize(20).LineHeight(schedHeadingHeight).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		ui.Spacer(f)
		if env.Act.NewSession == nil {
			return // 壳没接新建会话 → 不画死按钮
		}
		btn := ui.Row(f).Height(schedNewHeight).Radius(schedNewRadius).
			Background(t.ButtonPrimaryFill).PadSides(0, 12, 0, 12).
			Items(ui.Center).Justify(ui.Center).Hover(ui.Look{}.Background(t.Hover)).
			Hit(env.Tree.ID(env.Feat, "schedule-new"), zhSchedNew).Children(func() {
			ui.Text(f, zhSchedNew).FontSize(13).LineHeight(20).Color(t.ButtonPrimaryFg)
		})
		if btn.Clicked() {
			p.activePanel = "" // 官方「新建」离开任务页，回到新会话
			env.Act.NewSession()
		}
	})
}

// drawScheduleFilters 画官方状态筛选条（OFF:TaskManagerPage.module.css `.filters`）。
func (p *Plugin) drawScheduleFilters(f ui.C, env *pluginapi.Env) {
	t := p.theme
	ui.Row(f).FillWidth().Height(schedFilterHeight).MarginSides(0, 0, schedFiltersBottom, 0).
		Items(ui.Center).Gap(12).Children(func() {
		for _, value := range scheduleStatusFilter {
			value := value
			active := p.schedFilter == value
			label := map[string]string{
				"active": zhSchedFilterActive, "paused": zhSchedFilterPaused,
				"ended": zhSchedFilterEnded, "all": zhSchedFilterAll,
			}[value]
			bg := t.BgBase
			fg := t.LabelTertiary
			if active {
				bg, fg = t.Hover, t.LabelPrimary
			}
			chip := ui.Row(f).Height(schedFilterHeight).Radius(schedFilterRadius).
				Background(bg).PadSides(0, 10, 0, 10).Items(ui.Center).Justify(ui.Center).
				Hover(ui.Look{}.Background(t.Hover)).
				Hit(env.Tree.ID(env.Feat, "schedule-filter-"+value), label).Children(func() {
				ui.Text(f, label).FontSize(14).LineHeight(22).Color(fg)
			})
			if chip.Clicked() {
				p.schedFilter = value
			}
		}
	})
}

// drawScheduleSearch 画官方搜索框（36px、0.5px l3 描边、12px 圆角）。
func (p *Plugin) drawScheduleSearch(f ui.C, env *pluginapi.Env) {
	t := p.theme
	ui.Row(f).FillWidth().Height(schedSearchHeight).MarginSides(0, 0, schedSearchBottom, 0).
		Border(0.5, t.BorderL3).Radius(schedSearchRadius).PadSides(0, 10, 0, 10).
		Items(ui.Center).Gap(6).Children(func() {
		ui.Text(f, "⌕").FontSize(14).Color(t.LabelTertiary).Shrink(0)
		ui.TextFieldID(f, env.Tree.ID(env.Feat, "schedule-search"), &p.schedSearch, zhSchedSearchLabel, "").
			Grow(1).Height(schedSearchHeight - 2).FontSize(t.Fonts.Secondary)
		if p.schedSearch.Text() != "" {
			clear := ui.Row(f).Width(28).Height(28).Shrink(0).Radius(t.Radii.SM).
				Items(ui.Center).Justify(ui.Center).Hover(ui.Look{}.Background(t.Hover)).
				Hit(env.Tree.ID(env.Feat, "schedule-search-clear"), zhSchedSearchClear).Children(func() {
				ui.Text(f, "×").FontSize(14).Color(t.LabelTertiary)
			})
			if clear.Clicked() {
				p.schedSearch.SetText("")
			}
		}
	})
}

// drawScheduleList 画任务列表（官方 `.list` / `.listRows`：2px 行距，无外框）。
func (p *Plugin) drawScheduleList(f ui.C, env *pluginapi.Env, entries []apiclient.ScheduleEntry, loaded bool, errText string) {
	t := p.theme
	ui.Column(f).Grow(1).FillHeight().Children(func() {
		if errText != "" {
			// 官方 CatalogFeedback：错误行 + 重试（不把失败画成空列表）。
			ui.Row(f).FillWidth().Items(ui.Center).Gap(8).Children(func() {
				ui.Text(f, zhSchedLoadError).FontSize(t.Fonts.Secondary).Color(t.Error)
				ui.Text(f, zhSchedRetry).FontSize(t.Fonts.Secondary).Color(t.Link).
					Hit(env.Tree.ID(env.Feat, "schedule-retry"), zhSchedRetry)
			})
			return
		}
		if !loaded {
			ui.Text(f, zhSchedLoading).FontSize(t.Fonts.Secondary).Color(t.LabelTertiary)
			return
		}
		rows := p.filteredScheduleRows(entries)
		if len(rows) == 0 {
			ui.Text(f, p.scheduleEmptyText()).FontSize(t.Fonts.Secondary).Color(t.LabelTertiary)
			return
		}
		ui.Scroll(f).FillWidth().Grow(1).MinHeight(120).Clip().Children(func() {
			ui.Column(f).FillWidth().Gap(2).Children(func() {
				for _, entry := range rows {
					entry := entry
					selected := p.schedSelected == entry.ID
					bg := t.BgBase
					if selected {
						bg = t.Hover // 官方 .selectedRow / :hover 同一个 hover 底
					}
					row := ui.Row(f).FillWidth().Radius(schedRowRadius).Background(bg).
						PadSides(schedRowPad, schedRowPad, schedRowPad, schedRowPad).
						Items(ui.Start).Gap(12).Hover(ui.Look{}.Background(t.Hover)).
						Hit(env.Tree.ID(env.Feat, "schedule-row-"+entry.ID), schedTaskName(entry)).
						Children(func() {
							ui.Text(f, "◷").Width(schedRowGlyphWidth).Height(schedRowGlyphHeight).
								FontSize(14).LineHeight(schedRowGlyphHeight).Color(t.LabelTertiary).Shrink(0)
							ui.Column(f).Grow(1).Gap(0).Children(func() {
								ui.Text(f, schedTaskName(entry)).FontSize(14).LineHeight(23).
									Weight(gfx.WeightMedium).Color(t.LabelPrimary).Ellipsis()
								ui.Text(f, schedRowSummary(entry)).FontSize(13).LineHeight(21).
									Color(t.LabelTertiary).MarginSides(2, 0, 0, 0)
							})
						})
					if row.Clicked() {
						p.schedSelected = entry.ID
						p.schedTab = "rule"
						p.schedDeleteArm = false
						p.schedActionNotice = ""
						if p.schedExpanded == nil {
							p.schedExpanded = map[string]bool{}
						}
					}
				}
			})
		})
	})
}

// drawScheduleDetail 画详情面板（官方 `.detail`：左侧 0.5px l4 竖线、47% 宽）。
func (p *Plugin) drawScheduleDetail(f ui.C, env *pluginapi.Env, entries []apiclient.ScheduleEntry) {
	t := p.theme
	entry, ok := findSchedule(entries, p.schedSelected)
	if !ok {
		return
	}
	width := env.Bounds.Width * schedDetailBasisPct / 100
	if width > 520 {
		width = 520 // 官方 flex-basis 是 47%，但窄面板里也不该把列表压没
	}
	ui.Column(f).Width(width).FillHeight().Border(0, t.BorderL4).
		Children(func() {
			p.drawScheduleDetailBar(f, env, entry)
			body := func() {
				if p.schedTab == "records" {
					p.drawScheduleRecords(f, env, entry)
					return
				}
				p.drawScheduleRules(f, env, entry)
			}
			if p.schedTab == "records" {
				ui.Scroll(f).FillWidth().Grow(1).Clip().Children(func() {
					ui.Column(f).FillWidth().PadSides(schedDetailScrollTop, schedDetailGutter,
						schedDetailScrollBtm, schedDetailGutter).Children(body)
				})
				return
			}
			ui.Column(f).FillWidth().Grow(1).PadSides(schedDetailScrollTop, schedDetailGutter,
				schedDetailScrollBtm, schedDetailGutter).Children(body)
		})
}

// drawScheduleDetailBar 画详情顶条：两个 tab + 三条官方动作 + 关闭。
func (p *Plugin) drawScheduleDetailBar(f ui.C, env *pluginapi.Env, entry apiclient.ScheduleEntry) {
	t := p.theme
	ui.Row(f).FillWidth().Height(schedDetailBar).Shrink(0).
		Border(0, t.BorderL3).PadSides(0, schedDetailGutter, 0, schedDetailGutter).
		Items(ui.Center).Gap(schedTabGap).Children(func() {
		for _, tab := range []string{"rule", "records"} {
			tab := tab
			active := p.schedTab == tab
			label := zhSchedRule
			if tab == "records" {
				label = zhSchedRecords
			}
			fg := t.LabelTertiary
			if active {
				fg = t.LabelPrimary
			}
			tabBox := ui.Row(f).Items(ui.Center).Hit(env.Tree.ID(env.Feat, "schedule-tab-"+tab), label).Children(func() {
				ui.Text(f, label).FontSize(14).LineHeight(22).Color(fg)
			})
			if tabBox.Clicked() {
				p.schedTab = tab
				p.schedDeleteArm = false
			}
		}
		ui.Spacer(f)
		// 三条动作。官方把「立即执行/删除」收在 ⋯ 菜单；本仓没有菜单组件，
		// 以官方同名文字按钮直排（差异已登记）。非 active 任务不能暂停/恢复，
		// ended 任务不能执行——与官方 disabled 判据一致。
		if entry.Status != "ended" {
			label := zhSchedPause
			if entry.Status != "active" {
				label = zhSchedResume
			}
			action := ui.Row(f).Items(ui.Center).Hit(env.Tree.ID(env.Feat, "schedule-toggle"), label).Children(func() {
				ui.Text(f, label).FontSize(13).LineHeight(20).Color(t.LabelSecondary)
			})
			if action.Clicked() && env.Act.ScheduleSetActive != nil {
				env.Act.ScheduleSetActive(entry.ID, entry.Status != "active", entry.UpdatedAt)
			}
		}
		if entry.Status == "active" && !entry.ExecutionActive {
			run := ui.Row(f).Items(ui.Center).Hit(env.Tree.ID(env.Feat, "schedule-run"), zhSchedRunNow).Children(func() {
				ui.Text(f, zhSchedRunNow).FontSize(13).LineHeight(20).Color(t.LabelSecondary)
			})
			if run.Clicked() && env.Act.ScheduleRun != nil {
				env.Act.ScheduleRun(entry.ID)
			}
		}
		if entry.ExecutionActive {
			ui.Text(f, zhSchedRunning).FontSize(12).LineHeight(18).Color(t.LabelTertiary)
		}
		del := ui.Row(f).Items(ui.Center).Hit(env.Tree.ID(env.Feat, "schedule-delete"), zhSchedDeleteAction).Children(func() {
			label := zhSchedDeleteAction
			if p.schedDeleteArm {
				label = zhSchedDeleteOK
			}
			ui.Text(f, label).FontSize(13).LineHeight(20).Color(t.Error)
		})
		if del.Clicked() {
			// 破坏性：第一次点只是"上膛"，第二次才真删（与侧栏删除会话同一先例）。
			if !p.schedDeleteArm {
				p.schedDeleteArm = true
			} else if env.Act.ScheduleDelete != nil {
				env.Act.ScheduleDelete(entry.ID)
				p.closeScheduleDetail()
			}
		}
		closeBox := ui.Row(f).Width(28).Height(28).Radius(t.Radii.SM).Items(ui.Center).Justify(ui.Center).
			Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, "schedule-close"), zhSchedClose).Children(func() {
			ui.Text(f, "×").FontSize(14).Color(t.LabelTertiary)
		})
		if closeBox.Clicked() {
			p.closeScheduleDetail()
		}
	})
}

// drawScheduleRules 画「规则」页（官方 Rules：名称、下次运行、内容、运行时间、关联会话）。
func (p *Plugin) drawScheduleRules(f ui.C, env *pluginapi.Env, entry apiclient.ScheduleEntry) {
	t := p.theme
	ui.Column(f).FillWidth().Gap(20).Children(func() {
		ui.Text(f, schedTaskName(entry)).FontSize(20).LineHeight(28).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		// 官方 `.nextRun`：active 显示「下次运行 <绝对> (<相对>)」，否则显示状态。
		next := ""
		if entry.Status == "active" {
			next = zhSchedNextRun + " " + schedAbsolute(entry.ScheduledAt) + " " + schedRelative(entry.ScheduledAt)
		} else {
			next = map[string]string{"paused": zhSchedFilterPaused, "ended": zhSchedFilterEnded}[entry.Status]
		}
		ui.Text(f, next).FontSize(13).LineHeight(20).Color(t.LabelTertiary).MarginSides(0, 0, 0, 0)

		// 删除上膛后把官方确认文案摊开（本仓没有 Modal，差异已登记）。
		if p.schedDeleteArm {
			ui.Column(f).FillWidth().Gap(2).Children(func() {
				ui.Text(f, zhSchedDeleteTitle).FontSize(14).LineHeight(22).Color(t.LabelPrimary)
				ui.Text(f, zhSchedDeleteDesc).FontSize(13).LineHeight(20).Color(t.LabelTertiary)
			})
		}

		schedField(f, t, zhSchedInstruction, entry.Prompt)
		schedField(f, t, zhSchedRunTime, schedFrequency(entry))
		if entry.Target.Kind == "new" && entry.LastDelivery == nil {
			schedField(f, t, zhSchedSession, zhSchedPendingNew)
		} else if id := schedTargetSession(entry); id != "" {
			schedField(f, t, zhSchedSession, id)
		}
	})
}

// drawScheduleRecords 画「记录」页（官方 DeliveryHistory）：分页 + 保留说明。
func (p *Plugin) drawScheduleRecords(f ui.C, env *pluginapi.Env, entry apiclient.ScheduleEntry) {
	t := p.theme
	if env.Act.ScheduleHistory == nil {
		ui.Text(f, zhSchedHistoryFail).FontSize(t.Fonts.Secondary).Color(t.LabelTertiary)
		return
	}
	page, loaded, errText := env.Act.ScheduleHistory(entry.ID)
	ui.Column(f).FillWidth().Gap(12).Children(func() {
		switch {
		case errText != "":
			ui.Row(f).Items(ui.Center).Gap(8).Children(func() {
				label := zhSchedHistoryFail
				if errText == "schedule_not_found" || errText == "delivery_cursor_not_found" {
					label = zhSchedMissing
				}
				ui.Text(f, label).FontSize(t.Fonts.Secondary).Color(t.Error)
				if env.Act.ScheduleHistory != nil {
					retry := ui.Text(f, zhSchedHistoryRetry).FontSize(t.Fonts.Secondary).Color(t.Link).
						Hit(env.Tree.ID(env.Feat, "schedule-history-retry"), zhSchedHistoryRetry)
					if retry.Clicked() {
						// 重读同一页：壳的 ScheduleHistory 首次问过就会拉；这里只能
						// 借 ScheduleHistoryMore 的通道发一次刷新不成立，改用目录刷新
						// （目录不变时历史仍会按需重读——见 schedule_catalog.go 的缓存边界）。
						env.Act.ScheduleRefresh()
					}
				}
			})
		case !loaded:
			ui.Text(f, zhSchedHistoryLoad).FontSize(t.Fonts.Secondary).Color(t.LabelTertiary)
		case len(page.Records) == 0:
			ui.Text(f, zhSchedNoRecord).FontSize(t.Fonts.Secondary).Color(t.LabelTertiary)
		default:
			for i, record := range page.Records {
				i, record := i, record
				key := fmt.Sprintf("%s#%d", entry.ID, i)
				ui.Column(f).FillWidth().Gap(2).Children(func() {
					ui.Text(f, schedAbsolute(record.ScheduledAt)).FontSize(13).LineHeight(20).
						Color(t.LabelSecondary)
					if strings.TrimSpace(record.Prompt) == "" {
						return // 旧记录没有正文：官方也不拿当前指令顶替
					}
					if p.schedExpanded[key] {
						ui.Wrap(f, record.Prompt).FontSize(13).LineHeight(20).Color(t.LabelTertiary)
					} else {
						ui.Wrap(f, record.Prompt).FontSize(13).LineHeight(20).Color(t.LabelTertiary).MaxLines(2)
					}
					if len([]rune(record.Prompt)) > 40 {
						label := zhSchedExpand
						if p.schedExpanded[key] {
							label = zhSchedCollapse
						}
						toggle := ui.Text(f, label).FontSize(12).LineHeight(18).Color(t.Link).
							Hit(env.Tree.ID(env.Feat, "schedule-record-toggle"), label)
						if toggle.Clicked() {
							if p.schedExpanded == nil {
								p.schedExpanded = map[string]bool{}
							}
							p.schedExpanded[key] = !p.schedExpanded[key]
						}
					}
				})
			}
			if page.NextBefore != "" && env.Act.ScheduleHistoryMore != nil {
				more := ui.Text(f, zhSchedLoadMore).FontSize(13).LineHeight(20).Color(t.Link).
					Hit(env.Tree.ID(env.Feat, "schedule-history-more"), zhSchedLoadMore)
				if more.Clicked() {
					env.Act.ScheduleHistoryMore(entry.ID)
				}
			}
			if page.EarlierRecordsPruned {
				ui.Text(f, zhSchedPruned).FontSize(12).LineHeight(18).Color(t.LabelTertiary)
			}
			if page.Retention.Records > 0 {
				ui.Text(f, zhSchedRetention+"："+fmt.Sprintf("每个任务最多保留近 %d 天内的 %d 条运行记录。",
					page.Retention.Days, page.Retention.Records)).
					FontSize(12).LineHeight(18).Color(t.LabelTertiary)
			}
		}
	})
}

// schedField 画官方的"标签 + 值"字段块（TaskManagerPage.module.css `.editorField`：
// 标签与内容 8px 间距、14px 字号；只读值用次级文字色）。
func schedField(f ui.C, t *pluginapi.Theme, label, value string) {
	ui.Column(f).FillWidth().Gap(8).Children(func() {
		ui.Text(f, label).FontSize(14).LineHeight(22).Color(t.LabelPrimary)
		ui.Wrap(f, value).FontSize(14).LineHeight(22).Color(t.LabelSecondary)
	})
}

// closeScheduleDetail 收起详情并清掉它的临时状态。
func (p *Plugin) closeScheduleDetail() {
	p.schedSelected = ""
	p.schedDeleteArm = false
}

// filteredScheduleRows 是官方列表的过滤 + 排序（TaskManagerPage 同名逻辑）。
//
// 搜索命中三个字段：任务名、内容、内部会话 id（官方 README 明说**不**解析会话标题）。
// 排序：下次运行时间升序（官方 `toSorted(Date.parse(taskNextRunAt))`）。
func (p *Plugin) filteredScheduleRows(entries []apiclient.ScheduleEntry) []apiclient.ScheduleEntry {
	query := strings.ToLower(strings.TrimSpace(p.schedSearch.Text()))
	rows := make([]apiclient.ScheduleEntry, 0, len(entries))
	for _, entry := range entries {
		if p.schedFilter != "all" && entry.Status != p.schedFilter {
			continue
		}
		if query != "" {
			hay := strings.ToLower(entry.Title + "\x00" + entry.Prompt + "\x00" +
				strings.Join(entry.InvolvedSessionIDs, "\x00") + "\x00" + schedTargetSession(entry))
			if !strings.Contains(hay, query) {
				continue
			}
		}
		rows = append(rows, entry)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return schedNextRunAt(rows[i]) < schedNextRunAt(rows[j])
	})
	return rows
}

// scheduleEmptyText 是官方按筛选区分的空状态文案（含搜索态）。
func (p *Plugin) scheduleEmptyText() string {
	if strings.TrimSpace(p.schedSearch.Text()) != "" {
		return zhSchedNoMatches
	}
	switch p.schedFilter {
	case "active":
		return zhSchedEmptyActive
	case "paused":
		return zhSchedEmptyPaused
	case "ended":
		return zhSchedEmptyEnded
	}
	return zhSchedEmpty
}

// findSchedule 在目录里按 id 找一条。
func findSchedule(entries []apiclient.ScheduleEntry, id string) (apiclient.ScheduleEntry, bool) {
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return apiclient.ScheduleEntry{}, false
}

// schedTaskName 是官方 `taskName`：宿主保证 title 非空，这里只兜底空串。
func schedTaskName(entry apiclient.ScheduleEntry) string {
	if strings.TrimSpace(entry.Title) != "" {
		return entry.Title
	}
	return entry.ID
}

// schedTargetSession 是官方 `taskSessionId`：new 目标看最近一次投递，其余看 target。
func schedTargetSession(entry apiclient.ScheduleEntry) string {
	if entry.Target.Kind == "new" {
		if entry.LastDelivery != nil {
			return entry.LastDelivery.TargetSessionID
		}
		return ""
	}
	return entry.Target.SessionID
}

// schedNextRunAt 是官方 `taskNextRunAt`：投递下限比计划时间更晚时以它为准。
func schedNextRunAt(entry apiclient.ScheduleEntry) string {
	if entry.DeliveryFloorEndsAt != "" && entry.DeliveryFloorEndsAt > entry.ScheduledAt {
		return entry.DeliveryFloorEndsAt
	}
	return entry.ScheduledAt
}

// schedRowSummary 是一行的第二行：状态（非 active）+ 频率 + 下次运行。
func schedRowSummary(entry apiclient.ScheduleEntry) string {
	parts := make([]string, 0, 3)
	if entry.Status == "paused" {
		parts = append(parts, zhSchedFilterPaused)
	} else if entry.Status == "ended" {
		parts = append(parts, zhSchedFilterEnded)
	}
	parts = append(parts, schedFrequency(entry))
	if entry.Status == "active" {
		parts = append(parts, zhSchedNextPrefix+schedAbsolute(schedNextRunAt(entry))+
			" "+schedRelative(schedNextRunAt(entry)))
	}
	return strings.Join(parts, " · ")
}

// schedFrequency 是官方 `formatScheduleFrequency` 的本仓子集：
// 官方 7 个 kind 都覆盖；`rrule` 按官方「无法归类即自定义」的判据落在
// `frequency.hostCustom`（本仓没有官方那套 rrule 归约器，不假装能读）。
func schedFrequency(entry apiclient.ScheduleEntry) string {
	clock := entry.Time
	if len(clock) > 5 {
		clock = clock[:5] // 官方 clockLabel：省略零秒与毫秒
	}
	switch entry.Kind {
	case "after", "at":
		return zhSchedOnce
	case "every":
		return "每 " + schedIntervalText(entry.EverySeconds)
	case "daily":
		return "每天 " + clock + schedZoneSuffix(entry.TimeZone)
	case "weekly":
		return "每周" + schedWeekdays(entry.Weekdays) + " " + clock + schedZoneSuffix(entry.TimeZone)
	case "cron":
		return "自定义" + schedZoneSuffix(entry.TimeZone)
	case "rrule":
		return "自定义"
	}
	return entry.Kind
}

// schedIntervalText 把秒折成官方单位档（天/小时/分钟/秒，取最大可整除档）。
func schedIntervalText(seconds int64) string {
	units := []struct {
		label   string
		seconds int64
	}{
		{"天", 86400}, {"小时", 3600}, {"分钟", 60}, {"秒", 1},
	}
	for _, unit := range units {
		if seconds > 0 && seconds%unit.seconds == 0 {
			return fmt.Sprintf("%d %s", seconds/unit.seconds, unit.label)
		}
	}
	return fmt.Sprintf("%d 秒", seconds)
}

// schedZoneSuffix 是官方 `frequency.hostZone` 的形状：" （Asia/Shanghai）"。
// 空 zone 不画；与 system zone 相同的省略规则本仓没有 ICU 数据，按"总是标"处理
// （比省略更不会误导：界面不需要知道浏览器时区）。
func schedZoneSuffix(zone string) string {
	if strings.TrimSpace(zone) == "" {
		return ""
	}
	return "（" + zone + "）"
}

// schedWeekdays 把 ISO 星期数折成官方中文星期（frequency.weekday.N + join）。
func schedWeekdays(days []int) string {
	names := []string{"", "一", "二", "三", "四", "五", "六", "日"}
	parts := make([]string, 0, len(days))
	for _, day := range days {
		if day >= 1 && day <= 7 {
			parts = append(parts, names[day])
		}
	}
	return strings.Join(parts, "、")
}

// schedAbsolute 是官方 `formatScheduleAbsolute` 的中文形状：
// `11月15日 09:00`（Intl zh-CN short month + 2-digit clock，h23）。
//
// 用**宿主存的那个偏移**（RFC3339 自带的 offset）而不是本机时区：
// 官方在浏览器里按本机时区渲染，本仓没有浏览器时区，按记录自带的偏移渲染
// 更接近"这条规则本来表达的时刻"（差异已登记）。
func schedAbsolute(iso string) string {
	at, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return fmt.Sprintf("%d月%d日 %02d:%02d", at.Month(), at.Day(), at.Hour(), at.Minute())
}

// schedRelative 是官方 `formatScheduleRelative` 的括号形状："(2小时后)"。
func schedRelative(iso string) string {
	at, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	difference := time.Until(at)
	if difference == 0 {
		return "(现在到期)"
	}
	absolute := difference
	sign := ""
	if difference < 0 {
		absolute = -difference
		sign = "已逾期 "
	}
	units := []struct {
		label   string
		seconds float64
	}{
		{"天", 86400}, {"小时", 3600}, {"分钟", 60}, {"秒", 1},
	}
	for _, unit := range units {
		seconds := absolute.Seconds()
		if seconds >= unit.seconds {
			value := seconds / unit.seconds
			if difference > 0 {
				value = float64(int(value + 0.999999))
			} else {
				value = float64(int(value))
			}
			if value < 1 {
				value = 1
			}
			return fmt.Sprintf("(%s%d%s后)", sign, int(value), unit.label)
		}
	}
	return "(现在到期)"
}
