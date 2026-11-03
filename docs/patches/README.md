# 宿主侧改动（未提交，等授权）

这里放的是 `DSH-GO` 仓的改动 diff。按 `AGENTS.md` §4「commit/PR/push 由任务书显式授权」，
这些改动**没有提交到宿主仓**，只做了一份可恢复的备份。

## framesub-journal-replay.patch

`internal/transport/wssession.go`，两处：

1. **FrameSub 补 journal 重放**：客户端带 `sinceSeq` 订阅时，把缺的那段从 journal
   重放出去（**先重放、再 Subscribe**——重放期间 sub 还不在 `hub.topics` 里，
   `Publish` 不会推给它，所以不会有"新帧插到旧帧前"的乱序）。
2. **`sinceSeq >= 0` 也重放**：原先写成 `> 0`，导致**首次订阅**（sinceSeq=0）一条帧
   都不推——界面像卡住、水位永远是 0。改成 `>= 0` 后首次订阅能拿到历史。

## 怎么用

```bash
cd D:/DeepSeek Harness/default-workspace/DSH-GO
git apply --check ../dsh-ui/docs/patches/framesub-journal-replay.patch   # 先检查
git apply         ../dsh-ui/docs/patches/framesub-journal-replay.patch   # 再应用
```

如果目标工作区**已经包含**这些改动，`git apply --check` 会报 "already exists"；
用反向检查确认：`git apply --check --reverse <patch>` 能过就说明已经应用了。

## 验证过什么

- 探针 `tmp/wsprobe`：带 `sinceSeq=2` 订阅 → 从 `seq=3` 开始收到 3,4,5,…13
- 界面：首次订阅收到 seq 1..5；切回变成 `since=5 epoch=1`
- 宿主全量：`CGO_ENABLED=0 go test ./...` → **39 个包 ok，零 FAIL**

## 没验什么

`-race` 没跑（本机原先没有 C 工具链；2026-11-03 晚在装 mingw，装好后要补）。
