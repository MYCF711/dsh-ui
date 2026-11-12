package dscommands

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dssettings"
	"github.com/vivaui/viva/gfx"
)

// settings_test.go —— F2 面板里的「设置」行（A 方案）的行为钉子。
//
// 这一行不是字符串：它背后必须**真的**是设置浮层的打开动作
// （dssettings 通过 `ui.settings.open` Provide 的 `func() bool`）。
// 所以这里的判据是：
//   - 服务在 → 命令表**末尾**多一条「设置」，执行它 = 调一次那个服务；
//     走真实 Enter 键路径（不是直接调 Action）也要能打开；
//   - 服务缺失 / 类型不对 / typed nil → **一条都不加、装配不 panic**，
//     其余命令的名字、提示、顺序逐字不变；
//   - 布局脚本依赖位置（accept-theme.sh 用 ↓×9 选「主题：浅色」），
//     所以「设置」必须排在最后，前面的 18 条一个位置都不许挪。

// fakeSettings 替身只做一件事：Provide `ui.settings.open`，被调一次记一次。
// 它不是 dssettings 的复制品——真实 Open() 的行为由 dssettings 自己的测试和
// 真进程截图证明；这里证明的是 dscommands **接上了那个服务、调的是它**。
type fakeSettings struct {
	opens int
}

func (*fakeSettings) ID() string         { return "fake-settings" }
func (*fakeSettings) Provides() []string { return []string{settingsServiceKey} }
func (*fakeSettings) Requires() []string { return nil }
func (f *fakeSettings) Register(ctx pluginapi.Context) error {
	ctx.Provide(settingsServiceKey, func() bool {
		f.opens++
		return true
	})
	return nil
}

// wrongSettings 提供一个类型不对的值（或 typed nil），用来钉"零 panic"。
type wrongSettings struct{ value any }

func (*wrongSettings) ID() string         { return "wrong-settings" }
func (*wrongSettings) Provides() []string { return []string{settingsServiceKey} }
func (*wrongSettings) Requires() []string { return nil }
func (w *wrongSettings) Register(ctx pluginapi.Context) error {
	ctx.Provide(settingsServiceKey, w.value)
	return nil
}

// loadCommands 按真实装配形状装一遍：dscore（令牌/图元）→ 可选服务 → ds-commands。
//
// 生产顺序是 ds-settings 排在 ds-commands 之前（见 cmd/app/main.go 的
// `plugins = append([]pluginapi.Plugin{m.settings}, plugins...)` 与
// cmd/app/settings_order_test.go）。这里照抄那个先后关系，否则 Inject 拿不到值。
func loadCommands(t *testing.T, extra ...pluginapi.Plugin) *Plugin {
	t.Helper()
	p := &Plugin{}
	plugins := append([]pluginapi.Plugin{&dscore.Plugin{Dark: true}}, extra...)
	plugins = append(plugins, p)
	kernel := uikernel.New("dscommands-settings")
	if err := kernel.Load(plugins...); err != nil {
		t.Fatalf("装配失败（Register 里不该 panic/报错）: %v", err)
	}
	return p
}

type namedHint struct{ name, hint string }

func commandPairs(p *Plugin) []namedHint {
	var out []namedHint
	for _, c := range p.commands() {
		out = append(out, namedHint{c.Name, c.Hint})
	}
	return out
}

// baselineCommands 是"壳没装设置浮层"时的命令表（18 条）。
// 顺序被 scripts/accept-theme.sh 依赖（↓×9 = 主题：浅色），文案逐字钉住。
var baselineCommands = []namedHint{
	{"新建会话", "开一个新会话"},
	{"开关右栏", "显示或收起轨迹栏"},
	{"重试上一条", "重发最近一条用户消息"},
	{"打断当前 turn", "停止正在跑的这一轮"},
	{"侧栏宽一点", "当前值 +40（也可以直接拖分隔线）"},
	{"侧栏窄一点", "当前值 -40"},
	{"右栏宽一点", "当前值 +40（也可以直接拖分隔线）"},
	{"右栏窄一点", "当前值 -40"},
	{"主题：深色", "官方深色令牌（ds-dark）"},
	{"主题：浅色", "官方浅色令牌（ds-light）"},
	{"主题：跟随系统", "系统切浅色就浅色、切深色就深色"},
	{"权限：每次都问我", "写类工具每次都要点允许（ask）"},
	{"权限：只读", "写类工具一律拒绝（deny）"},
	{"权限：自动允许", "写类工具不问直接跑（allow）"},
	{"权限：按模式自动", "由当前模式决定（auto）"},
	{"权限：恢复默认", "回到宿主的 --policy，不留覆盖"},
	{"压缩上下文", "折叠较早的消息成摘要（会调一次模型）"},
	{"分支这个会话", "整份复制成新会话并切过去（不是从某条消息往后）"},
}

func assertBaselinePrefix(t *testing.T, got []namedHint) {
	t.Helper()
	if len(got) < len(baselineCommands) {
		t.Fatalf("命令表只有 %d 条，连原有 %d 条都不够：%v", len(got), len(baselineCommands), got)
	}
	for i, want := range baselineCommands {
		if got[i] != want {
			t.Fatalf("第 %d 条变了：got %+v，want %+v。\n"+
				"其余命令的顺序/文案是 accept-theme.sh 等脚本与用户肌肉记忆共同依赖的契约，"+
				"设置行只能加在末尾。\n  实际 %v", i, got[i], want, got)
		}
	}
}

// TestSettingsServiceAppearsLastAndIsExecutable 是主用例：
// 服务在 → 末尾一条「设置」→ Action 真的调到服务 → 真实 Enter 也能执行。
func TestSettingsServiceAppearsLastAndIsExecutable(t *testing.T) {
	// 防漂移：dscommands 里写的是字面量（插件之间不互相 import），
	// 一旦 dssettings 改了键名，这里立刻红，而不是"产品里设置行凭空消失"。
	if settingsServiceKey != dssettings.ServiceKey {
		t.Fatalf("服务键不一致：dscommands=%q，dssettings.ServiceKey=%q",
			settingsServiceKey, dssettings.ServiceKey)
	}

	fake := &fakeSettings{}
	p := loadCommands(t, fake)
	if p.openSettings == nil {
		t.Fatal("装了设置插件却没 Inject 到 ui.settings.open（服务键拼错了？）")
	}
	p.act = scriptedActions(func() {})

	got := commandPairs(p)
	assertBaselinePrefix(t, got)
	if len(got) != len(baselineCommands)+1 {
		t.Fatalf("有服务时命令表应当是 %d 条（末尾多「设置」），实际 %d：%v",
			len(baselineCommands)+1, len(got), got)
	}
	last := p.commands()[len(got)-1]
	if last.Name != "设置" {
		t.Fatalf("最后一条是 %q，want「设置」——设置行必须在末尾", last.Name)
	}
	if last.Action == nil {
		t.Fatal("「设置」没有 Action：画出来也点不了，等于死按钮")
	}
	for _, want := range []string{"外观", "模型", "权限", "会话"} {
		if !strings.Contains(last.Hint, want) {
			t.Errorf("提示要写清浮层里有哪几块（缺 %q）：%q", want, last.Hint)
		}
	}

	// ① 直接执行：调到的必须是服务本体。
	last.Action()
	last.Action()
	if fake.opens != 2 {
		t.Fatalf("执行两次「设置」应当调两次打开服务，实际 %d 次", fake.opens)
	}

	// ② 走真实按键路径：F2 打开 → ↓ 到最后一条 → Enter。
	// 这一步证明"末行"和"光标/执行"是同一个位置，不是只在表里躺着。
	p.shown = true
	p.cursor = len(p.filtered()) - 1
	if okay := p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEnter}); !okay {
		t.Fatal("面板开着时 Enter 必须被命令面板消费")
	}
	if fake.opens != 3 {
		t.Fatalf("Enter 落在「设置」上应当再调一次打开服务，实际总共 %d 次", fake.opens)
	}
	if p.shown {
		t.Fatal("执行完命令面板应当收起（设置浮层由服务自己打开）")
	}
}

// TestNoSettingsServiceKeepsCommandTableVerbatim 钉住"服务缺失 = 零新增"，
// 而且原有 18 条的顺序/文案/动作一个字节都不动。
func TestNoSettingsServiceKeepsCommandTableVerbatim(t *testing.T) {
	var newSession, cancel int
	p := loadCommands(t) // 没有 dssettings
	if p.openSettings != nil {
		t.Fatal("没装设置插件却拿到了 openSettings")
	}
	act := scriptedActions(func() {})
	act.NewSession = func() { newSession++ }
	act.Cancel = func() { cancel++ }
	p.act = act

	got := commandPairs(p)
	if len(got) != len(baselineCommands) {
		t.Fatalf("无服务时命令表应当是原有 %d 条，实际 %d：%v",
			len(baselineCommands), len(got), got)
	}
	assertBaselinePrefix(t, got)
	for _, c := range p.commands() {
		if c.Name == "设置" {
			t.Fatal("没有 ui.settings.open 却画出了「设置」——点了没反应比看不见更糟")
		}
	}

	// 动作也没被换掉：原有命令照旧调到原来的 Action。
	for _, c := range p.commands() {
		switch c.Name {
		case "新建会话":
			c.Action()
		case "打断当前 turn":
			c.Action()
		}
	}
	if newSession != 1 || cancel != 1 {
		t.Fatalf("原有命令的动作被改动了：NewSession=%d Cancel=%d，want 1/1", newSession, cancel)
	}
}

// TestMalformedSettingsServiceIsIgnored 钉住"服务不可用 = 零新增、零 panic"。
func TestMalformedSettingsServiceIsIgnored(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{"类型不对（string）", "ui.settings.open"},
		{"类型不对（int）", 42},
		{"typed nil 的 func() bool", (func() bool)(nil)},
		{"func() 但缺 bool 返回", func() {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := loadCommands(t, &wrongSettings{value: tc.value})
			if p.openSettings != nil {
				t.Fatalf("值 %#v 不该被当成可用的打开动作", tc.value)
			}
			p.act = scriptedActions(func() {})
			got := commandPairs(p)
			assertBaselinePrefix(t, got)
			if len(got) != len(baselineCommands) {
				t.Fatalf("坏服务不该改变命令表：%v", got)
			}
		})
	}
}
