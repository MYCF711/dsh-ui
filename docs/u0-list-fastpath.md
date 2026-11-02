# 给宿主线：`session.list` 从 4.37 s 降到 624 ms 的具体做法

> 2026-11-03 · 回应 `DSH-GO/AGENTS.md` 里记的「`session.list` 跑了 4.37 秒，根因在
> `internal/session/list.go:119` 用 `v4.ReadFile` 全量解码」。
>
> **这是 `dsh-ui` 侧已经写好的东西**，数字和方法都是我这边实测的，直接抄就行。

---

## 一、结论先给

| 做法 | 183 个会话耗时 |
|---|---:|
| 全量解码（现在的 `v4.ReadFile`） | **4.37 s**（宿主线实测）/ **6.1 s**（我这边 178 个） |
| **只解前 64KB 明文** | **624 ms** |

**差 7–10 倍。** 而且 `session.list` 要的字段（ID / cwd / createdAt）**全在 header 里**，
根本不需要解正文。

---

## 二、做法：给解码器加一个 `headerOnly`

我现在用的（`cmd/mockd/realsession.go`）：

```go
func decodeSession(path string, tail int, headerOnly bool) (*realSession, error) {
    raw, err := os.ReadFile(path)
    if err != nil { return nil, err }
    dec, err := zstd.NewReader(bytes.NewReader(raw))
    if err != nil { return nil, err }
    defer dec.Close()

    var out bytes.Buffer
    if headerOnly {
        // 只解前 64KB 明文：够 header 一行 + 第一条 user 消息
        if _, err := io.CopyN(&out, dec, 65536); err != nil && out.Len() == 0 && err != io.EOF {
            return nil, err
        }
    } else if _, err := out.ReadFrom(dec); err != nil && out.Len() == 0 {
        return nil, err // 一帧都没解出来才算坏；尾部半帧忽略
    }
    // ...再按行解析
}
```

**三个关键点**：

1. **`io.CopyN` 而不是 `ReadFrom`** —— `ReadFrom` 会把整个 zstd 流解到 EOF。
   `CopyN` 解够 64KB 就停，`dec.Close()` 不会报错。
2. **`err != io.EOF` 才判错** —— 读到 64KB 边界时 `CopyN` 会返回 `io.EOF`，
   这不是错误。**只在一帧都没解出来时才算坏文件。**
3. **`out.Len() == 0` 是"坏"的唯一判据** —— 和全量解码用同一个判据，
   这样"尾部半帧"在两种模式下行为一致。

---

## 三、为什么是 64KB（不是 4KB）

我一开始用 4KB，只够 header 一行，标题只能回落到 cwd：

```
DSH-GO
长期记忆系统
default workspace
```

改成 **64KB** 之后能拿到第一条 user 消息，标题变成真实的第一句话：

```
持续调研 AI / Agent / AGI / 人类等载体的「记忆…
修改一下当前工作区的AGENTS，做好区分，之前…
找一下我本次重装DSH前的DSH插件，重装是因为…
```

**182 个会话里 180 个变成了真标题**（剩下 2 个是会话本身没 user 消息）。

对宿主线：`session.list` 的 `Title` 如果也想给真话，用 64KB 这个预算就够。
**如果只要 ID/cwd/时间，4KB 就够，还能更快。**

---

## 四、我这边没做的两件事（宿主线要自己判断）

### ① 没有"只读首帧"的真实现

我用的是"解前 64KB 明文"，**不是**"只解第一帧"。

真正的"只读首帧"要能识别 zstd 帧边界——`klauspost/compress/zstd` 的
`Decoder.DecodeAll` 是整块解，没有公开的"解一帧就停"的 API。

**64KB 明文 ≈ 前几帧**（header 那帧很小），实测已经够。如果宿主线要更极致的做法，
得自己找帧边界（zstd magic `0xFD2FB528`），但**收益不一定值得**——
剩下的时间大头可能是 `os.ReadFile` 本身（184 个文件 × 约 2MB = 300MB IO）。

**建议先量一次：`os.ReadFile` 占多少、zstd 解码占多少。** 如果 IO 是大头，
只解首帧也救不了。

### ② 没有缓存

每次 `session.list` 都重新扫一遍。真实场景里会话文件几分钟才变一次，
**加一个按 mtime 的缓存**能把第二次起降到接近 0。

我这边没做是因为 mockd 是过渡管，不值得。宿主侧是真产品，值得。

---

## 五、一个必须保留的行为

**坏文件不能整个失败。**

我现在是：扫到文件 → 解 → 失败就 `bad++` → 继续下一个，最后报「扫到 N 个，跳过 M 个坏文件」。

宿主线如果改成"只解 header"，要保证**判坏的阈值一致**：
- 一帧都没解出来 → 坏
- 解出一部分但 JSON 解析失败 → **跳过这一行，不算坏**（我踩过：一条坏行废掉整个会话）

---

## 六、可以直接抄的清单

```go
// 1) 加一个 headerOnly 参数（或另开一个 DecodeHeader 函数）
// 2) 用 io.CopyN(&out, dec, 64<<10) 代替 ReadFrom
// 3) 判错条件：out.Len() == 0 && err != io.EOF
// 4) headerOnly 模式下，拿到第一条 user 消息就 early-return
// 5) 坏文件只计数不中断
```

**验证方法**：拿同一份 183 个会话的语料，跑两次（改前 / 改后），
两个数字都贴出来。我这边是 `6.1 s → 624 ms`，宿主线应该是 `4.37 s → 600 ms 左右`。
