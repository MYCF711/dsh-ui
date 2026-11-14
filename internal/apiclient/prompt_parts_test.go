// prompt_parts_test.go —— 官方 PromptContentPart 两支的客户端契约。
//
// 钉住三件事：
//  1. `fileUploads.upload` 发的是**平铺** {sessionId,data,name}，data 是完整字节的 base64；
//  2. 回执按 `{receiptId, file:{attachmentId,name,bytes}}` 解析；
//  3. `PromptParts` 的 content 顺序是**附件在前、文本最后**（官方 service.ts:302）。
package apiclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFileUploadsUploadSendsFlatBase64AndParsesReceipt(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/fileUploads.upload" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("请求不是合法 JSON：%v", err)
		}
		_, _ = io.WriteString(w, `{"receiptId":"rcpt-11111111-2222-3333-4444-555555555555",`+
			`"file":{"attachmentId":"sha-abc","name":"a.txt","bytes":5}}`)
	}))
	defer ts.Close()

	client := New(ts.URL)
	receipt, err := client.FileUploadsUpload(context.Background(), "s1", []byte("hello"), "a.txt")
	if err != nil {
		t.Fatalf("FileUploadsUpload 失败：%v", err)
	}
	if body["sessionId"] != "s1" {
		t.Errorf("sessionId = %v", body["sessionId"])
	}
	if body["data"] != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Errorf("data 不是完整字节的 base64：%v", body["data"])
	}
	if body["name"] != "a.txt" {
		t.Errorf("name = %v", body["name"])
	}
	if _, nested := body["request"]; nested {
		t.Error("本仓契约是平铺，不应发 request 嵌套")
	}
	if receipt.ReceiptID != "rcpt-11111111-2222-3333-4444-555555555555" ||
		receipt.File.AttachmentID != "sha-abc" || receipt.File.Bytes != 5 || receipt.File.Name != "a.txt" {
		t.Fatalf("回执解析不对：%+v", receipt)
	}

	// 本地校验：空会话 / 空字节不发出请求。
	if _, err := client.FileUploadsUpload(context.Background(), "", []byte("x"), "a"); err == nil {
		t.Fatal("空 sessionId 应本地报错")
	}
	if _, err := client.FileUploadsUpload(context.Background(), "s1", nil, "a"); err == nil {
		t.Fatal("空字节应本地报错")
	}
}

func TestPromptPartsPutsAttachmentsBeforeText(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session.prompt" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("请求不是合法 JSON：%v", err)
		}
		_, _ = io.WriteString(w, `{"turnId":"turn-1","accepted":true}`)
	}))
	defer ts.Close()

	client := New(ts.URL)
	turn, err := client.PromptParts(context.Background(), "s1", "看这两个文件", []PromptPart{
		{Type: "file", ReceiptID: "rcpt-a"},
		{Type: "file", ReceiptID: "rcpt-b"},
	})
	if err != nil {
		t.Fatalf("PromptParts 失败：%v", err)
	}
	if turn != "turn-1" {
		t.Fatalf("turnId = %q", turn)
	}
	content, ok := body["content"].([]any)
	if !ok {
		t.Fatalf("content 不是数组：%v", body["content"])
	}
	if len(content) != 3 {
		t.Fatalf("content 应有 2 附件 + 1 文本，实际 %d：%v", len(content), content)
	}
	first, _ := content[0].(map[string]any)
	second, _ := content[1].(map[string]any)
	last, _ := content[2].(map[string]any)
	if first["type"] != "file" || first["receiptId"] != "rcpt-a" {
		t.Errorf("第一段应是 rcpt-a 的 file：%v", first)
	}
	if second["type"] != "file" || second["receiptId"] != "rcpt-b" {
		t.Errorf("第二段应是 rcpt-b 的 file：%v", second)
	}
	if last["type"] != "text" || last["text"] != "看这两个文件" {
		t.Errorf("文本必须在最后：%v", last)
	}
	if _, has := body["text"]; has {
		t.Error("content 存在的请求不应再带顶层 text（会形成第二种内容形状）")
	}
}

func TestPromptPartsFallsBackToPlainPrompt(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = io.WriteString(w, `{"turnId":"t","accepted":true}`)
	}))
	defer ts.Close()
	client := New(ts.URL)
	if _, err := client.PromptParts(context.Background(), "s1", "hi", nil); err != nil {
		t.Fatal(err)
	}
	if body["text"] != "hi" {
		t.Fatalf("没有附件时应退回旧 Prompt（text 字段），实际 %v", body)
	}
	if !strings.EqualFold(strings.TrimSpace(body["id"].(string)), "s1") {
		t.Fatalf("id = %v", body["id"])
	}
}
