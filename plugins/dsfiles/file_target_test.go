package dsfiles

// file_target_test.go —— #18 的**生产方**钉子：文件行必须交出官方的 `file` target。
//
// 官方 `file` target 的身份是 (sessionId, 工作区相对路径)，不是"当前选中"
// （OFF:ui-context-menu/src/client/types.ts:30）。这一条错了的表现很隐蔽：
// 菜单照常弹，动作却作用在**另一个会话/另一个目录**的文件上——所以这里把
// "交出去的四个字段"钉死，而不是只钉"有菜单"。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// recordingMenus 只记录 Attach：本包不关心菜单怎么画，只关心交出去的 target。
type recordingMenus struct {
	attached []pluginapi.ContextMenuTarget
}

func (m *recordingMenus) RegisterContribution(pluginapi.ContextMenuContribution) (func(), error) {
	return func() {}, nil
}

func (m *recordingMenus) HasTarget(string) bool { return true }

func (m *recordingMenus) Attach(_ ui.C, _ gfx.ID, _ ui.Box, target pluginapi.ContextMenuTarget) {
	m.attached = append(m.attached, target)
}

func (m *recordingMenus) Button(ui.C, gfx.ID, pluginapi.ContextMenuTarget, string) bool { return false }

// TestRelPath 钉死官方 file target 的 path 形状（工作区相对、正斜杠、无前导斜杠）。
func TestRelPath(t *testing.T) {
	cases := []struct{ curDir, name, want string }{
		{"", "a.go", "a.go"},
		{"plugins", "a.go", "plugins/a.go"},
		{"/plugins/", "a.go", "plugins/a.go"},
		{`plugins\dsopen`, "a.go", "plugins/dsopen/a.go"}, // Windows 反斜杠也要归一
		{"", "/a.go", "a.go"},                             // 名字带前导斜杠不能拼出 "//"
		{"plugins", `sub\a.go`, "plugins/sub/a.go"},
	}
	for _, tc := range cases {
		if got := relPath(tc.curDir, tc.name); got != tc.want {
			t.Errorf("relPath(%q, %q) = %q，want %q", tc.curDir, tc.name, got, tc.want)
		}
	}
}

// TestFileRowAttachesFileTargetOnly 钉住两件事：
//   - 文件行交出的 target 是 file kind，并且带**会话 + 相对路径**；
//   - 目录行**不**交 target（官方 read 家族只认普通文件）。
func TestFileRowAttachesFileTargetOnly(t *testing.T) {
	menus := &recordingMenus{}
	p := &Plugin{menus: menus}
	const cwd = `D:\ws`
	listing := map[string][]pluginapi.FSEntry{
		cwd: {{Name: "a.txt"}, {Name: "sub", IsDir: true}},
	}
	act := pluginapi.Actions{
		FSList: func(dir string) []pluginapi.FSEntry { return listing[dir] },
		FSRead: func(string) (string, error) { return "", nil },
	}
	renderFiles(t, p, pluginapi.Snapshot{CWD: cwd, Cur: "sess-1"}, act, "")

	if len(menus.attached) != 1 {
		t.Fatalf("attach 次数 = %d，want 1（只有文件行挂 target，目录行不挂）", len(menus.attached))
	}
	got := menus.attached[0]
	if got.Kind != pluginapi.TargetFile {
		t.Errorf("kind = %q，want %q", got.Kind, pluginapi.TargetFile)
	}
	if got.SessionID != "sess-1" {
		t.Errorf("sessionId = %q，want %q（身份必须是捕获的会话，不是'当前选中'）", got.SessionID, "sess-1")
	}
	if got.Path != "a.txt" {
		t.Errorf("path = %q，want %q（工作区相对路径，不是绝对路径）", got.Path, "a.txt")
	}
	if got.DisplayTitle != "a.txt" {
		t.Errorf("displayTitle = %q，want %q", got.DisplayTitle, "a.txt")
	}
}

// TestFileRowWithoutSessionSkipsTarget 钉住"没有会话身份就不挂"：
// 拿不到 sessionId 时挂一个 target 出去，动作会去猜会话——那比没有菜单更坏。
func TestFileRowWithoutSessionSkipsTarget(t *testing.T) {
	menus := &recordingMenus{}
	p := &Plugin{menus: menus}
	const cwd = `D:\ws`
	act := pluginapi.Actions{
		FSList: func(string) []pluginapi.FSEntry { return []pluginapi.FSEntry{{Name: "a.txt"}} },
		FSRead: func(string) (string, error) { return "", nil },
	}
	renderFiles(t, p, pluginapi.Snapshot{CWD: cwd}, act, "")
	if len(menus.attached) != 0 {
		t.Fatalf("没有会话身份时 attach 了 %d 次，want 0", len(menus.attached))
	}
}
