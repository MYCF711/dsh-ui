// Command vivaeditor 是 Viva 上游缺陷 `text.(*Editor).String()` 不是只读的**最小
// 复现宿主**——真正的复现在同目录 `editor_string_race_test.go` 里（带 `viva_race_repro`
// 构建标签，默认不参与 `go test ./...`，否则本仓 `-race` 门禁会红）。
//
// 为什么单独放一个 main：复现只该以测试形态存在，产品构建不该带它；但 Go 的
// `go build ./...` 要求目录里至少有一个可编译文件，标签关掉时这个 main 就是那个文件。
//
// 上游说明与我们的兜底见 `docs/viva-upstream-editor-string-race.md`。
package main

import (
	"fmt"

	"github.com/vivaui/viva/text"
)

func main() {
	fmt.Println("viva text.(*Editor).String() 并发复现：")
	fmt.Println("\tCGO_ENABLED=1 go test -race -tags viva_race_repro ./spike/vivaeditor/ -v")
	fmt.Println("期望输出 WARNING: DATA RACE（上游修好后本复现会变绿，届时整目录可删）。")
	fmt.Printf("当前 Editor 文本：%q\n", text.NewEditor("hello").String())
}
