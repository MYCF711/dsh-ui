//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：技能行（SkillRow）。
//
// 前两轮验过的是官方几何（`skillrow_visual_test.go` 用真帧量 `.row` 24 高、
// 说明卡 260 封顶、分隔点 2×2）和正文裁剪。这一轮专挑前两轮**没看过**的三件事：
//
//	① `.row:hover` 的**换图**：官方 `.iconIdle{opacity:1}` / `.chevronHover{opacity:0}`，
//	   悬停时前者 0、后者 1（`SkillRow.module.css` 的 `@media (hover:hover)` 两条）。
//	   前两轮只验过 inspect 按钮的显形，没验过这组图标互换。
//	② 同一份 hover 规则里的**文字换色**：`.row{color:label-tertiary}` +
//	   `.row:hover{color:label-secondary}`——官方 `.title`/`.summary` 都写 `color:inherit`，
//	   所以整行字都跟着变；如果实现给每个 Text 单独写死颜色，这条就永远不生效。
//	③ `.inspectButton:focus-visible{opacity:1}`：**键盘焦点**下按钮必须显形。
//
// 官方依据（rc.2 asar 原件，`@deepseek-ai/dsh-client-ui-skill-*` 的 SkillRow.module.css）：
//
//	`.row{height:24px;color:label-tertiary;transition:color .1s}`
//	`@media (hover:hover){.row:hover{color:label-secondary}}`
//	`.leading{width:16px;height:16px;margin-right:6px}` / `.leading svg{width:14px;height:14px}`
//	`.iconIdle{opacity:1}` / `.chevronHover{opacity:0}`
//	`@media (hover:hover){.row:hover .iconIdle{opacity:0};.row:hover .chevronHover{opacity:1}}`
//	`.title{font-size:13px;line-height:24px}` / `.summary{...text-overflow:ellipsis}`
//	`.inspectButton{opacity:0;...}` / `.inspectButton:focus-visible{opacity:1}`
//	`@media (hover:hover){.card:hover .inspectButton{opacity:1}}`
//	（rc.1 抽取件 `dsh-ui/tmp/asar-css/129__SkillRow.module.css.mjs.css:1` 与 rc.2 逐字节同款，
//	 行号引用沿用该文件；rc.2 的差异只是把 `:focus-visible` 单独写成一条规则。）
//
// # 依据来源（rc.2 复核）
//
// 上面每一条都已用 **rc.2 asar 原件**（`Desktop-0.2.2-rc.2/resources/app.asar` 里
// `@deepseek-ai/dsh-client-ui-skill-*` 的 SkillRow.module.css、类名前缀 `iJmaOG_`）
// 复核：`.row{height:calc(24px + delta);color:label-tertiary}`、
// `@media (hover:hover){.row:hover{color:label-secondary}}`、
// `.iconIdle{opacity:1}` / `.chevronHover{opacity:0}`、
// `@media (hover:hover){.row:hover .iconIdle{opacity:0};.row:hover .chevronHover{opacity:1}}`、
// `.inspectButton{opacity:0}` 与独立的 `.inspectButton:focus-visible{opacity:1}` —— 全部逐字一致。
//
// 落盘目录：`$VR3_CAPTURE_DIR`（未设置时用 t.TempDir()）。
package dsskill

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dstool"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

// vr3SkillDir 是本次审计的落盘目录。
func vr3SkillDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("VR3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// vr3SkillShot 用 session.Screenshot 落一张 PNG，并核它非空。
func vr3SkillShot(t *testing.T, session *headless.Session, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := session.Screenshot(path); err != nil {
		t.Fatalf("%s：截图失败：%v", name, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("%s：截图为空：path=%s info=%v err=%v", name, path, info, err)
	}
	t.Logf("VR3_CAPTURE path=%s bytes=%d", path, info.Size())
	return path
}

// vr3SkillHitID 按 id 找命中区（`skill-row-<seq>` / `skill-inspect-<seq>` / `skill-card-<seq>`）。
func vr3SkillHitID(h *skillHarness, session *headless.Session, name string) (gfx.Hit, bool) {
	id := h.tree.ID(h.feat, name)
	for _, hit := range session.Snapshot().Hits {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// vr3SkillInk 数一块矩形里"与矩形外左侧 3px 处底色不同"的像素。
func vr3SkillInk(img *image.RGBA, r gfx.Rect) int {
	bg := img.RGBAAt(int(r.X)-3, int(r.Y+r.Height/2))
	ink := 0
	for y := int(r.Y); y < int(r.Y+r.Height); y++ {
		for x := int(r.X); x < int(r.X+r.Width); x++ {
			c := img.RGBAAt(x, y)
			d := vr3SkillAbs(int(c.R)-int(bg.R)) + vr3SkillAbs(int(c.G)-int(bg.G)) +
				vr3SkillAbs(int(c.B)-int(bg.B))
			if d > 24 {
				ink++
			}
		}
	}
	return ink
}

// vr3SkillDiff 数两块图在同一矩形内不同的像素数。
func vr3SkillDiff(a, b *image.RGBA, r gfx.Rect) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	count := 0
	for y := int(r.Y); y < int(r.Y+r.Height); y++ {
		for x := int(r.X); x < int(r.X+r.Width); x++ {
			if x < 0 || y < 0 || x >= a.Bounds().Max.X || y >= a.Bounds().Max.Y {
				continue
			}
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				count++
			}
		}
	}
	return count
}

func vr3SkillAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// vr3SkillLeadingRect 是 `.leading` 在行内的官方位置：
// `width:16px;height:16px`，`.row{align-items:center}` → 纵向居中于 24 高的行。
func vr3SkillLeadingRect(row gfx.Rect) gfx.Rect {
	return gfx.Rect{
		X: row.X, Y: row.Y + (SkillRowHeight-SkillLeadingHeight)/2,
		Width: SkillLeadingWidth, Height: SkillLeadingHeight,
	}
}

// vr3SkillTextColor 返回值为 value 的文字这一帧的绘制颜色。
func vr3SkillTextColor(session *headless.Session, value string) (gfx.Color, bool) {
	for _, text := range session.Snapshot().Texts {
		if text.Value == value {
			return text.Style.Color, true
		}
	}
	return gfx.Color{}, false
}

// TestVR3SkillRowHoverSwapsIconAndTintsText 真指针悬停一条**可展开**的技能行：
//
//	① `.iconIdle` → 0 / `.chevronHover` → 1：`.leading` 那 16×16 里必须换图（像素差 > 0）
//	② `.row:hover{color:label-secondary}`：整行文字从 label-tertiary 变 label-secondary
func TestVR3SkillRowHoverSwapsIconAndTintsText(t *testing.T) {
	dir := vr3SkillDir(t)
	harness := newSkillHarness(t, expandableCard())
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}

	row, ok := vr3SkillHitID(harness, session, "skill-row-1")
	if !ok {
		t.Fatalf("可展开的行没有命中区；hits=%+v", session.Snapshot().Hits)
	}
	if row.Bounds.Height != SkillRowHeight {
		t.Fatalf("行高（命中区）= %v，官方 `.row{height:24px}` = %v", row.Bounds.Height, SkillRowHeight)
	}
	leading := vr3SkillLeadingRect(row.Bounds)

	title := SkillText(harness.plugin.lang, KeyRowTitle)
	restColor, ok := vr3SkillTextColor(session, title)
	if !ok {
		t.Fatalf("行里没有标题 %q：texts=%v", title, session.Texts())
	}
	if restColor != harness.theme.LabelTertiary {
		t.Errorf("静止态行文字色 = %+v，官方 `.row{color:var(--dsw-alias-label-tertiary)}` = %+v",
			restColor, harness.theme.LabelTertiary)
	}
	rest, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	restInk := vr3SkillInk(rest, leading)
	t.Logf("VR3 静止态 .leading 墨量 = %d（图标 + 隐藏的 chevron）", restInk)
	vr3SkillShot(t, session, dir, "vr3-skill-row-rest.png")

	// 真指针进这一行（不是进 inspect 按钮）。
	if err := session.Move(row.Bounds.X+row.Bounds.Width/2, row.Bounds.Y+row.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	hover, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	vr3SkillShot(t, session, dir, "vr3-skill-row-hover.png")

	// ① 换图：`.row:hover .iconIdle{opacity:0}` + `.row:hover .chevronHover{opacity:1}`。
	swapped := vr3SkillDiff(rest, hover, leading)
	t.Logf("VR3 悬停后 .leading 像素变化 = %d", swapped)
	if swapped < 4 {
		t.Errorf("悬停整行后 `.leading` 只变了 %d 个像素——官方 `.row:hover .iconIdle{opacity:0}` + "+
			"`.row:hover .chevronHover{opacity:1}`（129__SkillRow.module.css.mjs.css:1；rc.2 asar 同款）"+
			"要求图标淡出、chevron 淡入", swapped)
	}

	// ② 文字换色：官方 `.title`/`.summary` 都是 `color:inherit`，整行跟着 `.row` 变。
	hoverColor, ok := vr3SkillTextColor(session, title)
	if !ok {
		t.Fatalf("悬停后标题不见了：texts=%v", session.Texts())
	}
	if hoverColor != harness.theme.LabelSecondary {
		t.Errorf("悬停后行文字色 = %+v，官方 `.row:hover{color:var(--dsw-alias-label-secondary)}` = %+v。"+
			"根因：skillrow.go:443-445 给 `.title` 的 Text 写死了 `Color(t.LabelTertiary)`，"+
			"而官方 `.title{...}` 不设 color（继承 `.row` 的），所以 hover 的换色被这行写死的颜色吃掉了。"+
			"修法：`.title` 与 `.summary` 不写 Color（或写 `Color(t.LabelTertiary)` 时改成 "+
			"`row.Hover(ui.Look{}.Color(t.LabelSecondary))` + 子 Text 继承），"+
			"error/stopped 两档仍保留它们各自的 `.errorSummary`/`.stoppedSummary` 颜色",
			hoverColor, harness.theme.LabelSecondary)
	}
}

// TestVR3SkillInspectButtonFocusVisible 走**真键盘**：Tab 到 inspect 按钮上，
// 官方要求它显形（`.inspectButton:focus-visible{opacity:1}`）。
func TestVR3SkillInspectButtonFocusVisible(t *testing.T) {
	dir := vr3SkillDir(t)
	harness := newSkillHarness(t, expandableCard())
	harness.plugin.expand.toggle(1) // 展开，`p.drawInspectButton` 才有机会画
	harness.plugin.SetInspect(func(string) {})
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}

	// 指针先离开：焦点态必须**自己**可见，不靠 hover。
	if err := session.Move(880, 680); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	// 静止态：官方 `.inspectButton{opacity:0}`，画面上不该有它的边框。
	if _, ok := rectOf(session, harness.theme.BorderL4, -1, 0); ok {
		t.Error("静止态画出了 inspect 按钮的边框——官方 `.inspectButton{opacity:0}`")
	}
	vr3SkillShot(t, session, dir, "vr3-skill-inspect-rest.png")

	inspectID := harness.tree.ID(harness.feat, "skill-inspect-1")
	reached := false
	for step := 0; step < 5; step++ {
		if err := session.Keys("tab"); err != nil {
			t.Fatalf("Keys(tab)：%v", err)
		}
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		if session.Snapshot().Focused == inspectID {
			reached = true
			break
		}
	}
	if !reached {
		t.Fatalf("Tab 走不到 inspect 按钮（focus=%v want=%v）", session.Snapshot().Focused, inspectID)
	}
	vr3SkillShot(t, session, dir, "vr3-skill-inspect-focused.png")

	button, ok := rectOf(session, harness.theme.BorderL4, -1, 0)
	if !ok {
		t.Errorf("键盘焦点落在 inspect 按钮上，按钮仍然不显形（画面上找不到 border-l4 的边框）——" +
			"官方 `.inspectButton:focus-visible{opacity:1}`（rc.2 asar 的 SkillRow.module.css；" +
			"rc.1 抽取件 129__SkillRow.module.css.mjs.css:1 同款）。" +
			"根因：official_skill.go:285 把 `SkillInspectFocusOpacity` 定为 1 却按 0 渲染，" +
			"skillrow.go:666 只挂了 `.Opacity(SkillInspectOpacity)` + `.Hover(...).StateFrom(card)`。" +
			"「viva 做不到」这条结论不成立：读 `Canvas.FocusVisible()==hitID` 就能条件给 1，" +
			"仓里已有同一手法（plugins/dsattachment/rail.go:1038、plugins/dsplan/focus.go:17 的 " +
			"`d.State.FocusVisible`）。修法：skillrow.go:656-676 先算 hitID，" +
			"`opacity := SkillInspectOpacity; if env.Canvas != nil && env.Canvas.FocusVisible() == hitID " +
			"{ opacity = SkillInspectFocusOpacity }`，再 `.Opacity(opacity)`")
	} else {
		t.Logf("VR3 焦点态 inspect 按钮 ✓ %+v", button.Bounds)
	}
}

// TestVR3SkillRowWithoutBodyHasNoChevron 是「没有说明正文」这一档（官方 `output === null`）：
// `.leading` 只有图标，**没有 chevron**，所以悬停不该换图；行也没有按钮语义（不可点）。
func TestVR3SkillRowWithoutBodyHasNoChevron(t *testing.T) {
	dir := vr3SkillDir(t)
	harness := newSkillHarness(t, dstool.Card{
		Seq: 7, Tool: SkillToolName, CallID: "call-7",
		Args:   `{"name":"no-body"}`,
		Status: dstool.StatusOK,
		// 没有 View：官方 `block.content` 为空 → `resultText` 返回 null → 不可展开。
	})
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}

	// 不可展开 ⇒ 官方不挂 role=button / tabIndex（`SkillRow.tsx:129-135`）。
	if hit, ok := vr3SkillHitID(harness, session, "skill-row-7"); ok {
		t.Errorf("没有说明正文的行仍挂了按钮命中区 %+v——官方 `expandable = output !== null`，"+
			"不可展开就不给按钮语义", hit.Bounds)
	}
	// 但图标还在（`disclosureLeading` 的 iconOnly 分支）。
	if _, ok := vr3SkillHitID(harness, session, "skill-card-7"); !ok {
		t.Fatalf("卡片本身没有命中区：hits=%+v", session.Snapshot().Hits)
	}
	card, _ := vr3SkillHitID(harness, session, "skill-card-7")
	leading := vr3SkillLeadingRect(gfx.Rect{X: card.Bounds.X, Y: card.Bounds.Y,
		Width: card.Bounds.Width, Height: SkillRowHeight})
	rest, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	restInk := vr3SkillInk(rest, leading)
	vr3SkillShot(t, session, dir, "vr3-skill-row-no-body.png")

	if err := session.Move(card.Bounds.X+card.Bounds.Width/2, card.Bounds.Y+SkillRowHeight/2); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	hover, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	vr3SkillShot(t, session, dir, "vr3-skill-row-no-body-hover.png")
	if changed := vr3SkillDiff(rest, hover, leading); changed != 0 {
		t.Errorf("没有说明正文的行，悬停后 `.leading` 变了 %d 个像素——官方 iconOnly 分支没有 "+
			"`.chevronHover`（`disclosureLeading`，复刻见 skillrow.go:333-341），所以不该换图",
			changed)
	}
	if restInk <= 0 {
		t.Errorf("`.leading` 里一个墨点都没有：官方 iconOnly 分支仍画 skill 图标（14×14）")
	}
}

// TestVR3SkillNarrowSummaryEllipsis 是**窄宽 900×700 + 超长摘要**：
// 官方 `.summary{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}`，
// 摘要必须收在行内，且行高不被撑开。
func TestVR3SkillNarrowSummaryEllipsis(t *testing.T) {
	dir := vr3SkillDir(t)
	longName := "a-very-long-skill-name-that-must-be-ellipsized-in-the-row"
	harness := newSkillHarness(t, dstool.Card{
		Seq: 21, Tool: SkillToolName, CallID: "call-21",
		Args:   `{"name":"` + longName + `"}`,
		Status: dstool.StatusOK,
		// 有正文才走 started 树（`.title ・ .summary` 那一行）；摘要就是 skill 名。
		View: &apiclient.ViewSpec{Kind: "plain", Body: "说明正文"},
	})
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	row, ok := vr3SkillHitID(harness, session, "skill-row-21")
	if !ok {
		t.Fatalf("有正文的行没有命中区；hits=%+v", session.Snapshot().Hits)
	}
	_ = row
	vr3SkillShot(t, session, dir, "vr3-skill-900x700-long-summary.png")

	// 摘要文字被 Ellipsis 截断时快照值以省略号收尾（官方 `text-overflow:ellipsis`）。
	var summary gfx.Text
	found := false
	for _, text := range session.Snapshot().Texts {
		if len(text.Value) > 8 && text.Value[:8] == "a-very-l" {
			summary, found = text, true
			break
		}
	}
	if !found {
		// 不可展开的卡没有 `.summary`（`summaryOf` 只在 started 树里画），这时改成断言
		// 整行唯一的那段文字仍在内容宽内。
		for _, text := range session.Snapshot().Texts {
			if text.Value == longName {
				summary, found = text, true
				break
			}
		}
	}
	if !found {
		t.Fatalf("超长技能名没排上版：texts=%v", session.Texts())
	}
	if summary.Style.Size == 0 {
		t.Fatalf("摘要文字没有字号信息：%+v", summary)
	}
	if !vr3SkillHasEllipsis(summary.Value) && summary.Bounds.Width > 860 {
		t.Errorf("超长技能名 %q 宽 %v 撑满整行且没有省略号——"+
			"官方 `.summary{text-overflow:ellipsis;overflow:hidden}` 应收在内容宽内",
			summary.Value, summary.Bounds.Width)
	}
	// 行高是硬值：`calc(24px + delta)`，长文本不许把它撑高。
	if row, ok := vr3SkillHitID(harness, session, "skill-row-21"); ok && row.Bounds.Height != SkillRowHeight {
		t.Errorf("超长文本把行高撑到 %v，官方 `.row{height:24px}` = %v", row.Bounds.Height, SkillRowHeight)
	}
}

// vr3SkillOutset 把矩形四边各外扩 d（官方 `outline` 落在元素外侧，采样要用它）。
func vr3SkillOutset(r gfx.Rect, d float32) gfx.Rect {
	return gfx.Rect{X: r.X - d, Y: r.Y - d, Width: r.Width + 2*d, Height: r.Height + 2*d}
}

// vr3SkillHasEllipsis 报告文字是否以省略号收尾（viva 的 Ellipsis 会写进快照值）。
func vr3SkillHasEllipsis(value string) bool {
	runes := []rune(value)
	return len(runes) > 0 && runes[len(runes)-1] == '…'
}

// 让 `app` 与 `image` 的 import 都用上（hit/primitives 类型断言在需要时用）。
var _ = app.RectanglePrimitive

// TestVR3SkillRowKeyboardFocusVisible 是**键盘焦点可见性**这条横切规则的旁证：
// rc.2 有一条全局规则给所有 `:focus-visible` 元素 2px 描边（`focus.css`：
// `:root{--dsw-focus-ring-width:2px}` + `:focus-visible{outline-color:var(--dsw-focus-ring-color,
// var(--dsw-alias-state-business-primary));outline-width:var(--dsw-focus-ring-width)}`），
// `.iJmaOG_row` 没有 `outline:none`，所以官方 Tab 到可展开的行上时行是有一圈 2px 描边的。
func TestVR3SkillRowKeyboardFocusVisible(t *testing.T) {
	dir := vr3SkillDir(t)
	harness := newSkillHarness(t, expandableCard())
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	row, ok := vr3SkillHitID(harness, session, "skill-row-1")
	if !ok {
		t.Fatalf("可展开的行没有命中区；hits=%+v", session.Snapshot().Hits)
	}
	// 指针离开，焦点态必须自己可见。
	if err := session.Move(880, 680); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	before, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	rowID := harness.tree.ID(harness.feat, "skill-row-1")
	reached := false
	for step := 0; step < 4; step++ {
		if err := session.Keys("tab"); err != nil {
			t.Fatalf("Keys(tab)：%v", err)
		}
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		if session.Snapshot().Focused == rowID {
			reached = true
			break
		}
	}
	if !reached {
		t.Fatalf("Tab 走不到可展开的行（focus=%v want=%v）", session.Snapshot().Focused, rowID)
	}
	vr3SkillShot(t, session, dir, "vr3-skill-row-focused.png")
	after, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	// 采样区 = 行矩形**加上外面 3px**：官方是 `outline`（画在元素外侧、以边为中心，
	// 宽 2px），只量行内会漏掉整圈环带。判据不变：环必须真的出现。
	changed := vr3SkillDiff(before, after, vr3SkillOutset(row.Bounds, 3))
	t.Logf("VR3 焦点落在技能行上时，行矩形（外扩 3px）内像素变化 = %d", changed)
	if changed == 0 {
		t.Errorf("键盘焦点落在可展开的技能行上，整行一个像素都没变——缺 rc.2 那条全局 2px 焦点描边" +
			"（`focus.css`：`:root{--dsw-focus-ring-width:2px}` + `:focus-visible{outline-color:…;" +
			"outline-width:var(--dsw-focus-ring-width)}`；`.iJmaOG_row` 没有 `outline:none`）。" +
			"根因与 inspect 按钮同源：skillrow.go:408-425 的 `.row` 只有 `Hover(ui.Look{}.Color(...))`，" +
			"没有 `Focus(` 也没有 ring。修法：`row.Focus(ui.Look{}.Ring(2, t.Info))`（viva 的 Ring 是" +
			"元素外描边，与 CSS `outline` 同义）")
	}
}
