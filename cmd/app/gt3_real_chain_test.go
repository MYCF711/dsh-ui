//go:build windows

package main

// gt3_real_chain_test.go —— **真机验收**：真 `dshd serve` + 真 apiclient + 真画布。
//
// 为什么必须单独一个文件（而不是在 mock 上再补一条断言）：本任务要干掉的是
// "渲染器全绿、真机主区空白"这种假通过。mockd 的帧是造出来的，证明不了
// "宿主推的**这一种**形状"能投影出消息行——真实 v4 事件里 `assistant/text`
// （顶层 `text` 增量）、`step/end`（无 content）、`request/context`（无
// surfaceOp）这些 mock 里没有的帧，正是最先把主区搞乱的那几种。
//
// 跑法（不给 DSHD_BIN 就跳过，CI 上没有宿主二进制）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_gt3/dshd.exe"
//	go test ./cmd/app -run TestRealChain -count=1 -v
//
// 依赖本机 magpie 网关（`dshd` 默认 `DSHD_LLM_BASE=http://127.0.0.1:3425/v1`）；
// 网关连不上时**跳过**并说清原因——不拿"没跑到"冒充通过。

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// realChainPrompt 是真实 prompt。**标记串只可能出现在 assistant 正文里**
// （用户 prompt 里没有它），所以"画布上有它"就是"真回答被画出来了"。
const (
	realChainPrompt = "只做两件事，不要解释：\n1) 用 bash 工具执行 echo GT3-TOOL-OK\n2) 最后单独一行回复：GT3-REAL-RENDER-OK"
	realChainMarker = "GT3-REAL-RENDER-OK"
	realChainTool   = "GT3-TOOL-OK"
	realChainModel  = "group/deepseek-v4.1-flash-1017"
)

// realHost 是一台真 dshd serve。
type realHost struct {
	url   string
	token string
	home  string
	cmd   *exec.Cmd
}

// startRealHost 起一台真宿主（临时 home + 端口 0 + 父进程看门狗关）。
//
// 三条铁律（AGENTS §4）：临时 home、--addr 127.0.0.1:0、只回收自己起的进程。
func startRealHost(t *testing.T) *realHost {
	t.Helper()
	bin := os.Getenv("DSHD_BIN")
	if bin == "" {
		t.Skip("没有 DSHD_BIN（宿主二进制），跳过真机验收")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("DSHD_BIN 指向的二进制不存在：%v", err)
	}
	// 网关探活：连不上就跳过，不拿"没跑到"冒充通过。
	if !gatewayAlive() {
		t.Skip("本机 magpie 网关（127.0.0.1:3425/v1）连不上，跳过真机验收")
	}
	// 临时 home 放在 GT3 临时目录下（**不用 t.TempDir()**：本机 %TEMP% 在
	// C:/Users/ADMINI~1 下，dshd 在里面写 sessions/state 一律 Access is denied）。
	homeRoot := os.Getenv("DSHD_HOME_ROOT")
	if homeRoot == "" {
		homeRoot = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_gt3"
	}
	if err := os.MkdirAll(homeRoot, 0o755); err != nil {
		t.Fatalf("建临时 home 根目录失败: %v", err)
	}
	home, err := os.MkdirTemp(homeRoot, "realhome-")
	if err != nil {
		t.Fatalf("建临时 home 失败: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	cmd := exec.Command(bin, "serve",
		"--home", home,
		"--addr", "127.0.0.1:0",
		// 看门狗会按 ppid 判活：测试进程退出时它该跟着退，但测试里我们主动 kill。
		"--parent-watch=false",
		"--policy", "allow",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("取 stdout 失败: %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("起不了 dshd serve: %v", err)
	}
	host := &realHost{home: home, cmd: cmd}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})

	// ready 行：`ready {"url":…,"token":…}`（stdout 只放这一行，日志走 stderr）。
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "ready ") {
				ready <- strings.TrimPrefix(line, "ready ")
				return
			}
		}
		close(ready)
	}()
	select {
	case raw, ok := <-ready:
		if !ok {
			t.Fatal("dshd serve 没打出 ready 行就退出了")
		}
		var parsed struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			t.Fatalf("ready 行解不开：%v（%q）", err, raw)
		}
		host.url, host.token = parsed.URL, parsed.Token
	case <-time.After(60 * time.Second):
		t.Fatal("等 ready 行超时（60s）")
	}
	if host.url == "" || host.token == "" {
		t.Fatalf("ready 行缺 url/token：url=%q token 空=%v", host.url, host.token == "")
	}
	return host
}

// gatewayAlive 探一下本机模型网关（不给 key 也能 /v1/models 就是活着）。
func gatewayAlive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:3425/v1/models", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// connectRealGUI 用**真 apiclient** 走 /connect → cookie（与产品同一条路径）。
func connectRealGUI(t *testing.T, host *realHost) (*apiclient.Client, *apiclient.Mux) {
	t.Helper()
	client := apiclient.New(host.url)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := client.Connect(ctx, host.token); err != nil {
		t.Fatalf("/connect 失败: %v", err)
	}
	mux := &apiclient.Mux{Base: host.url, Cookies: client.Cookies()}
	return client, mux
}

// waitTurnDone 轮询 `session.history` 直到出现 `turn/done`（真实一轮收工）。
func waitTurnDone(t *testing.T, client *apiclient.Client, sessionID string, timeout time.Duration) []apiclient.Frame {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		res, err := client.History(ctx, sessionID, 0, 500, 0)
		cancel()
		if err == nil {
			for _, frame := range res.Frames {
				if frame.Type == "turn/done" || frame.Type == "turn/error" {
					return res.Frames
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("%s 内没等到 turn/done（真实一轮没收工）", timeout)
	return nil
}

// newRealModel 造一个连着真宿主的产品 model（数据面全真）。
func newRealModel(client *apiclient.Client, mux *apiclient.Mux, sessionID string) *model {
	return &model{
		cli:      client,
		mux:      mux,
		cur:      sessionID,
		sessions: []apiclient.Session{{ID: sessionID}},
	}
}

// renderTranscriptSeat 用真内核 + 真画布跑一次产品绘制路径。
func renderTranscriptSeat(t *testing.T, snap pluginapi.Snapshot) (*ui.Canvas, *dsconversation.Plugin) {
	t.Helper()
	conversation := &dsconversation.Plugin{}
	kernel := uikernel.New("gt3-real-chain")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, conversation); err != nil {
		t.Fatalf("装配失败: %v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	canvas := &ui.Canvas{}
	tree := &ui.Tree{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	env := &pluginapi.Env{
		Theme: theme, Data: snap, Tree: tree, Feat: tree.Feature("gt3-real-chain"),
		Canvas: canvas, Bounds: ui.Rect{Width: 1180, Height: 832}, MainWidth: 840,
	}
	env.Has = func(slot pluginapi.Slot) bool { return kernel.Has(slot) }
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), ui.Rect{Width: 1180, Height: 832}, func(f *flex.Frame) {
		// **走官方槽**：这就是真机上 `ChatView.column` 那一次 drawSlot。
		kernel.DrawSlot(f, pluginapi.Slot("conversation.view"), env)
	})
	return canvas, conversation
}

// canvasHas 报告画布上有没有这段文本（前缀匹配：长文本会被省略号截断）。
func canvasHas(canvas *ui.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == want || strings.HasPrefix(text.Value, want) {
			return true
		}
	}
	return false
}

func canvasTextValues(canvas *ui.Canvas) []string {
	values := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	return values
}

// TestRealChainFramesRenderRealText 是**硬验收**：真宿主产生的帧 → 真画布上
// 出现真实回答正文（不是 mock 数据、不是空壳）。
func TestRealChainFramesRenderRealText(t *testing.T) {
	host := startRealHost(t)
	client, mux := connectRealGUI(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sessionID, err := client.Create(ctx, "", "", "gt3-real-chain")
	if err != nil {
		t.Fatalf("session.create 失败: %v", err)
	}
	t.Logf("真实会话 id=%s（home=%s）", sessionID, host.home)

	promptCtx, promptCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer promptCancel()
	if _, err := client.Prompt(promptCtx, sessionID, realChainPrompt); err != nil {
		t.Fatalf("session.prompt 失败: %v", err)
	}

	frames := waitTurnDone(t, client, sessionID, 180*time.Second)
	var (
		hasText, hasTool, hasReasoning bool
	)
	for _, frame := range frames {
		switch frame.Type {
		case "assistant/text", "assistant/message":
			hasText = true
		case "tool/call", "tool/result":
			hasTool = true
		case "assistant/reasoning":
			hasReasoning = true
		}
	}
	if !hasText {
		t.Fatalf("真实一轮没有 text 帧（frames=%d）", len(frames))
	}
	t.Logf("真实帧 %d 条：text=%v tool=%v reasoning=%v", len(frames), hasText, hasTool, hasReasoning)

	// 数据面：真 `session.history`（不是 mock、不是 load 投影）。
	m := newRealModel(client, mux, sessionID)
	m.refreshFrames(sessionID)
	journal := m.frames[sessionID]
	if journal == nil || len(journal.snapshot()) == 0 {
		t.Fatal("真 history 没有拉到任何原始帧（数据面空）")
	}
	if got, want := len(journal.snapshot()), len(frames); got < want {
		t.Fatalf("数据面 %d 帧 < 宿主 %d 帧（丢帧）", got, want)
	}

	// 绘制：真快照 → 官方 conversation.view 槽 → 真画布。
	snapshotFrames, snapshotEpoch := m.snapshotFramesLocked(sessionID)
	canvas, conversation := renderTranscriptSeat(t, pluginapi.Snapshot{
		Cur: sessionID, Frames: snapshotFrames, JournalEpoch: snapshotEpoch,
	})

	// ① 投影里必须有 assistant 正文（标记串只可能来自 assistant）。
	var assistantText string
	for _, entry := range conversation.TranscriptEntries() {
		if entry.Kind == dsconversation.ChatNodeAssistantStep && strings.Contains(entry.Data.Text, realChainMarker) {
			assistantText = entry.Data.Text
			break
		}
	}
	if assistantText == "" {
		t.Fatalf("投影里没有带标记的 assistant 正文；entries=%v", conversation.TranscriptEntries())
	}
	// ② 画布上必须真的画出来（这是本任务的核心断言）。
	if !canvasHas(canvas, realChainMarker) {
		t.Fatalf("画布上没有真实回答正文 %q；实际文本：%v", realChainMarker, canvasTextValues(canvas))
	}
	t.Logf("画布真实回答正文 = %q", assistantText)

	// ③ 工具帧也真的落成行（不要求模型必然调工具，但调了就必须画出来）。
	if hasTool && !canvasHas(canvas, "bash") {
		t.Fatalf("有工具帧但画布上没有工具行；文本：%v", canvasTextValues(canvas))
	}
	if hasReasoning {
		if !canvasHas(canvas, "思考") {
			t.Fatalf("有 reasoning 帧但画布上没有思考区；文本：%v", canvasTextValues(canvas))
		}
	}
}

// TestRealChainEpochRebuildKeepsEveryFrame 钉住「断线重连后 journalEpoch 重建
// 正确、不重复不丢」。
//
// 真宿主的 409 `stale_epoch` 是"你手里的水位属于另一代 journal"的唯一信号：
// 收到就**整段丢弃重建**（不能按 seq 硬补——那会静默少几条）。
func TestRealChainEpochRebuildKeepsEveryFrame(t *testing.T) {
	host := startRealHost(t)
	client, mux := connectRealGUI(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sessionID, err := client.Create(ctx, "", "", "gt3-real-epoch")
	if err != nil {
		t.Fatalf("session.create 失败: %v", err)
	}
	promptCtx, promptCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer promptCancel()
	if _, err := client.Prompt(promptCtx, sessionID, realChainPrompt); err != nil {
		t.Fatalf("session.prompt 失败: %v", err)
	}
	frames := waitTurnDone(t, client, sessionID, 180*time.Second)

	// ① 真宿主对**过期世代**回 409 stale_epoch（契约四字段 + 顶层 epoch/lastSeq）。
	realEpoch := 0
	{
		historyCtx, historyCancel := context.WithTimeout(context.Background(), 10*time.Second)
		res, err := client.History(historyCtx, sessionID, 0, 500, 0)
		historyCancel()
		if err != nil {
			t.Fatalf("history 失败: %v", err)
		}
		realEpoch = res.JournalEpoch
	}
	if realEpoch <= 0 {
		t.Fatalf("宿主没有给 journalEpoch（%d）——换代语义无从验证", realEpoch)
	}
	staleCtx, staleCancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, staleErr := client.History(staleCtx, sessionID, 0, 500, realEpoch+1)
	staleCancel()
	if !staleEpochError(staleErr) {
		t.Fatalf("过期 journalEpoch 没有回 409 stale_epoch：%v", staleErr)
	}
	t.Logf("真宿主 stale_epoch 已复现（epoch=%d → %d）：%v", realEpoch, realEpoch+1, staleErr)

	// ② 重建基线：整段丢弃后用真 history 重建。
	m := newRealModel(client, mux, sessionID)
	m.refreshFrames(sessionID)
	before := m.frames[sessionID].snapshot()

	// 模拟"收到 session/reset"：三份水位 + 帧日志一起清（onJournalResetLocked）。
	m.mu.Lock()
	m.onJournalResetLocked(sessionID)
	m.mu.Unlock()
	if len(m.frames[sessionID].snapshot()) != 0 {
		t.Fatal("session/reset 之后帧日志没清干净（新旧两代混排）")
	}

	// 重建：从 0 重新拉（世代用宿主的真值）。
	m.refreshFrames(sessionID)
	after := m.frames[sessionID].snapshot()

	// ③ 不丢、不重复：seq 严格递增且与重建前一致。
	if len(after) != len(before) {
		t.Fatalf("重建后 %d 帧 ≠ 重建前 %d 帧（丢帧或重复）", len(after), len(before))
	}
	for i := range after {
		if after[i].Seq != before[i].Seq {
			t.Fatalf("第 %d 帧 seq = %d，重建前是 %d（行序/水位错位）", i, after[i].Seq, before[i].Seq)
		}
		if i > 0 && after[i].Seq <= after[i-1].Seq {
			t.Fatalf("第 %d 帧 seq=%d 没有递增（重复帧）", i, after[i].Seq)
		}
	}
	if got, want := after[len(after)-1].Seq, frames[len(frames)-1].Seq; got < want {
		t.Fatalf("重建后最高 seq=%d < 宿主推过的 %d（少帧）", got, want)
	}

	// ④ 重建后画布仍然画出真实回答（重建不是"清空"）。
	snapshotFrames, snapshotEpoch := m.snapshotFramesLocked(sessionID)
	canvas, _ := renderTranscriptSeat(t, pluginapi.Snapshot{
		Cur: sessionID, Frames: snapshotFrames, JournalEpoch: snapshotEpoch,
	})
	if !canvasHas(canvas, realChainMarker) {
		t.Fatalf("重连重建后画布上没有真实回答；文本：%v", canvasTextValues(canvas))
	}
	t.Logf("重连重建：%d 帧、seq [%d..%d]、画布含真实回答",
		len(after), after[0].Seq, after[len(after)-1].Seq)
}

// TestRealChainWSIncrementalKeepsSingleRow 走**真 WS** 拿增量帧，断言第二次
// prompt 的增量不会把第一条消息画两遍（同 seq 只算一次）。
func TestRealChainWSIncrementalKeepsSingleRow(t *testing.T) {
	host := startRealHost(t)
	client, mux := connectRealGUI(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sessionID, err := client.Create(ctx, "", "", "gt3-real-ws")
	if err != nil {
		t.Fatalf("session.create 失败: %v", err)
	}

	// 真 WS 订阅（与产品 runSubscription 同一条客户端路径）。
	subCtx, subCancel := context.WithCancel(context.Background())
	defer subCancel()
	events, err := mux.Subscribe(subCtx, "session:"+sessionID, 0)
	if err != nil {
		t.Fatalf("WS 订阅失败: %v", err)
	}
	m := newRealModel(client, mux, sessionID)

	promptCtx, promptCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer promptCancel()
	if _, err := client.Prompt(promptCtx, sessionID, realChainPrompt); err != nil {
		t.Fatalf("session.prompt 失败: %v", err)
	}

	// 消费到 turn/done 为止，**每条都走产品归约**（applyEvent）。
	deadline := time.After(180 * time.Second)
	seen := 0
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("WS 连接断了")
			}
			if ev.Kind == "gap" || ev.Kind == "session/reset" {
				continue
			}
			m.applyEvent(sessionID, ev)
			seen++
			if ev.Kind == "turn/done" || ev.Kind == "turn/error" {
				goto collected
			}
		case <-deadline:
			t.Fatalf("180s 内 WS 没收到 turn/done（已收 %d 帧）", seen)
		}
	}
collected:
	if seen == 0 {
		t.Fatal("WS 一帧都没收到")
	}

	journal := m.frames[sessionID].snapshot()
	seqs := make([]int, 0, len(journal))
	for _, frame := range journal {
		seqs = append(seqs, frame.Seq)
	}
	for i := 1; i < len(seqs); i++ {
		if seqs[i] <= seqs[i-1] {
			t.Fatalf("WS 增量帧 seq 没有严格递增（第 %d 条 %d ≤ 前一条 %d）：重复帧", i, seqs[i], seqs[i-1])
		}
	}

	snapshotFrames, snapshotEpoch := m.snapshotFramesLocked(sessionID)
	canvas, _ := renderTranscriptSeat(t, pluginapi.Snapshot{
		Cur: sessionID, Frames: snapshotFrames, JournalEpoch: snapshotEpoch,
	})
	if !canvasHas(canvas, realChainMarker) {
		t.Fatalf("WS 增量路径画布上没有真实回答；文本：%v", canvasTextValues(canvas))
	}
	t.Logf("WS 增量：收 %d 条事件、数据面 %d 帧、画布含真实回答", seen, len(journal))
}
