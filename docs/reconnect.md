# 断线恢复 · 真宿主实测报告

> 验证对象：`tmp/dshd-serve.exe serve`（真宿主）+ `tmp/app-real.exe`（Go 原生界面）
> 全部进程都在本机回环上，临时 home = `tmp/rc-home`，端口固定 `127.0.0.1:7811`。
> 报告时间：2026-11-03 17:50–18:15。**实验期间 `cmd/app/main.go` 被外部改动过两次**（见 §1），
> 所以每条结论都标了它来自哪个二进制。

---

## 0. 一句话结论

| # | 问题 | 结论 |
|---|---|---|
| 1 | 宿主重启后界面能否自愈 | **不能。** 断线检测、带水位重订、1s→30s 退避都正常；但宿主重启后 cookie 作废，WS 握手恒 401，界面没有重新 `/connect` 的路径，永远重试。 |
| 1b | 宿主进程活着、只是链路断（中继被剪） | **能。** 断开→带水位重订→`since=` 正是断线前水位（5 / 340 / 354，不是 0），重订只发 `GET /ws`，没有整页 `session.load`。 |
| 2 | 真宿主上会不会断号 | **客户端视角会。** 宿主 journal 的 seq 域本身连续；但 **WS 重订不重放 journal**，重订后第一帧的 seq 必然跳号 → 客户端报 gap。旧二进制因此死循环（7 次 gap、连接 4→8）；新二进制只记录后跳过，缺的那段没人补。 |
| 3 | 断线期间的数据能否补回 | **宿主有、界面没补回来。** 断线期间的 turn 完整落盘（`session.load` 能读到完整的 `END-6`），`session.history` 也能按水位返回原始帧；但界面**一次都没调 history**（请求轨迹 0 次），且硬重启时连重订都过不了 401。 |

---

## 1. 环境与版本（重要：实验期间代码在动）

| 物 | 值 |
|---|---|
| 仓库 | `D:/DeepSeek Harness/default-workspace/dsh-ui`，HEAD `98dcf3b`（工作区有未提交改动） |
| 宿主 | `tmp/dshd-serve.exe`，14,548,992 B，mtime 17:44 |
| 界面（旧） | `tmp/app-real.exe`，14,015,488 B，mtime 17:46 —— 跑 A / D / E / F / G |
| 界面（新） | `tmp/app-real.exe`，14,027,264 B，mtime 18:04:55，md5 `33938bda68f06290bb23510b3b08c083` —— 跑 H / I / J |
| 源码现状 | `cmd/app/main.go` mtime 18:07:04（**比新二进制还新**）。`git diff` 显示 gap 分支刚从 `break`（重订）改成 `continue`（只记录） |

这带来一个直接后果：**同一个"断号"事件，两个二进制行为不同**。

- 旧（17:46）：`mux 断号 …，带水位重订（不整页刷新）` → `break` → 外层 `Subscribe(同一水位)` → 再断号 → **无限循环**。
- 新（18:04:55）：`mux 断号 …（待用 session.history 补洞；不整页刷新）` → `continue` → 继续消费后面的帧，**缺的那段跳过、不补**。

两种都不调 `session.history`（见 §4）。

**实验期间没有改任何 `.go` 文件**；只读源码 + 跑进程 + 写 `docs/reconnect.md` 和 `tmp/rc-*`。

---

## 2. 方法：为什么需要一个 TCP 中继

要把两件事分开测：

1. **客户端断线重连逻辑**（退避、水位、重订）—— 需要"链路断、宿主进程还在、凭据还有效"。
2. **宿主真重启** —— 这一步会连 cookie 一起废掉（宿主 `cookies` 是进程内存 map，见 §4.1），
   测的就不再是"客户端能不能重连"，而是"凭据能不能跨进程"。

所以用 `tmp/rc-proxy.py`（7812/7813/7814/7815/7817 → 7811，裸 TCP 中继）在中间剪一刀：
剪断时宿主毫发无损，界面看到的就是一次真实的连接中断。`tmp/rc-proxy3.py` 是同款 + 逐条记录 HTTP 请求行，
用来回答"界面到底发没发 history"。

---

## 3. 三件事逐条

### 3.1 宿主重启后界面能否自愈 —— **没验到（做不到）**

做法：界面连上宿主（直连 7811）→ `taskkill /F /PID <宿主>` → 同端口、同 home 重起宿主 → 观察界面。

断开与退避部分**完全符合预期**（app D，旧二进制）：

```
18:00:37 mux 已订阅 session:8fe1204a-… since=0
18:01:07 mux 连接断开 session:8fe1204a-…，带水位 75 重连
18:01:08 mux 订阅失败（连不上 ws://127.0.0.1:7811/ws：… actively refused it.），1s 后重试 …
18:01:09 … 2s 后重试 …
18:01:10 … 4s 后重试 …
18:01:14 … 8s 后重试 …
18:01:22 mux 订阅失败（… expected handshake response status code 101 but got 401），16s 后重试 …
18:01:38 mux 订阅失败（… 101 but got 401），32s 后重试 …
```

同端口重起（PID 19284）之后，界面**不出所料地一直 401**，直到它自己按 `-shot-wait` 退出。
第二次复现（app E，18:02:29 / 18:03:01）同样是 401。

**为什么**：`../DSH-GO/internal/transport/server.go:98` 的 `cookies map[string]struct{}` 是进程内存，
每次启动还会重新生成一次性 token（`newSecret()`），所以**重启 = 所有旧 cookie 作废**。
界面这头的 cookie 是启动时固定的（`cmd/app/main.go` 的 `-cookie` 分支只 `SetCookies` 一次），
`runSubscription` 里没有任何重新 `/connect` 或读新 token 的路径。

结论：**"宿主重启后自愈"在真宿主上不成立，卡在凭据，不卡在重连逻辑。**

顺带量到一个数字：退避日志最后一档是 **32s**，不是 30s。
`if backoff < 30*time.Second { backoff *= 2 }` 在 `time.After(backoff)` 之后执行，32s 那一次已经睡过了。

### 3.1b 链路断、宿主还活着 —— **验到了（带水位重订）**

app A（经中继 7812，旧二进制）：先跑完一轮拿到水位 5 → 剪断中继 → 恢复。

```
17:56:34 mux 已订阅 session:8fe1204a-… since=0
17:57:10 mux 连接断开 session:8fe1204a-…，带水位 5 重连        ← 断开前的水位
17:58:13 mux 已订阅 session:8fe1204a-… since=5                  ← 用 5 重订，不是 0
```

app H（经中继 7814，新二进制）：`18:06:50 mux 已订阅 … since=340`（断线前水位 340）。
app J（经中继 7817，新二进制）：`18:12:49 mux 已订阅 … since=354`。

**没有整页重载**：app J 的中继把每条请求都记下来了，重订那一段只有一条 `GET /ws`
（见 §4.4 的请求轨迹），没有 `POST /api/session.load`。

### 3.2 断号（gap）—— **真宿主上会断，但不是宿主 seq 域断**

先说清楚两件不同的事：

- **宿主的 seq 域是连续的**：`journal.Append` 在锁里 `seq++`（`internal/transport/journal.go`），
  我抽到的每页（1..100、167..277）都严格递增、无空洞。宿主本来也不发 gap 帧（契约如此）。
- **客户端看到的"断号"是投递层的**：订阅（重订）时宿主**不重放 journal**。
  `wssession.go:111` 的 `case FrameSub` 只做三件事：核世代（要求客户端带 `journalEpoch`，见 §4.3）、
  重放挂起的审批、`hub.Subscribe(topic, sinceSeq, sub)`。`journal.Since()` **只在 `session.history` 里被调用**。
  于是带水位 N 重订之后，客户端收到的第一帧是 N+k，中间 N+1..N+k-1 从来没到过 → `readLoop` 报 gap。

实测（app A）：

```
17:58:36 mux 断号 session:8fe1204a-… [6,15]，带水位重订（不整页刷新）   ← 正是断线期间那一轮
17:58:36 mux 连接断开 session:8fe1204a-…，带水位 5 重连
17:58:37 mux 断号 session:8fe1204a-… [6,24]，带水位重订（不整页刷新）
17:58:40 mux 断号 session:… [6,27] … 17:59:43 [6,44] … 17:59:47 [6,47]
```

共 7 次 gap，`from` 永远是 5+1（水位从没前进过）。旧二进制的 `break` 让每次 gap 都新开一条 WS，
旧连接一个都没关：**连接数从 4 涨到 8**（`netstat` 按界面 PID 数 :7812 的连接，4 → 8，正好 +4 = 新增 gap 数）。

新二进制（app H）改成只记录：

```
18:06:59 mux 断号 session:8fe1204a-… [341,408]（待用 session.history 补洞；不整页刷新）
```

后面没有再断、也没有重订——**缺的 341..408 就此永久跳过**（除非下一轮 follow 把整份历史重放一遍，见 §5.1）。

另一个细节：**`last == 0` 时不检测断号**（`mux.go` 的 `if last != 0 && f.Seq > last+1`）。
所以"打开一个 journal 已有内容的会话"（`session.load` 之后 `since=0` 订阅）会**静默地**从半截开始收：
app D 打开时会话在宿主 journal 里已经到 47，`since=0` 订阅，下一轮第一帧 48 直接当连续收下，不报 gap。

单测这一层是绿的（它不是宿主行为，是客户端归约）：

```
$ go test ./internal/apiclient/ -run 'MultiSession|Reconnect' -count=1 -v
--- PASS: TestMultiSessionNoCrossTalk
--- PASS: TestMultiSessionSwitchAwayAndBackKeepsWatermark
--- PASS: TestMultiSessionSeqGapIsReported
--- PASS: TestMultiSessionDuplicateSeqNotReplayed
--- PASS: TestMultiSessionReplayFromStartNotRedelivered
--- PASS: TestReconnectResumesFromWatermarkAfterDrop
--- PASS: TestReconnectCtxCancelNoLeak
--- PASS: TestReconnectDropClosesChannelNoLeak
--- PASS: TestReconnectErrorFrameNotContent
--- PASS: TestReconnectErrorFrameKeepsWatermark
ok  github.com/MYCF711/dsh-ui/internal/apiclient  1.961s
```

### 3.3 断线期间的事件能否补回 —— **最关键的一条：没补回来**

按任务要求原样做（app E，旧二进制，直连 7811）：

1. 18:01:40 订阅成功（`since=0`，该宿主进程里这是新 journal）。
2. 18:01:54 用 curl 发 prompt（`请写 6 个小节…末尾 END-6`）。
3. 18:01:57（prompt 被接受后约 3s）`taskkill /F /PID 19284` 杀掉宿主。
4. 18:01:58 界面：`mux 连接断开 session:…，带水位 27 重连` → 1/2/4/8/16s 退避 → 18:02:29、18:03:01 **401**。
5. 重启后检查盘上数据：**完整的**。

```jsonc
// session.load（重启后的新宿主，tmp/rc-load2.txt）
total 12 msgs 12
27 user      '请写 6 个小节，每节 3 句…'
28 assistant '**S1- 帧与序号** … 令、退出码、断言与未验项。\n\nEND-6'   ← 完整，没丢
```

也就是说：**那个被杀的 turn 的 run 子进程活过了父进程，把整条回答写进了 v4 日志。**
时间戳能对上，不是猜的：

```
18:01:54.505  dshd-2026-11-03.log「会话开始」（子进程 = dshd-serve.exe run）
18:01:57      父进程（serve）被 taskkill
18:02:07.330  session.load 里 seq=28 那条 assistant/message 的 time   ← 父进程死后 10.3s 才落盘
```

（副作用：杀宿主**不会**取消正在跑的 turn——子进程还会继续烧模型调用。这是另一条产品语义问题，不在本次范围。）

**但界面拿不到它**，原因有四层，逐层都对得上：

1. **401**：重启后旧 cookie 作废 → 连 WS 握手都过不去（§3.1）。这是硬重启时的第一道墙。
2. **重订不重放**：就算凭据有效（中继场景），`FrameSub` 也不从 journal 补 N+1..last（§3.2）。
3. **客户端不调 history**：`runSubscription`（`cmd/app/main.go:402`）只有 `Subscribe` 和 `applyEvent`；
   全仓（非测试）`grep -n "history" internal/apiclient/ cmd/app/` **只有注释，没有调用点**。
   请求轨迹也证实了（§4.4）：整场实验里界面发出的是 `session.list` / `session.load` / `GET /ws`，
   **`POST /api/session.history` 0 次**。
4. **世代协商缺位**：客户端的 sub 帧 `clientFrame{T, Topic, SinceSeq}`（`mux.go:99`）**不带 `journalEpoch`**，
   而宿主只在 `frame.JournalEpoch != 0` 时才核世代、才发 `session/reset`（`wssession.go:115`）。
   `LoadResult`（`client.go:82`）里也没有 `journalEpoch`/`lastSeq` 字段。→ 宿主那条 reset 分支对界面永远不触发。

**如果界面真的调了 history，会拿到什么？** 两个场景都实测了：

```text
# 宿主还活着（水位 166，断线期间跑了一轮）
session.history {"sinceSeq":166,"limit":100}
→ HTTP 200 {"frames":[167..277], "lastSeq":277, "journalEpoch":1}     ← 能补

# 宿主重启过（水位 354，新进程 journal 是空的）
session.history {"sinceSeq":354,"limit":50}
→ HTTP 409 {"error":{"code":"stale_epoch",
             "hint":"丢弃旧水位，重新 session.load 拿 journalEpoch/lastSeq"},
             "journalEpoch":1,"lastSeq":0}                          ← 补不了，只能重建基线
session.history {"sinceSeq":0,"limit":50}
→ HTTP 200 {"frames":null,"lastSeq":0,"journalEpoch":1}               ← 新 journal 里什么都没有
```

所以第 3 条的准确表述是：

> **数据没丢在盘上，丢在"界面的补洞链路上"。**
> 宿主活着断线 → history 能补（但界面没调）；
> 宿主重启 → history 也只能返回 409（journal 是内存的，重启即空），
> 此时唯一的数据来源是 `session.load`（磁盘投影），而界面只在"打开/切换会话"时调它，
> **重连路径不调**。

---

## 4. 卡在哪一环（代码定位）

### 4.1 凭据是进程内存

`../DSH-GO/internal/transport/server.go`

```go
cookies map[string]struct{}   // :98  已发放的有效会话 cookie
...
s.cookies[value] = struct{}{} // :264 /connect 时种下
```

重启 = 新 token + 空 map。界面侧 `cmd/app/main.go` 的 `-cookie` 只在启动时 `SetCookies` 一次，
`runSubscription` 失败分支只睡/重试，没有重新 `/connect` 的能力。

### 4.2 WS 订阅不重放

`../DSH-GO/internal/transport/wssession.go:111-137`

```go
case FrameSub:
    if hub.journals != nil && frame.JournalEpoch != 0 { ... 世代不符才发 session/reset ... }
    if hub.pendingReplay != nil { ... 只重放挂起的审批 ... }
    hub.Subscribe(frame.Topic, frame.SinceSeq, sub)   // 只登记，不回放
```

`journal.Since()` 的唯一调用点是 `handleSessionHistory`（`server.go:514`）。
`frame.go:48` 的注释写着"可带 sinceSeq 从某个序号起重放"，与实现不符——**这是一处契约/注释与实现的偏差**。

### 4.3 世代协商只在服务端有半边

- 客户端 sub 帧：`internal/apiclient/mux.go:99`（`T`/`Topic`/`SinceSeq`，无 `JournalEpoch`）。
- 客户端 `LoadResult`：`internal/apiclient/client.go:82`（无 `journalEpoch`/`lastSeq`）。
- 宿主 load 其实**给了** `journalEpoch`（实测响应里有；`lastSeq=0` 时被 `omitempty` 省掉），但界面不读。

后果（推演，未实测）：如果凭据问题解决了、客户端带着旧水位 N 订阅一个重启后从 1 重新发号的 journal，
`hub.Publish` 会按 `frame.Seq > sinceSeq` 把 seq ≤ N 的帧全部丢掉，客户端既不报 gap（`last==0` 那类边界另说）
也拿不到 reset，等于静默错位。

### 4.4 客户端从不补洞（实测请求轨迹）

app J：中继 7817，`rc-proxy3.py` 逐条记请求行（含 keep-alive 复用的连接）。整场实验的全部请求：

```
18:12:17 REQ POST /api/session.list HTTP/1.1      ← 启动
18:12:17 REQ POST /api/session.load HTTP/1.1      ← 打开会话
18:12:17 REQ GET /ws HTTP/1.1                     ← 订阅 since=0
--- 18:12:34 剪断中继，18:12:36 断线期间发了一轮 prompt ---
18:12:49 REQ GET /ws HTTP/1.1                     ← 重订 since=354（没有 list/load）
18:12:59 mux 断号 … [355,455]（待用 session.history 补洞；不整页刷新）
--- 之后没有任何请求 ---
```

`session.history` 出现次数：**0**。

---

## 5. 附带发现（不在三问里，但会影响"断线恢复"的判断）

### 5.1 每轮 follow 把整份 v4 日志按新 seq 重发一遍

`../DSH-GO/cmd/dshd/serve_tools.go` 的 `follow` 每轮从 `offset=0, lastSeq=0` 开始读，
把所有 `ev.Seq > lastSeq` 的事件重新 `emit`（journal 重新发号）。实测一页就能看出来：

```
# 第二轮 prompt 之后，since=5 拿到的帧
6 turn/start   7 user/message(P1)   8 assistant/message(P1 的正文)   9 turn/end
10 system/message
11 turn/start  12 user/message(P2)  13 assistant/message(P2)         14 turn/end  15 turn/done
```

→ journal 膨胀得快（3 轮就到 100+ 帧；跑到后面 lastSeq 已 455），界面会把历史重复渲染。
它也解释了为什么 app H 的截图里能看到一堆旧内容：那是**下一轮重放**带出来的，不是补洞补出来的。
**这是目前唯一能让缺口"意外回来"的路径，但它同时制造重复。**

### 5.2 真宿主不是逐字流

`assistant/message` 是整条到（reasoning + text 一起），不是 token 增量。
第一轮的时间戳：`turn/start` 1793699714911 → `assistant/message` 1793699718609（3.7s 后一次到齐）
→ `turn/end`/`turn/done` 2ms 内到。界面的"流式"只在 mockd 的 delta 形状下才真的是流式。

### 5.3 退避封顶是 32s

见 §3.1 末尾。

---

## 6. 证据文件

| 文件 | 内容 |
|---|---|
| `tmp/rc-app-a.log` | app A（中继断线 + 断线期间跑一轮 + 7 次 gap 循环） |
| `tmp/rc-ab.png` | **旧二进制**：宿主盘上有 P1–P4 四轮，界面只剩 P1 |
| `tmp/rc-app-d.log` | app D（真宿主重启 → 401） |
| `tmp/rc-app-e.log` | app E（边跑边杀宿主 → 401） |
| `tmp/rc-load2.txt` | 被杀那一轮的完整落盘（`END-6`） |
| `tmp/rc-load4.txt` | 同上，带每条消息的 `time`：assistant 落盘时间 18:02:07.330（父进程 18:01:57 已死） |
| `tmp/rc-home/logs/dshd-2026-11-03.log` | 子进程「会话开始」18:01:54.505 |
| `tmp/rc-app-h.log` / `tmp/rc-h.png` | **新二进制**：gap 只记录、缺段跳过 |
| `tmp/rc-proxy3c.log` / `tmp/rc-app-j.log` | **请求轨迹**：0 次 `session.history` |
| `tmp/rc-hist2.txt` / `rc-hist4.txt` | `session.history` 200 的两份原始响应 |
| `tmp/rc-serve-7811.{out,log}` | 宿主 ready 行与 stderr |
| `tmp/rc-*.py` | 实验驱动（起宿主 / 调 API / 中继 / 后台进程），只用于复现，不是产品代码 |

复现命令（全部是我实际跑过的）：

```bash
cd "D:/DeepSeek Harness/default-workspace/dsh-ui"
python tmp/rc_serve.py 7811                     # 起宿主，写 tmp/rc-serve-7811.json（pid/url/token）
python tmp/rc_api.py 7811 connect               # /connect 换 cookie → tmp/rc-cookie-7811.txt
python tmp/rc_api.py 7811 session.create '{"cwd":"'$PWD'","title":"rc-S2"}'
python tmp/rc_runbg.py tmp/rc-proxy3c.log python tmp/rc-proxy3.py 7817 7811 tmp/rc-proxy3c.log
python tmp/rc_runbg.py tmp/rc-app-j.log ./tmp/app-real.exe \
  -addr http://127.0.0.1:7817 -cookie dshd_session=$(cat tmp/rc-cookie-7811.txt) \
  -session <sid> -shot tmp/rc-j.png -shot-wait 110 -size 1180x800
```

---

## 7. 进程台账（全部是我 Popen 的，全部已回收）

只用了 `taskkill /F /PID <自己的 pid>`；**没有用过 `taskkill /IM dshd-serve.exe`**，
别人的 `tmp/ms-home`、`tmp/serve-home`、`tmp/ms-run-r2` 进程全程没碰。

| 角色 | PID | 结局 |
|---|---|---|
| serve 7811 | 27704 → 23388 → 19284 → 49636 → 21664 → 50576 | 全部 `taskkill /F /PID` |
| app | 12888(A) 28836(D) 46628(E) 49360(F) 33424(G) 39788(H) 13692(I) 26592(J) | 截完图自行退出或被 kill |
| proxy | 26732 37280 18016 17056 34660 19232 36508 46600 52084 4304 41428 | 全部 kill |

收尾核对：`netstat` 上 7811–7817 无 LISTEN；`Get-CimInstance dshd-serve.exe` 里没有 `--home …rc-home` 的残留。

---

## 8. 未验证 / 限制

- **没有验证"重启后凭据更新了会怎样"**：界面没有注入新 cookie 的运行时入口，
  要做到只能重开界面进程（那是新进程，不是自愈）。
- **没有验证超 4096 帧的 journal 截断**（`Journal.capacity` 默认 4096）：补洞超出容量时客户端应重新 load，
  这条路径没走到。
- **gap 只在客户端视角验证**：宿主 seq 域的连续性我是抽页验证（1..100、167..277）+ 读 `journal.Append`，
  没有逐帧扫完整区间。
- **新二进制的 gap 分支还不完整**：它只打日志，没有后续补洞动作，所以"新二进制下断线恢复"目前是
  **明确不恢复**，而不是"待定"。
- 实验期间源码被外部改动两次，若之后又改，§3.2/§4.4 的行为描述需要按新二进制重跑（脚本可直接复用）。

---

## 9. 建议（按代价从低到高）

1. **客户端在 gap 分支真的调 `session.history`**（只补 `[FromSeq,ToSeq]`，不要整页 load），
   并且**先补洞再继续消费**——现在新二进制是"继续消费"，补回来也只能靠下一轮重放。
2. **客户端订阅时带 `journalEpoch`**（`LoadResult`/`HistoryResult` 里都有），
   让宿主那条 `session/reset` 分支真正生效；否则重启后水位永远对不上。
3. **重连要能换凭据**：壳重启 serve 后把新 token/cookie 递进来（或界面自己 `/connect`）。
   否则"宿主重启自愈"这一条无论客户端逻辑多好都不可能成立。
4. **宿主 `FrameSub` 补上 journal 重放**（`sinceSeq` 之后到 lastSeq 的帧），
   这样"带水位重订"才名副其实；现在的注释与实现不符，容易让人以为已经能用。
5. 顺手：退避上限写成 `< 30s` 之后仍会睡到 32s，建议改成 `min(backoff*2, 30s)` 的写法。
