// Package dsbrand 是**品牌面**的独立部件：官方鲸鱼 logo 的矢量路径 + viewBox，
// 以及品牌名的官方 wordmark 矢量（`brand_wordmark.go`）。
//
// 为什么值得单独一个包：官方把它做成 `@deepseek-ai/dsh-client-ui-brand-official`，
// 这说明**品牌是可插拔的**——白标版本应该能换掉 logo 而不动会话渲染代码。
// 之前它是 `dsconversation` 里的一个内部函数（`fishIcon`），谁要用都得 import
// 那个庞大的会话包。
//
// 数据来源：`@deepseek-ai/dsh-client-ui-primitives` 的 FISH_LOGO_PATH /
// FISH_LOGO_VIEWBOX（该包在 npm 无独立目录，内容打进了
// `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`）。品牌名与徽标的几何、
// 以及**两个不同的标记尺寸**（首屏 34 / 侧栏 24）的出处见 brand_wordmark.go 头部。
//
// 本包同时提供 `ui.brand` 能力：第三方插件注册自己的 `Brand` 就能整体替换，
// 没有的话消费方用包级默认 `Default()`。
package dsbrand

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// ServiceKey 是品牌能力的注入键。
const ServiceKey = "ui.brand"

// Brand 是一个品牌面：一个图标 + 它的标准显示尺寸。
type Brand struct {
	// Icon 是 logo 矢量（含 viewBox）。
	Icon gfx.Icon
	// Width 是官方指定的显示宽度（EmptyHero.tsx:104：`width={34}`）。
	Width float32
}

// Default 返回内置的官方品牌（DeepSeek 鲸鱼）。
//
// 这里给的是**首屏**尺寸（34）；侧栏品牌标记是另一个值（SidebarMarkSize = 24，
// SidebarRoot.tsx:297），两者不可互借——见 brand_wordmark.go 的常量表。
func Default() Brand {
	return Brand{Icon: FishIcon(), Width: HeroMarkWidth}
}

// Service 是 `ui.brand` 能力的形状。
type Service struct {
	// Get 返回当前品牌；nil 表示没有自定义品牌。
	Get func() (Brand, bool)
}

// Plugin 提供 `ui.brand`。
type Plugin struct{}

func (*Plugin) ID() string { return "ds-brand" }

// Provides 让消费方可以用 Inject(ServiceKey) 拿到品牌，
// 而不用 import 本包（第三方可以替换实现）。
func (*Plugin) Provides() []string { return []string{ServiceKey} }

func (*Plugin) Requires() []string { return nil }

func (p *Plugin) Register(ctx pluginapi.Context) error {
	ctx.Provide(ServiceKey, Service{
		Get: func() (Brand, bool) { return Default(), true },
	})
	return nil
}
