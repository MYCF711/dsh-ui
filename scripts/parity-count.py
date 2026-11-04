#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""parity-count.py —— 数 `docs/official-parity.md` 的账。

# 为什么要有它

那份清单是"我们离官方还有多远"的**唯一账本**。账本上有两种错，都不是小事：

- 写着 ❌ 而代码里其实有 —— 后来的人会**重做一遍**；
- 合计那几个数是手填的 —— 每补完一个部件都没人回头改，于是**越错越久**。

2026-11-04 一次性查出九处：`secret-mask`、`message-feedback`、`open-in-app`、
`brand-official`、`input-trigger`、`sidebar-right` 已经从 ❌ 落下去了，
`session`/`renderer`/`deepseek-model-label-1017` 是 React 专属或临时角标，
本就不该记成"缺"。而表头还写着"我们有 10 个插件 · 缺 53 个"。

所以这个脚本做两件事：
  1. 从条目行数出真实分布（同一部件多行时取**最高**状态）；
  2. `--check` 时要求文档里那行合计和数出来的一致——**合计从此不能手改**。

用法：
    python scripts/parity-count.py            # 打印
    python scripts/parity-count.py --check    # CI 用：对不上就 exit 1
"""

import argparse
import io
import os
import re
import sys

# 路径按**脚本所在位置**算，不按当前目录：CI 从仓库根跑，人可能从任何地方跑。
DOC = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                   "docs", "official-parity.md")

# Windows 上的 Python 默认可能不是 UTF-8（cp1252/cp936），而这个脚本满屏中文。
# 不显式设的话，CI 里会是 UnicodeEncodeError —— 那看起来像"脚本坏了"，
# 实际只是输出的编码不对。这在本仓是个反复出现的坑。
try:
    sys.stdout.reconfigure(encoding="utf-8")
except Exception:
    pass

# 同一部件可能在不同分组里各列一行；按"最好的那个状态"算——
# 一个部件要么做了一半要么没做，两行都得认。
RANK = {"✅": 0, "🟡": 1, "❌": 2, "⬜": 3}
ORDER = ["✅", "🟡", "⬜", "❌"]
LABEL = {"✅": "做完", "🟡": "一半", "⬜": "不该有", "❌": "缺"}

ROW = re.compile(r"^- (✅|🟡|❌|⬜) `([^`]+)`", re.M)
TOTAL = re.compile(r"^## 合计：官方 (\d+) 个部件 —— ✅ (\d+) · 🟡 (\d+) · ⬜ (\d+)（不该有） · ❌ (\d+)$", re.M)


def counts(text):
    best = {}
    for status, name in ROW.findall(text):
        if name not in best or RANK[status] < RANK[best[name]]:
            best[name] = status
    out = {k: 0 for k in ORDER}
    for status in best.values():
        out[status] += 1
    return out, len(best)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true", help="合计行和条目对不上就 exit 1")
    a = ap.parse_args()

    text = io.open(DOC, encoding="utf-8").read()
    got, total = counts(text)

    line = " · ".join("%s %d（%s）" % (k, got[k], LABEL[k]) for k in ORDER)
    print("条目去重后 %d 个部件：%s" % (total, line))

    if not a.check:
        return 0

    m = TOTAL.search(text)
    if not m:
        print("FAIL  找不到合计行。它的形状必须是：")
        print("      ## 合计：官方 N 个部件 —— ✅ A · 🟡 B · ⬜ C（不该有） · ❌ D")
        return 1
    want = [int(x) for x in m.groups()]
    have = [total] + [got[k] for k in ORDER]
    if want != have:
        print("FAIL  合计行和条目对不上：")
        print("      文档写的是：官方 %d · ✅ %d · 🟡 %d · ⬜ %d · ❌ %d" % tuple(want))
        print("      数出来的是：官方 %d · ✅ %d · 🟡 %d · ⬜ %d · ❌ %d" % tuple(have))
        print("      合计是数出来的，不要手改——改完条目重跑这个脚本，把新的一行贴上。")
        return 1
    print("PASS  合计与条目一致")
    return 0


if __name__ == "__main__":
    sys.exit(main())
