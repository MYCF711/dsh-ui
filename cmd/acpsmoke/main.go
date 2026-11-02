// acpsmoke —— 端到端验证：订阅 /api/mux → 发 prompt → 收真实模型回答。
//
// 只用于验收，不是产品代码。跑完可以删。
//
//	go run ./cmd/acpsmoke -session s-001 -text "用一句话说 1+1 等于几"
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
)

func main() {
	sess := flag.String("session", "s-001", "会话 id")
	text := flag.String("text", "用一句话回答：1+1 等于几？不要调用工具。", "要发的问题")
	base := flag.String("base", "http://127.0.0.1:7788", "mockd 地址")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	wsURL := strings.Replace(*base, "http", "ws", 1) + "/api/mux"
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		log.Fatalf("连 mux 失败：%v", err)
	}
	defer c.CloseNow()

	sub, _ := json.Marshal(map[string]any{"t": "sub", "topic": "session:" + *sess})
	if err := c.Write(ctx, websocket.MessageText, sub); err != nil {
		log.Fatalf("sub 失败：%v", err)
	}
	fmt.Printf("已订阅 session:%s\n", *sess)

	// 订阅就绪后再发 prompt
	go func() {
		time.Sleep(300 * time.Millisecond)
		body, _ := json.Marshal(map[string]any{"params": map[string]any{"id": *sess, "text": *text}})
		resp, err := http.Post(*base+"/api/session.prompt", "application/json", bytes.NewReader(body))
		if err != nil {
			fmt.Println("prompt 失败:", err)
			return
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		fmt.Printf("prompt → HTTP %d %s\n", resp.StatusCode, strings.TrimSpace(buf.String()))
	}()

	var answer strings.Builder
	deadline := time.Now().Add(100 * time.Second)
	for time.Now().Before(deadline) {
		_, data, err := c.Read(ctx)
		if err != nil {
			break
		}
		var f struct {
			T    string          `json:"t"`
			Seq  int             `json:"seq"`
			Kind string          `json:"kind"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(data, &f) != nil || f.T != "ev" {
			continue
		}
		var d struct {
			Delta string `json:"delta"`
			Done  bool   `json:"done"`
			Tool  string `json:"tool"`
		}
		_ = json.Unmarshal(f.Data, &d)
		switch f.Kind {
		case "assistant/message":
			if d.Done {
				fmt.Printf("\n=== 流结束 ===\n%s\n", answer.String())
				os.Exit(0)
			}
			if d.Delta != "" {
				answer.WriteString(d.Delta)
				fmt.Print(d.Delta)
			}
		case "tool/call":
			fmt.Printf("\n  [工具] %s", d.Tool)
		case "approval/asked":
			fmt.Printf("\n  [审批] %s", d.Tool)
		}
	}
	fmt.Println("\n超时。已收到:", answer.String())
}
