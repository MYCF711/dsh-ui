# 工作区分组与拖拽顺序（#62 `dsh-client-ui-workspace`）的接线证据与实现台账

> 建立时间：2026-11-15 · 对应 [official-parity.md](official-parity.md) 第 62 行（🟡 PARTIAL → ✅ IMPLEMENTED）。
> 宿主侧 `workspace.registry` 的 `sessionIds` 与 `workspace.insertSessionBefore` **本就是官方形状**
> （`HOST:docs/transport-contract.md` §15.4），本轮补的是 GUI 侧那两半。

**锚点**：`OFF:` = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-workspace/`；
`HOST:` = 宿主仓 `feat/host-contract-gaps`；`GUI:` = 本仓。

---

## 0. 官方判据（逐条）

| 规则 | 官方锚点 | 本仓落点 |
|---|---|---|
| 一个会话归属哪一组 = **registry 的 `sessionIds` 里第一个认领它的工作区**；没人认领 → 「未分组」 | `OFF:src/client/tree.ts:26-28`（`owningGroupKey`）、`:18`（`UNGROUPED_KEY = ''`） | `GUI:plugins/dssidebar/grouping.go:181` `owningGroupKey` |
| 组内顺序 = 该工作区 `sessionIds` 的**手动顺序**（不是 updatedAt） | `OFF:src/client/tree.ts:325-367` | `grouping.go:127` `registryPlan`（按 `SessionIDs` 顺序收成员） |
| 组间顺序 = registry 的 `order` | 同上 | 同处 |
| 未归属会话排在「未分组」组 | `OFF:src/client/tree.ts:145-159` | 同处（`Key == ""` 组，组头文案取官方 `group.ungrouped`「未分组」） |
| 拖一次 = 在目标位置**前插**一次 | `OFF:dsh-api-workspace-controller/src/index.ts:149`（`insertSessionBefore`） | `HOST:internal/transport/handlers_workspace_official.go:817-845` → `GUI:internal/apiclient/workspaces.go` 的 `WorkspaceInsertSessionBefore` |
| 落点指示线 2px、行缘外 2px | `OFF:src/client/rows/Rows.module.css:370-408` | `GUI:plugins/dssidebar/drag.go:277` `drawDropMarker` |

**空 ≠ 没有**：`Snapshot.Workspaces` 为空切片表示"宿主没有 registry 或还没拉到"，
那时**原样退回按 cwd 分组**（`grouping.go:113` `cwdPlan`，即改动前的行为）。
把空当成"用户没有任何工作区"会画出一个空侧栏——那是另一句话。

---

## 1. 拖拽（Viva 指针模型，不新造事件层）

照 `dsshell` 分隔线拖拽那套（`GUI:plugins/dsshell/shell.go:426-500`）：按下后指针被捕获，
`PointerMove/Up` 一直送到按下的那个 id，所以指针离开行也照样跟手。

| 阶段 | 行为 | 落点 |
|---|---|---|
| PointerDown | 记住 `(workspaceId, sessionId)` 与按下时的行序；**不 Consume**（普通点击仍是选中会话） | `drag.go:98` |
| PointerMove | 越 4px 才开始算拖拽并 Consume（阈值是 viva 没有原生 draggable 的自取，官方由浏览器定） | 同处 |
| 落点解算 | 用本帧 `Canvas.HitBounds` 的行矩形 + 官方 rowHalf 判前半/后半 → `dropBeforeID` 算 `beforeSessionID` | `drag.go:171`、`grouping.go:232` |
| PointerUp | **只写一次**：`Actions.WorkspaceInsertSessionBefore(workspaceId, sessionId, before)` | `drag.go:239` |
| 写失败 | 走 `Act.Notify` 报宿主错误，**不翻成成功**、不回滚成"看起来成功了" | 同处 |

`beforeSessionID` 为空串 = 移到组末尾：客户端会**整个省略** `beforeSessionId` 字段
（宿主对"给了但空串"回 400，`HOST:internal/transport/handlers_workspace_official.go:840-842`）。

**可拖的前提**：`Actions.WorkspaceInsertSessionBefore != nil` **且**这一组是 registry 里
真实存在的工作区（cwd 回退组没有 workspace 身份，没有写路径）。条件不满足时
**不进入拖拽态**——不是"拖了没反应"（`grouping.go:93` `canDragGroup`）。

---

## 2. 没做 / 与官方不同（逐条写清缺什么）

1. **搜索态的工作区名仍按 cwd 推导。** 官方 `labelOf` 会先查 registry 归属；本轮只动了
   列表路径，搜索结果那一行没碰（历史行为保持不变，未回归）。
2. **`sectionMembers` 的 blank / pinned 提位未做。** 官方把"当前空白会话"与置顶会话提到
   组首；`pluginapi.Snapshot` 里没有这两个真值（`pinnedSessionIds` 在 registry 里有，
   但"哪个是当前空白会话"这一层没接），不猜。
3. **官方禁止拖 blank / archived 行，本仓一视同仁。** `apiclient.Session` 没有
   blank/archived 字段，画不出"这一行不可拖"的真值，所以不区分。
4. **插入线左侧那个 5×7 箭头（两条斜线渐变）未画。** Viva 没有对应图元，按
   「不自造图标」只保留官方那条 2px 横线。
5. **跨组拖动 = 无落点、不写。** 与官方一致（同组才可落）；不给"拖到别的组就搬会话"
   这种宿主契约里不存在的语义。

---

## 3. 验收（本仓可复跑）

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
CGO_ENABLED=0 go test ./plugins/dssidebar/... -count=1
```

`GUI:plugins/dssidebar/workspace_order_test.go`（699 行）覆盖：

| 判据 | 为什么必须有 |
|---|---|
| registry 归属优先于 cwd | 这是 #62 的主判据：两台机器同一个 cwd 不再是同一组 |
| 组内顺序 == `sessionIds` 顺序 | 手动序就是拖拽的结果，不能被 updatedAt 覆盖 |
| 未归属会话进「未分组」 | 官方 `UNGROUPED_KEY` 的语义 |
| registry 空 → cwd 回退 | 老宿主/mockd 的诚实回退，不许画空侧栏 |
| 拖拽落点 → `beforeSessionID`（含"移到末尾 = 空串"） | 写路径的正确性，错一格就把会话插到别处 |
| `Actions.WorkspaceInsertSessionBefore == nil` → 不进入拖拽态 | 不画"拖了没反应" |
| 拖拽事件用真 `gfx` 事件走一遍 | 纯函数测不出"指针被捕获"这件事 |

测试有效性用**变异**验过：改掉半区判据 / 组内成员收集 / 插入线偏移 / 兜底清场，
各自都被对应用例抓红后还原。
