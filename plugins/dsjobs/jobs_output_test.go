package dsjobs

// jobs_output_test.go —— 后台任务「读输出 / 停止」的**真实路径**测试。
//
// 方法同 `plugins/dsattachment/rail_test.go` 与 `plugins/dsdirpicker/browse_test.go`：
// ui.NewPage 真画一帧 + gfx.Canvas 真 HitTest + BeginEventPass 真点击，
// 断言只依据**官方能证明的东西**：
//
//   - 文案逐字来自官方 zh 字典（`ui-jobs/src/client/locales.ts`）；
//   - 停止是两段式（`kill.confirm` / `kill.confirmAction`）；
//   - 输出分页的 cursor/nextCursor/hasMore 语义来自宿主
//     `HOST:internal/transport/handlers_jobs.go:87-160`；
//   - 失败态必须为 `实时输出流中断：{error}`，不许折叠成"没有输出"。
//
// 所有断言都是"这一帧画出来的事实"，不跨帧读 ui.Box（viva 的 Box 只在本次 paint 内有效）。

import (
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// fakeJobs 是一个只记录调用、按测试给的视图应答的 Actions 替身。
type fakeJobs struct {
	mu sync.Mutex
	// jobs 是 JobsList 返回的清单。
	jobs []pluginapi.Job
	// output 是 JobsOutput 的应答（ok=false 表示"壳还没拉到"）。
	output func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool)
	// 记录每一次真实请求。
	outputCalls []outputCall
	stopCalls   []string
	// stopped 允许测试断言"第二次点击才真的发了停止"。
	copied []string
}

type outputCall struct {
	sessionID string
	id        string
	cursor    int
}

func (f *fakeJobs) actions() pluginapi.Actions {
	return pluginapi.Actions{
		JobsList: func() []pluginapi.Job {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.jobs
		},
		JobsOutput: func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool) {
			f.mu.Lock()
			f.outputCalls = append(f.outputCalls, outputCall{sessionID, id, cursor})
			respond := f.output
			f.mu.Unlock()
			if respond == nil {
				return pluginapi.JobOutput{}, false
			}
			return respond(sessionID, id, cursor)
		},
		JobsStop: func(sessionID, id string) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.stopCalls = append(f.stopCalls, sessionID+"/"+id)
		},
		Copy: func(text string) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.copied = append(f.copied, text)
		},
	}
}

func (f *fakeJobs) setOutput(respond func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.output = respond
}

func (f *fakeJobs) calls() []outputCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]outputCall(nil), f.outputCalls...)
}

func (f *fakeJobs) stops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.stopCalls...)
}

// harness 是一次无头绘制现场（不 mock 绘制层）。
type harness struct {
	p      *Plugin
	env    *pluginapi.Env
	tree   *gfx.Tree
	canvas *gfx.Canvas
	act    *fakeJobs
	theme  *pluginapi.Theme
}

func newHarness(t *testing.T, jobs []pluginapi.Job, output func(string, string, int) (pluginapi.JobOutput, bool)) *harness {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("jobs-test")
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	act := &fakeJobs{jobs: jobs, output: output}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feat, Act: act.actions()}
	return &harness{p: p, env: env, tree: tree, canvas: canvas, act: act, theme: theme}
}

func (h *harness) draw(t *testing.T, w, h2 float32) {
	t.Helper()
	h.canvas.Reset()
	pageTheme := dscore.VivaTheme(h.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(h.canvas, gfx.Rect{Width: w, Height: h2}, func(f ui.C) { h.p.draw(f, h.env) })
}

func (h *harness) click(t *testing.T, id gfx.ID, w, h2 float32) {
	t.Helper()
	h.canvas.BeginEventPass(id, gfx.ClickInfo{})
	h.draw(t, w, h2)
	h.canvas.EndEventPass()
}

func (h *harness) texts() []string {
	out := make([]string, 0, len(h.canvas.Texts()))
	for _, text := range h.canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func (h *harness) has(text string) bool {
	for _, value := range h.texts() {
		if value == text {
			return true
		}
	}
	return false
}

// id 造出与插件**同一条公式**的交互 id（同一个 tree + feature 作用域）。
func (h *harness) id(name string) gfx.ID { return h.tree.ID(h.env.Feat, name) }

// ── 纯函数：三态映射（官方五态里需要 jobs 控制流的两态不编）──────────────

func TestStatusLabelOnlyUsesThreeRealStates(t *testing.T) {
	cases := []struct {
		name string
		job  pluginapi.Job
		want string
	}{
		{"跑着", pluginapi.Job{ID: "a"}, zhStatusRunning},
		{"正常结束", pluginapi.Job{ID: "a", Done: true, ExitCode: 0}, zhStatusCompleted},
		{"非零退出", pluginapi.Job{ID: "a", Done: true, ExitCode: 2}, zhStatusFailed},
	}
	for _, tc := range cases {
		if got := statusLabel(tc.job); got != tc.want {
			t.Errorf("%s: statusLabel = %q，want %q", tc.name, got, tc.want)
		}
	}
	// 五态里的 stopping/killed 需要 jobs 控制流（JobView.status）；本仓没有，
	// 所以这两个词一个都不许出现——出现就是编了一条不存在的事实。
	for _, banned := range []string{"正在停止", "已取消"} {
		if statusLabel(pluginapi.Job{ID: "a"}) == banned ||
			statusLabel(pluginapi.Job{ID: "a", Done: true, ExitCode: 1}) == banned {
			t.Fatalf("画出了本仓拿不到的状态 %q", banned)
		}
	}
}

// ── 不给动作就不画（死控件比没控件更坏）────────────────────────────────

func TestNoOutputActionDrawsNoAffordance(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test", Bytes: 120},
	}, nil)
	// 壳没接 JobsOutput / JobsStop：清单照画，但不许出现展开箭头。
	h.env.Act.JobsOutput = nil
	h.env.Act.JobsStop = nil
	h.draw(t, 240, 700)
	if !h.has("npm") {
		t.Fatalf("清单行没画出来：%v", h.texts())
	}
	if h.has("▸") || h.has("停止") {
		t.Fatalf("没有动作却画了可点控件：%v", h.texts())
	}
}

// ── 展开 → 真读到输出 ────────────────────────────────────────────────

func TestRowClickLoadsOutputPage(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test", Bytes: 120},
	}, func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool) {
		return pluginapi.JobOutput{
			ID: id, SessionID: sessionID, Output: "line one\nline two",
			Done: false, Cursor: 0, NextCursor: 17, Total: 17,
		}, true
	})
	// 先画一帧拿到真实 ID（row id 由 env.Tree.ID 生成，测试按同一公式造）。
	h.draw(t, 240, 700)
	h.click(t, h.id("job-row-s-1\x00job-1"), 240, 700)
	// 点击那一帧 row.Clicked() 触发展开；下一帧才按展开态请求输出。
	h.draw(t, 240, 700)
	// 画布按行拆 text 图元：两行都在，就说明整段输出画出来了。
	if !h.has("line one") || !h.has("line two") {
		t.Fatalf("展开后没有画输出：%v", h.texts())
	}
	calls := h.act.calls()
	if len(calls) == 0 || calls[len(calls)-1].sessionID != "s-1" || calls[len(calls)-1].id != "job-1" {
		t.Fatalf("JobsOutput 调用不对：%+v", calls)
	}
	if calls[len(calls)-1].cursor != 0 {
		t.Fatalf("首次展开应从头读（cursor=0），实得 %d", calls[len(calls)-1].cursor)
	}
	// 有 HasMore 才画「加载更多」；这一页没有 → 不许画。
	if h.has(zhOutputMore) {
		t.Fatalf("hasMore=false 却画了「加载更多」：%v", h.texts())
	}
}

// ── 分页：加载更多要真的用 NextCursor 再要一页 ──────────────────────────

func TestLoadMoreUsesNextCursor(t *testing.T) {
	page2 := false
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
	}, func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool) {
		if !page2 {
			return pluginapi.JobOutput{
				ID: id, SessionID: sessionID, Output: "first", Cursor: 0, NextCursor: 5,
				HasMore: true, Total: 11,
			}, true
		}
		// 第二页：壳返回**累计视图**（Output 是从 0 到当前游标的全部文本）。
		return pluginapi.JobOutput{
			ID: id, SessionID: sessionID, Output: "firstsecond", Cursor: 0, NextCursor: 11,
			HasMore: false, Total: 11,
		}, true
	})
	h.draw(t, 240, 700)
	h.click(t, h.id("job-row-s-1\x00job-1"), 240, 700)
	h.draw(t, 240, 700)
	if !h.has(zhOutputMore) {
		t.Fatalf("hasMore=true 应画「加载更多」：%v", h.texts())
	}
	// 点「加载更多」→ 插件把请求游标推到 NextCursor(5)。
	// 顺序要紧：按钮必须在这一帧**还画着**的时候点，所以先点、再切到第二页。
	h.click(t, h.id("job-btn-s-1\x00job-1-more"), 240, 700)
	page2 = true
	h.draw(t, 240, 700)
	calls := h.act.calls()
	last := calls[len(calls)-1]
	if last.cursor != 5 {
		t.Fatalf("「加载更多」应当用 NextCursor=5 请求，实得 cursor=%d", last.cursor)
	}
	if !h.has("firstsecond") {
		t.Fatalf("第二页没画出来：%v", h.texts())
	}
	if h.has(zhOutputMore) {
		t.Fatalf("读完之后不该再画「加载更多」：%v", h.texts())
	}
}

// ── 失败必须如实画出来（不许折叠成"没有输出"）────────────────────────

func TestOutputErrorDrawsReasonAndRetry(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
	}, func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool) {
		return pluginapi.JobOutput{ID: id, SessionID: sessionID, Err: "job 已结束（轮次之外读不到）"}, true
	})
	h.draw(t, 240, 700)
	h.click(t, h.id("job-row-s-1\x00job-1"), 240, 700)
	h.draw(t, 240, 700)
	want := zhOutputError + "job 已结束（轮次之外读不到）"
	if !h.has(want) {
		t.Fatalf("错误行没画出来（want %q）：%v", want, h.texts())
	}
	if h.has(zhOutputEmpty) {
		t.Fatal("读失败被折叠成了「（无输出）」")
	}
	if !h.has(zhOutputRetry) {
		t.Fatalf("错误态应给「重试」：%v", h.texts())
	}
}

// ── 分页截断是独立事实 ──────────────────────────────────────────────

func TestTruncatedNoticeDrawn(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
	}, func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool) {
		return pluginapi.JobOutput{
			ID: id, SessionID: sessionID, Output: "tail only", Cursor: 0, NextCursor: 9,
			Total: 9, Truncated: true,
		}, true
	})
	h.draw(t, 240, 700)
	h.click(t, h.id("job-row-s-1\x00job-1"), 240, 700)
	h.draw(t, 240, 700)
	if !h.has(zhOutputGap) {
		t.Fatalf("truncated 提示没画出来：%v", h.texts())
	}
	if !h.has("tail only") {
		t.Fatalf("保留的输出没画出来：%v", h.texts())
	}
}

// ── 停止：两段式（第一次只进入确认，第二次才发请求）───────────────────

func TestStopNeedsTwoPresses(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
	}, nil)
	stop := h.id("job-stop-s-1\x00job-1")
	h.draw(t, 240, 700)
	if !h.has(zhStopLabel) {
		t.Fatalf("运行中的行应画停止钮：%v", h.texts())
	}
	// 第一击：进入确认态，不发请求。
	h.click(t, stop, 240, 700)
	if got := h.act.stops(); len(got) != 0 {
		t.Fatalf("第一击就发了停止请求：%v", got)
	}
	h.draw(t, 240, 700)
	if !h.has(zhStopConfirm) {
		t.Fatalf("第一击之后应显示 %q：%v", zhStopConfirm, h.texts())
	}
	// 第二击：这才真的发。
	h.click(t, stop, 240, 700)
	if got := h.act.stops(); len(got) != 1 || got[0] != "s-1/job-1" {
		t.Fatalf("第二击应发一次 stop(s-1, job-1)，实得 %v", got)
	}
}

// ── Escape 收起（只在真有东西可收时吃掉按键）──────────────────────────

func TestEscapeCollapsesOnlyWhenExpanded(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
	}, func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool) {
		return pluginapi.JobOutput{ID: id, SessionID: sessionID, Output: "out"}, true
	})
	if h.p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("没有展开时不该吃掉 Escape")
	}
	h.draw(t, 240, 700)
	h.click(t, h.id("job-row-s-1\x00job-1"), 240, 700)
	h.draw(t, 240, 700)
	if !h.has("out") {
		t.Fatalf("展开失败：%v", h.texts())
	}
	if !h.p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("展开时按 Escape 应被处理")
	}
	h.draw(t, 240, 700)
	if h.has("out") {
		t.Fatalf("Escape 之后输出应收起：%v", h.texts())
	}
}

// ── 空清单不画（原行为保留）────────────────────────────────────────

func TestEmptyListDrawsNothing(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.draw(t, 240, 700)
	if got := len(h.canvas.Texts()); got != 0 {
		t.Fatalf("没有任务时不该画任何文字，实际 %d 段：%v", got, h.texts())
	}
}
