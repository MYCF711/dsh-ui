# Transport 契约 v1（U0）

> 机器可读版本：`testdata/api.snapshot.json`。**改契约先改快照**，diff 必须出现在提交里。
> 本文是人类可读版，两者不一致时**以快照为准**。

---

## 0. 三条设计决定

| # | 决定 | 理由 |
|---|---|---|
| 1 | **不复刻官方的 `{path:[...], key}` 信封**，入参**平铺**（`{"id":…}`） | 官方那个信封是为 "service key 路径寻址" 设计的，我们只有 13 个 method；**平铺是真宿主 handler 的实际解包方式**，再套一层 `params` 会让参数静默变零值（§1.1） |
| 2 | **路径保持 `/api/<method>`** | 将来若要兼容官方客户端，加一层映射即可；现在不背它的复杂度 |
| 3 | **事件用 v4 的 62 种原始词汇**，不另造 | 压缩/遮蔽语义只有一份真相（对齐 `DSH-GO/AGENTS.md` R5、UIPLAN C4） |

---

## 1. 一元调用

```
POST /api/<method>
Content-Type: application/json
{ "id": "s-001", "tailMessages": 3 }        ← 参数直接放顶层，不套 params

200 { "result": { ... } }
4xx/5xx { "error": { "code", "message", "hint", "retryable" } }
```

### 1.1 为什么入参是平铺（2026-11-03 实测，别再踩）

**真宿主就是平铺 decode。** `../DSH-GO/internal/transport/server.go` 的每个 handler 都是
`var in struct{…}; json.NewDecoder(r.Body).Decode(&in)` —— 直接解到平铺结构：
`session.prompt:330`、`session.cancel:352`、`session.create:369`、`session.load:412`、
`session.approve:434`、`session.history:464`；fs/config 的 handler 在
`handlers_fs.go` / `handlers_config.go` 里同样平铺解。宿主自己的契约测试
`../DSH-GO/internal/transport/envelope_test.go:74` 发的也是 `{"id":"abc","tailMessages":2}`。

**曾经的坑：** 界面 apiclient 发的是 `{"params":{…}}`，宿主 decode 到平铺结构，于是
**每个字段都是零值**。症状极隐蔽：

- `session.list` 照常返回（它根本不需要参数）→ 看着一切正常；
- `session.load` 报 `no_such_session` → 看起来像"这个会话不存在"；
- `session.prompt` 起的子进程拿到空的 `--session` / `--prompt` → 看起来像"模型不回答"。

**排错时不会有人先怀疑信封**，所以这条必须留在文档里。唯一真相是**平铺**：

| 谁 | 发什么 | 收什么 |
|---|---|---|
| 新代码（界面、脚本、测试） | **只发平铺** | — |
| 真宿主 `dshd` | — | **只收平铺**（套 `params` 等于没给参数） |
| `cmd/mockd` | — | 平铺为准；**`{"params":{…}}` 作为向后兼容继续收**（老 curl 示例、`cmd/acpsmoke`、历史脚本） |

mockd 的兼容层只在一处实现（`decodeParams`，`cmd/mockd/main.go`），13 个 method 共用；
它**不代表契约**——拿 mockd 能把包裹形状跑通，不能证明真宿主也认。

**错误码**：

| code | 何时 | retryable |
|---|---|---|
| `not_found` | 会话/路径不存在 | false |
| `busy` | 该会话有 turn 在跑 | **true** |
| `invalid_arg` | 参数不合法 | false（`hint` 写清怎么改） |
| `unsupported` | 本版本未实现 | false |
| `internal` | 宿主内部错误 | false |

> `hint` 不是可选的：**模型和人都要靠它决定下一步**（对齐宿主线 R3）。

---

## 2. 流：`/ws`（WebSocket，H14 唯一基线）

```
客户端 → { "t":"sub", "topic":"session:<id>", "sinceSeq": 123, "journalEpoch": 7 }
服务端 → { "seq":124, "type":"user/message", "data":{...}, "time":1793693560098 }
服务端 → { "type":"pong" }
服务端 → { "type":"session/reset", "data":{"journalEpoch":8,"lastSeq":0} }   ← 无 seq 的控制帧
```

| 字段 | 说明 |
|---|---|
| 路径 | **`/ws`**。`/api/mux` 是界面早期的自定形状，已废弃，不要留两套 |
| 控制帧（客户端） | `t=sub` / `t=unsub` / `t=ping`（应用层心跳，不占用 WS 控制帧） |
| `t=sub` 的两个水位 | `sinceSeq`（收到哪了）+ `journalEpoch`（那一代是第几代）。**两个要一起带**，见 §3.1 |
| 事件帧（服务端） | `{seq,type,data,time}`；**没有 topic 字段**——连接本身就是会话归属 |
| `type` | v4 原始事件名，或 `pong`。不是 `{t:"ev",kind:…}` |
| `seq` | 由宿主每会话 Transport journal 统一分配；`pong` 无 seq、不入日志 |
| 连接粒度 | **一个会话一条连接**，不得在同连接混订多个会话 |
| 服务端不发 gap | 断号由**客户端**按 `seq` 连续性自己发现，再调 `session.history` 补缺失区间 |

**连续性规则**：
- 客户端记录 `lastSeq`；收到 `seq != lastSeq+1` 时，把 `[lastSeq+1, seq-1]` 作为缺口交给调用方；
- 调用方用 `session.history` 只补这一段（**不是**整页 `session.load`——`load` 是首屏视图投影）；
- **绝不"重连就整体刷新"**（长会话会闪、滚动位置会丢）。

> 2026-11-03 实测：真宿主 `/api/mux` 回 404（路径不对）、改 `/ws` 后回 401（`Mux` 没带 cookie）。
> 两处都修掉才连上，见 `docs/progress.md`。

---

## 3. 方法清单（**14 个**）

> 2026-11-03 核对：按宿主 `internal/transport` 里实际注册的 method 字符串逐个数，
> 是 14 个（`session.approve` 是 H13 加的，文档长期写着 13 个，漏的就是它）。
> 数一遍的命令：`grep -oE '"(session|workspace|config|fs)\.[a-z]+"' internal/transport/*.go | sort -u`

### 第一批（8 个，U1 之前必须全通）

「接受的入参名」这一列是**代码里真的解了哪些名字**（逐个从 `cmd/mockd/main.go` 的入参结构体
和宿主 handler 的结构体读出，不是设计意图）；机器可读版见快照 `methods.*.req`。

| method | 用途 | 接受的入参名（平铺） |
|---|---|---|
| `session.list` | 会话列表 | —（宿主忽略 body；`cwd`/`limit`/`cursor` 尚未实现） |
| `session.create` | 新建 | `mode`（`cwd`/`title` 客户端会发，宿主暂不读） |
| `session.load` | 打开会话（含尾页） | `id` / `sessionId`、`tailMessages` / `limit` |

### `session.load` 返回的 `messages[].reasoning`

宿主**只在正文为空时**才带这个键（`transport_source.go`："有正文时思考是草稿，
不是已经给用户的回答"）。所以拿到非空 `reasoning` 就等价于 `text == ""`。

界面必须按 `role=thinking` 渲染它——**和实时流同一条规则**
（`Stream.Messages()` 也是把思考落成 thinking）。不这么做的话，同一个回答
在两条路径上长得不一样：

```
正在跑：       看到一个思考块
切回来之后：   正文是空的 → 空气泡，或者干脆不见了
```

转换点只有一个：`apiclient.ReasoningOnlyThinking`（在载入解码里调用，见
`cmd/app/wpc1_data.go`）。刻意只认"正文为空 + reasoning 非空"这一种形状——
宿主根本不会发别的形状，多认一种就等于自己发明渲染规则。
| `session.history` | 断号补洞（**原始 frames**，不是 messages 投影） | `id` / `sessionId`、`sinceSeq`、`limit`、`journalEpoch` |
| `session.prompt` | 发消息 | `id` / `sessionId`、`text` |
| `session.cancel` | 取消 turn | `id` / `sessionId` |
| `session.approve` | **回传一次审批决定** | `requestId`、`optionId`（**注意：没有 sessionId**） |
| `workspace.list` | 工作区根 | — |
| `config.get` | 读配置 | —（宿主忽略 body；`keys` 尚未实现） |

`session.approve` 的三条硬约束（真机验过，见 `docs/product-acceptance.md`）：

- `optionId` 只接受宿主给过的值，**实测就是这三个**：`allow_once` / `allow_always` / `reject_once`（**下划线**，不是连字符）；
- 不是这三个 → 400 `bad_option`，提示原文 `optionId 必须是 allow_once / allow_always / reject_once 之一`；
- 请求已经应答或超过 120s → 404 `no_such_request`（`这个审批请求已经不在了（已应答或已超时）`）；
- 回 `{"accepted":true,"optionId":…}` 才算成功；**界面只在自己 200 且 accepted 时才收气泡**。

`allow_always` 是真生效的：宿主会记住「这个会话的 bash 以后都允许」，下一次直接
`早先已选择「以后都允许」，本轮自动放行`，不再推 `approval/request`。

别名（同一个字段两个名字）**两边都收**，冲突时的取法逐字对齐宿主：
`sessionId` 优先于 `id`（`server.go:395` `fillID`），`limit` 优先于 `tailMessages`
（`server.go:417` `firstNonZero(in.Limit, in.TailMessages)`）。
只认一个就是静默 404 —— 这是 H9/H10 的成因，别再加第三个名字。

### 3.1 `journalEpoch`：水位属于哪一代（U0 收口）

一个会话的 journal 会**换代**（宿主重启换了内存里的 journal，或会话被重建）。
换代之后 `seq` 从 1 重新数，所以"我收到 277 了"这句话在下一代里毫无意义。
`journalEpoch` 就是用来把水位钉在某一代上的整数。

| 位置 | 字段 | 语义 |
|---|---|---|
| `session.load` 响应 | `journalEpoch` / `lastSeq` | 当前世代的编号与该 journal 的末尾 seq。**订阅前的基线就取这两个** |
| `t=sub` 控制帧 | `journalEpoch` | 客户端手里的世代。`0` = 还没有基线，宿主不核（首次订阅） |
| `session.history` 请求 | `journalEpoch` | **必带**，每批都带同一个值（`0` = 还没有基线，宿主不核） |
| `session.history` 响应 | `journalEpoch` / `lastSeq` / `frames` | 同一个 journal 的原始帧；形状与 WS 事件帧一致 |
| `session/reset` 帧 | `data.journalEpoch` / `data.lastSeq` | 无 `seq` 的控制帧：宿主发现你在旧世代上 |

**不匹配的两条出口**（都不是"重试一下就好"）：

- `session.history` → **HTTP 409**，`error.code = "stale_epoch"`，并附当前
  `journalEpoch` / `lastSeq` 与 `hint`（原文：`丢弃旧水位，重新 session.load 拿 journalEpoch/lastSeq`）；
- WS → 推一条无 `seq` 的 `session/reset`。

**客户端收到这两个中的任何一个，动作完全一样**：

1. 丢掉这一代的水位——`seq` 与 `epoch` **一起**丢，`seqs`、`lastSeq`、订阅结构里那份都要清；
2. 重新 `session.load` 拿新基线（`journalEpoch` + `lastSeq`）；
3. 用新基线重连（`t=sub` 带新的 `journalEpoch` 与 `lastSeq`），**不要带旧 `since`**。

> 第 3 步用"新基线"而不是恒定的 `0`：从 0 重连会把新 journal 整条重放一遍，而那些
> 内容刚刚已经被 `load` 画到界面上了，结果是同一条消息出现两次。**只有在拿不到
> 新基线（load 失败 / 新 journal 本来就是空的）时，才从 `0` 开始。**
>
> 局部变量也算一份水位：界面 `cmd/app/main.go` 的 `runSubscription` 里重连用的是
> 局部 `since`，只删 map 不清它，重连照样带着旧值。这条曾在复核里被点名为
> 「reset 后局部 `since` 未归零」。

界面上这两条路的实现与断言：

| 行为 | 实现 | 断言 |
|---|---|---|
| `session/reset` 清三份水位 + 局部 `since=0` | `model.onJournalResetLocked`（`cmd/app/main.go`） | `TestJournalResetZeroesEveryWatermark`、`TestJournalResetOfBackgroundSessionKeepsForegroundWatermark`、`TestJournalResetToleratesMissingState`（`cmd/app/reset_watermark_test.go`） |
| history 请求带 `journalEpoch`（且分批每批都带） | `apiclient.History` / `apiclient.FillGap`（`internal/apiclient/history.go`） | `TestHistorySendsJournalEpoch`、`TestFillGapForwardsEpoch`（`internal/apiclient/history_test.go`，httptest 抓请求 body） |

> 快照 `testdata/api.snapshot.json` 仍是 v1 旧形状（`stream.path=/api/mux`、服务端帧用
> `kind`、history 返回 `messages`、缺 `session.approve`）。本次只改了实现与本文，
> **没有动快照**——整份重写归契约 owner（`DSH-GO` W0 / 本仓 W2），两边要一起改。

### 第二批（5 个，U1 之后）

| method | 接受的入参名（平铺） | mockd |
|---|---|---|
| `session.delete` | `sessionId` | 501 |
| `session.fork` | `sessionId`、`title` | 501 |
| `fs.read` | `path` | 501 |
| `fs.write` | `path`、`content` | 501 |
| `config.set` | `values`（扁平 key→string 表） | 501 |

> 宿主已经实现了 `session.approve`（`requestId` / `optionId`，H13），但快照 v1 的 13 个
> method 里还没有它。要补进契约时**两边快照一起改**（`AGENTS.md` §2），别只在实现里加。

---

## 4. 为什么是这 13 个

用户能干的五件事，逐条对到 method：

| 用户动作 | 需要的 method |
|---|---|
| 看到一个会话列表 | `session.list` |
| 点开一个会话，看到历史和滚动位置 | `session.load` + `session.history`（翻页） |
| 打字发出去，看到流式回答 | `session.prompt` + `topic:turn:<id>` |
| 中途停掉 | `session.cancel` |
| 换工作区 | `workspace.list` + `config.get` |

**少一个，界面就会逼出一条直连后门**——那正是 R7 要拦的。
