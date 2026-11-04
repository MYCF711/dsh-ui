#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""scripts-index.py —— `scripts/` 下每个脚本都必须在 `docs/scripts.md` 里有一行。

# 为什么

`AGENTS.md` 那条规矩是「**写了不跑的探针等于没有**」：

> 2026-11-04：一个代理写好 34 项断言、真宿主四轮的真机脚本，写完**没跑过就交付了**；
> 隔了几轮真正跑第一次，当场抓到「一次 prompt 在界面上变成 2~3 轮」——
> 那个 bug 在单测里每一条都看起来对。

同一条规矩的两半：能进 CI 的进 CI；进不去的，**文档里要写清"谁会跑、什么时候跑"**，
否则它下个月就不存在了。

这个检查管的是后一半。它不判断"写得对不对"，只判断"**还有没有人知道它存在**"。

# 判据

`git ls-files scripts/` 里每个 `.sh` / `.py` / `.ps1`，都要在 `docs/scripts.md` 里出现
（以 `` `scripts/<名字>` `` 的形式）。未跟踪的文件不要求——它们还没入库。

用法：
    python scripts/scripts-index.py            # 打印缺登记的脚本
    python scripts/scripts-index.py --check    # CI 用：有缺就 exit 1
"""

import io
import os
import re
import subprocess
import sys

INDEX = 'docs/scripts.md'
EXTS = ('.sh', '.py', '.ps1')


def tracked_scripts():
    """库里（不是磁盘上）的脚本清单。未跟踪的不算——它们还没进仓库。"""
    out = subprocess.run(['git', 'ls-files', 'scripts/'],
                         capture_output=True, text=True, check=True).stdout
    names = []
    for line in out.splitlines():
        if not line.endswith(EXTS):
            continue
        base = os.path.basename(line)
        if base.startswith('_'):
            continue
        names.append(base)
    return sorted(set(names))


def indexed():
    if not os.path.exists(INDEX):
        return set()
    text = io.open(INDEX, encoding='utf-8').read()
    return set(re.findall(r'`scripts/([A-Za-z0-9_.-]+)`', text))


def main():
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass
    check = '--check' in sys.argv

    try:
        have_files = tracked_scripts()
    except Exception as e:
        print('跑不了 git ls-files：%s' % e)
        return 2

    have_rows = indexed()
    missing = [n for n in have_files if n not in have_rows]

    print('库里的脚本 %d 个，`%s` 登记了 %d 个' % (len(have_files), INDEX, len(have_rows)))

    if missing:
        print('\n这些脚本**没有任何地方说它什么时候该跑**：')
        for n in missing:
            print('  scripts/%s' % n)
        print('\n在 %s 里补一行（做什么 / 谁什么时候跑 / 在不在 CI）。' % INDEX)
        print('理由：一个没人知道该什么时候跑的脚本，下个月就只剩一个文件名了。')
        return 1 if check else 0

    print('PASS  每个脚本都有一行说明')
    return 0


if __name__ == '__main__':
    sys.exit(main())
