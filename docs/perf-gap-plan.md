# 性能缺口测量设计（`docs/perf.md` §9 的 10 项）

> 基础 rev：`bf7e888`；owner：PERF（UIPLAN §4.1）；**本文件只设计，不含这 10 项的任何新实测数字**。
> 文中的数字分三类，逐条标来源：
> ① **引用**＝`docs/perf.md` 已有的实测值（标明它原来是什么口径）；
> ② **环境事实**＝本轮的只读探针（§1.1，命令可复跑，不是缺口结果）；
> ③ **推断**＝由 ① 或 ② 算出来的、不能当实测用的量（证据 JSON 里一律带 `"inferred": true`）。
>
> 边界：本文件不改 `docs/perf.md`、不改 `docs/progress.md`、不改任何 `.go`。所有"要改 X 文件"都只是**需求**，不是本次改动。
> 所有 `文件:行号` 均是 **rev `bf7e888` 下**的行号；本仓此刻有并行子代理在改 `plugins/`，若引用的行号漂移，以符号名（函数/flag/case 名）为准。

---

## 1. 一页结论

### 1.1 三类分类（每项按「可测的那部分」拆开算，不按整条算）

| # | §9 的项 | 分类 | 能测的那部分 | 拿不到的那部分（原因见各节） |
|---|---|---|---|---|
| 1 | GPU 纹理占用 / 显存 | **待实测**（零改动） | 进程 Dedicated/Local/Shared 显存字节数（WDDM 计数器） | 纹理单项字节数 |
| 2 | 帧率 / 掉帧 | **待实测**（需打点）＋**拿不到** | 每次 Paint 的耗时与间隔、超预算帧数 | 真实 present/上屏时序 |
| 3 | 输入延迟 | **待实测**（需打点）＋**拿不到** | 注入 → 绘制完成（CPU 段，µs 级） | 绘制完成 → 上屏 |
| 4 | invalidate → 重绘完成 | **待实测**（需打点）＋**拿不到** | 到 `Paint` 返回（含合并次数） | 到上屏；现有 153 ms 只是 CPU **代理** |
| 5 | 纯滚动（长列表滚轮） | **拿不到 →（接线后）待实测** | 接线后：滚动帧成本、连续 100 次的内存峰值 | 现状拿不到，根因是**产品缺口**不是工具缺口 |
| 6 | 真模型一轮 / 带工具一轮 | **待实测**（内存零改动） | 四个时刻的内存、整树（与 G8 同轮） | TTFT 需打点；历史值 68.2/75.6 MB 不可当本轮 |
| 7 | 5 会话切换的行为验收 | **待实测**（零改动 PNG / 推荐 `-dump`） | 切走再切回的内容一致性 | 流式期间切走再切回属 P0-B，不在本条 |
| 8 | 宿主的整树 | **待实测**（零改动） | serve + `dshd run` + 工具进程的树形内存 | 内核/DWM/pages 不在内 |
| 9 | 高 DPI / 窗口缩放 | **拿不到**（本机 96 DPI） | 只能给 `-size` 多尺寸代理（**不是 DPI**） | 真实 scale≠1 的一切 |
| 10 | 冷启动分解 | **已测（仅总数）**＋**待实测**（分解需打点） | 258 ms 是引用值；分解要打点 | 窗口创建→present 的内部细分 |

**已实测（本轮可引用的）**：只有三处，且都不是这 10 项本身——
`A 冷启动→可见 258.0 ms`（perf.md §2）、`D1 153.1 ms/次 CPU（代理）`（perf.md §2 补充）、`serve 空载 11.8 MB`（perf.md §2 顺带一次，**不是整树**）。
其余 10 项的准确计数：**零改动就能测 4 项**（G1/G6/G7/G8）、**要打点才能测 4 项**（G2/G3/G4/G10）、**G5 现状拿不到**（产品缺口）但接线后可测、**G9 拿不到**。
另有 5 个"永远拿不到、除非上游 viva 或系统状态变"的细分：G1 纹理单项、G2/G3/G4 的**上屏段**、G8 的内核/DWM 侧（拿不到的共性都是"上游没钩子"，不是"没空测"）。

### 1.2 优先级前 3

| 排序 | 项 | 一句话理由 |
|---|---|---|
| **P1** | **G5 纯滚动** | 它不是"没测到"，是**真机滚轮根本不工作**：`Config.OnScroll` 全仓从未接线，viva 在 `OnScroll == nil` 时直接吞掉 `WM_MOUSEWHEEL`。**1 行接线**即可修好并同时可测，用户能直接感知 |
| **P2** | **G4 invalidate → 重绘完成** | 任务点名要的数；已有 153 ms/次的 CPU 代理，只差打点就能拆成 p50/p95；**同一个 `-trace` 打点器同时解锁 G2/G3/G6-TTFT/G10**，一份改动换五项 |
| **P3** | **G8 宿主整树** | 零代码改动；`docs/perf.md` §6 自己写着「整产品对比要等宿主也按同一口径量」——这就是那句的答案；而且 `dshd serve` 每轮 turn 会起 `dshd run` 子进程（`../DSH-GO/cmd/dshd/serve_tools.go:44-90`），**三层树**，只量界面必然少算 |

第 4 位是 **G1 显存**（同样零改动，但与 G8 共用同一个外部 ctypes 采样器，建议合并成一次实现；它排第 4 是因为纹理拆不出来，只能给总量）。

### 1.3 需要改代码才能测的项

**5 项不改代码就测不了**：G2、G3、G4、G5、G10（其中 G2/G3/G4/G10 共用同一个 `-trace`）。
**另有 2 项只有"更严格的那半边"需要改**：G6 的 TTFT、G7 的结构化断言（零改动版都能先出数）。
全部落在 `cmd/app/main.go`；G1 的交叉验证是可选；上屏段与纹理单项属**上游 viva**，不在本仓权限内。

| 项 | 改哪儿 | 行数估计 |
|---|---|---|
| G5 | `cmd/app/main.go:1090` 附近加 `cfg.OnScroll`（1 行）＋可选 `-scroll` 驱动（~15 行） | 1 / 16 |
| G2、G3、G4、G10 | `cmd/app/main.go` 加 `-trace` 打点器：`main()`(1186)、`kernel.Load`(1298)、`RunCanvas`(1382)、`cfg.Paint`(948)、`cfg.OnKey`(1080)、10 处 `m.inv.Invalidate()`（152/170/242/303/609/679/720/764/825/1090）收成 `m.invalidate(src)`、`applyEvent`(692) 首个增量 | ~120 |
| G6（TTFT） | 同 G2 的 `-trace`（`applyEvent` 里记 `first_text`/`done`），无额外改动 | 0 |
| G7（严格版） | `-dump <path>`：退出前写 `{cur, msgs, lastTextSha256, scrollOffset}` | ~20 |
| G1（可选交叉验证） | 新文件 `cmd/app/vram_windows.go`：purego 绑 DXGI `IDXGIAdapter3::QueryVideoMemoryInfo` | ~100 |
| **上游 viva**（不在本仓权限内） | ① present 时间戳 / `IDXGISwapChain::GetFrameStatistics`（G2/G3/G4 的上屏段）② 纹理资源统计（G1 的纹理单项） | — |

**不改代码就能测的：G1（显存总量）、G6（内存）、G7（PNG 版）、G8。**
**永远拿不到的（除非改系统状态或换显示器）：G9。**

### 1.4 环境事实（本轮只读探针，不是缺口结果）

| 探针 | 命令 | 结果 |
|---|---|---|
| GPU 显存按进程计数器是否存在 | `Get-Counter -ListSet * \| ? CounterSetName -match GPU` | 有 `GPU Process Memory`（4 条路径：`Dedicated/Shared/Local/Non Local Usage`） |
| 实例名格式 | `(Get-Counter '\GPU Process Memory(*)\Dedicated Usage').CounterSamples` | `pid_10688_luid_0x00000000_0x00019e46_phys_0`，值单位**字节** |
| 显示适配器 | `Get-CimInstance Win32_VideoController` | NVIDIA RTX 5070 Ti、AMD Radeon(TM) Graphics、GameViewer Virtual Display Adapter |
| 显示器 | `Get-CimInstance Win32_DesktopMonitor` | 单台，1920×1080（**DPI 未重测**；perf.md §3 记录为 96 DPI） |
| 帧分析工具 | `command -v PresentMon/wpaexporter` | **都没有**；有 `wpr.exe`、`typeperf.exe`、`logman.exe`、`nvidia-smi.exe` |
| 二进制 | `ls tmp/perf-app.exe tmp/dshd-serve.exe` | 两者都在（14.0 MB / 14.6 MB） |

> `nvidia-smi` **不能**用来量 D3D 显存：它的 `--query-compute-apps` 只报 CUDA 上下文，D3D11 纹理不在里面。

---

## 2. 新增口径（在 perf.md §3 之上补的，不替换它）

| 项 | 取值 | 为什么 |
|---|---|---|
| 时钟对齐 | 外部 `time.perf_counter()` 与进程内 trace 的**原始 QPC**（`kernel32!QueryPerformanceCounter`）同源 | CPython 在 Windows 上 `perf_counter` 就是 QPC；Go 的单调时钟也是 QPC。trace 里**同时**写 `qpc` 与 `time.Now().UnixNano()`，外部只认 `qpc`。每轮开跑先校准：外部在 Popen 前后各读一次 QPC，与 trace 首行比，偏差 > 1 ms 则**整轮作废**（不猜） |
| 延迟类统计量 | **p50 / p95 / max**，N ≥ 30 次事件 | 内存可以 3 次取中位数，延迟不行：153 ms 是 10 次操作的均值，掩盖分布 |
| 内存类统计量 | 沿用 perf.md §3：静置段「最后 1 s 中位数」、操作段「窗口最大值」、跨进程 3 次取中位数 | 与已有表可直接对比 |
| 触发源 | trace 的 `src ∈ {key, scroll, search, switch, stream, resize, status, approval, startup}` | 没有触发源就没法回答"这帧是谁引起的"——153 ms 不可用的根本原因 |
| 进程范围 | **单进程**（界面）、**整树**（以我们自己 Popen 的 serve PID 为根，Toolhelp32 递归）、**两棵相加** 三个数分开报 | 现有表只有单进程；§6 的对照缺整树 |
| 「推断」标记 | 任何从别处换算出来的数，证据 JSON 加 `"inferred": true`，汇总表里带 `†` | 掉帧、纹理增量、交换链占用都属于这一类 |
| 静默跳过 | 前置不满足 → 结果写 `{"status":"unavailable","why":"…"}`，**不许当 0 或空** | perf.md §9 的教训就是"没测到"被当成"测过了" |

---

## 3. 逐项设计

### G1 · GPU 纹理占用 / 显存

| 列 | 内容 |
|---|---|
| **指标** | ① 进程 Dedicated 显存（每 adapter 一行 + 合计，字节）② Local / Shared / Non-Local 三项（区分"在 VRAM"与"回退到系统内存"）③ 与同一时刻的工作集并列 ④ 三个差分：`-tail 200` ↔ `-tail 0` 的 Dedicated 差（≈内容纹理增量†）、`-search` 峰值、`-switch` 峰值 |
| **怎么采** | 外部 Python `ctypes` → `pdh.dll`：`PdhOpenQueryW(0,0,&q)` → `PdhAddEnglishCounterW(q, "\\GPU Process Memory(*)\\Dedicated Usage", 0, &c)`（**用 English 版 API**，避免中文 locale 的计数器名）→ `PdhGetFormattedCounterArrayW` 取实例数组 → 过滤实例名前缀 `pid_<PID>_` → 同一 adapter 求和、跨 adapter 相加。同一循环里用 `K32GetProcessMemoryInfo` 采工作集，保证"显存与内存同一时刻"。采样 50 ms（操作段）/100 ms（静置段），与 `scripts/perf-ui.sh:402-406` 一致 |
| **口径** | 单进程（界面）。含 D3D 双缓冲交换链 backbuffer（`renderer_windows.go:197`：`Buffers:2, SwapEffect:3`），不含 DWM 侧的合成副本。静置段取最后 1 s 中位数，峰值取窗口最大值，3 次中位数 |
| **计时边界** | `mux 已订阅`（就绪）→ 静置 10 s → 采样；操作段从第一次操作注入到结束后再 2 s |
| **证据路径** | `tmp/perf-gap/G1/<n>/gpu.jsonl`（每行 `t_ms, instances{...}, dedicated, local, shared, nonlocal`）＋ `<n>/summary.json`；结论回填由 perf.md owner 执行（本设计不改它） |
| **依赖** | **不改代码**。前置：目标机有 `GPU Process Memory` 计数器（本机已确认）；没有则整条记 `unavailable` |
| **拿不到** | **纹理单项字节数**。D3D11 没有 per-resource 运行时统计，viva 也不暴露资源尺寸；拆不开纹理/字形图集/交换链。可做的只有总量 + 两个差分估计（标 †）。要真拆需上游 viva 暴露（`IDXGIAdapter3::QueryVideoMemoryInfo` 也只给进程总量，拆不到资源） |
| **可选交叉验证** | 进程内 DXGI `IDXGIAdapter3::QueryVideoMemoryInfo(node, DXGI_MEMORY_SEGMENT_GROUP_PROCESS)` 读 `LocalUsage/NonLocalUsage`，与 PDH 对表；需新文件 `cmd/app/vram_windows.go`（purego 绑 dxgi，~100 行），**本次不做** |

### G2 · 帧率 / 掉帧

| 列 | 内容 |
|---|---|
| **指标** | 不做"FPS"，做四个可归因的量：① 每帧 `Paint` 耗时（µs）② 相邻帧间隔 `dt` ③ 每个观察窗口的帧数 ④ **超预算帧数**（`dt > 1.5 × 刷新周期` 或 `Paint 耗时 > 刷新周期`）† |
| **怎么采** | 需打点：`cfg.Paint`（`cmd/app/main.go:948`）首尾各取一次 QPC 写 trace（`ev:"paint"`，含 `start/end/dur_us/trigger/coalesced`）。刷新周期由外部 `EnumDisplaySettingsW(ENUM_CURRENT_SETTINGS).dmDisplayFrequency` 取（**不得假设 60 Hz**）。观察窗口四个：W1 流式（真模型 delta 期间，见 G6）、W2 连续按键（现有 `-keys`，`main.go:1367`，0.35 s/次）、W3 缩放突发（外部 `SetWindowPos` 60 Hz × 60 次，触发 `OnResize`→`Invalidate`，`main.go:1090`）、W4 滚动（G5 接线后 16 ms 一次 wheel） |
| **口径** | 单进程。**空闲稳态 = 0 帧**（viva 按需绘制，`app/frame_timer.go` 只在动画中挂表）——报告必须写这句，否则"0 FPS"会被读成故障。每窗口 N ≥ 60 帧，报 p50/p95/max + 超预算计数 |
| **计时边界** | 一帧 = `Paint` 进入 → `Paint` 返回（我们自己画完，含布局、文本 shaping、图元/标签列表构建）。**不含** `painter.publish`、`renderer.apply`、`Present` |
| **证据路径** | `tmp/perf-gap/G2/<n>/trace.jsonl` ＋ `<n>/frames.csv`（`win,frame_idx,qpc_start,qpc_end,dur_us,dt_us,trigger`）＋ `summary.md` |
| **依赖** | 改 `cmd/app/main.go`（`-trace`）。viva 无需改 |
| **拿不到** | **真实帧率/掉帧**。`renderer_windows.go:431` 是 `comCall(renderer.swap, 8, 1, 0)` = `IDXGISwapChain::Present(SyncInterval=1, Flags=0)`，present 是阻塞等 vblank 的**内部调用**，viva 不暴露 present 前后的时间戳、也没有帧统计接口；外部拿不到每帧 present 事件（本机无 PresentMon；`wpr.exe` 在但无 `wpaexporter`，无法自动取数）。要拿到只有两条路：上游 viva 调 `IDXGISwapChain::GetFrameStatistics`，或引入 ETW 消费者。**"超预算帧数"是推断值，必须标 †** |
| 附：perf.md §9-2 的措辞需要更正 | 原文说「`app.Config` 的 `OnRedraw` 没有被 `cmd/app` 接」。实际是 **公开的 `app.Config` 根本没有 `OnRedraw` 字段**：`OnRedraw` 只存在于 `internal/platform/contract.go:87`，由 `app/run_canvas.go:123` 内部接线。所以"去接 OnRedraw"这条路不存在，打点只能落在 `Paint` / `OnKey` / `Invalidator` 这三处（本设计就是这么做的） |

### G3 · 输入延迟（按键 → 画面变化）

| 列 | 内容 |
|---|---|
| **指标** | 三段分开：① **注入 API 返回 → `OnKey` 进入**（含消息队列等待）② **`OnKey` 进入 → `Paint` 返回**（本进程 CPU 段）③ **`Paint` 返回 → 上屏**（拿不到）。报 ①+② 的合计 p50/p95/max，并单独给 ② |
| **怎么采** | 注入两条通道，默认第 1 条：<br>**(a) `PostMessage(hwnd, WM_KEYDOWN, vk, lparam)` + `WM_KEYUP`** —— viva 的 `wmKeyDown` 分支（`window_windows.go:739-743`）**不看焦点/前台**，直接喂 `OnKey`，所以跨进程 PostMessage 是有效通道，且**不抢焦点、不闪窗**。<br>**(b) `SendInput`**（含 OS 输入栈，最接近真按键）——需要目标窗口在前台，会抢焦点、会让窗口闪一下；只在专门一轮用，且**必须先把窗口从 -6000 移回屏幕**。<br>时间戳：外部注入前读一次 QPC；进程内 `cfg.OnKey`（`main.go:1080`）入口取 QPC 写 trace；`Paint` 首尾见 G2。对齐按 §2 |
| **口径** | 单进程。N ≥ 30 次按键（沿用 `-keys` 的键表，但用 `PostMessage` 而不是 `m.kernel.HandleKey` 直调——**现有 `-keys` 是进程内直调 `m.kernel.HandleKey`（`main.go:1367`），绕过 viva 的窗口过程与消息队列，不能代表输入延迟**，只能代表"事件进插件之后有多快"）。每个键记 `handled` |
| **计时边界** | 注入 API 返回（`PostMessage`/`SendInput` 返回）→ `Paint` 返回。**明确不含上屏** |
| **证据路径** | `tmp/perf-gap/G3/<n>/latency.csv`（`chan,key,inject_qpc,onkey_qpc,paint_start,paint_end,handled`）＋ `summary.md` |
| **依赖** | 改 `cmd/app/main.go`（`-trace` 的 `ev:"key"` 与 `ev:"paint"`）。外部注入用现有 ctypes |
| **拿不到** | **上屏段**（同 G2：无 present 时间戳）。另外 `SendInput` 通道本身会污染测量环境（抢焦点、DWM 可能判为"用户在操作"），所以默认不用它 |
| **读法（写进报告，防止过度解读）** | 若 ①② 合计 ≈ ②（差值 < 1 ms），说明延迟**全部来自绘制成本**，没有排队；若 ① 明显大，说明卡在窗口过程之前的消息泵/合成节流。只有前者才可以用"减少绘制工作量"来优化 |

### G4 · invalidate → 重绘完成

| 列 | 内容 |
|---|---|
| **指标** | ① 每次 invalidate 到对应 `Paint` 返回的时间 ② 一次操作内的 **invalidate 次数 / 实际帧数 / 合并掉的次数**（viva 的 Waker 把同一批 wake 合成一次重绘，`app/frame_timer.go`）③ 按 `src` 分组的 p50/p95/max |
| **怎么采** | 需打点：把 `cmd/app/main.go` 里 **10 处** `m.inv.Invalidate()`（152/170/242/303/609/679/720/764/825/1090）换成 `m.invalidate(src)`：先写 `ev:"inval"`（`t_qpc, src, n`）再调 `m.inv.Invalidate()`。`Paint` 首尾写 `ev:"paint"`（`trigger` = 自上次 paint 以来的第一个 `src`；`coalesced` = 这期间被合并的 inval 数）。配对规则：一个 `Paint` 认领它之前所有未被认领的 `inval` |
| **口径** | 单进程。三组各 N ≥ 30：会话切换（沿用 D1 的 ↓↑ 交替节奏）、搜索开关（`-search`）、resize 突发（见 G2-W3）。报 p50/p95/max + 合并率 |
| **计时边界** | `Invalidate()` 被调用那一刻 → 该帧 `Paint` 返回。**这是"到绘制完成"，不是"到上屏"**；必须在结论里逐字标注，否则会重演"153 ms 被当成延迟"的错误 |
| **证据路径** | `tmp/perf-gap/G4/<n>/inval.jsonl`（`inval` 与 `paint` 两种行）＋ `<n>/summary.md`（分组表） |
| **依赖** | 改 `cmd/app/main.go`（`-trace`，10 处换包装，机械替换） |
| **拿不到** | 到上屏的那一段（同 G2）。**现有 153.1 ms/次是 CPU 时间代理**（`GetProcessTimes` 的 kernel+user，10 次均值），它既不含排队也不区分绘制/布局/加载——本项测完后，§2 那行必须标成"已被 G4 取代"或"与 G4 不可比" |
| **为什么这条值得先做** | 它是唯一能回答"切会话为什么慢"的量：把 153 ms 拆成 `invalidate→paint` + `paint 内部`，才知道是该减布局还是减加载 |

### G5 · 纯滚动（长列表滚轮）

| 列 | 内容 |
|---|---|
| **现状（根因）** | **不是工具缺口，是产品缺口**：`Config.OnScroll` 在 dsh-ui 全仓**从未接线**（`grep -rn "OnScroll" --include=*.go` 0 命中），而 viva 的窗口过程是 `case wmWheel: if state.config.OnScroll != nil { … }`（`window_windows.go:745-750`）——**`OnScroll == nil` 时消息被直接吞掉**，连默认的 canvas 偏移都不会有。消息区的滚动状态由 `flex.ScrollArea` 的 `ScrollState` 管（`plugins/dsconversation/conversation.go:214-217`，`.Follow(true)`），只有 `ScrollState.Wheel`（`flex/scroll.go:119`）或 `View.Wheel`（`flex/scroll.go:142`）能推动它；全仓只有两处拿 `ScrollState`（`main.go:351/373`，save/restore），**没有一次 `Wheel` 调用**。<br>→ 这解释了 perf.md §9-5 的观察（PostMessage 前后像素完全相同），也意味着**真机上鼠标滚轮滚不动消息列表**（只能键盘 ↑↓ 或等自动跟随）。这一条应当作为**缺陷**上报，不只是性能缺口 |
| **指标** | ① 每次滚动是否真的位移（`ScrollState.Offset` 变化量）② 该帧 `Paint` 耗时 ③ 连续 N=100 次滚动的帧间隔分布 ④ 内存峰值 ⑤ 滚动前后两张抓图必须**不同**（否则判"没滚"） |
| **怎么采** | 两步：<br>**① 接线（产品修复，1 行）**：在 `newConfig` 里加 `cfg.OnScroll = func(ev ui.ScrollEvent) ui.EventResult { return m.view.Wheel(ev) }`（`m.view` 已是 `flex.View`，插件侧 `ScrollArea` 已具备状态）。用 `View.Wheel` 而不是 `ScrollState.Wheel`，是因为前者按指针位置找最内层可滚区域，语义与浏览器一致。<br>**② 注入（两条，建议都做）**：<br>(a) 外部 `PostMessage(hwnd, WM_MOUSEWHEEL, MAKEWPARAM(0, -120), MAKELPARAM(sx, sy))`——lParam 是**屏幕坐标**（viva 会 `screenToClient`，`wheel_windows.go:39-58`）；窗口被 `hide_offscreen` 挪到 -6000 时，`sx = rect.left + x*scale`。<br>(b) 调试开关 `-scroll N`：进程内每 16 ms 调一次 `m.view.ScrollState(tree.ID(home,"msgs-"+m.cur)).Wheel(ui.ScrollEvent{DeltaY:-24, Precise:true})` 后 `m.invalidate("scroll")`——确定性最好，不依赖窗口位置与消息泵 |
| **口径** | 单进程。N = 100 次滚动；N 次期间 50 ms 采内存，帧级 trace（G2 的 `ev:"paint"`，`trigger:"scroll"`）。滚到底（`Offset >= Max()-0.5`）后的请求记为 `noop`，不算性能事件 |
| **计时边界** | 注入返回 → 下一帧 `Paint` 返回（不含上屏）；连续测试的总窗口从第 1 次注入到第 100 次后的静置 2 s |
| **证据路径** | `tmp/perf-gap/G5/<n>/scroll.jsonl` ＋ `shot-before.png` / `shot-after.png`（md5 必须不同）＋ `summary.json` |
| **依赖** | 改 `cmd/app/main.go`：**1 行**接线（产品修复）＋可选 ~15 行 `-scroll`。外部注入零改动 |
| **拿不到（接线前）** | 现状下任何滚动量都拿不到，且**外部 PostMessage 永远无效**——不是姿势不对，是没有消费者。这一点必须写进 perf.md §9-5 的更正里 |
| **与 D1 的关系** | perf.md 把 D1 从"滚动"换成"会话切换"，理由是"没有滚动开关"。接线后 D1 可以回到滚动（更接近用户感知），会话切换另立一档 |

### G6 · 真模型一轮 / 带工具一轮

| 列 | 内容 |
|---|---|
| **指标** | 界面进程的内存四时刻：① 发送前静置 10 s ② 首字节到达 ③ 流式峰值 ④ turn 结束后静置 10 s。可选：TTFT（发出 → 首个正文/思考增量写入 `m.stream`）与 turn 总时长。整树同轮由 G8 采 |
| **怎么采** | ① 复制**真实 home**到临时目录（`settings.yaml` 的 provider 指向本机 magpie 网关，**不外发、不写原 home**；会话可用 `scripts/perf-ui.sh:500-552` 的只读副本逻辑）② `tmp/dshd-serve.exe serve --home <copy> --addr 127.0.0.1:0`（OS 分配端口 + 一次性 token）③ 界面 `-addr {URL} -token {TOKEN} -session $SID -tail 200 -smoke -prompt "<固定短句>"`（`main.go:1192-1193`；`-smoke` 在首个会话载入后自动发，`main.go:298-309`）④ 外部 50 ms 采样（现有 helper 已具备）。<br>**带工具一轮**：prompt 明确要求一次无副作用工具（如 `bash -c "echo hi"`）；策略为 ask 时，等审批帧出现后用现有 `-keys enter` 按一次（`parseKeys` 里 Enter 在审批条可见时 = 允许，`main.go` 的 `parseKeys` 注释） |
| **口径** | 单进程（界面）为主，整树见 G8。静置段取最后 1 s 中位数，峰值取整个 turn 窗口最大值，3 次独立进程取中位数 + 最小/最大。**必须与历史值（68.2 / 75.6 MB）分行列出，不合并、不平均** |
| **计时边界** | ① 从 `-smoke` 发出（`sendText` 返回，`main.go:810`）到流式结束帧（`m.turnBusy=false`，`applyEvent` 内 `m.stream.Done()` 分支，`main.go:754-760`）② 内存窗口另加结束后 10 s 静置 |
| **证据路径** | `tmp/perf-gap/G6/<n>/{mem.jsonl, app.log, serve.log, meta.json}`；`meta.json` 记 `model`、`prompt_sha256`、`policy`、`dshd_sha`、`app_sha` |
| **依赖** | 内存部分**不改代码**；TTFT 需 G2 的 `-trace`。前置：magpie 可用（`curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:3425/v1/models` = 200）。**默认不进基线脚本**：`PERF_GAP_REAL=1` 才跑，且需用户当次授权（消耗真实额度、依赖模型可用性） |
| **拿不到 / 风险** | 模型输出不可复现 → 只报内存与时长，不报 token 级指标的可比性；prompt 固定 + 记录 sha256，但不承诺输出长度一致（这也是它不进基线的第二个理由） |

### G7 · 5 会话切换的行为验收（切走再切回内容一致）

| 列 | 内容 |
|---|---|
| **指标** | 不是内存，是断言：① 回到的会话 id ② 消息条数 ③ 最后一条正文的 sha256 ④ 滚动偏移（`m.scrollPos`/`ScrollState.Offset`）⑤ 无串流（B 的正文不出现在 A 的消息里）⑥ 无丢字（正文长度与直接打开时相同） |
| **怎么采** | 三条路，按代价排：<br>**① 零改动（先做）**：参考轮 `-session $BIG -tail 200 -shot ref.png`；切换轮 `-session $BIG -tail 200 -switch s1,s2,s3,s4,s5,$BIG -switch-every 1 -shot switched.png`（`-switch` 已有：`main.go:1347-1360`，2 s 起、每 `-switch-every` 秒一个）。判据：两图尺寸相同 + **只看消息区矩形**的逐像素差为 0（`enum` 可用 PIL 12.3；无 PIL 时退化为 md5，但整窗 md5 会被状态栏文字/光标闪烁打断）。<br>**② `-dump <path>`（推荐，~20 行）**：退出前写 `{cur, msgs, lastTextSha256, scrollOffset, streamText}`，判据是结构化等值，不受渲染细节干扰。<br>**③ `-driver`（最强，需新模式）**：`app.NewDriver(newConfig(m), &canvas)` 无窗口回放，`Snapshot.Texts` 直接比对，毫秒级、可重复 N 次（见 G10 的交叉项） |
| **口径** | 单进程。每次切换后固定 1 s 再切下一个；整轮重复 3 次。参考轮与切换轮必须**同一二进制、同一 home 副本** |
| **计时边界** | `openSession(id)`（`main.go:186`）→ 该会话正文绘制完成（`mux 已订阅` + 首帧 `Paint` 返回） |
| **证据路径** | `tmp/perf-gap/G7/<n>/{ref.png, switched.png, dump.json, meta.json}` |
| **依赖** | ① 零改动 ② 改 `cmd/app/main.go` ③ 需新模式（或新 `cmd/perfdrv`） |
| **拿不到 / 边界** | 「流式进行中切走再切回」的内容一致性**不在本条**——那是 P0-B 的真宿主验收（`AGENTS.md` P0-B），这里只做"切走再切回一致"的最小断言，不替代它 |

### G8 · 宿主的整树（`dshd serve` + `dshd run` + 工具进程）

| 列 | 内容 |
|---|---|
| **指标** | ① 整树 WorkingSetSize/PrivateUsage 之和 ② 每个 PID 一行（名字、深度、父 PID、ws、priv）③ 进程数 ④ 树形变化事件（`run` 何时起、何时退）。四个时刻：serve ready 空载 / 会话已载入 / 一轮进行中（子进程在）/ turn 结束后 |
| **怎么采** | 外部：`CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS)` **递归**收集（现有 `scripts/perf-ui.sh:176-189` 只收一层，改成 BFS 收全树），每个 PID `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION|PROCESS_VM_READ)` + `K32GetProcessMemoryInfo` + `QueryFullProcessImageNameW`。采样 100 ms；同时记 PID 集合的差分（用于定位 `dshd run` 的起止）。**归属校验：只从我们自己 `Popen` 出来的 serve PID 往下走**，不按镜像名扫（`AGENTS.md` §4 明令） |
| **口径** | **整进程树**（serve 为根，递归 children）。与界面那棵分开报，再给「界面 + 整树」的和。不含 DWM/内核缓冲/pagefile 回退 |
| **计时边界** | serve `Popen` → stdout `ready {...}`；一轮：prompt 发出 → `dshd run` 退出（超时上限 120 s）；结束后再等 3 s 确认无残留 |
| **证据路径** | `tmp/perf-gap/G8/<n>/tree.jsonl`（每行一次采样：`t_ms` + `[{pid,name,depth,ws,priv}]`）＋ `<n>/tree-summary.md` |
| **依赖** | **不改代码**。要与 G6 同轮跑（一次真模型 turn 同时出 G6 与 G8 的数，只花一次额度）；`serve` 每轮起 `dshd run` 的证据在 `../DSH-GO/cmd/dshd/serve_tools.go:44-90` |
| **为什么重要** | perf.md §6 的 `56.6–79.0 MB` 只是界面；`11.8 MB` 只是 serve 空载**一次**。真正的"整产品"是 serve + run + tools 三层树。§6 那句「整产品对比要等宿主也按同一口径量」就是这一条 |
| **拿不到** | 内核/DWM/驱动侧占用（不在进程计数器里）；`dshd run` 内部的 goroutine/堆细节（那要宿主自己的 `/debug/stats`，`../DSH-GO/internal/transport/server.go:184,219`，只报宿主自己，不能替代界面或整树） |

### G9 · 高 DPI / 窗口缩放

| 列 | 内容 |
|---|---|
| **结论** | **拿不到（本机）** |
| **为什么** | viva 在窗口创建时调 `SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)`（`internal/platform/windows/window_windows.go:197-198`，`setDPI.Call(^uintptr(3))`），窗口的 scale 完全来自 `GetDpiForWindow`（同文件 291-293）。本机只有一台 1920×1080 显示器、perf.md §3 记录为 96 DPI（100%）。要让 scale ≠ 1 只有三条路：<br>① **改 Windows 显示缩放** —— 影响用户所有窗口，属系统状态；<br>② **换/加一台 DPI ≠ 96 的显示器**（本机有 GameViewer 虚拟显示适配器，但未验证它能给出不同有效 DPI）；<br>③ **改 viva 暴露 `Config.Scale`** —— 不是本仓权限。 |
| **能给的替代（明确标注"不是 DPI"）** | (a) `-size 1475x1000`（=125%）、`-size 1770x1200`（=150%）跑 A/B1/C/D1/D2，量几何与内存随像素数怎么长（这是**布局代理**，字体不缩放）；(b) 最小尺寸边界 `-size 776x559`（`WM_GETMINMAXINFO` 的钳制值，perf.md §8）；(c) 字体/行高令牌核对 `docs/official-specs.md`。 |
| **指标** | **本机无此项**（`unavailable`）。有 DPI ≠ 96 的环境时：同 A/B1/C/D1/D2，但每档记 `GetDpiForMonitor(MDT_EFFECTIVE_DPI)` 的 effective DPI 与该窗口的 `GetDpiForWindow`；另加"125% 下文字是否重排/是否被裁"的几何断言 |
| **怎么采** | **本机拿不到**，采样方法只在有 DPI 环境时才执行：不改任何东西，同一脚本换到 125%/150% 缩放的机器（或用户授权的临时缩放）。脚本第一步先打印 `GetDpiForMonitor`，**96 就直接记 `unavailable` 并退出**，不许把 96 DPI 的跑法当成"高 DPI 已测" |
| **口径** | 单进程；3 次中位数 |
| **计时边界** | 同 A（`CreateProcess` → 首帧可见）与 B/C 静置段 |
| **证据路径** | `tmp/perf-gap/G9/<n>/{dpi.json, size-125.json, size-150.json, shot-*.png}` |
| **依赖** | 不改代码；**需外部条件**（另一台机器 / 用户授权改系统缩放 + 预置还原脚本）。默认不做 |
| **禁止** | 不得为了"有数"而把 `-size` 的结果写进"高 DPI"行——这正是 §9-9 要避免的那类混报 |

### G10 · 冷启动的分解（258 ms 里各花多少）

| 列 | 内容 |
|---|---|
| **指标** | 六个时刻（同一个 QPC 时钟）：`t0` CreateProcess（外部 `Popen` 前）、`t1` `main()` 首行、`t2` `kernel.Load` 返回（`main.go:1298`）、`t3` `app.RunCanvas` 调用前（`main.go:1382`）、`t4` `Paint#1` 起/止、`t5` 窗口可见（外部 `EnumWindows` 5 ms 轮询）、`t6` 首张合法抓图落盘。<br>派生五段：**loader**（t0→t1）、**装配**（t1→t2）、**连接/首会话**（t2→t3，含 `/connect`、`session.list`）、**窗口创建**（t3→t4）、**首帧绘制**（t4 内部）；`t4→t5` 是 present+DWM 的混合段，**不可再细分**；`t5→t6` 含 `-shot` 自身固定等待（`headless_windows.go:89` 有 `Sleep(700ms)`，另有 300/80 ms 两处），**算启动时间时必须扣掉** |
| **怎么采** | 需打点（`-trace`）：`ev:"start"`（t1）、`ev:"kernel_loaded"`（t2，带插件数与顺序）、`ev:"run_canvas"`（t3）、`ev:"paint"` 第一帧（t4）。t0/t5/t6 外部。进程创建时刻也可用 `GetProcessTimes` 的 `CreationTime`（FILETIME）交叉核对，但**外部只认 QPC**（§2 校准） |
| **口径** | 单进程；3 次中位数 + 最小/最大（沿用 A 场景口径）；**必须扣掉 `-shot` 的固定等待**，否则会把 700 ms 算成启动 |
| **计时边界** | t0（CreateProcess）→ t5（`IsWindowVisible` 且尺寸 ≥ 1000×700，沿用 `scripts/perf-ui.sh:378-380`） |
| **证据路径** | `tmp/perf-gap/G10/<n>/{startup.json, trace.jsonl, shot.png}` |
| **依赖** | 改 `cmd/app/main.go`（`-trace`） |
| **拿不到** | 窗口创建 → Present 的内部细分（viva 不暴露 `OpenWindow`/`ShowWindow` 时刻）；`dshd` 侧启动不在内（要另算 G8 的 `serve_ready_ms`，现有 helper 已记） |
| **零改动的交叉项** | 用 `app.NewDriver`（`viva/app/driver.go:90`，无窗口、同步绘制、虚拟时钟）跑同一套插件：得到**纯 CPU 的「装配 + 首帧绘制」**，与 `t1→t4` 对表；两者之差大致是窗口创建 + 首帧 present（**推断，标 †**） |

---

## 4. 共用打点器 `-trace` 的需求（G2/G3/G4/G6/G10 用它）

**归属**：`cmd/app/main.go` 的 owner（本设计不改）。**约定**：JSONL，一行一事件，窗口线程只做"取时间戳 + 写定长槽"，落盘由独立 goroutine 批量写；缓冲满则丢弃并累加 `dropped`（最终写进 `ev:"exit"`），**绝不在 `Paint` 里做 I/O**。

| 事件 | 字段 | 打在哪 |
|---|---|---|
| `start` | `qpc`, `unix_ns`, `pid`, `exe`, `sha` | `main()` 第一行（`main.go:1186` 之后） |
| `kernel_loaded` | `qpc`, `plugins[]`, `n` | `main.go:1298` 之后 |
| `run_canvas` | `qpc`, `w`, `h`, `min_w`, `min_h` | `main.go:1382` 之前 |
| `inval` | `qpc`, `src`, `seq` | `m.invalidate(src)`（10 处换包装） |
| `key` | `qpc`, `key`, `handled`, `seq` | `cfg.OnKey`（`main.go:1080`） |
| `paint` | `qpc_start`, `qpc_end`, `dur_us`, `trigger`, `coalesced`, `msgs`, `offset` | `cfg.Paint` 首尾（`main.go:948`） |
| `stream` | `qpc`, `phase ∈ {first_think, first_text, done}`, `chars` | `applyEvent`（`main.go:692`，`m.stream.Apply` 之后） |
| `heap` | 每 500 ms：`/memory/classes/heap/objects:bytes`、`heap/unused`、`heap/released`、`total:bytes` | 独立 goroutine，`runtime/metrics` |
| `exit` | `qpc`, `dropped`, `paints` | 退出前 |

> `heap` 这一档顺带回答 perf.md §7 的"+3~4 MB 是温缓存还是分配器保留"——那一节现在只写了"待查，不下结论"。**它不是那 10 项之一，不在本次交付范围**，只是同一个打点器顺带能给的。

---

## 5. 脚本 `scripts/perf-gap.sh` 的接口设计（**本次不实现**）

| 项 | 设计 |
|---|---|
| 入口 | `bash scripts/perf-gap.sh`（默认跑不依赖真模型的场景：G1、G2-W2/W3、G4、G5、G7、G8-结构、G10） |
| 场景开关 | `PERF_GAP_ONLY=G1,G8`；`PERF_GAP_RUNS`（默认 3）；`PERF_GAP_EVENTS`（延迟类事件数，默认 30） |
| 前置校验（缺则记 `unavailable`，不静默跳过） | `-trace` 文件有 `start` 行才能跑 G2/G3/G4/G10；`OnScroll` 已接线才能跑 G5；`GPU Process Memory` 计数器存在才能跑 G1；`/v1/models` = 200 且 `PERF_GAP_REAL=1` 才能跑 G6/G8-真轮 |
| 复用 | 与 `scripts/perf-ui.sh` 共用同一堆 ctypes 样板（内存/窗口/子进程/PNG 尺寸），但**不修改** `perf-ui.sh`（避免与它的 owner 撞文件）；接受一份复制，或由 perf-ui 的 owner 后续把 helper 抽成 `scripts/perf_helper.py` |
| 产物 | `tmp/perf-gap/<G?>/<n>/…`；汇总 `tmp/perf-gap/summary.md`（每行带 `status ∈ {measured, unavailable, inferred}` 与 `why`） |
| 收尾 | 只 `TerminateProcess` 自己 `Popen` 的句柄；不按镜像名杀进程；临时 home 默认删除（`PERF_GAP_KEEP_HOMES=1` 保留） |

---

## 6. 优先级表

| 排名 | 项 | 用户能直接感知 | 已有数据只差一步 | 解锁面 | 代价 |
|---|---|---|---|---|---|
| **1** | **G5 滚动** | ★★★ 滚轮现在**完全没反应** | — | 解锁 G2-W4、让 D1 回到"滚动"这个真实负载 | 1 行接线（+15 行可选） |
| **2** | **G4 invalidate→重绘** | ★★★ 切会话/搜索/流式的卡顿 | ★★ 已有 153 ms/次 CPU 代理，差打点 | **一个 `-trace` 同时解锁 G2/G3/G6-TTFT/G10** | ~120 行打点 |
| **3** | **G8 宿主整树** | ★★ "整产品占多少"是对外主张的根 | ★★ perf.md §6 自己写了"要等宿主按同一口径量" | 补上"界面≠整产品"这个缺口 | 零改动，半天可实现 |
| 4 | G1 显存 | ★★ "极限占用"的另一半 | — | 与 G8 共用同一个采样器 | 零改动；但纹理拆不出 → 只给总量 |
| 5 | G10 冷启动分解 | ★★ 258 ms 是谁花的 | ★ 总数已测，只差分段 | 顺带（同一打点器） | 依赖 G4 的 `-trace` |
| 6 | G2 帧率 | ★★ 但空闲=0 帧，"帧率"本身语义弱 | — | 依赖打点；上屏拿不到 | 只能给帧成本 + 超预算推断 |
| 7 | G3 输入延迟 | ★★★ 但用户对 10 ms 级差异感知弱 | — | 上屏段拿不到，只能给 CPU 段 | 打点 + 注入通道 |
| 8 | G6 真模型一轮 | ★★ 但**不能进基线**（绑模型可用性） | ★ 历史值 68.2/75.6 在 | 与 G8 同轮，一次额度 | 需授权 + 网络 |
| 9 | G7 5 会话一致性 | ★ 行为验收，不是性能 | — | P0-B 的前置小断言 | 零改动版已够用 |
| 10 | G9 高 DPI | ★★ 但**根本拿不到** | — | — | 需外部条件 |

**为什么 G3 不进前三**：它测出来的数只能到"绘制完成"，而上屏段拿不到；用户对"按键到画面"的实际感知主要由 present/DWM 决定，缺了那一段，这个数解释力有限。它的正确做法是等 G2/G4 的 `-trace` 落地后**顺带**做（同一批打点），不需要独立立项。

**为什么 G1 排第 4 而不是前 3**：它零改动、便宜，但只能给"进程显存总量"，拆不出纹理——而"纹理占用"才是任务里那句的原话。它更适合与 G8 合并成"外部采样器一次实现"。

---

## 7. 不得混报的清单（照 perf.md 的教训写死）

| 不许 | 正确的写法 |
|---|---|
| 把 `D1 153.1 ms/次 CPU` 当延迟 | 它是 `GetProcessTimes` 的 kernel+user 除以 10，**是代理**；G4 出数后应标"已被 G4 取代" |
| 把 `WorkingSetSize` 当显存 | 显存看 `\GPU Process Memory(*)\Dedicated Usage`（G1）；工作集是**内存** |
| 把 `PrivateUsage` 与 `WorkingSetSize` 混着报 | 两者相差 ~27.5 MB（B1：84.1 − 56.6，perf.md §2），必须分列 |
| 把 `68.2 / 75.6 MB` 当本轮 | 那是历史值，G6 复测前一律标"历史" |
| 把 `56.6 MB` 当整产品 | 它是界面单进程；整产品看 G8 |
| 把 `-size` 的结果当高 DPI | 那是布局代理（G9），字号不变 |
| 把"空闲 0 FPS"当故障 | viva 按需绘制；只在 W1–W4 窗口内有帧率概念 |
| 把推断值当实测 | 超预算帧数、纹理增量、交换链占用一律 `"inferred": true` + † |

---

## 8. 验收自检（对照任务书）

| 要求 | 落点 |
|---|---|
| 10 项逐项有交代 | §3 的 G1–G10，每项 6 列齐全 |
| 「怎么采」落到具体 API / 打点位置 | PDH `PdhAddEnglishCounterW` / `K32GetProcessMemoryInfo` / `CreateToolhelp32Snapshot` / `EnumDisplaySettingsW` / `PostMessage` / `SendInput` / `cfg.Paint`(948) / `cfg.OnKey`(1080) / `m.invalidate`(10 处) / `applyEvent`(692) / `kernel.Load`(1298) / `RunCanvas`(1382) / `flex.ScrollState.Wheel`(scroll.go:119) / `app.NewDriver`(driver.go:90) |
| 「拿不到」写清为什么 | G1 纹理、G2 上屏、G3 上屏、G4 上屏、G5 现状、G8 内核侧、G9 全部、G10 内部细分 —— 每条都给了阻断点（viva 无钩子 / 无 PresentMon / `OnScroll==nil` / PMv2+96 DPI） |
| 区分「已实测」「待实测」「拿不到」 | §1.1 分类表（按可测部分拆开算）＋ 每节末的"拿不到"行 |
| 优先级前 3 + 理由 | §6（G5 / G4 / G8），并解释了 G3、G1 为什么不在前三 |
| 需改代码的项与位置 | §1.3 表 + §4 事件表 + §5 前置校验 |

**本文件不含任何新实测数字**：所有数字要么是 perf.md 的引用（258.0 / 153.1 / 56.6–79.0 / 11.8 / 68.2 / 75.6），要么是 §1.1 的只读探针（计数器是否存在、适配器名、显示器分辨率、二进制大小）。10 项本身**一项都还没测**。
