//go:build darwin || windows || linux

package main

// visual_regions_contract_test.go —— 视觉判据的**区域契约**守卫。
//
// # 这个文件守什么
//
// `scripts/visual-regions.py` 按 `testdata/visual-v06/regions.json` 里的
// 显式矩形裁图比像素。矩形一旦和真实布局分叉，那个脚本就会**拿错的区域比像素**：
// 区域偏了 10px，它会照样报 PASS（只是比的不是那个部件），而人看不出来。
//
// 所以这里把 JSON 和 `uikernel.ComputeShellLayout` 的真实几何**对拍**：
// 每个区域的矩形必须**逐像素等于**骨架算出来的那个容器。骨架改了、JSON 没跟上 → 红。
//
// # 为什么是 ComputeShellLayout 而不是截图
//
// 截图里的几何是"渲染结果"，会受字体、DPI、窗口边框影响；`ComputeShellLayout`
// 是壳交给插件的**同一份**几何（`cmd/app/shell.go` 的 `officialGeometry` 就是它），
// 确定性、无环境依赖。这条测试只证明"区域定义与布局表一致"；
// 像素本身由脚本验，两者互不替代。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

// visualRegion 是区域契约里的一条：一个命名矩形 + 它的类别。
type visualRegion struct {
	Name string     `json:"name"`
	Rect [4]float32 `json:"rect"` // [x, y, width, height]，左上原点、左闭右开
	Kind string     `json:"kind"` // structural / content / full
	Note string     `json:"note"`
}

// visualState 是一个产品形态：一份全新进程 + 一组参数 + 一个区域集。
type visualState struct {
	Name      string   `json:"name"`
	Regions   string   `json:"regions"`
	Size      string   `json:"size"` // "720x600"；空 = 用 window.size
	NoSession bool     `json:"no_session"`
	Host      string   `json:"host"` // "default" / "empty"
	Args      []string `json:"args"`
	Note      string   `json:"note"`
}

// visualAccepted 是一条**有上限的例外**：已归因的上游差异。
type visualAccepted struct {
	MaxPercent      float64 `json:"max_percent"`
	MeasuredPercent float64 `json:"measured_percent"`
	Attribution     string  `json:"attribution"`
	Reason          string  `json:"reason"`
}

// visualContract 是 regions.json 的机读形状。
type visualContract struct {
	SchemaVersion int      `json:"schema_version"`
	Comment       []string `json:"comment"`
	Window        struct {
		Width  float32 `json:"width"`
		Height float32 `json:"height"`
		Size   string  `json:"size"`
	} `json:"window"`
	Thresholds map[string]float64        `json:"thresholds"`
	RegionSets map[string][]visualRegion `json:"region_sets"`
	States     []visualState             `json:"states"`
	Accepted   map[string]visualAccepted `json:"accepted"`
}

// 路径相对**包目录**（`go test` 的工作目录是 cmd/app），与 api.snapshot.json 同一套。
const visualContractPath = "../../testdata/visual-v06/regions.json"

// visualBaselineDir 是 v0.5.1（迁移前）的真机截图基线。
const visualBaselineDir = "../../testdata/visual-v06/baseline"

// 区域类别 → 阈值键。类别是**语义**（骨架 / 正文 / 整窗），不是区域名，
// 这样加一个区域时只需要选类别，不用另想一个数。
var visualRegionKinds = map[string]string{
	"structural": "structural_percent",
	"content":    "content_percent",
	"full":       "full_percent",
}

func loadVisualContract(t *testing.T) visualContract {
	t.Helper()
	raw, err := os.ReadFile(visualContractPath)
	if err != nil {
		t.Fatalf("读不了区域契约 %s：%v", visualContractPath, err)
	}
	var contract visualContract
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatalf("%s 不是合法 JSON：%v", visualContractPath, err)
	}
	return contract
}

// windowOf 解出某个形态的窗口尺寸（形态自己的 size 优先）。
func windowOf(t *testing.T, contract visualContract, state visualState) (float32, float32) {
	t.Helper()
	if state.Size == "" {
		return contract.Window.Width, contract.Window.Height
	}
	parts := strings.SplitN(state.Size, "x", 2)
	if len(parts) != 2 {
		t.Fatalf("形态 %q 的 size %q 不是 WxH", state.Name, state.Size)
	}
	width, err := strconv.ParseFloat(parts[0], 32)
	if err != nil {
		t.Fatalf("形态 %q 的宽度 %q 解不了：%v", state.Name, parts[0], err)
	}
	height, err := strconv.ParseFloat(parts[1], 32)
	if err != nil {
		t.Fatalf("形态 %q 的高度 %q 解不了：%v", state.Name, parts[1], err)
	}
	return float32(width), float32(height)
}

// visualLayoutOptions 复刻 `cmd/app/shell.go` 的 `shellOptionsFor`
// 在**无头截图**那一档下的取值（Windows 上 CollapsedWidth 是 0）。
func visualLayoutOptions(state visualState) uikernel.ShellOptions {
	opts := uikernel.ShellOptions{
		Phase:            uikernel.PhaseActive,
		SidebarPreferred: uikernel.SidebarDefault,
		// Windows 下官方侧栏收起后整列消失（rail 0），见 shellOptionsFor。
		CollapsedWidth: 0,
		Blank:          state.NoSession,
		BottomHeight:   0,
		// 旧部件（dstopbar 的 main.header、dsconversation 的 main.body）还在，
		// 整列兼容桥开着——与 hits 矩阵同一档。
		BridgeLegacy: true,
	}
	if state.NoSession {
		opts.Phase = uikernel.PhaseHero
	}
	for _, arg := range state.Args {
		if arg == "-rightbar" {
			opts.RightbarOn = true
		}
	}
	return opts
}

// visualRegionRect 把区域名映射到它应该**正好盖住**的骨架容器。
//
// 新增区域时必须在这里登记——没有登记的名字会让测试红，而不是静默跳过
// （静默跳过等于那个区域永远没人验，和删掉它一样）。
func visualRegionRect(t *testing.T, name string, layout uikernel.ShellLayout) [4]float32 {
	t.Helper()
	switch name {
	case "full":
		return [4]float32{layout.Frame.X, layout.Frame.Y, layout.Frame.Width, layout.Frame.Height}
	case "sidebar":
		return [4]float32{layout.SidebarCol.X, layout.SidebarCol.Y, layout.SidebarCol.Width, layout.SidebarCol.Height}
	case "header":
		return [4]float32{layout.Header.X, layout.Header.Y, layout.Header.Width, layout.Header.Height}
	case "transcript":
		return [4]float32{layout.ScrollBody.X, layout.ScrollBody.Y, layout.ScrollBody.Width, layout.ScrollBody.Height}
	case "composer":
		return [4]float32{layout.ComposerSeat.X, layout.ComposerSeat.Y, layout.ComposerSeat.Width, layout.ComposerSeat.Height}
	case "rightbar":
		return [4]float32{layout.RightbarCol.X, layout.RightbarCol.Y, layout.RightbarCol.Width, layout.RightbarCol.Height}
	default:
		t.Fatalf("区域名 %q 没有登记到骨架容器（visualRegionRect）——新增区域必须在这里映射", name)
		return [4]float32{}
	}
}

// TestVisualRegionsMatchShellLayout 是**判据①**：每个区域的矩形都等于骨架算出来的容器。
func TestVisualRegionsMatchShellLayout(t *testing.T) {
	contract := loadVisualContract(t)
	if contract.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d，want 1", contract.SchemaVersion)
	}
	if len(contract.States) < 8 {
		t.Fatalf("形态只有 %d 个（< 8）：视觉判据的覆盖面退化了", len(contract.States))
	}
	for _, state := range contract.States {
		state := state
		t.Run(state.Name, func(t *testing.T) {
			regions, ok := contract.RegionSets[state.Regions]
			if !ok {
				t.Fatalf("区域集 %q 不存在", state.Regions)
			}
			width, height := windowOf(t, contract, state)
			layout := uikernel.ComputeShellLayout(width, height, visualLayoutOptions(state))
			seen := map[string]bool{}
			for _, region := range regions {
				if seen[region.Name] {
					t.Fatalf("区域 %q 在同一区域集里出现两次", region.Name)
				}
				seen[region.Name] = true
				if region.Rect[2] <= 0 || region.Rect[3] <= 0 {
					t.Fatalf("区域 %q 零面积：%v", region.Name, region.Rect)
				}
				if _, ok := visualRegionKinds[region.Kind]; !ok {
					t.Fatalf("区域 %q 的类别 %q 没有阈值（visualRegionKinds）", region.Name, region.Kind)
				}
				want := visualRegionRect(t, region.Name, layout)
				if region.Rect != want {
					t.Fatalf("区域 %q 的矩形 %v 与骨架算出来的 %v 不一致——布局改了，regions.json 没跟上",
						region.Name, region.Rect, want)
				}
			}
		})
	}
}

// TestVisualBaselinesExist 是**判据②**：每个形态都有一张基线图（v0.5.1 的真机截图）。
func TestVisualBaselinesExist(t *testing.T) {
	contract := loadVisualContract(t)
	for _, state := range contract.States {
		state := state
		t.Run(state.Name, func(t *testing.T) {
			path := filepath.Join(visualBaselineDir, state.Name+".png")
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("基线缺图 %s：%v", path, err)
			}
			if info.Size() == 0 {
				t.Fatalf("基线图 %s 是空文件", path)
			}
		})
	}
}

// TestVisualThresholdsAreExplicit 是**判据③**：阈值必须写全，且 accepted 表只能收紧。
func TestVisualThresholdsAreExplicit(t *testing.T) {
	contract := loadVisualContract(t)
	for _, key := range []string{"structural_percent", "content_percent", "full_percent"} {
		value, ok := contract.Thresholds[key]
		if !ok {
			t.Fatalf("阈值 %q 没写", key)
		}
		if value <= 0 || value > 10 {
			t.Fatalf("阈值 %q = %v 不像一个判据（0 < x ≤ 10）", key, value)
		}
	}
	// accepted 是**有上限的例外**，不是"把阈值调大"：每条必须
	// ① 指向真实存在的 state|region；② 带上界；③ 上界 ≥ 实测；④ 带归因与理由。
	byName := map[string]visualState{}
	for _, state := range contract.States {
		byName[state.Name] = state
	}
	for key, entry := range contract.Accepted {
		stateName, regionName, ok := strings.Cut(key, "|")
		if !ok || stateName == "" || regionName == "" {
			t.Fatalf("accepted 键 %q 不是 <state>|<region>", key)
		}
		state, ok := byName[stateName]
		if !ok {
			t.Fatalf("accepted 键 %q 引用了不存在的形态", key)
		}
		regions, ok := contract.RegionSets[state.Regions]
		if !ok {
			t.Fatalf("accepted 键 %q 的形态没有区域集", key)
		}
		found := false
		for _, region := range regions {
			if region.Name == regionName {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("accepted 键 %q 引用了不存在的区域", key)
		}
		if entry.MaxPercent <= 0 {
			t.Fatalf("accepted 键 %q 没有上界", key)
		}
		if entry.MaxPercent < entry.MeasuredPercent {
			t.Fatalf("accepted 键 %q 的上界 %v%% 小于实测 %v%%——上界必须 ≥ 实测",
				key, entry.MaxPercent, entry.MeasuredPercent)
		}
		if entry.Reason == "" || entry.Attribution == "" {
			t.Fatalf("accepted 键 %q 缺归因或理由", key)
		}
	}
}
