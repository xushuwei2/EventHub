# 功能说明：构建 EventHub 前后端可执行文件，输出到 .run 目录

param(
    [switch]$Release
)

$ErrorActionPreference = "Stop"
trap {
    Write-Host "构建失败: $_" -ForegroundColor Red
    Write-Host "$($_.InvocationInfo.ScriptName):$($_.InvocationInfo.ScriptLineNumber)" -ForegroundColor Red
    exit 1
}

function Get-ProjectRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

function Invoke-Build {
    param([bool]$IsRelease)

    $root = Get-ProjectRoot
    $src = Join-Path $root "src"
    $out = Join-Path $root ".run"
    $temp = Join-Path $root ".temp"

    $gocache = Join-Path $temp "gocache"
    $gotmp = Join-Path $temp "gotmp"
    foreach ($dir in @($out, $temp, $gocache, $gotmp)) {
        if (-not (Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
        }
    }

    $env:GOCACHE = $gocache
    $env:GOTMPDIR = $gotmp

    $ldflags = ""
    if ($IsRelease) {
        $ldflags = "-s -w"
        Write-Host "[build] Release 模式" -ForegroundColor Cyan
    } else {
        Write-Host "[build] Debug 模式" -ForegroundColor Cyan
    }

    Push-Location $src
    try {
        Write-Host "[build] 编译 eventhub..."
        if ($ldflags) {
            go build -ldflags $ldflags -o (Join-Path $out "eventhub.exe") ./cmd/eventhub
        } else {
            go build -o (Join-Path $out "eventhub.exe") ./cmd/eventhub
        }
        if ($LASTEXITCODE -ne 0) { throw "eventhub 编译失败" }

        Write-Host "[build] 编译 hashpassword..."
        if ($ldflags) {
            go build -ldflags $ldflags -o (Join-Path $out "hashpassword.exe") ./cmd/hashpassword
        } else {
            go build -o (Join-Path $out "hashpassword.exe") ./cmd/hashpassword
        }
        if ($LASTEXITCODE -ne 0) { throw "hashpassword 编译失败" }

        Write-Host "[build] 完成，输出目录: $out" -ForegroundColor Green
    } finally {
        Pop-Location
    }
}

Invoke-Build -IsRelease $Release.IsPresent
