// remote_e2e_test.go —— 数据面**真链路**：httptest 假宿主 → apiclient → 控制器 → 画面。
//
// 为什么要有这层：控制器单测用的 fakeRemote 只证明"控制器会用它"，
// 证明不了"它真的经 internal/apiclient 打到官方方法上"。这里起一个只会
// 认三个官方方法名的 httptest 服务，走真实 `*apiclient.Client`：
//
//	account.getNotifications / account.getNotificationContent / account.reportNotificationEvent
//
// 方法名或参数形状写错，这个测试立刻红——这正是"数据全部经 internal/apiclient 取"
// 的可执行证据。
package dsnotifications

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestE2EThroughApiclient(t *testing.T) {
	// 这两个切片由 httptest 的 handler goroutine 写、测试 goroutine 读，
	// 必须加锁（-race 实测会报 DATA RACE）。别用 atomic 拼切片——追加不是原子操作。
	var mu sync.Mutex
	var gotPaths []string
	var events []string
	pushPath := func(p string) {
		mu.Lock()
		defer mu.Unlock()
		gotPaths = append(gotPaths, p)
	}
	pushEvent := func(e string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, e)
	}
	snapshotEvents := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), events...)
	}
	snapshotPaths := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), gotPaths...)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushPath(r.URL.Path)
		switch r.URL.Path {
		case "/api/account.getNotifications":
			_, _ = w.Write([]byte(`{"batch":{"nextPollAfterSeconds":600,"notifications":[
				{"id":"n-e2e","createdAt":"2026-11-12T00:00:00Z","displayUntil":null,
				 "title":"端到端通知","body":"来自假宿主","actions":[]}]}}`))
		case "/api/account.reportNotificationEvent":
			var body struct {
				Event string `json:"event"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			pushEvent(body.Event)
			_, _ = w.Write([]byte(`true`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"unexpected method"}}`))
		}
	}))
	defer srv.Close()

	client := apiclient.New(srv.URL)
	p := newTestPlugin(client)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true})

	waitUntil(t, "经 apiclient 取到通知", func() bool {
		got := p.ctrl.Notices()
		return len(got) == 1 && got[0].Title == "端到端通知"
	})
	canvas := render(t, p)
	if !containsText(canvas, "来自假宿主") {
		t.Fatalf("画面里应有宿主给的正文；文本=%v", textValues(canvas))
	}

	p.ctrl.Dismiss("n-e2e")
	waitUntil(t, "dismissed 经 apiclient 上报", func() bool { return len(snapshotEvents()) == 1 })
	if got := snapshotEvents(); got[0] != apiclient.NotificationEventDismissed {
		t.Fatalf("上报事件应为 dismissed，得到 %q", got[0])
	}

	// 方法名逐条核对：写错一个字符这里就对不上。
	want := map[string]bool{
		"/api/account.getNotifications":        false,
		"/api/account.reportNotificationEvent": false,
	}
	for _, path := range snapshotPaths() {
		if _, ok := want[path]; ok {
			want[path] = true
		}
	}
	for path, seen := range want {
		if !seen {
			t.Fatalf("没有调到 %s（实际调用 %v）", path, gotPaths)
		}
	}
}
