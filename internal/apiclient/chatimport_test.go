package apiclient

// chatimport_test.go —— 用**真宿主回的字节**钉住 NDJSON 应答的解析。
//
// 夹具不是手写的：`testdata/chatimport/*.ndjson` 是 `DSH-GO-vi` 仓里
// `tmp/GO8-RC5/_vi/hostverify` 那支真机探针把真 dshd（临时 home、`--addr 127.0.0.1:0`）
// 的应答正文原样留档后拷过来的。所以这里验的是"宿主真的这么吐、我们真的这么读"，
// 而不是"mock 恰好这么吐"。
//
// 契约原件（rc.5 解包树 OFF = node_modules/@deepseek-ai/dsh-deepseek-chat-import）：
//
//	OFF:src/types.ts:65-73    一行 NDJSON 只有 progress{done,total} 或 outcome{outcome}
//	OFF:src/types.ts:56-58    outcome = {ok:true,value} | {ok:false,error{reason,message}}
//	OFF:src/types.ts:13-21    八个原因码（too-large / unreadable-archive / missing-manifest /
//	                          missing-index / unsupported-version / no-conversations /
//	                          workspace-not-found / busy）
//	OFF:src/upload.ts:118     进度行先发，outcome 行恰好一行且最后
//	OFF:src/client/upload.ts:92-96  没有 outcome 行 = 应答不完整，**不**当成功

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// noProgress 是"不关心进度"的回调。
//
// 为什么不用 nil：`readChatImportAnswer` 在遇到 progress 行时会**直接调用**回调，
// 生产路径由 `ChatImportArchive` 保证了非 nil（它把 nil 换成空函数），所以这里也
// 传空函数——拿 nil 去测等于测一条生产上不存在的路径。
func noProgress(ChatImportProgress) {}

// readChatImportFixture 读一份夹具；路径按 testdata 目录拼，不依赖工作目录之外的任何东西。
func readChatImportFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "chatimport", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读夹具 %s 失败: %v", path, err)
	}
	return data
}

// TestChatImportAnswerReadsRealHostSuccess 钉住成功应答：进度阶段 + 结果字段。
func TestChatImportAnswerReadsRealHostSuccess(t *testing.T) {
	var progress []ChatImportProgress
	result, err := readChatImportAnswer(strings.NewReader(string(readChatImportFixture(t, "main.ndjson"))), func(item ChatImportProgress) {
		progress = append(progress, item)
	})
	if err != nil {
		t.Fatalf("解析真宿主成功应答失败: %v", err)
	}
	// 结果字段逐条对官方 types.ts:34-47。
	if len(result.SessionIDs) != 2 || result.ConversationCount != 2 || result.OmittedByLimit != 0 {
		t.Fatalf("结果字段不对: %+v", result)
	}
	if len(result.Skipped) != 0 {
		t.Fatalf("skipped 应为空: %+v", result.Skipped)
	}
	if result.WorkspaceID == "" || result.WorkspacePath == "" {
		t.Fatalf("workspaceId/workspacePath 不能为空: %+v", result)
	}
	// 宿主只发 progress 行给客户端，客户端把它们投影成 importing 段。
	if len(progress) != 3 {
		t.Fatalf("进度回调 %d 次，期望 3（done=0,1,2）: %+v", len(progress), progress)
	}
	for index, item := range progress {
		if item.Phase != "importing" {
			t.Fatalf("第 %d 次回调阶段=%q，宿主 progress 行应投影成 importing", index, item.Phase)
		}
		if item.Done != index || item.Total != 2 {
			t.Fatalf("第 %d 次回调 done/total=%d/%d，期望 %d/2", index, item.Done, item.Total, index)
		}
	}
}

// TestChatImportAnswerKeepsOfficialReason 钉住失败应答的原因码原样透出。
func TestChatImportAnswerKeepsOfficialReason(t *testing.T) {
	cases := []struct {
		fixture string
		want    string
	}{
		{"d1-missing-manifest.ndjson", "missing-manifest"},
		{"d4-oversized-declaration.ndjson", "too-large"},
	}
	for _, item := range cases {
		_, err := readChatImportAnswer(strings.NewReader(string(readChatImportFixture(t, item.fixture))), noProgress)
		if err == nil {
			t.Fatalf("%s：期望失败，却成功", item.fixture)
		}
		// 原因码是界面选本地化文案的依据（chat_import_actions.go 的 chatImportFailureText），
		// 丢了它界面只能显示原始 message。
		if got := ChatImportFailureReason(err); got != item.want {
			t.Fatalf("%s：原因码=%q，期望 %q（err=%v）", item.fixture, got, item.want, err)
		}
		if !strings.HasPrefix(err.Error(), item.want+": ") {
			t.Fatalf("%s：错误文本没带原因码前缀: %v", item.fixture, err)
		}
	}
}

// TestChatImportAnswerSuccessWithSkipped 钉住"成功但有条目没导入"：
// 索引里有、归档里没有的对话写进 skipped，整体仍是 ok:true
// （官方 importer.ts:263-267）。
func TestChatImportAnswerSuccessWithSkipped(t *testing.T) {
	result, err := readChatImportAnswer(strings.NewReader(string(readChatImportFixture(t, "d5-missing-conversation.ndjson"))), noProgress)
	if err != nil {
		t.Fatalf("这条应答应算成功: %v", err)
	}
	if len(result.SessionIDs) != 1 || result.ConversationCount != 2 {
		t.Fatalf("结果字段不对: %+v", result)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("skipped=%d，期望 1: %+v", len(result.Skipped), result.Skipped)
	}
	skip := result.Skipped[0]
	if skip.ConversationID == "" || !strings.Contains(skip.Reason, "no file for this conversation") {
		t.Fatalf("跳过项不对: %+v", skip)
	}
	// title 缺失时是 null，不是空串（官方 types.ts:26-30）。
	if skip.Title != nil {
		t.Fatalf("没解析出标题时 title 必须是 null，拿到 %q", *skip.Title)
	}
}

// TestChatImportAnswerRejectsIncompleteAnswers 钉住"没有结论就不算成功"
// （官方 client/upload.ts:92-96 的 'ended the answer without a result'）。
func TestChatImportAnswerRejectsIncompleteAnswers(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		// 只有进度行：上传被中断（官方 upload.ts:124-125 在客户端断开时同样不写 outcome）。
		{"没有 outcome 行", `{"type":"progress","done":0,"total":2}` + "\n", "没有 outcome 行"},
		// 注：`readChatImportAnswer` 里那条 "宿主回了不止一行 outcome" 的检查目前**不可达**
		// ——成功分支在读到第一行 outcome 时就 return 了，第二行根本不会被读到。官方宿主
		// 也只发一行（types.ts:60-64），所以这里不把它写成断言（写上去就是钉一个假事实），
		// 已作为"健壮性观察"上报，不在本次改动面里。
		// 非 JSON：不是协议值，不能当进度吞掉。
		{"不是 JSON", "<html>gateway</html>\n", "不是 JSON"},
		// 未知行类型：官方只有 progress/outcome 两种（types.ts:65-73）。
		{"未知行类型", `{"type":"progress-v2","done":0,"total":1}` + "\n", "未知的导入行类型"},
		// ok=false 但没给原因：界面无从报错。
		{"失败无原因", `{"type":"outcome","outcome":{"ok":false}}` + "\n", "没给原因"},
		// ok=true 但没给结果：不能凭空造一个空结果冒充成功。
		{"成功无结果", `{"type":"outcome","outcome":{"ok":true}}` + "\n", "没给结果"},
	}
	for _, item := range cases {
		_, err := readChatImportAnswer(strings.NewReader(item.body), noProgress)
		if err == nil {
			t.Fatalf("%s：期望失败，却成功", item.name)
		}
		if !strings.Contains(err.Error(), item.want) {
			t.Fatalf("%s：错误=%v，期望包含 %q", item.name, err, item.want)
		}
	}
}

// TestChatImportUploadPathMatchesOfficial 钉住上传路由字符串（官方 protocol.ts:4）。
func TestChatImportUploadPathMatchesOfficial(t *testing.T) {
	if ChatImportUploadPath != "/api/deepseek-chat-import/archive" {
		t.Fatalf("上传路由=%q，官方 protocol.ts:4 是 /api/deepseek-chat-import/archive", ChatImportUploadPath)
	}
}
