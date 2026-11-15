// text.go —— 本插件要用的几个小工具（文案格式化与数字）。
//
// 为什么不引 fmt：插件是**每帧**画的，`fmt.Sprintf` 会为每条 diff 行分配一次；
// 这里全是单次线性拼接，够用且没有反射开销。行为与官方 `t(key, vars)` 的
// `{name}` 替换等价（本文件只支持本插件用到的 `%s` / `%d` 顺序替换）。
package dsforge

import "strconv"

// sprintf 按顺序把 `%s` 换成 args（数量不足时保留原文，绝不 panic）。
func sprintf(format string, args ...string) string {
	out := make([]byte, 0, len(format)+8*len(args))
	next := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) && format[i+1] == 's' {
			if next < len(args) {
				out = append(out, args[next]...)
				next++
				i++
				continue
			}
		}
		out = append(out, format[i])
	}
	return string(out)
}

// sprintf2 是按顺序掏两个整数的版本（`{count} 个文件` 这类）。
func sprintf2(format string, a, b int) string {
	return sprintf(format, itoa(a), itoa(b))
}

// itoa 是 strconv.Itoa 的别名（本文件里用得最多，省一处 import 心智负担）。
func itoa(v int) string { return strconv.Itoa(v) }
