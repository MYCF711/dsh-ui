# AGENTS.md · dsh-ui

> 用 Go + Viva 重写 DSH 的**原生界面线**。宿主在 `../DSH-GO`；本仓独立构建，不是宿主仓里的 TypeScript `ui/`。
> 目标：极限性能、极限占用、还原 DSH 原生使用体验、逐部件插件化。Web 兼容只能是可关闭的过渡件。
> 决策依据：`../DSH-GO/UIPLAN.md` 顶部用户原话、§2.10；本仓 `PLAN.md` §8。

## 0. 开工先读

1. 读 `PLAN.md` 的“当前执行口径”和 §8，再读 `docs/progress.md`、`docs/plugin-api.md` 的最新段落与相关源码。早期 E1–E3 spike 已完成选型，**不要重做 spike，不要按旧 60 MB 线或“失败退 WebView”改变路线**。
2. 当前优先级是：**H14 契约门 → 真宿主闭环与流式连续性**；**U1.5 官方首屏外观基线与 U1/U2 并行提前**，再做插件底座、逐部件和完整工作日。H14 迁移已有提交，先核源码不重复实现；不得等全部链路完成才补官方外观。
3. 历史 ✅ 只表示当时某一层有证据。装配成功、静态插件样例、JSON 只读部件、ACP 成功、mockd 成功，都不能单独证明真实 Transport 产品链路完成。
4. 完成记录写清版本、命令、退出码、输入、实际断言与未验证项；截图必须能对应被测功能，不能用“气泡消失”代替真实审批。

## 1. 架构与证据边界

L0 壳→L1 运行时→L2 根本依赖→L3 框架→L4 部件→L5 第三方。公开契约在 `pluginapi/`，槽位在 `internal/uikernel`。已有部件不等于官方包全部行为完成。编译期 Go、JSON 只读、WASM/Web 通道不得混称。mockd 只作过渡；产品只经 `internal/apiclient`，插件不读 v4 日志。历史数字不代替当前 UI+dshd 整树实测。

**硬前提**：`CGO_ENABLED=0` 可构建，不引入 cgo。

## 2. 跨线需求唯一通道

宿主侧需求一律写入 `../DSH-GO/docs/transport-contract.md`，写清字段/语义、为什么需要、提供端与消费端分别怎么验。不要只在对话或本仓文档里口头提。

- 改契约先同步 `../DSH-GO/internal/transport/testdata/api.snapshot.json` 与本仓 `testdata/api.snapshot.json`，再改两边实现。
- 快照目前不等于完整契约：入参别名、HTTP 信封、错误、WS 连续性、审批都要补行为测试。
- 宿主字段缺失时界面显示“未知”或整段隐藏，**绝不编 0、不拿 mode 冒充权限、不用 mockd 的自有字段假装宿主已经提供**。
- H 表某项 ✅ 不自动表示界面消费完成；本仓仍须核对自己的 struct、事件归约和实际画面。

## 3. 当前工作单（按依赖顺序）

### P0-A · 先过 H14 契约门

详细唯一契约读 `../DSH-GO/docs/transport-contract.md`：HTTP 裸参数、`/ws`、`{seq,type,data,time}`、每会话独立 WS + journal、原始 history frames、journalEpoch/reset。唯一 owner 冻结两份快照/golden 后，宿主与 UI 才能并行实现；共享 apiclient/mockd/快照不多人同时改。

H14 只用真实 apiclient 对契约夹具验合成 text/reasoning/tool/approval 重放、两会话不串流与 stale_epoch；**真实生成归 H6，真实审批执行归 H13**，不得在契约门里循环前置。迁移已有提交，先核最新实现，不重复施工。

### P0-B · H14 后验真实链路

- 先 `/connect` 成功，再 list/load/subscribe；HTTP 与 WS 同 cookie。用同一新 dshd、临时 home、真实 ready 地址/token，验 create → list → load → prompt → text/reasoning/tool → cancel → 第二轮 → 再 load，不用旧二进制或 mockd 代替。
- 每会话独立订阅、journalEpoch/seq、revision/index、流正文与缓存；后台继续收数据，切回不重播、不串流、不丢字。
- 断号只调 history 补原始 frames，不整页刷新；epoch 变化必须 reset 后重建基线。消费原始 v4 text/reasoning/tool，结束帧不丢。
- A 生成时切 B 再回 A，最终内容与一直停留一致；5 会话切换按 `PLAN.md` 性能口径复测。

### P0-C · 真审批与字段消费

- Allow/Deny 必须经 Transport 原样回传 requestId/optionId；允许后工具真的继续，拒绝后不执行，取消/超时也有真实结果。设计见 `../DSH-GO/docs/h13-approval.md`，本地气泡消失不算完成。
- LoadResult/Message 消费 model、permission、contentWidth、time、usage；顶栏/状态栏使用真值，未知隐藏，不拿 mode 冒充权限。
- 跨仓记录含 dshd_rev、ui_rev、配置、启动/清理命令、真实断言、证据路径和未验项。

### P1 · 插件生态底座

1. **授权树**：父槽显式允许子槽；未声明、越权、重复 single、循环挂载注册期 fail-loud，公开槽必须实画。
2. **禁用/替换/卸载**：强制的是基础能力而非具体实现；替换不改壳，重复 id 不静默装两份；Dispose 撤销 painter、输入、服务、goroutine，无残留。
3. **外部通道**：编译期 Go、JSON 只读、WASM/可关闭 Web 分别写能力、权限和证据，不混称完整生态。

### P1.5 · 官方首屏外观基线（与 P0/P1 并行）

对照官方同尺寸截图和 `docs/official-specs.md`，先补品牌/导航/工作区、真实会话行、居中欢迎区、完整 composer、模型/权限/附件控件及右栏默认关闭。1920×1032 与 1182×832 各留证据，关键容器几何偏差 ≤8px，字体/行高用官方令牌；未知值隐藏，不做死按钮或静态假还原。

## 4. 并行与安全

- Lead 只拆包/派单/验收：≥2 个独立 READY 同轮派给子代理或有上下文成员；派后不 wait/poll，完成方主动通知。验收后更新 `../DSH-GO/UIPLAN.md` §4.1 并派下一项，表空先补项。
- 任务书写 owner/paths/rev/依赖/验收/证据；独立插件目录并行，`cmd/app`、契约、uikernel、apiclient、快照串行，同文件单写者。
- 改前重读；遇 `FS_STALE_VERSION` 或并发变化先合并。不得 `git checkout --`/reset 覆盖别人，不移动/删除兄弟仓或共享源码目录来造失败分支。
- 不按镜像名杀 `dshd.exe`/`app.exe`，不碰其他会话进程；测试只回收自己启动且记录归属的 PID。不得重启/关闭 DSH 或 magpie，不写 profiles/asar/原始会话。
- 测试用临时 home、OS 分配端口与无头窗口；不向用户弹窗，不抢现有 GUI 地址。
- 每轮在 `docs/progress.md` 追加“实现/证据/未验证/下一步”，不要只写 ALL GREEN。
- 推 `MYCF711/dsh-ui` 或其他远端前先得到用户当次确认。

## 5. 常用验证

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
export PATH="$(go env GOPATH)/bin:$PATH"
export GOPROXY=https://goproxy.cn,direct
cd "D:/DeepSeek Harness/default-workspace/dsh-ui"
CGO_ENABLED=0 go build ./...
go vet ./...
go test ./...
```

无头抓图只用于对应视觉断言。`-keys` 只验证插件路由，不证明 OS 键盘；`-demo` 不证明宿主审批。Go 源码用 write/edit，不用 bash heredoc 拼接。
