package dsconversation

import (
	"encoding/json"
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// QuestionReplyView.module.css:1-81。气泡宽度与既有 UserMessageBubble 共用
// userStackMaxWidth（二者官方公式同为 min(content×0.702,82%)），不另造比例。
const (
	questionReplyRowGap          float32 = 6
	questionReplyDetailsGap      float32 = 12
	questionReplyQuestionGap     float32 = 3
	questionReplyAnswerMargin    float32 = 4
	questionReplyActionsGap      float32 = 8
	questionReplyActionsHeight   float32 = 28
	questionReplyActionsPadRight float32 = 4
	questionReplyTimeFont        float32 = 13
	questionReplyTimeLine        float32 = 24
	questionReplyTimePadRight    float32 = 4
	questionReplyActionSize      float32 = 28
	questionReplyActionPad       float32 = 6
	questionReplyActionGlyph     float32 = 15
	questionReplySkipped                 = "已跳过" // locales.ts:19
	questionReplyCopy                    = "复制"
	questionReplyCopied                  = "已复制"
	questionReplyCopyBackPath            = `M11.9792 1.53296C13.36 1.53296 14.4792 2.65225 14.4792 4.03296V9.42847C14.4792 10.3756 13.9521 11.1987 13.1755 11.6228V10.3298C13.3652 10.0787 13.4792 9.7674 13.4792 9.42847V4.03296C13.4792 3.20453 12.8077 2.53296 11.9792 2.53296H6.58374C6.27966 2.53301 5.99684 2.6235 5.7605 2.77905H4.42358C4.85652 2.03463 5.66056 1.53304 6.58374 1.53296H11.9792Z`
)

type questionReplyQuestion struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Header   string `json:"header"`
}

type questionReplyAnswer struct {
	ID       string   `json:"id"`
	Selected []string `json:"selected"`
	Custom   string   `json:"custom"`
}

// questionReplyPairsOf 是官方 replyPairsOf（question-reply.ts:56-87）的显示投影。
// 非 JSON 或缺 questions 时返回空列表，正文按普通用户气泡显示。
func questionReplyPairsOf(text string) ([]questionReplyQuestion, []questionReplyAnswer) {
	var value struct {
		Questions []questionReplyQuestion `json:"questions"`
		Answers   []questionReplyAnswer   `json:"answers"`
	}
	if json.Unmarshal([]byte(text), &value) != nil || value.Questions == nil {
		return nil, nil
	}
	questions := make([]questionReplyQuestion, 0, len(value.Questions))
	for _, q := range value.Questions {
		if q.ID != "" && q.Question != "" {
			questions = append(questions, q)
		}
	}
	answers := make([]questionReplyAnswer, 0, len(value.Answers))
	for _, a := range value.Answers {
		if a.ID != "" && a.Selected != nil {
			answers = append(answers, a)
		}
	}
	return questions, answers
}

func questionReplyValues(answers []questionReplyAnswer, id string) []string {
	for _, answer := range answers {
		if answer.ID != id {
			continue
		}
		values := append([]string(nil), answer.Selected...)
		if custom := strings.TrimSpace(answer.Custom); custom != "" {
			values = append(values, custom)
		}
		return values
	}
	return nil
}

func questionReplyClipboard(questions []questionReplyQuestion, answers []questionReplyAnswer, fallback string) string {
	if len(questions) == 0 {
		return fallback
	}
	blocks := make([]string, 0, len(questions))
	for _, question := range questions {
		heading := question.Question
		if question.Header != "" && question.Header != question.Question {
			heading = question.Header + " — " + question.Question
		}
		answer := "回答：" + strings.Join(questionReplyValues(answers, question.ID), ", ")
		if len(questionReplyValues(answers, question.ID)) == 0 {
			answer = questionReplySkipped
		}
		blocks = append(blocks, heading+"\n"+answer)
	}
	return strings.Join(blocks, "\n\n")
}

func (v *OfficialTranscriptView) drawOfficialQuestionReply(f ui.C, env *pluginapi.Env, e TranscriptEntry) {
	t := v.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	column := env.MainWidth
	if column <= 0 {
		column = env.Bounds.Width
	}
	questions, answers := questionReplyPairsOf(e.Data.Text)
	ui.Row(f).FillWidth().Justify(ui.End).Children(func() {
		ui.Column(f).Items(ui.End).Gap(questionReplyRowGap).MinWidth(0).MaxWidth(userStackMaxWidth(column)).Children(func() {
			v.Prims.Bubble(f, func() {
				if len(questions) == 0 {
					ui.Wrap(f, e.Data.Text).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
					return
				}
				ui.Column(f).FillWidth().Gap(questionReplyDetailsGap).Children(func() {
					for _, question := range questions {
						ui.Column(f).FillWidth().Children(func() {
							ui.Column(f).FillWidth().Gap(questionReplyQuestionGap).Children(func() {
								if question.Header != "" && question.Header != question.Question {
									ui.Text(f, question.Header).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).Color(t.LabelSecondary)
								}
								ui.Text(f, question.Question).FillWidth().FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelSecondary)
							})
							answer := strings.Join(questionReplyValues(answers, question.ID), ", ")
							if answer == "" {
								answer = questionReplySkipped
							}
							ui.Text(f, answer).FillWidth().MarginTop(questionReplyAnswerMargin).
								FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
						})
					}
				})
			})
			ui.Row(f).Height(questionReplyActionsHeight).Items(ui.Center).Gap(questionReplyActionsGap).
				PadSides(0, questionReplyActionsPadRight, 0, 0).Children(func() {
				if e.TimeKnown() {
					ui.Text(f, e.TimeText()).PadSides(0, questionReplyTimePadRight, 0, 0).
						FontSize(questionReplyTimeFont).LineHeight(questionReplyTimeLine).Color(t.LabelTertiary)
				}
				if env != nil && env.Act.Copy != nil {
					v.drawQuestionReplyCopy(f, env, t, e, questionReplyClipboard(questions, answers, e.Data.Text))
				}
			})
		})
	})
}

func (v *OfficialTranscriptView) drawQuestionReplyCopy(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, e TranscriptEntry, text string) {
	copied := v.questionReplyCopied[e.Key]
	label := questionReplyCopy
	if copied {
		label = questionReplyCopied
	}
	button := ui.Row(f).Width(questionReplyActionSize).Height(questionReplyActionSize).Center().
		Pad(questionReplyActionPad).Radius(t.Radii.SM).Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, "question-reply-copy-"+e.Key), label)
	if button.Clicked() {
		env.Act.Copy(text)
		if v.questionReplyCopied == nil {
			v.questionReplyCopied = map[string]bool{}
		}
		v.questionReplyCopied[e.Key] = true
	}
	button.Children(func() {
		if copied {
			if commands, ok := dsassets.CheckOutline(); ok {
				icon := gfx.Icon{ViewBox: 16, Stroke: 1, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands}
				ui.Icon(f, icon).Width(questionReplyActionGlyph).Height(questionReplyActionGlyph).Color(t.LabelTertiary)
			}
			return
		}
		drawQuestionReplyCopyIcon(f, t.LabelTertiary)
	})
}

// drawQuestionReplyCopyIcon 用官方 copy.jsx 的圆角 rect 与 fill path 原件组成 15px 图标；
// 不拿字符或自绘近似纸形代替。rect 来自 dsassets.Copy 的同一官方登记值。
func drawQuestionReplyCopyIcon(f ui.C, color ui.Color) {
	commands, ok := dsassets.ParsePath(questionReplyCopyBackPath)
	if !ok {
		return
	}
	ui.Row(f).Width(questionReplyActionGlyph).Height(questionReplyActionGlyph).
		Draw(func(d *ui.DrawContext) {
			scale := d.Rect.Width / 16
			rect := gfx.Rect{X: d.Rect.X + 1.52075*scale, Y: d.Rect.Y + 4.07373*scale,
				Width: 10.3932 * scale, Height: 10.3932 * scale}
			d.Canvas.RoundedOutline(rect, 2*scale, scale, d.Fade(color))
		}).Children(func() {
		icon := gfx.Icon{ViewBox: 16, Commands: commands}
		ui.Icon(f, icon).Width(questionReplyActionGlyph).Height(questionReplyActionGlyph).Color(color)
	})
}
