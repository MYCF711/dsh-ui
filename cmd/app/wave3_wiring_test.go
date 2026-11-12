// wave3_wiring_test.go —— 壳侧接线的证据（第三波两个父容器的数据面）。
//
// 三件事各自有断言：
//
//	① Provide     ui.apiclient / ui.shell.tier 在 Load **之前** 就位，插件 Register 能取到
//	② attach      取回两个 ServiceKey、把档位写给插件、接上"打开账号设置"（且失败如实回 false）
//	③ feed        账号态与投影**单向**流进运行时；未知 ≠ 未登录（不许写成 signed-out）
package main

import (
	"errors"
	"testing"

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

	// 没有拉过投影：目录未知（不画 0 条）。
	w.feed("s1", 280)
	view := w.team.View()
	if view.CatalogKnown {
		t.Fatal("没拉过投影时 CatalogKnown 必须 false（未知 ≠ 空目录）")
	}
	if view.ProjectionKnown {
		t.Fatal("没拉过投影时 ProjectionKnown 必须 false（Team 开关不渲染）")
	}

	// 拉到两条目录 + teamMode.enabled=true。
	w.mu.Lock()
	w.projSession = "s1"
	w.projCatalogKnown = true
	w.projCatalog = []dsagentteam.CatalogEntry{{ID: "child-1", Label: "调研"}}
	w.projTeamKnown = true
	w.projTeamEnabled = true
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
	// 编造检查：tasks 面宿主没有，界面不许编任务出来。
	if len(view.Projection.Tasks) != 0 {
		t.Fatalf("宿主没给 tasks，界面却编了 %d 条", len(view.Projection.Tasks))
	}

	// 切到另一个还没拉过的会话：立刻回到"未知"（不许沿用别人的目录）。
	w.feed("s2", 280)
	if view = w.team.View(); view.CatalogKnown {
		t.Fatal("切到未拉取过的会话后必须回到未知，不能沿用上一个会话的目录")
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
