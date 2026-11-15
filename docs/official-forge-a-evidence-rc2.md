# #64a · forge 面板 A 档（本地 git + 工作树请求）落地证据（rc.2）

> 账本条目：`docs/official-parity.md` 的 **#64**（`dsh-experimental-client-ui-forge`）。
> 本条只做 **A 档 14 个方法**那一半；CR（变更请求）那一半是 `#64b`，仍 HIDE。
> 官方包：`OFF:dsh-experimental-client-ui-forge`（`node_modules` 内 102 文件 / 2.7 MB，含 `src/**` 46 tsx + 39 ts）。
> 宿主：`DSH-GO`（`origin/main` 后，`forgePanel` 32 方法全部**真实现**，结构化未实现 = 0，
> 逐条 `HOST:docs/evidence/official-method-coverage-0.2.2-rc.2.md:68-99`）。

## 1. 这一轮到底做了哪 14 个方法

| 族 | 方法 | 官方契约 | 宿主真实现 | GUI 客户端 |
|---|---|---|---|---|
| 本地 git 3 | `branches` | `OFF:src/index.ts:222` / typert `:1029` | `HOST:cmd/dshd/serve_forge_git.go:54` | `Client.ForgeBranches` |
| | `localDiff` | `:301` / typert `:1840` | `serve_forge_git.go:62` | `Client.ForgeLocalDiff` |
| | `localText` | `:347` / typert `:1892` | `serve_forge_git.go:74` | `Client.ForgeLocalText` |
| worktree 6 | `requestWorktree` | `:729` / typert `:2200` | `HOST:cmd/dshd/serve_worktree_api.go:173` | `Client.ForgeRequestWorktree` |
| | `cancelWorktree` | `:748` / typert `:1064` | 同上 | `Client.ForgeCancelWorktree` |
| | `retryWorktree` | `:777` / typert `:2353` | 同上 | `Client.ForgeRetryWorktree` |
| | `stopCreation` | `:764` / typert `:2524` | 同上 | `Client.ForgeStopCreation` |
| | `restoreWorktree` | `:798` / typert `:2319` | 同上 | `Client.ForgeRestoreWorktree` |
| | `continueInProject` | `:829` / typert `:1615` | 同上 | `Client.ForgeContinueInProject` |
| 状态 2 | `track` / `dismiss` | `:253` / `:268` | `HOST:cmd/dshd/serve_forge_state.go` | `Client.ForgeTrack` / `ForgeDismiss` |
| rc.2 A 3 | `diffBases` | `:236` / typert `:1649` | `HOST:cmd/dshd/serve_forge_rc2.go:235` | `Client.ForgeDiffBases` |
| | `localCommits` | `:323` / typert `:1788` | `serve_forge_rc2.go:257` | `Client.ForgeLocalCommits` |
| | `worktreeCleanup` | `:210` / typert `:2763` | `serve_forge_rc2.go:330` | `Client.ForgeWorktreeCleanup` |

落点：`internal/apiclient/forge.go`（类型 + 14 个方法）、`testdata/api.snapshot.json`
**batch 21**（14 条，`counts.total` 83 → 97，形状逐条写进 `req_note` / `resp_note`）。

## 2. 画在哪（数据源 → 容器 → 渲染）

| 座位 | 官方注册 | 数据源 | 容器 | 渲染 |
|---|---|---|---|---|
| `conversation.hero.options` | `OFF:src/client/mount.ts:389-391` | `branches`（当前分支 / 分离 HEAD）+ 本壳的请求乐观态 | `cmd/app/forge_wiring.go`（`forgeState.worktree`） | `plugins/dsforge/hero.go`（`新建工作树` 勾选） |
| `conversation.input.dock` | `mount.ts:392-394`（官方这一格是 WorktreeProgress） | `localDiff{kind:'branch'}` | 同上（`forgeState.diff` + `branchRead`） | `plugins/dsforge/dock.go`（官方 `LocalChangesCard`） |
| `sidebar.right.pane.tab`(+`.title`) | `mount.ts:364-369`（key = 官方 `CHANGES_ID`） | `localDiff` 四个范围 / `localCommits` / `diffBases` | 同上 | `plugins/dsforge/changes.go`（Changes 视图的本地那一半） |

Shell 侧：`cmd/app/forge_wiring.go`（缓存 + 异步读 + 动作 + 会话/世代判定），
`Snapshot.Forge`（只读数据面）与 `Actions.Forge`（7 个动作）在 `pluginapi/forge.go`，
`cmd/app/main.go` 一处接线 + 一处 import。

## 3. 六条形状硬事实（都带出处，客户端据此写）

1. **五个 worktree 方法结果都是 `z.void()`**（`requestWorktree`/`cancel`/`retry`/`stopCreation`/`restore`/`continueInProject` 六个）。
   客户端签名是 `error`，不解析结果。
2. `track` / `dismiss` 官方也是 void，**本仓宿主多回 `{binding}`** —— 已知宽容偏差（DSH-GO gap 报告 G5），
   客户端按官方 void 处理；绑定内容的真相仍只由 `follow` 帧给。
3. `localText` 结果是**裸字符串**（`z.string()`），写成结构体会静默解成零值。
4. `branches.current` 是 **nullable**：分离 HEAD 时是 null（客户端用 `*string`）。
5. `localCommits.base` 与 `worktreeCleanup.revision` 都是**必填可空**：客户端显式发 `null`。
6. `ForgeLocalScope` 四支联合：`branch` 那支 `base`/`commits` **required+nullable**，
   其余三支 `additionalProperties:false` 只能有 `kind` —— 客户端 `forgeScopeWire` 逐 kind 造 wire，
   测试同时断言"该有的在"和"不该有的没有"（`internal/apiclient/forge_test.go`）。

## 4. 宿主限制（客户端据此**不画**）

**`localDiff` / `localText` 只认 `scope.kind`**：宿主把它解成 `struct{ Kind string }`
（`HOST:cmd/dshd/serve_forge_git.go:69-74`）再交给 `forge.Scope{Kind: …}`，所以 `branch` 范围的
`base`/`commits` 传了也被忽略 —— 分支范围**永远按自动分叉点**算。

于是界面：

- 只显示宿主回的 `base`（`HEAD → <base>`），**不提供**"选别的基准分支"的选择器；
- `diffBases` 只用来显示 `automatic`（官方 `diff.base.automatic` / `diff.base.worktree`）；
- `localCommits` 一律按自动分叉点读（避免"提交列表按 A 分支、diff 按另一个基准"）。

例外：`requestWorktree.from` 宿主**是真认的**（`internal/worktree` 真用它建分支），所以工作树勾选
按当前分支发 `request(currentBranch)` —— 这也正是官方 unchecked → checked 的默认语义
（`OFF:src/client/WorktreeToggle.tsx:150-153`）。

## 5. HIDE 清单（每条都写清"缺什么"，不是"不想做"）

| 官方组件 / 座位 | 缺的宿主能力 | 判据 |
|---|---|---|
| `WorktreeProgress` 创建卡（`conversation.input.dock` 官方那一格） | `worktree.creation`（创建进行到哪一步 `prepare`/`checkout`、`branch`、`held`）、`worktree.editable`/`blocker` | `internal/worktree.Service.State` **存在但没有任何客户端出口**；`forgePanel.follow` 流的是绑定清单帧 |
| `MissingCard`（工作树被移除 / 恢复中）——即"重试 / 停止 / 恢复回项目"三个动作的落点 | `worktree.created` / `worktree.missing` / `worktree.restoration` / `waiting` | 同上 |
| 归档清理 toast（`shell.overlay` 的 `ActionToast`，`mount.ts:395-397`） | GUI 没有"归档会话"这条交互（宿主有 `workspace.archiveSession`，界面从未接），官方那个 `workspace/archive-notice` 钩子没有投递点 | `grep archive plugins/ cmd/app/` 只有只读的 `archivedSessionIds` 清单 |
| 基准分支下拉菜单（`.base`，`WorktreeToggle.tsx:74-95`） | 本仓没有浮层菜单容器（`.menuRow`/`.menuList` 画在行内会顶掉布局） | `ui` 有 `Popup/Place*` 但本单不引新交互层；差异登记在 §6 |
| ForgeTab 本体（`TAB_ID`）、HeaderButton、会话行字形、`gh auth login` 横幅 | **G1**（`changeRequestStates` 官方是 `mode:'stream'` 零参数帧，宿主走 unary + 只认 env）、**G2**（`refresh`/`follow` 的 provider 是占位）、**G3**（凭据链三套互不相通） | 见 `docs/official-experimental3-evidence-rc2.md` §3 与 `#64b` |

新增一条宿主缺口（本单实测，已并入上面的判据）：

> **G6** `forgePanel.follow` **帧形状与官方不符**：官方结果类型是 `ForgePanelState`
> （`OFF:lib/typert.host.js:1752`，含 `directory/checkout/repository/readiness/worktree/local/changeRequests`），
> 本仓宿主流的是绑定清单帧 `{type:'baseline',value:{bindings}} / upsert / remove`
> （`HOST:internal/transport/handlers_forge_panel_state.go:300-345`）。
> 后果：**面板状态没有读出口**，工作树三张卡（progress / missing / toggle 的 blocker 层）无法真实渲染。
> 补法：宿主要么把 `follow` 改回官方 `ForgePanelState` 帧，要么在 `session.projections` 里发布
> `forgeWorktree` 投影（`internal/worktree.Fold` 的结果本来就在）。GUI 侧不需要再改。

## 6. 与官方的差异（逐条，全部登记）

1. **本地变更卡的座位不同**：官方在 ForgeTab 正文里（`OFF:src/client/ForgeTab.tsx:219`），
   本仓放在 `conversation.input.dock`（官方那格是 WorktreeProgress，本仓 HIDE）。理由：
   ForgeTab 一画就必须先画"本会话还没有变更请求"（`changes.empty`），而这句话在 G1 未解时**是假的**。
   卡片本体（结构 / 文案 / 几何）逐条照官方 `LocalChangesCard.tsx` + `ChangesTab.module.css:768-900`。
2. **范围菜单画在行内**（`.menuRow` 画成头部下方一列），官方是浮层菜单。
3. **没有文件树左栏**（`.treePane` / `--forge-tree-width`）：本仓没有可拖宽分栏视图；文件卡按官方
   `.file` 纵向堆叠（官方同样支持无树布局，`@container (min-width:760px)` 才显示树开关）。
4. **没有左右对比 / 换行开关**（`data-split` / `data-wrap`）：Viva 单元格宽度不可控。
5. **增删行不铺底**：官方用 `--dsw-alias-file-diff-*-bg/-gutter/-marker` 三档令牌，
   本仓 `pluginapi/theme.go` **没有**任何 diff 色令牌，只染符号与文字（+ 绿 / − 红）。
6. **刷新按钮是文字 `↻`**：官方那枚 `IconRefreshOutlineRegular` 在本仓只有
   `plugins/dsbrowser` 的**包内**常量（不跨包）；复制一份等于多一处会漂的官方坐标。
7. **分支图标不画**：官方 `IconBranchOutlineRegular` 含两枚 `<circle>`，viva 路径词汇只有
   M/L/Q/C/Z；素材库已登记为 `Unsupported`（`plugins/dsassets/registry.go:174-186`）。
   分支名用代码体文字表达。
8. **blocker 挡板缺失**：官方 `worktree.editable === false`（not-git / outside-writable-root / read-only）
   时整块不画勾选；本仓只能判 `checkout.state == 'ready'`（官方同样要求），
   "能读 git 但沙箱写不进去"那一层挡不住 —— 请求会被宿主拒绝并回滚（测试
   `TestForgeRequestWorktreePostsFromAndRollsBackOnFailure` 钉住了回滚）。
9. **没有"创建中"进度**：勾上之后只显示"已请求"，不画两步进度（理由同 §5 第一行）。
10. **已读标记只在本进程内存**：官方存 `localStorage`（`viewed-files.ts`），本仓插件内存；
    三态（未读/已读/已变更，按 digest 比）与官方一致。
11. **`diff.omitted` / 二进制 / 过大**三种提示照官方文案；被略掉的文件**不计入** `N 个文件`。

## 7. 验收

```
CGO_ENABLED=0 go build ./...                                  → 0
go vet ./...                                                   → 0
gofmt -l cmd internal plugins pluginapi                         → 空
go test ./... -count=1                                         → 全绿（见 §8 汇总）
python scripts/parity-count.py --check                         → PASS（66 · ✅50/🟡12/❌4）
python scripts/doc-links.py --check                             → PASS
python scripts/scripts-index.py --check                         → PASS
```

本单新增测试：

| 文件 | 钉住什么 |
|---|---|
| `internal/apiclient/forge_test.go` | 14 个方法的 **wire 形状**（scope 逐 kind、`from`/`base`/`revision` 显式 null、`track/dismiss` 忽略 binding、`localText` 裸字符串、`worktreeCleanup` null 解成 nil）+ 官方错误码原样留下 |
| `cmd/app/forge_wiring_test.go` | 对着真 HTTP：checkout 三态、本地摘要来自 branch 范围、范围切换只发 kind、未知范围不发请求、工作树请求的 `from`、**失败回滚勾选态**、换会话后旧回包丢弃 |
| `plugins/dsforge/forge_test.go` | 座位（槽/key/标签/order 逐条对官方注册）、**可见性闸门**（无数据面/会话不匹配一个字不画）、已读三态、默认展开、官方文案逐字、格式化 |
| `plugins/dsforge/visual_test.go` | **真实像素帧**（headless + pump 到条件成立）：三块座位同时画出的官方文案、命中区可点、以及两条否定判据（无数据面/换会话时不画） |

## 8. 缺口（交给 #64b 与宿主线）

1. **G6**（本节新增）：`follow` 帧形状 = 绑定清单，不是 `ForgePanelState` → 工作树三张卡无数据源。
2. **G1/G2/G3**（原样）：CR 卡片、会话行字形、`gh auth login` 横幅仍 HIDE。
3. **`localDiff`/`localText` 不认 `base`/`commits`**：官方"选基准分支"这一整块交互在宿主补齐前做不了。
4. **GUI 无归档入口**：`workspace.archiveSession` 宿主有、界面从未接，所以 `worktreeCleanup`
   已有客户端与测试但没有界面落点（`forgeCleanupAction` 只给测试/未来的归档入口用）。
5. **`branches` 不回 commit**：官方 `checkout.commit`（分离 HEAD 时的 `worktree.detachedTip`
   `{commit}`）因此为空，分离 HEAD 只能显示"未在分支上"。
