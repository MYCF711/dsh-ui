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

# 用法

    # 1) 先起宿主（临时 home + 真 dshd），拿到 cookie 与会话 id
    #    见 docs/handoff-viva-0.6.0.md §13（V-06-5）的完整命令
    export VISUAL_ADDR=http://127.0.0.1:60630
    export VISUAL_COOKIE='dshd_session=…'
    export VISUAL_SESSION=003991cf-f8d0-4c31-8179-1f4e0eba7b1f

    # 2) 当前树截图 → 与基线比
    python scripts/visual-regions.py --binary tmp/.../dsh-ui.exe

    # 3) 生成/重刷基线（**只有复核过差异之后**才做）
    python scripts/visual-regions.py --binary … --update

    # 指定基线目录（对拍 v0.5.1 时用）
    python scripts/visual-regions.py --binary … --baseline tmp/visual-v06/baseline-v051

退出码：0 = 全绿；1 = 有区域超阈值；2 = 输入/环境错误（不是"画面不对"）。
"""

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

SCHEMA_VERSION = 1
TOOL_VERSION = 1

# 这些形态由 `cmd/app` 的命令行开关驱动（`-shot` 是同一条产品路径的无头出口）。
# 新增可见部件时**必须**在这里加一帧，否则它永远不会被像素判据覆盖。
DEFAULT_CONTRACT = Path("testdata/visual-v06/regions.json")
DEFAULT_BASELINE = Path("testdata/visual-v06/baseline")
SHOT_TIMEOUT_SEC = 120


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
    parser.add_argument("--binary", type=Path, required=True,
                        help="被验的 app 可执行文件（已构建好的产品二进制）")
    parser.add_argument("--contract", type=Path, default=DEFAULT_CONTRACT)
    parser.add_argument("--baseline", type=Path, default=DEFAULT_BASELINE,
                        help="基线 PNG 目录")
    parser.add_argument("--out", type=Path, default=Path("tmp/visual-regions"),
                        help="本次截图与差分的落盘目录")
    parser.add_argument("--only", default="",
                        help="只跑这些状态（逗号分隔）")
    parser.add_argument("--update", action="store_true",
                        help="用本次截图覆盖基线（复核过差异之后才准用）")
    parser.add_argument("--version", action="version", version=f"%(prog)s {TOOL_VERSION}")
    args = parser.parse_args(argv)

    contract = load_contract(args.contract)
    env = {
        "addr_default": os.environ.get("VISUAL_ADDR", ""),
        "cookie_default": os.environ.get("VISUAL_COOKIE", ""),
        "addr_empty": os.environ.get("VISUAL_ADDR_EMPTY", ""),
        "cookie_empty": os.environ.get("VISUAL_COOKIE_EMPTY", ""),
        "session": os.environ.get("VISUAL_SESSION", ""),
    }
    if not env["addr_default"]:
        raise InputError("没有 VISUAL_ADDR（宿主的 base URL）")
    if not env["session"]:
        raise InputError("没有 VISUAL_SESSION（要打开的会话 id）")

    binary = args.binary
    if not binary.exists():
        raise InputError(f"二进制不存在：{binary}")

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
        "states": [s["name"] for s in states],
        "thresholds": contract["thresholds"],
        "rows": all_rows,
        "failures": failures,
    }, ensure_ascii=False, indent=2, allow_nan=False) + "\n", encoding="utf-8")

    total = len(all_rows)
    print(f"\n== 区域合计 {total}，超阈值 {failures}（报告 {report}）==")
    return 1 if failures else 0


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
