param(
    [string]$Go = (Join-Path $PSScriptRoot '..\..\.local-tools\go\bin\go.exe'),
    [string]$Output = (Join-Path $PSScriptRoot '..\..\publish\output\lianxiang-launcher.exe')
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$source = Join-Path $PSScriptRoot 'main.go'
$outputPath = [System.IO.Path]::GetFullPath($Output)

New-Item -ItemType Directory -Force -Path (Split-Path $outputPath) | Out-Null
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
& $Go build -trimpath -ldflags '-H=windowsgui' -o $outputPath $source
if ($LASTEXITCODE -ne 0) {
    throw "launcher build failed with exit code $LASTEXITCODE"
}

Get-Item $outputPath | Select-Object FullName, Length, LastWriteTime
