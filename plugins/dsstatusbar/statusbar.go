// Package dsstatusbar 提供 composer 下方的真值指标与独立连接状态。
//
// 官方 Activity/Usage/ContextMeter 属于 conversation.composer.dock，不是出厂
// shell.bottom。本插件利用现有 main.composer 槽，在 ds-composer(order10) 后以
// dock(order20) 画到输入卡之后；连接状态仍保留 shell.bottom，不把绿点冒充上下文环。
//
// 真值边界：
//   - 不从 assistant/tool 消息数推“轮/步”，只接受 whole-log sessionStats；
//   - totalTokens 与可证明的 input/output 保留，旧墙钟速率不冒充 decode 速率；
//   - cacheRead 没有官方三项分母时不显示缓存率；
//   - ContextTokens 是占用、ContextWindow 是容量；只有容量为正且占用不超容量时显示真实百分比。
package dsstatusbar

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/flex"
)

// 官方 InputBar/StatsPills 几何。24 是旧 shell.bottom，40 是 FG-1 caption；
// 它们都不是 dock 高度。官方 dock = padding-top4 + pill(20 行高 + 上下 padding1) = 26。
const (
	dockPaddingTop float32 = 4
	dockGap        float32 = 12
	pillLineHeight float32 = 20
	pillPadY       float32 = 1
	pillPadX       float32 = 8
	pillGap        float32 = 6
	pillHeight             = pillLineHeight + 2*pillPadY

	// 与 ds-composer 共用“官方 clearance16 - dsshell 已给12 = 4”的净边距。
	// 这是现有槽集成的第二个出处；壳层净 padding 改动时必须同步复核。
	dockComposerClearance float32 = 16
	dockShellMainPad      float32 = 12

	contextMeterSize   float32 = 14
	contextMeterStroke float32 = 2
)

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
	// tier 是壳档位（nil = 按 official 处理）。两个座位靠它二选一。
	tier *uikernel.ShellTier
	// dialog 是官方 Activity 详情弹窗的跨帧状态（展开态 + 锚点矩形）。
	dialog statsDialogState
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-statusbar" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	// 可选注入：没有这个能力键（单测 / 旧壳）时按 official（官方槽是终态）。
	if v := ctx.Inject(uikernel.KeyShellTier); v != nil {
		if tier, ok := v.(*uikernel.ShellTier); ok {
			p.tier = tier
		}
	}
	return nil
}

// Panels 把「用量 dock」与「连接指示」各挂到**官方座位 + 兼容桥**（同帧只有一个活）。
//
//	conversation.composer  官方 chain 座位（官方 conversation.composer.dock 今天
//	                       不可达，见 official_dock.go 文件头）—— rc.2 终态（常驻 + 未知）
//	main.composer          旧三栏 / 兼容桥的 dock 位置 —— legacy 档位的旧路径
//	sidebar.settings       官方设置行（SettingsRoot.triggerRow）—— 连接指示的官方家，
//	                       见 connection.go 文件头
//	sidebar.footer         整列还在兼容桥上时的过渡桥（官方槽那一帧不 dispatch）
//
// **不再占 `shell.bottom`**：官方默认 bottom 为空（结构树 W8），旧实现那条
// 全宽「就绪/已断开」行是错层；结构树 §4 S8 的原话是「连接指示由 settings 的
// 连接状态驱动，不能移到底部自创 statusbar」。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "dock", Slot: pluginapi.SlotMainComposer, Order: 20, Draw: func(f *flex.Frame, env *pluginapi.Env) {
			if p.composerSeatLive() {
				return // 官方座位活着：旧槽这一帧不画（不双画）
			}
			p.drawDock(f, env)
		}},
		{ID: "connection", Slot: uikernel.OffSidebarSettings, Order: connSettingsOrder,
			Draw: p.drawConnectionSettingsSeat},
		{ID: "connection-bridge", Slot: pluginapi.SlotSidebarFooter, Order: connFooterBridgeOrder,
			Draw: p.drawConnectionFooterBridge},
		{ID: "dock-official", Slot: uikernel.OffConvComposer, Order: dockOfficialOrder,
			Draw: p.drawOfficialDock, AfterDraw: p.afterOfficialDock},
	}
}

// dockHeight 是官方 dock 的内容高度；没有任何真值段时返回 0，不留黑带。
func dockHeight(segments []string) float32 {
	if len(segments) == 0 {
		return 0
	}
	return dockPaddingTop + pillHeight
}

// drawDock 在输入卡之后画现有 Snapshot 能证明的用量段。Activity、缓存率和
// ContextMeter 百分比只有各自 projection 到位后才能加入，不能在这里拿消息凑数。
func (p *Plugin) drawDock(f *flex.Frame, env *pluginapi.Env) flex.Box {
	// 官方 composer dock 是 session 作用域；空会话时即便上游残留了旧数字也不画。
	if env.Data.Cur == "" {
		return flex.Box{}
	}
	segments := usageSegments(env.Data)
	height := dockHeight(segments)
	if height == 0 {
		return flex.Box{}
	}
	extra := dockComposerClearance - dockShellMainPad
	if extra < 0 {
		extra = 0
	}
	return f.Row().FillWidth().Height(height).
		PadSides(dockPaddingTop, extra, 0, extra).
		Items(flex.Center).Justify(flex.Center).Gap(dockGap).Children(func() {
		for _, segment := range segments {
			p.pill(f, segment)
		}
	})
}

// pill 是官方 PlainPill：12px secondary 降档文字、20px 行高、padding 1px 8px。
// 没有真实弹层数据时不接点击、不画 hover，避免给出可展开假象。
func (p *Plugin) pill(f *flex.Frame, label string) {
	t := p.theme
	f.Row().Height(pillHeight).Radius(t.Radii.SM).
		PadSides(pillPadY, pillPadX, pillPadY, pillPadX).
		Items(flex.Center).Gap(pillGap).Children(func() {
		f.Text(label).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Ellipsis()
	})
}

// usageSegments 决定 legacy dock 当前能显示的真值段，返回空 = 整段不占位。
//
// 两段各自独立：用量（`usageTokenSegment`）与上下文（`contextSegment`）。
// legacy 档位保持「没真值就整段不画」的旧行为；官方 dock 走 `officialSegments`，
// 那里把同一份真值换成「未知」而不是消失（见 official_dock.go）。
func usageSegments(d pluginapi.Snapshot) []string {
	var segments []string
	if segment := usageTokenSegment(d); segment != "" {
		segments = append(segments, segment)
	}
	if segment := contextSegment(contextUsed(d), d.ContextWindow); segment != "" {
		segments = append(segments, segment)
	}
	return segments
}

// usageTokenSegment 是「用量」那一段；空串 = 宿主没给（未知）。
//
// UsageTotal 是真宿主 whole-log 累计，totalTokens 由宿主按 input+output 合计；
// 旧 mockd Usage.TotalTokens 则是最后一次 assistant 的上下文占用，两者不能互换。
func usageTokenSegment(d pluginapi.Snapshot) string {
	wholeLog := d.UsageTotal.Known()
	in, out := d.UsageTotal.InputTokens, d.UsageTotal.OutputTokens
	total := d.UsageTotal.TotalTokens
	if !wholeLog {
		in, out = d.Usage.InputTokens, d.Usage.OutputTokens
		// 旧 Usage 没有 whole-log total；它的 TotalTokens 只能退回原始上下文数量。
		total = 0
	}
	if wholeLog && total > 0 {
		return "总量 " + tok(total)
	}
	// total 字段缺席时保留可证明的 input/output；零值在当前 Snapshot 里表示
	// 未知，所以只画有真值的一侧，不能给另一侧补 ↓0 / ↑0。
	switch {
	case in > 0 && out > 0:
		return "↑" + tok(in) + " ↓" + tok(out)
	case in > 0:
		return "↑" + tok(in)
	case out > 0:
		return "↓" + tok(out)
	}
	return ""
}

// contextUsed 是当前上下文占用；0 = 未知（不是"占用 0"）。
//
// 旧 mockd 没有 Capacity 字段时才允许把旧 Usage.TotalTokens 当占用；正容量
// 必须搭配显式 ContextTokens，否则会把另一条旧 Usage 总量冒充成当前占用。
func contextUsed(d pluginapi.Snapshot) int64 {
	if d.ContextTokens != 0 || d.ContextWindow > 0 || d.UsageTotal.Known() {
		return d.ContextTokens
	}
	return d.Usage.TotalTokens
}

// contextSegment 分开处理占用与容量：未知容量保留原始占用，超容量/负占用明确
// 说明异常；现有组件没有 ContextMeter 圆弧能力，因此只增加真实百分比，不伪造环。
func contextSegment(used, window int64) string {
	if used < 0 {
		return "上下文 " + tok(used) + "（异常）"
	}
	if used == 0 && window <= 0 {
		return ""
	}
	raw := "上下文 " + tok(used)
	if window <= 0 {
		return raw
	}
	if used > window {
		return raw + " / " + tok(window) + "（超出容量）"
	}
	percent := contextPercent(&contextPressure{
		UsedTokens: &used, ContextWindow: &window,
	})
	if percent == "" {
		return raw
	}
	return raw + " · " + percent
}

// tok 把 token 数压成「6.5k / 1.2M」这种短写法。
func tok(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt1(float64(n)/1_000_000) + "M"
	case n >= 1_000:
		return fmt1(float64(n)/1000) + "k"
	default:
		return itoa(int(n))
	}
}

// fmt1 是一位小数格式化（不引 fmt：这一层保持极简依赖）。
func fmt1(v float64) string {
	if v < 0 {
		v = 0
	}
	whole := int64(v)
	frac := int64((v-float64(whole))*10 + 0.5)
	if frac >= 10 {
		whole++
		frac = 0
	}
	return itoa(int(whole)) + "." + itoa(int(frac))
}

// itoa 避免为一个小数字 import strconv。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [24]byte
	i := len(buf)
	for n > 0 && i > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg && i > 0 {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

var (
	_ pluginapi.Plugin     = (*Plugin)(nil)
	_ pluginapi.Panels     = (*Plugin)(nil)
	_ pluginapi.KeyHandler = (*Plugin)(nil)
)
