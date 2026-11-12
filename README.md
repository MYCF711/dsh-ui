# DSH GUI GO · 界面线（独立仓库）

> **这是 dsh-go 的界面线，不是宿主。** 宿主在 `../DSH-GO`（`PLAN.md`），界面在这里。
> 两条线只在一个地方握手：**Transport 契约**（HTTP `/api` + WS `remote.mux` + `ViewSpec`）。
> 本仓的第一件事不是写产品，是 **3 天 spike**——见 `PLAN.md`。
>
> **产品名是 `DSH GUI GO`，产物是 `DSH GUI GO.exe`**（`cmd/app/brand.go` 一根常量）。
> 窗口标题、内核名、无头截图认窗都引用它；`-title` 仍可覆盖，用于并行多实例。
> 名字是**全程**、分大小写、带空格（用户口径：「exe 要写产品名，而且要全程」）——
> 所以所有命令行形态都必须**整体加引号**。

---

## 现在到哪了（2026-11-03 深夜）

| 阶段 | 状态 | 证据 |
|---|---|---|
| **spike E1/E2/E3** | ✅ 通过 | `docs/e1-result.md`、`docs/e2-result.md`、`docs/verdict.md` |
| **U0 Transport 契约** | ✅ 13 个 method + `/api/mux` + 快照 | `testdata/api.snapshot.json`、`docs/api-contract.md` |
| **U1 最小界面** | ✅ 能聊、能看历史、能续会话 | 见下 |

U1 现在有的：真实会话列表（180 个）、Markdown 全要素（标题/粗体/列表/引用/代码块/表格）、
工具调用卡片、审批气泡（先命令后模型自述）、逐字流式、会话切换记住滚动位置、视口虚拟化。

**关键数字（实测）**：**72.5 MB / 单进程 / 0% 空闲 CPU / 698 ms 冷启动 / 4.02 MB 二进制**。
对照：官方 DSH **1,603.8 MB / 9 进程 / 2.0% 空闲 CPU**。

### 怎么跑

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
export GOPROXY=https://goproxy.cn,direct

# 1) 只读真实会话（假回答）
go run ./cmd/mockd -sessions "D:/DeepSeek Harness/home/sessions"

# 2) 接真模型（走 dshd acp）
go run ./cmd/mockd -sessions "D:/DeepSeek Harness/home/sessions"     -acp "../DSH-GO/dshd.exe" -acp-manifest tmp/dsh-acp.toml

# 3) 界面
go run ./cmd/app            # 或 go run ./cmd/app -tail 40 -demo

# 4) 构建产物叫 DSH GUI GO.exe，先认身份再跑（只打一行就退出，不连宿主）
go build -o "tmp/DSH GUI GO.exe" ./cmd/app && "./tmp/DSH GUI GO.exe" --version   # DSH GUI GO dev
```

### 一条命令复跑验收

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke-u1.ps1
```

7 项自动检查（mockd 起没起 / 窗口多快出来 / 内存 / 线程 / 点击切换 / 切换后内存），
**最新一次 7 pass 0 fail**：`mockd 624 ms` · `窗口 304 ms` · `65.5 MB` · `34 线程`。

滚动手感和"滚动位置回没回原位"它验不了——那必须人看。

### 这不是终局

| 现在的样子 | 该有的样子 |
|---|---|
| `mockd` 是过渡管（伪造 `/api` 再转发 ACP） | U0 的 `internal/transport` 直连真宿主 |
| 续会话靠**复制副本**（每次点开重新复制） | home 只有一个 |
| 没有搜索 / 过滤 / 侧栏折叠 / 多窗口 | U1 之后的活 |

---

## 为什么单独一个仓库

| 理由 | 展开 |
|---|---|
| AGENTS 已经分了两条线 | `DSH-GO/AGENTS.md` 开头就写了"宿主线 / 界面线"，分支会把这条线缝回去 |
| CI 互不干扰 | 宿主跑 `scripts/check.sh` + `go test -race`；本仓跑 `viva` 构建 + 截图验收，两套门禁不能混在一起 |
| 宿主的 main 一直在动 | 宿主正在 M60/M61 快速推进，UI 挂在分支上会天天解冲突 |
| 失败时丢掉成本低 | spike 不通过，删掉这个目录就完事，不污染宿主历史 |

**远程仓库**：`MYCF711/dsh-ui`（私有）。**但只在 spike 通过后才推**——先本地，避免建一个空仓库。

---

## 押注逻辑（用户 2026-11-03 定的方向）

> 「在这种过渡阶段，需要一定的风险投资，投资最有潜力的语言，而你的试错成本只不过是几天时间而已。他们不做 win，我们为什么不做？他们的经验我们可以借鉴啊。」

- **押注对象**：Go 的 GPU 原生 UI（Viva / MyGo 一族），2026-10 才出现，Windows 侧还是 Experimental。
- **赌注**：3 天。
- **赢了**：界面内存 ~50 MB（没有浏览器内核）、插件走 WASM（任何语言能写）、需要网页时再挂系统 WebView。
- **输了**：退回 WebView 路线（`DSH-GO/UIPLAN.md` §2.8 全部留着，一行没删）。

**保留 Web 接口不是骑墙，是选项**：一个 Transport 接口只占存储，不占运行时；等原生生态长起来，把 Web 插件通道关掉就是一行配置的事。

---

## 目录

```
dsh-ui/
├── PLAN.md          ← spike 方案（先读这个）
├── README.md        ← 本文件
├── docs/            ← 实测记录、截图、判定
├── spike/           ← 三个实验的代码（临时，可丢）
└── （通过后才建）app/ ← 真正的 U1 最小界面
```

---

## 红线（继承宿主线，不重议）

1. **不重启、不关闭正在运行的 DSH**；不写 `D:/DeepSeek Harness/home/profiles/`。
2. **asar 与存量会话只读**。
3. 界面**不得**直接读会话日志（`UIPLAN.md` R7 / U-R1）。
4. **每个数字都要实测**，估算必须标"估算"。
5. **没跑过的写"未验证"，不写"通过"**。
