// 一次性小工具：把一份 v4 会话日志解出来，只打印事件类型 + tool 事件的原始 data。
//
//	go run ./tmp/dumplog <session.v4.jsonl.zstd> [类型前缀]
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: dumplog <file> [prefix]")
		os.Exit(2)
	}
	prefix := "tool/"
	if len(os.Args) > 2 {
		prefix = os.Args[2]
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()

	dec, err := zstd.NewReader(f)
	if err != nil {
		panic(err)
	}
	defer dec.Close()

	raw, err := io.ReadAll(dec)
	if err != nil {
		panic(err)
	}

	counts := map[string]int{}
	var shown int
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		typ := extractType(line)
		counts[typ]++
		if strings.HasPrefix(typ, prefix) && shown < 4 {
			shown++
			s := string(line)
			if len(s) > 700 {
				s = s[:700] + "…"
			}
			fmt.Printf("---- %s ----\n%s\n", typ, s)
		}
	}
	fmt.Println("== 事件类型统计 ==")
	for k, v := range counts {
		fmt.Printf("%-24s %d\n", k, v)
	}
}

// extractType 抠出 "type":"xxx" 的值。
func extractType(line []byte) string {
	const key = `"type":"`
	i := bytes.Index(line, []byte(key))
	if i < 0 {
		return "?"
	}
	rest := line[i+len(key):]
	j := bytes.IndexByte(rest, '"')
	if j < 0 {
		return "?"
	}
	return string(rest[:j])
}
