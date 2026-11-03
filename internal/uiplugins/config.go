package uiplugins

// 配置文件里与「编译期外部插件」有关的那一半：dirs。
//
// 三条路的边界要分清：
//
//	plugins[]  运行时白名单：只决定「已经编译进二进制的插件装不装」
//	dirs[]     编译期扫描目录：只被 cmd/plugingen 读取，生成 blank import
//	Load()     运行时装配：**完全不看 dirs**，它看不到没编译进来的东西
//
// 所以 dirs 改了必须重跑 cmd/plugingen 并重新编译；配置本身不动二进制。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigDir 返回配置文件的所在目录（绝对路径）。
//
// 配置路径为空时返回当前工作目录 —— 与 Load 里「空路径 = 不用配置」的语义对齐。
func ConfigDir(configPath string) (string, error) {
	if configPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("取当前工作目录失败: %w", err)
		}
		return wd, nil
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("解析插件配置路径 %s 失败: %w", configPath, err)
	}
	return filepath.Dir(abs), nil
}

// ResolveDirs 把 Config.Dirs 解析成绝对路径，相对路径按配置文件所在目录算。
//
// 绝对路径原样保留（Clean 一下），相对路径 join 到配置目录上。
// 空项直接报错：静默跳过会让「我明明配了」变成最难查的一类问题。
func (c Config) ResolveDirs(configPath string) ([]string, error) {
	if len(c.Dirs) == 0 {
		return nil, nil
	}
	base, err := ConfigDir(configPath)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(c.Dirs))
	for _, d := range c.Dirs {
		raw := strings.TrimSpace(d)
		if raw == "" {
			return nil, fmt.Errorf("插件配置 %s 的 dirs 里有一个空项", configPath)
		}
		p := raw
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, p)
		}
		out = append(out, filepath.Clean(p))
	}
	return out, nil
}

// ReadConfig 读配置：文件不存在返回零值配置且不报错（和 Load 一样，
// 二进制要能在没有配置的情况下单独跑）。解析失败才是错误。
func ReadConfig(configPath string) (Config, error) {
	cfg, _, err := readConfig(configPath)
	return cfg, err
}

// readConfig 额外告诉调用方「文件到底在不在」——
// Load 需要这个信号：配置缺省时它不套白名单也不套 skip，直接全装。
func readConfig(configPath string) (Config, bool, error) {
	var cfg Config
	if configPath == "" {
		return cfg, false, nil
	}
	raw, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, fmt.Errorf("读插件配置 %s 失败: %w", configPath, err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, true, fmt.Errorf("解析插件配置 %s 失败: %w", configPath, err)
	}
	return cfg, true, nil
}
