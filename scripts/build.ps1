$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $npm = Get-Command npm -ErrorAction SilentlyContinue
    if (-not $npm) { throw 'npm not found; install Node.js to build the console' }
    if (-not (Test-Path 'webui-src\node_modules')) {
        npm ci --prefix webui-src
        if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }
    }
    npm --prefix webui-src run build
    if ($LASTEXITCODE -ne 0) { throw 'frontend build failed' }

    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
    go test github.com/Happy2018new/nemc-tan-lobby-solver/core/sctp -run 'TestChecksumWireCompatibility|TestPacketUnmarshal|TestPacketMarshal'
    if ($LASTEXITCODE -ne 0) { throw 'SCTP wire test failed' }

    New-Item -ItemType Directory -Path build\bin -Force | Out-Null
    Remove-Item build\CubeDockMain.exe, build\limbo.exe -Force -ErrorAction SilentlyContinue
    go build -trimpath -o build\bin\CubeDockMain.exe ./cmd/cubedock
    if ($LASTEXITCODE -ne 0) { throw 'build failed' }

    Push-Location limbo
    try {
        go test ./...
        if ($LASTEXITCODE -ne 0) { throw 'limbo test failed' }
        go vet ./...
        if ($LASTEXITCODE -ne 0) { throw 'limbo vet failed' }
        go build -trimpath -o ../build/bin/limbo.exe .
        if ($LASTEXITCODE -ne 0) { throw 'limbo build failed' }
    } finally {
        Pop-Location
    }

    $payload = Join-Path $root 'cmd\cubedock-launcher\payload'
    New-Item -ItemType Directory -Path $payload -Force | Out-Null
    try {
        Copy-Item build\bin\CubeDockMain.exe (Join-Path $payload 'CubeDockMain.exe') -Force
        Copy-Item build\bin\limbo.exe (Join-Path $payload 'limbo.exe') -Force
        go build -trimpath -o build/CubeDock.exe ./cmd/cubedock-launcher
        if ($LASTEXITCODE -ne 0) { throw 'launcher build failed' }
    } finally {
        Remove-Item (Join-Path $payload 'CubeDockMain.exe'), (Join-Path $payload 'limbo.exe') -Force -ErrorAction SilentlyContinue
    }
    Write-Output 'Build complete: build/CubeDock.exe (launcher), build/bin/CubeDockMain.exe, build/bin/limbo.exe'
} finally {
    Pop-Location
}
