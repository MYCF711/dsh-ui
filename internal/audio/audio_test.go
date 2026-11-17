package audio

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

func TestEncodeWAVIs16kMono16BitPCM(t *testing.T) {
	pcm := make([]byte, BlockBytes)
	wav := EncodeWAV(pcm)
	if len(wav) != 44+len(pcm) {
		t.Fatalf("len=%d，期望 %d", len(wav), 44+len(pcm))
	}
	if got := string(wav[0:4]); got != "RIFF" {
		t.Fatalf("RIFF 标记 = %q", got)
	}
	if got := string(wav[8:12]); got != "WAVE" {
		t.Fatalf("WAVE 标记 = %q", got)
	}
	if got := string(wav[12:16]); got != "fmt " {
		t.Fatalf("fmt 标记 = %q", got)
	}
	if got := string(wav[36:40]); got != "data" {
		t.Fatalf("data 标记 = %q", got)
	}
	if got := binary.LittleEndian.Uint32(wav[4:8]); int(got) != 36+len(pcm) {
		t.Fatalf("RIFF 长度 = %d，期望 %d", got, 36+len(pcm))
	}
	if got := binary.LittleEndian.Uint16(wav[20:22]); got != 1 {
		t.Fatalf("formatTag = %d，期望 1(PCM)", got)
	}
	if got := binary.LittleEndian.Uint16(wav[22:24]); got != Channels {
		t.Fatalf("channels = %d，期望 %d", got, Channels)
	}
	if got := binary.LittleEndian.Uint32(wav[24:28]); got != SampleRate {
		t.Fatalf("sampleRate = %d，期望 %d", got, SampleRate)
	}
	// 这两个字段是宿主 speech.wavSeconds 的唯一依据（HOST:internal/speech/engine.go:373-390）：
	// 写错的话识别照样能过，但 audioSeconds 会报 0。
	if got := binary.LittleEndian.Uint32(wav[28:32]); got != SampleRate*2 {
		t.Fatalf("byteRate = %d，期望 %d", got, SampleRate*2)
	}
	if got := binary.LittleEndian.Uint16(wav[32:34]); got != 2 {
		t.Fatalf("blockAlign = %d，期望 2", got)
	}
	if got := binary.LittleEndian.Uint16(wav[34:36]); got != 16 {
		t.Fatalf("bitsPerSample = %d，期望 16", got)
	}
	if got := binary.LittleEndian.Uint32(wav[40:44]); int(got) != len(pcm) {
		t.Fatalf("data 长度 = %d，期望 %d", got, len(pcm))
	}
}

func TestSecondsMatchesWAVByteRate(t *testing.T) {
	// 1 秒 = 16000 样本 × 2 字节。
	pcm := make([]byte, SampleRate*2)
	if got := Seconds(EncodeWAV(pcm)); math.Abs(got-1) > 1e-9 {
		t.Fatalf("Seconds = %v，期望 1", got)
	}
	// 0.5 秒。
	pcm = make([]byte, SampleRate)
	if got := Seconds(EncodeWAV(pcm)); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("Seconds = %v，期望 0.5", got)
	}
	if got := Seconds(nil); got != 0 {
		t.Fatalf("空输入 Seconds = %v，期望 0", got)
	}
}

func TestRMSOfSilenceAndFullScale(t *testing.T) {
	if got := RMS(nil); got != 0 {
		t.Fatalf("空 PCM RMS = %v，期望 0", got)
	}
	silence := make([]byte, 320)
	if got := RMS(silence); got != 0 {
		t.Fatalf("静音 RMS = %v，期望 0", got)
	}
	// 满量程方波：每个样本都是 32767 → RMS ≈ 32767/32768。
	full := make([]byte, 320)
	for i := 0; i < len(full); i += 2 {
		binary.LittleEndian.PutUint16(full[i:], 32767)
	}
	if got := RMS(full); math.Abs(got-1) > 1e-3 {
		t.Fatalf("满量程 RMS = %v，期望 ≈1", got)
	}
	// 半量程：16384/32768 = 0.5。
	half := make([]byte, 320)
	for i := 0; i < len(half); i += 2 {
		binary.LittleEndian.PutUint16(half[i:], 16384)
	}
	if got := RMS(half); math.Abs(got-0.5) > 1e-3 {
		t.Fatalf("半量程 RMS = %v，期望 ≈0.5", got)
	}
}

func TestBlockBytesIsOneHundredMilliseconds(t *testing.T) {
	// 100 ms × 16000 Hz × 2 字节 = 3200。
	if BlockBytes != 3200 {
		t.Fatalf("BlockBytes = %d，期望 3200", BlockBytes)
	}
	if MaxSeconds*1000/BlockMillis != 600 {
		t.Fatalf("block 数 = %d，期望 600", MaxSeconds*1000/BlockMillis)
	}
}

func TestWindowsOnlyErrorsAreNotSilentSuccess(t *testing.T) {
	// 两条错误必须是**结构化**的：界面靠它决定隐藏入口还是报错，
	// 不许用空切片/静音 WAV 冒充成功。
	if !errors.Is(ErrNoDevice, ErrNoDevice) || ErrNoDevice.Error() == "" {
		t.Fatalf("ErrNoDevice 为空")
	}
	if !errors.Is(ErrUnsupported, ErrUnsupported) || ErrUnsupported.Error() == "" {
		t.Fatalf("ErrUnsupported 为空")
	}
}
