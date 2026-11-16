// settings_items.go —— 官方 4 条「插件设置页」在本仓的**数据模型**
// （#39 / #45 / #46 上一批；#47「网页搜索与获取」本批，它的专用模型在
// settings_items_web.go，绘制在 settings_items_web_draw.go）。
//
// # 官方结构（为什么落点在「插件」页，而不是设置面板的某一页）
//
// 官方这 4 页不是 `settings.page` 那种顶级 section，而是 **`plugins.item` 槽的卡片**：
// 插件管理页（`dsh-client-ui-plugin-manager`）把官方插件的配置注册成卡片，
// 卡片打开后是一页表单。逐条锚点：
//
//	#39 OFF:dsh-client-ui-settings-agent-loop/src/client/index.ts:47-49
//	      whileServed([AGENT_LOOP_NS]) → slots.register({name:'plugins.item',
//	      id:'agent-loop', order:30, label:t('title')}, AgentLoopCard)
//	#45 OFF:dsh-client-ui-settings-shell/src/client/index.ts:50-52
//	      whileServed([BASH_NS, PWSH_NS]) → {id:'shell', order:40}
//	#46 OFF:dsh-client-ui-settings-subagent/src/client/index.ts:85-92
//	      whileServed([SUBAGENT_NS, SUBAGENT_MODEL_SELECTION_NS]) → {id:'subagent', order:10}
//	#47 OFF:dsh-client-ui-settings-web-search/src/client/index.ts:72-74
//	      whileServed([WEB_NS]) → {id:'web-search', order:20}   ← 本批（它的形态是
//	      provider 菜单 + 灰面板，不是字段表单：见 settingsItem.Kind）
//
// 卡片落点是插件管理页的 `basic` 组（官方 `renderGroup('basic', t('basicTitle'), …)`，
// OFF:PluginManagerPage.tsx:1855；`basicTitle: '内置插件配置'`，locales.ts:34），
// 顺序在「实验性插件」与「已安装插件」**之前**（同处 1855-1857）。
//
// 存在条件 = `whileServed`：宿主**服务**了这条命名空间，卡片才存在；不服务就整张卡
// 没有（不是灰色、不是禁用）。所以本文件的 `settingsItemCards` 只把「宿主真的发布了
// 至少一条它所绑定的 namespace」的条目放进卡片清单，别的**根本不出现**。
//
// # 值从哪来
//
// 宿主 `settings.describe` 的 `sections[].fields[]`：`current`（有效值）、
// `default`（base）、`overridden`（user 层是否有显式值）、`writable`，加上
// `sections[].revision` 当乐观锁。写路径用官方同一条：`settings.mutate` 提交一批
// `set`/`unset` 路径操作（官方 `SettingsFormModel.save` → `scope.mutate(ops, revision)`，
// OFF 见 bundled primitives `SettingsFormModel` 的 save/plan）。
//
// # 官方表单语义（逐条照做，不是"差不多"）
//
// 官方 `SettingsFormModel`（打字稿原件不在随包发布里，但**编译产物**在
// `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`；本仓把两处都当证据，锚点见
// `settings_items_geometry.go`）：
//
//	numberField.format(v) = typeof v === 'number' ? String(v) : ''
//	numberField.parse(t)  = t.trim()==='' ? {clear} : (Number.isFinite(Number(t)) ? {set} : undefined)
//	field(name): 无草稿 → {text: format(current), overridden: stored(name)}
//	             有草稿 → {text: 草稿文本, overridden: parse.kind==='set', invalid: parse===undefined}
//	resetField(name) → 草稿 = {text: format(default), clear: true}
//	plan(): clear → stored 时才发 unset；文本等于 format(current) → 跳过；
//	        parse 失败 → 记 invalid（不发）；parse=clear（空白）→ unset；否则 set
//	save(): plan 为空 / 只读 / invalid / saving → 不动；否则 mutate(ops, baselineRevision)，
//	        成功才清草稿，失败置 failed（界面画 saveFailed 那句）
//
// 本文件把上面这套状态机 1:1 落到 Go：`settingsItemDraft` = 草稿，`settingsItemPlan`
// = plan()，`applySettingsItemSave` = save()。
package dssidebar

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// ── 官方 zh 文案（逐字，勿改；锚点是 rc.2 随包 locales.ts）──────────────

const (
	// settingsItemsGroupTitle 是 `basic` 组标题（OFF:ui-plugin-manager/src/client/locales.ts:34）。
	settingsItemsGroupTitle = "内置插件配置"

	// #39 Agent 循环（OFF:ui-settings-agent-loop/src/client/locales.ts:36-47）。
	zhAgentLoopTitle       = "Agent 循环"             // 'title' :36
	zhAgentLoopSummary     = "控制 Agent 派发工具调用的方式。"  // 'description' :37
	zhAgentLoopParallel    = "并行工具调用数"              // 'maxParallel' :38
	zhAgentLoopParallelTip = "同一步内最多同时运行多少个可并行的调用。" // 'maxParallelHint' :39

	// #45 终端（OFF:ui-settings-shell/src/client/locales.ts:32-45）。
	zhShellTitle        = "终端"                     // 'title' :32
	zhShellSummary      = "限制每条命令最多能跑多久、最多输出多少内容。" // 'description' :33
	zhShellTimeout      = "命令超时（毫秒）"               // 'timeoutMs' :34
	zhShellTimeoutTip   = "单条命令允许运行多久，超时即终止。"      // 'timeoutMsHint' :35
	zhShellMaxOutput    = "单流输出上限（字节）"             // 'maxOutputBytes' :36
	zhShellMaxOutputTip = "超出部分会转存到临时文件，而不是被丢弃。"   // 'maxOutputBytesHint' :37

	// #46 子智能体（OFF:ui-settings-subagent/src/client/locales.ts:72-85）。
	zhSubagentTitle     = "子智能体"                                                        // 'subagentTitle' :72
	zhSubagentSummary   = "设置子智能体的递归层级、数量和模型。"                                          // 'subagentDescription' :73
	zhSubagentLimits    = "运行限制"                                                        // 'subagentLimitsTitle' :74
	zhSubagentMaxDepth  = "最大递归深度"                                                      // 'subagentMaxDepth' :75
	zhSubagentDepthHelp = "最大递归深度说明"                                                    // 'subagentDepthHelpLabel' :76
	zhSubagentDepthBody = "限制 Agent 创建子智能体的递归层级。"                                       // 'subagentDepthHelp' :77
	zhSubagentDepthZero = "禁用子智能体"                                                      // 'subagentDepthZero' :78
	zhSubagentDepthOne  = "仅允许主 Agent 创建子智能体"                                           // 'subagentDepthOne' :79
	zhSubagentDepthTail = "如果某个工具单独设置了最大递归深度，以该工具的设置为准。"                                // 'subagentDepthOverride' :80
	zhSubagentDepthBad  = "请输入不小于 0 的整数。"                                               // 'subagentDepthInvalid' :84
	zhSubagentActive    = "子智能体并行数量上限"                                                  // 'subagentMaxActive' :81
	zhSubagentCapHelp   = "子智能体并行数量上限说明"                                                // 'subagentCapacityHelpLabel' :82
	zhSubagentCapBody   = "同一主 Agent 下，所有递归层级同时存活的子智能体总数，主 Agent 不计入。达到上限时，新的启动请求会被拒绝。" // 'subagentCapacityHelp' :83
	zhSubagentCapBad    = "请输入不小于 1 的整数。"                                               // 'subagentCapacityInvalid' :85

	// #46 模型选择（OFF:ui-settings-subagent/src/client/locales.ts:86-99）。
	zhSubagentModels           = "模型选择"                                              // 'subagentModelSelectionTitle' :86
	zhSubagentModelToggle      = "允许 Agent 为子智能体选择模型"                                // 'subagentModelSelectionToggle' :87
	zhSubagentModelChoose      = "开启后，Agent 可以从下方授权模型中，为每个子智能体选择提供方、模型和推理强度。仅影响新会话。" // :88
	zhSubagentModelAllowed     = "Agent 可选择的模型"                                      // :89
	zhSubagentModelLoading     = "正在加载模型…"                                           // :90
	zhSubagentModelLoadFailed  = "无法加载模型。"                                           // :91
	zhSubagentModelRetry       = "重试"                                                // :92
	zhSubagentModelPartial     = "部分模型提供方暂时无法加载；已保存的选择仍可移除。"                         // :93
	zhSubagentModelUnavailable = "当前不可用"                                             // :94
	zhSubagentModelStaleGroup  = "已保存但当前不可用"                                         // :95
	zhSubagentModelEmpty       = "当前没有模型提供方公布模型。"                                    // :96
	zhSubagentModelRequired    = "保存前请至少选择一个模型。"                                     // :97
	zhSubagentModelConflict    = "设置已在其他位置更新。请放弃修改后重试。"                              // :98
	zhSubagentModelOff         = "关闭后，子智能体使用配置的默认模型或继承父 Agent 的模型；已选模型会保留。"          // :99

	// 三个页面共用的表单框架文案（各自 locales.ts 的同名键，逐字相同）：
	// agent-loop:40-47 / shell:38-45 / subagent:65-71。
	zhFormOverridden = "已覆盖"                 // 'overridden'
	zhFormReset      = "恢复默认"                // 'reset'
	zhFormReadOnly   = "本部署的设置为只读。"          // 'readOnly'
	zhFormUnavail    = "该插件当前未加载，暂时无法配置。"    // 'unavailable'
	zhFormSave       = "保存"                  // 'save'
	zhFormSaving     = "保存中…"                // 'saving'
	zhFormSaveFailed = "本部署没有接受这些值，已保留供你修改。" // 'saveFailed'
	zhFormInvalidNum = "请填数字；留空表示使用默认值。"     // 'invalidNumber'

	// 本仓自己的两句话（官方没有对应键，只在**宿主没给数据**时说，不冒充官方文案）。
	zhItemUnknownValue = "未知：宿主没有给出这个字段的当前值"
	zhItemNoDescribe   = "未知：宿主未接 settings.describe"
)

// ── 官方命名空间与字段（逐字）────────────────────────────────────────

const (
	// settingsNSAgentLoop 是 #39 绑的命名空间（OFF:agent-loop-card-controller.ts:13）。
	settingsNSAgentLoop = "agent-loop"
	// settingsFieldMaxParallelToolCalls 是同一页唯一的字段（OFF:AgentLoopCard.tsx:30-37）。
	settingsFieldMaxParallelToolCalls = "maxParallelToolCalls"

	// settingsNSBashSandbox / settingsNSPwshSandbox 是 #45 绑的两条执行器命名空间
	// （OFF:shell-card-controller.ts:10-12）。
	settingsNSBashSandbox = "bash-sandbox"
	settingsNSPwshSandbox = "pwsh-sandbox"
	// settingsFieldShellTimeout / settingsFieldShellMaxOutput 是那两页编辑的两个字段
	// （OFF:shell-card-controller.ts:14-20，官方注释自己写明是 served schema 的子集）。
	settingsFieldShellTimeout   = "timeoutMs"
	settingsFieldShellMaxOutput = "maxOutputBytes"

	// settingsNSSubagent 是 #46 的第一条命名空间（OFF:index.ts:50）。
	settingsNSSubagent = "subagent"
	// settingsNSSubagentModelSelection 是同一页的第二条命名空间
	// （OFF:subagent-model-selection-card-controller.ts:9）。allowedModels 是对象数组，
	// 不走 sections.fields，而从 NamespaceView.Value 的原始 JSON 投影；写路径仍是
	// settings.mutate（宿主 host_settings_pages_test.go:257-271 已钉住往返）。
	settingsNSSubagentModelSelection = "subagent-model-selection-settings"
	// settingsFieldMaxDepth / settingsFieldMaxActiveSubagents 是效果那一节的两个字段
	// （OFF:subagent-limits-card-controller.ts:10-13）。
	settingsFieldMaxDepth           = "maxDepth"
	settingsFieldMaxActiveSubagents = "maxActiveSubagents"
	// 模型选择一节在同一次 mutate 里写这两个字段
	// （OFF:subagent-model-selection-card-controller.ts:279-286）。
	settingsFieldSubagentModelEnabled = "enabled"
	settingsFieldAllowedModels        = "allowedModels"
)

// settingsItemField 是一页表单里的一个字段（官方 `SettingsValueField`）。
type settingsItemField struct {
	// Name 是宿主字段名（也是写路径的 path[0]）。
	Name string
	// Kind 是字段的解析/格式化语义；零值 = 官方 `settingsNumberField`。
	Kind settingsItemFieldKind
	// Label 是官方 `label`。
	Label string
	// Hint 是官方 `hint`（`''` = 官方这页没给 hint）。
	Hint string
	// HelpLabel 是官方 `help.label`（`''` = 没有帮助折行）。
	HelpLabel string
	// HelpLines 是官方 `help.content` 的正文（段落按顺序；表格压成 "0 禁用子智能体"
	// 这样的行——官方是 `<table>`，本仓没有表格组件，内容逐字、版式不同，已登记）。
	HelpLines []string
	// Invalid 是官方 `invalidLabel`（解析失败时画它）。
	Invalid string
	// DOMID 是官方写死的 input id（`plugin-config-…`），本仓当稳定交互 id 用。
	DOMID string
	// Limit 表示要用官方 `limitField` 的解析（整数 + 下限），否则是 `settingsNumberField`。
	Limit bool
	// Min 只对 Limit 字段有意义（官方 `limitField(field, minimum)` 的 minimum）。
	Min int64
	// Max 是额外上限（官方 `count` 的自定义 parse 里 `value <= 50`）；nil = 无上限。
	//
	// 为什么单独一个字段：官方 count 的 parse 不是 `limitField`，而是
	// `Number.isInteger(v) && v >= 1 && v <= 50`（OFF:web-search-card-controller.ts:296-299）。
	Max *int64
}

// settingsItemFieldKind 是字段的解析/格式化语义（官方三种 `settings*Field`）。
type settingsItemFieldKind uint8

const (
	// settingsItemFieldNumber 是官方 `settingsNumberField`
	// （format：只有 number 才有文本；parse：空白 clear、`Number.isFinite` 才 set）。
	settingsItemFieldNumber settingsItemFieldKind = iota
	// settingsItemFieldText 是官方 `settingsTextField`
	// （format：只有 string 才有文本；parse：空白 clear，否则 set **trim 后的原文**）。
	settingsItemFieldText
	// settingsItemFieldProvider 是 cot TranslationPreferencesForm 的 provider：
	// 空白 clear，非空只接受 PROVIDER_PREFERENCES（preferences-form.ts:25-31）。
	settingsItemFieldProvider
	// settingsItemFieldTargetLanguage 是 cot targetLanguage：空白也算 invalid，
	// 只接受 auto 或官方 BCP 47 pattern（preferences-form.ts:33-35）。
	settingsItemFieldTargetLanguage
)

// settingsItemSection 是一页里的一个 section（对应一条命名空间）。
type settingsItemSection struct {
	// NS 是命名空间名（写路径的目标）。
	NS string
	// Title 是官方 section 标题（`''` = 官方那页没有 section 标题，整页就是一个表单）。
	Title string
	// Fields 是这一节的字段（顺序即官方 JSX 里的顺序）。
	Fields []settingsItemField
}

// settingsItem 是一张官方插件配置卡片。
type settingsItem struct {
	// ID 是官方注册 id（`plugins.item` 的 id）。
	ID string
	// Order 是官方注册 order（升序）。
	Order int
	// Title 是卡片标题（官方 `label`）。
	Title string
	// Summary 是卡片/详情页的一句话（官方 summary 视图）。
	Summary string
	// Kind 是这一页的**渲染形态**：'' = 普通字段表单（Sections 驱动）；
	// `web-search` = #47 那一页（provider 菜单 + 灰面板 + 计费 + 密钥，见 settings_items_web.go）。
	//
	// 为什么需要它：官方 `plugins.item` 槽里各页的形状本来就不一样——#39/#45/#46 是
	// 一串 `SettingsValueField`，而 #47 自带 `.selector` 下拉、`.panel` 灰面板和
	// 只读的 advanced override 标签（`WebSearchCard.tsx:68-129`）。用一个字段类型
	// 去硬套另一种布局只会让两边都变形。
	Kind string
	// Sections 是这一页的 section（Kind='' 时驱动渲染）。
	Sections []settingsItemSection
	// Anchor 是注册点的官方锚点（报告与结构测试用）。
	Anchor string
}

// settingsItemKindWebSearch 是 #47 那一页的形态标记。
const settingsItemKindWebSearch = "web-search"

// Namespaces 返回这张卡绑定的全部命名空间（`whileServed([...])` 的那一组）。
func (it settingsItem) Namespaces() []string {
	out := make([]string, 0, len(it.Sections))
	for _, section := range it.Sections {
		out = append(out, section.NS)
	}
	return out
}

// settingsItemLedger 是本批实现的 4 张卡（#39 / #45 / #46 上批 + #47 本批）。
//
// 顺序按官方 order：subagent(10) → web-search(20) → agent-loop(30) → shell(40)。
// **不按名字重排**：顺序是官方注册意图。
func settingsItemLedger() []settingsItem {
	return []settingsItem{
		{
			ID:      "subagent",
			Order:   10,
			Title:   zhSubagentTitle,
			Summary: zhSubagentSummary,
			Anchor:  "ui-settings-subagent/src/client/index.ts:85-92",
			Sections: []settingsItemSection{{
				NS:    settingsNSSubagent,
				Title: zhSubagentLimits,
				Fields: []settingsItemField{
					{
						Name:      settingsFieldMaxDepth,
						Label:     zhSubagentMaxDepth,
						HelpLabel: zhSubagentDepthHelp,
						HelpLines: []string{
							zhSubagentDepthBody,
							"0 " + zhSubagentDepthZero,
							"1 " + zhSubagentDepthOne,
							zhSubagentDepthTail,
						},
						Invalid: zhSubagentDepthBad,
						DOMID:   "plugin-config-subagent-depth",
						Limit:   true,
						Min:     0,
					},
					{
						Name:      settingsFieldMaxActiveSubagents,
						Label:     zhSubagentActive,
						HelpLabel: zhSubagentCapHelp,
						HelpLines: []string{zhSubagentCapBody},
						Invalid:   zhSubagentCapBad,
						DOMID:     "plugin-config-subagent-capacity",
						Limit:     true,
						Min:       1,
					},
				},
			}},
		},
		{
			ID:      "web-search",
			Order:   20,
			Kind:    settingsItemKindWebSearch,
			Title:   zhWebTitle,
			Summary: zhWebSummary,
			Anchor:  "ui-settings-web-search/src/client/index.ts:72-74",
		},
		{
			ID:      "agent-loop",
			Order:   30,
			Title:   zhAgentLoopTitle,
			Summary: zhAgentLoopSummary,
			Anchor:  "ui-settings-agent-loop/src/client/index.ts:47-49",
			Sections: []settingsItemSection{{
				NS: settingsNSAgentLoop,
				Fields: []settingsItemField{{
					Name:    settingsFieldMaxParallelToolCalls,
					Label:   zhAgentLoopParallel,
					Hint:    zhAgentLoopParallelTip,
					Invalid: zhFormInvalidNum,
					DOMID:   "plugin-config-agent-loop-parallel",
				}},
			}},
		},
		{
			ID:      "shell",
			Order:   40,
			Title:   zhShellTitle,
			Summary: zhShellSummary,
			Anchor:  "ui-settings-shell/src/client/index.ts:50-52",
			Sections: []settingsItemSection{{
				// 官方按**服务的是哪条**绑哪个控制器：
				// `served.has(PWSH_NS) ? pwsh : bash`（index.ts:51）。
				// 本仓由 settingsItemSectionNS 在运行期决定，这里只列候选。
				NS: settingsNSPwshSandbox,
				Fields: []settingsItemField{
					{
						Name:    settingsFieldShellTimeout,
						Label:   zhShellTimeout,
						Hint:    zhShellTimeoutTip,
						Invalid: zhFormInvalidNum,
						DOMID:   "plugin-config-shell-timeout",
					},
					{
						Name:    settingsFieldShellMaxOutput,
						Label:   zhShellMaxOutput,
						Hint:    zhShellMaxOutputTip,
						Invalid: zhFormInvalidNum,
						DOMID:   "plugin-config-shell-output",
					},
				},
			}},
		},
	}
}

// settingsItemServed 报告宿主服务了哪些命名空间（`whileServed` 的等价物）。
//
// 判据是 describe 的 `namespaces[]`（官方 scope 的 ready 也是看这条），**不是**
// sections：sections 是本仓 additive 的字段投影，缺失只代表"看不到值"，不代表宿主
// 没服务那条命名空间。
func settingsItemServed(describe *apiclient.SettingsDescribeResult) map[string]bool {
	served := map[string]bool{}
	if describe == nil {
		return served
	}
	for _, ns := range describe.Namespaces {
		served[ns.NS] = true
	}
	return served
}

// settingsItemShellNS 选 #45 这一页这一帧绑哪条执行器命名空间。
//
// 官方：`served.has(PWSH_NS) ? pwsh : bash`（OFF:index.ts:51）——Windows 上是
// pwsh，其它平台是 bash。本仓只做 Windows（AGENTS.md §0），但**不硬编码平台判断**：
// 按官方同一判据（哪条被服务）选，两条都没服务时这一页整张不存在。
func settingsItemShellNS(served map[string]bool) string {
	switch {
	case served[settingsNSPwshSandbox]:
		return settingsNSPwshSandbox
	case served[settingsNSBashSandbox]:
		return settingsNSBashSandbox
	default:
		return ""
	}
}

// settingsItemSectionNS 解析一个 section 这一帧真正指向的命名空间。
func settingsItemSectionNS(it settingsItem, section settingsItemSection, served map[string]bool) string {
	if it.ID != "shell" {
		return section.NS
	}
	return settingsItemShellNS(served)
}

// ── 草稿 / plan / 写结果 ─────────────────────────────────────────────

// settingsItemDraft 是一个字段的草稿（官方 `staged` 表的一项）。
type settingsItemDraft struct {
	// Text 是草稿文本（reset 时是官方 default 的格式化文本）。
	Text string
	// Clear 表示"保存时发 unset"（官方 `{clear:true}`）。
	Clear bool
	// Ref 只对**密钥草稿**有意义：它是输入这份草稿时控件指向的凭据引用
	// （官方 `draftRef`，`web-search-card-controller.ts:427-432` 与 :545-551）。
	//
	// 为什么草稿要自己记引用：官方在发布时发现引用变了就把草稿**整份丢掉**
	// （`dropStaleKeyDraft`），所以一次输入永远不会被写到另一个引用名下。本仓
	// 把这条不变量放在 plan 里判（`draft.Ref == 当前引用` 才写），等价且没有
	// "绘制顺带改状态"的副作用；引用变了以后遗留的旧草稿在 plan/显示上都被忽略。
	Ref string
	// Subagent 非 nil 时，这份草稿承载 #46 整个模型选择 section，而不是文本字段。
	// enabled 与 allowedModels 必须共享一个草稿和 baseline，保存时才能按官方形状
	// 在同一次 settings.mutate 中提交（OFF:subagent-model-selection-card-controller.ts:262-286）。
	Subagent *settingsSubagentModelDraft
}

// settingsItemDraftKey 是草稿表的键：`ns\x00field`。
func settingsItemDraftKey(ns, field string) string { return ns + "\x00" + field }

// settingsItemWrite 是一次保存的结果（绘制层唯一读的状态）。
type settingsItemWrite struct {
	// Saving 表示有保存在飞（官方 `state.saving`）。
	Saving bool
	// Failed 表示上一次保存失败（官方 `state.failed`，画 saveFailed 那句）。
	Failed bool
	// Detail 是失败的真实原因（进状态栏，不冒充官方文案）。
	Detail string
}

// settingsItemForm 是一张卡的草稿与写状态。
//
// 为什么要独立 mutex：保存是 `go func`（绘制线程不能等 HTTP），写结果与草稿清理
// 落在另一个 goroutine 上，绘制线程同时读。与 workspace_actions.go 的 wsMu 同一先例。
type settingsItemForm struct {
	mu       sync.Mutex
	drafts   map[string]settingsItemDraft
	baseline map[string]int // ns → 第一次 stage 时的 revision（官方 baseline?.revision）
	write    settingsItemWrite
}

// ensureSettingsItemForm 惰性初始化（Register 里也会调一次）。
func (p *Plugin) ensureSettingsItemForm() *settingsItemForm {
	if p.itemForm == nil {
		p.itemForm = &settingsItemForm{
			drafts:   map[string]settingsItemDraft{},
			baseline: map[string]int{},
		}
	}
	return p.itemForm
}

// stage 记一份草稿，并锁住 baseline revision（官方 `stage()` 首写记 baseline）。
func (f *settingsItemForm) stage(ns, field string, draft settingsItemDraft, revision int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.drafts == nil {
		f.drafts = map[string]settingsItemDraft{}
	}
	if f.baseline == nil {
		f.baseline = map[string]int{}
	}
	if _, ok := f.baseline[ns]; !ok {
		f.baseline[ns] = revision
	}
	f.drafts[settingsItemDraftKey(ns, field)] = draft
	f.write.Failed = false
	f.write.Detail = ""
}

// resetField 是官方 `resetField`：草稿 = {default 的文本, clear}。
func (f *settingsItemForm) resetField(ns, field, defaultText string) {
	f.stage(ns, field, settingsItemDraft{Text: defaultText, Clear: true}, 0)
}

// discard 清掉全部草稿（官方 `discard()`）。
func (f *settingsItemForm) discard() {
	f.mu.Lock()
	f.drafts = map[string]settingsItemDraft{}
	f.baseline = map[string]int{}
	f.write.Failed = false
	f.write.Detail = ""
	f.mu.Unlock()
}

// snapshot 复制一份草稿/基线/写状态给绘制层（锁内拷贝，别再持锁画界面）。
func (f *settingsItemForm) snapshot() (map[string]settingsItemDraft, map[string]int, settingsItemWrite) {
	f.mu.Lock()
	defer f.mu.Unlock()
	drafts := make(map[string]settingsItemDraft, len(f.drafts))
	for key, draft := range f.drafts {
		if draft.Subagent != nil {
			draft.Subagent = cloneSettingsSubagentModelDraft(draft.Subagent)
		}
		drafts[key] = draft
	}
	baseline := make(map[string]int, len(f.baseline))
	for ns, revision := range f.baseline {
		baseline[ns] = revision
	}
	return drafts, baseline, f.write
}

// setWrite 记写状态并返回是否需要重绘。
func (f *settingsItemForm) setWrite(write settingsItemWrite) {
	f.mu.Lock()
	f.write = write
	f.mu.Unlock()
}

// setBaseline 把某个命名空间的乐观锁 revision 换新（保存成功后用宿主回来的值）。
func (f *settingsItemForm) setBaseline(ns string, revision int) {
	f.mu.Lock()
	if f.baseline == nil {
		f.baseline = map[string]int{}
	}
	f.baseline[ns] = revision
	f.mu.Unlock()
}

// clearDrafts 在保存成功后清草稿（官方 `this.staged.clear()`）。
func (f *settingsItemForm) clearDrafts(keys []string) {
	f.mu.Lock()
	for _, key := range keys {
		delete(f.drafts, key)
	}
	if len(f.drafts) == 0 {
		f.baseline = map[string]int{}
	}
	f.mu.Unlock()
}

// settingsItemOp 是一条待写操作连同它的命名空间。
type settingsItemOp struct {
	NS  string
	Ops []apiclient.SettingsOp
}

// settingsItemPlan 是官方 `plan()` 的结果。
type settingsItemPlan struct {
	// Sections 按 section 顺序列出要发的操作（只含真的需要写的 ns）。
	Sections []settingsItemOp
	// Invalid 表示至少一个草稿解析不了（官方 `invalid`：那时 save 不动）。
	Invalid bool
}

// Dirty 报告是否有需要写的东西（官方 `dirty`）。
func (plan settingsItemPlan) Dirty() bool { return len(plan.Sections) > 0 }

// TotalOps 是这次保存要发的路径操作条数（测试与状态栏用）。
func (plan settingsItemPlan) TotalOps() int {
	total := 0
	for _, section := range plan.Sections {
		total += len(section.Ops)
	}
	return total
}

// settingsItemFieldState 是一个字段这一帧的呈现状态（官方 `field(name)` 的等价物）。
type settingsItemFieldState struct {
	// Spec 是台账里的字段声明。
	Spec settingsItemField
	// Known 表示宿主给了 current。
	Known bool
	// Current / Default 是宿主的有效值与默认值（格式化后的文本）。
	Current string
	Default string
	// Stored 表示 user 层有显式值（官方 `stored(name)`，用来判 unset 与角标）。
	Stored bool
	// Text 是**该画在输入框里的文本**（有草稿就是草稿）。
	Text string
	// Overridden 是官方 `field().overridden`：有草稿时 = 草稿是 set；无草稿 = stored。
	Overridden bool
	// Invalid 是官方 `field().invalid`：草稿解析不了。
	Invalid bool
	// Disabled 是官方 `disabled={!state.writable}`，外加本仓一条：宿主没给值时
	// 也不许编辑（不然一次空保存会 unset 一个我们根本不知道的值）。
	Disabled bool
	// Note 是 Disabled 的原因（本仓自己的两句话，不冒充官方文案）。
	Note string
}

// settingsItemSectionState 是一节这一帧的呈现状态。
type settingsItemSectionState struct {
	NS        string
	Title     string
	Available bool
	Writable  bool
	Revision  int
	Fields    []settingsItemFieldState
}

// settingsItemCardState 是一张卡这一帧的呈现状态。
type settingsItemCardState struct {
	Item     settingsItem
	Known    bool // 宿主 describe 到了（卡片是否出现只看 whileServed）
	Writable bool
	Failed   bool
	FailNote string
	Sections []settingsItemSectionState
	Plan     settingsItemPlan
	// Saving 是保存中的写状态。
	Saving bool
	// Web 是 #47 那一页的专用状态（Kind==settingsItemKindWebSearch 时非 nil）。
	// 那一页不是"section → 字段"的形状（provider 菜单 + 灰面板），所以状态也是并行的。
	Web *settingsItemWebState
	// Cot 是 #63 `plugins.bundle.config` 的 provider 清单快照；nil = 普通表单。
	Cot *settingsCotState
	// Model 是 #46 的模型选择 section；allowedModels 是对象数组，与标量 limits 并行投影。
	Model *settingsSubagentModelState
}

// Available 报告这张卡这一帧有没有内容可画（= 至少一节被服务）。
func (c settingsItemCardState) Available() bool {
	if c.Web != nil {
		return c.Web.webAvailable
	}
	if c.Model != nil && c.Model.Available {
		return true
	}
	for _, section := range c.Sections {
		if section.Available {
			return true
		}
	}
	return false
}

// Dirty 报告表单有没有未保存的改动（官方 `state.dirty`）。
func (c settingsItemCardState) Dirty() bool {
	if c.Web != nil {
		return c.Web.plan.Dirty()
	}
	return c.Plan.Dirty() || (c.Model != nil && c.Model.Dirty)
}

// Invalid 报告表单里有没有解析不了的草稿（官方 `state.invalid`）。
func (c settingsItemCardState) Invalid() bool {
	if c.Web != nil {
		return c.Web.plan.Invalid
	}
	if c.Model != nil && (c.Model.Invalid || (c.Model.Dirty && c.Model.Conflicted)) {
		return true
	}
	return c.Plan.Invalid
}

// CanSave 是官方 Save 按钮的 disabled 判据：
// `!state.dirty || state.invalid || state.saving`，且不可写时一律不许保存。
func (c settingsItemCardState) CanSave() bool {
	return c.Available() && c.Writable && c.Dirty() && !c.Invalid() && !c.Saving
}

// formatItemNumber 是官方 `settingsNumberField.format`：
// 只有数字才有文本，其它一律空串（**不许把缺失变成 0**）。
func formatItemNumber(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return ""
		}
		if typed == math.Trunc(typed) && math.Abs(typed) < 1e15 {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case float32:
		return formatItemNumber(float64(typed))
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return formatItemNumber(string(typed))
	default:
		return ""
	}
}

// parseItemField 是官方 `settingsNumberField.parse` / `limitField.parse`。
//
// 返回 (draft, ok)：ok=false 是"解析不了"（官方 `undefined`，记 invalid）。
// 官方 parse 返回 {kind:'clear'} 时草稿等价于 unset，这里用 clear 标志表达。
//
// `Max` 非 nil 时按官方 count 的自定义 parse 判上限（`value <= 50`，
// OFF:web-search-card-controller.ts:296-299）。
func parseItemField(spec settingsItemField, text string) (value float64, clear, ok bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, true, true
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false, false
	}
	if spec.Limit {
		// 官方 `limitField`：`Number.isSafeInteger(value) && value >= minimum`。
		if parsed != math.Trunc(parsed) || math.Abs(parsed) > 1<<53 {
			return 0, false, false
		}
		if parsed < float64(spec.Min) {
			return 0, false, false
		}
	}
	if spec.Max != nil && parsed > float64(*spec.Max) {
		return 0, false, false
	}
	return parsed, false, true
}

// formatItemText 是官方 `settingsTextField.format`：`typeof v === 'string' ? v : ”`。
//
// 与数字字段同一判据：**不许把缺失变成空串以外的任何东西**，也**不许把数字变成
// 字符串**（官方那样做等于替宿主改类型）。
func formatItemText(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

// parseItemText 是官方 `settingsTextField.parse`：
// `r = t.trim(); r === ” ? {kind:'clear'} : {kind:'set', value: r}`。
//
// 注意官方对文本字段**也 trim**（写下去的是 trim 后的原文），与数字字段同一处实现。
func parseItemText(text string) (value string, clear, ok bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", true, true
	}
	return trimmed, false, true
}

// buildSettingsItemCard 把 describe + 草稿投影成一张卡的呈现状态。
//
// `describe == nil`（壳还没拉到）时卡片的 Available 仍是 false，但**卡片本身**由调用方
// 决定画不画：官方 whileServed 未满足时卡片不存在，所以 nil describe 下一张卡都不出现。
func buildSettingsItemCard(it settingsItem, describe *apiclient.SettingsDescribeResult,
	drafts map[string]settingsItemDraft, write settingsItemWrite) settingsItemCardState {
	state := settingsItemCardState{Item: it, Saving: write.Saving, Failed: write.Failed, FailNote: write.Detail}
	if describe == nil {
		return state
	}
	state.Known = true
	served := settingsItemServed(describe)
	var plan settingsItemPlan
	for _, section := range it.Sections {
		ns := settingsItemSectionNS(it, section, served)
		if ns == "" {
			continue
		}
		probe := section
		probe.NS = ns
		sectionState, sectionOps, invalid := buildSettingsItemSection(probe, describe, served, drafts)
		if !sectionState.Available {
			continue
		}
		state.Sections = append(state.Sections, sectionState)
		if len(sectionOps) > 0 {
			plan.Sections = append(plan.Sections, settingsItemOp{NS: ns, Ops: sectionOps})
		}
		if invalid {
			plan.Invalid = true
		}
	}
	state.Plan = plan
	if len(state.Sections) > 0 {
		state.Writable = true
		for _, section := range state.Sections {
			if !section.Writable {
				state.Writable = false
			}
		}
	}
	return state
}

// buildSettingsItemSection 投影一节，并算出这一节的待写操作。
func buildSettingsItemSection(section settingsItemSection, describe *apiclient.SettingsDescribeResult,
	served map[string]bool, drafts map[string]settingsItemDraft) (settingsItemSectionState, []apiclient.SettingsOp, bool) {
	state := settingsItemSectionState{NS: section.NS, Title: section.Title}
	if !served[section.NS] {
		return state, nil, false
	}
	state.Available = true
	view, ok := describe.Section(section.NS)
	if ok {
		state.Writable = view.Writable
		state.Revision = view.Revision
	}
	invalidAny := false
	var ops []apiclient.SettingsOp
	for _, spec := range section.Fields {
		fieldState, op, invalid := buildSettingsItemField(section.NS, spec, describe, view, ok, drafts)
		state.Fields = append(state.Fields, fieldState)
		if op != nil {
			ops = append(ops, *op)
		}
		if invalid {
			invalidAny = true
		}
	}
	return state, ops, invalidAny
}

// buildSettingsItemField 投影一个字段，并算出它的待写操作（官方 plan() 的单字段分支）。
func buildSettingsItemField(ns string, spec settingsItemField, describe *apiclient.SettingsDescribeResult,
	view apiclient.SettingsSectionView, sectionOK bool, drafts map[string]settingsItemDraft) (settingsItemFieldState, *apiclient.SettingsOp, bool) {
	state := settingsItemFieldState{Spec: spec}
	var field apiclient.SettingsFieldView
	found := false
	if sectionOK {
		for _, candidate := range view.Fields {
			if candidate.Name == spec.Name {
				field = candidate
				found = true
				break
			}
		}
	}
	if found {
		if spec.Kind != settingsItemFieldNumber {
			// 官方 `settingsTextField.format` 的判据：没有值 = **空文本**，不是一个
			// "未知值"。文本字段（baseURL / 端点）天生可以空，空就是"用默认"，
			// 所以这里 Known 恒为真（数字字段不能这样——见下一个分支的理由）。
			state.Known = true
			state.Current = formatItemText(field.Current)
			state.Default = formatItemText(field.Default)
		} else {
			state.Known = field.Current != nil
			state.Current = formatItemNumber(field.Current)
			state.Default = formatItemNumber(field.Default)
		}
		state.Stored = field.Overridden
	}
	draft, staged := drafts[settingsItemDraftKey(ns, spec.Name)]
	switch {
	case !found:
		state.Disabled = true
		state.Note = zhItemNoDescribe
	case !state.Known:
		state.Disabled = true
		state.Note = zhItemUnknownValue
	default:
		state.Text = state.Current
		state.Overridden = state.Stored
		state.Disabled = !view.Writable
	}
	if !staged || !found || !state.Known {
		return state, nil, false
	}
	// 有草稿：文本用草稿，overridden/invalid 按官方 `field()` 的规则重算。
	state.Text = draft.Text
	if draft.Clear {
		state.Overridden = false
		state.Invalid = false
		if state.Stored {
			return state, &apiclient.SettingsOp{Op: "unset", Path: []string{spec.Name}}, false
		}
		return state, nil, false
	}
	value, clear, ok := parseSettingsItemDraft(spec, draft.Text)
	if !ok {
		state.Invalid = true
		state.Overridden = false
		return state, nil, true
	}
	if clear {
		// 空白 = 清掉覆盖（官方 parse 的 {kind:'clear'}）。
		state.Overridden = false
		if state.Stored {
			return state, &apiclient.SettingsOp{Op: "unset", Path: []string{spec.Name}}, false
		}
		return state, nil, false
	}
	state.Overridden = true
	if draft.Text == state.Current {
		// 与当前值逐字相同 = 不需要写（官方 `r.text === format(sectionValue)` 跳过）。
		return state, nil, false
	}
	return state, &apiclient.SettingsOp{Op: "set", Path: []string{spec.Name}, Value: value}, false
}

// parseSettingsItemDraft 是按字段语义解析草稿文本的唯一入口（官方 `parse`）。
//
// 返回的 setValue 只在 `ok && !clear` 时有意义，且**类型跟官方走**：
// 数字字段发数字、文本字段发字符串。绝不把字符串数字发成字符串
// （那会让宿主的 `invalid_type` 拒掉一次本来合法的写入）。
func parseSettingsItemDraft(spec settingsItemField, text string) (setValue any, clear, ok bool) {
	switch spec.Kind {
	case settingsItemFieldProvider:
		return parseCotProviderDraft(text)
	case settingsItemFieldTargetLanguage:
		return parseCotTargetLanguageDraft(text)
	}
	if spec.Kind != settingsItemFieldNumber {
		value, textClear, textOK := parseItemText(text)
		if !textOK || textClear {
			return nil, textClear, textOK
		}
		return value, false, true
	}
	value, numberClear, numberOK := parseItemField(spec, text)
	if !numberOK || numberClear {
		return nil, numberClear, numberOK
	}
	return value, false, true
}

// applySettingsItemSave 是官方 `SettingsFormModel.save()`：逐 ns 提交，全部成功才清草稿。
//
// **在调用方的 goroutine 里跑**（绘制线程不能等 HTTP）；失败不静默，返回真实原因。
func (p *Plugin) applySettingsItemSave(act pluginapi.Actions, plan settingsItemPlan,
	baseline map[string]int, form *settingsItemForm, drafts map[string]settingsItemDraft) {
	if act.SettingsMutate == nil {
		form.setWrite(settingsItemWrite{Failed: true, Detail: "壳未接 settings.mutate"})
		if act.Invalidate != nil {
			act.Invalidate()
		}
		return
	}
	for _, section := range plan.Sections {
		revision := baseline[section.NS]
		view, err := act.SettingsMutate(section.NS, section.Ops, revision)
		if err != nil {
			form.setWrite(settingsItemWrite{Failed: true, Detail: sectionWriteDetail(section.NS, err)})
			if act.Invalidate != nil {
				act.Invalidate()
			}
			return
		}
		// 写成功后用宿主回来的 revision 更新 baseline：同一次保存里的下一个 ns 不受影响，
		// 但下一次保存必须 fence 新的 revision（官方也用 scope 的最新 revision）。
		if view.Revision > 0 {
			form.setBaseline(section.NS, view.Revision)
		}
	}
	keys := make([]string, 0, len(drafts))
	for key := range drafts {
		keys = append(keys, key)
	}
	form.clearDrafts(keys)
	form.setWrite(settingsItemWrite{})
	if act.Invalidate != nil {
		act.Invalidate()
	}
}

// sectionWriteDetail 把写失败的原因说清（命名空间 + 宿主原文）。
func sectionWriteDetail(ns string, err error) string {
	message := err.Error()
	if strings.TrimSpace(ns) != "" {
		return ns + "：" + message
	}
	return message
}
