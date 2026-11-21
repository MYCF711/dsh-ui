package dsbrowser

// storage_test.go —— 网页**存储账号**的断言（官方 `electron/workspace.ts` 的移植）。
//
// 官方判据（`OFF:…/src/client/electron/workspace.ts:31-32`）：
//
//	const workspace = source.getSnapshot().items.find(item => item.sessionIds.some(id => id === sessionId))
//	return workspace === undefined ? `session:${sessionId}` : `cwd:${workspace.path}`
//
// 主进程再按这个账号缓存分区名（`lib/main.js:18105-18111`）。本仓把两半分在：
//
//	plugins/dsbrowser/storage.go  账号（本文件钉住：同 workspace 两页 → 同账号）
//	cmd/app/web_pages.go          名字（web_pages_partition_test.go 钉住：同账号 → 同名字）
//
// 契约点就是 `pluginapi.WebOpenOptions.StorageAccount`，两边都用同一批字面量
// （`cwd:/work/alpha` / `session:<id>`），所以两半合起来就是官方的整条链。

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// storageEnv 造一个"某个会话的浏览器 tab"在画的 env，工作区名录由用例给。
func storageEnv(sessionID string, workspaces ...pluginapi.Workspace) *pluginapi.Env {
	return &pluginapi.Env{
		Data: pluginapi.Snapshot{
			Cur:        sessionID,
			Workspaces: workspaces,
			// registry 已经落地：账本形状是"拉到了才 known"，
			// 拉到空数组才是"真的没有工作区"（见 cmd/app/workspace_registry_wiring.go 文件头）。
			WorkspacesKnown: true,
		},
		RightTabID: "tab1",
	}
}

func TestStorageAccountUsesTheWorkspacePath(t *testing.T) {
	env := storageEnv("s1", pluginapi.Workspace{
		WorkspaceID: "ws-1", Path: "/work/alpha", SessionIDs: []string{"s1", "s2"},
	})
	if got := browserStorageAccount(env); got != "cwd:/work/alpha" {
		t.Fatalf("账号 = %q，want %q（官方用工作区**路径**，不用 registry 的 record id）", got, "cwd:/work/alpha")
	}
}

// TestStorageAccountIsSharedByEveryTabOfOneWorkspace 是"同一 workspace 的两页拿到同一
// 分区名"的**上半条**：两个 tab（不同的 RightTabID）算出来的账号必须相同。
func TestStorageAccountIsSharedByEveryTabOfOneWorkspace(t *testing.T) {
	workspace := pluginapi.Workspace{WorkspaceID: "ws-1", Path: "/work/alpha", SessionIDs: []string{"s1"}}
	tabA := storageEnv("s1", workspace)
	tabB := storageEnv("s1", workspace)
	tabB.RightTabID = "tab2"
	first := browserStorageAccount(tabA)
	if first == "" {
		t.Fatal("有会话身份却算不出存储账号")
	}
	if second := browserStorageAccount(tabB); second != first {
		t.Fatalf("同一 workspace 的两个 tab 算出 %q / %q —— 会是两套 cookies", first, second)
	}
	// 同一工作区**不同会话**也是同一个账号（账号按工作区，不按会话）。
	if other := browserStorageAccount(storageEnv("s2", pluginapi.Workspace{
		WorkspaceID: "ws-1", Path: "/work/alpha", SessionIDs: []string{"s1", "s2"},
	})); other != first {
		t.Fatalf("同一工作区的另一个会话算出 %q，want %q", other, first)
	}
}

// TestStorageAccountIsolatesDifferentWorkspaces 是"不同 workspace 拿到不同名字"的
// 上半条：账号本身就必须不同。
func TestStorageAccountIsolatesDifferentWorkspaces(t *testing.T) {
	alpha := browserStorageAccount(storageEnv("s1", pluginapi.Workspace{
		WorkspaceID: "ws-1", Path: "/work/alpha", SessionIDs: []string{"s1"},
	}))
	beta := browserStorageAccount(storageEnv("s1", pluginapi.Workspace{
		WorkspaceID: "ws-2", Path: "/work/beta", SessionIDs: []string{"s1"},
	}))
	if alpha == beta {
		t.Fatalf("两个不同 workspace 算出同一个账号 %q —— 会互相看见 cookies", alpha)
	}
}

// TestStorageAccountFallsBackToTheSession 钉住官方那条 ungrouped 分支：
// 不属于任何工作区的会话**隔离到会话**，不是丢回公共存储。
func TestStorageAccountFallsBackToTheSession(t *testing.T) {
	// 没有任何工作区。
	if got := browserStorageAccount(storageEnv("s1")); got != "session:s1" {
		t.Fatalf("账号 = %q，want session:s1（官方「ungrouped Sessions remain isolated」）", got)
	}
	// 有工作区但这个会话不在里面。
	got := browserStorageAccount(storageEnv("s9", pluginapi.Workspace{
		WorkspaceID: "ws-1", Path: "/work/alpha", SessionIDs: []string{"s1"},
	}))
	if got != "session:s9" {
		t.Fatalf("账号 = %q，want session:s9", got)
	}
	// registry 还没落地（Known=false、列表空）→ 同一条 ungrouped 分支，绝不空手。
	loading := &pluginapi.Env{Data: pluginapi.Snapshot{Cur: "s1"}}
	if got := browserStorageAccount(loading); got != "session:s1" {
		t.Fatalf("registry 未落地时账号 = %q，want session:s1", got)
	}
}

// TestStorageAccountSendsNothingWithoutASessionIdentity 钉住"默认路径不变"的上半条：
// 连会话身份都没有时给空串，壳那边一个字都不下发（`web_pages.go` 的 apply）。
func TestStorageAccountSendsNothingWithoutASessionIdentity(t *testing.T) {
	if got := browserStorageAccount(&pluginapi.Env{}); got != "" {
		t.Fatalf("没有会话身份却算出账号 %q —— 会造出一个所有会话共用的存储", got)
	}
	if got := browserStorageAccount(nil); got != "" {
		t.Fatalf("nil env 却算出账号 %q", got)
	}
}

// TestStorageAccountNeverCollapsesAnEmptyPathWorkspace 钉住本仓那条比官方**更保守**
// 的判断：命中工作区但 Path 为空时退回会话级隔离，而不是拼出 `cwd:`。
//
// 官方会拼 `cwd:`（空路径）→ 一堆"路径为空"的工作区共用同一个存储账号 =
// 互相看得见 cookies。硬边界不许在没有有效工作区标识时造共享分区。
func TestStorageAccountNeverCollapsesAnEmptyPathWorkspace(t *testing.T) {
	emptyPath := storageEnv("s1", pluginapi.Workspace{
		WorkspaceID: "ws-1", Path: "", SessionIDs: []string{"s1"},
	})
	if got := browserStorageAccount(emptyPath); got != "session:s1" {
		t.Fatalf("空路径工作区的账号 = %q，want session:s1（不许并进一个共享的 cwd:）", got)
	}
	// 两个这样的会话不许算到同一个账号上。
	if a, b := browserStorageAccount(emptyPath), browserStorageAccount(storageEnv("s2", pluginapi.Workspace{
		WorkspaceID: "ws-2", Path: "", SessionIDs: []string{"s2"},
	})); a == b {
		t.Fatalf("两个空路径工作区算出同一个账号 %q", a)
	}
}

// TestStorageAccountTakesTheFirstMatchingWorkspace 是官方 `.find()` 的语义：
// 只取第一条命中的，顺序变了才会变（本仓与官方同序）。
func TestStorageAccountTakesTheFirstMatchingWorkspace(t *testing.T) {
	env := storageEnv("s1",
		pluginapi.Workspace{WorkspaceID: "ws-1", Path: "/work/alpha", SessionIDs: []string{"s1"}},
		pluginapi.Workspace{WorkspaceID: "ws-2", Path: "/work/beta", SessionIDs: []string{"s1"}},
	)
	if got := browserStorageAccount(env); got != "cwd:/work/alpha" {
		t.Fatalf("账号 = %q，want cwd:/work/alpha（第一条命中的工作区）", got)
	}
	// 命中的是**后面的**那条也要认（find 是全表扫，不是只看第一条）。
	later := storageEnv("s1",
		pluginapi.Workspace{WorkspaceID: "ws-1", Path: "/work/alpha", SessionIDs: []string{"s9"}},
		pluginapi.Workspace{WorkspaceID: "ws-2", Path: "/work/beta", SessionIDs: []string{"s1"}},
	)
	if got := browserStorageAccount(later); got != "cwd:/work/beta" {
		t.Fatalf("账号 = %q，want cwd:/work/beta", got)
	}
}

// TestStorageAccountPrefixesMatchOfficial 钉住两个前缀**逐字**同官方
// （`workspace.ts:32` 的 `cwd:${...}` / `session:${...}`）：前缀漂了，壳那边的键就
// 与官方语义对不上，而两边都还在"看起来能跑"。
func TestStorageAccountPrefixesMatchOfficial(t *testing.T) {
	if storageAccountWorkspace != "cwd:" {
		t.Fatalf("工作区前缀 = %q，官方是 cwd:", storageAccountWorkspace)
	}
	if storageAccountSession != "session:" {
		t.Fatalf("会话前缀 = %q，官方是 session:", storageAccountSession)
	}
	// 两个前缀互相不是前缀（否则 ungrouped 会话会被误判进工作区那一支）。
	if strings.HasPrefix(storageAccountSession, storageAccountWorkspace) ||
		strings.HasPrefix(storageAccountWorkspace, storageAccountSession) {
		t.Fatal("两个前缀互为前缀，判据会串")
	}
}

// TestOpenPagePassesTheStorageAccount 钉住接线：`openPage` 真的把它交给了载具。
func TestOpenPagePassesTheStorageAccount(t *testing.T) {
	host := &fakeHost{}
	p, env := windowEnv(host)
	env.Data.Workspaces = []pluginapi.Workspace{
		{WorkspaceID: "ws-1", Path: "/work/alpha", SessionIDs: []string{"s1"}},
	}
	env.Data.WorkspacesKnown = true
	p.openPage(env, browserTarget{Kind: kindHTTPS, URL: "https://example.com/", Title: "example.com"})
	if host.lastStorageAccount != "cwd:/work/alpha" {
		t.Fatalf("Open 交进去的账号 = %q，want cwd:/work/alpha", host.lastStorageAccount)
	}
	// 第二次 Open（另一个 tab）还是同一个账号——两个 tab 因此共用一套 cookies。
	env.RightTabID = "tab2"
	p.openPage(env, browserTarget{Kind: kindHTTPS, URL: "https://example.com/2"})
	if host.lastStorageAccount != "cwd:/work/alpha" {
		t.Fatalf("第二个 tab 的账号 = %q，want cwd:/work/alpha", host.lastStorageAccount)
	}
}

// TestOpenPageSendsNoStorageAccountWithoutIdentity 钉住"默认路径不变"的接线：
// 没有会话身份时交给载具的是空串（壳据此保持旧窗口配置）。
func TestOpenPageSendsNoStorageAccountWithoutIdentity(t *testing.T) {
	host := &fakeHost{}
	p, env := windowEnv(host)
	env.Data.Cur = ""
	p.openPage(env, browserTarget{Kind: kindHTTPS, URL: "https://example.com/", Title: "example.com"})
	if host.opened == 0 {
		t.Fatal("没有会话身份就不开页了 —— 默认路径被改了")
	}
	if host.lastStorageAccount != "" {
		t.Fatalf("没有会话身份却下发了账号 %q", host.lastStorageAccount)
	}
}
