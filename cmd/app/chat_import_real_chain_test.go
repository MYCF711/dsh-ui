//go:build windows

package main

// chat_import_real_chain_test.go —— 官方 rc.5 `dsh-deepseek-chat-import` 的**真机**验收：
// 真 dshd serve + 真导出 ZIP + 真 `internal/apiclient`（GUI 客户端那两支方法）。
//
// 补的是一条此前**明确标注未验证**的证据：dsh-go#71 / dsh-ui#168 合并时只有合成 ZIP
// 与单测，没有"真宿主 + 真导出归档 + 真客户端"的端到端。
//
// 跑法（不给 DSHD_BIN 就跳过，CI 上没有宿主二进制）：
//
//	export DSHD_BIN=".../DSH-GO-vi/tmp/GO8-RC5/_vi/dshd.exe"
//	go test ./cmd/app -run TestRealChainChatImport -count=1 -v
//
// 导出归档默认取 `DSH-GO-vi/tmp/GO8-RC5/_vi/real/deepseek_data-20261118.zip`
// （由同目录 `make_export_zip.py` 按官方 manifest v3 结构生成，可复跑），
// 也可以用 `DSHD_IMPORT_ZIP` 指定别的真导出。
//
// 与别的 `*_real_chain_test.go` 的区别：**这条链不需要模型**。导入是纯归档转换
// （官方 OFF:src/importer.ts 一次 create+append+flush），所以这里不探 magpie 网关
// ——少一个"网关连不上就跳过"的理由。
//
// 官方契约（rc.5 解包树 OFF = node_modules/@deepseek-ai/dsh-deepseek-chat-import）：
//
//	OFF:src/protocol.ts:4           上传路由 /api/deepseek-chat-import/archive
//	OFF:src/types.ts:50-53          limits 结果 {maxUploadBytes}
//	OFF:src/types.ts:34-47          结果字段 workspaceId/workspacePath/sessionIds/
//	                                conversationCount/omittedByLimit/skipped
//	OFF:src/client/upload.ts:7-11   客户端三段进度 uploading / reading / importing
//	OFF:src/upload.ts:118           宿主只发 progress 行 + 恰好一行 outcome
//	OFF:src/importer.ts:434-439     会话按 updated_at 降序

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// chatImportDefaultZip 是 A 那份归档在宿主仓里的默认位置（与 make_export_zip.py 同目录）。
const chatImportDefaultZip = "D:/DeepSeek Harness/default-workspace/DSH-GO-vi/tmp/GO8-RC5/_vi/real/deepseek_data-20261118.zip"

// importHost 是一台只服务导入的真 dshd（不接模型）。
type importHost struct {
	url   string
	token string
	home  string
	cmd   *exec.Cmd
}

// startImportHost 起一台真宿主：临时 home、127.0.0.1:0（端口由 OS 给）、关父进程看门狗。
//
// 三条铁律（AGENTS §4）：临时 home、显式 127.0.0.1:0、只回收自己起的进程。
func startImportHost(t *testing.T) *importHost {
	t.Helper()
	bin := os.Getenv("DSHD_BIN")
	if bin == "" {
		t.Skip("没有 DSHD_BIN（宿主二进制），跳过真机验收")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("DSHD_BIN 指向的二进制不存在：%v", err)
	}
	// 临时 home 放宿主仓的 tmp 下：本机 %TEMP% 在 C:/Users/ADMINI~1 下，
	// dshd 在里面写 sessions/state 会一律 Access is denied（gt3 踩过）。
	homeRoot := os.Getenv("DSHD_HOME_ROOT")
	if homeRoot == "" {
		homeRoot = "D:/DeepSeek Harness/default-workspace/DSH-GO-vi/tmp/GO8-RC5/_vi/run"
	}
	if err := os.MkdirAll(homeRoot, 0o755); err != nil {
		t.Fatalf("建临时 home 根目录失败：%v", err)
	}
	home, err := os.MkdirTemp(homeRoot, "gui-home-")
	if err != nil {
		t.Fatalf("建临时 home 失败：%v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })

	cmd := exec.Command(bin, "serve",
		"--home", home,
		"--addr", "127.0.0.1:0",
		"--parent-watch=false",
		"--policy", "allow",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("取 stdout 失败：%v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("起不了 dshd serve：%v", err)
	}
	host := &importHost{home: home, cmd: cmd}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})

	// ready 行：`ready {"url":…,"token":…}`（stdout 只放这一行，日志走 stderr）。
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "ready ") {
				ready <- strings.TrimPrefix(line, "ready ")
				return
			}
		}
		close(ready)
	}()
	select {
	case raw, ok := <-ready:
		if !ok {
			t.Fatal("dshd serve 没打出 ready 行就退出了")
		}
		var parsed struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			t.Fatalf("ready 行解不开：%v（%q）", err, raw)
		}
		host.url, host.token = parsed.URL, parsed.Token
	case <-time.After(60 * time.Second):
		t.Fatal("等 ready 行超时（60s）")
	}
	return host
}

// TestRealChainChatImport 是 C 段的核心：GUI 客户端把 A 那份真 ZIP 发给真宿主，
// 断言结果、进度阶段与"导入后列会话能看到原标题"。
func TestRealChainChatImport(t *testing.T) {
	zipPath := os.Getenv("DSHD_IMPORT_ZIP")
	if zipPath == "" {
		zipPath = chatImportDefaultZip
	}
	info, err := os.Stat(zipPath)
	if err != nil {
		t.Skipf("没有导出归档（%v），先用 make_export_zip.py 生成或设 DSHD_IMPORT_ZIP", err)
	}
	if info.Size() == 0 {
		t.Fatalf("%s 是空文件", zipPath)
	}

	host := startImportHost(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client := apiclient.New(host.url)
	if err := client.Connect(ctx, host.token); err != nil {
		t.Fatalf("apiclient.Connect 失败：%v", err)
	}

	// 目标工作区：真目录，再注册成工作区（官方 workspace/create）。
	workspacePath := filepath.Join(host.home, "workspace")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("建工作区目录失败：%v", err)
	}
	workspace, err := client.WorkspaceCreate(ctx, workspacePath)
	if err != nil {
		t.Fatalf("workspace.create 失败：%v", err)
	}
	if workspace.WorkspaceID == "" {
		t.Fatal("workspace.create 没给 workspaceId")
	}

	// ---- limits：官方 Config 默认 512 MiB（OFF:src/index.ts:57-66） ----
	limits, err := client.ChatImportLimits(ctx)
	if err != nil {
		t.Fatalf("deepseekChatImport/limits 失败：%v", err)
	}
	if got, want := limits.MaxUploadBytes, int64(512*1024*1024); got != want {
		t.Fatalf("maxUploadBytes=%d，官方默认 %d", got, want)
	}
	if info.Size() > limits.MaxUploadBytes {
		t.Fatalf("归档 %d 字节超过上限 %d", info.Size(), limits.MaxUploadBytes)
	}

	// ---- 上传并跟随 NDJSON 应答 ----
	var phases []apiclient.ChatImportProgress
	result, err := client.ChatImportArchive(ctx, workspace.WorkspaceID, zipPath, func(progress apiclient.ChatImportProgress) {
		phases = append(phases, progress)
	})
	if err != nil {
		t.Fatalf("ChatImportArchive 失败：%v", err)
	}

	// 结果字段逐条对官方 types.ts:34-47。
	if result.WorkspaceID != workspace.WorkspaceID {
		t.Fatalf("workspaceId=%q，请求的是 %q", result.WorkspaceID, workspace.WorkspaceID)
	}
	if len(result.SessionIDs) != 2 {
		t.Fatalf("sessionIds=%v，期望 2 个（A 与 B 两个对话）", result.SessionIDs)
	}
	if result.ConversationCount != 2 {
		t.Fatalf("conversationCount=%d，期望 2", result.ConversationCount)
	}
	if result.OmittedByLimit != 0 {
		t.Fatalf("omittedByLimit=%d，期望 0", result.OmittedByLimit)
	}
	if len(result.Skipped) != 0 {
		t.Fatalf("skipped=%v，期望空", result.Skipped)
	}

	// ---- 进度阶段：官方三段词汇 + 顺序 ----
	//
	// 官方客户端的三段（OFF:src/client/upload.ts:7-11）：uploading（本机发字节）、
	// reading（字节发完、宿主在读归档）、importing（宿主在写会话）。宿主只发
	// progress/outcome 两种行（OFF:src/upload.ts:118），reading 由客户端在
	// "上传完成"那一刻自己发。
	officialPhases := map[string]bool{"uploading": true, "reading": true, "importing": true}
	seen := make([]string, 0, len(phases))
	sawUploading, sawImporting := false, false
	importingDone := make([]int, 0, 4)
	for _, progress := range phases {
		if !officialPhases[progress.Phase] {
			t.Fatalf("进度阶段 %q 不在官方三段 uploading/reading/importing 里", progress.Phase)
		}
		if len(seen) == 0 || seen[len(seen)-1] != progress.Phase {
			seen = append(seen, progress.Phase)
		}
		switch progress.Phase {
		case "uploading":
			sawUploading = true
		case "importing":
			sawImporting = true
			if len(importingDone) == 0 || importingDone[len(importingDone)-1] != progress.Done {
				importingDone = append(importingDone, progress.Done)
			}
		}
	}
	// 观测到的阶段序列进测试日志：缺了哪一段在 -v 里一眼能看到。
	t.Logf("进度阶段序列：%v（%d 次回调）", seen, len(phases))
	if !sawUploading {
		t.Fatal("一次 uploading 阶段都没报：客户端没把字节进度交出去")
	}
	if !sawImporting {
		t.Fatal("一次 importing 阶段都没报：宿主 progress 行没被解析")
	}
	if seen[0] != "uploading" {
		t.Fatalf("第一个阶段是 %q，官方顺序是 uploading 打头（OFF:src/client/upload.ts:114,210）", seen[0])
	}
	// 宿主进度行 done=0..2（官方 importer 在选完之后发 0，每写完一条发一次）。
	if len(importingDone) != 3 {
		t.Fatalf("importing 的 done 序列=%v，期望 [0 1 2]", importingDone)
	}
	for index, done := range importingDone {
		if done != index {
			t.Fatalf("importing 的 done 序列=%v，期望 [0 1 2]", importingDone)
		}
	}
	for _, progress := range phases {
		if progress.Phase == "importing" && progress.Total != 2 {
			t.Fatalf("importing.total=%d，期望 2", progress.Total)
		}
	}
	// 上传阶段的 fraction 必须走到 1（W 里 progressReader 报的是已发字节/总字节）。
	uploaded := 0.0
	for _, progress := range phases {
		if progress.Phase == "uploading" && progress.Fraction > uploaded {
			uploaded = progress.Fraction
		}
	}
	if uploaded != 1 {
		t.Fatalf("uploading 的最大 fraction=%v，期望 1", uploaded)
	}

	// ---- 导入后列会话：GUI 走的是同一条 session.list ----
	//
	// 官方客户端无论成败都 `await ctx.sessions.refresh()`（ImportSessions.tsx 的 finally），
	// 本仓对应 `m.refreshList()` → 同一个 session.list。
	sessions, err := client.List(ctx)
	if err != nil {
		t.Fatalf("session.list 失败：%v", err)
	}
	byID := make(map[string]apiclient.Session, len(sessions))
	for _, item := range sessions {
		byID[item.ID] = item
	}
	// 索引数组顺序是 [旧, 新]；导入结果按 updated_at 降序（OFF:src/importer.ts:434-439），
	// 所以 sessionIds[0] 必须是"跨天多步搜索"那条。
	wantTitles := []string{"跨天多步搜索：Go 的发布节奏", "安静的一天：读一份笔记"}
	for index, id := range result.SessionIDs {
		item, found := byID[id]
		if !found {
			t.Fatalf("session.list 里没有刚导入的会话 %s（列表里是 %d 条）", id, len(sessions))
		}
		if item.Title != wantTitles[index] {
			t.Fatalf("sessionIds[%d] 的标题=%q，期望 %q", index, item.Title, wantTitles[index])
		}
		if !strings.EqualFold(filepath.ToSlash(item.Cwd), filepath.ToSlash(workspacePath)) {
			t.Fatalf("会话 %s 的 cwd=%q，期望工作区路径 %q", id, item.Cwd, workspacePath)
		}
	}
}
