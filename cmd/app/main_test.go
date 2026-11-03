// main_test.go —— 壳里的**纯函数**单测。
//
// 为什么这个包之前是 0%：壳的大部分代码是"把各组件拼起来"，天然难测，
// 所以一直只有真机验证。但里面有几个纯判断（尺寸解析、按键解析、凭据错误判断、
// 审批选项转换）值得单独钉住——它们出错时的症状都很"远期"：
// 比如 -size 解析把 320 当合法下限、or 把 401 当网络抖动，用户只会看到
// "窗口怪怪的" / "一直连不上"。
package main

import (
	"errors"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

func TestParseWinSize(t *testing.T) {
	cases := []struct {
		in      string
		w, h    int
		wantOK  bool
		comment string
	}{
		{"1600x900", 1600, 900, true, "正常"},
		{"800X600", 800, 600, true, "大写 X"},
		{"1024×768", 1024, 768, true, "乘号"},
		{"320x240", 320, 240, true, "下限刚好"},
		{"319x240", 0, 0, false, "宽度低于下限"},
		{"320x239", 0, 0, false, "高度低于下限"},
		{"", 0, 0, false, "空"},
		{"1600", 0, 0, false, "没有分隔符"},
		{"x900", 0, 0, false, "缺宽"},
		{"1600x", 0, 0, false, "缺高"},
		{"axb", 0, 0, false, "不是数字"},
		{"1600x900x2", 0, 0, false, "多段"},
	}
	for _, c := range cases {
		w, h, ok := parseWinSize(c.in)
		if ok != c.wantOK || (ok && (w != c.w || h != c.h)) {
			t.Errorf("parseWinSize(%q) = (%d,%d,%v)，期望 (%d,%d,%v) [%s]",
				c.in, w, h, ok, c.w, c.h, c.wantOK, c.comment)
		}
	}
}

func TestParseKeys(t *testing.T) {
	keys := parseKeys("down,up,home,end,esc,enter")
	want := []ui.Key{ui.KeyDown, ui.KeyUp, ui.KeyHome, ui.KeyEnd, ui.KeyEscape, ui.KeyEnter}
	if len(keys) != len(want) {
		t.Fatalf("解析出 %d 个键，期望 %d", len(keys), len(want))
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("第 %d 个键 = %v，期望 %v", i, keys[i], want[i])
		}
	}
	// 大小写与空白都要容忍（命令行手写时常见的）
	if got := parseKeys(" DOWN , Esc "); len(got) != 2 || got[0] != ui.KeyDown || got[1] != ui.KeyEscape {
		t.Fatalf("大小写/空白容忍失败：%v", got)
	}
	// 不认识的键静默跳过，不 panic
	if got := parseKeys("down,bogus,up"); len(got) != 2 {
		t.Fatalf("未知键应被跳过，得到 %v", got)
	}
	if got := parseKeys(""); len(got) != 0 {
		t.Fatalf("空串应得到 0 个键，得到 %v", got)
	}
}

func TestIsAuthErr(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"websocket: bad handshake: got 401", true},
		{"failed: got 403 Forbidden", true},
		{"dial tcp: connection refused", false},
		{"timeout", false},
		{"got 404 Not Found", false}, // 4xx 但不是凭据问题
		{"", false},
	}
	for _, c := range cases {
		var err error
		if c.msg != "" {
			err = errors.New(c.msg)
		}
		if got := isAuthErr(err); got != c.want {
			t.Errorf("isAuthErr(%q) = %v，期望 %v", c.msg, got, c.want)
		}
	}
	if isAuthErr(nil) {
		t.Error("nil 不该被判成凭据错误")
	}
}

func TestApprovalOptionsConversion(t *testing.T) {
	in := []apiclient.ApprovalOption{
		{OptionID: "allow_once", Name: "允许这一次", Kind: "allow_once"},
		{OptionID: "allow_always", Name: "这个会话里以后都允许", Kind: "allow_always"},
		{OptionID: "reject_once", Name: "拒绝", Kind: "reject_once"},
	}
	out := approvalOptions(in)
	if len(out) != 3 {
		t.Fatalf("应转换 3 项，得到 %d", len(out))
	}
	// **原样**：id / name / kind 一个都不许改写（插件靠 kind 挑选项）
	for i := range in {
		if out[i].OptionID != in[i].OptionID || out[i].Name != in[i].Name || out[i].Kind != in[i].Kind {
			t.Fatalf("第 %d 项被改写了：%+v → %+v", i, in[i], out[i])
		}
	}
	if got := approvalOptions(nil); got != nil {
		t.Fatalf("nil 输入应返回 nil，得到 %v", got)
	}
}

func TestMaxInt(t *testing.T) {
	if maxInt(3, 5) != 5 || maxInt(5, 3) != 5 || maxInt(-1, -2) != -1 || maxInt(7, 7) != 7 {
		t.Fatal("maxInt 结果不对")
	}
}

var _ = pluginapi.Approval{} // 保持 pluginapi 引用（审批结构参与上面的转换测试）
