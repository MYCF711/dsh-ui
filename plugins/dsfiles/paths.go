package dsfiles

import (
	"errors"
	"net/url"
	"strings"
)

// paths.go 放这条链路上**能单测的部分**：拼路径、造 file: 链接。
//
// 抽出来的原因很直接：这个包长期**一个测试都没有**，于是"列表会进子目录、
// 但点文件仍用根目录"这个 bug 一直活着（见文件头）。路径拼接这种纯函数
// 是唯一能在单测里钉死的地方，剩下的绘制只能靠真像素。

// dirPath 是"当前正在浏览的目录"的绝对路径。
//
// **拼路径只走这一处**。列表问 `FSList(dirPath(...))`、点文件读
// `open(dirPath(...), name)`,两边必须同一个算法——分开写就会重演那个 bug：
// 列表进得去子目录，点文件却回头读根目录。
func dirPath(cwd, curDir string) string {
	if cwd == "" {
		return ""
	}
	if curDir == "" {
		return cwd
	}
	return strings.TrimRight(cwd, "/\\") + "/" + strings.Trim(curDir, "/\\")
}

// fileURL 把一个绝对路径翻成 `file:` 链接，给 `Actions.OpenFile` 用。
//
// 为什么非要是链接而不是裸路径：壳那边的 `filePathFromLink` **只接受显式
// file: URL**（"普通字符串不得在这里猜成路径"——猜错就会去开一个用户没想开的文件）。
// 所以插件这边负责把它造成一个合法的、带 scheme 的东西。
//
// Windows 盘符路径要补前导斜杠：`D:\ws\a.go` → `/D:/ws/a.go` → `file:///D:/ws/a.go`。
// 空格之类的字符交给 url.URL 去转义（`a b.txt` → `a%20b.txt`），不要自己拼 %。
func fileURL(abs string) (string, error) {
	if strings.TrimSpace(abs) == "" {
		return "", errors.New("空路径")
	}
	if abs != strings.TrimSpace(abs) {
		return "", errors.New("路径首尾有空白")
	}
	p := strings.ReplaceAll(abs, `\`, "/")
	// 先判绝对性，**再**补前导斜杠。顺序反了就判不出来：
	// 给 `相对/路径.txt` 补上 "/" 之后，它就"看起来"像绝对路径了。
	//
	// 第一版就是这么错的，判据写成"斜杠个数 ≥ 2 或者有冒号"——
	// 于是相对路径一路过，造出一个 `file:///%E7%9B%B8%E5%AF%B9/...` 的链接，
	// 壳那边解析出来是非绝对路径，报一个用户看不懂的错。
	if !isAbsSlashPath(p) {
		return "", errors.New("不像绝对路径：" + abs)
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // 盘符路径补前导斜杠：D:/ws → /D:/ws
	}
	return (&url.URL{Scheme: "file", Path: p}).String(), nil
}

// isAbsSlashPath 判一个**已经统一成正斜杠**的路径是不是绝对路径。
//
// 两种算绝对：以 `/` 开头（POSIX 根、以及 `\\server\share` 转过来的 UNC），
// 或者盘符形式 `X:/`。其余一律不认——宁可不造链接，也不要造一个壳会拒绝的。
func isAbsSlashPath(p string) bool {
	if strings.HasPrefix(p, "/") {
		return true
	}
	if len(p) < 3 || p[1] != ':' || p[2] != '/' {
		return false
	}
	c := p[0]
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}
