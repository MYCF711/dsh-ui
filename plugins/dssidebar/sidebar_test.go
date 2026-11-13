// sidebar_test.go —— 侧栏里那些"纯判断"的单测。
//
// 为什么这些要测：侧栏的错都很"远期"——排序错了用户只会觉得"会话跑到中间去了"，
// 过滤词大小写不对只会觉得"搜不到"，折叠状态错只会觉得"点了没反应"。这些都不是
// 崩溃，靠看代码很难再发现一遍，所以钉在这里。
package dssidebar

import (
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/gfx"
)

func sess(id, title, cwd, at string) apiclient.Session {
	return apiclient.Session{ID: id, Title: title, Cwd: cwd, UpdatedAt: at}
}

// TestGroupByWorkspaceFilter 覆盖搜索框真正会走的那条路：标题命中、cwd 命中、
// 大小写不敏感、首尾空白被裁掉。
func TestGroupByWorkspaceFilter(t *testing.T) {
	sessions := []apiclient.Session{
		sess("a", "Fix sidebar rows", `D:\work\alpha`, "2026-11-03T10:00:00Z"),
		sess("b", "讨论官方规格", `D:\work\beta`, "2026-11-03T11:00:00Z"),
		sess("c", "notes", `D:\work\alpha`, "2026-11-03T12:00:00Z"),
	}

	// 1) 标题命中（大小写与空白都不该影响）
	groups, order := groupByWorkspace(sessions, normalizeQuery("  SIDEBAR  "))
	if len(order) != 1 || order[0] != `D:\work\alpha` {
		t.Fatalf("标题过滤：期望只有 alpha 组，得到 %v", order)
	}
	if len(groups[`D:\work\alpha`]) != 1 || groups[`D:\work\alpha`][0].ID != "a" {
		t.Fatalf("标题过滤：期望只剩 a，得到 %v", groups[`D:\work\alpha`])
	}

	// 2) cwd 命中：alpha 两条都该在
	groups, order = groupByWorkspace(sessions, normalizeQuery(`WORK\ALPHA`))
	if len(order) != 1 || len(groups[`D:\work\alpha`]) != 2 {
		t.Fatalf("cwd 过滤：期望 alpha 组两条，得到 order=%v groups=%v", order, groups)
	}

	// 3) 没命中：组和顺序都应为空，而不是整张列表
	groups, order = groupByWorkspace(sessions, normalizeQuery("zzz-nope"))
	if len(order) != 0 || len(groups) != 0 {
		t.Fatalf("无命中：期望空列表，得到 %v", groups)
	}

	// 4) 空过滤词 = 全都要
	_, order = groupByWorkspace(sessions, "")
	if len(order) != 2 {
		t.Fatalf("空过滤词：期望两个组，得到 %v", order)
	}
}

// TestGroupByWorkspaceOrder：组内从新到旧、组间按各组最新一条从新到旧。
//
// 这是 2026-11 补的行为：官方侧栏按 updatedAt 排，宿主返回顺序不保证。
func TestGroupByWorkspaceOrder(t *testing.T) {
	sessions := []apiclient.Session{
		sess("old", "旧", `D:\w\a`, "2026-11-01T08:00:00Z"),
		sess("newest", "最新", `D:\w\a`, "2026-11-03T20:00:00Z"),
		sess("mid", "中间", `D:\w\a`, "2026-11-02T09:00:00Z"),
		sess("b1", "B 组唯一", `D:\w\b`, "2026-11-03T12:00:00Z"),
		sess("c1", "C 组唯一", `D:\w\c`, "2026-10-30T12:00:00Z"),
	}
	groups, order := groupByWorkspace(sessions, "")

	// 组间：a（最新 11-03 20:00）→ b（11-03 12:00）→ c（10-30）
	wantOrder := []string{`D:\w\a`, `D:\w\b`, `D:\w\c`}
	if len(order) != len(wantOrder) {
		t.Fatalf("组顺序数量不对：%v", order)
	}
	for i := range wantOrder {
		if order[i] != wantOrder[i] {
			t.Fatalf("组顺序 = %v，期望 %v", order, wantOrder)
		}
	}

	// 组内：newest → mid → old
	wantRows := []string{"newest", "mid", "old"}
	for i, s := range groups[`D:\w\a`] {
		if s.ID != wantRows[i] {
			t.Fatalf("组内顺序 = %v，期望 %v", groups[`D:\w\a`], wantRows)
		}
	}
}

// TestGroupByWorkspaceOrderFallback：updatedAt 解析不了时不许 panic，也不许
// 把整组按 1970 排——退回字符串比较，保持稳定。
func TestGroupByWorkspaceOrderFallback(t *testing.T) {
	sessions := []apiclient.Session{
		sess("x", "x", `D:\w`, "not-a-time"),
		sess("y", "y", `D:\w`, "2026-11-03T00:00:00Z"),
		sess("z", "z", `D:\w`, ""),
	}
	groups, order := groupByWorkspace(sessions, "")
	if len(order) != 1 || len(groups[`D:\w`]) != 3 {
		t.Fatalf("解析失败时丢了行：order=%v groups=%v", order, groups)
	}
}

// TestWorkspaceKeyAndDisplay：分组键去尾部分隔符；显示名只取最后一段。
func TestWorkspaceKeyAndDisplay(t *testing.T) {
	cases := []struct{ in, key, name string }{
		{`D:\work\alpha\`, `D:\work\alpha`, "alpha"},
		{`D:\work\alpha`, `D:\work\alpha`, "alpha"},
		{`/home/me/proj/`, `/home/me/proj`, "proj"},
		{"", "", "未指定工作区"},
		{"   ", "", "未指定工作区"},
	}
	for _, c := range cases {
		if got := workspaceKey(c.in); got != c.key {
			t.Errorf("workspaceKey(%q) = %q，期望 %q", c.in, got, c.key)
		}
		if got := displayWorkspace(c.key); got != c.name {
			t.Errorf("displayWorkspace(%q) = %q，期望 %q", c.key, got, c.name)
		}
	}
}

// TestRelativeTime：时间列的每一档；无法确认真值时整段隐藏。
func TestRelativeTime(t *testing.T) {
	// 用相对 now 造数据，避免固定时间在几个月后变成"01-02"档
	now := time.Now()
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{now.Add(-10 * time.Second).Format(time.RFC3339), "刚刚"},
		{now.Add(-5 * time.Minute).Format(time.RFC3339), "5 分钟前"},
		{now.Add(-3 * time.Hour).Format(time.RFC3339), "3 小时前"},
		{now.Add(-30 * time.Hour).Format(time.RFC3339), "昨天"},
		{now.Add(-4 * 24 * time.Hour).Format(time.RFC3339), "4 天前"},
		{now.Add(-40 * 24 * time.Hour).Format(time.RFC3339), now.Add(-40 * 24 * time.Hour).Format("01-02")},
		{"2026-11-03 10:00:00", ""}, // 非 RFC3339：不截片段冒充真时间
		{"2026-11-03", ""},
	}
	for _, c := range cases {
		if got := relativeTime(c.in); got != c.want {
			t.Errorf("relativeTime(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

// TestHandleKeyNavigation：↑↓/Home/End 在**当前可见顺序**里移动，并把选中的
// 会话交给壳的 OpenSession。越界、带修饰键都不该接管。
func TestHandleKeyNavigation(t *testing.T) {
	var opened []string
	var p Plugin
	p.act.OpenSession = func(id string) { opened = append(opened, id) }
	p.lastOrder = []string{"a", "b", "c"}
	p.lastCur = "b"

	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown}) {
		t.Fatal("↓ 应该被接管")
	}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyUp}) {
		t.Fatal("↑ 应该被接管")
	}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyHome}) {
		t.Fatal("Home 应该被接管")
	}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEnd}) {
		t.Fatal("End 应该被接管")
	}
	want := []string{"c", "a", "a", "c"}
	if len(opened) != len(want) {
		t.Fatalf("打开序列 = %v，期望 %v", opened, want)
	}
	for i := range want {
		if opened[i] != want[i] {
			t.Fatalf("打开序列 = %v，期望 %v", opened, want)
		}
	}

	// 当前会话不在可见列表里（比如被过滤掉了）：不猜，直接不管
	p.lastCur = "zzz"
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown}) {
		t.Fatal("找不到当前会话时不该假装切了")
	}

	// 带修饰键：留给别的插件
	p.lastCur = "a"
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown, Modifiers: gfx.ModifierControl}) {
		t.Fatal("带修饰键不该被侧栏吃掉")
	}

	// Esc 之类不归侧栏
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("Esc 不该被侧栏吃掉")
	}
}

// TestCollapseState：三处状态合成（每组 + 全局）以及"点组头"、"全部收起"。
func TestCollapseState(t *testing.T) {
	p := Plugin{collapsed: map[string]bool{}, known: []string{"a", "b"}}

	if !p.groupOpen("a") {
		t.Fatal("初始应全部展开")
	}
	p.toggleGroup("a") // 展开 → 收起
	if p.groupOpen("a") || p.groupOpen("b") == false {
		t.Fatalf("只该收起 a：a=%v b=%v", p.groupOpen("a"), p.groupOpen("b"))
	}
	p.toggleGroup("a") // 收起 → 展开
	if !p.groupOpen("a") {
		t.Fatal("再点一次应该展开")
	}

	// 全部收起：所有组都不可见
	p.toggleAll()
	if p.groupOpen("a") || p.groupOpen("b") {
		t.Fatal("全部收起后不该有组是展开的")
	}

	// 全局收起时点某一组：这一下就该展开它，而不是"先退出全局再点一次"
	p.toggleGroup("b")
	if !p.groupOpen("b") {
		t.Fatal("全局收起时点组头应该直接展开该组")
	}
	if p.allCollapsed {
		t.Fatal("点组头应该退出全局收起")
	}
	if p.groupOpen("a") {
		t.Fatal("a 仍然应该保持收起")
	}
}

// TestGeometryMatchesOfficialSpec：几何常量与 docs/official-specs.md 的逐条对应。
//
// 这不是"自证"，是防回归：这些数字被任何一次"顺手调一下"改动，都要在这里
// 先解释清楚它对应官方哪条 CSS。改对了就同步改这里和文档。
func TestGeometryMatchesOfficialSpec(t *testing.T) {
	want := map[string]struct{ got, spec float32 }{
		"sessionRowHeight": {sessionRowHeight, 32}, // .wnFoaW_sessionRow{height:32px}
		"groupRowHeight":   {groupRowHeight, 34},   // .wnFoaW_projectRow{height:34px}
		"rowGap":           {rowGap, 2},            // .GAkraW_flatList>*+*{margin-top:2px}
		"groupGap":         {groupGap, 4},          // .GAkraW_groupSection+.GAkraW_groupSection{margin-top:4px}
		"searchHeight":     {searchHeight, 30},     // .GAkraW_searchExpanded{height:30px}
		"sidebarInline":    {sidebarInline, 12},    // --dsh-sidebar-inline-padding:12px
		"indentStep":       {indentStep, 12},       // --dsh-workspace-indent: depth * 12px
		"slotWidth":        {slotWidth, 16},        // .wnFoaW_slot{width:16px}
		"slotHeight":       {slotHeight, 20},       // .wnFoaW_slot{height:20px}
		"titleLineHeight":  {titleLineHeight, 20},  // .wnFoaW_title{line-height:20px}
		"timeLineHeight":   {timeLineHeight, 16},   // .wnFoaW_time{line-height:16px}
		"titleMarginLeft":  {titleMarginLeft, 4},   // .wnFoaW_title{margin:0 6px 0 4px}
		"titleMarginRight": {titleMarginRight, 6},  // 同上
	}
	for name, c := range want {
		if c.got != c.spec {
			t.Errorf("%s = %v，官方是 %v", name, c.got, c.spec)
		}
	}
}

// TestNormalizeQuery：搜索词的空白与大小写。
func TestNormalizeQuery(t *testing.T) {
	cases := map[string]string{
		"":            "",
		"  Alpha  ":   "alpha",
		"DSH":         "dsh",
		"中文 标题":       "中文 标题", // 中间空格是内容，不裁
		"\t\r\n x \n": "x",
	}
	for in, want := range cases {
		if got := normalizeQuery(in); got != want {
			t.Errorf("normalizeQuery(%q) = %q，期望 %q", in, got, want)
		}
	}
}
