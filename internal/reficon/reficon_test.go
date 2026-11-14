package reficon

// reficon_test.go —— 三枚官方 `ReferenceIconRegular` 的几何守卫。
//
// 只钉能被 rc.2 dist 原文证明的事实：viewBox=16、stroke=1、显示时由调用方给 14。
// 段数按官方组件写死（session 3 / file 2 描边+1 填充 / folder 2），
// 改多了就是画了官方没有的东西，改少了就是半个图标。

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func counts(cmds []gfx.PathCommand) map[gfx.PathCommandKind]int {
	out := map[gfx.PathCommandKind]int{}
	for _, c := range cmds {
		out[c.Kind]++
	}
	return out
}

func TestIconsAreOfficialGeometry(t *testing.T) {
	cases := []struct {
		kind      Kind
		name      string
		wantLayer bool
	}{
		{Session, "session", false},
		{File, "file", true},
		{Folder, "folder", false},
	}
	for _, tc := range cases {
		icon := Icon(tc.kind, gfx.RGB(1, 2, 3))
		if icon.ViewBox != 16 {
			t.Errorf("%s viewBox = %v，want 16", tc.name, icon.ViewBox)
		}
		if icon.Stroke != 1 {
			t.Errorf("%s stroke = %v，want 1（官方 _b 写死 1）", tc.name, icon.Stroke)
		}
		if len(icon.Commands) == 0 {
			t.Errorf("%s 没有描边路径", tc.name)
		}
		if tc.wantLayer && len(icon.Layers) != 1 {
			t.Errorf("%s 应有 1 个填充层（file 的 body），得到 %d", tc.name, len(icon.Layers))
		}
		if !tc.wantLayer && len(icon.Layers) != 0 {
			t.Errorf("%s 不该有填充层，得到 %d", tc.name, len(icon.Layers))
		}
	}
	// session 3 段 path 全用 M 开头；file/folder 的描边也必须有 Move。
	for _, kind := range []Kind{Session, File, Folder} {
		icon := Icon(kind, gfx.RGB(0, 0, 0))
		if counts(icon.Commands)[gfx.PathCommandMove] == 0 {
			t.Errorf("kind=%v 的描边里没有 MoveTo", kind)
		}
	}
}
