// official_panels_wiring_test.go —— 壳给附件 rail 注入的上传能力（#6 官方主路径）。
//
// 只钉一件事：`attachSink.Upload` 必须先把 rail 已暂存的附件字节取回来
// （`attachment.get`），再走 `fileUploads.upload` 换会话回执；不能凭路径重读，
// 也不能把 sessionId 弄丢（回执只在它所属的会话里有效）。
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestAttachSinkUploadFetchesStoredBytesThenUploadsReceipt(t *testing.T) {
	var order []string
	var getBody map[string]any
	var uploadBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/api/attachment.get":
			order = append(order, "get")
			_ = json.Unmarshal(raw, &getBody)
			_, _ = io.WriteString(w, `{"attachment":{"id":"att-1","name":"a.txt","byteSize":5,`+
				`"mime":"text/plain","sha256":"abc"},"dataBase64":"`+base64.StdEncoding.EncodeToString([]byte("hello"))+`"}`)
		case "/api/fileUploads.upload":
			order = append(order, "upload")
			_ = json.Unmarshal(raw, &uploadBody)
			_, _ = io.WriteString(w, `{"receiptId":"rcpt-11111111-2222-3333-4444-555555555555",`+
				`"file":{"attachmentId":"att-1","name":"a.txt","bytes":5}}`)
		default:
			t.Errorf("意外路径 %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	sink := attachSink{cli: apiclient.New(ts.URL)}
	receipt, err := sink.Upload(context.Background(), "s1",
		apiclient.Attachment{ID: "att-1", Name: "a.txt", ByteSize: 5, MIME: "text/plain"})
	if err != nil {
		t.Fatalf("attachSink.Upload 失败：%v", err)
	}
	if len(order) != 2 || order[0] != "get" || order[1] != "upload" {
		t.Fatalf("必须先 attachment.get 再 fileUploads.upload，实际 %v", order)
	}
	if getBody["id"] != "att-1" {
		t.Fatalf("attachment.get 的 id=%v", getBody["id"])
	}
	if uploadBody["sessionId"] != "s1" || uploadBody["name"] != "a.txt" ||
		uploadBody["data"] != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Fatalf("fileUploads.upload 请求不对：%v", uploadBody)
	}
	if receipt.ReceiptID == "" || receipt.File.AttachmentID != "att-1" {
		t.Fatalf("回执没交给调用方：%+v", receipt)
	}
}
