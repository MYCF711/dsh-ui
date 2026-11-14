package main

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
)

type rawLoadResult struct {
	apiclient.LoadResult
	Messages []json.RawMessage `json:"messages"`
}

// loadSession 保留 apiclient 当前未暴露的原始 time/stopped，不改共享客户端。
func (m *model) loadSession(ctx context.Context, id string, tail int) (*apiclient.LoadResult, map[string]pubapi.MessageMetadata, error) {
	var raw rawLoadResult
	params := map[string]any{"id": id, "sessionId": id, "tailMessages": tail, "limit": tail}
	if err := m.cli.Call(ctx, "session.load", params, &raw); err != nil {
		return nil, nil, err
	}
	result := raw.LoadResult
	result.Messages = make([]apiclient.Message, 0, len(raw.Messages))
	metadata := map[string]pubapi.MessageMetadata{}
	for _, encoded := range raw.Messages {
		var msg apiclient.Message
		if err := json.Unmarshal(encoded, &msg); err != nil {
			return nil, nil, err
		}
		var extra struct {
			Time    int64 `json:"time"`
			Stopped bool  `json:"stopped"`
		}
		if err := json.Unmarshal(encoded, &extra); err != nil {
			return nil, nil, err
		}
		// 只有思考、没有正文的那条回答：v4 里它是一条正文为空的 assistant 消息。
		// 实时路径落成 role=thinking，重载路径必须**同一条规则**——
		// 不翻的话，切回会话时它就是一个空气泡（或者干脆不见了）。
		if thinking, ok := apiclient.ReasoningOnlyThinking(msg); ok {
			msg = thinking
		}
		index := len(result.Messages)
		result.Messages = append(result.Messages, msg)
		if extra.Time != 0 || extra.Stopped {
			metadata[pubapi.MessageKey(id, msg, index)] = pubapi.MessageMetadata{Time: extra.Time, Stopped: extra.Stopped}
		}
	}
	// 动作卡投影跟着会话一起刷（官方 `ui-action-cards` 的卡片是会话级投影，
	// 宿主从 `session.projections` 的 `values.actionCards` 给）。异步：不许在
	// 这条数据链上等第二个 HTTP；失败保留上一帧（插件自己的语义）。
	m.refreshActionCards(id)
	return &result, metadata, nil
}

type terminalData struct {
	TurnID      string `json:"turnId"`
	Turn        *int64 `json:"turn"`
	Error       string `json:"error"`
	Stopped     bool   `json:"stopped"`
	Interrupted bool   `json:"interrupted"`
}

// turnEndReason 是 v4 `turn/end` 的**真实**载荷形状（宿主 `session.TurnEndData`）：
//
//	{"turn":5,"reason":{"kind":"error","error":{"name":"Error","message":"…"}}}
//	{"turn":5,"reason":{"kind":"aborted","reason":"这一轮被取消…"}}
//	{"turn":5,"reason":{"kind":"completed"}}
//
// kind 取值只有 v4 词汇表里的那几个：completed / max-tokens / aborted /
// blocked / error / interrupted / forked（宿主 `turnEndForExit` 的注释写死"不新造 kind"）。
//
// ⚠️ `kind` 和失败原因都在 **reason 里面**，不在顶层。第一版只读顶层的
// `error string`，于是宿主 M371 之后失败轮次落到"既不是 stopped 也不是 error"
// 的兜底分支，被记成 **done**——界面上一次失败的轮次显示成"跑完了"。
type turnEndReason struct {
	Turn   *int64 `json:"turn"`
	Kind   string `json:"kind"` // 兼容顶层 kind（旧形状）
	Reason *struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
		Error  *struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"reason"`
}

// outcome 把两种形状统一成 (kind, 失败/中止的原因原文)。
func (d turnEndReason) outcome() (string, string) {
	kind, detail := d.Kind, ""
	if d.Reason != nil {
		if d.Reason.Kind != "" {
			kind = d.Reason.Kind
		}
		switch {
		case d.Reason.Error != nil && d.Reason.Error.Message != "":
			detail = d.Reason.Error.Message
		case d.Reason.Reason != "":
			detail = d.Reason.Reason
		}
	}
	return kind, detail
}

func stoppedEvent(ev apiclient.Event) bool {
	switch ev.Kind {
	case "turn/stopped", "turn/cancelled", "turn/canceled", "turn/interrupted":
		return true
	}
	if ev.Kind != "turn/end" && ev.Kind != "turn/done" && ev.Kind != "turn/error" {
		return false
	}
	var data terminalData
	_ = json.Unmarshal(ev.Data, &data)
	return data.Stopped || data.Interrupted
}

func (m *model) recordTurnEventsLocked(id string, ev apiclient.Event) {
	if m.turnEvents == nil {
		m.turnEvents = map[string][]pubapi.TurnEvent{}
	}
	var data terminalData
	_ = json.Unmarshal(ev.Data, &data)
	var end turnEndReason
	_ = json.Unmarshal(ev.Data, &end)
	kind, detail := end.outcome()
	// 旧形状把原因放在顶层 `error` 字符串里；新形状在 reason 里。
	if detail == "" && data.Error != "" {
		detail = data.Error
	}
	appendEvent := func(evKind, detail string) {
		key := id + ":" + evKind + ":local:" + strconv.Itoa(len(m.turnEvents[id]))
		m.turnEvents[id] = append(m.turnEvents[id], pubapi.TurnEvent{
			Key: key, Kind: evKind, TurnID: data.TurnID, Turn: cloneTurnNumber(data.Turn), Error: detail, Time: ev.Time,
		})
	}
	// 三种结局**互斥**，按 v4 的 kind 分派；`turn/error` 是旧形状，一并认。
	switch {
	case ev.Kind == "turn/error" || kind == "error" || kind == "blocked":
		appendEvent("error", detail)
	case stoppedEvent(ev) || kind == "aborted" || kind == "interrupted":
		appendEvent("stopped", detail)
	case ev.Kind == "turn/done" || ev.Kind == "turn/end":
		// done 只进入 Turn 页脚；WP-C1 的独立错误/停止卡仍保持原样。
		appendEvent("done", "")
	}
}
