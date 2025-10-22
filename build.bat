@echo off
chcp 65001 >nul
echo 🔨 编译域名监控系统
echo =====================

echo 正在编译...

REM 设置编译参数
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

REM 编译程序
go build -ldflags "-s -w -X main.AppVersion=1.0.0" -o domain-monitor.exe main.go

if %ERRORLEVEL% neq 0 (
    echo ❌ 编译失败
    pause
    exit /b 1
)

echo ✅ 编译成功！
echo 📦 可执行文件: domain-monitor.exe
echo.
echo 使用方法:
echo   1. 编辑 .env 文件配置系统参数
echo   2. 编辑 domains.yml 文件添加要监控的域名
echo   3. 运行 domain-monitor.exe
echo.

pause