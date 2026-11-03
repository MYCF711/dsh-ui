# 性能与内存基线（实测，不是估算）

> 2026-11-03 17:23 · 界面 `tmp/perf-app.exe` sha256 `f86ebf0644020a4a…`（git `2f8f728`，未提交改动 84 个文件）
> 宿主 `tmp/dshd-serve.exe` sha256 `df3b205d3f6c7c11…`
> 机器：Windows / 1920×1080 / 96 DPI（100%）；同机还跑着官方 DSH 本体与别的子代理，**没有 CPU 隔离**，所以每档取 3 次中位数、内存取「最后 1 秒中位数」。

一句话：**界面是 1 个进程，空载 56.6 MB，200 条大会话 70.4 MB，切会话峰值 79.0 MB，搜索态 73.4 MB；冷启动首帧 258 ms。** 同一台机器上官方 DSH 现在是 11 个进程 2,616 MB。

---

## 1. 怎么跑

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
cd "D:/DeepSeek Harness/default-workspace/dsh-ui"
bash scripts/perf-ui.sh                       # 全场景，每档 3 次，约 6-7 分钟
```

脚本自己做完这些事，**不需要传任何参数**：

1. `CGO_ENABLED=0 go build -o tmp/perf-app.exe ./cmd/app`（源码变了就重建），并打印界面/宿主的 sha256 与 `git rev-parse`；
2. 每次迭代起一个**全新的 `dshd serve`**：临时 home（`tmp/perf-run/<场景>/<n>/home`）、`--addr 127.0.0.1:0`（OS 分配端口）、一次性 token —— 不复用任何别人的端口或进程；
3. 从真实 home 复制 **4 份「不同 id」的大会话**（≤12 MB/份）到临时目录做只读副本（改名复制无效：`session id` 在 journal 里，改名不会变成新会话）；
4. 从进程外部量：`WorkingSetSize` / `PrivateUsage`（`K32GetProcessMemoryInfo`）、窗口（`EnumWindows` + `GetWindowRect`）、CPU（`GetProcessTimes`）；
5. 收尾只 `TerminateProcess` 自己 `Popen` 出来的句柄；结果留在 `tmp/perf-run/`（`summary.md` 是同一张表）。每轮的会话副本（~22 MB/轮）默认删掉，`PERF_KEEP_HOMES=1` 可留。

可调环境变量：`PERF_RUNS`（默认 3）、`PERF_TREND_RUNS`（5）、`PERF_IDLE`（10 秒）、`PERF_KEYS`（10 次）、`PERF_SESSIONS`（4 份）、`PERF_ONLY=A,B1,…`、`PERF_WORK`。

---

## 2. 实测表

| 场景 | 中位数 | 最小 | 最大 | 采样次数 | 单位 | 口径 |
|---|---:|---:|---:|---:|---|---|
| A 冷启动 → 窗口可见（首帧在 `ShowWindow` 之前已 apply） | 258.0 | 244.8 | 275.8 | 3 | ms | `EnumWindows` 5ms 轮询；首帧 `renderer.apply()` 早于 `ShowWindow`，所以它就是「首帧就绪」 |
| A 冷启动 → 第一张 1196×839 的图落盘 | 977.6 | 961.1 | 991.2 | 3 | ms | `-shot` 自带 ~780ms 固定等待（发现窗口后睡 700ms）+ PNG 编码，**不是纯首帧** |
| A 冷启动 → 窗口尺寸就绪（≥1000×700） | 126.9 | 124.6 | 134.2 | 3 | ms | 同可见口径，只判尺寸 |
| A 抓图那一刻的工作集（进程自报） | 66.9 | 66.8 | 67.1 | 3 | MB | 抓图自己会分配 ~8 MB（4 MB DIB + 4 MB RGBA），比 B1 稳态高是正常的 |
| B1 空 home（0 会话）静置 10s | 56.6 | 56.2 | 58.5 | 3 | MB | `WorkingSetSize` 单进程；最后 1s 中位数 |
| B2 已打开的空会话静置 10s | 57.5 | 57.2 | 57.6 | 3 | MB | 同上；会话由 `session.create` 建，0 条消息 |
| C 大会话 `-tail 200` 静置 10s | 70.4 | 68.1 | 70.8 | 3 | MB | 同上；真实会话只读副本（7.0 MB zstd / 200 条渲染） |
| D1 大会话 + 连续 10 次会话切换（↓↑ 交替）峰值 | 79.0 | 78.9 | 80.8 | 3 | MB | 操作期间 50ms 采样，取最大值 |
| D2 大会话 + 搜索（`-search exit`）峰值 | 73.4 | 72.7 | 74.6 | 3 | MB | 搜索态首屏 + 静置期间最大值 |

**复跑一致性**（同一个二进制、间隔几分钟各跑一遍完整流程）：内存差 ≤ **1.5 MB**、首帧差 ≤ **27 ms**、趋势 A 的极差 0.3 → 0.5 MB。表里是第二遍（就是验收那条命令跑出来的）。

补充（同一轮跑出来的，不是另测）：

- **子进程：0**。每次迭代在就绪时刻用 Toolhelp 拍一次进程树，所有迭代 `children_at_ready` 合计 0 —— **界面是单进程**，所以「整进程树」= 「单进程」，没有 Viva 的 GPU/渲染子进程。
- **PrivateUsage**（私有提交，含未触碰/已换出页）：B1 中位数 **84.1 MB**、C 中位数 **102.2 MB**。它比 `WorkingSetSize` **大**，两个数不能混着报。
- **污染检测：0 次**。每次迭代结束后数一遍临时 home 里的会话数，多于预期就说明有别的客户端接到了这个 serve（本轮出现过一次，脚本现在会报）。
- **废图（160×28）：0 次**。
- **D1 按键命中**：`handled=true` 30 次、`handled=false` 0 次（10 次/轮 × 3 轮）。
- **D1 每次操作的 CPU**：153.1 ms / 次（内核+用户时间；这是**代理指标，不是** invalidate→重绘延迟）。
- **宿主 serve 空载**（顺带量的一次，未做成场景）：11.8 MB 工作集 / 45.0 MB 私有。所以「整链」≈ 界面 + 12 MB：空载 ~68 MB、大会话 ~82 MB。

---

## 3. 口径（看数字之前必须先看这一段）

| 项 | 取值 | 说明 |
|---|---|---|
| 内存计数器 | `WorkingSetSize`（主）+ `PrivateUsage`（附） | 都来自 `K32GetProcessMemoryInfo`，和任务管理器「工作集/提交大小」同源。表里只用 WorkingSetSize |
| 进程范围 | **单进程** | 界面不 spawn 子进程（Viva 是 purego + 进程内 D3D，无 GPU 子进程）；`dshd serve` 是**另一个**进程，按需另算（空载 11.8 MB） |
| 采样 | 50ms 一次（D/T）、100ms 一次（B/C 的静置段） | 「最后 1 秒中位数」= 稳态值；「最大」= 整个观察窗口的最大值 |
| 时间 | 外部 `perf_counter` 从 `CreateProcess` 起算 | 窗口用 `EnumWindows` 5ms 轮询，误差 ≤ 1 个轮询周期（~5ms） |
| 宿主 | 每次迭代新起 `dshd serve`，`--addr 127.0.0.1:0` | 端口由 OS 分配，token 一次性；绝不复用别人的地址/进程 |
| 会话数据 | 真实 home 的**只读副本** | 复制到 `tmp/perf-run/big-template`；真实 `D:/DeepSeek Harness/home` 全程只读（本脚本不写它） |
| 环境噪声 | 无 CPU 隔离 | 同机有官方 DSH + 其它子代理在跑；所以每档 3 次取中位数，并给出最小/最大 |

---

## 4. 每个场景为什么值得测

- **A 冷启动到首帧**：用户按下回车到「看见界面」的全部代价。它决定了「这个应用是不是原生应用」——258 ms 里没有 Chromium 启动、没有 JS 解析。
- **B1 空 home**：**基线基线**。后面所有数都要减它才有意义（56.6 MB 是「一个能连上宿主的窗口」的底价）。
- **B2 空会话**：多打开一个空会话只值 0.9 MB（57.5 vs 56.6） —— 说明「打开会话」本身不贵，贵的是消息。
- **C 大会话 `-tail 200`**：这是虚拟化那条线的验收口径。历史线性规律是 0.46 MB/条（200 条 = 172.9 MB），现在 200 条只比空载高 13.8 MB。
- **D1 连续切会话**：切会话会触发重新 load + 重建文档树，是最容易积累内存的操作；温缓存上限 5 个会话也在这条路径上。
- **D2 会话内搜索**：搜索要过滤 200 条消息并重排整个列表，是「一次操作里最重的重绘」。

---

## 5. 和历史数字对表

| 历史口径 | 历史值 | 本轮 | 结论 |
|---|---:|---:|---|
| 冷态启动后工作集波动 | 66–84 MB | **A 抓图时刻自报 66.9 MB** | 对得上。那条数就是「抓图那一刻」的口径（含 8 MB 抓图缓冲）；纯稳态是 **56.6 MB** |
| 真宿主 + 真模型一轮（含流式） | 68.2 MB | 未复测（要真模型调用） | 但 **C 大会话稳态 70.4 MB** 已经落在同一量级 |
| 带工具调用的一轮 | 75.6 MB | 未复测 | 同上；**D1 峰值 79.0 MB** 是另一个「有真实工作量」的参照 |
| 每条消息 0.46 MB（线性） | 200 条 = 172.9 MB | **C：70.4 MB** | 虚拟化之后不再线性：200 条只比空载多 13.8 MB |

---

## 6. 与官方 DSH 的对照

| | 进程数 | 工作集 | 私有提交 |
|---|---:|---:|---:|
| 官方 DSH（用户给的历史对照） | 9 | 1,603.8 MB | — |
| 官方 DSH（本轮同机实测，当前会话） | **11** | **2,616.2 MB** | 2,181.6 MB |
| 官方 DSH 整棵树（含本会话的工具子进程 bash/node/python/go） | 28 | 3,143.1 MB | — |
| **dsh-ui 界面（本轮）** | **1** | **56.6 – 79.0 MB** | 84.1 – 102.2 MB |
| dsh-ui 界面 + 宿主 serve | 2 | 68 – 91 MB | — |

- 差距 **约 33–46 倍**（拿 2,616 MB 比 57–79 MB）。
- 这个对照**不是完全同面**：官方那 11 个进程里装着 Chromium 多进程 + V8 堆，而且其中的会话进程（单个 973 MB）里是**模型会话上下文和工具运行时**，我们的数字只是界面（宿主 serve 另算 11.8 MB）。
- 能说的确定结论只有一句：**界面这一层，单进程 Go + D3D 比 Electron 少两个数量级**；「整产品」对比要等宿主也按同一口径量。

---

## 7. 趋势：连跑之后涨不涨

### 趋势 A · 跨进程（同一场景连跑 5 次，每次一个独立进程）

| 第 N 次 | 1 | 2 | 3 | 4 | 5 |
|---|---:|---:|---:|---:|---:|
| WorkingSetSize (MB) | 56.5 | 56.5 | 56.7 | 56.2 | 56.5 |

- 首次 → 末次：**+0.0 MB**；后三次极差 **0.5 MB** → **平**。
- 这条量的是「启动开销有没有漂移」（比如上次的残留没回收），**不能**用来判定进程内泄漏。

### 趋势 B · 进程内（一个进程连做 5 组 × 10 次切会话）

| 组（每组 10 次切会话） | 1 | 2 | 3 | 4 | 5 |
|---|---:|---:|---:|---:|---:|
| WorkingSetSize (MB) | 78.1 | 79.7 | 80.6 | 82.3 | 82.1 |

- 第 1 组 → 最后一组：**+4.1 MB**；末两组差 **-0.1 MB**；全部组极差 **4.2 MB**。
- 读法：**第 1 组 78 MB，之后爬到 80–82 MB 就停在 82 附近**（末两组一升一降，极差 0.2）。
  上一次完整复跑是 77.5→80.9（+3.4、末两组 +1.0），两次都落在「抬升 3–4 MB 后走平」。
- 这 3–4 MB 是什么？**待查，不下结论。** 两个都可能：① 4 份大会话被温缓存住（缓存上限 5，本来就该占住）；② 分配器保留（Go 的 `HeapIdle` 不还 OS）。分清需要 `runtime/debug` 的 heap profile 或 `/debug/stats` 的 `heap_bytes/heap_idle/heap_released/sys_bytes` 对照，本轮没有采。
- 50 次按键 `handled=true` **50** 次，所以「每组」确实是 10 次真实会话切换（每次约 153 ms CPU）。

---

## 8. 已知问题：`-shot` 偶尔产出 160×28 的废图

**结论：160×28 是「窗口当时被最小化」的 `GetWindowRect`。** 那条 PNG 的内容是窗口标题栏（图标 + 最小化/最大化/关闭），不是界面。

### 证据链

1. **4 张历史废图字节完全相同**（`tmp/geo2.png`、`tmp/msg-bubble.png`、`tmp/real-list.png`、`tmp/h1-statusbar-1600.png`）：
   `md5 678ab724340f975554a7112cd3955996`，160×28，895 字节。同一张图 → 不是「随机截歪」，是同一个确定状态。
2. **复现 1（抓图等待期间最小化窗口）**：`-shot-wait 5` 期间对窗口调 `ShowWindow(hwnd, SW_MINIMIZE)` →
   产出 **同一个 md5**。
3. **复现 2（按「最小化」启动）**：用 `STARTUPINFO.wShowWindow = SW_SHOWMINNOACTIVE` 启动（= `start /min`、
   `Start-Process -WindowStyle Minimized`、快捷方式属性里选「最小化」）→ 窗口从第一帧起就是 `IsIconic=true`，
   `GetWindowRect` = **160×28**，抓图产出 **同一个 md5**。
4. **机制**（Viva 模块里的 `github.com/vivaui/viva@v0.2.3/internal/platform/windows/window_windows.go`，不是本仓）：
   `OpenWindow` 用 `ShowWindow(hwnd, 1)`（SW_SHOWNORMAL）
   显示窗口 —— Windows 会优先采用启动方 `STARTUPINFO.wShowWindow`；而 `cmd/app/headless_windows.go` 的
   `captureWindow` 只在 `w <= 0 || h <= 0` 时报错，160×28 是合法尺寸，于是照抓、照存 PNG。
   `GetWindowRect` 对最小化窗口返回的正是 160×28（`PrintWindow` 只能画出那条标题栏）。

### 排除掉的猜想（都做了实验）

| 猜想 | 实验 | 结果 |
|---|---|---|
| 并发起多个实例，互相抓错窗口 | 同时起 3 个实例各抓一张 | 3 张都是 1196×839，md5 相同 → **不是并发问题**（`findOwnWindow` 按 PID 过滤，跨进程抓不到） |
| 窗口还没起来（尺寸 0） | `-shot-wait 0` + 5ms 轮询 | 第一次枚举到窗口时就已经是 1196×839 → 创建时就带最终尺寸 |
| 用 `-size` 把窗口开小 | `SetWindowPos(…,160,28)` 强改尺寸 | 被 `WM_GETMINMAXINFO` 钳到 **776×559**（`MinWidth 760/MinHeight 520` + 边框）→ 正常态**不可能**出现 160×28 |
| 隐藏窗口（`SW_HIDE`） | 等待期间 `ShowWindow(hwnd, 0)` | 抓出来仍是 1196×839 → 隐藏不是成因 |
| 窗口在屏幕外被「拉回」 | 抓图前 `SetWindowPos(-6000,-6000)` | 尺寸不变（脚本和 `-shot` 都这么干） |

### 怎么复现（两条，各 10 秒）

```bash
# 0) 先有二进制：跑一次 bash scripts/perf-ui.sh，或 CGO_ENABLED=0 go build -o tmp/perf-app.exe ./cmd/app
# 1) 起一个自己的宿主（临时 home + OS 分配端口），记下 ready 行里的 url/token
./tmp/dshd-serve.exe serve --home tmp/perf-repro/home --addr 127.0.0.1:0
# ready {"url":"http://127.0.0.1:51103","token":"…"}   ← token 一次性：每复现一次要新起一个 serve

# 【复现 A】按「最小化」启动（窗口从第一帧起就是 IsIconic）
pwsh -NoProfile -Command "Start-Process -FilePath tmp\perf-app.exe -ArgumentList '-addr <url> -token <token> -shot tmp/perf-repro/a.png -shot-wait 3' -WindowStyle Minimized"

# 【复现 B】正常启动，在 -shot-wait 期间最小化窗口（存成 .ps1 再跑，免得引号打架）
cat > tmp/perf-repro/min.ps1 <<'PS1'
Add-Type 'using System;using System.Runtime.InteropServices;public class Win{[DllImport("user32.dll")]public static extern bool ShowWindow(IntPtr h,int c);}'
$p = Start-Process -FilePath tmp\perf-app.exe -ArgumentList '-addr <url> -token <token> -shot tmp/perf-repro/b.png -shot-wait 5' -PassThru
Start-Sleep -Milliseconds 1500
[Win]::ShowWindow($p.MainWindowHandle, 6) | Out-Null
$p.WaitForExit()
PS1
pwsh -NoProfile -File tmp/perf-repro/min.ps1

# 两条都会得到：
python -c "import struct,hashlib;d=open('tmp/perf-repro/a.png','rb').read();print(struct.unpack('>II',d[16:24]),hashlib.md5(d).hexdigest())"
# (160, 28) 678ab724340f975554a7112cd3955996      ← 和历史废图逐字节相同
```

### 规避

1. **启动器别要用「最小化」**：`start /min`、`Start-Process -WindowStyle Minimized`、快捷方式的「运行方式：最小化」都不行
   （`-WindowStyle Hidden` 反而没事，抓出来仍是 1196×839）。
2. **截图那几秒别动窗口**：最小化 / `Win+D`（显示桌面会把窗口最小化）/ 远程会话把窗口挂起。
3. **读图时校验尺寸**（最实用）：160×28 / 窄于 1000 宽的一律丢弃重截。`scripts/perf-ui.sh` 的 A 场景已经这么做
   （判 `shot.valid`，不合法自动补一次，并在汇总里报「废图 N 次」）。
4. **根治（要改 `.go`，本轮不许改，留给宿主/壳的 owner）**：`captureWindow` 拿到 hwnd 后判一次 `IsIconic`，
   是则 `ShowWindow(hwnd, SW_RESTORE)` 再抓；抓完再用 `MinWidth/MinHeight`（760×520）卡一道，不达标直接
   `log.Fatal` 退出——「静默存一张 160×28」比「抓图失败」危险得多，它会被当成证据。

### 还没解释清的

- **当时那 4 次到底是被谁最小化的，查不出来**：`-shot` 的日志只记「已保存 + 工作集」，不记窗口状态，
  事后再看无法区分「启动时就是最小化」还是「中途被最小化」。能产生 160×28 的路径只有最小化这一条（上面两个实验），
  但这台机器有虚拟显示适配器（`GameViewer Virtual Display Adapter`），远程会话的接入/断开会不会顺带把窗口最小化，
  **未验证**。要坐实得在 `captureWindow` 里把 `IsIconic` 记进日志（改 `.go`）。

---

## 9. 没测到的（逐条，别当成测过了）

1. **GPU 纹理占用 / 显存** —— 拿不到。Viva 的纹理在进程内 D3D 设备里，没有暴露给外部；脚本量的是进程工作集（含 D3D 的提交，但拆不出「纹理多少 MB」）。
2. **帧率 / 掉帧** —— 拿不到。Viva 没有暴露帧计时（`app.Config` 的 `OnRedraw` 没有被 `cmd/app` 接）；`-shot` 只给单帧。
3. **输入延迟（按键 → 画面变化）** —— 拿不到。需要真实输入注入 + 帧时间戳。
4. **invalidate → 重绘完成的时间** —— **拿不到**（这是任务里点名要的那个数）。没有钩子可以打点；D1 只给了 **153 ms/次 的 CPU 代理**，它既不是延迟也不区分绘制/布局/加载。
5. **纯滚动（长列表滚轮）** —— 拿不到。`cmd/app` 没有滚动开关；外部 `PostMessage(WM_MOUSEWHEEL)` 实测**无效**（前后两张 PrintWindow 的像素差 0%，md5 相同）；真滚动要么真鼠标，要么改壳加开关。
   → 所以 D1 换成了**会话切换**（`-keys` ↓↑ 交替，`handled=true`），这是现在唯一能真的「连续 N 次操作」的现成开关。
6. **真模型一轮 / 带工具一轮的内存** —— 未复测。要真模型调用（magpie），放进基线脚本会把「基线」绑到模型可用性上；要用历史数（68.2 / 75.6 MB）时请标明是历史值。
7. **5 会话切换的行为验收**（切走再切回内容一致）—— 只量了内存峰值，没做内容断言；那是 P0-B 的活。
8. **宿主的整树**（`dshd serve` + 它 spawn 的 `run` 子进程）—— 脚本只量界面；serve 空载 11.8 MB 是顺带量的，没有做成场景。
9. **高 DPI / 窗口缩放下的一切** —— 未测。本轮固定 96 DPI、1180×800 客户区（窗口 1196×839）。
10. **冷启动的分解**（258 ms 里各花多少：进程创建 / 插件装配 / 首帧绘制）—— 未打点，只有总数。

---

## 10. 产物

- 脚本：`scripts/perf-ui.sh`（唯一入口；内嵌 ctypes 测量器，只依赖 Python 3 标准库 + Go 工具链，不需要 pip 包）
- 本轮原始数据：`tmp/perf-run/<场景>/<n>.json` + `<n>/app.log`、`serve.log`（窗口事件、内存采样、按键命中、子进程、两边的日志）
- 同表副本：`tmp/perf-run/summary.md`
- 复现产物：`tmp/perf-probe/minexp-minimize.png`、`tmp/perf-probe/minlaunch.png`、`tmp/perf-repro/min.png`、`tmp/perf-repro/minlaunch2.png`（四张的 md5 都等于历史废图）
