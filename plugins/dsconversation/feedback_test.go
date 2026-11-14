package dsconversation

// feedback_test.go —— 反馈的**数据决定**（不画）。
//
// 钉住三件事：
//   - 消息身份怎么来（宿主 messageId → v4 seq；不许拿下标冒充）；
//   - 评分读的是壳缓存里的 authoritative 项（不是内存里的点击态）；
//   - 提交/撤回真的调到对应动作，且 note/category 原样传。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

func msgWithSeq(role string, seq int, text string) apiclient.Message {
	return apiclient.Message{Role: role, Seq: seq, Text: text}
}

func TestFeedbackMessageID(t *testing.T) {
	v4 := int64(42)
	cases := []struct {
		name string
		msg  apiclient.Message
		want string
	}{
		{"宿主 messageId 优先", apiclient.Message{MessageID: "m-1", Seq: 3}, "m-1"},
		{"v4Seq 适配", apiclient.Message{V4Seq: &v4, Seq: 3}, "v4:42"},
		{"load seq 适配", msgWithSeq("assistant", 7, "x"), "v4:7"},
		{"没有身份", apiclient.Message{Role: "assistant"}, ""},
	}
	for _, tc := range cases {
		if got := feedbackMessageID(tc.msg); got != tc.want {
			t.Errorf("%s：feedbackMessageID = %q，want %q", tc.name, got, tc.want)
		}
	}
}

func TestFeedbackRatingReadsHostCache(t *testing.T) {
	items := map[string]pluginapi.FeedbackItem{"v4:9": {MessageID: "v4:9", Rating: apiclient.FeedbackRatingPositive}}
	act := pluginapi.Actions{
		MessageFeedback: func(session string) (map[string]pluginapi.FeedbackItem, bool) {
			return items, true
		},
	}
	msg := msgWithSeq("assistant", 9, "answer")
	if got := feedbackRating(act, "s-1", msg); got != apiclient.FeedbackRatingPositive {
		t.Fatalf("rating = %q，want positive", got)
	}
	// 另一条消息不能被误认成同一条。
	if got := feedbackRating(act, "s-1", msgWithSeq("assistant", 10, "other")); got != "" {
		t.Fatalf("不同消息读到了 %q", got)
	}
	// 壳没接动作 = 未知，不是"没评分"的另一种说法——调用方据此不画按钮。
	if got := feedbackRating(pluginapi.Actions{}, "s-1", msg); got != "" {
		t.Fatalf("没有动作时 rating = %q，want 空", got)
	}
}

func TestSubmitFeedbackPassesDraftAndMarksSubmitting(t *testing.T) {
	var (
		gotSession, gotID, gotRating, gotNote, gotCategory string
	)
	act := pluginapi.Actions{
		MessageFeedbackPut: func(session, id, rating, note, category string) {
			gotSession, gotID, gotRating, gotNote, gotCategory = session, id, rating, note, category
		},
	}
	p := &Plugin{feedbackDialog: &feedbackDialogState{
		MessageID: "v4:5", Rating: apiclient.FeedbackRatingNegative, Category: apiclient.FeedbackCategoryOther,
	}}
	p.feedbackDialog.field.SetText("答非所问")
	p.submitFeedback(act, "s-1")

	if gotSession != "s-1" || gotID != "v4:5" || gotRating != apiclient.FeedbackRatingNegative ||
		gotNote != "答非所问" || gotCategory != apiclient.FeedbackCategoryOther {
		t.Fatalf("put 参数不对：%q %q %q %q %q", gotSession, gotID, gotRating, gotNote, gotCategory)
	}
	if !p.feedbackDialog.submitting {
		t.Fatal("提交后应进入 submitting（官方提交期间禁用表单）")
	}
}

func TestRetractFeedbackCallsDelete(t *testing.T) {
	var gotSession, gotID string
	act := pluginapi.Actions{
		MessageFeedbackDelete: func(session, id string) { gotSession, gotID = session, id },
	}
	p := &Plugin{}
	p.retractFeedback(act, "s-2", msgWithSeq("assistant", 11, "x"))
	if gotSession != "s-2" || gotID != "v4:11" {
		t.Fatalf("delete 参数 = %q/%q，want s-2/v4:11", gotSession, gotID)
	}
	// 没有稳定身份的消息不许撤回（否则会写一个瞎编的 key）。
	p.retractFeedback(act, "s-2", apiclient.Message{Role: "assistant"})
	if gotID != "v4:11" {
		t.Fatalf("没有身份的消息也发了 delete：%q", gotID)
	}
}

func TestFeedbackFailureText(t *testing.T) {
	cases := map[string]string{
		"version-conflict": zhFeedbackConflict,
		"note-too-large":   zhFeedbackNoteLarge,
		"load":             zhFeedbackLoad,
		"whatever":         zhFeedbackGeneric,
	}
	for code, want := range cases {
		if got := feedbackFailureText(code); got != want {
			t.Errorf("%q → %q，want %q", code, got, want)
		}
	}
}

func TestPollFeedbackConsumesResult(t *testing.T) {
	// 冲突：弹窗保持打开、进入失败态（官方 `state.failure`），不弹感谢 toast。
	p := &Plugin{feedbackDialog: &feedbackDialogState{MessageID: "m-1", Rating: apiclient.FeedbackRatingPositive, submitting: true}}
	env := &pluginapi.Env{
		Data: pluginapi.Snapshot{Cur: "s-1"},
		Act: pluginapi.Actions{FeedbackTakeResult: func(session string) (pluginapi.FeedbackResult, bool) {
			return pluginapi.FeedbackResult{MessageID: "m-1", OK: false, Code: "version-conflict"}, true
		}},
	}
	p.pollFeedback(env)
	if p.feedbackDialog == nil {
		t.Fatal("失败时弹窗应保持打开")
	}
	if p.feedbackDialog.submitting || p.feedbackDialog.failure != zhFeedbackConflict {
		t.Fatalf("失败态不对：%+v", p.feedbackDialog)
	}
	if p.feedbackToast != "" {
		t.Fatalf("失败时不该弹成功 toast：%q", p.feedbackToast)
	}

	// 成功：关弹窗 + 感谢 toast。
	p = &Plugin{feedbackDialog: &feedbackDialogState{MessageID: "m-1", submitting: true}}
	env.Act.FeedbackTakeResult = func(session string) (pluginapi.FeedbackResult, bool) {
		return pluginapi.FeedbackResult{MessageID: "m-1", OK: true}, true
	}
	p.pollFeedback(env)
	if p.feedbackDialog != nil || p.feedbackToast != zhFeedbackRecorded {
		t.Fatalf("成功态不对：dialog=%+v toast=%q", p.feedbackDialog, p.feedbackToast)
	}
}

func TestThumbIconMirrorsAndFills(t *testing.T) {
	outline := thumbIcon(true, false)
	filled := thumbIcon(true, true)
	if outline.Stroke <= 0 {
		t.Fatal("未评分的拇指该是描边（空心）")
	}
	if filled.Stroke != 0 {
		t.Fatal("已评分的拇指该是填充（实心）")
	}
	if len(outline.Commands) != len(filled.Commands) {
		t.Fatalf("空心/实心路径长度不同：%d vs %d（切换会跳形）", len(outline.Commands), len(filled.Commands))
	}

	down := thumbIcon(false, false)
	up := thumbIcon(true, false)
	if len(down.Commands) != len(up.Commands) {
		t.Fatalf("上/下拇指路径长度不同：%d vs %d", len(down.Commands), len(up.Commands))
	}
	var upPoint bool
	for i := range up.Commands {
		if up.Commands[i].Kind != gfx.PathCommandMove {
			continue
		}
		upPoint = true
		if down.Commands[i].Point.Y != 16-up.Commands[i].Point.Y || down.Commands[i].Point.X != up.Commands[i].Point.X {
			t.Fatalf("👎 不是 👍 的镜像：第 %d 个 MoveTo %+v vs %+v",
				i, down.Commands[i].Point, up.Commands[i].Point)
		}
	}
	if !upPoint {
		t.Fatal("拇指路径里没有 MoveTo")
	}
}
