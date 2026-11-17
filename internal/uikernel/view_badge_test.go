//go:build darwin || windows || linux

package uikernel

// view_badge_test.go —— 右栏 tab 芯片徽标的机器断言。
//
// 数据来源与形状全部来自官方终端标题（`sidebar.right.pane.tab.title` 的占用方）：
//
//	OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.tsx:42-50
//	  <TerminalIcon /> {badge} {title}   —— 徽标在标题**之前**
//	OFF:…/TerminalTitle.module.css:3     `.badge{display:inline-flex;flex:none}`
//	OFF:dsh-web-frontend/dist/assets/index-CSAdOBx6.css  `._tag_brmue_4`
//	  padding:1px 8px; font-size:11px; line-height:17px; font-weight:500;
//	  border-radius:999px（`.badge` 里那枚 Tag 本体）

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

func TestRightTabBadgeGeometryMatchesOfficialTag(t *testing.T) {
	checks := []struct {
		name string
		got  float32
		want float32
	}{
		{"tag line-height", RightTabBadgeLineHeight, 17},
		{"tag padding-y", RightTabBadgePadY, 1},
		{"tag padding-x", RightTabBadgePadX, 8},
		{"tag font-size", RightTabBadgeFontSize, 11},
		{"tag radius", RightTabBadgeRadius, 999},
		{"tag info tint", RightTabBadgeInfoTint, 0.10},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v，官方值 %v", check.name, check.got, check.want)
		}
	}
	// `padding:1px 8px` + `line-height:17px` → Tag 高 19（官方没有显式 height）。
	if got, want := RightTabBadgeLineHeight+2*RightTabBadgePadY, float32(19); got != want {
		t.Errorf("Tag 高度 = %v，官方 17+2×1 = %v", got, want)
	}
}

func TestDrawRightTabsDrawsBadgeBeforeTitle(t *testing.T) {
	tabs := []pluginapi.Tab{{
		Key: "terminal", Kind: "terminal", Label: "终端",
		Badge: &pluginapi.TabBadge{Text: "沙箱", Tone: pluginapi.TabBadgeNeutral, Hint: "只读沙箱"},
	}}
	session := openGuideFrame(t, 360, 120, func(env *pluginapi.Env) func(ui.C) {
		return func(f ui.C) { DrawRightTabs(f, env, tabs, "terminal", func(string) {}) }
	})
	badgeX, labelX := float32(-1), float32(-1)
	for _, one := range session.Snapshot().Texts {
		switch one.Value {
		case "沙箱":
			badgeX = one.Bounds.X
		case "终端":
			labelX = one.Bounds.X
		}
	}
	if badgeX < 0 || labelX < 0 {
		t.Fatalf("徽标与标题没有同时上屏（badge=%v label=%v）：%v", badgeX, labelX, session.Texts())
	}
	// TerminalTitle.tsx:44-45 的注释：徽标在名字之前，因为关闭按钮盖住名字末端。
	if badgeX >= labelX {
		t.Fatalf("徽标 x=%v 不在标题 x=%v 之前", badgeX, labelX)
	}
}
