@echo off
rem ============================================================================
rem 全量测试（Windows）：Go 单测 + 前端 E2E
rem ============================================================================
setlocal
set ROOT=%~dp0..
cd /d "%ROOT%"

echo === [1/2] Go 全量测试 ===
pushd gocore
go vet ./... || exit /b 1
go test ./... || exit /b 1
popd

echo === [2/2] 前端 E2E ===
pushd creatureforge\web
call pnpm test:e2e || exit /b 1
popd

echo [ok] 全部测试通过
endlocal
