// brand.go —— 产品名、产物名、版本号的**唯一**定义处。
//
// 为什么要有这个文件：窗口标题（`-title` 的默认值）、内核名（`uikernel.New`）、
// 无头截图按标题认窗（`waitOwnWindow` / `captureWindow`）原来各写一份产品名的字面量。
// 改产品名时只要漏掉一处，症状是**看起来像截图坏了**：
// `findOwnWindow` 按标题**包含**匹配，找不到就 `os.Exit(2)`，日志只说
// "没等到窗口"——没人会想到原因是"标题改了、认窗还在找旧名字"。
// 所以这里只有一根常量，四处都引用它。
package main

// productName 是产品名：窗口标题默认值、内核根作用域名、`--version` 的首段。
const productName = "DSH GO GUI"

// binaryName 是构建产物名（不带 .exe）：`scripts/` 里编的是 `dshgogui.exe`，
// 进程表里看到的也是它——出问题时人第一眼认的就是这个名字。
const binaryName = "dshgogui"

// version 是 `--version` 打印的版本号，默认 "dev"。
//
// 为什么不是某条写死的号：本仓没有 CHANGELOG，版本号没有单一来源；
// 编一个看起来像真的号（0.1.0 之类）比写 dev 更糟——它会被当成"这就是发出去的版本"。
// 要号的时候由构建方注入：
//
//	go build -ldflags "-X main.version=0.2.1-go.7" -o tmp/dshgogui.exe ./cmd/app
//
// 所以这里是 var 而不是 const（`-X` 只认变量），"一根常量"的约束管的是
// productName / binaryName 这两个**名称**，不管版本号。
var version = "dev"

// windowTitleNeedle 是按窗口标题认窗时用的串。
//
// 为什么不直接返回 productName：`-title` 可以覆盖窗口标题（并行跑多实例时要靠它
// 在任务管理器里区分），那时窗口上写的是覆盖后的标题，再拿默认名去找就**永远找不到**。
// winTitle 的初值来自 `-title` 的默认值，也就是 productName 本身——
// 默认路径等价于"引用常量"，覆盖路径才不会失配。
func windowTitleNeedle() string {
	if winTitle != "" {
		return winTitle
	}
	return productName
}

// Windows 的 VERSIONINFO 资源（任务管理器的「描述」列）由 `go generate` 生成：
//
//	cd cmd/app && go generate
//
// 生成器从**本文件**读产品名、产物名、版本号，所以资源里写的和运行时用的一定是
// 同一份——这类资源最常见的坏法就是两边各写一份、改一处忘一处。
// 产物 `versioninfo_windows_amd64.syso` 要跟着仓库走（文件名后缀让它只在
// windows/amd64 参与链接，别的平台照常构建）。
//
//go:generate go run ./internal/vsinfo -o versioninfo_windows_amd64.syso
