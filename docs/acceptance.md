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
| **审批（allow/deny）** | 需要模型真的调一个写类工具。`--policy ask` 下这依赖模型的自主决定，不能保证触发。**但**：Round 13 已经用点名工具的 prompt 验过完整闭环（`tmp/h13-allow2.png` / `tmp/h13-deny.png`） |
| **取消（turn 中断）** | 需要一轮足够长的生成；没有可靠的时间窗 |
| **续历史 / 断线恢复** | 要真的杀连接再重连；脚本里做会变慢且不稳。Round 12 单独验过（10 个多会话与重连测试 PASS） |
| **界面** | 这个脚本**只验宿主 HTTP**。界面侧的证据在 `tmp/*.png`（21 个插件齐全）和 e2e（22 PASS） |

## 这一份的定位

**它能证明**：一个新的 dshd 起来之后，认证、会话 CRUD、prompt、四条能力方法
在同一条链路上都能用。

**它不能证明**：界面长什么样、模型每次都会调工具、断线之后能恢复。

**两者不能互相替代。**
