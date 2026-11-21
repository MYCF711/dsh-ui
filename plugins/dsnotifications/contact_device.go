// contact_device.go —— 官方桌面端 `readDeviceInfo()` 的等价物的**平台无关那一半**。
//
// 官方形状（Electron 主进程，逐行核对 `Desktop/main.js` 的 `readDeviceInfo`）：
//
//	function readDeviceInfo() {
//	  const fields = [`platform=${process.platform}`]
//	  collect(fields, "os", () => process.getSystemVersion())
//	  fields.push(`app_arch=${process.arch}`)
//	  collect(fields, "cpu", () => cpus()[0]?.model)
//	  collect(fields, "memory_gib", () => (totalmem() / 1024 ** 3).toFixed(1))
//	  return fields.join("; ")
//	}
//
// 语义要点（注释原文）：「Fields are `name=value` pairs separated by `; `, in
// platform, os, app_arch, cpu and memory_gib order; memory is total physical memory
// in GiB with one decimal.」——**读不到的字段直接省略**（`collect` 的 catch 分支），
// `platform` 与 `app_arch` 是每帧都在的必填项。
//
// 本仓把两半分开：`platform` / `app_arch` 只是 runtime 常量（本文件，可跨平台单测），
// 其余三项按平台在 `contact_device_*.go` 里读真实系统事实。
package dsnotifications

import "runtime"

// contactPlatformName 是 `process.platform` 的对应值（不是 Go 的 GOOS 字面量：
// Electron 在 Windows 上回 `win32`，Go 回 `windows`）。
func contactPlatformName() string {
	switch runtime.GOOS {
	case "windows":
		return "win32"
	case "darwin", "linux", "freebsd", "openbsd", "netbsd":
		return runtime.GOOS
	default:
		return runtime.GOOS
	}
}

// contactArchName 是 `process.arch` 的对应值（Go 的 `amd64` 在 Electron 叫 `x64`，
// `386` 叫 `ia32`）。
func contactArchName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return runtime.GOARCH
	}
}
