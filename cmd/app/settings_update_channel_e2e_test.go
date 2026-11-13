// settings_update_channel_e2e_test.go —— 钉住「壳更新渠道探针 -> 面板行」这条真链路。
//
// 为什么需要它：截图看不到这一行（order 95 在面板很下面，窗口高度被屏幕限制）。
// 与其靠"我盯着截图找不到"下结论，不如把**链路**跑一遍断言出来：
//
//	desktopUpdate.channel.get 200  ->  service.shellChannelServed = true  ->  面板出现该行
//
// 三个方向都钉：能应答（出现）/ 501（不出现）/ channel=null（出现但显示「配置不可用」）。
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dssettings"
)

// channelProbeServer 造一个只回答壳渠道 + 空 describe 的宿主。
func channelProbeServer(t *testing.T, handled func(w http.ResponseWriter, method string) bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[len("/api/"):]
		if handled(w, method) {
			return
		}
		if method == "settings.describe" {
			_, _ = w.Write([]byte(`{"result":{"namespaces":[],"sections":[],"errors":[],"hasDocument":false}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"x","hint":"y","retryable":false}}`))
	}))
}

func newChannelProbeModel(t *testing.T, baseURL string) (*model, *dssettings.Plugin) {
	t.Helper()
	settings := &dssettings.Plugin{}
	kernel := uikernel.New("update-channel-e2e")
	// dssettings 的依赖是 dscore 的 ui.theme/ui.primitives（装配卡住是**正确的**约束）。
	if err := kernel.Load(&dscore.Plugin{Dark: true}, settings); err != nil {
		t.Fatalf("装 dssettings: %v", err)
	}
	return &model{cli: apiclient.New(baseURL), kernel: kernel}, settings
}

// 宿主能应答 -> 行必须出现在面板里，且显示宿主报的真值。
func TestUpdateChannelRowAppearsWhenHostServesIt(t *testing.T) {
	srv := channelProbeServer(t, func(w http.ResponseWriter, method string) bool {
		if method != "desktopUpdate.channel.get" {
			return false
		}
		_, _ = w.Write([]byte(`{"result":{"channel":"nightly","packagedChannel":"stable","changing":false}}`))
		return true
	})
	defer srv.Close()

	m, _ := newChannelProbeModel(t, srv.URL)
	m.refreshSettingsDescribe()

	facts := m.settingsHostFacts()
	if !facts.ShellChannelServed {
		t.Fatal("宿主应答了 desktopUpdate.channel.get，ShellChannelServed 必须是 true")
	}
	if facts.ShellChannel == nil || *facts.ShellChannel != "nightly" {
		t.Fatalf("事实位必须带上宿主报的渠道，实得 %v", facts.ShellChannel)
	}

	// 事实位这一段到这里就钉完了。「事实位 -> 面板行」那一段在 dssettings 包里
	// 钉（那边有 fromAPIHostFacts 的转换，跨包类型不通用）。
}

// 宿主回 501（没这条能力）-> 行**不出现**（不是出现一个「未知」）。
func TestUpdateChannelRowAbsentWhenHostReturns501(t *testing.T) {
	srv := channelProbeServer(t, func(w http.ResponseWriter, method string) bool {
		if method != "desktopUpdate.channel.get" {
			return false
		}
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"未接线","hint":"挂 Options.UpdateChannel","retryable":false}}`))
		return true
	})
	defer srv.Close()

	m, _ := newChannelProbeModel(t, srv.URL)
	m.refreshSettingsDescribe()

	facts := m.settingsHostFacts()
	if facts.ShellChannelServed {
		t.Fatal("501 = 宿主没这条能力，ShellChannelServed 必须是 false")
	}
}

// 宿主应答但 `channel: null`（用户还没选过）-> 行出现，显示「配置不可用」。
//
// 这条最容易被写错：把 null 当成 stable 会让界面**谎报**用户的选择。
func TestUpdateChannelRowShowsUnavailableOnNull(t *testing.T) {
	srv := channelProbeServer(t, func(w http.ResponseWriter, method string) bool {
		if method != "desktopUpdate.channel.get" {
			return false
		}
		_, _ = w.Write([]byte(`{"result":{"channel":null,"packagedChannel":"stable","changing":false}}`))
		return true
	})
	defer srv.Close()

	m, _ := newChannelProbeModel(t, srv.URL)
	m.refreshSettingsDescribe()

	facts := m.settingsHostFacts()
	if !facts.ShellChannelServed {
		t.Fatal("应答成功就说明这条能力在")
	}
	if facts.ShellChannel != nil {
		t.Fatalf("channel=null 时事实位必须是 nil，实得 %v", *facts.ShellChannel)
	}
}
