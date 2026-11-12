// card.go —— NoticeCard 叠层的画法（官方 `AccountNotifications.module.css` 全量几何）。
//
// 所有取值都来自结构树 §4 的表，**没有一个是我们自选的**：
//
//	.stack   width:100%; max-width:280px
//	.card    padding:20px; border-radius:16px; font-size:14px; line-height:22px;
//	         background:var(--dsw-specific-input-major); box-shadow:elevation-panel;
//	         transform: translateY(-4px × depth) scaleX((252/260)^depth)
//	.copy    flex-column; gap:2px
//	.title   flex; align-items:flex-start; gap:2px; line-height:20px; font-weight:500
//	.icon    width:14px; height:14px; margin-top:1px; color:brand
//	.actions flex-wrap; gap:8px; margin-top:12px
//	.action  max-width:100%; min-height:28px; white-space:normal
//	.close   absolute; top:6px; right:6px; width:18px; height:18px; padding:4px
//
// # 两处**有意降级**（viva 没有 CSS transform，如实登记）
//
//  1. 非前景卡：官方是"整张卡 translateY(-4px×depth) + scaleX(252/260)^depth"，
//     结果只露出顶部一条 4px 的边。这里直接画那条**可见的边**：高度 4px、
//     宽度按 252/260 收窄并按中心对齐。外观一致，DOM 结构不同（本来也没有 DOM）。
//  2. 关卡片头：官方 `top:6px; right:6px` 的绝对定位在 viva 里可用
//     （`Absolute()` 相对 padding box，与 CSS 同义），所以这处**没有**降级。
package dsnotifications

import (
	"math"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方几何（结构树 §4 逐条对应）。
const (
	// StackMaxWidth 是 `.stack{max-width:280px}`——README 的 "expanded launcher up to 280px"。
	StackMaxWidth float32 = 280
	// CardPadding 是 `.card{padding:20px}`。
	CardPadding float32 = 20
	// CardRadius 是 `.card{border-radius:16px}`。
	CardRadius float32 = 16
	// CardFontSize / CardLineHeight 是 `.card{font-size:14px;line-height:22px}`。
	CardFontSize   float32 = 14
	CardLineHeight float32 = 22
	// CopyGap 是 `.copy{gap:2px}`。
	CopyGap float32 = 2
	// TitleLineHeight / TitleWeight 是 `.title{line-height:20px;font-weight:500}`。
	TitleLineHeight float32 = 20
	TitleWeight     float32 = 500
	// IconSize 是 `.icon{width:14px;height:14px}`。
	IconSize float32 = 14
	// IconTopGap 是 `.icon{margin-top:1px}`。
	IconTopGap float32 = 1
	// ActionsGap / ActionsTopMargin 是 `.actions{gap:8px;margin-top:12px}`。
	ActionsGap       float32 = 8
	ActionsTopMargin float32 = 12
	// ActionMinHeight 是 `.action{min-height:28px}`。
	ActionMinHeight float32 = 28
	// CloseSize / CloseInset / CloseIconSize 是 `.close{18×18;top/right:6px}` 与图标本体 10px。
	CloseSize     float32 = 18
	CloseInset    float32 = 6
	CloseIconSize float32 = 10
	// DepthOffset 是 `translateY(-4px × depth)` 的 4px。
	DepthOffset float32 = 4
)

// depthScaleBase 是官方叠层比例 `252/260`（AccountNotifications.tsx:145 的 Math.pow）。
const depthScaleBase = 252.0 / 260.0

// labelClose 是本包**唯一**属于自己的文案：官方语言域 `account.notifications`
// 只有 `close` 一个键（`locales.ts:2-18`，8 语言）。标题/正文/按钮文案全部来自服务端。
const labelClose = "关闭"

// depthScale 算第 depth 层的横向缩放（官方 `Math.pow(252/260, depth)`）。
func depthScale(depth int) float64 { return math.Pow(depthScaleBase, float64(depth)) }

// drawStack 画整摞卡片：非前景层先画（只露可见边），前景卡最后画。
//
// 顺序 = 服务端顺序：`notices[0]` 是前景（唯一可交互的一张）。
func (p *Plugin) drawStack(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, notices []apiclient.Notification) {
	if len(notices) == 0 {
		return
	}
	ui.Column(f).FillWidth().MaxWidth(StackMaxWidth).Children(func() {
		for depth := len(notices) - 1; depth >= 1; depth-- {
			// 官方非前景卡 `pointer-events:none`：这里根本不给命中区，天然不可点。
			ui.Row(f).FillWidth().Height(DepthOffset).Justify(ui.Center).Children(func() {
				ui.Row(f).Height(DepthOffset).
					WidthPercent(float32(depthScale(depth)) * 100).
					Radius(DepthOffset / 2).
					Background(t.InputMajor).
					Shadow(gfx.ShadowXS)
			})
		}
		p.drawCard(f, env, t, notices[0])
	})
}

// drawCard 画前景卡（唯一可交互的一张）。
func (p *Plugin) drawCard(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, n apiclient.Notification) {
	ui.Column(f).FillWidth().Radius(CardRadius).Background(t.InputMajor).Shadow(gfx.ShadowXS).
		Pad(CardPadding).Gap(CopyGap).Children(func() {
		// .copy：标题行 + 正文。`role="status"` 是官方可访问性标注；
		// viva 的 Text 没有 role，写进 aria 只能靠标签语义，此处从简（不影响结构）。
		if n.Title != "" || n.Body != "" {
			ui.Row(f).FillWidth().Items(ui.Start).Gap(CopyGap).Children(func() {
				// .title 的 span.icon 只在有标题时画（官方标题行同时承载 icon 与 title）。
				if n.Title != "" {
					ui.Icon(f, noticeBellIcon(t.BrandPrimary)).Size(IconSize, IconSize).
						MarginSides(IconTopGap, 0, 0, 0)
				}
				ui.Text(f, n.Title).FontSize(CardFontSize).LineHeight(TitleLineHeight).
					Weight(TitleWeight).Color(t.LabelSecondary).Grow(1)
			})
		}
		if n.Body != "" {
			// `.copy p{margin:0;white-space:pre-wrap}`：正文原样换行，不重排。
			ui.Text(f, n.Body).FillWidth().FontSize(CardFontSize).LineHeight(CardLineHeight).
				Color(t.LabelSecondary)
		}

		if len(n.Actions) > 0 {
			// `.actions{display:flex;flex-wrap:wrap;gap:8px;margin-top:12px}`。
			// viva 的 Row 不自动换行（没有 flex-wrap）；动作通常 1–2 个，先按单行排，
			// 超出宽度由父级裁剪——这条差异登记在报告里。
			ui.Row(f).FillWidth().Gap(ActionsGap).MarginSides(ActionsTopMargin, 0, 0, 0).
				Children(func() {
					for i, act := range n.Actions {
						act := act
						btn := ui.Button(f.KeyInt(i), act.Label).Small().
							MinHeight(ActionMinHeight)
						if act.Style != "primary" {
							// 官方 `variant = primary | outline`，按 action.style 选。
							btn = btn.Outline()
						}
						if btn.Clicked() {
							p.activate(env, n, act)
						}
					}
				})
		}

		// .close：18×18，绝对定位 6/6。按钮本体透明（只有命中区），图标单独覆画 10px。
		//
		// 为什么分两步：viva 的 Button 自己画 label，塞不进矢量图标；
		// 图标没有命中区，所以点在图标上仍落到按钮的命中区里（与官方"图标 + 按钮"一致）。
		if ui.Button(f, labelClose).Ghost().IconOnly().
			Size(CloseSize, CloseSize).Absolute().Top(CloseInset).Right(CloseInset).Clicked() {
			p.ctrl.Dismiss(n.ID)
		}
		iconInset := CloseInset + (CloseSize-CloseIconSize)/2
		ui.Row(f).Absolute().Top(iconInset).Right(iconInset).
			Size(CloseIconSize, CloseIconSize).Children(func() {
			ui.Icon(f, closeIcon()).Size(CloseIconSize, CloseIconSize).Color(t.LabelCaption)
		})
	})
}

// activate 执行一个服务端动作（官方动作集**只有两条**，见结构树 §5.2）。
//
//	app_route(name=account) → 先报 cta_clicked，再打开账号设置；卡片**保留**
//	dismiss                → 直接关闭（上报 dismissed 由 Dismiss 负责）
//
// 未知 target 官方 `assertNever` 抛错（闭集）。界面不能抛：这里**不报事件、
// 不关卡片**，只在日志里留痕——比"猜一个动作"诚实。
func (p *Plugin) activate(env *pluginapi.Env, n apiclient.Notification, act apiclient.NotificationAction) {
	switch act.Target.Type {
	case "app_route":
		p.ctrl.CTAClicked(n.ID)
		if !p.ctrl.OpenAccount() && env.Act.Notify != nil {
			// 账号设置面还没落地：点了要**说清楚**，不能装没事。
			env.Act.Notify("账号设置页还没有实现（settings.launcher 父槽待接入）")
		}
	case "dismiss":
		p.ctrl.Dismiss(n.ID)
	default:
		// 官方在这里 assertNever；我们只记日志，不改界面。
		if p.ctrl.log != nil {
			p.ctrl.log.Warn("账号通知动作目标未知（官方是闭集）", "target", act.Target.Type, "id", n.ID)
		}
	}
}

// itoa 是给交互 id 拼序号用的小工具（避免引 fmt 只为一次 Sprintf）。
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := [8]byte{}
	n := len(digits)
	for i > 0 {
		n--
		digits[n] = byte('0' + i%10)
		i /= 10
	}
	return string(digits[n:])
}
