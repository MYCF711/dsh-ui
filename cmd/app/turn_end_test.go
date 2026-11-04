package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
)

// 这个文件钉住 `turn/end` 的**真实**载荷形状。
//
// # 为什么
//
// 宿主 M371 之后，一轮的结局统一走 `turn/end`，关键信息在 `reason` 里面：
//
//	{"turn":5,"reason":{"kind":"error","error":{"name":"Error","message":"…"}}}
//	{"turn":5,"reason":{"kind":"aborted","reason":"这一轮被取消…"}}
//	{"turn":5,"reason":{"kind":"completed"}}
//
// 而界面原来只读**顶层**的 `error` 字符串和 `stopped/interrupted` 布尔。
// 于是失败的轮次既不是 stopped 也不是 error，落到兜底那个
// `turn/done || turn/end` 分支，被记成 **done** —— 界面上一次失败的轮次
// 显示成"跑完了"，而且原因一个字都没有。
//
// 这类错误不崩、不报错，只是把**错误状态伪装成正常状态**，比崩溃难查得多。

func recordEnd(t *testing.T, payload string) []pubapi.TurnEvent {
	t.Helper()
	m := &model{}
	m.recordTurnEventsLocked("s1", apiclient.Event{
		Kind: "turn/end", Data: []byte(payload), Time: 100,
	})
	return m.turnEvents["s1"]
}

func onlyKind(t *testing.T, events []pubapi.TurnEvent) pubapi.TurnEvent {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("应当正好记一条结局事件，实际 %d 条：%+v", len(events), events)
	}
	return events[0]
}

// TestTurnEndErrorIsNotDone 是那条回归。
func TestTurnEndErrorIsNotDone(t *testing.T) {
	ev := onlyKind(t, recordEnd(t, `{"turn":5,"reason":{"kind":"error","error":{"name":"Error","message":"模型超时，这一轮没跑完"}}}`))
	if ev.Kind != "error" {
		t.Fatalf("记成了 %q，want error —— 失败的轮次不能显示成跑完了", ev.Kind)
	}
	if ev.Error == "" {
		t.Fatal("失败原因是空的：用户只会看到一个红块，不知道发生了什么")
	}
	if ev.Error != "模型超时，这一轮没跑完" {
		t.Errorf("原因原文没保住：%q（不许归纳、不许翻译）", ev.Error)
	}
}

func TestTurnEndAbortedIsStopped(t *testing.T) {
	ev := onlyKind(t, recordEnd(t, `{"turn":6,"reason":{"kind":"aborted","reason":"这一轮被取消（上下文已结束），结果不完整"}}`))
	if ev.Kind != "stopped" {
		t.Fatalf("aborted 应当记成 stopped，实际 %q", ev.Kind)
	}
	if ev.Error != "这一轮被取消（上下文已结束），结果不完整" {
		t.Errorf("取消原因没保住：%q", ev.Error)
	}
}

func TestTurnEndCompletedAndMaxTokensAreDone(t *testing.T) {
	// host 的注释：completed 与 max-tokens 都是"这一轮按预期结束"。
	for _, kind := range []string{"completed", "max-tokens"} {
		ev := onlyKind(t, recordEnd(t, `{"turn":7,"reason":{"kind":"`+kind+`"}}`))
		if ev.Kind != "done" {
			t.Errorf("%s 应当记成 done，实际 %q", kind, ev.Kind)
		}
		if ev.Error != "" {
			t.Errorf("%s 不该带失败原因，实际 %q", kind, ev.Error)
		}
	}
}

func TestTurnEndBlockedIsError(t *testing.T) {
	// blocked 也是"没跑成"的一种（被闸门挡住）。
	ev := onlyKind(t, recordEnd(t, `{"turn":8,"reason":{"kind":"blocked","reason":"计划模式：先出方案"}}`))
	if ev.Kind != "error" {
		t.Fatalf("blocked 应当记成 error，实际 %q", ev.Kind)
	}
}

// TestLegacyTurnErrorStillWorks：旧形状（顶层 error 字符串）不能因为这次改动坏掉。
func TestLegacyTurnErrorStillWorks(t *testing.T) {
	m := &model{}
	m.recordTurnEventsLocked("s1", apiclient.Event{
		Kind: "turn/error", Data: []byte(`{"turn":9,"error":"老形状的失败原因"}`), Time: 1,
	})
	events := m.turnEvents["s1"]
	found := false
	for _, e := range events {
		if e.Kind == "error" && e.Error == "老形状的失败原因" {
			found = true
		}
	}
	if !found {
		t.Fatalf("旧形状的 turn/error 没被认出来：%+v", events)
	}
}

// TestTurnEndRecordedOnce：三种结局互斥，一帧只记一条。
//
// 原来的写法是"先判 turn/error 记一条，再判 stopped / done 又记一条"，
// 一个 failed turn 可能同时出现两条（error + done）。
func TestTurnEndRecordedOnce(t *testing.T) {
	for _, payload := range []string{
		`{"turn":1,"reason":{"kind":"completed"}}`,
		`{"turn":1,"reason":{"kind":"error","error":{"message":"x"}}}`,
		`{"turn":1,"reason":{"kind":"aborted","reason":"y"}}`,
		`{"turn":1,"stopped":true}`,
	} {
		if n := len(recordEnd(t, payload)); n != 1 {
			t.Errorf("载荷 %s 记了 %d 条结局事件，应当正好 1 条", payload, n)
		}
	}
}
