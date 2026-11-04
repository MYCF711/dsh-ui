package main

import (
	"log"
	"sync"

	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
)

// theme.go 是"运行时换主题"。
//
// # 为什么不能只靠 -theme 启动参数
//
// 在这之前主题只能靠 `-theme dark|light` 在启动时定死：改一次要重启一次。
// 而官方把这件事做成 **Appearance 设置行**（`dsh-client-ui-theme`：
// "DOM-free ThemeRuntime for light/dark/**system** state"），
// 其中的 system 是"跟着系统走"——那本来就不可能靠启动参数表达。
//
// # 为什么"就地改"而不是"换个主题对象"
//
// 每个插件在 Register 里 `ctx.Inject(KeyTheme).(*pluginapi.Theme)` 拿的是**指针**。
// 换对象意味着所有插件手里的指针失效，得重新装配一遍。
// 原地把新主题的字段拷进那个对象，所有持有者下一帧就看到新颜色——
// **一个动作，不需要通知任何人**。
//
// 代价是这件事只能有一个地方写（就是这里）。谁要是自己 `*m.theme = ...`，
// 另一处就会落后半帧。

// 三种模式。`system` 不是一种颜色，是"每次解析时问一次系统"。
const (
	themeDark   = "dark"
	themeLight  = "light"
	themeSystem = "system"
)

// validThemeMode 只认这三种。传别的**不改主题**——
// 悄悄忽略比显示一个用户以为生效了的名字好。
func validThemeMode(mode string) bool {
	return mode == themeDark || mode == themeLight || mode == themeSystem
}

// resolveThemeMode 把模式解析成"深色还是浅色"。
//
// 系统说不出来时（`AppearanceUnknown`：某些桌面、以及无头驱动）退回深色，
// 和 `-theme` 的历史默认值一致——**后退要退到旧行为，不是退到新惊喜**。
func resolveThemeMode(mode string) (dark bool, resolved string) {
	switch mode {
	case themeLight:
		return false, themeLight
	case themeSystem:
		if app.SystemAppearance() == app.AppearanceLight {
			return false, themeLight
		}
		return true, themeDark
	}
	return true, themeDark
}

// queueTheme 只排队、不落盘。从配置读回来的那条路走这里——
// 读到的值本来就是磁盘上的，再写一次只会多一次无意义的写。
func (m *model) queueTheme(mode string) {
	m.mu.Lock()
	m.pendingTheme = mode
	m.mu.Unlock()
	m.inv.Invalidate()
}

// requestTheme 只是**排队**：真正改颜色发生在下一帧的 Paint 里（见 applyPendingTheme）。
//
// 为什么不在调用点直接改：`-keys` 注入那条路是从另一个 goroutine 调 HandleKey 的，
// 而包级 `theme` 变量是绘制线程在读——两边同时动就是一个数据竞争。
// 排队 + 在 Paint 里消费，让"写"永远只发生在 UI 线程。
func (m *model) requestTheme(mode string) {
	if !validThemeMode(mode) {
		m.setStatus("未知主题：" + mode)
		return
	}
	m.queueTheme(mode)

	// 落盘：下次启动还是这个模式。`ui.theme` 和其它 `ui.*` 一样由宿主的
	// config KV 存（宿主不解释值）。
	if m.actions.ConfigSet != nil {
		m.actions.ConfigSet(map[string]string{"ui.theme": mode})
	}
}

// 系统外观的监听只装一次。装过就不再装——重复装会让一次系统切换触发多次重绘。
var systemWatchOnce sync.Once

// ensureSystemWatch 在"跟随系统"生效期间听系统的明暗变化。
//
// 没有它的话，`system` 只是"切到 system 那一刻系统是什么"——用户之后再切系统的
// 深浅，界面就不跟了。而"跟随系统"这个名字承诺的是**一直跟**。
//
// 回调里只在当前模式仍是 system 时才重排一次：用户已经手动选了深色，
// 系统再变也不该把他的选择冲掉。
func (m *model) ensureSystemWatch() {
	systemWatchOnce.Do(func() {
		app.WatchSystemAppearance(func() {
			m.mu.Lock()
			mode := m.themeMode
			m.mu.Unlock()
			if mode == themeSystem {
				m.queueTheme(themeSystem)
			}
		})
	})
}

// applyPendingTheme 消费排队的主题。**必须在 UI 线程（Paint）里调。**
func (m *model) applyPendingTheme() {
	m.mu.Lock()
	mode := m.pendingTheme
	m.pendingTheme = ""
	m.mu.Unlock()
	if mode == "" || m.theme == nil {
		return
	}
	dark, resolved := resolveThemeMode(mode)
	if dark {
		*m.theme = *dscore.Dark()
	} else {
		*m.theme = *dscore.Light()
	}
	theme = dscore.VivaTheme(m.theme)
	m.mu.Lock()
	m.themeMode = mode
	m.mu.Unlock()
	if mode == themeSystem {
		m.ensureSystemWatch()
	}
	log.Printf("主题：%s（实际 %s）", mode, resolved)
}
