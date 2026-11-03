package dstool

import (
	"testing"
)

// 这个文件只测纯函数：不碰 flex / canvas / 窗口，所以能裸跑。
func TestParseArgs(t *testing.T) {
	// parseArgs 的输入是**宿主给的任意字符串**。它必须把"不是 JSON"当成正常情况
	// 返回 nil（调用方据此退回原文），而不是返回空 map —— 空 map 会让界面显示
	// 一条没有内容的工具卡，等于把宿主的参数吞掉。
	cases := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"JSON 对象", `{"filePath":"a/b.go","offset":10}`, map[string]string{"filePath": "a/b.go", "offset": "10"}},
		{"一行命令不是 JSON", `go test ./...`, nil},
		{"裸路径不是 JSON", `internal/apiclient/mux.go`, nil},
		{"坏 JSON", `{"filePath":`, nil},
		{"JSON 数组不是对象", `[1,2]`, nil},
		{"空串", ``, nil},
		{"空白", "  \n ", nil},
		{"数字不带小数点", `{"limit":40}`, map[string]string{"limit": "40"}},
		{"小数保留", `{"ratio":1.5}`, map[string]string{"ratio": "1.5"}},
		{"布尔", `{"replaceAll":true}`, map[string]string{"replaceAll": "true"}},
		// 数组/对象在这一层看不懂：**丢掉**，由调用方退回原文，不塞一段 JSON 进去。
		{"数组值被丢掉", `{"paths":["a","b"],"filePath":"c"}`, map[string]string{"filePath": "c"}},
		{"prefix 有空白也要认", `  {"pattern":"x"}`, map[string]string{"pattern": "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseArgs(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("parseArgs(%q) = %v，期望 nil（不是 JSON 对象）", tc.in, got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseArgs(%q) = %v，期望 %v", tc.in, got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("parseArgs(%q)[%q] = %q，期望 %q", tc.in, k, got[k], v)
				}
			}
		})
	}
}

func TestCommand(t *testing.T) {
	// command 是 bash 标题行/展开体的来源。三条规矩：
	//   · JSON 里没有 command → 返回空串，**不许**把整段 JSON 当命令摆上去；
	//   · 不是 JSON → 它本身就是命令，原样返回（这条最常走）；
	//   · 前后空白要收掉。
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"裸命令", `go test ./...`, "go test ./..."},
		{"带空白", "  git status --short \n", "git status --short"},
		{"JSON 的 command", `{"command":"CGO_ENABLED=0 go build ./..."}`, "CGO_ENABLED=0 go build ./..."},
		{"JSON 的 cmd 别名", `{"cmd":"ls -la"}`, "ls -la"},
		{"JSON 里没有命令", `{"filePath":"a.go"}`, ""},
		{"空串", ``, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := command(tc.in); got != tc.want {
				t.Fatalf("command(%q) = %q，期望 %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitExit(t *testing.T) {
	// 退出码的两种真实写法（mockd 的 "exit 0 · …" 与真宿主工具输出的 "[exit 0] …"）。
	// 拆不出来时必须原样返回 —— **不许**"没写就是成功"。
	cases := []struct {
		name     string
		in       string
		wantCode string
		wantRest string
	}{
		{"点分隔", "exit 0 · 3 files changed", "exit 0", "3 files changed"},
		{"只有退出码", "exit 1", "exit 1", ""},
		{"方括号写法", "[exit 0] … TOOL-LIVE", "exit 0", "… TOOL-LIVE"},
		{"方括号无后续", "[exit 2]", "exit 2", ""},
		{"负退出码", "exit -1 · 被信号打断", "exit -1", "被信号打断"},
		{"没有退出码", "1.2 KB · 78 行", "", "1.2 KB · 78 行"},
		{"空串", "", "", ""},
		{"中间提到 exit 不算", "工具 exit 0 完成了", "", "工具 exit 0 完成了"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, rest := splitExit(tc.in)
			if code != tc.wantCode || rest != tc.wantRest {
				t.Fatalf("splitExit(%q) = (%q, %q)，期望 (%q, %q)",
					tc.in, code, rest, tc.wantCode, tc.wantRest)
			}
		})
	}
}

func TestBadExit(t *testing.T) {
	// 只有非 0 才算错。这条不成立时，失败的 bash 会顶着绿徽标显示——比不显示更糟。
	cases := map[string]bool{
		"exit 0":   false,
		"exit 1":   true,
		"exit 127": true,
		"exit -1":  true,
		"exit 00":  false,
	}
	for code, want := range cases {
		if got := badExit(code); got != want {
			t.Fatalf("badExit(%q) = %v，期望 %v", code, got, want)
		}
	}
}
