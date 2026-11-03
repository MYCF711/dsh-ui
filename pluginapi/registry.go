package pluginapi

import "sync"

// 注册表：让「装一个插件」变成一次 blank import，而不是去改壳的参数表。
//
// 这就是 database/sql 的驱动模式——壳不认识任何插件，插件自己报到。
// 顺序由 Requires 的拓扑排序决定，注册先后不影响正确性。

var (
	regMu  sync.Mutex
	regs   []func() Plugin // 每个注册项一个工厂：每次装配拿一个新实例
	regIDs []string        // 只为去重与报错信息保留
)

// Register 登记一个内置插件工厂。
//
// 用法（插件包里的 init）：
//
//	func init() {
//	    pluginapi.Register("ds-demo", func() pluginapi.Plugin { return &Plugin{} })
//	}
//
// 同名重复登记是装配错误——两个人抢一个 id 一定有一方写错了。
func Register(id string, factory func() Plugin) {
	regMu.Lock()
	defer regMu.Unlock()
	if factory == nil {
		panic("pluginapi: Register 收到 nil 工厂（id=" + id + "）")
	}
	for _, seen := range regIDs {
		if seen == id {
			panic("pluginapi: 插件 id 重复登记: " + id)
		}
	}
	regIDs = append(regIDs, id)
	regs = append(regs, factory)
}

// Registered 返回已登记的内置插件 id（按登记顺序）。
func Registered() []string {
	regMu.Lock()
	defer regMu.Unlock()
	return append([]string(nil), regIDs...)
}

// Builtin 是「所有通过 Register 登记过的插件」这个来源。
func Builtin() Source {
	return SourceFunc(func() ([]Plugin, error) {
		regMu.Lock()
		factories := append([]func() Plugin(nil), regs...)
		regMu.Unlock()
		out := make([]Plugin, 0, len(factories))
		for _, f := range factories {
			out = append(out, f())
		}
		return out, nil
	})
}
