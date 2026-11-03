package main

import "testing"

// 这三条测的是「工具只声明、不渲染」的显示契约本身。
//
// 为什么值得单测：视图契约是界面与宿主唯一握手的地方，一旦 Kind 或字段名漂了，
// 界面上就会静默退化成纯文本——不报错、不崩溃，只是"看起来没生效"，
// 这种 bug 靠截图是发现不了的（截图只能告诉你"现在长得对"）。

func TestToolViewShellIsCard(t *testing.T) {
	got := tvToolView("bash", `{"command":"go build ./..."}`, "ok\nBUILD_OK", false)
	if got == nil {
		t.Fatal("bash 应该声明 card，实际 nil")
	}
	if got.Kind != "card" {
		t.Fatalf("Kind = %q，want card", got.Kind)
	}
	if got.Title != "bash" {
		t.Fatalf("Title = %q，want bash", got.Title)
	}
	if got.Badge != "ok" {
		t.Fatalf("Badge = %q，want ok", got.Badge)
	}
	if len(got.Fields) != 2 {
		t.Fatalf("Fields = %d 项，want 2（命令 + 输出）", len(got.Fields))
	}
	if got.Fields[0].Key != "命令" || got.Fields[0].Value != "go build ./..." {
		t.Fatalf("第一项 = %+v，want 命令=go build ./...", got.Fields[0])
	}
}

func TestToolViewListIsTable(t *testing.T) {
	got := tvToolView("glob", `{"pattern":"**/*.go"}`, "a.go\nb.go\nc.go", false)
	if got == nil || got.Kind != "table" {
		t.Fatalf("glob 应该声明 table，实际 %+v", got)
	}
	if len(got.Rows) != 3 {
		t.Fatalf("Rows = %d，want 3", len(got.Rows))
	}
	// 每一行的长度必须等于列数：界面按列数取单元格，多一列少一列都会画歪。
	for i, row := range got.Rows {
		if len(row) != len(got.Columns) {
			t.Fatalf("第 %d 行有 %d 个单元格，列有 %d 个", i, len(row), len(got.Columns))
		}
	}
}

func TestToolViewReadIsPlain(t *testing.T) {
	got := tvToolView("read", `{"file_path":"x.md"}`, "第一行\n第二行", false)
	if got == nil || got.Kind != "plain" {
		t.Fatalf("read 应该声明 plain，实际 %+v", got)
	}
	if got.Body == "" {
		t.Fatal("plain 的 Body 不能为空")
	}
}

// 不认识的工具给一张**通用卡片**（参数 + 结果首行），而不是回到纯文本。
//
// 这是有意的设计取舍，不是漏判：
//   - 纯文本会同时显示 tool / args / summary 三段，在长会话里比卡片更吵；
//   - 通用卡片仍然是"从真实参数与结果里取的"，没有编造字段。
// 但**空结果必须回到 nil**——只有标题的空卡片是纯噪音（见下一个用例）。
func TestToolViewUnknownGetsGenericCard(t *testing.T) {
	got := tvToolView("some_future_tool", `{"path":"x.go"}`, "一些输出", false)
	if got == nil {
		t.Fatal("未知工具应该给一张通用卡片，实际 nil")
	}
	if got.Kind != "card" {
		t.Fatalf("Kind = %q，want card", got.Kind)
	}
	if len(got.Fields) == 0 {
		t.Fatal("通用卡片必须有字段，不能只有标题")
	}
}

// 参数和结果都空的时候必须不声明：空卡片比纯文本还糟。
func TestToolViewUnknownEmptyIsNil(t *testing.T) {
	if got := tvToolView("some_future_tool", `{}`, "", false); got != nil {
		t.Fatalf("参数与结果都空时不该声明视图，实际 %+v", got)
	}
}

// 结果为空时也必须守住：不能产出只有标题、没有任何内容的空卡片。
func TestToolViewEmptyResult(t *testing.T) {
	got := tvToolView("bash", `{"command":"true"}`, "", false)
	if got != nil && got.Kind == "card" && len(got.Fields) == 0 {
		t.Fatal("空结果不该产出没有字段的空卡片")
	}
}

// 错误必须反映在 Badge 上：界面靠它上色，不靠解析文本。
func TestToolViewErrorBadge(t *testing.T) {
	got := tvToolView("bash", `{"command":"false"}`, "boom", true)
	if got == nil {
		t.Fatal("nil")
	}
	if got.Badge != "error" {
		t.Fatalf("Badge = %q，want error", got.Badge)
	}
}
