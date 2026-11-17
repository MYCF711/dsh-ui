//go:build windows

package audio

// capture_windows.go —— Windows 端的采集实现，只用 `syscall`（无 cgo）。
//
// # 为什么不是 WASAPI/COM
//
// 宿主 `internal/speech` 已经踩过一次 COM 的坑：在 Go 的 exec 管道下 COM 会挂起
// （实测 60 s 不返回），所以它改成受管 C# helper（`HOST:internal/speech/engine.go:1-10`）。
// 采集这边更简单——`winmm` 的 `waveIn*` 是**扁平 C API**，没有 COM 封送、
// 没有跨线程同类，`syscall.NewLazyDLL` 直接调，正好保住 `CGO_ENABLED=0`。
//
// # 缓冲策略
//
// 一次性排入 MaxSeconds 那么多个 100 ms block（600 个 × 3200 B = 1.92 MB），
// 采集期间只做"收割 + 算电平"，**不回收**。回收（unprepare→prepare→addBuffer）
// 要在驱动仍持有队列时反复进出声卡状态机，出错只会表现成"后半段没声音"这种
// 静默失败；固定上限换来的是确定行为：到点自动停止（官方也是定时器到点即停，
// `client.js:1962-1964` 的 `active.timer`）。

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	winmmDLL = syscall.NewLazyDLL("winmm.dll")

	procWaveInGetNumDevs      = winmmDLL.NewProc("waveInGetNumDevs")
	procWaveInGetDevCaps      = winmmDLL.NewProc("waveInGetDevCapsW")
	procWaveInOpen            = winmmDLL.NewProc("waveInOpen")
	procWaveInClose           = winmmDLL.NewProc("waveInClose")
	procWaveInPrepareHeader   = winmmDLL.NewProc("waveInPrepareHeader")
	procWaveInUnprepareHeader = winmmDLL.NewProc("waveInUnprepareHeader")
	procWaveInAddBuffer       = winmmDLL.NewProc("waveInAddBuffer")
	procWaveInStart           = winmmDLL.NewProc("waveInStart")
	procWaveInStop            = winmmDLL.NewProc("waveInStop")
	procWaveInReset           = winmmDLL.NewProc("waveInReset")
)

// waveFormatEx 是 `WAVEFORMATEX`（mmreg.h）。
type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	CBSize         uint16
}

// waveHdr 是 `WAVEHDR`（mmeapi.h）。
type waveHdr struct {
	Data          uintptr
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

// waveInCaps 是 `WAVEINCAPSW`（mmeapi.h）。
type waveInCaps struct {
	Mid           uint16
	Pid           uint16
	DriverVersion uint32
	Name          [32]uint16
	Formats       uint32
	Channels      uint16
	Reserved      uint16
	Support       uint32
}

const (
	// whdrDone 是 `WHDR_DONE`：驱动已经把这一块交还给应用。
	whdrDone = 0x00000001
	// callBackNull 是 `CALLBACK_NULL`：不用函数指针回调，由本层轮询收割。
	callBackNull = 0
	// waveMapper 是 `WAVE_MAPPER`（设备号 -1）。
	waveMapper = ^uintptr(0)
	// waveFormat1M16 是 `WAVE_FORMAT_1M16`：16 kHz / 单声道 / 16-bit。
	waveFormat1M16 = 0x00000004
	// blockCount 是预排的 block 数（MaxSeconds / BlockMillis）。
	blockCount = MaxSeconds * 1000 / BlockMillis
	// pollInterval 是收割轮询间隔：block 是 100 ms，20 ms 一轮足够，且电平表够顺。
	pollInterval = 20 * time.Millisecond
)

// Devices 列出系统里的录音设备。
//
// 官方用 `navigator.mediaDevices.enumerateDevices()` 过滤 `kind === "audioinput"`
// （`client.js:3157`）；这里对应 `waveInGetNumDevs` + `waveInGetDevCapsW`。
func Devices() ([]Device, error) {
	n, _, _ := procWaveInGetNumDevs.Call()
	out := make([]Device, 0, n)
	for i := uintptr(0); i < n; i++ {
		var caps waveInCaps
		if r, _, _ := procWaveInGetDevCaps.Call(i, uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps)); r != 0 {
			continue
		}
		name := syscall.UTF16ToString(caps.Name[:])
		if name == "" {
			name = fmt.Sprintf("录音设备 %d", int(i))
		}
		out = append(out, Device{ID: int(i), Name: name, Channels: int(caps.Channels)})
	}
	if len(out) == 0 {
		return nil, ErrNoDevice
	}
	return out, nil
}

// Open 打开一个采集器。deviceID 为 -1 表示系统默认（`WAVE_MAPPER`）。
func Open(deviceID int) (Recorder, error) {
	r := &recorder{deviceID: deviceID, level: levelBits{}, stop: make(chan struct{}), loopDone: make(chan struct{})}
	return r, nil
}

type recorder struct {
	deviceID int

	mu        sync.Mutex
	h         uintptr
	bufs      [][]byte
	hdrs      []waveHdr
	opened    bool
	started   bool
	closed    bool
	taken     []bool
	collected []byte
	err       error

	level levelBits

	stop     chan struct{}
	stopOnce sync.Once
	loopDone chan struct{}
}

// Start 打开设备并开始采集。
func (r *recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fmt.Errorf("audio: 采集器已关闭")
	}
	if r.started {
		return fmt.Errorf("audio: 采集已经开始了")
	}
	wfx := waveFormatEx{
		FormatTag:      1, // WAVE_FORMAT_PCM
		Channels:       Channels,
		SamplesPerSec:  SampleRate,
		AvgBytesPerSec: SampleRate * Channels * BitsPerSample / 8,
		BlockAlign:     Channels * BitsPerSample / 8,
		BitsPerSample:  BitsPerSample,
	}
	dev := waveMapper
	if r.deviceID >= 0 {
		dev = uintptr(r.deviceID)
	}
	var h uintptr
	// flags = CALLBACK_NULL：不要 WAVE_FORMAT_DIRECT，让 Windows 音频引擎把
	// 设备原生格式转成上面要的 16 kHz/单声道（官方那步 OfflineAudioContext 的等价物）。
	if rc, _, _ := procWaveInOpen.Call(uintptr(unsafe.Pointer(&h)), dev, uintptr(unsafe.Pointer(&wfx)), 0, 0, callBackNull); rc != 0 {
		return fmt.Errorf("audio: waveInOpen 失败（MMSYSERR %d）", rc)
	}
	r.h = h
	r.opened = true
	r.bufs = make([][]byte, blockCount)
	r.hdrs = make([]waveHdr, blockCount)
	r.taken = make([]bool, blockCount)
	for i := 0; i < blockCount; i++ {
		r.bufs[i] = make([]byte, BlockBytes)
		hdr := &r.hdrs[i]
		hdr.Data = uintptr(unsafe.Pointer(&r.bufs[i][0]))
		hdr.BufferLength = BlockBytes
		if rc, _, _ := procWaveInPrepareHeader.Call(r.h, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr)); rc != 0 {
			r.unprepareAllLocked()
			procWaveInClose.Call(r.h)
			r.opened = false
			return fmt.Errorf("audio: waveInPrepareHeader 失败（MMSYSERR %d）", rc)
		}
		if rc, _, _ := procWaveInAddBuffer.Call(r.h, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr)); rc != 0 {
			r.unprepareAllLocked()
			procWaveInClose.Call(r.h)
			r.opened = false
			return fmt.Errorf("audio: waveInAddBuffer 失败（MMSYSERR %d）", rc)
		}
	}
	if rc, _, _ := procWaveInStart.Call(r.h); rc != 0 {
		r.unprepareAllLocked()
		procWaveInClose.Call(r.h)
		r.opened = false
		return fmt.Errorf("audio: waveInStart 失败（MMSYSERR %d）", rc)
	}
	r.started = true
	go r.loop()
	return nil
}

// loop 收割已完成的 block 并更新电平。它不回收缓冲（见文件头）。
func (r *recorder) loop() {
	defer close(r.loopDone)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.harvest()
		}
	}
}

// harvest 把这一轮已完成的 block 抄进 collected，并只用**本轮最新的**那块算电平。
func (r *recorder) harvest() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.opened {
		return
	}
	var latest []byte
	for i := range r.hdrs {
		if r.taken[i] {
			continue
		}
		hdr := &r.hdrs[i]
		if hdr.Flags&whdrDone == 0 {
			continue
		}
		r.taken[i] = true
		n := int(hdr.BytesRecorded)
		if n <= 0 {
			continue
		}
		if n > len(r.bufs[i]) {
			n = len(r.bufs[i])
		}
		chunk := r.bufs[i][:n]
		r.collected = append(r.collected, chunk...)
		latest = chunk
	}
	if latest != nil {
		r.level.set(RMS(latest))
	}
}

// Level 返回最近一块的 RMS 电平。
func (r *recorder) Level() float64 { return r.level.get() }

// Stop 停止采集，交回 WAV。
func (r *recorder) Stop() ([]byte, error) {
	r.mu.Lock()
	started, opened, h := r.started, r.opened, r.h
	r.mu.Unlock()
	if !started {
		return nil, ErrNotStarted
	}
	// 先让收割循环退出，再由 Stop 自己做最后一次收割——避免两边同时对
	// collected 追加（那会丢数据或乱序，而乱序的 PCM 听不出来，只表现为识别变差）。
	r.stopOnce.Do(func() { close(r.stop) })
	<-r.loopDone
	r.harvest()

	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = false
	if opened {
		procWaveInStop.Call(h)
		procWaveInReset.Call(h)
	}
	// reset 之后所有在队列里的 block 都会带上 DONE 被退回，再收割一次把它们拿全。
	r.harvestLockedFinal()
	r.unprepareAllLocked()
	if r.opened {
		procWaveInClose.Call(r.h)
		r.opened = false
		r.h = 0
	}
	pcm := r.collected
	r.collected = nil
	r.level.set(0)
	if len(pcm) == 0 {
		return nil, ErrEmpty
	}
	return EncodeWAV(pcm), nil
}

// harvestLockedFinal 是 Stop 里那次"最后收割"的在锁版本：不留未读的块。
func (r *recorder) harvestLockedFinal() {
	for i := range r.hdrs {
		if r.taken[i] {
			continue
		}
		hdr := &r.hdrs[i]
		if hdr.Flags&whdrDone == 0 {
			continue
		}
		r.taken[i] = true
		n := int(hdr.BytesRecorded)
		if n <= 0 || n > len(r.bufs[i]) {
			continue
		}
		r.collected = append(r.collected, r.bufs[i][:n]...)
	}
}

func (r *recorder) unprepareAllLocked() {
	if !r.opened {
		return
	}
	for i := range r.hdrs {
		procWaveInUnprepareHeader.Call(r.h, uintptr(unsafe.Pointer(&r.hdrs[i])), unsafe.Sizeof(r.hdrs[i]))
	}
	runtime.KeepAlive(r.hdrs)
	runtime.KeepAlive(r.bufs)
}

// Close 释放设备。幂等。
func (r *recorder) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	closed := r.closed
	r.closed = true
	started := r.started
	r.mu.Unlock()
	if closed {
		return
	}
	if started {
		// 未 Stop 就 Close：静默丢弃这段录音，但设备必须释放干净。
		r.stopOnce.Do(func() { close(r.stop) })
		<-r.loopDone
		r.mu.Lock()
		if r.opened {
			procWaveInStop.Call(r.h)
			procWaveInReset.Call(r.h)
		}
		r.started = false
		r.unprepareAllLocked()
		if r.opened {
			procWaveInClose.Call(r.h)
			r.opened = false
			r.h = 0
		}
		r.mu.Unlock()
	}
}
