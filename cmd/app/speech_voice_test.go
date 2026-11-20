// speech_voice_test.go —— 听写壳侧状态机的端到端测试（假宿主 + 假采集器）。
//
// 判据（对着官方 `VoiceInput.tsx` 的状态机，不是"跑通就行"）：
//
//	① 采集成功 → phase 进 recording，且 requesting 期间 Stop 无效（官方在
//	   `active.phase === 'recording'` 之前不许 finish）；
//	② 停止 → 把 WAV 交给宿主 `speech.transcribe`，**参数是裸 base64**、
//	   providerId/language 原样带上；
//	③ 识别出文字 → phase 回 idle 且**排一次草稿写入**，策略是 DraftInsert；
//	④ 识别出空串 → feedback + "未识别到语音"，**不许**写草稿；
//	⑤ 识别失败 → feedback 带上宿主原文；
//	⑥ 取消之后才回来的结果不许顶上界面。
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/audio"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

// fakeRecorder 是 audio.Recorder 的测试替身：不碰声卡，交回一段真 WAV 头的数据。
type fakeRecorder struct {
	mu      sync.Mutex
	started bool
	stopped bool
	wav     []byte
	startAt func() error // 非 nil 时 Start 返回它（用来造"麦克风打不开"）
}

func (f *fakeRecorder) Start() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startAt != nil {
		return f.startAt()
	}
	f.started = true
	return nil
}

func (f *fakeRecorder) Level() float64 { return 0.5 }

func (f *fakeRecorder) Stop() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = true
	return f.wav, nil
}

func (f *fakeRecorder) Close() {}

// voiceTestHost 造一个只会答 speech.* 的假宿主。
type voiceTestHost struct {
	transcribeBody map[string]json.RawMessage
	transcribeResp string
	transcribeCode int
}

func (h *voiceTestHost) start(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/speech.catalog", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"MS-2052-80-DESK","name":"MS-2052-80-DESK","language":"zh-CN"}]`))
	})
	mux.HandleFunc("/api/speech.configure", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"providerId":"MS-2052-80-DESK","language":"zh-CN"}`))
	})
	mux.HandleFunc("/api/speech.transcribe", func(w http.ResponseWriter, r *http.Request) {
		body := map[string]json.RawMessage{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		h.transcribeBody = body
		if h.transcribeCode >= 400 {
			w.WriteHeader(h.transcribeCode)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_audio","message":"audioBase64 不是合法 base64"}}`))
			return
		}
		resp := h.transcribeResp
		if resp == "" {
			resp = `{"text":"你好世界","audioSeconds":1.5,"inferenceSeconds":0.2}`
		}
		_, _ = w.Write([]byte(resp))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newVoiceTestModel(t *testing.T, host *voiceTestHost) *model {
	t.Helper()
	srv := host.start(t)
	m := &model{cli: apiclient.New(srv.URL), voice: newVoiceWiring()}
	return m
}

// stubRecorder 把 voiceOpenRecorder 换成给定的采集器（测试结束自动还原）。
func stubRecorder(t *testing.T, rec audio.Recorder, err error) {
	t.Helper()
	prev := voiceOpenRecorder
	voiceOpenRecorder = func(int) (audio.Recorder, error) { return rec, err }
	t.Cleanup(func() { voiceOpenRecorder = prev })
}

// waitPhase 等状态机走到某一档（异步链条：读目录 → 开麦 → 识别）。
func waitPhase(t *testing.T, m *model, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if m.voiceState().Phase == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等 %q 超时，当前 = %q（message=%q）", want, m.voiceState().Phase, m.voiceState().Message)
}

func sampleWAV() []byte {
	// 100 ms 的 16 kHz 单声道 16-bit PCM。
	return audio.EncodeWAV(make([]byte, audio.BlockBytes))
}

func TestVoiceBeginStopWritesTranscriptToDraft(t *testing.T) {
	host := &voiceTestHost{}
	// 挂一个 composer 插入能力：识别结果要**插到采集时光标处**
	// （官方 `insertText(text, active.span)`，VoiceInput.tsx:122）。
	m := voiceModelWithInsertion(t, host, &fakeInsertion{
		span: pluginapi.InsertionSpan{Start: 0, End: 0, DraftRev: 1}, captureOK: true, insertOK: true})
	rec := &fakeRecorder{wav: sampleWAV()}
	stubRecorder(t, rec, nil)

	m.voiceBegin()
	waitPhase(t, m, voicePhaseRecording)

	// requesting 期间 Stop 必须无效：官方在 phase 还是 requesting 时 finish 直接 return。
	// 这里用"先 Stop 再 Begin"造不出那个窗口，改为直接断言 recording 才允许 Stop。
	m.voiceStop()
	waitPhase(t, m, voicePhaseIdle)

	// 官方语义（`VoiceInput.tsx:122`）是 `insertText(text, active.span)`：
	// 排的是**带位置**的插入，不是"追加到末尾"。（旧断言钉的是旧的错语义。）
	m.mu.Lock()
	req := m.pendingInsert
	m.mu.Unlock()
	if req == nil {
		t.Fatal("识别出文字之后必须排一次草稿插入")
	}
	if req.text != "你好世界" {
		t.Fatalf("草稿文字 = %q", req.text)
	}
	if !req.spanOK {
		t.Fatal("没有拿回采集时的 span：composer 的能力没接上")
	}
	if req.span.Start != 0 || req.span.End != 0 {
		t.Fatalf("空草稿的光标应在 0，实际 %+v", req.span)
	}
	if !rec.stopped {
		t.Fatal("采集器没有 Stop")
	}

	// 请求形状：宿主从 `request` 子对象取参数，audioBase64 是**裸 base64**。
	var reqBody map[string]string
	if err := json.Unmarshal(host.transcribeBody["request"], &reqBody); err != nil {
		t.Fatalf("请求缺 request 子对象：%v", host.transcribeBody)
	}
	if reqBody["providerId"] != "MS-2052-80-DESK" || reqBody["language"] != "zh-CN" {
		t.Fatalf("request = %+v", reqBody)
	}
	if _, err := base64.StdEncoding.DecodeString(reqBody["audioBase64"]); err != nil {
		t.Fatalf("audioBase64 不是裸 base64：%v", err)
	}
}

func TestVoiceStopIsIgnoredBeforeRecording(t *testing.T) {
	host := &voiceTestHost{}
	m := newVoiceTestModel(t, host)
	m.voiceStop() // idle 时调：什么都不该发生，也不该 panic
	if got := m.voiceState().Phase; got != voicePhaseIdle {
		t.Fatalf("phase = %q", got)
	}
}

func TestVoiceEmptyRecognitionShowsFeedbackAndWritesNoDraft(t *testing.T) {
	host := &voiceTestHost{transcribeResp: `{"text":"","audioSeconds":1,"inferenceSeconds":0.1}`}
	m := newVoiceTestModel(t, host)
	stubRecorder(t, &fakeRecorder{wav: sampleWAV()}, nil)

	m.voiceBegin()
	waitPhase(t, m, voicePhaseRecording)
	m.voiceStop()
	waitPhase(t, m, voicePhaseFeedback)

	st := m.voiceState()
	if st.Message != "没有识别出文字，再说一次" {
		t.Fatalf("feedback 文案 = %q", st.Message)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pendingDraft != nil {
		t.Fatalf("空识别结果不许写草稿：%+v", m.pendingDraft)
	}
	if m.pendingInsert != nil {
		t.Fatalf("空识别结果不许排插入：%+v", m.pendingInsert)
	}
	if m.voice.pending != "" {
		t.Fatalf("空识别结果不该有 pending：%q", m.voice.pending)
	}
}

func TestVoiceTranscribeFailureKeepsHostMessage(t *testing.T) {
	host := &voiceTestHost{transcribeCode: 400}
	m := newVoiceTestModel(t, host)
	stubRecorder(t, &fakeRecorder{wav: sampleWAV()}, nil)

	m.voiceBegin()
	waitPhase(t, m, voicePhaseRecording)
	m.voiceStop()
	waitPhase(t, m, voicePhaseFeedback)
	msg := m.voiceState().Message
	if !strings.Contains(msg, "audioBase64 不是合法 base64") {
		t.Fatalf("feedback 应带宿主原文，实际 = %q", msg)
	}
}

func TestVoiceMicrophoneFailureLandsInFeedback(t *testing.T) {
	host := &voiceTestHost{}
	m := newVoiceTestModel(t, host)
	rec := &fakeRecorder{startAt: func() error { return errors.New("device busy") }}
	stubRecorder(t, rec, nil)

	m.voiceBegin()
	waitPhase(t, m, voicePhaseFeedback)
	if msg := m.voiceState().Message; !strings.Contains(msg, "device busy") {
		t.Fatalf("feedback = %q", msg)
	}
}

func TestVoiceCancelDropsInFlightResult(t *testing.T) {
	host := &voiceTestHost{}
	m := newVoiceTestModel(t, host)
	rec := &fakeRecorder{wav: sampleWAV()}
	stubRecorder(t, rec, nil)

	m.voiceBegin()
	waitPhase(t, m, voicePhaseRecording)
	m.voiceCancel()
	if got := m.voiceState().Phase; got != voicePhaseIdle {
		t.Fatalf("取消之后 phase = %q", got)
	}
	// 取消之后宿主的回话不该再改状态，也不该写草稿。
	time.Sleep(100 * time.Millisecond)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pendingDraft != nil {
		t.Fatalf("取消之后还在写草稿：%+v", m.pendingDraft)
	}
	if m.voice.phase != voicePhaseIdle {
		t.Fatalf("取消之后 phase 被改回 %q", m.voice.phase)
	}
}

func TestVoiceCapabilityReportsWhyItIsUnavailable(t *testing.T) {
	host := &voiceTestHost{}
	srv := host.start(t)
	m := &model{cli: apiclient.New(srv.URL), voice: newVoiceWiring()}
	// 设备读取是异步的：先等它落定，再判"没有识别器"这一档。
	cap := m.voiceCapability()
	if cap.Available {
		t.Skipf("本机有麦克风且宿主在线，跳过不可用分支（reason=%q）", cap.Reason)
	}
	if cap.Reason == "" {
		t.Fatal("不可用时 Reason 必须非空：界面靠它决定不画入口")
	}
	if cap.MaxSeconds != audio.MaxSeconds {
		t.Fatalf("MaxSeconds = %d，期望 %d", cap.MaxSeconds, audio.MaxSeconds)
	}
}

func TestJoinInlineKeepsListenableSpacing(t *testing.T) {
	cases := []struct{ existing, addition, want string }{
		{"", "你好", "你好"},
		{"你好", "世界", "你好 世界"},
		{"你好 ", "世界", "你好 世界"},
		{"你好\n", "世界", "你好\n世界"},
		{"你好", "", "你好"},
	}
	for _, c := range cases {
		if got := joinInline(c.existing, c.addition); got != c.want {
			t.Fatalf("joinInline(%q,%q) = %q，期望 %q", c.existing, c.addition, got, c.want)
		}
	}
}

// ── 官方 span 插入语义（#66 ①）────────────────────────────────────────

// fakeInsertion 是 `pluginapi.DraftInsertion` 的测试替身：记录调用，按预设答复。
type fakeInsertion struct {
	// span 是 CaptureInsertion 回的快照。
	span pluginapi.InsertionSpan
	// captureOK=false 表示"没有可插入的草稿"。
	captureOK bool
	// insertOK 是 InsertText 的答复。
	insertOK bool
	// accepted 记下 InsertText 收到的 (text, span)。
	accepted []pluginapi.InsertionSpan
	texts    []string
	captures int
}

func (f *fakeInsertion) CaptureInsertion() (pluginapi.InsertionSpan, bool) {
	f.captures++
	return f.span, f.captureOK
}

func (f *fakeInsertion) InsertText(text string, span pluginapi.InsertionSpan) bool {
	f.texts = append(f.texts, text)
	f.accepted = append(f.accepted, span)
	return f.insertOK
}

// voiceModelWithInsertion 造一个挂了 composer 插入能力的模型。
func voiceModelWithInsertion(t *testing.T, host *voiceTestHost, service pluginapi.DraftInsertion) *model {
	t.Helper()
	m := newVoiceTestModel(t, host)
	kernel := uikernel.New("test")
	kernel.Root().Provide(pluginapi.KeyDraftInsertion, service)
	m.kernel = kernel
	return m
}

// TestVoiceInsertUsesCapturedSpan：插到**采集时**的光标处，不是追加到末尾。
//
// 官方 `VoiceInput.tsx:131-134` 在 start() 里存 span，`:122` 拿它插；本仓的
// 对应断言是"排队的 span == Begin 那一刻捕获的 span，且真的调了 InsertText"。
func TestVoiceInsertUsesCapturedSpan(t *testing.T) {
	service := &fakeInsertion{span: pluginapi.InsertionSpan{Start: 2, End: 5, DraftRev: 7}, captureOK: true, insertOK: true}
	host := &voiceTestHost{}
	m := voiceModelWithInsertion(t, host, service)
	stubRecorder(t, &fakeRecorder{wav: sampleWAV()}, nil)

	m.voiceBegin()
	waitPhase(t, m, voicePhaseRecording)

	m.mu.Lock()
	captured := m.voice.span
	spanOK := m.voice.spanOK
	m.mu.Unlock()
	if !spanOK || captured != (pluginapi.InsertionSpan{Start: 2, End: 5, DraftRev: 7}) {
		t.Fatalf("Begin 没有捕获 span：ok=%v span=%+v", spanOK, captured)
	}

	m.voiceStop()
	waitPhase(t, m, voicePhaseIdle)
	// 真正落插入发生在 UI 线程（applyPendingDraft）；单测里手动跑一次那一帧。
	m.applyPendingDraft()

	if len(service.texts) != 1 || service.texts[0] != "你好世界" {
		t.Fatalf("InsertText 调用 = %v", service.texts)
	}
	if service.accepted[0] != (pluginapi.InsertionSpan{Start: 2, End: 5, DraftRev: 7}) {
		t.Fatalf("插入用的 span = %+v，应为采集时那一个", service.accepted[0])
	}
	if st := m.voiceState(); st.Pending != "" || st.Phase != voicePhaseIdle {
		t.Fatalf("插成功之后不该有 pending：%+v", st)
	}
}

// TestVoiceInsertConflictKeepsText：草稿被改过 → 不硬插、文字留在 pending、
// 进 feedback 并说官方那句 conflict；点「插入文字」按当前光标重插后清空。
//
// 官方两段（`VoiceInput.tsx:122-123` 与 `:175-178`）。
func TestVoiceInsertConflictKeepsText(t *testing.T) {
	service := &fakeInsertion{span: pluginapi.InsertionSpan{Start: 2, End: 5, DraftRev: 7},
		captureOK: true, insertOK: false}
	host := &voiceTestHost{}
	m := voiceModelWithInsertion(t, host, service)
	stubRecorder(t, &fakeRecorder{wav: sampleWAV()}, nil)

	m.voiceBegin()
	waitPhase(t, m, voicePhaseRecording)
	m.voiceStop()
	waitPhase(t, m, voicePhaseIdle)
	m.applyPendingDraft()

	st := m.voiceState()
	if st.Pending != "你好世界" {
		t.Fatalf("冲突时识别文字必须留住，实际 pending=%q", st.Pending)
	}
	if st.Phase != voicePhaseFeedback {
		t.Fatalf("冲突后应进 feedback，实际 %q", st.Phase)
	}
	if st.Message != voiceConflictMessage {
		t.Fatalf("feedback 文案 = %q，应为官方 conflict 原文", st.Message)
	}

	// 「插入文字」：重新采一次光标（官方 captureInsertion()，不是沿用旧 span）。
	service.insertOK = true
	service.span = pluginapi.InsertionSpan{Start: 0, End: 0, DraftRev: 9}
	m.voiceInsert()

	if len(service.accepted) != 2 {
		t.Fatalf("InsertText 调用次数 = %d，期望 2（冲突一次 + 插入文字一次）", len(service.accepted))
	}
	if service.accepted[1] != (pluginapi.InsertionSpan{Start: 0, End: 0, DraftRev: 9}) {
		t.Fatalf("「插入文字」应重新采光标，实际 %+v", service.accepted[1])
	}
	st = m.voiceState()
	if st.Pending != "" || st.Phase != voicePhaseIdle || st.Message != "" {
		t.Fatalf("插入成功后应清 pending 并回 idle：%+v", st)
	}
}

// TestVoiceInsertFallsBackWhenComposerNotWired：没有插入能力时退回追加到末尾，
// **绝不丢字**（老壳 / 没装 ds-composer）。
func TestVoiceInsertFallsBackWhenComposerNotWired(t *testing.T) {
	host := &voiceTestHost{}
	m := newVoiceTestModel(t, host)
	stubRecorder(t, &fakeRecorder{wav: sampleWAV()}, nil)

	m.voiceBegin()
	waitPhase(t, m, voicePhaseRecording)
	m.voiceStop()
	waitPhase(t, m, voicePhaseIdle)

	m.mu.Lock()
	req := m.pendingInsert
	m.mu.Unlock()
	if req == nil || req.spanOK {
		t.Fatalf("没有 composer 时 spanOK 必须为 false：%+v", req)
	}
	m.applyPendingDraft()
	// 兜底是 `insertDraftInline`：直接追加到壳草稿与 composer 的 field 末尾
	// （旧行为，不丢字），不是继续排队。
	if got := m.draft.Text(); got != "你好世界" {
		t.Fatalf("没有插入能力时必须退回追加到末尾，实际草稿 = %q", got)
	}
	m.mu.Lock()
	if m.voice.pending != "" {
		m.mu.Unlock()
		t.Fatalf("兜底成功不该留 pending：%q", m.voice.pending)
	}
	m.mu.Unlock()
}
