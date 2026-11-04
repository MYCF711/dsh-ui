package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
)

// 这个文件是"插件用了某个 Action、壳却没接线"的守卫。
//
// # 为什么要它
//
// `pluginapi.Actions` 是壳给插件的唯一动作面。插件按 `Act.X != nil` 判断
// "这个能力有没有"，于是**没接线的动作等于功能不存在**——而且没有任何报错：
// 按钮不画、命令不出现、点了没反应。用户看到的是"这个功能没做"。
//
// 这个仓真的踩过：`OpenFile` 写好了、插件用了，但壳没接线，
// 结果是"文档预览里没有打开按钮"，查了三轮才确认不是渲染问题。
//
// 判据：**插件里引用到的每个 Actions 字段，壳的 `pluginapi.Actions{…}` 字面量里都要有。**
// 反方向不查——壳接线了但没人用是允许的（`SetDraft` 就是这种，文档里标了"空转接口"）。

// actionFields 是 `pluginapi.Actions` 的全部字段名。
func actionFields() map[string]bool {
	out := map[string]bool{}
	rt := reflect.TypeOf(pubapi.Actions{})
	for i := 0; i < rt.NumField(); i++ {
		out[rt.Field(i).Name] = true
	}
	return out
}

// actionReceiverNames 是可能持有 `pluginapi.Actions` 的标识符。
//
// 这份名单窄得刻意：多写一个名字就多一批假阳性，而假阳性会让这个守卫被关掉。
// 现在全仓只有这三种写法（`Act` 是 Env 的字段、`lastAct` 是插件缓存的上一帧、
// `a` 是 dscommands 里 `a := p.act` 的短名）。
var actionReceiverNames = map[string]bool{"Act": true, "act": true, "lastAct": true, "a": true}

// isActionReceiver 判断 `X.Field` 里的 X 是不是一个 Actions 值。
//
// ⚠️ 这里第一版写错过，而且**测试照样绿**：只认 `*ast.Ident`（`lastAct.OpenFile`、
// `a.OpenFile`），漏掉了仓里最常见的那种 `env.Act.OpenFile` —— 它的 X 是
// **另一个 SelectorExpr**（`env.Act`），不是一个 Ident。
//
// 漏掉之后这个守卫变成"只扫少数写法"，而它照样报"全部已接线"。
// 这正是本仓反复吃亏的"0 匹配 ≠ 绿"：守卫自己也会这样。
//
// 所以现在两种形状都认：
//
//	Ident      : lastAct.OpenFile / a.OpenFile / act.OpenFile
//	Selector   : env.Act.OpenFile / p.act.OpenFile（Sel 是 Act/act 即可）
func isActionReceiver(x ast.Expr) bool {
	switch v := x.(type) {
	case *ast.Ident:
		return actionReceiverNames[v.Name]
	case *ast.SelectorExpr:
		// `X.Act` / `X.act`：字段名对就认，不再往下追 X 的类型——
		// 追类型要给整棵树做类型检查，为一条判据不值得。
		return v.Sel.Name == "Act" || v.Sel.Name == "act" || v.Sel.Name == "lastAct"
	case *ast.ParenExpr:
		return isActionReceiver(v.X)
	}
	return false
}

// scanUsedActions 扫 plugins/ 下所有非测试 Go 文件，收集被引用的 Actions 字段。
func scanUsedActions(t *testing.T, fields map[string]bool) map[string][]string {
	t.Helper()
	root := filepath.Join("..", "..", "plugins")
	used := map[string]map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, filepath.Join("..", "..")+string(filepath.Separator)))
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !fields[sel.Sel.Name] {
				return true
			}
			if !isActionReceiver(sel.X) {
				return true
			}
			if used[sel.Sel.Name] == nil {
				used[sel.Sel.Name] = map[string]bool{}
			}
			used[sel.Sel.Name][rel] = true
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("扫 plugins/ 失败：%v", err)
	}
	out := map[string][]string{}
	for field, files := range used {
		var list []string
		for f := range files {
			list = append(list, f)
		}
		sort.Strings(list)
		out[field] = list
	}
	return out
}

// scanWiredActions 从壳的 `pluginapi.Actions{…}` 字面量里取出已经接线的字段。
func scanWiredActions(t *testing.T) map[string]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 main.go 失败：%v", err)
	}
	wired := map[string]bool{}
	found := 0
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Actions" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "pluginapi" {
			return true
		}
		found++
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok {
				wired[key.Name] = true
			}
		}
		return true
	})
	if found == 0 {
		t.Fatal("在 main.go 里找不到 pluginapi.Actions{…} 字面量——壳的接线方式变了，这个守卫要跟着改")
	}
	return wired
}

// TestEveryUsedActionIsWired 是守卫本身。
func TestEveryUsedActionIsWired(t *testing.T) {
	fields := actionFields()
	if len(fields) < 25 {
		t.Fatalf("只反射出 %d 个 Actions 字段，结构体是不是改了？", len(fields))
	}
	used := scanUsedActions(t, fields)
	wired := scanWiredActions(t)

	var missing []string
	for field, files := range used {
		if wired[field] {
			continue
		}
		missing = append(missing, field+"（"+strings.Join(files, "、")+"）")
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("这些 Action 插件在用、壳没接线——插件会因为 nil 而**静默不画那个入口**：\n  %s\n"+
			"在 cmd/app 的 pluginapi.Actions{…} 里接上，或者把插件里那段删掉。",
			strings.Join(missing, "\n  "))
	}
	if len(used) < 15 {
		t.Fatalf("只扫到 %d 个被使用的 Action——扫描是不是失效了（全仓实际有 25+ 个）", len(used))
	}
	t.Logf("插件使用的 Action %d 个，全部已接线；壳共接线 %d 个", len(used), len(wired))
}

// TestActionReceiverShapesCoverBothForms 是给上面那个**已踩过的坑**留的钉子。
//
// 第一版 `isActionReceiver` 只认 `*ast.Ident`，于是 `env.Act.OpenFile`
// （仓里最常见的写法）一个都扫不到，而测试**照样报绿**、还写着"全部已接线"。
//
// 这条测试把两种形状都钉住：少认一种就红，不必等到某天有人手工反证。
func TestActionReceiverShapesCoverBothForms(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		// 仓里最常见的：Env 的字段
		{"env.Act.OpenFile", true},
		{"env.Act.Goal", true},
		// 插件缓存的上一帧
		{"p.lastAct.Approve", true},
		// dscommands 里的短名 `a := p.act`
		{"a.SetPermission", true},
		{"p.act.Notify", true},
		// 不该认的
		{"env.Data.OpenFile", false},
		{"m.theme.Link", false},
		{"x.y.z", false},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			sel, ok := parseSelector(t, tc.expr)
			if !ok {
				t.Fatalf("%q 不是选择表达式，样本本身有问题", tc.expr)
			}
			if got := isActionReceiver(sel.X); got != tc.want {
				t.Fatalf("isActionReceiver(%q 的接收者) = %v，want %v"+
					"（少认一种形状，守卫就会静默变窄，然后照样报绿）", tc.expr, got, tc.want)
			}
		})
	}
}

func parseSelector(t *testing.T, expr string) (*ast.SelectorExpr, bool) {
	t.Helper()
	e, err := parser.ParseExpr(expr)
	if err != nil {
		t.Fatalf("解析 %q 失败：%v", expr, err)
	}
	sel, ok := e.(*ast.SelectorExpr)
	return sel, ok
}
