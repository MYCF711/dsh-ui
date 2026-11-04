package dsconversation

import (
	"math"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
)

// reasoning.go —— 思考块的折叠预览、<signature> 清理与真实时长。
//
// 对照 docs/conversation-gap.md 第 12 项。三件事都只做在**显示**这一层：
//
//   - 折叠预览取「最近一个已经写完的段落」的第一行，不是原始文本开头 N 个字；
//   - <signature>…</signature> 只从预览里消失，展开仍画 msg.Text 原文；
//   - 「想了 N 秒」只在宿主真给了起止时间戳时出现，缺任一端就整段隐藏，
//     绝不用 time.Now() 补一个看起来合理的数字。
//
// 判据来自官方 dsh-client-ui-chat 的 lib/types/client/chat/ReasoningRow.js
// （latestCompletedParagraphFirstLine / reasoningText），但有一条刻意的差异：
// 官方把清理后的文本也交给展开态 Body Slot，这里按本轮验收要求让展开保持
// 原始文本——用户点开看到的是落盘内容，不是界面改写过的版本。

const (
	signatureOpenTag  = "<signature>"
	signatureCloseTag = "</signature>"

	// reasoningPreviewRunes 是预览的兜底长度上限。它**只在已经选定最近完成
	// 段落之后**截断，不参与选段；旧的「固定开头 120 字」正是被选段规则取代的
	// 那件事，上限留着只是为了不让超长单行拖慢收起行的测量。
	reasoningPreviewRunes = 120
)

// reasoningDisplayText 去掉模型签名块，只给折叠预览用。
//
// 签名不是思考内容：它是模型侧的校验/加密块。展开态的 reasoningBodyText
// 原样返回 msg.Text，所以这里的结果**绝不能**被写回消息。
func reasoningDisplayText(text string) string {
	if !strings.Contains(text, "<sig") {
		return text
	}
	// 流式刚到半个标签（"<sig"、"<signature"）时还没有正文可显示。
	if trimmed := strings.TrimLeft(text, " \t\r\n"); trimmed != "" && strings.HasPrefix(signatureOpenTag, trimmed) {
		return ""
	}
	var out strings.Builder
	rest := text
	for {
		start := strings.Index(rest, signatureOpenTag)
		if start < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:start])
		after := rest[start+len(signatureOpenTag):]
		end := strings.Index(after, signatureCloseTag)
		if end < 0 {
			// 未闭合：后面整段都是签名，没有可显示的正文。
			break
		}
		rest = after[end+len(signatureCloseTag):]
	}
	return strings.TrimLeft(out.String(), "\r\n")
}

// reasoningBodyText 是展开态正文：**原样返回** msg.Text。
//
// 单独成函数，是为了让「预览清理、正文不清理」这条边界能在一个地方被读到，
// 也能被测试钉住；不要在这里加任何 strings.Replace。
func reasoningBodyText(msg apiclient.Message) string { return msg.Text }

// reasoningPreview 是收起行的一行预览：最近完成段落的第一行（去掉 ** 标记）。
//
// 没有「完成段落」可报时（单行文本、或第一段还在写）退回第一行——那不是
// 「取开头 120 字」，只是没有别的真值可用。
func reasoningPreview(text string) string {
	display := reasoningDisplayText(text)
	if strings.TrimSpace(display) == "" {
		return ""
	}
	summary := latestCompletedParagraphFirstLine(display)
	if summary == "" {
		summary = firstLine(display)
	}
	summary = strings.ReplaceAll(summary, "**", "")
	return truncateRunes(strings.TrimSpace(summary), reasoningPreviewRunes)
}

// latestCompletedParagraphFirstLine 取最近一个已经写完的段落的第一行。
//
// 与官方同名函数同一判据：
//   - 段落分隔 = 一个换行 + 至少一个只含空白的行 + 换行；
//   - 段落里出现第二个换行（哪怕就是分隔符自己的换行），说明这一段的**第一行**
//     已经写完，可以当预览；
//   - 最后一段只有一行（还在写）时，保留上一个完成段落的第一行。
func latestCompletedParagraphFirstLine(text string) string {
	var summary string
	paragraphStart := 0
	for {
		sepAt, sepLen := nextParagraphBreak(text, paragraphStart)
		end := len(text)
		if sepAt >= 0 {
			// 把分隔符自己的换行算进本段：只占一行的段落也因此算「写完」。
			end = sepAt + 1
		}
		if nl := strings.IndexByte(text[paragraphStart:end], '\n'); nl >= 0 {
			if candidate := strings.TrimSpace(text[paragraphStart : paragraphStart+nl]); candidate != "" {
				summary = candidate
			}
		}
		if sepAt < 0 {
			return summary
		}
		paragraphStart = sepAt + sepLen
	}
}

// nextParagraphBreak 找从 from 开始的段落分隔（等价于官方 /\r?\n(?:[\t ]*\r?\n)+/）。
//
// 返回 (第一个换行的下标, 分隔符总长)；没有分隔符返回 (-1, 0)。
func nextParagraphBreak(text string, from int) (int, int) {
	for i := from; i < len(text); i++ {
		if text[i] != '\n' {
			continue
		}
		// 从 i 的换行开始，至少要跟一个空白行。
		j := i + 1
		for {
			k := j
			for k < len(text) && (text[k] == ' ' || text[k] == '\t' || text[k] == '\r') {
				k++
			}
			if k < len(text) && text[k] == '\n' {
				j = k + 1
				continue
			}
			break
		}
		if j > i+1 {
			return i, j - i
		}
	}
	return -1, 0
}

// firstLine 取第一行（没有换行就是全文）。
func firstLine(text string) string {
	if nl := strings.IndexByte(text, '\n'); nl >= 0 {
		return text[:nl]
	}
	return text
}

// truncateRunes 按 rune 截断，避免超长单行拖慢收起行的文本测量。
func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

// thinkingElapsed 用宿主真实时间戳算「想了 N 秒」。
//
// 起止两端都必须是真值：
//
//	起点 start = 前一条消息的 MessageMetadata.Time（这一块思考的输入边界：
//	             上一轮 user/tool 结果，或上一条 assistant 输出被记录的时刻）；
//	终点 end   = 本思考块自己的 MessageMetadata.Time；它缺值时退回同一 Turn 的
//	             终局 TurnEvent.Time（done/error/stopped，且 TurnID 必须真的相同）。
//
// 任一端缺失、时间倒退、间隔不足 1 秒 → (0,false)，调用方不画任何时长。
//
// 这里**不读 time.Now()**：墙钟差是「用户在屏幕前坐了多久」，不是「模型想了多久」。
// 唯一的实时运行态（Busy 尾块）由 thinkingElapsedFor 直接隐藏，连近似值都不给。
func thinkingElapsed(data pluginapi.Snapshot, index int) (int, bool) {
	if index <= 0 || index >= len(data.Msgs) {
		return 0, false
	}
	msg := data.Msgs[index]
	end := data.MessageMeta[pubapi.MessageKey(data.Cur, msg, index)].Time
	if end <= 0 {
		end = terminalEventTime(data.TurnEvents, msg.TurnID)
	}
	if end <= 0 {
		return 0, false
	}
	prev := data.Msgs[index-1]
	start := data.MessageMeta[pubapi.MessageKey(data.Cur, prev, index-1)].Time
	if start <= 0 {
		return 0, false
	}
	// 两条消息都带真 TurnID 且不同：这不是同一次等待，端点不能拼在一起。
	if msg.TurnID != "" && prev.TurnID != "" && msg.TurnID != prev.TurnID {
		return 0, false
	}
	if end <= start {
		return 0, false
	}
	seconds := int(math.Round(float64(end-start) / 1000))
	if seconds < 1 {
		return 0, false
	}
	return seconds, true
}

// thinkingElapsedFor 是界面实际用的入口：正在跑的尾块不报时长。
//
// "running 时保持现有 spinner/preview，不伪造 Thought for Ns"——运行中的
// 尾块即使两端都有历史真值，那也不是这一段的结束时间。
func thinkingElapsedFor(data pluginapi.Snapshot, index int) (int, bool) {
	if data.Busy && index == len(data.Msgs)-1 {
		return 0, false
	}
	return thinkingElapsed(data, index)
}

// terminalEventTime 找同一 Turn 的终局事件时间；TurnID 未知或不匹配都不借。
func terminalEventTime(events []pubapi.TurnEvent, turnID string) int64 {
	if turnID == "" {
		return 0
	}
	for _, event := range events {
		switch event.Kind {
		case "done", "error", "stopped":
		default:
			continue
		}
		if event.TurnID != turnID || event.Time <= 0 {
			continue
		}
		return event.Time
	}
	return 0
}
