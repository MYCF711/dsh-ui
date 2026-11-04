#!/usr/bin/env python3
"""bin-check.py —— 真机测试前先验「用的二进制是不是当前源码编出来的」。

为什么要这个：2026-11-03 我用 `tmp/dshd-serve.exe` 测 H13 审批，得到
「ask 策略下子进程在终端 stdin 问 [y/N、WS 帧推不出来」的结论，写进了文档。
后来重建才发现**那个二进制是 M244 的**，宿主 HEAD（M272）早就实现了
approval_stdio + WS 推送——整条结论是错的，浪费了一轮。

这不是"小心一点"能解决的问题：二进制没有版本指纹，跑起来和新的长得一样。
所以用**时间戳**做最粗但有效的判据：二进制必须比它对应的所有源码都新。

用法：
    python scripts/bin-check.py            # 检查，陈旧就 exit 1
    python scripts/bin-check.py --warn     # 只警告不失败（CI 里给提示用）

映射（二进制 → 源码目录）：
    tmp/dshd*.exe           → ../DSH-GO/cmd/dshd, ../DSH-GO/internal, ../DSH-GO/modules, ../DSH-GO/codecs
    tmp/dshgogui*.exe        → cmd/app, internal, plugins, pluginapi
    tmp/app*.exe            → 同上（`app.exe` 是旧名，历史产物还在 tmp/ 里，照样体检）
    tmp/mockd*.exe          → cmd/mockd, internal

**名字是统一的两根**：界面 `dshgogui.exe`、宿主 `dshd.exe`。
以前每个脚本各编一个名字（`dshd-acc` / `dshd-goal` / `dshd-term` / `dshd-serve`……），
于是这份体检得靠前缀碰运气去猜哪个对应哪份源码——现在前缀就是产品名。
"""
from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
GO_EXT = {".go", ".mod", ".sum"}

# 二进制前缀 → 它吃哪些源码目录（相对 dsh-ui 根；.. 表示兄弟仓）
MAPPING: dict[str, list[str]] = {
    "tmp/dshgo": [
        "../DSH-GO/cmd/dshd", "../DSH-GO/internal",
        "../DSH-GO/modules", "../DSH-GO/codecs", "../DSH-GO/go.mod",
    ],
    "tmp/dshgogui": ["cmd/app", "internal", "plugins", "pluginapi", "go.mod"],
    "tmp/app": ["cmd/app", "internal", "plugins", "pluginapi", "go.mod"],
    "tmp/mockd": ["cmd/mockd", "internal", "go.mod"],
}

# 前缀 → 重建它的 go 包。**不能从文件名推**：`dshgogui` 拆前缀会得到 `dsh`（不存在的包），
# 那条提示是坏了之后人照着敲的第一条命令，错一个字就会让人以为"源码没了"。
PKG: dict[str, str] = {
    "tmp/dshgo": "./cmd/dshd",
    "tmp/dshgogui": "./cmd/app",
    "tmp/app": "./cmd/app",
    "tmp/mockd": "./cmd/mockd",
}


def newest_source_mtime(paths: list[str]) -> tuple[float, str]:
    """返回这些路径下最新一个源码文件的 (mtime, 路径)。目录不存在就跳过。"""
    newest, where = 0.0, ""
    for rel in paths:
        p = (ROOT / rel).resolve()
        if not p.exists():
            continue
        if p.is_file():
            m = p.stat().st_mtime
            if m > newest:
                newest, where = m, str(p)
            continue
        for f in p.rglob("*"):
            # `_test.go` 不参与构建：改了它不需要重建二进制。
            # 不排除的话，任何人加个测试都会让所有二进制"陈旧"——误报比不报更糟。
            if f.name.endswith("_test.go"):
                continue
            if f.suffix in GO_EXT and f.is_file():
                m = f.stat().st_mtime
                if m > newest:
                    newest, where = m, str(f)
    return newest, where


def check_one(exe: Path, sources: list[str], pkg: str) -> tuple[bool, str]:
    if not exe.exists():
        return True, f"{exe.name}: 不存在（跳过）"
    src_mtime, where = newest_source_mtime(sources)
    if src_mtime == 0:
        return True, f"{exe.name}: 找不到对应源码（跳过）"
    exe_mtime = exe.stat().st_mtime
    if exe_mtime + 1.0 >= src_mtime:  # 1s 容差：同一次构建里 mtime 可能同秒
        return True, f"{exe.name}: OK（二进制比最新源码新）"
    delta = src_mtime - exe_mtime
    rel = exe.relative_to(ROOT) if str(exe).startswith(str(ROOT)) else exe
    return False, (
        f"{exe.name}: **陈旧 {delta/60:.0f} 分钟**——源码 {where} 比它新。\n"
        f"    重建：CGO_ENABLED=0 go build -o {rel} {pkg}"
    )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--warn", action="store_true", help="只警告，不返回非零")
    ap.add_argument("--all", action="store_true",
                    help="连 tmp/ 下的历史调试二进制一起查（默认只查真机常用的那几个）")
    args = ap.parse_args()

    failures = 0
    checked = 0
    # 默认只查"真机验收会用到"的那几个；tmp/ 里几十个历史调试产物打出来全是噪音，
    # 噪音大了这条检查就没人看了——那还不如不做。
    # 前两个是统一命名后的正式产物；后三个是旧名，留着是为了让历史产物不会
    # 因为"改名了所以不在名单里"而静默逃过体检。
    default_names = {
        "dshd.exe", "dshgogui.exe",
        "dshd-serve.exe", "app-real.exe", "mockd-ap.exe",
    }
    for pattern, sources in MAPPING.items():
        base = ROOT / pattern
        cands = [base] if base.exists() else sorted(base.parent.glob(base.name + "*.exe"))
        if not args.all:
            cands = [c for c in cands if c.name in default_names]
        for exe in cands:
            if not exe.is_file():
                continue
            checked += 1
            good, msg = check_one(exe, sources, PKG[pattern])
            print(("  OK   " if good else "  STALE ") + msg)
            if not good:
                failures += 1

    if checked == 0:
        print("  没找到任何待检二进制（tmp/*.exe）")
        return 0
    if failures:
        print(f"\n{failures} 个二进制陈旧。**先重建再跑真机**，否则结论可能是错的。")
        return 0 if args.warn else 1
    print(f"\n{checked} 个二进制都不比源码旧。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
