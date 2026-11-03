# 进度

| 时间 | 做了什么 | 结论 |
|---|---|---|
| 2026-11-03 05:19 | 建仓、写 README/PLAN、git init | 方案定稿 |
| 2026-11-03 05:20 | `go install viva@v0.2.3`（18 s）、`viva doctor` | 环境完备，无需 cgo |
| 2026-11-03 05:21 | `viva init hello` + `viva build .`（3.6 s，**CGO_ENABLED=0**） | **3.92 MiB exe，E1 通过** |
| 2026-11-03 05:22 | 起窗口量内存（1 进程 / 67.2 MB / 28 线程） | 比目标 60 MB 高，E3 要复查 |
| 2026-11-03 05:24 | E2 程序渲染 7 项矩阵，起窗口截图 | 1/2/3/4/6 通过；IME 与缩放待人工；1000 条 90.2 MB |
| 2026-11-03 05:25 | 裁剪窗口区域存 `e2-window.png` | 中文断行、emoji 彩色、等宽对齐全部正常 |
| 2026-11-03 05:31 | 字形覆盖矩阵（charmap）：ASCII/CJK/日韩/符号/emoji 逐项核过 | 全通过；反引号是误判，已撤回 |
| 2026-11-03 05:33 | 冷启动 10 次 | 中位 **171 ms** |
| 2026-11-03 05:34 | 空闲/滚动内存 + 静置 CPU | 75.5 / 84.2 MB；静置 **0.0% CPU** |
| 2026-11-03 05:35 | GOGC 100/50/20/10 四档 | 只差 5 MB → 内存不是 Go 堆，是图形栈 |
| 2026-11-03 05:38 | 用户实测 IME：「可以输入删除光标点选插入」 | **E2 第 5 项通过** |
| 2026-11-03 05:38 | 三份文档成稿 + 判定 | **走原生**，附三条件 |
| 2026-11-03 05:48 | U0 契约落盘：`testdata/api.snapshot.json`（13 method = 8 第一批 + 5 第二批）+ `docs/api-contract.md` | 契约定稿 v1 |
| 2026-11-03 05:50 | `cmd/mockd` 假宿主（标准库，13 method） | 7 个冒烟点全过：list/load/tail/空text 400/未知 404/正常 prompt/重复 prompt 409 busy |
| 2026-11-03 06:00 | `internal/apiclient`（13 method 的客户端，只依赖 net/http） | 可编译 |
| 2026-11-03 06:05 | `cmd/app` 第一版：Viva 窗口连 mockd，左栏会话 / 右栏消息 / 输入框 | 真实拉到 2 个会话，**59.7 MB** |
| 2026-11-03 06:10 | 修：会话项改 Row+Hit 左对齐、启动自动选第一个会话 | 40 条消息渲染正常 |
| 2026-11-03 06:20 | mockd 加 s-003（1000 条）；app 加 `-limit` / `-session` / `-tail` 三个量测参数 | 可测三档 |
| 2026-11-03 06:24 | 1000 条三档实测：全渲染 71.8 / 只渲染 60 → **60.8** / 只渲染 300 → 62.2 MB | **虚拟化收益 11.0 MB**，估算验证 |
| 2026-11-03 05:36 | **U0 流**：`cmd/mockd` 加 `/api/mux`（WS，200ms 一条 ev，第 7 条后跳号发 gap）；新建 `internal/apiclient/mux.go`（`Mux.Subscribe` + 连续性检查 + gap 兜底 + 应用层 ping/pong） | `CGO_ENABLED=0 go build ./...` ✅ / `go vet` ✅ / **`go test -run TestMuxLive -v` PASS 3.45s：ev=10 gap=1 区间[108,109] 末 seq=112** |

<details><summary>TestMuxLive 自测输出（2026-11-03 05:36）</summary>

```
--- 订阅 session:s-001  sinceSeq=100 ---
  EV   seq=101 kind=user/message       data={"text":"第 101 条假事件（topic=session:s-001）"}
  EV   seq=102 kind=assistant/message  data={"text":"第 102 条假事件（topic=session:s-001）"}
  EV   seq=103 kind=user/message       data={"text":"第 103 条假事件（topic=session:s-001）"}
  EV   seq=104 kind=assistant/message  data={"text":"第 104 条假事件（topic=session:s-001）"}
  EV   seq=105 kind=user/message       data={"text":"第 105 条假事件（topic=session:s-001）"}
  EV   seq=106 kind=assistant/message  data={"text":"第 106 条假事件（topic=session:s-001）"}
  EV   seq=107 kind=user/message       data={"text":"第 107 条假事件（topic=session:s-001）"}
  GAP  from=108 to=109 reason="mockd: 故意跳号，客户端应去 session.history 补洞"
  EV   seq=110 kind=assistant/message  data={"text":"第 110 条假事件（topic=session:s-001）"}
  EV   seq=111 kind=user/message       data={"text":"第 111 条假事件（topic=session:s-001）"}
  EV   seq=112 kind=assistant/message  data={"text":"第 112 条假事件（topic=session:s-001）"}

--- 小结：ev=10  gap=1  gap区间=[108,109]  最后 seq=112 ---
--- PASS: TestMuxLive (3.45s)
--- PASS: TestMuxBadBase (0.00s)
```
</details>
| 2026-11-03 06:45 | 并行三路：Viva API 调研（`docs/viva-api-notes.md`）、WS mux 客户端+服务端（`internal/apiclient/mux.go`，10 ev + 1 gap 测试过）、窗口缩放三档（`docs/e2-scale.md`） | 全部完成 |
| 2026-11-03 06:50 | **Enter 发送接通**（`Box.Submitted()`，不是 `Field.Submitted()`）；**会话温缓存**（LRU 5 个，切回来先显示缓存再后台刷新）；**MinHeight 修 Grow 塌陷**；`Config.MinWidth/MinHeight` 760×520 | `go build`/`go vet`/`mux_test` 全绿 |
| 2026-11-03 06:48 | **smoke 端到端**：界面 `send()` → `session.prompt` → mockd，会话总数 **40 → 41**，末条 `41 user smoke-053642` | 整条链路通 |
| 2026-11-03 07:05 | 流式接进界面后发现两个 bug：①把 mockd 的背景心跳当 delta 吃进消息（内存涨到 81MB）②列表不跟随底部 | 删 text 分支 + `Follow(true)` → 57.4MB / CPU 0.23s |
| 2026-11-03 07:12 | 「新建」按钮真的建会话（`session.create` → 刷新列表 → 打开新会话） | build+vet 绿 |
| 2026-11-03 07:40 | Markdown 验证程序（spike/md）：标题/粗斜体/行内代码/列表/引用/代码块/表格/未闭合粗体 **全部正确** | 4.02 MB exe |
| 2026-11-03 07:50 | 助手消息接入 `components.MarkdownView`（Slot 占位 + Draw 后按 Rect 绘制） | 58.3 MB，三句合一段渲染正确 |
| 2026-11-03 08:10 | mockd 回答改成 Markdown（标题/行内样式/列表/代码块），走完「prompt → 60ms/字 → MarkdownView 边收边渲染」 | 67.3 MB，五项全部正确渲染 |
| 2026-11-03 08:45 | **mockd 能读真实会话**（`-sessions <dir>`）：zstd 多帧解码 178 个文件，0 个坏；`session.list` 列真实会话、`session.load` 拿真实消息；界面显示真数据（117 MB） | 解码 6.1 s，截图 `docs/u1-real-sessions.png` |
| 2026-11-03 08:55 | mockd 读到了**真实会话**（agent 贡献），界面显示真实标题与内容 | 111.2 MB ← 性能问题暴露 |
| 2026-11-03 09:02 | **修掉 MarkdownView 每帧重排**：缓存 `text+width → height`，只在变化时 Layout | **111.2 → 50.1 MB，CPU 0.2s 不再增长** |
| 2026-11-03 08:50 | 工具调用卡片（名字 + 参数 + 结果摘要），demo 模式可注入两条验证 | 渲染正确 |
| 2026-11-03 08:45 | 会话切换记住滚动位置（只存 float32，不存指针） | 待真机多次切换验证 |
| 2026-11-03 09:45 | 代码块横向溢出：新增 `internal/mdlite/overflow.go`（封装 components.ScrollArea），`spike/e2` 接上 | 700×640 实测：滚动条出现 / offset=0 与 max 渲染不同（md5 4aa401dc vs b671ad3c）/ 99999 被夹紧到 max |
| 2026-11-03 09:25 | 首启动 6.1s → **698 ms**（mockd 索引期只解 header，load 时才全量解） | 8.7 倍 |
| 2026-11-03 09:40 | 内存四档实测：tail=30/60/100/200 → 94.6/111.7/128.4/172.9 MB | **线性：80.8 基线 + 0.46 MB/条**；三个嫌疑全排除，剩下的是文本纹理 |
| 2026-11-03 09:15 | 代码块横向滚动（agent 贡献，`internal/mdlite/overflow.go`）：三次平移 md5 对比 + 夹紧验证 | 700px 下不再硬切 |
| 2026-11-03 09:55 | 默认 `-tail` 200→40：**172.9 → 99.1 MB**（按实测 0.46 MB/条） | 立即可用的解 |
| 2026-11-03 10:04 | **审批气泡**：先命令、后模型自述（标注不可信），拒绝/允许两个按钮 | 截图 `u1-approval2.png` |
| 2026-11-03 10:02 | 踩到并修掉一个自死锁：demo 注入写在 `refreshSession` 的锁内又 `Lock()` | 白屏「未响应」→ 正常 |
| 2026-11-03 10:05 | 视口虚拟化原型 `spike/virt/`：500 条三档实测 94.2 / 58.5 / 60.4 MB，CPU 1.98 / 0.14 / 0.17 s | 降 38% 内存、93% CPU；**滚动不跳**；估算系数 1.0→0.62 消掉留白 |
| 2026-11-03 10:44 | **视口虚拟化接进界面**（`estimateMsgHeight` + 视口判断 + 视口外只 `f.Slot`） | **99.1 → 72.5 MB，CPU 1.31 → 0.31 s** |
| 2026-11-03 10:42 | 拿到的工作区改动里有一处 `"\n"` 被写成真换行（`strings.Split`），已修 | 之前那次 build 红是中间态 |
| 2026-11-03 10:45 | ⚠️ 发现会话标题显示成「会话 session-xxxx」——`mockd.firstUserLine` 取错了行 | 待修 |
| 2026-11-03 10:36 | **虚拟化接进 U1**：固定高度盒子 + 视口判断，视口外只放 Slot | 200 条 **172.9 → 80.8 MB**，CPU 4.16→0.47s |
| 2026-11-03 10:40 | 四档曲线：40/100/200/400 → 72.2/77.3/80.8/86.8 MB | **亚线性**：10 倍消息只涨 14.6 MB |
| 2026-11-03 12:05 | **接上真 dshd（ACP）**：initialize/session-new/session-prompt 全通，工具调用与审批帧都出来 | 真模型回答「2。」与「输出就是 hello-dsh-go。」 |
| 2026-11-03 12:05 | 三个坑：manifest 多列模块 / 权限 id 必须原样回 / busy 没人清 | 全部修掉，见 `docs/acp.md` |
| 2026-11-03 06:25 | **真 ACP 打通**：界面 → mockd `-acp` → dshd → 真模型 → 流式回界面。实测问「只回答两个字：收到」→ 回 **收到**（不是 mock 的 Markdown 样板） | 端到端 |
| 2026-11-03 06:22 | 踩到 PowerShell `Start-Process -ArgumentList` 不自动加引号：含空格路径被截断（`D:/DeepSeek Harness` → `D:/DeepSeek`），回落假数据且 ACP 没起 | 手动加引号后正常 |
| 2026-11-03 06:32 | 修会话标题：懒解码下 `Messages` 为空 → 回落到 cwd 最后一段（"DSH-GO" / "长期记忆系统" / "GOkifu上下文压缩"） | 不再是「会话 session-xxxx」 |
| 2026-11-03 06:34 | 界面连真 ACP 跑起来：**75.5 MB**，左栏真实标题，右侧真实内容 + Markdown | |
| 2026-11-03 07:00 | 写 `session/resume` 续会话：客户端 + 缓存都有了，但**跑不通**——dshd 用的是 `tmp/dshd-home`，真实会话在 `D:/DeepSeek Harness/home` | 根因与三条出路写进 `docs/acp.md` |
| 2026-11-03 06:58 | 顺带发现：`dshd.exe`（03:45）比 `internal/acp/server.go`（05:26）旧 —— 旧二进制要求 resume 必须带 cwd，源码已经改成可选 | 客户端补了 cwd 兜底 |
| 2026-11-03 07:40 | **续会话跑通**：resume 前把会话复制进 dshd 的 home（复刻 `projectKey`/`encodeSegment`）。实测模型带着历史上下文回答（提到"迁移脚本"），真实文件 total 仍是 16 没变 | 写进 `docs/acp.md` |
| 2026-11-03 07:52 | U1 验收清单落盘 `docs/u1-checklist.md`（六组 42 条 + 9 条「已知不能做」）：每条都是能当场做的动作 + 具体判据，数字分开标「实测 / 预期」 | 只写文档，没碰 .go |
| 2026-11-03 09:20 | **修侧栏标题竖排**：`ScrollArea` 不给宽度时内容是"最大子元素宽"，而子元素宽度又依赖父 → 互相等 → 宽度 0，标题被压成一个字一行。`f.ScrollArea(...).Width(240)` 钉死后正常 | 截图 `u1-sidebar3.png` |
| 2026-11-03 09:05 | 侧栏加搜索框（按标题 + cwd 实时过滤） | |
| 2026-11-03 09:15 | 顺带修 `Hit(id, s.Title)` 与 `f.Text(...)` 重复画标题的问题（标签留空只当点击区） | |
| 2026-11-03 09:42 | 会话标题从 cwd 换成**真实第一句话**：懒解码读前 64KB（够 header + 第一条 user 消息）。真标题从 0 → **180/182**；启动 966 ms（+270 ms） | 例：「持续调研 AI / Agent / AGI / 人类等载体的「记忆…」」 |
| 2026-11-03 09:52 | **接进代码块横向滚动**（`MarkdownView.Codes()` 拿位置和原文，`mdlite.Overflow` 重画）：超宽代码不再被硬切 | `u1-code-scroll.png` |
| 2026-11-03 10:30 | **修消息区溢出到侧栏**：`ScrollArea` 用 `Grow(1)` 时内容宽度算不出来，user 消息的 `f.Wrap` 直接画到 x=0。改成 `.Width(f.Bounds().Width-278)` 钉死 + `Clip()` | `u1-width.png`，界面终于对了 |
| 2026-11-03 10:20 | 踩坑记录：`heredoc`/python 拼 Go 源码时 `"\n"` 被变成真换行**四次**了。现在源码里用 `const newline = "\n"` 一个名字，不再写字面量 | 编译红 4 次 |
| 2026-11-03 11:50 | 超宽代码块：试了方案 A（`Codes()` 拿位置 + `Fill` 盖住 + `Overflow` 重画），**Fill 盖不住**（MarkdownView 绘制顺序不受调用方控制），两层文字重叠 → **回退**。实测结论：MarkdownView 对 129 字符的单行代码是**折行**不是硬切，清单 3.3 那条是误判 | `docs/u1-codeblock.md` + `u1-codeblock.png` |
| 2026-11-03 09:58 | **纠正一个误判**：清单 3.3 说「60+ 字符单行代码块会被硬切」——实测**本来就折行**，内容不丢。`Codes()` 位置数据可用、但在 `MarkdownView.Paint` 上叠画走不通（绘制顺序不受控，Fill 盖不住） | 结论写进 `docs/u1-codeblock.md` |
| 2026-11-03 10:15 | U1 验收实测：mockd 冷启 **915 ms**、app 到窗口 **327 ms**、**69.5 MB / 38 线程 / CPU 0.17 s**、182 个会话里 **180 个标题是真实第一句话** | `docs/u1-acceptance-report.md` |
| 2026-11-03 10:30 | **语法高亮**：`internal/mdlite/highlight.go` 通用词法高亮（关键字/字符串/注释/数字/函数名，Go/Py/JS/TS/Bash/Rust 关键字表），接进 `MarkdownStyle.Highlight` | 截图 `u1-highlight.png`，代码块有颜色 |
| 2026-11-03 10:20 | **语法高亮**（K8）：`internal/mdlite/highlight.go` 手写逐行扫描（注释/字符串/数字/关键字/函数调用），6 条测试全绿。接进 `MarkdownStyle.Highlight` 后**不生效**——`MarkdownLookFor` 不设 `Code` 数组，所有 class 都被画成正文色；补了深色主题六个颜色才看得见 | 截图 `u1-highlight.png`：`func`/`int`/`string`/`return`/`nil` 蓝、函数名黄 |
| 2026-11-03 11:00 | 最小尺寸实测（`SetWindowPos` 776×559）：布局没塌，但**右边缘有内容溢出**（新 bug B1） | `docs/u1-min-size.png` |
| 2026-11-03 11:55 | **更正 B1**：右边缘的"溢出"是背景里的另一个窗口（`GetWindowRect` 算进了不可见边框），不是我们画的。只有 resize 后的底部残留是真的 | 这已是第二次把"没验证的观察"当 bug 记（第一次是清单 3.3） |
| 2026-11-03 11:35 | **点击切换实测通过**：模拟鼠标点 4 次（1→2→3→回1），会话正确切换、内存 71.7 MB 不涨、CPU 0.34 s | `docs/u1-click.png` |
| 2026-11-03 12:15 | 三项交互用 `mouse_event` 补验：点击切换 ✅ / 流式中途切换 ✅ / 审批点「允许」✅（气泡消失）。**按钮命中区偏小**（第一次差 7px 没点到） | `docs/u1-approve-click2.png` |
| 2026-11-03 12:50 | **`scripts/smoke-u1.ps1`**：一条命令跑完 7 项自动验收（mockd 起速 / 窗口起速 / 内存 / 线程 / 点击切换 / 切换后内存）。最新 **7 pass 0 fail**：`624 ms` · `304 ms` · `65.5 MB` · `34 线程` | 踩到两个坑：PowerShell 5.1 读 UTF-8 要 BOM；`Get-Process` 存在 ≠ 窗口出现（先报 51 ms，改成等 `MainWindowHandle` 后是 304 ms） |
| 2026-11-03 13:30 | 写 `docs/u0-handoff.md`：界面线对宿主侧 U0 的完整要求（13 method 的语义、WS 帧形状、三条硬要求、三个我们踩过的坑、验收线） | 给写 `internal/transport/` 的人看 |
| 2026-11-03 15:56 | 写 `docs/u0-list-fastpath.md`：宿主线 `session.list` 4.37s 的解法（我这边实测 6.1s→624ms）。含 `headerOnly` 实现、为什么是 64KB、判坏阈值、两条没做的边界 | 宿主线已经在做 U0 了（`AGENTS.md` 记了 4.37s） |
| 2026-11-03 16:26 | 宿主线把 `session.list` 做到 **0.23 s 且标题还在**（`ReadHeader` + `ReadSummary` 两步），比我给的"解 64KB 一次拿完"更干净。更新 `u0-list-fastpath.md` 记下对比与他们的一个真 bug（`firstText(整帧)` → 全退占位符，旧断言 `Title != ""` 恰好被占位符骗过） | 我的方案只在"不想改两处读函数"时更省事 |
| 2026-11-03 12:25 | **界面线的架构落地：插件内核 + 根本依赖插件**。新增 `internal/pluginapi`（契约，零实现）、`internal/uikernel`（作用域 / 能力表 / 拓扑装配，缺依赖即报错）、`plugins/dscore`（照抄官方 `--dsw-*` 令牌 + 基础组件）、`plugins/dssidebar`（**第一个**移植的部件：按 cwd 工作区分组、可折叠、搜索、相对时间、选中态）。壳只做三件事：装插件、给槽位、递快照 | `docs/p1-sidebar.png`；装配日志 `[ds-core ds-sidebar]`；**65.6 MB** |
| 2026-11-03 12:35 | **五个插件全部落地并跑起来**：`ds-core`（根）、`ds-sidebar`、`ds-conversation`（消息流/工具卡/审批，含 AfterDraw 两阶段绘制）、`ds-composer`、`ds-statusbar`。主区宽度必须钉死（`Width(mainW)`），否则插件里百分比按整窗解析、气泡被推到窗口外；状态栏必须「占位行 + 绝对定位」两步，否则被 Grow(1) 的主体挤成 0 高 | `docs/p3-plugins.png`；**68.2 MB** |
