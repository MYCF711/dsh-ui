//go:build darwin || windows || linux

package dsconversation

// official_transcript_paging_test.go —— 主区「加载更早」那一行的**五态**断言。
//
// 两半：
//
//	① 纯函数：`historyLoadState` 的五态判据 + 文案 + 可点性（喂 `HistoryPaging` 就能算）
//	② 真画布：五态各自渲染成什么（文本/按钮/禁用），以及**行序**（它在 flow 之前）
//
// 还有一条**真点击**（`headless` 真 Paint → 真 HitTest → 真 Press/Release）：
// 只断言"按钮画出来了"是不够的——`Clicked()` 没接 Action 的话，界面上按钮
// 一切正常，点下去什么都不会发生。
//
// 五态与官方文案的对应（`dsh-client-ui-chat/src/client/locale.ts:94-98`）：
//
//	首次    `.hint`「正在加载历史…」      chat.loadingHistory
//	有更多  按钮「加载更早」              chat.loadOlder
//	加载中  按钮「正在加载」（disabled）   chat.loadingOlder
//	无更多  按钮「已到会话开头」（disabled） chat.historyStart
//	失败    按钮「重新加载」              chat.retryOlder

import (
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// ── ① 纯函数：五态判据 ────────────────────────────────────────────────

// TestHistoryLoadStateFiveStates 五态逐个钉住。
//
// 判据顺序照官方（`ChatView.tsx:266-273`）：先 `!hasMore`（start），
// 再 `loadingOlder`（loading），再 `historyLoadStatus === 'error'`（error），
// 最后落到 more。顺序错了会出现"到头了还在转圈"这类假状态。
func TestHistoryLoadStateFiveStates(t *testing.T) {
	cases := []struct {
		name  string
		in    pluginapi.HistoryPaging
		want  HistoryLoadState
		label string
		click bool
	}{
		{
			name:  "首次（基线还在拉）",
			in:    pluginapi.HistoryPaging{Baseline: false, Loading: true},
			want:  HistoryFirst,
			label: "正在加载历史…",
			click: false, // 基线没到，没有可点的位置
		},
		{
			name:  "有更多",
			in:    pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true},
			want:  HistoryMore,
			label: "加载更早",
			click: true,
		},
		{
			name:  "加载中",
			in:    pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true, Loading: true},
			want:  HistoryLoading,
			label: "正在加载",
			click: false,
		},
		{
			name:  "无更多（已到会话开头）",
			in:    pluginapi.HistoryPaging{Baseline: true, HasMore: false, Partial: true},
			want:  HistoryStart,
			label: "已到会话开头",
			click: false,
		},
		{
			name:  "失败（可点重试）",
			in:    pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true, Error: "boom"},
			want:  HistoryFailed,
			label: "重新加载",
			click: true,
		},
		{
			// 一页装得下的短会话：官方根本不画这一行（`(hasMore || historyWasPartial)`）。
			// 画出来的话每个短会话顶部永远挂着「已到会话开头」。
			name: "短会话整行不画",
			in:   pluginapi.HistoryPaging{Baseline: true, HasMore: false, Partial: false},
			want: HistoryHidden,
		},
		{
			name: "老宿主没有分页信息",
			in:   pluginapi.HistoryPaging{},
			want: HistoryHidden,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := historyLoadState(c.in)
			if got != c.want {
				t.Fatalf("historyLoadState(%+v) = %v，想要 %v", c.in, got, c.want)
			}
			if got != HistoryHidden {
				if label := historyLoadLabel(got); label != c.label {
					t.Fatalf("文案 = %q，想要 %q", label, c.label)
				}
				if clickable := historyLoadClickable(got); clickable != c.click {
					t.Fatalf("可点 = %v，想要 %v", clickable, c.click)
				}
			}
		})
	}
}

// TestHistoryLoadStateNames 状态名是诊断/报告用的稳定词表（不参与绘制）。
func TestHistoryLoadStateNames(t *testing.T) {
	want := map[HistoryLoadState]string{
		HistoryHidden:  "hidden",
		HistoryFirst:   "first",
		HistoryMore:    "more",
		HistoryLoading: "loading",
		HistoryStart:   "start",
		HistoryFailed:  "error",
	}
	for state, name := range want {
		if got := state.String(); got != name {
			t.Fatalf("%d.String() = %q，想要 %q", int(state), got, name)
		}
	}
}

// pagingTexts 把画布上的文本原样列出来（失败时的诊断信息）。
func pagingTexts(canvas *ui.Canvas) []string {
	values := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	return values
}

// ── ② 真画布：五态各自渲染成什么 ──────────────────────────────────────

// pagingView 用真 flex + 确定性字宽测量器画一帧，返回视图与画布。
func pagingView(t *testing.T, entries []TranscriptEntry, history pluginapi.HistoryPaging, onLoadOlder func()) (*OfficialTranscriptView, *ui.Canvas) {
	t.Helper()
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas, tree := newTranscriptCanvas()
	data := pluginapi.Snapshot{Cur: "s-paging", History: history}
	env := &pluginapi.Env{
		Theme: theme, Data: data, Tree: tree, Feat: tree.Feature("transcript-paging-test"),
		Canvas: canvas, Bounds: ui.Rect{Width: 900, Height: 700}, MainWidth: 900,
	}
	env.Act.LoadOlder = onLoadOlder
	var fv flex.View
	fv.Draw(canvas, dscore.VivaTheme(theme), ui.Rect{Width: 900, Height: 700}, func(f *flex.Frame) {
		view.Draw(f, env, hostViewport(tree), entries)
	})
	return view, canvas
}

// pagingEntries 造三条消息行（用来验行序：分页行在 flow 之前）。
func pagingEntries() []TranscriptEntry {
	return []TranscriptEntry{
		{Key: "u1", Kind: ChatNodeUser, Seq: 1, Turn: 0, Data: TranscriptData{Text: "第一问"}},
		{Key: "a1", Kind: ChatNodeAssistantStep, Seq: 2, Turn: 0, Data: TranscriptData{Text: "第一答"}},
	}
}

// TestPagingRowRendersFiveStates 五态各自画出来的**文本**。
func TestPagingRowRendersFiveStates(t *testing.T) {
	cases := []struct {
		state HistoryLoadState
		in    pluginapi.HistoryPaging
	}{
		{HistoryFirst, pluginapi.HistoryPaging{Baseline: false, Loading: true}},
		{HistoryMore, pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true}},
		{HistoryLoading, pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true, Loading: true}},
		{HistoryStart, pluginapi.HistoryPaging{Baseline: true, Partial: true}},
		{HistoryFailed, pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true, Error: "boom"}},
	}
	for _, c := range cases {
		t.Run(c.state.String(), func(t *testing.T) {
			view, canvas := pagingView(t, pagingEntries(), c.in, nil)
			if view.Boxes.HistoryState != c.state {
				t.Fatalf("这一帧的五态 = %v，想要 %v", view.Boxes.HistoryState, c.state)
			}
			want := historyLoadLabel(c.state)
			if _, ok := canvasText(canvas, want); !ok {
				t.Fatalf("画布上没有 %q：%q", want, pagingTexts(canvas))
			}
			// 消息行照画（分页行不能把消息顶掉）。
			if _, ok := canvasText(canvas, "第一问"); !ok {
				t.Fatalf("消息行没画出来：%q", pagingTexts(canvas))
			}
			if got := len(view.Boxes.Rows); got != 2 {
				t.Fatalf("消息行数 = %d，想要 2（分页行不属于 Rows）", got)
			}
		})
	}
}

// TestPagingRowHiddenForShortSession 短会话：**一个像素都不画**。
func TestPagingRowHiddenForShortSession(t *testing.T) {
	view, canvas := pagingView(t, pagingEntries(),
		pluginapi.HistoryPaging{Baseline: true}, nil)
	if view.Boxes.HistoryState != HistoryHidden {
		t.Fatalf("短会话的五态 = %v，想要 hidden", view.Boxes.HistoryState)
	}
	for _, text := range []string{"加载更早", "已到会话开头", "正在加载", "重新加载", "正在加载历史…"} {
		if _, ok := canvasText(canvas, text); ok {
			t.Fatalf("短会话画出了分页行 %q", text)
		}
	}
}

// TestPagingRowSitsAboveFlow 行序：分页行在**消息流之前**（官方 `.column` 子级顺序）。
//
// 画在下面的话，用户翻到会话开头时会先看到它、再看到最早那条消息，
// 与官方"顶部有一条加载更早"完全相反。
func TestPagingRowSitsAboveFlow(t *testing.T) {
	view, _ := pagingView(t, pagingEntries(),
		pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true}, nil)
	if view.Boxes.HistoryState != HistoryMore {
		t.Fatalf("五态 = %v，想要 more", view.Boxes.HistoryState)
	}
	older := view.Boxes.Older.Rect()
	first := view.Boxes.Rows[0].Box.Rect()
	if older.Y >= first.Y {
		t.Fatalf("分页行 Y=%.1f 不在首条消息 Y=%.1f 之上（行序反了）", older.Y, first.Y)
	}
	if older.Height <= 0 {
		t.Fatalf("分页行高度 = %.1f（零高 = 没真画）", older.Height)
	}
}

// TestPagingRowDisabledStates 禁用态：加载中 / 无更多都点不动。
func TestPagingRowDisabledStates(t *testing.T) {
	for _, c := range []struct {
		state HistoryLoadState
		in    pluginapi.HistoryPaging
	}{
		{HistoryLoading, pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true, Loading: true}},
		{HistoryStart, pluginapi.HistoryPaging{Baseline: true, Partial: true}},
	} {
		t.Run(c.state.String(), func(t *testing.T) {
			called := 0
			view, _ := pagingView(t, pagingEntries(), c.in, func() { called++ })
			if view.Boxes.HistoryState != c.state {
				t.Fatalf("五态 = %v，想要 %v", view.Boxes.HistoryState, c.state)
			}
			// 绘制期不会自己触发；这里验的是"这一态被算成不可点"。
			if historyLoadClickable(c.state) {
				t.Fatalf("%v 不该可点", c.state)
			}
			if called != 0 {
				t.Fatalf("绘制期自己发了 %d 次 LoadOlder", called)
			}
		})
	}
}

// ── ③ 真点击：headless 真 Paint → 真 HitTest → 真 Press/Release ───────

// TestPagingButtonRealClick 点一次真按钮，Action 必须被调到。
//
// 只断言"按钮画出来了"是不够的：`Clicked()` 没接 Action 时界面完全正常，
// 点下去什么都不发生（"点了没反应"是这一行最坏的失败形态）。
func TestPagingButtonRealClick(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	tree := &ui.Tree{}
	feat := tree.Feature("paging-headless")
	var (
		invalidator app.Invalidator
		loads       int
	)
	data := pluginapi.Snapshot{
		Cur:     "s-paging-headless",
		History: pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true},
	}
	config := app.Config{
		Title: "paging-headless", Width: 900, Height: 700, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	var fv flex.View
	config.Paint = func(canvas *ui.Canvas, current ui.Size) {
		bounds := ui.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: current.Width, Scroll: fv.ScrollState,
		}
		env.Act.LoadOlder = func() { loads++ }
		fv.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			view.Draw(f, env, TranscriptViewport{HostScroll: true, HostID: tree.ID(feat, "host"), SelfID: tree.ID(feat, "self")}, pagingEntries())
		})
	}
	config.OnScroll = func(event ui.ScrollEvent) ui.EventResult { return fv.Wheel(event) }
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	defer session.Close()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}

	id := tree.ID(feat, "history-older")
	var hit ui.Hit
	found := false
	for _, candidate := range session.Snapshot().Hits {
		if candidate.ID == id {
			hit, found = candidate, true
			break
		}
	}
	if !found {
		t.Fatalf("「加载更早」按钮没有命中区 id=%v；hits=%+v", id, session.Snapshot().Hits)
	}
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	if err := session.Press(x, y); err != nil {
		t.Fatal(err)
	}
	if err := session.Release(x, y); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if loads != 1 {
		t.Fatalf("真点了一次「加载更早」，LoadOlder 被调 %d 次，想要 1 次", loads)
	}
}

// ── ④ 接管条件：什么时候由 transcript 接管主区 ────────────────────────

// TestDrawProductTranscriptTakeover 首屏基线还在拉时**且**会话有内容 → 接管
// （画 `.hint`「正在加载历史…」）；空白会话 / 老宿主 → 回退旧渲染路径。
func TestDrawProductTranscriptTakeover(t *testing.T) {
	cases := []struct {
		name string
		data pluginapi.Snapshot
		want bool
	}{
		{
			name: "基线还在拉 + 有投影消息 → 接管（首次态）",
			data: pluginapi.Snapshot{
				Msgs:    []apiclient.Message{{Seq: 1, Text: "旧消息"}},
				History: pluginapi.HistoryPaging{Loading: true},
			},
			want: true,
		},
		{
			name: "空白会话（Hero）→ 不接管",
			data: pluginapi.Snapshot{History: pluginapi.HistoryPaging{Loading: true}},
			want: false,
		},
		{
			name: "老宿主没有任何分页信息 → 不接管",
			data: pluginapi.Snapshot{},
			want: false,
		},
		{
			name: "已经有原始帧 → 接管",
			data: pluginapi.Snapshot{
				Frames:  []apiclient.Frame{{Seq: 1, Type: "user/message"}},
				History: pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true},
			},
			want: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &Plugin{}
			canvas, tree := newTranscriptCanvas()
			theme := dscore.Dark()
			p.theme = theme
			p.prims = &dscore.Primitives{Theme: theme}
			env := &pluginapi.Env{
				Theme: theme, Data: c.data, Tree: tree, Feat: tree.Feature("takeover"),
				Canvas: canvas, Bounds: ui.Rect{Width: 900, Height: 700}, MainWidth: 900,
			}
			var fv flex.View
			got := false
			fv.Draw(canvas, dscore.VivaTheme(theme), ui.Rect{Width: 900, Height: 700}, func(f *flex.Frame) {
				got = p.drawProductTranscript(f, env, true)
			})
			if got != c.want {
				t.Fatalf("drawProductTranscript = %v，想要 %v", got, c.want)
			}
		})
	}
}

// 编译期：字宽测量器必须与画布一致（避免测试里出现"看起来画了但宽度是 0"）。
var _ = utf8.RuneCountInString
