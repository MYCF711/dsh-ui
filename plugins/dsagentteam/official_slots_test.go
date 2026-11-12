// official_slots_test.go —— 三个 rc.2 新槽位都进了 uikernel 官方槽表，
// 且第 4 个落点（shell.overlay）**不被重复登记**（那是既有槽）。
package dsagentteam

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

func TestOfficialSlotTableEntries(t *testing.T) {
	if err := RegisterOfficialSlots(); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	cases := []struct {
		name   pluginapi.Slot
		kind   uikernel.SlotKind
		scope  uikernel.SlotScope
		parent pluginapi.Slot
		anchor string
	}{
		{SlotInputModelOptions, uikernel.KindList, uikernel.ScopeSession,
			uikernel.OffInputModel, "ui-model-selection/src/client/index.ts:177"},
		{SlotSubagentFooter, uikernel.KindList, uikernel.ScopeSession,
			uikernel.OffSessionHeaderActions, "ui-subagent/src/client/index.ts:105"},
		{SlotChatCommandView, uikernel.KindKeyed, uikernel.ScopeSession,
			uikernel.OffConvContent, "ui-chat/src/client/chat/register-node-renderers.ts:74"},
	}
	for _, tc := range cases {
		spec, ok := uikernel.LookupOfficialSlot(tc.name)
		if !ok {
			t.Fatalf("官方槽表里没有 %q", tc.name)
		}
		if spec.Kind != tc.kind || spec.Scope != tc.scope {
			t.Fatalf("%q 形态应为 %v/%v，得到 %v/%v", tc.name, tc.kind, tc.scope, spec.Kind, spec.Scope)
		}
		if spec.Parent != tc.parent {
			t.Fatalf("%q 父槽应为 %q，得到 %q", tc.name, tc.parent, spec.Parent)
		}
		if !strings.Contains(spec.Anchor, tc.anchor) {
			t.Fatalf("%q 锚点应为 %q，得到 %q", tc.name, tc.anchor, spec.Anchor)
		}
	}

	// 第 4 个落点是既有槽：必须本来就在表里，且不是我们登记的重复条目。
	if _, ok := uikernel.LookupOfficialSlot(pluginapi.SlotShellOverlay); !ok {
		t.Fatal("shell.overlay 应在 uikernel 内置槽表里")
	}
}

func TestOfficialSlotRegistrationIsIdempotent(t *testing.T) {
	if err := RegisterOfficialSlots(); err != nil {
		t.Fatalf("第一次登记失败：%v", err)
	}
	before := len(uikernel.OfficialSlots())
	if err := RegisterOfficialSlots(); err != nil {
		t.Fatalf("重复登记必须幂等，得到错误：%v", err)
	}
	if after := len(uikernel.OfficialSlots()); after != before {
		t.Fatalf("重复登记不该新增条目：%d → %d", before, after)
	}
}
