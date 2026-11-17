# 终端面板视觉复刻证据 · rc.2

> 2026-11-17。范围是几何、样式与官方素材，不把 ANSI 文本投影冒充完整 xterm。
> 官方根：`OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai`。
> `Button.module.css` 来自官方开源仓库 `dsh-v0.2.2-rc.2` 的 `packages/client/ui-primitives/src/Button.module.css`。

## 1. 旧值 → 官方值

| 部分 | 旧实现 | 本次官方值 | 官方出处 | 实现 |
|---|---|---|---|---|
| root | 四边 14/14/12/14、gap 4，文字沿用各控件 | `height:100%`、`min-height:0`、**padding-top 8、font 13** | `dsh-client-ui-sidebar-terminal/src/client/terminal.module.css:1` | `plugins/dsterminal/terminal.go:163-166` |
| screen | 没有独立 screen；输出 12px、非等宽、label-secondary | **四边 padding 8、bg-base、label-primary、等宽 13、lineHeight 1（13px 行盒）** | `terminal.module.css:2-4`；`terminal.tsx:95-103,185`；`ui-theme/src/theme-settings.ts:57-68`；`lib/client.terminal.js:8` 的 `lineHeight:1` | `terminal.go:204-220` |
| status / error | 普通 12px tertiary/secondary 文本，无官方容器 | status **gap 8、padding 5×12、bg-l2**；error **padding 10×12、bg-l2**；均 13px primary | `terminal.module.css:5-6` | `terminal.go:176-203,222-231` |
| tab 标题 | 只有文字，无官方 TerminalIcon | **16×16** prompt 图标；tab 保持 28 高、80–170 宽、0×10、13/18.2 | `TerminalIcon.tsx:8-9`；dockkit `dsh-web-frontend/dist/assets/index-CSAdOBx6.css:1` | `terminal.go:49-56`；`internal/uikernel/view_tabs.go:307-316` |
| tab 关闭 | 已复用官方 close-fill，无文字 × | **20×20、top/right 4、图标 14**，active/hover/focus 显示，核对不变 | dockkit `index-CSAdOBx6.css:1`；sprite `index-ghOpC9uV.js:129` | `internal/uikernel/view_tabs.go:326-342`；`view_tab_icons.go:10-20` |
| small Button | Viva 默认 Small；新建是 ghost 文本「新建」 | **28 高、0×10、gap 4、12/18、radius-sm、outline 0.5**；primary 新建带 16px plus | `Button.module.css:2-16,28-34,36-50,56-60,78-84`；`terminal.tsx:31-35` | `plugins/dsterminal/visual.go:75-106` |
| rename | Viva 默认输入框 | **width 120、min 48、padding 0×4、border 0.5、radius-sm、font 13** | `TerminalTitle.module.css:2` | `terminal.go:238-245` |

## 2. 官方素材

- tab：`TerminalIcon.tsx:9` 的两条 path `M3 4L7 8L3 12`、`M9 12H13`，viewBox 16、stroke 1；原文件 SHA-256 `62e47b45856a202c4b99cb2e3af7c19e650dc8d30eca4d498edaf083ef391ea3`。实现 `plugins/dsterminal/icons.go`。
- 新建：复用 `dsassets.MarketPlusOutline` 中已登记的官方 `IconPlusOutlineRegular` 两条 path（`M8 2V14`、`M2 8H14`），不复制坐标。
- 新建/关闭 tab：沿用右栏已有的官方 `plus-outline` / `close-fill`（`internal/uikernel/view_tab_icons.go`），没有字符或手画替代。
- 官方 terminal 包没有独立「设置」工具栏按钮；本轮不新增设置图标或按钮。终端的常驻新建/关闭在 dockkit tab strip，不另造一条 body toolbar。

## 3. 其余 CSS 逐项结论与边界

- `terminal.module.css:7-9`：empty 为 24 padding、16 gap，emptyMessage 为 `28ch/1.6`。本仓状态子集没有官方 `issue=missingTerminal`，本轮不伪造该空态；actionIcon 的 16px 按钮图标槽已在结束态新建按钮复用。
- `TerminalTitle.module.css:1-3`：标题 ellipsis、120/48/4/0.5 的 rename 与 inline-flex badge 已核。实时标题仍由 `RightTabTitleProvider` 提供；Agent/沙箱 badge 与 tab 内双击改名尚未迁入通用 tab 标题，F6/重命名控件作为既有 derived 能力保留，不冒充官方 xterm 结构。
- `TerminalGuide.module.css:1-12`：entry 56/14×20/14、icon 26、title 14/1.4、description 11/1.4、trigger 20/14 已核；Shell guide 尚未实现，本轮不新增假 guide。
- `TerminalCleanup.module.css:1-3`：stack 20/20、notice 12×16/12/13 已核；官方 `src/client/index.ts:18-19,165-171` 仍注释禁用 cleanup/recovery，因此不在产品里新造通知叠层。
- 正式画面仍是 ANSI 剥离后的文本投影；VT 光标、颜色、滚动缓冲区、原始键盘输入与 resize 不属于这次几何复刻已完成项。

## 4. 验证

- `plugins/dsterminal/visual_test.go` 逐项钉住几何与 path，并真画一帧验证 13px 等宽输出、13px 行盒、(8,16) 首行、primary 色、status 居中与 28px 按钮命中区。
- `internal/uikernel/view_tab_icon_size_test.go` 验证终端 16px 图标经名录投影，未声明尺寸的浏览器仍走原 14px 默认。
- 全量门禁输出保存在 `DSH-GO/tmp/GO8-RC2/_vcterm/`。
