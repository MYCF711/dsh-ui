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
// 父槽：官方真实父级是 `settings.launcher`（账号菜单自己的挂点），而
// `settings.launcher` 又是 `sidebar.settings` 的 children。两级父容器都由
// `launcher.go` 实现，所以这里按**真实层级**登记（父槽必须先登记——uikernel
// 对"父槽未登记"是 fail-loud，这正是我们想要的：层级写错就装配失败）。
func RegisterOfficialSlot() error {
	officialSlotOnce.Do(func() {
		// 先补一级父槽 `settings.launcher`（幂等）；没有它子槽登记会被拒。
		if err := registerLauncherSlots(); err != nil {
			officialSlotErr = err
			return
		}
		if _, ok := uikernel.LookupOfficialSlot(SlotSettingsLauncherNotifications); ok {
			return
		}
		officialSlotErr = uikernel.RegisterOfficialSlot(uikernel.SlotSpec{
			Name:   SlotSettingsLauncherNotifications,
			Kind:   uikernel.KindSingle,
			Scope:  uikernel.ScopeRoot,
			Parent: SlotSettingsLauncher,
			Host:   "AccountMenu.notifications",
			Anchor: "ui-settings-account/src/client/index.ts:316",
			Note: "single/root；官方父级是账号菜单里的 settings.launcher（AccountMenu.tsx:60 的 " +
				"div.notifications，hidden={!wide}，bottom:calc(100% + 4px)）——父链 " +
				"sidebar.settings → settings.launcher → 本槽由 launcher.go 两级实现",
		})
	})
	return officialSlotErr
}
