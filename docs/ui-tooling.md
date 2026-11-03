# UI 工具集选择：不换主壳，补测量与自动化

## 结论

当前 GUI 与 DSH 的差距不能靠模型识图补齐。主壳继续使用 **Go + Viva**；新增的是确定性的测量、对拍和原生行为验证工具，不引入 Electron/WebView 或 TypeScript 主链。

## 采用顺序

| 层级 | 选择 | 用途 | 当前状态 |
|---|---|---|---|
| 规格真值 | 官方 DOM / JSX / CSS / theme tokens | 部件结构、几何、字号、颜色、状态，不靠看图猜 | 已有官方 asar 提取资料 |
| 原生截图 | Viva headless `-shot`、只读 `PrintWindow` | 同一尺寸、同一数据、同一 DPI 的真实渲染图 | 已有；官方参考图须验来源 |
| 像素测量 | **Python + Pillow + NumPy** | 区域颜色、色带/边界、bbox、逐像素 diff、区域 JSON | 本机已有 Pillow 12.3 / NumPy 2.3.5，立即可用 |
| Windows 行为 | **pywinauto / UIA**（需要时） | 真实鼠标、键盘、焦点、窗口状态，不依赖坐标猜控件 | 本机未安装；不作为当前前置 |
| 视觉回归参考 | pixelmatch / Playwright `toHaveScreenshot` | 感知色差、抗锯齿处理、基线管理思路 | 不引入主程序；本机 Node pixelmatch 未安装 |
| 备用操作 | AutoHotkey | 无 UIA 时的窗口/输入兜底 | 坐标脆弱，不作主验收 |

## 为什么不是直接换热门 UI 框架

- **Playwright** 适合浏览器/Electron 的截图和行为回归；Viva 是原生窗口，不能用 Playwright 直接驱动。它的价值是基线管理与 pixelmatch 算法参考。
- **pixelmatch** 是优秀的像素比较库，但它是 JS 测试工具，不应成为 Go 宿主运行时依赖。当前用 Pillow/NumPy 即可满足区域测量；确需感知色差时可作为独立测试工具。
- **pywinauto** 能操作 Windows GUI，但不能证明布局正确；它负责行为，截图/几何工具负责外观。
- Fyne/Gio/Wails/React 是另一套技术路线，不会自动消除部件和宿主能力缺口。现在换框架只会丢掉已完成的 Viva 工作。

## 验收方式改变

不再以“模型看着像”签收，改为四层证据：

1. **结构**：官方 DOM/源码符号 ↔ 当前插件槽/控件映射。
2. **几何**：真实 `flex.Rect` 与指定区域像素测量，记录偏差，不靠 OCR 猜位置。
3. **外观**：同版本、同数据、同尺寸、同 DPI 的参考图/实测图 diff；动态文本、时间、光标先遮罩，禁止盲更新 golden。
4. **行为**：真实发送/审批/切换/滚动/快捷键；像素图不能证明功能等价。

每个区域报告 `target / actual / tolerance / verdict / evidence`，无官方参考图就写 `reference_unavailable`，不得把“没有找到参考图”写成“整屏相似度通过”。

## 外部资料

- Playwright 视觉比较：`toHaveScreenshot`、基线、`maxDiffPixels`、`threshold`，并要求同环境生成基线：https://playwright.dev/docs/test-snapshots
- pixelmatch：感知色差、抗锯齿识别、区域/窗口化差异：https://www.npmjs.com/package/pixelmatch
- pywinauto：Windows GUI 自动化与 UIA 后端：https://pywinauto.readthedocs.io/en/latest/
- AutoHotkey v2：窗口、鼠标、键盘自动化：https://www.autohotkey.com/docs/v2/
