package dsconversation

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// 这个文件只测**纯函数**：不碰 flex / canvas / 窗口，所以能在 CI 里裸跑。
// 展示逻辑（draw*）不进单测——它要的是真实布局，靠无头截图验收（见 docs/plugin-api.md §8）。

func TestHumanRunes(t *testing.T) {
	// humanRunes 给「▸ 思考 · N 字」用。它**不是**精确格式化：
	// 只保留一位小数（截断不四舍五入），因为这是给人扫一眼的量级，不是账目。
	// 边界重要性：999/1000 是唯一的单位切换点，写错就会看到「1000 字」vs「1.0k 字」
	// 两套写法混在一起。
	cases := []struct {
		name string
		in   int
		want string
	}{
		{"零", 0, "0"},
		{"个位", 7, "7"},
		{"三位数不换单位", 999, "999"},
		{"整一千换成 k", 1000, "1.0k"},
		{"千位截断不四舍五入", 1250, "1.2k"},
		{"999 后面那一位仍是 9 也不进位", 1999, "1.9k"},
		{"五位数", 12340, "12.3k"},
		{"百万也走 k（本函数不认 M）", 1000000, "1000.0k"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := humanRunes(tc.in); got != tc.want {
				t.Fatalf("humanRunes(%d) = %q，want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestItoa(t *testing.T) {
	// itoa 是为了「不为一个数字 import strconv」手写的。负数分支很容易写漏负号，
	// 而它会影响 humanRunes / 时长显示的每一处。
	cases := []struct {
		name string
		in   int
		want string
	}{
		{"零要走特判分支", 0, "0"},
		{"个位", 7, "7"},
		{"多位", 123, "123"},
		{"负数带符号", -5, "-5"},
		{"负多位数", -1234, "-1234"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := itoa(tc.in); got != tc.want {
				t.Fatalf("itoa(%d) = %q，want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsKey(t *testing.T) {
	// isKey 决定 Ctrl+R / Ctrl+Shift+C 认不认。
	// 为什么不用 ui.KeyC：viva 只给功能键（KeyUp/KeyEnter…）定义常量，可打印键
	// 只能靠 ev.Text / ev.Typed 识别；Text 是"不受修饰键影响"的字符，
	// 所以按住 Shift 打出来的也仍然是 "c"。这里必须两端都认。
	cases := []struct {
		name   string
		ev     ui.KeyEvent
		letter string
		want   bool
	}{
		{"小写 text", ui.KeyEvent{Text: "c"}, "c", true},
		{"大写 text（Shift 之后）", ui.KeyEvent{Text: "C"}, "c", true},
		{"只填 Typed", ui.KeyEvent{Typed: "r"}, "r", true},
		{"两者都没有", ui.KeyEvent{}, "c", false},
		{"别的字母", ui.KeyEvent{Text: "x"}, "c", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isKey(tc.ev, tc.letter); got != tc.want {
				t.Fatalf("isKey(%+v, %q) = %v，want %v", tc.ev, tc.letter, got, tc.want)
			}
		})
	}
}

func TestMin(t *testing.T) {
	// min 只给工具卡表格算「最多画几行」用；两个分支都要踩到，
	// 否则 a>b 那半边永远没被跑过。
	cases := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{3, 3, 3},
		{-3, -5, -5},
	}
	for _, tc := range cases {
		if got := min(tc.a, tc.b); got != tc.want {
			t.Fatalf("min(%d, %d) = %d，want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
