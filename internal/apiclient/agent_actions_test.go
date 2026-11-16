package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCommandsExecuteWireShapeAndOutcome(t *testing.T) {
	var path string
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("请求不是 JSON：%v", err)
		}
		_, _ = w.Write([]byte(`{"result":{"commandId":"cmd-1","result":{"kind":"success","text":"plan off","sourceEventSeq":17}}}`))
	}))
	defer srv.Close()

	execution, err := New(srv.URL).CommandsExecute(context.Background(), "sess-1", "/plan off")
	if err != nil {
		t.Fatalf("CommandsExecute：%v", err)
	}
	if path != "/api/commands.execute" {
		t.Fatalf("path=%q", path)
	}
	var agentID, line string
	_ = json.Unmarshal(body["agentId"], &agentID)
	_ = json.Unmarshal(body["line"], &line)
	if agentID != "sess-1" || line != "/plan off" {
		t.Fatalf("官方入参错位：%s", body)
	}
	var attachments []json.RawMessage
	if err := json.Unmarshal(body["submittedAttachments"], &attachments); err != nil || attachments == nil || len(attachments) != 0 {
		t.Fatalf("submittedAttachments 必须显式为 []：raw=%s err=%v", body["submittedAttachments"], err)
	}
	if len(body) != 3 {
		t.Fatalf("只能发送 agentId,line,submittedAttachments：%v", body)
	}
	if execution == nil || execution.CommandID != "cmd-1" || execution.Result.Kind != "success" || execution.Result.Text != "plan off" {
		t.Fatalf("执行结果没解对：%+v", execution)
	}
	if execution.Result.SourceEventSeq == nil || *execution.Result.SourceEventSeq != 17 {
		t.Fatalf("sourceEventSeq 没解对：%+v", execution.Result.SourceEventSeq)
	}
}

func TestCommandsExecuteNullOrUndefinedIsUnknown(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "null result", body: `{"result":null}`},
		{name: "direct null", body: `null`},
		{name: "undefined empty body", body: ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			execution, err := New(srv.URL).CommandsExecute(context.Background(), "s1", "/nope")
			if err != nil || execution != nil {
				t.Fatalf("未知命令必须返回 (nil,nil)，得到 execution=%+v err=%v", execution, err)
			}
		})
	}
}

func TestUserQuestionsAnswerWireShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/userQuestions.answer" {
			t.Errorf("path=%q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("请求不是 JSON：%v", err)
		}
		_, _ = w.Write([]byte(`{"result":true}`))
	}))
	defer srv.Close()

	answer := QuestionAnswer{Answers: []QuestionAnswerItem{{
		ID: "q1", Selected: []string{"批准", "继续"}, Custom: "补充说明",
	}}}
	continued, err := New(srv.URL).UserQuestionsAnswer(context.Background(), "sess-1", "call-1", answer)
	if err != nil || !continued {
		t.Fatalf("UserQuestionsAnswer = (%v,%v)，want (true,nil)", continued, err)
	}
	want := map[string]any{
		"agentId": "sess-1",
		"callId":  "call-1",
		"answer": map[string]any{"answers": []any{map[string]any{
			"id": "q1", "selected": []any{"批准", "继续"}, "custom": "补充说明",
		}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("官方入参不一致：\n got=%#v\nwant=%#v", got, want)
	}
}

func TestUserQuestionsAnswerFalseIsBusinessResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`false`))
	}))
	defer srv.Close()
	continued, err := New(srv.URL).UserQuestionsAnswer(context.Background(), "s1", "c1", QuestionAnswer{})
	if err != nil || continued {
		t.Fatalf("false 必须原样返回且不是错误：(%v,%v)", continued, err)
	}
}

func TestViewSpecPersistentQuestions(t *testing.T) {
	var view ViewSpec
	raw := `{
		"kind":"plain",
		"awaiting_answer":true,
		"questions":[{
			"id":"q-plan","header":"计划评审","question":"批准计划吗？","detail":"# 计划\n正文",
			"options":[{"label":"批准","description":"执行计划"}],
			"multi_select":false,
			"intent":{"kind":"plan-review","approve":"批准","callId":"call-plan"}
		}]
	}`
	if err := json.Unmarshal([]byte(raw), &view); err != nil {
		t.Fatal(err)
	}
	if !view.AwaitingAnswer || len(view.Questions) != 1 {
		t.Fatalf("持久问题没解出来：%+v", view)
	}
	q := view.Questions[0]
	if q.ID != "q-plan" || q.Header != "计划评审" || q.Question != "批准计划吗？" || q.Detail != "# 计划\n正文" || q.MultiSelect {
		t.Fatalf("QuestionItem 字段错位：%+v", q)
	}
	if len(q.Options) != 1 || q.Options[0] != (QuestionOption{Label: "批准", Description: "执行计划"}) {
		t.Fatalf("QuestionOption 字段错位：%+v", q.Options)
	}
	if q.Intent == nil || *q.Intent != (QuestionIntent{Kind: "plan-review", Approve: "批准", CallID: "call-plan"}) {
		t.Fatalf("QuestionIntent 字段错位：%+v", q.Intent)
	}
}
