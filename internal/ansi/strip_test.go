package ansi

import "testing"

// TestStripRemovesCSI 验 CSI 序列（颜色/光标/清屏）被整条去掉。
func TestStripRemovesCSI(t *testing.T) {
	cases := map[string]string{
		"\x1b[31mred\x1b[0m":  "red",
		"\x1b[2J\x1b[H":       "", // 清屏 + 归位：什么都不剩
		"a\x1b[1;32mb\x1b[mc": "abc",
		"\x1b[Kafter":         "after", // 擦行
		"x\x1b[38;5;196my":    "xy",
	}
	for in, want := range cases {
		if got := Strip(in); got != want {
			t.Errorf("Strip(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestStripRemovesOSC 验 OSC（改标题、超链接）被整条去掉。
//
// OSC 有两种结尾：BEL（0x07）和 ESC \。**漏掉任何一种都会让后面的正文被吃掉**，
// 因为解析器会一直找结尾直到字符串末尾。
func TestStripRemovesOSC(t *testing.T) {
	if got := Strip("\x1b]0;my title\x07hello"); got != "hello" {
		t.Errorf("BEL 结尾的 OSC：got %q", got)
	}
	if got := Strip("\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\"); got != "link" {
		t.Errorf("ESC 反斜杠结尾的 OSC：got %q", got)
	}
}

// TestStripKeepsPlain 验普通文本原样（**不能因为没转义就丢东西**）。
func TestStripKeepsPlain(t *testing.T) {
	const s = "hello world\n中文也能过\n"
	if got := Strip(s); got != s {
		t.Fatalf("普通文本被改了：%q", got)
	}
}

// TestStripTruncatedEscape 验末尾的半个转义不会 panic、也不会吐出垃圾。
func TestStripTruncatedEscape(t *testing.T) {
	for _, in := range []string{"\x1b", "\x1b[", "\x1b[31", "\x1b]0;title"} {
		_ = Strip(in) // 只要不 panic
	}
	if got := Strip("abc\x1b["); got != "abc" {
		t.Fatalf("got %q，want %q", got, "abc")
	}
}

// TestLastLine 验 `\r` 覆盖语义（进度条的最终态）。
//
// **Strip 不碰 `\r`**：删掉它会得到 `10%50%100%`，比原样更糟。
// 只有知道"这是终端"的一层才该按 `\r` 切。
func TestLastLine(t *testing.T) {
	if got := LastLine("10%\r50%\r100%"); got != "100%" {
		t.Fatalf("got %q，want %q", got, "100%")
	}
	if got := LastLine("no-cr"); got != "no-cr" {
		t.Fatalf("没有 \r 时应当原样，got %q", got)
	}
}
