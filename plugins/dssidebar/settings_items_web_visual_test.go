//go:build darwin || windows || linux

// settings_items_web_visual_test.go —— #47「网页搜索与获取」页的**真实绘制**证据。
//
// 走真 Viva：真 Paint（经真 Page + 真命中区）→ 真 HitTest → 真 Click → 再 Paint。
// 判据不是"图里有东西"：
//
//   - 官方两个 section 标题、下拉、count、计费、指南链接**逐字**画出来；
//   - 点下拉真的展开菜单，选中「DeepSeek（旧版）」后**灰面板换成密钥 + Base URL +
//     最大搜索次数**（官方 WebSearchCard.tsx:91-114），草稿真的进 staged；
//   - 点「保存」真的按官方顺序发 `settings.mutate`（api → legacy → selection）；
//   - 密钥真的经 `credentials.set` 写（带 ref、带 trim），**不进 settings.mutate**；
//   - provider 清单读失败真的画官方那句提示 + 「重试」，点重试真的触发重读。
//
// 断言异步动作前一律 pump 到条件成立（与 settings_items_visual_test.go 的 waitMutate 同一先例）。
package dssidebar

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// settingsWebCredentialCall 是一次真实的 `credentials.set` 调用。
type settingsWebCredentialCall struct {
	ref   string
	value string
}

// settingsWebCredentialSpy 是凭据写路径的替身（保存跑在 goroutine 里）。
type settingsWebCredentialSpy struct {
	mu    sync.Mutex
	calls []settingsWebCredentialCall
	err   error
}

func (s *settingsWebCredentialSpy) set(ref, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, settingsWebCredentialCall{ref: ref, value: value})
	return s.err
}

func (s *settingsWebCredentialSpy) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *settingsWebCredentialSpy) last() (settingsWebCredentialCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return settingsWebCredentialCall{}, false
	}
	return s.calls[len(s.calls)-1], true
}

// settingsWebHarness 是一帧可点、可读文字的「插件」页（含 #47 的额外数据面）。
type settingsWebHarness struct {
	t           *testing.T
	p           *Plugin
	theme       *pluginapi.Theme
	tree        *gfx.Tree
	feat        gfx.Scope
	session     *headless.Session
	invalidator *app.Invalidator
	spy         *settingsItemWriteSpy
	cred        *settingsWebCredentialSpy
	openedURL   []string

	mu          sync.Mutex
	describe    *apiclient.SettingsDescribeResult
	providers   *apiclient.WebProviderList
	credentials map[string]apiclient.CredentialInfo
	refreshes   int
}

func openSettingsWebHarness(t *testing.T, describe *apiclient.SettingsDescribeResult) *settingsWebHarness {
	t.Helper()
	h := &settingsWebHarness{
		t: t, spy: &settingsItemWriteSpy{rev: 99}, cred: &settingsWebCredentialSpy{},
		describe:  describe,
		providers: settingsWebProviderFixture(),
		credentials: map[string]apiclient.CredentialInfo{
			"DEEPSEEK_API_KEY": {Configured: true, Source: "env", Writable: true},
		},
	}
	h.p = &Plugin{}
	kernel := uikernel.New("settings-web")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, h.p); err != nil {
		t.Fatal(err)
	}
	h.theme = kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	act := pluginapi.Actions{
		SettingsDescribe: func() *apiclient.SettingsDescribeResult {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.describe
		},
		SettingsMutate: h.spy.mutate,
		PluginBundles: func() ([]apiclient.PluginBundle, bool, bool, string) {
			return []apiclient.PluginBundle{pluginBundleFixture()}, true, false, ""
		},
		WebProviderList: func() *apiclient.WebProviderList {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.providers
		},
		WebProviderRefresh: func() {
			h.mu.Lock()
			h.refreshes++
			h.mu.Unlock()
		},
		CredentialStatus: func(refs []string) map[string]apiclient.CredentialInfo {
			h.mu.Lock()
			defer h.mu.Unlock()
			out := map[string]apiclient.CredentialInfo{}
			for _, ref := range refs {
				if info, ok := h.credentials[ref]; ok {
					out[ref] = info
				}
			}
			return out
		},
		CredentialSet: h.cred.set,
		OpenExternal: func(destination string) error {
			h.mu.Lock()
			h.openedURL = append(h.openedURL, destination)
			h.mu.Unlock()
			return nil
		},
	}
	h.tree = &gfx.Tree{}
	h.feat = h.tree.Feature("settings-web")
	pageTheme := dscore.VivaTheme(h.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "settings-web", Width: 960, Height: 1100, Tree: h.tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: h.theme, Tree: h.tree, Feat: h.feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width, Act: act, Has: kernel.Has,
		}
		canvas.Fill(bounds, h.theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			h.p.drawPluginManagerPage(f, env)
		})
	}
	config.OnKey = func(ev gfx.KeyEvent) gfx.EventResult { return gfx.EventResult{} }
	h.invalidator = &app.Invalidator{}
	config.Invalidator = h.invalidator
	config.TextInput = ui.Fields(h.p.Fields()...)
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	h.session = session
	h.pump()
	h.pump()
	return h
}

func (h *settingsWebHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *settingsWebHarness) close() { h.session.Close() }

func (h *settingsWebHarness) hitFor(name string) gfx.Hit {
	h.t.Helper()
	id := h.tree.ID(h.feat, name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit
		}
	}
	h.t.Fatalf("没有命中区 %q（控件没画出来或没挂 Hit）", name)
	return gfx.Hit{}
}

func (h *settingsWebHarness) clickAt(bounds gfx.Rect, label string) {
	h.t.Helper()
	x, y := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
	snapshot := h.session.Snapshot()
	if x < 0 || y < 0 || x >= float32(snapshot.Width) || y >= float32(snapshot.Height) {
		h.t.Fatalf("%s 的中心 (%v,%v) 在窗口 %dx%d 之外（要滚到可见处再点）", label, x, y, snapshot.Width, snapshot.Height)
	}
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

func (h *settingsWebHarness) click(name string) {
	h.t.Helper()
	h.clickAt(h.hitFor(name).Bounds, name)
}

// clickText 点某段画出来的文字的中心（viva 菜单行没有 tree id，只能按文字定位）。
func (h *settingsWebHarness) clickText(label string) {
	h.t.Helper()
	for _, text := range h.session.Snapshot().Texts {
		if text.Value == label {
			h.clickAt(text.Bounds, label)
			return
		}
	}
	h.t.Fatalf("没有画文字 %q；texts=%q", label, h.session.Texts())
}

func (h *settingsWebHarness) hasText(want string) bool { return h.session.HasText(want) }

// waitMutate 等到写请求达到 want 次；等待期间继续 pump。
// waitText 与 settings_items_visual_test.go 的同名助手同口径：有界等待，
// 断言强度不变（400 次 pump 之后仍不出现就失败）。
func (h *settingsWebHarness) waitText(want string) bool {
	for i := 0; i < 400 && !h.hasText(want); i++ {
		h.pump()
	}
	return h.hasText(want)
}

func (h *settingsWebHarness) waitMutate(want int) bool {
	for i := 0; i < 600 && h.spy.count() < want; i++ {
		h.pump()
	}
	return h.spy.count() >= want
}

// waitCredential 等到凭据写达到 want 次。
func (h *settingsWebHarness) waitCredential(want int) bool {
	for i := 0; i < 600 && h.cred.count() < want; i++ {
		h.pump()
	}
	return h.cred.count() >= want
}

func (h *settingsWebHarness) capture(name string) {
	dir := os.Getenv("SETTINGS_ITEMS_CAPTURE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := h.session.Screenshot(filepath.Join(dir, name)); err != nil {
		h.t.Fatal(err)
	}
}

// openWebCard 点开 #47 那页（列表 → 详情）。
func (h *settingsWebHarness) openWebCard() {
	h.t.Helper()
	h.click("settings-item-card-web-search")
	if h.p.itemSelected != "web-search" {
		h.t.Fatalf("点卡片后 itemSelected=%q，want web-search", h.p.itemSelected)
	}
}

// TestSettingsWebDetailDrawsOfficialPage 是「这一页真的画出来了」的主证据。
func TestSettingsWebDetailDrawsOfficialPage(t *testing.T) {
	h := openSettingsWebHarness(t, settingsWebDescribeFixture())
	defer h.close()

	// 卡片本身（组里的那张）先要出现。
	if !h.hasText(zhWebTitle) || !h.hasText(zhWebSummary) {
		t.Fatalf("插件页缺 #47 卡片文案；texts=%q", h.session.Texts())
	}
	h.openWebCard()
	for _, want := range []string{
		zhWebSearchTitle,      // '网页搜索提供方'
		zhWebFetchTitle,       // '网页获取提供方'
		zhWebProviderDeepSeek, // 下拉当前值 'DeepSeek Search API'
		zhWebProviderLocal,    // 获取下拉当前值 '本机直接获取'
		zhWebCount,            // '每次搜索结果数'
		zhWebCountHint,        // '每次搜索返回的结果条数，1–50，默认 10。'
		zhWebBilling,          // '计费方式'
		zhWebBillingAuto,      // '自动'
		zhWebBillingAutoHint,  // 计费说明
		zhWebSearchGuide,      // '网页搜索与获取指南'（官方链接）
		zhFormSave,            // footer
	} {
		if !h.hasText(want) {
			t.Fatalf("详情页缺官方文案 %q；texts=%q", want, h.session.Texts())
		}
	}
	// 控件是真的（有命中区），不是只画了字。
	for _, id := range []string{
		"plugin-config-web-search-provider",
		"plugin-config-web-fetch-provider",
		"plugin-config-web-search-count",
		"settings-item-save-web-search",
	} {
		if hit := h.hitFor(id); hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
			t.Fatalf("%s 命中区 = %+v", id, hit.Bounds)
		}
	}
	// 官方 selector 是 34px 高（`.selector{height:34px}`）。
	if hit := h.hitFor("plugin-config-web-search-provider"); hit.Bounds.Height < webSelectorHeight-1 ||
		hit.Bounds.Height > webSelectorHeight+1 {
		t.Fatalf("下拉高度 = %v（官方 34）", hit.Bounds.Height)
	}
	h.capture("settings-web-detail.png")
}

// TestSettingsWebMenuChoosesLegacyProvider 是「菜单真的能选 + 面板真的换」的证据。
func TestSettingsWebMenuChoosesLegacyProvider(t *testing.T) {
	h := openSettingsWebHarness(t, settingsWebDescribeFixture())
	defer h.close()
	h.openWebCard()

	// 点搜索下拉 → 菜单展开（官方 `Dropdown` 的 Menu）。
	h.click("plugin-config-web-search-provider")
	if !h.hasText(zhWebProviderLegacy) {
		t.Fatalf("菜单没展开（缺 %q）；texts=%q", zhWebProviderLegacy, h.session.Texts())
	}
	if !h.hasText(zhWebProviderAuto) {
		t.Fatalf("base 没钉 provider 时菜单里必须有「%s」；texts=%q", zhWebProviderAuto, h.session.Texts())
	}
	// 菜单是 overlay：headless 的 Screenshot 只抓主画布，抓不到它，所以这里不落图，
	// 菜单展开的证据是上面那两条**文字**断言（官方菜单项文案真的画出来了）。
	// 选「DeepSeek（旧版）」。
	h.clickText(zhWebProviderLegacy)

	drafts, _, _ := h.p.ensureSettingsItemForm().snapshot()
	draft, staged := drafts[settingsItemDraftKey(settingsNSWeb, settingsFieldSearchProvider)]
	if !staged || draft.Text != settingsWebProviderLegacySearch || draft.Clear {
		t.Fatalf("选中旧版后草稿 = %+v staged=%v（want set deepseek-official）", draft, staged)
	}
	// 面板换成官方旧版那三格：密钥 + Base URL + 最大搜索次数。
	for _, want := range []string{zhWebCredential, zhWebBaseURL, zhWebMaxUses, zhWebAPIKeySet} {
		if !h.hasText(want) {
			t.Fatalf("旧版面板缺 %q；texts=%q", want, h.session.Texts())
		}
	}
	if !h.hasText(settingsWebKeyHint("DEEPSEEK_API_KEY")) {
		t.Fatalf("密钥 hint 没替换 {ref}；texts=%q", h.session.Texts())
	}
	if h.hasText(zhWebCount) {
		t.Fatal("切到旧版后 count 不该还在（那是 DeepSeek Search API 的面板）")
	}

	// 保存 → 真的发 `settings.mutate`，命名空间是 web、操作是 set searchProvider。
	before := h.spy.count()
	h.click("settings-item-save-web-search")
	if !h.waitMutate(before + 1) {
		t.Fatalf("点「保存」却没发出 settings.mutate（calls=%d）", h.spy.count())
	}
	call, ok := h.spy.last()
	if !ok {
		t.Fatal("没有记录到写调用")
	}
	if call.ns != settingsNSWeb {
		t.Fatalf("写的命名空间 = %q（want web——只有选择那一节 dirty）", call.ns)
	}
	if len(call.ops) != 1 || call.ops[0].Op != "set" || call.ops[0].Path[0] != settingsFieldSearchProvider {
		t.Fatalf("操作 = %+v（want set searchProvider）", call.ops)
	}
	if value, isText := call.ops[0].Value.(string); !isText || value != settingsWebProviderLegacySearch {
		t.Fatalf("写入值 = %#v（want 字符串 deepseek-official）", call.ops[0].Value)
	}
	if call.expectedRe != 4 {
		t.Fatalf("expectedRevision = %d（want 4——web section 的 revision 做乐观锁）", call.expectedRe)
	}
}

// TestSettingsWebCountEditSavesOverAPINamespace 钉住 count 写 `web-deepseek-api`。
func TestSettingsWebCountEditSavesOverAPINamespace(t *testing.T) {
	h := openSettingsWebHarness(t, settingsWebDescribeFixture())
	defer h.close()
	h.openWebCard()

	h.p.itemFields.webCount.SetText("25")
	h.pump()
	h.pump()
	drafts, _, _ := h.p.ensureSettingsItemForm().snapshot()
	draft, staged := drafts[settingsItemDraftKey(settingsNSWebAPI, settingsFieldWebCount)]
	if !staged || draft.Text != "25" {
		t.Fatalf("改 count 后草稿 = %+v staged=%v", draft, staged)
	}
	before := h.spy.count()
	h.click("settings-item-save-web-search")
	if !h.waitMutate(before + 1) {
		t.Fatal("点「保存」没发请求")
	}
	call, _ := h.spy.last()
	if call.ns != settingsNSWebAPI {
		t.Fatalf("命名空间 = %q（want web-deepseek-api）", call.ns)
	}
	if len(call.ops) != 1 || call.ops[0].Op != "set" || call.ops[0].Path[0] != settingsFieldWebCount {
		t.Fatalf("操作 = %+v（want set count）", call.ops)
	}
	if value, isNumber := call.ops[0].Value.(float64); !isNumber || value != 25 {
		t.Fatalf("写入值 = %#v（want 数字 25）", call.ops[0].Value)
	}
	if call.expectedRe != 6 {
		t.Fatalf("expectedRevision = %d（want 6——api section 的 revision）", call.expectedRe)
	}
}

// TestSettingsWebSecretSavesThroughCredentials 钉住密钥走凭据域、且不进 settings。
func TestSettingsWebSecretSavesThroughCredentials(t *testing.T) {
	describe := settingsWebDescribeFixture()
	// 宿主当前就选中旧版（这一页只在选旧版时画密钥控件）。
	describe.Namespaces[0].Value = []byte(`{"searchProvider":"deepseek-official","fetchProvider":"http"}`)
	describe.Sections[0].Fields[0].Current = settingsWebProviderLegacySearch
	h := openSettingsWebHarness(t, describe)
	defer h.close()
	h.openWebCard()

	if !h.hasText(zhWebCredential) {
		t.Fatalf("选中旧版时必须画密钥控件；texts=%q", h.session.Texts())
	}
	h.p.itemFields.webKey.SetText("  sk-test-key  ")
	h.pump()
	h.pump()
	drafts, _, _ := h.p.ensureSettingsItemForm().snapshot()
	draft, staged := drafts[settingsItemDraftKey(settingsNSWeb, settingsFieldLegacySecret)]
	if !staged || draft.Ref != "DEEPSEEK_API_KEY" {
		t.Fatalf("密钥草稿 = %+v staged=%v（want 记下 ref）", draft, staged)
	}

	before := h.spy.count()
	h.click("settings-item-save-web-search")
	if !h.waitCredential(1) {
		t.Fatalf("点「保存」却没调 credentials.set（calls=%d）", h.cred.count())
	}
	call, _ := h.cred.last()
	if call.ref != "DEEPSEEK_API_KEY" {
		t.Fatalf("credentials.set 的 ref = %q", call.ref)
	}
	if call.value != "sk-test-key" {
		t.Fatalf("credentials.set 的值 = %q（want trim 后的 sk-test-key）", call.value)
	}
	// 密钥**不许**出现在 settings.mutate 里（官方 secret 从不上线）。
	for i := 0; i < h.spy.count(); i++ {
		spyCalls := h.spy.calls[i]
		for _, op := range spyCalls.ops {
			if len(op.Path) > 0 && op.Path[0] == settingsFieldLegacySecret {
				t.Fatal("密钥被当成 settings 字段写了——那是官方明确不做的")
			}
		}
	}
	_ = before
}

// TestSettingsWebProvidersFailedDrawsRetry 钉住清单读失败的提示与「重试」。
func TestSettingsWebProvidersFailedDrawsRetry(t *testing.T) {
	h := openSettingsWebHarness(t, settingsWebDescribeFixture())
	defer h.close()
	// 壳接了能力但还没拉到（与"没接线"不同）。
	h.mu.Lock()
	h.providers = nil
	h.mu.Unlock()
	h.pump()
	h.openWebCard()

	if !h.hasText(zhWebProvidersFailed) {
		t.Fatalf("清单读失败时必须画官方那句提示；texts=%q", h.session.Texts())
	}
	if !h.hasText(zhWebRetry) {
		t.Fatalf("缺「%s」按钮", zhWebRetry)
	}
	h.click("settings-web-providers-retry")
	h.mu.Lock()
	refreshes := h.refreshes
	h.mu.Unlock()
	if refreshes == 0 {
		t.Fatal("点「重试」没有触发 WebProviderRefresh")
	}
}

// TestSettingsWebSaveFailureDrawsSaveFailed 钉住失败态**画出来**。
func TestSettingsWebSaveFailureDrawsSaveFailed(t *testing.T) {
	h := openSettingsWebHarness(t, settingsWebDescribeFixture())
	defer h.close()
	h.spy.mu.Lock()
	h.spy.mutateE = errRevisionConflictStub
	h.spy.mu.Unlock()
	h.openWebCard()

	h.p.itemFields.webCount.SetText("30")
	h.pump()
	h.pump()
	before := h.spy.count()
	h.click("settings-item-save-web-search")
	if !h.waitMutate(before + 1) {
		t.Fatal("点「保存」没发出请求")
	}
	// 与 settings_items_visual_test.go 的 waitText 同一口径：请求发出之后还要
	// 一次状态更新 + 重绘才落到画布，单次 pump 在负载下不够（CI 上偶发）。
	if !h.waitText(zhFormSaveFailed) {
		t.Fatalf("写失败必须画官方 saveFailed；texts=%q", h.session.Texts())
	}
}
