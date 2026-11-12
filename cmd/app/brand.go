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
//
// 用户口径（2026-11-04）：「exe 要写产品名，而且要全程」。所以它是**全程**
// ——逐字等于命令行里敲的那个名字、分大小写，不是 `DSHGUI` / `dsh-go-gui`
// 这类缩写（官方参照物 `DeepSeek Harness.exe` 就是这么写的）。缩写名的问题
// 不在"不好看"，在**任务管理器里认不出是哪个产品**：进程表只看得到文件名。
const productName = "DSH GUI GO"

// binaryName 是构建产物名，**含扩展名**：`scripts/` 里编的是 `DSH GUI GO.exe`。
//
// 为什么把它和 productName 分成两根而不是一根加 `.exe`：这两件事会分头变
// （换平台、或者哪天要做 `-cli` 变体），合成一根就得在两个方向上都加特判。
// 现在是同一个字符串——**但它是两个事实**：一个给人看，一个是文件名。
// 所以路径里带空格是**故意的**，`scripts/` 与文档里每一处命令行形态都必须加引号。
const binaryName = "DSH GUI GO.exe"

// version 是 `--version` 打印的版本号，默认 "dev"。
//
// 为什么不是某条写死的号：本仓没有 CHANGELOG，版本号没有单一来源；
// 编一个看起来像真的号（0.1.0 之类）比写 dev 更糟——它会被当成"这就是发出去的版本"。
// 要号的时候由构建方注入：
//
//	go build -ldflags "-X main.version=0.2.1-go.7" -o "tmp/DSH GUI GO.exe" ./cmd/app
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
