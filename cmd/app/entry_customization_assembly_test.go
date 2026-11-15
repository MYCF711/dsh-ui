package main

// entry_customization_assembly_test.go —— #17 的**装配级**证据（走产品装配路径）。
//
// # 它守的是什么
//
// 本仓的两个 More 菜单座位（`sidebar.panellist.actions` /
// `conversation.session.header.views.actions`）只有在**有占用方**时宿主才会画出入口
// （官方 `hasEntryActions`）。所以"内容插件在不在 Builtin 里、装没装上、座位有没有
// 占用方"这三件事只要断一环，界面上就一行都看不到——而且不报错。
//
// 这里按 `main()` 的同一条路径装配（`uiplugins.Load` + `pluginapi.Builtin` +
// `orderPlugins` + `uikernel.Load`），断言三件事：
//
//	① `ds-entry-customization` 真的在装配后的插件表里
//	② 两个座位各有一份占用方（否则 More 入口根本不出现）
//	③ 面板导航能力（`ui.pluginNavigation`）在装配里真的可注入，且能开插件列表
//
// 与 `file_actions_assembly_test.go` 同一套写法（keep 白名单 + 手动追加两个地基实例）。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/internal/uiplugins"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	_ "github.com/MYCF711/dsh-ui/plugins/dsentrycustom"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	_ "github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar/panelapi"
)

func TestEntryCustomizationOccupiesBothMoreSeatsInProductionAssembly(t *testing.T) {
	keep := map[string]bool{
		"ds-entry-customization": true,
		"ds-sidebar":             true,
		"ds-core":                true,
		"ds-shell":               true,
	}
	plugins, err := uiplugins.Load("", pluginapi.Builtin(), func(id string) bool {
		return !keep[id]
	})
	if err != nil {
		t.Fatalf("插件来源加载失败：%v", err)
	}
	plugins = append(plugins, &dscore.Plugin{Dark: true}, &dsshell.Plugin{})

	kernel := uikernel.New("entry-customization-assembly")
	if err := kernel.Load(orderPlugins(plugins)...); err != nil {
		t.Fatalf("装配失败（ds-entry-customization 的依赖顺序断了？）：%v", err)
	}

	// ① 插件在册。
	found := false
	for _, p := range kernel.Plugins() {
		if p.ID() == "ds-entry-customization" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("装配后的插件表里没有 ds-entry-customization：%v", kernel.Order())
	}

	// ② 两个座位各有一份占用方（宿主按这个判据画 More 入口）。
	for _, slot := range []pluginapi.Slot{
		uikernel.OffSidebarPanelActions,
		uikernel.OffSessionHeaderViewAction,
	} {
		if painters := kernel.Painters(slot); len(painters) == 0 {
			t.Errorf("座位 %q 没有占用方：官方 More 入口（hasEntryActions）不会出现", slot)
		}
	}

	// ③ 面板导航能力可注入（「浏览更多插件」行按它存在与否决定画不画）。
	service := kernel.Get(panelapi.ServiceKey)
	nav, ok := service.(panelapi.Navigation)
	if !ok || nav == nil {
		t.Fatalf("装配后 %s = %T，want panelapi.Navigation（ds-sidebar 没提供？）",
			panelapi.ServiceKey, service)
	}
	// 调一次：实现走的是 ds-sidebar 自己那条 `openPanel` 路径，不该 panic。
	nav.OpenList()
}
