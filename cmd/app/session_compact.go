package main

import (
	"context"
	"fmt"
	"time"
)

// compactSession 是 `session.compact` 的壳侧接线。
//
// # 为什么超时给得比别的动作长
//
// 压缩要**调一次模型**（生成摘要）。别的动作 5 秒够，这个不够——
// 5 秒超时的表现是"点了没反应"或者一条假的"压缩失败"，而实际上宿主还在总结。
// 宁可多等，也不要让用户以为压缩坏了再点一次（那会再调一次模型）。
const compactTimeout = 120 * time.Second

// # 为什么压完要丢缓存
//
// 会话缓存（`m.cache`）里存的是**压缩前**的消息。不丢的话 `openSession` 走温缓存，
// 界面上还是原来那一长串——用户看到的是"点了压缩，什么都没变"。
func (m *model) compactSession() {
	m.mu.Lock()
	id := m.cur
	m.mu.Unlock()
	if id == "" {
		// 和权限那条一样：没有当前会话就别发，宿主那边 id 必填，发了也是 400，
		// 而界面上会像"点了但没用"。
		m.setStatus("没有当前会话，压缩不了")
		return
	}
	m.setStatus("压缩中…（要调一次模型生成摘要）")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), compactTimeout)
		defer cancel()
		// keep 传 0：用宿主默认。两边各写一个默认值，分叉的表现是
		// "界面以为保留 N 条，实际保留了别的数"。
		res, err := m.cli.Compact(ctx, id, 0)
		if err != nil {
			// 失败原样给原因：读日志失败 / 生成摘要失败 / 写盘失败是三种事。
			m.setStatus("压缩失败: " + err.Error())
			return
		}
		if !res.Compacted {
			// 宿主说"没压"也是一个正常结局（内容太少，不值得折）。
			// 报失败就错了——用户没做错什么。
			m.setStatus("没有可压缩的内容（太短）")
			return
		}
		m.mu.Lock()
		delete(m.cache, id)
		m.mu.Unlock()

		msg := fmt.Sprintf("已压缩：折叠 %d 条，摘要 %d 字", res.Shadowed, res.SummaryChars)
		if res.SummaryChars > 0 && res.Shadowed > 0 {
			m.setStatus(msg)
		} else {
			// 数字缺一半时不要编：宿主老版本可能不填这些字段。
			m.setStatus("已压缩")
		}
		// 重新载入，让前台看到压缩后的投影。
		m.openSession(id)
	}()
}
