//go:build !windows

package audio

// capture_other.go —— 非 Windows 的 best-effort 桩。
//
// 本仓的支持面只有 Windows 稳定版（`DSH-GO/AGENTS.md` §0「只做 Windows 稳定版，
// Linux 不写支持」）；这一份只保证别的平台上**能编译**，并在运行时结构化地
// 说"没有这条能力"，而不是返回静音 PCM 冒充成功。
//
// 界面据此隐藏麦克风入口（`Actions.VoiceStart == nil`），与官方在
// `navigator.mediaDevices` 不存在时抛 `RecordingError("unavailable")`
// （`client.js:1528-1529`）表达的是同一件事。

import "fmt"

// Devices 在非 Windows 上永远没有设备。
func Devices() ([]Device, error) { return nil, ErrNoDevice }

// Open 在非 Windows 上永远失败。
func Open(int) (Recorder, error) {
	return nil, fmt.Errorf("%w: 仅 Windows 有 winmm 采集实现", ErrUnsupported)
}
