//go:build darwin || windows || linux

// hero_base_menu_test.go —— hero 基准分支菜单的**真实像素判据**（Viva headless）。
//
// 每条判据都对着"值不是常量"来写：同一份数据面只改一个字段（是否已请求 / 清单到没到 /
// 读没读到 / 点到哪一行），画出来的东西必须跟着变——写死、读错字段、漏判代价都会红。
//
// 官方出处见 hero_base_menu.go 的文件头（`WorktreeToggle.tsx:41-95`、
// `WorktreeToggle.module.css:10-110`、`locales.ts:313-317`）。
package dsforge

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// requestedView 造一份「已请求工作树」的数据面：官方此时把 `.current` 换成 `.base`
// 菜单芯片（`WorktreeToggle.tsx:140-141`）。
func requestedView(from *string, branches []string, current string) *pluginapi.ForgeView {
	return &pluginapi.ForgeView{
		SessionID:     "s-1",
		Checkout:      pluginapi.ForgeCheckout{Kind: pluginapi.ForgeCheckoutReady, Branch: current},
		Branches:      branches,
		BranchesKnown: true,
		Worktree: pluginapi.ForgeWorktree{
			PanelKnown: true,
			Editable:   true,
			Requested:  true,
			From:       from,
		},
	}
}

// monoTextWidth 量一段等宽文字的字宽：字符单位数 × 单元宽。
//
// `Snapshot.TextWidths` 对等宽行不记录（headless 那边给 0），但 `gfx.Text.CellWidth`
// 与 `Baseline` 有值——等宽面每个 ASCII 字符占一格，所以宽度就是字符数 × 单元宽。
// 判据只在**纯 ASCII** 的行上成立（分支名就是），有宽字符时返回 false 让调用方跳过。
func monoTextWidth(snapshot app.Snapshot, rect gfx.Rect) (float32, bool) {
	for _, text := range snapshot.Texts {
		if text.Bounds.X != rect.X || text.Bounds.Y != rect.Y || !text.Style.Mono || text.CellWidth <= 0 {
			continue
		}
		cells := 0
		for _, r := range text.Value {
			if r < 0x20 || r > 0x7e {
				return 0, false
			}
			cells++
		}
		return float32(cells) * text.CellWidth, true
	}
	return 0, false
}

// hasHoverFill 报告这一帧有没有在 chip 那块区域铺官方悬停底
// （`--dsw-alias-interactive-bg-hover`）。Viva 铺的矩形会按像素取整，所以按 1px
// 容差比，不做逐位相等。
func hasHoverFill(snapshot app.Snapshot, color gfx.Color, chip gfx.Rect) bool {
	const tolerance float32 = 1
	near := func(a, b float32) bool { return a-b <= tolerance && b-a <= tolerance }
	for _, prim := range snapshot.Primitives {
		if prim.Color != color {
			continue
		}
		if near(prim.Bounds.X, chip.X) && near(prim.Bounds.Y, chip.Y) &&
			near(prim.Bounds.Width, chip.Width) && near(prim.Bounds.Height, chip.Height) {
			return true
		}
	}
	return false
}

// textStyle 取某段文字的绘制样式（字号/等宽/墨色都在这一层）。
func textStyle(snapshot app.Snapshot, value string) (gfx.TextStyle, bool) {
	for _, text := range snapshot.Texts {
		if text.Value == value {
			return text.Style, true
		}
	}
	return gfx.TextStyle{}, false
}

// sameInk 比两段墨色是否同色。
//
// 只比 RGB 三通道：headless 记录「canvas 直接画的 Label」时把不透明色的 alpha 记成 0，
// 而经 `ui.Text` 那条路径画的记 1（同一份主题色一张帧里两种写法），拿 alpha 比会把
// 同一个令牌判成不同色。判据要的是"用的是不是那个令牌"，三个通道足够证伪。
func sameInk(got, want gfx.Color) bool {
	return got.Red == want.Red && got.Green == want.Green && got.Blue == want.Blue
}

// textBounds 取某段文字的绘制矩形。
func textBounds(snapshot app.Snapshot, value string) (gfx.Rect, bool) {
	for _, text := range snapshot.Texts {
		if text.Value == value {
			return text.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// TestBaseChipReplacesTheCurrentChipOnlyOnceRequested 是**可见性判据**
// （`WorktreeToggle.tsx:140-150`）：没请求时官方画只读的 `.current`，请求了才画
// 可点开的 `.base`；两态各有自己的文案，不能互相顶替。
func TestBaseChipReplacesTheCurrentChipOnlyOnceRequested(t *testing.T) {
	idle := newForgeCardsFixture(t, creationView(pluginapi.ForgeWorktree{
		PanelKnown: true, Editable: true,
	}), "s-1", drawToggleOnly)
	if _, ok := idle.hit("forge-worktree-base"); ok {
		t.Fatal("没请求工作树时官方画的是只读的 `.current`（WorktreeToggle.tsx:140-150），不该有菜单芯片")
	}
	if !idle.hasText("当前分支：feat/base") {
		t.Fatal("没请求时 `.current` 必须写出当前分支（worktree.currentBranchTip）")
	}

	from := "main"
	requested := newForgeCardsFixture(t, requestedView(&from, []string{"main", "feat/base"}, "feat/base"),
		"s-1", drawToggleOnly)
	if _, ok := requested.hit("forge-worktree-base"); !ok {
		t.Fatal("已请求时芯片必须是可点开的 `.base`（WorktreeToggle.tsx:141）")
	}
	if !requested.hasText("基于 main") {
		t.Fatal("`.base` 芯片文案是官方 `worktree.base`（基于 {branch}）")
	}
	if requested.hasText("当前分支：feat/base") {
		t.Fatal("已请求时不该再画 `.current` 的文案：官方这一支换成 `.base` 了")
	}
}

// TestBaseChipSaysHeadWhenTheRequestHasNoBranch 钉住**两个值不是一回事**：
// 夹具文案用 `request.from` 本身（null → 「基于当前 HEAD」，官方 `:132`），
// 菜单的 selectedId 才回落到当前分支。
func TestBaseChipSaysHeadWhenTheRequestHasNoBranch(t *testing.T) {
	f := newForgeCardsFixture(t, requestedView(nil, []string{"main", "feat/base"}, "feat/base"),
		"s-1", drawToggleOnly)
	if !f.hasText(copyWorktreeBaseHead) {
		t.Fatalf("`from` 为 null 时官方文案是 worktree.baseHead（%q）", copyWorktreeBaseHead)
	}
	if f.hasText(copyWorktreeBaseHead + "feat/base") {
		t.Fatal("芯片文案不许自己拼一个分支名")
	}
}

// TestBaseMenuListsEveryBranchWithTheCurrentTag 是**内容判据**：打开后先一行分组标题
// 「选择基准分支」（`WorktreeToggle.tsx:66`），其后每个本地分支一行；只有**当前分支**
// 那一行多一段「当前分支」注记（`:67-72`）。
func TestBaseMenuListsEveryBranchWithTheCurrentTag(t *testing.T) {
	from := "main"
	view := requestedView(&from, []string{"main", "feat/base", "release"}, "feat/base")
	f := newForgeCardsFixture(t, view, "s-1", drawToggleOnly)
	closed, ok := labelInk(f.session.Snapshot(), copyWorktreeBaseMenu)
	if ok {
		t.Fatalf("菜单没打开却画了分组标题（%v）：菜单是浮层，关着什么也不画", closed)
	}
	chip, ok := f.hit("forge-worktree-base")
	if !ok {
		t.Fatal("已请求时必须有 `.base` 芯片")
	}
	f.click(chip)
	snapshot := f.session.Snapshot()

	title, ok := labelInk(snapshot, copyWorktreeBaseMenu)
	if !ok {
		t.Fatalf("打开的菜单缺官方分组标题 %q", copyWorktreeBaseMenu)
	}
	rows := map[string]gfx.Rect{}
	for _, name := range []string{"main", "feat/base", "release"} {
		row, ok := labelInk(snapshot, name)
		if !ok {
			t.Fatalf("打开的菜单缺分支行 %q", name)
		}
		if row.Y <= title.Y {
			t.Fatalf("分支行 %q 排在分组标题之前（行 %v，标题 %v）：官方标题在最前", name, row.Y, title.Y)
		}
		rows[name] = row
	}
	if rows["main"].Y >= rows["feat/base"].Y || rows["feat/base"].Y >= rows["release"].Y {
		t.Fatalf("分支行顺序必须按宿主给的清单自上而下：%v", rows)
	}

	// 「当前分支」注记只有一行，并且落在当前分支（feat/base）那一行上。
	tag, ok := labelInk(snapshot, copyWorktreeBranchTag)
	if !ok {
		t.Fatalf("当前分支那一行缺官方注记 %q（worktree.currentBranch）", copyWorktreeBranchTag)
	}
	if tag.Y != rows["feat/base"].Y {
		t.Fatalf("注记落在 Y=%v，当前分支行在 Y=%v", tag.Y, rows["feat/base"].Y)
	}
	// 位置：注记紧跟在名字的**字尾**之后、中间隔官方 `.option{gap:8px}`
	// （`WorktreeToggle.module.css:99-106`），不是被推到行尾。
	if tag.X <= rows["feat/base"].X {
		t.Fatal("注记必须排在分支名之后（官方 `.option{gap:8px}`）")
	}
	if width, ok := monoTextWidth(snapshot, rows["feat/base"]); ok {
		if got, want := tag.X-rows["feat/base"].X, width+8; got-want > 0.8 || want-got > 0.8 {
			t.Fatalf("名字到注记的距离 = %.2f，官方 `.option{gap:8px}` 是名字宽(%.2f) + 8", got, width)
		}
	} else {
		t.Fatal("量不到分支名的绘制宽度：注记间距这条判据失效")
	}
	// 打开一次就重读一次分支（官方 show() 第一句就是 branches()）。
	if f.calls.branches != 1 {
		t.Fatalf("展开菜单调了 %d 次 LoadBranches，官方每次展开都重读一遍", f.calls.branches)
	}
}

// TestBaseMenuRowsUseOfficialTypography 钉住行的**字体读数**
// （`WorktreeToggle.module.css:99-110`）：分支名 12px 等宽，注记 12px label-tertiary；
// 分组标题 11px label-tertiary（原语 `.label`，`index-CSAdOBx6.css`）。
func TestBaseMenuRowsUseOfficialTypography(t *testing.T) {
	from := "main"
	view := requestedView(&from, []string{"main", "feat/base"}, "feat/base")
	f := newForgeCardsFixture(t, view, "s-1", drawToggleOnly)
	chip, _ := f.hit("forge-worktree-base")
	f.click(chip)
	snapshot := f.session.Snapshot()

	branch, ok := textStyle(snapshot, "main")
	if !ok {
		t.Fatal("打开的菜单缺分支行 main")
	}
	if !branch.Mono {
		t.Fatal("分支名必须走等宽面（官方 `.optionName{font-family:var(--ds-font-family-code)}`）")
	}
	if branch.Size != worktreeOptionFont {
		t.Fatalf("分支名字号 = %v，官方 `.optionName` 是 %v", branch.Size, worktreeOptionFont)
	}
	tag, ok := textStyle(snapshot, copyWorktreeBranchTag)
	if !ok {
		t.Fatal("缺「当前分支」注记")
	}
	if tag.Mono {
		t.Fatal("注记不是等宽面（官方 `.optionNote` 只有字号与颜色）")
	}
	if tag.Size != worktreeOptionFont || !sameInk(tag.Color, f.theme.LabelTertiary) {
		t.Fatalf("注记样式 = %v/%v，官方是 %v 与 label-tertiary", tag.Size, tag.Color, worktreeOptionFont)
	}
	title, ok := textStyle(snapshot, copyWorktreeBaseMenu)
	if !ok {
		t.Fatal("缺分组标题")
	}
	if title.Size != 11 || !sameInk(title.Color, f.theme.LabelTertiary) {
		t.Fatalf("分组标题样式 = %v/%v，官方 `._label` 是 11px label-tertiary", title.Size, title.Color)
	}
}

// TestBaseMenuChoosesTheBranchItWasClickedOn 是**行为判据**：点一行就发出
// `request(那一行)`（官方 onSelect → `request(id)`），点**当前基准**那一行什么也不发
// （官方 `if (id !== base)`）。
func TestBaseMenuChoosesTheBranchItWasClickedOn(t *testing.T) {
	from := "main"
	picked := newForgeCardsFixture(t, requestedView(&from, []string{"main", "feat/base"}, "main"),
		"s-1", drawToggleOnly)
	chip, _ := picked.hit("forge-worktree-base")
	picked.click(chip)
	row, ok := labelInk(picked.session.Snapshot(), "feat/base")
	if !ok {
		t.Fatal("打开的菜单缺分支行 feat/base")
	}
	picked.click(gfx.Hit{Bounds: row})
	if len(picked.calls.requested) != 1 {
		t.Fatalf("点了分支行发出 %d 次 requestWorktree，应该正好 1 次", len(picked.calls.requested))
	}
	if got := picked.calls.requested[0]; got == nil || *got != "feat/base" {
		t.Fatalf("request(from) = %v，官方发的是点中的那个分支", got)
	}

	// 对照：点当前基准那一行（`id === base`）不许重发请求。
	same := newForgeCardsFixture(t, requestedView(&from, []string{"main", "feat/base"}, "main"),
		"s-1", drawToggleOnly)
	chip2, _ := same.hit("forge-worktree-base")
	same.click(chip2)
	current, ok := labelInk(same.session.Snapshot(), "main")
	if !ok {
		t.Fatal("打开的菜单缺分支行 main")
	}
	same.click(gfx.Hit{Bounds: current})
	if len(same.calls.requested) != 0 {
		t.Fatalf("点当前基准那一行发了 %d 次请求，官方 `if (id !== base)` 不重发", len(same.calls.requested))
	}
}

// TestBaseMenuWaitsForTheBranchList 钉住 `show()` 的时序（`WorktreeToggle.tsx:52-64`）：
// 清单没到 → 不发请求、不打开；清单到了 → 自己打开。
func TestBaseMenuWaitsForTheBranchList(t *testing.T) {
	from := "main"
	view := requestedView(&from, nil, "feat/base")
	view.BranchesKnown = false
	f := newForgeCardsFixture(t, view, "s-1", drawToggleOnly)
	chip, ok := f.hit("forge-worktree-base")
	if !ok {
		t.Fatal("清单没到也必须有芯片（官方 `disabled={loading}` 只是禁用，不是隐藏）")
	}
	f.click(chip)
	if _, open := labelInk(f.session.Snapshot(), copyWorktreeBaseMenu); open {
		t.Fatal("清单没到就打开菜单了：官方是读到结果才 setOpen(true)")
	}
	if f.calls.branches != 1 {
		t.Fatalf("点击后调了 %d 次 LoadBranches，应该有且只有 1 次", f.calls.branches)
	}

	// 清单到了：下一帧菜单自己打开。
	view.Branches = []string{"main", "feat/base"}
	view.BranchesKnown = true
	f.pump()
	f.pump()
	if _, open := labelInk(f.session.Snapshot(), copyWorktreeBaseMenu); !open {
		t.Fatal("分支清单到了之后菜单必须自己打开（官方 `.then(() => setOpen(true))`）")
	}
}

// TestBaseMenuStaysShutWhenTheReadFailed 钉住 `if (result === undefined) return`：
// 读分支失败时官方**整场不开菜单**，也不编一行假数据。
func TestBaseMenuStaysShutWhenTheReadFailed(t *testing.T) {
	from := "main"
	view := requestedView(&from, nil, "feat/base")
	view.BranchesKnown = false
	view.BranchesErr = "git 不可用"
	f := newForgeCardsFixture(t, view, "s-1", drawToggleOnly)
	chip, _ := f.hit("forge-worktree-base")
	f.click(chip)
	view.BranchesKnown = true
	f.pump()
	f.pump()
	if _, open := labelInk(f.session.Snapshot(), copyWorktreeBaseMenu); open {
		t.Fatal("读分支失败还打开了菜单：官方这一支是 return，不是打开空菜单")
	}
}

// TestBaseChipMatchesOfficialGeometry 钉住 `.base` 的读数
// （`WorktreeToggle.module.css:44-50` + `:86-88`）：左内边距 8、右内边距 4、
// chevron 12px 贴右缘；芯片高 28。
func TestBaseChipMatchesOfficialGeometry(t *testing.T) {
	from := "main"
	view := requestedView(&from, []string{"main", "feat/base"}, "feat/base")
	f := newForgeCardsFixture(t, view, "s-1", drawToggleOnly)
	chip, ok := f.hit("forge-worktree-base")
	if !ok {
		t.Fatal("缺 `.base` 芯片")
	}
	// 读数一律写**字面量**，不引用被测常量：常量改了测试必须跟着红。
	// `.toggle,.base,.current{height:28px;padding:0 8px}` + `.base{padding-right:4px}`
	// （WorktreeToggle.module.css:10-50）、`<IconChevronDownRegular size={12}>`
	// （WorktreeToggle.tsx:90）。
	const officialHeight, officialPadLeft, officialPadRight, officialChevron float32 = 28, 8, 4, 12
	if chip.Bounds.Height != officialHeight {
		t.Fatalf("芯片高 = %v，官方 `.toggle,.base,.current{height:28px}`", chip.Bounds.Height)
	}
	label, ok := textBounds(f.session.Snapshot(), "基于 main")
	if !ok {
		t.Fatal("芯片没画出文案")
	}
	if label.X != chip.Bounds.X+officialPadLeft {
		t.Fatalf("文案左沿 = %v，官方 `padding:0 8px` 应是 %v", label.X, chip.Bounds.X+officialPadLeft)
	}
	// chevron 是这一帧芯片里唯一的 label-tertiary 墨迹（`.base` 没有分支图标）。
	ink, count := triangleInk(f.session.Snapshot(), f.theme.LabelTertiary, chip.Bounds)
	if count == 0 {
		t.Fatal("芯片里没有 chevron 的墨迹：`.base` 必须画官方 chevron（WorktreeToggle.tsx:90）")
	}
	right := chip.Bounds.X + chip.Bounds.Width
	boxX := right - officialPadRight - officialChevron
	boxY := chip.Bounds.Y + (officialHeight-officialChevron)/2
	// 官方素材 `chevron-down-outline` 的墨迹在 viewBox 16 里是 x∈[4,12]、y∈[6,9.683]
	// （`dsassets.ChevronDownOutline` 的路径原文），描边 1 按两侧各 0.35 计入。
	scale := officialChevron / 16
	for _, side := range []struct {
		name   string
		got    float32
		want   float32
		tol    float32
		reason string
	}{
		{"chevron 左沿", ink.X, boxX + 4*scale, 0.6, "右内边距 4px + size=12 的盒子左沿"},
		{"chevron 宽", ink.Width, 8 * scale, 0.9, "viewBox 16 里墨迹宽 8、按 12px 渲染"},
		{"chevron 上沿", ink.Y, boxY + 6*scale, 0.6, "行内竖直居中（`.base` 是 align-items:center）"},
		{"chevron 高", ink.Height, 3.683 * scale, 0.9, "viewBox 16 里墨迹高 3.683、按 12px 渲染"},
	} {
		if diff := side.got - side.want; diff > side.tol || diff < -side.tol {
			t.Fatalf("%s = %.3f，期望 %.3f（%s）", side.name, side.got, side.want, side.reason)
		}
	}
}

// TestBaseChipIsInertWhileTheWorktreeIsBeingCreated 钉住 `disabled={loading || held}`
// （`WorktreeToggle.tsx:88`）：创建正在跑时点它不发任何动作，也不给悬停底
// （`.base:hover:not(:disabled)`，`WorktreeToggle.module.css:31-35`）。
func TestBaseChipIsInertWhileTheWorktreeIsBeingCreated(t *testing.T) {
	from := "main"
	view := requestedView(&from, []string{"main", "feat/base"}, "feat/base")
	view.Worktree.Creation = &pluginapi.ForgeWorktreeCreation{
		Phase: pluginapi.ForgeCreationPrepare, Branch: "dsh/feat-base", From: &from, Held: true,
	}
	f := newForgeCardsFixture(t, view, "s-1", drawToggleOnly)
	chip, ok := f.hit("forge-worktree-base")
	if !ok {
		t.Fatal("创建中芯片仍然要画（官方只是禁用）")
	}
	f.moveTo(chip.Bounds.X+chip.Bounds.Width/2, chip.Bounds.Y+chip.Bounds.Height/2)
	// 禁用态不许出现悬停底：官方 `.base:hover:not(:disabled)`。
	if hasHoverFill(f.session.Snapshot(), f.theme.Hover, chip.Bounds) {
		t.Fatal("禁用态的 `.base` 出现了悬停底色")
	}
	f.click(chip)
	if f.calls.branches != 0 {
		t.Fatalf("禁用态点芯片调了 %d 次 LoadBranches，官方 `disabled` 时点不动", f.calls.branches)
	}
	if _, open := labelInk(f.session.Snapshot(), copyWorktreeBaseMenu); open {
		t.Fatal("禁用态点芯片把菜单打开了")
	}
}

// TestBaseChipHoversLikeTheOfficialButton 是上面那条的**反向对照**：同一处悬停，
// 没被禁用时必须出现官方悬停底（`.base:hover:not(:disabled)`），否则上面那条可能
// 因为"根本不画悬停"而假绿。
func TestBaseChipHoversLikeTheOfficialButton(t *testing.T) {
	from := "main"
	f := newForgeCardsFixture(t, requestedView(&from, []string{"main", "feat/base"}, "feat/base"),
		"s-1", drawToggleOnly)
	chip, ok := f.hit("forge-worktree-base")
	if !ok {
		t.Fatal("缺 `.base` 芯片")
	}
	f.moveTo(chip.Bounds.X+chip.Bounds.Width/2, chip.Bounds.Y+chip.Bounds.Height/2)
	if !hasHoverFill(f.session.Snapshot(), f.theme.Hover, chip.Bounds) {
		t.Fatal("可用态的 `.base` 悬停时没有官方悬停底")
	}
}

// TestBaseMenuOpenStateIsAPerSessionFlag 证明展开态不是常量：再点一次就关
// （官方 `show()` 第一句 `if (open) { setOpen(false); return }`）。
func TestBaseMenuOpenStateIsAPerSessionFlag(t *testing.T) {
	from := "main"
	f := newForgeCardsFixture(t, requestedView(&from, []string{"main", "feat/base"}, "feat/base"),
		"s-1", drawToggleOnly)
	chip, found := f.hit("forge-worktree-base")
	if !found {
		t.Fatal("缺 `.base` 芯片")
	}
	f.click(chip)
	if _, open := labelInk(f.session.Snapshot(), copyWorktreeBaseMenu); !open {
		t.Fatal("点了芯片菜单没打开")
	}
	f.click(chip)
	if text := drawnText(f.session.Snapshot()); strings.Contains(text, copyWorktreeBaseMenu) {
		t.Fatalf("再点一次没有关掉菜单：\n%s", text)
	}
}

// chevronInk 收芯片里那个 chevron 的墨迹点（`.base` 里唯一的 label-tertiary 图元）。
func chevronInk(snapshot app.Snapshot, color gfx.Color, chip gfx.Rect) [][3]gfx.Point {
	var out [][3]gfx.Point
	for _, prim := range snapshot.Primitives {
		if prim.Kind != app.TrianglePrimitive || prim.Color != color {
			continue
		}
		inside := true
		for _, p := range prim.Points {
			if p.X < chip.X || p.X > chip.X+chip.Width || p.Y < chip.Y || p.Y > chip.Y+chip.Height {
				inside = false
			}
		}
		if inside {
			out = append(out, prim.Points)
		}
	}
	return out
}

// TestBaseChevronFlipsWhileOpen 钉住 `.chevronOpen{transform:rotate(180deg)}`
// （`WorktreeToggle.module.css:86-88`）：同一个 chevron 展开时被转了半圈，
// 所以两帧的墨迹点必须不同（没转就是逐点相同）。
func TestBaseChevronFlipsWhileOpen(t *testing.T) {
	from := "main"
	f := newForgeCardsFixture(t, requestedView(&from, []string{"main", "feat/base"}, "feat/base"),
		"s-1", drawToggleOnly)
	chip, ok := f.hit("forge-worktree-base")
	if !ok {
		t.Fatal("缺 `.base` 芯片")
	}
	closed := chevronInk(f.session.Snapshot(), f.theme.LabelTertiary, chip.Bounds)
	if len(closed) == 0 {
		t.Fatal("关着的时候芯片里没有 chevron 墨迹")
	}
	f.click(chip)
	open := chevronInk(f.session.Snapshot(), f.theme.LabelTertiary, chip.Bounds)
	if len(open) != len(closed) {
		t.Fatalf("展开前后 chevron 三角数 %d → %d：该是同一枚转了半圈", len(closed), len(open))
	}
	same := true
	for i := range closed {
		if closed[i] != open[i] {
			same = false
		}
	}
	if same {
		t.Fatal("展开后 chevron 与关着时逐点相同：官方 `.chevronOpen` 要 rotate(180deg)")
	}
}
