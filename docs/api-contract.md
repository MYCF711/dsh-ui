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
客户端 → { "t":"sub", "topic":"session:<id>", "sinceSeq": 123 }
服务端 → { "seq":124, "type":"user/message", "data":{...}, "time":1793693560098 }
服务端 → { "type":"pong" }
```

| 字段 | 说明 |
|---|---|
| 路径 | **`/ws`**。`/api/mux` 是界面早期的自定形状，已废弃，不要留两套 |
| 控制帧（客户端） | `t=sub` / `t=unsub` / `t=ping`（应用层心跳，不占用 WS 控制帧） |
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

## 3. 方法清单（13 个）

### 第一批（8 个，U1 之前必须全通）

「接受的入参名」这一列是**代码里真的解了哪些名字**（逐个从 `cmd/mockd/main.go` 的入参结构体
和宿主 handler 的结构体读出，不是设计意图）；机器可读版见快照 `methods.*.req`。

| method | 用途 | 接受的入参名（平铺） |
|---|---|---|
| `session.list` | 会话列表 | —（宿主忽略 body；`cwd`/`limit`/`cursor` 尚未实现） |
| `session.create` | 新建 | `mode`（`cwd`/`title` 客户端会发，宿主暂不读） |
| `session.load` | 打开会话（含尾页） | `id` / `sessionId`、`tailMessages` / `limit` |
| `session.history` | 向前翻页 | `id` / `sessionId`、`sinceSeq`、`limit` |
| `session.prompt` | 发消息 | `id` / `sessionId`、`text` |
| `session.cancel` | 取消 turn | `id` / `sessionId` |
| `workspace.list` | 工作区根 | — |
| `config.get` | 读配置 | —（宿主忽略 body；`keys` 尚未实现） |

别名（同一个字段两个名字）**两边都收**，冲突时的取法逐字对齐宿主：
`sessionId` 优先于 `id`（`server.go:395` `fillID`），`limit` 优先于 `tailMessages`
（`server.go:417` `firstNonZero(in.Limit, in.TailMessages)`）。
只认一个就是静默 404 —— 这是 H9/H10 的成因，别再加第三个名字。

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
