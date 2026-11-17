// persistence.go —— 每 Session、每 tab occurrence 的浏览器状态与恢复。
//
// 官方 createBrowserStore 使用 persist:'dsh.sidebar-browser.v1'，BrowserController
// 以 tabId 调 replace/forget。这里落在宿主 ui-config 的同名 namespace；JSON 仍是
// bySession → byTab → BrowserTabState。恢复只重建应用侧记录，不自动创建 WebPage。
package dsbrowser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

const browserPersistence = "dsh.sidebar-browser.v1"

func browserConfigKey(sessionID string) string { return browserPersistence + "." + sessionID }

type browserStoreEnvelope struct {
	BySession map[string]browserSessionStore `json:"bySession"`
}

type browserSessionStore struct {
	ByTab map[string]browserTabState `json:"byTab"`
}

// browserInstance 是一个 occurrence 的独立运行态。Plugin 当前显示的字段在
// 切换时与这里交换；未选中的 WebPage 保留但不再 NativeView 声明。
type browserInstance struct {
	nav          browserNavigation
	page         pluginapi.WebPage
	openErr      error
	failure      addressFailure
	addressDraft string
	// paramsURL 记住这个 occurrence 已经消费过的那份官方
	// `tab.navigation.params.url`（contract/params.ts:31-46）；同一份只消费一次。
	paramsURL string
	// started 对应官方 `BrowserController.started`：`start(initialUrl)` 是一次性的，
	// 只有"还没请求过页面"时才吃初始地址（BrowserController.ts:90-98）。
	started bool
}

func browserInstanceKey(sessionID, tabID string) string { return sessionID + "\x00" + tabID }

func browserEnvIdentity(env *pluginapi.Env) (sessionID, tabID string) {
	if env == nil {
		return "", RightTabBrowser
	}
	sessionID = env.Data.Cur
	tabID = env.RightTabID
	if tabID == "" {
		tabID = env.ActiveRightTab
	}
	if tabID == "" {
		tabID = RightTabBrowser
	}
	return sessionID, tabID
}

// activate 把共享地址框与当前绘制字段绑定到本帧 occurrence。
// 第一次绑定不覆盖调用方已填的地址（兼容 submit 先于首帧的旧测试与壳）。
func (p *Plugin) activate(env *pluginapi.Env) *browserInstance {
	sessionID, tabID := browserEnvIdentity(env)
	if p.loadedSessions == nil {
		p.loadedSessions = map[string]bool{}
	}
	if p.dirtySessions == nil {
		p.dirtySessions = map[string]bool{}
	}
	refreshed := false
	if !p.loadedSessions[sessionID] && env != nil && env.Act.ConfigGet != nil {
		cfg := env.Act.ConfigGet()
		if cfg != nil {
			p.loadedSessions[sessionID] = true
			p.saveActive()
			var err error
			if p.dirtySessions[sessionID] {
				err = p.mergeRestoredSession(sessionID, cfg[browserConfigKey(sessionID)])
			} else {
				err = p.restoreSession(sessionID, cfg[browserConfigKey(sessionID)])
			}
			if err != nil && env.Act.Notify != nil {
				env.Act.Notify("浏览器恢复数据无效，已忽略: " + err.Error())
			}
			refreshed = true
		}
	}
	key := browserInstanceKey(sessionID, tabID)
	if p.instances == nil {
		p.instances = map[string]*browserInstance{}
	}
	instance := p.instances[key]
	if instance == nil {
		instance = &browserInstance{nav: newBrowserNavigation()}
		p.instances[key] = instance
	}
	if p.currentKey == key {
		if refreshed {
			p.nav = instance.nav
			p.page = instance.page
			p.openErr = instance.openErr
			p.failure = instance.failure
			p.address.SetText(instance.addressDraft)
		}
		if refreshed && p.dirtySessions[sessionID] {
			p.saveActive()
			if err := p.persistSession(env, sessionID); err != nil && env.Act.Notify != nil {
				env.Act.Notify("保存浏览器状态失败: " + err.Error())
			}
		}
		return instance
	}
	if p.currentKey != "" {
		p.saveActive()
		p.address.SetText(instance.addressDraft)
	} else if p.address.Text() == "" {
		p.address.SetText(instance.addressDraft)
	}
	p.currentKey = key
	p.nav = instance.nav
	p.page = instance.page
	p.openErr = instance.openErr
	p.failure = instance.failure
	p.content = ui.Box{}
	p.drawn = false
	if refreshed && p.dirtySessions[sessionID] {
		p.saveActive()
		if err := p.persistSession(env, sessionID); err != nil && env.Act.Notify != nil {
			env.Act.Notify("保存浏览器状态失败: " + err.Error())
		}
	}
	return instance
}

func (p *Plugin) saveActive() {
	if p.currentKey == "" {
		return
	}
	instance := p.instances[p.currentKey]
	if instance == nil {
		return
	}
	instance.nav = p.nav
	instance.page = p.page
	instance.openErr = p.openErr
	instance.failure = p.failure
	instance.addressDraft = p.address.Text()
}

func (p *Plugin) restoreSession(sessionID, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var envelope browserStoreEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return err
	}
	saved, ok := envelope.BySession[sessionID]
	if !ok {
		return nil
	}
	restored := map[string]*browserInstance{}
	for tabID, state := range saved.ByTab {
		if tabID == "" {
			return fmt.Errorf("空 tabId")
		}
		if err := validateBrowserTabState(state); err != nil {
			return fmt.Errorf("tab %s: %w", tabID, err)
		}
		target, _ := currentTarget(state)
		restored[browserInstanceKey(sessionID, tabID)] = &browserInstance{
			nav: browserNavigation{value: state}, addressDraft: target.URL,
			failure: browserFailureReason(state.Failure),
		}
	}
	if p.instances == nil {
		p.instances = map[string]*browserInstance{}
	}
	for key, instance := range restored {
		p.instances[key] = instance
	}
	return nil
}

// mergeRestoredSession 在配置晚于首次用户动作到达时，只补回内存里不存在的
// occurrence；当前新开的 tab 不被旧配置覆盖，旧 tab 也不会在下次保存时消失。
func (p *Plugin) mergeRestoredSession(sessionID, raw string) error {
	saved := &Plugin{}
	if err := saved.restoreSession(sessionID, raw); err != nil {
		return err
	}
	for key, instance := range saved.instances {
		if _, exists := p.instances[key]; !exists {
			p.instances[key] = instance
		}
	}
	return nil
}

func (p *Plugin) persistActive(env *pluginapi.Env) {
	p.saveActive()
	sessionID, _ := browserEnvIdentity(env)
	if err := p.persistSession(env, sessionID); err != nil && env != nil && env.Act.Notify != nil {
		env.Act.Notify("保存浏览器状态失败: " + err.Error())
	}
}

func (p *Plugin) persistSession(env *pluginapi.Env, sessionID string) error {
	if p.dirtySessions == nil {
		p.dirtySessions = map[string]bool{}
	}
	p.dirtySessions[sessionID] = true
	// nil map = 配置尚未读到，不是空配置。先只留内存 dirty，等 activate
	// 读到旧值并合并后再写，避免首次导航覆盖其他已保存 occurrence。
	if env != nil && env.Act.ConfigGet != nil && !p.loadedSessions[sessionID] {
		return nil
	}
	byTab := map[string]browserTabState{}
	prefix := sessionID + "\x00"
	for key, instance := range p.instances {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		state := instance.nav.value
		state.Failure = browserFailureFor(instance.failure)
		byTab[strings.TrimPrefix(key, prefix)] = state
	}
	raw, err := json.Marshal(browserStoreEnvelope{BySession: map[string]browserSessionStore{
		sessionID: {ByTab: byTab},
	}})
	if err != nil {
		return err
	}
	if env != nil && env.Act.ConfigSet != nil {
		env.Act.ConfigSet(map[string]string{browserConfigKey(sessionID): string(raw)})
	}
	return nil
}

// RightTabTitle 实现官方 BrowserTitle：当前 hostname 优先，否则类型名。
func (p *Plugin) RightTabTitle(sessionID, tabID string) string {
	key := browserInstanceKey(sessionID, tabID)
	if key == p.currentKey {
		if target, ok := currentTarget(p.nav.value); ok {
			return target.Title
		}
		return browserTypeLabel
	}
	if instance := p.instances[key]; instance != nil {
		if target, ok := currentTarget(instance.nav.value); ok {
			return target.Title
		}
		return browserTypeLabel
	}
	if tabID == RightTabBrowser {
		return browserTypeLabel
	}
	// 不是本插件拥有的 occurrence：空串让壳保留该类型自己的标题。
	return ""
}

// CloseRightTab 实现官方 close handler：先关页面、forget(tabId)，再让壳删布局。
func (p *Plugin) CloseRightTab(sessionID, tabID, kind string, actions pluginapi.Actions) error {
	if kind != RightTabBrowser {
		return nil
	}
	if p.loadedSessions == nil {
		p.loadedSessions = map[string]bool{}
	}
	if !p.loadedSessions[sessionID] && actions.ConfigGet != nil {
		cfg := actions.ConfigGet()
		if cfg == nil {
			return fmt.Errorf("浏览器配置尚未加载，请稍后重试")
		}
		p.loadedSessions[sessionID] = true
		p.saveActive()
		var err error
		if p.dirtySessions[sessionID] {
			err = p.mergeRestoredSession(sessionID, cfg[browserConfigKey(sessionID)])
		} else {
			err = p.restoreSession(sessionID, cfg[browserConfigKey(sessionID)])
		}
		if err != nil {
			return fmt.Errorf("读取浏览器保存状态: %w", err)
		}
	}
	key := browserInstanceKey(sessionID, tabID)
	if key == p.currentKey {
		p.saveActive()
	}
	instance := p.instances[key]
	if instance != nil && instance.page != nil {
		instance.page.Close()
	}
	delete(p.instances, key)
	if key == p.currentKey {
		p.currentKey = ""
		p.nav = newBrowserNavigation()
		p.page = nil
		p.openErr = nil
		p.failure = ""
		p.address.SetText("")
	}
	env := &pluginapi.Env{Act: actions}
	env.Data.Cur = sessionID
	env.RightTabID = tabID
	return p.persistSession(env, sessionID)
}

// dispose 是插件卸载，不是关闭 tab：释放原生页面，但保留恢复记录。
func (p *Plugin) dispose() {
	p.saveActive()
	for _, instance := range p.instances {
		if instance.page != nil {
			instance.page.Close()
			instance.page = nil
		}
	}
	p.page = nil
}

func browserFailureFor(reason addressFailure) *browserFailure {
	if reason == "" {
		return nil
	}
	return &browserFailure{Kind: "address", Reason: reason}
}

func browserFailureReason(failure *browserFailure) addressFailure {
	if failure == nil || failure.Kind != "address" {
		return ""
	}
	return failure.Reason
}

func validateBrowserTabState(state browserTabState) error {
	if len(state.Entries) > maxBrowserHistory || state.Index < -1 || state.Index >= len(state.Entries) {
		return fmt.Errorf("历史索引或条数无效")
	}
	if state.Index == -1 && len(state.Entries) != 0 || state.Index >= 0 && len(state.Entries) == 0 {
		return fmt.Errorf("历史索引与 entries 不一致")
	}
	for _, entry := range state.Entries {
		parsed := parseBrowserAddress(entry.URL, "")
		if !parsed.OK || parsed.Target.URL != entry.URL {
			return fmt.Errorf("无效地址 %q", entry.URL)
		}
	}
	if state.Request != nil {
		if state.Request.Revision < 0 {
			return fmt.Errorf("负 revision")
		}
		parsed := parseBrowserAddress(state.Request.Target.URL, "")
		if !parsed.OK {
			return fmt.Errorf("无效 request 地址")
		}
	}
	if state.Navigation.State > navUnknown || state.Navigation.Revision < 0 {
		return fmt.Errorf("无效 navigation 状态")
	}
	if failure := state.Failure; failure != nil {
		if failure.Kind != "address" || addressFailureCopy(failure.Reason) == "" {
			return fmt.Errorf("无效 failure")
		}
	}
	return nil
}

func (state navState) MarshalJSON() ([]byte, error) {
	names := [...]string{"empty", "loading", "known", "unknown"}
	if state > navUnknown {
		return nil, fmt.Errorf("未知 navState %d", state)
	}
	return json.Marshal(names[state])
}

func (state *navState) UnmarshalJSON(raw []byte) error {
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return err
	}
	for i, candidate := range [...]string{"empty", "loading", "known", "unknown"} {
		if name == candidate {
			*state = navState(i)
			return nil
		}
	}
	return fmt.Errorf("未知 navigation.status %q", name)
}

func (kind targetKind) MarshalJSON() ([]byte, error) {
	if kind == kindHTTPS {
		return json.Marshal("https")
	}
	if kind == kindHTTP {
		return json.Marshal("http")
	}
	return nil, fmt.Errorf("未知 target kind %d", kind)
}

func (kind *targetKind) UnmarshalJSON(raw []byte) error {
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return err
	}
	switch name {
	case "https":
		*kind = kindHTTPS
	case "http":
		*kind = kindHTTP
	default:
		return fmt.Errorf("未知 target kind %q", name)
	}
	return nil
}
