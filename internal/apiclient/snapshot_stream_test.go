package apiclient

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSnapshotStreamPathMatchesClient 把"契约快照里写的流路径"和"客户端真正连的流路径"
// 钉在一起。
//
// 为什么需要这条：`testdata/api.snapshot.json` 是这个仓里"契约的唯一可执行副本"，
// 但它此前**没有任何测试读过**。于是它在 `stream.path` 上一直写着 `/api/mux`
// （界面早期的自定形状，宿主那边从来都是 `/ws`），而代码里一直是 `/ws`——
// 两边错着，谁也没红。照快照写第二个客户端的人会连错地址，且错得毫无提示。
//
// 这条测试同时卡住两头：快照改了要红，`muxURL()` 不再走 `WSPath` 也要红。
func TestSnapshotStreamPathMatchesClient(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "api.snapshot.json"))
	if err != nil {
		t.Fatalf("读不到契约快照：%v", err)
	}
	var snap struct {
		Stream struct {
			Path      string `json:"path"`
			Transport string `json:"transport"`
		} `json:"stream"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("快照不是合法 JSON：%v", err)
	}

	if snap.Stream.Path == "" {
		t.Fatal("快照 stream.path 是空的——快照被改坏了，或者 stream 段被删了")
	}
	if snap.Stream.Path != WSPath {
		t.Errorf("快照 stream.path = %q，客户端连的是 %q。\n"+
			"两者必须一样：宿主 internal/transport/server.go 只认 %q。",
			snap.Stream.Path, WSPath, WSPath)
	}
	if snap.Stream.Transport != "websocket" {
		t.Errorf("快照 stream.transport = %q，期望 websocket", snap.Stream.Transport)
	}

	// 反向：真的拿 Mux 算一遍 URL，而不是信 WSPath 这个字符串。
	// 只比常量的话，"常量对了但 muxURL 没用它"这种改法照样绿。
	m := NewMux("http://127.0.0.1:1/")
	got, err := m.muxURL()
	if err != nil {
		t.Fatalf("muxURL：%v", err)
	}
	if want := "ws://127.0.0.1:1" + WSPath; got != want {
		t.Errorf("Mux 连的地址是 %q，期望 %q", got, want)
	}
}

// TestSnapshotNoAbandonedMuxPath 反向探针：快照里不许再出现废弃的 `/api/mux`。
//
// 与上一条分开写，是因为这两件事失败的含义不同：上一条是"两边不一致"，
// 这一条是"废弃形状回来了"。合成一条的话，红的时候得先猜是哪一种。
func TestSnapshotNoAbandonedMuxPath(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "api.snapshot.json"))
	if err != nil {
		t.Fatalf("读不到契约快照：%v", err)
	}
	var snap struct {
		Stream struct {
			Path string `json:"path"`
		} `json:"stream"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("快照不是合法 JSON：%v", err)
	}
	if snap.Stream.Path == "/api/mux" {
		t.Error("快照又写回 /api/mux 了——那是界面早期的自定形状，宿主从来没有这个端点")
	}
}
