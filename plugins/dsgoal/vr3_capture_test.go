//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计（dsgoal）：**空态 / 长文案 / 清空**。
//
// 前两轮只验过「有目标时目标条画在哪」，这一轮补三件没看过的：
//
//	① 空态是"整块不出现"还是"留了空白占位"？
//	   官方 `GoalBar.tsx:207`：`if (goal === void 0 || goal === null ||
//	   goal.phase === "complete" || goal.id === clearedGoalId) return null;`
//	   —— React 返回 null 就是**连 DOM 都不创建**，所以既不该有条，也不该留 36px 空位。
//	   官方 CSS 侧同一结论：`.Gc24Bq_dock{margin:0 auto}`（`GoalBar.module.css:1`）
//	   没有 min-height / padding，空 div 高度也是 0。
//	   本测试的判据不是"图里没有目标条"，而是**下方的输入卡占位块没有被推下去**：
//	   空态时占位块与无目标条时同 y，有目标时恰好下移 36px（`.bar{height:36px}`）。
//
//	② 超长 objective 是不是**单行省略**？
//	   官方 `.Gc24Bq_objective{min-width:0;text-overflow:ellipsis;white-space:nowrap;
//	   flex:1;font-size:13px;line-height:20px;overflow:hidden}`（`GoalBar.module.css:1`）
//	   —— 单行 + 省略号 + 不越过 bar 右内边距（`.bar{padding:4px 5px 4px 12px}`）。
//
//	③ 目标被清空（goal 变回空串）之后，这一块必须**整块消失并让位**，
//	   不能留一条"上一帧的条"或一条空占位。
//
// 数值断言逐条对着官方 CSS 的具体值，出处写在断言旁边；截图落 `$VR3_CAPTURE_DIR`。
package dsgoal

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// vr3GoalDir 是截图目录（`VR3_CAPTURE_DIR`；未设置时用临时目录）。
func vr3GoalDir(t *testing.T) string {
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

// vr3GoalHarness 是一帧可换数据的官方座位：`conversation.composer`（official 档位）。
//
// 版面是**真实布局**：顶部 200px 空白 → 目标条座位 → 98px 输入卡占位（带命中区）。
// 占位块的 y 就是"目标条占了多少版面"的唯一可测读数。
type vr3GoalHarness struct {
	t           *testing.T
	session     *headless.Session
	invalidator app.Invalidator
	tree        *gfx.Tree
	feat        gfx.Scope
	theme       *pluginapi.Theme
	mainWidth   float32
	topOffset   float32
	goal        string
}

const (
	vr3GoalTopOffset      float32 = 200 // 顶部空白：目标条 y 的锚
	vr3GoalComposerHeight float32 = 98  // 与 dsc omposer 卡片同档的占位高度
	vr3GoalMarkerID               = "vr3-composer-marker"
	vr3GoalMarkerLabel            = "vr3-composer"
)

// vr3OpenGoal 装配一台真 harness（官方座位 + 官方档位 + 真命中区）。
func vr3OpenGoal(t *testing.T, dir string, width, height int, mainWidth float32) *vr3GoalHarness {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true) // official 档位：`composerSeatLive()` 真
	kernel := uikernel.New("vr3-goal")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配 dsgoal 失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("vr3-goal")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})

	h := &vr3GoalHarness{t: t, tree: tree, feat: feat, theme: theme, mainWidth: mainWidth, topOffset: vr3GoalTopOffset}
	config := app.Config{
		Title: "vr3-goal", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &h.invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: pluginapi.Snapshot{Cur: "s-vr3"},
			Act:  pluginapi.Actions{Goal: func() string { return h.goal }},
			Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			MainWidth: mainWidth,
			Geometry:  pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: mainWidth},
			Has:       kernel.Has,
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillWidth().FillHeight().Children(func() {
				ui.Row(f).FillWidth().Height(vr3GoalTopOffset)
				// 座位本身：与壳一样按 Order 派发 `conversation.composer`。
				ui.Column(f).Width(mainWidth).Children(func() {
					for _, painter := range kernel.Painters(uikernel.OffConvComposer) {
						if painter.Draw != nil {
							painter.Draw(f, env)
						}
					}
					// 输入卡片占位：目标条该在它**上方**，且空态时不该把它推下去。
					ui.Row(f).FillWidth().Height(vr3GoalComposerHeight).Radius(28).
						Background(theme.InputMajor).
						Hit(tree.ID(feat, vr3GoalMarkerID), vr3GoalMarkerLabel)
				})
			})
		})
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

func (h *vr3GoalHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

// setGoal 换目标并画两帧（第一帧是数据变更，第二帧是稳定后的画面）。
func (h *vr3GoalHarness) setGoal(goal string) {
	h.t.Helper()
	h.goal = goal
	h.pump()
	h.pump()
}

// marker 返回输入卡占位块的矩形（真实命中区）。
func (h *vr3GoalHarness) marker() gfx.Rect {
	h.t.Helper()
	id := h.tree.ID(h.feat, vr3GoalMarkerID)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit.Bounds
		}
	}
	h.t.Fatalf("没有输入卡占位命中区（标记块没画出来）")
	return gfx.Rect{}
}

// bar 返回本帧目标条的矩形；没有目标条时 ok=false。
//
// 判据与官方 CSS 同一组值：`.bar{height:36px;border-radius:radius-md}`
// + `:before{background:--dsw-specific-menu}`（`GoalBar.module.css:1`）。
func (h *vr3GoalHarness) bar() (gfx.Rect, bool) {
	for _, primitive := range h.session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive {
			continue
		}
		if primitive.Color == h.theme.Menu && primitive.Radius == h.theme.Radii.MD &&
			primitive.Bounds.Height == goalBarHeight {
			return primitive.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// label 按文字内容找本帧**真正画出来**的那一行（`Labels` 是画出来的，含省略化结果）。
func (h *vr3GoalHarness) label(value string) (app.Label, bool) {
	for _, label := range h.session.Snapshot().Labels {
		if label.Value == value {
			return label, true
		}
	}
	return app.Label{}, false
}

// labelsWithPrefix 找所有以 prefix 开头的已画文字（用来判断"单行还是多行/省略"）。
func (h *vr3GoalHarness) labelsWithPrefix(prefix string) []app.Label {
	var out []app.Label
	for _, label := range h.session.Snapshot().Labels {
		if strings.HasPrefix(label.Value, prefix) {
			out = append(out, label)
		}
	}
	return out
}

func (h *vr3GoalHarness) capture(name string) string {
	h.t.Helper()
	path := filepath.Join(vr3GoalDir(h.t), name)
	if err := h.session.Screenshot(path); err != nil {
		h.t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		h.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	h.t.Logf("VR3_GOAL_CAPTURE path=%s bytes=%d", path, info.Size())
	return path
}

func vr3CloseEnough(got, want, tolerance float32) bool {
	return math.Abs(float64(got-want)) <= float64(tolerance)
}

// iconInkWidth 返回本帧 label-tertiary 描边墨迹的横向包围盒宽度（官方 goal-outline 图标）。
//
// 只聚合 label-tertiary 的三角形：36px 条自己的描边是 BorderL1，不会混进来。
func (h *vr3GoalHarness) iconInkWidth() (float32, bool) {
	minX, maxX, seen := float32(0), float32(0), false
	for _, primitive := range h.session.Snapshot().Primitives {
		if primitive.Kind != app.TrianglePrimitive || primitive.Color != h.theme.LabelTertiary {
			continue
		}
		for _, point := range primitive.Points {
			if !seen {
				minX, maxX, seen = point.X, point.X, true
				continue
			}
			if point.X < minX {
				minX = point.X
			}
			if point.X > maxX {
				maxX = point.X
			}
		}
	}
	if !seen {
		return 0, false
	}
	return maxX - minX, true
}

// TestVR3GoalEmptyStateAndOverflow 验空态（整块不出现）、官方几何、长文案单行省略、
// 以及清空之后整块消失并让位。
func TestVR3GoalEmptyStateAndOverflow(t *testing.T) {
	const (
		width     = 1182
		height    = 832
		mainWidth = 902
	)
	h := vr3OpenGoal(t, vr3GoalDir(t), width, height, mainWidth)
	defer h.session.Close()

	// ── ① 空态：goal == "" ────────────────────────────────────────────────
	// 官方 `GoalBar.tsx:207` 返回 null → 什么都不创建。
	h.setGoal("")
	if _, ok := h.bar(); ok {
		t.Fatal("目标为空却画出了 36px 目标条（官方 GoalBar.tsx:207 `goal === null` 时 return null）")
	}
	if h.session.HasText(goalLabelText) {
		t.Fatalf("目标为空却画出了标签 %q；texts=%q", goalLabelText, h.session.Texts())
	}
	emptyMask := h.marker()
	if !vr3CloseEnough(emptyMask.Y, vr3GoalTopOffset, 0.01) {
		t.Fatalf("空态输入卡占位 y=%.2f，want %.2f（目标条不许留空白占位：官方 return null 不建 DOM/不占版面）",
			emptyMask.Y, vr3GoalTopOffset)
	}
	emptyShot := h.capture("goal-empty-1182x832.png")

	// ── ② 有目标：官方 36px 条 + 内容宽 clamp + 内边距 ──────────────────
	h.setGoal("把 DSH 重写成 Go 版本，界面用 dsh-ui")
	bar, ok := h.bar()
	if !ok {
		t.Fatalf("有目标却没画出目标条；texts=%q", h.session.Texts())
	}
	// `GoalBar.module.css:1` `.bar{height:36px}`。
	if bar.Height != goalBarHeight {
		t.Fatalf("目标条高 %.2f，want %.2f（GoalBar.module.css:1 `.bar{height:36px}`）", bar.Height, goalBarHeight)
	}
	// 宽度 = `--dsh-chat-content-width` = clamp(680, 主列宽×.64, 920)：
	// 902×0.64 = 577.28 → 取 680（官方 `.bar{max-width:calc(card-max-width - 4*inset)}`）。
	wantBarWidth := float32(680)
	if !vr3CloseEnough(bar.Width, wantBarWidth, 0.5) {
		t.Fatalf("目标条宽 %.2f，want %.2f（clamp(680, %v×0.64, 920)）", bar.Width, wantBarWidth, mainWidth)
	}
	// `.dock{margin:0 auto}` + `.bar{margin:0 auto}`：在主列里水平居中。
	wantBarX := (mainWidth - wantBarWidth) / 2
	if !vr3CloseEnough(bar.X, wantBarX, 0.5) {
		t.Fatalf("目标条 x=%.2f，want %.2f（主列 %d 内居中）", bar.X, wantBarX, mainWidth)
	}
	// 有目标条时输入卡片恰好被推下 36px（顺序 = 位置，且高度就是 .bar 的 36）。
	activeMask := h.marker()
	if !vr3CloseEnough(activeMask.Y, emptyMask.Y+goalBarHeight, 0.01) {
		t.Errorf("有目标时占位块 y=%.2f，want %.2f（空态 %.2f + 条高 %.1f）",
			activeMask.Y, emptyMask.Y+goalBarHeight, emptyMask.Y, goalBarHeight)
	}

	// `.label{font-size:13px;font-weight:500;line-height:24px;color:label-primary}`。
	label, ok := h.label(goalLabelText)
	if !ok {
		t.Fatalf("没有画出标签 %q；labels=%v", goalLabelText, vr3GoalLabelValues(h.session.Snapshot().Labels))
	}
	if label.FontSize != goalLabelFont {
		t.Errorf("标签字号 %.1f，want %.1f（GoalBar.module.css:1 `.label{font-size:13px}`）", label.FontSize, goalLabelFont)
	}
	if label.Color != h.theme.LabelPrimary {
		t.Errorf("标签颜色 %v，want label-primary %v（`.label{color:var(--dsw-alias-label-primary)}`）",
			label.Color, h.theme.LabelPrimary)
	}
	// `.bar{padding:4px 5px 4px 12px}` + `.bar{gap:10px}` + glyph 14（GoalBar.tsx:137 `size={14}`）：
	// 标签左沿 = 条左沿 + 12 + 14 + 10。
	wantLabelX := bar.X + goalBarPadLeft + goalGlyphSize + goalBarGap
	if !vr3CloseEnough(label.Bounds.X, wantLabelX, 0.75) {
		t.Errorf("标签 x=%.2f，want %.2f（bar.x %.2f + 左内边距 12 + glyph 14 + gap 10）",
			label.Bounds.X, wantLabelX, bar.X)
	}
	h.capture("goal-active-1182x832.png")

	// ── ③ 超长 objective：单行省略，且不越过右内边距 ────────────────────
	long := strings.Repeat("把整个 DSH 宿主重写成 Go 版本并保持官方交互语义一致", 12)
	h.setGoal(long)
	bar, ok = h.bar()
	if !ok {
		t.Fatal("超长目标时目标条消失了")
	}
	// 仍是单行 36px：`white-space:nowrap` + `.bar{height:36px}` 都不允许长高。
	if bar.Height != goalBarHeight {
		t.Errorf("超长目标把条撑到 %.2f 高，want %.2f（`.objective{white-space:nowrap}` 单行）", bar.Height, goalBarHeight)
	}
	drawn := h.labelsWithPrefix(string([]rune(long)[:12]))
	if len(drawn) != 1 {
		t.Fatalf("超长目标画出了 %d 行（want 1 行 + 省略号）：%v", len(drawn), vr3GoalLabelValues(drawn))
	}
	if !strings.HasSuffix(drawn[0].Value, "…") {
		t.Errorf("超长目标没有被省略：画出来的是 %q（官方 `.objective{text-overflow:ellipsis}`）", drawn[0].Value)
	}
	// `.objective` 颜色 label-primary-dimmed。
	if drawn[0].Color != h.theme.LabelPrimaryDimmed {
		t.Errorf("目标文本颜色 %v，want label-primary-dimmed %v", drawn[0].Color, h.theme.LabelPrimaryDimmed)
	}
	// `.objective{font-size:13px;line-height:20px}`。
	if drawn[0].FontSize != goalObjectiveFont {
		t.Errorf("目标文字字号 %.1f，want %.1f（`.objective{font-size:13px}`）", drawn[0].FontSize, goalObjectiveFont)
	}
	// `.bar{padding-right:5px}`：文字右沿不许越过条右沿 - 5。
	if right := drawn[0].Bounds.X + drawn[0].Bounds.Width; right > bar.X+bar.Width-goalBarPadRight+0.75 {
		t.Errorf("目标文字右沿 %.2f 越过条右沿内边距（条 %.2f..%.2f，padding-right 5）",
			right, bar.X, bar.X+bar.Width)
	}

	// 长目标下**固定件**不能被挤：官方 `.label{flex:none}` 与 `.goalGlyph{flex:none}`
	// （`GoalBar.module.css:1`）——只有 `.objective{flex:1;min-width:0}` 可收缩。
	//
	// 判据用**同一帧的自校准**：短目标那一帧标签已经按内容画到了自然宽度，
	// 长目标这一帧它必须还是同一个宽度与同一个 x（官方 flex:none 就是"尺寸不随兄弟变"）。
	longLabel, ok := h.label(goalLabelText)
	if !ok {
		t.Errorf("长目标下固定标签不见了；labels=%v", vr3GoalLabelValues(h.session.Snapshot().Labels))
	} else {
		if longLabel.Value != label.Value {
			t.Errorf("长目标把固定标签改写了：%q，want %q", longLabel.Value, label.Value)
		}
		if !vr3CloseEnough(longLabel.Bounds.Width, label.Bounds.Width, 0.5) {
			t.Errorf("长目标把固定标签挤窄：宽 %.2f，短目标时 %.2f（官方 `.label{flex:none}` 不收缩）"+
				"；clipped=%v bounds=%+v",
				longLabel.Bounds.Width, label.Bounds.Width, longLabel.Clipped, longLabel.Bounds)
		}
		if !vr3CloseEnough(longLabel.Bounds.X, label.Bounds.X, 0.5) {
			t.Errorf("长目标把固定标签挪位：x=%.2f，短目标时 %.2f", longLabel.Bounds.X, label.Bounds.X)
		}
	}
	// 图标墨迹宽度直接问 sprite 自己（与 TestGoalBarHeadlessCapture 同一算法，
	// 不写死像素）：官方 sprite 的墨迹比 14px 框小。
	_, _, wantInkW, _ := iconInk(goalOutlineIcon(), goalGlyphSize)
	if inkW, ok := h.iconInkWidth(); !ok {
		t.Error("长目标下找不到 goal-outline 图标墨迹")
	} else if !vr3CloseEnough(inkW, wantInkW, 1.5) {
		t.Errorf("长目标下图标墨迹宽 %.2f，want ≈%.2f（官方 `.goalGlyph{flex:none}` + size=14）",
			inkW, wantInkW)
	}
	h.capture("goal-long-objective-1182x832.png")

	// ── ④ 清空：整块消失并让位 ──────────────────────────────────────────
	h.setGoal("")
	if _, ok := h.bar(); ok {
		t.Fatal("目标清空后目标条还在（官方 `goal.id === clearedGoalId` 也会 return null）")
	}
	if back := h.marker(); !vr3CloseEnough(back.Y, emptyMask.Y, 0.01) {
		t.Errorf("清空后占位块 y=%.2f，want 回到 %.2f（不许留空白占位）", back.Y, emptyMask.Y)
	}
	h.capture("goal-cleared-1182x832.png")

	// ── ⑤ 窄窗 900×700：主列 620 时内容宽仍按 clamp 取 680，不能挤出窗口 ─
	narrow := vr3OpenGoal(t, vr3GoalDir(t), 900, 700, 620)
	defer narrow.session.Close()
	narrow.setGoal("窄窗里的目标")
	bar, ok = narrow.bar()
	if !ok {
		t.Fatal("窄窗里没有画出目标条")
	}
	// 两级宽度按官方两条 CSS 逐级算，不写死一个数：
	//   `.dock{width:calc(100% - 2*clearance - 4*inset)}` → 座位扣完左右各 2*inset=16
	//   `.bar{width:100%;max-width:calc(card-max-width - 4*inset)}` → 填满 dock，再被 clamp 住
	// 主列 620 时 dock 宽 620-32=588 < clamp 的 680，所以条宽是 588（不是 680）。
	wantNarrow := float32(680)
	if avail := float32(620) - 2*goalDockMargin; avail < wantNarrow {
		wantNarrow = avail
	}
	if !vr3CloseEnough(bar.Width, wantNarrow, 0.5) {
		t.Errorf("窄窗目标条宽 %.2f，want %.2f（dock=主列 %.0f - 4*inset=%.0f，再与 clamp(680, …) 取小）",
			bar.Width, wantNarrow, float32(620), 2*goalDockMargin)
	}
	// 主列放得下时条宽就是 clamp 的 680 —— 那一支在 ② 里已经验过（wantBarWidth）。

	narrow.capture("goal-narrow-900x700.png")
	// 画出来的任何东西都不许超出窗口。
	snapshot := narrow.session.Snapshot()
	for _, label := range snapshot.Labels {
		if label.Bounds.X < -0.5 || label.Bounds.X+label.Bounds.Width > float32(snapshot.Width)+0.5 {
			t.Errorf("窄窗里文字 %q 越界：x=%.2f..%.2f（窗口宽 %d）",
				label.Value, label.Bounds.X, label.Bounds.X+label.Bounds.Width, snapshot.Width)
		}
	}

	t.Logf("VR3_GOAL shots=%s goal-empty/active/long-objective/cleared/narrow", emptyShot)
}

func vr3GoalLabelValues(labels []app.Label) []string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		out = append(out, label.Value)
	}
	return out
}
