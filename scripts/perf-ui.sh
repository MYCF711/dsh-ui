#!/usr/bin/env bash
# perf-ui.sh —— dsh-ui 的性能/内存基线（实测，不估算）。
#
# 用法：
#   bash scripts/perf-ui.sh            # 全部场景，默认每档 3 次（趋势档 5 次）
#   PERF_RUNS=5 bash scripts/perf-ui.sh
#   PERF_ONLY=A,B bash scripts/perf-ui.sh
#
# 它自己起止 dshd serve：每次迭代一个全新的临时 home、`--addr 127.0.0.1:0`（OS 分配端口）、
# 一次性 token，绝不复用别人的端口或进程；结束只 TerminateProcess 自己 Popen 出来的那个句柄。
#
# 四个场景（口径见 docs/perf.md）：
#   A  冷启动到首帧（窗口可见 / 第一张 1196x839 的图落盘）
#   B1 空 home（0 会话）连上后静置 10s
#   B2 已打开的空会话（session.create 出来的 0 条消息会话）静置 10s
#   C  大会话 -tail 200 稳态（真实会话的只读副本，复制到临时 home）
#   D1 大会话 + 连续 10 次 down（-keys）
#   D2 大会话 + 会话内搜索（-search <会话里最高频的词>）
#
# 指标一律从进程外部量：WorkingSetSize / PrivateUsage（K32GetProcessMemoryInfo），
# 窗口用 EnumWindows + GetWindowRect，CPU 用 GetProcessTimes。不读界面自己的日志数字。

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export PATH="/c/Program Files/Go/bin:$PATH"

PY="${PERF_PYTHON:-}"
if [ -z "$PY" ]; then
  if command -v python >/dev/null 2>&1; then PY=python
  elif command -v python3 >/dev/null 2>&1; then PY=python3
  else echo "需要 python（3.8+）来量内存/窗口；找不到 python/python3" >&2; exit 2; fi
fi

RUNS="${PERF_RUNS:-3}"          # 每个场景的采样次数（取中位数）
TREND_RUNS="${PERF_TREND_RUNS:-5}" # 趋势档：同一场景连跑 5 次
ONLY="${PERF_ONLY:-A,B1,B2,C,D1,D2,T}"
IDLE="${PERF_IDLE:-10}"         # B/C 的静置秒数
KEYS="${PERF_KEYS:-10}"         # D1 的按键次数
TREND_GROUPS="${PERF_TREND_GROUPS:-5}"   # 进程内趋势：几组
TREND_BATCH="${PERF_TREND_BATCH:-10}"    # 每组几次操作（每次 0.35s）
REAL_HOME="${PERF_REAL_HOME:-D:/DeepSeek Harness/home}"   # 只读来源，只做复制
MAX_SESSION_MB="${PERF_MAX_SESSION_MB:-12}"               # 单份大会话的上限（太大会把测量变成 I/O 测量）
SESSIONS="${PERF_SESSIONS:-4}"                            # 复制几份「不同 id」的真实大会话（D1 切会话要用）
WORK="${PERF_WORK:-tmp/perf-run}"
APP="${PERF_APP:-tmp/perf-app.exe}"
SERVE="${PERF_SERVE:-tmp/dshd-serve.exe}"

log() { printf '%s\n' "$*" >&2; }

have_scenario() { case ",$ONLY," in *",$1,"*) return 0;; *) return 1;; esac; }

# ── 0. 二进制：自己构建，源码一变就重建（数字必须对应某一版源码） ──────────
mkdir -p "$WORK"
if [ ! -x "$SERVE" ]; then
  log "== 构建真宿主 tmp/dshd-serve.exe（来自 ../DSH-GO）"
  ( cd ../DSH-GO && CGO_ENABLED=0 go build -o "../dsh-ui/$SERVE" ./cmd/dshd )
fi
log "== 构建界面 $APP"
CGO_ENABLED=0 go build -o "$APP" ./cmd/app

APP_SHA=$(sha256sum "$APP" | cut -d' ' -f1)
SERVE_SHA=$(sha256sum "$SERVE" | cut -d' ' -f1)
GIT_HEAD=$(git rev-parse --short HEAD 2>/dev/null || echo "no-git")
GIT_DIRTY=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')

# ── 1. 内嵌的测量器（win32 ctypes；不依赖 psutil / PowerShell） ────────────
cat > "$WORK/helper.py" <<'PY'
"""perf helper：起 serve、起 app、从外部量窗口/内存/CPU，输出一个 JSON。"""
import argparse, ctypes, json, os, re, shutil, struct, subprocess, sys, threading, time
from ctypes import wintypes

k32 = ctypes.WinDLL("kernel32", use_last_error=True)
u32 = ctypes.WinDLL("user32", use_last_error=True)

PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
PROCESS_VM_READ = 0x0010
TH32CS_SNAPPROCESS = 0x00000002


class PMC_EX(ctypes.Structure):
    _fields_ = [("cb", wintypes.DWORD), ("PageFaultCount", wintypes.DWORD),
                ("PeakWorkingSetSize", ctypes.c_size_t), ("WorkingSetSize", ctypes.c_size_t),
                ("QuotaPeakPagedPoolUsage", ctypes.c_size_t), ("QuotaPagedPoolUsage", ctypes.c_size_t),
                ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t), ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
                ("PagefileUsage", ctypes.c_size_t), ("PeakPagefileUsage", ctypes.c_size_t),
                ("PrivateUsage", ctypes.c_size_t)]


class RECT(ctypes.Structure):
    _fields_ = [("left", wintypes.LONG), ("top", wintypes.LONG),
                ("right", wintypes.LONG), ("bottom", wintypes.LONG)]


class PROCESSENTRY32W(ctypes.Structure):
    _fields_ = [("dwSize", wintypes.DWORD), ("cntUsage", wintypes.DWORD),
                ("th32ProcessID", wintypes.DWORD), ("th32DefaultHeapID", ctypes.POINTER(ctypes.c_ulong)),
                ("th32ModuleID", wintypes.DWORD), ("cntThreads", wintypes.DWORD),
                ("th32ParentProcessID", wintypes.DWORD), ("pcPriClassBase", wintypes.LONG),
                ("dwFlags", wintypes.DWORD), ("szExeFile", ctypes.c_wchar * 260)]


WNDENUMPROC = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HWND, wintypes.LPARAM)


def open_process(pid):
    return k32.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION | PROCESS_VM_READ, False, pid)


def mem_counters(handle, eproc=False):
    pmc = PMC_EX()
    pmc.cb = ctypes.sizeof(PMC_EX)
    fn = k32.K32GetProcessMemoryInfo
    if fn(handle, ctypes.byref(pmc), pmc.cb) == 0:
        return None
    return {"ws": int(pmc.WorkingSetSize), "priv": int(pmc.PrivateUsage),
            "peak_ws": int(pmc.PeakWorkingSetSize)}


def cpu_ms(handle):
    class FT(ctypes.Structure):
        _fields_ = [("low", wintypes.DWORD), ("high", wintypes.DWORD)]
    ct, et, kt, ut = FT(), FT(), FT(), FT()
    if k32.GetProcessTimes(handle, ctypes.byref(ct), ctypes.byref(et),
                           ctypes.byref(kt), ctypes.byref(ut)) == 0:
        return None
    def q(f):
        return (f.high << 32) | f.low
    return (q(kt) + q(ut)) / 10000.0  # 100ns -> ms


def title_of(hwnd):
    n = u32.GetWindowTextLengthW(hwnd)
    if n <= 0:
        return ""
    buf = ctypes.create_unicode_buffer(n + 1)
    u32.GetWindowTextW(hwnd, buf, n + 1)
    return buf.value


def enum_windows(pid, needle):
    """返回本进程的 (hwnd, title, w, h, visible, iconic) 列表；needle 为空 = 只要标题非空。"""
    out = []

    def cb(hwnd, _):
        owner = wintypes.DWORD(0)
        u32.GetWindowThreadProcessId(hwnd, ctypes.byref(owner))
        if owner.value != pid:
            return True
        t = title_of(hwnd)
        if not t:
            return True
        if needle and needle.lower() not in t.lower():
            return True
        r = RECT()
        u32.GetWindowRect(hwnd, ctypes.byref(r))
        out.append((int(hwnd), t, r.right - r.left, r.bottom - r.top,
                    bool(u32.IsWindowVisible(hwnd)), bool(u32.IsIconic(hwnd))))
        return True

    u32.EnumWindows(WNDENUMPROC(cb), 0)
    return out


def hide_offscreen(hwnd):
    """挪到屏幕外：窗口照画，但点到 / 抢焦点都碰不到它。
    SWP_NOSIZE(0x1)|SWP_NOZORDER(0x4)|SWP_NOACTIVATE(0x10)。"""
    u32.SetWindowPos(wintypes.HWND(hwnd), wintypes.HWND(0), -6000, -6000, 0, 0, 0x1 | 0x4 | 0x10)


def child_processes(pid):
    snap = k32.CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0)
    if snap in (0, -1):
        return []
    entry = PROCESSENTRY32W()
    entry.dwSize = ctypes.sizeof(PROCESSENTRY32W)
    kids = []
    ok = k32.Process32FirstW(snap, ctypes.byref(entry))
    while ok:
        if entry.th32ParentProcessID == pid:
            kids.append({"pid": int(entry.th32ProcessID), "name": entry.szExeFile})
        ok = k32.Process32NextW(snap, ctypes.byref(entry))
    k32.CloseHandle(snap)
    return kids


def png_dims(path):
    try:
        with open(path, "rb") as f:
            head = f.read(24)
        if len(head) < 24 or head[:8] != b"\x89PNG\r\n\x1a\n":
            return None, None
        w, h = struct.unpack(">II", head[16:24])
        return w, h
    except OSError:
        return None, None


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
        raise SystemExit("serve 没给出 ready 行：" + repr(line[:200]))
    return json.loads(line[len("ready "):])


def http_json(base, method, path, cookie=None, body=None):
    import http.client
    host = base.split("://", 1)[1]
    conn = http.client.HTTPConnection(host, timeout=60)
    headers = {"Content-Type": "application/json"}
    if cookie:
        headers["Cookie"] = cookie
    payload = json.dumps(body).encode() if body is not None else None
    conn.request(method, path, body=payload, headers=headers)
    resp = conn.getresponse()
    data = resp.read()
    hdrs = {k.lower(): v for k, v in resp.getheaders()}
    conn.close()
    parsed = None
    try:
        parsed = json.loads(data.decode("utf-8"))
    except Exception:  # noqa
        pass
    return resp.status, hdrs, parsed, data


def api(base, token, method, params):
    """先 /connect 换 cookie，再 /api/<method>（裸参数，和界面走同一条路）。"""
    status, hdrs, _, _ = http_json(base, "GET", "/connect?token=" + token)
    cookie = ""
    raw = hdrs.get("set-cookie", "")
    if raw:
        cookie = raw.split(";")[0]
    status, _, parsed, data = http_json(base, "POST", "/api/" + method, cookie, params)
    return status, parsed, data


def marker_in(path, marker):
    """日志里有没有这行（每次重读；10s 日志只有几 KB，不值得做增量）。"""
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            return marker in f.read()
    except OSError:
        return False


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--work", required=True, help="本次迭代的临时目录（绝对或相对仓库根）")
    ap.add_argument("--repo", required=True)
    ap.add_argument("--serve", required=True)
    ap.add_argument("--app", required=True)
    ap.add_argument("--home-template", default="", help="把它复制成 <work>/home（大会话副本）")
    ap.add_argument("--create-session", action="store_true", help="用 session.create 建一个空会话")
    ap.add_argument("--args", default="", help="app 参数；{URL}/{TOKEN}/{SESSION} 会被替换")
    ap.add_argument("--shot", default="", help="-shot 的目标（相对 work 目录），给空 = 不截图")
    ap.add_argument("--shot-wait", default="0")
    ap.add_argument("--ready-marker", default="", help="app stderr 里出现它就当作「连上了」")
    ap.add_argument("--until-exit", action="store_true", help="等 app 自己退出（-shot 场景）")
    ap.add_argument("--observe", type=float, default=0.0, help="ready 之后再观察多少秒")
    ap.add_argument("--win-title", default="dsh-ui")
    ap.add_argument("--expect-sessions", type=int, default=None, help="home 里应该有几个会话（多了=被别的客户端污染）")
    ap.add_argument("--series", action="store_true", help="把每次内存采样也写进 JSON（进程内趋势用）")
    ap.add_argument("--timeout", type=float, default=45.0)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()

    repo = os.path.abspath(a.repo)
    work = os.path.abspath(os.path.join(repo, a.work))
    os.makedirs(work, exist_ok=True)
    home = os.path.join(work, "home")
    if a.home_template:
        if os.path.exists(home):
            shutil.rmtree(home)
        shutil.copytree(os.path.join(repo, a.home_template), home)
    else:
        os.makedirs(home, exist_ok=True)

    result = {"work": a.work, "home": os.path.relpath(home, repo)}
    serve_log = os.path.join(work, "serve.log")
    app_log = os.path.join(work, "app.log")

    # ── serve：只用 OS 分配的端口，只认它 stdout 的 ready 行 ──
    t_boot = time.perf_counter()
    with open(serve_log, "wb") as slerr:
        serve = subprocess.Popen([os.path.join(repo, a.serve), "serve", "--home", home,
                                  "--addr", "127.0.0.1:0"],
                                 cwd=repo, stdout=subprocess.PIPE, stderr=slerr)
    try:
        info = read_ready(serve, 60)
        result["serve_ready_ms"] = round((time.perf_counter() - t_boot) * 1000, 1)
        base, token = info["url"], info["token"]
        result["serve_url"] = base
        result["serve_pid"] = serve.pid

        session_id = ""
        if a.create_session:
            status, parsed, data = api(base, token, "session.create", {"mode": ""})
            if status != 200:
                raise SystemExit("session.create 失败：%s %s" % (status, data[:200]))
            session_id = parsed["result"]["sessionId"]
            # create 会把 token 用掉；界面要自己再换一次 —— 所以这里重起一个 serve。
            serve.terminate()
            try:
                serve.wait(timeout=10)
            except subprocess.TimeoutExpired:
                serve.kill(); serve.wait()
            with open(serve_log, "ab") as slerr:
                slerr.write(b"\n--- restart: token consumed by session.create ---\n")
                serve = subprocess.Popen([os.path.join(repo, a.serve), "serve", "--home", home,
                                          "--addr", "127.0.0.1:0"],
                                         cwd=repo, stdout=subprocess.PIPE, stderr=slerr)
            info = read_ready(serve, 60)
            base, token = info["url"], info["token"]
            result["serve_pid"] = serve.pid
            result["session_id"] = session_id
            result["serve_ready_ms"] = round((time.perf_counter() - t_boot) * 1000, 1)

        args = a.args.replace("{URL}", base).replace("{TOKEN}", token).replace("{SESSION}", session_id)
        shot_rel = os.path.join(a.work, a.shot) if a.shot else ""
        if shot_rel:
            args += " -shot " + shot_rel + " -shot-wait " + a.shot_wait
        result["app_args"] = args

        # ── app：从 Popen 起计时（含 CreateProcess），窗口/内存/CPU 全部外部量 ──
        t0 = time.perf_counter()
        with open(app_log, "wb") as aerr:
            app = subprocess.Popen([os.path.join(repo, a.app)] + args.split(),
                                   cwd=repo, stdout=subprocess.DEVNULL, stderr=aerr)
        hp = open_process(app.pid)
        if not hp:
            raise SystemExit("OpenProcess(app) 失败，err=%d" % ctypes.get_last_error())

        deadline = t0 + a.timeout
        ev = {"first_window": None, "visible": None, "sized": None, "iconic": None,
              "title": None, "rect": None}
        samples = []           # (t_ms, ws, priv)
        shot_abs = os.path.join(repo, shot_rel) if shot_rel else ""
        t_shot = None
        shot_dims = (None, None)
        iconic_at_shot = None
        ready_at = None
        seen_rects = []
        hidden = False
        last_mem = 0.0
        ready_deadline = deadline

        while time.perf_counter() < deadline:
            now = time.perf_counter()
            for hwnd, title, w, h, vis, iconic in enum_windows(app.pid, a.win_title):
                if ev["first_window"] is None:
                    ev["first_window"] = round((now - t0) * 1000, 1)
                    ev["title"] = title
                if not hidden:
                    hide_offscreen(hwnd)
                    hidden = True
                if (w, h) not in seen_rects:
                    seen_rects.append((w, h))
                if vis and ev["visible"] is None:
                    ev["visible"] = round((now - t0) * 1000, 1)
                if w >= 1000 and h >= 700 and ev["sized"] is None:
                    ev["sized"] = round((now - t0) * 1000, 1)
                    ev["rect"] = [w, h]
                if iconic and ev["iconic"] is None:
                    ev["iconic"] = round((now - t0) * 1000, 1)
                    iconic_at_shot = True
            if ready_at is None and (not a.ready_marker or marker_in(app_log, a.ready_marker)):
                ready_at = now
                result["children"] = child_processes(app.pid)
                result["mem_at_ready"] = mem_counters(hp)
                result["cpu_at_ready_ms"] = cpu_ms(hp)
                if a.observe:
                    ready_deadline = now + a.observe
            if shot_abs and t_shot is None and os.path.exists(shot_abs):
                t_shot = now
                dims = png_dims(shot_abs)
                shot_dims = dims if dims[0] else (0, 0)
            if app.poll() is not None:
                break
            if a.until_exit:
                pass
            else:
                if ready_at is not None and now >= ready_deadline:
                    break
            if now - last_mem >= 0.05:
                m = mem_counters(hp)
                if m:
                    samples.append((round((now - t0) * 1000, 1), m["ws"], m["priv"]))
                last_mem = now
            time.sleep(0.005)

        end = time.perf_counter()
        cpu_end = cpu_ms(hp)
        exited = app.poll()
        if exited is None:
            app.kill()
        app.wait(timeout=20)

        if shot_abs:
            # 进程退出后再读一次：PNG 可能还没写完
            dims = png_dims(shot_abs)
            if dims[0]:
                shot_dims = dims

        result["app_exit"] = app.returncode
        result["events"] = ev
        result["rects_seen"] = seen_rects
        result["children_at_ready"] = result.get("children", [])
        result["shot"] = None
        if a.shot:
            result["shot"] = {
                "path": a.shot,
                "w": shot_dims[0], "h": shot_dims[1],
                "t_ms": round((t_shot - t0) * 1000, 1) if t_shot else None,
                "iconic_seen": bool(iconic_at_shot),
                # 160x28 = 最小化窗口的 GetWindowRect，已知的废图签名
                "valid": bool(shot_dims[0] and shot_dims[0] >= 1000 and shot_dims[1] >= 700),
            }
        result["t_exit_ms"] = round((end - t0) * 1000, 1)
        result["cpu_ms"] = round((cpu_end or 0) - (result.get("cpu_at_ready_ms") or 0), 1) \
            if cpu_end is not None else None
        result["cpu_total_ms"] = round(cpu_end, 1) if cpu_end is not None else None

        # 内存口径：ws_final = 观察窗口最后 1 秒的中位数；ws_max = 整个窗口的最大值
        if samples:
            result["ws_max_mb"] = round(max(s[1] for s in samples) / 1048576, 1)
            result["priv_max_mb"] = round(max(s[2] for s in samples) / 1048576, 1)
            tail = [s for s in samples if s[0] >= samples[-1][0] - 1000] or samples[-1:]
            tail_ws = sorted(s[1] for s in tail)
            tail_pv = sorted(s[2] for s in tail)
            mid = len(tail_ws) // 2
            result["ws_final_mb"] = round(tail_ws[mid] / 1048576, 1)
            result["priv_final_mb"] = round(tail_pv[mid] / 1048576, 1)
            result["samples"] = len(samples)
        try:
            sd = os.path.join(home, "sessions")
            n_sessions = sum(len(os.listdir(os.path.join(sd, p)))
                             for p in os.listdir(sd)) if os.path.isdir(sd) else 0
        except OSError:
            n_sessions = -1
        result["sessions_in_home"] = n_sessions
        expect = a.expect_sessions
        if expect is None:
            expect = 1 if a.create_session else (0 if not a.home_template else None)
        result["contaminated"] = bool(expect is not None and n_sessions > expect)
        try:
            with open(app_log, "r", encoding="utf-8", errors="replace") as f:
                logtext = f.read()
        except OSError:
            logtext = ""
        m = re.search(r"工作集 ([0-9.]+) MB", logtext)
        result["self_ws_mb"] = float(m.group(1)) if m else None
        result["keys_handled"] = logtext.count("handled=true")
        result["keys_ignored"] = logtext.count("handled=false")
        if a.series:
            result["ws_series"] = samples
        result["ready_found"] = ready_at is not None
        result["ok"] = ready_at is not None or a.until_exit
    finally:
        try:
            if serve.poll() is None:
                serve.terminate()
                try:
                    serve.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    serve.kill(); serve.wait()
        except Exception:  # noqa
            pass
        result["serve_exit"] = serve.returncode

    with open(os.path.join(repo, a.out), "w", encoding="utf-8") as f:
        json.dump(result, f, ensure_ascii=False, indent=1)
    print(json.dumps({k: result.get(k) for k in
                      ("app_exit", "ws_final_mb", "ws_max_mb", "priv_final_mb",
                       "shot", "events", "cpu_ms", "children_at_ready")}, ensure_ascii=False))


if __name__ == "__main__":
    main()
PY

# ── 2. 大会话：真实 home 的只读副本（绝不直接写真实会话） ──────────────────
prepare_session() {
  if [ -f "$WORK/big-sessions.json" ]; then return 0; fi
  log "== 从 $REAL_HOME 挑 $SESSIONS 份大会话做只读副本（每份 id 不同；改名复制无效，session id 在 journal 里）"
  SESSION_LIMIT="$SESSIONS" "$PY" - "$REAL_HOME" "$MAX_SESSION_MB" "$WORK" <<'PYEOF'
import json, os, shutil, sys
real, max_mb, work = sys.argv[1], float(sys.argv[2]), sys.argv[3]
want = int(os.environ.get("SESSION_LIMIT", "4"))
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
    if sid in seen or size > max_mb * 1024 * 1024:
        continue
    seen.add(sid)
    picked.append((size, proj, sid))
    if len(picked) >= want:
        break
if not picked:
    print("没有可用会话（都超过 %.1f MB）" % max_mb, file=sys.stderr)
    sys.exit(2)
tpl = os.path.join(work, "big-template")
if os.path.exists(tpl):
    shutil.rmtree(tpl)
os.makedirs(os.path.join(tpl, "sessions"), exist_ok=True)
sessions = []
for size, proj, sid in picked:
    dst = os.path.join(tpl, "sessions", proj, sid)
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    shutil.copytree(os.path.join(root, proj, sid), dst)
    sessions.append({"project": proj, "sessionId": sid, "bytes": size})
total = sum(s["bytes"] for s in sessions)
info = {"sessions": sessions, "primary": sessions[0]["sessionId"],
        "primary_project": sessions[0]["project"], "total_bytes": total,
        "all_sessions": len(rows), "source_home": real}
with open(os.path.join(work, "big-sessions.json"), "w", encoding="utf-8") as f:
    json.dump(info, f, ensure_ascii=False, indent=1)
with open(os.path.join(work, "big-session.json"), "w", encoding="utf-8") as f:
    json.dump({"project": sessions[0]["project"], "sessionId": sessions[0]["sessionId"],
               "bytes": sessions[0]["bytes"], "total_sessions": len(rows)}, f, ensure_ascii=False, indent=1)
print("复制 %d 份：%s（共 %.1f MB；真实 home 里 %d 个会话）"
      % (len(sessions), ", ".join(s["sessionId"][:16] for s in sessions), total / 1048576, len(rows)),
      file=sys.stderr)
PYEOF
}

# 搜索词：起一个临时 serve 读一次 session.load，取出现在 ≥3 条消息里的最高频词
prepare_search_word() {
  [ -f "$WORK/search-word.txt" ] && return 0
  log "== 挑一个搜索词（读一次 session.load，只读）"
  "$PY" - "$WORK" "$SERVE" "$ROOT" <<'PY'
import json, os, re, subprocess, sys, threading, collections
work, serve_exe, repo = sys.argv[1], sys.argv[2], sys.argv[3]
info = json.load(open(os.path.join(work, "big-session.json"), encoding="utf-8"))
home = os.path.join(repo, work, "word-home")
if not os.path.exists(home):
    import shutil
    shutil.copytree(os.path.join(repo, work, "big-template"), home)
p = subprocess.Popen([os.path.join(repo, serve_exe), "serve", "--home", home, "--addr", "127.0.0.1:0"],
                     cwd=repo, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
box = {}
def rd():
    box["l"] = p.stdout.readline()
th = threading.Thread(target=rd, daemon=True); th.start(); th.join(60)
line = (box.get("l") or b"").decode().strip()
try:
    ready = json.loads(line[len("ready "):])
except Exception:
    print("拿不到搜索词：serve 没起来", file=sys.stderr); sys.exit(0)
import http.client
host = ready["url"].split("://", 1)[1]
c = http.client.HTTPConnection(host, timeout=60)
c.request("GET", "/connect?token=" + ready["token"]); r = c.getresponse(); r.read()
cookie = (r.getheader("Set-Cookie") or "").split(";")[0]; c.close()
def call(method, body):
    c = http.client.HTTPConnection(host, timeout=120)
    c.request("POST", "/api/" + method, body=json.dumps(body).encode(),
              headers={"Content-Type": "application/json", "Cookie": cookie})
    r = c.getresponse(); data = r.read(); c.close()
    return json.loads(data.decode())
res = call("session.load", {"id": info["sessionId"], "tailMessages": 200})
msgs = (res.get("result") or res).get("messages") or []
cnt = collections.Counter()
for m in msgs:
    words = set(re.findall(r"[A-Za-z][A-Za-z0-9_\-]{3,}", (m.get("text") or "")))
    for w in words:
        cnt[w] += 1
word = ""
for w, n in cnt.most_common(50):
    if n >= 3 and w.lower() not in ("bash", "true", "false", "null", "this", "that"):
        word = w
        break
messages = len(msgs)
with open(os.path.join(work, "search-word.txt"), "w", encoding="utf-8") as f:
    f.write(word)
with open(os.path.join(work, "search-word.json"), "w", encoding="utf-8") as f:
    json.dump({"word": word, "messages": messages,
               "total": (res.get("result") or res).get("total")}, f, ensure_ascii=False, indent=1)
p.terminate(); p.wait(timeout=10)
print("搜索词 %r（%d 条消息命中素材）" % (word, messages), file=sys.stderr)
PY
}

# ── 3. 跑场景 ────────────────────────────────────────────────────────────
run_case() { # name  runs  helper 参数...
  local name="$1" runs="$2"; shift 2
  local i
  mkdir -p "$WORK/$name"
  for i in $(seq 1 "$runs"); do
    log "   [$name] 第 $i/$runs 次"
    "$PY" "$WORK/helper.py" --repo "$ROOT" --work "$WORK/$name/$i" \
      --serve "$SERVE" --app "$APP" --out "$WORK/$name/$i.json" "$@" >"$WORK/$name/$i.stdout" 2>"$WORK/$name/$i.stderr" \
      || { log "   [$name] 第 $i 次失败：$(tail -3 "$WORK/$name/$i.stderr" | tr '\n' ' ')"; }
  done
}

BIG_ID=""
if have_scenario C || have_scenario D1 || have_scenario D2; then
  prepare_session
  BIG_ID=$("$PY" -c "import json,sys;print(json.load(open(sys.argv[1],encoding='utf-8'))['sessionId'])" "$WORK/big-session.json")
fi

if have_scenario A; then
  run_case A "$RUNS" --args "-addr {URL} -token {TOKEN} -shot-wait 0" \
    --shot "shot.png" --shot-wait 0 --until-exit --win-title dsh-ui
  # 废图（160x28 = 抓图时窗口处于最小化态）不算有效采样：补一次
  for f in "$WORK"/A/*.json; do
    [ -f "$f" ] || continue
    if ! "$PY" -c "import json,sys;sys.exit(0 if (json.load(open(sys.argv[1],encoding='utf-8')).get('shot') or {}).get('valid') else 1)" "$f" 2>/dev/null; then
      n=$(basename "$f" .json)
      log "   [A] 第 $n 次是废图（160x28），补一次"
      "$PY" "$WORK/helper.py" --repo "$ROOT" --work "$WORK/A/retry$n" --serve "$SERVE" --app "$APP" --out "$WORK/A/retry$n.json" --args "-addr {URL} -token {TOKEN}" --shot "shot.png" --shot-wait 0 --until-exit --win-title dsh-ui >/dev/null 2>"$WORK/A/retry$n.stderr" || true
    fi
  done
fi

if have_scenario B1; then
  run_case B1 "$RUNS" --args "-addr {URL} -token {TOKEN}" \
    --ready-marker "插件 [" --observe "$IDLE"
fi

if have_scenario B2; then
  run_case B2 "$RUNS" --args "-addr {URL} -token {TOKEN} -session {SESSION}" \
    --create-session --ready-marker "mux 已订阅" --observe "$IDLE"
fi

if have_scenario C; then
  run_case C "$RUNS" --home-template "$WORK/big-template" \
    --args "-addr {URL} -token {TOKEN} -session $BIG_ID -tail 200" \
    --ready-marker "mux 已订阅" --observe "$IDLE"
fi

if have_scenario D1; then
  # ↓↑ 交替：home 里只有有限个会话，单向连按会在边界被忽略（handleKey 返回 false）
  KEYS_SEQ=""
  for k in $(seq 1 "$KEYS"); do
    if [ $((k % 2)) -eq 1 ]; then KEYS_SEQ="$KEYS_SEQ,down"; else KEYS_SEQ="$KEYS_SEQ,up"; fi
  done
  KEYS_SEQ="${KEYS_SEQ#,}"
  run_case D1 "$RUNS" --home-template "$WORK/big-template" --expect-sessions "$SESSIONS" \
    --args "-addr {URL} -token {TOKEN} -session $BIG_ID -tail 200 -keys $KEYS_SEQ" \
    --ready-marker "mux 已订阅" --observe 12
fi

if have_scenario T; then
  # 进程内趋势：一个进程里连做 5 组 × 10 次切会话，每组之后看一次工作集
  TSEQ=""
  for k in $(seq 1 $((TREND_BATCH * TREND_GROUPS))); do
    if [ $((k % 2)) -eq 1 ]; then TSEQ="$TSEQ,down"; else TSEQ="$TSEQ,up"; fi
  done
  TSEQ="${TSEQ#,}"
  run_case T 1 --home-template "$WORK/big-template" --expect-sessions "$SESSIONS" --series \
    --args "-addr {URL} -token {TOKEN} -session $BIG_ID -tail 200 -keys $TSEQ" \
    --ready-marker "mux 已订阅" --observe 26 --timeout 150
fi

if have_scenario D2; then
  prepare_search_word
  WORD=$(cat "$WORK/search-word.txt" 2>/dev/null || true)
  if [ -n "$WORD" ]; then
    run_case D2 "$RUNS" --home-template "$WORK/big-template" \
      --args "-addr {URL} -token {TOKEN} -session $BIG_ID -tail 200 -search $WORD" \
      --ready-marker "mux 已订阅" --observe 10
  else
    log "   [D2] 没有拿到搜索词，跳过（记成「拿不到」）"
  fi
fi

# ── 4. 汇总成 markdown 表 ────────────────────────────────────────────────
{
"$PY" - "$WORK" "$ROOT" "$APP_SHA" "$SERVE_SHA" "$GIT_HEAD" "$GIT_DIRTY" "$IDLE" "$KEYS" "$TREND_GROUPS" "$TREND_BATCH" <<'PY'
import glob, json, os, statistics, sys

work, repo, app_sha, serve_sha, head, dirty, idle, keys = sys.argv[1:9]
work = os.path.join(repo, work)

def load(name):
    out = []
    for f in sorted(glob.glob(os.path.join(work, name, "*.json"))):
        try:
            out.append(json.load(open(f, encoding="utf-8")))
        except Exception:
            pass
    return out

def fmt(v):
    return "—" if v is None else ("%.1f" % v if isinstance(v, float) else str(v))

def agg(values):
    values = [v for v in values if v is not None]
    if not values:
        return None, None, None
    return statistics.median(values), min(values), max(values)

rows = []

a = load("A")
vis = [r.get("events", {}).get("visible") for r in a]
sized = [r.get("events", {}).get("sized") for r in a]
shot_t = [(r.get("shot") or {}).get("t_ms") for r in a]
bad = [r for r in a if r.get("shot") and not r["shot"].get("valid")]
for label, vals, unit, note in [
    ("A 冷启动 → 窗口可见（首帧在 ShowWindow 之前已 apply）", vis, "ms",
     "EnumWindows 5ms 轮询；进程内首帧 apply 早于窗口可见"),
    ("A 冷启动 → 第一张 1196x839 的图落盘", shot_t, "ms",
     "-shot 自带 ~780ms 固定等待（发现窗口后睡 700ms）+ PNG 编码，不是纯首帧"),
    ("A 冷启动 → 窗口尺寸就绪（>=1000x700）", sized, "ms", "同可见口径，只判尺寸"),
    ("A 抓图那一刻的工作集（进程自报）", [r.get("self_ws_mb") for r in a], "MB",
     "抓图/编码自己也会分配 ~8 MB（4 MB DIB + 4 MB RGBA），比 B1 稳态高是正常的"),
]:
    mid, lo, hi = agg(vals)
    rows.append((label, mid, lo, hi, len([v for v in vals if v is not None]), unit, note))

for label, name, key, note in [
    ("B1 空 home（0 会话）静置 10s", "B1", "ws_final_mb", "WorkingSetSize 单进程；最后 1s 中位数"),
    ("B2 已打开的空会话静置 10s", "B2", "ws_final_mb", "同上；会话由 session.create 建，0 条消息"),
    ("C 大会话 -tail 200 静置 10s", "C", "ws_final_mb", "WorkingSetSize 单进程；真实会话只读副本"),
    ("D1 大会话 + 连续 %s 次会话切换（↓↑ 交替）峰值" % keys, "D1", "ws_max_mb", "操作期间采样最大值（50ms 一次）"),
    ("D2 大会话 + 搜索（-search）峰值", "D2", "ws_max_mb", "搜索态首屏 + 静置期间最大值"),
]:
    data = load(name)
    vals = [r.get(key) for r in data]
    mid, lo, hi = agg(vals)
    rows.append((label, mid, lo, hi, len([v for v in vals if v is not None]), "MB", note))

print()
print("## dsh-ui 性能/内存基线（实测）")
print()
print("- 界面二进制：`%s` sha256 `%s`" % ("tmp/perf-app.exe", app_sha[:16]))
print("- 宿主二进制：`tmp/dshd-serve.exe` sha256 `%s`" % serve_sha[:16])
print("- git：`%s`（未提交改动 %s 个文件）" % (head, dirty))
print()
print("| 场景 | 中位数 | 最小 | 最大 | 采样次数 | 单位 | 口径 |")
print("|---|---:|---:|---:|---:|---|---|")
for label, mid, lo, hi, n, unit, note in rows:
    print("| %s | %s | %s | %s | %d | %s | %s |" % (label, fmt(mid), fmt(lo), fmt(hi), n, unit, note))
print()

# 补充：整进程树 / 子进程 / CPU / 废图
kids = sum(len(r.get("children_at_ready") or []) for r in a + load("B1") + load("B2") + load("C") + load("D1") + load("D2"))
print("- 子进程：所有迭代 `children_at_ready` 合计 **%d** 个 → 界面是**单进程**（见 docs/perf.md 口径）。" % kids)
b1 = load("B1"); c = load("C")
if b1 and c:
    pm = agg([r.get("priv_final_mb") for r in b1])[0]
    cm = agg([r.get("priv_final_mb") for r in c])[0]
    print("- PrivateUsage 对照：B1 中位数 %s MB，C 中位数 %s MB（PrivateUsage = 私有提交，含未触碰/已换出页；"
          "这里比 WorkingSetSize **大**，两个数不能混着报）。"
          % (fmt(pm), fmt(cm)))
if bad:
    b = bad[0]["shot"]
    print("- **废图（160x28）**：%d 次（%dx%d，抓图时窗口是最小化态=%s）。签名 = 最小化窗口的 GetWindowRect；"
          "复现与规避见 docs/perf.md「已知问题」。" % (len(bad), b["w"], b["h"], b["iconic_seen"]))
else:
    print("- **废图（160x28）**：0 次。")
d1 = load("D1")
if d1:
    kh = sum(r.get("keys_handled") or 0 for r in d1)
    ki = sum(r.get("keys_ignored") or 0 for r in d1)
    print("- D1 按键命中：日志里 `handled=true` **%d** 次、`handled=false` %d 次（false = 侧栏没接住，通常是在列表边界）。"
          % (kh, ki))
    cpu = agg([r.get("cpu_ms") for r in d1])[0]
    print("- D1 每次操作的平均 CPU 时间（代理指标，**不是** invalidate→重绘延迟）：%s ms / %s 次 = %s ms。"
          % (fmt(cpu), keys, fmt(None if cpu is None else round(cpu / int(keys), 1))))
print()
PY
} | tee "$WORK/summary.md"

# ── 5. 趋势：同一场景连跑 N 次（独立进程），看内存是不是一路涨 ─────────────
if have_scenario TREND || have_scenario B1; then
  log "== 趋势：B1 连跑 $TREND_RUNS 次（独立进程）"
  mkdir -p "$WORK/trend"
  for i in $(seq 1 "$TREND_RUNS"); do
    "$PY" "$WORK/helper.py" --repo "$ROOT" --work "$WORK/trend/$i" \
      --serve "$SERVE" --app "$APP" --out "$WORK/trend/$i.json" \
      --args "-addr {URL} -token {TOKEN}" --ready-marker "插件 [" --observe "$IDLE" \
      >"$WORK/trend/$i.stdout" 2>"$WORK/trend/$i.stderr" \
      || log "   [trend] 第 $i 次失败"
  done
  "$PY" - "$WORK" <<'PY'
import glob, json, os, sys
work = sys.argv[1]
vals = []
for f in sorted(glob.glob(os.path.join(work, "trend", "*.json")),
                key=lambda p: int(os.path.basename(p).split(".")[0])):
    try:
        r = json.load(open(f, encoding="utf-8"))
    except Exception:
        continue
    vals.append(r.get("ws_final_mb"))
print()
print("### 趋势（B1 连跑 %d 次，每次一个独立进程）" % len(vals))
print()
print("| 第 N 次 | " + " | ".join(str(i + 1) for i in range(len(vals))) + " |")
print("|---|" + "---:|" * len(vals))
print("| WorkingSetSize (MB) | " + " | ".join("—" if v is None else "%.1f" % v for v in vals) + " |")
real = [v for v in vals if v is not None]
if len(real) >= 2:
    first, last = real[0], real[-1]
    print()
    print("- 首次 → 末次：%+.1f MB" % (last - first))
    tail = real[-3:]
    print("- 后三次范围：%.1f – %.1f MB（极差 %.1f MB）" % (min(tail), max(tail), max(tail) - min(tail)))
    print("- 判定：%s" % ("平（极差 <= 3 MB）" if max(tail) - min(tail) <= 3 else "有抬升，见数字"))
print("- 注意：跨进程连跑量的是「启动开销是否漂移」；**不是**进程内泄漏判定。")

# 进程内趋势：一个进程里连做 5 组 × N 次切会话，看每组的稳态工作集
tfiles = sorted(glob.glob(os.path.join(work, "T", "*.json")))
if tfiles:
    try:
        t = json.load(open(tfiles[0], encoding="utf-8"))
    except Exception:
        t = {}
    series = t.get("ws_series") or []
    groups = int(sys.argv[9]) if len(sys.argv) > 9 else 5
    batch = int(sys.argv[10]) if len(sys.argv) > 10 else 10
    print()
    print("### 进程内趋势（T：一个进程，连做 %d 组 × %d 次切会话）" % (groups, batch))
    print()
    if not series:
        print("- 拿不到：这次没有采样序列。")
    else:
        # 按键在启动后 2.5s 开始、每 0.35s 一次；每组结束取该时刻之后 400ms 内的中位数
        marks = []
        for g in range(1, groups + 1):
            end_ms = (2.5 + 0.35 * batch * g) * 1000
            win = [v for (tm, v, _p) in series if end_ms + 200 <= tm <= end_ms + 600]
            if not win:
                near = sorted(series, key=lambda x: abs(x[0] - (end_ms + 400)))[:5]
                win = [v for (_tm, v) in ((x[0], x[1]) for x in near)]
            marks.append(statistics.median(win) / 1048576 if win else None)
        print("| 组（%d 次切会话/组） | " % batch + " | ".join(str(i + 1) for i in range(len(marks))) + " |")
        print("|---|" + "---:|" * len(marks))
        print("| WorkingSetSize (MB) | " + " | ".join("—" if m is None else "%.1f" % m for m in marks) + " |")
        real = [m for m in marks if m is not None]
        if len(real) >= 2:
            print()
            print("- 第 1 组 → 最后一组：%+.1f MB" % (real[-1] - real[0]))
            print("- 末两组差：%+.1f MB；全部组极差 %.1f MB" % (real[-1] - real[-2], max(real) - min(real)))
            print("- 判读：%s（只陈述数字；是分配器保留还是泄漏需要 heap profile，**待查**）"
                  % ("平" if max(real) - min(real) <= 2 else "有抬升，见上面的数"))
    print("- 口径：采样 50ms 一次；组边界按 `-keys` 的固定节奏（2.5s 起、0.35s 一键）换算。")
PY
fi

log "== 产物目录：$WORK（保留；删掉不影响下次重跑）"
