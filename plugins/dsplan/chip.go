package dsplan

import (
	"sync"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 PlanModeControl.module.css:3-24,46-55,78-82：28px 高、min-width:34px、
// 0/8 padding、14px glyph、radius-sm、13/20 字体；错误行 12/18。
// business-primary=Info；tertiary 使用同名主题令牌，不再借 BgLayer3。
const (
	planChipWrapGap  float32 = 6
	planChipHeight   float32 = 28
	planChipMinWidth float32 = 34
	planChipGap      float32 = 4
	planChipPadX     float32 = 8
	planChipGlyph    float32 = 14
	planChipFont     float32 = 13
	planChipLine     float32 = 20
	planChipErrFont  float32 = 12
	planChipErrLine  float32 = 18
	planChipHoverMix float32 = 0.06
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
		chip := ui.Row(f).Height(planChipHeight).MinWidth(planChipMinWidth).Items(ui.Center).Gap(planChipGap).
			PadSides(0, planChipPadX, 0, planChipPadX).Radius(t.Radii.SM).
			Background(t.BusinessTertiary).
			DrawOver(func(d *ui.DrawContext) { drawPlanFocusRing(d, planChipFocusOffset, t.Radii.SM, t.Info) }).
			// 无障碍名用官方 `aria-label`（chip.on.aria，locales.ts:18）：
			// 官方那个 button 的 aria-label 与 title 是两条不同文案
			// （PlanModeControl.tsx:56-57），Hit 承载的是前者。
			Hit(env.Tree.ID(env.Feat, "plan-mode-off"), zhChipAria)
		if locked {
			chip = chip.Opacity(0.6)
		} else {
			chip = chip.Hover(ui.Look{}.Background(mixPlanColor(t.BusinessTertiary, t.Info, planChipHoverMix)))
			if chip.Clicked() {
				p.exitPlanMode(env)
			}
		}
		chip.Children(func() {
			state := chip.State()
			if !locked && (state.Hovered || state.FocusVisible) {
				ui.Icon(f, dsassets.PlanModeCloseCircle(t.Info)).Width(planChipGlyph).Height(planChipGlyph)
			} else if icon, ok := dsassets.PlanModeOutline(t.Info); ok {
				ui.Icon(f, icon).Width(planChipGlyph).Height(planChipGlyph).Color(t.Info)
			}
			ui.Text(f, zhChipLabel).FontSize(planChipFont).LineHeight(planChipLine).
				Weight(500).Color(t.Info)
		})
		if failure != "" {
			ui.Text(f, zhChipExitFailed).FontSize(planChipErrFont).LineHeight(planChipErrLine).
				Color(t.Error)
		}
	})
}

// mixPlanColor 是官方 color-mix(in srgb, a, b t) 的逐通道等价计算。
func mixPlanColor(a, b gfx.Color, t float32) gfx.Color {
	return gfx.Color{
		Red: a.Red*(1-t) + b.Red*t, Green: a.Green*(1-t) + b.Green*t,
		Blue: a.Blue*(1-t) + b.Blue*t, Alpha: a.Alpha*(1-t) + b.Alpha*t,
	}
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
