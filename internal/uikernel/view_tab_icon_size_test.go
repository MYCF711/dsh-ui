//go:build darwin || windows || linux

package uikernel

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

func TestRightTabRosterCarriesOfficialIconSize(t *testing.T) {
	icon := gfx.Icon{ViewBox: 16, Stroke: 1, Commands: []gfx.PathCommand{gfx.MoveTo(3, 4), gfx.LineTo(7, 8)}}
	k := loadTabs(t,
		pluginapi.Painter{ID: "terminal-body", Slot: pluginapi.SlotRightPaneTab, Key: "terminal",
			IconImage: icon, IconSize: 16, Draw: noopDraw},
		pluginapi.Painter{ID: "browser-body", Slot: pluginapi.SlotRightPaneTab, Key: "browser",
			IconImage: icon, Draw: noopDraw},
	)
	got := RightTabs(k)
	if len(got) != 2 || got[0].IconSize != 16 {
		t.Fatalf("TerminalIcon 的 16px 必须投影到名录：%+v", got)
	}
	if got[1].IconSize != 0 {
		t.Fatalf("未声明 IconSize 的浏览器必须保留 dockkit 默认档，实际 %v", got[1].IconSize)
	}
}
