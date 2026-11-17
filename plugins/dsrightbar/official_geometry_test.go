package dsrightbar

import "testing"

// 官方几何锚点见各常量注释；这里防止后续把值改回自造档位。
func TestRightbarOfficialGeometry(t *testing.T) {
	if rightbarAutoFullscreenWidth != 768 || rightbarChromeIconSize != 15 {
		t.Fatalf("右栏断点/图标 = %v/%v，官方 768/15", rightbarAutoFullscreenWidth, rightbarChromeIconSize)
	}
	if ledgerTurnHeight != 30 || ledgerCellHeight != 30 || ledgerSummaryHeight != 20 || ledgerGroupHeight != 36 {
		t.Fatalf("账本行高 = turn %v cell %v summary %v group %v", ledgerTurnHeight, ledgerCellHeight, ledgerSummaryHeight, ledgerGroupHeight)
	}
	if ledgerGroupPadX != 20 || ledgerRowGap != 0 {
		t.Fatalf("账本间距 = pad %v gap %v，官方 20/0", ledgerGroupPadX, ledgerRowGap)
	}
	for name, icon := range map[string]struct {
		viewBox float32
		ok      bool
	}{
		"collapse":   {rightbarCollapseIcon.ViewBox, len(rightbarCollapseIcon.Commands) > 0},
		"fullscreen": {rightbarFullscreenIcon.ViewBox, len(rightbarFullscreenIcon.Commands) > 0},
		"restore":    {rightbarRestoreIcon.ViewBox, len(rightbarRestoreIcon.Commands) > 0},
	} {
		if icon.viewBox != 16 || !icon.ok {
			t.Errorf("%s 官方图标未正确加载：%+v", name, icon)
		}
	}
}
