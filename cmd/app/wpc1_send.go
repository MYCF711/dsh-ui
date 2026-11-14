package main

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsattachment"
)

func (m *model) sendLocal(text string) {
	// 官方允许附件-only 消息（text 为空、content 只有附件）；没有附件时才拒绝空文本。
	if text == "" && !dsattachment.HasItems() {
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
		// 附件准备是两步 HTTP（attachment.get + fileUploads.upload），大文件比普通
		// prompt 慢得多，单独给它 60s；真正的 session.prompt 仍用原来的 5s 超时。
		attachCtx, attachCancel := context.WithTimeout(context.Background(), 60*time.Second)
		batch, prepErr := dsattachment.PreparePrompt(attachCtx)
		attachCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var (
			turn string
			err  error
		)
		switch {
		case prepErr != nil:
			err = prepErr
		case batch.Empty():
			turn, err = m.cli.Prompt(ctx, session, text)
		default:
			turn, err = m.cli.PromptParts(ctx, session, text, batch.Parts())
		}
		if err == nil && !batch.Empty() {
			// 宿主已接收这条消息，rail 上的这批卡才算发出去（官方提交成功才释放草稿）。
			batch.Commit()
		}
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
				// 草稿写回只能由 UI 线程做（Viva 的 `*flex.Field` 不是并发安全的，
				// 见 docs/viva-upstream-editor-string-race.md）。这里只排队；和
				// status 在同一个临界区，所以"看到失败状态"就一定能消费到这次排队。
				m.pendingDraft = &draftRestoreRequest{session: session, text: text}
			}
			m.mu.Unlock()
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

// draftRestoreRequest 是"把草稿写回输入框"的排队项。
//
// session 非空时只有当前会话仍是它才写（切走的会话不写草稿）；overwrite=false
// 表示"只在草稿为空时写"——用户已经输入的新草稿不被覆盖，而失败卡片仍保留
// 同一份文本供显式重试。
type draftRestoreRequest struct {
	session   string
	text      string
	overwrite bool
}

// queueDraftOverwrite 排一次"直接写这段文字"（不检查草稿是否为空）。
// 插件的 `SetDraft` 语义就是覆盖，但调用点可能在非 UI 线程上。
func (m *model) queueDraftOverwrite(text string) {
	m.mu.Lock()
	m.pendingDraft = &draftRestoreRequest{text: text, overwrite: true}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// queueSmokeSend 排一次"灌草稿并发送"。发送本身要读 `m.draft`（见 send），
// 所以它也只能在 UI 线程上发生——这里是 `-smoke` 的后台 goroutine，只排队。
func (m *model) queueSmokeSend(text string) {
	m.mu.Lock()
	m.pendingSmoke = &text
	m.mu.Unlock()
	m.inv.Invalidate()
}

// applyPendingDraft 消费排队的草稿写回与 smoke 发送。
// **必须在 UI 线程（Paint）里调。**
func (m *model) applyPendingDraft() {
	m.mu.Lock()
	req := m.pendingDraft
	m.pendingDraft = nil
	smoke := m.pendingSmoke
	m.pendingSmoke = nil
	cur := m.cur
	m.mu.Unlock()
	if req != nil && (req.session == "" || req.session == cur) {
		m.writeDraft(req.text, req.overwrite)
	}
	if smoke != nil {
		m.writeDraft(*smoke, true)
		m.send()
	}
}

// writeDraft 是**唯一**允许写草稿的地方，只能在 UI 线程上调用。
// 它同时写壳旧输入框和 ds-composer 通过 Fields 暴露的真实草稿——两处都是
// Viva 的 editor，不能从后台 goroutine 直接动。
func (m *model) writeDraft(text string, overwrite bool) {
	if overwrite || strings.TrimSpace(m.draft.Text()) == "" {
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
			if field == nil {
				continue
			}
			if overwrite || strings.TrimSpace(field.Text()) == "" {
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
	if len(outgoing) == 0 {
		return
	}
	if durableText == "" {
		// 附件-only 消息的 durable user event 文本就是空串；只在候选唯一时对账，
		// 两条同时飞的空文本不猜哪条被接受（与下方同文本歧义同一条判据）。
		candidate := -1
		for i, item := range outgoing {
			if item.State == pubapi.OutgoingFailed || item.Text != "" {
				continue
			}
			if candidate >= 0 {
				return
			}
			candidate = i
		}
		if candidate >= 0 {
			m.outgoing[session] = append(outgoing[:candidate], outgoing[candidate+1:]...)
		}
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
