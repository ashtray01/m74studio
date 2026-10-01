$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot
if (-not $env:CI -and -not $env:GOCACHE) { $env:GOCACHE = Join-Path $PSScriptRoot '.cache\go' }
$env:CGO_ENABLED = '1'
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Install Go 1.26 or newer.' }
if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) { throw 'Install MinGW-w64 / w64devkit and add its bin folder to PATH.' }
if (-not (Get-Command windres -ErrorAction SilentlyContinue)) { throw 'Install windres (MinGW binutils).' }
windres -i app.rc -o app_windows_amd64.syso -O coff
if ($LASTEXITCODE -ne 0) { throw 'Windows resources failed to compile.' }
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Tests failed.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
go build -trimpath -ldflags '-H windowsgui -s -w' -o M74Studio.exe .
if ($LASTEXITCODE -ne 0) { throw 'Build failed. Close M74Studio.exe before rebuilding.' }
Write-Host 'Built M74Studio.exe'
