#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""doc-links.py —— 检查 `docs/*.md` 与根目录 `*.md` 里的**相对链接**是否指得着。

# 为什么

这个仓的文档里塞了 300 多条相对链接，指向源码、测试数据、以及本机的 tmp/ 证据。
它们坏掉的时候**没有任何东西会说话**：

- 源码搬了家（`plugins/dsconversation/brand_path.go` → `plugins/dsbrand/`），
  文档里的链接照旧写着老路径——点进去 404，而读者只会以为"文档过期了"，
  于是整份文档的可信度一起下降；
- 更贵的一种：文档**承诺了证据**（"两尺寸截图见 xxx.png，26 029 字节，sha256 65a3…"），
  而那个文件从来没入库。链接是坏的，但正文读起来像是验过了。
  2026-11-04 真的撞到：`docs/reasoning-ui.md` 的四张图一条都不在仓库里。

# 两类链接，处理方式不同

| 指向 | 判据 |
|---|---|
| 仓内文件（源码 / 测试数据 / 已入库的图片） | **必须存在**，否则 exit 1 |
| `tmp/…`（gitignored，本机证据） | 存在就算好，不存在只 **WARN**——干净的检出里本来就没有它们 |

把 `tmp/` 那些也算失败的话，这个检查在 CI 上永远红，然后就会被关掉——
那是本仓最不想要的结局（一条永远红的门禁等于没有门禁）。

用法：
    python scripts/doc-links.py            # 打印
    python scripts/doc-links.py --check    # CI 用：有坏链就 exit 1（WARN 不影响退出码）
"""

import argparse
import glob
import io
import os
import re
import sys

# 只认 markdown 的内联链接 `[text](target)`；跳过图片语法里的同类目标（同一正则覆盖）。
LINK = re.compile(r'\]\(([^)\s]+)\)')

SKIP_SCHEMES = ('http://', 'https://', '#', 'mailto:', 'file:')


def is_local_evidence(target):
    """指向 `tmp/` 的链接是本机证据：gitignored，干净检出里没有是正常的。"""
    return target.startswith('tmp/') or target.startswith('/tmp/') or '/tmp/' in target


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--check', action='store_true', help='有坏链就 exit 1')
    a = ap.parse_args()

    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

    docs = sorted(glob.glob('*.md')) + sorted(glob.glob('docs/*.md'))
    if not docs:
        print('找不到任何 .md（在仓库根跑这个脚本）')
        return 2

    total = 0
    errors = []
    warns = []
    for path in docs:
        base = os.path.dirname(path)
        text = io.open(path, encoding='utf-8').read()
        for m in LINK.finditer(text):
            target = m.group(1)
            if target.startswith(SKIP_SCHEMES):
                continue
            target = target.split('#')[0]
            if not target:
                continue
            total += 1
            resolved = os.path.normpath(os.path.join(base, target))
            if os.path.exists(resolved):
                continue
            if is_local_evidence(target):
                warns.append((path, target))
            else:
                errors.append((path, target, resolved))

    print('相对链接 %d 条：坏 %d 条，本机证据缺失 %d 条' % (total, len(errors), len(warns)))

    if warns:
        print('\nWARN 这些指向 tmp/（gitignored，本机证据）——干净检出里没有，不算坏链：')
        for path, target in warns[:10]:
            print('  %-32s → %s' % (path, target))
        if len(warns) > 10:
            print('  ……还有 %d 条' % (len(warns) - 10))

    if errors:
        print('\nFAIL 这些链接指向仓内文件，但那个路径不存在：')
        for path, target, resolved in errors:
            print('  %-32s → %s' % (path, target))
            print('  %-32s   （解析成 %s）' % ('', resolved))
        print('\n要么改链接、要么把文件补进来。**文档承诺了证据而证据不在仓库里**时，')
        print('读者会以为已经验过——这比"文档过期"更贵。')
        if a.check:
            return 1
        return 0

    print('PASS  仓内链接全部指得着')
    return 0


if __name__ == '__main__':
    sys.exit(main())
