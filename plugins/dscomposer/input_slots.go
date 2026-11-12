// input_slots.go —— composer 作为官方 `conversation.input.*` 子槽的**父容器**。
//
// # 官方事实（rc.2 `ui-conversation/src/client/contract/slots.ts`）
//
//	conversation.composer                chain  :290   本插件占这个座位
//	├─ conversation.composer.bar         single :336   InputBar 容器（占用方=本插件，不再派发）
//	├─ conversation.composer.dock        list   :315   Queue/Todo dock（贴卡片上方）
//	└─ .card
//	   ├─ conversation.input.attachments single :341   卡片顶部附件 rail（空不占位）
//	   ├─ .input（编辑器）
//	   │  └─ conversation.input.overlay  list   :310   覆盖层（不占位）
//	   └─ .row
//	      ├─ conversation.input.permission single :355 权限 trigger
//	      ├─ conversation.input.plan       single :350 plan 模式
//	      ├─ conversation.input.left       list   :320 控件行左（尾部）
//	      ├─ conversation.input.right      list   :325 控件行右（头部）
//	      ├─ conversation.input.model      single :362 模型 trigger（模型菜单占用）
//	      └─ conversation.input.activity   single :331 活动 pill
//
// # 为什么由本插件派发、而不是壳
//
// 官方这几个槽是 **InputBar 内部**的位置（卡片顶 / 编辑器上 / 控件行左/右），
// 只有画 InputBar 的那一方知道这些位置。壳（`uikernel.drawComposerSeat`）
// 只负责把 `conversation.composer` 派给占用方（GR 已落地的座位链路），
// 子槽由占用方在自己的布局里派发——这正是 `pluginapi.Painter.Children`
// 存在的意义（父容器拿到内核的 `drawSlot`），与 `plugins/dsagentteam` 的
// 模型菜单/命令节点两个父容器同一套做法，**没有第二条链路**。
//
// # 两个座位、同一份派发
//
// 旧三栏座位（`main.composer`）与官方座位（`conversation.composer`）都调
// `drawLegacyComposerChildren` / `drawOfficialComposerChildren`，两者内部
// 共用 `drawCard` 并在同一批位置派发同一批子槽。区别只有外壳边距与 dock：
//
//	旧三栏  外壳已给中栏 12px，本插件只补差值 4；不画上下文 dock（保持旧外观）
//	官方    左右 clearance 16 由壳给；卡片下方画 dock（输入卡片十项元素之一）
//
// 某一帧没有占用方的槽，画法回退到本插件自己的只读实现（模型 chip / 权限 chip /
// 活动 chip），判据只有一条：`env.Has(slot)`——与壳判 `BridgeLegacyComposer`
// 用的是同一个内核事实（`kernel.Has`）。**同帧只有一个活**：占用方在，本插件
// 就不画；占用方不在，本插件才画。
package dscomposer

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

// composerInputSlotOwned 报告某个官方 composer 子槽这一帧有没有占用方。
//
// 没有 Env.Has（单测/旧壳）时一律按"没有占用方"处理：那样画出的是本插件的
// 只读回退实现，**不会**与占用方同时出现标签。
func composerInputSlotOwned(env *pluginapi.Env, slot pluginapi.Slot) bool {
	return env != nil && env.Has != nil && env.Has(slot)
}

// composerDispatchTo 报告"这个座位这一帧该不该把子槽交出去"：座位有内核
// `drawSlot`（官方座位）**且**槽上有占用方。
//
// 没有 drawSlot 的座位（旧三栏兼容桥、单测直调 `draw`）一律不交——那时候
// 画法只能是本插件自己的只读回退。**注意顺序**：先看有没有派发通道，再看
// 占用方；反过来的话，dsagentteam 装配着（`env.Has` 恒真）而座位又不派发，
// 结果就是"没人画模型标签"。
func composerDispatchTo(drawSlot func(pluginapi.Slot), env *pluginapi.Env, slot pluginapi.Slot) bool {
	return drawSlot != nil && composerInputSlotOwned(env, slot)
}

// dispatchComposerSlot 把子槽交回内核。
//
// drawSlot == nil 表示"这个座位没有内核"（单测直调 `draw` / `drawOfficialComposer`，
// 或旧三栏以外的兼容路径）：这时**什么都不派发**，所有位置都走本插件的只读回退。
func dispatchComposerSlot(drawSlot func(pluginapi.Slot), slot pluginapi.Slot) {
	if drawSlot == nil || slot == "" {
		return
	}
	drawSlot(slot)
}

// composerInputSlotModel 是模型子槽（与 `uikernel.OffInputModel` 同一个字面量）。
//
// 单独写一遍是为了让"本文件派发了哪些槽"能在一处读全（`composerInputSubSlots`
// 用它做自检），值必须与官方逐字一致。
const composerInputSlotModel pluginapi.Slot = uikernel.OffInputModel

// composerInputSubSlots 是**官方座位**每帧派发一次的官方子槽（`conversation.input.*`
// 里除了卡片下方那条 dock 之外的全部；dock 见下一张表）。
//
// 旧三栏座位（`main.composer`）**一个都不派发**（兼容桥，见文件头的"哪些座位
// 派发子槽"）。断言（`input_slots_test.go`）用的就是这两张表：官方档每槽一帧
// 恰好 dispatch 1 次、旧座位 0 次；且模型槽在画布上只产生一个模型标签
// （占用方或回退，二者其一）。
var composerInputSubSlots = []pluginapi.Slot{
	uikernel.OffConvComposerDock,
	uikernel.OffInputAttachments,
	uikernel.OffInputOverlay,
	uikernel.OffInputPermission,
	uikernel.OffInputPlan,
	uikernel.OffInputLeft,
	uikernel.OffInputRight,
	uikernel.OffInputModel,
	uikernel.OffInputActivity,
}

// composerDockSubSlots 是**只有官方座位**才有的 dock 子槽。
//
// 旧三栏座位（dsshell 的 `main.composer`）没有卡片下方那一条 dock（上下文读数
// 是官方 InputBar 的结构），所以它那份实现不派发 `conversation.input.dock`——
// 派发一个本座位不存在的结构位置，等于让占用方画到虚空里。
var composerDockSubSlots = []pluginapi.Slot{
	uikernel.OffInputDock,
}

// composerAllInputSubSlots 是上面两张表的并集（探针/自检用）。
func composerAllInputSubSlots() []pluginapi.Slot {
	out := make([]pluginapi.Slot, 0, len(composerInputSubSlots)+len(composerDockSubSlots))
	out = append(out, composerInputSubSlots...)
	return append(out, composerDockSubSlots...)
}
