# 问题面板视觉复刻证据（rc.2）

> 基线：`dsh-client-ui-user-questions/src/client/QuestionComposer.module.css:1-645`、
> `QuestionReplyView.module.css:1-81`、`PlanReviewPanel.module.css:1-109`。
> 官方包名不是任务简写的 `dsh-client-ui-questions`；rc.2 asar 与上游 tag 均为
> `dsh-client-ui-user-questions`。计划审阅几何已在 #113 对齐，本轮未改其数值。

## 1. 通用问题卡

| 部分 | 旧实现 → 官方值 | 官方出处 | 实现 |
|---|---|---|---|
| header / eyebrow | 自造圆点与「在等你回答」→ `question.header`；标题移出 header；eyebrow 间距 **4→5**；header 主间距 **16** | `QuestionComposer.module.css:79-106`、`QuestionComposer.tsx:469-519` | `plugins/dsquestions/question_flow.go` |
| 问题标题 | header 内普通正文 → 正文区 **margin 5/24/0、16/22/500、最多 3 行**；≤720px **15/21** | `QuestionComposer.module.css:101-140,603-619` | 同上 |
| options | 间距 **6→1**；补 **margin-top 8**、上下 **padding 4**；桌面左右 **12**、窄窗 **8** | `QuestionComposer.module.css:234-240,612-614` | 同上 |
| option | 补 **min-height 40**；padding **8/8/8/8→8/12/8/8**；窄窗 **8/6**；背景 `BgLayer2`→透明，选中/悬停 `interactive-bg-hover` + **0.5px** 选中边 | `QuestionComposer.module.css:242-286,474-505,621-624` | 同上 |
| option 文案 | label 补 **24px 行高/500**；description **12→13px、20px 行高/400、最多 2 行** | `QuestionComposer.module.css:363-398` | 同上 |
| 单选序号 / 多选框 | 自造 F7/F8 文本→**20×20** 序号位、**2px** 顶部补偿、**12/18/500**；多选 **14×14、0.5px、radius-xs**，选中 check **12** | `QuestionComposer.module.css:288-344`、`QuestionComposer.tsx:588-594,644-656` | 同上 |
| 自定义输入 | 缺失→inline **40px min-height、14/24、0 padding**；optionless **64px min-height、144px max、8/12 padding、0.5px border、radius-lg** | `QuestionComposer.module.css:470-582` | 同上 |
| header / pager 图标 | 缺失→**24×24、gap 4、999px round、14px glyph**；progress **14/24/500、0/4 padding** | `QuestionComposer.module.css:56-64,174-223` | 同上 |
| footer | 缺失→**margin-top 12、padding 0/10/0/18、gap 12、actions gap 12**；≤720px **0/10 + flex-end**；md Button **36/14/14/22** | `QuestionComposer.module.css:584-601,630-637`；共享 Button `index-CSAdOBx6.css:1` | 同上 |
| card | 补 **min(60vh,520px)**、`--dsh-chat-content-width` 居中、overflow clip 与官方 elevation-panel | `QuestionComposer.module.css:12-36` | 同上 |

序号底新增主题令牌 `BgOverlay`：亮色 `neutral-bluish-150`、暗色 `neutral-bluish-700`，
对应 `dsh-client-ui-theme/src/styles/design-platform.css:191,343`，实现于
`pluginapi/theme.go` 与 `plugins/dscore/dscore.go`。check 前景复用与官方
`label-primary-foreground` 同值的 `ButtonPrimaryFg`（`design-platform.css:257,409`）。

## 2. 已回答气泡

旧实现仅把 JSON 与 `answered` 当两段左对齐正文（行距 4）。现按官方 `QuestionReplyView`
改为右对齐共享用户气泡：row **gap 6**、QA 组间 **12**、question 内 **3**、answer **margin-top 4**；
actions **28 高 / gap 8 / right 4**，time **13/24 / right 4**，copy **28×28 / padding 6 / glyph 15**。
出处：`QuestionReplyView.module.css:1-81`、`QuestionReplyView.tsx:26-61`；实现：
`plugins/dsconversation/official_question_reply.go`。气泡宽度直接复用既有
`userStackMaxWidth`，其公式与官方 `min(content×0.702,82%)` 相同。

## 3. 官方素材

- `IconChevronDownOutlineRegular` / `IconCheckOutlineRegular`：复用 `dsassets` 既有官方路径，
  镜像原件 `assets/official/icons/composer/{chevron-down,check}.jsx:1`。
- `IconChevronLeft/RightOutlineRegular`、`IconCloseOutlineRegular`：`assets/official/icons/settings/{chevron-left,chevron-right,close-outline}.jsx:1-2`。
- `IconChevronUpOutlineRegular`：rc.2 `index-ghOpC9uV.js:129` 的 `chevron-up-outline` 原 path。
- 自定义答案 `IconEditOutlineRegular`：复用 `dsassets.PlanReviewEdit`，原件 `assets/official/icons/tools/edit.jsx:1-2`。
- 已回答复制：`IconCopyOutlineRegular` 的官方 rounded rect + `copyBackPath`；复制完成复用 `IconCheckOutlineRegular`。

## 4. 验证与未做

- `plugins/dsquestions/question_flow_geometry_test.go`：常量、真实画布字号、选择/结构化回答、图标命令数。
- `plugins/dsconversation/official_question_reply_test.go`：回复几何、真实气泡文字与颜色、官方 copy path。
- 未删测试、未放宽断言、未改 golden。原「在等你回答」断言改为更严格的 `question.header` 与禁止自造文案断言。
- 未做 OS 真实窗口逐像素对拍；CSS scroll-timeline 的长文案渐隐/展开按钮、textarea mirror 的逐行自动增高未在 Viva 复刻；
  当前以官方行数上限与 **144px** 输入高度上限收口，不宣称动态效果已对齐。
