# 功能说明：编译 Docker 镜像、推送到 Registry，并部署到生产环境

param(
    [string]$Version = "",
    [string]$SshHost = "43.167.254.13-match3",
    [string]$SshDockerHost = "root@43.167.254.13",
    [string]$SshDockerKey = "d:\xsw\Dropbox\data\.ssh\hotgame.root.key",
    [string]$RemoteDir = "/home/match3/eventhub/eventhub",
    [string]$Registry = "jpccr.ccs.tencentyun.com/eventhub/eventhub",
    [string]$HealthUrl = "https://eventhub.bffbond.com/healthz",
    [switch]$SkipBuild,
    [switch]$SkipDeploy,
    [switch]$AlsoLatest
)

$ErrorActionPreference = "Stop"
trap {
    Write-Host "部署失败: $_" -ForegroundColor Red
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

function Test-SshAvailable {
    param([string]$HostName)

    Write-Host "[deploy] 检查 SSH 连接: $HostName"
    ssh -o BatchMode=yes -o ConnectTimeout=15 $HostName "echo ok" 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "无法 SSH 连接到 $HostName，请检查网络和 ~/.ssh/config"
    }
}

function Invoke-BuildAndPush {
    param(
        [string]$Ver,
        [string]$ImageRegistry,
        [bool]$NoBuild,
        [bool]$PushLatest,
        [string]$ImageTarPath
    )

    if ($NoBuild) {
        Write-Host "[deploy] 跳过镜像构建与推送" -ForegroundColor Yellow
        return $null
    }

    Test-DockerAvailable

    $pushArgs = @{
        Version  = $Ver
        Registry = $ImageRegistry
    }
    if ($PushLatest) { $pushArgs.AlsoLatest = $true }

    try {
        & (Join-Path $PSScriptRoot "docker-push.ps1") @pushArgs
        if ($LASTEXITCODE -eq 0) { return $null }
    } catch {
        Write-Host "[deploy] Registry 推送异常: $_" -ForegroundColor Yellow
    }

    if ($LASTEXITCODE -ne 0) {
        Write-Host "[deploy] Registry 推送失败，改为本地打包镜像上传" -ForegroundColor Yellow
    }
    $tag = "${ImageRegistry}:${Ver}"
    if (-not (Test-Path $ImageTarPath)) {
        New-Item -ItemType Directory -Path (Split-Path $ImageTarPath -Parent) -Force | Out-Null
    }
    docker save -o $ImageTarPath $tag
    if ($LASTEXITCODE -ne 0) { throw "镜像打包失败: $tag" }
    return $ImageTarPath
}

function Get-ComposeProdPath {
    return Join-Path (Get-ProjectRoot) "docker\eventhub\eventhub\docker-compose.prod.yml"
}

function Get-EnvExamplePath {
    return Join-Path (Get-ProjectRoot) "docker\eventhub\eventhub\config\.env.example"
}

function Invoke-RemoteShell {
    param(
        [string]$HostName,
        [string]$Command,
        [string]$IdentityFile = ""
    )

    if ($IdentityFile -and (Test-Path $IdentityFile)) {
        ssh -i $IdentityFile $HostName $Command
    } else {
        ssh $HostName $Command
    }
    if ($LASTEXITCODE -ne 0) {
        throw "远程命令失败: $Command"
    }
}

function Invoke-SyncDeployFiles {
    param(
        [string]$HostName,
        [string]$TargetDir
    )

    $composeFile = Get-ComposeProdPath
    if (-not (Test-Path $composeFile)) {
        throw "缺少生产 compose 文件: $composeFile"
    }

    Write-Host "[deploy] 同步部署文件到 ${HostName}:${TargetDir}"
    Invoke-RemoteShell -HostName $HostName -Command "mkdir -p $TargetDir/config $TargetDir/data/log"

    scp $composeFile "${HostName}:${TargetDir}/docker-compose.prod.yml"
    if ($LASTEXITCODE -ne 0) { throw "上传 docker-compose.prod.yml 失败" }

    $remoteEnv = "$TargetDir/config/.env"
    $envExists = ssh $HostName "test -f $remoteEnv && echo yes || echo no"
    if ($envExists.Trim() -ne "yes") {
        $envExample = Get-EnvExamplePath
        if (-not (Test-Path $envExample)) {
            throw "远程缺少 $remoteEnv，且本地无 .env.example 可上传"
        }
        Write-Host "[deploy] 远程无 config/.env，上传 .env.example（请登录服务器修改生产配置）" -ForegroundColor Yellow
        scp $envExample "${HostName}:${remoteEnv}"
        if ($LASTEXITCODE -ne 0) { throw "上传 config/.env 失败" }
    }
}

function Invoke-RemoteDeploy {
    param(
        [string]$HostName,
        [string]$DockerHost,
        [string]$DockerKey,
        [string]$TargetDir,
        [string]$Ver,
        [string]$ImageTar
    )

    if ($ImageTar -and (Test-Path $ImageTar)) {
        $remoteTar = "/root/eventhub-$Ver.tar"
        Write-Host "[deploy] 上传镜像到生产服务器"
        if ($DockerKey -and (Test-Path $DockerKey)) {
            scp -i $DockerKey $ImageTar "${DockerHost}:${remoteTar}"
        } else {
            scp $ImageTar "${HostName}:${remoteTar}"
        }
        if ($LASTEXITCODE -ne 0) { throw "上传镜像失败" }
        $loadCmd = "docker load -i $remoteTar && rm -f $remoteTar"
        Invoke-RemoteShell -HostName $DockerHost -Command $loadCmd -IdentityFile $DockerKey
    } else {
        $pullCmd = "cd $TargetDir && export EVENTHUB_VERSION=$Ver && docker compose -f docker-compose.prod.yml pull eventhub"
        Invoke-RemoteShell -HostName $DockerHost -Command $pullCmd -IdentityFile $DockerKey
    }

    $deployCmd = @(
        "cd $TargetDir",
        "export EVENTHUB_VERSION=$Ver",
        "docker compose -f docker-compose.prod.yml up -d",
        "docker compose -f docker-compose.prod.yml ps"
    ) -join " && "

    Write-Host "[deploy] 远程启动服务 (版本: $Ver)"
    Invoke-RemoteShell -HostName $DockerHost -Command $deployCmd -IdentityFile $DockerKey
}

function Test-DeployHealth {
    param([string]$Url)

    if ([string]::IsNullOrWhiteSpace($Url)) { return }

    Write-Host "[deploy] 健康检查: $Url"
    Start-Sleep -Seconds 3
    $code = curl.exe -s -o NUL -w "%{http_code}" --max-time 20 $Url
    if ($code -eq "200") {
        Write-Host "[deploy] 健康检查通过" -ForegroundColor Green
        return
    }

    Write-Host "[deploy] 健康检查未通过 (HTTP $code)，服务可能仍在启动或 CDN 未就绪" -ForegroundColor Yellow
    Write-Host "[deploy] 可在服务器执行: ssh $SshHost 'cd $RemoteDir && docker compose -f docker-compose.prod.yml logs --tail=50 eventhub'"
}

function Invoke-DeployProd {
    param(
        [string]$Ver,
        [string]$HostName,
        [string]$DockerHost,
        [string]$DockerKey,
        [string]$TargetDir,
        [string]$ImageRegistry,
        [string]$CheckUrl,
        [bool]$NoBuild,
        [bool]$NoDeploy,
        [bool]$PushLatest
    )

    $imageTar = Join-Path (Get-ProjectRoot) ".temp\eventhub-$Ver.tar"

    Write-Host "[deploy] 版本: $Ver" -ForegroundColor Cyan
    Write-Host "[deploy] 镜像: ${ImageRegistry}:$Ver"
    Write-Host "[deploy] 文件同步: ${HostName}:${TargetDir}"
    Write-Host "[deploy] Docker 部署: $DockerHost"

    $uploadedTar = Invoke-BuildAndPush -Ver $Ver -ImageRegistry $ImageRegistry -NoBuild $NoBuild -PushLatest $PushLatest -ImageTarPath $imageTar

    if ($NoDeploy) {
        Write-Host "[deploy] 跳过远程部署" -ForegroundColor Yellow
        return
    }

    Test-SshAvailable -HostName $HostName
    Invoke-SyncDeployFiles -HostName $HostName -TargetDir $TargetDir
    Invoke-RemoteDeploy -HostName $HostName -DockerHost $DockerHost -DockerKey $DockerKey -TargetDir $TargetDir -Ver $Ver -ImageTar $uploadedTar
    Test-DeployHealth -Url $CheckUrl

    Write-Host "[deploy] 生产部署完成" -ForegroundColor Green
    Write-Host "[deploy] CDN: https://eventhub.bffbond.com"
    Write-Host "[deploy] 后台: https://eventhub.bffbond.com/reporting/admin/login"
}

if (-not $Version) {
    $Version = Get-VersionFromSource
}

Invoke-DeployProd `
    -Ver $Version `
    -HostName $SshHost `
    -DockerHost $SshDockerHost `
    -DockerKey $SshDockerKey `
    -TargetDir $RemoteDir `
    -ImageRegistry $Registry `
    -CheckUrl $HealthUrl `
    -NoBuild $SkipBuild.IsPresent `
    -NoDeploy $SkipDeploy.IsPresent `
    -PushLatest $AlsoLatest.IsPresent
