// focus.go —— 键盘焦点环的**局部**常量（不跨包 import，按仓里既有惯例每个包自带一份）。
//
// 官方 rc.2 的焦点环是**全局**规则，不是逐组件写死的：
//
//	focus.css：
//	  :root{--dsw-focus-ring-width:2px}
//	  :focus-visible{outline-color:var(--dsw-focus-ring-color,
//	                 var(--dsw-alias-state-business-primary));
//	                 outline-width:var(--dsw-focus-ring-width)}
//	  html[data-input-modality=pointer] body :focus-visible:not(:read-write){
//	                 --dsw-focus-ring-color:transparent;outline-color:#0000}
//
// `--dsw-alias-state-business-primary` 在 rc.2 取 `--dsw-static-deepseek-400`（dark）/
// `-500`（light），就是主题里的 `Info`（plugins/dscore/dscore.go:217,319）。
//
// 本包用 **outline** 语义（元素外侧、以边为中心），viva 的 `ui.Look.Ring` 正是它
// （ui/style.go:369-377 的 paintRing），所以关闭按钮直接 `.Focus(ui.Look{}.Ring(...))`。
package dsshortcuts

// sheetFocusRingWidth 是 rc.2 `focus.css` 的 `--dsw-focus-ring-width:2px`。
const sheetFocusRingWidth float32 = 2
