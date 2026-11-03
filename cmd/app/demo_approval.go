package main

import (
	"flag"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// 这个文件是**调试用的假数据**，只为一件事：
// 在没有宿主推 approval/request 帧的情况下，也能给审批条截一张真实的图。
//
// 它不是产品路径的一部分：开关关着（默认）时 demoApprovalView 返回 nil，
// 快照里的 Approval 完全来自宿主（H13）。也就是说，它让人**假装**收到审批，
// 所以只有显式加 -demo-approval 才会发生——正式行为不依赖它，也不能依赖它。

// demoApprovalFlag 是**纯调试**开关（见上）。
var demoApprovalFlag = flag.Bool("demo-approval", false,
	"调试：塞一条假的待审批（只用于截图/验收，不连宿主；正式路径不看它）")

// demoApprovalArgs 故意很长：用来验"压成一行 + 省略号 + 不切坏中文"。
const demoApprovalArgs = `{"command":"go test ./... -count=1 -run 'TestApproval|TestComposer' -v && go vet ./... && gofmt -l pluginapi plugins cmd internal/pluginapi && echo 这一串中文是故意超长的，用来确认审批条把参数压成一行并加省略号，而不是把中文切成半个字"}`

// demoApprovalView 在开关打开时返回一条照 H13 帧形状造的审批，否则 nil。
//
// 字段来源见 pluginapi.Approval 的注释（data.requestId / data.toolCall.* / data.options）。
func demoApprovalView() *pluginapi.Approval {
	if !*demoApprovalFlag {
		return nil
	}
	return &pluginapi.Approval{
		RequestID: "perm-demo-1a2b3c",
		Tool:      "bash",
		Title:     "bash: go test ./... && go vet ./...",
		Kind:      "execute",
		Args:      demoApprovalArgs,
		Detail:    "模型自述（仅供参考）：改完审批条，跑一遍测试确认没把输入框的 Enter 抢坏",
		Options: []pluginapi.ApprovalOption{
			{OptionID: "allow-once", Name: "允许这一次", Kind: "allow_once"},
			{OptionID: "allow-always", Name: "这个会话里以后都允许", Kind: "allow_always"},
			{OptionID: "reject-once", Name: "拒绝", Kind: "reject_once"},
		},
	}
}
