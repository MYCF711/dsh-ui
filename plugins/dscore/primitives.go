package dscore

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
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
// 几何照官方：圆角 MD(12)、最小高 36、横向内边距 10、行间距 10
// （`dsh-client-ui-chat` 里的 .row 规则）。选中态用 NavActive，悬停用 NavHover。
func (p *Primitives) ListRow(f *flex.Frame, id ui.ID, active bool, height float32, body func()) flex.Box {
	t := p.Theme
	row := f.Row().FillWidth().MinHeight(height).Radius(t.Radii.MD).
		PadX(10).PadY(6).Items(flex.Center).Gap(8)
	if active {
		row = row.Background(t.NavActive)
	} else {
		row = row.HoverBackground(t.NavHover)
	}
	return row.Hit(id, "").Children(body)
}

// SectionHeader 画一个分组头（工作区名 + 条数 + 折叠箭头）。
//
// 官方侧栏的分组头是「小号灰字 + 右边数字」，不是按钮样式；这里保持同样的克制：
// 12px、第三级文字、没有底色，只有悬停时轻微变色。
func (p *Primitives) SectionHeader(f *flex.Frame, id ui.ID, title string, count int, open bool) flex.Box {
	t := p.Theme
	arrow := "▸"
	if open {
		arrow = "▾"
	}
	return f.Row().FillWidth().Height(26).Radius(t.Radii.SM).PadX(6).
		Items(flex.Center).Gap(6).HoverBackground(t.Hover).Hit(id, "").Children(func() {
		f.Text(arrow).FontSize(t.Fonts.Tiny).Color(t.LabelTertiary)
		f.Text(title).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Ellipsis()
		f.Spacer()
		if count > 0 {
			f.Text(itoa(count)).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		}
	})
}

// Badge 是行尾的小徽标（状态、条数、错误）。
func (p *Primitives) Badge(f *flex.Frame, text string, color ui.Color) flex.Box {
	t := p.Theme
	return f.Row().Height(18).Radius(t.Radii.XS).PadX(5).Items(flex.Center).
		Background(t.BgLayer3).Children(func() {
		f.Text(text).FontSize(t.Fonts.Tiny).Color(color)
	})
}

// Dot 是一个状态圆点（未读、运行中、错误）。
func (p *Primitives) Dot(f *flex.Frame, color ui.Color, size float32) flex.Box {
	return f.Row().Width(size).Height(size).Radius(ui.RadiusFull).Background(color)
}

// Divider 是一条最轻的分隔线（官方 border-l1）。
func (p *Primitives) Divider(f *flex.Frame) {
	t := p.Theme
	f.Row().FillWidth().Height(1).Background(t.BorderL1)
}

// itoa 避免为了一个数字去 import strconv（这一层要保持极简依赖）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 && i > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
