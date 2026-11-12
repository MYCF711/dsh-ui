package dssettings

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// describeFixture 是宿主三个命名空间（model / permission / llm）的真实形状。
//
// llm 的 value 按官方 retry-policy.ts 的形状给：`providers.<route>.retryPolicy`。
func describeFixture() *apiclient.SettingsDescribeResult {
	return &apiclient.SettingsDescribeResult{
		Writable:    true,
		HasDocument: false,
		Namespaces: []apiclient.SettingsNamespaceView{
			{NS: apiclient.SettingsNSModel, Revision: 3, Value: json.RawMessage(`{"defaultModel":"group/deepseek-v4.1"}`)},
			{NS: apiclient.SettingsNSPermission, Revision: 2, Value: json.RawMessage(`{"defaultPreset":"ask"}`)},
			{NS: apiclient.SettingsNSLLM, Revision: 5, Value: json.RawMessage(`{
				"providers": {
					"deepseek": {
						"retryPolicy": {
							"mode": "normal",
							"maxRetries": 3,
							"retryableCodes": ["RATE_LIMIT", "SERVER"],
							"backoff": {"initialDelayMs": 500, "maxDelayMs": 10000, "jitterRatio": 0.1}
						}
					}
				}
			}`)},
		},
		Sections: []apiclient.SettingsSectionView{
			{
				NS: apiclient.SettingsNSModel, Title: "模型", Writable: true, Revision: 3,
				Fields: []apiclient.SettingsFieldView{{
					Name: "defaultModel", Path: []string{"defaultModel"}, Type: "string",
					Current: "group/deepseek-v4.1", Writable: true,
				}},
			},
			{
				NS: apiclient.SettingsNSPermission, Title: "权限", Writable: true, Revision: 2,
				Fields: []apiclient.SettingsFieldView{{
					Name: "defaultPreset", Path: []string{"defaultPreset"}, Type: "string",
					Current: "ask", Writable: true,
					Enum: []string{"deny", "ask", "allow", "auto"},
				}},
			},
		},
	}
}

func crendetialFacts() HostFacts {
	f := fullDesktopFacts()
	f.CredentialStored = true
	return f
}

// TestBuildPanelRendersAllFiveSections 钉住 5 个 section 都渲染。
func TestBuildPanelRendersAllFiveSections(t *testing.T) {
	model := BuildPanel(describeFixture(), crendetialFacts())
	want := []string{"account", "general", "models", "plugins", "agent-presets"}
	if len(model.Sections) != len(want) {
		t.Fatalf("渲染 section = %d 个，官方是 %d 个：%+v", len(model.Sections), len(want), sectionIDs(model.Sections))
	}
	for i, id := range want {
		if model.Sections[i].ID != id {
			t.Fatalf("第 %d 个 section = %q，期望 %q", i, model.Sections[i].ID, id)
		}
		if model.Sections[i].Title == "" {
			t.Fatalf("%s 没有标题", id)
		}
		if model.Sections[i].Icon == "" {
			t.Fatalf("%s 没有图标", id)
		}
	}
	// general 页的可见行数（官方 14）。
	general, _ := model.Section("general")
	if got := len(general.Rows); got != 14 {
		t.Fatalf("general 行数 = %d，官方默认 14 行", got)
	}
	if model.RowCount != 14 {
		t.Fatalf("RowCount = %d，应为 14", model.RowCount)
	}
}

// TestBuildPanelNeverFabricatesValues 是「绝不编 0」的核心断言。
//
// 只给 permission 一个命名空间：其余 general 行的数据源都不在 describe 里，
// 必须渲染成「未知」，且**任何行都不许出现 0 / 空串冒充真值**。
func TestBuildPanelNeverFabricatesValues(t *testing.T) {
	describe := &apiclient.SettingsDescribeResult{
		Writable: true,
		Namespaces: []apiclient.SettingsNamespaceView{
			{NS: apiclient.SettingsNSPermission, Revision: 1},
		},
		Sections: []apiclient.SettingsSectionView{
			{
				NS: apiclient.SettingsNSPermission, Title: "权限", Writable: true, Revision: 1,
				Fields: []apiclient.SettingsFieldView{{
					Name: "defaultPreset", Path: []string{"defaultPreset"}, Type: "string",
					Current: "ask", Writable: true,
				}},
			},
		},
	}
	model := BuildPanel(describe, fullDesktopFacts())
	general, _ := model.Section("general")

	var unknown int
	for _, row := range general.Rows {
		if row.ID == "permission" {
			if !row.Known || row.Display() != "ask" {
				t.Fatalf("permission 行应有真值 ask，实得 known=%v display=%q", row.Known, row.Display())
			}
			continue
		}
		if row.Known {
			continue // 有真值的行（例如 current-version 走宿主事实）不在此列
		}
		unknown++
		if got := row.Display(); got != "未知" {
			t.Fatalf("%s 未知时应显示「未知」，实得 %q", row.ID, got)
		}
		if row.Value != "" {
			t.Fatalf("%s 未知时 Value 必须为空（实得 %q）——显示层不许拿 0/空串当真值", row.ID, row.Value)
		}
		if !row.Disabled {
			t.Fatalf("%s 未知时应该禁用写入（不知道当前值就不能改）", row.ID)
		}
	}
	if unknown == 0 {
		t.Fatal("这个夹具里应该有多行未知（ui-theme / ui-chat 等命名空间都没给）")
	}
	// current-version 走宿主事实（build 版本），不是 describe 命名空间。
	for _, row := range general.Rows {
		if row.ID == "current-version" {
			if !row.Known || row.Value != "0.2.2-rc.2" {
				t.Fatalf("current-version 应显示 build 版本，实得 known=%v value=%q", row.Known, row.Value)
			}
		}
	}
	// 未知字段清单必须登记（报告/测试用）。
	if len(model.Unknown) == 0 {
		t.Fatal("Unknown 清单为空：未知字段必须被登记，不能静默")
	}
	for _, field := range model.Unknown {
		if !strings.Contains(field, ".") {
			t.Fatalf("Unknown 里的 %q 不是 <ns>.<field> 形式", field)
		}
	}
}

// TestBuildPanelNilDescribeStillRendersStructure 钉住「结构先于数据」。
func TestBuildPanelNilDescribeStillRendersStructure(t *testing.T) {
	model := BuildPanel(nil, fullDesktopFacts())
	if len(model.Sections) != 4 { // 没有 credential-stored → 没有 account
		t.Fatalf("无 describe 时 section = %d 个（general/models/plugins/agent-presets）", len(model.Sections))
	}
	general, ok := model.Section("general")
	if !ok {
		t.Fatal("无 describe 时 general 页仍要在（结构骨架）")
	}
	if len(general.Rows) != 14 {
		t.Fatalf("general 行数 = %d，结构骨架应保持 14 行", len(general.Rows))
	}
	for _, row := range general.Rows {
		// current-version 走宿主事实（build 注入的 DSH_CLIENT_VERSION），
		// 不是 settings.describe 的字段，所以它有真值是**正确**的。
		if row.ID == "current-version" {
			if !row.Known || row.Value != "0.2.2-rc.2" {
				t.Fatalf("current-version 应来自 build 注入，实得 known=%v value=%q", row.Known, row.Value)
			}
			continue
		}
		if row.Known {
			t.Fatalf("%s 在无 describe 时不该有真值", row.ID)
		}
	}
	if general.State != PanelUnknown {
		t.Fatalf("general 状态 = %q，应为 unknown", general.State)
	}
}

// TestBuildPanelAccountHiddenWithoutCredential 钉住条件 section 的整段隐藏。
func TestBuildPanelAccountHiddenWithoutCredential(t *testing.T) {
	model := BuildPanel(describeFixture(), fullDesktopFacts())
	if _, ok := model.Section("account"); ok {
		t.Fatal("没有 credential-stored 时 account 不该出现（官方条件注册）")
	}
	withCred := BuildPanel(describeFixture(), crendetialFacts())
	if _, ok := withCred.Section("account"); !ok {
		t.Fatal("credential-stored 时 account 必须出现")
	}
}

// TestBuildPanelReadOnlyDisablesRows 钉住只读命名空间的渲染。
func TestBuildPanelReadOnlyDisablesRows(t *testing.T) {
	describe := describeFixture()
	describe.Writable = false
	for i := range describe.Sections {
		describe.Sections[i].Writable = false
		for j := range describe.Sections[i].Fields {
			describe.Sections[i].Fields[j].Writable = false
		}
	}
	model := BuildPanel(describe, fullDesktopFacts())
	general, _ := model.Section("general")
	var checked bool
	for _, row := range general.Rows {
		if row.ID != "permission" {
			continue
		}
		checked = true
		if !row.Disabled {
			t.Fatal("只读命名空间的行必须禁用")
		}
		if !strings.Contains(row.Reason, "只读") {
			t.Fatalf("只读行必须写清原因，实得 %q", row.Reason)
		}
	}
	if !checked {
		t.Fatal("没找到 permission 行")
	}
}

// TestBuildPanelRetryPolicyRowsAreRendered 钉住 retryPolicy 的 8 个 code 候选。
func TestBuildPanelRetryPolicyRowsAreRendered(t *testing.T) {
	model := BuildPanel(describeFixture(), fullDesktopFacts())
	models, ok := model.Section("models")
	if !ok {
		t.Fatal("缺 models 页")
	}
	byID := map[string]PanelRow{}
	for _, row := range models.Rows {
		byID[row.ID] = row
	}

	// 8 个官方 code 候选一个不少，且顺序与定义顺序一致。
	var codeRows []string
	for _, row := range models.Rows {
		if strings.HasPrefix(row.ID, "retry-deepseek-code-") {
			codeRows = append(codeRows, strings.TrimPrefix(row.ID, "retry-deepseek-code-"))
		}
	}
	if len(codeRows) != len(RetryCodeChoices) {
		t.Fatalf("code 行 = %d 个，官方 8 个：%v", len(codeRows), codeRows)
	}
	for i, want := range RetryCodeChoices {
		if codeRows[i] != want {
			t.Fatalf("第 %d 个 code 候选 = %q，官方定义顺序为 %q", i, codeRows[i], want)
		}
	}
	// 选中态来自宿主 retryableCodes（RATE_LIMIT / SERVER 勾上）。
	for _, code := range []string{"RATE_LIMIT", "SERVER"} {
		row := byID["retry-deepseek-code-"+code]
		if row.Selected != "true" {
			t.Fatalf("%s 应为选中态，实得 %q", code, row.Selected)
		}
	}
	for _, code := range []string{"EMPTY_RESPONSE", "QUOTA", "TIMEOUT", "TRANSPORT", "MALFORMED_RESPONSE", "STREAM_CLOSED"} {
		row := byID["retry-deepseek-code-"+code]
		if row.Selected != "false" {
			t.Fatalf("%s 不应选中，实得 %q", code, row.Selected)
		}
	}
	// 模式 / 次数 / 三个 backoff 字段都在，且带官方边界。
	for _, id := range []string{
		"retry-deepseek-mode", "retry-deepseek-maxRetries",
		"retry-deepseek-initialDelayMs", "retry-deepseek-maxDelayMs", "retry-deepseek-jitterRatio",
	} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("缺行 %s（全量行：%v）", id, panelRowIDs(models.Rows))
		}
	}
	if got := byID["retry-deepseek-initialDelayMs"].Bounds.Min; got != PositiveLowerBound {
		t.Fatalf("initialDelayMs 的下界 = %v，官方是 %v（5e-324）", got, PositiveLowerBound)
	}
	if got := byID["retry-deepseek-maxDelayMs"].Bounds.Max; got != float64(RetryMaxTimerDelayMs) {
		t.Fatalf("maxDelayMs 的上界 = %v，官方 MAX_TIMER_DELAY_MS = %d", got, RetryMaxTimerDelayMs)
	}
	if got := byID["retry-deepseek-jitterRatio"].Bounds.Min; got != 0 {
		t.Fatalf("jitterRatio 的下界 = %v，官方是 0", got)
	}
	if got := byID["retry-deepseek-maxRetries"].Value; got != "3" {
		t.Fatalf("maxRetries 显示值 = %q，应为宿主值 3", got)
	}
}

// TestBuildPanelMissingLLMShowsUnknown 钉住 llm 缺失时的降级（不编默认策略）。
func TestBuildPanelMissingLLMShowsUnknown(t *testing.T) {
	describe := describeFixture()
	describe.Namespaces = []apiclient.SettingsNamespaceView{{NS: apiclient.SettingsNSModel, Revision: 1}}
	model := BuildPanel(describe, fullDesktopFacts())
	models, _ := model.Section("models")
	var found bool
	for _, row := range models.Rows {
		if row.ID == "retry-policy-unknown" {
			found = true
			if row.Known {
				t.Fatal("llm 缺失时 retryPolicy 不该有真值")
			}
			if !strings.Contains(row.Reason, "llm") {
				t.Fatalf("未知原因应说明缺哪个命名空间，实得 %q", row.Reason)
			}
		}
	}
	if !found {
		t.Fatalf("llm 缺失时应渲染一条未知说明行（实得 %v）", panelRowIDs(models.Rows))
	}
}

func sectionIDs(sections []PanelSection) []string {
	out := make([]string, 0, len(sections))
	for _, s := range sections {
		out = append(out, s.ID)
	}
	return out
}

func panelRowIDs(rows []PanelRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}
