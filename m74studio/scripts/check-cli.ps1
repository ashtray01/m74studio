$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$output = Join-Path $project 'test-output\cli'
New-Item -ItemType Directory -Path $output -Force | Out-Null
$inputFile = Join-Path $output 'fixture-2026-10-01.log'
$csv = Join-Path $output 'fixture.csv'
$txt = Join-Path $output 'fixture.txt'
$session = Join-Path $output 'fixture.json'
$fixture = "ВАЗ: Ителма M74CAN`nTime: 08:21:01,261`nSend: 220001`nReceive: 62 00 01 04 77 00 4D 00 00 00 00 00 00 01 1E 00 00 10 AA 00 00 7E FF 36 FF 80 00 3F DD FF 93 A3 00 77 00 00 4B 00 00 00 00 00 7F 05 8F 00 00 00 00 00 00 4D 00 23 00 01 80 1A DB 29 EC 01`n"
[IO.File]::WriteAllText($inputFile, $fixture, [Text.UTF8Encoding]::new($false))
$exe = Join-Path $project 'M74Studio.exe'
foreach ($format in @('csv', 'txt')) {
    $destination = if ($format -eq 'csv') { $csv } else { $txt }
    $arguments = @('-input', ('"' + $inputFile + '"'), '-export', ('"' + $destination + '"'), '-format', $format, '-json', ('"' + $session + '"'))
    $process = Start-Process -FilePath $exe -ArgumentList $arguments -WindowStyle Hidden -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw "CLI $format failed: $($process.ExitCode)" }
}
$decoded = Get-Content -LiteralPath $session -Raw -Encoding utf8 | ConvertFrom-Json
if ($decoded.frames.Count -ne 1 -or $decoded.quality.complete -ne 1) { throw 'Incorrect decoded fixture' }
$rows = @(Import-Csv -LiteralPath $csv -Delimiter ';' -Encoding utf8)
if ($rows.Count -ne 1) { throw 'CSV row missing' }
if ((Get-Content -LiteralPath $txt -Raw -Encoding utf8) -notmatch 'ПАСПОРТ') { throw 'TXT report missing' }
Write-Host 'PASS: packaged executable decodes a fixture and exports CSV/TXT/JSON.'
