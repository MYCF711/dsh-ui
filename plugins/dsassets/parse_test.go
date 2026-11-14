package dsassets

import (
	"math"
	"testing"

	"github.com/MYCF711/dsh-ui/plugins/dsbrand"
	"github.com/vivaui/viva/gfx"
)

// TestParseCommandCounts 钉住每条素材解析出来的命令数。
//
// 这些数字是"整条路径都吃完了"的证据：官方 `d` 里有几段几何，就该有几条命令。
// 少一条＝被截断；多一条＝把坐标当成了命令。数字不是手数的，是先跑出来再写死
// （见跟这次改动一起提交的 docs/official-assets.md「命令数」一列）。
func TestParseCommandCounts(t *testing.T) {
	want := map[string]int{
		"composer-send":              16,
		"send-outline":               12,
		"paperclip":                  17,
		"composer-add":               4,
		"thumb-up-outline":           18,
		"thumb-up-fill":              18,
		"thumb-down-outline":         18,
		"thumb-down-fill":            18,
		"permission-read-only":       13,
		"permission-workspace-write": 45,
		"permission-full-access":     13,
		"permission-auto-review":     14,
	}
	for _, a := range All() {
		if !a.Buildable() {
			continue
		}
		expect, ok := want[a.ID]
		if !ok {
			t.Fatalf("素材 %s 没有登记期望命令数（新素材要一起写进测试）", a.ID)
		}
		if got := CommandCount(a); got != expect {
			t.Errorf("%s 命令数 = %d，want %d", a.ID, got, expect)
		}
	}
	if len(want) != len(All())-3 {
		t.Errorf("期望表里有 %d 条可构建素材，All() 里是 %d 条（3 条不可构建）", len(want), len(All()))
	}
}

// TestParseBoundsStayInViewBox：16 viewBox 的图标，所有端点/控制点必须落在 [0,16]。
//
// 越界最典型的来源是把相对命令（小写）当成绝对的：一步之差会让整条路径飞出画布，
// 而屏幕上看起来只是"图标偏了"。
func TestParseBoundsStayInViewBox(t *testing.T) {
	for _, a := range All() {
		if !a.Buildable() {
			continue
		}
		minX, minY, maxX, maxY, ok := Bounds(a)
		if !ok {
			t.Fatalf("%s 没有解析出任何坐标", a.ID)
		}
		if minX < -0.001 || minY < -0.001 || maxX > a.ViewBox+0.001 || maxY > a.ViewBox+0.001 {
			t.Errorf("%s 包围盒 [%v %v %v %v] 越出 viewBox %v", a.ID, minX, minY, maxX, maxY, a.ViewBox)
		}
		// 也不能缩成一个小点：那说明数字解析全错。
		if maxX-minX < 1 || maxY-minY < 1 {
			t.Errorf("%s 包围盒太小 (%v x %v)，解析疑似失败", a.ID, maxX-minX, maxY-minY)
		}
	}
}

// TestParseHVEqualsLine 证明 H/V 展开是**恒等变形**：拿官方那条发送箭头
// （含 1 个 V、1 个 H）和手写成 L 的等价路径比，命令必须逐字节相同。
func TestParseHVEqualsLine(t *testing.T) {
	withHV, ok := ParsePath("M1 2H9V8H1Z")
	if !ok {
		t.Fatal("H/V 路径没解析出来")
	}
	withLines, ok := ParsePath("M1 2L9 2L9 8L1 8Z")
	if !ok {
		t.Fatal("L 路径没解析出来")
	}
	if len(withHV) != len(withLines) {
		t.Fatalf("命令数不同：H/V=%d L=%d", len(withHV), len(withLines))
	}
	for i := range withHV {
		if withHV[i] != withLines[i] {
			t.Fatalf("第 %d 条命令不同：H/V=%+v L=%+v", i, withHV[i], withLines[i])
		}
	}
}

// TestParseRelativeMatchesAbsolute 证明相对命令累加正确。
func TestParseRelativeMatchesAbsolute(t *testing.T) {
	rel, ok := ParsePath("m1 2 l3 4 h5 v6 c1 1 2 2 3 3 z")
	if !ok {
		t.Fatal("相对命令没解析出来")
	}
	abs, ok := ParsePath("M1 2L4 6L9 6L9 12C10 13 11 14 12 15Z")
	if !ok {
		t.Fatal("绝对命令没解析出来")
	}
	if len(rel) != len(abs) {
		t.Fatalf("命令数不同：相对=%d 绝对=%d", len(rel), len(abs))
	}
	for i := range rel {
		if !sameCommand(rel[i], abs[i]) {
			t.Fatalf("第 %d 条命令不同：相对=%+v 绝对=%+v", i, rel[i], abs[i])
		}
	}
}

// TestParseSReflectsPreviousCubic：`s` 的第一个控制点是上一段三次曲线控制点的镜像。
func TestParseSReflectsPreviousCubic(t *testing.T) {
	got, ok := ParsePath("M0 0C1 2 3 4 5 6S7 8 9 10")
	if !ok {
		t.Fatal("S 命令没解析出来")
	}
	if len(got) != 3 {
		t.Fatalf("命令数 = %d，want 3", len(got))
	}
	// 当前点 (5,6)，上一段第二控制点 (3,4) → 镜像 (7,8)。
	want := gfx.CubicTo(7, 8, 7, 8, 9, 10)
	if got[2] != want {
		t.Fatalf("s 的镜像控制点 = %+v，want %+v", got[2], want)
	}
}

// TestParseRejectsUnsupportedCommands：弧和二次曲线**整条作废**，不画一半。
//
// viva 的 PathCommand 没有弧/二次曲线；把它们近似成贝塞尔就是"看着差不多"，
// 所以这里要求硬失败——将来某条官方素材真用了弧，调用方会立刻发现。
func TestParseRejectsUnsupportedCommands(t *testing.T) {
	for _, d := range []string{
		"M0 0A1 1 0 0 1 2 2Z",
		"M0 0Q1 1 2 2Z",
		"M0 0T2 2Z",
		"",
		"M0 0 foo",
		"M",
	} {
		if _, ok := ParsePath(d); ok {
			t.Errorf("ParsePath(%q) 应该失败（不支持的命令/残缺路径）", d)
		}
	}
}

// TestParseAgreesWithDSBrandOnFishLogo：两处解析器在重叠子集上必须同解。
//
// 全仓只有 dsbrand.ParseSVGPath 一份路径解析器（本包只做"先展开成绝对 M/L/C/Z"），
// 这条断言就是那句话的证据：把官方鲸鱼路径从本包走一遍，和直接交给 dsbrand 比，
// 命令必须完全一致。
func TestParseAgreesWithDSBrandOnFishLogo(t *testing.T) {
	viaDSAssets, ok := ParsePath(dsbrand.FishLogoPath)
	if !ok {
		t.Fatal("鲸鱼路径经 dsassets 解析失败")
	}
	direct, ok := dsbrand.ParseSVGPath(dsbrand.FishLogoPath)
	if !ok {
		t.Fatal("鲸鱼路径经 dsbrand 解析失败")
	}
	if len(viaDSAssets) != len(direct) {
		t.Fatalf("命令数不同：dsassets=%d dsbrand=%d", len(viaDSAssets), len(direct))
	}
	for i := range viaDSAssets {
		if viaDSAssets[i] != direct[i] {
			t.Fatalf("第 %d 条命令不同：dsassets=%+v dsbrand=%+v", i, viaDSAssets[i], direct[i])
		}
	}
}

// sameCommand 比较两条命令（浮点用极小容差，只用于相对命令累加的末位误差）。
func sameCommand(a, b gfx.PathCommand) bool {
	if a.Kind != b.Kind {
		return false
	}
	near := func(x, y float32) bool { return math.Abs(float64(x-y)) < 1e-5 }
	return near(a.Point.X, b.Point.X) && near(a.Point.Y, b.Point.Y) &&
		near(a.Control1.X, b.Control1.X) && near(a.Control1.Y, b.Control1.Y) &&
		near(a.Control2.X, b.Control2.X) && near(a.Control2.Y, b.Control2.Y)
}
