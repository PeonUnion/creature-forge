# CreatureForge dev - start backend + frontend (LAN-accessible), write .dev\runtime.json
# Usage: scripts\start-dev.bat   (env: API_PORT / WEB_PORT)
# Stop:  scripts\stop-dev.bat
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$apiPort = if ($env:API_PORT) { [int]$env:API_PORT } else { 8765 }
$webPort = if ($env:WEB_PORT) { [int]$env:WEB_PORT } else { 5173 }
$py  = Join-Path $root '.venv\Scripts\python.exe'
$dev = Join-Path $root '.dev'
New-Item -ItemType Directory -Force -Path $dev | Out-Null

if (-not (Test-Path $py)) {
    Write-Host '[x] .venv not found: .venv\Scripts\python.exe'
    Write-Host '    run: py -m venv .venv && .venv\Scripts\pip install -r requirements.txt'
    exit 1
}
if (-not (Test-Path (Join-Path $root 'creatureforge\web\node_modules'))) {
    Write-Host '[x] frontend deps missing: cd creatureforge\web && pnpm install'
    exit 1
}

# --- backend (python server.py --dev, LAN 0.0.0.0) --------------------------
$apiLog    = Join-Path $dev 'api.log'
$apiErrLog = Join-Path $dev 'api.err.log'
$api = Start-Process -FilePath $py `
    -ArgumentList @('creatureforge\server.py', '--dev', '--port', "$apiPort") `
    -WorkingDirectory $root `
    -RedirectStandardOutput $apiLog -RedirectStandardError $apiErrLog `
    -PassThru -WindowStyle Hidden

# --- frontend (pnpm dev --host, proxy /api /run -> apiPort) -----------------
$webLog = Join-Path $dev 'web.log'
$webCmd = "cd /d `"$(Join-Path $root 'creatureforge\web')`" && set API_TARGET=http://127.0.0.1:$apiPort && pnpm dev --host --port $webPort > `"$webLog`" 2>&1"
$web = Start-Process -FilePath 'cmd.exe' `
    -ArgumentList @('/c', $webCmd) `
    -WorkingDirectory $root -WindowStyle Minimized -PassThru

# --- LAN IP ----------------------------------------------------------------
$lan = (Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Where-Object { $_.IPAddress -notlike '127.*' -and $_.PrefixOrigin -ne 'WellKnown' } |
    Select-Object -First 1 -ExpandProperty IPAddress)
if (-not $lan) { $lan = 'localhost' }

# --- runtime info (.dev\runtime.json) ---------------------------------------
$runtime = Join-Path $dev 'runtime.json'
[ordered]@{
    api_pid    = $api.Id
    web_pid    = $web.Id
    api_port   = $apiPort
    web_port   = $webPort
    host       = '0.0.0.0'
    lan_ip     = $lan
    started_at = (Get-Date -Format o)
    logs       = @{ api = $apiLog; web = $webLog }
} | ConvertTo-Json | Set-Content -Path $runtime -Encoding UTF8

Write-Host "[i] backend API: http://127.0.0.1:$apiPort  (--dev, CORS)"
Write-Host "[i] front-end:   http://127.0.0.1:$webPort  (proxy /api /run -> $apiPort)"
Write-Host "[i] LAN:         http://$lan`:$webPort (web) / http://$lan`:$apiPort (api)"
Write-Host "[i] runtime:     $runtime"
Write-Host "[i] logs:        $apiLog / $webLog"
Write-Host "[i] stop:        scripts\stop-dev.bat"
