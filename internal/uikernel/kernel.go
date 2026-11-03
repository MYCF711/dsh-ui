// Package uikernel 是界面线的插件运行时：作用域、能力表、装配器。
//
// 它是宿主线 internal/kernel 的界面侧对应物，刻意保持同样的语义：
//
//	作用域（Scope）  插件实例的生命周期，Provide/Inject/Effect 三个动作
//	装配（Load）     按 Requires 拓扑排序；缺依赖就**报错**，不静默跳过
//	画布（Painters） 装配结果按槽位和顺序摊平，壳只管照着画
//
// 本包不认识任何具体组件（侧栏 / 会话流 / 输入框都不在这里），这是 R1 的界面版。
package uikernel

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// Kernel 是插件运行时的根。
type Kernel struct {
	root    *Scope
	log     *slog.Logger
	byID    map[string]pluginapi.Plugin
	order   []string // 装配成功的插件 id，按装配顺序
	provide map[string]string
	values  map[string]any
	painters []pluginapi.Painter
	// scopes 记下每个插件的作用域，卸载时按装配逆序跑它们的 Effect。
	scopes map[string]*Scope
}

// New 创建根内核。
func New(name string) *Kernel {
	k := &Kernel{
		log:     slog.Default(),
		byID:    map[string]pluginapi.Plugin{},
		provide: map[string]string{},
		values:  map[string]any{},
	}
	k.root = &Scope{kernel: k, name: name}
	return k
}

// Log 返回内核的日志器。
func (k *Kernel) Log() *slog.Logger { return k.log }

// Root 返回根作用域。
func (k *Kernel) Root() *Scope { return k.root }

// Scope 是一个插件实例的作用域。
type Scope struct {
	kernel   *Kernel
	name     string
	provides []string
	effects  []func()
	disposed bool
}

// Name 实现 pluginapi.Context。
func (s *Scope) Name() string { return s.name }

// Provide 登记一个能力。键重复即装配错误（两个插件抢同一个键一定有一方是错的）。
func (s *Scope) Provide(key string, value any) {
	s.kernel.provide[key] = s.name
	s.kernel.values[key] = value
	s.provides = append(s.provides, key)
	s.kernel.log.Debug("provide", "plugin", s.name, "key", key)
}

// Inject 取一个能力，没有就返回 nil。
func (s *Scope) Inject(key string) any { return s.kernel.values[key] }

// Effect 登记卸载时要跑的清理函数。
func (s *Scope) Effect(fn func()) {
	if fn != nil {
		s.effects = append(s.effects, fn)
	}
}

// Logf 打日志，前缀是插件名。
func (s *Scope) Logf(format string, args ...any) {
	s.kernel.log.Info(s.name+": "+fmt.Sprintf(format, args...))
}

// Load 按依赖顺序装配插件，并登记它们挂的画。
//
// 语义（对齐宿主线 manifest.Apply 的「不静默」原则）：
//   - ID 重复 → 错误
//   - Provides 撞车 → 错误
//   - Requires 里有谁都没提供的能力 → 错误，**列出缺的是哪个键、谁要的**
//   - 有环 → 错误，列出环上的插件
//
// 「少画一块但不报错」是这类架构里最难查的 bug，所以这里一律 fail loud。
func (k *Kernel) Load(plugins ...pluginapi.Plugin) error {
	pending := make([]pluginapi.Plugin, 0, len(plugins))
	for _, p := range plugins {
		if p == nil {
			return fmt.Errorf("uikernel: 传进来一个 nil 插件")
		}
		id := p.ID()
		if id == "" {
			return fmt.Errorf("uikernel: 有插件没有 ID")
		}
		if _, dup := k.byID[id]; dup {
			return fmt.Errorf("uikernel: 插件 id %q 重复", id)
		}
		k.byID[id] = p
		pending = append(pending, p)
	}

	// 拓扑排序：每一轮挑出「依赖全齐」的插件装配。轮数退化为 0 就说明有环或缺依赖。
	for len(pending) > 0 {
		var next []pluginapi.Plugin
		progressed := false
		for _, p := range pending {
			if !k.satisfied(p) {
				next = append(next, p)
				continue
			}
			if err := k.install(p); err != nil {
				return err
			}
			progressed = true
		}
		if !progressed {
			return fmt.Errorf("uikernel: 装配卡住了，这些插件的依赖没人提供或有环: %s", describe(next))
		}
		pending = next
	}
	return nil
}

// satisfied 判断一个插件的 Requires 是否都已就位。
func (k *Kernel) satisfied(p pluginapi.Plugin) bool {
	for _, key := range p.Requires() {
		if _, ok := k.values[key]; !ok {
			return false
		}
	}
	return true
}

// install 调 Register，并在成功后就地收集它挂的画。
func (k *Kernel) install(p pluginapi.Plugin) error {
	scope := &Scope{kernel: k, name: p.ID()}
	for _, key := range p.Provides() {
		if owner, dup := k.provide[key]; dup {
			return fmt.Errorf("uikernel: 能力 %q 被 %s 和 %s 同时提供", key, owner, p.ID())
		}
	}
	if err := p.Register(scope); err != nil {
		return fmt.Errorf("uikernel: 插件 %s 注册失败: %w", p.ID(), err)
	}
	k.order = append(k.order, p.ID())
	k.registerScope(p.ID(), scope)

	if withPanels, ok := p.(pluginapi.Panels); ok {
		for _, painter := range withPanels.Panels() {
			if painter.Draw == nil || painter.Slot == "" {
				return fmt.Errorf("uikernel: 插件 %s 挂了一块没有槽位或没有 Draw 的画", p.ID())
			}
			k.painters = append(k.painters, painter)
		}
	}
	k.log.Info("插件已装配", "id", p.ID(), "provides", p.Provides(), "requires", p.Requires())
	return nil
}

// Get 取一个能力；没提供过返回 nil。
func (k *Kernel) Get(key string) any { return k.values[key] }

// MustGet 取一个能力，缺失直接 panic。
//
// 只给壳在启动阶段用（此时装配已经成功，缺键就是代码 bug，早炸比晚炸好）。
func (k *Kernel) MustGet(key string) any {
	v, ok := k.values[key]
	if !ok {
		panic("uikernel: 装配完成后仍然没有能力 " + key)
	}
	return v
}

// Order 返回装配顺序（插件 id）。
func (k *Kernel) Order() []string { return append([]string(nil), k.order...) }

// Painters 返回某个槽位的画，已按 Order 排好序。
//
// 排序键是 (Order, 装配顺序)：Order 相同的按装配顺序，保证同一份插件集合
// 每次画出来都一样（不稳定性会让截图对比失效）。
func (k *Kernel) Painters(slot pluginapi.Slot) []pluginapi.Painter {
	var out []pluginapi.Painter
	for _, p := range k.painters {
		if p.Slot == slot {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// Has 判断某个槽位有没有画（壳据此决定要不要留白、要不要显示分隔线）。
func (k *Kernel) Has(slot pluginapi.Slot) bool { return len(k.Painters(slot)) > 0 }

// Dispose 按装配的逆序卸载。
func (k *Kernel) Dispose() {
	for i := len(k.order) - 1; i >= 0; i-- {
		if s, ok := k.scopeOf(k.order[i]); ok {
			s.disposed = true
			for j := len(s.effects) - 1; j >= 0; j-- {
				s.effects[j]()
			}
		}
	}
}

// scopeOf 取某个插件的作用域（卸载用）。
func (k *Kernel) scopeOf(id string) (*Scope, bool) {
	s, ok := k.scopes[id]
	return s, ok
}
func (k *Kernel) registerScope(id string, s *Scope) {
	if k.scopes == nil {
		k.scopes = map[string]*Scope{}
	}
	k.scopes[id] = s
}

// describe 把卡住的插件列成人能看的一行。
func describe(ps []pluginapi.Plugin) string {
	var b strings.Builder
	for i, p := range ps {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s 需要 %v", p.ID(), p.Requires())
	}
	return b.String()
}
