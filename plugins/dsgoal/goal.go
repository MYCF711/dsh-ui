// Package dsgoal 把这条会话「现在在做的事」画在 composer 上方。
//
// 官方 `dsh-client-ui-goal`：*Session goal surface: GoalBar docked above the
// composer, read from the goal session projection*。
//
// # 为什么值得单独占一行
//
// 长会话里最容易丢的信息不是"上一句说了什么"，是"**我们到底在解决什么问题**"。
// 一条始终可见的目标，比往上翻二十屏去重建上下文便宜得多。
//
// # 它是只读的
//
// 这一版不提供编辑（改 goal 走宿主 `session.setGoal`，界面上还没做入口）。
// **画一个点不动的输入框，不如老老实实显示一行字。**
package dsgoal

import (
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

type Plugin struct {
	theme *pluginapi.Theme
	// tier 是壳档位（nil = 按 official 处理，见 uikernel.ShellTier 包注释第 2 条）。
	// 两个座位靠它**二选一**：official 档挂官方座位、legacy 档走旧 main.composer。
	tier *uikernel.ShellTier
}

func (*Plugin) ID() string { return "ds-goal" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	// 可选注入：没有这个能力键（单测 / 旧壳）时按 official（官方槽是终态）。
	if v := ctx.Inject(uikernel.KeyShellTier); v != nil {
		if tier, ok := v.(*uikernel.ShellTier); ok {
			p.tier = tier
		}
	}
	return nil
}

// Panels 把目标条挂到**两个座位**，同一帧只有一个活（判据 `composerSeatLive`）：
//
//	conversation.composer  官方 chain 座位（官方 ui-goal 的 conversation.input.dock
//	                       今天不可达，见 official_seat.go 文件头）—— rc.2 终态
//	main.composer          dsshell 三栏 / 官方骨架兼容桥 —— legacy 档位的旧路径
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:   "goal-bar",
		Slot: pluginapi.SlotMainComposer,
		// **Order 5 < composer 的 10**：同一个槽里小的先画，于是它落在输入框上方。
		// 官方就是"docked above the composer"——顺序在这里就是位置。
		Order: 5,
		Draw: func(f ui.C, env *pluginapi.Env) {
			if p.composerSeatLive() {
				return // 官方座位活着：旧槽这一帧不画（不双画）
			}
			p.draw(f, env)
		},
	}, {
		ID:    "goal-bar-official",
		Slot:  uikernel.OffConvComposer,
		Order: goalDockOrder,
		Draw:  p.drawOfficial,
	}}
}

// draw 画 legacy 座位；与官方座位共用 GoalBar.module.css 的同一份几何。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	p.drawGoalBar(f, env)
}
