# V1 视觉证据：官方品牌欢迎区与主区导航

## 0. 结论与版本

- Base rev：`bf7e8885a73a21d839d46c0684eab41d6e20f5e1`。
- 最终抓图 UI rev：`a0ada779f1e4c87c6142364327187c56484cec6b` + 本文列出的 mono 胶囊工作树补丁。
- 最终抓图使用真实 `dshd serve`，宿主 rev `e49bc223f93f0e2731eb8df73dda8bf3419021db`；不是 mockd 画面。
- 空态已由「← 左边选一个会话」替换为官方鱼标、「探索未至之境」、「预览版」与真实当前工作区标签。
- 空 Session Header 隐藏标题、模型/权限、条数和分割线，只保留有真实 action 且右栏 slot 存在的 corner 控件；活动会话使用官方 14/20 当前 crumb。
- 本文的 ≤8px 结论只覆盖下列**可由官方 CSS 计算的关键容器指标**。这些实测偏差为 0～0.0045px。官方主区同尺寸参考图未提供，绝对窗口坐标与逐像素相似度不作已通过声明。
- 同一 READY 包曾被并行派出；最终文件是合并后的实现。Lead 在过程中将一批修改收入 `f4440d0`，本 owner 未执行 commit。

## 1. 官方来源

| 项目 | 官方原文/位置 | 采用值 |
|---|---|---|
| Hero 文案 | `ui-conversation` zh：`hero.headline`、`hero.preview`，`client.js:16143-16145` | 探索未至之境 / 预览版 |
| Hero 根与 stack | `HeroShell.module.css`，模块 `WOwLGG`，`client.js:17642` | root 水平 padding 24；stack 居中 |
| Headline | 同上：`.WOwLGG_headline` | 26px / 32px / 500；列 gap 10 |
| 预览胶囊 | 同上：`.WOwLGG_previewBadge` | 12px / 18px / 500 / code font；0.5 描边；999 圆角；padding `1px 7px 0`；margin-top 2 |
| 鱼标 | `HeroFish` 与 primitives 的 `FISH_LOGO_PATH/VIEWBOX` | 34×25.0155；viewBox 23.16×17.04；3448 字节、80 条 M/L/C/Z 命令 |
| Hero 最大宽度 | `ConversationRoot.module.css` 的 body/composer 变量，`client.js:17384` | `clamp(680, main×0.64, 920)+32`，即本两档 712 / 952 |
| 工作区行 | 同文件 `.an3WzG_heroWorkspaceRow`；HeroShell `.WOwLGG_workspace` | 外层 padding 20/16；chip 高 28、左右 padding 8、max 360；13/20/500 |
| Header | 同文件 `.an3WzG_header*`、`.an3WzG_titleRow`、`.an3WzG_crumb*` | 无 tabs active 50+0.5；blank 40；sessionless 10；crumb 14/20 |
| 右栏 corner | `ui-sidebar-right` `ExpandButton.module.css` 与组件，`client.js:655-690` | 28px 控件、15px 图标；真实开合动作 |

所有颜色均来自 `pluginapi.Theme`。深色预览胶囊使用 `BgLayer3`（官方 tertiary 同为 `#292929`）、`Hover`（interactive-bg-hover）和 `LabelPrimary`；形状由 flex/`ui.Icon` primitives 表达，未在插件中写十六进制色值。

## 2. 实现与真实数据边界

| 文件 | 行为 |
|---|---|
| `plugins/dsconversation/hero.go` | 官方 Hero、动态 712/952 stack、矢量鱼标、mono 预览胶囊、真实 cwd 工作区标签、几何记录 |
| `plugins/dsconversation/brand_path.go` | 官方鱼标路径与 viewBox；不可变 `ui.Icon` 缓存 |
| `plugins/dsconversation/brand_parse.go` | 只接受官方实际使用的 M/L/C/Z；未知命令整条失败，不画半个 logo |
| `plugins/dsconversation/conversation.go` | 仅在 `len(Msgs)==0 && Stream=="" && !Busy` 时进入 Hero；消息、工具、搜索、流式与 Markdown 分支不改 |
| `plugins/dstopbar/topbar.go` | blank/active/sessionless 三种官方 Header；真实 crumb；action+slot 双门控的右栏 corner |
| `plugins/dsconversation/brand_parse_test.go` | 官方路径字节、命令数、几何、非法输入与 Icon 非空 |
| `plugins/dsconversation/visual_test.go` | Hero 文案/字体/矢量/工作区/活动隐藏/两档几何 |
| `plugins/dstopbar/visual_test.go` | blank 隐藏、无 slot 无死按钮、活动 crumb 与 Header 几何 |

工作区选择器需要当前快照没有提供的切换 Action，因此只画当前会话 cwd 的**只读标签**，没有 Hit、hover 或 chevron；没有当前会话/cwd 时整行隐藏。`conversation.hero.options` 与 `agentPreset` 同样因无真实动作而隐藏。

Busy 也是活动状态：已发送但首个正文 token 尚未到达时，不因 `Msgs` 和 `Stream` 暂时为空而闪回欢迎区。

## 3. 定向测试

新增测试先于本 owner 的实现写入；首次运行返回 exit 1，暴露空态与 blank Header 缺口。随后合并并行实现并补齐几何、Busy、slot 门控与 mono 字体验证。

最终命令：

```bash
CGO_ENABLED=0 go test ./plugins/dsconversation ./plugins/dstopbar -count=1 -v
```

实际结果：`ok github.com/MYCF711/dsh-ui/plugins/dsconversation`、`ok github.com/MYCF711/dsh-ui/plugins/dstopbar`，`GO_TEST_EXIT=0`。完整输出：[tests.txt](../plugins/dsconversation/testdata/visual-v1/tests.txt)。

几何定向命令：

```bash
CGO_ENABLED=0 go test ./plugins/dsconversation ./plugins/dstopbar \
  -run 'TestEmptyStateUsesOfficialHero|TestHeroGeometryAtOfficialWindowWidths|TestHeaderGeometryMatchesOfficialBlankActiveAndSessionless' \
  -count=1 -v
```

实际结果：`GEOMETRY_TEST_EXIT=0`。原始 Rect 记录：[geometry.txt](../plugins/dsconversation/testdata/visual-v1/geometry.txt)。

## 4. 真实首屏截图

输入是独立 home 中复制的 header-only 真实会话 `b11c7eee-8a12-7334-123b-b9070a6a8261`。真实 `session.load` 返回 `total:0`、`messages:[]`，原始响应：[real-load.json](../plugins/dsconversation/testdata/visual-v1/real-load.json)。

| PNG 外框 | `-size` 客户区 | 证据 | 数据与右栏 |
|---|---|---|---|
| 1920×1032 | 1904×993 | [current-1920x1032.png](../plugins/dsconversation/testdata/visual-v1/current-1920x1032.png) | 同一真实空会话；`-rightbar=false`；SHA-256 `82e1c57207541a914ed324c67b06daa6a1a9e2bafed66a1fa11b6412ed8b4a0a` |
| 1182×832 | 1166×793 | [current-1182x832.png](../plugins/dsconversation/testdata/visual-v1/current-1182x832.png) | 同一真实空会话；`-rightbar=false`；SHA-256 `9c157edb95ecce62df30815ec7c78df41bdc9327f0bd211ee7448901deaf6810` |

最终先构建再抓图（两个尺寸仅替换 `-size` 与输出路径）：

```bash
CGO_ENABLED=0 go build -o plugins/dsconversation/testdata/visual-v1/app-v1.exe ./cmd/app
plugins/dsconversation/testdata/visual-v1/app-v1.exe \
  -addr "$URL" -cookie "dshd_session=$CK" \
  -session b11c7eee-8a12-7334-123b-b9070a6a8261 \
  -rightbar=false -size 1904x993 \
  -shot plugins/dsconversation/testdata/visual-v1/current-1920x1032.png -shot-wait 3
```

`-size` 是客户区；Windows 外框增加 16×39，因此上述客户区得到要求的外框像素。早期 mockd 截图被其后台假事件污染，已覆盖，未作为本证据。

## 5. 逐容器几何与偏差

坐标来自真实 `flex.Box.Rect()`，不是目测。Hero 测试 root 高固定为 700 的布局夹具；root/stack 原点因而是局部坐标，不能当作官方整窗绝对 y。

### 1182 外框对应主列 886

| 容器 | 实测 Rect（局部） | 官方指标 | 偏差 |
|---|---|---|---|
| stack | `{X:87 Y:314 Width:712 Height:72}` | 宽 712 | 0 |
| headline | `{X:87 Y:314 Width:712 Height:32}` | 行高 32 | 0 |
| 鱼标 | `{Width:34 Height:25.015545}` | 34×25.0155 | <0.001 |
| workspaceRow | `{X:87 Y:358 Width:712 Height:28}` | 高 28 | 0 |
| workspace 左边 | stack x+20 | 左 padding 20 | 0 |
| stack 中心 | root 水平/垂直中心 | 双轴居中 | 0 |

### 1920 外框对应主列 1624

| 容器 | 实测 Rect（局部） | 官方指标 | 偏差 |
|---|---|---|---|
| stack | `{X:336 Y:314 Width:952 Height:72}` | 宽 952 | 0 |
| headline | `{X:336 Y:314 Width:952 Height:32}` | 行高 32 | 0 |
| 鱼标 | `{Width:34 Height:25.015545}` | 34×25.0155 | <0.001 |
| workspaceRow | `{X:336 Y:358 Width:952 Height:28}` | 高 28 | 0 |
| workspace 左边 | stack x+20 | 左 padding 20 | 0 |
| stack 中心 | root 水平/垂直中心 | 双轴居中 | 0 |

### Header

| 状态 | 实测 | 官方 | 偏差 |
|---|---|---|---|
| blank root | 高 40；无 Divider box | 10+30；无分割线 | 0 |
| blank titleRow | `{X:20 Y:10 Width:852 Height:30}` | 高 30 | 0 |
| corner | 28×28 | 28×28 | 0 |
| active root | 高 50.5 | 50+0.5 | 0 |
| active Divider | y=50，高 0.5 | 0.5 | 0 |
| sessionless root | 高 10 | 10 | 0 |

上述负责指标均远低于 8px。

## 6. 字体与行高

| 元素 | 实际断言 | 官方来源 | 状态 |
|---|---|---|---|
| Hero headline | Canvas `Size=26`、Bounds 高 32、`WeightMedium` | 26/32/500 | 已验 |
| 预览胶囊 | Canvas `Size=12`、`Mono=true`；20 外高减 1 描边总量与 1 上 padding = 18 内容行 | 12/18/500/code font | 已验 |
| 工作区 | 13/20/`WeightMedium`；chip 高 28 | 13/20/500 | 已验 |
| 当前 crumb | Canvas `Size=14`、Bounds 高 20、`WeightMedium` | 14/20/500 | 已验 |

Viva 的 mono 文本忽略 `Text.LineHeight`，所以胶囊用固定 20px 容器表达 CSS 行盒，避免把 15px glyph 框误报成 18px line-height。

## 7. 未验项与保留缺口

1. **未做官方逐像素/绝对坐标对比**：当前资料只有官方 CSS/组件，没有官方主区 1920×1032、1182×832 参考 PNG。本文不伪造该项 ≤8px 结论。
2. **Hero+composer 的整体垂直中心未验**：本仓 body 与 composer 是不同插件槽，当前只证明 Hero stack 在其实际 body root 内双轴居中；不宣称与官方整组绝对 y 相同。
3. **OS 字体栅格、baseline、DPI 未与官方测量**：字号、行高、weight 和 mono 路由有布局断言；实际字体文件、baseline 与栅格差异未测。
4. **浅色主题未截图**：深色胶囊三个 theme 映射有同值来源；浅色 `state-business-tertiary` 在现有 Theme 中没有独立令牌，当前使用最近档位，未宣称色值完全相同。
5. **工作区选择/agent preset/hero options 未实现**：缺真实 Action，当前为只读真实标签或隐藏，不做死按钮。
6. **右栏打开时仍保留 corner 收起入口**：官方 ExpandButton 在展开后隐藏，由右栏自己的 collapse 接管；当前 `ds-rightbar` 没有该按钮，为避免删掉唯一收起出口而保留，属于明确行为差异。
7. 截图右上 toast 属于现有 `ds-demo`，不在 V1 允许修改路径，未为美观隐藏或改它。
