package main

// `Actions.UIState` 的壳侧行为：**同步读缓存 + 后台拉 + 异步写**。
//
// 判据（对的是 ui_state_wiring.go 的实现与 pluginapi 的约定）：
//  1. 第一次 Get 立刻回 (nil,false) 并且**只**触发一次后台拉（插件每帧都会问）；
//  2. 拉回来之后同一 key 直接命中缓存，不再发请求；
//  3. 宿主说"没有"时缓存住 found=false，也不反复问；
//  4. Set/Delete 立刻改缓存（下一帧读到的就是新值），并真的发出对应方法；
//  5. 没有 apiclient（无头/未连宿主）时安静退回内存，不发请求、不 panic。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// uiStateHost 是一个假宿主：记录收到的方法与请求体，按预设回答。
type uiStateHost struct {
	mu     sync.Mutex
	calls  []string
	bodies map[string]map[string]any
	found  bool
	value  string
	gets   int
}

func newUIStateHost(t *testing.T, found bool, value string) (*uiStateHost, *httptest.Server) {
	t.Helper()
	host := &uiStateHost{found: found, value: value, bodies: map[string]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[len("/api/"):]
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		host.mu.Lock()
		host.calls = append(host.calls, method)
		host.bodies[method] = body
		if method == "uiState.get" {
			host.gets++
		}
		found, value := host.found, host.value
		host.mu.Unlock()
		switch method {
		case "uiState.get":
			if found {
				_, _ = w.Write([]byte(`{"result":{"found":true,"value":` + value + `}}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":{"found":false}}`))
		case "uiState.set":
			_, _ = w.Write([]byte(`{"result":{"key":"k","bytes":0}}`))
		case "uiState.delete":
			_, _ = w.Write([]byte(`{"result":{"deleted":true}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return host, srv
}

func (h *uiStateHost) count(method string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, call := range h.calls {
		if call == method {
			n++
		}
	}
	return n
}

// waitForUIState 等一个条件成立（后台拉是 goroutine，测试不能用固定 sleep 赌时序）。
func waitForUIState(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等不到：%s", what)
}

func TestUIStateGetLoadsOnceThenHitsCache(t *testing.T) {
	host, srv := newUIStateHost(t, true, `{"byTab":{"browser":{"url":"https://example.com"}}}`)
	m := &model{cli: apiclient.New(srv.URL)}
	key := `dsh.sidebar-browser.v1."sess-1"`

	// 第一次：立刻回 false，并触发一次后台拉。
	if value, found := m.uiStateGet(key); found || value != nil {
		t.Fatalf("首帧应是 (nil,false)，得到 (%s,%v)", value, found)
	}
	// 同一帧再问（插件每帧都问）不许再发一条。
	if _, found := m.uiStateGet(key); found {
		t.Fatal("首帧第二次问不该有值")
	}
	// 用 uiStateGet 自己探（它拿锁）：直接读 m.uiState.entries 是数据竞争，
	// -race 下会红（这条测试自己踩过）。
	waitForUIState(t, "后台拉回基线", func() bool {
		if _, found := m.uiStateGet(key); found {
			return true
		}
		return false
	})
	// 拿到之后直接命中缓存。
	value, found := m.uiStateGet(key)
	if !found || len(value) == 0 {
		t.Fatalf("拉到之后应命中缓存，得到 found=%v value=%s", found, value)
	}
	if host.count("uiState.get") != 1 {
		t.Fatalf("命中缓存后不该再发 get，实际 %d 次", host.count("uiState.get"))
	}
	// 发出去的参数只有 key。
	body := host.bodies["uiState.get"]
	if len(body) != 1 || body["key"] != key {
		t.Fatalf("裸参数不对：%v", body)
	}
}

func TestUIStateGetCachesAbsence(t *testing.T) {
	host, srv := newUIStateHost(t, false, "")
	m := &model{cli: apiclient.New(srv.URL)}
	key := `dsh.forge-viewed.v1."sess-1"`
	m.uiStateGet(key)
	waitForUIState(t, "后台拉回 found=false", func() bool {
		return host.count("uiState.get") == 1
	})
	for i := 0; i < 5; i++ {
		if _, found := m.uiStateGet(key); found {
			t.Fatal("宿主说没有时不该给值")
		}
	}
	if host.count("uiState.get") != 1 {
		t.Fatalf("没有也要缓存，不许每帧重问；实际 %d 次", host.count("uiState.get"))
	}
}

func TestUIStateSetAndDeleteUpdateCacheThenPersist(t *testing.T) {
	host, srv := newUIStateHost(t, false, "")
	m := &model{cli: apiclient.New(srv.URL)}
	key := `dsh.forge-viewed.v1."sess-1"`
	payload := json.RawMessage(`{"source":{"at":1,"files":{"a.go":"d1"}}}`)

	m.uiStateSet(key, payload)
	// 缓存立刻变（插件下一帧画的就是新值）。
	value, found := m.uiStateGet(key)
	if !found || string(value) != string(payload) {
		t.Fatalf("Set 之后缓存应立刻是新值，得到 found=%v value=%s", found, value)
	}
	waitForUIState(t, "set 发出去", func() bool { return host.count("uiState.set") == 1 })
	body := host.bodies["uiState.set"]
	if body["key"] != key {
		t.Fatalf("set 的 key 不对：%v", body)
	}
	if _, ok := body["value"].(map[string]any); !ok {
		t.Fatalf("value 必须是 JSON 值：%v", body["value"])
	}

	m.uiStateDelete(key)
	if _, found := m.uiStateGet(key); found {
		t.Fatal("Delete 之后缓存应立刻变没有")
	}
	waitForUIState(t, "delete 发出去", func() bool { return host.count("uiState.delete") == 1 })
	if host.bodies["uiState.delete"]["key"] != key {
		t.Fatalf("delete 的 key 不对：%v", host.bodies["uiState.delete"])
	}
}

func TestUIStateWithoutClientStaysInMemory(t *testing.T) {
	m := &model{}
	key := `dsh.forge-viewed.v1."sess-1"`
	if _, found := m.uiStateGet(key); found {
		t.Fatal("没有 client 时不该有值")
	}
	// Set 仍要更新内存（插件读到的是它自己刚写的），只是不落盘。
	m.uiStateSet(key, json.RawMessage(`{"a":1}`))
	if value, found := m.uiStateGet(key); !found || string(value) != `{"a":1}` {
		t.Fatalf("没有 client 时也要更新缓存，得到 found=%v value=%s", found, value)
	}
	m.uiStateDelete(key)
	if _, found := m.uiStateGet(key); found {
		t.Fatal("Delete 之后应为没有")
	}
}
