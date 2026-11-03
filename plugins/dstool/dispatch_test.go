package dstool

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 这个文件只验"分派到谁"和"标题从哪来"，不画像素（像素在 dstool_visual_test.go）。
// 所以 New(nil) 就够——prims 只影响徽标。

// TestDispatchTableHasBothKinds 是这次抽取的**核心断言**：
// 分派表里必须同时有 bash 和文件类工具，否则"按工具类型定制"就是空话。
func TestDispatchTableHasBothKinds(t *testing.T) {
	s := New(nil)
	bash, ok := s.renderers["bash"]
	if !ok {
		t.Fatal("分派表里没有 bash")
	}
	if bash.Head == nil || bash.Body == nil {
		t.Fatalf("bash 不是定制渲染器（Head=%v Body=%v）", bash.Head != nil, bash.Body != nil)
	}
	for _, tool := range []string{"read", "edit", "write", "list", "glob", "grep"} {
		r, ok := s.renderers[tool]
		if !ok {
			t.Fatalf("分派表里没有 %s", tool)
		}
		if r.Head == nil || r.Body == nil {
			t.Fatalf("%s 不是定制渲染器（Head=%v Body=%v）", tool, r.Head != nil, r.Body != nil)
		}
	}
	if _, ok := s.renderers["webfetch"]; ok {
		t.Fatal("没实现的工具不该被登记：那样它「看起来」就有定制画法了")
	}
}

// TestRegisterOverrides 验扩展点：后登记的同名渲染器覆盖先登记的。
// 宿主以后加 mcp__foo 这类工具时，界面靠它加画法，不用改消息流。
func TestRegisterOverrides(t *testing.T) {
	s := New(nil)
	s.Register(Renderer{Tool: "bash", Head: func(c Card) (string, string) {
		return "自定义", ""
	}})
	title, _ := s.renderers["bash"].Head(Card{})
	if title != "自定义" {
		t.Fatalf("Register 没有覆盖已有的 bash 渲染器，标题仍是 %q", title)
	}
	// 空 Tool 的渲染器不该进表（否则默认画法会被一个空名的完全接管）。
	s.Register(Renderer{Tool: ""})
	if _, ok := s.renderers[""]; ok {
		t.Fatal("空工具名不该被登记")
	}
}

// TestHeadDerivation 验「宿主没给摘要时，从参数里取标题」。
//
// 这是定制渲染里平时看得见的那一半：真宿主的 read/glob 经常不给 Summary，
// 抽取前标题行会显示一整段 JSON（`{"pattern":"**/*.go","path":"plugins"}`），
// 现在显示 `**/*.go`。有 Summary 时一律照用——抽取前后截图一致靠的就是这一条。
func TestHeadDerivation(t *testing.T) {
	s := New(nil)
	cases := []struct {
		name        string
		card        Card
		wantTitle   string
		wantSummary string
	}{
		{
			name:        "bash 无摘要取命令",
			card:        Card{Tool: "bash", Args: `{"command":"go test ./..."}`},
			wantTitle:   "bash",
			wantSummary: "go test ./...",
		},
		{
			name:        "bash 有摘要就照用",
			card:        Card{Tool: "bash", Args: "git status --short", Summary: "exit 0 · 3 files changed"},
			wantTitle:   "bash",
			wantSummary: "exit 0 · 3 files changed",
		},
		{
			name:        "read 无摘要取路径",
			card:        Card{Tool: "read", Args: `{"filePath":"internal/apiclient/mux.go","limit":40}`},
			wantTitle:   "read",
			wantSummary: "internal/apiclient/mux.go",
		},
		{
			name:        "glob 无摘要取模式",
			card:        Card{Tool: "glob", Args: `{"pattern":"**/*.go","path":"plugins"}`},
			wantTitle:   "glob",
			wantSummary: "**/*.go",
		},
		{
			name:        "grep 无摘要取模式",
			card:        Card{Tool: "grep", Args: `{"pattern":"TODO","path":"internal"}`},
			wantTitle:   "grep",
			wantSummary: "TODO",
		},
		{
			name:        "参数不是 JSON 就退回原文",
			card:        Card{Tool: "read", Args: "internal/apiclient/mux.go"},
			wantTitle:   "read",
			wantSummary: "internal/apiclient/mux.go",
		},
		{
			name:        "没名字的工具用 View.Title",
			card:        Card{Tool: "", View: &apiclient.ViewSpec{Title: "webfetch"}},
			wantTitle:   "webfetch",
			wantSummary: "",
		},
		{
			name:        "View.Badge=error 的摘要要标错误",
			card:        Card{Tool: "bash", Args: "boom", View: &apiclient.ViewSpec{Badge: "error"}},
			wantTitle:   "bash",
			wantSummary: "错误：boom",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			title, summary := s.head(tc.card)
			if title != tc.wantTitle || summary != tc.wantSummary {
				t.Fatalf("(%q, %q)，期望 (%q, %q)", title, summary, tc.wantTitle, tc.wantSummary)
			}
		})
	}
}

// TestHeight 钉住消息流视口虚拟化用的高度估计。
// 数字与抽取前逐字相同——变了会让"滚出去的内容不画、滚回来又缺失"。
func TestHeight(t *testing.T) {
	s := New(nil)
	if got := s.Height(7, nil); got != 28 {
		t.Fatalf("收起时高度 = %v，期望 28", got)
	}
	s.open[7] = true
	if got := s.Height(7, nil); got != 70 {
		t.Fatalf("展开且无 View 时高度 = %v，期望 70", got)
	}
	view := &apiclient.ViewSpec{
		Fields: []apiclient.ViewField{{Key: "a"}, {Key: "b"}, {Key: "c"}},
		Rows:   [][]string{{"1"}, {"2"}},
	}
	if got, want := s.Height(7, view), float32(50+20*(3+2)); got != want {
		t.Fatalf("展开且有 View 时高度 = %v，期望 %v", got, want)
	}
	// 展开状态按 seq 分开：别的 seq 不受影响（切会话后 seq 仍然唯一）。
	if !s.Open(7) || s.Open(8) {
		t.Fatalf("展开状态串了：Open(7)=%v Open(8)=%v", s.Open(7), s.Open(8))
	}
}
