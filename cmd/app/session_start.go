package main

// session_start.go —— 官方 uiWorkspace.startSession 的最小真实闭环。
//
// #17 Creator 调 `startSession(undefined, {agentPreset:'cordis', prompt,
// whenDraftExists:'append'})`：宿主 session.create 的 mode 就是 header.agentPreset；
// 会话打开后只初始化草稿，不替用户发送。任何创建失败都留在状态栏，绝不灌一份
// 看似属于 cordis、实际落在 default 的草稿。

import (
	"context"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

const startSessionTimeout = 8 * time.Second

func (m *model) startSession(options pluginapi.StartSessionOptions) {
	if m.cli == nil {
		m.setStatus("新建会话失败：宿主客户端未接线")
		return
	}
	switch options.WhenDraftExists {
	case "", pluginapi.DraftKeep, pluginapi.DraftAppend, pluginapi.DraftReplace:
	default:
		m.setStatus("新建会话失败：未知 whenDraftExists=" + options.WhenDraftExists)
		return
	}
	m.setStatus("新建会话…")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), startSessionTimeout)
		defer cancel()
		id, err := m.cli.Create(ctx, "", strings.TrimSpace(options.AgentPreset), "")
		if err != nil {
			m.setStatus("新建失败: " + err.Error())
			return
		}
		if id == "" {
			m.setStatus("新建失败: 宿主 session.create 返回了空 sessionId")
			return
		}
		m.refreshList()
		m.openSession(id)
		if options.Prompt != "" && options.WhenDraftExists != pluginapi.DraftKeep {
			m.queueDraftInitialization(id, options.Prompt, options.WhenDraftExists)
		}
	}()
}
