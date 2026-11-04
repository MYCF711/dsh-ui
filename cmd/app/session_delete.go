package main

import (
	"context"
	"time"
)

// deleteSession 是 `session.delete` 的壳侧接线。
//
// # 为什么确认不在这里做
//
// 要不要问用户，取决于**插件把删除做成了几次点击**（我们的侧栏是两次：
// 先点 ×，再点一次确认）。壳不知道那套交互，也就不该替它决定。
// 壳只负责：发请求、把结果写状态栏、**删完刷新列表**。
//
// # 删的是当前会话怎么办
//
// 先把 `m.cur` 清掉再刷新列表：不清的话，界面会继续显示一个已经不存在的会话，
// 而下一帧的 `session.load` 会 404——用户看到的是"界面卡在一个空会话上"。
func (m *model) deleteSession(id string) {
	if id == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.cli.Delete(ctx, id); err != nil {
			// 失败只说一次，且原样给原因：404 和 500 是两种事，
			// 用户需要知道是"这个会话本来就不在了"还是"这次没删掉"。
			m.setStatus("删除会话失败: " + err.Error())
			return
		}
		m.mu.Lock()
		if m.cur == id {
			m.cur = ""
			m.msgs = nil
			m.sessions = nil
		}
		m.mu.Unlock()
		m.setStatus("已删除会话")
		m.refreshList()
	}()
}
