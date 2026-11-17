package dscore

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
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
//	Rows.module.css:1-16,123-126：height:32px;gap:0；
//	  border-inline:3px solid transparent；padding:0 8px；
//	  padding-inline-start:calc(12px - 3px + indent)。
//	Rows.module.css:18-26,317-325：hover/selected 都走 interactive-bg-hover。
//
// 3px 透明 border 用等效 margin 表达；左侧 9px 是官方 12-3，右侧仍是 8px。
// height 由调用方给（会话行/项目行均 32），这里只管形状。
func (p *Primitives) ListRow(f ui.C, id gfx.ID, active bool, height float32, body func()) ui.Box {
	t := p.Theme
	row := ui.Row(f).FillWidth().Height(height).Radius(t.Radii.MD).
		MarginSides(0, 3, 0, 3).PadSides(0, 8, 0, 9).Items(ui.Center).Gap(0)
	if active {
		row = row.Background(t.Hover)
	} else {
		row = row.Hover(ui.Look{}.Background(t.Hover))
	}
	return row.Hit(id, "").Children(body)
}

// CircleButton 画一个 28px 的圆形图标按钮（官方 .action：QueueDock.module.css:219-233，
// 28×28、radius 999px）。
//
// 是用文字字形（+ ↑ ×）的**退路**：viva 的 Icon 是分层路径，字形和图标在这个尺寸下
// 看不出差别，所以只在没有对应官方矢量素材时用；有素材请走 CircleIconButton。
func (p *Primitives) CircleButton(f ui.C, id gfx.ID, glyph string, filled bool) bool {
	t := p.Theme
	bg, fg := t.BgLayer3, t.LabelSecondary
	if filled {
		bg, fg = t.ButtonPrimaryFill, t.ButtonPrimaryFg
	}
	return ui.Row(f).Width(28).Height(28).Radius(14).Background(bg).
		Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).Hit(id, "").Children(func() {
		ui.Text(f, glyph).FontSize(15).Color(fg)
	}).Clicked()
}

// CircleIconButton 与 CircleButton 同一套几何，图标换成官方矢量素材。
//
// 官方图标键的实际画法就是 `<svg width=15>`：MessageFeedbackActions.module.css:20-23
// 的 `.action svg { width: 15px; height: 15px }`；静止态颜色取官方 `.action` 的
// label-tertiary（QueueDock.module.css:231）。hover 只换底色（同处 :229-233 的
// `background: var(--dsw-alias-interactive-bg-hover)`），图标色不变。
func (p *Primitives) CircleIconButton(f ui.C, id gfx.ID, icon gfx.Icon, filled bool) bool {
	t := p.Theme
	bg, fg := t.BgLayer3, t.LabelTertiary
	if filled {
		bg, fg = t.ButtonPrimaryFill, t.ButtonPrimaryFg
	}
	return ui.Row(f).Width(28).Height(28).Radius(14).Background(bg).
		Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).Hit(id, "").Children(func() {
		ui.Icon(f, icon).Size(15, 15).Color(fg)
	}).Clicked()
}

// Chip 一个只读小标签。官方 .chip：max-width 240、gap 3px、business-primary 文字。
func (p *Primitives) Chip(f ui.C, label string) ui.Box {
	t := p.Theme
	return ui.Row(f).Height(20).Radius(t.Radii.XS).PadX(6).Items(ui.Center).
		Background(t.BgLayer2).Children(func() {
		ui.Text(f, label).FontSize(t.Fonts.Tiny).Color(t.Info)
	})
}

// Bubble 一个用户气泡容器。官方 UserMessageBubble.module.css:1-11：
// padding:10px 16px、radius xl、specific-bubble 底、14px/22px；
// 文字行高由调用方正文令牌给。
func (p *Primitives) Bubble(f ui.C, body func()) ui.Box {
	t := p.Theme
	return ui.Column(f).PadSides(10, 16, 10, 16).Radius(t.Radii.XL).Background(t.Bubble).Children(body)
}

// SectionHeader 画官方项目行：16×20 文件夹槽 + 14/20 标题。
// Rows.module.css:1-16,112-116,132-140,173-188；Rows.tsx:314-322。
// 官方没有会话计数，count 仅为保持 pluginapi.Primitives 契约兼容而保留。
// 静止态画官方 folder-open/close；hover 换 triangle 的动效尚未由 Viva 图元提供。
func (p *Primitives) SectionHeader(f ui.C, id gfx.ID, title string, count int, open bool) ui.Box {
	t := p.Theme
	return ui.Row(f).FillWidth().Height(32).Radius(t.Radii.MD).
		MarginSides(0, 3, 0, 3).PadSides(0, 8, 0, 9).
		Items(ui.Center).Gap(6).Hover(ui.Look{}.Background(t.Hover)).Hit(id, "").Children(func() {
		ui.Row(f).Width(16).Height(20).Shrink(0).Items(ui.Center).Justify(ui.Center).Children(func() {
			if icon, ok := dsassets.WorkspaceFolder(t.LabelTertiary, open); ok {
				ui.Icon(f, icon).Size(16, 16).Color(t.LabelTertiary)
			}
		})
		ui.Text(f, title).FontSize(t.Fonts.Body).LineHeight(20).Color(t.LabelPrimary).Ellipsis().Grow(1)
	})
}

// Badge 是行尾的小徽标（状态、条数、错误）。
func (p *Primitives) Badge(f ui.C, text string, color ui.Color) ui.Box {
	t := p.Theme
	return ui.Row(f).Height(18).Radius(t.Radii.XS).PadX(5).Items(ui.Center).
		Background(t.BgLayer3).Children(func() {
		ui.Text(f, text).FontSize(t.Fonts.Tiny).Color(color)
	})
}

// Dot 是一个状态圆点（未读、运行中、错误）。
func (p *Primitives) Dot(f ui.C, color ui.Color, size float32) ui.Box {
	return ui.Row(f).Width(size).Height(size).Radius(gfx.RadiusFull).Background(color)
}

// Divider 是一条最轻的分隔线（官方 border-l1）。
func (p *Primitives) Divider(f ui.C) {
	t := p.Theme
	ui.Row(f).FillWidth().Height(1).Background(t.BorderL1)
}

// Hairline 是一条 0.5px 的分隔线（官方到处用的 `.5px solid var(--dsw-alias-border-l2)`）。
//
// 和 Divider 的区别只在粗细和颜色层级：Divider 是 1px / border-l1，给栏内分块用；
// Hairline 是官方底板（底部整行、菜单 footer）那一档，给"整条边"用。
func (p *Primitives) Hairline(f ui.C) {
	t := p.Theme
	ui.Row(f).FillWidth().Height(0.5).Background(t.BorderL2)
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
func (p *Primitives) StatusBarRow(f ui.C, height float32, body func()) ui.Box {
	t := p.Theme
	return ui.Column(f).FillWidth().Height(height).Background(t.BgBase).Children(func() {
		p.Hairline(f)
		ui.Row(f).FillWidth().FillHeight().PadX(12).Items(ui.Center).Gap(6).Children(body)
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
