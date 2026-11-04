package main

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// open_local_test.go 钉住「打开本地路径」的分叉：目录走文件管理器、常规文件走
// Viva 的 app.OpenPath。这两条不许混——混了就是"点一次失败一次"，
// 而失败还不一定看得见（旧实现把裸路径塞给 OpenExternal，连打开的尝试都没有）。

func swapOpenLocal(t *testing.T) (dirs *[]string, files *[]string, failDir *error) {
	t.Helper()
	originalDirectory, originalPath := openDirectory, openPath
	dirCalls, fileCalls := []string{}, []string{}
	dirErr := error(nil)
	openDirectory = func(path string) error {
		dirCalls = append(dirCalls, path)
		return dirErr
	}
	openPath = func(path string) error {
		fileCalls = append(fileCalls, path)
		return nil
	}
	t.Cleanup(func() {
		openDirectory, openPath = originalDirectory, originalPath
	})
	return &dirCalls, &fileCalls, &dirErr
}

func TestOpenLocalRoutesDirectoryAndFileToDifferentRealOpeners(t *testing.T) {
	dirs, files, _ := swapOpenLocal(t)

	cwd := t.TempDir()
	file := filepath.Join(cwd, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	isDir, err := openLocal(cwd)
	if err != nil || !isDir {
		t.Fatalf("openLocal(目录) = %v, %v；want true, nil", isDir, err)
	}
	if len(*dirs) != 1 || (*dirs)[0] != cwd {
		t.Fatalf("目录没有交给文件管理器：%v，want [%q]", *dirs, cwd)
	}
	if len(*files) != 0 {
		t.Fatalf("目录被当成文件交给了 app.OpenPath：%v", *files)
	}

	isDir, err = openLocal(file)
	if err != nil || isDir {
		t.Fatalf("openLocal(文件) = %v, %v；want false, nil", isDir, err)
	}
	if len(*files) != 1 || (*files)[0] != file {
		t.Fatalf("常规文件没有交给 app.OpenPath：%v，want [%q]", *files, file)
	}
	if len(*dirs) != 1 {
		t.Fatalf("文件被当成目录交给文件管理器：%v", *dirs)
	}

	if _, err := openLocal(filepath.Join(cwd, "missing.txt")); err == nil {
		t.Fatal("不存在的路径应该报错（不许退回到进程 CWD 或父目录）")
	}
	if len(*dirs) != 1 || len(*files) != 1 {
		t.Fatalf("不存在的路径仍然触发了 OS 动作：dirs=%v files=%v", *dirs, *files)
	}
}

func TestOpenLocalDirectoryFailureIsVisibleInTheStatusBar(t *testing.T) {
	dirs, files, dirErr := swapOpenLocal(t)
	*dirErr = errors.New("injected file manager failure")

	cwd := t.TempDir()
	link := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(cwd)}).String()
	m := &model{}
	if err := m.openFile(link); err == nil {
		t.Fatal("文件管理器失败时 openFile 必须返回错误")
	}
	if len(*dirs) != 1 || (*dirs)[0] != cwd {
		t.Fatalf("精确 CWD 没有到达文件管理器：%v", *dirs)
	}
	if len(*files) != 0 {
		t.Fatalf("目录被交给了 app.OpenPath：%v", *files)
	}
	if status := openInAppStatus(m); !strings.Contains(status, "打开目录失败") {
		t.Fatalf("状态栏 = %q，want 含「打开目录失败」", status)
	}
}

func TestOpenFileRejectsBarePathsBeforeAnyOSCall(t *testing.T) {
	dirs, files, _ := swapOpenLocal(t)
	m := &model{}
	for _, bare := range []string{`D:\ws`, "README.md", "https://example.com/x"} {
		if err := m.openFile(bare); err == nil {
			t.Fatalf("裸路径 %q 被当成可打开目标", bare)
		}
	}
	if len(*dirs) != 0 || len(*files) != 0 {
		t.Fatalf("裸路径触发了 OS 动作：dirs=%v files=%v", *dirs, *files)
	}
	if status := openInAppStatus(m); !strings.Contains(status, "打开文件失败") {
		t.Fatalf("状态栏 = %q，want 含「打开文件失败」", status)
	}
}
