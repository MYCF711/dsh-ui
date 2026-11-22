package dsconversation

// 官方 CSS 的 font-weight 数值 → viva Box.Weight 归一化档位 的转换守卫。
//
// ## 这个病是什么
//
// 官方 CSS 里 `.runTitle` 是 `font-weight:510`、`.memberButton .memberLabel` 是
// `font-weight:500`。几何表把这两个**官方读数原样**记成常量（那是对的 —— 那张表
// 的规矩就是「实现值必须等于官方值」，常量承担的是**证据**职责）。
//
// 但 viva 的 `Box.Weight` 收的不是 CSS 数值，而是 0..1 的归一化档位
// （`gfx/theme.go`：Regular=0 / Medium=0.15 / Semibold=0.23 / Bold=0.40），
// 且内部映射是「**>= 0.35 即 700**」。
//
// **于是把 510 或 500 直接喂进去，两处都远超 0.35 ⇒ 全都画成 700 的粗体。**
// 用户看到的是加粗的标题与成员名，而官方那里是 medium。
//
// ## 修法为什么放在调用点而不是常量
//
// 常量要保持官方读数（`official_transcript_geometry.go` 的规矩 + 现有几何测试
// 就是拿它跟官方 CSS 逐行核对的）。**单位不一致要在跨越单位边界的那一行解决** ——
// 也就是 `Weight(...)` 这个调用点，用 `cssWeight()` 折一下。

import (
	"os"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// TestCSSWeightMapsToVivaTiers 是转换函数本身的守卫。
func TestCSSWeightMapsToVivaTiers(t *testing.T) {
	cases := []struct {
		css  float32
		want float32
		why  string
	}{
		{400, gfx.WeightRegular, "CSS 400 就是 regular"},
		{500, gfx.WeightMedium, "CSS 500 就是 medium"},
		{510, gfx.WeightMedium, "CSS 510 离 500 比 600 近 —— 这一格正是原先画成 700 的那处"},
		{550, gfx.WeightMedium, "中点仍归 medium"},
		{600, gfx.WeightSemibold, "CSS 600 就是 semibold"},
		{700, gfx.WeightBold, "CSS 700 就是 bold"},
		{800, gfx.WeightBold, ">=700 一律 bold"},
	}
	for _, c := range cases {
		if got := cssWeight(c.css); got != c.want {
			t.Errorf("cssWeight(%v) = %v，应为 %v（%s）", c.css, got, c.want, c.why)
		}
	}
}

// TestCSSWeightNeverProducesBoldForMediumReadings 是本 bug 的**回归守卫**。
//
// 修前：510 与 500 被直接传给 Weight()，而 viva 按 >=0.35 映射成 700 ⇒
// 这两处实际画的全是粗体。修后必须落到 medium。
//
// 断言故意写成「不等于 WeightBold」而不是「等于某个具体值」——
// 这样即使将来 viva 增加档位（比如 0.18），这条守卫仍然表达正确的意图：
// **这两个官方读数不是粗体。**
func TestCSSWeightNeverProducesBoldForMediumReadings(t *testing.T) {
	for _, css := range []float32{500, 510} {
		got := cssWeight(css)
		if got == gfx.WeightBold {
			t.Errorf("cssWeight(%v) = WeightBold —— 官方是 medium 的读数被画成了 700 的粗体（这就是本 bug）", css)
		}
		if got >= 0.35 {
			t.Errorf("cssWeight(%v) = %v，>=0.35 会被 viva 映射成 700", css, got)
		}
	}
}

// TestWorkflowRunWeightsAreConvertedNotRaw 钉住**调用点真的走了转换**。
//
// ## 这条为什么必须是源码级断言
//
// 第一版这条只测了「常量等于官方读数」和「cssWeight(510) < 0.35」—— 把调用点
// 写回 `Weight(WorkflowRunTitleWeight)` 时**它仍然全绿**，因为它根本没看调用点。
// 变异验证抓出来了：那是一份假守卫。
//
// 要证明「调用点用了转换」，只有两条路：① 真画一遍再看像素（贵且脆）；
// ② 直接读源码。**本仓门禁本来就有一批源码级守卫**（`scripts/perm-guard.py`、
// `scripts/text-only-guard.py`），这里沿用同一套做法。
func TestWorkflowRunWeightsAreConvertedNotRaw(t *testing.T) {
	src, err := os.ReadFile("official_workflow_run.go")
	if err != nil {
		t.Fatalf("读不到 official_workflow_run.go：%v", err)
	}
	text := string(src)

	// 两个常量本身必须保持**官方读数**（那是几何表的证据职责）
	if WorkflowRunTitleWeight != 510 {
		t.Errorf("WorkflowRunTitleWeight = %v，应保持官方读数 510（转换在调用点做，不要改常量）", WorkflowRunTitleWeight)
	}
	if WorkflowRunMemberLabelWeight != 500 {
		t.Errorf("WorkflowRunMemberLabelWeight = %v，应保持官方读数 500", WorkflowRunMemberLabelWeight)
	}

	// 调用点：裸传必须为 0，走转换必须 >= 2（title 两处 + memberLabel 一处）
	//
	// **注意 `cssWeight(X)` 本身就以 `Weight(X)` 结尾** —— 用 strings.Count 数裸传
	// 会把正确代码也数进去（这条断言第一版就是这么错的，被自己的变异验证抓到）。
	// 所以要按「前三个字符是不是 css」逐处判断。
	for _, name := range []string{"WorkflowRunTitleWeight", "WorkflowRunMemberLabelWeight"} {
		if n := countBareWeightCalls(text, name); n != 0 {
			t.Errorf("发现 %d 处裸传 Weight(%s) —— CSS 数值必须经 cssWeight() 折档", n, name)
		}
	}
	if total := strings.Count(text, "Weight(cssWeight("); total < 2 {
		t.Errorf("只找到 %d 处 Weight(cssWeight(...))，至少应有 2 处（title 与 memberLabel）", total)
	}
}

// countBareWeightCalls 数「没被 cssWeight 包住的」Weight(X) 调用。
func countBareWeightCalls(src, name string) int {
	needle := "Weight(" + name + ")"
	n := 0
	for i := 0; ; {
		j := strings.Index(src[i:], needle)
		if j < 0 {
			return n
		}
		at := i + j
		// 若前面紧挨着 "css"，那是 cssWeight(...) —— 正确用法，不算裸传
		if at < 3 || src[at-3:at] != "css" {
			n++
		}
		i = at + len(needle)
	}
}
