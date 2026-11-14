//go:build darwin || windows || linux

// official_agent_presets_visual_test.go —— #3「Agent 预设」页的**真实绘制**证据。
//
// 走真 Viva：真 Paint（经真 dsshell 的 shell.overlay 槽）→ 真 HitTest → 真 Click
// → 再 Paint。判据不是「模型里算对了」（那是 official_agent_presets_test.go 的事）：
//
//   - 宿主名册里的 preset 名与 id **真的画在屏幕上**（不是空网格 / 未知）；
//   - 只有宿主点名的那一条画「新任务默认」角标；
//   - 可选卡的命中区真的存在，点它**真的**把
//     `agent-preset-registry.selectedDefault` 写出去（值 = 被点那条的 id）；
//   - 写成功后壳的"重读名册"入口真的被调了一次；
//   - 宿主没接 `agentPresets.list` 时画面上是**未知说明**，不是空网格。
//
// 截图只在断言通过之后落盘（SETTINGS_CAPTURE_DIR 非空时保留），与另外两套视觉测试
// 同一个开关。
package dssettings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/gfx"
)

// agentPresetDescribeForDraw 在既有夹具上加**真的** agent-preset-registry 命名空间。
//
// 不复用 describeFixture：那一份里没有这条命名空间，拿它测会把"宿主没发布"与
// "写成功"混成一件事。
func agentPresetDescribeForDraw(revision int) *apiclient.SettingsDescribeResult {
	base := describeFixture()
	value, _ := json.Marshal(map[string]any{"selectedDefault": "default"})
	base.Namespaces = append(base.Namespaces, apiclient.SettingsNamespaceView{
		NS: apiclient.SettingsNSAgentPreset, Revision: revision, Value: value,
	})
	base.Sections = append(base.Sections, apiclient.SettingsSectionView{
		NS: apiclient.SettingsNSAgentPreset, Title: "Default agent preset",
		Writable: true, Revision: revision,
		Fields: []apiclient.SettingsFieldView{{
			Name: apiclient.SettingsFieldSelectedDefault,
			Path: []string{apiclient.SettingsFieldSelectedDefault},
			Type: "string", Current: "default", Writable: true,
		}},
	})
	return base
}

// openAgentPresetsPage 打开设置面板并切到「Agent 预设」页。
func openAgentPresetsPage(t *testing.T, h *officialHarness) {
	t.Helper()
	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatalf("Shift+F2：%v", err)
	}
	h.pump()
	if !h.p.OfficialShown() {
		t.Fatal("Shift+F2 没打开设置面板")
	}
	h.click("settings-nav-agent-presets")
	if got := h.p.ActiveSection(); got != "agent-presets" {
		t.Fatalf("点导航后活动 section = %q，应为 agent-presets", got)
	}
}

// TestAgentPresetsPageDrawsRosterAndWritesDefault 是 #3 的主证据。
func TestAgentPresetsPageDrawsRosterAndWritesDefault(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	h.mu.Lock()
	h.describe = agentPresetDescribeForDraw(7)
	h.roster = agentPresetRosterFixture()
	h.mu.Unlock()

	openAgentPresetsPage(t, h)

	// ① 名册真的画出来了（名字与 id 都在）。
	for _, want := range []string{"Default", "Read only", "Workspace write",
		"read-only", "workspace-write"} {
		if !h.hasText(want) {
			t.Fatalf("画面上没有 %q——名册没画出来", want)
		}
	}
	// ② 官方简介（sectionIntro）逐字。
	if !h.hasText("选择 Agent 的工具和工作方式") {
		t.Fatal("没有画官方 sectionIntro")
	}
	// ③ 只有宿主点名的那一条带「新任务默认」角标，且「设为新任务默认」是可选卡的动作。
	if !h.hasText(zhAgentPresetInUse) {
		t.Fatal("没有画「新任务默认」角标")
	}
	if !h.hasText(zhAgentPresetSetDefault) {
		t.Fatal("没有画「设为新任务默认」——那就成了一个点不动的卡片")
	}

	// ④ 点可选卡的命中区：真的写 `agent-preset-registry.selectedDefault`。
	//
	// 断言前必须**等异步动作落地**：点击在产品里就是 `go applyAgentPresetDefault(…)`，
	// 抽完立刻读 writer 是在赌 goroutine 的调度（本机 1/150、CI 更容易输）。
	// 见 waitWriterCalls 的注释；这里等的是"写请求真的发出"，不是放宽判据。
	before := h.writer.callCount()
	beforeRefresh := h.agentPresetRefreshCount()
	h.click("settings-agent-preset-read-only")
	if !waitWriterCalls(h, before+1) {
		t.Fatalf("点了卡片却没发出任何写请求（callCount=%d want=%d status=%q）",
			h.writer.callCount(), before+1, h.statusText())
	}
	call, ok := h.writer.lastCall()
	if !ok {
		t.Fatal("点了卡片却没发出任何写请求")
	}
	if call.ns != apiclient.SettingsNSAgentPreset {
		t.Fatalf("ns = %q，官方命名空间逐字是 agent-preset-registry", call.ns)
	}
	if len(call.path) != 1 || call.path[0] != apiclient.SettingsFieldSelectedDefault {
		t.Fatalf("path = %v，应为 [selectedDefault]", call.path)
	}
	if call.value != "read-only" {
		t.Fatalf("写的值 = %v，应为被点那一条的 id", call.value)
	}
	if call.expectedRev == nil || *call.expectedRev != 7 {
		t.Fatalf("expectedRevision = %v，应为 describe 里的 7", call.expectedRev)
	}
	// ⑤ 写成功后壳的名册重读入口被调了一次（角标要搬到新那一条上）。
	//
	// 与 ④ 同一个理由：`refresh()` 在 `applyAgentPresetDefault` 里紧跟在写之后，
	// 也可能比断言晚一步。基线在点击前就取（见 ④）。
	for i := 0; i < 400 && h.agentPresetRefreshCount() <= beforeRefresh; i++ {
		h.pump()
	}
	if refreshed := h.agentPresetRefreshCount(); refreshed != beforeRefresh+1 {
		t.Fatalf("写成功后应触发一次名册重读，实得 %d 次（基线 %d）", refreshed, beforeRefresh)
	}

	dir := os.Getenv("SETTINGS_CAPTURE_DIR")
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("建截图目录：%v", err)
		}
		if err := h.session.Screenshot(filepath.Join(dir, "agent-presets.png")); err != nil {
			t.Fatalf("截图：%v", err)
		}
	}
}

// TestAgentPresetsPageShowsUnknownWhenRosterUnwired 钉住「未接线」画的是说明。
func TestAgentPresetsPageShowsUnknownWhenRosterUnwired(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	h.mu.Lock()
	h.describe = agentPresetDescribeForDraw(7)
	h.roster = nil // 宿主没接 agentPresets.list
	h.mu.Unlock()

	openAgentPresetsPage(t, h)

	if !h.hasText("agentPresets.list") {
		t.Fatal("宿主没接名册时没画「未知」说明——那就成了空网格")
	}
	// 未知态下不许出现"把某一条设为默认"的动作面：那是没有数据却在画可点卡片。
	want := h.tree.ID(h.feat, "settings-agent-preset-default")
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == want {
			t.Fatal("没有名册却画出了可点的卡片命中区")
		}
	}
}
