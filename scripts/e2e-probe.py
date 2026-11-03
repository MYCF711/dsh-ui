#!/usr/bin/env python3
"""e2e-probe.py —— e2e 探针的断言层：每条 PASS 都必须说清楚「需要什么具体值」。

为什么要有这一层（2026-11-04，U1-H6）：

`scripts/e2e-workday.sh` 原来的断言是

    curl ... | grep -q "WORKDAY-1" && ok ... || bad ...

这条在**模型一个字都没答**的时候也会 PASS：用户 prompt 原文里就写着
`Reply with exactly: WORKDAY-1`，而 `session.load` 会把 user 消息一起返回，
grep 命中的是**用户自己的话**。同类的假阳性一共四类：

1. 只看词、不看角色：user 回显冒充 assistant 正文（本轮真机复现过）。
2. 只看词、不看信封：宿主出错时回的是 `{"error":{"code":...}}`（HTTP 也可能是 404），
   `grep -q WORKDAY-OTHER` 不命中反而被当成「两会话内容隔离」通过。
3. 负向断言没有正向控制：load 空/失败 → 不命中 → 负向断言 PASS。
4. 空 body 也算通过：`http_code==200` 之外什么都不看。

所以这里统一成：**先校验信封，再校验结构，最后校验具体字段**，
每条探针都能被一份坏输入打成红（见 `scripts/e2e-probe-selftest.sh`）。

用法：
    python scripts/e2e-probe.py load-ok tmp/x.json --min-messages 2
    python scripts/e2e-probe.py role-text tmp/x.json --role assistant --contains WORKDAY-1
    printf '%s' "$body" | python scripts/e2e-probe.py load-ok - --min-messages 1

每个子命令：成功 exit 0 打 `OK  <细节>`；失败 exit 1 打 `NG  <原因>`。
`FILE` 传 `-` 表示从 stdin 读。
"""
from __future__ import annotations

import argparse
import json
import pathlib
import re
import sys

ROLES = ("user", "assistant", "tool")

try:  # Windows 控制台默认 cp936，日志里中文会变乱码；统一走 UTF-8。
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")
except Exception:  # noqa: BLE001 - 老 Python / 非 tty 不支持就算了
    pass


def ng(msg: str) -> "NoReturn":  # type: ignore[name-defined]
    print("NG  " + msg)
    sys.exit(1)


def ok(msg: str) -> "NoReturn":  # type: ignore[name-defined]
    print("OK  " + msg)
    sys.exit(0)


# ---------------------------------------------------------------- 读入 + 信封

def read_bytes(path: str) -> bytes:
    if path == "-":
        return sys.stdin.buffer.read()
    try:
        return pathlib.Path(path).read_bytes()
    except OSError as e:
        ng(f"读不到 {path}：{e}")


def read_json(path: str) -> dict:
    raw = read_bytes(path)
    if not raw.strip():
        ng(f"{path} 是空的（0 字节）——空响应绝不算通过")
    try:
        text = raw.decode("utf-8-sig")  # Go 侧是 UTF-8；BOM 兼容一下
    except UnicodeDecodeError as e:
        ng(f"{path} 不是 UTF-8：{e}")
    try:
        obj = json.loads(text)
    except json.JSONDecodeError as e:
        ng(f"{path} 不是 JSON（{e}）；前 120 字节：{raw[:120]!r}")
    if not isinstance(obj, dict):
        ng(f"{path} 的顶层是 {type(obj).__name__}，不是对象")
    return obj


def envelope(path: str, obj: dict) -> dict:
    """宿主回的是 {"result":{...}} / {"error":{...}}（HTTP 分别 200 / 4xx-5xx）。

    只看 body 不看信封，是 `grep -q` 那代探针最大的假阳性来源：错误体里没有
    要找的词，负向断言就"通过"了。
    """
    if "error" in obj:
        e = obj["error"]
        if isinstance(e, dict):
            ng(f"{path} 是错误信封：code={e.get('code')!r} message={e.get('message')!r}")
        ng(f"{path} 是错误信封：{e!r}")
    if "result" not in obj:
        ng(f"{path} 没有 result 字段（顶层键：{sorted(obj)[:8]}）")
    r = obj["result"]
    if not isinstance(r, dict):
        ng(f"{path} 的 result 是 {type(r).__name__}，不是对象")
    return r


def load_struct(path: str) -> dict:
    """校验 session.load 的结构，返回 result。任何一条不满足都是 NG。"""
    r = envelope(path, read_json(path))
    msgs = r.get("messages")
    if not isinstance(msgs, list):
        ng(f"{path} 的 result.messages 不是数组（{type(msgs).__name__}）")
    total = r.get("total")
    if not isinstance(total, int):
        ng(f"{path} 的 result.total 不是整数（{total!r}）")
    if total < len(msgs):
        ng(f"{path} 的 total={total} < messages 条数 {len(msgs)}，结构不自洽")
    for i, m in enumerate(msgs):
        if not isinstance(m, dict):
            ng(f"{path} 的 messages[{i}] 不是对象")
        role = m.get("role")
        if role not in ROLES:
            ng(f"{path} 的 messages[{i}].role={role!r} 不在 {ROLES} 里")
        if not isinstance(m.get("seq"), int):
            ng(f"{path} 的 messages[{i}].seq 不是整数")
        if not isinstance(m.get("text"), str):
            ng(f"{path} 的 messages[{i}].text 不是字符串")
    return r


def describe(r: dict, limit: int = 60) -> str:
    """把会话尾巴压成一行，失败时直接看见「到底有什么」。"""
    parts = []
    for m in r.get("messages") or []:
        t = (m.get("text") or "").replace("\n", "\\n")
        if len(t) > limit:
            t = t[:limit] + "…"
        parts.append(f"{m.get('role')}:{t}")
    return " | ".join(parts) if parts else "（没有消息）"


# ---------------------------------------------------------------- 各子命令

def cmd_load_ok(a) -> None:
    r = load_struct(a.file)
    msgs = r["messages"]
    if len(msgs) < a.min_messages:
        ng(f"messages 只有 {len(msgs)} 条，要求 ≥{a.min_messages}；内容：{describe(r)}")
    if r["total"] < a.min_total:
        ng(f"total={r['total']}，要求 ≥{a.min_total}（空会话/丢了历史都会到这里）")
    if a.require_full and r.get("truncated"):
        ng(f"truncated=true：这份 load 只是 total={r['total']} 里的一段尾巴"
           f"（拿到 {len(msgs)} 条）。要证明「整个会话里没有 X」，必须拿全量")
    if a.permission is not None:
        got = r.get("permission")
        if got != a.permission:
            ng(f"permission={got!r}，要求 {a.permission!r}（宿主没按 --policy 起？）")
    ok(f"{a.file}：messages={len(msgs)} total={r['total']} "
       f"truncated={r.get('truncated')} permission={r.get('permission')!r}")


def cmd_role_text(a) -> None:
    r = load_struct(a.file)
    hit = [m for m in r["messages"]
           if m.get("role") == a.role and a.contains in (m.get("text") or "")]
    if not hit:
        ng(f"没有 role={a.role} 且 text 含 {a.contains!r} 的消息"
           f"（**别的角色里的同名词不算**）；实际内容：{describe(r)}")
    if a.not_contains and any(a.not_contains in (m.get("text") or "") for m in hit):
        ng(f"命中的 role={a.role} 消息里还含 {a.not_contains!r}（本该不含）")
    t = hit[-1]["text"].replace("\n", "\\n")
    ok(f"{a.file}：role={a.role} 命中 {a.contains!r}（{len(hit)} 条，最后一条：{t[:80]!r}）")


def cmd_tool_ran(a) -> None:
    r = load_struct(a.file)
    hit = []
    for m in r["messages"]:
        if m.get("role") != "tool" or not (m.get("tool") or ""):
            continue
        # 只看输出的 text / summary：args 里本来就有整条命令，
        # 拿 args 命中就等于「用户 prompt 回显」那个老假阳性换个位置重演。
        out = (m.get("text") or "") + "\n" + (m.get("summary") or "")
        if a.contains in out:
            hit.append(m)
    if not hit:
        ng(f"没有「role=tool 且输出（text/summary）含 {a.contains!r}」的消息；"
           f"实际内容：{describe(r)}")
    last = hit[-1]
    out = (last.get("text") or "").strip()
    if a.no_error_prefix and out.startswith("Error"):
        ng(f"工具结果以 Error 开头（被拒/失败，不是「执行了」）：{out[:120]!r}")
    ok(f"{a.file}：tool={last.get('tool')!r} 的输出含 {a.contains!r}（{out[:80]!r}）")


def cmd_not_contains(a) -> None:
    """负向断言 —— **先要求结构成立**，否则「load 失败」会冒充「内容隔离」。"""
    r = load_struct(a.file)
    if not r["messages"]:
        ng(f"{a.file} 的 messages 是空的——空结果不能用来证明「不含 {a.contains}」")
    for i, m in enumerate(r["messages"]):
        for k in ("text", "tool", "args", "summary"):
            v = m.get(k)
            if isinstance(v, str) and a.contains in v:
                ng(f"messages[{i}].{k} 含 {a.contains!r}：{v[:100]!r}")
    ok(f"{a.file}：{len(r['messages'])} 条消息、全字段都不含 {a.contains!r}")


def cmd_has_running_turn(a) -> None:
    """判「turn 在跑」：尾部有一条 text 为空的 assistant（回合开始未收尾）。"""
    r = load_struct(a.file)
    running = [m for m in r["messages"]
               if m.get("role") == "assistant" and not (m.get("text") or "").strip()]
    if not running:
        ng(f"尾部没有空的 assistant（turn 已收尾或根本没开始）；内容：{describe(r)}")
    ok(f"{a.file}：在途 turn 的 assistant 是空的（seq={running[-1]['seq']}）")


def cmd_accepted(a) -> None:
    r = envelope(a.file, read_json(a.file))
    if r.get("accepted") is not True:
        ng(f"accepted={r.get('accepted')!r}，要求 True")
    opt = r.get("optionId")
    if not opt:
        ng(f"accepted=true 但没回显 optionId（{opt!r}）——无从判断回传的是哪个选项")
    ok(f"{a.file}：accepted=true optionId={opt!r}")


def cmd_cancelled(a) -> None:
    r = envelope(a.file, read_json(a.file))
    if r.get("cancelled") is not True:
        ng(f"cancelled={r.get('cancelled')!r}，要求 True")
    ok(f"{a.file}：cancelled=true")


def cmd_http_ok(a) -> None:
    code = read_bytes(a.code).decode("ascii", "replace").strip()
    if code != str(a.expect):
        body = read_bytes(a.file)[:160]
        ng(f"HTTP {code}（要求 {a.expect}），body 前 160 字节：{body!r}")
    envelope(a.file, read_json(a.file))
    ok(f"{a.file}：HTTP {code} 且是 result 信封")


def cmd_list_has(a) -> None:
    r = envelope(a.file, read_json(a.file))
    ss = r.get("sessions")
    if not isinstance(ss, list):
        ng(f"{a.file} 的 result.sessions 不是数组（{type(ss).__name__}）")
    ids = [s.get("id") for s in ss if isinstance(s, dict)]
    if a.id not in ids:
        ng(f"session.list 里没有 {a.id!r}（有 {ids[:6]}）")
    ok(f"{a.file}：list 里有 {a.id}（共 {len(ids)} 个会话）")


def cmd_shot(a) -> None:
    """截图证据：日志说存了 + 文件真存在 + 是真 PNG + 尺寸像真窗口。"""
    log = read_bytes(a.log).decode("utf-8", "replace")
    m = re.search(r"shot: 已保存 (.+?) · 工作集 ([\d.]+) MB", log)
    if not m:
        tail = " / ".join(log.strip().splitlines()[-3:])
        ng(f"{a.log} 里没有「shot: 已保存 … 工作集」这行；尾部：{tail[:200]}")
    saved, mb = m.group(1).strip(), float(m.group(2))
    if pathlib.Path(saved).name != pathlib.Path(a.png).name:
        ng(f"日志说存的是 {saved!r}，期望 {a.png!r}")
    p = pathlib.Path(a.png)
    if not p.is_file():
        ng(f"日志说已保存，但 {a.png} 不存在")
    b = p.read_bytes()
    if len(b) < a.min_bytes:
        ng(f"{a.png} 只有 {len(b)} 字节（要求 ≥{a.min_bytes}）——空图/废图")
    if b[:8] != b"\x89PNG\r\n\x1a\n":
        ng(f"{a.png} 不是 PNG（头 8 字节 {b[:8]!r}）")
    w = int.from_bytes(b[16:20], "big")
    h = int.from_bytes(b[20:24], "big")
    if w < a.min_w or h < a.min_h:
        ng(f"{a.png} 尺寸 {w}x{h}，要求 ≥{a.min_w}x{a.min_h}（最小化/空窗口抓出来的废图）")
    ok(f"{a.png}：{len(b)} 字节 {w}x{h}，日志指向同一路径，工作集 {mb} MB")


def cmd_mem(a) -> None:
    """工作集读数 —— 原来只是 echo 一下，取不到就静默空行（等于没测）。"""
    log = read_bytes(a.log).decode("utf-8", "replace")
    ms = re.findall(r"工作集 ([\d.]+) MB", log)
    if not ms:
        ng(f"{a.log} 里没有「工作集 N MB」——内存这条其实没测到")
    mb = float(ms[-1])
    if not (a.min_mb <= mb <= a.max_mb):
        ng(f"工作集 {mb} MB 不在 [{a.min_mb}, {a.max_mb}] 区间，先查是不是量错了")
    ok(f"{a.log}：工作集 {mb} MB（区间 [{a.min_mb}, {a.max_mb}]）")


# ---------------------------------------------------------------- CLI

def main() -> int:
    ap = argparse.ArgumentParser(description="e2e 断言层：每条 PASS 都要有具体值")
    sub = ap.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("load-ok", help="session.load 的信封 + 结构 + 条数")
    p.add_argument("file")
    p.add_argument("--min-messages", type=int, default=1)
    p.add_argument("--min-total", type=int, default=1)
    p.add_argument("--permission", default=None)
    p.add_argument("--require-full", action="store_true",
                   help="要求 truncated=false（负向断言要覆盖整个会话，不能只看尾巴）")
    p.set_defaults(fn=cmd_load_ok)

    p = sub.add_parser("role-text", help="某角色正文里含某词（其它角色不算）")
    p.add_argument("file")
    p.add_argument("--role", required=True, choices=ROLES)
    p.add_argument("--contains", required=True)
    p.add_argument("--not-contains", default=None)
    p.set_defaults(fn=cmd_role_text)

    p = sub.add_parser("tool-ran", help="tool 消息的输出里含某词（args 不算）")
    p.add_argument("file")
    p.add_argument("--contains", required=True)
    p.add_argument("--no-error-prefix", action="store_true")
    p.set_defaults(fn=cmd_tool_ran)

    p = sub.add_parser("not-contains", help="负向断言（先要求结构成立）")
    p.add_argument("file")
    p.add_argument("--contains", required=True)
    p.set_defaults(fn=cmd_not_contains)

    p = sub.add_parser("has-running-turn", help="尾部有空 assistant = turn 在跑")
    p.add_argument("file")
    p.set_defaults(fn=cmd_has_running_turn)

    p = sub.add_parser("accepted", help="session.approve 的 accepted/optionId")
    p.add_argument("file")
    p.set_defaults(fn=cmd_accepted)

    p = sub.add_parser("cancelled", help="session.cancel 的 cancelled")
    p.add_argument("file")
    p.set_defaults(fn=cmd_cancelled)

    p = sub.add_parser("http-ok", help="HTTP 码 + result 信封")
    p.add_argument("file")
    p.add_argument("--code", required=True, help="curl -w %%{http_code} 写进的文件")
    p.add_argument("--expect", type=int, default=200)
    p.set_defaults(fn=cmd_http_ok)

    p = sub.add_parser("list-has", help="session.list 里有这个 id")
    p.add_argument("file")
    p.add_argument("--id", required=True)
    p.set_defaults(fn=cmd_list_has)

    p = sub.add_parser("shot", help="截图证据（日志 + 文件 + PNG 魔数 + 尺寸）")
    p.add_argument("png")
    p.add_argument("--log", required=True)
    p.add_argument("--min-bytes", type=int, default=8192)
    p.add_argument("--min-w", type=int, default=600)
    p.add_argument("--min-h", type=int, default=400)
    p.set_defaults(fn=cmd_shot)

    p = sub.add_parser("mem", help="工作集读数（取不到就是失败）")
    p.add_argument("--log", required=True)
    p.add_argument("--min-mb", type=float, default=5.0)
    p.add_argument("--max-mb", type=float, default=400.0)
    p.set_defaults(fn=cmd_mem)

    a = ap.parse_args()
    a.fn(a)
    return 0


if __name__ == "__main__":
    sys.exit(main())
