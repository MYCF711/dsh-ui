// Package dsassets 是**官方素材的唯一来源**：官方安装包里的 SVG 矢量（`d` 原文、
// viewBox、描边宽度、线帽/线接）连同它们的出处（npm 包 / 包内文件 / 符号名）一起
// 存在这里，插件不再各自手绘近似形状。
//
// # 为什么要有这个包
//
// 在这之前，同一个图标在不同插件里各画一份：发送键在 `dscomposer/icons.go` 里
// 逐点抄过一次，拇指在 `dsconversation/feedback.go` 里是**凭空画的**（连 👎 都是
// 👍 的镜像，而官方两枚是两条完全不同的路径）。抄一次没人能证明它还是官方的：
// 数字少一位、控制点顺序颠倒，肉眼都看不出来。
//
// 所以这里的规则只有三条：
//
//  1. `paths.go` 里的 `d` 是官方原文，一个字节都不改（`d` 里有多少空格、多少位小数
//     都保持原样）。`golden_test.go` 每次跑测试都拿 `testdata/official/` 里的原文
//     逐字节比对，并且记录官方文件的 sha256。
//  2. 能不能画，由 `Build` 明确回答。官方素材里 `<rect>` / `<circle>` 这类图元需要
//     椭圆弧（SVG `A`）才能精确表达，而 viva 的路径词汇只有 M/L/Q/C/Z——遇到这种
//     素材 `Build` 返回 false，**调用方必须自己决定退回什么**，本包绝不悄悄用折线
//     或贝塞尔去"差不多"地替代。
//  3. 解析只有一份实现：`ParsePath` 先把官方用到的 H/V/S 与相对命令**无损**展开成
//     绝对 M/L/C/Z，再交给 `dsbrand.ParseSVGPath`。全仓只有那一个路径解析器。
//
// # 怎么刷新
//
// 素材来自 `D:/DeepSeek Harness/Desktop/resources/app.asar`。刷新步骤见
// `docs/official-assets.md`「刷新流程」：脚本只读 asar、只写 `tmp/`，
// 人工核对后把新原文贴进 `paths.go`，golden 测试会把差异喊出来。
package dsassets
