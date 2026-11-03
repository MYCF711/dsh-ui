// Package uiplugins 把「装了哪些插件」从壳里搬出来。
//
// 一个插件来自三条路，装配时没有区别（都是 []pluginapi.Plugin）：
//
//	内置    blank import 一个包，它自己在 init 里 Register
//	外部    配置文件里列出来（当前只能列出已经编译进来的；远程 module 是下一层）
//	测试    直接塞一个 SourceFunc
package uiplugins

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// Entry 是配置文件里的一项。
type Entry struct {
	ID      string `json:"id"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// Skip 表明这一项由壳自己注入实例（它要拿同一个实例做交互），
// 加载器照常校验 id 存在，但不返回工厂产出的那个。
type Skip func(id string) bool

// Config 是 <home>/ui-plugins.json 的形状。
//
// 之所以不是"目录扫描加载 .so/.dll"：那是进程插件层（wazero / go-plugin）的事，
// 界面线这一层只负责「装哪些」。远程 module 的加载是下一层，接口相同。
type Config struct {
	// Plugins 有内容时 = 白名单：只装列出来的；为空 = 全装（默认全开）。
	Plugins []Entry `json:"plugins"`
}

// Load 读配置（可选），返回装配用的插件列表。
//
// 语义对齐宿主线的 manifest.Apply：
//   - 配置文件不存在 -> 全装（二进制要能单独跑）
//   - 配置里出现没编译进来的 id -> 报错，不静默忽略
//   - enabled:false -> 不装
func Load(configPath string, builtin pluginapi.Source, skip Skip) ([]pluginapi.Plugin, error) {
	all, err := builtin.Plugins()
	if err != nil {
		return nil, err
	}
	if configPath == "" {
		out := make([]pluginapi.Plugin, 0, len(all))
		for _, p := range all {
			if skip != nil && skip(p.ID()) {
				continue
			}
			out = append(out, p)
		}
		return out, nil
	}
	raw, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return all, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读插件配置 %s 失败: %w", configPath, err)
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("解析插件配置 %s 失败: %w", configPath, err)
	}
	if len(cfg.Plugins) == 0 {
		return all, nil
	}

	byID := make(map[string]pluginapi.Plugin, len(all))
	for _, p := range all {
		byID[p.ID()] = p
	}
	out := make([]pluginapi.Plugin, 0, len(cfg.Plugins))
	seen := make(map[string]bool, len(cfg.Plugins))
	for _, e := range cfg.Plugins {
		if e.ID == "" {
			return nil, fmt.Errorf("插件配置里有一项缺 id")
		}
		p, ok := byID[e.ID]
		if !ok {
			return nil, fmt.Errorf("插件 %q 没有编译进这个二进制；可用: %v", e.ID, ids(all))
		}
		if seen[e.ID] {
			return nil, fmt.Errorf("插件配置里 %q 出现多次", e.ID)
		}
		seen[e.ID] = true
		if e.Enabled != nil && !*e.Enabled {
			continue
		}
		if skip != nil && skip(e.ID) {
			continue // 壳会自己 append 它的实例
		}
		out = append(out, p)
	}
	return out, nil
}

func ids(ps []pluginapi.Plugin) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.ID())
	}
	return out
}
