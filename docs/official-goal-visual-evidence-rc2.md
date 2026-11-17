# 目标面板视觉复刻证据（rc.2）

官方来源：`dsh-client-ui-goal/src/client/GoalBar.module.css`、`GoalCommandInputView.module.css`、`GoalBar.tsx`、`locales.ts`。实现位于 `plugins/dsgoal`；`/goal` 命令回显的官方渲染器虽属 goal 包，本仓落在 `plugins/dsconversation` 的 transcript 分派中。

## 1. 数值对照

| 部件 | 旧实现 | 官方值 / 本版实现 | 官方出处 |
|---|---|---|---|
| GoalBar `.dock` | 左右各 8px margin | 左右各 **16px**；座位已扣 2×clearance，再扣 4×8 inset | `GoalBar.module.css:5-17` |
| `.bar` 宽度 | `contentWidth`，不变 | `card-max-width - 4×8 = (content+32)-32 = content` | `GoalBar.module.css:26-30`；`dscomposer/composer.go:94-99` |
| `.bar` 几何 | official 已一致；legacy 为 8px gap、6/16 padding、无 36px 卡 | 两座位统一 **36px 高、10px gap、4/5/4/12 padding、radius-md** | `GoalBar.module.css:19-35` |
| `.bar` 材质 | `BgLayer2` | `Menu = --dsw-specific-menu`；亮 `rgba(252,253,254,.70)`，暗 `rgba(48,49,54,.64)` | `GoalBar.module.css:37-46`；`design-platform.css:316-318,467-469` |
| `.bar` 投影 | Viva `ShadowXS`（0/1/2/.05） | **0.5px BorderL1 stroke + 0/3/8/.03 + 0/0/16/.02** | `GoalBar.module.css:31-34`；`gradient-shadow-text.css:33-35` |
| 活跃标签 | 「目标」；legacy 12px/tertiary | **「进行中的目标」、13/24/500、label-primary** | `GoalBar.module.css:56-62`；`locales.ts:5` |
| 目标正文 | `LabelSecondary`；legacy 12px | **13/20、label-primary-dimmed、单行省略** | `GoalBar.module.css:64-73`；`design-platform.css:256,408` |
| `/goal` 回显 | 居中、整行 Mono、label-secondary、无气泡 | **右对齐 stack、6px gap、0.702/82% cap、10×16 bubble、14/22、label-primary** | `GoalCommandInputView.module.css:1-33` |
| `/goal` chip | 与正文同为 secondary Mono | `/goal` 用 business-primary，正文保持 label-primary；command appearance 没有官方图标，不补自造图标 | `GoalCommandReference.tsx:10-11`；`ReferenceChip.module.css:3-9` |

## 2. 状态与进度边界

- 官方可见阶段是 `active / active.disarmed / paused / blocked`，都复用同一个 `.label` 样式；没有另造状态点或状态色（`GoalBar.tsx:30-41,130-139`）。
- **complete 官方整条隐藏**（`GoalBar.tsx:86-87`），本版继续以空目标不画；当前 `Actions.Goal` 只提供活跃目标字符串，没有 paused/blocked/activation 投影，因此不伪造这些阶段文案。
- rc.2 goal 包两份 CSS **没有进度条或百分比**。`GoalSnapshot` 的可见消费也没有 progress 字段；本版不添加进度轨或 `%`。
- 编辑、清除、暂停/恢复按钮需要真实动作与阶段数据；本任务不添加死按钮。

## 3. 官方素材

`assets/official/icons/tools/goal.jsx:1-4` 的四条 path 由 `plugins/dsassets/goal.go` 原样登记并经统一 `Build/ParsePath` 构建；`plugins/dsgoal/icons.go` 只调用 `dsassets.GoalOutline`。显示规格为 **viewBox=16、stroke=1、size=14**（`GoalBar.tsx:137`），插件内不再逐点写坐标。

## 4. 验证与剩余

- `plugins/dsgoal`：真实 flex/headless 帧断言 36px 卡、12px 左内边距、14px glyph、两层 elevation、13px 标签/正文、primary-dimmed、specific-menu 与 dock 16px。
- `plugins/dsconversation`：真实画布断言 `/goal` 气泡、chip/正文字号与颜色；`plugins/dsassets` 断言四条官方 path 的 34 条命令。
- 剩余：Viva 未提供 CSS `backdrop-filter: blur(40px) saturate(150%)` 的等价背景采样（`gradient-shadow-text.css:20`）；本版使用官方菜单填充色与投影，不谎称已实现模糊材质。未做 OS 真实窗口像素对拍。
