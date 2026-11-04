package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// forkGuard 是"现在能不能分支"的那条判断。
//
// 为什么要有它：空会话在宿主那边是**500**（`分支失败：源会话没有可继承的 surface`），
// 而那句是内部术语、状态码也不对（严格说该是 409）。用户看到只会莫名其妙，
// 而他自己没做错任何事——只是这个会话还是空的。
//
// 这条判断的真实行为来自 `scripts/accept-session-ops.sh`（真宿主实测），不是猜。
func TestForkGuard(t *testing.T) {
	msg := apiclient.Message{Role: "user", Text: "你好"}

	cases := []struct {
		name string
		cur  string
		msgs []apiclient.Message
		want string // 空 = 可以分支
	}{
		{"没有当前会话", "", []apiclient.Message{msg}, "没有当前会话，分支不了"},
		{"空会话不能分支", "s1", nil, "这个会话还是空的，没有可分支的内容"},
		{"有一条消息就可以", "s1", []apiclient.Message{msg}, ""},
		{"工具消息也算内容", "s1", []apiclient.Message{{Role: "tool", Text: "x"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := forkGuard(tc.cur, tc.msgs); got != tc.want {
				t.Fatalf("forkGuard = %q，want %q", got, tc.want)
			}
		})
	}
}
