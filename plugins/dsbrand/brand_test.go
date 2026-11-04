package dsbrand

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

// 品牌面是**每个截图里都有**的东西：侧栏顶上的鲸鱼、首屏的 `探索未至之境`。
// 它挂掉的样子很特别——不报错、不崩，只是**那块地方空了**，
// 而人看到的是"这个界面没做完"。
//
// 具体怎么挂：`FishLogoPath` 是 3448 字节的官方原文，解析一次缓存成 ui.Icon。
// 解析失败（ok=false）时图标就是空的。所以这一层的测试不是"覆盖率"，
// 是"logo 还在不在"。

// countKinds 数一遍路径里各类命令的条数。
func countKinds(cmds []ui.PathCommand) map[ui.PathCommandKind]int {
	out := map[ui.PathCommandKind]int{}
	for _, c := range cmds {
		out[c.Kind]++
	}
	return out
}

// TestOfficialLogoPathStillParses 是这个包最重要的一条。
//
// 它同时钉住**常量**和**解析器**：
//   - 常量被改了（换来源、手改一个数）→ 解析可能失败 → 这里红；
//   - 解析器被改了（少认一种命令、数读错）→ 这里也红。
//
// 官方路径的形状（文件头写的）：4 个 M、71 个 C、1 个 L、4 个 Z。
// 数量对不上不一定是错（官方可能换图），但**必须有人在这次提交里看见它变了**。
func TestOfficialLogoPathStillParses(t *testing.T) {
	cmds, ok := ParseSVGPath(FishLogoPath)
	if !ok {
		t.Fatal("官方 logo 路径解析失败了——侧栏和首屏的鲸鱼会变成空白，且不会有任何报错")
	}
	if len(cmds) == 0 {
		t.Fatal("解析成功但一条命令都没有")
	}
	got := countKinds(cmds)
	want := map[ui.PathCommandKind]int{
		ui.PathCommandMove:  4,
		ui.PathCommandCubic: 71,
		ui.PathCommandLine:  1,
		ui.PathCommandClose: 4,
	}
	for kind, n := range want {
		if got[kind] != n {
			t.Errorf("%v 命令 %d 条，want %d（官方路径的形状变了？改之前先确认来源）", kind, got[kind], n)
		}
	}
	if got[ui.PathCommandQuad] != 0 {
		t.Errorf("出现 %d 条二次曲线命令——官方路径里没有，解析器不该凭空产出", got[ui.PathCommandQuad])
	}
}

// TestLogoIconIsNotBlank 走完整条链：路径 → 缓存图标。
//
// `FishIcon()` 内部用 sync.Once，所以这条同时验证"缓存那一步没把结果吞掉"。
func TestLogoIconIsNotBlank(t *testing.T) {
	icon := FishIcon()
	if len(icon.Commands) == 0 {
		t.Fatal("FishIcon() 是空的——界面上的鲸鱼会不见（解析失败或缓存没写进去）")
	}
	// 再取一次必须一致：sync.Once 没被写坏。
	again := FishIcon()
	if len(again.Commands) != len(icon.Commands) {
		t.Fatalf("两次 FishIcon() 命令数不同：%d vs %d", len(icon.Commands), len(again.Commands))
	}
}

// TestParserRejectsWhatItDoesNotSupport 钉住那个**故意的取舍**：
// 只认 M/L/C/Z 绝对命令，别的一律整条作废。
//
// 为什么不"尽量画"：半个 logo 比没有 logo 更像 bug——用户会以为界面坏了，
// 而它看起来"只是有点不对"。
func TestParserRejectsWhatItDoesNotSupport(t *testing.T) {
	cases := []struct{ name, d string }{
		{"相对命令 l", "M0 0 l10 10 Z"},
		{"相对命令 c", "M0 0 c1 1 2 2 3 3 Z"},
		{"弧线 A", "M0 0 A 5 5 0 0 1 10 10 Z"},
		{"二次曲线 Q", "M0 0 Q 5 5 10 10 Z"},
		{"平滑曲线 S", "M0 0 S 5 5 10 10 Z"},
		{"未知命令 X", "M0 0 X 1 1 Z"},
		{"缺参数的 M（只有 x）", "M 1 L 2 2 Z"},
		{"空串", ""},
		{"只有数字", "12 34"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmds, ok := ParseSVGPath(tc.d)
			if ok {
				t.Fatalf("%q 不该被接受（解析出 %d 条命令）——画一半比不画更糟", tc.d, len(cmds))
			}
		})
	}
}

// TestParserAcceptsTheSubset 是上一条的**正向对照**。
//
// 没有它的话，把 ParseSVGPath 改成"永远返回 false"，上面那条照样全绿。
func TestParserAcceptsTheSubset(t *testing.T) {
	cmds, ok := ParseSVGPath("M1.5 2.5 L0 0 C1 2 3 4 5 6 Z")
	if !ok {
		t.Fatal("合法的 M/L/C/Z 被拒了")
	}
	got := countKinds(cmds)
	if got[ui.PathCommandMove] != 1 || got[ui.PathCommandLine] != 1 ||
		got[ui.PathCommandCubic] != 1 || got[ui.PathCommandClose] != 1 {
		t.Fatalf("命令数不对：%v", got)
	}
	// 小数、负数、科学计数法都是官方路径里真出现过的写法（`-0.0263635`、`1e-3`）。
	for _, d := range []string{"M-0.0263635 1e-3 L 2 3 Z", "M.5 .5 L 1 1 Z"} {
		if _, ok := ParseSVGPath(d); !ok {
			t.Errorf("%q 应当能解析（负数/小数/科学计数法都在官方路径里出现过）", d)
		}
	}
}

// TestFishAspect 钉住那个系数**怎么用**——因为这个函数被误解过一次。
//
// 它不是"给宽度算高度"，是"高度 = 宽度 × 它"：
//
//	FishAspect(34, 0.7357) = 0.0216   ← 按"直接算高度"读会得到这个，看着像坏了
//	34 * FishAspect(FishViewBoxW, FishViewBoxH) = 25.02  ← 真实用法
func TestFishAspect(t *testing.T) {
	ratio := FishAspect(FishViewBoxW, FishViewBoxH)
	if got := 34 * ratio; got < 24.9 || got > 25.1 {
		t.Errorf("34 × FishAspect(viewBox) = %v，want ≈25.02", got)
	}
	// 非正宽度不该算出 NaN/Inf：退回 1 是个明确定义的退化值。
	if got := FishAspect(0, 17.04); got != 1 {
		t.Errorf("FishAspect(0, …) = %v，want 1（宽度为 0 时的退化值）", got)
	}
}

// TestLogoPathIsNotTruncated 防"复制粘贴截断"。
//
// 3.4KB 的常量最容易在传递中被截掉尾巴，而截断后的路径**仍然可能解析成功**
// （最后一段变成开区间），于是 logo 少一块——看起来像"设计就是这样"。
func TestLogoPathIsNotTruncated(t *testing.T) {
	d := strings.TrimSpace(FishLogoPath)
	if !strings.HasPrefix(d, "M") {
		t.Errorf("路径应当以 M 开头，实际 %q", d[:min(20, len(d))])
	}
	if !strings.HasSuffix(d, "Z") {
		t.Errorf("路径应当以 Z 结尾（闭合），实际结尾 %q", d[max(0, len(d)-20):])
	}
	if len(d) < 3000 {
		t.Errorf("路径只有 %d 字节，官方原文是 3448——像是被截断了", len(d))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
