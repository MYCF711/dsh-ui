//go:build darwin || windows || linux

// composer_fit_test.go —— 最小窗口下，审批卡出现时**输入区与底栏必须还在窗内**。
//
// 官方 `ConversationContent` 的结构是"正文滚动区 + composer（flex:none）"：
// 审批卡是 composer 区的接管项，它变高时被压缩的是**正文滚动区**，不是底栏。
//
// 本仓 `dsconversation` 的转写滚动区写了 `MinHeight(160)`（自造的下限，官方
// `overflow:auto` 的正文是 `min-height:0`），而 composer 区是内容驱动高度
// （审批卡 ≈172 + 输入卡 98 + 读数行 26）。窗口 760×520（本仓 MinHeight）下
// 两者相加超过窗口高：滚动区不肯让，于是 composer 整块溢到窗口外——
// 输入框下半截、上下文读数、「N 轮 N 步」全部看不见（2026-11-22 实测
// composerSeat 在 268.5 起、312 高，底边 580.5 > 520）。
//
// 判据全是真窗口量出来的：
//
//	① 每个可点元素都在窗口内（溢出 = 用户点不到）；
//	② 审批的两个按钮、输入框、发送按钮的中心都能命中它自己；
//	③ 底栏读数行真的落在窗口底边以内。
package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// approvalFixture 是"有会话、有消息、有一次待审批"的快照（最小窗口复现用）。
func approvalFixture() pluginapi.Snapshot {
	snap := hitsSessionFixture()
	snap.Approval = &pluginapi.Approval{
		RequestID: "ap-1",
		Tool:      "bash",
		Title:     "bash: rm -rf build",
		Kind:      "execute",
		Options: []pluginapi.ApprovalOption{
			{OptionID: "allow-once", Name: "允许这一次", Kind: "allow_once"},
			{OptionID: "reject-once", Name: "拒绝", Kind: "reject_once"},
		},
	}
	return snap
}

func TestComposerFitsWindowWithApproval(t *testing.T) {
	// 760×520 是本仓窗口的 MinWidth/MinHeight（cmd/app/main.go 的 newConfig）：
	// 用户能把窗口拖到的最小尺寸，不通过就等于"最小窗口下输入区半截没了"。
	const width, height = 760, 520
	frame := openDXFrame(t, hitsFrame{name: "approval-fit", snap: approvalFixture()}, width, height)
	snap := frame.snapshot()

	// ① 溢出：任何可点元素都不能越过窗口边界。
	for _, hit := range snap.Hits {
		if hit.ID.IsZero() || hit.Block || hit.Container || hit.MovesWindow {
			continue
		}
		if hit.Bounds.X < 0 || hit.Bounds.Y < 0 ||
			hit.Bounds.X+hit.Bounds.Width > width+0.5 || hit.Bounds.Y+hit.Bounds.Height > height+0.5 {
			t.Fatalf("%q 溢出窗口：%+v（窗口 %dx%d）——用户点不到它",
				hit.Label, hit.Bounds, width, height)
		}
	}

	// ② 关键控件必须真的点得到（中心命中自己）。
	// 发送在空草稿时是 disabled（官方同样 disable），disabled 命中区本来就不
	// 参与 hover/点击路由，所以那一枚只由 ①（在窗内）保证，不在这里要求命中。
	for _, want := range []string{"允许这一次", "拒绝", "输入"} {
		hit, ok := frame.findHitByLabel(want)
		if !ok {
			t.Fatalf("找不到 %q 的命中区：%v", want, frame.labels())
		}
		cx, cy := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
		if id, hitOK := frame.session.Driver().HitTest(cx, cy); !hitOK || id != hit.ID {
			t.Fatalf("%q 的中心 (%v,%v) 没命中它自己：got=%v want=%v", want, cx, cy, id, hit.ID)
		}
	}

	// ③ 底栏读数行（dsstatusbar 的 dock）必须在窗口底边以内。
	stats, ok := frame.findHitByLabel("2 轮 4 步")
	if !ok {
		t.Fatalf("找不到底栏轮次读数：%v", frame.labels())
	}
	if stats.Bounds.Y+stats.Bounds.Height > height {
		t.Fatalf("底栏读数行在窗口外：%+v（窗口高 %d）", stats.Bounds, height)
	}
	send, ok := frame.findHitByLabel("发送")
	if !ok {
		t.Fatalf("找不到发送按钮：%v", frame.labels())
	}
	if send.Bounds.Y+send.Bounds.Height > height {
		t.Fatalf("发送按钮溢出窗口：%+v（窗口高 %d）", send.Bounds, height)
	}
}

// findHitByLabel 按可访问名找命中区。
func (f *dxFrame) findHitByLabel(label string) (gfx.Hit, bool) {
	for _, hit := range f.snapshot().Hits {
		if hit.Label == label && !hit.ID.IsZero() {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// labels 列出本帧所有有名字的命中区（失败信息用）。
func (f *dxFrame) labels() []string {
	out := make([]string, 0, len(f.snapshot().Hits))
	for _, hit := range f.snapshot().Hits {
		if hit.Label != "" {
			out = append(out, hit.Label)
		}
	}
	return out
}
