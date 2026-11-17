package dsplan

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestPlanCardAndChipOfficialGeometry(t *testing.T) {
	for name, values := range map[string][2]float32{
		"cards.gap": {cardsGap, 10}, "card.height": {cardHeight, 60},
		"card.padding-y": {cardPadY, 8}, "card.padding-x": {cardPadX, 10},
		"card.gap": {cardGap, 10}, "card.border": {cardBorder, 0.5},
		"card.icon": {cardIconSize, 40}, "card.icon-radius": {cardIconRadius, 10},
		"card.title-font": {cardTitleFont, 13}, "card.title-line": {cardTitleLine, 20},
		"card.description-font": {cardDescFont, 10}, "card.description-line": {cardDescLine, 16},
		"card.details-gap": {cardDetailsGap, 2}, "card.open-height": {cardOpenHeight, 28},
		"card.open-padding-y": {cardOpenPadY, 4}, "card.open-padding-x": {cardOpenPadX, 8},
		"card.open-font": {cardOpenFont, 12}, "card.open-line": {cardOpenLine, 18},
		"chip.min-width": {planChipMinWidth, 34}, "chip.glyph": {planChipGlyph, 14},
		"chip.height": {planChipHeight, 28}, "chip.hover-mix": {planChipHoverMix, 0.06},
		"focus.ring":        {planFocusRing, 2},
		"focus.card-offset": {planCardFocusOffset, 3}, "focus.chip-offset": {planChipFocusOffset, 2},
	} {
		if values[0] != values[1] {
			t.Errorf("%s=%v，官方值=%v", name, values[0], values[1])
		}
	}
	if planCardMotion.Duration != 120*time.Millisecond || planCardMotion.Easing != gfx.EaseCSS {
		t.Fatalf("卡片 transition=%+v，官方是 120ms ease", planCardMotion)
	}
}

func TestPlanFocusRingOfficialOffset(t *testing.T) {
	canvas := &gfx.Canvas{}
	draw := ui.DrawContext{Canvas: canvas, Rect: gfx.Rect{X: 20, Y: 20, Width: 60, Height: 28}, Opacity: 1}
	drawPlanFocusRing(&draw, planChipFocusOffset, 8, gfx.Color{Alpha: 1})
	if got := len(canvas.Triangles()); got != 0 {
		t.Fatalf("非 focus-visible 不应画焦点环：%d triangles", got)
	}
	draw.State.FocusVisible = true
	drawPlanFocusRing(&draw, planChipFocusOffset, 8, gfx.Color{Alpha: 1})
	if got := len(canvas.Triangles()); got == 0 {
		t.Fatal("focus-visible 应按 2px/2px offset 画焦点环")
	}
}

func TestPreviewResponsiveOfficialInsets(t *testing.T) {
	_, env, _, _ := fixture(t, nil, 360)
	for _, tc := range []struct {
		width          float32
		top, x, bottom float32
	}{
		{1182, 20, 24, 40}, {768, 20, 24, 40}, {767, 16, 18, 32}, {600, 16, 18, 32},
	} {
		env.Bounds.Width = tc.width
		top, x, bottom := previewInsets(env)
		if top != tc.top || x != tc.x || bottom != tc.bottom {
			t.Errorf("viewport=%v：padding=(%v %v %v)，want (%v %v %v)",
				tc.width, top, x, bottom, tc.top, tc.x, tc.bottom)
		}
	}
}
