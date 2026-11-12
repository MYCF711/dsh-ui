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

// draw 画目标条。**没有目标就整块不画**——空白条会占掉一行高度却不说话。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	if env.Act.Goal == nil {
		return
	}
	goal := env.Act.Goal()
	if goal == "" {
		return
	}
	t := p.theme
	// ⚠️ `PadSides(top, right, bottom, left)`——不是"横、竖"。
	//
	// 这里原来写的是 `PadSides(16, 16, 6, 6)`，看着像"左右 16、上下 6"，
	// 按真实签名读出来是**上 16、右 16、下 6、左 6**：上边被撑开一倍，
	// 左边只剩 6，于是这一行的文字比输入框里的文字更贴边、也更靠下。
	//
	// 判据不是"我觉得好看"，是全仓的写法：其它每一条 `PadSides` 都是
	// `PadSides(10,16,10,16)` / `PadSides(14,16,14,16)` / `PadSides(14,14,12,14)`
	// 这种**四个数两两对称**的形状。这一条是唯一一条两个方向都不对称的，
	// 而它出现在一个只画一行字的部件里——那不像设计，像把顺序写串了。
	ui.Row(f).FillWidth().Items(ui.Center).Gap(8).PadSides(6, 16, 6, 16).Children(func() {
		// 官方 `.goalGlyph`：sprite `goal-outline`（viewBox 16）按 size 14 画，
		// 左边中性色 `label-tertiary`。不用绿（绿在状态栏已经表示"连着宿主"，
		// 复用会混淆），也不用圆点冒充图标（见 icons.go 的取证）。
		ui.Icon(f, goalOutlineIcon()).Size(goalGlyphSize, goalGlyphSize).Color(t.LabelTertiary)
		ui.Text(f, "目标").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		// Ellipsis：目标可能是一句话，但这一行只给一眼的高度。
		// 完整内容该在别处看——这里的作用是"别忘了我",不是"读完我"。
		ui.Text(f, goal).FontSize(t.Fonts.Caption).Color(t.LabelSecondary).Grow(1).Ellipsis()
	})
}
