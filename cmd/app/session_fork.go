package main

import (
	"context"
	"time"
)

// forkSession 是 `session.fork` 的壳侧接线。
//
// # 它复制的是**整条会话**，不是"从某条消息往后"
//
// 宿主 `serveFork` 把源会话整份复制到新 id（同一个 cwd），没有切点参数。
// 所以这里不做"分支点"逻辑——那需要在宿主侧有切点，而契约里没有。
//
// # 复制完要做什么
//
// 复制出来的是一个**真实存在的会话**（宿主已经落盘），所以刷列表 + 直接切过去。
// 不切的话用户会以为没成功（列表里多一条，但他还在原来那条上）。
func (m *model) forkSession() {
	m.mu.Lock()
	id := m.cur
	m.mu.Unlock()
	if id == "" {
		m.setStatus("没有当前会话，分支不了")
		return
	}
	m.setStatus("复制会话中…")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		newID, err := m.cli.Fork(ctx, id, "")
		if err != nil {
			// 404 与 500 是两件事，原样给原因。
			m.setStatus("分支失败: " + err.Error())
			return
		}
		m.setStatus("已复制成新分支")
		// 先刷列表再切：顺序反了的话 openSession 会打开一个列表里还没有的会话，
		// 侧栏高亮不上。
		m.refreshList()
		m.openSession(newID)
	}()
}
