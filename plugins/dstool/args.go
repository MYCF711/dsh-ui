package dstool

import (
	"encoding/json"
	"strconv"
	"strings"
)

// parseArgs 把宿主给的参数原文解成 key → 单行值。
//
// 不是 JSON 对象就返回 nil ——**这不是异常**：`bash` 的 Args 经常就是一行命令
// （`go test ./...`），`read` 在演示数据里也可能只是一条路径。调用方拿到 nil
// 时要**原样显示 Args**，不许显示空串（那就等于把内容吞了）。
//
// 只收标量（字符串/数字/布尔）：数组和对象在这一层看不懂，塞成 JSON 片段
// 反而比原文更难看。它们由调用方退回原文。
func parseArgs(raw string) map[string]string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "{") {
		return nil
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(s), &values); err != nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		switch v := value.(type) {
		case string:
			out[key] = v
		case float64:
			out[key] = formatNumber(v)
		case bool:
			out[key] = strconv.FormatBool(v)
		}
	}
	return out
}

// formatNumber 让 40 显示成 "40" 而不是 "40.000000"。
func formatNumber(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// firstArg 按 keys 的顺序取第一个非空值。
//
// 为什么要多个键：宿主的参数名不统一（filePath / path / file 都见过），
// 界面不认某一种写法，只按可能性从高到低试。
func firstArg(args map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := args[k]; v != "" {
			return v
		}
	}
	return ""
}

// presentArg 与 firstArg 不同：空字符串也是调用方显式提供的真值（例如新建文件
// 的 old_string=""），所以同时返回值与“字段是否存在”。
func presentArg(args map[string]string, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := args[key]; ok {
			return value, true
		}
	}
	return "", false
}

// command 从参数里取命令。
//
// 参数是 JSON 时**只认里面的键**：解出来了却没有 command，返回空串——
// 直接把整段 JSON 当命令显示比什么都不显示更误导。
// 参数不是 JSON 时它本身就是命令，原样返回。
func command(raw string) string {
	args := parseArgs(raw)
	if args == nil {
		return strings.TrimSpace(raw)
	}
	return firstArg(args, "command", "cmd")
}

// splitExit 从摘要里拆出退出码。
//
// 两种写法都是真实见过的：mockd/演示数据给 `exit 0 · 3 files changed`，
// 真宿主的工具输出给 `[exit 0] …`（见 docs/progress.md 2026-11-03 21:21 那条）。
// 拆不出来就返回 ("", 原摘要) —— 不猜"没写就是成功"。
func splitExit(summary string) (code, rest string) {
	t := strings.TrimSpace(summary)
	if t == "" {
		return "", ""
	}
	// [exit 0] … ：
	if strings.HasPrefix(t, "[exit ") {
		if end := strings.IndexByte(t, ']'); end > 0 {
			if code = t[1:end]; isExitCode(code) {
				return code, trimRest(t[end+1:])
			}
		}
	}
	// exit 0 · … ：
	if strings.HasPrefix(t, "exit ") {
		i := len("exit ")
		if i < len(t) && t[i] == '-' {
			i++
		}
		start := i
		for i < len(t) && t[i] >= '0' && t[i] <= '9' {
			i++
		}
		if i > start {
			return t[:i], trimRest(t[i:])
		}
	}
	return "", summary
}

// isExitCode 只认 "exit <数字>"（可带负号）。
func isExitCode(code string) bool {
	rest, ok := strings.CutPrefix(code, "exit ")
	if !ok || rest == "" {
		return false
	}
	if rest[0] == '-' {
		rest = rest[1:]
	}
	if rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// trimRest 去掉分隔用的 "·" 和空白。
func trimRest(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "·")
	return strings.TrimSpace(s)
}

// badExit 给退出码挑颜色：非 0 才是错误。
//
// 按**数值**比，不按字面比：`exit 00` 和 `exit 0` 是同一个退出码，
// 字面比较会把 `00` 判成失败（自测里就是这么红的）。
//
// **只在退出码是从摘要里拆出来的时候用**：拆不出来就不画徽标，
// 不拿"没看见 0"当"成功了"。
func badExit(code string) bool {
	rest, ok := strings.CutPrefix(code, "exit ")
	if !ok {
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest))
	if err != nil {
		return false
	}
	return n != 0
}
