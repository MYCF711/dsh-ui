//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：「在工作目录里打开」按钮的**降级 / hover /
// focus-visible / 窄窗**。
//
// 上一轮已钉过"按钮真的被光栅化出来"（`open_visual_test.go`）。这一轮换三个角度：
//
//	① 没有目标时必须**整个消失**（不是画一个灰按钮）：官方判据是
//	   `OpenTargetButton.tsx:97,146-156`「只有一个可用动作时没有 chevron」以及
//	   split 只在有动作时渲染；本仓 `openDestination` 三条同时成立才画
//	   （`open.go:150-160`）。空 CWD、相对 CWD、壳没接 OpenFile 三种都要验。
//
//	② `.mOxcpG_main:focus-visible{background:var(--dsw-alias-interactive-bg-hover)}`
//	   —— rc.2 asar 偏移 **34265977**（`OpenTargetButton.module.css` 常量）。
//	   注意 rc.2 这里给的是**底色**而不是 outline：键盘焦点落在按钮上时，
//	   按钮必须出现 hover 那层底。真按 Tab 验。
//
//	③ hover 与窄窗：`.mOxcpG_main:hover:not(:disabled){background:interactive-bg-hover}`
//	   （同段）；900 宽下按钮仍须整颗在窗口里。
//
// 截图落 `$VR3_CAPTURE_DIR`（未设置时用临时目录）。
package dsopen

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
)

func vr3CaptureDir(t *testing.T) string {
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

func vr3Shot(t *testing.T, session interface {
	Screenshot(string) error
	Image() (*image.RGBA, error)
}, dir, name string) *image.RGBA {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	img, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("VR3_CAPTURE path=%s size=%v", path, img.Bounds())
	return img
}

// vr3Pixel 把颜色换算成 image.RGBA 里的 4 个字节（渲染目标是预乘的）。
func vr3Pixel(c gfx.Color) [4]int {
	scale := func(v float32) int {
		scaled := float64(v) * float64(c.Alpha) * 255
		switch {
		case scaled < 0:
			return 0
		case scaled > 255:
			return 255
		}
		return int(scaled + 0.5)
	}
	return [4]int{scale(c.Red), scale(c.Green), scale(c.Blue), int(float64(c.Alpha)*255 + 0.5)}
}

func vr3Abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// vr3Ink 数 rect 里与 reference 不同的像素（8 级容差）。
func vr3Ink(img *image.RGBA, rect gfx.Rect, reference gfx.Color) int {
	ref := vr3Pixel(reference)
	ink := 0
	x0, y0 := maxInt(int(rect.X), 0), maxInt(int(rect.Y), 0)
	x1 := minInt(int(rect.X+rect.Width), img.Bounds().Max.X)
	y1 := minInt(int(rect.Y+rect.Height), img.Bounds().Max.Y)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			pixel := img.RGBAAt(x, y)
			if vr3Abs(int(pixel.R)-ref[0]) > 8 || vr3Abs(int(pixel.G)-ref[1]) > 8 ||
				vr3Abs(int(pixel.B)-ref[2]) > 8 || vr3Abs(int(pixel.A)-ref[3]) > 8 {
				ink++
			}
		}
	}
	return ink
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// vr3OpenButton 是这一段固定动作面：接了本地打开，但没有右栏开合（corner 不在）。
func vr3OpenButton() pluginapi.Actions {
	return pluginapi.Actions{OpenFile: func(string) error { return nil }}
}

// ── ① 无目标 → 整颗消失 ─────────────────────────────────────────────────────

// TestVR3OpenButtonVanishesWithoutTarget 三种"没有目标"的形状都不许画按钮。
func TestVR3OpenButtonVanishesWithoutTarget(t *testing.T) {
	dir := vr3CaptureDir(t)
	theme := dscore.Dark()
	cases := []struct {
		name string
		cwd  string
		act  pluginapi.Actions
	}{
		{"no-cwd", "", vr3OpenButton()},
		{"relative-cwd", "ws/项目", vr3OpenButton()},
		{"drive-relative", `D:ws`, vr3OpenButton()},
		{"no-open-action", `D:\ws\项目`, pluginapi.Actions{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := openHeaderData(tc.cwd)
			session := openHeaderSession(t, &data, tc.act, 1182, 832)
			defer session.Close()
			if hit, ok := session.Hit("在工作目录里打开"); ok {
				t.Fatalf("CWD=%q 时仍然画了按钮：bounds=%+v（官方只在有目标时渲染）", tc.cwd, hit.Bounds)
			}
			img := vr3Shot(t, session, dir, "open-button-absent-"+tc.name+".png")
			// 标题行右半边（按钮可能落的那一带）不许有 ink：整颗都没了。
			band := gfx.Rect{X: 1182 - 120, Y: 20, Width: 100, Height: 32}
			if ink := vr3Ink(img, band, theme.BgBase); ink != 0 {
				t.Errorf("无目标时标题行右侧仍有 %d 个非底色像素（按钮该整颗消失）", ink)
			}
		})
	}
}

// ── ② hover / focus-visible 都是 hover 那层底 ───────────────────────────────

// TestVR3OpenButtonHoverAndFocusBackground 真指针 hover 与真 Tab 键盘焦点：
// 官方两处都画 `interactive-bg-hover` 底色（rc.2 asar 偏移 34265977）。
func TestVR3OpenButtonHoverAndFocusBackground(t *testing.T) {
	dir := vr3CaptureDir(t)
	theme := dscore.Dark()
	data := openHeaderData(`D:\ws\项目`)
	session := openHeaderSession(t, &data, vr3OpenButton(), 1182, 832)
	defer session.Close()

	hit, ok := session.Hit("在工作目录里打开")
	if !ok {
		t.Fatalf("有目标时找不到按钮：texts=%q", session.Texts())
	}
	// 官方 compact 分支：`height:24px;border:.5px solid;radius-sm`，
	// `main` 的 `padding:3px 5px` + 13px 图标 + 2×0.5 描边 → 24×24（open.go:71-77）。
	if hit.Bounds.Width != buttonWidth || hit.Bounds.Height != buttonHeight {
		t.Errorf("按钮=%vx%v，官方 compact 是 %vx%v（.split{height:24px} + 图标 13 + padding 3/5）",
			hit.Bounds.Width, hit.Bounds.Height, buttonWidth, buttonHeight)
	}

	// 指针挪到窗口另一侧：静止帧。
	if err := session.Move(4, 4); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	idle := vr3Shot(t, session, dir, "open-button-idle.png")
	// 静止帧的 ink 是**描边 + 13px 图标**（官方 .main 没有底色），把它当基线：
	// hover / focus-visible 各自都要在这之上多出一层 `interactive-bg-hover` 的底。
	idleInk := vr3Ink(idle, hit.Bounds, theme.BgBase)
	if idleInk == 0 {
		t.Errorf("静止帧按钮区域零 ink（0.5px 描边 + 13px 图标都没画出来）")
	}

	// 真指针 hover。
	if err := session.Move(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	hover := vr3Shot(t, session, dir, "open-button-hover.png")
	hoverInk := vr3Ink(hover, hit.Bounds, theme.BgBase)
	if hoverInk <= idleInk+40 {
		t.Errorf("hover 后按钮区域 ink=%d，静止基线 %d：没有多出 hover 底色"+
			"（官方 `.main:hover:not(:disabled){background:var(--dsw-alias-interactive-bg-hover)}`）",
			hoverInk, idleInk)
	}

	// 真的把指针挪走，再走键盘：焦点用 Tab 落到按钮上。
	if err := session.Move(4, 4); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	focused := false
	for i := 1; i <= 16; i++ {
		if err := session.Keys("tab"); err != nil {
			t.Fatalf("第 %d 次 Tab 失败：%v", i, err)
		}
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		if session.Snapshot().Focused == hit.ID {
			focused = true
			break
		}
	}
	if !focused {
		t.Fatalf("按 16 次 Tab 焦点没走到「在工作目录里打开」——按钮不是键盘停靠点")
	}
	focus := vr3Shot(t, session, dir, "open-button-focus-visible.png")
	focusInk := vr3Ink(focus, hit.Bounds, theme.BgBase)
	if focusInk <= idleInk+40 {
		t.Errorf("按钮拿到键盘焦点（focus-visible）却没有底色：" +
			"官方 rc.2 asar 偏移 34265977 的 `.mOxcpG_main:focus-visible{background:" +
			"var(--dsw-alias-interactive-bg-hover)}` 没落到画面上")
	}
	t.Logf("按钮 ink：静止=%d hover=%d focus-visible=%d（区域 %vx%v）",
		idleInk, hoverInk, focusInk, hit.Bounds.Width, hit.Bounds.Height)
}

// ── ③ 窄窗：按钮仍整颗在窗口里 ──────────────────────────────────────────────

// TestVR3OpenButtonInsideNarrowWindow 900×700：`.split{overflow:hidden}` 那一颗按钮
// 不许被窗口右缘切掉（官方 headerUtilities 贴 headerCorner 左边，见 open.go:35-44 的算法）。
func TestVR3OpenButtonInsideNarrowWindow(t *testing.T) {
	dir := vr3CaptureDir(t)
	data := openHeaderData(`D:\ws\项目`)
	session := openHeaderSession(t, &data, vr3OpenButton(), 900, 700)
	defer session.Close()

	hit, ok := session.Hit("在工作目录里打开")
	if !ok {
		t.Fatalf("900×700 下找不到按钮：texts=%q", session.Texts())
	}
	if hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > 900 {
		t.Errorf("窄窗下按钮越界：%+v（窗口宽 900）", hit.Bounds)
	}
	if hit.Bounds.Y < 0 || hit.Bounds.Y+hit.Bounds.Height > 700 {
		t.Errorf("窄窗下按钮纵向越界：%+v", hit.Bounds)
	}
	vr3Shot(t, session, dir, "open-button-narrow-900x700.png")
	// 按钮必须整颗落在标题行里（`titleRowHeight=30`、headerPadTop=10 + columnPad=12）：
	// 顶部距离 = 12 + 10 + (30-24)/2 = 25。
	wantTop := float32(columnPad + headerPadTop + (titleRowHeight-buttonHeight)/2)
	if d := hit.Bounds.Y - wantTop; d > 1 || d < -1 {
		t.Errorf("按钮顶=%v，官方标题行居中应为 %v（columnPad 12 + headerPadTop 10 + (30-24)/2）",
			hit.Bounds.Y, wantTop)
	}
}
