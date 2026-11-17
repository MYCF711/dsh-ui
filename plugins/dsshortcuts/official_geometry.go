// official_geometry.go —— 快捷键面板的几何与样式数值，**逐项来自官方 CSS**，不自造。
//
// # 官方依据（有）
//
//	包名  @deepseek-ai/dsh-client-ui-shortcuts@0.2.2-rc.2
//	      （"Keyboard shortcut reference, recording, and local preference editing"）
//	样式  node_modules/@deepseek-ai/dsh-client-ui-shortcuts/src/client/Reference.module.css
//	      —— 下面每一个常量后面都写着它在该文件的行号
//	结构  Reference.tsx:155-212（Modal → header/title/close → searchRow → list → footer）
//
// 键位**注册表**是另一个官方包 @deepseek-ai/dsh-client-shortcuts（无 CSS，只有注册、
// 物理键路由与持久化）。面板消费的是"有哪些键"，两条都不自造。
//
// # 为什么 .dialog 的 box-shadow 不逐字抄
//
// 官方 `.dialog{box-shadow:0 0 1px #0003,0 0 4px #00000005,0 12px 32px #00000014}`
// （Reference.module.css:63）是**三层自定义阴影**；viva v0.6.4 的 `ui.Box` 只接受
// `gfx.ShadowXS..ShadowLG` 五档预设（`ui/box.go:299-304`），表达不出自定义层。
// 这里取语义最近的一档 `gfx.ShadowLG`（`ui/box.go:110` 的注释就是 "for a surface
// that covers part of the page: sheets and submenus"），它是**预设名**不是新数值。
package dsshortcuts

// 面板外框。官方是 Modal 里的 `.dialog`。
const (
	// sheetWidth 官方 .dialog{width:min(480px,100%)}：Reference.module.css:56。
	sheetWidth float32 = 480
	// sheetHeight 官方 .dialog{height:600px}：Reference.module.css:57。
	sheetHeight float32 = 600
	// sheetMaxInset 官方 .dialog{max-height:calc(100dvh - 108px)}：:58。
	// 108 = 上下各 54 的 Modal 边距。
	sheetMaxInset float32 = 108
	// sheetOffsetY 官方 .dialog{transform:translateY(30px)}：:61（Modal 居中后再下移 30）。
	sheetOffsetY float32 = 30
	// sheetRadius 官方 .dialog{border-radius:24px}：:62。
	// **不是** --dsw-radius-panel(28)，官方这里写死 24。
	sheetRadius float32 = 24
)

// 头部：标题 + 关闭按钮。
const (
	// sheetHeaderH 官方 .header{flex:0 0 46px}：Reference.module.css:69。
	sheetHeaderH float32 = 46
	// sheetHeaderPadTop/.header{padding:22px 54px 0 24px}：:70。
	// 右侧 54 = 关闭按钮（28 宽 + 右 14）+ 12 的呼吸位，别再自己加 padding。
	sheetHeaderPadTop   float32 = 22
	sheetHeaderPadRight float32 = 54
	sheetHeaderPadLeft  float32 = 24
	// sheetHeaderPadBottom 官方 padding 的下边是 0：:70。
	sheetHeaderPadBottom float32 = 0

	// sheetTitleSize/.title{font-size:16px}：:76。
	sheetTitleSize float32 = 16
	// sheetTitleLine/.title{line-height:24px}：:77。
	sheetTitleLine float32 = 24

	// sheetCloseTop 官方 .close{position:absolute;top:20px}：:82。
	sheetCloseTop float32 = 20
	// sheetCloseRight 官方 .close{right:14px}：:83。
	sheetCloseRight float32 = 14
	// sheetCloseSize 官方 .close{width:28px;height:28px}：:87-88。
	sheetCloseSize float32 = 28
	// sheetCloseRadius 官方 .close{border-radius:28px}（整圆）：:91。
	sheetCloseRadius float32 = 28
	// sheetCloseIconSize 官方 Reference.tsx:165 `<IconCloseOutlineRegular size={14} />`。
	// 注意官方 .close 还有 padding:6（:89），14 + 2*6 = 26 ≤ 28，图标是内缩的。
	sheetCloseIconSize float32 = 14
)

// 列表与行。官方 `.list` 是滚动容器（flex:1 + overflow-y:auto）。
const (
	// sheetListPadX 官方 .list{padding:0 16px 18px}：Reference.module.css:139。
	sheetListPadX float32 = 16
	// sheetListPadBottom 同上：:139。
	sheetListPadBottom float32 = 18

	// sheetRowMinH 官方 .row{min-height:42px}：:183。
	sheetRowMinH float32 = 42
	// sheetRowPad 官方 .row{padding:8px}：:184。
	sheetRowPad float32 = 8
	// sheetRowRadius 官方 .row{border-radius:var(--dsw-radius-md)}：:181；
	// --dsw-radius-md = 12px（dsh-client-ui-theme/src/styles/base.css:20）。
	sheetRowRadius float32 = 12
	// sheetRowFont 官方 .row{font-size:13px}：:186。
	sheetRowFont float32 = 13
	// sheetRowLine 官方 .row{line-height:18px}：:187。
	sheetRowLine float32 = 18
	// sheetRowGap 官方 .setting,.row,.binding{gap:8px}：:5。
	sheetRowGap float32 = 8

	// sheetKeyH 官方 .keyBadge{height:26px}：:210。
	sheetKeyH float32 = 26
	// sheetKeyPadX 官方 .keyBadge{padding:0 12px}：:210。
	sheetKeyPadX float32 = 12
	// sheetKeyRadius 官方 .keyBadge{border-radius:10px}：:210。
	sheetKeyRadius float32 = 10

	// sheetHintSize 官方 .hint{font-size:12px}：:192。
	sheetHintSize float32 = 12
	// sheetHintLine 官方 .hint{line-height:18px}：:192。
	sheetHintLine float32 = 18
	// sheetHintMarginY 官方 p.hint{margin:6px 0}：:193。
	sheetHintMarginY float32 = 6
)

// 面板文案。标题取自官方 Reference.tsx:163 的 `t('title')`（zh 词典值），
// 提示语是本仓的补充（官方面板没有这一行，见交付说明）。
const (
	shortcutsTitle      = "键盘快捷键"
	shortcutsHint       = "按 F1 或 Esc 关闭"
	shortcutsCloseLabel = "关闭键位表"
)

// panelWidth 官方 .dialog{width:min(480px,100%)}（Reference.module.css:56）。
//
// min() 的第二项是 100%：窄窗口里**直接等于窗口宽**，官方没有下限，所以这里也不设
// 自造的 min-width（旧的 320 下限没有官方出处，已删）。
func panelWidth(window float32) float32 {
	if window <= 0 {
		return sheetWidth
	}
	if window < sheetWidth {
		return window
	}
	return sheetWidth
}

// panelHeight 官方 .dialog{height:600px;max-height:calc(100dvh - 108px)}
// （Reference.module.css:57-58）：固定 600，窗口矮时按 100dvh-108 收。
func panelHeight(window float32) float32 {
	h := sheetHeight
	if window > 0 && h > window-sheetMaxInset {
		h = window - sheetMaxInset
	}
	if h < 0 {
		h = 0
	}
	return h
}

// panelTop 官方 Modal 纵向居中 + .dialog{transform:translateY(30px)}
// （Reference.module.css:61）。官方靠 max-height 预留的 54px 边距容下这 30px，
// 这里再夹一档 ≥0，防止极矮窗口把整块推出上边。
func panelTop(windowH, h float32) float32 {
	top := (windowH-h)/2 + sheetOffsetY
	if top < 0 {
		top = 0
	}
	return top
}
