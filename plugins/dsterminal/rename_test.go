package dsterminal

import (
	"errors"
	"testing"
)

// TestRightTabRenameOwnsOnlyTerminal：`RightTabRenameProvider` 的 kind 归属约定
// 与 `CloseRightTab` 一致——不是 terminal 就 owned=false，让壳继续问下一个实现者。
func TestRightTabRenameOwnsOnlyTerminal(t *testing.T) {
	p := &Plugin{}
	owned, err := p.RightTabRename("s1", "tab1", "browser", "x")
	if owned || err != nil {
		t.Fatalf("browser kind 应不认领：owned=%v err=%v", owned, err)
	}
	// 认领但没接线：必须回错，不能假装成功。
	owned, err = p.RightTabRename("s1", "tab1", RightTabTerminal, "构建")
	if !owned || err == nil {
		t.Fatalf("terminal 认领但未接线应报错：owned=%v err=%v", owned, err)
	}
}

// TestRightTabRenamePassesTitleThrough：标题原样交给 `TerminalRenameOfficial`
// （壳已经做过 trim / 空值 / 同值判据，插件不重复猜）。
func TestRightTabRenamePassesTitleThrough(t *testing.T) {
	p := &Plugin{}
	var gotID, gotTitle string
	p.act.TerminalRenameOfficial = func(id, title string) error {
		gotID, gotTitle = id, title
		return nil
	}
	owned, err := p.RightTabRename("s1", "tab9", RightTabTerminal, "构建")
	if !owned || err != nil {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
	if gotID != "tab9" || gotTitle != "构建" {
		t.Fatalf("宿主 rename = (%q,%q)，want (tab9,构建)", gotID, gotTitle)
	}
	// 宿主失败原样回错（界面负责显示，插件不改标题）。
	wantErr := errors.New("host down")
	p.act.TerminalRenameOfficial = func(string, string) error { return wantErr }
	if _, err := p.RightTabRename("s1", "tab9", RightTabTerminal, "x"); !errors.Is(err, wantErr) {
		t.Fatalf("宿主错误没有透传：%v", err)
	}
}
