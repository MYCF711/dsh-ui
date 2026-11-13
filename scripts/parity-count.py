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

# 官方包清单。**它是分母的唯一来源，不许在别处硬编码数字。**
#
# 2026-11-14 修的是这个坑：原先分母写死在文档的合计行里（rc.1 时代是 55），
# 而 `--check` 只校验「合计行 == 本文条目数」——文档自洽就绿，
# **陈旧宇宙照样过**。官方 rc.2 实际有 66 个包，账本却按 55 在数。
#
# 所以现在：分母从这份清单**动态读**，并且逐 id 断言集合相等
# （官方有而文档没交代 -> 红；文档编了官方没有的名字 -> 红）。
OFFICIAL = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                        "docs", "evidence", "official-ui-plugins-0.2.2-rc.2.txt")

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


# 官方对账表的数据行：`| 12 | \`pkg-name\` | ✅ IMPLEMENTED | 落点 |`
TABLE_ROW = re.compile(r"^\|\s*(\d+)\s*\|\s*`([^`]+)`\s*\|\s*[✅🟡❌]\s*(IMPLEMENTED|PARTIAL|MISSING)\s*\|", re.M)


def official_packages():
    """读官方清单（每行一个包名）。读不到就抛——**不许静默退化成空**。"""
    with io.open(OFFICIAL, encoding="utf-8") as fh:
        return [line.strip() for line in fh if line.strip()]


def ledger_table(text):
    """从官方对账表里取出 {包名: 状态} 与原始顺序。"""
    out, order = {}, []
    for _, name, status in TABLE_ROW.findall(text):
        out[name] = status
        order.append(name)
    return out, order


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true", help="合计行和条目对不上就 exit 1")
    a = ap.parse_args()

    text = io.open(DOC, encoding="utf-8").read()

    # **以官方对账表为准**（2026-11-14 起）。
    #
    # 文档里历史上还有一份"短名"清单（`- ✅ \`settings\`` 那种），它是 rc.1 时代的
    # 口径、只有 55 条、且名字不与官方包一一对应——**那正是假绿的来源**。
    # 官方对账表按官方包名索引、66 条齐全，所以计数与分母都以它为准；
    # 短名那份保留为沿革说明，不再参与断言。
    table, order = ledger_table(text)
    if table:
        # 键要与上面的 ORDER（emoji）对齐，否则下面的打印会 KeyError。
        got = {"✅": 0, "🟡": 0, "⬜": 0, "❌": 0}
        mark = {"IMPLEMENTED": "✅", "PARTIAL": "🟡", "MISSING": "❌"}
        for status in table.values():
            got[mark[status]] += 1
        total = len(table)
    else:
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
    # ── 对官方清单断言（2026-11-14 新增：这才是真正守住"离官方多远"的三条）──
    try:
        official = official_packages()
    except OSError as exc:
        print("FAIL  读不到官方清单 %s：%s" % (OFFICIAL, exc))
        print("      分母必须来自官方清单；读不到就不许放行（宁可红，不许假绿）。")
        return 1
    table, order = ledger_table(text)
    if not table:
        print("FAIL  文档里找不到「官方包逐条对账」表。它的行形状必须是：")
        print("      | 12 | `dsh-client-ui-xxx` | ✅ IMPLEMENTED | 本仓落点 |")
        return 1

    missing = [p for p in official if p not in table]
    extra = [p for p in table if p not in set(official)]
    if missing:
        print("FAIL  官方有 %d 个包，账本没交代：" % len(missing))
        for name in missing[:12]:
            print("        %s" % name)
        if len(missing) > 12:
            print("        … 还有 %d 个" % (len(missing) - 12))
        return 1
    if extra:
        print("FAIL  账本里有 %d 个包在官方清单里不存在（编造的名字？）：" % len(extra))
        for name in extra[:12]:
            print("        %s" % name)
        return 1
    if order != official:
        print("FAIL  对账表顺序与官方清单不一致（第 %d 行起）。"
              % next(i + 1 for i, (a, b) in enumerate(zip(order, official)) if a != b))
        return 1

    # 合计行的分母必须等于官方清单长度——**动态算，不许硬编码**。
    if want[0] != len(official):
        print("FAIL  合计行的分母是 %d，官方清单是 %d 行。" % (want[0], len(official)))
        return 1
    got_table = {"IMPLEMENTED": 0, "PARTIAL": 0, "MISSING": 0}
    for status in table.values():
        got_table[status] += 1
    if [got_table[k] for k in ("IMPLEMENTED", "PARTIAL", "MISSING")] != [want[1], want[2], want[4]]:
        print("FAIL  合计行与官方对账表对不上：")
        print("      合计行：✅ %d · 🟡 %d · ❌ %d" % (want[1], want[2], want[4]))
        print("      对账表：✅ %d · 🟡 %d · ❌ %d"
              % (got_table["IMPLEMENTED"], got_table["PARTIAL"], got_table["MISSING"]))
        return 1

    print("PASS  合计与条目一致 · 官方清单 %d 个包逐 id 对齐（零缺零多）" % len(official))
    return 0


if __name__ == "__main__":
    sys.exit(main())
