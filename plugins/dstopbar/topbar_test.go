package dstopbar

import (
	"testing"

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
	p.Select("", viewapi.Trajectory)
	p.Select("b", "theme-fish")
	if got := p.Active("b"); got != viewapi.Chat {
		t.Fatalf("未知扩展/空会话不应改状态，B=%q", got)
	}
}
