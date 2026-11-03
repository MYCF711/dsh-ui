// Package uiplugins 把「装了哪些插件」从壳里搬出来。
//
// 一个插件来自三条路，装配时没有区别（都是 []pluginapi.Plugin）：
//
//	内置    blank import 一个包，它自己在 init 里 Register
//	外部    仓库外 module：由配置的 dirs 扫描出来、cmd/plugingen 生成 blank import（编译期接入）
//	测试    直接塞一个 SourceFunc
//
// 注意「编译期」三个字：本包运行时只看得到**已经编译进二进制**的插件。
// 让一个外部 module 进入二进制的那一步在 cmd/plugingen（读 dirs、生成 import、
// 改 go.mod），本包的 Load 一行都不碰它。
package uiplugins

import (
	"fmt"

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

// Config 是 <home>/ui-plugins.json 的形状：
//
//	{
//	  "plugins": [{"id": "ds-demo", "enabled": true}],
//	  "dirs":    ["../dsh-ui-plugins"]
//	}
//
// 两个字段互不影响，各自有一份职责：
//   - plugins 给运行时：装哪些**已编译进来**的插件（白名单 / 关掉某个）
//   - dirs 给编译期：cmd/plugingen 去这些目录里找外部 module 并生成 blank import
//
// 之所以不是"目录扫描加载 .so/.dll"：那是进程插件层（wazero / go-plugin）的事，
// 界面线这一层只负责「装哪些」。外部 module 靠编译期接入（见 cmd/plugingen），
// 不进二进制的东西运行时一律看不到。
type Config struct {
	// Plugins 有内容时 = 白名单：只装列出来的；为空 = 全装（默认全开）。
	Plugins []Entry `json:"plugins"`
	// Dirs 是编译期插件父目录列表，**相对配置文件所在目录**解析（绝对路径原样用）。
	// 只被 cmd/plugingen 读；Load 不看它 —— 这里列出来的东西在编译前并不存在。
	Dirs []string `json:"dirs,omitempty"`
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
	cfg, found, err := readConfig(configPath)
	if err != nil {
		return nil, err
	}
	if !found {
		return all, nil
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
