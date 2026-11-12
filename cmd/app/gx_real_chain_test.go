//go:build windows

package main

// gx_real_chain_test.go —— 真机验收：官方 composer 的 `conversation.input.*` 子槽 +
// 模型标签唯一 + 官方模型菜单（`conversation.input.model` 占用方）。
//
// # 为什么要真机
//
// 单测（`plugins/dscomposer/input_slots_test.go`、`plugins/dsagentteam/model_slot_test.go`、
// `cmd/app/gx_input_slots_test.go`）用的是手写快照；这里的数据面全部来自**真宿主**：
//
//	provider 目录  真 `llm.listConfigurableProviders`（本机 magpie 网关的配置）
//	模型/权限/上下文 真 `session.load`
//	Team 投影      真 `session.projections`（**先真调 `agentTeams.setEnabled(true)`**，
//	               宿主按官方形状 append `team/mode {enabled:true}`，投影折出
//	               `teamMode={"enabled":true}` → 官方开关出现且为「开」）
//	modelSelection 真 `session.projections`（开关可用性判据）
//
// # 五条判据
//
//	① 官方座位每帧 dispatch 全部 10 个 `conversation.input.*` 子槽各 1 次；
//	   `main.composer` 0 次（不双画）。
//	② 画布上模型标签**恰好 1 个**（真 `load.Model`）。
//	③ trigger 真的在 composer 控件行里（底部区域内、28 高、可展开）。
//	④ 点 trigger 真开官方菜单：真 provider 行出现；Team 投影已知时官方开关也出现。
//	⑤ 截图：官方档（菜单展开）+ 旧三栏档各一张，两档都只有一个模型标签。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_gr/dshd.exe"
//	go test ./cmd/app -run 'TestRealChainGXModelSlot' -count=1 -v
//
// 依赖本机 magpie 网关（`dshd` 默认 `DSHD_LLM_BASE=http://127.0.0.1:3425/v1`）；
// 网关连不上时**跳过**并说清原因——不拿"没跑到"冒充通过。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsagentteam"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// gxRealProviders 把真 apiclient 的 provider 目录转成模型菜单的行数据。
//
// 转换是**逐字段**的（不丢 error / declared 语义）：菜单里"未在模型目录中声明"
// 与"这一项自己报了错"是两种不同的真话，不能合并成一个空行。
func gxRealProviders(list []apiclient.ConfigurableProvider) []dsagentteam.ModelProviderOption {
	out := make([]dsagentteam.ModelProviderOption, 0, len(list))
	for _, p := range list {
		seed := p.SettingsNamespace
		if seed == "" {
			seed = "未在模型目录中声明"
		}
		out = append(out, dsagentteam.ModelProviderOption{
			Provider:    p.Provider,
			DisplayName: p.DisplayName,
			Seed:        seed,
			Failure:     p.Error,
		})
	}
	return out
}

// gxRealSnap 用真 `session.load` 组一帧快照。
func gxRealSnap(sessionID string, load *apiclient.LoadResult) pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur: sessionID, Status: "已连接",
		Model:         load.Model,
		Permission:    load.Permission,
		ContextTokens: load.ContextTokens,
		ContextWindow: load.ContextWindow,
	}
}

// TestRealChainGXModelSlot 真宿主 + 真 apiclient + 官方 composer 子槽。
func TestRealChainGXModelSlot(t *testing.T) {
	host := startRealHost(t)
	client, _ := connectRealGUI(t, host)

	createCtx, createCancel := context.WithTimeout(context.Background(), 30*time.Second)
	sessionID, err := client.Create(createCtx, host.home, "workspace-write", "gx-model-slot")
	createCancel()
	if err != nil {
		t.Fatalf("session.create 失败：%v", err)
	}
	t.Logf("真实会话 id=%s（home=%s）", sessionID, host.home)

	load, err := client.Load(context.Background(), sessionID, 0)
	if err != nil {
		t.Fatalf("session.load 失败：%v", err)
	}
	t.Logf("真宿主真值：model=%q permission=%q contextTokens=%d contextWindow=%d",
		load.Model, load.Permission, load.ContextTokens, load.ContextWindow)

	// ── 真 provider 目录（`llm.listConfigurableProviders`）。
	provCtx, provCancel := context.WithTimeout(context.Background(), 20*time.Second)
	providers, provErr := client.ListConfigurableProviders(provCtx)
	provCancel()
	if provErr != nil {
		t.Logf("警告：llm.listConfigurableProviders 失败（%v）——模型菜单会走降级只读 chip", provErr)
	} else {
		t.Logf("真 provider 目录 %d 项：%+v", len(providers), providers)
	}

	// ── 真 Team 投影：**先开 Team**，否则宿主没有 `team/mode` 事件，
	// `session.projections` 不会给 `teamMode` 键（官方 undefined 语义下开关不渲染）。
	enableCtx, enableCancel := context.WithTimeout(context.Background(), 20*time.Second)
	if err := client.SetAgentTeamEnabled(enableCtx, sessionID, true); err != nil {
		enableCancel()
		t.Fatalf("agentTeams.setEnabled(true) 失败：%v", err)
	}
	enableCancel()

	var teamMode *apiclient.TeamModeProjection
	teamKnown := false
	modelSelectionKnown, modelLastUsed := false, false
	projCtx, projCancel := context.WithTimeout(context.Background(), 20*time.Second)
	projections, projErr := client.SessionProjections(projCtx, sessionID)
	projCancel()
	if projErr == nil && projections != nil {
		teamMode, teamKnown = projections.TeamMode()
		if selection, ok := projections.ModelSelection(); ok {
			modelSelectionKnown = true
			modelLastUsed = selection != nil && selection.LastUsed != nil
		}
	}
	if !teamKnown {
		t.Fatalf("开了 Team 之后宿主仍没有 teamMode 投影（err=%v）——GUI 的官方开关无从渲染", projErr)
	}
	if !modelSelectionKnown {
		t.Fatal("宿主没给 modelSelection 投影（官方 Team 开关的可用性判据）")
	}
	t.Logf("真 teamMode 投影：enabled=%v；modelSelection known=%v lastUsed=%v",
		teamMode.Enabled, modelSelectionKnown, modelLastUsed)

	p := newGXProduct(t, shellOfficial)
	p.team.SetProviders(provErr == nil, "", gxRealProviders(providers))
	p.team.SetView(dsagentteam.View{
		SessionID:           sessionID,
		ProjectionKnown:     true,
		Projection:          dsagentteam.TeamProjection{Enabled: teamMode.Enabled},
		ModelSelectionKnown: modelSelectionKnown,
		ModelLastUsed:       modelLastUsed,
	})
	snap := gxRealSnap(sessionID, load)

	// ── ① 子槽派发 + ② 单一模型标签 + ③ trigger 在控件行。
	first := p.paint(t, snap, "")
	if got := p.dispatched[uikernel.OffConvComposer]; got != 1 {
		t.Fatalf("官方 composer 座位 dispatch = %d，want 1（全部=%v）", got, p.dispatched)
	}
	if got := p.dispatched[pluginapi.SlotMainComposer]; got != 0 {
		t.Fatalf("官方座位活着时 main.composer 也被 dispatch（双画）：%d", got)
	}
	for _, slot := range gxComposerInputSubSlots {
		if got := p.dispatched[slot]; got != 1 {
			t.Fatalf("子槽 %q 一帧 dispatch = %d，want 1（全部=%v）", slot, got, p.dispatched)
		}
	}
	label := strings.TrimSpace(load.Model)
	if label == "" {
		label = "未知"
	}
	if got := gxTextCount(first, label); got != 1 {
		t.Fatalf("模型标签 %q 出现 %d 次，want 恰好 1；文本=%v", label, got, gxTexts(first))
	}
	trigger, ok := gxHit(first, gxModelMenuAria)
	if !ok {
		t.Fatalf("官方 trigger 没有命中区（模型菜单点不开）；hits=%v", first.Hits())
	}
	if trigger.Bounds.Height != 28 {
		t.Fatalf("trigger 高 = %v，官方 28", trigger.Bounds.Height)
	}
	if trigger.Bounds.Y < 700 {
		t.Fatalf("trigger 不在 composer 控件行（y=%v）；hits=%v", trigger.Bounds.Y, first.Hits())
	}
	if _, ok := gxHit(first, "发送"); !ok {
		t.Fatalf("控件行没有发送键；hits=%v", first.Hits())
	}
	t.Logf("trigger 几何：x=%.1f y=%.1f w=%.1f h=%.1f（模型标签=%q）",
		trigger.Bounds.X, trigger.Bounds.Y, trigger.Bounds.Width, trigger.Bounds.Height, label)

	// ── ④ 点 trigger 真开官方菜单。
	canvas := p.paint(t, snap, "model-menu-trigger")
	if !p.team.ModelMenuOpen() {
		t.Fatal("点了 trigger 但菜单没开")
	}
	if provErr == nil && len(providers) > 0 {
		// 真 provider 行必须出现（displayName 或 provider 名，任一）。
		found := false
		for _, pv := range providers {
			if gxHasText(canvas, pv.DisplayName) || gxHasText(canvas, pv.Provider) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("菜单里没有真 provider 行（providers=%+v）；文本=%v", providers, gxTexts(canvas))
		}
	} else if provErr != nil && !gxHasText(canvas, "未接线") {
		t.Fatalf("目录取失败时菜单应显示官方 error 行；文本=%v", gxTexts(canvas))
	}
	if !gxHasText(canvas, gxTeamModeLabel) {
		t.Fatalf("Team 投影已知时官方开关应出现在菜单里（缺 %q）；文本=%v",
			gxTeamModeLabel, gxTexts(canvas))
	}
	// 开关状态必须等于宿主真值（`team/mode {enabled:true}` → checked），
	// 而且 `modelSelection` 已知时**可用**（不是灰着的假开关）。
	if hit, ok := gxSwitchHit(canvas); !ok {
		t.Fatalf("官方 Team 开关没有命中区；hits=%v", canvas.Hits())
	} else if !hit.Checked {
		t.Fatalf("宿主 teamMode.enabled=true，开关却是「关」：%+v", hit)
	} else if hit.Disabled {
		t.Fatalf("modelSelection 已知时开关不该禁用：%+v", hit)
	}
	if got := gxTextCount(canvas, label); got != 1 {
		t.Fatalf("菜单展开后模型标签出现 %d 次，want 1；文本=%v", got, gxTexts(canvas))
	}

	// ── ⑤ 截图：官方档（菜单展开）+ 旧三栏档（只读 chip）。
	dir := os.Getenv("GX_CAPTURE_DIR")
	if dir == "" {
		dir = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_gx"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建截图目录失败：%v", err)
	}
	stamp := time.Now().Unix()

	officialShot := filepath.Join(dir, fmt.Sprintf("gx-official-model-menu-%d.png", stamp))
	gxCapture(t, p, snap, officialShot)
	t.Logf("GX_CAPTURE official path=%s", officialShot)

	// 旧三栏档：同一份产品插件集，只 dispatch `main.composer`（产品默认 auto 的形状）。
	legacy := newGXProduct(t, shellLegacy)
	legacy.team.SetProviders(provErr == nil, "", gxRealProviders(providers))
	legacyCanvas := legacy.paintLegacySeat(t, snap)
	if got := legacy.dispatched[pluginapi.SlotMainComposer]; got != 1 {
		t.Fatalf("旧三栏档 main.composer dispatch = %d，want 1（全部=%v）", got, legacy.dispatched)
	}
	// 旧三栏档的模型 chip 是 composer 的回退实现：宿主给了真值才画，
	// 没给就整段隐藏（降级允许"隐藏"，但不许编 0、更不许双标签）。
	legacyLabels := gxTextCount(legacyCanvas, label)
	if legacyLabels > 1 {
		t.Fatalf("旧三栏档模型标签出现 %d 次（双标签）；文本=%v", legacyLabels, gxTexts(legacyCanvas))
	}
	if label == "未知" {
		t.Log("宿主没给模型（load.Model 为空）→ 旧三栏档隐藏模型 chip（降级：隐藏，不编 0）")
	} else if legacyLabels != 1 {
		t.Fatalf("旧三栏档模型标签出现 %d 次，want 1；文本=%v", legacyLabels, gxTexts(legacyCanvas))
	}
	if _, ok := gxHit(legacyCanvas, gxModelMenuAria); !ok {
		t.Fatalf("旧三栏档缺模型菜单命中区（dsagentteam 兼容 painter）；hits=%v", legacyCanvas.Hits())
	}
	legacyShot := filepath.Join(dir, fmt.Sprintf("gx-legacy-model-chip-%d.png", stamp))
	gxCapture(t, legacy, snap, legacyShot)
	t.Logf("GX_CAPTURE legacy path=%s", legacyShot)
}

// gxCapture 用真 headless 画布（真文本测量）落一张 PNG。
//
// 与 `gxProduct.paint` 的区别：这里走 `headless.Open` 的**真窗口过程**（真测量 +
// 真截图），画的内容与刚才断言的那一帧一致（同一份 kernel / store / 菜单展开态）。
func gxCapture(t *testing.T, p *gxProduct, snap pluginapi.Snapshot, path string) {
	t.Helper()
	theme := p.theme
	shotTree := &gfx.Tree{}
	shotFeat := shotTree.Feature("gx-shot")
	var invalidator app.Invalidator
	config := app.Config{
		Title: "gx-model-slot", Width: 1182, Height: 832, Tree: shotTree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: snap, Tree: shotTree, Feat: shotFeat, Canvas: canvas,
			Bounds: bounds, MainWidth: 902,
			Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: 902},
			Has:      func(slot pluginapi.Slot) bool { return p.kernel.Has(slot) },
		}
		var view flex.View
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			if p.mode == shellLegacy {
				// 旧三栏档：dsshell 的形状（中栏 Pad(12) 里 dispatch `main.composer`）。
				f.Column().FillWidth().FillHeight().Pad(12).Children(func() {
					p.kernel.DrawSlot(f, pluginapi.SlotMainComposer, env)
				})
				return
			}
			uikernel.FirstScreen(f, env, p.opts, func(slot pluginapi.Slot) {
				p.kernel.DrawSlot(f, slot, env)
			})
		})
		for _, painter := range p.kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	defer session.Close()
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatalf("pump 失败：%v", err)
	}
	if err := session.Screenshot(path); err != nil {
		t.Fatalf("截图失败：%v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图文件无效 path=%s info=%v err=%v", path, info, err)
	}
}
