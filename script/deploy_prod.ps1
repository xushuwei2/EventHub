# 功能说明：交叉编译 Linux 二进制并部署到生产环境（systemd 用户服务）

param(
    [string]$Version = "",
    [string]$SshHost = "match3@52.193.110.105",
    [string]$RemoteDir = "/home/match3/eventhub/eventhub",
    [string]$HealthUrl = "https://eventhub.bffbond.com/healthz",
    [switch]$SkipBuild,
    [switch]$SkipDeploy
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

function Get-LinuxBinaryPath {
    return Join-Path (Get-ProjectRoot) ".temp\linux\eventhub"
}

function Get-EnvExamplePath {
    return Join-Path (Get-ProjectRoot) "config\.env.example"
}

function Get-ServiceUnitPath {
    return Join-Path (Get-ProjectRoot) "deploy\eventhub.service"
}

function Test-SshAvailable {
    param([string]$HostName)

    Write-Host "[deploy] 检查 SSH 连接: $HostName"
    ssh -o BatchMode=yes -o ConnectTimeout=15 $HostName "echo ok" 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "无法 SSH 连接到 $HostName，请检查网络和 ~/.ssh/config"
    }
}

function Invoke-BuildLinux {
    param([bool]$NoBuild)

    if ($NoBuild) {
        $bin = Get-LinuxBinaryPath
        if (-not (Test-Path $bin)) {
            throw "跳过构建但本地不存在 Linux 二进制: $bin"
        }
        Write-Host "[deploy] 跳过构建，使用已有二进制: $bin" -ForegroundColor Yellow
        return (Resolve-Path $bin).Path
    }

    Write-Host "[deploy] 交叉编译 Linux 二进制..."
    & (Join-Path $PSScriptRoot "build.ps1") -Release -Linux
    if ($LASTEXITCODE -ne 0) { throw "Linux 二进制构建失败" }

    $bin = Get-LinuxBinaryPath
    if (-not (Test-Path $bin)) {
        throw "构建完成但未找到二进制: $bin"
    }
    return (Resolve-Path $bin).Path
}

function Invoke-RemoteShell {
    param(
        [string]$HostName,
        [string]$Command
    )

    ssh $HostName $Command
    if ($LASTEXITCODE -ne 0) {
        throw "远程命令失败: $Command"
    }
}

function Invoke-StopLegacyDocker {
    param(
        [string]$HostName,
        [string]$TargetDir
    )

    Write-Host "[deploy] 停止旧 Docker 部署（如存在）" -ForegroundColor Yellow
    $cmd = @(
        "export PATH=`"`$HOME/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`"",
        "export XDG_RUNTIME_DIR=`"`${XDG_RUNTIME_DIR:-/run/user/`$(id -u)}`"",
        "export DOCKER_HOST=`"unix:///run/user/`$(id -u)/docker.sock`"",
        "cd $TargetDir",
        "docker compose -f docker-compose.prod.yml down 2>/dev/null || true",
        "DOCKER_HOST=unix:///var/run/docker.sock docker compose -f docker-compose.prod.yml down 2>/dev/null || true",
        "rm -f docker-compose.prod.yml .env"
    ) -join "; "
    ssh $HostName $cmd
}

function Invoke-EnsureRemoteLayout {
    param([string]$HostName)

    $remoteEnv = "$RemoteDir/config/.env"
    $envExample = Get-EnvExamplePath
    if (-not (Test-Path $envExample)) {
        throw "缺少配置模板: $envExample"
    }

    Write-Host "[deploy] 初始化远程目录: ${HostName}:${RemoteDir}"
    Invoke-RemoteShell -HostName $HostName -Command "mkdir -p $RemoteDir/config $RemoteDir/log ~/.config/systemd/user"

    $envExists = ssh $HostName "test -f $remoteEnv && echo yes || echo no"
    if ($envExists.Trim() -ne "yes") {
        Write-Host "[deploy] 远程无 config/.env，上传 .env.example（请登录服务器修改生产配置）" -ForegroundColor Yellow
        scp $envExample "${HostName}:${remoteEnv}"
        if ($LASTEXITCODE -ne 0) { throw "上传 config/.env 失败" }
    }
}

function Invoke-InstallSystemdUnit {
    param([string]$HostName)

    $unitFile = Get-ServiceUnitPath
    if (-not (Test-Path $unitFile)) {
        throw "缺少 systemd 单元文件: $unitFile"
    }

    Write-Host "[deploy] 安装 systemd 用户服务"
    scp $unitFile "${HostName}:~/.config/systemd/user/eventhub.service"
    if ($LASTEXITCODE -ne 0) { throw "上传 eventhub.service 失败" }

    Invoke-RemoteShell -HostName $HostName -Command "systemctl --user daemon-reload"
}

function Invoke-EnsureLinger {
    param([string]$HostName)

    $username = ($HostName -split "@")[-1]
    if ($HostName -match "^([^@]+)@") {
        $username = $Matches[1]
    }

    Write-Host "[deploy] 确保用户服务可脱离登录会话运行"
    ssh $HostName "loginctl show-user $username -p Linger 2>/dev/null | grep -q yes || loginctl enable-linger $username 2>/dev/null || true"
}

function Invoke-RemoteDeploy {
    param(
        [string]$HostName,
        [string]$BinaryPath,
        [string]$Ver
    )

    Invoke-StopLegacyDocker -HostName $HostName -TargetDir $RemoteDir
    Invoke-EnsureRemoteLayout -HostName $HostName
    Invoke-InstallSystemdUnit -HostName $HostName
    Invoke-EnsureLinger -HostName $HostName

    Write-Host "[deploy] 停止旧服务以便替换二进制"
    Invoke-RemoteShell -HostName $HostName -Command "systemctl --user stop eventhub 2>/dev/null || true"

    Write-Host "[deploy] 上传二进制 (版本: $Ver)"
    scp $BinaryPath "${HostName}:${RemoteDir}/eventhub"
    if ($LASTEXITCODE -ne 0) { throw "上传二进制失败" }

    Invoke-RemoteShell -HostName $HostName -Command "chmod +x $RemoteDir/eventhub"
    Invoke-RemoteShell -HostName $HostName -Command "systemctl --user enable --now eventhub"
    Invoke-RemoteShell -HostName $HostName -Command "systemctl --user status eventhub --no-pager"
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
    Write-Host "[deploy] 可在服务器执行: ssh $SshHost 'journalctl --user -u eventhub --no-pager -n 50'"
}

function Invoke-DeployProd {
    param(
        [string]$Ver,
        [string]$HostName,
        [string]$TargetDir,
        [string]$CheckUrl,
        [bool]$NoBuild,
        [bool]$NoDeploy
    )

    Write-Host "[deploy] 版本: $Ver" -ForegroundColor Cyan
    Write-Host "[deploy] SSH: $HostName"
    Write-Host "[deploy] 部署目录: $TargetDir"

    $binaryPath = Invoke-BuildLinux -NoBuild $NoBuild

    if ($NoDeploy) {
        Write-Host "[deploy] 跳过远程部署" -ForegroundColor Yellow
        return
    }

    Test-SshAvailable -HostName $HostName
    Invoke-RemoteDeploy -HostName $HostName -BinaryPath $binaryPath -Ver $Ver
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
    -TargetDir $RemoteDir `
    -CheckUrl $HealthUrl `
    -NoBuild $SkipBuild.IsPresent `
    -NoDeploy $SkipDeploy.IsPresent
