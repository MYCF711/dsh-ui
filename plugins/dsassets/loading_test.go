package dsassets

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// 官方 `loading-outline` 的形状常量（由 `loadingOutlinePath` 的坐标反算得出，
// 不是设计参数）：圆心 (8,8)、半径 6.5，起点 45°（右下），每段 33.75°、共 8 段。
const (
	loadingOutlineCenterX = 8
	loadingOutlineCenterY = 8
	loadingOutlineRadius  = 6.5
	loadingOutlineStart   = 45
	loadingOutlineStep    = 33.75
	loadingOutlineSegs    = 8
)

func loadingOutlineIcon(t *testing.T) gfx.Icon {
	t.Helper()
	icon, ok := LoadingOutline(gfx.Color{Alpha: 1})
	if !ok {
		t.Fatal("官方 loading-outline 应该能画（一条 path、无 cap/join 冲突），Build 却返回 false")
	}
	return icon
}

// TestLoadingOutlineCompilesEveryOfficialCommand 钉住"整条 d 都吃完了"。
//
// 官方 `d` 是 1 个 M + 8 个 C：少一条＝解析截断，多一条＝把坐标当成了命令。
func TestLoadingOutlineCompilesEveryOfficialCommand(t *testing.T) {
	icon := loadingOutlineIcon(t)
	if icon.ViewBox != 16 {
		t.Fatalf("viewBox=%v，官方是 16", icon.ViewBox)
	}
	if icon.Stroke != regularStroke {
		t.Fatalf("stroke=%v，官方 Regular 档是 %v", icon.Stroke, float32(regularStroke))
	}
	// 官方这条 path 没写 stroke-linecap / stroke-linejoin，取 SVG 默认值，
	// 正好是 gfx.Icon 的零值；写成别的就是自造。
	if icon.Cap != gfx.CapButt || icon.Join != gfx.JoinMiter {
		t.Fatalf("cap=%v join=%v，官方没写 linecap/linejoin 即默认 butt/miter", icon.Cap, icon.Join)
	}
	if len(icon.Layers) != 0 {
		t.Fatalf("纯描边素材不该有填充层，却给了 %d 层", len(icon.Layers))
	}
	want := 1 + loadingOutlineSegs
	if len(icon.Commands) != want {
		t.Fatalf("命令数=%d，官方 d 是 1 个 M + %d 个 C = %d", len(icon.Commands), loadingOutlineSegs, want)
	}
	if icon.Commands[0].Kind != gfx.PathCommandMove {
		t.Fatalf("第一条命令不是 M：%v", icon.Commands[0].Kind)
	}
	for i, command := range icon.Commands {
		if i == 0 {
			continue
		}
		if command.Kind != gfx.PathCommandCubic {
			t.Fatalf("第 %d 条命令是 %v，官方这一段全是三次曲线", i, command.Kind)
		}
	}
	if ok := (Asset{ViewBox: 16, Elements: []Element{{Kind: PathElement, D: loadingOutlinePath, Stroke: true}}}).Buildable(); !ok {
		t.Fatal("素材被标成不可画")
	}
}

// TestLoadingOutlineIsTheOfficialCircularArc 复算那是一条 (8,8) r=6.5 的圆弧。
//
// 这是**可判定的几何事实**，不是"看着像"：官方 8 段三次贝塞尔的端点必须逐个
// 落在半径 6.5 的圆上、角度从 45° 起每段走 33.75°，控制点落在三次贝塞尔逼近
// 圆弧的经典外扩半径上（约 6.6255），曲线本身与真圆弧的最大径向误差 < 0.001。
func TestLoadingOutlineIsTheOfficialCircularArc(t *testing.T) {
	icon := loadingOutlineIcon(t)
	radius := func(x, y float32) float64 {
		return math.Hypot(float64(x-loadingOutlineCenterX), float64(y-loadingOutlineCenterY))
	}
	angle := func(x, y float32) float64 {
		return math.Atan2(float64(y-loadingOutlineCenterY), float64(x-loadingOutlineCenterX)) * 180 / math.Pi
	}

	previous := icon.Commands[0].Point
	if got := radius(previous.X, previous.Y); math.Abs(got-loadingOutlineRadius) > 0.01 {
		t.Fatalf("起点半径=%.4f，官方是 %.1f", got, loadingOutlineRadius)
	}
	for i := 1; i < len(icon.Commands); i++ {
		command := icon.Commands[i]
		if got := radius(command.Point.X, command.Point.Y); math.Abs(got-loadingOutlineRadius) > 0.01 {
			t.Fatalf("第 %d 段终点半径=%.4f，官方是 %.1f（端点必须在圆上）", i, got, loadingOutlineRadius)
		}
		want := loadingOutlineStart + float64(i)*loadingOutlineStep
		got := angle(command.Point.X, command.Point.Y)
		// atan2 把 180° 之后折回负数区间，按一整圈折算再比。
		for got < 0 {
			got += 360
		}
		if math.Abs(got-want) > 0.01 {
			t.Fatalf("第 %d 段终点角度=%.3f°，按官方应为 %.2f°（45° + %d×33.75°）", i, got, want, i)
		}
		// 控制点的半径是三次贝塞尔逼近圆弧的外扩值，官方 8 段一致的 ~6.6255。
		for _, control := range []gfx.Point{command.Control1, command.Control2} {
			if r := radius(control.X, control.Y); math.Abs(r-6.6255) > 0.01 {
				t.Fatalf("第 %d 段控制点半径=%.4f，与官方圆弧逼近值 6.6255 差得太多", i, r)
			}
		}
		// 曲线中点也必须贴在圆上（官方 svg 画的是一条真圆弧）。
		mid := loadingCubicAt(previous, command.Control1, command.Control2, command.Point, 0.5)
		if r := radius(mid.X, mid.Y); math.Abs(r-loadingOutlineRadius) > 0.01 {
			t.Fatalf("第 %d 段曲线中点半径=%.4f，偏离 %.1f°，说明它不是圆弧", i, r, loadingOutlineRadius)
		}
		previous = command.Point
	}

	// 缺口：从 315°（最后一段终点）回到 45°（起点）是 90°，正东那一块。
	end := angle(previous.X, previous.Y)
	for end < 0 {
		end += 360
	}
	gap := 360 - end + loadingOutlineStart
	if math.Abs(gap-90) > 0.01 {
		t.Fatalf("缺口=%.3f°，官方是正东 90° 的缺口", gap)
	}
}

// TestLoadingOutlineStaysInsideItsViewBox 兜住"相对命令被当成绝对"这类解析错误。
func TestLoadingOutlineStaysInsideItsViewBox(t *testing.T) {
	asset := Asset{ViewBox: 16, Elements: []Element{{Kind: PathElement, D: loadingOutlinePath, Stroke: true}}}
	minX, minY, maxX, maxY, ok := Bounds(asset)
	if !ok {
		t.Fatal("Bounds 拿不到几何")
	}
	if minX < 0 || minY < 0 || maxX > 16 || maxY > 16 {
		t.Fatalf("包围盒 [%v,%v]-[%v,%v] 越出 16 viewBox", minX, minY, maxX, maxY)
	}
	// 逐边都由官方圆弧算得出来，不是抄的：
	//   - 最左 = 正西那个端点 = 8-6.5 = 1.5（缺口在正东，所以右边到不了 14.5）；
	//   - 最右 = 起点 (12.596,12.596) = 8+6.5/√2；
	//   - 上下 = 控制点外扩半径 6.6255，即 8∓6.6255。
	for _, side := range []struct {
		name string
		got  float32
		want float64
	}{
		{"minX", minX, loadingOutlineCenterX - loadingOutlineRadius},
		{"maxX", maxX, loadingOutlineCenterX + loadingOutlineRadius/math.Sqrt2},
		{"minY", minY, loadingOutlineCenterY - 6.6255},
		{"maxY", maxY, loadingOutlineCenterY + 6.6255},
	} {
		if math.Abs(float64(side.got)-side.want) > 0.01 {
			t.Fatalf("%s=%v，按官方圆弧应为 %.4f", side.name, side.got, side.want)
		}
	}
}

// loadingCubicAt 求一段三次贝塞尔上 t 处的点。
func loadingCubicAt(p0, p1, p2, p3 gfx.Point, t float32) gfx.Point {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return gfx.Point{
		X: a*p0.X + b*p1.X + c*p2.X + d*p3.X,
		Y: a*p0.Y + b*p1.Y + c*p2.Y + d*p3.Y,
	}
}
