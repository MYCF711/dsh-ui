// market_test.go —— 条目/版本投影的规则测试。
//
// 每一条判据都对着官方同一段代码，不是猜的：
//
//	presentedVersion   OFF:../dsh-client-ui-plugin-market/src/client/presentation.ts:69-77
//	prerelease         OFF:../dsh-client-ui-plugin-market/src/client/DiscoverSection.tsx:78-81
//	sourceFacts        OFF:../dsh-client-ui-plugin-market/src/client/presentation.ts:85-104
//	版本行标签与说明    OFF:../dsh-client-ui-plugin-market/src/client/DiscoverSection.tsx:166-215
package dsmarket

import (
	"regexp"
	"strings"
	"testing"
)

func TestPresentedVersionPrefersDefault(t *testing.T) {
	entry := Entry{
		Versions:       []Version{{Version: "2.0.0"}, {Version: "1.9.0"}, {Version: "1.8.0"}},
		DefaultVersion: "1.9.0",
	}
	got, ok := PresentedVersion(entry)
	if !ok || got.Version != "1.9.0" {
		t.Fatalf("应当选默认版本 1.9.0，得到 %+v（ok=%v）", got, ok)
	}
}

// TestPresentedVersionFallsBackToFirst 钉住官方那条兜底："默认版本不在列里时退到第一条"
// （presentation.ts:74-77：`entry.versions.find(…) ?? entry.versions[0]`）。
func TestPresentedVersionFallsBackToFirst(t *testing.T) {
	entry := Entry{Versions: []Version{{Version: "2.0.0"}, {Version: "1.0.0"}}, DefaultVersion: "9.9.9"}
	got, _ := PresentedVersion(entry)
	if got.Version != "2.0.0" {
		t.Fatalf("默认版本缺席时应退到第一条，得到 %q", got.Version)
	}
	if _, ok := PresentedVersion(Entry{}); ok {
		t.Fatal("一条版本都没有时不该给出版本")
	}
}

func TestPrereleaseFollowsSemver(t *testing.T) {
	cases := map[string]bool{
		"1.0.0":          false,
		"1.0.0-beta.1":   true,
		"1.0.0+build.5":  false,
		"1.0.0-rc.1+sha": true,
		"":               false,
	}
	for version, want := range cases {
		if got := Prerelease(version); got != want {
			t.Fatalf("Prerelease(%q) = %v，想要 %v", version, got, want)
		}
	}
}

// TestSourceFactsNPMAndGit 逐条对官方 sourceFacts：npm 给"包名@版本"（+ registry），
// git 给仓库 + 完整 commit（+ 包目录）。
func TestSourceFactsNPMAndGit(t *testing.T) {
	npm := SourceFacts("demo", Version{Version: "1.2.3", Source: Source{Kind: SourceNPM, Registry: "https://r.example"}})
	want := []SourceFact{{FactPackage, "demo@1.2.3"}, {FactRegistry, "https://r.example"}}
	if len(npm) != 2 || npm[0] != want[0] || npm[1] != want[1] {
		t.Fatalf("npm 来源事实 = %+v，想要 %+v", npm, want)
	}
	noRegistry := SourceFacts("demo", Version{Version: "1.2.3", Source: Source{Kind: SourceNPM}})
	if len(noRegistry) != 1 {
		t.Fatalf("没声明 registry 时不该画 registry 行：%+v", noRegistry)
	}

	git := SourceFacts("demo", Version{Version: "0.1.0", Source: Source{
		Kind: SourceGit, URL: "https://github.com/x/y", Commit: "deadbeef", Path: "packages/demo",
	}})
	wantGit := []SourceFact{
		{FactRepository, "https://github.com/x/y"},
		{FactCommit, "deadbeef"},
		{FactDirectory, "packages/demo"},
	}
	if len(git) != 3 || git[0] != wantGit[0] || git[1] != wantGit[1] || git[2] != wantGit[2] {
		t.Fatalf("git 来源事实 = %+v，想要 %+v", git, wantGit)
	}
}

// TestVersionRowsTagOrderAndMeta 钉住版本行的标签顺序与说明行三档。
//
// 标签顺序（官方 :169-190）：推荐 → 预发布 → 当前已安装 → 不兼容/已豁免。
// 说明行（官方 :193-203）：兼容 → 发布时间；已豁免 → 豁免文案；不兼容 → 运行时 + 要求范围。
func TestVersionRowsTagOrderAndMeta(t *testing.T) {
	entry := Entry{
		Versions: []Version{
			{
				Version:          "2.0.0",
				PublishTimestamp: 1767268800,
				Compatibility:    Compatibility{Compatible: true},
			},
			{Version: "2.0.0-beta.1", Compatibility: Compatibility{Compatible: true}},
			{Version: "1.9.0", Compatibility: Compatibility{Compatible: false, RuntimeVersion: "0.2.2",
				Requirements: map[string]string{"dsh": "^0.2.0"}}},
			{Version: "1.8.0", Compatibility: Compatibility{Compatible: false, Exempted: true}},
		},
		DefaultVersion: "2.0.0",
		Installation:   Installation{Status: InstallStatusInstalled, InstalledVersion: "2.0.0"},
	}
	rows := VersionRows(entry, "zh-CN")
	if len(rows) != 4 {
		t.Fatalf("版本行数 = %d", len(rows))
	}
	if got := tagTexts(rows[0].Tags); got != "推荐,当前已安装" {
		t.Fatalf("默认版本行的标签 = %q（应当是 推荐,当前已安装）", got)
	}
	if got := tagTexts(rows[1].Tags); got != "预发布" {
		t.Fatalf("预发布行的标签 = %q", got)
	}
	if got := tagTexts(rows[2].Tags); got != "不兼容" {
		t.Fatalf("不兼容行的标签 = %q", got)
	}
	if got := tagTexts(rows[3].Tags); got != "已豁免" {
		t.Fatalf("豁免行的标签 = %q", got)
	}
	if rows[2].Meta != "此版本与当前的 DSH 0.2.2 不兼容，它要求 DSH ^0.2.0" {
		t.Fatalf("不兼容说明行 = %q", rows[2].Meta)
	}
	if rows[3].Meta != "此版本与当前 DSH 不兼容，但你已为它设置了兼容性豁免" {
		t.Fatalf("豁免说明行 = %q", rows[3].Meta)
	}
	if rows[0].Meta != "2026年1月1日" {
		t.Fatalf("兼容行的说明应当是发布时间 medium 档，得到 %q", rows[0].Meta)
	}
}

// tagTexts 把标签拼成一行，方便断言顺序。
func tagTexts(tags []Tag) string {
	parts := make([]string, 0, len(tags))
	for _, tag := range tags {
		parts = append(parts, tag.Text)
	}
	return strings.Join(parts, ",")
}

// TestRequirementsRangeIsStable 钉住"多键时顺序稳定"：官方 JS 用对象插入序，Go 的
// map 遍历序是随机的，不排序就会同一份数据每次画不同行。
func TestRequirementsRangeIsStable(t *testing.T) {
	requirements := map[string]string{"b": "^2", "a": "^1", "c": "~3"}
	first := requirementsRange(requirements)
	for i := 0; i < 20; i++ {
		if got := requirementsRange(requirements); got != first {
			t.Fatalf("同一输入多次结果不同：%q vs %q", first, got)
		}
	}
	if first != "^1 ^2 ~3" {
		t.Fatalf("要求范围 = %q（按键排序）", first)
	}
}

// TestFormatDateBranches 钉住日期两档与空值。
//
// 官方是 `Intl.DateTimeFormat(language, {dateStyle:'medium'})`；Go 没有 CLDR 数据，
// 所以只实现真正会跑到的 zh / 其余（英文）两档——这是登记过的差异。
func TestFormatDateBranches(t *testing.T) {
	const stamp = 1767268800 // 2026-01-01T12:00:00Z（任何时区都还是 1 月 1 日）
	if got := FormatDate(stamp, "zh-CN"); got != "2026年1月1日" {
		t.Fatalf("zh medium = %q", got)
	}
	if got := FormatDate(stamp, "zh"); got != "2026年1月1日" {
		t.Fatalf("zh 主标签 medium = %q", got)
	}
	en := regexp.MustCompile(`^[A-Z][a-z]{2} \d{1,2}, \d{4}$`)
	if got := FormatDate(stamp, "en-US"); !en.MatchString(got) {
		t.Fatalf("en medium 形状不对：%q", got)
	}
	// 其余语言退回英文格式（而不是消失或画成 0001-01-01）。
	if got := FormatDate(stamp, "ja"); !en.MatchString(got) {
		t.Fatalf("未实现语言应退回英文 medium，得到 %q", got)
	}
	if got := FormatDate(0, "zh-CN"); got != "" {
		t.Fatalf("没有发布时间时不该画日期，得到 %q", got)
	}
}

// TestTitleTextFallsBackToName 钉住官方 entryTitle（DiscoverSection.tsx:63-65）。
func TestTitleTextFallsBackToName(t *testing.T) {
	if got := (Entry{Name: "demo"}).TitleText(); got != "demo" {
		t.Fatalf("没有标题时该用包名，得到 %q", got)
	}
	if got := (Entry{Name: "demo", Title: "演示"}).TitleText(); got != "演示" {
		t.Fatalf("有标题时该用标题，得到 %q", got)
	}
}
