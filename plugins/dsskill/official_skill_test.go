// official_skill_test.go —— 官方 `dsh-client-ui-skill` 面的**逐条**断言。
//
// 三张表各一组断言，互不替代：
//
//	几何（officialSkillGeometry）  每个数值逐行核对：aligned 必须相等，
//	                               derived 必须有官方公式，deviation 必须有原因
//	声明（officialSkillDecls）     非数值 CSS 逐条核对：实现值与官方值必须相等
//	令牌（officialSkillTokens）    每个 `var(--…)` 都必须映射到本仓真实字段
//
// 再加两张表：
//
//	结构（officialSkillTree / officialSkillPreparingTree）  父子关系逐条核对
//	文案（officialSkillDictionaries）                       8 语言 × 8 键逐条核对
package dsskill

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// ── ① 几何：官方 CSS 的每一个数值 ──────────────────────────────────

// TestOfficialSkillGeometry 逐行核对几何表。
//
// 三条判据（与 `internal/uikernel/official_geometry.go` 同源）：
//
//	aligned   → Impl 必须 == Official
//	derived   → 必须写 Formula（官方是 calc/clamp/内容自适应），且 Impl 就是实现取值
//	deviation → 必须写 Reason（**一行都不许有**：本包今天没有 deviation）
func TestOfficialSkillGeometry(t *testing.T) {
	if len(officialSkillGeometry) == 0 {
		t.Fatal("几何表是空的")
	}
	seen := map[string]bool{}
	for _, row := range officialSkillGeometry {
		key := row.Node + "|" + row.Prop + "|" + row.Anchor
		if seen[key] {
			t.Errorf("几何行重复：%s", key)
		}
		seen[key] = true

		if row.Node == "" || row.Prop == "" {
			t.Errorf("几何行缺 Node/Prop：%+v", row)
		}
		if row.Anchor == "" {
			t.Errorf("%s.%s 没有官方锚点", row.Node, row.Prop)
		}
		if !strings.HasPrefix(row.Anchor, "SkillRow.module.css:") {
			t.Errorf("%s.%s 的锚点必须是 SkillRow.module.css 的行号，实际 %q", row.Node, row.Prop, row.Anchor)
		}
		switch row.Status {
		case SkillAligned:
			if row.Impl != row.Official {
				t.Errorf("%s.%s：aligned 但实现值 %v ≠ 官方值 %v", row.Node, row.Prop, row.Impl, row.Official)
			}
			if row.Formula != "" {
				t.Errorf("%s.%s：aligned 行不该有 Formula（%q）", row.Node, row.Prop, row.Formula)
			}
		case SkillDerived:
			if row.Formula == "" {
				t.Errorf("%s.%s：derived 行必须写官方公式原文", row.Node, row.Prop)
			}
		case SkillDeviation:
			if row.Reason == "" {
				t.Errorf("%s.%s：deviation 行必须写原因", row.Node, row.Prop)
			}
		default:
			t.Errorf("%s.%s：未知 status %q", row.Node, row.Prop, row.Status)
		}
	}
}

// TestOfficialSkillGeometryNoDeviation 钉住「本包没有 deviation」。
//
// 为什么要单独一条：deviation 是**允许存在**的逃逸口，一旦有人图省事把某行
// 标成 deviation + 编个理由，几何就再也对不齐了。本包今天一条都没有，
// 将来真有必须先改这条断言并写进报告。
func TestOfficialSkillGeometryNoDeviation(t *testing.T) {
	for _, row := range officialSkillGeometry {
		if row.Status == SkillDeviation {
			t.Errorf("本包不该有 deviation 行：%s.%s（%s）", row.Node, row.Prop, row.Reason)
		}
	}
}

// TestOfficialSkillGeometryCoversEveryCssValue 钉住「官方 CSS 的数值一个不漏」。
//
// 判据不是"数目对得上"，而是**逐个属性名点出来核对**——数目相等但漏了一个
// 补了一个假的是最容易发生的漂移。
func TestOfficialSkillGeometryCoversEveryCssValue(t *testing.T) {
	// 官方 `SkillRow.module.css` 里所有带单位的声明（逐条手数列出，行号见锚点）。
	want := []struct{ node, prop string }{
		{"SkillRow.row", "height"},
		{"SkillRow.row", "transition-duration"},
		{"SkillRow.leading", "width"},
		{"SkillRow.leading", "height"},
		{"SkillRow.leading", "margin-right"},
		{"SkillRow.leading svg", "width"},
		{"SkillRow.leading svg", "height"},
		{"SkillRow.iconIdle", "opacity"},
		{"SkillRow.iconIdle", "transition-duration"},
		{"SkillRow.chevronHover", "opacity"},
		{"SkillRow.chevronHover", "transition-duration"},
		{"SkillRow.row:hover .iconIdle", "opacity"},
		{"SkillRow.row:hover .chevronHover", "opacity"},
		{"SkillRow.title", "font-size"},
		{"SkillRow.title", "line-height"},
		{"SkillRow.separator", "width"},
		{"SkillRow.separator", "height"},
		{"SkillRow.separator", "border-radius"},
		{"SkillRow.separator", "margin-block"},
		{"SkillRow.separator", "margin-inline"},
		{"SkillRow.summary", "font-size"},
		{"SkillRow.summary", "line-height"},
		{"SkillRow.instructionsCard", "max-height"},
		{"SkillRow.instructionsCard", "margin-top"},
		{"SkillRow.instructionsCard", "margin-right"},
		{"SkillRow.instructionsCard", "margin-bottom"},
		{"SkillRow.instructionsCard", "margin-left"},
		{"SkillRow.instructionsCard", "border-width"},
		{"SkillRow.instructionsCard", "border-radius"},
		{"SkillRow.instructionsHeader", "padding-top"},
		{"SkillRow.instructionsHeader", "padding-right"},
		{"SkillRow.instructionsHeader", "padding-bottom"},
		{"SkillRow.instructionsHeader", "padding-left"},
		{"SkillRow.instructionsHeader", "border-bottom-width"},
		{"SkillRow.instructionsHeader", "font-size"},
		{"SkillRow.instructionsHeader", "font-weight"},
		{"SkillRow.instructionsHeader", "line-height"},
		{"SkillRow.instructionsHeader", "letter-spacing"},
		{"SkillRow.instructions", "padding-top"},
		{"SkillRow.instructions", "padding-right"},
		{"SkillRow.instructions", "padding-bottom"},
		{"SkillRow.instructions", "padding-left"},
		{"SkillRow.instructions", "font-size"},
		{"SkillRow.instructions", "line-height"},
		{"SkillRow.instructions::-webkit-scrollbar-thumb", "border-width"},
		{"SkillRow.instructions::-webkit-scrollbar-thumb", "border-radius"},
		{"SkillRow.instructions::-webkit-scrollbar-track", "margin-block"},
		{"SkillRow.inspectButton", "gap"},
		{"SkillRow.inspectButton", "margin-top"},
		{"SkillRow.inspectButton", "margin-right"},
		{"SkillRow.inspectButton", "margin-bottom"},
		{"SkillRow.inspectButton", "margin-left"},
		{"SkillRow.inspectButton", "padding-top"},
		{"SkillRow.inspectButton", "padding-right"},
		{"SkillRow.inspectButton", "padding-bottom"},
		{"SkillRow.inspectButton", "padding-left"},
		{"SkillRow.inspectButton", "border-width"},
		{"SkillRow.inspectButton", "border-radius"},
		{"SkillRow.inspectButton", "font-size"},
		{"SkillRow.inspectButton", "line-height"},
		{"SkillRow.inspectButton", "opacity"},
		{"SkillRow.inspectButton", "transition-duration"},
		{"SkillRow.inspectButton:focus-visible", "opacity"},
		{"SkillRow.card:hover .inspectButton", "opacity"},
		{"SkillRow.visuallyHidden", "width"},
		{"SkillRow.visuallyHidden", "height"},
	}
	have := map[string]bool{}
	for _, row := range officialSkillGeometry {
		have[row.Node+"|"+row.Prop] = true
	}
	for _, w := range want {
		if !have[w.node+"|"+w.prop] {
			t.Errorf("官方 CSS 有 %s.%s，几何表里没有", w.node, w.prop)
		}
	}
}

// TestOfficialSkillGeometryExactValues 钉住几处**最容易记错**的官方值。
//
// 这些数字不是"看着差不多"：`instructionsCard.max-height` 是 260 不是 240、
// `instructionsHeader.line-height` 是 16 不是 18、`inspectButton` 的
// `border-radius` 是 999（药丸）不是 8、`instructions` 的 `line-height`
// 是 19（不是 16）——写错一个，展开体就会差一行。
func TestOfficialSkillGeometryExactValues(t *testing.T) {
	cases := []struct {
		node, prop string
		want       float32
	}{
		{"SkillRow.row", "height", 24},
		{"SkillRow.leading", "width", 16},
		{"SkillRow.leading", "margin-right", 6},
		{"SkillRow.leading svg", "width", 14},
		{"SkillRow.title", "font-size", 13},
		{"SkillRow.title", "line-height", 24},
		{"SkillRow.separator", "width", 2},
		{"SkillRow.separator", "height", 2},
		{"SkillRow.separator", "border-radius", 1},
		{"SkillRow.separator", "margin-inline", 8},
		{"SkillRow.summary", "font-size", 13},
		{"SkillRow.summary", "line-height", 24},
		{"SkillRow.instructionsCard", "max-height", 260},
		{"SkillRow.instructionsCard", "margin-top", 4},
		{"SkillRow.instructionsCard", "margin-left", 4},
		{"SkillRow.instructionsCard", "border-width", 0.5},
		{"SkillRow.instructionsCard", "border-radius", 16},
		{"SkillRow.instructionsHeader", "padding-top", 8},
		{"SkillRow.instructionsHeader", "padding-left", 12},
		{"SkillRow.instructionsHeader", "font-size", 11},
		{"SkillRow.instructionsHeader", "font-weight", 500},
		{"SkillRow.instructionsHeader", "line-height", 16},
		{"SkillRow.instructionsHeader", "letter-spacing", 0.04},
		{"SkillRow.instructions", "padding-top", 10},
		{"SkillRow.instructions", "padding-right", 12},
		{"SkillRow.instructions", "padding-bottom", 12},
		{"SkillRow.instructions", "font-size", 11},
		{"SkillRow.instructions", "line-height", 19},
		{"SkillRow.inspectButton", "gap", 4},
		{"SkillRow.inspectButton", "margin-top", 4},
		{"SkillRow.inspectButton", "margin-bottom", 2},
		{"SkillRow.inspectButton", "margin-left", 4},
		{"SkillRow.inspectButton", "padding-top", 2},
		{"SkillRow.inspectButton", "padding-left", 8},
		{"SkillRow.inspectButton", "border-width", 0.5},
		{"SkillRow.inspectButton", "border-radius", 999},
		{"SkillRow.inspectButton", "font-size", 11},
		{"SkillRow.inspectButton", "line-height", 16},
		{"SkillRow.inspectButton", "opacity", 0},
		{"SkillRow.visuallyHidden", "width", 1},
		{"SkillRow.visuallyHidden", "height", 1},
	}
	for _, c := range cases {
		found := false
		for _, row := range officialSkillGeometry {
			if row.Node != c.node || row.Prop != c.prop {
				continue
			}
			found = true
			if row.Official != c.want {
				t.Errorf("%s.%s 官方值 = %v，want %v", c.node, c.prop, row.Official, c.want)
			}
		}
		if !found {
			t.Errorf("几何表里没有 %s.%s", c.node, c.prop)
		}
	}
}

// ── ② 非数值声明 ──────────────────────────────────────────────────

// TestOfficialSkillDecls 逐条核对非数值 CSS 声明。
//
// 这类声明最容易被"实现方便"改掉：`position:absolute` 改成 flex、
// `white-space:nowrap` 改成换行、`text-transform:uppercase` 忘掉——
// 它们都不报错，只是画出来不一样。
func TestOfficialSkillDecls(t *testing.T) {
	if len(officialSkillDecls) == 0 {
		t.Fatal("声明表是空的")
	}
	for _, row := range officialSkillDecls {
		if row.Node == "" || row.Prop == "" {
			t.Errorf("声明行缺 Node/Prop：%+v", row)
		}
		if row.Anchor == "" {
			t.Errorf("%s.%s 没有官方锚点", row.Node, row.Prop)
		}
		switch row.Status {
		case SkillAligned:
			if row.Official != row.Impl {
				t.Errorf("%s.%s：官方 %q ≠ 实现 %q", row.Node, row.Prop, row.Official, row.Impl)
			}
		case SkillDerived:
			if row.Note == "" {
				t.Errorf("%s.%s：derived 声明必须写等价说明", row.Node, row.Prop)
			}
		case SkillDeviation:
			if row.Note == "" {
				t.Errorf("%s.%s：deviation 声明必须写原因", row.Node, row.Prop)
			}
		default:
			t.Errorf("%s.%s：未知 status %q", row.Node, row.Prop, row.Status)
		}
	}
}

// TestOfficialSkillDeclsCoverTheLayoutSkeleton 钉住结构性的那几条声明。
func TestOfficialSkillDeclsCoverTheLayoutSkeleton(t *testing.T) {
	want := map[string]string{
		"SkillRow.card|flex-direction":               "column",
		"SkillRow.row|display":                       "flex",
		"SkillRow.row|align-items":                   "center",
		"SkillRow.leading|display":                   "inline-flex",
		"SkillRow.leading|justify-content":           "center",
		"SkillRow.chevronHover|position":             "absolute",
		"SkillRow.chevronHover|margin":               "auto",
		"SkillRow.summary|white-space":               "nowrap",
		"SkillRow.summary|text-overflow":             "ellipsis",
		"SkillRow.bodyWrap|flex-direction":           "column",
		"SkillRow.instructionsCard|overflow":         "hidden",
		"SkillRow.instructionsHeader|text-transform": "uppercase",
		"SkillRow.instructions|white-space":          "pre-wrap",
		"SkillRow.instructions|overflow-wrap":        "anywhere",
		"SkillRow.instructions|overflow":             "auto",
		"SkillRow.inspectButton|align-self":          "flex-start",
		"SkillRow.inspectButton|cursor":              "pointer",
		"SkillRow.visuallyHidden|position":           "absolute",
		"SkillRow.visuallyHidden|clip":               "rect(0 0 0 0)",
	}
	have := map[string]string{}
	for _, row := range officialSkillDecls {
		have[row.Node+"|"+row.Prop] = row.Official
	}
	for key, wantValue := range want {
		got, ok := have[key]
		if !ok {
			t.Errorf("声明表里没有 %s", key)
			continue
		}
		if got != wantValue {
			t.Errorf("%s 官方值 = %q，want %q", key, got, wantValue)
		}
	}
}

// ── ③ 令牌映射 ────────────────────────────────────────────────────

// TestOfficialSkillTokens 逐条核对 `var(--…)` 映射。
//
// 判据两条：
//
//	① 每个 `--dsw-` / `--dsh-` 变量都必须有映射（漏一个就会在实现里写死色号）
//	② 映射到的本仓字段必须**真实存在**（这里用白名单核对，字段改名会红）
func TestOfficialSkillTokens(t *testing.T) {
	// 本仓 `pluginapi.Theme` 的字段白名单（改名字段必须同步改这里）。
	known := map[string]bool{
		"(基准 0)": true, "Fonts.Secondary": true, "Fonts.Tiny": true,
		"(等宽字族)":        true,
		"LabelTertiary": true, "LabelSecondary": true, "LabelCaption": true,
		"LabelPrimary": true, "Error": true, "WarnLabel": true,
		"BorderL1": true, "BorderL2": true, "BorderL4": true,
		"BgBase": true, "CodeBlock": true, "CodeBanner": true,
		"hoverSolidToken(t)": true,
		"Radii.LG":           true, "Radii.SM": true,
	}
	seen := map[string]bool{}
	for _, row := range officialSkillTokens {
		if row.Var == "" || row.Token == "" {
			t.Errorf("令牌行缺 Var/Token：%+v", row)
		}
		if seen[row.Var] {
			t.Errorf("令牌 %s 重复映射", row.Var)
		}
		seen[row.Var] = true
		if row.Anchor == "" {
			t.Errorf("令牌 %s 没有官方锚点", row.Var)
		}
		if !known[row.Token] {
			t.Errorf("令牌 %s 映射到 %q，但本仓没有这个字段", row.Var, row.Token)
		}
	}
	// 官方 CSS 里出现的每一个 var(--…) 都必须有映射。
	for _, name := range []string{
		"--dsh-content-font-delta",
		"--dsh-content-font-size-secondary",
		"--dsh-code-font-size",
		"--ds-font-family-code",
		"--dsw-alias-label-tertiary",
		"--dsw-alias-label-secondary",
		"--dsw-alias-label-caption",
		"--dsw-alias-label-primary",
		"--dsw-alias-state-error-primary",
		"--dsw-alias-state-warn-label",
		"--dsw-alias-border-l1",
		"--dsw-alias-border-l2",
		"--dsw-alias-border-l4",
		"--dsw-alias-bg-base",
		"--dsw-alias-markdown-code-block",
		"--dsw-alias-markdown-code-block-banner",
		"--dsw-alias-interactive-bg-hover-solid",
		"--dsw-radius-lg",
		"--dsw-radius-sm",
	} {
		if !seen[name] {
			t.Errorf("官方 CSS 用了 %s，令牌表里没有映射", name)
		}
	}
}

// TestHoverSolidTokenFollowsTheme 钉住 `-solid` 变体的按主题取色。
//
// 官方 `--dsw-alias-interactive-bg-hover-solid`：暗色 bluish-800、亮色 bluish-75。
// 本仓没有独立字段，靠两套主题里各一个同值字段取——**取错就会在亮色下
// 比官方浅一档**（这正是本仓 `Hover` 与 `-solid` 的差别）。
func TestHoverSolidTokenFollowsTheme(t *testing.T) {
	dark := &pluginapi.Theme{Dark: true, BgLayer3: ui.RGBA(1, 2, 3, 1), NavHover: ui.RGBA(9, 9, 9, 1)}
	if got := hoverSolidToken(dark); got != dark.BgLayer3 {
		t.Fatalf("暗色应取 BgLayer3，实际 %v", got)
	}
	light := &pluginapi.Theme{Dark: false, BgLayer3: ui.RGBA(1, 2, 3, 1), NavHover: ui.RGBA(9, 9, 9, 1)}
	if got := hoverSolidToken(light); got != light.NavHover {
		t.Fatalf("亮色应取 NavHover，实际 %v", got)
	}
}

// ── ④ 组件树形状 ──────────────────────────────────────────────────

// TestOfficialSkillTreeShape 逐条核对**父子关系**（谁是谁的子节点）。
//
// 这条断言挡的是"结构画得差不多"：`bodyWrap` 是 `card` 的子节点（**不是**
// `row` 的子节点，展开体在整个行下面）、`title/separator/summary` 是
// `shimmer` 的三个兄弟（**不是**嵌套）、`leading` 的三种形态互斥。
func TestOfficialSkillTreeShape(t *testing.T) {
	names := map[string]SkillNode{}
	for _, node := range officialSkillTree {
		if _, dup := names[node.Name]; dup {
			t.Errorf("节点重复：%s", node.Name)
		}
		names[node.Name] = node
	}
	if len(officialSkillTree) == 0 {
		t.Fatal("结构树是空的")
	}
	// ① 每个非根节点的父节点必须存在。
	for _, node := range officialSkillTree {
		if node.Parent == "" {
			if node.Name != "SkillRow" {
				t.Errorf("只有根节点可以没有父节点，%s 的 Parent 是空", node.Name)
			}
			continue
		}
		parent, ok := names[node.Parent]
		if !ok {
			t.Errorf("%s 的父节点 %q 不在树里", node.Name, node.Parent)
			continue
		}
		if parent.Name == node.Name {
			t.Errorf("%s 是自己的父节点", node.Name)
		}
	}
	// ② 官方那几个**最容易画错**的父子关系，逐条点出来。
	cases := []struct{ child, parent string }{
		{"SkillRow.row", "SkillRow"},
		{"SkillRow.row.leading", "SkillRow.row"},
		{"SkillRow.row.status", "SkillRow.row"},
		{"SkillRow.row.shimmer", "SkillRow.row"},
		{"SkillRow.row.shimmer.title", "SkillRow.row.shimmer"},
		{"SkillRow.row.shimmer.separator", "SkillRow.row.shimmer"},
		{"SkillRow.row.shimmer.summary", "SkillRow.row.shimmer"},
		// 展开体挂在 card 下，**不是** row 下
		{"SkillRow.bodyWrap", "SkillRow"},
		{"SkillRow.bodyWrap.instructionsCard", "SkillRow.bodyWrap"},
		{"SkillRow.bodyWrap.instructionsCard.header", "SkillRow.bodyWrap.instructionsCard"},
		{"SkillRow.bodyWrap.instructionsCard.instructions", "SkillRow.bodyWrap.instructionsCard"},
		{"SkillRow.bodyWrap.inspectButton", "SkillRow.bodyWrap"},
	}
	for _, c := range cases {
		node, ok := names[c.child]
		if !ok {
			t.Errorf("结构树里没有 %s", c.child)
			continue
		}
		if node.Parent != c.parent {
			t.Errorf("%s 的父节点 = %q，want %q", c.child, node.Parent, c.parent)
		}
	}
	// ③ `bodyWrap` 不许是 `row` 的子节点（这是官方最反直觉的一条）。
	if node, ok := names["SkillRow.bodyWrap"]; ok && node.Parent == "SkillRow.row" {
		t.Error("bodyWrap 挂在 row 下面了——官方是整个 row 的**兄弟**级子节点")
	}
	// ④ 每个节点都得有官方锚点。
	for _, node := range officialSkillTree {
		if node.Anchor == "" {
			t.Errorf("%s 没有官方锚点", node.Name)
		}
	}
}

// TestOfficialSkillPreparingTreeShape 钉住 preparing 态**少掉**的那些节点。
//
// 官方 preparing 态只有 leading 图标 + 隐藏状态文案 + 闪烁标题；
// 没有 separator、没有 summary、没有展开体。少一条就是"preparing 时多画了东西"。
func TestOfficialSkillPreparingTreeShape(t *testing.T) {
	names := map[string]bool{}
	for _, node := range officialSkillPreparingTree {
		names[node.Name] = true
	}
	for _, forbidden := range []string{
		"SkillRow.row.shimmer.separator",
		"SkillRow.row.shimmer.summary",
		"SkillRow.bodyWrap",
	} {
		if names[forbidden] {
			t.Errorf("preparing 态不该有 %s", forbidden)
		}
	}
	for _, required := range []string{
		"SkillRow.row.leading",
		"SkillRow.row.preparing",
		"SkillRow.row.shimmer",
	} {
		if !names[required] {
			t.Errorf("preparing 态缺 %s", required)
		}
	}
}

// ── ⑤ 文案：8 语言 × 8 键 ──────────────────────────────────────────

// TestOfficialSkillDictionaries 逐条核对 8 条字典。
//
// 判据：
//
//	① 语言数 = 8（官方 `index.ts` 注册的那 8 个）
//	② 每条字典的**键集完全相同**（官方用 `Record<SkillKey, string>` 钉住）
//	③ zh 的每一条文案**逐字**等于官方 `locales.ts`
func TestOfficialSkillDictionaries(t *testing.T) {
	if len(officialSkillDictionaries) != 8 {
		t.Fatalf("字典数 = %d，want 8（zh/en/ja/es/fr/de/ru/pt）", len(officialSkillDictionaries))
	}
	for _, lang := range []string{"zh", "en", "ja", "es", "fr", "de", "ru", "pt"} {
		dict, ok := officialSkillDictionaries[lang]
		if !ok {
			t.Errorf("缺 %s 字典", lang)
			continue
		}
		if len(dict) != len(officialSkillKeys) {
			t.Errorf("%s 字典键数 = %d，want %d", lang, len(dict), len(officialSkillKeys))
		}
		for _, key := range officialSkillKeys {
			text, ok := dict[key]
			if !ok {
				t.Errorf("%s 缺键 %s", lang, key)
				continue
			}
			if text == "" {
				t.Errorf("%s 的 %s 是空串", lang, key)
			}
		}
	}
}

// TestOfficialSkillZhText 逐字核对中文文案（官方 `locales.ts:7-16`）。
func TestOfficialSkillZhText(t *testing.T) {
	want := map[SkillKey]string{
		KeyRowTitle:        "加载技能",
		KeyRowRunning:      "正在加载 skill",
		KeyRowPreparing:    "准备加载技能",
		KeyRowFailed:       "skill 加载失败",
		KeyRowStopped:      "skill 加载已中止",
		KeyRowInstructions: "说明",
		KeyRowInspect:      "查看",
		KeyMenuUserOnly:    "仅用户",
	}
	for key, text := range want {
		if got := zh[key]; got != text {
			t.Errorf("zh[%s] = %q，want %q", key, got, text)
		}
	}
}

// TestOfficialSkillEnText 逐字核对英文文案（官方 `locales.ts:22-31`）。
func TestOfficialSkillEnText(t *testing.T) {
	want := map[SkillKey]string{
		KeyRowTitle:        "Skill",
		KeyRowRunning:      "Loading skill",
		KeyRowPreparing:    "Preparing to load a skill",
		KeyRowFailed:       "Skill load failed",
		KeyRowStopped:      "Skill load stopped",
		KeyRowInstructions: "Instructions",
		KeyRowInspect:      "Inspect",
		KeyMenuUserOnly:    "user-only",
	}
	for key, text := range want {
		if got := en[key]; got != text {
			t.Errorf("en[%s] = %q，want %q", key, got, text)
		}
	}
}

// TestSkillTextFallback 钉住回退阶梯（官方 `ctx.locale.bind(NS)` 的语义）。
func TestSkillTextFallback(t *testing.T) {
	cases := []struct {
		lang string
		key  SkillKey
		want string
	}{
		{"zh", KeyRowTitle, "加载技能"},
		{"en", KeyRowTitle, "Skill"},
		{"zh-CN", KeyRowTitle, "加载技能"},               // 带地区：按 `-` 取主语言
		{"en-US", KeyRowTitle, "Skill"},              // 同上
		{"", KeyRowTitle, "加载技能"},                    // 空 = zh
		{"xx", KeyRowTitle, "加载技能"},                  // 未知语言 → 回退 zh
		{"en", "row.nonexistent", "row.nonexistent"}, // 连 zh 都没有 → 键名本身
	}
	for _, c := range cases {
		if got := SkillText(c.lang, c.key); got != c.want {
			t.Errorf("SkillText(%q, %q) = %q，want %q", c.lang, c.key, got, c.want)
		}
	}
}
