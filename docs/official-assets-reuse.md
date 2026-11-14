# 官方可复用素材清单（W101）

> 本清单只登记当前官方安装包中实际定位到的素材。独立文件逐字节复制；CSS data URI 只做 base64 解码；
> primitives 图标只保存官方 `d.jsx` 图元原文，不手画 SVG、不补不存在的文件。

## P0 · 冻结复核（GUI-P0，2026-11-08 (GUI-P0)）

> 本节由 GUI-P0 追加；口径：**主库 `$DSH_HOME/dsh-ui-official/` 只读**，本节只记录按**当前产品树**（`../dsh-ui` 工作树，HEAD `3925e84` + 未提交改动）复核的结果。证据：`DSH-GO/tmp/GO2-TEAM/GUI-P0/{audit.json,node-diff.md,logs/}`。

- 复核方式：机器逐条解析本文的 `.go` 引用（链接式 `label` + 相对路径 + `#Lx-Ly` 与 `` `path:N` `` 两种写法），核对文件存在性、行号范围与首行内容；**未重拆 asar、未重跑解构、未启动 GUI**。
- 镜像与主库：本文档与 `home/dsh-ui-official/structure/` 同名文件在本次修正前 **逐字节相同**；本次只改镜像，**主库对应文件待同步**（更新顺序：主库 → 镜像，本次受写作用域限制只动了镜像）。

### 素材校验（56 行全量）

| 检查 | 结果 |
|---|---|
| 表中素材行 | **56** |
| 产物文件存在 | **56/56** |
| 表内声明 sha256 == 产物实际 sha256 | **56/56 一致**（0 不一致） |
| 在 `assets/official/manifest.json` 在册 | **56/56** |
| 主库 `LIBRARY-MANIFEST.json` | `fileCount = 69`，与入口文档「69 项」一致 |
| 缺官方证据的自造素材 | **0**（无「无来源/无 hash/无 viewBox」的条目） |

结论：`same` 之外的 `derived` 条目在本表有明确来源与 hash，**未发现手画冒充官方**的素材；素材侧无 P0 阻断项。

---

## 1. 来源、rev 与证据边界

- 本仓 rev：`3925e84f1524dd101e020d74b1d5bb422d25cf4d`。
- 官方版本：**`0.2.2-rc.1`**（六包 `package.json` 一致）。
- 官方 asar：`D:/DeepSeek Harness/Desktop-0.2.2-rc.1/resources/app.asar`，sha256：`5fa8a6b21bceb8f3e99e30efa2270efd0cc773a11e42aa0488f804a64b34e637`（213202918 B）。
  - **上一版基线（rc.5）**：`D:/DeepSeek Harness/Desktop/resources/app.asar`，sha256 `e1b6f25fa937da98086c3e77af77aa14791105963c4d337a1c42179d232c6915`（182766714 B）。本表的 `source` 路径与哈希**仍为 rc.5 来源**；rc.1 复核结论：10 条文件型素材逐字节相同、0 条变化；46 条符号型片段未重摘（见主库 `SOURCES.md §5.2`）。
- 只读提取物：`D:/DeepSeek Harness/default-workspace/dsh-ui/tmp/official/**` 与 `tmp/asar-css/**`（**均为 rc.5**，不是 rc.1；rc.1 事实用主库 `scripts/asar-facts.py` 从 asar 偏移读取）。
- 机器索引：[manifest.json](../assets/official/manifest.json)。它记录每项来源、源文件哈希、产物哈希、尺寸、同物状态、目标组件和许可证；manifest 本身不是素材，不做自哈希。
- 本任务不改产品代码、现有素材或测试，不启动 GUI。“复用目标”是后续接线位置，不表示本任务已经接线。

### 三类条目

| kind | 含义 | 使用边界 |
|---|---|---|
| `asar-file` | asar 中真实独立文件，字节不变 | 可直接作为对应 SVG/WOFF2/CSS/TEXT 使用 |
| `embedded-data-uri` | 官方 CSS 内嵌图像的 base64 解码 | 字节即官方图像；输出文件名是本清单描述性命名，源位置保留在表内 |
| `official-jsx-fragment` | 官方前端 chunk 中 `d.jsx("path"/"rect"/"circle", …)` 原文片段 | 供 `dsassets` 登记/解析；不是独立官方 SVG，不能声称官方包曾提供同名 `.svg` |

## 2. 许可证边界

- **DeepSeek / MIT**：`dsh-web-frontend`、相关 client 包及桌面包声明 MIT；随附 [DeepSeek-MIT.txt](../assets/official/licenses/DeepSeek-MIT.txt)。复制、修改和再分发须保留版权与许可原文。MIT 文本没有商标授权条款；鲸鱼、`DEEPSEEK`、`HARNESS` 只用于还原官方品牌，不得用来暗示本仓或衍生产品获得官方背书。
- **Montserrat / SIL OFL 1.1**：随附 [Montserrat-OFL.txt](../assets/official/fonts/Montserrat-OFL.txt)。字体可随软件捆绑、嵌入和再分发，但不得单独出售、不得改用其他许可证，修改版须遵守保留字体名规则。官方只把 Light/Regular/Medium 用于品牌欢迎页；普通 UI 仍用系统字体栈。
- `official-jsx-fragment` 的许可来自其官方包，不因截取片段而变成另一许可；`embedded-data-uri` 同理。每行“许可”列指向上述两类完整边界。

## 3. 素材总表

“dsassets 同物”只回答当前 `plugins/dsassets/**` 是否已有同一官方图元；`dsbrand` 中的品牌路径不冒充 `dsassets` 已登记。
所有哈希均为**本仓产物文件**的 sha256；源文件 sha256 另见 manifest 的 `source_sha256`。

### 3.1 品牌

| 产物 | 官方来源路径 / 符号 | 类型 | sha256 | 尺寸 / viewBox | dsassets 同物 | 复用目标组件 | 许可 |
|---|---|---|---|---|---|---|---|
| [`brand/deepseek-harness.svg`](../assets/official/brand/deepseek-harness.svg) | `/renderer/assets/welcome-brand.svg` | SVG | `182f040f9d3eccf25b20f5bbfc9d50a21296a88978b91b4dec7e64cbd16b4039` | 472×40；viewBox=0 0 472 40 | 否 | `plugins/dssidebar/frame.go；plugins/dsconversation/hero.go` | MIT |
| [`brand/whale-light.svg`](../assets/official/brand/whale-light.svg) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/favicon.svg` | SVG | `9e983b4f649c25c6ca0623a50be1a6e705fd8f49f756638b980fa13b40575ab6` | 50×50；viewBox=0 0 50 50 | 否 | `plugins/dsbrand/brand_path.go；plugins/dssidebar/frame.go` | MIT |
| [`brand/whale-dark.svg`](../assets/official/brand/whale-dark.svg) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/favicon-dark.svg` | SVG | `dde7db781f41e7aeb4709ff5485af2bdcf7f4b4ea81b4500cfd5df44efdbd817` | 50×50；viewBox=0 0 50 50 | 否 | `plugins/dsbrand/brand_path.go；plugins/dssidebar/frame.go` | MIT |
| [`brand/running-whale.png`](../assets/official/brand/running-whale.png) | `/dsh/node_modules/@deepseek-ai/dsh-client-ui-chat/lib/client.js#ChatView.module.css runningWhaleAnimated data:image/png;base64` | APNG | `0fd990d118113d19af7af4480f2b55d610b103451a08be6f7bf5a146823f8bd3` | 28×28；CSS 默认 14×14 | 否 | `plugins/dsconversation/message_area.go 运行态（仅动画能力具备时）` | MIT |

### 3.2 侧栏 / 应用导航 JSX

| 产物 | 官方来源路径 / 符号 | 类型 | sha256 | 尺寸 / viewBox | dsassets 同物 | 复用目标组件 | 许可 |
|---|---|---|---|---|---|---|---|
| [`icons/sidebar/panel-left.jsx`](../assets/official/icons/sidebar/panel-left.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconPanelLeftOutlineRegular` | JSX | `a171093fd638852875113c5f629f876acc9d399af1868efb5207c305fa2a4b8a` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 侧栏折叠` | MIT |
| [`icons/sidebar/new-chat.jsx`](../assets/official/icons/sidebar/new-chat.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconNewChatOutlineRegular` | JSX | `962dfe14684e51461af02e38bd4931dd28e49b9725dd0fd6fdf88c6ccbec7011` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 新会话` | MIT |
| [`icons/sidebar/apps-add.jsx`](../assets/official/icons/sidebar/apps-add.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconAppsAddOutlineRegular` | JSX | `18d9d7d453637bde0b65a3e2357ddb48836eaff517ede6355b64d583a832b45d` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 应用/插件入口` | MIT |
| [`icons/sidebar/plugin-pinwheel.jsx`](../assets/official/icons/sidebar/plugin-pinwheel.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconPluginPinwheelOutlineRegular` | JSX | `29e4835f09ea781114691721eaa8fa074223d73c72ca3e2fc3bf73a17bca645d` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 插件导航` | MIT |
| [`icons/sidebar/clock.jsx`](../assets/official/icons/sidebar/clock.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconClockOutlineRegular` | JSX | `efb0189375678eb11824f7375af93cfd93b72eab97311e2d9f16553daa2bdb89` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/task_actions.go 自动化任务` | MIT |
| [`icons/sidebar/compact.jsx`](../assets/official/icons/sidebar/compact.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconCompactOutlineRegular` | JSX | `ecf9b854d8f8f390ccfae6a3819761630fa5032282a397d5ea0afdd070dfdfc2` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/task_actions.go 上下文压缩` | MIT |
| [`icons/sidebar/ellipsis.jsx`](../assets/official/icons/sidebar/ellipsis.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconEllipsisOutlineRegular` | JSX | `925f452c2ee227ac01c78f5bc92da8e01d3d2d39cd00afda600971ac8bd7aeb8` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 更多菜单` | MIT |

### 3.3 工作区 JSX

| 产物 | 官方来源路径 / 符号 | 类型 | sha256 | 尺寸 / viewBox | dsassets 同物 | 复用目标组件 | 许可 |
|---|---|---|---|---|---|---|---|
| [`icons/workspace/search.jsx`](../assets/official/icons/workspace/search.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconSearchOutlineRegular` | JSX | `07b6073592f85b5e0279fdf09f4d2c1dbd26ab2f1431008435b2ab46d735c9f3` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 工作区搜索` | MIT |
| [`icons/workspace/folder-close.jsx`](../assets/official/icons/workspace/folder-close.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconFolderCloseRegular` | JSX | `48ba7d908e53cc751a9b52e6f60b3e98377adcc20063e3887e4642cc7cc52c38` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 项目/工作区` | MIT |
| [`icons/workspace/folder-open.jsx`](../assets/official/icons/workspace/folder-open.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconFolderOpenRegular` | JSX | `ddb2118422416d6d2f307751bec7e36176c9bb7196c48183595a5f02d3c6efb1` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 展开项目/工作区` | MIT |
| [`icons/workspace/project-add.jsx`](../assets/official/icons/workspace/project-add.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconProjectAddOutlineRegular` | JSX | `f74e43cbb496b641d43113cd8dc56656a797d8b05790a57a0967c9cdea98b200` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 添加项目` | MIT |
| [`icons/workspace/workspace-tree.jsx`](../assets/official/icons/workspace/workspace-tree.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconWorkspaceTreeOutlineRegular` | JSX | `d0dd0f271229f0144f4fd0c37e131ac2075bbc70b3d60afbd6b8afed34f226cd` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 工作区树视图` | MIT |
| [`icons/workspace/flat-list.jsx`](../assets/official/icons/workspace/flat-list.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconFlatListOutlineRegular` | JSX | `92b63393c1f9252502319caff637675eef4e15c0bc744aab82cef17232580be2` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssidebar/frame.go 平铺列表视图` | MIT |

### 3.4 Composer / 模型 / 权限 JSX

| 产物 | 官方来源路径 / 符号 | 类型 | sha256 | 尺寸 / viewBox | dsassets 同物 | 复用目标组件 | 许可 |
|---|---|---|---|---|---|---|---|
| [`icons/composer/paperclip.jsx`](../assets/official/icons/composer/paperclip.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconPaperclipOutlineRegular` | JSX | `29290ac8fc4c9d0d9797b970bfc28119dfe1fc4982b36544dc9005d93b67587a` | viewBox=0 0 16 16；默认 16px | 有：paperclip | `plugins/dscomposer/attachments.go 附件` | MIT |
| [`icons/composer/model-data.jsx`](../assets/official/icons/composer/model-data.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconDataOutlineRegular` | JSX | `e06f541facd2205ad6e5eb4b32c0044cb3df823aeb0ac4d21868ddccec88a0ec` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dscomposer/select.go 模型选择` | MIT |
| [`icons/composer/chevron-down.jsx`](../assets/official/icons/composer/chevron-down.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconChevronDownOutlineRegular` | JSX | `865d156be1a74c3829a50acb1b56a59007b9c4280886fe728a5b0ff096a17632` | viewBox=0 0 16 16；默认 14px | 有（未登记）：ChevronDownOutline | `plugins/dscomposer/select.go 下拉` | MIT |
| [`icons/composer/check.jsx`](../assets/official/icons/composer/check.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconCheckOutlineRegular` | JSX | `3b47a8c21d5bf626867f2d6b45a6ff8b2f7362123cfa76de5a395a2b2916bb8c` | viewBox=0 0 16 16；默认 16px | 有（未登记）：CheckOutline | `plugins/dscomposer/select.go 当前项` | MIT |
| [`icons/composer/plus-medium.jsx`](../assets/official/icons/composer/plus-medium.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconPlusOutlineMedium` | JSX | `7f32634ad9eecb3701fbfc6e695341f4a62eaba0ad7bdb68e7291726ae5dcd51` | viewBox=0 0 16 16；默认 16px | 有：composer-add | `plugins/dscomposer/composer.go 命令入口` | MIT |
| [`icons/composer/send-outline.jsx`](../assets/official/icons/composer/send-outline.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconSendOutlineRegular` | JSX | `fce2c0ec337bbd8d7100a9ce231bb8be75cbad6616adec75dec0f72bf2e0cf90` | viewBox=0 0 16 16；默认 14px | 有：send-outline | `plugins/dsconversation/message_area.go 队列发送` | MIT |
| [`icons/composer/permission-read-only.jsx`](../assets/official/icons/composer/permission-read-only.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#PermissionIconReadOnlyRegular` | JSX | `01f900b1380916b2602a582695297099b7273dbdf76fd3a64aa334122c3e7e64` | viewBox=0 0 16 16；默认 16px | 有：permission-read-only | `plugins/dscomposer/select.go 只读` | MIT |
| [`icons/composer/permission-workspace-write.jsx`](../assets/official/icons/composer/permission-workspace-write.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#PermissionIconWorkspaceWriteRegular` | JSX | `54eb0aab8ca431bcae23c2eb246137969d52ff7bdef611e52bc55319c5a56936` | viewBox=0 0 16 16；默认 16px | 有：permission-workspace-write | `plugins/dscomposer/select.go 工作区写入` | MIT |
| [`icons/composer/permission-full-access.jsx`](../assets/official/icons/composer/permission-full-access.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#PermissionIconFullAccessRegular` | JSX | `8e0588606994ccc50236b10e3b8512abdc2fd5408f1a244ad0a435fafb91b5f0` | viewBox=0 0 16 16；默认 16px | 有：permission-full-access | `plugins/dscomposer/select.go 完全访问` | MIT |
| [`icons/composer/permission-auto-review.jsx`](../assets/official/icons/composer/permission-auto-review.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#PermissionIconAutoReviewRegular` | JSX | `a3d728c5508caaa0a1cf9f5f721d8b04cf15fae492d0ec935c72e0d5d47ad431` | viewBox=0 0 16 16；默认 16px | 有：permission-auto-review | `plugins/dscomposer/select.go 自动审查` | MIT |
| [`icons/composer/send.jsx`](../assets/official/icons/composer/send.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-client-ui-conversation/lib/client.js#InputBar-send-path` | JSX | `24c975ad1fcfde5344d06b733ec1823492f64949b6cb8087321c8fb73c5f68c5` | viewBox=0 0 16 16；默认 16px | 有：composer-send | `plugins/dscomposer/icons.go 发送主按钮` | MIT |
| [`icons/composer/stop.jsx`](../assets/official/icons/composer/stop.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-client-ui-conversation/lib/client.js#InputBar-stop-rect` | JSX | `304524de22b68dbae333458f2a86079dc6247e84d4dcbcc257ef5a456e9fc3be` | viewBox=0 0 16 16；默认 16px | 有：composer-stop | `plugins/dscomposer/icons.go 停止主按钮` | MIT |

### 3.5 设置与关闭

| 产物 | 官方来源路径 / 符号 | 类型 | sha256 | 尺寸 / viewBox | dsassets 同物 | 复用目标组件 | 许可 |
|---|---|---|---|---|---|---|---|
| [`settings/update-close.svg`](../assets/official/settings/update-close.svg) | `/renderer/update-close.svg` | SVG | `1a1931528c542a460d1db6fde84d82d1a03a06155485265d43634031273b20d5` | 14×14；viewBox=0 0 14 14 | 否 | `cmd/app 更新对话框（仅该语义；固定 #0F1115，不作 currentColor 设置关闭）` | MIT |
| [`icons/settings/settings.jsx`](../assets/official/icons/settings/settings.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconSettingsOutlineRegular` | JSX | `3558d09746b62933c13eb1933fdc062e1cf327b4591e10cf4c40d668ef5a88cf` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssettings/settings.go；plugins/dssidebar/footer.go 设置导航` | MIT |
| [`icons/settings/close-outline.jsx`](../assets/official/icons/settings/close-outline.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconCloseOutlineRegular` | JSX | `e2c28e3b32bb838195cbdb6112f8a0965d9b14650bd08ccdeed254bce55d16f6` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dssettings/settings.go 关闭` | MIT |
| [`icons/settings/chevron-left.jsx`](../assets/official/icons/settings/chevron-left.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconChevronLeftOutlineRegular` | JSX | `93fb79b9412f953ccab79a61c1d28dced4520dbaec76220a910a7654201d98cc` | viewBox=0 0 16 16；默认 14px | 否 | `plugins/dssettings/settings.go 返回` | MIT |
| [`icons/settings/chevron-right.jsx`](../assets/official/icons/settings/chevron-right.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconChevronRightOutlineRegular` | JSX | `66aba03adedfd3194cd9cbc2e4036f4a56159d710f6f3c706abcd2a151263736` | viewBox=0 0 16 16；默认 14px | 否 | `plugins/dssettings/settings.go 导航/折叠` | MIT |

### 3.6 消息反馈 JSX

| 产物 | 官方来源路径 / 符号 | 类型 | sha256 | 尺寸 / viewBox | dsassets 同物 | 复用目标组件 | 许可 |
|---|---|---|---|---|---|---|---|
| [`icons/messages/copy.jsx`](../assets/official/icons/messages/copy.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconCopyOutlineRegular` | JSX | `a3849e3ee0fc45cbbff8db6c45a5d2d7acfd3f3375aaca916b4dbed7afdebd91` | viewBox=0 0 16 16；默认 16px | 有：copy | `plugins/dsconversation/message_area.go 复制` | MIT |
| [`icons/messages/branch.jsx`](../assets/official/icons/messages/branch.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconBranchOutlineRegular` | JSX | `5090854ac51aa941cdd4d84ce37a39df0bb36638f90ab334af7a642f3558798b` | viewBox=0 0 16 16；默认 16px | 有：branch | `plugins/dsconversation/message_area.go 分支` | MIT |
| [`icons/messages/thumb-up-outline.jsx`](../assets/official/icons/messages/thumb-up-outline.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconLikeOutlineRegular` | JSX | `e8542dde5621affff0b15430f8a53330080917b1a5d11a72a41424de296c7124` | viewBox=0 0 16 16；默认 16px | 有：thumb-up-outline | `plugins/dsconversation/feedback.go 赞未评分` | MIT |
| [`icons/messages/thumb-up-fill.jsx`](../assets/official/icons/messages/thumb-up-fill.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconLikeFillRegular` | JSX | `823267add6304fbd612f9d6be3f6c3b8ff019f31d2f7ba52d848cb89e76e5813` | viewBox=0 0 16 16；默认 16px | 有：thumb-up-fill | `plugins/dsconversation/feedback.go 赞已评分` | MIT |
| [`icons/messages/thumb-down-outline.jsx`](../assets/official/icons/messages/thumb-down-outline.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconDislikeOutlineRegular` | JSX | `78598ca4e500897f371cfa9a76b995a695257308631087a8bd75750d93125541` | viewBox=0 0 16 16；默认 16px | 有：thumb-down-outline | `plugins/dsconversation/feedback.go 踩未评分` | MIT |
| [`icons/messages/thumb-down-fill.jsx`](../assets/official/icons/messages/thumb-down-fill.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconDislikeFillRegular` | JSX | `c01092bf440cdfb3a0ced60f01dfebfaf61fd3703f5060272c6c502b2c90c7a2` | viewBox=0 0 16 16；默认 16px | 有：thumb-down-fill | `plugins/dsconversation/feedback.go 踩已评分` | MIT |

### 3.7 消息 / 工具 JSX

| 产物 | 官方来源路径 / 符号 | 类型 | sha256 | 尺寸 / viewBox | dsassets 同物 | 复用目标组件 | 许可 |
|---|---|---|---|---|---|---|---|
| [`icons/tools/api.jsx`](../assets/official/icons/tools/api.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconApiOutlineRegular` | JSX | `08ac75bf8ab9ba4ba783a77f24ac69045b8a3e419b00620e2575ea1e80196dad` | viewBox=0 0 16 16；默认 14px | 否 | `plugins/dsconversation/message_area.go 命令/终端工具` | MIT |
| [`icons/tools/browse.jsx`](../assets/official/icons/tools/browse.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconBrowseOutlineRegular` | JSX | `aeffe8eb753d2127311a6274b5c549187eecfcdee38c585d6adfd399434c68d8` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dsconversation/message_area.go 读取/抓取工具` | MIT |
| [`icons/tools/edit.jsx`](../assets/official/icons/tools/edit.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconEditOutlineRegular` | JSX | `ef277a97334931a3d252f4694056c672439521bd55d8e7b1f997a77bfda26031` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dsconversation/message_area.go 写入/编辑工具` | MIT |
| [`icons/tools/code.jsx`](../assets/official/icons/tools/code.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconCodeOutlineRegular` | JSX | `6fd4e0149e71353fa443c428aa18a450fbefa7b95dc57635d81860bbc68ccc8d` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dsconversation/message_area.go 代码工具` | MIT |
| [`icons/tools/sparkle.jsx`](../assets/official/icons/tools/sparkle.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconSparkleRegular` | JSX | `0496e421e2b488a170ca01919a36e7e19a269139c6f429b7dfd5638ea08123ce` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dsconversation/message_area.go 其他工具` | MIT |
| [`icons/tools/think.jsx`](../assets/official/icons/tools/think.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconThinkOutlineRegular` | JSX | `fe37706153cd066139c3af6310e5d7351f9bbfb04dac321dca749e5d493fbc09` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dsconversation/reasoning.go 思考` | MIT |
| [`icons/tools/question.jsx`](../assets/official/icons/tools/question.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconQuestionOutlineRegular` | JSX | `89668344eeaf1151a5f838e863c5deb0f859578a8e1b746f3127fa1f7a720035` | viewBox=0 0 16 16；默认 14px | 否 | `plugins/dsquestions/questions.go 问题工具` | MIT |
| [`icons/tools/agent-preset.jsx`](../assets/official/icons/tools/agent-preset.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconAgentPresetOutlineRegular` | JSX | `507f11511a3e72cd535db3a911cd8fde994862d72e66735bdfabcfcceec59d44` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dsconversation/message_area.go 子代理工具` | MIT |
| [`icons/tools/goal.jsx`](../assets/official/icons/tools/goal.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconGoalOutlineRegular` | JSX | `aca46bf4068391abddb67c70b4af3c84c12a46144e587de4361ca22f14584ea0` | viewBox=0 0 16 16；默认 16px | 否 | `plugins/dsconversation/message_area.go goal 工具` | MIT |
| [`icons/tools/checklist.jsx`](../assets/official/icons/tools/checklist.jsx) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js#IconChecklistOutlineRegular` | JSX | `1890f4e51b2d3410f5259e33ae44221023cc4e9b3e7efc158ca068f7c3d1d80c` | viewBox=0 0 16 16；默认 14px | 否 | `plugins/dsconversation/message_area.go 任务工具` | MIT |

### 3.8 字体与随附许可证

| 产物 | 官方来源路径 / 符号 | 类型 | sha256 | 尺寸 / viewBox | dsassets 同物 | 复用目标组件 | 许可 |
|---|---|---|---|---|---|---|---|
| [`fonts/montserrat-light.woff2`](../assets/official/fonts/montserrat-light.woff2) | `/dsh/node_modules/@deepseek-ai/dsh-client-ui-theme/lib/styles/montserrat-light.woff2` | WOFF2 | `da25f1327eb1dcc70c193897b5d7e00d31821d9518c0a49e073ff50d97a3f10a` | 不适用（Montserrat light，weight 300） | 否 | `plugins/dsconversation/hero.go 品牌字；普通 UI 保持系统字体栈` | OFL 1.1 |
| [`fonts/montserrat-regular.woff2`](../assets/official/fonts/montserrat-regular.woff2) | `/dsh/node_modules/@deepseek-ai/dsh-client-ui-theme/lib/styles/montserrat-regular.woff2` | WOFF2 | `0f4a15e29a9535af580a64db967929a8d6b202fad9a467be390619a591d2c4c5` | 不适用（Montserrat regular，weight 400） | 否 | `plugins/dsconversation/hero.go 品牌字；普通 UI 保持系统字体栈` | OFL 1.1 |
| [`fonts/montserrat-medium.woff2`](../assets/official/fonts/montserrat-medium.woff2) | `/dsh/node_modules/@deepseek-ai/dsh-client-ui-theme/lib/styles/montserrat-medium.woff2` | WOFF2 | `8a81ca6f05cb6831d5487e4b287ce5a625a718c39226b736f155900adc394166` | 不适用（Montserrat medium，weight 500） | 否 | `plugins/dsconversation/hero.go 品牌字；普通 UI 保持系统字体栈` | OFL 1.1 |
| [`fonts/brand-font.css`](../assets/official/fonts/brand-font.css) | `/dsh/node_modules/@deepseek-ai/dsh-client-ui-theme/lib/styles/brand-font.css` | CSS | `d0733ee69620e686561800ac788e54fc0bb9e02af5294c8b6f74c109132210cf` | 不适用（@font-face；引用同目录 3 个 WOFF2） | 否 | `Web 兼容层/字体加载清单；不作为普通 UI 字体` | OFL 1.1 |
| [`fonts/Montserrat-OFL.txt`](../assets/official/fonts/Montserrat-OFL.txt) | `/dsh/node_modules/@deepseek-ai/dsh-client-ui-theme/lib/styles/Montserrat-OFL.txt` | TEXT | `861d64229728cd65a98d119612c12769582574d1a092f0b63c8b0fd474e7065f` | 不适用（许可证） | 否 | `Montserrat 再分发随附许可证` | OFL 1.1 |
| [`licenses/DeepSeek-MIT.txt`](../assets/official/licenses/DeepSeek-MIT.txt) | `/dsh/node_modules/@deepseek-ai/dsh-web-frontend/LICENSE` | TEXT | `ebb4f09972aee8608be255debaf78451a68e95c290f55c240dec2ecfa16ea6be` | 不适用（许可证） | 否 | `全部 DeepSeek MIT 素材随附许可证` | MIT |

## 4. 明确不伪造的缺项与特殊边界

1. **Application / Edit 没有菜单图标**：`/lib/preload-app.cjs` 的 `installWindowsMenu()` 用 `buttons[0].textContent = messages.application`、`buttons[1].textContent = messages.edit`，CSS 只设文字按钮。目标 `plugins/dstopbar/topbar.go` 应复用文字与原生菜单行为，本任务不添加“应用/编辑”假图标。
2. **审批条没有独立警告 SVG**：`dsh-client-ui-approval/lib/client.js` 的 strip 是 `StateDot(state="warning") + 文案`，允许/拒绝是文字 Button。目标 `plugins/dscomposer/approval.go` 不拿 warning 图标冒充官方审批图标。
3. **primitives 没有独立 SVG 文件**：asar 中没有 `@deepseek-ai/dsh-client-ui-primitives` 独立资源目录，官方图标压入 `index-BXV-oxvX.js`。所以表中保留 JSX，不生成虚构的 `Icon*.svg`。
4. **更新关闭 ≠ 设置关闭**：`settings/update-close.svg` 是 `/renderer/update-close.svg` 的真实 14×14 固定色图元；`icons/settings/close-outline.jsx` 是 primitives 16 viewBox 的另一组件，二者不互称同物。
5. **现有近似不改成“官方完成”**：`composer-stop` 的官方图元是 `<rect rx=3>`；`copy` 含 `<rect rx=2>`；`branch` 含两个 `<circle>`。本任务只保存官方原文，不改当前 Go 近似/能力边界。
6. **品牌同物位置**：当前鲸鱼路径已在 `plugins/dsbrand/**`，不在 `plugins/dsassets/**`；表中照实写“否”。running APNG 只在具备动画解码与主题遮罩能力时接入，否则保留静态官方鲸鱼。

## 5. CSS 引用闭环

[brand-font.css](../assets/official/fonts/brand-font.css) 只含三个相对 URL：
- `./montserrat-regular.woff2` → [真实文件](../assets/official/fonts/montserrat-regular.woff2)；
- `./montserrat-light.woff2` → [真实文件](../assets/official/fonts/montserrat-light.woff2)；
- `./montserrat-medium.woff2` → [真实文件](../assets/official/fonts/montserrat-medium.woff2)。
本任务没有新增引用 SVG/PNG 的 CSS；因此不存在指向虚构 SVG/PNG 的 CSS URL。APNG 来源 CSS 使用 data URI，解码后的真实文件见 `brand/running-whale.png`。

## 6. 验收命令与未验项

在仓根执行：
```bash
python scripts/doc-links.py
python scripts/doc-links.py --check
git diff --check
# 另用 manifest 逐项重算 sha256、检查字节数、源文件哈希、CSS URL 与同物 JSX。
# patch 用 git diff --no-index --binary 对每个新增文件生成，再用 reverse apply 校验。
```
实际原样输出由 W101 在交付消息与 Lead 通知中附上；文档链接、manifest 哈希和 patch 必须在生成后复跑。

未验项：
- 未启动 GUI，未做官方/原生同尺寸像素对照，未验证 APNG 在 Go/Viva 中的动画解码。
- 未把新 JSX 登记进 `dsassets`，未验证每枚新图标的 Viva path 词汇覆盖；现有 `rect/circle` 限制保持原状。
- 未做字体渲染、字形覆盖或平台字体回退测试；WOFF2 只核验来源、哈希和 CSS 相对引用。
- 未验证安装包升级后的路径稳定性；chunk 文件名与哈希只钉住 `0.2.1-rc.5` 当前安装态。
- 未把 MIT 解读为商标许可；品牌公开分发/宣传口径仍需项目维护者另行决定。
