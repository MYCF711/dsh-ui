package dsassets

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestScheduleActionIconsUseOfficialSpriteGeometry(t *testing.T) {
	fill := gfx.RGB24(0x667788)
	for name, build := range map[string]func(gfx.Color) (gfx.Icon, bool){
		"play":  SchedulePlayOutline,
		"pause": SchedulePauseOutline,
		"close": ScheduleCloseOutline,
	} {
		icon, ok := build(fill)
		if !ok {
			t.Fatalf("%s 官方 path 解析失败", name)
		}
		if icon.ViewBox != 16 || icon.Stroke != 1 || len(icon.Commands) == 0 {
			t.Errorf("%s 几何不符：viewBox=%v stroke=%v commands=%d", name, icon.ViewBox, icon.Stroke, len(icon.Commands))
		}
	}
	ellipsis, ok := ScheduleEllipsisOutline(fill)
	if !ok {
		t.Fatal("ellipsis 官方 path 解析失败")
	}
	if ellipsis.ViewBox != 16 || ellipsis.Stroke != 0 || len(ellipsis.Layers) != 3 {
		t.Fatalf("ellipsis 必须是 16 viewBox 的三枚填充圆点：%+v", ellipsis)
	}
}
