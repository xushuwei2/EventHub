# 功能说明：编译并运行 EventHub，支持单元测试

param(
    [switch]$Release,
    [switch]$Test,
    [string]$TestFilter = ""
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$SupportedParams = @("--release", "--test", "--test-filter")
$Unknown = $args | Where-Object { $_ -notin $SupportedParams -and $_ -notmatch '^--test-filter=' }
if ($Unknown.Count -gt 0) {
    Write-Host "不支持的参数: $($Unknown -join ', ')" -ForegroundColor Red
    Write-Host "支持的参数:" -ForegroundColor Yellow
    Write-Host "  --release          构建 Release 版本"
    Write-Host "  --test             运行所有单元测试"
    Write-Host "  --test-filter=NAME 运行指定测试（如 fingerprint）"
    exit 1
}

foreach ($arg in $args) {
    if ($arg -eq "--release") { $Release = $true }
    if ($arg -eq "--test") { $Test = $true }
    if ($arg -match '^--test-filter=(.+)$') { $TestFilter = $Matches[1]; $Test = $true }
}

trap {
    Write-Host "执行失败: $_" -ForegroundColor Red
    Set-Location $script:ProjectRoot
    exit 1
}

$script:ProjectRoot = (Resolve-Path $PSScriptRoot).Path

function Stop-EventHubProcesses {
    Write-Host "[run] 停止旧进程..."
    Get-Process -Name "eventhub" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
}

function Invoke-BuildStep {
    param([bool]$IsRelease)
    Write-Host "[run] 编译..."
    if ($IsRelease) {
        & (Join-Path $script:ProjectRoot "script\build.ps1") -Release
    } else {
        & (Join-Path $script:ProjectRoot "script\build.ps1")
    }
    if ($LASTEXITCODE -ne 0) { throw "编译失败" }
}

function Clear-RunLogs {
    $logDir = Join-Path $script:ProjectRoot ".run\log"
    if (Test-Path $logDir) {
        Write-Host "[run] 清除日志..."
        Get-ChildItem $logDir -File | Remove-Item -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-UnitTests {
    param([string]$Filter)

    Write-Host "[run] 运行单元测试..."
    $src = Join-Path $script:ProjectRoot "src"
    $tests = Join-Path $script:ProjectRoot "tests"

    Push-Location $src
    try {
        Write-Host "[run] 测试 src 模块..."
        go test -v ./...
        if ($LASTEXITCODE -ne 0) { throw "src 模块测试失败" }
    } finally {
        Pop-Location
    }

    Push-Location $tests
    try {
        Write-Host "[run] 测试 tests 模块..."
        if ($Filter) {
            go test -v -run $Filter .
        } else {
            go test -v .
        }
        if ($LASTEXITCODE -ne 0) { throw "tests 模块测试失败" }
    } finally {
        Pop-Location
    }
    Write-Host "[run] 单元测试通过" -ForegroundColor Green
}

function Load-EnvFile {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return }
    # 开发配置以 .run/config/.env 为准，覆盖 shell 中已有的同名变量
    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        $key = $line.Substring(0, $idx).Trim()
        $val = $line.Substring($idx + 1).Trim()
        if (-not [string]::IsNullOrEmpty($key)) {
            Set-Item -Path "env:$key" -Value $val
        }
    }
}

function Get-HttpBaseUrl {
    $addr = $env:HTTP_ADDR
    if ([string]::IsNullOrWhiteSpace($addr)) { $addr = ":8080" }
    if ($addr.StartsWith(":")) { return "http://localhost$addr" }
    if ($addr -match '^\d+$') { return "http://localhost:$addr" }
    return "http://$addr"
}

function Invoke-RunApp {
    $runDir = Join-Path $script:ProjectRoot ".run"
    $exe = Join-Path $runDir "eventhub.exe"
    $envFile = Join-Path $runDir "config\.env"

    if (-not (Test-Path $exe)) {
        throw "可执行文件不存在: $exe"
    }

    Set-Location $runDir
    Load-EnvFile $envFile

    $baseUrl = Get-HttpBaseUrl
    Write-Host "[run] 启动 EventHub（工作目录: $runDir）..."
    Write-Host "[run] 健康检查: $baseUrl/healthz"
    Write-Host "[run] 后台管理: $baseUrl/reporting/admin/login"
    Write-Host "[run] 按 Ctrl+C 停止"

    & $exe
}

function Check-RunLogs {
    $logDir = Join-Path $script:ProjectRoot ".run\log"
    if (-not (Test-Path $logDir)) { return }

    $problemFiles = @()
    Get-ChildItem $logDir -File | ForEach-Object {
        $content = Get-Content $_.FullName -Raw -ErrorAction SilentlyContinue
        if ($_.Name -match '_crash\.log$' -and $content -and $content.Trim().Length -gt 0) {
            $problemFiles += $_.FullName
        }
        if ($content -and $content -match '\[ERR\]|\[FATAL\]|error') {
            $problemFiles += $_.FullName
        }
    }

    $unique = $problemFiles | Select-Object -Unique
    foreach ($f in $unique) {
        Write-Host "[run] 发现问题日志: $f" -ForegroundColor Red
    }
}

try {
    Stop-EventHubProcesses
    Invoke-BuildStep -IsRelease $Release.IsPresent
    Clear-RunLogs

    if ($Test) {
        Invoke-UnitTests -Filter $TestFilter
    } else {
        Invoke-RunApp
    }
} finally {
    Check-RunLogs
    Set-Location $script:ProjectRoot
}
