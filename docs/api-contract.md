# Transport 契约 v1（U0）

> 机器可读版本：`testdata/api.snapshot.json`。**改契约先改快照**，diff 必须出现在提交里。
> 本文是人类可读版，两者不一致时**以快照为准**。

---

## 0. 三条设计决定

| # | 决定 | 理由 |
|---|---|---|
| 1 | **不复刻官方的 `{path:[...], key}` 信封**，用 `{params:{...}}` | 官方那个信封是为 "service key 路径寻址" 设计的，我们只有 13 个 method，扁平更省事 |
| 2 | **路径保持 `/api/<method>`** | 将来若要兼容官方客户端，加一层映射即可；现在不背它的复杂度 |
| 3 | **事件用 v4 的 62 种原始词汇**，不另造 | 压缩/遮蔽语义只有一份真相（对齐 `DSH-GO/AGENTS.md` R5、UIPLAN C4） |

---

## 1. 一元调用

```
POST /api/<method>
Content-Type: application/json
{ "params": { ... } }

200 { "result": { ... } }
4xx/5xx { "error": { "code", "message", "hint", "retryable" } }
```

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

## 2. 流：`/api/mux`（WebSocket）

```
客户端 → { "t":"sub", "topic":"session:<id>", "sinceSeq": 123 }
服务端 → { "t":"ev", "topic":"...", "seq":124, "kind":"user/message", "data":{...} }
服务端 → { "t":"gap", "topic":"...", "fromSeq":130, "toSeq":140, "reason":"..." }
```

| topic | 内容 |
|---|---|
| `session:<id>` | 该会话的 v4 事件（62 种原始词汇） |
| `turn:<id>` | 助手流式增量：`text-delta` / `reasoning-delta` / `tool-call-delta`，带 `revision` / `index` |

**连续性规则（照抄官方语义，这是它做对的地方）**：
- `seq` 必须连续；**断号发 `gap` 帧**，客户端用 `session.history` 补洞；
- **绝不"重连就整体刷新"**（长会话会闪、滚动位置会丢）。

---

## 3. 方法清单（13 个）

### 第一批（8 个，U1 之前必须全通）

| method | 用途 | 关键参数 |
|---|---|---|
| `session.list` | 会话列表 | `cwd?` `limit=50` `cursor?` |
| `session.create` | 新建 | `cwd?` `mode=default` `title?` |
| `session.load` | 打开会话（含尾页） | `id` `tailMessages=200` |
| `session.history` | 向前翻页 | `id` `beforeSeq?` `limit=200` |
| `session.prompt` | 发消息 | `id` `text` `attachments[]` |
| `session.cancel` | 取消 turn | `id` |
| `workspace.list` | 工作区根 | — |
| `config.get` | 读配置 | `keys[]` |

### 第二批（5 个，U1 之后）

`session.delete` · `session.fork` · `fs.read` · `fs.write` · `config.set`

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
