// desktop_update_test.go —— 「桌面更新指示器」的数据面接线（壳 ↔ 宿主）行为测试。
//
// 六条判据，全部对着真 HTTP（httptest）+ 可注入的 opener 桩，**不真开浏览器**：
//
//	① status 200 available  -> Snapshot.DesktopUpdate 带上 phase/version；
//	② 首次 status 回 checking -> **再问一次**才拿到结论（宿主 status 不阻塞）；
//	③ 501（宿主没这条能力）-> 什么都不画（nil），**不是** failed；
//	④ open 200 -> 用宿主给的 URL 唤起系统默认浏览器（桩断言精确 URL）；
//	⑤ open 失败 -> 如实提示（状态栏）+ 按官方记 failed，**不静默失败**；
//	⑥ open 回来时把宿主那次（绕过 TTL 的）新结论收进缓存。
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// updateServer 起一个只回答桌面更新两族的宿主。
func updateServer(t *testing.T, handle func(w http.ResponseWriter, method string, calls int) bool) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[len("/api/"):]
		n := int(atomic.AddInt32(&calls, 1))
		if handle(w, method, n) {
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"x","hint":"y","retryable":false}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// waitForUpdate 等到条件成立（后台拉取是 goroutine，测试必须等它落地）。
func waitForUpdate(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等不到：%s", what)
}

// ① status 200 available -> 指示器拿到真值。
func TestDesktopUpdateSnapshotReflectsHostAvailable(t *testing.T) {
	srv, _ := updateServer(t, func(w http.ResponseWriter, method string, _ int) bool {
		if method != "desktopUpdate.status" {
			return false
		}
		_, _ = w.Write([]byte(`{"result":{"phase":"available","version":"0.2.2-go.3"}}`))
		return true
	})
	m := &model{cli: apiclient.New(srv.URL)}

	// 首帧同步返回 nil（还没问到），并在后台起一次拉取——插件在 draw 里调它，
	// 等 HTTP 会卡首帧。
	if got := m.desktopUpdateSnapshot(); got != nil {
		t.Fatalf("首帧还没问到就该是 nil，实得 %+v", got)
	}
	waitForUpdate(t, "status 结论落地", func() bool { return m.desktopUpdateSnapshot() != nil })

	got := m.desktopUpdateSnapshot()
	if got.Phase != "available" || got.Version != "0.2.2-go.3" {
		t.Fatalf("presentation = %+v，want available/0.2.2-go.3", got)
	}
	if got.Failed {
		t.Fatal("成功拿到结论时 failed 必须为假")
	}
}

// ② 宿主 `status()` 不阻塞：冷缓存回 checking，要**再问一次**才拿到结论。
//
// 这条是本单最容易写错的一处——把第一次的 checking 当答案，指示器就会永远
// 停在「正在检查更新…」上。
func TestDesktopUpdateSnapshotReasksWhileChecking(t *testing.T) {
	srv, calls := updateServer(t, func(w http.ResponseWriter, method string, n int) bool {
		if method != "desktopUpdate.status" {
			return false
		}
		if n == 1 {
			_, _ = w.Write([]byte(`{"result":{"phase":"checking"}}`))
			return true
		}
		_, _ = w.Write([]byte(`{"result":{"phase":"available","version":"0.2.2-go.3"}}`))
		return true
	})
	m := &model{cli: apiclient.New(srv.URL)}
	m.desktopUpdateSnapshot()
	waitForUpdate(t, "回问后拿到结论", func() bool {
		got := m.desktopUpdateSnapshot()
		return got != nil && got.Phase == "available"
	})
	if got := atomic.LoadInt32(calls); got != 2 {
		t.Fatalf("checking 之后应回问一次（共 2 次 status），实际 %d 次", got)
	}
}

// ③ 501 = 宿主没这条能力 = 官方**根本没有 updates 桥** -> 整枚不画。
//
// 把它画成「重试更新」是最容易犯的错：那是"有桥但坏了"，与"没有桥"是两件事。
func TestDesktopUpdateSnapshotTreats501AsNoBridge(t *testing.T) {
	srv, _ := updateServer(t, func(w http.ResponseWriter, method string, _ int) bool {
		if method != "desktopUpdate.status" {
			return false
		}
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"未接线","hint":"","retryable":false}}`))
		return true
	})
	m := &model{cli: apiclient.New(srv.URL)}
	m.desktopUpdateSnapshot()

	waitForUpdate(t, "501 结论落地", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return !m.desktopUpdatePending && m.desktopUpdateAt.After(time.Time{})
	})
	if got := m.desktopUpdateSnapshot(); got != nil {
		t.Fatalf("宿主没这条能力时不该有 presentation（官方无桥 = 不画），实得 %+v", got)
	}
	m.mu.Lock()
	failed := m.desktopUpdateFailed
	m.mu.Unlock()
	if failed {
		t.Fatal("501 不许写成 failed——那是编了一条「更新坏了」的事实")
	}
}

// ④ open 真调宿主，并用宿主给的 URL 唤起系统默认浏览器（桩断言，不真开浏览器）。
func TestDesktopUpdateOpenOpensSystemBrowser(t *testing.T) {
	const entry = "https://github.com/MYCF711/dsh-go/releases/tag/v0.2.2-go.3"
	srv, _ := updateServer(t, func(w http.ResponseWriter, method string, _ int) bool {
		if method != "desktopUpdate.open" {
			return false
		}
		_, _ = w.Write([]byte(`{"result":{"url":"` + entry + `","channel":"stable",` +
			`"presentation":{"phase":"available","version":"0.2.2-go.3"}}}`))
		return true
	})
	original := openURL
	defer func() { openURL = original }()
	opened := make(chan string, 4)
	openURL = func(destination string) error {
		opened <- destination
		return nil
	}

	m := &model{cli: apiclient.New(srv.URL)}
	m.desktopUpdateOpen()
	select {
	case got := <-opened:
		if got != entry {
			t.Fatalf("唤起浏览器的 URL = %q，want %q（宿主给的那一条）", got, entry)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("点了更新入口却没有唤起系统默认浏览器")
	}

	waitForUpdate(t, "opening 复位", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return !m.desktopUpdateOpening
	})
	// ⑥ open 的返回值（宿主绕过 TTL 真查的新结论）直接收下，不用再问一次。
	m.mu.Lock()
	got := m.desktopUpdate
	m.mu.Unlock()
	if got == nil || got.Phase != "available" || got.Version != "0.2.2-go.3" {
		t.Fatalf("open 的结果应直接进缓存，实得 %+v", got)
	}
}

// ⑤ open 失败：**不静默失败**——状态栏要说话，并记成官方 `view.failed`。
func TestDesktopUpdateOpenFailureIsReported(t *testing.T) {
	srv, _ := updateServer(t, func(w http.ResponseWriter, method string, _ int) bool {
		if method != "desktopUpdate.open" {
			return false
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"boom","hint":"","retryable":false}}`))
		return true
	})
	original := openURL
	defer func() { openURL = original }()
	opened := 0
	openURL = func(string) error { opened++; return nil }

	m := &model{cli: apiclient.New(srv.URL)}
	m.desktopUpdateOpen()
	waitForUpdate(t, "失败状态落地", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return !m.desktopUpdateOpening && m.desktopUpdateFailed
	})
	m.mu.Lock()
	status := m.status
	m.mu.Unlock()
	if !strings.Contains(status, "打开更新入口失败") {
		t.Fatalf("状态栏 = %q，应如实说「打开更新入口失败」", status)
	}
	if opened != 0 {
		t.Fatal("宿主没给出入口时不该去开浏览器")
	}
}

// ⑤b open 时宿主给空 URL：契约说不会，真给了就如实说，不假装打开了。
func TestDesktopUpdateOpenEmptyURLIsReported(t *testing.T) {
	srv, _ := updateServer(t, func(w http.ResponseWriter, method string, _ int) bool {
		if method != "desktopUpdate.open" {
			return false
		}
		_, _ = w.Write([]byte(`{"result":{"url":"","channel":"stable","presentation":{"phase":"idle"}}}`))
		return true
	})
	original := openURL
	defer func() { openURL = original }()
	opened := 0
	openURL = func(string) error { opened++; return nil }

	m := &model{cli: apiclient.New(srv.URL)}
	m.desktopUpdateOpen()
	waitForUpdate(t, "空 URL 状态落地", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return !m.desktopUpdateOpening && strings.Contains(m.status, "更新入口为空")
	})
	if opened != 0 {
		t.Fatal("空 URL 不该去唤起浏览器")
	}
}

// ④b 并发点击只发一次（官方 `DesktopUpdateSource.open()` 的 opening 守卫）。
func TestDesktopUpdateOpenIsNotReentrant(t *testing.T) {
	var opens int32
	srv, _ := updateServer(t, func(w http.ResponseWriter, method string, _ int) bool {
		if method != "desktopUpdate.open" {
			return false
		}
		atomic.AddInt32(&opens, 1)
		time.Sleep(60 * time.Millisecond)
		_, _ = w.Write([]byte(`{"result":{"url":"https://example.invalid/x","channel":"stable",` +
			`"presentation":{"phase":"idle"}}}`))
		return true
	})
	original := openURL
	defer func() { openURL = original }()
	opened := int32(0)
	openURL = func(string) error { atomic.AddInt32(&opened, 1); return nil }

	m := &model{cli: apiclient.New(srv.URL)}
	m.desktopUpdateOpen()
	m.desktopUpdateOpen()
	m.desktopUpdateOpen()
	waitForUpdate(t, "opening 复位", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return !m.desktopUpdateOpening
	})
	if got := atomic.LoadInt32(&opens); got != 1 {
		t.Fatalf("opening 期间重复点击应只发 1 次 open，实际 %d 次", got)
	}
	if got := atomic.LoadInt32(&opened); got != 1 {
		t.Fatalf("应只唤起 1 次浏览器，实际 %d 次", got)
	}
}
