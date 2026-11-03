// acp.go —— 把 dshd 的 ACP 流接进 mockd 的 /ws。
//
// 分工：acpclient.go 只管协议；这里只管"把 ACP 的 update 变成一条要推给界面的事件"。
// muxHandler 仍然是唯一的写者 —— 这个文件只往 acpLive 里塞，不直接写 socket。
package main

import (
	"log"
	"path/filepath"
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
	acp        *ACPClient
	acpSess    string
	acpResumed = map[string]bool{} // 已经 resume 过的会话（避免每次 prompt 都 resume）
	acpLive    chan acpEvent
	acpCWD     string
	acpPrompt  bool
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

	// 点开的是历史会话（session-xxxx）时，先 resume 才能续写；acpSess 是启动时新建的那个。
	tgt := acpSess
	if sid != "" && sid != acpSess {
		mu.Lock()
		done := acpResumed[sid]
		mu.Unlock()
		if !done {
			// 这个二进制的 resume 要求 cwd 也带上（源码已改成可选，但当前 dshd.exe 是旧的）。
			// cwd 从索引里拿：会话文件头里有。
			rcwd := ""
			if realDB != nil {
				if rs := realDB.byID[sid]; rs != nil {
					rcwd = rs.Cwd
				}
			}
			// 先把会话"摆"进 dshd 的 home（默认 <cwd>/tmp/dshd-home），否则它找不到。
			// 是**复制品**：聊完写进副本，真实历史一个字节都不动。
			if dst, serr := stageSession(sid, filepath.Join(acpCWD, "tmp", "dshd-home")); serr != nil {
				log.Printf("ACP 暂存会话失败：%v", serr)
			} else if dst != "" {
				log.Printf("ACP 已暂存会话到 %s", dst)
			}
			if err := acp.Resume(sid, rcwd); err != nil {
				log.Printf("ACP resume %s 失败（回落到新会话）：%v", sid, err)
			} else {
				mu.Lock()
				acpResumed[sid] = true
				mu.Unlock()
				log.Printf("ACP 已续上会话 %s", sid)
			}
		}
		if acpResumed[sid] {
			tgt = sid
		}
	}

	_, err := acp.Prompt(tgt, text, func(u acpUpdate) {
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
