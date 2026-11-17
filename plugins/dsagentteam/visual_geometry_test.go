package dsagentteam

import (
	"testing"

	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
)

// TestOfficialVisualGeometry 钉住本轮从 rc.2 CSS 原件逐项转写的数值。
// 这些断言的 want 全部直接来自官方文件，不是从实现反推的 golden。
func TestOfficialVisualGeometry(t *testing.T) {
	tests := []struct {
		name string
		got  float32
		want float32
		src  string
	}{
		{"StateDot 外框", dotSize, 10, "index-ghOpC9uV.js:129"},
		{"StateDot 内核", dotCoreSize, 6, "index-CSAdOBx6.css:1 inset:20%"},
		{"StateDot ongoing", dotOngoingSize, 14, "index-ghOpC9uV.js:129"},
		{"目录触发器图标", catalogTriggerIconSize, 14, "SubagentHeaderLineage.tsx:770"},
		{"目录分组图标", catalogSectionIconSize, 14, "SubagentHeaderLineage.tsx:799"},
		{"成员主名称字号", catalogLabelFont, 13, "SubagentHeaderLineage.module.css:358-362"},
		{"成员尾槽", catalogSidebarSize, 28, "SubagentHeaderLineage.module.css:393-400"},
		{"成员尾槽纵向 margin", catalogSidebarMarginV, 4, "SubagentHeaderLineage.module.css:393-400"},
		{"分支箭头", catalogDisclosureIcon, 12, "SubagentHeaderLineage.module.css:329-333"},
		{"子级左 padding", catalogChildPadLeft, 3, "SubagentHeaderLineage.module.css:432-438"},
		{"菜单视口高度扣减", catalogMenuViewportInset, 140, "SubagentHeaderLineage.module.css:159-160"},
		{"目录错误字号", catalogErrorFont, 11, "SubagentHeaderLineage.module.css:478-484"},
		{"目录错误行高", catalogErrorLine, 16, "SubagentHeaderLineage.module.css:478-484"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("几何 = %v，官方值 %v（%s）", tc.got, tc.want, tc.src)
			}
		})
	}
}

func TestStateIdlePrimaryTokens(t *testing.T) {
	// design-platform.css:46,51 定义 neutral-300=#d4d4d4、neutral-600=#545557；
	// :290,442 分别把浅色/深色 state-idle-primary 指到这两档。
	if got, want := dscore.Light().IdlePrimary, gfx.RGB24(0xd4d4d4); got != want {
		t.Fatalf("浅色 idle = %+v，官方值 %+v", got, want)
	}
	if got, want := dscore.Dark().IdlePrimary, gfx.RGB24(0x545557); got != want {
		t.Fatalf("深色 idle = %+v，官方值 %+v", got, want)
	}
}

func TestOfficialVectorAssets(t *testing.T) {
	check := checkOutlineIcon()
	if check.ViewBox != 16 || check.Stroke != 1 || len(check.Commands) != 4 {
		t.Fatalf("check-outline 几何不符：%+v", check)
	}
	right := chevronRightIcon()
	if right.ViewBox != 16 || right.Stroke != 1 || len(right.Commands) != 4 {
		t.Fatalf("chevron-right-outline 几何不符：%+v", right)
	}
	up := chevronUpIcon()
	if up.ViewBox != 16 || up.Stroke != 1 || len(up.Commands) != 4 {
		t.Fatalf("chevron-up-outline 几何不符：%+v", up)
	}
	// 官方 sprite 未声明 linecap/linejoin，必须保持 SVG 默认 butt/miter；
	// 只有 StateDot spinner 的 CSS 明确要求 round。
	for name, icon := range map[string]gfx.Icon{
		"users": usersIcon(), "checklist": checklistIcon(), "down": chevronDownIcon(),
		"right": right, "up": up, "check": check,
	} {
		if icon.Cap != gfx.CapButt || icon.Join != gfx.JoinMiter {
			t.Fatalf("%s 应使用官方默认 butt/miter，实际 cap=%v join=%v", name, icon.Cap, icon.Join)
		}
	}
	spinner := stateDotOngoingIcon(newTestPlugin(nil).theme.LabelTertiary)
	if spinner.ViewBox != stateDotSpinnerViewBox || spinner.Stroke != stateDotSpinnerStroke ||
		len(spinner.Commands) == 0 || len(spinner.Layers) != 1 {
		t.Fatalf("StateDot ongoing 几何不符：%+v", spinner)
	}
}
