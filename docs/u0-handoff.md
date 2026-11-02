# U0 交接：界面线要宿主侧给什么

> 2026-11-03 · 给写 `DSH-GO/internal/transport/` 的人看。
>
> **这份文档只写"界面需要什么"，不写"宿主怎么实现"**——实现是宿主线的自由，
> 我们只钉住**契约**。契约的机器可读版本在 `testdata/api.snapshot.json`（13 个 method + WS 帧形状）。

---

## 一、为什么需要这份文档

现在的 `cmd/mockd` 是**过渡管**：它伪造 `/api`，再转发给 `dshd acp`。
它把界面接通了，但有三笔债：

| 债 | 代价 |
|---|---|
| **续会话靠复制副本** | 每次点开同一会话都重新复制，上次聊的不带过来 |
| **事件是 ACP 的投影，不是 v4 原始词汇** | 压缩/遮蔽的语义在中间被翻译了一次 |
| **home 不是一个** | mockd 读 `home/sessions`，dshd 写 `tmp/dshd-home` |

U0 做完这三笔债一起清掉：**home 只有一个，事件是 v4 原始词汇，续会话不需要复制**。

---

## 二、13 个 method（界面真的在用的）

全在 `testdata/api.snapshot.json` 里，这里只写**界面依赖的语义**。

### 会话

| method | 界面用它做什么 | 必须保证 |
|---|---|---|
| `session.list` | 左栏列表 | 每个会话至少给 `id` / `title` / `cwd` / `updatedAt`。**title 要能看懂**（现在靠读第一条 user 消息，宿主侧可以直接给） |
| `session.load` | 点开一个会话 | 给 `messages[]`（`seq`/`role`/`text`）+ `total` + `truncated`。**`role` 必须区分 `user`/`assistant`/`tool`**，`tool` 带 `tool`/`args`/`summary` 三字段（工具卡要它） |
| `session.history` | gap 补洞 | **断流后按 seq 增量补**，不是整页重拉 |
| `session.prompt` | 发消息 | 立刻返回（`accepted`+`turnId`），**回答走 WS**，不要在这里阻塞 |
| `session.cancel` | 中断 | 取消后 `busy` 必须清（否则下一次 prompt 永远 409） |
| `session.create` | 「新建」按钮 | 返回新 `id` |
| `session.delete` | 现在没接 UI | 可以先不做 |
| `session.fork` | 现在没接 UI | 可以先不做 |

### 文件 / 配置

| method | 状态 |
|---|---|
| `workspace.list` / `fs.read` / `fs.write` | **契约里有，界面还没用**。等 U2 的文件面板 |
| `config.get` / `config.set` | 同上 |

**建议**：U0 第一版只做**会话那 6 个 + WS**，文件和配置留到 U2。少做一半，早点能用。

---

## 三、WS 帧形状（这个不能改）

```jsonc
// 客户端发
{"t":"sub", "topic":"session:<id>", "sinceSeq": 123}   // sinceSeq 可省 = 从头
{"t":"unsub", "topic":"..."}
{"t":"ping"}

// 服务端发
{"t":"ev",   "topic":"...", "seq": 124, "kind":"assistant/message", "data":{...}}
{"t":"gap",  "topic":"...", "fromSeq": 130, "toSeq": 145, "reason":"..."}
{"t":"pong"}
```

### 三条硬要求

1. **`seq` 必须连续**。界面靠它判 continuity——断号就发 `gap`，客户端按 `session.history` 补。
2. **打断不许整体刷新**。长会话整页重拉会闪，用户会以为丢东西了。
3. **`kind` 用 v4 原始词汇**（62 种那一套），**不要为 GUI 造第二套事件名**。
   压缩/遮蔽的语义只有一份真相，翻译一次就多一个出错的地方。

### 界面现在认得的 kind

| kind | data | 界面怎么用 |
|---|---|---|
| `assistant/message` | `{"delta": "…"}` 或 `{"done": true}` | 逐字累积 / 落成一条 |
| `tool/call` | `{"tool": "bash", "args": "…", "summary": "…"}` | 工具卡 |
| `approval/asked` | `{"tool": "bash", "cmd": "…", "reason": "…"}` | 审批气泡（**先命令、后自述**，自述标注不可信） |
| `user/message` | v4 原样 | 用户自己的消息 |

**其他 kind 界面会忽略，不会崩。** 所以宿主可以放心吐全量 v4 事件。

---

## 四、认证与端口（界面的铁律，宿主必须配合）

| 铁律 | 为什么 |
|---|---|
| 端口一律 `--port 0` 由 OS 分配 | 写死端口 = 多开必冲突 |
| 壳从 `ready{url}` 拿地址 | 同上 |
| launch token → `/connect` 换 cookie | **cookie 永不进 WebView**：渲染进程能读 token = 一个 XSS 交掉整台机器 |

> 现在 mockd 没有认证（本机回环）。U0 落地时这三条要一起上，
> 否则「本机回环」这个假设会被自己打破（比如以后支持远程）。

---

## 五、宿主侧要知道的三个坑（我们踩过）

### ① 审批的 id 必须**原样**回

```go
// 错：转成 int 再回，类型不匹配就静默失配
var id int
json.Unmarshal(raw, &id)
reply(id)

// 对：原样透传
reply(json.RawMessage(rawID))
```

**现象**：整轮挂住、**日志里什么都没有**、看起来像"模型不响应"。
这和 `DSH-GO/AGENTS.md` 里记的「必须应答 `session/request_permission`」是同一件事的两面——
**应答了但 id 不对，一样挂住**。

### ② `busy` 标志必须有唯一的结束点

ACP 模式下我们踩过：`runACPPrompt` 是唯一清 `busy` 的地方，漏了它第二次 prompt 永远 409。
宿主侧同理：**turn 结束（成功/失败/取消）都必须在同一处清**。

### ③ 全量解码真实会话要 6 秒

182 个会话全量解 = **6.1 s**。只解 header + 第一条 user 消息（前 64KB）是 **624 ms**，
差 **10 倍**。

**建议**：`session.list` 不要解正文，正文留给 `session.load`。

---

## 六、界面这边需要改什么（U0 落地时）

| 文件 | 改什么 |
|---|---|
| `cmd/mockd` | **整个删掉**（它就是过渡管） |
| `cmd/app/main.go` | 只改 `-addr` 的默认值（指向真宿主），其余不动 |
| `internal/apiclient` | 不动——它只认契约，不认谁在服务 |

**这正说明契约先行是对的**：界面一行不用改，换个后端就能跑。

---

## 七、验收线

U0 做完了，这三条应该同时成立：

1. `session.load` 一个**真实历史会话** → 发消息 → **回答带着那个会话的上下文**
   （现在靠复制副本，U0 后应该不需要复制）
2. `home` 只有一个：`dshd` 读写的和界面看到的**是同一个目录**
3. 断流（拔网线 / 杀 WS）后重连，**历史不闪、只补缺口**
