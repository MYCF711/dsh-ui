package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestExecuteCommandActionOutcomeAndTransportErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantErr    string
		wantAPIErr string
		refresh    bool
	}{
		{name: "success", body: `{"result":{"commandId":"cmd-1","result":{"kind":"success","text":"disabled"}}}`, refresh: true},
		{name: "command outcome error", body: `{"result":{"commandId":"cmd-2","result":{"kind":"error","text":"计划模式关闭失败"}}}`, wantErr: "计划模式关闭失败"},
		{name: "unknown null", body: `{"result":null}`, wantErr: "unknown command: /plan off"},
		{name: "transport error", status: http.StatusInternalServerError, body: `{"error":{"code":"internal","message":"command failed","hint":"","retryable":false}}`, wantAPIErr: "internal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var request map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("请求不是 JSON：%v", err)
				}
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			m := &model{cli: apiclient.New(srv.URL), cur: "sess-current"}
			oldPlan := &apiclient.PlanProjection{Active: true, Pending: true}
			m.wave3.projSession = "sess-current"
			m.wave3.projPlan = oldPlan
			m.wave3.projAt = time.Now()
			err := m.executeCommandLine("/plan off")
			if tc.wantAPIErr != "" {
				var apiErr *apiclient.APIError
				if !errors.As(err, &apiErr) || apiErr.Code != tc.wantAPIErr {
					t.Fatalf("Transport 错误必须原样冒泡：%T %v", err, err)
				}
			} else if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("err=%v，want %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("成功 outcome 被当成错误：%v", err)
			}
			if request["agentId"] != "sess-current" || request["line"] != "/plan off" {
				t.Fatalf("没有使用点击时的当前会话：%v", request)
			}
			if got := request["submittedAttachments"]; !reflect.DeepEqual(got, []any{}) {
				t.Fatalf("submittedAttachments=%#v，want []", got)
			}
			if tc.refresh != m.wave3.projAt.IsZero() {
				t.Fatalf("成功后才应清零 projAt：refresh want=%v projAt=%v", tc.refresh, m.wave3.projAt)
			}
			if m.wave3.projPlan != oldPlan {
				t.Fatal("动作不得本地乐观改写 plan；必须等投影重读")
			}
		})
	}
}

func TestAnswerQuestionActionUsesCurrentSession(t *testing.T) {
	var request map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("请求不是 JSON：%v", err)
		}
		_, _ = w.Write([]byte(`{"result":false}`))
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL), cur: "sess-current"}
	answer := apiclient.QuestionAnswer{Answers: []apiclient.QuestionAnswerItem{{
		ID: "q1", Selected: []string{"批准"}, Custom: "现在执行",
	}}}
	continued, err := m.answerQuestion("call-current", answer)
	if err != nil || continued {
		t.Fatalf("answerQuestion=(%v,%v)，want (false,nil)", continued, err)
	}
	want := map[string]any{
		"agentId": "sess-current", "callId": "call-current",
		"answer": map[string]any{"answers": []any{map[string]any{
			"id": "q1", "selected": []any{"批准"}, "custom": "现在执行",
		}}},
	}
	if !reflect.DeepEqual(request, want) {
		t.Fatalf("回答入参不对：\n got=%#v\nwant=%#v", request, want)
	}
}

func TestWave3PlanCacheAndPollingWithoutTeam(t *testing.T) {
	var fail bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"boom","hint":"","retryable":true}}`))
			return
		}
		_, _ = w.Write([]byte(`{"asOfSeq":8,"values":{"plan":{"active":true,"pending":true}}}`))
	}))
	defer srv.Close()

	w := newWave3Wiring() // team == nil：dsplan 独立触发投影拉取
	m := &model{cli: apiclient.New(srv.URL)}
	account, projections := w.refreshDue(m, "sess-a")
	if account || !projections {
		t.Fatalf("team=nil 也必须调度 projections：account=%v projections=%v", account, projections)
	}
	w.fetchProjections(m, "sess-a")
	plan := w.planSnapshot("sess-a")
	if plan == nil || !plan.Active || !plan.Pending {
		t.Fatalf("plan 没进缓存：%+v", plan)
	}
	if got := w.planSnapshot("sess-b"); got != nil {
		t.Fatalf("切会话不能串 plan：%+v", got)
	}
	if _, again := w.refreshDue(m, "sess-a"); again {
		t.Fatal("既有 10s 节流被绕过")
	}

	fail = true
	w.fetchProjections(m, "sess-a")
	if got := w.planSnapshot("sess-a"); got != nil {
		t.Fatalf("投影失败必须回到未知，不能沿用旧 plan：%+v", got)
	}
}
