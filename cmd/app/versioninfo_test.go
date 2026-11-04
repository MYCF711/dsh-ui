package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 这条守卫盯的是**入库的生成物与它的事实来源是否还一致**。
//
// # 为什么它不是"洁癖"
//
// `cmd/app/versioninfo_windows_amd64.syso` 是 Windows 资源段，由
// `go run ./internal/vsinfo` 从 `brand.go` 生成（`go:generate` 就在 brand.go 里）。
// 它的全部价值只有一个：**任务管理器/资源管理器里能看出这是谁**。
//
// 而它坏掉的时候**什么都不响**：
//
//   - `go build` 正常；
//   - 链接正常；
//   - 程序正常跑；
//   - 单测全绿；
//   - 只有 Windows 读 exe 的 VersionInfo 时才表现为"描述列还是旧名字"。
//
// 2026-11-04 真撞到：改名成 `DSH GO GUI` / `dshgogui.exe` 之后，仓库里那个 .syso
// 还是上一版生成的，于是
//
//	(Get-Item tmp/dshgogui.exe).VersionInfo
//	  ProductName      : DSH GUI GO        ← 旧名字
//	  OriginalFilename : dsh-gui.exe       ← 旧名字
//
// 而所有测试都是绿的。
//
// # 判据
//
// 重新生成一份，和入库的那份**逐字节比**。不一致就红，并把人话和修法一起打出来。

const (
	sysoPath   = "versioninfo_windows_amd64.syso"
	genPackage = "./internal/vsinfo"
)

func TestVersionInfoResourceMatchesBrand(t *testing.T) {
	committed, err := os.ReadFile(sysoPath)
	if err != nil {
		t.Fatalf("读不到入库的资源段 %s：%v\n"+
			"它应该由 `go generate ./cmd/app` 生成并入库——没有它，exe 的「任务管理器描述」就是空的。",
			sysoPath, err)
	}

	out := filepath.Join(t.TempDir(), "fresh.syso")
	cmd := exec.Command("go", "run", genPackage, "-o", out)
	cmd.Dir = "."
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("生成器跑不起来：%v\n%s", err, stderr.String())
	}
	fresh, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("生成器没写出 %s：%v", out, err)
	}

	if bytes.Equal(committed, fresh) {
		return
	}

	// 不一致：把两边的可读字符串都抽出来，人一眼就知道差在哪。
	diff := describe(committed, fresh)
	t.Fatalf("入库的 %s 和 brand.go 对不上（%d 字节 vs 新生成的 %d 字节）。\n\n%s\n\n"+
		"修法：\n  cd cmd/app && go run ./internal/vsinfo -o %s\n然后重新提交这个 .syso。\n"+
		"为什么必须修：这个资源段的唯一用途就是让 Windows 的『描述』列显示产品名；\n"+
		"**陈旧时程序照常跑、测试照常绿**，只有任务管理器里还是旧名字。",
		sysoPath, len(committed), len(fresh), diff, sysoPath)
}

// describe 抽出十六进制的 UTF-16LE 字符串，够看清 ProductName / OriginalFilename 这类值。
func describe(a, b []byte) string {
	old := utf16Strings(a)
	fresh := utf16Strings(b)
	var lines []string
	seen := map[string]bool{}
	for _, s := range old {
		seen[s] = true
	}
	for _, s := range fresh {
		if !seen[s] {
			lines = append(lines, "  新生成的里面有、入库的没有： "+s)
		}
	}
	inNew := map[string]bool{}
	for _, s := range fresh {
		inNew[s] = true
	}
	for _, s := range old {
		if !inNew[s] {
			lines = append(lines, "  入库的那份里有、新生成的没有： "+s)
		}
	}
	if len(lines) == 0 {
		return "  （可读字符串一致，差异在二进制布局）"
	}
	return strings.Join(lines, "\n")
}

// utf16Strings 抠出资源段里的 UTF-16LE 字符串（版本资源的键和值都是这么存的）。
func utf16Strings(b []byte) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) >= 3 {
			out = append(out, string(cur))
		}
		cur = cur[:0]
	}
	for i := 0; i+1 < len(b); i += 2 {
		r := rune(b[i]) | rune(b[i+1])<<8
		if r >= 32 && r < 0x7f {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
