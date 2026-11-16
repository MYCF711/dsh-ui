package dssidebar

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func settingsSubagentCatalogFixture() *apiclient.ModelsPageSnapshot {
	return &apiclient.ModelsPageSnapshot{Catalog: &apiclient.ModelCatalog{
		Groups: []apiclient.ModelProvider{
			{ID: "provider-a", Name: "提供方 A", Models: []apiclient.CatalogModel{{ID: "model-a1", Name: "模型 A1"}, {ID: "model-a2", Name: "模型 A2"}}},
			{ID: "provider-b", Name: "提供方 B", Models: []apiclient.CatalogModel{{ID: "model-b1", Name: "模型 B1"}}},
		},
	}}
}

func settingsSubagentCardWithFallback(t *testing.T, describe *apiclient.SettingsDescribeResult, fallback settingsItemFormSnapshot) settingsItemCardState {
	t.Helper()
	for _, card := range settingsItemCards(describe, fallback) {
		if card.Item.ID == "subagent" {
			return card
		}
	}
	t.Fatal("subagent 卡不存在")
	return settingsItemCardState{}
}

func enableSettingsSubagentFixture(t *testing.T, describe *apiclient.SettingsDescribeResult) {
	t.Helper()
	for index := range describe.Namespaces {
		if describe.Namespaces[index].NS == settingsNSSubagentModelSelection {
			describe.Namespaces[index].Value = json.RawMessage(`{"enabled":true,"allowedModels":[{"provider":"saved-provider","model":"saved-model"}]}`)
		}
	}
	for index := range describe.Sections {
		if describe.Sections[index].NS == settingsNSSubagentModelSelection {
			describe.Sections[index].Fields[0].Current = true
		}
	}
}

func TestSettingsSubagentModelCandidatesJoinCatalogAndStaleRoutes(t *testing.T) {
	describe := settingsItemDescribeFixture()
	enableSettingsSubagentFixture(t, describe)
	card := settingsSubagentCardWithFallback(t, describe, settingsItemFormSnapshot{
		drafts: map[string]settingsItemDraft{}, catalogWired: true, catalog: settingsSubagentCatalogFixture(),
	})
	model := card.Model
	if model == nil || !model.Available || !model.Known || !model.Enabled {
		t.Fatalf("模型选择 section 投影不完整：%+v", model)
	}
	if model.CatalogStatus != settingsSubagentCatalogReady || model.CatalogPartial {
		t.Fatalf("目录状态=%q partial=%v，want ready/false", model.CatalogStatus, model.CatalogPartial)
	}
	if len(model.Candidates) != 4 {
		t.Fatalf("候选应为目录 3 条 + 失效已存 1 条，实际 %+v", model.Candidates)
	}
	stale := model.Candidates[len(model.Candidates)-1]
	if stale.Available || !stale.Selected || stale.Provider != "saved-provider" || stale.Model != "saved-model" {
		t.Fatalf("失效路由必须保留且可移除，实际 %+v", stale)
	}
	available, unavailable := groupSettingsSubagentCandidates(model.Candidates)
	if len(available) != 2 || len(available[0].Candidates) != 2 || len(available[1].Candidates) != 1 {
		t.Fatalf("provider 分组不符合目录顺序：%+v", available)
	}
	if len(unavailable.Candidates) != 1 || unavailable.Name != zhSubagentModelStaleGroup {
		t.Fatalf("失效组不符合官方形状：%+v", unavailable)
	}
}

func TestSettingsSubagentModelPlanWritesBothFieldsAtOneRevision(t *testing.T) {
	describe := settingsItemDescribeFixture()
	enableSettingsSubagentFixture(t, describe)
	form := (&Plugin{}).ensureSettingsItemForm()
	base := settingsSubagentCardWithFallback(t, describe, settingsItemFormSnapshot{
		drafts: map[string]settingsItemDraft{}, catalogWired: true, catalog: settingsSubagentCatalogFixture(),
	})
	key := settingsSubagentModelKey(settingsSubagentModelRoute{Provider: "provider-a", Model: "model-a1"})
	routes, ok := toggleSubagentModel(base.Model.DesiredRoutes, base.Model.Candidates, key)
	if !ok {
		t.Fatal("目录里的真实候选没有切换成功")
	}
	form.stageSubagentModel(base.Model, true, routes)
	drafts, baseline, write := form.snapshot()
	card := settingsSubagentCardWithFallback(t, describe, settingsItemFormSnapshot{
		drafts: drafts, baseline: baseline, write: write, catalogWired: true, catalog: settingsSubagentCatalogFixture(),
	})
	if !card.Dirty() || card.Invalid() || !card.CanSave() {
		t.Fatalf("合法模型草稿必须可保存：%s", stageComment(card))
	}
	if len(card.Plan.Sections) != 1 || card.Plan.Sections[0].NS != settingsNSSubagentModelSelection {
		t.Fatalf("模型变更必须先写自己的 namespace，实际 %+v", card.Plan.Sections)
	}
	ops := card.Plan.Sections[0].Ops
	if len(ops) != 2 || ops[0].Path[0] != settingsFieldSubagentModelEnabled || ops[1].Path[0] != settingsFieldAllowedModels {
		t.Fatalf("必须同一次 mutate 写 enabled + allowedModels，实际 %+v", ops)
	}
	routeValues, ok := ops[1].Value.([]settingsSubagentModelRoute)
	if !ok || len(routeValues) != 2 {
		t.Fatalf("allowedModels 必须是对象数组而不是字符串，实际 %#v", ops[1].Value)
	}
	if baseline[settingsNSSubagentModelSelection] != 11 {
		t.Fatalf("baseline=%v，want revision 11", baseline)
	}
	// 再次点同一路由 = 删除，顺序不同不影响 sameRoutes 判等。
	routes, _ = toggleSubagentModel(routes, base.Model.Candidates, key)
	if !sameSettingsSubagentModelRoutes(routes, base.Model.CurrentRoutes) {
		t.Fatal("取消新增路由后应回到原授权集合")
	}
}

func stageComment(card settingsItemCardState) string {
	return fmt.Sprintf("dirty=%v invalid=%v writable=%v saving=%v", card.Dirty(), card.Invalid(), card.Writable, card.Saving)
}

func TestSettingsSubagentModelRequiredAndRevisionConflictBlockSave(t *testing.T) {
	describe := settingsItemDescribeFixture()
	enableSettingsSubagentFixture(t, describe)
	card := settingsSubagentCardWithFallback(t, describe, settingsItemFormSnapshot{drafts: map[string]settingsItemDraft{}})
	form := (&Plugin{}).ensureSettingsItemForm()
	form.stageSubagentModel(card.Model, true, []settingsSubagentModelRoute{})
	drafts, baseline, _ := form.snapshot()
	fallback := settingsItemFormSnapshot{drafts: drafts, baseline: baseline, catalogWired: true, catalog: settingsSubagentCatalogFixture()}
	card = settingsSubagentCardWithFallback(t, describe, fallback)
	if !card.Model.Invalid || !card.Invalid() || card.CanSave() {
		t.Fatalf("enabled 且空授权必须 invalid 并禁止保存：%+v", card.Model)
	}
	// 有合法草稿后，describe revision 前进必须显示官方 conflict 并禁止覆盖新文档。
	changed := append(cloneSettingsSubagentModelRoutes(card.Model.CurrentRoutes), settingsSubagentModelRoute{Provider: "provider-a", Model: "model-a1"})
	form.stageSubagentModel(card.Model, true, changed)
	drafts, baseline, _ = form.snapshot()
	fallback.drafts, fallback.baseline = drafts, baseline
	for index := range describe.Namespaces {
		if describe.Namespaces[index].NS == settingsNSSubagentModelSelection {
			describe.Namespaces[index].Revision = 12
		}
	}
	for index := range describe.Sections {
		if describe.Sections[index].NS == settingsNSSubagentModelSelection {
			describe.Sections[index].Revision = 12
		}
	}
	card = settingsSubagentCardWithFallback(t, describe, fallback)
	if !card.Model.Conflicted || !card.Invalid() || card.CanSave() {
		t.Fatalf("revision 变化必须 conflict 并禁止保存：%+v", card.Model)
	}
}

func TestSettingsSubagentModelRevisionConvergedDraftIsDiscarded(t *testing.T) {
	describe := settingsItemDescribeFixture()
	enableSettingsSubagentFixture(t, describe)
	base := settingsSubagentCardWithFallback(t, describe, settingsItemFormSnapshot{drafts: map[string]settingsItemDraft{}})
	form := (&Plugin{}).ensureSettingsItemForm()
	form.stageSubagentModel(base.Model, true, base.Model.CurrentRoutes)
	for index := range describe.Sections {
		if describe.Sections[index].NS == settingsNSSubagentModelSelection {
			describe.Sections[index].Revision = 12
		}
	}
	form.reconcileSubagentModel(describe)
	drafts, baseline, _ := form.snapshot()
	if len(drafts) != 0 || len(baseline) != 0 {
		t.Fatalf("Host 已收敛到草稿值时必须清草稿与旧 baseline：drafts=%v baseline=%v", drafts, baseline)
	}
}

func TestSettingsSubagentModelCatalogStatesAreDistinct(t *testing.T) {
	describe := settingsItemDescribeFixture()
	enableSettingsSubagentFixture(t, describe)
	cases := []struct {
		name     string
		snapshot *apiclient.ModelsPageSnapshot
		wired    bool
		want     settingsSubagentCatalogStatus
		partial  bool
	}{
		{name: "loading", snapshot: &apiclient.ModelsPageSnapshot{Loading: true}, wired: true, want: settingsSubagentCatalogLoading},
		{name: "error", snapshot: &apiclient.ModelsPageSnapshot{CatalogError: "501 not_implemented"}, wired: true, want: settingsSubagentCatalogError},
		{name: "not wired", snapshot: nil, wired: false, want: settingsSubagentCatalogError},
		{name: "empty", snapshot: &apiclient.ModelsPageSnapshot{Catalog: &apiclient.ModelCatalog{}}, wired: true, want: settingsSubagentCatalogReady},
		{name: "partial", snapshot: &apiclient.ModelsPageSnapshot{Catalog: &apiclient.ModelCatalog{Failures: []apiclient.ModelFailure{{ID: "provider-b"}}}}, wired: true, want: settingsSubagentCatalogReady, partial: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card := settingsSubagentCardWithFallback(t, describe, settingsItemFormSnapshot{
				drafts: map[string]settingsItemDraft{}, catalog: tc.snapshot, catalogWired: tc.wired,
			})
			if card.Model.CatalogStatus != tc.want || card.Model.CatalogPartial != tc.partial {
				t.Fatalf("status=%q partial=%v，want %q/%v", card.Model.CatalogStatus, card.Model.CatalogPartial, tc.want, tc.partial)
			}
		})
	}
}

func TestSettingsSubagentModelDisabledDoesNotRequestCatalog(t *testing.T) {
	describe := settingsItemDescribeFixture()
	if settingsSubagentModelWantsCatalog(describe, map[string]settingsItemDraft{}) {
		t.Fatal("官方默认关闭时不应加载目录")
	}
	enableSettingsSubagentFixture(t, describe)
	if !settingsSubagentModelWantsCatalog(describe, map[string]settingsItemDraft{}) {
		t.Fatal("enabled=true 必须加载目录")
	}
}

func TestSettingsSubagentModelMalformedValueDoesNotBecomeEmptyAuthorization(t *testing.T) {
	describe := settingsItemDescribeFixture()
	for index := range describe.Namespaces {
		if describe.Namespaces[index].NS == settingsNSSubagentModelSelection {
			describe.Namespaces[index].Value = json.RawMessage(`{"enabled":`)
		}
	}
	card := settingsSubagentCardWithFallback(t, describe, settingsItemFormSnapshot{drafts: map[string]settingsItemDraft{}})
	if card.Model.Known || card.Model.Dirty || len(card.Model.DesiredRoutes) != 0 || card.CanSave() {
		t.Fatalf("坏 Value 不得伪造为可写空配置：%+v", card.Model)
	}
}

func TestSettingsSubagentModelGeometryMatchesOfficialCSS(t *testing.T) {
	rows := settingsSubagentModelGeometry()
	if len(rows) < 20 {
		t.Fatalf("模型选择几何台账只有 %d 行", len(rows))
	}
	for _, row := range rows {
		if row.Anchor == "" || row.Official != row.Impl {
			t.Errorf("%s.%s official=%v impl=%v anchor=%q", row.Node, row.Prop, row.Official, row.Impl, row.Anchor)
		}
	}
	if subagentModelsMaxH != 280 || subagentModelPad != 6 || subagentPermissionPadY != 12 {
		t.Fatal("模型列表 280 / 行内缩 6 / permission 12 是官方 CSS 写死的值")
	}
}
