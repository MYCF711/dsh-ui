//go:build darwin || windows || linux

// settings_items_visual_test.go —— 官方 4 条插件设置页的**真实绘制**证据（本轮 #39/#45/#46）。
//
// 走真 Viva：真 Paint（经真 Page + 真命中区）→ 真 HitTest → 真 Click → 再 Paint。
// 判据不是「图里有东西」：
//
//   - 「内置插件配置」组与三张卡**真的画在屏幕上**，且排在「已安装插件」之前；
//   - 点卡片真的打开详情页，官方的 label / hint / 「保存」逐字画出来；
//   - user 层有值的字段真的带「已覆盖」+「恢复默认」；
//   - 只读 section 真的画「本部署的设置为只读。」；
//   - 输入框里改了值 → 真的进草稿 → 点「保存」**真的**发出
//     `settings.mutate`（带 baseline revision 做乐观锁）。
//
// 断言异步动作前一律 pump 到条件成立（与 dssettings 的 waitWriterCalls 同一先例）：
// 保存本来就是 `go func`，点完立刻读 writer 是在赌 goroutine 调度。
package dssidebar

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

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

// errRevisionConflictStub 是一次写失败的替身（模拟宿主 409）。
var errRevisionConflictStub = errors.New("revision_conflict: settings 已被外部修改")

// settingsItemMutateCall 是一次真实的 `settings.mutate` 调用。
type settingsItemMutateCall struct {
	ns         string
	ops        []apiclient.SettingsOp
	expectedRe int
}

// settingsItemWriteSpy 是线程安全的写路径替身（保存跑在 goroutine 里）。
type settingsItemWriteSpy struct {
	mu      sync.Mutex
	calls   []settingsItemMutateCall
	rev     int
	mutateE error
}

func (s *settingsItemWriteSpy) mutate(ns string, ops []apiclient.SettingsOp, expectedRevision int) (apiclient.SettingsNamespaceView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, settingsItemMutateCall{ns: ns, ops: append([]apiclient.SettingsOp(nil), ops...), expectedRe: expectedRevision})
	if s.mutateE != nil {
		return apiclient.SettingsNamespaceView{}, s.mutateE
	}
	return apiclient.SettingsNamespaceView{NS: ns, Revision: s.rev}, nil
}

func (s *settingsItemWriteSpy) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *settingsItemWriteSpy) last() (settingsItemMutateCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return settingsItemMutateCall{}, false
	}
	return s.calls[len(s.calls)-1], true
}

// settingsItemHarness 是一帧可点、可读文字的插件管理页。
type settingsItemHarness struct {
	t              *testing.T
	p              *Plugin
	kernel         *uikernel.Kernel
	theme          *pluginapi.Theme
	tree           *gfx.Tree
	feat           gfx.Scope
	session        *headless.Session
	invalidator    *app.Invalidator
	spy            *settingsItemWriteSpy
	mu             sync.Mutex
	describe       *apiclient.SettingsDescribeResult
	bundles        []apiclient.PluginBundle
	cotLimits      apiclient.CotTranslationLimits
	cotRefreshes   int
	catalog        *apiclient.ModelsPageSnapshot
	catalogReads   int
	catalogRetries int
}

func openSettingsItemHarness(t *testing.T, width, height int) *settingsItemHarness {
	t.Helper()
	h := &settingsItemHarness{t: t, spy: &settingsItemWriteSpy{rev: 99}}
	h.describe = settingsItemDescribeFixture()
	h.catalog = settingsSubagentCatalogFixture()
	h.bundles = []apiclient.PluginBundle{pluginBundleFixture()}
	h.cotLimits = apiclient.CotTranslationLimits{
		MaxTextChars:       100000,
		Preferences:        apiclient.CotTranslationPreferences{Provider: "deepseek-official", TargetLanguage: "auto"},
		AvailableProviders: []string{"deepseek-official", "deepseek-account"},
	}
	h.p = &Plugin{}
	h.kernel = uikernel.New("settings-items")
	if err := h.kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, h.p); err != nil {
		t.Fatal(err)
	}
	h.theme = h.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	act := pluginapi.Actions{
		SettingsDescribe: func() *apiclient.SettingsDescribeResult {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.describe
		},
		SettingsMutate: h.spy.mutate,
		CotTranslationLimits: func() (apiclient.CotTranslationLimits, bool) {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.cotLimits, true
		},
		CotTranslationRefresh: func() {
			h.mu.Lock()
			h.cotRefreshes++
			h.mu.Unlock()
		},
		ModelsPageSnapshot: func() *apiclient.ModelsPageSnapshot {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.catalogReads++
			return h.catalog
		},
		ModelsPageRefresh: func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.catalogRetries++
			h.catalog = settingsSubagentCatalogFixture()
		},
		PluginBundles: func() ([]apiclient.PluginBundle, bool, bool, string) {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.bundles, true, false, ""
		},
	}
	h.tree = &gfx.Tree{}
	h.feat = h.tree.Feature("settings-items")
	pageTheme := dscore.VivaTheme(h.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "settings-items", Width: width, Height: height, Tree: h.tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: h.theme, Tree: h.tree, Feat: h.feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width, Act: act, Has: h.kernel.Has,
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

func (h *settingsItemHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *settingsItemHarness) close() { h.session.Close() }

func (h *settingsItemHarness) hitFor(name string) gfx.Hit {
	h.t.Helper()
	id := h.tree.ID(h.feat, name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit
		}
	}
	h.t.Fatalf("没有命中区 %q（卡片/按钮没画出来或没挂 Hit）", name)
	return gfx.Hit{}
}

func (h *settingsItemHarness) click(name string) {
	h.t.Helper()
	hit := h.hitFor(name)
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	snapshot := h.session.Snapshot()
	if x < 0 || y < 0 || x >= float32(snapshot.Width) || y >= float32(snapshot.Height) {
		h.t.Fatalf("%s 的中心 (%v,%v) 在窗口 %dx%d 之外", name, x, y, snapshot.Width, snapshot.Height)
	}
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

func (h *settingsItemHarness) hasText(want string) bool { return h.session.HasText(want) }

// clickText 点菜单项文字的中心；viva 菜单项没有 tree id，与 web 设置页同一先例。
func (h *settingsItemHarness) clickText(label string) {
	h.t.Helper()
	for _, text := range h.session.Snapshot().Texts {
		if text.Value == label {
			x, y := text.Bounds.X+text.Bounds.Width/2, text.Bounds.Y+text.Bounds.Height/2
			if err := h.session.Click(x, y); err != nil {
				h.t.Fatal(err)
			}
			h.pump()
			h.pump()
			return
		}
	}
	h.t.Fatalf("没有画文字 %q；texts=%q", label, h.session.Texts())
}

// waitMutate 等到写请求达到 want 次；等待期间继续 pump。
//
// 为什么不能点完立刻断言：保存是 `go applySettingsItemSave(...)`（绘制线程不能等 HTTP），
// 新 goroutine 未必已被调度。断言本身一点没放松，只是等它落地。
func (h *settingsItemHarness) waitMutate(want int) bool {
	// 有界等待：**按时间**且有**调度让出**。
	//
	// 旧写法是 `for i := 0; i < 400; i++ { h.pump() }` —— 400 次紧凑循环只花微秒，
	// 而 pump() 不让出调度，取消/写盘那条 goroutine 在负载下可能一次机会都没拿到，
	// 于是等待必然在次数用完时落空、报成「没发出请求」。那是测试自己把等待写成空转，
	// 不是被测行为。这里换成时间上限 + runtime.Gosched()，断言强度不变。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && h.spy.count() < want {
		h.pump()
		runtime.Gosched()
	}
	return h.spy.count() >= want
}

// waitDraftClear 有界等待草稿清空，与 waitMutate 同风格：
// 保存成功之后 staged.clear() 还要经过一次状态更新才可见，单次 pump 在负载下
// 不够（CI 上偶发：drafts 里仍是刚提交的那一项）。
func (h *settingsItemHarness) waitDraftClear() (map[string]settingsItemDraft, bool) {
	for i := 0; i < 400; i++ {
		if drafts, _, _ := h.p.ensureSettingsItemForm().snapshot(); len(drafts) == 0 {
			return drafts, true
		}
		h.pump()
	}
	drafts, _, _ := h.p.ensureSettingsItemForm().snapshot()
	return drafts, false
}

// waitText 有界等待某段文字出现，与 waitMutate 同风格：请求发出之后，
// 失败响应还要经过一次状态更新 + 重绘才落到画布上，单次 pump 在负载下不够
// （CI 上偶发：texts 里已是设置页但还没有 saveFailed）。
// 断言强度不变——400 次 pump 之后仍没出现就还是失败。
func (h *settingsItemHarness) waitText(want string) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !h.hasText(want) {
		h.pump()
		runtime.Gosched()
	}
	return h.hasText(want)
}

// capture 在断言通过后落盘（SETTINGS_ITEMS_CAPTURE_DIR 非空时保留）。
func (h *settingsItemHarness) capture(name string) {
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

// TestSettingsItemsGroupDrawsOfficialCards 是「组 + 三张卡」的主证据。
func TestSettingsItemsGroupDrawsOfficialCards(t *testing.T) {
	h := openSettingsItemHarness(t, 960, 900)
	defer h.close()

	// ① 官方组标题（OFF:locales.ts:34 'basicTitle'）。
	if !h.hasText(settingsItemsGroupTitle) {
		t.Fatalf("没有画「%s」组标题；texts=%q", settingsItemsGroupTitle, h.session.Texts())
	}
	// ② 三张卡的标题与一句话（官方 zh 逐字）。
	for _, want := range []string{zhSubagentTitle, zhSubagentSummary, zhAgentLoopTitle, zhAgentLoopSummary, zhShellTitle, zhShellSummary} {
		if !h.hasText(want) {
			t.Fatalf("缺官方文案 %q——卡片没画出来", want)
		}
	}
	// ③ 三张卡各有真实命中区（不是只画了字）。
	for _, id := range []string{"settings-item-card-subagent", "settings-item-card-agent-loop", "settings-item-card-shell"} {
		if hit := h.hitFor(id); hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
			t.Fatalf("%s 命中区=%+v", id, hit.Bounds)
		}
	}
	// ④ 组序：官方是 basic → extensions → bundles，「已安装插件」必须在卡片之后。
	basicY := h.hitFor("settings-item-card-subagent").Bounds.Y
	bundlesTitleY := float32(-1)
	for _, text := range h.session.Snapshot().Texts {
		if text.Value == zhPluginBundlesTtl {
			bundlesTitleY = text.Bounds.Y
		}
	}
	if bundlesTitleY >= 0 && bundlesTitleY < basicY {
		t.Fatalf("「%s」画在卡片之前（y=%v vs %v）——官方组序是 basic 在前", zhPluginBundlesTtl, bundlesTitleY, basicY)
	}
	h.capture("settings-items-group.png")
}

// TestSettingsItemDetailDrawsOfficialForm 钉住详情页的官方文案与控件。
func TestSettingsItemDetailDrawsOfficialForm(t *testing.T) {
	h := openSettingsItemHarness(t, 960, 900)
	defer h.close()

	h.click("settings-item-card-subagent")
	if h.p.itemSelected != "subagent" {
		t.Fatalf("点卡片后 itemSelected=%q，want subagent", h.p.itemSelected)
	}
	for _, want := range []string{
		zhSubagentLimits,    // section 标题「运行限制」
		zhSubagentMaxDepth,  // 字段 label
		zhSubagentActive,    // 字段 label
		zhSubagentDepthBody, // 帮助正文（官方 help.content 第一段）
		zhSubagentDepthTail, // 帮助正文（官方 help.content 末段）
		zhSubagentCapBody,   // 第二个字段的帮助正文
		zhFormOverridden,    // maxActiveSubagents 在 user 层有值 → 「已覆盖」
		zhFormReset,         // 「恢复默认」
		zhFormSave,          // footer 的保存
	} {
		if !h.hasText(want) {
			t.Fatalf("详情页缺官方文案 %q；texts=%q", want, h.session.Texts())
		}
	}
	// 没被覆盖过的字段（maxDepth）不该带「恢复默认」按钮：整页只有一个 reset 命中区。
	resets := 0
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == h.tree.ID(h.feat, "settings-item-reset-plugin-config-subagent-depth") {
			resets++
		}
	}
	if resets != 0 {
		t.Fatal("maxDepth 没被覆盖过，不该画「恢复默认」")
	}
	h.capture("settings-item-detail-subagent.png")

	// 面包屑回列表。
	h.click("settings-item-back")
	if h.p.itemSelected != "" {
		t.Fatalf("点面包屑后 itemSelected=%q，want 空", h.p.itemSelected)
	}
	if !h.hasText(settingsItemsGroupTitle) {
		t.Fatal("回列表后应重新画组标题")
	}
}

// TestSettingsItemReadOnlySectionDrawsNote 钉住只读文案（官方 readOnly）。
func TestSettingsItemReadOnlySectionDrawsNote(t *testing.T) {
	h := openSettingsItemHarness(t, 960, 900)
	defer h.close()
	h.mu.Lock()
	h.describe.Sections[1].Writable = false // agent-loop
	h.mu.Unlock()

	h.click("settings-item-card-agent-loop")
	if !h.hasText(zhFormReadOnly) {
		t.Fatalf("只读 section 必须画「%s」；texts=%q", zhFormReadOnly, h.session.Texts())
	}
	if !h.hasText(zhAgentLoopParallel) || !h.hasText(zhAgentLoopParallelTip) {
		t.Fatal("只读也该画字段与 hint（官方只是禁用输入框）")
	}
}

// TestSettingsItemSaveSendsMutateWithBaseline 是**写入**的主证据：
// 输入框改值 → 草稿 → 点「保存」 → 真的发 settings.mutate，带 baseline revision。
func TestSettingsItemSaveSendsMutateWithBaseline(t *testing.T) {
	h := openSettingsItemHarness(t, 960, 900)
	defer h.close()

	h.click("settings-item-card-shell")
	if !h.hasText(zhShellTimeout) {
		t.Fatalf("终端页没画出来；texts=%q", h.session.Texts())
	}
	// 用户在输入框里改了值（走的是同一只 ui.Field：壳把键盘事件灌给它）。
	h.p.itemFields.shellTimeout.SetText("90000")
	h.pump()
	h.pump()

	drafts, _, _ := h.p.ensureSettingsItemForm().snapshot()
	draft, staged := drafts[settingsItemDraftKey(settingsNSPwshSandbox, settingsFieldShellTimeout)]
	if !staged || draft.Text != "90000" {
		t.Fatalf("输入框改值后必须进草稿，实际 drafts=%v（staged=%v）", drafts, staged)
	}
	// 「保存」此时必须可点（dirty + writable + 无 invalid）。
	before := h.spy.count()
	h.click("settings-item-save-shell")
	if !h.waitMutate(before + 1) {
		t.Fatalf("点「保存」却没发出 settings.mutate（calls=%d）", h.spy.count())
	}
	call, ok := h.spy.last()
	if !ok {
		t.Fatal("没有记录到写调用")
	}
	if call.ns != settingsNSPwshSandbox {
		t.Fatalf("写的命名空间=%q，want %q", call.ns, settingsNSPwshSandbox)
	}
	if len(call.ops) != 1 || call.ops[0].Op != "set" || call.ops[0].Path[0] != settingsFieldShellTimeout {
		t.Fatalf("操作=%+v，want set timeoutMs", call.ops)
	}
	if value, isNumber := call.ops[0].Value.(float64); !isNumber || value != 90000 {
		t.Fatalf("写入值=%#v，want 数字 90000", call.ops[0].Value)
	}
	if call.expectedRe != 3 {
		t.Fatalf("expectedRevision=%d，want 3（section.revision 做乐观锁）", call.expectedRe)
	}
	// 成功后草稿清空（官方 `staged.clear()`）。
	if leftovers, cleared := h.waitDraftClear(); !cleared {
		t.Fatalf("保存成功后草稿必须清空，实际 %v", leftovers)
	}
}

// TestSettingsItemWriteFailureDrawsSaveFailed 钉住失败态**画出来**（不是只存字段）。
func TestSettingsItemWriteFailureDrawsSaveFailed(t *testing.T) {
	h := openSettingsItemHarness(t, 960, 900)
	defer h.close()
	h.spy.mu.Lock()
	h.spy.mutateE = errRevisionConflictStub
	h.spy.mu.Unlock()

	h.click("settings-item-card-agent-loop")
	h.p.itemFields.agentLoopParallel.SetText("16")
	h.pump()
	h.pump()
	before := h.spy.count()
	h.click("settings-item-save-agent-loop")
	if !h.waitMutate(before + 1) {
		t.Fatal("点「保存」没发出请求")
	}
	if !h.waitText(zhFormSaveFailed) {
		t.Fatalf("写失败必须画官方 saveFailed；texts=%q", h.session.Texts())
	}
}
