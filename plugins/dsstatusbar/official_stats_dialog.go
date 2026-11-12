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
// # 第二枚弹窗：用量（官方 `UsagePill` 的 DialogPill）
//
// 官方 dock 上两颗胶囊各有一个**独立**弹窗（`ActivityPill` → 会话统计 /
// `UsagePill` → Token 用量），同一时刻只开一个（`stat-dialog.ts:21-55`）。
// 本文件用同一个 `statsDialogState` 承载两个 kind（`officialPillKind`）：
//
//	活动  标题 `会话统计` + `{turns} 轮 {steps} 步`，四条计时明细（statsDialogRows）
//	用量  标题 `Token 用量` + `{exact} tok`，四桶 + 缓存命中（usageDialogRows）
//
// 用量胶囊的可点性由**本文件自己的命中层**提供（`drawUsagePillHit`）：dock 的
// 胶囊循环只记住**最后**一颗可点胶囊的矩形，把两颗都标成 interactive 会把活动
// 胶囊的锚点挤掉，而 `official_dock.go` 此刻属于并行写者（HI 的动画/图标单）。
// 用量胶囊的矩形用官方 dock 几何从活动胶囊的矩形推出来（同行同高、间距 dockGap）。
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
	usagePillHitName    = "usage-pill"
	statsDialogPanelHit = "stats-dialog-panel"
	statsDialogBackdrop = "stats-dialog-backdrop"
)

// usagePillHitSlack 是用量胶囊命中区往右多留的宽度。
//
// 命中区的位置由官方 dock 几何从活动胶囊的矩形推出来（同行、间距 dockGap），
// 宽度是"官方胶囊内容宽"的实测估计（`2×pillPadX + pillGap + 图标 + 文字`）。
// 右边是空白，多留 24px 只会让命中区略超胶囊右沿，不会抢别的控件，却能让
// 整颗胶囊都点得到（真机上胶囊比估计宽一点点也不至于点不到）。
const usagePillHitSlack float32 = 24

// officialPillKind 区分 dock 上两颗胶囊（官方就是两个独立的 DialogPill）。
type officialPillKind int

const (
	// pillActivity 是官方 ActivityPill（会话统计）。
	pillActivity officialPillKind = iota
	// pillUsage 是官方 UsagePill（Token 用量）。
	pillUsage
)

// statsDialogState 是 dock 弹窗的跨帧状态（两个 kind 共用一份：官方同时只开一个）。
//
// pill / panel 两个 Box 与它们的矩形只在**同一帧内**有意义：`Box.Rect()` 要等
// 布局解算之后才有效（Draw 阶段读到的是上一帧的值），所以按
// `plugins/dsagentteam/model_slot.go` 的老办法——Draw 阶段记下盒子，
// **AfterDraw** 读矩形，下一帧用它当弹窗锚点。
type statsDialogState struct {
	open    bool
	kind    officialPillKind
	session string // 展开时所在会话；切会话即关闭

	pill      flex.Box
	pillDrawn bool
	pillRect  ui.Rect

	// usagePillRect 是**用量**胶囊的矩形：由 AfterDraw 从活动胶囊的矩形加官方
	// dock 几何算出来（见 afterOfficialDock），不来自 dock 的胶囊缓存。
	usagePillRect ui.Rect

	panel      flex.Box
	panelDrawn bool
	panelRect  ui.Rect
}

// officialPillSpec 是一枚 dock 胶囊要画的东西。
//
// `interactive` 就是官方的**两种胶囊**：DialogPill（有弹窗行 → button + hover +
// 命中区）与 PlainPill（没有行 → 静态 span，既不可点也没有 hover）。
type officialPillSpec struct {
	// kind 报告这一枚是活动还是用量（弹窗状态按它分派）。
	kind officialPillKind
	// text 是完整读数（官方 `aria-label={text}`，也是胶囊上那一行字）。
	text string
	// interactive 报告这一枚有没有官方意义上的弹窗行。
	interactive bool
	// icon 是官方胶囊左边那枚 14px glyph：ActivityPill = `IconGaugeOutlineRegular`
	// （sprite `gauge-outline`），UsagePill = `IconDatabaseOutlineRegular`
	// （sprite `database-outline`）。见 StatsPills.tsx:255 / :305。
	icon officialGlyph
}

// officialPillSpecs 是 dock 上两枚胶囊的规格（活动 / 用量）。
//
// 判据逐条对齐官方：
//
//	活动   `len(statsDialogRows(sessionStats)) > 0` 才是 DialogPill；
//	       投影缺席 / steps=0 / 计时全 0 都退回静态读数（官方 :288-289）。
//	用量   官方 `usage === undefined || (billed === 0 && output === 0)` → 不画；
//	       否则是 DialogPill。本仓的"可点"在**活动胶囊有弹窗行**时由本文件自己的
//	       命中层提供（drawUsagePillHit）；活动胶囊没有行时，dock 上那颗唯一的
//	       可点胶囊就是用量，这样锚点矩形仍然来自真实布局而不是估算。
func officialPillSpecs(d pluginapi.Snapshot) []officialPillSpec {
	activityRows := len(statsDialogRows(d.SessionStats)) > 0
	usageDialog := usageHasDialog(d.TokenUsage)
	return []officialPillSpec{
		{kind: pillActivity, text: activitySegment(d), interactive: activityRows, icon: glyphGauge},
		{kind: pillUsage, text: usagePillSegment(d), interactive: usageDialog && !activityRows, icon: glyphDatabase},
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
	if f == nil || env == nil || t == nil {
		return
	}
	// 用量胶囊的命中层每帧都要画（它是那块可点区域的**唯一**声明处），
	// 与"弹窗此刻开着没有"无关。
	p.drawUsagePillHit(f, env, t)
	if !p.dialog.open {
		return
	}
	rows := dialogRowsFor(p.dialog.kind, env.Data)
	title, headValue := dialogHead(p.dialog.kind, env.Data)
	// 会话切换 / 真值没了 / 没有行可画 → 关掉（官方 dock 是 session 作用域）。
	if env.Data.Cur == "" || env.Data.Cur != p.dialog.session || len(rows) == 0 {
		p.closeStatsDialog()
		return
	}

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
	width := statsDialogWidth(env, rows, headValue)
	anchor := statsDialogAnchor(p.dialogAnchorPill(env, p.dialog.kind), width, env.Bounds)
	panel := f.Popup().Above(anchor, StatsDialogAnchorGap, flex.Start).Width(width)
	panel = panel.Radius(t.Radii.LG).Background(t.BgLayer3).Shadow(ui.ShadowLG).
		Pad(StatsDialogPad)
	// 弹窗本体吃掉点击：官方点框内不关闭（stat-dialog.ts 只认"外面"）。
	// 角色用 RoleGroup：viva v0.2.3 没有 RoleDialog，Group 是"成组的非动作区域"。
	panel = panel.HitWith(statsTreeID(env, statsDialogPanelHit), title,
		ui.HitOptions{Role: ui.RoleGroup})

	labelColumn := statsDialogLabelColumn(env, rows)
	panel.Children(func() {
		// .title：章节名左、头条数值右（flex space-between + gap16 + mb8 + 500）。
		f.Row().FillWidth().Items(flex.Center).Gap(StatsDialogTitleGap).
			MarginSides(0, 0, StatsDialogTitleMarginBottom, 0).Children(func() {
			// .titleLabel：`display:inline-flex;align-items:center;gap:6px` +
			// `.titleLabel svg{width:14px;height:14px;flex:none}`。官方章节名左边是
			// **这枚胶囊自己的** glyph（Activity = gauge / Usage = database）。
			f.Row().Items(flex.Center).Children(func() {
				f.Icon(pillIcon(dialogGlyph(p.dialog.kind), t.LabelPrimary)).
					Size(StatsDialogTitleIconSize, StatsDialogTitleIconSize).Color(t.LabelPrimary)
				f.Row().Width(StatsDialogTitleLabelGap).Height(StatsDialogTitleIconSize)
				f.Text(title).FontSize(StatsDialogFontSize).
					LineHeight(StatsDialogLineHeight).Weight(ui.WeightMedium).Color(t.LabelPrimary)
			})
			f.Spacer()
			f.Text(headValue).FontSize(StatsDialogFontSize).LineHeight(StatsDialogLineHeight).
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
	// 官方 .titleLabel 是「14px glyph + gap6 + 章节名」的一格，宽度要连图标一起算，
	// 否则图标一加进来章节名就会被 Ellipsis 吃掉最后一个字。
	title := StatsDialogTitleIconSize + StatsDialogTitleLabelGap + statsTextWidth(env, zhStatsDialogTitle) +
		StatsDialogTitleGap + statsTextWidth(env, counts)
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

// toggleStatsDialog 切换展开态（dock 上那颗可点胶囊被点中时调用）。
//
// dock 的胶囊循环只认一个命中 id（`statsPillHitName`），而"可点的那一颗"是
// 活动（有计时行）或用量（活动没有行时退回它，见 officialPillSpecs）。所以
// 这里按同一判据选 kind：有活动行 → 活动弹窗；否则 → 用量弹窗。
func (p *Plugin) toggleStatsDialog(env *pluginapi.Env) {
	if env == nil {
		return
	}
	kind := pillActivity
	if len(statsDialogRows(env.Data.SessionStats)) == 0 {
		kind = pillUsage
	}
	p.toggleDialog(kind, env)
}

// toggleDialog 按 kind 切换弹窗（官方 `setOpen(!open)`：点已开的胶囊关闭它，
// 点另一颗则切换——同一时刻只有一个弹窗，`stat-dialog.ts:21-55`）。
func (p *Plugin) toggleDialog(kind officialPillKind, env *pluginapi.Env) {
	if env == nil {
		return
	}
	if p.dialog.open && p.dialog.kind == kind && p.dialog.session == env.Data.Cur {
		p.closeStatsDialog()
		return
	}
	// 没有可画的行就不开：官方在那种情况下画的是 PlainPill（不可点）。
	if len(dialogRowsFor(kind, env.Data)) == 0 {
		return
	}
	p.dialog.open = true
	p.dialog.kind = kind
	p.dialog.session = env.Data.Cur
}

// closeStatsDialog 关掉弹窗（点外面 / Esc / 切会话 / 真值消失共用）。
func (p *Plugin) closeStatsDialog() {
	p.dialog.open = false
	p.dialog.kind = pillActivity
	p.dialog.session = ""
}

// dialogRowsFor 返回某个 kind 这一刻能画的明细行（空 = 官方不画这个弹窗）。
func dialogRowsFor(kind officialPillKind, data pluginapi.Snapshot) []statsRow {
	if kind == pillUsage {
		return usageDialogRows(data.TokenUsage)
	}
	return statsDialogRows(data.SessionStats)
}

// dialogHead 返回某个 kind 的（章节名, 头条数值）——官方 `.title` 的两格。
func dialogHead(kind officialPillKind, data pluginapi.Snapshot) (title, value string) {
	if kind == pillUsage {
		return zhUsageDialogTitle, usageTitleValue(data.TokenUsage)
	}
	return zhStatsDialogTitle, statsCountsText(data.SessionStats)
}

// dialogGlyph 返回某个 kind 标题左侧那枚官方 glyph（与胶囊上同一枚）。
func dialogGlyph(kind officialPillKind) officialGlyph {
	if kind == pillUsage {
		return glyphDatabase
	}
	return glyphGauge
}

// dialogOpenFor 报告某个 kind 的弹窗此刻是不是**真的开着**（状态 + 会话 + 行都在）。
func (p *Plugin) dialogOpenFor(kind officialPillKind, data pluginapi.Snapshot) bool {
	if !p.dialog.open || p.dialog.kind != kind || p.dialog.session != data.Cur {
		return false
	}
	return len(dialogRowsFor(kind, data)) > 0
}

// dialogAnchorPill 返回某个 kind 的弹窗锚点胶囊矩形。
//
// 活动：dock 缓存的活动胶囊真实矩形；还没画过时用过渡几何。
// 用量：AfterDraw 推出的用量胶囊矩形；没推出来时同样退回过渡几何。
func (p *Plugin) dialogAnchorPill(env *pluginapi.Env, kind officialPillKind) ui.Rect {
	if kind == pillUsage {
		if rect := p.dialog.usagePillRect; rect.Width > 0 && rect.Height > 0 {
			return rect
		}
		return placeholderStatsAnchor(env)
	}
	return p.statsDialogPillRect(env)
}

// afterOfficialDock 在布局之后缓存胶囊与面板的真实矩形（下一帧的锚点与几何断言）。
//
// 用量胶囊的矩形也在这里算：dock 的胶囊循环只把**最后**一颗可点胶囊的 Box 交出来
// （`official_dock.go` 的 `p.dialog.pill`），活动的矩形靠它拿到；用量那颗用官方
// dock 几何补齐——同一行（Y/Height 相同）、紧随活动胶囊、间距 `dockGap`
// （`InputBar.module.css:13` 的 `.dock{gap:12}`），见 usagePillRectFrom。
func (p *Plugin) afterOfficialDock(_ *ui.Canvas, env *pluginapi.Env) {
	if p.dialog.pillDrawn {
		if rect := p.dialog.pill.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.dialog.pillRect = rect
		}
	}
	p.dialog.pillDrawn = false
	p.dialog.usagePillRect = p.usagePillRectFrom(env)
	if p.dialog.panelDrawn {
		if rect := p.dialog.panel.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.dialog.panelRect = rect
		}
	}
	p.dialog.panelDrawn = false
}

// usagePillRectFrom 推出用量胶囊的矩形。
//
// 两种情况：
//
//	① 活动胶囊可点（有计时行）→ 缓存的是**活动**胶囊矩形：用量胶囊 = 右沿 + dockGap，
//	   同行同高；宽度按胶囊内容实测（`2×pillPadX + pillGap + 图标 + 文字`，
//	   见 official_dock.go 的 drawPillIcon）。
//	② 活动胶囊不可点（没有计时行）→ dock 上那颗唯一的可点胶囊**就是**用量
//	   （officialPillSpecs 的 interactive），缓存的矩形已经是它本身。
//
// 推不出来（还没画过胶囊 / 没有 canvas）时返回零矩形 = 「不知道位置」，
// 调用方据此不声明命中区（不画一个位置错误的假入口）。
func (p *Plugin) usagePillRectFrom(env *pluginapi.Env) ui.Rect {
	base := p.dialog.pillRect
	if env == nil || base.Width <= 0 || base.Height <= 0 {
		return ui.Rect{}
	}
	if len(statsDialogRows(env.Data.SessionStats)) == 0 {
		return base
	}
	width := p.usagePillWidth(env)
	if width <= 0 {
		return ui.Rect{}
	}
	return ui.Rect{
		X:      base.X + base.Width + dockGap,
		Y:      base.Y,
		Width:  width,
		Height: base.Height,
	}
}

// usagePillWidth 是官方胶囊内容的实测宽：`2×pillPadX + pillGap + 图标 + 文字`。
//
// `pillPadX` / `pillGap` / `StatsPillIconSize` 都是 `official_dock.go` 画胶囊时
// 用的同一批常量（并行单 HI 已确认不再改这三项），所以这里算出来的宽度与真实
// 布局同源；变化的是文字（读数随用量增长），所以每帧重算。
func (p *Plugin) usagePillWidth(env *pluginapi.Env) float32 {
	if env == nil || env.Canvas == nil {
		return 0
	}
	text := ""
	for _, spec := range officialPillSpecs(env.Data) {
		if spec.kind == pillUsage {
			text = spec.text
		}
	}
	if text == "" {
		return 0
	}
	theme := p.themeOf(env)
	fontSize := StatsDialogFontSize
	if theme != nil {
		fontSize = theme.Fonts.Caption
	}
	return 2*pillPadX + pillGap + StatsPillIconSize + env.Canvas.TextWidth(text, fontSize)
}

// drawUsagePillHit 声明用量胶囊的命中区（官方 dock 上它是第二颗 DialogPill）。
//
// # 为什么不在 dock 的胶囊循环里给它标 interactive
//
// `official_dock.go` 的循环只记住**最后**一个可点胶囊的 Box（`p.dialog.pill`），
// 两颗都标 interactive 会把活动胶囊的锚点挤掉；而那个文件属于并行写者（HI 的
// 动画/图标单），本单不改它。这里用同一行几何推出用量胶囊的位置，命中区在本文件
// 里画，活动胶囊的锚点不受影响。
//
// # 命中区只在官方会画 DialogPill 时声明
//
// `usageHasDialog` 为假（键缺席 / 四桶全 0）时官方整枚不渲染、也没有弹窗：
// 那时声明命中区就是一个假入口（点了没东西）。未知与真实的 0 在**读数**上
// 已经分开（「未知」vs `0 tok`），这里只表达"有没有弹窗"。
//
// # 视觉缺口（登记）
//
// 官方这颗胶囊有 hover / `aria-expanded` 底色（`interactive-bg-hover`）。本层只
// 声明命中区与 a11y（button + expandable + expanded），**不画底色**：底色要在
// 胶囊文字下面（本文件的 Popup 只会画在文字上面），得由 official_dock.go 的
// DialogPill 分支来画——那条改动留给集成（报告 §未实现第 1 条）。
func (p *Plugin) drawUsagePillHit(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme) {
	if env == nil || t == nil {
		return
	}
	// 活动没有计时行时，dock 上那颗可点胶囊就是用量本身（命中区已经由它声明）。
	if len(statsDialogRows(env.Data.SessionStats)) == 0 {
		return
	}
	if !usageHasDialog(env.Data.TokenUsage) {
		return
	}
	rect := p.dialog.usagePillRect
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	id := statsTreeID(env, usagePillHitName)
	label := usagePillText(env.Data.TokenUsage)
	open := p.dialogOpenFor(pillUsage, env.Data)
	if !id.IsZero() && env.Canvas != nil && env.Canvas.Clicked(id) {
		p.toggleDialog(pillUsage, env)
		open = p.dialogOpenFor(pillUsage, env.Data)
	}
	box := f.Popup().At(rect.X, rect.Y).Size(rect.Width+usagePillHitSlack, rect.Height)
	if !id.IsZero() {
		box = box.HitWith(id, label, ui.HitOptions{
			Role: ui.RoleButton, Expandable: true, Expanded: open,
		})
	}
}

// statsDialogOpen 报告 dock 上那颗可点胶囊此刻是否**真的展开**（状态 + 可画的行都在）。
//
// 它只服务 dock 的 interactive 分支（活动，或活动没有行时的用量），所以 kind 按
// 同一判据选，与 toggleStatsDialog 一致。
func (p *Plugin) statsDialogOpen(d pluginapi.Snapshot) bool {
	if !p.dialog.open || p.dialog.session != d.Cur {
		return false
	}
	kind := pillActivity
	if len(statsDialogRows(d.SessionStats)) == 0 {
		kind = pillUsage
	}
	return p.dialogOpenFor(kind, d)
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
