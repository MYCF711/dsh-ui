// account_settings.go —— 设置页与 AccountMenu 共享的账号动作接线。
// 所有动作都经真实 Transport；插件只消费快照并异步调用，不直接接触 Platform。
package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
)

func (m *model) accountClientMetadata() (apiclient.AccountClientMetadata, error) {
	if version == "" {
		return apiclient.AccountClientMetadata{}, fmt.Errorf("account: 当前构建没有 DSH_CLIENT_VERSION")
	}
	_, offset := time.Now().Zone()
	return apiclient.AccountClientMetadata{Version: version, Locale: "zh-CN", TimezoneOffsetSeconds: float64(offset)}, nil
}

func (m *model) accountCallbackOrigin() (string, error) {
	if m.cli == nil {
		return "", fmt.Errorf("account: Transport 未接线")
	}
	u, err := url.Parse(m.cli.Base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("account: Transport base URL 无有效 origin: %q", m.cli.Base)
	}
	return u.Scheme + "://" + u.Host, nil
}

// accountRefresh 合并重复刷新；真正读写由 fetchAccount 完成。
func (m *model) accountRefresh() {
	// 官方 cot mount 同时监听 session-expired/model-sign-in-required；账号快照
	// 重读与 provider 可用性失效共用这次触发，不传思考正文。
	m.cotTranslationRefresh()
	m.wave3.mu.Lock()
	if m.wave3.accountPending {
		m.wave3.mu.Unlock()
		return
	}
	m.wave3.accountPending = true
	m.wave3.mu.Unlock()
	go m.wave3.fetchAccount(m)
}

func (m *model) accountShowLogin(visible bool) {
	m.wave3.mu.Lock()
	m.wave3.accountLoginVisible = visible
	if visible {
		m.wave3.accountLoginFailed = false
	}
	m.wave3.mu.Unlock()
	m.inv.Invalidate()
}

// accountAuthorizeURL 保留宿主已有的全部 query，只覆盖官方 theme 参数。
func (m *model) accountAuthorizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("account: authorizeUrl 无效")
	}
	scheme := "dark"
	if m.theme != nil && m.theme.BgBase == dscore.Light().BgBase {
		scheme = "light"
	}
	q := u.Query()
	q.Set("theme", scheme)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (m *model) accountStartSignIn() error {
	m.wave3.accountOpMu.Lock()
	m.wave3.mu.Lock()
	m.wave3.accountLoginVisible = true
	m.wave3.accountLoginFailed = false
	m.wave3.mu.Unlock()
	m.inv.Invalidate()

	meta, err := m.accountClientMetadata()
	if err == nil {
		var origin string
		origin, err = m.accountCallbackOrigin()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), settingsWriteTimeout)
			var view *apiclient.AccountView
			view, err = m.cli.StartAccountSignIn(ctx, meta, origin, "desktop")
			cancel()
			if err == nil {
				m.wave3.applyAccountView(view)
				m.wave3.mu.Lock()
				m.wave3.accountLoginVisible = view == nil || !view.SignedIn()
				m.wave3.mu.Unlock()
				if view != nil && view.Attempt != nil && view.Attempt.AuthorizeURL != "" {
					if destination, urlErr := m.accountAuthorizeURL(view.Attempt.AuthorizeURL); urlErr == nil {
						// 打开失败由 openExternal 报状态栏；对话框仍保留复制链接的真实退路。
						_ = m.openExternal(destination)
					} else {
						m.setStatus("登录页地址无效: " + urlErr.Error())
					}
				}
			}
		}
	}
	if err != nil {
		m.wave3.mu.Lock()
		m.wave3.accountLoginFailed = true
		m.wave3.mu.Unlock()
	}
	m.wave3.accountOpMu.Unlock()
	m.inv.Invalidate()
	if err == nil {
		m.accountRefresh()
	}
	return err
}

func (m *model) accountCancelSignIn(attemptID string) error {
	if attemptID == "" {
		return fmt.Errorf("account: attemptId 为空")
	}
	if m.cli == nil {
		return fmt.Errorf("account: Transport 未接线")
	}
	m.wave3.accountOpMu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), settingsWriteTimeout)
	view, err := m.cli.CancelAccountSignIn(ctx, attemptID)
	cancel()
	if err == nil {
		m.wave3.applyAccountView(view)
		m.wave3.mu.Lock()
		m.wave3.accountLoginVisible = false
		m.wave3.mu.Unlock()
	}
	m.wave3.accountOpMu.Unlock()
	m.inv.Invalidate()
	if err == nil {
		m.accountRefresh()
	}
	return err
}

func (m *model) accountSignOut() error {
	if m.cli == nil {
		return fmt.Errorf("account: Transport 未接线")
	}
	meta, err := m.accountClientMetadata()
	if err != nil {
		return err
	}
	m.wave3.accountOpMu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), settingsWriteTimeout)
	view, err := m.cli.SignOutAccount(ctx, meta)
	cancel()
	if err == nil {
		m.wave3.applyAccountView(view)
		m.wave3.mu.Lock()
		m.wave3.accountLoginVisible = false
		m.wave3.accountLoginFailed = false
		m.wave3.mu.Unlock()
	}
	m.wave3.accountOpMu.Unlock()
	m.inv.Invalidate()
	if err == nil {
		m.accountRefresh()
	}
	return err
}

func (m *model) accountHasRunningTasks() (bool, error) {
	if m.cli == nil {
		return false, fmt.Errorf("account: Transport 未接线")
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsWriteTimeout)
	defer cancel()
	return m.cli.HasRunningAccountTasks(ctx)
}
