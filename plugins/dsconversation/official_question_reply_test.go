package dsconversation

import (
	"testing"

	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

func TestQuestionReplyOfficialGeometry(t *testing.T) {
	for name, values := range map[string][2]float32{
		"row.gap":           {questionReplyRowGap, 6},
		"details.gap":       {questionReplyDetailsGap, 12},
		"question.gap":      {questionReplyQuestionGap, 3},
		"answer.margin":     {questionReplyAnswerMargin, 4},
		"actions.gap":       {questionReplyActionsGap, 8},
		"actions.height":    {questionReplyActionsHeight, 28},
		"actions.pad-right": {questionReplyActionsPadRight, 4},
		"time.font":         {questionReplyTimeFont, 13},
		"time.line":         {questionReplyTimeLine, 24},
		"action.size":       {questionReplyActionSize, 28},
		"action.padding":    {questionReplyActionPad, 6},
		"action.glyph":      {questionReplyActionGlyph, 15},
	} {
		if values[0] != values[1] {
			t.Errorf("%s=%v，官方值=%v", name, values[0], values[1])
		}
	}
}

func TestQuestionReplyBubbleShowsQuestionAndAnswer(t *testing.T) {
	_, tree := newTranscriptCanvas()
	entries := []TranscriptEntry{{Key: "reply-json", Kind: ChatNodeQuestionReply, Seq: 1, Turn: 1,
		Visibility: ChatVisibilityVisible, Data: TranscriptData{Outcome: "answered", CallID: "call-1",
			Text: `{"questions":[{"id":"q1","header":"选择","question":"选哪个？"}],"answers":[{"id":"q1","selected":["A"],"custom":"补充"}]}`}}}
	_, canvas := renderTranscript(t, entries, hostViewport(tree))
	header := mustCanvasText(t, canvas, "选择")
	question := mustCanvasText(t, canvas, "选哪个？")
	answer := mustCanvasText(t, canvas, "A, 补充")
	if header.Style.Size != 13 || question.Style.Size != 14 || answer.Style.Size != 14 {
		t.Fatalf("问题回复字号=header %v question %v answer %v", header.Style.Size, question.Style.Size, answer.Style.Size)
	}
	if header.Style.Color != question.Style.Color {
		t.Fatal("header 与 question 应共用 label-secondary")
	}
	if answer.Style.Color == question.Style.Color {
		t.Fatal("answer 应使用用户气泡正文色，而不是 question 的次要色")
	}
}

func TestQuestionReplyCopyUsesOfficialPath(t *testing.T) {
	commands, ok := dsassets.ParsePath(questionReplyCopyBackPath)
	if !ok || len(commands) == 0 {
		t.Fatal("官方 copyBackPath 无法编译")
	}
	var minX, minY float32 = 100, 100
	for _, command := range commands {
		if command.Kind == gfx.PathCommandClose {
			continue
		}
		minX = min(minX, command.Point.X)
		minY = min(minY, command.Point.Y)
	}
	if minX < 0 || minY < 0 {
		t.Fatalf("官方 copy path 坐标越界：%v %v", minX, minY)
	}
}
