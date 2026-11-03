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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
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
	want := []ui.Key{ui.KeyDown, ui.KeyUp, ui.KeyHome, ui.KeyEnd, ui.KeyEscape, ui.KeyEnter}
	if len(keys) != len(want) {
		t.Fatalf("解析出 %d 个键，期望 %d", len(keys), len(want))
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("第 %d 个键 = %v，期望 %v", i, keys[i], want[i])
		}
	}
	// 大小写与空白都要容忍（命令行手写时常见的）
	if got := parseKeys(" DOWN , Esc "); len(got) != 2 || got[0] != ui.KeyDown || got[1] != ui.KeyEscape {
		t.Fatalf("大小写/空白容忍失败：%v", got)
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

func TestRightbarOpensWithToolTrajectory(t *testing.T) {
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
	if !m.showRightbar {
		t.Fatal("真实 role=tool 轨迹加载后右栏未自动打开")
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
	assertOpen(true, "tools 自动打开")
	m.toggleRightbar() // 用户在 tools 明确关闭
	assertOpen(false, "tools 用户关闭")

	show("fresh", apiclient.Message{Role: "assistant", Text: "另一个空会话"})
	assertOpen(false, "fresh 不继承 empty 的打开状态")
	show("empty", apiclient.Message{Role: "assistant", Text: "无工具"})
	assertOpen(true, "返回 empty 恢复它自己的选择")
	show("tools", apiclient.Message{Role: "tool", Tool: "bash"})
	assertOpen(false, "返回 tools 保留用户关闭选择")
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
