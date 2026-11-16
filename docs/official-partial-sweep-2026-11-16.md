# 15 条 PARTIAL 逐条清扫（2026-11-16）

> 范围：官方 rc.2 66 个 UI 包中账本原有的 15 条 PARTIAL。分类严格使用：A＝差异已消除可直接 ✅；B＝小补后可 ✅；C＝真实平台/框架限制；D＝需要宿主新能力；G＝宿主能力基本在，剩余是整块 GUI 工程（不属于小补）。本报告不拿 G 冒充 B，也不拿 C/D 给未写的 GUI 免责。

## 总结

- **A：0。B：1（#44，已补并升 ✅）。**
- **C：#12、#18、#19；其中 #19 查明现有代码属于 attachment 而非 file-drag，降为 ❌。**
- **D：#17、#28、#46、#54、#63、#64。**
- **G：#16、#38、#41、#50、#59。**
- 新合计：**✅ 51 · 🟡 13 · ❌ 2**（#66 voice-input 保持 ❌）。

## B（已小补并升 ✅）

### #44 `dsh-client-ui-settings-session-log` → ✅

官方恰好两块：general Switch（id `session-log-deepseek`）和 shell.overlay Toast（id `session-log-upload-toast`），见官方 `src/client/index.ts:25-40`、`UploadRow.tsx:26-49`。原账把两个 id 混为一个，且运行时把 served 写死为 false。

本轮已补：

1. general id 改为 `session-log-deepseek`，补官方 description「帮助改进 DeepSeek 模型与产品」：`plugins/dssettings/official_tree.go:325-330`。
2. `SessionLogServed` 改由 `settings.describe.namespaces` 真值计算：`cmd/app/settings_panel_wiring.go:116-137`。
3. 写入先发布 saving，busy 防重入并禁用同 namespace 行：`plugins/dssettings/official_panel.go:529-539,653-668`。
4. 新增 `shell.overlay/session-log-upload-toast`，官方「设置已保存 / 无法保存设置」、成功/失败色、自动 dismiss、旧定时器不能清掉新通知：`plugins/dssettings/session_log.go:16-140`；装配在 `plugins/dssettings/settings.go:141-149`。
5. 测试：注册两 id、成功/失败/dismiss、防重入、其它 namespace 隔离：`plugins/dssettings/session_log_test.go`；describe 两态：`cmd/app/settings_update_channel_e2e_test.go:122-150`。

**宿主行为差异保留记录**：宿主已服务 namespace，但 `enabled=true` 因没有 `dsh_session_log` 上传方而返回 501（`DSH-GO/internal/settingsform/client_ui_schema.go:824-846`）。这不妨碍官方 UI 契约两块落地：失败会走本轮实现的官方失败 Toast；真正开启上传仍是宿主后续能力，不伪报成功。

## C（真实平台/框架限制）

### #12 `dsh-client-ui-cordis` — 保持 🟡

面板四方法与事件重读已落地，但整包仍缺：① 动态 Client/React `tool.view.cordis` 与 getClientCode/settle/renderFailure；② 含 Client 半包的 Run；③ Cordis 审批队列及按钮；④ 无生产者的四张历史工具卡。官方动态组件证据 `slots.ts:23-37`、`CordisRunRow.tsx:119-130`；本仓 API 仅 inventory/stop/undefine/runHostHalf（`internal/apiclient/cordis.go`）。另有跨会话水位 bug：`eventSeq` 是全局单值而 seq 属每会话 journal（`cmd/app/cordis_wiring.go:68-88,129-140`），A 会话高 seq 会压住 B 会话低 seq 新事件，需按 `(sessionID,journalEpoch)` 记水位。

### #18 `dsh-client-ui-file-actions` — 保持 🟡

主阻断是生产级图片剪贴板写入：官方要解码/PNG 后 `clipboard.write`，Viva 公开面只有图片读取；`SetClipboardFiles` 不是同语义。`image` target 仍无图片生产方，URL-only 也无取字节链。菜单三枚官方图标未接，同步 Resolve 无 stat 预取置灰，保存链在 UI 线程同步 HTTP。

旧理由修正：reveal 宿主已经存在（`DSH-GO/cmd/dshd/serve_native_api.go:79-90`），不是“宿主无 `openWorkspacePath`”；只是 GUI 尚未接 apiclient 与可行动反馈。stat 宿主也已有（`handlers_workspace_official.go:1214-1240`）。这些是可补子项，但不能消除图片平台阻断，故不升 ✅。

### #19 `dsh-client-ui-file-drag` — 降 ❌

原账把 `plugins/dsattachment/overlay.go` 算作本包实现，包归属错了。该压层对应 `dsh-client-ui-attachment/src/client/ComposerAttachments.tsx:45-60` 的 DropOverlay；官方 file-drag 是另一套：`ctx.fileDrag.actions.prepare/read/reportFailure`（`dsh-client-ui-file-drag/src/client/index.ts:13-49`）、应用内 token/DownloadURL 与桌面 prepare/start/release/cancel（`transfers.ts:32-117,120-154`）、`file-drag.feedback` 失败 Toast（`feedback.tsx:18-25`）。这些在本仓均为零实现；Viva 的 OnFileDrop 是 OS 文件拖入，DragSource 也只是应用内拖放，不是 OS IDropSource 拖出。插画/blur/fade 属 #6 attachment 的剩余差异，不能给 #19 记 PARTIAL。

## D（需要宿主新能力）

### #17 `dsh-client-ui-entry-customization` — 保持 🟡

真正阻断不是单纯 `agentPreset` 字段：`mode` 已会写入 `header.agentPreset`（`DSH-GO/cmd/dshd/serve_create.go:52-65`）。但宿主 roster 无 cordis preset，且 preset 只是工具可见性模式，不是官方 Creator 插件清单；`internal/creation/service.go:39-44` 明确无 Creator。仅发 `mode:'cordis'` 会在运行时回落 default，形成假 Creator。需要 cordis preset 组合 + Creator 运行时/工具链 + 区域 prompt + `whenDraftExists:'append'`。guide More/owner props 仍未做。

旧 hover 理由已失效：Viva 有 Hovered/Focused/FocusVisible，可补 activeMark/pin 三态；pin 行内而非绝对覆盖、图标色与 0.5px 分隔线也仍是小差异，但均不改变 D 主判定。

### #28 `dsh-client-ui-plan` — 保持 🟡

旧“宿主没有 plan 投影”已失效：`DSH-GO/internal/transport/plan_view.go:8-38` 与 `handlers_session_official.go:994-999` 已提供 `{active,pending}`。剩余：① GUI 未提取 plan，且 chip 需要真实 `/plan off`，宿主命令注册表尚无；② `exit_plan_mode` 仍直接 Stop，没有 `intent.kind=plan-review` 待答问题，父提问卡、手动/自动查看全文及临时 review 地址无生产者；③ 本仓只画最新一轮计划卡，官方逐轮；④ 缺 navigation 地址、多实例、plan-review/subagent 地址和 tab title 派发；⑤ 临时预览 loading/failed/expired、相对链接等既有差异仍在。

### #46 `dsh-client-ui-settings-subagent` — 保持 🟡

旧“allowedModels 值到不了界面”不准确：它虽不进 `sections.fields`，但在 `SettingsNamespaceView.Value/Base/User` 原始 JSON 中，宿主也支持对象数组写入与校验。模型选择 UI 因此可做，不是平台限制。

仍为 D 的原因是宿主没有消费方：子代理固定父模型，无 provider/model 参数与授权投影（`DSH-GO/internal/settingsform/subagent_schema.go:49-68`）。GUI 还缺 enabled Switch、按 provider 分组候选、失效路由保留、loading/error/partial/empty、至少一项校验和同 revision 两字段保存；这些属于 G 子项，但仅画可读写表单不会让设置真实生效。

### #54 `dsh-client-ui-sidebar-terminal` — 保持 🟡

宿主用户终端能力已齐：create/attach/list/close/environment/shells/rename/retain/watch 与 PTY input/resize（`DSH-GO/internal/transport/handlers_job_terminal_official.go:89-100`）。GUI 仍只有 open/input/output、单 termID，缺 per-tab 多终端、close/retain、shell guide、动态标题/重命名、VT snapshot/raw input/resize 和连接状态，这是 G 子项。

整条仍 D：agent 终端生产者、控制权移交和 Sandbox 徽标所需控制状态宿主没有。只做用户终端不能升 ✅。

### #63 `dsh-experimental-client-ui-cot-translation` — 保持 🟡

宿主仍无 `cot-translation` provider/targetLanguage 设置 namespace，limits 固定 `deepseek-official/auto`，偏好/上限事件刷新链也未接。正文仍非官方 Markdown+代码工具条，原文可带 `<signature>`；收起不取消请求、缓存整消息常驻；pending 用文字非 spinner；provider 不可用时入口被错误隐藏；超长片段空白只认 ASCII。

原“默认不启用”与代码相反：空配置全装（`internal/uiplugins/loader.go:45,52-64`），ds-conversation 无条件 append，展开思考即自动翻译（`plugins/dsconversation/conversation.go:672-680`、`cot_translation.go:282-298`）。需要实验部件独立启停/卸载生命周期，不是只剩 2 条差异。

### #64 `dsh-experimental-client-ui-forge` — 保持 🟡

宿主仍有实质缺口：① localDiff 丢弃 base/commits，无 upstream 时可形成 `HEAD...HEAD`，且 `DiffFile` 不产官方必填 layer/digest，tracked 状态、previousPath、omitted 也不完整；② follow 仍不是官方 ForgePanelState 流，changeRequestStates 仍 unary 且只读 env；③ 凭据来源分裂；④ 无 worktree 设置 namespace，`retainOnArchive` 只有私有 JSON 读取点；⑤ editable/blocker、目录 missing、waiting/restoration 无统一读出口。

旧“refresh/follow provider 是占位”已过时：REST/GraphQL/写入已接真实 HTTP，应改记为流形状、状态来源与凭据整合不符。GUI 也未完成 CR 主体、CommitMenu、localText 消费、未变更行展开、文件树、split/wrap、基准分支菜单、工作树 journal 恢复与 guide 入口。空配置同样默认全装 ds-forge，不是账本所称默认不启用。

## G（宿主能力基本在，剩余是整块 GUI 工程）

### #16 `dsh-client-ui-directory-picker-native` — 保持 🟡

本轮已补首屏 WorkspaceChip/WorkspacePicker、native/browse 防重入与 `WorkspaceCreate → onPick(workspaceId)`；两种后端只复用经 `session.load` 证实 `Total==0`、cwd 匹配且未归档的空白会话。ENV_BOUNDARY 仍只是人工验收边界，不是产品能力缺失。**新确认的 D 阻断**：宿主 `session.create` 只解 `mode`（`DSH-GO/internal/transport/server.go:1211-1227`），空工作区无法按官方 `{workspaceId,sessionId?}` 创建/复用，GUI 明确报错而不在进程 cwd 假建。宿主补齐此契约并完成桌面人工 `DSH_NATIVE_PICKER=1 go test ./internal/native -run TestRealPickerProbe -v` 后才可升 ✅。

### #38 `dsh-client-ui-settings-account` — 保持 🟡

宿主 profile/balance/start/cancel/signOut/state/runningTasks/watch 已发布，不再记 D。**2026-11-16 后续实现已闭合主链**：`CredentialStored` 不再写死，账号 section 不再是静态未知页；真实 profile/contact、充值/赠金余额、usage/top-up、打开设置边沿刷新、失败态、登录/取消、运行任务三态确认与登出均经宿主真方法。证据：`internal/apiclient/account_actions_test.go`、`cmd/app/account_settings_test.go`、`plugins/dssettings/official_account_test.go`、`plugins/dsnotifications/account_menu_test.go`；API 快照 102→108，counts 与 methods 一致。**保持 🟡的剩余项**：avatar 图片加载（目前仅官方 skeleton）、contact、addApiKey、两类 onboarding、quota notice、Platform page、完整 AccountMenu，以及 `account.watch` 流（当前仍是 30s 轮询，未冒充 watch 已接）。

### #41 `dsh-client-ui-settings-models` — 保持 🟡

宿主 provider 目录、discoverModels、modelCatalog、credentials describe/set/unset 已有。GUI 目前只有 defaultModel 与 retryPolicy 行，且 retry 是逐行立即写。缺 provider directory join、账号行、凭据点与密钥写删、baseURL/protocol/models/input/reasoning 编辑、catalog/custom 添加、模型发现测试、删除 Modal、保存/只读/加载/失败/冲突态、动态 provider-card/footer、onboarding 与推送失效刷新；是独立 Models store/page，不是小文案补丁。

### #50 `dsh-client-ui-sidebar-browser` — 保持 🟡

URL 白名单、导航状态机、工具条和 WebView2 载具已在。剩余是右栏 occurrence、多实例/关闭、恢复卡、导航与页面生命周期的 GUI 大改。原“官方 Web 也不持久化”错误：官方 `browser/store.ts:20-33` 的 `persist:'dsh.sidebar-browser.v1'` 被 iframe 与 Electron 共用，所以恢复卡是本仓真实缺口。沙箱开关是官方 optional，不再算作契约缺口；前往按钮常显与地球图标线帽是小差异。

### #59 `dsh-client-ui-trajectory` — 保持 🟡

原“缺分组/折叠”已过时：Turn 分组折叠在 `plugins/dsrightbar/rightbar.go:228-281`，搜索/选择/虚拟化/Step Inspect 在 `wpd1.go:49-131,343-477`。真实剩余：① groupTurns 只收 role=tool，缺 assistant/user/system/context/compaction/request 完整账本；② assistant calls、递归 subcalls 与三套全局折叠；③ duration/actual-time、TTFT/解码段、缩放拖选 timeline；④ request/options/usage/timing/schema/diff 多页 Inspector；⑤ 未接 LoadOlder，仍限当前窗口与最近 80 条；⑥ 旧三栏仍靠视口偏移桥。数据通道基本在，属于 GUI 大改。

## 验证

- 原基线：`python scripts/parity-count.py --check` PASS（50/15/1）。
- 本轮 B 代码：`go test ./plugins/dssettings ./cmd/app ./internal/apiclient`，三包 exit 0。
- 既有定向回归：`go test ./internal/apiclient ./plugins/dsrightbar ./plugins/dssettings ./cmd/app`，四包 exit 0。
- 子代理只读复验：dscordis/dsentrycustom/dsfileactions/dsattachment、dsconversation/dsforge 等相关包均 PASS；未把“现有测试自洽”当作缺失契约已实现的证据。
