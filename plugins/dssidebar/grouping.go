// grouping.go —— #62 的分组与手动顺序：组归属来自 `workspace.registry` 的
// `sessionIds`，不再按 cwd 字符串猜。
//
// # 官方判据（唯一来源）
//
//	OFF:ui-workspace/src/client/tree.ts:18        UNGROUPED_KEY = ''
//	OFF:ui-workspace/src/client/tree.ts:26-32     owningGroupKey：会话属于**第一条**
//	    sessionIds 里含它的工作区；一条都不含 → UNGROUPED_KEY。
//	OFF:ui-workspace/src/client/tree.ts:325-367   groupByWorkspace：
//	    - 组顺序 = workspaces 数组顺序（宿主 registry 给出的顺序）；
//	    - 组内顺序 = 该工作区 `sessionIds` 的**手动顺序**，不按 updatedAt 重排；
//	    - 名册里有、会话列表还没到的成员跳过，等它到货再显示（tree.ts:339 的
//	      `if (summary === undefined) continue`）；
//	    - 名册里的成员**先记账**（accounted）再看可见性（tree.ts:340-341）：
//	      所以被过滤词挡掉的成员不会掉进「未分组」；
//	    - 没有任何工作区记账的会话按 updatedAt 降序（orderByRecency）收尾进
//	      「未分组」，这一组空着就不画（tree.ts:356）。
//	OFF:ui-workspace/src/client/tree.ts:348-350   组头文案 = workspace.title；
//	    渲染层对 UNGROUPED_KEY 用字典 `group.ungrouped`（Rows.tsx:282-284）。
//	OFF:ui-workspace/src/client/locales.ts:12     group.ungrouped =「未分组」
//
// # 本仓做不到的两件事（没有真值就不摆位置，不猜）
//
//   - 官方 `sectionMembers`（tree.ts:264-278）把 blank 会话提到组首、把 pinned
//     会话提到普通行之前。`pluginapi.Snapshot` 里没有 registry-global 的 pinned
//     集合，`apiclient.Session` 也没有 blank 位，所以这里按 sessionIds 原序排。
//   - 官方「未分组」还有一份浏览器本地手工序（stores.ts 的 ungroupedOrder）；
//     本仓没有这份本地存储，按官方"还没初始化手工序"的那一支走：按最近更新降序。
package dssidebar

import (
	"sort"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// ungroupedKey 是官方 `UNGROUPED_KEY`（OFF:ui-workspace/src/client/tree.ts:18）：
// 不属于任何工作区的会话归到空键这一组。空键同时是"侧栏没有 registry"时 cwd 组
// 的"未指定工作区"键，两者语义不同但都不带工作区身份，判据一致（见 sessionGroup.Workspace）。
const ungroupedKey = ""

// sessionGroup 是分组后的一条组：组键 + 组头文案 + 可选的 registry 身份 + 成员。
type sessionGroup struct {
	// Key 是组键：registry 模式 = workspaceId；cwd 回退模式 = 去尾分隔符的 cwd；
	// 「未分组」= ungroupedKey。
	Key string
	// Label 是组头文案（已按官方规则解析好，绘制层不再二次猜）。
	Label string
	// Workspace 非 nil = 这条组背后有**真实 registry 工作区**：
	// 可以改名/删除（groupWorkspaceTarget），也可以拖拽排会话序（canDragGroup）。
	// cwd 回退组永远是 nil，官方 UNGROUPED_KEY 那组也是 nil——
	// 它们都没有工作区身份，也就没有写路径。
	Workspace *pluginapi.Workspace
	// Sessions 是这一组当前可见的成员，**已经按该组的手动顺序排好**。
	Sessions []apiclient.Session
}

// groupPlan 是一帧的分组结果。
type groupPlan struct {
	Groups []sessionGroup
	// Registry 表示这些组来自 `workspace.registry`（键是 workspaceId），
	// 而不是 cwd 回退（键是目录路径）。绘制层据此选组身份解析方式。
	Registry bool
}

// Keys 返回组键的显示顺序。
func (p groupPlan) Keys() []string {
	out := make([]string, 0, len(p.Groups))
	for i := range p.Groups {
		out = append(out, p.Groups[i].Key)
	}
	return out
}

// sessionIDs 把一条组的成员 id 按顺序摊开（拖拽换算要用这份"手动顺序"）。
func (g sessionGroup) sessionIDs() []string {
	out := make([]string, 0, len(g.Sessions))
	for i := range g.Sessions {
		out = append(out, g.Sessions[i].ID)
	}
	return out
}

// canDragGroup 是"这一组能不能拖"的唯一判据：
//
//   - 组背后必须有真实 registry 工作区（OFF:rows/Rows.tsx:275/633 的
//     `workspace/insertSessionBefore` 访问权是按**工作区**问的，未分组那组
//     `UngroupedProjectRow` 明确 reorder:false，它的顺序是浏览器本地的，
//     没有对应的宿主写面）；
//   - 壳必须接了写动作（`Actions.WorkspaceInsertSessionBefore` 非 nil）。
//     契约写明 nil = 壳没接 → 侧栏**不进入拖拽态**，不是"拖了没反应"。
func canDragGroup(group sessionGroup, act pluginapi.Actions) bool {
	return group.Workspace != nil && group.Workspace.WorkspaceID != "" &&
		act.WorkspaceInsertSessionBefore != nil
}

// planGroups 是分组的总入口。
//
// `workspaces` 为空 = 宿主没有 registry 或还没拉到（pluginapi.Snapshot 的
// Workspaces 字段写明这条语义），那时**原样退回按 cwd 分组**——老宿主和 mockd
// 上这是唯一诚实的结果；绝不把空当成"用户没有任何工作区"而画空列表。
func planGroups(sessions []apiclient.Session, workspaces []pluginapi.Workspace,
	q string) groupPlan {
	if len(workspaces) == 0 {
		return cwdPlan(sessions, q)
	}
	return registryPlan(sessions, workspaces, q)
}

// cwdPlan 是回退路径：完全沿用 groupByWorkspace 的语义（组内/组间按最近更新降序），
// 只把它包成 groupPlan，好让绘制层只有一条渲染路径。
func cwdPlan(sessions []apiclient.Session, q string) groupPlan {
	groups, order := groupByWorkspace(sessions, q)
	plan := groupPlan{Groups: make([]sessionGroup, 0, len(order))}
	for _, key := range order {
		plan.Groups = append(plan.Groups, sessionGroup{
			Key:      key,
			Label:    displayWorkspace(key),
			Sessions: groups[key],
		})
	}
	return plan
}

// registryPlan 按 registry 的 `sessionIds` 归属和顺序分组，见文件头官方判据。
func registryPlan(sessions []apiclient.Session, workspaces []pluginapi.Workspace,
	q string) groupPlan {
	byID := make(map[string]apiclient.Session, len(sessions))
	for _, s := range sessions {
		if _, dup := byID[s.ID]; !dup {
			byID[s.ID] = s
		}
	}
	accounted := make(map[string]bool, len(sessions))
	plan := groupPlan{Registry: true, Groups: make([]sessionGroup, 0, len(workspaces)+1)}
	for i := range workspaces {
		// 取副本：plan 里的指针不能被下一帧的壳缓存改写带着跑。
		workspace := workspaces[i]
		members := make([]apiclient.Session, 0, len(workspace.SessionIDs))
		for _, id := range workspace.SessionIDs {
			s, ok := byID[id]
			if !ok {
				continue // 名册领先于会话列表：等这条会话到货（tree.ts:339）
			}
			accounted[id] = true // 先记账再看可见性（tree.ts:340-341）
			if !matchesQuery(s, q) {
				continue
			}
			members = append(members, s)
		}
		plan.Groups = append(plan.Groups, sessionGroup{
			Key:       workspace.WorkspaceID,
			Label:     workspaceTitle(workspace, workspace.Path),
			Workspace: &workspace,
			Sessions:  members,
		})
	}

	stray := make([]apiclient.Session, 0)
	for _, s := range sessions {
		if accounted[s.ID] || !matchesQuery(s, q) {
			continue
		}
		stray = append(stray, s)
	}
	if len(stray) > 0 {
		// 官方 orderByRecency（tree.ts:145-159）：更新的在前，同一时刻按 id 稳定。
		sort.SliceStable(stray, func(i, j int) bool { return newerSession(stray[i], stray[j]) })
		plan.Groups = append(plan.Groups, sessionGroup{
			Key:      ungroupedKey,
			Label:    officialUngrouped, // 官方 group.ungrouped（OFF:locales.ts:12），与搜索结果卡同一个词
			Sessions: stray,
		})
	}
	return plan
}

// owningGroupKey 是官方 `owningGroupKey`（OFF:ui-workspace/src/client/tree.ts:26-32）：
// 返回**第一条** sessionIds 里含这条会话的工作区 id；一条都不含 → ungroupedKey。
func owningGroupKey(workspaces []pluginapi.Workspace, sessionID string) string {
	for i := range workspaces {
		for _, id := range workspaces[i].SessionIDs {
			if id == sessionID {
				return workspaces[i].WorkspaceID
			}
		}
	}
	return ungroupedKey
}

// workspaceTargetOf 把一条真实 registry 工作区变成右键菜单 target。
//
// 组身份就是工作区本体（按 workspaceId 直达），不需要像 cwd 回退那样"按路径猜"——
// 猜的版本见 workspaceMenuTarget（保留给 cwd 回退组）。
func workspaceTargetOf(workspace pluginapi.Workspace) pluginapi.ContextMenuTarget {
	return pluginapi.ContextMenuTarget{
		Kind:         "workspace",
		ID:           workspace.WorkspaceID,
		DisplayTitle: workspaceTitle(workspace, workspace.Path),
		Path:         workspace.Path,
	}
}

// groupWorkspaceTarget 解析一条组的工作区身份。
//
// cwd 回退组走**旧的按路径匹配**（workspaceMenuTarget）：那时 registry 是空的，
// 匹配必然失败——这正是老宿主/mockd 的诚实结果（没有 registry 就没有改名/删除
// 的写面），不画空菜单。
func groupWorkspaceTarget(workspaces []pluginapi.Workspace,
	group sessionGroup) (pluginapi.ContextMenuTarget, bool) {
	if group.Workspace != nil {
		return workspaceTargetOf(*group.Workspace), true
	}
	return workspaceMenuTarget(workspaces, group.Key)
}

// dropBeforeID 把「把 sessionID 落到 targetID 这一行的 before/after 半区」
// 换算成宿主 `workspace.insertSessionBefore` 的 `beforeSessionId` 锚点。
//
// 官方换算照抄（OFF:ui-workspace/src/client/rows/WorkspaceBrowser.tsx:218-238
// 的 `sessionDragOrder`）：先在名册里抽掉被拖的那条，再按落点插回去；
// 锚点 = 插回去之后**紧跟它**的那一条，没有就是空串。
//
// 空串 = 官方"移到末尾"的写法，而且必须**整个省略该字段**——宿主对"给了但是空串"
// 回 400（HOST:internal/transport/handlers_workspace_official.go:840-842），
// 省略由壳侧 apiclient 负责（internal/apiclient/workspaces.go:88-95）。
//
// changed=false 表示这次落点不产生可写的顺序变化：拖回原位（官方
// `if (insertAt === sourceIndex) return`）、拖到自己身上、或落点不在这一组里。
// 那时**一次请求都不该发**。
func dropBeforeID(order []string, sessionID, targetID string, after bool) (string, bool) {
	if sessionID == "" || targetID == "" || sessionID == targetID {
		return "", false
	}
	source := indexOfID(order, sessionID)
	target := indexOfID(order, targetID)
	if source < 0 || target < 0 {
		return "", false
	}
	without := make([]string, 0, len(order)-1)
	for _, id := range order {
		if id != sessionID {
			without = append(without, id)
		}
	}
	at := indexOfID(without, targetID)
	if at < 0 {
		return "", false
	}
	if after {
		at++
	}
	if at == source {
		return "", false // 落点就在原位：没有可写的顺序变化
	}
	next := make([]string, 0, len(order))
	next = append(next, without[:at]...)
	next = append(next, sessionID)
	next = append(next, without[at:]...)
	pos := indexOfID(next, sessionID)
	if pos+1 < len(next) {
		return next[pos+1], true
	}
	return "", true // 成了这一组最后一条 → 空串（宿主侧省略字段）
}

func indexOfID(ids []string, want string) int {
	for i, id := range ids {
		if id == want {
			return i
		}
	}
	return -1
}
