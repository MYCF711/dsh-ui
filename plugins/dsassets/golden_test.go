package dsassets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 本文件是"素材真的是官方那一串"的证据链，分三层：
//
//  1. `source_test`（registry_test.go）：每条素材都写了包/文件/符号/sha256，不能空；
//  2. 这里：`paths.go` 的常量逐字节出现在 `testdata/official/<ID>.jsx`（官方原文片段），
//     且 `manifest.json` 里记的 sha256 与常量一致；
//  3. 本机有 `tmp/official/` 时（解包过的官方包），再验一遍片段确实出现在官方文件里、
//     官方文件的 sha256 也与清单一致——把"当时取到的是哪一份"钉死。
//
// 三层里第 3 层在干净检出的 CI 上会跳过（tmp/ 是 gitignored，见 scripts/doc-links.py
// 对"本机证据"的同类处理），但 1、2 两层永远跑。

const testdataDir = "testdata/official"

// manifestEntry 对应 scripts 生成的清单结构（见 tmp/gen-testdata.js）。
type manifestEntry struct {
	Package        string `json:"package"`
	File           string `json:"file"`
	Symbol         string `json:"symbol"`
	Binding        string `json:"binding"`
	FileSHA256     string `json:"file_sha256"`
	FragmentSHA256 string `json:"fragment_sha256"`
	Paths          []struct {
		D      string `json:"d"`
		SHA256 string `json:"sha256"`
		Len    int    `json:"len"`
	} `json:"paths"`
	Elements []struct {
		Tag string `json:"tag"`
	} `json:"elements"`
}

func readManifest(t *testing.T) map[string]manifestEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testdataDir, "manifest.json"))
	if err != nil {
		t.Fatalf("读不到清单 %s/manifest.json: %v", testdataDir, err)
	}
	var out map[string]manifestEntry
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("清单 JSON 坏了: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("清单是空的")
	}
	return out
}

// fragmentText 去掉片段文件末尾的换行：清单里的 sha256 与"官方原文里能不能找到"
// 都按**没有尾换行**的那一串算（片段文件本身以换行结尾，那是 git 卫生，不是数据）。
func fragmentText(raw []byte) string {
	return strings.TrimRight(string(raw), "\n")
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestConstantsAppearVerbatimInOfficialFragment：常量必须逐字节出现在官方片段里。
//
// 这条是"别看差不多"的机器版：改一个数字、少一个空格、把 V 提前展开成 L，
// 都会让 `strings.Contains` 失败。
func TestConstantsAppearVerbatimInOfficialFragment(t *testing.T) {
	for _, a := range All() {
		fragment, err := os.ReadFile(filepath.Join(testdataDir, a.ID+".jsx"))
		if err != nil {
			t.Fatalf("%s 缺少官方片段文件: %v", a.ID, err)
		}
		text := fragmentText(fragment)
		for i, el := range a.Elements {
			if el.Kind != PathElement {
				continue
			}
			if !strings.Contains(text, el.D) {
				t.Errorf("%s 第 %d 条 path 不是官方原文（片段里找不到）:\n  %s", a.ID, i, el.D)
			}
		}
	}
}

// TestManifestMatchesRegistry：清单与登记表一一对应，且 d 的 sha256/长度都对得上。
func TestManifestMatchesRegistry(t *testing.T) {
	manifest := readManifest(t)
	if len(manifest) != len(All()) {
		t.Fatalf("清单 %d 条，登记表 %d 条", len(manifest), len(All()))
	}
	for _, a := range All() {
		entry, ok := manifest[a.ID]
		if !ok {
			t.Fatalf("%s 不在清单里", a.ID)
		}
		if entry.Package != a.Source.Package || entry.File != a.Source.File || entry.Symbol != a.Source.Symbol {
			t.Errorf("%s 出处不一致：清单=%s/%s#%s 登记表=%s",
				a.ID, entry.Package, entry.File, entry.Symbol, a.Source)
		}
		if entry.FileSHA256 != a.Source.SHA256 {
			t.Errorf("%s 文件 sha256 不一致：清单=%s 登记表=%s", a.ID, entry.FileSHA256, a.Source.SHA256)
		}
		// 片段本身的哈希也得对得上——防止有人改了 testdata 却没改 constants，
		// 或者反过来。
		fragment, err := os.ReadFile(filepath.Join(testdataDir, a.ID+".jsx"))
		if err != nil {
			t.Fatal(err)
		}
		if got := hashOf(fragmentText(fragment)); got != entry.FragmentSHA256 {
			t.Errorf("%s 片段文件被改过：sha256=%s，清单=%s", a.ID, got, entry.FragmentSHA256)
		}

		var ourPaths []string
		for _, el := range a.Elements {
			if el.Kind == PathElement {
				ourPaths = append(ourPaths, el.D)
			}
		}
		if len(entry.Paths) != len(ourPaths) {
			t.Fatalf("%s path 数不一致：清单=%d 登记表=%d", a.ID, len(entry.Paths), len(ourPaths))
		}
		for i, p := range entry.Paths {
			if p.D != ourPaths[i] {
				t.Errorf("%s 第 %d 条 path 与清单不同", a.ID, i)
			}
			if p.SHA256 != hashOf(ourPaths[i]) {
				t.Errorf("%s 第 %d 条 path 的 sha256 与清单不同", a.ID, i)
			}
			if p.Len != len(ourPaths[i]) {
				t.Errorf("%s 第 %d 条 path 长度 %d != 清单 %d", a.ID, i, len(ourPaths[i]), p.Len)
			}
		}
		// 图元种类也要对上：`<rect>`/`<circle>` 素材正是不可构建的那几条。
		if len(entry.Elements) != len(a.Elements) {
			t.Fatalf("%s 图元数不一致：清单=%d 登记表=%d", a.ID, len(entry.Elements), len(a.Elements))
		}
		for i, el := range a.Elements {
			wantTag := "path"
			switch el.Kind {
			case RectElement:
				wantTag = "rect"
			case CircleElement:
				wantTag = "circle"
			}
			if entry.Elements[i].Tag != wantTag {
				t.Errorf("%s 第 %d 个图元种类：清单=%s 登记表=%s", a.ID, i, entry.Elements[i].Tag, wantTag)
			}
		}
	}
}

// TestOfficialFileWhenExtracted：本机解包目录在时，验官方文件哈希与片段原文。
//
// 干净检出里 `tmp/official/` 不存在，这条只打印原因并跳过——不能因为"没解包"
// 就让整个测试红，但**解包了却对不上**必须红：那说明素材与当前安装态官方包
// 已经不是同一份了，文档里的 exact/replace 结论要重新核。
func TestOfficialFileWhenExtracted(t *testing.T) {
	root := filepath.Join("..", "..", ExtractedDir)
	if _, err := os.Stat(root); err != nil {
		t.Skipf("本机没有解包目录 %s（干净检出正常），跳过官方文件核对", root)
	}
	checked := 0
	for _, a := range All() {
		// 本机两种解包布局都要认：npm 包原样是 <pkg>/lib/client.js，
		// 早期按包根导出的是 <pkg>/client.js。登记表保留官方包内路径，
		// 这里只对本机解包做一次等价回退；哈希仍按同一份原文校验。
		candidates := []string{a.Source.File}
		if strings.Contains(a.Source.File, "/lib/") {
			candidates = append(candidates, strings.Replace(a.Source.File, "/lib/", "/", 1))
		}
		var raw []byte
		var used string
		var firstErr error
		for _, rel := range candidates {
			file := filepath.Join(root, filepath.FromSlash(rel))
			b, err := os.ReadFile(file)
			if err == nil {
				raw, used = b, rel
				break
			}
			if firstErr == nil {
				firstErr = err
			}
		}
		if raw == nil {
			// 解包目录存在但缺这个包：说明解包不完整，跳过但说清楚。
			t.Logf("%s: 读不到 %s（%v），跳过该条", a.ID, a.Source.File, firstErr)
			continue
		}
		if got := hashOf(string(raw)); got != a.Source.SHA256 {
			t.Errorf("%s 的官方文件哈希变了：%s = %s，登记表 %s", a.ID, used, got, a.Source.SHA256)
			continue
		}
		// 逐条 `d` 在一份可能被 prettier 排过版、也可能被压成一行的文件里找：
		// 两边都去掉空白再比，比对的是**字符本身**，不是排版。
		haystack := compactSpace(string(raw))
		for i, el := range a.Elements {
			if el.Kind != PathElement {
				continue // <rect>/<circle> 的数值由"文件哈希 + 清单"那一层保证
			}
			if !strings.Contains(haystack, compactSpace(el.D)) {
				t.Errorf("%s 第 %d 条 path 在 %s 里找不到（素材与当前安装态官方包已经不是同一份）", a.ID, i, used)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Skip("解包目录不完整（本机缺 web-frontend/子目录布局），改由 testdata/official 自包含清单核对")
	}
	t.Logf("官方文件核对通过：%d 条素材", checked)
}

// compactSpace 去掉所有空白：官方文件有的包是 prettier 排过版的、有的是压成一行的，
// 比对素材原文时只该看字符，不该看缩进和换行。
func compactSpace(s string) string {
	return strings.Join(strings.Fields(s), "")
}
