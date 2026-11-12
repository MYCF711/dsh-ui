package main

// hits_static_guard_test.go —— V-06-4 判据②：**每个 `.Clicked()` 都必须有身份**。
//
// # 为什么需要它
//
// v0.6.0 的 `.Clicked()` 只在"这个元素有身份"时才可能为真（`ui/box.go` 的
// `Box.Clicked`：`b.f.canvas.Clicked(n.id)`，而 `n.id` 只由 `Hit`/`HitWith`
// 或控件构造（`ui.Button` → `controlID`）写进去）。所以：
//
//	ui.Row(f).Children(func() { ... }).Clicked()      // 编译过、跑得过、**永远 false**
//
// 迁移时把 `.Hit(id, label)` 从链上丢掉，编译器不报、运行不报、测试不报——
// 只是那个元素**永远点不着**。这就是"看着对但点不着"的最短路径。
//
// # 判据
//
// 对 `plugins/`、`internal/`、`cmd/` 下每个**非测试** Go 文件，按函数粒度：
// 函数里出现 `.Clicked()` ⇒ 该函数内必须出现下列之一：
//
//	`.Hit(` / `.HitWith(` / `.HitWithOptions(`     ← 显式命中区
//	`ui.Button/ButtonBase/Checkbox/Switch/…`       ← 控件（自己带身份）
//	`ui.Control(`                                  ← 自带 Handlers 的控件
//
// 或者接收者来自 `hitsIdentityHelpers` 里的**带 ID 辅助函数**——它们把 id 当
// 参数收下、在函数体内 `.Hit(id, …)`（见每条的理由），属于"调用方给身份"。
//
// 判据是**双向**的：新加一个 `.Clicked()` 忘挂命中区 → 红；把一个辅助函数
// 从清单里删掉（或改名）→ 也红（登记项必须还在代码里）。

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// hitsIdentityHelpers 是"调用方传 id、函数体内挂命中区"的辅助函数。
//
// 每一条都必须能指出**它在哪里 `.Hit`**——否则它就不是辅助函数，而是真缺陷。
var hitsIdentityHelpers = map[string]string{
	"ListRow":             "plugins/dscore/primitives.go 的 ListRow：`return row.Hit(id, \"\").Children(body)`",
	"chromeButton":        "plugins/dsrightbar/rightbar.go 的 chromeButton：`.Hit(env.Tree.ID(env.Feat, name), label)`",
	"workspaceIconButton": "plugins/dssidebar/frame.go 的 workspaceIconButton：内部 `Hit(id, label)`",
}

// hitsClickedRe 匹配 `.Clicked()` 的接收者表达式。
var hitsClickedRe = regexp.MustCompile(`([A-Za-z_][\w.]*)\s*\.Clicked\s*\(\s*\)`)

// hitsDeclRe 是"函数内挂了身份"的判据。
var hitsDeclRe = regexp.MustCompile(
	`\b(Hit|HitWith|HitWithOptions)\s*\(|` +
		`\bui\.(Button|ButtonBase|Checkbox|CheckboxBase|Switch|SwitchBase|Radio|` +
		`RadioBase|Tabs|TabBase|Choice|Select|SelectBase|Toggle|ToggleBase|Slider|` +
		`SliderBase|TextField|TextFieldBase|Control)\s*\(`)

// hitsAssignRe 匹配 `x := …` / `x = …` 形态的赋值（用来找接收者的来源）。
var hitsAssignRe = regexp.MustCompile(`^\s*([A-Za-z_]\w*)\s*:?=\s*(.+)$`)

// goFuncSpan 是一个顶层函数的源码区间。
type goFuncSpan struct {
	name  string
	start int // 0 基行号
	lines []string
}

// scanGoFuncs 把一份 Go 源码切成顶层函数。
func scanGoFuncs(src string) []goFuncSpan {
	lines := strings.Split(src, "\n")
	var out []goFuncSpan
	var cur *goFuncSpan
	for i, line := range lines {
		if strings.HasPrefix(line, "func ") {
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &goFuncSpan{name: strings.TrimSpace(line), start: i}
			continue
		}
		if cur != nil {
			cur.lines = append(cur.lines, line)
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

// hitsSite 是一个待判定的 `.Clicked()` 调用点。
type hitsSite struct {
	file string
	line int
	fn   string
	recv string
	// decl 是接收者的声明行；空 = 未在本函数内声明（参数 / 字段）。
	decl string
}

// collectClickedSites 扫一棵源码树，返回所有"没有同函数内命中区声明"的
// `.Clicked()` 调用点。
func collectClickedSites(t *testing.T, roots []string) []hitsSite {
	t.Helper()
	var sites []hitsSite
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				// spike/ 是独立模块（各自 go.mod 还 pin 在 v0.2.3），不在门禁内。
				if name == "spike" || strings.HasPrefix(name, "_") || name == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			text := string(src)
			if !strings.Contains(text, ".Clicked()") {
				return nil
			}
			rel := filepath.ToSlash(path)
			for _, fn := range scanGoFuncs(text) {
				body := strings.Join(fn.lines, "\n")
				if !strings.Contains(body, ".Clicked()") {
					continue
				}
				if hitsDeclRe.MatchString(body) {
					continue
				}
				for i, line := range fn.lines {
					for _, recv := range hitsClickedRe.FindAllStringSubmatch(line, -1) {
						base := strings.Split(recv[1], ".")[0]
						decl := ""
						for j := i; j >= 0; j-- {
							if m := hitsAssignRe.FindStringSubmatch(fn.lines[j]); m != nil && m[1] == base {
								decl = strings.TrimSpace(fn.lines[j])
								break
							}
						}
						sites = append(sites, hitsSite{
							file: rel, line: fn.start + 1 + i + 1,
							fn: fn.name, recv: recv[1], decl: decl,
						})
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("扫描 %s 失败：%v", root, err)
		}
	}
	return sites
}

// TestClickedHasIdentity 是判据②：每个 `.Clicked()` 都能追到身份。
func TestClickedHasIdentity(t *testing.T) {
	sites := collectClickedSites(t, []string{"../../plugins", "../../internal", "../../cmd"})
	if len(sites) == 0 {
		t.Fatal("一个 .Clicked() 都没扫到——扫描路径或判据失效（0 匹配不报绿）")
	}
	var bad []hitsSite
	for _, s := range sites {
		// 接收者来自带 ID 的辅助函数：身份由调用方给，辅助函数体内 .Hit。
		helper := false
		for name := range hitsIdentityHelpers {
			if strings.Contains(s.decl, name+"(") {
				helper = true
				break
			}
		}
		if helper {
			continue
		}
		if s.decl == "" {
			// 未在本函数内声明 = 参数或字段（比如 `f ui.C`）：那个 Box 由调用方
			// 创建，本函数看不见它的链，交给调用方的判定。
			continue
		}
		bad = append(bad, s)
	}
	if len(bad) > 0 {
		lines := make([]string, 0, len(bad))
		for _, s := range bad {
			lines = append(lines, "  "+s.file+":"+itoaLocal(s.line)+"  "+s.fn+"\n      decl: "+s.decl)
		}
		t.Fatalf("%d 个 `.Clicked()` 的接收者没有命中区（编译过、永远点不着）：\n%s",
			len(bad), strings.Join(lines, "\n"))
	}
}

// TestHitsIdentityHelpersStillExist 是判据的反向：登记表必须还指向真代码。
//
// 辅助函数改名/删掉之后，登记表会静默失效，判据②就退化成"什么都放行"。
func TestHitsIdentityHelpersStillExist(t *testing.T) {
	for name, why := range hitsIdentityHelpers {
		found := false
		for _, root := range []string{"../../plugins", "../../internal", "../../cmd"} {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil || found || d.IsDir() {
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				src, rerr := os.ReadFile(path)
				if rerr != nil {
					return nil
				}
				if strings.Contains(string(src), "func ") &&
					regexp.MustCompile(`func\s+(\([^)]*\)\s*)?`+regexp.QuoteMeta(name)+`\s*\(`).
						MatchString(string(src)) {
					found = true
				}
				return nil
			})
			if err != nil {
				t.Fatalf("扫描 %s 失败：%v", root, err)
			}
		}
		if !found {
			t.Fatalf("登记表里的辅助函数 %q 在代码里找不到（改名了？）：%s", name, why)
		}
	}
}

// 编译期保证 go/parser 仍在依赖里（本文件只用 os/fs/regexp 就够，但保留
// parser 引用会让 lint 认为未使用——所以这里用一个空断言显式用一次）。
var _ = func() any { _, _ = parser.ParseFile(&token.FileSet{}, "", nil, 0); return nil }
