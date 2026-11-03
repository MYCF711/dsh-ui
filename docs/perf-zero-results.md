# 零改动四项实测：G1 显存 / G6 真模型轮 / G7 五会话一致 / G8 宿主整树

> owner：PERF0（UIPLAN §4.1）。被测版本：界面 `bc79784` + 宿主 `4f7c885`。
> 本文件只记录**实测**：数字全部来自进程外采样，命令/退出码/原始序列/证据路径逐项给出。
> 设计口径来自 `docs/perf-gap-plan.md`（G1/G6/G7/G8 四节）；本文件不改 `docs/perf.md`、`docs/progress.md`、UIPLAN。
> 脚本：`scripts/perf-gap.sh`（自包含，采样器随脚本展开）。汇总：`tmp/perf-gap/summary.md` / `summary.json`。

---

## 0. 一页结论

| # | 项 | 状态 | 一句话结果 |
|---|---|---|---|
| G1 | GPU 显存（WDDM） | **measured** | 界面进程静置：Dedicated **5.1 MB**（跨 3 轮 3.1–7.1）、Local 24.3、Shared 19.2、Non Local 0；`Local = Dedicated + Shared` 三条轮次全部成立。**纹理单项拆不出**（unavailable）。`-tail 200` vs `-tail 0` 的差分 −2.0 MB 落在跨轮噪声内 → **不可分辨**，不当增量报 |
| G6 | 真模型一轮 / 带工具一轮 | **measured（骨干）＋ 1 项 unavailable** | 纯文本轮 3/3：turn 中位 **30.2 s**（17.7–43.1），界面峰值 WS **97.5 MB**、priv **139.0 MB**、Dedicated **9.1 MB**；turn 结束静置 10 s 后 WS 95.7/102.9/93.2，**没有回落到发送前 69.4**。工具轮 4 跑 3 有效：峰值 WS 95.4、`tool_calls=1` 真执行。**首字节 / TTFT = unavailable**（零改动无该时标） |
| G7 | 五会话切换（切走再切回） | **判据 FAIL（稳定复现）** | 参考轮 vs 切换轮逐像素差 **73,364–73,424 px（7.3%）**，9/9 对全中；而参考轮之间 0–60 px、切换轮之间 **0 px**。差异 100% 在中栏消息区（bbox x300–676, y102–680，标题栏 0）。根因现场：**直接打开时中栏唯一色数 = 1（纯背景），切回后 185 色** |
| G8 | 宿主整进程树 | **measured** | serve 空载 WS **8.6–8.8 MB** / priv 45.0–45.4；载入会话后 WS 101.3–123.9；一轮进行中（serve + `dshd run`）整树 WS **281.9–304.2 MB**；**界面 + 整树 = 169.7–176.3 MB WS / 243.2–246.6 MB priv** |

**三个跨项发现**（不属这四项，但会污染别人的测试，见 §6）：

1. **脏工作树的 `wssession.go`（H14 重放改动，+30 行未提交）会让 turn 轮进入 WS 重连风暴**：同一 turn、同一参数，脏树 120 s 超时 + **137,246 次重连**（app.log 274,530 行），干净 `4f7c885` 14.0 s 正常结束 + 2 次重连。所以本文件所有宿主侧数字都用 **`4f7c885` 的干净 `git archive` 导出**构建。
2. **PID 复用会把别人的进程算进"宿主整树"**：一次采样里华硕 `ArmourySocketServer.exe` / `asus_framework.exe`（3 层 5 个进程）的 ppid 恰好等于本轮 serve 的 PID。已加"子进程创建时间 ≥ 父进程创建时间"剪枝（§5.3），但正式轮次没再撞上（`tree_dropped_pid_reuse` 全为 0），所以**剪枝生效本身未在正式轮次实证**。
3. **直接打开会话时中栏消息区是空的**（G7 的根因）：`-tail 200` 与 `-tail 40` 都是纯背景（唯一色数 1），切走再切回才有内容（185 色）。这是行为缺陷的现场，不是 G7 判据的噪声。

---

## 1. 版本、环境与执行

### 1.1 被测二进制（sha256 全文）

| 角色 | 路径 | rev | sha256 | 字节 |
|---|---|---|---|---|
| 界面 | `tmp/perf-gap/bin/app.exe` | `bc79784` | `bfc64a519dde20f5e20310ba4964ff6a3c10fc2be559438355bc6c86c5959c83` | 14,155,264 |
| 宿主 | `tmp/perf-gap/bin/dshd-4f7c885.exe` | `4f7c885` | `f7878f5d175bfe034f396434beab69457256b9680468da0c55526d0d4c18c8d4` | 14,606,336 |

**宿主为什么不是直接构建 `../DSH-GO` 工作树**：该工作树有 19 个未提交改动，其中 `internal/transport/wssession.go` 有 +30 行（H14 断线重放）正在被并行 owner 修改；用它构建出来的 serve 在 turn 轮会进入重连风暴（§6.1）。所以脚本从 `git archive 4f7c885` 导出一份**干净源码树**到 `tmp/perf-gap/host-4f7c885/` 再构建（只读于宿主仓，产物落在本任务的临时目录）。

界面二进制来自本仓当前 HEAD（`bc79784`，工作树有 `AGENTS.md`/`PLAN.md` 的改动与若干未跟踪 tmp 文件，**均不是 `cmd/app` 的源码**，不影响 app 行为；`go build ./cmd/app` 的输入就是 HEAD 版本）。

### 1.2 机器与采样环境

| 项 | 值 | 来源 |
|---|---|---|
| 显示器 / 窗口 | 1196×839（客户区 1180×800，96 DPI） | 每轮 `result.json` 的 `window.first_rect` / `last_rect`，3 轮 idle 一致 |
| GPU 计数器 | `GPU Process Memory` 4 条（Dedicated / Local / Shared / Non Local），实例名形如 `pid_<PID>_luid_0x00000000_0x00019B8B_phys_0` | `series.jsonl` 的 `gpu.by_adapter` |
| 显存适配器实例 | 每轮 2 个：`luid_0x…19B8B_phys_0`（NVIDIA，有值）、`luid_0x…1B3C4_phys_0`（GameViewer 虚拟显示，全 0） | 同上 |
| 采样粒度 | 界面内存 + 显存 50 ms；整树 100 ms；静置 10 s | `harness.py` 常量 |
| 采样期间系统 CPU（自采，`GetSystemTimes` 差值，200 ms 窗口） | idle 三轮中位 34.1% / 25.4% / 14.4%（最大 85.7% / 58.3% / 50.5%）；turn 轮中位 4.8–19.3%（最大 38.5–98.2%） | 各轮 `series.jsonl` 的 `sys_cpu_pct`（`turn-text-1` 是加该字段之前跑的，**无此列**） |
| 起始负载闸门 | 系统 CPU 10.9% / 18.8% / 24.4%（三次编排执行的起始值），疑似 Go 构建进程 **0** | `tmp/perf-gap/meta.json` 的 `load_pct_at_start` / `go_build_procs_at_start`（最后一次覆盖写） |

> 关于系统负载：本机是共享的（`SearchIndexer`、`AiPyPro`、其他 owner 的 agent 等一直在跑）。上表如实给出，**没有声称"空闲机"**；idle-1 那轮的 34.1% 中位负载偏高的，其数字（尤其 Dedicated 3.1 MB）应按"可能被抢占"读。任务书要求"不与压测同时报性能数"——本轮没有观察到 Go 编译/链接进程，也没有与其他性能压测同轮（`sys_cpu_pct` 的自采就是这条的证据），但不排除后台常规负载。

### 1.3 临时 home 与进程归属

- 每次场景一个全新临时 home：`tmp/perf-gap/<case>/home/`，内容是真实 home 里 6 个会话的**只读副本**（primary = `session-858445da-685e-43d3-ba73-f25ade9619a8`，7.00 MB；12 个会话里挑的 6 个，总 32.6 MB，单份 ≤ 12 MB）。真实 home（`D:/DeepSeek Harness/home`）只被 `shutil.copytree` 读过，**没有任何写**。
- 端口：`serve --addr 127.0.0.1:0`，URL 与一次性 token 只从 serve 的 stdout `ready {...}` 行取；token 不落盘（`result.json` 里只有 `serve_token_sha256_16`）。
- 进程归属：每轮 `Owned` 记录 serve/app 的 `(pid, ctime, image)`，回收前重读一次并写入 `result.json.owned`（`ctime_match` / `image_match` / `action` / `exit`）。例：`tmp/perf-gap/idle-1/result.json` → `owned[0].app.ctime_match = true`。
- 18 个正式场景轮次里，`leftover_after_stop` 仅在 **`turn-tool-2`** 非空（见 §3.4），其余全为 `[]`；`turn-dirty` 的 5 个"残留"是 PID 复用的别人的进程（§6.2），脚本只记录、**没有杀**。
- 收尾核对（`Get-Process` / `Win32_Process.CommandLine`）：本任务的 `tmp/perf-gap/bin/*` 二进制**没有遗留进程**；系统上另有别人的 `tmp/dshd-serve.exe serve --home tmp/cx-home --policy ask`（PID 20260/18360，00:20 起，属并发 owner 的审批测试），**未触碰**。

### 1.4 实际执行的命令与退出码

| # | 命令 | 退出码 | 说明 |
|---|---|---|---|
| 1 | `bash -n scripts/perf-gap.sh` | **0** | 验收要求的语法检查 |
| 2 | `PERF_GAP_REAL=1 bash scripts/perf-gap.sh` | **0** | 编排 + 汇总（当时所有测量已完成，逐项 `[skip]`，跑 pngmatrix/summarize） |
| 3 | `python tmp/perf-gap/harness.py run … --case idle …` ×3（`-tail 200`）+ ×2（`-tail 0`） | **0** | G1/G8-idle 的 5 轮 |
| 4 | `… --case turn … --app-arg=-smoke --app-arg=-prompt …` ×3（纯文本） | **0** | G6/G8-turn |
| 5 | `… --case turn … --serve-arg=--policy --serve-arg=allow …` ×4（工具轮） | **0** | 其中 `turn-tool-2` 内部 `turn_timeout=true`（§3.4）；进程本身正常退出 0 |
| 6 | `… --case exit --shot …/ref.png` ×3 + `… --switch … --shot …/switched.png` ×3 | **0** | G7 六轮 |
| 7 | `python tmp/perf-gap/pngmatrix.py --work tmp/perf-gap --runs 3` | **0** | G7 矩阵（`verdict=FAIL` 写在 JSON 里，**退出码不表达判据**——`exit 0` 只代表程序跑完） |
| 8 | `python tmp/perf-gap/summarize.py tmp/perf-gap` | **0** | 汇总 |

> **`exit 0` ≠ PASS**。本轮 G7 的判据是 FAIL、G6 有 unavailable 项，这些都是写进证据/本文档的结论，不是退出码。

---

## 2. G1 · GPU 显存（WDDM 计数器）

### 2.1 怎么采

`pdh.dll` 的 English 计数器 API（避免中文 locale 的计数器名）：

```
PdhOpenQueryW(0,0,&q)
PdhAddEnglishCounterW(q, "\\GPU Process Memory(*)\\Dedicated Usage", 0, &c)   # 另外 Local/Shared/Non Local
PdhCollectQueryData(q) → PdhGetFormattedCounterArrayW(c, PDH_FMT_LARGE, …)
```

返回的实例数组里按 `pid_<PID>_` 前缀过滤，同一 adapter 求和、跨 adapter 相加。同一轮里 50 ms 一次，与 `K32GetProcessMemoryInfo`（ws/priv）**同一采样点**。首次采集前有 `present=false` 的预热样本（D3D 设备建好前没有实例），这些行照实留在 `series.jsonl` 里、不计入中位数。

### 2.2 原始数据（静置段最后 1 s 中位数，MB）

`-tail 200`（行为 settle 完、静置 10 s）：

| 轮次 | Dedicated | Local | Shared | Non Local | WorkingSet | PrivateUsage | 采样点数 | 窗口 |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| `idle-1` | **3.1** | 24.3 | 21.2 | 0.0 | 71.1 | 106.1 | 196 | 1196×839 |
| `idle-2` | **5.1** | 24.3 | 19.2 | 0.0 | 69.4 | 105.3 | 196 | 1196×839 |
| `idle-3` | **7.1** | 24.3 | 17.2 | 0.0 | 67.2 | 105.4 | 194 | 1196×839 |
| 中位数 | **5.1** | 24.3 | 19.2 | 0.0 | 69.4 | 105.4 | — | — |

`-tail 0`（同一会话、不拉历史）：

| 轮次 | Dedicated | Local | Shared | Non Local | WorkingSet | PrivateUsage |
|---|---:|---:|---:|---:|---:|---:|
| `idle-tail0-1` | 7.1 | 24.3 | 17.2 | 0.0 | 69.5 | 104.6 |
| `idle-tail0-2` | 7.1 | 24.3 | 17.2 | 0.0 | 69.5 | 106.4 |

### 2.3 结论与读数

- **Dedicated 的跨轮波动（3.1–7.1 MB，跨度 4.0 MB）大于 tail 差分（−2.0 MB）**，且方向与"内容更多→显存更多"相反。所以 `-tail 200 − -tail 0` **不可当内容纹理增量**：它落在噪声内。计划里那个"差分 ≈ 内容纹理增量†"的做法，在本机这台 GPU 上**分辨不出来**，本文件不报该增量。
- 三条 `-tail 200` 轮次都满足 **Local = Dedicated + Shared**（3.1+21.2=24.3；5.1+19.2=24.3；7.1+17.2=24.3），`-tail 0` 也满足（7.1+17.2=24.3）。这是采样自洽的交叉验证（三个量来自同一次 `PdhCollectQueryData`）。
- **Dedicated 只有 3.1–7.1 MB**，而界面 WS 是 67–71 MB：本机这套 viva/D3D11 路径的显存占用很小，**"极限占用"的压力不在 GPU 显存侧**。这与 `docs/perf.md` 里"每条约 0.46 MB 纹理"的旧说法量级不符，那条旧说法**不应当继续引用**（本次也无法验证它，因为它没有给测量方法）。
- Shared 17.2–21.2 MB 是唯一随轮次单调变化的量（3 轮 21.2→19.2→17.2，`-tail 0` 两轮都 17.2）。它与 Dedicated 反向变化、总和恒为 24.3，像是同一笔内存在两个计数器间的归属漂移；**不解释为"内容占显存"**。

### 2.4 unavailable / 未验边界

| 项 | 状态 | 为什么 |
|---|---|---|
| **纹理单项字节数** | **unavailable** | D3D11 没有 per-resource 运行时统计，viva 不暴露资源尺寸；`IDXGIAdapter3::QueryVideoMemoryInfo` 也只给进程总量。要拆需要上游 viva 改。 |
| DWM 合成副本 | **不可测** | 不在进程计数器里 |
| 进程内 DXGI 交叉验证 | **未做** | 计划 §G1 的可选项，需要新文件 `cmd/app/vram_windows.go`（产品代码），本轮不允许改 |
| 更长静置 / 更多轮次 | **未做** | 每场景 3 轮（tail0 2 轮），只够给中位与范围；dedicated 的 4 MB 波动需要更多轮次才能定分布 |

### 2.5 证据

```
tmp/perf-gap/idle-{1,2,3}/{result.json,series.jsonl,tree.jsonl,app.log,serve.log}
tmp/perf-gap/idle-tail0-{1,2}/…
```
`series.jsonl` 每行：`{t_ms, phase, cpu_ms, sys_cpu_pct, ws, priv, peak_ws, gpu:{instances, by_adapter[], dedicated, local, shared, nonlocal, present}}`。

---

## 3. G6 · 真模型一轮 / 带工具一轮

### 3.1 怎么采（零改动）

1. 临时 home 复制真实会话 → `serve --home <copy> --addr 127.0.0.1:0`（读 `ready` 行取 URL/token）。
2. 界面：`-addr {URL} -token {TOKEN} -session <primary> -tail 200 -smoke -prompt "<固定短句>"`（`-smoke` 在首个会话载入后自动发；`-prompt` 覆盖默认的 `smoke-<时间>` 文本）。工具轮另给 `serve --policy allow`，prompt 要求执行一次 `echo hi`。
3. 外部 50 ms 采界面 ws/priv/WDDM 显存 + 100 ms 采整树，直到「turn 结束」再静置 10 s。
4. **turn 结束判据（零改动）**：以 serve 为根的整树里出现 `dshd run` 子进程（`dshd` 的 `loopRunner` 每轮 `exec.CommandContext(exe, "run" …)`）→ 子进程消失并持续 4 s（工具轮 5 s）→ 判 turn 结束；serve 日志出现 `stats: tool_calls=` 作为交叉验证写进证据（`turn_end_basis`）。
   - **不用「CPU 静默」当判据**：本轮实测有一轮 turn 结束后界面进入 WS 重连风暴、CPU 满，CPU 判据会把 20 s 的 turn 拖到超时（`turn-dirty` 就是这么超时的，见 §6.1）。

固定 prompt（sha256 前 16 位写进 `meta.json`）：

| 轮 | prompt | sha256_16 |
|---|---|---|
| 纯文本 | `PERF0 纯文本轮：只回一句 "perf0-text-ok"，不要调用任何工具。` | `8aa78d28a9a7a05e` |
| 带工具 | `PERF0 工具轮：请用 bash 工具执行一次 echo hi，然后把原始输出贴回来。只做这一次工具调用。` | `47a49f261f21cf12` |

### 3.2 原始数据（界面进程，MB；峰值取整个 turn 窗口最大值）

| 轮次 | turn 时长 ms | 峰值 WS | 峰值 priv | 峰值 Dedicated | 结束后静置 10 s WS | 结束后 Dedicated | `dshd run` 峰值 WS | 退出判据 |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| `turn-text-1` | 43,076.4 | 97.5 | 139.0 | 9.1 | 95.7 | 9.1 | 182.9 | turn_done+settle |
| `turn-text-2` | 30,165.3 | 102.9 | 146.4 | 9.1 | 102.9 | 9.1 | 181.7 | turn_done+settle |
| `turn-text-3` | 17,734.2 | 93.6 | 138.4 | 9.1 | 93.2 | 9.1 | 183.2 | turn_done+settle |
| `turn-tool-1` | 45,414.6 | 96.6 | 136.9 | 9.1 | 96.6 | 9.1 | 182.2 | turn_done+settle |
| `turn-tool-2` | —（超时 150 s） | 96.6 | 139.7 | 9.1 | — | — | 182.5 | **turn_timeout** |
| `turn-tool-3` | 24,435.8 | 94.6 | 139.8 | 9.1 | 94.4 | 9.1 | 181.4 | turn_done+settle |
| `turn-tool-4`（补跑） | 10,767.5 | 95.4 | 139.6 | 9.1 | 94.2 | 9.1 | 181.8 | turn_done+settle |

| 组 | 有效/总 | turn 时长中位（范围） | 峰值 WS 中位（范围） | 峰值 priv 中位 | 峰值 Dedicated 中位 | run 子进程 WS 中位 |
|---|---|---:|---:|---:|---:|---:|
| 纯文本 | 3/3 | **30,165.3 ms**（17,734–43,076） | **97.5 MB**（93.6–102.9） | **139.0 MB** | **9.1 MB** | 182.9 MB |
| 带工具 | 3/4 | **24,435.8 ms**（10,768–45,415） | **95.4 MB**（94.6–96.6） | **139.6 MB** | **9.1 MB** | 181.8 MB |

工具轮的真实执行证据（来自 serve 日志，`child(stderr):` 前缀转发）：

```
turn-tool-1  stats: tool_calls=1 input=597090 output=581    spawn pid=…   （1 次）
turn-tool-3  stats: tool_calls=1 input=35456   output=307    spawn pid=…   （1 次）
turn-tool-4  stats: tool_calls=1 input=35198   output=163    spawn pid=…   （1 次）
```

### 3.3 四时刻（计划 §G6 的四个内存时刻）

| 时刻 | 值 | 口径与边界 |
|---|---|---|
| ① 发送前静置 | WS **69.4** / priv **105.4** / Dedicated **5.1** MB | **跨轮参照**：来自 `idle-1..3`（同一二进制、同一 home 副本、同一 `-tail 200`）的静置中位数。**不是同一进程的发送前瞬时值**——`-smoke` 在首个会话载入后立即发，进程内没有"发送前静置 10 s"这个窗口，零改动拿不到。标 **参照值**，不是同一轮的数 |
| ② 首字节到达 | **unavailable** | 零改动无该时标：app 侧没有"首字节"日志（`streamDebug` 只在"有思考且无正文"时打前 3 条），serve 侧 turn 期间只打 `spawn`/`follow`。**不拿设计值或内存曲线推测冒充** |
| ③ 流式峰值 | WS **97.5** / priv **139.0** / Dedicated **9.1** MB（纯文本 3 轮中位） | 实测，turn 窗口最大值；工具轮 95.4 / 139.6 / 9.1 |
| ④ turn 结束后静置 10 s | WS **93.2–102.9**（中位 95.7）/ Dedicated 9.1 | 实测，最后 1 s 中位数。**没有回落到 ①（69.4）**：净增 ≈ +26 MB WS / +34 MB priv，10 s 静置不释放 |

> ③④ 的 Dedicated 是 **9.1 MB，三轮完全一致**（而 idle 是 3.1–7.1 抖动）：一旦真跑过一轮，显存里多出来的那块（≈ +2–6 MB）在整个静置段都保留着。这是"一轮之后显存不回落"的直接证据。

### 3.4 一次超时（`turn-tool-2`）的实况

- 现象：turn 子进程（PID 4840）在 `t=559.6 ms` 起来，到 `t=149,919 ms`（150 s 上限）**仍在**，`serve.log` 里没有任何 `stats: tool_calls=` 行（这一轮模型没跑完）。harness 按 150 s 上限退出并回收。
- 回收记录：`result.json.leftover_after_stop` 里当时还有 `{pid:4840, ppid:22648, name:"dshd-4f7c885.exe", ws:134,144,000, priv:169,857,024}`（父 PID = 本轮 serve）；随后 `Get-Process -Id 4840` 返回空，`Get-Process -Name dshd-4f7c885` 计数 0 → 回收后无残留。
- 该轮**不计入中位**（`tool_median.n_valid=3 / n_total=4`），补跑 `turn-tool-4`。
- **未验**：这一轮到底卡在模型侧（长思考 + 生成慢）还是 `pause_turn`/工具循环未返回——零改动下只能看到"子进程不退出 + 没有 `stats` 行"。日志里同轮有 `自动压缩失败（不影响本轮）：模型返回了空摘要（finish="length"）`，那是压缩子任务，不是 turn 本身。

### 3.5 未验边界

| 项 | 为什么 |
|---|---|
| TTFT / 首字节时刻 | 需 `-trace` 打点（G2/G6 的"要打点"半边），零改动做不到 |
| 模型输出长度可比性 | 不可控：纯文本轮 input 580,268 token（这个会话上下文很长）、output 175；工具轮 input 35k–597k、output 163–581。**只报内存与时长，不报 token 级可比性** |
| 「发送前」同一进程瞬时值 | 见 ①，只能跨轮参照 |
| `turn-tool-2` 的根因 | 见 3.4 |
| 真审批路径 | 工具轮用的是 `--policy allow`（免审批）。`ask` 策略需要界面按 Enter 批准，属 H13，不在本项 |

### 3.6 证据

```
tmp/perf-gap/turn-text-{1,2,3}/   tmp/perf-gap/turn-tool-{1,2,3,4}/
```
每目录：`result.json`（`phases`/`peak`/`turn_children`/`turn_end_basis`/`owned`/`leftover_after_stop`）、`series.jsonl`（界面 50 ms）、`tree.jsonl`（整树 100 ms）、`app.log`、`serve.log`。

---

## 4. G7 · 五会话切换（切走再切回 vs 直接打开）

### 4.1 怎么采（零改动，`-shot`）

- 6 个会话副本（primary + 5 个不同 id）。切换序列 = `S2,S3,S4,S5,S6,S1(primary)`，`-switch-every 1`（2 s 后开始、每 1 s 一个，末尾打印 `switch 结束后当前会话`）。
- 参考轮：`-session S1 -tail 200 -shot ref.png -shot-wait 12`
- 切换轮：`-session S1 -tail 200 -switch <seq> -switch-every 1 -shot switched.png -shot-wait 13`
- 判据：两图尺寸相同 + **逐像素差为 0**（先全窗，再把「标题栏 y<34」单独扣出来，因为窗口激活态会改标题栏配色）。

### 4.2 原始矩阵（1196×839，`tmp/perf-gap/g7-matrix.json`）

| 组 | 对数 | `diff_pixels` |
|---|---:|---|
| ref × ref（自身噪声底） | 3 | **60 / 60 / 0**（差异只在 bbox [236,178]–[247,185]，一个 12×8 的图标区） |
| sw × sw（自身噪声底） | 3 | **0 / 0 / 0**（三张切换图逐像素完全相同） |
| **ref × sw（判据）** | **9** | **73,424 / 73,424 / 73,424 / 73,364 ×6**（7.31%–7.32%） |

- 9 对 `ref×sw` 的 bbox 全部落在中栏：**x 300–676、y 102–680**（有一组从 x=236 起，多出的 64 px 是侧栏选中行的高亮块）。
- **标题栏（y<34）差异 = 0**：这不是窗口激活态噪声。
- 每一轮切换日志都证明末尾回到了 primary：
  ```
  switch 结束后当前会话: session-858445da-685e-43d3-ba73-f25ade9619a8   （×3 轮全一致）
  ```

### 4.3 判据结果：**FAIL（稳定复现，不是噪声）**

噪声底（ref×ref 0–60 px、sw×sw 0 px）比判据的差异（7.3 万 px）小三个数量级，且 9/9 对同向。这个差异是**确定性的**。

### 4.4 根因现场（零改动能做到的诊断）

对两张图的中栏矩形 `(300,102)-(676,680)` 做颜色统计：

| 图 | 中栏唯一色数 | 说明 |
|---|---:|---|
| 参考轮 `ref.png`（直接打开 S1，`-tail 200`） | **1** | 217,328 像素全是背景色 `(21,21,23)` —— **消息区完全空白** |
| 参考轮（直接打开 S1，`-tail 40`，对照轮） | **1** | 与 tail 无关 |
| 切换轮 `switched.png`（切走 5 个再切回） | **185** | 有消息、工具步骤卡、代码块 |

也就是说：**「直接打开会话」时中栏什么都没画，而「切走再切回」后能正常显示**。两边的状态栏都写着「已连接 · 109 条（总 -1）」、中栏标题都是「共 109」，所以不是"没加载到数据"。

这是**行为缺陷的现场**，值得单独建项（不在本任务范围，也不是 G7 判据本身）：它意味着用户直接打开一个大会话时，看到的是空白消息区。

**未验的根因细分**（零改动只能到这一步）：

- 是滚动位置（`ScrollState.Offset` 在内容之外，切回时被恢复成"能看见内容"的位置）？
- 还是首开时的虚拟化/布局没把消息放进视口（切回时重算了一次）？
- 还是 `-shot` 的 `PrintWindow` 在首开时抓到了未合成的一帧？

要回答需要 `-dump {cur, msgs, lastTextSha256, scrollOffset}`（计划 §G7 的推荐版，~20 行产品代码）或 `-trace` 打点——本轮不允许改产品代码。

### 4.5 证据

```
tmp/perf-gap/g7-{1,2,3}/ref.png  tmp/perf-gap/g7-{1,2,3}/switched.png
tmp/perf-gap/g7-{1,2,3}/{ref,sw}/{result.json,app.log,serve.log,series.jsonl,tree.jsonl}
tmp/perf-gap/g7-matrix.json          # 9+6 对，含 bbox / 标题栏拆分
tmp/perf-gap/g7-tail40/ref.png       # 对照：-tail 40 直接打开，中栏同样唯一色数=1
```

---

## 5. G8 · 宿主整进程树（serve + `dshd run` + 工具进程）

### 5.1 怎么采（零改动）

`CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS)` 一次全量快照 → 建 `ppid → children` 映射 → 从**我们自己 Popen 出来的 serve PID** 递归 BFS（不按镜像名扫）。每个 PID：`K32GetProcessMemoryInfo`（ws/priv）。100 ms 一次。

**归属校验（PID 复用剪枝）**：子进程的创建时间必须 **≥** 父进程的创建时间，否则这条边是 PID 复用造成的假父子，剪掉并把 `{pid,ppid,why,child_ctime,parent_ctime}` 记进 `tree_dropped_pid_reuse`。为什么需要：见 §6.2。

### 5.2 原始数据（MB；四时刻）

| 轮次 | ① serve ready 空载 WS/priv | ② 会话载入后 WS/priv | ③ 一轮进行中整树 WS/priv（进程数） | ④ 一轮结束后 WS/priv |
|---|---|---|---:|---:|
| `idle-1` | 8.7 / 45.1 | 113.1 / 148.2 | —（无 turn） | 105.2 / 140.5 |
| `idle-2` | 8.7 / 45.0 | 104.7 / 140.2 | — | 104.5 / 140.0 |
| `idle-3` | 8.8 / 45.4 | 122.5 / 157.9 | — | 102.5 / 137.8 |
| `turn-text-1` | 8.6 / 45.0 | 122.9 / 158.2 | **281.9 / 351.0**（2） | 96.1 / 130.7 |
| `turn-text-2` | 8.7 / 45.1 | 123.5 / 158.7 | **282.9 / 352.5**（2） | 54.8 / 89.5 |
| `turn-text-3` | 8.7 / 45.1 | 118.9 / 154.5 | **304.2 / 373.9**（2） | 109.1 / 144.0 |
| `turn-tool-1` | 8.7 / 45.2 | 101.3 / 137.0 | **295.6 / 365.9**（2） | 95.3 / 130.4 |
| `turn-tool-2` | 8.6 / 45.0 | 123.1 / 158.7 | **302.7 / 373.4**（2） | 50.7 / 85.7 |
| `turn-tool-3` | 11.3 / 45.0 | 107.8 / 142.3 | **294.3 / 363.4**（2） | 60.4 / 94.5 |
| `turn-tool-4` | 8.6 / 45.0 | 123.9 / 159.5 | **300.9 / 371.1**（2） | 52.0 / 87.0 |

一轮进行中的整树（`turn-text-1` 的原始记录，来自 `tree.jsonl`）：

```json
{"t_ms": …, "phase": "turn", "kids": 2, "procs": [
  {"pid": …, "depth": 0, "name": "dshd-4f7c885.exe", "ws": 103,940,096, "priv": 140,222,464},
  {"pid": …, "depth": 1, "name": "dshd-4f7c885.exe", "ws": 191,791,104, "priv": 227,852,288}]}
```

（上面表里的 281.9 MB 是两个进程 ws 相加；`turn_children` 里记的 `ws_peak` 是子进程自己的峰值 182.9 MB。）

### 5.3 结论与读数

- **serve 空载只有 8.6–8.8 MB WS / 45.0–45.4 MB priv**（11 轮含 turn 轮全部一致）。45 MB priv 与 8.7 MB ws 的差是 pagefile 回退——**这两个量必须分列，不能互相代替**。
- **载入一个 7 MB 的会话（`-tail 200`）把 serve 顶到 101–124 MB WS**：serve 把 journal/消息读进内存并推给客户端。这一步比界面进程（69 MB）还大。`docs/perf.md` 里"serve 空载 11.8 MB"只覆盖了空载那一刻，**不能代表"宿主占多少"**。
- **一轮进行中整树 = 281.9–304.2 MB WS / 351.0–373.9 MB priv**（serve + 一个 `dshd run` 子进程；纯文本轮也是 2 个进程——`loopRunner` 每轮都 spawn）。其中 `dshd run` 子进程单独就 181.7–183.2 MB WS。
- **界面 + 整树（会话载入后静置）**：

| 轮 | 界面 WS | 整树 WS | 合计 WS | 界面 priv | 整树 priv | 合计 priv |
|---|---:|---:|---:|---:|---:|---:|
| `idle-1` | 71.1 | 105.2 | **176.3** | 106.1 | 140.5 | **246.6** |
| `idle-2` | 69.4 | 104.5 | **173.9** | 105.3 | 140.0 | **245.3** |
| `idle-3` | 67.2 | 102.5 | **169.7** | 105.4 | 137.8 | **243.2** |

  这就是计划 §G8 要回答的"整产品"：**≈ 170–176 MB WS / 243–247 MB priv**（一个 7 MB 的会话、静置）。它不是 `docs/perf.md` §6 的 56.6–79.0 MB（那只是界面）。
- `turn` 结束后的 serve ws 波动很大（50.7–109.1 MB）：有的轮次释放了会话缓存（`turn-text-2` → 54.8），有的没有（`turn-tool-4` → 52.0 也不高，但 `turn-text-3` 109.1 反而不降）。**零改动看不出释放策略的触发条件**（需要宿主自己的 `/debug/stats` 或打点）。

### 5.4 PID 复用剪枝：已实现，本轮未再次触发

- 发现污染的现场（`turn-dirty`，旧采样器）：`serve_ready_tree` 里 5 个进程的 ppid 恰好等于本轮 serve PID（12444）：
  `ArmourySocketServer.exe`(depth 1) → `asus_framework.exe` ×2 / `ArmouryHtmlDebugServer.exe`(depth 2) → `asus_framework.exe` ×2(depth 3)。
  这些是**别人的进程**（华硕 Armoury Crate），`leftover_after_stop` 里也带着它们；脚本只记录、没有杀。
- 剪枝逻辑已加（子进程创建时间 ≥ 父进程创建时间），默认开启。
- 但**正式 18 轮里 `tree_dropped_pid_reuse` 全是 0**：没有再次遇到 PID 冲突，所以"剪枝真的能剪掉"没在正式轮次中被实证。这是一个明确的未验项（§7）。
- 该轮数字（281.9/304.2 这类）全部来自剪枝后的正式轮次，且每轮 `procs` 只有 2 个进程，与"serve + 1 个 run"的期望一致 → 正式数据没有混入别人的进程。

### 5.5 未验边界

| 项 | 为什么 |
|---|---|
| 内核/DWM/驱动侧占用 | 不在进程计数器里 |
| `dshd run` 内部的 goroutine/堆细节 | 要宿主 `/debug/stats`（只报宿主自己，不能替代整树） |
| 工具进程（`bash`/`powershell`）的独立树 | 工具轮里 `dshd run` 执行 `echo hi`，进程太快（<1 s），100 ms 采样没有稳定抓到 `bash.exe` 子进程；`turn_children` 里只有 `dshd run` |
| serve 释放策略的触发条件 | 见 5.3 |
| 剪枝的有效性 | 见 5.4 |

### 5.6 证据

```
tmp/perf-gap/{idle-1…3,turn-text-1…3,turn-tool-1…4}/tree.jsonl    # 每 100 ms 一行整树
tmp/perf-gap/…/result.json                                         # phases.serve_ready_tree / tree_dropped_pid_reuse / leftover_after_stop
```

---

## 6. 三个跨项发现（会影响别人的测试，建议 Lead 单独派单）

### 6.1 脏工作树的 `wssession.go` 让 turn 轮进入 WS 重连风暴（**高优先级**）

同一台机器、同一个界面二进制、同一个临时 home、同一个 prompt，只换 serve：

| serve | 来源 | turn 结果 | app.log 重连次数 | app.log 行数 | 界面峰值 ws |
|---|---|---|---:|---:|---:|
| `dshd-4f7c885.exe` | `4f7c885` 干净 archive | **turn_done**，14.0 s | **2** | 25 | 109.7 MB |
| `dshd-serve.exe` | `../DSH-GO` 工作树（`wssession.go` +30 行未提交） | **turn_timeout**（120 s 上限） | **137,246** | 274,530 | 87.1 MB |

- 风暴形态（每秒约 2,200–2,400 行）：`mux 已订阅` → `mux 连接断开 … 带水位 N 重连` → `mux 断号 [a,b]` → `history 补洞完成 … 补回 0 帧` → 立即再订。
- 水位在风暴里快速推进（0 → 9 → 75 → 231 → 447 → 716 …），每次 `断号` 补洞都"补回 0 帧"。
- 脏树里那份 `wssession.go` 未提交改动正是 **H14 的"断线重放"**：`j.Since(frame.SinceSeq, 0)` 把漏掉的帧一次性 `sub.write(f)` 出去，写失败就 `return err` 关连接。turn 期间 journal 帧量大，重放 → 断开 → 带水位重订 → 再重放。
- **对别人的影响**：任何用当前脏工作树构建 dshd 去跑 h-suite / race / 真宿主链路的测试，turn 期间的界面侧行为（重连风暴、满 CPU）都会污染内存/CPU/延迟数字；`perf-ui.sh` 的历史轮次（B1/C/D1）没有这个风暴，说明它是在这段时间的改动之后才出现的。
- **本文件的处置**：所有宿主侧数字只用 `4f7c885` 的干净构建；风暴本身**不在本任务范围**（不改产品代码），仅作为证据上报。

### 6.2 PID 复用会把别人的进程算进整树（已修，见 §5.4）

`ArmourySocketServer.exe` 的 ppid 恰好等于本轮 serve PID → 不剪枝就会把 5 个华硕进程（合计 ~25 MB ws）算进"宿主整树"。`AGENTS.md` §4 的"不按镜像名杀进程"在这里救了一次：脚本只记录、不动手。

### 6.3 直接打开会话时中栏消息区是空的（G7 的根因，行为缺陷）

见 §4.4：参考轮（`-tail 200` 与 `-tail 40` 都试过）中栏唯一色数 = 1（纯背景），切走再切回后 = 185 色。状态栏/标题的条数一致（109 条），所以数据是加载了的。这是**用户可见**的缺陷候选，建议单独建项（需要 `-dump` 或打点才能定位到滚动位置 vs 渲染时机）。

---

## 7. unavailable 与未验边界（汇总）

| 项 | 状态 | 原因 / 需要什么 |
|---|---|---|
| G1 纹理单项字节数 | **unavailable** | D3D11 无 per-resource 统计，viva 不暴露；需上游改 |
| G1 DWM 合成副本 | 不可测 | 不在进程计数器 |
| G1 tail 差分的"内容纹理增量" | **不可分辨** | 差分 −2.0 MB 落在跨轮噪声 3.1–7.1 MB 内；不当增量报 |
| G1 进程内 DXGI 交叉验证 | 未做 | 需要新文件 `cmd/app/vram_windows.go`（产品代码） |
| G6 首字节 / TTFT | **unavailable** | 零改动无该时标；需 `-trace` 打点 |
| G6 发送前静置（同一进程瞬时） | **不可得** | `-smoke` 紧随载入发送；用 idle 轮跨轮参照代替并标注 |
| G6 模型输出可比性 | 未承诺 | 输出长度不可控（input 35k–597k token，output 163–581） |
| G6 `turn-tool-2` 卡住根因 | 未验 | 只能看到"子进程 150 s 不退出 + 无 `stats` 行" |
| G7 差异根因细分 | **未验** | 需 `-dump`（scrollOffset/msgs 结构化）或打点；零改动只能给到"中栏空白 vs 有内容" |
| G7 流式期间切走再切回 | 不在本项 | 属 P0-B 的真宿主验收 |
| G8 内核/DWM 侧 | 不可测 | 不在进程计数器 |
| G8 工具子进程（bash/powershell） | 未抓到 | `echo hi` < 1 s，100 ms 采样没稳定抓到 |
| G8 PID 复用剪枝的有效性 | **未在正式轮次验证** | 15 轮 `tree_dropped_pid_reuse` 全 0（没再撞上）；污染现场在 `turn-dirty`（旧采样器） |
| G7/G1 的轮次数 | 偏少 | G1 tail0 只有 2 轮；dedicated 的 4 MB 波动需要更多轮次才能定分布 |

---

## 8. 怎么复跑

```bash
cd "D:/DeepSeek Harness/default-workspace/dsh-ui"

# 语法（验收要求）
bash -n scripts/perf-gap.sh

# 只跑不吃额度的三项（G1/G7/G8）
bash scripts/perf-gap.sh

# 连真模型轮一起跑（要 magpie 在 127.0.0.1:3425 返回 200，会消耗真实额度）
PERF_GAP_REAL=1 bash scripts/perf-gap.sh

# 只补某一项 / 强制重跑
PERF_GAP_ONLY=G7 PERF_GAP_REUSE=0 bash scripts/perf-gap.sh
```

- 默认 `PERF_GAP_REUSE=1`：已有 `result.json` 的轮次直接跳过（本文档的数字就是这么保留下来的）；`PERF_GAP_REUSE=0` 全部重跑并覆盖。
- 宿主二进制默认从 `git archive 4f7c885` 的干净导出构建；要换 rev：`PERF_GAP_HOST_REV=<rev>`。**不要**直接指向脏工作树的构建（§6.1）。
- 负载闸门：`PERF_GAP_LOAD_MAX=70`（默认）；超阈值打印警告并继续（把负载记进 meta）。要让它在压测期间直接失败：`PERF_GAP_LOAD_STRICT=1`（退出码 3）。

产物：`tmp/perf-gap/summary.md`（人读）、`summary.json`（机器读）、`meta.json`（rev/sha256/负载/prompt sha）、各轮 `result.json` + `series.jsonl` + `tree.jsonl` + 两个日志。

---

## 9. 证据索引

| 路径 | 内容 |
|---|---|
| `scripts/perf-gap.sh` | 编排 + 自包含采样器（harness/summarize/pngmatrix 随脚本展开到 `tmp/perf-gap/`） |
| `tmp/perf-gap/summary.md` / `summary.json` | 四项汇总（本文档的表都能从这里复算） |
| `tmp/perf-gap/meta.json` | ui/host rev、两个 `sha256`、起始负载、Go 构建进程数、prompt sha256_16 |
| `tmp/perf-gap/template-meta.json` | 6 个会话副本的 id/project/字节数、真实 home 里会话总数（303） |
| `tmp/perf-gap/bin/{app.exe,dshd-4f7c885.exe}` | 被测二进制 |
| `tmp/perf-gap/host-4f7c885/` | 从 `git archive 4f7c885` 导出的干净宿主源码（构建用，只读于宿主仓） |
| `tmp/perf-gap/idle-{1,2,3}/` | G1/G8 idle（`-tail 200`） |
| `tmp/perf-gap/idle-tail0-{1,2}/` | G1 的 `-tail 0` 对照 |
| `tmp/perf-gap/turn-text-{1,2,3}/` | G6 纯文本轮 |
| `tmp/perf-gap/turn-tool-{1,2,3,4}/` | G6 工具轮（`-2` 超时，`-4` 补跑） |
| `tmp/perf-gap/g7-{1,2,3}/{ref,sw}/` + `*.png` | G7 六轮抓图与日志 |
| `tmp/perf-gap/g7-matrix.json` | G7 的 15 对逐像素结果（含 bbox、标题栏拆分） |
| `tmp/perf-gap/g7-tail40/` | G7 根因对照（`-tail 40` 直接打开，中栏同样空白） |
| `tmp/perf-gap/turn-base4f7/` vs `turn-dirty/` | §6.1 的脏树 vs 干净树对照 |
| `tmp/perf-gap/smoke-idle/`、`smoke-g7/`、`smoke-turn/` | 脚本成型前的冒烟轮（不进本文档数字） |
