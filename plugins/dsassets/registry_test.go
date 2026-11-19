package dsassets

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// TestEveryAssetCarriesProvenance：每条素材都必须说得出"从哪来"。
//
// 空字段是这类"集中存放"的典型腐坏方式：先把素材抄进来，出处慢慢就没人填了，
// 过半年谁也证明不了它还是官方的。所以这里对每条素材逐字段要求非空，
// 并且 sha256 必须是 64 位十六进制（防止写个占位符混过去）。
func TestEveryAssetCarriesProvenance(t *testing.T) {
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	seen := map[string]bool{}
	for _, a := range All() {
		if a.ID == "" {
			t.Error("有素材没写 ID")
			continue
		}
		if seen[a.ID] {
			t.Errorf("素材 ID 重复：%s", a.ID)
		}
		seen[a.ID] = true
		if a.Title == "" {
			t.Errorf("%s 没写中文名", a.ID)
		}
		if a.Source.Package == "" || a.Source.File == "" || a.Source.Symbol == "" {
			t.Errorf("%s 出处不全：%+v", a.ID, a.Source)
		}
		if !hex64.MatchString(a.Source.SHA256) {
			t.Errorf("%s 的文件 sha256 不合法：%q", a.ID, a.Source.SHA256)
		}
		if a.ViewBox <= 0 || a.Size <= 0 {
			t.Errorf("%s viewBox/size 不合理：%v/%v", a.ID, a.ViewBox, a.Size)
		}
		if len(a.Elements) == 0 {
			t.Errorf("%s 没有任何图元", a.ID)
		}
	}
	if len(seen) != len(All()) {
		t.Fatalf("All() 里有重复 ID：%d 条 / %d 个唯一 ID", len(All()), len(seen))
	}
}

// TestEveryElementKindBuilds：声明与实现必须对得上，而且**官方每种图元都真能画**。
//
// 这条以前要求"至少有一条不可构建"——当时的理由是官方确实有 `<rect rx>` / `<circle>`
// 素材、而转换器不会画弧。2026-11-19 弧→三次贝塞尔落地后那个前提不成立了：15 条
// 素材全部可画。所以判据换成**更强**的那一句，而不是把断言删掉：
//
//   - 声明与 Build 必须一致（不许"标了不可画其实能画"，也不许反过来）；
//   - 不可构建的必须写明原因；
//   - 登记表里出现的**每一种图元**都必须真的换算成命令——这条是"弧能画了"的正面证据。
func TestEveryElementKindBuilds(t *testing.T) {
	white := gfx.RGB(255, 255, 255)
	kinds := map[ElementKind]bool{}
	for _, a := range All() {
		_, built := Build(a, white)
		if a.Buildable() == built {
			// 能画且画出来了 / 不能画且没画，都对。
		} else {
			t.Errorf("%s: Buildable()=%v 但 Build()=%v（声明与实现对不上）", a.ID, a.Buildable(), built)
		}
		if !a.Buildable() && a.Unsupported == "" {
			t.Errorf("%s 不可构建却没写原因", a.ID)
		}
		for _, el := range a.Elements {
			kinds[el.Kind] = true
		}
	}
	// 官方这套素材三种图元都用到了；少了任何一种，下面的逐种判据就失去意义。
	for _, k := range []ElementKind{PathElement, RectElement, CircleElement} {
		if !kinds[k] {
			t.Errorf("素材表里没有 %v 图元：官方确实有 path/rect/circle 三种，这个覆盖不该是空的", k)
		}
	}
	// 每一种图元都必须能单独换算成非空命令——这是"弧→贝塞尔"真正落地的判据。
	samples := map[ElementKind]Element{
		PathElement:   {Kind: PathElement, D: composerSendPath},
		RectElement:   {Kind: RectElement, X: 3, Y: 3, Width: 10, Height: 10, RX: 3},
		CircleElement: {Kind: CircleElement, CX: 12.4502, CY: 3.3079, R: 1.56962},
	}
	for k, sample := range samples {
		cmds, ok := elementCommands(sample)
		if !ok || len(cmds) == 0 {
			t.Errorf("%v 图元换算失败（ok=%v，命令数=%d）", k, ok, len(cmds))
		}
	}
}

// TestOfficialNonPathAssetsBuildToOfficialGeometry 证明"非路径图元也能画，而且画的是官方那个"。
//
// 三枚素材（`composer-stop` / `copy` / `branch`）以前都被标 `Unsupported`，理由是
// `<rect rx>` / `<circle>` 的弧在 viva 的路径词汇里没有对应命令。这条测试是那句
// 结论**已经作废**的证据：它们现在都能 `Build`，而且换算出来的几何逐项等于官方
// 图元的属性（`testdata/official/*.jsx` 里那几个数）。
//
// 判据是**包围盒**：四分之一弧的两个控制点分别落在端点的切线上，不会超出圆弧自身的
// 包围盒，所以非路径图元的 `Bounds` 必须精确等于官方 x/y/width/height 或 cx±r/cy±r。
// 差一点点就说明圆心、半径或起止角算错了。
//
// 两层都查：**图元自己**（隔离出"这一枚圆/这个矩形"的几何）与**整枚素材**
// （证明它们与官方 path 拼在一起后仍落在 16 viewBox 内）。
func TestOfficialNonPathAssetsBuildToOfficialGeometry(t *testing.T) {
	const tol = 1e-4
	near := func(t *testing.T, id, what string, got, want float32) {
		t.Helper()
		if d := float32(math.Abs(float64(got - want))); d > tol {
			t.Errorf("%s %s = %v，want %v（差 %v）", id, what, got, want, d)
		}
	}

	// 整枚素材：描边宽度、命令数、包围盒。
	assets := []struct {
		id                     string
		wantStroke             float32
		wantCommands           int
		minX, minY, maxX, maxY float32
	}{
		// `<rect x=3 y=3 width=10 height=10 rx=3>` → [3,13]×[3,13]。
		{id: "composer-stop", wantStroke: 0, wantCommands: 10, minX: 3, minY: 3, maxX: 13, maxY: 13},
		// 描边 rect 10 条 + 填充路径（copyBackPath）14 条；包围盒由路径撑到 14.4792/1.53296。
		{id: "copy", wantStroke: 1, wantCommands: 10, minX: 1.52075, minY: 1.53296, maxX: 14.4792, maxY: 14.46693},
		// 两条支线（x 从 1.01221 起）+ 两枚 `<circle cx=12.4502 r=1.56962>`。
		{id: "branch", wantStroke: 1, wantCommands: 24, minX: 1.01221, minY: 1.73828, maxX: 14.01982, maxY: 14.26172},
	}
	for _, tc := range assets {
		t.Run(tc.id, func(t *testing.T) {
			a, ok := Lookup(tc.id)
			if !ok {
				t.Fatalf("找不到素材 %s", tc.id)
			}
			icon, built := Build(a, gfx.RGB(0, 0, 0))
			if !built {
				t.Fatalf("%s 应该能构建（弧→贝塞尔已经实现）", tc.id)
			}
			if icon.Stroke != tc.wantStroke {
				t.Errorf("%s 描边 = %v，want %v", tc.id, icon.Stroke, tc.wantStroke)
			}
			if len(icon.Commands) != tc.wantCommands {
				t.Errorf("%s 描边命令数 = %d，want %d", tc.id, len(icon.Commands), tc.wantCommands)
			}
			minX, minY, maxX, maxY, ok := Bounds(a)
			if !ok {
				t.Fatalf("%s 没有解析出任何坐标", tc.id)
			}
			near(t, tc.id, "minX", minX, tc.minX)
			near(t, tc.id, "minY", minY, tc.minY)
			near(t, tc.id, "maxX", maxX, tc.maxX)
			near(t, tc.id, "maxY", maxY, tc.maxY)
			t.Logf("%s 包围盒 = [%v %v %v %v]，描边命令 %d 条", tc.id, minX, minY, maxX, maxY, len(icon.Commands))
		})
	}

	// 非路径图元自己：把"这一枚圆/这个矩形"的几何单独钉住。
	elements := []struct {
		id                     string
		index                  int
		minX, minY, maxX, maxY float32
	}{
		// 官方 `<rect x=3 y=3 width=10 height=10 rx=3>`。
		{id: "composer-stop", index: 0, minX: 3, minY: 3, maxX: 13, maxY: 13},
		// 官方 `<rect x=1.52075 y=4.07373 width=10.3932 height=10.3932 rx=2>`。
		{id: "copy", index: 0, minX: 1.52075, minY: 4.07373, maxX: 11.91395, maxY: 14.46693},
		// 官方 `<circle cx=12.4502 cy=3.3079 r=1.56962>`。
		{id: "branch", index: 2, minX: 10.88058, minY: 1.73828, maxX: 14.01982, maxY: 4.87752},
		// 官方 `<circle cx=12.4502 cy=12.6921 r=1.56962>`。
		{id: "branch", index: 3, minX: 10.88058, minY: 11.12248, maxX: 14.01982, maxY: 14.26172},
	}
	for _, tc := range elements {
		a, ok := Lookup(tc.id)
		if !ok {
			t.Fatalf("找不到素材 %s", tc.id)
		}
		if tc.index >= len(a.Elements) {
			t.Fatalf("%s 只有 %d 个图元，取不到第 %d 个", tc.id, len(a.Elements), tc.index)
		}
		if a.Elements[tc.index].Kind == PathElement {
			t.Fatalf("%s 第 %d 个图元是 path，这条测试只查非路径图元", tc.id, tc.index)
		}
		cmds, ok := elementCommands(a.Elements[tc.index])
		if !ok {
			t.Fatalf("%s 第 %d 个图元换算失败", tc.id, tc.index)
		}
		minX, minY, maxX, maxY := commandBounds(cmds)
		near(t, tc.id, fmt.Sprintf("第 %d 个图元 minX", tc.index), minX, tc.minX)
		near(t, tc.id, fmt.Sprintf("第 %d 个图元 minY", tc.index), minY, tc.minY)
		near(t, tc.id, fmt.Sprintf("第 %d 个图元 maxX", tc.index), maxX, tc.maxX)
		near(t, tc.id, fmt.Sprintf("第 %d 个图元 maxY", tc.index), maxY, tc.maxY)
	}
}

// commandBounds 返回一组路径命令的端点/控制点包围盒（与 `Bounds` 同口径）。
func commandBounds(cmds []gfx.PathCommand) (minX, minY, maxX, maxY float32) {
	first := true
	add := func(x, y float32) {
		if first {
			minX, minY, maxX, maxY = x, y, x, y
			first = false
			return
		}
		minX = min32(minX, x)
		minY = min32(minY, y)
		maxX = max32(maxX, x)
		maxY = max32(maxY, y)
	}
	for _, c := range cmds {
		if c.Kind == gfx.PathCommandClose {
			continue
		}
		add(c.Point.X, c.Point.Y)
		if c.Kind == gfx.PathCommandCubic || c.Kind == gfx.PathCommandQuad {
			add(c.Control1.X, c.Control1.Y)
		}
		if c.Kind == gfx.PathCommandCubic {
			add(c.Control2.X, c.Control2.Y)
		}
	}
	return minX, minY, maxX, maxY
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// TestBuildShapes：逐条钉住 Build 出来的形状（描边/填充/线帽线接/命令数）。
//
// 这比"图标非空"强得多：它证明的是"哪几条命令、用填充还是描边、多粗"，
// 也就是替换后屏幕上那一枚图标到底是什么。
func TestBuildShapes(t *testing.T) {
	white := gfx.RGB(255, 255, 255)
	cases := []struct {
		id        string
		stroke    float32
		cap       gfx.StrokeCap
		join      gfx.StrokeJoin
		commands  int
		layers    int
		layerCmds int
	}{
		// 发送键：纯填充，走 Commands 而不是 Layers，禁用态的 alpha 才能整体降下来。
		{id: "composer-send", stroke: 0, commands: 16},
		// 停止键：官方 `<rect rx=3>` 纯填充 → M + 4L + 4C + Z = 10 条，同样走 Commands。
		// 这条以前是"画不出来"，现在是**路径也能画**（dscomposer 仍走 Viva 原生圆角矩形）。
		{id: "composer-stop", stroke: 0, commands: 10},
		{id: "send-outline", stroke: 1, commands: 2, layers: 1, layerCmds: 10},
		{id: "paperclip", stroke: 1, commands: 17},
		// 命令入口加号：官方 Medium 档，描边 1.3，两笔各 M+L = 4 条命令。
		{id: "composer-add", stroke: 1.3, commands: 4},
		// 复制：前面那张纸是 `<rect rx=2>`（描边 10 条），后面那张纸是填充 14 条。
		{id: "copy", stroke: 1, commands: 10, layers: 1, layerCmds: 14},
		// 分支：两条支线 12 条 + 两枚 `<circle r=1.56962>` 12 条，全走描边。
		{id: "branch", stroke: 1, commands: 24},
		// 反馈拇指：空心只有描边；实心有描边+填充两条同形路径。
		{id: "thumb-up-outline", stroke: 1, commands: 18},
		{id: "thumb-up-fill", stroke: 1, commands: 18, layers: 1, layerCmds: 18},
		{id: "thumb-down-outline", stroke: 1, commands: 18},
		{id: "thumb-down-fill", stroke: 1, commands: 18, layers: 1, layerCmds: 18},
		// 权限四档：盾牌那条写了 linejoin=round；自动审查那条还写了 linecap=round。
		{id: "permission-read-only", stroke: 1, join: gfx.JoinRound, commands: 13},
		{id: "permission-full-access", stroke: 1, join: gfx.JoinRound, commands: 13},
		{id: "permission-auto-review", stroke: 1, cap: gfx.CapRound, join: gfx.JoinRound, commands: 14},
		{id: "permission-workspace-write", stroke: 1, commands: 4, layers: 3, layerCmds: 41},
	}
	for _, tc := range cases {
		a, ok := Lookup(tc.id)
		if !ok {
			t.Fatalf("找不到素材 %s", tc.id)
		}
		icon, built := Build(a, white)
		if !built {
			t.Fatalf("%s 应该能构建", tc.id)
		}
		if icon.Stroke != tc.stroke {
			t.Errorf("%s 描边宽度 = %v，want %v", tc.id, icon.Stroke, tc.stroke)
		}
		if icon.Cap != tc.cap {
			t.Errorf("%s 线帽 = %v，want %v", tc.id, icon.Cap, tc.cap)
		}
		if icon.Join != tc.join {
			t.Errorf("%s 线接 = %v，want %v", tc.id, icon.Join, tc.join)
		}
		if len(icon.Commands) != tc.commands {
			t.Errorf("%s 描边命令数 = %d，want %d", tc.id, len(icon.Commands), tc.commands)
		}
		if len(icon.Layers) != tc.layers {
			t.Errorf("%s 填充层 = %d，want %d", tc.id, len(icon.Layers), tc.layers)
		}
		total := 0
		for _, l := range icon.Layers {
			if l.Fill != white {
				t.Errorf("%s 填充层颜色没有透传", tc.id)
			}
			total += len(l.Commands)
		}
		if total != tc.layerCmds {
			t.Errorf("%s 填充命令数 = %d，want %d", tc.id, total, tc.layerCmds)
		}
	}
}

// TestDocumentedAssetsMatchRegistry：docs/official-assets.md 里必须出现每条素材的 ID。
//
// 文档是本包的"人读视图"：加了素材却不写文档、或者文档里写了一个已经删掉的素材，
// 都会让后来者按文档去调用一个不存在的东西。这条把两边钉在一起。
func TestDocumentedAssetsMatchRegistry(t *testing.T) {
	raw, err := os.ReadFile("../../docs/official-assets.md")
	if err != nil {
		t.Fatalf("读不到 docs/official-assets.md: %v", err)
	}
	doc := string(raw)
	for _, a := range All() {
		if !containsID(doc, a.ID) {
			t.Errorf("docs/official-assets.md 里没有 %s 这一行", a.ID)
		}
		// 每条 `d` 原文也必须出现在文档里：文档是给人看的"官方原文"，
		// 它和 paths.go 一旦漂移，读文档的人就会以为代码抄的是那一串。
		for i, el := range a.Elements {
			if el.Kind == PathElement && !strings.Contains(doc, el.D) {
				t.Errorf("%s 第 %d 条 path 的原文不在文档里（文档与 paths.go 漂移了）", a.ID, i)
			}
		}
	}
}

// containsID 在文档里找素材 ID（避免 `composer-send` 命中 `composer-sendfoo`）。
func containsID(doc, id string) bool {
	for i := 0; i+len(id) <= len(doc); i++ {
		if doc[i:i+len(id)] != id {
			continue
		}
		before := byte(' ')
		if i > 0 {
			before = doc[i-1]
		}
		after := byte(' ')
		if i+len(id) < len(doc) {
			after = doc[i+len(id)]
		}
		okBefore := !(before >= 'a' && before <= 'z' || before >= 'A' && before <= 'Z' || before == '-' || before >= '0' && before <= '9')
		okAfter := !(after >= 'a' && after <= 'z' || after >= 'A' && after <= 'Z' || after == '-' || after >= '0' && after <= '9')
		if okBefore && okAfter {
			return true
		}
	}
	return false
}
