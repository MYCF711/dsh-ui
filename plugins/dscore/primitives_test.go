package dscore

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// dscore 是**所有插件共用的那一层**。它错了，错的是整个界面。
// 这个包以前一个测试都没有（theme_test.go 也是这一轮才加的）。
//
// 这里钉两件事：
//   1. `.Hit(id).Clicked()` 这条点击契约——每个用 Primitives 的插件都靠它；
//   2. `itoa` 的边界——它返回空串或错数字时**不报错**。

func newFrame(t *testing.T, click string) (*dscoreFrame, func()) {
	t.Helper()
	theme := Dark()
	p := &Primitives{Theme: theme}
	tree := &ui.Tree{}
	feat := tree.Feature("dscore-test")
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	if click != "" {
		canvas.BeginEventPass(tree.ID(feat, click))
	}
	var view flex.View
	var box flex.Box
	view.Draw(canvas, VivaTheme(theme), ui.Rect{Width: 320, Height: 200}, func(f *flex.Frame) {
		box = p.ListRow(f, tree.ID(feat, "row-1"), false, 32, func() {
			f.Text("一行").FontSize(14).Color(theme.LabelPrimary)
		})
	})
	return &dscoreFrame{box: box, canvas: canvas, tree: tree, feat: feat}, func() {
		if click != "" {
			canvas.EndEventPass()
		}
	}
}

type dscoreFrame struct {
	box    flex.Box
	canvas *ui.Canvas
	tree   *ui.Tree
	feat   ui.Scope
}

// TestListRowHitMatchesID 钉住点击契约。
//
// 每个用 `Primitives.ListRow` 的插件都靠 `row.Clicked()` 判断"这一行被点了"。
// 如果 `Hit` 和 `Clicked` 的配对断了（改 id 拼法、改 Children 顺序），
// **所有列表都会静默失去响应**：画得对、看着能点，按下去没反应。
func TestListRowHitMatchesID(t *testing.T) {
	// 点到这一行 → true
	f, done := newFrame(t, "row-1")
	defer done()
	if !f.box.Clicked() {
		t.Fatal("点的是这一行自己的 id，Clicked() 却是 false——所有列表都会失去响应")
	}

	// 点别的 id → false
	f2, done2 := newFrame(t, "row-2")
	defer done2()
	if f2.box.Clicked() {
		t.Fatal("点的是另一行的 id，这一行却报被点了")
	}

	// 根本没点 → false
	f3, done3 := newFrame(t, "")
	defer done3()
	if f3.box.Clicked() {
		t.Fatal("没有事件 pass 时报被点了")
	}
}

// TestListRowGeometry 钉住行高与形状：调用方给 32（官方会话行），就得是 32。
func TestListRowGeometry(t *testing.T) {
	f, done := newFrame(t, "")
	defer done()
	rect := f.box.Rect()
	if rect.Height != 32 {
		t.Errorf("行高 %v，want 32（调用方给的值）", rect.Height)
	}
	if rect.Width != 320 {
		t.Errorf("行宽 %v，want 320（FillWidth）", rect.Width)
	}
}

// TestCircleButtonReturnsClick 是同一条契约的另一个使用者。
func TestCircleButtonReturnsClick(t *testing.T) {
	theme := Dark()
	p := &Primitives{Theme: theme}
	tree := &ui.Tree{}
	feat := tree.Feature("dscore-circle")
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(string, float32, float32) float32 { return 10 })
	canvas.BeginEventPass(tree.ID(feat, "send"))
	defer canvas.EndEventPass()
	var clicked bool
	var view flex.View
	view.Draw(canvas, VivaTheme(theme), ui.Rect{Width: 320, Height: 200}, func(f *flex.Frame) {
		clicked = p.CircleButton(f, tree.ID(feat, "send"), "↑", true)
	})
	if !clicked {
		t.Fatal("点了 CircleButton 自己的 id，返回值是 false")
	}
}

// TestItoaEdges 是这一轮修掉的那个边界。
//
// 原来缓冲区只有 8 字节、循环条件 `n > 0`：
//   - `itoa(-5)` → 循环不进 → 空串；
//   - `itoa(100000000)` → 高位被截 → "00000000"。
//
// 两个都不报错。现在都够不到（计数先被 `> 0` 挡住），但一个**返回错数字却不报错**
// 的辅助函数比一个会 panic 的更难查。
func TestItoaEdges(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{7, "7"},
		{12345, "12345"},
		{-5, "-5"},
		{-12345, "-12345"},
		{99999999, "99999999"},
		{100000000, "100000000"},   // 旧实现会返回 "00000000"
		{1234567890, "1234567890"}, // 旧实现会截成低 8 位
	}
	for _, tc := range cases {
		if got := itoa(tc.in); got != tc.want {
			t.Errorf("itoa(%d) = %q，want %q", tc.in, got, tc.want)
		}
	}
}

// 编译期：Primitives 必须仍然满足契约（改了签名这里先红）。
var _ pluginapi.Primitives = (*Primitives)(nil)
