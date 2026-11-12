// settings_panel_wiring.go —— 设置面板（rc.2 官方四区）的数据面接线。
//
// # 为什么单独一个文件
//
// 设置面板要三样东西：`settings.describe` 的缓存、宿主事实、三条写路径。
// 它们都是「壳 ↔ 宿主」的胶水，与 main.go 里的窗口/装配逻辑无关；放在一起
// 让 main.go 的 Actions 字面量只多五行引用，改设置面时不必在一万行里找。
//
// # 缓存策略（与 ModelsList / JobsList 同一套）
//
//   - `SettingsDescribe` **同步返回缓存**：插件在 draw 里调，等 HTTP 会卡首帧；
//   - 首次问触发一次后台拉取；写成功后也重新拉一次（官方 §2.3：写后重读）；
//   - 拉失败**保留上一次**（别把界面清空成"未知"），原因写状态栏；
//   - 写路径是**同步**的（调用方在 goroutine 里），冲突原样回给插件渲染。
package main

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// settingsWriteTimeout 是一次设置写的上限（与 dssettings 的 panelWriteTimeout 对齐）。
const settingsWriteTimeout = 8 * time.Second

// refreshSettingsDescribe 重新拉一次设置描述符并发布到缓存。
func (m *model) refreshSettingsDescribe() {
	ctx, cancel := context.WithTimeout(context.Background(), settingsWriteTimeout)
	defer cancel()
	describe, err := m.cli.SettingsDescribe(ctx)

	m.mu.Lock()
	m.settingsDescribePending = false
	if err != nil {
		m.settingsDescribeErr = err
		m.mu.Unlock()
		// 拉失败保留上一次（可能是 nil = 未知），只报状态栏。
		m.setStatus("读设置失败: " + err.Error())
		return
	}
	m.settingsDescribe = &describe
	m.settingsDescribeErr = nil
	m.mu.Unlock()
	m.inv.Invalidate()
}

// settingsDescribeSnapshot 是给插件用的同步快照（首次问触发后台拉取）。
func (m *model) settingsDescribeSnapshot() *apiclient.SettingsDescribeResult {
	m.mu.Lock()
	describe := m.settingsDescribe
	pending := m.settingsDescribePending
	m.mu.Unlock()
	if !pending && describe == nil {
		m.mu.Lock()
		m.settingsDescribePending = true
		m.mu.Unlock()
		go m.refreshSettingsDescribe()
	}
	return describe
}

// settingsHostFacts 汇总这一帧的宿主事实（决定动态注册哪些 section/item）。
//
// 每一项都必须**来自真实来源**：宿主没提供的一律零值（= 不注册），
// 不许为了"界面上多一行"编一个 true。
func (m *model) settingsHostFacts() pluginapi.SettingsHostFacts {
	facts := pluginapi.SettingsHostFacts{
		// Go GUI 没有官方 Electron 桌面桥（那是 dshDesktop 协议，协议版本 1）。
		DesktopProtocolVersion: 0,
		// 右侧栏插件（ds-rightbar）存在：`sidebarRightTabs` 服务可用。
		SidebarRightTabs: true,
		// 宿主当前不服务 session-log-deepseek（没有上传通道）。
		SessionLogServed: false,
		// build 注入的版本号（brand.go 的 version，`-X main.version=…`）。
		ClientVersion: version,
		// 账号默认 Local（W0 架构裁定）：没有 credential-stored 状态。
		CredentialStored: false,
	}

	m.mu.Lock()
	describe := m.settingsDescribe
	uiConfig := uiConfig
	m.mu.Unlock()

	// 权限目录就绪 = 宿主 describe 里有 permission 命名空间。
	if describe != nil {
		if _, ok := describe.Namespace(apiclient.SettingsNSPermission); ok {
			facts.PermissionCatalog = true
		}
	}
	// transcriptView 是壳自己的配置（ui-chat.transcriptView）；空 = 未知。
	facts.TranscriptView = uiConfig["ui-chat.transcriptView"]
	return facts
}

// settingsFormSet 写一个字段（宿主 `settings.form.set`）。
func (m *model) settingsFormSet(ns string, path []string, value any, expectedRevision int) (apiclient.SettingsSectionView, error) {
	ctx, cancel := context.WithTimeout(context.Background(), settingsWriteTimeout)
	defer cancel()
	var rev *int
	if expectedRevision > 0 {
		rev = &expectedRevision
	}
	section, err := m.cli.SettingsFormSet(ctx, ns, path, value, rev)
	if err != nil {
		return apiclient.SettingsSectionView{}, err
	}
	// 写后重读（官方 §2.3）：下一次 describe 拿到新 revision。
	go m.refreshSettingsDescribe()
	return section, nil
}

// settingsFormReset 清一个字段 / 整个 namespace。
func (m *model) settingsFormReset(ns string, path []string, expectedRevision int) (apiclient.SettingsSectionView, error) {
	ctx, cancel := context.WithTimeout(context.Background(), settingsWriteTimeout)
	defer cancel()
	var rev *int
	if expectedRevision > 0 {
		rev = &expectedRevision
	}
	section, err := m.cli.SettingsFormReset(ctx, ns, path, rev)
	if err != nil {
		return apiclient.SettingsSectionView{}, err
	}
	go m.refreshSettingsDescribe()
	return section, nil
}

// settingsMutate 原子提交一批路径操作（retryPolicy 的嵌套路径走这条）。
func (m *model) settingsMutate(ns string, ops []apiclient.SettingsOp, expectedRevision int) (apiclient.SettingsNamespaceView, error) {
	ctx, cancel := context.WithTimeout(context.Background(), settingsWriteTimeout)
	defer cancel()
	var rev *int
	if expectedRevision > 0 {
		rev = &expectedRevision
	}
	view, err := m.cli.SettingsMutate(ctx, ns, ops, rev)
	if err != nil {
		return apiclient.SettingsNamespaceView{}, err
	}
	go m.refreshSettingsDescribe()
	return view, nil
}
