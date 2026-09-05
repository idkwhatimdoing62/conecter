$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$goExecutable = Join-Path $projectRoot 'conecter.exe'
$goCommand = Get-Command go.exe -ErrorAction SilentlyContinue

if ($goCommand) {
    $goSources = Get-ChildItem -LiteralPath $projectRoot -Filter '*.go' -File
    $newestSource = $goSources | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if (-not (Test-Path -LiteralPath $goExecutable) -or ($newestSource -and $newestSource.LastWriteTime -gt (Get-Item -LiteralPath $goExecutable).LastWriteTime)) {
        Write-Host 'Building the Go service...' -ForegroundColor DarkGray
        & $goCommand.Source build -o $goExecutable $projectRoot
        if ($LASTEXITCODE -ne 0) {
            Write-Host 'Go build failed.' -ForegroundColor Red
            exit 1
        }
    }
}

function Test-Port([int]$port) {
    return [bool](Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue)
}

if (-not (Test-Port 5173)) {
    if (-not (Test-Path -LiteralPath $goExecutable)) {
        Write-Host 'conecter.exe not found. Run: go build -o conecter.exe .' -ForegroundColor Yellow
        exit 1
    }
    Start-Process -FilePath $goExecutable -WorkingDirectory $projectRoot -WindowStyle Hidden | Out-Null
}

if (-not (Test-Port 5174)) {
    $npmCommand = (Get-Command npm.cmd -ErrorAction Stop).Source
    Start-Process -FilePath $npmCommand -ArgumentList @('run', 'dev', '--', '--host', '0.0.0.0', '--port', '5174') -WorkingDirectory $projectRoot -WindowStyle Hidden | Out-Null
}

Start-Sleep -Milliseconds 700
$lanIp = '127.0.0.1'
try {
    $lanIp = (Invoke-RestMethod -UseBasicParsing 'http://127.0.0.1:5173/api/info').ip
} catch {
    Write-Host 'Go service started, but the LAN address could not be detected.' -ForegroundColor Yellow
}

Write-Host ''
Write-Host 'Conecter is running' -ForegroundColor Green
Write-Host "Local        http://localhost:5174/"
Write-Host "LAN          http://${lanIp}:5174/" -ForegroundColor Cyan
Write-Host 'API          http://localhost:5173/'
Write-Host ''
Write-Host 'Closing this window does not stop the services. Stop conecter.exe and vite when finished.' -ForegroundColor DarkGray
Read-Host 'Press Enter to close'
