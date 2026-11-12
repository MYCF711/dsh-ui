// official_slot.go —— 把本插件的官方槽位登记进 uikernel 的 rc.2 官方槽表。
//
// 为什么需要这一步：`pluginapi.Slot` 只是"能挂上去的字符串"，它回答不了
// "官方结构树里这个槽到底有没有对应节点"。GA 的 `uikernel` 槽表把这件事变成
// 可断言的数据（`OfficialSlots()` / `LookupOfficialSlot` / `RegisterOfficialSlot`），
// 所以 rc.2 新插件的槽位用 `RegisterOfficialSlot` 补进去，而不是留在插件里当裸字符串。
//
// 幂等：同一进程里多次装配只会登记一次；表里已有同名声明时**校验后跳过**
// （`RegisterOfficialSlot` 对重名是 fail-loud 错误，重复调必须自己挡住）。
package dsnotifications

import (
	"sync"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

var (
	officialSlotOnce sync.Once
	officialSlotErr  error
)

// RegisterOfficialSlot 登记 `settings.launcher.notifications`（官方 rc.2 新增）。
//
// 官方声明方是 `ui-settings-account`（children 声明 `index.ts:316`），
// 渲染方是 `AccountMenu.tsx:60`；本插件是**消费方**（mount 在 `src/client/index.ts:64-67`）。
//
// 父槽说明：官方父级是账号菜单里的 `settings.launcher`（属于设置模态，不在首屏
// 槽表里）。这里挂到 `root` 名下并在 Note 写明真实父级——槽表只描述结构，
// 不改变实际绘制位置（绘制仍由父级 `drawSlot` 决定）。
func RegisterOfficialSlot() error {
	officialSlotOnce.Do(func() {
		if _, ok := uikernel.LookupOfficialSlot(SlotSettingsLauncherNotifications); ok {
			return
		}
		officialSlotErr = uikernel.RegisterOfficialSlot(uikernel.SlotSpec{
			Name:   SlotSettingsLauncherNotifications,
			Kind:   uikernel.KindSingle,
			Scope:  uikernel.ScopeRoot,
			Parent: uikernel.OffRoot,
			Host:   "AccountMenu.notifications",
			Anchor: "ui-settings-account/src/client/index.ts:316",
			Note: "single/root；官方父级是设置模态里的 settings.launcher（账号菜单，" +
				"ui-settings-account/src/client/AccountMenu.tsx:60 用 hidden={!wide} 包住），" +
				"该父槽不属于首屏槽表，故这里挂 root 名下",
		})
	})
	return officialSlotErr
}
