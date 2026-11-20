// attachment_bytes_test.go —— 消息区图片面取数入口的 wire 断言（`#18` ②③ 壳侧）。
//
// 钉三件事：
//
//	① 发的是宿主真形状：平铺 `{"id": …}`（不是 `{"params": …}`）；
//	② 回的是**真字节**：`dataBase64` 解码后与宿主给的一致，元数据逐字段带上
//	   （尤其 width/height —— 图片面的 singleFit 全靠这两个数）；
//	③ 0 字节 / 没接线 / 空 id 一律**如实失败**，不许当成功（界面据此画官方失败态）。
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// solidPNG 编一张真 PNG（宽×高）。
func solidPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			source.Set(x, y, color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		t.Fatalf("编 PNG 失败：%v", err)
	}
	return buf.Bytes()
}

// TestAttachmentGetReturnsRealBytes：宿主回真字节 + 真尺寸，壳原样交给插件。
func TestAttachmentGetReturnsRealBytes(t *testing.T) {
	body := solidPNG(t, 240, 180)
	var gotID string
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("/api/attachment.get", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		mu.Lock()
		gotID = in["id"]
		mu.Unlock()
		if _, wrapped := in["params"]; wrapped {
			t.Errorf("请求被包了 params 信封：宿主的 finish() 是 DisallowUnknownFields，会 400")
		}
		_, _ = w.Write([]byte(`{"attachment":{"id":"att-1","name":"shot.png","byteSize":` +
			strconv.Itoa(len(body)) + `,"mime":"image/png","sha256":"abc","width":240,"height":180},` +
			`"dataBase64":"` + base64.StdEncoding.EncodeToString(body) + `"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	blob, ok := m.attachmentGetAction("att-1")
	if !ok {
		t.Fatal("宿主有真字节，必须成功")
	}
	mu.Lock()
	id := gotID
	mu.Unlock()
	if id != "att-1" {
		t.Fatalf("请求 id = %q", id)
	}
	if blob.ID != "att-1" || blob.Name != "shot.png" || blob.MIME != "image/png" {
		t.Fatalf("元数据 = %+v", blob)
	}
	if blob.Width != 240 || blob.Height != 180 {
		t.Fatalf("尺寸 = %dx%d，期望 240x180（singleFit 靠它）", blob.Width, blob.Height)
	}
	if !bytes.Equal(blob.Bytes, body) {
		t.Fatalf("字节不一致：拿回 %d 字节，宿主给 %d 字节", len(blob.Bytes), len(body))
	}
}

// TestAttachmentGetZeroBytesIsFailure：0 字节算失败，不许当"成功但没有内容"。
//
// 宿主根本不登记 0 字节附件（`HOST:cmd/dshd/attachments.go:129-131`），
// 拿到 0 字节只可能是链上出了问题。
func TestAttachmentGetZeroBytesIsFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/attachment.get", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"attachment":{"id":"att-0","name":"empty.png","byteSize":0,"mime":"image/png"},"dataBase64":""}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	if _, ok := m.attachmentGetAction("att-0"); ok {
		t.Fatal("0 字节必须算失败")
	}
}

// TestAttachmentGetWithoutClientOrID：两条本地防御。
func TestAttachmentGetWithoutClientOrID(t *testing.T) {
	if _, ok := (&model{}).attachmentGetAction("att-1"); ok {
		t.Fatal("没有 apiclient 时必须失败（不静默当成功）")
	}
	m := &model{cli: apiclient.New("http://127.0.0.1:1")}
	if _, ok := m.attachmentGetAction("  "); ok {
		t.Fatal("空 id 必须失败")
	}
}
