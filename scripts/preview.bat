@echo off
chcp 65001 >nul
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0preview.ps1" %*
if errorlevel 1 (
  echo.
  echo Preview failed. Press any key to close...
  pause >nul
)
endlocal
