package reficon

// reficon.go —— 官方 `ReferenceIconRegular` 的三枚图标（file / folder / session）。
//
// 来源逐字：rc.2 dist `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`
// 里 `ReferenceIconRegular`（`_b`）转交的 `yb`——
//
//	session → eZ  （3 条 path）
//	file    → rZ  （2 条描边 path + 1 条填充体）
//	folder  → vb  （2 条 path）
//
// 几何：viewBox 16、regular 权重 stroke-width=1（`_b` 里写死 1）、显示尺寸 14
//（`ReferenceChip.tsx` 传 `size={14}`）。官方 sprite 没写 linecap/linejoin，
// 所以 Cap/Join 留零值，**不要**改成 Round（与 `official-icon-sprite-register.md`
// 的三条先例同一条规矩）。
//
// 这三枚不是"照 sprite 表抄的"——它们不在 `dsh-client-ui-primitives` 的 sprite
// 表里，而是 bundle 内联组件；转写后在 `docs/official-icon-sprite-register.md`
// 追加一行登记（来源仍是同一份 rc.2 dist）。

import (
	"github.com/MYCF711/dsh-ui/plugins/dsbrand"
	"github.com/vivaui/viva/gfx"
)

// Kind 是官方 `ReferenceIconKind` 的三种 appearance。
type Kind int

const (
	File Kind = iota
	Folder
	Session
)

// referenceIconPath 是官方三枚图标的原始 path 组。
type referenceIconPath struct {
	strokes []string
	fills   []string
}

const (
	// 官方 eZ（appearance="session"）：三条描边路径。
	sessionIconStroke1 = "M5 6.75H11"
	sessionIconStroke2 = "M5 9H8"
	sessionIconStroke3 = "M2.37067 11.2497C1.5872 9.89252 1.32042 8.29798 1.61945 6.7597C1.91847 5.22141 2.76317 3.84293 3.99801 2.87809C5.23285 1.91325 6.7747 1.427 8.33964 1.50888C9.90458 1.59076 11.3873 2.23526 12.5147 3.32369C13.6422 4.41232 14.3384 5.8717 14.4751 7.43304C14.6118 8.99438 14.1797 10.5525 13.2585 11.8205C12.3372 13.0885 10.9889 13.9809 9.4617 14.3334C8.18666 14.6277 6.8587 14.529 5.64964 14.0601C5.17095 13.8745 4.76937 13.4929 4.26509 13.3963C3.67389 13.2832 2.95232 13.5595 2.0377 14.3334"

	// 官方 rZ（appearance="file"）：两条描边 + 一条填充体。
	fileIconStroke1 = "M4.9375 5.90295H11.0625"
	fileIconStroke2 = "M4.9375 9.02991H8.27841"
	fileIconFill1   = "M12.5 1.32617C13.3039 1.32617 14 1.95171 14 2.77637V13.2246C13.9996 14.0489 13.3036 14.6738 12.5 14.6738H3.5C2.69637 14.6738 2.00042 14.0489 2 13.2246V2.77637C2 1.95171 2.69613 1.32617 3.5 1.32617H12.5ZM3.5 2.32617C3.1993 2.32617 3 2.55186 3 2.77637V13.2246C3.00044 13.4489 3.19963 13.6738 3.5 13.6738H12.5C12.8004 13.6738 12.9996 13.4489 13 13.2246V2.77637C13 2.55186 12.8007 2.32617 12.5 2.32617H3.5Z"

	// 官方 vb（appearance="folder"）：两条描边。
	folderIconStroke1 = "M1.50439 3.11059C1.50439 2.55831 1.95211 2.1106 2.50439 2.1106H5.43389C5.67773 2.1106 5.91318 2.19969 6.09593 2.36113L7.71649 3.79265C7.89924 3.95409 8.1347 4.04319 8.3785 4.04319H13.4958C14.0481 4.04319 14.4958 4.4909 14.4958 5.04319V12.8894C14.4958 13.4417 14.0481 13.8894 13.4958 13.8894H2.50439C1.95211 13.8894 1.50439 13.4417 1.50439 12.8894V4.04319V3.11059Z"
	folderIconStroke2 = "M3.63501 7.66614H12.3647"
)

// referenceIcon 造一枚官方引号图标。color 只用于 file 的填充体；
// 描边始终是 currentColor 等价的 `Color` 传给 `ui.Icon` 的那一层。
func Icon(kind Kind, color gfx.Color) gfx.Icon {
	icon := gfx.Icon{ViewBox: 16, Stroke: 1}
	var paths referenceIconPath
	switch kind {
	case Folder:
		paths = referenceIconPath{strokes: []string{folderIconStroke1, folderIconStroke2}}
	case Session:
		paths = referenceIconPath{strokes: []string{sessionIconStroke1, sessionIconStroke2, sessionIconStroke3}}
	default:
		paths = referenceIconPath{strokes: []string{fileIconStroke1, fileIconStroke2}, fills: []string{fileIconFill1}}
	}
	for _, d := range paths.strokes {
		if cmds, ok := dsbrand.ParseSVGPath(d); ok {
			icon.Commands = append(icon.Commands, cmds...)
		}
	}
	for _, d := range paths.fills {
		if cmds, ok := dsbrand.ParseSVGPath(d); ok {
			icon.Layers = append(icon.Layers, gfx.IconLayer{Fill: color, Commands: cmds})
		}
	}
	return icon
}
