package dsjobs

// official_jobs.go —— 官方槽位上的后台任务入口（`conversation.session.header.actions`
// 的 `job-list` 触发器 + 500px 弹层）。
//
// 为什么单独一份：默认 `-shell auto` 在本仓会落到**旧三栏**（dsshell 占着
// `shell.root`），那时官方子槽根本不被 dispatch，能力只能挂在 `sidebar.footer`
// （见 jobs.go 顶部）。两个档位要各自有一个真入口，所以：
//
//	官方档  ← 本文件：header actions 触发器 + shell.overlay 弹层
//	旧三栏档 ← jobs.go：侧栏底部清单（本仓自有槽，derived）
//
// 几何逐值取 `ui-jobs/src/client/JobListAction.module.css`（行号见常量注释），
// 官方结构取 `JobListAction.tsx:527-640`：触发器 = 活动槽（无任务跑着时是 layers
// 图标、有跑着的时是 ongoing 状态点）+ `{count} 个后台任务`；弹层 = 活任务行
// 直接列，已结束段有折叠按钮 + 「清空」。
//
// 四件事本仓没有真值，**一律不画**（不是简化，是不编）：
//
//   - `kind` 角标（宿主 jobs.list 没有 kind）；
//   - `duration`（没有 startedAt/finishedAt，就只有 bytes）；
//   - `progress` / `stopping` / `killed`（要 jobs 控制流，宿主 unary 面没有）；
//   - 停止失败的行内红字（Actions.JobsStop 不回结果，失败由壳写状态栏）。

import (
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// SlotSessionHeaderActions 是官方 `conversation.session.header.actions`
// （`uikernel.OffSessionHeaderActions`；官方 ui-jobs 就注册在这里，id job-list、order 20）。
const SlotSessionHeaderActions = pluginapi.Slot("conversation.session.header.actions")

// 官方几何（`JobListAction.module.css`）。
const (
	jobsTriggerMinHeight  = 28 // .trigger{min-height:28px}
	jobsTriggerPadV       = 3  // .trigger{padding:3px 2px}
	jobsTriggerPadH       = 2
	jobsTriggerGap        = 4  // .trigger{gap:4px}
	jobsTriggerFont       = 12 // .trigger{font-size:12px;line-height:18px}
	jobsTriggerLine       = 18
	jobsActivitySlot      = 14                                 // .activitySlot{width:14px;height:14px}
	jobsMenuWidth         = 500                                // .menu{width:500px}
	jobsMenuMaxWidth      = 560                                // max-width:min(560px,100vw-32px)
	jobsMenuMaxHeight     = 480                                // max-height:min(480px,100dvh-140px)
	jobsMenuPad           = 4                                  // .menuBody{padding:4px}
	jobsMenuGap           = 1                                  // .menuBody{gap:1px}
	jobsMenuRadius        = 12                                 // MenuSurface 的 radius md
	jobsMenuTopOffset     = 5                                  // top: calc(anchor(bottom) + 5px)
	jobsMenuWindowInset   = 16                                 // max-width 的 100vw-32px = 两侧 16
	jobsMenuHeaderBase    = 10 + 30/2 + jobsTriggerMinHeight/2 // header padding-top 10 + titleRow 30 + 触发器 28
	jobsSectionMarginTop  = 4                                  // .sectionHeader{margin:4px 4px 1px}
	jobsSectionMarginSide = 4
	jobsSectionPadTop     = 4 // .sectionHeader{padding:4px 0 3px}
	jobsSectionPadBottom  = 3
	jobsSectionFont       = 11 // .sectionHeader{font-size:11px;line-height:16px}
	jobsSectionLine       = 16
	jobsToggleGap         = 3 // .sectionToggle/.sectionClear{gap:3px;padding:2px 4px}
	jobsTogglePadV        = 2
	jobsTogglePadH        = 4
	jobsRowLineGap        = 6 // .rowLine{gap:6px}
	jobsRowLivePadV       = 4 // .rowLineLive{padding:4px 8px 4px 2px}
	jobsRowLivePadH       = 8
	jobsRowLivePadL       = 2
	jobsRowMinHeight      = 28 // .row{min-height:28px;padding:4px 7px}
	jobsRowPadV           = 4
	jobsRowPadH           = 7
	jobsRowGap            = 6 // .row{gap:6px;font-size:12px;line-height:17px}
	jobsRowFont           = 12
	jobsRowLine           = 17
	jobsChevronBox        = 20  // .chevronBox{width:20px;height:20px}
	jobsChevronIcon       = 12  // IconChevronDownOutlineRegular size={12}
	jobsTerminalMaxH      = 288 // .panel --dsl-terminal-output-max-height:288px
	jobsTerminalGutter    = 12  // .panel [data-terminal]{--dsl-terminal-gutter:12px}
	jobsNoticeFont        = 11  // .notice{font-size:11px;line-height:16px}
	jobsNoticeLine        = 16
)

// 官方文案（zh 字典逐字；`ui-jobs/src/client/locales.ts`）。
const (
	zhJobsCount      = "%d 个后台任务" // count.idle.one / other 同模板
	zhSectionSettled = "已结束 %d"
	zhSectionClear   = "清空"
	zhKillFailed     = "停止失败"
)

// AttachShellTier 让壳把档位持有者交进来（与 dsagentteam/catalog.go 同一时序）。
//
// 官方子槽只在**官方骨架且没走兼容桥**时被 dispatch；旧三栏档由 jobs.go 的
// 侧栏清单兜住。判据留给每帧读，不在登记时写死。
func (p *Plugin) AttachShellTier(tier *uikernel.ShellTier) {
	p.mu.Lock()
	p.tier = tier
	p.mu.Unlock()
}

// officialSlotsLive 报告本帧官方子槽是否真的会被 dispatch。
func (p *Plugin) officialSlotsLive() bool {
	p.mu.Lock()
	tier := p.tier
	p.mu.Unlock()
	if tier == nil {
		// 没有档位持有者（单测 / 老壳）按官方读：官方是文档里的默认形态。
		return true
	}
	return tier.OfficialSlotsLive()
}

// jobRows 选出弹层里的两段：活任务（原序）与已结束（排除被「清空」的）。
func (p *Plugin) jobRows(env *pluginapi.Env) (liveRows, settledRows []pluginapi.Job) {
	if env.Act.JobsList == nil {
		return nil, nil
	}
	jobs := env.Act.JobsList()
	p.mu.Lock()
	cleared := make(map[string]bool, len(p.cleared))
	for key, ok := range p.cleared {
		cleared[key] = ok
	}
	p.mu.Unlock()
	for _, j := range jobs {
		if live(j) {
			liveRows = append(liveRows, j)
			continue
		}
		if cleared[jobKey(j)] {
			continue
		}
		settledRows = append(settledRows, j)
	}
	return liveRows, settledRows
}

// countLabel 是触发器与 aria 的计数文案（官方 `count.idle.*`）。
func countLabel(count int) string {
	// 官方用 i18n 模板 `{count} 个后台任务`，中英文 one/other 同形。
	return strings.Replace(zhJobsCount, "%d", itoa(count), 1)
}

// drawTrigger 画官方 header 触发器（官方 `JobListAction.tsx:527-570`）。
func (p *Plugin) drawTrigger(f ui.C, env *pluginapi.Env) {
	if !p.officialSlotsLive() {
		return
	}
	liveRows, settledRows := p.jobRows(env)
	visible := len(liveRows) + len(settledRows)
	p.noteOpenable(visible)
	if visible == 0 {
		// 官方：`if (visibleCount === 0) return null`。
		return
	}
	t := p.theme
	// 官方：有活任务时计数只数活任务；否则数全部可见行。
	count := visible
	if len(liveRows) > 0 {
		count = len(liveRows)
	}
	label := countLabel(count)
	id := env.Tree.ID(env.Feat, "jobs-trigger")
	row := ui.Row(f).MinHeight(jobsTriggerMinHeight).Items(ui.Center).
		Gap(jobsTriggerGap).
		PadSides(jobsTriggerPadV, jobsTriggerPadH, jobsTriggerPadV, jobsTriggerPadH).
		Radius(t.Radii.SM).
		Hit(id, label).Children(func() {
		ui.Row(f).Size(jobsActivitySlot, jobsActivitySlot).Items(ui.Center).
			Justify(ui.Center).Children(func() {
			if len(liveRows) > 0 {
				// 官方 `.textModeActivity`: 有活任务时活动槽画 ongoing 状态点。
				p.prims.Dot(f, t.Link, 6)
				return
			}
			ui.Icon(f, layersIcon()).Size(jobsActivitySlot-2, jobsActivitySlot-2).
				Color(t.LabelTertiary)
		})
		ui.Text(f, label).FontSize(jobsTriggerFont).LineHeight(jobsTriggerLine).
			Color(t.LabelTertiary)
	})
	if row.Clicked() {
		p.toggleOpen()
	}
}

// drawPanelBackdrop 是全窗透明命中区（官方菜单外 pointerdown 关闭的等价物）。
func (p *Plugin) drawPanelBackdrop(f ui.C, env *pluginapi.Env) {
	if !p.officialSlotsLive() {
		return
	}
	liveRows, settledRows := p.jobRows(env)
	if len(liveRows)+len(settledRows) == 0 {
		p.setOpen(false)
		return
	}
	if !p.isOpen() {
		return
	}
	id := env.Tree.ID(env.Feat, "jobs-backdrop")
	if ui.Row(f).Absolute().Inset(0).Hit(id, zhJobsTitle).Clicked() {
		p.setOpen(false)
	}
}

// drawPanel 画弹层本体（官方 MenuSurface + menuBody）。
func (p *Plugin) drawPanel(f ui.C, env *pluginapi.Env, _ func(pluginapi.Slot)) {
	if !p.officialSlotsLive() || !p.isOpen() {
		return
	}
	liveRows, settledRows := p.jobRows(env)
	if len(liveRows)+len(settledRows) == 0 {
		return
	}
	t := p.theme

	width := float32(jobsMenuWidth)
	if env.Bounds.Width > 0 && width > env.Bounds.Width-2*jobsMenuWindowInset {
		width = env.Bounds.Width - 2*jobsMenuWindowInset
	}
	if width < 0 {
		width = 0
	}
	// 官方 `top: max(12px, anchor(bottom)+5px)`：触发器的 bottom = header
	// padding-top 10 + titleRow 30/2 + 触发器 28/2 = 39，加 5。
	top := float32(jobsMenuHeaderBase + jobsMenuTopOffset)
	if env.Bounds.Height > 0 {
		if limit := env.Bounds.Height - 140; limit > 0 && float32(jobsMenuMaxHeight) > limit {
			// max-height: min(480px, 100dvh-140px)
		}
	}

	panel := ui.Column(f).Absolute().Top(top).Right(float32(jobsMenuWindowInset)).
		Width(width).MaxHeight(jobsMenuMaxHeight).Radius(jobsMenuRadius).
		Background(t.BgLayer1).Border(1, t.BorderL2).Shadow(gfx.ShadowMD).
		Pad(jobsMenuPad)
	// 面板本体吃掉点击：点菜单里面不关（只有菜单外 pointerdown 才关）。
	panel.Hit(env.Tree.ID(env.Feat, "jobs-menu"), zhJobsTitle)
	panel.Children(func() {
		ui.Scroll(f).FillWidth().Grow(1).MinHeight(0).Gap(jobsMenuGap).
			Children(func() {
				// 活任务段：官方直接列（没有段标题）。
				for _, j := range liveRows {
					p.drawOfficialItem(f, env, j, true)
				}
				// 已结束段：折叠按钮（chevron + `已结束 N`）+「清空」。
				if len(settledRows) > 0 {
					p.drawSettledHeader(f, env, len(settledRows), settledRows, len(liveRows) == 0)
					if p.settledExpanded(len(liveRows) == 0) {
						for _, j := range settledRows {
							p.drawOfficialItem(f, env, j, false)
						}
					}
				}
			})
	})
}

// drawSettledHeader 画 `.sectionHeader`（官方 `JobListAction.tsx:590-610`）。
func (p *Plugin) drawSettledHeader(f ui.C, env *pluginapi.Env, count int, settledRows []pluginapi.Job, defaultOpen bool) {
	t := p.theme
	open := p.settledExpanded(defaultOpen)
	ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.Between).
		MarginSides(jobsSectionMarginTop, jobsSectionMarginSide, 0, jobsSectionMarginSide).
		PadSides(jobsSectionPadTop, 0, jobsSectionPadBottom, 0).Children(func() {
		toggleID := env.Tree.ID(env.Feat, "jobs-settled-toggle")
		label := strings.Replace(zhSectionSettled, "%d", itoa(count), 1)
		toggle := ui.Row(f).Items(ui.Center).Gap(jobsToggleGap).
			PadSides(jobsTogglePadV, jobsTogglePadH, jobsTogglePadV, jobsTogglePadH).
			Radius(t.Radii.SM).Hit(toggleID, label).Children(func() {
			glyph := "▸"
			if open {
				glyph = "▾"
			}
			ui.Text(f, glyph).FontSize(jobsChevronIcon).Color(t.LabelTertiary)
			ui.Text(f, label).FontSize(jobsSectionFont).LineHeight(jobsSectionLine).
				Color(t.LabelTertiary)
		})
		if toggle.Clicked() {
			p.toggleSettled(open)
		}
		clearID := env.Tree.ID(env.Feat, "jobs-settled-clear")
		clear := ui.Row(f).Items(ui.Center).
			PadSides(jobsTogglePadV, jobsTogglePadH, jobsTogglePadV, jobsTogglePadH).
			Radius(t.Radii.SM).Hit(clearID, zhSectionClear).Children(func() {
			ui.Text(f, zhSectionClear).FontSize(jobsSectionFont).LineHeight(jobsSectionLine).
				Color(t.LabelTertiary)
		})
		if clear.Clicked() {
			p.clearSettled(settledRows)
		}
	})
}

// drawOfficialItem 画一行（官方 JobItem：rowLine + 可选输出面板）。
func (p *Plugin) drawOfficialItem(f ui.C, env *pluginapi.Env, j pluginapi.Job, isLive bool) {
	t := p.theme
	key := jobKey(j)
	id := env.Tree.ID(env.Feat, "jobs-row-"+key)
	clickable := env.Act.JobsOutput != nil && j.SessionID != ""

	line := ui.Row(f).FillWidth().Items(ui.Center).Gap(jobsRowLineGap)
	if isLive {
		// `.rowLineLive`：活任务是填充卡（把运行中的工作抬到已结束尾巴之上）。
		line = line.PadSides(jobsRowLivePadV, jobsRowLivePadH, jobsRowLivePadV, jobsRowLivePadL).
			Radius(t.Radii.MD).Background(t.BgLayer2)
	}
	line.Children(func() {
		row := ui.Row(f).Grow(1).MinHeight(jobsRowMinHeight).Items(ui.Center).
			Gap(jobsRowGap).PadSides(jobsRowPadV, jobsRowPadH, jobsRowPadV, jobsRowPadH).
			Radius(t.Radii.MD).Hit(id, shortCmd(j.Command)).Children(func() {
			p.prims.Dot(f, dotColor(t, j), jobDotSize)
			// `.label` 是等宽、单行省略；`.status` 是 10/16 tertiary。
			ui.Text(f, shortCmd(j.Command)).FontSize(jobsRowFont).LineHeight(jobsRowLine).
				Mono().Color(labelColor(t, isLive)).Grow(1).Ellipsis()
			ui.Text(f, statusLabel(j)).FontSize(10).LineHeight(16).
				Color(statusColor(t, j))
			if j.Done && j.Bytes > 0 {
				ui.Text(f, sizeLabel(j.Bytes)).FontSize(jobsNoticeFont).
					LineHeight(16).Color(t.LabelTertiary)
			}
		})
		if clickable {
			p.drawChevronBox(f, env, key)
		}
		if clickable && row.Clicked() {
			p.toggleExpanded(key)
		}
		if isLive && env.Act.JobsStop != nil && j.SessionID != "" {
			p.drawOfficialStop(f, env, j)
		}
	})
	if p.isExpanded(j) {
		p.drawOutputWithHeight(f, env, j, jobsTerminalMaxH)
	}
}

// labelColor 是行主标签的颜色（官方 `.rowSettled{color:label-tertiary}`）。
func labelColor(t *pluginapi.Theme, isLive bool) ui.Color {
	if isLive {
		return t.LabelPrimary
	}
	return t.LabelTertiary
}

// drawChevronBox 画 `.chevronBox`（20×20 描边方块 + 折叠箭头）。
//
// 官方是 `IconChevronDownOutlineRegular size={12}` 加 CSS `rotate(-90deg/0)`；
// viva 的 gfx.Icon 没有旋转变换，这里用同尺寸的三角形文字代替，
// **登记为 deviation**（方块尺寸/命中区/描边一致）。
func (p *Plugin) drawChevronBox(f ui.C, env *pluginapi.Env, key string) {
	t := p.theme
	glyph := "▸"
	if p.expandedKey() == key {
		glyph = "▾"
	}
	id := env.Tree.ID(env.Feat, "jobs-chevron-"+key)
	box := ui.Row(f).Size(jobsChevronBox, jobsChevronBox).Items(ui.Center).
		Justify(ui.Center).Radius(t.Radii.SM).Border(1, t.BorderL2).
		Background(t.BgLayer1).Hit(id, zhJobsTitle).Children(func() {
		ui.Text(f, glyph).FontSize(jobsChevronIcon).Color(t.LabelTertiary)
	})
	if box.Clicked() {
		p.toggleExpanded(key)
	}
}

// drawOfficialStop 画官方两段式停止钮（.stop → .stopArmed）。
func (p *Plugin) drawOfficialStop(f ui.C, env *pluginapi.Env, j pluginapi.Job) {
	t := p.theme
	key := jobKey(j)
	armed := p.armedFor(key)
	id := env.Tree.ID(env.Feat, "jobs-stop-"+key)
	border := t.BorderL2
	if armed {
		border = t.Error
	}
	label := zhStopLabel
	if armed {
		label = zhStopConfirm
	}
	box := ui.Row(f).MinHeight(jobsChevronBox).Items(ui.Center).Gap(jobsTriggerGap).
		Radius(t.Radii.SM).Border(1, border).Background(t.BgLayer1).
		PadSides(0, jobsRowPadH, 0, jobsRowPadH).
		Hit(id, zhStopTitle+" "+shortCmd(j.Command)).Children(func() {
		if armed {
			ui.Text(f, label).FontSize(jobStopFont).LineHeight(18).Color(t.Error)
			return
		}
		ui.Icon(f, stopIcon()).Size(10, 10).Color(stopColor(t, false))
	})
	if armed {
		box.Background(t.BgLayer2)
	}
	if box.Clicked() {
		if p.takeArmed(key) {
			env.Act.JobsStop(j.SessionID, j.ID)
			return
		}
		p.arm(key, env.Act.Invalidate)
	}
}

// HandleKey 扩展：ArrowDown 打开弹层、Escape 优先关弹层（官方同一手势）。
func (p *Plugin) handleOfficialKey(ev gfx.KeyEvent) (handled bool) {
	switch ev.Key {
	case gfx.KeyDown:
		if !p.openable() {
			return false
		}
		p.setOpen(true)
		return true
	case gfx.KeyEscape:
		if !p.isOpen() {
			return false
		}
		p.setOpen(false)
		return true
	}
	return false
}

// ── 弹层状态 ────────────────────────────────────────────────────────

func (p *Plugin) openable() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.openableCount > 0
}

func (p *Plugin) noteOpenable(count int) {
	p.mu.Lock()
	p.openableCount = count
	p.mu.Unlock()
}

func (p *Plugin) isOpen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.open
}

func (p *Plugin) setOpen(open bool) {
	p.mu.Lock()
	p.open = open
	p.mu.Unlock()
}

func (p *Plugin) toggleOpen() {
	p.mu.Lock()
	p.open = !p.open
	p.mu.Unlock()
}

func (p *Plugin) settledExpanded(defaultOpen bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.settledOpen != nil {
		return *p.settledOpen
	}
	return defaultOpen
}

func (p *Plugin) toggleSettled(current bool) {
	p.mu.Lock()
	next := !current
	p.settledOpen = &next
	p.mu.Unlock()
}

func (p *Plugin) clearSettled(rows []pluginapi.Job) {
	p.mu.Lock()
	if p.cleared == nil {
		p.cleared = map[string]bool{}
	}
	for _, j := range rows {
		p.cleared[jobKey(j)] = true
	}
	// 清空时把展开的已结束行收起（官方 `clearSettled` 同）。
	if p.expanded != "" {
		for _, j := range rows {
			if p.expanded == jobKey(j) {
				p.expanded = ""
			}
		}
	}
	p.mu.Unlock()
}

func (p *Plugin) expandedKey() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.expanded
}

// drawOutputWithHeight 是输出块的带高度版本（弹层用官方 288，侧栏用 220）。
func (p *Plugin) drawOutputWithHeight(f ui.C, env *pluginapi.Env, j pluginapi.Job, maxH float32) {
	p.drawOutputBlock(f, env, j, maxH, jobsTerminalGutter)
}

// nowForTest 让测试不必背时间（生产里没有调用者）。
var _ = time.Now
