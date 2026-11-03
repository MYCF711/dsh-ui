"""截一张窗口图：按窗口标题找窗口，裁掉不可见的窗框，存 PNG。

用法：
    python scripts/shot.py dsh-ui docs/shot.png [延迟秒]

为什么要自己写：
  · Get-Process 存在 != 窗口已经显示（实测差 6 倍：51ms vs 304ms），所以要等窗口。
  · GetWindowRect 拿到的是**含不可见边框**的矩形，直接截会带进左右各 8px 的
    桌面残影（踩过一次，当成"我们的界面溢出到窗口外"排查了半天）。这里用
    DwmGetWindowAttribute 的 DWMWA_EXTENDED_FRAME_BOUNDS 取真实可见边界。
  · 全程 ctypes，不调 tasklist / PowerShell：中文环境的编码问题不值得再踩一次。
"""

import ctypes
import ctypes.wintypes as wt
import sys
import time

from PIL import Image

user32 = ctypes.windll.user32
dwmapi = ctypes.windll.dwmapi
psapi = ctypes.windll.psapi
gdi32 = ctypes.windll.gdi32
kernel32 = ctypes.windll.kernel32

SW_RESTORE = 9
DWMWA_EXTENDED_FRAME_BOUNDS = 9


class RECT(ctypes.Structure):
    _fields_ = [("left", wt.LONG), ("top", wt.LONG),
                ("right", wt.LONG), ("bottom", wt.LONG)]


class PROCESS_MEMORY_COUNTERS(ctypes.Structure):
    _fields_ = [("cb", wt.DWORD), ("PageFaultCount", wt.DWORD),
                ("PeakWorkingSetSize", ctypes.c_size_t),
                ("WorkingSetSize", ctypes.c_size_t),
                ("QuotaPeakPagedPoolUsage", ctypes.c_size_t),
                ("QuotaPagedPoolUsage", ctypes.c_size_t),
                ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t),
                ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
                ("PagefileUsage", ctypes.c_size_t),
                ("PeakPagefileUsage", ctypes.c_size_t)]


def visible_rect(hwnd):
    """真实可见边界；DWM 不可用时退回 GetWindowRect。"""
    r = RECT()
    ok = dwmapi.DwmGetWindowAttribute(
        wt.HWND(hwnd), ctypes.c_uint(DWMWA_EXTENDED_FRAME_BOUNDS),
        ctypes.byref(r), ctypes.sizeof(r))
    if ok != 0 or r.right <= r.left or r.bottom <= r.top:
        r = RECT()
        user32.GetWindowRect(wt.HWND(hwnd), ctypes.byref(r))
    return r


def find_window(title, timeout=15.0):
    """按标题找可见的顶层窗口，返回 (hwnd, pid)。"""
    needle = title.lower()
    deadline = time.time() + timeout
    while time.time() < deadline:
        hit = []

        @ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)
        def cb(hwnd, _):
            if not user32.IsWindowVisible(hwnd):
                return True
            n = user32.GetWindowTextLengthW(hwnd)
            if n <= 0:
                return True
            buf = ctypes.create_unicode_buffer(n + 1)
            user32.GetWindowTextW(hwnd, buf, n + 1)
            if needle in buf.value.lower():
                pid = wt.DWORD()
                user32.GetWindowThreadProcessId(hwnd, ctypes.byref(pid))
                hit.append((pid.value, hwnd))
            return True

        user32.EnumWindows(cb, 0)
        if hit:
            # 同名窗口可能存在多个（旧的没退干净）：取 pid 最大的那个 = 最晚起的。
            hit.sort()
            return hit[-1][1], hit[-1][0]
        time.sleep(0.12)
    return None, None


def working_set_mb(pid):
    """取进程工作集（MB）；取不到返回 0。"""
    PROCESS_QUERY_INFORMATION = 0x0400
    h = kernel32.OpenProcess(PROCESS_QUERY_INFORMATION, False, pid)
    if not h:
        return 0.0
    try:
        pmc = PROCESS_MEMORY_COUNTERS()
        pmc.cb = ctypes.sizeof(pmc)
        if psapi.GetProcessMemoryInfo(h, ctypes.byref(pmc), pmc.cb):
            return pmc.WorkingSetSize / (1024 * 1024)
        return 0.0
    finally:
        kernel32.CloseHandle(h)


class BITMAPINFOHEADER(ctypes.Structure):
    _fields_ = [("biSize", wt.DWORD), ("biWidth", wt.LONG), ("biHeight", wt.LONG),
                ("biPlanes", wt.WORD), ("biBitCount", wt.WORD),
                ("biCompression", wt.DWORD), ("biSizeImage", wt.DWORD),
                ("biXPelsPerMeter", wt.LONG), ("biYPelsPerMeter", wt.LONG),
                ("biClrUsed", wt.DWORD), ("biClrImportant", wt.DWORD)]


def grab_window(hwnd, r):
    """用 PrintWindow 直接抓这个窗口的像素，**不受遮挡/前后台影响**。

    为什么要这样：ImageGrab 截的是屏幕，只要我们的窗口被别的窗口盖住，
    截到的就是别人（踩过：截成了 DSH 主窗口，还以为是渲染坏了）。
    PW_RENDERFULLCONTENT(2) 让 DWM 把离屏内容也画进来。
    """
    w, h = r.right - r.left, r.bottom - r.top
    if w <= 0 or h <= 0:
        return None
    hdc_screen = user32.GetDC(0)
    hdc_mem = gdi32.CreateCompatibleDC(hdc_screen)
    hbmp = gdi32.CreateCompatibleBitmap(hdc_screen, w, h)
    old = gdi32.SelectObject(hdc_mem, hbmp)
    ok = user32.PrintWindow(wt.HWND(hwnd), hdc_mem, 2)
    if not ok:
        user32.PrintWindow(wt.HWND(hwnd), hdc_mem, 0)

    bi = BITMAPINFOHEADER()
    bi.biSize = ctypes.sizeof(BITMAPINFOHEADER)
    bi.biWidth, bi.biHeight = w, -h   # 负高度 = top-down
    bi.biPlanes, bi.biBitCount = 1, 32
    bi.biCompression = 0              # BI_RGB
    buf = ctypes.create_string_buffer(w * h * 4)
    got = gdi32.GetDIBits(hdc_mem, hbmp, 0, h, buf,
                          ctypes.byref(bi), 0)

    gdi32.SelectObject(hdc_mem, old)
    gdi32.DeleteObject(hbmp)
    gdi32.DeleteDC(hdc_mem)
    user32.ReleaseDC(0, hdc_screen)

    if got == 0:
        return None
    return Image.frombuffer("RGBA", (w, h), buf, "raw", "BGRA", 0, 1).convert("RGB")


def main():
    title = sys.argv[1] if len(sys.argv) > 1 else "dsh-ui"
    out = sys.argv[2] if len(sys.argv) > 2 else "docs/shot.png"
    delay = float(sys.argv[3]) if len(sys.argv) > 3 else 1.6

    # 高 DPI 下不声明就拿不到真实像素：会截成缩小版。
    try:
        user32.SetProcessDPIAware()
    except Exception:
        pass

    hwnd, pid = find_window(title)
    if not hwnd:
        print("NO_WINDOW")
        return 1

    user32.ShowWindow(wt.HWND(hwnd), SW_RESTORE)
    # 置顶再拉前台：SetForegroundWindow 经常被系统拒绝（前台锁），
    # 拒绝时会截到盖在上面的别的窗口——踩过一次，截成了 DSH 主窗口。
    HWND_TOPMOST, HWND_NOTOPMOST = -1, -2
    SWP_NOSIZE, SWP_NOMOVE, SWP_SHOWWINDOW = 0x0001, 0x0002, 0x0040
    user32.SetWindowPos(wt.HWND(hwnd), HWND_TOPMOST, 0, 0, 0, 0,
                        SWP_NOSIZE | SWP_NOMOVE | SWP_SHOWWINDOW)
    user32.SetForegroundWindow(wt.HWND(hwnd))
    time.sleep(delay)

    r = visible_rect(hwnd)
    img = grab_window(hwnd, r)
    if img is None:
        print("GRAB_FAILED")
        return 1
    img.save(out)
    user32.SetWindowPos(wt.HWND(hwnd), HWND_NOTOPMOST, 0, 0, 0, 0,
                        SWP_NOSIZE | SWP_NOMOVE | SWP_SHOWWINDOW)

    print("OK %dx%d pid=%d ws=%.1fMB -> %s" %
          (img.width, img.height, pid, working_set_mb(pid), out))
    return 0


if __name__ == "__main__":
    sys.exit(main())
