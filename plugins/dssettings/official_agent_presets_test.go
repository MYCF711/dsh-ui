// official_agent_presets_test.go —— #3「Agent 预设」页的判据。
//
// 这一页最容易犯的错是**把"没有数据"画成"数据是空的"**：
// `agentPresets.list` 未接线（未知）与名册真的是空数组（这个部署没组合 preset）
// 是两句不同的话，用户看到的下一步也不同。四条状态各自钉一条。
package dssettings

import (
	"context"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// agentPresetDescribe 造一份**含** agent-preset-registry 命名空间的 describe。
//
// 单独造而不是复用 describeFixture：那份夹具里没有这个命名空间，
// 拿它测写路径会把"命名空间不在"和"写失败"混成一件事。
func agentPresetDescribe(revision int, fieldWritable bool) *apiclient.SettingsDescribeResult {
	return &apiclient.SettingsDescribeResult{
		Writable: true,
		Namespaces: []apiclient.SettingsNamespaceView{
			{NS: apiclient.SettingsNSAgentPreset, Revision: revision,
				Value: []byte(`{"selectedDefault":"default"}`)},
		},
		Sections: []apiclient.SettingsSectionView{{
			NS: apiclient.SettingsNSAgentPreset, Title: "Default agent preset",
			Writable: true, Revision: revision,
			Fields: []apiclient.SettingsFieldView{{
				Name: apiclient.SettingsFieldSelectedDefault,
				Path: []string{apiclient.SettingsFieldSelectedDefault},
				Type: "string", Current: "default", Writable: fieldWritable,
			}},
		}},
	}
}

// agentPresetRosterFixture 是宿主 `agentPresets.list` 的真实形状（顺序照宿主返回）。
func agentPresetRosterFixture() *apiclient.AgentPresetRoster {
	return &apiclient.AgentPresetRoster{Presets: []apiclient.AgentPreset{
		{ID: "default", IsDefault: true, Name: "Default", Description: "全部工具可用"},
		{ID: "read-only", Name: "Read only", Description: "只能读"},
		{ID: "workspace-write", Name: "Workspace write"},
	}}
}

func agentPresetSection(t *testing.T, describe *apiclient.SettingsDescribeResult,
	roster *apiclient.AgentPresetRoster) PanelSection {
	t.Helper()
	model := BuildPanelWithSources(describe, fullDesktopFacts(), nil, PluginsView{}, roster)
	section, ok := model.Section("agent-presets")
	if !ok {
		t.Fatal("模型里没有 agent-presets 这一页")
	}
	return section
}

// TestAgentPresetsUnknownWhenRosterUnwired 钉住「宿主没这条能力」≠「名册是空的」。
func TestAgentPresetsUnknownWhenRosterUnwired(t *testing.T) {
	section := agentPresetSection(t, agentPresetDescribe(4, true), nil)
	if section.State != PanelUnknown {
		t.Fatalf("roster 未接线时 state = %q，应为 unknown", section.State)
	}
	if section.AgentPresets != nil {
		t.Fatal("roster 未接线时不该有卡片模型（那就成了「画空网格」）")
	}
	if !strings.Contains(section.Note, "agentPresets.list") {
		t.Fatalf("未知原因没点名缺的是哪条能力：%q", section.Note)
	}
}

// TestAgentPresetsEmptyRosterIsItsOwnState 钉住空名册自己的说法。
func TestAgentPresetsEmptyRosterIsItsOwnState(t *testing.T) {
	section := agentPresetSection(t, agentPresetDescribe(4, true), &apiclient.AgentPresetRoster{})
	if section.State != PanelEmpty {
		t.Fatalf("空名册 state = %q，应为 empty", section.State)
	}
	if section.Note == "" {
		t.Fatal("空名册必须有一句说明，不能只画一个空页")
	}
}

// TestAgentPresetsCardsFollowRosterOrderAndIsDefault 钉住卡片来自宿主真值。
func TestAgentPresetsCardsFollowRosterOrderAndIsDefault(t *testing.T) {
	section := agentPresetSection(t, agentPresetDescribe(4, true), agentPresetRosterFixture())
	if section.State != PanelOK {
		t.Fatalf("state = %q，应为 ok（note=%q）", section.State, section.Note)
	}
	panel := section.AgentPresets
	if panel == nil {
		t.Fatal("有名册却没有卡片模型")
	}
	want := []string{"default", "read-only", "workspace-write"}
	if len(panel.Cards) != len(want) {
		t.Fatalf("卡片 = %d 张，应为 %d", len(panel.Cards), len(want))
	}
	for i, id := range want {
		if panel.Cards[i].ID != id {
			t.Fatalf("第 %d 张卡是 %q，应为 %q（顺序是宿主的意图，不许按名字重排）",
				i, panel.Cards[i].ID, id)
		}
	}
	// isDefault 只有一条，且必须是宿主点名的那一条——客户端不许自己推。
	for i, card := range panel.Cards {
		want := want[i] == "default"
		if card.InUse != want {
			t.Fatalf("卡 %q 的 InUse = %v，应为 %v", card.ID, card.InUse, want)
		}
	}
	if !strings.Contains(panel.Intro, "选择 Agent 的工具和工作方式") {
		t.Fatalf("简介不是官方 sectionIntro 逐字：%q", panel.Intro)
	}
}

// TestAgentPresetsNameFallsBackToIDAndMissingDescription 钉住回落规则。
func TestAgentPresetsNameFallsBackToIDAndMissingDescription(t *testing.T) {
	roster := &apiclient.AgentPresetRoster{Presets: []apiclient.AgentPreset{
		{ID: "custom-x"}, // 既没有 name 也没有 description
	}}
	panel := agentPresetSection(t, agentPresetDescribe(1, true), roster).AgentPresets
	if panel == nil || len(panel.Cards) != 1 {
		t.Fatalf("卡片模型不对：%+v", panel)
	}
	card := panel.Cards[0]
	if card.Name != "custom-x" {
		t.Fatalf("name 缺席时应回落成 id，实得 %q", card.Name)
	}
	// 描述缺席不在这里编：渲染层读 `zhAgentPresetNoDesc`。
	if card.Description != "" {
		t.Fatalf("description 缺席时不该编一句话，实得 %q", card.Description)
	}
	if zhAgentPresetNoDesc != "暂无描述。" {
		t.Fatalf("官方 noDescription 不是这句：%q", zhAgentPresetNoDesc)
	}
}

// TestAgentPresetsWriteTargetFollowsDescribe 钉住写目标来自 describe 的事实。
func TestAgentPresetsWriteTargetFollowsDescribe(t *testing.T) {
	panel := agentPresetSection(t, agentPresetDescribe(7, true), agentPresetRosterFixture()).AgentPresets
	if panel == nil {
		t.Fatal("没有卡片模型")
	}
	if !panel.CanSetDefault {
		t.Fatalf("命名空间可写时不该禁写：%q", panel.DenyReason)
	}
	if panel.Revision != 7 {
		t.Fatalf("revision = %d，应为 describe 里的 7（乐观锁种子）", panel.Revision)
	}

	// 宿主没发布这个命名空间 -> 明说，不画一个点了没反应的卡。
	withoutNS := agentPresetSection(t, describeFixture(), agentPresetRosterFixture()).AgentPresets
	if withoutNS == nil {
		t.Fatal("名册在、命名空间不在时仍应列出名册")
	}
	if withoutNS.CanSetDefault {
		t.Fatal("命名空间不在时却报告可写——那会点出一个必然失败的动作")
	}
	if withoutNS.DenyReason == "" {
		t.Fatal("禁写必须写明原因")
	}

	// 字段级只读：整条命名空间可写，但这一字段被标成只读。
	readonlyField := agentPresetSection(t, agentPresetDescribe(3, false), agentPresetRosterFixture()).AgentPresets
	if readonlyField == nil || readonlyField.CanSetDefault {
		t.Fatal("字段级只读时仍报告可写")
	}
}

// TestAgentPresetsMakeDefaultWritesSelectedDefault 钉住「设为默认」写的是什么。
//
// 这是 #3 的核心：官方写的**不是** preset 自身属性，而是设置命名空间里的
// `agent-preset-registry.selectedDefault`；revision 是乐观锁。
func TestAgentPresetsMakeDefaultWritesSelectedDefault(t *testing.T) {
	p := &Plugin{}
	o := p.ensureOfficial()
	writer := &fakeWriter{section: apiclient.SettingsSectionView{
		NS: apiclient.SettingsNSAgentPreset, Revision: 8,
	}}
	refreshed := 0
	o.mu.Lock()
	o.writer = writer
	o.agentPresetRefresh = func() { refreshed++ }
	o.mu.Unlock()

	panel := agentPresetSection(t, agentPresetDescribe(7, true), agentPresetRosterFixture()).AgentPresets
	state := p.applyAgentPresetDefault(panel, panel.Cards[1]) // read-only

	if !state.Success() {
		t.Fatalf("写失败：%+v", state)
	}
	if len(writer.setCalls) != 1 {
		t.Fatalf("发了 %d 次写，应为 1", len(writer.setCalls))
	}
	call := writer.setCalls[0]
	if call.ns != "agent-preset-registry" {
		t.Fatalf("ns = %q，官方命名空间逐字是 agent-preset-registry", call.ns)
	}
	if len(call.path) != 1 || call.path[0] != "selectedDefault" {
		t.Fatalf("path = %v，应为 [selectedDefault]", call.path)
	}
	if call.value != "read-only" {
		t.Fatalf("写的值 = %v，应为被点的那条的 id", call.value)
	}
	if call.expectedRev == nil || *call.expectedRev != 7 {
		t.Fatalf("expectedRevision = %v，应为 describe 里的 7", call.expectedRev)
	}
	if refreshed != 1 {
		t.Fatalf("写成功后应触发一次名册重读，实得 %d 次", refreshed)
	}
}

// TestAgentPresetsMakeDefaultFailureIsNotSilentSuccess 钉住失败不刷新、原样渲染。
func TestAgentPresetsMakeDefaultFailureIsNotSilentSuccess(t *testing.T) {
	p := &Plugin{}
	o := p.ensureOfficial()
	writer := &fakeWriter{setErr: apiError(apiclient.SettingsCodeInvalidEnum,
		`preset "nope" 不存在`, "先用 agentPresets.list 确认 id", false)}
	refreshed := 0
	o.mu.Lock()
	o.writer = writer
	o.agentPresetRefresh = func() { refreshed++ }
	o.mu.Unlock()

	panel := agentPresetSection(t, agentPresetDescribe(7, true), agentPresetRosterFixture()).AgentPresets
	state := p.applyAgentPresetDefault(panel, panel.Cards[0])
	if state.Success() {
		t.Fatal("宿主拒绝了却报成功——那正是「静默成功」")
	}
	if state.Kind != WriteInvalid {
		t.Fatalf("invalid_enum 应分类成 WriteInvalid，实得 %q", state.Kind)
	}
	if refreshed != 0 {
		t.Fatalf("写失败时不该重读名册（实得 %d 次）", refreshed)
	}
	if !strings.Contains(state.Render(), "不存在") {
		t.Fatalf("宿主 message 没进渲染文案：%q", state.Render())
	}
}

// TestAgentPresetsMakeDefaultWithoutWriter 钉住"壳没接写路径"的诚实失败。
func TestAgentPresetsMakeDefaultWithoutWriter(t *testing.T) {
	p := &Plugin{}
	panel := agentPresetSection(t, agentPresetDescribe(7, true), agentPresetRosterFixture()).AgentPresets
	state := p.applyAgentPresetDefault(panel, panel.Cards[1])
	if state.Success() {
		t.Fatal("没有 writer 时不可能成功")
	}
	if state.Kind != WriteFailed {
		t.Fatalf("没有 writer 应分类成 WriteFailed，实得 %q", state.Kind)
	}
	if !strings.Contains(state.Render(), "未接") {
		t.Fatalf("文案没说清是壳没接线：%q", state.Render())
	}
}

// TestAgentPresetsPageIsItsOwnRenderFace 钉住这一页走自己的渲染面。
//
// 与「内置插件」页同一条判据：它不是「标题 + 一行一个控件」，Rows 必须是空的
// ——否则绘制层会在卡片网格之外再画一遍行。
func TestAgentPresetsPageIsItsOwnRenderFace(t *testing.T) {
	section := agentPresetSection(t, agentPresetDescribe(7, true), agentPresetRosterFixture())
	if len(section.Rows) != 0 {
		t.Fatalf("Agent 预设页不该有 PanelRow 行（实得 %d 行）", len(section.Rows))
	}
	if section.AgentPresets == nil {
		t.Fatal("没有卡片模型，绘制层就画不出这一页")
	}
}

// TestAgentPresetCardLabelUsesOfficialCopy 钉住可访问名用官方两条逐字文案
// （官方 `aria-label = ${selectionAction}: ${display.name}`）。
func TestAgentPresetCardLabelUsesOfficialCopy(t *testing.T) {
	if got := agentPresetCardLabel(AgentPresetCard{ID: "a", Name: "A"}); got != "设为新任务默认: A" {
		t.Fatalf("可选卡的标签 = %q", got)
	}
	if got := agentPresetCardLabel(AgentPresetCard{ID: "a", Name: "A", InUse: true}); got != "新任务默认: A" {
		t.Fatalf("已选卡的标签 = %q", got)
	}
	if zhAgentPresetSetDefault != "设为新任务默认" || zhAgentPresetInUse != "新任务默认" {
		t.Fatal("官方 zh 文案不是这两句")
	}
}

var _ = context.Background
