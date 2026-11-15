package dstopbar

// icons.go —— 会话头 view tab 行 More 按钮的图标（**不是手画的**）。
//
// # ellipsis-outline（官方 `IconEllipsisOutlineRegular`）
//
// 官方用处：`ViewsMoreMenu` 的锚点按钮
// `<IconEllipsisOutlineRegular size={13}/>`（OFF:dsh-client-ui-conversation/src/client/
// skeleton/ConversationSession.tsx:37-43）。
//
// 取法同 `plugins/dssidebar/icons.go`（rc.2 安装态的图标 sprite 表）：
//
//	dist: /dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-ghOpC9uV.js
//	      整文件 1093231 B，sha256 061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8
//	sprite `"ellipsis-outline"` 起于字节 670780；`svg(1)` 片段 sha256
//	      fe08b1b020c452a8887c22c846ce995fce5f0e54cb2110f25a356886fc1ad1ad
//
// 形状：viewBox `0 0 16 16`，三个**已填充**的圆点（三条 path 都是
// `fill="currentColor"`，没有描边），所以整枚图标走 `Commands`（单色 tint 直接生效，
// 见 `dsassets.Build` 的纯填充分支）。

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// ellipsisDotPaths 是 sprite `"ellipsis-outline"` 的三条 `d` 原文（逐字节）。
var ellipsisDotPaths = [3]string{
	`M3 9C3.55228 9 4 8.55228 4 8C4 7.44772 3.55228 7 3 7C2.44772 7 2 7.44772 2 8C2 8.55228 2.44772 9 3 9Z`,
	`M8 9C8.55228 9 9 8.55228 9 8C9 7.44772 8.55228 7 8 7C7.44772 7 7 7.44772 7 8C7 8.55228 7.44772 9 8 9Z`,
	`M13 9C13.5523 9 14 8.55228 14 8C14 7.44772 13.5523 7 13 7C12.4477 7 12 7.44772 12 8C12 8.55228 12.4477 9 13 9Z`,
}

// ellipsisIcon 造官方 `IconEllipsisOutlineRegular`；颜色由调用方 `.Color()` 给。
//
// 返回 false 表示路径解析失败（与 dsassets 的约定一致：调用方自己决定不画）。
func ellipsisIcon() (gfx.Icon, bool) {
	commands := make([]gfx.PathCommand, 0, 3*9)
	for _, d := range ellipsisDotPaths {
		parsed, ok := dsassets.ParsePath(d)
		if !ok {
			return gfx.Icon{}, false
		}
		commands = append(commands, parsed...)
	}
	return gfx.Icon{ViewBox: 16, Commands: commands}, true
}
