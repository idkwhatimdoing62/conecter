$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$goExecutable = Join-Path $projectRoot 'conecter.exe'
$distIndex = Join-Path $projectRoot 'dist\index.html'
$packageManifest = Join-Path $projectRoot 'package.json'

function Test-Port([int]$port) {
    return [bool](Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue)
}

function Wait-Endpoint([string]$url, [int]$attempts = 20) {
    for ($i = 0; $i -lt $attempts; $i++) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec 2
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500) {
                return $true
            }
        } catch {
            Start-Sleep -Milliseconds 250
        }
    }
    return $false
}

Set-Location -LiteralPath $projectRoot

# Source checkouts can rebuild automatically; release ZIPs already contain the binary.
$goCommand = Get-Command go.exe -ErrorAction SilentlyContinue
if ($goCommand) {
    $goSources = Get-ChildItem -LiteralPath $projectRoot -Filter '*.go' -File -Recurse
    $newestSource = $goSources | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    $needsGoBuild = -not (Test-Path -LiteralPath $goExecutable)
    if (-not $needsGoBuild -and $newestSource) {
        $needsGoBuild = $newestSource.LastWriteTime -gt (Get-Item -LiteralPath $goExecutable).LastWriteTime
    }
    if ($needsGoBuild) {
        Write-Host '正在构建 Go 服务...' -ForegroundColor DarkGray
        & $goCommand.Source build -o $goExecutable ./cmd/conecter
        if ($LASTEXITCODE -ne 0) {
            Write-Host 'Go 构建失败。' -ForegroundColor Red
            exit 1
        }
    }
}

if (-not (Test-Path -LiteralPath $goExecutable)) {
    Write-Host '找不到 conecter.exe，也没有可用的 Go。' -ForegroundColor Red
    Write-Host '请从 GitHub Release 下载 Windows ZIP，或安装 Go 1.22+。'
    Read-Host '按 Enter 退出'
    exit 1
}

# A source checkout may need a frontend build. The release ZIP already has dist/.
$npmCommand = Get-Command npm.cmd -ErrorAction SilentlyContinue
if (-not (Test-Path -LiteralPath $distIndex)) {
    if (-not $npmCommand) {
        Write-Host '找不到 dist\index.html，也没有可用的 npm。' -ForegroundColor Red
        Write-Host '请安装 Node.js 20+，或从 GitHub Release 下载 Windows ZIP。'
        Read-Host '按 Enter 退出'
        exit 1
    }
    if (-not (Test-Path -LiteralPath (Join-Path $projectRoot 'node_modules'))) {
        Write-Host '正在安装前端依赖...' -ForegroundColor DarkGray
        & $npmCommand.Source ci --ignore-scripts
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    Write-Host '正在构建前端...' -ForegroundColor DarkGray
    & $npmCommand.Source run build
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if (-not (Test-Port 5173)) {
    Start-Process -FilePath $goExecutable -WorkingDirectory $projectRoot -WindowStyle Hidden | Out-Null
}

$frontendPort = 5173
$frontendUrl = 'http://localhost:5173/'
$canRunVite = $npmCommand -and (Test-Path -LiteralPath $packageManifest)
if ($canRunVite -and -not (Test-Port 5174)) {
    if (-not (Test-Path -LiteralPath (Join-Path $projectRoot 'node_modules'))) {
        Write-Host '正在安装前端依赖...' -ForegroundColor DarkGray
        & $npmCommand.Source ci --ignore-scripts
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    # The port and host are defined in package.json; do not append npm CLI flags.
    Start-Process -FilePath $npmCommand.Source -ArgumentList @('run', 'dev') -WorkingDirectory $projectRoot -WindowStyle Hidden | Out-Null
    $frontendPort = 5174
    $frontendUrl = 'http://localhost:5174/'
}

$goReady = Wait-Endpoint 'http://127.0.0.1:5173/api/info'
$webReady = Wait-Endpoint $frontendUrl
$lanIp = '127.0.0.1'
if ($goReady) {
    try { $lanIp = (Invoke-RestMethod -UseBasicParsing 'http://127.0.0.1:5173/api/info').ip } catch { }
}

Write-Host ''
if ($goReady -and $webReady) {
    Write-Host 'Conecter 已启动' -ForegroundColor Green
} else {
    Write-Host 'Conecter 启动超时，请检查端口 5173/5174 是否被占用。' -ForegroundColor Yellow
}
Write-Host "本机       $frontendUrl"
Write-Host "局域网     http://${lanIp}:${frontendPort}/" -ForegroundColor Cyan
Write-Host 'API        http://localhost:5173/'
Write-Host ''
Start-Process $frontendUrl
Write-Host '关闭此窗口不会停止服务；结束 conecter.exe 和 Vite 进程即可停止。' -ForegroundColor DarkGray
Read-Host '按 Enter 关闭此窗口'
