//go:build windows

package main

// hj_usage_dialog_real_chain_test.go —— **真机验收**：宿主真 `tokenUsage` 四桶 →
// 真 apiclient → 产品接线（`wave3Wiring` → `Snapshot.TokenUsage`）→ 状态栏用量
// 胶囊 → 点开官方 **Token 用量** 弹窗，四桶逐字等于 `session.projections` 里的真值。
//
// 判据（task-59 §任务 5）：
//
//	① 真 `dshd serve`（临时 home + `--addr 127.0.0.1:0`）+ 真 apiclient；
//	② 真跑一轮**含工具调用**的 prompt → 宿主真写 `assistant/message.usage`；
//	③ `session.projections` 的 `tokenUsage` 四桶 → 产品接线 → 胶囊读数
//	   `{compact total} tok · 缓存命中 p%`；
//	④ 点用量胶囊 → 弹窗四桶明细 = 投影里的四个数（用 `Values` 里的**原始 JSON**
//	   独立读一遍，不经过被测结构体）+ 标题值 = exact total；
//	⑤ 交叉一致：`tokenUsage.outputTokens` 与 `sessionStats.decodeTokens` **逐字相等**
//	   （两个投影折的是同一批 provider 上报）；
//	⑥ 空会话（没 prompt）：`tokenUsage` 键**在**但四桶全 0 → 读数 `0 tok`、
//	   没有可点入口（官方 gate）——这是"0 与未知"在真机上的可观测差别；
//	⑦ 截图三张（收起态 / 展开态 / 空会话 0 态）。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN="C:/Users/Administrator/AppData/Local/Temp/hj-bin/dshd.exe"
//	export DSHD_HOME_ROOT=".../tmp/GO8-RC2/_hj/home"
//	export HJ_CAPTURE_DIR=".../tmp/GO8-RC2/_hj/shots"
//	go test ./cmd/app -run 'TestRealChainHJUsageDialog' -count=1 -v
//
// 依赖本机 magpie 网关（`dshd` 默认 `DSHD_LLM_BASE=http://127.0.0.1:3425/v1`）；
// 网关连不上时**跳过**并说清原因——不拿"没跑到"冒充通过。

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

const (
	// hjRealPrompt 让真模型调一次 bash 工具：标记串只可能来自工具输出。
	hjRealPrompt = "只做两件事，不要解释：\n1) 用 bash 工具执行 echo HJ-TOOL-OK\n2) 最后单独一行回复：HJ-USAGE-DIALOG-OK"
	// hjUsagePillHit 是 dsstatusbar 里用量胶囊的交互名（与插件内常量同名）。
	hjUsagePillHit = "usage-pill"
	// hjTextPrompt 是"纯文本一轮"的 prompt（不调工具）：它是 outputTokens 与
	// decodeTokens 逐字相等的判据会话（工具步不流文本 → 没有首 token）。
	hjTextPrompt = "只做一件事，不要解释：最后单独一行回复：HJ-TEXT-ONLY-OK"
	// 官方 zh 文案（locale.ts:87/196-200），弹窗里必须有这些字。
	hjUsageDialogTitle = "Token 用量"
	hjLabelCacheHit    = "缓存命中"
	hjLabelInput       = "未缓存输入"
	hjLabelCacheRead   = "缓存读取"
	hjLabelCacheWrite  = "缓存写入"
	hjLabelOutput      = "输出"
)

// hjCompactTokens 是官方 `formatTokens`（token-format.ts:9-15）的独立第二实现。
func hjCompactTokens(value int64) string {
	scaled := func(candidate float64) string {
		if candidate >= 100 {
			return fmt.Sprintf("%d", int(math.Round(candidate)))
		}
		rounded := math.Round(candidate*10) / 10
		if rounded == math.Trunc(rounded) {
			return fmt.Sprintf("%d", int(rounded))
		}
		return fmt.Sprintf("%.1f", rounded)
	}
	switch {
	case value < 1_000:
		return fmt.Sprintf("%d", value)
	case value < 1_000_000:
		return scaled(float64(value)/1_000) + "K"
	default:
		return scaled(float64(value)/1_000_000) + "M"
	}
}

// hjExactTokens 是官方 `formatExactTokens`（token-format.ts:23-30）的独立实现。
func hjExactTokens(value int64) string {
	digits := fmt.Sprintf("%d", value)
	var out strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return out.String()
}

// hjTokenCount 是官方 `exactCount` = `{exact} tok`。
func hjTokenCount(value int64) string { return hjExactTokens(value) + " tok" }

// hjUsageBuckets 是从 **原始 JSON** 独立解出来的四桶（不经过 apiclient 的结构体）。
type hjUsageBuckets struct {
	UncachedInputTokens int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheWriteTokens    int64
}

// billed 是官方 `billedInputTokens`（三个 prompt 侧计费桶之和）。
func (b hjUsageBuckets) billed() int64 {
	return b.UncachedInputTokens + b.CacheReadTokens + b.CacheWriteTokens
}

// total 是官方胶囊合计：prompt 侧三桶 + 输出。
func (b hjUsageBuckets) total() int64 { return b.billed() + b.OutputTokens }

// hjPercent 是官方 `formatCacheHitPercent(read, billed, 0)` 的独立第二实现：
// 整数百分比、正数 ties-up；"差一个 token"不显示成 100%（改用 99.9）。
func hjPercent(read, billed int64) string {
	if billed <= 0 {
		return ""
	}
	if read >= billed {
		return "100"
	}
	// floor(read×100/billed + 0.5) 的整数写法。
	units := (read*200 + billed) / (2 * billed)
	if units >= 100 {
		return "99.9"
	}
	return fmt.Sprintf("%d", units)
}

// reading 是官方胶囊读数 `{compact total} tok`〔` · 缓存命中 p%`〕。
//
// 缓存率只在分母成立时出现。第三个返回值 = 是否可做**逐字**断言（近 100% 时
// 官方"多点小数位"的规则在这里没复刻，只比前缀）。
func (b hjUsageBuckets) reading() (full string, prefix string, exact bool) {
	prefix = hjCompactTokens(b.total()) + " tok"
	if b.billed() <= 0 {
		return prefix, prefix, true
	}
	percent := hjPercent(b.CacheReadTokens, b.billed())
	full = prefix + " · 缓存命中 " + percent + "%"
	// 官方在"舍入会到 100 但还差 token"时给多位小数（99.9…），这里只比前缀。
	return full, prefix, b.CacheReadTokens < b.billed()
}

// rawUsageBuckets 从投影的**原始 values** 里读四桶（`proj.Values["tokenUsage"]`）。
//
// 第二个返回值 = 键在不在。**键缺席是"未知"，不是四个 0**——这条判据在真机上
// 也要能表达，所以这里不把缺席折成零值。
func rawUsageBuckets(t *testing.T, proj *apiclient.SessionProjections) (hjUsageBuckets, bool) {
	t.Helper()
	raw, ok := proj.Values["tokenUsage"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return hjUsageBuckets{}, false
	}
	var decoded struct {
		UncachedInputTokens *int64 `json:"uncachedInputTokens"`
		OutputTokens        *int64 `json:"outputTokens"`
		CacheReadTokens     *int64 `json:"cacheReadTokens"`
		CacheWriteTokens    *int64 `json:"cacheWriteTokens"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("tokenUsage 原始 JSON 解不动：%v（raw=%s）", err, raw)
	}
	// 官方 wire 四桶**全部必填**：缺任何一个都是宿主违约，不是"未知"。
	for name, value := range map[string]*int64{
		"uncachedInputTokens": decoded.UncachedInputTokens,
		"outputTokens":        decoded.OutputTokens,
		"cacheReadTokens":     decoded.CacheReadTokens,
		"cacheWriteTokens":    decoded.CacheWriteTokens,
	} {
		if value == nil {
			t.Fatalf("tokenUsage 缺必填桶 %s（官方 projectionSchema 四桶全必填）：%s", name, raw)
		}
	}
	return hjUsageBuckets{
		UncachedInputTokens: *decoded.UncachedInputTokens,
		OutputTokens:        *decoded.OutputTokens,
		CacheReadTokens:     *decoded.CacheReadTokens,
		CacheWriteTokens:    *decoded.CacheWriteTokens,
	}, true
}

// TestRealChainHJUsageDialog 硬验收：真宿主四桶 → 真用量弹窗。
func TestRealChainHJUsageDialog(t *testing.T) {
	host := startRealHost(t)
	client, _ := connectRealGUI(t, host)

	createCtx, createCancel := context.WithTimeout(context.Background(), 30*time.Second)
	sessionID, err := client.Create(createCtx, host.home, "workspace-write", "hj-usage")
	createCancel()
	if err != nil {
		t.Fatalf("session.create 失败：%v", err)
	}
	t.Logf("真实会话 id=%s（home=%s）", sessionID, host.home)

	// ── ② 真跑一轮含工具调用的 prompt（provider 才会回报 usage）。
	promptCtx, promptCancel := context.WithTimeout(context.Background(), 60*time.Second)
	if _, err := client.Prompt(promptCtx, sessionID, hjRealPrompt); err != nil {
		promptCancel()
		t.Fatalf("session.prompt 失败：%v", err)
	}
	promptCancel()
	frames := waitTurnDone(t, client, sessionID, 240*time.Second)
	var toolCalls int
	for _, frame := range frames {
		if frame.Type == "tool/call" {
			toolCalls++
		}
	}
	t.Logf("真实帧 %d 条，tool/call=%d", len(frames), toolCalls)

	// ── ③ 产品接线：真 HTTP 拉 session.projections → Snapshot。
	m := &model{cli: client}
	wiring := newWave3Wiring()
	wiring.tier.Set(true)
	wiring.fetchProjections(m, sessionID)
	wiring.feed(sessionID, 280)
	usage := wiring.tokenUsageSnapshot(sessionID)
	stats := wiring.sessionStatsSnapshot(sessionID)
	if usage == nil {
		t.Fatal("产品接线后 TokenUsage 仍为 nil（宿主拿不出 tokenUsage 四桶）")
	}
	if stats == nil {
		t.Fatal("产品接线后 SessionStats 仍为 nil（交叉一致需要它）")
	}
	t.Logf("真 tokenUsage：uncached=%d output=%d cacheRead=%d cacheWrite=%d",
		usage.UncachedInputTokens, usage.OutputTokens, usage.CacheReadTokens, usage.CacheWriteTokens)
	t.Logf("真 sessionStats：turns=%d steps=%d decodeMs=%d decodeTokens=%d",
		stats.Turns, stats.Steps, stats.DecodeMs, stats.DecodeTokens)

	// ④ 再从**原始 JSON** 独立读一遍四桶（不经被测结构体），两者必须逐字相等。
	readCtx, readCancel := context.WithTimeout(context.Background(), 30*time.Second)
	proj, err := client.SessionProjections(readCtx, sessionID)
	readCancel()
	if err != nil {
		t.Fatalf("session.projections 失败：%v", err)
	}
	raw, rawOK := rawUsageBuckets(t, proj)
	if !rawOK {
		t.Fatal("session.projections 的 values 里没有 tokenUsage（宿主没给这个键）")
	}
	if raw.UncachedInputTokens != usage.UncachedInputTokens ||
		raw.OutputTokens != usage.OutputTokens ||
		raw.CacheReadTokens != usage.CacheReadTokens ||
		raw.CacheWriteTokens != usage.CacheWriteTokens {
		t.Fatalf("产品接线的四桶 ≠ 投影原始 JSON：接线=%+v 原始=%+v", usage, raw)
	}
	// 真值的下限判据：跑过一轮带工具调用的 prompt，prompt 侧与输出侧都必须有量。
	if raw.UncachedInputTokens <= 0 || raw.OutputTokens <= 0 {
		t.Fatalf("四桶不像真值（应 > 0）：%+v", raw)
	}
	// ⑤ 交叉一致：两个投影折的是同一批 provider 上报。
	//
	// 这里**不是**无条件逐字相等：`sessionStats.decodeTokens` 只在"该 step 记到了
	// 首 token"时才累加（宿主 `session_stats.go:441-452` 的 `if firstToken != nil`），
	// 而只发工具调用、不流文本的 step 没有首 token；`tokenUsage.outputTokens` 则
	// 计入每一条 provider 样本。所以工具轮里 decodeTokens 是**下界**。
	// 逐字相等的判据放在下面那个"纯文本一轮"的会话上（ttftSteps == steps）。
	if stats.TTFTSteps == stats.Steps {
		if raw.OutputTokens != stats.DecodeTokens {
			t.Fatalf("每步都有首 token 时 outputTokens(%d) 必须 == decodeTokens(%d)",
				raw.OutputTokens, stats.DecodeTokens)
		}
		t.Logf("交叉一致：outputTokens == decodeTokens == %d", raw.OutputTokens)
	} else {
		if raw.OutputTokens < stats.DecodeTokens {
			t.Fatalf("outputTokens(%d) < decodeTokens(%d)：累计桶不可能少于只算了部分步骤的那一桶",
				raw.OutputTokens, stats.DecodeTokens)
		}
		t.Logf("交叉一致（工具轮）：outputTokens=%d ≥ decodeTokens=%d（无首 token 的工具步不计入 decodeTokens）",
			raw.OutputTokens, stats.DecodeTokens)
	}

	// ── ③④ 真画一帧 → 点用量胶囊 → 读弹窗四桶。
	product := newHERealProduct(t)
	snap := pluginapi.Snapshot{
		Cur: sessionID, Status: "已连接", CWD: host.home,
		SessionStats: stats, TokenUsage: usage,
	}
	// 第一帧只是"让 AfterDraw 学到胶囊矩形"（命中区在下一帧才声明），
	// 所以收起态要画两帧再断言。
	product.paint(t, snap, "")
	closed := product.paint(t, snap, "")
	closedHit, closedOK := heHitByName(t, product, closed, hjUsagePillHit)
	wantFull, wantPrefix, wantExact := raw.reading()
	if !closedOK {
		t.Fatalf("真机上用量胶囊没有命中区（应有可点入口）：%v", hjHitLabels(closed))
	}
	if wantExact {
		if closedHit.Label != wantFull {
			t.Fatalf("胶囊读数 = %q，want 逐字 %q（由投影原始 JSON 算出）", closedHit.Label, wantFull)
		}
		t.Logf("收起态用量胶囊读数=%q（与投影逐字相等）", closedHit.Label)
	} else if !strings.HasPrefix(closedHit.Label, wantPrefix) {
		t.Fatalf("胶囊读数 = %q，want 前缀 %q", closedHit.Label, wantPrefix)
	}

	openCanvas := product.paint(t, snap, hjUsagePillHit)
	openTexts := heTexts(openCanvas)
	if !heHasText(openCanvas, hjUsageDialogTitle) {
		t.Fatalf("点用量胶囊没有打开 %q 弹窗；texts=%v", hjUsageDialogTitle, openTexts)
	}

	// 四桶逐个：期望值是**从原始 JSON 算出来的官方格式**，不是调被测函数。
	wantRows := map[string]string{
		hjLabelInput:     hjTokenCount(raw.UncachedInputTokens),
		hjLabelCacheRead: hjTokenCount(raw.CacheReadTokens),
		hjLabelOutput:    hjTokenCount(raw.OutputTokens),
	}
	if raw.CacheWriteTokens != 0 {
		wantRows[hjLabelCacheWrite] = hjTokenCount(raw.CacheWriteTokens)
	}
	for label, value := range wantRows {
		if !heHasText(openCanvas, label) {
			t.Fatalf("弹窗缺标签 %q；texts=%v", label, openTexts)
		}
		if !heHasText(openCanvas, value) {
			t.Fatalf("弹窗缺 %q 的真值 %q；texts=%v", label, value, openTexts)
		}
	}
	// 缓存命中行：prompt 侧有量时官方一定算得出百分比，数值也是客户端算的。
	if raw.billed() > 0 {
		if !heHasText(openCanvas, hjLabelCacheHit) {
			t.Fatalf("prompt 侧有量时缺 %q 行；texts=%v", hjLabelCacheHit, openTexts)
		}
		wantPercent := hjPercent(raw.CacheReadTokens, raw.billed()) + "%"
		if raw.CacheReadTokens < raw.billed() && !heHasText(openCanvas, wantPercent) {
			t.Fatalf("缓存命中率 = %v，want %q（cacheRead=%d / billed=%d；官方客户端算法）",
				openTexts, wantPercent, raw.CacheReadTokens, raw.billed())
		}
	}
	if raw.CacheWriteTokens == 0 {
		for _, text := range openTexts {
			if text == hjLabelCacheWrite {
				t.Fatalf("缓存写入为 0 却画了这一行（官方 :336 条件渲染）：%v", openTexts)
			}
		}
	}
	// 标题值 = exactCount(total)（官方 titleValue）。
	wantTitleValue := hjTokenCount(raw.total())
	if !heHasText(openCanvas, wantTitleValue) {
		t.Fatalf("弹窗标题值缺 %q；texts=%v", wantTitleValue, openTexts)
	}
	t.Logf("弹窗真值：title=%q 值=%q 未缓存输入=%q 缓存读取=%q 缓存写入=%q 输出=%q",
		hjUsageDialogTitle, wantTitleValue, wantRows[hjLabelInput],
		wantRows[hjLabelCacheRead], wantRows[hjLabelCacheWrite], wantRows[hjLabelOutput])

	// 用量胶囊命中区：aria 名 = 胶囊读数；展开态 = true。
	hit, ok := heHitByName(t, product, openCanvas, hjUsagePillHit)
	if !ok {
		t.Fatalf("用量胶囊没有命中区：%v", hjHitLabels(openCanvas))
	}
	if !hit.Expanded {
		t.Fatal("展开后用量胶囊 aria-expanded 不是 true")
	}
	panelLabel := heHitByLabel(openCanvas, hjUsageDialogTitle)
	if !panelLabel.ok {
		t.Fatalf("弹窗没有 aria 名 %q：%v", hjUsageDialogTitle, hjHitLabels(openCanvas))
	}
	// 弹窗底沿在胶囊顶沿上方 8px（官方 bottom: anchor(top)+8）。
	if got, want := panelLabel.hit.Bounds.Y+panelLabel.hit.Bounds.Height, hit.Bounds.Y-8; math.Abs(float64(got-want)) > 0.5 {
		t.Fatalf("弹窗底沿 = %v，want 用量胶囊顶沿-8 = %v", got, want)
	}

	// ── ⑦ 截图。
	dir := os.Getenv("HJ_CAPTURE_DIR")
	if dir == "" {
		dir = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_hj/shots"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建截图目录失败：%v", err)
	}
	stamp := time.Now().Unix()
	openShot := filepath.Join(dir, fmt.Sprintf("hj-usage-dialog-open-%d.png", stamp))
	heCapture(t, product, snap, openShot)
	t.Logf("HJ_CAPTURE open path=%s", openShot)
	closedProduct := newHERealProduct(t)
	closedProduct.paint(t, snap, "")
	closedProduct.paint(t, snap, "")
	closedShot := filepath.Join(dir, fmt.Sprintf("hj-usage-dialog-closed-%d.png", stamp))
	heCapture(t, closedProduct, snap, closedShot)
	t.Logf("HJ_CAPTURE closed path=%s", closedShot)

	// ── ⑥ 空会话：宿主给"键在 + 四桶全 0" → 读数 `0 tok`，但没有可点入口。
	emptyCtx, emptyCancel := context.WithTimeout(context.Background(), 30*time.Second)
	emptyID, err := client.Create(emptyCtx, host.home, "workspace-write", "hj-usage-empty")
	emptyCancel()
	if err != nil {
		t.Fatalf("空会话 session.create 失败：%v", err)
	}
	emptyReadCtx, emptyReadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	emptyProj, err := client.SessionProjections(emptyReadCtx, emptyID)
	emptyReadCancel()
	if err != nil {
		t.Fatalf("空会话 session.projections 失败：%v", err)
	}
	emptyRaw, emptyOK := rawUsageBuckets(t, emptyProj)
	if !emptyOK {
		t.Fatal("空会话宿主没给 tokenUsage 键（那就不是「已知 0」，无法验证两态区分）")
	}
	if emptyRaw.total() != 0 {
		t.Fatalf("空会话四桶 = %+v，want 全 0", emptyRaw)
	}
	wiring.fetchProjections(m, emptyID)
	wiring.feed(emptyID, 280)
	emptyUsage := wiring.tokenUsageSnapshot(emptyID)
	if emptyUsage == nil {
		t.Fatal("空会话宿主也应给 tokenUsage（四桶全 0 的真值），得到 nil")
	}
	emptyProduct := newHERealProduct(t)
	emptySnap := pluginapi.Snapshot{Cur: emptyID, SessionStats: stats, TokenUsage: emptyUsage}
	emptyProduct.paint(t, emptySnap, "")
	emptyClosed := emptyProduct.paint(t, emptySnap, "")
	if !heHasText(emptyClosed, "0 tok") {
		t.Fatalf("空会话用量读数 = %v，want「0 tok」（宿主说 0）", heTexts(emptyClosed))
	}
	if heHasText(emptyClosed, "用量 未知") {
		t.Fatalf("空会话把真实的 0 写成了「未知」：%v", heTexts(emptyClosed))
	}
	emptyClicked := emptyProduct.paint(t, emptySnap, hjUsagePillHit)
	if heHasText(emptyClicked, hjUsageDialogTitle) {
		t.Fatalf("空会话（全 0）点出了用量弹窗：%v", heTexts(emptyClicked))
	}
	if _, ok := heHitByName(t, emptyProduct, emptyClicked, hjUsagePillHit); ok {
		t.Fatal("空会话（全 0）用量胶囊仍有命中区（假入口）")
	}
	emptyShot := filepath.Join(dir, fmt.Sprintf("hj-usage-dialog-zero-%d.png", stamp))
	heCapture(t, emptyProduct, emptySnap, emptyShot)
	t.Logf("HJ_CAPTURE zero path=%s", emptyShot)

	// ── ⑤' 逐字相等的**判据会话**：一轮纯文本（没有工具步）→ 每个 step 都记到首
	// token，`decodeTokens` 覆盖全部 provider 样本，两个投影必须**逐字相等**。
	textCtx, textCancel := context.WithTimeout(context.Background(), 30*time.Second)
	textID, err := client.Create(textCtx, host.home, "workspace-write", "hj-usage-text")
	textCancel()
	if err != nil {
		t.Fatalf("纯文本会话 session.create 失败：%v", err)
	}
	textPromptCtx, textPromptCancel := context.WithTimeout(context.Background(), 60*time.Second)
	if _, err := client.Prompt(textPromptCtx, textID, hjTextPrompt); err != nil {
		textPromptCancel()
		t.Fatalf("纯文本会话 session.prompt 失败：%v", err)
	}
	textPromptCancel()
	waitTurnDone(t, client, textID, 240*time.Second)
	wiring.fetchProjections(m, textID)
	wiring.feed(textID, 280)
	textUsage := wiring.tokenUsageSnapshot(textID)
	textStats := wiring.sessionStatsSnapshot(textID)
	if textUsage == nil || textStats == nil {
		t.Fatalf("纯文本会话投影缺席：usage=%v stats=%v", textUsage, textStats)
	}
	t.Logf("纯文本一轮：outputTokens=%d decodeTokens=%d ttftSteps=%d steps=%d",
		textUsage.OutputTokens, textStats.DecodeTokens, textStats.TTFTSteps, textStats.Steps)
	if textStats.Steps != textStats.TTFTSteps {
		t.Fatalf("纯文本一轮每步都该有首 token：ttftSteps=%d steps=%d",
			textStats.TTFTSteps, textStats.Steps)
	}
	if textUsage.OutputTokens != textStats.DecodeTokens {
		t.Fatalf("纯文本一轮 outputTokens=%d ≠ decodeTokens=%d（应逐字相等）",
			textUsage.OutputTokens, textStats.DecodeTokens)
	}
	if textUsage.OutputTokens <= 0 {
		t.Fatalf("纯文本一轮的 outputTokens 应是真值 > 0，得到 %d", textUsage.OutputTokens)
	}
	if got, want := hjTokenCount(textUsage.OutputTokens), hjTokenCount(textStats.DecodeTokens); got != want {
		t.Fatalf("输出桶读数 %q ≠ decodeTokens 读数 %q（逐字相等）", got, want)
	}
}

// hjHitLabels 列命中区的 aria 名（失败信息用）。
func hjHitLabels(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Hits()))
	for _, hit := range canvas.Hits() {
		out = append(out, hit.Label)
	}
	return out
}
