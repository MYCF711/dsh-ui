package dssettings

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// fullDesktopFacts 是「全量桌面 Host、各服务可用、字体项未展开、transcriptView!=verbose」
// 的那一帧——官方台账的「默认 14 行」口径（结构树 §4 开头）。
func fullDesktopFacts() HostFacts {
	return HostFacts{
		SidebarRightTabs:  true,
		SessionLogServed:  true,
		ClientVersion:     "0.2.2-rc.2",
		PermissionCatalog: true,
		TranscriptView:    "standard",
		// 本窗口键位表装了（真机上是 13 条）；设置面板「快捷键」行的真值来源。
		ShortcutCount: 13,
	}
}

// electronDesktopFacts 是「官方 Electron 桌面 + 通道桥」那一帧（15 行）。
func electronDesktopFacts() HostFacts {
	f := fullDesktopFacts()
	f.DesktopProtocolVersion = 1
	f.UpdatesChannels = []string{"nightly", "stable"}
	return f
}

// TestOfficialSectionsAreTheFiveRegisteredOnes 钉住 5 个 settings.section。
//
// 官方 rc.2 全树只有 5 个 `settings.section` 注册点（结构树 §0 第 1 条）；
// `archived-sessions` 只是 navIcon 的图标分支，**不是第六页**。
func TestOfficialSectionsAreTheFiveRegisteredOnes(t *testing.T) {
	facts := fullDesktopFacts()
	facts.CredentialStored = true
	sections := facts.Sections()

	want := []struct {
		id    string
		order int
	}{
		{"account", -10},
		{"general", 0},
		{"models", 10},
		{"plugins", 15},
		{"agent-presets", 20},
	}
	if len(sections) != len(want) {
		t.Fatalf("section 数量 = %d，官方注册点是 %d 个：%+v", len(sections), len(want), sections)
	}
	for i, w := range want {
		if sections[i].ID != w.id {
			t.Fatalf("第 %d 个 section = %q，官方 order 排序应为 %q", i, sections[i].ID, w.id)
		}
		if sections[i].Order != w.order {
			t.Fatalf("%s 的 order = %d，官方是 %d", w.id, sections[i].Order, w.order)
		}
		if sections[i].Anchor == "" {
			t.Fatalf("%s 缺 rc.2 锚点", w.id)
		}
	}
	for _, s := range sections {
		if s.ID == "archived-sessions" {
			t.Fatal("archived-sessions 不是注册的 section（官方只在 navIcon 里有图标分支）")
		}
	}
}

// TestAccountSectionOnlyWhenCredentialStored 钉住 account 的条件注册两态。
func TestAccountSectionOnlyWhenCredentialStored(t *testing.T) {
	base := fullDesktopFacts()
	if _, ok := sectionByID(base.Sections(), "account"); ok {
		t.Fatal("没有 credential-stored 时 account 不该注册")
	}

	withCred := base
	withCred.CredentialStored = true
	if _, ok := sectionByID(withCred.Sections(), "account"); !ok {
		t.Fatal("credential-stored 时 account 必须注册")
	}
}

// TestThirteenGeneralItemsInOfficialOrder 钉住 general item 是 **13 个**（rc.2 D5）。
func TestThirteenGeneralItemsInOfficialOrder(t *testing.T) {
	items := electronDesktopFacts().GeneralItems()
	if len(items) != 13 {
		t.Fatalf("general item = %d 个，rc.2 是 13 个：%+v", len(items), itemIDs(items))
	}
	want := []string{
		"permission", "language", "appearance", "font-size", "transcript-view",
		"developer-tools", "shortcuts", "link-opening", "composer-enter",
		"performance-usage", "session-log-upload-toast", "desktop-update-channel",
		"current-version",
	}
	for i, id := range want {
		if items[i].ID != id {
			t.Fatalf("第 %d 个 item = %q，官方 order 排序应为 %q（全量：%v）", i, items[i].ID, id, itemIDs(items))
		}
	}
	// 每个 item 都必须有 rc.2 锚点（否则就是「凭印象加的」）。
	for _, it := range items {
		if it.Anchor == "" || it.RowAnchor == "" {
			t.Fatalf("%s 缺 rc.2 注册/行锚点", it.ID)
		}
		if it.HostField == "" {
			t.Fatalf("%s 缺宿主字段", it.ID)
		}
	}
}

// TestDesktopUpdateChannelConditionalRegistration 是 rc.2 D5 的**两态**断言。
//
// 官方条件：`dshDesktop.protocolVersion === 1` **且** 桥里带 `updates.channels`
// （ui-settings-general/src/client/index.ts:109-122）。Go GUI 没有该桥 → 不出现。
func TestDesktopUpdateChannelConditionalRegistration(t *testing.T) {
	cases := []struct {
		name  string
		facts HostFacts
		want  bool
	}{
		{"Go GUI（无桌面桥）", fullDesktopFacts(), false},
		{"有 protocolVersion 但没有 channels", HostFacts{DesktopProtocolVersion: 1}, false},
		{"有 channels 但 protocolVersion != 1", HostFacts{UpdatesChannels: []string{"stable"}}, false},
		{"protocolVersion=1 且 channels 非空", electronDesktopFacts(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got := sectionGeneralItem(tc.facts, "desktop-update-channel")
			if got != tc.want {
				t.Fatalf("desktop-update-channel 注册 = %v，期望 %v（facts=%+v）", got, tc.want, tc.facts)
			}
		})
	}

	// 两态各自的行数（官方 14 / 15）。
	if got := fullDesktopFacts().VisibleRowCount(); got != 14 {
		t.Fatalf("Go GUI 默认可见行 = %d，官方是 14", got)
	}
	if got := electronDesktopFacts().VisibleRowCount(); got != 15 {
		t.Fatalf("桌面通道桥下可见行 = %d，官方是 15", got)
	}
}

// TestTranscriptViewRowsCollapseInVerbose 钉住 transcript-view 的三行与 verbose 折叠。
func TestTranscriptViewRowsCollapseInVerbose(t *testing.T) {
	facts := fullDesktopFacts()
	if got := facts.VisibleRowCount(); got != 14 {
		t.Fatalf("standard 模式可见行 = %d，官方是 14", got)
	}
	verbose := facts
	verbose.TranscriptView = "verbose"
	if got := verbose.VisibleRowCount(); got != 13 {
		t.Fatalf("verbose 模式可见行 = %d，官方是 13（第三行「收起后显示的回复」消失）", got)
	}
	if got := facts.ItemRows(mustItem(t, "transcript-view")); got != 3 {
		t.Fatalf("transcript-view 在 standard 下应贡献 3 行，实得 %d", got)
	}
	if got := verbose.ItemRows(mustItem(t, "transcript-view")); got != 2 {
		t.Fatalf("transcript-view 在 verbose 下应贡献 2 行，实得 %d", got)
	}
	// 三行的标题/字段/默认值都要在（官方 TranscriptViewRow.tsx:57-93）。
	if got := len(transcriptViewRowTitles); got != 3 {
		t.Fatalf("transcript-view 行标题 = %d 条", got)
	}
	for i, want := range []string{"ui-chat.transcriptView", "ui-chat.collapseTiming", "ui-chat.foldedReplies"} {
		if transcriptViewRowFields[i] != want {
			t.Fatalf("transcript-view 第 %d 行字段 = %q，官方是 %q", i, transcriptViewRowFields[i], want)
		}
	}
}

// TestItemConditionsHideRowsWhenServicesMissing 钉住「服务缺失 → 整行不注册」。
func TestItemConditionsHideRowsWhenServicesMissing(t *testing.T) {
	bare := HostFacts{}
	for _, id := range []string{"permission", "link-opening", "session-log-upload-toast", "current-version", "desktop-update-channel"} {
		if _, ok := sectionGeneralItem(bare, id); ok {
			t.Fatalf("没有任何服务时 %s 不该注册", id)
		}
	}
	// 固定注册的（ItemAlways）必须在。
	for _, id := range []string{"language", "appearance", "font-size", "transcript-view", "developer-tools", "shortcuts", "composer-enter", "performance-usage"} {
		if _, ok := sectionGeneralItem(bare, id); !ok {
			t.Fatalf("无条件 item %s 必须注册", id)
		}
	}
}

// TestSectionIconDynamicSlotWithFallback 钉住 section 图标是动态 slot + 回退。
func TestSectionIconDynamicSlotWithFallback(t *testing.T) {
	if got := SectionIcon("general", nil); got != IconSettings {
		t.Fatalf("general 的 fallback 图标 = %q，官方是 settings 齿轮", got)
	}
	if got := SectionIcon("models", nil); got != IconData {
		t.Fatalf("models 的 fallback 图标 = %q，官方是 data", got)
	}
	if got := SectionIcon("unknown-third-party", nil); got != IconSettings {
		t.Fatalf("未知 section 的 fallback 图标 = %q，官方是 settings 齿轮", got)
	}
	// 第三方注册同名 slot（id = section id）时必须替换。
	if got := SectionIcon("models", map[string]string{"models": "custom-data"}); got != "custom-data" {
		t.Fatalf("注册了 settings.section.icon 后应替换，实得 %q", got)
	}
	if TriggerIcon() != IconSettings {
		t.Fatalf("启动器图标 = %q", TriggerIcon())
	}
}

// TestOfficialSubSlotsCoverLedger 钉住子 slot 清单（结构树 §2.1 的 ledger）。
func TestOfficialSubSlotsCoverLedger(t *testing.T) {
	want := []string{
		"settings.launcher", "settings.trigger", "settings.header", "settings.close",
		"settings.action", "settings.section", "settings.section.icon",
		"settings.onboarding", "settings.general.item", "settings.plugins.tab",
		"settings.models.provider-card", "settings.models.footer",
		"settings.models.sign-in", "settings.launcher.notifications",
	}
	for _, name := range want {
		slot, ok := LookupSubSlot(name)
		if !ok {
			t.Fatalf("子 slot %q 没登记", name)
		}
		if slot.Kind == "" || slot.Anchor == "" {
			t.Fatalf("子 slot %q 缺 kind/anchor：%+v", name, slot)
		}
	}
	if got := len(SubSlots()); got != len(want) {
		t.Fatalf("子 slot 数量 = %d，清单是 %d", got, len(want))
	}
}

// TestActiveSectionDefaultIsFirstLedgerRow 钉住「打开时默认第一页」。
func TestActiveSectionDefaultIsFirstLedgerRow(t *testing.T) {
	facts := fullDesktopFacts()
	facts.CredentialStored = true
	if got := ActiveSectionDefault(facts.Sections()); got != "account" {
		t.Fatalf("默认 section = %q，官方取 ledger 第一项（account）", got)
	}
	if got := ActiveSectionDefault(nil); got != "" {
		t.Fatalf("没有 section 时默认值 = %q，应为空", got)
	}
}

// ── 小工具 ─────────────────────────────────────────────────────────

func sectionByID(sections []OfficialSection, id string) (OfficialSection, bool) {
	for _, s := range sections {
		if s.ID == id {
			return s, true
		}
	}
	return OfficialSection{}, false
}

func sectionGeneralItem(facts HostFacts, id string) (OfficialGeneralItem, bool) {
	for _, it := range facts.GeneralItems() {
		if it.ID == id {
			return it, true
		}
	}
	return OfficialGeneralItem{}, false
}

func mustItem(t *testing.T, id string) OfficialGeneralItem {
	t.Helper()
	it, ok := sectionGeneralItem(fullDesktopFacts(), id)
	if !ok {
		t.Fatalf("general item %q 没注册", id)
	}
	return it
}

func itemIDs(items []OfficialGeneralItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

// TestGeneralRowsQueryTheirOwnHostFields 守住"每一行查的是**自己**那个字段"。
//
// 2026-11-13 真机暴露：宿主补上 4 个 UI 命名空间后，界面仍有几行显示未知。原因不在宿主，
// 在**取字段这一侧**：
//   - 「语言」行写的是 `locale.active`，官方字段是 `locale.preference`
//     （`locale-settings.ts:9` LOCALE_PREFERENCE_FIELD）；
//   - transcript-view 的三行把 `it.HostField`（恒为 `ui-chat.transcriptView`）
//     当成 hostField 传给了 `generalRow`，于是第 2/3 行都去读同一个字段，
//     宿主明明发了 `ui-chat.collapseTiming` / `ui-chat.foldedReplies` 也读不到。
//
// **必须走 `BuildPanel` 真链路**，不能直接调 `generalRow`：缺陷在**调用点**，
// 直接调 helper 会把调用点绕过去，测不出问题（第一版就是这么写的，变异验证没红）。
func TestGeneralRowsQueryTheirOwnHostFields(t *testing.T) {
	// ① 语言行：官方字段名是 locale.preference。
	if lang := mustItem(t, "language"); lang.HostField != "locale.preference" {
		t.Fatalf("语言行的 HostField = %q，官方是 locale.preference", lang.HostField)
	}

	// ② 走真链路造 general 页，看三行各自查的是哪个字段。
	facts := HostFacts{TranscriptView: "standard"}
	model := BuildPanel(describeFixture(), facts)
	panel, ok := model.Section("general")
	if !ok {
		t.Fatal("BuildPanel 没有 general 页")
	}
	want := []string{"ui-chat.transcriptView", "ui-chat.collapseTiming", "ui-chat.foldedReplies"}
	var got []string
	for i := 0; i < len(want); i++ {
		row, ok := panel.Row("transcript-view-" + strconv.Itoa(i))
		if !ok {
			t.Fatalf("general 页缺 transcript-view-%d 这一行", i)
		}
		got = append(got, row.Field)
	}
	seen := map[string]bool{}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 行查的字段 = %q，官方是 %q", i, got[i], want[i])
		}
		if seen[got[i]] {
			t.Fatalf("第 %d 行与前面的行查了同一个字段 %q——宿主发了也读不到", i, got[i])
		}
		seen[got[i]] = true
	}
}

// TestLanguageRowShowsEffectiveLocaleWhenPreferenceUnset 守住「未设值 ≠ 未知」。
//
// 官方 `locale.preference` 是**可选**字段：缺省 = 交给浏览器
// （`locale-settings.ts:12` "absence delegates to the browser"）。
// 官方 `LanguageRow` 显示的是**生效** locale 的 label，不是「未知」：
// `options.find(o => o.id === active)?.label ?? active`（LanguageRow.tsx:36-37）。
//
// 真机 2026-11-13：宿主已按官方发 `locale.preference`（值 `{}` = 未设），
// 界面却显示「未知」——与官方行为不符。本仓界面只有中文一套文案，生效 locale 是 `zh`。
func TestLanguageRowShowsEffectiveLocaleWhenPreferenceUnset(t *testing.T) {
	// 夹具：locale 命名空间在、`preference` 字段在，但 current 为空（官方缺省态）。
	describe := &apiclient.SettingsDescribeResult{
		Namespaces: []apiclient.SettingsNamespaceView{{
			NS: "locale", Applies: "live", Value: json.RawMessage(`{}`),
		}},
		Sections: []apiclient.SettingsSectionView{{
			NS: "locale", Title: "Language", Writable: true,
			Fields: []apiclient.SettingsFieldView{{
				Name: "preference", Path: []string{"preference"}, Type: "string", Writable: true,
				// Current 故意留空：官方 axp 是「absence delegates to the browser」
			}},
		}},
	}
	model := BuildPanel(describe, HostFacts{TranscriptView: "standard"})
	panel, ok := model.Section("general")
	if !ok {
		t.Fatal("BuildPanel 没有 general 页")
	}
	row, ok := panel.Row("language")
	if !ok {
		t.Fatal("general 页缺 language 行")
	}
	if row.Disabled {
		t.Fatalf("字段在且可写，不该 disabled：reason=%q", row.Reason)
	}
	if !row.Known {
		t.Fatal("未设值不等于未知：官方显示生效 locale 的 label，这里应已知（zh / 简体中文）")
	}
	if row.Value != "简体中文" {
		t.Fatalf("生效 locale 是 zh，label 应为「简体中文」，拿到 %q", row.Value)
	}
}

// 反向：字段**真的不存在**时仍然必须是未知（不能拿 zh 把真缺陷盖掉）。
func TestLanguageRowStaysUnknownWhenFieldMissing(t *testing.T) {
	describe := &apiclient.SettingsDescribeResult{
		Namespaces: []apiclient.SettingsNamespaceView{{NS: "locale", Applies: "live"}},
		Sections: []apiclient.SettingsSectionView{{
			NS: "locale", Title: "Language", Writable: true,
			Fields: []apiclient.SettingsFieldView{{Name: "somethingElse", Path: []string{"somethingElse"}, Type: "string", Writable: true}},
		}},
	}
	model := BuildPanel(describe, HostFacts{TranscriptView: "standard"})
	panel, _ := model.Section("general")
	row, ok := panel.Row("language")
	if !ok {
		t.Fatal("general 页缺 language 行")
	}
	if row.Known {
		t.Fatalf("字段真的不存在时必须未知，不能拿 zh 盖过去：value=%q", row.Value)
	}
}
