package dsassets

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestContextMenuOfficialIcons(t *testing.T) {
	for _, key := range []string{
		"workspace.rename", "workspace.delete", "workspace.copy",
		"open-in-app.copy", "file-actions.copy", "file-actions.download", "file-actions.save",
	} {
		icon, ok := ContextMenuIcon(key, gfx.RGB24(0xffffff))
		if !ok || icon.ViewBox != 16 || len(icon.Commands) == 0 {
			t.Errorf("%s 官方图标不可用：ok=%v icon=%+v", key, ok, icon)
		}
	}
	if _, ok := ContextMenuIcon("invented.icon", gfx.Color{}); ok {
		t.Fatal("未知图标 key 不得拿自造形状补位")
	}
	ellipsis, ok := ContextMenuButtonIcon(gfx.RGB24(0xffffff))
	if !ok || ellipsis.Stroke != 0 || len(ellipsis.Layers) != 3 {
		t.Fatalf("省略号应是官方三枚填充圆点：ok=%v icon=%+v", ok, ellipsis)
	}
	copyIcon, _ := ContextMenuIcon("workspace.copy", gfx.RGB24(0xffffff))
	if len(copyIcon.Layers) != 1 {
		t.Fatal("copy 的后一张纸必须保留官方填充层，不能全改成描边")
	}
	saveIcon, _ := ContextMenuIcon("file-actions.save", gfx.Color{})
	if saveIcon.Join != gfx.JoinRound || saveIcon.Cap != gfx.CapButt {
		t.Fatalf("save 应保持官方 join=round、默认 cap=butt：%+v", saveIcon)
	}
}
