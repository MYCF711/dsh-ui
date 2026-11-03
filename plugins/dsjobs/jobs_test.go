package dsjobs

import "testing"

// TestShortCmd 验命令压缩：只取第一个 token，太长截断，空命令有兜底。
//
// **不做 shell 解析**是有意的：这个标签只是"这条在干嘛"的提示，
// 真解析 `echo a; rm -rf b` 会得出一个看着像结论、其实不可靠的东西。
func TestShortCmd(t *testing.T) {
	cases := map[string]string{
		"sleep 60":              "sleep",
		"npm test -- --watch":   "npm",
		"  go   build  ./...  ": "go",
		"":                      "(空命令)",
		"   ":                   "(空命令)",
		"averyveryveryveryveryverylongcommand name": "averyveryveryveryveryver…",
	}
	for in, want := range cases {
		if got := shortCmd(in); got != want {
			t.Errorf("shortCmd(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSizeLabel 验三档换算。**没有精确到小数的必要**：侧栏宽度只够放 4-5 个字符。
func TestSizeLabel(t *testing.T) {
	cases := map[int]string{
		0:               "0 B",
		512:             "512 B",
		1024:            "1 KB",
		3584:            "3 KB",
		1024 * 1024:     "1 MB",
		3 * 1024 * 1024: "3 MB",
	}
	for in, want := range cases {
		if got := sizeLabel(in); got != want {
			t.Errorf("sizeLabel(%d) = %q, want %q", in, got, want)
		}
	}
}
