package main

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
)

func (m *model) sendLocal(text string) {
	if text == "" {
		return
	}
	m.mu.Lock()
	cur := m.cur
	if cur == "" {
		m.mu.Unlock()
		m.setStatus("先选一个会话")
		return
	}
	if m.outgoing == nil {
		m.outgoing = map[string][]pubapi.OutgoingMessage{}
	}
	m.localSend++
	key := cur + ":local-send:" + strconv.FormatUint(m.localSend, 10)
	m.outgoing[cur] = append(m.outgoing[cur], pubapi.OutgoingMessage{Key: key, Text: text, State: pubapi.OutgoingSending})
	m.mu.Unlock()
	if m.draft.Text() == text {
		m.draft.SetText("")
	}
	m.inv.Invalidate()
	m.dispatchSend(cur, key, text)
}

func (m *model) retrySend(key string) {
	m.mu.Lock()
	cur := m.cur
	found := false
	text := ""
	for i := range m.outgoing[cur] {
		if m.outgoing[cur][i].Key == key && m.outgoing[cur][i].State == pubapi.OutgoingFailed {
			text = m.outgoing[cur][i].Text
			m.outgoing[cur][i].State = pubapi.OutgoingSending
			m.outgoing[cur][i].Error = ""
			found = true
			break
		}
	}
	m.mu.Unlock()
	if !found {
		m.setStatus("这条失败草稿已不可重试")
		return
	}
	m.inv.Invalidate()
	m.dispatchSend(cur, key, text)
}

func (m *model) dispatchSend(session, key, text string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		turn, err := m.cli.Prompt(ctx, session, text)
		m.mu.Lock()
		index := -1
		for i := range m.outgoing[session] {
			if m.outgoing[session][i].Key == key {
				index = i
				break
			}
		}
		if index < 0 {
			// durable user event 已经对账；迟到的 HTTP 结果不再改本地视图。
			m.mu.Unlock()
			return
		}
		state := m.streamStateLocked(session)
		if err != nil {
			m.outgoing[session][index].State = pubapi.OutgoingFailed
			m.outgoing[session][index].Error = err.Error()
			state.busy = false
			state.activeTurnID = ""
			state.activeTurn = nil
			m.publishSessionLocked(session)
			if m.cur == session {
				m.status = "发送失败: " + err.Error()
				m.loading = false
			}
			m.mu.Unlock()
			m.restoreDraftIfEmpty(session, text)
			m.inv.Invalidate()
			return
		}
		m.outgoing[session][index].State = pubapi.OutgoingSent
		m.outgoing[session][index].TurnID = turn
		m.outgoing[session][index].Error = ""
		state.busy = true
		state.activeTurnID = turn
		state.activeTurn = nil
		m.publishSessionLocked(session)
		if m.cur == session {
			m.status = "已发送"
			if turn != "" {
				m.status += " · turn " + turn
			}
		}
		m.mu.Unlock()
		m.inv.Invalidate()
	}()
}

// restoreDraftIfEmpty 恢复壳旧输入框以及 ds-composer 通过 Fields 暴露的真实草稿；
// 用户已输入的新草稿不被覆盖。失败卡片仍保留同一份文本供显式重试。
func (m *model) restoreDraftIfEmpty(session, text string) {
	m.mu.Lock()
	current := m.cur == session
	m.mu.Unlock()
	if !current {
		return
	}
	if strings.TrimSpace(m.draft.Text()) == "" {
		m.draft.SetText(text)
	}
	if m.kernel == nil {
		return
	}
	for _, plugin := range m.kernel.Plugins() {
		if plugin.ID() != "ds-composer" {
			continue
		}
		fields, ok := plugin.(pluginapi.Fields)
		if !ok {
			continue
		}
		for _, field := range fields.Fields() {
			if field != nil && strings.TrimSpace(field.Text()) == "" {
				field.SetText(text)
			}
		}
	}
}

// reconcileOutgoingLocked 只消费本次新到的 durable user event，不扫描历史。
// TurnID 精确匹配优先；没有 rpcId 时，文本兜底仅在候选唯一时成立。一次 event
// 最多清一条，两个同文本发送不会因一次回包被批量误清。失败态仍需显式重试。
func (m *model) reconcileOutgoingLocked(session, durableText string, durableTurn ...string) {
	outgoing := m.outgoing[session]
	if len(outgoing) == 0 || durableText == "" {
		return
	}
	turnID := ""
	if len(durableTurn) > 0 {
		turnID = durableTurn[0]
	}
	remove := func(index int) {
		m.outgoing[session] = append(outgoing[:index], outgoing[index+1:]...)
	}
	if turnID != "" {
		for i, candidate := range outgoing {
			if candidate.State != pubapi.OutgoingFailed && candidate.TurnID == turnID && candidate.Text == durableText {
				remove(i)
				return
			}
		}
	}
	candidate := -1
	for i, outgoing := range outgoing {
		if outgoing.State == pubapi.OutgoingFailed || outgoing.Text != durableText {
			continue
		}
		// durable TurnID 与本地已知的另一个 TurnID 不同时，不能拿文本硬配。
		if turnID != "" && outgoing.TurnID != "" && outgoing.TurnID != turnID {
			continue
		}
		if candidate >= 0 {
			return // 同文本多候选是歧义，不猜哪一条被 durable event 接受。
		}
		candidate = i
	}
	if candidate >= 0 {
		remove(candidate)
	}
}
