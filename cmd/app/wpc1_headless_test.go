//go:build darwin || windows || linux

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// WP-C1 产品接线验收：真实 Viva hit/click/scroll，经 cmd/app 动作进入 Viva 的
// OS capability。headless driver 记录 OS 请求，不在测试机弹浏览器或污染系统剪贴板。
func TestWPC1HeadlessRealActionsAndTwoSizeEvidence(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{name: "1182x832", width: 1182, height: 832}, {name: "1920x1032", width: 1920, height: 1032}} {
		t.Run(size.name, func(t *testing.T) {
			runWPC1Headless(t, size.width, size.height)
		})
	}
}

func runWPC1Headless(t *testing.T, width, height int) {
	t.Helper()
	var (
		mu      sync.Mutex
		prompts []string
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		text, _ := body["text"].(string)
		mu.Lock()
		prompts = append(prompts, text)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"turnId":"turn-retry-real","accepted":true}`)
	}))
	defer ts.Close()

	file := filepath.Join(t.TempDir(), "真实 文件.txt")
	if err := os.WriteFile(file, []byte("open me"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileURL := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(file)}).String() + "#L24-L30"
	code := "package main\n\nfunc main() {\n\tprintln(\"" + strings.Repeat("long-", 80) + "\")\n}"
	msg := apiclient.Message{Role: "assistant", TurnID: "turn-message", Text: "## 回答\n\n" +
		"[HTTP 链接](https://example.com/docs) 与 [文件 链接](" + fileURL + ")\n\n" +
		"```go title=main.go\n" + code + "\n```\n"}
	msgs := make([]apiclient.Message, 0, 25)
	for i := 0; i < 22; i++ {
		msgs = append(msgs, apiclient.Message{Seq: i + 1, Role: "assistant", Text: fmt.Sprintf("历史消息 %02d", i+1)})
	}
	msgs = append(msgs, msg)
	at := time.Date(2026, 11, 4, 3, 4, 5, 0, time.Local).UnixMilli()
	m := &model{
		cli: apiclient.New(ts.URL), cur: "s-wpc1", msgs: msgs,
		outgoing: map[string][]pluginapi.OutgoingMessage{
			"s-wpc1": {{Key: "local-send-1", State: pluginapi.OutgoingFailed, Text: "失败后重试这句", Error: "injected offline"}},
		},
		msgMeta: map[string]map[string]pluginapi.MessageMetadata{
			"s-wpc1": {pluginapi.MessageKey("s-wpc1", msg, len(msgs)-1): {Time: at, Stopped: true}},
		},
		turnEvents: map[string][]pluginapi.TurnEvent{
			"s-wpc1": {
				{Key: "turn-error-1", Kind: "error", TurnID: "turn-real", Error: "model failed", Time: at},
				{Key: "turn-stopped-1", Kind: "stopped", Time: at},
			},
		},
	}
	m.actions.CopyResult = m.copyText
	m.actions.OpenExternal = m.openExternal
	m.actions.OpenFile = m.openFile
	m.actions.RetrySend = m.retrySend
	m.actions.Notify = m.setStatus

	conversation := &dsconversation.Plugin{}
	kernel := uikernel.New("wpc1-test")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, conversation); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	painter := conversation.Panels()[0]
	tree := &gfx.Tree{}
	feat := tree.Feature("wpc1-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var (
		invalidator app.Invalidator
	)
	// 快照每帧重建并在锁内拷贝：插件拿到的必须是**这一帧的副本**。
	// 直接把 `m.outgoing[m.cur]` 交给绘制线程，dispatchSend 的 goroutine 会
	// 就地改它的元素（发送状态），`-race` 就会报
	// drawOutgoing ↔ dispatchSend——那是测试自己没遵守壳的快照契约。
	snapshot := func() pluginapi.Snapshot {
		m.mu.Lock()
		defer m.mu.Unlock()
		metadata := make(map[string]pluginapi.MessageMetadata, len(m.msgMeta[m.cur]))
		for key, value := range m.msgMeta[m.cur] {
			metadata[key] = value
		}
		return pluginapi.Snapshot{
			Cur:         m.cur,
			Msgs:        append([]apiclient.Message(nil), m.msgs...),
			MessageMeta: metadata,
			TurnEvents:  append([]pluginapi.TurnEvent(nil), m.turnEvents[m.cur]...),
			Outgoing:    append([]pluginapi.OutgoingMessage(nil), m.outgoing[m.cur]...),
		}
	}
	config := app.Config{
		Title: "wpc1-headless", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	// v0.6.0 没有 ID→ScrollState 查表：消息流滚动区由插件交给壳（env.ScrollBox）。
	// 区域每帧都声明，所以拿到的是同一个 ScrollState 对象，可以跨帧读。
	var bodyState *ui.ScrollState
	config.Paint = func(canvas *gfx.Canvas, current ui.Size) {
		bounds := ui.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: snapshot(), Act: m.actions, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: current.Width,
			ScrollBox: func(box ui.Box) { bodyState = box.ScrollState() },
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			painter.Draw(f, env)
		})
		painter.AfterDraw(canvas, env)
	}
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult {
		if result := conversation.HandleScroll(event); result.Handled {
			return result
		}
		return view.Wheel(event)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for range 2 {
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}

	for _, want := range []string{"复制", "折行", "go", "turn-real", "model failed", "已停止", "发送失败", "重试发送", time.UnixMilli(at).Format("15:04")} {
		if !session.HasText(want) {
			t.Fatalf("首帧缺 %q：%q", want, session.Texts())
		}
	}
	captureWPC1(t, session, width, height)

	if err := session.ClickLabel("复制代码"); err != nil {
		t.Fatal(err)
	}
	if got := session.Driver().Clipboard(); got != code {
		t.Fatalf("代码复制原文不同：got=%q want=%q", got, code)
	}
	if !session.HasText("已复制") {
		t.Fatalf("复制成功反馈不可见：%q", session.Texts())
	}

	originalClipboard := setClipboardText
	setClipboardText = func(string) error { return errors.New("clipboard busy") }
	if err := session.ClickLabel("复制代码"); err != nil {
		t.Fatal(err)
	}
	setClipboardText = originalClipboard
	if !session.HasText("复制失败") {
		t.Fatalf("OS 复制失败反馈不可见：%q", session.Texts())
	}

	if err := session.ClickLabel("HTTP 链接"); err != nil {
		t.Fatal(err)
	}
	if opened, ok := session.Driver().LastOpenedURL(); !ok || opened != "https://example.com/docs" {
		t.Fatalf("HTTP 打开动作 = %q, %v", opened, ok)
	}
	if err := session.ClickLabel("文件 链接"); err != nil {
		t.Fatal(err)
	}
	if opened, ok := session.Driver().LastOpenedPath(); !ok || opened != file {
		m.mu.Lock()
		status := m.status
		m.mu.Unlock()
		t.Fatalf("文件打开动作 = %q, %v；期望 %q；fileURL=%q status=%q", opened, ok, file, fileURL, status)
	}

	m.mu.Lock()
	m.mu.Unlock()
	mainState := bodyState
	if mainState == nil || mainState.Max() <= 0 {
		t.Fatalf("测试需要可纵向滚动的消息区：%+v", mainState)
	}
	codeText := findWPC1CodeText(t, session.Snapshot().Texts)
	mainBefore := mainState.Offset
	if err := session.Scroll(gfx.ScrollEvent{X: codeText.Bounds.X + 4, Y: codeText.Bounds.Y + 4, DeltaX: 96, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if mainState.Offset != mainBefore {
		t.Fatalf("代码横滚抢了主滚动：before=%v after=%v", mainBefore, mainState.Offset)
	}
	movedCode := findWPC1CodeText(t, session.Snapshot().Texts)
	if movedCode.Bounds.X >= codeText.Bounds.X {
		t.Fatalf("代码横向滚动未移动正文：before=%+v after=%+v", codeText.Bounds, movedCode.Bounds)
	}
	if err := session.Scroll(gfx.ScrollEvent{X: float32(width) / 2, Y: codeText.Bounds.Y + 4, DeltaY: -120, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if mainState.Offset >= mainBefore {
		t.Fatalf("普通纵向滚轮被代码块吞掉：before=%v after=%v", mainBefore, mainState.Offset)
	}
	if err := session.ClickLabel("切换折行"); err != nil {
		t.Fatal(err)
	}
	if !session.HasText("横滚") {
		t.Fatalf("wrap toggle 未生效：%q", session.Texts())
	}

	mainState.ScrollTo(mainState.Max())
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("重试发送"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		outgoing := append([]pluginapi.OutgoingMessage(nil), m.outgoing[m.cur]...)
		m.mu.Unlock()
		if len(outgoing) == 1 && outgoing[0].State == pluginapi.OutgoingSent && outgoing[0].TurnID == "turn-retry-real" {
			break
		}
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(prompts) != 1 || prompts[0] != "失败后重试这句" {
		t.Fatalf("重试没有发出真实原草稿：%q", prompts)
	}
}

func findWPC1CodeText(t *testing.T, texts []gfx.Text) gfx.Text {
	t.Helper()
	for _, text := range texts {
		if text.Style.Mono && strings.Contains(text.Value, "println") {
			return text
		}
	}
	t.Fatal("找不到实画代码行")
	return gfx.Text{}
}

func captureWPC1(t *testing.T, session *headless.Session, width, height int) {
	t.Helper()
	dir := os.Getenv("WPC1_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("conversation-%dx%d.png", width, height))
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	image, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if image.Bounds().Dx() != width || image.Bounds().Dy() != height {
		t.Fatalf("截图尺寸 = %v，期望 %dx%d", image.Bounds(), width, height)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：%s info=%v err=%v", path, info, err)
	}
	t.Logf("WPC1_CAPTURE path=%s size=%dx%d bytes=%d", path, width, height, info.Size())
}
