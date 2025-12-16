@echo off
echo Building Bastion V3 for multiple platforms...
echo.

if not exist "dist" mkdir dist

REM Check if rsrc tool is available for embedding icon
where rsrc >nul 2>&1
if %errorlevel% equ 0 (
    echo Generating Windows resources with icon...
    rsrc -ico icon.ico -o rsrc.syso
    if %errorlevel% neq 0 (
        echo Warning: Failed to generate resources, continuing without icon...
    )
) else (
    echo Note: rsrc tool not found. Install with: go install github.com/akavel/rsrc@latest
    echo Building without icon...
)

echo [1/4] Building for Windows (amd64) - GUI mode (no console)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w -H windowsgui" -o dist\bastion-windows-amd64.exe
if %errorlevel% neq 0 goto :error

echo [2/4] Building for Windows (amd64) - Console mode...
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w" -o dist\bastion-windows-amd64-console.exe
if %errorlevel% neq 0 goto :error

echo [3/4] Building for Linux (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-s -w" -o dist\bastion-linux-amd64
if %errorlevel% neq 0 goto :error

echo [4/4] Building for macOS (amd64)...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags "-s -w" -o dist\bastion-darwin-amd64
if %errorlevel% neq 0 goto :error

echo.
echo ========================================
echo Build completed successfully!
echo ========================================
echo.
echo Output files:
dir /B dist\bastion-*
echo.
goto :end

:error
echo.
echo ========================================
echo Build failed!
echo ========================================
echo.
echo Press any key to close this window...
pause >nul
exit /b 1

:end
