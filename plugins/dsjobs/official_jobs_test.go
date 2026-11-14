package dsjobs

// official_jobs_test.go —— 官方槽位（`conversation.session.header.actions` 触发器 +
// 500px 弹层）的真绘制测试。
//
// 依据是官方 `JobListAction.tsx:527-640` 与 `JobListAction.module.css`：
//
//   - 触发器：活动槽（无活任务=layers 图标 / 有活任务=ongoing 点）+ `{count} 个后台任务`；
//   - 弹层：活任务行直接列；已结束段有折叠按钮（`已结束 N`）与「清空」；
//   - 停止：跑着的行两段式；已结束的行没有停止钮；
//   - Escape 关闭（官方 document keydown），ArrowDown 打开。
//
// 所有断言只对本帧画布做（不跨帧读 ui.Box）。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// clickOfficial 走官方档的一次真实点击（BeginEventPass → 画官方帧 → EndEventPass）。
func (h *harness) clickOfficial(t *testing.T, id gfx.ID, w, hh float32) {
	t.Helper()
	h.canvas.BeginEventPass(id, gfx.ClickInfo{})
	h.drawOfficial(t, w, hh)
	h.canvas.EndEventPass()
}

// drawOfficial 画「官方档」的一帧：触发器 + （打开时的）遮罩与弹层。
func (h *harness) drawOfficial(t *testing.T, w, hh float32) {
	t.Helper()
	h.canvas.Reset()
	pageTheme := dscore.VivaTheme(h.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(h.canvas, gfx.Rect{Width: w, Height: hh}, func(f ui.C) {
		h.p.drawTrigger(f, h.env)
		h.p.drawPanelBackdrop(f, h.env)
		h.p.drawPanel(f, h.env, nil)
	})
}

func TestOfficialTriggerShowsCountAndOpensPanel(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
		{ID: "job-2", SessionID: "s-1", Command: "go build", Done: true, ExitCode: 0},
	}, nil)
	h.drawOfficial(t, 1182, 832)
	if !h.has("1 个后台任务") {
		t.Fatalf("有活任务时计数只数活任务（官方 count = liveRows）：%v", h.texts())
	}
	if h.p.isOpen() {
		t.Fatal("初始不应打开")
	}
	h.clickOfficial(t, h.id("jobs-trigger"), 1182, 832)
	if !h.p.isOpen() {
		t.Fatal("点击触发器应打开弹层")
	}
	h.drawOfficial(t, 1182, 832)
	// 活任务行 + 已结束段头（官方 section.settledCount = `已结束 N`）与清空。
	if !h.has("npm") {
		t.Fatalf("活任务行没画：%v", h.texts())
	}
	// 有活任务时已结束段默认折叠（官方 settledOpen ?? liveRows.length===0），
	// 但段头与「清空」必须在。
	if !h.has("已结束 1") || !h.has("清空") {
		t.Fatalf("已结束段头没画：%v", h.texts())
	}
	if h.has("go") {
		t.Fatalf("有活任务时已结束段应默认折叠：%v", h.texts())
	}
	h.clickOfficial(t, h.id("jobs-settled-toggle"), 1182, 832)
	h.drawOfficial(t, 1182, 832)
	if !h.has("go") {
		t.Fatalf("展开已结束段后应看到已结束行：%v", h.texts())
	}
}

func TestOfficialTriggerHiddenWithoutJobs(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.drawOfficial(t, 1182, 832)
	if h.has("0 个后台任务") || h.p.isOpen() {
		t.Fatalf("没有任务时不应出现触发器：%v", h.texts())
	}
}

func TestOfficialSettledSectionFoldsAndClears(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "done-one", Done: true, ExitCode: 0},
		{ID: "job-2", SessionID: "s-1", Command: "done-two", Done: true, ExitCode: 1},
	}, nil)
	h.p.setOpen(true)
	h.drawOfficial(t, 1182, 832)
	// 没有活任务时已结束段默认展开（官方 settledOpen ?? liveRows.length===0）。
	if !h.has("done-one") || !h.has("done-two") {
		t.Fatalf("已结束段默认应展开：%v", h.texts())
	}
	h.clickOfficial(t, h.id("jobs-settled-toggle"), 1182, 832)
	h.drawOfficial(t, 1182, 832)
	if h.has("done-one") || h.has("done-two") {
		t.Fatalf("点折叠后不该再画已结束行：%v", h.texts())
	}
	// 再展开，点「清空」：行被客户端隐藏（官方 clearedKeys 语义）。
	h.clickOfficial(t, h.id("jobs-settled-toggle"), 1182, 832)
	h.drawOfficial(t, 1182, 832)
	h.clickOfficial(t, h.id("jobs-settled-clear"), 1182, 832)
	h.drawOfficial(t, 1182, 832)
	if h.has("done-one") || h.has("done-two") || h.has("已结束 2") {
		t.Fatalf("清空后不该再画已结束段：%v", h.texts())
	}
}

func TestOfficialStopNeedsTwoPressesAndSettledHasNone(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
		{ID: "job-2", SessionID: "s-1", Command: "finished", Done: true, ExitCode: 0},
	}, nil)
	h.p.setOpen(true)
	h.drawOfficial(t, 1182, 832)
	stop := h.id("jobs-stop-s-1\x00job-1")
	// 已结束的行没有停止钮：对同一个 id 连点两下也不该发出任何停止请求。
	settledStop := h.id("jobs-stop-s-1\x00job-2")
	h.clickOfficial(t, settledStop, 1182, 832)
	h.drawOfficial(t, 1182, 832)
	h.clickOfficial(t, settledStop, 1182, 832)
	if got := h.act.stops(); len(got) != 0 {
		t.Fatalf("已结束的行发出了停止请求：%v", got)
	}
	h.clickOfficial(t, stop, 1182, 832)
	if got := h.act.stops(); len(got) != 0 {
		t.Fatalf("第一击不该发停止：%v", got)
	}
	h.drawOfficial(t, 1182, 832)
	if !h.has(zhStopConfirm) {
		t.Fatalf("第一击后应显示确认：%v", h.texts())
	}
	h.clickOfficial(t, stop, 1182, 832)
	if got := h.act.stops(); len(got) != 1 || got[0] != "s-1/job-1" {
		t.Fatalf("第二击应发 stop(s-1, job-1)，实得 %v", got)
	}
}

func TestOfficialPanelOutputExpandsWithPaging(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
	}, func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool) {
		return pluginapi.JobOutput{
			ID: id, SessionID: sessionID, Output: "official output line",
			Cursor: 0, NextCursor: 20, HasMore: true, Total: 20,
		}, true
	})
	h.p.setOpen(true)
	h.drawOfficial(t, 1182, 832)
	h.clickOfficial(t, h.id("jobs-row-s-1\x00job-1"), 1182, 832)
	h.drawOfficial(t, 1182, 832)
	if !h.has("official output line") {
		t.Fatalf("弹层里展开后没画输出：%v", h.texts())
	}
	if !h.has(zhOutputMore) {
		t.Fatalf("hasMore=true 时应画加载更多：%v", h.texts())
	}
}

func TestOfficialKeyboardGestures(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
	}, nil)
	h.drawOfficial(t, 1182, 832) // 让 handleKey 记住"有任务可开"
	if !h.p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown}) {
		t.Fatal("有任务时 ArrowDown 应打开弹层")
	}
	if !h.p.isOpen() {
		t.Fatal("ArrowDown 后弹层没打开")
	}
	if !h.p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("打开时 Escape 应关闭")
	}
	if h.p.isOpen() {
		t.Fatal("Escape 后弹层还开着")
	}
	if h.p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("没开时不该吃掉 Escape")
	}
}

func TestOfficialTierGatingKeepsLegacySurfaceOnly(t *testing.T) {
	// 没有档位持有者 = 官方（默认）；显式 legacy 时官方触发器必须不画。
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
	}, nil)
	tier := uikernel.NewShellTier()
	tier.Set(false)
	h.p.AttachShellTier(tier)
	h.drawOfficial(t, 1182, 832)
	if h.has("1 个后台任务") {
		t.Fatalf("legacy 档不该画官方触发器：%v", h.texts())
	}
}
