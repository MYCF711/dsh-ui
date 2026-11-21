package dsnotifications

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

func TestAccountMenuEntriesFollowSignedState(t *testing.T) {
	// contact 是否出现由壳有没有接 OpenExternal 决定（2026-11-21 起「意见反馈」已接线，
	// 见 contact.go）；这里两个开关都钉住，不放过「画了却点不动」。
	signedOut := accountMenuEntries(AccountState{SignedIn: false}, false, false)
	if len(signedOut) != 2 || signedOut[0].id != "settings" || signedOut[1].id != "signin" {
		t.Fatalf("登出菜单 = %+v", signedOut)
	}
	signedIn := accountMenuEntries(AccountState{SignedIn: true}, false, false)
	if len(signedIn) != 2 || signedIn[1].id != "signout" {
		t.Fatalf("登录菜单 = %+v", signedIn)
	}
	for _, entry := range signedIn {
		if entry.label == "意见反馈" || entry.label == "添加 API Key" {
			t.Fatalf("没接 OpenExternal 时不该画反馈行：%+v", entry)
		}
	}
	wired := accountMenuEntries(AccountState{SignedIn: true}, false, true)
	if len(wired) != 3 || wired[1].id != "contact" || wired[1].label != "意见反馈" {
		t.Fatalf("接了 OpenExternal 的菜单 = %+v", wired)
	}
}

func TestAccountMenuSignInRunsRealAction(t *testing.T) {
	p := newTestPlugin(nil)
	var called atomic.Int32
	env := &pluginapi.Env{Act: pluginapi.Actions{
		AccountStartSignIn: func() error { called.Add(1); return nil },
	}}
	p.chooseAccountMenu("signin", env)
	waitUntil(t, "登录动作", func() bool {
		_, _, _, _ = p.accountUIStateSnapshot()
		return called.Load() == 1
	})
}

func TestAccountMenuSignOutImpactAndResult(t *testing.T) {
	p := newTestPlugin(nil)
	env := &pluginapi.Env{Act: pluginapi.Actions{
		AccountHasRunningTasks: func() (bool, error) { return true, nil },
		AccountSignOut:         func() error { return nil },
	}}
	p.requestAccountSignOut(env)
	waitUntil(t, "运行任务确认", func() bool {
		_, _, impact, _ := p.accountUIStateSnapshot()
		return impact == "running"
	})
	p.confirmAccountSignOut(env)
	waitUntil(t, "退出登录", func() bool {
		_, _, impact, failed := p.accountUIStateSnapshot()
		return impact == "" && !failed
	})
}

func TestAccountMenuSignOutUnknownAndFailureStayVisible(t *testing.T) {
	p := newTestPlugin(nil)
	env := &pluginapi.Env{Act: pluginapi.Actions{
		AccountHasRunningTasks: func() (bool, error) { return false, errors.New("unknown") },
		AccountSignOut:         func() error { return errors.New("injected") },
	}}
	p.requestAccountSignOut(env)
	waitUntil(t, "任务未知确认", func() bool {
		_, _, impact, _ := p.accountUIStateSnapshot()
		return impact == "unknown"
	})
	p.confirmAccountSignOut(env)
	waitUntil(t, "登出失败可见", func() bool {
		_, _, impact, failed := p.accountUIStateSnapshot()
		return impact == "unknown" && failed
	})
}

func TestAccountMenuSettingsStillOpensRealPanel(t *testing.T) {
	p := newTestPlugin(nil)
	called := false
	p.settings = func() bool { called = true; return true }
	p.chooseAccountMenu("settings", &pluginapi.Env{})
	if !called {
		t.Fatal("设置菜单没有调用真实打开能力")
	}
}

func TestDismissSignInCancelsCurrentAttemptBeforeClosing(t *testing.T) {
	p := newTestPlugin(nil)
	var cancelled atomic.Value
	cancelled.Store("")
	var cancelCalls atomic.Int32
	var visible atomic.Bool
	visible.Store(true)
	release := make(chan struct{})
	env := &pluginapi.Env{Act: pluginapi.Actions{
		AccountCancelSignIn: func(id string) error {
			cancelCalls.Add(1)
			cancelled.Store(id)
			<-release
			return nil
		},
		AccountShowLogin: func(value bool) { visible.Store(value) },
	}}
	snapshot := apiclient.AccountSnapshot{
		View:         &apiclient.AccountView{Status: "signed-out", Attempt: &apiclient.AccountAttempt{ID: "a1", Phase: "waiting-browser"}},
		LoginVisible: true,
	}
	p.dismissAccountSignIn(env, snapshot)
	p.dismissAccountSignIn(env, snapshot) // busy 中的第二次关闭不得重复 cancel。
	close(release)
	waitUntil(t, "取消后关闭", func() bool { return cancelled.Load() == "a1" && !visible.Load() })
	if got := cancelCalls.Load(); got != 1 {
		t.Fatalf("busy 防重入失效：cancel 调用 %d 次", got)
	}
}
