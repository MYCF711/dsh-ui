package main

import (
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
)

// 这套测试钉住"运行时换主题"的三条：
//   1. 模式 → 明暗的解析（尤其是 `system` 真的问了系统，不是默认成深色）；
//   2. 换主题是**就地改同一个对象**——插件手里是它的指针；
//   3. 不认识的模式不改任何东西（悄悄忽略比"看起来生效了"好）。

// 注意：`model.inv` 是 `app.Invalidator`（**结构体**，不是接口），零值也安全——
// `Waker.Wake()` 在没有绑定唤醒函数时是空操作。所以这里没法"数重绘次数"，
// 只验证状态本身。重绘请求那条链由真实窗口跑（e2e / 截图）覆盖。
func newThemeModel(t *testing.T) *model {
	t.Helper()
	// 从深色起步：和 -theme 的历史默认一致。
	return &model{theme: dscore.Dark()}
}

func TestResolveThemeMode(t *testing.T) {
	// dark / light 是确定的。
	if dark, resolved := resolveThemeMode(themeDark); !dark || resolved != themeDark {
		t.Errorf("dark 解析成 dark=%v resolved=%q", dark, resolved)
	}
	if dark, resolved := resolveThemeMode(themeLight); dark || resolved != themeLight {
		t.Errorf("light 解析成 dark=%v resolved=%q", dark, resolved)
	}

	// system 必须**真的问系统**：拿 resolve 的结果和 app.SystemAppearance() 对。
	// 写成"system 就返回深色"也能过一次测试，但那不是跟随，是假装。
	dark, resolved := resolveThemeMode(themeSystem)
	wantLight := app.SystemAppearance() == app.AppearanceLight
	if dark == wantLight {
		t.Fatalf("system 解析成 dark=%v，但系统说 %v（wantLight=%v）——没真的问系统",
			dark, app.SystemAppearance(), wantLight)
	}
	wantResolved := themeDark
	if wantLight {
		wantResolved = themeLight
	}
	if resolved != wantResolved {
		t.Errorf("system 解析出的具体名字 = %q，want %q", resolved, wantResolved)
	}
}

// TestUnknownModeChangesNothing：不认识的模式**什么都不做**——
// 不改颜色、不改模式、也不落盘（落盘会把用户上次的选择冲掉）。
func TestUnknownModeChangesNothing(t *testing.T) {
	for _, bad := range []string{"", "DARK", "sepia", "light ", "system2"} {
		m := newThemeModel(t)
		before := *m.theme
		m.requestTheme(bad)
		if *m.theme != before {
			t.Errorf("模式 %q 不该改主题，但颜色变了", bad)
		}
		m.mu.Lock()
		pending, mode := m.pendingTheme, m.themeMode
		m.mu.Unlock()
		if pending != "" || mode != "" {
			t.Errorf("模式 %q 不该被排队（pending=%q mode=%q）", bad, pending, mode)
		}
	}
}

// TestApplySwapsTheSameObject 是这个功能的核心承诺。
//
// 每个插件在 Register 里拿的是 `*pluginapi.Theme`。换主题如果要"换对象"，
// 所有插件手里的指针就会失效。所以这里断言的是**指针身份不变、内容变了**——
// 连插件侧那种"提前拿住指针"的写法也要能看到新颜色。
func TestApplySwapsTheSameObject(t *testing.T) {
	m := newThemeModel(t)
	pluginaSide := m.theme // 模拟插件在 Register 里拿到的那个指针
	ptrBefore := m.theme
	if pluginaSide.Name != "ds-dark" {
		t.Fatalf("起点不是深色：%q", pluginaSide.Name)
	}

	m.queueTheme(themeLight)
	m.applyPendingTheme()

	if m.theme != ptrBefore {
		t.Fatal("换主题把对象换掉了——插件手里的指针会失效")
	}
	if pluginaSide.Name != "ds-light" {
		t.Errorf("插件侧拿住的指针看到的是 %q，want ds-light（就地改才看得见）", pluginaSide.Name)
	}
	if pluginaSide.BgBase == dscore.Dark().BgBase {
		t.Error("底色还是深色主题的值")
	}
	if m.themeMode != themeLight {
		t.Errorf("themeMode = %q，want light", m.themeMode)
	}
	// 包级 `theme`（paint 用的那份）也要跟着换，否则壳自己画的底色还是旧的。
	if theme.Background == dscore.VivaTheme(dscore.Dark()).Background {
		t.Error("包级 theme 变量没跟着换：壳画的背景还是深色")
	}
}

// TestSwitchBackAndForth：来回切要能回到原样，不能越切越偏。
func TestSwitchBackAndForth(t *testing.T) {
	m := newThemeModel(t)
	darkBase := dscore.Dark().BgBase

	m.queueTheme(themeLight)
	m.applyPendingTheme()
	m.queueTheme(themeDark)
	m.applyPendingTheme()

	if m.theme.BgBase != darkBase {
		t.Errorf("切回深色之后底色 = %+v，want %+v", m.theme.BgBase, darkBase)
	}
}

// TestQueueFromConfigDoesNotPersist：从配置读回来的那条路只排队。
//
// 读到的值本来就在磁盘上，再写回去只会多一次无意义的写；
// 而且那条路跑在后台 goroutine 里，不该顺手触发 IO。
func TestQueueFromConfigDoesNotPersist(t *testing.T) {
	m := newThemeModel(t)
	var (
		mu  sync.Mutex
		got = map[string]string{}
		cnt int
	)
	m.actions = pluginapi.Actions{
		ConfigSet: func(kv map[string]string) {
			mu.Lock()
			defer mu.Unlock()
			cnt++
			for k, v := range kv {
				got[k] = v
			}
		},
	}
	m.queueTheme(themeLight)
	mu.Lock()
	if cnt != 0 {
		t.Errorf("queueTheme 不该落盘，却写了 %d 次", cnt)
	}
	mu.Unlock()

	m.requestTheme(themeLight)
	mu.Lock()
	defer mu.Unlock()
	if got["ui.theme"] != themeLight {
		t.Errorf("requestTheme 应当落盘 ui.theme=%q，实际 %v", themeLight, got)
	}
}

// TestSystemAppearanceOnlyRequeuesInSystemMode 是"跟随系统"的另一半。
//
// 系统明暗变了要不要跟，取决于**用户当前选的模式**：
//   - 模式是 system  → 跟（重新排一次）
//   - 用户手动选了深/浅 → **不跟**。他的选择比系统的默认更明确，
//     系统一变化就把它冲掉，等于用户的选择"自己会跑"。
//
// 为什么这样测：真正的触发来自 `app.WatchSystemAppearance`，而它在测试里不会响
// ——除非去改操作系统的深浅设置，那是在动用户的桌面。所以把回调抽成方法直接调，
// 验的是**回调里的判断**，不验"系统那边会不会响"（那由 viva 负责，也验不了）。
func TestSystemAppearanceOnlyRequeuesInSystemMode(t *testing.T) {
	m := newThemeModel(t)

	// ① 用户选了浅色：系统再怎么变都不该被冲掉。
	m.queueTheme(themeLight)
	m.applyPendingTheme()
	drainPending(m)
	m.onSystemAppearanceChanged()
	if got := takePending(m); got != "" {
		t.Errorf("模式是 light 时，系统变化不该重排主题（排入了 %q）", got)
	}

	// ② 模式是 system：应当重排一次。
	m.mu.Lock()
	m.themeMode = themeSystem
	m.pendingTheme = ""
	m.mu.Unlock()
	m.onSystemAppearanceChanged()
	if got := takePending(m); got != themeSystem {
		t.Errorf("模式是 system 时，系统变化应当重排 system（排入了 %q，want %q）", got, themeSystem)
	}
}

func drainPending(m *model) { _ = takePending(m) }

func takePending(m *model) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	mode := m.pendingTheme
	m.pendingTheme = ""
	return mode
}
