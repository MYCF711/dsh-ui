package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCredentialUnsetWireShape 钉官方 `credentials.unset` 的线上形状：
// `POST /api/credentials.unset`、裸参数 `{"ref": "..."}`、成功信封是 `{}`（结果 void）。
//
// 官方调用点 `OFF:operations.ts:82-85`；宿主契约
// `../DSH-GO/internal/transport/handlers_settings_official.go:625`。
func TestCredentialUnsetWireShape(t *testing.T) {
	var path string
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("请求不是 JSON：%v", err)
		}
		// 官方结果是 void：宿主回的就是空信封。
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()

	if err := New(srv.URL).CredentialUnset(context.Background(), "DEEPSEEK_API_KEY"); err != nil {
		t.Fatalf("CredentialUnset：%v", err)
	}
	if path != "/api/credentials.unset" {
		t.Fatalf("path=%q，官方方法名就是 credentials.unset", path)
	}
	if len(body) != 1 {
		t.Fatalf("只能发送 ref：%v", body)
	}
	var ref string
	if err := json.Unmarshal(body["ref"], &ref); err != nil || ref != "DEEPSEEK_API_KEY" {
		t.Fatalf("ref = %q（err=%v）", ref, err)
	}
}

// TestCredentialUnsetSurfacesHostRefusal 钉宿主拒绝原样回给调用方（不吞成成功）：
// 删不掉时界面必须能说清，而不是假装删了。
func TestCredentialUnsetSurfacesHostRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":{"code":"credentials/refused","message":"凭据库只读","retryable":false}}`))
	}))
	defer srv.Close()

	err := New(srv.URL).CredentialUnset(context.Background(), "REF")
	if err == nil {
		t.Fatal("宿主拒绝必须变成错误")
	}
	if apiErr, ok := err.(*APIError); ok {
		if apiErr.Code != "credentials/refused" || apiErr.Message != "凭据库只读" {
			t.Fatalf("四字段没解对：%+v", apiErr)
		}
	}
}
