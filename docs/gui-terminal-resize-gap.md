# GUI 终端尺寸接 PTY resize：上行已接通（HTTP 回落投影）

> 状态：**已接通**（2026-11-19）。面板量出的 cols/rows 现在真的发给宿主，
> 宿主 PTY 的列行数跟着变——端到端证据见 §7。本文前半部分保留"当初卡在哪"
> 的取证记录（它解释为什么走的是 HTTP 加法方法而不是 mux 上行），
> 现在的事实以下面 §7 为准。基线 `dsh-ui@origin/main 0bddb2f`、
> 宿主 `DSH-GO@origin/main 5d8e7dd`（dsh-go#79 `terminal.resize` 已合）。

## 1. 目标

面板比宿主屏幕窄时，只按整格裁掉右侧是**已知差距**。官方做法是把面板量出来的
cols/rows 发给宿主，让 PTY 真的变成那么宽（`xterm.resize` + `model.resize`）。

## 2. 官方口径（触发时机 / 算法 / 节流）

官方源码在 `app.asar` 内（`D:\DeepSeek Harness\Desktop-0.2.2-rc.2\resources\app.asar`）：

| 事实 | 出处 |
|---|---|
| 触发：`new ResizeObserver(measure)` 观察终端 DOM 节点 | `dsh-client-ui-sidebar-terminal/src/client/terminal.tsx:126-127` |
| 另有重排点：可见性/可写性变化、字号与字体族变化、挂载初测 | 同文件 `:169-175`、`:145-155`、`:100-138` |
| 算法：`FitAddon.proposeDimensions()` → `min(maxCols)`/`min(maxRows)` → **`cols<2 \|\| rows<1` 直接 return** → `xterm.resize` + `model.resize` | 同文件 `:191-200`（`fitScreen`） |
| 客户端上行帧：`{type:'resize', sequence, cols, rows}`，同尺寸直接 return | `dsh-api-terminal-controller/lib/types/client/model.js:212-223` |
| 服务端收帧：`case "resize"` → 校验 `2 <= cols <= maxCols`、`1 <= rows <= maxRows` → `handle.resize` + `screen.resize` + 广播 state | `dsh-api-terminal-controller/lib/index.js:622-633` |
| **节流：没有** | `terminal.tsx` 全文无 `setTimeout`/`setInterval`/`requestAnimationFrame`/`debounce`/`throttle`（grep 零命中）。去重只有两条：ResizeObserver 的按帧合并 + `model.resize` 的同尺寸 return |

结论：**不自造节流参数**。同尺寸不发、尺寸变了就发，这就是官方的全部节奏。

## 3. 宿主侧真相（取证当时）：`terminal.resize` 没有 HTTP 面

> **这一节是历史**：它证明"官方 uplink 只在 mux 面"而不是路由漏读。宿主在
> dsh-go#79 之后补了 additive 的 `POST /api/terminal.resize`
> （`DSH-GO/internal/transport/handlers_terminal.go` 的 `handleTerminalResize`
> + `cmd/dshd/serve_terminal_resize.go` 的 `serveTerminalResizeHook`），
> GUI 走的就是这条。下面的原始取证保留，说明为什么需要这个加法方法。

任务书里「`cmd/dshd/terminal_pty.go:849` 有 `terminal.resize` 路由」这一条**读错了**：

- `terminal_pty.go:849` 是 `a.lookup(sessionID, id, "terminal.resize")` 的**第三个入参**
  （方法名只用于错误文案），不是路由注册。真正的实现是
  `func (a *officialPTYTerminalAPI) Resize(sessionID, id string, cols, rows int) error`
  （`cmd/dshd/terminal_pty.go:848-872`）。
- HTTP 面注册的方法只有：`terminal.open/input/output`（`internal/transport/api.go:85-87`）
  与官方 `terminal.create/list/close/environment/shells/rename`（
  `internal/transport/handlers_job_terminal_official.go:89-96`），流方法
  `terminal.attach/retain/watch`。**没有 `terminal.resize`**（`Method` 常量表逐条核过）。
- `Resize` 的唯一入口是 attach 流的 **uplink 帧机**：
  `cmd/dshd/serve_terminal_api.go:644-659`（`case "resize"`）。
- 这条 uplink **只在 `/api/remote.mux`（officialwire）上存在**：
  `cmd/dshd/serve_official_wire_stream.go:56` 的 `wrapOfficialMuxUplinkStream` 只给实现了
  `transport.OfficialTerminalUplink` 的流套上行外壳；宿主自己的文档也这么写：
  「官方 wire 的上行只在 mux 面（attach 的 write/resize）；本仓 HTTP 面的一问一答
  没有上行通道」（`cmd/dshd/serve_terminal_api.go:1045-1047`），
  以及 gap register 的残留项「L1 HTTP 面只回首帧 snapshot」
  （`DSH-GO/docs/host-stable-gap-register.md:544`）。

## 4. GUI 当初为什么发不出去（历史）

`internal/apiclient` 里**没有任何 officialwire / `/api/remote.mux` 客户端**（grep 零命中），
终端的 attach 走的是 HTTP NDJSON 投影（`internal/apiclient/terminal.go:153-193`）。
要走 mux 上行，还得多做三件现在都没有的事：

1. **第二份凭据**：`/api/remote.mux` 用官方 v2 cookie（`dsh-auth-<sha256(authority)>`），
   只能由 `GET /connect?token=…&interactions=none|answer` 换取
   （`internal/officialwire/server.go:281-331`）。GUI 现在只兑过 L1 的
   `/connect?token=`（`internal/apiclient/client.go:217-241`），没有官方这一支。
2. **一把 socket 一条 Peer**：同一个 Peer 同时只允许一条 API socket，第二条 409
   （`internal/officialwire/mux.go:104-112`）。多条终端视图必须复用同一条连接、按
   `streamId` 多路复用，不能一终端一连接。
3. **换 attach 通道**：宿主只有**最新 attach** 持输入权
   （`cmd/dshd/terminal_pty.go:959-970` 的 `controller` 判定）。若 HTTP attach 与 mux
   attach 并存，HTTP 这条会变 `writable=false`，界面上终端会突然变成只读。
   所以真接 resize 就得把 attach 整体挪到 mux，HTTP 那条只能留作回落。

## 5. 两条收口路径（**A 已被采纳并落地**）

**A. 宿主加一个 additive 的 HTTP 一元方法 `terminal.resize`**（小，30–60 行 + 测试）
与既有的 `terminal.input`/`terminal.output` 回落投影同型（`serve_terminal_api.go:1114-1170`）：
入参 `{sessionId|agentId, id, cols, rows}`，内部直接调已有的
`officialPTYTerminalAPI.Resize`。GUI 侧随后就是纯加法：
`apiclient.TerminalResize` + `pluginapi.Actions.TerminalResizeOfficial` +
壳里按 `terminalResizeWanted` 去重后调用，**不需要** officialwire 客户端。
代价：官方那条真通道仍未被 GUI 用上（L2 兼容性不动）。

**B. GUI 实现 officialwire mux 客户端**（大，跨层）
：官方 connect → 一次 v2 cookie → 单 socket 多路复用 → attach 挪到 mux（write 也随之上行，
官方语义）→ resize 作为 uplink item 发。这是**真 parity**，但它是一整块 L2 wire 客户端，
不属于「接 resize」这一单的量级，且要重写 `cmd/app` 的 attach 读循环与三处测试夹具。

## 6. 已落地

- `plugins/dsterminal/resize.go`：`terminalFitSize`（官方 FitAddon 除法 + `environment`
  上下限夹取 + `cols<2||rows<1` 拒发）、`terminalResizeWanted`（官方同尺寸去重）、
  `requestTerminalResize`（按官方判据决定发不发，并把"已请求尺寸"记成 `state.info`
  的乐观副本——本仓触发是绘制帧，不像 ResizeObserver 那样按帧合并）。
- `plugins/dsterminal/grid.go`：绘制面按 `terminalFitSize` 决定可见列行，
  宿主 `environment.maxCols/maxRows` 真正生效；fit 结果经 `paintGrid` 的 `onFit`
  回调**在裁剪之前**交给上行（屏幕比面板小时也要把 PTY 放大）。
- `internal/apiclient/terminal.go`：`TerminalResize(ctx, sessionID, id, cols, rows)`，
  参数就是宿主 handler decode 的那四项（平铺，不包 `params`）。
- `pluginapi.Actions.TerminalResizeOfficial`：壳侧实现
  `cmd/app/terminal_wiring.go` 的 `terminalResizeOfficial`——异步发、挡住
  `cols<2||rows<1` 与越过 `environment` 上限的尺寸、成功后把本地 VT 屏幕
  `vterm.Resize` 同步到新尺寸（否则下一帧又算出一个"不同"的 fit）。
- 契约快照 `testdata/api.snapshot.json` 收录 `terminal.resize`（batch27；当时 total 129，`credentials.unset` 加入后为 130）。
- 测试：`plugins/dsterminal/resize_test.go`（面板宽度→列数、上限夹取、去重语义、
  上限真的到绘制面、**尺寸变→发一次 / 同尺寸→不发 / 夹取后与实际发送一致 /
  壳没接不发 / 未 connected 不发**）、`internal/apiclient/terminal_test.go`
  （参数形状 + 宿主越界错误原样返回）、`cmd/app/terminal_wiring_test.go`
  （生产接线：Actions → HTTP 形状 → 本地 VT/Info 同步）。
- `internal/vterm.Resize` 口径此前已被上一单钉住
  （`internal/vterm/vterm_test.go:98-115` `TestResizeKeepsContent`：尺寸变、
  内容不丢）。宿主 `internal/terminal/vtscreen.go` 的「resize 不重排折行」是**已登记
  差异**，本单不修、也不据此改断言。

## 7. 端到端证据（真宿主 + 真 GUI，2026-11-19）

做法：宿主 `dshd.exe` 由 `DSH-GO@origin/main 5d8e7dd` 现编（含 `terminal.resize`），
GUI 由本分支现编；GUI 与宿主之间串一条**只记请求不改字节**的 TCP 隧道，
专门记 `/api/terminal.resize` 的请求行与请求体。

```text
1795038283.106 POST /api/terminal.resize body={"cols":69,"id":"tab8","rows":42,"sessionId":"7e6e4325-…"}
（空闲 8 秒，GUI 仍在逐帧绘制 → 计数仍是 1：同尺寸不发）
1180x800 → 1620x1100 拉宽窗口后：
1795038319.608 POST /api/terminal.resize body={"cols":101,"id":"tab8","rows":62,"sessionId":"7e6e4325-…"}
（再空闲 6 秒 → 计数仍是 2）
```

宿主那条 PTY 里真的 `stty size`（`terminal.input` 写命令 + `terminal.output` 读回）：

```text
拉宽前: 42 69
拉宽后: 62 101     ← terminal.list 同时报 cols=101 rows=62
拉窄回: 62 69
```

结论：**面板尺寸 → fit → HTTP resize → 宿主 PTY 列行数** 这条链真的通了，
且一次尺寸变化只发一条请求。仍未用上的是官方 mux uplink（§5 的 B 路径），
那是另一单的量级；登记在这里，不冒充 parity。
