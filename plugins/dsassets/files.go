package dsassets

import (
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/gfx"
)

// 文件面板与附件卡的官方素材。
//
// FileTypeIcon 的底板、折角与标记来自 rc.2
// dsh-web-frontend/dist/assets/index-ghOpC9uV.js:134（_Z/S7/$Z/or）；
// 类型色来自同包 index-CSAdOBx6.css:1 的 ._code_1wejo_8、
// ._image_1wejo_24、._markdown_1wejo_28、._other_1wejo_32。
// chevron/right-up 来自同一 sprite 文件 :129，regular stroke=1。

// FileTypeAsset 是 FileTypeIcon 的可绘制投影。Sheet 是彩色底板与填充标记；
// 需要描边的标记单独放在 Mark，避免 gfx.Icon 的 Layers 把描边盖住。
type FileTypeAsset struct {
	Sheet gfx.Icon
	Mark  *gfx.Icon
}

var (
	fileTypeSheetBlue   = gfx.RGB24(0x4176e6) // --dsw-static-deepseek-500
	fileTypeSheetViolet = gfx.RGB24(0x8b76f6) // --dsh-file-type-violet
	fileTypeSheetOther  = gfx.RGB24(0xcfd3d6) // --dsw-static-neutral-bluish-300
	fileTypeMarkWhite   = gfx.RGB24(0xffffff) // --dsw-static-neutral-00
	fileTypeFoldMuted   = gfx.RGB24(0xa2a4a6) // --dsw-static-neutral-400
)

// FileType 返回官方 FileTypeIcon(path) 的素材。代码、图片、Markdown 与其他
// 类型分别使用官方 code/image/markdown/other 分支；尚未接入的传统 Office
// 分支回退到官方 other，不用扩展名文字或手画图形补位。
func FileType(name string) (FileTypeAsset, bool) {
	switch classifyFileType(name) {
	case "code":
		sheet, ok := fileTypeSheet(fileTypeSheetBlue, nil, 0)
		if !ok {
			return FileTypeAsset{}, false
		}
		mark, ok := fileTypeCodeMark()
		return FileTypeAsset{Sheet: sheet, Mark: &mark}, ok
	case "image":
		marks := make([]gfx.IconLayer, 0, 3)
		for _, d := range []string{
			`M10.4212 15.9204C10.5756 15.6558 10.9579 15.6558 11.1123 15.9204L13.6493 20.2696C13.8048 20.5362 13.6125 20.8711 13.3037 20.8711H8.22974C7.92102 20.8711 7.72868 20.5362 7.88423 20.2696L10.4212 15.9204Z`,
			`M15.4981 13.186C15.6505 12.9117 16.0451 12.9117 16.1975 13.186L20.1368 20.2769C20.2849 20.5435 20.0922 20.8711 19.7872 20.8711H11.9084C11.6034 20.8711 11.4107 20.5435 11.5588 20.2769L15.4981 13.186Z`,
			`M11.8603 11.3997C11.8603 12.286 11.1418 13.0045 10.2555 13.0045C9.36924 13.0045 8.65076 12.286 8.65076 11.3997C8.65076 10.5134 9.36924 9.79492 10.2555 9.79492C11.1418 9.79492 11.8603 10.5134 11.8603 11.3997Z`,
		} {
			commands, ok := ParsePath(d)
			if !ok {
				return FileTypeAsset{}, false
			}
			for i := range commands {
				transformFileTypeMark(&commands[i], 16, 1.12)
			}
			marks = append(marks, gfx.IconLayer{Fill: fileTypeMarkWhite, Commands: commands})
		}
		sheet, ok := fileTypeSheet(fileTypeSheetViolet, marks, 0.7)
		return FileTypeAsset{Sheet: sheet}, ok
	case "markdown":
		sheet, ok := FileTypeMarkdown(fileTypeSheetBlue, fileTypeMarkWhite)
		return FileTypeAsset{Sheet: sheet}, ok
	default:
		sheet, ok := fileTypeSheet(fileTypeSheetOther, nil, 0)
		return FileTypeAsset{Sheet: sheet}, ok
	}
}

// fileTypeSheet 构造官方 or 组件的 currentColor 底板与折角。muted 使用
// neutral-400；其余分支使用 neutral-00 的 0.7 alpha。
func fileTypeSheet(base gfx.Color, marks []gfx.IconLayer, foldAlpha float32) (gfx.Icon, bool) {
	basePath, ok := ParsePath(fileTypeBasePath)
	if !ok {
		return gfx.Icon{}, false
	}
	foldPath, ok := ParsePath(fileTypeFoldPath)
	if !ok {
		return gfx.Icon{}, false
	}
	fold := fileTypeFoldMuted
	if foldAlpha > 0 {
		fold = fileTypeMarkWhite
		fold.Alpha *= foldAlpha
	}
	layers := []gfx.IconLayer{{Fill: base, Commands: basePath}, {Fill: fold, Commands: foldPath}}
	layers = append(layers, marks...)
	return gfx.Icon{ViewBox: 28, Layers: layers}, true
}

// fileTypeCodeMark 是 $Z 的 code 分支三条 path，strokeWidth=1.35；
// 官方这个分支不传 markTransform，因此坐标保持原值。
func fileTypeCodeMark() (gfx.Icon, bool) {
	var commands []gfx.PathCommand
	for _, d := range []string{
		`M10.0053 13.126L7.0236 16.3788C6.96052 16.4476 6.96052 16.5532 7.0236 16.622L10.0053 19.8748`,
		`M17.9941 13.126L20.9759 16.3788C21.039 16.4476 21.039 16.5532 20.9759 16.622L17.9941 19.8748`,
		`M15.2652 12.957L12.7344 20.0433`,
	} {
		parsed, ok := ParsePath(d)
		if !ok {
			return gfx.Icon{}, false
		}
		commands = append(commands, parsed...)
	}
	return gfx.Icon{ViewBox: 28, Stroke: 1.35, Commands: commands}, true
}

// transformFileTypeMark 执行官方 translate(c c) scale(s) translate(-c -c)。
func transformFileTypeMark(command *gfx.PathCommand, center, scale float32) {
	if command.Kind == gfx.PathCommandClose {
		return
	}
	command.Point = scaleFileTypePoint(command.Point, center, scale)
	command.Control1 = scaleFileTypePoint(command.Control1, center, scale)
	command.Control2 = scaleFileTypePoint(command.Control2, center, scale)
}

func scaleFileTypePoint(point gfx.Point, center, scale float32) gfx.Point {
	return gfx.Point{X: center + (point.X-center)*scale, Y: center + (point.Y-center)*scale}
}

// classifyFileType 是官方 classifyFileType 的当前已接线子集。代码语言在官方
// 另有语言徽标；这里统一复用官方 code 分支，不把语言缩写字母当图标。
func classifyFileType(name string) string {
	base := strings.ToLower(filepath.Base(strings.ReplaceAll(name, `\`, "/")))
	if base == "readme" || base == "changelog" || base == "contributing" {
		return "markdown"
	}
	ext := strings.TrimPrefix(filepath.Ext(base), ".")
	switch ext {
	case "md", "mdx", "markdown":
		return "markdown"
	case "png", "jpg", "jpeg", "gif", "svg", "webp", "avif", "bmp", "ico", "tif", "tiff", "heic", "heif":
		return "image"
	case "scss", "sass", "less", "vue", "svelte", "astro", "bat", "cmd",
		"bash", "sh", "c", "h", "cc", "cpp", "cxx", "hpp", "cs", "css",
		"dart", "go", "graphql", "gql", "java", "js", "jsx", "json", "json5", "jsonc",
		"kt", "kts", "lua", "php", "py", "rb", "rs", "sql", "swift", "toml",
		"ts", "tsx", "wasm", "xml", "yaml", "yml", "zig":
		return "code"
	default:
		return "other"
	}
}

// FileChevron 复用 IconChevronLeft/RightOutlineRegular（AttachmentRail.tsx:145,166）。
// sprite 默认 size=14、viewBox=16、regular stroke=1。
func FileChevron(left bool) (gfx.Icon, bool) {
	d := `M6 12L9.29289 8.70711C9.68342 8.31658 9.68342 7.68342 9.29289 7.29289L6 4`
	if left {
		d = `M10 4L6.70711 7.29289C6.31658 7.68342 6.31658 8.31658 6.70711 8.70711L10 12`
	}
	commands, ok := ParsePath(d)
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}

// FileRightUp 复用 OpenTargetButton 的 IconRightUpOutlineRegular（:75,112），
// 默认 16 viewBox；调用方按官方 compact 档显示为 13。
func FileRightUp(fill gfx.Color) (gfx.Icon, bool) {
	return Build(Asset{ViewBox: 16, Size: 16, StrokeWidth: 1, Elements: []Element{
		{Kind: PathElement, D: `M11.7256 2.77441C12.5538 2.77469 13.2256 3.44616 13.2256 4.27441V10.1416H12.2256V4.27441C12.2256 3.99844 12.0015 3.77469 11.7256 3.77441H5.7207V2.77441H11.7256Z`, Fill: true},
		{Kind: PathElement, D: `M2.77441 13.2255L12.3756 3.62427`, Stroke: true},
	}}, fill)
}
