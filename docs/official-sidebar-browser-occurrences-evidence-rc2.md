# #50 浏览器多实例与恢复证据（rc.2）

> 2026-11-16 · 工作分支 `feat/gui-browser`。本证据只收 occurrence / 多实例 / 关闭 / 恢复；
> `window.open/new-window`、下载、页面生命周期与两处小视觉差异仍保留在账本，不把 #50 冒升 ✅。

## 1. 官方依据

| 官方行为 | 原件 |
|---|---|
| Browser 类型允许多个并存 | `dsh-client-ui-sidebar-browser/src/client/definition.tsx:15-25`，尤其 `:19 multiple:true` |
| 每个 occurrence 有自己的 `tabId`、初始 checkpoint、page factory 与 restore 命令 | `browser/BrowserController.ts:20-29,205-228,260-265` |
| mount 缓存按 `tabId` 隔离，切走可以保留，关闭先 dispose 再 forget | `browser/BrowserController.ts:176-197,224-228` |
| 浏览器 store 是 Session 作用域的 `byTab`，持久化 namespace 为 `dsh.sidebar-browser.v1` | `browser/store.ts:6-33` |
| 每条保存记录含 `entries/index/request/navigation/failure` | `browser/BrowserPersistence.ts:17-24` |
| 标题从当前 saved entry 取，否则回退类型标题 | `view/BrowserTitle.tsx:11-17` |
| 启动只把 saved 当 `initial`；恢复卡显示 title/url，用户点「恢复页面」才 `restore(tab.id)` | `view/BrowserBody.tsx:51-59,118-125` |
| 同地址提交走 reload，不重复追加历史 | `browser/IframeImpl.ts:50-56,84-103` |
| `multiple:false` 同 pane 复用，`multiple:true` 每次 open 都 mint 新 tabId | `dsh-client-ui-sidebar-right/src/client/service.ts:519-539` |
| 右栏布局本身也持久化，并在 store create 时恢复 | `dsh-client-ui-sidebar-right/src/client/stores.ts:512-521`、`persistence.ts:63-90` |

## 2. 本仓落点

### 2.1 occurrence 与多实例

- `pluginapi.Painter.Multiple` 把官方类型声明带进名录；`Tab.Key` 是 occurrence 身份，`Tab.Kind` 才是 keyed 派发类型。
- `cmd/app/right_tab_occurrences.go` 每 Session 保存有序 occurrence、`activeTabId` 与单调 `minted`；浏览器每次 `OpenRightTab("browser")` 新建，timeline 等单实例类型复用。
- `internal/uikernel/view_tabs.go` 芯片行有真实「+ / ×」；关闭先走 `RightTabCloseHandler`，失败保留 tab，不静默删布局。
- `plugins/dsbrowser` 按 `(sessionId,tabId)` 保存独立 `BrowserNavigation`、`WebPage`、地址草稿、错误与 late-load 状态；切走只撤销该页的 `NativeView` 声明，不销毁其他页。
- `RightTabTitleProvider` 只改写自己 occurrence 的芯片标题；其他类型返回空串，避免串名。

### 2.2 两层恢复

1. `dsh.sidebar-right.v1.<sessionId>`：恢复有哪些浏览器 occurrence、顺序和选中项。全关后的空布局是合法保存态，不重新播种。
2. `dsh.sidebar-browser.v1.<sessionId>`：按 `byTab` 恢复每个 occurrence 的 `entries/index/request/navigation/failure`。地址、索引、revision、枚举与 URL 白名单都校验；坏数据明确提示且不自动开页。
3. 本仓没有浏览器 `localStorage`，两个 namespace 落在既有宿主 `ui-config.json`，仍走 `Actions.ConfigGet/ConfigSet`，未新增宿主 API 方法。
4. 新进程只重建应用记录，`WebHost.Open` 次数保持 0；页面显示官方「上次打开 / title / URL / 恢复页面」卡。显式点击后才创建 WebView，revision 单调 +1，历史不重复追加。
5. 关闭 occurrence 先 `WebPage.Close`，再 `forget(tabId)` 与删布局；即使这个恢复 tab 从未绘制过，也不会用空内存覆盖其他 tab 的 checkpoint。
6. `config.get` 的 nil map 只表示「尚未读到」，不是空配置。到达前不写回；新建/关闭明确提示稍后重试，避免旧 `tab<N>` 冲突；浏览器在配置前产生的当前修改会与旧 `byTab` 合并，而不是互相覆盖。

## 3. 机器断言

- `plugins/dsbrowser/persistence_test.go`：两 occurrence 的 URL/历史/地址/NativeViewID 独立；tab1 晚到 load 不推进 tab2；全新 Plugin 恢复不自动 Open；显式 restore 不重复历史；关闭 forget；未绘制 tab 关闭仍保留其他 checkpoint；坏 JSON 有提示。
- `TestRestoreCardReallyClickOpensSavedPage`：无头窗口 pump 到恢复卡出现，点击「恢复页面」，再 pump 到 `Open==1` 才断言。
- `cmd/app/right_tab_occurrences_test.go`：多开、单实例复用、选择、关闭、重开新 tabId、全新 model 恢复布局、全关后保持空、空态重开；配置未到时新建/关闭明确等待，到达后恢复再新建。
- `TestDelayedBrowserConfigMergesInsteadOfOverwritingSavedTabs`：nil 配置时零写回，当前输入不被旧值覆盖，旧 tab 不丢，合并后写回完整 store。
- `internal/uikernel/view_tabs_test.go:TestDrawRightTabsCreateAndCloseButtonsReallyClick`：真实点击 Hit 标签「新建浏览器标签页」「关闭」，pump 到动作收到 `browser` / `tab1`。
- 既有 URL、100 条历史、前进分支、loading→known→unknown、工具条、NativeView 与两座位测试未删、未放宽。

## 4. 仍未收口的差异

- `window.open/new-window`、下载与完整页面生命周期仍按 #50 原账保留；本轮没有把系统浏览器跳转伪装成应用内 popup。
- 「前往」仍常显（官方 `:focus-within` 才显示）；地球图标赤道线线帽仍不同。
- 本仓布局是单 pane 子集，不含 dockkit 分屏、浮动、undo/redo；guide tab 未实现，因此「+」直接新建浏览器 occurrence，而不是先打开官方 guide。
- 桌面官方还按 workspace 选择存储分区；本仓沿用既有 WebView 默认 profile，本轮未新增分区。

## 5. 全量门禁（本 worktree 实测）

| 命令 | 结果 |
|---|---|
| `python scripts/parity-count.py --check` | exit 0；66 = ✅51 / 🟡13 / ❌2，PASS |
| `CGO_ENABLED=0 go build ./...` | exit 0，无输出 |
| `go vet ./...` | exit 0，无输出 |
| `CGO_ENABLED=0 go test ./... -count=1` | exit 0，0 FAIL |
| `test -z "$(gofmt -l cmd internal plugins pluginapi)"` | **单独执行 exit 0，无输出** |
| `python scripts/doc-links.py` | exit 0；478 条仓内链接坏 0，PASS |
| `python scripts/scripts-index.py` | exit 0；23 个脚本均有登记，PASS |

原始日志（追加延迟配置修复后的复跑）：`DSH-GO/tmp/GO8-RC2/_br/final-gate-{parity,build,vet,test,gofmt,doc-links,scripts-index}.log`。
