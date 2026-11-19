package dsforge

// 已读标记持久化的判据全部来自官方 rc.5 源码（见 viewed_store.go 文件头）：
//
//	OFF:viewed-files.ts:11    PREFIX = 'dsh.forge-viewed.v1'
//	OFF:viewed-files.ts:53    key = `${PREFIX}.${JSON.stringify(session)}`
//	OFF:viewed-files.ts:14    MAX_SOURCES = 20
//	OFF:viewed-files.ts:19    source = { at: number, files: Record<path, digest> }
//	OFF:ChangesTab.tsx:290-294 viewedSourceOf = JSON.stringify(['local', scope, baseRef, commits])
//
// 这里钉住"键名逐字相同""值形状相同""裁剪规则相同""三态语义不变"，以及
// "标记真的经宿主往返"（不是只写内存）。

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

func testDiff(session, scope, base string, files map[string]string) *pluginapi.ForgeDiff {
	diff := &pluginapi.ForgeDiff{SessionID: session, Scope: scope, Base: base}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	// 稳定顺序，便于断言。
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			if paths[j] < paths[i] {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}
	for _, path := range paths {
		diff.Files = append(diff.Files, apiclient.ForgeDiffFile{Path: path, Digest: files[path]})
	}
	return diff
}

func TestForgeViewedKeyIsOfficiallyVerbatim(t *testing.T) {
	// 官方：`${PREFIX}.${JSON.stringify(session)}` —— session 带双引号。
	if got := forgeViewedKey("sess-1"); got != `dsh.forge-viewed.v1."sess-1"` {
		t.Fatalf("存储键 = %q，want `dsh.forge-viewed.v1.\"sess-1\"`", got)
	}
	if forgeViewedPrefix != "dsh.forge-viewed.v1" {
		t.Fatalf("前缀 = %q", forgeViewedPrefix)
	}
	if forgeViewedMaxSources != 20 {
		t.Fatalf("MAX_SOURCES = %d，want 20", forgeViewedMaxSources)
	}
}

func TestForgeViewedSourceKeyMatchesOfficialShape(t *testing.T) {
	diff := testDiff("s", "branch", "origin/main", map[string]string{"a.go": "d1"})
	diff.Commits = []pluginapi.ForgeCommit{{SHA: "c1"}, {SHA: "c2"}}
	if got, want := forgeViewedSourceKey(diff), `["local","branch","origin/main",["c1","c2"]]`; got != want {
		t.Fatalf("源 key = %s，want %s", got, want)
	}
	// 没有提交时是空数组，不是 null（官方 JSON.stringify([])）。
	empty := testDiff("s", "uncommitted", "", map[string]string{})
	if got, want := forgeViewedSourceKey(empty), `["local","uncommitted","",[]]`; got != want {
		t.Fatalf("空提交源 key = %s，want %s", got, want)
	}
}

func TestForgeViewedPrunesToTwentySourcesByRecency(t *testing.T) {
	doc := map[string]forgeViewedEntry{}
	base := time.Now().UnixMilli()
	for i := 0; i < 25; i++ {
		// 序号越大越新。
		doc[fmt.Sprintf("source-%02d", i)] = forgeViewedEntry{At: base + int64(i), Files: map[string]string{"a": "d"}}
	}
	kept := pruneForgeViewed(doc)
	if len(kept) != forgeViewedMaxSources {
		t.Fatalf("裁剪后 %d 个源，want %d", len(kept), forgeViewedMaxSources)
	}
	// 最新的 20 个（source-05..source-24）留下，最旧的 5 个丢掉。
	for i := 0; i < 5; i++ {
		if _, ok := kept[fmt.Sprintf("source-%02d", i)]; ok {
			t.Fatalf("最旧的 source-%02d 应被丢掉", i)
		}
	}
	if _, ok := kept["source-24"]; !ok {
		t.Fatal("最新的源必须留下")
	}
}

func TestForgeViewedRoundTripsThroughHostUIState(t *testing.T) {
	// 一个最小的宿主持久面：内存 map，形状与 `uiState.get/set` 一致。
	stored := map[string]json.RawMessage{}
	act := pluginapi.Actions{UIState: &pluginapi.UIStateActions{
		Get: func(key string) (json.RawMessage, bool) {
			raw, ok := stored[key]
			return raw, ok
		},
		Set: func(key string, value json.RawMessage) {
			stored[key] = append(json.RawMessage(nil), value...)
		},
		Delete: func(key string) { delete(stored, key) },
	}}
	env := &pluginapi.Env{Act: act}

	diff := testDiff("sess-1", "branch", "origin/main", map[string]string{"a.go": "digest-a", "b.go": "digest-b"})
	plugin := &Plugin{open: newChangesOpen()}
	plugin.syncViewed(env, diff)

	// 标记 a.go 已读，然后写回。
	plugin.open.markViewed("sess-1", "a.go", "digest-a", true)
	plugin.persistViewed(env, diff)

	raw, ok := stored[forgeViewedKey("sess-1")]
	if !ok {
		t.Fatalf("没有写到官方键 %q：%v", forgeViewedKey("sess-1"), stored)
	}
	var doc map[string]forgeViewedEntry
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("值不是 {源: {at, files}}：%v（%s）", err, raw)
	}
	entry, ok := doc[forgeViewedSourceKey(diff)]
	if !ok {
		t.Fatalf("值里没有本帧的源 %s：%s", forgeViewedSourceKey(diff), raw)
	}
	if entry.At == 0 {
		t.Fatalf("at 必须是毫秒时间戳：%+v", entry)
	}
	if entry.Files["a.go"] != "digest-a" {
		t.Fatalf("标记没存下来：%+v", entry.Files)
	}
	if _, ok := entry.Files["b.go"]; ok {
		t.Fatalf("没标记的文件不该出现在 files 里：%+v", entry.Files)
	}

	// 新实例（等价于重启）读回来：三态与官方一致。
	reloaded := &Plugin{open: newChangesOpen()}
	reloaded.syncViewed(env, diff)
	if got := reloaded.open.viewedState("sess-1", "a.go", "digest-a"); got != viewedDone {
		t.Fatalf("重启后 a.go 应为已读，得到 %q", got)
	}
	if got := reloaded.open.viewedState("sess-1", "a.go", "digest-new"); got != viewedChanged {
		t.Fatalf("digest 变了应为已变更，得到 %q", got)
	}
	if got := reloaded.open.viewedState("sess-1", "b.go", "digest-b"); got != viewedUnviewed {
		t.Fatalf("没标记的应为未读，得到 %q", got)
	}
}

func TestForgeViewedDropsMarksOfVanishedPaths(t *testing.T) {
	// 官方 mark() 只保留 diff 里仍存在的路径（viewed-files.ts:87-104 的 `present`）。
	stored := map[string]json.RawMessage{}
	act := pluginapi.Actions{UIState: &pluginapi.UIStateActions{
		Get: func(key string) (json.RawMessage, bool) { raw, ok := stored[key]; return raw, ok },
		Set: func(key string, value json.RawMessage) { stored[key] = append(json.RawMessage(nil), value...) },
	}}
	env := &pluginapi.Env{Act: act}

	withBoth := testDiff("sess-2", "branch", "origin/main", map[string]string{"a.go": "d1", "gone.go": "d2"})
	plugin := &Plugin{open: newChangesOpen()}
	plugin.syncViewed(env, withBoth)
	plugin.open.markViewed("sess-2", "a.go", "d1", true)
	plugin.open.markViewed("sess-2", "gone.go", "d2", true)
	plugin.persistViewed(env, withBoth)

	// 再画一次的 diff 里 gone.go 没了：写回时它必须被丢掉。
	slim := testDiff("sess-2", "branch", "origin/main", map[string]string{"a.go": "d1"})
	plugin.viewedPaths = map[string]bool{"a.go": true}
	plugin.persistViewed(env, slim)
	var doc map[string]forgeViewedEntry
	if err := json.Unmarshal(stored[forgeViewedKey("sess-2")], &doc); err != nil {
		t.Fatal(err)
	}
	files := doc[forgeViewedSourceKey(slim)].Files
	if _, ok := files["gone.go"]; ok {
		t.Fatalf("diff 里没有的路径必须被丢掉：%+v", files)
	}
	if files["a.go"] != "d1" {
		t.Fatalf("留下的标记错了：%+v", files)
	}
}

func TestForgeViewedWithoutUIStateStaysInMemory(t *testing.T) {
	// 壳没接 UIState（nil）时必须安静退回进程内记忆，不能 panic、不能假装持久化。
	env := &pluginapi.Env{}
	diff := testDiff("sess-3", "branch", "", map[string]string{"a.go": "d1"})
	plugin := &Plugin{open: newChangesOpen()}
	plugin.syncViewed(env, diff)
	plugin.open.markViewed("sess-3", "a.go", "d1", true)
	plugin.persistViewed(env, diff)
	if got := plugin.open.viewedState("sess-3", "a.go", "d1"); got != viewedDone {
		t.Fatalf("内存里的标记仍应生效，得到 %q", got)
	}
}
