package dssettings

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestVisualReplicaControlGeometry(t *testing.T) {
	if got := OfficialRowHeight(ItemCubes); got != 144 {
		t.Fatalf("Appearance 行高 = %v，官方 16+22+8+82+16 = 144", got)
	}
	for _, tc := range []struct {
		node, prop string
		want       float32
	}{
		{"AppearanceRow.themeCube", "padding-block", 20},
		{"AppearanceRow.themeCube", "padding-inline", 32},
		{"FontSizeRow.stepper", "min-width", 72},
		{"FontSizeRow.arrow", "width", 17},
		{"FontSizeRow.arrow", "height", 12},
		{"ModelsSection.rowCard", "border-width", 0.5},
		{"AccountSection.identity", "min-height", 44},
		{"AccountSection.signedOutCopy", "gap", 4},
	} {
		row, ok := geometryRow(tc.node, tc.prop)
		if !ok || row.Official != tc.want || row.Impl != tc.want {
			t.Fatalf("%s.%s = %+v, ok=%v；官方/实现都应为 %v", tc.node, tc.prop, row, ok, tc.want)
		}
	}
	if ModelsInputMinHeight != 32 {
		t.Fatalf("Models input 高 = %v，官方 ModelsSection.module.css:810 是 32", ModelsInputMinHeight)
	}
}

func TestVisualReplicaUsesOfficialVectorIcons(t *testing.T) {
	icons := map[string]gfx.Icon{
		"close":    closeOutlineIcon(),
		"search":   searchOutlineIcon(),
		"chevron":  chevronOutlineIcon(),
		"user":     userOutlineIcon(),
		"archive":  archiveOutlineIcon(),
		"light":    appearanceOutlineIcon("light"),
		"dark":     appearanceOutlineIcon("dark"),
		"system":   appearanceOutlineIcon("system"),
		"right-up": rightUpOutlineIcon(gfx.RGB24(0)),
		"data":     modelDataOutlineIcon(gfx.RGB24(0)),
		"preset":   agentPresetOutlineIcon(gfx.RGB24(0)),
		"plugins":  personalizationOutlineIcon(gfx.RGB24(0)),
	}
	for name, icon := range icons {
		if icon.ViewBox != 16 {
			t.Fatalf("%s viewBox = %v，官方 sprite 是 16", name, icon.ViewBox)
		}
		if len(icon.Commands)+len(icon.Layers) == 0 {
			t.Fatalf("%s 官方 path 没有成功解析", name)
		}
	}
	if userOutlineIcon().Stroke != officialMediumStroke {
		t.Fatalf("账号导航 stroke = %v，官方 Medium = 1.3", userOutlineIcon().Stroke)
	}
}
