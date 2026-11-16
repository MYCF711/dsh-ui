package dsbrowser

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// occurrenceEnv 固定一次 tab 身份；ConfigGet/Set 模拟宿主 ui-config 的合并语义。
func occurrenceEnv(sessionID, tabID string, host pluginapi.WebHost, cfg map[string]string) *pluginapi.Env {
	env := &pluginapi.Env{Web: host}
	env.Data.Cur = sessionID
	env.RightTabID = tabID
	env.ActiveRightTab = tabID
	env.ActiveRightTabKind = RightTabBrowser
	env.Act.ConfigGet = func() map[string]string { return cfg }
	env.Act.ConfigSet = func(kv map[string]string) {
		for key, value := range kv {
			cfg[key] = value
		}
	}
	return env
}

func navigateOccurrence(p *Plugin, env *pluginapi.Env, address string) {
	p.activate(env)
	p.address.SetText(address)
	p.submit(env)
}

func savedBrowserTabs(t *testing.T, cfg map[string]string, sessionID string) map[string]browserTabState {
	t.Helper()
	raw := cfg[browserConfigKey(sessionID)]
	if raw == "" {
		t.Fatal("浏览器状态没有写入 dsh.sidebar-browser.v1")
	}
	// restoreSession 复用生产解码与全部校验，测试不另造一套 JSON 判据。
	probe := &Plugin{}
	if err := probe.restoreSession(sessionID, raw); err != nil {
		t.Fatalf("保存状态读回失败：%v", err)
	}
	out := map[string]browserTabState{}
	prefix := sessionID + "\x00"
	for key, instance := range probe.instances {
		out[key[len(prefix):]] = instance.nav.value
	}
	return out
}

func TestBrowserOccurrencesKeepIndependentStateAndLateLoads(t *testing.T) {
	cfg := map[string]string{}
	host := &fakeHost{}
	p := &Plugin{}
	first := occurrenceEnv("s1", "tab1", host, cfg)
	second := occurrenceEnv("s1", "tab2", host, cfg)

	navigateOccurrence(p, first, "https://example.com/one")
	navigateOccurrence(p, first, "https://example.com/two")
	pageOne := p.page
	navigateOccurrence(p, second, "https://other.org/path")
	pageTwo := p.page
	if host.opened != 2 || pageOne.ID() == pageTwo.ID() {
		t.Fatalf("两个 occurrence 应有独立 WebPage：opened=%d ids=%v/%v", host.opened, pageOne.ID(), pageTwo.ID())
	}
	if got := len(p.instances[browserInstanceKey("s1", "tab1")].nav.value.Entries); got != 2 {
		t.Fatalf("tab1 历史被 tab2 串改：%d，want 2", got)
	}
	if got := p.instances[browserInstanceKey("s1", "tab1")].addressDraft; got != "https://example.com/two" {
		t.Fatalf("tab1 地址草稿 = %q", got)
	}
	p.activate(first)
	if p.page != pageOne || p.address.Text() != "https://example.com/two" {
		t.Fatalf("切回 tab1 没恢复自己的页面/地址：page=%p/%p address=%q", p.page, pageOne, p.address.Text())
	}
	p.activate(second)
	if p.nav.value.Navigation.State != navLoading || p.instances[browserInstanceKey("s1", "tab1")].nav.value.Navigation.State != navLoading {
		t.Fatal("前置：两个载具都应还在 loading")
	}
	// tab1 的晚到 load 只能推进 tab1，不能把当前 tab2 标成 known。
	host.onLoads[0](pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/two"})
	if got := p.instances[browserInstanceKey("s1", "tab1")].nav.value.Navigation.State; got != navKnown {
		t.Fatalf("tab1 晚到 load 后状态 = %v，want known", got)
	}
	if got := p.nav.value.Navigation.State; got != navLoading {
		t.Fatalf("tab1 的 load 串到当前 tab2：%v", got)
	}
	if got := p.RightTabTitle("s1", "timeline"); got != "" {
		t.Fatalf("浏览器改写了其他插件的标题：%q", got)
	}
	saved := savedBrowserTabs(t, cfg, "s1")
	if len(saved) != 2 || saved["tab1"].Index != 1 || saved["tab2"].Index != 0 {
		t.Fatalf("byTab 持久化错误：%+v", saved)
	}
}

func TestBrowserRestartShowsRestoreWithoutOpeningAndForgetsOnClose(t *testing.T) {
	cfg := map[string]string{}
	originalHost := &fakeHost{}
	original := &Plugin{}
	first := occurrenceEnv("s1", "tab1", originalHost, cfg)
	second := occurrenceEnv("s1", "tab2", originalHost, cfg)
	navigateOccurrence(original, first, "https://example.com/saved")
	navigateOccurrence(original, second, "https://other.org/saved")
	firstRevision := original.instances[browserInstanceKey("s1", "tab1")].nav.value.Request.Revision

	// 全新 Plugin = 进程重启。读回两份 checkpoint，但 Open 次数必须为 0。
	host := &fakeHost{}
	restarted := &Plugin{}
	first = occurrenceEnv("s1", "tab1", host, cfg)
	second = occurrenceEnv("s1", "tab2", host, cfg)
	restarted.activate(first)
	if host.opened != 0 || restarted.page != nil {
		t.Fatalf("恢复记录时自动开了网页：opened=%d page=%p", host.opened, restarted.page)
	}
	if got := restarted.address.Text(); got != "https://example.com/saved" {
		t.Fatalf("tab1 恢复地址 = %q", got)
	}
	if got := restarted.RightTabTitle("s1", "tab1"); got != "example.com" {
		t.Fatalf("恢复卡标题 = %q，want example.com", got)
	}
	restarted.activate(second)
	if host.opened != 0 || restarted.address.Text() != "https://other.org/saved" {
		t.Fatalf("tab2 恢复时不该开页，地址应独立：opened=%d address=%q", host.opened, restarted.address.Text())
	}
	restarted.activate(first)
	entriesBefore := len(restarted.nav.value.Entries)
	restarted.restore(first)
	if host.opened != 1 || restarted.page == nil {
		t.Fatalf("显式恢复没有创建页面：opened=%d", host.opened)
	}
	if len(restarted.nav.value.Entries) != entriesBefore {
		t.Fatalf("恢复重复追加历史：%d → %d", entriesBefore, len(restarted.nav.value.Entries))
	}
	if got := restarted.nav.value.Request.Revision; got != firstRevision+1 {
		t.Fatalf("恢复 revision = %d，want %d", got, firstRevision+1)
	}
	pageOne := restarted.page
	if err := restarted.CloseRightTab("s1", "tab1", RightTabBrowser, first.Act); err != nil {
		t.Fatalf("关闭 tab1：%v", err)
	}
	if !pageOne.(*fakePage).closed {
		t.Fatal("关闭 occurrence 没有 Close 它的 WebPage")
	}
	saved := savedBrowserTabs(t, cfg, "s1")
	if _, exists := saved["tab1"]; exists {
		t.Fatal("关闭后仍保留 tab1 checkpoint（forget 未生效）")
	}
	if _, exists := saved["tab2"]; !exists {
		t.Fatal("关闭 tab1 误删 tab2 checkpoint")
	}
}

func TestCloseUnseenRestoredTabPreservesOtherCheckpoints(t *testing.T) {
	cfg := map[string]string{}
	host := &fakeHost{}
	p := &Plugin{}
	first := occurrenceEnv("s1", "tab1", host, cfg)
	second := occurrenceEnv("s1", "tab2", host, cfg)
	navigateOccurrence(p, first, "https://example.com/")
	navigateOccurrence(p, second, "https://other.org/")

	// 没画过任何浏览器体就直接从右栏关闭 tab2：close handler 必须先读 store，
	// 再 forget(tab2)，不能用空内存覆盖掉 tab1。
	fresh := &Plugin{}
	if err := fresh.CloseRightTab("s1", "tab2", RightTabBrowser, first.Act); err != nil {
		t.Fatal(err)
	}
	saved := savedBrowserTabs(t, cfg, "s1")
	if len(saved) != 1 || saved["tab1"].Index != 0 {
		t.Fatalf("未加载实例关闭时误清空其他 tab：%+v", saved)
	}
}

func TestInvalidBrowserRestoreIsLoudAndDoesNotOpen(t *testing.T) {
	cfg := map[string]string{browserConfigKey("s1"): `{not-json`}
	host := &fakeHost{}
	p := &Plugin{}
	env := occurrenceEnv("s1", "tab1", host, cfg)
	var notices []string
	env.Act.Notify = func(message string) { notices = append(notices, message) }
	p.activate(env)
	if len(notices) == 0 {
		t.Fatal("损坏的恢复数据被静默吞掉")
	}
	if host.opened != 0 || p.page != nil {
		t.Fatal("损坏恢复数据不应创建页面")
	}
}

func TestDelayedBrowserConfigMergesInsteadOfOverwritingSavedTabs(t *testing.T) {
	savedCfg := map[string]string{}
	host := &fakeHost{}
	original := &Plugin{}
	first := occurrenceEnv("s1", "tab1", host, savedCfg)
	second := occurrenceEnv("s1", "tab2", host, savedCfg)
	navigateOccurrence(original, first, "https://example.com/old")
	navigateOccurrence(original, second, "https://other.org/old")
	savedRaw := savedCfg[browserConfigKey("s1")]

	var delayed map[string]string
	restartedHost := &fakeHost{}
	restarted := &Plugin{}
	env := occurrenceEnv("s1", "tab1", restartedHost, nil)
	writes := 0
	env.Act.ConfigGet = func() map[string]string { return delayed }
	env.Act.ConfigSet = func(kv map[string]string) {
		writes++
		for key, value := range kv {
			delayed[key] = value
		}
	}
	navigateOccurrence(restarted, env, "https://new.example/current")
	if writes != 0 {
		t.Fatal("配置仍是 nil 时就覆盖写回了浏览器 store")
	}
	delayed = map[string]string{browserConfigKey("s1"): savedRaw}
	restarted.activate(env)
	if restarted.address.Text() != "https://new.example/current" {
		t.Fatal("配置到达后用旧值覆盖了用户刚输入的当前 tab")
	}
	saved := savedBrowserTabs(t, delayed, "s1")
	if len(saved) != 2 || saved["tab2"].Index != 0 {
		t.Fatal("延迟配置合并后丢失旧 tab：", saved)
	}
	if writes == 0 {
		t.Fatal("配置到达并合并后没有写回完整 store")
	}
}

func TestCloseBrowserBeforeConfigArrivesFailsWithoutDeleting(t *testing.T) {
	p := &Plugin{}
	actions := pluginapi.Actions{ConfigGet: func() map[string]string { return nil }}
	if err := p.CloseRightTab("s1", "tab1", RightTabBrowser, actions); err == nil {
		t.Fatal("配置未就绪时关闭必须明确失败，不能按空 store 删除")
	}
}

func TestRestoreCardReallyClickOpensSavedPage(t *testing.T) {
	cfg := map[string]string{}
	host := &fakeHost{}
	original := &Plugin{}
	env := occurrenceEnv("s1", "tab1", host, cfg)
	navigateOccurrence(original, env, "https://example.com/saved")

	restartedHost := &fakeHost{}
	restarted := &Plugin{}
	restarted.theme = dscore.Dark()
	restartEnv := occurrenceEnv("s1", "tab1", restartedHost, cfg)
	tree := &gfx.Tree{}
	feat := tree.Feature("browser-restore-card")
	pageTheme := dscore.VivaTheme(restarted.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "browser-restore", Width: 360, Height: 600, Tree: tree, WindowMode: app.WindowFrameless}
	config.TextInput = ui.Fields(restarted.Fields()...)
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		restartEnv.Theme = restarted.theme
		restartEnv.Tree = tree
		restartEnv.Feat = feat
		restartEnv.Canvas = canvas
		restartEnv.Bounds = bounds
		canvas.Fill(bounds, restarted.theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).Width(360).Height(600).Children(func() { restarted.drawBody(f, restartEnv) })
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	session.Resize(360, 600)
	ready := false
	for i := 0; i < 20; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		texts := strings.Join(session.Texts(), "\n")
		if strings.Contains(texts, copyRestorePrevious) && strings.Contains(texts, "https://example.com/saved") {
			ready = true
			break
		}
	}
	if !ready {
		t.Fatal("恢复卡没有出现：", session.Texts())
	}
	if restartedHost.opened != 0 {
		t.Fatal("恢复卡出现之前就自动打开网页")
	}
	if err := session.ClickLabel(copyRestoreAction); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20 && restartedHost.opened == 0; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if restartedHost.opened != 1 {
		t.Fatal("点击恢复页面后 Open 次数 = ", restartedHost.opened, "，want 1")
	}
	if got := len(restarted.nav.value.Entries); got != 1 {
		t.Fatal("点击恢复重复追加历史，entries = ", got)
	}
}
