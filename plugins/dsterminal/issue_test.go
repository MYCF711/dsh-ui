package dsterminal

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// TestIssueCopyMatchesOfficialLocales 钉住官方 `sidebarTerminal` 的 5 条 issue 文案
// （OFF:dsh-client-ui-sidebar-terminal/src/client/locales.ts:21-25，中文逐字）。
//
// 为什么值得一条测试：这些 key 曾经被壳当成"错误原文"塞进 Error，界面于是把
// `missingTerminal` 这种 key 直接打到屏幕上（「终端错误：missingTerminal」）。
// 现在壳只给分类，文案在这里。
func TestIssueCopyMatchesOfficialLocales(t *testing.T) {
	want := map[string]string{
		"missingTerminal": "此终端已不存在，请新建终端。",
		"inputFull":       "输入缓冲区已满，请重新连接后重试。",
		"attachmentEnded": "终端连接已结束，请重新连接。",
		"invalidOutput":   "终端画面传输异常，请重新连接。",
		"terminalLimit":   "终端数量已达上限，请关闭不用的终端后重试。已退出的终端也计入数量。",
	}
	for issue, copy := range want {
		if got := terminalIssueText(issue); got != copy {
			t.Errorf("issue %q 文案 = %q，官方是 %q", issue, got, copy)
		}
	}
	if got := terminalIssueText(""); got != "" {
		t.Errorf("空 issue 不该有文案，得到 %q", got)
	}
	// 官方 TerminalViewIssue 只有这五档（model.ts:19）；多出来的 key 不给文案，
	// 免得界面把不认识的东西画成一句"看起来正常"的话。
	if got := terminalIssueText("notAnIssue"); got != "" {
		t.Errorf("未知 issue 不该有文案，得到 %q", got)
	}
}

// TestMissingTerminalDrawsTheOfficialEmptyState 是官方 `.empty` 那一支：
//
//	issue === 'missingTerminal' → section.root > .empty（terminal.tsx:36-45）
//	.empty{flex:1;column;center;gap:16px;padding:24px}（terminal.module.css:7）
//
// 这支**不是错误行**：整块换成空态 + 主要按钮「新建终端」。
func TestMissingTerminalDrawsTheOfficialEmptyState(t *testing.T) {
	canvas, _ := terminalVisualFrame(t, pluginapi.TerminalViewState{
		Phase: "closed", Issue: terminalIssueMissingTerminal,
	})
	message := "此终端已不存在，请新建终端。"
	if !terminalHasText(canvas, message) {
		t.Fatalf("缺失终端没有画官方空态文案：%+v", canvas.Texts())
	}
	if terminalHasText(canvas, "终端错误："+message) {
		t.Fatal("空态被画成了错误行")
	}
	var newHit bool
	for _, hit := range canvas.Hits() {
		if hit.Label == "新建终端" {
			newHit = true
			if hit.Bounds.Height != terminalButtonHeight {
				t.Errorf("「新建终端」应是 small Button（%v 高），实际 %v", terminalButtonHeight, hit.Bounds.Height)
			}
		}
	}
	if !newHit {
		t.Fatal("空态里没有「新建终端」按钮")
	}
	// 空态的内边距是官方 24：文案不会贴着面板左上角。
	for _, text := range canvas.Texts() {
		if text.Value != message {
			continue
		}
		if text.Bounds.X <= terminalEmptyPad || text.Bounds.Y <= terminalEmptyPad {
			t.Errorf("空态文案没有落在 24px 内边距之内：%+v", text.Bounds)
		}
	}
}

// TestIssueDoesNotLeakTheKeyIntoTheErrorRow 是「key 泄漏」那条真实缺陷：
// 非 missingTerminal 的 issue 走错误段，但错误段里必须是**官方中文**，
// 不能是 `attachmentEnded` 这样的内部 key。
func TestIssueDoesNotLeakTheKeyIntoTheErrorRow(t *testing.T) {
	canvas, _ := terminalVisualFrame(t, pluginapi.TerminalViewState{
		Phase: "failed", Issue: "attachmentEnded",
	})
	want := "终端错误：终端连接已结束，请重新连接。"
	if !terminalHasText(canvas, want) {
		t.Fatalf("错误段应是官方文案 %q：%+v", want, canvas.Texts())
	}
	for _, text := range canvas.Texts() {
		if strings.Contains(text.Value, "attachmentEnded") {
			t.Fatalf("内部 issue key 漏到了界面上：%q", text.Value)
		}
	}
}

// TestFailedPhaseWithoutInfoDrawsNoStatusText 是官方状态链的一处边界：
// `phase==='failed'` 且 `info===undefined` 时 status 为 undefined，只留 Retry
// 按钮（terminal.tsx:46-52 的判定链里没有 failed 这一支）。
//
// 之前的实现给 `failed` 印了「不可用」，等于凭空多出一句话。
func TestFailedPhaseWithoutInfoDrawsNoStatusText(t *testing.T) {
	canvas, _ := terminalVisualFrame(t, pluginapi.TerminalViewState{Phase: "failed"})
	if terminalHasText(canvas, "不可用") {
		t.Fatal("failed + 无 info 时不该输出状态文字（官方 status 为 undefined）")
	}
	var retry bool
	for _, hit := range canvas.Hits() {
		if hit.Label == "重试" {
			retry = true
		}
	}
	if !retry {
		t.Fatal("failed 态必须留 Retry 按钮（官方只出按钮不出文字）")
	}
	// info.state==='failed' 才是「不可用」的出处。
	if got := terminalPhaseText("failed"); got != "不可用" {
		t.Fatalf("failed 的文案仍在（供 info 缺失以外的路径用）：%q", got)
	}
}

// terminalHasText 报告画布上是否画过 value。
func terminalHasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}
