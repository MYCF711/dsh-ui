package dsshortcuts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 这个文件是"键位表会过期"的守卫。
//
// # 为什么需要它
//
// 这张表是手写的，而加一个键位**从来不是一个人的活**：插件的 HandleKey 要改、
// 壳的键名表（`cmd/app` 的 parseKeys）要改、这张表也要改。前两处漏改会**立刻坏**
// （按不出来），只有这张表漏改是**静默**的——功能是好的，只是没人知道有这个键。
//
// 这个仓已经踩过三次：F5/F6（终端）加的时候表没动、F7/F8（选项）加的时候表没动、
// F2（命令面板）从一开始就不在表里。等到有人问"我还能按什么键"，
// 这张表就是他唯一的答案，而它答错了一半。
//
// # 怎么扫（为什么用 AST 不用 grep）
//
// 第一版是正则扫源码文本，结果**自己把自己扫出来了**：本文件的注释里写着
// "扫 ui.KeyXxx"，那个 `ui.KeyXxx` 被当成了一个真实的键。
// 用 go/ast 解析、只看真实代码里的 `ui.KeyXxx` 选择表达式，注释天然不参与——
// 这类"扫源码"的守卫都该这么写，正则版本总会栽在注释和字符串上。
//
// # 只扫 plugins/
//
// `cmd/app` 里的 `ui.Key*` 大多是**注入用的**（`-keys` 的键名表），不是用户绑定。
// 多扫会把"注入器认识什么键"当成"用户能按什么键"。

// displayToKey 把表里那种给人看的写法还原成具名的键。
//
// 表的第一列是**给人看**的（`↑ / ↓`、`Esc / 停止键`），不是机器格式；
// 这里只做必要的翻译，不打算把它变成 DSL——那会让这张表变得没人愿意读。
var displayToKey = map[string]string{
	"↑": "KeyUp", "↓": "KeyDown", "→": "KeyRight", "←": "KeyLeft",
	"Enter": "KeyEnter", "Esc": "KeyEscape", "Tab": "KeyTab",
	"Home": "KeyHome", "End": "KeyEnd",
	"Space": "KeySpace", "Backspace": "KeyBackspace", "Delete": "KeyDelete",
	"PageUp": "KeyPageUp", "PageDown": "KeyPageDown",
	"F1": "KeyF1", "F2": "KeyF2", "F3": "KeyF3", "F4": "KeyF4",
	"F5": "KeyF5", "F6": "KeyF6", "F7": "KeyF7", "F8": "KeyF8",
	"F9": "KeyF9", "F10": "KeyF10", "F11": "KeyF11", "F12": "KeyF12",
}

// keyLabel 把 "KeyF7" 说成人话（红的提示里用）。
func keyLabel(name string) string {
	for disp, key := range displayToKey {
		if key == name {
			return disp
		}
	}
	return name
}

// documentedKeys 把表里所有行摊平成"这份表提到了哪些键"。
func documentedKeys() map[string]bool {
	out := map[string]bool{}
	for _, b := range bindings {
		// 分隔符按表里实际用过的写：`/`、`,`、`、`、空白。
		// 「停止键」这类描述性词翻译不出来，自然被丢掉——这正是想要的。
		for _, tok := range strings.FieldsFunc(b.Key, func(r rune) bool {
			return r == '/' || r == ',' || r == '、' || r == ' ' || r == '\u3000'
		}) {
			if key, ok := displayToKey[tok]; ok {
				out[key] = true
			}
		}
	}
	return out
}

// discoveredKeys 用 AST 扫 plugins/ 下所有非测试 Go 文件，收集真实的 `ui.KeyXxx`。
// 返回 键名 → 出现过的文件（去重、排好序）。
func discoveredKeys(t *testing.T) map[string][]string {
	t.Helper()
	root := filepath.Join("..", "..", "plugins")
	seen := map[string]map[string]bool{}
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		// 不传 parser.ParseComments：注释不参与 AST，扫不到也就不算数。
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, filepath.Join("..", "..")+string(filepath.Separator)))
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			// v0.6.0：键盘常量从 ui 搬到了 gfx（V-06-1 的 ui→gfx 拆分），
			// 两个包名都算数——不然这条守卫会把全部键报成"没人处理"。
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || (pkg.Name != "ui" && pkg.Name != "gfx") {
				return true
			}
			key := sel.Sel.Name
			if !strings.HasPrefix(key, "Key") || key == "KeyEvent" {
				return true
			}
			if seen[key] == nil {
				seen[key] = map[string]bool{}
			}
			seen[key][rel] = true
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("扫 plugins/ 失败：%v", err)
	}

	out := map[string][]string{}
	for key, files := range seen {
		var list []string
		for f := range files {
			list = append(list, f)
		}
		sort.Strings(list)
		out[key] = list
	}
	return out
}

// TestEveryHandledKeyIsDocumented 是这条守卫本身。
func TestEveryHandledKeyIsDocumented(t *testing.T) {
	documented := documentedKeys()
	found := discoveredKeys(t)

	var missing []string
	for key, files := range found {
		if documented[key] {
			continue
		}
		missing = append(missing, keyLabel(key)+"（"+key+"）出现于 "+strings.Join(files, "、"))
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Fatalf("这些键在插件里被 HandleKey 吃掉了，但 F1 键位表里没有：\n  %s\n"+
			"要么把它加进 bindings，要么在 displayToKey 里补一条翻译——"+
			"但不要为了让测试变绿而删掉真实绑定。", strings.Join(missing, "\n  "))
	}
}

// TestBindingsCoverRealKeys 是反方向的：表里写的键必须**真的有人在处理**。
//
// 少了这条，守卫只防"漏记"，不防"记了一条早就删掉的绑定"——
// 那种表更糟：用户照着按，什么都没发生，然后开始怀疑自己的键盘。
func TestBindingsCoverRealKeys(t *testing.T) {
	found := discoveredKeys(t)
	for key := range documentedKeys() {
		if len(found[key]) > 0 {
			continue
		}
		t.Errorf("表里有 %s（%s）这一行，但 plugins/ 下没有任何文件在处理它——"+
			"要么补上实现，要么把这一行删掉。", keyLabel(key), key)
	}
}
