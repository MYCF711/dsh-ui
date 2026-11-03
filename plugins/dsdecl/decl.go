package dsdecl

import (
	"fmt"
	"os"

	"github.com/MYCF711/dsh-ui/internal/uidecl"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// Plugin 是声明通道的装配者。它持有的状态很少：注入来的主题/图元，
// 以及这次扫到的声明——声明本身是只读的，所以没有第二份状态需要同步。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
	dir   string
	decls []*uidecl.Decl
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-decl" }

// Provides 实现 pluginapi.Plugin：声明部件不对外提供能力（它们连代码都没有）。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin：画东西只需要令牌 + 图元。
//
// 刻意**不**要求别的东西：声明通道和会话数据无关，缺会话也要能画（{sessions} 就是 0）。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Dir 返回这次扫描的目录（没扫就是空串；日志与测试用）。
func (p *Plugin) Dir() string { return p.dir }

// Decls 返回这次扫到的声明（测试用；运行时只读）。
func (p *Plugin) Decls() []*uidecl.Decl { return p.decls }

// Register 扫目录并把声明挂上去。
//
// 失败策略：**返回错误而不是跳过**。壳拿到错误会 log.Fatalf 并打出文件名，
// 这比「少画一块、日志里一句话都没有」好得多——后者会让第三方反复怀疑自己。
// 唯一的例外是「目录不存在」，那是正常状态（见 uidecl.LoadDir）。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	theme, _ := ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	if theme == nil {
		return fmt.Errorf("ds-decl: 拿不到 %s——它的提供者没排在前面", pluginapi.KeyTheme)
	}
	prims, _ := ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	if prims == nil {
		return fmt.Errorf("ds-decl: 拿不到 %s——它的提供者没排在前面", pluginapi.KeyPrimitives)
	}
	p.theme, p.prims = theme, prims

	dir := os.Getenv(uidecl.EnvDir)
	if dir == "" {
		def, err := uidecl.DefaultDir()
		if err != nil {
			// 取不到用户配置目录 == 这台机器上没有声明目录，不是错误。
			ctx.Logf("跳过声明式插件（%v）", err)
			return nil
		}
		dir = def
	}

	decls, err := uidecl.LoadDir(dir)
	if err != nil {
		return err
	}
	p.dir, p.decls = dir, decls
	if len(decls) == 0 {
		ctx.Logf("声明目录 %s 里没有 *.json，不挂任何部件", dir)
		return nil
	}
	// 每个声明单独打一行：这是「我到底挂上了没有」的唯一权威答案。
	for _, d := range decls {
		ctx.Logf("声明部件 %s: %s @ %s order=%d ← %s", d.ID, d.Kind, d.Slot, d.Order, d.File)
	}
	return nil
}

// Panels 实现 pluginapi.Panels：把每个声明包成的插件摊平成画。
//
// 一个声明 = 一个 pluginapi.Plugin（它有自己的 ID / Slot / Order），
// 但在内核眼里只有 ds-decl 一个插件——所以这里把它们的画汇总起来。
func (p *Plugin) Panels() []pluginapi.Painter {
	out := make([]pluginapi.Painter, 0, len(p.decls))
	for _, d := range p.decls {
		with, ok := d.Plugin(p.theme, p.prims).(pluginapi.Panels)
		if !ok {
			continue
		}
		out = append(out, with.Panels()...)
	}
	return out
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)
