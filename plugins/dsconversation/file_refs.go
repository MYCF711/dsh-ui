package dsconversation

import (
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// knownFileReference 把 write/edit 参数里的原始显示路径与已经解析好的绝对
// file URL 配对。显示路径保持原样；CWD 只进入 destination，不改写助手正文。
type knownFileReference struct {
	display     string
	destination string
}

// rewriteAssistantFileLinks 先保留 WP-C1 的显式 file: Markdown 链接，再只把
// dsdeliverables 已确认的 write/edit 参数路径提升为链接。普通字符串、未知路径、
// URL、行内码、既有 Markdown 链接和 fenced code 都不会在这里获得文件语义。
func rewriteAssistantFileLinks(source string, writtenPaths []string, cwd string) (string, map[string]string) {
	rewritten, files := rewriteExplicitFileLinks(source)
	refs := knownFileReferences(writtenPaths, cwd)
	if len(refs) == 0 {
		return rewritten, files
	}
	lines := strings.SplitAfter(rewritten, "\n")
	var fence string
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		indent := len(line) - len(trimmed)
		if indent <= 3 && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			marker := trimmed[:3]
			if fence == "" {
				fence = marker
			} else if marker == fence {
				fence = ""
			}
			continue
		}
		if fence == "" {
			lines[i] = rewriteKnownFileReferencesInText(line, refs, files)
		}
	}
	return strings.Join(lines, ""), files
}

func knownFileReferences(paths []string, cwd string) []knownFileReference {
	refs := make([]knownFileReference, 0, len(paths))
	seen := map[string]bool{}
	for _, written := range paths {
		display, _ := splitFileLineFragment(written)
		if display == "" {
			continue
		}
		absolute, ok := resolveKnownFilePath(display, cwd)
		if !ok {
			continue
		}
		destination := knownFileURL(absolute, "")
		key := display + "\x00" + destination
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, knownFileReference{display: display, destination: destination})
	}
	// 一个短文件名可能正好是另一个已知路径的后缀；最长优先可避免只链接内层片段。
	sort.SliceStable(refs, func(i, j int) bool { return len(refs[i].display) > len(refs[j].display) })
	return refs
}

// resolveKnownFilePath 只解析工具参数已经明确给出的路径。相对路径必须同时有
// 绝对 Snapshot.CWD；缺 CWD、CWD 本身未知或不完整时都返回 false，不退回进程 CWD。
func resolveKnownFilePath(path, cwd string) (string, bool) {
	if absolute, ok := cleanAbsoluteFilePath(path); ok {
		return absolute, true
	}
	root, ok := cleanAbsoluteFilePath(cwd)
	if !ok {
		return "", false
	}
	absolute := filepath.Clean(filepath.Join(root, path))
	if !filepath.IsAbs(absolute) {
		return "", false
	}
	return absolute, true
}

func cleanAbsoluteFilePath(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	// DSH 工具在 Windows 上也允许 Git Bash 的 /c/... 绝对形式。这里只转换
	// 明确的盘符形式；/tmp、/name 不会被猜成当前盘符下的文件。
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' &&
		isASCIILetter(path[1]) && path[2] == '/' {
		path = strings.ToUpper(path[1:2]) + ":" + path[2:]
	}
	if !filepath.IsAbs(path) {
		return "", false
	}
	return filepath.Clean(path), true
}

func knownFileURL(absolute, fragment string) string {
	slash := filepath.ToSlash(absolute)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	return (&url.URL{Scheme: "file", Path: slash, Fragment: fragment}).String()
}

// splitFileLineFragment 识别显示文本末尾的 #L24 或 #L24-L30。返回值 fragment
// 不含 '#'，只用于链接显示和未来编辑器定位，绝不参与 filepath.Join/Clean。
func splitFileLineFragment(text string) (base, fragment string) {
	start := strings.LastIndex(text, "#L")
	if start <= 0 {
		return text, ""
	}
	end, ok := fileLineFragmentEnd(text, start)
	if !ok || end != len(text) {
		return text, ""
	}
	return text[:start], text[start+1 : end]
}

func fileLineFragmentEnd(text string, start int) (int, bool) {
	if start < 0 || start+2 >= len(text) || text[start] != '#' || text[start+1] != 'L' {
		return start, false
	}
	i := start + 2
	first := i
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	if i == first {
		return start, false
	}
	if i == len(text) || text[i] != '-' {
		return i, true
	}
	i++
	if i >= len(text) || text[i] != 'L' {
		return start, false
	}
	i++
	second := i
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	if i == second {
		return start, false
	}
	return i, true
}

func rewriteKnownFileReferencesInText(line string, refs []knownFileReference, files map[string]string) string {
	var out strings.Builder
	for i := 0; i < len(line); {
		// 行内码整段原样复制，包括反引号数量与正文。
		if line[i] == '`' {
			n := repeatedByteRun(line, i, '`')
			marker := strings.Repeat("`", n)
			if end := strings.Index(line[i+n:], marker); end >= 0 {
				end += i + n + n
				out.WriteString(line[i:end])
				i = end
				continue
			}
		}
		// 既有 Markdown 链接（含 WP-C1 已改写的显式 file 链接）整体跳过，
		// 不嵌套链接，也不把 destination 里的已知字符串再次猜成文件。
		if line[i] == '[' {
			if end := markdownLinkEnd(line, i); end > i {
				out.WriteString(line[i:end])
				i = end
				continue
			}
		}
		// URL 即使 query 或 path 中含有同名字符串，也仍然只是 URL。
		if end := explicitURLEnd(line, i); end > i {
			out.WriteString(line[i:end])
			i = end
			continue
		}
		if i == 0 || !fileReferenceByte(line[i-1]) {
			if end := rewriteKnownFileReferenceAt(line, i, refs, files, &out); end > i {
				i = end
				continue
			}
		}
		out.WriteByte(line[i])
		i++
	}
	return out.String()
}

// rewriteKnownFileReferenceAt 成功时把下一处普通文本写成 Markdown 链接并返回结束下标。
func rewriteKnownFileReferenceAt(line string, start int, refs []knownFileReference, files map[string]string, out *strings.Builder) int {
	for _, ref := range refs {
		if !strings.HasPrefix(line[start:], ref.display) {
			continue
		}
		end := start + len(ref.display)
		fragment := ""
		if end < len(line) && line[end] == '#' {
			if fragmentEnd, ok := fileLineFragmentEnd(line, end); ok {
				fragment = line[end+1 : fragmentEnd]
				end = fragmentEnd
			}
		}
		if end < len(line) && !fileReferenceBoundaryAfter(line, end) {
			continue
		}
		destination := ref.destination
		if fragment != "" {
			destination = knownFileURLFromBase(ref.destination, fragment)
		}
		fake := "https://file.invalid/" + strconv.Itoa(len(files)+1)
		files[fake] = destination
		out.WriteByte('[')
		out.WriteString(escapeMarkdownLinkLabel(ref.display))
		if fragment != "" {
			out.WriteByte('#')
			out.WriteString(fragment)
		}
		out.WriteString("](")
		out.WriteString(fake)
		out.WriteByte(')')
		return end
	}
	return -1
}

func knownFileURLFromBase(base, fragment string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	u.Fragment = fragment
	return u.String()
}

func repeatedByteRun(text string, start int, b byte) int {
	n := 0
	for start+n < len(text) && text[start+n] == b {
		n++
	}
	return n
}

// markdownLinkEnd 返回既有 [label](destination) 的结束下标；不支持或不完整时
// 返回 -1，让后续扫描把符号当普通文本。括号转义与嵌套括号均不进入文件判断。
func markdownLinkEnd(text string, start int) int {
	depth := 0
	labelEnd := -1
	for i := start; i < len(text) && i-start <= 1000; i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		switch text[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				labelEnd = i
			}
		}
		if labelEnd >= 0 {
			break
		}
	}
	if labelEnd < 0 || labelEnd+1 >= len(text) || text[labelEnd+1] != '(' {
		return -1
	}
	parens := 0
	for i := labelEnd + 2; i < len(text) && i-labelEnd <= 2048; i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		switch text[i] {
		case '(':
			parens++
		case ')':
			if parens == 0 {
				return i + 1
			}
			parens--
		}
	}
	return -1
}

func explicitURLEnd(text string, start int) int {
	colon := strings.IndexByte(text[start:], ':')
	if colon <= 0 || start+colon+1 >= len(text) {
		return -1
	}
	colon += start
	for i := start; i < colon; i++ {
		c := text[i]
		if !isASCIILetter(c) && !(i > start && (c == '+' || c == '-' || c == '.' || c >= '0' && c <= '9')) {
			return -1
		}
	}
	if !strings.HasPrefix(text[colon+1:], "//") && !strings.EqualFold(text[start:colon], "mailto") {
		return -1
	}
	end := colon + 1
	for end < len(text) && text[end] > ' ' && !strings.ContainsRune("<>`", rune(text[end])) {
		end++
	}
	return end
}

func fileReferenceBoundaryAfter(text string, end int) bool {
	c := text[end]
	if c == '.' {
		// foo.go.bak 是更长路径；foo.go. 的句点则是普通句子标点。
		return end+1 == len(text) || !isASCIIAlphaNumeric(text[end+1])
	}
	return !fileReferenceByte(c)
}

func fileReferenceByte(c byte) bool {
	return isASCIIAlphaNumeric(c) || strings.ContainsRune(`/\\._-~:%?&=#+@$`, rune(c))
}

func isASCIILetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isASCIIAlphaNumeric(c byte) bool {
	return isASCIILetter(c) || c >= '0' && c <= '9'
}

// fileLinkCacheKey 让 fake URL 背后的真实目的地也参与缓存身份。长度前缀避免
// 路径里的分隔符造成拼接歧义；排序只用于键，不改变链接出现顺序。
func fileLinkCacheKey(files map[string]string) string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, key := range keys {
		destination := files[key]
		out.WriteString(strconv.Itoa(len(key)))
		out.WriteByte(':')
		out.WriteString(key)
		out.WriteString(strconv.Itoa(len(destination)))
		out.WriteByte(':')
		out.WriteString(destination)
	}
	return out.String()
}

func escapeMarkdownLinkLabel(label string) string {
	return strings.NewReplacer(
		`\`, `\\`, `[`, `\[`, `]`, `\]`, "`", "\\`",
		`*`, `\*`, `_`, `\_`, `~`, `\~`,
	).Replace(label)
}
