package uidecl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// EnvDir 是声明目录的环境变量名。
//
// 用环境变量而不是命令行参数：`cmd/app` 的 flag 表不许再加东西（那会把
// 「加一个部件」的成本又推回 Go 代码里），而环境变量是**外面**能控制的东西——
// 换一套配置只要换一个变量的值，测试、截图、多开都能各用各的目录。
const EnvDir = "DSH_UI_DECL_DIR"

// SubDir 是默认声明目录在用户配置目录下的相对路径：dsh-ui/ui-plugins.d。
const SubDir = "ui-plugins.d"

// DefaultDir 返回默认声明目录：<用户配置目录>/dsh-ui/ui-plugins.d。
//
// Windows 下 os.UserConfigDir() 就是 %APPDATA%（Roaming），也就是用户在
// 「资源管理器 → 地址栏输入 %APPDATA%」能直接打开的地方——第三方不用记路径。
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("取不到用户配置目录: %w", err)
	}
	return filepath.Join(base, "dsh-ui", SubDir), nil
}

// LoadDir 扫描目录下所有 *.json，逐个校验。
//
// 目录不存在 = 这台机器上没有声明式插件 = **正常状态**，返回 (nil, nil)。
// 这是唯一一处「静止不算错」：绝大多数机器根本不会建这个目录，
// 为它报错等于每次启动都吓唬用户一次。
//
// 其余情况一律返回错误，且错误信息里一定有文件名：
// 读不了、JSON 坏了、缺 id、槽位/颜色/kind 不认识、两个文件抢同一个 id。
func LoadDir(dir string) ([]*Decl, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读声明目录 %s 失败: %w", dir, err)
	}

	seen := make(map[string]string, len(entries))
	out := make([]*Decl, 0, len(entries))
	// ReadDir 已按文件名排序，所以画出来的顺序在不同机器上一致（截图对比要用）。
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), ".json") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: 读不了: %w", path, err)
		}
		d, err := parseJSON(path, data)
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[d.ID]; dup {
			return nil, fmt.Errorf("%s: 部件 id %q 已经被 %s 用过了（id 必须全局唯一，否则装配时两个部件会打架）", path, d.ID, prev)
		}
		seen[d.ID] = path
		out = append(out, d)
	}
	return out, nil
}

// Load 扫描 + 包装：装配者想要「目录 → []pluginapi.Plugin」时用它。
func Load(dir string, theme *pluginapi.Theme, prims pluginapi.Primitives) ([]pluginapi.Plugin, error) {
	decls, err := LoadDir(dir)
	if err != nil {
		return nil, err
	}
	return Plugins(decls, theme, prims), nil
}
