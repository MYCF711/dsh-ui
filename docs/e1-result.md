# E1 · 能不能跑起来 —— **通过**

> 实测时间 2026-11-03 05:20–05:23 · 本机 Windows 10.0.26200 / Go 1.27.3 / **`CGO_ENABLED=0` 且无任何 C 编译器**

## 结果

| 步骤 | 预期 | 实测 | 判定 |
|---|---|---|---|
| `go install github.com/vivaui/viva/cmd/viva@v0.2.3` | 成功 | 18 s，二进制 16.7 MB | ✅ |
| `viva doctor` | 报环境 | **`Everything viva needs is here.`** | ✅ |
| `viva init hello` | 生成工程 | 6 个文件（含 `AGENTS.md`、`viva.toml`） | ✅ |
| `viva build .`（**CGO_ENABLED=0**） | 能出 exe | **成功，3.6 s** | ✅ |
| exe 体积 | ≤ 25 MB | **4,114,944 B = 3.92 MiB**（stripped） | ✅ |
| 双击运行 | 窗口出现 | 窗口标题 `hello`，进程存活 | ✅ |

## 最关键的一个发现

`go.mod` 的依赖里有 **`github.com/ebitengine/purego v0.11.1`**。

**purego 是纯 Go 的 FFI 库——直接 `dlopen`/`syscall` 调系统 DLL，不需要 cgo。**
这解释了为什么 `CGO_ENABLED=0` + 无 gcc 也能编出 Windows D3D11/DirectWrite 程序。

**推论（重要）**：
- 分发不需要用户装任何东西（DLL 全用系统的）；
- **交叉编译可行**（CI 上三平台一份 Go 就能出）；
- 我们那条"No cgo"的老经验，在 Viva 这里不构成障碍。

## 第一个内存实测（E3 的一部分，顺手拿到）

用 `Get-Process hello` 量一个**空窗口**：

| 指标 | 值 |
|---|---:|
| 进程数 | **1** |
| WorkingSet | **67.2 MB** |
| Private | **57 MB** |
| 线程 | 28 |
| 空闲 5 s 后 WorkingSet | 67.2 MB（不变） |
| 累计 CPU（含启动） | 0.23 s |

**如实说：67.2 MB 高于 PLAN 里写的"≤60 MB"目标**，原因待查（怀疑：D3D11 设备 + DirectWrite 字体缓存 + Go runtime 的初始堆）。这不影响 E1 判定，但**E3 要重新量一次真实 UI 状态**，并确认这 67 MB 里哪部分是固定成本。

## 对标的三个数

| | 进程 | 内存 |
|---|---:|---:|
| 官方 DSH（用户截图） | 9 | **1,603.8 MB** |
| dshd（宿主，实测） | 1 | 15.7 MB |
| **Viva 空窗口（本次）** | **1** | **67.2 MB** |
| WebView2（Raycast 口径，空/有内容） | 4+ | 50 / 120–200 MB |

## E1 结论

**不存在的"C 工具链"门槛，Viva 在 Windows 上纯 Go 能跑。继续 E2。**
