//go:build viva_race_repro

// 上游缺陷最小复现：viva v0.2.3 的 `text.(*Editor).String()` 名为"取值"，
// 实现里却**会写** editor.text / editor.textValid（text/editor.go:87-94），
// 而 `Editor` 内部没有任何 sync.Mutex / sync/atomic。
//
// 后果：只要"一个 goroutine 想看一眼草稿（Field.Text()）、另一个 goroutine
// 还在编辑同一个 Field（SetText/Insert）"，就是数据竞争。这不是调用方误用
// API——getter 自己就是写者，调用方无论怎么读都错。
//
// 跑法（Windows 必须 MinGW + CGO，否则报 "gcc not found"）：
//
//	export PATH="/c/Program Files/Go/bin:/c/toolchains/mingw64/bin:$PATH"
//	CGO_ENABLED=1 go test -race -tags viva_race_repro ./spike/vivaeditor/ -run 'TestEditor|TestField' -v
//
// 期望：**WARNING: DATA RACE 两条**（这是预期的红，不是本仓回归）。
// 上游修好（String 只读、或 Editor 自带同步）之后本文件会变绿，届时
// spike/vivaeditor/ 整个目录可删——见 docs/viva-upstream-editor-string-race.md。
package main

import (
	"sync"
	"testing"

	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/text"
)

// TestEditorStringIsNotARead 钉住根因：String() 自己写缓存。
func TestEditorStringIsNotARead(t *testing.T) {
	editor := text.NewEditor("initial")
	var wg sync.WaitGroup
	wg.Add(2)
	// 读者：另一个 goroutine 只是想拿到当前文本。
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = editor.String()
		}
	}()
	// 写者：UI 线程继续编辑（每次 Insert 都会清掉 String 的缓存）。
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			editor.SetSelection(text.Range{Start: 0, End: editor.RuneCount()})
			editor.Insert("x")
		}
	}()
	wg.Wait()
}

// TestFieldTextIsNotARead 是产品里真正踩到的形状：`flex.Field` 是 Viva
// 推荐给业务持有的输入框状态，`Field.Text()` 转调 `Editor.String()`。
func TestFieldTextIsNotARead(t *testing.T) {
	var field flex.Field
	field.SetText("不要丢的草稿")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = field.Text()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			field.SetText("new draft")
		}
	}()
	wg.Wait()
}
