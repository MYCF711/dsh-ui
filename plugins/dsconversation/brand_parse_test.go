package dsconversation

import (
	"math"
	"testing"

	"github.com/vivaui/viva/ui"
)

// TestFishLogoPathIsOfficialBytes 钉住"品牌路径就是官方那一串"。
//
// 3448 是官方 FISH_LOGO_PATH 的字节数（从 asar 里
// `@deepseek-ai/dsh-client-ui-primitives` 取出来的原文长度）。
// 这条断言的意义：以后谁"顺手美化"这个字符串、或者从别处复制了近似形状，
// 这里会立刻红——首屏 logo 是品牌面，不能靠肉眼看着像。
func TestFishLogoPathIsOfficialBytes(t *testing.T) {
	if got := len(fishLogoPath); got != 3448 {
		t.Fatalf("fishLogoPath 长度 = %d，官方原文是 3448", got)
	}
	if fishLogoPath[0] != 'M' {
		t.Fatalf("路径必须以 M 开头，实际 %q", fishLogoPath[:1])
	}
}

// TestFishViewBox 钉住官方 FISH_LOGO_VIEWBOX（23.16 x 17.04）。
func TestFishViewBox(t *testing.T) {
	if fishViewBoxW != 23.16 || fishViewBoxH != 17.04 {
		t.Fatalf("viewBox = %v x %v，官方是 23.16 x 17.04", fishViewBoxW, fishViewBoxH)
	}
	// 34px 宽时官方高度 = 34 * 17.04 / 23.16 = 25.02。
	got := heroFishWidth * fishAspect(fishViewBoxW, fishViewBoxH)
	want := float32(34 * 17.04 / 23.16)
	if math.Abs(float64(got-want)) > 0.01 {
		t.Fatalf("鱼高 = %v，want %v", got, want)
	}
	if math.Abs(float64(heroFishHeight-25.02)) > 0.01 {
		t.Fatalf("heroFishHeight = %v，want 25.02", heroFishHeight)
	}
}

// TestParseFishPathCommandCounts：官方这条路径只用 M/L/C/Z，
// 数过了是 4 个 M、71 个 C、1 个 L、4 个 Z，一共 80 条命令。
//
// 这个数字是"解析器真的把整条路径吃完了"的证据：少一条就是截断，
// 多一条就是把坐标当成了命令。
func TestParseFishPathCommandCounts(t *testing.T) {
	commands, ok := parseSVGPath(fishLogoPath)
	if !ok {
		t.Fatal("官方路径解析失败")
	}
	if len(commands) != 80 {
		t.Fatalf("命令数 = %d，want 80", len(commands))
	}
	counts := map[ui.PathCommandKind]int{}
	for _, c := range commands {
		counts[c.Kind]++
	}
	if counts[ui.PathCommandMove] != 4 {
		t.Fatalf("M = %d，want 4", counts[ui.PathCommandMove])
	}
	if counts[ui.PathCommandCubic] != 71 {
		t.Fatalf("C = %d，want 71", counts[ui.PathCommandCubic])
	}
	if counts[ui.PathCommandLine] != 1 {
		t.Fatalf("L = %d，want 1", counts[ui.PathCommandLine])
	}
	if counts[ui.PathCommandClose] != 4 {
		t.Fatalf("Z = %d，want 4", counts[ui.PathCommandClose])
	}
}

// TestParseFishPathGeometry：解析出来的点必须落在 viewBox 里。
// 坐标错位（比如把 C 的 6 个参数当成 2 个点）会让图形炸到画布外，
// 而它照样"能画"——所以要看值域。
func TestParseFishPathGeometry(t *testing.T) {
	commands, ok := parseSVGPath(fishLogoPath)
	if !ok {
		t.Fatal("解析失败")
	}
	for i, c := range commands {
		// 曲线终点必须落在 viewBox 里：坐标错位（比如把 C 的 6 个参数当成 2 个点）
		// 会让图形炸到画布外，而它照样"能画"——所以要看值域。
		if c.Kind == ui.PathCommandClose {
			continue
		}
		if c.Point.X < 0 || c.Point.X > fishViewBoxW || c.Point.Y < 0 || c.Point.Y > fishViewBoxH {
			t.Fatalf("第 %d 条命令的终点 (%v,%v) 出界（viewBox %v x %v）",
				i, c.Point.X, c.Point.Y, fishViewBoxW, fishViewBoxH)
		}
		// 控制点允许出界：官方原路径里就有 -0.222916 这样的负控制点
		// （SVG 允许控制点在形状外，曲线本身仍在框内）。留 3 个单位的余量。
		for _, p := range []ui.Point{c.Control1, c.Control2} {
			if p.X < -3 || p.X > fishViewBoxW+3 || p.Y < -3 || p.Y > fishViewBoxH+3 {
				t.Fatalf("第 %d 条命令的控制点 (%v,%v) 离 viewBox 太远", i, p.X, p.Y)
			}
		}
	}
}

// TestParseFishPathRejectsUnknownCommands：不认识的命令整条作废，
// 绝不"画一半"（半个 logo 比没有 logo 更像 bug）。
func TestParseFishPathRejectsUnknownCommands(t *testing.T) {
	cases := []string{
		"M0 0 c1 1 2 2 3 3",   // 相对命令
		"M0 0 A1 1 0 0 1 2 2", // 弧线
		"M0 0 Q1 1 2 2",       // 二次曲线
		"M0 0 C1 1 2 2",       // 参数不够
		"",                    // 空
		"M",                   // 只有命令没有参数
	}
	for _, src := range cases {
		if _, ok := parseSVGPath(src); ok {
			t.Fatalf("parseSVGPath(%q) 应该失败", src)
		}
	}
}

// TestFishIcon 保证 fishIcon 返回的是"解析成功"的图标，
// 而不是缓存了一个空 Icon（空 Icon 在 flex 里会被静默跳过——首屏就没 logo 了）。
func TestFishIcon(t *testing.T) {
	icon := fishIcon()
	if icon.ViewBox != fishViewBoxW || icon.ViewHeight != fishViewBoxH {
		t.Fatalf("fishIcon 的 viewBox = %v x %v，want %v x %v",
			icon.ViewBox, icon.ViewHeight, fishViewBoxW, fishViewBoxH)
	}
	if len(icon.Commands) != 80 {
		t.Fatalf("fishIcon 命令数 = %d，want 80", len(icon.Commands))
	}
	// 纯填充：Stroke 为 0，颜色由 f.Icon 的 tint 给（官方 .fish{color:label-primary}）。
	if icon.Stroke != 0 {
		t.Fatalf("fishIcon 应该走填充而不是描边，Stroke = %v", icon.Stroke)
	}
}
