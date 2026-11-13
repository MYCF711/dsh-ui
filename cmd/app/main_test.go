// main_test.go —— 壳里的**纯函数**单测。
//
// 为什么这个包之前是 0%：壳的大部分代码是"把各组件拼起来"，天然难测，
// 所以一直只有真机验证。但里面有几个纯判断（尺寸解析、按键解析、凭据错误判断、
// 审批选项转换）值得单独钉住——它们出错时的症状都很"远期"：
// 比如 -size 解析把 320 当合法下限、or 把 401 当网络抖动，用户只会看到
// "窗口怪怪的" / "一直连不上"。
package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestParseWinSize(t *testing.T) {
	cases := []struct {
		in      string
		w, h    int
		wantOK  bool
		comment string
	}{
		{"1600x900", 1600, 900, true, "正常"},
		{"800X600", 800, 600, true, "大写 X"},
		{"1024×768", 1024, 768, true, "乘号"},
		{"320x240", 320, 240, true, "下限刚好"},
		{"319x240", 0, 0, false, "宽度低于下限"},
		{"320x239", 0, 0, false, "高度低于下限"},
		{"", 0, 0, false, "空"},
		{"1600", 0, 0, false, "没有分隔符"},
		{"x900", 0, 0, false, "缺宽"},
		{"1600x", 0, 0, false, "缺高"},
		{"axb", 0, 0, false, "不是数字"},
		{"1600x900x2", 0, 0, false, "多段"},
	}
	for _, c := range cases {
		w, h, ok := parseWinSize(c.in)
		if ok != c.wantOK || (ok && (w != c.w || h != c.h)) {
			t.Errorf("parseWinSize(%q) = (%d,%d,%v)，期望 (%d,%d,%v) [%s]",
				c.in, w, h, ok, c.w, c.h, c.wantOK, c.comment)
		}
	}
}

func TestParseKeys(t *testing.T) {
	keys := parseKeys("down,up,home,end,esc,enter")
	want := []gfx.Key{gfx.KeyDown, gfx.KeyUp, gfx.KeyHome, gfx.KeyEnd, gfx.KeyEscape, gfx.KeyEnter}
	if len(keys) != len(want) {
		t.Fatalf("解析出 %d 个键，期望 %d", len(keys), len(want))
	}
	for i := range want {
		if keys[i].Key != want[i] || keys[i].Modifiers != 0 {
			t.Fatalf("第 %d 个键 = %+v，期望 %v（无修饰键）", i, keys[i], want[i])
		}
	}
	// 大小写与空白都要容忍（命令行手写时常见的）
	if got := parseKeys(" DOWN , Esc "); len(got) != 2 || got[0].Key != gfx.KeyDown || got[1].Key != gfx.KeyEscape {
		t.Fatalf("大小写/空白容忍失败：%v", got)
	}
	// Shift+F2 是设置浮层的入口：**修饰键必须保留**，否则注入的按键只会打开命令面板。
	if got := parseKeys("shift+f2"); len(got) != 1 || got[0].Key != gfx.KeyF2 || got[0].Modifiers != gfx.ModifierShift {
		t.Fatalf("shift+f2 应解析成带 Shift 的 F2，得到 %+v", got)
	}
	// 不认识的键静默跳过，不 panic
	if got := parseKeys("down,bogus,up"); len(got) != 2 {
		t.Fatalf("未知键应被跳过，得到 %v", got)
	}
	if got := parseKeys(""); len(got) != 0 {
		t.Fatalf("空串应得到 0 个键，得到 %v", got)
	}
}

func TestIsAuthErr(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"websocket: bad handshake: got 401", true},
		{"failed: got 403 Forbidden", true},
		{"dial tcp: connection refused", false},
		{"timeout", false},
		{"got 404 Not Found", false}, // 4xx 但不是凭据问题
		{"", false},
	}
	for _, c := range cases {
		var err error
		if c.msg != "" {
			err = errors.New(c.msg)
		}
		if got := isAuthErr(err); got != c.want {
			t.Errorf("isAuthErr(%q) = %v，期望 %v", c.msg, got, c.want)
		}
	}
	if isAuthErr(nil) {
		t.Error("nil 不该被判成凭据错误")
	}
}

func TestApprovalOptionsConversion(t *testing.T) {
	in := []apiclient.ApprovalOption{
		{OptionID: "allow_once", Name: "允许这一次", Kind: "allow_once"},
		{OptionID: "allow_always", Name: "这个会话里以后都允许", Kind: "allow_always"},
		{OptionID: "reject_once", Name: "拒绝", Kind: "reject_once"},
	}
	out := approvalOptions(in)
	if len(out) != 3 {
		t.Fatalf("应转换 3 项，得到 %d", len(out))
	}
	// **原样**：id / name / kind 一个都不许改写（插件靠 kind 挑选项）
	for i := range in {
		if out[i].OptionID != in[i].OptionID || out[i].Name != in[i].Name || out[i].Kind != in[i].Kind {
			t.Fatalf("第 %d 项被改写了：%+v → %+v", i, in[i], out[i])
		}
	}
	if got := approvalOptions(nil); got != nil {
		t.Fatalf("nil 输入应返回 nil，得到 %v", got)
	}
}

func TestMaxInt(t *testing.T) {
	if maxInt(3, 5) != 5 || maxInt(5, 3) != 5 || maxInt(-1, -2) != -1 || maxInt(7, 7) != 7 {
		t.Fatal("maxInt 结果不对")
	}
}

func TestRightbarDefaultsClosedWithoutToolTrajectory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"empty","messages":[{"seq":1,"role":"assistant","text":"没有工具"}],"total":1}`))
	}))
	defer ts.Close()

	// true 模拟旧 main 的 -rightbar 默认值；加载一个无工具会话后必须纠正为关闭。
	m := &model{
		cli:          apiclient.New(ts.URL),
		cache:        map[string]*cachedSession{},
		epochs:       map[string]int64{},
		cur:          "empty",
		showRightbar: true,
	}
	m.refreshSession("empty", false)
	if m.showRightbar {
		t.Fatal("无工具轨迹的会话加载后右栏仍打开；旧默认让空右栏占据主区")
	}
}

func TestRightbarDefaultsClosedEvenWithToolTrajectory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"tools","messages":[{"seq":1,"role":"tool","tool":"bash","summary":"exit 0"}],"total":1}`))
	}))
	defer ts.Close()

	m := &model{
		cli:    apiclient.New(ts.URL),
		cache:  map[string]*cachedSession{},
		epochs: map[string]int64{},
		cur:    "tools",
	}
	m.refreshSession("tools", false)
	if m.showRightbar {
		t.Fatal("含真实 role=tool 轨迹的历史会话首次打开仍应默认收起右栏")
	}
}

func TestRightbarSessionStateDoesNotLeak(t *testing.T) {
	m := &model{}
	show := func(id string, msgs ...apiclient.Message) {
		t.Helper()
		m.mu.Lock()
		m.cur = id
		m.msgs = msgs
		m.syncRightbarLocked()
		m.mu.Unlock()
	}
	assertOpen := func(want bool, context string) {
		t.Helper()
		if m.showRightbar != want {
			t.Fatalf("%s：showRightbar=%v，期望 %v", context, m.showRightbar, want)
		}
	}

	show("empty", apiclient.Message{Role: "assistant", Text: "无工具"})
	assertOpen(false, "空会话默认")
	m.toggleRightbar() // 用户在 empty 明确打开
	assertOpen(true, "empty 用户打开")

	show("tools", apiclient.Message{Role: "tool", Tool: "bash"})
	assertOpen(false, "tools 有历史轨迹也默认收起")
	m.toggleRightbar() // 用户在 tools 明确打开
	assertOpen(true, "tools 用户打开")

	show("fresh", apiclient.Message{Role: "assistant", Text: "另一个空会话"})
	assertOpen(false, "fresh 不继承 empty 的打开状态")
	show("empty", apiclient.Message{Role: "assistant", Text: "无工具"})
	assertOpen(true, "返回 empty 恢复它自己的选择")
	show("tools", apiclient.Message{Role: "tool", Tool: "bash"})
	assertOpen(true, "返回 tools 保留用户打开选择")
}

func TestRightbarGeometryTracksVisibility(t *testing.T) {
	const total, sidebar = float32(1180), float32(280)
	closed := geomRightWidth(total, sidebar, false)
	opened := geomRightWidth(total, sidebar, true)
	if closed != 0 {
		t.Fatalf("右栏关闭时宽度 = %v，期望 0", closed)
	}
	if opened <= 0 {
		t.Fatalf("右栏打开时宽度 = %v，期望正数", opened)
	}
	if got, want := total-sidebar-closed, float32(900); got != want {
		t.Fatalf("右栏关闭时主区宽度 = %v，期望 %v", got, want)
	}
	if got, want := total-sidebar-opened, float32(400); got != want {
		t.Fatalf("右栏打开时主区宽度 = %v，期望 %v", got, want)
	}
}

// TestGeometryMatchesOfficialColumns 直接钉住**壳传给插件的那份 Geometry**。
//
// 三条是 UIPLAN V4B 的验收算例，来自官方 computeColumns（`AppFrame.js`）：
//
//	1920 无轨迹 → 侧栏 280 / 中栏 1640 / 右栏 0
//	1182 无轨迹 → 280 / 902 / 0
//	1182 有轨迹 → 280 / 400 / 502
//
// 为什么要在单测里钉：以前这组数字只能在起窗口截图里用像素量，
// 量出来的是"插件画成这样"，而不是"壳说成这样"。resize 时真正决定
// 三栏宽的是这份传入值，它错一档，插件再怎么按官方求解也救不回来。
func TestGeometryMatchesOfficialColumns(t *testing.T) {
	cases := []struct {
		name       string
		width      float32
		rightbarOn bool
		wantRight  float32
		wantMain   float32
	}{
		{"1920 无轨迹（右栏关）", 1920, false, 0, 1640},
		{"1182 无轨迹（右栏关）", 1182, false, 0, 902},
		{"1182 有轨迹（右栏开）", 1182, true, 502, 400},
		{"1920 有轨迹（右栏开）", 1920, true, 864, 776},
	}
	for _, c := range cases {
		g := buildGeometry(c.width, 832, 280, c.rightbarOn, true)
		if g.SidebarWidth != 280 || g.RightWidth != c.wantRight || g.MainWidth != c.wantMain {
			t.Fatalf("%s：得到 侧栏/右栏/中栏 = %v/%v/%v，期望 280/%v/%v",
				c.name, g.SidebarWidth, g.RightWidth, g.MainWidth, c.wantRight, c.wantMain)
		}
		if g.Width != c.width || g.Height != 832 || g.BottomHeight != 24 {
			t.Fatalf("%s：Width/Height/BottomHeight = %v/%v/%v，期望 %v/832/24",
				c.name, g.Width, g.Height, g.BottomHeight, c.width)
		}
		// 三列之和必须严格等于视口，否则中间会空一条或把右栏挤出窗口。
		if sum := g.SidebarWidth + g.MainWidth + g.RightWidth; sum != c.width {
			t.Fatalf("%s：三列之和 = %v，期望 %v", c.name, sum, c.width)
		}
	}
}

// TestGeometryResizeSequence 是同一进程连续改尺寸的回归：1182 → 1920 → 1182。
//
// buildGeometry 是纯函数，这个测试防的是"以后有人把它改成带缓存/带字段的实现"——
// 那时第二次算同一宽度会沿用中间态，而真机 resize 正是这个形状
// （证据：tmp/v4b-shots/C-open-1182.png / C-open-1920.png / C-open-1182-back.png）。
func TestGeometryResizeSequence(t *testing.T) {
	steps := []struct {
		width     float32
		wantRight float32
		wantMain  float32
	}{
		{1182, 502, 400},
		{1920, 864, 776},
		{1182, 502, 400}, // 缩回去必须回到第一档，不能留下 1920 的 864
	}
	for i, s := range steps {
		g := buildGeometry(s.width, 832, 280, true, true)
		if g.RightWidth != s.wantRight || g.MainWidth != s.wantMain {
			t.Fatalf("第 %d 步 %v 宽：右栏/中栏 = %v/%v，期望 %v/%v",
				i+1, s.width, g.RightWidth, g.MainWidth, s.wantRight, s.wantMain)
		}
	}
}

var _ = pluginapi.Approval{} // 保持 pluginapi 引用（审批结构参与上面的转换测试）

// TestGeometryRightbarVanishesWhenNoRoom 钉住官方 computeColumns 的「整列消失」。
//
// 早先的实现（geomRightWidth）在中栏被压到 400 以下时，把右栏硬压到 300 硬塞：
// 960 宽实测 280 + 389 + 291——中栏掉到官方下限以下，右栏也不到 300。
// 官方是：available = viewport - sidebar - 400，available < 300 时右栏直接 0。
func TestGeometryRightbarVanishesWhenNoRoom(t *testing.T) {
	cases := []struct {
		width float32
		want  [3]float32 // sidebar / main / right
		note  string
	}{
		{960, [3]float32{280, 680, 0}, "右栏放不下 → 整列消失（官方 680/0）"},
		{1182, [3]float32{280, 400, 502}, "available=502，右栏偏好 531.9 被卡到 502"},
		{1920, [3]float32{280, 776, 864}, "偏好 864 未超上限"},
		{600, [3]float32{280, 320, 0}, "极窄：右栏消失，中栏恰好 320（600-280）"},
	}
	for _, c := range cases {
		g := buildGeometry(c.width, 832, 280, true, false)
		got := [3]float32{g.SidebarWidth, g.MainWidth, g.RightWidth}
		if got != c.want {
			t.Errorf("width=%.0f (%s): 得到 sidebar/main/right = %.0f/%.0f/%.0f，期望 %.0f/%.0f/%.0f",
				c.width, c.note, got[0], got[1], got[2], c.want[0], c.want[1], c.want[2])
		}
		if c.width >= 320 && got[0]+got[1]+got[2] > c.width+0.5 {
			t.Errorf("width=%.0f: 三列之和 %.1f 超过视口", c.width, got[0]+got[1]+got[2])
		}
	}
	// 右栏关：右栏必须是 0，中栏吃满
	g := buildGeometry(1182, 832, 280, false, false)
	if g.RightWidth != 0 || g.MainWidth != 902 {
		t.Fatalf("右栏关应是 280/902/0，得到 %.0f/%.0f/%.0f", g.SidebarWidth, g.MainWidth, g.RightWidth)
	}
}

// TestFirstLoadedMessagesGetFollowUpFrame 复现 G7 首载空白，不依赖 OS 窗口。
//
// Viva 的 Follow 在布局末尾才把 Offset 移到尾部，而消息插件在声明阶段已经
// 用旧 Offset 做了视口虚拟化。第一批消息因此需要两帧：第一帧确定内容高度，
// 第二帧才按尾部 Offset 实画消息。空消息 Hero 帧不得提前消费 pendingScroll。
func TestFirstLoadedMessagesGetFollowUpFrame(t *testing.T) {
	const id = "first-load"
	m := &model{
		cur:           id,
		pendingScroll: id,
		scrollPos:     map[string]float32{},
	}

	// v0.6.0：主题是构造期给的（Draw 不再收 theme）；滚动状态由声明方持有，
	// 壳拿到的是插件交回来的 Box（这里由本支架自己声明并交回）。
	pageTheme := ui.LightTheme()
	m.view = ui.NewPage(ui.PageConfig{Theme: &pageTheme})

	// 首开先画的是空消息 Hero；此时还没有 ScrollArea 的内容高度。
	if m.applyPendingScroll(ui.Box{}, false) {
		t.Fatal("空消息 Hero 帧没有可虚拟化的消息，不应请求补帧")
	}
	if m.pendingScroll != id {
		t.Fatal("空消息首帧消费了 pendingScroll；首批消息布局后不会再有恢复/补帧机会")
	}

	m.msgs = make([]apiclient.Message, 20)
	for i := range m.msgs {
		m.msgs[i] = apiclient.Message{Seq: i + 1, Role: "assistant", Text: fmt.Sprintf("message-%02d", i+1)}
	}
	draw := func() (*gfx.Canvas, bool) {
		t.Helper()
		canvas := &gfx.Canvas{}
		canvas.SetMeasurer(func(value string, size, weight float32) float32 {
			return float32(len(value)) * size / 2
		})
		var area ui.Box
		m.view.Draw(canvas, gfx.Rect{Width: 100, Height: 100}, func(f ui.C) {
			ui.Column(f).Fill().Children(func() {
				area = ui.Scroll(f).Grow(1).Follow(true)
				area.Children(func() {
					// 与 dsconversation 相同的声明期虚拟化时序：Offset 是上一帧的值。
					offset := area.ScrollState().Offset
					for i := range m.msgs {
						y := float32(i * 30)
						if y+30 < offset || y > offset+100 {
							ui.Slot(f).Height(30)
							continue
						}
						ui.Text(f, m.msgs[i].Text).Height(30)
					}
				})
			})
		})
		// 产品路径里这一步是壳用插件交回的 Box 做的（env.ScrollBox）。
		repaint := m.applyPendingScroll(area, true)
		return canvas, repaint
	}
	hasLast := func(canvas *gfx.Canvas) bool {
		for _, text := range canvas.Texts() {
			if text.Value == "message-20" {
				return true
			}
		}
		return false
	}

	first, repaint := draw()
	if hasLast(first) {
		t.Fatal("测试支架失效：第一批消息在旧 Offset 下不应直接实画尾消息")
	}
	if !repaint {
		t.Fatal("首批消息布局后没有请求补帧；尾部占位会定格成空白中栏")
	}
	if m.pendingScroll != "" {
		t.Fatal("首批消息完成布局后 pendingScroll 仍未消费")
	}
	second, again := draw() // 产品路径由 cfg.Paint 里的 Invalidate 请求这一次重绘。
	if again {
		t.Fatal("pendingScroll 已消费，不应每帧都要求重绘")
	}
	if !hasLast(second) {
		t.Fatal("Follow 已移动到尾部后，补帧仍没有实画尾消息")
	}
}

// TestPendingScrollRestoresOffset 钉住"切回会话恢复阅读位置"那条链的**后半段**：
// m.scrollPos → Box.ScrollState().ScrollTo。
//
// 背景：v0.6.0 没有 `ScrollState(id)` 这种按身份查表的入口，滚动状态由声明方持有。
// 壳的做法是——插件把本帧滚动区的 Box 交给壳（Env.ScrollBox），壳在 Draw 返回后
// （布局已定）先恢复、再记偏移。**顺序不能反**：先记的话，切会话那一帧新滚动区
// 是从 0 布局的，会把切走前的位置覆盖成 0。
func TestPendingScrollRestoresOffset(t *testing.T) {
	const id = "restore"
	pageTheme := ui.LightTheme()
	m := &model{
		cur:           id,
		view:          ui.NewPage(ui.PageConfig{Theme: &pageTheme}),
		pendingScroll: id,
		scrollPos:     map[string]float32{id: 120},
	}
	m.msgs = make([]apiclient.Message, 20)
	for i := range m.msgs {
		m.msgs[i] = apiclient.Message{Seq: i + 1, Role: "assistant", Text: fmt.Sprintf("message-%02d", i+1)}
	}

	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len(value)) * size / 2
	})
	var area ui.Box
	m.view.Draw(canvas, gfx.Rect{Width: 100, Height: 100}, func(f ui.C) {
		ui.Column(f).Fill().Children(func() {
			area = ui.Scroll(f).Grow(1).Follow(true)
			area.Children(func() {
				for i := range m.msgs {
					ui.Text(f, m.msgs[i].Text).Height(30)
				}
			})
		})
	})

	// 产品这一段就是 settleScroll：先恢复，再记偏移（顺序反了会把待恢复值覆盖成 0）。
	if !m.settleScroll(area, true) {
		t.Fatal("有待恢复的偏移时 settleScroll 必须请求补帧")
	}
	if got := area.ScrollState().Offset; got != 120 {
		t.Fatalf("恢复后的偏移 = %v，want 120（切走前记下的值）", got)
	}
	if m.pendingScroll != "" {
		t.Fatal("恢复之后 pendingScroll 必须清空，否则每帧都会重绘")
	}
	if got := m.scrollPos[id]; got != 120 {
		t.Fatalf("settleScroll 记下的偏移 = %v，want 120（先记后恢复会把这里写成 0）", got)
	}
	// 再调一次：pendingScroll 已消费，不该再要求补帧。
	if m.settleScroll(area, true) {
		t.Fatal("pendingScroll 已消费，不应再要求补帧")
	}
}
