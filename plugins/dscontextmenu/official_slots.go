package dscontextmenu

// official_slots.go —— 把官方 `dsh-client-ui-context-menu` 的三个槽位登记进
// uikernel 的 rc.2 官方槽表（`OFF:ui-context-menu/src/client/index.ts`）。
//
// 为什么需要：uikernel 的槽表是「官方结构树有没有这个座位」的可断言数据；
// 不登记的话，#10 即使有服务实现，结构账本上仍然是黑的。
//
// 三个槽的官方声明（逐字）：
//
//	contextMenu.button  factory/root  index.ts:36-37
//	contextMenu.icon    keyed/root    index.ts:40（inline: true）
//	contextMenu.content keyed/root    index.ts:42
//
// 三者的父级在官方是 root（由 context-menu 插件自己注册，不属于某个业务包）。
// 本仓的 `RegisterOfficialSlot` 要求父槽已登记，所以统一挂 `OffRoot`。
//
// 注意：`shell.overlay` 是官方既有槽，uikernel 内置表里已有，**不重复登记**。

import (
	"sync"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

var (
	officialSlotsOnce sync.Once
	officialSlotsErr  error
)

// 官方槽名。声明在这里而不是散在消费方，避免两处写错字。
const (
	// SlotContextMenuButton 是行尾省略号按钮的工厂槽（factory/root）。
	SlotContextMenuButton = "contextMenu.button"
	// SlotContextMenuIcon 是业务图标键槽（keyed/root/inline）。
	SlotContextMenuIcon = "contextMenu.icon"
	// SlotContextMenuContent 是业务自定义菜单行槽（keyed/root）。
	SlotContextMenuContent = "contextMenu.content"
)

// RegisterOfficialSlots 登记三个槽（幂等；同名已存在则跳过）。
func RegisterOfficialSlots() error {
	officialSlotsOnce.Do(func() {
		for _, spec := range []uikernel.SlotSpec{
			{
				Name:   SlotContextMenuButton,
				Kind:   uikernel.KindFactory,
				Scope:  uikernel.ScopeRoot,
				Parent: uikernel.OffRoot,
				Host:   "ContextMenu.button",
				Anchor: "ui-context-menu/src/client/index.ts:37",
				Note: "factory/root；官方注释：目标 kind 没有已注册贡献者时**整枚按钮不渲染**" +
					"（index.ts:37-40）。本仓消费入口是 ContextMenus.Button。",
			},
			{
				Name:   SlotContextMenuIcon,
				Kind:   uikernel.KindKeyed,
				Scope:  uikernel.ScopeRoot,
				Parent: uikernel.OffRoot,
				Host:   "ContextMenu.icon",
				Anchor: "ui-context-menu/src/client/index.ts:40",
				Note: "keyed/root/inline；owner 是 { icon: ContextMenuIcon }（key + 可选 src）。" +
					"本仓没有官方 primitives 图标资产，菜单项按文案渲染，不画自造图标。",
			},
			{
				Name:   SlotContextMenuContent,
				Kind:   uikernel.KindKeyed,
				Scope:  uikernel.ScopeRoot,
				Parent: uikernel.OffRoot,
				Host:   "ContextMenu.content",
				Anchor: "ui-context-menu/src/client/index.ts:42",
				Note: "keyed/root；官方的可选 React 行扩展位（owner {target, dismiss}）。" +
					"本仓不是 React，业务动作全部走 ContextMenuContribution 数据面。",
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
