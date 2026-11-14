// official_agent_presets.go —— 「Agent 预设」设置页的**数据模型**（#3）。
//
// # 官方出处（逐条）
//
//	结构   OFF:dsh-client-ui-agent-preset/src/client/AgentPresetSection.tsx:96-176
//	       （`section` → `h2.title` + `p.intro` + 分组 → `ul.cards` → 每张卡）
//	几何   OFF:dsh-client-ui-agent-preset/src/client/AgentPresetSection.module.css:1-120
//	文案   OFF:dsh-client-ui-agent-preset/src/client/locales.ts:83-121（zh 逐字）
//	读名册 OFF:dsh-client-ui-agent-preset/src/client/section-store.ts:36-46
//	       （`ctx.remote.agentPresets.list()`）
//	写默认 OFF:dsh-client-ui-agent-preset/src/client/settings-store.ts:19-26
//	       （`settings.update('agent-preset-registry', {selectedDefault: id}, rev)`）
//
// # 这一页为什么不是「标题 + 一行一个控件」
//
// 官方这里是**卡片网格**（`AgentPresetSection.module.css` 开头那段注释写得很直白：
// "Cards, not rows: a preset is a thing you pick, and the description is the part
// that tells them apart —— a row would bury it beside the actions"）。
// 所以它跟「内置插件」页一样走自己的渲染面，不塞进 PanelRow 那套。
//
// # 只画真值
//
//   - 名册没接（`agentPresets.list` 不存在）→ 整页「未知」，**不画空网格**；
//   - 名册是空数组 → 「这个部署没有组合 preset」，与上一条是两句不同的话；
//   - `name` 缺席显示 id（官方 `presetDisplayText` 的回落），`description` 缺席
//     显示官方 `noDescription`「暂无描述。」——**不编**一句介绍。
package dssettings

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 官方 zh 文案（`locales.ts:83-121` 逐字）。
const (
	zhAgentPresetIntro       = "选择 Agent 的工具和工作方式。日常任务用「标准模式」，扩展 DSH 的能力用「创造模式」。"
	zhAgentPresetSetDefault  = "设为新任务默认"
	zhAgentPresetInUse       = "新任务默认"
	zhAgentPresetNoDesc      = "暂无描述。"
	zhAgentPresetBroken      = "加载失败"
	zhAgentPresetUnavailable = "宿主没有 preset 名册（agentPresets.list 未接线）"
	zhAgentPresetEmpty       = "这个部署没有组合任何 Agent 预设。"
	zhAgentPresetNoNamespace = "宿主没有发布 agent-preset-registry 命名空间，无法把某一条设为默认。"
)

// AgentPresetCard 是名册里的一行 → 一张卡（官方 `AgentPresetSection.tsx:130-165`）。
type AgentPresetCard struct {
	// ID 是 preset id（显示在右上角的等宽小字里，也是写 `selectedDefault` 的值）。
	ID string
	// Name 是显示名：宿主给的 `name`，缺席时回落成 ID。
	Name string
	// Description 是宿主给的描述；缺席时渲染层显示 `zhAgentPresetNoDesc`。
	Description string
	// Broken 非空 = 声明存在但装不起来（官方 `broken`）。本仓宿主当前不产生它。
	Broken string
	// InUse = 这一条就是"新任务默认"（宿主 `isDefault`，不是客户端自己猜的）。
	InUse bool
}

// AgentPresetsPanel 是这一页的全部内容。
type AgentPresetsPanel struct {
	// Intro 是官方 `sectionIntro` 逐字（渲染层直接画）。
	Intro string
	// State 与 PanelState 同一套语义（ok / unknown / empty）。
	State PanelState
	// Note 是 unknown / empty 的原因（**不许**用它假装有数据）。
	Note string
	// Cards 是名册的卡片，顺序**照宿主返回的顺序**（官方也是 roster 顺序，
	// 不按名字重排——顺序是宿主/用户的意图）。
	Cards []AgentPresetCard
	// Revision 是 `agent-preset-registry` 命名空间当前的 revision，写默认时当乐观锁。
	// 0 = 没拿到（不 fence）。
	Revision int
	// CanSetDefault 报告写路径成不成立：命名空间发布了、可写、且这一帧有 revision。
	//
	// false 时卡片**不可点**（渲染层显示原因）——不是画一个点了没反应的卡片。
	CanSetDefault bool
	// DenyReason 是 CanSetDefault=false 的原因（给人看的）。
	DenyReason string
}

// buildAgentPresetsSection 把 `agentPresets.list` 的结果投影成这一页。
//
// `roster == nil` 与 `len(roster.Presets) == 0` 是**两件不同的事**：
// 前者是"宿主没这条能力"（官方 `readRoster` 对 `gateway/invocation-unavailable`
// 也走这条语义），后者是"宿主查过了，一个 preset 都没有"。两者都不能画成空网格。
func buildAgentPresetsSection(panel *PanelSection, describe *apiclient.SettingsDescribeResult,
	roster *apiclient.AgentPresetRoster) {
	if describe == nil {
		panel.State = PanelUnknown
		panel.Note = "未知：宿主未接 settings.describe"
		return
	}
	if roster == nil {
		panel.State = PanelUnknown
		panel.Note = "未知：" + zhAgentPresetUnavailable
		return
	}
	write, canWrite, deny := agentPresetWriteTarget(describe)
	if len(roster.Presets) == 0 {
		panel.State = PanelEmpty
		panel.Note = zhAgentPresetEmpty
		return
	}
	cards := make([]AgentPresetCard, 0, len(roster.Presets))
	for _, preset := range roster.Presets {
		id := strings.TrimSpace(preset.ID)
		if id == "" {
			// 没有 id 的行写不出去（`selectedDefault` 要的就是 id），
			// 静默丢掉比画一张点了没用的卡诚实：它是一个不可能成立的条目。
			continue
		}
		name := strings.TrimSpace(preset.Name)
		if name == "" {
			name = id
		}
		cards = append(cards, AgentPresetCard{
			ID:          id,
			Name:        name,
			Description: strings.TrimSpace(preset.Description),
			Broken:      strings.TrimSpace(preset.Broken),
			InUse:       preset.IsDefault,
		})
	}
	if len(cards) == 0 {
		panel.State = PanelUnknown
		panel.Note = "未知：名册里没有一条带 id 的 preset（写不出去，不画假卡片）"
		return
	}
	panel.State = PanelOK
	panel.AgentPresets = &AgentPresetsPanel{
		Intro:         zhAgentPresetIntro,
		State:         PanelOK,
		Cards:         cards,
		Revision:      write.revision,
		CanSetDefault: canWrite,
		DenyReason:    deny,
	}
}

// agentPresetWrite 是一次「设为默认」的写入目标。
type agentPresetWrite struct {
	ns       string
	path     []string
	revision int
}

// agentPresetWriteTarget 解析写目标：命名空间必须**真的**出现在 describe 里。
//
// 为什么以 describe 为准而不是硬编码常量：命名空间存在与否是宿主的事实
// （`internal/settingsform` 挂没挂这条 schema）。没挂时写出去只会拿到
// `unknown_field`/`not_found`，界面却看起来像点过了——宁可先在这里判死。
func agentPresetWriteTarget(describe *apiclient.SettingsDescribeResult) (agentPresetWrite, bool, string) {
	if describe == nil {
		return agentPresetWrite{}, false, zhAgentPresetNoNamespace
	}
	ns, ok := describe.Namespace(apiclient.SettingsNSAgentPreset)
	if !ok {
		return agentPresetWrite{}, false, zhAgentPresetNoNamespace
	}
	// 字段级可写性：宿主可能整条命名空间可写但把这一字段标成只读。
	writable := false
	if section, ok := describe.Section(apiclient.SettingsNSAgentPreset); ok {
		writable = section.Writable
		for _, field := range section.Fields {
			if field.Name == apiclient.SettingsFieldSelectedDefault {
				writable = field.Writable
				break
			}
		}
	}
	if !writable {
		return agentPresetWrite{}, false, "宿主把 agent-preset-registry.selectedDefault 标成只读，改不了。"
	}
	return agentPresetWrite{
		ns:       apiclient.SettingsNSAgentPreset,
		path:     []string{apiclient.SettingsFieldSelectedDefault},
		revision: ns.Revision,
	}, true, ""
}
