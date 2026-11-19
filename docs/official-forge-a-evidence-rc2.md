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

## 9. 2026-11-19 复核（task-103：只做不依赖宿主的那一档）

先纠正 **§6 里三条已经过期的差异**——它们不是"没做"，是账本没跟着代码更新：

| §6 条 | 原文结论 | 现在的事实（带出处） |
|---|---|---|
| 5 | "本仓 `pluginapi/theme.go` 没有任何 diff 色令牌" | **不成立**：`pluginapi/theme.go:132-142` 有六档 `FileDiff{Added,Deleted}{Bg,Gutter,Marker}`；取值 `plugins/dscore/dscore.go:227-232`（暗）/`:329-334`（亮），逐条注释 `design-platform.css:281-286`/`:433-438`；消费 `plugins/dsforge/changes.go:602-640`（`.add`/`.del` 铺 bg、`.number` 铺 gutter 且文字用 marker）；双钉测试 `plugins/dsforge/diff_colors_test.go:26-37` |
| 6 | "刷新按钮是文字 `↻`" | **不成立**：`plugins/dsforge/changes.go:336-346` 画的是 `dsassets.MarketRefreshOutline`，15px（官方 `.THWGDq_tool svg{width:15px;height:15px}`，`OFF:lib/client.js:4466` 的 ChangesTab 模块 CSS）；官方素材 viewBox `0 0 16 16`、`stroke-width` 由调用方给（`OFF:index-ghOpC9uV.js:129`，sha256 `061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8`）；路径逐字在 `plugins/dsassets/market.go:15-20` |
| 2 | "范围菜单画在行内（`.menuRow`），官方是浮层" | **位置已改**：现在用 `ui.Menu` 锚在范围芯片下方（见下）；另纠正一处**认错类名**：官方 `.menuRow`/`.menuList`/`.menuPath` 在 ChangesTab 模块 CSS 里**没有任何使用点**（`grep menuRow lib/client.js` 只有 CSS 与 class map 两处），不是范围菜单的样式 |

### 9.1 本轮唯一的代码改动：范围菜单改成官方浮层

官方 `ScopeMenu`（`OFF:lib/client.js:6436-6479`）是 `primitives.Menu`：
`portal: true` / `align: "start"` / `dense` / `autoFocus` / `selectedId`，`anchor` 就是那个
`.select` 按钮；条目顺序 **未提交 → 未暂存 → 已暂存 → 分隔线 → 分支**。

本仓 `plugins/dsforge/changes.go` 的 `drawScopeMenu` 改为 `ui.Menu(f, anchor, …)`：
锚定浮层（`popup.BelowBox(anchor, 4, Start)`，`viva@v0.6.4/ui/menu.go:126-131`）、
点外部/Esc 关、键盘可走、选中项 `Chosen` 打勾（`ui/menu.go:465-473`）；
条目顺序与分隔线按官方，选中动作走新加的 `chooseScope`（测试接缝，同
`plugins/dscreationreco` 的 `chooseLocation`）。**不再**把菜单画成头部下方的一列。

判据（`plugins/dsforge/visual_test.go`）：

- `TestScopeMenuIsAnOverlayWithOfficialOrder` — ① 打开菜单时 hunk 头的 Y **一格不动**
  （行内菜单会从 154 推到 266，实测变异红）；② 关着时不出现菜单条目；
  ③ 打开后四条按官方顺序出现。
- `TestChooseScopeSendsTheScope` — 选中哪个发哪个；`env.Act.Forge == nil` 时静默返回不崩。

**仍差一档（新登记，替代原"位置不同"）**：官方菜单面是 `--dsw-elevation-prominent`，
Viva 的浮层固定 `gfx.ShadowMD`（`viva@v0.6.4/components/dropdown_menu.go:769-771`），
且官方 `dense` 的行高/内边距在可用证据里取不到（`primitives` 包源码不在
本机的官方包 dump 里）。手绘面板能换回官方阴影但会失去浮层，两害相权保留浮层。

### 9.2 分支图标：素材有、渲染词汇没有（不是"缺素材"）

- 官方素材：`assets/official/icons/messages/branch.jsx`（sha256
  `5090854ac51aa941cdd4d84ce37a39df0bb36638f90ab334af7a642f3558798b`，605 B，
  `viewBox=0 0 16 16`）= 两条支线 `<path>` + **两枚 `<circle>`**
  （`cx 12.4502 / cy 3.3079` 与 `cy 12.6921`，`r 1.56962`）。
- 挡在哪：Viva 的路径词汇只有 `Move/Line/Quad/Cubic/Close`（`gfx/path.go:9-12`），
  没有弧，也没有圆的图元可放进 `gfx.Icon`；`plugins/dsassets/registry.go:170-182`
  因此把它登记成 `Unsupported`（"圆需要椭圆弧"）。
- 结论：**不画**。官方两条支线的原文已在 `plugins/dsassets/paths.go:51-52`，
  但缺了两枚端点圆的分支图标是残图，比不画更糟——`gfx.Icon` 里补圆要先有弧。
  分支名继续用代码体文字表达（`plugins/dsforge/hero.go` 的 `基于 X`）。

### 9.3 第二档（会动布局）的评估结论：这一轮不做

| 差异 | 官方要什么 | 卡在哪 |
|---|---|---|
| 文件树左栏 | `FileTreePane.module.css`（`OFF:lib/client.js:5902` 起）：`.pane` 只在 `@container (width>=760px)` 里 `display:block`；宽度**可拖**（`MIN_WIDTH=200` / `MAX_WIDTH=600` / `DIFF_MIN_WIDTH=320`，`OFF:lib/client.js:5919-5921`），拖手 `.resize{width:8px;cursor:col-resize}` | ① Viva 没有**容器查询**（`ui.C` 只有整页尺寸，`pluginapi.Env.Geometry` 给的是侧栏/主列宽度）；② 仓内与 Viva 都没有**可拖宽分栏**（`grep SplitPane\|WidthHandle\|Resizable` 零命中，`.resize` 这种 8px 抓手的拖动/光标要自建） |
| 左右对比 / 换行 | `.code[data-split]` 的双 `.cell` 网格（`.pair{grid-template-columns:minmax(0,1fr) minmax(0,1fr)}`、`.cell+.cell{border-left:.5px}`）每格自带行号列且**各自横向滚动**（`.code{overflow-x:auto}` + `white-space:pre`），`data-wrap` 换 `pre-wrap`+`overflow-wrap:anywhere` | Viva 的横向滚动只有 `ui.List` 的 `RowWidth`（整行滚），没有"格内独立横向滚动 + 其余列固定"；两态并存（`data-split`/`data-wrap`）要求同一棵子树两种宽度语义 |

结论：两条都要先有渲染器能力（容器查询 / 可拖宽分栏 / 格内横向滚动），本单不动布局。
按官方形状"缺就 HIDE"而不是画一个假的树/假的分栏。

### 9.4 第三档：宿主具体缺什么（GUI 侧不再改）

**G6 仍然成立，且已用宿主当前源码复核**：`forgePanel.follow` 发的第一帧是
`{type:'baseline', value:{bindings}}`（`HOST:internal/transport/handlers_forge_panel_state.go:318-321`），
之后是 upsert/remove 的绑定清单帧；官方结果类型是 `ForgePanelState`
（`OFF:lib/typert.host.js:1752`，含 `directory/checkout/repository/readiness/worktree/local/changeRequests`）。
宿主自己那份缺口表也写着这条（`HOST:internal/transport/handlers_forge_liveshare_official.go:295-302`：
"本仓没有 panel 状态机；开一条只发空帧的流就是假流式"）。

要让下面两张卡落地，宿主二选一：

1. 把 `follow` 的帧改成官方 `ForgePanelState`（首帧全量 + 后续 union）；**或**
2. 在 `session.projections` 里发布 `forgeWorktree` 投影——`internal/worktree.Fold` 的
   `Service.State` 本来就在，缺的只是出口。

依赖它的两条（宿主补齐即可真画，GUI 侧不需要再改）：

- **blocker 挡板**（§6 条 8）：官方 `worktree.editable === false`（`not-git` /
  `outside-writable-root` / `read-only`）时整块不画勾选。本仓现在只能判
  `checkout.state == 'ready'`，"能读 git 但沙箱写不进去"那一层判不了——`worktree.editable`
  正是 `ForgePanelState.worktree` 里的字段。
- **创建进度卡**（§6 条 9）：官方 `WorktreeProgress` 要 `worktree.creation`
  （`prepare`/`checkout` 到哪一步）、`branch`、`held`；`internal/worktree.Service.State`
  有这些阶段，缺的同样是读出口。

**已读只在本进程**（§6 条 10）不是宿主缺口：官方存 `localStorage`（`viewed-files.ts`），
本仓插件内存；三态（未读/已读/已变更，按 digest 比）与官方一致。要做持久化得先有
插件级存储 seam，属另一单。

### 9.5 复核后的剩余清单（10 → 8）

10 条里 **3 条本轮/前一轮已消除**：`无 diff 底色令牌`（2026-11-17 已落地）、
`刷新图标`（已落地）、`范围菜单位置`（本轮改成浮层）。剩下 **8 条**：

| # | 剩余差异 | 归属 |
|---|---|---|
| 1 | 座位（`conversation.session.header.utilities` 的"创建变更请求"按钮 + `shell.overlay` 的归档清理 toast） | CR 面（G1/G2/G3）与 GUI 无归档入口 |
| 2 | 文件树左栏（可拖宽 200–600，容器 ≥760 才显示） | 渲染器（容器查询 + 可拖宽分栏） |
| 3 | 左右对比 + 换行（`data-split`/`data-wrap`） | 渲染器（格内横向滚动） |
| 4 | 分支图标（两枚圆） | 渲染器（路径词汇无弧） |
| 5 | blocker 挡板 | 宿主（G6：`worktree.editable`） |
| 6 | 创建进度卡 | 宿主（G6：`worktree.creation`） |
| 7 | 已读只在本进程 | GUI（缺插件持久化 seam） |
| 8 | 范围菜单面阴影/密度 | 渲染器（`gfx.ShadowMD` 写死；官方 elevation-prominent） |

## 10. 2026-11-19 复核 + 收口（task-105：接上门禁 ⑤⑥）

**前置已就位**：宿主 `dsh-go#84`（`0081922`）把 `forgePanel.follow` 从"绑定清单帧"改成
官方 `ForgePanelState` 整帧流——`HOST:internal/transport/handlers_forge_panel_state.go:574-631`
（NDJSON，头在首帧之前发；流内错误帧 `{"error":{code,message}}`）+
`HOST:cmd/dshd/serve_forge_state.go:239-300`（`worktreeState` 九格：
`editable = !TurnStarted && Created == nil`、`creation` 取 `worktree.Service.Progress`）。
§8/§9 里的 **G6 因此关闭**，上一节表里的第 5、6 两条解除。

### 10.1 两条差异的官方出处（逐条 file:line）

**⑤ blocker 挡板 —— 官方 `WorktreeToggle`**

| 判据 | 官方 |
|---|---|
| `!worktree.editable` 或 `blocker === 'not-git'` 或 `checkout !== 'ready'` → 整块不画 | `OFF:…/src/client/WorktreeToggle.tsx:97-109` |
| `usable = blocker === null`；`disabled = held || pending !== null || (!usable && !requested)` | `:128-129` |
| 芯片文案（`worktree.currentBranchTip` / `worktree.base` / `worktree.baseHead` / `worktree.detachedTip`） | `:130-133` |
| 浮条＝`blocker === null ? 'worktree.hint' : 'worktree.blocked.<blocker>'`，`delayMs={500}` | `:151` |
| `<label data-disabled>` + `<input type=checkbox disabled>`；`.toggle[data-disabled]{color:label-dimmed}` | `:152-161`、`WorktreeToggle.module.css:38-41` |
| 文案（`worktree.hint` / `worktree.blocked.outside-writable-root` / `worktree.blocked.read-only`） | `OFF:…/src/client/locales.ts:310-312` |

**⑥ 创建进度卡 —— 官方 `WorktreeProgress`**

| 判据 | 官方 |
|---|---|
| 只在 `worktree.creation !== null` 时画（`creationOf`：creation→running；其余两支本轮不画） | `OFF:…/src/client/WorktreeProgress.tsx:252-280`、`:105-126` |
| 卡片：flex column、`gap 6`、`max-width:card-max-4*inset`、`padding:10px 8px 6px 12px`、`radius md`、`bg:--dsw-specific-menu`、`box-shadow:--dsw-elevation-panel`、`13px/20px` | `WorktreeProgress.module.css:17-52` |
| 头部：`gap 8`、`padding-right 4`；标题 `font-weight:500`；分支 `code 12px` + 省略号 | `:54-86` |
| 步骤：`column; gap 2; padding-left 22`；`.step{gap 8; min-height 24; label-secondary}`、running→`label-primary` | `:88-111` |
| 步骤字形：done=`IconCheck`(14, success)、running=`StateDot(ongoing,14)`、waiting=14×14 空环 `1.5px label-dimmed` | `WorktreeProgress.tsx:82-91`、`…module.css:113-134` |
| 两步：`CREATION_STEPS = [prepare 创建分支, checkout 检出文件]`；`stepState` = current→running / 更早→done / 更晚→waiting | `WorktreeProgress.tsx:45-80` |
| 详情（`open` 时）：`grid auto/minmax(0,1fr)`、`gap 2 12`、`margin 2px 0 0 22px`、`12px/18px` | `:109-128`、`…module.css:136-152` |
| 页脚：`height 28`、`.disclosure{gap 4; padding 0 6; 12px}`、展开时箭头 `rotate(90deg)` | `…module.css:161-204` |
| `creation.held` 才画「取消」（ghost sm + close 图标 14，动作 `stopCreation`） | `WorktreeProgress.tsx:163-181`、`:176-180` |
| 文案（创建中/两步/更多详情/分支/基于/当前 HEAD/取消） | `locales.ts:319-332` |

### 10.2 GUI 落点

| 层 | 文件 | 内容 |
|---|---|---|
| 客户端 | `internal/apiclient/forge.go` | `ForgePanelFollow`（NDJSON 逐行整帧；错误帧→四字段；非 `application/x-ndjson` 直接报错）+ `ForgePanelState`/`ForgePanelWorktree`/`ForgeWorktreeCreation` |
| 数据面 | `pluginapi/forge.go` | `ForgeWorktree.{PanelKnown,Editable,Blocker,Creation}` + `ForgeWorktreeCreation` + 三个 blocker 常量 + `ForgeCreation{Prepare,Checkout}` + `ForgeActions.StopCreation` |
| 壳 | `cmd/app/forge_wiring.go` | `forgeFollow`（每会话一条流、世代守卫、断线 2s 重连、`not_implemented`/`bad_request` 不重试）、`forgeWorktreePanel`（纯函数投影）、`forgeStopCreationAction`；`cmd/app/main.go` 接动作面 |
| 挡板 | `plugins/dsforge/hero.go` | 官方三层可见性 + `disabled` 判据 + 浮条（`ui.Tooltip`）+ 禁用态 `label-dimmed`；点击按 `disabled` 屏蔽 |
| 进度卡 | `plugins/dsforge/progress.go` | 官方几何/文案/两步态/展开详情/held 取消；座位 `conversation.input.dock` **order 4**（本地卡仍占官方 order 5，同槽不许同序） |

### 10.3 判据（三条都做了变异验证）

| 测试 | 断言 | 变异（实测变红） |
|---|---|---|
| `plugins/dsforge/progress_test.go` 的 `TestProgressFollowsCreationPhase` | 只把 `creation.phase` 从 prepare 换成 checkout：`创建分支`/`检出文件` 两行颜色对翻，checkout 帧出现绿色 done 勾 | `stepState` 改成恒按 prepare 判 → 红（checkout 帧「创建分支」仍是 label-primary） |
| 同上 `TestBlockerBaffleHidesToggleWhenNotEditable` / `…KeepsToggleDisabledWithOfficialCopy` | `editable=false`、`blocker=not-git` 不画；两块 blocker 照画但 `label-dimmed`、浮条逐字、点击零动作（`request=0 cancel=0`） | 可见性判据短路 → 红（editable=false 仍画了勾选框） |
| `cmd/app/forge_wiring_test.go` 的 `TestForgeFollowDrivesEditableAndCreation` | 推帧 `editable true→false`、`creation prepare→checkout→null`，数据面逐帧跟着变 | `forgeWorktreePanel` 把 `Editable` 写死 true → 红（creation 在跑时仍说能建） |
| `internal/apiclient/forge_test.go` | 三帧整帧流（含 `blocker:read-only`、`creation.prepare/checkout`、`from:null`）、流内错误帧、非 NDJSON 拒绝 | —— |

### 10.4 宿主侧一处如实缺口（不擅改）

`worktree.blocker` 今天**恒为 null**：宿主 `worktreeState`（`HOST:cmd/dshd/serve_forge_state.go:236-300`）只填 `editable` / `request` / `created` / `creation` / `failure` / `missing` / `restoration` / `waiting`，没有官方 `panel.ts:320-340` 的 `blockerOf()` 三条探测（沙箱能否写 Git 目录 / 是否在 checkout 里 / 会话是否只读）。

后果：界面上官方 `:103`（`!editable` → 整块不画）这一条**真的生效**，而 `:104-105` + `:128-129` + `:151`（挡着时画出来但禁用、浮条换 `worktree.blocked.<blocker>`）**今天不可达**，因为宿主永远不会给出那两个值。两块挡板文案、禁用判据与用例都已按官方写好，宿主填值即生效；这条缺口归宿主线（本仓不擅改），不重复计进 GUI 的剩余差异。

### 10.5 剩余差异：8 → 6 条

消除 5、6 两条。**仍剩 6 条：1 座位 / 2 文件树左栏 / 3 左右对比+换行 / 4 分支图标 / 7 已读只在本进程 / 8 范围菜单面阴影与密度**
（编号沿用 §9 表格，不重排）。

本轮**没有**动、但值得记一笔的三支：`worktree.failure`（失败态）、`worktree.missing`（MissingCard）、
`worktree.restoration`（恢复进度）——宿主 follow 帧里都已有真值，但本轮任务只接 `creation`，
这三支仍整块不画（不画假卡），留给后续任务。
