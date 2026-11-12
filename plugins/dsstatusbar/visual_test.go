//go:build darwin || windows || linux

package dsstatusbar

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestFG5HeadlessDockAndConnection 用真实 composer、ds-core 与 flex 布局验收。
// FG5_CAPTURE_DIR 非空时保留 PNG；默认也在临时目录完成相同实画断言。
func TestFG5HeadlessDockAndConnection(t *testing.T) {
	dir := os.Getenv("FG5_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	status := &Plugin{}
	// 假时钟：连接指示有官方 150ms 离场淡出（旧态再画 150ms 才换新态）。
	// 这个测试每个状态只画一帧，不给假时钟的话每一帧都落在离场窗里——
	// 那是"动画没走完"，不是"状态错了"。每次换数据把时钟推过 150ms。
	fakeNow := time.Unix(0, 0)
	status.now = func() time.Time { return fakeNow }
	composer := &dscomposer.Plugin{}
	kernel := uikernel.New("fg5-headless")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, composer, status); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &ui.Tree{}
	feat := tree.Feature("fg5-headless")
	data := pluginapi.Snapshot{
		Cur: "fg5-real-layout", Status: "已连接",
		// 连接三态由壳给（`Snapshot.Link`）：这一组用例先验"连着"那一态。
		Link:          pluginapi.LinkConnected,
		UsageTotal:    pluginapi.Usage{InputTokens: 12000, OutputTokens: 345, TotalTokens: 12345},
		ContextTokens: 2000,
		ContextWindow: 10000,
		// 这些窗口消息必须不能变成官方轮/步。
		Msgs: []apiclient.Message{{Role: "assistant"}, {Role: "assistant"}, {Role: "tool"}},
	}
	var (
		view        flex.View
		invalidator app.Invalidator
		dockBox     flex.Box
	)
	config := app.Config{
		Title: "fg5-headless", Width: 1920, Height: 1032, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height,
				MainWidth: size.Width, BottomHeight: 24,
			},
			Has: kernel.Has,
		}
		dockBox = flex.Box{}
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			f.Column().FillWidth().FillHeight().Children(func() {
				f.Row().Grow(1)
				// 与 dsshell 中栏一致：先 Pad(12)，再按 Order 画 composer 和 dock。
				f.Column().FillWidth().Pad(12).Gap(8).Children(func() {
					for _, painter := range kernel.Painters(pluginapi.SlotMainComposer) {
						if painter.ID == "dock" {
							dockBox = status.drawDock(f, env)
							continue
						}
						painter.Draw(f, env)
					}
				})
				// 连接指示的官方座位：`sidebar.settings`（官方左栏默认 280 宽）。
				// 这里模拟骨架在 footArea 里 dispatch 那个槽（不再有 shell.bottom 整行）。
				f.Row().Width(280).Height(connRowHeight + 2*connRowMarginY).Children(func() {
					for _, painter := range kernel.Painters(uikernel.OffSidebarSettings) {
						painter.Draw(f, env)
					}
				})
			})
		})
		for _, painter := range kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	assertFrame := func(width, height int, name, wantTotal, wantContext string, disconnected bool) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		snapshot := session.Snapshot()
		for _, want := range []string{wantTotal, wantContext} {
			if !session.HasText(want) {
				t.Fatalf("%s 缺真值文本 %q；texts=%q", name, want, session.Texts())
			}
		}
		joined := strings.Join(session.Texts(), " ")
		for _, banned := range []string{" 轮", " 步", "tok/s", "缓存命中"} {
			if strings.Contains(joined, banned) {
				t.Fatalf("%s 出现未供真值的指标 %q；texts=%q", name, banned, joined)
			}
		}
		if !strings.Contains(wantContext, "%") && strings.Contains(joined, "%") {
			t.Fatalf("%s 容量未知/异常却出现百分比；texts=%q", name, joined)
		}
		var contextTexts []string
		for _, text := range session.Texts() {
			if strings.HasPrefix(text, "上下文 ") {
				contextTexts = append(contextTexts, text)
			}
		}
		if len(contextTexts) != 1 || contextTexts[0] != wantContext {
			t.Fatalf("%s 上下文文本残留或重复：got=%q want=%q", name, contextTexts, wantContext)
		}
		// 连接指示走官方 `ConnectionIndicator`：28 高胶囊 + 官方 success/warning 色。
		wantConnection := zhConnectionConnected
		wantFill := theme.SuccessTertiary
		if disconnected {
			wantConnection = zhConnectionError
			wantFill = theme.WarnTertiary
			if session.HasText(zhConnectionConnected) {
				t.Fatalf("%s 断线后仍显示「%s」", name, zhConnectionConnected)
			}
		}
		if !session.HasText(wantConnection) {
			t.Fatalf("%s 缺连接状态 %q；texts=%q", name, wantConnection, session.Texts())
		}
		var connectionPill bool
		var card app.Primitive
		for _, primitive := range snapshot.Primitives {
			if primitive.Kind == app.RectanglePrimitive && primitive.Bounds.Height == connIndicatorHeight-2 &&
				primitive.Color == wantFill {
				connectionPill = true
			}
			if primitive.Color == theme.InputMajor && primitive.Bounds.Width*primitive.Bounds.Height > card.Bounds.Width*card.Bounds.Height {
				card = primitive
			}
		}
		if !connectionPill {
			t.Fatalf("%s 侧栏没画出官方连接胶囊（%v 底 / %v 高）", name, wantFill, connIndicatorHeight)
		}
		dock := dockBox.Rect()
		if closeEnoughFG5(dock.Height, 0) {
			t.Fatalf("%s dock 未实画", name)
		}
		if !closeEnoughFG5(dock.Height, 26) {
			t.Fatalf("%s dock 高 = %.2f，官方 26（不是旧 24/标题栏 40）", name, dock.Height)
		}
		if card.Bounds.Width <= 0 || dock.Y < card.Bounds.Y+card.Bounds.Height {
			t.Fatalf("%s dock 不在 composer 卡片下方：card=%+v dock=%+v", name, card.Bounds, dock)
		}
		cardCenter := card.Bounds.X + card.Bounds.Width/2
		dockCenter := dock.X + dock.Width/2
		if math.Abs(float64(cardCenter-dockCenter)) > 0.01 {
			t.Fatalf("%s 卡片与 dock 不同轴：cardCenter=%.2f dockCenter=%.2f", name, cardCenter, dockCenter)
		}
		path := filepath.Join(dir, name)
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("%s 截图为空：path=%s info=%v err=%v", name, path, info, err)
		}
		t.Logf("FG5_CAPTURE path=%s size=%dx%d bytes=%d dock=(x %.1f y %.1f w %.1f h %.1f) cardCenter=%.1f connection=%s",
			path, width, height, info.Size(), dock.X, dock.Y, dock.Width, dock.Height, cardCenter, wantConnection)
	}

	// pump 画一帧，并先把假时钟推过官方 150ms 离场窗。
	//
	// 两帧才换态是**故意**的：第一帧进入离场窗（仍画旧态），时钟推过 150ms
	// 后第二帧才换成新态。少画一帧就会停在"旧态还亮着"（动画没走完）。
	pump := func() {
		t.Helper()
		fakeNow = fakeNow.Add(200 * time.Millisecond)
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	// setData 画三帧才换态，因为官方动画是「离场 150ms + 入场 150ms」：
	//   第 1 帧：进入离场窗（仍画旧态）
	//   第 2 帧：离场走完、换成新态，同一帧开始 `_indicator-enter`（opacity 0）
	//   第 3 帧：入场走完，新态才是全不透明（断言看到的每一帧都是这一帧）
	setData := func(next pluginapi.Snapshot) {
		t.Helper()
		data = next
		pump()
		pump()
		pump()
	}

	// 官方 `_indicator-enter .15s ease-out`：首帧是入场动画的起点（opacity 0），
	// 所以要先把假时钟推过 150ms 让胶囊真正画出来，再开始断言。
	pump()
	pump()

	// 同一真实 flex/canvas 窗口覆盖两种尺寸、已知/未知/异常容量与压缩回落。
	assertFrame(1920, 1032, "stats-wire-known-1920x1032.png", "总量 12.3k", "上下文 2.0k · 20%", false)
	assertFrame(1182, 832, "stats-wire-known-1182x832.png", "总量 12.3k", "上下文 2.0k · 20%", false)
	unknown := data
	unknown.ContextWindow = 0
	setData(unknown)
	assertFrame(1920, 1032, "stats-wire-unknown-1920x1032.png", "总量 12.3k", "上下文 2.0k", false)
	assertFrame(1182, 832, "stats-wire-unknown-1182x832.png", "总量 12.3k", "上下文 2.0k", false)
	overCapacity := data
	overCapacity.ContextTokens = 12000
	overCapacity.ContextWindow = 10000
	setData(overCapacity)
	assertFrame(1920, 1032, "stats-wire-over-capacity-1920x1032.png", "总量 12.3k", "上下文 12.0k / 10.0k（超出容量）", false)
	assertFrame(1182, 832, "stats-wire-over-capacity-1182x832.png", "总量 12.3k", "上下文 12.0k / 10.0k（超出容量）", false)
	compacted := data
	compacted.ContextTokens = 500
	compacted.ContextWindow = 10000
	setData(compacted)
	assertFrame(1920, 1032, "stats-wire-compacted-1920x1032.png", "总量 12.3k", "上下文 500 · 5%", false)
	assertFrame(1182, 832, "stats-wire-compacted-1182x832.png", "总量 12.3k", "上下文 500 · 5%", false)

	// A→B→A：每个会话只显示自己的 used/capacity，不能把 A 的 10k 套到 B 的 32k。
	setData(pluginapi.Snapshot{
		Cur: "session-b", Status: "已连接", Link: pluginapi.LinkConnected,
		UsageTotal: pluginapi.Usage{TotalTokens: 777}, ContextTokens: 8000, ContextWindow: 32000,
	})
	assertFrame(1920, 1032, "stats-wire-session-b-1920x1032.png", "总量 777", "上下文 8.0k · 25%", false)
	assertFrame(1182, 832, "stats-wire-session-b-1182x832.png", "总量 777", "上下文 8.0k · 25%", false)
	setData(compacted)
	assertFrame(1920, 1032, "stats-wire-session-a-return-1920x1032.png", "总量 12.3k", "上下文 500 · 5%", false)
	assertFrame(1182, 832, "stats-wire-session-a-return-1182x832.png", "总量 12.3k", "上下文 500 · 5%", false)

	disconnected := compacted
	disconnected.Link = pluginapi.LinkDown
	disconnected.Disconnected = true
	disconnected.Status = "连不上宿主，重试中…"
	setData(disconnected)
	assertFrame(1182, 832, "stats-wire-disconnected-1182x832.png", "总量 12.3k", "上下文 500 · 5%", true)

	// 同一真实窗口切到 B：只能出现 B 的 whole-log 数字，不能残留 A。
	data = pluginapi.Snapshot{
		Cur: "session-b", Status: "已连接", Link: pluginapi.LinkConnected,
		UsageTotal: pluginapi.Usage{TotalTokens: 777}, ContextTokens: 100,
	}
	pump()
	pump()
	for _, want := range []string{"总量 777", "上下文 100"} {
		if !session.HasText(want) {
			t.Fatalf("切到 B 后缺 B 的真值 %q；texts=%q", want, session.Texts())
		}
	}
	for _, stale := range []string{"总量 12.3k", "上下文 2.0k", zhConnectionError} {
		if session.HasText(stale) {
			t.Fatalf("切到 B 后仍显示 A/断线残留 %q", stale)
		}
	}

	// 空会话即便上游错误地残留 usage，session-scoped dock 也不能创建盒子。
	data = pluginapi.Snapshot{UsageTotal: aUsageForStaleTest(), ContextTokens: 2000}
	pump()
	pump()
	if session.HasText("总量 12.3k") || session.HasText("上下文 2.0k") {
		t.Fatalf("空会话显示了残留 usage；texts=%q", session.Texts())
	}
	if dockBox != (flex.Box{}) {
		t.Fatal("空会话仍创建了 dock 盒子")
	}
	t.Log("FG5_SESSION_ISOLATION A->B->empty passed; empty dock box absent")
}

func aUsageForStaleTest() pluginapi.Usage {
	return pluginapi.Usage{InputTokens: 12000, OutputTokens: 345, TotalTokens: 12345}
}

func closeEnoughFG5(got, want float32) bool {
	return math.Abs(float64(got-want)) <= 0.01
}
