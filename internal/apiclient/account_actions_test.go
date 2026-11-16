package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func accountTestMetadata() AccountClientMetadata {
	return AccountClientMetadata{Version: "0.2.2-go.3", Locale: "zh-CN", TimezoneOffsetSeconds: 28800}
}

func TestAccountProfileAndBalanceWireShape(t *testing.T) {
	bodies := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		bodies[r.URL.Path] = body
		switch r.URL.Path {
		case "/api/account.getProfile":
			_, _ = w.Write([]byte(`{"profile":{"status":"ready","value":{"id":"u1","name":"测试用户","contact":"u@example.com","avatarUrl":null},"source":"official"}}`))
		case "/api/account.getBalance":
			_, _ = w.Write([]byte(`{"balance":{"status":"ready","value":[{"currency":"CNY","balance":"12.345"}],"bonusWallets":[{"currency":"USD","balance":"0.25"}],"source":"official"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	cli := New(srv.URL)
	profile, err := cli.GetAccountProfile(context.Background(), accountTestMetadata())
	if err != nil || profile == nil || profile.Value == nil || profile.Value.Name == nil || *profile.Value.Name != "测试用户" {
		t.Fatalf("profile = %+v, err = %v", profile, err)
	}
	balance, err := cli.GetAccountBalance(context.Background(), accountTestMetadata())
	if err != nil || balance == nil || len(balance.Value) != 1 || balance.Value[0].Balance != "12.345" || len(balance.BonusWallets) != 1 {
		t.Fatalf("balance = %+v, err = %v", balance, err)
	}
	for _, path := range []string{"/api/account.getProfile", "/api/account.getBalance"} {
		client, ok := bodies[path]["client"].(map[string]any)
		if !ok || client["version"] != "0.2.2-go.3" || client["locale"] != "zh-CN" || client["timezoneOffsetSeconds"] != float64(28800) {
			t.Fatalf("%s client = %#v", path, bodies[path]["client"])
		}
		if len(bodies[path]) != 1 {
			t.Fatalf("%s 只能带 client，实际 %v", path, bodies[path])
		}
	}
}

func TestAccountProfileAndBalancePreserveNull(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/account.getProfile" {
			_, _ = w.Write([]byte(`{"profile":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"balance":null}`))
	}))
	defer srv.Close()
	cli := New(srv.URL)
	profile, err := cli.GetAccountProfile(context.Background(), accountTestMetadata())
	if err != nil || profile != nil {
		t.Fatalf("profile:null 必须保留 nil，得到 %+v, %v", profile, err)
	}
	balance, err := cli.GetAccountBalance(context.Background(), accountTestMetadata())
	if err != nil || balance != nil {
		t.Fatalf("balance:null 必须保留 nil，得到 %+v, %v", balance, err)
	}
}

func TestAccountAuthenticationActionsWireShape(t *testing.T) {
	bodies := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies[r.URL.Path] = body
		switch r.URL.Path {
		case "/api/account.startSignIn":
			_, _ = w.Write([]byte(`{"status":"signed-out","links":{"usageUrl":"https://platform.example/usage","topUpUrl":"https://platform.example/topup"},"attempt":{"id":"a1","phase":"waiting-browser","authorizeUrl":"https://platform.example/auth"}}`))
		case "/api/account.cancelSignIn":
			_, _ = w.Write([]byte(`{"status":"signed-out","links":{},"attempt":{"id":"a1","phase":"cancelled"}}`))
		case "/api/account.signOut":
			_, _ = w.Write([]byte(`{"status":"signed-out","links":{},"attempt":null}`))
		case "/api/account.hasRunningAccountTasks":
			_, _ = w.Write([]byte(`true`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	cli := New(srv.URL)
	started, err := cli.StartAccountSignIn(context.Background(), accountTestMetadata(), "http://127.0.0.1:7788", "desktop")
	if err != nil || started == nil || !started.ActiveSignIn() || started.Attempt.ID != "a1" {
		t.Fatalf("start = %+v, err = %v", started, err)
	}
	cancelled, err := cli.CancelAccountSignIn(context.Background(), "a1")
	if err != nil || cancelled == nil || cancelled.Attempt.Phase != "cancelled" {
		t.Fatalf("cancel = %+v, err = %v", cancelled, err)
	}
	signedOut, err := cli.SignOutAccount(context.Background(), accountTestMetadata())
	if err != nil || signedOut == nil || signedOut.SignedIn() {
		t.Fatalf("signOut = %+v, err = %v", signedOut, err)
	}
	running, err := cli.HasRunningAccountTasks(context.Background())
	if err != nil || !running {
		t.Fatalf("running = %v, err = %v", running, err)
	}
	wantStart := map[string]any{"callbackOrigin": "http://127.0.0.1:7788", "loginSource": "desktop"}
	for key, want := range wantStart {
		if bodies["/api/account.startSignIn"][key] != want {
			t.Fatalf("start %s = %#v, want %#v", key, bodies["/api/account.startSignIn"][key], want)
		}
	}
	if !reflect.DeepEqual(bodies["/api/account.cancelSignIn"], map[string]any{"attemptId": "a1"}) {
		t.Fatalf("cancel body = %v", bodies["/api/account.cancelSignIn"])
	}
	if len(bodies["/api/account.hasRunningAccountTasks"]) != 0 {
		t.Fatalf("runningTasks 入参应为空对象：%v", bodies["/api/account.hasRunningAccountTasks"])
	}
}
