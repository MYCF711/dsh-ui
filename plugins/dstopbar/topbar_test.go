package dstopbar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar/viewapi"
)

func TestConversationViewStoreIsPerSession(t *testing.T) {
	p := &Plugin{}
	if got := p.Active("a"); got != viewapi.Chat {
		t.Fatalf("未选择会话视图时默认 = %q，want chat", got)
	}
	p.Select("a", viewapi.Trajectory)
	p.Select("b", viewapi.Chat)
	if got := p.Active("a"); got != viewapi.Trajectory {
		t.Fatalf("A 的选择 = %q，want trajectory", got)
	}
	if got := p.Active("b"); got != viewapi.Chat {
		t.Fatalf("B 被 A 串扰：%q", got)
	}
	// 空会话不写：这一条是**存储本身**的契约，没变。
	p.Select("", viewapi.Trajectory)
	if got := p.Active("a"); got != viewapi.Trajectory {
		t.Fatalf("空会话的选择串到了 A：%q", got)
	}
}

// TestConversationViewStoreKeepsRegisteredIDs 验存储**不再自带名单**。
//
// 2026-11-15 之前这里断言的是「Select 拒绝 chat/trajectory 之外的值」。那条
// 规则的落点搬走了：**准入归注册表**（`uikernel.ViewTabs` 从内核的
// `conversation.view` 占用方投影），壳在写之前先查名录
// （`cmd/app/view_tabs_wiring.go` 的 selectViewKey / activeViewKey，
// 闸门断言在 `cmd/app/view_tabs_wiring_test.go`）。
//
// 为什么必须搬：名单写死在存储里时，「插件注册了一个新 view」在界面上永远不出现
// ——闸门在错误的地方。存储现在只保证「空 session 不持久化」这一条。
func TestConversationViewStoreKeepsRegisteredIDs(t *testing.T) {
	p := &Plugin{}
	p.Select("s", pluginapi.ViewStats)
	if got := p.Active("s"); got != pluginapi.ViewStats {
		t.Fatalf("注册过的 view 应当存得下，实际 %q", got)
	}
}
