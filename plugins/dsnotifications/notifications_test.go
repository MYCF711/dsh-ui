// notifications_test.go —— 槽位注册 + 三种降级（未授权 / 未启用 / 数据缺失）。
//
// 这个文件钉的是**槽位与可见性**，不是画得好不好看：
//
//	① 槽位名逐字等于官方 `settings.launcher.notifications`（写错一个字符，
//	   ui-settings-account 的 children 声明就认不到它）；
//	② 未授权（没有 remote / 账号未登录）→ 控制器不跑，整块不渲染；
//	③ 未启用（父级 active=false，账号菜单没打开）→ 整块不渲染；
//	④ 数据缺失（列表为空 / 全部过期 / 服务端回 null）→ 整块不渲染。
//
// "不渲染"的判据是 canvas 上一个图元都没有——不是"画了但透明"。
package dsnotifications

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// fakeRemote 是 Remote 的测试替身：数据、错误、阻塞、调用计数都可控。
type fakeRemote struct {
	mu sync.Mutex

	batch    *apiclient.NotificationBatch
	batchErr error
	// block 非 nil 时 GetNotifications 会等它关闭（用来测"迟到的回复"）。
	block    chan struct{}
	contents map[string]*apiclient.Notification
	fetches  int
	reports  []fakeReport
	// returned 在 GetNotifications 真的返回之后关闭一次（测"迟到的回复"用）。
	returned     chan struct{}
	returnedOnce sync.Once
}

type fakeReport struct {
	id    string
	event string
}

func (f *fakeRemote) GetNotifications(ctx context.Context, meta apiclient.NotificationClientMetadata) (*apiclient.NotificationBatch, error) {
	f.mu.Lock()
	block := f.block
	f.fetches++
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	f.mu.Lock()
	batch, err := f.batch, f.batchErr
	returned := f.returned
	f.mu.Unlock()
	if returned != nil {
		f.returnedOnce.Do(func() { close(returned) })
	}
	return batch, err
}

func (f *fakeRemote) GetNotificationContent(ctx context.Context, id string, meta apiclient.NotificationClientMetadata) (*apiclient.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.contents == nil {
		return nil, nil
	}
	return f.contents[id], nil
}

func (f *fakeRemote) ReportNotificationEvent(ctx context.Context, id, event string, meta apiclient.NotificationClientMetadata) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, fakeReport{id: id, event: event})
	return true, nil
}

func (f *fakeRemote) setBatch(b *apiclient.NotificationBatch) {
	f.mu.Lock()
	f.batch = b
	f.mu.Unlock()
}

func (f *fakeRemote) setErr(err error) {
	f.mu.Lock()
	f.batchErr = err
	f.mu.Unlock()
}

func (f *fakeRemote) fetchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fetches
}

func (f *fakeRemote) reportList() []fakeReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeReport(nil), f.reports...)
}

// waitUntil 等一个条件成立（控制器是异步的，测试必须等，但**不许** sleep 一个常数）。
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("等不到：%s", what)
}

// newTestPlugin 造一个不经过 kernel 的插件实例（Register 只做注入，测试直接搭）。
func newTestPlugin(remote Remote) *Plugin {
	theme := dscore.Dark()
	p := &Plugin{theme: theme}
	p.ctrl = NewController(remote)
	p.ctrl.SetTheme(theme)
	return p
}

// render 画一帧，返回画布（判据取 Rectangles/Texts/Hits）。
func render(t *testing.T, p *Plugin) *ui.Canvas {
	t.Helper()
	theme := dscore.Dark()
	tree := &ui.Tree{}
	feat := tree.Feature("notice-test")
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: ui.Rect{Width: 1180, Height: 800}, MainWidth: 900,
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
		p.draw(f, env)
	})
	return canvas
}

func sampleNotice(id string) apiclient.Notification {
	return apiclient.Notification{
		ID:        id,
		CreatedAt: "2026-11-12T00:00:00Z",
		Title:     "赠金到账",
		Body:      "1000 tokens 已加入你的账户",
		Actions: []apiclient.NotificationAction{
			{Label: "查看", Style: "primary", Target: apiclient.NotificationTarget{Type: "app_route", Name: "account"}},
			{Label: "关闭", Style: "secondary", Target: apiclient.NotificationTarget{Type: "dismiss"}},
		},
	}
}

// TestSlotNameIsOfficial 是槽位契约本身：名字必须逐字等于官方。
func TestSlotNameIsOfficial(t *testing.T) {
	if got := string(SlotSettingsLauncherNotifications); got != "settings.launcher.notifications" {
		t.Fatalf("槽位名必须逐字等于官方 settings.launcher.notifications，得到 %q", got)
	}
}

// TestPanelsRegisterOfficialSlot 钉住注册点：**叶槽一块画 + 两级父容器三块画**。
//
// 期望形状（官方层级，见 launcher.go）：第 1节侧栏设置行 → 第 2节账号菜单 → 叶槽通知卡片。
// 叶槽仍然只有一块画（single 槽），父容器那三块各自挂在父级槽上。
func TestPanelsRegisterOfficialSlot(t *testing.T) {
	p := newTestPlugin(nil)
	var leaves int
	seen := map[pluginapi.Slot]string{}
	for _, painter := range p.Panels() {
		if painter.ID == "" {
			t.Fatalf("画缺 ID：%+v", painter)
		}
		if painter.Draw == nil && painter.AfterDraw == nil {
			t.Fatalf("%q 缺 Draw/AfterDraw（内核装配会 fail-loud）", painter.ID)
		}
		if other, dup := seen[painter.Slot]; dup {
			t.Fatalf("槽 %q 被画了两次（%q 与 %q）", painter.Slot, other, painter.ID)
		}
		seen[painter.Slot] = painter.ID
		if painter.Slot == SlotSettingsLauncherNotifications {
			leaves++
			if painter.Draw == nil {
				t.Fatalf("叶槽 %q 必须有 Draw", painter.ID)
			}
		}
	}
	if leaves != 1 {
		t.Fatalf("叶槽（single）只该有一块画，得到 %d", leaves)
	}
	// 只有两个父槽：`sidebar.settings`（官方 triggerRow 容器）与
	// `settings.launcher`（AccountMenu）。过渡桥已经删掉——壳在**所有**绘制
	// 路径下都画设置行，不需要第二份挂载点（HH 单第 1 件）。
	for _, want := range []pluginapi.Slot{SlotSidebarSettings, SlotSettingsLauncher} {
		if _, ok := seen[want]; !ok {
			t.Fatalf("父容器缺 painter：槽 %q 上没有画", want)
		}
	}
}

// TestSlotOrderStable：每个槽里的顺序本身无意义，但必须稳定（截图对比用）。
//
// 叶槽与二级父槽 Order 0；侧栏 footer 的两块（旧 footer 行 Order 30/40、设置行 60）
// 必须保持"先动作、后设置"的官方顺序。
func TestSlotOrderStable(t *testing.T) {
	p := newTestPlugin(nil)
	first := p.Panels()
	second := p.Panels()
	if len(first) != len(second) {
		t.Fatalf("同一份插件两次 Panels() 数量不同：%d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID || first[i].Slot != second[i].Slot || first[i].Order != second[i].Order {
			t.Fatalf("第 %d 块画不稳定：%+v vs %+v", i, first[i], second[i])
		}
	}
	orders := map[string]int{}
	for _, painter := range first {
		orders[painter.ID] = painter.Order
	}
	if orders["account-notices"] != 0 {
		t.Fatalf("叶槽 Order 应为 0，得到 %d", orders["account-notices"])
	}
	if orders["account-launcher.row"] <= 30 {
		t.Fatalf("设置行必须排在旧 footer 行（Order 30/40）之后，得到 %d", orders["account-launcher.row"])
	}
}

// TestDrawsNothingWithoutRemote 是**降级 ①：未授权**。
//
// 没有 remote（或账号未登录）时控制器根本不发请求，界面整块不渲染。
func TestDrawsNothingWithoutRemote(t *testing.T) {
	p := newTestPlugin(nil)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true})
	if got := p.ctrl.Notices(); len(got) != 0 {
		t.Fatalf("没有 remote 不该有通知，得到 %+v", got)
	}
	if rects := render(t, p).Rectangles(); len(rects) != 0 {
		t.Fatalf("未授权时不该画任何东西，画了 %d 个矩形", len(rects))
	}
}

// TestDrawsNothingWhenParentInactive 是**降级 ②：未启用**。
//
// 父级（账号菜单）没打开 → `active=false` → 整块不渲染（官方 AccountNotifications.tsx:29）。
func TestDrawsNothingWhenParentInactive(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: false, SignedIn: true})
	waitUntil(t, "取到通知（父级未启用也要有数据，才能证明是 active 挡住的）", func() bool {
		return len(p.ctrl.Notices()) == 1
	})
	if rects := render(t, p).Rectangles(); len(rects) != 0 {
		t.Fatalf("账号菜单没打开时不该画任何东西，画了 %d 个矩形", len(rects))
	}
}

// TestDrawsNothingWithoutNotices 是**降级 ③：数据缺失**。
//
// Active 为真、控制器在跑，但列表为空 → 不画空壳（官方"没有卡片就没有 stack"）。
func TestDrawsNothingWithoutNotices(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{NextPollAfterSeconds: 600}}
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "第一次取列表完成", func() bool { return remote.fetchCount() >= 1 })
	if rects := render(t, p).Rectangles(); len(rects) != 0 {
		t.Fatalf("没有通知时不该画任何东西，画了 %d 个矩形", len(rects))
	}
}

// TestDrawsNothingWhenBatchIsNull：服务端 null（未登录/授权在查询中变了）也是数据缺失。
func TestDrawsNothingWhenBatchIsNull(t *testing.T) {
	remote := &fakeRemote{batch: nil}
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "第一次取列表完成", func() bool { return remote.fetchCount() >= 1 })
	if rects := render(t, p).Rectangles(); len(rects) != 0 {
		t.Fatalf("null 批次时不该画任何东西，画了 %d 个矩形", len(rects))
	}
}

// TestDrawsNothingWhenExpired：全部过期 → 与空列表同一条路。
func TestDrawsNothingWhenExpired(t *testing.T) {
	past := "2000-01-01T00:00:00Z"
	n := sampleNotice("n-old")
	n.DisplayUntil = &past
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{n},
	}}
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "第一次取列表完成", func() bool { return remote.fetchCount() >= 1 })
	if got := p.ctrl.Notices(); len(got) != 0 {
		t.Fatalf("过期通知不该留在列表里，得到 %+v", got)
	}
	if rects := render(t, p).Rectangles(); len(rects) != 0 {
		t.Fatalf("过期后不该画任何东西，画了 %d 个矩形", len(rects))
	}
}

// TestDrawsCardWhenActiveAndNotices 是"该画"那条分支——否则上面四条
// "什么都不画"可能只是因为插件根本不会画。
func TestDrawsCardWhenActiveAndNotices(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "取到通知", func() bool { return len(p.ctrl.Notices()) == 1 })

	canvas := render(t, p)
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("有通知时应当画出卡片")
	}
	// 文案全部来自服务端：标题、正文、两个动作标签、关闭按钮标签都在画面里。
	for _, want := range []string{"赠金到账", "1000 tokens 已加入你的账户", "查看", "关闭"} {
		if !containsText(canvas, want) {
			t.Fatalf("画面里找不到 %q；画面文本=%v", want, textValues(canvas))
		}
	}
}

// TestUpdateSignedOutClearsNotices：账号退出 → end()，卡片清空（不是留着旧卡）。
func TestUpdateSignedOutClearsNotices(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-1")},
	}}
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "取到通知", func() bool { return len(p.ctrl.Notices()) == 1 })

	p.ctrl.Update(AccountState{Active: true, SignedIn: false})
	if got := p.ctrl.Notices(); len(got) != 0 {
		t.Fatalf("退出后卡片应清空，仍有 %+v", got)
	}
	if rects := render(t, p).Rectangles(); len(rects) != 0 {
		t.Fatalf("退出后不该画任何东西，画了 %d 个矩形", len(rects))
	}
}

// TestUpdateFailureAlsoEnds：账号态 failed 与未登录同一条路（官方 !signedIn）。
func TestUpdateFailureAlsoEnds(t *testing.T) {
	remote := &fakeRemote{}
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true, Failed: true})
	if remote.fetchCount() != 0 {
		t.Fatalf("failed 的账号态不该开始轮询，实际发了 %d 次请求", remote.fetchCount())
	}
}

// TestFetchErrorIsNotFatal：读取失败不是"没有通知"，只是这一轮没数据。
func TestFetchErrorIsNotFatal(t *testing.T) {
	remote := &fakeRemote{batchErr: errors.New("boom")}
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "第一次取列表完成", func() bool { return remote.fetchCount() >= 1 })
	if rects := render(t, p).Rectangles(); len(rects) != 0 {
		t.Fatalf("读取失败且没有旧卡片时不该画任何东西，画了 %d 个矩形", len(rects))
	}
}

func containsText(canvas *ui.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func textValues(canvas *ui.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}
