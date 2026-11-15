// web_provider_wiring.go —— #47 `ui-settings-web-search` 的两条数据面接线：
// 官方 `web.listProviders` 与凭据域 `credentials.describe` / `credentials.set`。
//
// # 为什么是两条独立的路
//
// 那一页要三样东西，各来自不同来源：
//
//	provider 菜单   ← `web.listProviders`（OFF:dsh-web/src/index.ts:146-157）
//	设置字段值      ← `settings.describe`（既有那一路，见 settings_panel_wiring.go）
//	密钥"设没设"    ← `credentials.describe`（官方凭据域，值从不上线）
//
// 三者都不是彼此的一部分，所以缓存也各归各的。把它塞进 settings 那一路会让
// `settings.describe` 的失败连带 provider 菜单一起空掉，而官方它们是三条独立的
// 失效路径（`providers-changed` / `connection/reset` / `credentials/reference-updated`）。
//
// # 缓存策略（与 SettingsDescribe / PluginInventory / AgentPresetRoster 同一套）
//
//   - `WebProviderList` / `CredentialStatus` **同步返回缓存**：插件在 draw 里调，
//     等 HTTP 会卡首帧；
//   - 首次问触发一次后台拉取（按 ref 粒度防重入：一次只问没问到且没在飞的那些）；
//   - 拉失败**保留上一次**（别把已看到的清单/凭据态清空），原因写状态栏；
//   - 拉成功后 `Invalidate`，插件下一帧读到新值。
//
// # 为什么"没接线"与"没拉到"必须分开
//
// 宿主没挂 `OfficialWebProviderAPI` / 凭据后端时回 501 `not_implemented`：那是
// **没有这条能力**。界面据此不画菜单项/不给密钥控件，而不是画一个"空清单"——
// 后者是在说"宿主查过了，一个 provider 都没注册"，那是另一句话（官方
// `listProviders` 的可用性也**不过滤**清单，见 OFF:index.ts:146-157）。
package main

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// webProviderTimeout 是一次 provider 清单读的上限（与设置面同一档）。
const webProviderTimeout = 8 * time.Second

// credentialTimeout 是一次凭据状态读/写的上限。
const credentialTimeout = 8 * time.Second

// refreshWebProviders 重新拉一次 provider 清单并发布到缓存。
func (m *model) refreshWebProviders() {
	if m.cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), webProviderTimeout)
	defer cancel()
	list, err := m.cli.WebListProviders(ctx)

	m.mu.Lock()
	m.webProvidersPending = false
	if err != nil {
		// 保留上一次（可能是 nil = 还没成功过），只报状态栏。
		m.webProvidersErr = err
		m.mu.Unlock()
		m.setStatus("读 web provider 清单失败: " + err.Error())
		return
	}
	m.webProviders = &list
	m.webProvidersErr = nil
	m.mu.Unlock()
	m.inv.Invalidate()
}

// webProviderListSnapshot 是给插件用的同步快照（首次问触发后台拉取）。
func (m *model) webProviderListSnapshot() *apiclient.WebProviderList {
	m.mu.Lock()
	list := m.webProviders
	pending := m.webProvidersPending
	m.mu.Unlock()
	if !pending && list == nil && m.cli != nil {
		m.mu.Lock()
		m.webProvidersPending = true
		m.mu.Unlock()
		go m.refreshWebProviders()
	}
	return list
}

// webProviderRefresh 是页内「重试」与"provider 变了，重读"的显式入口。
func (m *model) webProviderRefresh() {
	m.mu.Lock()
	if m.webProvidersPending {
		m.mu.Unlock()
		return // 已经有一次在飞，别叠请求
	}
	m.webProvidersPending = true
	m.mu.Unlock()
	go m.refreshWebProviders()
}

// refreshCredentialRefs 问一批 ref 的凭据状态并并入缓存。
//
// 逐 ref 合并而不是整体替换：不同的页可能问不同的 ref，别让后一次把前一次
// 的答案抹掉（官方 `credentials.describe` 也是按 ref 一问一答的）。
func (m *model) refreshCredentialRefs(refs []string) {
	if m.cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), credentialTimeout)
	defer cancel()
	views, err := m.cli.CredentialDescribe(ctx, refs)

	m.mu.Lock()
	for _, ref := range refs {
		delete(m.credentialPending, ref)
	}
	if err != nil {
		m.credentialErr = err
		m.mu.Unlock()
		m.setStatus("读凭据状态失败: " + err.Error())
		return
	}
	if m.credentials == nil {
		m.credentials = map[string]apiclient.CredentialInfo{}
	}
	for ref, info := range views {
		m.credentials[ref] = info
	}
	m.credentialErr = nil
	m.mu.Unlock()
	m.inv.Invalidate()
}

// credentialStatusSnapshot 是给插件用的同步快照：只回**真的问到过**的 ref。
//
// 没问到的 ref 会在这里排一次后台读（按 ref 防重入），本帧不回它——
// 缺一个键 = "不知道"，插件那边画中性说明，不许当"未配置"。
func (m *model) credentialStatusSnapshot(refs []string) map[string]apiclient.CredentialInfo {
	m.mu.Lock()
	if m.credentialPending == nil {
		m.credentialPending = map[string]bool{}
	}
	out := make(map[string]apiclient.CredentialInfo, len(refs))
	var ask []string
	for _, ref := range refs {
		if info, ok := m.credentials[ref]; ok {
			out[ref] = info
			continue
		}
		if !m.credentialPending[ref] && m.cli != nil {
			m.credentialPending[ref] = true
			ask = append(ask, ref)
		}
	}
	m.mu.Unlock()
	if len(ask) > 0 {
		go m.refreshCredentialRefs(ask)
	}
	return out
}

// credentialSet 写一个凭据引用（官方 `credentials.set`），写成功后**重新读一次**
// 该 ref 的状态。
//
// 为什么必须再读：`credentials.set` 的官方结果是 void，不回"现在配好了没有"；
// 而环境的 `writable=false`、宿主的写入校验（空白/首尾空格/NUL）都只在
// 真实落库之后才知道。官方 `writeKey` 也是同样的形状
// （OFF:web-search-card-controller.ts 的 `writeKey` → `readCredential(ref)`）。
//
// **同步返回错误**：调用方在 goroutine 里调，把错误交给界面画 saveFailed。
func (m *model) credentialSet(ref, value string) error {
	if m.cli == nil {
		return errWebProviderNotWired
	}
	ctx, cancel := context.WithTimeout(context.Background(), credentialTimeout)
	defer cancel()
	if err := m.cli.CredentialSet(ctx, ref, value); err != nil {
		return err
	}
	go m.refreshCredentialRefs([]string{ref})
	return nil
}

// errWebProviderNotWired 是壳没有这条能力时的错误（与宿主 501 同一句话）。
var errWebProviderNotWired = notWiredError("宿主未接 credentials.set")

// notWiredError 是"没有这条能力"的错误类型（不冒充宿主的四字段）。
type notWiredError string

func (e notWiredError) Error() string { return string(e) }
