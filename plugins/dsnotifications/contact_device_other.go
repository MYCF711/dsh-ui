//go:build !windows

// contact_device_other.go —— 非 Windows 的 best-effort 实现（OUT_OF_SUPPORT）。
//
// 本仓支持范围只有 Windows（AGENTS.md「只做 Windows 稳定版，Linux 不写支持」）；
// 这个文件存在只为保证 `go build ./...` 在别的平台能过、装配不缺符号。
//
// 它只交每台机器都确定的两项（platform / app_arch，与官方一致的位置与顺序），
// os / cpu / memory_gib 一律省略——官方 `collect` 的语义就是「读不到就不写」，
// 在这里编一个 Linux 内核版本冒充 `getSystemVersion()` 反而是自创。
package dsnotifications

import "strings"

// readContactDeviceInfo 见 contact_device.go 的官方形状。
func readContactDeviceInfo() string {
	return strings.Join([]string{
		"platform=" + contactPlatformName(),
		"app_arch=" + contactArchName(),
	}, "; ")
}
