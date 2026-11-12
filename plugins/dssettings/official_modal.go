// official_modal.go —— 设置模态的**等价层**（rc.2 真变 D3 + D4）。
//
// # 官方 rc.2 的形状
//
//	// SettingsRoot.tsx:64
//	useModalLayer(panel, true, onClose)
//	// SettingsRoot.tsx:70-113
//	return createPortal((<div className={css.overlay} role="presentation"> … </div>),
//	  document.body)
//
// 也就是说 rc.2 **不再用 primitives 的 `Modal` 组件**：面板自己 `createPortal`
// 到 `document.body`，再由 `useModalLayer(panel, true, onClose)` 负责
// initial focus / Tab 循环 / Escape / root inert / 关闭后焦点恢复；初始焦点靠
// 当前 section 的 nav cell 上的 `data-modal-autofocus`（:87）。
//
// # 本仓的等价物
//
// Go + Viva 没有 React 的 portal 与 hook，所以把这两件事拆成两份可测的数据：
//
//	Portal      —— 渲染到哪一层（body 级 overlay，z-index 1000），而不是画在
//	               面板父节点里（那样会被后面的 chrome 行覆盖，官方注释 :70-75 写过）。
//	ModalLayer  —— 一个显式的模态状态机：打开时记下恢复目标、把焦点放到
//	               autofocus 元素、Tab/Shift+Tab 在 focusable 列表内循环、
//	               Escape 关闭、root 进入 inert；关闭时恢复焦点。
//
// **不要**换成 primitives 的 Modal 组件（viva 的 Dialog 那一族）：那正是 rc.2
// 去掉的东西。`official_modal_test.go` 用源码守卫钉住这一条。
//
// # aria-labelledby（D3）
//
// rc.1 台账写面板有 `aria-label`；rc.2 实测是 `aria-labelledby={titleId}`
// （titleId 指向 navTitle 节点，SettingsRoot.tsx:61/73/77-78）。Go 侧没有 ARIA
// 属性，等价物是 DialogSemantics：**有 AriaLabelledBy、没有 AriaLabel**。
package dssettings

import (
	"github.com/vivaui/viva/gfx"
	"sync"
)

// 模态层的渲染落点（createPortal 的等价物）。
const (
	// PortalTargetBody 是官方 `createPortal(…, document.body)` 的目标。
	//
	// 为什么是 body 不是 #root：官方注释（SettingsRoot.tsx:70-75）说明——挂在
	// root 里的覆盖层排在列 chrome 之前，声明 window-drag 的 chrome 行会把它
	// 的减高覆盖掉；body 层靠 base.css 的 `body > :not(#root)` 规则统一减高。
	PortalTargetBody = "document.body"
	// PortalPhase 是 portal 的挂载阶段（挂载后才允许第一次布局）。
	PortalPhase = "mounted"
)

// DialogSemantics 是面板的**无障碍语义**（ARIA 的 Go 等价物）。
//
// 字段与官方 JSX 逐个对应；测试断言的是「有 labelledby、没有 label」——
// 这正是 rc.2 的 D3 真变，写错成 aria-label 会让读屏软件读不到标题。
type DialogSemantics struct {
	// Role 是 `role="dialog"`。
	Role string
	// AriaModal 是 `aria-modal="true"`。
	AriaModal bool
	// AriaLabelledBy 是 `aria-labelledby` 指向的节点 id（navTitle 节点）。
	AriaLabelledBy string
	// AriaLabel 必须为空：rc.2 用 labelledby，不再用 label。
	AriaLabel string
	// TabIndex 是 `tabIndex={-1}`（面板自身可编程聚焦，不进 Tab 序列）。
	TabIndex int
	// PortalTarget 是渲染落点（createPortal 的第二个参数）。
	PortalTarget string
	// ZIndex 是覆盖层 z-index（官方 1000）。
	ZIndex float32
	// Inert 表示 root 当前是否 inert（模态打开期间为真）。
	Inert bool
}

// DialogRole / DialogTabIndex 是官方面板的两个固定属性值。
const (
	DialogRole     = "dialog"
	DialogTabIndex = -1
)

// NewDialogSemantics 造一份面板语义。
//
// titleNodeID 是 navTitle 节点的交互 id（`useId()` 的等价物）；空串是编程错误
// （没有标题的对话框对读屏软件是「未命名对话框」），这里保留空串是为了让调用方
// 的测试能看见它——**绘制层不会传空**。
func NewDialogSemantics(titleNodeID string, inert bool) DialogSemantics {
	return DialogSemantics{
		Role:           DialogRole,
		AriaModal:      true,
		AriaLabelledBy: titleNodeID,
		// 故意留空：rc.2 用 labelledby 取代 label（D3）。
		AriaLabel:    "",
		TabIndex:     DialogTabIndex,
		PortalTarget: PortalTargetBody,
		ZIndex:       OverlayZIndex,
		Inert:        inert,
	}
}

// ModalLayer 是模态层状态机（`useModalLayer(panel, true, onClose)` 的等价物）。
//
// 语义逐条（官方行为契约，结构树 §2.2）：
//
//	打开   → root inert；初始焦点 = autofocus 元素（当前 section 的 nav cell）；
//	        记住关闭后要恢复的焦点
//	Tab    → 下一个 focusable（末尾回到开头）
//	Shift+Tab → 上一个 focusable（开头回到末尾）
//	Escape → 关闭（回调 onClose），恢复焦点，root 解除 inert
//	其它键 → 不消费（交给面板自己的行导航）
//
// 为什么带锁：键盘从窗口线程进来，绘制线程也在读 focus/inert；不加锁就是
// 数据竞争（本仓 -race 门禁要求 0）。
type ModalLayer struct {
	mu sync.Mutex

	open       bool
	titleID    string
	autofocus  string
	focus      string
	focusables []string
	restore    string
	inert      bool
	onClose    func()

	// opened / closed 是累计计数（测试用它证明「确实开关过」，而不是看一个
	// 可能被重置的布尔）。
	opened int
	closed int
}

// NewModalLayer 造一个模态层。onClose 是 Escape / 遮罩点击 / 关闭按钮共用的
// 关闭回调（官方 `:64` 三者同一个 close）。
func NewModalLayer(onClose func()) *ModalLayer {
	return &ModalLayer{onClose: onClose}
}

// Open 打开模态层。
//
// titleID 是 navTitle 节点 id（进 aria-labelledby）；focusables 是面板内可聚焦
// 元素的**有序** id 列表；autofocus 必须是 focusables 里的一个（否则焦点落到
// 第一个，并让测试看得见）；restore 是关闭后要恢复的焦点（一般是打开前的目标）。
func (m *ModalLayer) Open(titleID string, focusables []string, autofocus, restore string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.open = true
	m.titleID = titleID
	m.focusables = append(m.focusables[:0], focusables...)
	m.autofocus = autofocus
	m.restore = restore
	m.inert = true
	m.opened++
	m.focus = autofocus
	if !containsString(m.focusables, m.focus) {
		if len(m.focusables) > 0 {
			m.focus = m.focusables[0]
		} else {
			m.focus = ""
		}
	}
}

// Close 关闭模态层，返回要恢复的焦点。
//
// 幂等：已经关着时返回 ("", false)，不重复触发回调。
func (m *ModalLayer) Close() (restore string, didClose bool) {
	m.mu.Lock()
	if !m.open {
		m.mu.Unlock()
		return "", false
	}
	m.open = false
	m.inert = false
	restore = m.restore
	m.closed++
	m.focus = ""
	cb := m.onClose
	m.mu.Unlock()
	if cb != nil {
		cb()
	}
	return restore, true
}

// Opened 报告模态层是否开着。
func (m *ModalLayer) Opened() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.open
}

// Inert 报告 root 当前是否 inert（模态打开期间为真）。
func (m *ModalLayer) Inert() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inert
}

// Focused 返回当前焦点 id（空 = 没有可聚焦元素）。
func (m *ModalLayer) Focused() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.focus
}

// Focusables 返回可聚焦 id 的快照。
func (m *ModalLayer) Focusables() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.focusables...)
}

// SetFocusables 更新可聚焦列表（section 切换后行变了）。
//
// 当前焦点不在新列表里时回退到 autofocus（再不行取第一个）——官方在
// section 切换时就是靠 data-modal-autofocus 把焦点带到新 section 的导航项。
func (m *ModalLayer) SetFocusables(focusables []string, autofocus string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.focusables = append(m.focusables[:0], focusables...)
	if autofocus != "" {
		m.autofocus = autofocus
	}
	if !containsString(m.focusables, m.focus) {
		if containsString(m.focusables, m.autofocus) {
			m.focus = m.autofocus
		} else if len(m.focusables) > 0 {
			m.focus = m.focusables[0]
		} else {
			m.focus = ""
		}
	}
}

// Counts 返回累计打开/关闭次数（测试用）。
func (m *ModalLayer) Counts() (opened, closed int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opened, m.closed
}

// Semantics 返回面板当前的对话框语义。
func (m *ModalLayer) Semantics() DialogSemantics {
	m.mu.Lock()
	defer m.mu.Unlock()
	return NewDialogSemantics(m.titleID, m.inert)
}

// HandleKey 实现模态层键盘：Escape 关闭、Tab / Shift+Tab 循环焦点。
//
// 只吃这三个键；其它键返回 false（面板自己的行导航、行激活才消费）。
// 官方语义：Escape 挂在 document 上、Tab 循环由 useModalLayer 负责。
func (m *ModalLayer) HandleKey(ev gfx.KeyEvent) bool {
	if ev.Release {
		return false
	}
	m.mu.Lock()
	if !m.open {
		m.mu.Unlock()
		return false
	}
	switch ev.Key {
	case gfx.KeyEscape:
		m.mu.Unlock()
		// Escape 走关闭路径（会触发回调并恢复焦点）。
		_, _ = m.Close()
		return true
	case gfx.KeyTab:
		n := len(m.focusables)
		if n == 0 {
			m.mu.Unlock()
			return true // 模态层吃掉 Tab，不让焦点跑到面板外
		}
		idx := indexOf(m.focusables, m.focus)
		if idx < 0 {
			idx = 0
			if ev.Modifiers&gfx.ModifierShift != 0 {
				idx = n
			}
		}
		delta := 1
		if ev.Modifiers&gfx.ModifierShift != 0 {
			delta = -1
		}
		m.focus = m.focusables[((idx+delta)%n+n)%n]
		m.mu.Unlock()
		return true
	default:
		m.mu.Unlock()
		return false
	}
}

// containsString / indexOf 是这两个小工具（不引 slices：本包只需要「找得到吗」）。
func containsString(list []string, want string) bool {
	return indexOf(list, want) >= 0
}

func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}
