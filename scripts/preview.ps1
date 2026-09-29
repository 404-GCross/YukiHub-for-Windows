#requires -Version 5.1
<#
  重新构建并启动 YukiHub 桌面应用（预览用）。
  直接双击 scripts\preview.bat 即可运行本脚本。

  参数：
    -Check         只检查工具链，不构建、不启动
    -RunOnly       跳过构建，直接启动已有的 bin\YukiHub.exe
    -SkipFrontend  跳过前端打包（前端没改动时更快）

  说明：本机 Go / wails3 / pnpm 均为便携版，未加入系统 PATH，
        脚本会在当前进程内临时注入环境变量，不影响系统设置。
#>
[CmdletBinding()]
param(
  [switch]$Check,
  [switch]$RunOnly,
  [switch]$SkipFrontend
)

$ErrorActionPreference = "Stop"
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 }
catch { }

$root = Split-Path -Parent $PSScriptRoot
$tools = Join-Path $env:USERPROFILE ".workbuddy\tools"

$goRoot = Join-Path $tools "go-dl\extract\go"
$goPath = Join-Path $tools "gopath2"
$goCache = Join-Path $tools "gocache3"
$corepack = Join-Path $tools "corepack"
$wailsBin = Join-Path $tools "gopath3\bin"
$pnpmBin = Join-Path $tools "corepack-bin"
$mingwBin = Join-Path $tools "mingw\mingw64\bin"

$env:GOROOT = $goRoot
$env:GOPATH = $goPath
$env:GOCACHE = $goCache
$env:GOMODCACHE = Join-Path $goPath "pkg\mod"
$env:COREPACK_HOME = $corepack
$env:CGO_ENABLED = "1"
$env:PATH = (@("$goRoot\bin", $wailsBin, $pnpmBin, $mingwBin) -join ";") + ";" + $env:PATH

Write-Host "YukiHub 预览" -ForegroundColor Magenta
Write-Host "项目目录: $root"
Write-Host ""

$go = Get-Command go -ErrorAction SilentlyContinue
$wails = Get-Command wails3 -ErrorAction SilentlyContinue
$pnpm = Get-Command pnpm.cmd -ErrorAction SilentlyContinue
if (-not $pnpm) { $pnpm = Get-Command pnpm -ErrorAction SilentlyContinue }

Write-Host "工具链:" -ForegroundColor Cyan
Write-Host ("  go     : " + $(if ($go) { $go.Source } else { "未找到" }))
Write-Host ("  wails3 : " + $(if ($wails) { $wails.Source } else { "未找到" }))
Write-Host ("  pnpm   : " + $(if ($pnpm) { $pnpm.Source } else { "未找到" }))

if (-not ($go -and $wails -and $pnpm)) {
  Write-Host ""
  Write-Host "工具链不完整，请确认 $tools 下的便携工具是否还在。" -ForegroundColor Red
  exit 1
}

if ($Check) {
  Write-Host ""
  Write-Host "检查通过。" -ForegroundColor Green
  exit 0
}

$exe = Join-Path $root "bin\YukiHub.exe"

Push-Location $root
try {
  if (-not $RunOnly) {
    if ($SkipFrontend) {
      Write-Host ""
      Write-Host "[1/3] 跳过前端打包" -ForegroundColor Yellow
    }
    else {
      Write-Host ""
      Write-Host "[1/3] 打包前端（frontend/dist）..." -ForegroundColor Cyan
      & $pnpm.Source --dir frontend build
      if ($LASTEXITCODE -ne 0) { throw "前端打包失败" }
    }

    Write-Host ""
    Write-Host "[2/3] 构建桌面程序（bin\YukiHub.exe）..." -ForegroundColor Cyan
    & wails3 build
    if ($LASTEXITCODE -ne 0) { throw "wails3 build 失败" }
  }
  else {
    Write-Host ""
    Write-Host "跳过构建（-RunOnly）" -ForegroundColor Yellow
  }

  if (-not (Test-Path $exe)) {
    throw "未找到 $exe，请先完整构建一次"
  }

  $running = Get-Process -Name YukiHub -ErrorAction SilentlyContinue
  if ($running) {
    Write-Host ""
    Write-Host "检测到 YukiHub 正在运行，正在关闭旧实例..." -ForegroundColor Yellow
    $running | ForEach-Object { [void]$_.CloseMainWindow() }
    Start-Sleep -Seconds 2
    $still = Get-Process -Name YukiHub -ErrorAction SilentlyContinue
    if ($still) {
      $still | Stop-Process -Force -ErrorAction SilentlyContinue
    }
    Start-Sleep -Milliseconds 500
  }

  Write-Host ""
  Write-Host "[3/3] 启动 YukiHub" -ForegroundColor Cyan
  Start-Process -FilePath $exe -WorkingDirectory $root

  Write-Host ""
  Write-Host "已启动，可以开始预览了。" -ForegroundColor Green
}
catch {
  Write-Host ""
  Write-Host "失败：$_" -ForegroundColor Red
  exit 1
}
finally {
  Pop-Location
}
