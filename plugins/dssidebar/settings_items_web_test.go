// settings_items_web_test.go —— #47「网页搜索与获取」的数据模型证据。
//
// 每条断言都对着官方原文（`OFF:` 行号在 settings_items_web.go 文件头）或官方编译产物里
// `SettingsFormModel` 的判据；**不测"大概能用"**：
//
//   - 页面存在条件 = `whileServed([web])`（#47 注册点）
//   - 菜单项顺序 = MENU_ORDER + 宿主顺序 + 缺失的已选 id
//   - 面板出现条件 = 选中 provider 的 kind（deepseek / legacy / local / plugin / none）
//   - 计费的"已被 … 覆盖"锁 = 两个端点字段非空
//   - 三个 form 的 plan：api → legacy → selection，密钥在 selection 之后
//   - 密钥草稿的引用不变量：引用变了就绝不写
package dssidebar

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// settingsWebFieldView 造一条 section 字段投影。
func settingsWebFieldView(name, typ string, current, def any, overridden bool) apiclient.SettingsFieldView {
	return apiclient.SettingsFieldView{
		Name: name, Path: []string{name}, Type: typ,
		Current: current, Default: def, Overridden: overridden, Writable: true,
	}
}

// settingsWebDescribeFixture 是三条 web 命名空间都在、且值正常的 describe。
func settingsWebDescribeFixture() *apiclient.SettingsDescribeResult {
	describe := &apiclient.SettingsDescribeResult{Writable: true, HasDocument: true}
	add := func(ns string, revision int, base, value map[string]any, writable bool, fields ...apiclient.SettingsFieldView) {
		raw := func(v map[string]any) json.RawMessage {
			if v == nil {
				return nil
			}
			encoded, err := json.Marshal(v)
			if err != nil {
				panic(err)
			}
			return encoded
		}
		describe.Namespaces = append(describe.Namespaces, apiclient.SettingsNamespaceView{
			NS: ns, Revision: revision, Base: raw(base), Value: raw(value),
		})
		describe.Sections = append(describe.Sections, apiclient.SettingsSectionView{
			NS: ns, Writable: writable, Revision: revision, Fields: fields,
		})
	}
	// `web`：官方无默认值（base 里一个键都不写）。
	add(settingsNSWeb, 4, map[string]any{}, map[string]any{
		settingsFieldSearchProvider: settingsWebProviderAPISearch,
		settingsFieldFetchProvider:  settingsWebProviderLocalFetch,
	}, true,
		settingsWebFieldView(settingsFieldSearchProvider, "string", settingsWebProviderAPISearch, nil, false),
		settingsWebFieldView(settingsFieldFetchProvider, "string", settingsWebProviderLocalFetch, nil, false),
	)
	// `web-deepseek-api`：count + 两个 billing + 四个只读端点/引用字段。
	add(settingsNSWebAPI, 6,
		map[string]any{settingsFieldWebCount: float64(10), settingsFieldSearchBilling: "auto", settingsFieldFetchBilling: "auto"},
		map[string]any{settingsFieldWebCount: float64(10), settingsFieldSearchBilling: "auto", settingsFieldFetchBilling: "auto"},
		true,
		settingsWebFieldView(settingsFieldWebCount, "integer", float64(10), float64(10), false),
		settingsWebFieldView(settingsFieldSearchBilling, "string", "auto", "auto", false),
		settingsWebFieldView(settingsFieldFetchBilling, "string", "auto", "auto", false),
		settingsWebFieldView(settingsFieldSearchBaseURL, "string", "", nil, false),
		settingsWebFieldView(settingsFieldSearchKeyEnv, "string", "", nil, false),
		settingsWebFieldView(settingsFieldFetchBaseURL, "string", "", nil, false),
		settingsWebFieldView(settingsFieldFetchKeyEnv, "string", "", nil, false),
	)
	// `web-search-deepseek`：密钥引用 + 端点 + 最大次数。
	add(settingsNSWebSearch, 8,
		map[string]any{settingsFieldLegacyKeyEnv: "DEEPSEEK_API_KEY", settingsFieldLegacyMaxUses: float64(5)},
		map[string]any{settingsFieldLegacyKeyEnv: "DEEPSEEK_API_KEY", settingsFieldLegacyMaxUses: float64(5)},
		true,
		settingsWebFieldView(settingsFieldLegacyKeyEnv, "string", "DEEPSEEK_API_KEY", "DEEPSEEK_API_KEY", false),
		settingsWebFieldView(settingsFieldLegacyBaseURL, "string", "", nil, false),
		settingsWebFieldView(settingsFieldLegacyMaxUses, "integer", float64(5), float64(5), false),
	)
	return describe
}

// settingsWebMergeDescribe 把两份 describe 合成一份（四条命名空间一起服务的场景）。
func settingsWebMergeDescribe(base, extra *apiclient.SettingsDescribeResult) *apiclient.SettingsDescribeResult {
	merged := &apiclient.SettingsDescribeResult{Writable: true, HasDocument: true}
	merged.Namespaces = append(append(merged.Namespaces, base.Namespaces...), extra.Namespaces...)
	merged.Sections = append(append(merged.Sections, base.Sections...), extra.Sections...)
	return merged
}

// settingsWebProviderFixture 是宿主 `web.listProviders` 的一份真实形状（含一个插件 provider）。
func settingsWebProviderFixture() *apiclient.WebProviderList {
	return &apiclient.WebProviderList{
		Search: []apiclient.WebProviderInfo{
			{ID: settingsWebProviderAPISearch},
			{ID: settingsWebProviderLegacySearch},
			{ID: "searxng", Title: "SearXNG"},
		},
		Fetch: []apiclient.WebProviderInfo{
			{ID: settingsWebProviderLocalFetch},
			{ID: settingsWebProviderAPIFetch},
		},
	}
}

// settingsWebSnapshot 是一帧的输入快照（provider 清单 + 凭据答案 + 草稿）。
func settingsWebSnapshot(drafts map[string]settingsItemDraft) settingsItemFormSnapshot {
	if drafts == nil {
		drafts = map[string]settingsItemDraft{}
	}
	return settingsItemFormSnapshot{
		drafts:        drafts,
		baseline:      map[string]int{},
		providers:     settingsWebProviderFixture(),
		providerWired: true,
		credentials: map[string]apiclient.CredentialInfo{
			"DEEPSEEK_API_KEY": {Configured: true, Source: "env", Writable: true},
			"MY_KEY":           {Configured: false, Writable: true},
		},
		credentialWired: true,
	}
}

// settingsWebCardOf 取 #47 卡片的状态。
func settingsWebCardOf(t *testing.T, describe *apiclient.SettingsDescribeResult,
	snap settingsItemFormSnapshot) settingsItemCardState {
	t.Helper()
	for _, card := range settingsItemCards(describe, snap) {
		if card.Item.ID == "web-search" {
			if card.Web == nil {
				t.Fatal("web-search 卡片的 Web 状态是 nil")
			}
			return card
		}
	}
	t.Fatalf("#47 卡片不存在（宿主服务了 web 时应存在）")
	return settingsItemCardState{}
}

// TestSettingsWebCardOnlyWhileWebServed 钉住 `whileServed([WEB_NS])`。
func TestSettingsWebCardOnlyWhileWebServed(t *testing.T) {
	// 四条命名空间（上批三条 + 本批 web）都服务时：四张卡，顺序按官方 order。
	describe := settingsWebMergeDescribe(settingsItemDescribeFixture(), settingsWebDescribeFixture())
	ids := settingsItemCardIDs(describe)
	want := []string{"subagent", "web-search", "agent-loop", "shell"}
	if len(ids) != len(want) {
		t.Fatalf("四张卡的 id = %v（want %v）", ids, want)
	}
	for index := range want {
		if ids[index] != want[index] {
			t.Fatalf("卡片顺序 = %v（want 官方 order %v）", ids, want)
		}
	}
	// 顺序按官方 order：subagent(10) → web-search(20) → agent-loop(30) → shell(40)。
	order := settingsItemLedger()
	positions := map[string]int{}
	for index, item := range order {
		positions[item.ID] = index
	}
	if positions["web-search"] <= positions["subagent"] || positions["web-search"] >= positions["agent-loop"] {
		t.Fatalf("web-search 的 order 位置不对：%v", positions)
	}
	_ = want

	// 只服务 web：只剩 #47（另外三张整张不存在）。
	describe = settingsWebDescribeFixture()
	only := &apiclient.SettingsDescribeResult{Writable: true}
	for _, ns := range describe.Namespaces {
		if ns.NS == settingsNSWeb {
			only.Namespaces = append(only.Namespaces, ns)
		}
	}
	for _, section := range describe.Sections {
		if section.NS == settingsNSWeb {
			only.Sections = append(only.Sections, section)
		}
	}
	if got := settingsItemCardIDs(only); len(got) != 1 || got[0] != "web-search" {
		t.Fatalf("只服务 web 时应当只剩 #47，实际 %v", got)
	}
	// web 没服务：这张卡整张不存在（不是灰的）。
	none := &apiclient.SettingsDescribeResult{Writable: true}
	for _, ns := range describe.Namespaces {
		if ns.NS != settingsNSWeb {
			none.Namespaces = append(none.Namespaces, ns)
		}
	}
	for _, section := range describe.Sections {
		if section.NS != settingsNSWeb {
			none.Sections = append(none.Sections, section)
		}
	}
	for _, id := range settingsItemCardIDs(none) {
		if id == "web-search" {
			t.Fatal("宿主没服务 web 时 #47 不该出现（官方 whileServed）")
		}
	}
}

// TestSettingsWebProviderMenuOrder 钉住官方 `options()` 的顺序与"缺失项"。
func TestSettingsWebProviderMenuOrder(t *testing.T) {
	options := settingsWebProviderOptions(settingsWebProviderFixture(), "search", settingsWebProviderAPISearch)
	if len(options) != 3 {
		t.Fatalf("搜索菜单项 = %d 个（%v），want 3（两个官方 + 插件）", len(options), options)
	}
	// 官方 id 按 MENU_ORDER 在前，其余按宿主顺序跟在后面。
	if options[0].ID != settingsWebProviderAPISearch || options[1].ID != settingsWebProviderLegacySearch {
		t.Fatalf("官方 provider 顺序 = %q,%q（want deepseek-search-api, deepseek-official）", options[0].ID, options[1].ID)
	}
	if options[2].ID != "searxng" || options[2].Title != "SearXNG" {
		t.Fatalf("插件 provider = %+v（want 排在官方之后并带注册 title）", options[2])
	}
	// 选中的 id 宿主没注册 → 补一条 missing（官方 `options.push({missing:true})`）。
	options = settingsWebProviderOptions(settingsWebProviderFixture(), "search", "gone-provider")
	last := options[len(options)-1]
	if last.ID != "gone-provider" || !last.Missing {
		t.Fatalf("缺失的已选 id 必须补成 missing 项，实际 %+v", last)
	}
	if label := settingsWebProviderLabel(last); !strings.Contains(label, "未加载") {
		t.Fatalf("missing 项的显示名 = %q（want 含官方「（未加载）」）", label)
	}
	// 清单还没到：只列选中的那个 id（官方 `options()` 的第一个分支）。
	options = settingsWebProviderOptions(nil, "search", "deepseek-official")
	if len(options) != 1 || options[0].ID != "deepseek-official" || options[0].Missing {
		t.Fatalf("清单未到时菜单 = %+v（want 只有已选 id 且不标 missing）", options)
	}
	if options := settingsWebProviderOptions(nil, "search", ""); options != nil {
		t.Fatalf("清单未到且没选任何 provider 时菜单应为空，实际 %+v", options)
	}
}

// TestSettingsWebKinds 钉住官方 `searchKind()` / `fetchKind()` 的分支。
func TestSettingsWebKinds(t *testing.T) {
	providers := settingsWebProviderFixture()
	cases := []struct {
		name       string
		capability string
		selected   string
		api        bool
		legacy     bool
		providers  *apiclient.WebProviderList
		want       string
	}{
		{"搜索：DeepSeek Search API 且 api 被服务", "search", settingsWebProviderAPISearch, true, true, providers, settingsWebKindDeepSeek},
		{"搜索：api 没被服务 → none", "search", settingsWebProviderAPISearch, false, true, providers, settingsWebKindNone},
		{"搜索：旧版且 legacy 被服务", "search", settingsWebProviderLegacySearch, true, true, providers, settingsWebKindLegacy},
		{"搜索：旧版但 legacy 没服务 → none", "search", settingsWebProviderLegacySearch, true, false, providers, settingsWebKindNone},
		{"搜索：插件 provider", "search", "searxng", true, true, providers, settingsWebKindPlugin},
		{"搜索：没选 → none", "search", "", true, true, providers, settingsWebKindNone},
		{"搜索：选了宿主没注册的 → none", "search", "gone", true, true, providers, settingsWebKindNone},
		{"获取：本机直接获取", "fetch", settingsWebProviderLocalFetch, true, true, providers, settingsWebKindLocal},
		{"获取：DeepSeek Fetch API", "fetch", settingsWebProviderAPIFetch, true, true, providers, settingsWebKindDeepSeek},
		{"获取：清单未到且 id 未知 → none", "fetch", "gone", true, true, nil, settingsWebKindNone},
	}
	for _, tc := range cases {
		if got := settingsWebSectionKind(tc.capability, tc.selected, tc.providers, tc.api, tc.legacy); got != tc.want {
			t.Errorf("%s：kind = %q，want %q", tc.name, got, tc.want)
		}
	}
}

// TestSettingsWebPanelsMatchOfficialJSX 钉住三个面板的组成（官方 JSX 顺序）。
func TestSettingsWebPanelsMatchOfficialJSX(t *testing.T) {
	describe := settingsWebDescribeFixture()

	// 默认：搜索 = DeepSeek Search API（count + billing），获取 = 本机直接获取（无面板）。
	card := settingsWebCardOf(t, describe, settingsWebSnapshot(nil))
	if card.Web.Search.Kind != settingsWebKindDeepSeek {
		t.Fatalf("搜索 kind = %q，want deepseek", card.Web.Search.Kind)
	}
	if len(card.Web.Search.Panel) != 2 {
		t.Fatalf("DeepSeek Search API 面板 = %d 格（want count + billing），实际 %+v", len(card.Web.Search.Panel), card.Web.Search.Panel)
	}
	if card.Web.Search.Panel[0].Kind != settingsWebEntryField || card.Web.Search.Panel[0].Field.Spec.Name != settingsFieldWebCount {
		t.Fatalf("面板第一格 = %+v（want count 输入框）", card.Web.Search.Panel[0])
	}
	if card.Web.Search.Panel[1].Kind != settingsWebEntryBilling {
		t.Fatalf("面板第二格 = %+v（want billing 菜单）", card.Web.Search.Panel[1])
	}
	if card.Web.Fetch.Kind != settingsWebKindLocal || len(card.Web.Fetch.Panel) != 0 {
		t.Fatalf("本机直接获取不该有面板，实际 kind=%q panel=%+v", card.Web.Fetch.Kind, card.Web.Fetch.Panel)
	}
	if card.Web.SearchBilling == nil || card.Web.SearchBilling.Value != "auto" {
		t.Fatalf("搜索计费 = %+v（want 默认 auto）", card.Web.SearchBilling)
	}

	// 搜索切到旧版：面板 = 密钥 + Base URL + 最大搜索次数（官方 WebSearchCard.tsx:91-114）。
	describe = settingsWebDescribeFixture()
	describe.Namespaces[0].Value = json.RawMessage(`{"searchProvider":"deepseek-official","fetchProvider":"http"}`)
	describe.Sections[0].Fields[0].Current = settingsWebProviderLegacySearch
	card = settingsWebCardOf(t, describe, settingsWebSnapshot(nil))
	if card.Web.Search.Kind != settingsWebKindLegacy {
		t.Fatalf("搜索 kind = %q，want legacy", card.Web.Search.Kind)
	}
	if len(card.Web.Search.Panel) != 3 {
		t.Fatalf("旧版面板 = %d 格（want 密钥 + Base URL + maxUses），实际 %+v", len(card.Web.Search.Panel), card.Web.Search.Panel)
	}
	if card.Web.Search.Panel[0].Kind != settingsWebEntrySecret {
		t.Fatalf("旧版面板第一格 = %+v（want 密钥控件）", card.Web.Search.Panel[0])
	}
	if card.Web.Search.Panel[1].Field.Spec.Name != settingsFieldLegacyBaseURL ||
		card.Web.Search.Panel[2].Field.Spec.Name != settingsFieldLegacyMaxUses {
		t.Fatalf("旧版面板字段顺序 = %+v", card.Web.Search.Panel)
	}
	if card.Web.Key == nil || card.Web.Key.Ref != "DEEPSEEK_API_KEY" {
		t.Fatalf("密钥控件 = %+v（want ref=DEEPSEEK_API_KEY）", card.Web.Key)
	}
	if !card.Web.Key.Configured || !card.Web.Key.Known || card.Web.Key.StateLabel != zhWebAPIKeySet {
		t.Fatalf("凭据域报告已配置 → 控件必须说「已配置密钥。」，实际 %+v", card.Web.Key)
	}

	// 获取切到 DeepSeek Fetch API：面板只有 billing。
	describe = settingsWebDescribeFixture()
	describe.Namespaces[0].Value = json.RawMessage(`{"searchProvider":"deepseek-search-api","fetchProvider":"deepseek-fetch-api"}`)
	describe.Sections[0].Fields[1].Current = settingsWebProviderAPIFetch
	card = settingsWebCardOf(t, describe, settingsWebSnapshot(nil))
	if card.Web.Fetch.Kind != settingsWebKindDeepSeek {
		t.Fatalf("获取 kind = %q（want deepseek）", card.Web.Fetch.Kind)
	}
	if len(card.Web.Fetch.Panel) != 1 || card.Web.Fetch.Panel[0].Kind != settingsWebEntryBilling {
		t.Fatalf("DeepSeek Fetch API 面板 = %+v（want 只有 billing）", card.Web.Fetch.Panel)
	}
	if card.Web.FetchBilling == nil {
		t.Fatal("Fetch 计费状态不能为 nil")
	}
}

// TestSettingsWebBillingAdvancedOverrideLocks 钉住「已被 … 覆盖」的判据与顺序。
func TestSettingsWebBillingAdvancedOverrideLocks(t *testing.T) {
	describe := settingsWebDescribeFixture()
	// 两个端点字段都非空 → 标签列出两个字段（baseURL 在前，官方 controller:147-148）。
	for index, field := range describe.Sections[1].Fields {
		if field.Name == settingsFieldSearchBaseURL {
			describe.Sections[1].Fields[index].Current = "https://example.test/v1"
		}
		if field.Name == settingsFieldSearchKeyEnv {
			describe.Sections[1].Fields[index].Current = "MY_KEY"
		}
	}
	card := settingsWebCardOf(t, describe, settingsWebSnapshot(nil))
	billing := card.Web.SearchBilling
	if billing == nil || len(billing.AdvancedOverride) != 2 {
		t.Fatalf("计费高级覆盖 = %+v（want 两个字段）", billing)
	}
	if billing.AdvancedOverride[0] != settingsFieldSearchBaseURL || billing.AdvancedOverride[1] != settingsFieldSearchKeyEnv {
		t.Fatalf("覆盖字段顺序 = %v（want baseURL 在前）", billing.AdvancedOverride)
	}
	// 只有 fetch 的端点非空时，搜索的计费不受影响。
	describe = settingsWebDescribeFixture()
	for index, field := range describe.Sections[1].Fields {
		if field.Name == settingsFieldFetchBaseURL {
			describe.Sections[1].Fields[index].Current = "https://example.test/v1"
		}
	}
	card = settingsWebCardOf(t, describe, settingsWebSnapshot(nil))
	if len(card.Web.SearchBilling.AdvancedOverride) != 0 {
		t.Fatalf("搜索计费被 fetch 的端点误锁：%v", card.Web.SearchBilling.AdvancedOverride)
	}
}

// TestSettingsWebAutoSelectionFollowsComposedBase 钉住 `auto = base?.[field] === undefined`。
func TestSettingsWebAutoSelectionFollowsComposedBase(t *testing.T) {
	describe := settingsWebDescribeFixture()
	card := settingsWebCardOf(t, describe, settingsWebSnapshot(nil))
	if !card.Web.Search.Auto || !card.Web.Fetch.Auto {
		t.Fatal("base 里没钉 provider 时两个菜单都必须有「自动选择」项")
	}
	// base 钉死了 → 菜单里没有自动项（官方 `base?.searchProvider === undefined`）。
	describe.Namespaces[0].Base = json.RawMessage(`{"searchProvider":"deepseek-search-api"}`)
	card = settingsWebCardOf(t, describe, settingsWebSnapshot(nil))
	if card.Web.Search.Auto {
		t.Fatal("base 钉死 searchProvider 时菜单不该有「自动选择」")
	}
	if !card.Web.Fetch.Auto {
		t.Fatal("只钉死搜索时，获取仍应有「自动选择」")
	}
	if card.Web.searchComposed != "deepseek-search-api" || !card.Web.searchComposedSet {
		t.Fatalf("组合值 = %q set=%v", card.Web.searchComposed, card.Web.searchComposedSet)
	}
}

// TestSettingsWebCountField 钉住官方 count 的 parse（整数 1..50，空白 clear）。
func TestSettingsWebCountField(t *testing.T) {
	spec := settingsWebCountSpec()
	cases := []struct {
		text  string
		ok    bool
		clear bool
		want  float64
	}{
		{"", true, true, 0},
		{"   ", true, true, 0},
		{"1", true, false, 1},
		{"50", true, false, 50},
		{"0", false, false, 0},
		{"51", false, false, 0},
		{"1.5", false, false, 0},
		{"abc", false, false, 0},
	}
	for _, tc := range cases {
		value, clear, ok := parseItemField(spec, tc.text)
		if ok != tc.ok || clear != tc.clear {
			t.Errorf("count parse(%q) = (%v,%v,%v)，want ok=%v clear=%v", tc.text, value, clear, ok, tc.ok, tc.clear)
			continue
		}
		if ok && !clear && value != tc.want {
			t.Errorf("count parse(%q) = %v，want %v", tc.text, value, tc.want)
		}
	}
	// maxUses 同样整数、下限 1（官方 `.min(1)`），但没有上限。
	maxUses := settingsWebMaxUsesSpec()
	if _, _, ok := parseItemField(maxUses, "0"); ok {
		t.Fatal("maxUses 的 0 必须 rejected（.min(1)）")
	}
	if value, _, ok := parseItemField(maxUses, "9"); !ok || value != 9 {
		t.Fatal("maxUses 的 9 应通过")
	}
}

// TestSettingsWebTextField 钉住官方 `settingsTextField` 的 format/parse。
func TestSettingsWebTextField(t *testing.T) {
	if got := formatItemText(nil); got != "" {
		t.Fatalf("formatItemText(nil) = %q（want 空串，官方 typeof !== 'string'）", got)
	}
	if got := formatItemText(float64(3)); got != "" {
		t.Fatalf("formatItemText(3) = %q（want 空串——数字不是文本字段的值）", got)
	}
	if got := formatItemText("https://x"); got != "https://x" {
		t.Fatalf("formatItemText = %q", got)
	}
	value, clear, ok := parseItemText("  https://api.test/v1  ")
	if !ok || clear || value != "https://api.test/v1" {
		t.Fatalf("parseItemText 必须 trim 后 set，实际 (%q,%v,%v)", value, clear, ok)
	}
	if _, clear, ok = parseItemText("   "); !ok || !clear {
		t.Fatal("空白文本必须 clear（官方 {kind:'clear'}）")
	}
	// 文本字段"没有值"不是"未知"：baseURL 空着也应可编辑（官方 format(undefined)===''）。
	describe := settingsWebDescribeFixture()
	card := settingsWebCardOf(t, describe, settingsWebSnapshot(nil))
	baseURL := card.Web.BaseURL
	if !baseURL.Known || baseURL.Disabled || baseURL.Text != "" {
		t.Fatalf("baseURL 投影 = %+v（want 已知、可编辑、空文本）", baseURL)
	}
}

// TestSettingsWebPlanOrderMatchesOfficialSave 钉住 plan 的三段与官方保存顺序。
func TestSettingsWebPlanOrderMatchesOfficialSave(t *testing.T) {
	describe := settingsWebDescribeFixture()
	drafts := map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSWebAPI, settingsFieldWebCount):         {Text: "25"},
		settingsItemDraftKey(settingsNSWebAPI, settingsFieldFetchBilling):     {Text: "api-key"},
		settingsItemDraftKey(settingsNSWebSearch, settingsFieldLegacyMaxUses): {Text: "9"},
		settingsItemDraftKey(settingsNSWeb, settingsFieldSearchProvider):      {Text: "searxng"},
	}
	card := settingsWebCardOf(t, describe, settingsWebSnapshot(drafts))
	plan := card.Web.plan
	if !plan.Dirty() || plan.Invalid {
		t.Fatalf("plan dirty=%v invalid=%v（want dirty 且不 invalid）", plan.Dirty(), plan.Invalid)
	}
	if len(plan.API) != 2 {
		t.Fatalf("api 段 = %+v（want count + fetchBilling 两条）", plan.API)
	}
	if len(plan.Legacy) != 1 || plan.Legacy[0].Path[0] != settingsFieldLegacyMaxUses {
		t.Fatalf("legacy 段 = %+v（want maxUses）", plan.Legacy)
	}
	if len(plan.Selection) != 1 || plan.Selection[0].Path[0] != settingsFieldSearchProvider {
		t.Fatalf("selection 段 = %+v（want searchProvider）", plan.Selection)
	}
	if len(plan.API)+len(plan.Legacy)+len(plan.Selection) != plan.TotalOps() {
		t.Fatal("TotalOps 与三段之和不一致")
	}
	// 值是**数字/字符串原文**（类型不许漂）。
	if value, ok := plan.API[0].Value.(float64); !ok || value != 25 {
		t.Fatalf("count 写入值 = %#v（want 数字 25）", plan.API[0].Value)
	}
	if value, ok := plan.API[1].Value.(string); !ok || value != "api-key" {
		t.Fatalf("billing 写入值 = %#v（want 字符串 api-key）", plan.API[1].Value)
	}
	if value, ok := plan.Selection[0].Value.(string); !ok || value != "searxng" {
		t.Fatalf("provider 写入值 = %#v（want 字符串 searxng）", plan.Selection[0].Value)
	}

	// 坏输入 → invalid，且 save 按钮禁用（官方 `invalid` 时不动）。
	drafts = map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSWebAPI, settingsFieldWebCount): {Text: "0"},
	}
	card = settingsWebCardOf(t, describe, settingsWebSnapshot(drafts))
	if !card.Invalid() || card.CanSave() {
		t.Fatalf("坏 count 必须 invalid 且禁用保存，实际 invalid=%v canSave=%v", card.Invalid(), card.CanSave())
	}
	if !card.Web.selectionBlocked {
		t.Fatal("invalid 时 provider 菜单必须锁死（官方 selectionBlocked）")
	}
}

// TestSettingsWebProviderUnsetWhenEqualsComposed 钉住"选中组合值 = 清覆盖"。
func TestSettingsWebProviderUnsetWhenEqualsComposed(t *testing.T) {
	describe := settingsWebDescribeFixture()
	// base 钉死 deepseek-search-api，user 层写过别家（overridden）。
	describe.Namespaces[0].Base = json.RawMessage(`{"searchProvider":"deepseek-search-api","fetchProvider":"http"}`)
	form := &settingsItemForm{drafts: map[string]settingsItemDraft{}, baseline: map[string]int{}}
	// 选回组合值 → 草稿是 clear（发 unset），不是存一份同样的副本。
	form.settingsWebStageSelection(settingsFieldSearchProvider, "deepseek-search-api", "deepseek-search-api", true, 4)
	draft := form.drafts[settingsItemDraftKey(settingsNSWeb, settingsFieldSearchProvider)]
	if !draft.Clear {
		t.Fatalf("选回组合值时草稿必须 clear，实际 %+v", draft)
	}
	// 选「自动选择」（空 id）→ 也是 clear。
	form.settingsWebStageSelection(settingsFieldFetchProvider, "", "http", true, 4)
	if draft := form.drafts[settingsItemDraftKey(settingsNSWeb, settingsFieldFetchProvider)]; !draft.Clear {
		t.Fatalf("选「自动选择」时草稿必须 clear，实际 %+v", draft)
	}
	// 选别家 → set 文本。
	form.settingsWebStageSelection(settingsFieldSearchProvider, "searxng", "deepseek-search-api", true, 4)
	if draft := form.drafts[settingsItemDraftKey(settingsNSWeb, settingsFieldSearchProvider)]; draft.Clear || draft.Text != "searxng" {
		t.Fatalf("选别家时应 stage searxng，实际 %+v", draft)
	}
	// 计费同理（组合值 = base ?? auto）。
	form.settingsWebStageBilling("search", "auto", "auto", 6)
	if draft := form.drafts[settingsItemDraftKey(settingsNSWebAPI, settingsFieldSearchBilling)]; !draft.Clear {
		t.Fatalf("计费选回组合值必须 clear，实际 %+v", draft)
	}
	form.settingsWebStageBilling("search", "api-key", "auto", 6)
	if draft := form.drafts[settingsItemDraftKey(settingsNSWebAPI, settingsFieldSearchBilling)]; draft.Clear || draft.Text != "api-key" {
		t.Fatalf("计费选 api-key 应 stage，实际 %+v", draft)
	}
}

// TestSettingsWebKeyDraftIsReferenceAnchored 钉住密钥草稿的引用不变量。
func TestSettingsWebKeyDraftIsReferenceAnchored(t *testing.T) {
	describe := settingsWebDescribeFixture()
	// 切到旧版 provider，密钥控件才出现。
	describe.Namespaces[0].Value = json.RawMessage(`{"searchProvider":"deepseek-official","fetchProvider":"http"}`)
	describe.Sections[0].Fields[0].Current = settingsWebProviderLegacySearch

	drafts := map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSWeb, settingsFieldLegacySecret): {Text: "  sk-abc  ", Ref: "DEEPSEEK_API_KEY"},
	}
	card := settingsWebCardOf(t, describe, settingsWebSnapshot(drafts))
	if card.Web.plan.KeyRef != "DEEPSEEK_API_KEY" {
		t.Fatalf("密钥 plan = %q（want DEEPSEEK_API_KEY）", card.Web.plan.KeyRef)
	}
	if card.Web.plan.KeyValue != "sk-abc" {
		t.Fatalf("密钥 plan 值 = %q（want trim 后的 sk-abc）", card.Web.plan.KeyValue)
	}
	if !card.Dirty() {
		t.Fatal("只有密钥草稿时表单也必须是 dirty（官方 plan 含 secret run）")
	}

	// 引用变了（apiKeyEnv 换成 MY_KEY）→ 旧草稿**不写**，也不显示。
	describe = settingsWebDescribeFixture()
	describe.Namespaces[0].Value = json.RawMessage(`{"searchProvider":"deepseek-official","fetchProvider":"http"}`)
	describe.Sections[0].Fields[0].Current = settingsWebProviderLegacySearch
	describe.Sections[2].Fields[0].Current = "MY_KEY"
	card = settingsWebCardOf(t, describe, settingsWebSnapshot(drafts))
	if card.Web.Key.Ref != "MY_KEY" {
		t.Fatalf("引用 = %q（want MY_KEY）", card.Web.Key.Ref)
	}
	if card.Web.plan.KeyRef != "" {
		t.Fatalf("引用变了就不许写（plan.KeyRef=%q）——那会把密钥写到另一个引用名下", card.Web.plan.KeyRef)
	}
	if card.Web.Key.Text != "" {
		t.Fatalf("引用变了草稿必须不再显示，实际 %q", card.Web.Key.Text)
	}

	// 空白草稿不写（官方 `text.trim() !== ''` 才 push run）。
	drafts[settingsItemDraftKey(settingsNSWeb, settingsFieldLegacySecret)] = settingsItemDraft{Text: "   ", Ref: "DEEPSEEK_API_KEY"}
	describe = settingsWebDescribeFixture()
	describe.Namespaces[0].Value = json.RawMessage(`{"searchProvider":"deepseek-official","fetchProvider":"http"}`)
	describe.Sections[0].Fields[0].Current = settingsWebProviderLegacySearch
	card = settingsWebCardOf(t, describe, settingsWebSnapshot(drafts))
	if card.Web.plan.KeyRef != "" {
		t.Fatal("纯空白密钥草稿不许写（官方 trim 后为空就跳过）")
	}
}

// TestSettingsWebKeyHiddenWithoutCredentialDomain 钉住"壳没接凭据域就不画密钥"。
func TestSettingsWebKeyHiddenWithoutCredentialDomain(t *testing.T) {
	describe := settingsWebDescribeFixture()
	describe.Namespaces[0].Value = json.RawMessage(`{"searchProvider":"deepseek-official","fetchProvider":"http"}`)
	describe.Sections[0].Fields[0].Current = settingsWebProviderLegacySearch
	snap := settingsWebSnapshot(nil)
	snap.credentialWired = false
	snap.credentials = nil
	card := settingsWebCardOf(t, describe, snap)
	if card.Web.Key != nil {
		t.Fatalf("壳没接凭据域时不该画密钥控件，实际 %+v", card.Web.Key)
	}
	// **整个灰面板**都依赖 `legacyKey !== undefined`（WebSearchCard.tsx:91）：官方把密钥、
	// Base URL、maxUses 三格放在同一个 `&&` 分支里，所以没有凭据域时三格一起没有——
	// 不是"少画一格剩两格"。
	if len(card.Web.Search.Panel) != 0 {
		t.Fatalf("无凭据域时旧版面板 = %d 格（want 一格都没有），实际 %+v",
			len(card.Web.Search.Panel), card.Web.Search.Panel)
	}

	// 凭据域答"未知"时：不说"未配置"，用本仓自己的中性句子。
	describe = settingsWebDescribeFixture()
	describe.Namespaces[0].Value = json.RawMessage(`{"searchProvider":"deepseek-official","fetchProvider":"http"}`)
	describe.Sections[0].Fields[0].Current = settingsWebProviderLegacySearch
	describe.Sections[2].Fields[0].Current = "UNKNOWN_KEY"
	snap = settingsWebSnapshot(nil)
	card = settingsWebCardOf(t, describe, snap)
	if card.Web.Key == nil {
		t.Fatal("凭据域接了就该画控件")
	}
	if card.Web.Key.Known {
		t.Fatal("没问到的 ref 必须标成未知")
	}
	if card.Web.Key.StateLabel == zhWebAPIKeyUnset {
		t.Fatal("未知不许说「未配置密钥。」——那是另一句话")
	}
	if !strings.Contains(card.Web.Key.StateLabel, "未知") {
		t.Fatalf("未知态文案 = %q", card.Web.Key.StateLabel)
	}
}

// TestSettingsWebProvidersFailedNotice 钉住 provider 清单读失败的那条提示。
func TestSettingsWebProvidersFailedNotice(t *testing.T) {
	describe := settingsWebDescribeFixture()
	snap := settingsWebSnapshot(nil)
	snap.providers = nil // 壳接了能力但还没拉到
	card := settingsWebCardOf(t, describe, snap)
	if !card.Web.providersFailed || !card.Web.Search.ProvidersFailed || !card.Web.Fetch.ProvidersFailed {
		t.Fatal("provider 清单没拉到时两个菜单下都要画「无法读取提供方列表」+ 重试")
	}
	// 壳根本没接这条能力：不冒充"读失败"（那是两个不同的事实）。
	snap = settingsWebSnapshot(nil)
	snap.providerWired = false
	snap.providers = nil
	card = settingsWebCardOf(t, describe, snap)
	if card.Web.providersFailed {
		t.Fatal("壳没接 web.listProviders 时不该说「读失败」")
	}
	if card.Web.providerWired {
		t.Fatal("providerWired 必须为 false")
	}
}

// TestSettingsWebGeometryMatchesOfficialCSS 逐行核对 #47 的几何台账。
func TestSettingsWebGeometryMatchesOfficialCSS(t *testing.T) {
	rows := settingsWebGeometry()
	if len(rows) < 15 {
		t.Fatalf("几何台账只有 %d 行（官方这一页的几何不止这些）", len(rows))
	}
	for _, row := range rows {
		if row.Anchor == "" {
			t.Errorf("%s.%s 没有官方锚点——那就是猜的", row.Node, row.Prop)
		}
		if row.Official != row.Impl {
			t.Errorf("%s.%s 官方=%v 实现=%v（锚点 %s）", row.Node, row.Prop, row.Official, row.Impl, row.Anchor)
		}
	}
	// 几个容易被改坏的数（官方 CSS 原文）单独再钉一遍。
	if webSelectorHeight != 34 || webPanelPadX != 16 || webSectionGap != 32 || webFieldPadTop != 8 {
		t.Fatal("下拉 34 / 面板内缩 16 / section 间距 32 / 下拉格上内缩 8 被改了")
	}
}

// TestSettingsWebQuotedCopyMatchesOfficial 钉住带占位符的官方文案拼接。
func TestSettingsWebQuotedCopyMatchesOfficial(t *testing.T) {
	// 官方 `baseUrlHint({path:'/messages'})`（WebSearchCard.tsx:105）。
	if got := settingsWebBaseURLSpec().Hint; !strings.Contains(got, "/messages") || strings.Contains(got, "{path}") {
		t.Fatalf("Base URL hint = %q（want 已替换 {path} 为 /messages）", got)
	}
	// 官方 `legacyKeyHint({ref})`（WebSearchCard.tsx:94）。
	if got := settingsWebKeyHint("MY_KEY"); !strings.Contains(got, "MY_KEY") || strings.Contains(got, "{ref}") {
		t.Fatalf("密钥 hint = %q（want 已替换 {ref}）", got)
	}
	// 官方 `billingAdvancedOverride({fields})` 用 zh 的「、」连接（WebSearchCard.tsx:198-199）。
	label := strings.ReplaceAll(zhWebBillingOverride, "{fields}",
		strings.Join([]string{settingsFieldSearchBaseURL, settingsFieldSearchKeyEnv}, zhWebBillingSeparator))
	if label != "已被 searchBaseURL、searchApiKeyEnv 覆盖" {
		t.Fatalf("高级覆盖文案 = %q", label)
	}
	// 计费三档的名字与说明逐条对官方。
	if settingsWebBillingName("auto") != "自动" || settingsWebBillingName("account") != "已登录的 DeepSeek 账号" ||
		settingsWebBillingName("api-key") != "DeepSeek API Key" {
		t.Fatal("计费菜单文案与官方 locales.ts:105-109 不一致")
	}
	if !strings.Contains(settingsWebBillingHint("auto"), "跟随当前会话") {
		t.Fatalf("auto 的说明 = %q", settingsWebBillingHint("auto"))
	}
}
