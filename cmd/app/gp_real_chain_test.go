//go:build windows

package main

// gp_real_chain_test.go —— 真机验收：**向前分页** 与 **审批卡归 composer**。
//
// 两条都必须"真"：
//
//	① 真 `dshd serve`（临时 home、端口 0）+ 真 apiclient + 真 WS；
//	② 真模型跑出来的 >1 页 journal，向上翻页取到旧帧并**画到真画布上**；
//	③ 真审批（`--policy ask` 的工具调用）→ 审批卡出现在 **composer 区域**，
//	   三动作可点，决定之后本地状态正确（宿主 200 accepted 才收卡片）。
//
// 为什么不能用 mock：mockd 的帧是造的，证明不了"宿主推的这一种形状"（分页
// 409 的两种来源、审批帧的 options 三项）能被产品正确消费——而这两种恰恰是
// 只在真宿主上才出现的形状。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_gt3/dshd.exe"
//	go test ./cmd/app -run 'TestRealChainGP' -count=1 -v
//
// 依赖本机 magpie 网关（`dshd` 默认 `DSHD_LLM_BASE=http://127.0.0.1:3425/v1`）；
// 网关连不上时**跳过**并说清原因——不拿"没跑到"冒充通过。

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	// gpTurnPrompt 让模型做一次工具调用再收尾：一轮约 80 条原始帧。
	// 三轮 > 200 = 一页（frameHistoryChunk），才能验"向前翻页"。
	gpTurnPrompt = "用 bash 执行 echo GP-TURN-OK，然后只回复一行：GP-TURN-DONE"
	// gpApprovalPrompt 触发需要审批的 bash 调用（宿主 --policy ask）。
	gpApprovalPrompt = "用 bash 执行 echo GP-APPROVAL-OK，然后只回复一行：GP-APPROVAL-DONE"
	gpApprovalMarker = "GP-APPROVAL-OK"
	gpApprovalModel  = "group/deepseek-v4.1-flash-1017"
)

// startGPHost 起一台真 `dshd serve`（临时 home + 端口 0 + 父进程看门狗关）。
//
// 与 gt3 的 startRealHost 的差别只有 `--policy`：审批链必须 `ask`，
// 分页链用 `allow`（不然每轮都卡在审批上）。
func startGPHost(t *testing.T, policy string) *realHost {
	t.Helper()
	bin := os.Getenv("DSHD_BIN")
	if bin == "" {
		t.Skip("没有 DSHD_BIN（宿主二进制），跳过真机验收")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("DSHD_BIN 指向的二进制不存在：%v", err)
	}
	if !gatewayAlive() {
		t.Skip("本机 magpie 网关（127.0.0.1:3425/v1）连不上，跳过真机验收")
	}
	homeRoot := os.Getenv("DSHD_HOME_ROOT")
	if homeRoot == "" {
		homeRoot = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_gp"
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
		"--parent-watch=false",
		"--policy", policy,
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

// lastSeqOf 取当前 journal 的最高 seq（用来区分"这一轮的 turn/done"与历史里旧的）。
func lastSeqOfHistory(t *testing.T, client *apiclient.Client, sessionID string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := client.History(ctx, sessionID, 0, 100000, 0)
	if err != nil {
		t.Fatalf("拉 history 失败: %v", err)
	}
	if len(res.Frames) == 0 {
		return 0
	}
	return res.Frames[len(res.Frames)-1].Seq
}

// waitTurnDoneAfter 等**这一轮**收工：只认 seq > afterSeq 的 turn/done。
//
// 不能用 gt3 的 waitTurnDone：它一看到 history 里**任何** turn/done 就返回，
// 而第二轮开始时上一轮的 done 还在历史里——连发三轮会直接撞上
// `busy: 上一轮还没结束`（第一次跑真机实测踩到）。
func waitTurnDoneAfter(t *testing.T, client *apiclient.Client, sessionID string, afterSeq int, timeout time.Duration) []apiclient.Frame {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		res, err := client.History(ctx, sessionID, afterSeq, 100000, 0)
		cancel()
		if err == nil {
			for _, frame := range res.Frames {
				if frame.Seq > afterSeq && (frame.Type == "turn/done" || frame.Type == "turn/error") {
					return res.Frames
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("%s 内没等到这一轮的 turn/done（afterSeq=%d）", timeout, afterSeq)
	return nil
}

// waitApproval 轮询到本地收到一条真审批（`approval/request` 帧进 model）。
func waitApproval(t *testing.T, m *model, timeout time.Duration) *approvalReq {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		ap := m.approval
		m.mu.Unlock()
		if ap != nil {
			return ap
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("%s 内没有收到真实审批帧（模型可能没调用需要审批的工具）", timeout)
	return nil
}

// renderProductSeats 在**同一张真画布**上按产品顺序画两帧：
//
//	main.body        transcript（消息流）
//	main.composer    composer 区域（审批条 + 输入卡片）
//
// 判据：审批卡必须画在 **composer 区域**（Y 在消息流最后一行之下），
// 而不是消息流里的一段——同屏两套「需要审批」是官方明确要避免的。
func renderProductSeats(t *testing.T, snap pluginapi.Snapshot) (*gfx.Canvas, *dsconversation.Plugin, *dscomposer.Plugin) {
	t.Helper()
	conversation := &dsconversation.Plugin{}
	composer := &dscomposer.Plugin{}
	kernel := uikernel.New("gp-real-chain")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, conversation, composer); err != nil {
		t.Fatalf("装配失败: %v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	canvas := &gfx.Canvas{}
	tree := &gfx.Tree{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	env := &pluginapi.Env{
		Theme: theme, Data: snap, Tree: tree, Feat: tree.Feature("gp-real-chain"),
		Canvas: canvas, Bounds: ui.Rect{Width: 1180, Height: 832}, MainWidth: 840,
	}
	env.Has = func(slot pluginapi.Slot) bool { return kernel.Has(slot) }
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), ui.Rect{Width: 1180, Height: 832}, func(f *flex.Frame) {
		f.Column().FillWidth().FillHeight().Children(func() {
			kernel.DrawSlot(f, pluginapi.SlotMainBody, env)
			kernel.DrawSlot(f, pluginapi.SlotMainComposer, env)
		})
	})
	return canvas, conversation, composer
}

// canvasTextGP 找一段文本（返回整条 Text，含几何）。
func canvasTextGP(canvas *gfx.Canvas, value string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value || strings.HasPrefix(text.Value, value) {
			return text, true
		}
	}
	return gfx.Text{}, false
}

// TestRealChainGPLoadOlderPagesBack 真宿主上**向上翻页**取到旧帧并渲染。
//
// 断言四条：
//   - 基线只取尾窗一页（不是从 0 拉到拉完）；
//   - `paging.HasMore` 为真（宿主手里还有更早的）；
//   - 逐页向前翻，最终拿到**宿主全量**帧、seq 严格连续（不丢帧、不重复）；
//   - 最早那条用户消息的正文**真的画到了画布上**（不是只在数据面里）。
func TestRealChainGPLoadOlderPagesBack(t *testing.T) {
	host := startGPHost(t, "allow")
	client, mux := connectRealGUI(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sessionID, err := client.Create(ctx, "", "", "gp-paging")
	if err != nil {
		t.Fatalf("session.create 失败: %v", err)
	}
	t.Logf("真实会话 id=%s（home=%s）", sessionID, host.home)

	// 三轮真模型：每轮 ~80 帧，三轮 > 200 = 一页。
	const rounds = 3
	for i := 0; i < rounds; i++ {
		before := lastSeqOfHistory(t, client, sessionID)
		promptCtx, promptCancel := context.WithTimeout(context.Background(), 120*time.Second)
		_, err := client.Prompt(promptCtx, sessionID, gpTurnPrompt)
		promptCancel()
		if err != nil {
			t.Fatalf("第 %d 轮 session.prompt 失败: %v", i+1, err)
		}
		frames := waitTurnDoneAfter(t, client, sessionID, before, 180*time.Second)
		t.Logf("第 %d 轮收工：seq %d → %d", i+1, before, frames[len(frames)-1].Seq)
	}
	// 宿主全量（对照真值）。
	allCtx, allCancel := context.WithTimeout(context.Background(), 30*time.Second)
	allRes, err := client.History(allCtx, sessionID, 0, 100000, 0)
	allCancel()
	if err != nil {
		t.Fatalf("拉全量 history 失败: %v", err)
	}
	total := len(allRes.Frames)
	if total <= frameHistoryChunk {
		t.Fatalf("真宿主只产生 %d 帧，不足一页（%d）——分页验不了", total, frameHistoryChunk)
	}
	t.Logf("宿主全量 %d 帧（epoch=%d）", total, allRes.JournalEpoch)

	m := newRealModel(client, mux, sessionID)
	m.refreshFrames(sessionID)
	journal := m.frames[sessionID]
	baseline := len(journal.snapshot())
	if baseline != frameHistoryChunk {
		t.Fatalf("基线 = %d 帧，想要尾窗一页 %d（不是全量、也不是从 0 开始）", baseline, frameHistoryChunk)
	}
	if !journal.paging.HasMore {
		t.Fatal("宿主还有更早的帧，paging.HasMore 必须为 true")
	}
	firstBaselineSeq := journal.snapshot()[0].Seq
	t.Logf("基线 %d 帧，最早 seq=%d，HasMore=%v", baseline, firstBaselineSeq, journal.paging.HasMore)

	// 逐页向上翻。
	pages := 0
	for journal.paging.HasMore {
		m.loadOlderFrames(sessionID)
		pages++
		if pages > 20 {
			t.Fatal("翻页没有收敛")
		}
	}
	got := journal.snapshot()
	if len(got) != total {
		t.Fatalf("翻完 = %d 帧，宿主全量 = %d（丢帧）", len(got), total)
	}
	prev := 0
	for i, frame := range got {
		if frame.Seq <= prev {
			t.Fatalf("第 %d 条 seq=%d 不严格大于上一条 %d（乱序或重复）", i, frame.Seq, prev)
		}
		prev = frame.Seq
	}
	t.Logf("向前翻 %d 页：%d 帧、seq [%d..%d] 严格递增", pages, len(got), got[0].Seq, got[len(got)-1].Seq)

	// 真画布：最早那条用户消息必须真的画出来。
	snapCtx, snapCancel := context.WithTimeout(context.Background(), 20*time.Second)
	_ = snapCtx
	m.mu.Lock()
	frames, framesEpoch := m.snapshotFramesLocked(sessionID)
	paging := m.snapshotHistoryPagingLocked(sessionID)
	m.mu.Unlock()
	snapCancel()
	snap := pluginapi.Snapshot{
		Cur: sessionID, Frames: frames, JournalEpoch: framesEpoch,
		History: pluginapi.HistoryPaging{
			Baseline: paging.Baseline, HasMore: paging.HasMore, Partial: paging.Partial,
			Loading: paging.Loading, Error: paging.Error,
		},
	}
	canvas, conversation, _ := renderProductSeats(t, snap)
	rows := conversation.TranscriptEntries()
	if len(rows) == 0 {
		t.Fatal("真帧没有投影出任何消息节点")
	}
	// 最早那条**用户消息**的正文（真模型回显的第一轮 prompt）。
	// 不能取 rows[0]：官方 transcript 的第一行是 system-prompt 披露行
	// （整段系统提示，画布上按省略号截断），拿它做前缀匹配永远失败。
	var oldest dsconversation.TranscriptEntry
	foundUser := false
	for _, entry := range rows {
		if entry.Kind == dsconversation.ChatNodeUser && entry.Data.Text != "" {
			oldest, foundUser = entry, true
			break
		}
	}
	if !foundUser {
		t.Fatalf("真帧里没有用户消息节点：%d 行", len(rows))
	}
	if _, ok := canvasTextGP(canvas, oldest.Data.Text); !ok {
		t.Fatalf("最早那条用户消息没画到画布上：%q（画布前 5 条：%v）",
			oldest.Data.Text, firstN(canvasTextsGP(canvas), 5))
	}
	// 分页行这时应该是「已到会话开头」（HasMore=false 且 Partial=true）。
	if got := conversation.TranscriptEntries(); len(got) != len(rows) {
		t.Fatalf("投影行数不稳定：%d → %d", len(rows), len(got))
	}
	if paging.HasMore {
		t.Fatal("翻到底之后 HasMore 必须为 false")
	}
	if !paging.Partial {
		t.Fatal("Partial 是粘性的：翻到底之后仍应为 true（那一行显示「已到会话开头」）")
	}
	if _, ok := canvasTextGP(canvas, "已到会话开头"); !ok {
		t.Fatalf("画布上没有「已到会话开头」：%v", canvasTextsGP(canvas))
	}
	t.Logf("真画布：最早消息 %q 已渲染；分页行 = 「已到会话开头」", truncateRunes(oldest.Data.Text, 40))
}

// TestRealChainGPApprovalInComposer 真审批 → 卡片在 **composer 区域**、三动作可点、决定后收卡片。
func TestRealChainGPApprovalInComposer(t *testing.T) {
	host := startGPHost(t, "ask")
	client, mux := connectRealGUI(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sessionID, err := client.Create(ctx, "", "", "gp-approval")
	if err != nil {
		t.Fatalf("session.create 失败: %v", err)
	}
	t.Logf("真实会话 id=%s（home=%s，policy=ask）", sessionID, host.home)

	m := newRealModel(client, mux, sessionID)
	m.subscribe(sessionID)
	t.Cleanup(func() {
		m.mu.Lock()
		sub := m.subs[sessionID]
		m.mu.Unlock()
		if sub != nil && sub.cancel != nil {
			sub.cancel()
		}
	})

	promptCtx, promptCancel := context.WithTimeout(context.Background(), 120*time.Second)
	_, err = client.Prompt(promptCtx, sessionID, gpApprovalPrompt)
	promptCancel()
	if err != nil {
		t.Fatalf("session.prompt 失败: %v", err)
	}
	ap := waitApproval(t, m, 120*time.Second)
	if ap.RequestID == "" {
		t.Fatalf("真审批没有 requestId：%+v", ap)
	}
	if len(ap.Options) < 3 {
		t.Fatalf("真宿主 policy=ask 应给 3 个选项（allow_once/allow_always/reject_once），得到 %d：%+v",
			len(ap.Options), ap.Options)
	}
	t.Logf("真审批：requestId=%s tool=%q title=%q options=%d", ap.RequestID, ap.Tool, ap.Title, len(ap.Options))

	// 数据面：真 WS 帧进快照（审批帧也进 journal —— 一个序号域）。
	m.refreshFrames(sessionID)
	m.mu.Lock()
	frames, framesEpoch := m.snapshotFramesLocked(sessionID)
	approvalView := m.approval
	m.mu.Unlock()
	if len(frames) == 0 {
		t.Fatal("真 history 没有拉到任何原始帧")
	}
	snap := pluginapi.Snapshot{Cur: sessionID, Frames: frames, JournalEpoch: framesEpoch}
	if approvalView != nil {
		snap.Approval = &pluginapi.Approval{
			RequestID: approvalView.RequestID,
			Tool:      approvalView.Tool,
			Title:     approvalView.Title,
			Kind:      approvalView.Kind,
			Args:      approvalView.Args,
			Cmd:       approvalView.Cmd,
			Detail:    approvalView.Detail,
			Options:   approvalOptions(approvalView.Options),
		}
	}

	canvas, conversation, _ := renderProductSeats(t, snap)

	// ① 审批卡画在 composer 区域：卡片标题在**消息流最后一行之下**。
	card, ok := canvasTextGP(canvas, "需要审批")
	if !ok {
		t.Fatalf("画布上没有审批卡：%v", canvasTextsGP(canvas))
	}
	var lastRowY float32
	if rows := conversation.OfficialTranscript().Boxes.Rows; len(rows) > 0 {
		lastRowY = rows[len(rows)-1].Box.Rect().Y + rows[len(rows)-1].Box.Rect().Height
	}
	if card.Bounds.Y < lastRowY {
		t.Fatalf("审批卡 Y=%.1f 在消息流最后一行 Y=%.1f 之上 —— 它跑到消息流里去了",
			card.Bounds.Y, lastRowY)
	}
	t.Logf("审批卡 Y=%.1f > 消息流末行 Y=%.1f（在 composer 区域）", card.Bounds.Y, lastRowY)

	// ② transcript 里没有审批节点（同屏两条「需要审批」是明确要避免的）。
	for _, entry := range conversation.TranscriptEntries() {
		if entry.Kind == dsconversation.ChatNodeUnknown && strings.Contains(entry.Data.Raw, "approval") {
			t.Fatalf("审批帧被投影成了 transcript 行：%+v", entry)
		}
	}

	// ③ 三动作可点：宿主给的三个选项都在画布上。
	for _, option := range approvalView.Options {
		if option.Name == "" {
			continue
		}
		if _, ok := canvasTextGP(canvas, option.Name); !ok {
			t.Fatalf("选项 %q 没画出来：%v", option.Name, canvasTextsGP(canvas))
		}
	}

	// ④ 决定：按"拒绝"回传（宿主 200 accepted 才收卡片）。
	rejectID := ""
	for _, option := range approvalView.Options {
		if strings.HasPrefix(option.Kind, "reject") {
			rejectID = option.OptionID
			break
		}
	}
	if rejectID == "" {
		t.Fatalf("宿主没给拒绝选项：%+v", approvalView.Options)
	}
	m.approve(approvalView.RequestID, rejectID)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		pending := m.approval
		m.mu.Unlock()
		if pending == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	m.mu.Lock()
	pending := m.approval
	status := m.status
	m.mu.Unlock()
	if pending != nil {
		t.Fatalf("回传之后审批卡还挂着（requestId=%s）", pending.RequestID)
	}
	if !strings.Contains(status, "已审批") {
		t.Fatalf("状态栏没有审批结果：%q", status)
	}
	// ⑤ 决定之后重画：审批卡消失（不是画一张空卡）。
	snap.Approval = nil
	canvasAfter, _, _ := renderProductSeats(t, snap)
	if _, ok := canvasTextGP(canvasAfter, "需要审批"); ok {
		t.Fatal("决定之后画布上还有审批卡")
	}
	t.Logf("决定后状态 = %q；重画无审批卡", status)
}

// canvasTextsGP 把画布文本列出来（失败诊断用）。
func canvasTextsGP(canvas *gfx.Canvas) []string {
	values := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	return values
}

// firstN 取前 n 条（失败诊断用）。
func firstN(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	return values[:n]
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// 编译期：审批模型常量必须被用到（防止以后删了 prompt 常量却没人发现）。
var _ = gpApprovalModel
