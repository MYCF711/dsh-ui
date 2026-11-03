package dscomposer

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// 官方 composer 的输入区有**两档高度**，差 16px：
//
//	.GRgSLq_input{min-height:36px;padding:4px 8px 0 14px}          ← 有会话时
//	.GRgSLq_hero .GRgSLq_input{min-height:52px}                    ← 首屏（还没有会话）
//
// 第二条是后代选择器，只挂在 `.GRgSLq_hero` 上，**不是**"后面的规则覆盖前面的"。
// 这两条一起决定卡片总高：
//
//	有会话 8 + 36 + 12 + (2+34+6) = 98
//	首屏   8 + 52 + 12 + (2+34+6) = 114
//
// 常驻一档都会错 16px——所以这里两种状态各断言一次。
func TestCardHeightFollowsHeroAndActiveStates(t *testing.T) {
	cases := []struct {
		name string
		data pluginapi.Snapshot
		want float32
	}{
		{"首屏（没有会话）", pluginapi.Snapshot{}, 114},
		{"有会话", pluginapi.Snapshot{Cur: "s-001", Sessions: nil}, 98},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Plugin{}
			canvas := renderComposer(t, p, tc.data, pluginapi.Actions{}, 1624, 1904, "")
			card, ok := largestSurface(canvas, p.theme.InputMajor)
			if !ok {
				t.Fatal("找不到 composer 卡片（InputMajor 圆角面板）")
			}
			if !closeEnough(card.Bounds.Height, tc.want) {
				t.Fatalf("卡片高 = %.1f，官方 %.0f（%s）", card.Bounds.Height, tc.want, tc.name)
			}
			// 宽度不受状态影响：两档都是 clamp(680, 列宽*.64, 920)+32
			if !closeEnough(card.Bounds.Width, 952) {
				t.Fatalf("卡片宽 = %.1f，官方 952", card.Bounds.Width)
			}
		})
	}
}
