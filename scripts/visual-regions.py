#!/usr/bin/env python3
"""visual-regions.py —— **像素级视觉判据**：真机截图 + 逐区域像素差 + 阈值。

# 为什么需要它

从 v0.2.3 跨 11 个版本升到 v0.6.0，V-06-1/2/3 改了 188+89+90 个文件的**渲染调用**，
V-06-4 只验了"点得着"（hits 矩阵）。**编译绿 ≠ 画面对**：一次机械改名完全可能
让某个部件画到 0 高、画到窗口外、或者被另一个部件盖住——这些在 hits 矩阵里
都可能"有面积、有身份"，只有**看像素**才能发现。

本脚本对每个**产品形态**（`states`）跑一次真实 app 进程、用 `-shot` 抓窗口像素，
再和**基线图**按**显式区域**（`testdata/visual-v06/regions.json`）逐区域比像素差。
区域不是 OCR 猜的，是从 `uikernel.ComputeShellLayout` 的真实几何抄下来的；
`TestVisualRegionsMatchShellLayout`（`cmd/app/visual_regions_contract_test.go`）
会核对这份 JSON 与骨架算出来的是同一份矩形——几何改了而区域没改就会红。

# 种子（seeding）：为什么必须由脚本自己种

**只截图不种会话，这个门禁就不可复现**：`active-messages` / `demo-tool` /
`approval-card` 这些形态渲染的是**会话内容**，内容不同，像素差就是 30%+ 的
假红（实测：全新空会话跑出来 33 个区域超阈值，`demo-tool transcript 36.7%`、
`narrow-720x600 composer 76.3%`——不是迁移缺陷，是内容不同）。

所以 `--seed` 把会话**按固定脚本种成基线图那一份**：

- 种子定义**全在** `testdata/visual-v06/regions.json` 的 `seed` 块里
  （工作区文件、每轮 prompt、思考、正文、工具调用与结果、用量），
  **不硬编码在脚本里**——基线图和种子必须同源，改一处两处一起变。
- 脚本把 `seed.turns` 展开成 v4 journal 帧（`turn/start` → `step/start` →
  `user/message` → `assistant/message` → `tool/call` → `tool/result` →
  `step/end` → `turn/end`），写进 dshd 的临时 home。
- **时间戳是固定的**：帧 `time`、文件 mtime 全部来自 `seed.created_at`。
  这不是洁癖——状态栏渲染 `186 tok/s`（= outputTokens ÷ 相邻帧墙钟差）、
  消息行渲染 `01:07`、侧栏渲染 `08-01`，**任何一处拿实时钟都会让两次跑出不同像素**。
- **幂等**：固定 session id + 每次先删后建，同一 seed 跑两次得到**同一份内容**。

## 为什么是"回放录好的 transcript"而不是"真发一轮模型"

真发一轮模型**不可能**满足硬要求"两次跑像素差 0"：模型的回复文本、流式分块、
token 速率、帧时间戳每次都不一样，而这些全都直接进像素。所以种子是**录制产物**
（prompt 与回复都在 `regions.json` 里，可读可审），脚本负责**确定性回放**。
它验的是**渲染**（同一份 journal 帧在两个 viva 版本下画出来一样），不是模型行为。

## 用法（自包含，一条命令）

    # 全绿检查：脚本自己起两份临时 dshd（有会话 + 零会话），种好，再截图对比
    python scripts/visual-regions.py --dshd <dshd.exe> --binary <v0.6.0 的 app.exe>

    # 重刷基线（**只有复核过差异之后**才做；用 v0.5.1 的 app 二进制）
    python scripts/visual-regions.py --dshd <dshd.exe> --binary <v0.5.1 的 app.exe> --update

    # 只种会话、不截图（调试种子内容时用）
    python scripts/visual-regions.py --seed-home tmp/home --seed-only

    # 用已起好的宿主（老用法：先自己 `dshd serve --home …`，再导出 VISUAL_*）
    export VISUAL_ADDR=http://127.0.0.1:60630 VISUAL_COOKIE='dshd_session=…'
    python scripts/visual-regions.py --binary <app.exe>

    # 指定基线目录（对拍别的版本时用）
    python scripts/visual-regions.py --binary … --baseline tmp/visual-v06/baseline-v051

`--dshd` 模式每次都在**新的临时 home** 里种同一份种子，跑完回收进程与目录
（`--keep` 保留现场）。所以"干净环境复现"不需要任何手工步骤。

# 判据（阈值写死在这里，且**只准收紧**）

| 区域类别 | 阈值 | 为什么 |
|---|---|---|
| `structural`（三栏骨架 / header / composer / 状态栏 / 侧栏） | **> 2.0% 即红** | 这些是纯几何与色块，同一个二进制两次跑是 **0 像素差**（确定性已实测），2% 只留给字体微调 |
| `content`（消息流正文） | **> 5.0% 即红** | 正文有抗锯齿文字与换行，跨版本允许少量重排 |
| `full`（整窗） | **> 1.0% 即红** | 整窗是兜底：任何区域漏检都会在这里露头 |

阈值**不是**为"变绿"服务的：某处超阈值时，正确做法是判定归因
（① 上游行为变化 ② 我们的迁移缺陷 ③ 本来就没有），而不是把阈值调大。
确有**已接受**的上游变化时，写进 `accepted` 表并注明理由与上界——
那是**有上限的例外**，不是"把阈值蒙过去"。

退出码：0 = 全绿；1 = 有区域超阈值；2 = 输入/环境错误（不是"画面不对"）。
"""

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from http.cookiejar import CookieJar
from pathlib import Path

SCHEMA_VERSION = 1
TOOL_VERSION = 2

# 仓库根：默认路径**按脚本位置**解析，不按 cwd——从任何目录跑都指向同一份契约与基线
# （否则在 tmp/ 下跑一次就会去读 tmp/testdata/…，报一个和画面无关的错）。
REPO_ROOT = Path(__file__).resolve().parent.parent

# 这些形态由 `cmd/app` 的命令行开关驱动（`-shot` 是同一条产品路径的无头出口）。
# 新增可见部件时**必须**在这里加一帧，否则它永远不会被像素判据覆盖。
DEFAULT_CONTRACT = REPO_ROOT / "testdata" / "visual-v06" / "regions.json"
DEFAULT_BASELINE = REPO_ROOT / "testdata" / "visual-v06" / "baseline"
SHOT_TIMEOUT_SEC = 120
SEED_SCHEMA_VERSION = 1
DSHD_READY_TIMEOUT_SEC = 60
DSHD_STOP_TIMEOUT_SEC = 10


class InputError(RuntimeError):
    """环境或输入不合法——不是"画面不对"。"""


def load_contract(path):
    try:
        contract = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise InputError(f"读不了区域契约 {path}: {exc}") from exc
    if contract.get("schema_version") != SCHEMA_VERSION:
        raise InputError(f"{path}: schema_version 必须是 {SCHEMA_VERSION}")
    for key in ("window", "thresholds", "region_sets", "states"):
        if key not in contract:
            raise InputError(f"{path}: 缺 {key}")
    return contract


# ── 种子（seeding）──────────────────────────────────────────────────────
#
# 这一节把 `regions.json` 的 `seed` 块展开成 **v4 journal**（zstd 压缩的 JSONL，
# 第一帧是 session header，其余每行一个事件）。规则逐字对齐
# `DSH-GO/codecs/v4`（path.go 的 ProjectKey、write.go 的 header 键序），
# 因为宿主读的就是那一份，抄错一个字段就是"会话读不出来"。
#
# 为什么不复用宿主的写入器：宿主在 DSH-GO 仓（另一条线），本仓只能**产出它读得懂
# 的文件**。帧形状是从真实 journal 抄的（`internal/session` 的投影只认这些键），
# `--seed-only` 之后紧接着跑一次 `session.load` 就能验证形状对不对。


def _escape_utf16(text):
    """`codecs/v4/path.go:escapeUTF16` 的 Python 版：非 [A-Za-z0-9._-] → ~XXXX。"""
    raw = text.encode("utf-16-le", "surrogatepass")
    out = []
    for i in range(0, len(raw), 2):
        code = raw[i] | (raw[i + 1] << 8)
        ch = chr(code)
        if ("a" <= ch <= "z") or ("A" <= ch <= "Z") or ("0" <= ch <= "9") or ch in "._-":
            out.append(ch)
        else:
            out.append(f"~{code:04X}")
    return "".join(out)


def project_key(cwd):
    """`codecs/v4/path.go:ProjectKey` 的 Python 版（会话目录名）。"""
    for prefix in ("\\\\?\\", "\\\\.\\"):
        if cwd.startswith(prefix):
            cwd = cwd[len(prefix):]
            break
    replaced = "".join("-" if ch in "/\\:" else ch for ch in cwd)
    collapsed = []
    for i, ch in enumerate(replaced):
        if ch == "-" and i > 0 and replaced[i - 1] == "-":
            continue
        collapsed.append(ch)
    key = _escape_utf16("".join(collapsed)).lstrip("-")
    if not key:
        key = "root"
    return "--" + key[:251] + "--"


def parse_rfc3339_ms(text):
    """RFC3339 → epoch 毫秒（种子里的固定时间）。"""
    from datetime import datetime, timezone

    raw = (text or "").strip()
    if not raw:
        raise InputError("seed.created_at 不能为空（种子必须带固定时间戳）")
    if raw.endswith("Z"):
        raw = raw[:-1] + "+00:00"
    try:
        moment = datetime.fromisoformat(raw)
    except ValueError as exc:
        raise InputError(f"seed.created_at 不是 RFC3339 时间：{text!r}（{exc}）") from exc
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=timezone.utc)
    return int(moment.timestamp() * 1000)


def _seed_usage(step, seed):
    """一步 assistant/message 的用量（固定值，来自契约）。"""
    usage = step.get("usage") or seed.get("usage") or {}
    return {
        "inputTokens": int(usage.get("input_tokens", 0)),
        "outputTokens": int(usage.get("output_tokens", 0)),
        "totalTokens": int(usage.get("total_tokens", 0)),
        "cacheReadTokens": int(usage.get("cache_read_tokens", 0)),
    }


def _seed_blocks(step, turn_index, step_index):
    """把一步的内容展开成 v4 `content` 块（reasoning / text / tool-call）。"""
    blocks = []
    if step.get("reasoning"):
        blocks.append({"type": "reasoning", "text": step["reasoning"]})
    if step.get("text"):
        blocks.append({"type": "text", "text": step["text"]})
    for index, tool in enumerate(step.get("tools") or []):
        blocks.append({
            "type": "tool-call",
            "id": tool.get("call_id") or f"call_seed_{turn_index:02d}_{step_index:02d}_{index:02d}",
            "name": tool["name"],
            "arguments": json.dumps(tool.get("arguments") or {}, ensure_ascii=False,
                                    separators=(",", ":")),
        })
    return blocks


def seed_frames(seed, cwd, session_id):
    """把 `seed.turns` 展开成 v4 帧列表（不含 header）。

    时间轴是**算出来的固定值**：`seed.created_at` + 每步/每个工具结果的固定偏移。
    这样 `186 tok/s`、`01:07` 这些由时间算出来的像素每次跑都相同。
    """
    if not seed.get("turns"):
        raise InputError("seed.turns 不能为空（没有内容的种子等于没种）")
    base = parse_rfc3339_ms(seed["created_at"])
    model = seed.get("model") or {}
    provider = model.get("provider", "magpie")
    model_id = model.get("id", "group/deepseek-v4.1-flash-1017")
    step_ms = int(seed.get("step_duration_ms", 2000))
    tool_ms = int(seed.get("tool_duration_ms", 200))
    frames = []
    cursor = base
    turn_count = 0

    def emit(kind, data, moment, **envelope):
        frame = {"type": kind, "seq": len(frames), "time": moment, "data": data}
        frame.update(envelope)
        frames.append(frame)

    for turn_index, turn in enumerate(seed["turns"], start=1):
        turn_count = turn_index
        cursor += int(seed.get("turn_gap_ms", 1000))
        emit("turn/start", {"turn": turn_index}, cursor)
        for step_index, step in enumerate(turn["steps"], start=1):
            cursor += int(seed.get("step_gap_ms", 120))
            emit("step/start", {"turn": turn_index, "step": step_index}, cursor)
            if step_index == 1 and turn.get("user"):
                cursor += int(seed.get("user_gap_ms", 80))
                emit(
                    "user/message",
                    {
                        "content": [{"type": "text", "text": turn["user"]}],
                        "source": {"kind": "user"},
                        "role": "user",
                        "id": f"seed-user-{turn_index:02d}",
                    },
                    cursor,
                    surfaceOp="append",
                )
                if turn_index == 1:
                    emit(
                        "request/context",
                        {
                            "provider": provider,
                            "model": model_id,
                            "contextWindow": int(model.get("context_window", 1000000)),
                        },
                        cursor,
                    )
                    emit(
                        "session/title",
                        {"title": seed["title"], "messageSeqs": [len(frames) - 2],
                         "source": {"kind": "fallback"}},
                        cursor,
                    )
            cursor += step_ms
            blocks = _seed_blocks(step, turn_index, step_index)
            emit(
                "assistant/message",
                {
                    "turn": turn_index,
                    "step": step_index,
                    "message": {
                        "role": "assistant",
                        "content": blocks,
                        "source": {"kind": "model", "provider": provider, "model": model_id},
                        "id": f"seed-assistant-{turn_index:02d}-{step_index:02d}",
                    },
                    "usage": _seed_usage(step, seed),
                },
                cursor,
            )
            for block in blocks:
                if block["type"] != "tool-call":
                    continue
                cursor += tool_ms
                emit(
                    "tool/call",
                    {
                        "turn": turn_index,
                        "step": step_index,
                        "callId": block["id"],
                        "name": block["name"],
                        "arguments": block["arguments"],
                    },
                    cursor,
                )
            for block, tool in zip(
                [b for b in blocks if b["type"] == "tool-call"], step.get("tools") or []
            ):
                cursor += tool_ms
                emit(
                    "tool/result",
                    {
                        "turn": turn_index,
                        "step": step_index,
                        "message": {
                            "role": "tool",
                            "source": {"kind": "tool", "callId": block["id"]},
                            "toolCallId": block["id"],
                            "content": [{"type": "text", "text": tool.get("result", "")}],
                        },
                    },
                    cursor,
                )
            cursor += int(seed.get("step_tail_ms", 120))
            emit("step/end", {"turn": turn_index, "step": step_index}, cursor)
        cursor += int(seed.get("turn_tail_ms", 200))
        emit("turn/end", {"turn": turn_index, "reason": {"kind": "completed"}}, cursor)
    return frames, turn_count


def _write_zstd_jsonl(path, header, frames):
    """写 v4 journal：**每行一个独立 zstd 帧**（`codecs/v4/frame.go` 的格式）。"""
    try:
        import zstandard
    except ImportError as exc:  # pragma: no cover - 环境缺库
        raise InputError(
            "种会话需要 python 包 `zstandard`（pip install zstandard）：%s" % exc
        ) from exc
    path.parent.mkdir(parents=True, exist_ok=True)
    compressor = zstandard.ZstdCompressor(level=6, write_checksum=True)
    with path.open("wb") as stream:
        stream.write(compressor.compress(
            (json.dumps(header, ensure_ascii=False, separators=(",", ":")) + "\n").encode("utf-8")
        ))
        for frame in frames:
            stream.write(compressor.compress(
                (json.dumps(frame, ensure_ascii=False, separators=(",", ":")) + "\n").encode("utf-8")
            ))


def _seed_workspace(seed, home):
    """建种子工作区（**先删后建**：幂等，且不让上一次的残留文件进画面）。"""
    workspace = (home / seed.get("workspace", "visual-workspace")).resolve()
    if workspace.exists():
        shutil.rmtree(workspace)
    workspace.mkdir(parents=True)
    files = seed.get("files") or {}
    if not files:
        raise InputError("seed.files 不能为空（侧栏文件树必须每次长一样）")
    stamp = parse_rfc3339_ms(seed["created_at"]) / 1000.0
    for name, content in files.items():
        target = workspace / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8", newline="\n")
        os.utime(target, (stamp, stamp))
    for item in sorted(workspace.rglob("*"), reverse=True):
        os.utime(item, (stamp, stamp))
    return workspace


def write_seed(home, contract):
    """把种子写进一个 dshd home；返回 (session_id, journal 路径)。

    幂等：既有会话目录先删再写，工作区同理；文件 mtime 冻结在 `seed.created_at`
    （侧栏「08-01」与"最近会话"排序都读它）。
    """
    seed = contract.get("seed")
    if not seed:
        raise InputError("区域契约里没有 seed 块（--seed 需要它）")
    if seed.get("schema_version", SEED_SCHEMA_VERSION) != SEED_SCHEMA_VERSION:
        raise InputError(f"seed.schema_version 必须是 {SEED_SCHEMA_VERSION}")
    session_id = seed.get("session_id") or ""
    if not session_id:
        raise InputError("seed.session_id 不能为空（幂等靠它）")
    home = Path(home).resolve()
    home.mkdir(parents=True, exist_ok=True)
    for name in ("sessions", "artifacts", "plugins", "skills", "state"):
        (home / name).mkdir(exist_ok=True)
    workspace = _seed_workspace(seed, home)
    cwd = str(workspace)
    frames, _ = seed_frames(seed, cwd, session_id)
    header = {
        "type": "session",
        "version": 4,
        "id": session_id,
        "createdAt": parse_rfc3339_ms(seed["created_at"]),
        "cwd": cwd,
        "isSeeded": False,
        "delegationDepth": 0,
        "agentPreset": seed.get("agent_preset", "standard"),
    }
    target = home / "sessions" / project_key(cwd) / session_id / "session.v4.jsonl.zstd"
    if target.parent.exists():
        shutil.rmtree(target.parent)
    _write_zstd_jsonl(target, header, frames)
    stamp = parse_rfc3339_ms(seed["created_at"]) / 1000.0
    os.utime(target, (stamp, stamp))
    return session_id, target


# ── 宿主（--dshd 自包含模式）───────────────────────────────────────────


class Host:
    """一份临时 dshd：进程 + base URL + 会话 cookie。"""

    def __init__(self, name, proc, url, cookie, home):
        self.name = name
        self.proc = proc
        self.url = url
        self.cookie = cookie
        self.home = home


def _stop_host(host):
    """回收 dshd 进程树（**只杀我们自己起的那个 PID**）。"""
    proc = getattr(host, "proc", None)
    if proc is None or proc.poll() is not None:
        return
    if os.name == "nt":
        subprocess.run(
            ["taskkill", "/T", "/F", "/PID", str(proc.pid)],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False,
        )
    else:  # pragma: no cover - 本仓只支持 Windows
        proc.terminate()
    try:
        proc.wait(timeout=DSHD_STOP_TIMEOUT_SEC)
    except subprocess.TimeoutExpired:  # pragma: no cover - 极端情况
        proc.kill()


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    """`/connect` 用 303 回 Set-Cookie；跟到 Location 只会拿到 404。

    我们只要 cookie，所以**不跟跳转**，把 3xx 当成成功（cookie 已由
    HTTPCookieProcessor 从响应头里收下）。
    """

    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: D102
        return None


def _start_host(name, dshd_bin, home, log_path):
    """起一份 dshd serve（`--port 0`），等 `ready` 行，再用 launch token 换 cookie。"""
    home = Path(home).resolve()
    home.mkdir(parents=True, exist_ok=True)
    log = Path(log_path).resolve()
    log.parent.mkdir(parents=True, exist_ok=True)
    argv = [
        str(dshd_bin), "serve",
        "--home", str(home),
        "--addr", "127.0.0.1:0",
        "--policy", "allow",
        "--parent-watch=false",
    ]
    stream = log.open("wb")
    proc = subprocess.Popen(argv, stdout=stream, stderr=subprocess.STDOUT)
    deadline = time.time() + DSHD_READY_TIMEOUT_SEC
    payload = None
    while time.time() < deadline:
        if proc.poll() is not None:
            stream.close()
            raise InputError(f"dshd（{name}）退出码 {proc.returncode}（日志 {log}）")
        text = log.read_text(encoding="utf-8", errors="replace") if log.exists() else ""
        for line in text.splitlines():
            if line.startswith("ready "):
                try:
                    payload = json.loads(line[len("ready "):])
                except json.JSONDecodeError as exc:
                    raise InputError(f"dshd（{name}）的 ready 行不是 JSON：{exc}") from exc
                break
        if payload:
            break
        time.sleep(0.1)
    if not payload:
        _stop_host(Host(name, proc, "", "", home))
        stream.close()
        raise InputError(f"dshd（{name}）{DSHD_READY_TIMEOUT_SEC}s 没打印 ready（日志 {log}）")
    url = payload["url"].rstrip("/")
    jar = CookieJar()
    opener = urllib.request.build_opener(
        urllib.request.HTTPCookieProcessor(jar), _NoRedirect())
    try:
        opener.open(f"{url}/connect?token={payload['token']}", timeout=30).read()
    except urllib.error.HTTPError as exc:
        if exc.code not in (301, 302, 303, 307, 308):
            _stop_host(Host(name, proc, url, "", home))
            stream.close()
            raise InputError(f"dshd（{name}）的 /connect 返回 {exc.code}") from exc
    except urllib.error.URLError as exc:
        _stop_host(Host(name, proc, url, "", home))
        stream.close()
        raise InputError(f"dshd（{name}）连不上：{exc}") from exc
    cookie = "".join(f"{item.name}={item.value}" for item in jar)
    if not cookie:
        _stop_host(Host(name, proc, url, "", home))
        stream.close()
        raise InputError(f"dshd（{name}）的 /connect 没有种 cookie")
    return Host(name, proc, url, cookie, home)


def start_hosts(dshd_bin, root, contract):
    """起两份宿主：`default`（带种子会话）+ `empty`（零会话 Hero 首态）。

    为什么必须两份：`cmd/app` 在会话列表到达后会自动打开 `sessions[0]`
    （`main.go` 的 `if empty && len(ss) > 0`），所以**只要有会话就进不了 Hero 首屏**。
    """
    root = Path(root).resolve()
    root.mkdir(parents=True, exist_ok=True)
    seed_home = root / "home"
    empty_home = root / "home-empty"
    session_id, journal = write_seed(seed_home, contract)
    print(f"  SEED     session={session_id} journal={journal}")
    default = _start_host("default", dshd_bin, seed_home, root / "dshd.log")
    try:
        empty = _start_host("empty", dshd_bin, empty_home, root / "dshd-empty.log")
    except InputError:
        _stop_host(default)
        raise
    return default, empty, session_id


def build_env(default, empty, session_id):
    return {
        "addr_default": default.url,
        "cookie_default": default.cookie,
        "addr_empty": empty.url,
        "cookie_empty": empty.cookie,
        "session": session_id,
    }


def region_kind_limit(contract, kind):
    thresholds = contract["thresholds"]
    key = f"{kind}_percent"
    if key not in thresholds:
        raise InputError(f"区域类别 {kind!r} 没有对应阈值（{key}）")
    return float(thresholds[key])


def regions_for(contract, state):
    name = state.get("regions")
    sets = contract["region_sets"]
    if name not in sets:
        raise InputError(f"状态 {state['name']!r} 引用了不存在的区域集 {name!r}")
    return sets[name]


def state_args(contract, state, env):
    """把 `{session}` / `{cwd}` 之类的占位换成本次运行环境里的真值。"""
    replacements = {
        "session": env["session"],
        "cwd": env.get("cwd", ""),
    }
    out = []
    for arg in state.get("args", []):
        for key, value in replacements.items():
            if f"{{{key}}}" in arg and not value:
                raise InputError(f"状态 {state['name']!r} 需要 -{key}，但环境里没有")
            arg = arg.replace(f"{{{key}}}", value)
        out.append(arg)
    return out


def run_diff(a, b, regions, out_json, top_colors=8):
    """复用 `scripts/visual-diff.py`：本仓唯一一份逐像素测量实现。"""
    tool = Path(__file__).resolve().parent / "visual-diff.py"
    regions_json = out_json.with_suffix(".regions.json")
    regions_json.write_text(
        json.dumps(
            {"schema_version": 1, "top_colors": top_colors,
             "regions": [{"name": r["name"], "rect": r["rect"]} for r in regions]},
            ensure_ascii=False, indent=2,
        ) + "\n",
        encoding="utf-8",
    )
    proc = subprocess.run(
        [sys.executable, str(tool), str(a), str(b),
         "--regions", str(regions_json), "--out", str(out_json)],
        capture_output=True, text=True,
    )
    if proc.returncode != 0:
        raise InputError(
            f"visual-diff.py 失败（{proc.returncode}）：{proc.stderr.strip()}"
        )
    return json.loads(out_json.read_text(encoding="utf-8"))


def compare(contract, state, baseline_png, current_png, out_dir):
    regions = regions_for(contract, state)
    out_json = out_dir / f"{state['name']}.diff.json"
    result = run_diff(baseline_png, current_png, regions, out_json)
    rows = []
    for region in result["regions"]:
        name = region["name"]
        spec = next(r for r in regions if r["name"] == name)
        kind = spec.get("kind", "structural")
        percent = region["diff"]["difference_percent"]
        accepted = contract.get("accepted", {}).get(f"{state['name']}|{name}")
        limit = float(accepted["max_percent"]) if accepted else region_kind_limit(contract, kind)
        rows.append({
            "state": state["name"],
            "region": name,
            "kind": kind,
            "percent": percent,
            "limit": limit,
            "different_pixels": region["diff"]["different_pixels"],
            "pixel_count": region["diff"]["pixel_count"],
            "rect_xywh": spec["rect"],
            "accepted": bool(accepted),
            "accepted_reason": accepted["reason"] if accepted else "",
            "ok": percent <= limit,
        })
    return rows, out_json


def main(argv=None):
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--binary", type=Path,
                        help="被验的 app 可执行文件（已构建好的产品二进制）")
    parser.add_argument("--contract", type=Path, default=DEFAULT_CONTRACT)
    parser.add_argument("--baseline", type=Path, default=DEFAULT_BASELINE,
                        help="基线 PNG 目录")
    parser.add_argument("--out", type=Path, default=REPO_ROOT / "tmp" / "visual-regions",
                        help="本次截图与差分的落盘目录")
    parser.add_argument("--only", default="",
                        help="只跑这些状态（逗号分隔）")
    parser.add_argument("--update", action="store_true",
                        help="用本次截图覆盖基线（复核过差异之后才准用）")
    parser.add_argument("--dshd", type=Path,
                        help="自包含模式：由本脚本起两份临时 dshd（有会话 + 零会话）")
    parser.add_argument("--home", type=Path,
                        help="自包含模式的临时根目录（默认系统临时目录；--keep 时保留）")
    parser.add_argument("--keep", action="store_true",
                        help="自包含模式跑完不删临时目录（调试用）")
    parser.add_argument("--seed", action="store_true",
                        help="截图前先把会话种成 regions.json 里那一份（幂等）")
    parser.add_argument("--seed-home", type=Path,
                        help="--seed 要写进的 dshd home（配合已起好的宿主）")
    parser.add_argument("--seed-only", action="store_true",
                        help="只种会话、打印 session id、退出（不截图）")
    parser.add_argument("--version", action="version", version=f"%(prog)s {TOOL_VERSION}")
    args = parser.parse_args(argv)

    contract = load_contract(args.contract)

    if args.seed_only:
        if not args.seed_home:
            raise InputError("--seed-only 需要 --seed-home <dshd home>")
        session_id, journal = write_seed(args.seed_home, contract)
        print(f"seed ok: session={session_id}")
        print(f"  journal: {journal}")
        return 0

    if not args.binary:
        raise InputError("没有 --binary（被验的 app 可执行文件）")
    binary = args.binary
    if not binary.exists():
        raise InputError(f"二进制不存在：{binary}")

    hosts = ()
    tmp_root = None
    try:
        if args.dshd:
            if not args.dshd.exists():
                raise InputError(f"dshd 不存在：{args.dshd}")
            tmp_root = Path(args.home) if args.home else Path(
                tempfile.mkdtemp(prefix="visual-v06-"))
            if tmp_root.exists() and args.home is None:
                pass
            hosts = start_hosts(args.dshd, tmp_root, contract)
            env = build_env(*hosts)
        else:
            env = {
                "addr_default": os.environ.get("VISUAL_ADDR", ""),
                "cookie_default": os.environ.get("VISUAL_COOKIE", ""),
                "addr_empty": os.environ.get("VISUAL_ADDR_EMPTY", ""),
                "cookie_empty": os.environ.get("VISUAL_COOKIE_EMPTY", ""),
                "session": os.environ.get("VISUAL_SESSION", ""),
            }
            if args.seed:
                if not args.seed_home:
                    raise InputError(
                        "--seed 需要 --seed-home <dshd home>（或改用 --dshd 自包含模式）")
                session_id, journal = write_seed(args.seed_home, contract)
                print(f"  SEED     session={session_id} journal={journal}")
                env["session"] = session_id
        if not env["addr_default"]:
            raise InputError("没有 VISUAL_ADDR（宿主的 base URL）")
        if not env["session"]:
            raise InputError("没有 VISUAL_SESSION（要打开的会话 id）")

        wanted = {s for s in args.only.split(",") if s}
        states = [s for s in contract["states"] if not wanted or s["name"] in wanted]
        if not states:
            raise InputError(f"--only 没匹配到任何状态：{sorted(wanted)}")

        args.out.mkdir(parents=True, exist_ok=True)
        if args.update:
            args.baseline.mkdir(parents=True, exist_ok=True)

        all_rows = []
        failures = 0
        for state in states:
            png = run_capture(contract, state, env, binary, args.out)
            if args.update:
                target = args.baseline / f"{state['name']}.png"
                target.write_bytes(png.read_bytes())
                print(f"  UPDATED  {state['name']} -> {target}")
                continue
            baseline_png = args.baseline / f"{state['name']}.png"
            if not baseline_png.exists():
                raise InputError(
                    f"基线缺 {baseline_png}；先跑一次 --update 生成（或用 --baseline 指向 v0.5.1 的截图）")
            rows, diff_json = compare(contract, state, baseline_png, png, args.out)
            all_rows.extend(rows)
            for row in rows:
                mark = "PASS" if row["ok"] else "FAIL"
                if not row["ok"]:
                    failures += 1
                note = "（已接受：" + row["accepted_reason"] + "）" if row["accepted"] else ""
                print("  %-4s %-24s %-12s %8.3f%% ≤ %5.2f%%  %7d/%d px  %s%s" % (
                    mark, row["state"], row["region"], row["percent"], row["limit"],
                    row["different_pixels"], row["pixel_count"], diff_json.name, note))

        report = args.out / "report.json"
        report.write_text(json.dumps({
            "schema_version": SCHEMA_VERSION,
            "tool": {"name": "visual-regions.py", "version": TOOL_VERSION},
            "binary": str(binary),
            "baseline": str(args.baseline),
            "seeded": bool(hosts) or args.seed,
            "states": [s["name"] for s in states],
            "thresholds": contract["thresholds"],
            "rows": all_rows,
            "failures": failures,
        }, ensure_ascii=False, indent=2, allow_nan=False) + "\n", encoding="utf-8")

        total = len(all_rows)
        print(f"\n== 区域合计 {total}，超阈值 {failures}（报告 {report}）==")
        return 1 if failures else 0
    finally:
        for host in hosts:
            _stop_host(host)
        if tmp_root is not None:
            if args.keep:
                print(f"  KEEP     {tmp_root}")
            else:
                shutil.rmtree(tmp_root, ignore_errors=True)


def host_for(state, env):
    """每个状态可以指定宿主：默认那一份，或 `empty`（**零会话**的宿主）。

    为什么需要两份宿主：`cmd/app` 在会话列表到达后会自动打开 `sessions[0]`
    （`main.go` 的 `if empty && len(ss) > 0`），所以**只要有会话就进不了 Hero 首屏**。
    Hero 形态必须用一份 `sessions/` 为空的宿主——这是产品正确行为，不是绕过。
    """
    name = state.get("host", "default")
    addr, cookie = env.get(f"addr_{name}"), env.get(f"cookie_{name}")
    if not addr:
        raise InputError(f"状态 {state['name']!r} 要宿主 {name!r}，但没有 VISUAL_ADDR_{name.upper()}")
    return addr, cookie


def run_capture(contract, state, env, binary, out_dir):
    """跑一次真 app 进程，落一张 PNG。

    `-cookie` 是**一次性 token 的替代**：`-token` 换来的 cookie 只能用一次，
    多个状态各起一个进程时，第二个就会 401。
    """
    addr, cookie = host_for(state, env)
    out_dir.mkdir(parents=True, exist_ok=True)
    png = (out_dir / f"{state['name']}.png").resolve()
    if png.exists():
        png.unlink()
    title = f"visual-regions-{state['name']}"
    argv = [
        str(binary),
        "-addr", addr,
        "-shot", str(png),
        "-shot-wait", str(state.get("wait", 6)),
        "-size", state.get("size", contract["window"]["size"]),
        "-title", title,
    ]
    if cookie:
        argv += ["-cookie", cookie]
    if not state.get("no_session", False):
        argv += ["-session", env["session"]]
    argv += state_args(contract, state, env)

    log = out_dir / f"{state['name']}.log"
    try:
        with log.open("wb") as stream:
            proc = subprocess.run(
                argv, stdout=stream, stderr=subprocess.STDOUT,
                timeout=SHOT_TIMEOUT_SEC,
            )
    except subprocess.TimeoutExpired as exc:
        raise InputError(
            f"{state['name']}: {SHOT_TIMEOUT_SEC}s 超时（日志 {log}）"
        ) from exc
    if proc.returncode != 0:
        raise InputError(
            f"{state['name']}: 进程退出码 {proc.returncode}（日志 {log}）"
        )
    if not png.exists() or png.stat().st_size == 0:
        raise InputError(f"{state['name']}: 没有截图（日志 {log}）")
    return png


if __name__ == "__main__":
    try:
        sys.exit(main())
    except InputError as exc:
        print(f"visual-regions: error: {exc}", file=sys.stderr)
        sys.exit(2)
