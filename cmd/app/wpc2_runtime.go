package main

import (
	"encoding/json"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
)

// sessionStreamState 是属于一个会话的 canonical 流状态。前台的 streamText/busy
// 只是它的发布副本；切走不会 Reset，也不会把另一会话的 Stream 拿过来接着写。
type sessionStreamState struct {
	stream       apiclient.Stream
	text         string
	busy         bool
	activeTurnID string
	activeTurn   *int64
}

func cloneTurnNumber(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (m *model) streamStateLocked(id string) *sessionStreamState {
	if m.streamStates == nil {
		m.streamStates = map[string]*sessionStreamState{}
	}
	state := m.streamStates[id]
	if state == nil {
		state = &sessionStreamState{}
		m.streamStates[id] = state
	}
	return state
}

func (m *model) cachedSessionLocked(id string) *cachedSession {
	if m.cache == nil {
		m.cache = map[string]*cachedSession{}
	}
	cache := m.cache[id]
	if cache == nil {
		cache = &cachedSession{at: time.Now()}
		m.cache[id] = cache
	}
	return cache
}

// publishSessionLocked 把指定会话的 canonical 内容发布到前台；后台调用只更新
// 自己的 cache/streamStates，不写 m.msgs/m.streamText，也不碰另一会话的阅读位置。
func (m *model) publishSessionLocked(id string) {
	if id != m.cur {
		return
	}
	if cache := m.cache[id]; cache != nil {
		m.msgs = cache.msgs
		m.usage = cache.usage
		m.usageTotal = cache.usageTotal
		m.contextTokens = cache.contextTokens
		m.modelName = cache.model
		m.cwd = cache.cwd
	}
	state := m.streamStateLocked(id)
	m.streamText = state.text
	m.turnBusy = state.busy
}

// integrateLoadLocked 将一次 load 并入已有 canonical 缓存。保留 tail 之前已经
// 加载的消息以及 load 飞行期间到达的后台帧；相同 MSGID/v4 身份由 apiclient 对账。
func (m *model) integrateLoadLocked(id string, result *apiclient.LoadResult, metadata map[string]pubapi.MessageMetadata) {
	cache := m.cachedSessionLocked(id)
	cache.msgs = apiclient.MergeMessages(cache.msgs, result.Messages)
	cache.total = result.Total
	if cache.total < len(cache.msgs) {
		cache.total = len(cache.msgs)
	}
	cache.title = result.Title
	cache.usage = result.Usage
	cache.usageTotal = result.UsageTotal
	cache.contextTokens = result.ContextTokens
	cache.model = result.Model
	cache.cwd = result.Cwd
	cache.truncated = result.Truncated
	cache.at = time.Now()
	if m.msgMeta == nil {
		m.msgMeta = map[string]map[string]pubapi.MessageMetadata{}
	}
	if m.msgMeta[id] == nil {
		m.msgMeta[id] = map[string]pubapi.MessageMetadata{}
	}
	for key, value := range metadata {
		m.msgMeta[id][key] = value
	}
	m.publishSessionLocked(id)
}

// eventTurnIdentity 只读取帧 data 里的真实 TurnID/turn；Event.Seq 是 journal
// 水位，永远不会在这里变成 Turn 或消息身份。
func eventTurnIdentity(ev apiclient.Event) (string, *int64) {
	var data struct {
		TurnID  string `json:"turnId"`
		Turn    *int64 `json:"turn"`
		Message *struct {
			TurnID string `json:"turnId"`
			Turn   *int64 `json:"turn"`
		} `json:"message"`
	}
	_ = json.Unmarshal(ev.Data, &data)
	if data.Message != nil {
		if data.TurnID == "" {
			data.TurnID = data.Message.TurnID
		}
		if data.Turn == nil {
			data.Turn = data.Message.Turn
		}
	}
	return data.TurnID, cloneTurnNumber(data.Turn)
}
