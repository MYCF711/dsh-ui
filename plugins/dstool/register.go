package dstool

import "github.com/MYCF711/dsh-ui/pluginapi"

// 自己在 init 里报到：壳不需要知道有这个插件（见 pluginapi.Register 的注释）。
// 接入方式是 cmd/app/main.go 里一行 blank import。
func init() {
	pluginapi.Register("ds-tool", func() pluginapi.Plugin { return &Plugin{} })
}

// Plugin 是工具卡这个部件的插件外壳。
//
// 它只做三件事：报 ID、声明提供 ui.tool-card、在 Register 里把 ds-core 的
// 基础组件接过来。画法全在 Service 里，因此换渲染实现不影响装配。
type Plugin struct {
	cards *Cards
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-tool" }

// Provides 实现 pluginapi.Plugin：消息流靠这个键拿到工具卡渲染器。
func (*Plugin) Provides() []string { return []string{ServiceKey} }

// Requires 实现 pluginapi.Plugin。
//
// 只要主题和基础组件：工具卡是"画在别人的框里"的部件（它不占槽位、没有
// Panels），所以不需要任何布局能力，也就不该让装配顺序为它让步。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	prims, _ := ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	p.cards = New(prims)
	ctx.Provide(ServiceKey, p.cards)
	return nil
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ Service          = (*Cards)(nil)
)
