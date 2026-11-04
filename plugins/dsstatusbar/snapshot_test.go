package dsstatusbar

import (
	"reflect"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

func TestWholeLogUsageDoesNotDependOnTail(t *testing.T) {
	base := pluginapi.Snapshot{
		Cur: "session-a",
		UsageTotal: pluginapi.Usage{
			InputTokens: 100, OutputTokens: 50, TotalTokens: 150,
		},
		ContextTokens: 120,
	}
	tail1 := base
	tail1.Msgs = []apiclient.Message{{Role: "assistant"}}
	tail50 := base
	tail50.Msgs = make([]apiclient.Message, 50)
	for i := range tail50.Msgs {
		tail50.Msgs[i].Role = "tool"
	}
	if got1, got50 := usageSegments(tail1), usageSegments(tail50); !reflect.DeepEqual(got1, got50) {
		t.Fatalf("whole-log 数字随 tail 改变：tail1=%q tail50=%q", got1, got50)
	}
}

func TestCompactionOnlyChangesRawContext(t *testing.T) {
	data := pluginapi.Snapshot{
		UsageTotal:    pluginapi.Usage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
		ContextTokens: 2000,
	}
	before := usageSegments(data)
	data.ContextTokens = 100
	after := usageSegments(data)
	wantBefore := []string{"总量 150", "上下文 2.0k"}
	wantAfter := []string{"总量 150", "上下文 100"}
	if !reflect.DeepEqual(before, wantBefore) || !reflect.DeepEqual(after, wantAfter) {
		t.Fatalf("压缩归约错误：before=%q want=%q；after=%q want=%q", before, wantBefore, after, wantAfter)
	}
}

func TestSessionSnapshotsDoNotReuseNumbers(t *testing.T) {
	a := pluginapi.Snapshot{
		Cur: "session-a", UsageTotal: pluginapi.Usage{TotalTokens: 12345}, ContextTokens: 2000,
	}
	b := pluginapi.Snapshot{
		Cur: "session-b", UsageTotal: pluginapi.Usage{TotalTokens: 777}, ContextTokens: 100,
	}
	if gotA, gotB := usageSegments(a), usageSegments(b); reflect.DeepEqual(gotA, gotB) {
		t.Fatalf("A/B 会话数字相同：%q", gotA)
	}
	empty := usageSegments(pluginapi.Snapshot{Cur: "session-b"})
	if len(empty) != 0 {
		t.Fatalf("B 的空快照复用了 A 的数字：%q", empty)
	}
}
