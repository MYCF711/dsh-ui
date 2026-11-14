package dscomposer

import (
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// mention.go 是统一触发管道（trigger.go）里**老壳文件源**的那一份兜底。
//
// # 它现在的位置
//
// `plugins/dscomposer/trigger.go` 是官方 `dsh-client-ui-input-trigger` 的骨架：
// `@` / `/` detect、source 注册表、分组菜单、pick 路由。文件候选优先走宿主
// `fileReferences.list`（官方 `ui-reference` 的 source）。
//
// 但老壳（没有 `Actions.FileReferences`）只有 `Actions.FSList`——
// 本文件保留那套"按目录列候选 + 前缀补全"的实现，由 trigger.go 的
// `legacyFileRows` 调用。它**不是**第二套菜单：候选最终仍由 trigger.go 统一
// 排序/绘制/pick，这里的函数只负责"从路径前缀算出候选"这一小步。
//
// 官方 `@[label](dsh-session:id)` 会话引用与 `/name` 技能候选在新管道里；
// 用户消息里这些文本再被 `plugins/dsconversation/usertext.go` 识别回 chip。

// atRef 是草稿里"正在敲的那一个 @ 引用"。
type atRef struct {
	// Start 是 '@' 在文本里的下标（字节）。
	Start int
	// Raw 是 '@' 之后到文本末尾的那一段（不含空白）。
	Raw string
	// Dir 是 Raw 里最后一个 '/' 之前的部分，**含尾部 '/'**；没有 '/' 时是 ""。
	Dir string
	// Prefix 是最后一个 '/' 之后的部分，也就是正在敲的名字前缀。
	Prefix string
}

// parseAt 从草稿里认出正在敲的 @ 引用。
//
// 只认**末尾**那一个 token，而且 '@' 必须在开头或前面是空白：
//
//   - `看下 @tmp/a`     → 认出来（正在敲路径）
//   - `发到 a@b.com`    → **不认**（那是邮箱，'@' 前面不是空白）
//   - `@tmp/a 然后呢`   → 不认（token 已经被空格结束了，用户敲完了）
//
// 第三条是关键：不认它的话，用户打完路径接着打字时菜单会一直挂着，
// 而 Enter 会被我们吃掉去"补全"——他**永远发不出这条消息**。
func parseAt(text string) (atRef, bool) {
	i := -1
	for k := len(text) - 1; k >= 0; k-- {
		if text[k] != '@' {
			continue
		}
		if k == 0 || isSpaceByte(text[k-1]) {
			i = k
			break
		}
	}
	if i < 0 {
		return atRef{}, false
	}
	raw := text[i+1:]
	if strings.ContainsAny(raw, " \t\r\n") {
		return atRef{}, false
	}
	ref := atRef{Start: i, Raw: raw}
	if k := strings.LastIndex(raw, "/"); k >= 0 {
		ref.Dir = raw[:k+1]
		ref.Prefix = raw[k+1:]
	} else {
		ref.Prefix = raw
	}
	return ref, true
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// completeAt 把候选补进草稿：替换掉那个 token，其余原样保留。
//
// isDir 决定要不要补一个尾斜杠——补上之后菜单自然进到下一层，
// 用户少按一次键，也不会出现"补全了却发现还得自己打斜杠"。
func completeAt(text string, ref atRef, name string, isDir bool) string {
	repl := "@" + ref.Dir + name
	if isDir {
		repl += "/"
	}
	return text[:ref.Start] + repl
}

// filterMentions 按前缀筛候选：目录在前，然后按名字排（大小写不敏感）。
//
// 大小写不敏感是刻意的：Windows 上 `@SRC` 和 `@src` 指同一个目录，
// 而这是个 Windows 优先的产品。
func filterMentions(entries []pluginapi.FSEntry, prefix string) []pluginapi.FSEntry {
	if len(entries) == 0 {
		return nil
	}
	low := strings.ToLower(prefix)
	out := make([]pluginapi.FSEntry, 0, len(entries))
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(e.Name), low) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	// 一个都没匹配时返回 **nil** 而不是空切片：调用方拿它判断"要不要画菜单"，
	// nil 是"没有"，空切片是"有、但一个都没有"——后者会让读的人多停一秒。
	if len(out) == 0 {
		return nil
	}
	return out
}

// dropExactFile 去掉"已经完整敲出来的那个文件"。
//
// 为什么必须去掉：`@plugins/dsopen/open.go` 敲全之后，候选里正好有它自己，
// 于是菜单一直挂着、Enter 一直被我们吃掉去"补全"（补成同一个字符串），
// **这条消息就永远发不出去**。
//
// 只对文件这么做，目录不这么做：`@plugins/dsopen/` 是"进到这一层"，
// 菜单该继续显示里面的东西。
func dropExactFile(cands []pluginapi.FSEntry, prefix string) []pluginapi.FSEntry {
	if len(cands) != 1 {
		return cands
	}
	if !cands[0].IsDir && cands[0].Name == prefix {
		return nil
	}
	return cands
}
