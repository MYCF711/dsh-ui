package main

import (
	"path/filepath"
	"testing"
)

func TestWPC1FilePathFromExplicitLinkOnly(t *testing.T) {
	for _, input := range []string{
		"README.md", `C:\\work\\README.md`, "https://example.com/file.txt", "file.txt#L2", "",
	} {
		if got, err := filePathFromLink(input); err == nil {
			t.Fatalf("普通字符串/非 file link 被猜成路径 %q → %q", input, got)
		}
	}
	got, err := filePathFromLink("file:///C:/work/a%20b.go#L24-L30")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.FromSlash("C:/work/a b.go")
	if got != want {
		t.Fatalf("文件路径 = %q，期望 %q（行号不得进入文件名）", got, want)
	}
}

func TestWPC1OpenExternalFailureIsVisible(t *testing.T) {
	original := openURL
	openURL = func(string) error { return errInjectedOpen }
	defer func() { openURL = original }()
	m := &model{}
	m.openExternal("https://example.com")
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status == "" || m.status == "已打开链接" {
		t.Fatalf("打开失败没有可见反馈：%q", m.status)
	}
}

var errInjectedOpen = injectedOpenError("injected open failure")

type injectedOpenError string

func (e injectedOpenError) Error() string { return string(e) }
