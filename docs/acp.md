# 界面接上了真的 dshd（ACP）

> 2026-11-03 12:05 · 端到端跑通 · 不是设计，是实测

## 一句话

`mockd -acp <dshd.exe>` 之后，界面发一条消息会**真的走模型**：
`session.prompt` → ACP `session/prompt` → 模型 → 流式 chunk → `/api/mux` → 界面。

## 怎么起

```bash
./mockd -sessions "D:/DeepSeek Harness/home/sessions" \
        -acp "D:/DeepSeek Harness/default-workspace/DSH-GO/dshd.exe" \
        -acp-manifest "D:/DeepSeek Harness/default-workspace/dsh-ui/tmp/dsh-acp.toml"
```

日志里会有两行：
```
真实会话已载入: 180 个（扫到 180 个文件，跳过 0 个坏文件）
ACP 就绪：session=acp-09a55de5… cwd=D:/DeepSeek Harness/default-workspace/dsh-ui
```

## 实测输出（两轮，都真跑）

**第一轮（不调工具）**
```
prompt → HTTP 200 {"result":{"accepted":true,"turnId":"t-101"}}
2。
=== 流结束 ===
```

**第二轮（调工具 + 审批）**
```
prompt → HTTP 200 {"result":{"accepted":true,"turnId":"t-101"}}

  [工具] bash
  [审批] bash: echo hello-dsh-go
输出就是 `hello-dsh-go`。
=== 流结束 ===
```

**注意最后一行的反引号**：那是模型自己写的 Markdown 行内代码，说明文本是原样透过来的，没被我们转义。

## 三个踩到的坑（每个都会让整件事"看起来像模型不响应"）

### ① `dsh.toml` 列的模块比二进制里多

`dshd acp` 起来就退：
```
dshd: manifest 指定的模块 "subagent" 没有编译进二进制；
可用模块: [exec fs history search tool-ask tool-edit …]
```
`dsh.toml` 写了 30 个，当前二进制只有 17 个。**必须给一份只含已编译模块的 manifest**——
`tmp/dsh-acp.toml` 就是为此生成的（`scripts` 没写，手工生成一次即可）。

### ② `session/request_permission` 的 id 必须原样回

第一版我把 id `json.Unmarshal` 成 `int` 再回。**类型不匹配时静默失配**：
模型一直在等审批，整轮挂住，日志里什么都没有。

改成**原样回 `json.RawMessage`** 之后立刻就通。

> 这条和 `DSH-GO/AGENTS.md` 里记的那条是同一件事的两面：那边是"不应答会永久挂起"，
> 这边是"应答了但 id 不对，一样永久挂起"。

### ③ `busy` 标志在 ACP 模式下没人清

`session.prompt` 进来先置 `busy[id]=true`。假回答那套里是 `commitPending` 清的；
走 ACP 时没人清 → 第二次 prompt 永远 `409 busy`。

修：`runACPPrompt` 的 `defer` 里清（它是唯一的结束点）。

## ACP 与我们的 `/api/mux` 怎么对应

| ACP update | mux 帧 |
|---|---|
| `agent_message_chunk` | `{"kind":"assistant/message","data":{"delta":"…"}}` |
| `agent_thought_chunk` | `{"kind":"assistant/thought","data":{"delta":"…"}}` |
| `tool_call` / `tool_call_update` | `{"kind":"tool/call","data":{"tool":"…"}}` |
| `session/request_permission` | 先回 allow，再发 `{"kind":"approval/asked","data":{"tool":"…"}}` |
| `session/prompt` 返回 | `{"kind":"assistant/message","data":{"done":true}}` |

**没有做的**：`session/load` 走 ACP（现在真实会话还是读文件）、多会话并发、
重连、`session/cancel` 转发。都是后续。

## 一个重要限制

`session/new` 建的是一个**新会话**，不是列表里那个真实会话的继续。
所以现在「打开历史会话 → 发消息」实际是：历史来自文件，回答来自一个新 ACP 会话。

**要做成"续上那个会话"，得用 `session/load` 或 `session/resume`**，见
`DSH-GO/AGENTS.md` 里 ACP 三方法的记录。这不在本次范围里。

---

## 续会话（session/resume）：代码写了，但**跑不通**——限制写在这里

实测 2026-11-03 07:00：

```
ACP resume session-1710f716-… 失败（回落到新会话）：
  session/resume: 恢复失败: 按 cwd 算出的位置没有
    （tmp\dshd-home\sessions\--E-Documents-deepseek-harness-default-workspace--\…），
  按 id 也没找到: 找不到会话 session-1710f716-…（在 tmp/dshd-home 下）
```

### 根因：**home 不是一个**

| | 用的 home |
|---|---|
| mockd 读会话（`-sessions`） | `D:/DeepSeek Harness/home/sessions`（真实） |
| dshd acp 子进程（`tmp/dsh-acp.toml` 起的） | **`tmp/dshd-home`**（隔离的测试 home） |

所以 dshd 在自己那个空 home 里当然找不到真实会话，**按 id 按 cwd 都找不到**。

### 三条可走的路（都不在本次范围）

| 方案 | 代价 |
|---|---|
| 给 dshd 传 `--home "D:/DeepSeek Harness/home"` | **会写真实 home**，和正在运行的 DSH 抢会话文件（虽有 `.lock`，但风险不该由测试承担） |
| resume 前把目标会话文件**复制**进 `tmp/dshd-home/sessions/<同结构>` | 要复刻 `projectKey` 目录算法；复制后是**副本**，聊完写进副本，真实历史不会变 |
| **在宿主侧实现 `internal/transport`**（UIPLAN §4 的 U0） | 正道。界面直连真宿主，不走 ACP 绕路 |

**结论**：现在的状态是「**能聊，但不能续真实会话**」。这不是 mockd 的 bug，是**绕道 ACP 的架构代价**。
真要续，就得走第三条——那正是 `UIPLAN.md` 里 U0 应该在宿主侧做的原因。

---

## 续会话（第二轮）：用「暂存副本」跑通了

上面那条结论是**上一版**。之后换了个做法：**resume 前把会话文件复制进 dshd 的 home**。

### 做法

```
真实：D:/DeepSeek Harness/home/sessions/<projectKey>/<seg(id)>/session.v4.jsonl.zstd
   ↓ 复制（只读源，写副本）
ACP ：<cwd>/tmp/dshd-home/sessions/<projectKey>/<seg(id)>/session.v4.jsonl.zstd
   ↓ session/resume {sessionId, cwd}
dshd：读副本、带着历史上下文回答、写回副本
```

`projectKey` 与 `encodeSegment` 是按 `DSH-GO/AGENTS.md` §4 第 1、2 条复刻的
（`cmd/mockd/realsession.go`）。dshd 的默认 home 是 `<cwd>/tmp/dshd-home`
（`cmd/dshd/acp.go:388`：`envOr("DSHD_HOME", "tmp/dshd-home")`）。

### 实测（2026-11-03 06:22）

```
ACP 已暂存会话到 …	mp\dshd-home\sessions\--E-Documents-deepseek-harness-default-workspace--  session-1710f716-…\session.v4.jsonl.zstd

问：只回答两个字：好。

答：好的，我明白了。我只回答你想听的结论，不生成脚本，也不继续往下做额外的事情。
    如果你需要我写出这个迁移脚本，直接说一声就行。
```

**注意这个回答**：它提到了「迁移脚本」「不生成脚本」——**那是历史会话里的事**。
前几轮不带 resume 时，同样的问法只回「收到」「续上」这种没有上下文的两个字。

### 副作用检查

| 检查 | 结果 |
|---|---|
| 真实文件的 total | **仍是 16，没变** |
| 副本文件 | 429,691 字节，mtime 是刚才 |
| 结论 | 聊天写进**副本**，真实历史一个字节没动 |

### 这仍然不是终局

副本是**一次性的**：每次点开同一个会话都会重新复制，上次聊的内容不会带过来。
真正的续会话要 U0（`internal/transport`）直连真宿主——那时 home 就是一个，不需要复制。

代码留着（`ACPClient.Resume` + `acpResumed` 缓存），换到真 home 之后就能用。
