// reasoning_test.go —— 思考块折叠预览与真实时长（docs/conversation-gap.md 第 12 项）。
//
// 这些用例先按**旧实现**写：旧摘要固定取原始文本前 120 rune、不清理
// <signature>、没有「想了 N 秒」。所以它们在旧实现上必须红，实现之后转绿。
// 视觉/命中测试在 reasoning_visual_test.go（需要无头窗口，带 build tag）。
package dsconversation

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
)

func reasoningKey(data pluginapi.Snapshot, index int) string {
	return pubapi.MessageKey(data.Cur, data.Msgs[index], index)
}

// TestReasoningPreviewUsesLatestCompletedParagraph 是「开头段落 vs 最近完成段落」
// 的判据：多段思考收起时，预览必须来自**最近一个已经写完的段落**，
// 不是原始文本开头。
func TestReasoningPreviewUsesLatestCompletedParagraph(t *testing.T) {
	text := "开头第一段：先看目录结构，确认仓库基线与测试基线。\n\n" +
		"中间一段：把会话加载路径和流式路径对齐。\n\n" +
		"最近完成段落第一行\n最近段落还在写"
	got := reasoningPreview(text)
	if got != "最近完成段落第一行" {
		t.Fatalf("折叠预览必须取最近完成段落的第一行，实际 %q", got)
	}
	if strings.Contains(got, "开头第一段") {
		t.Fatalf("折叠预览仍在取原始文本开头：%q", got)
	}
}

// TestReasoningPreviewStripsSignature 证明 <signature> 只从预览里消失；
// 展开态正文由 reasoningBodyText 原样给出，一个字符都不能少。
func TestReasoningPreviewStripsSignature(t *testing.T) {
	const raw = "<signature>c2lnbmF0dXJlLWJsb2Nr</signature>\n" +
		"第一段：先看目录结构。\n\n" +
		"最近完成段落第一行\n最近段落还在写"

	preview := reasoningPreview(raw)
	if preview != "最近完成段落第一行" {
		t.Fatalf("清理签名后预览应为最近完成段落，实际 %q", preview)
	}
	if strings.Contains(preview, "signature") || strings.Contains(preview, "c2lnbmF0dXJl") {
		t.Fatalf("预览仍带 <signature> 内容：%q", preview)
	}

	// 「只用于 preview」的另一半：展开正文必须与原始文本逐字相同。
	if body := reasoningBodyText(apiclient.Message{Role: "thinking", Text: raw}); body != raw {
		t.Fatalf("展开正文被清理了，内容丢失：\nwant=%q\ngot=%q", raw, body)
	}
	if !strings.Contains(reasoningBodyText(apiclient.Message{Text: raw}), "<signature>") {
		t.Fatal("展开正文丢了 <signature> 原文；签名只允许从预览里消失")
	}
}

// TestReasoningPreviewSignatureShapes 覆盖签名在流式/异常数据里的几种形状。
func TestReasoningPreviewSignatureShapes(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "整段只有签名", text: "<signature>abc</signature>", want: ""},
		{name: "签名未闭合", text: "<signature>abc", want: ""},
		{name: "签名在中间", text: "先看目录。\n\n<signature>abc</signature>\n\n最后一段第一行\n还在写", want: "最后一段第一行"},
		{name: "没有签名", text: "第一段。\n\n第二段。", want: "第一段。"},
		{name: "单行文本", text: "只有一行思考，没有换行", want: "只有一行思考，没有换行"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reasoningPreview(tc.text); got != tc.want {
				t.Fatalf("reasoningPreview(%q) = %q，want %q", tc.text, got, tc.want)
			}
		})
	}
}

// TestReasoningPreviewDropsMarkdownBold 官方摘要会去掉 ** 标记（粗体在收起行里
// 只是一堆星号）。段落选择不变。
func TestReasoningPreviewDropsMarkdownBold(t *testing.T) {
	text := "开头。\n\n**最近完成段落**第一行\n第二行还在写"
	if got := reasoningPreview(text); got != "最近完成段落第一行" {
		t.Fatalf("预览应去掉 ** 标记，实际 %q", got)
	}
}

// reasoningElapsedFixture 造一条 user → thinking 的两条消息序列。
func reasoningElapsedFixture(t0 int64) (pluginapi.Snapshot, map[string]pubapi.MessageMetadata) {
	msgs := []apiclient.Message{
		{Seq: 1, Role: "user", Text: "开始吧。"},
		{Seq: 2, Role: "thinking", Text: "想一想。"},
	}
	data := pluginapi.Snapshot{Cur: "sess-reasoning-elapsed", Msgs: msgs, Total: len(msgs)}
	meta := map[string]pubapi.MessageMetadata{
		reasoningKey(data, 0): {Time: t0},
		reasoningKey(data, 1): {Time: t0 + 7000},
	}
	return data, meta
}

// TestThinkingElapsedUsesRealTimestamps 证明有起止真值时显示真实秒数。
func TestThinkingElapsedUsesRealTimestamps(t *testing.T) {
	data, meta := reasoningElapsedFixture(1762222444000)
	data.MessageMeta = meta
	got, ok := thinkingElapsed(data, 1)
	if !ok || got != 7 {
		t.Fatalf("真实时间戳 7s 应显示 7 秒，实际 (%d,%v)", got, ok)
	}
	// 历史时间戳（2001 年）同样只算差值，不掺当前墙钟。
	data.MessageMeta[reasoningKey(data, 0)] = pubapi.MessageMetadata{Time: 1000000000000}
	data.MessageMeta[reasoningKey(data, 1)] = pubapi.MessageMetadata{Time: 1000000000000 + 12000}
	if got, ok := thinkingElapsed(data, 1); !ok || got != 12 {
		t.Fatalf("历史真值应为 12 秒，实际 (%d,%v)", got, ok)
	}
}

// TestThinkingElapsedHiddenWhenUnknown 是「未知隐藏、不用墙钟补」的判据：
// 任一端没有真值、时间倒退、间隔不足 1 秒，都必须整段不显示。
func TestThinkingElapsedHiddenWhenUnknown(t *testing.T) {
	t.Run("没有元数据", func(t *testing.T) {
		data, _ := reasoningElapsedFixture(1762222444000)
		if seconds, ok := thinkingElapsed(data, 1); ok || seconds != 0 {
			t.Fatalf("两端都未知时必须隐藏，实际 (%d,%v)", seconds, ok)
		}
	})
	t.Run("只有终点", func(t *testing.T) {
		data, meta := reasoningElapsedFixture(1762222444000)
		delete(meta, reasoningKey(data, 0))
		data.MessageMeta = meta
		if _, ok := thinkingElapsed(data, 1); ok {
			t.Fatal("缺起点真值时必须隐藏")
		}
	})
	t.Run("只有起点", func(t *testing.T) {
		data, meta := reasoningElapsedFixture(1762222444000)
		delete(meta, reasoningKey(data, 1))
		data.MessageMeta = meta
		if _, ok := thinkingElapsed(data, 1); ok {
			t.Fatal("缺终点真值时必须隐藏")
		}
	})
	t.Run("时间倒退", func(t *testing.T) {
		data, meta := reasoningElapsedFixture(1762222444000)
		meta[reasoningKey(data, 1)] = pubapi.MessageMetadata{Time: 1762222444000 - 3000}
		data.MessageMeta = meta
		if _, ok := thinkingElapsed(data, 1); ok {
			t.Fatal("终点早于起点是异常数据，必须隐藏而不是画负数")
		}
	})
	t.Run("不足一秒", func(t *testing.T) {
		data, meta := reasoningElapsedFixture(1762222444000)
		meta[reasoningKey(data, 1)] = pubapi.MessageMetadata{Time: 1762222444000 + 400}
		data.MessageMeta = meta
		if seconds, ok := thinkingElapsed(data, 1); ok || seconds != 0 {
			t.Fatalf("不足一秒不能显示「想了 0 秒」，实际 (%d,%v)", seconds, ok)
		}
	})
	t.Run("第一条消息", func(t *testing.T) {
		data, meta := reasoningElapsedFixture(1762222444000)
		data.MessageMeta = meta
		if _, ok := thinkingElapsed(data, 0); ok {
			t.Fatal("没有前一条消息时没有起点真值，必须隐藏")
		}
	})
}

// TestThinkingElapsedTurnEventFallback 证明 TurnEvent 也是合法的终点真值：
// 思考块自己缺时间时，同一 Turn 的终局事件时间可以补终点；TurnID 不同就不许借。
func TestThinkingElapsedTurnEventFallback(t *testing.T) {
	t0 := int64(1762222444000)
	msgs := []apiclient.Message{
		{Seq: 1, Role: "user", Text: "开始吧。"},
		{Seq: 2, Role: "thinking", TurnID: "turn-9", Text: "想一想。"},
	}
	data := pluginapi.Snapshot{Cur: "sess-reasoning-turn", Msgs: msgs, Total: len(msgs)}
	data.MessageMeta = map[string]pubapi.MessageMetadata{
		reasoningKey(data, 0): {Time: t0},
	}
	data.TurnEvents = []pubapi.TurnEvent{
		{Key: "other", Kind: "done", TurnID: "turn-other", Time: t0 + 99000},
		{Key: "same", Kind: "done", TurnID: "turn-9", Time: t0 + 5000},
	}
	got, ok := thinkingElapsed(data, 1)
	if !ok || got != 5 {
		t.Fatalf("同 Turn 终局事件应给出 5 秒，实际 (%d,%v)", got, ok)
	}

	// TurnID 对不上：不许拿别的 Turn 的结束时间冒充这一块思考的终点。
	data.TurnEvents = []pubapi.TurnEvent{{Key: "other", Kind: "done", TurnID: "turn-other", Time: t0 + 99000}}
	if seconds, ok := thinkingElapsed(data, 1); ok || seconds != 0 {
		t.Fatalf("TurnID 不同时必须隐藏，实际 (%d,%v)", seconds, ok)
	}
}

// TestThinkingElapsedHiddenWhileRunning 是「running 时不伪造 Thought for Ns」：
// 正在跑的尾块即使两端都有真值也不显示时长。
func TestThinkingElapsedHiddenWhileRunning(t *testing.T) {
	data, meta := reasoningElapsedFixture(1762222444000)
	data.MessageMeta = meta
	data.Busy = true
	if seconds, ok := thinkingElapsedFor(data, 1); ok || seconds != 0 {
		t.Fatalf("运行中的尾块不许显示时长，实际 (%d,%v)", seconds, ok)
	}
	// 结束后同一条（Busy=false）才允许显示。
	data.Busy = false
	if got, ok := thinkingElapsedFor(data, 1); !ok || got != 7 {
		t.Fatalf("收尾后应显示 7 秒，实际 (%d,%v)", got, ok)
	}
}

// TestReasoningOnlyThinkingKeepsThinkingBranch 是「reasoning-only assistant 不消失」
// 的插件侧钉子：宿主给的 reasoning-only assistant 仍然翻成 thinking 分支；
// 即使整段文本只有一个签名（清理后预览为空），它也不是"没有内容"。
func TestReasoningOnlyThinkingKeepsThinkingBranch(t *testing.T) {
	msg, ok := apiclient.ReasoningOnlyThinking(apiclient.Message{
		Role: "assistant", Reasoning: "只有思考的回答。",
	})
	if !ok {
		t.Fatal("reasoning-only assistant 没有翻成 thinking，重载后会变成空气泡")
	}
	if got := roleKind(msg.Role); got != kindThinking {
		t.Fatalf("翻出来的消息必须走思考分支，角色 %q → kind %v", msg.Role, got)
	}
	if reasoningPreview(msg.Text) == "" {
		t.Fatal("正常思考文本不应没有预览")
	}

	// 只剩签名的思考：预览为空是**允许**的（不是隐藏整行），
	// 收起行仍由 drawThinking 画标题与字数。
	sigOnly, ok := apiclient.ReasoningOnlyThinking(apiclient.Message{
		Role: "assistant", Reasoning: "<signature>abc</signature>",
	})
	if !ok {
		t.Fatal("签名-only 的 reasoning-only assistant 也应翻成 thinking")
	}
	if reasoningPreview(sigOnly.Text) != "" {
		t.Fatalf("清理签名后应无预览，实际 %q", reasoningPreview(sigOnly.Text))
	}
}
