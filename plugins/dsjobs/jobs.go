// Package dsjobs 在侧栏底部列出**后台任务**（官方 `dsh-client-ui-jobs` 的一小块）。
//
// # 它是只读的，而且是快照
//
// `jobs.list` 的数据来自子进程推上来的一份快照（`@@JOBS@@`），不是活句柄。
// 所以：
//   - 不提供 kill（`jobs.kill` 宿主没有这个方法）
//   - 不提供点开看输出（`jobs.output` 宿主那边还是 nil，需要请求-应答通道）
//   - `Done: false` **不代表此刻还在跑**，可能刚结束还没推
//
// 画一个点不动的按钮就是在骗人，所以这一版刻意只有展示。
package dsjobs

import (
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// maxShown 最多列几条。
const maxShown = 4

type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
}

func (*Plugin) ID() string { return "ds-jobs" }

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
		ID:    "jobs-list",
		Slot:  pluginapi.SlotSidebarFooter,
		Order: 70, // dsfiles(40) → dsschedule(50) → dsdeliverables(60) → 这里
		Draw:  p.draw,
	}}
}

// draw 画「后台任务」区。**没有任务就整块不画**。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	if env.Act.JobsList == nil {
		return
	}
	jobs := env.Act.JobsList()
	if len(jobs) == 0 {
		return
	}
	t := p.theme
	shown := jobs
	more := 0
	if len(shown) > maxShown {
		more = len(shown) - maxShown
		shown = shown[:maxShown]
	}

	ui.Column(f).FillWidth().Gap(2).PadY(4).Children(func() {
		ui.Text(f, "后台任务").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		for _, j := range shown {
			// 状态点：跑着是中性色，结束且非零是错误色。
			// **不用绿点**：绿在状态栏已经表示"连着宿主"，再复用会混淆。
			dot := t.LabelTertiary
			if j.Done && j.ExitCode != 0 {
				dot = t.Error
			}
			ui.Row(f).FillWidth().Items(ui.Center).Gap(6).Children(func() {
				p.prims.Dot(f, dot, 5)
				ui.Text(f, shortCmd(j.Command)).FontSize(12).Color(t.LabelSecondary).Grow(1).Ellipsis()
				ui.Text(f, sizeLabel(j.Bytes)).FontSize(11).Color(t.LabelTertiary)
			})
		}
		if more > 0 {
			ui.Text(f, "还有 "+itoa(more)+" 条").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
	})
}

// shortCmd 把命令压成一行：取第一个 token，太长就截断。
//
// **不做 shell 解析**：`echo a; rm -rf b` 这种取第一个 token 就够表示"这条在干嘛"，
// 真解析 shell 语法会出错，而且这里只是给人看的标签。
func shortCmd(cmd string) string {
	c := strings.TrimSpace(cmd)
	if i := strings.IndexAny(c, " \t\n"); i > 0 {
		c = c[:i]
	}
	if len(c) > 24 {
		c = c[:24] + "…"
	}
	if c == "" {
		return "(空命令)"
	}
	return c
}

// sizeLabel 把字节数变成 `12 B` / `3.4 KB` / `1.2 MB`。
func sizeLabel(n int) string {
	switch {
	case n < 1024:
		return itoa(n) + " B"
	case n < 1024*1024:
		return itoa(n/1024) + " KB"
	default:
		return itoa(n/1024/1024) + " MB"
	}
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
