//go:build windows

// contact_device_windows.go —— 官方 `readDeviceInfo()` 在 Windows 上的三项系统事实。
//
// 每一项都走进程内系统调用 / 注册表，**不起子进程、不读用户数据**：
//
//	os          RtlGetVersion（ntdll 文档：永不失败）→ "10.0.22631"，与
//	            Electron `process.getSystemVersion()` 同一来源、同一格式
//	cpu         注册表 HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0 的
//	            ProcessorNameString —— Node `os.cpus()[0].model` 在 Windows 读的就是它
//	memory_gib  GlobalMemoryStatusEx 的 TotalPhys ÷ 1024³，保留一位小数
//	            （官方 `(totalmem() / 1024 ** 3).toFixed(1)`）
//
// 读失败一律省略该字段（官方 `collect` 的 catch 分支同语义），绝不填占位值。
package dsnotifications

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// readContactDeviceInfo 见 contact_device.go 的官方形状；本文件只提供 Windows 取值。
func readContactDeviceInfo() string {
	fields := []string{"platform=" + contactPlatformName()}
	if version, ok := windowsSystemVersion(); ok {
		fields = append(fields, "os="+version)
	}
	fields = append(fields, "app_arch="+contactArchName())
	if model, ok := windowsCPUModel(); ok {
		fields = append(fields, "cpu="+model)
	}
	if memory, ok := windowsMemoryGiB(); ok {
		fields = append(fields, "memory_gib="+memory)
	}
	return strings.Join(fields, "; ")
}

// windowsSystemVersion 回 `process.getSystemVersion()` 同形的 "major.minor.build"。
func windowsSystemVersion() (string, bool) {
	info := windows.RtlGetVersion()
	if info == nil {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d", info.MajorVersion, info.MinorVersion, info.BuildNumber), true
}

// windowsCPUModel 读注册表里的处理器型号（Node 的 `os.cpus()[0].model` 同一来源）。
func windowsCPUModel() (string, bool) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	model, _, err := key.GetStringValue("ProcessorNameString")
	if err != nil {
		return "", false
	}
	// 注册表里的值常带尾部空格；Node 侧也 trim（`os.cpus()` 的 model 是干净串）。
	model = strings.TrimSpace(model)
	if model == "" {
		return "", false
	}
	return model, true
}

// memoryStatusEx 是 MEMORYSTATUSEX（winbase.h）。字段顺序与宽度必须逐字一致：
// 两个 uint32 之后全是 8 字节成员，所以在 Go 的 4/8 字节对齐下偏移都与 Win32 相同。
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// windowsMemoryGiB 回总物理内存的 GiB 值，一位小数（官方 `.toFixed(1)`）。
func windowsMemoryGiB() (string, bool) {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	if err := proc.Find(); err != nil {
		return "", false
	}
	status := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return "", false
	}
	return fmt.Sprintf("%.1f", float64(status.TotalPhys)/(1024*1024*1024)), true
}
