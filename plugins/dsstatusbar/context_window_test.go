package dsstatusbar

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// STATS-WIRE：只有占用与正容量同时已知才显示真实百分比，异常不得钳成 100%。
func TestContextWindowSnapshotPercentageAndAnomalies(t *testing.T) {
	tests := []struct {
		name      string
		used      int64
		window    int64
		want      string
		noPercent bool
	}{
		{name: "真实已知", used: 2000, window: 10000, want: "上下文 2.0k · 20%"},
		{name: "真实零占用", used: 0, window: 10000, want: "上下文 0 · 0%"},
		{name: "容量未知只显示原始值", used: 2000, window: 0, want: "上下文 2.0k", noPercent: true},
		{name: "负容量不编百分比", used: 2000, window: -10000, want: "上下文 2.0k", noPercent: true},
		{name: "负占用说明异常", used: -2000, window: 10000, want: "上下文 -2000（异常）", noPercent: true},
		{name: "超容量说明异常且不钳一百", used: 12000, window: 10000, want: "上下文 12.0k / 10.0k（超出容量）", noPercent: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(usageSegments(pluginapi.Snapshot{
				ContextTokens: tc.used, ContextWindow: tc.window,
			}), " · ")
			if got != tc.want {
				t.Fatalf("usageSegments() = %q，want %q", got, tc.want)
			}
			if tc.noPercent && strings.Contains(got, "%") {
				t.Fatalf("异常/未知容量不得显示百分比：%q", got)
			}
			if strings.Contains(got, "轮") || strings.Contains(got, "步") || strings.Contains(got, "tok/s") || strings.Contains(got, "缓存命中") {
				t.Fatalf("contextWindow 接线不得编造其他指标：%q", got)
			}
		})
	}
}
