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
// 壳只负责：发请求、把结果写状态栏、**删完刷新列表、把那会话的流停掉**。
//
// # 删的是当前会话怎么办
//
// 先把 `m.cur` 清掉再刷新列表：不清的话，界面会继续显示一个已经不存在的会话，
// 而下一帧的 `session.load` 会 404——用户看到的是"界面卡在一个空会话上"。
//
// # 为什么删成功之后必须退订
//
// 订阅是**per-session 一直活着**的（切走不退订，见 subscribe 的注释）。删掉的
// 会话如果留着那条订阅，`runSubscription` 会拿一个已经不存在的 topic 无限重连：
// 1s→2s→…→30s 退避，每次尝试都写连接事实并重绘（连接事实现在挂在订阅对象上，
// 写进的是这一代 `muxSub`，`releaseSession` 一删订阅就跟着没了——但那条
// goroutine 与 socket 仍然没人收，白耗重连和日志，所以要在这里取消）。
//
// per-session 的其余状态（水位 / 世代 / 原始帧 / 流状态 / 温缓存 / 待审批 /
// 连接事实）同理：会话已经不在宿主上了，留着只会在下一帧被当成真值画出来
// （僵尸审批条就是这么来的；连接事实留着还会让指示器停在一个死会话的结论上）。
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
			//
			// **失败时不能动订阅**：宿主那边会话还在，界面的流也该照旧。
			m.setStatus("删除会话失败: " + err.Error())
			return
		}
		m.releaseSession(id)
		m.setStatus("已删除会话")
		m.refreshList()
	}()
}

// releaseSession 丢掉一个**已经不在宿主上**的会话的全部壳侧状态。
//
// 单独抽出来是为了让判据只有一处：删会话（本文件）与将来的任何"会话消失"
// 入口（宿主侧删掉 / 列表里没了）都必须走同一份清理，否则就会出现"订阅停了
// 但水位还在"这种半清状态——那种状态不报错，只是下次订阅从错位处重新开始。
func (m *model) releaseSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 订阅对象里同时挂着这个会话的连接事实：删订阅 = 那三枚布尔一起消失，
	// 于是指示器不会再停在一个已经不存在的会话的结论上（删掉的正是当前会话时，
	// 下面把 cur 清空，`connLinkLocked` 自然回到「未知」）。
	if sub := m.subs[id]; sub != nil {
		if sub.cancel != nil {
			sub.cancel()
		}
		delete(m.subs, id)
	}
	delete(m.seqs, id)
	delete(m.epochs, id)
	delete(m.frames, id)
	delete(m.streamStates, id)
	delete(m.cache, id)
	delete(m.approvals, id)
	if m.cur == id {
		m.cur = ""
		m.msgs = nil
		m.sessions = nil
		// 前台水位与审批是 `seqs`/`approvals` 的发布副本：源已经清掉，
		// 副本留着就会拿一个删掉的会话的 seq 去判"这条帧过期了"。
		m.lastSeq = 0
		m.approval = nil
	}
}
