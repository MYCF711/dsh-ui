package dsassets

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestPlanPanelOfficialIcons(t *testing.T) {
	fill := gfx.RGB24(0x4176e6)
	plan, ok := PlanModeOutline(fill)
	if !ok || plan.ViewBox != 16 || plan.Stroke != 1 || len(plan.Commands) != 4 || len(plan.Layers) != 3 {
		t.Fatalf("IconPlanOutlineRegular 结构不符：ok=%v icon=%+v", ok, plan)
	}
	chevron, ok := PlanReviewChevron()
	if !ok || chevron.ViewBox != 16 || chevron.Stroke != 1 || len(chevron.Commands) != 4 || len(chevron.Layers) != 0 {
		t.Fatalf("IconChevronRightOutlineRegular 结构不符：ok=%v icon=%+v", ok, chevron)
	}
	edit, ok := PlanReviewEdit(fill)
	if !ok || edit.ViewBox != 16 || edit.Stroke != 1 || len(edit.Commands) != 2 || len(edit.Layers) != 1 {
		t.Fatalf("IconEditOutlineRegular 结构不符：ok=%v icon=%+v", ok, edit)
	}
	closeIcon := PlanModeCloseCircle(fill)
	if closeIcon.ViewBox != 16 || closeIcon.Stroke != 0 || len(closeIcon.Layers) != 1 ||
		closeIcon.Layers[0].FillRule != gfx.PathFillEvenOdd || len(closeIcon.Layers[0].Commands) != 19 {
		t.Fatalf("IconCloseCircleFillRegular 结构不符：%+v", closeIcon)
	}
}
