// official_slot_test.go —— 槽位进的是 **uikernel 的 rc.2 官方槽表**，不是插件里的裸字符串。
package dsnotifications

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

func TestOfficialSlotTableEntry(t *testing.T) {
	if err := RegisterOfficialSlot(); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	spec, ok := uikernel.LookupOfficialSlot(SlotSettingsLauncherNotifications)
	if !ok {
		t.Fatal("官方槽表里没有 settings.launcher.notifications")
	}
	if spec.Kind != uikernel.KindSingle || spec.Scope != uikernel.ScopeRoot {
		t.Fatalf("官方形态是 single/root，登记成了 %v/%v", spec.Kind, spec.Scope)
	}
	if !strings.Contains(spec.Anchor, "ui-settings-account/src/client/index.ts:316") {
		t.Fatalf("锚点必须是 rc.2 src 行号，得到 %q", spec.Anchor)
	}
	if spec.Host == "" {
		t.Fatal("缺实现节点名（Host）")
	}
}

func TestOfficialSlotRegistrationIsIdempotent(t *testing.T) {
	if err := RegisterOfficialSlot(); err != nil {
		t.Fatalf("第一次登记失败：%v", err)
	}
	before := len(uikernel.OfficialSlots())
	if err := RegisterOfficialSlot(); err != nil {
		t.Fatalf("重复登记必须幂等（uikernel 对重名是 fail-loud），得到错误：%v", err)
	}
	if after := len(uikernel.OfficialSlots()); after != before {
		t.Fatalf("重复登记不该新增条目：%d → %d", before, after)
	}
}
