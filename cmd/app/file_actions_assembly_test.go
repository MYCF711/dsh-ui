package main

// file_actions_assembly_test.go —— #18 的**装配级**证据（走产品装配路径，不是单测替身）。
//
// # 它守的是什么
//
// `plugins/dsfileactions` 的菜单服务是**可选注入**（不写进 Requires，理由见那个包
// 注释：用户可以在插件页停用 ds-contextmenu，那时整棵插件树不该起不来）。可选注入
// 的风险是"装配顺序不对 → 服务是 nil → 菜单静默不存在"。
//
// 所以这里按 `main()` 的同一条路径装配（`uiplugins.Load` + `pluginapi.Builtin` +
// `orderPlugins` + `uikernel.Load`），再断言两个 target 真的挂上了——这是"生产里
// 这条路走得通"的唯一证据。
//
// `uiplugins.Load` 的第三个参数是 **skip**（`true` = 跳过），与
// settings_order_test.go 同一口径：把要用的那三个留在 Builtin 里，壳自己注入实例的
// 两个地基（ds-core / ds-shell）跳过并手动追加。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/internal/uiplugins"
	_ "github.com/MYCF711/dsh-ui/plugins/dscontextmenu"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	_ "github.com/MYCF711/dsh-ui/plugins/dsfileactions"
	_ "github.com/MYCF711/dsh-ui/plugins/dsfiles"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
)

func TestFileActionsRegisterFileAndImageTargetsInProductionAssembly(t *testing.T) {
	keep := map[string]bool{
		"ds-contextmenu": true, "ds-fileactions": true, "ds-files": true,
		"ds-core": true, "ds-shell": true,
	}
	plugins, err := uiplugins.Load("", pluginapi.Builtin(), func(id string) bool {
		// skip = true 表示**跳过**：把要用的那五个留下，其余全跳过（最小装配）。
		return !keep[id]
	})
	if err != nil {
		t.Fatalf("插件来源加载失败：%v", err)
	}
	plugins = append(plugins, &dscore.Plugin{Dark: true}, &dsshell.Plugin{})

	kernel := uikernel.New("file-actions-assembly")
	if err := kernel.Load(orderPlugins(plugins)...); err != nil {
		t.Fatalf("装配失败（ds-fileactions 与 ds-contextmenu 的注入顺序断了？）：%v", err)
	}
	value := kernel.Get(pluginapi.KeyContextMenus)
	menus, ok := value.(pluginapi.ContextMenus)
	if !ok || menus == nil {
		t.Fatalf("装配后 %s = %T，want ContextMenus", pluginapi.KeyContextMenus, value)
	}
	for _, kind := range []string{pluginapi.TargetFile, pluginapi.TargetImage} {
		if !menus.HasTarget(kind) {
			t.Errorf("target %q 没有贡献者：右键菜单不会弹（ds-fileactions 没拿到菜单服务？）", kind)
		}
	}
	// 官方两个 target 之外的 kind 不该被这个包顺带声明（防手滑复制）。
	for _, kind := range []string{pluginapi.TargetSession, pluginapi.TargetWorkspace} {
		if menus.HasTarget(kind) {
			t.Errorf("target %q 被 ds-fileactions 声明了，但它只拥有 file/image", kind)
		}
	}
}
