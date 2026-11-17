package dsjobs

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// TestOfficialVisualGeometry 钉住本轮从 rc.2 CSS 原件转写的数值。
// want 全部直接来自官方文件，不是从实现反推的 golden。
func TestOfficialVisualGeometry(t *testing.T) {
	tests := []struct {
		name string
		got  float32
		want float32
		src  string
	}{
		{"触发器高度", jobsTriggerMinHeight, 28, "JobListAction.module.css:5"},
		{"触发器图标", jobsActivitySlot, 14, "JobListAction.tsx:588"},
		{"compact 断点", jobsTriggerCompactBreakpoint, 460, "JobListAction.module.css:39"},
		{"compact 图标", jobsTriggerCompactSlot, 15, "JobListAction.module.css:61-65"},
		{"菜单宽", jobsMenuWidth, 500, "JobListAction.module.css:90"},
		{"菜单最大高", jobsMenuMaxHeight, 480, "JobListAction.module.css:92"},
		{"菜单视口扣减", jobsMenuViewportInset, 140, "JobListAction.module.css:92"},
		{"菜单右缘", jobsMenuRight, 12, "JobListAction.module.css:102"},
		{"段头下 margin", jobsSectionMarginBottom, 1, "JobListAction.module.css:127"},
		{"段头顶线", jobsSectionBorder, 0.5, "JobListAction.module.css:129"},
		{"行最小高", jobsRowMinHeight, 28, "JobListAction.module.css:317"},
		{"行纵 padding", jobsRowPadV, 4, "JobListAction.module.css:318"},
		{"行横 padding", jobsRowPadH, 7, "JobListAction.module.css:318"},
		{"行字号", jobsRowFont, 12, "JobListAction.module.css:323"},
		{"行行高", jobsRowLine, 17, "JobListAction.module.css:324"},
		{"箭头槽", jobsChevronBox, 20, "JobListAction.module.css:242-243"},
		{"箭头图标", jobsChevronIcon, 12, "JobListAction.tsx:222"},
		{"控制描边", jobStopBorder, 0.5, "JobListAction.module.css:244,259"},
		{"停止钮", jobStopBox, 20, "JobListAction.module.css:256-257"},
		{"确认横 padding", jobStopArmPadH, 7, "JobListAction.module.css:284"},
		{"确认描边混合", jobStopArmBorderMix, 0.45, "JobListAction.module.css:285"},
		{"确认底色混合", jobStopArmBgMix, 0.12, "JobListAction.module.css:286"},
		{"详情最大高", jobOutputMaxH, 288, "JobListAction.module.css:437"},
		{"详情 gutter", jobOutputIndent, 12, "JobListAction.module.css:445"},
		{"详情下 padding", jobOutputPadBot, 8, "JobListAction.module.css:431"},
		{"通知下 margin", jobNoticeMarginB, 6, "JobListAction.module.css:449"},
		{"StateDot 外框", jobDotSize, 10, "index-ghOpC9uV.js:129"},
		{"StateDot 内核", jobDotCoreSize, 6, "index-CSAdOBx6.css:1 inset:20%"},
		{"StateDot ongoing", jobDotOngoingSize, 14, "index-ghOpC9uV.js:129"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("几何 = %v，官方值 %v（%s）", tc.got, tc.want, tc.src)
			}
		})
	}
}

func TestOfficialStateColorsAndVectorAssets(t *testing.T) {
	h := newHarness(t, nil, nil)
	if got := dotColor(h.theme, pluginapi.Job{}); got != h.theme.LabelTertiary {
		t.Fatalf("ongoing 颜色 = %v，应为 label-tertiary %v", got, h.theme.LabelTertiary)
	}
	if got := dotColor(h.theme, pluginapi.Job{Done: true}); got != h.theme.SuccessPrimary {
		t.Fatalf("done 颜色 = %v，应为 state-success-primary %v", got, h.theme.SuccessPrimary)
	}
	if got := dotColor(h.theme, pluginapi.Job{Done: true, ExitCode: 1}); got != h.theme.Error {
		t.Fatalf("error 颜色 = %v，应为 state-error-primary %v", got, h.theme.Error)
	}
	for name, icon := range map[string]gfx.Icon{
		"layers": layersIcon(), "stop": stopIcon(),
		"down": jobChevronDownIcon(), "right": jobChevronRightIcon(), "up": jobChevronUpIcon(),
	} {
		if icon.ViewBox != 16 || len(icon.Commands) == 0 {
			t.Fatalf("%s 不是官方 16px sprite：%+v", name, icon)
		}
	}
	spinner := jobStateDotOngoingIcon(h.theme.LabelTertiary)
	if spinner.ViewBox != 24 || spinner.Stroke != 2 || spinner.Cap != gfx.CapRound ||
		len(spinner.Commands) == 0 || len(spinner.Layers) != 1 {
		t.Fatalf("StateDot ongoing 几何不符：%+v", spinner)
	}
}

func TestOfficialCompactTriggerUsesOnlyLayersIcon(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{{ID: "job-1", SessionID: "s-1", Command: "npm test"}}, nil)
	h.env.Bounds = gfx.Rect{Width: 440, Height: 832}
	h.drawOfficial(t, 440, 832)
	if h.has("1 个后台任务") {
		t.Fatalf("compact 触发器仍画计数：%v", h.texts())
	}
	h.env.Bounds.Width = 1182
	h.drawOfficial(t, 1182, 832)
	if !h.has("1 个后台任务") {
		t.Fatalf("非 compact 触发器应画计数：%v", h.texts())
	}
}

func TestOfficialLiveRowDoesNotInventDetail(t *testing.T) {
	h := newHarness(t, []pluginapi.Job{{ID: "job-1", SessionID: "s-1", Command: "npm test"}}, nil)
	h.p.setOpen(true)
	h.drawOfficial(t, 1182, 832)
	if h.has(zhStatusRunning) {
		t.Fatalf("宿主没有 progress/detail 时不应拿 running 状态补 secondary：%v", h.texts())
	}
}
