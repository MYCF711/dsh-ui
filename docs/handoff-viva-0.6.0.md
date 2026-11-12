# Viva 引擎升级交接：`v0.2.3` → `v0.6.0`

> 基线：`origin/main`（`545a2e2`）的 `github.com/vivaui/viva v0.2.3`，上游最新 **`v0.6.0`**（2026-11-11）。
> 隔离 worktree：`../dsh-ui-viva`，分支 `chore/viva-0.6.0`（从 `origin/main` 拉出，未 commit）。
> 上游原文一律取自模块缓存里的**仓库原文** `$(go env GOMODCACHE)/github.com/vivaui/viva@v0.6.0/docs/releases/*.md`
> 与 `docs/upgrading.md`（不是 GitHub Release 页面转述）。

---

## 0. 结论先行

| 阶段 | 版本 | 状态 | 依据 |
|---|---|---|---|
| **阶段一** | `v0.2.3` → `v0.5.1`（跨 v0.2.4/v0.3.0/v0.3.1/v0.4.0/v0.5.0/v0.5.1） | **已落地，全绿** | build/vet/test/race/gofmt/守卫脚本/真机截图全过（§3、§6、§7） |
| **阶段二** | `v0.5.1` → `v0.6.0` | **未落地，只做清点** | v0.6.0 不是版本号跃迁，而是**编写模型重写**（§4） |

阶段二规模（实测计数，不是估计）：

| 面 | 影响面 |
|---|---|
| `ui.Frame`/`View` 编写模型（旧 `flex.Frame`/`flex.View`） | 192 个文件 import viva；112 处 `*flex.Frame` 形参；≈1000 处 frame 方法调用；109 处 `flex.View` |
| 包合并 `flex` → `ui` | **139 个文件** import `github.com/vivaui/viva/flex`，v0.6.0 里**该包已不存在** |
| 包拆分 `ui` → `gfx` | **81 个 `ui` 类型没了**（80 个在 `gfx` 同名）；本仓用到其中 **74 个标识符 / 1225 处 / 144 个文件**；另有 **9 个名字从类型变函数**（见 §4.2） |
| 控件身份与绑定值改写 | Button 22、Field 7、Switch 1、Select 2、ScrollArea 10、Scroll 8、Popup 1、End 5（需逐点语义改写，不是改名） |

一句话：**v0.6.0 的迁移量约等于把全仓 UI 声明层重写一遍**，按本仓"全绿才合"的门禁，它不是一次版本 bump 能收口的活；本文件把阶段一交付掉、把阶段二拆成可下发的清点。

---

## 1. 上游原文：哪些版本有 breaking

`@v0.6.0/docs/releases/` 逐版自述（**仓库原文**）：

| 版本 | 上游自述 | 对公开 API 的影响 |
|---|---|---|
| v0.2.4 | "公开 API 只有新增，没有删除或改签名" | 无 |
| v0.3.0 | "`components` 包也移除了几个成员" | `components.CodeEditor` 重建（`Layout` 字段、`TextareaSnapshot` 嵌入消失） |
| v0.3.1 | "公开 API 只有新增" | 无 |
| v0.4.0 | "含一处不兼容改动：`flex.Box.HoverBackground` 已删除" | `HoverBackground` → `Hover(Look)` |
| v0.5.0 | "公开 API 只有新增，没有删除或改签名" | 无（但有行为变化：`RoundedBorder` 零填充、终端单元按字体度量） |
| v0.5.1 | "公开 API 只有新增" | 无 |
| v0.6.0 | "Page 成为唯一的公开界面编写模型，`ui.Frame` 与 `ui.View` 不再导出，旧的 View 宿主拖动 API 也移除了，所以升级需要改调用点" | **编写模型重写**（本文件 §4） |

> ⚠️ 上游 `docs/upgrading.md`（跨版本升级指南）自称"从 v0.2.2 到最新，只有两处删/改公开成员"（v0.4.0 的 `HoverBackground` 与 v0.3.0 的 `components`）——**这句对 v0.6.0 不成立**：
> 该指南没有提到 `flex` 包整体并入 `ui`、也没有提到 `ui` 的绘图/事件/主题标识符（`Canvas`/`Rect` 等）拆到新包 `gfx`。
> 这是上游**文档缺口**（不是我们的误读：v0.6.0 tag 里的 `flex/` 目录确实不存在，`go get github.com/vivaui/viva/flex@v0.6.0` 直接报 `does not contain package`）。
> 按本仓 `AGENTS.md` §0 的贡献边界，这属于"文档修复、可小改直提 PR"的一类；**对外动作需用户当次明确授权**，本轮未发。

---

## 2. 阶段一（v0.2.3 → v0.5.1）breaking 清单

### 2.1 `flex.Box.HoverBackground(ui.Color)` 删除 → `Hover(flex.Look{}.Background(c))`

| 项 | 内容 |
|---|---|
| 旧写法 | `row.HoverBackground(t.Hover)` / `.HoverBackground(t.Hover).Hit(id, "")` |
| 新写法 | `row.Hover(flex.Look{}.Background(t.Hover))` |
| 影响 | **19 处 / 13 个 .go 文件**（另有 go.mod、go.sum 各 1 处） |
| 是否已改 | **已改**（`git diff --stat`：15 files changed, 22 insertions, 23 deletions） |

改动文件（`git status --porcelain`）：

```
plugins/dsagentteam/model_slot.go        plugins/dscomposer/composer.go
plugins/dscomposer/official_seat.go      plugins/dsconversation/conversation.go
plugins/dsconversation/feedback.go       plugins/dscore/primitives.go
plugins/dsrightbar/rightbar.go           plugins/dsrightbar/wpd1.go
plugins/dssettings/official_panel.go     plugins/dssidebar/frame.go
plugins/dssidebar/sidebar.go             plugins/dstool/dstool.go
plugins/dstopbar/topbar.go               go.mod  go.sum
```

注意两种书写形态都要处理：`.HoverBackground(x)` **紧跟链式调用**、以及换行后**行首独立** `HoverBackground(x)`（本次 19 处里有 5 处是后者）。

### 2.2 `components.CodeEditor`（v0.3.0 重建）

- 旧：`CodeEditorSnapshot` 嵌 `TextareaSnapshot`，可用 `Layout`、`NoWrap`、`NativeEditing` 等从快照提升上来的字段。
- 新：字符网格底座；移动/定位/滚动改为 `CodeEditor.Move/OffsetAt/Reveal/ScrollBy/Scroll/ScrollY/SetScrollY`；`NoWrap` 换成反向的 `Wrap`。
- **影响本仓：0 处**（`grep -rn "CodeEditor" --include=*.go` = 0）。无需改动。

### 2.3 `go.sum` 变化（阶段一）

```
- github.com/vivaui/viva v0.2.3 h1:... / v0.2.3/go.mod h1:...
+ github.com/vivaui/viva v0.5.1 h1:c6xHo0yKsMDEYfVt9t70dpSDoK7dXqF3DtfD2sVcuSI=
+ github.com/vivaui/viva v0.5.1/go.mod h1:iXOoCNuLlzKuz3cLna2y5bgXAFY8eLJSld6narLbpk4=
```

依赖面不变：仍只有 `github.com/vivaui/viva` + `github.com/ebitengine/purego v0.11.1`（indirect）。**本仓仍 `CGO_ENABLED=0` 可构建，未引入 cgo**（Viva 走 purego 运行期 `dlopen`）。

### 2.4 阶段一附带的行为变化（编译不报，但要写进验收）

| 变化 | 对本站的影响 | 判定 |
|---|---|---|
| v0.5.0 `RoundedBorder` 零填充不再画实心底 | 我们的徽标/输入框都是"先 Background 再 Border"或只描边，headless 布局与视觉测试全绿（§6） | 无回归 |
| v0.5.0 终端单元按字体自身 advance/行盒（不再是固定 0.6em×1.18em） | `plugins/dsterminal` 的存活测试通过；终端列数由字体度量决定 | 已核实，无需改 |
| v0.3.0 隐藏/最小化/被遮挡窗口仍收 `OnWake` 但不绘制、不提交帧 | 我们是事件驱动重绘（不靠持续 `Invalidate` 刷新），不影响 | 无回归 |
| v0.3.0 热重载协议版本提升（host/worker 必须同版本） | 本仓热重载 worker 与宿主同模块构建 | 无回归 |
| v0.5.0 `ui.Canvas.HitBounds`、`ui.State` 等新增 | 未使用，不引入 | — |

---

## 3. 阶段一的门禁输出（全绿）

`bash` 里逐条跑，`> log 2>&1; echo $?` 取真实退出码（不用 `| head` 偷状态）：

| 门禁 | 命令 | 退出码 |
|---|---|---|
| 构建 | `CGO_ENABLED=0 go build ./...` | **0** |
| 静态检查 | `CGO_ENABLED=0 go vet ./...` | **0** |
| 单测 | `CGO_ENABLED=0 go test ./... -count=1` | **0**（35 个包 ok，0 FAIL） |
| 竞态（MinGW `gcc 16.2.0`，`/c/toolchains/mingw64`） | `CGO_ENABLED=1 go test -race ./... -count=1` | **0**（`DATA RACE` = 0） |
| 格式 | `gofmt -l .` | 0 个文件 |
| 守卫自检 | `bash scripts/guard-selftest.sh` | **0**（`OK=8 FAIL=0`） |
| 部件口径 | `python scripts/parity-count.py --check` | **0**（PASS） |
| 文档链接 | `python scripts/doc-links.py --check` | **0**（PASS） |
| 脚本索引 | `python scripts/scripts-index.py --check` | **0**（PASS） |

原始日志：`DSH-GO/tmp/GO8-RC2/_hv/{test-051.log,race-051.log}`。

---

## 4. 阶段二（v0.5.1 → v0.6.0）breaking 清单 —— **未落地**

### 4.1 包合并：`github.com/vivaui/viva/flex` 整个并入 `ui`

| 项 | 内容 |
|---|---|
| 旧写法 | `import "github.com/vivaui/viva/flex"`；`flex.Box`、`flex.Field`、`flex.Fields`、`flex.Look`、`flex.ScrollState`、`flex.DialogSpec`、`flex.Center/Start/End/Between` |
| 新写法 | `import "github.com/vivaui/viva/ui"`；`ui.Box`、`ui.Field`、`…` |
| 影响 | **139 个文件 import flex**；`flex.Box` 103、`flex.Field` 41、`flex.Fields` 8、`flex.Look` 19、`flex.ScrollState` 6 |
| 证据 | v0.6.0 模块根目录**没有 `flex/`**；`go get github.com/vivaui/viva/flex@v0.6.0` → `does not contain package`；`ui.Box` 在 v0.6.0 有 **172 个导出方法**，v0.5.1 的 `flex.Box` 130 个方法**一个都没少**（纯搬家 + 新增，方法面不回退） |
| 是否已改 | **否** |

### 4.2 包拆分：`ui.*` 的绘图/事件/主题标识符 → 新包 `gfx`

| 项 | 内容 |
|---|---|
| 旧写法 | `ui.Canvas`、`ui.Tree`、`ui.KeyEvent`、`ui.Icon`、`ui.Hit`、`ui.EventResult`、`ui.Scope`、`ui.PathCommand`、`ui.TextStyle`… |
| 新写法 | `gfx.Canvas`、`gfx.Tree`、`gfx.KeyEvent`、`gfx.Icon`、`gfx.Hit`、`gfx.EventResult`、`gfx.Scope`、`gfx.PathCommand`、`gfx.TextStyle`… |
| 影响 | 上游：v0.5.1 `ui` 有 **86 个类型**，v0.6.0 只剩 **54** 个——**81 个类型没了**，其中 80 个在 `gfx` 里同名存在。本仓实测：用到其中 **74 个标识符 / 1225 处 / 144 个文件**。最大头：`ui.Canvas` 241 处 / 103 文件、`ui.LineTo` 106 / 9、`ui.CubicTo` 87 / 5、`ui.KeyEvent` 81 / 30、`ui.RGB24` 67 / 1、`ui.MoveTo` 63 / 9、`ui.Hit` 41 / 22、`ui.PathCommand` 37 / 10、`ui.EventResult` 37 / 16 |
| 保留不动的 | `ui` 在 v0.6.0 保留 5 个**类型别名**：`type Rect = gfx.Rect`、`Color`、`Size`、`Theme`、`Point` → 本仓 **876 处**（`ui.Rect` 405、`ui.Color` 99、`ui.Size` 59、`ui.Theme` 15、`ui.Point` 2 等）**不用改** |
| ⚠️ 9 个名字从类型变成函数 | `ui.ID`、`ui.Tree`、`ui.Row`、`ui.Column`、`ui.Grid`、`ui.Text`、`ui.Icon`、`ui.Image`、`ui.State` 在 v0.6.0 的 `ui` 里是**元素函数**（如 `func ID(c C) gfx.ID`、`func Tree(c C, state *TreeState, n int, …) Box`），**类型**搬去 `gfx`。所以纯文本替换 `ui.X`→`gfx.X` 会误伤这些名字的函数调用，必须按编译器 "… is not a type" 逐点判。本仓具体：`ui.ID` 61 处（全是类型用法 → `gfx.ID`）、`ui.Tree` 112 处、`ui.Icon` 56 处、`ui.Image` 19 处，`ui.Row/Column/Grid/Text/State` 视用法而定 |
| 是否已改 | **否** |

### 4.3 编写模型重写：`flex.Frame` / `flex.View` 不再存在

| 项 | 旧写法（v0.5.1） | 新写法（v0.6.0） | 影响 |
|---|---|---|---|
| 声明上下文类型 | `*flex.Frame` | `ui.C`（= `*ui.Ctx`，**不带 `*`**） | `*flex.Frame` 形参 **112 处**；`flex.Frame` 出现 **360 次** |
| 容器 | `f.Row()` / `f.Column()` / `f.Stack()` | `ui.Row(f)` / `ui.Column(f)` / `ui.Stack(f)` | `f.Row(` 259、`f.Column(` 198 |
| 文本 | `f.Text(s)` | `ui.Text(f, s)` | `f.Text(` **347** |
| 叶子/装饰 | `f.Spacer()`、`f.Divider()`、`f.Spinner()`、`f.Wrap(s)`、`f.Paragraph(s,n)`、`f.Slot()`、`f.Image(b)`、`f.Icon(i)` | `ui.Spacer(f)` … `ui.Wrap(f,s)`、`ui.Paragraph(f,s,n)`、`ui.Slot(f)`、`ui.Image(f,b)`、`ui.Icon(f,i)` | Spacer 16、Divider 10、Spinner 1、Wrap 12、Paragraph 1、Slot 58（含 `pluginapi.Slot(` 等非 frame 计数，需按编译报错收敛）、Image 19、Icon 18 |
| 仍留在 `C` 上的方法 | `f.Theme()`、`f.Bounds()`、`f.Canvas()` | `c.Theme()`、`c.Bounds()`、`c.Canvas()`（**同名方法，无需改**） | 减轻一部分迁移量 |
| 窗口/嵌入宿主 | `flex.View`（零值可用，`Draw(canvas, theme, bounds, fn)`、`Begin`、`Wheel`、`ScrollState`） | `ui.Page`（`ui.NewPage(ui.PageConfig{View: func(ui.C)})`；`Page.Draw(canvas, bounds, func(ui.C))` **少一个 theme 参数**；Wheel/Key/Pointer/TextInput 变成 `Page` 方法） | `flex.View` **109 处**（大量在 `cmd/app/*_test.go` 的 harness 里以 `var view flex.View` 形态出现，需要改成持有 `*ui.Page`） |
| 是否需要改 | — | — | **全部未改** |

`page.Draw` 签名（v0.6.0 原文，`ui/page.go:664`）：

```go
func (p *Page) Draw(canvas *gfx.Canvas, bounds Rect, view func(C))
```

### 4.4 控件面：不只是改名，是"身份 + 绑定值"改写（需逐点语义改）

| 旧写法（v0.5.1 `Frame`） | 新写法（v0.6.0 `ui`） | 本仓处数 | 说明 |
|---|---|---|---|
| `f.Button(id ui.ID, label string)` | `ui.Button(c, label)` | 22 | **ID 参数消失**：身份改由 `c.Key(...)` / 声明位置给；`Clicked()` 语义不变 |
| `f.Switch(id, on bool)` / `f.Checkbox(id, checked bool)` | `ui.Switch(c, value *bool, label)` / `ui.Checkbox(c, value *bool, label)` | 1（Switch） | 从"传状态"改成"绑指针"，点击时写 `*value`（本仓 Switch 只 1 处，改造成本低但要重接 state） |
| `f.Select(scope, *SelectState, labels, selected)` | `ui.Select[T](c, value *T, choices []T, label func(T) string)` | 2 | 泛型 + 绑定指针；`scope` 身份消失 |
| `f.Tabs(scope, labels, selected)` | `ui.Tabs[T](c, value *T, tabs []T, label func(T) string)` | 0 | — |
| `f.Table(*TableState, TableSpec)` | `ui.Table(c, *TableState, TableSpec)` | 0 | 只加 `c` |
| `f.ScrollArea(id)` | v0.6.0 无同名；`Scroll`/`ScrollAt` + `ScrollState` | 10 | 需按用途改写 |
| `f.Scroll(offset float32)` | `ui.ScrollAt(c, offset)` | 8 | 语义相当 |
| `f.Field(*flex.Field, id, label, placeholder)` | `ui.TextField/TextArea` 一族 + `flex.Field`→`ui.Field` | 7 | 需按"绑定值"改写 |
| `f.Popup()` | `ui.Popup`（v0.6.0 改为 `Box.Popup` 层/`PopupLayer`） | 1 | 需按弹层语义改写 |
| `f.End()` | v0.6.0 无 `End` | 5 | 容器闭合方式变化（改用 `Children(func(){...})`） |
| `f.Dialog(flex.DialogSpec)` | `ui.Dialog(c, spec)` / `ui.Modal` | 2 | 加 `c`，`Scope/DismissID/CloseID/Layer` 需显式给 |

### 4.5 机械迁移实测（在一次性副本里做的，不动交付 worktree）

为把"能不能靠脚本过编译第一关"变成数字，在 `tmp/GO8-RC2/_hv/mig060`（worktree 的拷贝）上跑了两遍脚本化迁移：

| 轮次 | 做了什么 | 改写文件 | `go build ./...` 结果 |
|---|---|---|---|
| 第 1 轮 | `flex` 包并入 `ui`、`*flex.Frame`→`ui.C`、`flex.View`→`ui.Page`、`flex.`→`ui.`、按（当时不准的）名单做 `ui.`→`gfx.` | **168 个文件** | 红；错误集中在 `ui.ID (value of type func(c ui.C) gfx.ID) is not a type`（9 处/包批）、`undefined: ui.Canvas`、`undefined: ui.Scope` |
| 第 2 轮 | 用**源码**抽出的名单重做 `ui.`→`gfx.`，并把 `ui.ID`→`gfx.ID` | **110 个文件** | 红；首个错误变成 `ui.Tree (value of type func(c ui.C, state *ui.TreeState, n int, …) ui.Box) is not a type` |

结论（这是阶段二计划的关键约束）：**纯文本改名过不了编译，卡点是"同名不同类型"**——`ID/Tree/Row/Column/Grid/Text/Icon/Image/State` 在 v0.6.0 的 `ui` 里是元素函数，类型搬去了 `gfx`。所以 V-06-1 必须按编译器的 `… is not a type` 逐点判（或者用 `go/types` 做类型感知改写），不能一把 sed。改完这些，剩下的才是 §4.4 的控件语义改写。

### 4.6 阶段二为什么本轮不做

1. **它是重写，不是改名**：§4.4 合计 ~60 处控件要按"身份 + 绑定值"重新接线，`flex.View` 109 处要改成 `*ui.Page` 生命周期，改完必须逐帧验证界面几何（本仓有 `tools/visualparity`/headless 快照类判据）。
2. **门禁是"全绿才合"**：190+ 文件同时红，脚本化改名只能过编译第一关，测试与视觉判据要按新身份模型重校；强行半途停在红树会污染 `chore/viva-0.6.0` 分支。
3. **可以机械化的部分已经量化**（§4.1–4.3：`flex.`→`ui.`、`ui.`→`gfx.`、`Frame`→`C`、`f.X()`→`ui.X(f)`），适合拆成独立任务**逐个包**推进（见 §8），不适合与本轮的真机验收混在一起。

---

## 5. `text.(*Editor).String()` 并发缺陷（issue #239）核对

| 项 | 结论 |
|---|---|
| 上游修了吗 | **没有**。v0.5.1 与 v0.6.0 的 `text/editor.go` 都还是"读时惰性写缓存" |
| 证据（v0.5.1） | `text/editor.go:113` `func (editor *Editor) String() string`，函数体 `:119-121` 是 `if !editor.textValid { editor.text = string(editor.runes); editor.textValid = true }`；`Editor` 内**无任何 mutex/atomic** |
| 证据（v0.6.0） | 同一函数体、同一行号（`editor.go:113`，缓存写在 `:120`），**逐字相同，未修** |
| 本仓兜底 | `cmd/app/main.go` 的 `pendingDraft` / `pendingSmoke`（后台只排队，`cmd/app/wpc1_send.go:146 applyPendingDraft()` 在 UI 线程消费） |
| 处置 | **兜底保留，不删**。文档 `docs/viva-upstream-editor-string-race.md` 继续有效（其"上游修好后删 `spike/vivaeditor/`"的条件仍未满足） |

---

## 6. 真机证据（阶段一，`v0.5.1`）

- 宿主：`DSH-GO` 现编 `dshgo.exe`（`CGO_ENABLED=0 go build -o …/dshgo.exe ./cmd/dshd`，`python scripts/bin-check.py` → `OK=1 FAIL=0`，确认不是陈旧二进制）
- 宿主启动：`dshgo.exe serve --home <临时 home> --policy ask` → `http://127.0.0.1:60494`（ready 行带 url + token）
- 真 apiclient：`GET /connect?token=…` → cookie（`dshd_session`）→ `POST /api/session.create` → `{sessionId: 3b2155a4-…}` → `POST /api/session.list` 回读到该会话
- 界面：`DSH GUI GO.exe -addr <url> -cookie dshd_session=… -shell official -size 1180x800 -shot …`（v0.5.1 构建）
  - `shot-official.png`：官方骨架三栏、25 个插件全部装配（`ds-brand … ds-rightbar`）、侧栏工作区 `_hv` + 会话行、输入框、状态栏 `已连接 · 0 条`、`上下文 0/1M` 完整
  - `shot-settings.png`：设置浮层
  - `shot-demo.png`：`ds-demo` 插件 toast + 空态大标题
  - 三张均为 1180×800、非空、无白屏（对照 `docs/blank-center.md` 的空白窗症状）
- 截图路径：`DSH-GO/tmp/GO8-RC2/_hv/shot-{official,settings,demo}.png`
- 收尾：本轮自己起的 `dshgo.exe`（PID 2388，命令行含我的临时 home）已 `Stop-Process`；未动其它实例

---

## 7. 阶段二建议的拆法（若决定做 v0.6.0）

按"每步都能单独全绿"切：

| 步骤 | 内容 | 机械度 | 验收 |
|---|---|---|---|
| V-06-1 | `ui.*` → `gfx.*`（74 标识符 / 99 文件）+ `flex.*` → `ui.*` 类型搬家 | 纯改名，可脚本 | `go build ./...` 绿 |
| V-06-2 | `flex.Frame` → `ui.C`，`f.X()` → `ui.X(f)`（≈1000 处） | 半机械（编译器逐个报 `undefined`） | `go build` + `go vet` 绿 |
| V-06-3 | `flex.View` → `ui.Page`（109 处，多在 `cmd/app/*_test.go` harness） | 需设计：harness 持有 `*ui.Page` | 该包 `go test` 绿 |
| V-06-4 | 控件面（§4.4，~60 处）按绑定值/Key 改写 | 语义改写 | headless 快照/视觉判据 + 真机截图 |
| V-06-5 | 全量门禁 + race + 真机 + 视觉对照 | — | 与阶段一同口径 |

---

## 8. 本轮未做 / 待决策

1. **v0.6.0 未落地**（§4）——需要用户/Lead 决定是否单独立项按 §7 拆；本轮已把清点与迁移配方固化，可直接下发。
2. **上游文档缺口未提**：`docs/upgrading.md` 漏了 `flex`→`ui` 合并与 `ui`→`gfx` 拆分（§1 注）。按本仓规矩可提 issue/文档 PR，但**对外动作需用户当次明确授权**；本轮只记录，未发。
3. **issue #239 仍开**：上游 v0.5.1/v0.6.0 都未修（§5），我们继续兜底；不需要重提。
4. 本 worktree **未 commit**（按任务约定由 Lead 统一提交）。
