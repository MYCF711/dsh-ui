package dsbrowser

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
)

// TestBrowserTabTypeEntersTheRightTabRoster 证明"机制已建、页面已挂"这件事
// 在**真实装配链**上成立，而不是"文件写了没接上"。
//
// 右栏 tab 是两阶段注册（OFF:dsh-client-ui-sidebar-right/src/client/index.ts:22-26）：
// ① 类型进 `sidebarRightTabs`；② 体进 keyed 座位。本仓的类型名录由壳从
// keyed 槽的占用方投影出来（`internal/uikernel/view_tabs.go:97` 的 `RightTabs`），
// 所以"注册了 Painter"就等于"进了名录"——这条测试盯的就是这一步。
//
// 它同时钉住芯片文案：官方 `type.label` 的中文是「浏览器」
// （OFF:dsh-client-ui-sidebar-browser/src/client/locales.ts:4）。
func TestBrowserTabTypeEntersTheRightTabRoster(t *testing.T) {
	kernel := uikernel.New("browser-tabs")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	tabs := uikernel.RightTabs(kernel)
	if len(tabs) == 0 {
		t.Fatal("右栏 tab 名录是空的：Painter 没进内核")
	}
	var found bool
	for _, tab := range tabs {
		if tab.Key == RightTabBrowser {
			found = true
			if tab.Label != browserTypeLabel {
				t.Fatalf("芯片文案 = %q，应当是官方 type.label 的 %q", tab.Label, browserTypeLabel)
			}
		}
	}
	if !found {
		t.Fatalf("名录里没有 %q：%+v", RightTabBrowser, tabs)
	}

	// 激活判据：只有一枚时直接选它（官方 `defaultSeed` 的单条目分支，
	// `contract/seed.ts:26-33`；本仓移植在 internal/uikernel/view_tabs.go:162）。
	active, ok := uikernel.ResolveActiveRightTab(tabs, "")
	if !ok || active.Key != RightTabBrowser {
		t.Fatalf("单选解析 = %+v/%v，应当是 browser", active, ok)
	}
	active, ok = uikernel.ResolveActiveRightTab(tabs, RightTabBrowser)
	if !ok || active.Key != RightTabBrowser {
		t.Fatalf("指定 browser 解析 = %+v/%v", active, ok)
	}
}
