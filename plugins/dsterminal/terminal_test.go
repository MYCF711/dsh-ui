package dsterminal

import (
	"errors"
	"fmt"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// TestF6SendsLine 验 F6 把输入框那一行发给终端，并**补上换行**。
//
// 补换行不是小事：终端是行缓冲的，不带 `\n` 它不会执行——
// 用户看到的是"按了 F6 但什么都没发生"。
func TestF6SendsLine(t *testing.T) {
	var got []struct {
		id, data string
		typed    bool
	}
	p := &Plugin{open: true, currentTab: "term-1"}
	p.act = pluginapi.Actions{
		TerminalWriteOfficial: func(id, data string, typed bool) {
			got = append(got, struct {
				id, data string
				typed    bool
			}{id, data, typed})
		},
		Notify: func(string) {},
	}
	p.field.SetText("echo hi")

	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF6}) {
		t.Fatal("F6 应当被消费（否则会冒泡成别的键）")
	}
	if len(got) != 1 {
		t.Fatalf("应当发一次，实际 %d 次", len(got))
	}
	if got[0].id != "term-1" {
		t.Fatalf("发错终端：%q", got[0].id)
	}
	if got[0].data != "echo hi\n" {
		t.Fatalf("要带上换行，实际 %q", got[0].data)
	}
	if !got[0].typed {
		t.Fatal("F6 是真实人员输入，官方 typed 必须为 true")
	}
	if p.field.Text() != "" {
		t.Fatalf("发完要清空输入框，实际 %q", p.field.Text())
	}
}

// TestF6EmptyDoesNothing 验空输入不发送（但**仍然消费掉按键**）。
//
// 不消费的话，空框按 F6 会冒泡成别的键——那比"什么都不做"更让人意外。
func TestF6EmptyDoesNothing(t *testing.T) {
	called := false
	p := &Plugin{open: true, currentTab: "t"}
	p.act = pluginapi.Actions{
		TerminalWriteOfficial: func(string, string, bool) { called = true },
		Notify:                func(string) {},
	}
	p.field.SetText("   ")
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF6}) {
		t.Fatal("空输入也要消费掉按键")
	}
	if called {
		t.Fatal("空输入不该发出去")
	}
}

// TestF6WhenClosedDoesNothing 验面板关着时 F6 不干活（也不消费）。
func TestF6WhenClosedDoesNothing(t *testing.T) {
	p := &Plugin{open: false}
	p.act = pluginapi.Actions{}
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF6}) {
		t.Fatal("关着的时候不该消费 F6")
	}
}

// TestF5Toggles 验 F5 开关。
func TestF5Toggles(t *testing.T) {
	p := &Plugin{}
	p.act = pluginapi.Actions{Notify: func(string) {}}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF5}) {
		t.Fatal("F5 应当被消费")
	}
	if !p.open {
		t.Fatal("第一次 F5 应当打开")
	}
	p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF5})
	if p.open {
		t.Fatal("第二次 F5 应当关闭")
	}
}

func TestTerminalTabIsMultipleAndCloseWaitsForHost(t *testing.T) {
	p := &Plugin{}
	panels := p.Panels()
	if len(panels) == 0 || !panels[0].Multiple {
		t.Fatal("官方 terminal tab 必须 multiple:true")
	}
	wantErr := fmt.Errorf("close failed")
	actions := pluginapi.Actions{TerminalCloseOfficial: func(tabID string) error {
		if tabID != "tab7" {
			t.Fatalf("关闭错 occurrence：%q", tabID)
		}
		return wantErr
	}}
	if err := p.CloseRightTab("s1", "tab7", RightTabTerminal, actions); !errors.Is(err, wantErr) {
		t.Fatalf("Host close 失败必须阻止删 tab：%v", err)
	}
	if err := p.CloseRightTab("s1", "tab7", "browser", pluginapi.Actions{}); err != nil {
		t.Fatalf("其它类型的 close 不归终端插件：%v", err)
	}
}
