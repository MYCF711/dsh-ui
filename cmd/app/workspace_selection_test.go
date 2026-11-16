package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

func TestBlankSessionInWorkspaceUsesOnlyConfirmedBlank(t *testing.T) {
	var mu sync.Mutex
	var loads []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session.load" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		id, _ := req["sessionId"].(string)
		mu.Lock()
		loads = append(loads, id)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if id == "blank" {
			_, _ = w.Write([]byte(`{"result":{"id":"blank","cwd":"C:\\work","messages":[],"total":0}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"id":"history","cwd":"C:\\work","messages":[{"role":"user","text":"old"}],"total":1}}`))
	}))
	t.Cleanup(srv.Close)
	m := &model{cli: apiclient.New(srv.URL)}
	workspace := pluginapi.Workspace{WorkspaceID: "w1", Path: `C:\work`, SessionIDs: []string{"history", "blank"}}
	id, err := m.blankSessionInWorkspace(context.Background(), workspace, nil)
	if err != nil || id != "blank" {
		t.Fatalf("应复用宿主证实 Total=0 的 blank，实际 id=%q err=%v", id, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(loads) != 2 || loads[0] != "history" || loads[1] != "blank" {
		t.Fatalf("候选顺序应跟随 registry.sessionIds，实际 %v", loads)
	}
}

func TestBlankSessionInWorkspaceSkipsArchivedAndNeverGuessesCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/session.create" {
			t.Fatal("当前宿主只解 mode，不能拿 session.create 假装在工作区新建")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"id":"archived","cwd":"C:\\work","messages":[],"total":0}}`))
	}))
	t.Cleanup(srv.Close)
	m := &model{cli: apiclient.New(srv.URL)}
	workspace := pluginapi.Workspace{WorkspaceID: "w1", Path: `C:\work`, SessionIDs: []string{"archived"}}
	if id, err := m.blankSessionInWorkspace(context.Background(), workspace, map[string]bool{"archived": true}); !errors.Is(err, errWorkspaceSessionCreateUnsupported) || id != "" {
		t.Fatalf("归档空白会话不能复用；没有候选时必须暴露创建能力缺口，id=%q err=%v", id, err)
	}
}

func TestSelectWorkspaceFailureClearsPendingAndReportsHostBoundary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	m := &model{
		cli:        apiclient.New(srv.URL),
		workspaces: []pluginapi.Workspace{{WorkspaceID: "w-new", Path: `C:\new`, SessionIDs: nil}},
	}
	m.selectWorkspace("w-new")
	deadline := time.Now().Add(3 * time.Second)
	for {
		m.mu.Lock()
		pending, status := m.workspaceSelectionPending, m.status
		m.mu.Unlock()
		if pending == "" && status != "" {
			if !strings.Contains(status, errWorkspaceSessionCreateUnsupported.Error()) {
				t.Fatalf("状态栏没有说明真实宿主边界：%q", status)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("选择失败没有清 pending/上报状态：pending=%q status=%q", pending, status)
		}
		time.Sleep(time.Millisecond)
	}
}
