$ErrorActionPreference = "Stop"

$ScriptPath = Join-Path $PSScriptRoot "install.ps1"
$Tokens = $null
$Errors = $null
$Ast = [System.Management.Automation.Language.Parser]::ParseFile(
    $ScriptPath,
    [ref]$Tokens,
    [ref]$Errors
)
if ($Errors.Count -gt 0) {
    throw "install.ps1 has parser errors: $($Errors -join '; ')"
}

$Text = Get-Content -LiteralPath $ScriptPath -Raw
$RequiredPatterns = @(
    '#Requires -RunAsAdministrator',
    '\[string\]\$Binary',
    'Get-Service -Name \$ServiceName',
    'New-Service',
    '-BinaryPathName \(''"\{0\}" -daemon'' -f \$InstallPath\)',
    '-StartupType Automatic',
    'Stop-Service -Name \$ServiceName',
    'Start-Service -Name \$ServiceName',
    'sc\.exe config \$ServiceName'
)
foreach ($Pattern in $RequiredPatterns) {
    if ($Text -notmatch $Pattern) {
        throw "install.ps1 is missing required behavior matching: $Pattern"
    }
}

$Commands = $Ast.FindAll({
    param($Node)
    $Node -is [System.Management.Automation.Language.CommandAst]
}, $true) | ForEach-Object { $_.GetCommandName() }
foreach ($Command in @('Get-Service', 'Stop-Service', 'New-Service', 'Start-Service')) {
    if ($Commands -notcontains $Command) {
        throw "install.ps1 AST does not invoke $Command"
    }
}

Write-Host "PASS: Windows installer semantics"
