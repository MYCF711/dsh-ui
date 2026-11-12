package main

// hf_link_test.go —— 壳把"两枚布尔事实"压成连接三态的那一处（`linkStateOf`）。
//
// 为什么值得单独钉：界面文案「已连接 / 未知 / 连接异常」全部由这三个值决定。
// 错一档的后果不是崩溃，是**界面说谎**——这个仓已经踩过"宿主被杀 33 秒后
// 界面还写「已连接 · 就绪」"。所以三条边界各自断言一次：
//
//	两个都假 → 未知（还没有任何事实，最容易被误当成"连着"的那一态）
//	成功为真 → 已连接
//	失败为真 → 断开（**优先于成功**：连过又断时两个都为真，结论必须是断）

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

func TestLinkStateOf(t *testing.T) {
	// 四个事实组合 + 优先级：断线证据 > 连着证据 > 正在重连 > 未知。
	for _, tc := range []struct {
		name       string
		up         bool
		down       bool
		connecting bool
		want       pluginapi.LinkState
	}{
		{"还没拿到事实", false, false, false, pluginapi.LinkUnknown},
		{"订阅成功", true, false, false, pluginapi.LinkConnected},
		{"订阅失败", false, true, false, pluginapi.LinkDown},
		{"连过又断（断线证据优先）", true, true, false, pluginapi.LinkDown},
		// 一次重连尝试正在飞行中：官方 `data-phase=connecting`。
		{"正在重连（首连 / 退避到点 / 点了立即重连）", false, false, true, pluginapi.LinkConnecting},
		{"重连中但上次失败还没清（断线证据优先，先报断）", false, true, true, pluginapi.LinkDown},
		{"连着时又发起重连（连着证据优先，不抖动）", true, false, true, pluginapi.LinkConnected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := linkStateOf(tc.up, tc.down, tc.connecting); got != tc.want {
				t.Fatalf("linkStateOf(up=%v, down=%v, connecting=%v) = %v，want %v",
					tc.up, tc.down, tc.connecting, got, tc.want)
			}
		})
	}
}

// TestLinkZeroValueIsUnknown 钉住"零值 = 不知道"：任何没接这个字段的壳
// （老壳 / 首帧 / 单测里手搓的 Snapshot）都必须是「未知」，而不是被当成已连接。
func TestLinkZeroValueIsUnknown(t *testing.T) {
	var snap pluginapi.Snapshot
	if snap.Link != pluginapi.LinkUnknown {
		t.Fatalf("Snapshot 零值的 Link = %v，want LinkUnknown（零值绝不能是「已连接」）", snap.Link)
	}
	if snap.Link.String() != "unknown" {
		t.Fatalf("LinkUnknown.String() = %q，want unknown（日志/断言用同一套词）", snap.Link.String())
	}
}
