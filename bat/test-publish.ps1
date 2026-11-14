Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# Load just the filesystem helpers; never invoke a build or touch the real output.
$sourcePath = Join-Path $PSScriptRoot 'publish.ps1'
$tokens = $null
$parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($sourcePath, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw ($parseErrors | Out-String) }
foreach ($name in @('Reset-PublishWorkspace', 'Add-ReleaseArtifact', 'Write-ReleaseChecksums', 'Remove-WindowsStaging', 'Assert-WindowsStagingPrivacy', 'Assert-ReleaseConfigPrivacy', 'Test-LocalBackupArtifact')) {
    $definition = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name }, $true)
    if (-not $definition) { throw "Missing helper: $name" }
    Invoke-Expression $definition.Extent.Text
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ('lianxiang-publish-test-' + [guid]::NewGuid().ToString('N'))
$repoRoot = $testRoot
$script:ResolvedVersion = '1.9.4'
$script:OutputDir = Join-Path $repoRoot 'publish/output'
$script:ReleaseArtifacts = New-Object System.Collections.Generic.List[string]
New-Item -ItemType Directory -Path (Join-Path $script:OutputDir 'data') -Force | Out-Null
$sentinels = @{
    'publish/output/config.yaml' = 'user configuration must survive'
    'publish/output/data/app.db' = 'user database must survive'
    'publish/staging/previous/data/app.db' = 'unrelated staging data must survive'
}
try {
    foreach ($entry in $sentinels.GetEnumerator()) {
        $file = Join-Path $repoRoot $entry.Key
        New-Item -ItemType Directory -Path (Split-Path $file) -Force | Out-Null
        [IO.File]::WriteAllText($file, $entry.Value)
    }
    Reset-PublishWorkspace
    foreach ($entry in $sentinels.GetEnumerator()) {
        if ([IO.File]::ReadAllText((Join-Path $repoRoot $entry.Key)) -ne $entry.Value) { throw 'Reset changed user data' }
    }

    $artifact = Join-Path $script:OutputDir 'LianXiangBrowser-1.9.4-windows-amd64.exe'
    [IO.File]::WriteAllText($artifact, 'fixture artifact')
    Add-ReleaseArtifact -Path $artifact
    Write-ReleaseChecksums
    $lines = @(Get-Content -LiteralPath (Join-Path $script:OutputDir 'SHA256SUMS.txt'))
    $expected = (Get-FileHash -LiteralPath $artifact -Algorithm SHA256).Hash.ToLowerInvariant() + '  ' + (Split-Path $artifact -Leaf)
    if ($lines.Count -ne 1 -or $lines[0] -ne $expected) { throw 'Checksums included local data or omitted the artifact' }

    $rejected = $false
    try { Remove-WindowsStaging -StagingDir $script:OutputDir } catch { $rejected = $true }
    if (-not $rejected) { throw 'Cleanup accepted output as staging' }

    New-Item -ItemType Directory -Path $script:WindowsStagingDir -Force | Out-Null
    [IO.File]::WriteAllText((Join-Path $script:WindowsStagingDir 'config.yaml'), "app:`n  name: fixture`n")
    Assert-WindowsStagingPrivacy -StagingDir $script:WindowsStagingDir
    [IO.File]::WriteAllText((Join-Path $script:WindowsStagingDir 'app.db'), 'must not ship')
    $rejected = $false
    try { Assert-WindowsStagingPrivacy -StagingDir $script:WindowsStagingDir } catch { $rejected = $true }
    if (-not $rejected) { throw 'Privacy check accepted a database' }
    Remove-WindowsStaging -StagingDir $script:WindowsStagingDir
    if (Test-Path -LiteralPath $script:WindowsStagingDir) { throw 'Owned staging was not cleaned' }
    foreach ($entry in $sentinels.GetEnumerator()) {
        if ([IO.File]::ReadAllText((Join-Path $repoRoot $entry.Key)) -ne $entry.Value) { throw 'Cleanup changed user data' }
    }
    Write-Host 'PASS: output preservation, checksum scope, cleanup boundary, payload privacy'
}
finally {
    $resolved = [IO.Path]::GetFullPath($testRoot)
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\'
    if (-not $resolved.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $resolved -Leaf) -notlike 'lianxiang-publish-test-*') {
        throw "Refusing to remove unexpected test directory: $resolved"
    }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
