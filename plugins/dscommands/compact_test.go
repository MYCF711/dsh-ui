package dscommands

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// compact_test.go —— 压缩命令 + 命令表**位置**的钉子。
//
// 位置为什么值得单测：`scripts/accept-theme.sh` 是**按位置**选「主题：浅色」的
// （F2 → ↓×9 → Enter），因为 viva 的 OnKey 不送字母，没有别的办法按名字选。
// 所以命令表里"第几条是什么"是一个**被脚本依赖的契约**——在这里钉住，
// 谁往中间插一条命令，测试立刻红，而不是等验收脚本莫名其妙地选错命令。

// scriptedActions 复刻"没有宿主在跑"时壳接的那一组动作。
//
// 这个形状决定命令表的顺序，也就决定 accept-theme.sh 的那个 9。
func scriptedActions(onCompact func()) pluginapi.Actions {
	return pluginapi.Actions{
		NewSession:     func() {},
		ToggleRightbar: func() {},
		Retry:          func() {},
		Cancel:         func() {},
		// 无宿主时 models.list 连不上 → 空列表 → 一条模型命令都不出现。
		ModelsList: func() []pluginapi.Model { return nil },
		ConfigGet:  func() map[string]string { return map[string]string{} },
		ConfigSet:  func(map[string]string) {},
		SetTheme:   func(string) {},
		// 权限命令在主题**之后**；压缩在最后。
		SetPermission:  func(string) {},
		CompactSession: onCompact,
	}
}

func commandNames(p *Plugin) []string {
	var out []string
	for _, c := range p.commands() {
		out = append(out, c.Name)
	}
	return out
}

// TestCompactCommandRunsTheAction 验命令真的接到动作上。
func TestCompactCommandRunsTheAction(t *testing.T) {
	var n int
	p := &Plugin{}
	p.act = scriptedActions(func() { n++ })

	var cmd *Command
	for _, c := range p.commands() {
		if c.Name == "压缩上下文" {
			c := c
			cmd = &c
			break
		}
	}
	if cmd == nil {
		t.Fatalf("壳接了 CompactSession，却没有「压缩上下文」这条命令：%v", commandNames(p))
	}
	cmd.Action()
	if n != 1 {
		t.Fatalf("执行一次应当调一次 CompactSession，实际 %d", n)
	}
	// 文案要说清"会调一次模型"：这不是免费动作，误触是有代价的。
	if !strings.Contains(cmd.Hint, "模型") {
		t.Errorf("提示里要写明会调模型，实际 %q", cmd.Hint)
	}
}

// TestNoCompactActionMeansNoCommand：壳没接时一条都不画。
func TestNoCompactActionMeansNoCommand(t *testing.T) {
	p := &Plugin{}
	act := scriptedActions(nil)
	act.CompactSession = nil
	p.act = act
	for _, name := range commandNames(p) {
		if name == "压缩上下文" {
			t.Fatal("壳没接 CompactSession，却画了「压缩上下文」——点了没反应比看不见更糟")
		}
	}
}

// TestCommandTableOrderIsStable 钉住 accept-theme.sh 依赖的那个位置。
func TestCommandTableOrderIsStable(t *testing.T) {
	p := &Plugin{}
	p.act = scriptedActions(func() {})
	names := commandNames(p)

	want := []string{
		"新建会话", "开关右栏", "重试上一条", "打断当前 turn",
		"侧栏宽一点", "侧栏窄一点", "右栏宽一点", "右栏窄一点",
		"主题：深色", "主题：浅色", "主题：跟随系统",
		"权限：每次都问我", "权限：只读", "权限：自动允许", "权限：按模式自动", "权限：恢复默认",
		"压缩上下文",
	}
	if len(names) != len(want) {
		t.Fatalf("命令 %d 条，want %d：\n  实际 %v", len(names), len(want), names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("第 %d 条是 %q，want %q。\n"+
				"命令是按位置选的（accept-theme.sh 用 ↓×9 选「主题：浅色」），"+
				"往中间插一条就会把后面整体挪一位。\n新命令往后加。\n  实际 %v",
				i, names[i], want[i], names)
		}
	}
	if names[9] != "主题：浅色" {
		t.Fatalf("accept-theme.sh 用的位置 9 现在是 %q —— 脚本会选错命令", names[9])
	}
}
