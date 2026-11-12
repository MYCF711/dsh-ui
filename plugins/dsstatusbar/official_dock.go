// official_dock.go —— 状态栏的**官方座位**（rc.2 `conversation.composer.dock`）。
//
// # 官方挂在哪里（逐字）
//
// `ui-conversation/src/client/contract/slots.ts:312-318`：
//
//	/** Ambient entries below the composer card; … */
//	'conversation.composer.dock': { kind: 'list'; scope: 'session' }
//
// 官方把 Activity / Usage / ContextMeter 这三枚读数胶囊画在 `InputBar.dock`
// 里（`InputBar.module.css:13` 的 `.dock`：`justify-content:center; gap:12;
// padding-top:4`；非 hero 的 `min-height:calc(22px + font-delta-secondary)`），
// 也就是**输入卡片下方**、以 composer 为宿主的环境条目。结构树 §7 对
// dsstatusbar 的这一条判定同样是「错层：仍向 shell.bottom 注册固定连接行」，
// 本文件把它挪回官方座位。
//
// # 官方子槽今天**不可达**，所以挂同一个 chain 座位
//
// 与 dsgoal 同一条原因（GR 报告 §8 第 1 条）：composer 占用方还没转发自己的
// 子槽，`conversation.composer.dock` 今天不会被任何人 dispatch。照着官方槽名
// 注册会得到一个"登记了但永远画不到"的座位。
//
// 所以本插件挂在**真的会被 dispatch 的那个官方座位**上：`conversation.composer`
// （chain），用相对顺序表达官方位置（dock 在输入卡片**下方**，所以 Order 20 排在
// dscomposer 的 10 之后）：
//
//	conversation.composer
//	├─ questions-bar-official (3)  ├─ goal-bar-official (5)
//	├─ composer-official      (10) ← 输入卡片
//	└─ dock-official          (20) ← 本插件：卡片下方那行读数胶囊
//
// 等 dscomposer 转发 `.dock` 之后，把 painter 的 Slot 换成
// `uikernel.OffConvComposerDock` 即可（一行改动，报告里登记为剩余差距）。
//
// # 可见性（比官方**更严**的一处显式差异）
//
// 官方 C20：缺真值的 pill 各自**不渲染**。本仓跨线 §12 与 GR 报告 §3 的口径是
// 「结构位置存在的读数必须写『未知』」——所以这一行**常驻**，三枚胶囊各自由
// 自己的真值决定写数还是写「未知」。**未知不写成 0、不写成 0%。**
package dsstatusbar

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	// 官方 `conversation.composer.dock` 是"卡片下方的环境条目"；本仓挂在 chain
	// 座位上，用**相对顺序**表达同一位置：排在输入卡片（dscomposer 的 10）之后。
	dockOfficialOrder = 20
)

// composerSeatLive 报告**这一帧**官方 composer 座位是不是活的。
//
// 判据只有档位（与 dsgoal / dsquestions 同源、理由同）：legacy 档位 dsshell 只画
// 旧槽；official 档位官方骨架一定 dispatch `conversation.composer`（本插件也注册在
// 它上面，所以 `BridgeLegacyComposer` 必为假）。不能用 `OfficialSlotsLive()`：
// `-shell official` 的产品形状里整列兼容桥还开着，但 composer 是分段开关。
func (p *Plugin) composerSeatLive() bool { return p.tier.Official() }

// drawOfficialDock 画官方 `conversation.composer.dock`：卡片下方一行居中的读数胶囊。
//
// 可见性：**常驻**（有会话就画）；每枚胶囊的字段按宿主实际能力写数或写「未知」。
// 有官方弹窗行的那一枚（活动）按官方 DialogPill 画成 button（hover + 命中区），
// 并在被点开时画详情弹窗；其余保持官方 PlainPill 的静态读数。
func (p *Plugin) drawOfficialDock(f ui.C, env *pluginapi.Env) {
	if !p.composerSeatLive() {
		// 旧档位没有这个座位：弹窗跟着座位一起消失，别留一个飞在旧界面上的浮层。
		p.closeStatsDialog()
		return
	}
	// 官方这个槽是 session 作用域：没有会话就不画（Hero 阶段 Hero 内嵌 composer
	// 自己那条 dock 由骨架画，不在这里重复）。
	if env.Data.Cur == "" {
		p.closeStatsDialog()
		return
	}
	t := p.themeOf(env)
	specs := officialPillSpecs(env.Data)
	var pill ui.Box
	var pillDrawn bool
	ui.Row(f).FillWidth().Height(dockPaddingTop+pillHeight).
		PadSides(dockPaddingTop, 0, 0, 0).
		Items(ui.Center).Justify(ui.Center).Gap(dockGap).Children(func() {
		for _, spec := range specs {
			box, drawn := p.officialPill(f, t, env, spec)
			if spec.interactive {
				pill, pillDrawn = box, drawn
			}
		}
	})
	// 弹窗锚点要的是"这一帧胶囊的真实矩形"，Rect() 只有布局后才有效 → AfterDraw 取。
	p.dialog.pill, p.dialog.pillDrawn = pill, pillDrawn
	p.drawStatsDialog(f, env, t)
}

// officialPill 画一枚读数胶囊（官方 `.pill`：20 行高、左右 8、radius sm）。
//
// # 为什么左右内边距用**定宽占位盒**而不是 `PadSides`
//
// viva 的换行判据是 `TextWidthWeight(line) <= rect.Width - padX()`。胶囊的宽度
// 是"文字 pref + 左右 padding"再减回 padding，float32 往返会差 ~1e-7，于是
// 刚好放得下的读数被判成放不下，末尾被吃掉几个字符——真机验收里
// 「1 轮 2 步 · 571 tok/s」被截成「1 轮 2 步 · 571 tok…」（HC 真机报告）。
//
// 用定宽占位盒时，文字节点拿到的就是**它自己的 pref 宽度**（不是容器宽减
// padding），不存在这次往返。同样的修法见 `plugins/dsagentteam/model_slot.go`
// 的「为什么内边距用定宽占位盒而不是 PadSides」。
// 返回的 Box 是本帧胶囊（AfterDraw 从它读锚点矩形）；bool 表示这一帧真的画了
// 一颗**可点**的胶囊（官方 DialogPill）。
func (p *Plugin) officialPill(f ui.C, t *pluginapi.Theme, env *pluginapi.Env, spec officialPillSpec) (ui.Box, bool) {
	if !spec.interactive {
		// 官方 PlainPill：静态 span。没有命中区、没有 hover——画一个点不开的
		// 按钮比不画更糟（用户会以为它坏了）。
		box := ui.Row(f).Height(pillHeight).Radius(t.Radii.SM).
			Items(ui.Center).Gap(0).Children(func() {
			ui.Row(f).Width(pillPadX).Height(pillHeight)
			p.drawPillIcon(f, spec.icon, t.LabelTertiary)
			ui.Text(f, spec.text).FontSize(t.Fonts.Caption).LineHeight(pillLineHeight).
				Color(t.LabelTertiary).Ellipsis()
			ui.Row(f).Width(pillPadX).Height(pillHeight)
		})
		return box, false
	}
	// 官方 DialogPill：`button.pill`，悬停/展开时底 = interactive-bg-hover、
	// 字 = label-secondary（StatsPills.module.css:38-52）。
	id := statsTreeID(env, statsPillHitName)
	// **在画之前**处理这次点击：官方是 `setOpen(!open)` 之后重渲染，胶囊的
	// `aria-expanded`/底色与同一帧的弹窗必须是同一个状态。viva 的
	// `Canvas.Clicked(id)` 只比对本次事件派发的 id，不需要先声明命中区，
	// 所以这里可以先改状态、再按新状态画胶囊与弹窗。
	if !id.IsZero() && env != nil && env.Canvas != nil && env.Canvas.Clicked(id) {
		p.toggleStatsDialog(env)
	}
	open := p.statsDialogOpen(env.Data)
	lit := statsHovered(env, id, open)
	textColor := t.LabelTertiary
	if lit {
		textColor = t.LabelSecondary
	}
	box := ui.Row(f).Height(pillHeight).Radius(t.Radii.SM).Hover(ui.Look{}.Background(t.Hover))
	if open {
		// `button.pill[aria-expanded=true]{background: interactive-bg-hover}`
		box = box.Background(t.Hover)
	}
	box = box.Items(ui.Center).Gap(0).Children(func() {
		ui.Row(f).Width(pillPadX).Height(pillHeight)
		p.drawPillIcon(f, spec.icon, textColor)
		ui.Text(f, spec.text).FontSize(t.Fonts.Caption).LineHeight(pillLineHeight).
			Color(textColor).Ellipsis()
		ui.Row(f).Width(pillPadX).Height(pillHeight)
	})
	if !id.IsZero() {
		// 官方 trigger 是 button：`aria-haspopup="dialog"` + `aria-expanded={open}`。
		box = box.HitWith(id, spec.text, gfx.HitOptions{
			Role: gfx.RoleButton, Expandable: true, Expanded: open,
		})
	}
	return box, true
}

// drawPillIcon 画官方胶囊左边那枚 14px glyph（`StatsPills.module.css:30-34`）。
//
// 官方几何：`.pill{display:inline-flex;align-items:center;gap:6px}` +
// `.pill svg{width:14px;height:14px;flex:none}`。gap 用的是同一张几何表里的
// `pillGap`（=6），随图标一起出现，所以胶囊宽度 = 8 + 14 + 6 + 文字 + 8。
//
// 图标颜色随胶囊文字色（官方 svg 无自带 color，继承 `.pill{color:…}`）：
// 常态 label-tertiary、悬停/展开 label-secondary（本函数由调用方传色）。
// `viewBox` 仍是官方 16，这里只按官方 CSS 渲染成 14×14。
func (p *Plugin) drawPillIcon(f ui.C, kind officialGlyph, tint gfx.Color) {
	if kind == glyphNone {
		return
	}
	// 官方 `.pill{gap:6px}`：图标与文字之间那 6px 由这里给（不在图标里内缩）。
	ui.Row(f).Width(pillGap).Height(pillHeight)
	ui.Icon(f, pillIcon(kind, tint)).Size(StatsPillIconSize, StatsPillIconSize).Color(tint)
}

// pillIcon 按种类取官方 sprite（Activity = gauge-outline，Usage = database-outline）。
func pillIcon(kind officialGlyph, tint gfx.Color) gfx.Icon {
	switch kind {
	case glyphGauge:
		return gaugeOutlineIcon(tint)
	case glyphDatabase:
		return databaseOutlineIcon()
	default:
		return gfx.Icon{}
	}
}

// officialSegments 是常驻 dock 的两枚读数：活动 / 用量。
//
// # 为什么**不含上下文**
//
// 官方这一行的 ContextMeter 已经由 composer 占用方画了（`plugins/dscomposer`
// `official_seat.go` 的 `drawDock`：`上下文 6.2k / 1M（1%）`，GR 报告 §3 第 8 项）。
// 同一个读数画两遍会让人分不清哪个是"当前上下文"——GR §3 结尾明确分工：
// **Activity / Usage / 缓存率归 dsstatusbar 的 dock**，ContextMeter 归 composer。
//
// # 两枚的结构位置永远在，值按宿主能力给
//
//   - 活动（官方 ActivityPill）：读 `Snapshot.SessionStats`（宿主 whole-log
//     `sessionStats` 投影的八字段）。有值就按官方读数写 `{turns} 轮 {steps} 步`
//     （`decodeMs > 0` 时再拼 ` · {tps} tok/s`）；**没有投影**（老宿主 / 还没拉到）
//     才写「未知」。绝不拿消息窗口数轮/步。
//   - 用量（官方 UsagePill）：`UsageTotal` / 旧 `Usage` 的真值，缺则「未知」。
func officialSegments(d pluginapi.Snapshot) []string {
	specs := officialPillSpecs(d)
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.text)
	}
	return out
}

// 未知文案（与 GR 报告 §3 的「上下文 未知」同一套写法：标签 + 未知）。
const (
	zhActivityUnknown = "活动 未知"
	zhUsageUnknown    = "用量 未知"
)

// activitySegment 是「活动」那一枚：官方 whole-log `sessionStats` 投影的读数。
//
// 两态（这就是"真消费投影"的判据）：
//
//	有投影   → `activityText` 的官方读数（`2 轮 3 步 · 41 tok/s`）
//	没有投影 → 「活动 未知」
//
// 为什么不能从 `Msgs` 数：轮数是 whole-log 的唯一 turn 数、步数是 assistant/step
// 数，消息窗口（tail）既不是全量也不区分 step；用窗口消息凑出来的数字**看起来
// 像真值**，是这一层最坏的一种假（`TestWindowMessagesNeverBecomeOfficialTurnsOrSteps`
// 钉住这条）。**未知不写成 0 步。**
func activitySegment(d pluginapi.Snapshot) string {
	return orUnknown(activityText(d.SessionStats), zhActivityUnknown)
}

// orUnknown 是「有真值写数、没真值写未知」的唯一入口（不写 0、不写空）。
func orUnknown(segment, unknown string) string {
	if segment == "" {
		return unknown
	}
	return segment
}
