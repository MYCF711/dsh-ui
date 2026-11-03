//go:build windows

// headless_windows.go —— 把窗口挪到屏幕外、用 PrintWindow 抓一张真图，然后退出。
//
// 为什么需要这个：每次验证界面都要在用户屏幕上弹一个窗口，很烦。
// 用 GDI 的 PrintWindow(PW_RENDERFULLCONTENT) 可以让窗口自己把内容画到
// 一个内存位图上，再存成 PNG——不需要窗口可见地停在屏幕上。
//
// 用法： ./app.exe -addr http://127.0.0.1:7788 -shot docs/x.png
// 抓完直接 os.Exit(0)，不留窗口。
package main

import (
	"image"
	"image/png"
	"log"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procEnumWindows        = user32.NewProc("EnumWindows")
	procGetWindowThreadPID = user32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW     = user32.NewProc("GetWindowTextW")
	procGetWindowTextLen   = user32.NewProc("GetWindowTextLengthW")
	procSetWindowPos       = user32.NewProc("SetWindowPos")
	procPrintWindow        = user32.NewProc("PrintWindow")
	procGetWindowRect      = user32.NewProc("GetWindowRect")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procGetWindowDC        = user32.NewProc("GetWindowDC")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procGetDIBits              = gdi32.NewProc("GetDIBits")

	psapi                 = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemory  = psapi.NewProc("GetProcessMemoryInfo")
	procGetCurrentProcess = kernel32.NewProc("GetCurrentProcess")
)

type rect struct {
	Left, Top, Right, Bottom int32
}

// winRect 是 GetWindowRect 的结果。
type winRect = rect

// captureWindow 抓当前进程那个标题含 title 的窗口，存成 PNG。
//
// 做法：先把它挪到屏幕外（这样用户看不见），等一帧画完，再 PrintWindow。
// PrintWindow 的 2 = PW_RENDERFULLCONTENT，D3D 合成的窗口靠它才能抓到内容。
func captureWindow(title, path string, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	var hwnd syscall.Handle
	for time.Now().Before(deadline) && hwnd == 0 {
		hwnd = findOwnWindow(title)
		if hwnd == 0 {
			time.Sleep(80 * time.Millisecond)
		}
	}
	if hwnd == 0 {
		log.Printf("shot: 没找到自己的窗口（title 含 %q）", title)
		return false
	}

	time.Sleep(700 * time.Millisecond) // 等一帧画完

	var r winRect
	procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	if w <= 0 || h <= 0 {
		log.Printf("shot: 窗口尺寸无效 %dx%d", w, h)
		return false
	}

	hdcWindow, _, _ := procGetWindowDC.Call(uintptr(hwnd))
	if hdcWindow == 0 {
		log.Print("shot: GetWindowDC 失败")
		return false
	}
	defer procReleaseDC.Call(uintptr(hwnd), hdcWindow)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcWindow)
	if hdcMem == 0 {
		log.Print("shot: CreateCompatibleDC 失败")
		return false
	}
	defer procDeleteDC.Call(hdcMem)

	hbm, _, _ := procCreateCompatibleBitmap.Call(hdcWindow, uintptr(w), uintptr(h))
	if hbm == 0 {
		log.Print("shot: CreateCompatibleBitmap 失败")
		return false
	}
	defer procDeleteObject.Call(hbm)

	procSelectObject.Call(hdcMem, hbm)
	ok, _, _ := procPrintWindow.Call(uintptr(hwnd), hdcMem, 2) // PW_RENDERFULLCONTENT
	if ok == 0 {
		log.Print("shot: PrintWindow 失败（退回可见窗口截图）")
		return false
	}

	img := readDIB(hdcMem, hbm, w, h)
	if img == nil {
		return false
	}
	f, err := os.Create(path)
	if err != nil {
		log.Printf("shot: 建文件失败: %v", err)
		return false
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Printf("shot: 编码 PNG 失败: %v", err)
		return false
	}
	return true
}

// readDIB 把内存 DC 里的位图读成 image.RGBA（GDI 给的是 BGRA、行倒序）。
func readDIB(hdcMem, hbm uintptr, w, h int) image.Image {
	const biRGB = 0
	type bitmapInfoHeader struct {
		Size          uint32
		Width         int32
		Height        int32
		Planes        uint16
		BitCount      uint16
		Compression   uint32
		SizeImage     uint32
		XPelsPerMeter int32
		YPelsPerMeter int32
		ClrUsed       uint32
		ClrImportant  uint32
	}

	buf := make([]byte, w*h*4)
	bih := bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(w),
		Height:      int32(-h), // 负数 = 从上到下，省一次翻转
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}
	n, _, _ := procGetDIBits.Call(hdcMem, hbm, 0, uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bih)), 0)
	if n == 0 {
		log.Print("shot: GetDIBits 失败")
		return nil
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		b, g, r := buf[i*4], buf[i*4+1], buf[i*4+2]
		img.Pix[i*4] = r
		img.Pix[i*4+1] = g
		img.Pix[i*4+2] = b
		img.Pix[i*4+3] = 255
	}
	return img
}

// workingSetMB 取当前进程工作集（含图形栈那几十 MB，是用户真正看到的数）。
func workingSetMB() float64 {
	type pmc struct {
		cb                         uint32
		PageFaultCount             uint32
		PeakWorkingSetSize         uintptr
		WorkingSetSize             uintptr
		QuotaPeakPagedPoolUsage    uintptr
		QuotaPagedPoolUsage        uintptr
		QuotaPeakNonPagedPoolUsage uintptr
		QuotaNonPagedPoolUsage     uintptr
		PagefileUsage              uintptr
		PeakPagefileUsage          uintptr
	}
	var m pmc
	m.cb = uint32(unsafe.Sizeof(m))
	h, _, _ := procGetCurrentProcess.Call()
	if ok, _, _ := procGetProcessMemory.Call(h, uintptr(unsafe.Pointer(&m)), uintptr(m.cb)); ok == 0 {
		return 0
	}
	return float64(m.WorkingSetSize) / (1024 * 1024)
}

// moveOffscreen 把窗口挪到屏幕外：用户看不见，但窗口依然"在"，PrintWindow 照样能画。
func moveOffscreen(hwnd syscall.Handle) {
	var off int32 = -6000
	procSetWindowPos.Call(uintptr(hwnd), 0,
		uintptr(off), uintptr(off), 0, 0, // SWP_NOSIZE|SWP_NOZORDER
		0x0001|0x0004)
}

// waitOwnWindow 等自己的窗口出现。
func waitOwnWindow(title string, wait time.Duration) syscall.Handle {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if h := findOwnWindow(title); h != 0 {
			return h
		}
		time.Sleep(60 * time.Millisecond)
	}
	return 0
}

// findOwnWindow 在本进程的窗口里找标题含 needle 的那个。
func findOwnWindow(needle string) syscall.Handle {
	pid := uint32(os.Getpid())
	var found syscall.Handle

	cb := syscall.NewCallback(func(hwnd syscall.Handle, _ uintptr) uintptr {
		var owner uint32
		procGetWindowThreadPID.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&owner)))
		if owner != pid {
			return 1
		}
		n, _, _ := procGetWindowTextLen.Call(uintptr(hwnd))
		if n == 0 {
			return 1
		}
		buf := make([]uint16, n+1)
		procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), n+1)
		text := syscall.UTF16ToString(buf)
		if needle == "" || containsFold(text, needle) {
			found = hwnd
			return 0 // 停
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return found
}

// containsFold 是最小的大小写不敏感包含判断（只为匹配窗口标题，不值得引 strings）。
func containsFold(s, sub string) bool {
	ls, lsub := lower(s), lower(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return true
		}
	}
	return false
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// startShot 在后台等窗口起来、抓一张图、然后退出进程。
//
// delay 是"等界面把第一屏画出来"的时间，不是加载时间——窗口 300ms 就出来了，
// 但真实会话的数据是异步拉的，所以给 4 秒。
func startShot(path string, delay time.Duration) {
	go func() {
		// 先等窗口出现，**立刻**挪到屏幕外——不能让用户先看到一个弹窗再等 4 秒。
		hwnd := waitOwnWindow("dsh-ui", 10*time.Second)
		if hwnd == 0 {
			log.Print("shot: 没等到窗口")
			os.Exit(2)
		}
		moveOffscreen(hwnd)
		time.Sleep(delay) // 再等界面把真实会话拉回来

		if captureWindow("dsh-ui", path, 8*time.Second) {
			log.Printf("shot: 已保存 %s · 工作集 %.1f MB", path, workingSetMB())
			os.Exit(0)
		}
		log.Print("shot: 抓取失败")
		os.Exit(2)
	}()
}
