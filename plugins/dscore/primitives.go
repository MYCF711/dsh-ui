package dscore

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Primitives 是基础组件的画法：只吃令牌，不自己做设计决定。
//
// 规矩（下游插件必须守）：**不要在插件里写十六进制色号**，一律从这里或
// Theme 取。写死色号的那一刻，换主题就漏了这一块。
type Primitives struct {
	Theme *pluginapi.Theme
}

// ListRow 画一条列表行，返回盒子给调用方接 .Clicked()。
//
// 几何照官方**会话行**（`ui-workspace/src/client/rows/Rows.module.css`）：
//
//	.wnFoaW_sessionRow{gap:0;height:32px}
//	.wnFoaW_projectRow,.wnFoaW_sessionRow{border-radius:var(--dsw-radius-md);
//	  padding:0 8px;…;padding-inline-start:calc(8px + var(--dsh-workspace-indent,0px))}
//	.wnFoaW_projectRow:hover,.wnFoaW_sessionRow:hover,
//	.wnFoaW_sessionRow.wnFoaW_selected{background:var(--dsw-alias-interactive-bg-hover)}
//
// 注意两点，和"想当然"不一样：
//   - 行高是**固定 32**，不是最小高；字号 14/20（标题那层给）。
//   - **选中态和悬停态同色**，都是 `--dsw-alias-interactive-bg-hover`。
//     我们原来的 NavActive 是自造的深一档色，官方没有这个东西。
//
// height 由调用方给（官方会话行 32、项目行 34），这里只管形状。
func (p *Primitives) ListRow(f *flex.Frame, id gfx.ID, active bool, height float32, body func()) ui.Box {
	t := p.Theme
	row := f.Row().FillWidth().Height(height).Radius(t.Radii.MD).
		PadX(8).PadY(0).Items(ui.Center).Gap(0)
	if active {
		row = row.Background(t.Hover)
	} else {
		row = row.Hover(ui.Look{}.Background(t.Hover))
	}
	return row.Hit(id, "").Children(body)
}

// CircleButton 画一个 28px 的圆形图标按钮（官方 .action 就是 28×28、radius 999）。
//
// 用文字字形（+ ↑ ×）而不是矢量图标：viva 的 Icon 是分层路径，为三个字形引一套
// 图标构造不划算；这个尺寸下字形和图标看不出差别。
func (p *Primitives) CircleButton(f *flex.Frame, id gfx.ID, glyph string, filled bool) bool {
	t := p.Theme
	bg, fg := t.BgLayer3, t.LabelSecondary
	if filled {
		bg, fg = t.ButtonPrimaryFill, t.ButtonPrimaryFg
	}
	return f.Row().Width(28).Height(28).Radius(14).Background(bg).
		Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).Hit(id, "").Children(func() {
		f.Text(glyph).FontSize(15).Color(fg)
	}).Clicked()
}

// Chip 一个只读小标签。官方 .chip：max-width 240、gap 3px、business-primary 文字。
func (p *Primitives) Chip(f *flex.Frame, label string) ui.Box {
	t := p.Theme
	return f.Row().Height(20).Radius(t.Radii.XS).PadX(6).Items(ui.Center).
		Background(t.BgLayer2).Children(func() {
		f.Text(label).FontSize(t.Fonts.Tiny).Color(t.Info)
	})
}

// Bubble 一个用户气泡容器。官方 .bubble：radius xl、specific-bubble 底、
// font-size/line-height 跟正文令牌走。
func (p *Primitives) Bubble(f *flex.Frame, body func()) ui.Box {
	t := p.Theme
	return f.Column().Pad(10).Radius(t.Radii.XL).Background(t.Bubble).Children(body)
}

// SectionHeader 画一个分组头（工作区名 + 条数 + 折叠箭头）。
//
// 照官方**项目行** `.wnFoaW_projectRow`（它就是侧栏里那一行可折叠的工作区名）：
//
//	.wnFoaW_projectRow{box-sizing:border-box;align-items:center;height:34px}
//	.wnFoaW_projectRow,.wnFoaW_sessionRow{border-radius:var(--dsw-radius-md);
//	  padding:0 8px;gap:6px;color:var(--dsw-alias-label-primary);…}
//	.wnFoaW_title{font-size:14px;line-height:20px}
//	.wnFoaW_chevron{color:var(--dsw-alias-label-caption)}
//	.wnFoaW_arrow{transition:transform .15s var(--ds-ease-in-out)}
//	.wnFoaW_arrowOpen{transform:rotate(90deg)}
//
// 所以它比会话行**高 2px（34 vs 32）**、字是正文色而不是灰字——
// 原来那版 26px 的灰字按钮和官方不是一个东西。
func (p *Primitives) SectionHeader(f *flex.Frame, id gfx.ID, title string, count int, open bool) ui.Box {
	t := p.Theme
	arrow := "▸"
	if open {
		arrow = "▾"
	}
	return f.Row().FillWidth().Height(34).Radius(t.Radii.MD).PadX(8).
		Items(ui.Center).Gap(6).Hover(ui.Look{}.Background(t.Hover)).Hit(id, "").Children(func() {
		f.Text(arrow).FontSize(t.Fonts.Caption).Color(t.LabelCaption)
		f.Text(title).FontSize(t.Fonts.Body).Color(t.LabelPrimary).Ellipsis()
		f.Spacer()
		if count > 0 {
			// 官方项目行没有计数；计数用的字形取自 `.wnFoaW_meta`
			// （12px/20、--dsw-alias-label-tertiary）。
			f.Text(itoa(count)).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
	})
}

// Badge 是行尾的小徽标（状态、条数、错误）。
func (p *Primitives) Badge(f *flex.Frame, text string, color ui.Color) ui.Box {
	t := p.Theme
	return f.Row().Height(18).Radius(t.Radii.XS).PadX(5).Items(ui.Center).
		Background(t.BgLayer3).Children(func() {
		f.Text(text).FontSize(t.Fonts.Tiny).Color(color)
	})
}

// Dot 是一个状态圆点（未读、运行中、错误）。
func (p *Primitives) Dot(f *flex.Frame, color ui.Color, size float32) ui.Box {
	return f.Row().Width(size).Height(size).Radius(gfx.RadiusFull).Background(color)
}

// Divider 是一条最轻的分隔线（官方 border-l1）。
func (p *Primitives) Divider(f *flex.Frame) {
	t := p.Theme
	f.Row().FillWidth().Height(1).Background(t.BorderL1)
}

// Hairline 是一条 0.5px 的分隔线（官方到处用的 `.5px solid var(--dsw-alias-border-l2)`）。
//
// 和 Divider 的区别只在粗细和颜色层级：Divider 是 1px / border-l1，给栏内分块用；
// Hairline 是官方底板（底部整行、菜单 footer）那一档，给"整条边"用。
func (p *Primitives) Hairline(f *flex.Frame) {
	t := p.Theme
	f.Row().FillWidth().Height(0.5).Background(t.BorderL2)
}

// StatusBarRow 画底部整行的壳：一条 0.5px 的分隔线 + 一行内容。
//
// 官方对应物是 AppFrame 的 bottomRow（它是**整行**，不是某一列的一部分）：
//
//	.mP3x-q_bottomRow{background:var(--dsw-alias-bg-base);grid-column:1/-1;
//	  min-width:0;min-height:0;overflow:auto}
//
// 官方没有给这一行写死高度、padding 和字号（slot 文档：「Its occupant controls
// its own height and visibility」），所以高度由壳的 Geometry 给；
// 内容的字号/间隔取官方统计胶囊 StatsPills 的那一档：
//
//	.hNGZaa_anchor{font-size:calc(var(--dsh-content-font-size-secondary,13px) - 1px);
//	  line-height:calc(20px + var(--dsh-content-font-delta-secondary,0px))}
//	.hNGZaa_pill{color:var(--dsw-alias-label-tertiary);
//	  font-variant-numeric:tabular-nums;align-items:center;gap:6px;padding:1px 8px}
//	.hNGZaa_sep{color:var(--dsw-alias-separator-primary);margin:0 6px}
//
// 左边距 12 取 `--dsh-sidebar-inline-padding`，这样底部条的文字和侧栏文字左对齐，
// 而不是从窗口中间（中栏）开始。
func (p *Primitives) StatusBarRow(f *flex.Frame, height float32, body func()) ui.Box {
	t := p.Theme
	return f.Column().FillWidth().Height(height).Background(t.BgBase).Children(func() {
		p.Hairline(f)
		f.Row().FillWidth().FillHeight().PadX(12).Items(ui.Center).Gap(6).Children(body)
	})
}

// itoa 避免为了一个数字去 import strconv（这一层要保持极简依赖）。
//
// 两处刻意写宽：
//   - 缓冲区按 int64 的最多位数（19 + 符号）给，原来 [8]byte 在 ≥10^8 时
//     会**静默截掉高位**（`itoa(100000000)` 返回 "00000000"，不报错）；
//   - 负数正常带符号，原来 `for n > 0` 直接不进循环，返回空串。
//
// 这两个边界现在都够不到（计数都先被 `> 0` 挡住），但"够不到"不是"不会变"——
// 一个不报错但返回错数字的辅助函数，比一个会 panic 的更难查。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	var buf [21]byte
	i := len(buf)
	// 用无符号位运算取绝对值：MinInt64 取负会溢出，按位处理不会。
	u := uint64(n)
	if neg {
		u = -u
	}
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// 编译期断言：dscore 必须满足契约里的 Primitives 接口。
var _ pluginapi.Primitives = (*Primitives)(nil)
