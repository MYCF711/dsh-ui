package dsassets

import (
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

// TestUnsupportedAssetsSayWhy：画不出来的素材必须写明原因，而且真的画不出来。
//
// 这条防的是"标了不可画、其实能画"和"能画、却悄悄标了不可画"两种漂移：
// 前者说明原因写错了，后者会让某个图标白白退回旧实现。
func TestUnsupportedAssetsSayWhy(t *testing.T) {
	for _, a := range All() {
		_, built := Build(a, gfx.RGB(255, 255, 255))
		if a.Buildable() == built {
			continue // 能画且画出来了 / 不能画且没画，都对
		}
		t.Errorf("%s: Buildable()=%v 但 Build()=%v（声明与实现对不上）", a.ID, a.Buildable(), built)
	}
	unsupported := 0
	for _, a := range All() {
		if a.Buildable() {
			continue
		}
		unsupported++
		if a.Unsupported == "" {
			t.Errorf("%s 不可构建却没写原因", a.ID)
		}
	}
	if unsupported == 0 {
		t.Fatal("一条不可构建的素材都没有：官方确实有 <rect>/<circle> 素材，这个数字不该是 0")
	}
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
		{id: "send-outline", stroke: 1, commands: 2, layers: 1, layerCmds: 10},
		{id: "paperclip", stroke: 1, commands: 17},
		// 命令入口加号：官方 Medium 档，描边 1.3，两笔各 M+L = 4 条命令。
		{id: "composer-add", stroke: 1.3, commands: 4},
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
