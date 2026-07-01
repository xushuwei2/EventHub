# 功能说明：将 Release 版本发布到 .dist 目录

param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
trap {
    Write-Host "发布失败: $_" -ForegroundColor Red
    Write-Host "$($_.InvocationInfo.ScriptName):$($_.InvocationInfo.ScriptLineNumber)" -ForegroundColor Red
    exit 1
}

function Get-ProjectRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

function Get-VersionFromSource {
    $versionFile = Join-Path (Get-ProjectRoot) "src\version.go"
    $content = Get-Content $versionFile -Raw
    if ($content -match 'Version\s*=\s*"([^"]+)"') {
        return $Matches[1]
    }
    throw "无法从 src/version.go 读取版本号"
}

function Invoke-Publish {
  param([string]$Ver)

    $root = Get-ProjectRoot
    $dist = Join-Path $root ".dist"
    $releaseDir = Join-Path $dist "eventhub-$Ver"

    Write-Host "[publish] 版本: $Ver" -ForegroundColor Cyan
    Write-Host "[publish] Release 构建..."
    & (Join-Path $PSScriptRoot "build.ps1") -Release
    if ($LASTEXITCODE -ne 0) { throw "构建失败" }

    if (Test-Path $releaseDir) {
        Remove-Item -Recurse -Force $releaseDir
    }
    New-Item -ItemType Directory -Path $releaseDir -Force | Out-Null

    $files = @("eventhub.exe", "hashpassword.exe")
    foreach ($f in $files) {
        $src = Join-Path $root ".run\$f"
        if (-not (Test-Path $src)) {
            throw "缺少构建产物: $src"
        }
        Copy-Item $src (Join-Path $releaseDir $f)
    }

    Copy-Item (Join-Path $root ".run\config\.env.example") (Join-Path $releaseDir ".env.example")
    Copy-Item (Join-Path $root "README.md") (Join-Path $releaseDir "README.md")

    Write-Host "[publish] 已发布到 $releaseDir" -ForegroundColor Green
}

if (-not $Version) {
    $Version = Get-VersionFromSource
}
Invoke-Publish -Ver $Version
