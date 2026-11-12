// assembly_test.go —— 装配级证据：插件真的能进 kernel，槽位与服务真的注册上了。
//
// 单测直接 newTestPlugin 是绕过 kernel 的，它证明不了"壳装得上"。这里走
// 真正的 uikernel：Load(ds-core, 本插件) 后，
//
//	① 官方槽上恰好一块画；
//	② ServiceKey 能在内核里取到（壳/父级靠它驱动生命周期）；
//	③ 插件 id 在全局注册表里（没有 register.go 的 init 就装不上）。
package dsnotifications

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
)

func TestKernelAssemblyRegistersSlotAndRuntime(t *testing.T) {
	k := uikernel.New("test")
	if err := k.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	painters := k.Painters(SlotSettingsLauncherNotifications)
	if len(painters) != 1 {
		t.Fatalf("官方槽上应恰好 1 块画，得到 %d", len(painters))
	}
	if painters[0].Draw == nil {
		t.Fatal("画没有 Draw")
	}
	if v := k.Get(ServiceKey); v == nil {
		t.Fatal("ServiceKey 没有提供给下游")
	} else if _, ok := v.(*Controller); !ok {
		t.Fatalf("ServiceKey 的值应为 *Controller，得到 %T", v)
	}
}

func TestPluginIsRegisteredByName(t *testing.T) {
	found := false
	for _, id := range pluginapi.Registered() {
		if id == "ds-account-notifications" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("插件 id 未登记（register.go 的 init 没生效？）：%v", pluginapi.Registered())
	}
}
