//go:build darwin || windows || linux

package dsrightbar

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const wpd1SessionID = "wpd1-session"

type wpd1Fixture struct {
	data          pluginapi.Snapshot
	width, height int
}

// openWPD1 走真正的 Viva Paint/HitTest/TextInput。Fields 用接口发现，特意让
// 这份测试在旧 Plugin 上也能编译：旧实现应当因没有搜索/Inspect 而行为失败。
func openWPD1(t *testing.T, width, height int, msgs []apiclient.Message) (*headless.Session, *Plugin, *wpd1Fixture) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	fixture := &wpd1Fixture{width: width, height: height, data: pluginapi.Snapshot{
		Cur: wpd1SessionID, Msgs: msgs, Total: len(msgs), MessageMeta: map[string]pubapi.MessageMetadata{},
	}}
	tree := &gfx.Tree{}
	feat := tree.Feature("wpd1-test")
	var view ui.Page
	config := app.Config{Title: "wpd1-test", Width: width, Height: height, Tree: tree}
	if fields, ok := any(p).(interface{ Fields() []*ui.Field }); ok {
		config.TextInput = ui.Fields(fields.Fields()...)
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Data: fixture.data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Scroll: view.ScrollState, Has: func(pluginapi.Slot) bool { return true },
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f ui.C) { p.draw(f, env) })
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session, p, fixture
}

func setWPD1Query(t *testing.T, session *headless.Session, p *Plugin, fixture *wpd1Fixture, query string) {
	t.Helper()
	fields, ok := any(p).(interface{ Fields() []*ui.Field })
	if !ok || len(fields.Fields()) == 0 {
		t.Fatal("旧实现没有可输入的 Trajectory 搜索 Field")
	}
	fields.Fields()[0].SetText(query)
	if err := session.Resize(fixture.width, fixture.height); err != nil {
		t.Fatal(err)
	}
}

func wpd1SearchMessages() []apiclient.Message {
	step7, step8, step9 := int64(7), int64(8), int64(9)
	return []apiclient.Message{
		{Role: "user", TurnID: "turn-alpha", Text: "alpha question"},
		{Role: "tool", TurnID: "turn-alpha", Step: &step7, Tool: "AlphaTool", CallID: "call-alpha",
			Args: `{"command":"Echo MixedCase"}`, Summary: "RESULT-Needle", View: &apiclient.ViewSpec{Badge: "ok"}},
		{Role: "tool", TurnID: "turn-alpha", Step: &step8, Tool: "OtherTool", CallID: "call-other",
			Args: `{"path":"other.txt"}`, Summary: "other output", View: &apiclient.ViewSpec{Badge: "ok"}},
		{Role: "user", TurnID: "turn-beta", Text: "beta question"},
		{Role: "tool", TurnID: "turn-beta", Step: &step9, Tool: "BetaTool", CallID: "call-beta",
			Args: `{"path":"beta.txt"}`, Summary: "beta output", View: &apiclient.ViewSpec{Badge: "error", Body: "permission denied"}},
	}
}

func TestWPD1SearchRealFieldCoversLoadedTurnStepToolArgsResultAndError(t *testing.T) {
	session, p, fixture := openWPD1(t, 560, 760, wpd1SearchMessages())
	if !session.HasText("仅当前已加载窗口") {
		t.Fatalf("没有明确当前窗口边界：%q", session.Texts())
	}
	// 第一次查询必须经真实焦点与 Type，而不是只调比较函数。
	if err := session.ClickLabel("搜索当前窗口"); err != nil {
		t.Fatalf("旧实现没有真实搜索框：%v", err)
	}
	if err := session.Type("tUrN-aLpHa"); err != nil {
		t.Fatal(err)
	}
	if !session.HasText("AlphaTool") || !session.HasText("OtherTool") || session.HasText("BetaTool") {
		t.Fatalf("Turn 大小写不敏感搜索结果错误：%q", session.Texts())
	}

	cases := []struct {
		query string
		want  string
		gone  []string
	}{
		{query: "STEP 7", want: "AlphaTool", gone: []string{"OtherTool", "BetaTool"}},
		{query: "aLpHaToOl", want: "AlphaTool", gone: []string{"OtherTool", "BetaTool"}},
		{query: "echo mixedcase", want: "AlphaTool", gone: []string{"OtherTool", "BetaTool"}},
		{query: "result-needle", want: "AlphaTool", gone: []string{"OtherTool", "BetaTool"}},
		{query: "PERMISSION DENIED", want: "BetaTool", gone: []string{"AlphaTool", "OtherTool"}},
	}
	for _, tc := range cases {
		setWPD1Query(t, session, p, fixture, tc.query)
		if !session.HasText(tc.want) {
			t.Errorf("查询 %q 没有命中 %q：%q", tc.query, tc.want, session.Texts())
		}
		for _, gone := range tc.gone {
			if session.HasText(gone) {
				t.Errorf("查询 %q 仍显示非命中项 %q：%q", tc.query, gone, session.Texts())
			}
		}
	}

	setWPD1Query(t, session, p, fixture, "  \t  ")
	// 空查询恢复既有 A-TRAJ 分组，而不是强制展开所有历史 Turn。
	for _, want := range []string{"第 1 轮 · 2 次工具", "第 2 轮 · 1 次工具", "BetaTool"} {
		if !session.HasText(want) {
			t.Errorf("纯空白查询应恢复全部 Turn/当前 Step，缺 %q：%q", want, session.Texts())
		}
	}
	if err := session.ClickLabel("第 1 轮 · 2 次工具"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"AlphaTool", "OtherTool"} {
		if !session.HasText(want) {
			t.Errorf("空查询后展开 Turn 缺少 %q：%q", want, session.Texts())
		}
	}
	setWPD1Query(t, session, p, fixture, "not-present-xyz")
	if !session.HasText("没有匹配「not-present-xyz」的 Step") {
		t.Fatalf("无结果没有真实显示：%q", session.Texts())
	}
	for _, gone := range []string{"AlphaTool", "OtherTool", "BetaTool"} {
		if session.HasText(gone) {
			t.Fatalf("无结果仍绘制 %q：%q", gone, session.Texts())
		}
	}
	if session.HasText("loadOlder") {
		t.Fatalf("当前窗口闭环不得画 loadOlder：%q", session.Texts())
	}
}

func TestWPD1StepInspectAndTurnSelectionUseRealClicks(t *testing.T) {
	msgs := wpd1SearchMessages()[:3]
	step7, step8 := int64(7), int64(8)
	msgs[1].Step = step7Ptr(step7)
	msgs[2].Step = step7Ptr(step8)
	msgs[1].Args = `{"command":"Echo MixedCase","path":"a.go"}`
	msgs[1].Summary = "first raw line\nsecond raw line"
	msgs[1].View = &apiclient.ViewSpec{Badge: "ok"}
	msgs[2].Summary = "other inspect result"
	session, p, fixture := openWPD1(t, 620, 820, msgs)
	fixtureTime := int64(1762222445123)
	// MessageKey 必须使用当前已加载切片里的原始下标，而不是过滤后的下标。
	fixture.data.MessageMeta[pubapi.MessageKey(wpd1SessionID, msgs[1], 1)] = pubapi.MessageMetadata{Time: fixtureTime}
	if err := session.Resize(620, 820); err != nil {
		t.Fatal(err)
	}

	if err := session.ClickLabel("Turn 1 · Step 7 · AlphaTool"); err != nil {
		t.Fatalf("Step 不可选择/展开：%v；texts=%q", err, session.Texts())
	}
	for _, want := range []string{
		"Inspect", "args", msgs[1].Args, "result", "first raw line", "second raw line",
		"callId", "call-alpha", "time", "1762222445123",
	} {
		if !session.HasText(want) {
			t.Errorf("Inspect 缺少真值 %q：%q", want, session.Texts())
		}
	}
	for _, absent := range []string{"parentCallId", "isError", "error"} {
		if session.HasText(absent) {
			t.Errorf("当前 Message 没有 %s 真值，Inspect 不得编造：%q", absent, session.Texts())
		}
	}

	selection, ok := any(p).(interface{ selectionState(string) (string, string) })
	if !ok {
		t.Fatal("旧实现没有 Turn↔Step 选择状态")
	}
	turnKey, stepKey := selection.selectionState(wpd1SessionID)
	if turnKey == "" || stepKey == "" {
		t.Fatalf("点击 Step 后选择联动为空：turn=%q step=%q", turnKey, stepKey)
	}
	_, firstStepKey := stepKey, stepKey

	if err := session.ClickLabel("Turn 1 · Step 8 · OtherTool"); err != nil {
		t.Fatal(err)
	}
	_, secondStepKey := selection.selectionState(wpd1SessionID)
	if secondStepKey == firstStepKey {
		t.Fatal("选择第二个 Step 后仍停在第一个 Step")
	}
	if err := session.ClickLabel("第 1 轮 · 2 次工具"); err != nil {
		t.Fatal(err)
	}
	selectedTurn, selectedStep := selection.selectionState(wpd1SessionID)
	if selectedTurn != turnKey || selectedStep != firstStepKey {
		t.Fatalf("点击 Turn 未联动到同组首个 Step：turn=%q/%q step=%q/%q", selectedTurn, turnKey, selectedStep, firstStepKey)
	}

	// Turn 再展开后，单步自己的展开状态仍在；点同一 Step 必须收起 Inspect。
	if err := session.ClickLabel("第 1 轮 · 2 次工具"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("Turn 1 · Step 7 · AlphaTool"); err != nil {
		t.Fatal(err)
	}
	if session.HasText("call-alpha") {
		t.Fatalf("再次点击同一 Step 没有收起它自己的 Inspect：%q", session.Texts())
	}
}

func step7Ptr(v int64) *int64 { return &v }

func wpd1ThousandMessages() []apiclient.Message {
	msgs := make([]apiclient.Message, 0, 1001)
	msgs = append(msgs, apiclient.Message{Role: "user", TurnID: "turn-large", Text: "1000 steps"})
	for i := 0; i < 1000; i++ {
		step := int64(i + 1)
		name := fmt.Sprintf("tool-%04d", i)
		msgs = append(msgs, apiclient.Message{
			Role: "tool", TurnID: "turn-large", Step: &step, Tool: name, CallID: "call-" + name,
			Args: `{"index":` + fmt.Sprint(i) + `}`, Summary: "result " + name,
		})
	}
	return msgs
}

func TestWPD1ThousandStepsSearchesAllButLayoutsOnlyVisibleRows(t *testing.T) {
	msgs := wpd1ThousandMessages()
	session, p, fixture := openWPD1(t, 560, 600, msgs)
	countTools := func() int {
		count := 0
		for _, text := range session.Snapshot().Texts {
			if strings.HasPrefix(text.Value, "tool-") {
				count++
			}
		}
		return count
	}
	if got := countTools(); got == 0 || got >= 80 {
		t.Fatalf("1000 条首屏实际布局工具行 = %d；应只布局可见/overscan 行而不是 80/1000 条", got)
	}
	if !session.HasText("1000 次") || !session.HasText("920") {
		t.Fatalf("既有最近 80 条截断真值丢失：%q", session.Texts())
	}

	// 搜索必须扫描全部当前已加载消息，包括被最近 80 条展示上限裁掉的第 0 条。
	setWPD1Query(t, session, p, fixture, "TOOL-0000")
	if got := countTools(); got != 1 || !session.HasText("tool-0000") {
		t.Fatalf("全窗口搜索没有找到最早 Step：layout=%d texts=%q", got, session.Texts())
	}
	if session.HasText("tool-9999") {
		t.Fatalf("单条搜索仍绘制末尾 Step：%q", session.Texts())
	}
}
