package dscommands

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// runWidth 执行名字里带 word 的那条命令，返回它写进去的键值。
func runWidth(t *testing.T, cur map[string]string, word string) map[string]string {
	t.Helper()
	var saved map[string]string
	p := &Plugin{}
	p.act = pluginapi.Actions{
		ConfigGet: func() map[string]string { return cur },
		ConfigSet: func(kv map[string]string) { saved = kv },
		Notify:    func(string) {},
	}
	for _, c := range p.commands() {
		if strings.Contains(c.Name, word) {
			c.Action()
			return saved
		}
	}
	t.Fatalf("没找到名字里带 %q 的命令", word)
	return nil
}

// TestWidthBumpClampsHigh 验**上界**：拖到 600 之后再加就是 600，不是 640。
//
// 夹不住的话，中栏会被挤到 0——那时候界面看起来"还在"，但什么都看不见了。
func TestWidthBumpClampsHigh(t *testing.T) {
	got := runWidth(t, map[string]string{"ui.sidebarWidth": "580"}, "侧栏宽一点")
	if got["ui.sidebarWidth"] != "600" {
		t.Fatalf("580+40 应当夹到 600，实际 %q", got["ui.sidebarWidth"])
	}
}

// TestWidthBumpClampsLow 验**下界**：200 再减是 180，不是 160。
func TestWidthBumpClampsLow(t *testing.T) {
	got := runWidth(t, map[string]string{"ui.sidebarWidth": "200"}, "侧栏窄一点")
	if got["ui.sidebarWidth"] != "180" {
		t.Fatalf("200-40 应当夹到 180，实际 %q", got["ui.sidebarWidth"])
	}
}

// TestWidthBumpUsesFallbackWhenUnset 验**没配置过时用默认值**（264），不报错。
func TestWidthBumpUsesFallbackWhenUnset(t *testing.T) {
	got := runWidth(t, map[string]string{}, "侧栏窄一点")
	if got["ui.sidebarWidth"] != "224" { // 264 - 40
		t.Fatalf("没配置时应当从 264 起算，实际 %q", got["ui.sidebarWidth"])
	}
}

// TestWidthBumpIgnoresGarbage 验配置里是垃圾串（"abc"）时也不崩、退到默认值。
//
// 用户手改过 `ui-config.json` 是很正常的事——那一份是纯文本，任何东西都可能出现。
func TestWidthBumpIgnoresGarbage(t *testing.T) {
	got := runWidth(t, map[string]string{"ui.sidebarWidth": "abc"}, "侧栏宽一点")
	if got["ui.sidebarWidth"] != "304" { // 264 + 40
		t.Fatalf("垃圾值应当退到默认 264，实际 %q", got["ui.sidebarWidth"])
	}
}

// TestNoConfigActionsMeansNoWidthCommands 验壳没接配置时一条都不画。
func TestNoConfigActionsMeansNoWidthCommands(t *testing.T) {
	p := &Plugin{}
	p.act = pluginapi.Actions{}
	for _, c := range p.commands() {
		if strings.Contains(c.Name, "宽一点") || strings.Contains(c.Name, "窄一点") {
			t.Fatalf("没接 ConfigSet 却画出了 %q", c.Name)
		}
	}
}
