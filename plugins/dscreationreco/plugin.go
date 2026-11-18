// Package dscreationreco 是官方 `dsh-client-ui-creation-recommendations` 的 Go 版：
// 「个性化页面」这张动作卡（action = `create_personalized_page`）的正文。
//
// # 结构证据（唯一权威，几何/文案/状态机全部照它）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-creation-recommendations
//	OFF:src/client/Card.tsx:18-24   detailsSchema / dataSchema（slot 枚举、customLocation、bundleName、application 五值）
//	OFF:src/client/Card.tsx:26-29   PHASE_KEYS 11 档（dismissed/replaced/creating/installing/cancelled/installed/kept/removed/failed/interrupted/unconfirmed）
//	OFF:src/client/Card.tsx:30-33   REASON_KEYS 8 条（其余 reason 一律读作 unavailable）
//	OFF:src/client/Card.tsx:35-37   APPLICATION_KEYS：应用结果四条专属文案
//	OFF:src/client/Card.tsx:43-66   LocationPicker（Menu 四项，每项「名字 + hint」两行）
//	OFF:src/client/Card.tsx:68-90   PageCardBody 的状态推导（offer / infoOpen / installed / busy）
//	OFF:src/client/Card.tsx:90-101  run()：placement 只在 create/dismiss 传；refused/unconfirmed 取 reason 文案
//	OFF:src/client/Card.tsx:102-159 header / info 说明 / body / initialContent / 两套控件（offer vs placement）
//	OFF:src/client/Card.module.css    28 行：全部几何值（本包每个常量都对着这里的一行；
//	                                  :1-16 是 header / info / body / 两套控件，:17-28 是
//	                                  `.controls .footer` / `.locationRow` / `.locationLabel` /
//	                                  `.locationButton` / `.locationMenu` / `.locationOption` /
//	                                  `.customLocation` + textarea 与它的焦点、占位态）
//	OFF:web/dist/assets/index-CSAdOBx6.css  primitives 的 `_button_1ulci_2` 块：按钮原语
//	                                  （`._sm_1ulci_28` = 28 高 / 12 字号 / 18 行高 / 0 10 内边距 /
//	                                  radius-sm）。官方 `dsh-client-ui-primitives` 不随包发 src，
//	                                  这份 CSS 是它唯一的可核原件——本包两处按钮字号由此而来。
//	OFF:src/client/locales.ts:44-78  zh 字典 58 条（本包**只许**用这 58 条原文）
//	OFF:src/client/locales.ts:44-78  heading/location/creationInfo…逐条照抄，见下方常量注释
//	OFF:src/client/info-store.ts:6-8  `infoDismissed` 是**跨会话持久**偏好（官方 localStorage）
//	OFF:src/client/mount.ts:15        `inject: ['locale', 'actionCards']`——**没有 remote**
//	OFF:src/client/mount.ts:22-27     注册 `{action, locale, Body}` 三件套
//
// # 数据面：**零新增宿主方法**
//
// 官方这个包一条 Remote 调用都没有（mount.ts:15 的 inject 里只有 locale 与
// actionCards）。卡片要什么都在 props 上：
//
//	proposal.details      → slot / initialContent（宿主 propose_action 时校验过的同一份）
//	decision.phase        → 11 档 phase（宿主 internal/creation 常量 Phase*）
//	decision.data         → slot / customLocation / bundleName / application
//	act(verb, payload)    → 共享的 `actionCards.act`（宿主四个动词由 kind 自己按 phase 分派）
//
// 所以本包**不加** apiclient 方法，`testdata/api.snapshot.json` 不变；四个动词
// （create/undo/cancel/restore）与共享的 dismiss 都经 `pluginapi.ActionCardBodyProps.Act`
// 打到**这张卡**的 callId 上（框架保证，见 dsactioncards.drawKindCard）。
//
// # 宿主证据（生产路径会真的走到的那些）
//
//	HOST:internal/creation/constants.go  Action/Skill/Slot/Phase/Application/Reason 全部字面量
//	HOST:internal/creation/service.go    act 的四个动词分派 + refused 的 reason 出口
//	HOST:internal/creation/service.go    create：没有 Creator 时**写之前**就 refused{creator-unavailable}
//	HOST:internal/recommendations/service.go 同一插件的注册面（kind 注册进 actioncards）
//
// # `refused{creator-unavailable}` 在官方文案里怎么显示
//
// 官方 `creator-unavailable` **不在** REASON_KEYS 里（Card.tsx:30-33 只有 8 条），
// 而 Card.tsx:98 的规则是 `keyOf(REASON_KEYS, reason) ?? 'unavailable'`——
// **任何其它 reason 都读作 `unavailable`**（英文注释原话：
// "Refusal reasons with their own copy; any other reason reads as unavailable."）。
//
// 所以这里逐字照官方走：`creator-unavailable` → `unavailable` =
// '暂时无法执行，请到插件页面查看'（红色反馈行在 `.status` 位置，**不是**成功态）。
// 不另写一句"宿主没有生成器"：这张卡的文案表只有官方那 58 条，多写一句就不是官方那张卡了；
// 真实原因由宿主写日志（HOST:internal/creation/service.go 的 logf）与 skill 文档承担。
// 差异登记：`unavailable` 的措辞指向"插件页"，而本宿主的实际原因是"没有 Creator"——
// 用户从卡片上看不出区别，这是**官方兜底档的语义宽度**，不改写。
//
// # 它要壳注入什么
//
// **什么都不用注入**。`Requires()` 只声明 `KeyTheme`（配色）与 `KeyActionCardBodies`
// （正文注册表）。官方 `infoDismissed` 的持久化在本仓走壳已接的 `Actions.ConfigGet/
// ConfigSet`（宿主 `<home>/state/ui-config.json`，纯 key/value），键 `ui.creationInfoDismissed`；
// 官方那份是 localStorage 键 `dsh.creation-recommendations.info-dismissed`（info-store.ts:7）。
//
// # 没搬 / 对不上的地方（照实登记，不假装搬了）
//
//  1. **flex-wrap 没有对等物**：官方 `.header` / `.controls` / `.footer` 都是
//     `flex-wrap: wrap`（Card.module.css:1/9/7），viva 的 `ui.Row` 不换行。所以
//     `.controls` 里「位置行 / 自定义输入 / 两个按钮」按官方的换行**顺序**摆成三行
//     （位置行 + 按钮同行；选了自定义时输入独占一行、按钮落到第三行右端），
//     但窗口极窄时官方会继续折行，本仓不会。
//  2. **`maxLength={1000}` 没有对等物**（Card.tsx:140）：viva `ui.TextArea` 不收长度上限。
//     超长不会静默截断——宿主 `parseLocation` 对 `customLocation` 的判据是
//     trim 后 1..1000（HOST:internal/creation/service.go），超了回
//     `refused{invalid-location}`，卡片按官方文案 '请选择位置，或填写不超过 1,000 字的位置说明' 显示。
//  3. **info 按钮的图标已是官方那一枚**。旧实现用 `IconInfoOutlineRegular` 顶替，
//     理由是「rc.2 解包树里只有使用处、没有 path 数据」——**那条结论是错的**：
//     sprite 里带完整 SVG 正文（`"warning-triangle-outline":{size:16,svg:…}`）。
//     现在 `icons.go` 存的就是官方原文（viewBox=16、stroke=1），按官方 `size={14}`
//     缩到 14px 显示。仍不同的是官方那个 `Tooltip` 在 info 展开时 `disabled`，
//     这里按同一判据**条件挂载** `ui.Tooltip`（disabled 时干脆不挂）。
//  4. **菜单没有"当前项高亮"**：官方 `Menu selectedId={value}`（Card.tsx:52），viva 的
//     `ui.Menu` 只有 `Chosen` 单选圆点 / `Checked` 勾选，那会多画一个官方没有的标记 →
//     不标（四项照常可选，选中的那一项在按钮文字上是可见的）。
//  5. **ARIA 关系没有对等物**：官方 `role=status`（phase 与应用结果行）、`role=alert`
//     （错误行）、`role=note`（info 块）、`aria-controls/aria-expanded`（info 按钮与
//     textarea）都是 DOM 属性。本仓只保留**可交互**部分的无障碍名（每个按钮的
//     `Hit` label 用官方文案，info 按钮的 label 就是 `creationInfo`），纯文本行不带角色。
//  6. **`transport` 那一档**：官方在"响应丢失"时显示 `transport`（Card.tsx:97 的
//     `if (result === undefined) setError('transport')`）。本仓 apiclient 把丢包
//     报成 error，框架以 `ActError` 交给正文，所以 `ActError != ""` → 官方 `transport`
//     文案（**不把宿主错误原文塞进这张卡**：那 58 条以外的字样不是官方卡）。
//  7. **grapheme 切分**：官方用 `Intl.Segmenter` 按字素算宽度，Go 侧按 rune 算
//     （与 dsactioncards 同一处差异），只影响折叠出现的位置，不改写原文。
//  8. **菜单项内部的三条官方声明由 viva 的 MenuItems 决定**，本包只能给到宽度：
//     `.locationMenu{ max-width: calc(100vw - 24px) }`（:22）、`.locationOption{ gap:2px;
//     line-height:1.5 }`（:23）、`.locationOption > span + span{ font-size:12px }`（:24）。
//     `WidthRange(264,264)` 已按 :22 的 width 给；三条 max-width / 行距 / 第二行字号
//     在 `ui.MenuItems` 里没有对应参数（它自己排版行与说明行），所以不改、只登记。
package dscreationreco

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Action 是官方注册的 kind 名（OFF:src/client/mount.ts:24-26 的 `action:`；
// 宿主同一常量 HOST:internal/creation/constants.go 的 `Action`）。
const Action = "create_personalized_page"

// ServiceID 是本插件的装配 id（壳与测试按它查表）。
const ServiceID = "ds-creation-reco"

// ── 文案：只有这 58 条，逐字来自 OFF:src/client/locales.ts:44-78 的 zh 字典 ────
//
// **不许新写中文**：界面上一旦出现这 58 条以外的字样，就不是官方那张卡了。
const (
	zhHeading            = "个性化页面"                                      // heading
	zhLocation           = "添加到"                                        // location
	zhCreationInfo       = "创建方式与 token 消耗"                             // creationInfo
	zhCreationMethod     = "用创造模式制作新页面，可随时移除"                           // creationMethod
	zhCreationCost       = "创建可能耗时较长，并消耗大量 token"                       // creationCost
	zhCreationBackground = "点击「创建」后在后台运行，你可以继续聊天"                       // creationBackground
	zhInitialContent     = "预填内容"                                       // initialContent
	zhExpandContent      = "展开内容"                                       // expandContent
	zhCollapseContent    = "收起内容"                                       // collapseContent
	zhSlotSidebarRight   = "右侧栏"                                        // sidebar-right
	zhSlotPage           = "会话 Tab"                                     // page
	zhSlotSidebarLeft    = "全局页面"                                       // sidebar-left
	zhChoiceCustom       = "自行撰写"                                       // custom
	zhSidebarRightHint   = "在当前会话旁查看"                                   // sidebar-rightHint
	zhPageHint           = "与「对话」「轨迹」并列"                                // pageHint
	zhSidebarLeftHint    = "从左侧入口打开"                                    // sidebar-leftHint
	zhCustomHint         = "描述你希望放在哪里"                                  // customHint
	zhCustomLabel        = "你希望放在哪里？"                                   // customLabel
	zhCustomPlaceholder  = "例如：在左侧加一个入口，打开全屏日历"                         // customPlaceholder
	zhCreate             = "创建"                                         // create
	zhStarting           = "开始创建…"                                      // starting
	zhDismiss            = "暂不需要"                                       // dismiss
	zhRetry              = "重新创建"                                       // retry
	zhUndo               = "撤销"                                         // undo
	zhRestore            = "恢复"                                         // restore
	zhRestoring          = "恢复中…"                                       // restoring
	zhDismissed          = "已忽略"                                        // dismissed
	zhReplaced           = "已替换"                                        // replaced
	zhCreating           = "正在后台创建"                                     // creating
	zhCancel             = "取消创建"                                       // cancel
	zhCancelling         = "正在取消…"                                      // cancelling
	zhInstalling         = "正在安装…"                                      // installing
	zhSourceRetained     = "撤销后，源码仍保留在原工作区的 .dsh-created-plugins/ 文件夹中" // sourceRetained
	zhInstalled          = "已安装"                                        // installed
	zhKept               = "已保留"                                        // kept
	zhRemoved            = "已移除"                                        // removed
	zhFailed             = "创建失败，可重试"                                   // failed
	zhInterrupted        = "创建中断，可重试"                                   // interrupted
	zhUnconfirmed        = "结果待确认，请重试"                                  // unconfirmed
	zhInvalidLocation    = "请选择位置，或填写不超过 1,000 字的位置说明"                  // invalid-location
	zhRestartRequired    = "变更已保存，重启 DSH 后生效"                           // restart-required
	zhOverridden         = "其他配置覆盖了本次变更"                                // overridden
	zhCancelled          = "已取消"                                        // cancelled
	zhApplicationFailed  = "上一步更改未生效，请到插件页查看"                           // applicationFailed
	zhApplicationCancel  = "上一步更改已取消"                                   // applicationCancelled
	zhTransport          = "暂时无法确认结果，请重新连接并查看卡片状态后再重试"                  // transport
	zhUnknown            = "这条推荐已不可用"                                   // unknown
	zhUnavailable        = "暂时无法执行，请到插件页面查看"                            // unavailable
	zhAccessDenied       = "当前连接没有修改插件的权限"                              // access-denied
	zhNotLiveRoot        = "请打开原会话后使用这张卡片"                              // not-live-root
	zhClosing            = "创建插件正在停止，请等待重新加载后再试"                        // closing
	zhReadOnly           = "当前会话为只读模式，创建页面需要切换权限预设"                     // read-only
	zhSourceMissing      = "页面文件已不在，需要重新提案"                             // source-missing
)

// zhExpandBrief / zhCollapseBrief 是 **brief** 的折叠开关，来自卡片框那一层的
// zh 字典（OFF:action-cards/src/client/locales.ts:16 的 `expandBrief/collapseBrief`）：
// 官方 `summary` 是 `<strong>{title}</strong><FoldedText text={brief} …/>`
// （OFF:action-cards/src/client/Card.tsx:112-115），body 收到的 `summary` 就是这个节点。
const (
	zhExpandBrief   = "展开" // expandBrief
	zhCollapseBrief = "收起" // collapseBrief
)

// ── 几何：全部来自 OFF:src/client/Card.module.css ───────────────────────────
const (
	// `.header{ gap:8px 12px; padding:12px 20px 0; font-size:12px; line-height:1.5 }`（:1）
	// → 行高 12×1.5=18。
	createdHeaderPadTop    float32 = 12
	createdHeaderPadX      float32 = 20
	createdHeaderFont      float32 = 12
	createdHeaderLineH     float32 = 18
	createdHeaderGapColumn float32 = 12
	// `.heading{ gap:4px }`（:2）、`.infoButton{ width:28px; padding:0 }`（:3）。
	//
	// `.infoButton` 的高度来自官方原语：它是 `<Button variant="ghost" size="sm">`
	// （OFF:Card.tsx:107），而 `size="sm"` 的量法是
	// `._sm_1ulci_28{height:28px; font-size:12px; line-height:18px; padding:0 10px;
	// border-radius:var(--dsw-radius-sm)}`（OFF:web/dist/assets/index-CSAdOBx6.css 的
	// primitives `_button_1ulci_2` 块）→ **28×28 方形**。viva 的 `ButtonBase`
	// 没有自带的 sm 高度，不写这一条就只剩图标那 14px 高，整个 header 会矮 10px
	// （官方 header = 12+28 = 40；漏了就 12+18 = 30）。
	createdHeadingGap  float32 = 4
	createdInfoButtonW float32 = 28
	createdInfoButtonH float32 = 28
	createdInfoIcon    float32 = 14
	// `.info{ margin:12px 20px 0; padding:12px; font-size:13px; line-height:1.7 }`（:4）
	// → 行高 13×1.7=22.1；`.info p + p{margin-top:4px}`、`.info strong{font-weight:500}`（:6-7）。
	createdInfoMarginTop float32 = 12
	createdInfoPad       float32 = 12
	createdInfoFont      float32 = 13
	createdInfoLineH     float32 = 22.1
	createdInfoParaGap   float32 = 4
	createdInfoStrong    float32 = 500
	// `.body{ padding:12px 20px 20px }`（:9）。
	createdBodyPadTop    float32 = 12
	createdBodyPadX      float32 = 20
	createdBodyPadBottom float32 = 20
	// `.content{ margin-top:12px }`（:10）、`.contentLabel{ font-size:12px; line-height:1.6 }`（:11）
	// → 行高 12×1.6=19.2。
	createdContentMarginTop  float32 = 12
	createdContentLabelFont  float32 = 12
	createdContentLabelLineH float32 = 19.2
	// `.placement{ gap:12px; margin-top:16px; padding-top:12px; border-top:0.5px solid … }`（:12）。
	createdPlacementMarginTop float32 = 16
	createdPlacementPadTop    float32 = 12
	createdBorderWidth        float32 = 0.5
	createdRowGap             float32 = 12
	// `.description{ font-size:12px; line-height:1.6 }`（:13）。
	createdDescriptionFont  float32 = 12
	createdDescriptionLineH float32 = 19.2
	// `.footer{ gap:12px; margin-top:12px }`（:14）、`.actions{ gap:8px; margin-left:auto }`（:15）。
	createdFooterGap       float32 = 12
	createdFooterMarginTop float32 = 12
	createdActionsGap      float32 = 8
	// `.status{ margin:12px 0 0; font-size:12px; line-height:1.6 }`（:16）。
	createdStatusMarginTop float32 = 12
	createdStatusFont      float32 = 12
	createdStatusLineH     float32 = 19.2
	// `.controls{ gap:12px; margin-top:16px; padding-top:16px; border-top:0.5px solid … }`
	// 与 `.controls .footer{ margin:0 0 0 auto }`（:17-18）。
	createdControlsMarginTop float32 = 16
	createdControlsPadTop    float32 = 16
	createdControlsGap       float32 = 12
	// `.locationRow{ gap:8px }`（:19）、`.locationLabel{ font-size:12px; line-height:1.6 }`（:20）、
	// `.locationButton{ gap:8px; font-size:13px }`（:21）、`.locationMenu{ width:264px }`（:22）。
	createdLocationRowGap     float32 = 8
	createdLocationLabelFont  float32 = 12
	createdLocationLabelLineH float32 = 19.2
	createdLocationChevron    float32 = 14
	createdLocationMenuWidth  float32 = 264
	// 官方 `.locationButton{ font-size:13px }`（:21）：`size="sm"` 的原语字号是 12，
	// 这个包把它**抬到 13**。viva 的 `Small()` 按主题 FontSizeM 画字（本仓 = 14），
	// 所以两档都对不上，必须逐处给 13——`ui.Box.FontSize` 对控件就是"换掉这一处的
	// 主题字号"（viva `ui/control_theme.go:30-31` 的 ownTheme）。
	createdLocationButtonFont float32 = 13
	// 官方原语 `._sm_1ulci_28{ height:28px; font-size:12px; line-height:18px; padding:0 10px;
	// border-radius:var(--dsw-radius-sm) }`（OFF:web/dist/assets/index-CSAdOBx6.css 的
	// primitives `_button_1ulci_2` 块）：卡片里其余按钮（暂不需要 / 创建 / 重新创建 /
	// 取消创建 / 撤销 / 恢复）全是 `size="sm"`。viva 的 `Small()` 自带 28 高、10 内边距、
	// 8 圆角（Radii.SM = 官方 `--dsw-radius-sm: 8px`，base.css:19），**只有字号是主题的 14**
	// （本仓主题 FontSizeM=14）——逐处给 12 才和官方一致。同一组数字本仓
	// dsmarket/discover.go:116-120 已采用（那边是手画按钮，这里是 viva 按钮）。
	createdSmallButtonFont float32 = 12
	// `.customLocation{ gap:6px }`（:25）、`textarea{ min-height:72px; padding:8px 10px;
	// font-size:13px; line-height:1.7 }`（:26）。
	createdCustomGap    float32 = 6
	createdCustomHeight float32 = 72
	createdCustomFont   float32 = 13
	createdCustomLineH  float32 = 22.1
	// rc.2 `Card.module.css:27` 的 `outline:1px solid state-business-primary` + `outline-offset:1px`。
	createdCustomFocusWidth  float32 = 1
	createdCustomFocusOffset float32 = 1
	// 卡片框那一层的 `.text` / `.toggle`（OFF:action-cards/Card.module.css:4-5）：正文与 brief
	// 折叠都是这一对几何（13/1.7 与 12px、min-height 24）。
	// 卡片框 `.title{ font-size:16px; font-weight:500; line-height:1.5 }`（OFF:action-cards/Card.module.css:3）
	// → 行高 16×1.5=24。summary 的标题那一层就是这个几何。
	createdTitleFontSize   float32 = 16
	createdTitleLineH      float32 = 24
	createdTitleWeight     float32 = 500
	createdTextMarginTop   float32 = 8
	createdTextFont        float32 = 13
	createdTextLineH       float32 = 22.1
	createdToggleMarginTop float32 = 8
	createdToggleMinHeight float32 = 24
	createdToggleFont      float32 = 12
)

// 以下两个官方值**没有对应的绘制参数**，留着是为了让后来的人不必再翻一遍 CSS：
//
//	createdHeaderGapRow = 8        官方 `.header{gap:8px 12px}` 的**行间距**——viva 的
//	                               `ui.Row` 不换行，这一档在绘制里用不到（列间距 12 用了）；
//	createdLocationOptionHintSz = 12 官方 `.locationOption > span + span{font-size:12px}`
//	                               ——菜单第二行由 `ui.MenuItems.Description` 自己排版。
//
// （原先这里还挂着 `createdLocationButtonFont = 13` 并写着"viva 的 ui.Button 字号由主题给，
// 无法逐处覆盖"——**那句是错的**：`ui.Box.FontSize` 对控件就是换掉主题字号，
// 见 viva `ui/control_theme.go:30-31`；该值现已真的用在位置按钮上。）
const (
	createdHeaderGapRow         float32 = 8
	createdLocationOptionHintSz float32 = 12
)

// createdBorderRadius 是 `.info` 与 textarea 的 `var(--dsw-radius-md)`（:4/:25）。
// 直接取主题的 Radii.MD（12）——主题那一档就是官方 `--dsw-radius-md`。

// foldWidth 是折叠预算，单位是"拉丁字母宽"（官方 `FOLD_WIDTH = 260`，
// OFF:action-cards/src/client/Card.tsx:41）。
const foldWidth = 260

// prefInfoDismissed 是 `infoDismissed` 的持久键（官方 localStorage 键见包注释；
// 本仓落在宿主 `ui-config.json`，与 `ui.sidebarWidth` 同一处）。
const prefInfoDismissed = "ui.creationInfoDismissed"

// ── 官方枚举（逐字）────────────────────────────────────────────────────────

// 官方 PageVerb 四值（OFF:src/index.ts 的 `verbs:`；宿主 constants.go 的 Verb*）。
const (
	verbCreate  = "create"
	verbUndo    = "undo"
	verbCancel  = "cancel"
	verbRestore = "restore"
)

// choiceCustom 是位置选择的第四项（官方 `type LocationChoice = CreationSlot | 'custom'`）。
const choiceCustom = "custom"

// 官方 slot 三值（OFF:Card.tsx:18 的 slotSchema；宿主 constants.go 的 Slot*）。
const (
	slotSidebarRight = "sidebar-right"
	slotPage         = "page"
	slotSidebarLeft  = "sidebar-left"
)

// locationChoices 是位置菜单的四项（顺序照 OFF:Card.tsx:54 的
// `['sidebar-right','page','sidebar-left','custom']`）。
var locationChoices = []string{slotSidebarRight, slotPage, slotSidebarLeft, choiceCustom}

// phaseText 是官方 PHASE_KEYS（OFF:Card.tsx:26-29）的 11 档逐条文案。
//
// 11 档全做：`dismissed`/`replaced` 是共享 phase（框架也会画，但正文接管后由正文画），
// 其余 9 档是宿主 `internal/creation/constants.go` 的 Phase* 全集。
var phaseText = map[string]string{
	"dismissed":   zhDismissed,
	"replaced":    zhReplaced,
	"creating":    zhCreating,
	"installing":  zhInstalling,
	"cancelled":   zhCancelled,
	"installed":   zhInstalled,
	"kept":        zhKept,
	"removed":     zhRemoved,
	"failed":      zhFailed,
	"interrupted": zhInterrupted,
	"unconfirmed": zhUnconfirmed,
}

// reasonText 是官方 REASON_KEYS（OFF:Card.tsx:30-33）的 8 条专属文案。
//
// **不在表里的 reason 一律读作 `unavailable`**（OFF:Card.tsx:98 的
// `keyOf(REASON_KEYS, reason) ?? 'unavailable'`）——宿主扩展的
// `creator-unavailable` / `not-live-root` 里，前者就落这一档（见包注释）。
var reasonText = map[string]string{
	"unknown":          zhUnknown,
	"access-denied":    zhAccessDenied,
	"not-live-root":    zhNotLiveRoot,
	"closing":          zhClosing,
	"unavailable":      zhUnavailable,
	"invalid-location": zhInvalidLocation,
	"read-only":        zhReadOnly,
	"source-missing":   zhSourceMissing,
}

// applicationText 是官方 APPLICATION_KEYS（OFF:Card.tsx:35-37）：应用结果的四条专属文案。
//
// `applied` 不在表里（官方 `Exclude<…, 'applied'>`：安装成功不另画一行）。
var applicationText = map[string]string{
	"restart-required": zhRestartRequired,
	"overridden":       zhOverridden,
	"failed":           zhApplicationFailed,
	"cancelled":        zhApplicationCancel,
}

// openPhases 是官方 openPhases（OFF:Card.tsx:87 的四个 phase；宿主
// HOST:internal/creation/service.go 的 OpenPhases 同一组）：这些 phase 的卡还能再试一次。
var openPhases = map[string]bool{
	"cancelled": true, "failed": true, "interrupted": true, "unconfirmed": true,
}

// ── 本地状态（官方 React 组件的 useState 的 Go 对等物）─────────────────────

// Plugin 是官方个性化页面卡的正文。
type Plugin struct {
	theme *pluginapi.Theme

	mu sync.Mutex
	// card 是这一帧状态归属的卡（sessionID + callID）。换了卡就整份重置——
	// 官方每张卡一个组件实例，状态不会串到下一张卡上。
	card string
	// selected 是用户选的位置（官方 `selected: LocationChoice | undefined`）；
	// 空串 = 还没选（undefined）。
	selected string
	// customLocation 是 textarea 的文字（官方 `custom: string | undefined`）。
	customLocation string
	// customPrefilled 记住"这张卡已经用真实数据预填过"，之后不再覆盖用户输入
	// （官方 `custom ?? data?.customLocation ?? ''` 是**只读**投影，输入框里的编辑
	// 由 React state 持有；这里用一次性预填达到同一效果）。
	customPrefilled bool
	// infoSet 是用户显式开关过说明块的记（官方 `info: boolean | undefined`）。
	infoSet *bool
	// noticeDismissed 是持久偏好（官方 `infoDismissed` store）。
	noticeDismissed bool
	// pending 是在飞的那个动词（官方 `pending: CardVerb | undefined`）：
	// 三个按钮的进行时文案（开始创建…/正在取消…/恢复中…）都读它。
	pending string
	// menuOpen 是位置菜单的开关（官方 `useState(false)` 的 `open`）。
	menuOpen *bool
	// expanded 是两个折叠位（brief / 预填内容）的展开态。
	expanded map[string]bool
	// dispatch 是"把一次用户动作交给框架"的接缝（产品路径就是 props.Act）。
	// 存成字段而不是在绘制里直接调：run 的几个分支共用同一条出口，测试也只需替换它。
	dispatch func(verb string, payload map[string]json.RawMessage)
}

// errNoBodyRegistry 是"没有动作卡正文注册表"（ds-actioncards 没装配）。
var errNoBodyRegistry = errors.New("ds-creation-reco: 没有动作卡正文注册表（ds-actioncards 未装配）")

// screen 是这一帧算好的、绘制要用的全部值。
//
// 单独一个结构是因为官方把它们算在一处（Card.tsx:69-89），而绘制分给了几个函数：
// 分开放会让"哪个函数读的是真值"变成猜测。
type screen struct {
	details         cardDetails
	data            cardData
	hasData         bool
	phase           string
	offer           bool
	infoOpen        bool
	installed       bool
	busy            bool
	slot            string
	location        string
	customLocation  string
	applicationText string
	errorText       string
	noticeSource    bool // data.bundleName 存在（`.info` 里多一句 sourceRetained）
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return ServiceID }

// Provides 实现 pluginapi.Plugin：正文由动作卡框架持有，本包不对外提供服务键。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin：没有主题画不出官方配色，没有正文注册表就没地方登记。
func (p *Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyActionCardBodies}
}

// Register 实现 pluginapi.Plugin：把正文登记进动作卡框架的注册表。
//
// 重复登记**返回错误**（官方 registry 抛 `a body for "…" is already registered`）：
// 两个插件抢 `create_personalized_page` 的绘制权时静默覆盖，会让被覆盖那方永远看不见自己的改动。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme, _ = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	registry, _ := ctx.Inject(pluginapi.KeyActionCardBodies).(pluginapi.ActionCardBodyRegistry)
	if registry == nil {
		return errNoBodyRegistry
	}
	return registry.RegisterBody(p)
}

// Action 实现 pluginapi.ActionCardBody：这个正文画哪一种卡。
func (*Plugin) Action() string { return Action }

// Draw 实现 pluginapi.ActionCardBody：画官方 PageCardBody（OFF:Card.tsx:102-159）。
func (p *Plugin) Draw(f ui.C, env *pluginapi.Env, props pluginapi.ActionCardBodyProps) {
	if p.theme == nil && env != nil {
		p.theme = env.Theme
	}
	t := p.theme
	if t == nil {
		return
	}
	p.adoptCard(props)
	p.readPreferences(env)
	p.dispatch = props.Act

	details, hasDetails := detailsOf(props.Details)
	// 官方 :109：details 解析不了就只画 summary（不给任何动作）。
	if !hasDetails {
		ui.Column(f).FillWidth().
			PadSides(createdBodyPadTop, createdBodyPadX, createdBodyPadBottom, createdBodyPadX).
			Children(func() {
				p.drawSummary(f, env, t, props)
			})
		return
	}
	s := p.screenOf(env, props, details)
	p.drawHeader(f, env, t, s)
	if s.infoOpen {
		p.drawInfo(f, t, s)
	}
	ui.Column(f).FillWidth().
		PadSides(createdBodyPadTop, createdBodyPadX, createdBodyPadBottom, createdBodyPadX).
		Children(func() {
			p.drawSummary(f, env, t, props)
			if details.hasInitialContent {
				p.drawInitialContent(f, env, t, details.initialContent)
			}
			// 官方 :130-132：应用结果行（`applied` 不画；`failed` phase 有自己的文案，不叠加）。
			if s.applicationText != "" {
				ui.Text(f, s.applicationText).MarginTop(createdStatusMarginTop).
					FontSize(createdStatusFont).LineHeight(createdStatusLineH).Color(t.LabelSecondary)
			}
			// 官方 :133-134：动作反馈行（`error`，`role=alert`）。
			if s.errorText != "" {
				ui.Text(f, s.errorText).MarginTop(createdStatusMarginTop).
					FontSize(createdStatusFont).LineHeight(createdStatusLineH).Color(t.LabelSecondary)
			}
			if s.offer {
				p.drawControls(f, env, t, s)
				return
			}
			p.drawPlacement(f, env, t, s)
		})
}

// ── 这一帧的状态推导（官方 Card.tsx:69-89 的逐行对等物）─────────────────────

// screenOf 把 props 与本地状态压成这一帧要画的东西。
func (p *Plugin) screenOf(env *pluginapi.Env, props pluginapi.ActionCardBodyProps, details cardDetails) screen {
	phase := ""
	if props.HasDecision {
		phase = props.Phase
	}
	// 框架在动作落地后清 Acting；本地 pending 跟着落，三个"进行时"文案才不会卡住。
	if p.pending != "" && !props.Acting {
		p.pending = ""
	}
	data, hasData := dataOf(props.Data)
	// 预填一次：数据里的 customLocation 是真实值，但用户之后敲的字不许被它覆盖。
	if !p.customPrefilled {
		if hasData {
			p.customLocation = data.customLocation
		}
		p.customPrefilled = true
	}
	// 官方 :85-86：slot 与 location 是**两个**值（slot 永远是三值之一，location 还可能是 custom）。
	slot := details.slot
	if p.selected != "" && p.selected != choiceCustom {
		slot = p.selected
	} else if hasData && data.slot != "" {
		slot = data.slot
	}
	location := slot
	switch {
	case p.selected != "":
		location = p.selected
	case hasData && data.hasCustomLocation:
		location = choiceCustom
	}
	infoOpen := false
	if p.infoSet != nil {
		infoOpen = *p.infoSet
	} else {
		infoOpen = offerOf(phase, props.HasDecision) && !p.noticeDismissed
	}
	s := screen{
		details: details, data: data, hasData: hasData,
		phase: phase,
		offer: offerOf(phase, props.HasDecision),
		// 官方 `busy = pending !== undefined`；本仓还要认框架的在飞标记（异步回合里
		// pending 已被本地记住，这里多认一次不会让任何按钮提前复活）。
		busy:           props.Acting || p.pending != "",
		infoOpen:       infoOpen,
		installed:      phase == "installed" || phase == "kept",
		slot:           slot,
		location:       location,
		customLocation: p.customLocation,
		noticeSource:   hasData && data.bundleName != "",
	}
	// 官方 :130-131：应用结果行只在 `application !== 'applied'` 且 phase 不是 failed 时画
	// （failed 那一档的文案在 phase 行里，两者不叠加）。
	if hasData && data.application != "" && data.application != "applied" && phase != "failed" {
		s.applicationText = applicationText[data.application]
	}
	s.errorText = p.errorText(props)
	return s
}

// offerOf 是官方 `offer`（OFF:Card.tsx:87）：没决定，或决定的 phase 在 openPhases 里。
func offerOf(phase string, hasDecision bool) bool {
	return !hasDecision || openPhases[phase]
}

// errorText 是官方 `error`（OFF:Card.tsx:95-101）的 Go 版。
//
//	refused / unconfirmed → reason 的专属文案，没有就用官方的 unavailable
//	ActError（回复丢失 / 调用失败）→ 官方 transport
//	settled / accepted → 没有错误行
func (p *Plugin) errorText(props pluginapi.ActionCardBodyProps) string {
	if props.ActError != "" {
		return zhTransport
	}
	if props.LastResult == nil {
		return ""
	}
	switch props.LastResult.Status {
	case "refused", "unconfirmed":
		if text, ok := reasonText[props.LastResult.Reason]; ok {
			return text
		}
		return zhUnavailable
	default:
		return ""
	}
}

// ── header（通栏，官方 :103-114）─────────────────────────────────────────────

// drawHeader 画标题行：左「个性化页面 + info 按钮」，右 phase 状态。
func (p *Plugin) drawHeader(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, s screen) {
	header := ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.Between).
		Gap(createdHeaderGapColumn).
		PadSides(createdHeaderPadTop, createdHeaderPadX, 0, createdHeaderPadX)
	header.Children(func() {
		heading := ui.Row(f).Items(ui.Center).Gap(createdHeadingGap)
		heading.Children(func() {
			ui.Text(f, zhHeading).FontSize(createdHeaderFont).LineHeight(createdHeaderLineH).
				Color(t.LabelSecondary)
			p.drawInfoButton(f, env, t, s.infoOpen)
		})
		if text, ok := phaseText[s.phase]; ok {
			// 官方 `.phase{ color: label-primary }`，`role=status`（见包注释「没搬」第 5 条）。
			ui.Text(f, text).FontSize(createdHeaderFont).LineHeight(createdHeaderLineH).
				Color(t.LabelPrimary)
		}
	})
}

// drawInfoButton 画说明块的开关（官方 :104-112）：ghost 按钮、28×28、14px 图标。
func (p *Plugin) drawInfoButton(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, infoOpen bool) {
	button := ui.ButtonBase(f, zhCreationInfo).
		Width(createdInfoButtonW).Height(createdInfoButtonH).
		Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).
		Hit(p.hitID(env, "creation-info"), zhCreationInfo)
	// 官方 `Tooltip disabled={infoOpen}`：说明块开着的时候不再重复那一句提示。
	if !infoOpen {
		button = button.Tooltip(zhCreationInfo).TooltipOn(ui.TooltipTop)
	}
	button.Children(func() {
		// 官方是 `IconWarningTriangleOutlineRegular size={14}`（OFF:Card.tsx:112）：
		// viewBox=16 的官方原文缩到 14px 显示，见 icons.go。
		ui.Icon(f, warningIcon()).Size(createdInfoIcon, createdInfoIcon).Color(t.LabelSecondary)
	})
	if !button.Clicked() {
		return
	}
	next := !infoOpen
	p.infoSet = &next
	// 官方：关闭说明块 = 这条偏好已消费（`infoDismissed.set(true)`）。
	if !next {
		p.dismissNotice(env)
	}
}

// noticeFill 是说明块的底色：官方 `.info{ background: var(--dsw-alias-bg-module-platform) }`（:4）。
//
// **不是 `--dsw-alias-bg-layer-3`**（旧实现用的就是这个，属于量错了档）：亮色下
// layer-3 = `--dsw-static-neutral-bluish-00`(#ffffff)、module-platform =
// `--dsw-static-neutral-bluish-60`(#f9fafb)（OFF:dsh-client-ui-theme/src/styles/design-platform.css:183/189）；
// 暗色下两者同为 bluish-800(#353638)（同文件 :335/:341）——所以这一档只有亮色看得出来，
// 暗色下"看不出差别"不能当成取对了。
func noticeFill(t *pluginapi.Theme) ui.Color { return t.BgModulePlatform }

// drawInfo 画说明块（官方 :115-122）：三句话 + 可选的一句 sourceRetained。
func (p *Plugin) drawInfo(f ui.C, t *pluginapi.Theme, s screen) {
	info := ui.Column(f).FillWidth().
		MarginSides(createdInfoMarginTop, createdHeaderPadX, 0, createdHeaderPadX).
		Pad(createdInfoPad).Radius(t.Radii.MD).Background(noticeFill(t))
	info.Children(func() {
		ui.Text(f, zhCreationMethod).FillWidth().FontSize(createdInfoFont).
			LineHeight(createdInfoLineH).Color(t.LabelPrimary)
		ui.Text(f, zhCreationCost).FillWidth().MarginTop(createdInfoParaGap).
			FontSize(createdInfoFont).LineHeight(createdInfoLineH).Weight(createdInfoStrong).
			Color(t.LabelPrimary)
		ui.Text(f, zhCreationBackground).FillWidth().MarginTop(createdInfoParaGap).
			FontSize(createdInfoFont).LineHeight(createdInfoLineH).Color(t.LabelPrimary)
		if s.noticeSource {
			ui.Text(f, zhSourceRetained).FillWidth().MarginTop(createdInfoParaGap).
				FontSize(createdInfoFont).LineHeight(createdInfoLineH).Color(t.LabelPrimary)
		}
	})
}

// ── body：summary / 预填内容（官方 :126-129）─────────────────────────────────

// drawSummary 画卡片框交给正文的 `summary`（标题 + 折叠后的 brief）。
//
// 官方 summary 是 `<strong class=title>{title}</strong><FoldedText text={brief}/>`
// （OFF:action-cards/src/client/Card.tsx:112-115），它由**卡片框**拼好再交给 body
// （这个包的 body 拿到的 `summary` prop 就是那个节点），所以正文必须自己画这两层。
func (p *Plugin) drawSummary(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, props pluginapi.ActionCardBodyProps) {
	ui.Text(f, props.Title).FillWidth().
		FontSize(createdTitleFontSize).LineHeight(createdTitleLineH).
		Weight(createdTitleWeight).Color(t.LabelPrimary)
	if strings.TrimSpace(props.Brief) == "" {
		return
	}
	p.drawFolded(f, env, t, "brief", props.Brief, zhExpandBrief, zhCollapseBrief)
}

// drawInitialContent 画预填内容（官方 :127-128）：一行标签 + 可折叠的正文。
func (p *Plugin) drawInitialContent(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, content string) {
	ui.Column(f).FillWidth().MarginTop(createdContentMarginTop).Children(func() {
		ui.Text(f, zhInitialContent).FillWidth().FontSize(createdContentLabelFont).
			LineHeight(createdContentLabelLineH).Color(t.LabelSecondary)
		p.drawFolded(f, env, t, "content", content, zhExpandContent, zhCollapseContent)
	})
}

// drawFolded 画一段可折叠文本（官方 `FoldedText`，OFF:action-cards/Card.tsx:43-67）。
//
// key 区分两个折叠位（brief / 内容）；两者各有自己的展开态（官方两个组件实例）。
func (p *Plugin) drawFolded(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, key, text, expandLabel, collapseLabel string) {
	preview, omitted := foldText(text)
	shown := text
	if omitted && !p.expanded[key] {
		shown = preview
	}
	ui.Text(f, shown).FillWidth().MarginTop(createdTextMarginTop).
		FontSize(createdTextFont).LineHeight(createdTextLineH).Color(t.LabelSecondary)
	if !omitted {
		return
	}
	label := expandLabel
	if p.expanded[key] {
		label = collapseLabel
	}
	// 官方是 `Button variant=ghost size=sm`（`.toggle{ margin:8px 0 0; padding-inline:0;
	// height:auto; min-height:24px; font-size:12px }`）：本仓按 dsactioncards 的先例用
	// 文字 + 命中区（同一几何，省掉 viva 按钮那套自带的 padding）。
	toggle := ui.Text(f, label).MarginTop(createdToggleMarginTop).
		MinHeight(createdToggleMinHeight).FontSize(createdToggleFont).Color(t.LabelSecondary).
		Hit(p.hitID(env, "creation-fold-"+key), label)
	if !toggle.Clicked() {
		return
	}
	if p.expanded == nil {
		p.expanded = map[string]bool{}
	}
	p.expanded[key] = !p.expanded[key]
}

// ── 控件：还能重试的卡（官方 :135-150）───────────────────────────────────────

// drawControls 画「选位置 + 暂不需要 + 创建」（官方 `.controls`，:135-150）。
func (p *Plugin) drawControls(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, s screen) {
	ui.Divider(f).FillWidth().Height(createdBorderWidth).MarginTop(createdControlsMarginTop).
		Background(t.BorderL1)
	ui.Column(f).FillWidth().PadTop(createdControlsPadTop).Children(func() {
		row := ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.Between).Gap(createdControlsGap)
		row.Children(func() {
			locationRow := ui.Row(f).Items(ui.Center).Gap(createdLocationRowGap)
			locationRow.Children(func() {
				ui.Text(f, zhLocation).FontSize(createdLocationLabelFont).
					LineHeight(createdLocationLabelLineH).Color(t.LabelSecondary)
				p.drawLocationPicker(f, env, t, s)
			})
			if s.location != choiceCustom {
				p.drawOfferActions(f, env, t, s, false)
			}
		})
		if s.location != choiceCustom {
			return
		}
		// 官方 `textarea` 那一格是 `flex:1 0 100%`（Card.module.css:23）：永远独占一行，
		// 于是 footer（`margin:0 0 0 auto`）落到它下面。见包注释「没搬」第 1 条。
		p.drawCustomLocation(f, env, t, s)
		p.drawOfferActions(f, env, t, s, true)
	})
}

// drawLocationPicker 画位置选择按钮与它的四项菜单（官方 `LocationPicker`，:43-66）。
func (p *Plugin) drawLocationPicker(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, s screen) {
	if p.menuOpen == nil {
		open := false
		p.menuOpen = &open
	}
	label := locationLabel(s.location)
	// 官方这枚按钮的 `aria-label` 是 `${t('location')}: ${t(value)}`（"添加到: 右侧栏"）。
	// viva 的 `Hit(id, label)` 里那个 label **同时是可见文字**（实测：写全串会把按钮
	// 文字改掉），所以这里用可见文字当无障碍名，`添加到` 由左边那格文字承担。
	anchor := ui.Row(f).Items(ui.Center).Gap(createdLocationRowGap).Children(func() {
		// 官方 `.locationButton{ font-size:13px }`（:21）：sm 原语是 12，这个包抬到 13。
		button := ui.Button(f, label).Outline().Small().FontSize(createdLocationButtonFont).
			Disabled(s.busy).
			Hit(p.hitID(env, "creation-location"), label)
		if !s.busy && button.Clicked() {
			*p.menuOpen = !*p.menuOpen
		}
		// 官方 chevron 在按钮**内部**（`.locationButton{gap:8px}`）；viva 的 `ui.Button`
		// 放不进子节点，所以画在它右侧同一行（点击面积 = 按钮那部分），见「没搬」第 1 条。
		if icon, ok := chevronIcon(); ok {
			ui.Icon(f, icon).Size(createdLocationChevron, createdLocationChevron).
				Color(t.LabelSecondary)
		}
	})
	// 官方 `open={open && !disabled}`（OFF:Card.tsx:47）：动作在飞时菜单不出现。
	// viva 的 `ui.Menu` 收的是 `*bool`（它会写回 false），所以"禁用"= 干脆不声明。
	if s.busy {
		return
	}
	ui.Menu(f, anchor, p.menuOpen, func(items *ui.MenuItems) {
		if items == nil {
			return
		}
		items.WidthRange(createdLocationMenuWidth, createdLocationMenuWidth)
		for _, choice := range locationChoices {
			// 官方每一项是两行（`.locationOption`：名字 + hint），viva 的 `Description`
			// 就是第二行——同一形状。
			item := items.Item(locationLabel(choice)).Description(locationHint(choice))
			if item.Clicked() {
				p.chooseLocation(choice)
			}
		}
	})
}

// chooseLocation 是菜单选中一项之后的状态变更（官方 `onSelect={(id) => { onChange(id);
// setOpen(false) }}`，OFF:Card.tsx:52）。
//
// 单独一个方法是**测试接缝**：viva 的弹层项由框架自己持有身份（`MenuItems.Item` 内部
// 分配 scope），仓里的测试点不到它（`plugins/dscontextmenu` 的测试同样只做冒烟），
// 所以"选中之后状态怎么变"要有一条能直接被调用的真路径。
func (p *Plugin) chooseLocation(choice string) {
	p.selected = choice
	if p.menuOpen != nil {
		*p.menuOpen = false
	}
}

// drawCustomLocation 画「你希望放在哪里？」的 textarea（官方 :138-141）。
func (p *Plugin) drawCustomLocation(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, s screen) {
	ui.Column(f).FillWidth().MarginTop(createdRowGap).Gap(createdCustomGap).Children(func() {
		ui.Text(f, zhCustomLabel).FillWidth().FontSize(createdLocationLabelFont).
			LineHeight(createdLocationLabelLineH).Color(t.LabelSecondary)
		area := ui.TextArea(f, &p.customLocation, zhCustomLabel, zhCustomPlaceholder).
			FillWidth().Height(createdCustomHeight).
			FontSize(createdCustomFont).LineHeight(createdCustomLineH).
			Radius(t.Radii.MD).Border(createdBorderWidth, t.BorderL4).Background(t.BgLayer1).
			// rc.2 `Card.module.css:27`：
			//   `.customLocation textarea:focus-visible{
			//      outline:1px solid var(--dsw-alias-state-business-primary);outline-offset:1px}`
			// `outline-offset:1` ⇒ 1px 宽的环带落在框外 1..2px，路径中心在 1.5px 处。
			// 用 DrawOver 自己画：`Look.Focus(...)` 对文本域不生效（vr3 实测环带零命中）。
			DrawOver(func(d *ui.DrawContext) {
				if !d.State.FocusVisible {
					return
				}
				const half = createdCustomFocusWidth / 2
				rect := d.Rect
				rect.X -= createdCustomFocusOffset + half
				rect.Y -= createdCustomFocusOffset + half
				rect.Width += 2 * (createdCustomFocusOffset + half)
				rect.Height += 2 * (createdCustomFocusOffset + half)
				d.Canvas.RoundedOutline(rect,
					t.Radii.MD+createdCustomFocusOffset+half, createdCustomFocusWidth, d.Fade(t.Info))
			})
		if s.busy {
			area = area.Disabled(true)
		}
	})
}

// drawOfferActions 画「暂不需要 + 创建/重新创建」（官方 `.footer`，:142-149）。
//
// ownLine=true 时它自己占一行、右对齐（选了自定义位置时；官方 `.controls .footer
// { margin:0 0 0 auto }` + wrap 的结果）。
func (p *Plugin) drawOfferActions(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, s screen, ownLine bool) {
	footer := ui.Row(f).Items(ui.Center).Gap(createdActionsGap)
	if ownLine {
		footer = ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.End).
			Gap(createdActionsGap).MarginTop(createdFooterMarginTop)
	}
	footer.Children(func() {
		dismiss := ui.Button(f, zhDismiss).Ghost().Small().FontSize(createdSmallButtonFont).Disabled(s.busy).
			Hit(p.hitID(env, "creation-dismiss"), zhDismiss)
		if !s.busy && dismiss.Clicked() {
			p.runOffer(env, "dismiss", s)
		}
		label := zhCreate
		switch {
		case p.pending == "create":
			label = zhStarting
		case s.phase != "":
			label = zhRetry
		}
		// 官方 :146-147：`disabled={busy || (location === 'custom' && customLocation.trim() === '')}`。
		disabled := s.busy || (s.location == choiceCustom && strings.TrimSpace(s.customLocation) == "")
		create := ui.Button(f, label).Small().FontSize(createdSmallButtonFont).Background(t.ButtonPrimaryFill).
			Color(t.ButtonPrimaryFg).Disabled(disabled).
			Hit(p.hitID(env, "creation-create"), label)
		// `Disabled` 在 viva 里只画灰态，命中区仍可能报点击 → 这一侧再判一次
		// （官方 `<Button disabled>` 是 DOM 语义上的不可交互，没有这层歧义）。
		if !disabled && create.Clicked() {
			p.runOffer(env, "create", s)
		}
	})
}

// ── 控件：已经落定的卡（官方 :151-157）───────────────────────────────────────

// drawPlacement 画「位置 + 撤销/取消创建/恢复」那一行（官方 `.placement`，:151-157）。
func (p *Plugin) drawPlacement(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, s screen) {
	ui.Divider(f).FillWidth().Height(createdBorderWidth).MarginTop(createdPlacementMarginTop).
		Background(t.BorderL1)
	row := ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.Between).
		Gap(createdRowGap).PadTop(createdPlacementPadTop)
	row.Children(func() {
		// 官方 `{t('location')} · {data?.customLocation ?? t(data?.slot ?? details.slot)}`。
		target := locationLabel(s.slot)
		if s.hasData && s.data.hasCustomLocation {
			target = s.data.customLocation
		}
		ui.Text(f, zhLocation+" · "+target).Grow(1).MinWidth(0).
			FontSize(createdDescriptionFont).LineHeight(createdDescriptionLineH).
			Color(t.LabelSecondary)
		actions := ui.Row(f).Items(ui.Center).Gap(createdActionsGap)
		actions.Children(func() {
			// 官方 :154-155：`creating` 档给「取消创建」。
			if s.phase == "creating" {
				label := zhCancel
				if p.pending == "cancel" {
					label = zhCancelling
				}
				cancel := ui.Button(f, label).Ghost().Small().FontSize(createdSmallButtonFont).Disabled(s.busy).
					Hit(p.hitID(env, "creation-cancel"), label)
				if !s.busy && cancel.Clicked() {
					p.runVerb(verbCancel, s)
				}
			}
			// 官方 :156：已安装/已保留给「撤销」。
			if s.installed {
				undo := ui.Button(f, zhUndo).Ghost().Small().FontSize(createdSmallButtonFont).Disabled(s.busy).
					Hit(p.hitID(env, "creation-undo"), zhUndo)
				if !s.busy && undo.Clicked() {
					p.runVerb(verbUndo, s)
				}
			}
			// 官方 :157：已移除给「恢复」。
			if s.phase == "removed" {
				label := zhRestore
				if p.pending == "restore" {
					label = zhRestoring
				}
				restore := ui.Button(f, label).Small().FontSize(createdSmallButtonFont).Background(t.ButtonPrimaryFill).
					Color(t.ButtonPrimaryFg).Disabled(s.busy).
					Hit(p.hitID(env, "creation-restore"), label)
				if !s.busy && restore.Clicked() {
					p.runVerb(verbRestore, s)
				}
			}
		})
	})
}

// ── 动作 ───────────────────────────────────────────────────────────────────

// runOffer 是 create / dismiss 那两下（官方 `run(verb)` 里带 placement 的那一支）。
func (p *Plugin) runOffer(env *pluginapi.Env, verb string, s screen) {
	if s.busy {
		return
	}
	// 官方 :93-94：`placement = { slot, ...(location === 'custom' ? { customLocation:
	// customLocation.trim() } : {}) }`。
	placement := map[string]json.RawMessage{}
	if encoded, err := json.Marshal(s.slot); err == nil {
		placement["slot"] = encoded
	}
	if s.location == choiceCustom {
		if encoded, err := json.Marshal(strings.TrimSpace(s.customLocation)); err == nil {
			placement["customLocation"] = encoded
		}
	}
	// 官方 :92：点 create/dismiss 就先记下"这条提示已经消费"，并关掉说明块。
	p.dismissNotice(env)
	closed := false
	p.infoSet = &closed
	p.fire(verb, placement)
}

// runVerb 是 undo / cancel / restore 那三下（官方同一次 `run`，payload 为 undefined）。
//
// 官方 :96：`act(verb, verb === 'create' || verb === 'dismiss' ? placement : undefined)`
// ——非 create/dismiss 的动词**不带** payload。
func (p *Plugin) runVerb(verb string, s screen) {
	if s.busy {
		return
	}
	p.fire(verb, nil)
}

// fire 记下在飞的动词并把它交给框架（框架负责 HTTP、在飞标记与结果回灌）。
func (p *Plugin) fire(verb string, payload map[string]json.RawMessage) {
	p.pending = verb
	if p.dispatch != nil {
		p.dispatch(verb, payload)
	}
}

// ── 持久偏好（官方 info-store.ts）──────────────────────────────────────────

// readPreferences 读一次持久偏好。
//
// 每次读、只让 `1` 生效（壳的 ConfigGet 返回的是**缓存**，首帧可能是空 map——
// 那时读不到不能当成"用户没关过"并把它写回去）。
func (p *Plugin) readPreferences(env *pluginapi.Env) {
	if env == nil || env.Act.ConfigGet == nil {
		return
	}
	if env.Act.ConfigGet()[prefInfoDismissed] == "1" {
		p.noticeDismissed = true
	}
}

// dismissNotice 记下"说明块已被消费"并落盘（宿主 `config.set`，合并语义）。
func (p *Plugin) dismissNotice(env *pluginapi.Env) {
	p.noticeDismissed = true
	if env == nil || env.Act.ConfigSet == nil {
		return
	}
	env.Act.ConfigSet(map[string]string{prefInfoDismissed: "1"})
}

// ── 卡身份与状态归属 ────────────────────────────────────────────────────────

// adoptCard 在换卡时整份重置本地状态（官方每张卡一个组件实例）。
func (p *Plugin) adoptCard(props pluginapi.ActionCardBodyProps) {
	key := props.SessionID + "\x00" + props.CallID
	if p.card == key {
		return
	}
	p.card = key
	p.selected = ""
	p.customLocation = ""
	p.customPrefilled = false
	p.infoSet = nil
	p.pending = ""
	p.menuOpen = nil
	p.expanded = map[string]bool{}
}

// hitID 造交互 id（不许写死数字：环境里有 Tree 时用 Tree.ID）。
//
// 没有 Tree（单测直接画一眼）时返回零值 ID：零值 = "没有目标"，`Clicked()` 恒 false。
func (p *Plugin) hitID(env *pluginapi.Env, name string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, name)
}

// ── 数据解析（官方 zod schema 的 Go 落地）──────────────────────────────────

// cardDetails 是官方 `detailsSchema`（OFF:Card.tsx:19）的投影。
type cardDetails struct {
	slot              string
	initialContent    string
	hasInitialContent bool
}

// detailsOf 解析 proposal.details；slot 不是三值之一就是"解析不了"（官方 safeParse 失败）。
func detailsOf(raw map[string]json.RawMessage) (cardDetails, bool) {
	var out cardDetails
	if raw == nil {
		return out, false
	}
	if value, ok := raw["slot"]; ok {
		var slot string
		if json.Unmarshal(value, &slot) == nil && knownSlot(slot) {
			out.slot = slot
		}
	}
	if out.slot == "" {
		return cardDetails{}, false
	}
	if value, ok := raw["initialContent"]; ok {
		var text string
		if json.Unmarshal(value, &text) != nil {
			return cardDetails{}, false
		}
		out.initialContent = text
		out.hasInitialContent = true
	}
	return out, true
}

// cardData 是官方 `dataSchema`（OFF:Card.tsx:20-23）的投影。
type cardData struct {
	slot              string
	customLocation    string
	hasCustomLocation bool
	bundleName        string
	application       string
}

// dataOf 解析 decision.data；没有合法 slot 就整份解析失败（官方 safeParse 同一判据：
// dataSchema 的 slot 必填且必须是三值之一）。
func dataOf(raw map[string]json.RawMessage) (cardData, bool) {
	var out cardData
	if raw == nil {
		return out, false
	}
	if value, ok := raw["slot"]; ok {
		var slot string
		if json.Unmarshal(value, &slot) == nil && knownSlot(slot) {
			out.slot = slot
		}
	}
	if out.slot == "" {
		return cardData{}, false
	}
	if value, ok := raw["customLocation"]; ok {
		var text string
		if json.Unmarshal(value, &text) == nil {
			out.customLocation = text
			out.hasCustomLocation = true
		}
	}
	if value, ok := raw["bundleName"]; ok {
		var name string
		if json.Unmarshal(value, &name) == nil {
			out.bundleName = name
		}
	}
	if value, ok := raw["application"]; ok {
		var application string
		if json.Unmarshal(value, &application) == nil {
			out.application = application
		}
	}
	return out, true
}

// knownSlot 报告 slot 是不是官方三值之一。
func knownSlot(slot string) bool {
	switch slot {
	case slotSidebarRight, slotPage, slotSidebarLeft:
		return true
	default:
		return false
	}
}

// locationLabel 是 `t(value)`：四个位置选项与三个 slot 的名字。
func locationLabel(value string) string {
	switch value {
	case slotSidebarRight:
		return zhSlotSidebarRight
	case slotPage:
		return zhSlotPage
	case slotSidebarLeft:
		return zhSlotSidebarLeft
	case choiceCustom:
		return zhChoiceCustom
	default:
		return value
	}
}

// locationHint 是 `t(\`${id}Hint\`)`：菜单每一项的第二行。
func locationHint(value string) string {
	switch value {
	case slotSidebarRight:
		return zhSidebarRightHint
	case slotPage:
		return zhPageHint
	case slotSidebarLeft:
		return zhSidebarLeftHint
	case choiceCustom:
		return zhCustomHint
	default:
		return ""
	}
}

// foldText 把文本折成预览（官方 `FoldedText` 的算法，OFF:action-cards/Card.tsx:52-61）。
//
// 先压平空白，再按"拉丁字母宽"累计到 260 就截断加 `…`；CJK（Han/Hiragana/Katakana/
// Hangul）、CJK 标点与全角形式算 2，其余算 1。**原文一个字都不改写**。
func foldText(text string) (preview string, omitted bool) {
	compact := strings.Join(strings.Fields(strings.ReplaceAll(text, "\u00a0", " ")), " ")
	if compact == "" {
		return "", false
	}
	width := 0
	for i, r := range compact {
		if wideRune(r) {
			width += 2
		} else {
			width++
		}
		if width > foldWidth {
			return strings.TrimRight(compact[:i], " ") + "…", true
		}
	}
	return compact, false
}

// wideRune 报告一个 rune 是不是"画出来约两倍宽"的字（官方 WIDE_GRAPHEME 的 rune 等价物）。
func wideRune(r rune) bool {
	switch {
	case r >= 0x2E80 && r <= 0x303F: // CJK 部首补充 + CJK 标点（官方 〿 区间同源）
		return true
	case r >= 0xFF00 && r <= 0xFF60: // 全角形式
		return true
	case r >= 0x3040 && r <= 0x30FF: // 平假名 / 片假名
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // Han（基本区）
		return true
	case r >= 0x3400 && r <= 0x4DBF: // Han 扩展 A
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // Hangul 音节
		return true
	case r >= 0x1100 && r <= 0x11FF: // Hangul Jamo
		return true
	}
	return false
}
