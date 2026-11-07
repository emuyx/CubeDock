$ErrorActionPreference = 'Stop'
$taskUrl = 'http://127.0.0.1:18321/'
$taskRoot = Split-Path -Parent $PSScriptRoot
try {
    $null = Invoke-WebRequest -Uri ($taskUrl + 'login') -UseBasicParsing -TimeoutSec 2
    Start-Process $taskUrl
    Write-Output 'CubeDock 已运行，请登录管理页面查看房间。'
    exit
} catch { }
$taskExe = Join-Path $taskRoot 'build/CubeDock.exe'
if (-not (Test-Path -LiteralPath $taskExe)) {
    throw '未找到程序，请先运行 scripts/build.ps1 或下载 Windows 发布版。'
}
$taskData = Join-Path $taskRoot 'data/dev'
New-Item -ItemType Directory -Path $taskData -Force | Out-Null
Write-Output '首次启动会在控制台打印一次性管理员密码；请先在控制台设置上游地址并添加网易账号。'
& $taskExe -data-dir $taskData -web-port 18321
