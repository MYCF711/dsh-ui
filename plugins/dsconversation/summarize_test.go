// summarize_test.go —— 消息摘要的边界。
//
// 这个函数在画工具卡/标题时被调用，输入是**宿主任意文本**（可能含换行、
// 制表符、超长单行、中文）。它错了的症状是"摘要看起来怪"，很难归因到一行函数。
package dsconversation

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSummarizeCollapsesWhitespace(t *testing.T) {
	cases := map[string]string{
		"a  b":         "a b",
		"a\tb":         "a b",
		"a\nb":         "a b",
		"  a\n\n b  ":  "a b",
		"a\r\nb":       "a b",
		"already fine": "already fine",
	}
	for in, want := range cases {
		if got := summarize(in, 100); got != want {
			t.Errorf("summarize(%q) = %q，期望 %q（空白应折成单空格）", in, got, want)
		}
	}
}

func TestSummarizeTruncates(t *testing.T) {
	long := strings.Repeat("好", 50)
	got := summarize(long, 10)
	r := []rune(got)
	if len(r) != 11 || r[10] != '…' {
		t.Fatalf("应截成 10 个字符 + 省略号，得到 %q（%d rune）", got, len(r))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("截断破坏了 UTF-8：%q", got)
	}
	// 短串原样返回（不补省略号）
	if got := summarize("短", 10); got != "短" {
		t.Fatalf("短串不该截断，得到 %q", got)
	}
	// 正好等于上限也不截
	exact := strings.Repeat("x", 10)
	if got := summarize(exact, 10); got != exact {
		t.Fatalf("恰好等于上限不该截断，得到 %q", got)
	}
}

func TestSummarizeNonPositiveLimit(t *testing.T) {
	// n <= 0 曾经会走到 r[:n] 直接 panic——输入是宿主任意文本、
	// n 来自调用方常量，虽然现在没人传 0，但"不会有人传"不是不修的借口。
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("summarize 在 n<=0 时 panic 了：%v", r)
		}
	}()
	if got := summarize("abc", 0); got != "" {
		t.Fatalf("n=0 应返回空串，得到 %q", got)
	}
	if got := summarize("abc", -5); got != "" {
		t.Fatalf("n<0 应返回空串，得到 %q", got)
	}
}
