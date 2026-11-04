package dsschedule

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// TestShortTime 验时间压缩。
//
// **不做时区换算**：宿主给的已经是带偏移的绝对时刻，这里只是变短。
// 换成本机时区反而会让"我设的 9 点"变成别的数字。
func TestShortTime(t *testing.T) {
	cases := map[string]string{
		"2026-11-05T09:00:00+08:00": "11-05 09:00",
		"2026-01-01T00:00:00Z":      "01-01 00:00",
		// 格式不对就**原样返回**，不猜、不补零：
		// 猜错的半截时间比"原样"更难发现。
		"":                 "",
		"2026-11-05":       "2026-11-05",
		"not-a-time":       "not-a-time",
		"2026/11/05 09:00": "2026/11/05 09:00",
	}
	for in, want := range cases {
		if got := shortTime(in); got != want {
			t.Errorf("shortTime(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTitleFallsBackToKind 验没标题时退回种类。
//
// 显示一行空白比显示 `daily` 更糟：空白让人以为数据坏了，
// `daily` 至少说明"这是一条每天的任务"。
func TestTitleFallsBackToKind(t *testing.T) {
	if got := title(pluginapi.ScheduleItem{Title: "每日巡检", Kind: "daily"}); got != "每日巡检" {
		t.Fatalf("有标题时该用标题，实际 %q", got)
	}
	if got := title(pluginapi.ScheduleItem{Kind: "daily"}); got != "daily" {
		t.Fatalf("没标题时该退回种类，实际 %q", got)
	}
	if got := title(pluginapi.ScheduleItem{}); got != "(未命名)" {
		t.Fatalf("都没有时该有个兜底，实际 %q", got)
	}
}

// TestItoa 验数字转换（侧栏「还有 N 条」用）。
func TestItoa(t *testing.T) {
	for in, want := range map[int]string{0: "0", 1: "1", 9: "9", 10: "10", 123: "123"} {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}
