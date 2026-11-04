# FG-3 会话 header / 页签 / 右栏验收

> 原 owner `48b92856` 在最终文档收尾前失败；本文是 Lead 独立验收记录。范围仅 `plugins/dstopbar/**`、`plugins/dsrightbar/**` 与 FG-1 已完成的右栏 glue。

## Lead 复验

```bash
CGO_ENABLED=0 go test ./plugins/dstopbar ./plugins/dsrightbar -count=1  # 两包 exit 0
CGO_ENABLED=0 go vet ./plugins/dstopbar ./plugins/dsrightbar            # 两包 exit 0
CGO_ENABLED=0 go test ./cmd/app -count=1                               # exit 0
CGO_ENABLED=0 go build ./...                                           # exit 0
```

## 已核实的行为

- 真实点击 `chat → trajectory → chat`，不是只换页签颜色。
- A/B 会话 view 记忆；切换会话保留各自选择。
- draft、取消、load 在 view 切换中零副作用。
- 默认收起，28px corner 可开合；右栏全屏/恢复。
- `cmd/app` 测试证明有工具消息且非 explicit 时也默认收起；按会话 explicit 状态恢复。
- 团队/子智能体无真值不画；后续正式 `Snapshot.View/Actions.SelectView` 由 Lead 串行集成，不把插件内适配冒充公共契约。

## 实图与几何

| 文件 | 内容 |
|---|---|
| `plugins/dstopbar/testdata/frame-header/frame-header-chat-1182x832.png` | 对话视图，chat 选中，用户气泡与工具摘要 |
| `plugins/dstopbar/testdata/frame-header/frame-header-trajectory-1182x832.png` | 轨迹视图，trajectory 选中，真实工具轨迹内容 |
| 同目录 1920×1032 两图 | 大窗口两视图 |

owner 报告两尺寸几何 76/30/25/28、top10、gap20 偏差 0；两包测试已随 Lead 复跑通过。1182 chat PNG SHA-256 `fdc4eed34191fb4c2d10062c57f470b071ece68d582fae524eec1ab13a4bc462`，trajectory PNG SHA-256 `9a6ad8fe157f24ae6d01463717e5145136aa1d565d74983ef520951f544f06ff`。

## 边界

- WP-D 的 Turn/Step 分组、搜索、Inspect、完整历史不在本包。
- 团队模型、agent preset、子智能体数量没有契约真值，不显示。
- 公共 view/pluginapi 契约、OS 原生点击、高 DPI、真实 dshd 长链未在本包验收。
