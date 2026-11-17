package dsquestions

import (
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
)

func TestQuestionFlowOfficialGeometry(t *testing.T) {
	for name, values := range map[string][2]float32{
		"header.gap":              {questionHeaderGap, 16},
		"eyebrow.gap":             {questionEyebrowGap, 5},
		"options.margin-top":      {questionOptionsMarginTop, 8},
		"options.gap":             {questionOptionsGap, 1},
		"option.min-height":       {questionOptionMinHeight, 40},
		"option.padding-right":    {questionOptionPadRight, 12},
		"option.label-line":       {questionOptionLabelLine, 24},
		"option.description-font": {questionOptionDescFont, 13},
		"option.description-line": {questionOptionDescLine, 20},
		"indicator.seat":          {questionIndicatorSeat, 20},
		"checkbox.size":           {questionCheckboxSize, 14},
		"checkbox.icon":           {questionCheckGlyph, 12},
		"icon-button.size":        {questionIconButtonSize, 24},
		"icon-button.gap":         {questionIconButtonGap, 4},
		"footer.margin-top":       {questionFooterMarginTop, 12},
		"footer.padding-left":     {questionFooterPadLeft, 18},
		"footer.actions-gap":      {questionFooterActionsGap, 12},
		"custom.block-min-height": {questionCustomBlockMinHeight, 64},
		"custom.field-max-height": {questionFieldMaxHeight, 144},
		"custom.focus-border":     {questionFieldFocusBorder, 1},
		"option.focus-ring":       {questionFocusRing, 2},
		"card.max-height":         {questionCardMaxHeight, 520},
		"card.viewport-ratio":     {questionCardViewportRatio, 0.60},
		"compact.title-font":      {questionTitleCompactFont, 15},
		"compact.title-line":      {questionTitleCompactLine, 21},
	} {
		if values[0] != values[1] {
			t.Errorf("%s=%v，官方值=%v", name, values[0], values[1])
		}
	}
}

func TestQuestionFlowCanvasUsesOfficialFontSizes(t *testing.T) {
	canvas, texts, _ := questionFrame(t, true, true, []apiclient.Message{toolMsg(twoOpts)})
	for _, value := range []string{"F7", "F8"} {
		if countText(texts, value) != 0 {
			t.Fatalf("官方序号指示器不得写成自造快捷键 %q：%v", value, texts)
		}
	}
	want := map[string]float32{"确认": 11, "选哪个？": 16, "A": 14, "第一个": 13, "1 / 1": 14, "跳过": 14, "提交": 14}
	seen := map[string]bool{}
	for _, text := range canvas.Texts() {
		if size, ok := want[text.Value]; ok {
			seen[text.Value] = true
			if text.Style.Size != size {
				t.Errorf("%q 字号=%v，官方值=%v", text.Value, text.Style.Size, size)
			}
		}
	}
	for value := range want {
		if !seen[value] {
			t.Errorf("画布缺 %q：%v", value, texts)
		}
	}
}

func TestQuestionOptionSelectionAndStructuredAnswer(t *testing.T) {
	answers := make(chan apiclient.QuestionAnswer, 1)
	p := &Plugin{}
	env := &pluginapi.Env{Data: pluginapi.Snapshot{Cur: "s1"}, Act: pluginapi.Actions{
		AnswerQuestion: func(callID string, answer apiclient.QuestionAnswer) (bool, error) {
			if callID != "call-q" {
				t.Errorf("callID=%q", callID)
			}
			answers <- answer
			return false, nil
		},
	}}
	qs := []Question{{ID: "q1", Question: "选哪个？", CallID: "call-q", Options: []Option{{Label: "A"}, {Label: "B"}}}}
	p.prepareQuestionFlow(env, qs)
	p.chooseQuestionOption(qs, 0)
	if !p.currentQuestionAnswered(qs) || !questionContains(p.flow.selected["q1"], "A") {
		t.Fatalf("选项没有进入已回答草稿：%+v", p.flow)
	}
	p.submitQuestionFlow(env, qs)
	select {
	case answer := <-answers:
		if len(answer.Answers) != 1 || answer.Answers[0].ID != "q1" || len(answer.Answers[0].Selected) != 1 || answer.Answers[0].Selected[0] != "A" {
			t.Fatalf("结构化答案错误：%+v", answer)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("提交没有调用 userQuestions.answer")
	}
}

func TestQuestionOfficialIcons(t *testing.T) {
	for name, tc := range map[string]struct {
		build    func() (gfx.Icon, bool)
		commands int
	}{
		"up":    {questionChevronUpIcon, 4},
		"left":  {questionChevronLeftIcon, 4},
		"right": {questionChevronRightIcon, 4},
		"close": {questionCloseIcon, 4},
		"check": {questionCheckIcon, 4},
	} {
		icon, ok := tc.build()
		if !ok || icon.ViewBox != 16 || icon.Stroke != 1 || len(icon.Commands) != tc.commands {
			t.Errorf("%s 图标不是官方 path 原件：ok=%v icon=%+v", name, ok, icon)
		}
	}
}
