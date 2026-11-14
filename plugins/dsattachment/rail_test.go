package dsattachment

// 无头测试：真的画一帧（ui.NewPage + gfx.Canvas），点真的 Hit id，断言画面里的文字与
// 几何。不 mock 绘制层、不绕过按钮直接调方法——点击走的是产品同一条路径
// （与 `plugins/dsdirpicker/browse_test.go` 同一套做法）。
//
// 一个 viva 的硬约束：`ui.Box` 只在**本次 paint** 内有效（`box.go:9-12`），跨帧再读会
// panic。所以每次 draw 完立刻把矩形抄成纯数值，断言只对数值做。
//
// 还有一条本包的规矩：**所有断言都只依据官方能证明的东西**——文案逐字来自
// conversation zh 字典，几何逐值来自 FileCard/AttachmentRail 的 CSS（行号在 rail.go）。

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 假 sink：记录每一次真实调用 ────────────────────────────────────────────

type attachCall struct {
	sessionID string
	id        string
}

type fakeSink struct {
	mu       sync.Mutex
	session  string
	puts     []string
	attaches []attachCall
	// respond 是这一个假宿主的"上传结果"；nil = 默认成功（1536 字节的 txt）。
	respond func(path string) (*apiclient.Attachment, error)
	// gate 非 nil 时 Put 先等它放行（用来把"上传中"这一态钉在画面上）。
	gate chan struct{}
}

func (f *fakeSink) Put(_ context.Context, path string) (*apiclient.Attachment, error) {
	if f.gate != nil {
		<-f.gate
	}
	f.mu.Lock()
	f.puts = append(f.puts, path)
	respond := f.respond
	f.mu.Unlock()
	if respond != nil {
		return respond(path)
	}
	return &apiclient.Attachment{
		ID:       "att-" + strconv.Itoa(len(f.puts)),
		Name:     baseName(path),
		ByteSize: 1536,
		MIME:     "text/plain",
	}, nil
}

func (f *fakeSink) Attach(_ context.Context, sessionID string, a apiclient.Attachment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attaches = append(f.attaches, attachCall{sessionID: sessionID, id: a.ID})
	return nil
}

func (f *fakeSink) CurrentSession() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.session
}

func (f *fakeSink) putPaths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.puts...)
}

func (f *fakeSink) attachCalls() []attachCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]attachCall(nil), f.attaches...)
}

// ── 夹具 ─────────────────────────────────────────────────────────────────

// fixture 是一个设备齐全的无头环境：真主题、真 Tree、真 Canvas。
//
// `run` 换成同步执行：产品路径把 `attachment.put` 丢给 goroutine，测试必须在同一线程
// 看到结算结果，否则断言就是撞运气（同 dsdirpicker 的 fixture）。
func fixture(t *testing.T, sink AttachmentSink) (*Plugin, *pluginapi.Env, *gfx.Tree, *gfx.Canvas) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("attachment-test")
	p := &Plugin{
		theme: theme,
		prims: &dscore.Primitives{Theme: theme},
		run:   func(fn func()) { fn() },
	}
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feat}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	bindInstance(p)
	SetSink(sink)
	t.Cleanup(func() {
		bindInstance(nil)
		SetSink(nil)
	})
	return p, env, tree, canvas
}

// drawOnce 画一帧，并在布局之后跑一次 AfterDraw（壳在 `cmd/app/main.go:1917-1919`
// 就是这么跑的——本包在那里量 rail 宽度决定翻页窗口）。
//
// **先 Reset**：canvas 的图元是累加的，不 Reset 的话"上一帧画过、这一帧不画"的节点
// 会留在画面上，断言就成了假的。
func drawOnce(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, w, h float32) {
	t.Helper()
	canvas.Reset()
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: w, Height: h}, func(f ui.C) { p.draw(f, env) })
	p.afterDraw(canvas, env)
}

// click 走一次真实的点击事件通道：BeginEventPass → 画一帧 → EndEventPass。
func click(t *testing.T, p *Plugin, env *pluginapi.Env, tree *gfx.Tree, canvas *gfx.Canvas, w, h float32, id gfx.ID) {
	t.Helper()
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	drawOnce(t, p, env, canvas, w, h)
	canvas.EndEventPass()
}

// textValues 把这一帧画出来的文字全部列出来（断言失败时给人看现场）。
func textValues(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// hasText 报告画面里有没有这一段文字（完全相等）。
func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

// items 抄一份当前的 rail 条目（不跨帧，纯数值/字符串）。
func (p *Plugin) itemSnapshot() []item {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]item(nil), p.items...)
}

// drop 造一个 drop 相位的系统拖入事件。
func drop(paths ...string) gfx.FileDropEvent {
	return gfx.FileDropEvent{Phase: gfx.FileDropDrop, Paths: paths}
}

// ── 1. 没有 sink → 一个节点都不画，拖入也不消费 ───────────────────────────

func TestNoSinkDrawsNothingAndIgnoresDrop(t *testing.T) {
	p, env, _, canvas := fixture(t, nil)
	drawOnce(t, p, env, canvas, 1180, 700)

	if p.boxes.Rail.Valid() || len(p.boxes.Cards) != 0 || len(p.boxes.Remove) != 0 {
		t.Fatalf("没有 sink 时不该画 rail：rail=%v cards=%d remove=%d",
			p.boxes.Rail.Valid(), len(p.boxes.Cards), len(p.boxes.Remove))
	}
	if got := len(canvas.Texts()); got != 0 {
		t.Fatalf("没有 sink 时画面上不该有文字，实际 %d 段：%v", got, textValues(canvas))
	}
	// 拖入必须"不消费"：Handled=false 时系统会自己处理这些文件（官方 drop-events.ts:71-81
	// 的 `if (canAcceptDrop)` 反面就是什么都不做）。
	if result := HandleFileDrop(drop("/tmp/a.txt")); result.Handled {
		t.Fatal("没有 sink 时 HandleFileDrop 不该消费事件")
	}
	if len(p.itemSnapshot()) != 0 {
		t.Fatalf("没有 sink 时不该出现条目：%v", p.itemSnapshot())
	}
}

// ── 2. 0 条附件 → rail 零节点（官方 railItems.length > 0 才画）────────────

func TestZeroItemsDrawsNoRail(t *testing.T) {
	sink := &fakeSink{session: "s1"}
	p, env, _, canvas := fixture(t, sink)
	drawOnce(t, p, env, canvas, 1180, 700)

	if p.boxes.Rail.Valid() {
		t.Fatal("0 条附件时不该画 rail 外框（官方 ComposerAttachments.tsx:62 的空 rail 不占位）")
	}
	if len(p.boxes.Cards) != 0 {
		t.Fatalf("0 条附件时不该画卡片，实际 %d 张", len(p.boxes.Cards))
	}
	if got := len(canvas.Texts()); got != 0 {
		t.Fatalf("0 条附件时画面上不该有文字，实际 %d 段：%v", got, textValues(canvas))
	}
}

// ── 3. 1 条 ready → 名字 + 字节标签逐字 ──────────────────────────────────

func TestReadyCardShowsNameAndByteLabel(t *testing.T) {
	sink := &fakeSink{session: "s1", respond: func(string) (*apiclient.Attachment, error) {
		return &apiclient.Attachment{ID: "att-1", Name: "报告.TXT", ByteSize: 1536, MIME: "text/plain"}, nil
	}}
	p, env, _, canvas := fixture(t, sink)
	if result := HandleFileDrop(drop("/tmp/报告.TXT")); !result.Handled {
		t.Fatal("有 sink 时 drop 应该被消费")
	}
	drawOnce(t, p, env, canvas, 1180, 700)

	if len(p.boxes.Cards) != 1 {
		t.Fatalf("1 条附件应画出 1 张卡，实际 %d", len(p.boxes.Cards))
	}
	// 名字：官方 `attachment.file.name || t('file.label')`（ComposerAttachments.tsx:73）。
	if !hasText(canvas, "报告.TXT") {
		t.Fatalf("卡片缺文件名；实际文字=%v", textValues(canvas))
	}
	// 字节标签：官方 `[extension, fileSizeText(bytes)].join(' ')`（FileCard.tsx:34-38），
	// 扩展名大写、单位与 B 之间**没有空格**、<10 才带一位小数。
	if !hasText(canvas, "TXT 1.5KB") {
		t.Fatalf("卡片缺 %q 这一行；实际文字=%v", "TXT 1.5KB", textValues(canvas))
	}
	// 尺寸：官方 .card 是 240×64（FileCard.module.css:1-14）。
	card := p.boxes.Cards[0].Rect()
	if card.Width != cardWidth || card.Height != cardHeight {
		t.Fatalf("卡片尺寸=%vx%v，官方是 %vx%v", card.Width, card.Height, cardWidth, cardHeight)
	}
	// × 按钮 18×18（FileCard.module.css:84-102）。
	if len(p.boxes.Remove) != 1 {
		t.Fatalf("1 张卡应有 1 个 × 按钮，实际 %d", len(p.boxes.Remove))
	}
	if size := p.boxes.Remove[0].Rect(); size.Width != removeSize || size.Height != removeSize {
		t.Fatalf("× 按钮尺寸=%vx%v，官方是 %vx%v", size.Width, size.Height, removeSize, removeSize)
	}
}

// ── 4. 3 条 → 3 项；删 1 → 2；删到 0 → 整块消失 ──────────────────────────

func TestRemoveWalksListDownToNothing(t *testing.T) {
	sink := &fakeSink{session: "s1"}
	p, env, tree, canvas := fixture(t, sink)
	HandleFileDrop(drop("/tmp/a.txt", "/tmp/b.txt", "/tmp/c.txt"))
	drawOnce(t, p, env, canvas, 1180, 700)
	if len(p.boxes.Cards) != 3 {
		t.Fatalf("3 条附件应画出 3 张卡，实际 %d", len(p.boxes.Cards))
	}
	keys := p.itemSnapshot()
	if len(keys) != 3 {
		t.Fatalf("应有 3 条 rail 条目，实际 %d", len(keys))
	}

	// 删第 2 条：点的是产品自己的 × 按钮 hit id。
	click(t, p, env, tree, canvas, 1180, 700, p.hitID(env, "attachments.remove."+keys[1].Key))
	drawOnce(t, p, env, canvas, 1180, 700)
	if len(p.boxes.Cards) != 2 {
		t.Fatalf("删 1 条后应剩 2 张卡，实际 %d", len(p.boxes.Cards))
	}
	if gone := p.itemSnapshot(); len(gone) != 2 || gone[0].Key == keys[1].Key || gone[1].Key == keys[1].Key {
		t.Fatalf("删的不是第 2 条：剩 %v", gone)
	}

	// 删到 0：整块消失（rail 外框、卡片、× 全部不画）。
	for _, it := range p.itemSnapshot() {
		click(t, p, env, tree, canvas, 1180, 700, p.hitID(env, "attachments.remove."+it.Key))
	}
	drawOnce(t, p, env, canvas, 1180, 700)
	if len(p.itemSnapshot()) != 0 {
		t.Fatalf("应删空，实际还剩 %v", p.itemSnapshot())
	}
	if p.boxes.Rail.Valid() || len(p.boxes.Cards) != 0 {
		t.Fatalf("删到 0 后整块应消失：rail=%v cards=%d", p.boxes.Rail.Valid(), len(p.boxes.Cards))
	}
	if got := len(canvas.Texts()); got != 0 {
		t.Fatalf("删到 0 后画面上不该有文字，实际 %v", textValues(canvas))
	}
}

// ── 5. Paths=[a,b] → 对 a、b 各调一次 Put，参数原样 ──────────────────────

func TestHandleFileDropPutsEachPathVerbatim(t *testing.T) {
	sink := &fakeSink{session: "s1"}
	p, env, _, canvas := fixture(t, sink)
	result := HandleFileDrop(drop("/tmp/a.txt", "/tmp/b.txt"))
	if !result.Handled {
		t.Fatal("有 sink 且有路径时 drop 应被消费（viva 拿 Handled 告诉系统这里收文件）")
	}
	got := sink.putPaths()
	if len(got) != 2 || got[0] != "/tmp/a.txt" || got[1] != "/tmp/b.txt" {
		t.Fatalf("Put 调用=%v，应逐字是 [/tmp/a.txt /tmp/b.txt] 且各一次", got)
	}
	drawOnce(t, p, env, canvas, 1180, 700)
	if len(p.boxes.Cards) != 2 {
		t.Fatalf("两个路径应得到两张卡，实际 %d", len(p.boxes.Cards))
	}
}

// ── 6. Paths 为空 → 不调 Put、零节点 ─────────────────────────────────────

func TestHandleFileDropIgnoresEmptyPaths(t *testing.T) {
	sink := &fakeSink{session: "s1"}
	p, env, _, canvas := fixture(t, sink)
	if result := HandleFileDrop(drop()); result.Handled {
		t.Fatal("空 Paths 不该被消费（官方 files.length > 0 那道门）")
	}
	if result := HandleFileDrop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, Paths: []string{"", "  "}}); result.Handled {
		t.Fatal("空串/纯空白的 Paths 也不该被消费")
	}
	if got := sink.putPaths(); len(got) != 0 {
		t.Fatalf("空 Paths 时不该调 Put，实际 %v", got)
	}
	if len(p.itemSnapshot()) != 0 {
		t.Fatalf("空 Paths 时不该出现条目：%v", p.itemSnapshot())
	}
	drawOnce(t, p, env, canvas, 1180, 700)
	if len(p.boxes.Cards) != 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("空 Paths 后应零节点：cards=%d texts=%v", len(p.boxes.Cards), textValues(canvas))
	}
}

// ── 7. 宿主 0 字节错误 → 显示宿主原文 ────────────────────────────────────

func TestHostErrorTextIsShownVerbatim(t *testing.T) {
	// 这段原文来自宿主的真实分支：HOST:cmd/dshd/attachments.go:129-131。
	hostText := "附件不能是 0 bytes"
	sink := &fakeSink{session: "s1", respond: func(string) (*apiclient.Attachment, error) {
		return nil, &apiclient.APIError{Code: "bad_request", Message: hostText}
	}}
	p, env, _, canvas := fixture(t, sink)
	HandleFileDrop(drop("/tmp/empty.txt"))
	drawOnce(t, p, env, canvas, 1180, 700)

	if !hasText(canvas, hostText) {
		t.Fatalf("失败卡应显示宿主原文 %q；实际文字=%v", hostText, textValues(canvas))
	}
	if hasText(canvas, zhFileUploadFailed) {
		t.Fatalf("宿主已经给了原文时不该再画通用文案 %q", zhFileUploadFailed)
	}
	if state := p.itemSnapshot()[0].State; state != stateError {
		t.Fatalf("宿主拒绝后应是 error 态，实际 %q", state.key())
	}

	// 宿主没给文字时回退官方那句通用文案（**不自己编中文**）。
	silent := &fakeSink{session: "s1", respond: func(string) (*apiclient.Attachment, error) {
		return nil, &apiclient.APIError{Code: "bad_request"}
	}}
	p2, env2, _, canvas2 := fixture(t, silent)
	HandleFileDrop(drop("/tmp/empty.txt"))
	drawOnce(t, p2, env2, canvas2, 1180, 700)
	if !hasText(canvas2, zhFileUploadFailed) {
		t.Fatalf("没有原文时应回退 %q；实际文字=%v", zhFileUploadFailed, textValues(canvas2))
	}
}

// ── 8. uploading → ready ────────────────────────────────────────────────

func TestUploadingThenReady(t *testing.T) {
	gate := make(chan struct{})
	sink := &fakeSink{session: "s1", gate: gate}
	p, env, _, canvas := fixture(t, sink)
	// 这一条要**异步**：上传卡在 gate 上，才能看到"上传中"这一态。
	p.run = nil

	HandleFileDrop(drop("/tmp/log.txt"))
	drawOnce(t, p, env, canvas, 1180, 700)

	snapshot := p.itemSnapshot()
	if len(snapshot) != 1 {
		t.Fatalf("应有 1 条条目，实际 %d", len(snapshot))
	}
	// 官方三态之一：'uploading'（FileCard.tsx:18-19），meta 行是 file.uploading。
	if snapshot[0].State != stateUploading {
		t.Fatalf("上传在飞时状态应是 %q，实际 %q", stateUploading.key(), snapshot[0].State.key())
	}
	if !hasText(canvas, zhFileUploading) {
		t.Fatalf("上传中应显示 %q；实际文字=%v", zhFileUploading, textValues(canvas))
	}
	if hasText(canvas, "TXT 1.5KB") {
		t.Fatal("还没上传完就显示字节标签，等于假装已就绪")
	}

	close(gate)
	waitFor(t, func() bool {
		items := p.itemSnapshot()
		return len(items) == 1 && items[0].State == stateReady
	})
	drawOnce(t, p, env, canvas, 1180, 700)
	if hasText(canvas, zhFileUploading) {
		t.Fatalf("已就绪后不该还写着 %q", zhFileUploading)
	}
	if !hasText(canvas, "TXT 1.5KB") {
		t.Fatalf("就绪后应显示 %q；实际文字=%v", "TXT 1.5KB", textValues(canvas))
	}
}

// waitFor 轮询一个条件（只用在异步落地的断言上，不用于产品逻辑）。
func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("等待异步上传落地超时")
}

// ── AttachReady：提交前把就绪附件按会话挂上去（宿主 session.attachment）──

func TestAttachReadyAttachesReadyItemsOnly(t *testing.T) {
	sink := &fakeSink{session: "s1"}
	p, env, _, canvas := fixture(t, sink)
	HandleFileDrop(drop("/tmp/a.txt", "/tmp/b.txt"))
	drawOnce(t, p, env, canvas, 1180, 700)
	if err := AttachReady(context.Background()); err != nil {
		t.Fatalf("AttachReady 不该失败：%v", err)
	}
	calls := sink.attachCalls()
	if len(calls) != 2 {
		t.Fatalf("两条就绪附件应各挂一次，实际 %v", calls)
	}
	for _, call := range calls {
		if call.sessionID != "s1" {
			t.Fatalf("挂载应带当前会话 s1，实际 %q", call.sessionID)
		}
	}
	// 没有当前会话时**什么都不做**（不是错误）：壳可能还没选会话。
	sink.mu.Lock()
	sink.session = ""
	sink.mu.Unlock()
	if err := AttachReady(context.Background()); err != nil {
		t.Fatalf("没有当前会话时不该报错：%v", err)
	}
	if got := len(sink.attachCalls()); got != 2 {
		t.Fatalf("没有当前会话时不该再挂，实际累计 %d 次", got)
	}
}

// ── 翻页窗口：放不下的卡也必须点得到（不能出现"看不见也删不掉"）──────────

func TestPagingKeepsEveryCardReachable(t *testing.T) {
	sink := &fakeSink{session: "s1"}
	p, env, tree, canvas := fixture(t, sink)
	HandleFileDrop(drop("/tmp/a.txt", "/tmp/b.txt", "/tmp/c.txt"))
	drawOnce(t, p, env, canvas, 560, 700) // 560px 只放得下 2 张 240px 的卡
	drawOnce(t, p, env, canvas, 560, 700) // 第二帧用 AfterDraw 量到的窗口

	if len(p.boxes.Cards) != 2 {
		t.Fatalf("560px 一页应放 2 张卡，实际 %d", len(p.boxes.Cards))
	}
	if len(p.boxes.Arrows) != 1 {
		t.Fatalf("第一页只该有「下一页」箭头（官方 edges.right），实际 %d 枚", len(p.boxes.Arrows))
	}
	click(t, p, env, tree, canvas, 560, 700, p.hitID(env, "attachments.scroll-right"))
	drawOnce(t, p, env, canvas, 560, 700)
	if len(p.boxes.Cards) != 1 {
		t.Fatalf("第二页应只剩 1 张卡，实际 %d", len(p.boxes.Cards))
	}
	if len(p.boxes.Arrows) != 1 {
		t.Fatalf("第二页只该有「上一页」箭头，实际 %d 枚", len(p.boxes.Arrows))
	}
	if !hasText(canvas, "c.txt") {
		t.Fatalf("第二页应看到第三张卡 c.txt；实际文字=%v", textValues(canvas))
	}
}

// ── 官方几何常量逐条钉住（改动必须对着 CSS 行号改）──────────────────────

func TestOfficialGeometryConstants(t *testing.T) {
	cases := map[string]struct{ got, want float32 }{
		// OFF:ComposerAttachments.module.css:1-7
		"railPadTop":     {railPadTop, 2},
		"railPadX":       {railPadX, 10},
		"railPullBottom": {railPullBottom, -6},
		// OFF:AttachmentRail.module.css:6-9 / :25-28
		"itemGap":    {itemGap, 10},
		"itemHeight": {itemHeight, 64},
		// OFF:FileCard.module.css:1-14
		"cardWidth":  {cardWidth, 240},
		"cardHeight": {cardHeight, 64},
		"cardPadX":   {cardPadX, 12},
		"cardGap":    {cardGap, 10},
		"cardBorder": {cardBorder, 0.5},
		// OFF:FileCard.module.css:20-27 / :29-37
		"iconSize":    {iconSize, 28},
		"spinnerSize": {spinnerSize, 20},
		"spinnerRing": {spinnerRing, 2},
		// OFF:FileCard.module.css:43-49 / :61-69 / :71-78
		"bodyPadY":     {bodyPadY, 8},
		"nameFontSize": {nameFontSize, 14},
		"nameLineH":    {nameLineH, 22},
		"nameWeight":   {nameWeight, 500},
		"metaFontSize": {metaFontSize, 12},
		"metaLineH":    {metaLineH, 15},
		// OFF:FileCard.module.css:84-102
		"removeTop":   {removeTop, 6},
		"removeRight": {removeRight, 6},
		"removeSize":  {removeSize, 18},
		// OFF:FileCard.module.css:142-151 / :153-160
		"progressInsetX":       {progressInsetX, 12},
		"progressBottom":       {progressBottom, 5},
		"progressHeight":       {progressHeight, 2},
		"progressRadius":       {progressRadius, 1},
		"progressWidthPercent": {progressWidthPercent, 35},
		// OFF:AttachmentRail.module.css:90-108 / :116-123
		"arrowSize":  {arrowSize, 24},
		"arrowInset": {arrowInset, 4},
	}
	for name, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s=%v，官方源=%v", name, tc.got, tc.want)
		}
	}
}

// ── 官方格式化函数逐条对齐（fileSizeText / fileExtension）────────────────

func TestFileSizeTextMatchesOfficial(t *testing.T) {
	// 官方 primitives 的 fileSizeText 逐行照抄（见 rail.go 的注释）：
	// <1024 → "NB"；<10 单位 → 一位小数；否则整数；单位不带空格。
	cases := map[int64]string{
		0:                  "0B",
		1:                  "1B",
		1023:               "1023B",
		1024:               "1.0KB",
		1536:               "1.5KB",
		10240:              "10KB", // 10/1024=10 → 不取小数，Math.round
		1024 * 1024:        "1.0MB",
		1024 * 1024 * 15:   "15MB",
		1024 * 1024 * 1024: "1.0GB",
	}
	for in, want := range cases {
		if got := fileSizeText(in); got != want {
			t.Errorf("fileSizeText(%d)=%q，官方是 %q", in, got, want)
		}
	}
}

func TestExtensionTextMatchesOfficial(t *testing.T) {
	cases := map[string]string{
		// basename 最后一个点之后那段，全大写，最多 8 个字符（FileCard.tsx:33）
		"report.txt":               "TXT",
		"C:\\tmp\\照片.JPG":          "JPG",
		"/tmp/no-extension":        "",
		"/tmp/trailing.":           "",
		"/tmp/a.verylongextension": "VERYLONG", // 截到 8 个字符
	}
	for in, want := range cases {
		if got := extensionText(in); got != want {
			t.Errorf("extensionText(%q)=%q，官方是 %q", in, got, want)
		}
	}
	if got := glyphOf("report.txt"); got != "TXT" {
		t.Errorf("图标槽文字=%q，应是扩展名（官方那里是 SVG sprite，本仓用文字顶替）", got)
	}
	if got := glyphOf("a.verylongextension"); got != "VERY" {
		t.Errorf("图标槽只放得下 %d 个字符，实际 %q", glyphMaxLetters, got)
	}
}

// ── 三条宿主通道的 wire 形状（真 HTTP，不解码靠猜）────────────────────────

func TestAttachmentWireShape(t *testing.T) {
	var putPath string
	var attachBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/attachment.put":
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				t.Errorf("attachment.put 请求不是合法 JSON：%v", err)
			}
			putPath = in.Path
			// 宿主的真实形状：HOST:internal/transport/server.go:861
			_, _ = io.WriteString(w, `{"attachment":{"id":"att-1","name":"a.txt",`+
				`"byteSize":12,"mime":"text/plain","sha256":"abc"}}`)
		case "/api/attachment.get":
			// 宿主的真实形状：HOST:internal/transport/server.go:890-893
			_, _ = io.WriteString(w, `{"attachment":{"id":"att-1","name":"a.txt",`+
				`"byteSize":2,"mime":"text/plain","sha256":"abc"},"dataBase64":"aGk="}`)
		case "/api/session.attachment":
			if err := json.Unmarshal(raw, &attachBody); err != nil {
				t.Errorf("session.attachment 请求不是合法 JSON：%v", err)
			}
			_, _ = io.WriteString(w, `{"attachment":{"attachmentId":"att-1",`+
				`"mediaType":"image/png","bytes":2,"width":1,"height":1},"data":"aGk="}`)
		default:
			t.Errorf("意外的路径 %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := apiclient.New(server.URL)
	ctx := context.Background()

	att, err := client.AttachmentPut(ctx, "/tmp/a.txt")
	if err != nil {
		t.Fatalf("AttachmentPut 失败：%v", err)
	}
	if putPath != "/tmp/a.txt" {
		t.Fatalf("attachment.put 的 path=%q，应原样发绝对路径", putPath)
	}
	if att.ID != "att-1" || att.ByteSize != 12 || att.MIME != "text/plain" || att.SHA256 != "abc" {
		t.Fatalf("attachment.put 解出来不对：%+v", att)
	}

	gotAtt, data, err := client.AttachmentGet(ctx, "att-1")
	if err != nil {
		t.Fatalf("AttachmentGet 失败：%v", err)
	}
	if string(data) != "hi" {
		t.Fatalf("dataBase64 没解成原始字节：%q", data)
	}
	if gotAtt.ID != "att-1" {
		t.Fatalf("attachment.get 的元数据不对：%+v", gotAtt)
	}

	if err := client.SessionAttach(ctx, "s1", apiclient.Attachment{
		ID: "att-1", Name: "a.png", MIME: "image/png", ByteSize: 2, Width: 1, Height: 1,
	}); err != nil {
		t.Fatalf("SessionAttach 失败：%v", err)
	}
	// 入参形状照 HOST:internal/transport/handlers_session_official.go:1509-1526。
	if attachBody["sessionId"] != "s1" {
		t.Fatalf("sessionId 没发出去：%v", attachBody)
	}
	ref, ok := attachBody["attachment"].(map[string]any)
	if !ok {
		t.Fatalf("attachment 不是对象：%v", attachBody["attachment"])
	}
	for key, want := range map[string]any{
		"attachmentId": "att-1",
		"mediaType":    "image/png",
		"bytes":        float64(2),
		"width":        float64(1),
		"height":       float64(1),
		"name":         "a.png",
	} {
		if ref[key] != want {
			t.Errorf("attachment.%s=%v，应是 %v", key, ref[key], want)
		}
	}
	if _, has := ref["originalDimensions"]; has {
		t.Error("originalDimensions 不该发：本仓没有原图尺寸这份真值（宿主也只解码不校验）")
	}

	// 本地校验：空路径/空 id 不发出请求。
	if _, err := client.AttachmentPut(ctx, "  "); !errors.Is(err, apiclient.ErrAttachmentPath) {
		t.Fatalf("空 path 应本地报 ErrAttachmentPath，实际 %v", err)
	}
	if _, _, err := client.AttachmentGet(ctx, ""); err == nil {
		t.Fatal("空 id 应本地报错")
	}
}

// ── sink 的 nil 语义：壳收回能力后 rail 立刻消失 ─────────────────────────

func TestSetSinkNilHidesRail(t *testing.T) {
	sink := &fakeSink{session: "s1"}
	p, env, _, canvas := fixture(t, sink)
	HandleFileDrop(drop("/tmp/a.txt"))
	drawOnce(t, p, env, canvas, 1180, 700)
	if len(p.boxes.Cards) != 1 {
		t.Fatalf("有 sink 时应画 1 张卡，实际 %d", len(p.boxes.Cards))
	}
	SetSink(nil)
	drawOnce(t, p, env, canvas, 1180, 700)
	if p.boxes.Rail.Valid() || len(canvas.Texts()) != 0 {
		t.Fatalf("收回 sink 后应整块不画：cards=%d texts=%v", len(p.boxes.Cards), textValues(canvas))
	}
	if strings.TrimSpace(zhDropTitle) == "" || strings.TrimSpace(zhDropBlocked) == "" {
		// 这两条官方文案读到了但**没有可见落点**（悬停层没画，见 rail.go 包注释）；
		// 留一条断言，保证它们不会在重构里被悄悄删掉又被"发明"一遍。
		t.Fatal("官方 dropTitle/dropBlocked 文案不该丢")
	}
}
