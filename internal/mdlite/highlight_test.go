package mdlite

import (
	"sort"
	"testing"

	"github.com/vivaui/viva/components"
)

// runsOf 是测试里简化断言用的小结构。
type run struct {
	Start, End int
	Class      components.MarkdownCodeClass
}

func hl(language, line string) []run {
	got := Highlight(language, line, nil)
	out := make([]run, len(got))
	for i, r := range got {
		out[i] = run{r.Start, r.End, r.Class}
	}
	return out
}

func findClass(rs []run, class components.MarkdownCodeClass) []string {
	var out []string
	for _, r := range rs {
		if r.Class == class {
			out = append(out, "")
			out[len(out)-1] = "" // 占位，下面用 Start/End 截
			_ = r
		}
	}
	return out
}

// 1) Go：关键字 func / return、注释、字符串
func TestHighlightGo(t *testing.T) {
	line := `func main() { return "hi" } // 收尾`
	rs := hl("go", line)

	var sawKeywordFunc, sawKeywordReturn, sawString, sawComment bool
	for _, r := range rs {
		switch r.Class {
		case components.CodeKeyword:
			switch line[r.Start:r.End] {
			case "func":
				sawKeywordFunc = true
			case "return":
				sawKeywordReturn = true
			}
		case components.CodeString:
			if line[r.Start:r.End] == `"hi"` {
				sawString = true
			}
		case components.CodeComment:
			if line[r.Start:r.End] == "// 收尾" {
				sawComment = true
			}
		}
	}
	if !sawKeywordFunc || !sawKeywordReturn || !sawString || !sawComment {
		t.Fatalf("Go 高亮不全：func=%v return=%v string=%v comment=%v（runs=%v）",
			sawKeywordFunc, sawKeywordReturn, sawString, sawComment, rs)
	}
}

// 2) Python：def 是关键字、# 是注释、单引号是字符串
func TestHighlightPython(t *testing.T) {
	line := `def add(a, b): return a + b  # 相加`
	rs := hl("python", line)

	var sawDef, sawHash bool
	for _, r := range rs {
		switch r.Class {
		case components.CodeKeyword:
			if line[r.Start:r.End] == "def" || line[r.Start:r.End] == "return" {
				sawDef = true
			}
		case components.CodeComment:
			if line[r.Start:r.End] == "# 相加" {
				sawHash = true
			}
		}
	}
	if !sawDef || !sawHash {
		t.Fatalf("Python 高亮不全：def/return=%v 注释=%v（runs=%v）", sawDef, sawHash, rs)
	}

	// `#` 在 Go 里不是注释——同一行换个语言不该被当注释
	rsGo := hl("go", line)
	for _, r := range rsGo {
		if r.Class == components.CodeComment {
			t.Fatalf("Go 不该把 # 当注释：%q", line[r.Start:r.End])
		}
	}
}

// 3) 未知语言：只认注释/字符串，不认关键字
func TestHighlightUnknownLanguage(t *testing.T) {
	line := `func main() { "x" }`
	rs := hl("brainfuck", line)

	var sawString bool
	for _, r := range rs {
		if r.Class == components.CodeKeyword {
			t.Fatalf("未知语言不该有关键字：%q", line[r.Start:r.End])
		}
		if r.Class == components.CodeString && line[r.Start:r.End] == `"x"` {
			sawString = true
		}
	}
	if !sawString {
		t.Fatalf("未知语言至少要认出字符串（runs=%v）", rs)
	}
}

// 4) 空行 / 空语言不 panic，也不产出 run
func TestHighlightEmpty(t *testing.T) {
	if got := Highlight("", "", nil); len(got) != 0 {
		t.Fatalf("空行不该产出 run：%v", got)
	}
	if got := Highlight("go", "", nil); len(got) != 0 {
		t.Fatalf("空行不该产出 run（带语言）：%v", got)
	}
}

// 5) 没闭合的字符串 / 全是引号 / 纯符号——不许 panic，也不许越界
func TestHighlightNoPanic(t *testing.T) {
	cases := []string{
		`"unterminated`,
		`'`,
		"`",
		`\"\\\"`,
		`))))`,
		`123.`,
		`//`,
		`#`,
		`"`,
		`""""`,
	}
	for _, line := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("输入 %q 触发 panic: %v", line, r)
				}
			}()
			for _, r := range Highlight("go", line, nil) {
				if r.Start < 0 || r.End > len(line) || r.Start >= r.End {
					t.Fatalf("输入 %q 产出越界 run：[%d,%d)", line, r.Start, r.End)
				}
			}
		}()
	}
}

// 6) run 不重叠、按 Start 严格升序——这是 MarkdownView 能不能正确上色的前提
func TestHighlightRunsOrdered(t *testing.T) {
	lines := []string{
		`func Foo(n int) (string, error) { return "x", nil }`,
		`const HTTP_PORT = 8080 // 端口`,
		`x := []byte{0, 1, 2}; y := 3.14`,
		`if err != nil { log.Fatal(err) }`,
		`"a" + "b" + "c"`,
	}
	for _, line := range lines {
		rs := hl("go", line)
		if !sort.SliceIsSorted(rs, func(i, j int) bool { return rs[i].Start < rs[j].Start }) {
			t.Fatalf("run 没按 Start 升序：%q → %v", line, rs)
		}
		for i := 1; i < len(rs); i++ {
			if rs[i].Start < rs[i-1].End {
				t.Fatalf("run 重叠：%q → %v（第 %d 与第 %d 条）", line, rs, i-1, i)
			}
		}
		for _, r := range rs {
			if r.Start < 0 || r.End > len(line) || r.Start >= r.End {
				t.Fatalf("越界 run：%q → [%d,%d)", line, r.Start, r.End)
			}
		}
	}
}
