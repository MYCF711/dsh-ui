package uikernel

// official_shell_test.go —— 首屏结构骨架的机器断言（本任务的验收核心）。
//
// 断言分四类：
//
//	A. 槽表自洽：名字唯一、kind/scope 合法、父槽在前、锚点非空且不是 rc.1 行号。
//	B. 结构覆盖：**官方每个槽都有对应实现节点**（SlotSpec.Host 能在骨架树里找到，
//	   且节点挂的就是那个槽），并且骨架不引入任何官方表之外的槽名。
//	C. 几何一致：官方值 ↔ 实现值同一张表逐行核对（aligned 必须相等，deviation
//	   必须写原因），且 flex 真画出来的矩形与纯布局算的一致。
//	D. rc.2 三处真变各有专门用例：
//	   ① `.crumbSubagent` 类消失  ② flash 改属性选择器  ③ headerActions.gap 8→10。

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── A. 槽表自洽 ──────────────────────────────────────────────────────

func TestOfficialSlotTableIsWellFormed(t *testing.T) {
	specs := OfficialSlots()
	if len(specs) < 50 {
		t.Fatalf("官方首屏槽表只有 %d 条，结构树至少 50 个槽；是不是漏了整块？", len(specs))
	}
	seen := map[pluginapi.Slot]int{}
	for i, spec := range specs {
		if spec.Name == "" {
			t.Fatalf("第 %d 条槽没有名字", i)
		}
		if _, dup := seen[spec.Name]; dup {
			t.Fatalf("槽 %q 重复登记", spec.Name)
		}
		for _, other := range specs[:i] {
			if other.Name == spec.Parent {
				goto parentOK
			}
		}
		if spec.Parent != "" {
			t.Fatalf("槽 %q 的父槽 %q 没有排在它前面（父槽必须先登记）", spec.Name, spec.Parent)
		}
	parentOK:
		seen[spec.Name] = i
		if spec.Host == "" {
			t.Fatalf("槽 %q 没有实现节点（Host 为空）= 官方结构树上有一个座位我们没画", spec.Name)
		}
		if !validSlotKind(spec.Kind) {
			t.Fatalf("槽 %q 的 kind %q 不认识", spec.Name, spec.Kind)
		}
		if !validSlotScope(spec.Scope) {
			t.Fatalf("槽 %q 的 scope %q 不认识", spec.Name, spec.Scope)
		}
		if spec.Anchor == "" {
			t.Fatalf("槽 %q 没有 rc.2 源码锚点", spec.Name)
		}
		if strings.Contains(spec.Anchor, "lib/client.js") {
			t.Fatalf("槽 %q 的锚点还是 rc.1 口径（lib/client.js）：%s", spec.Name, spec.Anchor)
		}
		if !strings.Contains(spec.Anchor, "/src/") {
			t.Fatalf("槽 %q 的锚点必须指 rc.2 随包 src/**：%s", spec.Name, spec.Anchor)
		}
	}

	// 七个 root children 必须全在（官方 ui-layout/src/client/index.ts:196-201）。
	must := []pluginapi.Slot{
		OffRoot, OffSidebar, OffMain, pluginapi.SlotRightbar,
		pluginapi.SlotShellBottom, pluginapi.SlotShellOverlay, pluginapi.SlotShellLeading,
	}
	for _, name := range must {
		if _, ok := LookupOfficialSlot(name); !ok {
			t.Fatalf("root 槽 %q 没进官方槽表", name)
		}
	}
}

func TestOfficialClassesAreWellFormed(t *testing.T) {
	classes := OfficialClasses()
	if len(classes) < 40 {
		t.Fatalf("官方类表只有 %d 条，首屏结构至少 40 类", len(classes))
	}
	seen := map[string]bool{}
	seenName := map[string]bool{}
	_ = seenName
	for _, c := range classes {
		if c.Name == "" || c.Module == "" {
			t.Fatalf("类表有空项：%+v", c)
		}
		// 同一个类名（如 .root）在不同 CSS Module 里合法共存，去重键要带模块。
		key := c.Module + ":" + c.Name
		if seen[key] {
			t.Fatalf("类 %q 在 %s 里重复登记", c.Name, c.Module)
		}
		seen[key] = true
		if c.Line <= 0 {
			t.Fatalf("类 %q 没有官方行号", c.Name)
		}
		if !strings.Contains(c.Module, "/src/") || !strings.HasSuffix(c.Module, ".css") {
			t.Fatalf("类 %q 的模块路径不是 rc.2 CSS Module：%s", c.Name, c.Module)
		}
	}
	for _, rc := range RemovedClasses() {
		if rc.Evidence == "" || rc.Replacement == "" {
			t.Fatalf("已删除类 %q 缺证据或替代结构", rc.Name)
		}
		for key := range seen {
			if strings.HasSuffix(key, ":"+rc.Name) {
				t.Fatalf("类 %q 既在 rc.2 类表里又在已删除表里，自相矛盾", rc.Name)
			}
		}
	}
}

// ── B. 结构覆盖：官方每个槽都有实现节点 ──────────────────────────────

func TestEveryOfficialSlotHasImplementationNode(t *testing.T) {
	layouts := map[string]ShellOptions{
		"1920x1032-windows-tabs-rightbar": {Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true, PanellistRows: 3, BottomHeight: 24},
		"1182x832-web-tabs-rightbar":      {Tabs: true, SidebarPreferred: 280, RightbarOn: true},
		"960x600-narrow-no-rightbar":      {SidebarPreferred: 280},
		"hero-blank":                      {Blank: true, Phase: PhaseHero, SidebarPreferred: 280},
	}
	for name, opts := range layouts {
		t.Run(name, func(t *testing.T) {
			tree := BuildFirstScreen(ComputeShellLayout(1920, 1032, opts))
			if name == "1182x832-web-tabs-rightbar" {
				tree = BuildFirstScreen(ComputeShellLayout(1182, 832, opts))
			}
			if name == "960x600-narrow-no-rightbar" {
				tree = BuildFirstScreen(ComputeShellLayout(960, 600, opts))
			}
			if name == "hero-blank" {
				tree = BuildFirstScreen(ComputeShellLayout(1182, 832, opts))
			}
			for _, spec := range OfficialSlots() {
				node, ok := tree.Node(spec.Host)
				if !ok {
					t.Fatalf("槽 %q 的实现节点 %q 在骨架树里不存在", spec.Name, spec.Host)
				}
				if node.Slot != spec.Name {
					t.Fatalf("节点 %q 挂的是 %q，但槽表说它应该挂 %q", spec.Host, node.Slot, spec.Name)
				}
				if hosts := tree.NodesForSlot(spec.Name); len(hosts) != 1 {
					t.Fatalf("槽 %q 有 %d 个实现节点（single 槽必须恰好一个）", spec.Name, len(hosts))
				}
			}
		})
	}
}

func TestSkeletonNodesOnlyUseOfficialSlots(t *testing.T) {
	tree := BuildFirstScreen(ComputeShellLayout(1920, 1032, ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true}))
	for _, node := range tree.Nodes() {
		if node.Slot == "" {
			continue
		}
		if _, ok := LookupOfficialSlot(node.Slot); !ok {
			t.Fatalf("骨架节点 %q 挂了官方槽表之外的槽 %q（平行体系）", node.Name, node.Slot)
		}
	}
	// 父节点必须存在（树是连通的，不是一堆孤岛）。
	for _, node := range tree.Nodes() {
		if node.Parent == "" {
			if node.Name != "AppFrame.frame" {
				t.Fatalf("节点 %q 没有父节点，又不是根", node.Name)
			}
			continue
		}
		if _, ok := tree.Node(node.Parent); !ok {
			t.Fatalf("节点 %q 的父节点 %q 不存在", node.Name, node.Parent)
		}
	}
}

// ── C. 几何：官方值 ↔ 实现值同一张表 ────────────────────────────────

func TestGeometryTableIsSingleSourceOfTruth(t *testing.T) {
	rows := OfficialGeometry()
	if len(rows) < 100 {
		t.Fatalf("几何表只有 %d 行，首屏结构容器的 gap/padding/min-max 至少 100 行", len(rows))
	}
	seen := map[string]bool{}
	for i, row := range rows {
		if row.Node == "" || row.Prop == "" {
			t.Fatalf("第 %d 行缺节点或属性名：%+v", i, row)
		}
		key := row.Node + "|" + row.Prop
		if seen[key] {
			t.Fatalf("几何行重复：%s", key)
		}
		seen[key] = true
		if row.Anchor == "" {
			t.Fatalf("%s 的 %s 没有官方锚点", row.Node, row.Prop)
		}
		if strings.Contains(row.Anchor, "lib/client.js") {
			t.Fatalf("%s 的 %s 锚点还是 rc.1 行号：%s", row.Node, row.Prop, row.Anchor)
		}
		switch row.Status {
		case GeoAligned:
			if row.Official != row.Impl {
				t.Fatalf("%s 的 %s：aligned 行官方值 %v ≠ 实现值 %v", row.Node, row.Prop, row.Official, row.Impl)
			}
		case GeoDerived:
			if row.Formula == "" {
				t.Fatalf("%s 的 %s：derived 行必须写官方公式原文", row.Node, row.Prop)
			}
		case GeoDeviation:
			if row.Reason == "" {
				t.Fatalf("%s 的 %s：deviation 行必须写清原因（只允许遗留桥）", row.Node, row.Prop)
			}
			if row.Official == row.Impl {
				t.Fatalf("%s 的 %s：deviation 行官方值与实现值相同，应标 aligned", row.Node, row.Prop)
			}
		default:
			t.Fatalf("%s 的 %s：未知状态 %q", row.Node, row.Prop, row.Status)
		}
		switch row.Unit {
		case UnitPx, UnitRatio, UnitPct, UnitZIndex:
		default:
			t.Fatalf("%s 的 %s：未知量纲 %q", row.Node, row.Prop, row.Unit)
		}
	}

	// 窗口框架/三列/header/composer 的关键几何必须都在表里（缺一行就等于漏一条官方事实）。
	required := []string{
		"AppFrame.frame|padding-top (Windows caption)",
		"columns|sidebar min", "columns|sidebar max", "columns|center min", "columns|rightbar min",
		"ConversationRoot.header|min-height (tabs)",
		"ConversationRoot.headerActions|gap",
		"ConversationRoot.composerSeat|z-index (active)",
		"InputBar.card|padding-top",
		"InputBar.primary|size",
	}
	for _, key := range required {
		if !seen[key] {
			t.Fatalf("几何表缺少关键行：%s", key)
		}
	}
}

// geometryAnchor 是官方锚点解析结果。
type geometryAnchor struct {
	pkg  string
	file string
	line int
	raw  string
}

var anchorLineRE = regexp.MustCompile(`:(\d+)`)

// parseOfficialAnchor 把 `ui-layout/src/client/index.ts:196` 拆成包 + 文件 + 行号。
// 行号段允许 `:192-206` / `:28,36` 两种写法，取第一个数字。
func parseOfficialAnchor(t *testing.T, raw string) geometryAnchor {
	t.Helper()
	idx := strings.Index(raw, ":/")
	_ = idx
	sep := strings.LastIndex(raw, ":")
	if sep <= 0 || sep == len(raw)-1 {
		t.Fatalf("锚点格式不对（缺 :行号）：%s", raw)
	}
	path, spec := raw[:sep], raw[sep+1:]
	m := anchorLineRE.FindStringSubmatch(":" + spec)
	if m == nil {
		t.Fatalf("锚点没有行号：%s", raw)
	}
	line, err := strconv.Atoi(m[1])
	if err != nil || line <= 0 {
		t.Fatalf("锚点行号非法：%s", raw)
	}
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		t.Fatalf("锚点缺包路径：%s", raw)
	}
	return geometryAnchor{pkg: parts[0], file: path, line: line, raw: raw}
}

// TestGeometryAnchorsPointAtRC2Source 真的去 rc.2 解包树里核每个锚点：
// 文件存在、行号不越界、该行不是空行。官方树不在本机时跳过（不是通过）。
func TestGeometryAnchorsPointAtRC2Source(t *testing.T) {
	root := os.Getenv("DSH_OFFICIAL_TREE")
	if root == "" {
		root = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai"
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		t.Skipf("官方 rc.2 解包树不在本机（%s），跳过锚点核验；设 DSH_OFFICIAL_TREE 可指过去", root)
	}

	check := func(raw string) {
		a := parseOfficialAnchor(t, raw)
		// `ui-layout/...` → `@deepseek-ai/dsh-client-ui-layout/...`
		dir := filepath.Join(root, "dsh-client-"+a.pkg, filepath.FromSlash(strings.TrimPrefix(a.file, a.pkg+"/")))
		data, err := os.ReadFile(dir)
		if err != nil {
			t.Fatalf("锚点 %s 指的文件不存在：%v", raw, err)
		}
		lines := strings.Split(string(data), "\n")
		if a.line > len(lines) {
			t.Fatalf("锚点 %s 行号越界（文件只有 %d 行）", raw, len(lines))
		}
		if strings.TrimSpace(lines[a.line-1]) == "" {
			t.Fatalf("锚点 %s 指向空行", raw)
		}
	}
	for _, row := range OfficialGeometry() {
		check(row.Anchor)
	}
	for _, spec := range OfficialSlots() {
		check(spec.Anchor)
	}
	for _, c := range OfficialClasses() {
		check(c.Module + ":" + strconv.Itoa(c.Line))
	}
}

// TestGeometryTableDrivesLayout 校验「表里的数字 = 代码里真的用的数字」：
// 关键几何必须能由布局函数复现，否则表就是一张没人用的文档。
func TestGeometryTableDrivesLayout(t *testing.T) {
	layout := ComputeShellLayout(1920, 1032, ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true, PanellistRows: 3})
	if layout.Caption != CaptionHeight {
		t.Fatalf("Windows 标题栏 = %v，表里是 %v", layout.Caption, CaptionHeight)
	}
	if layout.HeaderH != HeaderMinHeightTabs {
		t.Fatalf("有 tabs 的 header 高度 = %v，表里是 %v", layout.HeaderH, HeaderMinHeightTabs)
	}
	if layout.TitleRow.Height != TitleRowMinHeight {
		t.Fatalf("titleRow 高度 = %v，表里是 %v", layout.TitleRow.Height, TitleRowMinHeight)
	}
	if layout.PanelList.Height != 3*SidebarPanelRowMinH+2*SidebarPanelListGap+SidebarPanelListBottom {
		t.Fatalf("三层导航列表高度 = %v，按表应为 3*32+2*2+8=108", layout.PanelList.Height)
	}
	if layout.ComposerSeat.Height != ComposerSeatHeight {
		t.Fatalf("composer 座高度 = %v，表里是 %v", layout.ComposerSeat.Height, ComposerSeatHeight)
	}
	// composer 座 = 卡片(8+36+12+42=98) + dock(4+22=26) + 底边距 4 = 128。
	//
	// 控件行是 42 不是 36：官方 `.row` 没有 height，被最高的子元素撑开，而主按钮
	// 是 34（`.primary` 的 translateY(-2px) 只挪视觉、不参与布局），
	// 所以行高 = 2+34+6。按 trigger 的 28 算出来的 36/92/122 与真渲染差 6px
	// （dscomposer 的卡片一直是 98）—— 两个数字对不上就是这一条。
	if ComposerSeatHeight != 128 {
		t.Fatalf("composer 座高度常量 = %v，官方三块相加应为 128", ComposerSeatHeight)
	}
}

// ── D. rc.2 三处真变：各一条专门用例 ────────────────────────────────

// TestRC2HeaderActionsGapIsTen 覆盖真变③：`.headerActions` gap 8px → 10px。
func TestRC2HeaderActionsGapIsTen(t *testing.T) {
	const rc1Gap = 8
	if HeaderActionsGap != 10 {
		t.Fatalf("rc.2 的 .headerActions gap 是 10px，实现值是 %v", HeaderActionsGap)
	}
	if HeaderActionsGap == rc1Gap {
		t.Fatalf("实现又把 gap 写回 rc.1 的 8px 了")
	}
	if HeaderActionsGapNarrow != 8 {
		t.Fatalf("窄容器覆盖仍是 8px（rc.2 新增 @container max-width 460px），实现值是 %v", HeaderActionsGapNarrow)
	}
	if HeaderActionsNarrowMax != 460 {
		t.Fatalf("窄容器断点是 460px，实现值是 %v", HeaderActionsNarrowMax)
	}
	var found, narrow bool
	for _, row := range OfficialGeometry() {
		if row.Node != "ConversationRoot.headerActions" {
			continue
		}
		if row.Prop == "gap" {
			found = true
			if row.Official != 10 || row.Impl != 10 || row.Status != GeoAligned {
				t.Fatalf("几何表里 headerActions.gap = 官方 %v / 实现 %v / %s，应为 10/10/aligned", row.Official, row.Impl, row.Status)
			}
			if !strings.HasSuffix(row.Anchor, ":175") {
				t.Fatalf("headerActions.gap 的锚点应指 rc.2 ConversationRoot.module.css:175，实际 %s", row.Anchor)
			}
		}
		if row.Prop == "gap (@container <=460px)" {
			narrow = true
			if row.Official != 8 || row.Impl != 8 {
				t.Fatalf("窄容器 gap 应为 8/8，实际 %v/%v", row.Official, row.Impl)
			}
			if !strings.Contains(row.Anchor, ":178") {
				t.Fatalf("窄容器 gap 的锚点应指 :178-181，实际 %s", row.Anchor)
			}
		}
	}
	if !found || !narrow {
		t.Fatalf("几何表缺 headerActions 的 gap 行（found=%v narrow=%v）", found, narrow)
	}
}

// TestRC2FlashUsesDataAttribute 覆盖真变②：`.flashEven/.flashOdd` 类 → 属性选择器。
func TestRC2FlashUsesDataAttribute(t *testing.T) {
	if SidebarRightFlashAttr != "data-sidebar-right-flash" {
		t.Fatalf("rc.2 的闪烁选择器属性是 data-sidebar-right-flash，实现值是 %q", SidebarRightFlashAttr)
	}
	if SidebarRightFlashOdd != "odd" || SidebarRightFlashEven != "even" {
		t.Fatalf("rc.2 的属性取值是 'odd'/'even'，实现值 %q/%q", SidebarRightFlashOdd, SidebarRightFlashEven)
	}

	removed := map[string]RemovedClass{}
	for _, rc := range RemovedClasses() {
		removed[rc.Name] = rc
	}
	for _, name := range []string{"flashEven", "flashOdd"} {
		rc, ok := removed[name]
		if !ok {
			t.Fatalf("rc.2 已把 .%s 从类改成属性选择器，但实现没登记这条删除", name)
		}
		if !rc.WasRC1 {
			t.Fatalf(".%s 是 rc.1→rc.2 删掉的，WasRC1 应为 true", name)
		}
		if !strings.Contains(rc.Evidence, "data-sidebar-right-flash") {
			t.Fatalf(".%s 的删除证据里应写明 rc.2 用的是属性选择器", name)
		}
	}
	for _, c := range OfficialClasses() {
		if c.Name == "flashEven" || c.Name == "flashOdd" {
			t.Fatalf("rc.2 已经没有 .%s 类，类表不许把它登记回来", c.Name)
		}
	}
	// ring / near / far 三个百分比在 rc.2 没变（只是选择器形态变了）。
	percent := map[string]float32{}
	for _, row := range OfficialGeometry() {
		if row.Node == "SidebarRight.flash" {
			percent[row.Prop] = row.Official
		}
	}
	if percent["ring mix"] != 48 || percent["near mix"] != 16 || percent["far mix"] != 10 {
		t.Fatalf("闪烁三个混色比例应为 48/16/10，实际 %v/%v/%v", percent["ring mix"], percent["near mix"], percent["far mix"])
	}
}

// TestRC2CrumbSubagentClassIsGone 覆盖真变①：`.crumbSubagent` 类消失。
func TestRC2CrumbSubagentClassIsGone(t *testing.T) {
	var removed *RemovedClass
	for i := range RemovedClasses() {
		if RemovedClasses()[i].Name == "crumbSubagent" {
			rc := RemovedClasses()[i]
			removed = &rc
		}
	}
	if removed == nil {
		t.Fatal("rc.2 已删除 .crumbSubagent 类，实现没登记这条删除")
	}
	if !removed.WasRC1 {
		t.Fatal(".crumbSubagent 是 rc.1→rc.2 删掉的，WasRC1 应为 true")
	}
	if !strings.Contains(removed.Replacement, "lineage") {
		t.Fatalf(".crumbSubagent 的替代结构应是 lineage 槽，实际 %q", removed.Replacement)
	}
	for _, c := range OfficialClasses() {
		if c.Name == "crumbSubagent" {
			t.Fatal("类表不许登记 rc.2 已删除的 .crumbSubagent")
		}
	}
	// 槽面上也不许再出现 subagent 面包屑槽：rc.2 只有 lineage + 当前层级。
	for _, spec := range OfficialSlots() {
		if strings.Contains(string(spec.Name), "subagent") {
			t.Fatalf("官方槽表里出现 subagent 相关槽 %q（rc.2 无此类）", spec.Name)
		}
	}
	// 骨架的标题簇只有 lineage / actions / utilities / corner 四个座位。
	tree := BuildFirstScreen(ComputeShellLayout(1920, 1032, ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true}))
	for _, name := range []string{"ConversationHeader.titleCluster", "ConversationHeader.lineage", "ConversationHeader.actions", "ConversationHeader.utilities", "ConversationHeader.corner"} {
		if _, ok := tree.Node(name); !ok {
			t.Fatalf("标题簇缺节点 %q", name)
		}
	}
	for _, node := range tree.Nodes() {
		if strings.Contains(strings.ToLower(node.Name), "subagent") {
			t.Fatalf("骨架节点 %q 用了 rc.2 已删除的 subagent 结构", node.Name)
		}
	}
}

// ── 三列求解：官方 computeColumns 的对拍向量 ─────────────────────────

func TestSolveColumnsMatchesOfficial(t *testing.T) {
	cases := []struct {
		name                                  string
		viewport, sidebar, rightbar, collapse float32
		centered                              bool
		want                                  Columns
	}{
		{name: "1920 默认 280 + 右栏 45%", viewport: 1920, sidebar: 280, rightbar: 864, collapse: 56, want: Columns{Sidebar: 280, Center: 776, Rightbar: 864}},
		{name: "1920 右栏偏好空 → 0（不出现）", viewport: 1920, sidebar: 280, rightbar: 0, collapse: 56, want: Columns{Sidebar: 280, Center: 1640}},
		{name: "1182 右栏吃掉剩余（中栏恰好 400）", viewport: 1182, sidebar: 280, rightbar: 532, collapse: 56, want: Columns{Sidebar: 280, Center: 400, Rightbar: 502}},
		{name: "960 放不下右栏 → 整列消失", viewport: 960, sidebar: 280, rightbar: 300, collapse: 56, want: Columns{Sidebar: 280, Center: 680}},
		{name: "左栏 0 → 用收缩轨道 56", viewport: 1920, sidebar: 0, rightbar: 0, collapse: 56, want: Columns{Sidebar: 56, Center: 1864}},
		{name: "左栏 0 且轨道 0（Windows/macOS）", viewport: 1920, sidebar: 0, rightbar: 0, collapse: 0, want: Columns{Sidebar: 0, Center: 1920}},
		{name: "左栏小于下限 → 264", viewport: 1920, sidebar: 100, rightbar: 0, collapse: 56, want: Columns{Sidebar: 264, Center: 1656}},
		{name: "左栏大于上限 → 420", viewport: 1920, sidebar: 900, rightbar: 0, collapse: 56, want: Columns{Sidebar: 420, Center: 1500}},
		{name: "右栏超上限 → 先夹 70% 再让中栏下限", viewport: 1920, sidebar: 280, rightbar: 2000, collapse: 56, want: Columns{Sidebar: 280, Center: 400, Rightbar: 1240}},
		{name: "centeredSidebar → 左栏落中点", viewport: 1920, sidebar: 300, rightbar: 0, collapse: 56, centered: true, want: Columns{Sidebar: 960, Center: 960}},
		{name: "viewport 0", viewport: 0, want: Columns{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SolveColumns(tc.viewport, tc.sidebar, tc.rightbar, tc.collapse, tc.centered)
			if got != tc.want {
				t.Fatalf("SolveColumns(%v,%v,%v,%v,%v) = %+v，期望 %+v",
					tc.viewport, tc.sidebar, tc.rightbar, tc.collapse, tc.centered, got, tc.want)
			}
		})
	}
}

// ── 布局：两个官方验收尺寸的逐容器矩形 ──────────────────────────────

func assertRect(t *testing.T, name string, got, want ui.Rect) {
	t.Helper()
	const eps = 0.01
	if absOff(got.X-want.X) > eps || absOff(got.Y-want.Y) > eps ||
		absOff(got.Width-want.Width) > eps || absOff(got.Height-want.Height) > eps {
		t.Fatalf("%s 矩形 = %+v，期望 %+v", name, got, want)
	}
}

func absOff(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func TestFirstScreenGeometryAtOfficialSizes(t *testing.T) {
	// 1920×1032：PLAN §U1.5 的第一个证据尺寸；Windows 有 40px caption。
	t.Run("1920x1032", func(t *testing.T) {
		l := ComputeShellLayout(1920, 1032, ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true, PanellistRows: 3})
		assertRect(t, "frame", l.Frame, ui.Rect{Width: 1920, Height: 1032})
		assertRect(t, "sidebarCol", l.SidebarCol, ui.Rect{Y: 40, Width: 280, Height: 992})
		assertRect(t, "centerCol", l.CenterCol, ui.Rect{X: 280, Y: 40, Width: 776, Height: 992})
		assertRect(t, "rightbarCol", l.RightbarCol, ui.Rect{X: 1056, Y: 40, Width: 864, Height: 992})
		// 左栏：padding 6/8 → 内容 264 宽；logoRow 40（Windows）。
		assertRect(t, "SidebarRoot.root", l.SidebarRoot, ui.Rect{X: 8, Y: 46, Width: 264, Height: 980})
		assertRect(t, "SidebarRoot.logoRow", l.SidebarLogoRow, ui.Rect{X: 8, Y: 46, Width: 264, Height: 40})
		assertRect(t, "SidebarRoot.newSession", l.NewSession, ui.Rect{X: 12, Y: 86, Width: 256, Height: 38})
		assertRect(t, "SidebarRoot.panelList", l.PanelList, ui.Rect{X: 8, Y: 136, Width: 264, Height: 108})
		// header 76 = 10 + 30 + 10 + 25 + 1 hairline；titleRow 30；tabs 25。
		assertRect(t, "header", l.Header, ui.Rect{X: 280, Y: 40, Width: 776, Height: 76})
		assertRect(t, "titleRow", l.TitleRow, ui.Rect{X: 300, Y: 50, Width: 728, Height: 30})
		assertRect(t, "tabsRow", l.TabsRow, ui.Rect{X: 308, Y: 90, Width: 720, Height: 25})
		assertRect(t, "body", l.Body, ui.Rect{X: 280, Y: 116, Width: 776, Height: 916})
		assertRect(t, "scrollBody", l.ScrollBody, ui.Rect{X: 280, Y: 116, Width: 776, Height: 788})
		// 正文列：clamp(680, 776*.64=496.6, 920) = 680，滚动区两侧 16+16 → 710 可用，居中。
		assertRect(t, "column", l.Column, ui.Rect{X: 327, Y: 132, Width: 680, Height: 756})
		assertRect(t, "composerSeat", l.ComposerSeat, ui.Rect{X: 280, Y: 904, Width: 776, Height: 128})
		assertRect(t, "composerRoot", l.ComposerRoot, ui.Rect{X: 296, Y: 904, Width: 744, Height: 128})
		assertRect(t, "composerCard", l.ComposerCard, ui.Rect{X: 312, Y: 904, Width: 712, Height: 98})
		assertRect(t, "composerRow", l.ComposerRow, ui.Rect{X: 312, Y: 960, Width: 712, Height: 42})
		assertRect(t, "composerDock", l.ComposerDock, ui.Rect{X: 296, Y: 1002, Width: 744, Height: 26})
		assertRect(t, "overlay", l.Overlay, ui.Rect{Width: 1920, Height: 1032})
		if l.LeadingSeat.X != 88 || l.LeadingSeat.Y != 11 {
			t.Fatalf("leadingSeat = (%v,%v)，官方是 (88,11)", l.LeadingSeat.X, l.LeadingSeat.Y)
		}
	})

	// 1182×832：PLAN §U1.5 的第二个证据尺寸；Web 无 caption，中栏恰好 400。
	t.Run("1182x832", func(t *testing.T) {
		l := ComputeShellLayout(1182, 832, ShellOptions{Tabs: true, SidebarPreferred: 280, RightbarOn: true})
		assertRect(t, "sidebarCol", l.SidebarCol, ui.Rect{Width: 280, Height: 832})
		assertRect(t, "centerCol", l.CenterCol, ui.Rect{X: 280, Width: 400, Height: 832})
		assertRect(t, "rightbarCol", l.RightbarCol, ui.Rect{X: 680, Width: 502, Height: 832})
		assertRect(t, "titleRow", l.TitleRow, ui.Rect{X: 300, Y: 10, Width: 352, Height: 30})
		// 正文列被可用宽度夹住：710 → 398-2-64=332? 实际 innerW=398-2-64=332。
		assertRect(t, "column", l.Column, ui.Rect{X: 312, Y: 92, Width: 334, Height: 596})
		// 主栏 400 时 composer 卡片宽 = clamp(680,…)=680 + 32 = 712 → 被可用宽夹到 368。
		assertRect(t, "composerCard", l.ComposerCard, ui.Rect{X: 296, Y: 704, Width: 368, Height: 98})
	})
}

func TestBottomRowReservesNoSpaceWhenEmpty(t *testing.T) {
	empty := ComputeShellLayout(1920, 1032, ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280})
	if empty.BottomRow.Height != 0 {
		t.Fatalf("shell.bottom 空内容不预留高度，实际 %v", empty.BottomRow.Height)
	}
	if empty.BodyRow.Height != 992 {
		t.Fatalf("没有 bottom 时三列应有全部 992 高，实际 %v", empty.BodyRow.Height)
	}
	with := ComputeShellLayout(1920, 1032, ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true, BottomHeight: 24})
	assertRect(t, "bottomRow", with.BottomRow, ui.Rect{Y: 1008, Width: 1920, Height: 24})
	assertRect(t, "centerCol", with.CenterCol, ui.Rect{X: 280, Y: 40, Width: 776, Height: 968})
}

func TestCaptionDeductedOnce(t *testing.T) {
	win := ComputeShellLayout(1920, 1032, ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true})
	if win.Content.Y != 40 {
		t.Fatalf("Windows caption 只由 L0 扣一次；内容区 y = %v，应为 40", win.Content.Y)
	}
	if win.Content.Height != 1032-40 {
		t.Fatalf("内容区高度 = %v，应为 1032-40", win.Content.Height)
	}
	if win.SidebarCol.Y != 40 || win.CenterCol.Y != 40 || win.RightbarCol.Y != 40 {
		t.Fatalf("三列的 y 都必须是 40（各列不再重复扣标题栏）：%v/%v/%v", win.SidebarCol.Y, win.CenterCol.Y, win.RightbarCol.Y)
	}
	// overlay/leadingSeat 是相对窗口的绝对层，不吃 caption 偏移。
	if win.Overlay.Y != 0 || win.LeadingSeat.Y != 11 {
		t.Fatalf("overlay.y=%v leadingSeat.y=%v，两个绝对层不该被 caption 推下去", win.Overlay.Y, win.LeadingSeat.Y)
	}
	web := ComputeShellLayout(1182, 832, ShellOptions{Tabs: true, SidebarPreferred: 280})
	if web.Content.Y != 0 {
		t.Fatalf("Web 没有 caption，内容区 y 应为 0，实际 %v", web.Content.Y)
	}
}

// ── 绘制：flex 真画出来的矩形必须等于布局算的 ───────────────────────

func newSkeletonEnv(width, height float32, has func(pluginapi.Slot) bool) (*pluginapi.Env, *gfx.Canvas) {
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	return &pluginapi.Env{
		Tree:   tree,
		Feat:   tree.Feature("official-shell-test"),
		Canvas: canvas,
		Bounds: ui.Rect{Width: width, Height: height},
		Has:    has,
	}, canvas
}

// TestFirstScreenDrawMatchesLayout 把骨架真的布出来，逐个容器核对「布局函数算的」
// 与「flex 画出来的」是同一份几何——防止表/布局/绘制三处各算一遍。
func TestFirstScreenDrawMatchesLayout(t *testing.T) {
	theme := dscore.Dark()
	env, canvas := newSkeletonEnv(1920, 1032, func(pluginapi.Slot) bool { return false })
	opts := ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true, PanellistRows: 3}

	boxes := map[string]ui.Box{}
	var layout ShellLayout
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f ui.C) {
		layout = FirstScreenWithMeasure(f, env, opts, func(pluginapi.Slot) {}, func(name string, box ui.Box) {
			boxes[name] = box
		})
	})

	want := map[string]ui.Rect{
		"AppFrame.frame":                layout.Frame,
		"AppFrame.sidebarCol":           layout.SidebarCol,
		"AppFrame.centerCol":            layout.CenterCol,
		"AppFrame.rightbarCol":          layout.RightbarCol,
		"ConversationRoot.header":       layout.Header,
		"ConversationRoot.body":         layout.Body,
		"ConversationRoot.scrollBody":   layout.ScrollBody,
		"ConversationRoot.composerSeat": layout.ComposerSeat,
		"InputBar.card":                 layout.ComposerCard,
	}
	for name, rect := range want {
		box, ok := boxes[name]
		if !ok {
			t.Fatalf("测点 %q 没有被画出来（FirstScreenWithMeasure 没上报）", name)
		}
		assertRect(t, "drawn "+name, box.Rect(), rect)
	}
}

// TestFirstScreenDispatchesEveryHostedSlot 断言「结构表里的每个官方槽，绘制路径都会
// dispatch 一次」——不是所有槽都在同一帧画（hero 与 transcript 互斥），所以合并三帧。
func TestFirstScreenDispatchesEveryHostedSlot(t *testing.T) {
	theme := dscore.Dark()
	record := func(width, height float32, opts ShellOptions, has func(pluginapi.Slot) bool) map[pluginapi.Slot]bool {
		env, canvas := newSkeletonEnv(width, height, has)
		seen := map[pluginapi.Slot]bool{}
		var view ui.Page
		view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f ui.C) {
			FirstScreen(f, env, opts, func(slot pluginapi.Slot) { seen[slot] = true })
		})
		return seen
	}

	active := record(1920, 1032, ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true, PanellistRows: 3, BottomHeight: 24}, func(pluginapi.Slot) bool { return false })
	hero := record(1182, 832, ShellOptions{Blank: true, Phase: PhaseHero, SidebarPreferred: 280, RightbarOn: true}, func(pluginapi.Slot) bool { return false })
	frame := record(1920, 1032, ShellOptions{Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true}, func(slot pluginapi.Slot) bool {
		return slot == pluginapi.SlotShellOverlay || slot == pluginapi.SlotShellLeading
	})

	union := map[pluginapi.Slot]bool{}
	for _, set := range []map[pluginapi.Slot]bool{active, hero, frame} {
		for slot := range set {
			union[slot] = true
		}
	}
	for _, spec := range OfficialSlots() {
		if !union[spec.Name] {
			t.Fatalf("官方槽 %q（实现节点 %q）在三帧里都没有被 dispatch = 挂上去也不会显示", spec.Name, spec.Host)
		}
	}
	// 不许 dispatch 官方表之外的槽（那就是平行体系）。
	for slot := range union {
		if _, ok := LookupOfficialSlot(slot); !ok {
			t.Fatalf("绘制路径 dispatch 了官方槽表之外的槽 %q", slot)
		}
	}
	if got, want := len(union), len(OfficialSlots()); got != want {
		t.Fatalf("三帧合计 dispatch 了 %d 个槽，官方槽表是 %d 个", got, want)
	}
}

// ── RegisterOfficialSlot：后续任务（rc.2 新插件）扩槽的入口 ──────────

func TestRegisterOfficialSlotIsFailLoud(t *testing.T) {
	cases := []struct {
		name string
		spec SlotSpec
		want string
	}{
		{"没有名字", SlotSpec{Kind: KindSingle, Scope: ScopeRoot, Anchor: "ui-layout/src/client/index.ts:1"}, "没有名字"},
		{"没有锚点", SlotSpec{Name: "test.no.anchor", Kind: KindSingle, Scope: ScopeRoot}, "没有 rc.2 源码锚点"},
		{"kind 非法", SlotSpec{Name: "test.bad.kind", Kind: "gizmo", Scope: ScopeRoot, Anchor: "ui-layout/src/client/index.ts:1"}, "kind"},
		{"scope 非法", SlotSpec{Name: "test.bad.scope", Kind: KindSingle, Scope: "galaxy", Anchor: "ui-layout/src/client/index.ts:1"}, "scope"},
		{"父槽没登记", SlotSpec{Name: "test.orphan", Kind: KindSingle, Scope: ScopeRoot, Parent: "nope.not.registered", Anchor: "ui-layout/src/client/index.ts:1"}, "父槽"},
		{"重复登记", SlotSpec{Name: OffSidebar, Kind: KindSingle, Scope: ScopeRoot, Anchor: "ui-layout/src/client/index.ts:196"}, "重复"},
	}
	for _, tc := range cases {
		err := RegisterOfficialSlot(tc.spec)
		if err == nil {
			t.Fatalf("%s：期望报错，实际成功", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s：错误信息里应包含 %q，实际 %q", tc.name, tc.want, err.Error())
		}
	}

	// 合法登记：父槽是已登记的 root 槽。
	spec := SlotSpec{
		Name: "test.rc2.new.slot", Kind: KindSingle, Scope: ScopeRoot, Parent: OffRoot,
		Host: "AppFrame.frame", Anchor: "ui-layout/src/client/index.ts:201",
	}
	if err := RegisterOfficialSlot(spec); err != nil {
		t.Fatalf("合法槽登记失败：%v", err)
	}
	if _, ok := LookupOfficialSlot("test.rc2.new.slot"); !ok {
		t.Fatal("登记成功后 Lookup 找不到该槽")
	}
	// 重复再登一次必须红（否则同槽会被挂两份）。
	if err := RegisterOfficialSlot(spec); err == nil {
		t.Fatal("重复登记同一个槽必须报错")
	}
}
