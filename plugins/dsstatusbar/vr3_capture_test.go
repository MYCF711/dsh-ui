//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计（dsstatusbar）：**空态 / 单行不换行 /
// 长文案 / 窄窗 / spinner 帧 / disabled 与真回传**。
//
// 官方依据全部用 **rc.2 asar 原件**复核过（不是 `tmp/asar-css` 的 rc.1 抽取）：
//
//	InputBar.module.css   .dock{justify-content:center;align-items:center;gap:12px;
//	                            max-width:100%;padding-top:4px;display:flex}
//	                      .root:not(.hero)>.dock{min-height:calc(22px + font-delta-secondary)}
//	StatsPills.module.css .anchor{font-size:calc(content-font-size-secondary - 1px);line-height:20px}
//	                      .pill{border-radius:radius-sm;max-width:100%;white-space:nowrap;gap:6px;
//	                            padding:1px 8px;display:inline-flex;align-items:center}
//	                      .pill svg{flex:none;width:14px;height:14px}
//	                      button.pill[aria-expanded=true]{background:interactive-bg-hover;
//	                            color:label-secondary}
//	                      @media (hover:hover){button.pill:hover{同上}}
//	                      .label{text-overflow:ellipsis;min-width:0;overflow:hidden}
//	ConnectionIndicator   ._indicator{grid-template-columns:14px max-content;column-gap:4px;
//	                            height:28px;padding:0 8px;border:1px solid transparent;
//	                            border-radius:radius-sm;font-size:12px;line-height:18px;
//	                            white-space:nowrap;animation:_indicator-enter .15s ease-out}
//	                      ._success{background:state-success-tertiary}
//	                      ._warning{background:state-warn-tertiary}
//	                      ._leaving{opacity:0}
package dsstatusbar

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const (
	vr3StatusTopOffset  float32 = 200
	vr3StatusBelowID            = "vr3-below"
	vr3StatusBelowLabel         = "vr3-below"
	vr3StatusMainWidth  float32 = 902
	vr3StatusWidth              = 1182
	vr3StatusHeight             = 832
	vr3StatusSideWidth          = 280
)

// vr3StatusHarness：两个真实座位一起画——composer 下方的 dock 与侧栏设置行的连接指示。
type vr3StatusHarness struct {
	t           *testing.T
	p           *Plugin
	session     *headless.Session
	invalidator app.Invalidator
	tree        *gfx.Tree
	feat        gfx.Scope
	theme       *pluginapi.Theme
	data        pluginapi.Snapshot
	now         time.Time
	reconnects  int
	mainWidth   float32
}

func vr3StatusDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("VR3_CAPTURE_DIR")
	if base == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

func vr3OpenStatus(t *testing.T, width, height int, mainWidth float32, data pluginapi.Snapshot) *vr3StatusHarness {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true) // official 档位：`composerSeatLive()` 真
	kernel := uikernel.New("vr3-statusbar")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	p := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatalf("装配 dsstatusbar 失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("vr3-statusbar")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})

	h := &vr3StatusHarness{
		t: t, p: p, tree: tree, feat: feat, theme: theme, data: data,
		mainWidth: mainWidth, now: time.Unix(0, 0),
	}
	// 指针/动画都要假时钟：连接指示的三档色调靠 150ms 离场/入场窗切换。
	p.now = func() time.Time { return h.now }

	config := app.Config{
		Title: "vr3-statusbar", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &h.invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: h.data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			MainWidth: mainWidth,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height,
				SidebarWidth: vr3StatusSideWidth, MainWidth: mainWidth,
			},
			Has: kernel.Has,
			Act: pluginapi.Actions{
				Reconnect:  func() { h.reconnects++ },
				Invalidate: func() { h.invalidator.Invalidate() },
			},
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillWidth().FillHeight().Children(func() {
				ui.Row(f).FillWidth().Height(vr3StatusTopOffset)
				ui.Column(f).Width(mainWidth).Children(func() {
					for _, painter := range kernel.Painters(uikernel.OffConvComposer) {
						if painter.Draw != nil {
							painter.Draw(f, env)
						}
					}
					// 下方占位：dock 占多少版面，用它的 y 量（0 = 整块不占位）。
					ui.Row(f).FillWidth().Height(98).Radius(28).Background(theme.BgLayer2).
						Hit(tree.ID(feat, vr3StatusBelowID), vr3StatusBelowLabel)
				})
				// 侧栏设置行（`SettingsRoot.triggerRow`，官方 280 宽）。
				ui.Row(f).Width(vr3StatusSideWidth).Height(connRowHeight + 2*connRowMarginY).Children(func() {
					for _, painter := range kernel.Painters(uikernel.OffSidebarSettings) {
						if painter.Draw != nil {
							painter.Draw(f, env)
						}
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
	h.session = session
	h.pump()
	h.pump()
	return h
}

func (h *vr3StatusHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

// settle 把连接指示的三档动画（入场/离场 150ms + connecting 最小可见 800ms）走完。
//
// 每轮把假时钟推 200ms 再画一帧：官方 `_indicator-enter .15s` 的起点是 opacity 0，
// 只画一帧会停在半透明上（那是"动画没走完"，不是"状态错了"）。
func (h *vr3StatusHarness) settle(frames int) {
	h.t.Helper()
	for i := 0; i < frames; i++ {
		h.now = h.now.Add(200 * time.Millisecond)
		h.pump()
	}
}

// moveAway 把指针移开：可点胶囊 hover 时底色是 color-mix(warn-tertiary, warn 6%)，
// 不挪开就会污染"底色 = state-warn-tertiary"的断言。
func (h *vr3StatusHarness) moveAway() {
	h.t.Helper()
	if err := h.session.Move(2, 2); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

func (h *vr3StatusHarness) setData(next pluginapi.Snapshot) {
	h.t.Helper()
	h.data = next
	// 7×200ms = 1400ms：官方连接态最小可见 800ms（`CONNECTING_MIN_VISIBLE_MS`）
	// + 150ms 离场窗 + 150ms 入场窗，一帧一帧走过去要够久，否则拍到的是上一态。
	h.settle(7)
}

func (h *vr3StatusHarness) hit(name string) (gfx.Hit, bool) {
	id := h.tree.ID(h.feat, name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func (h *vr3StatusHarness) below() gfx.Rect {
	h.t.Helper()
	hit, ok := h.hit(vr3StatusBelowID)
	if !ok {
		h.t.Fatalf("没有下方占位命中区")
	}
	return hit.Bounds
}

func (h *vr3StatusHarness) label(value string) (app.Label, bool) {
	for _, label := range h.session.Snapshot().Labels {
		if label.Value == value {
			return label, true
		}
	}
	return app.Label{}, false
}

// rectByColor 找本帧第一块指定颜色、指定高度的矩形（胶囊底）。
func (h *vr3StatusHarness) rectByColor(color gfx.Color, height float32) (gfx.Rect, bool) {
	for _, primitive := range h.session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Color != color {
			continue
		}
		if height > 0 && !vr3StatusClose(primitive.Bounds.Height, height, 0.5) {
			continue
		}
		return primitive.Bounds, true
	}
	return gfx.Rect{}, false
}

// rectCovering 找覆盖 point 的指定颜色矩形（hover / 展开态底色）。
func (h *vr3StatusHarness) rectCovering(point gfx.Point, color gfx.Color) (gfx.Rect, bool) {
	for _, primitive := range h.session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Color != color {
			continue
		}
		b := primitive.Bounds
		if point.X >= b.X && point.X <= b.X+b.Width && point.Y >= b.Y && point.Y <= b.Y+b.Height {
			return b, true
		}
	}
	return gfx.Rect{}, false
}

// spinnerSignature 是本帧 label-tertiary 描边的三角点串（spinner 角度不同 → 串不同）。
func (h *vr3StatusHarness) spinnerSignature() string {
	var parts []string
	for _, primitive := range h.session.Snapshot().Primitives {
		if primitive.Kind != app.TrianglePrimitive || primitive.Color != h.theme.LabelTertiary {
			continue
		}
		for _, point := range primitive.Points {
			parts = append(parts, itoa(int(math.Round(float64(point.X))))+","+itoa(int(math.Round(float64(point.Y)))))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

func (h *vr3StatusHarness) capture(name string) string {
	h.t.Helper()
	path := filepath.Join(vr3StatusDir(h.t), name)
	if err := h.session.Screenshot(path); err != nil {
		h.t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		h.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	h.t.Logf("VR3_STATUS_CAPTURE path=%s bytes=%d", path, info.Size())
	return path
}

func vr3StatusClose(got, want, tolerance float32) bool {
	return math.Abs(float64(got-want)) <= float64(tolerance)
}

// vr3StatusActivityStats 是一份"有投影"的 whole-log 统计（活动胶囊因此变成可点 DialogPill）。
func vr3StatusActivityStats(turns, steps, decodeTokens, decodeMs int64) *apiclient.SessionStats {
	return &apiclient.SessionStats{
		Turns: turns, Steps: steps, DecodeTokens: decodeTokens, DecodeMs: decodeMs,
		LLMMs: 1200, ToolMs: 400,
	}
}

// TestVR3StatusbarDockStates 覆盖：无会话空态、无活动（未知）、官方胶囊几何、
// 单行不换行、窄窗 900×700、长文案不省略、hover 态。
func TestVR3StatusbarDockStates(t *testing.T) {
	// ── ① 空会话（`Cur == ""`）── 官方槽是 session 作用域，什么都不该画 ──
	empty := vr3OpenStatus(t, vr3StatusWidth, vr3StatusHeight, vr3StatusMainWidth, pluginapi.Snapshot{})
	empty.settle(2)
	if empty.session.HasText("活动 未知") || empty.session.HasText("用量 未知") {
		t.Fatalf("空会话画出了读数；texts=%q", empty.session.Texts())
	}
	emptyBelow := empty.below()
	if !vr3StatusClose(emptyBelow.Y, vr3StatusTopOffset, 0.01) {
		t.Fatalf("空会话下方占位 y=%.2f，want %.2f（dock 不许留空白）", emptyBelow.Y, vr3StatusTopOffset)
	}
	empty.capture("statusbar-empty-session-1182x832.png")
	empty.session.Close()

	// ── ② 有会话、没有活动投影 ────────────────────────────────────────────
	// 本仓的显式差异（official_dock.go 文件头已登记）：结构位置存在的读数写「未知」，
	// 不写 0、也不整枚消失。官方 C20 是"缺真值的 pill 各自不渲染"。
	idle := vr3OpenStatus(t, vr3StatusWidth, vr3StatusHeight, vr3StatusMainWidth,
		pluginapi.Snapshot{Cur: "s-vr3", Link: pluginapi.LinkConnected})
	idle.settle(3)
	for _, want := range []string{zhActivityUnknown, zhUsageUnknown} {
		if !idle.session.HasText(want) {
			t.Fatalf("没有活动投影时缺 %q；texts=%q", want, idle.session.Texts())
		}
	}
	// `.dock{padding-top:4px}` + 非 hero `min-height:calc(22px + delta)` = 26：
	// 下方占位被推下 26（不多不少，也不换行成 52）。
	if got, want := idle.below().Y, emptyBelow.Y+26; !vr3StatusClose(got, want, 0.5) {
		t.Errorf("dock 占版面 %.2f，want %.2f（`.dock{padding-top:4px}` + `.pill{line-height:20px;padding:1px 8px}` = 22+4）",
			got-emptyBelow.Y, want-emptyBelow.Y)
	}
	// 没有投影时两枚都是静态 PlainPill：官方不给静态读数挂 Button，也就不该有命中区。
	if _, ok := idle.hit("activity-pill"); ok {
		t.Error("没有活动投影却给出了可点胶囊（官方只有 DialogPill 才是 button）")
	}
	idle.capture("statusbar-idle-1182x832.png")
	idle.session.Close()

	// ── ③ 有活动投影：官方胶囊几何（可点 DialogPill） ────────────────────
	stats := vr3StatusActivityStats(3, 7, 480, 1200) // 400 tok/s
	live := vr3OpenStatus(t, vr3StatusWidth, vr3StatusHeight, vr3StatusMainWidth, pluginapi.Snapshot{
		Cur: "s-vr3", Link: pluginapi.LinkConnected,
		SessionStats:  stats,
		UsageTotal:    pluginapi.Usage{InputTokens: 12000, OutputTokens: 345, TotalTokens: 12345},
		ContextTokens: 2000, ContextWindow: 10000,
	})
	defer live.session.Close()
	live.settle(3)
	pill, ok := live.hit("activity-pill")
	if !ok {
		t.Fatalf("有活动投影时没有可点胶囊（activity-pill）；texts=%q", live.session.Texts())
	}
	// `.pill{padding:1px 8px;line-height:20px}` → 高 = 20 + 2×1 = 22。
	if !vr3StatusClose(pill.Bounds.Height, pillHeight, 0.5) {
		t.Errorf("胶囊高 %.2f，want %.2f（`.pill{line-height:20px;padding:1px 8px}`）",
			pill.Bounds.Height, pillHeight)
	}
	// 活动读数文案（官方 `{turns} 轮 {steps} 步 · {tps} tok/s`）。
	activityText := "3 轮 7 步 · 400 tok/s"
	activityLabel, ok := live.label(activityText)
	if !ok {
		t.Fatalf("没画出活动读数 %q；texts=%q", activityText, live.session.Texts())
	}
	// `.pill{padding-inline:8px}` + `.pill svg{width:14px}` + `.pill{gap:6px}`：
	// 文字左沿 = 胶囊左沿 + 8 + 14 + 6 = +28。
	if want := pill.Bounds.X + pillPadX + StatsPillIconSize + pillGap; !vr3StatusClose(activityLabel.Bounds.X, want, 0.75) {
		t.Errorf("活动文字 x=%.2f，want %.2f（8 内边距 + 14 图标 + 6 间距）", activityLabel.Bounds.X, want)
	}
	// 胶囊宽 = 文字宽 + 2×8 + 14 + 6（官方 `.pill` 是 inline-flex，不收缩）。
	if want := activityLabel.Bounds.Width + 2*pillPadX + StatsPillIconSize + pillGap; !vr3StatusClose(pill.Bounds.Width, want, 1.5) {
		t.Errorf("胶囊宽 %.2f，want %.2f（文字 %.2f + 2×8 + 14 + 6）",
			pill.Bounds.Width, want, activityLabel.Bounds.Width)
	}
	// 第二枚（用量）：`.dock{gap:12px}`，之后是它自己的 `padding-left:8px` + 14px 图标 + 6 gap，
	// 所以文字左沿 = 第一枚右沿 + 12 + 8 + 14 + 6 = +40。
	usageText := "总量 12.3k"
	usageLabel, ok := live.label(usageText)
	if !ok {
		t.Fatalf("没画出用量读数 %q；texts=%q", usageText, live.session.Texts())
	}
	wantUsageX := pill.Bounds.X + pill.Bounds.Width + dockGap + pillPadX + StatsPillIconSize + pillGap
	if !vr3StatusClose(usageLabel.Bounds.X, wantUsageX, 0.75) {
		t.Errorf("用量文字 x=%.2f，want %.2f（第一枚右沿 %.2f + gap 12 + 内边距 8 + 图标 14 + 间距 6）",
			usageLabel.Bounds.X, wantUsageX, pill.Bounds.X+pill.Bounds.Width)
	}
	// 两枚在**同一行**（官方 `.dock` 是 nowrap 的 flex 单行，不换行成两行）。
	if !vr3StatusClose(activityLabel.Bounds.Y, usageLabel.Bounds.Y, 0.75) {
		t.Errorf("两枚胶囊不在同一行：活动 y=%.2f 用量 y=%.2f（`.dock{display:flex}` 单行）",
			activityLabel.Bounds.Y, usageLabel.Bounds.Y)
	}
	// 整行在主列里居中（`.dock{justify-content:center}`）。
	rowLeft := pill.Bounds.X
	rowRight := usageLabel.Bounds.X + usageLabel.Bounds.Width + pillPadX
	if want := live.mainWidth / 2; !vr3StatusClose((rowLeft+rowRight)/2, want, 1) {
		t.Errorf("读数行中心 %.2f，want %.2f（`.dock{justify-content:center}`）", (rowLeft+rowRight)/2, want)
	}
	live.capture("statusbar-live-1182x832.png")

	// ── ④ hover 态：`@media (hover:hover){button.pill:hover{background:interactive-bg-hover}}` ──
	if err := live.session.Move(pill.Bounds.X+pill.Bounds.Width/2, pill.Bounds.Y+pill.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	live.pump()
	live.pump()
	if _, ok := live.rectCovering(gfx.Point{X: pill.Bounds.X + 4, Y: pill.Bounds.Y + pill.Bounds.Height/2}, live.theme.Hover); !ok {
		t.Error("悬停可点胶囊没有 interactive-bg-hover 底色（官方 `button.pill:hover`）")
	}
	live.capture("statusbar-live-hover-1182x832.png")
	if err := live.session.Move(2, 2); err != nil {
		t.Fatal(err)
	}
	live.pump()
	live.pump()

	// ── ⑤ 长文案：不许省略成 "…"，也不许换行 ─────────────────────────────
	longStats := vr3StatusActivityStats(12345, 6789, 1234567, 1000) // 1.2M tok/s
	long := vr3OpenStatus(t, vr3StatusWidth, vr3StatusHeight, vr3StatusMainWidth, pluginapi.Snapshot{
		Cur: "s-vr3", Link: pluginapi.LinkConnected, SessionStats: longStats,
		UsageTotal: pluginapi.Usage{TotalTokens: 987654321},
	})
	defer long.session.Close()
	long.settle(3)
	longText := "12345 轮 6789 步 · 1234567 tok/s"
	longLabel, ok := long.label(longText)
	if !ok {
		t.Fatalf("长读数被改写了（want 完整 %q）；texts=%q", longText, long.session.Texts())
	}
	if longLabel.Bounds.Y != long.labelY() { // 单行：所有胶囊文字同一 y
		t.Errorf("长读数换行了：y=%.2f，整行基线 %.2f", longLabel.Bounds.Y, long.labelY())
	}
	// `.pill{white-space:nowrap}` + `.label{text-overflow:ellipsis}`：宽度不够时省略，
	// 但主列 902 宽放得下 → 必须是完整文案（不是 "12345 轮 6789 步 · 12…"）。
	for _, label := range long.session.Snapshot().Labels {
		if strings.HasSuffix(label.Value, "…") {
			t.Errorf("主列放得下却出现了省略号：%q", label.Value)
		}
	}
	long.capture("statusbar-long-1182x832.png")

	// ── ⑥ 窄窗 900×700：单行、不越界、仍居中 ────────────────────────────
	narrow := vr3OpenStatus(t, 900, 700, 620, pluginapi.Snapshot{
		Cur: "s-vr3", Link: pluginapi.LinkConnected, SessionStats: stats,
		UsageTotal: pluginapi.Usage{TotalTokens: 12345},
	})
	defer narrow.session.Close()
	narrow.settle(3)
	snapshot := narrow.session.Snapshot()
	for _, label := range snapshot.Labels {
		if label.Bounds.X < -0.5 || label.Bounds.X+label.Bounds.Width > float32(snapshot.Width)+0.5 ||
			label.Bounds.Y < -0.5 || label.Bounds.Y+label.Bounds.Height > float32(snapshot.Height)+0.5 {
			t.Errorf("窄窗里文字 %q 越界：x=%.2f..%.2f y=%.2f..%.2f（窗口 %dx%d）",
				label.Value,
				label.Bounds.X, label.Bounds.X+label.Bounds.Width,
				label.Bounds.Y, label.Bounds.Y+label.Bounds.Height,
				snapshot.Width, snapshot.Height)
		}
	}
	narrowActivity, ok := narrow.label("3 轮 7 步 · 400 tok/s")
	if !ok {
		t.Fatalf("窄窗里活动读数丢了；texts=%q", narrow.session.Texts())
	}
	narrowUsage, ok := narrow.label("总量 12.3k")
	if !ok {
		t.Fatalf("窄窗里用量读数丢了；texts=%q", narrow.session.Texts())
	}
	if !vr3StatusClose(narrowActivity.Bounds.Y, narrowUsage.Bounds.Y, 0.75) {
		t.Errorf("窄窗把读数挤成了两行：活动 y=%.2f 用量 y=%.2f（官方 `.dock` 是单行 flex）",
			narrowActivity.Bounds.Y, narrowUsage.Bounds.Y)
	}
	narrow.capture("statusbar-narrow-900x700.png")
}

// labelY 返回本帧第一个胶囊文字的基线 y（用来判"有没有换行"）。
func (h *vr3StatusHarness) labelY() float32 {
	for _, label := range h.session.Snapshot().Labels {
		if strings.HasSuffix(label.Value, "tok/s") || strings.HasPrefix(label.Value, "总量 ") ||
			strings.HasPrefix(label.Value, "活动 ") || strings.HasPrefix(label.Value, "用量 ") {
			return label.Bounds.Y
		}
	}
	return 0
}

// TestVR3StatusbarConnectionStates 覆盖：连接三档色调与几何、spinner 真动画帧、
// 可点胶囊的真回传（Actions.Reconnect）、以及 150ms 离场窗不闪。
func TestVR3StatusbarConnectionStates(t *testing.T) {
	h := vr3OpenStatus(t, vr3StatusWidth, vr3StatusHeight, vr3StatusMainWidth,
		pluginapi.Snapshot{Cur: "s-vr3", Link: pluginapi.LinkConnected})
	defer h.session.Close()
	h.settle(4)

	// ── ① connected：官方 `._success{background:state-success-tertiary}` ──
	// `._indicator{height:28px;border:1px solid transparent;box-sizing:border-box}`：
	// 底色矩形是描边内框，所以是 28-2×1 高、左右各内缩 1。
	connectedPill, ok := h.rectByColor(h.theme.SuccessTertiary, connIndicatorHeight-2*connIndicatorBorder)
	if !ok {
		t.Fatalf("连接成功态没画出 success 胶囊（%v 底 / %.0f 高）；texts=%q",
			h.theme.SuccessTertiary, connIndicatorHeight, h.session.Texts())
	}
	// `._indicator{height:28px}`。
	if !vr3StatusClose(connectedPill.Height, connIndicatorHeight-2*connIndicatorBorder, 0.5) {
		t.Errorf("连接胶囊底色高 %.2f，want %.0f（`._indicator{height:28px}` - 2×1px border）",
			connectedPill.Height, connIndicatorHeight-2*connIndicatorBorder)
	}
	connectedText, ok := h.label(zhConnectionConnected)
	if !ok {
		t.Fatalf("连接成功态没画出 %q；texts=%q", zhConnectionConnected, h.session.Texts())
	}
	// `._indicator{display:inline-grid;grid-template-columns:14px max-content;
	//   column-gap:4px;padding:0 8px;border:1px solid transparent;box-sizing:border-box}`：
	// 列只有两列（14 图标 + max-content 文案），**列间距只加一次**，
	// 所以文案左沿（相对**描边盒左沿**）= 1 + 8 + 14 + 4 = 27。
	// 注意 `connectedPill` 是描边**内框**矩形，它的 X 已经把 1px 描边算进去了
	// （实测 X=1），再补一次 border 就会多算 1px —— 第一版这里写 28 是公式错，
	// 不是产品差 1px。
	if want := connectedPill.X + connIndicatorPadX + connectionIconSize + connIndicatorGap; !vr3StatusClose(connectedText.Bounds.X, want, 0.75) {
		t.Errorf("连接文案 x=%.2f，want %.2f（官方 grid 两列：8 内边距 + 14 图标 + 4 列间距，只加一次）",
			connectedText.Bounds.X, want)
	}
	if want := connectedText.Bounds.Width + 2*connIndicatorPadX + connectionIconSize + connIndicatorGap; !vr3StatusClose(connectedPill.Width, want, 1.5) {
		t.Errorf("连接胶囊内宽 %.2f，want %.2f（文字 %.2f + 8+14+4+8；差额是 flex Gap 在占位盒两侧各加了一次）",
			connectedPill.Width, want, connectedText.Bounds.Width)
	}
	// `._indicator{font-size:12px;line-height:18px}`。
	if connectedText.FontSize != connIndicatorFont {
		t.Errorf("连接文案字号 %.1f，want %.1f（`._indicator{font-size:12px}`）",
			connectedText.FontSize, connIndicatorFont)
	}
	h.capture("statusbar-connection-connected-1182x832.png")

	// ── ② connecting：warning 底 + spinner + 可点（官方 `<button onClick={onReconnect}>`）──
	h.setData(pluginapi.Snapshot{Cur: "s-vr3", Link: pluginapi.LinkConnecting})
	if _, ok := h.rectByColor(h.theme.WarnTertiary, connIndicatorHeight-2*connIndicatorBorder); !ok {
		t.Fatalf("重新连接态没画出 warn 胶囊；texts=%q", h.session.Texts())
	}
	wantConnecting := zhConnectionConnecting + zhConnectionDots
	if !h.session.HasText(wantConnecting) {
		t.Fatalf("重新连接态缺 %q；texts=%q", wantConnecting, h.session.Texts())
	}
	// spinner 真动画：官方是 1.5s 旋转（合成动画），本仓按 33ms 排帧。
	// 判据 = 相隔 120ms 的两帧里，描边的三角点串必须不同（角度变了）。
	first := h.spinnerSignature()
	if first == "" {
		t.Fatal("connecting 态没有任何 label-tertiary 描边（spinner 没画）")
	}
	h.now = h.now.Add(120 * time.Millisecond)
	h.pump()
	second := h.spinnerSignature()
	if first == second {
		t.Errorf("spinner 两帧完全一样（相隔 120ms）：实测帧串 %q —— 官方是 1.5s 连续旋转", second)
	}
	h.capture("statusbar-connection-connecting-1182x832.png")

	// 真回传：点 warning 胶囊必须调到 `Actions.Reconnect`（官方 onClick={onReconnect}）。
	pill, ok := h.hit("connection-reconnect")
	if !ok {
		t.Fatal("warning 态没有 connection-reconnect 命中区（官方 warning 是 button）")
	}
	before := h.reconnects
	if err := h.session.Move(pill.Bounds.X+pill.Bounds.Width/2, pill.Bounds.Y+pill.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := h.session.Click(pill.Bounds.X+pill.Bounds.Width/2, pill.Bounds.Y+pill.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if h.reconnects != before+1 {
		t.Errorf("点了重连胶囊，Actions.Reconnect 调用数 %d → %d（want +1）", before, h.reconnects)
	}
	h.moveAway()

	// ── ③ down：warning 底 + 官方 error 文案 ─────────────────────────────
	h.setData(pluginapi.Snapshot{Cur: "s-vr3", Link: pluginapi.LinkDown, Status: "连不上宿主"})
	if _, ok := h.rectByColor(h.theme.WarnTertiary, connIndicatorHeight-2*connIndicatorBorder); !ok {
		t.Fatalf("断线态没画出 warn 胶囊；texts=%q", h.session.Texts())
	}
	if !h.session.HasText(zhConnectionError) {
		t.Fatalf("断线态缺 %q；texts=%q", zhConnectionError, h.session.Texts())
	}
	h.capture("statusbar-connection-down-1182x832.png")

	// ── ④ 离场窗：官方 `._leaving{opacity:0}` + `transition:opacity .15s` ──
	// 目标态一到，旧态还要再画 150ms 才卸载；这一帧不许"闪成空白"。
	h.setData(pluginapi.Snapshot{Cur: "s-vr3", Link: pluginapi.LinkConnected})
	h.now = h.now.Add(60 * time.Millisecond) // 落在 150ms 离场窗里
	h.pump()
	if !h.session.HasText(zhConnectionError) && !h.session.HasText(zhConnectionConnected) {
		t.Errorf("离场窗里两侧文案都消失了（闪空）；texts=%q", h.session.Texts())
	}
	h.capture("statusbar-connection-leaving-1182x832.png")
}
