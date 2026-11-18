package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOfficialTerminalUnaryAndInputShapes(t *testing.T) {
	var requests = map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requests[strings.TrimPrefix(r.URL.Path, "/api/")] = body
		w.Header().Set("Content-Type", "application/json")
		switch strings.TrimPrefix(r.URL.Path, "/api/") {
		case "terminal.create":
			fmt.Fprint(w, `{"result":{"id":"tab1","title":"bash","shell":{"path":"/bin/bash","args":[],"name":"bash"},"cwd":"/work","cols":80,"rows":24,"state":"running","exitCode":null,"origin":"user","control":"user"}}`)
		case "terminal.list":
			fmt.Fprint(w, `{"result":[]}`)
		case "terminal.environment":
			fmt.Fprint(w, `{"result":{"cwd":"/work","maxInputBytes":65536,"maxBufferedBytes":262144,"maxCols":500,"maxRows":200,"scrollback":1000}}`)
		case "terminal.shells":
			fmt.Fprint(w, `{"result":[{"path":"/bin/bash","args":[],"name":"bash"}]}`)
		default:
			fmt.Fprint(w, `{"result":{"ok":true}}`)
		}
	}))
	defer srv.Close()
	c := New(srv.URL)
	ctx := context.Background()
	info, err := c.TerminalCreate(ctx, "s1", TerminalCreateRequest{ID: "tab1", Cols: 80, Rows: 24})
	if err != nil || info.ID != "tab1" || info.Origin != "user" || info.Control != "user" {
		t.Fatalf("create：%+v, %v", info, err)
	}
	request := requests["terminal.create"]["request"].(map[string]any)
	if requests["terminal.create"]["agentId"] != "s1" || request["id"] != "tab1" || request["cols"] != float64(80) {
		t.Fatalf("官方 create 参数形状错误：%v", requests["terminal.create"])
	}
	if _, err := c.TerminalList(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	env, err := c.TerminalEnvironment(ctx, "s1")
	if err != nil || env.MaxInputBytes != 65536 || env.Scrollback != 1000 {
		t.Fatalf("environment：%+v, %v", env, err)
	}
	shells, err := c.TerminalShells(ctx, "s1")
	if err != nil || len(shells) != 1 || shells[0].Name != "bash" {
		t.Fatalf("shells：%+v, %v", shells, err)
	}
	if err := c.TerminalRename(ctx, "s1", "tab1", "构建"); err != nil {
		t.Fatal(err)
	}
	if err := c.TerminalWrite(ctx, "s1", "tab1", "echo hi\n", true); err != nil {
		t.Fatal(err)
	}
	write := requests["terminal.input"]
	if write["sessionId"] != "s1" || write["data"] != "echo hi\n" || write["typed"] != true {
		t.Fatalf("官方 PTY write 形状错误：%v", write)
	}
	if err := c.TerminalClose(ctx, "s1", "tab1"); err != nil {
		t.Fatal(err)
	}
}

// TestTerminalResizeShape 钉住 `terminal.resize` 的参数形状。
//
// 判据是**宿主 handler 的 decode 结构**（`DSH-GO/internal/transport/
// handlers_terminal.go` 的 `handleTerminalResize`：sessionId/agentId + id +
// cols + rows，平铺不包 `params`）。形状错了宿主会 400「缺 id」，而错误文案
// 与"终端不在了"很像，查起来很绕——所以这里逐字段钉住。
func TestTerminalResizeShape(t *testing.T) {
	var got map[string]any
	reject := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path := strings.TrimPrefix(r.URL.Path, "/api/"); path != "terminal.resize" {
			t.Errorf("应该只打 terminal.resize，实际 %s", path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("请求体不是 JSON 对象：%v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if reject {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"code":"bad_request","message":"尺寸越界：cols∈[2,500]、rows∈[1,200]（收到 1×0）","hint":"按官方 fitScreen 重算","retryable":false}}`)
			return
		}
		fmt.Fprint(w, `{"result":{"ok":true}}`)
	}))
	defer srv.Close()
	c := New(srv.URL)
	ctx := context.Background()
	if err := c.TerminalResize(ctx, "s1", "tab1", 140, 45); err != nil {
		t.Fatalf("resize 应该成功：%v", err)
	}
	// 平铺：外层只有 sessionId/id/cols/rows 四个键，没有 params 包壳。
	if len(got) != 4 || got["sessionId"] != "s1" || got["id"] != "tab1" {
		t.Fatalf("resize 参数形状错误：%v", got)
	}
	if got["cols"] != float64(140) || got["rows"] != float64(45) {
		t.Fatalf("cols/rows 必须原样上行（不夹取）：%v", got)
	}
	// 宿主拒绝时错误要原样交给调用方：壳据此在状态栏说真话，而不是假装改成。
	reject = true
	err := c.TerminalResize(ctx, "s1", "tab1", 1, 0)
	if err == nil || !strings.Contains(err.Error(), "尺寸越界") {
		t.Fatalf("越界错误必须原样返回：%v", err)
	}
}

func TestOfficialTerminalAttachRetainWatchStreams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		switch strings.TrimPrefix(r.URL.Path, "/api/") {
		case "terminal.attach":
			fmt.Fprintln(w, `{"type":"snapshot","sequence":0,"screen":"ready\r\n","info":{"id":"tab1","title":"bash","state":"running","origin":"user","control":"user"},"writable":true}`)
			fmt.Fprintln(w, `{"type":"output","sequence":1,"data":"hello\r\n"}`)
			fmt.Fprintln(w, `{"type":"state","info":{"id":"tab1","title":"bash","state":"exited","origin":"user","control":"user"},"writable":false}`)
		case "terminal.retain":
			fmt.Fprintln(w, `{"type":"retained"}`)
		case "terminal.watch":
			fmt.Fprintln(w, `[{"id":"tab1","title":"bash","state":"running","origin":"user","control":"user"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	attach, err := c.TerminalAttach(ctx, "s1", "tab1")
	if err != nil {
		t.Fatal(err)
	}
	defer attach.Close()
	snapshot, err := attach.Next()
	if err != nil || snapshot.Type != "snapshot" || snapshot.Sequence != 0 || !snapshot.Writable {
		t.Fatalf("snapshot：%+v, %v", snapshot, err)
	}
	output, err := attach.Next()
	if err != nil || output.Type != "output" || output.Sequence != 1 || output.Data != "hello\r\n" {
		t.Fatalf("output：%+v, %v", output, err)
	}
	state, err := attach.Next()
	if err != nil || state.Type != "state" || state.Info.State != "exited" || state.Writable {
		t.Fatalf("state：%+v, %v", state, err)
	}
	retain, err := c.TerminalRetain(ctx, "s1", "tab1")
	if err != nil {
		t.Fatal(err)
	}
	defer retain.Close()
	if err := retain.NextRetained(); err != nil {
		t.Fatal(err)
	}
	watch, err := c.TerminalWatch(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()
	roster, err := watch.NextRoster()
	if err != nil || len(roster) != 1 || roster[0].ID != "tab1" {
		t.Fatalf("watch：%+v, %v", roster, err)
	}
}

func TestTerminalStreamErrorIsNotEOF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"type":"error","error":{"code":"gateway/internal","message":"boom","hint":"fix","retryable":false}}`)
	}))
	defer srv.Close()
	stream, err := New(srv.URL).TerminalWatch(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.NextRoster(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("流中的 error 帧必须交给调用方：%v", err)
	}
}
