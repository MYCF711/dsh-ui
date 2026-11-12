// assembly_test.go —— 装配级证据：4 个官方槽位真的挂进了 kernel。
//
// 判据：Load(ds-core, 本插件) 之后，四个槽位各自恰好一块画，且 ServiceKey
// 能在内核里取到（父级/壳靠它喂投影与命令节点）。
package dsagentteam

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
)

func TestKernelAssemblyRegistersFourSlots(t *testing.T) {
	k := uikernel.New("test")
	if err := k.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	for _, slot := range []pluginapi.Slot{
		SlotSubagentFooter, SlotInputModelOptions, SlotDisableOverlay, SlotChatCommandView,
	} {
		if got := len(k.Painters(slot)); got != 1 {
			t.Fatalf("槽 %q 上应恰好 1 块画，得到 %d", slot, got)
		}
	}
	if v := k.Get(ServiceKey); v == nil {
		t.Fatal("ServiceKey 没有提供给下游")
	} else if _, ok := v.(*Store); !ok {
		t.Fatalf("ServiceKey 的值应为 *Store，得到 %T", v)
	}
}

func TestPluginIsRegisteredByName(t *testing.T) {
	found := false
	for _, id := range pluginapi.Registered() {
		if id == "ds-agent-team" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("插件 id 未登记（register.go 的 init 没生效？）：%v", pluginapi.Registered())
	}
}
