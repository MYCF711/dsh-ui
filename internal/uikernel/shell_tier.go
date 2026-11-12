// shell_tier.go —— 壳的**结构档位**（official 骨架 / legacy 三栏）的共享持有者。
//
// # 为什么需要它
//
// 同一个部件在两个档位里的**挂载点不同**：
//
//	官方骨架   sidebar.settings / conversation.session.header.actions / …
//	旧三栏     sidebar.footer / main.header / …
//
// 壳在 `Load` 之后才能解析档位（`auto` 要等 `shell.root` 有没有占用方），
// 而插件在 `Register` 时就已登记 painter —— 登记早于档位确定。
//
// 所以这里提供一个**可写的小对象**：壳在 `Load` 之前 `Provide` 它，
// `Load` 之后写一次 `Official`，插件在登记时 `Inject` 到这个指针、
// 在**每一帧画之前**读它，决定自己该走官方挂载点还是兼容桥。
//
// # 三条纪律
//
//  1. 指针共享、写入一次（壳在首帧之前写完），读侧不加锁也不会看到半个值。
//     `Set` 仍然只允许一次：档位在一帧里变来变去只会画出两套结构叠着。
//  2. 没有这个对象时插件**按 official 处理**（官方槽是终态，旧槽是过渡）。
//  3. 它只回答"壳现在是哪一档"，不回答"哪个槽有谁"——后者是内核的事。
package uikernel

import "sync"

// KeyShellTier 是壳提供 ShellTier 的能力键。
//
// 插件用 `ctx.Inject(uikernel.KeyShellTier).(*uikernel.ShellTier)` 取；
// 取不到（测试/旧壳）时按 official 处理。
const KeyShellTier = "ui.shell.tier"

// ShellTier 是壳档位的共享持有者（零值 = official，见包注释第 2 条）。
type ShellTier struct {
	mu sync.RWMutex
	// set 记录壳是否已经写过一次（只允许写一次）。
	set      bool
	official bool
	// bridgeLegacy 是**每帧可写**的事实：官方骨架这一帧是否走了旧部件兼容桥。
	//
	// 为什么光有 official 不够（2026-11-12 真机踩到）：官方骨架在旧部件还在
	// 占槽时（`BridgeLegacy`）会**整段换成旧三栏排版**，于是
	// `sidebar.settings` / `conversation.session.header.actions` 这些官方子槽
	// **根本不会被 dispatch**，而 `sidebar.footer` / `main.header` 会被。
	// 插件只看 official 就会把画挂在永远不 dispatch 的槽上 → 界面空白但不报错。
	//
	// 所以壳每帧把 `BridgeLegacy` 写进来，插件判 `OfficialSlotsLive()`。
	bridgeLegacy bool
}

// NewShellTier 造一个"尚未确定"的档位持有者（按 official 读，直到 Set 被调用）。
func NewShellTier() *ShellTier { return &ShellTier{official: true} }

// Set 由壳在 `Load` 之后调用一次，写死本进程的档位。
//
// 重复调用被忽略：档位不是一个"随时可改"的开关，改它等于要求所有插件
// 重挂一遍 painter，而 painter 在登记时就已经是静态的。
func (t *ShellTier) Set(official bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.set {
		return
	}
	t.set = true
	t.official = official
}

// Official 报告本进程是否走官方骨架。nil 接收者按 official。
func (t *ShellTier) Official() bool {
	if t == nil {
		return true
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.official
}

// SetLegacyBridge 由壳每帧写一次：官方骨架这一帧是否走了旧部件兼容桥。
//
// 与 Set 不同，它是**可变**的（窗口变窄/插件热装都可能让兼容桥开关翻转），
// 所以不加"只写一次"的限制。
func (t *ShellTier) SetLegacyBridge(bridge bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.bridgeLegacy = bridge
	t.mu.Unlock()
}

// LegacyBridge 报告官方骨架这一帧是否走了旧部件兼容桥。
func (t *ShellTier) LegacyBridge() bool {
	if t == nil {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.bridgeLegacy
}

// OfficialSlotsLive 报告**官方子槽这一帧会不会被 dispatch**。
//
//	legacy 档位                       → false（dsshell 只 dispatch 旧槽）
//	official 档位 + 旧部件兼容桥      → false（骨架换成旧三栏排版，只 dispatch 旧槽）
//	official 档位 + 无旧部件          → true（官方槽全量 dispatch）
//
// 插件用它二选一挂载点：true 走官方槽，false 走过渡桥。两个都注册，
// 但**同一帧只有一个活**，所以不会双画。
func (t *ShellTier) OfficialSlotsLive() bool { return t.Official() && !t.LegacyBridge() }

// Settled 报告壳是否已经写过档位（诊断/测试用）。
func (t *ShellTier) Settled() bool {
	if t == nil {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.set
}
