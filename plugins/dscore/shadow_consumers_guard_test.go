package dscore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 这个文件是「阴影不许退回 viva 预设」的守卫。
//
// # 为什么需要它
//
// 官方把阴影写成 7 个显式令牌（`--dsw-shadow-lv1/lv1-blur/lv2/lv3` 与
// `--dsw-elevation-panel/prominent/soft`，见 shadow.go 顶部的取值表）。viva 的
// `gfx.ShadowXS..ShadowLG` 是 **Tailwind 阶梯**，与官方档位没有一一对应：
// `ShadowMD` 是 `0 4px 6px -1px .10 + 0 2px 4px -2px .10`，官方 lv2 是
// `0 4px 12px .02 + 0 2px 8px .04`——位移、模糊、**负扩展**、透明度全不同，
// 而且 viva 预设**没有描边层**，`lv1-blur` / `lv3` 根本没有对应档
// （viva@v0.6.4/gfx/canvas.go:117-129）。
//
// 于是「顺手写一行 `.Shadow(gfx.ShadowMD)`」的画面上就是另一张浮层，而且是**静默**
// 的：编译过、测试过、看起来也像有阴影。20 多个插件各抄一份的结果，是只有抄得
// 最认真的那几个是对的。`task-80` 把这 15 个包全部换成了令牌，这条守卫保证它
// 不会在下一轮被改回预设。
//
// # 怎么扫
//
// 用 go/ast 而不是正则：注释里写着 `Shadow(gfx.ShadowMD)` 的地方多得是
// （本单的改动说明就写了不少），正则版本会把自己扫出来——`dsshortcuts` 的键位
// 守卫已经踩过这个坑，这里照它的写法（不传 parser.ParseComments，注释不进 AST）。

// convertedPlugins 是本轮（#150 + task-80）已改成官方令牌的消费方。
//
// 只扫这些目录：不在名单里的包还有别处的既有用法，那条线不归本单，扫进来会
// 变成"改别人的代码才绿"。新包要加进这份名单时，先把它的阴影换成
// `t.Shadows.*` + `PaintShadow`/`PaintElevation`。
var convertedPlugins = []string{
	"dsagentteam", "dscomposer", "dsconversation", "dscordis", "dsjobs",
	"dssettings", "dsshortcuts", "dssidebar", "dsstatusbar", "dsterminal",
	"dsattachment", "dscontextmenu", "dsdirpicker", "dsgoal", "dsquestions",
}

// vivaShadowPresets 是 viva 的 Tailwind 阴影档名字（`gfx.ShadowLevel` 常量）。
// `ShadowStyle` / `ShadowNone` 不在里面：前者是"一层 box-shadow 的四个参数"
// （画法要用），后者是"不画"。
var vivaShadowPresets = map[string]bool{
	"ShadowXS": true,
	"ShadowSM": true,
	"ShadowMD": true,
	"ShadowLG": true,
}

// TestConvertedPluginsUseOfficialShadowTokens 扫这 15 个包的非测试源码：
// ① 不许再出现 `gfx.ShadowXS/SM/MD/LG`（预设档）；
// ② 必须能扫到 `dscore.PaintShadow` / `dscore.PaintElevation`（证明令牌真的接上了，
// 而不是把调用整段删掉——删掉同样是"画面上的阴影消失"，一样是回退）。
func TestConvertedPluginsUseOfficialShadowTokens(t *testing.T) {
	root := filepath.Join("..", "..", "plugins")
	fset := token.NewFileSet()

	for _, name := range convertedPlugins {
		dir := filepath.Join(root, name)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("守卫名单里的包不存在：%s（改名单要同步改代码）", filepath.ToSlash(dir))
		}

		var presets []string
		paints := 0
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") ||
				strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			// 不传 parser.ParseComments：注释不参与 AST，扫不到也就不算数。
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(path)
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "gfx" {
					return true
				}
				if vivaShadowPresets[sel.Sel.Name] {
					presets = append(presets, rel+": gfx."+sel.Sel.Name)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("扫 %s 失败：%v", name, err)
		}

		// ② 令牌真的接上了：数 `dscore.PaintShadow` / `dscore.PaintElevation`
		// （含 Overlay 变体，`strings.HasPrefix` 一并算）。
		paints = countShadowTokenCalls(t, fset, dir)
		if paints == 0 {
			t.Errorf("%s: 一个 `dscore.PaintShadow*` / `dscore.PaintElevation*` 调用都没有。"+
				"阴影是官方令牌，删掉调用等于这张浮层没有阴影", name)
		}
		if len(presets) > 0 {
			for _, hit := range presets {
				t.Errorf("%s: 还在用 viva 的 Tailwind 阴影预设 —— %s。"+
					"官方档位是 `t.Shadows.Lv1/Lv1Blur/Lv2/Lv3/Panel/Prominent/Soft`"+
					"（取值与出处见 plugins/dscore/shadow.go），用 `dscore.PaintShadow` 或 "+
					"`dscore.PaintElevation` 画；带 `.Clip()` 的元素要在 AfterDraw 里用 Overlay 变体",
					name, hit)
			}
		}
	}
}

// countShadowTokenCalls 数目录里 `dscore.PaintShadow*` / `dscore.PaintElevation*` 的调用点。
func countShadowTokenCalls(t *testing.T, fset *token.FileSet, dir string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") ||
			strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "dscore" {
				return true
			}
			if strings.HasPrefix(sel.Sel.Name, "PaintShadow") ||
				strings.HasPrefix(sel.Sel.Name, "PaintElevation") {
				count++
				return true
			}
			// 共享画笔同一个道理：它内部就是 PaintElevationOverlay，档位由
			// `TestElevationDecisionTablePerSite` 的实现文件那一行钉住。
			if _, shared := sharedShadowPainters[sel.Sel.Name]; shared {
				count++
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("扫 %s 的令牌调用失败：%v", dir, err)
	}
	return count
}
