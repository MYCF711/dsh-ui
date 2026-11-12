// official_stats_dialog.go —— 官方 **Activity 详情弹窗**（`StatsPills.tsx` 的 `DialogPill`）。
//
// # 官方结构（rc.2 `dsh-client-ui-chat/src/client/chat/`）
//
//	ActivityPill（StatsPills.tsx:249-296）
//	├─ mode === 'compact'        → PlainPill（只画速率，不可点）
//	├─ stats.steps === 0         → null（整枚不渲染）
//	├─ 四条计时一条都没有        → PlainPill（:288-289：没有行就不开空弹窗）
//	└─ 否则                      → DialogPill（:201-247）
//	   ├─ button.pill            aria-haspopup="dialog" aria-expanded={open}
//	   │                         悬停/展开时底 = interactive-bg-hover、
//	   │                         字 = label-secondary（StatsPills.module.css:42-52）
//	   └─ div[role=dialog]       aria-label={t('stats.dialog.title')}（stat-dialog.module.css:3-40）
//	      ├─ .title             flex/space-between/gap16/mb8/500（:44-51）
//	      │  ├─ .titleLabel     inline-flex/gap6/图标14（:64-75）
//	      │  └─ .titleValue     tabular-nums（:59-61）
//	      ├─ .titleRule         border-top .5px border-l2/mb10（:54-57）
//	      └─ dl.details         grid minmax(76,auto) minmax(0,1fr) gap 6/16（:77-95）
//	         ├─ dt 模型用时          dd formatDuration(llmMs)
//	         ├─ dt 工具调用用时      dd formatDuration(toolMs)
//	         ├─ dt 首 token 平均     dd formatDuration(ttftMs / ttftSteps)
//	         └─ dt 输出速度          dd {tokensPerSecond} tok/s
//
// 行为（stat-dialog.ts:21-55）：点胶囊开/关；**点弹窗外面关闭**；Esc 关闭；
// 同一 dock 上最多一个弹窗。
//
// # 本仓的四处实现映射（不是自造结构）
//
//	flex.Popup().Above(锚点, 8, Start)   ↔ `bottom: max(12px, calc(anchor(top) + 8px))`
//	                                       （viva 的 popup 在所有节点之后绘制 = z-index 1100；
//	                                       放不下时自动翻到下方 = 官方的 position-try-fallbacks）
//	全窗 Popup 命中层                     ↔ `useDismissOnOutsidePointer`（document 级 pointerdown）
//	HandleKey(Esc)                       ↔ stat-dialog.ts 的 keydown → Escape
//	胶囊的 `HoverBackground` + 字色切换    ↔ `button.pill:hover` / `[aria-expanded=true]`
//
// # 降级（不编 0、不出假入口）
//
//	SessionStats == nil / steps == 0 / 四个计时全 0   → 胶囊仍是**静态读数**（官方 PlainPill），
//	                                                  没有命中区、没有 hover、点不开
//	会话切换                                          → 弹窗关闭（官方 dock 是 session 作用域）
//
// **用量弹窗今天做不了**：官方 `UsagePill` 的弹窗读 `tokenUsage` 投影的四个桶
// （uncachedInput / cacheRead / cacheWrite / output，StatsPills.tsx:300-346），
// 本宿主没有这个投影（`cmd/dshd` 只在 `context.usage` 里给 usageBuckets，
// `internal/apiclient` 也没接那个方法）——所以用量胶囊按官方「没有行就是
// PlainPill」的规则保持静态，报告里登记为缺口，不用近似数据编一版。
package dsstatusbar

import (
	"math"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// 交互 id（`env.Tree.ID(env.Feat, …)` 的稳定名字；写死数字会让 Tree 无法去重）。
const (
	statsPillHitName    = "activity-pill"
	statsDialogPanelHit = "stats-dialog-panel"
	statsDialogBackdrop = "stats-dialog-backdrop"
)

// statsDialogState 是 Activity 弹窗的跨帧状态。
//
// pill / panel 两个 Box 与它们的矩形只在**同一帧内**有意义：`Box.Rect()` 要等
// 布局解算之后才有效（Draw 阶段读到的是上一帧的值），所以按
// `plugins/dsagentteam/model_slot.go` 的老办法——Draw 阶段记下盒子，
// **AfterDraw** 读矩形，下一帧用它当弹窗锚点。
type statsDialogState struct {
	open    bool
	session string // 展开时所在会话；切会话即关闭

	pill      flex.Box
	pillDrawn bool
	pillRect  ui.Rect

	panel      flex.Box
	panelDrawn bool
	panelRect  ui.Rect
}

// officialPillSpec 是一枚 dock 胶囊要画的东西。
//
// `interactive` 就是官方的**两种胶囊**：DialogPill（有弹窗行 → button + hover +
// 命中区）与 PlainPill（没有行 → 静态 span，既不可点也没有 hover）。
type officialPillSpec struct {
	// text 是完整读数（官方 `aria-label={text}`，也是胶囊上那一行字）。
	text string
	// interactive 报告这一枚有没有官方意义上的弹窗行。
	interactive bool
}

// officialPillSpecs 是 dock 上两枚胶囊的规格（活动 / 用量）。
//
// 判据逐条对齐官方：
//
//	活动   `len(statsDialogRows(sessionStats)) > 0` 才是 DialogPill；
//	       投影缺席 / steps=0 / 计时全 0 都退回静态读数（官方 :288-289）。
//	用量   官方 DialogPill 需要 `tokenUsage` 投影（本宿主没有，见文件头）⇒ 恒静态。
func officialPillSpecs(d pluginapi.Snapshot) []officialPillSpec {
	return []officialPillSpec{
		{text: activitySegment(d), interactive: len(statsDialogRows(d.SessionStats)) > 0},
		{text: orUnknown(usageTokenSegment(d), zhUsageUnknown), interactive: false},
	}
}

// themeOf 取这一帧的主题（Env 优先，退回插件注册时注入的那份）。
func (p *Plugin) themeOf(env *pluginapi.Env) *pluginapi.Theme {
	if env != nil && env.Theme != nil {
		return env.Theme
	}
	return p.theme
}

// statsTreeID 造一个交互 id；没有 Tree（纯几何单测）时返回零值 = 不接交互。
func statsTreeID(env *pluginapi.Env, name string) ui.ID {
	if env == nil || env.Tree == nil {
		return ui.ID{}
	}
	return env.Tree.ID(env.Feat, name)
}

// statsHovered 报告某个交互 id 这一帧是否处于"点亮"状态。
//
// 官方 `button.pill:hover` 与 `[aria-expanded=true]` 用的是同一套视觉（底 =
// interactive-bg-hover、字 = label-secondary），所以展开态与悬停/按下/聚焦取并集。
func statsHovered(env *pluginapi.Env, id ui.ID, open bool) bool {
	if open {
		return true
	}
	if env == nil || env.Canvas == nil || id.IsZero() {
		return false
	}
	return env.Canvas.Hovered() == id || env.Canvas.Pressed() == id || env.Canvas.Focused() == id
}

// drawStatsDialog 画 Activity 弹窗（遮罩 + 面板）。在 dock 的胶囊之后调用。
func (p *Plugin) drawStatsDialog(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme) {
	if f == nil || env == nil || t == nil || !p.dialog.open {
		return
	}
	stats := env.Data.SessionStats
	rows := statsDialogRows(stats)
	// 会话切换 / 真值没了 / 没有行可画 → 关掉（官方 dock 是 session 作用域）。
	if env.Data.Cur == "" || env.Data.Cur != p.dialog.session || len(rows) == 0 {
		p.closeStatsDialog()
		return
	}
	counts := statsCountsText(stats)

	// ① 遮罩：官方 stat-dialog.ts 的 `useDismissOnOutsidePointer`。全窗一层透明
	// 命中面——点它 = 点弹窗外面 = 关闭。它先于面板声明，所以面板在它上面，
	// 点弹窗内部落在面板自己的命中区上（不会误关）。
	if env.Bounds.Width > 0 && env.Bounds.Height > 0 {
		backdrop := f.Popup().At(0, 0).Size(env.Bounds.Width, env.Bounds.Height).
			HitWith(statsTreeID(env, statsDialogBackdrop), "", ui.HitOptions{SkipFocus: true})
		if backdrop.Clicked() {
			p.closeStatsDialog()
			return
		}
	}

	// ② 面板：锚在胶囊上方 8px（官方 bottom: anchor(top)+8）、宽按 max-content 夹。
	width := statsDialogWidth(env, rows, counts)
	anchor := statsDialogAnchor(p.statsDialogPillRect(env), width, env.Bounds)
	panel := f.Popup().Above(anchor, StatsDialogAnchorGap, flex.Start).Width(width)
	panel = panel.Radius(t.Radii.LG).Background(t.BgLayer3).Shadow(ui.ShadowLG).
		Pad(StatsDialogPad)
	// 弹窗本体吃掉点击：官方点框内不关闭（stat-dialog.ts 只认"外面"）。
	// 角色用 RoleGroup：viva v0.2.3 没有 RoleDialog，Group 是"成组的非动作区域"。
	panel = panel.HitWith(statsTreeID(env, statsDialogPanelHit), zhStatsDialogTitle,
		ui.HitOptions{Role: ui.RoleGroup})

	labelColumn := statsDialogLabelColumn(env, rows)
	panel.Children(func() {
		// .title：章节名左、头条数值右（flex space-between + gap16 + mb8 + 500）。
		f.Row().FillWidth().Items(flex.Center).Gap(StatsDialogTitleGap).
			MarginSides(0, 0, StatsDialogTitleMarginBottom, 0).Children(func() {
			f.Text(zhStatsDialogTitle).FontSize(StatsDialogFontSize).
				LineHeight(StatsDialogLineHeight).Weight(ui.WeightMedium).Color(t.LabelPrimary)
			f.Spacer()
			f.Text(counts).FontSize(StatsDialogFontSize).LineHeight(StatsDialogLineHeight).
				Weight(ui.WeightMedium).Color(t.LabelPrimary)
		})
		// .titleRule：0.5px border-top（border-l2）+ mb10。
		f.Column().FillWidth().Height(StatsDialogTitleRuleHeight).Background(t.BorderL2).
			MarginSides(0, 0, StatsDialogTitleRuleMarginBottom, 0)
		// dl.details：两列 + 行距 6；dt 列宽 = max(76, 最长标签)（官方 minmax(76px,auto)）。
		f.Column().FillWidth().Gap(StatsDialogDetailsRowGap).Children(func() {
			for _, row := range rows {
				f.Row().FillWidth().Items(flex.Center).Gap(StatsDialogDetailsColumnGap).Children(func() {
					f.Row().Width(labelColumn).Children(func() {
						f.Text(row.Label).FontSize(StatsDialogFontSize).
							LineHeight(StatsDialogLineHeight).Color(t.LabelTertiary).Ellipsis()
					})
					f.Spacer()
					f.Text(row.Value).FontSize(StatsDialogFontSize).
						LineHeight(StatsDialogLineHeight).Color(t.LabelSecondary)
				})
			}
		})
	})
	p.dialog.panel, p.dialog.panelDrawn = panel, true
}

// statsDialogAnchor 返回交给 popup 的锚点矩形。
//
// 纵向由 viva 的 `Above` 负责（等价官方 `bottom: anchor(top)+8`，放不下时翻到
// 下方 = 官方 `position-try-fallbacks: --dsh-stat-opposite`）；横向按官方
// `left: max(12px, anchor(left))` 取，再往窗口内夹一档——比官方**更严一档**：
// 靠右时还会往左推，保证整块不出窗（官方 max-width 已预留 24px，正常窗口下
// 两者结果相同）。
func statsDialogAnchor(anchor ui.Rect, width float32, bounds ui.Rect) ui.Rect {
	left := anchor.X
	if left < StatsDialogWindowInset {
		left = StatsDialogWindowInset
	}
	if bounds.Width > 0 {
		if maxLeft := bounds.Width - StatsDialogWindowInset - width; left > maxLeft {
			left = maxLeft
			if left < StatsDialogWindowInset {
				left = StatsDialogWindowInset
			}
		}
	}
	return ui.Rect{X: left, Y: anchor.Y, Width: width, Height: anchor.Height}
}

// statsDialogPillRect 是这一帧可用的锚点：优先上一帧量到的胶囊真实矩形，
// 还没画过胶囊时退回过渡几何。
func (p *Plugin) statsDialogPillRect(env *pluginapi.Env) ui.Rect {
	if rect := p.dialog.pillRect; rect.Width > 0 && rect.Height > 0 {
		return rect
	}
	return placeholderStatsAnchor(env)
}

// placeholderStatsAnchor 是"胶囊还没被真画过"时的过渡锚点（首帧 / 程序化打开）。
//
// 真机上弹窗只可能在用户点过胶囊之后出现，而胶囊每帧都画、AfterDraw 每帧都缓存
// 矩形，所以这条路径只服务"没画过胶囊就打开"的场景（测试、未来的命令面板）。
func placeholderStatsAnchor(env *pluginapi.Env) ui.Rect {
	rect := ui.Rect{Width: 120, Height: pillHeight}
	if env == nil || env.Bounds.Width <= 0 || env.Bounds.Height <= 0 {
		return rect
	}
	rect.X = (env.Bounds.Width - rect.Width) / 2
	rect.Y = env.Bounds.Height - rect.Height - StatsDialogWindowInset
	return rect
}

// statsDialogWidth 是官方 `width: max-content` + 两条 clamp（stat-dialog.module.css:10-12）。
//
//	avail = 100vw - 2×12
//	minW  = min(300, avail)      // min-width: min(300px, calc(100vw - 24px))
//	maxW  = min(440, avail)      // max-width: min(440px, calc(100vw - 24px))
//	w     = clamp(内容宽 + 2×16, minW, maxW)
func statsDialogWidth(env *pluginapi.Env, rows []statsRow, counts string) float32 {
	available := StatsDialogMaxWidth
	if env != nil && env.Bounds.Width > 0 {
		available = env.Bounds.Width - 2*StatsDialogWindowInset
	}
	minWidth := minFloat(StatsDialogMinWidth, available)
	maxWidth := minFloat(StatsDialogMaxWidth, available)
	if maxWidth < minWidth {
		maxWidth = minWidth
	}
	width := statsDialogContentWidth(env, rows, counts) + 2*StatsDialogPad
	if width < minWidth {
		width = minWidth
	}
	if width > maxWidth {
		width = maxWidth
	}
	return width
}

// statsDialogContentWidth 量这一份内容天然多宽（官方 max-content）：
// 标题行（章节名 + gap16 + 头条数值）与明细行（dt 列 + gap16 + 最长 dd）取大者。
func statsDialogContentWidth(env *pluginapi.Env, rows []statsRow, counts string) float32 {
	title := statsTextWidth(env, zhStatsDialogTitle) + StatsDialogTitleGap + statsTextWidth(env, counts)
	details := float32(0)
	if len(rows) > 0 {
		details = statsDialogLabelColumn(env, rows) + StatsDialogDetailsColumnGap
		maxValue := float32(0)
		for _, row := range rows {
			if measured := statsTextWidth(env, row.Value); measured > maxValue {
				maxValue = measured
			}
		}
		details += maxValue
	}
	if title > details {
		return title
	}
	return details
}

// statsDialogLabelColumn 是 dt 列宽（官方 `minmax(76px, auto)` 的 auto 侧）：
// 下界 76，上界是最长标签的实测宽（向上取整 + 1px float 容差，避免"刚好放不下"
// 被 ellipsis 吃掉——同样的 float32 往返坑见 officialPill 的注释）。
func statsDialogLabelColumn(env *pluginapi.Env, rows []statsRow) float32 {
	width := StatsDialogDetailsLabelMinWidth
	for _, row := range rows {
		if measured := statsTextWidth(env, row.Label); measured > width {
			width = measured
		}
	}
	return float32(math.Ceil(float64(width))) + 1
}

// statsTextWidth 是一次真实文本量算；没有 canvas（纯几何单测）时返回 0。
func statsTextWidth(env *pluginapi.Env, value string) float32 {
	if env == nil || env.Canvas == nil || value == "" {
		return 0
	}
	return env.Canvas.TextWidth(value, StatsDialogFontSize)
}

// toggleStatsDialog 切换展开态（胶囊被点中时调用）。
func (p *Plugin) toggleStatsDialog(env *pluginapi.Env) {
	if p.dialog.open {
		p.closeStatsDialog()
		return
	}
	// 没有可画的行就不开：官方在那种情况下画的是 PlainPill（不可点）。
	if env == nil || len(statsDialogRows(env.Data.SessionStats)) == 0 {
		return
	}
	p.dialog.open = true
	p.dialog.session = env.Data.Cur
}

// closeStatsDialog 关掉弹窗（点外面 / Esc / 切会话 / 真值消失共用）。
func (p *Plugin) closeStatsDialog() {
	p.dialog.open = false
	p.dialog.session = ""
}

// afterOfficialDock 在布局之后缓存胶囊与面板的真实矩形（下一帧的锚点与几何断言）。
func (p *Plugin) afterOfficialDock(_ *ui.Canvas, _ *pluginapi.Env) {
	if p.dialog.pillDrawn {
		if rect := p.dialog.pill.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.dialog.pillRect = rect
		}
	}
	p.dialog.pillDrawn = false
	if p.dialog.panelDrawn {
		if rect := p.dialog.panel.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.dialog.panelRect = rect
		}
	}
	p.dialog.panelDrawn = false
}

// statsDialogOpen 报告弹窗此刻是否**真的展开**（状态 + 可画的行都在）。
func (p *Plugin) statsDialogOpen(d pluginapi.Snapshot) bool {
	return p.dialog.open && p.dialog.session == d.Cur && len(statsDialogRows(d.SessionStats)) > 0
}

// HandleKey 实现 pluginapi.KeyHandler：弹窗开着时 Esc 关闭它
// （官方 stat-dialog.ts:36-38 的 keydown → Escape）。
//
// **优先级说明**：kernel 按装配顺序转发按键（第一个返回 true 的赢），本插件排在
// dscomposer 之后，而 dscomposer 在"没有审批"时把 Esc 当"停止 turn"。所以生产
// 路径上"turn 正在跑"时 Esc 会先被 composer 吃掉，弹窗还在——这是已知差异，报告
// §剩余差距登记。没有可取消的 turn 时（Cancel 未接线 / 测试）Esc 会落到这里。
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	if ev.Release || !p.dialog.open {
		return false
	}
	if ev.Key != ui.KeyEscape || ev.Modifiers != 0 {
		return false
	}
	p.closeStatsDialog()
	return true
}

// minFloat 是两个 float32 取小。
func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
