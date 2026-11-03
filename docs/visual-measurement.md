# 确定性截图 / 几何测量

- 任务书 base UI：`f007fad`；本 owner 未 commit、未 push、未改产品代码、`docs/progress.md` 或 UIPLAN。
- 实现：[`scripts/visual-diff.py`](../scripts/visual-diff.py)，只使用系统现有 Python 3.13.15 + Pillow 12.3.0；未安装依赖。
- 证据目录：[`tmp/visual-measurement/`](../tmp/visual-measurement/)。
- 测量不使用 OCR、模型识图或语义猜测。区域、坐标和阈值由调用者显式给出；当前版本的逐像素规则是 **RGBA 任一通道不同即不同，容差为 0**。

## 1. 旧 `tmp/official-*.png` 来源审计

文件名不是来源证明。逐张检查了已有脚本/文档记录、PNG metadata、尺寸、mtime、SHA-256 和窗口/进程证据；完整机读结果见 [`source-audit.json`](../tmp/visual-measurement/source-audit.json)。

| 文件 | 尺寸 | PNG `info` | 审计结论 | 依据 |
|---|---:|---|---|---|
| `tmp/official-final.png` | 1196×839 | `{}` | **本仓原生 UI，不是官方 Electron 参考图** | [`docs/official-specs.md:110`](official-specs.md#L110) 明确把它称为原生修复后的“修后”图；[`docs/progress.md:152-154`](progress.md#L152-L154) 同时记录原生 shell/sidebar 修改和相同像素几何 |
| `tmp/official-sidebar-top.png` | 750×330 | `{}` | **unknown** | 无捕获命令、日志、窗口标题、进程或文档链证明来源 |
| `tmp/official-sidebar.png` | 1196×839 | `{}` | **unknown** | 与同时段原生图同尺寸不是来源证明；无捕获链或 metadata |
| `tmp/official-sidebar-rows.png` | 1120×380 | `{}` | **unknown** | 无原始父图、裁剪命令或官方窗口证据 |
| `tmp/official-statusbar.png` | 2392×88 | `{}` | **unknown** | 无原始父图、裁剪/缩放命令或官方窗口证据 |

因此，**没有任何一张旧 `tmp/official-*.png` 被接纳为官方 Electron 对拍参考图**。`official-final.png` 有反证表明它是本仓原生 UI；另外四张不能证明来源，保持 `unknown`，不按文件名猜测。

## 2. 当前官方窗口的只读捕获

开工时只读枚举到一个已经存在的可见窗口：

- pid：`18824`；hwnd：`66882`。
- 进程：`D:\DeepSeek Harness\Desktop\DeepSeek Harness.exe`。
- 捕获时标题：`提取 Monkey Code 代理供应商 — DeepSeek Harness`。
- 图像：[`official-current.png`](../tmp/visual-measurement/official-current.png)，1920×1032，SHA-256 `bf5d96fee4bf71c18fb1acc2158d4280f11e6a1c111615e858d87a0625ee2d52`。
- 元数据：[`official-current.meta.json`](../tmp/visual-measurement/official-current.meta.json)。`PrintWindow(PW_RENDERFULLCONTENT=2)` 返回 `true`，`window_operations=[]`，图中共有 2702 个 RGB 颜色，非空图。

捕获脚本为 [`capture_official_readonly.py`](../tmp/visual-measurement/capture_official_readonly.py)。它只枚举、查询矩形并调用 `PrintWindow`；没有 `ShowWindow`、`SetForegroundWindow`、`SetWindowPos`、输入注入、启动或关闭进程。没有新建第二实例，也没有操作现有 GUI。

第一次捕获退出 **1**，因为只读复查发现窗口标题已被外部会话改变，精确标题匹配拒绝捕获，日志见 [`capture-official.log`](../tmp/visual-measurement/capture-official.log)。重新只读枚举当前标题后，第二次退出 **0**，日志见 [`capture-official-attempt2.log`](../tmp/visual-measurement/capture-official-attempt2.log)：

```bash
python tmp/visual-measurement/capture_official_readonly.py \
  --pid 18824 \
  --title '提取 Monkey Code 代理供应商 — DeepSeek Harness' \
  --exe 'D:\DeepSeek Harness\Desktop\DeepSeek Harness.exe' \
  --out tmp/visual-measurement/official-current.png \
  --metadata tmp/visual-measurement/official-current.meta.json \
  > tmp/visual-measurement/capture-official-attempt2.log 2>&1
# exit 0
```

证据边界：这张图只证明 **捕获时该官方进程的该窗口像素**。标题显示它处于具体会话，不是已证明的官方首屏同状态样本。**同状态官方首屏：没有参考图。** 不得拿这张当前会话图冒充首屏基线，也不得用它反推旧 `official-*.png` 的来源。

## 3. 工具用法与区域契约

```bash
python scripts/visual-diff.py A.png B.png \
  --regions regions.json --out result.json
```

- A、B 必须同尺寸；尺寸不同退出 **2**，不输出误导性的重叠区 diff。
- 找到差异仍退出 **0**：这是测量工具，不把“不同”自动解释成测试失败。
- 图像先转为 RGBA；颜色数、主色、diff 都包含 alpha。
- 区域采用 `[x,y,width,height]`，原点在左上，矩形左闭右开。
- 全图始终输出 `comparison`；每个命名区域另输出 A 测量、B 测量和区域 diff。

本次完整区域文件为 [`regions-1920x1032.json`](../tmp/visual-measurement/regions-1920x1032.json)：

```json
{
  "schema_version": 1,
  "top_colors": 8,
  "regions": [
    {"name": "full", "rect": [0, 0, 1920, 1032]},
    {"name": "sidebar_formula_280", "rect": [0, 0, 280, 1008]},
    {"name": "main_formula_776", "rect": [280, 0, 776, 1008]},
    {"name": "bottom_24", "rect": [0, 1008, 1920, 24]}
  ]
}
```

区域名称只是调用者给的标签；工具不会根据名称推断位置。上述 `sidebar_formula_280` 等矩形来自任务既有几何公式，**不是 OCR 或模型从图中识别的结果**。

### 输出字段

| 字段 | 确定性含义 |
|---|---|
| `inputs.a/b` | 路径、原始格式/mode、尺寸、文件 SHA-256 |
| `comparison` | 全图逐像素 diff |
| `regions[].a/b.size` | 裁剪后的尺寸 |
| `color_count_rgba` | 区域中不同 RGBA 颜色的精确数量 |
| `dominant_colors` | 按计数降序排列的主色；同数时按 RGBA 字典序，含 count/ratio |
| `dominant_bands.x` | 每一列的精确众数颜色，相邻同众数列合并为垂直色带 |
| `dominant_bands.y` | 每一行的精确众数颜色，相邻同众数行合并为水平色带 |
| `boundaries` | 色带起点列表；`start/end` 左闭右开，并附每条带的众数占比范围 |
| `different_pixels` | RGBA 任一通道不同的像素数，无容差 |
| `difference_ratio/percent` | `different_pixels / pixel_count` 的比例/百分比 |
| `bbox_ltrb/bbox_xywh` | 差异包围盒；区域内结果是**区域局部坐标** |
| `image_bbox_ltrb/image_bbox_xywh` | 同一差异包围盒换算到**全图坐标** |
| `max_channel_delta` | 所有像素、所有 RGBA 通道的最大绝对差 |
| `mean_absolute_channel_delta` | 四通道绝对差总和除以 `像素数×4` |

“主色带”是逐行/逐列众数的游程编码，**不是语义容器识别**。文字、图标、滚动内容、遮挡和抗锯齿都会影响众数；需要判断容器几何时，必须同时有调用者给定的区域/期望色值或其他布局证据。

## 4. 已有同尺寸图的实际样例

样例输入：

- A：本次只读捕获的 [`official-current.png`](../tmp/visual-measurement/official-current.png)，1920×1032。
- B：已有的 [`tmp/v4a/app-1920x1032.png`](../tmp/v4a/app-1920x1032.png)，1920×1032；其历史捕获方式与版本边界见 [`docs/visual-v4a.md`](visual-v4a.md#L64-L92)，它是本仓原生 app + mockd/demo 图，不是真宿主功能证据。
- 输出：[`sample-official-vs-native.json`](../tmp/visual-measurement/sample-official-vs-native.json)。
- 日志：[`sample.log`](../tmp/visual-measurement/sample.log)。

```bash
python scripts/visual-diff.py \
  tmp/visual-measurement/official-current.png \
  tmp/v4a/app-1920x1032.png \
  --regions tmp/visual-measurement/regions-1920x1032.json \
  --out tmp/visual-measurement/sample-official-vs-native.json \
  > tmp/visual-measurement/sample.log 2>&1
# exit 0
```

### 数值结果

| 区域 | 不同像素 | 差异比例 | 全图 bbox `[x,y,w,h]` |
|---|---:|---:|---|
| full | 1,156,345 / 1,981,440 | 58.358819848% | `[0,8,1920,1024]` |
| sidebar_formula_280 | 72,118 / 282,240 | 25.552012472% | `[0,8,280,1000]` |
| main_formula_776 | 374,224 / 782,208 | 47.842006218% | `[280,8,776,1000]` |
| bottom_24 | 25,797 / 46,080 | 55.983072917% | `[0,1008,1920,24]` |

全图 A/B 分别有 **2702 / 1194** 个 RGBA 颜色。A 的前三个主色为 `[21,21,23,255]`（48.7365%）、`[27,27,28,255]`（31.1529%）、`[44,44,46,255]`（12.3089%）；B 为 `[27,27,28,255]`（53.8380%）、`[21,21,23,255]`（28.8135%）、`[44,44,46,255]`（8.6908%）。

全图 `dominant_bands.x` 给出的垂直众数色带：

| 图像 | 色带 `[start,end)` / RGBA |
|---|---|
| 官方当前图 A | `[0,8)` `[32,32,32,255]`；`[8,288)` `[27,27,28,255]`；`[288,1920)` `[21,21,23,255]` |
| 原生历史图 B | `[0,8)` `[0,0,0,255]`；`[8,288)` `[27,27,28,255]`；`[288,1055)` `[21,21,23,255]`；`[1055,1912)` `[27,27,28,255]`；`[1912,1920)` `[0,0,0,255]` |

这能直接证明：两张图都具有 `[8,288)` 的 280px 同色众数带；B 还多出 `[1055,1912)` 的 857px 众数带，而 A 的对应范围众数不同。它**不能仅凭色带证明这些范围就是完整 sidebar/rightbar 容器**，更不能证明控件行为存在或等价。

## 5. 验收命令、退出码与反证

以下命令均在仓库根目录执行，路径均相对仓库根目录。

| 命令 | 退出码 | 输出 / 断言 |
|---|---:|---|
| `python scripts/visual-diff.py --help > tmp/visual-measurement/help.log 2>&1` | **0** | [`help.log`](../tmp/visual-measurement/help.log)，包含用法、位置参数和 `--regions/--out` |
| 上文“已有同尺寸图的实际样例”命令 | **0** | [`sample.log`](../tmp/visual-measurement/sample.log)；JSON 可解析 |
| 下文“生成单像素夹具”命令 | **0** | [`one-pixel-fixture.log`](../tmp/visual-measurement/one-pixel-fixture.log) |
| 下文“单像素 diff”命令 | **0** | [`one-pixel.log`](../tmp/visual-measurement/one-pixel.log)，检出 1 像素 |
| 下文“同图 diff”命令 | **0** | [`self.log`](../tmp/visual-measurement/self.log)，全图和全部区域均为 0 |
| `python tmp/visual-measurement/verify_results.py > tmp/visual-measurement/verify-results.log 2>&1` | **0** | [`verify-results.log`](../tmp/visual-measurement/verify-results.log)，JSON、bbox、区域隔离、色带连续性、来源哈希全部通过 |
| 上文第一次旧标题只读捕获 | **1** | [`capture-official.log`](../tmp/visual-measurement/capture-official.log)，精确标题匹配 0 个窗口，未生成/伪造参考图 |
| 上文第二次当前标题只读捕获 | **0** | [`capture-official-attempt2.log`](../tmp/visual-measurement/capture-official-attempt2.log) |

### 5.1 生成单像素夹具

只在任务专用目录新建 B 的副本，把 `(400,400)` 从 `(21,21,23,255)` 改为 `(20,21,23,255)`；不修改已有截图：

```bash
python -c "from PIL import Image; src='tmp/v4a/app-1920x1032.png'; dst='tmp/visual-measurement/app-1920-one-pixel.png'; im=Image.open(src).convert('RGBA'); p=(400,400); old=im.getpixel(p); new=(old[0]^1,old[1],old[2],old[3]); im.putpixel(p,new); im.save(dst); print('source='+src); print('output='+dst); print('xy='+str(p)); print('old_rgba='+str(old)); print('new_rgba='+str(new))" \
  > tmp/visual-measurement/one-pixel-fixture.log 2>&1
# exit 0
```

### 5.2 单像素 diff

```bash
python scripts/visual-diff.py \
  tmp/v4a/app-1920x1032.png \
  tmp/visual-measurement/app-1920-one-pixel.png \
  --regions tmp/visual-measurement/regions-1920x1032.json \
  --out tmp/visual-measurement/one-pixel-result.json \
  > tmp/visual-measurement/one-pixel.log 2>&1
# exit 0
```

[`one-pixel-result.json`](../tmp/visual-measurement/one-pixel-result.json) 和独立复核共同断言：

- 全图 `different_pixels=1`，`difference_ratio=5.04683462532e-07`。
- 全图 `bbox_xywh=[400,400,1,1]`，`max_channel_delta=1`。
- `main_formula_776` 内局部 bbox 为 `[120,400,1,1]`，换算全图 bbox 为 `[400,400,1,1]`。
- `sidebar_formula_280=0`、`bottom_24=0`，证明区域裁剪没有把区域外变化算进来。
- [`verify_results.py`](../tmp/visual-measurement/verify_results.py) 不调用工具的 diff 实现，另行逐像素扫描两张夹具，确认唯一变化坐标就是 `(400,400)`。

### 5.3 同图 diff

```bash
python scripts/visual-diff.py \
  tmp/v4a/app-1920x1032.png \
  tmp/v4a/app-1920x1032.png \
  --regions tmp/visual-measurement/regions-1920x1032.json \
  --out tmp/visual-measurement/self-result.json \
  > tmp/visual-measurement/self.log 2>&1
# exit 0
```

[`self-result.json`](../tmp/visual-measurement/self-result.json) 的全图与四个区域均为 `different_pixels=0`、`bbox_xywh=null`。

## 6. 能证明 / 不能证明

### 工具能证明

1. 输入文件的字节哈希、解码尺寸和给定裁剪区域的精确像素统计。
2. 在相同尺寸、相同坐标系下，哪些像素不同、差异 bbox、差异像素数/比例和通道差值。
3. 精确 RGBA 颜色数、主色，以及逐行/逐列众数形成的色带和边界。
4. 单像素变化不会被整屏平均或比例阈值吞掉；本任务已用 1 通道只差 1 的像素反证。
5. 对已明确给定的区域，可以稳定复跑并得到机读 JSON，不依赖模型“看图觉得像不像”。

### 工具不能单独证明

1. **整屏 pixel diff 不是功能等价度。** 本样例 58.3588% 不同不能解释成 58.3588% 功能缺失；0% 也不能证明交互、Transport、审批、IME、键盘或可访问性等价。
2. 两张图虽然同为 1920×1032，但 A 是官方当前会话窗口，B 是历史原生 mockd/demo 窗口，状态、内容、右栏开关和捕获版本均未对齐；本样例只演示测量，不是 U1.5 首屏验收。
3. 众数色带不是语义容器边界；相同颜色可能跨越多个容器，文字/抗锯齿也可能切断真实色带。
4. 截图不包含不可见功能、数据真伪、事件顺序、模型输出正确性或真实工具副作用。
5. 旧 `official-*` 文件没有得到追认；当前官方图也没有被冒充成同状态首屏参考。要做首屏几何验收，仍需来源可证、状态对齐、同尺寸/DPI 的官方首屏图和显式区域契约。
