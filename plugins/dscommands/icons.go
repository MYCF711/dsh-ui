package dscommands

import (
	"strings"

	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 compact-outline 的两条 path，逐字复用
// assets/official/icons/sidebar/compact.jsx:1-2；第一条官方 opacity=0.35。
// ui-commands/src/client/presentation.ts:45 把「压缩」命令绑定到同一符号。
const (
	compactBasePath = `M8 14.5C11.5899 14.5 14.5 11.5899 14.5 8C14.5 4.41015 11.5899 1.5 8 1.5C4.41015 1.5 1.5 4.41015 1.5 8C1.5 11.5899 4.41015 14.5 8 14.5Z`
	compactArcPath  = `M8 1.5C8.85359 1.5 9.69883 1.66813 10.4874 1.99478C11.2761 2.32144 11.9926 2.80022 12.5962 3.40381C13.1998 4.00739 13.6786 4.72394 14.0052 5.51256C14.3319 6.30117 14.5 7.14641 14.5 8`
)

// drawCommandIcon 只画有官方符号可对应的命令；没有官方映射的行保持无图标，
// 不拿自造字形补位。显示框 14×14 的来源见 geometry.go。
func drawCommandIcon(f ui.C, name string, color gfx.Color) {
	switch {
	case name == "新建会话":
		// 官方 IconNewChatOutlineMedium：assets/official/icons/sidebar/new-chat.jsx:1-3。
		if icon, ok := dsassets.SidebarNewChatMedium(color); ok {
			ui.Icon(f, icon).Size(commandIconSize, commandIconSize).Shrink(0).Color(color)
		}
	case name == "压缩上下文":
		base, baseOK := dsassets.ParsePath(compactBasePath)
		arc, arcOK := dsassets.ParsePath(compactArcPath)
		if !baseOK || !arcOK {
			return
		}
		faint := color
		faint.Alpha *= 0.35 // compact.jsx:1 的官方 opacity，不是估出来的透明度。
		ui.Row(f).Size(commandIconSize, commandIconSize).Shrink(0).Children(func() {
			ui.Icon(f, gfx.Icon{ViewBox: 16, Stroke: 1, Commands: base}).Absolute().Inset(0).Color(faint)
			ui.Icon(f, gfx.Icon{ViewBox: 16, Stroke: 1, Commands: arc}).Absolute().Inset(0).Color(color)
		})
	case strings.HasPrefix(name, "权限："):
		// 官方 HOST_FACES：permission → PermissionIconFullAccessRegular
		// （ui-commands/src/client/presentation.ts:46）；图元原文见
		// assets/official/icons/composer/permission-full-access.jsx:1-3。
		if icon, ok := dsassets.Build(dsassets.PermissionFullAccess, color); ok {
			ui.Icon(f, icon).Size(commandIconSize, commandIconSize).Shrink(0).Color(color)
		}
	}
}
