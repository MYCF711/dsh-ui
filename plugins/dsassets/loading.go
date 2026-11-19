package dsassets

import "github.com/vivaui/viva/gfx"

// loadingOutlinePath 是官方 `loading-outline`（Regular 档）的 `d` 原文。
//
// 出处：安装态 rc.2 `@deepseek-ai/dsh-web-frontend/dist/assets/index-ghOpC9uV.js`
// （sha256 `061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8`、
// 偏移 721120）的 sprite 表条目
//
//	"loading-outline":{size:16,svg:e=>`<svg xmlns="http://www.w3.org/2000/svg"
//	  viewBox="0 0 16 16" fill="none" stroke-width="${e}"><path d="…"
//	  stroke="currentColor"></path></svg>`}
//
// 它的 `${e}` 由同一文件的权重表 `p7={regular:1,medium:pi}` 给：Regular 档 = 1，
// 与 `registry.go` 的 `regularStroke` 同值。整枚图标**只有一条 path、没有
// stroke-linecap / stroke-linejoin**，所以按 SVG 1.1 §11.4/§11.5 取默认的
// butt 线帽与 miter 线接——正好是 `gfx.Icon` 的零值，不需要 `CapSet`。
//
// 形状（可复算，测试按这两条判）：以 (8,8) 为圆心、半径 6.5 的圆弧，
// 从 45°（右下）顺时针画 270° 到 315°（右上），缺口是正东那 90°；
// 8 段三次贝塞尔，每段 33.75°。
const loadingOutlinePath = `M12.596 12.596C11.687 13.5049 10.5288 14.1239 9.26798 14.3747C8.00716 14.6255 6.70028 14.4968 5.51261 14.0048C4.32494 13.5129 3.30981 12.6798 2.59557 11.611C1.88134 10.5421 1.50008 9.2855 1.5 7.99998C1.50008 6.71446 1.88134 5.45783 2.59557 4.38898C3.30981 3.32013 4.32494 2.48707 5.51261 1.99513C6.70028 1.50319 8.00716 1.37447 9.26798 1.62524C10.5288 1.87602 11.687 2.49502 12.596 3.40398`

// LoadingOutline 返回官方 `IconLoadingOutlineRegular` 的 gfx.Icon。
//
// 官方在 cot-translation 的 pending 态画的就是它：`<IconLoadingOutlineRegular
// size={14}/>`（`@deepseek-ai/dsh-experimental-client-ui-cot-translation`
// `src/client/TranslationBody.tsx:55`），外面套
// `.spinner{animation:cot-translation-spin 1s linear infinite}`
// （同包 `src/client/Translation.module.css:17-18`）自己转圈——
// **图标本身不带旋转**，转不转、转到哪一帧由调用方决定（本仓用
// `ui.Box.IconTransform`，见 `plugins/dsconversation/cot_translation.go`）。
//
// 与 `market.go` 同一档：**不登记进 `All()`**（登记表那条 golden 基线要求同步
// `docs/official-assets.md` 的素材表），按需接线。`fill` 对纯描边图标不参与绘制，
// 只为与 `Build` 的签名一致。
func LoadingOutline(fill gfx.Color) (gfx.Icon, bool) {
	return Build(Asset{
		ID:    "loading-outline",
		Title: "加载中",
		// Size 取官方这处调用点的 size={14}；同一枚图标在别处也有 16 的用法，
		// 显示尺寸一律由调用方给，这里只记本仓唯一使用点的官方值。
		ViewBox: 16, Size: 14, StrokeWidth: regularStroke,
		Elements: []Element{{Kind: PathElement, D: loadingOutlinePath, Stroke: true}},
	}, fill)
}
