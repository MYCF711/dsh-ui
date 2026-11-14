package dsassets

// 本文件是**出处**：每条素材都能回答"它从哪个包、哪个文件、哪个符号来"。
//
// 为什么连 sha256 一起记：`docs/official-assets.md` 里的引用如果是"某个版本的
// 某个符号"，半年后没人能复现当时取到的是哪一串。文件哈希把"当时"钉死——
// 官方包一升级，`golden_test.go` 会先发现文件对不上，再去比 `d`。

// Snapshot 指向本机安装态官方包的位置。它只是**核对用**的路径，
// 素材本身已经以原文形式存在 `paths.go` 里，运行时不需要这些文件。
const (
	// AsarPath 是官方 DSH 桌面端的 asar（只读；本仓任何代码都不写它）。
	AsarPath = "D:/DeepSeek Harness/Desktop/resources/app.asar"
	// ExtractedDir 是解包目录：`tmp/official/`（gitignored）。
	// 干净的检出里没有它，相关断言会跳过——见 golden_test.go 的说明。
	ExtractedDir = "tmp/official"
)

// Source 是一条素材的出处。
type Source struct {
	// Package 是 npm 包名，例如 "@deepseek-ai/dsh-client-ui-conversation"。
	Package string
	// File 是包内文件路径，例如 "lib/client.js"。
	File string
	// Symbol 是官方导出的符号名（组件名）。行内绘制的图元没有导出符号，
	// 这时写它在源码里的位置描述，例如 "InputBar 主按钮内联 <path>"。
	Symbol string
	// SHA256 是 File 的整文件 sha256，用来确认"当时取到的是这一份"。
	SHA256 string
}

// String 返回 `包/文件#符号`，日志和文档里用同一串。
func (s Source) String() string {
	return s.Package + "/" + s.File + "#" + s.Symbol
}
