# 脚本索引：每个脚本**谁跑、什么时候跑**

> 规则来自 `AGENTS.md`：「**写了不跑的探针等于没有**」。新探针交付时要贴出**这一次运行**
> 的汇总行；能被自动跑的就接进门禁；接不进去的要在文档里写清"谁会跑、什么时候跑"，
> 否则它下个月就不存在了。
>
> 这张表就是那句话的落地：**`scripts/` 下每个脚本都必须在这里有一行**，
> 由 `python scripts/scripts-index.py --check` 机械校验（CI 里跑）。
> 新加脚本忘了登记 → 红。

## 在 CI 里自动跑的（不需要人记得）

| 脚本 | 做什么 | 谁跑 / 什么时候 | 在 CI |
|---|---|---|---|
| `scripts/gofmt`（内联步骤） | `gofmt -l` 门禁 | 每个 PR | ✅ |
| `scripts/parity-count.py` | 校验 `docs/official-parity.md` 的合计行与条目一致 | 每个 PR | ✅ |
| `scripts/doc-links.py` | 校验文档里的**相对链接**指得着（仓内必须存在；`tmp/` 与跨仓只 WARN） | 每个 PR | ✅ |
| `scripts/guard-selftest.sh` | 给每条守卫各弄坏一处，要求它们当场红（`git archive` 隔离副本）+ 原样对照组 | 每个 PR | ✅ |
| `scripts/scripts-index.py` | 这个索引本身：`scripts/` 下每个脚本都要有行 | 每个 PR | ✅ |
| `scripts/headcheck.sh` | 验证 **HEAD 单独**能不能编译/测试（不靠工作树） | CI 的「干净检出」步骤调它；人也能手动跑 | ✅ |

## 本地手动跑的：验收类

一条命令跑全部：`bash scripts/accept-all.sh`（约 13 分钟）。
改完一处跑 `--fast`（约 2 分钟），发版前跑全。

| 脚本 | 做什么 | 谁跑 / 什么时候 | 在 CI |
|---|---|---|---|
| `scripts/accept-all.sh` | 把下面这些串起来跑，最后给一张表；判据是**退出码** + 自报汇总行 | 人：改完一处 / 发版前 | ❌（要真窗口、真模型） |
| `scripts/accept-theme.sh` | 主题：两套令牌 + 运行时切换 + 系统跟随（无头两尺寸） | 由 `accept-all.sh` 串跑；改主题相关时单独跑 | ❌（要真窗口） |
| `scripts/accept-session-ops.sh` | 三个会话操作的真宿主验证：create / compact（含真压）/ fork / delete | 同上 | ❌（要真模型） |
| `scripts/accept-approval.sh` | 审批整链：`--policy ask` 下让模型调写类工具，从 journal 到 HTTP 批准 | 同上 | ❌（要真模型） |
| `scripts/accept-cancel.sh` | 取消整链：长生成 → 取消 → 帧序与收尾 | 同上 | ❌（要真模型） |
| `scripts/accept-resume.sh` | 续历史 / 断线恢复：换一条连接按 `sinceSeq` 补洞 | 同上 | ❌（要真模型） |
| `scripts/accept-chain.sh` | 整链终验：一个真 `dshd` + 临时 home，逐项打勾 | 同上 | ❌（要真模型） |
| `scripts/e2e-workday.sh` | 「连续使用」的自动化版本（22 项）：建会话 → 发 prompt → 工具 → 审批 → 取消 | 同上 | ❌（要真模型 + 真窗口） |
| `scripts/e2e-probe-selftest.sh` | 给 e2e 断言层做反证：每条探针在坏输入上必须红 | 由 `accept-all.sh` **第二位**串跑（第一位是守卫反证） | ❌（依赖 e2e 夹具） |

## 本地手动跑的：证据 / 测量类

这些**不产生 PASS/FAIL 门禁**，只产出证据或数字。跑的时候要人看结论。

| 脚本 | 做什么 | 谁跑 / 什么时候 | 在 CI |
|---|---|---|---|
| `scripts/bin-check.py` | 真机前先验「这个二进制与源码一致」（陈旧的会把自身问题伪装成产品缺陷） | **每次真机第 0 步**；`accept-chain.sh` 里也会调（在宿主仓里跑） | ❌ |
| `scripts/perf-ui.sh` | 界面性能/内存基线（实测，不估算） | 改性能相关或需要 A/B 时；结论进 `docs/perf.md` | ❌（要真窗口） |
| `scripts/perf-gap.sh` | `docs/perf-gap-plan.md` 里「零改动就能测」的那部分 | 同上 | ❌ |
| `scripts/slot-proof.sh` | 公开槽「实画」的可复现证据（默认槽 / F1 浮层 / 逐槽裁剪） | 改了槽位装配或面板布局时 | ❌（要真窗口） |
| `scripts/shot.py` | 截一张窗口图：按窗口标题找窗，裁掉窗外框 | 手动取证时 | ❌ |
| `scripts/visual-diff.py` | 对同尺寸 PNG 做确定性区域测量与逐像素 diff | 视觉回归核对时 | ❌ |
| `scripts/visual-regions.py` | **像素判据**：9 个产品形态真机截图 × 显式区域，与 v0.5.1 基线逐区域比像素差并按阈值判红（结构 2% / 正文 5% / 整窗 1%） | 改渲染调用、Viva 升级、发布前；结论进 `docs/handoff-viva-0.6.0.md` §13 | ❌（要真窗口 + 真宿主） |
| `scripts/e2e-probe.py` | e2e 探针的断言层（每条 PASS 都必须能变红） | 被 `e2e-workday.sh` / `e2e-probe-selftest.sh` 调用 | ❌ |
| `scripts/smoke-u1.ps1` | U1 的冒烟（PowerShell；`-Acp` 走 ACP 分支） | 手动冒烟时 | ❌ |

## 谁调谁（调用关系，2026-11-04 实测）

```
accept-all.sh ──┬─ guard-selftest.sh            （第一位：守卫自己没被反证过时，后面的绿都不算数）
                ├─ e2e-probe-selftest.sh ── e2e-probe.py
                ├─ accept-theme.sh
                ├─ accept-session-ops.sh
                ├─ accept-approval.sh
                ├─ accept-cancel.sh
                ├─ accept-resume.sh
                ├─ accept-chain.sh ── ../DSH-GO/scripts/bin-check.py   （在宿主仓里跑）
                └─ e2e-workday.sh ── e2e-probe.py
```

`perf-*.sh` / `slot-proof.sh` / `shot.py` / `visual-diff.py` / `visual-regions.py` / `smoke-u1.ps1` **不在任何链路里**，
是独立取证工具——所以它们的行必须写清"什么时候跑"，否则就是没人跑。

> `visual-regions.py` 有一条**进 CI 的**配套守卫：`cmd/app/visual_regions_contract_test.go`
> 把 `testdata/visual-v06/regions.json` 的每个矩形和 `uikernel.ComputeShellLayout`
> 的真实几何逐像素对拍（骨架改了、区域没跟上 → 红），并接进 `guard-selftest.sh`。
> 脚本本身要真窗口 + 真宿主，跑不了 CI；但"区域定义已经和布局分叉"这件事每次 PR 都会被抓到。

## 归属说明

- `scripts/real-chain*.{sh,py}`（真 `dshd` + 真 UI 进程的整条产品链）由 REAL-CHAIN 那一包交付，
  它自带 `docs/real-chain.md` 写清了怎么跑、证据在哪。入库后**要补进这张表**。
- 这张表只承诺"**有人知道它该什么时候跑**"，不承诺"它现在一定是绿的"——
  绿不绿看它自己的汇总行。

## 怎么维护

```bash
python scripts/scripts-index.py --check     # CI 跑这个
python scripts/scripts-index.py             # 只打印：哪些脚本没登记
```

新加脚本时：先在 `scripts/` 里写，再在**这张表**里加一行（做什么 / 谁什么时候跑 / 在不在 CI）。
漏了这一步，CI 会点名。
