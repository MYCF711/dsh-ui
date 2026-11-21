package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/app"
)

// 这个文件钉住右栏浏览器的**存储分区**（官方 `DesktopBrowserGuests.partitions`，
// `OFF:app.asar:lib/main.js:18093,18105-18111`；渲染端 `acquire(workspace)` 见
// `src/types.ts:22`）。四条判据：
//
//	同一个账号 → 永远同一个名字（同 workspace 的两个 tab 共用 cookies）
//	不同账号   → 不同名字（两个 workspace 互不可见）
//	空账号     → **一个字都不下发**（默认路径不变；也绝不造全局共享分区）
//	有账号     → `Ephemeral`（官方是内存会话，没有 `persist:` 前缀）
//
// 名字本身还有三条硬约束：长度 ≤ 64、字符集在 WebView2 的 profile 名表里、
// **不含账号内容**（否则工作区路径会被写进 WebView2 的 profile 元数据）。

// validPartitionName 是 `viva@v0.6.11-part/app/web.go:306-341` 那三条规则的复述：
// 最长 64、字符集是字母数字加 `#@$()+-_~. `、不以 `.` 或空格结尾。
var validPartitionName = regexp.MustCompile(`^[A-Za-z0-9#@$()+\-_~. ]{1,64}$`)

// TestWebPartitionNameIsStableForOneAccount 钉住"缓存"这半条：
// 同一个账号问多少次都只有一个名字（官方 `partitions.get(workspace)` 命中就复用）。
func TestWebPartitionNameIsStableForOneAccount(t *testing.T) {
	var cache webPartitionCache
	first := cache.name("cwd:/work/alpha")
	if first == "" {
		t.Fatal("第一次要名字就拿到了空串：有账号时必须下发分区")
	}
	for i := 0; i < 8; i++ {
		if got := cache.name("cwd:/work/alpha"); got != first {
			t.Fatalf("第 %d 次问同一个账号拿到了 %q，第一次是 %q —— 同一个 workspace 的两个 tab 会分到两套 cookies", i+2, got, first)
		}
	}
}

// TestWebPartitionNameDiffersForDifferentAccounts 钉住"隔离"那半条：
// 账号不同名字就必须不同，而且**不许有全局共享的名字**。
//
// 变异成"所有账号一个名字"（官方没有的实现）时这条会红。
func TestWebPartitionNameDiffersForDifferentAccounts(t *testing.T) {
	var cache webPartitionCache
	accounts := []string{
		"cwd:/work/alpha",
		"cwd:/work/beta",
		// 官方那条 ungrouped 分支（`workspace.ts:32`）：同一个工作区外的两个会话
		// 各自隔离，不许并进一个名字。
		"session:s-1",
		"session:s-2",
	}
	seen := map[string]string{}
	for _, account := range accounts {
		name := cache.name(account)
		if name == "" {
			t.Fatalf("账号 %q 没拿到分区名", account)
		}
		if other, ok := seen[name]; ok {
			t.Fatalf("账号 %q 与 %q 拿到了同一个分区名 %q —— 两个 workspace 会互相看见 cookies", account, other, name)
		}
		seen[name] = account
	}
	// 反向也要成立：同一份账号集合再问一遍，名字不许变（缓存不是"每次随机"）。
	for _, account := range accounts {
		if name := cache.name(account); seen[name] != account {
			t.Fatalf("第二次问账号 %q 拿到了 %q，它属于 %q —— 名字漂了", account, name, seen[name])
		}
	}
}

// TestWebPartitionNameIsAValidEngineName 钉住名字本身能被引擎接受、且**不含账号**。
func TestWebPartitionNameIsAValidEngineName(t *testing.T) {
	// 前缀逐字照官方 `lib/main.js:18109` 那条模板串，**不用本仓常量比常量**——
	// 否则常量漂了这条还绿（变异实验里真发生过）。
	const officialPrefix = "dsh-sidebar-browser-"
	if webPartitionPrefix != officialPrefix {
		t.Fatalf("分区名前缀 = %q，官方是 %q", webPartitionPrefix, officialPrefix)
	}
	var cache webPartitionCache
	// 故意拿一个带空格、反斜杠、冒号、中文的路径当账号：这些字符都在账号里合法
	// （它是 Map 的键），但一个都不能进名字。
	account := `cwd:C:\Users\Administrator\我 的工作区\alpha`
	name := cache.name(account)
	if name == "" {
		t.Fatal("带特殊字符的账号也该拿到分区名")
	}
	if !strings.HasPrefix(name, officialPrefix) {
		t.Fatalf("分区名 %q 没有官方前缀 %q（`lib/main.js:18109`）", name, officialPrefix)
	}
	if !validPartitionName.MatchString(name) || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		t.Fatalf("分区名 %q 不合 WebView2 的 profile 名规则（≤64、字母数字加 #@$()+-_~. 空格、不以 . 或空格结尾）", name)
	}
	// 长度：前缀 21 + 36 字符 UUID = 57，与官方同一个数。
	if len(name) != len(officialPrefix)+36 {
		t.Fatalf("分区名 %q 长 %d，期望 %d（前缀 + UUID 文本）", name, len(name), len(officialPrefix)+36)
	}
	if ok, _ := regexp.MatchString(`^`+regexp.QuoteMeta(officialPrefix)+`[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, name); !ok {
		t.Fatalf("分区名 %q 不是「前缀 + 小写 v4 UUID」的形状（官方 randomUUID 同形）", name)
	}
	// **账号内容不许进名字**：工作区路径进了名字就等于写进 WebView2 的 profile 元数据。
	for _, leak := range []string{"Users", "工作区", "cwd", "C:", "alpha"} {
		if strings.Contains(name, leak) {
			t.Fatalf("分区名 %q 里出现了账号片段 %q —— 本机路径会被写进引擎的 profile 元数据", name, leak)
		}
	}
}

// TestWebPartitionSendsNothingWithoutAnAccount 钉住"默认路径不变"：
// 没有存储账号时 `Partition`/`Ephemeral` 必须保持零值——那正是本字段出现之前的
// 窗口配置，逐字节相同。
func TestWebPartitionSendsNothingWithoutAnAccount(t *testing.T) {
	var cache webPartitionCache
	var config app.WebViewConfig
	cache.apply(&config, "")
	if config.Partition != "" {
		t.Fatalf("没有账号却下发了分区名 %q —— 会造出一个所有会话共用的存储", config.Partition)
	}
	if config.Ephemeral {
		t.Fatal("没有账号却打开了 Ephemeral —— 默认路径的窗口配置变了")
	}
	// 整份配置也要与"根本不接这条线"逐字段相同（webViewConfig 的输出）。
	want := webViewConfig(pluginapi.WebOpenOptions{URL: "https://example.com/"})
	got := webViewConfig(pluginapi.WebOpenOptions{URL: "https://example.com/"})
	(&webPartitionCache{}).apply(&got, "")
	if got.Partition != want.Partition || got.Ephemeral != want.Ephemeral {
		t.Fatalf("空账号改动了配置：Partition %q→%q Ephemeral %v→%v",
			want.Partition, got.Partition, want.Ephemeral, got.Ephemeral)
	}
}

// TestWebPartitionIsEphemeralForAnAccount 钉住"照官方不 persist"：
// 官方浏览器侧的分区名没有 `persist:` 前缀 = 进程内内存会话
// （`lib/main.js:18109`），而 Viva 里只给 `Partition` 是**持久**的
// （`viva@v0.6.11-part/app/web.go:136-152`），必须一起给 `Ephemeral` 才是内存会话。
func TestWebPartitionIsEphemeralForAnAccount(t *testing.T) {
	cache := &webPartitionCache{}
	var config app.WebViewConfig
	cache.apply(&config, "cwd:/work/alpha")
	if config.Partition == "" {
		t.Fatal("有账号时没下发分区名")
	}
	if !config.Ephemeral {
		t.Fatalf("分区 %q 没有 Ephemeral —— 那会变成磁盘持久存储，比官方多出磁盘残留（官方是进程内内存会话）", config.Partition)
	}
	if strings.Contains(config.Partition, "persist:") {
		t.Fatalf("分区名 %q 带了 persist: 前缀 —— 那是 Electron 的持久分区写法，官方浏览器侧不用", config.Partition)
	}
}

// TestWebPartitionIsSharedByTabsOfOneWorkspace 是这条账最关键的一条**端到端**判据：
// 同一个工作区的两页（两个 tab / 两次 Open）必须拿到同一个分区名，而不同工作区的
// 必须拿到不同名字。账号来自插件（`plugins/dsbrowser/storage.go` 的
// `browserStorageAccount`，形状逐字同官方 `workspace.ts:32`）。
func TestWebPartitionIsSharedByTabsOfOneWorkspace(t *testing.T) {
	cache := &webPartitionCache{}
	// 「同一 workspace 的两页」= 两个 tab 都算出同一个账号（插件那一半由
	// plugins/dsbrowser/storage_test.go 钉住），这里走壳这一半：账号 → 名字。
	alphaTabA := cache.name("cwd:/work/alpha")
	alphaTabB := cache.name("cwd:/work/alpha")
	if alphaTabA != alphaTabB {
		t.Fatalf("同一 workspace 的两个 tab 拿到 %q / %q，应当是同一个分区", alphaTabA, alphaTabB)
	}
	beta := cache.name("cwd:/work/beta")
	if beta == alphaTabA {
		t.Fatalf("两个不同 workspace 拿到同一个分区 %q —— 它们会互相看见 cookies", beta)
	}
	// ungrouped 会话（官方 `session:<id>`）同样彼此隔离。
	if cache.name("session:s-1") == cache.name("session:s-2") {
		t.Fatal("两个 ungrouped 会话拿到了同一个分区")
	}
}

// TestWebPagesConfigCarriesThePartition 钉住接线：`webPages.config` 真的把账号
// 变成了窗口配置里的两字段（`Open` 走的就是它）。
func TestWebPagesConfigCarriesThePartition(t *testing.T) {
	cache := &webPartitionCache{}
	pages := webPages{partitions: cache}
	config := pages.config(pluginapi.WebOpenOptions{URL: "https://example.com/", StorageAccount: "cwd:/work/alpha"})
	if config.Partition == "" || !config.Ephemeral {
		t.Fatalf("配置里没有分区：Partition=%q Ephemeral=%v", config.Partition, config.Ephemeral)
	}
	if config.URL != "https://example.com/" {
		t.Fatalf("翻译那半丢了：URL=%q", config.URL)
	}
	// 同一个账号两次 Open 必须同名字（否则 tab 之间不共享登录态）。
	again := pages.config(pluginapi.WebOpenOptions{StorageAccount: "cwd:/work/alpha"})
	if again.Partition != config.Partition {
		t.Fatalf("同一账号两次 Open 拿到 %q / %q", config.Partition, again.Partition)
	}
}

// TestWebModelHandsTheCacheToWebPages 钉住**壳的装配**：`webPages` 是每帧新建的值，
// 缓存必须来自 model 上那份长期状态（`m.webPartitions`）。少接这一根线的话每帧都是
// 新缓存 = 每次随机 = 同一个 workspace 的两个 tab 分到两套 cookies，而这种退化
// 在别的测试里看不出来。
//
// 用 AST 读 `main.go`（不是字符串 grep）：要求那行复合字面量里真有 `partitions` 键，
// 且取的是 `m.webPartitions` 的地址。
func TestWebModelHandsTheCacheToWebPages(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("解不动 main.go：%v", err)
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if ident, ok := literal.Type.(*ast.Ident); !ok || ident.Name != "webPages" {
			return true
		}
		for _, element := range literal.Elts {
			kv, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "partitions" {
				continue
			}
			unary, ok := kv.Value.(*ast.UnaryExpr)
			if !ok || unary.Op != token.AND {
				t.Fatalf("webPages{partitions: …} 的值不是地址：%T", kv.Value)
			}
			selector, ok := unary.X.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "webPartitions" {
				t.Fatalf("webPages{partitions: …} 不是 &m.webPartitions：%T", unary.X)
			}
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("main.go 里没有 webPages{… partitions: &m.webPartitions …}：分区缓存没有跨帧活着，每帧都会重新随机")
	}
}

// TestWebPartitionMintFailureSendsNothing 钉住那条安全分支：铸不出名字时**不下发**
// 分区，而不是让所有账号共用一个名字（"看起来隔离、其实共用一套 cookies"）。
// 下一次还能成功——失败不缓存。
func TestWebPartitionMintFailureSendsNothing(t *testing.T) {
	fail := true
	cache := &webPartitionCache{newID: func() string {
		if fail {
			return ""
		}
		return "0123abcd-0123-4123-8123-0123456789ab"
	}}
	var config app.WebViewConfig
	cache.apply(&config, "cwd:/work/alpha")
	if config.Partition != "" || config.Ephemeral {
		t.Fatalf("铸不出名字却下发了 Partition=%q Ephemeral=%v", config.Partition, config.Ephemeral)
	}
	// 空名字**不许**被缓存：恢复后这个账号要能拿到自己的分区。
	fail = false
	if name := cache.name("cwd:/work/alpha"); name != webPartitionPrefix+"0123abcd-0123-4123-8123-0123456789ab" {
		t.Fatalf("恢复后拿到 %q", name)
	}
}

// TestWebPartitionCacheIsRaceFree 让 `-race` 打这条锁：网页事件在窗口线程、
// Open 也可能从别处来，同一个账号并发问必须还是一个名字。
func TestWebPartitionCacheIsRaceFree(t *testing.T) {
	cache := &webPartitionCache{}
	const workers = 16
	names := make([]string, workers)
	accounts := make([]string, workers)
	for i := 0; i < workers; i++ {
		// 一半并发问同一个账号，一半各问各的：既验锁，也验"不会串号"。
		if i%2 == 0 {
			accounts[i] = "cwd:/work/alpha"
		} else {
			accounts[i] = fmt.Sprintf("cwd:/work/w%d", i)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			names[index] = cache.name(accounts[index])
		}(i)
	}
	wg.Wait()
	byAccount := map[string]string{}
	for i := 0; i < workers; i++ {
		if names[i] == "" {
			t.Fatalf("第 %d 个并发调用没拿到名字", i)
		}
		if previous, ok := byAccount[accounts[i]]; ok && previous != names[i] {
			t.Fatalf("账号 %q 并发拿到两个名字 %q / %q", accounts[i], previous, names[i])
		}
		byAccount[accounts[i]] = names[i]
	}
	// 不同账号之间不许撞（包括那条共享的 alpha）。
	seen := map[string]string{}
	for account, name := range byAccount {
		if other, ok := seen[name]; ok {
			t.Fatalf("账号 %q 与 %q 撞了名字 %q", account, other, name)
		}
		seen[name] = account
	}
}
