// settings_items_test.go —— 官方 4 条插件设置页里**能脱离真机验证的判断**（本轮 #39/#45/#46）。
//
// 绘制与点击由 settings_items_visual_test.go 走真 Viva；这里钉住四类东西：
//
//	① whileServed：宿主不服务那条命名空间时**整张卡不存在**（不是灰色）；
//	② 官方 `SettingsFormModel` 的语义：format / parse / plan（unset、跳过、invalid）、
//	   overridden 的判定、`limitField` 的整数+下限判据；
//	③ 宿主没给值 → 未知 + 禁用（不许把缺失变成 0，也不许空保存 unset 一个未知值）；
//	④ 几何台账逐行对齐（官方 CSS 原文 ↔ 本仓常量）。
package dssidebar

import (
	"errors"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// settingsItemDescribeFixture 是这 3 页的宿主 describe 替身（形状与真宿主一致：
// `sections[].fields[]` 带 current/default/overridden/writable + revision）。
func settingsItemDescribeFixture() *apiclient.SettingsDescribeResult {
	describe := &apiclient.SettingsDescribeResult{Writable: true, HasDocument: true}
	add := func(ns string, revision int, fields ...apiclient.SettingsFieldView) {
		describe.Namespaces = append(describe.Namespaces, apiclient.SettingsNamespaceView{NS: ns, Revision: revision})
		describe.Sections = append(describe.Sections, apiclient.SettingsSectionView{
			NS: ns, Writable: true, Revision: revision, Fields: fields,
		})
	}
	add(settingsNSPwshSandbox, 3,
		apiclient.SettingsFieldView{Name: settingsFieldShellTimeout, Path: []string{settingsFieldShellTimeout}, Type: "integer", Current: float64(120000), Default: float64(120000), Writable: true},
		apiclient.SettingsFieldView{Name: settingsFieldShellMaxOutput, Path: []string{settingsFieldShellMaxOutput}, Type: "integer", Current: float64(64000), Default: float64(64000), Writable: true},
	)
	add(settingsNSAgentLoop, 5,
		apiclient.SettingsFieldView{Name: settingsFieldMaxParallelToolCalls, Path: []string{settingsFieldMaxParallelToolCalls}, Type: "integer", Current: float64(10), Default: float64(10), Writable: true},
	)
	add(settingsNSSubagent, 7,
		apiclient.SettingsFieldView{Name: settingsFieldMaxDepth, Path: []string{settingsFieldMaxDepth}, Type: "integer", Current: float64(1), Default: float64(1), Writable: true},
		apiclient.SettingsFieldView{Name: settingsFieldMaxActiveSubagents, Path: []string{settingsFieldMaxActiveSubagents}, Type: "integer", Current: float64(8), Default: float64(8), Overridden: true, Writable: true},
	)
	return describe
}

// settingsItemByID 取一张卡的投影（测试用）。
func settingsItemByID(t *testing.T, describe *apiclient.SettingsDescribeResult, id string) settingsItemCardState {
	t.Helper()
	for _, card := range settingsItemCards(describe, settingsItemFormSnapshot{drafts: map[string]settingsItemDraft{}}) {
		if card.Item.ID == id {
			return card
		}
	}
	t.Fatalf("卡片 %q 不存在；现有卡片=%v", id, settingsItemCardIDs(describe))
	return settingsItemCardState{}
}

func settingsItemCardIDs(describe *apiclient.SettingsDescribeResult) []string {
	var ids []string
	for _, card := range settingsItemCards(describe, settingsItemFormSnapshot{drafts: map[string]settingsItemDraft{}}) {
		ids = append(ids, card.Item.ID)
	}
	return ids
}

// TestSettingsItemGeometryMatchesOfficialCSS 逐行核对几何台账（Official 必须等于 Impl）。
func TestSettingsItemGeometryMatchesOfficialCSS(t *testing.T) {
	rows := settingsItemGeometry()
	if len(rows) < 20 {
		t.Fatalf("几何台账只有 %d 行，官方这批字段的几何远不止这些", len(rows))
	}
	for _, row := range rows {
		if row.Anchor == "" {
			t.Errorf("%s.%s 没有官方锚点——那就是猜的", row.Node, row.Prop)
		}
		if row.Official != row.Impl {
			t.Errorf("%s.%s 官方=%v 实现=%v（锚点 %s）", row.Node, row.Prop, row.Official, row.Impl, row.Anchor)
		}
	}
	// 几个容易被改坏的数（官方原文写死）单独再钉一遍。
	if itemFieldInputH != 34 || itemFieldPadY != 12 || itemFormSavePadX != 14 || itemTagLine != 17 {
		t.Fatal("字段/按钮/Tag 的关键几何被改了：官方是 input 34、field 12、save 14、Tag 17")
	}
}

// TestSettingsItemsWhileServed 钉住官方 `whileServed` 的存在条件。
func TestSettingsItemsWhileServed(t *testing.T) {
	describe := settingsItemDescribeFixture()
	if got := settingsItemCardIDs(describe); len(got) != 3 {
		t.Fatalf("三条命名空间都服务时应有 3 张卡，实际 %v", got)
	}
	// 只服务 agent-loop：另外两张整张不存在（不是灰色、不是禁用）。
	only := &apiclient.SettingsDescribeResult{Writable: true}
	for _, ns := range describe.Namespaces {
		if ns.NS == settingsNSAgentLoop {
			only.Namespaces = append(only.Namespaces, ns)
		}
	}
	for _, section := range describe.Sections {
		if section.NS == settingsNSAgentLoop {
			only.Sections = append(only.Sections, section)
		}
	}
	if got := settingsItemCardIDs(only); len(got) != 1 || got[0] != "agent-loop" {
		t.Fatalf("只服务 agent-loop 时应当只剩那一张卡，实际 %v", got)
	}
	// 一条都没服务 / 壳还没拉到 describe：一张卡都没有。
	if got := settingsItemCardIDs(&apiclient.SettingsDescribeResult{}); len(got) != 0 {
		t.Fatalf("宿主什么都没服务时不该有卡片，实际 %v", got)
	}
	if got := settingsItemCardIDs(nil); len(got) != 0 {
		t.Fatalf("describe 还没到时不该有卡片，实际 %v", got)
	}
}

// TestSettingsItemShellBindsServedExecutor 钉住 `served.has(PWSH_NS) ? pwsh : bash`。
func TestSettingsItemShellBindsServedExecutor(t *testing.T) {
	if got := settingsItemShellNS(map[string]bool{settingsNSPwshSandbox: true, settingsNSBashSandbox: true}); got != settingsNSPwshSandbox {
		t.Fatalf("两条都服务时应绑 pwsh（官方 index.ts:51），实际 %q", got)
	}
	if got := settingsItemShellNS(map[string]bool{settingsNSBashSandbox: true}); got != settingsNSBashSandbox {
		t.Fatalf("只服务 bash 时应绑 bash，实际 %q", got)
	}
	if got := settingsItemShellNS(map[string]bool{}); got != "" {
		t.Fatalf("两条都没服务时应返回空（那张卡整张不存在），实际 %q", got)
	}
	// 端到端：只有 bash-sandbox 时，卡片的 section 指向 bash-sandbox。
	describe := &apiclient.SettingsDescribeResult{
		Writable:   true,
		Namespaces: []apiclient.SettingsNamespaceView{{NS: settingsNSBashSandbox, Revision: 2}},
		Sections: []apiclient.SettingsSectionView{{
			NS: settingsNSBashSandbox, Writable: true, Revision: 2,
			Fields: []apiclient.SettingsFieldView{{
				Name: settingsFieldShellTimeout, Path: []string{settingsFieldShellTimeout},
				Type: "integer", Current: float64(120000), Default: float64(120000), Writable: true,
			}},
		}},
	}
	card := settingsItemByID(t, describe, "shell")
	if len(card.Sections) != 1 || card.Sections[0].NS != settingsNSBashSandbox {
		t.Fatalf("shell 卡应绑 bash-sandbox，实际 %+v", card.Sections)
	}
}

// TestSettingsItemFieldProjection 钉住 `field()` 的投影规则。
func TestSettingsItemFieldProjection(t *testing.T) {
	describe := settingsItemDescribeFixture()
	card := settingsItemByID(t, describe, "subagent")
	if len(card.Sections) != 1 || len(card.Sections[0].Fields) != 2 {
		t.Fatalf("subagent 应有一节两个字段，实际 %+v", card.Sections)
	}
	depth, active := card.Sections[0].Fields[0], card.Sections[0].Fields[1]
	if depth.Spec.Name != settingsFieldMaxDepth || !depth.Known || depth.Text != "1" {
		t.Fatalf("maxDepth 投影=%+v，want 已知且文本 1", depth)
	}
	if depth.Overridden {
		t.Fatal("没被用户写过的字段不该带「已覆盖」")
	}
	if !active.Overridden {
		t.Fatal("user 层有值（overridden=true）的字段必须带「已覆盖」——官方 stored(name)")
	}
	if card.Sections[0].Revision != 7 {
		t.Fatalf("section revision=%d，want 7（写路径的乐观锁）", card.Sections[0].Revision)
	}
	// 宿主没给 current → 未知 + 禁用（不许显示 0）。
	describe.Sections[2].Fields[0].Current = nil
	card = settingsItemByID(t, describe, "subagent")
	if field := card.Sections[0].Fields[0]; field.Known || !field.Disabled || field.Note == "" {
		t.Fatalf("缺 current 时必须未知+禁用+有原因，实际 %+v", field)
	}

	// section 在但 writable=false → 字段禁用（官方 disabled={!state.writable}）。
	describe = settingsItemDescribeFixture()
	describe.Sections[1].Writable = false
	card = settingsItemByID(t, describe, "agent-loop")
	if card.Writable {
		t.Fatal("section 只读时卡片必须是只读")
	}
	if field := card.Sections[0].Fields[0]; !field.Disabled {
		t.Fatal("只读时字段必须禁用")
	}
}

// TestSettingsItemNumberFormat 钉住官方 `settingsNumberField.format`。
func TestSettingsItemNumberFormat(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{float64(10), "10"},
		{float64(120000), "120000"},
		{float64(1.5), "1.5"},
		{int(7), "7"},
		{nil, ""},
		{"120000", ""}, // 官方 typeof number 才格式化；字符串不是数字
		{float64(0), "0"},
	}
	for _, tc := range cases {
		if got := formatItemNumber(tc.value); got != tc.want {
			t.Errorf("formatItemNumber(%#v)=%q，want %q", tc.value, got, tc.want)
		}
	}
}

// TestSettingsItemParseMatchesOfficial 钉住 parse 与 `limitField` 的判据。
func TestSettingsItemParseMatchesOfficial(t *testing.T) {
	plain := settingsItemField{Name: settingsFieldMaxParallelToolCalls}
	limit0 := settingsItemField{Name: settingsFieldMaxDepth, Limit: true, Min: 0}
	limit1 := settingsItemField{Name: settingsFieldMaxActiveSubagents, Limit: true, Min: 1}

	if _, clear, ok := parseItemField(plain, "  "); !ok || !clear {
		t.Fatal("空白必须是 clear（官方 {kind:'clear'}）")
	}
	if value, _, ok := parseItemField(plain, " 12 "); !ok || value != 12 {
		t.Fatalf("带空格的 12 应解析成 12，实际 %v ok=%v", value, ok)
	}
	if _, _, ok := parseItemField(plain, "abc"); ok {
		t.Fatal("非数字必须解析失败（官方 undefined → invalid）")
	}
	if _, _, ok := parseItemField(plain, "1.5"); !ok {
		t.Fatal("裸 number 页接受小数（官方 settingsNumberField 不看 step）")
	}
	if value, _, ok := parseItemField(limit0, "0"); !ok || value != 0 {
		t.Fatal("maxDepth 的 0 是官方合法值（禁用子智能体）")
	}
	if _, _, ok := parseItemField(limit0, "-1"); ok {
		t.Fatal("maxDepth 的负数必须 rejected（.min(0)）")
	}
	if _, _, ok := parseItemField(limit1, "0"); ok {
		t.Fatal("maxActiveSubagents 的 0 必须 rejected（.min(1)）")
	}
	if _, _, ok := parseItemField(limit1, "2.5"); ok {
		t.Fatal("limit 字段必须整数（官方 Number.isSafeInteger）")
	}
}

// TestSettingsItemPlanMatchesOfficial 钉住 plan() 的四条分支。
func TestSettingsItemPlanMatchesOfficial(t *testing.T) {
	describe := settingsItemDescribeFixture()
	// ① 文本等于当前值 → 不发操作（官方跳过）。
	plan := settingsItemPlanOf(t, describe, map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSAgentLoop, settingsFieldMaxParallelToolCalls): {Text: "10"},
	})
	if plan.Dirty() {
		t.Fatalf("与当前值相同的草稿不该发操作，实际 %+v", plan.Sections)
	}
	// ② 真的改了 → 一条 set，值是数字（不是字符串）。
	plan = settingsItemPlanOf(t, describe, map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSAgentLoop, settingsFieldMaxParallelToolCalls): {Text: "16"},
	})
	if len(plan.Sections) != 1 || len(plan.Sections[0].Ops) != 1 {
		t.Fatalf("改一个字段应产生一条操作，实际 %+v", plan.Sections)
	}
	op := plan.Sections[0].Ops[0]
	if op.Op != "set" || op.Path[0] != settingsFieldMaxParallelToolCalls {
		t.Fatalf("操作=%+v，want set maxParallelToolCalls", op)
	}
	if value, ok := op.Value.(float64); !ok || value != 16 {
		t.Fatalf("值必须是数字 16（官方 {kind:'set',value}），实际 %#v", op.Value)
	}
	if plan.Sections[0].NS != settingsNSAgentLoop {
		t.Fatalf("操作的命名空间=%q", plan.Sections[0].NS)
	}
	// ③ 清空 = unset，但**只有 user 层有值时才发**（官方 `stored` 判据）。
	plan = settingsItemPlanOf(t, describe, map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSAgentLoop, settingsFieldMaxParallelToolCalls): {Text: ""},
	})
	if plan.Dirty() {
		t.Fatal("没被覆盖过的字段清空不该发 unset（会写一条无意义的 unset）")
	}
	plan = settingsItemPlanOf(t, describe, map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSSubagent, settingsFieldMaxActiveSubagents): {Text: ""},
	})
	if len(plan.Sections) != 1 || len(plan.Sections[0].Ops) != 1 || plan.Sections[0].Ops[0].Op != "unset" {
		t.Fatalf("被覆盖过的字段清空必须发 unset，实际 %+v", plan.Sections)
	}
	// ④ reset 草稿 = clear + default 文本：发 unset。
	plan = settingsItemPlanOf(t, describe, map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSSubagent, settingsFieldMaxActiveSubagents): {Text: "8", Clear: true},
	})
	if len(plan.Sections) != 1 || plan.Sections[0].Ops[0].Op != "unset" {
		t.Fatalf("reset（clear）必须发 unset，实际 %+v", plan.Sections)
	}
	// ⑤ 解析不了 → invalid（官方那时 save 不动）。
	plan = settingsItemPlanOf(t, describe, map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSAgentLoop, settingsFieldMaxParallelToolCalls): {Text: "abc"},
	})
	if !plan.Invalid || plan.Dirty() {
		t.Fatalf("坏输入必须只置 invalid、不发操作，实际 invalid=%v dirty=%v", plan.Invalid, plan.Dirty())
	}
	card := settingsItemCardState{Plan: plan, Writable: true}
	if card.CanSave() {
		t.Fatal("invalid 时保存按钮必须是禁用的（官方 disabled 判据）")
	}
}

// settingsItemPlanOf 用给定的草稿造一张卡的呈现状态，返回它的 plan。
func settingsItemPlanOf(t *testing.T, describe *apiclient.SettingsDescribeResult, drafts map[string]settingsItemDraft) settingsItemPlan {
	t.Helper()
	var plan settingsItemPlan
	for _, card := range settingsItemCards(describe, settingsItemFormSnapshot{drafts: drafts}) {
		plan.Sections = append(plan.Sections, card.Plan.Sections...)
		if card.Plan.Invalid {
			plan.Invalid = true
		}
	}
	return plan
}

// TestSettingsItemSaveSuccessClearsDrafts 钉住 save() 的成功路径。
func TestSettingsItemSaveSuccessClearsDrafts(t *testing.T) {
	p := &Plugin{}
	form := p.ensureSettingsItemForm()
	form.stage(settingsNSAgentLoop, settingsFieldMaxParallelToolCalls, settingsItemDraft{Text: "16"}, 5)
	drafts, baseline, _ := form.snapshot()
	mutator := &settingsItemMutatorStub{revision: 6}
	plan := settingsItemPlanOf(t, settingsItemDescribeFixture(), drafts)
	p.applySettingsItemSave(pluginapi.Actions{SettingsMutate: mutator.settingsMutate}, plan, baseline, form, drafts)

	if mutator.calls != 1 {
		t.Fatalf("应当只发一次 mutate，实际 %d", mutator.calls)
	}
	if mutator.lastNS != settingsNSAgentLoop || mutator.lastRevision == nil || *mutator.lastRevision != 5 {
		t.Fatalf("写目标/revision 不对：ns=%q rev=%v（官方用 baseline revision 做 fence）", mutator.lastNS, mutator.lastRevision)
	}
	if _, _, write := form.snapshot(); write.Failed || write.Saving {
		t.Fatalf("成功后写状态应为空，实际 %+v", write)
	}
	drafts, baseline, _ = form.snapshot()
	if len(drafts) != 0 || len(baseline) != 0 {
		t.Fatalf("成功后必须清草稿与 baseline，实际 drafts=%v baseline=%v", drafts, baseline)
	}
}

// TestSettingsItemSaveFailureKeepsDrafts 钉住 save() 的失败路径（不静默、不清草稿）。
func TestSettingsItemSaveFailureKeepsDrafts(t *testing.T) {
	p := &Plugin{}
	form := p.ensureSettingsItemForm()
	form.stage(settingsNSSubagent, settingsFieldMaxDepth, settingsItemDraft{Text: "2"}, 7)
	drafts, baseline, _ := form.snapshot()
	mutator := &settingsItemMutatorStub{err: errors.New("revision_conflict: 409")}
	plan := settingsItemPlanOf(t, settingsItemDescribeFixture(), drafts)
	p.applySettingsItemSave(pluginapi.Actions{SettingsMutate: mutator.settingsMutate}, plan, baseline, form, drafts)

	_, _, write := form.snapshot()
	if !write.Failed || write.Detail == "" {
		t.Fatalf("失败必须置 failed 并带真实原因，实际 %+v", write)
	}
	drafts, _, _ = form.snapshot()
	if len(drafts) != 1 {
		t.Fatalf("失败必须保留草稿（官方保留编辑上下文），实际 %v", drafts)
	}
	// 壳没接 mutate：也是失败，不是静默成功。
	p2 := &Plugin{}
	form2 := p2.ensureSettingsItemForm()
	form2.stage(settingsNSSubagent, settingsFieldMaxDepth, settingsItemDraft{Text: "2"}, 7)
	p2.applySettingsItemSave(pluginapi.Actions{}, plan, map[string]int{settingsNSSubagent: 7}, form2, drafts)
	if _, _, write := form2.snapshot(); !write.Failed {
		t.Fatal("壳没接 settings.mutate 时必须失败")
	}
}

// settingsItemMutatorStub 是写路径替身（线程安全由调用方保证：这里同步调）。
type settingsItemMutatorStub struct {
	calls        int
	lastNS       string
	lastRevision *int
	revision     int
	err          error
}

func (s *settingsItemMutatorStub) settingsMutate(ns string, ops []apiclient.SettingsOp, expectedRevision int) (apiclient.SettingsNamespaceView, error) {
	s.calls++
	s.lastNS = ns
	revision := expectedRevision
	s.lastRevision = &revision
	if s.err != nil {
		return apiclient.SettingsNamespaceView{}, s.err
	}
	return apiclient.SettingsNamespaceView{NS: ns, Revision: s.revision}, nil
}
