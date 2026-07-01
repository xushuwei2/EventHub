# 功能说明：初始化 EventHub 开发环境

param()

$ErrorActionPreference = "Stop"
trap {
    Write-Host "初始化失败: $_" -ForegroundColor Red
    Write-Host "$($_.InvocationInfo.ScriptName):$($_.InvocationInfo.ScriptLineNumber)" -ForegroundColor Red
    exit 1
}

function Get-ProjectRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

function Invoke-InitDev {
    $root = Get-ProjectRoot
    $src = Join-Path $root "src"
    $runConfig = Join-Path $root ".run\config"
    $envExample = Join-Path $runConfig ".env.example"
    $envFile = Join-Path $runConfig ".env"

    Write-Host "[init] 检查 Go 环境..."
    $goVersion = go version 2>$null
    if (-not $goVersion) {
        throw "未找到 Go，请先安装 Go 1.23+"
    }
    Write-Host "[init] $goVersion"

    foreach ($dir in @(".run", ".run\log", ".run\config", ".temp", ".dist")) {
        $path = Join-Path $root $dir
        if (-not (Test-Path $path)) {
            New-Item -ItemType Directory -Path $path -Force | Out-Null
            Write-Host "[init] 创建目录: $path"
        }
    }

    Write-Host "[init] 下载 Go 依赖..."
    Push-Location $src
    try {
        go mod download
        if ($LASTEXITCODE -ne 0) { throw "go mod download 失败" }
    } finally {
        Pop-Location
    }

    if (-not (Test-Path $envFile)) {
        Write-Host "[init] 生成开发配置 .run/config/.env ..."
        Copy-Item $envExample $envFile

        Push-Location $src
        try {
            $hash = go run ./cmd/hashpassword admin123 2>$null
            if ($LASTEXITCODE -ne 0 -or -not $hash) {
                throw "生成密码哈希失败"
            }
        } finally {
            Pop-Location
        }

        $content = Get-Content $envFile -Raw
        $content = $content -replace 'ADMIN_PASSWORD_HASH=.*', "ADMIN_PASSWORD_HASH=$hash"
        if ($content -notmatch 'ADMIN_SESSION_SECRET=.{16,}') {
            $secret = -join ((48..57) + (65..90) + (97..122) | Get-Random -Count 48 | ForEach-Object { [char]$_ })
            $content = $content -replace 'ADMIN_SESSION_SECRET=.*', "ADMIN_SESSION_SECRET=$secret"
        }
        Set-Content -Path $envFile -Value $content.TrimEnd() -NoNewline
        Add-Content -Path $envFile -Value ""
        Write-Host "[init] 已创建 .env（默认密码 admin123）" -ForegroundColor Yellow
    } else {
        Write-Host "[init] .run/config/.env 已存在，跳过"
    }

    Write-Host "[init] 开发环境就绪" -ForegroundColor Green
}

Invoke-InitDev
