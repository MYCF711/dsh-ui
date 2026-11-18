package apiclient

import (
	"encoding/json"
	"testing"
)

// TestSubagentIdentityCarriesOfficialAddressSegments 钉住 dsh-go#68 之后的新形状。
//
// 宿主 `subagent` 投影现在是「官方三键 ∪ 官方 SubagentAddress 两段」：
//   - 三键 mode/label/seq 来自 typert.host.js:493-501
//   - parentSessionId/childSessionId 来自 SubagentAddress（typert.host.js:320-323），
//     值取自 durable `subagent/spawn` 同名字段
//
// 之前本仓只能靠客户端构造五段地址，因为投影里没有父会话 id。这条测试证明
// **真实投影就能给出五段所需的三段**（parent/child/mode），callId 另由 tool/call 帧提供。
func TestSubagentIdentityCarriesOfficialAddressSegments(t *testing.T) {
	raw := json.RawMessage(`{
		"mode": "continuable",
		"label": "审计",
		"seq": 7,
		"parentSessionId": "parent",
		"childSessionId": "kid"
	}`)
	p := &SessionProjections{Values: map[string]json.RawMessage{"subagent": raw}}
	id, ok := p.SubagentIdentity()
	if !ok {
		t.Fatal("subagent 键应存在")
	}
	if id.ParentSessionID != "parent" || id.ChildSessionID != "kid" {
		t.Fatalf("父/子会话 id 没解出来：%+v", id)
	}
	if id.Mode != "continuable" || id.Label != "审计" || id.Seq != 7 {
		t.Fatalf("官方三键被改动了：%+v", id)
	}
}

// TestSubagentIdentityToleratesLegacyThreeFieldPayload 钉住老负载仍能解。
// 老宿主（dsh-go#68 之前）不发 parentSessionId/childSessionId，消费方不能因此炸。
func TestSubagentIdentityToleratesLegacyThreeFieldPayload(t *testing.T) {
	raw := json.RawMessage(`{"mode":"one-shot","label":"x","seq":1}`)
	p := &SessionProjections{Values: map[string]json.RawMessage{"subagent": raw}}
	id, ok := p.SubagentIdentity()
	if !ok {
		t.Fatal("subagent 键应存在")
	}
	if id.ParentSessionID != "" || id.ChildSessionID != "" {
		t.Fatalf("老负载不该凭空多出父/子 id：%+v", id)
	}
}
