param(
    [string]$InstallRoot = '',
    [string]$Launcher = ''
)

$ErrorActionPreference = 'Stop'
$scriptRoot = (Resolve-Path -LiteralPath $PSScriptRoot).Path
if ([string]::IsNullOrWhiteSpace($InstallRoot)) {
    $InstallRoot = 'D:\Program Files\链享浏览器'
}
if ([string]::IsNullOrWhiteSpace($Launcher)) {
    $Launcher = [System.IO.Path]::GetFullPath((Join-Path $scriptRoot '..\..\publish\output\lianxiang-launcher.exe'))
}
$resolvedRoot = (Resolve-Path -LiteralPath $InstallRoot).Path
$browser = Join-Path $resolvedRoot 'lianxiang-browser.exe'
$core = Join-Path $resolvedRoot 'lianxiang-browser-core.exe'
$launcherPath = (Resolve-Path -LiteralPath $Launcher).Path

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw '请以管理员身份运行此脚本；脚本只替换程序文件，不操作 data 目录。'
}
if (-not (Test-Path -LiteralPath $browser)) {
    throw "找不到原程序: $browser"
}
if (Test-Path -LiteralPath $core) {
    throw "备份文件已存在，为避免覆盖请先检查: $core"
}

# The data directory is deliberately not referenced. Only the executable is
# renamed and the ignored launcher build artifact is copied into its place.
Move-Item -LiteralPath $browser -Destination $core
try {
    Copy-Item -LiteralPath $launcherPath -Destination $browser
} catch {
    Move-Item -LiteralPath $core -Destination $browser -ErrorAction SilentlyContinue
    throw
}

Get-FileHash -LiteralPath $browser, $core -Algorithm SHA256
