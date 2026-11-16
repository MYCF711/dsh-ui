package dssettings

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

func TestSessionLogOfficialRegistrationContract(t *testing.T) {
	item, ok := sectionGeneralItem(fullDesktopFacts(), "session-log-deepseek")
	if !ok {
		t.Fatal("namespace 被服务时 general Switch 必须注册")
	}
	if item.Desc != "帮助改进 DeepSeek 模型与产品" {
		t.Fatalf("description = %q", item.Desc)
	}
	if _, ok := sectionGeneralItem(fullDesktopFacts(), "session-log-upload-toast"); ok {
		t.Fatal("session-log-upload-toast 是 shell.overlay id，不是 general item id")
	}
	found := false
	for _, painter := range (&Plugin{}).Panels() {
		if painter.ID == "session-log-upload-toast" {
			found = true
			if painter.Slot != pluginapi.SlotShellOverlay {
				t.Fatalf("Toast slot = %q", painter.Slot)
			}
		}
	}
	if !found {
		t.Fatal("缺少 shell.overlay/session-log-upload-toast painter")
	}
}

func TestSessionLogToastSuccessFailureAndDismiss(t *testing.T) {
	p := &Plugin{}
	row := PanelRow{NS: apiclient.SettingsNSSessionLog, Path: []string{"enabled"}}
	p.setWriteState(WriteState{Kind: WriteSaving, NS: row.NS, Path: row.Path})
	if text, _ := p.sessionLogToastSnapshot(); text != "" {
		t.Fatalf("saving 不应画旧 Toast：%q", text)
	}
	p.setWriteState(WriteState{Kind: WriteSaved, NS: row.NS, Path: row.Path})
	text, success := p.sessionLogToastSnapshot()
	if text != zhSessionLogSaved || !success {
		t.Fatalf("成功 Toast = %q success=%v", text, success)
	}
	// 不让测试等真实时间窗；直接执行同一代 dismiss 回调。
	p.expireSessionLogToast(p.ensureOfficial().sessionLogGeneration)
	if text, _ := p.sessionLogToastSnapshot(); text != "" {
		t.Fatalf("dismiss 后仍有 Toast：%q", text)
	}
	p.setWriteState(WriteState{Kind: WriteFailed, NS: row.NS, Path: row.Path})
	text, success = p.sessionLogToastSnapshot()
	if text != zhSessionLogFailed || success {
		t.Fatalf("失败 Toast = %q success=%v", text, success)
	}
	// 后一次写拥有新 generation；旧回调不能把它撤掉。
	o := p.ensureOfficial()
	o.mu.Lock()
	oldGeneration := o.sessionLogGeneration - 1
	o.mu.Unlock()
	p.expireSessionLogToast(oldGeneration)
	if text, _ := p.sessionLogToastSnapshot(); text != zhSessionLogFailed {
		t.Fatalf("旧定时器清掉了新 Toast：%q", text)
	}
	o.mu.Lock()
	if o.sessionLogToastTimer != nil {
		o.sessionLogToastTimer.Stop()
	}
	o.mu.Unlock()
}

func TestSessionLogBusyBlocksRepeatedActivation(t *testing.T) {
	p := &Plugin{}
	row := PanelRow{ID: "session-log-deepseek", NS: apiclient.SettingsNSSessionLog, Path: []string{"enabled"}}
	p.setWriteState(WriteState{Kind: WriteSaving, NS: row.NS, Path: row.Path})
	// writer 故意不接：若 busy 防重入失效，applyRowWrite 会把状态改成失败。
	p.activateRow(row)
	if got := p.WriteState().Kind; got != WriteSaving {
		t.Fatalf("busy 中重复激活改变了写状态：%q", got)
	}
	// 其它 namespace 的共享 WriteState 变化不能解除本行自己的 busy。
	p.setWriteState(WriteState{Kind: WriteSaved, NS: apiclient.SettingsNSPermission})
	p.activateRow(row)
	if !p.sessionLogBusySnapshot() {
		t.Fatal("其它 namespace 的写入错误地解除了 Session Log busy")
	}
}

func TestSessionLogToastDoesNotCaptureOtherNamespace(t *testing.T) {
	p := &Plugin{}
	p.setWriteState(WriteState{Kind: WriteSaved, NS: apiclient.SettingsNSPermission})
	if text, _ := p.sessionLogToastSnapshot(); text != "" {
		t.Fatalf("其它命名空间触发了 Session Log Toast：%q", text)
	}
}
