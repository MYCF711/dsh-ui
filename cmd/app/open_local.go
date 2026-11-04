package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// open_local.go 是「打开本地路径」这一条链的最后一格。
//
// 为什么单独一个文件：这里有**两种**真实落点，而它们的行为不一样——
// 文件交给 Viva 的 `app.OpenPath`（它会拒绝可执行文件/符号链接，driver 运行
// 还会记录 `LastOpenedPath`），目录交给平台文件管理器（Viva 的 OpenPath
// 只收常规文件）。把它们混在一处写，很容易把"目录"也塞给 OpenPath，
// 于是"在工作目录里打开"变成状态栏里的一条失败。

// openDirectory 是"在文件管理器里打开一个目录"的真实落点。
//
// 换成变量是为了让测试能看见它收到的**精确路径**：Viva v0.2.3 的 driver 只
// 记录 `app.OpenPath` 的常规文件请求，没有目录打开的记录位；而真实运行下
// 它会在桌面弹出文件管理器，测试里不该弹窗。
var openDirectory = platformOpenDirectory

// openLocal 打开一个绝对本地路径，返回它是不是目录。
//
// 目录**不能**交给 `app.OpenPath`：它对目录直接返回
// `app: "…" is not a regular file`（viva app/open_path.go 的 checkOpenPath），
// 于是按钮点下去除了报错什么也不会发生。
func openLocal(path string) (bool, error) {
	// os.Stat 而不是 Lstat：Windows 的 junction/目录符号链接很常见
	// （工作目录本身经常就是链过去的），按 Lstat 会被当成"非常规文件"，
	// 于是落到 app.OpenPath 的拒绝分支上。指向**文件**的链接仍由
	// app.OpenPath 自己按 Lstat 拒绝——那条安全规则不在这里放宽。
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return true, openDirectory(path)
	}
	return false, openPath(path)
}

// platformOpenDirectory 用各平台自带的文件管理器打开目录：
// Windows 资源管理器、macOS Finder、Linux 由 xdg-open 决定。
//
// 只等"启动"不等"退出"：文件管理器的生命周期比一次点击长得多，
// 等它会把界面卡住。Start 失败（找不到程序）才是这次动作的失败。
func platformOpenDirectory(path string) error {
	var name string
	switch runtime.GOOS {
	case "windows":
		name = "explorer.exe"
	case "darwin":
		name = "open"
	default:
		name = "xdg-open"
	}
	command := exec.Command(name, path)
	if err := command.Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	go func() { _ = command.Wait() }()
	return nil
}
