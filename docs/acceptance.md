# 整链验收记录

> 脚本：`scripts/accept-chain.sh`（可复跑）。
> 这一份是**一次真实运行**的记录，不是"应该能通过"的清单。

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
