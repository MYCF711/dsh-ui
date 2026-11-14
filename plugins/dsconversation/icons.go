package dsconversation

// icons.go —— 引用 chip 的图标**只有一份**，在 internal/reficon。
//
// 为什么抽出去：composer 的 `@` 候选菜单要画同一组图标（官方
// `MenuView.tsx:182` 用 `ReferenceIconRegular`），两处各写一份就是两份
// 会漂移的转写。这里只做 refKind → reficon.Kind 的映射。
//
// 取值一一对应：refFile=0 / refFolder=1 / refSession=2，与 reficon.File/Folder/Session 同序。
// 来源、viewBox、stroke、sha256 登记见 docs/official-icon-sprite-register.md。

import (
	"github.com/MYCF711/dsh-ui/internal/reficon"
	"github.com/vivaui/viva/gfx"
)

func referenceIcon(kind refKind, color gfx.Color) gfx.Icon {
	return reficon.Icon(reficon.Kind(kind), color)
}
