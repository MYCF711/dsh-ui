# 交付物按文件 diff（#14 `dsh-client-ui-deliverables`）的接线证据与实现台账

> 建立时间：2026-11-15 · 对应 [official-parity.md](official-parity.md) 第 14 行（🟡 PARTIAL → ✅ IMPLEMENTED）。
> 解锁它的宿主契约：`/api/changes.summary|diff|open`（`HOST:docs/transport-contract.md` §15.3）。

**锚点**：`OFF:` = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-deliverables/`；
`HOST:` = 宿主仓 `feat/host-contract-gaps`；`GUI:` = 本仓。

---

## 0. 官方包是什么（四件）

`package.json` description 逐字：

> Changed-files card with per-file comparison tabs, delivery cards, and clickable final-response file references for Web

| 官方能力 | 本仓 | 落点 |
|---|---|---|
| Changed-files card | ✅ | `GUI:plugins/dsdeliverables/deliverables.go`（从 `write`/`edit` 工具参数提路径）+ 侧栏「写过的文件」 |
| **per-file comparison（点文件看 diff）** | ✅ **这一轮补的** | `GUI:internal/apiclient/changes.go` + `GUI:plugins/dsdeliverables/changes.go` |
| delivery cards | ❌ | 触发语义在官方源码里仍是"回复里显式的交付声明"，本仓没有那份约定 |
| clickable final-response file references | ✅ | `GUI:plugins/dsconversation/file_refs.go` |

---

## 1. 三条路由是**裸 JSON**，不是 typert 方法

官方消费端（`OFF:src/changes.ts:6-8`）直接 `fetch` 相对路径再 `response.json()`，
**没有 `{result:…}` 信封**。所以客户端不能复用 `internal/apiclient` 的 `Call`
（那是 `POST /api/<method>` + 信封）。新增的 `changesDo`
（`GUI:internal/apiclient/changes.go:275`）刻意做两件事与 `Call` 不同：

1. 路径由调用方给全（`/api/changes.summary` 本身就是路径），不拼 `/api/`+方法名；
2. **不解析信封**，正文原样交给调用方。

解不动怎么办：`ChangesSummary` 额外断言 `turn >= 1`（官方 `isChangesSummary` 的判据，
`OFF:src/changes.ts:68`）——拿到 `{"result":…}` 时 `json.Unmarshal` 会"成功"解出一个
全零结构体，只靠 unmarshal 的错误码是挡不住的。`ChangesDiff` 则按 `kind` 分支校验
（`text` 支的每行必须带 `+`/`-`/空格 前缀，官方 `isHunk` 判据 `OFF:src/changes.ts:77`）。

状态码 → 分类（客户端只看状态码，`OFF:src/client/changes-diff.ts:19-25`）：

| 状态 | 分类 | 界面行为 |
|---|---|---|
| 400 | `ChangesBadRequest` | 参数错（不该发生；报真实错误，不静默） |
| 404 | `ChangesMissing` | **「这一轮没有变更记录」** + 退回全文预览（非 git 会话的已登记裁剪） |
| 422 | `ChangesNativeUnavailable` | 本机没有原生打开通道，**不是**可重试失败 |
| 501 | `ChangesNotWired` | 宿主没接这三条路由 → 整页退回旧路径（全文预览） |
| 500 | `ChangesFailed` | 可重试失败，浮层给「重试」 |

---

## 2. seq 从哪来（**不许猜**）

摘要按 `(sessionId, seq)` 寻址，`seq` 是那条 `workspace/changes` 事件在 **Transport
journal** 里的序号。插件从 `pluginapi.Snapshot.Frames` 里筛
`type == "workspace/changes"` 取**最新**一条的 `seq`
（`GUI:plugins/dsdeliverables/changes.go:125` 的 `LatestChangesSeq`）。
没有这条帧 = 这一轮没有变更记录（宿主对非 git 会话不写记录，也不伪造空摘要）。

`index` 是摘要 `files[]` 的**下标**，不是路径：所以必须先拿 summary 再按下标要 diff
（`ChangedIndex`，`:155`）。路径比较做了规范化（Windows 大小写不敏感 + 斜杠统一 +
相对/绝对两种形状）。

---

## 3. 渲染（按官方 `FileDiff.tsx` 的取舍）

| 官方 | 本仓 | 说明 |
|---|---|---|
| hunk 头 `@@ -a,b +c,d @@` | ✅ `hunkHeaderText`（`:224`） | 逐字 |
| 每行 `旧号 \| 新号 \| sign \| 正文` | ✅ `hunkRows`（`:231`） | sign 三色 |
| `binary` / `oversized` | ✅ 只显示官方状态词、**零行 hunk** | 不伪造空 diff |
| `coarse` | ✅ 按契约读、恒 false | 宿主用 git 自己的 diff，没有"行比较超时"那件事 |
| 渲染上限 | ⚠️ 300 行（官方 DOM 5000） | 截断文案报**真实画出的行数**，不报"5000 行" |
| 正负底色 `--dsw-alias-file-diff-*` | ⚠️ 取 `t.Success`/`t.Error` 与同色 12% 底 | 六个 diff 令牌不在 `pluginapi.Theme`；`pluginapi/**` 不在本轮写作用域，已在代码注释登记 |
| 原生打开按钮（`present-open.ts`） | ⚠️ 契约面已接（`ChangesOpen`），**不画按钮** | 官方那枚按钮是 slot 贡献项，本包内没有它的文案，**不编** |
| split / wrap 开关 | ❌ | 那是官方右栏 review 面板的控件，不是侧栏交付物卡片的 |

数据面注入走**包级注入点**（`SetChangesSource`，`:75`），与 `dsattachment.SetSink`
同一个形状：插件拿不到 `*apiclient.Client`（红线 R2），壳在
`GUI:cmd/app/official_panels_wiring.go` 的 `wireOfficialPanels` 里转一手。

---

## 4. 验收（本仓可复跑）

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
CGO_ENABLED=0 go test ./internal/apiclient/... ./plugins/dsdeliverables/... -count=1
```

- `GUI:internal/apiclient/changes_test.go`：200/400/404/422/501 + 裸 JSON（信封**必须解不出**）
  + cookie + `kind`/行前缀校验 + 空 `files` 不伪造；
- `GUI:plugins/dsdeliverables/changes_test.go`：缓存键、`LatestChangesSeq`、`ChangedIndex`、
  `planOverlay` 六种结果（无记录 / 失败 / diff 缺失 / binary / oversized / 正常）；
- `GUI:plugins/dsdeliverables/render_test.go`：真 `ui.NewPage` 画帧 + 真点 Hit，
  断言画面上真的出现 hunk 行与「这一轮没有变更记录」。
