# 功能说明：构建并推送 EventHub Docker 镜像到 Registry

param(
    [string]$Version = "",
    [string]$Registry = "jpccr.ccs.tencentyun.com/eventhub/eventhub",
    [string]$Platform = "linux/amd64",
    [switch]$SkipBuild,
    [switch]$AlsoLatest
)

$ErrorActionPreference = "Stop"
trap {
    Write-Host "推送失败: $_" -ForegroundColor Red
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

function Test-DockerAvailable {
    $null = docker version 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "未找到 Docker 或 Docker 未运行，请先安装并启动 Docker"
    }
}

function Invoke-DockerBuild {
    param(
        [string]$Root,
        [string]$ImageRef,
        [string]$TargetPlatform
    )

    $dockerfile = Join-Path $Root "docker\eventhub\Dockerfile"
    if (-not (Test-Path $dockerfile)) {
        throw "缺少 Dockerfile: $dockerfile"
    }

    Write-Host "[docker-push] 构建镜像: $ImageRef" -ForegroundColor Cyan
    Write-Host "[docker-push] 平台: $TargetPlatform"
    docker build `
        --platform $TargetPlatform `
        -f $dockerfile `
        -t $ImageRef `
        $Root
    if ($LASTEXITCODE -ne 0) {
        throw "Docker 构建失败"
    }
}

function Invoke-DockerTag {
    param(
        [string]$SourceRef,
        [string]$TargetRef
    )

    Write-Host "[docker-push] 打标签: $TargetRef"
    docker tag $SourceRef $TargetRef
    if ($LASTEXITCODE -ne 0) {
        throw "Docker 打标签失败: $TargetRef"
    }
}

function Invoke-DockerPush {
    param([string]$ImageRef)

    Write-Host "[docker-push] 推送: $ImageRef" -ForegroundColor Cyan
    docker push $ImageRef
    if ($LASTEXITCODE -ne 0) {
        throw @"
Docker 推送失败: $ImageRef
请先登录 Registry，例如:
  docker login $($Registry.Split('/')[0])
"@
    }
}

function Invoke-DockerPushRelease {
    param(
        [string]$Ver,
        [string]$ImageRegistry,
        [string]$TargetPlatform,
        [bool]$NoBuild,
        [bool]$PushLatest
    )

    Test-DockerAvailable

    $root = Get-ProjectRoot
    $versionRef = "${ImageRegistry}:${Ver}"
    $tags = @($versionRef)

    if (-not $NoBuild) {
        Invoke-DockerBuild -Root $root -ImageRef $versionRef -TargetPlatform $TargetPlatform
    } else {
        $imageExists = docker image inspect $versionRef 2>$null
        if ($LASTEXITCODE -ne 0) {
            throw "本地不存在镜像 $versionRef，请去掉 -SkipBuild 先构建"
        }
    }

    if ($PushLatest) {
        $latestRef = "${ImageRegistry}:latest"
        Invoke-DockerTag -SourceRef $versionRef -TargetRef $latestRef
        $tags += $latestRef
    }

    foreach ($tag in $tags) {
        Invoke-DockerPush -ImageRef $tag
    }

    Write-Host "[docker-push] 完成，已推送:" -ForegroundColor Green
    foreach ($tag in $tags) {
        Write-Host "  $tag"
    }
}

if (-not $Version) {
    $Version = Get-VersionFromSource
}

Write-Host "[docker-push] 版本: $Version" -ForegroundColor Cyan
Write-Host "[docker-push] 仓库: $Registry"

Invoke-DockerPushRelease `
    -Ver $Version `
    -ImageRegistry $Registry `
    -TargetPlatform $Platform `
    -NoBuild $SkipBuild.IsPresent `
    -PushLatest $AlsoLatest.IsPresent
