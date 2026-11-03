package dscommands

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// TestPermissionCommandsCarryTheirOwnValue 验五条权限命令**各自带自己的值**。
//
// 这条防的是 Go 循环变量捕获的经典坑：`for _, c := range ...` 里如果
// 不写 `c := c`，五个闭包会共享同一个变量，执行完整张表里每条命令都设成
// **最后一个值**。症状不是崩溃，是"点哪个都一样"——很容易被当成"命令没生效"。
func TestPermissionCommandsCarryTheirOwnValue(t *testing.T) {
	var got []string
	p := &Plugin{}
	p.act = pluginapi.Actions{SetPermission: func(v string) { got = append(got, v) }}

	var permCmds []Command
	for _, c := range p.commands() {
		if strings.HasPrefix(c.Name, "权限") {
			permCmds = append(permCmds, c)
		}
	}
	if len(permCmds) != 5 {
		t.Fatalf("应当有 5 条权限命令（4 个值 + 恢复默认），实际 %d", len(permCmds))
	}

	// 逐条执行，收集被设进去的值
	for _, c := range permCmds {
		c.Action()
	}
	want := []string{"ask", "deny", "allow", "auto", ""}
	if len(got) != len(want) {
		t.Fatalf("调了 %d 次，want %d；got=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条设的是 %q，want %q（闭包没捕获住？）——整串 got=%v", i, got[i], want[i], got)
		}
	}
}

// TestNoPermissionActionsMeansNoCommands 验：壳没接 SetPermission 时**一条都不画**。
//
// 画出来的命令点了没反应，比看不见它更糟。
func TestNoPermissionActionsMeansNoCommands(t *testing.T) {
	p := &Plugin{}
	p.act = pluginapi.Actions{} // 什么都没接
	for _, c := range p.commands() {
		if strings.HasPrefix(c.Name, "权限") {
			t.Fatalf("没接 SetPermission 却画出了命令 %q", c.Name)
		}
	}
}
