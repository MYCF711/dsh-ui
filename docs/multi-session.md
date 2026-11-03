# 多会话并发隔离与切换稳定性（真宿主 + 真模型）

> 任务：3 会话同时跑不串台、反复切会话不错配、来回切不涨内存。
> 结论先行：**18 张截图、15 次起停、5+5 个内存采样点，全部通过；没发现标题与内容错配。**
> 这一版对应二进制 `tmp/app-real.exe`（sha256 `f57466a0…`，17:46 构建）与 `tmp/dshd-serve.exe`（sha256 `779290c8…`，宿主 HEAD 重建）。
> 另用**当前源码重编**的 `tmp/ms-app-head.exe`（`git 0de2c8d` + 未提交改动）复截 A：title/msg/status 三个区域与 `app-real.exe` **指纹完全一致**，说明源码没在这条路径上漂移。
> 同一支新二进制截 B 时指纹与 `app-real.exe` 的不同，原因是 B 的会话在 `phase4` 之后多了 6 条消息（`total` 8 → 14、用量 21.9k → 41.9k），不是渲染回归——对照截图 `head-B.png` 与 `p2-B.png` 就能看出多出来的就是那三条 `sleep 20`。

## 断言结果

| # | 断言 | 结论 | 证据 |
|---|---|---|---|
| 1a | 3 个会话各自收到可区分回复（MARK-ALPHA / BETA / GAMMA） | **验到** | `setup` 输出：A `total=2`、B `total=8`、C `total=8`；`usage` 三条各自独立 |
| 1b | 界面连 A、B/C 都在跑时截图，A 的消息区**不含** BETA/GAMMA 任何内容 | **验到** | `tmp/ms-run-r2/p1-A-concurrent.png`；宿主日志证明截图时刻 B、C 都在 `sleep 25` 的 bash 里 |
| 1c | 界面连 B、再连 C，各自只显示自己的标记 | **验到** | `tmp/ms-run-r2/p2-B.png`、`tmp/ms-run-r2/p2-C.png` |
| 1d | 侧栏也"完全不出现"别人的字 | **没验到（按设计不成立）** | 侧栏会列出另外两个会话的**标题**（`MARK-BETA` / `MARK-GAMMA`），来自 `session.list`，是正常列表；消息区/顶栏/状态栏/轨迹栏没有它们的任何内容 |
| 2 | 3 会话 × 5 次起停，每次截图里标题/消息区/用量属于同一会话 | **验到** | 15 张 `p3-*.png`；同一会话 5 张的 title/msg/status 三区 md5 **全等**，跨会话三区 md5 **互不相同** |
| 3a | 来回切 15 次工作集不持续上涨（5 个采样点） | **验到** | 58.9–62.3 MB 区间振荡；首末 A −3.4 / B −2.3 / C −0.9 MB |
| 3b | 进程内不涨（后台会话在流时） | **验到** | 常驻客户端 5 点：59.0 → 59.8 → 59.8 → 59.8 → 59.8 MB，句柄 1193 → 1194 |
| 4 | 有没有"标题是 A、内容是 B"这类错配 | **没有** | 18 张截图逐张比对，三处文字与宿主 `session.load` 的用量逐项对得上 |

## 环境与复现

```bash
cd "D:/DeepSeek Harness/default-workspace/dsh-ui"
export MS_RUN=r2
python tmp/ms-e2e.py setup      # 真宿主（临时 home、OS 分配端口、--policy allow）+ /connect + 3 会话 + 3 条 prompt
python tmp/ms-e2e.py phase1     # B、C 起长轮；A 在这段时间里截图
python tmp/ms-e2e.py phase2     # B、C 静置后各截一张
python tmp/ms-e2e.py phase3 5   # 3 会话 × 5 次起停，每次截图 + 外部量内存/句柄
python tmp/ms-e2e.py phase4     # 常驻客户端看 A，B、C 在跑，采 5 个内存点
python tmp/ms-e2e.py crops      # 按区域算 md5，判"同会话一致 / 跨会话不同"
python tmp/ms-e2e.py usage      # 宿主侧 per-session usage，与截图上的数字对账
python tmp/ms-e2e.py teardown   # 只回收自己起的宿主 PID
```

- 宿主：`./tmp/dshd-serve.exe serve --home tmp/ms-run-r2/home --policy allow`（不带 `--addr`，端口由 OS 分配，从 stdout 的 `ready {"url","token"}` 读）。
- token 一次性：脚本用 `/connect?token=…` 换 `dshd_session` cookie，界面用 `-cookie dshd_session=<v>` 与 HTTP 共用同一个宿主。
- 界面起停：`tmp/app-real.exe -addr <url> -cookie <v> -session <id> -shot <png> -shot-wait N -title dsh-ui-<tag>`（窗口只在屏幕外抓图，不弹给用户）。
- 全部原始记录：`tmp/ms-run-r2/results.jsonl`（18 条截图记录 + 1 条并发快照 + 1 条进程内采样）。

## 一、三会话并发不串台

三个会话同一宿主、同一个 cookie，三条 prompt 只用 ASCII，避免任何编码变量：

| 会话 | id | prompt | 回复 |
|---|---|---|---|
| A | `ad76b706…` | `Reply with exactly: MARK-ALPHA. No tools, no explanation, nothing else.` | `MARK-ALPHA` |
| B | `06d66e0c…` | 同上，`MARK-BETA` | `MARK-BETA` |
| C | `b2dc4010…` | 同上，`MARK-GAMMA` | `MARK-GAMMA` |

### A 的截图确实落在 B、C "在跑"的窗口里

不是"发完就截"的想当然——用宿主日志和轮次状态双重对齐：

| 时刻 | 事件 |
|---|---|
| 18:02:00.24 | 宿主 `会话开始`：B（`06d66e0c`）、C（`b2dc4010`）同时开新轮 |
| 18:02:02.8 | 两个会话各起一条 bash（`ms=25121` / `25121`，18:02:27.9 与 18:02:28.0 才返回）→ **25 秒的工具调用** |
| **18:02:09.43** | **A 的截图落盘**（`p1-A-concurrent.png`） |
| 18:02:09.45 | 立刻回查：B `total=4`、C `total=4`，末条都是**空的 assistant** → 两轮都还没结束 |

也就是说截图那一刻，B、C 都卡在各自的 `sleep 25` 里，而 A 的界面是干净的。

### 屏幕上的字（逐张实读）

**A（并发窗口内）：`tmp/ms-run-r2/p1-A-concurrent.png`**

- 顶栏：`MARK-ALPHA · dsh-ui`，右侧 `轨迹·开`、`共 2 条`
- 消息区：用户气泡 `Reply with exactly: MARK-ALPHA. No tools, no explanation, nothing else.` → 助手 `MARK-ALPHA` → `复制  重试`
- 底部：`1 轮 · 0 步` · `↑7.1k ↓6` · `上下文 7.1k`；右端 `已连接 · 2 条 (总 2)`；左端 `3 个会话`
- 右栏轨迹：`本会话还没有工具调用`
- 侧栏：`MARK-BETA`、`MARK-GAMMA`、`MARK-ALPHA`（三行标题，属 `session.list` 正常内容）

**B：`tmp/ms-run-r2/p2-B.png`**

- 顶栏：`MARK-BETA · dsh-ui`，`共 8 条`
- 消息区：`MARK-BETA` 问答 → 第二轮 prompt → 3 张 bash 工具卡（`{"command":"sleep 25","description":"sleep 25 (call 1/2/3)"}`）→ `MARK-BETA`
- 底部：`3 轮 · 3 步` · `↑21.7k ↓262` · `上下文 7.4k`；`已连接 · 8 条 (总 8)`
- 右栏轨迹：3 次 bash

**C：`tmp/ms-run-r2/p2-C.png`**

- 顶栏：`MARK-GAMMA · dsh-ui`，`共 8 条`
- 消息区：`MARK-GAMMA` 问答 → 3 张 bash 工具卡（`Sleep 25 seconds (call 1/2/3 of 3)`）→ `MARK-GAMMA`
- 底部：`3 轮 · 3 步` · `↑21.7k ↓391` · `上下文 7.4k`；`已连接 · 8 条 (总 8)`
- 右栏轨迹：3 次 bash

三处数字与宿主同刻的 `session.load` **逐项对得上**（判据不是"看着像"）：

| 会话 | 屏幕 `↑/↓/上下文` | 宿主 `usageTotal.inputTokens / outputTokens` / `contextTokens` |
|---|---|---|
| A | 7.1k / 6 / 7.1k | 7115 / 6 / 7121 |
| B | 21.7k / 262 / 7.4k | 21675 / 262 / 7414 |
| C | 21.7k / 391 / 7.4k | 21694 / 391 / 7430 |

**"不串"的机内证据**：每个客户端进程的日志各只有一条订阅，且是自己的会话：

```
p1-A.log  mux 已订阅 session:ad76b706-…
p2-B.log  mux 已订阅 session:06d66e0c-…
p2-C.log  mux 已订阅 session:b2dc4010-…
```

18 个客户端进程（3 + 15）全部如此，没有一条订阅别人的 topic。

## 二、快速切换压力（3 会话 × 各 5 次起停）

界面没有切换会话的热键，侧栏也不能脚本化点击（`-keys` 只注入 `down/up/home/end/esc/enter`，`main.go` 里 `openSession` 只有侧栏和启动两个入口），所以"切换"是**进程重启语义**：每次 `-session <id>` 起一个新进程、重新 `session.load`、自己订阅、自己截图、自己退出（15 次 15 个 PID，全部 `rc=0`）。

判"标题/内容/用量同属一个会话"用的是**区域指纹**而不是整图 md5（整图会因侧栏相对时间、窗口标题而不同）：

- 区域：title = 顶栏标题条，msg = 消息列，status = 底部状态栏（含 `↑/↓/上下文`）；侧栏单独算、**不参与**这条断言。
- 同一会话 5 张：title / msg / status **三个区域 md5 全等** → 文字、排版、数字逐像素相同。
- 跨会话：三个区域的 md5 **互不相同** → 不存在"两张图其实是同一个会话"的假通过。

| 区域 | A（5 张） | B（5 张） | C（5 张） |
|---|---|---|---|
| title | `5e63891c80f5` ×5 | `82ff728c658b` ×5 | `3b7f6dd43b1e` ×5 |
| msg | `355a3863f129` ×5 | `21f0589deae4` ×5 | `f3953913c73e` ×5 |
| status | `8895e146ec96` ×5 | `be7a8d39068c` ×5 | `55840b6677f4` ×5 |

额外一条：`p1-A-concurrent.png`（B、C 正在 `sleep 25` 时截的）的三个区域指纹与 A 的 5 张静置截图**完全相同** —— 后台会话在流，前台 A 的渲染一个像素都没动。

侧栏区域 md5 在 `r3→r4` 之间变化（`刚刚` → `2 分钟前`），符合预期，也说明区域切分抓得到"该变的地方变了、不该变的没变"。

## 三、内存与句柄

全部从进程外部量（`K32GetProcessMemoryInfo` / `GetProcessHandleCount`，ctypes 直调），不读界面自报的数字。

### 3.1 跨进程：15 次起停，5 个采样点（每点 = 一轮 A/B/C）

| 采样点 | A 工作集 | B 工作集 | C 工作集 |
|---|---|---|---|
| #1（第 1 轮） | 62.3 MB | 62.0 MB | 60.5 MB |
| #2 | 62.0 MB | 60.4 MB | 59.9 MB |
| #3 | 59.3 MB | 59.9 MB | 60.4 MB |
| #4 | 59.5 MB | 62.2 MB | 59.7 MB |
| #5 | 58.9 MB | 59.7 MB | 59.6 MB |

- 全体区间 **58.9–62.3 MB**（极差 3.4），**首末都在往下走**：A −3.4、B −2.3、C −0.9 MB。
- 句柄 1048–1202，同样在振荡、无单调趋势（A 1048/1202/1172/1171/1163）。
- 每次起停的完整耗时 6.05–6.32 s（窗口起来 + `-shot-wait 5` + 抓图 + 退出）。

### 3.2 进程内：常驻客户端，后台两个会话在流

一个不截图、不退出的客户端看 A；期间让 B、C 各跑三条 `sleep 20`（宿主日志：18:07:02 两轮同时开跑，bash `ms≈20122/20117/20040/20035/20068/20043`），每 18 秒采一次：

| 采样点 | 工作集 | PrivateUsage | 句柄 |
|---|---|---|---|
| #1（B、C 刚开跑） | 59.0 MB | 84.7 MB | 1193 |
| #2 | 59.8 MB | 85.3 MB | 1196 |
| #3 | 59.8 MB | 85.3 MB | 1196 |
| #4 | 59.8 MB | 85.3 MB | 1196 |
| #5（B、C 跑完） | 59.8 MB | 85.2 MB | 1194 |

+0.8 MB 后**完全走平**，句柄 +3 后回落 —— 与 `docs/progress.md` 里性能基线那条（"进程内 5 组 ×10 次切会话 78.1→82.1 后走平"）同向。**没有持续上涨**。

### 3.3 第一次运行（RUN=r1）是同一结论

换一套 home 与端口重跑了一整轮（`tmp/ms-run-r1` 未留，数字在这里）：A 63.4/57.5/66.5/57.8/59.5、B 59.0/56.7/59.0/62.7/61.1、C 62.5/65.3/61.1/61.5/63.4，同样是区间振荡、无趋势。

## 四、观察与副作用（都不影响上面的结论，但值得记一笔）

1. **侧栏会显示别的会话的标题**。这是 `session.list` 的正常列表，不是串台判据。如果验收口径是"A 的屏幕上一个 BETA 字都不能有"，那条要改成"消息区/顶栏/状态栏/轨迹栏"。本例里三行标题恰好就长成 `MARK-BETA` / `MARK-GAMMA`，所以特别容易误读。
2. **`curl` + 中文 = 编码陷阱（我的工具链问题，不是产品缺陷）**。第一批 prompt 是在 Git Bash 里 `curl -d "…只回 MARK-ALPHA…"` 发的，落进会话的用户消息变成 `ֻ�� MARK-ALPHA����Ҫ…`；把会话文件解压出来看，字节是 `\xd6\xbb`（GBK 的"只"）后接 `U+FFFD`。原因：Git Bash 调**原生** `curl.exe` 时 argv 走了 CP936 转换（同一条命令里的 `printf` 是干净的 UTF-8）。改用 Python `urllib` 直接发 UTF-8 后同一条会话的第二条用户消息完全正常。**所以 `r1` 数据集里那几条乱码气泡是我造成的**，`r2` 全部改用 Python 发，截图里就没有乱码了。
3. **第一个宿主（`tmp/ms-home`，PID 未记录）在 18:01–18:04 之间被外部终止**：退出码 1、无 panic 栈、日志停在 17:57:09。不是我杀的（我只回收了 `45748`、`40356` 两个我自己的 PID）。`r1` 的证据在它死前已全部落盘，所以不影响结论。
4. **进程内切会话这条路没量到**：界面没有切换热键、侧栏不可脚本化，而任务边界禁止改 `.go`。所以第二条断言是"重启语义的切换"，第三条的进程内部分是"后台会话在流"而不是"切来切去"。要补这块，最小的改动是在壳里加一个 `-switch <id,id,…>` 之类的调试开关（`openSession` 已经存在，只是没有外部触发口）。

## 五、证据清单

- 关键三张：`tmp/ms-run-r2/p1-A-concurrent.png`、`tmp/ms-run-r2/p2-B.png`、`tmp/ms-run-r2/p2-C.png`
- 15 张切换：`tmp/ms-run-r2/p3-{A,B,C}-r{1..5}.png`
- 当前源码重编对照：`tmp/ms-run-r2/head-A.png`、`head-B.png`
- 机读记录：`tmp/ms-run-r2/results.jsonl`、`tmp/ms-run-r2/config.json`
- 客户端日志（含每进程的订阅行）：`tmp/ms-run-r2/logs/p*.log`
- 宿主日志（轮次与工具调用时刻）：`tmp/ms-run-r2/home/logs/dshd-*.log`
- 脚本：`tmp/ms-e2e.py`（全流程，可重跑）
