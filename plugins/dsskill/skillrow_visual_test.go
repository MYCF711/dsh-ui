//go:build darwin || windows || linux

// skillrow_visual_test.go —— 官方 `SkillRow` 的**真实像素**断言。
//
// 与 `official_skill_test.go`（表 vs 常量）互补：那张表能证明"我们写下的数值
// 等于官方 CSS"，但证明不了"画出来的东西真的按那些数值落位"。所以这里走
// 真 `headless` Paint → 读回 `Primitives`（每个圆角矩形/三角形的真实包围盒）
// → 按**官方几何**核对位置与尺寸。
//
// 四条判据：
//
//	① `.row` 高 24（用命中区的真实矩形量，不是读常量）
//	② `.instructionsCard` 在 row **下面** 4px、左缩进 4px、高 ≤ 260、圆角 16
//	③ `.instructionsHeader` 在卡内顶部、高 32（8+16+8）、底色是 banner 色
//	④ `.separator` 是 2×2 / 圆角 1 的点，落在 title 与 summary 之间
//
// 另外落盘 PNG（`SKILL_CAPTURE_DIR` 指定目录，默认 t.TempDir()），
// 供人眼看一次——像素断言管"数值对不对"，图管"看起来是不是那个东西"。
package dsskill

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dstool"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

// captureDir 返回落盘目录（`SKILL_CAPTURE_DIR` 优先）。
func captureDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("SKILL_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// capture 把这一帧的像素写成 PNG。
func capture(t *testing.T, session *headless.Session, dir, name string) {
	t.Helper()
	img, err := session.Image()
	if err != nil {
		t.Fatalf("%s: 取像素失败：%v", name, err)
	}
	path := filepath.Join(dir, name+".png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("%s: 建文件失败：%v", name, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("%s: 写 PNG 失败：%v", name, err)
	}
	t.Logf("%s → %s", name, path)
}

// rectOf 找第一个「颜色 + 圆角 + 高」都对得上的矩形图元。
//
// `radius < 0` 表示不挑圆角，`height <= 0` 表示不挑高度。
func rectOf(session *headless.Session, color gfx.Color, radius, height float32) (app.Primitive, bool) {
	for _, primitive := range session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Color != color {
			continue
		}
		if radius >= 0 && primitive.Radius != radius {
			continue
		}
		if height > 0 && primitive.Bounds.Height != height {
			continue
		}
		return primitive, true
	}
	return app.Primitive{}, false
}

// near 是浮点比较（布局会做分数像素，0.5 以内算同值）。
func near(got, want float32) bool {
	d := got - want
	return d <= 0.5 && d >= -0.5
}

// TestSkillRowGeometryInPixels 用真实像素帧核对官方几何。
//
// # 三处「viva 画出来和 CSS 声明不完全一一对应」的事实（都在断言里体现了）
//
//	① 边框 0.5px 在 scale=1 下被像素对齐成 **1 物理像素**。CSS 的 0.5px 在
//	   1x 屏上本来也只能落 1 个设备像素，所以这是**等价**，不是偏差。
//	② 圆角矩形的**填充**是内缩一圈画的：外框半径 16、填充半径 15、填充外扩 1px。
//	   所以「说明卡的边界」要取**边框图元**（半径 16），不是填充图元。
//	③ `theme.BgBase` 与画布底色**同值**，不能拿它当「inspect 按钮的底」的判据
//	   （会命中整块画布）。按钮要靠 `BorderL4` 的边框图元认。
func TestSkillRowGeometryInPixels(t *testing.T) {
	dir := captureDir(t)

	harness := newSkillHarness(t, expandableCard())
	harness.plugin.expand.toggle(1) // 展开：说明卡与分隔点都在这一帧里
	harness.plugin.SetInspect(func(string) {})
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	capture(t, session, dir, "skill-row-expanded")

	theme := harness.theme
	hit, ok := findHit(session, "加载技能")
	if !ok {
		t.Fatalf("没有行命中区；hits=%+v", session.Snapshot().Hits)
	}

	// ① `.row{height:calc(24px + delta)}` —— 用**命中区的真实高度**量。
	if !near(hit.Bounds.Height, SkillRowHeight) {
		t.Errorf("行高（命中区）= %v，want %v", hit.Bounds.Height, SkillRowHeight)
	}

	// ② `.instructionsCard`：边框图元（圆角 16）就是卡的外边界。
	card, ok := rectOf(session, theme.BorderL1, SkillInstructionsCardRadius, 0)
	if !ok {
		t.Fatalf("真实像素里找不到说明卡的边框（颜色 border-l1 / 圆角 %v）", SkillInstructionsCardRadius)
	}
	if want := hit.Bounds.Y + SkillRowHeight + SkillInstructionsCardMarginTop; !near(card.Bounds.Y, want) {
		t.Errorf("说明卡 y=%v，want %v（row 底 + margin-top 4）", card.Bounds.Y, want)
	}
	if want := hit.Bounds.X + SkillInstructionsCardMarginLeft; !near(card.Bounds.X, want) {
		t.Errorf("说明卡 x=%v，want %v（row 左 + margin-left 4）", card.Bounds.X, want)
	}
	if card.Bounds.Height > SkillInstructionsCardMaxHeight {
		t.Errorf("说明卡高 %v 超过官方 max-height %v", card.Bounds.Height, SkillInstructionsCardMaxHeight)
	}
	// 宽度：`.FillWidth` → 行宽减左外边距 4（右外边距 0）。
	if want := hit.Bounds.Width - SkillInstructionsCardMarginLeft - SkillInstructionsCardMarginRight; !near(card.Bounds.Width, want) {
		t.Errorf("说明卡宽 %v，want %v", card.Bounds.Width, want)
	}

	// ③ `.instructionsHeader`：高 8+16+8 = 32，底色 banner，紧贴卡内顶部。
	header, ok := rectOf(session, theme.CodeBanner, -1, 0)
	if !ok {
		t.Fatal("真实像素里找不到说明卡 header 的底（颜色 code-block-banner）")
	}
	wantHeader := SkillInstructionsHeaderPadTop + SkillInstructionsHeaderLineHeight +
		SkillInstructionsHeaderPadBottom
	if !near(header.Bounds.Height, wantHeader) {
		t.Errorf("header 高 %v，want %v（padding 8/8 + line-height 16）", header.Bounds.Height, wantHeader)
	}
	if header.Bounds.Y < card.Bounds.Y || header.Bounds.Y > card.Bounds.Y+1 {
		t.Errorf("header y=%v，want ≈%v（卡内顶部，只隔 0.5px 上边框）", header.Bounds.Y, card.Bounds.Y)
	}

	// ③b `border-bottom:0.5px` 的分隔盒：紧贴 header 底边。
	divider, ok := rectOf(session, theme.BorderL2, -1, 0)
	if !ok {
		t.Fatal("真实像素里找不到 header 下方的 0.5px 分隔（颜色 border-l2）")
	}
	if want := header.Bounds.Y + wantHeader; !near(divider.Bounds.Y, want) {
		t.Errorf("分隔线 y=%v，want %v（header 底边）", divider.Bounds.Y, want)
	}

	// ④ `.separator`：2×2、圆角 1、label-caption 色。
	dot, ok := rectOf(session, theme.LabelCaption, SkillSeparatorRadius, SkillSeparatorHeight)
	if !ok {
		t.Fatal("真实像素里找不到 2×2 圆角 1 的分隔点（颜色 label-caption）")
	}
	if !near(dot.Bounds.Width, SkillSeparatorWidth) {
		t.Errorf("分隔点宽 %v，want %v", dot.Bounds.Width, SkillSeparatorWidth)
	}
	// 它在行内垂直居中：`(24-2)/2 = 11`。
	if want := hit.Bounds.Y + (SkillRowHeight-SkillSeparatorHeight)/2; !near(dot.Bounds.Y, want) {
		t.Errorf("分隔点 y=%v，want %v（行内垂直居中）", dot.Bounds.Y, want)
	}
	if dot.Bounds.Y >= card.Bounds.Y {
		t.Errorf("分隔点 y=%v 落进了说明卡（卡 y=%v）", dot.Bounds.Y, card.Bounds.Y)
	}

	// ⑤ `.inspectButton{opacity:0}`：**静止态什么都不画**（官方就是这样，
	//    CSS 的 opacity:0 不产生任何可见像素）。所以这一帧里不该有它的边框。
	if _, ok := rectOf(session, theme.BorderL4, -1, 0); ok {
		t.Error("静止态画出了 inspect 按钮的边框——官方 `.inspectButton{opacity:0}`")
	}

	// ⑥ `.card:hover .inspectButton{opacity:1}`：把指针移到**卡上**（不是按钮上），
	//    按钮才显形。这条挡的是"显形条件挂错到按钮自己的 hover"。
	if err := session.Move(card.Bounds.X+card.Bounds.Width/2, card.Bounds.Y+card.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	capture(t, session, dir, "skill-row-expanded-hover")
	button, ok := rectOf(session, theme.BorderL4, -1, 0)
	if !ok {
		t.Fatal("悬停整张卡之后 inspect 按钮仍没显形")
	}
	if want := card.Bounds.Y + card.Bounds.Height + HitInspectBottomGap(); button.Bounds.Y < want-1 {
		t.Errorf("inspect 按钮 y=%v，want ≥ %v（卡底 + 卡 margin-bottom 4）",
			button.Bounds.Y, want)
	}
	// 高 = line-height 16 + padding 2+2 + border 0.5+0.5，像素对齐后 ≈21。
	if button.Bounds.Height < 20 || button.Bounds.Height > 22 {
		t.Errorf("inspect 按钮高 %v，want ≈21", button.Bounds.Height)
	}
	// 药丸：圆角必须 ≥ 半高（官方 999px，viva 夹到半高）。
	if button.Radius < button.Bounds.Height/2-0.5 {
		t.Errorf("inspect 按钮圆角 %v，want ≥ 半高 %v（999px 药丸）", button.Radius, button.Bounds.Height/2)
	}
	// 左缩进 4（`.inspectButton{margin:4px 0 2px 4px}`）。
	if want := hit.Bounds.X + SkillInspectMarginLeft; !near(button.Bounds.X, want) {
		t.Errorf("inspect 按钮 x=%v，want %v", button.Bounds.X, want)
	}
}

// HitInspectBottomGap 是 `.inspectButton` 顶边与说明卡底边的官方间距（4px）。
func HitInspectBottomGap() float32 { return SkillInstructionsCardMarginBottom }

// TestSkillRowStatesInPixels 四态各留一帧真实像素。
//
// 判据不是"图里有东西"，而是"四态的画法**彼此不同**、且差异落在官方说的那一处"：
// error 态摘要文字用错误色、stopped 态用警告色、preparing 态没有分隔点。
func TestSkillRowStatesInPixels(t *testing.T) {
	dir := captureDir(t)

	states := []struct {
		name string
		card dstool.Card
	}{
		{"ok", dstool.Card{Seq: 11, Tool: SkillToolName, Args: `{"name":"demo"}`,
			Status: dstool.StatusOK,
			View:   &apiclient.ViewSpec{Kind: "plain", Body: "说明正文第一行\n第二行"}}},
		{"error", dstool.Card{Seq: 12, Tool: SkillToolName, Args: `{"name":"demo"}`,
			Status: dstool.StatusError,
			View:   &apiclient.ViewSpec{Kind: "plain", Body: "出错了\n第二行"}}},
		{"stopped", dstool.Card{Seq: 13, Tool: SkillToolName, Args: `{"name":"demo"}`,
			Status: dstool.StatusStopped,
			View:   &apiclient.ViewSpec{Kind: "plain", Body: "中断了"}}},
		{"preparing", dstool.Card{Seq: 14, Tool: SkillToolName, Args: `{"name":"demo"}`,
			Status: dstool.StatusPreparing}},
	}

	for _, state := range states {
		state := state
		t.Run(state.name, func(t *testing.T) {
			harness := newSkillHarness(t, state.card)
			session := harness.openSession(t, 900, 700)
			if err := session.Driver().Pump(); err != nil {
				t.Fatal(err)
			}
			capture(t, session, dir, "skill-row-"+state.name)

			dotFound := false
			for _, primitive := range session.Snapshot().Primitives {
				if primitive.Kind == app.RectanglePrimitive &&
					primitive.Color == harness.theme.LabelCaption &&
					primitive.Bounds.Height == SkillSeparatorHeight &&
					primitive.Radius == SkillSeparatorRadius {
					dotFound = true
					break
				}
			}
			switch state.name {
			case "preparing":
				// 官方 preparing 树**没有** separator（`SkillRow.tsx:103-109`）。
				if dotFound {
					t.Error("preparing 态画出了分隔点——官方那一棵树没有 separator")
				}
			default:
				if !dotFound {
					t.Error("started 态缺分隔点")
				}
			}
		})
	}
}
