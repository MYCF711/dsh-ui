// plugin_test.go —— 无头测试：真画一帧（ui.NewPage + gfx.Canvas），点真的 Hit id，
// 断言画面文字与真实动作调用。
//
// 官方证据全部在 plugin.go 的常量注释里（OFF: 行号 + 对应哪个 CSS 值/哪一条文案）。
//
// 三条本仓的测试规矩：
//
//	① `ui.Box` 只在本次 paint 内有效 → 断言只对**画布上的文字**与**记录下来的调用**做；
//	② 异步结果**不许立刻读**：动作都要 pump 到条件成立（`props.Act` 是同步假实现时
//	   条件在同一帧里成立；产品路径换成 goroutine 后同一套 pump 写法仍然成立）；
//	③ 弹层（`ui.Menu` 的四项）由 viva 自己持有身份，测试点不到——所以菜单**声明**
//	   用画布文字断言，菜单**选中之后**的状态变更走 `chooseLocation` 这个真接缝。
package dscreationreco

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsactioncards"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// recorder 记下这一帧的配置读写（官方 localStorage 的等价动作）。
type recorder struct {
	values map[string]string
	writes []map[string]string
}

// fakeEnv 造一份带真实主题、真实交互树、可记录配置读写的环境。
func fakeEnv(t *testing.T, prefs map[string]string) (*Plugin, *pluginapi.Env, *gfx.Canvas, *recorder) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("creation-reco-test")
	rec := &recorder{values: prefs}
	p := &Plugin{theme: theme}
	env := &pluginapi.Env{
		Theme: theme,
		Tree:  tree,
		Feat:  feat,
		Data:  pluginapi.Snapshot{Cur: "s1"},
	}
	env.Act.ConfigGet = func() map[string]string {
		out := make(map[string]string, len(rec.values))
		for key, value := range rec.values {
			out[key] = value
		}
		return out
	}
	env.Act.ConfigSet = func(kv map[string]string) {
		rec.writes = append(rec.writes, kv)
		if rec.values == nil {
			rec.values = map[string]string{}
		}
		for key, value := range kv {
			rec.values[key] = value
		}
	}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	return p, env, canvas, rec
}

// raw 把一段 JSON 文本转成 props 里的原始值。
func raw(text string) json.RawMessage { return json.RawMessage(text) }

// baseProps 造一张卡（官方 proposal 的逐字段投影）。
func baseProps() pluginapi.ActionCardBodyProps {
	return pluginapi.ActionCardBodyProps{
		SessionID: "s1",
		CallID:    "call-1",
		Action:    Action,
		Title:     "验收清单",
		Brief:     "在当前会话右侧展示验收清单。",
		Details:   map[string]json.RawMessage{"slot": raw(`"sidebar-right"`)},
	}
}

// withDecision 给 props 挂一条决定。
func withDecision(props pluginapi.ActionCardBodyProps, phase string, data map[string]json.RawMessage) pluginapi.ActionCardBodyProps {
	props.HasDecision = true
	props.Phase = phase
	props.Data = data
	return props
}

// drawFrame 画一帧并返回画布。clickID 非空时把这一帧当成"点了那个 id"。
func drawFrame(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, props pluginapi.ActionCardBodyProps, clickID string) {
	t.Helper()
	canvas.Reset()
	if clickID != "" {
		canvas.BeginEventPass(env.Tree.ID(env.Feat, clickID), gfx.ClickInfo{})
		defer canvas.EndEventPass()
	}
	pageTheme := dscore.VivaTheme(env.Theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 640, Height: 560}, func(f ui.C) { p.Draw(f, env, props) })
}

func texts(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

// pumpUntil 反复画帧直到条件成立（异步结果落地后必须 pump，不许立刻读）。
func pumpUntil(t *testing.T, done func() bool, draw func()) {
	t.Helper()
	for i := 0; i < 8; i++ {
		if done() {
			return
		}
		draw()
	}
	if !done() {
		t.Fatal("pump 8 帧后条件仍不成立")
	}
}

// ── 1. 装配：正文真的进了动作卡框架的注册表 ─────────────────────────────────

func TestRegistersIntoActionCardRegistry(t *testing.T) {
	kernel := uikernel.New("creation-reco-registry-test")
	cards := &dsactioncards.Plugin{}
	reco := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, cards, reco); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	body := cards.BodyFor(Action)
	if body == nil {
		t.Fatalf("注册表里没有 %q 这一格", Action)
	}
	if body.Action() != Action {
		t.Fatalf("注册进来的是 %q，应为 %q", body.Action(), Action)
	}
}

// ── 2. 新卡：标题 + 说明块三句 + 预填内容 + 位置行 + 两个按钮 ────────────────

func TestNewCardDrawsOfficialFrame(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	props.Details["initialContent"] = raw(`"1. 导出成功\n2. 断网提示清楚"`)

	drawFrame(t, p, env, canvas, props, "")

	for _, want := range []string{
		zhHeading, zhLocation, locationLabel(slotSidebarRight),
		zhCreationMethod, zhCreationCost, zhCreationBackground,
		zhInitialContent, "1. 导出成功\n2. 断网提示清楚",
		zhDismiss, zhCreate, "验收清单",
	} {
		if !hasText(canvas, want) {
			t.Fatalf("画面缺少 %q；实际=%v", want, texts(canvas))
		}
	}
	// 还没定局：不该有 phase 状态行，也不该有 placement 那一行。
	for _, text := range texts(canvas) {
		if text == zhInstalled || text == zhCreating || text == zhUndo {
			t.Fatalf("新卡不该画 %q", text)
		}
	}
}

// ── 3. 说明块：默认展开，点了就收起并落盘（官方 infoDismissed）──────────────

func TestInfoNoticeTogglesAndPersists(t *testing.T) {
	p, env, canvas, rec := fakeEnv(t, nil)
	props := baseProps()
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhCreationCost) {
		t.Fatalf("可重建的卡默认要展开说明块；实际=%v", texts(canvas))
	}

	drawFrame(t, p, env, canvas, props, "creation-info")
	drawFrame(t, p, env, canvas, props, "")
	if hasText(canvas, zhCreationCost) {
		t.Fatalf("点过 info 之后说明块应当收起；实际=%v", texts(canvas))
	}
	if len(rec.writes) == 0 || rec.writes[0][prefInfoDismissed] != "1" {
		t.Fatalf("收起说明块必须落盘 %s=1；实际=%v", prefInfoDismissed, rec.writes)
	}
	// 再点一次重新展开。
	drawFrame(t, p, env, canvas, props, "creation-info")
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhCreationCost) {
		t.Fatal("再点一次 info 应当重新展开说明块")
	}
}

// ── 4. 持久偏好：读过一次 `1` 之后，说明块默认收起 ───────────────────────────

func TestDismissedNoticeStaysClosed(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, map[string]string{prefInfoDismissed: "1"})
	props := baseProps()
	drawFrame(t, p, env, canvas, props, "")
	if hasText(canvas, zhCreationCost) {
		t.Fatalf("偏好说已消费，说明块不该默认展开；实际=%v", texts(canvas))
	}
}

// ── 5. 位置菜单：四项 + 每项第二行 hint + 点过之后按钮文字跟着变 ──────────────

func TestLocationMenuDeclaresOfficialChoices(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	drawFrame(t, p, env, canvas, props, "")
	if p.menuOpen == nil {
		t.Fatal("画过一帧之后菜单开关必须已经分配")
	}
	*p.menuOpen = true
	drawFrame(t, p, env, canvas, props, "")
	for _, want := range []string{
		locationLabel(slotSidebarRight), locationLabel(slotPage),
		locationLabel(slotSidebarLeft), locationLabel(choiceCustom),
		zhSidebarRightHint, zhPageHint, zhSidebarLeftHint, zhCustomHint,
	} {
		if !hasText(canvas, want) {
			t.Fatalf("菜单缺少 %q；实际=%v", want, texts(canvas))
		}
	}
	// 官方顺序：右侧栏 → 会话 Tab → 全局页面 → 自行撰写（OFF:Card.tsx:54）。
	want := []string{zhSlotSidebarRight, zhSlotPage, zhSlotSidebarLeft, zhChoiceCustom}
	// 触发按钮上也是「右侧栏」，所以先去掉**连续重复**（按钮文字 + 菜单第一项）。
	var got []string
	for _, text := range texts(canvas) {
		for _, candidate := range want {
			if text == candidate && (len(got) == 0 || got[len(got)-1] != text) {
				got = append(got, text)
			}
		}
	}
	if len(got) < len(want) {
		t.Fatalf("四项没画全：%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("菜单顺序 %v，应为 %v", got, want)
		}
	}
}

// ── 6. 创建：动词 + placement 落在自己的 callId 上 ───────────────────────────

func TestCreateSendsSlotPlacement(t *testing.T) {
	p, env, canvas, rec := fakeEnv(t, nil)
	props := baseProps()
	var verbs []string
	var payloads []map[string]json.RawMessage
	props.Act = func(verb string, payload map[string]json.RawMessage) {
		verbs = append(verbs, verb)
		payloads = append(payloads, payload)
	}
	drawFrame(t, p, env, canvas, props, "")
	p.chooseLocation(slotSidebarLeft)
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "creation-create")

	if len(verbs) != 1 || verbs[0] != verbCreate {
		t.Fatalf("动词 = %v，应为 [%s]", verbs, verbCreate)
	}
	if len(payloads[0]) != 1 || string(payloads[0]["slot"]) != `"sidebar-left"` {
		t.Fatalf("placement 应当是 {slot:sidebar-left}；实际=%v", payloads[0])
	}
	if _, ok := payloads[0]["customLocation"]; ok {
		t.Fatal("没选「自行撰写」时不许带 customLocation")
	}
	// 点创建等于消费掉那条说明（官方 :92）。
	if got := rec.writes[len(rec.writes)-1][prefInfoDismissed]; got != "1" {
		t.Fatalf("点创建也要落盘偏好；实际=%v", rec.writes)
	}
}

// ── 7. 自行撰写：输入框出现、空着时创建按钮不可点、非空带 trim 后的位置 ─────

func TestCustomLocationDrivesTextareaAndPayload(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	var gotPayload map[string]json.RawMessage
	props.Act = func(_ string, payload map[string]json.RawMessage) { gotPayload = payload }

	drawFrame(t, p, env, canvas, props, "")
	p.chooseLocation(choiceCustom)
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhCustomLabel) {
		t.Fatalf("选了自行撰写之后要画输入框标签；实际=%v", texts(canvas))
	}
	// 官方 `disabled={busy || location==='custom' && customLocation.trim()===''}`：
	// 空输入时点创建不该发出任何动作。
	drawFrame(t, p, env, canvas, props, "creation-create")
	if gotPayload != nil {
		t.Fatalf("输入框空着时创建必须不可点，却发出了 %v", gotPayload)
	}
	p.customLocation = "  左侧入口，全屏日历  "
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "creation-create")
	if gotPayload == nil || string(gotPayload["customLocation"]) != `"左侧入口，全屏日历"` {
		t.Fatalf("placement.customLocation 必须是 trim 后的值；实际=%v", gotPayload)
	}
	if string(gotPayload["slot"]) != `"sidebar-right"` {
		t.Fatalf("选了自定义位置时 slot 仍是推荐值；实际=%v", gotPayload)
	}
}

// ── 8. refused{creator-unavailable} → 官方 unavailable 文案（不是成功态）────

func TestCreatorUnavailableShowsOfficialUnavailableCopy(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	props.Act = func(string, map[string]json.RawMessage) {}
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "creation-create")

	// 宿主真实返回：refused（HOST:internal/creation/service.go 的 ReasonCreatorUnavailable）。
	props.Acting = false
	props.LastResult = &pluginapi.ActionCardResultInfo{
		Status: "refused", Reason: "creator-unavailable",
	}
	drawFrame(t, p, env, canvas, props, "")

	if !hasText(canvas, zhUnavailable) {
		t.Fatalf("creator-unavailable 必须显示官方兜底文案 %q；实际=%v", zhUnavailable, texts(canvas))
	}
	// 绝不画成成功：没有「已安装」，也没有「正在后台创建」。
	for _, banned := range []string{zhInstalled, zhCreating, zhInstalling, zhKept} {
		if hasText(canvas, banned) {
			t.Fatalf("被拒绝的创建画出了 %q", banned)
		}
	}
	// 还能重试：按钮与位置行仍在。
	if !hasText(canvas, zhCreate) {
		t.Fatalf("被拒绝之后仍然要能重试；实际=%v", texts(canvas))
	}
}

// ── 9. refused 的八条专属文案（含 invalid-location）────────────────────────

func TestRefusedReasonsUseTheirOwnCopy(t *testing.T) {
	reasons := map[string]string{
		"unknown":          zhUnknown,
		"access-denied":    zhAccessDenied,
		"not-live-root":    zhNotLiveRoot,
		"closing":          zhClosing,
		"unavailable":      zhUnavailable,
		"invalid-location": zhInvalidLocation,
		"read-only":        zhReadOnly,
		"source-missing":   zhSourceMissing,
	}
	for reason, want := range reasons {
		reason, want := reason, want
		t.Run(reason, func(t *testing.T) {
			p, env, canvas, _ := fakeEnv(t, nil)
			props := baseProps()
			props.LastResult = &pluginapi.ActionCardResultInfo{Status: "refused", Reason: reason}
			drawFrame(t, p, env, canvas, props, "")
			if !hasText(canvas, want) {
				t.Fatalf("reason=%s 缺 %q；实际=%v", reason, want, texts(canvas))
			}
		})
	}
}

// ── 10. 动作失败（回复丢失/调用失败）→ 官方 transport 文案 ──────────────────

func TestActErrorShowsTransportCopy(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	props.ActError = "Post \"http://127.0.0.1:1/api/actionCards.act\": connectex: 拒绝连接"
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhTransport) {
		t.Fatalf("动作失败要画官方 transport 文案；实际=%v", texts(canvas))
	}
	if hasText(canvas, "拒绝连接") {
		t.Fatal("宿主错误原文不许进这张卡（58 条以外没有官方文案）")
	}
}

// ── 11. 11 档 phase 逐档文案 ────────────────────────────────────────────────

func TestEveryPhaseHasItsOfficialCopy(t *testing.T) {
	phases := map[string]string{
		"dismissed": zhDismissed, "replaced": zhReplaced, "creating": zhCreating,
		"installing": zhInstalling, "cancelled": zhCancelled, "installed": zhInstalled,
		"kept": zhKept, "removed": zhRemoved, "failed": zhFailed,
		"interrupted": zhInterrupted, "unconfirmed": zhUnconfirmed,
	}
	if len(phases) != 11 {
		t.Fatalf("官方 PHASE_KEYS 是 11 档，这里只有 %d", len(phases))
	}
	for phase, want := range phases {
		phase, want := phase, want
		t.Run(phase, func(t *testing.T) {
			p, env, canvas, _ := fakeEnv(t, nil)
			props := withDecision(baseProps(), phase, map[string]json.RawMessage{"slot": raw(`"sidebar-right"`)})
			drawFrame(t, p, env, canvas, props, "")
			if !hasText(canvas, want) {
				t.Fatalf("phase=%s 缺 %q；实际=%v", phase, want, texts(canvas))
			}
		})
	}
}

// ── 12. creating 档：取消创建（不带 payload）───────────────────────────────

func TestCreatingPhaseOffersCancel(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := withDecision(baseProps(), "creating", map[string]json.RawMessage{"slot": raw(`"sidebar-right"`)})
	var verbs []string
	var payloads []map[string]json.RawMessage
	props.Act = func(verb string, payload map[string]json.RawMessage) {
		verbs = append(verbs, verb)
		payloads = append(payloads, payload)
	}
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhCancel) {
		t.Fatalf("creating 档要给「取消创建」；实际=%v", texts(canvas))
	}
	if hasText(canvas, zhCreate) || hasText(canvas, zhDismiss) {
		t.Fatal("已经开始的卡不该再给创建/暂不需要")
	}
	drawFrame(t, p, env, canvas, props, "creation-cancel")
	if len(verbs) != 1 || verbs[0] != verbCancel {
		t.Fatalf("动词 = %v，应为 [%s]", verbs, verbCancel)
	}
	if payloads[0] != nil {
		t.Fatalf("非 create/dismiss 的动词不许带 payload；实际=%v", payloads[0])
	}
}

// ── 13. installed / kept → 撤销；removed → 恢复 ───────────────────────────

func TestInstalledAndRemovedPhases(t *testing.T) {
	for _, phase := range []string{"installed", "kept"} {
		phase := phase
		t.Run(phase, func(t *testing.T) {
			p, env, canvas, _ := fakeEnv(t, nil)
			props := withDecision(baseProps(), phase, map[string]json.RawMessage{"slot": raw(`"sidebar-left"`)})
			var verb string
			props.Act = func(got string, _ map[string]json.RawMessage) { verb = got }
			drawFrame(t, p, env, canvas, props, "")
			if !hasText(canvas, zhUndo) {
				t.Fatalf("%s 档要给「撤销」；实际=%v", phase, texts(canvas))
			}
			if !hasText(canvas, zhLocation+" · "+zhSlotSidebarLeft) {
				t.Fatalf("%s 档要画「添加到 · 全局页面」；实际=%v", phase, texts(canvas))
			}
			drawFrame(t, p, env, canvas, props, "creation-undo")
			if verb != verbUndo {
				t.Fatalf("动词 = %q，应为 %q", verb, verbUndo)
			}
		})
	}
	t.Run("removed", func(t *testing.T) {
		p, env, canvas, _ := fakeEnv(t, nil)
		props := withDecision(baseProps(), "removed", map[string]json.RawMessage{"slot": raw(`"page"`)})
		var verb string
		props.Act = func(got string, _ map[string]json.RawMessage) { verb = got }
		drawFrame(t, p, env, canvas, props, "")
		if !hasText(canvas, zhRestore) {
			t.Fatalf("removed 档要给「恢复」；实际=%v", texts(canvas))
		}
		drawFrame(t, p, env, canvas, props, "creation-restore")
		if verb != verbRestore {
			t.Fatalf("动词 = %q，应为 %q", verb, verbRestore)
		}
	})
}

// ── 14. 应用结果行：四档专属文案，且 failed phase 不叠加 ────────────────────

func TestApplicationOutcomeCopy(t *testing.T) {
	cases := []struct {
		application string
		phase       string
		want        string
	}{
		{"restart-required", "installed", zhRestartRequired},
		{"overridden", "installed", zhOverridden},
		{"failed", "installed", zhApplicationFailed},
		{"cancelled", "removed", zhApplicationCancel},
	}
	for _, c := range cases {
		c := c
		t.Run(c.application, func(t *testing.T) {
			p, env, canvas, _ := fakeEnv(t, nil)
			data := map[string]json.RawMessage{
				"slot": raw(`"sidebar-right"`), "application": raw(`"` + c.application + `"`),
			}
			props := withDecision(baseProps(), c.phase, data)
			drawFrame(t, p, env, canvas, props, "")
			if !hasText(canvas, c.want) {
				t.Fatalf("application=%s 缺 %q；实际=%v", c.application, c.want, texts(canvas))
			}
		})
	}
	// applied 不画额外一行；failed phase 也不叠加应用结果（官方 :130-131）。
	t.Run("applied-and-failed-phase", func(t *testing.T) {
		p, env, canvas, _ := fakeEnv(t, nil)
		data := map[string]json.RawMessage{"slot": raw(`"page"`), "application": raw(`"failed"`)}
		props := withDecision(baseProps(), "failed", data)
		drawFrame(t, p, env, canvas, props, "")
		if hasText(canvas, zhApplicationFailed) {
			t.Fatalf("phase=failed 时不该再叠加应用结果行；实际=%v", texts(canvas))
		}
		if !hasText(canvas, zhFailed) {
			t.Fatalf("phase=failed 要画 %q；实际=%v", zhFailed, texts(canvas))
		}
		p2, env2, canvas2, _ := fakeEnv(t, nil)
		props2 := withDecision(baseProps(), "installed",
			map[string]json.RawMessage{"slot": raw(`"page"`), "application": raw(`"applied"`)})
		drawFrame(t, p2, env2, canvas2, props2, "")
		if hasText(canvas2, zhInstalled) == false {
			t.Fatal("installed 档要画 phase 文案")
		}
	})
}

// ── 15. 自定义位置是真实值：placement 那一行画 customLocation 原文 ───────────

func TestPlacementShowsCustomLocationText(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	data := map[string]json.RawMessage{
		"slot": raw(`"sidebar-right"`), "customLocation": raw(`"左侧入口，全屏日历"`),
	}
	props := withDecision(baseProps(), "installed", data)
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhLocation+" · 左侧入口，全屏日历") {
		t.Fatalf("placement 要画用户写的位置原文；实际=%v", texts(canvas))
	}
}

// ── 16. details 坏掉 → 只画 summary（官方 :109）──────────────────────────

func TestInvalidDetailsDrawsSummaryOnly(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	props.Details = map[string]json.RawMessage{"slot": raw(`"not-a-slot"`)}
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, "验收清单") || !hasText(canvas, "在当前会话右侧展示验收清单。") {
		t.Fatalf("坏 details 也要画 summary；实际=%v", texts(canvas))
	}
	if hasText(canvas, zhHeading) || hasText(canvas, zhCreate) || hasText(canvas, zhDismiss) {
		t.Fatalf("坏 details 不许给任何动作；实际=%v", texts(canvas))
	}
}

// ── 17. 长 brief 折叠：出现「展开」，点了变「收起」───────────────────────

func TestLongBriefFoldsWithOfficialLabels(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	props.Brief = strings.Repeat("这是一段很长的说明，用来把折叠阈值撑开。", 20)
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhExpandBrief) {
		t.Fatalf("超长 brief 要出现「展开」；实际=%v", texts(canvas))
	}
	if hasText(canvas, props.Brief) {
		t.Fatal("折叠态不该整段画出来")
	}
	drawFrame(t, p, env, canvas, props, "creation-fold-brief")
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhCollapseBrief) {
		t.Fatalf("展开之后要出现「收起」；实际=%v", texts(canvas))
	}
	if !hasText(canvas, props.Brief) {
		t.Fatal("展开之后要画全文")
	}
}

// ── 18. 换卡：本地状态不串到下一张卡 ───────────────────────────────────────

func TestCardStateResetsOnAnotherCard(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	first := baseProps()
	drawFrame(t, p, env, canvas, first, "")
	p.chooseLocation(slotSidebarLeft)
	p.customLocation = "留下的字"
	drawFrame(t, p, env, canvas, first, "")

	second := baseProps()
	second.CallID = "call-2"
	drawFrame(t, p, env, canvas, second, "")
	if p.selected != "" || p.customLocation != "" {
		t.Fatalf("换卡之后状态没重置：selected=%q custom=%q", p.selected, p.customLocation)
	}
	if !hasText(canvas, locationLabel(slotSidebarRight)) {
		t.Fatalf("新卡应当回到 details 推荐的位置；实际=%v", texts(canvas))
	}
}

// ── 19. 菜单选中之后的状态变更（viva 的弹层身份在框架内部，测试走这个真接缝）──

func TestChooseLocationUpdatesSelection(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	drawFrame(t, p, env, canvas, props, "")
	if p.menuOpen == nil {
		t.Fatal("菜单开关没分配")
	}
	*p.menuOpen = true
	for _, choice := range locationChoices {
		p.chooseLocation(choice)
		if p.selected != choice {
			t.Fatalf("chooseLocation(%q) 之后 selected=%q", choice, p.selected)
		}
		if *p.menuOpen {
			t.Fatalf("选中之后菜单必须收起（官方 setOpen(false)）")
		}
		drawFrame(t, p, env, canvas, props, "")
		if !hasText(canvas, locationLabel(choice)) {
			t.Fatalf("选了 %q 之后按钮上要是它的官方名字；实际=%v", choice, texts(canvas))
		}
	}
}

// ── 20. 还能重试的 phase：创建按钮读作「重新创建」（官方 :146-147）────────────

func TestOpenPhasesRelabelCreateAsRetry(t *testing.T) {
	for _, phase := range []string{"cancelled", "failed", "interrupted", "unconfirmed"} {
		phase := phase
		t.Run(phase, func(t *testing.T) {
			p, env, canvas, _ := fakeEnv(t, nil)
			props := withDecision(baseProps(), phase, map[string]json.RawMessage{"slot": raw(`"sidebar-right"`)})
			drawFrame(t, p, env, canvas, props, "")
			if !hasText(canvas, zhRetry) {
				t.Fatalf("phase=%s 要给「重新创建」；实际=%v", phase, texts(canvas))
			}
			if hasText(canvas, zhCreate) {
				t.Fatalf("phase=%s 已经有决定，按钮不该还是「创建」", phase)
			}
			// 官方 openPhases 的卡仍然给「暂不需要」+ 位置选择。
			if !hasText(canvas, zhDismiss) || !hasText(canvas, zhLocation) {
				t.Fatalf("phase=%s 仍在 openPhases 里，要保留位置行与暂不需要；实际=%v", phase, texts(canvas))
			}
		})
	}
}

// ── 21. 在飞文案：三个动词各有自己的进行时（官方 pending 分支）───────────────

func TestPendingVerbShowsItsOwnProgressCopy(t *testing.T) {
	// create → 开始创建…
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	props.Act = func(string, map[string]json.RawMessage) {}
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "creation-create")
	props.Acting = true
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhStarting) {
		t.Fatalf("创建在飞时要显示 %q；实际=%v", zhStarting, texts(canvas))
	}
	// 结果落地（Acting 落下）之后回到「创建」。
	props.Acting = false
	props.LastResult = &pluginapi.ActionCardResultInfo{Status: "settled"}
	drawFrame(t, p, env, canvas, props, "")
	if hasText(canvas, zhStarting) {
		t.Fatalf("结果落地之后不该还停在 %q；实际=%v", zhStarting, texts(canvas))
	}

	// cancel → 正在取消…
	p2, env2, canvas2, _ := fakeEnv(t, nil)
	creating := withDecision(baseProps(), "creating", map[string]json.RawMessage{"slot": raw(`"page"`)})
	creating.Act = func(string, map[string]json.RawMessage) {}
	drawFrame(t, p2, env2, canvas2, creating, "")
	drawFrame(t, p2, env2, canvas2, creating, "creation-cancel")
	creating.Acting = true
	drawFrame(t, p2, env2, canvas2, creating, "")
	if !hasText(canvas2, zhCancelling) {
		t.Fatalf("取消在飞时要显示 %q；实际=%v", zhCancelling, texts(canvas2))
	}

	// restore → 恢复中…
	p3, env3, canvas3, _ := fakeEnv(t, nil)
	removed := withDecision(baseProps(), "removed", map[string]json.RawMessage{"slot": raw(`"page"`)})
	removed.Act = func(string, map[string]json.RawMessage) {}
	drawFrame(t, p3, env3, canvas3, removed, "")
	drawFrame(t, p3, env3, canvas3, removed, "creation-restore")
	removed.Acting = true
	drawFrame(t, p3, env3, canvas3, removed, "")
	if !hasText(canvas3, zhRestoring) {
		t.Fatalf("恢复在飞时要显示 %q；实际=%v", zhRestoring, texts(canvas3))
	}
}

// ── 22. 动作在飞时菜单不出现（官方 `open && !disabled`）─────────────────────

func TestMenuHiddenWhileBusy(t *testing.T) {
	p, env, canvas, _ := fakeEnv(t, nil)
	props := baseProps()
	drawFrame(t, p, env, canvas, props, "")
	if p.menuOpen == nil {
		t.Fatal("菜单开关没分配")
	}
	*p.menuOpen = true
	props.Acting = true
	drawFrame(t, p, env, canvas, props, "")
	if hasText(canvas, zhPageHint) || hasText(canvas, zhCustomHint) {
		t.Fatalf("动作在飞时不该画位置菜单；实际=%v", texts(canvas))
	}
	// 落地之后菜单恢复（开关状态本身没被改掉）。
	props.Acting = false
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhPageHint) {
		t.Fatalf("空闲时菜单要回来；实际=%v", texts(canvas))
	}
}
