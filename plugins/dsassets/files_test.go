package dsassets

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestFileTypeUsesOfficialSheetsAndMarks(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		base       gfx.Color
		layers     int
		mark       bool
	}{
		{name: "main.go", kind: "code", base: fileTypeSheetBlue, layers: 2, mark: true},
		{name: "photo.PNG", kind: "image", base: fileTypeSheetViolet, layers: 5},
		{name: "README.md", kind: "markdown", base: fileTypeSheetBlue, layers: 3},
		{name: "notes.txt", kind: "other", base: fileTypeSheetOther, layers: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyFileType(tc.name); got != tc.kind {
				t.Fatalf("classifyFileType(%q)=%q，官方分支=%q", tc.name, got, tc.kind)
			}
			asset, ok := FileType(tc.name)
			if !ok || asset.Sheet.ViewBox != 28 || len(asset.Sheet.Layers) != tc.layers {
				t.Fatalf("FileType(%q)=%+v, %v", tc.name, asset, ok)
			}
			if got := asset.Sheet.Layers[0].Fill; got != tc.base {
				t.Errorf("底板色=%v，官方=%v", got, tc.base)
			}
			if (asset.Mark != nil) != tc.mark {
				t.Errorf("独立描边标记存在=%v，want %v", asset.Mark != nil, tc.mark)
			}
			if tc.mark && asset.Mark.Stroke != 1.35 {
				t.Errorf("code 标记 stroke=%v，官方=1.35", asset.Mark.Stroke)
			}
		})
	}
}

func TestFileTypeOfficialFoldAndImageTransform(t *testing.T) {
	other, ok := FileType("a.txt")
	if !ok || other.Sheet.Layers[1].Fill != fileTypeFoldMuted {
		t.Fatalf("other 折角必须使用 neutral-400：%+v", other.Sheet.Layers)
	}
	image, ok := FileType("a.png")
	if !ok {
		t.Fatal("图片素材解析失败")
	}
	first := image.Sheet.Layers[2].Commands[0].Point
	want := scaleFileTypePoint(gfx.Point{X: 10.4212, Y: 15.9204}, 16, 1.12)
	if first != want {
		t.Errorf("image markTransform 后首点=%+v，官方=%+v", first, want)
	}
}

func TestFileActionIconsUseOfficialPaths(t *testing.T) {
	left, ok := FileChevron(true)
	if !ok || left.ViewBox != 16 || left.Stroke != 1 || len(left.Commands) == 0 {
		t.Fatalf("left chevron=%+v, %v", left, ok)
	}
	right, ok := FileChevron(false)
	if !ok || len(right.Commands) == 0 {
		t.Fatalf("right chevron=%+v, %v", right, ok)
	}
	if left.Commands[0].Point.X != 10 || right.Commands[0].Point.X != 6 {
		t.Errorf("左右 chevron 方向错误：left=%+v right=%+v", left.Commands[0].Point, right.Commands[0].Point)
	}
	open, ok := FileRightUp(gfx.RGB24(0xffffff))
	if !ok || open.ViewBox != 16 || open.Stroke != 1 || len(open.Commands) == 0 || len(open.Layers) != 1 {
		t.Fatalf("right-up=%+v, %v", open, ok)
	}
}
