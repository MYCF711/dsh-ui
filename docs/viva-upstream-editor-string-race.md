# Viva 上游缺陷：`text.(*Editor).String()` 不是只读（下游兜底说明）

> 状态：**已定位、已兜底、上游未修**。上游仓 `github.com/vivaui/viva`，版本 **v0.2.3**（`go.mod` 钉的同一份）。
> 发现于 `GUI-GU · dsh-ui 全仓 -race 竞态清理`（共享任务 `task-37`），`-race` 的 5 条报告里有 4 条追到这一条路径。
> 最小复现：`spike/vivaeditor/`（带 `viva_race_repro` 构建标签，默认不参与门禁）。
> 上游 issue 草稿：`$DSH_HOME/outbox/vivaui-viva-editor-string-race.md`（**未发**，对外动作需用户当次明确授权）。

---

## 1. 结论先行

| 项 | 结论 |
|---|---|
| 缺陷 | `text.(*Editor).String()` 名为取值，实现里**会写** `editor.text` / `editor.textValid`；`Editor` 内部**没有任何** `sync.Mutex` / `sync/atomic` |
| 谁踩到 | 任何"一个 goroutine 读 `flex.Field.Text()`、另一个 goroutine 编辑同一个 Field"的用法。**这不是调用方误用 API——getter 自己就是写者** |
| 本仓影响 | 草稿写回（`cmd/app/wpc1_send.go` 的 `dispatchSend` 失败路径）、`-smoke` 自发送、插件 `SetDraft` 动作——三处都在后台 goroutine 里碰过 `*flex.Field` |
| 兜底 | **单 goroutine 所有权**：后台只排队（`pendingDraft` / `pendingSmoke`），写回统一在 UI 线程的 `applyPendingDraft()` 里做；测试按壳的快照契约在锁内拷贝，不把模型切片交给绘制线程 |
| 要不要提 issue/PR | **要提 issue**（getter 写 + 无同步，一句话可复现，属于上游并发契约缺失）；**不建议直接提 PR**：改法有两种（让 `String` 只读、或给 `Editor` 加锁），是上游的 API/性能取舍，不是我们该替他定的 |
| 上游修好之后 | `spike/vivaeditor/` 整个目录删掉；本文降级为历史记录。**产品侧的排队机制不要回退**——"UI 线程独占 widget 状态"本身是对的架构，与上游修不修无关 |

---

## 2. 上游事实（只读源码取证）

源码：`$(go env GOMODCACHE)/github.com/vivaui/viva@v0.2.3/text/editor.go`

```go
// text/editor.go:60-94
	// text caches String's conversion so a paint that reads the value every
	// frame does not allocate; every mutation of runes clears textValid.
	text      string
	textValid bool
...

// String returns the current UTF-8 text. The conversion is cached until the
// text next changes, so repeated calls between edits allocate nothing.
func (editor *Editor) String() string {
	if !editor.textValid {
		editor.text = string(editor.runes)   // ← 写
		editor.textValid = true              // ← 写
	}
	return editor.text                        // ← 读
}
```

| 证据 | 位置 |
|---|---|
| `String()` 在缓存失效时写 `text` / `textValid` | `text/editor.go:87-94`（写：`:89`、`:90`；读：`:88`、`:92`） |
| 每次改 runes 都清缓存 | `text/editor.go:388`（`replace`）、`:597`（`replaceExact`） |
| `Editor` 无锁 | 全文 `grep -n "sync\.\|Mutex\|atomic" text/editor.go` → **0 命中** |
| `Field.Text()` 直接转调 | `flex/field.go:41`：`func (field *Field) Text() string { return field.ensure().String() }` |
| `Field.SetText()` 走 `Insert` | `flex/field.go:44-48` → `text/editor.go:181`（`Insert`）→ `:388`（清缓存） |

上游自己的注释说明了动机（"paint 每帧读一次不分配"），也就是说这是**有意**的缓存，只是没考虑跨 goroutine 访问——`Editor` 的并发契约从未写下来。

---

## 3. 最小复现（独立 `go test`）

位置：`spike/vivaeditor/editor_string_race_test.go`，构建标签 `viva_race_repro`（**默认不参与** `go test ./...`，否则本仓 `-race` 门禁会红）。

```bash
cd "D:/DeepSeek Harness/default-workspace/dsh-ui-rc2"
export PATH="/c/Program Files/Go/bin:/c/toolchains/mingw64/bin:$PATH"
CGO_ENABLED=1 go test -race -tags viva_race_repro ./spike/vivaeditor/ -run 'TestEditor|TestField' -v
```

实跑（2026-11-12）：`exit=1`，`WARNING: DATA RACE` **7 条**，两侧都落在上游文件上：

```
WARNING: DATA RACE
Write at 0x00c0000fc250 by goroutine 11:
  github.com/vivaui/viva/text.(*Editor).replace()
      .../viva@v0.2.3/text/editor.go:388
  github.com/vivaui/viva/text.(*Editor).Insert()
      .../viva@v0.2.3/text/editor.go:181
  ...TestEditorStringIsNotARead.func2()   // 写者：UI 线程继续编辑

Previous read at 0x00c0000fc250 by goroutine 10:
  github.com/vivaui/viva/text.(*Editor).String()
      .../viva@v0.2.3/text/editor.go:88
  ...TestEditorStringIsNotARead.func1()   // 读者：只是想拿当前文本
```

两个测试各自命中：

| 测试 | 钉住什么 |
|---|---|
| `TestEditorStringIsNotARead` | 根因：`String()` 自己写缓存 |
| `TestFieldTextIsNotARead` | 产品形状：`flex.Field` 是 Viva 推荐给业务持有的输入框状态，`Field.Text()` 并发 `Field.SetText()` 就竞争 |

---

## 4. 本仓兜底（带标注，可回退）

### 4.1 产品侧：草稿只由 UI 线程写

`cmd/app` 的模型新增两个**排队槽**，形状与既有的 `pendingTheme` 完全一致（那个也是"包级/UI 状态只能绘制线程写"）：

```go
// cmd/app/main.go（model 字段）
pendingDraft *draftRestoreRequest // 草稿写回（失败发送时把原文还回去）
pendingSmoke *string              // -smoke 的"灌草稿并发送"

// cmd/app/wpc1_send.go
func (m *model) applyPendingDraft()  // **只在 Paint（UI 线程）里调**
func (m *model) writeDraft(text string, overwrite bool) // 唯一允许写草稿的地方
func (m *model) queueDraftOverwrite(text string) // 插件 SetDraft 动作
func (m *model) queueSmokeSend(text string)      // -smoke 后台 goroutine
```

接入点：

| 位置 | 之前 | 现在 |
|---|---|---|
| `cmd/app/main.go` `cfg.Paint` | 只消费 `pendingTheme` | 紧跟其后消费 `pendingDraft`（`applyPendingDraft()`） |
| `cmd/app/wpc1_send.go` `dispatchSend` 失败路径 | 后台 goroutine 直接 `m.draft.Text()` + `SetText()` | 在 **status 同一个临界区**里排队 `pendingDraft`；写回由 UI 线程做 |
| `cmd/app/main.go` `refreshSession` 的 `-smoke` 路径 | 后台 goroutine 直接 `SetText` + `send()` | `queueSmokeSend`，UI 线程消费时再 `writeDraft` + `send()` |
| `pluginapi.Actions.SetDraft` 接线 | 直接 `SetDraft` | `queueDraftOverwrite`（注释标了"空转接口"，但仍先修掉跨线程写） |

为什么"排队 + 同一临界区"能保证确定性（不是把竞态挪到别处）：`pendingDraft` 的写入与 `m.status = "发送失败: …"` 在**同一次 `m.mu` 临界区**内，所以任何观察到失败状态的读者，一定已经能看到这次排队——测试因此可以确定性消费它，不需要 sleep 猜时序。

### 4.2 测试侧：按壳的快照契约拷贝

`cmd/app/wpc1_headless_test.go` 原来**只建一次** `pluginapi.Snapshot` 并直接塞 `m.outgoing[m.cur]`（切片与模型共享底层数组），而 `dispatchSend` 的 goroutine 会就地改元素 → `drawOutgoing ↔ dispatchSend` 竞争。现在按生产 `draw` 的口径，**每帧在 `m.mu` 内重建快照并拷贝**（`Msgs` / `MessageMeta` / `TurnEvents` / `Outgoing` 四份都拷）。

顺带修掉同一测试里两处**未加锁**读 `m.status` / `m.cur` 的地方（只在 `t.Fatalf` 参数里，仍是竞争）。

### 4.3 没有用（也禁止用）的手段

`t.Skip`、删断言、放宽超时、`-race` 排除包——一条都没用。修的是"谁拥有这份状态"，不是"让检测器闭嘴"。

---

## 5. 删除/回退条件

| 触发 | 动作 |
|---|---|
| 上游让 `String()` 只读（或给 `Editor` 加同步），且 viva 版本升级进 `go.mod` | 删 `spike/vivaeditor/` 整个目录；本文顶部状态改成"上游已修（版本 x.y.z）" |
| 上游明确拒绝修（"调用方自行同步"） | 保留兜底，本文保留；**不要**在 `$GOPATH` 里打补丁（AGENTS.md §0：不许直接改 viva 源码） |
| 无论哪条 | 产品侧排队机制**不回退**：UI 线程独占 widget 状态是正确的架构，不是 workaround |

---

## 6. 上游 issue 草稿

见 `$DSH_HOME/outbox/vivaui-viva-editor-string-race.md`（标题、正文、复现命令齐全）。**未发**：对外动作需要用户当次明确授权（`$DSH_HOME/AGENTS.md` 铁律 8）。
