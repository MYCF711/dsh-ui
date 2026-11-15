// cordis_test.go —— `dynamicCordisRunner.*` 四个客户端的**形状**测试（GUI 账本 #12）。
//
// 判据三条，都对着官方 zod 逐字段（OFF:dsh-cordis-host-runner/lib/typert.host.js）：
//
//	① 入参命名对：面板路径用 sessionId，runHostHalf 用 agentId（宿主两个都收，
//	   但 wire 上与官方第一个位置参数 agent 对应的是这两个名字，不许混着发成第三种）。
//	② 结果解码对：`waitingFor` 恒空数组要解成空切片（不是 nil 崩）、
//	   `ok:false` 的判别联合要保留 reason / message 原文（**不是** error）。
//	③ 缺席字段保持零值：approvalRequestId / requiresApproval 本宿主不出现，
//	   不许被解成"有审批"。
package apiclient

import (
	"context"
	"testing"
)

// TestCordisInventoryDecodesOfficialRow 钉住 inventory 的官方行形状与"如实缺席"的字段。
func TestCordisInventoryDecodesOfficialRow(t *testing.T) {
	cli := forgeServer(t, "/api/dynamicCordisRunner.inventory",
		`{"result":[{"pluginId":"hello","agentId":"","packages":[{"packageId":"pkg-1","name":"hello","purpose":"","hasHostHalf":true,"hasClientHalf":false}],"currentPackageId":"pkg-1","activeRun":{"pluginRunId":"run-1","packageId":"pkg-1"},"latestRun":{"pluginRunId":"run-1","packageId":"pkg-1","mode":"run","status":"running","host":{"status":"running","waitingFor":[]},"client":{"status":"absent","waitingFor":[]}}}]}`,
		nil)
	rows, err := cli.CordisInventory(context.Background())
	if err != nil {
		t.Fatalf("inventory 应当成功：%v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d，want 1", len(rows))
	}
	row := rows[0]
	if row.PluginID != "hello" || row.AgentID != "" {
		t.Fatalf("pluginId/agentId = %q/%q（agentId 空串是本宿主的如实值）", row.PluginID, row.AgentID)
	}
	if len(row.Packages) != 1 || !row.Packages[0].HasHostHalf || row.Packages[0].HasClientHalf {
		t.Fatalf("packages = %+v", row.Packages)
	}
	if row.Packages[0].Purpose != "" {
		t.Fatalf("purpose 应当保持空串（盘上没有来源），实得 %q", row.Packages[0].Purpose)
	}
	if row.NextPackageID != "" {
		t.Fatalf("nextPackageId 本宿主不出现，实得 %q", row.NextPackageID)
	}
	if row.ActiveRun == nil || row.ActiveRun.PluginRunID != "run-1" {
		t.Fatalf("activeRun = %+v", row.ActiveRun)
	}
	if row.LatestRun == nil || row.LatestRun.Status != "running" {
		t.Fatalf("latestRun = %+v", row.LatestRun)
	}
	if row.LatestRun.ApprovalRequestID != "" || row.LatestRun.RequiresApproval {
		t.Fatalf("审批字段本宿主不出现，实得 %+v", row.LatestRun)
	}
	if len(row.LatestRun.Host.WaitingFor) != 0 || len(row.LatestRun.Client.WaitingFor) != 0 {
		t.Fatalf("waitingFor 恒 []，实得 host=%v client=%v",
			row.LatestRun.Host.WaitingFor, row.LatestRun.Client.WaitingFor)
	}
}

// TestCordisInventoryEmptyIsNotError 钉住"空清单是合法结果"（≠ 读失败）。
func TestCordisInventoryEmptyIsNotError(t *testing.T) {
	cli := forgeServer(t, "/api/dynamicCordisRunner.inventory", `{"result":[]}`, nil)
	rows, err := cli.CordisInventory(context.Background())
	if err != nil {
		t.Fatalf("空清单不应当是错误：%v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %v", rows)
	}
}

// TestCordisStopFromPanelSendsSessionID 钉住面板路径的命名参数与回执解码。
func TestCordisStopFromPanelSendsSessionID(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/dynamicCordisRunner.stopFromPanel",
		`{"result":{"ok":false,"reason":"not-running","message":"插件 \"hello\" 没有在跑"}}`,
		func(b map[string]any) { body = b })
	answer, err := cli.CordisStopFromPanel(context.Background(), "s-1", "hello")
	if err != nil {
		t.Fatalf("stop 应当成功（业务失败在 result 里）：%v", err)
	}
	if body["sessionId"] != "s-1" || body["pluginId"] != "hello" {
		t.Fatalf("入参 = %v", body)
	}
	if _, ok := body["agentId"]; ok {
		t.Fatalf("面板路径不该同时发 agentId：%v", body)
	}
	if answer.OK || answer.Reason != "not-running" || answer.Message == "" {
		t.Fatalf("回执 = %+v（reason/message 必须是宿主原文，不能被折成 error）", answer)
	}
}

// TestCordisUndefineFromPanelSendsSessionID 钉住 undefine 的 wasRunning 解码。
func TestCordisUndefineFromPanelSendsSessionID(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/dynamicCordisRunner.undefineFromPanel",
		`{"result":{"ok":true,"wasRunning":true}}`,
		func(b map[string]any) { body = b })
	receipt, err := cli.CordisUndefineFromPanel(context.Background(), "s-2", "hello")
	if err != nil {
		t.Fatalf("undefine 应当成功：%v", err)
	}
	if body["sessionId"] != "s-2" {
		t.Fatalf("sessionId = %v", body["sessionId"])
	}
	if !receipt.OK || !receipt.WasRunning {
		t.Fatalf("回执 = %+v（wasRunning 必须解出来）", receipt)
	}
}

// TestCordisRunHostHalfSendsAgentIDAndFourParams 钉住：
//   - 第一个参数叫 agentId（不是 sessionId）；
//   - **不发** requestId / approveFutureVersions（本宿主没有审批队列，
//     官方面板路径同样传 null / false）；
//   - `ok:false` 时 message/stack 原文保留。
func TestCordisRunHostHalfSendsAgentIDAndFourParams(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/dynamicCordisRunner.runHostHalf",
		`{"result":{"ok":true,"pluginId":"hello","packageId":"pkg-1","pluginRunId":"run-7","waitingFor":[],"startedHere":true}}`,
		func(b map[string]any) { body = b })
	got, err := cli.CordisRunHostHalf(context.Background(), "s-3", "hello", "pkg-1", CordisRunModeRun)
	if err != nil {
		t.Fatalf("runHostHalf 应当成功：%v", err)
	}
	if body["agentId"] != "s-3" || body["pluginId"] != "hello" ||
		body["packageId"] != "pkg-1" || body["mode"] != CordisRunModeRun {
		t.Fatalf("入参 = %v", body)
	}
	for _, forbidden := range []string{"sessionId", "requestId", "approveFutureVersions"} {
		if _, ok := body[forbidden]; ok {
			t.Fatalf("不该发 %s：%v（本宿主没有 cordis 审批队列）", forbidden, body)
		}
	}
	if !got.OK || !got.StartedHere || got.PluginRunID != "run-7" {
		t.Fatalf("回执 = %+v", got)
	}
}

// TestCordisRunHostHalfFailureKeepsHostText 钉住 ok:false 分支：
// 它是**业务失败**（200 里的值），message 是宿主原文，不能被当成传输错误丢掉。
func TestCordisRunHostHalfFailureKeepsHostText(t *testing.T) {
	cli := forgeServer(t, "/api/dynamicCordisRunner.runHostHalf",
		`{"result":{"ok":false,"message":"加载 host.cjs 失败：boom","stack":"at load"}}`, nil)
	got, err := cli.CordisRunHostHalf(context.Background(), "", "hello", "pkg-1", CordisRunModeUpdate)
	if err != nil {
		t.Fatalf("业务失败不该是传输错误：%v", err)
	}
	if got.OK {
		t.Fatal("ok:false 被解成了成功")
	}
	if got.Message != "加载 host.cjs 失败：boom" || got.Stack != "at load" {
		t.Fatalf("宿主原文丢了：%+v", got)
	}
}
