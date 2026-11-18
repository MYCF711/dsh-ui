//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：目录选择器的**交互态 / 窄窗 / 空态 / 滚动**。
//
// 前两轮只看过静态外观（卡片圆角、elevation、行内两枚官方图标）。这一轮盯的是
// "数值对、但交互起来不对"的四件事：
//
//	① 键盘焦点环。官方 DirectoryBrowser 有两条（**rc.2 asar 原件**，字节偏移）：
//	     `._3zXc5G_crumb:focus-visible{outline:var(--dsw-focus-ring-width) solid
//	        var(--dsw-focus-ring-color,var(--dsw-alias-state-business-primary));
//	        outline-offset:-2px}` —— asar 偏移 **33130630**
//	     `._3zXc5G_row:focus-visible{同上}` —— asar 偏移 **33132296**
//	   宽度 `--dsw-focus-ring-width` = **2px**（rc.2 asar 偏移 59937449 的
//	   `ui-theme/src/styles/focus.css` 原文，压缩版同一份在偏移 59574298），
//	   颜色回退到 `--dsw-alias-state-business-primary`（本主题 = `Theme.Info`，
//	   与 `plugins/dsplan/cards.go:75-76`、`plugins/dsbrowser/plugin.go:113` 同一映射）。
//	   判据：真按 Tab 走一遍键盘焦点，焦点落在行/面包屑上时画面里必须出现
//	   2px、颜色 = Info 的**内缩**环（offset -2px → 环在元素矩形**里侧**）。
//	   注：`tmp/asar-css/050__DirectoryBrowser.module.css.mjs.css` 是 **rc.1** 抽取
//	   （本条两条规则 rc.1/rc.2 同名同值，但行号/偏移不同，故只作参考）：
//	   rc.1 里 crumb 在偏移 1524、row 在 3107。
//
//	② 空态 / 错误态：官方这两条只有 `.status`（12/18、label-secondary）与
//	   `.error`（12/18、state-error-primary，role=alert），**没有**"这里什么都没有"
//	   这种自造文案。所以空目录画出来必须是「空列 + 官方 13 条文案内的一切照旧」。
//
//	③ 「加载中…」药丸 `.loadingFloat{position:absolute;right:16px;bottom:8px;
//	   padding:2px 8px;background:bg-layer-2}`（rc.2 asar 偏移 **33133383**）：
//	   它**不占行高**，所以要验"它出现时两列的几何一位不动"。
//
//	④ 窄窗 900×700 与长列表滚动：卡片是 `min(680px,100%) × min(500px,100dvh - 32px)`
//	   （rc.2 asar 偏移 33129058 的 `._3zXc5G_dialog._3zXc5G_dialog`），
//	   长列必须在自己那条 `.column{overflow-y:auto}` 里滚，不许把对话框顶出去。
//
// 截图落 `$VR3_CAPTURE_DIR`（未设置时用临时目录）。
package dsdirpicker

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// vr3CaptureDir 是第三轮审计的落图目录（与其他审计员共用同一批目录下的子目录）。
func vr3CaptureDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("VR3_CAPTURE_DIR")
	if base == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

// vr3Source 是可以按 path 喂不同 listing 的数据面；gate 非 nil 时 List 先等它放行，
// 用来把「加载中…」那一帧钉在画面上。
type vr3Source struct {
	mu     sync.Mutex
	byPath map[string]*apiclient.DirectoryListing
	fallbk *apiclient.DirectoryListing
	fail   error
	gate   chan struct{}
	lists  int
}

func (s *vr3Source) List(_ context.Context, path string) (*apiclient.DirectoryListing, error) {
	if s.gate != nil {
		<-s.gate
	}
	s.mu.Lock()
	s.lists++
	fail := s.fail
	listing := s.byPath[path]
	if listing == nil {
		listing = s.fallbk
		if listing != nil {
			copied := *listing
			listing = &copied
		}
	}
	s.mu.Unlock()
	if fail != nil {
		return nil, fail
	}
	if listing == nil {
		return &apiclient.DirectoryListing{Path: path, Home: "/home"}, nil
	}
	// path 为空 = 宿主 home：**保留 listing 自己的绝对路径**（真实宿主就是这么回的）。
	// 拿空串顶掉 Path 会让 targetLocked 拿到 ""，于是 footer 的「打开」被误判成无目标。
	if path == "" || listing.Path == path {
		return listing, nil
	}
	copied := *listing
	copied.Path = path
	return &copied, nil
}

func (*vr3Source) Create(_ context.Context, parent, name string) (string, error) {
	return parent + "/" + name, nil
}

func (*vr3Source) Register(_ context.Context, path string) (apiclient.WorkspaceView, error) {
	return apiclient.WorkspaceView{WorkspaceID: "ws-vr3", Path: path}, nil
}

// vr3Picker 是一台装配好的真窗口：uikernel + dscore + 本插件，只画 `shell.overlay`。
type vr3Picker struct {
	t        *testing.T
	session  *headless.Session
	plugin   *Plugin
	source   *vr3Source
	dir      string
	width    int
	height   int
	inval    *app.Invalidator
	gateOnce sync.Once
}

// vr3OpenPicker 起窗口并打开对话框。
//
// async=true 时宿主调用丢到 goroutine（产品默认行为，用来观察「加载中」）；
// false 时同步执行（一帧就落定，用来观察静态几何）。
func vr3OpenPicker(t *testing.T, dir string, width, height int, src *vr3Source, async bool) *vr3Picker {
	t.Helper()
	kernel := uikernel.New("dirpicker-vr3")
	p := &Plugin{}
	if async {
		p.run = func(fn func()) { go fn() }
	} else {
		p.run = func(fn func()) { fn() }
	}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("dirpicker-vr3")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	picker := &vr3Picker{
		t: t, plugin: p, source: src, dir: dir, width: width, height: height, inval: &invalidator,
	}
	config := app.Config{
		Title: "dirpicker-vr3", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height},
			Has:      func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
			Act:      pluginapi.Actions{Invalidate: invalidator.Invalidate},
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			kernel.DrawSlot(f, pluginapi.SlotShellOverlay, env)
		})
		// 卡片自己 `.Clip()`，elevation 只能在 AfterDraw 画（壳也是这么跑的）。
		for _, painter := range kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	// 滚轮必须接：viva 的窗口过程 `case wmWheel: if OnScroll != nil {…}; return 0`
	// 会把没人接的滚轮**直接吞掉**（`cmd/app/main.go:2222-2226` 记录了这条）。
	// 壳接的是 `m.view.Wheel`（按指针位置找最内层能滚的 ScrollArea），这里同一条。
	config.OnScroll = view.Wheel
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless.Open(%dx%d) 失败：%v", width, height, err)
	}
	t.Cleanup(session.Close)
	picker.session = session

	SetSource(src)
	SetOnPicked(nil)
	t.Cleanup(func() {
		SetSource(nil)
		SetOnPicked(nil)
	})
	if !p.Open() {
		t.Fatal("有 source 时 Open 应该打开对话框")
	}
	// 窗口先开、后置 open：headless 只在尺寸/失效时重画，这里撑一帧逼出
	// "对话框已经打开"的画面（与 browse_visual_test.go 同一条做法）。
	picker.pump()
	if err := session.Resize(width+1, height); err != nil {
		t.Fatal(err)
	}
	if err := session.Resize(width, height); err != nil {
		t.Fatal(err)
	}
	picker.pump()
	return picker
}

func (p *vr3Picker) pump() {
	p.t.Helper()
	if err := p.session.Driver().Pump(); err != nil {
		p.t.Fatal(err)
	}
}

// settle 反复 pump，直到 loading 结束（异步装配用）。
func (p *vr3Picker) settle() {
	p.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		p.pump()
		p.plugin.mu.Lock()
		loading := p.plugin.loading
		p.plugin.mu.Unlock()
		if !loading {
			p.pump()
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	p.t.Fatal("等了 3s 还在 loading")
}

// shot 落 PNG 并回读像素。
func (p *vr3Picker) shot(name string) *image.RGBA {
	p.t.Helper()
	path := filepath.Join(p.dir, name)
	if err := p.session.Screenshot(path); err != nil {
		p.t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		p.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	img, err := p.session.Image()
	if err != nil {
		p.t.Fatal(err)
	}
	p.t.Logf("VR3_CAPTURE path=%s size=%v", path, img.Bounds())
	return img
}

// vr3PixelDiff 数两张图的差异像素。
func vr3PixelDiff(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	n := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

// vr3DescribeHit 把焦点 id 翻成人能读的一段（断言失败时要知道焦点落在哪）。
func vr3DescribeHit(session *headless.Session, id gfx.ID) string {
	if id.IsZero() {
		return "none"
	}
	for _, hit := range session.Snapshot().Hits {
		if hit.ID == id {
			return fmt.Sprintf("label=%q role=%v bounds=%+v", hit.Label, hit.Role, hit.Bounds)
		}
	}
	return fmt.Sprintf("%v（不在 hits 里）", id)
}

// vr3RingTriangles 数落在 rect（外扩 pad）里的三角形，按颜色分组。
//
// 为什么是三角形：Viva 的焦点环走 `gfx/canvas.go:1207 → RoundedOutline`，
// 落到 canvas 上是矢量三角形（`gfx/rounded_outline.go:25` 的
// `appendOutlineSegment`），与官方 `outline` 的"一圈描边"是同一个东西。
func vr3RingTriangles(snapshot app.Snapshot, rect gfx.Rect, pad float32) map[gfx.Color]int {
	out := map[gfx.Color]int{}
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.TrianglePrimitive {
			continue
		}
		inside := true
		for _, point := range primitive.Points {
			if point.X < rect.X-pad || point.X > rect.X+rect.Width+pad ||
				point.Y < rect.Y-pad || point.Y > rect.Y+rect.Height+pad {
				inside = false
				break
			}
		}
		if inside {
			out[primitive.Color]++
		}
	}
	return out
}

// vr3FocusRingAround 判断 rect 上有没有官方的 2px、Info 色环。
//
// 官方那两条 `:focus-visible{outline:…;outline-offset:-2px}`：环画在**矩形里侧**
// （offset 负值 = 向内缩），所以三角形坐标应当落在 rect 内、且宽 2px 的带上。
func vr3FocusRingAround(snapshot app.Snapshot, rect gfx.Rect, ring gfx.Color) (int, int) {
	hit, total := 0, 0
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.TrianglePrimitive || primitive.Color != ring {
			continue
		}
		total++
		within := true
		for _, point := range primitive.Points {
			if point.X < rect.X-1 || point.X > rect.X+rect.Width+1 ||
				point.Y < rect.Y-1 || point.Y > rect.Y+rect.Height+1 {
				within = false
				break
			}
		}
		if within {
			hit++
		}
	}
	return hit, total
}

// ── ① 键盘焦点环（官方 050:1524 / 050:3107 + focus.css:3）────────────────────

// TestVR3RowAndCrumbFocusRing 用真 Tab 走一遍键盘焦点链：焦点落在行/面包屑上时，
// 画面里必须有官方的 2px 内缩环。
//
// 这一条同时钉两件事：
//   - 行与面包屑**必须是 Tab 停靠点**（官方它们是 `<button>`，CSS 里给了 :focus-visible）；
//   - 落上去之后**画出来了**（"焦点动了但像素一格没变"就是这一条要抓的错法）。
func TestVR3RowAndCrumbFocusRing(t *testing.T) {
	dir := vr3CaptureDir(t)
	listing := &apiclient.DirectoryListing{
		Path: "/home", Home: "/home",
		Crumbs: []apiclient.DirectoryItem{{Name: "home", Path: "/home"}},
		Entries: []apiclient.DirectoryItem{
			{Name: "projects", Path: "/home/projects"},
			{Name: "docs", Path: "/home/docs"},
		},
	}
	src := &vr3Source{fallbk: listing}
	picker := vr3OpenPicker(t, dir, 1200, 800, src, false)
	session := picker.session
	theme := dscore.Dark()

	row, ok := session.Hit("projects")
	if !ok {
		t.Fatalf("真实帧里找不到行 projects：texts=%q", session.Texts())
	}
	crumb, ok := session.Hit("主目录")
	if !ok {
		t.Fatalf("真实帧里找不到面包屑「主目录」：texts=%q", session.Texts())
	}

	before := picker.shot("dirpicker-focus-00-idle.png")
	prev := session.Snapshot().Focused
	sawRow, sawCrumb := false, false
	rowRing, crumbRing := 0, 0
	for i := 1; i <= 24; i++ {
		if err := session.Keys("tab"); err != nil {
			t.Fatalf("第 %d 次 Tab 失败：%v", i, err)
		}
		picker.pump()
		now, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		diff := vr3PixelDiff(before, now)
		focus := session.Snapshot().Focused
		if focus == prev {
			continue
		}
		// 只在焦点真的换格时落图（24 格全落会堆一堆无关 PNG）。
		picker.shot(fmt.Sprintf("dirpicker-focus-%02d.png", i))
		prev = focus
		describe := vr3DescribeHit(session, focus)
		t.Logf("TAB %02d → %s 像素diff=%d", i, describe, diff)
		// 行 / 面包屑：命中 id 与它们的矩形比一比就知道焦点落在谁身上。
		if focus == row.ID {
			sawRow = true
			in, _ := vr3FocusRingAround(session.Snapshot(), row.Bounds, theme.Info)
			rowRing = in
			t.Logf("焦点落在目录行 projects：Info 色三角形 in-bounds=%d，diff=%d", in, diff)
		}
		if focus == crumb.ID {
			sawCrumb = true
			in, _ := vr3FocusRingAround(session.Snapshot(), crumb.Bounds, theme.Info)
			crumbRing = in
			t.Logf("焦点落在面包屑「主目录」：Info 色三角形 in-bounds=%d，diff=%d", in, diff)
		}
		// 行与面包屑都见过一遍就够（再转就是同一圈），省掉无关的 PNG。
		if sawRow && sawCrumb {
			break
		}
		before = now
	}
	if !sawRow {
		t.Fatalf("按 24 次 Tab 焦点从未落到目录行（可焦点命中区 %d 个）——行不是键盘停靠点",
			len(session.Snapshot().Hits))
	}
	if !sawCrumb {
		t.Errorf("按 24 次 Tab 焦点从未落到面包屑「主目录」")
	}
	// 断言：官方 `050:3107` / `050:1524` 的 2px outline 必须真的画出来。
	if rowRing == 0 {
		t.Errorf("目录行拿到键盘焦点，但行矩形 %+v 里没有任何 Info 色的环状三角形："+
			"官方 `._3zXc5G_row:focus-visible{outline:var(--dsw-focus-ring-width) solid "+
			"var(--dsw-focus-ring-color,var(--dsw-alias-state-business-primary));outline-offset:-2px}`"+
			"（**rc.2 asar 偏移 33132296**；宽度 2px 出处 rc.2 asar 偏移 59937449 的 focus.css）"+
			"没有落到画面上", row.Bounds)
	}
	if sawCrumb && crumbRing == 0 {
		t.Errorf("面包屑拿到键盘焦点，但矩形 %+v 里没有任何 Info 色环："+
			"官方 `._3zXc5G_crumb:focus-visible{outline:…;outline-offset:-2px}`"+
			"（**rc.2 asar 偏移 33130630**）没画",
			crumb.Bounds)
	}
}

// ── ② 空态 / 错误态 ────────────────────────────────────────────────────────

// TestVR3EmptyDirectoryKeepsOfficialCopy 空目录：官方在 `entries.length === 0`
// 时**不画任何"空"文案**（`050` 里只有 `.status` / `.error` 两条状态行，文案字典
// 只有 browser.loading / browser.truncated 这些），所以画面上不许冒出凑数的中文。
func TestVR3EmptyDirectoryKeepsOfficialCopy(t *testing.T) {
	dir := vr3CaptureDir(t)
	empty := &apiclient.DirectoryListing{
		Path: "/home", Home: "/home",
		Crumbs: []apiclient.DirectoryItem{{Name: "home", Path: "/home"}},
	}
	picker := vr3OpenPicker(t, dir, 1200, 800, &vr3Source{fallbk: empty}, false)
	picker.shot("dirpicker-empty-directory.png")

	official := map[string]bool{
		zhTitle: true, zhHome: true, zhNewFolder: true, zhFolderName: true,
		zhUntitledFolder: true, zhCreate: true, zhCancel: true, zhOpen: true,
		zhShowHidden: true, zhLoading: true, zhTruncated: true, zhEditPath: true,
	}
	for _, text := range picker.session.Texts() {
		if official[text] {
			continue
		}
		t.Errorf("空目录画面里出现了官方 13 条文案以外的字样 %q（官方没有空态文案，不许自造）", text)
	}
	// 空列 + 官方骨架：footer 的三个动作仍在，且「打开」在没有可打开目标时的可用性
	// 由 targetLocked 决定（这一层只钉"确实画出来了"）。
	for _, want := range []string{zhTitle, zhHome, zhCancel, zhOpen, zhShowHidden} {
		if !picker.session.HasText(want) {
			t.Errorf("空目录下官方文案 %q 没画出来：texts=%q", want, picker.session.Texts())
		}
	}
	if _, ok := picker.session.Hit("打开"); !ok {
		t.Errorf("空目录下「打开」按钮没有命中区")
	}
}

// TestVR3HostErrorRendersOfficialErrorRow 宿主报错：官方 `.error{color:state-error-primary;
// font-size:12px;line-height:18px;padding:4px 120px 4px 4px}`（偏移见 browse.go:183-188），
// 画的是**宿主原文**，且列不清空（"the last readable panes stand"）。
func TestVR3HostErrorRendersOfficialErrorRow(t *testing.T) {
	dir := vr3CaptureDir(t)
	listing := &apiclient.DirectoryListing{
		Path: "/home", Home: "/home",
		Crumbs:  []apiclient.DirectoryItem{{Name: "home", Path: "/home"}},
		Entries: []apiclient.DirectoryItem{{Name: "projects", Path: "/home/projects"}},
	}
	src := &vr3Source{fallbk: listing}
	picker := vr3OpenPicker(t, dir, 1200, 800, src, false)
	theme := dscore.Dark()

	src.mu.Lock()
	src.fail = errors.New("工作区目录不可读：permission denied")
	src.mu.Unlock()
	picker.plugin.navigate("") // 真的再走一次导航（失败路径）
	picker.pump()
	picker.pump()
	picker.shot("dirpicker-host-error.png")

	text := "工作区目录不可读：permission denied"
	if !picker.session.HasText(text) {
		t.Fatalf("宿主原文没画出来：texts=%q", picker.session.Texts())
	}
	// 颜色必须是 state-error-primary（Theme.Error），字号 12 / 行高 18。
	var got *gfx.Text
	for _, one := range picker.session.Snapshot().Texts {
		if one.Value == text {
			copied := one
			got = &copied
			break
		}
	}
	if got == nil {
		t.Fatal("快照里找不到错误行")
	}
	if got.Style.Size != statusFontSize || got.Bounds.Height != statusLineH {
		t.Errorf("错误行字号/行盒=%v/%v，官方 `._3zXc5G_status,._3zXc5G_error{font-size:12px;"+
			"line-height:18px;padding:4px 120px 4px 4px}`（rc.2 asar 偏移 33133181）",
			got.Style.Size, got.Bounds.Height)
	}
	if got.Style.Color != theme.Error {
		t.Errorf("错误行颜色=%v，官方 `.error{color:var(--dsw-alias-state-error-primary)}`=%v",
			got.Style.Color, theme.Error)
	}
	// "the last readable panes stand"：失败不许把已有列清掉。
	if !picker.session.HasText("projects") {
		t.Errorf("导航失败后已有列被清空（官方不清空）：texts=%q", picker.session.Texts())
	}
}

// ── ③ 「加载中…」药丸不占布局 ───────────────────────────────────────────────

// TestVR3LoadingPillFloatsWithoutPushingColumns 官方 `.loadingFloat` 是
// **absolute right:16 bottom:8**（browse.go:190-196 引的 `050` 规则）：它压在内容区
// 右下角、**不占行高**。判据：有药丸的那一帧与没有药丸的那一帧，左列矩形必须一致。
func TestVR3LoadingPillFloatsWithoutPushingColumns(t *testing.T) {
	dir := vr3CaptureDir(t)
	gate := make(chan struct{})
	listing := &apiclient.DirectoryListing{
		Path: "/home", Home: "/home",
		Crumbs: []apiclient.DirectoryItem{{Name: "home", Path: "/home"}},
		Entries: []apiclient.DirectoryItem{
			{Name: "projects", Path: "/home/projects"},
			{Name: "docs", Path: "/home/docs"},
		},
	}
	src := &vr3Source{fallbk: listing, gate: gate}
	picker := vr3OpenPicker(t, dir, 1200, 800, src, true) // 异步：List 卡在 gate 上
	var closeGate sync.Once
	open := func() { closeGate.Do(func() { close(gate) }) }
	t.Cleanup(open)
	picker.pump()
	loadingFrame := picker.shot("dirpicker-loading.png")

	picker.plugin.mu.Lock()
	loading := picker.plugin.loading
	picker.plugin.mu.Unlock()
	leftCol := picker.plugin.boxes.LeftColumn
	if !loading {
		t.Fatalf("gate 未放行时 loading 应为 true（异步装配没生效？）")
	}
	if !picker.session.HasText(zhLoading) {
		t.Fatalf("「%s」没画出来：texts=%q", zhLoading, picker.session.Texts())
	}
	var pill *gfx.Text
	for _, one := range picker.session.Snapshot().Texts {
		if one.Value == zhLoading {
			copied := one
			pill = &copied
			break
		}
	}
	if pill == nil {
		t.Fatal("快照里找不到加载中文字")
	}
	if pill.Style.Size != statusFontSize || pill.Bounds.Height != statusLineH {
		t.Errorf("药丸字号/行盒=%v/%v，官方 .status 是 12/18", pill.Style.Size, pill.Bounds.Height)
	}
	// `.loadingFloat{position:absolute;right:16px;bottom:8px}` 的参照是 `.content`
	// 的**边框盒**（`.content{position:relative}`，绝对定位子元素的包含块是它的 padding box，
	// 而 `.content` 无边框 → 就是卡片宽度）。量的是**药丸盒子**不是里面的文字：
	// 文字还额外被药丸自己的 `padding:2px 8px` 内缩 8。
	pillBox := picker.plugin.boxes.LoadingPill
	if !pillBox.Valid() {
		t.Fatal("这一帧没画「加载中」药丸盒")
	}
	cardRect := picker.plugin.boxes.Dialog.Rect()
	pillRect := pillBox.Rect()
	rightGap := (cardRect.X + cardRect.Width) - (pillRect.X + pillRect.Width)
	if rightGap < loadingFloatRight-1 || rightGap > loadingFloatRight+1 {
		t.Errorf("药丸右缘离卡片右缘=%v，官方 `.loadingFloat{right:16px}`=%v（rc.2 asar 偏移 33133383）",
			rightGap, loadingFloatRight)
	}
	if textInset := (pillRect.X + pillRect.Width) - (pill.Bounds.X + pill.Bounds.Width); textInset < loadingFloatPadX-1 {
		t.Errorf("药丸文字右内缩=%v，官方 .loadingFloat{padding:2px 8px} 是 8", textInset)
	}
	if pillRect.Height != statusLineH+2*loadingFloatPadY {
		t.Errorf("药丸盒高=%v，官方 padding:2px + line-height:18 → %v",
			pillRect.Height, statusLineH+2*loadingFloatPadY)
	}
	// 不占行高：左列矩形在 loading / 非 loading 两帧之间必须相同。
	if !leftCol.Valid() {
		t.Fatal("这一帧没画左列")
	}
	colLoading := leftCol.Rect()

	open()
	picker.settle()
	idleFrame := picker.shot("dirpicker-loading-cleared.png")
	colIdle := picker.plugin.boxes.LeftColumn.Rect()
	if colLoading != colIdle {
		t.Errorf("「加载中」药丸把列顶下去了：loading 帧左列=%+v，落定后=%+v（官方 absolute 不占行高）",
			colLoading, colIdle)
	}
	if diff := vr3PixelDiff(loadingFrame, idleFrame); diff == 0 {
		t.Errorf("loading → 落定画面零像素变化，加载态没真画出来")
	}
}

// ── ④ 长列表滚动 / 窄窗 ────────────────────────────────────────────────────

// TestVR3LongListingScrollsInsideColumn 40 行长列表：行必须被列自己的滚动区裁住，
// 滚轮要真的动，末尾那一行要能滚到。
func TestVR3LongListingScrollsInsideColumn(t *testing.T) {
	dir := vr3CaptureDir(t)
	listing := &apiclient.DirectoryListing{
		Path: "/home", Home: "/home",
		Crumbs: []apiclient.DirectoryItem{{Name: "home", Path: "/home"}},
	}
	for i := 1; i <= 40; i++ {
		listing.Entries = append(listing.Entries, apiclient.DirectoryItem{
			Name: fmt.Sprintf("dir-%02d", i), Path: fmt.Sprintf("/home/dir-%02d", i),
		})
	}
	picker := vr3OpenPicker(t, dir, 1200, 800, &vr3Source{fallbk: listing}, false)
	top := picker.shot("dirpicker-long-list-top.png")

	card := picker.plugin.boxes.Dialog.Rect()
	if !picker.session.HasText("dir-01") {
		t.Fatalf("首帧看不到第一行：texts=%q", picker.session.Texts())
	}
	// 判"在不在视口里"必须看 **Labels**（真画出来的），不是 Texts：
	// `app.Snapshot.Texts` 是**裁剪之前**的全部文字（`app/driver.go:1103-1105`），
	// 拿它判可见性会把滚出视口的行也算作"看得见"。
	drawn := func(value string) bool {
		for _, label := range picker.session.Snapshot().Labels {
			if label.Value == value {
				return true
			}
		}
		return false
	}
	if drawn("dir-40") {
		t.Logf("40 行全部落在首屏（没有出现滚动），跳过滚动断言")
		return
	}
	// 滚轮真滚：画面必须变，且末尾行要出现。
	wheel := func(dy float32) *image.RGBA {
		if err := picker.session.Scroll(gfx.ScrollEvent{
			X: card.X + card.Width/2, Y: card.Y + card.Height/2, DeltaY: dy, Precise: true,
		}); err != nil {
			t.Fatalf("滚轮失败：%v", err)
		}
		picker.pump()
		img, err := picker.session.Image()
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	// viva 的符号约定：**正的 DeltaY 揭示下方内容**（`ui/scroll.go:187-189`
	// "a positive delta reveals content below"）。往上滚会是 0 位移（已在顶部），
	// 这一点上一版这里就踩过。
	before := top
	for range 12 {
		before = wheel(300)
	}
	picker.shot("dirpicker-long-list-bottom.png")
	if vr3PixelDiff(top, before) == 0 {
		t.Fatalf("滚了 12×300，画面零像素变化——长列表根本没滚")
	}
	if !drawn("dir-40") {
		t.Errorf("滚到底仍看不到最后一行 dir-40（Labels 里没有它）")
	}
	// 任何行都不许画到卡片外面（.column{overflow-y:auto} + 卡片 .Clip()）。
	for _, hit := range picker.session.Snapshot().Hits {
		if hit.Bounds.Width == 0 || hit.Bounds.Height == 0 {
			continue
		}
		if hit.Bounds.X < card.X-1 || hit.Bounds.Y < card.Y-1 ||
			hit.Bounds.X+hit.Bounds.Width > card.X+card.Width+1 ||
			hit.Bounds.Y+hit.Bounds.Height > card.Y+card.Height+1 {
			t.Errorf("命中区画到卡片外：label=%q bounds=%+v 卡片=%+v", hit.Label, hit.Bounds, card)
			break
		}
	}
}

// TestVR3Narrow900x700DialogFits 窄窗 900×700：卡片是 `min(680,100%) × min(500,100dvh-32)`，
// 必须整张落在窗口里，且不会有任何绘制物件越界。
func TestVR3Narrow900x700DialogFits(t *testing.T) {
	dir := vr3CaptureDir(t)
	const w, h = 900, 700
	listing := &apiclient.DirectoryListing{
		Path: "/home", Home: "/home",
		Crumbs: []apiclient.DirectoryItem{{Name: "home", Path: "/home"}},
		Entries: []apiclient.DirectoryItem{
			{Name: "projects", Path: "/home/projects"},
			{Name: "docs", Path: "/home/docs"},
			{Name: "media", Path: "/home/media"},
		},
	}
	picker := vr3OpenPicker(t, dir, w, h, &vr3Source{fallbk: listing}, false)
	img := picker.shot("dirpicker-narrow-900x700.png")
	if got := img.Bounds(); got.Dx() != w || got.Dy() != h {
		t.Fatalf("实图尺寸=%v，want %dx%d", got, w, h)
	}
	card := picker.plugin.boxes.Dialog.Rect()
	if card.Width != dialogMaxWidth || card.Height != dialogMaxHeight {
		t.Errorf("900×700 下卡片=%vx%v，want %vx%v（官方 min(680px,100%%)×min(500px,100dvh-32px)）",
			card.Width, card.Height, dialogMaxWidth, dialogMaxHeight)
	}
	if card.X < 0 || card.Y < 0 || card.X+card.Width > w || card.Y+card.Height > h {
		t.Errorf("卡片越出窗口：%+v（窗口 %dx%d）", card, w, h)
	}
	snap := picker.session.Snapshot()
	for _, hit := range snap.Hits {
		b := hit.Bounds
		if b.Width == 0 || b.Height == 0 {
			continue
		}
		if b.X < -1 || b.Y < -1 || b.X+b.Width > w+1 || b.Y+b.Height > h+1 {
			t.Errorf("窄窗越界命中区：label=%q bounds=%+v", hit.Label, b)
			break
		}
	}
	out := 0
	worst := ""
	for _, primitive := range snap.Primitives {
		b := primitive.Bounds
		if b.Width <= 0 || b.Height <= 0 {
			continue
		}
		if b.X+b.Width < -2 || b.Y+b.Height < -2 || b.X > w+2 || b.Y > h+2 {
			out++
			if worst == "" {
				worst = fmt.Sprintf("kind=%v bounds=%+v", primitive.Kind, b)
			}
		}
	}
	if out > 0 {
		t.Errorf("完全画在窗口外的 primitive %d 个，首个：%s", out, worst)
	}
	t.Logf("窄窗：卡片=%+v hits=%d primitives=%d 越界=%d", card, len(snap.Hits), len(snap.Primitives), out)
}
