//go:build windows

package main

// gq_real_chain_test.go —— **真机验收**：真 `dshd serve` + 真 apiclient + 真画布，
// 让模型回一段**含代码块 + 标题 + 列表**的 markdown，断言块级元素真的画出来了。
//
// 为什么必须真机（而不是只跑 dsconversation 的单测）：单测里的 markdown 是
// 手写的固定字符串，证不了"宿主推来的**这一种**形状（真实模型输出的 markdown
// 带空行、带列表缩进、带围栏 info）能投影成块级节点"。GT2/GT3 都踩过
// "渲染器全绿、真机主区空白/串行"的坑。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_gt3/dshd.exe"
//	go test ./cmd/app -run TestGQRealChain -count=1 -v
//
// 依赖本机 magpie 网关；连不上时**跳过**并说清原因——不拿"没跑到"冒充通过。

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// gqPrompt 是真实 prompt。
//
// 三个标记串**只可能出现在各自的块级元素里**（标题 / 列表项 / 代码块），
// 所以"画布上有它"就是"那个块级元素真的被画出来了"。
//
// 为什么用纯 ASCII 标记：Git Bash 的 curl 会把中文按 cp936 发出去（GT3 §3.4
// 踩过），标记串一旦含中文，"画布上有它"就不再是干净判据。
const (
	gqPrompt = "只回复一段 markdown，不要解释，不要用代码块把整段包起来。内容必须严格包含：\n" +
		"1) 一个二级标题，文字正好是 GQ-HEADING-OK\n" +
		"2) 一个无序列表，恰好两项：GQ-LIST-A 和 GQ-LIST-B\n" +
		"3) 一个 go 围栏代码块，里面只有一行：fmt.Println(\"GQ-CODE-OK\")\n"
	gqHeading = "GQ-HEADING-OK"
	gqListA   = "GQ-LIST-A"
	gqListB   = "GQ-LIST-B"
	gqCode    = `fmt.Println("GQ-CODE-OK")`
)

// gqRenderSeat 用真内核 + 真画布跑一次**完整**产品绘制路径
// （布局 Draw + 覆画 PaintCodeBlocks）。
//
// 与 gt3 的 `renderTranscriptSeat` 的唯一区别就是多调了覆画阶段——
// 代码块的 banner/高亮/横滚视口/复制按钮全在覆画里画，少了这一步
// 画布上只有一行空占位（这正是"必须先布局后覆画"的现场证据）。
func gqRenderSeat(t *testing.T, snap pluginapi.Snapshot) (*gfx.Canvas, *dsconversation.Plugin) {
	t.Helper()
	conversation := &dsconversation.Plugin{}
	kernel := uikernel.New("gq-real-chain")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, conversation); err != nil {
		t.Fatalf("装配失败: %v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	canvas := &gfx.Canvas{}
	tree := &gfx.Tree{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size
	})
	env := &pluginapi.Env{
		Theme: theme, Data: snap, Tree: tree, Feat: tree.Feature("gq-real-chain"),
		Canvas: canvas, Bounds: gfx.Rect{Width: 1180, Height: 832}, MainWidth: 840,
	}
	env.Has = func(slot pluginapi.Slot) bool { return kernel.Has(slot) }
	env.Act.CopyResult = func(string) error { return nil }
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 1180, Height: 832}, func(f ui.C) {
		kernel.DrawSlot(f, pluginapi.Slot("conversation.view"), env)
	})
	conversation.OfficialTranscript().PaintCodeBlocks(canvas, env)
	return canvas, conversation
}

// TestGQRealChainMarkdownBlocks 是**硬验收**：真宿主 + 真模型产出的 markdown，
// 标题 / 列表 / 代码块三种块级元素必须真的画在画布上，且几何成立。
func TestGQRealChainMarkdownBlocks(t *testing.T) {
	host := startRealHost(t)
	client, mux := connectRealGUI(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sessionID, err := client.Create(ctx, "", "", "gq-markdown-blocks")
	if err != nil {
		t.Fatalf("session.create 失败: %v", err)
	}
	t.Logf("真实会话 id=%s（home=%s）", sessionID, host.home)

	promptCtx, promptCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer promptCancel()
	if _, err := client.Prompt(promptCtx, sessionID, gqPrompt); err != nil {
		t.Fatalf("session.prompt 失败: %v", err)
	}

	frames := waitTurnDone(t, client, sessionID, 180*time.Second)
	t.Logf("真实帧 %d 条", len(frames))

	// 数据面：真 `session.history`（不是 mock、不是 load 投影）。
	m := newRealModel(client, mux, sessionID)
	m.refreshFrames(sessionID)
	snapshotFrames, snapshotEpoch := m.snapshotFramesLocked(sessionID)
	if len(snapshotFrames) == 0 {
		t.Fatal("真 history 没有拉到任何原始帧（数据面空）")
	}

	// 绘制：真快照 → 官方 conversation.view 槽 → 真画布（布局 + 覆画）。
	canvas, conversation := gqRenderSeat(t, pluginapi.Snapshot{
		Cur: sessionID, Frames: snapshotFrames, JournalEpoch: snapshotEpoch,
	})

	// ① 投影里必须有带标记的 assistant 正文（标记只可能来自模型回答）。
	var assistantText string
	for _, entry := range conversation.TranscriptEntries() {
		if entry.Kind == dsconversation.ChatNodeAssistantStep && strings.Contains(entry.Data.Text, gqHeading) {
			assistantText = entry.Data.Text
			break
		}
	}
	if assistantText == "" {
		t.Fatalf("投影里没有带 %q 的 assistant 正文", gqHeading)
	}
	t.Logf("真实回答正文 = %q", assistantText)

	// ② 画布上三种块级元素都必须真的画出来。
	if !canvasHas(canvas, gqHeading) {
		t.Fatalf("画布上没有标题 %q；实际文本：%v", gqHeading, canvasTextValues(canvas))
	}
	if !canvasHas(canvas, gqListA) || !canvasHas(canvas, gqListB) {
		t.Fatalf("画布上没有列表项 %q/%q；实际文本：%v", gqListA, gqListB, canvasTextValues(canvas))
	}
	if !canvasHas(canvas, gqCode) {
		t.Fatalf("画布上没有代码块正文 %q；实际文本：%v", gqCode, canvasTextValues(canvas))
	}
	if !canvasHas(canvas, "go") {
		t.Fatalf("画布上没有代码块 banner 的语言标签 go；实际文本：%v", canvasTextValues(canvas))
	}

	// ③ 几何：标题字号来自 derived 阶梯表、代码块等宽、列表有标记槽。
	heading := canvasTextExact(canvas, gqHeading)
	if heading.Style.Size != dsconversation.MarkdownHeadingFor(2).Size {
		t.Errorf("h2 字号 = %v，应为 %v", heading.Style.Size, dsconversation.MarkdownHeadingFor(2).Size)
	}
	code := canvasTextExact(canvas, gqCode)
	if !code.Style.Mono {
		t.Error("代码块正文必须等宽（Mono）")
	}
	if !canvasHas(canvas, dsconversation.MarkdownListBullet) {
		t.Errorf("画布上没有无序列表标记 %q", dsconversation.MarkdownListBullet)
	}

	// ④ 代码块几何：banner + 视口必须真实（覆画阶段跑过）。
	view := conversation.OfficialTranscript()
	if view == nil {
		t.Fatal("transcript 渲染器不存在")
	}
	geometries := 0
	for _, box := range view.CodeBlocks() {
		geometries++
		if box.Geometry.Banner.Height != dsconversation.CodeBlockBannerHeight {
			t.Errorf("banner 高 = %v，应为 %v", box.Geometry.Banner.Height, dsconversation.CodeBlockBannerHeight)
		}
		if box.Geometry.Content.Width <= 0 || box.Geometry.Content.Height <= 0 {
			t.Errorf("代码块视口几何为零：%+v", box.Geometry.Content)
		}
	}
	if geometries == 0 {
		t.Fatal("画布上一个代码块都没有（模型没回围栏，或 placeCodeBlock 没跑）")
	}
}

// canvasTextExact 找**完全等于** value 的文本（长文本会被省略号截断，这里不匹配前缀）。
func canvasTextExact(canvas *gfx.Canvas, value string) gfx.Text {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text
		}
	}
	return gfx.Text{}
}
