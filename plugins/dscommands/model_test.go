package dscommands

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// TestModelCommandsCarryTheirOwnID 验模型命令**各自带自己的 id**。
//
// 和权限命令是同一个坑的第二个实例：`for _, m := range shown` 里不写 `m := m`，
// 所有闭包共享同一个变量，执行完整张表每条都设成**最后一个模型**。
// 症状不是崩溃，是「点哪个都变成同一个」——比"命令没生效"更难查，
// 因为状态栏确实显示"模型已设为 X"，只是 X 不对。
func TestModelCommandsCarryTheirOwnID(t *testing.T) {
	var got []string
	p := &Plugin{}
	p.act = pluginapi.Actions{
		ModelsList: func() []pluginapi.Model {
			return []pluginapi.Model{
				{ID: "group/model-a", OwnedBy: "ch1"},
				{ID: "group/model-b", OwnedBy: "ch2"},
				{ID: "streamlake/model-c", OwnedBy: "ch3"},
			}
		},
		SetModel: func(id string) { got = append(got, id) },
	}

	var modelCmds []Command
	for _, c := range p.commands() {
		if strings.HasPrefix(c.Name, "模型：") {
			modelCmds = append(modelCmds, c)
		}
	}
	// 3 个模型 + 1 条"恢复默认"
	if len(modelCmds) != 4 {
		t.Fatalf("应当是 3 个模型 + 恢复默认 = 4 条，实际 %d：%v", len(modelCmds), modelCmds)
	}
	for _, c := range modelCmds {
		c.Action()
	}
	want := []string{"group/model-a", "group/model-b", "streamlake/model-c", ""}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条设的是 %q，want %q（闭包没捕获住？）——整串 got=%v", i, got[i], want[i], got)
		}
	}
}

// TestModelCommandsCapAtEight 验上限：网关几十个模型不能全塞进命令面板。
//
// **超出的不画成命令**（`Action` 为 nil 的命令点了没反应，那就是骗人），
// 而是写进"恢复默认"的提示里。
func TestModelCommandsCapAtEight(t *testing.T) {
	many := make([]pluginapi.Model, 20)
	for i := range many {
		many[i] = pluginapi.Model{ID: "m" + itoa(i)}
	}
	p := &Plugin{}
	p.act = pluginapi.Actions{
		ModelsList: func() []pluginapi.Model { return many },
		SetModel:   func(string) {},
	}
	var modelCmds []Command
	for _, c := range p.commands() {
		if strings.HasPrefix(c.Name, "模型：") {
			modelCmds = append(modelCmds, c)
		}
	}
	// 8 个模型 + 1 条恢复默认
	if len(modelCmds) != 9 {
		t.Fatalf("上限 8 个模型 + 恢复默认 = 9 条，实际 %d", len(modelCmds))
	}
	last := modelCmds[len(modelCmds)-1]
	if !strings.Contains(last.Hint, "12") {
		t.Fatalf("恢复默认的提示里要写「另有 12 个未列出」，实际 %q", last.Hint)
	}
	for _, c := range modelCmds {
		if c.Action == nil {
			t.Fatalf("命令 %q 的 Action 是 nil——画出来点了没反应就是骗人", c.Name)
		}
	}
}

// TestNoModelsMeansNoModelCommands 验：壳没拉到模型就**一条都不画**。
func TestNoModelsMeansNoModelCommands(t *testing.T) {
	p := &Plugin{}
	p.act = pluginapi.Actions{
		ModelsList: func() []pluginapi.Model { return nil },
		SetModel:   func(string) {},
	}
	for _, c := range p.commands() {
		if strings.HasPrefix(c.Name, "模型：") {
			t.Fatalf("没有模型却画出了 %q", c.Name)
		}
	}
}
