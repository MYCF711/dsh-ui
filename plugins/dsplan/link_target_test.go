package dsplan

// link_target_test.go —— 正文里的链接该不该被当成**工作区文件**（#28 ⑥）。
//
// # 官方到底怎么做的（逐条核实过，结论与账本原来的假设相反）
//
// 官方 `<MarkdownText>` 收到一个链接目的地时先问 `Fp(destination)`「这是不是一条
// 工作区文件路径」，是就渲染成文件链接按钮、点击调 `openFile(path, {line})`。
// 但那一步**依赖一个 MarkdownDelegateProvider**：
//
//	MarkdownText 里的 `rK`（dsh-web-frontend/dist/assets/index-ghOpC9uV.js）：
//	  const { openFile, … } = Is()                       // Is() = useContext(qb)，默认 {}
//	  if (i === void 0) return <>{children}</>           // 没有 delegate → 只渲染文字
//
// 而 `qb`（`MarkdownDelegateProvider`）在全仓只有**一处**挂载：
//
//	OFF:dsh-client-ui-chat/src/client/chat/ChatView.tsx:274-285
//	  <MarkdownDelegateProvider … openFile={requestOpenFile} openExternalLink={…}>
//	    {renderSlot('conversation.chat.flow', …)}
//	  </MarkdownDelegateProvider>
//
// 它只包住**会话正文槽** `conversation.chat.flow`。计划右栏 `PlanPreview` 是另一个
// 区域、在它外面渲染：
//
//	OFF:dsh-client-ui-plan/src/client/PlanPreview.tsx:37
//	  <MarkdownText text={plan.markdown} labels={labels} />   // 没有任何链接回调
//
// 所以官方计划正文里的相对链接**也不可点**：`openFile` 是 undefined，`rK` 退化成
// `<>{children}</>`——只画链接文字。`openFile` 的那条工作区解析
// （`resolveWorkspacePath` + `fileAddressFor`，`dsh-util-workspace-path/src/index.ts:27,92`）
// 只在会话正文里生效。
//
// 本仓的判据与它同形：viva 的 markdown 解析器只把 http/https/mailto 目的地做成链接
// （`markdown/markdown.go:16-18`「any other destination is shown as plain text」、
// `markdown/inline.go:76-86`），相对路径同样退化成普通文字；`openLink` 只认明确
// scheme。两边一致，所以这里**不是差异**。
//
// 下面两条测试把这条一致钉住：谁把相对路径猜成宿主文件（例如加一个工作区解析、
// 或把 `file://` 塞进外开），这里就红。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestOpenLinkNeverGuessesAWorkspaceFile 钉住官方那条降级：
// 非 http/https/mailto 的目的地（相对路径、`file://`、裸路径）**不触发任何打开动作**。
func TestOpenLinkNeverGuessesAWorkspaceFile(t *testing.T) {
	p, env, act, _ := fixture(t, []apiclient.Frame{
		callFrame(t, 1, 1, "call-a", "exit_plan_mode", planArgs(t, "# 方案\n\n见 [说明](./docs/notes.md)。")),
	}, 360)

	for _, destination := range []string{
		"./docs/notes.md",                    // 相对路径（官方在计划正文里也不打开）
		"docs/notes.md",                      // 同上，不带 ./
		"../outside.md",                      // 往上跳
		"file:///C:/tmp/a.md",                // 明确是文件，但不是可外开的 scheme
		"notes.md#L24-L30",                   // 带行号片段的相对路径
		"dsh-resource://file/session-1/a.md", // 官方 file 协议地址也不归计划正文
	} {
		p.openLink(env, destination)
		if len(act.external) != 0 {
			t.Fatalf("目的地 %q 被猜成可打开的链接：%v", destination, act.external)
		}
	}

	// 对照组：明确 scheme 的链接照常交给壳外开（官方 `<a target=_blank>` 同一条）。
	for _, destination := range []string{"https://example.com/a", "http://example.com/b", "mailto:a@b.c"} {
		before := len(act.external)
		p.openLink(env, destination)
		if len(act.external) != before+1 || act.external[before] != destination {
			t.Fatalf("目的地 %q 没有外开，得到 %v", destination, act.external)
		}
	}
}

// TestPlanMarkdownRelativeLinkRendersAsPlainText 钉住渲染侧的同一条：
// 正文里的相对链接在解析结果里**不是**链接（与官方 `rK` 的 `<>{children}</>` 同形）。
//
// 判据取产品同一条排版路径（`layoutPreview`，右栏正文真正的排版函数）：
// 相对路径那条不该占一个链接位，只剩那条 https。
func TestPlanMarkdownRelativeLinkRendersAsPlainText(t *testing.T) {
	p, _, _, canvas := fixture(t, nil, 360)
	src := "# 方案\n\n见 [说明](./docs/notes.md)。\n\n外部 [文档](https://example.com/x)。\n"
	if height := p.layoutPreview(canvas, src, 320); height <= 0 {
		t.Fatal("排版失败：高度 0")
	}
	if got := p.preview.view.LinkCount(); got != 1 {
		t.Fatalf("链接数 = %d，want 1（只有那条 https；相对路径该是普通文字）", got)
	}
	if got := p.preview.view.Link(0); got != "https://example.com/x" {
		t.Fatalf("唯一的链接是 %q，want https://example.com/x", got)
	}
}
