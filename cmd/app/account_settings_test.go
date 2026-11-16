package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func waitAccountRefreshIdle(t *testing.T, m *model) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.wave3.mu.Lock()
		pending := m.wave3.accountPending
		m.wave3.mu.Unlock()
		if !pending {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("账号后台刷新未结束")
}

func TestAccountFetchFeedsRealProfileBalanceAndCredentialFact(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls[r.URL.Path]++
		mu.Unlock()
		switch r.URL.Path {
		case "/api/account.getState":
			_, _ = w.Write([]byte(`{"status":"credential-stored","links":{"usageUrl":"https://platform.example/usage","topUpUrl":"https://platform.example/top-up"},"attempt":null,"source":"official"}`))
		case "/api/account.getProfile":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["client"] == nil {
				t.Error("getProfile 缺 client")
			}
			_, _ = w.Write([]byte(`{"profile":{"status":"ready","value":{"id":"u1","name":"真实用户","contact":"real@example.com"}}}`))
		case "/api/account.getBalance":
			_, _ = w.Write([]byte(`{"balance":{"status":"ready","value":[{"currency":"CNY","balance":"8.01"}],"bonusWallets":[{"currency":"CNY","balance":"1.00"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}
	m.wave3.fetchAccount(m)
	snapshot := m.wave3.accountDetailsSnapshot()
	if snapshot.View == nil || !snapshot.View.SignedIn() || snapshot.Profile == nil || snapshot.Profile.Value == nil || *snapshot.Profile.Value.Name != "真实用户" {
		t.Fatalf("账号快照没有真值：%+v", snapshot)
	}
	if snapshot.Balance == nil || len(snapshot.Balance.Value) != 1 || snapshot.Balance.Value[0].Balance != "8.01" || len(snapshot.Balance.BonusWallets) != 1 {
		t.Fatalf("余额没有真值：%+v", snapshot.Balance)
	}
	if facts := m.settingsHostFacts(); !facts.CredentialStored {
		t.Fatal("credential-stored 必须驱动 CredentialStored=true")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/api/account.getState", "/api/account.getProfile", "/api/account.getBalance"} {
		if calls[path] != 1 {
			t.Fatalf("%s 调用次数 = %d", path, calls[path])
		}
	}
}

func TestAccountStartCancelAndRunningTasksUseHostTruth(t *testing.T) {
	var openedMu sync.Mutex
	var opened []string
	originalOpenURL := openURL
	openURL = func(destination string) error {
		openedMu.Lock()
		opened = append(opened, destination)
		openedMu.Unlock()
		return nil
	}
	defer func() { openURL = originalOpenURL }()
	var state atomic.Value
	state.Store(`{"status":"signed-out","links":{},"attempt":null}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/account.startSignIn":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["callbackOrigin"] == "" || body["loginSource"] != "desktop" || body["client"] == nil {
				t.Errorf("start 参数不完整：%v", body)
			}
			state.Store(`{"status":"signed-out","links":{},"attempt":{"id":"a1","phase":"waiting-browser","authorizeUrl":"https://platform.example/auth?client=dsh"}}`)
			_, _ = w.Write([]byte(state.Load().(string)))
		case "/api/account.cancelSignIn":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["attemptId"] != "a1" {
				t.Errorf("cancel attemptId = %q", body["attemptId"])
			}
			state.Store(`{"status":"signed-out","links":{},"attempt":{"id":"a1","phase":"cancelled"}}`)
			_, _ = w.Write([]byte(state.Load().(string)))
		case "/api/account.hasRunningAccountTasks":
			_, _ = w.Write([]byte(`true`))
		case "/api/account.getState":
			_, _ = w.Write([]byte(state.Load().(string)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}
	if err := m.accountStartSignIn(); err != nil {
		t.Fatalf("start 失败：%v", err)
	}
	snapshot := m.wave3.accountDetailsSnapshot()
	if !snapshot.LoginVisible || snapshot.View == nil || snapshot.View.Attempt == nil || snapshot.View.Attempt.ID != "a1" {
		t.Fatalf("start 后快照错误：%+v", snapshot)
	}
	openedMu.Lock()
	if len(opened) != 1 || !strings.Contains(opened[0], "client=dsh") || !strings.Contains(opened[0], "theme=dark") {
		t.Fatalf("授权页没有自动打开/携带 theme：%v", opened)
	}
	openedMu.Unlock()
	waitAccountRefreshIdle(t, m)
	if err := m.accountCancelSignIn("a1"); err != nil {
		t.Fatalf("cancel 失败：%v", err)
	}
	snapshot = m.wave3.accountDetailsSnapshot()
	if snapshot.LoginVisible || snapshot.View.Attempt.Phase != "cancelled" {
		t.Fatalf("cancel 后快照错误：%+v", snapshot)
	}
	waitAccountRefreshIdle(t, m)
	running, err := m.accountHasRunningTasks()
	if err != nil || !running {
		t.Fatalf("runningTasks = %v, %v", running, err)
	}
}

func TestAccountSignOutClearsDetailsAndCredentialFact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/account.signOut":
			_, _ = w.Write([]byte(`{"status":"signed-out","links":{},"attempt":null}`))
		case "/api/account.getState":
			_, _ = w.Write([]byte(`{"status":"signed-out","links":{},"attempt":null}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}
	m.wave3.applyAccountView(&apiclient.AccountView{Status: apiclient.AccountCredentialStored})
	m.wave3.mu.Lock()
	m.wave3.accountProfile = &apiclient.AccountProfile{Status: "ready"}
	m.wave3.accountBalance = &apiclient.AccountBalance{Status: "ready"}
	m.wave3.mu.Unlock()
	if err := m.accountSignOut(); err != nil {
		t.Fatalf("signOut 失败：%v", err)
	}
	snapshot := m.wave3.accountDetailsSnapshot()
	if snapshot.View == nil || snapshot.View.SignedIn() || snapshot.Profile != nil || snapshot.Balance != nil {
		t.Fatalf("登出后残留账号数据：%+v", snapshot)
	}
	if facts := m.settingsHostFacts(); facts.CredentialStored {
		t.Fatal("signed-out 后账号 section 必须注销")
	}
	waitAccountRefreshIdle(t, m)
}

func TestAccountStartFailureIsVisibleAndNotSignedIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"injected","hint":"","retryable":true}}`))
	}))
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}
	if err := m.accountStartSignIn(); err == nil {
		t.Fatal("宿主失败必须返回错误")
	}
	snapshot := m.wave3.accountDetailsSnapshot()
	if !snapshot.LoginVisible || !snapshot.LoginFailed || snapshot.View != nil && snapshot.View.SignedIn() {
		t.Fatalf("失败被隐藏或伪造登录：%+v", snapshot)
	}
}
