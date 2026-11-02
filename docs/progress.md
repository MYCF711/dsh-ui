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
