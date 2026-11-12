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
	// 判据从"槽上只有 1 块"改成"**这四个 ID** 各在各自的槽上"：`shell.overlay`
	// 会合法地多一块（子代理目录面板 agent-team.catalog-panel 也挂同一层），
	// 数量断言会把"多了一块正常的画"当成失败。
	for slot, id := range map[pluginapi.Slot]string{
		SlotSubagentFooter:    "agent-team.board",
		SlotInputModelOptions: "agent-team.toggle",
		SlotDisableOverlay:    "agent-team.disable",
		SlotChatCommandView:   "agent-team.command",
	} {
		found := 0
		for _, painter := range k.Painters(slot) {
			if painter.ID == id {
				found++
			}
		}
		if found != 1 {
			t.Fatalf("槽 %q 上应恰好 1 块 %q，得到 %d（该槽实际有 %d 块）",
				slot, id, found, len(k.Painters(slot)))
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
