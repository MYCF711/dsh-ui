package dscore

// 这个文件是「每一处浮层用哪一档」的**决策表**，用测试钉住。
//
// # 为什么是表
//
// 官方不是"浮层都用同一个阴影"：菜单面是 `elevation-prominent`、通知卡是
// `elevation-panel`、输入卡片是 `elevation-soft`、快捷键面板干脆是
// `--dsw-shadow-lv3`（不是三件套），描边色也按各处 `--dsw-elevation-stroke-color`
// 的重绑走 l1/l2/l4/l2-darkmode-thin 四种。全表与逐条 file:line 在
// `docs/official-elevation-tokens-evidence-rc2.md`。
//
// 一刀切成同一档在画面上是好几个一眼可见的错，而单看某个包自己的测试是绿的
// （它只证明"我这一处画了两层"）。所以这张表放在令牌包里：改任何一处的档位或
// 描边色，这里点名报出是**哪个文件**变了。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// elevationSite 是一处消费方的期望：用哪一档、哪一圈描边。
//
// stroke 为空串 = 该处不传 `t.Border*` 而是自定义（例如 dsattachment 的
// `border-l2-darkmode-thin`，`attachmentArrowBorder` 自己按亮/暗算）。
type elevationSite struct {
	shadow string // Shadows.* 字段名
	stroke string // Border* 字段名，或 ""
}

// elevationSites 是 task-80 的 18 处（13 处预设 + 5 处手抄副本）的决策表。
// 官方依据逐条在 docs/official-elevation-tokens-evidence-rc2.md §10。
var elevationSites = map[string]elevationSite{
	// —— 13 处原本用 viva 预设的 ——
	"dsagentteam/catalog.go":               {"Prominent", "BorderL1"}, // TeamAction .panel / 菜单原语
	"dsagentteam/confirm.go":               {"Prominent", "BorderL4"}, // 对话框原语（未重绑描边）
	"dscomposer/composer.go":               {"Soft", "BorderL2"},      // InputBar .card
	"dsconversation/feedback.go":           {"Prominent", "BorderL4"}, // 对话框原语
	"dscordis/panel.go":                    {"Prominent", "BorderL1"}, // CordisPanel .panel
	"dsjobs/official_jobs.go":              {"Prominent", "BorderL1"}, // JobListAction .menu
	"dssettings/official_panel.go":         {"Prominent", "BorderL4"}, // SettingsRoot .panel
	"dssettings/settings.go":               {"Prominent", "BorderL4"}, // 同设置面（rc.1 兼容桥）
	"dsshortcuts/shortcuts.go":             {"Lv3", ""},               // Reference .dialog（lv3 无描边层）
	"dssidebar/workspace_actions.go":       {"Prominent", "BorderL4"}, // 对话框原语
	"dsstatusbar/official_stats_dialog.go": {"Prominent", "BorderL1"}, // stat-dialog .panel
	"dsterminal/guide.go":                  {"Prominent", "BorderL1"}, // 菜单原语（走 PaintMenuPanel）

	// —— 共享画笔自己那一处 ——
	// `PaintMenuPanel` 是"菜单原语的面"的唯一实现（forge 范围菜单与终端 shell 菜单
	// 都走它）。它的档位必须在这一行钉住，否则上面的 `dsterminal/guide.go` 与
	// `dsforge` 那两处调用就等于没有判据。
	"dscore/menu.go": {"Prominent", "BorderL1"}, // 菜单原语 Menu.module.css 面

	// —— 5 处手抄了同一套数值的（改成令牌，数值不变）——
	"dsattachment/rail.go":          {"Panel", ""},             // AttachmentRail .arrow（l2-darkmode-thin）
	"dscontextmenu/context_menu.go": {"Prominent", "BorderL1"}, // MenuView .menu / 菜单原语
	"dsdirpicker/browse.go":         {"Prominent", "BorderL4"}, // 对话框原语
	"dsgoal/official_seat.go":       {"Panel", "BorderL1"},     // GoalBar .bar
	"dsquestions/plan_review.go":    {"Panel", "BorderL2"},     // PlanReviewPanel .card
}

// sharedShadowPainters 是「一把画完一处浮层」的共享画笔：调用方只说"画菜单面"，
// 具体档位由实现文件决定，**不许**在调用点再写一遍档次。
//
// 值是那一档在 elevationSites 里的键（= 实现文件），所以档位只有一份读数：
// 改 `PaintMenuPanel` 里的 Shadows.X，`dscore/menu.go` 那一行立刻红。
var sharedShadowPainters = map[string]string{
	"PaintMenuPanel": "dscore/menu.go",
}

// TestElevationDecisionTablePerSite 扫源码，逐处核对「档位 + 描边色」。
//
// 扫的是 `dscore.PaintElevation(d, r, t.Shadows.X, t.BorderY)` 这一类调用的
// 实参名（`dsattachment` 的描边色是函数调用，因此只核对档位）。
func TestElevationDecisionTablePerSite(t *testing.T) {
	root := filepath.Join("..", "..", "plugins")
	fset := token.NewFileSet()

	for rel, want := range elevationSites {
		path := filepath.Join(root, filepath.FromSlash(rel))
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", rel, err)
		}
		var found []elevationSite
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			// 同包内的直接调用：`dscore/menu.go` 这类**实现文件**自己就在这个包里，
			// 它写的是 `PaintElevationOverlay(...)` 而不是 `dscore.PaintElevationOverlay(...)`。
			if ident, ok := call.Fun.(*ast.Ident); ok {
				if strings.HasPrefix(ident.Name, "PaintElevation") ||
					strings.HasPrefix(ident.Name, "PaintShadow") {
					found = append(found, siteOfCall(call))
				}
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "dscore" {
				return true
			}
			switch sel.Sel.Name {
			case "PaintElevation", "PaintElevationOverlay", "PaintShadow", "PaintShadowOverlay":
			default:
				// 共享画笔：档位取它实现文件在表里的那一行（单一读数）。
				key, shared := sharedShadowPainters[sel.Sel.Name]
				if !shared {
					return true
				}
				want, ok := elevationSites[key]
				if !ok {
					t.Errorf("%s: 共享画笔 dscore.%s 的档位没有登记（缺 %q 这一行）",
						rel, sel.Sel.Name, key)
					return true
				}
				found = append(found, want)
				return true
			}
			found = append(found, siteOfCall(call))
			return true
		})
		if len(found) == 0 {
			t.Errorf("%s: 没找到 dscore.PaintElevation/PaintShadow 调用——"+
				"这一处的阴影被删掉了（等于没画）", rel)
			continue
		}
		matched := false
		for _, got := range found {
			if got.shadow != want.shadow {
				continue
			}
			if want.stroke != "" && got.stroke != want.stroke {
				continue
			}
			matched = true
		}
		if !matched {
			t.Errorf("%s: 期望 Shadows.%s（描边 %q），实际调用 %+v。"+
				"档位/描边色的官方依据见 docs/official-elevation-tokens-evidence-rc2.md §10；"+
				"改这一处必须先改那张表", rel, want.shadow, want.stroke, found)
		}
	}
}

// siteOfCall 从一次 Paint* 调用的实参里读出 `Shadows.X` 与描边色字段名。
func siteOfCall(call *ast.CallExpr) elevationSite {
	var out elevationSite
	for _, arg := range call.Args {
		switch v := arg.(type) {
		case *ast.SliceExpr:
			// `t.Shadows.Lv3[:]`：切片表达式包着一层选择表达式。
			if inner, ok := v.X.(*ast.SelectorExpr); ok {
				if sel, ok := inner.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Shadows" {
					out.shadow = inner.Sel.Name
				}
			}
		case *ast.SelectorExpr:
			// t.Shadows.X：X 是 SelectorExpr 的 Sel，外层 X 是 t.Shadows
			if inner, ok := v.X.(*ast.SelectorExpr); ok {
				if inner.Sel.Name == "Shadows" {
					out.shadow = v.Sel.Name
				}
				continue
			}
			// t.BorderL1
			if id, ok := v.X.(*ast.Ident); ok && id.Name == "t" {
				out.stroke = v.Sel.Name
			}
		}
	}
	return out
}
