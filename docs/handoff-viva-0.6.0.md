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

---

## 9. V-06-1 实测（阶段二第一步：`flex`→`ui` 类型搬家 + `ui`→`gfx` 拆分）

> 现场：隔离 worktree `../dsh-ui-v06`（`D:/DeepSeek Harness/default-workspace/dsh-ui-v06`），分支 `chore/viva-0.6.0-stage2`，从 `origin/main`（`3f85ed1`，PR #46 后）拉出，**未 commit**。
> `go.mod`: `github.com/vivaui/viva v0.6.0`；`go.sum` 为该版本 tidy 后的 10 行（由诊断副本 `go mod tidy` 全绿产出后回填）。
> 本节数字全部是**本轮实测**（AST 计数 + 编译器），替代 §4.2 里那份偏低的估算（74 标识符 / 1225 处 / 144 文件）。

### 9.1 结论

| 项 | 结果 |
|---|---|
| `flex.*` → `ui.*` 类型搬家 | **完成**（本仓用到的 9 个 flex 名字 / 382 处 / 73 文件全部改到 `ui`） |
| `ui.*` → `gfx.*` 拆分 | **完成**（81 个标识符 / 1737 处 / 166 文件） |
| `gofmt -l` | **0 个文件** |
| `go build ./...` | **红**——唯一根因是 `github.com/vivaui/viva/flex` 在 v0.6.0 不存在（146 个文件仍 import 它，里面是 `flex.Frame`/`flex.View`，属 V-06-2/V-06-3） |
| `go vet ./...` / `go test ./...` | 同根因：受影响包 `[setup failed]`，**没有一条错误属于 V-06-1 类**（见 9.5 的编译器验证） |

结论一句话：**V-06-1 的改名已 100% 做完且可证明没有残留**；build 不绿是 V-06-2/3 的施工面（`flex.Frame`/`flex.View` 不再导出），不是改名没做完。

### 9.2 四张上游名单（从模块缓存源码抽的，不是照 release notes 猜）

| 面 | v0.5.1 | v0.6.0 | 本仓用到 |
|---|---|---|---|
| `ui` 导出类型 | 86 | **54** | — |
| `ui` 导出名（含函数/常量） | 259 | 177 | 改名后仍有 **1066** 处 `ui.*`（全部命中 v0.6.0 `ui`） |
| 从 `ui` 消失的导出名 | — | **241**，其中 **240** 在 `gfx` 同名存在（唯一例外 `ReplaceBlockElements`，本仓 0 处） | 81 个 |
| `flex` 导出名 | 35 | **包整体不存在**；33 个原样进 `ui`，`Frame`/`View` 不再导出 | 9 个 |

保留不动的 5 个名字是**类型别名**（`ui/vocab.go`）：`type Color = gfx.Color`、`Rect`、`Size`、`Point`、`Theme`——所以本仓剩下的 `ui.Rect/Color/Size/Point/Theme` 与 `gfx` 侧完全可互操作，不用改。

### 9.3 9 个"同名不同类型"的实测（§4.3 的坑，实际比预估窄）

`ID / Tree / Row / Column / Grid / Text / Icon / Image / State` 在 v0.6.0 的 `ui` 里是元素函数、类型搬去 `gfx`。本仓实测：

| 名字 | 作为**类型**用法 | 处理 |
|---|---|---|
| `Tree` | 127 | → `gfx.Tree` |
| `ID` | 65 | → `gfx.ID` |
| `Icon` | 64 | → `gfx.Icon` |
| `Text` | 39 | → `gfx.Text` |
| `Row` / `Column` / `Grid` / `Image` / `State` | **0**（本仓没有当类型用过） | 无需改 |

为什么这批可以按名字判而不会误伤：在**旧树**（v0.5.1）里这 9 个名字在 `ui` 中**只有类型身份**，所以任何 `ui.X` 出现都只能是类型用法 → 改到 `gfx` 恒等价；改完用 v0.6.0 的 `ui`/`gfx` 导出名单全量校验，`ui.*` 1066 处、`gfx.*` 1737 处**零未知名**（0 处 `is not a type`、0 处 `undefined`）。

### 9.4 改名清单（实测）

`flex.*` → `ui.*`（9 名 / **382 处** / 73 文件）：`Center` 188、`Box` 109、`Field` 26、`Look` 23、`End` 11、`Start` 11、`Fields` 6、`ScrollState` 6、`Between` 2。

`ui.*` → `gfx.*`（81 名 / **1737 处** / 166 文件），前 20：`Canvas` 282、`CubicTo` 160、`LineTo` 136、`Tree` 127、`KeyEvent` 87、`RGB24` 75、`MoveTo` 73、`ID` 65、`Icon` 64、`Hit` 48、`PathCommand` 42、`Text` 39、`EventResult` 37、`KeyEscape` 33、`ClosePath` 29、`Scope` 23、`WeightMedium` 22、`KeyEnter` 20、`ModifierShift` 20、`KeyDown` 19；其余 61 个见报告。

批量改动：**188 个 .go 文件 + go.mod + go.sum**（`1996 insertions / 1854 deletions`）；import 面：`ui` 164 文件、`gfx` 166 文件（135 个文件同时 import 两个），`flex` 仍是 146 文件（V-06-2/3 残留）。**注释零丢失**（`//` 行 16895 → 16895）。

### 9.5 怎么做的（可复现的配方）

1. `names.exe` 从**模块缓存源码**抽四个名单（v0.5.1 `ui`/`flex`、v0.6.0 `ui`/`gfx`），算 `move_gfx`（从 `ui` 搬走的 **240** 个 + 9 个"同名类型变函数" = **249** 个；本仓命中 **81** 个）与 `flex_move`（33 个，本仓命中 9 个）。
2. `rename.exe`：`go/parser` 拿 **selector 精确字节区间**改写（不是正则/不是 sed），import 声明**每个 decl 至多一次编辑**（保留注释与顺序），改完**回读解析校验**，任何不解析的文件自动回滚并记 `parse_fail`（本次 0）。
3. `check_names.py`：把改后树里每个 `ui.X`/`gfx.X` 与 v0.6.0 导出名单比对（先剥注释/字符串）→ `ui` 0 未知、`gfx` 0 未知。
4. **诊断副本**（一次性，不进交付树）：把 `*flex.Frame`→`ui.C`、`flex.View`→`ui.Page` 机械铺开后再 `go mod tidy`（**绿**）+ `go build`——剩 22 个包 / 168 行错误，**全部**是 `f.X undefined (type ui.C has no field or method X)`，即 V-06-2 的施工面；**没有一条** `undefined: ui.<搬走的类型>` 或 `is not a type`。这条是"V-06-1 无残留"的编译器级证据。

### 9.6 还剩什么（下一步的准确工作量）

| 归属 | 内容 | 实测 |
|---|---|---|
| **V-06-2** | `flex.Frame`→`ui.C` + `f.X()`→`ui.X(f)` | frame 方法调用 **912 处 / 85 文件**：`Text` 301、`Row` 267、`Column` 199、`Slot` 36、`Button` 22、`Icon` 22、`Spacer` 18、`Wrap` 12、`ScrollArea` 10、`Field` 7、`Bounds` 6、`Divider` 5、`Popup` 3、`Paragraph` 1、`Spinner` 1、`Switch` 1、`Theme` 1；减掉 `Bounds`(6)/`Theme`(1)（v0.6.0 `Ctx` 保留同名方法）→ **需改写 905 处**。另 `*flex.Frame` 形参 **371 处**、`f.End()` 3 处（测试收尾） |
| **V-06-3** | `flex.View`→`ui.Page` | **119 处 / 85 文件**；`var view flex.View` 97 处 / 69 文件、`view.Draw(` 113、`view.Begin(` 4；**非测试文件只有 6 处**（`cmd/app/main.go` + 5 个 `spike/*/main.go`），其余全在测试 harness |
| **V-06-4** | 控件面语义改写 | 落在 V-06-2 的调用里：`Button` 22、`ScrollArea` 10、`Field` 7、`Popup` 3、`Switch` 1；`f.Dialog(` / `f.Scroll(` **本仓 0 处**（§4.4 里 Dialog 2 / Scroll 8 是清点误差，实际 frame 上没有这两个调用） |

上游其它包的签名变化已核对：`app`/`components`/`headless`/`keymap` 共 **192 个函数**把参数/返回值里的 `ui.X` 换成 `gfx.X`，因 `Color/Rect/Size/Point/Theme` 是**别名**、其余都是被搬走的真类型，**调用点无需改**；`gfx.Canvas.BlockRectangles` 在 v0.6.0 删除，本仓 0 处使用。搬走类型的**方法面不回退**（抽查 `Canvas` 193→224 个方法、`Tree` 18→18、`Icon` 6→6、`KeyEvent` 1→1）。

### 9.7 门禁与证据位置

| 门禁 | 结果 | 证据 |
|---|---|---|
| `gofmt -l` | **0** | — |
| `CGO_ENABLED=0 go build ./...` | 红：`pluginapi/plugin.go:26: no required module provides package github.com/vivaui/viva/flex` | `DSH-GO/tmp/GO8-RC2/_v06/06-final-build.log` |
| `CGO_ENABLED=0 go vet ./...` | 同上 | `_v06/07-final-vet.log` |
| `CGO_ENABLED=0 go test ./... -count=1` | 各包 `[setup failed]`（同根因，无 V-06-1 类失败） | `_v06/08-final-test.log` |
| 诊断副本 tidy/build | tidy **绿**；build 只剩 V-06-2 类 | `_v06/03-diag-tidy.log`、`_v06/04-diag-build.log`、`_v06/05-diag-vet.log` |
| 名字全量校验 | `ui` 0 未知 / `gfx` 0 未知 | `_v06/rename-report.json`、`check_names.py` 输出 |

给 V-06-2 的可选起点：`DSH-GO/tmp/GO8-RC2/_v06/v061-seed-frame-view.patch`（146 文件，`git apply --check` 通过）——它把 `*flex.Frame`→`ui.C`、`flex.View`→`ui.Page` 机械铺开，铺开后 `go build` 会直接列出需要 `ui.X(f)` 的每个点。**该 patch 不在本交付树里**，是否采用由 V-06-2 决定；V-06-2 也可以从本 worktree 的干净状态自己做。

---

## 10. V-06-2 实测（阶段二第二步：frame 方法调用 → v0.6.0 元素函数）

> 现场：同一隔离 worktree（分支 `chore/viva-0.6.0-stage2`，起点 HEAD `d826b18` = V-06-1 提交），**未 commit**（按约定由 Lead 统一提交）。
> 起点：`DSH-GO/tmp/GO8-RC2/_v06/v061-seed-frame-view.patch`（146 文件）`git apply --check` → `git apply` 通过后开始。

### 10.1 结论

| 项 | 结果 |
|---|---|
| frame 方法调用改写 | **925 处 / 89 文件**（接收者三种：`f.`、`fr.`（测试）、`b.Frame.`（`plugins/dstool` 的 `BodyCtx.Frame`））；其中 **54 处 / 5 文件在 `spike/*` 五个独立子模块**里（各自 `go.mod`，不在 `./...` 门禁内）→ **主模块 871 处 / 84 文件** |
| 其中**机械一一对应** `f.X(…)`→`ui.X(f, …)` | **882 处** |
| 其中**语义改写**（v0.6.0 换了形状） | **43 处**：`Button` 22、`ScrollArea` 10、`Field` 7、`Popup` 3、`Switch` 1 |
| 保持不动的 `Ctx` 方法 | `f.Bounds()` 15 处、`f.Theme()` 1 处（v0.6.0 `Ctx` 保留同名方法；`Theme()` 返回 `*Theme`，那 1 处调用点补了 `*`） |
| `gofmt -l` | **0** 个文件 |
| `CGO_ENABLED=0 go build ./...` | **红，只剩 `cmd/app` 4 处**，且 4 处**全是 V-06-3 的 `ui.Page` 面**（`m.view.ScrollState` ×3、`m.view.Draw` 带 theme ×1）；本包所有非测试代码类型检查通过 |
| `CGO_ENABLED=0 go vet ./...` | 红 **20 处，全部是 V-06-3 面**：`view.Draw` 13、`canvas.BeginEventPass` 5、`view.ScrollState` 2 |
| `CGO_ENABLED=0 go test ./... -count=1` | **15 包 ok / 20 包 `[build failed]`**；`[build failed]` 全部来自上面 20 处 vet 级错误，**0 条运行期失败** |

一句话：**V-06-2 的施工面已 100% 做完**（全树 `f.X(` 型 frame 调用 **0 处残留**，`grep` 可证）；build/vet 不绿**不是** V-06-2 的残留，而是 V-06-3 的 `ui.Page` 面（`Page` 没有 `ScrollState`/`Draw(theme)` 了）。

### 10.2 改写清单（实测，按方法）

| 方法 | 处数 | v0.6.0 写法 |
|---|---|---|
| `Text` | **312**（303 + `b.Frame.` 9） | `ui.Text(c, s)` |
| `Row` | **271**（269 + 2） | `ui.Row(c)` |
| `Column` | **201**（200 + 1） | `ui.Column(c)` |
| `Slot` | **37**（31 `f.` + 6 `fr.`） | `ui.Slot(c)` |
| `Icon` | **23** | `ui.Icon(c, icon)` |
| `Button` | **22** | `ui.Button(c, label)` ← **丢显式 ID** |
| `Spacer` | **18** | `ui.Spacer(c)` |
| `Wrap` | **12** | `ui.Wrap(c, s)` |
| `ScrollArea` | **10** | `ui.Scroll(c)` ← **丢显式 ID** |
| `Field` | **7** | `ui.TextFieldID(c, id, field, label, placeholder)` ← **参数重排** |
| `Divider` | **6** | `ui.Divider(c)` |
| `Popup` | **3** | `ui.Overlay(c, ui.OverlaySpec{Band, Bounds})` + `Absolute()` 子面板 |
| `Paragraph` / `Spinner` / `Switch` | 各 **1** | `ui.Paragraph(c, s, n)` / `ui.Spinner(c)` / `ui.Switch(c, *bool, label)` |

机械对应 882 = 312+271+201+37+23+18+12+6+1+1；语义 43 = 22+10+7+3+1；合计 **925**。

### 10.3 五个形状变化点（以 v0.6.0 源码为准，不是照报错猜）

1. **`ui.C` = `*Ctx`，元素是包级函数**：`func X(c C, …) Box`，所以 `f.X(a)`→`ui.X(f, a)`，零参 `f.X()`→`ui.X(f)`。`Box` 的链式 API（`Width/Pad/Children/…`）不变，链式写法整段不用动。
2. **`f.Popup()` 在 v0.6.0 不导出**（公开浮层只剩 `Menu` / `Popover` / `Dialog` / `Drawer` / `Modal` / `Overlay`；`frame.Popup()` 仍是 unexported）。应用的自由浮层走 `ui.Overlay(c, OverlaySpec{Band: LayerPaneChrome, Bounds: 页面矩形})`，它内部做 `At+Size`；被定位的子面板用 `ui.Column(c).Absolute().Left/Right(…).Width(…).Bottom(…)`（`solveAbsolute` 对 `dimAuto` + 单边锚定给内容高度）。3 处都这么改：模型菜单（`Above(…, End)` → 右缘对齐）、统计弹窗（`At(0,0).Size(窗口)` → `Inset(0)` 遮罩；`Above(…, Start)` → 左缘 + `Bottom`）。
3. **`ScrollArea(id)` 的显式 ID 没了**：v0.6.0 的滚动区身份来自声明名（`Key`/`Part`/位置），`ui.Scroll(c)` 不接受 ID。10 处丢 ID，走位置命名（同一位置跨帧稳定）；`docs/…` 里那条"每个会话一个滚动 id，否则切会话时滚动位置会串"的注释现在只能靠位置命名兜底——**跨视图共享滚动的语义丢了**，登记 V-06-4。
4. **`Field(field, id, label, ph)` → `ui.TextFieldID(c, id, field, label, ph)`**：参数顺序变了（`id` 提到 `field` 前），7 处按位重排。
5. **`Switch(id, on)` → `ui.Switch(c, *bool, label)`**：v0.6.0 的开关绑 `*bool` 并由控件写回。`plugins/dsagentteam` 给 `Plugin` 加了绑定位 `toggleValue`，每帧先重置成投影值，再 `ui.Switch(f, &p.toggleValue, "").Label(…)` + 仍用 `.Clicked()` 走 `p.team.Select(...)`——**非乐观更新语义不变**（写回的是绑定位，真值仍来自投影）。

另外：`Ctx.Theme()` 返回 `*Theme`（v0.5.1 是值），`plugins/dscomposer` 那 1 处补 `*`。

### 10.4 怎么做的

1. `git apply` 起点补丁（146 文件：`*flex.Frame`→`ui.C`、`flex.View`→`ui.Page`）。
2. **自己写括号配平器做改写**（不用 sed）：扫描 `IDENT.Name(`，**跨注释/字符串**配平括号取实参，按方法名分派（零参/一参/两参/丢 ID/换序），只认接收者 `f`、`fr`、`IDENT.Frame` 三种。第一版漏掉了 `.Children(func(){ … })` 里的嵌套调用（扫描器把 `Children(` 整段当成一个调用吞掉了），改成"不是自己的方法名就只跳过 `(`"后再跑一遍：**709 → 909 处**。脚本 `DSH-GO/tmp/GO8-RC2/_v062/rewrite.py`、`rewrite_frame_field.py`，**幂等**（再跑输出 `files=0`）。
3. 剩下的 3 处 `Popup`、1 处 `Switch`、以及丢 ID 后产生的 2 个未使用变量、1 个未使用的 `id` 闭包，逐个手改。
4. `gofmt -w` 过一遍（13 个文件因实参增加需要重排），`gofmt -l` 归零。
5. `go build` / `go vet` / `go test` 三档跑到底，按报错归类。

### 10.5 还剩什么（归类，不是"边界"带过）

**① V-06-3 · `ui.Page` 面（唯一阻塞 `go build` 的东西，4 处 / 1 文件）**

| 位置 | 现状 | v0.6.0 要求 |
|---|---|---|
| `cmd/app/main.go:716,743` `m.view.ScrollState(id)` | `View.ScrollState(gfx.ID) *ScrollState` 的 **ID 查表** | v0.6.0 **没有** ID→state 的公开查表：`ScrollState` 只在 `Box.ScrollState()`（声明期拿 Box、`Draw` 之后读同一个 paint）。`saveScroll`/`restoreScroll` 要改成"声明时把 `ui.Scroll(...)` 的 Box 交给壳"，或由声明方自己存取 |
| `cmd/app/main.go:1805` `Env.Scroll: m.view.ScrollState` | `pluginapi.Env.Scroll func(gfx.ID) *ui.ScrollState` | 同上；`plugins/dsconversation`（`conversation.go:331`、`wpc2_turns.go:79`）是在**声明期**要 state 的，v0.6.0 等价物是声明处直接拿 Box |
| `cmd/app/main.go:1830` `m.view.Draw(canvas, theme, bounds, fn)` | 每次 Draw 传 theme | `Draw(canvas, bounds, View)`；主题改由 `ui.NewPage(ui.PageConfig{Theme: *Theme})` 给，**主题切换要重建/重配 Page** |

**② V-06-3 · 测试 harness（阻塞 `go vet` / `go test`，20 处 / 16 文件）**：`view.Draw` 13 处（删 theme 实参）、`canvas.BeginEventPass` 5 处（v0.6.0 的 gfx 签名参数变多）、`view.ScrollState` 1 处、`f.End()` 3 处（`Page` 没有 `Begin`/`End`，`var view ui.Page` + `view.Begin(...)` 的整段 harness 要按 `Draw(canvas, bounds, func(c ui.C))` 重写）。

**③ V-06-4 · 控件面语义（编译过得去，语义要另立判据）**：
- `Button` 22 / `ScrollArea` 10 丢掉的**显式交互 ID**：现在身份来自 `Key`/`Part`/位置。用 `env.Tree.ID(env.Feat, "…")` 造 ID 再按 ID 断言命中区的测试，要按新身份重写（本轮只有 2 处用 `f.KeyInt(i)` 显式保住"每行一个身份"）。
- `Popup` 3 处的 `Above()` **"放不下翻到另一面"** 语义丢掉了（Overlay 绝对定位不翻面）；两处锚点都在窗口底部，正常窗口不触发，但这是真实差异。
- 浮层改写的**几何等价**已由冒烟实测（见 10.6），但**真实插件路径**（`DrawModelMenu` / `drawStatsDialog` 的实际锚点与样式）无画面验证，需要 V-06-5 的视觉/命中对照。

### 10.6 本轮做了什么验证 / 没做到什么

- **做到了**：925 处全部过编译器（`go build` 除 `cmd/app` 外全通）；15 个包 `go test` 全绿，其中 `internal/uidecl`（11 处）、`plugins/dsdeliverables`（12）、`plugins/dsjobs`（6）、`plugins/dsschedule`（6）、`plugins/dscommands`（7）、`plugins/dsterminal`（5）**含本轮改写点且测试真跑并全绿**——这是改写后插件仍能真出画面的运行期证据。
- **另外做了运行期冒烟**（一次性，`_v062smoke/`，跑完已删、源码存档 `_v062/smoke/main.go.txt`）：`SMOKE OK` —— 遮罩命中区正好 `(0,0,900,700)`（`Popup().At+Size` → `Overlay+Inset(0)` 等价）、面板 `x=100 w=200 底边=650(=700−50) h=20(=内容高)`（`Popup().Above(anchor,50,Start)` → `Absolute().Left/Width/Bottom` 等价，**内容高度成立**）、10 类改写元素同帧声明全出命中区、无 panic。
- **没做到**：paint 类测试（`view.Draw` harness）在 V-06-3 落地前跑不起来，所以 20 个 `[build failed]` 的 UI 包（改写点最多的是 `dsconversation` 198、`dsagentteam` 83、`dssidebar` 74、`internal/uikernel` 71、`dscomposer` 54、`dssettings` 51、`dsrightbar` 51）**只有编译器验证，没有画面验证**；浮层 3 处同样只有编译器验证。5 个 `spike/*` 子模块同理（它们的 `view.Draw` 属 V-06-3，本轮不动）。

### 10.7 证据位置

| 内容 | 路径 |
|---|---|
| 改写脚本（幂等，可复跑核对） | `DSH-GO/tmp/GO8-RC2/_v062/rewrite.py`、`rewrite_frame_field.py` |
| 最终 `go build` 输出 | `_v062/10-build-final.log` |
| `go vet` 输出（20 处全清单） | `_v062/11-vet.log` |
| `go test` 输出（15 ok / 20 build failed） | `_v062/12-test.log` |
| 被改写的文件清单（88 个 `f.`/`fr.` 面；`plugins/dstool/render.go` 另计 1） | `_v062/13-rewritten-files.txt` |

## 11. V-06-3 实测（阶段二第三步：`ui.Page` 面收尾 —— `Draw`/`ScrollState`/`Begin`+`End`）

> 现场：同一隔离 worktree（分支 `chore/viva-0.6.0-stage2`，起点 HEAD `ab2b9db` = V-06-2 提交），**未 commit**（按约定由 Lead 统一提交）。

### 11.1 结论

| 项 | 结果 |
|---|---|
| `CGO_ENABLED=0 go build ./...` | **绿**（V-06-2 收尾时红 4 处，全在 `cmd/app/main.go`） |
| `CGO_ENABLED=0 go vet ./...` | **绿**（V-06-2 收尾时红 20 处 / 20 包） |
| `CGO_ENABLED=0 go test ./... -count=1` | **绿：35 包 ok / 0 包红 / 0 条运行期失败**（V-06-2 时 15 ok + 20 `[build failed]`） |
| `gofmt -l .` | **0** 个文件 |
| 新增回归测试 | **1 条**：`cmd/app/main_test.go` `TestPendingScrollRestoresOffset`（壳侧"切回会话恢复阅读位置"，含 `settleScroll` 顺序的变异验证） |
| 改动面 | **90 文件**（代码 89：非测试 **7** / 测试 **82**；另 1 份本文档），`+920 / −421` |
| `view.Draw` 去 theme | **117 处 / 82 文件**（脚本改写 105 + 手改 12；不含 `main.go` 产品 1 处与 `spike/*`） |
| `view.ScrollState(id)` | **6 处**（`main.go` 3 + 测试 3）全部按 v0.6.0 的"声明方持有"重写 |
| `view.Begin` / `f.End` | **3 组**（`dsconversation` ×2、`dstopbar` ×1）改写成 `Draw` |
| `canvas.BeginEventPass` | **27 处**补 `gfx.ClickInfo{}`（零值 = 无修饰键的普通单击，语义等价） |

一句话：**阶段二的编译面到此闭合**——`flex.View` → `ui.Page` 的类型搬家（V-06-1）与元素函数改写（V-06-2）之后，`ui.Page` 自己那层方法面（`Draw` 的 theme 形参、`ScrollState(id)` 查表、`Begin`/`End`）在本轮按 v0.6.0 的**所有权模型**重新接了一遍，三档门禁全绿。

### 11.2 四个形状变化点（都以上游 v0.6.0 源码为准，不照报错猜）

1. **主题是构造期的，不是 `Draw` 的形参**：`Page.Draw(canvas, bounds, view)`。壳的做法是
   `ui.NewPage(ui.PageConfig{Tree: &tree, Theme: &theme})` —— `Theme` 给的是**包级 `theme` 的地址**，
   而 `Page.currentTheme()` 每次 paint 都解引用它，所以 `applyPendingTheme` 在 Paint 里改 `theme`
   之后**下一帧就换色**，不需要重建 Page（这是本轮唯一"必须想清楚"的主题问题）。
2. **没有 `Page.ScrollState(id)`**：v0.6.0 只有 `Box.ScrollState()`，而且只在**同一个 paint 内**有效
   （`scrollStore` 按帧 sweep，未声明的滚动区当场丢状态）。所以"壳按 ID 回头查"这条路**不存在**，
   见 §11.3。
3. **没有 `Page.Begin` / `Page.End`**：`Draw` 内部完成声明与解算（`paintOnce` + `reportScrolls`），
   harness 里 `f := view.Begin(canvas, theme, bounds)` … `f.End()` 的整段写法按
   `view.Draw(canvas, bounds, func(c ui.C) {...})` 重写。
4. **`canvas.BeginEventPass(id)` → `(id, gfx.ClickInfo{})`**：`ClickInfo` 的注释写明
   "The zero value is a plain click with no modifier"，所以补 `gfx.ClickInfo{}` 与旧行为等价。

### 11.3 `ScrollState` 的替代：`Env.ScrollBox` + 每帧抄偏移（本轮唯一的接口改动）

v0.6.0 的设计取向是"**应用不持滚动状态**"（`ui.Scroll`/`ScrollAt` 声明、`Page.Wheel` 滚轮、
`Box.ScrollState()` 读回），而我们的壳原本是"按 ID 查上一帧"（`saveScroll`/`applyPendingScroll`/`Env.Scroll`）。
本轮按"声明方持有"落地，**没有**改成 `ScrollAt`（那会把滚轮控制权交给应用，见 §11.5）：

| 位置 | 旧形状 | 新形状 |
|---|---|---|
| `pluginapi.Env.Scroll func(gfx.ID) *ui.ScrollState` | 壳→插件按 ID 查表 | **删除** |
| `pluginapi.Env.ScrollBox func(ui.Box)` | — | 插件声明完消息流滚动区后把**本帧的 Box** 交回壳 |
| `pluginapi.Env.ViewportOffset *float32` | 右栏伪造 `&ui.ScrollState{Offset:1e9}` 骗虚拟化 | 壳/接管方给一个偏移覆盖值（轨迹视图把正文推到视口外） |
| `cmd/app` `saveScroll(id)` | 切会话时回头查上一帧 | `recordScroll(box, handed)`：**每帧**从壳收到的 Box 抄一次偏移 |
| `cmd/app` `applyPendingScroll()` | `st := view.ScrollState(id)` 再 `ScrollTo` | `applyPendingScroll(box, handed)`：布局后（`Draw` 返回）用同一个 Box 做 `ScrollTo` |
| `cmd/app` Draw 之后的处理顺序 | 无此问题（旧实现是先恢复、后无记录） | 收进 `settleScroll(box, handed)`：**先恢复、再记偏移**。反了的话切会话那一帧的新滚动区是从 0 布局的，`recordScroll` 会把"切走前的位置"覆盖成 0 → 恢复等于没恢复。新增回归测试 `TestPendingScrollRestoresOffset`，并把 `settleScroll` 内部两行对调做过变异验证（红：`恢复后的偏移 = 500，want 120`；复原后绿） |
| `dsconversation` 消息流 | `env.Scroll(scrID)` | 自己声明滚动区（身份用 `f.Key("msgs-"+会话ID)`，保住"每会话一份滚动位置"）后 `p.bodyBox.ScrollState()` |
| `dsconversation` Turn 导航 | 声明期 `env.Scroll(...)` | 用 `afterDraw` 抄下的 `scrollOffset`/`scrollAtEnd` 值（**值的时序与旧实现一致**：都是"上一帧 settled"） |
| `dsconversation` Turn 跳转 | `env.Scroll(id).ScrollTo(...)` | 先排队（`jumpTo`），滚动区声明时再落位 |
| `dsrightbar` 轨迹列表 | `env.Scroll(scrID)` | 自己声明的 Box 取 `Viewport`/`Offset`，搜索词变化时 `ScrollTo(0)` |
| `dssidebar` 列表 | — | 插件多存一个 `scrollBox ui.Box` 字段（测试按 Box 取状态） |

为什么不用 `ui.ScrollAt(c, offset)` 做恢复：它会把 `scrollManual` 置真，
`view.Wheel` 遇到 manual 区**直接返回未处理**（`ui/scroll.go:209`），滚轮就废了。
精确恢复留给 V-06-4 连同"应用自己接管滚轮"一起做。

### 11.4 显式交互 ID 消失带来的测试改写（7 处，只改测试，不改产品语义）

V-06-2 的 `Button`(22)/`ScrollArea`(10)/`Switch`(1) 丢了显式 ID，身份改由 `Key`/`Part`/声明位置决定。
之前这些包是 `[build failed]`（跑不到），本轮一编译就暴露了 5 个包的断言：

| 文件 | 旧断言 | 新断言（强度不变） |
|---|---|---|
| `dsshortcuts/shortcuts_guard_test.go` | 扫 `ui.KeyXxx`（AST） | 认 `ui.` **和** `gfx.` 两个包名（V-06-1 把键常量搬到了 `gfx`） |
| `dsagentteam/toggle_test.go` `switchHit` | `hit.Label == "switch"` | `hit.Role == gfx.RoleSwitch`（`.Label()` 现在写的是用户文案） |
| `dsagentteam/slots_test.go` `hasSwitchHit` | 同上 | 同上 |
| `dscomposer/approval_official_test.go` | 按旧 ID `approval-deny` 找命中区 | 按文案「拒绝」+ `RoleButton` 找，再按它的真实矩形 `Press/Release` |
| `dscomposer/composer_test.go` | 按旧 ID `approval-*` 造点击目标 | 新增 `renderComposerClickLabel`：先画一帧拿真实命中区，再用它的 ID 走事件 pass |
| `dsconversation/official_transcript_paging_test.go` | 按旧 ID `history-older` | 按文案「加载更早」+ `RoleButton` |
| `dsfiles/files_test.go` | 按旧 ID `files-open-external` | 按文案「用默认程序打开」（负例"按钮不该存在"时不点，由调用方的断言兜底） |

**这不是放宽**：点中的仍是同一个按钮、触发同一条回调；只是"怎么找到那个按钮"从 ID 换成了文案/role。
产品侧 `Button`/`ScrollArea` 是否需要在 v0.6.0 里补回显式 ID，属 V-06-4 的设计判断（见 §11.5）。

### 11.5 还剩什么（V-06-4 / 后续；本轮**记录并跳过**）

1. **显式交互 ID 的产品语义**：`Button`/`ScrollArea` 现在只有"位置身份"，跨帧稳定但不可寻址。
   需要一次全量 hits/交互矩阵复核（谁是 Key、谁是位置、有没有两行同时出现导致身份交换）。
2. **`Popup` 的"放不下翻到另一面"**：V-06-2 已登记（`Overlay` 绝对定位不翻面），仍在。
3. **按 Turn 精确恢复滚动位置**：现在靠 `recordScroll`/`applyPendingScroll` + `ScrollTo` 恢复了会话级偏移，
   但"应用自己管滚轮"（`ScrollAt`）那条路没走；若将来要 `ScrollAt`，必须同时接管滚轮。
4. **视觉对照**：本轮只做到"编译 + 35 包测试全绿"，画面对照仍是 V-06-5。
5. **`spike/*` 五个子模块**：各自 `go.mod` pin 在 `viva v0.2.3`，不在 `./...` 门禁内，
   且 V-06-1/V-06-2 按新 API 改过之后**本来就不能编译**（`go.mod` 没跟着升）。本轮不动，
   要恢复得单独一条"spike 模块升 v0.6.0"的活。

### 11.6 证据位置

| 内容 | 路径 |
|---|---|
| 批量改写脚本（Draw 去 theme + 页面构造 + Env.Scroll 删除；`--apply` 才写盘） | `DSH-GO/tmp/GO8-RC2/_v063/migrate.py` |
| 站点扫描（括号/方括号/花括号配平，只认 `ui.Page` 型接收者） | `_v063/scan.py` |
| 起始红点 | `_v063/build0.log`、`_v063/vet0.log` |
| 最终三档 | `_v063/final-build.log`、`final-vet.log`、`final-test.log`（35 ok / 0 FAIL） |
| 产品路径补丁快照（V-06-3 中途） | `_v063/product.patch` |
| 逐条报告 | `DSH-GO/tmp/GO8-RC2/reports/V06-3-page-migration.md` |

## 12. V-06-4 实测（阶段二第四步：控件语义改写 —— 命中区矩阵 / 翻面 / 滚轮）

> 现场：同一隔离 worktree（分支 `chore/viva-0.6.0-stage2`），起点 HEAD `56377f5`
> （= V-06-3 的 `3555eb0` + 把 `origin/main` 合进来那次 merge），**未 commit**（按约定由 Lead 统一提交）。

### 12.1 结论

| 项 | 结果 |
|---|---|
| `CGO_ENABLED=0 go build ./...` | **绿**（退出码 0） |
| `CGO_ENABLED=0 go vet ./...` | **绿**（退出码 0） |
| `CGO_ENABLED=0 go test ./... -count=1` | **绿：35 包 ok / 0 包红 / 0 条 FAIL** |
| `CGO_ENABLED=1 go test -race -timeout 20m ./...` | **绿，RACE_EXIT=0**（无 `DATA RACE`） |
| `gofmt -l .`（不含 `spike/`） | **0** 个文件 |
| `scripts/guard-selftest.sh` | **OK=8 FAIL=0** |
| 改动面 | **10 文件**（改 4：`uikernel/official_model_menu.go`、`dsagentteam/{model_slot,parent_containers}.go`、`dsstatusbar/official_stats_dialog.go`；新增 6：`uikernel/official_popup_place.go`(+_test)、`uikernel/official_popup_flip_test.go`、`cmd/app/{hits_matrix,hits_static_guard,scroll_wheel}_test.go`），`+73 / −16`（不含新增测试文件） |

一句话：**"看着对但点不着"这件事本轮有了可执行的判据**——全量产品 11 个形态的命中区矩阵
（323 个可点元素实例、零面积 0 个）+ 静态 `.Clicked()` 身份守卫（56 处，0 处无身份）+
两处 `Popup().Above()` 的翻面等价实现 + 滚轮/恢复互斥的回归断言。

### 12.2 全量 hits 矩阵（本单最重要的判据）

**做法**：`cmd/app/hits_matrix_test.go` 用 `pluginapi.Builtin()` 装**全部 25 个内置插件**
（不手抄清单——`plugins_assembled_test.go` 已经证明"目录在、没装配"真会发生），
再走产品同一条 `ui.NewPage` + `uikernel.FirstScreen` 路径真画，然后读 `gfx.Canvas.Hits()`。

**11 个产品形态**（`hitsFrames()`，每个形态一份**全新装配**——插件里的 `shown/open`
是真状态，复用产品会让上一帧开着的浮层污染下一帧的清单）：

| 形态 | 可点元素 |
|---|---|
| hero 空首屏 | 10 |
| 有会话有消息 | 30 |
| 发送中（Busy） | 30 |
| 审批待定 | 32 |
| 历史分页五态 | 30 |
| 设置浮层打开（官方四区面板） | 46 |
| 点活动胶囊开统计弹窗（**真点击路径**） | 30 |
| 子代理目录打开 | 30 |
| 右栏关闭 | 28 |
| 侧栏收起（窄窗） | 30 |
| 窄窗 720×600 | 27 |
| **合计（去重前实例数）** | **323** |

**三条判据**：

| 判据 | 内容 | 结果 |
|---|---|---|
| ① 零面积 | 每个命中区 `Width>0 && Height>0`，且无 NaN | **全绿**（用 `Height(0)` 变异验证：红，报 `新建会话 … 252.0x0.0`） |
| ② 有身份 | 每个 `.Clicked()` 的接收者都能追到 `.Hit`/`HitWith`/控件/带 ID 辅助函数 | **全绿**（56 处，用删掉 `dsopen` 的 `.Hit(...)` 变异验证：红） |
| ③ 无身份交换 | 同帧同 `(label, role)` 不存在"两个完全重合但身份不同"的命中区 | **全绿** |

**判据②的静态实现**：`cmd/app/hits_static_guard_test.go` 按函数粒度扫
`plugins/`+`internal/`+`cmd/` 的**非测试** Go 文件；函数里出现 `.Clicked()` ⇒ 该函数内必须
出现身份来源，或接收者来自 `hitsIdentityHelpers` 登记的三个**带 ID 辅助函数**
（`prims.ListRow` / `chromeButton` / `workspaceIconButton`——它们把 id 当参数、函数体内 `.Hit`）。
**登记表是双向的**：辅助函数改名 ⇒ `TestHitsIdentityHelpersStillExist` 红（否则判据②会静默退化成放行）。

### 12.3 `Popup().Above()` 的翻面：实际 **2 处**，不是 3 处

`git grep "Popup()" ab2b9db^`（V-06-2 迁移前的树）只有三处，第三处**没有翻面语义**：

| 站点 | 旧形状 | 是否翻面 |
|---|---|---|
| `internal/uikernel/official_model_menu.go` | `Popup().Above(anchor, 8, End)` | **是** |
| `plugins/dsstatusbar/official_stats_dialog.go` | `Popup().Above(anchor, 8, Start)` | **是** |
| `plugins/dsstatusbar/official_stats_dialog.go`（遮罩） | `Popup().At(0, 0).Size(...)` | **否**（`At` 就是定坐标，没有"另一面"） |

**等价行为**：新增 `internal/uikernel/official_popup_place.go` 的 `PlaceAbove`，判据逐字照
viva v0.5.1 `ui/popup.go` 的 `sideOf`（请求的那侧放得下就用它；放不下时去空间更大的那边；
两侧一样宽时留在请求的那侧，不做无谓抖动）。

**关键设计（踩过一次才知道）**：`PlaceAbove` **只判"哪一侧"，不判坐标**；坐标由调用方
**按边钉**——

- 上方：`.Bottom(bounds.Height - anchor.Y + gap)`（底边贴锚点顶）
- 下方：`.Top(anchor.Y + anchor.Height + gap)`（顶边贴锚点底）

为什么：v0.6.0 的 `Absolute()` 子元素高度由**内容**决定，声明期拿不到。第一版让
`PlaceAbove` 返回算出来的 `top` 并统一 `.Top(top)`，结果**首帧（高度未知=0）面板从锚点顶
往下长、压住 trigger**——`TestModelMenuIsDrawnAboveAnchor` 当场红。按边钉之后，
"没量过高度"也不会有错帧。

高度从**上一帧**量：`dsagentteam` 的 `modelSlotState` 新增 `panel/panelDrawn/panelRect`
（同已有点 `chip/drawn/rect` 那套，在 `afterModelSlot` 里量）；`dsstatusbar` 用它已有的
`dialog.panelRect`。0 = 还没量过 → 按"放得下"处理（与 v0.5.1 首帧一致）。

**行为断言**（`internal/uikernel/official_popup_flip_test.go`，真画一帧再量矩形，不是只测算式）：

- 锚点靠上（Y=40 / 窗 400 高 / 面板 200 高）→ 菜单**整块落在窗口内**且底边 ≥ 锚点底（翻到下方）；
- 锚点贴窗口底部（Y=740 / 窗 800 高，正常产品形态）→ 菜单留在上方，且面积 > 0。

### 12.4 滚轮接管与 `OnScroll` 同步（V-06-3 记的 ③）

**那条坑是什么**：v0.6.0 里 `ScrollAt` 会置 `scrollManual`，而 `view.Wheel` 遇到 manual 区
**直接返回未处理**（`ui/scroll.go` 的 `view.Wheel`）。也就是说"应用自己接管滚轮"和
"应用自己恢复偏移"是**互斥**的两条路；本仓走的是 `ScrollTo`，所以**目前**不冲突——
但没有任何断言守着，哪天有人把 `ScrollTo` 换成 `ScrollAt`，编译过、单测过、门禁全绿，
只有真机上鼠标滚轮滚不动。

`cmd/app/scroll_wheel_test.go` 三条断言（全部真滚轮事件，不是读字段）：

| 断言 | 内容 |
|---|---|
| ① `TestWheelScrollsMessageArea` | `view.Wheel` 对消息区返回 `Handled=true`，且 `Offset` 真的变大、`Repaint=true` |
| ② `TestWheelOffsetReachesScrollPos` | 滚轮之后 `recordScroll` 抄进 `m.scrollPos` 的是**新**偏移（切会话恢复的就是它） |
| ③ `TestScrollToRestoreKeepsWheelWorking` | `settleScroll` 恢复位置之后**滚轮仍然 Handled**（`ScrollTo` 没把 `scrollManual` 打开），且新位置同帧被抄回 `m.scrollPos` |

### 12.5 剩余项（不是"边界"带过）

1. **两个 overlay 不在全量矩阵里**：`dsstatusbar` 的活动胶囊弹窗走 `p.dialog` 内部态、
   `dsagentteam` 的子代理目录要 `p.team.View().CatalogKnown` 数据；全量装配下前者需要
   模拟真实点击链、后者在空快照下 `visible=false`（**官方正确行为**：目录未知就不画）。
   两者的命中区由各自插件测试守（`plugins/dsstatusbar/official_stats_dialog_test.go` 已断言
   面板命中区），**全量矩阵只覆盖 Snapshot 驱动的形态**——这条是明确的覆盖边界，不是漏。
2. **`dsopen` 的"在工作目录里打开"**：只在 `Snapshot` 带 destination 时出现，矩阵里没有该状态；
   它的 `.Clicked()` 身份由静态守卫②守（`Hit(env.Tree.ID(env.Feat, "open-in-app"), …)` 在链上）。
3. **视觉对照**仍是 V-06-5：本轮做到"每个可点元素都有正面积命中区"，**没有**做像素级对照。
4. **`spike/*` 五个子模块**仍各自 pin `viva v0.2.3`，不在 `./...` 门禁内（V-06-3 已登记）。

### 12.6 证据位置

| 内容 | 路径 |
|---|---|
| 命中区矩阵（11 形态 / 323 实例） | `DSH-GO/tmp/GO8-RC2/_v064/matrix-dump.log` |
| 最终三档 | `_v064/11-final-test.log`（35 ok / 0 FAIL）、`build`/`vet` 退出码 0 |
| race | `_v064/20-race.log`（RACE_EXIT=0） |
| 守卫自测 | `_v064/30-guard.log`（OK=8 FAIL=0） |
| 静态 `.Clicked()` 盘点脚本 | `_v064/clicked_audit.py`、`_v064/clicked_detail.py` |
| 只读盘点副本（HEAD `3555eb0`） | `_v064/head/` |
| 逐条报告 | `DSH-GO/tmp/GO8-RC2/reports/V06-4-control-semantics.md` |

---

## 13. 上游回应（2026-11-12 深夜，两个 issue 都已处理）

### 13.1 #239 `text.(*Editor).String()` 并发写缓存 → **not planned 关闭**

上游 `@t8y2` 的处理结论（逐条摘录 + 我们的判断）：

| 上游说法 | 我们的核对 |
|---|---|
| 复现成立：用当前 API 对等改写（`flex`→`ui`，`Field.Bind` + 并发 `Text`/`SetText`），macOS arm64 headless 下实测复现 race | ✅ 与我们 Windows + MinGW 的复现一致 |
| **不打算把 `Editor`/`Field` 改成线程安全对象**；getter ≠ 纯读，`String()`/`CommittedString()` 会填缓存 | ✅ 与我们的代码事实一致 |
| 正确用法：**窗口线程**先取不可变字符串快照，再交给后台；**不要把 Field/Editor 指针交给 worker** | ✅ **正是我们 `pendingDraft` 的做法** |
| 页面作用域任务用 `ui.NewEffectKeyed`（文本快照作 key，`Take()` 应用）；后台结果用 `Invalidator`/`OnWake` 回窗口线程 | 记录：我们的实现是自建队列，等价语义；不迁 |
| 已在 `3b1f9f2` 补齐契约文档 + 无头 race 回归测试 `TestFieldBackgroundSnapshotSurvivesTypingAndDropsStaleResult`，随 **v0.6.1** 发布 | 待 v0.6.1 发布后复验 |

**结论：本仓 `pendingDraft` / `pendingSmoke` 兜底不是 workaround，而是上游认可的**唯一正确用法**——保留，不删。**
`docs/viva-upstream-editor-string-race.md` 里"上游修好后删 `spike/vivaeditor/`"的条件**仍未满足**（上游明确不修）。

### 13.2 #240 `docs/upgrading.md` 漏写 v0.6.0 包重组 → **fixed（`bfcc26b`，随 v0.6.1）**

上游确认我们报的缺口成立，并补齐了：

- `flex` 导入路径移除、page/layout/control API 迁到 `ui`
- 原 `ui` 底层 API 迁到 `gfx`，附代表性映射表 + 版本钉死的包文档链接
- 保留的 page 词汇（`Color`/`Rect`/`Size`/`Point`/`Theme` + 主题/颜色 helper）
- `Frame`/`View` → `Page`/`C` 的编写迁移（含主题配置、Page 持久所有权、嵌入式宿主输入路由、拖放 API）
- **我们没发现的一条渲染迁移**：`ReplaceBlockElements` / `Canvas.BlockRectangles`
  → `ReplaceCellGlyphs` / `Canvas.CellGlyphGeometry`（输出含矩形**和**三角形）

**对我们的一处纠正**：`ui.ID` 的**类型**搬到了 `gfx.ID`；v0.6.0 里的 `ui.ID(c)` 是新的 **page helper 函数**（返回 `gfx.ID`），不是保留的类型别名。`ui.Rect` 才是保留别名。
—— 这正是 V-06-1 里那 9 个"同名坑"的成因，我们的处理（按类型/函数上下文分开改）与该纠正一致。

**渲染迁移自查**：`grep -rn "ReplaceBlockElements\|BlockRectangles" --include=*.go .` → **0 处**（本仓不用这两个 API），无需额外迁移。

### 13.3 后续动作
- 等 **v0.6.1** 发布 → 按 13.1 的交接方式复验 #239（**只在遵守窗口线程所有权的路径上**验；并发 `Text`/`SetText` 仍不受支持）
- 复验通过后才删 `spike/vivaeditor/` 与 `pendingDraft` 兜底（**两个条件同时满足**）
- watcher 已配 `vivaui/viva`（30 分钟一次，每次最多 3 条唤醒）只盯这两个 issue 的后续

---

## 14. V-06-5 实测（阶段二第五步：**视觉终验** —— 区域截图对照 + 像素判据）

> 现场：`chore/viva-0.6.0-final`（HEAD `9c6f37e`，= V-06-1/2/3/4 全部合入后的 v0.6.0）。
> 基线：隔离副本 `git archive 9de99a2`（**`go.mod` = `github.com/vivaui/viva v0.5.1`**，
> 即 V-06-1 的父提交 `3f85ed1` 再合上 PR #47 的那个 main 提交）。
> ⚠ 任务书里写的"v0.5.1 = `3555eb0`"是笔误：`3555eb0` 的 `go.mod` 已经是 `viva v0.6.0`
> （它是 V-06-3）。拿它当基线只能验 V-06-4 那一步，验不到整条迁移——所以本轮改用
> `9de99a2`，理由与证据在 §14.1 的"基线选择"里。

### 14.1 结论

| 项 | 结果 |
|---|---|
| 对照形态 | **9 个**产品形态（`-shell official` 真机）+ **2 份真 `dshd`**（有会话 / 零会话） |
| 对照区域 | **46 个**（形态 × 区域） |
| 逐像素相同 | **27 个**区域（`difference_percent = 0.0000`） |
| 有差异但已归因 | **19 个**区域（全部 ① 上游行为变化；② 我们的迁移缺陷 **0** 处；③ 本来就没有 **0** 处） |
| 超阈值 | **0** |
| 确定性 | 同一个二进制连跑两次：**0 / 983424 像素差**（`-shot` 路径逐字节相同） |
| 三档门禁 | `build=0` / `vet=0` / `test=0`（35 包 ok / 0 FAIL）· `-race` **RACE=0** · `gofmt -l` = 0 · `guard-selftest` **OK=8 FAIL=0** · `parity-count` / `doc-links` / `scripts-index --check` 全 PASS |

**基线选择**（为什么不是任务书写的 `3555eb0`）：

| 候选 | `go.mod` 里的 viva | 能不能当 v0.5.1 基线 |
|---|---|---|
| `3f85ed1`（`d826b18^`） | `v0.5.1` | 能，但**缺 PR #47**（用量弹窗接 `tokenUsage` 真值） |
| **`9de99a2`（选定）** | **`v0.5.1`** | **能**：`3f85ed1` 的后代，含 PR #47；与 v0.6.0 树只差迁移那 4 个提交 |
| `3555eb0`（任务书写的） | `v0.6.0` | **不能**：它已经是迁移后的树，比它等于只验 V-06-4 |

### 14.2 区域对照表（9 形态 / 46 区域，实测）

区域矩形**不是从截图猜的**：它们逐像素等于 `uikernel.ComputeShellLayout` 算出来的
容器（`sidebar=SidebarCol`、`header=Header`、`transcript=ScrollBody`、
`composer=ComposerSeat`、`rightbar=RightbarCol`、`full=Frame`），
由 `cmd/app/visual_regions_contract_test.go` 对拍，骨架改了区域没跟上就红。

| # | 形态 | 窗口 | 区域 | 差异 | 阈值 | 判定 |
|---|---|---|---|---|---:|---|
| 1 | `hero-empty`（零会话宿主） | 1182×832 | sidebar | 0.000% | 2.0% | ✅ 逐像素相同 |
| | | | header（40 高，Blank 态） | 0.000% | 2.0% | ✅ |
| | | | transcript（Hero 首屏） | 0.440% † | 1.0% | ✅ 已归因 |
| | | | composer | 0.000% | 2.0% | ✅ |
| 2 | `active-messages` | 1182×832 | sidebar | 0.000% | 2.0% | ✅ |
| | | | header | 0.000% | 2.0% | ✅ |
| | | | transcript | 1.440% † | 2.0% | ✅ 已归因 |
| | | | composer | 0.000% | 2.0% | ✅ |
| 3 | `active-rightbar`（右栏展开） | 1182×832 | sidebar / rightbar | 0.000% / 0.000% | 2.0% | ✅ |
| | | | header / composer | 0.000% / 0.000% | 2.0% | ✅ |
| | | | transcript | 0.848% † | 1.5% | ✅ 已归因 |
| 4 | `settings-overlay` | 1182×832 | sidebar / header / composer | 0.000% | 2.0% | ✅ |
| | | | transcript | 1.440% † | 2.0% | ✅ 已归因 |
| 5 | `approval-card` | 1182×832 | sidebar / header / composer | 0.000% | 2.0% | ✅ |
| | | | transcript | 0.376% † | 1.0% | ✅ 已归因 |
| 6 | `search` | 1182×832 | sidebar / header / composer | 0.000% | 2.0% | ✅ |
| | | | transcript | 1.367% † | 2.0% | ✅ 已归因 |
| 7 | `demo-tool`（工具卡） | 1182×832 | sidebar / header / composer | 0.000% | 2.0% | ✅ |
| | | | transcript | **2.395%** † | 3.0% | ✅ 已归因（最大一处） |
| 8 | `narrow-720x600` | 720×600 | sidebar / header | 0.000% | 2.0% | ✅ |
| | | | transcript / composer | 1.027% † / 0.550% † | 1.5% / 1.0% | ✅ 已归因 |
| 9 | `light-theme` | 1182×832 | sidebar / header / composer | 0.000% | 2.0% | ✅ |
| | | | transcript | 1.440% † | 2.0% | ✅ 已归因 |

† 这 19 个区域**不是**"差不多就行"——它们逐行定位过，**只落在含等宽片段的行上**：
纯文本行、三栏骨架、header、composer 全是 **0 像素差**（见 §14.3）。

### 14.3 差异归因：19 处全部是 ① 上游行为变化

**逐行定位**（`hero-empty`，把每个 ink 行段单独算最佳 `dy` 与残差）：

| 行段 | 内容 | 最佳 `dy` | 残差 | 结论 |
|---|---|---|---|---|
| y=56..102 / 107..119 / 125..129 / 147..158 | 头部与纯文本行 | +0 | **0.000** | 逐像素相同 |
| y=168..181 / 188..208 / 215..235 / 242..262 / 269..289 | 5 个列表项（**含行内代码**） | **−1** | 6.665 → 1.103 | 整体位移 1px |
| y=304..318 / 326..340 / 364..382 / 420..432 / 469..483 / 496..509 / 522..537 | 其余纯文本行 | +0 | **0.000** | 逐像素相同 |
| y=394..414 / 443..463 / 541..561 | 含行内代码的行 | +0 | 8.052 / 2.195 / 1.554 | **不是位移**，是字形高度差 |

`demo-tool` 更直接：代码卡片的**上边框**在 v0.5.1 是 y=291，v0.6.0 是 y=294
（`[35,35,36]` vs `[21,21,23]`，整行 x=296..1165 共 870 像素）——**卡片整体矮了 3px**。

**最小复现**（`AGENTS.md` §0 要求：先写能独立跑的一段程序）：

| 复现 | 程序 | v0.5.1 | v0.6.0 | 结论 |
|---|---|---|---|---|
| 文字排版 | `ui.Wrap` + `ui.Text` @14px，无头渲染 400×200 | — | — | **0 / 80000 像素差**：viva 的**文字**排版逐像素不变 |
| 等宽单元 | `Canvas.MonoCellSize(size, MonoSystem)` | 12→`6.5977×15.0000`<br>13→`7.1475×16.0000`<br>14→`7.6973×17.0000`<br>15→`8.2471×18.0000`<br>16→`8.7969×19.0000` | 12→`6.5977×**14**.0000`<br>13→`7.1475×**15**.0000`<br>14→`7.6973×**16**.0000`<br>15→`8.2471×18.0000`<br>16→`8.7969×19.0000` | **宽度不变，12/13/14px 的高度各少 1px** |

对应上游 v0.6.0 自述：「gfx 的**等宽排版改为按字体自身的格子度量**……修复 Direct2D
中等/半粗等宽字形」。我们的代码调 `canvas.MonoCellSize(style.FontSize, style.MonoFace)`
拿行高（`internal/mdlite/codeblock.go:408`），所以**自动跟随**——差异不是我们写死的数值。

**归因判定**：① **上游行为变化**（等宽单元高度 −1px，是上游修的度量 bug）。
可接受性由三条钉住：
1. 差异**只**出现在含等宽片段的行上，纯文本行与三栏骨架 0 像素差 → 不是布局回归；
2. 文字**内容**逐行比对完全一致（24 个 ink 行段的起止行号两边相同）→ 没有裁切、没有换行变化；
3. 19 个区域的实测值全部写进 `accepted` 表并带**上界**（实测 ×1.2 向上取整），
   `TestVisualThresholdsAreExplicit` 会拒绝"上界 < 实测"和"缺归因"的条目。

### 14.4 像素判据（自动化，进得了门禁）

| 件 | 内容 |
|---|---|
| `scripts/visual-regions.py` | 9 形态真机截图 → 逐区域像素差（复用 `scripts/visual-diff.py`）→ 按阈值判红 → `report.json` |
| `testdata/visual-v06/regions.json` | 区域契约：矩形 / 类别 / 阈值 / 9 形态 / 19 条 `accepted` |
| `testdata/visual-v06/baseline/*.png` | **v0.5.1 真机截图基线**（9 张，956 KB） |
| `cmd/app/visual_regions_contract_test.go` | 进 `go test`：区域矩形 == `ComputeShellLayout`；基线存在；阈值写全；`accepted` 只能收紧 |
| `scripts/guard-selftest.sh` §7 | **弄坏必红**：把 sidebar 宽度 280 改成 281 → `TestVisualRegionsMatchShellLayout` 当场红（实测已验证） |

**阈值**（写死在契约里，**只准收紧**）：

| 类别 | 阈值 | 为什么是这个数 |
|---|---|---|
| `structural`（侧栏 / header / composer / 右栏 / 状态栏） | **> 2.0% 即红** | 这些是纯几何与色块；同一个二进制两次跑实测 **0 像素差**，2% 只留给字体微调 |
| `content`（消息流正文） | **> 5.0% 即红** | 正文有抗锯齿文字与换行，跨版本允许少量重排 |
| `full`（整窗兜底） | **> 1.0% 即红** | 任何区域漏检都会在这里露头 |

19 条 `accepted` 的 `max_percent` 是**实测 × 1.2 向上取整到 0.5%**，每一格都 ≤ 对应类别阈值
（`transcript` 最紧的一格 1.0%，最松 3.0%）——**没有一格比阈值松**。

**跑法**（要真窗口 + 真宿主；进不了 `go test` 默认路径，所以配套的契约守卫进 CI）：

```bash
# 1) 两份临时 dshd：一份带会话（基线与当前都从它取同一份数据），一份零会话（Hero 首态）
#    临时 home + --port 0；会话文件 mtime 冻结（否则侧栏「N 分钟前」会漂）
# 2) 两个二进制各截 9 张
export VISUAL_ADDR=… VISUAL_COOKIE=… VISUAL_ADDR_EMPTY=… VISUAL_COOKIE_EMPTY=… VISUAL_SESSION=…
python scripts/visual-regions.py --binary <v0.6.0.exe>            # 与基线比
python scripts/visual-regions.py --binary <v0.5.1.exe> --update   # 重刷基线（复核过差异才准用）
```

### 14.5 剩余缺口（不是"边界"带过）

1. **`-shot` 注入不了鼠标**：活动胶囊 → 统计弹窗、子代理目录、`dsopen` 的"在工作目录里打开"
   这几个**必须点开**的浮层不在像素基线里。它们的命中区与几何由 V-06-4 的
   `hits_matrix_test.go`（323 个实例 / 11 形态）+ 各插件测试守，**但"点开之后画得像不像"本轮没有像素证据**。
   补法：给 `cmd/app` 加"启动就打开某个浮层"的调试开关（V-06-4 的 `-demo-approval` 就是这个形状），
   每个浮层一条基线。本轮**未做**（要改产品代码，超出视觉终验的写作用域）。
2. **`-expand-think` 在本视口没有可见差异**：实测与 `active-messages` **逐字节相同**
   （该会话尾部没有思考块）。已从基线里撤掉，避免"假覆盖"；要覆盖得换一条带 reasoning 的会话。
3. **目标条没覆盖**：种子会话没有 `goal`，`-shot` 也没有"设一个目标"的开关。
4. **只有 deepseek 深/浅两套令牌**：`-theme light` 已覆盖；系统跟随（`system`）不在像素基线内
   （它依赖桌面设置，不是确定性输入）。
5. **`spike/*` 五个子模块**仍各自 pin `viva v0.2.3`，不在 `./...` 门禁内（V-06-3 已登记）。

### 14.6 证据位置

| 内容 | 路径 |
|---|---|
| 区域对照报告（46 行，机读） | `DSH-GO/tmp/GO8-RC2/_v065/diff-final/report.json` |
| 逐区域 diff 明细（9 个） | `DSH-GO/tmp/GO8-RC2/_v065/diff-final/*.diff.json` |
| v0.5.1 / v0.6.0 截图（各 9 张） | `_v065/baseline-v051/`、`_v065/baseline-v060/` |
| 最小复现（两份程序 + 输出） | `_v065/repro/v051`、`_v065/repro/v060`、`_v065/repro/mono051`、`_v065/repro/mono060` |
| 三档门禁日志 | `_v065/build.log`、`_v065/vet.log`、`_v065/test.log`、`_v065/race.log` |
| 守卫反证 | `_v065/guard.log`（OK=8 FAIL=0） |
| 逐条报告 | `DSH-GO/tmp/GO8-RC2/reports/V06-5-visual-final.md` |
