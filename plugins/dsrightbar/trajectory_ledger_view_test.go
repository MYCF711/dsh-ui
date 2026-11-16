//go:build darwin || windows || linux

package dsrightbar

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func ledgerFrame(seq int, kind, data string, ms int64) apiclient.Frame {
	return apiclient.Frame{Seq: seq, Type: kind, Data: json.RawMessage(data), Time: ms}
}

func ledgerViewFrames() []apiclient.Frame {
	return []apiclient.Frame{
		ledgerFrame(1, "system/message", `{"turn":0,"step":0,"message":{"role":"system","content":[{"type":"text","text":"system prompt"}]}}`, 1000),
		ledgerFrame(2, "turn/start", `{"turn":1}`, 1100),
		ledgerFrame(3, "user/message", `{"id":"u1","turn":1,"step":0,"source":{"kind":"user"},"content":[{"type":"text","text":"question"}]}`, 1200),
		ledgerFrame(4, "step/start", `{"turn":1,"step":1}`, 1300),
		ledgerFrame(5, "request/header", `{"turn":1,"step":1,"header":{"system":"system prompt","config":{"model":"model-a"},"tools":[{"name":"run_code","description":"code","parameters":{"type":"object"}}]}}`, 1400),
		ledgerFrame(6, "assistant/text", `{"turn":1,"step":1,"text":"answer"}`, 1500),
		ledgerFrame(7, "assistant/message", `{"turn":1,"step":1,"usage":{"inputTokens":10,"outputTokens":5},"message":{"content":[{"type":"text","text":"answer"},{"type":"tool-call","id":"call-1","name":"run_code","arguments":"{\"code\":\"return 1\",\"description\":\"demo\"}"}]}}`, 1800),
		ledgerFrame(8, "tool/call", `{"turn":1,"step":1,"callId":"call-1","name":"run_code","arguments":"{\"code\":\"return 1\",\"description\":\"demo\"}"}`, 1450),
		ledgerFrame(9, "tool/result", `{"turn":1,"step":1,"message":{"source":{"callId":"call-1"},"content":[{"type":"text","text":"1"}],"isError":false}}`, 1900),
		ledgerFrame(10, "step/end", `{"turn":1,"step":1,"reason":{"kind":"stop"}}`, 2000),
		ledgerFrame(11, "turn/end", `{"turn":1,"time":2000}`, 2000),
	}
}

func openLedgerView(t *testing.T, frames []apiclient.Frame, act pluginapi.Actions) (*headless.Session, *Plugin, *pluginapi.Snapshot) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	data := &pluginapi.Snapshot{Cur: "ledger-view", Frames: frames, History: pluginapi.HistoryPaging{Baseline: true}}
	tree := &gfx.Tree{}
	feat := tree.Feature("ledger-view-test")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "ledger-view-test", Width: 1000, Height: 720, Tree: tree,
		TextInput: ui.Fields(p.Fields()...)}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{Theme: theme, Data: *data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Act: act, Has: func(pluginapi.Slot) bool { return true }}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) { p.drawTimeline(f, env, *data, "ledger-test-", false) })
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session, p, data
}

func pumpLedgerUntil(t *testing.T, session *headless.Session, condition func() bool, label string) {
	t.Helper()
	for i := 0; i < 30; i++ {
		if condition() {
			return
		}
		if err := session.Resize(1000, 720); err != nil {
			t.Fatal(err)
		}
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if condition() {
		return
	}
	t.Fatalf("pump 后条件仍不成立：%s；texts=%q", label, session.Texts())
}

func TestLedgerViewFullSurfaceAndRequestInspector(t *testing.T) {
	session, _, _ := openLedgerView(t, ledgerViewFrames(), pluginapi.Actions{})
	pumpLedgerUntil(t, session, func() bool {
		return session.HasText("轮次") && session.HasText("调用") && session.HasText("子调用") && session.HasText("实际时间")
	}, "完整工具栏")
	for _, text := range []string{"系统", "用户", "助手", "工具", "请求 #1"} {
		if !session.HasText(text) {
			t.Fatalf("完整账本缺 %q：%q", text, session.Texts())
		}
	}
	if err := session.ClickLabel("请求 #1"); err != nil {
		t.Fatal(err)
	}
	pumpLedgerUntil(t, session, func() bool { return session.HasText("事件详情") && session.HasText("选项") }, "请求 Inspector")
	if err := session.ClickLabel("选项"); err != nil {
		t.Fatal(err)
	}
	pumpLedgerUntil(t, session, func() bool { return session.HasText("model-a") }, "Options 真值")
	if err := session.ClickLabel("用量"); err != nil {
		t.Fatal(err)
	}
	pumpLedgerUntil(t, session, func() bool { return session.HasText("本次请求") && session.HasText("会话累计") }, "Usage 两组")
}

func TestLedgerViewSearchANDAndGlobalFold(t *testing.T) {
	session, p, _ := openLedgerView(t, ledgerViewFrames(), pluginapi.Actions{})
	pumpLedgerUntil(t, session, func() bool { return session.HasText("answer") }, "账本首帧")
	if err := session.ClickLabel("轮次"); err != nil {
		t.Fatal(err)
	}
	pumpLedgerUntil(t, session, func() bool { return session.HasText("已收起轮次") }, "Turn 全局折叠")
	p.searchField.SetText("answer missing-token")
	pumpLedgerUntil(t, session, func() bool { return session.HasText("没有匹配") }, "AND 查询无匹配")
	if session.HasText("已收起轮次") {
		t.Fatal("搜索必须绕过折叠")
	}
	p.searchField.SetText("answer run_code")
	pumpLedgerUntil(t, session, func() bool { return session.HasText("answer") && session.HasText("run_code") }, "AND 查询命中")
}

func TestLedgerTimelineModesZoomAndEscape(t *testing.T) {
	model := &ledgerModel{Records: []*ledgerRecord{
		{ID: "a", Kind: ledgerMessage},
		{ID: "b", Kind: ledgerTool},
	}}
	sequence := deriveLedgerTimeline(model, "sequence")
	if len(sequence.Spans) != 2 || sequence.End != 2 {
		t.Fatalf("sequence = %+v", sequence)
	}
	p := &Plugin{}
	s := p.ledgerState("zoom")
	s.timelineRect = gfx.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	s.timelineFull = ledgerRange{Start: 0, End: 10}
	s.timelineMode = "sequence"
	result := p.HandleScroll(gfx.ScrollEvent{X: 50, Y: 20, DeltaY: -1000})
	if !result.Handled || !result.Repaint || s.viewport == nil {
		t.Fatalf("滚轮未缩放：%+v viewport=%+v", result, s.viewport)
	}
	if got := s.viewport.End - s.viewport.Start; got >= 10 || got < 4 {
		t.Fatalf("缩放 domain=%f，应在 [4,10)", got)
	}
	selection := ledgerRange{Start: 1, End: 2}
	s.selection = &selection
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) || s.selection != nil {
		t.Fatal("Esc 未清除 timeline range")
	}
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("无 range 时不得吞 Esc")
	}
}

func TestLedgerSearchBypassesButPreservesFoldState(t *testing.T) {
	s := &ledgerViewState{collapsedTurns: map[int]bool{1: true}, collapsedAssistant: map[string]bool{}, collapsedTools: map[string]bool{}}
	r := &ledgerRecord{ID: "m", Kind: ledgerMessage, Turn: 1, Group: "第 1 步", Text: "answer"}
	rows := buildLedgerRows(&ledgerModel{Turns: []*ledgerTurn{{Turn: 1, Records: []*ledgerRecord{r}}}, Records: []*ledgerRecord{r}}, map[string]bool{"m": true}, nil, true)
	if len(rows) != 3 { // Turn + group + record；不追加 collapsed summary。
		t.Fatalf("搜索行数=%d：%+v", len(rows), rows)
	}
	if !s.collapsedTurns[1] {
		t.Fatal("搜索清空了折叠状态")
	}
}

func TestLedgerViewLoadOlderUsesResidentThenHost(t *testing.T) {
	frames := make([]apiclient.Frame, 0, 55)
	for i := 0; i < 55; i++ {
		frames = append(frames, ledgerFrame(i+1, "user/message", fmt.Sprintf(`{"id":"u%d","turn":%d,"step":0,"source":{"kind":"user"},"content":[{"type":"text","text":"question-%d"}]}`, i, i+1, i), int64(1000+i)))
	}
	calls := 0
	session, p, data := openLedgerView(t, frames, pluginapi.Actions{LoadOlder: func() { calls++ }})
	pumpLedgerUntil(t, session, func() bool { return session.HasText("加载更早的历史") }, "resident LoadOlder")
	if err := session.ClickLabel("加载更早的历史"); err != nil {
		t.Fatal(err)
	}
	pumpLedgerUntil(t, session, func() bool { return p.ledgerStates[data.Cur].historyLimit == 100 }, "resident 增页")
	if calls != 0 {
		t.Fatalf("resident 尚有记录时不应请求宿主：%d", calls)
	}
	data.History.HasMore = true
	p.ledgerStates[data.Cur].historyLimit = 1000
	pumpLedgerUntil(t, session, func() bool { return session.HasText("加载更早的历史") }, "host LoadOlder")
	if err := session.ClickLabel("加载更早的历史"); err != nil {
		t.Fatal(err)
	}
	pumpLedgerUntil(t, session, func() bool { return calls == 1 }, "调用宿主 LoadOlder")
}

func TestLedgerPanelsUseOnlyOfficialConversationSeat(t *testing.T) {
	p := &Plugin{}
	for _, panel := range p.Panels() {
		if panel.Slot == pluginapi.SlotMainBody {
			t.Fatalf("完整账本不得再注册旧三栏视口偏移桥：%+v", panel)
		}
	}
	found := false
	for _, panel := range p.Panels() {
		if panel.Slot == pluginapi.SlotConvView && panel.Key == pluginapi.ViewTrajectory && panel.Order == 10 {
			found = true
		}
	}
	if !found {
		t.Fatal("缺 conversation.view/trajectory/order:10")
	}
}

func TestLedgerTTFTPortionRejectsIncompleteTiming(t *testing.T) {
	start, first, end := int64(100), int64(150), int64(250)
	r := &ledgerRecord{Kind: ledgerMessage, StartedAt: &start, FirstTokenAt: &first, CompletedAt: &end}
	portion, ok := ledgerTTFTPortion(r)
	if !ok || portion < 0.32 || portion > 0.34 {
		t.Fatalf("TTFT 段=%f,%v", portion, ok)
	}
	r.CompletedAt = nil
	if _, ok := ledgerTTFTPortion(r); ok {
		t.Fatal("运行中 assistant 不得伪造解码段")
	}
}

func TestLedgerDetailRunCodeAndRawResultUnavailable(t *testing.T) {
	r := &ledgerRecord{Kind: ledgerTool, Text: "run_code", Input: `{"code":"const x = 1\nreturn x","description":"demo"}`, Output: "1"}
	tabs := ledgerDetailTabs(r, nil)
	foundCode := false
	for _, tab := range tabs {
		foundCode = foundCode || tab.Label == "代码"
	}
	if !foundCode {
		t.Fatalf("run_code 缺代码 tab：%+v", tabs)
	}
	input := ledgerDetailText(r, nil, "input", nil)
	if !strings.Contains(input, "   1  const x = 1") || !strings.Contains(input, "原始 JSON") {
		t.Fatalf("run_code 详情=%q", input)
	}
	output := ledgerDetailText(r, nil, "output", nil)
	if !strings.Contains(output, "渲染结果") || !strings.Contains(output, "原始结果仅在实时工具调用中可用") {
		t.Fatalf("工具结果双区=%q", output)
	}
}

func TestLedgerWindowPrependsKeepIdentity(t *testing.T) {
	one := &ledgerRecord{ID: "stable", Kind: ledgerMessage}
	model := &ledgerModel{Records: []*ledgerRecord{one}, Turns: []*ledgerTurn{{Turn: 1, Records: []*ledgerRecord{one}}}}
	if got := windowLedger(model, 50).Records[0]; got != one {
		t.Fatal("窗口投影复制了 record，稳定身份被破坏")
	}
}

func TestLedgerDoubleClickWindow(t *testing.T) {
	s := &ledgerViewState{lastClick: map[string]time.Time{}}
	if ledgerDoubleClick(s, "assistant:a") {
		t.Fatal("第一次点击被当成双击")
	}
	if !ledgerDoubleClick(s, "assistant:a") {
		t.Fatal("第二次点击未折叠")
	}
}

func TestLedgerUsageDoesNotInventMissingBuckets(t *testing.T) {
	output := int64(5)
	text := ledgerRequestUsageText(&ledgerUsage{Output: &output})
	if !strings.Contains(text, "输出") || strings.Contains(text, "输入") || strings.Contains(text, "缓存") || strings.Contains(text, "推理") {
		t.Fatalf("缺桶被补成 0：%q", text)
	}
	if got := ledgerRequestUsageText(&ledgerUsage{}); got != "未报告用量" {
		t.Fatalf("全缺 usage=%q", got)
	}
	reasoning := int64(3)
	if got := ledgerRequestUsageText(&ledgerUsage{Reasoning: &reasoning}); strings.Contains(got, "内容") {
		t.Fatalf("只有 reasoning 时伪造 content 0：%q", got)
	}
}

func TestLedgerRunningAndSettledEmptyStates(t *testing.T) {
	assistant := &ledgerRecord{Kind: ledgerMessage}
	if got := ledgerRecordStatus(assistant); got != "等待中" {
		t.Fatalf("partial assistant=%q", got)
	}
	now := int64(1000)
	assistant.CompletedAt = &now
	if got := ledgerRecordStatus(assistant); got != "已完成" {
		t.Fatalf("final assistant=%q", got)
	}
	tool := &ledgerRecord{Kind: ledgerTool, CompletedAt: &now}
	if got := ledgerRecordStatus(tool); got != "已完成" {
		t.Fatalf("空结果工具被当成 running：%q", got)
	}
	if got := ledgerDetailText(tool, nil, "output", nil); !strings.Contains(got, "无输出") {
		t.Fatalf("settled empty result=%q", got)
	}
	found := false
	for _, tab := range ledgerDetailTabs(tool, nil) {
		found = found || tab.ID == "output"
	}
	if !found {
		t.Fatal("成功的空结果丢 Result tab")
	}
}

func TestLedgerRunCodeOptionalDescriptionAndEmptyCode(t *testing.T) {
	for _, input := range []string{`{"code":"return 1"}`, `{"code":""}`} {
		r := &ledgerRecord{Kind: ledgerTool, Text: "run_code", Input: input}
		if _, _, ok := ledgerCodeProgram(r); !ok {
			t.Fatalf("合法 run_code 被排除：%s", input)
		}
	}
	r := &ledgerRecord{Kind: ledgerTool, Text: "run_code", Input: `{"code":"return 1","description":123}`}
	if _, _, ok := ledgerCodeProgram(r); ok {
		t.Fatal("非字符串 description 不得进入 run_code 特化")
	}
}

func TestLedgerRequestOnlyIsNinePixelRequestAnchor(t *testing.T) {
	req := &ledgerRequest{ID: "assistant:1:1", Number: 1}
	r := &ledgerRecord{ID: "assistant-anchor", Kind: ledgerMessage, RequestOnly: true, Request: req, Turn: 1, Group: "Step 1"}
	model := &ledgerModel{Records: []*ledgerRecord{r}, Turns: []*ledgerTurn{{Turn: 1, Records: []*ledgerRecord{r}}}}
	rows := buildLedgerRows(model, nil, nil, false)
	if len(rows) != 2 || rows[1].Kind != ledgerRowRequest || rows[1].Height != 9 {
		t.Fatalf("requestOnly 行形状=%+v", rows)
	}
	if got := ledgerContentRecordCount(model); got != 0 {
		t.Fatalf("requestOnly 被计成 assistant 内容：%d", got)
	}
}

func TestLedgerAttemptOnlyRequestUsesDurableStreamTiming(t *testing.T) {
	start, first, completed := int64(1000), int64(1250), int64(1600)
	req := &ledgerRequest{ID: "assistant:1:1", Number: 1, StartedAt: &start, CompletedAt: &completed}
	r := &ledgerRecord{Kind: ledgerMessage, RequestOnly: true, Request: req, StartedAt: &start, FirstTokenAt: &first, CompletedAt: &completed}
	model := &ledgerModel{Records: []*ledgerRecord{r}}
	text := ledgerDetailText(nil, req, "timing", model)
	if !strings.Contains(text, "首 token 延迟\n250 毫秒") || !strings.Contains(text, "生成\n350 毫秒") {
		t.Fatalf("attempt-only TTFT/decode 丢失：%q", text)
	}
}
