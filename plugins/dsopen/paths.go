package dsopen

import (
	"errors"
	"net/url"
	"strings"
)

// paths.go 只放纯函数：判"是不是绝对路径"、把绝对路径造成 `file:` 链接。
//
// 为什么这里有一份、`plugins/dsfiles` 里还有一份：两个插件互不依赖，
// 包内函数不导出（契约里只有 `Actions.OpenFile(destination string)`）。
// 两边遵守的是同一条契约——**只交显式 `file:` 链接，裸路径到这里为止**：
// 壳的 `filePathFromLink` 不会把普通字符串猜成路径，猜错就会去开一个
// 用户没想开的文件。

var errNotAbsolute = errors.New("不是绝对路径")

// absoluteCWD 判断 `Snapshot.CWD` 能不能当成"一个真实的绝对目录"。
//
// 认：`D:\ws`、`d:/ws`、`D:\`、`\\server\share`、`/srv/ws`、`/`。
// 不认：空串、相对路径、`D:ws`（盘符相对）、首尾空白、`//`（没有 server/share
// 的 UNC 残片）。
//
// **绝不退回进程 CWD**：猜出来的路径会把用户带到"另一个目录"，而且看起来
// 一切正常——这是这一层最贵的错法。
func absoluteCWD(cwd string) bool {
	_, ok := slashPath(cwd)
	return ok
}

// slashPath 把反斜杠统一成正斜杠并判绝对性，返回规范化后的路径。
//
// 两处（判绝对、造链接）共用同一个判据：分开写就会出现"判得过、链接造不出"
// 或者反过来的裂缝。
func slashPath(abs string) (string, bool) {
	if abs == "" || abs != strings.TrimSpace(abs) {
		return "", false
	}
	path := strings.ReplaceAll(abs, `\`, "/")
	switch {
	case strings.HasPrefix(path, "//"):
		// UNC：`\\server\share`。只写 `\\` 或 `\\server` 都不算——那不是一个
		// 能打开的共享目录。
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return "", false
		}
		return path, true
	case strings.HasPrefix(path, "/"):
		return path, true
	case len(path) >= 3 && path[1] == ':' && path[2] == '/':
		if isDriveLetter(path[0]) {
			return path, true
		}
	}
	return "", false
}

func isDriveLetter(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// fileURL 把绝对路径翻成显式的 `file:` 链接，交给 `Actions.OpenFile`。
//
// Windows 盘符路径要补前导斜杠（`D:/ws` → `/D:/ws` → `file:///D:/ws`），
// 空格和 Unicode 交给 url.URL 转义——不要自己拼 `%`。
func fileURL(abs string) (string, error) {
	path, ok := slashPath(abs)
	if !ok {
		return "", errNotAbsolute
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String(), nil
}
