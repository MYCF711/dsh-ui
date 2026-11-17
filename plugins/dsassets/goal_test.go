package dsassets

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestGoalOutlineUsesOfficialSprite(t *testing.T) {
	icon, ok := GoalOutline()
	if !ok {
		t.Fatal("官方 goal-outline 的四条 path 必须可由统一素材解析器构建")
	}
	if icon.ViewBox != 16 || icon.Stroke != 1 || icon.Cap != gfx.CapButt || icon.Join != gfx.JoinMiter {
		t.Fatalf("goal-outline 几何 = %+v，want viewBox=16/stroke=1/butt/miter", icon)
	}
	if len(icon.Commands) != 34 || len(icon.Layers) != 0 {
		t.Fatalf("goal-outline 命令/填充层 = %d/%d，want 34/0（官方四条 path）", len(icon.Commands), len(icon.Layers))
	}
	first := icon.Commands[0]
	if first.Kind != gfx.PathCommandMove || first.Point.X != 14.5001 || first.Point.Y != 8 {
		t.Fatalf("首命令 = %+v，want 官方 M14.5001 8", first)
	}
}
