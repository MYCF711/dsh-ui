// realsession.go —— 让 mockd 读**真实会话**，界面显示的就是真数据。
//
// 只做最小可用：解 v4 日志、取 header、抽 user/assistant 消息的文本。
// **不做 surface 投影、不做压缩遮蔽、不写任何文件**——`home/sessions` 全程只读。
//
// 格式（见 DSH-GO/AGENTS.md §4）：文件是 session.v4.jsonl.zstd，zstd **多帧**，
// 第 1 帧只有一行 header，之后每帧多行事件。klauspost 的 Reader 会连续解到 EOF。
package main

import (
	"bytes"
	"io"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// realSession 是一个从磁盘读出来的会话。
type realSession struct {
	ID        string
	Cwd       string
	Path      string // 会话文件路径：list 只解 header，load 时再按它做全量解
	UpdatedAt time.Time
	Total     int       // 日志里的消息事件总数（截断前）
	Messages  []message // 已按 tail 截断
	// Full 表示这次是把整个文件解完的结果。
	// 索引期只解到第一条 user 消息（快），所以 Messages 非空**不等于**解全了。
	// 踩过的坑：load 里用 len(Messages)==0 判断要不要全量解，
	// 结果永远命中"已有 1 条"，真实会话只显示第一条消息、总数永远 1。
	Full      bool
}

// realStore 是全部真实会话的索引。
type realStore struct {
	byID  map[string]*realSession
	order []string // 按 UpdatedAt 降序的 id
	bad   int      // 解不开的文件数（跳过并计数，不中断）
	files int      // 扫到的会话文件数
}

// sessionFile 是会话日志的固定文件名。
const sessionFile = "session.v4.jsonl.zstd"

// v4Header 是第 1 帧的那一行。只取我们需要的键。
type v4Header struct {
	Type      string `json:"type"`
	Version   int    `json:"version"`
	ID        string `json:"id"`
	CreatedAt int64  `json:"createdAt"` // 毫秒
	Cwd       string `json:"cwd"`
}

// v4Event 只解析到能拿消息文本为止。
type v4Event struct {
	Type string `json:"type"`
	Seq  int    `json:"seq"`
	Data struct {
		Content []block `json:"content"`

		// tool/call：arguments 是 **JSON 字符串**，不是对象
		CallID    string `json:"callId"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`

		// tool/result：消息体在 data.message 里，不在 data 自己身上
		Message *struct {
			ToolCallID string  `json:"toolCallId"`
			IsError    bool    `json:"isError"`
			Content    []block `json:"content"`
		} `json:"message"`
	} `json:"data"`
}

// block 是 content 数组里的一块（只关心 text）。
type block struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// text 把一条消息的 content 块拼成纯文本（只要 text 块）。
func (e *v4Event) text() string {
	if len(e.Data.Content) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range e.Data.Content {
		if c.Type == "text" && c.Text != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

// loadRealSessions 扫 root 下所有 session.v4.jsonl.zstd，返回索引。
//
// tail 是每个会话最多保留多少条消息（从尾部取）；<=0 表示不限。
// 坏文件跳过并计入 store.bad；一个都读不到时返回 error，调用方回落到假数据。
func loadRealSessions(root string, tail int) (*realStore, error) {
	st := &realStore{byID: map[string]*realSession{}}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 单个目录读不了不影响全局
		}
		if d.IsDir() || d.Name() != sessionFile {
			return nil
		}
		st.files++
		rs, err := decodeSession(path, tail, true) // 索引期只解 header
		if err != nil || rs == nil || rs.ID == "" {
			st.bad++
			return nil
		}
		st.byID[rs.ID] = rs
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(st.byID) == 0 {
		return st, fmt.Errorf("%s 下没有可读会话（扫到 %d 个文件，坏了 %d 个）", root, st.files, st.bad)
	}

	st.order = make([]string, 0, len(st.byID))
	for id := range st.byID {
		st.order = append(st.order, id)
	}
	sort.Slice(st.order, func(i, j int) bool {
		return st.byID[st.order[i]].UpdatedAt.After(st.byID[st.order[j]].UpdatedAt)
	})
	return st, nil
}

// decodeSession 解一个会话文件。zstd 多帧由 Reader 连续解到 EOF。
// decodeSession 解一个会话文件。headerOnly 时只解前 64KB 明文（够 header + 第一条 user 消息），
// 用于启动期建索引——178 个文件全量解要 6 秒，只解 header 约 0.3 秒。
func decodeSession(path string, tail int, headerOnly bool) (*realSession, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec, err := zstd.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer dec.Close()

	var out bytes.Buffer
	if headerOnly {
		if _, err := io.CopyN(&out, dec, 65536); err != nil && out.Len() == 0 && err != io.EOF {
			return nil, err
		}
	} else if _, err := out.ReadFrom(dec); err != nil && out.Len() == 0 {
		return nil, err // 一帧都没解出来才算坏；尾部半帧忽略
	}

	rs := &realSession{Path: path}
	if fi, err := os.Stat(path); err == nil {
		rs.UpdatedAt = fi.ModTime()
	}

	calls := make(map[string]tvCall)
	for _, line := range strings.Split(out.String(), "\n") {
		if line == "" {
			continue
		}
		if rs.ID == "" {
			var h v4Header
			if err := json.Unmarshal([]byte(line), &h); err != nil || h.Type != "session" {
				return nil, fmt.Errorf("首行不是合法 header: %s", trunc(line, 90))
			}
			rs.ID = h.ID
			rs.Cwd = h.Cwd
			continue // headerOnly 也继续读几行：标题比 cwd 有信息量
		}
		var e v4Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // 坏行跳过，别让一条坏行废掉整个会话
		}
		// 工具调用本身不是消息：先记下来，等 tool/result 用 callId 配对。
		if e.Type == "tool/call" {
			if e.Data.CallID != "" {
				calls[e.Data.CallID] = tvCall{name: e.Data.Name, arguments: e.Data.Arguments}
			}
			continue
		}

		// 工具结果：role=tool，文本在 data.message.content 里。
		if e.Type == "tool/result" && e.Data.Message != nil && !headerOnly {
			call := calls[e.Data.Message.ToolCallID]
			txt := tvJoinBlocks(e.Data.Message.Content)
			rs.Total++
			rs.Messages = append(rs.Messages, message{
				Seq:     e.Seq,
				Role:    "tool",
				Text:    tvOneLine(txt, 120),
				Tool:    call.name,                        // 工具名（界面的卡片标题）
				Args:    tvOneLine(call.arguments, 110),   // 参数压一行，给折叠态看
				Summary: tvOneLine(txt, 120),
				View:    tvToolView(call.name, call.arguments, txt, e.Data.Message.IsError),
			})
			continue
		}

		var role string
		switch e.Type {
		case "user/message":
			role = "user"
		case "assistant/message":
			role = "assistant"
		default:
			continue
		}
		txt := e.text()
		if txt == "" {
			continue
		}
		rs.Total++
		rs.Messages = append(rs.Messages, message{Seq: e.Seq, Role: role, Text: txt})
		if headerOnly && role == "user" {
			// 索引期拿到第一条就够做标题了，别为它把整个文件解完。
			// 这里必须留个记号：Messages 有 1 条，但**没解完**。
			rs.Full = false
			return rs, nil
		}
	}

	if tail > 0 && len(rs.Messages) > tail {
		rs.Messages = rs.Messages[len(rs.Messages)-tail:]
	}
	rs.Full = !headerOnly
	return rs, nil
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// firstUserLine 用第一条用户消息的首行当标题（没有就退到会话 id 前 8 位）。
func firstUserLine(rs *realSession) string {
	for _, m := range rs.Messages {
		if m.Role != "user" {
			continue
		}
		line := m.Text
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return trunc(line, 60)
	}
	// 懒解码下 Messages 是空的（索引期只解 header）——这时用 cwd 的最后一段当标题，
	// 比 "会话 session-xxxx" 有信息量得多。
	if rs.Cwd != "" {
		parts := strings.Split(strings.ReplaceAll(rs.Cwd, "\\", "/"), "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if p := strings.TrimSpace(parts[i]); p != "" {
				return trunc(p, 60)
			}
		}
	}
	if len(rs.ID) > 8 {
		return "会话 " + rs.ID[:8]
	}
	return "会话 " + rs.ID
}

// ---- 把真实会话"摆"进 ACP 的 home，让 session/resume 找得到 ----
//
// dshd 在自己那个 home 下按 <home>/sessions/<projectKey>/<encodeSegment(id)>/session.v4.jsonl.zstd
// 找会话。真实会话在别的 home 里，所以续之前得先复制过去（复制品，不碰原文件）。

// projectKey 复刻 DSH v4 的目录命名规则（AGENTS.md §4 第 1 条）。
func projectKey(cwd string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range cwd {
		switch {
		case r == '/' || r == '\\' || r == ':':
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
			prevDash = false
		default:
			fmt.Fprintf(&b, "~%04X", r)
			prevDash = false
		}
	}
	s := strings.TrimLeft(b.String(), "-")
	if s == "" {
		s = "root"
	}
	if len(s) > 251 {
		s = s[:251]
	}
	return "--" + s + "--"
}

// encodeSegment 复刻会话 id 的目录命名规则（AGENTS.md §4 第 2 条）。
func encodeSegment(id string) string {
	if id == "." {
		return "~002E"
	}
	if id == ".." {
		return "~002E~002E"
	}
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			fmt.Fprintf(&b, "~%04X", r)
		}
	}
	return b.String()
}

// stageSession 按 id 把真实会话复制进指定 home（复制品，不碰原文件）。
func stageSession(sid, acpHome string) (string, error) {
	if realDB == nil {
		return "", nil
	}
	rs := realDB.byID[sid]
	if rs == nil || rs.Path == "" {
		return "", nil
	}
	return stageSessionForACP(rs.Path, rs.Cwd, sid, acpHome)
}

// stageSessionForACP 把一条真实会话复制进 ACP 的 home，返回落点路径。
// 复制品，聊天写进去不回污染真实历史。
func stageSessionForACP(src, cwd, id, acpHome string) (string, error) {
	if src == "" || acpHome == "" {
		return "", fmt.Errorf("缺 src 或 acpHome")
	}
	dst := filepath.Join(acpHome, "sessions", projectKey(cwd), encodeSegment(id), "session.v4.jsonl.zstd")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	return dst, nil
}

// MARKER_TEST_REALSESSION
