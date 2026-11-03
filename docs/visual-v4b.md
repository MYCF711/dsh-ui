# V4B · 壳右栏状态证据

- 日期：2026-11-03（Asia/Shanghai）
- 任务 base：`bf7e8885a73a21d839d46c0684eab41d6e20f5e1`
- 无头验证时 HEAD：`7597ab78d9ca`。并发提交 `66ec07d` 在加入 OnScroll 时已把工作区中的本次 `main.go` 右栏实现一并纳入；本 owner 未执行提交、重置或覆盖。
- 本 owner 改动范围：`cmd/app/main.go`、`cmd/app/main_test.go`、本文件。未改插件或共享文档。

## 实现与断言

1. `-rightbar` 普通默认从 `true` 改为 `false`；无 `role=tool` 消息时右栏默认关闭。
2. `refreshSession` 收到真实 `role=tool` 轨迹后自动展开；`openSession` 和流消息归约后重新同步当前会话状态。
3. 用户开合选择按 session id 保存。自动状态不跨会话；刷新不覆盖用户明确选择；切回恢复原会话选择。
4. 未改三栏视觉布局或右栏宽度公式。`total=1180`、`sidebar=280` 时：关闭右栏宽度 0、主区 900；打开右栏宽度 500、主区 400。

## 先红后绿

实现前新增测试并运行：

```bash
CGO_ENABLED=0 go test ./cmd/app -run Rightbar -count=1
```

退出码 **1**。实际失败：

- `TestRightbarDefaultsClosedWithoutToolTrajectory`：无工具会话加载后右栏仍打开。
- `TestRightbarOpensWithToolTrajectory`：真实 `role=tool` 轨迹加载后未自动打开。
- 几何对照项通过。

实现后：

```bash
CGO_ENABLED=0 go test ./cmd/app -run Rightbar -count=1
# ok github.com/MYCF711/dsh-ui/cmd/app；退出码 0

CGO_ENABLED=0 go test ./cmd/app -count=1
# ok github.com/MYCF711/dsh-ui/cmd/app；退出码 0
```

覆盖空态默认、真实 tool 消息自动展开、用户逐会话选择与切回不串、开合主区几何。

### 最终并发复核状态

上述完整绿灯取得后，其他 owner 继续并发修改插件。再次运行指定命令时先退出 **1**，错误为 `dscomposer/composer.go` 的未使用 `components` 导入与 `undefined: cardBorder`；该文件被其 owner 修正后复跑，又退出 **1**，错误转为 `dsconversation/conversation.go:97` 的 `undefined: heroBoxes` 与 `hero.go:110` 的 `undefined: heroStackMax`。两处均是本任务禁止修改的插件中间态，不是 V4B 测试断言失败；已通知 Lead 协调对应 owner。

插件 owner 收尾后，最终再跑 `CGO_ENABLED=0 go test ./cmd/app`：`ok github.com/MYCF711/dsh-ui/cmd/app 0.103s`，退出码 **0**。因此最终指定验收通过，上述两次失败只作并发过程记录。

## 无头真实 v4 记录验证

构建（退出码 **0**）：

```bash
CGO_ENABLED=0 go build -o /tmp/dsh-ui-v4b/app-v4b.exe ./cmd/app
CGO_ENABLED=0 go build -o /tmp/dsh-ui-v4b/mockd-v4b.exe ./cmd/mockd
```

只读加载现有真实 v4 记录，OS 分配端口；未使用 `-demo`，未启动真实模型：

```bash
/tmp/dsh-ui-v4b/mockd-v4b.exe -addr 127.0.0.1:0 \
  -sessions "D:/DeepSeek Harness/default-workspace/dsh-ui/tmp/ms-run-r2/home/sessions" -tail 40
# listening on http://127.0.0.1:62707；3 个真实会话
```

Transport 数据先核验：A=`ad76b706-73f2-fc61-e012-32722aa8b908`，2 条，user/assistant，各 1 条，**0 个 tool**；B=`06d66e0c-599d-af12-23a0-067f50dfa229`，14 条，含 **6 个真实 bash tool**。

四次界面命令均以 `-size 1180x800` 运行并退出 **0**；PNG 外框均为 **1196×839**。为遵守只允许新增本证据文件的限制，PNG 放在系统临时目录，未新增仓库文件。

| 场景 | 实际画面断言 | 临时证据 / SHA-256 |
|---|---|---|
| 直接打开 A | 主区占满右栏位置，无“轨迹”栏 | [empty.png](C:/Users/Administrator/AppData/Local/Temp/dsh-ui-v4b/empty.png) · `8ac32617ba81ffec4763b48249bc14fc5275a10f3ddad3d1eb30369321cf2023` |
| 直接打开 B | 自动出现“轨迹 6 次”及 6 个 bash 节点；主区让出右栏 | [tools.png](C:/Users/Administrator/AppData/Local/Temp/dsh-ui-v4b/tools.png) · `2116505cba3f9de45a2a8d31556cdfa79a6b05f65aac904b4f89e58ba2afd4a5` |
| 同进程 B→A | 日志确认终态 A；右栏消失，不残留 B 的轨迹 | [switch-empty.png](C:/Users/Administrator/AppData/Local/Temp/dsh-ui-v4b/switch-empty.png) · `4808263e785d5e160dd614a54e7fbec908bf69b2594f6bc49b24a4946a35117b` |
| 同进程 A→B | 日志确认终态 B；自动显示 B 的 6 次 bash 轨迹 | [switch-tools.png](C:/Users/Administrator/AppData/Local/Temp/dsh-ui-v4b/switch-tools.png) · `a02b9bc25a085d52f5afee740c0d65100b5f8f7bfc29fc3c23b1e9bdbd809f48` |

切换命令使用现有 `-switch <id> -switch-every 2 -shot-wait 7`；日志均有 `switch 结束后当前会话: <目标 id>`。抓图后自建 mockd job `bash-0a75a85e64534f62` 已回收，未杀其他进程。

注意：mockd 订阅后会向画面追加其合成的“假事件”文本；这里的右栏断言只依据上述真实 v4 `session.load` 数据和可见 tool 轨迹，不把合成文本当作真实生成证据。

## 未验证

- 未验真实 dshd + 真模型**实时产生** tool 事件当刻的自动展开；本次自动展开证据是真实 v4 历史经 mockd 的 `session.load`，不是 live generation。
- 未验 OS 真实鼠标点击顶栏“轨迹”按钮；用户开合与逐会话隔离由 `toggleRightbar` 单测覆盖。
- 未做其他视觉部件、其他 Transport 链路或真实审批回归；本任务未改这些实现。
