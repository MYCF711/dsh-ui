// Package dsstatusbar 是「底部状态栏」的移植。
//
// 官方对应物是 `dsh-client-ui-layout` 里那一行状态条（右侧显示会话名 / 轮次 /
// token 速率）。我们放上去的都是**真算得出来**的东西：
//
//	轮数 / 步数    数 env.Data.Msgs 里的 assistant / tool 条数
//	累计 ↑ ↓      宿主 §12 H1 的 usageTotal（全会话累计，不随 tail 切页变化）
//	tok/s          宿主只给用量不给耗时，用事件墙钟时间的差算（见 mockd/realsession.go）
//	上下文         宿主 §12 H1 的 contextTokens（最近一条 assistant 的 totalTokens）
//
// 累计与上下文是**两件事**：累计一直涨，上下文会随压缩回落。不相加、不互相顶替。
//
// 明确的克制：所有用量字段的零值都表示**不知道**——`Usage.Known()` 为假、
// `ContextTokens == 0`、`UsageTotal` 全零时**一个字都不显示**。
// 显示「↑0 ↓0」「上下文 0」比不显示更糟：用户会以为模型真的没消耗。
package dsstatusbar

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
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

// Panels 实现 pluginapi.Panels。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "bar", Slot: pluginapi.SlotShellBottom, Order: 10, Draw: p.draw},
	}
}

// bar 是 dscore 那个「底部整行」图元的可选能力。
//
// 为什么不写进 pluginapi.Primitives：那份契约是跨线共用的（`pluginapi/plugin.go`），
// 改它要两边一起动；而这里只是"dscore 正好多提供了一个画法"。
// 类型断言拿得到就用，拿不到就退回自己画——插件之间只认契约，不认具体类型。
type bar interface {
	StatusBarRow(f *flex.Frame, height float32, body func()) flex.Box
}

// draw 画一整条状态栏。
//
// 宽度分配：左边的数字段全部 Shrink(0)（不许被压缩，否则最后一个字会被切掉——
// 实测 "571.3k" 显示成 "571.3"），右边只留一段可省略的 Status，
// 空间不够时压缩的永远是它。
//
// 整行的壳（背景、0.5px 分隔线、左内边距）交给 prims，这里只排内容：
// 官方 bottomRow 是横跨三列的整行，内容从 `--dsh-sidebar-inline-padding`(12)
// 开始，不是从窗口中间开始。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme

	turns, steps := 0, 0
	for i := range env.Data.Msgs {
		switch env.Data.Msgs[i].Role {
		case "assistant":
			turns++
		case "tool":
			steps++
		}
	}

	// 状态点：待审批时是警告色，否则就绪。
	dotColor, dotLabel := t.Success, "就绪"
	if env.Data.Approval != nil {
		dotColor, dotLabel = t.Warn, "待审批"
	}

	h := env.Geometry.BottomHeight
	if h <= 0 {
		h = 24
	}

	p.bar(f, h, func() {
		if turns+steps > 0 {
			p.num(f, itoa(turns)+" 轮 · "+itoa(steps)+" 步", t.LabelTertiary)
		}
		if segs := usageSegments(env.Data); len(segs) > 0 {
			p.sep(f)
			for i, s := range segs {
				if i > 0 {
					p.sep(f)
				}
				p.num(f, s, t.LabelTertiary)
			}
		}
		f.Spacer()
		if env.Data.Status != "" {
			// 唯一可压缩的一段：太长就省略，状态栏不该抢主区的注意力。
			f.Text(env.Data.Status).FontSize(t.Fonts.Caption).
				Color(t.LabelTertiary).Ellipsis().MaxWidth(240)
		}
		f.Row().Gap(6).Items(flex.Center).Shrink(0).Children(func() {
			p.prims.Dot(f, dotColor, 6)
			f.Text(dotLabel).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		})
	})
}

// usageSegments 决定「用量那一段」画哪几块文字，返回按顺序要画的段
// （不含中间的 · 分隔点）。返回空 = 整段不画。
//
// 数据有两条来源，语义**不同**，不许混用：
//
//	真宿主 §12 H1   UsageTotal（全会话累计，一直涨）+ ContextTokens（当前占用，压缩后回落）
//	mockd           Usage（旧的一份合计；它的 TotalTokens 是最后一次 assistant 的 totalTokens）
//
// 规则三条：
//
//	① 累计 ↑↓ 与上下文各自独立判断，谁有值画谁——**不相加，也不拿一个顶替另一个**；
//	② ContextTokens 为 0 表示"不知道"，不是"上下文里有 0 个 token"，所以不画；
//	③ UsageTotal 全零时才退回旧 Usage（兼容 mockd）；两者都没有就整段不画。
func usageSegments(d pluginapi.Snapshot) []string {
	in, out := d.UsageTotal.InputTokens, d.UsageTotal.OutputTokens
	perSec, hasRate := d.UsageTotal.TokPerSec, d.UsageTotal.HasRate
	ctx := d.ContextTokens
	if !d.UsageTotal.Known() {
		in, out = d.Usage.InputTokens, d.Usage.OutputTokens
		perSec, hasRate = d.Usage.TokPerSec, d.Usage.HasRate
		// 旧路径里 TotalTokens 就是"最后一次 usage 的 totalTokens"，
		// 语义正好是上下文占用，可以退过去；真宿主的 UsageTotal 则**不是**。
		if ctx == 0 {
			ctx = d.Usage.TotalTokens
		}
	}
	var segs []string
	// 只有 input/output 任一有值才画 ↑↓：旧路径可能只给了 TotalTokens，
	// 那时画 "↑0 ↓0" 就是编数字。
	if in > 0 || out > 0 {
		segs = append(segs, "↑"+tok(in)+" ↓"+tok(out))
		if hasRate && perSec > 0 {
			segs = append(segs, rate(perSec))
		}
	}
	if ctx > 0 {
		segs = append(segs, "上下文 "+tok(ctx))
	}
	return segs
}

// bar 调 dscore 的整行图元；拿不到就自己画一条等价的（保证插件仍能单独跑）。
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

// num 画一个不可压缩的数字段。
func (p *Plugin) num(f *flex.Frame, s string, c ui.Color) {
	f.Text(s).FontSize(p.theme.Fonts.Caption).Color(c).PadX(2).Shrink(0)
}

// sep 画一个间隔点。左右各 6px 照官方 `.hNGZaa_sep{margin:0 6px}`。
//
// 官方那个 `--dsw-alias-separator-primary` 在打包产物里只有引用、没有定义
// （app.asar 里 grep 不到它的赋值），所以退到现有的最弱一档文字色。
func (p *Plugin) sep(f *flex.Frame) {
	f.Text("·").FontSize(p.theme.Fonts.Caption).Color(p.theme.LabelDimmed).PadX(6).Shrink(0)
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

// rate 是 token 速率，一位小数。
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
	var buf [12]byte
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
