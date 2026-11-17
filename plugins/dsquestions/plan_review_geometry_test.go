package dsquestions

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestPlanReviewButtonAndResponsiveGeometry(t *testing.T) {
	for name, values := range map[string][2]float32{
		"button.height":      {planReviewButtonHeight, 36},
		"button.padding-x":   {planReviewButtonPadX, 14},
		"button.font":        {planReviewButtonFont, 14},
		"button.line":        {planReviewButtonLine, 22},
		"button.gap":         {planReviewButtonGap, 4},
		"discuss.gap":        {planReviewDiscussGap, 6},
		"icon.slot":          {planReviewIconSlot, 16},
		"icon.glyph":         {planReviewIconGlyph, 14},
		"disabled.opacity":   {planReviewDisabled, 0.4},
		"state.warning-size": {planReviewStateDot, 10},
		"state.ongoing-size": {planReviewStateOngoing, 14},
	} {
		if values[0] != values[1] {
			t.Errorf("%s=%v，官方值=%v", name, values[0], values[1])
		}
	}
	p, env, canvas, _, _ := planReviewQuestionFixture(t, nil)
	for _, tc := range []struct {
		width     float32
		x, bottom float32
		align     ui.Align
	}{
		{1182, 16, 12, ui.Center}, {721, 16, 12, ui.Center}, {720, 12, 10, ui.End}, {600, 12, 10, ui.End},
	} {
		env.Bounds.Width = tc.width
		x, bottom, align := planReviewFooterLayout(env)
		if x != tc.x || bottom != tc.bottom || align != tc.align {
			t.Errorf("viewport=%v：footer=(%v %v %v)，want (%v %v %v)",
				tc.width, x, bottom, align, tc.x, tc.bottom, tc.align)
		}
	}
	env.Bounds.Width = 1182
	drawPlanReviewQuestion(t, p, env, canvas)
	if got := len(canvas.Shadows()); got != 2 {
		t.Fatalf("elevation-panel 柔光层数=%d，官方是 2", got)
	}
}
