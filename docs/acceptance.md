# 整链验收记录

> 脚本：`scripts/accept-chain.sh`（可复跑）。
> 这一份是**一次真实运行**的记录，不是"应该能通过"的清单。

## 一条命令跑全部：`scripts/accept-all.sh`

```
bash scripts/accept-all.sh            # 全跑（约 12 分钟）
bash scripts/accept-all.sh --fast     # 跳过要真模型的那几条（约 2 分钟）
bash scripts/accept-all.sh --only theme,session-ops
```

它依次跑 9 条脚本（含守卫反证），最后给一张表。**判据只有退出码**——各脚本自己的汇总格式不一样
（`OK=n FAIL=m` / `PASS=n FAIL=m`），去解析那些字符串只会引入一类新的脆弱。

第一次合跑就抓到三件事，都修在这里面了：

1. **四条脚本的退出码是假的**：`accept-approval` / `accept-cancel` / `accept-resume` /
   `accept-chain` 结尾都是无条件 `exit 0`——**打了 `FAIL=1` 却对调用方说"我过了"**。
   现在 `[ "$fail" -eq 0 ] || exit 1`，并且 `accept-all.sh` 另加一道判据：
   汇总行里 `FAIL=[1-9]` 也算红（防的是同一个坑换个人踩）。
2. **共享的宿主二进制会被没退出的旧宿主锁住**（Windows），锁住之后
   "编不出新二进制"，整个验收停在第一步。现在每条脚本都认 `DSHGO_BIN` 环境变量，
   `accept-all.sh` 每轮编到**自己的目录**里再传进去。
   > 2026-11-04 起产物名统一：宿主一律 `dshgo.exe`、界面一律 **`DSH GUI GO.exe`**
   > （产品名全程、带空格、分大小写；命令行一律加引号），
   > 实例用 `--home` 区分。以前每条脚本各编一个名字（`dshd-acc` / `dshd-goal` /
   > `dshd-term` / `dshd-serve`），34 个进程混在任务管理器里时人分不清谁是谁。
3. **`accept-chain` 的 bin-check 会红**：它验的是共享路径上的二进制，
   而合跑时没人负责先编。现在 `accept-all.sh` 在跑之前编一次，`bin-check` 才有意义
   （它验的是"绝不拿陈旧二进制跑真机"，不是"文件存在"）。
4. **那条 bin-check 一度"在跑、但结论没有意义"**：它调的是 `../DSH-GO/scripts/bin-check.py`，
   而那个脚本的 `SRC_ROOTS`（`cmd` / `internal` / `modules` / `codecs`）是**相对当前目录**扫的。
   在界面仓里跑它，等于拿**界面**的源码去比**宿主**的二进制——前几轮它一直绿，
   只是碰巧 mtime 顺序让它过了。2026-11-04 真红了一次才看出来：
   报「`dshgo.exe` 比 `cmdpp\headless_windows.go` 旧」，那是界面的文件。
   现在改成 `(cd ../DSH-GO && python scripts/bin-check.py "$BIN_ABS")`，
   红了就是真陈旧（实测报出 `cmd\dshd\main.go` 更新，重建后 OK=11 FAIL=0）。

### 进程回收：每条脚本退出时杀掉**自己起的那个宿主**

四条脚本（`approval` / `cancel` / `resume` / `chain`）原来起了 `serve` 就再也没人管它。
异常或 Ctrl+C 之后宿主留成孤儿，下一次跑会撞两件事：**Windows 上它还锁着 exe**
（"编不出新二进制"，验收停在第一步）、以及**它占着旧的 home**（下一轮可能读到别人的会话）。

2026-11-04 现场：`dshd-acc.exe` 积了 **7 个 6 小时前的残留**；宿主侧同一形态查出 **34 个**孤儿
（他们那边已经用 `_proc.py` 连进程树一起收）。

现在每条脚本 `SRV=$!` + `trap cleanup EXIT` + `INT/TERM → exit 130/143`，
**只回收 `$!` 那一个 PID**，不按镜像名扫（按名字杀会连带别人的宿主）。

实测：

```
正常跑完 accept-approval  → 残留 0
中途 kill -TERM           → 第 20 秒回收干净（0）
```

**一个要知道的延迟**：bash 收到 TERM/INT 时会把 trap 推迟到**当前那条命令结束**
（一次 curl 或 sleep 的时长），所以回收可能晚几秒到几十秒。真正跑不到 trap 的是
**硬杀**（Windows `TerminateProcess`）——那种情况靠 `accept-all.sh` 每轮自己编二进制兜底。

### 守卫反证：`scripts/guard-selftest.sh`（在合跑里排第一，**CI 里也跑**）

这条**不需要窗口、不需要模型、不需要宿主** —— 只跑 `go test` + `git archive`，
所以它进了 CI（`.github/workflows/ci.yml` 的「守卫可反证」一步）；
`accept-all.sh` 里另外几条要真桌面的进不了。

### 守卫反证：`scripts/guard-selftest.sh`（在合跑里排第一）

规矩是"**守卫必须可反证**"。但反证一直是**人做的事**：写守卫时手工改一次、看它红、再改回来。
人一旦省了这一步，守卫就退化成一句安慰——而它照样绿，谁也不知道它什么都没看。

第 74 轮真撞到过：那条 Action 接线守卫的第一版只认 `*ast.Ident`，漏掉了仓里最常见的
`env.Act.X`，只扫到 16 个字段（真值 32），**把接线拆掉它也不红**。

这条脚本把"弄坏"自动化：在 `git archive HEAD` 出来的**隔离副本**里，每条守卫各弄坏一处，
要求它当场红。

| 用例 | 怎么弄坏 | 要求谁红 |
|---|---|---|
| 键位表 | 从 `bindings` 里删掉 F5 那一行（F5 在 `dsterminal` 里真的被处理） | `TestEveryHandledKeyIsDocumented` |
| 槽位 Order | 把命令面板改回 `Order: 96`（和交付物面板撞） | `TestNoTwoPanelsShareOneSlotAndOrder` |
| 契约快照 | 从快照的 methods 里删掉 `session.approve` | `TestSnapshotCoversEveryCalledMethod` |
| Action 接线 | 拆掉 `OpenFile: m.openFile` | `TestEveryUsedActionIsWired` |
| **对照组** | 副本原样 | 四组**全部绿** |

最后那行对照组不是陪衬：没有它，把四个守卫全改成"永远失败"也能过前四条 ——
那就从"假绿"翻到了"假红"。

## 怎么跑

```bash
# 先 build 宿主到 dsh-ui/tmp/（脚本会验它与源码一致）
cd ../DSH-GO && CGO_ENABLED=0 go build -o ../dsh-ui/tmp/dshd-acc.exe ./cmd/dshd
cd ../dsh-ui && bash scripts/accept-chain.sh
```

**每次用一个新的临时 home**（`tmp/acc-home-$$`）——第一版写死 `acc-home`，
被上一个没退出的宿主占着，`rm` 失败，于是**每一轮都在同一个旧会话上跑**，
读出来的消息数是别人的。这是这个脚本最值得记住的一处。

## 结果

```
PASS  二进制与源码一致（bin-check）
PASS  ready 行带 url+token
PASS  认证 /connect 拿到 cookie
PASS  session.create 返回 id
PASS  session.list 含新会话
PASS  session.load 有 result
PASS  session.prompt 已受理
PASS  产生对话（≥2 条：user+assistant）
SKIP  工具调用进了消息流（role=tool 且 tool=write）
PASS  schedule.list 可用
PASS  models.list 可用
PASS  jobs.list 可用
----- 合计 OK=11 FAIL=0
```

## 那个 SKIP 是什么

**不是链路坏了，是这次模型没调工具。**

同一个 prompt（"用 write 工具在 acc-demo.txt 里写一行"）在 Round 49 调过，
留下了完整证据（`tmp/deliv-crop.png`：侧栏出现那个文件名）。

**为什么标 SKIP 而不是 PASS**：验收脚本判的是**这一次**的结果。
`grep -q "write"` 那种写法会**假阳性**（匹配到用户消息里的 "write" 这个词）——
这一版改成"必须有一条 `role=tool` 且 `tool=write`"，于是它诚实地报出了
"这次没有"。

**为什么标 SKIP 而不是 FAIL**：模型调不调工具不是这条链路能控制的。
把它算成 FAIL 会让这个脚本**永远红**，而永远红的守卫等于没有。

## 没验的（诚实列）

| 项 | 为什么没验 |
|---|---|
| ~~审批（allow/deny）~~ | ✅ **Round 53 补上了**：`scripts/accept-approval.sh`，5 PASS / 0 FAIL |
| ~~取消（turn 中断）~~ | ✅ **Round 54 补上了**：`scripts/accept-cancel.sh`，4 PASS / 0 FAIL |
| ~~续历史 / 断线恢复~~ | ✅ **Round 55 补上了**：`scripts/accept-resume.sh`，6 PASS / 0 FAIL |
| **界面** | 这个脚本**只验宿主 HTTP**。界面侧的证据在 `tmp/*.png`（21 个插件齐全）和 e2e（22 PASS） |

## 这一份的定位

**它能证明**：一个新的 dshd 起来之后，认证、会话 CRUD、prompt、四条能力方法
在同一条链路上都能用。

**它不能证明**：界面长什么样、模型每次都会调工具、断线之后能恢复。

**两者不能互相替代。**

---

# 审批整链（`scripts/accept-approval.sh`）

> Round 52 把这一项列为"没验"，理由是"依赖模型自主决定"。**Round 53 补上了。**

## 结果

```
PASS  收到 approval/request（requestId 非空）
PASS  approval 帧带选项（optionId 非空）
PASS  session.approve 被接受（allow_once）
（本轮共批准 2 次）
PASS  批准之后写工具真的跑了
PASS  同一个 requestId 不能重复批准（no_such_request）
----- 合计 OK=5 FAIL=0
```

## 脚本自己踩的两个坑（都值得记）

**① 帧类型是 `approval/request`，不是 `approval`**

契约里用的是 **v4 原词**，带斜杠。我第一版按 `type=='approval'` 匹配，
**一次都没命中**，于是轮询整整 80 秒直到审批超时——**而超时的表现是
`tool/result: Error: 用户拒绝了 write`**，看起来像"审批链路坏了"，
实际是**我的判断条件写错了**。

**② 「批准一次」这个假设是错的**

第一版批准一次就去验"写工具跑了吗"，结果 FAIL。

看 journal 才明白：**模型在一个 turn 里发起了两个工具调用**——

```
approval/request → tool/call ls -la appr-demo.txt    ← 第一次批准后跑了
approval/request → tool/call write {appr-demo.txt}   ← 它也要批准
```

**每个工具调用都要单独批。** 改成循环批准之后，`（本轮共批准 2 次）`，
写工具才真的执行。

**这不是链路的问题，是脚本对"一次 turn 有几次审批"的假设错了。**

## 顺带验到的

**`同一个 requestId 不能重复批准`** —— 第二次回同一个 `requestId` 得到
`no_such_request`（404）。这条**幂等保护**是刻意留的：没有它，
一个点两下的界面会把同一个 tool call 执行两次。

---

# 取消整链（`scripts/accept-cancel.sh`）

## 结果

```
PASS  turn 真的开始了（turn/start 帧）
PASS  session.cancel 被接受
PASS  取消后 turn 收尾（done/error/cancelled）
PASS  取消后帧数基本不再增长（3 → 3）
----- 合计 OK=4 FAIL=0
帧序：turn/start user/message turn/error
```

**取消的收尾帧是 `turn/error`**，不是 `turn/done`，也没有单独的 `cancelled` 类型。
这不奇怪：**从消费方看，"这一轮没跑完"就是错误**。但**判据不能只认 `error`**——
把它写成 `done|error|cancelled` 三种都算，是因为**收尾的具体措辞是契约的实现细节**，
而**"这一轮不再产出"才是要验的东西**（第四条正是在验这个）。

## 脚本踩的坑

**① 「发个长 prompt 等它慢慢生成」这个假设是错的**

第一版让模型"从 1 数到 300"，然后 `sleep 2` 再取消。结果：

```
session.cancel → {"error":{"message":"没有这一轮"}}
帧序：turn/start user/message assistant/message turn/end turn/done
```

**模型两秒内就 turn/end 了**（它只回了一条消息就收工）。`cancel` 打在一个
已经结束的 turn 上，得到"没有这一轮"。

**修法不是加长 sleep，是换一个真的会挂住的动作**：

```
"用 bash 工具执行 sleep 30，等它跑完再告诉我结果。"
```

**工具执行期间 turn 会一直挂着**——那才是可中断的窗口。

**② 收到的错误像链路故障，其实是用法错误**

`"没有这一轮"` 读起来像"取消这个功能坏了"，实际是**取消得太晚**。
这和上一轮 `approval/request` 那次一样：**先怀疑自己的用法，再怀疑链路**。

---

# 续历史 / 断线恢复（`scripts/accept-resume.sh`）

## 结果

```
PASS  原连接看到完整历史（8 帧，lastSeq=8）
PASS  历史里有 tool 帧
PASS  重连沿用原 cookie（token 是一次性的）
PASS  历史 append-only（h1 是 h2 的前缀）
PASS  sinceSeq=7 只补回后几帧（不是整段重发）
PASS  两条连接的 journalEpoch 一致
----- 合计 OK=6 FAIL=0
```

## 这个脚本踩了三个坑，每个都像"链路坏了"

**① 想用同一个 token 再换一个 cookie → 401**

第一版模拟重连的写法是"再 `connect` 一次拿新 cookie"。**实查：`/connect` 的 token 是一次性的。**

这符合实际：**重连时浏览器本地那条 cookie 并没有丢，丢的是连接**。
所以脚本改成复制 cookie 文件。

**② `since` 参数被静默忽略，回了全量**

传 `{"since": 7}`，结果**回了全部 8 帧**。查契约发现参数名是 **`sinceSeq`**。

**这种错最难发现**：它不报错，只是**行为不对**——如果不是专门验"只补回几帧"，
会一直以为补洞是好的（因为它确实给了你要的数据，只是给多了）。

**③ 拿两个不同时间点的快照比相等**

第一版判据是"h1 的帧序 == h2 的帧序"，结果 FAIL。

**但 h1 是轮询期间抓的**（那时 turn 还没完，7 帧），**h2 是之后抓的**（10 帧）。
**两个时间点的快照本来就不该相等。**

正确判据是 **h2 以 h1 为前缀** —— 这恰恰是 **journal append-only 的直接推论**，
而且失败了就说明真的有人改写了历史。

## 这一条验到的

| 事实 | 靠什么验的 |
|---|---|
| `/connect` 的 token 一次性 | 第二次 connect 得 401 |
| 历史只增不改（append-only） | h1 是 h2 的前缀 |
| `sinceSeq` 真的按水位补洞 | `sinceSeq=7` 只回 seq 8 之后的帧 |
| 两条连接看同一份 journal | `journalEpoch` 相等 |
