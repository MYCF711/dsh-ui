package dsplan

// plan_address_subagent_test.go —— 官方计划地址的**两条分支**（#28 ④）。
//
// 官方契约（唯一权威，OFF 根见 plan.go 文件头）：
//
//	OFF:plan.ts:19-22   `PlanAddress{session: SessionAddress, callId}`
//	OFF:plan.ts:56-62   `planAddress`
//	  kind='session'  → dsh-resource://plan/<sessionId>/<callId>                     （2 段）
//	  kind='subagent' → dsh-resource://plan/subagent/<父>/<子>/<mode>/<callId>       （5 段）
//	OFF:plan.ts:69-87   `parsePlanAddress`：段数只认 2 或 5；5 段时 parts[0]=='subagent'
//	                    且 parts[3] ∈ {'one-shot','continuable','unknown'}；任何一段解码后为空即无效
//	OFF:index.ts:72-74  五段来自 `ctx.sessions.subagentAddress(sessionId)`（父会话 id、子会话 id、mode）

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// previewEnv 造一个"正在看某一枚带地址的右栏 tab"的最小 env：
// 官方 `tab.navigation.address` 走 `Env.RightTabAddress`（见 pluginapi 的注释）。
func previewEnv(session, address string) *pluginapi.Env {
	return &pluginapi.Env{
		Data:            pluginapi.Snapshot{Cur: session},
		RightTabAddress: address,
	}
}

// TestPlanAddressSubagentFiveSegments 钉住官方那条「subagent 五段地址」的形状与往返。
//
// 五段是：`subagent` · 父会话 id · 子会话 id · mode · callId。
func TestPlanAddressSubagentFiveSegments(t *testing.T) {
	cases := []struct {
		parent, child, mode, callID string
	}{
		{"parent-1", "child-1", subagentModeContinuable, "call-1"},
		{"父 会话", "子/会话", subagentModeOneShot, "call:2"},
		{"$&+,;=:@", "p q", subagentModeUnknown, "!'()*"},
	}
	for _, c := range cases {
		session := planSession{Subagent: true, SessionID: c.child, ParentSessionID: c.parent, Mode: c.mode}
		address := planAddressFor(session, c.callID)
		if address == "" {
			t.Fatalf("造不出地址：%+v", c)
		}
		// 段数就是官方那个 5：`subagent` · 父 · 子 · mode · callId
		// （`encodeURIComponent` 把段内的 `/` 转义成 %2F，所以直接按 `/` 切是安全的）。
		if got := len(strings.Split(strings.TrimPrefix(address, "dsh-resource://plan/"), "/")); got != 5 {
			t.Fatalf("地址 %q 的段数 = %d，want 5", address, got)
		}
		if !strings.HasPrefix(address, "dsh-resource://plan/subagent/") {
			t.Fatalf("五段地址前缀不对：%q", address)
		}
		got, callID, ok := parsePlanAddressFull(address)
		if !ok {
			t.Fatalf("五段地址解析失败：%q", address)
		}
		if !got.Subagent || got.ParentSessionID != c.parent || got.SessionID != c.child ||
			got.Mode != c.mode || callID != c.callID {
			t.Fatalf("往返失败：%q → %+v callId=%q", address, got, callID)
		}
		// 内容所在的会话必须是**子**会话：官方 `planResourceProvider` 拿
		// `target.session` 去读 history（OFF:plan-resource.ts:45）。
		if sessionID, call, ok := parsePlanAddress(address); !ok || sessionID != c.child || call != c.callID {
			t.Fatalf("两段视图取错了会话：%q → (%q,%q,%v)", address, sessionID, call, ok)
		}
	}
	// 缺父会话 id 或 mode 不在闭集里 = 造不出合法地址（官方 `SessionAddress` 三个字段都在）。
	if got := planAddressFor(planSession{Subagent: true, SessionID: "c", Mode: subagentModeUnknown}, "x"); got != "" {
		t.Fatalf("缺父会话 id 时不该造地址，得到 %q", got)
	}
	if got := planAddressFor(planSession{Subagent: true, SessionID: "c", ParentSessionID: "p", Mode: "external"}, "x"); got != "" {
		t.Fatalf("mode 不在官方三值闭集里时不该造地址，得到 %q", got)
	}
}

// TestParsePlanAddressFiveSegmentRejections 钉住官方五段的校验条件（OFF:plan.ts:73-81）。
func TestParsePlanAddressFiveSegmentRejections(t *testing.T) {
	bad := []string{
		"dsh-resource://plan/subagent/p/c",                    // 4 段
		"dsh-resource://plan/subagent/p/c/one-shot/call/x",    // 6 段
		"dsh-resource://plan/notsubagent/p/c/one-shot/call",   // parts[0] 不是 subagent
		"dsh-resource://plan/subagent/p/c/external/call",      // mode 不在闭集
		"dsh-resource://plan/subagent//c/one-shot/call",       // 某段为空
		"dsh-resource://plan/subagent/p/c/one-shot/call?x=1",  // 带查询串
		"dsh-resource://plan/subagent/p/c/one-shot/call#frag", // 带 fragment
		"dsh-resource://plan/subagent/p/c/one-shot/%zz",       // 非法百分号编码
		"dsh-resource://plan-review/s1/req",                   // 不是 plan scheme
		"https://plan/subagent/p/c/one-shot/call",             // 不是 dsh-resource
	}
	for _, address := range bad {
		if _, _, ok := parsePlanAddressFull(address); ok {
			t.Errorf("%q 不该解析成功", address)
		}
	}
	// 官方三值闭集里的另外两个要能过。
	for _, mode := range []string{subagentModeOneShot, subagentModeContinuable, subagentModeUnknown} {
		address := planAddressFor(planSession{Subagent: true, SessionID: "c", ParentSessionID: "p", Mode: mode}, "call")
		if _, _, ok := parsePlanAddressFull(address); !ok {
			t.Errorf("mode=%q 的合法地址被拒：%q", mode, address)
		}
	}
}

// TestPreviewResolvesSubagentAddressToChildSession 钉住「五段地址能真的定位到内容」：
// 当前会话就是那段地址里的**子**会话时，应在它的帧里按 callId 找到计划。
func TestPreviewResolvesSubagentAddressToChildSession(t *testing.T) {
	plugin := &Plugin{planTitles: map[string]string{}}
	plans := []Document{
		{CallID: "call-1", Title: "子代理的方案", Markdown: "# 子代理的方案\n"},
		{CallID: "call-2", Title: "另一份", Markdown: "# 另一份\n"},
	}
	address := planAddressFor(
		planSession{Subagent: true, SessionID: "child-1", ParentSessionID: "parent-1", Mode: subagentModeContinuable},
		"call-2",
	)
	env := previewEnv("child-1", address)
	plan, status := plugin.resolve(env, plans)
	if status != previewReady || plan.CallID != "call-2" {
		t.Fatalf("五段地址解析到 (%q,%v)，want (call-2, ready)", plan.CallID, status)
	}
	// 地址属于别的子会话：按"没选过"退到本会话最新一份，而不是报 notFound。
	other := planAddressFor(
		planSession{Subagent: true, SessionID: "child-2", ParentSessionID: "parent-1", Mode: subagentModeContinuable},
		"call-2",
	)
	plan, status = plugin.resolve(previewEnv("child-1", other), plans)
	if status != previewReady || plan.CallID != "call-2" {
		t.Fatalf("跨会话地址应退到本会话最新计划，得到 (%q,%v)", plan.CallID, status)
	}
}
