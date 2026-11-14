package dsdirpicker

// 无头测试：真的画一帧（ui.NewPage + gfx.Canvas），点真的 Hit id，断言窗口里的文字
// 与几何。不 mock 绘制层、不绕过按钮直接调方法——点击走的是产品同一条路径。
//
// 官方几何的证据全部在 browse.go 的常量注释里（OFF:…module.css 的行号）。
//
// 一个 viva 的硬约束：`ui.Box` 只在**本次 paint** 内有效（`box.go:9-12`），跨帧再读
// 会 panic。所以每次 draw 完立刻把矩形抄成纯数值（snap），断言只对数值做。

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 假数据源：记录每一次真实调用 ──────────────────────────────────────────

type fakeCreate struct {
	parent string
	name   string
}

type fakeSource struct {
	mu        sync.Mutex
	levels    map[string]*apiclient.DirectoryListing
	lists     []string
	creates   []fakeCreate
	registers []string
	// err 非 nil 时三条调用全部失败（用来验「宿主报错原文」那条）。
	err error
}

func (f *fakeSource) List(_ context.Context, path string) (*apiclient.DirectoryListing, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lists = append(f.lists, path)
	if f.err != nil {
		return nil, f.err
	}
	if listing, ok := f.levels[path]; ok {
		return listing, nil
	}
	return &apiclient.DirectoryListing{Path: path}, nil
}

func (f *fakeSource) Create(_ context.Context, parent, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates = append(f.creates, fakeCreate{parent: parent, name: name})
	if f.err != nil {
		return "", f.err
	}
	return parent + "/" + name, nil
}

func (f *fakeSource) Register(_ context.Context, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registers = append(f.registers, path)
	return f.err
}

func (f *fakeSource) listed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lists...)
}

func (f *fakeSource) registered() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.registers...)
}

func (f *fakeSource) created() []fakeCreate {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCreate(nil), f.creates...)
}

// ── 夹具 ─────────────────────────────────────────────────────────────────

// fixture 是一个设备齐全的无头环境：真主题、真 Tree、真 Canvas。
//
// `run` 换成同步执行：产品路径把宿主调用丢给 goroutine，测试必须在同一线程看到
// 结算结果，否则断言就是撞运气（同 dssettings 的 applyRowWrite 思路）。
func fixture(t *testing.T, src DirSource) (*Plugin, *pluginapi.Env, *gfx.Tree, *gfx.Canvas) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("dirpicker-test")
	p := &Plugin{
		theme: theme,
		run:   func(fn func()) { fn() },
	}
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feat}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	SetSource(src)
	SetOnPicked(nil)
	t.Cleanup(func() {
		SetSource(nil)
		SetOnPicked(nil)
	})
	return p, env, tree, canvas
}

// frame 是一帧画完之后抄下来的几何（纯数值，可以跨帧比较）。
type frame struct {
	DialogW, DialogH float32
	Rows             []gfx.Rect
	LeftColumnW      float32
	RightColumnW     float32
	DividerW         float32
	Footer           gfx.Rect
	FooterInner      gfx.Rect
	CreateDialogW    float32
	CreateInputH     float32
}

// rectOf 抄一个 Box 的矩形；这一帧没画这个节点就是零值（Box.Valid 会告诉我们）。
func rectOf(box ui.Box) gfx.Rect {
	if !box.Valid() {
		return gfx.Rect{}
	}
	return box.Rect()
}

// snap 把这一帧的几何抄成数值。**必须紧接着 draw 调**（Box 不跨帧）。
func (p *Plugin) snap() frame {
	out := frame{
		DialogW:       rectOf(p.boxes.Dialog).Width,
		DialogH:       rectOf(p.boxes.Dialog).Height,
		LeftColumnW:   rectOf(p.boxes.LeftColumn).Width,
		RightColumnW:  rectOf(p.boxes.RightColumn).Width,
		DividerW:      rectOf(p.boxes.Divider).Width,
		Footer:        rectOf(p.boxes.Footer),
		FooterInner:   rectOf(p.boxes.FooterInner),
		CreateDialogW: rectOf(p.boxes.CreateDialog).Width,
		CreateInputH:  rectOf(p.boxes.CreateInput).Height,
	}
	for _, row := range p.boxes.Rows {
		out.Rows = append(out.Rows, rectOf(row))
	}
	return out
}

// drawOnce 画一帧并抄下几何。viewport 就是 `100dvh` 那个参照物。
//
// **先 Reset**：canvas 的图元是累加的（dssidebar 的测试也是这么写的），不 Reset
// 的话"上一帧画过、这一帧不画"的节点会留在画面上，断言就成了假的。
// Reset 不动事件态（`clicked` / `eventPass`），所以放在 BeginEventPass 之后也安全。
func drawOnce(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, w, h float32) frame {
	t.Helper()
	canvas.Reset()
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: w, Height: h}, func(f ui.C) { p.draw(f, env) })
	return p.snap()
}

// click 走一次真实的点击事件通道：BeginEventPass → 画一帧 → EndEventPass。
func click(t *testing.T, p *Plugin, env *pluginapi.Env, tree *gfx.Tree, canvas *gfx.Canvas, w, h float32, id string) frame {
	t.Helper()
	canvas.BeginEventPass(tree.ID(env.Feat, id), gfx.ClickInfo{})
	out := drawOnce(t, p, env, canvas, w, h)
	canvas.EndEventPass()
	return out
}

// textValues 把这一帧画出来的文字全部列出来（断言失败时给人看现场）。
func textValues(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// hasText 报告画面里有没有这一段文字（完全相等）。
func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

// textOf 返回画面上那一段文字（缺失立即失败，不用零值误判字号）。
func textOf(t *testing.T, canvas *gfx.Canvas, value string) gfx.Text {
	t.Helper()
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text
		}
	}
	t.Fatalf("画面缺少 %q；实际文字=%v", value, textValues(canvas))
	return gfx.Text{}
}

// homeListing 是一层夹着「普通目录 + 隐藏目录」的 home。
func homeListing() *apiclient.DirectoryListing {
	return &apiclient.DirectoryListing{
		Path: "/home",
		Home: "/home",
		Crumbs: []apiclient.DirectoryItem{
			{Name: "home", Path: "/home"},
		},
		Entries: []apiclient.DirectoryItem{
			{Name: "projects", Path: "/home/projects"},
			{Name: "docs", Path: "/home/docs"},
			{Name: ".cache", Path: "/home/.cache", Hidden: true},
		},
	}
}

// TestOfficialGeometryConstants 把官方几何值逐条钉住（改动必须对着 CSS 行号改）。
func TestOfficialGeometryConstants(t *testing.T) {
	cases := map[string]struct{ got, want float32 }{
		// OFF:DirectoryBrowser.module.css:6-7
		"dialogMaxWidth":      {dialogMaxWidth, 680},
		"dialogMaxHeight":     {dialogMaxHeight, 500},
		"dialogViewportInset": {dialogViewportInset, 32},
		// :26-33 / :35-44
		"headerPadTop":    {headerPadTop, 16},
		"headerPadRight":  {headerPadRight, 14},
		"headerPadBottom": {headerPadBottom, 8},
		"headerPadLeft":   {headerPadLeft, 24},
		"headerGap":       {headerGap, 8},
		"titleMinHeight":  {titleMinHeight, 28},
		"titleFontSize":   {titleFontSize, 16},
		"titleLineH":      {titleLineH, 24},
		"titleWeight":     {titleWeight, 510},
		// :51-61 / :114-127
		"crumbBarMinHeight": {crumbBarMinHeight, 24},
		"crumbBarPullLeft":  {crumbBarPullLeft, -9},
		"crumbBarPadX":      {crumbBarPadX, 8},
		"crumbBarGap":       {crumbBarGap, 4},
		"crumbMaxWidth":     {crumbMaxWidth, 160},
		"crumbFontSize":     {crumbFontSize, 13},
		"crumbLineH":        {crumbLineH, 20},
		// :205-215 / :83-94 / :221-232 / :234-238
		"contentPadTop":    {contentPadTop, 16},
		"contentPadRight":  {contentPadRight, 16},
		"contentPadBottom": {contentPadBottom, 16},
		"contentPadLeft":   {contentPadLeft, 24},
		"millerGap":        {millerGap, 12},
		"columnMinWidth":   {columnMinWidth, 256},
		"columnGap":        {columnGap, 2},
		"columnPadRight":   {columnPadRight, 8},
		"dividerWidth":     {dividerWidth, 0.5},
		// :245-258 / :293-303
		"rowHeight":       {rowHeight, 28},
		"rowGap":          {rowGap, 4},
		"rowPad":          {rowPad, 4},
		"rowNameFontSize": {rowNameFontSize, 13},
		"rowNameLineH":    {rowNameLineH, 20},
		"rowNameWeight":   {rowNameWeight, 500},
		// :310-327 / :348-358 / :398-400
		"statusFontSize":       {statusFontSize, 12},
		"statusLineH":          {statusLineH, 18},
		"footerPadV":           {footerPadV, 16},
		"footerPadH":           {footerPadH, 24},
		"footerGap":            {footerGap, 8},
		"footerActionMinWidth": {footerActionMinWidth, 72},
		// :403-455
		"createDialogMaxWidth": {createDialogMaxWidth, 380},
		"createBodyPadTop":     {createBodyPadTop, 22},
		"createBodyPadX":       {createBodyPadX, 24},
		"createBodyPadBottom":  {createBodyPadBottom, 20},
		"createBodyGap":        {createBodyGap, 12},
		"createInputHeight":    {createInputHeight, 44},
		"createActionsGap":     {createActionsGap, 8},
	}
	for name, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s=%v，官方源=%v", name, tc.got, tc.want)
		}
	}
}

// TestDialogGeometryMatchesOfficial 量卡片/行/列/footer 的**实际**布局。
func TestDialogGeometryMatchesOfficial(t *testing.T) {
	src := &fakeSource{levels: map[string]*apiclient.DirectoryListing{"": homeListing()}}
	p, env, _, canvas := fixture(t, src)
	if !p.Open() {
		t.Fatal("有 source 时 Open 应该打开对话框")
	}
	got := drawOnce(t, p, env, canvas, 1200, 800)

	// `.dialog.dialog{width:min(680px,100%);height:min(500px,calc(100dvh - 32px))}`（:5-7）。
	if got.DialogW != 680 || got.DialogH != 500 {
		t.Fatalf("卡片=%vx%v，want 680x500（module.css:6-7）", got.DialogW, got.DialogH)
	}

	// 行高 28（`.row{height:28px}`，:250）；默认滤掉隐藏条目 → 只剩两行。
	if len(got.Rows) != 2 {
		t.Fatalf("默认（隐藏未显示）可见行=%d，want 2", len(got.Rows))
	}
	for i, row := range got.Rows {
		if row.Height != 28 {
			t.Errorf("第 %d 行高=%v，want 28（module.css:250）", i, row.Height)
		}
	}

	// 列 floor 256（`.column{min-width:256px}`，:226）；单列时它吃掉整个内容宽。
	if got.LeftColumnW < 256 {
		t.Errorf("左列宽=%v，want >= 256（module.css:226）", got.LeftColumnW)
	}

	// footer padding 16px / 24px（`.footerBar{padding:16px 24px}`，:356）。
	if d := got.FooterInner.X - got.Footer.X; d != 24 {
		t.Errorf("footer 左 padding=%v，want 24", d)
	}
	if d := got.FooterInner.Y - got.Footer.Y; d != 16 {
		t.Errorf("footer 上 padding=%v，want 16", d)
	}
	if d := got.Footer.Y + got.Footer.Height - (got.FooterInner.Y + got.FooterInner.Height); d != 16 {
		t.Errorf("footer 下 padding=%v，want 16", d)
	}

	// 文案与字号：标题 16/510、行名 13/500（`.title` :38-42、`.rowName` :299-301）。
	title := textOf(t, canvas, zhTitle)
	if title.Style.Size != 16 || title.Style.Weight != 510 {
		t.Errorf("标题字号/字重=%v/%v，want 16/510", title.Style.Size, title.Style.Weight)
	}
	if title.Bounds.Height != 28 {
		t.Errorf("标题框高=%v，want 28（min-height:28px，:38）", title.Bounds.Height)
	}
	name := textOf(t, canvas, "projects")
	if name.Style.Size != 13 || name.Style.Weight != 500 || name.Bounds.Height != 20 {
		t.Errorf("行名字号/字重/行高=%v/%v/%v，want 13/500/20",
			name.Style.Size, name.Style.Weight, name.Bounds.Height)
	}
}

// TestDialogHeightClampsToViewport 验 `.dialog` 那条 `calc(100dvh - 32px)` 的钳制。
func TestDialogHeightClampsToViewport(t *testing.T) {
	src := &fakeSource{levels: map[string]*apiclient.DirectoryListing{"": homeListing()}}
	p, env, _, canvas := fixture(t, src)
	p.Open()
	got := drawOnce(t, p, env, canvas, 1200, 400)
	if got.DialogH != 400-32 {
		t.Fatalf("短视口卡片高=%v，want 368（min(500, 100dvh-32)，:7）", got.DialogH)
	}
}

// TestRowClickListsChildPath 点左列一行 → 对**那一行**的路径发 List（Miller 右列）。
func TestRowClickListsChildPath(t *testing.T) {
	src := &fakeSource{levels: map[string]*apiclient.DirectoryListing{"": homeListing()}}
	p, env, tree, canvas := fixture(t, src)
	p.Open()
	drawOnce(t, p, env, canvas, 1200, 800)

	click(t, p, env, tree, canvas, 1200, 800, "row-/home/projects")
	listed := src.listed()
	if len(listed) < 2 || listed[len(listed)-1] != "/home/projects" {
		t.Fatalf("点行后 List 调用=%v，want 最后一次是 /home/projects", listed)
	}
	p.mu.Lock()
	child := p.child
	p.mu.Unlock()
	if child == nil || child.Path != "/home/projects" {
		t.Fatalf("右列 listing=%+v，want path=/home/projects", child)
	}

	// 两列 + 分隔线（`.millerRow{gap:12px}` / `.divider{width:.5px}` / `.column{min-width:256px}`）。
	got := drawOnce(t, p, env, canvas, 1200, 800)
	if got.DividerW != 0.5 {
		t.Errorf("分隔线宽=%v，want 0.5（module.css:236）", got.DividerW)
	}
	if got.RightColumnW < 256 {
		t.Errorf("右列宽=%v，want >= 256（module.css:226）", got.RightColumnW)
	}
}

// TestOpenRegistersCurrentDirectory 「打开」= 对当前目录调 workspace.create。
func TestOpenRegistersCurrentDirectory(t *testing.T) {
	src := &fakeSource{levels: map[string]*apiclient.DirectoryListing{"": homeListing()}}
	p, env, tree, canvas := fixture(t, src)
	picked := ""
	SetOnPicked(func(path string) { picked = path })

	p.Open()
	drawOnce(t, p, env, canvas, 1200, 800)
	click(t, p, env, tree, canvas, 1200, 800, "open")

	if got := src.registered(); len(got) != 1 || got[0] != "/home" {
		t.Fatalf("Register 调用=%v，want [/home]", got)
	}
	if picked != "/home" {
		t.Fatalf("选定回调收到 %q，want /home", picked)
	}
	p.mu.Lock()
	open := p.open
	p.mu.Unlock()
	if open {
		t.Fatal("注册成功后对话框应该关闭")
	}
}

// TestNewFolderCreatesInCurrentDirectory 「新建文件夹」= Create(当前目录, 输入名)，
// 成功后进入新目录。
func TestNewFolderCreatesInCurrentDirectory(t *testing.T) {
	src := &fakeSource{levels: map[string]*apiclient.DirectoryListing{"": homeListing()}}
	p, env, tree, canvas := fixture(t, src)
	p.Open()
	drawOnce(t, p, env, canvas, 1200, 800)

	click(t, p, env, tree, canvas, 1200, 800, "new-folder")
	p.mu.Lock()
	createOpen := p.createOpen
	p.mu.Unlock()
	if !createOpen {
		t.Fatal("点「新建文件夹」没有弹出子对话框")
	}

	// 子对话框：标题/说明行/输入框/两个按钮都在，盒子宽 380、输入框高 44。
	dialog := drawOnce(t, p, env, canvas, 1200, 800)
	if dialog.CreateDialogW != 380 {
		t.Errorf("子对话框宽=%v，want 380（module.css:404）", dialog.CreateDialogW)
	}
	if dialog.CreateInputH != 44 {
		t.Errorf("输入框高=%v，want 44（module.css:434）", dialog.CreateInputH)
	}
	if !hasText(canvas, createInText("home")) {
		t.Fatalf("子对话框缺少说明行 %q；实际文字=%v", createInText("home"), textValues(canvas))
	}
	for _, label := range []string{zhNewFolder, zhCreate, zhCancel} {
		if !hasText(canvas, label) {
			t.Errorf("子对话框缺少 %q", label)
		}
	}

	p.createName.SetText("新生目录")
	click(t, p, env, tree, canvas, 1200, 800, "create-confirm")

	if got := src.created(); len(got) != 1 || got[0].parent != "/home" || got[0].name != "新生目录" {
		t.Fatalf("Create 调用=%v，want [{/home 新生目录}]", got)
	}
	p.mu.Lock()
	level, stillCreating := p.level, p.creating
	p.mu.Unlock()
	if stillCreating {
		t.Fatal("创建完成后 creating 应该落回 false")
	}
	if level == nil || level.Path != "/home/新生目录" {
		t.Fatalf("创建后当前层=%+v，want 进入 /home/新生目录", level)
	}
}

// TestHiddenEntriesHiddenUntilToggled 默认滤掉宿目标了 Hidden 的条目，开关后出现。
func TestHiddenEntriesHiddenUntilToggled(t *testing.T) {
	src := &fakeSource{levels: map[string]*apiclient.DirectoryListing{"": homeListing()}}
	p, env, tree, canvas := fixture(t, src)
	p.Open()
	drawOnce(t, p, env, canvas, 1200, 800)

	if hasText(canvas, ".cache") {
		t.Fatal("默认不该画出隐藏条目 .cache")
	}
	if !hasText(canvas, "projects") {
		t.Fatal("普通条目 projects 应该画出来")
	}

	click(t, p, env, tree, canvas, 1200, 800, "toggle-hidden")
	p.mu.Lock()
	shown := p.showHidden
	p.mu.Unlock()
	if !shown {
		t.Fatal("点「显示隐藏文件」没有把开关打开")
	}
	drawOnce(t, p, env, canvas, 1200, 800)
	if !hasText(canvas, ".cache") {
		t.Fatalf("开关打开后应画出 .cache；实际文字=%v", textValues(canvas))
	}
}

// TestNoSourceDrawsNothing 没有 source = 一个节点都不画，Open 也不该假装打开。
func TestNoSourceDrawsNothing(t *testing.T) {
	src := &fakeSource{levels: map[string]*apiclient.DirectoryListing{"": homeListing()}}
	p, env, _, canvas := fixture(t, src)

	SetSource(nil)
	if p.Open() {
		t.Fatal("没有 source 时 Open 不该返回 true（不许画死按钮）")
	}
	got := drawOnce(t, p, env, canvas, 1200, 800)
	if texts := textValues(canvas); len(texts) != 0 {
		t.Fatalf("没有 source 时不该画任何文字，实际=%v", texts)
	}
	if got.DialogW != 0 {
		t.Fatalf("没有 source 时不该有卡片，实际宽=%v", got.DialogW)
	}

	// 中途收回能力：已经打开的对话框也必须整块消失（不是留着旧帧）。
	SetSource(src)
	p.Open()
	if got := drawOnce(t, p, env, canvas, 1200, 800); got.DialogW != 680 {
		t.Fatalf("重新注入 source 后卡片应该回来，实际宽=%v", got.DialogW)
	}
	SetSource(nil)
	if texts := func() []string { drawOnce(t, p, env, canvas, 1200, 800); return textValues(canvas) }(); len(texts) != 0 {
		t.Fatalf("收回 source 后不该再画文字，实际=%v", texts)
	}
}

// TestHostErrorShowsOriginalTextAndKeepsCard 宿主报错时显示**原文**，卡片不关。
func TestHostErrorShowsOriginalTextAndKeepsCard(t *testing.T) {
	src := &fakeSource{
		levels: map[string]*apiclient.DirectoryListing{"": homeListing()},
		err:    &apiclient.APIError{Code: "internal", Message: "宿主炸了：目录读不出来"},
	}
	p, env, _, canvas := fixture(t, src)
	if !p.Open() {
		t.Fatal("有 source 时 Open 应该打开对话框")
	}
	got := drawOnce(t, p, env, canvas, 1200, 800)

	// 原文 = APIError.Message，不是我们拼的 "internal: …（hint）"。
	if !hasText(canvas, "宿主炸了：目录读不出来") {
		t.Fatalf("画面没有宿主错误原文；实际文字=%v", textValues(canvas))
	}
	if hasText(canvas, "internal: 宿主炸了：目录读不出来") {
		t.Fatal("错误行不该是我们拼的 code: message，官方只显示宿主那句 message")
	}
	p.mu.Lock()
	open := p.open
	p.mu.Unlock()
	if !open {
		t.Fatal("宿主报错不该关闭卡片")
	}
	if got.DialogW != 680 {
		t.Fatalf("报错时卡片=%v 宽，want 680（卡片留着）", got.DialogW)
	}
}

// TestOpenWithoutPickedCallbackStillCloses 没有回调时「打开」也要能关掉对话框
// （回调是壳的可选接线，不是对话框的前置条件）。
func TestOpenWithoutPickedCallbackStillCloses(t *testing.T) {
	src := &fakeSource{levels: map[string]*apiclient.DirectoryListing{"": homeListing()}}
	p, env, tree, canvas := fixture(t, src)
	p.Open()
	drawOnce(t, p, env, canvas, 1200, 800)
	click(t, p, env, tree, canvas, 1200, 800, "open")
	p.mu.Lock()
	open := p.open
	p.mu.Unlock()
	if open {
		t.Fatal("注册成功、没有回调时对话框也应该关闭")
	}
	if got := src.registered(); len(got) != 1 {
		t.Fatalf("Register 调用=%v，want 一次", got)
	}
}

// TestFailureTextKeepsHostMessage 钉住失败文案的取法：宿主业务错误取 message，
// 其它错误原样（**不新写中文**）。
func TestFailureTextKeepsHostMessage(t *testing.T) {
	if got := failureText(&apiclient.APIError{Code: "no_such_dir", Message: "目录不存在"}); got != "目录不存在" {
		t.Fatalf("APIError 文案=%q，want 宿主 message", got)
	}
	plain := errNoCreatedPath
	if got := failureText(plain); got != plain.Error() {
		t.Fatalf("普通错误文案=%q，want 原样 %q", got, plain.Error())
	}
	if got := failureText(nil); got != "" {
		t.Fatalf("nil 错误文案=%q，want 空", got)
	}
	if !strings.Contains(zhCreateIn, "{name}") {
		t.Fatal("createIn 模板必须保留 {name} 占位")
	}
	if got := createInText("home"); got != `在"home"中新建文件夹` {
		t.Fatalf("createIn 文案=%q，want 套模板后的官方串", got)
	}
}

// TestDisplayCrumbsStartsAtHome 拷官方 displayCrumbs（OFF:DirectoryBrowser.tsx:116-121）。
func TestDisplayCrumbsStartsAtHome(t *testing.T) {
	inside := &apiclient.DirectoryListing{
		Path: "/home/a/b",
		Home: "/home",
		Crumbs: []apiclient.DirectoryItem{
			{Name: "", Path: "/"},
			{Name: "home", Path: "/home"},
			{Name: "a", Path: "/home/a"},
			{Name: "b", Path: "/home/a/b"},
		},
	}
	crumbs := displayCrumbs(inside, zhHome)
	if len(crumbs) != 3 || crumbs[0].Name != zhHome || crumbs[0].Path != "/home" {
		t.Fatalf("home 子树的链=%v，want [主目录 a b]", crumbs)
	}
	outside := &apiclient.DirectoryListing{
		Path:   "/mnt/x",
		Home:   "/home",
		Crumbs: []apiclient.DirectoryItem{{Name: "", Path: "/"}, {Name: "mnt", Path: "/mnt"}, {Name: "x", Path: "/mnt/x"}},
	}
	if crumbs := displayCrumbs(outside, zhHome); len(crumbs) != 3 || crumbs[1].Name != "mnt" {
		t.Fatalf("home 之外的链应原样显示，实际=%v", crumbs)
	}
}

// TestLoadingAndTruncatedStatusLines 加载中/文件夹过多都是官方 `.status` 那一行
// （OFF:DirectoryBrowser.module.css:310-327，TSX 的 :952-961）。
func TestLoadingAndTruncatedStatusLines(t *testing.T) {
	listing := homeListing()
	listing.Truncated = true
	src := &fakeSource{levels: map[string]*apiclient.DirectoryListing{"": listing}}
	p, env, _, canvas := fixture(t, src)
	p.Open()
	drawOnce(t, p, env, canvas, 1200, 800)

	// 宿主说这一层被截断了 → 必须让人看见（不许静默丢掉一大截）。
	if !hasText(canvas, zhTruncated) {
		t.Fatalf("truncated 为真时应显示 %q；实际文字=%v", zhTruncated, textValues(canvas))
	}

	// 请求在飞 → 「加载中…」。产品路径靠 goroutine 结算，这里直接把状态置上，
	// 验的是**这一行确实会被画出来**（而不是等一次真实的慢盘）。
	truncatedListing := homeListing() // 未截断的下一层，免得两条状态叠在一起
	truncatedListing.Truncated = false
	src.mu.Lock()
	src.levels["/home/docs"] = truncatedListing
	src.mu.Unlock()
	p.mu.Lock()
	p.level = truncatedListing
	p.loading = true
	p.mu.Unlock()
	drawOnce(t, p, env, canvas, 1200, 800)
	if !hasText(canvas, zhLoading) {
		t.Fatalf("loading 为真时应显示 %q；实际文字=%v", zhLoading, textValues(canvas))
	}
	if hasText(canvas, zhTruncated) {
		t.Fatal("未截断的那一层不该显示「文件夹过多」")
	}
}
