// acp.go —— 把 dshd 的 ACP 流接进 mockd 的 /api/mux。
//
// 分工：acpclient.go 只管协议；这里只管"把 ACP 的 update 变成一条要推给界面的事件"。
// muxHandler 仍然是唯一的写者 —— 这个文件只往 acpLive 里塞，不直接写 socket。
package main

import (
	"log"
	"time"
)

// acpEvent 是一条准备推给界面的流事件（seq 由 muxHandler 统一编号）。
type acpEvent struct {
	Kind  string // assistant/message / tool/call / approval/asked
	Delta string // assistant/message 的增量
	Tool  string // tool/call 的工具名
	Args  string // tool/call 的一行摘要
	Done  bool   // 一轮结束
}

var (
	acp       *ACPClient
	acpSess   string
	acpLive   chan acpEvent
	acpCWD    string
	acpPrompt bool
)

// startACP 起 dshd 子进程并建一个会话。失败只打日志，mockd 继续用假数据服务。
func startACP(exe, manifest, cwd string) {
	c, err := StartACP(exe, manifest, cwd)
	if err != nil {
		log.Printf("ACP 起不来（回落假回答）：%v", err)
		return
	}
	sid, err := c.NewSession(cwd)
	if err != nil {
		log.Printf("ACP session/new 失败（回落假回答）：%v", err)
		c.Close()
		return
	}
	acp, acpSess, acpCWD = c, sid, cwd
	acpLive = make(chan acpEvent, 256)
	log.Printf("ACP 就绪：session=%s cwd=%s", sid, cwd)
}

// runACPPrompt 把一条用户消息交给真模型，并把流事件塞进 acpLive。
// **它会阻塞**，调用方必须开 goroutine。
func runACPPrompt(sid, text string) {
	if acp == nil {
		return
	}
	defer func() {
		// 一定要清 busy：runACPPrompt 是唯一的结束点，不清的话下一次 prompt 永远 409
		mu.Lock()
		busy[sid] = false
		mu.Unlock()
		select {
		case acpLive <- acpEvent{Kind: "assistant/message", Done: true}:
		default:
		}
		// 60 秒还没人取就丢掉，免得把整条流卡死
		time.AfterFunc(60*time.Second, func() {
			for {
				select {
				case <-acpLive:
				default:
					return
				}
			}
		})
	}()

	_, err := acp.Prompt(acpSess, text, func(u acpUpdate) {
		ev := acpEvent{}
		switch u.Kind {
		case "agent_message_chunk":
			ev.Kind, ev.Delta = "assistant/message", u.Text
		case "agent_thought_chunk":
			ev.Kind, ev.Delta = "assistant/thought", u.Text
		case "tool_call", "tool_call_update":
			ev.Kind, ev.Tool, ev.Args = "tool/call", u.Tool, ""
		case "approval_asked":
			ev.Kind, ev.Tool = "approval/asked", u.Tool
		default:
			return
		}
		select {
		case acpLive <- ev:
		default: // 界面没订阅时丢掉，不阻塞模型
		}
	}, 180*time.Second)
	if err != nil {
		log.Printf("ACP prompt 出错：%v", err)
	}
}
