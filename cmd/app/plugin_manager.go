// plugin_manager.go —— 官方插件管理页（`dsh-client-ui-plugin-manager`）的壳侧
// 数据面：bundle 清单（`pluginManager.listBundles`）、bundle 文档
// （`readBundleDetail`）、以及两条真实写动作（`setBundleEnabled` / `removeBundle`）。
//
// # 为什么壳要有这一份
//
// 官方页面消费的是 `pluginManager.*` Remote；本仓 Transport 没有
// `plugin-manager/changed` 推送（官方 `index.ts` 的 `ctx.remote.$on` 那一族），
// 所以刷新边界与任务页同一条先例：**面板打开时拉、每个成功动作后重拉、失败给重试**。
//
// # 三条边界（与 schedule_catalog.go 同一套态度）
//
//  1. **不乐观更新**：启停/卸载都不改本地清单，等宿主回话后重拉。乐观地把一个
//     仍然装着的东西画成"已卸载"，是拿界面骗用户。
//  2. **501 不是错误**：宿主没挂接 pluginManager API 时回四字段 `not_implemented`。
//     官方文案是 `unavailable`（"本部署没有可管理的 profile"）——界面画它，
//     **不画重试**；重试一个不存在的方法永远同一个 501。所以这一位单独记。
//  3. **失败当场说一次**：动作 id 交给插件画官方那一句本地化文案，宿主原文写
//     状态栏（诊断用），不另编文案。
//
// # 为什么不带语言参数
//
// 官方 `readBundleDetail(name, language)` 的 language 是界面当前语言；本仓界面
// 只有中文，所以壳固定发 `zh-CN`（与 dssettings 的「内置插件」页同一口径）。
package main

import (
	"context"
	"log"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// pluginDetailLanguage 是 `readBundleDetail` 的语言标签（官方 BCP-47，宿主按
// `pmLanguagePattern` 校验）。界面只有中文，取 zh-CN。
const pluginDetailLanguage = "zh-CN"

// pluginCallTimeout 是一次 pluginManager 调用的上限（宿主动作可能真要落盘）。
const pluginCallTimeout = 10 * time.Second

// refreshPluginBundles 拉一次官方 bundle 清单。
//
// 失败**不清旧缓存**：把已经看到的插件突然清空，用户会以为"插件没了"；
// 只把失败原因放进 errText，让列表画错误行 + 重试。
func (m *model) refreshPluginBundles() {
	ctx, cancel := context.WithTimeout(context.Background(), pluginCallTimeout)
	defer cancel()
	bundles, err := m.cli.PluginBundles(ctx)

	m.mu.Lock()
	m.pluginBundlesPending = false
	switch {
	case err != nil:
		m.pluginBundlesUnavailable = apiclient.PluginManagerUnavailable(err)
		m.pluginBundlesErr = err.Error()
	default:
		m.pluginBundleList = bundles
		m.pluginBundlesLoaded = true
		m.pluginBundlesErr = ""
		m.pluginBundlesUnavailable = false
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// pluginBundles 是 `Actions.PluginBundles` 的实现：同步返回缓存，
// 首次问时触发一次后台拉取（与 ScheduleCatalog 同一模式，插件每帧都会问）。
func (m *model) pluginBundles() ([]apiclient.PluginBundle, bool, bool, string) {
	m.mu.Lock()
	bundles := m.pluginBundleList
	loaded := m.pluginBundlesLoaded
	unavailable := m.pluginBundlesUnavailable
	errText := m.pluginBundlesErr
	pending := m.pluginBundlesPending
	if !loaded && !pending {
		m.pluginBundlesPending = true
	}
	m.mu.Unlock()

	if !loaded && !pending {
		go m.refreshPluginBundles()
	}
	// 浅拷贝：缓存整体替换，不原地改元素。
	out := make([]apiclient.PluginBundle, len(bundles))
	copy(out, bundles)
	return out, loaded, unavailable, errText
}

// pluginBundlesRefresh 是 `Actions.PluginBundlesRefresh`：显式重拉（重试按钮）。
func (m *model) pluginBundlesRefresh() {
	m.mu.Lock()
	if m.pluginBundlesPending {
		m.mu.Unlock()
		return // 已经有一次在飞，别叠加
	}
	m.pluginBundlesPending = true
	m.mu.Unlock()
	go m.refreshPluginBundles()
}

// pluginBundleDetail 是 `Actions.PluginBundleDetail`：同步返回缓存，
// 首次问触发拉取（按 bundle 名分键）。
func (m *model) pluginBundleDetail(name string) (apiclient.PluginBundleDetail, bool, string) {
	m.mu.Lock()
	if m.pluginDetail == nil {
		m.pluginDetail = map[string]*apiclient.PluginBundleDetail{}
	}
	if m.pluginDetailLoaded == nil {
		m.pluginDetailLoaded = map[string]bool{}
	}
	if m.pluginDetailPending == nil {
		m.pluginDetailPending = map[string]bool{}
	}
	detail := m.pluginDetail[name]
	loaded := m.pluginDetailLoaded[name]
	errText := m.pluginDetailErr[name]
	pending := m.pluginDetailPending[name]
	if !loaded && !pending && errText == "" {
		m.pluginDetailPending[name] = true
	}
	m.mu.Unlock()

	if !loaded && !pending && errText == "" {
		go m.refreshPluginBundleDetail(name)
	}
	if detail == nil {
		return apiclient.PluginBundleDetail{}, loaded, errText
	}
	return *detail, loaded, errText
}

// pluginBundleDetailRefresh 是 `Actions.PluginBundleDetailRefresh`：重读一份文档。
func (m *model) pluginBundleDetailRefresh(name string) {
	m.mu.Lock()
	if m.pluginDetailPending == nil {
		m.pluginDetailPending = map[string]bool{}
	}
	if m.pluginDetailPending[name] {
		m.mu.Unlock()
		return
	}
	m.pluginDetailPending[name] = true
	m.mu.Unlock()
	go m.refreshPluginBundleDetail(name)
}

// refreshPluginBundleDetail 读一个 bundle 的 README。
//
// 失败同样**不清旧值**（成功的文档保持可见），只记 errText 让页面画重试。
func (m *model) refreshPluginBundleDetail(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), pluginCallTimeout)
	defer cancel()
	detail, err := m.cli.PluginReadBundleDetail(ctx, name, pluginDetailLanguage)

	m.mu.Lock()
	if m.pluginDetail == nil {
		m.pluginDetail = map[string]*apiclient.PluginBundleDetail{}
	}
	if m.pluginDetailLoaded == nil {
		m.pluginDetailLoaded = map[string]bool{}
	}
	m.pluginDetailPending[name] = false
	if err != nil {
		m.pluginDetailErr[name] = err.Error()
	} else {
		m.pluginDetail[name] = &detail
		m.pluginDetailLoaded[name] = true
		m.pluginDetailErr[name] = ""
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// pluginSetBundleEnabled 是 `Actions.PluginSetBundleEnabled`：真实启停 + 重拉。
func (m *model) pluginSetBundleEnabled(name string, enabled bool) {
	ctx, cancel := context.WithTimeout(context.Background(), pluginCallTimeout)
	defer cancel()
	result, err := m.cli.PluginSetBundleEnabled(ctx, name, enabled)
	if err == nil && result.Failed() {
		// 管理性失败（application=failed）走 result：把它折成动作失败，
		// 与 HTTP 错误同一条显示路径（宿主原文进状态栏）。
		err = &pluginChangeError{code: result.ErrorCode(), diagnostic: result.ErrorDiagnostic()}
	}
	if err != nil {
		action := "disable"
		if enabled {
			action = "enable"
		}
		m.markPluginActionFailed(action, name, err)
	} else {
		m.refreshPluginBundles()
	}
	m.inv.Invalidate()
}

// pluginRemoveBundle 是 `Actions.PluginRemoveBundle`：真实卸载 + 重拉。
func (m *model) pluginRemoveBundle(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), pluginCallTimeout)
	defer cancel()
	result, err := m.cli.PluginRemoveBundle(ctx, name)
	if err == nil && result.Failed() {
		err = &pluginChangeError{code: result.ErrorCode(), diagnostic: result.ErrorDiagnostic()}
	}
	if err != nil {
		m.markPluginActionFailed("uninstall", name, err)
	} else {
		// 卸载后详情缓存作废：那个 bundle 可能已经不在清单里。
		m.mu.Lock()
		delete(m.pluginDetail, name)
		delete(m.pluginDetailLoaded, name)
		delete(m.pluginDetailErr, name)
		m.mu.Unlock()
		m.refreshPluginBundles()
	}
	m.inv.Invalidate()
}

// markPluginActionFailed 记下一次用户动作失败（动作 id + 状态栏原文）。
func (m *model) markPluginActionFailed(action, name string, err error) {
	if err != nil {
		log.Printf("pluginManager %s %s 失败: %v", action, name, err)
	}
	m.mu.Lock()
	m.pluginActionFail = action
	m.pluginActionFailName = name
	m.mu.Unlock()
}

// pluginActionError 是 `Actions.PluginActionError`：取走即清空。
func (m *model) pluginActionError() (string, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pluginActionFail == "" {
		return "", "", false
	}
	action, name := m.pluginActionFail, m.pluginActionFailName
	m.pluginActionFail, m.pluginActionFailName = "", ""
	return action, name, true
}

// pluginChangeError 把官方 `ChangeResult.application === 'failed'` 折成 error，
// 让管理性失败与 HTTP 错误共用同一条"状态栏原文 + 本地化文案"的显示路径。
type pluginChangeError struct {
	code       string
	diagnostic string
}

func (e *pluginChangeError) Error() string {
	if e.diagnostic != "" {
		return e.code + ": " + e.diagnostic
	}
	return e.code
}
