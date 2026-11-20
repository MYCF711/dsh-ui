// viewed_store.go —— 「已读」标记的持久化（官方 viewed-files.ts 的等价实现）。
//
// # 官方形状（逐条，取自 rc.5 源码）
//
//	OFF:…/src/client/viewed-files.ts:11   const PREFIX = 'dsh.forge-viewed.v1'
//	OFF:…/src/client/viewed-files.ts:53   this.key = `${PREFIX}.${JSON.stringify(session)}`
//	OFF:…/src/client/viewed-files.ts:14   MAX_SOURCES = 20（超出的按 at 丢掉最旧的）
//	OFF:…/src/client/viewed-files.ts:19   source = { at: number, files: Record<path, digest> }
//	OFF:…/src/client/viewed-files.ts:87-104 mark()：写之前**重读**存储（另一个 tab 的标记要保留），
//	                                      只保留 diff 里仍存在的路径，at = now()
//	OFF:…/src/client/ChangesTab.tsx:290-294 viewedSourceOf = JSON.stringify(['local', scope, baseRef, commits])
//	                                       （change request 那支是 ['change', url, commits]）
//	OFF:…/src/client/viewed-files.ts:160-163 存储读不动 → 不显示标记，并且**拒绝写入**
//
// 官方把这份表放在浏览器 `Local Storage`；本仓没有那个容器，走宿主的
// `uiState.get/set`（落 `<home>/state/ui-state/<key>.json`），**键名逐字相同**。
//
// # 本仓的差异（登记，不假装一致）
//
// 官方的 `source.commits` 是"选中的提交"；本仓 `ForgeDiff.Commits` 只在 branch 范围
// 拉到（`localCommits`），其余范围为空数组——所以同一个 scope 在"选没选提交"
// 两态下会是两个 source key，与官方一致（官方也一样把 commits 编进 key）。
//
// 官方还有一条本仓没有的：跨 tab 的 `storage` 事件（多窗口共享一份）。本仓是单窗口
// 进程，读一次就够，标记只在本进程内广播。
package dsforge

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

const (
	// forgeViewedPrefix 是官方存储键前缀（逐字，见文件头）。
	forgeViewedPrefix = "dsh.forge-viewed.v1"
	// forgeViewedMaxSources 是一个会话保留的 diff 源数量上限（官方 MAX_SOURCES）。
	forgeViewedMaxSources = 20
	// forgeViewedStateKey 是"存过哪些会话"的索引键（官方 viewed-files.ts:6 的 `sessions` 后缀）。
	//
	// 本仓**不写**这个索引：它只用来给浏览器腾地方（官方按它删到 50 个会话）。
	// 单 home 的宿主没有配额压力，写它反而多一份要维护的状态。
	forgeViewedStateKey = forgeViewedPrefix + ".sessions"
)

// forgeViewedEntry 是一个 diff 源的标记（官方 `source`）。
type forgeViewedEntry struct {
	// At 是最后标记时间（毫秒），只用于按最近使用裁剪源数量。
	At int64 `json:"at"`
	// Files 是 path → 标记当时的 digest。digest 不同即"已变更"。
	Files map[string]string `json:"files"`
}

// forgeViewedKey 是一个会话的存储键：`<prefix>.<JSON.stringify(session)>`。
//
// 注意 session 是**带引号的 JSON 字符串**（`dsh.forge-viewed.v1."sess-1"`）：
// 官方用 `JSON.stringify(session)` 拼键，这里必须逐字相同，否则换了实现的人
// 读不到同一份数据。
func forgeViewedKey(session string) string {
	quoted, err := json.Marshal(session)
	if err != nil {
		quoted = []byte(`""`)
	}
	return forgeViewedPrefix + "." + string(quoted)
}

// forgeViewedSourceKey 是官方 `viewedSourceOf` 的等价物：local 范围的源身份。
//
// 官方原文：`JSON.stringify(['local', source.scope, source.baseRef, source.commits])`。
// 本仓 `ForgeDiff` 里这三样都在（Scope / Base / Commits），所以逐项照搬；
// commits 用 sha 序列，空就是空数组。
func forgeViewedSourceKey(diff *pluginapi.ForgeDiff) string {
	commits := make([]string, 0, len(diff.Commits))
	for _, commit := range diff.Commits {
		commits = append(commits, commit.SHA)
	}
	raw, err := json.Marshal([]any{"local", diff.Scope, diff.Base, commits})
	if err != nil {
		return `["local","","",[]]`
	}
	return string(raw)
}

// parseForgeViewed 解析一份存储值。坏数据按"读不动"处理（官方同样 refuses writes）。
func parseForgeViewed(raw json.RawMessage) (map[string]forgeViewedEntry, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var doc map[string]forgeViewedEntry
	if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
		return nil, false
	}
	return doc, true
}

// pruneForgeViewed 按 at 保留最近的 N 个源（官方 `MAX_SOURCES`）。
func pruneForgeViewed(doc map[string]forgeViewedEntry) map[string]forgeViewedEntry {
	if len(doc) <= forgeViewedMaxSources {
		return doc
	}
	type pair struct {
		key   string
		entry forgeViewedEntry
	}
	pairs := make([]pair, 0, len(doc))
	for key, entry := range doc {
		pairs = append(pairs, pair{key: key, entry: entry})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].entry.At != pairs[j].entry.At {
			return pairs[i].entry.At > pairs[j].entry.At
		}
		return pairs[i].key < pairs[j].key
	})
	kept := make(map[string]forgeViewedEntry, forgeViewedMaxSources)
	for _, item := range pairs[:forgeViewedMaxSources] {
		kept[item.key] = item.entry
	}
	return kept
}

// syncViewed 在画文件列表时读一次该会话的已读标记（每个 (会话, 源) 只读一次）。
//
// 官方在挂载时构造 `new ViewedFiles(sessionId)` 并读存储；本仓的插件是按帧绘制的，
// 所以把"读一次"放在这里：第一次画到某个会话/范围时去问宿主，读到就填进内存桶。
func (p *Plugin) syncViewed(env *pluginapi.Env, diff *pluginapi.ForgeDiff) {
	if env == nil || env.Act.UIState == nil || env.Act.UIState.Get == nil || diff == nil {
		return
	}
	session := diff.SessionID
	if session == "" {
		return
	}
	source := forgeViewedSourceKey(diff)
	if p.viewedSynced == nil {
		p.viewedSynced = map[string]bool{}
	}
	slot := session + "\x00" + source
	if p.viewedSynced[slot] {
		return
	}
	p.viewedSynced[slot] = true
	p.viewedSource = source
	p.viewedDiff = diff
	p.viewedPaths = make(map[string]bool, len(diff.Files))
	for _, file := range diff.Files {
		p.viewedPaths[file.Path] = true
	}
	raw, found := env.Act.UIState.Get(forgeViewedKey(session))
	if !found {
		return
	}
	doc, ok := parseForgeViewed(raw)
	if !ok {
		return
	}
	mark := doc[source]
	if len(mark.Files) == 0 {
		return
	}
	if p.open.viewed[session] == nil {
		p.open.viewed[session] = map[string]string{}
	}
	for path, digest := range mark.Files {
		p.open.viewed[session][path] = digest
	}
}

// persistViewed 把该会话的标记写回宿主（官方 mark() 的写那半）。
//
// 写的是**整份** `{源: {at, files}}`：先取上次读到的整份文档，替换当前源，
// 按 at 裁到 20 个源，再发出去。只保留 diff 里仍存在的路径（官方同一判据）。
func (p *Plugin) persistViewed(env *pluginapi.Env, diff *pluginapi.ForgeDiff) {
	if diff == nil {
		diff = p.viewedDiff
	}
	if env == nil || env.Act.UIState == nil || env.Act.UIState.Set == nil || diff == nil {
		return
	}
	session := diff.SessionID
	if session == "" {
		return
	}
	source := forgeViewedSourceKey(diff)
	if p.viewedSource != "" {
		source = p.viewedSource
	}
	doc := map[string]forgeViewedEntry{}
	if env.Act.UIState.Get != nil {
		if raw, found := env.Act.UIState.Get(forgeViewedKey(session)); found {
			if parsed, ok := parseForgeViewed(raw); ok {
				doc = parsed
			}
		}
	}
	files := map[string]string{}
	for path, digest := range p.open.viewed[session] {
		if len(p.viewedPaths) > 0 && !p.viewedPaths[path] {
			continue // diff 里已经没有这个路径了：官方同样丢掉
		}
		files[path] = digest
	}
	doc[source] = forgeViewedEntry{At: time.Now().UnixMilli(), Files: files}
	doc = pruneForgeViewed(doc)
	raw, err := json.Marshal(doc)
	if err != nil {
		return
	}
	env.Act.UIState.Set(forgeViewedKey(session), raw)
}
