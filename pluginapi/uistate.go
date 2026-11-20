// uistate.go —— 界面状态（官方 Electron `Local Storage` 的等价物）的动作面。
//
// # 这一层为什么长这样
//
// 官方把一批**纯前端记忆**放在 `Local Storage` 里（官方包里能直接读到的两处：
// `dsh-client-ui-sidebar-browser/lib/client.js:1695` 的
// `persist: "dsh.sidebar-browser.v1"`，`dsh-client-ui-user-questions/lib/client.js:439`
// 的 `persist: "dsh.user-questions.drafts.v1"`）。本仓没有 Electron，宿主给了
// 一条同语义的窄通道（`uiState.get/set/delete`，落
// `<home>/state/ui-state/<key>.json`），这里把它变成插件能用的形状。
//
// # 明确**不是**的东西
//
//   - **不是设置**：不走 `settings.mutate`。那份文档有 revision 栅栏，界面每点
//     一次"已读"都会推进 revision，用户这时保存设置就会 revision_conflict。
//   - **不是官方 host 方法**：官方那两条 persist 是客户端 store，宿主侧没有
//     对应方法名；`uiState.*` 是宿主加法面，不进 canonical 25。
//   - **不是数据库**：单写者、last-write-wins、没有版本比较也没有事务。
package pluginapi

import "encoding/json"

// UIStateActions 是界面状态的动作面；`Actions.UIState` 为 nil = 壳没接。
//
// 读是**同步读壳缓存**、写是**异步落地**：
//
//	读 —— Get 立刻回缓存；第一次问（或还没回来）回 found=false 并触发一次后台拉，
//	      拉完壳 Invalidate，下一帧再读就拿到了。这条路上**绝不**同步等 HTTP：
//	      插件的 Draw 在绘制线程上，等一次网络就会卡首帧。
//	写 —— Set/Delete 立刻更新缓存并丢进 goroutine；失败只报壳的状态栏，
//	      插件不等、不重试（界面记忆丢了不致命，卡住界面才致命）。
type UIStateActions struct {
	// Get 读一份界面状态。第二个返回值 = 壳是否已经拿到过这份状态
	// （false = "还没读到"，与"存的是空"是两回事）。
	Get func(key string) (value json.RawMessage, found bool)
	// Set 整份覆盖一份界面状态。value 必须是合法 JSON。
	Set func(key string, value json.RawMessage)
	// Delete 删掉一份界面状态。
	Delete func(key string)
}
