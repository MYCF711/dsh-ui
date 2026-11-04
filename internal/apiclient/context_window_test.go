package apiclient

import (
	"encoding/json"
	"testing"
)

// STATS-WIRE：session.load 的容量上限必须与最近 assistant 的占用分别解码。
func TestLoadResultDecodesContextWindowIndependently(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantUsed   int64
		wantWindow int64
	}{
		{name: "真实容量", raw: `{"contextTokens":2000,"contextWindow":10000}`, wantUsed: 2000, wantWindow: 10000},
		{name: "字段缺失是未知", raw: `{"contextTokens":2000}`, wantUsed: 2000, wantWindow: 0},
		{name: "真实零占用保留", raw: `{"contextTokens":0,"contextWindow":10000}`, wantUsed: 0, wantWindow: 10000},
		{name: "异常负数原样保留", raw: `{"contextTokens":-1,"contextWindow":-10000}`, wantUsed: -1, wantWindow: -10000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got LoadResult
			if err := json.Unmarshal([]byte(tc.raw), &got); err != nil {
				t.Fatal(err)
			}
			if got.ContextTokens != tc.wantUsed || got.ContextWindow != tc.wantWindow {
				t.Fatalf("load 解码 = used %d window %d，want used %d window %d",
					got.ContextTokens, got.ContextWindow, tc.wantUsed, tc.wantWindow)
			}
		})
	}
}
