package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/internal/uiplugins"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dssettings"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
)

// TestSettingsPluginIsInstalledBeforeKeyConsumers 钉住装配顺序——
// 这不是洁癖，是**行为**：
//
//	kernel.HandleKey 按装配顺序把按键交给插件，第一个返回 true 的赢。
//
// `ds-settings` 只在「浮层开着」时吃 Esc/↑↓/Enter。如果它排在 `ds-composer` 后面，
// composer 会先看到 Esc —— 那在它那里是"停止当前 turn"（见 dscomposer.HandleKey），
// 于是**浮层永远关不掉，而且按一下 Esc 会打断正在跑的 turn**。
// 同理普通 F2 必须落到 `ds-commands`：设置只认 Shift+F2，落在它前面也不会抢 F2。
//
// 这条顺序是 main() 里那行 `plugins = append([]pluginapi.Plugin{m.settings}, plugins...)`
// 保证的。有人把它改成 append 到末尾，这个测试就红。
func TestSettingsPluginIsInstalledBeforeKeyConsumers(t *testing.T) {
	// 和 main() 用同一个来源、同一个 skip 判据（壳自己注入实例的那几个）。
	plugins, err := uiplugins.Load("", pluginapi.Builtin(), func(id string) bool {
		return id == "ds-sidebar" || id == "ds-conversation" || id == "ds-core" || id == "ds-shell" || id == "ds-settings"
	})
	if err != nil {
		t.Fatalf("插件来源加载失败: %v", err)
	}
	settings := &dssettings.Plugin{}
	plugins = append([]pluginapi.Plugin{settings}, plugins...)
	// main() 里壳自己注入实例的那几个：侧栏（↑↓ 切会话）与会话流。
	plugins = append(plugins, &dssidebar.Plugin{}, &dsconversation.Plugin{})
	// fixedFirst 的两块地基（main() 里同样在最后追加）。
	plugins = append(plugins, &dscore.Plugin{Dark: true}, &dsshell.Plugin{})

	kernel := uikernel.New("settings-order")
	if err := kernel.Load(orderPlugins(plugins)...); err != nil {
		t.Fatalf("装配失败: %v", err)
	}

	pos := map[string]int{}
	for i, id := range kernel.Order() {
		pos[id] = i
	}
	self, ok := pos["ds-settings"]
	if !ok {
		t.Fatal("ds-settings 没被装配（import 丢了吗）")
	}
	for _, consumer := range []string{"ds-composer", "ds-commands", "ds-sidebar"} {
		other, ok := pos[consumer]
		if !ok {
			t.Fatalf("%s 不在装配结果里，这个守卫会失去意义", consumer)
		}
		if self > other {
			t.Fatalf("ds-settings 排在 %s 之后（%d > %d）：浮层开着时 Esc/↑↓ 会先被它吃掉",
				consumer, self, other)
		}
	}
}
