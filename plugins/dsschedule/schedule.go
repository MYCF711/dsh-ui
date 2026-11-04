// Package dsschedule 在侧栏底部列出**定时任务**（官方 `dsh-client-ui-schedule` 的一小块）。
//
// 它只做"看见"：标题 + 下次时间。**没有开关、没有删除**——
// 宿主那边 `schedule.*` 目前只有 list（`dshd schedule list|due|run`），
// 建/删/暂停都还没有 Transport 方法。画一个点不动的开关就是死按钮，
// 所以这一版刻意只有只读展示。
package dsschedule

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
)

// maxShown 最多列几条。侧栏底部那块地方有限，超出的写「还有 N 条」。
const maxShown = 5

// Plugin 无状态：数据每帧从 env.Act.ScheduleList() 取（壳的缓存）。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
}

func (*Plugin) ID() string { return "ds-schedule" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:    "schedule-list",
		Slot:  pluginapi.SlotSidebarFooter,
		Order: 50, // 排在 dsfiles（40）之后
		Draw:  p.draw,
	}}
}

// draw 画「定时任务」区。**没有任务就整块不画**——
// 空标题会让侧栏看着像坏了（"定时任务"下面什么都没有）。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	if env.Act.ScheduleList == nil {
		return
	}
	items := env.Act.ScheduleList()
	if len(items) == 0 {
		return
	}
	t := p.theme
	shown := items
	more := 0
	if len(shown) > maxShown {
		more = len(shown) - maxShown
		shown = shown[:maxShown]
	}

	f.Column().FillWidth().Gap(2).PadY(4).Children(func() {
		f.Text("定时任务").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		for _, it := range shown {
			title := title(it)
			f.Row().FillWidth().Items(flex.Center).Gap(6).Children(func() {
				f.Text(title).FontSize(12).Color(t.LabelSecondary).Grow(1).Ellipsis()
				// 下次时间用最暗的色：它是参考信息，不该抢任务名的注意力。
				f.Text(shortTime(it.NextAt)).FontSize(11).Color(t.LabelTertiary)
			})
		}
		if more > 0 {
			f.Text("还有 " + itoa(more) + " 条").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
	})
}

// title 取这条任务的显示名。
//
// **没起标题就退回种类**（daily/weekly/every）：显示一行空白比显示 `daily` 更糟——
// 空白让人以为数据坏了，`daily` 至少说明"这是一条每天的任务"。
//
// 抽成函数而不是留在 draw 里，是因为它是这块**唯一能单测的判断**
// （其余都是绘制）。抽出来之后 `TestTitleFallsBackToKind` 才能钉住它。
func title(it pluginapi.ScheduleItem) string {
	if it.Title != "" {
		return it.Title
	}
	if it.Kind != "" {
		return it.Kind
	}
	return "(未命名)"
}

// shortTime 把 `2026-11-05T09:00:00+08:00` 缩成 `11-05 09:00`。
//
// **不做时区换算**：宿主给的已经是带偏移的绝对时刻，这里只是把它变短，
// 换成本机时区反而会让"我设的 9 点"变成别的数字。格式不对就原样返回。
func shortTime(s string) string {
	if len(s) < 16 || s[10] != 'T' {
		return s
	}
	return s[5:10] + " " + s[11:16]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
