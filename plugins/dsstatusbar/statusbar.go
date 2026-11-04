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
//   - ContextTokens 只显示原始 token 数，缺 contextWindow 时不显示百分比或环。
package dsstatusbar

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
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
	return nil
}

// Panels 把统计与连接状态拆到各自应在的位置。Order20 保证 dock 跟在
// ds-composer 的卡片(order10) 后；这不是对官方 conversation.composer.dock
// 槽契约的冒名声明，而是现有 main.composer 槽内的等价布局集成。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "dock", Slot: pluginapi.SlotMainComposer, Order: 20, Draw: func(f *flex.Frame, env *pluginapi.Env) {
			p.drawDock(f, env)
		}},
		{ID: "connection", Slot: pluginapi.SlotShellBottom, Order: 10, Draw: p.draw},
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

// connectionTone 是连接状态的颜色语义，不与 ContextMeter 混用。
type connectionTone uint8

const (
	connectionReady connectionTone = iota
	connectionApproval
	connectionDisconnected
)

// connectionState 保持断线 > 待审批 > 就绪的既有真值优先级。
func connectionState(data pluginapi.Snapshot) (string, connectionTone) {
	switch {
	case data.Disconnected:
		return "已断开", connectionDisconnected
	case data.Approval != nil:
		return "待审批", connectionApproval
	default:
		return "就绪", connectionReady
	}
}

// bar 是 dscore 那个底部整行图元的可选能力，只服务连接状态。
type bar interface {
	StatusBarRow(f *flex.Frame, height float32, body func()) flex.Box
}

// draw 画独立连接状态行。统计指标已经移到 composer 后，不再占这条全宽行的左侧。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	label, tone := connectionState(env.Data)
	dotColor := t.Success
	switch tone {
	case connectionDisconnected:
		dotColor = t.Error
	case connectionApproval:
		dotColor = t.Warn
	}

	h := env.Geometry.BottomHeight
	if h <= 0 {
		h = 24
	}
	p.bar(f, h, func() {
		f.Spacer()
		if env.Data.Status != "" {
			f.Text(env.Data.Status).FontSize(t.Fonts.Caption).
				Color(t.LabelTertiary).Ellipsis().MaxWidth(240)
		}
		f.Row().Gap(6).Items(flex.Center).Shrink(0).Children(func() {
			p.prims.Dot(f, dotColor, 6)
			f.Text(label).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		})
	})
}

// usageSegments 决定 dock 当前能显示的真值段，返回空 = 整段不占位。
//
// UsageTotal 是真宿主 whole-log 累计，totalTokens 由宿主按 input+output 合计；
// 旧 mockd Usage.TotalTokens 则是最后一次 assistant 的上下文占用，两者不能互换。
// ContextTokens 与累计相互独立，压缩后允许回落；缺 contextWindow 时只画原始数量。
func usageSegments(d pluginapi.Snapshot) []string {
	in, out := d.UsageTotal.InputTokens, d.UsageTotal.OutputTokens
	total := d.UsageTotal.TotalTokens
	ctx := d.ContextTokens
	wholeLog := d.UsageTotal.Known()
	if !wholeLog {
		in, out = d.Usage.InputTokens, d.Usage.OutputTokens
		// 旧 Usage 没有 whole-log total；它的 TotalTokens 只能退回原始上下文数量。
		total = 0
		if ctx == 0 {
			ctx = d.Usage.TotalTokens
		}
	}

	var segments []string
	if wholeLog && total > 0 {
		segments = append(segments, "总量 "+tok(total))
	} else {
		// total 字段缺席时保留可证明的 input/output；零值在当前 Snapshot 里表示
		// 未知，所以只画有真值的一侧，不能给另一侧补 ↓0 / ↑0。
		switch {
		case in > 0 && out > 0:
			segments = append(segments, "↑"+tok(in)+" ↓"+tok(out))
		case in > 0:
			segments = append(segments, "↑"+tok(in))
		case out > 0:
			segments = append(segments, "↓"+tok(out))
		}
	}
	if ctx > 0 {
		segments = append(segments, "上下文 "+tok(ctx))
	}
	return segments
}

// bar 调 dscore 的整行图元；拿不到就自己画一条等价的连接状态行。
func (p *Plugin) bar(f *flex.Frame, height float32, body func()) {
	if sb, ok := p.prims.(bar); ok {
		sb.StatusBarRow(f, height, body)
		return
	}
	t := p.theme
	f.Column().FillWidth().Height(height).Background(t.BgBase).Children(func() {
		f.Row().FillWidth().Height(0.5).Background(t.BorderL2)
		f.Row().FillWidth().FillHeight().PadX(12).Items(flex.Center).Gap(6).Children(body)
	})
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

// rate 是官方 decodeTokens/decodeMs 速率的显示格式。
func rate(v float64) string { return fmt1(v) + " tok/s" }

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
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)
