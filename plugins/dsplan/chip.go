package dsplan

import (
	"sync"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// 官方 PlanModeControl.module.css:3-24,78-82：28px 高、0/8 padding、
// radius-sm、13/20 字体；错误行 12/18。business-primary 在本主题就是 Info；
// tertiary 没有独立令牌，沿用同档 BgLayer3（与 Hero 预览徽标同一映射）。
const (
	planChipWrapGap float32 = 6
	planChipHeight  float32 = 28
	planChipGap     float32 = 4
	planChipPadX    float32 = 8
	planChipFont    float32 = 13
	planChipLine    float32 = 20
	planChipErrFont float32 = 12
	planChipErrLine float32 = 18
)

type planChipState struct {
	mu      sync.Mutex
	leaving bool
	err     string
}

// drawPlanChip 是官方 conversation.input.plan 的 PlanChip。它只消费宿主
// session.projections.plan，不从工具调用猜当前状态；点击只执行真实 /plan off，
// 等下一份宿主投影让 chip 消失，不在客户端乐观改 active。
func (p *Plugin) drawPlanChip(f ui.C, env *pluginapi.Env) {
	if env == nil || p.theme == nil || env.Data.Plan == nil || env.Act.ExecuteCommand == nil {
		return
	}
	projection := env.Data.Plan
	target := projection.Active
	if projection.Pending {
		target = !target
	}
	if !target {
		return
	}
	leaving, failure := p.chipSnapshot()
	locked := env.Data.Busy || leaving
	t := p.theme
	ui.Row(f).Items(ui.Center).Gap(planChipWrapGap).Children(func() {
		chip := ui.Row(f).Height(planChipHeight).Items(ui.Center).Gap(planChipGap).
			PadSides(0, planChipPadX, 0, planChipPadX).Radius(t.Radii.SM).
			Background(t.BgLayer3).
			Hit(env.Tree.ID(env.Feat, "plan-mode-off"), zhChipTitle)
		if locked {
			chip = chip.Opacity(0.6)
		} else if chip.Clicked() {
			p.exitPlanMode(env)
		}
		chip.Children(func() {
			ui.Text(f, zhChipLabel).FontSize(planChipFont).LineHeight(planChipLine).
				Weight(500).Color(t.Info)
		})
		if failure != "" {
			ui.Text(f, zhChipExitFailed).FontSize(planChipErrFont).LineHeight(planChipErrLine).
				Color(t.Error)
		}
	})
}

func (p *Plugin) chipSnapshot() (bool, string) {
	p.chip.mu.Lock()
	defer p.chip.mu.Unlock()
	return p.chip.leaving, p.chip.err
}

func (p *Plugin) exitPlanMode(env *pluginapi.Env) {
	p.chip.mu.Lock()
	if p.chip.leaving {
		p.chip.mu.Unlock()
		return
	}
	p.chip.leaving = true
	p.chip.err = ""
	p.chip.mu.Unlock()
	execute := env.Act.ExecuteCommand
	invalidate := env.Act.Invalidate
	go func() {
		err := execute("/plan off")
		p.chip.mu.Lock()
		p.chip.leaving = false
		if err != nil {
			p.chip.err = err.Error()
		}
		p.chip.mu.Unlock()
		if invalidate != nil {
			invalidate()
		}
	}()
}
