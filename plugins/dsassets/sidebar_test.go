package dsassets

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSidebarAssetsBuildWithoutGlyphFallback(t *testing.T) {
	white := gfx.RGB(255, 255, 255)
	cases := []struct {
		name  string
		build func() (gfx.Icon, bool)
	}{
		{"new-chat-medium", func() (gfx.Icon, bool) { return SidebarNewChatMedium(white) }},
		{"plugin-pinwheel", func() (gfx.Icon, bool) { return SidebarPluginPinwheel(white) }},
		{"clock", func() (gfx.Icon, bool) { return SidebarClock(white) }},
		{"search", func() (gfx.Icon, bool) { return WorkspaceSearch(white) }},
		{"project-add", func() (gfx.Icon, bool) { return WorkspaceProjectAdd(white) }},
		{"folder-close", func() (gfx.Icon, bool) { return WorkspaceFolder(white, false) }},
		{"folder-open", func() (gfx.Icon, bool) { return WorkspaceFolder(white, true) }},
		{"close-fill", func() (gfx.Icon, bool) { return WorkspaceCloseFill(white) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			icon, ok := tc.build()
			if !ok || len(icon.Commands)+len(icon.Layers) == 0 || icon.ViewBox != 16 {
				t.Fatalf("官方图标未完整构建：ok=%v commands=%d layers=%d viewBox=%v",
					ok, len(icon.Commands), len(icon.Layers), icon.ViewBox)
			}
		})
	}
}

func TestSidebarCompositeGeometryIsOfficial(t *testing.T) {
	white := gfx.RGB(255, 255, 255)
	_, rings, ok := WorkspaceSlidersTwo(white)
	if !ok || len(rings) != 2 || rings[0] != (OfficialRing{CX: 9.95, CY: 5, Radius: 1.45, Stroke: 1}) ||
		rings[1] != (OfficialRing{CX: 5.75, CY: 11, Radius: 1.45, Stroke: 1}) {
		t.Fatalf("sliders-two 圆环偏离 rc.2 sprite：ok=%v rings=%+v", ok, rings)
	}
	_, rects, ok := SidebarAppsAdd(white)
	if !ok || len(rects) != 3 || rects[0] != (OfficialRoundedRect{X: 2.25, Y: 2.25, Width: 4, Height: 4, Radius: 1, Stroke: 1}) {
		t.Fatalf("apps-add 圆角矩形偏离 rc.2 sprite：ok=%v rects=%+v", ok, rects)
	}
	open, ok := WorkspaceFolder(white, true)
	if !ok || len(open.Layers) != 3 || open.Layers[0].Fill.Alpha != 0.16 {
		t.Fatalf("folder-open 的 0.16 填充层丢失：ok=%v layers=%+v", ok, open.Layers)
	}
}
