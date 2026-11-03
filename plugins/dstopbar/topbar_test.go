package dstopbar

import (
	"testing"
)

// TestHostLabel 钉住顶栏上宿主字段的显示规则（§12 H4/H5）。
//
// 规则只有一条：**空串 = 不知道，整段不画**。分组断言：
//   - 两个都空 → 返回空串（不是画一个空的）；
//   - 只有模型 → 只画模型；
//   - 只有权限 → 只画"审批 X"（带前缀，免得和 mode 混）；
//   - 只有空白字符也当空（宿主偶尔会回 " "，那不是模型名）。
func TestHostLabel(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		permission string
		want       string
	}{
		{name: "都空", model: "", permission: "", want: ""},
		{name: "只有模型", model: "deepseek-v4", permission: "", want: "deepseek-v4"},
		{name: "只有权限", model: "", permission: "ask", want: "审批 ask"},
		{name: "都有", model: "deepseek-v4", permission: "deny", want: "deepseek-v4 · 审批 deny"},
		{name: "空白算空", model: "  ", permission: "\t", want: ""},
		{name: "两边带空白要修掉", model: " m ", permission: " auto ", want: "m · 审批 auto"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hostLabel(tc.model, tc.permission); got != tc.want {
				t.Fatalf("hostLabel(%q,%q) = %q，want %q", tc.model, tc.permission, got, tc.want)
			}
		})
	}
}
