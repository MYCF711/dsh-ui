// schedule_page_test.go —— 官方任务管理页（#34）里**能脱离帧验证的判断**。
//
// 绘制本身由 `-demo`/真机截图看（本包的既有权变测试管外观）；这里钉住的是
// 数据 → 行的投影：筛选、搜索三字段、下次运行排序、频率文案。这些都是官方
// TaskManagerPage 同名逻辑（OFF:`TaskManagerPage.tsx` 的 `rows` memo、
// `schedule-format.ts`），错了会在界面上表现得像"宿主没数据"。
package dssidebar

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func scheduleFixture() []apiclient.ScheduleEntry {
	return []apiclient.ScheduleEntry{
		{
			ID: "t-daily", Kind: "daily", Title: "晨报", Prompt: "汇总阅读清单",
			Time: "09:00", TimeZone: "Asia/Shanghai",
			ScheduledAt: "2026-11-16T09:00:00+08:00", Status: "active",
			Target:             apiclient.ScheduleTarget{Kind: "session", SessionID: "sess-1"},
			InvolvedSessionIDs: []string{"sess-1"},
		},
		{
			ID: "t-every", Kind: "every", Title: "巡检", Prompt: "检查部署状态",
			EverySeconds: 7200, ScheduledAt: "2026-11-15T10:00:00+08:00", Status: "paused",
			Target:             apiclient.ScheduleTarget{Kind: "new"},
			InvolvedSessionIDs: []string{"sess-2"},
		},
		{
			ID: "t-weekly", Kind: "weekly", Title: "周报", Prompt: "写周报",
			Time: "18:30", TimeZone: "Asia/Shanghai", Weekdays: []int{1, 3, 5},
			ScheduledAt: "2026-11-17T18:30:00+08:00", Status: "active",
			Target:             apiclient.ScheduleTarget{Kind: "session", SessionID: "sess-3"},
			InvolvedSessionIDs: []string{"sess-3"},
		},
	}
}

func TestScheduleFiltersByStatusAndSearchFields(t *testing.T) {
	p := &Plugin{schedFilter: "active"}
	rows := p.filteredScheduleRows(scheduleFixture())
	if len(rows) != 2 || rows[0].ID != "t-daily" || rows[1].ID != "t-weekly" {
		t.Fatalf("active 行 = %+v（应按下次运行升序 daily → weekly）", rows)
	}

	p.schedFilter = "all"
	p.schedSearch.SetText("巡检")
	rows = p.filteredScheduleRows(scheduleFixture())
	if len(rows) != 1 || rows[0].ID != "t-every" {
		t.Fatalf("按标题搜索 = %+v", rows)
	}

	// 官方 README：搜索匹配存储的任务名、指令、内部 Session id，**不解析会话标题**。
	p.schedSearch.SetText("sess-3")
	rows = p.filteredScheduleRows(scheduleFixture())
	if len(rows) != 1 || rows[0].ID != "t-weekly" {
		t.Fatalf("按会话 id 搜索 = %+v", rows)
	}

	p.schedSearch.SetText("写周报")
	rows = p.filteredScheduleRows(scheduleFixture())
	if len(rows) != 1 || rows[0].ID != "t-weekly" {
		t.Fatalf("按指令搜索 = %+v", rows)
	}
}

func TestScheduleNextRunPrefersDeliveryFloor(t *testing.T) {
	entry := apiclient.ScheduleEntry{
		ScheduledAt:         "2026-11-15T09:00:00+08:00",
		DeliveryFloorEndsAt: "2026-11-15T10:30:00+08:00",
	}
	if got := schedNextRunAt(entry); got != entry.DeliveryFloorEndsAt {
		t.Fatalf("nextRunAt = %q，应取更晚的 deliveryFloorEndsAt", got)
	}
	entry.DeliveryFloorEndsAt = "2026-11-15T08:00:00+08:00"
	if got := schedNextRunAt(entry); got != entry.ScheduledAt {
		t.Fatalf("nextRunAt = %q，应取更晚的 scheduledAt", got)
	}
}

func TestScheduleFrequencyUsesOfficialWording(t *testing.T) {
	cases := []struct {
		entry apiclient.ScheduleEntry
		want  string
	}{
		{apiclient.ScheduleEntry{Kind: "after"}, "仅一次"},
		{apiclient.ScheduleEntry{Kind: "at"}, "仅一次"},
		{apiclient.ScheduleEntry{Kind: "every", EverySeconds: 7200}, "每 2 小时"},
		{apiclient.ScheduleEntry{Kind: "every", EverySeconds: 90}, "每 90 秒"},
		{apiclient.ScheduleEntry{Kind: "daily", Time: "09:00", TimeZone: "Asia/Shanghai"}, "每天 09:00（Asia/Shanghai）"},
		{apiclient.ScheduleEntry{Kind: "weekly", Time: "18:30", TimeZone: "Asia/Shanghai", Weekdays: []int{1, 3, 5}}, "每周一、三、五 18:30（Asia/Shanghai）"},
		{apiclient.ScheduleEntry{Kind: "cron", TimeZone: "Asia/Shanghai"}, "自定义（Asia/Shanghai）"},
		{apiclient.ScheduleEntry{Kind: "rrule"}, "自定义"},
	}
	for _, tc := range cases {
		if got := schedFrequency(tc.entry); got != tc.want {
			t.Fatalf("schedFrequency(%+v) = %q，want %q", tc.entry, got, tc.want)
		}
	}
}

func TestScheduleEmptyTextFollowsFilterAndSearch(t *testing.T) {
	p := &Plugin{schedFilter: "active"}
	if got := p.scheduleEmptyText(); got != zhSchedEmptyActive {
		t.Fatalf("active 空态 = %q", got)
	}
	p.schedFilter = "paused"
	if got := p.scheduleEmptyText(); got != zhSchedEmptyPaused {
		t.Fatalf("paused 空态 = %q", got)
	}
	p.schedFilter = "all"
	p.schedSearch.SetText("zzz")
	if got := p.scheduleEmptyText(); got != zhSchedNoMatches {
		t.Fatalf("搜索空态 = %q", got)
	}
}

func TestScheduleAbsoluteAndRelativeAreDerivedFromStoredOffset(t *testing.T) {
	if got := schedAbsolute("2026-11-15T09:00:00+08:00"); got != "11月15日 09:00" {
		t.Fatalf("absolute = %q", got)
	}
	if got := schedAbsolute("不是时间"); got != "不是时间" {
		t.Fatalf("坏输入应原样返回：%q", got)
	}
	// 相对时间只验形状：值依赖 now，钉死具体数字会变成时间炸弹。
	rel := schedRelative("2026-11-15T09:00:00+08:00")
	if !strings.HasPrefix(rel, "(") || !strings.HasSuffix(rel, ")") {
		t.Fatalf("relative = %q，应为括号形状", rel)
	}
}

func TestScheduleTaskNameAndTargetSession(t *testing.T) {
	entry := apiclient.ScheduleEntry{ID: "id-1", Title: "  "}
	if got := schedTaskName(entry); got != "id-1" {
		t.Fatalf("空标题兜底 = %q", got)
	}
	entry.Title = "巡检"
	if got := schedTaskName(entry); got != "巡检" {
		t.Fatalf("标题 = %q", got)
	}
	// new 目标：看最近一次投递的会话；没有投递就为空（官方显示「待创建」）。
	newTarget := apiclient.ScheduleEntry{Target: apiclient.ScheduleTarget{Kind: "new"}}
	if got := schedTargetSession(newTarget); got != "" {
		t.Fatalf("new 无投递 = %q", got)
	}
	newTarget.LastDelivery = &apiclient.ScheduleLastDelivery{TargetSessionID: "sess-9"}
	if got := schedTargetSession(newTarget); got != "sess-9" {
		t.Fatalf("new 投递会话 = %q", got)
	}
}

// TestSchedulePanelDrawsOfficialListAndDetail 是无头渲染冒烟：走真实的
// `drawSchedulePage`，钉住官方结构里**用户能看见的那些字**。
//
// 为什么需要它：纯函数测试证明不了"面板真的画出来了"——本仓踩过
// "状态切了没重绘"的坑（docs/official-parity.md 的复盘）。这里用与
// frame_test.go 同一套 canvas 断言。
func TestSchedulePanelDrawsOfficialListAndDetail(t *testing.T) {
	p, _, env, tree, canvas, _ := searchFixture(t)
	entries := scheduleFixture()
	env.Bounds = gfx.Rect{Width: 1182, Height: 832}
	env.Geometry = pluginapiGeometryForSchedule()
	env.Act.NewSession = func() {}
	env.Act.ScheduleRefresh = func() {}
	env.Act.ScheduleCatalog = func() ([]apiclient.ScheduleEntry, bool, string) {
		return entries, true, ""
	}
	env.Act.ScheduleHistory = func(string) (apiclient.ScheduleHistoryPage, bool, string) {
		return apiclient.ScheduleHistoryPage{ID: "t-daily", Records: []apiclient.ScheduleHistoryRecord{{
			ScheduledAt: "2026-11-15T09:00:00+08:00", DeliveredAt: "2026-11-15T09:00:03+08:00",
			MessageID: "m1", TargetSessionID: "sess-1", Prompt: "晨报正文",
		}}}, true, ""
	}
	env.Act.ScheduleSetActive = func(string, bool, string) {}
	env.Act.ScheduleRun = func(string) {}
	env.Act.ScheduleDelete = func(string) {}

	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	draw := func() {
		view.Draw(canvas, env.Bounds, func(f ui.C) {
			ui.Column(f).FillWidth().FillHeight().Children(func() { p.drawSchedulePage(f, env) })
		})
	}
	draw()
	for _, want := range []string{
		"自动化任务", "新建", "已开启", "已暂停", "已结束", "全部",
		"晨报", "周报",
	} {
		fg2Text(t, canvas, want)
	}
	if fg2HasText(canvas, "巡检") {
		t.Fatal("默认 active 筛选不该画 paused 任务")
	}

	// 点任务行 → 详情（官方 Rules）：名称/内容/运行时间。
	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "schedule-row-t-daily"), gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
	if p.schedSelected != "t-daily" {
		t.Fatalf("点击任务行后选中 = %q", p.schedSelected)
	}
	for _, want := range []string{"规则", "任务运行记录", "自动化任务内容", "汇总阅读清单", "运行时间", "每天 09:00（Asia/Shanghai）"} {
		fg2Text(t, canvas, want)
	}

	// 记录 tab：真实读 ScheduleHistory 的缓存形态。
	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "schedule-tab-records"), gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
	if p.schedTab != "records" {
		t.Fatalf("点击记录 tab 后 = %q", p.schedTab)
	}
	fg2Text(t, canvas, "晨报正文")
}

// pluginapiGeometryForSchedule 是主区接管时的三栏几何（与 frame_test 同值）。
func pluginapiGeometryForSchedule() pluginapi.Geometry {
	return pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902}
}
