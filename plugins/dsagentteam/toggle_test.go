// toggle_test.go —— 官方 Team 开关（槽 ② `conversation.input.model.options`）的
// **两态** + 两条降级。
//
// 为什么要单独一个文件：`slots_test.go` 的 `TestToggleDegradations` 只钉"该不该
// 渲染"，没有钉"渲染出来的开关是开还是关"。而本任务要证明的正是 GUI **真消费**
// 宿主 rc.2 新增的 `team/mode` 投影（形状 `{enabled:bool}`）——不钉 checked 状态，
// 就分不清"读了投影"和"恒画一个关掉的开关"。
//
// 官方判据（`TeamModeToggle.tsx:43-47`）：
//
//	mode === undefined                       → 整块不渲染（投影缺失）
//	checked = mode.enabled                   → 宿主值，非乐观更新
//	disabled = locked || pending || modelSelection === undefined
//
// 所以四个用例分别是：enabled=true / enabled=false / 投影缺失 / modelSelection 缺失。
package dsagentteam

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// switchHit 取开关本体的命中区（viva 的 Switch 命中标签固定是 "switch"）。
func switchHit(canvas *gfx.Canvas) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == "switch" {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func TestTeamToggleTwoStates(t *testing.T) {
	cases := []struct {
		name         string
		view         View
		wantRender   bool
		wantChecked  bool
		wantDisabled bool
		why          string
	}{
		{
			name: "投影已知且开启",
			view: View{
				SessionID:           "sess-root",
				ProjectionKnown:     true,
				Projection:          TeamProjection{Enabled: true},
				ModelSelectionKnown: true,
			},
			wantRender:  true,
			wantChecked: true,
			why:         "teamMode.enabled=true → 开关必须画成「开」（宿主真值，不是恒关）",
		},
		{
			name: "投影已知且关闭",
			view: View{
				SessionID:           "sess-root",
				ProjectionKnown:     true,
				Projection:          TeamProjection{Enabled: false},
				ModelSelectionKnown: true,
			},
			wantRender:  true,
			wantChecked: false,
			why:         "teamMode.enabled=false → 开关画成「关」",
		},
		{
			name:       "投影缺失（老宿主）",
			view:       View{SessionID: "sess-root", ModelSelectionKnown: true},
			wantRender: false,
			why:        "官方 mode === undefined → 整块不渲染（未知不画成「已关闭」）",
		},
		{
			name: "modelSelection 缺失 → 渲染但禁用",
			view: View{
				SessionID:       "sess-root",
				ProjectionKnown: true,
				Projection:      TeamProjection{Enabled: true},
			},
			wantRender:   true,
			wantChecked:  true,
			wantDisabled: true,
			why:          "官方 modelSelection === undefined → 开关禁用（不是隐藏）",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(nil)
			p.team.SetView(tc.view)
			canvas := renderSlot(t, p, SlotInputModelOptions)

			if !tc.wantRender {
				if len(canvas.Texts()) != 0 || len(canvas.Hits()) != 0 {
					t.Fatalf("%s：该整块不渲染，却画了 %d 文本 / %d 命中区",
						tc.why, len(canvas.Texts()), len(canvas.Hits()))
				}
				return
			}
			if !hasText(canvas, zhModeLabel) {
				t.Fatalf("%s：缺开关文案 %q；文本=%v", tc.why, zhModeLabel, textValues(canvas))
			}
			hit, ok := switchHit(canvas)
			if !ok {
				t.Fatalf("%s：缺开关命中区（用户点不了）；hits=%v", tc.why, hitLabels(canvas))
			}
			if hit.Checked != tc.wantChecked {
				t.Fatalf("%s：开关 checked = %v，want %v", tc.why, hit.Checked, tc.wantChecked)
			}
			if hit.Disabled != tc.wantDisabled {
				t.Fatalf("%s：开关 disabled = %v，want %v", tc.why, hit.Disabled, tc.wantDisabled)
			}
		})
	}
}

// TestTeamToggleHidesForSubagent 钉住两条"子会话不能改 Team"的守卫
// （官方 TeamModeToggle.tsx:30-32：`subagent !== null` / `origin === 'subagent'`）。
func TestTeamToggleHidesForSubagent(t *testing.T) {
	for _, tc := range []struct {
		name string
		view View
	}{
		{"按地址", View{SessionID: "sess-child", ProjectionKnown: true,
			Projection: TeamProjection{Enabled: true}, ModelSelectionKnown: true, SubagentChild: true}},
		{"按 origin", View{SessionID: "sess-child", ProjectionKnown: true,
			Projection: TeamProjection{Enabled: true}, ModelSelectionKnown: true, ListedAsSubagent: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(nil)
			p.team.SetView(tc.view)
			canvas := renderSlot(t, p, SlotInputModelOptions)
			if len(canvas.Texts()) != 0 {
				t.Fatalf("子会话不该渲染 Team 开关；文本=%v", textValues(canvas))
			}
		})
	}
}

var _ = pluginapi.Slot("")
