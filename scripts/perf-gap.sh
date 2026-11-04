#!/usr/bin/env bash
# perf-gap.sh —— docs/perf-gap-plan.md 里「零改动就能测」的四项，实测（不估算、不套用设计值）：
#
#   G1  GPU 显存（WDDM 计数器：Dedicated / Local / Shared / Non Local）
#   G6  真模型一轮 / 带工具一轮（界面进程内存四时刻）
#   G7  五会话切换的行为验收（切走再切回 vs 直接打开，逐像素比对）
#   G8  宿主的整进程树（serve + dshd run + 工具进程，以我们自己起的 serve PID 为根递归）
#
# 用法：
#   bash scripts/perf-gap.sh                       # 默认 G1/G7/G8（不吃额度，不跑真模型）
#   PERF_GAP_REAL=1 bash scripts/perf-gap.sh       # 加上 G6（真模型轮；要 magpie 可用、会消耗真实额度）
#   PERF_GAP_ONLY=G1,G8 bash scripts/perf-gap.sh   # 只跑其中几项
#   PERF_GAP_REUSE=0 bash scripts/perf-gap.sh      # 重跑并覆盖已有证据（默认 1 = 已有 result.json 就跳过）
#
# 开关：
#   PERF_GAP_RUNS=3            每个场景重复次数（跨进程取中位数）
#   PERF_GAP_IDLE=10           静置段秒数
#   PERF_GAP_SESSIONS=6        从真实 home 复制几份会话（G7 需要 5 个不同 id + 1 个参考）
#   PERF_GAP_MAX_SESSION_MB=12 单份会话上限（太大就把测量变成 I/O 测量）
#   PERF_GAP_REAL_HOME=...     真实 home（只读来源，只做复制）
#   PERF_GAP_HOST_REPO=../DSH-GO / PERF_GAP_HOST_REV=4f7c885
#   PERF_GAP_UI_REV / PERF_GAP_APP / PERF_GAP_SERVE   覆盖默认二进制
#   PERF_GAP_LOAD_MAX=70       系统 CPU 占用阈值（%）；PERF_GAP_LOAD_STRICT=1 时超阈值直接退出 3
#
# 口径与边界（细节见 docs/perf-gap-plan.md §2 与 docs/perf-zero-results.md）：
#   * 端口只用 OS 分配：`serve --addr 127.0.0.1:0`，URL/一次性 token 只认它 stdout 的 ready 行；
#   * 每个场景一个全新的临时 home（真实会话的只读副本），绝不写真实 home；
#   * 只回收自己 Popen 出来的 PID —— 回收前用「创建时间 + 映像名」核对，核对结果写进证据；
#   * 整树归属用「子进程创建时间 >= 父进程创建时间」剪掉 PID 复用造成的假父子
#     （实测踩过：华硕 ArmourySocketServer 的 ppid 恰好等于本轮 serve 的 PID）；
#   * 内存 ws 与 priv 分列、显存 4 个量分列；静置段取最后 1 s 中位数，峰值取窗口最大值；
#   * 拿不到的一律写 unavailable + 原因，**不写 0、不写设计值**。
#
# 证据：tmp/perf-gap/<case>/<n>/{result.json,series.jsonl,tree.jsonl,app.log,serve.log}
#       tmp/perf-gap/summary.json + summary.md（汇总）

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
export PATH="/c/Program Files/Go/bin:$PATH"

PY="${PERF_GAP_PYTHON:-}"
if [ -z "$PY" ]; then
  if command -v python >/dev/null 2>&1; then PY=python
  elif command -v python3 >/dev/null 2>&1; then PY=python3
  else echo "需要 python 3.8+（tkinter/PIL 用来比对 G7 的 PNG）" >&2; exit 2; fi
fi

WORK="${PERF_GAP_WORK:-tmp/perf-gap}"
ONLY="${PERF_GAP_ONLY:-G1,G6,G7,G8}"
RUNS="${PERF_GAP_RUNS:-3}"
REAL="${PERF_GAP_REAL:-0}"
REUSE="${PERF_GAP_REUSE:-1}"
IDLE="${PERF_GAP_IDLE:-10}"
SESSIONS="${PERF_GAP_SESSIONS:-6}"
MAX_SESSION_MB="${PERF_GAP_MAX_SESSION_MB:-12}"
REAL_HOME="${PERF_GAP_REAL_HOME:-D:/DeepSeek Harness/home}"
HOST_REPO="${PERF_GAP_HOST_REPO:-../DSH-GO}"
HOST_REV="${PERF_GAP_HOST_REV:-4f7c885}"
UI_REV="${PERF_GAP_UI_REV:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
APP="${PERF_GAP_APP:-$WORK/bin/dshgogui.exe}"
# 宿主也叫 `dshd.exe`：**版本写在目录上，不写在文件名上**。
# 以前是 `dshd-4f7c885.exe`——每换一个 rev 就多一个镜像名，进程表里就成了
# "dshd.exe / dshd-acc.exe / dshd-4f7c885.exe" 混排，人要逐个猜它们是谁。
SERVE="${PERF_GAP_SERVE:-$WORK/bin/host-$HOST_REV/dshd.exe}"
TPL="$WORK/home-template"
LOAD_MAX="${PERF_GAP_LOAD_MAX:-70}"
LOAD_STRICT="${PERF_GAP_LOAD_STRICT:-0}"
BASE_URL="${PERF_GAP_LLM_BASE:-http://127.0.0.1:3425/v1}"
TURN_TIMEOUT="${PERF_GAP_TURN_TIMEOUT:-150}"
TEXT_PROMPT='PERF0 纯文本轮：只回一句 "perf0-text-ok"，不要调用任何工具。'
TOOL_PROMPT='PERF0 工具轮：请用 bash 工具执行一次 echo hi，然后把原始输出贴回来。只做这一次工具调用。'

log() { printf '%s\n' "$*" >&2; }
have() { case ",$ONLY," in *",$1,"*) return 0;; *) return 1;; esac; }

mkdir -p "$WORK/bin"

# ── 0. 采样器（脚本自包含：harness.py / summarize.py 由本脚本展开到 $WORK）────────
write_helpers() {
  cat > "$WORK/harness.py" <<'PERF_GAP_HARNESS_EOF'
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""perf-gap harness —— G1(显存) / G6(真模型轮) / G7(会话一致) / G8(宿主整树) 的进程外采样器。

只做外部观测，不改被测进程：
  * 端口只用 OS 分配（serve --addr 127.0.0.1:0，URL 只认 stdout 的 ready 行）；
  * home 只用临时副本（真实 home 只读复制，绝不写原 home）；
  * 只回收自己 Popen 出来的 PID —— 回收前用「创建时间 + 映像名」核对，核对结果写进证据；
  * 数字来源：PDH `\\GPU Process Memory(*)\\*`（WDDM 计数器）/ K32GetProcessMemoryInfo /
    CreateToolhelp32Snapshot 递归整树 / GetProcessTimes。不读界面或宿主自己打印的数字当指标；
  * 拿不到就写 null + why，**绝不写 0 冒充**。

子命令：
  run     起 serve + app，按 case 采样（idle / turn / exit）
  pngdiff 两张 PNG 的逐像素比对（G7 的判据）
"""

from __future__ import annotations

import argparse
import ctypes
import hashlib
import json
import os
import shutil
import subprocess
import sys
import threading
import time
from ctypes import wintypes

k32 = ctypes.WinDLL("kernel32", use_last_error=True)
u32 = ctypes.WinDLL("user32", use_last_error=True)

PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
PROCESS_VM_READ = 0x0010
TH32CS_SNAPPROCESS = 0x00000002
MAX_PATH = 32768
MB = 1048576.0


# ── win32 结构与基础采样 ────────────────────────────────────────────────────
class PMC_EX(ctypes.Structure):
    _fields_ = [("cb", wintypes.DWORD), ("PageFaultCount", wintypes.DWORD),
                ("PeakWorkingSetSize", ctypes.c_size_t), ("WorkingSetSize", ctypes.c_size_t),
                ("QuotaPeakPagedPoolUsage", ctypes.c_size_t), ("QuotaPagedPoolUsage", ctypes.c_size_t),
                ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t), ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
                ("PagefileUsage", ctypes.c_size_t), ("PeakPagefileUsage", ctypes.c_size_t),
                ("PrivateUsage", ctypes.c_size_t)]


class FILETIME(ctypes.Structure):
    _fields_ = [("dwLowDateTime", wintypes.DWORD), ("dwHighDateTime", wintypes.DWORD)]


class PROCESSENTRY32W(ctypes.Structure):
    _fields_ = [("dwSize", wintypes.DWORD), ("cntUsage", wintypes.DWORD),
                ("th32ProcessID", wintypes.DWORD), ("th32DefaultHeapID", ctypes.POINTER(ctypes.c_ulong)),
                ("th32ModuleID", wintypes.DWORD), ("cntThreads", wintypes.DWORD),
                ("th32ParentProcessID", wintypes.DWORD), ("pcPriClassBase", wintypes.LONG),
                ("dwFlags", wintypes.DWORD), ("szExeFile", ctypes.c_wchar * 260)]


class RECT(ctypes.Structure):
    _fields_ = [("left", wintypes.LONG), ("top", wintypes.LONG),
                ("right", wintypes.LONG), ("bottom", wintypes.LONG)]


WNDENUMPROC = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HWND, wintypes.LPARAM)
u32.EnumWindows.argtypes = [WNDENUMPROC, wintypes.LPARAM]
u32.EnumWindows.restype = wintypes.BOOL
u32.GetWindowThreadProcessId.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.DWORD)]
u32.GetWindowThreadProcessId.restype = wintypes.DWORD
u32.GetWindowRect.argtypes = [wintypes.HWND, ctypes.POINTER(RECT)]
u32.GetWindowRect.restype = wintypes.BOOL
u32.IsWindowVisible.argtypes = [wintypes.HWND]
u32.IsWindowVisible.restype = wintypes.BOOL
u32.SetWindowPos.argtypes = [wintypes.HWND, wintypes.HWND, ctypes.c_int, ctypes.c_int,
                             ctypes.c_int, ctypes.c_int, wintypes.UINT]
u32.SetWindowPos.restype = wintypes.BOOL


def own_windows(pid):
    """本进程（且只有本进程）的顶层窗口：(hwnd, w, h, visible)。"""
    out = []

    def cb(hwnd, _):
        owner = wintypes.DWORD(0)
        u32.GetWindowThreadProcessId(hwnd, ctypes.byref(owner))
        if owner.value == pid:
            r = RECT()
            u32.GetWindowRect(hwnd, ctypes.byref(r))
            out.append((int(hwnd), r.right - r.left, r.bottom - r.top, bool(u32.IsWindowVisible(hwnd))))
        return True

    u32.EnumWindows(WNDENUMPROC(cb), 0)
    return out


def hide_offscreen(hwnd):
    """挪到屏幕外：用户看不见，但窗口照画（PrintWindow/采样都不受影响）。
    SWP_NOSIZE(0x1)|SWP_NOZORDER(0x4)|SWP_NOACTIVATE(0x10)。"""
    u32.SetWindowPos(wintypes.HWND(hwnd), wintypes.HWND(0), -6000, -6000, 0, 0, 0x1 | 0x4 | 0x10)


k32.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
k32.OpenProcess.restype = wintypes.HANDLE
k32.CloseHandle.argtypes = [wintypes.HANDLE]
k32.K32GetProcessMemoryInfo.argtypes = [wintypes.HANDLE, ctypes.c_void_p, wintypes.DWORD]
k32.K32GetProcessMemoryInfo.restype = wintypes.BOOL
k32.GetProcessTimes.argtypes = [wintypes.HANDLE, ctypes.POINTER(FILETIME), ctypes.POINTER(FILETIME),
                                ctypes.POINTER(FILETIME), ctypes.POINTER(FILETIME)]
k32.GetProcessTimes.restype = wintypes.BOOL
k32.QueryFullProcessImageNameW.argtypes = [wintypes.HANDLE, wintypes.DWORD, wintypes.LPWSTR,
                                           ctypes.POINTER(wintypes.DWORD)]
k32.QueryFullProcessImageNameW.restype = wintypes.BOOL
k32.CreateToolhelp32Snapshot.argtypes = [wintypes.DWORD, wintypes.DWORD]
k32.CreateToolhelp32Snapshot.restype = wintypes.HANDLE
k32.Process32FirstW.argtypes = [wintypes.HANDLE, ctypes.POINTER(PROCESSENTRY32W)]
k32.Process32FirstW.restype = wintypes.BOOL
k32.Process32NextW.argtypes = [wintypes.HANDLE, ctypes.POINTER(PROCESSENTRY32W)]
k32.Process32NextW.restype = wintypes.BOOL


k32.GetSystemTimes.argtypes = [ctypes.POINTER(FILETIME), ctypes.POINTER(FILETIME), ctypes.POINTER(FILETIME)]
k32.GetSystemTimes.restype = wintypes.BOOL


def sys_times():
    """(idle, kernel, user) 的 FILETIME 计数值 —— 用来给出「本轮采样期间的系统 CPU 占用」，
    作为「没有和压测同时跑」的环境证据（kernel 里已经包含 idle）。"""
    i, k, u = FILETIME(), FILETIME(), FILETIME()
    if k32.GetSystemTimes(ctypes.byref(i), ctypes.byref(k), ctypes.byref(u)) == 0:
        return None
    q = lambda f: (f.dwHighDateTime << 32) | f.dwLowDateTime
    return q(i), q(k), q(u)


def open_process(pid):
    return k32.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION | PROCESS_VM_READ, False, pid)


def mem_counters(handle):
    pmc = PMC_EX()
    pmc.cb = ctypes.sizeof(PMC_EX)
    if k32.K32GetProcessMemoryInfo(handle, ctypes.byref(pmc), pmc.cb) == 0:
        return None
    return {"ws": int(pmc.WorkingSetSize), "priv": int(pmc.PrivateUsage),
            "peak_ws": int(pmc.PeakWorkingSetSize)}


def proc_create_time(pid):
    h = k32.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, False, pid)
    if not h:
        return None
    try:
        c, e, k, u = FILETIME(), FILETIME(), FILETIME(), FILETIME()
        if k32.GetProcessTimes(h, ctypes.byref(c), ctypes.byref(e), ctypes.byref(k), ctypes.byref(u)) == 0:
            return None
        return (c.dwHighDateTime << 32) | c.dwLowDateTime
    finally:
        k32.CloseHandle(h)


def cpu_ms(handle):
    """GetProcessTimes 的 kernel+user（毫秒）。"""
    c, e, k, u = FILETIME(), FILETIME(), FILETIME(), FILETIME()
    if k32.GetProcessTimes(handle, ctypes.byref(c), ctypes.byref(e), ctypes.byref(k), ctypes.byref(u)) == 0:
        return None
    q = lambda f: (f.dwHighDateTime << 32) | f.dwLowDateTime
    return (q(k) + q(u)) / 10000.0


def proc_image(pid):
    h = k32.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, False, pid)
    if not h:
        return ""
    try:
        size = wintypes.DWORD(MAX_PATH)
        buf = ctypes.create_unicode_buffer(MAX_PATH)
        if k32.QueryFullProcessImageNameW(h, 0, buf, ctypes.byref(size)):
            return buf.value
        return ""
    finally:
        k32.CloseHandle(h)


def snapshot_processes():
    """{pid: (ppid, exeName)} —— 一次 Toolhelp32 全量快照。"""
    snap = k32.CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0)
    if snap in (0, -1, None):
        return {}
    out = {}
    try:
        entry = PROCESSENTRY32W()
        entry.dwSize = ctypes.sizeof(PROCESSENTRY32W)
        ok = k32.Process32FirstW(snap, ctypes.byref(entry))
        while ok:
            out[int(entry.th32ProcessID)] = (int(entry.th32ParentProcessID), entry.szExeFile)
            ok = k32.Process32NextW(snap, ctypes.byref(entry))
    finally:
        k32.CloseHandle(snap)
    return out


def process_tree(root_pid, snapshot=None, dropped=None, strict=True):
    """以 root_pid 为根递归收全树（BFS），每个 PID 带 ws/priv/映像名。

    **strict**：Windows 不清理已退出父进程留下的 ppid，PID 一旦被复用就会出现
    "别人的进程挂在我们的根下面"（实测：华硕 ArmourySocketServer/asus_framework 的
    ppid 恰好等于本轮 serve 的 PID）。判据用创建时间：子进程的创建时间必须
    >= 父进程的创建时间，否则这条边是 PID 复用造成的假父子，剪掉并记进 dropped。
    """
    procs = snapshot if snapshot is not None else snapshot_processes()
    kids = {}
    for pid, (ppid, _name) in procs.items():
        kids.setdefault(ppid, []).append(pid)
    ctimes = {}

    def ctime_of(pid):
        if pid not in ctimes:
            ctimes[pid] = proc_create_time(pid)
        return ctimes[pid]

    out, seen, queue = [], set(), [(root_pid, 0, ctime_of(root_pid))]
    while queue:
        pid, depth, pct = queue.pop(0)
        if pid in seen:
            continue
        seen.add(pid)
        ppid, name = procs.get(pid, (0, ""))
        row = {"pid": pid, "ppid": ppid, "depth": depth, "name": name}
        h = open_process(pid)
        if h:
            m = mem_counters(h)
            k32.CloseHandle(h)
            if m:
                row["ws"], row["priv"] = m["ws"], m["priv"]
        out.append(row)
        for c in sorted(kids.get(pid, [])):
            cct = ctime_of(c)
            if strict and pct is not None and cct is not None and cct < pct:
                if dropped is not None:
                    dropped.append({"pid": c, "ppid": pid, "why": "child_ctime < parent_ctime",
                                    "child_ctime": cct, "parent_ctime": pct})
                continue
            queue.append((c, depth + 1, cct))
    return out


# ── PDH / WDDM 显存（G1）────────────────────────────────────────────────────
PDH_FMT_LARGE = 0x00000100
PDH_MORE_DATA = 0x800007D2
PDH_CSTATUS_VALID_DATA = 0x00000000
PDH_CSTATUS_NEW_DATA = 0x00000001

GPU_METRICS = [("Dedicated Usage", "dedicated"), ("Local Usage", "local"),
               ("Shared Usage", "shared"), ("Non Local Usage", "nonlocal")]


class PDH_FMT_COUNTERVALUE(ctypes.Structure):
    _fields_ = [("CStatus", wintypes.DWORD), ("largeValue", ctypes.c_longlong)]


class PDH_FMT_COUNTERVALUE_ITEM_W(ctypes.Structure):
    _fields_ = [("szName", ctypes.c_wchar_p), ("FmtValue", PDH_FMT_COUNTERVALUE)]


class GPUProbe:
    """`\\GPU Process Memory(*)\\<metric>`（English 计数器 API）→ 按 pid_<PID>_ 前缀过滤求和。"""

    def __init__(self):
        self.err = ""
        self.counters = {}
        self.hq = ctypes.c_void_p()
        try:
            self.pdh = ctypes.WinDLL("pdh", use_last_error=True)
        except OSError as exc:
            self.err = "pdh.dll 打不开：%r" % (exc,)
            return
        p = self.pdh
        p.PdhOpenQueryW.argtypes = [wintypes.LPCWSTR, ctypes.c_size_t, ctypes.POINTER(ctypes.c_void_p)]
        p.PdhOpenQueryW.restype = ctypes.c_ulong
        p.PdhAddEnglishCounterW.argtypes = [ctypes.c_void_p, wintypes.LPCWSTR, ctypes.c_size_t,
                                            ctypes.POINTER(ctypes.c_void_p)]
        p.PdhAddEnglishCounterW.restype = ctypes.c_ulong
        p.PdhCollectQueryData.argtypes = [ctypes.c_void_p]
        p.PdhCollectQueryData.restype = ctypes.c_ulong
        p.PdhGetFormattedCounterArrayW.argtypes = [ctypes.c_void_p, wintypes.DWORD,
                                                   ctypes.POINTER(wintypes.DWORD),
                                                   ctypes.POINTER(wintypes.DWORD), ctypes.c_void_p]
        p.PdhGetFormattedCounterArrayW.restype = ctypes.c_ulong
        p.PdhCloseQuery.argtypes = [ctypes.c_void_p]
        p.PdhCloseQuery.restype = ctypes.c_ulong
        st = p.PdhOpenQueryW(None, 0, ctypes.byref(self.hq))
        if st != 0:
            self.err = "PdhOpenQueryW=0x%08X" % st
            return
        for metric, key in GPU_METRICS:
            hc = ctypes.c_void_p()
            path = "\\GPU Process Memory(*)\\" + metric
            st = p.PdhAddEnglishCounterW(self.hq, path, 0, ctypes.byref(hc))
            if st != 0:
                self.err = "PdhAddEnglishCounterW(%s)=0x%08X（本机没有这个计数器）" % (path, st)
                return
            self.counters[key] = hc
        p.PdhCollectQueryData(self.hq)  # 第一次建立基线

    def _array(self, key):
        p = self.pdh
        hc = self.counters[key]
        size = wintypes.DWORD(0)
        cnt = wintypes.DWORD(0)
        st = p.PdhGetFormattedCounterArrayW(hc, PDH_FMT_LARGE, ctypes.byref(size), ctypes.byref(cnt), None)
        if st not in (0, PDH_MORE_DATA) or size.value == 0:
            return []
        buf = ctypes.create_string_buffer(size.value)
        st = p.PdhGetFormattedCounterArrayW(hc, PDH_FMT_LARGE, ctypes.byref(size), ctypes.byref(cnt), buf)
        if st != 0:
            return []
        arr = ctypes.cast(buf, ctypes.POINTER(PDH_FMT_COUNTERVALUE_ITEM_W))
        out = []
        for i in range(cnt.value):
            it = arr[i]
            if it.szName is None:
                continue
            if it.FmtValue.CStatus not in (PDH_CSTATUS_VALID_DATA, PDH_CSTATUS_NEW_DATA):
                continue
            out.append((it.szName, int(it.FmtValue.largeValue)))
        return out

    def sample(self, pid):
        if self.err:
            return {"error": self.err}
        st = self.pdh.PdhCollectQueryData(self.hq)
        if st != 0:
            return {"error": "PdhCollectQueryData=0x%08X" % st}
        pref = "pid_%d_" % pid
        out = {"instances": 0, "by_adapter": []}
        per_adapter = {}
        for _metric, key in GPU_METRICS:
            tot = 0
            seen = 0
            for name, val in self._array(key):
                if not name.lower().startswith(pref):
                    continue
                seen += 1
                tot += val
                per_adapter.setdefault(name, {})[key] = val
            out[key] = tot
            if key == "dedicated":
                out["instances"] = seen
        for name, vals in sorted(per_adapter.items()):
            row = {"instance": name}
            row.update(vals)
            out["by_adapter"].append(row)
        out["present"] = bool(out["instances"] > 0)
        return out

    def close(self):
        try:
            if self.hq:
                self.pdh.PdhCloseQuery(self.hq)
        except Exception:  # noqa
            pass


# ── 子进程与日志 ────────────────────────────────────────────────────────────
class LogTail:
    """增量读日志文件，支持 seen(marker)。"""

    def __init__(self, path):
        self.path = path
        self.off = 0
        self.text = ""

    def poll(self):
        try:
            with open(self.path, "r", encoding="utf-8", errors="replace") as f:
                f.seek(self.off)
                chunk = f.read()
                self.off = f.tell()
        except OSError:
            return self.text
        self.text += chunk
        return self.text

    def seen(self, marker):
        return marker in self.text


def read_ready(proc, timeout):
    """serve 的 stdout 只有一行 ready {...}；用线程读，别把主循环堵住。"""
    box = {}

    def reader():
        try:
            box["line"] = proc.stdout.readline()
        except Exception as exc:  # noqa
            box["err"] = repr(exc)

    th = threading.Thread(target=reader, daemon=True)
    th.start()
    th.join(timeout)
    line = (box.get("line") or b"").decode("utf-8", "replace").strip()
    if not line.startswith("ready "):
        raise SystemExit("serve 没给出 ready 行：" + repr(line[:300]))
    return json.loads(line[len("ready "):])


class Owned:
    """自己 Popen 的进程：回收前核对创建时间 + 映像名，核对结果写进证据。"""

    def __init__(self, proc, label):
        self.proc = proc
        self.label = label
        self.ctime = proc_create_time(proc.pid)
        self.image = proc_image(proc.pid)

    def snapshot(self):
        return {"label": self.label, "pid": self.proc.pid, "ctime": self.ctime,
                "image": self.image}

    def stop(self, wait=10.0):
        rec = {"pid": self.proc.pid, "label": self.label, "ctime": self.ctime,
               "image": self.image}
        rec["ctime_now"] = proc_create_time(self.proc.pid)
        rec["image_now"] = proc_image(self.proc.pid)
        rec["ctime_match"] = (rec["ctime_now"] == self.ctime)
        rec["image_match"] = (rec["image_now"] == self.image)
        if self.proc.poll() is not None:
            rec["action"] = "already_exited"
            rec["exit"] = self.proc.returncode
            return rec
        self.proc.terminate()
        try:
            self.proc.wait(timeout=wait)
            rec["action"] = "terminate"
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait(timeout=wait)
            rec["action"] = "kill"
        rec["exit"] = self.proc.returncode
        return rec


# ── 场景执行 ────────────────────────────────────────────────────────────────
def cmd_run(a):
    repo = os.path.abspath(a.repo)
    work = os.path.abspath(a.work)
    os.makedirs(work, exist_ok=True)
    home = os.path.join(work, "home")
    if a.home_template:
        tpl = a.home_template if os.path.isabs(a.home_template) else os.path.join(repo, a.home_template)
        if os.path.exists(home):
            shutil.rmtree(home)
        shutil.copytree(tpl, home)
    else:
        os.makedirs(home, exist_ok=True)

    res = {"case": a.case, "harness_argv": sys.argv[1:], "work": os.path.relpath(work, repo),
           "home": os.path.relpath(home, repo), "app_args_template": list(a.app_arg),
           "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
           "phases": {}, "available": {}, "unavailable": {}, "owned": []}
    serve_log = os.path.join(work, "serve.log")
    app_log = os.path.join(work, "app.log")
    series_path = os.path.join(work, "series.jsonl")
    tree_path = os.path.join(work, "tree.jsonl")
    series_f = open(series_path, "w", encoding="utf-8")
    tree_f = open(tree_path, "w", encoding="utf-8")
    tree_dropped = []

    t_ref = time.perf_counter()
    serve = app = None
    ow_serve = ow_app = None
    gpu = GPUProbe() if a.gpu else None
    if gpu and gpu.err:
        res["unavailable"]["gpu_pdh"] = gpu.err
    app_handle = None
    try:
        with open(serve_log, "wb") as sl:
            serve_cmd = [os.path.join(repo, a.serve), "serve", "--home", "<home>",
                         "--addr", "127.0.0.1:0"] + list(a.serve_arg)
            res["serve_argv"] = serve_cmd
            serve = subprocess.Popen([os.path.join(repo, a.serve), "serve", "--home", home,
                                      "--addr", "127.0.0.1:0"] + list(a.serve_arg),
                                     cwd=repo, stdout=subprocess.PIPE, stderr=sl)
        ow_serve = Owned(serve, "serve")
        res["serve_owned"] = ow_serve.snapshot()
        info = read_ready(serve, 120)
        base, token = info["url"], info["token"]
        res["serve_url"] = base
        res["serve_token_sha256_16"] = hashlib.sha256(token.encode()).hexdigest()[:16]
        res["phases"]["serve_ready_ms"] = round((time.perf_counter() - t_ref) * 1000, 1)

        # 时刻 ①（G8）：serve ready 空载 —— 还没有界面、没有会话
        snap = snapshot_processes()
        row = {"t_ms": 0.0, "phase": "serve_ready", "root": serve.pid,
               "kids": 0, "procs": process_tree(serve.pid, snap, tree_dropped)}
        tree_f.write(json.dumps(row, ensure_ascii=False) + "\n")
        tree_f.flush()
        res["phases"]["serve_ready_tree"] = row["procs"]

        # 起界面
        args = [x.replace("{URL}", base).replace("{TOKEN}", token).replace("{SESSION}", a.session)
                for x in a.app_arg]
        if a.shot:
            args += ["-shot", a.shot, "-shot-wait", str(a.shot_wait)]
        res["app_argv"] = [("<token>" if x == token else x) for x in args]
        app_log_f = open(app_log, "wb")
        t0 = time.perf_counter()
        app = subprocess.Popen([os.path.join(repo, a.app)] + args, cwd=repo,
                               stdout=subprocess.DEVNULL, stderr=app_log_f)
        ow_app = Owned(app, "app")
        res["app_owned"] = ow_app.snapshot()
        hp = open_process(app.pid)
        if not hp:
            raise SystemExit("OpenProcess(app) 失败 err=%d" % ctypes.get_last_error())
        app_handle = hp
        applog = LogTail(app_log)
        servetail = LogTail(serve_log)

        marker_at = None
        phase = "app_start"
        turn_children = {}
        last_mem = 0.0
        last_tree = 0.0
        last_win = 0.0
        win_info = None
        last_cpu_chk = t0
        prev_cpu = cpu_ms(hp) or 0.0
        cpu_idle_since = None
        last_child_seen = None
        last_child_first = None
        kids_now = 0
        turn_end_t = None
        settle_deadline = None
        peak = {"ws": 0, "priv": 0, "dedicated": 0}
        first_growth = None
        base_ws = None
        n_mem = n_tree = 0
        deadline = t0 + a.timeout
        exit_reason = ""

        prev_sys = sys_times()
        last_sys_pct = None
        while True:
            now = time.perf_counter()
            tms = round((now - t0) * 1000, 1)
            applog.poll()

            # —— 每 200 ms：把界面窗口挪出屏幕（无头，不向用户弹窗）——
            if now - last_win >= 0.2:
                last_win = now
                ws_ = own_windows(app.pid)
                if ws_:
                    for h, _ww, _hh, _vis in ws_:
                        hide_offscreen(h)
                    if win_info is None:
                        win_info = {"first_rect": [ws_[0][1], ws_[0][2]], "n_windows": len(ws_),
                                    "hidden_at_ms": tms,
                                    "visible_when_found": any(w[3] for w in ws_)}
                    else:
                        win_info["last_rect"] = [ws_[0][1], ws_[0][2]]

            # —— 每 50 ms：界面内存 + WDDM 显存 + CPU 时间 ——
            if now - last_mem >= 0.05:
                last_mem = now
                m = mem_counters(hp)
                row = {"t_ms": tms, "phase": phase, "cpu_ms": round(cpu_ms(hp) or 0.0, 1),
                       "sys_cpu_pct": last_sys_pct}
                if m:
                    row.update(m)
                    n_mem += 1
                    if base_ws is None and m["ws"] > 0:
                        base_ws = m["ws"]
                    if m["ws"] > peak["ws"]:
                        peak["ws"] = m["ws"]
                    if m["priv"] > peak["priv"]:
                        peak["priv"] = m["priv"]
                    if first_growth is None and base_ws and m["ws"] - base_ws > 4 * MB:
                        first_growth = tms
                if gpu:
                    g = gpu.sample(app.pid)
                    row["gpu"] = g
                    if not g.get("error") and g.get("dedicated", 0) > peak["dedicated"]:
                        peak["dedicated"] = g["dedicated"]
                series_f.write(json.dumps(row, ensure_ascii=False) + "\n")

            # —— 每 100 ms：整树（serve 为根）——
            if now - last_tree >= 0.10:
                last_tree = now
                procs = process_tree(serve.pid, None, tree_dropped)
                kids = [p for p in procs if p["depth"] > 0]
                for k in kids:
                    rec = turn_children.setdefault(k["pid"], {"first_t": tms, "name": k["name"]})
                    rec["last_t"] = tms
                    rec["ws_peak"] = max(rec.get("ws_peak", 0), k.get("ws", 0))
                if kids:
                    last_child_seen = tms
                    if last_child_first is None:
                        last_child_first = tms
                kids_now = len(kids)
                tree_f.write(json.dumps({"t_ms": tms, "phase": phase, "root": serve.pid,
                                         "kids": len(kids),
                                         "kids_detail": [{k: v for k, v in p.items() if k != "procs"}
                                                         for p in kids],
                                         "procs": procs}, ensure_ascii=False) + "\n")
                n_tree += 1

            # —— 时刻标记 ——
            if marker_at is None and a.ready_marker and applog.seen(a.ready_marker):
                marker_at = tms
                phase = "turn" if a.case == "turn" else ("observe" if a.case == "idle" else "ready")
                res["phases"]["ready_marker_ms"] = tms
                tree_f.write(json.dumps({"t_ms": tms, "phase": "session_loaded", "root": serve.pid,
                                         "kids": len([p for p in process_tree(serve.pid, None, tree_dropped) if p["depth"] > 0]),
                                         "procs": process_tree(serve.pid, None, tree_dropped)}, ensure_ascii=False) + "\n")
                tree_f.flush()

            # —— CPU 活动：只记进证据，不做判据 ——
            # （实测 turn 结束后界面可能进入 WS 重连风暴、CPU 满，那样“CPU 静默”永远不成立。）
            if now - last_cpu_chk >= 0.2:
                cur = cpu_ms(hp)
                if cur is not None:
                    d = cur - prev_cpu
                    prev_cpu = cur
                    if d < 5.0:
                        if cpu_idle_since is None:
                            cpu_idle_since = now
                    else:
                        cpu_idle_since = None
                last_cpu_chk = now
                cur_sys = sys_times()
                if prev_sys and cur_sys:
                    di = cur_sys[0] - prev_sys[0]
                    dk = cur_sys[1] - prev_sys[1]
                    du = cur_sys[2] - prev_sys[2]
                    tot = dk + du
                    if tot > 0:
                        last_sys_pct = round(100.0 * (tot - di) / tot, 1)
                prev_sys = cur_sys or prev_sys
            cpu_quiet = cpu_idle_since is not None and (now - cpu_idle_since) > 3.0

            # —— turn 结束判据（零改动）：serve 为根的子进程（dshd run）出现 → 消失 + settle ——
            if a.case == "turn":
                servetail.poll()
                if phase == "turn" and res.get("serve_stats_ms") is None and servetail.seen("stats: tool_calls="):
                    res["serve_stats_ms"] = tms
                if phase == "turn" and turn_end_t is None and last_child_seen is not None \
                        and kids_now == 0 and (tms - last_child_seen) > a.turn_settle * 1000.0:
                    turn_end_t = tms
                    phase = "after_turn"
                    res["phases"]["turn_end_ms"] = tms
                    res["phases"]["first_child_ms"] = last_child_first
                    res["phases"]["duration_from_first_child_ms"] = round(tms - (last_child_first or tms), 1)
                    res["turn_end_basis"] = "tree: dshd_run 子进程消失 + settle %.1fs" % a.turn_settle
                    if res.get("serve_stats_ms") is not None:
                        res["turn_end_basis"] += "（serve 日志同轮出现 stats: tool_calls=）"
                    res["cpu_quiet_when_turn_end"] = bool(cpu_quiet)
                    settle_deadline = now + a.observe_after

            # —— 结束条件 ——
            if a.case == "idle":
                if marker_at is not None and now - t0 >= marker_at / 1000.0 + a.observe_after:
                    exit_reason = "observe_done"
            elif a.case == "turn":
                if settle_deadline is not None and now >= settle_deadline:
                    exit_reason = "turn_done+settle"
                elif now - t0 > a.turn_timeout:
                    res["turn_timeout"] = True
                    exit_reason = "turn_timeout"
            else:
                if app.poll() is not None:
                    exit_reason = "app_exited"
            if not exit_reason and app.poll() is not None:
                exit_reason = "app_exited"
            if not exit_reason and now > deadline:
                res["deadline_hit"] = True
                exit_reason = "deadline"
            if exit_reason:
                break
            time.sleep(0.005)

        res["exit_reason"] = exit_reason
        res["window"] = win_info
        end = time.perf_counter()
        res["phases"]["t_end_ms"] = round((end - t0) * 1000, 1)
        res["phases"]["first_growth_ms"] = first_growth
        res["turn_children"] = turn_children
        res["samples_mem"] = n_mem
        res["samples_tree"] = n_tree
        res["peak"] = peak
        res["available"]["series_jsonl"] = os.path.relpath(series_path, repo)
        res["available"]["tree_jsonl"] = os.path.relpath(tree_path, repo)
        sp = os.path.join(repo, a.shot) if a.shot and not os.path.isabs(a.shot) else a.shot
        if a.shot and os.path.exists(sp):
            res["shot"] = {"path": a.shot, "bytes": os.path.getsize(sp),
                           "md5": hashlib.md5(open(sp, "rb").read()).hexdigest()}
        elif a.shot:
            res["shot"] = {"path": a.shot, "exists": False}
    except BaseException as exc:  # noqa
        import traceback
        res["error"] = "%s: %s" % (type(exc).__name__, exc)
        res["traceback"] = traceback.format_exc()
    finally:
        series_f.close()
        tree_f.close()
        if gpu:
            gpu.close()
        if app_handle:
            k32.CloseHandle(app_handle)
        if ow_app is not None:
            try:
                res["owned"].append({"app": ow_app.stop()})
            except Exception as exc:  # noqa
                res["owned"].append({"app_error": repr(exc)})
        if ow_serve is not None:
            try:
                res["owned"].append({"serve": ow_serve.stop()})
            except Exception as exc:  # noqa
                res["owned"].append({"serve_error": repr(exc)})
            try:
                left = process_tree(serve.pid, None, tree_dropped)
                res["leftover_after_stop"] = [p for p in left if p["depth"] > 0]
            except Exception as exc:  # noqa
                res["leftover_after_stop"] = repr(exc)

    res["tree_dropped_pid_reuse"] = tree_dropped
    with open(os.path.join(work, "result.json"), "w", encoding="utf-8") as f:
        json.dump(res, f, ensure_ascii=False, indent=1)
    print(json.dumps({k: res.get(k) for k in
                      ("case", "error", "exit_reason", "phases", "peak", "turn_children",
                       "samples_mem", "samples_tree", "leftover_after_stop", "unavailable")},
                     ensure_ascii=False))


def cmd_pngdiff(a):
    from PIL import Image, ImageChops

    out = {"a": a.a, "b": a.b}
    ia, ib = Image.open(a.a).convert("RGB"), Image.open(a.b).convert("RGB")
    out["size_a"], out["size_b"] = list(ia.size), list(ib.size)
    out["md5_a"] = hashlib.md5(open(a.a, "rb").read()).hexdigest()
    out["md5_b"] = hashlib.md5(open(a.b, "rb").read()).hexdigest()
    if ia.size != ib.size:
        out["same_size"] = False
        out["verdict"] = "FAIL:尺寸不同，无法逐像素比"
        print(json.dumps(out, ensure_ascii=False))
        return
    out["same_size"] = True
    db = ImageChops.difference(ia, ib)
    bbox = db.getbbox()
    if bbox is None:
        out["diff_pixels"] = 0
        out["verdict"] = "PASS:逐像素零差异"
        print(json.dumps(out, ensure_ascii=False))
        return
    px = db.load()
    x0, y0, x1, y1 = bbox
    ndiff = 0
    rows, cols = {}, {}
    for y in range(y0, y1):
        for x in range(x0, x1):
            p = px[x, y]
            if p != (0, 0, 0):
                ndiff += 1
                rows[y] = rows.get(y, 0) + 1
                cols[x] = cols.get(x, 0) + 1
    out["diff_pixels"] = ndiff
    out["total_pixels"] = ia.size[0] * ia.size[1]
    out["diff_ratio"] = round(ndiff / float(ia.size[0] * ia.size[1]), 6)
    out["bbox"] = [x0, y0, x1 - 1, y1 - 1]
    out["diff_rows_top"] = sorted(rows.items(), key=lambda kv: -kv[1])[:8]
    out["diff_cols_top"] = sorted(cols.items(), key=lambda kv: -kv[1])[:8]
    out["diff_row_range"] = [min(rows), max(rows)]
    out["diff_col_range"] = [min(cols), max(cols)]
    out["verdict"] = "FAIL:%d 像素不同" % ndiff
    print(json.dumps(out, ensure_ascii=False))


def main():
    ap = argparse.ArgumentParser()
    sub = ap.add_subparsers(dest="cmd", required=True)

    r = sub.add_parser("run")
    r.add_argument("--repo", required=True)
    r.add_argument("--work", required=True)
    r.add_argument("--app", required=True)
    r.add_argument("--serve", required=True)
    r.add_argument("--home-template", default="")
    r.add_argument("--session", default="")
    r.add_argument("--case", required=True, choices=["idle", "turn", "exit"])
    r.add_argument("--app-arg", action="append", default=[])
    r.add_argument("--serve-arg", action="append", default=[])
    r.add_argument("--ready-marker", default="")
    r.add_argument("--observe-after", type=float, default=10.0)
    r.add_argument("--turn-timeout", type=float, default=180.0)
    r.add_argument("--turn-settle", type=float, default=4.0)
    r.add_argument("--timeout", type=float, default=300.0)
    r.add_argument("--shot", default="")
    r.add_argument("--shot-wait", type=int, default=6)
    r.add_argument("--gpu", action="store_true")
    r.set_defaults(func=cmd_run)

    p = sub.add_parser("pngdiff")
    p.add_argument("--a", required=True)
    p.add_argument("--b", required=True)
    p.set_defaults(func=cmd_pngdiff)

    a = ap.parse_args()
    a.func(a)


if __name__ == "__main__":
    main()
PERF_GAP_HARNESS_EOF
  cat > "$WORK/summarize.py" <<'PERF_GAP_SUMMARIZE_EOF'
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""perf-gap 汇总：只读 tmp/perf-gap/<case>/result.json + series.jsonl + tree.jsonl，
算出 G1/G6/G7/G8 的汇总（不重新测量，不改任何证据文件）。

口径（docs/perf-gap-plan.md §2）：
  * 静置段 = 最后 1 s 的中位数；
  * 峰值 = 整个观察窗口的最大值；
  * 内存 ws / priv 分列，显存 dedicated/local/shared/nonlocal 分列；
  * 跨进程 3 次取中位数（并给 min/max）；
  * 单位统一 MB（1048576 字节）。
"""

from __future__ import annotations

import glob
import json
import os
import statistics
import sys

MB = 1048576.0


def load_json(p):
    try:
        with open(p, encoding="utf-8") as f:
            return json.load(f)
    except (OSError, ValueError):
        return None


def load_jsonl(p):
    out = []
    try:
        with open(p, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line:
                    out.append(json.loads(line))
    except (OSError, ValueError):
        pass
    return out


def mb(v):
    return None if v is None else round(v / MB, 1)


def median(xs):
    xs = [x for x in xs if x is not None]
    return round(statistics.median(xs), 1) if xs else None


def rng(xs):
    xs = [x for x in xs if x is not None]
    if not xs:
        return [None, None]
    return [round(min(xs), 1), round(max(xs), 1)]


def steady(rows, phase=None, window_ms=1000):
    """静置段：最后 window_ms 内的中位数（ws/priv + 4 个显存量）。"""
    rs = [r for r in rows if phase is None or r.get("phase") == phase]
    if not rs:
        return None
    end = rs[-1]["t_ms"]
    win = [r for r in rs if r["t_ms"] >= end - window_ms] or rs[-1:]
    out = {"n": len(win)}
    for k in ("ws", "priv"):
        out[k] = mb(statistics.median([r[k] for r in win if k in r])) if any(k in r for r in win) else None
    gp = [r["gpu"] for r in win if isinstance(r.get("gpu"), dict) and not r["gpu"].get("error")]
    for k in ("dedicated", "local", "shared", "nonlocal"):
        out[k] = mb(statistics.median([g[k] for g in gp])) if gp else None
    out["gpu_present"] = bool(gp and any(g.get("present") for g in gp))
    return out


def case_dir(work, name):
    return os.path.join(work, name)


def one_run(work, name):
    d = case_dir(work, name)
    r = load_json(os.path.join(d, "result.json"))
    if not r:
        return None
    s = load_jsonl(os.path.join(d, "series.jsonl"))
    t = load_jsonl(os.path.join(d, "tree.jsonl"))
    return {"name": name, "result": r, "series": s, "tree": t, "dir": d}


def g8_points(run):
    """G8：四个时刻的整树（serve 为根）。"""
    r = run["result"]
    out = {}
    ready = r.get("phases", {}).get("serve_ready_tree") or []
    if ready:
        out["serve_ready"] = {"ws": mb(ready[0].get("ws")), "priv": mb(ready[0].get("priv")),
                              "n": len(ready)}
    loaded = [x for x in run["tree"] if x.get("phase") == "session_loaded"]
    if loaded:
        p = loaded[0]["procs"]
        out["session_loaded"] = {"ws": mb(p[0].get("ws")), "priv": mb(p[0].get("priv")), "n": len(p)}
    turns = [x for x in run["tree"] if x.get("kids")]
    if turns:
        best = max(turns, key=lambda x: sum(q.get("ws", 0) for q in x["procs"]))
        out["turn_in_flight"] = {
            "tree_ws": mb(sum(q.get("ws", 0) for q in best["procs"])),
            "tree_priv": mb(sum(q.get("priv", 0) for q in best["procs"])),
            "n": len(best["procs"]),
            "procs": [{"name": q["name"], "depth": q["depth"], "ws": mb(q.get("ws")),
                       "priv": mb(q.get("priv"))} for q in best["procs"]],
        }
    if run["tree"]:
        last = run["tree"][-1]
        out["after_turn"] = {
            "ws": mb(last["procs"][0].get("ws")), "priv": mb(last["procs"][0].get("priv")),
            "n": len(last["procs"]), "phase": last.get("phase"),
        }
    return out


def main():
    work = sys.argv[1] if len(sys.argv) > 1 else "tmp/perf-gap"
    meta = load_json(os.path.join(work, "meta.json")) or {}
    out = {"meta": meta, "G1": {}, "G6": {}, "G7": {}, "G8": {}, "runs": {}}

    # ── G1 / G8-idle：tail=200 三轮 + tail=0 两轮 ──
    idle200 = [x for x in (one_run(work, "idle-%d" % i) for i in (1, 2, 3)) if x]
    idle0 = [x for x in (one_run(work, "idle-tail0-%d" % i) for i in (1, 2)) if x]
    out["G1"]["idle_tail200"] = []
    for x in idle200:
        st = steady(x["series"], "observe")
        peak_ded = max([r["gpu"]["dedicated"] for r in x["series"]
                        if isinstance(r.get("gpu"), dict) and not r["gpu"].get("error")] or [0])
        out["G1"]["idle_tail200"].append({
            "run": x["name"], "steady": st, "peak_dedicated_mb": mb(peak_ded),
            "samples": len(x["series"]), "exit": x["result"].get("exit_reason"),
            "window_rect": (x["result"].get("window") or {}).get("last_rect"),
        })
    out["G1"]["idle_tail0"] = []
    for x in idle0:
        st = steady(x["series"], "observe")
        out["G1"]["idle_tail0"].append({"run": x["name"], "steady": st,
                                        "samples": len(x["series"]),
                                        "exit": x["result"].get("exit_reason")})
    if idle200 and idle0:
        pick = lambda rows, k: median([r["steady"][k] for r in rows if r.get("steady")])
        r200, r0 = out["G1"]["idle_tail200"], out["G1"]["idle_tail0"]
        a = {"dedicated": pick(r200, "dedicated"), "local": pick(r200, "local"),
             "shared": pick(r200, "shared"), "nonlocal": pick(r200, "nonlocal"),
             "ws": pick(r200, "ws"), "priv": pick(r200, "priv")}
        b = {"dedicated": pick(r0, "dedicated"), "local": pick(r0, "local"),
             "shared": pick(r0, "shared"), "nonlocal": pick(r0, "nonlocal"),
             "ws": pick(r0, "ws"), "priv": pick(r0, "priv")}
        out["G1"]["steady_median"] = {"tail200": a, "tail0": b,
                                      "delta_tail200_minus_tail0": {
                                          k: (None if a[k] is None or b[k] is None
                                              else round(a[k] - b[k], 1)) for k in a}}
        out["G1"]["steady_range_tail200"] = {k: rng([r["steady"][k] for r in r200 if r.get("steady")])
                                             for k in a}

    # ── G6：真模型轮 ──
    def turn_runs(prefix):
        runs = []
        for d in sorted(glob.glob(os.path.join(work, prefix + "*"))):
            name = os.path.basename(d)
            if not os.path.isdir(d):
                continue
            x = one_run(work, name)
            if x:
                runs.append(x)
        return runs

    for label, prefix in (("text", "turn-text-"), ("tool", "turn-tool-")):
        rows = []
        for x in turn_runs(prefix):
            r = x["result"]
            if not r:
                continue
            peak = r.get("peak") or {}
            after = steady(x["series"], "after_turn")
            before = steady(x["series"], "turn", 1000) if False else None
            child = [v for v in (r.get("turn_children") or {}).values()]
            rows.append({
                "run": x["name"], "exit": r.get("exit_reason"), "error": r.get("error"),
                "turn_timeout": bool(r.get("turn_timeout")),
                "serve_ready_ms": r.get("phases", {}).get("serve_ready_ms"),
                "first_child_ms": r.get("phases", {}).get("first_child_ms"),
                "turn_end_ms": r.get("phases", {}).get("turn_end_ms"),
                "turn_duration_ms": r.get("phases", {}).get("duration_from_first_child_ms"),
                "peak_ws_mb": mb(peak.get("ws")), "peak_priv_mb": mb(peak.get("priv")),
                "peak_dedicated_mb": mb(peak.get("dedicated")),
                "after_turn": after,
                "child_ws_peak_mb": mb(max([c.get("ws_peak", 0) for c in child] or [0])),
                "leftover": r.get("leftover_after_stop"),
                "serve_stats_ms": r.get("serve_stats_ms"),
            })
        out["G6"][label] = rows
        ok = [r for r in rows if not r["turn_timeout"]]
        out["G6"][label + "_median"] = {
            "n_valid": len(ok), "n_total": len(rows),
            "turn_duration_ms": median([r["turn_duration_ms"] for r in ok]),
            "turn_duration_range": rng([r["turn_duration_ms"] for r in ok]),
            "peak_ws_mb": median([r["peak_ws_mb"] for r in ok]),
            "peak_ws_range": rng([r["peak_ws_mb"] for r in ok]),
            "peak_priv_mb": median([r["peak_priv_mb"] for r in ok]),
            "peak_priv_range": rng([r["peak_priv_mb"] for r in ok]),
            "peak_dedicated_mb": median([r["peak_dedicated_mb"] for r in ok]),
            "child_ws_peak_mb": median([r["child_ws_peak_mb"] for r in ok]),
        }
    # 发送前静置：用 idle 轮的跨轮值（零改动拿不到同一进程的发送前瞬时值，见文档）
    if idle200:
        out["G6"]["before_send_reference"] = {
            "source": "idle-1..3 的 observe 段最后 1 s 中位数（同一二进制/同一 home 副本/同一 tail）",
            "ws_mb": median([r["steady"]["ws"] for r in out["G1"]["idle_tail200"] if r["steady"]]),
            "priv_mb": median([r["steady"]["priv"] for r in out["G1"]["idle_tail200"] if r["steady"]]),
            "dedicated_mb": median([r["steady"]["dedicated"] for r in out["G1"]["idle_tail200"] if r["steady"]]),
        }
    out["G6"]["unavailable"] = {
        "first_byte_ms": "零改动无「首字节」时标：app 侧没有该日志（streamDebug 只在有思考且无正文时打），"
                         "serve 侧 turn 期间只打 spawn/follow，没有首帧正文事件；不拿设计值代替。",
        "ttft": "同上（TTFT 需要 -trace 打点，属 G2/G6 的「要打点」半边）。",
    }

    # ── G7 ──
    mat = load_json(os.path.join(work, "g7-matrix.json"))
    out["G7"] = mat or {}
    out["G7"]["tail40_control"] = {"note": "tail=40 参考轮消息区唯一色数=1（同 tail=200 参考轮），见文档"}
    out["G7"]["runs"] = []
    for i in (1, 2, 3):
        ref = one_run(work, "g7-%d/ref" % i)
        sw = one_run(work, "g7-%d/sw" % i)
        if ref or sw:
            out["G7"]["runs"].append({
                "round": i,
                "ref_shot": (ref or {}).get("result", {}).get("shot"),
                "sw_shot": (sw or {}).get("result", {}).get("shot"),
                "sw_last_session": None,
            })

    # ── G8：整树（idle 与 turn 轮都算） ──
    out["G8"]["idle"] = [{"run": x["name"], **g8_points(x)} for x in idle200]
    out["G8"]["turn_text"] = [{"run": x["name"], **g8_points(x)}
                              for x in turn_runs("turn-text-")]
    out["G8"]["turn_tool"] = [{"run": x["name"], **g8_points(x)}
                              for x in turn_runs("turn-tool-")]
    # 界面单进程 + 整树相加（turn 结束时）
    combined = []
    for x in idle200:
        p = g8_points(x)
        st = steady(x["series"], "observe") or {}
        if p.get("after_turn") and st.get("ws"):
            combined.append({"run": x["name"],
                             "ui_ws": st["ws"], "tree_ws": p["after_turn"]["ws"],
                             "ui_priv": st["priv"], "tree_priv": p["after_turn"]["priv"]})
    out["G8"]["idle_combined"] = combined

    with open(os.path.join(work, "summary.json"), "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=1)
    print(json.dumps(out, ensure_ascii=False, indent=1))


if __name__ == "__main__":
    main()
PERF_GAP_SUMMARIZE_EOF
  cat > "$WORK/pngmatrix.py" <<'PERF_GAP_PNGMATRIX_EOF'
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""G7 的比对矩阵：把 <work>/g7-<i>/{ref,switched}.png 两两比对，落 g7-matrix.json。

判据（docs/perf-gap-plan.md §G7）：参考轮（直接打开 primary）与切换轮（切走 5 个再切回）
应该是**同一张图**。这里同时报三类：
  * ref×ref  —— 同一场景两次独立进程的自身噪声（判据的噪声底）
  * sw×sw    —— 切换轮之间的自身噪声
  * ref×sw   —— 判据本身
另外把差异里「标题栏 y<34」的部分单独扣出来（窗口激活态会改标题栏配色，属于抓图噪声）。
"""

import argparse
import json
import os

from PIL import Image, ImageChops

LUT = [0] + [255] * 255


def diff(path_a, path_b):
    ia = Image.open(path_a).convert("RGB")
    ib = Image.open(path_b).convert("RGB")
    out = {"size_a": list(ia.size), "size_b": list(ib.size)}
    if ia.size != ib.size:
        out.update({"same_size": False, "diff_pixels": None})
        return out
    out["same_size"] = True
    db = ImageChops.difference(ia, ib)
    bbox = db.getbbox()
    if bbox is None:
        out.update({"diff_pixels": 0, "bbox": None, "diff_ratio": 0.0,
                    "diff_in_titlebar_y_lt_34": 0, "diff_outside_titlebar": 0})
        return out
    r, g, b = db.split()
    m = ImageChops.lighter(r.point(LUT), ImageChops.lighter(g.point(LUT), b.point(LUT)))
    n = m.histogram()[255]
    w, h = ia.size
    title = m.crop((0, 0, w, 34)).histogram()[255]
    out.update({"diff_pixels": n, "bbox": list(bbox), "diff_ratio": round(n / float(w * h), 6),
                "diff_in_titlebar_y_lt_34": title, "diff_outside_titlebar": n - title,
                "total_pixels": w * h})
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--work", required=True)
    ap.add_argument("--runs", type=int, default=3)
    a = ap.parse_args()

    files = {}
    for i in range(1, a.runs + 1):
        for tag, name in (("ref", "ref.png"), ("sw", "switched.png")):
            p = os.path.join(a.work, "g7-%d" % i, name)
            if os.path.exists(p):
                files["%s%d" % (tag, i)] = p

    out = {"work": a.work, "files": files, "pairs": {}}
    names = list(files)
    for i in range(len(names)):
        for j in range(i + 1, len(names)):
            x, y = names[i], names[j]
            out["pairs"]["%s|%s" % (x, y)] = diff(files[x], files[y])

    groups = {"ref_vs_ref": [], "sw_vs_sw": [], "ref_vs_sw": []}
    for k, v in out["pairs"].items():
        x, y = k.split("|")
        n = v.get("diff_pixels")
        if n is None:
            continue
        if x.startswith("ref") and y.startswith("ref"):
            groups["ref_vs_ref"].append(n)
        elif x.startswith("sw") and y.startswith("sw"):
            groups["sw_vs_sw"].append(n)
        else:
            groups["ref_vs_sw"].append(n)
    out["groups"] = groups
    out["verdict"] = {
        "ref_vs_ref": "自身噪声底：%s" % groups["ref_vs_ref"],
        "sw_vs_sw": "自身噪声底：%s" % groups["sw_vs_sw"],
        "ref_vs_sw": ("PASS:逐像素零差异" if groups["ref_vs_sw"] and
                      all(n == 0 for n in groups["ref_vs_sw"])
                      else "FAIL:切走再切回与直接打开不同：%s" % groups["ref_vs_sw"]),
    }
    with open(os.path.join(a.work, "g7-matrix.json"), "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=1)
    print(json.dumps({"files": files, "groups": groups, "verdict": out["verdict"]},
                     ensure_ascii=False))


if __name__ == "__main__":
    main()
PERF_GAP_PNGMATRIX_EOF
}
write_helpers

# ── 1. 环境闸门：负载 / Go 构建进程 / magpie ────────────────────────────────
probe_load() {
  "$PY" - <<'PERF_GAP_LOAD_EOF'
import ctypes, time
from ctypes import wintypes
k32 = ctypes.WinDLL("kernel32")
class FT(ctypes.Structure):
    _fields_ = [("lo", wintypes.DWORD), ("hi", wintypes.DWORD)]
def st():
    i, k, u = FT(), FT(), FT()
    k32.GetSystemTimes(ctypes.byref(i), ctypes.byref(k), ctypes.byref(u))
    q = lambda f: (f.hi << 32) | f.lo
    return q(i), q(k), q(u)
a = st(); time.sleep(1.0); b = st()
di, dk, du = b[0] - a[0], b[1] - a[1], b[2] - a[2]
tot = dk + du
print("%.1f" % (100.0 * (tot - di) / tot if tot > 0 else 0.0))
PERF_GAP_LOAD_EOF
}

probe_goproc() {
  "$PY" - <<'PERF_GAP_GOPROC_EOF'
import ctypes
from ctypes import wintypes
k32 = ctypes.WinDLL("kernel32")
TH32CS_SNAPPROCESS = 0x2
class PE(ctypes.Structure):
    _fields_ = [("dwSize", wintypes.DWORD), ("cntUsage", wintypes.DWORD),
                ("th32ProcessID", wintypes.DWORD), ("th32DefaultHeapID", ctypes.POINTER(ctypes.c_ulong)),
                ("th32ModuleID", wintypes.DWORD), ("cntThreads", wintypes.DWORD),
                ("th32ParentProcessID", wintypes.DWORD), ("pcPriClassBase", wintypes.LONG),
                ("dwFlags", wintypes.DWORD), ("szExeFile", ctypes.c_wchar * 260)]
snap = k32.CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0)
e = PE(); e.dwSize = ctypes.sizeof(PE)
hit = []
ok = k32.Process32FirstW(snap, ctypes.byref(e))
while ok:
    n = e.szExeFile.lower()
    if n.startswith("go") or "compile" in n or n.startswith("link"):
        hit.append("%s:%d" % (n, e.th32ProcessID))
    ok = k32.Process32NextW(snap, ctypes.byref(e))
k32.CloseHandle(snap)
print(len(hit), " ".join(hit[:8]))
PERF_GAP_GOPROC_EOF
}

LOAD="$(probe_load)"
GOINFO="$(probe_goproc)"
GOCOUNT="${GOINFO%% *}"
log "== 环境：系统 CPU 占用 ${LOAD}%（1 s 窗口）；疑似 Go 构建/编译/链接进程 ${GOCOUNT} 个 ${GOINFO#* }"
if [ "${LOAD%%.*}" -gt "$LOAD_MAX" ]; then
  log "!! 负载 ${LOAD}% > 阈值 ${LOAD_MAX}%：压测/编译期间报性能数会失真（任务书要求不与压测同时报）"
  if [ "$LOAD_STRICT" = 1 ]; then log "PERF_GAP_LOAD_STRICT=1 → 退出 3"; exit 3; fi
fi

# ── 2. 二进制：界面用本仓源码；宿主用指定 rev 的**干净导出**构建 ─────────────────
abs_path() { case "$1" in /*|[A-Za-z]:*) printf '%s' "$1";; *) printf '%s/%s' "$ROOT" "$1";; esac; }
APP_ABS="$(abs_path "$APP")"
SERVE_ABS="$(abs_path "$SERVE")"

build_app() {
  if [ -x "$APP_ABS" ] && [ "$REUSE" = 1 ]; then log "== 复用界面二进制 $APP"; return 0; fi
  log "== 构建界面 $APP（ui rev $(git rev-parse --short HEAD 2>/dev/null || echo '?')）"
  CGO_ENABLED=0 go build -o "$APP_ABS" ./cmd/app
}

build_serve() {
  if [ -x "$SERVE_ABS" ] && [ "$REUSE" = 1 ]; then log "== 复用宿主二进制 $SERVE"; return 0; fi
  local src="$WORK/host-$HOST_REV"
  log "== 从 $HOST_REPO 导出 rev $HOST_REV 的干净源码到 $src 后构建（工作树可能有并行 owner 的未提交改动）"
  rm -rf "$src"
  mkdir -p "$src"
  ( cd "$HOST_REPO" && git archive "$HOST_REV" | tar -x -C "$ROOT/$src" )
  mkdir -p "$(dirname "$SERVE_ABS")"
  ( cd "$src" && CGO_ENABLED=0 go build -o "$SERVE_ABS" ./cmd/dshd )
}

build_app
build_serve

# ── 3. 临时 home：真实会话的只读副本（绝不写真实 home）────────────────────────
prepare_home() {
  if [ -f "$WORK/template-meta.json" ] && [ -d "$TPL/sessions" ] && [ "$REUSE" = 1 ]; then
    log "== 复用会话副本模板 $TPL"
    return 0
  fi
  log "== 从 $REAL_HOME 复制 $SESSIONS 份会话（每份 ≤ ${MAX_SESSION_MB} MB，id 各不相同）到 $TPL"
  "$PY" - "$REAL_HOME" "$MAX_SESSION_MB" "$SESSIONS" "$TPL" "$WORK" <<'PERF_GAP_PREP_EOF'
import json, os, shutil, sys
real, max_mb, want, tpl, work = sys.argv[1], float(sys.argv[2]), int(sys.argv[3]), sys.argv[4], sys.argv[5]
MB = 1048576.0
root = os.path.join(real, "sessions")
rows = []
for proj in os.listdir(root):
    p = os.path.join(root, proj)
    if not os.path.isdir(p):
        continue
    for sid in os.listdir(p):
        f = os.path.join(p, sid, "session.v4.jsonl.zstd")
        if os.path.isfile(f):
            rows.append((os.path.getsize(f), proj, sid))
rows.sort(reverse=True)
picked, seen = [], set()
for size, proj, sid in rows:
    if sid in seen or size > max_mb * MB:
        continue
    seen.add(sid)
    picked.append((size, proj, sid))
    if len(picked) >= want:
        break
if not picked:
    raise SystemExit("没有可用会话（都超过 %.1f MB）" % max_mb)
if os.path.exists(tpl):
    shutil.rmtree(tpl)
os.makedirs(os.path.join(tpl, "sessions"), exist_ok=True)
sessions = []
for size, proj, sid in picked:
    dst = os.path.join(tpl, "sessions", proj, sid)
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    shutil.copytree(os.path.join(root, proj, sid), dst)
    sessions.append({"project": proj, "sessionId": sid, "bytes": size})
info = {"sessions": sessions, "primary": sessions[0]["sessionId"],
        "primary_project": sessions[0]["project"],
        "total_bytes": sum(s["bytes"] for s in sessions),
        "all_sessions_in_real_home": len(rows), "source_home": real,
        "max_session_mb": max_mb}
with open(os.path.join(work, "template-meta.json"), "w", encoding="utf-8") as f:
    json.dump(info, f, ensure_ascii=False, indent=1)
print("picked %d sessions; primary=%s (%.2f MB); total %.1f MB"
      % (len(sessions), sessions[0]["sessionId"], sessions[0]["bytes"] / MB,
         info["total_bytes"] / MB), file=sys.stderr)
PERF_GAP_PREP_EOF
}

prepare_home
PRIMARY="$("$PY" -c 'import json,sys;print(json.load(open(sys.argv[1],encoding="utf-8"))["primary"])' "$WORK/template-meta.json")"
log "== 参考会话（primary）：$PRIMARY"

# ── 4. 单个场景的执行器 ────────────────────────────────────────────────────
run_case() { # name case 额外参数...
  local name="$1" kind="$2"
  shift 2
  if [ "$REUSE" = 1 ] && [ -f "$WORK/$name/result.json" ]; then
    log "   [skip] $name 已有证据（PERF_GAP_REUSE=1）"
    return 0
  fi
  mkdir -p "$WORK/$name"
  log "   [run ] $name（case=$kind）"
  "$PY" "$WORK/harness.py" run --repo "$ROOT" --work "$WORK/$name" \
    --app "$APP_ABS" --serve "$SERVE_ABS" --home-template "$TPL" --session "$PRIMARY" \
    --case "$kind" --ready-marker 'mux 已订阅' "$@" \
    >>"$WORK/$name/stdout.txt" 2>&1 \
    || log "   [fail] $name 退出码 $?（看 $WORK/$name/stdout.txt）"
}

APP_BASE=(--app-arg=-addr --app-arg={URL} --app-arg=-token --app-arg={TOKEN}
          --app-arg=-session --app-arg={SESSION} --app-arg=-tail
          --app-arg=-title --app-arg=dsh-ui-perf0)

# G1 / G8-idle：会话已载入 → 静置 → 采样（界面内存+显存 / 整树）
if have G1 || have G8; then
  log "== G1/G8 idle：-tail 200 静置 ${IDLE}s（界面 ws/priv + WDDM 显存 + 整树），$RUNS 轮"
  for i in $(seq 1 "$RUNS"); do
    run_case "idle-$i" idle --observe-after "$IDLE" --timeout 180 --gpu \
      "${APP_BASE[@]}" --app-arg=200
  done
fi

# G1 的差分项：-tail 0 vs -tail 200（推断「内容纹理增量†」用；不能当实测增量）
if have G1; then
  log "== G1 差分：-tail 0 静置 ${IDLE}s，2 轮"
  for i in 1 2; do
    run_case "idle-tail0-$i" idle --observe-after "$IDLE" --timeout 180 --gpu \
      "${APP_BASE[@]}" --app-arg=0
  done
fi

# G6 / G8-turn：真模型一轮（纯文本 + 带工具）
if have G6 || have G8; then
  if [ "$REAL" != 1 ]; then
    log "== G6/G8-真轮 跳过：PERF_GAP_REAL != 1（真模型轮消耗真实额度、依赖 magpie；需要时显式打开）"
  else
    CODE="$(curl.exe -s -o /dev/null -w '%{http_code}' "$BASE_URL/models" 2>/dev/null || echo 000)"
    if [ "$CODE" != 200 ]; then
      log "!! magpie $BASE_URL/models 返回 $CODE → G6/G8-真轮记 unavailable（不硬跑）"
    else
      log "== G6/G8 真模型轮：纯文本 $RUNS 轮"
      for i in $(seq 1 "$RUNS"); do
        run_case "turn-text-$i" turn --observe-after "$IDLE" --turn-timeout "$TURN_TIMEOUT" \
          --turn-settle 4 --timeout $((TURN_TIMEOUT + 60)) --gpu \
          "${APP_BASE[@]}" --app-arg=200 --app-arg=-smoke --app-arg=-prompt --app-arg="$TEXT_PROMPT"
      done
      log "== G6/G8 真模型轮：带工具 $RUNS 轮（serve --policy allow，工具=echo hi，无副作用）"
      for i in $(seq 1 "$RUNS"); do
        run_case "turn-tool-$i" turn --observe-after "$IDLE" --turn-timeout "$TURN_TIMEOUT" \
          --turn-settle 5 --timeout $((TURN_TIMEOUT + 60)) --gpu \
          --serve-arg=--policy --serve-arg=allow \
          "${APP_BASE[@]}" --app-arg=200 --app-arg=-smoke --app-arg=-prompt --app-arg="$TOOL_PROMPT"
      done
    fi
  fi
fi

# G7：参考轮（直接打开 primary） vs 切换轮（切 5 个会话再切回 primary）
if have G7; then
  SW_SEQ="$("$PY" - "$WORK/template-meta.json" <<'PERF_GAP_SEQ_EOF'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
ids = [s["sessionId"] for s in m["sessions"]]
p = m["primary"]
print(",".join([i for i in ids if i != p] + [p]))
PERF_GAP_SEQ_EOF
)"
  log "== G7：参考轮 vs 切换轮（-switch $SW_SEQ -switch-every 1），$RUNS 轮"
  log "   切换轮末尾必须回到 primary：判据要求「切走再切回」后与直接打开一致"
  for i in $(seq 1 "$RUNS"); do
    run_case "g7-$i/ref" exit --timeout 120 --shot "$WORK/g7-$i/ref.png" --shot-wait 12 \
      "${APP_BASE[@]}" --app-arg=200
    run_case "g7-$i/sw" exit --timeout 150 --shot "$WORK/g7-$i/switched.png" --shot-wait 13 \
      "${APP_BASE[@]}" --app-arg=200 --app-arg=-switch --app-arg="$SW_SEQ" \
      --app-arg=-switch-every --app-arg=1
  done
  "$PY" "$WORK/pngmatrix.py" --work "$WORK" --runs "$RUNS" > "$WORK/g7-matrix.stdout" 2>&1 \
    || log "!! G7 比对失败（看 $WORK/g7-matrix.stdout）"
fi

# ── 5. meta + 汇总 ────────────────────────────────────────────────────────
"./$PY" --version >/dev/null 2>&1 || true
"$PY" - "$WORK" "$UI_REV" "$APP" "$SERVE" "$HOST_REV" "$LOAD" "$GOCOUNT" "$REAL" \
  "$RUNS" "$IDLE" "$SESSIONS" "$MAX_SESSION_MB" "$TEXT_PROMPT" "$TOOL_PROMPT" "$ONLY" <<'PERF_GAP_META_EOF'
import hashlib, json, os, sys, time
(work, ui_rev, app, serve, host_rev, load, gocount, real, runs, idle,
 sessions, max_mb, text_prompt, tool_prompt, only) = sys.argv[1:16]


def sha(p):
    try:
        h = hashlib.sha256()
        with open(p, "rb") as f:
            for chunk in iter(lambda: f.read(1 << 20), b""):
                h.update(chunk)
        return h.hexdigest()
    except OSError:
        return None


meta = {
    "generated_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
    "ui_rev": ui_rev, "host_rev": host_rev,
    "app": app, "app_sha256": sha(app),
    "serve": serve, "serve_sha256": sha(serve),
    "only": only, "runs": int(runs), "idle_s": float(idle),
    "sessions": int(sessions), "max_session_mb": float(max_mb),
    "perf_gap_real": real,
    "load_pct_at_start": float(load), "go_build_procs_at_start": int(gocount),
    "text_prompt_sha256_16": hashlib.sha256(text_prompt.encode()).hexdigest()[:16],
    "tool_prompt_sha256_16": hashlib.sha256(tool_prompt.encode()).hexdigest()[:16],
    "note": "所有数字来自进程外采样（PDH/K32GetProcessMemoryInfo/Toolhelp32）；"
            "app/serve 的 sha256 对应上面两个 rev 的构建；宿主来自 rev 的干净 git archive 导出。",
}
with open(os.path.join(work, "meta.json"), "w", encoding="utf-8") as f:
    json.dump(meta, f, ensure_ascii=False, indent=1)
print(json.dumps(meta, ensure_ascii=False))
PERF_GAP_META_EOF

log "== 汇总（summary.json / summary.md）"
"$PY" "$WORK/summarize.py" "$WORK" > "$WORK/summary.json.stdout" 2>&1 \
  || log "!! summarize.py 失败（看 $WORK/summary.json.stdout）"

"$PY" - "$WORK" <<'PERF_GAP_MD_EOF' > "$WORK/summary.md" 2>"$WORK/summary.md.stderr" || true
import json, os, sys
work = sys.argv[1]
s = json.load(open(os.path.join(work, "summary.json"), encoding="utf-8"))
m = s.get("meta") or {}


def row(cells):
    return "| " + " | ".join("" if c is None else str(c) for c in cells) + " |"


def head(cells):
    print(row(cells))
    print("|" + "---|" * len(cells))


print("# perf-gap 汇总（零改动四项）")
print()
print("- 生成时间：%s" % m.get("generated_at"))
print("- ui rev `%s`（app sha256 `%s`）" % (m.get("ui_rev"), (m.get("app_sha256") or "")[:16]))
print("- host rev `%s`（serve sha256 `%s`，来自该 rev 的干净 git archive）"
      % (m.get("host_rev"), (m.get("serve_sha256") or "")[:16]))
print("- 起始负载：系统 CPU %s%%、Go 构建进程 %s 个；runs=%s idle=%ss"
      % (m.get("load_pct_at_start"), m.get("go_build_procs_at_start"),
         m.get("runs"), m.get("idle_s")))
print("- 真模型轮：PERF_GAP_REAL=%s" % m.get("perf_gap_real"))
print()
print("## G1 · 界面进程 WDDM 显存（静置段最后 1 s 中位数，MB）")
print()
head(["场景", "Dedicated", "Local", "Shared", "Non Local", "WorkingSet", "PrivateUsage", "轮数"])
sm = s["G1"].get("steady_median") or {}
a, b = sm.get("tail200") or {}, sm.get("tail0") or {}
print(row(["`-tail 200`"] + [a.get(k) for k in ("dedicated", "local", "shared", "nonlocal", "ws", "priv")]
          + [len(s["G1"].get("idle_tail200") or [])]))
print(row(["`-tail 0`"] + [b.get(k) for k in ("dedicated", "local", "shared", "nonlocal", "ws", "priv")]
          + [len(s["G1"].get("idle_tail0") or [])]))
d = sm.get("delta_tail200_minus_tail0") or {}
print(row(["差分（tail200 − tail0，**推断†**）"] + [d.get(k) for k in
          ("dedicated", "local", "shared", "nonlocal", "ws", "priv")] + ["—"]))
print()
print("跨轮范围（tail200）：%s" % json.dumps(s["G1"].get("steady_range_tail200") or {}, ensure_ascii=False))
print()
print("## G6 · 真模型一轮（界面进程，MB）")
print()
head(["轮次", "turn 时长 ms", "峰值 WS", "峰值 PrivateUsage", "峰值 Dedicated", "结束后静置 WS",
      "`dshd run` 峰值 WS", "退出"])
for label in ("text", "tool"):
    for r in s["G6"].get(label) or []:
        at = r.get("after_turn") or {}
        print(row([r["run"], r.get("turn_duration_ms"), r.get("peak_ws_mb"), r.get("peak_priv_mb"),
                   r.get("peak_dedicated_mb"), at.get("ws"), r.get("child_ws_peak_mb"),
                   r.get("exit")]))
print()
for label in ("text", "tool"):
    md = s["G6"].get(label + "_median") or {}
    print("- %s：有效 %s/%s 轮；turn 时长中位 %s ms（%s）；峰值 WS 中位 %s MB（%s）；"
          "峰值 priv 中位 %s MB；峰值 dedicated 中位 %s MB；run 子进程 WS 中位 %s MB"
          % (label, md.get("n_valid"), md.get("n_total"), md.get("turn_duration_ms"),
             md.get("turn_duration_range"), md.get("peak_ws_mb"), md.get("peak_ws_range"),
             md.get("peak_priv_mb"), md.get("peak_dedicated_mb"), md.get("child_ws_peak_mb")))
bs = s["G6"].get("before_send_reference") or {}
print("- 发送前静置（跨轮参照：同样的二进制/home 副本/tail 的 idle 轮）：WS %s / priv %s / dedicated %s MB"
      % (bs.get("ws_mb"), bs.get("priv_mb"), bs.get("dedicated_mb")))
print()
print("## G7 · 五会话切换（切走再切回 vs 直接打开，1196×839 逐像素）")
print()
pairs = (s.get("G7") or {}).get("pairs") or {}
kinds = {"ref_vs_ref": [], "sw_vs_sw": [], "ref_vs_sw": []}
import re
for k, v in pairs.items():
    a_, b_ = k.split("|")
    n = v.get("diff_pixels")
    if a_.startswith("ref") and b_.startswith("ref"):
        kinds["ref_vs_ref"].append(n)
    elif a_.startswith("sw") and b_.startswith("sw"):
        kinds["sw_vs_sw"].append(n)
    else:
        kinds["ref_vs_sw"].append(n)
for k, v in kinds.items():
    print("- %s：%d 对，diff_pixels = %s" % (k, len(v), v))
print()
print("## G8 · 宿主整进程树（以自己起的 serve PID 为根，MB）")
print()
head(["轮次", "serve 空载 WS/priv", "会话载入后 WS/priv", "一轮中整树 WS/priv（进程数）",
      "一轮结束后 WS/priv", "结束后进程数"])
for group in ("idle", "turn_text", "turn_tool"):
    for r in s["G8"].get(group) or []:
        ready = r.get("serve_ready") or {}
        loaded = r.get("session_loaded") or {}
        tf = r.get("turn_in_flight") or {}
        after = r.get("after_turn") or {}
        print(row([r["run"],
                   "%s / %s" % (ready.get("ws"), ready.get("priv")),
                   "%s / %s" % (loaded.get("ws"), loaded.get("priv")),
                   "%s / %s" % (tf.get("tree_ws"), tf.get("tree_priv")),
                   "%s / %s" % (after.get("ws"), after.get("priv")),
                   tf.get("n") if tf else after.get("n")]))
print()
print("界面单进程 + 整树（idle）：")
for c in s["G8"].get("idle_combined") or []:
    print("- %s：界面 WS %s + 整树 WS %s = **%s MB**；界面 priv %s + 整树 priv %s = **%s MB**"
          % (c["run"], c["ui_ws"], c["tree_ws"], round(c["ui_ws"] + c["tree_ws"], 1),
             c["ui_priv"], c["tree_priv"], round(c["ui_priv"] + c["tree_priv"], 1)))
PERF_GAP_MD_EOF

log "== 完成：$WORK/summary.md（汇总）、$WORK/summary.json（机器可读）、各 case 的原始序列在 $WORK/<case>/*.jsonl"
