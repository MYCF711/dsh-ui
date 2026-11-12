// controller_test.go —— 官方通知控制器语义（结构树 §5.4 的七条命令 + 四条保护）。
//
// 这些规则看着琐碎，但每一条都对应一个**用户能看见**的行为：
//
//	过期不过滤      → 过期的赠金通知永远挂在账号菜单上
//	失败清卡        → 网络抖一下，卡片消失再出现
//	间隔不跟服务端  → 平台要求 30s 后再说，我们继续 10 分钟一次
//	只有前景能上报  → 用户没看见的卡也被记成 displayed
//	displayed 报两次→ 平台侧展示计数翻倍
//	迟到回复发布    → 退出账号后卡片又冒出来
//	前景不保留      → 用户刚要点的卡在下一轮消失
package dsnotifications

import (
	"errors"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// noticeWithExpiry 造一条带 displayUntil 的通知。
func noticeWithExpiry(id string, displayUntil *string) apiclient.Notification {
	n := sampleNotice(id)
	n.DisplayUntil = displayUntil
	return n
}

func strptr(s string) *string { return &s }

// TestLiveRules 钉住官方 `live(displayUntil)` 三条规则。
func TestLiveRules(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		when *string
		want bool
	}{
		{"null 永不过期", nil, true},
		{"未来时间不过期", strptr("2026-11-12T12:00:01Z"), true},
		{"过去时间已过期", strptr("2026-11-12T11:59:59Z"), false},
		{"解析不出来的时间不算过期", strptr("not-a-timestamp"), true},
		{"空串也不算过期", strptr(""), true},
		{"正好到点算过期", strptr("2026-11-12T12:00:00Z"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := live(noticeWithExpiry("n", tc.when), now); got != tc.want {
				t.Fatalf("live(%v) = %v，应为 %v", tc.when, got, tc.want)
			}
		})
	}
}

// TestBeginFetchesImmediately：官方 begin() 立刻读一次（不等下一轮定时）。
func TestBeginFetchesImmediately(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "begin 后立刻有一次读", func() bool { return remote.fetchCount() == 1 })
	waitUntil(t, "通知已发布", func() bool { return len(c.Notices()) == 1 })
}

// TestUpdateIsIdempotent：重复 Update(signedIn) 不会重复 begin（幂等）。
func TestUpdateIsIdempotent(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{NextPollAfterSeconds: 600}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "第一次读", func() bool { return remote.fetchCount() == 1 })
	for i := 0; i < 5; i++ {
		c.Update(AccountState{Active: true, SignedIn: true})
	}
	if got := remote.fetchCount(); got != 1 {
		t.Fatalf("重复 Update 不该重复读，实际读了 %d 次", got)
	}
}

// TestEndStopsPolling：退出账号后不再有新的读。
func TestEndStopsPolling(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{NextPollAfterSeconds: 600}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "第一次读", func() bool { return remote.fetchCount() == 1 })
	c.Update(AccountState{Active: true, SignedIn: false})
	before := remote.fetchCount()
	time.Sleep(15 * time.Millisecond)
	if got := remote.fetchCount(); got != before {
		t.Fatalf("end 之后不该再读，前 %d 后 %d", before, got)
	}
}

// TestIntervalFollowsServer：成功后用 nextPollAfterSeconds 覆盖间隔（官方"表在请求 settle 后才起"）。
func TestIntervalFollowsServer(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 42.5,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "第一次读", func() bool { return remote.fetchCount() == 1 })
	c.mu.Lock()
	interval := c.interval
	c.mu.Unlock()
	if interval != 42500*time.Millisecond {
		t.Fatalf("间隔应来自服务端 42.5s，得到 %v", interval)
	}
}

// TestFailureKeepsCardsAndInterval：读失败**保留卡片**、沿用旧间隔（官方 §5.4）。
func TestFailureKeepsCardsAndInterval(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "第一张卡", func() bool { return len(c.Notices()) == 1 })

	remote.setErr(errors.New("network down"))
	c.Refresh()
	waitUntil(t, "第二次读（失败）", func() bool { return remote.fetchCount() >= 2 })
	if got := len(c.Notices()); got != 1 {
		t.Fatalf("读失败必须保留已有卡片，得到 %d 张", got)
	}
	c.mu.Lock()
	interval := c.interval
	c.mu.Unlock()
	if interval != 600*time.Second {
		t.Fatalf("失败时沿用旧间隔（600s），得到 %v", interval)
	}
}

// TestNullBatchClears：服务端 null（未登录/授权在查询中变了）→ 清空卡片。
func TestNullBatchClears(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "第一张卡", func() bool { return len(c.Notices()) == 1 })

	remote.setBatch(nil)
	c.Refresh()
	waitUntil(t, "第二次读", func() bool { return remote.fetchCount() >= 2 })
	waitUntil(t, "null 清空卡片", func() bool { return len(c.Notices()) == 0 })
}

// TestOnlyForegroundReports：非前景卡的交互不算数（官方校验 current[0].id）。
func TestOnlyForegroundReports(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications: []apiclient.Notification{
			sampleNotice("n-front"), sampleNotice("n-back"),
		},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "两张卡", func() bool { return len(c.Notices()) == 2 })

	c.Shown("n-back")
	c.CTAClicked("n-back")
	c.Dismiss("n-back")
	time.Sleep(10 * time.Millisecond)
	if reports := remote.reportList(); len(reports) != 0 {
		t.Fatalf("非前景卡不该上报任何事件，实际 %+v", reports)
	}
	if got := len(c.Notices()); got != 2 {
		t.Fatalf("非前景卡的 Dismiss 不该移除卡片，剩 %d 张", got)
	}
}

// TestShownReportsOnce：一次挂载只报一次 displayed（官方 reported ref）。
func TestShownReportsOnce(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "一张卡", func() bool { return len(c.Notices()) == 1 })

	for i := 0; i < 3; i++ {
		c.Shown("n-1")
	}
	waitUntil(t, "上报到达", func() bool { return len(remote.reportList()) >= 1 })
	time.Sleep(10 * time.Millisecond)
	reports := remote.reportList()
	if len(reports) != 1 || reports[0].event != apiclient.NotificationEventDisplayed {
		t.Fatalf("displayed 只该报一次，实际 %+v", reports)
	}
}

// TestCTAClickedKeepsCard：官方 CTA 打开账号设置后**卡片保留**。
func TestCTAClickedKeepsCard(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "一张卡", func() bool { return len(c.Notices()) == 1 })

	c.CTAClicked("n-1")
	waitUntil(t, "cta_clicked 到达", func() bool { return len(remote.reportList()) == 1 })
	if remote.reportList()[0].event != apiclient.NotificationEventCTAClicked {
		t.Fatalf("事件应为 cta_clicked，得到 %q", remote.reportList()[0].event)
	}
	if got := len(c.Notices()); got != 1 {
		t.Fatalf("CTA 不该关卡片，剩 %d 张", got)
	}
}

// TestDismissRemovesAndReports：关闭 = 本地立刻移除 + 上报 dismissed。
func TestDismissRemovesAndReports(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "一张卡", func() bool { return len(c.Notices()) == 1 })

	c.Dismiss("n-1")
	if got := len(c.Notices()); got != 0 {
		t.Fatalf("关闭后卡片必须立刻消失，剩 %d 张", got)
	}
	waitUntil(t, "dismissed 到达", func() bool { return len(remote.reportList()) == 1 })
	if remote.reportList()[0].event != apiclient.NotificationEventDismissed {
		t.Fatalf("事件应为 dismissed，得到 %q", remote.reportList()[0].event)
	}
}

// TestForegroundRetention：已上报 displayed 的前景卡，即便新列表省略也保留。
func TestForegroundRetention(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "一张卡", func() bool { return len(c.Notices()) == 1 })
	c.Shown("n-1")
	waitUntil(t, "displayed 到达", func() bool { return len(remote.reportList()) == 1 })

	// 服务端下一轮不再返回它（官方：已展示的前景卡要保留到用户关掉）。
	remote.setBatch(&apiclient.NotificationBatch{NextPollAfterSeconds: 600})
	c.Refresh()
	waitUntil(t, "第二次读", func() bool { return remote.fetchCount() >= 2 })
	time.Sleep(10 * time.Millisecond)
	if got := c.Notices(); len(got) != 1 || got[0].ID != "n-1" {
		t.Fatalf("已展示的前景卡必须保留，得到 %+v", got)
	}
}

// TestStaleReplyDiscarded：end() 之后迟到的回复**不得发布**（官方 §5.4 过期替换保护）。
func TestStaleReplyDiscarded(t *testing.T) {
	returned := make(chan struct{})
	block := make(chan struct{})
	remote := &fakeRemote{
		block:    block,
		returned: returned,
		batch: &apiclient.NotificationBatch{
			NextPollAfterSeconds: 600,
			Notifications:        []apiclient.Notification{sampleNotice("n-late")},
		},
	}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "请求已在途", func() bool { return remote.fetchCount() == 1 })

	c.End() // 退出账号：代次变了
	close(block)
	<-returned
	time.Sleep(10 * time.Millisecond)
	if got := c.Notices(); len(got) != 0 {
		t.Fatalf("迟到的回复不得发布，得到 %+v", got)
	}
}

// TestSetLocaleRefreshesContentOnly：切语言只换文案，不重新列表（官方 setLocale）。
func TestSetLocaleRefreshesContentOnly(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true, Locale: "zh-CN"})
	waitUntil(t, "一张卡", func() bool { return len(c.Notices()) == 1 })
	before := remote.fetchCount()

	localized := sampleNotice("n-1")
	localized.Title = "Bonus credited"
	remote.mu.Lock()
	remote.contents = map[string]*apiclient.Notification{"n-1": &localized}
	remote.mu.Unlock()

	c.SetLocale("en-US")
	waitUntil(t, "内容已按新语言刷新", func() bool {
		got := c.Notices()
		return len(got) == 1 && got[0].Title == "Bonus credited"
	})
	if after := remote.fetchCount(); after != before {
		t.Fatalf("切语言不该重新列表（fetch 次数 %d → %d）", before, after)
	}
	if got := c.Meta().Locale; got != "en-US" {
		t.Fatalf("meta.locale 应跟着变，得到 %q", got)
	}
}

// TestSetLocaleSameIsNoop：语言没变直接返回（官方 setLocale 的第一行）。
func TestSetLocaleSameIsNoop(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	c := NewController(remote)
	c.Update(AccountState{Active: true, SignedIn: true, Locale: "zh-CN"})
	waitUntil(t, "一张卡", func() bool { return len(c.Notices()) == 1 })
	c.SetLocale("zh-CN")
	time.Sleep(10 * time.Millisecond)
	if got := len(c.Notices()); got != 1 {
		t.Fatalf("语言没变时不该动卡片，剩 %d 张", got)
	}
}

// TestOpenAccountNotWired：没接线时 OpenAccount 返回 false（CTA 据此给提示，不装跳转）。
func TestOpenAccountNotWired(t *testing.T) {
	c := NewController(nil)
	if c.OpenAccount() {
		t.Fatal("没 SetOpenAccount 时不该返回 true")
	}
	called := 0
	c.SetOpenAccount(func() bool { called++; return true })
	if !c.OpenAccount() || called != 1 {
		t.Fatalf("接线后应执行一次，called=%d", called)
	}
}
