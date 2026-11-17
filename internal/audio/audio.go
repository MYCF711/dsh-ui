// Package audio 是界面侧的**麦克风采集层**。
//
// # 为什么需要它
//
// 官方 `dsh-experimental-client-ui-voice-input` 的客户端半边完全长在浏览器音频栈上
// （`OFF:dsh-experimental-client-ui-voice-input/lib/client.js:1528` 的
// `navigator.mediaDevices`、`:1532` 的 `getUserMedia`、`:1573-1577` 的
// `MediaRecorder`、`:1651` 的 `OfflineAudioContext(1,…,16000)`）。
// 本仓界面是 Go + Viva，没有 DOM、没有 WebAudio，`navigator` 这一层不存在。
//
// 但**能力本身不缺**：Windows 的 `winmm` 录音 API 用纯 `syscall` 就能调，
// 不需要 cgo（而本仓 CI 强制 `CGO_ENABLED=0`，见 `dsh-ui/.github/workflows/ci.yml`）。
// 所以这一层把"采集 → 16 kHz 单声道 PCM → WAV"做成纯 Go 实现，
// 与官方浏览器的产物**同一形状**：宿主 `speech/transcribe` 要的就是一个 WAV。
//
// # 与官方三步的逐条对应
//
//	官方                                         本仓
//	getUserMedia({audio:{deviceId}})              Open(deviceID)（waveInOpen）
//	MediaRecorder 累积 chunk                      采集循环把每个 block 追加到 PCM
//	OfflineAudioContext(1,…,16000) 重采样         向驱动**直接要** 16 kHz 单声道
//	                                              16-bit（Windows 音频引擎负责转换）
//	encodeWave(Float32Array) 出 WAV               EncodeWAV(pcm)
//	btoa(...) 出 base64                           调用方做 base64（见 apiclient.SpeechTranscribe）
//	analyser.getFloatTimeDomainData → RMS         Recorder.Level()（同一公式）
//
// 向驱动直接要 16 kHz 单声道，是官方重采样的等价物：`waveInGetDevCaps` 的
// `dwFormats` 位掩码里 `WAVE_FORMAT_1M16` 表示"16 kHz / 单声道 / 16-bit"，
// 由 Windows 音频引擎做格式转换，等价于官方在 `OfflineAudioContext` 里做的事，
// 但少一次整段解码。驱动不支持时 Open 直接失败（结构化错误），不做静默降级。
package audio

import (
	"encoding/binary"
	"errors"
	"math"
	"sync/atomic"
)

// 采集格式：16 kHz / 单声道 / 16-bit PCM。
//
// 这三个数是**宿主入参的硬要求**，不是我们的偏好：官方客户端把重采样目标写死成
// `OfflineAudioContext(1, …, 16e3)`（`client.js:1651`），宿主 `internal/speech`
// 也只接受 RIFF/WAVE（`HOST:internal/speech/engine.go:318`）。
const (
	SampleRate    = 16000
	Channels      = 1
	BitsPerSample = 16
)

// MaxSeconds 是一次录音的上限（秒）。
//
// 官方这个数来自宿主目录（`catalog.maxDurationSeconds`），我们的宿主
// `speech/catalog` **不返回**它（只返回 id/name/language，见
// `HOST:internal/transport/handlers_speech_official.go` 的 `OfficialSpeechProvider`），
// 所以上限由本层自己定，并在界面上按它自动停止（与官方 `active.timer` 同一行为）。
const MaxSeconds = 60

// BlockMillis 是一个采集 block 的时长。100 ms 是延迟与开销的折中：
// 电平表按 10 Hz 刷新已经足够顺滑，1 秒只产生 10 个 block 的回调检查。
const BlockMillis = 100

// BlockBytes 是一个采集 block 的字节数（16 kHz × 16-bit × 单声道 × 100 ms）。
const BlockBytes = SampleRate * Channels * BitsPerSample / 8 * BlockMillis / 1000

var (
	// ErrUnsupported 表示本平台没有这条采集实现（非 Windows 的 best-effort 桩）。
	ErrUnsupported = errors.New("audio: 本平台不支持麦克风采集")
	// ErrNoDevice 表示系统里没有录音设备。
	ErrNoDevice = errors.New("audio: 没有可用的麦克风")
	// ErrEmpty 表示停止时一个样本都没收到（设备被拔出/静音到没数据）。
	ErrEmpty = errors.New("audio: 没有录到任何音频")
	// ErrNotStarted 表示在 Start 之前调用了 Stop。
	ErrNotStarted = errors.New("audio: 录音尚未开始")
)

// Device 是一个录音设备。
//
// ID 是 `waveInGetNumDevs` 的设备序号（0..n-1），不是持久的 GUID：
// 设备插拔后序号会变。所以 Shell 只把它当"这一次选择"，与官方
// `deviceId` 的语义不同（官方那个是 Chromium 的稳定 id）——这一点在账本里登记。
type Device struct {
	ID   int
	Name string
	// Channels 是设备原生声道数，仅用于显示（实际采集固定 16 kHz 单声道）。
	Channels int
}

// Recorder 是一次录音会话。
//
// 生命周期：Open → Start → (Level) → Stop → Close。
// Start/Stop 可以在非 UI 线程调用；Level 任意线程安全；Close 幂等。
type Recorder interface {
	// Start 开始采集。重复调用返回错误。
	Start() error
	// Level 返回最近一个 block 的 RMS 电平（0..1 量纲，与官方 `amplitude()` 同公式）。
	// 未开始时返回 0。
	Level() float64
	// Stop 停止采集并交回 16 kHz 单声道 16-bit 的 **WAV** 字节。
	// 一个 block 都没收到时返回 ErrEmpty。
	Stop() ([]byte, error)
	// Close 释放设备与缓冲。幂等，Stop 之后可以再调。
	Close()
}

// EncodeWAV 把 16 kHz 单声道 16-bit 小端 PCM 编成 RIFF/WAVE 字节。
//
// 44 字节标准头；`byteRate` 与 `blockAlign` 按真实格式写——宿主
// `internal/speech` 的 `wavSeconds` 就是读这两个字段算时长的
// （`HOST:internal/speech/engine.go:373-390`），写错了识别能过但时长报 0。
func EncodeWAV(pcm []byte) []byte {
	out := make([]byte, 44+len(pcm))
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+len(pcm)))
	copy(out[8:], "WAVE")
	copy(out[12:], "fmt ")
	binary.LittleEndian.PutUint32(out[16:], 16) // fmt chunk size
	binary.LittleEndian.PutUint16(out[20:], 1)  // PCM
	binary.LittleEndian.PutUint16(out[22:], Channels)
	binary.LittleEndian.PutUint32(out[24:], SampleRate)
	binary.LittleEndian.PutUint32(out[28:], SampleRate*Channels*BitsPerSample/8)
	binary.LittleEndian.PutUint16(out[32:], Channels*BitsPerSample/8)
	binary.LittleEndian.PutUint16(out[34:], BitsPerSample)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(len(pcm)))
	copy(out[44:], pcm)
	return out
}

// RMS 是 16-bit 小端 PCM 的均方根电平，归一到 0..1 量纲。
//
// 与官方 `amplitude()`（`client.js:1601-1610`）同一公式，只差刻度：
// 官方吃的是 `getFloatTimeDomainData` 的 -1..1 浮点，这里先把 int16 归一再求方均根。
// 波形条按它画，所以这个换算必须**线性**——用 log 会让静音时也在动。
func RMS(pcm []byte) float64 {
	n := len(pcm) / 2
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		v := float64(int16(binary.LittleEndian.Uint16(pcm[i*2:]))) / 32768
		sum += v * v
	}
	return math.Sqrt(sum / float64(n))
}

// Seconds 按 WAV 的 byteRate 与 data 长度算时长（与宿主同一算法）。
func Seconds(wav []byte) float64 {
	if len(wav) < 44 {
		return 0
	}
	byteRate := binary.LittleEndian.Uint32(wav[28:32])
	if byteRate == 0 {
		return 0
	}
	for off := 12; off+8 <= len(wav); {
		id := string(wav[off : off+4])
		size := int(binary.LittleEndian.Uint32(wav[off+4 : off+8]))
		if id == "data" {
			if size > len(wav)-off-8 {
				size = len(wav) - off - 8
			}
			return float64(size) / float64(byteRate)
		}
		off += 8 + size + (size & 1)
	}
	return 0
}

// levelBits 用原子位模式装 float64，让采集 goroutine 写、UI 线程读不用锁。
type levelBits struct{ v atomic.Uint64 }

func (l *levelBits) set(f float64) { l.v.Store(math.Float64bits(f)) }
func (l *levelBits) get() float64  { return math.Float64frombits(l.v.Load()) }
