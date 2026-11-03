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

from PIL import ImageGrab

user32 = ctypes.windll.user32
dwmapi = ctypes.windll.dwmapi
psapi = ctypes.windll.psapi
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
                hit.append(hwnd)
            return True

        user32.EnumWindows(cb, 0)
        if hit:
            hwnd = hit[0]
            pid = wt.DWORD()
            user32.GetWindowThreadProcessId(wt.HWND(hwnd), ctypes.byref(pid))
            return hwnd, pid.value
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
    user32.SetForegroundWindow(wt.HWND(hwnd))
    time.sleep(delay)

    r = visible_rect(hwnd)
    img = ImageGrab.grab(bbox=(r.left, r.top, r.right, r.bottom), all_screens=True)
    img.save(out)

    print("OK %dx%d pid=%d ws=%.1fMB -> %s" %
          (img.width, img.height, pid, working_set_mb(pid), out))
    return 0


if __name__ == "__main__":
    sys.exit(main())
