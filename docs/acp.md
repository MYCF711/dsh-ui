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
