//go:build windows

package audio

// capture_windows_test.go —— 真机采集冒烟。
//
// 这条测试**依赖真实声卡**：没有麦克风（或 CI 是没有音频设备的容器）时
// 直接 Skip，不报红也不假装通过。判据不是"跑过了"，而是"跑的时候
// 系统里真的有录音设备"——`Devices()` 就是那个前提的检查点。

import (
	"errors"
	"testing"
	"time"
)

func TestDevicesReportsRealInputDevices(t *testing.T) {
	devs, err := Devices()
	if errors.Is(err, ErrNoDevice) {
		t.Skip("本机没有录音设备")
	}
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	for _, d := range devs {
		if d.Name == "" {
			t.Fatalf("设备 %d 没有名字", d.ID)
		}
	}
	t.Logf("录音设备 %d 个，首选 %q（原生 %d 声道）", len(devs), devs[0].Name, devs[0].Channels)
}

func TestRecorderCapturesBlocksAsWAV(t *testing.T) {
	if _, err := Devices(); errors.Is(err, ErrNoDevice) {
		t.Skip("本机没有录音设备")
	}
	rec, err := Open(-1)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rec.Close()
	if err := rec.Start(); err != nil {
		t.Skipf("设备打不开（可能被独占）：%v", err)
	}
	// 300 ms 足够出 3 个 block；电平表至少要被刷新过一次。
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	wav, err := rec.Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(wav) <= 44 {
		t.Fatalf("WAV 只有 %d 字节，没有任何 PCM", len(wav))
	}
	secs := Seconds(wav)
	if secs <= 0 {
		t.Fatalf("WAV 时长 = %v，byteRate 或 data 长度写错了", secs)
	}
	t.Logf("录到 %.3f 秒 / %d 字节", secs, len(wav))
	if rec.Level() != 0 {
		t.Fatalf("Stop 之后 Level 应为 0，实际 %v", rec.Level())
	}
}

func TestRecorderStopBeforeStartIsAnError(t *testing.T) {
	rec, err := Open(-1)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rec.Close()
	if _, err := rec.Stop(); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("Stop 前应返回 ErrNotStarted，实际 %v", err)
	}
}

func TestRecorderCloseIsIdempotent(t *testing.T) {
	rec, err := Open(-1)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	rec.Close()
	rec.Close()
}
