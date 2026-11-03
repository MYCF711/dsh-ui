# AGENTS.md · dsh-ui

> 用 Go 重写 DSH 的**界面线**。宿主在 `../DSH-GO`（那份 `AGENTS.md` 管宿主，本文件管这里）。
> 目标态：**原生**（把 Electron/Chromium/npm 那套踹开）；**Web 兼容层是可拆卸的过渡件**。
> 上游依据：`../DSH-GO/UIPLAN.md` §2.9 / §2.9.1 / §2.10。

---

## 0. 这条线的驱动方式（用户 2026-11-03 定的规则）

> **「goal 随便你自己写。定时任务和 goal 是为了唤醒你继续这个项目，不用给我汇报。」**

翻译成可执行的三条：

| # | 规则 |
|---|---|
| 1 | **goal 由我自己写、自己改、自己续**。不必等用户点头，也不必把 goal 当成"向用户交差"的凭证 |
| 2 | **定时任务的唯一用途是唤醒我继续干活**。每轮直接做，不做"给用户汇报"的简报任务；用户要看时自己会问 |
| 3 | 只有三种情况值得打断用户：**要他拍板**、**要他动手**（真机测输入法/拖窗口这类我做不到的）、**真卡死了** |

---

## 1. 现状（2026-11-03）

**架构：一切皆插件。** 壳 `cmd/app` 只做三件事——装配插件、给槽位、递一帧快照；
七个插件挂在它上面，装配顺序由 `Requires` 拓扑排序决定，缺一个键**启动即失败**。

| 插件 | 移植的官方部件 | 说明 |
|---|---|---|
| `ds-core` | `dsh-client-ui-primitives`（打包在 `index-*.js` 里） | **根本依赖插件**：照抄官方 `--dsw-static-*` / `--dsw-alias-*` 深/浅两套令牌 + `ListRow`/`SectionHeader`/`Badge`/`Dot`/`Divider` |
| `ds-sidebar` | `dsh-client-ui-session` | 按 cwd 分组（可折叠/有计数）、搜索、相对时间、`↑↓/Home/End` 切会话 |
| `ds-conversation` | `chat` + `conversation` + `approval` | 用户气泡 / 助手平铺 / 可折叠思考块 / 工具卡（按 `ViewSpec` 分 card·table·plain）/ 审批气泡 / Markdown 两阶段绘制 |
| `ds-composer` | `chat` 的 composer | 自己持有 `flex.Field` 并实现 `pluginapi.Fields` |
| `ds-statusbar` | `layout` 状态行 | `N 轮 · M 步 · ↑in ↓out · tok/s · 上下文`；没有 usage 时**一个字都不显示** |
| `ds-topbar` | `layout` | 标题 / 模式 / 共 N 条 |
| `ds-rightbar` | `plan` + `goal`（只取真实有的数据） | 工具调用时间线，最近 80 条 |

- 契约：`internal/pluginapi`（零实现）、运行时 `internal/uikernel`、移植手册 `docs/plugin-api.md`。
- 数据源：`cmd/mockd` 解真实 v4 日志（会话 / 工具调用 / usage / reasoning）。
- 实测：`CGO_ENABLED=0` 可编译；`-s -w` 后 **9.08 MB**；工作集 **≈86 MB**（`-tail 20/60/200` 基本不变 → 视口虚拟化有效）。

**硬前提**：Viva 靠 `purego`，**不能引入 cgo**。`CGO_ENABLED=0` 必须一直能编过。

---

## 2. 跨线需求：**唯一通道是 `DSH-GO/AGENTS.md` §12**

界面线要宿主侧多做一步的东西（多一个字段、少一次丢数据），**写进 `../DSH-GO/AGENTS.md` §12 那张表**，
不要在对话里口头提——宿主线的会话读的是那份文件，看不到这个会话的聊天记录。

| 谁 | 做什么 |
|---|---|
| 界面线 | 需求进 §12 表：**要什么字段/语义**、**为什么界面需要**、**怎么验**。有替代做法的写清替代方案 |
| 宿主线 | 做完把状态改 ✅ 并在 `CHANGELOG.md` 记一行；没做保持 ⏳ 并写卡点 |
| 界面线 | 表里还是 ⏳ 的，界面**显示「未知 / 整段不显示」，绝不编数据**（假数字比没有更糟） |
| 两边 | 改契约（字段名/形状）先改 `testdata/api.snapshot.json`，再改代码 |

> 当前 7 条（H1 usage / H2 事件时间 / H3 ViewSpec 透传 / H4 model / H5 权限 / H6 reasoning 增量 / H7 content 落法）
> 界面侧代码都已写好并跑通，数据源暂由 `cmd/mockd` 提供；宿主一提供，**渲染代码一行不用改**。

---

## 3. 下一步（按顺序）

1. 等 H1–H7：宿主侧提供后把数据源从 `cmd/mockd` 切到 `internal/transport`（渲染层不动）。
2. 消息级操作补可见入口的展开态：目前只有「复制 / 重试」两个 Ghost 按钮。
3. 推 `MYCF711/dsh-ui` 私有仓库 —— **先问用户确认，不自己推**。

---

## 4. 红线

1. **`CGO_ENABLED=0` 能编过**（一次都不能破）。
2. **界面不直读会话日志**，历史只能经 Transport（对齐 `DSH-GO/AGENTS.md` R7）。
   —— `cmd/mockd` 是**假宿主**，它读日志是它的职责；插件里一行都碰不到日志。
3. 数字一律实测；估算标"估算"；没跑过写"未验证"。
4. **不重启/不关闭正在运行的 DSH**；不写 `D:/DeepSeek Harness/home/profiles/`；asar 与存量会话只读。
5. 每轮产出写 `docs/`，并在 `docs/progress.md` 追加一行。
6. **插件里不许写十六进制色号**：颜色问 `theme`，形状问 `prims`。写死一处，换主题就漏一块。

---

## 5. 本机环境

```bash
export PATH="/c/Program Files/Go/bin:$(go env GOPATH)/bin:$PATH"   # go 1.27.3 + viva v0.2.3
export GOPROXY=https://goproxy.cn,direct
cd "D:/DeepSeek Harness/default-workspace/dsh-ui"
```

**验证界面用无头，别弹窗给用户**：

```bash
CGO_ENABLED=0 go build -o tmp/app.exe ./cmd/app
./tmp/app.exe -addr http://127.0.0.1:7788 -tail 60 -session <id> \
  -shot docs/x.png          # 窗口一起就挪到屏幕外，PrintWindow 抓图后退出，日志里有工作集
```

调试开关（都在 `cmd/app`）：`-shot`（无头抓图）、`-keys down,down,up`（注入按键走插件的
`HandleKey`，验证键盘路由）、`-collapse`（侧栏全收起）、`-expand-think`（思考块全展开）、`-title`。

**写 Go 源码别用 bash heredoc**：反斜杠/换行会被吞（踩过三次）。用 `write` 工具或 `python`，
源码里的换行一律用常量 `nlChar`，不写字面量。
