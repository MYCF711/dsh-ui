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

**spike 已完成，判定：走原生（Viva）。** 三份实测文档在 `docs/`：

| 文档 | 结论 |
|---|---|
| `docs/e1-result.md` | `CGO_ENABLED=0` + 无 gcc 可编译（**purego** 调系统 DLL，不需 cgo）；3.92 MB exe |
| `docs/e2-result.md` | 中文断行 / emoji 彩色 / 等宽 / 卡片 / 1000 条 / **中文 IME（用户实测通过）** |
| `docs/e3-result.md` | 冷启 **171 ms**、静置 CPU **0.0%**、单进程、内存 **75.5 MB** |
| `docs/verdict.md` | **走原生**，附三条件 |

**硬前提**：Viva 靠 `purego`，**不能引入 cgo**。`CGO_ENABLED=0` 必须一直能编过。

---

## 2. 下一步（按顺序）

1. **U0 · Transport 契约** ← 当前主线
   `/api` 的 12 个 method + WS `/api/remote.mux` 流。**先写 `testdata/api.snapshot.json`，再写实现**；method 变更必须让快照 diff 出现在提交里。
   method 清单见 `PLAN.md` §5。
2. **U1 视口虚拟化**：现在 1000 条全渲染。只渲染视口 ±1 屏，目标内存再降 10–15 MB。
3. 补测窗口缩放（E2 第 7 项）。
4. 推 `MYCF711/dsh-ui` 私有仓库 —— **先问用户确认，不自己推**。

---

## 3. 红线

1. **`CGO_ENABLED=0` 能编过**（一次都不能破）。
2. **界面不直读会话日志**，历史只能经 Transport（对齐 `DSH-GO/AGENTS.md` R7）。
3. 数字一律实测；估算标"估算"；没跑过写"未验证"。
4. **不重启/不关闭正在运行的 DSH**；不写 `D:/DeepSeek Harness/home/profiles/`；asar 与存量会话只读。
5. 每轮产出写 `docs/`，并在 `docs/progress.md` 追加一行。

---

## 4. 本机环境

```bash
export PATH="/c/Program Files/Go/bin:$(go env GOPATH)/bin:$PATH"   # go 1.27.3 + viva v0.2.3
export GOPROXY=https://goproxy.cn,direct
cd "D:/DeepSeek Harness/default-workspace/dsh-ui"
```

- `viva build .` 出 `build/<name>.exe`；`viva dev .` 热重载。
- 截图：PowerShell `System.Drawing` 截全屏再裁剪（见 `docs/progress.md` 的命令）。
- **写 Go 源码别用 bash heredoc**：反斜杠/反引号会被吞（踩过两次）。用 `write` 工具或 `python` 写文件。
