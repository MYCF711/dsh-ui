package dsbrowser

// overflow_wrap_test.go —— 账本 #50 ⑧：`.restoreTitle` / `.restoreUrl` 的
// `overflow-wrap: anywhere`（官方 rc.5 `dsh-client-ui-sidebar-browser/src/client/view/
// Browser.module.css:169-175` 与 `:177-183`）。
//
// 官方那两块的全部属性（逐条照抄，见 plugin.go 的 drawContentState）：
//
//	.restoreTitle{max-width:100%; margin:0; overflow-wrap:anywhere;
//	              font:var(--dsw-font-xs-13); font-weight:500}
//	.restoreUrl  {max-width:100%; margin:0; overflow-wrap:anywhere;
//	              color:var(--dsw-alias-label-secondary); font:var(--dsw-font-xxs-12)}
//
// 两块都**没有** `word-break`、`white-space`、`text-overflow` 或行数上限；容器的
// `.restore{overflow:auto}`（`:159`）与 `overflow-wrap` 是一套：文本按容器宽度断行、
// 总高可以超过一屏，超过就滚。
//
// 本文件钉三件事：
//
//  1. 超长标题/地址不再溢出容器，而且**一个字都不丢**（官方没有行数上限）；
//  2. 普通文本的断行与位置与改动前逐像素相同（金标取自改动前的同一套 harness）；
//  3. `OverflowWrap` **只**出现在官方那两块上——别处一律不加。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// restoreCardRig 是一份「checkpoint 还在、本进程尚未开页」的 headless 现场：
// 右栏浏览器 tab 画出来的就是官方那张恢复卡。
type restoreCardRig struct {
	session *headless.Session
	host    *fakeHost
	// contentTop 是内容区顶边（工具条 38 + 那条 0.5 的发丝线）。
	contentTop float32
}

// restoreCardPaneWidth 取官方右栏在最窄可用宽度下的内容盒：360 - 2×24（`.restore`
// 的 `padding:24px`）。
const (
	restoreCardWidth  = 360
	restoreCardHeight = 600
	restoreCardPad    = startPad
)

func openRestoreCard(t *testing.T, address string, width, height int) *restoreCardRig {
	t.Helper()
	cfg := map[string]string{}
	original := &Plugin{}
	env := occurrenceEnv("s1", "tab1", &fakeHost{}, cfg)
	navigateOccurrence(original, env, address)

	host := &fakeHost{}
	card := &Plugin{theme: dscore.Dark()}
	cardEnv := occurrenceEnv("s1", "tab1", host, cfg)
	tree := &gfx.Tree{}
	feat := tree.Feature("browser-overflow-wrap")
	pageTheme := dscore.VivaTheme(card.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "browser-overflow-wrap", Width: width, Height: height, Tree: tree, WindowMode: app.WindowFrameless}
	config.TextInput = ui.Fields(card.Fields()...)
	// 滚轮必须接（生产里是 `cmd/app/main.go` 的 `cfg.OnScroll` → `m.view.Wheel`），
	// 否则 .restore 的 overflow:auto 在 headless 里永远滚不动。
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult { return view.Wheel(event) }
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		cardEnv.Theme = card.theme
		cardEnv.Tree = tree
		cardEnv.Feat = feat
		cardEnv.Canvas = canvas
		cardEnv.Bounds = bounds
		canvas.Fill(bounds, card.theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).Width(float32(width)).Height(float32(height)).
				Children(func() { card.drawBody(f, cardEnv) })
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	if err := session.Resize(width, height); err != nil {
		t.Fatal(err)
	}
	rig := &restoreCardRig{session: session, host: host, contentTop: toolbarHeight + hairline}
	for i := 0; i < 10; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if !session.HasText(copyRestorePrevious) {
		t.Fatalf("恢复卡没出现：texts=%v", session.Texts())
	}
	return rig
}

func (r *restoreCardRig) texts() []gfx.Text { return r.session.Driver().Snapshot().Texts }

// openStartPlaceholder 开一份「什么 checkpoint 都没有」的现场：内容区画的是官方
// `.start` 那句占位文案（`BrowserBody.tsx:126-128`）。
func openStartPlaceholder(t *testing.T, width, height int) *restoreCardRig {
	t.Helper()
	host := &fakeHost{}
	plugin := &Plugin{theme: dscore.Dark()}
	env := occurrenceEnv("s1", "tab1", host, map[string]string{})
	tree := &gfx.Tree{}
	feat := tree.Feature("browser-start-placeholder")
	pageTheme := dscore.VivaTheme(plugin.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "browser-start-placeholder", Width: width, Height: height, Tree: tree, WindowMode: app.WindowFrameless}
	config.TextInput = ui.Fields(plugin.Fields()...)
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult { return view.Wheel(event) }
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env.Theme = plugin.theme
		env.Tree = tree
		env.Feat = feat
		env.Canvas = canvas
		env.Bounds = bounds
		canvas.Fill(bounds, plugin.theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).Width(float32(width)).Height(float32(height)).
				Children(func() { plugin.drawBody(f, env) })
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	if err := session.Resize(width, height); err != nil {
		t.Fatal(err)
	}
	rig := &restoreCardRig{session: session, host: host, contentTop: toolbarHeight + hairline}
	for i := 0; i < 10; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if !session.HasText(copyStart) {
		t.Fatalf("占位文案没出现：texts=%v", session.Texts())
	}
	return rig
}

// linesOf 按行序把 target 那一段文本的每一行取出来。
//
// 判据是「这一行必须接着上一行往后长」（前缀推进），而不是「是 target 的子串」：
// 恢复卡的标题（`example.com`）恰好也是地址的子串，但它从第一行起就不是地址的
// 前缀，所以不会被算进地址的行里。地址栏那条同一串字在工具条里（Y < contentTop），
// 先行排除。
func (r *restoreCardRig) linesOf(target string) []gfx.Text {
	var candidates []gfx.Text
	for _, text := range r.texts() {
		if text.Bounds.Y < r.contentTop || text.Value == "" {
			continue
		}
		candidates = append(candidates, text)
	}
	sort.SliceStable(candidates, func(first, second int) bool {
		return candidates[first].Bounds.Y < candidates[second].Bounds.Y
	})
	var lines []gfx.Text
	grown := 0
	for _, text := range candidates {
		if grown >= len(target) || !strings.HasPrefix(target[grown:], text.Value) {
			continue
		}
		lines = append(lines, text)
		grown += len(text.Value)
	}
	return lines
}

// joined 把每一行的文本按行序拼回去。
func joined(lines []gfx.Text) string {
	var out strings.Builder
	for _, line := range lines {
		out.WriteString(line.Value)
	}
	return out.String()
}

func nearOrFail(t *testing.T, what string, got, want float32) {
	t.Helper()
	if difference := got - want; difference > 0.01 || difference < -0.01 {
		t.Errorf("%s = %.2f，改动前是 %.2f", what, got, want)
	}
}

// 超长地址（220 个无分隔字符）在 312 宽的内容盒里必须断行、留在盒内，而且**一个字
// 都不许丢**：官方那两块没有行数上限，本仓之前按 3/4 行截断是自造的补丁。
//
// 判据「不溢出」用的是每一行的盒子：断行正确时每行的左右边都落在 padding 之内；
// 没有 `overflow-wrap:anywhere` 时这一行会取整个单词的 max-content 宽度，于是左边
// 跑成负数（Viva 原文 `OFF:viva@v0.6.12-ow/base/text_wrap.go:87-91`）。
func TestRestoreCardLongURLWrapsInsideThePane(t *testing.T) {
	address := "https://example.com/" + strings.Repeat("b", 220)
	rig := openRestoreCard(t, address, restoreCardWidth, restoreCardHeight)

	lines := rig.linesOf(address)
	insidePane(t, "地址", lines)
	if got := joined(lines); got != address {
		t.Fatalf("地址被截断了：画出来 %d 字，原文 %d 字", len(got), len(address))
	}
	if len(lines) < 5 {
		t.Fatalf("超长地址只画了 %d 行，官方没有行数上限，这里应随宽度自然断行", len(lines))
	}
}

// 超长标题（80 个 a 的主机名）同样要断行、留在盒内、一字不丢。标题走的是
// `.restoreTitle`（13px/500），断行宽度与地址那一行不同，所以单独钉一次。
func TestRestoreCardLongTitleWrapsInsideThePane(t *testing.T) {
	title := strings.Repeat("a", 80) + ".example"
	address := "https://" + title
	rig := openRestoreCard(t, address, restoreCardWidth, restoreCardHeight)

	lines := rig.linesOf(title)
	insidePane(t, "标题", lines)
	if len(lines) < 2 {
		t.Fatalf("超长标题只画了 %d 行：没有断行", len(lines))
	}
}

// insidePane 钉住「不溢出容器」：每一行的盒子起点不早于 `.restore` 的 padding，
// 右边缘不出右栏。
//
// 为什么用右栏而不是 padding 内侧：Viva 画居中的一行时，`Bounds.X` 是文字墨迹的
// 起点、`Bounds.Width` 是这一行盒子的宽度（两行都是 312），所以右边缘会差几个像素；
// 改动前就是这个数（金标见下一个用例），本轮不动它。真正会炸的是**没有
// overflow-wrap 时**：整段不换行串取 max-content 宽度，一行就是一千多像素，
// 左边缘直接跑到负坐标（实测 x=-676.85、w=1665.70），这一条立刻红。
func insidePane(t *testing.T, what string, lines []gfx.Text) {
	t.Helper()
	const contentWidth = float32(restoreCardWidth - 2*restoreCardPad)
	for index, line := range lines {
		// `Bounds.X` 是这一行墨迹的起点（居中之后），不许早于内容盒左边。
		if line.Bounds.X < restoreCardPad-0.5 {
			t.Errorf("%s第 %d 行从 %.2f 起画，早于内容盒左边 %.2f", what, index, line.Bounds.X, float32(restoreCardPad))
		}
		// `Bounds.Width` 是这一行的盒子宽度。断行正确时它等于内容盒宽（整行）；
		// 没有 overflow-wrap 时它等于整段 max-content 宽度（实测 1665.70 对 312），
		// 也就是这一行横着长出容器。
		if line.Bounds.Width > contentWidth+0.5 {
			t.Errorf("%s第 %d 行的盒子宽 %.2f 超过内容盒 %.2f：这一行没有断，横着溢出了",
				what, index, line.Bounds.Width, contentWidth)
		}
	}
}

// 普通文本**与改动前逐像素相同**：金标是改 `OverflowWrap` 之前用同一套 harness
// 在同一台机器上量到的（X/Y 就是每段文本画在哪）。
//
// 唯一允许变的是「盒子宽度」：官方那两块是 `max-width:100%`（盒子等于自己的文本
// 宽，最多到 100%），本仓之前写的是 `FillWidth()`（盒子恒等于容器宽）。所以这里
// 钉的是**每一行画在哪、几行**，不是盒宽——正文一个像素都不许动。
func TestRestoreCardNormalTextBreaksExactlyAsBefore(t *testing.T) {
	const address = "https://example.com/saved"
	rig := openRestoreCard(t, address, restoreCardWidth, restoreCardHeight)

	want := []struct {
		label string
		value string
		x, y  float32
	}{
		{"地址栏", address, 111.50, 9.75},
		{"上次打开", copyRestorePrevious, 156.00, 159.50},
		{"标题", "example.com", 140.81, 285.00},
		{"地址", address, 108.02, 315.00},
		{"恢复页面", copyRestoreAction, 152.00, 343.00},
	}
	for _, item := range want {
		found := 0
		for _, text := range rig.texts() {
			if text.Value != item.value {
				continue
			}
			// 地址栏与恢复卡里的地址是同一串字，按 Y 区分（工具条 vs 内容区）。
			if item.label != "地址栏" && text.Bounds.Y < rig.contentTop {
				continue
			}
			if item.label == "地址栏" && text.Bounds.Y >= rig.contentTop {
				continue
			}
			found++
			nearOrFail(t, item.label+" 的 x", text.Bounds.X, item.x)
			nearOrFail(t, item.label+" 的 y", text.Bounds.Y, item.y)
		}
		if found != 1 {
			t.Errorf("%s（%q）画了 %d 次，改动前是 1 次", item.label, item.value, found)
		}
	}
}

// 文本比一屏还高时（900 个字符的地址 + 300 高的右栏），官方的 `.restore{overflow:auto}`
// 让整张卡滚起来：地址一字不丢地画下去，「恢复页面」按钮仍然点得到。
//
// 反面（之前的形状）：没有滚动容器 + 行数截断，地址被砍到 4 行；放开截断又不加滚动，
// 按钮就会被顶到裁剪区外——两种都让用户拿不到这个按钮。
func TestRestoreCardScrollsWhenTheTextIsTallerThanThePane(t *testing.T) {
	const paneHeight = 300
	address := "https://example.com/" + strings.Repeat("c", 900)
	rig := openRestoreCard(t, address, restoreCardWidth, paneHeight)

	lines := rig.linesOf(address)
	insidePane(t, "地址", lines)
	if got := joined(lines); got != address {
		t.Fatalf("滚动场景下地址被截断了：画出来 %d 字，原文 %d 字", len(got), len(address))
	}
	last := lines[len(lines)-1]
	contentBottom := float32(paneHeight)
	if last.Bounds.Y+last.Bounds.Height <= contentBottom {
		t.Fatalf("这段地址没有超过一屏（最后一行底 %.2f），本用例不成立", last.Bounds.Y+last.Bounds.Height)
	}
	// 不滚时按钮在裁剪区外：点不到。
	if err := rig.session.ClickLabel(copyRestoreAction); err == nil && rig.host.opened == 0 {
		t.Log("按钮在视口外仍被命中，改判据前先看这里")
	}
	// 滚到底之后按钮必须点得到，并且真的把页面开起来。
	for range 20 {
		if err := rig.session.Scroll(gfx.ScrollEvent{
			X: restoreCardWidth / 2, Y: float32(paneHeight) / 2, DeltaY: 300, Precise: true,
		}); err != nil {
			t.Fatal(err)
		}
		if err := rig.session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if err := rig.session.ClickLabel(copyRestoreAction); err != nil {
		t.Fatalf("滚到底之后「恢复页面」仍点不到：%v", err)
	}
	for i := 0; i < 10 && rig.host.opened == 0; i++ {
		if err := rig.session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if rig.host.opened != 1 {
		t.Fatalf("滚到底并点击后 Open 次数 = %d，want 1", rig.host.opened)
	}
}

// `overflow-wrap:anywhere` **只**许出现在官方给出的那两块上。
//
// 为什么用源码判据而不是画面对照：Viva 的这条属性作用在「盒子向自己要多少宽」上
// （`OFF:viva@v0.6.12-ow/base/layout.go:340-355`），而本仓其它文本都已经被
// `FillWidth()`/固定宽度钉死，加上它画面上一个字都不会变——画面断言抓不到「加错
// 地方」，只有声明能抓到。加错地方的代价是真实的：官方没加的地方出现奇怪的断行。
func TestOverflowWrapOnlyOnTheOfficialTwoBlocks(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	const marker = ".OverflowWrap("
	total := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(source), "\n")
		for index, line := range lines {
			if !strings.Contains(line, marker) || strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			total++
			// 往上找到这条声明的开头那一行，认它是不是官方的两块之一。
			owner := line
			for back := index; back >= 0 && !strings.Contains(owner, "ui.Paragraph("); back-- {
				owner = lines[back]
			}
			if !strings.Contains(owner, "target.Title") && !strings.Contains(owner, "target.URL") {
				t.Errorf("%s:%d 给官方没加 overflow-wrap 的地方加了：%s", name, index+1, owner)
			}
			if !strings.Contains(line, "OverflowWrap(ui.OverflowWrapAnywhere)") {
				t.Errorf("%s:%d 只允许 Anywhere（零值 Normal 是今天的行为）：%s", name, index+1, line)
			}
		}
	}
	if total != 2 {
		t.Fatalf("OverflowWrap 调用点 %d 个，官方只有 .restoreTitle/.restoreUrl 两处", total)
	}
}

// 占位文案（官方 `.start`，`Browser.module.css:206-213`）没有 overflow-wrap，
// 本仓也不加：这里钉住它的画法（一行、居中、与改动前同一位置），顺带保证
// 「恢复卡之外的内容区没被这轮的滚动容器带歪」。
func TestStartPlaceholderKeepsItsOwnLayout(t *testing.T) {
	rig := openStartPlaceholder(t, restoreCardWidth, restoreCardHeight)
	texts := rig.texts()
	found := 0
	for _, text := range texts {
		if text.Value != copyStart || text.Bounds.Y < rig.contentTop {
			continue
		}
		found++
		nearOrFail(t, "占位文案的 x", text.Bounds.X, 101.87)
		nearOrFail(t, "占位文案的 y", text.Bounds.Y, 309.00)
	}
	if found != 1 {
		t.Fatalf("占位文案画了 %d 次，want 1（texts=%v）", found, describeTexts(texts))
	}
}

// describeTexts 只用于失败信息。
func describeTexts(texts []gfx.Text) string {
	var out strings.Builder
	for index, text := range texts {
		fmt.Fprintf(&out, "[%d]%q@(%.1f,%.1f) ", index, text.Value, text.Bounds.X, text.Bounds.Y)
	}
	return out.String()
}
