// storage.go —— 网页的**存储账号**（官方 `electron/workspace.ts` 的同步移植）。
//
// # 官方是什么形状
//
// 桌面载具开一个 guest 之前，渲染端先算出"这一页属于哪份存储"，再把它交给主进程：
//
//	OFF:dsh-client-ui-sidebar-browser/src/client/electron/workspace.ts:11-33
//	  export async function browserWorkspace(source, sessionId, signal) {
//	    if (source.getSnapshot().phase !== 'ready') { await 等它 ready }
//	    const workspace = source.getSnapshot().items.find(
//	      item => item.sessionIds.some(id => id === sessionId))
//	    return workspace === undefined ? `session:${sessionId}` : `cwd:${workspace.path}`
//	  }
//
//	OFF:…/src/client/electron/ElectronWebViewImpl.ts:145-147
//	  this.workspaceKey ??= await this.workspace(attachmentSignal)   // 一页解析一次
//	  const reservation = await this.bridge.acquire(this.workspaceKey)
//	  换回 `DesktopBrowserReservation{lease, partition}`（`src/types.ts:7-11,22`）
//	OFF:…/src/client/electron/ElectronWebviewPresentation.ts:57
//	  element.setAttribute('partition', reservation.partition)
//
// 主进程那一侧按 workspace **缓存**分区名（`lib/main.js:18093` 的
// `partitions = new Map()`、`:18105-18111` 的 `acquire`），所以同一个 workspace 里
// 后开的 tab 拿到的还是同一个名字——这才是"同工作区共用登录态"的落点。
//
// # 本仓怎么分工
//
// 插件只报**账号**（本文件的 `browserStorageAccount`，形状逐字同官方那条 return），
// 分区名的生成/缓存/校验都在壳里（`cmd/app/web_pages.go` 的 `webPartitionCache`），
// 与"主进程批分区名"同形。这样插件看不懂引擎，壳也不用知道什么是 workspace。
package dsbrowser

import "github.com/MYCF711/dsh-ui/pluginapi"

// 官方那条 return 的两个前缀（`workspace.ts:32`，逐字）。
const (
	// storageAccountWorkspace 是"属于某个工作区"：用**路径**不用 registry 的
	// record id——官方注释原文「Use the Workspace's canonical CWD, not its record
	// id」，同一个目录在 registry 里换一条记录，用户眼里还是同一个工作区。
	storageAccountWorkspace = "cwd:"
	// storageAccountSession 是"不属于任何工作区"：官方注释原文
	// 「ungrouped Sessions remain isolated」，隔离到会话，不是丢回公共存储。
	storageAccountSession = "session:"
)

// browserStorageAccount 复算官方 `browserWorkspace()` 的返回值（见文件头）。
//
// 四条判据，逐条对应：
//
//	会话属于某个工作区（registry 的 sessionIds 里命中）→ `cwd:<工作区路径>`
//	不属于任何工作区                                 → `session:<会话 id>`
//	没有会话身份（`Data.Cur` 为空：无头/首屏前/测试夹具）→ **空串 = 不下发分区**
//	命中工作区但那条 Path 是空的                     → `session:<会话 id>`
//
// 最后一条是本仓比官方多的一道**保守**判断：官方会拼出 `cwd:`（空路径），于是一堆
// "路径为空"的工作区共用同一个存储账号 = 互相看得见 cookies。硬边界是不许在没有
// 有效工作区标识时静默造共享分区，所以这里退回会话级隔离——方向是**更隔离**，
// 不共享、不串台。
//
// 与官方的另一处差异如实登记：官方会**等** workspace registry `phase === 'ready'`
// 再批 guest（IPC 可以异步等）；本仓的画帧是同步的、`Open` 不能挂起，所以 registry
// 还没落地时按官方"find 不到工作区"那一支退回 `session:<id>`。后果只在启动瞬态：
// 若某个会话在 registry 落地前就开过页，它和之后开的 tab 会是两套存储——两套都是
// 隔离的，不会互看，只是不共享登录态。等 registry 的收益不抵"点开浏览器要等一帧
// 以上"的代价，所以不照抄那个等待。
func browserStorageAccount(env *pluginapi.Env) string {
	if env == nil {
		return ""
	}
	sessionID := env.Data.Cur
	if sessionID == "" {
		return ""
	}
	for _, workspace := range env.Data.Workspaces {
		if !workspaceHasSession(workspace, sessionID) {
			continue
		}
		// 命中就是它（官方 `.find()` 只取第一条命中的工作区，这里同序同语义）。
		if workspace.Path == "" {
			break
		}
		return storageAccountWorkspace + workspace.Path
	}
	return storageAccountSession + sessionID
}

// workspaceHasSession 是官方 `item.sessionIds.some(id => id === sessionId)` 的移植。
func workspaceHasSession(workspace pluginapi.Workspace, sessionID string) bool {
	for _, id := range workspace.SessionIDs {
		if id == sessionID {
			return true
		}
	}
	return false
}
