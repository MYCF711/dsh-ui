package main

// 扫描：认定一个子目录「是不是一个界面插件」。
//
// 规则写死在这里，也写在 docs/plugin-api.md 的第 13 节。改规则 = 两边一起改。
//
//	1. 目录里有 go.mod（module 根包 = 该 module 的 import 路径）
//	2. 根包（或 plugin.json 里声明的那个包）import 了公开契约包
//	   github.com/MYCF711/dsh-ui/pluginapi
//
// 不满足的在扫描结果里带一条原因，打印出来但不阻断（一个插件父目录里
// 躺着 README、脚本、半成品很正常，报错会把工具变得没法用）。

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// pluginAPI 是唯一判据：没有它就不是界面插件。
const pluginAPI = "github.com/MYCF711/dsh-ui/pluginapi"

// pluginJSON 是插件目录里可选的声明文件：
//
//	{"id": "hello.counter", "module": "example.com/dsh-hello", "package": "hello"}
type pluginJSON struct {
	ID      string `json:"id"`
	Module  string `json:"module"`
	Package string `json:"package"`
}

// Plugin 是一条扫描出来的外部插件。
type Plugin struct {
	Dir      string // 插件目录（绝对路径）
	Module   string // import 路径（module 根包）
	Package  string // 根包包名（只用于报告）
	ID       string // plugin.json 里的 id；没有就是空（id 由插件 init 里 Register 决定）
	Manifest bool   // module 是不是 plugin.json 声明/覆盖的
	Warn     string // 不阻断的提醒（比如插件自己的 go.mod 没 require dsh-ui）
}

// skip 是一个被跳过的子目录及原因。
type skip struct {
	Name   string
	Reason string
	Dir    string
}

// scanRoot 扫一个插件父目录的**直接子目录**。
//
// 父目录不存在 = 错误（列出路径），绝不静默跳过：配置里写了就说明用户认为它在。
func scanRoot(root string) (found []Plugin, skipped []skip, err error) {
	st, statErr := os.Stat(root)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, nil, fmt.Errorf("插件目录不存在: %s", root)
		}
		return nil, nil, fmt.Errorf("插件目录不可用 %s: %w", root, statErr)
	}
	if !st.IsDir() {
		return nil, nil, fmt.Errorf("插件目录不是目录: %s", root)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		return nil, nil, fmt.Errorf("读插件目录 %s 失败: %w", root, readErr)
	}
	// os.ReadDir 已按文件名排序；这里再排一次是因为下面要按 module 去重，
	// 谁先谁后会影响报错信息里的顺序。
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		if !e.IsDir() {
			continue // 父目录里的散文件（README、脚本）不是插件
		}
		dir := filepath.Join(root, e.Name())
		p, reason, perr := scanPluginDir(dir)
		if perr != nil {
			return nil, nil, perr
		}
		if reason != "" {
			skipped = append(skipped, skip{Name: e.Name(), Reason: reason, Dir: dir})
			continue
		}
		found = append(found, p)
	}
	return found, skipped, nil
}

// scanPluginDir 判断一个子目录是不是插件目录。
// reason 非空表示「不是插件」，不是错误。
func scanPluginDir(dir string) (Plugin, string, error) {
	gomod := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(gomod); err != nil {
		if os.IsNotExist(err) {
			return Plugin{}, "没有 go.mod", nil
		}
		return Plugin{}, "", fmt.Errorf("检查 %s 失败: %w", gomod, err)
	}
	modPath, err := modulePath(gomod)
	if err != nil {
		return Plugin{}, "", err
	}

	var man pluginJSON
	manifestPath := filepath.Join(dir, "plugin.json")
	if raw, err := os.ReadFile(manifestPath); err == nil {
		if err := json.Unmarshal(raw, &man); err != nil {
			return Plugin{}, "", fmt.Errorf("解析 %s 失败: %w", manifestPath, err)
		}
	} else if !os.IsNotExist(err) {
		return Plugin{}, "", fmt.Errorf("读 %s 失败: %w", manifestPath, err)
	}

	// 根包的 .go 文件（跳过 _test.go：测试包可能叫 foo_test，会把包名判断带偏）
	files, err := goFiles(dir)
	if err != nil {
		return Plugin{}, "", err
	}
	if len(files) == 0 {
		return Plugin{}, "根包没有 .go 文件", nil
	}
	pkgOf := make(map[string]string, len(files)) // 文件名 -> 包名
	importsOf := make(map[string][]string, len(files))
	fset := token.NewFileSet()
	for _, f := range files {
		af, perr := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if perr != nil {
			return Plugin{}, "", fmt.Errorf("解析 %s 失败: %w", f, perr)
		}
		pkgOf[f] = af.Name.Name
		var paths []string
		for _, im := range af.Imports {
			p, uerr := strconv.Unquote(im.Path.Value)
			if uerr != nil {
				continue // 转义坏掉的 import 交给编译器报
			}
			paths = append(paths, p)
		}
		importsOf[f] = paths
	}

	// 包名：plugin.json 声明优先，否则取源码（按文件名排序的第一个，稳定）
	pkg := man.Package
	if pkg == "" {
		names := make([]string, 0, len(pkgOf))
		for _, n := range pkgOf {
			names = append(names, n)
		}
		sort.Strings(names)
		pkg = names[0]
	} else {
		mismatch := true
		for _, n := range pkgOf {
			if n == pkg {
				mismatch = false
				break
			}
		}
		if mismatch {
			return Plugin{}, "", fmt.Errorf(
				"%s 的 plugin.json 声明 package=%q，但源码里的包名不是它", manifestPath, pkg)
		}
	}

	// 判据：这个包 import 了公开契约包
	isPlugin := false
	for f, paths := range importsOf {
		if pkgOf[f] != pkg {
			continue
		}
		for _, p := range paths {
			if p == pluginAPI {
				isPlugin = true
				break
			}
		}
		if isPlugin {
			break
		}
	}
	if !isPlugin {
		return Plugin{}, "根包没有 import " + pluginAPI, nil
	}

	mod := modPath
	manifestUsed := false
	if man.Module != "" && man.Module != modPath {
		mod = man.Module
		manifestUsed = true
	}

	p := Plugin{Dir: dir, Module: mod, Package: pkg, ID: man.ID, Manifest: manifestUsed}

	// 不阻断的提醒：插件自己的 go.mod 得 require（或 replace）宿主契约包，
	// 否则生成出来的 blank import 在编译期会以一句很难读的错失败。
	if raw, err := os.ReadFile(gomod); err == nil {
		if !strings.Contains(string(raw), "github.com/MYCF711/dsh-ui") {
			p.Warn = "go.mod 里没有 require github.com/MYCF711/dsh-ui，编译会失败"
		}
	}
	return p, "", nil
}

// goFiles 返回目录下非测试的 .go 文件（绝对路径，按文件名排序）。
func goFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读目录 %s 失败: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, n))
	}
	sort.Strings(out)
	return out, nil
}

// modulePath 从 go.mod 里取 module 路径。
//
// 只认 module 行：不引入 golang.org/x/mod 依赖（那是新依赖，为了读一行不值）。
func modulePath(gomod string) (string, error) {
	raw, err := os.ReadFile(gomod)
	if err != nil {
		return "", fmt.Errorf("读 %s 失败: %w", gomod, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}
		p := fields[1]
		if uq, uerr := strconv.Unquote(p); uerr == nil {
			p = uq
		}
		return p, nil
	}
	return "", fmt.Errorf("%s 里没有 module 行", gomod)
}
