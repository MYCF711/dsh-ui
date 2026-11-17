package dsbrand

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"os"
	"regexp"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// 官方品牌名 artwork 的断言。
//
// 这一组测试问的**不是覆盖率**，是三个容易悄悄坏掉的问题：
//
//  1. 9+7 条路径是不是仍是官方原文（sha256 逐条钉住，手改一个数字就红）；
//  2. 侧栏品牌区的几何是不是仍是官方值（52 / 46 / 3 / 2 / 156 / 24 —— 这些数被抄错过）；
//  3. 本文件的路径与仓内官方素材（assets/official/brand/deepseek-harness.svg）是否仍是同一份 artwork。
//
// 为什么值得钉：logo 坏了不报错、不崩，只是那块地方变成空白或错位，
// 而人看到的是「这界面没做完」。

// officialBrandNameSHA256 是官方 9 条字形路径的 sha256（顺序 = brandNamePaths）。
var officialBrandNameSHA256 = [9]string{
	"7257e49a33c5b3c9dcd203f63dd4d39bf88086d9f45db7ff2915efc98d43b71a",
	"140164197416b55048a8ef75ce58c55c13e2609795bb013b6e1dbc598cb076f0",
	"5e0b9b4e753f9b5e635a1e890aed8bb4ccc1a448d356e6d88c897a26109ea82e",
	"d52c270ba77dfed0a176afed120b91a5b622afd791b928b44b18c8fcbdedd272",
	"fa32d391546e34a6a4c61160ca281cee699d1a9113d7db8ce4cbbefaf9002d49",
	"cfe1158991f15fb4f7a06f64237e00cec74b6dcf2190b84a0e06616acb87b1cb",
	"3a5ce73ed7f3e7cd93dd9d67bbe55c8496f8a56e74743dc5b90aee8ca3315f2a",
	"a1a5cac07c3311fad06b081fc1e9e11aa3e7dc26923d36448ddb07609b015f8e",
	"bbb5519d1c1719690495b78bc676d9c17cbdb14a6cc2220f6b7594b315a26d78",
}

// officialBadgeInkSHA256 是官方徽标内 7 条反色字形的 sha256。
var officialBadgeInkSHA256 = [7]string{
	"b481226fed18eb6da955c23d3e7226a5d39ad01fd4ae8adffc3046161a69b2dd",
	"b2dc3ebdfa1a2d22d6d348882c0a1d3663bac31c1b0121393e7c7cf10641987e",
	"be9b8c22cf0f708234a4a4a7e7de425e2774e95376ccac69d3eb4ce27b4f0097",
	"f1a5287499a0f27b91911cbc7473a2c666ee3ac74f48017de0469a31661a168b",
	"1f2f450be8ed0315d3961c969d673395da52cd2e31beb5b187ced5bea2e84149",
	"e8f349eb7783705e0c137789f68be2e273bf45034549b5c2c955d5cb53d981fd",
	"0a8bcb64d3df6f9f6345376d1c38b5abaf9ea48a5f48c09153f82026567ab34a",
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestBrandNamePathsAreOfficialBytes 钉住「这些路径是从官方抄来的，不是画出来的」。
func TestBrandNamePathsAreOfficialBytes(t *testing.T) {
	if len(brandNamePaths) != len(officialBrandNameSHA256) {
		t.Fatalf("字形路径 %d 条，官方 %d 条", len(brandNamePaths), len(officialBrandNameSHA256))
	}
	for i, want := range officialBrandNameSHA256 {
		if got := digest(brandNamePaths[i]); got != want {
			t.Errorf("字形路径[%d] 不是官方原文：sha256=%s，want %s", i, got, want)
		}
	}
}

// TestBrandBadgeInkPathsAreOfficialBytes 同上，钉住徽标里的反色字形。
func TestBrandBadgeInkPathsAreOfficialBytes(t *testing.T) {
	if len(brandBadgeInkPaths) != len(officialBadgeInkSHA256) {
		t.Fatalf("徽标字形 %d 条，官方 %d 条", len(brandBadgeInkPaths), len(officialBadgeInkSHA256))
	}
	for i, want := range officialBadgeInkSHA256 {
		if got := digest(brandBadgeInkPaths[i]); got != want {
			t.Errorf("徽标字形[%d] 不是官方原文：sha256=%s，want %s", i, got, want)
		}
	}
}

// TestBrandZoneGeometryIsOfficial 钉住侧栏品牌区那几个**已经被抄错过**的数。
//
// 断言写成「等于官方值」而不是「等于某个常量」：常量本身被改也会红。
func TestBrandZoneGeometryIsOfficial(t *testing.T) {
	cases := []struct {
		name string
		got  float32
		want float32
		src  string
	}{
		{"侧栏标记边长 SidebarMarkSize", SidebarMarkSize, 24, "SidebarRoot.tsx:297 renderSlot('sidebar.brand.mark', { size: 24 })"},
		{"首屏鲸鱼宽 HeroMarkWidth", HeroMarkWidth, 34, "EmptyHero.tsx:104 width={34}"},
		{"品牌名 size BrandNameSize", BrandNameSize, 24, "Brand.tsx:18 BrandWordmark 默认 size"},
		{"品牌名 viewBox 宽", BrandNameViewBoxW, 156, `BrandWordmark includeMark=false viewBox "26 0 156 24"`},
		{"品牌名 viewBox 高", BrandNameViewBoxH, 24, `同上`},
		{"徽标板 x", BadgePlateX, 129.348, `BrandWordmark <rect x="129.348">`},
		{"徽标板 y", BadgePlateY, 5.5, `BrandWordmark <rect y="5.5">`},
		{"徽标板宽（不是 46）", BadgePlateW, 52, `BrandWordmark <rect width="52">`},
		{"徽标板高", BadgePlateH, 14, `BrandWordmark <rect height="14">`},
		{"徽标圆角（不是 theme.XS=4）", BadgeRadius, 2, `BrandWordmark <rect rx="2">`},
		{"徽标文字裁切区 x", BadgeClipX, 132.348, `clipPath#dsh-wordmark-badge-clip rect translate(132.348 5.5)`},
		{"徽标文字裁切区宽", BadgeClipW, 46, `同上 rect width=46`},
		{"徽标内缩", BadgeInset, 3, `132.348 - 129.348 = 52 - 46`},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v，官方 %v（%s）", c.name, c.got, c.want, c.src)
		}
	}
	// 与官方 wordmark 的横向布局自洽（都从 182x24 画布、includeMark=false 的 156 宽推出来）：
	//   板上右缘 = 129.348 + 52 - 26 = 155.348，还差 0.652 才是 156（官方不是正好贴边）。
	if right := BadgePlateX - brandNameMarkInset + BadgePlateW; right != 155.348 {
		t.Errorf("徽标板右缘 = %v，官方 155.348（129.348+52-26）", right)
	}
	//   文字区在板内**左右各内缩 3px**：52 - 46 = 6 = 3 + 3。
	if left := BadgeClipX - BadgePlateX; left != BadgeInset {
		t.Errorf("文字区左侧内缩 %v，官方 %v（132.348-129.348）", left, BadgeInset)
	}
	if right := BadgePlateX + BadgePlateW - (BadgeClipX + BadgeClipW); right != BadgeInset {
		t.Errorf("文字区右侧内缩 %v，官方 %v（181.348-178.348）", right, BadgeInset)
	}
}

// iconBox 复算一个 gfx.Icon 的实际坐标包围盒（含控制点，取保守外框）。
func iconBox(cmds []gfx.PathCommand) (minX, minY, maxX, maxY float32) {
	minX, minY = math.MaxFloat32, math.MaxFloat32
	maxX, maxY = -math.MaxFloat32, -math.MaxFloat32
	// 本包的 min/max 是 brand_test.go 里的 int 版本，这里不能用内建泛型版。
	acc := func(p gfx.Point) {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	for _, c := range cmds {
		switch c.Kind {
		case gfx.PathCommandCubic:
			acc(c.Control1)
			acc(c.Control2)
			acc(c.Point)
		case gfx.PathCommandClose:
		default:
			acc(c.Point)
		}
	}
	return minX, minY, maxX, maxY
}

// TestBrandNameIconIsNotBlank 走完整链：官方路径 → 平移 -26 → 缓存图标。
//
// 同时验证平移**只**在 x 方向挪 26：字形仍完整落在 156x24 的 viewBox 内，
// 且不越过徽标板起点（官方布局：字形 26..121.5，板从 129.348 起）。
func TestBrandNameIconIsNotBlank(t *testing.T) {
	icon := BrandNameIcon()
	if len(icon.Commands) == 0 {
		t.Fatal("BrandNameIcon() 是空的——侧栏品牌名会变成空白（解析失败或缓存没写进去）")
	}
	if icon.ViewBox != BrandNameViewBoxW || icon.ViewHeight != BrandNameViewBoxH {
		t.Fatalf("viewBox = %vx%v，want %vx%v", icon.ViewBox, icon.ViewHeight, BrandNameViewBoxW, BrandNameViewBoxH)
	}
	if icon.Stroke != 0 {
		t.Errorf("品牌名是纯填充，Stroke 应为 0，得到 %v", icon.Stroke)
	}
	minX, minY, maxX, maxY := iconBox(icon.Commands)
	if minX < 0 || minY < 0 || maxX > BrandNameViewBoxW || maxY > BrandNameViewBoxH {
		t.Errorf("字形越界：x %v..%v / y %v..%v 不在 0..%v x 0..%v 内",
			minX, maxX, minY, maxY, BrandNameViewBoxW, BrandNameViewBoxH)
	}
	if plateStart := float32(BadgePlateX - brandNameMarkInset); maxX > plateStart {
		t.Errorf("字形右缘 %v 越过了徽标板起点 %v——两者会叠在一起", maxX, plateStart)
	}
	again := BrandNameIcon()
	if len(again.Commands) != len(icon.Commands) {
		t.Fatalf("两次调用命令数不同：%d vs %d（sync.Once 被写坏？）", len(icon.Commands), len(again.Commands))
	}
}

// TestBrandBadgeInkIconFitsItsClip 钉住徽标字形落在官方裁切区（46x14）内。
func TestBrandBadgeInkIconFitsItsClip(t *testing.T) {
	icon := BrandBadgeInkIcon()
	if len(icon.Commands) == 0 {
		t.Fatal("BrandBadgeInkIcon() 是空的——徽标里会是一块空板")
	}
	if icon.ViewBox != BadgeClipW || icon.ViewHeight != BadgePlateH {
		t.Fatalf("viewBox = %vx%v，want %vx%v（官方文字裁切区）", icon.ViewBox, icon.ViewHeight, BadgeClipW, BadgePlateH)
	}
	minX, minY, maxX, maxY := iconBox(icon.Commands)
	// 0.5px 容差：官方靠 clipPath 硬裁，我们靠 viewBox，字形边缘允许压线。
	if minX < -0.5 || minY < -0.5 || maxX > BadgeClipW+0.5 || maxY > BadgePlateH+0.5 {
		t.Errorf("徽标字形越出裁切区：x %v..%v / y %v..%v，裁切区 0..%v x 0..%v",
			minX, maxX, minY, maxY, BadgeClipW, BadgePlateH)
	}
}

// TestTranslatePathShiftsEveryCoordinate 钉住平移函数本身：三种带坐标的命令都要挪，
// Close 不动；且传入 0 位移时原样返回（拷贝语义靠它保证）。
func TestTranslatePathShiftsEveryCoordinate(t *testing.T) {
	src := []gfx.PathCommand{
		gfx.MoveTo(10, 20),
		gfx.LineTo(30, 40),
		gfx.CubicTo(1, 2, 3, 4, 5, 6),
		gfx.ClosePath(),
	}
	moved := TranslatePath(src, -26, 8)
	if moved[0].Point.X != -16 || moved[0].Point.Y != 28 {
		t.Errorf("Move 平移错：%+v", moved[0])
	}
	if moved[1].Point.X != 4 || moved[1].Point.Y != 48 {
		t.Errorf("Line 平移错：%+v", moved[1])
	}
	c := moved[2]
	if c.Point.X != -21 || c.Point.Y != 14 || c.Control1.X != -25 || c.Control1.Y != 10 ||
		c.Control2.X != -23 || c.Control2.Y != 12 {
		t.Errorf("Cubic 平移错：%+v", c)
	}
	if moved[3].Kind != gfx.PathCommandClose {
		t.Errorf("Close 应当原样不动：%+v", moved[3])
	}
	// 原切片不能被就地改动：调用方可能还在用同一份命令。
	if src[0].Point.X != 10 || src[0].Point.Y != 20 {
		t.Errorf("TranslatePath 改动了入参：%+v", src[0])
	}
	if identity := TranslatePath(src, 0, 0); len(identity) != len(src) || identity[0].Point.X != 10 {
		t.Errorf("零位移应当原样返回：%+v", identity)
	}
}

// TestWordmarkArtworkMatchesOfficialAsset 把本文件的路径锚回**仓内官方素材**。
//
// assets/official/brand/deepseek-harness.svg 是 welcome 页的官方 brand 素材
// （sha256 已登记在 docs/official-assets-reuse.md:61），内容是同一份 182x24 lockup
// 平移 (145.327, 8) 后放进 472x40 画布。
//
// 所以：素材里的字形几何必须逐点等于「本文件路径 + 平移」，sha256 必须是登记值。
// 一旦有人为了「看着更像」改本文件路径，或替换了素材文件，两边立刻对不上——
// 而不是等到某张截图里 logo 变形才发现。
func TestWordmarkArtworkMatchesOfficialAsset(t *testing.T) {
	const wantSHA = "182f040f9d3eccf25b20f5bbfc9d50a21296a88978b91b4dec7e64cbd16b4039"
	const path = "../../assets/official/brand/deepseek-harness.svg"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("官方素材不在（%s）：%v", path, err)
	}
	if got := digest(string(raw)); got != wantSHA {
		t.Fatalf("%s 的 sha256 = %s，want 登记值 %s", path, got, wantSHA)
	}
	// 素材平移量 = welcome 画布里 lockup 起点 (145.327, 8)。
	const dx, dy float32 = 145.327, 8
	paths := regexp.MustCompile(`<path[^>]*\bd="([^"]+)"`).FindAllStringSubmatch(string(raw), -1)
	if len(paths) != 17 {
		t.Fatalf("%s 有 %d 条 path，官方 artwork 是 17 条（9 字形 + 1 鲸鱼 + 7 徽标字）", path, len(paths))
	}
	own := append(append([]string{}, brandNamePaths[:]...), brandBadgeInkPaths[:]...)
	// 素材第 9 条是鲸鱼（本包另有 FishLogoPath 服务首屏），所以两侧各跳过它。
	theirs := make([]string, 0, len(own))
	for _, m := range append(append([][]string{}, paths[:9]...), paths[10:17]...) {
		theirs = append(theirs, m[1])
	}
	for i := range own {
		want, ok := ParseSVGPath(own[i])
		if !ok {
			t.Fatalf("本文件第 %d 条路径解析失败", i)
		}
		got, ok := ParseSVGPath(theirs[i])
		if !ok {
			t.Fatalf("素材第 %d 条路径解析失败", i)
		}
		want = TranslatePath(want, dx, dy)
		if len(got) != len(want) {
			t.Fatalf("第 %d 条命令数不同：素材 %d，本文件平移后 %d", i, len(got), len(want))
		}
		for k := range want {
			checks := [][3]float32{
				{got[k].Point.X, want[k].Point.X, 0},
				{got[k].Point.Y, want[k].Point.Y, 0},
				{got[k].Control1.X, want[k].Control1.X, 0},
				{got[k].Control1.Y, want[k].Control1.Y, 0},
				{got[k].Control2.X, want[k].Control2.X, 0},
				{got[k].Control2.Y, want[k].Control2.Y, 0},
			}
			for _, c := range checks {
				if math.Abs(float64(c[0]-c[1])) > 0.01 {
					t.Fatalf("第 %d 条第 %d 个命令坐标偏差 %v（素材 %v，本文件平移后 %v）",
						i, k, math.Abs(float64(c[0]-c[1])), c[0], c[1])
				}
			}
		}
	}
}
