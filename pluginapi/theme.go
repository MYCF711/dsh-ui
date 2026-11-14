package pluginapi

import "github.com/vivaui/viva/ui"

// Theme 是界面线的设计令牌：只有数据，没有画法。
//
// 值不是我们定的，是**量出来的**：来自官方界面包
// `@deepseek-ai/dsh-client-ui-theme` 里注入的 `--dsw-static-*` 原始色板与
// `--dsw-alias-*` 语义令牌（dump 在 tmp/official-theme.js，规格表在
// docs/official-visual-spec.md）。命名一一对应，方便回头核对：
//
//	BgBase        ← --dsw-alias-bg-base
//	SidebarFill   ← --dsw-specific-sidebar-fill
//	LabelPrimary  ← --dsw-alias-label-primary
//	BorderL1      ← --dsw-alias-border-l1
//
// 改这里等于改整套界面的长相，所以任何取值的改动都要在文档里写清出处。
type Theme struct {
	Name string
	Dark bool

	// ── 底色 ──
	BgBase      ui.Color // 主区背景
	SidebarFill ui.Color // 左栏背景
	BgLayer1    ui.Color // 抬高一层（卡片）
	BgLayer2    ui.Color // 抬高两层（气泡、输入框）
	BgLayer3    ui.Color // 抬高三层（选择器、模块台）

	// ── 交互态 ──
	NavActive ui.Color // 侧栏选中项
	NavHover  ui.Color // 侧栏悬停项
	Hover     ui.Color // 通用悬停叠色（原值是半透明白 / 黑）

	// ── 描边 ──
	BorderL1 ui.Color // 最轻：分隔线
	BorderL2 ui.Color // 轻：卡片描边
	BorderL3 ui.Color // 中
	BorderL4 ui.Color // 重

	// ── 文字 ──
	LabelPrimary   ui.Color // 正文
	LabelSecondary ui.Color // 次要
	LabelTertiary  ui.Color // 第三级
	LabelCaption   ui.Color // 说明 / 状态栏
	LabelDimmed    ui.Color // 几乎不可见（占位）

	// ── 品牌与链接 ──
	Link              ui.Color
	BrandPrimary      ui.Color
	ButtonPrimaryFill ui.Color
	ButtonPrimaryFg   ui.Color

	// ── 代码 ──
	CodeBlock    ui.Color // 代码块底
	CodeBanner   ui.Color // 代码块顶栏（语言名）
	InlineCode   ui.Color // 行内代码底
	CodeComment  ui.Color
	CodeString   ui.Color
	CodeNumber   ui.Color
	CodeKeyword  ui.Color
	CodeFunction ui.Color
	CodeVariable ui.Color

	// ── 气泡与输入 ──
	Bubble     ui.Color
	InputMajor ui.Color

	// MarkdownTag ← --dsw-alias-markdown-tag
	//
	// 名字来自 Markdown 的行内标记底色，但 dockkit 的**选中 tab 底**用的就是它
	// （`_tabActive_123xe_344{color:label-primary;background:var(--dsw-alias-markdown-tag)}`
	// ，见 dsh-web-frontend/dist/assets/index-CSAdOBx6.css 的 `_123xe_` 模块）。
	// 取值来自 dsh-client-ui-theme/src/styles/design-platform.css:272（亮）
	// 与 :424（暗）：分别是 --dsw-static-neutral-bluish-75 / -850。
	MarkdownTag ui.Color

	// ── 滚动条 ──
	Scrollbar      ui.Color
	ScrollbarHover ui.Color

	// ── 状态色 ──
	Success ui.Color
	Error   ui.Color
	Warn    ui.Color
	Info    ui.Color

	// ── 状态**容器**色（官方 `--dsw-alias-state-*` 里成对的那几档）──
	//
	// 官方的小标签/胶囊（连接指示 `ConnectionIndicator`、审批条）不是"纯色点"，
	// 而是「tertiary 底 + 同色系 label 文字 + 20% 该色描边」的三件套：
	//
	//	连接指示 warning   background: --dsw-alias-state-warn-tertiary
	//	                   color:      --dsw-alias-state-warn-label
	//	连接指示 success   background: --dsw-alias-state-success-tertiary
	//	                   color:      --dsw-alias-state-success-primary
	//
	// 上述两条来自官方 `index-CSAdOBx6.css` 的 `._indicator_16cyo_1` /
	// `._warning_16cyo_35` / `._success_16cyo_65`；取值在
	// `dsh-client-ui-theme/src/styles/design-platform.css` 里映射到 `--dsw-static-*`
	// （暗色用 900 档，亮色用 100 档）。只有 Success/Warn 两个"纯色"时没法画这套底，
	// 那时只能拿纯色降透明度凑——那就不是官方那个色了。
	SuccessTertiary ui.Color // ← --dsw-alias-state-success-tertiary
	SuccessPrimary  ui.Color // ← --dsw-alias-state-success-primary
	WarnTertiary    ui.Color // ← --dsw-alias-state-warn-tertiary
	WarnLabel       ui.Color // ← --dsw-alias-state-warn-label

	// ── 文本选区 ──
	Selection ui.Color

	Radii Radii
	Fonts Fonts
}

// Radii 是圆角，对应官方 `--dsw-radius-*`。
//
//	xs 4 / sm 8 / md 12 / lg 16 / xl 20 / panel 28
//
// 记住哪一层用哪个，别到处写 8：
//
//	会话行、工具调用行、代码块   → SM  (8)
//	选择器、面包屑、文件卡       → MD  (12)
//	输入框面板、大卡片           → LG  (16)
//	消息气泡                     → XL  (20)
type Radii struct {
	XS    float32
	SM    float32
	MD    float32
	LG    float32
	XL    float32
	Panel float32
}

// Fonts 是字号阶梯。
//
// 官方正文没有写死：`--dsh-content-font-size` 是用户可调的变量，
// 其余字号由它 calc 出来。这里取官方默认档次，并把推导关系写清楚：
//
//	Body      = content-font-size
//	Secondary = content-font-size - 1
//	Caption   = content-font-size - 2
//	Code      = content-font-size - 1（等宽字看上去更大，所以比正文小一号）
type Fonts struct {
	Body          float32
	BodyLine      float32
	Secondary     float32
	SecondaryLine float32
	Caption       float32
	CaptionLine   float32
	Code          float32
	CodeLine      float32
	Title         float32 // 面板标题
	Tiny          float32 // 徽标 / 时间戳
}

// Palette 返回这套主题的调色板快照，给需要按名字取色的小部件用。
func (t *Theme) Palette() map[string]ui.Color {
	if t == nil {
		return nil
	}
	return map[string]ui.Color{
		"bg":        t.BgBase,
		"sidebar":   t.SidebarFill,
		"layer1":    t.BgLayer1,
		"layer2":    t.BgLayer2,
		"layer3":    t.BgLayer3,
		"navActive": t.NavActive,
		"navHover":  t.NavHover,
		"border1":   t.BorderL1,
		"border2":   t.BorderL2,
		"primary":   t.LabelPrimary,
		"secondary": t.LabelSecondary,
		"tertiary":  t.LabelTertiary,
		"caption":   t.LabelCaption,
		"link":      t.Link,
		"bubble":    t.Bubble,
		"input":     t.InputMajor,
		"success":   t.Success,
		"error":     t.Error,
		"warn":      t.Warn,
	}
}
