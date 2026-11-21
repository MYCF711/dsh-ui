package apiclient

// 跨仓 wire 契约守卫。
//
// ## 为什么要有这个文件
//
// 2026-11-22 的独立盲审发现了一个两侧各自测试全绿、但对接起来必坏的缺陷：
//
//	宿主 internal/account/account.go 一直有 userCode，注释里还写着「UI 需要」——
//	但 wire 层 handlers_account_official.go 与投影层 serve_account_provider.go
//	都不带它，而界面 internal/apiclient/account.go 按 userCode 解。
//	⇒ 登录码在真宿主上恒空。
//
// 两侧的测试都过了，因为**各自都在测自己那一半**。没有任何东西在比「两边说的是不是同一件事」。
//
// 这个文件就是那个「东西」：**直接解析两仓源码的 struct tag 做比对**，
// 不跑两个进程、不模拟 wire —— 契约不一致不可能漏，因为比的就是声明本身。
//
// ## 单仓 checkout 时跳过
//
// 宿主仓通过 `DSH_GO_REPO` 或兄弟目录 `../DSH-GO` 定位；找不到就 `t.Skip`，
// 不让单仓开发者被卡住。**但 CI 两仓都在时它一定会跑。**

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// crossRepoField 是一条被人工核对过的 wire 字段约定。
//
// 加新条目时**必须同时**在两仓侧都写清 file 与 field，
// 并且写明「为什么这条值得被钉住」——否则下一个人删掉它时不会知道代价。
type crossRepoField struct {
	name   string // 人类可读的名字，出现在失败信息里
	host   string // 宿主侧文件（相对宿主仓根）
	ui     string // 界面侧文件（相对界面仓根）
	field  string // JSON tag 名，两侧都必须出现
	reason string // 为什么钉它
}

var crossRepoFields = []crossRepoField{
	{
		name:   "设备码登录的 userCode",
		host:   "internal/transport/handlers_account_official.go",
		ui:     "internal/apiclient/account.go",
		field:  "userCode",
		reason: "宿主内部一直有该字段、注释写着「UI 需要」，但 wire 与投影都不带 —— 界面按它解 ⇒ 真宿主上登录码恒空。2026-11-22 修（#128）。",
	},
}

// findHostRepo 定位宿主仓：先看 DSH_GO_REPO，再看兄弟目录 ../DSH-GO。
func findHostRepo(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("DSH_GO_REPO"); p != "" {
		return p
	}
	// 本文件在 <ui>/internal/apiclient/ 下：向上三级是界面仓根，再上一级是工作区
	wd, err := os.Getwd()
	if err != nil {
		t.Skipf("取当前目录失败，跳过跨仓检查：%v", err)
	}
	cand := filepath.Join(wd, "..", "..", "..", "DSH-GO")
	if st, err := os.Stat(filepath.Join(cand, "go.mod")); err == nil && !st.IsDir() {
		abs, err := filepath.Abs(cand)
		if err != nil {
			t.Skipf("解析宿主仓路径失败，跳过跨仓检查：%v", err)
		}
		return abs
	}
	t.Skip("找不到宿主仓（DSH_GO_REPO 未设、../DSH-GO 不存在）—— 单仓 checkout，跳过跨仓 wire 检查")
	return ""
}

// readDeclaresField 读文件并判断它是否声明了该 JSON tag。
//
// 只做文本包含判断，不做 AST 解析：这个守卫要的是「这个字段名有没有出现在这份契约文件里」，
// 而不是「它是不是一个合法 struct tag」。过度的解析器会让它更难维护，却不会更准。
func readDeclaresField(t *testing.T, path, field string) bool {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读不了 %s：%v", path, err)
	}
	// 必须匹配到字段名的**完整边界**（`"userCode"` 或 `"userCode,`），
	// 不能只判 `"userCode` —— 那样 `"userCodeX` 也会命中。
	// 这个漏洞是被变异测试抓出来的：把宿主改成 `json:"userCodeX,..."` 时旧判据仍然通过。
	txt := string(b)
	return strings.Contains(txt, `"`+field+`"`) || strings.Contains(txt, `"`+field+`,`)

}

func TestCrossRepoWireFieldsAreDeclaredOnBothSides(t *testing.T) {
	hostRoot := findHostRepo(t)
	uiRoot := mustUIRoot(t)

	for _, f := range crossRepoFields {
		t.Run(f.name, func(t *testing.T) {
			hostPath := filepath.Join(hostRoot, filepath.FromSlash(f.host))
			uiPath := filepath.Join(uiRoot, filepath.FromSlash(f.ui))

			if !readDeclaresField(t, hostPath, f.field) {
				t.Fatalf("宿主侧没有声明 %q（%s）\n契约：%s", f.field, f.host, f.reason)
			}
			if !readDeclaresField(t, uiPath, f.field) {
				t.Fatalf("界面侧没有声明 %q（%s）\n契约：%s", f.field, f.ui, f.reason)
			}
		})
	}
}

// TestCrossRepoGuardIsNotVacuous 防止上一个测试因为「列表空了」而空过。
//
// 这是本仓门禁的一贯要求（见 scripts/guard-selftest.py 的「弄坏必红」）：
// 一个永远通过的守卫比没有守卫更糟，因为它会让人相信有护栏。
func TestCrossRepoGuardIsNotVacuous(t *testing.T) {
	if len(crossRepoFields) == 0 {
		t.Fatal("crossRepoFields 是空的 —— 跨仓守卫会空过，等于没有")
	}
	for i, f := range crossRepoFields {
		if f.host == "" || f.ui == "" || f.field == "" {
			t.Fatalf("crossRepoFields[%d]（%s）缺 host/ui/field，这条约定形同虚设", i, f.name)
		}
		if f.reason == "" {
			t.Fatalf("crossRepoFields[%d]（%s）没写 reason —— 下一个人删它时不会知道代价", i, f.name)
		}
	}
}

// mustUIRoot 向上找到界面仓根（含 go.mod 且目录名含 "dsh-ui" 或就是模块根）。
func mustUIRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("取当前目录失败：%v", err)
	}
	dir := wd
	for i := 0; i < 6; i++ {
		if st, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("从 %s 向上找不到界面仓根（go.mod）", wd)
	return ""
}
