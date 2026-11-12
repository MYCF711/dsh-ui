# smoke-u1.ps1 —— U1 能自动验的那部分，一条命令跑完。
#
# 用法：powershell -NoProfile -File scripts/smoke-u1.ps1 [-Acp]
#   -Acp  接真 dshd（慢，要 60s+）；不带就是假数据（3 秒出结果）
#
# 它只做**机器能判断**的事：进程起没起、内存多少、点得到不到、切不切得过去。
# 滚动手感、滚动位置回没回原位这类**必须人看**，这里不假装验。

param([switch]$Acp)

$ErrorActionPreference = 'Stop'
# 控制台默认是 GBK，中文会乱码
[Console]::OutputEncoding = [Text.Encoding]::UTF8
$OutputEncoding = [Text.Encoding]::UTF8
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$tmp  = $env:TEMP
$fail = 0
$pass = 0

function Ok($name, $detail) { Write-Host ("  [ok] " + $name + " — " + $detail) -ForegroundColor Green; $script:pass++ }
function No($name, $detail) { Write-Host ("  [FAIL] " + $name + " — " + $detail) -ForegroundColor Red; $script:fail++ }
function Step($t) { Write-Host ("`n== " + $t) -ForegroundColor Cyan }

Add-Type @'
using System; using System.Runtime.InteropServices;
public class U1 {
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x,int y);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
  [DllImport("user32.dll")] public static extern void mouse_event(uint f,uint dx,uint dy,uint d,IntPtr e);
  public struct RECT { public int L,T,R,B; } }
'@

function ClickAt($h, $cx, $cy) {
  $r = New-Object U1+RECT
  [U1]::GetWindowRect($h, [ref]$r) | Out-Null
  [U1]::SetCursorPos(($r.L + $cx), ($r.T + $cy)) | Out-Null
  Start-Sleep -Milliseconds 70
  [U1]::mouse_event(0x0002,0,0,0,[IntPtr]::Zero)
  Start-Sleep -Milliseconds 40
  [U1]::mouse_event(0x0004,0,0,0,[IntPtr]::Zero)
  Start-Sleep -Milliseconds 300
}
function Mem($name) {
  $p = Get-Process $name -ErrorAction SilentlyContinue
  if (-not $p) { return $null }
  return [math]::Round(($p | Measure-Object WorkingSet64 -Sum).Sum / 1MB, 1)
}

Step '清场'
# 进程名 = 产品名全程 `DSH GUI GO`（exe 是 `DSH GUI GO.exe`，见 cmd/app/brand.go）。
# PowerShell 里带空格的进程名**必须加引号**，不引会被当成两个参数。
Get-Process 'DSH GUI GO', mockd -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2
Ok 'cleanup' 'killed old DSH GUI GO/mockd'

Step '起 mockd'
$mockdArgs = @()
if ($Acp) {
  $mockdArgs = @('-sessions','"D:\DeepSeek Harness\home\sessions"',
                 '-acp','"D:\DeepSeek Harness\default-workspace\DSH-GO\dshd.exe"',
                 '-acp-manifest','"' + $root + '\tmp\dsh-acp.toml"')
} else {
  $mockdArgs = @('-sessions','"D:\DeepSeek Harness\home\sessions"')
}
$t0 = Get-Date
Start-Process -FilePath "$tmp\mockd.exe" -ArgumentList ($mockdArgs -join ' ') -WindowStyle Hidden
for ($i = 0; $i -lt 300; $i++) {
  try { $null = Invoke-RestMethod -Uri 'http://127.0.0.1:7788/healthz' -TimeoutSec 1; break } catch { Start-Sleep -Milliseconds 100 }
}
$ready = [int]((Get-Date) - $t0).TotalMilliseconds
if ($ready -lt 5000) { Ok 'mockd ready' "$ready ms" } else { No 'mockd ready' "$ready ms（>5s）" }

Step '起 DSH GUI GO'
$t0 = Get-Date
Start-Process -FilePath "$tmp\DSH GUI GO.exe"
# 等**窗口真的出现**（MainWindowHandle 非 0），不是"进程存在"——
# 进程起来到窗口出现之间还有几百毫秒，报"进程存在"的时间会骗人。
for ($i = 0; $i -lt 300; $i++) {
  $p = Get-Process 'DSH GUI GO' -ErrorAction SilentlyContinue
  if ($p -and $p.MainWindowHandle -ne 0) { break }
  Start-Sleep -Milliseconds 100
}
$up = [int]((Get-Date) - $t0).TotalMilliseconds
if ($up -lt 3000) { Ok 'app up' "$up ms" } else { No 'app up' "$up ms（>3s）" }
Start-Sleep -Seconds 6

$m = Mem 'DSH GUI GO'
if ($m -lt 110) { Ok 'memory' "$m MB（目标 <110）" } else { No 'memory' "$m MB（>110）" }

$p = Get-Process 'DSH GUI GO'
$threads = $p.Threads.Count
if ($threads -lt 80) { Ok 'threads' "$threads" } else { No 'threads' "$threads（>80，可能泄漏）" }

Step '点击切换会话'
$h = $p.MainWindowHandle
[U1]::SetForegroundWindow($h) | Out-Null
Start-Sleep -Milliseconds 500
$before = (Get-Process 'DSH GUI GO').MainWindowTitle
ClickAt $h 60 118
ClickAt $h 60 148
ClickAt $h 60 118
$after = (Get-Process 'DSH GUI GO').MainWindowTitle
if ($after -eq 'DSH GUI GO') { Ok 'click switch' '窗口仍在、没崩' } else { No 'click switch' "标题变了：$after" }

$m2 = Mem 'DSH GUI GO'
if ($m2 -lt ($m + 15)) { Ok 'memory after switch' "$m -> $m2 MB" } else { No 'memory after switch' "$m -> $m2 MB（涨太多）" }

Step '汇总'
Write-Host ""
Write-Host ("  pass=" + $pass + "  fail=" + $fail)
if ($fail -gt 0) { exit 1 }
