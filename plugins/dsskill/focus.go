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
// 即：**键盘**焦点一律 2px、business-primary；指针点出来的焦点不画环。
// `--dsw-alias-state-business-primary` 在 rc.2 取 `--dsw-static-deepseek-400`（dark）/
// `-500`（light），就是主题里的 `Info`（plugins/dscore/dscore.go:217,319）。
//
// SkillRow 用的是 **outline** 语义（元素外侧、以边为中心），与 `box-shadow:inset` 不同：
// viva 的 `ui.Look.Ring` 正是这个语义（ui/style.go:369-377 的 paintRing），
// 所以 `.row` 直接 `.Focus(ui.Look{}.Ring(skillFocusRingWidth, t.Info))` 即可。
package dsskill

// skillFocusRingWidth 是 rc.2 `focus.css` 的 `--dsw-focus-ring-width:2px`。
const skillFocusRingWidth float32 = 2
