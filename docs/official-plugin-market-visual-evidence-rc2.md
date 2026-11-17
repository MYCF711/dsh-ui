# 市场面板视觉复刻证据 · rc.2

> 2026-11-17。只核几何、样式与官方素材；不扩展安装/更新能力，不改 parity 状态。
> 官方市场根：`OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai`。
> primitives 源码来自官方开源仓库 `deepseek-ai/deepseek-harness` 的 `dsh-v0.2.2-rc.2` 标签
> （[Input](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.2-rc.2/packages/client/ui-primitives/src/Input.module.css)、
> [Pill](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.2-rc.2/packages/client/ui-primitives/src/Pill.module.css)、
> [Tag](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.2-rc.2/packages/client/ui-primitives/src/Tag.module.css)、
> [Button](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.2-rc.2/packages/client/ui-primitives/src/Button.module.css)）。

## 1. 旧值 → 官方值

| 部分 | 旧实现 | 本次官方值 | 官方出处 | 落点 |
|---|---|---|---|---|
| 卡片 | padding 8、gap 14、左右 margin -8、48px 框 / 0.5px；框内首字 18/22 | 几何不变；框内改 **36×36 官方默认插件图** | `DiscoverSection.module.css:106-114,122-132`；`DiscoverSection.tsx:27-28,44-51` | `plugins/dsmarket/discover.go:686,801` |
| 卡片文字 | 标题 14/20/500、说明 13/18、meta 12/18 | 核对一致，未改 | `DiscoverSection.module.css:155-170,188-196,41-53` | `plugins/dsmarket/discover.go:692-712` |
| 搜索框 | Viva 默认 Field，无官方高度/内距；无搜索图标 | **32 高、0 8 padding、6 gap、0.5 边框、16 图标槽 / 14 glyph、14/22 输入文字** | `Input.module.css:1-9,16-23,25-38`；`DiscoverSection.tsx:512-514` | `plugins/dsmarket/discover.go:401-421` |
| 分类 Pill | 18 高、0 7 padding、10 字号、BgLayer3 / MarkdownTag | **24 高、0 8、12/18、999 圆角、BgLayer2；选中 ghost-active-fill + 1px active-border** | `Pill.module.css:1-14,26-30` | `plugins/dsmarket/discover.go:602-620` |
| Tag | 18 高、0 7、10/18、普通字重；tertiary 容器色 | **1px 8px（总高 19）、11/17/500、999 圆角；neutral=module-platform，success/info=10% tint，warning=12% tint** | `Tag.module.css:4-14,26-29,39-56` | `plugins/dsmarket/detail.go:78-88,363-384` |
| small Button | 索引菜单 14/22，筛选/排序 13/20；其余 Viva 默认或自造 MD 圆角 | **28 高、0 10、4 gap、12/18、radius-sm；16 图标槽；outline 0.5 / BorderL3** | `Button.module.css:2-16,28-34,56-60,78-84` | `plugins/dsmarket/discover.go:739-753` |
| 详情标题/描述 | 20/28/500；14/22，secondary | 核对一致，未改 | `DiscoverSection.module.css:277-290` | `plugins/dsmarket/detail.go:252-280` |
| 详情头 | 没有图标 | 48 框 + 36 官方默认图，head gap 14 | `DiscoverSection.tsx:243-245`；`DiscoverSection.module.css:271-275` | `plugins/dsmarket/detail.go:252-255` |
| notice | 只有文字，无图标 | 增加 **14px warning、8 gap、2 top**；padding 10/12、12/18 不变 | `DiscoverSection.module.css:78-95`；`DiscoverSection.tsx:256,349` | `plugins/dsmarket/detail.go:302-315` |
| 空态 | 无插画，标题/按钮无额外 margin | 48px 官方插画；标题与按钮各 **margin-top 8** | `DiscoverSection.tsx:609-619`；`DiscoverSection.module.css:216-234` | `plugins/dsmarket/discover.go:376-397` |
| 骨架 | 额外可见 loading 文案；bar 无 margin/宽度 | 无字骨架；bar **12 高、4 0 margin、144/60%、280/85%**；bg-skeleton | `DiscoverSection.module.css:397-429`；`design-platform.css:192,344` | `plugins/dsmarket/discover.go:336-362` |

## 2. 官方素材

统一原文件：`OFF:dsh-web-frontend/dist/assets/index-ghOpC9uV.js:129`；
SHA-256 **`061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8`**。
实现：`plugins/dsassets/market.go:15-86`。

- `refresh-outline`、`warning-outline`、`trash-outline`、`plus-outline`：均 **viewBox 0 0 16 16**，使用原始 `d`，regular stroke=1。
- `PluginArtworkDefault`：**viewBox 0 0 36 36**，原始 path + `userSpaceOnUse` 线性渐变，端点 `(15.1481,9.94188) → (15.1481,13.6375)`，颜色 `#54ECE7 → #658EFF`。
- 搜索复用 `WorkspaceSearch`，过滤复用 `WorkspaceSlidersTwo`（path + 两枚官方圆环），箭头复用 `ChevronDownOutline`；没有手画替代图标。

## 3. 验证与仍未覆盖

- `plugins/dsmarket/visual_geometry_test.go`：逐项钉住官方几何、渐变与主题令牌。
- `TestDiscoverDrawsToolbarAndCards` 增加真实帧命中区高度断言：Input=32、Pill=24、small Button=28，Tag 字号=11；刷新必须有命中区且不得画文字。
- 仍未覆盖：`entry.icon` 的 URL/data URL 图片加载；卡片 hover 与 toolbar 窄宽度 wrap；管理索引源菜单项的 sliders 复合图标；安装/更新按钮（宿主能力仍未接）；详情文档自己的 border/max-height 内滚动；OS 真实窗口截图与鼠标逐像素对拍。
