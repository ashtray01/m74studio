$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $project 'dist'
New-Item -ItemType Directory -Path $dist -Force | Out-Null
$archive = Join-Path $dist 'M74Studio-windows-x64.zip'
$items = @('M74Studio.exe', 'README.md', 'THIRD_PARTY.md', 'licenses', 'docs') | ForEach-Object { Join-Path $project $_ }
foreach ($item in $items) {
    if (-not (Test-Path -LiteralPath $item)) { throw "Missing package input: $item" }
}
Compress-Archive -LiteralPath $items -DestinationPath $archive -CompressionLevel Optimal -Force
$hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
[IO.File]::WriteAllText((Join-Path $dist 'SHA256SUMS.txt'), "$hash  M74Studio-windows-x64.zip`n", [Text.UTF8Encoding]::new($false))
Write-Host "Packaged $archive"
