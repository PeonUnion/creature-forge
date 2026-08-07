# CreatureForge dev - stop backend + frontend
# Prefer .dev\runtime.json PID (exact); fallback by process/port.
# Usage: scripts\stop-dev.bat
$root = Split-Path -Parent $PSScriptRoot
$dev  = Join-Path $root '.dev'
$rt   = Join-Path $dev 'runtime.json'

if (Test-Path $rt) {
    $r = Get-Content $rt -Raw | ConvertFrom-Json
    if ($r.api_pid) {
        Stop-Process -Id $r.api_pid -Force -ErrorAction SilentlyContinue
        Write-Host "[ok] stopped backend API (pid $($r.api_pid))"
    }
    if ($r.web_pid) {
        Stop-Process -Id $r.web_pid -Force -ErrorAction SilentlyContinue
        Write-Host "[ok] stopped frontend (pid $($r.web_pid))"
    }
    Remove-Item $rt -Force -ErrorAction SilentlyContinue
}
else {
    # no runtime -> process name fallback
    Get-Process -Name python -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -like '*creatureforge*' } |
        Stop-Process -Force -ErrorAction SilentlyContinue
}

# port fallback (ensures release, incl. pnpm->node children)
foreach ($p in @(8765, 5173)) {
    Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue |
        ForEach-Object {
            Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue
            Write-Host "[i] released port $p"
        }
}

Write-Host '[ok] dev stopped'
