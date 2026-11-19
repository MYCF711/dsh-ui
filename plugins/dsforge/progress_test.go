//go:build darwin || windows || linux

// progress_test.go —— 创建进度卡与 blocker 挡板的**真实像素判据**（Viva headless）。
//
// 两条判据都对着"值不是常量"来写：
//
//	进度  同一份数据面只把 `worktree.creation.phase` 从 prepare 换成 checkout，
//	      "创建分支 / 检出文件"两行的颜色必须对翻，且 prepare 行变成官方 done 勾
//	      （绿色 check 图元）——常量、写死或读错字段都会红。
//	挡板  `worktree.editable == false` 时整块不画；`blocker` 两值时勾选框**画但禁用**
//	      且浮条是官方那两句原因；点它没有任何动作发出。
//
// 官方出处见 progress.go / hero.go 文件头（`WorktreeProgress.tsx:59-91,160-181`、
// `WorktreeToggle.tsx:97-165`、`WorktreeProgress.module.css`、`locales.ts:303-332`）。
package dsforge

import (
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// forgeCardCalls 记下插件真的发出了哪些动作（挡板"点不动"要能证伪）。
type forgeCardCalls struct {
	requested []*string
	cancelled int
	stopped   int
}

// forgeCardsFixture 是一块只画目标卡片的画布（与 visual_test 的整页夹具分开：
// 这里要精确到"这一张卡画了什么"，整页会把别的卡的文案混进断言）。
type forgeCardsFixture struct {
	t           *testing.T
	tree        *gfx.Tree
	feat        gfx.Scope
	theme       *pluginapi.Theme
	p           *Plugin
	session     *headless.Session
	invalidator *app.Invalidator
	calls       forgeCardCalls
	// envValue 是这一帧复用的插件环境（页对象只建一次，每帧只换 Data/Act）。
	envValue *pluginapi.Env
}

// newForgeCardsFixture 造夹具；draw 决定这一帧画哪几张卡（顺序即自上而下）。
func newForgeCardsFixture(t *testing.T, view *pluginapi.ForgeView, cur string,
	draw func(*Plugin, ui.C, *pluginapi.Env)) *forgeCardsFixture {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{}
	if err := p.Register(testContext{theme: theme}); err != nil {
		t.Fatal(err)
	}
	f := &forgeCardsFixture{t: t, theme: theme, p: p}
	f.tree = &gfx.Tree{}
	f.feat = f.tree.Feature("forge-cards")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	f.invalidator = &app.Invalidator{}
	f.env(view, cur)
	config := app.Config{
		Title: "forge-cards", Width: 860, Height: 480, Tree: f.tree,
		WindowMode: app.WindowFrameless, Invalidator: f.invalidator,
		Paint: func(canvas *gfx.Canvas, size ui.Size) {
			bounds := gfx.Rect{Width: size.Width, Height: size.Height}
			canvas.Fill(bounds, theme.BgBase)
			f.env(view, cur).Canvas = canvas
			f.env(view, cur).Bounds = bounds
			page.Draw(canvas, bounds, func(c ui.C) {
				ui.Column(c).Width(760).FillHeight().Pad(12).Gap(10).Children(func() {
					draw(p, c, f.env(view, cur))
				})
			})
		},
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	f.session = session
	f.pump()
	f.pump()
	t.Cleanup(session.Close)
	return f
}

// env 造这一帧的插件环境（动作面记进 calls，判"点得动/点不动"）。
func (f *forgeCardsFixture) env(view *pluginapi.ForgeView, cur string) *pluginapi.Env {
	if f.envValue == nil {
		f.envValue = &pluginapi.Env{
			Theme: f.theme, Tree: f.tree, Feat: f.feat,
			Geometry:  pluginapi.Geometry{Width: 860, Height: 480, MainWidth: 760},
			MainWidth: 760,
		}
	}
	f.envValue.Act = pluginapi.Actions{
		SelectRightTab: func(string) {},
		Forge: &pluginapi.ForgeActions{
			Refresh:      func() {},
			LoadBranches: func() {},
			LoadScope:    func(string) {},
			LoadBases:    func(string) {},
			LoadText:     func(string) {},
			RequestWorktree: func(from *string) {
				f.calls.requested = append(f.calls.requested, from)
			},
			CancelWorktree: func() { f.calls.cancelled++ },
			StopCreation:   func() { f.calls.stopped++ },
		},
	}
	f.envValue.Data = pluginapi.Snapshot{Cur: cur, Forge: view}
	return f.envValue
}

func (f *forgeCardsFixture) pump() {
	f.t.Helper()
	f.invalidator.Invalidate()
	if err := f.session.Driver().Pump(); err != nil {
		f.t.Fatal(err)
	}
}

// moveTo 把指针移到 (x,y) 并重画（悬停态由驱动在指针移动时更新）。
func (f *forgeCardsFixture) moveTo(x, y float32) {
	f.t.Helper()
	if err := f.session.Move(x, y); err != nil {
		f.t.Fatal(err)
	}
	f.pump()
	f.pump()
}

// click 点一个命中区并重画。
func (f *forgeCardsFixture) click(hit gfx.Hit) {
	f.t.Helper()
	if err := f.session.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		f.t.Fatal(err)
	}
	f.pump()
	f.pump()
}

// hit 找命中区；没有就 nil。
func (f *forgeCardsFixture) hit(name string) (gfx.Hit, bool) {
	id := f.tree.ID(f.feat, name)
	for _, candidate := range f.session.Snapshot().Hits {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return gfx.Hit{}, false
}

// hasText 报告这一帧有没有画某段文字。
func (f *forgeCardsFixture) hasText(value string) bool {
	for _, text := range f.session.Snapshot().Texts {
		if strings.Contains(text.Value, value) {
			return true
		}
	}
	return false
}

// textColor 取某段文字的绘制颜色（官方状态色就落在这一层）。
func (f *forgeCardsFixture) textColor(value string) (ui.Color, bool) {
	for _, text := range f.session.Snapshot().Texts {
		if text.Value == value {
			return text.Style.Color, true
		}
	}
	return ui.Color{}, false
}

// hasWrappedText 报告这一帧有没有画出某段文字，**允许被换行切开**。
//
// 浮条（Tooltip）会把长句按宽度折行，`Snapshot.Texts` 里就是几段；把它们按绘制
// 顺序拼回去（去掉空白）再比，才是"官方整句真的画出来了"。
func (f *forgeCardsFixture) hasWrappedText(value string) bool {
	var joined strings.Builder
	for _, text := range f.session.Snapshot().Texts {
		joined.WriteString(text.Value)
	}
	normalize := func(s string) string { return strings.ReplaceAll(s, " ", "") }
	return strings.Contains(normalize(joined.String()), normalize(value))
}

// countColor 数这一帧有多少图元用了某个颜色（官方 done 勾是 state-success-primary）。
func (f *forgeCardsFixture) countColor(color ui.Color) int {
	n := 0
	for _, prim := range f.session.Snapshot().Primitives {
		if prim.Color == color && prim.Color.Alpha > 0 {
			n++
		}
	}
	return n
}

// creationView 造一份只有工作树那一格有意义的数据面。
func creationView(worktree pluginapi.ForgeWorktree) *pluginapi.ForgeView {
	return &pluginapi.ForgeView{
		SessionID: "s-1",
		Checkout:  pluginapi.ForgeCheckout{Kind: pluginapi.ForgeCheckoutReady, Branch: "feat/base"},
		Worktree:  worktree,
	}
}

// runningCreation 是"创建正在跑"的官方状态（`ForgeWorktreeCreation`）。
func runningCreation(phase string, held bool) pluginapi.ForgeWorktree {
	from := "main"
	return pluginapi.ForgeWorktree{
		PanelKnown: true,
		Editable:   false, // 已经在建，官方 editable 就是 false
		Creation: &pluginapi.ForgeWorktreeCreation{
			Phase:  phase,
			Branch: "dsh/feat-base",
			From:   &from,
			Held:   held,
		},
	}
}

// drawProgressOnly 只画进度卡。
func drawProgressOnly(p *Plugin, c ui.C, env *pluginapi.Env) { p.drawWorktreeProgress(c, env) }

// TestProgressCardDrawsOfficialCopy 钉住官方文案与结构
// （WorktreeProgress.tsx:138-181 / locales.ts:319-332）。
func TestProgressCardDrawsOfficialCopy(t *testing.T) {
	view := creationView(runningCreation(pluginapi.ForgeCreationPrepare, true))
	f := newForgeCardsFixture(t, view, "s-1", drawProgressOnly)
	for _, want := range []string{
		"正在创建工作树", "dsh/feat-base", "创建分支", "检出文件", "更多详情", "取消",
	} {
		if !f.hasText(want) {
			t.Fatalf("创建进度卡缺官方文案 %q", want)
		}
	}
	// "更多详情"是按钮（官方 `<button className={css.disclosure}>`），要有身份。
	if _, ok := f.hit("forge-creation-details"); !ok {
		t.Fatal("「更多详情」没有命中区（画了但点不开）")
	}
}

// TestProgressFollowsCreationPhase 是**变异判据**：只换 phase，两行步骤的颜色
// 与 done 勾必须跟着换（写死、常量、读错字段都会红）。
func TestProgressFollowsCreationPhase(t *testing.T) {
	prepare := newForgeCardsFixture(t, creationView(runningCreation(pluginapi.ForgeCreationPrepare, true)), "s-1", drawProgressOnly)
	prepareBranch, ok := prepare.textColor("创建分支")
	if !ok {
		t.Fatal("prepare 帧没有「创建分支」这一行")
	}
	prepareCheckout, ok := prepare.textColor("检出文件")
	if !ok {
		t.Fatal("prepare 帧没有「检出文件」这一行")
	}
	// 官方 `.step[data-state='running']{color:label-primary}`、其余 label-secondary。
	if prepareBranch != prepare.theme.LabelPrimary {
		t.Fatalf("prepare 帧「创建分支」颜色 = %v，running 要用 label-primary", prepareBranch)
	}
	if prepareCheckout != prepare.theme.LabelSecondary {
		t.Fatalf("prepare 帧「检出文件」颜色 = %v，未开始要用 label-secondary", prepareCheckout)
	}
	if n := prepare.countColor(prepare.theme.Success); n != 0 {
		t.Fatalf("prepare 帧不该有 done 勾（绿色图元 %d 个）", n)
	}

	checkout := newForgeCardsFixture(t, creationView(runningCreation(pluginapi.ForgeCreationCheckout, true)), "s-1", drawProgressOnly)
	checkoutBranch, _ := checkout.textColor("创建分支")
	checkoutStep, _ := checkout.textColor("检出文件")
	if checkoutBranch != checkout.theme.LabelSecondary {
		t.Fatalf("checkout 帧「创建分支」颜色 = %v，已经做过要用 label-secondary", checkoutBranch)
	}
	if checkoutStep != checkout.theme.LabelPrimary {
		t.Fatalf("checkout 帧「检出文件」颜色 = %v，running 要用 label-primary", checkoutStep)
	}
	if n := checkout.countColor(checkout.theme.Success); n == 0 {
		t.Fatal("checkout 帧必须有 done 勾（prepare 那步已完成，官方 `.done{state-success-primary}`）")
	}
}

// TestProgressHiddenWithoutCreation：没有在跑的创建就一个字都不画
// （官方 `WorktreeProgress.tsx:266` 的 `creation === undefined → null`）。
func TestProgressHiddenWithoutCreation(t *testing.T) {
	view := creationView(pluginapi.ForgeWorktree{
		PanelKnown: true, Editable: true,
	})
	f := newForgeCardsFixture(t, view, "s-1", drawProgressOnly)
	for _, unwanted := range []string{"正在创建工作树", "创建分支", "检出文件", "更多详情"} {
		if f.hasText(unwanted) {
			t.Fatalf("没有创建时不该画 %q", unwanted)
		}
	}
}

// TestProgressCancelOnlyWhileHeld：官方只在 `creation.held` 时画「取消」
// （WorktreeProgress.tsx:176-180），点它调 stopCreation。
func TestProgressCancelOnlyWhileHeld(t *testing.T) {
	held := newForgeCardsFixture(t, creationView(runningCreation(pluginapi.ForgeCreationCheckout, true)), "s-1", drawProgressOnly)
	if !held.hasText("取消") {
		t.Fatal("held 的创建没有「取消」按钮")
	}
	var cancelHit gfx.Hit
	found := false
	for _, hit := range held.session.Snapshot().Hits {
		if hit.Label == "取消" {
			cancelHit, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("「取消」按钮没有命中区")
	}
	held.click(cancelHit)
	if held.calls.stopped != 1 {
		t.Fatalf("点「取消」应发出一次 stopCreation，实得 %d 次", held.calls.stopped)
	}

	notHeld := newForgeCardsFixture(t, creationView(runningCreation(pluginapi.ForgeCreationCheckout, false)), "s-1", drawProgressOnly)
	if notHeld.hasText("取消") {
		t.Fatal("held=false 时官方不画「取消」（没有消息在等，停不掉）")
	}
}

// TestBlockerBaffleHidesToggleWhenNotEditable：官方 `!editable` 整块不画
// （WorktreeToggle.tsx:103），连 not-git 也一样（:104-105）。
func TestBlockerBaffleHidesToggleWhenNotEditable(t *testing.T) {
	notEditable := creationView(pluginapi.ForgeWorktree{PanelKnown: true, Editable: false})
	f := newForgeCardsFixture(t, notEditable, "s-1", drawToggleOnly)
	if f.hasText("新建工作树") {
		t.Fatal("editable=false 时官方整块不画，但这里画了勾选框")
	}

	notGit := creationView(pluginapi.ForgeWorktree{
		PanelKnown: true, Editable: true, Blocker: pluginapi.ForgeWorktreeBlockerNotGit,
	})
	f2 := newForgeCardsFixture(t, notGit, "s-1", drawToggleOnly)
	if f2.hasText("新建工作树") {
		t.Fatal("blocker=not-git 时官方同样不画（WorktreeToggle.tsx:104-105）")
	}
}

// drawToggleOnly 只画 hero 上的工作树勾选。
func drawToggleOnly(p *Plugin, c ui.C, env *pluginapi.Env) { p.drawWorktreeToggle(c, env) }

// TestBlockerBaffleKeepsToggleDisabledWithOfficialCopy 钉住两块挡板的**外观与交互**：
// 勾选框照画但禁用（`.toggle[data-disabled]{label-dimmed}`），浮条是官方那一句原因，
// 点它不发任何动作。
func TestBlockerBaffleKeepsToggleDisabledWithOfficialCopy(t *testing.T) {
	cases := []struct {
		blocker string
		tip     string
	}{
		{pluginapi.ForgeWorktreeBlockerReadOnly, "只读模式下无法使用工作树"},
		{pluginapi.ForgeWorktreeBlockerOutsideWritableRoot,
			"这个工作区的 Git 数据不在可写目录中（例如工作区本身就是工作树），暂不支持使用工作树"},
	}
	for _, tc := range cases {
		view := creationView(pluginapi.ForgeWorktree{
			PanelKnown: true, Editable: true, Blocker: tc.blocker,
		})
		f := newForgeCardsFixture(t, view, "s-1", drawToggleOnly)
		if !f.hasText("新建工作树") {
			t.Fatalf("blocker=%s 时勾选框应当照画（官方只禁用不隐藏）", tc.blocker)
		}
		// 官方 `data-disabled` → label-dimmed。
		if color, ok := f.textColor("新建工作树"); !ok || color != f.theme.LabelDimmed {
			t.Fatalf("blocker=%s 时勾选框文案颜色 = %v，禁用态要用 label-dimmed", tc.blocker, color)
		}
		toggle, ok := f.hit("forge-worktree-toggle")
		if !ok {
			t.Fatalf("blocker=%s 时勾选框没有身份（浮条挂不上）", tc.blocker)
		}
		f.moveTo(toggle.Bounds.X+toggle.Bounds.Width/2, toggle.Bounds.Y+toggle.Bounds.Height/2)
		if err := f.session.Wait(300 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
		f.pump()
		if !f.hasWrappedText(tc.tip) {
			t.Fatalf("blocker=%s 的挡板浮条没出来（要官方 `worktree.blocked.%s`）", tc.blocker, tc.blocker)
		}
		// 点它：官方 `disabled` 的 checkbox 点不动，壳里也不许发请求。
		f.click(toggle)
		if len(f.calls.requested) != 0 || f.calls.cancelled != 0 {
			t.Fatalf("被挡住的勾选框点出了动作：request=%d cancel=%d",
				len(f.calls.requested), f.calls.cancelled)
		}
	}
}

// TestToggleRequestsWithCurrentBranchWithoutBlocker 是上一条的**反向对照**：
// 没有挡板时同一处点击必须真的发出 `request(当前分支)`（官方 onChange → request(option.current)）。
func TestToggleRequestsWithCurrentBranchWithoutBlocker(t *testing.T) {
	view := creationView(pluginapi.ForgeWorktree{PanelKnown: true, Editable: true})
	f := newForgeCardsFixture(t, view, "s-1", drawToggleOnly)
	toggle, ok := f.hit("forge-worktree-toggle")
	if !ok {
		t.Fatal("没有挡板时勾选框必须有命中区")
	}
	f.click(toggle)
	if len(f.calls.requested) != 1 {
		t.Fatalf("点勾选应发出一次 requestWorktree，实得 %d 次", len(f.calls.requested))
	}
	if from := f.calls.requested[0]; from == nil || *from != "feat/base" {
		t.Fatalf("基准分支 = %v，官方 unchecked→checked 是 request(option.current)=feat/base", from)
	}
}
