package dsforge

import (
	"math"
	"testing"

	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/ui"
)

// TestDiffColorsComeFromOfficialTokens 钉住六档 diff 令牌是**官方取值**，
// 不是"绿一点 / 红一点"自己调的：值逐条来自
// `dsh-client-ui-theme/src/styles/design-platform.css:281-286`（亮）与 `:433-438`（暗）。
func TestDiffColorsComeFromOfficialTokens(t *testing.T) {
	dark, light := dscore.Dark(), dscore.Light()
	check := func(name string, got ui.Color, want uint32) {
		t.Helper()
		wantR := float32(want>>16) / 255
		wantG := float32((want>>8)&0xff) / 255
		wantB := float32(want&0xff) / 255
		if math.Abs(float64(got.Red-wantR)) > 1e-6 || math.Abs(float64(got.Green-wantG)) > 1e-6 ||
			math.Abs(float64(got.Blue-wantB)) > 1e-6 {
			t.Fatalf("%s = (%v,%v,%v)，官方 #%06x", name, got.Red, got.Green, got.Blue, want)
		}
	}
	check("dark added-bg", dark.FileDiffAddedBg, 0x1f3124)
	check("dark added-gutter", dark.FileDiffAddedGutter, 0x132016)
	check("dark added-marker", dark.FileDiffAddedMarker, 0x41c977)
	check("dark deleted-bg", dark.FileDiffDeletedBg, 0x3c1f1b)
	check("dark deleted-gutter", dark.FileDiffDeletedGutter, 0x28130e)
	check("dark deleted-marker", dark.FileDiffDeletedMarker, 0xfa423e)
	check("light added-bg", light.FileDiffAddedBg, 0xe6f4e7)
	check("light added-gutter", light.FileDiffAddedGutter, 0xedf7ed)
	check("light added-marker", light.FileDiffAddedMarker, 0x01a241)
	check("light deleted-bg", light.FileDiffDeletedBg, 0xfce6e2)
	check("light deleted-gutter", light.FileDiffDeletedGutter, 0xfdece9)
	check("light deleted-marker", light.FileDiffDeletedMarker, 0xba2723)
}
