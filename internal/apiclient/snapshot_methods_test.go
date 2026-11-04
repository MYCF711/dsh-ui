// snapshot_methods_test.go —— 契约快照的**方法表**必须覆盖客户端真正在调的每一个方法。
//
// 为什么需要：`testdata/api.snapshot.json` 是这个仓里"契约的唯一可执行副本"
// （`docs/api-contract.md` 开头就写着"两者不一致时以快照为准"）。
// 2026-11-04 它还是 v1：只有 14 个方法、缺 `session.approve`、`session.compact`
// 这些真宿主已经在用的；更糟的是 **stream 段整段是旧的**
// （`{t:ev,kind,topic}` + gap 帧，而真形状是 `{seq,type,data,time}` 且服务端不发 gap）。
//
// 一个"以它为准"的文件写着错的形状，比没有这个文件更糟——照它写客户端的人
// 会解析 `kind` 而永远拿不到 `type`。
package apiclient

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// snapshotMethods 读快照里的方法表。
func snapshotMethods(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "api.snapshot.json"))
	if err != nil {
		t.Fatalf("读不到契约快照：%v", err)
	}
	var snap struct {
		Methods map[string]json.RawMessage `json:"methods"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("快照不是合法 JSON：%v", err)
	}
	out := map[string]bool{}
	for k := range snap.Methods {
		out[k] = true
	}
	return out
}

// calledMethods 扫本包所有非测试文件，取出 `Call(ctx, "method"` 里的方法名。
func calledMethods(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Call" {
				return true
			}
			lit, ok := call.Args[1].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			seen[strings.Trim(lit.Value, `"`)] = true
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("扫 apiclient 失败：%v", err)
	}
	var out []string
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

func TestSnapshotCoversEveryCalledMethod(t *testing.T) {
	called := calledMethods(t)
	if len(called) < 15 {
		t.Fatalf("只扫到 %d 个被调用的方法——扫描失效了（实际有 20+）", len(called))
	}
	have := snapshotMethods(t)
	var missing []string
	for _, m := range called {
		if !have[m] {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("这些方法客户端在调，契约快照里却没有：%v\n"+
			"快照是'以它为准'的那一份，缺一个就会有人照着它写出过时的客户端。", missing)
	}
	t.Logf("客户端调用 %d 个方法，快照收录 %d 个", len(called), len(have))
}

// TestSnapshotStreamIsNotTheAbandonedShape 钉住 v1 那段**错**的流形状不会再回来。
//
// v1 写的是 `{t:"ev", topic, kind}` + 服务端发 `gap` 帧。两样都是错的：
// 真形状是 `{seq,type,data,time}`，而断号由**客户端**发现（服务端不发 gap）。
func TestSnapshotStreamIsNotTheAbandonedShape(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "api.snapshot.json"))
	if err != nil {
		t.Fatalf("读不到契约快照：%v", err)
	}
	var snap struct {
		Stream struct {
			ServerFrames  []map[string]any `json:"server_frames"`
			ControlFrames []map[string]any `json:"control_frames"`
			NoGap         string           `json:"no_gap_frames"`
		} `json:"stream"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("快照不是合法 JSON：%v", err)
	}
	if len(snap.Stream.ServerFrames) == 0 {
		t.Fatal("快照没有 server_frames")
	}
	first := snap.Stream.ServerFrames[0]
	for _, bad := range []string{"t", "kind", "topic"} {
		if _, ok := first[bad]; ok {
			t.Errorf("服务端帧里还有 %q 字段——那是 v1 旧形状（{t:ev,kind,topic}），真形状是 {seq,type,data,time}", bad)
		}
	}
	for _, good := range []string{"seq", "type", "data", "time"} {
		if _, ok := first[good]; !ok {
			t.Errorf("服务端帧缺 %q 字段", good)
		}
	}
	if snap.Stream.NoGap == "" {
		t.Error("没有写明'服务端不发 gap 帧'——v1 就是这么错的")
	}
	seenReset := false
	for _, f := range snap.Stream.ControlFrames {
		if f["type"] == "session/reset" {
			seenReset = true
			if _, ok := f["seq"]; ok {
				t.Error("session/reset 是控制帧，不该带 seq")
			}
		}
	}
	if !seenReset {
		t.Error("控制帧里没有 session/reset——换代重建基线全靠它")
	}
}
