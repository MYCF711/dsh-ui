# 产品级链路验收清单（2026-11-03 18:13）

> 口径：**同一真宿主（`tmp/dshd-serve.exe`，宿主 HEAD 重建）+ 本仓原生界面 + 临时 home**。
> 每一项都写"证据"——没有证据的写「未验」，不拿"代码存在"冒充通过。
> 上表是**本轮**（2026-11-03 下午到 18:13）的实测，不是历史记录。

## 一、逐项状态

| # | 项目 | 状态 | 证据 |
|---|---|---|---|
| 1 | 认证（launch token → cookie） | ✅ | `connect=303`；`tmp/serve-*.log` 每轮都有 |
| 2 | 界面连真宿主（不是 mockd） | ✅ | `已用 launch token 换到 cookie` + 侧栏出现真会话 |
| 3 | list（会话列表） | ✅ | `tmp/real-list2.png`（1 个会话）；`tmp/nc-A.png`（3 个） |
| 4 | load（含尾页） | ✅ | 顶栏 cwd、消息区历史、「共 N 条」 |
| 5 | 发送 prompt | ✅ | `tmp/real-stream2.png`（用户气泡 + 助手回答） |
| 6 | 流式 text | ✅ | 同上，回答完整 |
| 7 | 流式 reasoning（思考块） | ✅ | `tmp/h13-ask.png` 的「思考 · 257 字」；`思考增量 累计 N 字` 日志 |
| 8 | 流式 tool（工具卡 + 轨迹） | ✅ | `tmp/real-tool.png`：`[exit 0] hello-from-tool`、右栏轨迹、「12 轮 · 2 步」 |
| 9 | **真实审批：宿主推帧** | ✅ | `待审批 requestId=perm-887a042f-… tool=execute options=3`（真宿主 M272，`policy=ask`） |
| 10 | **真实审批：允许 → 工具真的执行** | ✅ | `approve 成功 … option=allow_once` → 会话里 `tool(bash) = [exit 0] allow-real-1` |
| 11 | **真实审批：拒绝 → 工具不执行** | ✅ | `approve 成功 … option=reject_once` → `Error: 用户拒绝了 bash 提示` |
| 12 | 审批三选项（allow_once/allow_always/reject_once） | ✅ | `tmp/h13-3btn2.png`：`拒绝 / 这个会话里以后都允许 / 允许这一次` |
| 13 | 审批超时（120s） | ✅ | 复用过期 requestId → `no_such_request（已应答或已超时）` |
| 14 | **取消 turn** | ✅ | `key 4 handled=true` → `cancel 已发送 id=a0eaa207…`（停止键 ■ / 无审批时 Esc） |
| 15 | 多会话不串流 | ✅ | **3 会话**（不止 2 个）：`docs/multi-session.md` —— A 界面不含 BETA/GAMMA；`tmp/ms-run-r2/p1-A-concurrent.png` |
| 16 | 切会话状态隔离 | ✅ | 切到 B 后用量 `17.1k`（A 是 `1110.6k`）（`tmp/nc-B.png`）；3 会话 × 5 次起停，标题/消息区/用量三区 md5 同会话全等、跨会话互不相同 |
| 16b | 反复切会话不涨内存 | ✅ | `docs/multi-session.md`：来回切 15 次 58.9–62.3 MB 振荡（首末 −3.4/−2.3/−0.9 MB）；常驻 5 点 59.0→59.8 后走平 |
| 17 | 断线重连（单元级） | ✅ | `TestReconnect*` / `TestMultiSession*` 10/10 |
| 18 | 断号检测（不整页刷新） | ✅ | `TestMultiSessionSeqGapIsReported`；真机曾经无限重连（已修） |
| 19 | 会话续历史 | ✅ | 重启界面后 `session.load` 带回历史 |
| 20 | 断线恢复（真机） | ⏳ | 子代理验证中，见 `docs/reconnect.md`（未出） |
| 21 | 完整工作日 | ❌ 未验 | 需要连续数小时真实使用 |
| 22 | 同任务 A/B（vs 官方） | ❌ 未验 | — |
| 23 | 整进程树性能 | ✅ | `docs/perf.md`：界面 56.6–79 MB；宿主 serve 空载 11.8 MB；官方 1603.8 MB/9 进程 |

## 二、本轮修掉的真 bug（每一条都有触发条件与根因）

| bug | 症状 | 根因 | 证据 |
|---|---|---|---|
| 入参信封不一致 | `load` 404、`prompt` 子进程参数为空、`list` 却正常 | 界面发 `{"params":{…}}`，宿主解裸参数 | commit `acb34c7` |
| `refreshList` 在 `Connect` 之前 | 界面「0 个会话 / unauthorized」而 `/connect` 成功 | 顺序写反 | `acb34c7` |
| WS 404 → 401 | `/api/mux` 不存在；改对后 401 | 路径旧 + `Mux` 没有 cookie | `acb34c7` |
| 帧形状 | 流一字不进 | 客户端认 `{t:"ev",kind}`，宿主发 `{seq,type,data,time}` | `acb34c7` |
| 审批字段丢失 | 「宿主没给 requestId」但日志里有 | 快照转换只传了 3 个字段 | `201b903` |
| allow_always 被吞 | 3 个选项只画 2 个按钮 | `planApproval` 只取一个 allow 项 | `0de2c8d` |
| ping goroutine 泄漏 | channel **40.01s** 才关 | `defer` 顺序反了（先 Wait 后 stop） | `a3bd655` |
| error 帧污染水位 | `seq 1→5` 报 **0 个 gap** | `seq=0` 被写进 `last` | `a3bd655` |
| 多次会话串流 | B 的内容出现在 A | `applyEvent` 无条件写全局视图 | `a3bd655` |
| 断号无限重连 | 4 秒内重连三次，收不到后续 | gap 后 `break` 重连但水位没变 | `2d4cc1b` |
| 160×28 废图 | 截图只有 895 字节 | 抓图时窗口最小化 | `f4467ee` |

## 三、仍然缺的（不假装有）

1. **完整工作日**：没有连续数小时的真实使用记录。这一步只能真的用，不能模拟。
2. **断线恢复真机**：子代理在验（宿主重启 / 同端口重起），结论未回。
3. **`session.history` 补洞**：宿主仍回 `messages` 投影（§11.6 新基线是 journal `frames`）。界面**故意没接**补洞调用，避免对着两套形状写两次。断号目前只上报、不重放。
4. **审批的 `allow_always` 效果**：按钮与回传都验了（`TestAllowAlwaysGetsItsOwnButton`），但"以后都允许"在宿主侧是否真的记住了，**未验**。
5. **性能里没测到的 10 项**：GPU 显存、帧率、输入延迟、invalidate→重绘耗时、纯滚轮、真模型一轮内存、5 会话内容一致性、宿主整树、高 DPI、258ms 冷启动分解（见 `docs/perf.md` §9）。
