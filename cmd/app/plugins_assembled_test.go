package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 这个文件是"写了插件但没装配"的守卫。
//
// # 为什么要它
//
// `plugins/<名字>/` 目录本身不会做任何事——它要**被 `cmd/app` import**（哪怕只是
// `_` 空导入，靠包里的 `init()` 去 `pluginapi.Register`）才会出现在装配里。
// 目录建了、代码写了、测试绿了，**唯独没进 import 表**，结果就是：界面里没有它，
// 而且没有任何报错。
//
// 这个仓真踩过：`dsopen` 的按钮查了三轮才确认"插件没装配"——因为它当时
// `grep -c plugins/dsopen cmd/app/main.go` 是 0，而工作树里那个 .go 文件好端端地躺着。
//
// 判据：`plugins/` 下每个目录都要在 `cmd/app` 的 import 里出现；反过来，
// 每个 import 也要在磁盘上真的存在（后者 build 会抓，但一起写着更省事）。

const pluginsPrefix = "github.com/MYCF711/dsh-ui/plugins/"

// importedPlugins 从本包的 Go 文件里收集被 import 的插件目录名。
func importedPlugins(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读 cmd/app 失败：%v", err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if perr != nil {
			t.Fatalf("解析 %s 失败：%v", name, perr)
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(path, pluginsPrefix) {
				continue
			}
			dir := strings.TrimPrefix(path, pluginsPrefix)
			// 只认"一整层目录名"，`plugins/dsconversation/wpc3` 这种子包不算
			if strings.Contains(dir, "/") {
				continue
			}
			out[dir] = name
		}
	}
	return out
}

// diskPlugins 列出 plugins/ 下的所有目录。
func diskPlugins(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("..", "..", "plugins"))
	if err != nil {
		t.Fatalf("读 plugins/ 失败：%v", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// TestEveryPluginDirIsImported 是守卫本身。
func TestEveryPluginDirIsImported(t *testing.T) {
	dirs := diskPlugins(t)
	if len(dirs) < 20 {
		t.Fatalf("plugins/ 下只找到 %d 个目录——这个守卫没有意义了？", len(dirs))
	}
	imported := importedPlugins(t)
	if len(imported) < 20 {
		t.Fatalf("cmd/app 里只解析到 %d 个插件 import——解析失效了（实际 20+）", len(imported))
	}

	var missing []string
	for _, d := range dirs {
		if _, ok := imported[d]; !ok {
			missing = append(missing, d)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("这些插件目录存在、但 `cmd/app` **没有 import**——它们在界面上等于不存在，"+
			"而且不报错：\n  %v\n"+
			"在 cmd/app/main.go 的 import 块里加一行（插件自己 init() 注册，`_` 空导入即可）。", missing)
	}

	// 反向：import 了但目录没了（build 本来也会红，这里给一句人话）
	var orphan []string
	for d, file := range imported {
		if _, err := os.Stat(filepath.Join("..", "..", "plugins", d)); err != nil {
			orphan = append(orphan, d+"（在 "+file+"）")
		}
	}
	sort.Strings(orphan)
	if len(orphan) > 0 {
		t.Fatalf("这些 import 指向不存在的插件目录：%v", orphan)
	}
	t.Logf("插件目录 %d 个，全部已 import", len(dirs))
}
