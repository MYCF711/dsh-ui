package dsdeliverables

import (
	"reflect"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestPathsOnlyExplicitFileArgs 验：只认 write/edit 的**明确路径参数**。
//
// 这条测试守的是"交付物"这个词的**证据强度**：
// 从参数里拿 filePath 是痕跡，从 bash 命令串里正则挖路径就是猜。
func TestPathsOnlyExplicitFileArgs(t *testing.T) {
	msgs := []apiclient.Message{
		{Role: "user", Text: "改一下 a.go"},
		// write：filePath → 认
		{Role: "tool", Tool: "write", Args: `{"filePath":"src/a.go","content":"x"}`},
		// edit：path → 认
		{Role: "tool", Tool: "edit", Args: `{"path":"src/b.go","oldString":"x","newString":"y"}`},
		// 同一个文件第二次 → 去重
		{Role: "tool", Tool: "edit", Args: `{"filePath":"src/a.go"}`},
		// bash：路径在命令串里 → **不认**（猜出来会列出删掉的文件）
		{Role: "tool", Tool: "bash", Args: `{"command":"rm -rf src/c.go"}`},
		// read 只读 → 不认
		{Role: "tool", Tool: "read", Args: `{"filePath":"src/d.go"}`},
		// 参数不是 JSON → 跳过
		{Role: "tool", Tool: "write", Args: `not json`},
		// assistant 的消息不参与
		{Role: "assistant", Tool: "write", Args: `{"filePath":"src/e.go"}`},
	}
	got := paths(msgs)
	want := []string{"src/a.go", "src/b.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %v，want %v", got, want)
	}
}

// TestPathsEmpty 验：没有写类工具时返回 nil（插件据此整块不画）。
func TestPathsEmpty(t *testing.T) {
	if got := paths(nil); got != nil {
		t.Fatalf("空输入应当返回 nil，得到 %v", got)
	}
	msgs := []apiclient.Message{{Role: "tool", Tool: "bash", Args: `{"command":"ls"}`}}
	if got := paths(msgs); len(got) != 0 {
		t.Fatalf("只有 bash 时不该有交付物，得到 %v", got)
	}
}
