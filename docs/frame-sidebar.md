# FG-2 品牌 / 导航 / 工作区侧栏验收

> 原 owner `8f80db61` 在收尾前失败；实现、测试与两张实图已留在工作树。本文是 Lead 的独立验收记录，不声称原 owner 完成交付。

## 范围与基线

- 基线：`2aea9f3` + FG-2/U3A 工作树。
- 修改路径：`plugins/dssidebar/**`；未修改 `cmd/app`、共享 `pluginapi`、宿主 catalog 或其他插件。
- 本 DONE 只覆盖 **品牌、导航 UI、真实本地路由与能力空态**。插件/自动化任务的正式内容目录仍需 C-PANEL/C-WORKSPACE/扩展 registry，不冒充完整面板功能。

## Lead 复验

```bash
CGO_ENABLED=0 go test ./plugins/dssidebar -count=1  # exit 0
CGO_ENABLED=0 go vet ./plugins/dssidebar            # exit 0
```

测试文件：`frame_test.go`、`frame_visual_test.go`。主要断言：

- `logoRow=40`、`brandIdentity=24`、鱼标 24、品牌间距 8、wordmark 18/24；
- 新会话 38px、上间距 8、下间距 12、文字 14/22/500；点击只调用一次真实 `NewSession` 并返回会话；
- 导航行 36px、gap 2；默认只有 plugins(order0)/schedules(order10) 两条，均有真实 `Draw`；
- 主题鱼/上下文压缩/更多在没有真实扩展 Draw 时不出现；提供真实扩展后才按 order 加入；
- 点击 schedules 后实画“当前宿主未提供任务目录”的能力空态，不修改当前会话、搜索现场或草稿；点击会话行可返回原会话；
- 工作区搜索、折叠/展开与 V2/U3A 行为随整包测试通过。

## 两尺寸真实 Viva 实图

| 图 | 尺寸 | SHA-256 | 自动断言 |
|---|---|---|---|
| `plugins/dssidebar/testdata/frame-1920x1032.png` | 1920×1032 | `693175cc5495e7ddcc6174c11ade5f24f9b1647814f6eb6d1eaebad6fbedc00a` | 品牌/新会话/导航/工作区文字齐全；无扩展项；负责矩形与官方推导偏差≤8px |
| `plugins/dssidebar/testdata/frame-1182x832.png` | 1182×832 | `d14425ef955d7a3f48076a4bdda79ab0e60940127ba0a0efbdcc9924ff4e444a` | 同上；两图均为同一 headless session 缩放并当场截图 |

测试使用 `headless.Session.Resize`、`session.HasText`、`frameBoxes` 实际矩形与 PNG 尺寸断言，不是模型看图或手工示意图。截图只装 dscore+dsshell+dssidebar，主区空白是夹具边界，不冒充完整应用实图。

## 未闭环 / 后续

1. `pluginapi` 尚无正式 PanelDescriptor/activePanel/SelectPanel；当前路由在 dssidebar 内。Lead 后续串行集成公共 panel registry，不允许多个插件各造一套。
2. schedules/plugins 目前只证明路由与真实能力空态；正式 catalog、任务列表和操作需要宿主契约。
3. 工作区只使用 Snapshot 已有数据；目录选择、add/remove、归档/pin/rename 未实现。
4. OS 鼠标、真实焦点、高 DPI、完整应用组合帧未在本包验收。
5. 原 owner 未提交；当前实现/测试/截图仍在工作树。
