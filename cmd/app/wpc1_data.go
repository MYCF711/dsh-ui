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
	return &result, metadata, nil
}

type terminalData struct {
	TurnID      string `json:"turnId"`
	Turn        *int64 `json:"turn"`
	Error       string `json:"error"`
	Stopped     bool   `json:"stopped"`
	Interrupted bool   `json:"interrupted"`
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
	appendEvent := func(kind string) {
		key := id + ":" + kind + ":local:" + strconv.Itoa(len(m.turnEvents[id]))
		m.turnEvents[id] = append(m.turnEvents[id], pubapi.TurnEvent{
			Key: key, Kind: kind, TurnID: data.TurnID, Turn: cloneTurnNumber(data.Turn), Error: data.Error, Time: ev.Time,
		})
	}
	if ev.Kind == "turn/error" {
		appendEvent("error")
	}
	if stoppedEvent(ev) {
		appendEvent("stopped")
	} else if ev.Kind == "turn/done" || ev.Kind == "turn/end" {
		// done 只进入 Turn 页脚；WP-C1 的独立错误/停止卡仍保持原样。
		appendEvent("done")
	}
}
