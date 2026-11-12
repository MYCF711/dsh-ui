// official_slots.go —— 把本插件的三个 rc.2 新槽位登记进 uikernel 官方槽表。
//
// 第 4 个落点 `shell.overlay` 是**官方既有槽**，uikernel 的内置槽表里已经登记过
// （`pluginapi.SlotShellOverlay`），这里不重复登记（重名会被 RegisterOfficialSlot
// 判 fail-loud）。
//
// 父槽：三者在官方结构树里都挂在既有槽下，所以能按真实父级登记——
//
//	input.model.options      ← conversation.input.model（ui-model-selection 声明）
//	...actions.subagent.footer ← conversation.session.header.actions（官方中间层
//	                             `...actions.subagent` 由 ui-subagent 声明，不在首屏槽表）
//	chat.commandview          ← conversation.content（ui-chat 的命令节点渲染点）
package dsagentteam

import (
	"sync"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

var (
	officialSlotsOnce sync.Once
	officialSlotsErr  error
)

// RegisterOfficialSlots 登记槽 ②③④（槽 ① 的 shell.overlay 已在内置表里）。
//
// 幂等：同一进程多次装配只登记一次；已存在同名声明时跳过。
func RegisterOfficialSlots() error {
	officialSlotsOnce.Do(func() {
		for _, spec := range []uikernel.SlotSpec{
			{
				Name:   SlotInputModelOptions,
				Kind:   uikernel.KindList,
				Scope:  uikernel.ScopeSession,
				Parent: uikernel.OffInputModel,
				Host:   "ModelSelect.options",
				Anchor: "ui-model-selection/src/client/index.ts:177",
				Note:   "list/session；官方渲染点是 ModelSelect.tsx:505 的 renderSlot，本包消费（mount.ts:57-65）",
			},
			{
				Name:   SlotSubagentFooter,
				Kind:   uikernel.KindList,
				Scope:  uikernel.ScopeSession,
				Parent: uikernel.OffSessionHeaderActions,
				Host:   "SubagentCatalog.footer",
				Anchor: "ui-subagent/src/client/index.ts:105",
				Note: "list/session；官方中间层 conversation.session.header.actions.subagent " +
					"由 ui-subagent 声明（slots.ts:6-9），不在首屏槽表，故挂在父层 header.actions 下",
			},
			{
				Name:   SlotChatCommandView,
				Kind:   uikernel.KindKeyed,
				Scope:  uikernel.ScopeSession,
				Parent: uikernel.OffConvContent,
				Host:   "Chat.commandNode",
				Anchor: "ui-chat/src/client/chat/register-node-renderers.ts:74",
				Note:   "keyed/session，key 是命令名（本包占 team）；渲染点 CommandNodeView.tsx:18",
			},
		} {
			if _, ok := uikernel.LookupOfficialSlot(spec.Name); ok {
				continue
			}
			if err := uikernel.RegisterOfficialSlot(spec); err != nil {
				officialSlotsErr = err
				return
			}
		}
	})
	return officialSlotsErr
}
