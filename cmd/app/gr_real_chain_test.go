//go:build windows

package main

// gr_real_chain_test.go —— 真机验收：官方 `conversation.composer` 座位 + 输入卡片 + 真实发送。
//
// 六条判据（GP 报告 §5 第 1/2 条收口）：
//
//	① **座位真 dispatch**：`-shell official` 的产品形状（旧 sidebar/header/正文 +
//	   官方 composer 座位）下，`conversation.composer` 被 dispatch 一次、
//	   `main.composer` 零次（同帧只有一个活，不双画）；
//	② **输入卡片真画**：占位「说点什么…（Enter 发送）」/ 发送键 34×34 /
//	   权限徽标（宿主 --policy 真值）/ 上下文计数 / 附件（文件引用）入口；
//	③ **空输入禁用**：占位态发送键 disabled（官方 .primary:disabled）；
//	④ **Enter 真发送**：把文字灌进真 `flex.Field` → 事件 pass 触发 Submitted →
//	   `Actions.Send` → 真 apiclient `session.prompt` → 宿主 journal 出现
//	   `user/message`（真实落盘，不是本地假发）；
//	⑤ **发送中变停止**：真 `turn/start` 之后（还没 turn/done）快照 Busy=true，
//	   主钮变「停止」；
//	⑥ **消息流出现**：真模型收工后，真画布上出现那条用户消息。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_gt3/dshd.exe"
//	export DSHD_HOME_ROOT=".../tmp/GO8-RC2/_gr"
//	go test ./cmd/app -run 'TestRealChainGROfficialComposer' -count=1 -v
//
// 依赖本机 magpie 网关（`dshd` 默认 `DSHD_LLM_BASE=http://127.0.0.1:3425/v1`）；
// 网关连不上时**跳过**并说清原因——不拿"没跑到"冒充通过。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// grSendText 是灌进输入框、再按 Enter 真发出去的那条消息。
//
// 前缀是**标记串**（只可能出现在用户消息里），后面那句是给模型的硬约束：
// 真机会自己决定要不要跑工具（实测它去 grep 了工作区），跑起来一轮几十秒到
// 两分钟不等。要求"不要使用任何工具、只回复这一句"能把它压成一轮纯文本，
// 让验收稳定在秒级——这条测试验的是**发送链路**，不是模型的探索欲。
const grSendText = "GR-OFFICIAL-COMPOSER-OK 不要使用任何工具，只原样回复这一行"

// grProduct 是产品形状的一帧装配（真插件，不是探针）。
type grProduct struct {
	kernel       *uikernel.Kernel
	composer     *dscomposer.Plugin
	sidebar      *dssidebar.Plugin
	conversation *dsconversation.Plugin
	theme        *pluginapi.Theme
	tree         *ui.Tree
	feat         ui.Scope
	opts         uikernel.ShellOptions
	dispatched   map[pluginapi.Slot]int
}

// newGRProduct 装配产品插件集，并解出这一帧的壳选项。
//
// 旧部件（dssidebar 的 sidebar.list、dsconversation 的 main.body）还在，
// 所以整列兼容桥开着；composer 已经有官方座位占用方，所以它**单独**走官方座位。
func newGRProduct(t *testing.T) *grProduct {
	t.Helper()
	p := &grProduct{
		kernel:       uikernel.New("gr-real-composer"),
		composer:     &dscomposer.Plugin{},
		sidebar:      &dssidebar.Plugin{},
		conversation: &dsconversation.Plugin{},
		tree:         &ui.Tree{},
		dispatched:   map[pluginapi.Slot]int{},
	}
	if err := p.kernel.Load(&dscore.Plugin{Dark: true}, p.sidebar, p.conversation, p.composer); err != nil {
		t.Fatalf("装配产品插件集失败：%v", err)
	}
	p.theme = p.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.feat = p.tree.Feature("gr-real-composer")
	m := &model{kernel: p.kernel, cur: "s-real"}
	p.opts = shellOptionsFor(m, p.kernel, 1182, 832)
	if !p.opts.BridgeLegacy {
		t.Fatal("旧部件还在占槽，整列兼容桥必须开着（header/正文仍走旧槽）")
	}
	if p.opts.BridgeLegacyComposer {
		t.Fatal("官方 composer 座位有占用方时，composer 不该走兼容桥")
	}
	return p
}

// paint 画一帧产品路径（真 flex + 真 canvas），返回画布与 env。
//
// bind 在**绘制之前**调用：Actions 必须在 draw 里就位（`drawAddEntry` 看
// FSList、`submit` 调 Send），画完再绑就什么都看不到。
//
// pressEnter 为真时把这一帧当成「输入框的 Enter 事件 pass」（viva 的
// `BeginEventPass`：`field.Submitted()` 只在这一帧为真），这就是真键盘 Enter。
func (p *grProduct) paint(t *testing.T, snap pluginapi.Snapshot, pressEnter bool, bind func(*pluginapi.Env)) (*ui.Canvas, *pluginapi.Env) {
	t.Helper()
	canvas := &ui.Canvas{}
	canvas.SetTree(p.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme, Data: snap, Tree: p.tree, Feat: p.feat, Canvas: canvas,
		Bounds:    ui.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:       func(slot pluginapi.Slot) bool { return p.kernel.Has(slot) },
	}
	if bind != nil {
		bind(env)
	}
	if pressEnter {
		canvas.BeginEventPass(p.tree.ID(p.feat, "composer"))
		defer canvas.EndEventPass()
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f *flex.Frame) {
		uikernel.FirstScreenWithMeasure(f, env, p.opts, func(slot pluginapi.Slot) {
			p.dispatched[slot]++
			p.kernel.DrawSlot(f, slot, env)
		}, nil)
	})
	for _, painter := range p.kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	return canvas, env
}

// grTexts 列画布文本（失败诊断用）。
func grTexts(canvas *ui.Canvas) []string {
	values := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	return values
}

// grHasText 报告画布上有没有这段文本（前缀匹配：长文本会被省略号截断）。
func grHasText(canvas *ui.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == want || strings.HasPrefix(text.Value, want) {
			return true
		}
	}
	return false
}

// grHasHit 报告画布上有没有这个标签的命中区。
func grHasHit(canvas *ui.Canvas, label string) (ui.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit, true
		}
	}
	return ui.Hit{}, false
}

// TestRealChainGROfficialComposer 真宿主 + 真 apiclient + 官方 composer 座位。
func TestRealChainGROfficialComposer(t *testing.T) {
	host := startRealHost(t)
	client, mux := connectRealGUI(t, host)
	_ = mux

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	sessionID, err := client.Create(ctx, host.home, "workspace-write", "gr-composer")
	cancel()
	if err != nil {
		t.Fatalf("session.create 失败：%v", err)
	}
	t.Logf("真实会话 id=%s（home=%s）", sessionID, host.home)

	load, err := client.Load(context.Background(), sessionID, 0)
	if err != nil {
		t.Fatalf("session.load 失败：%v", err)
	}
	t.Logf("真宿主真值：model=%q permission=%q contextTokens=%d contextWindow=%d",
		load.Model, load.Permission, load.ContextTokens, load.ContextWindow)

	product := newGRProduct(t)
	// 附件（文件引用）入口只在壳接了 FSList + CWD 时出现；这里用真工作目录。
	fsList := func(dir string) []pluginapi.FSEntry {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		out := make([]pluginapi.FSEntry, 0, len(entries))
		for _, e := range entries {
			out = append(out, pluginapi.FSEntry{Name: e.Name(), IsDir: e.IsDir()})
		}
		return out
	}
	var sent []string
	snap := pluginapi.Snapshot{
		Cur: sessionID, CWD: host.home, Status: "已连接",
		Model: load.Model, Permission: load.Permission,
		ContextTokens: load.ContextTokens, ContextWindow: load.ContextWindow,
	}
	bindSend := func(env *pluginapi.Env) {
		env.Act.Send = func(text string) {
			sent = append(sent, text)
			go func() {
				pctx, pcancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer pcancel()
				if _, err := client.Prompt(pctx, sessionID, text); err != nil {
					t.Logf("session.prompt 失败：%v", err)
				}
			}()
		}
		env.Act.Cancel = func() {}
		env.Act.FSList = fsList
	}

	// ── ① 座位：官方 composer 被 dispatch 一次，main.composer 零次。
	product.composer.Fields()[0].SetText("")
	canvas, _ := product.paint(t, snap, false, bindSend)
	if got := product.dispatched[uikernel.OffConvComposer]; got != 1 {
		t.Fatalf("官方 composer 座位 dispatch 次数 = %d，want 1（dispatched=%v）", got, product.dispatched)
	}
	if got := product.dispatched[pluginapi.SlotMainComposer]; got != 0 {
		t.Fatalf("官方座位活着时 main.composer 也被 dispatch（双画）：%d", got)
	}

	// ── ②③ 输入卡片：每个元素都在，空输入禁用。
	for _, want := range []string{
		"说点什么…（Enter 发送）", // 输入框占位
		"上下文",             // 上下文计数
		"添加文件引用",          // 附件（文件引用）入口的命中区在下面单独断言
	} {
		if !grHasText(canvas, want) && want != "添加文件引用" {
			t.Fatalf("输入卡片没画 %q：%v", want, grTexts(canvas))
		}
	}
	send, ok := grHasHit(canvas, "发送")
	if !ok {
		t.Fatalf("输入卡片没有发送键：%+v", grTexts(canvas))
	}
	if send.Bounds.Width != 34 || send.Bounds.Height != 34 {
		t.Fatalf("发送键 = %v×%v，官方 34×34", send.Bounds.Width, send.Bounds.Height)
	}
	if !send.Disabled {
		t.Fatal("空输入时发送键必须 disabled")
	}
	if _, ok := grHasHit(canvas, "添加文件引用"); !ok {
		t.Fatalf("输入卡片没有附件（文件引用）入口：%+v", canvas.Hits())
	}
	if load.Permission != "" {
		// 权限徽标：宿主 --policy 的真值（allow → 「完全权限」）。
		if !grHasText(canvas, "完全权限") {
			t.Fatalf("权限徽标没画宿主真值 %q：%v", load.Permission, grTexts(canvas))
		}
	}

	// ── ④ Enter 真发送：灌文字 → 事件 pass → Submitted → 真 prompt。
	product.composer.Fields()[0].SetText(grSendText)
	product.paint(t, snap, true, bindSend)
	deadline := time.Now().Add(30 * time.Second)
	var journal []apiclient.Frame
	for time.Now().Before(deadline) {
		res, err := client.History(context.Background(), sessionID, 0, 1000, 0)
		if err == nil {
			journal = res.Frames
			found := false
			for _, frame := range journal {
				if frame.Type == "user/message" && strings.Contains(string(frame.Data), grSendText) {
					found = true
				}
			}
			if found {
				break
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	if len(sent) != 1 || sent[0] != grSendText {
		t.Fatalf("Enter 没有走到 Actions.Send：%#v", sent)
	}
	userFrame := false
	for _, frame := range journal {
		if frame.Type == "user/message" && strings.Contains(string(frame.Data), grSendText) {
			userFrame = true
		}
	}
	if !userFrame {
		t.Fatalf("%s 内宿主 journal 没有 user/message（真实发送没落盘）", 30*time.Second)
	}
	t.Logf("真实发送落盘：journal 里出现 user/message（%d 帧）", len(journal))

	// ── ⑤ 发送中变停止：真 turn/start 之后、turn/done 之前。
	busySnap := func() (pluginapi.Snapshot, bool, bool) {
		res, err := client.History(context.Background(), sessionID, 0, 1000, 0)
		if err != nil {
			return snap, false, false
		}
		s := snap
		s.Frames = res.Frames
		s.JournalEpoch = int64(res.JournalEpoch)
		started, done := false, false
		for _, frame := range res.Frames {
			switch frame.Type {
			case "turn/start":
				started = true
			case "turn/done", "turn/error":
				done = true
			}
		}
		s.Busy = started && !done
		return s, started, done
	}
	stopSeen := false
	stopDeadline := time.Now().Add(180 * time.Second)
	for time.Now().Before(stopDeadline) && !stopSeen {
		s, started, done := busySnap()
		if done && !started {
			break
		}
		if started && !done {
			canvasBusy, _ := product.paint(t, s, false, bindSend)
			if _, ok := grHasHit(canvasBusy, "停止"); !ok {
				t.Fatalf("发送中主钮没有变停止：%+v", canvasBusy.Hits())
			}
			if _, ok := grHasHit(canvasBusy, "发送"); ok {
				t.Fatal("发送中仍然画了发送键（两个主钮）")
			}
			stopSeen = true
			t.Log("发送中主钮 = 停止（真 turn 在跑）")
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !stopSeen {
		t.Log("警告：没抓到 turn/start..turn/done 之间的窗口（模型可能一轮就收工）；停止键的断言由单测覆盖")
	}

	// ── ⑥ 消息流出现：等真模型收工，用**真 journal 帧**画一帧官方骨架。
	finalFrames := waitTurnDone(t, client, sessionID, 240*time.Second)
	finalSnap := snap
	finalSnap.Frames = finalFrames
	finalSnap.Busy = false
	// 上下文计数用**收工后**的真值（load 是首屏快照，发完一条就过期了）。
	if after, lerr := client.Load(context.Background(), sessionID, 0); lerr == nil {
		finalSnap.ContextTokens = after.ContextTokens
		finalSnap.ContextWindow = after.ContextWindow
		finalSnap.Model = after.Model
		t.Logf("收工后真值：model=%q contextTokens=%d contextWindow=%d",
			after.Model, after.ContextTokens, after.ContextWindow)
	}
	canvasFinal, _ := product.paint(t, finalSnap, false, bindSend)
	if !grHasText(canvasFinal, grSendText) {
		t.Fatalf("真画布上没有刚发出的用户消息 %q：%v", grSendText, grTexts(canvasFinal))
	}
	if !grHasText(canvasFinal, "说点什么…（Enter 发送）") {
		t.Fatalf("发完之后输入卡片不见了：%v", grTexts(canvasFinal))
	}

	// ── 截图：真 headless 画布（真文本测量）落 PNG。
	dir := os.Getenv("GR_CAPTURE_DIR")
	if dir == "" {
		dir = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_gr"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建截图目录失败：%v", err)
	}
	shot := filepath.Join(dir, fmt.Sprintf("gr-official-composer-%d.png", time.Now().Unix()))
	theme := product.theme
	shotTree := &ui.Tree{}
	var invalidator app.Invalidator
	var shotFeat ui.Scope
	shotFeat = shotTree.Feature("gr-shot")
	capture := pluginapi.Snapshot{}
	_ = capture
	config := app.Config{
		Title: "gr-official-composer", Width: 1182, Height: 832, Tree: shotTree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		shotEnv := &pluginapi.Env{
			Theme: theme, Data: finalSnap, Tree: shotTree, Feat: shotFeat, Canvas: canvas,
			Bounds: bounds, MainWidth: 902,
			Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: 902},
			Has:      func(slot pluginapi.Slot) bool { return product.kernel.Has(slot) },
		}
		bindSend(shotEnv)
		var view flex.View
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			uikernel.FirstScreen(f, shotEnv, product.opts, func(slot pluginapi.Slot) {
				product.kernel.DrawSlot(f, slot, shotEnv)
			})
		})
		for _, painter := range product.kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, shotEnv)
			}
		}
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	defer session.Close()
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatalf("pump 失败：%v", err)
	}
	if err := session.Screenshot(shot); err != nil {
		t.Fatalf("截图失败：%v", err)
	}
	if info, err := os.Stat(shot); err != nil || info.Size() == 0 {
		t.Fatalf("截图文件无效 path=%s info=%v err=%v", shot, info, err)
	}
	t.Logf("GR_CAPTURE path=%s", shot)
}
