// wave3_wiring_test.go —— 壳侧接线的证据（第三波两个父容器的数据面）。
//
// 三件事各自有断言：
//
//	① Provide     ui.apiclient / ui.shell.tier 在 Load **之前** 就位，插件 Register 能取到
//	② attach      取回两个 ServiceKey、把档位写给插件、接上"打开账号设置"（且失败如实回 false）
//	③ feed        账号态与投影**单向**流进运行时；未知 ≠ 未登录（不许写成 signed-out）
package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsagentteam"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsnotifications"
)

// probePlugin 记录 Register 时拿到的注入值（证明 Provide 早于 Load）。
type probePlugin struct {
	apiClient any
	tier      any
	// tierGiven 记录 attach 阶段传进来的档位对象。
	tierGiven *uikernel.ShellTier
	// requires 用来模拟"插件声明了依赖"的情形。
	requires []string
}

func (p *probePlugin) ID() string         { return "probe-wave3" }
func (p *probePlugin) Provides() []string { return nil }
func (p *probePlugin) Requires() []string { return p.requires }

func (p *probePlugin) Register(ctx pluginapi.Context) error {
	p.apiClient = ctx.Inject(keyAPIClient)
	p.tier = ctx.Inject(uikernel.KeyShellTier)
	return nil
}

func (p *probePlugin) AttachShellTier(tier *uikernel.ShellTier) { p.tierGiven = tier }

// TestWave3ProvideHappensBeforeLoad ①：两个键必须在 Load 之前就位。
func TestWave3ProvideHappensBeforeLoad(t *testing.T) {
	w := newWave3Wiring()
	cli := apiclient.New("http://127.0.0.1:1")
	k := uikernel.New("wave3-provide")
	w.provide(k, cli)

	probe := &probePlugin{}
	if err := k.Load(&dscore.Plugin{Dark: true}, probe); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	if probe.apiClient == nil {
		t.Fatal("插件 Register 里 Inject(ui.apiclient) 拿到 nil —— Provide 没有早于 Load")
	}
	if got, ok := probe.apiClient.(*apiclient.Client); !ok || got != cli {
		t.Fatalf("ui.apiclient 应是同一个 *apiclient.Client，得到 %#v", probe.apiClient)
	}
	if probe.tier == nil {
		t.Fatal("插件 Register 里 Inject(ui.shell.tier) 拿到 nil")
	}
	if _, ok := probe.tier.(*uikernel.ShellTier); !ok {
		t.Fatalf("ui.shell.tier 应是 *uikernel.ShellTier，得到 %T", probe.tier)
	}
}

// TestWave3AttachWiresServicesAndTier ②：取回服务、写死档位、把档位传给容器插件。
func TestWave3AttachWiresServicesAndTier(t *testing.T) {
	w := newWave3Wiring()
	k := uikernel.New("wave3-attach")
	w.provide(k, apiclient.New("http://127.0.0.1:1"))
	probe := &probePlugin{}
	if err := k.Load(&dscore.Plugin{Dark: true},
		&dsnotifications.Plugin{}, &dsagentteam.Plugin{}, probe); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	m := &model{kernel: k}
	w.attach(m)

	if w.notices == nil {
		t.Fatal("没有取回 ui.account.notifications（通知控制器）")
	}
	if w.team == nil {
		t.Fatal("没有取回 ui.agent-team（子代理目录/Team 运行时）")
	}
	if !w.tier.Settled() {
		t.Fatal("attach 必须把壳档位写死（否则插件一直按默认档位画）")
	}
	// 本内核里没有 shell.root 占用方 → auto 解析为 official。
	if !w.tier.Official() {
		t.Fatal("没有 shell.root 占用方时档位应是 official")
	}
	if probe.tierGiven != w.tier {
		t.Fatal("attach 没有把档位对象传给实现 AttachShellTier 的插件")
	}
}

// TestWave3OpenAccountReportsRealResult ②b：`SetOpenAccount` 不许把"打不开"说成成功。
func TestWave3OpenAccountReportsRealResult(t *testing.T) {
	w := newWave3Wiring()
	k := uikernel.New("wave3-open-account")
	w.provide(k, nil)
	if err := k.Load(&dscore.Plugin{Dark: true}, &dsnotifications.Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	m := &model{kernel: k} // m.settings == nil：设置面板没装配
	w.attach(m)
	if w.notices.OpenAccount() {
		t.Fatal("没装设置面板时 OpenAccount() 必须回 false（卡片要如实说「还没有」）")
	}
}

// TestWave3FeedAccountStates ③：四种账号输入 → 运行时的三个可见性字段。
func TestWave3FeedAccountStates(t *testing.T) {
	cases := []struct {
		name          string
		known         bool
		signedIn      bool
		failed        bool
		sidebarWidth  float32
		wantSignedIn  bool
		wantActive    bool
		wantWide      bool
		wantFailedSet bool
		why           string
	}{
		// 未知（宿主没有账号状态机 / 查询失败过）→ 不许当成"已登录"，也不当成"未登录"
		{"未知", false, false, false, 280, false, false, true, false,
			"unknown：SignedIn=false、Failed=false（既不登录也不报失败）"},
		{"未授权", true, false, false, 280, false, false, true, false,
			"确定未登录：座位隐藏，但启动器按官方登出态画"},
		{"失败", true, false, true, 280, false, false, true, true,
			"账号态失败：Failed 要传下去（控制器据此不轮询）"},
		{"正常", true, true, false, 280, true, true, true, false,
			"已登录 + 宽态：通知座位激活"},
		{"收起轨", true, true, false, 56, true, false, false, false,
			"侧栏收起（56px 轨）：官方 hidden={!wide}，座位不激活"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := newWave3Wiring()
			k := uikernel.New("wave3-feed")
			w.provide(k, nil)
			if err := k.Load(&dscore.Plugin{Dark: true}, &dsnotifications.Plugin{}); err != nil {
				t.Fatalf("装配失败：%v", err)
			}
			m := &model{kernel: k}
			w.attach(m)

			w.mu.Lock()
			w.accountKnown, w.accountSignedIn, w.accountFailed = tc.known, tc.signedIn, tc.failed
			w.mu.Unlock()

			w.feed("s1", tc.sidebarWidth)
			st := w.notices.AccountStateSnapshot()
			if st.SignedIn != tc.wantSignedIn {
				t.Fatalf("%s：SignedIn 应为 %v，实际 %v", tc.why, tc.wantSignedIn, st.SignedIn)
			}
			if st.Active != tc.wantActive {
				t.Fatalf("%s：Active 应为 %v，实际 %v", tc.why, tc.wantActive, st.Active)
			}
			if st.Wide != tc.wantWide {
				t.Fatalf("%s：Wide 应为 %v，实际 %v", tc.why, tc.wantWide, st.Wide)
			}
			if st.Failed != tc.wantFailedSet {
				t.Fatalf("%s：Failed 应为 %v，实际 %v", tc.why, tc.wantFailedSet, st.Failed)
			}
		})
	}
}

// TestWave3FeedProjections ③b：投影两键喂给子代理目录容器，缺键就是"未知"。
func TestWave3FeedProjections(t *testing.T) {
	w := newWave3Wiring()
	k := uikernel.New("wave3-proj")
	w.provide(k, nil)
	if err := k.Load(&dscore.Plugin{Dark: true}, &dsagentteam.Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	m := &model{kernel: k}
	w.attach(m)

	// 没有拉过投影：目录未知（不画 0 条），Team 开关不渲染，活动写「未知」。
	w.feed("s1", 280)
	view := w.team.View()
	if view.CatalogKnown {
		t.Fatal("没拉过投影时 CatalogKnown 必须 false（未知 ≠ 空目录）")
	}
	if view.ProjectionKnown {
		t.Fatal("没拉过投影时 ProjectionKnown 必须 false（Team 开关不渲染）")
	}
	if view.ModelSelectionKnown {
		t.Fatal("没拉过投影时 ModelSelectionKnown 必须 false（开关禁用）")
	}
	if stats := w.sessionStatsSnapshot("s1"); stats != nil {
		t.Fatalf("没拉过投影时 sessionStats 必须 nil（未知，不编 0 步）：%+v", stats)
	}
	// 切到别的会话同样未知（不许跨会话复用）。
	if stats := w.sessionStatsSnapshot("s-other"); stats != nil {
		t.Fatalf("别的会话不该有 sessionStats：%+v", stats)
	}

	// 拉到两条目录 + teamMode.enabled=true + sessionStats 真值 + modelSelection。
	w.mu.Lock()
	w.projSession = "s1"
	w.projCatalogKnown = true
	w.projCatalog = []dsagentteam.CatalogEntry{{ID: "child-1", Label: "调研"}}
	w.projTeamKnown = true
	w.projTeamEnabled = true
	w.projStats = &apiclient.SessionStats{Turns: 2, Steps: 3, DecodeMs: 5000, DecodeTokens: 200}
	w.projModelKnown = true
	w.projModelLastUsed = true
	w.mu.Unlock()
	w.feed("s1", 280)
	view = w.team.View()
	if !view.CatalogKnown || len(view.Catalog) != 1 || view.Catalog[0].ID != "child-1" {
		t.Fatalf("目录没喂进去：%+v", view.Catalog)
	}
	if view.Members != 1 {
		t.Fatalf("Members 应 = 1，实际 %d", view.Members)
	}
	if !view.ProjectionKnown || !view.Projection.Enabled {
		t.Fatalf("teamMode 没喂进去：known=%v enabled=%v",
			view.ProjectionKnown, view.Projection.Enabled)
	}
	// 官方 TeamModeToggle 的两个附加判据必须来自 `modelSelection` 投影
	// （宿主总是给这个键；给不出来时开关禁用，不许"看起来能点"）。
	if !view.ModelSelectionKnown || !view.ModelLastUsed {
		t.Fatalf("modelSelection 判据没喂进去：known=%v lastUsed=%v",
			view.ModelSelectionKnown, view.ModelLastUsed)
	}
	stats := w.sessionStatsSnapshot("s1")
	if stats == nil || stats.Turns != 2 || stats.Steps != 3 || stats.DecodeMs != 5000 || stats.DecodeTokens != 200 {
		t.Fatalf("sessionStats 没喂进去：%+v", stats)
	}
	// 编造检查：tasks 面宿主没有，界面不许编任务出来。
	if len(view.Projection.Tasks) != 0 {
		t.Fatalf("宿主没给 tasks，界面却编了 %d 条", len(view.Projection.Tasks))
	}

	// 切到另一个还没拉过的会话：立刻回到"未知"（不许沿用别人的目录/数字）。
	w.feed("s2", 280)
	if view = w.team.View(); view.CatalogKnown {
		t.Fatal("切到未拉取过的会话后必须回到未知，不能沿用上一个会话的目录")
	}
	if view.ProjectionKnown || view.ModelSelectionKnown {
		t.Fatalf("切到未拉取过的会话后 Team 判据必须未知：%+v", view)
	}
	if stats := w.sessionStatsSnapshot("s2"); stats != nil {
		t.Fatalf("切到未拉取过的会话后 sessionStats 必须 nil（不跨会话复用）：%+v", stats)
	}
}

// TestWave3CapabilityMissingIsUnknown "宿主没这个能力"不等于"加载失败"。
func TestWave3CapabilityMissingIsUnknown(t *testing.T) {
	for _, code := range []string{"not_implemented", "unsupported", "not_found", "method_not_found"} {
		if !isCapabilityMissing(&apiclient.APIError{Code: code}) {
			t.Fatalf("%q 应判为「能力缺失」（按未知/隐藏处理）", code)
		}
	}
	for _, code := range []string{"internal", "unauthorized", "bad_request"} {
		if isCapabilityMissing(&apiclient.APIError{Code: code}) {
			t.Fatalf("%q 是真失败，不能被当成「能力缺失」藏起来", code)
		}
	}
	if isCapabilityMissing(errors.New("dial tcp: connection refused")) {
		t.Fatal("普通网络错误不是「能力缺失」")
	}
	if isCapabilityMissing(nil) {
		t.Fatal("nil 错误不该被判成能力缺失")
	}
}

// TestLauncherWideThreshold 侧栏宽态判据只有一处（与插件里的 LauncherWide 同源）。
func TestLauncherWideThreshold(t *testing.T) {
	for _, tc := range []struct {
		width float32
		want  bool
		why   string
	}{
		{280, true, "官方默认侧栏 280 = 宽态"},
		{264, true, "SIDEBAR_MIN 264 = 宽态"},
		{56, false, "收起轨 56 = 不上座位"},
		{0, true, "宽度未知（0）**不当成收起**：不知道 ≠ 没有"},
	} {
		if got := dsnotifications.LauncherWide(tc.width); got != tc.want {
			t.Fatalf("宽 %.0f（%s）：应得 %v，实际 %v", tc.width, tc.why, tc.want, got)
		}
	}
}

// TestWave3FetchProjectionsFeedsConsumers 是**真 HTTP** 一条：`session.projections`
// 回什么，缓存里就有什么（`fetchProjections` 的解析 + 存储 + 视图三重接线）。
//
// 为什么不能只测 `feed`：`feed` 读的是缓存，缓存字段手写等于"看起来接线了"。
// 这条从 JSON 一路走到 `View` / `sessionStatsSnapshot`，任何一处漏解都会红。
func TestWave3FetchProjectionsFeedsConsumers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session.projections" {
			t.Fatalf("路径 = %q，want /api/session.projections", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"asOfSeq":12,"values":{
			"subagentCatalog":[{"id":"child-1","createdAt":1762900000000,"mode":"continuable","label":"调研"}],
			"teamMode":{"enabled":true},
			"sessionStats":{"turns":2,"steps":3,"llmMs":1500,"toolMs":800,"ttftMs":120,"ttftSteps":2,"decodeMs":5000,"decodeTokens":204},
			"modelSelection":{"lastUsed":{"provider":"deepseek","model":"group/deepseek-v4.1-flash-1017"},"next":null}
		}}`))
	}))
	defer srv.Close()

	w := newWave3Wiring()
	k := uikernel.New("wave3-fetch-proj")
	w.provide(k, nil)
	if err := k.Load(&dscore.Plugin{Dark: true}, &dsagentteam.Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	m := &model{kernel: k, cli: apiclient.New(srv.URL)}
	w.attach(m)

	w.fetchProjections(m, "sess-root")

	stats := w.sessionStatsSnapshot("sess-root")
	if stats == nil {
		t.Fatal("sessionStats 没进缓存（状态栏只能写「未知」）")
	}
	want := apiclient.SessionStats{
		Turns: 2, Steps: 3, LLMMs: 1500, ToolMs: 800,
		TTFTMs: 120, TTFTSteps: 2, DecodeMs: 5000, DecodeTokens: 204,
	}
	if *stats != want {
		t.Fatalf("sessionStats 解错：%+v，want %+v", *stats, want)
	}
	if known, lastUsed := w.modelSelectionSnapshot("sess-root"); !known || !lastUsed {
		t.Fatalf("modelSelection 判据 = (known %v, lastUsed %v)，want (true, true)", known, lastUsed)
	}
	_, _, _, teamEnabled, teamKnown := w.projectionSnapshot("sess-root")
	if !teamKnown || !teamEnabled {
		t.Fatalf("teamMode 没进缓存：known=%v enabled=%v", teamKnown, teamEnabled)
	}

	// 切到没拉过的会话：三个投影全部回到未知（不跨会话复用）。
	if stats := w.sessionStatsSnapshot("sess-other"); stats != nil {
		t.Fatalf("别的会话不该有 sessionStats：%+v", stats)
	}
	if known, _ := w.modelSelectionSnapshot("sess-other"); known {
		t.Fatal("别的会话不该有 modelSelection 判据")
	}
}

// TestWave3FetchProjectionsCapabilityMissingIsUnknown 钉住降级：老宿主没有
// `session.projections`（501 not_implemented）时，两个投影都必须是**未知**，
// 不许把"没有这个能力"写成"0 轮 0 步 / 已关闭"。
func TestWave3FetchProjectionsCapabilityMissingIsUnknown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"session.projections","hint":"","retryable":false}}`))
	}))
	defer srv.Close()

	w := newWave3Wiring()
	k := uikernel.New("wave3-fetch-missing")
	w.provide(k, nil)
	if err := k.Load(&dscore.Plugin{Dark: true}, &dsagentteam.Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	m := &model{kernel: k, cli: apiclient.New(srv.URL)}
	w.attach(m)

	w.fetchProjections(m, "sess-old")
	if stats := w.sessionStatsSnapshot("sess-old"); stats != nil {
		t.Fatalf("老宿主没有 sessionStats 时必须未知（不编 0）：%+v", stats)
	}
	if known, _ := w.modelSelectionSnapshot("sess-old"); known {
		t.Fatal("老宿主没有 modelSelection 时必须未知")
	}
	_, _, errText, _, teamKnown := w.projectionSnapshot("sess-old")
	if teamKnown {
		t.Fatal("老宿主没有 teamMode 时必须未知（开关不渲染）")
	}
	if errText != "" {
		t.Fatalf("能力缺失不该画成加载失败：%q", errText)
	}
}

// TestWave3FetchChildProjectionsFeedsCatalogRows 钉住 #56 的数据面：
// 目录项的**子会话投影**（官方 `prepareCatalog` 逐个 `refreshProjections(child)`）
// 必须被拉回来贝壳缓存，并单向流进 AgentTeam 运行时；切换父会话后不得串会话。
func TestWave3FetchChildProjectionsFeedsCatalogRows(t *testing.T) {
	parentCatalog := `[{"id":"child-1","createdAt":1762900000000,"mode":"continuable","label":"调研"}]`
	var parentBody atomic.Value
	parentBody.Store(parentCatalog)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SessionID string `json:"sessionId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch body.SessionID {
		case "sess-root":
			_, _ = w.Write([]byte(`{"asOfSeq":12,"values":{"subagentCatalog":` + parentBody.Load().(string) + `}}`))
		case "child-1":
			_, _ = w.Write([]byte(`{"asOfSeq":3,"values":{
				"subagentCatalog":[{"id":"grand-1","createdAt":1762900200000,"mode":"continuable","label":"孙任务"}],
				"subagentTiming":{"settledMs":65000,"lastTurnCompleted":true},
				"tokenUsage":{"uncachedInputTokens":100,"outputTokens":20,"cacheReadTokens":400,"cacheWriteTokens":30}
			}}`))
		default:
			t.Fatalf("不该请求的会话：%q", body.SessionID)
		}
	}))
	defer srv.Close()

	w := newWave3Wiring()
	k := uikernel.New("wave3-child-proj")
	w.provide(k, nil)
	if err := k.Load(&dscore.Plugin{Dark: true}, &dsagentteam.Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	m := &model{kernel: k, cli: apiclient.New(srv.URL)}
	w.attach(m)

	w.fetchProjections(m, "sess-root")

	children := w.childrenSnapshot("sess-root")
	child, ok := children["child-1"]
	if !ok {
		t.Fatalf("子会话投影没进缓存：%+v", children)
	}
	if !child.CatalogKnown || len(child.Catalog) != 1 || child.Catalog[0].ID != "grand-1" {
		t.Fatalf("下一层目录没解对：%+v", child)
	}
	if child.Timing == nil || child.Timing.SettledMs != 65000 {
		t.Fatalf("subagentTiming 没解对：%+v", child.Timing)
	}
	if child.Timing.LastTurnCompleted == nil || !*child.Timing.LastTurnCompleted {
		t.Fatalf("lastTurnCompleted 没解对：%+v", child.Timing)
	}
	if child.TokenTotal == nil || *child.TokenTotal != 550 {
		t.Fatalf("tokenUsage 合计没解对：%v", child.TokenTotal)
	}

	// 单向流进运行时：feed 之后 Store 的视图里必须带上这份 Children。
	w.feed("sess-root", 280)
	if got := w.team.View().Children["child-1"].Catalog; len(got) != 1 || got[0].ID != "grand-1" {
		t.Fatalf("Store 视图没拿到子投影：%+v", w.team.View().Children)
	}

	// 切到没拉过的父会话：子投影不跨会话复用（否则会画出别的树的行）。
	if got := w.childrenSnapshot("sess-other"); got != nil {
		t.Fatalf("别的父会话不该有子投影：%+v", got)
	}

	// 目录换了一批（child-1 不在里面了）→ 旧子投影必须被清掉。
	parentBody.Store(`[]`)
	w.mu.Lock()
	w.projAt = time.Time{} // 绕过 10s 轮询节流，模拟下一次刷新
	w.mu.Unlock()
	w.fetchProjections(m, "sess-root")
	if _, still := w.childrenSnapshot("sess-root")["child-1"]; still {
		t.Fatal("目录里已经没有这个子会话，旧子投影必须清掉")
	}
}
