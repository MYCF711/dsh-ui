package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vivaui/viva/app"
)

var (
	setClipboardText = app.SetClipboardText
	openURL          = app.OpenURL
	openPath         = app.OpenPath
)

func (m *model) copyText(text string) error {
	if err := setClipboardText(text); err != nil {
		m.setStatus("复制失败: " + err.Error())
		return err
	}
	m.setStatus("已复制 " + itoaLocal(len([]rune(text))) + " 字")
	return nil
}

func (m *model) openExternal(destination string) error {
	if err := openURL(destination); err != nil {
		m.setStatus("打开链接失败: " + err.Error())
		return err
	}
	m.setStatus("已打开链接")
	return nil
}

// openFile 打开一条显式 file: 链接指向的本地路径。文件与目录都走这里：
//
//   - 常规文件 → Viva `app.OpenPath`（平台默认程序；driver 运行记录 LastOpenedPath）
//   - 目录     → 平台文件管理器（见 open_local.go：`app.OpenPath` 明确拒绝目录，
//     拿它去开工作目录只会得到一条 `is not a regular file`）
//
// 失败只写既有状态栏——一处动作失败该只有一处提示。
func (m *model) openFile(destination string) error {
	path, err := filePathFromLink(destination)
	isDir := false
	if err == nil {
		isDir, err = openLocal(path)
	}
	if err != nil {
		if isDir {
			m.setStatus("打开目录失败: " + err.Error())
		} else {
			m.setStatus("打开文件失败: " + err.Error())
		}
		return err
	}
	if isDir {
		m.setStatus("已打开目录")
	} else {
		m.setStatus("已打开文件")
	}
	return nil
}

// filePathFromLink 只接受显式 file: URL；#L24-L30 是编辑器定位信息，不属于
// 文件名。普通路径、https URL 与相对 file host 都不会在这里被升级成打开动作。
func filePathFromLink(destination string) (string, error) {
	u, err := url.Parse(destination)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(u.Scheme, "file") || u.User != nil || (u.Host != "" && !strings.EqualFold(u.Host, "localhost")) {
		return "", fmt.Errorf("不是可打开的显式 file 链接")
	}
	if u.Path == "" || strings.IndexByte(u.Path, 0) >= 0 {
		return "", fmt.Errorf("file 链接缺少路径")
	}
	path := filepath.FromSlash(u.Path)
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == filepath.Separator && path[2] == ':' {
		path = path[1:]
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("file 链接必须是绝对路径")
	}
	return filepath.Clean(path), nil
}
