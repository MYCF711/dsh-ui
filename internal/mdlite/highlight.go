// highlight.go —— 给 components.MarkdownStyle.Highlight 用的通用词法高亮。
//
// 不追求完整 lexer：按**行**扫，识别注释 / 字符串 / 数字 / 关键字 / 函数调用五类。
// 语言的差别只在关键字表；不认识的语言退化成"只认注释和字符串"。
//
// 为什么不用 regexp：这里每行只扫一遍，手写比正则快一个量级，
// 而且正则的 `\b` 在中文标点旁边行为不符合直觉。
package mdlite

import (
	"github.com/vivaui/viva/components"
)

// keywords 是各语言的关键字表。键是小写语言名（`` `` `` 表示未知语言，空表）。
var keywords = map[string]map[string]bool{
	"go": toSet(
		"break", "case", "chan", "const", "continue", "default", "defer", "else",
		"fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
		"map", "package", "range", "return", "select", "struct", "switch", "type", "var",
		"nil", "true", "false", "iota", "string", "int", "int64", "float32", "float64",
		"byte", "rune", "error", "any", "bool", "make", "new", "len", "cap", "append",
		"copy", "delete", "panic", "recover", "print", "println",
	),
	"python": toSet(
		"def", "class", "return", "if", "elif", "else", "for", "while", "break",
		"continue", "import", "from", "as", "try", "except", "finally", "raise",
		"with", "lambda", "yield", "global", "nonlocal", "assert", "del", "pass",
		"and", "or", "not", "in", "is", "None", "True", "False", "self", "async", "await",
	),
	"py": toSet(
		"def", "class", "return", "if", "elif", "else", "for", "while", "import",
		"from", "as", "try", "except", "finally", "with", "lambda", "None", "True", "False", "self",
	),
	"javascript": toSet(
		"const", "let", "var", "function", "return", "if", "else", "for", "while",
		"do", "switch", "case", "default", "break", "continue", "new", "this",
		"class", "extends", "super", "import", "export", "from", "async", "await",
		"try", "catch", "finally", "throw", "typeof", "instanceof", "null",
		"undefined", "true", "false", "delete", "in", "of", "yield",
	),
	"js": toSet(
		"const", "let", "var", "function", "return", "if", "else", "for", "while",
		"class", "new", "this", "async", "await", "try", "catch", "null", "undefined", "true", "false",
	),
	"typescript": toSet(
		"const", "let", "var", "function", "return", "if", "else", "for", "while",
		"class", "interface", "type", "enum", "implements", "extends", "public",
		"private", "protected", "readonly", "static", "abstract", "as", "any",
		"string", "number", "boolean", "void", "never", "unknown", "null",
		"undefined", "true", "false", "async", "await", "import", "export", "from", "new", "this",
	),
	"ts": toSet(
		"const", "let", "function", "return", "if", "else", "interface", "type",
		"string", "number", "boolean", "async", "await", "import", "export", "new", "this",
	),
	"bash": toSet(
		"if", "then", "else", "elif", "fi", "for", "while", "do", "done", "case",
		"esac", "function", "return", "in", "local", "export", "unset", "readonly",
		"echo", "cd", "exit", "set", "shift", "source", "alias", "true", "false",
	),
	"sh": toSet(
		"if", "then", "else", "fi", "for", "while", "do", "done", "function",
		"return", "export", "echo", "cd", "exit", "set", "source",
	),
	"shell": toSet(
		"if", "then", "else", "fi", "for", "while", "do", "done", "function",
		"return", "export", "echo", "cd", "exit", "set", "source",
	),
	"rust": toSet(
		"fn", "let", "mut", "const", "static", "struct", "enum", "impl", "trait",
		"pub", "use", "mod", "crate", "self", "super", "return", "if", "else",
		"match", "for", "while", "loop", "break", "continue", "move", "ref",
		"as", "where", "async", "await", "dyn", "unsafe", "true", "false", "None", "Some", "Ok", "Err",
	),
}

func toSet(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// commentPrefixes 是按语言分的注释前缀。值为 `//` 或 `#`。
func commentPrefix(language string) string {
	switch language {
	case "python", "py", "bash", "sh", "shell", "yaml", "yml", "toml", "makefile", "dockerfile":
		return "#"
	default:
		return "//"
	}
}

// Highlight 是 components.MarkdownStyle.Highlight 的实现。
//
// 它逐字节扫一行，把命中的片段 append 到 dst 并返回；**产出的 run 不重叠、按 Start 升序**。
// 一行里只看得到这一行能判断的东西：跨行字符串/注释只会在有引号或前缀的那一行被认出来。
func Highlight(language, line string, dst []components.MarkdownCodeRun) []components.MarkdownCodeRun {
	if line == "" {
		return dst
	}
	lang := normalizeLanguage(language)
	kw := keywords[lang]
	comment := commentPrefix(lang)

	n := len(line)
	i := 0
	for i < n {
		c := line[i]

		// 1) 注释：到行尾，直接结束整行扫描
		if strings4HasPrefix(line, i, comment) {
			dst = append(dst, components.MarkdownCodeRun{Start: i, End: n, Class: components.CodeComment})
			return dst
		}

		// 2) 字符串：单引号 / 双引号 / 反引号。反斜杠转义只对前两种生效
		if c == '"' || c == '\'' || c == '`' {
			q := c
			j := i + 1
			for j < n {
				if line[j] == '\\' && q != '`' {
					j += 2
					continue
				}
				if line[j] == q {
					j++
					break
				}
				j++
			}
			if j > n {
				j = n
			}
			dst = append(dst, components.MarkdownCodeRun{Start: i, End: j, Class: components.CodeString})
			i = j
			continue
		}

		// 3) 数字：连续的 0-9，可带一个小数点
		if c >= '0' && c <= '9' {
			j := i
			for j < n && line[j] >= '0' && line[j] <= '9' {
				j++
			}
			// 小数点后面还得是数字才算（`1.foo` 不该吃掉点）
			if j+1 < n && line[j] == '.' && line[j+1] >= '0' && line[j+1] <= '9' {
				j++
				for j < n && line[j] >= '0' && line[j] <= '9' {
					j++
				}
			}
			dst = append(dst, components.MarkdownCodeRun{Start: i, End: j, Class: components.CodeNumber})
			i = j
			continue
		}

		// 4) 标识符：字母/下划线开头，后面接字母数字下划线
		if isIdentStart(c) {
			j := i
			for j < n && isIdentPart(line[j]) {
				j++
			}
			word := line[i:j]
			switch {
			case kw[word]:
				dst = append(dst, components.MarkdownCodeRun{Start: i, End: j, Class: components.CodeKeyword})
			case j < n && line[j] == '(':
				// 名字后面紧跟括号 → 当函数调用。排除已经命中的关键字
				dst = append(dst, components.MarkdownCodeRun{Start: i, End: j, Class: components.CodeFunction})
			case isAllUpper(word) && len(word) >= 2:
				// 全大写的短标识符当常量/变量（HTTP_PORT 这种）
				dst = append(dst, components.MarkdownCodeRun{Start: i, End: j, Class: components.CodeVariable})
			}
			i = j
			continue
		}

		i++
	}
	return dst
}

// normalizeLanguage 把 ```go / ```Go / ```golang / ```go,title=... 都归一成小写短名。
func normalizeLanguage(language string) string {
	lang := language
	// 去掉逗号后面的选项（有些渲染器会带 ```go,linenos）
	for k := 0; k < len(lang); k++ {
		if lang[k] == ',' || lang[k] == ' ' {
			lang = lang[:k]
			break
		}
	}
	lang = lowerASCII(lang)
	switch lang {
	case "golang":
		return "go"
	case "python3":
		return "python"
	case "node", "nodejs":
		return "javascript"
	case "zsh", "console", "shell-session":
		return "bash"
	}
	return lang
}

// strings4HasPrefix 是 strings.HasPrefix(line[i:], prefix)，但省一次切片。
func strings4HasPrefix(line string, i int, prefix string) bool {
	if len(prefix) == 0 || i+len(prefix) > len(line) {
		return false
	}
	for k := 0; k < len(prefix); k++ {
		if line[i+k] != prefix[k] {
			return false
		}
	}
	return true
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// isAllUpper 判断是不是全大写（允许数字和下划线），至少两个字符。
func isAllUpper(s string) bool {
	if len(s) < 2 {
		return false
	}
	hasLetter := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			hasLetter = true
		case c >= '0' && c <= '9' || c == '_':
			// ok
		default:
			return false
		}
	}
	return hasLetter
}

// lowerASCII 只处理 ASCII 大小写；语言名不会有非 ASCII。
func lowerASCII(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
