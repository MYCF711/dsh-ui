## 现象

Viva 有**文件拖入**（`app.Config.OnFileDrop`）和**拖拽预览窗**（`app.OpenDragPreview`），
但**没有任何"把文件拖出到资源管理器/桌面"的入口**，也**不暴露窗口原生句柄**，
所以应用层即使想自己调 `DoDragDrop` 也无从下手。

## 复现（编译期即可复现，v0.6.4）

```go
package main

import "github.com/vivaui/viva/app"

func main() {
	application := &app.Application{}
	window, _ := application.OpenWindow(app.Config{Width: 400, Height: 300}, nil)
	h := window.Handle()                               // ① 不存在
	application.OpenDragSource(h, []string{"a.txt"})    // ② 不存在
}
```

```
$ go build main.go
main.go:7:14: window.Handle undefined (type *app.Window has no field or method Handle)
main.go:8:14: application.OpenDragSource undefined (type *app.Application has no field or method OpenDragSource)
```

源码侧核对（v0.6.4）：

- `grep -rni "DoDragDrop\|IDropSource\|OleSetClipboard\|IDataObject\|OleDataObject"` → 零命中
- `app/clipboard_files.go:52` 的 `SetClipboardFiles` 是 **CF_HDROP 写剪贴板**（复制语义），不是拖拽
- `app/app.go:154-160` 的 `OnFileDrop` 是**拖入**
- `app/drag_preview.go:20-36` 的 `OpenDragPreview` 是"跟着指针的置顶穿透窗"，只解决观感，不参与 OLE 拖拽循环
- `app` 包不导出任何窗口句柄（`grep -rn "HWND\|NativeHandle\|WindowHandle" app/ internal/platform/contract.go` 无输出）

## 为什么应用层无法自己兜底

Windows 的拖出必须由**持有窗口**的一方在**窗口线程**上调
`DoDragDrop(hwnd, pDataObject, DROPEFFECT_COPY|DROPEFFECT_MOVE, &effect)`：

1. 需要 `HWND` —— `internal/platform` 是 Go 的内部包，外部 import 不了；`app` 没有导出。
2. 拖拽循环会**接管鼠标捕获**并跑自己的消息循环（`OleInitialize` + `MsgWaitForMultipleObjects`），
   与 Viva 自己的指针/捕获状态机在同一线程上直接冲突。

也就是说这不是"加个 shim"能解决的，它必须长在窗口层。

## 期望的 API 形状（两个都可以，二选一）

**方案 A：先只暴露句柄**（最小改动，把选择权交给应用）

```go
// Window 上
// Handle 返回窗口的原生句柄（Windows 为 HWND，macOS 为 NSWindow*，Linux 为 GtkWindow*）。
// 它是给必须直接调平台 API 的应用用的逃生口：返回的句柄只在窗口存活期内有效。
func (window *Window) Handle() uintptr
```

**方案 B：直接给拖出入口**（推荐；应用不用碰 COM）

```go
type DragItem struct {
	Path     string // 已存在的文件路径
	Name     string // 拖出去显示的名字（空 = 用 Path 的 basename）
	MediaID  string // 可选：Windows 上的 FileGroupDescriptor 媒体类型
}

type DragSourceConfig struct {
	Items   []DragItem
	Effects DragEffect // Copy / Move / Link，默认 Copy
	// Preview 可选：拖动时跟随指针的预览（复用现有的 OpenDragPreview）
	Preview *DragPreviewConfig
}

// OpenDragSource 在 window 的窗口线程上跑一次 OLE 拖拽循环，直到用户松手或按 Esc。
// 必须在 OnPointer 的按下事件里调用（窗口此刻持有鼠标捕获）；
// 阻塞期间 Viva 不派发其它指针事件——与系统原生拖拽一致。
func (application *Application) OpenDragSource(window *Window, config DragSourceConfig) (DragEffect, error)
```

平台语义建议：

- **Windows**：`IDropSource` + `IDataObject`（`CF_HDROP`；多文件时给 `FILEGROUPDESCRIPTOR`，
  这样资源管理器/桌面/邮件客户端都能收）。`DROPEFFECT_COPY` 时不动源文件。
- **macOS**：`NSDraggingSession`（`beginDraggingSession(with:event:source:)`）。
- **Linux**：GTK `gtk_drag_source_set` + `gtk_drag_begin_with_coordinates`。

## 验收判据（我们会用它来验证）

1. 从窗口里把一个文件拖到资源管理器 → 目标位置真的出现该文件；
2. 拖到自己窗口的 `OnFileDrop` → 拿到的 `Paths` 与拖出的源一致（应用内拖放闭环）；
3. 拖拽期间按 Esc → `OpenDragSource` 返回 `DragEffectNone`，源文件未被改动；
4. `go vet ./...` 干净、`CGO_ENABLED=0 go build ./...` 可过（Windows 侧用 `purego`/`syscall` 即可，不需要 cgo）。

## 备注

- 我们**不**请求"给已删除的 API 加回兼容包装"这类设计取舍；这里只是缺一条平台能力。
- 如果上游认为拖出应当由应用自己实现，那么**只导出句柄**（方案 A）就足够，我们自己写 COM 部分——
  但请至少给出窗口线程上安全进入 OLE 循环的约定（是否允许在 `OnPointer` 回调里阻塞、捕获状态如何交还）。
