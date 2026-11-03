// mockd 的流式回答：session.prompt 之后登记一条待推送的回答，
// /ws 的订阅者按字符把它推出去。
//
// 设计上刻意分成两半：
//   - 登记（enqueueReplyLocked）：在 route 的锁内调用，只改 pending 表；
//   - 取字符（popPendingChar）：在 muxHandler 的 60ms ticker 里调用，一帧一个字符。
//
// 这样"回答"和"连接"解耦：没有订阅者时回答就等着，订阅者来了从第一个字开始推。
package main

import (
	"fmt"
	"strings"
	"time"
)

// pendingReply 是一条还没推完的回答。三句话，按字符逐句推。
type pendingReply struct {
	lines []string // 待推的句子
	line  int      // 当前推到第几句
	pos   int      // 当前句推到第几个字符（按 rune 算）
}

// pending 是"会话 id → 待推送的回答"。读写在 mu 内。
var pending = map[string]*pendingReply{}

// replyLines 造三句假回答。第一句回显用户输入，好让端到端一眼看出对应关系。
func replyLines(userText string) []string {
	short := []rune(userText)
	if len(short) > 24 {
		short = append(short[:24], '…')
	}
	// 三句刻意用 Markdown，验证界面：行内样式 / 列表 / 代码块。
	// 流式推送时 MarkdownView 会边收边重排——未闭合的标记应原样显示。
	return []string{
		fmt.Sprintf("### 收到：%s\n\n这是**加粗**、*斜体*、`行内代码` 与 [链接](https://example.com) 的混排验证。", string(short)),
		"- 列表项一：中文与 English 混排\n- 列表项二\n- 列表项三",
		"```go\nfunc main() {\n\tfmt.Println(\"等宽代码块\")\n}\n```\n\n收尾一句普通文本。",
	}
}

// enqueueReplyLocked 登记一条待推送回答。**调用方必须已持有 mu。**
func enqueueReplyLocked(sid, userText string) {
	pending[sid] = &pendingReply{lines: replyLines(userText)}
	// 兜底：没人订阅时不能把 busy 永久卡住。
	go func(stamp *pendingReply) {
		time.Sleep(60 * time.Second)
		mu.Lock()
		still := pending[sid] == stamp
		mu.Unlock()
		if still {
			commitPending(sid)
		}
	}(pending[sid])
}

// popPendingChar 取下一个字符。
//
//	ok=false        —— 当前没有待推内容（或已经推完）
//	lineDone=true   —— 这一句刚发完，调用方应再发一个 {"done":true} 帧
//	allDone=true    —— 三句都发完了，调用方应 commitPending
func popPendingChar(sid string) (ch rune, ok, lineDone, allDone bool) {
	mu.Lock()
	defer mu.Unlock()

	pr := pending[sid]
	if pr == nil {
		return 0, false, false, false
	}
	if pr.line >= len(pr.lines) {
		return 0, false, false, true
	}
	cur := []rune(pr.lines[pr.line])
	if pr.pos >= len(cur) {
		pr.line++
		pr.pos = 0
		return 0, false, pr.line >= len(pr.lines), pr.line >= len(pr.lines)
	}
	ch = cur[pr.pos]
	pr.pos++
	ok = true
	if pr.pos >= len(cur) {
		lineDone = true
		pr.line++
		pr.pos = 0
		allDone = pr.line >= len(pr.lines)
	}
	return
}

// commitPending 把整条回答写进 history，并清掉 pending 与 busy。
// 返回写进去的正文（没有待推行时返回空串）。
func commitPending(sid string) string {
	mu.Lock()
	defer mu.Unlock()

	pr := pending[sid]
	if pr == nil {
		return ""
	}
	full := strings.Join(pr.lines, "\n")
	h := history[sid]
	h = append(h, message{Seq: len(h) + 1, Role: "assistant", Text: full})
	history[sid] = h
	delete(pending, sid)
	busy[sid] = false
	return full
}

// pendingLen 只给自测用：报告某会话还有几条待推。
func pendingLen(sid string) int {
	mu.Lock()
	defer mu.Unlock()
	pr := pending[sid]
	if pr == nil {
		return 0
	}
	return len(pr.lines) - pr.line
}
