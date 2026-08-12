#Requires -RunAsAdministrator

[CmdletBinding()]
param(
    [string]$Binary = (Join-Path $PSScriptRoot "..\..\perflock.exe")
)

$ErrorActionPreference = "Stop"
$ServiceName = "perflock"
$InstallDirectory = Join-Path $env:ProgramFiles "perflock"
$InstallPath = Join-Path $InstallDirectory "perflock.exe"

if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
    throw "perflock.exe was not found at '$Binary'. Run 'go build -o perflock.exe ./cmd/perflock' first."
}

$Service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($null -ne $Service -and $Service.Status -ne "Stopped") {
    Stop-Service -Name $ServiceName
    $Service.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(30))
}

New-Item -ItemType Directory -Path $InstallDirectory -Force | Out-Null
Copy-Item -LiteralPath $Binary -Destination $InstallPath -Force

if ($null -eq $Service) {
    New-Service `
        -Name $ServiceName `
        -BinaryPathName ('"{0}" -daemon' -f $InstallPath) `
        -DisplayName "Perflock benchmark locking daemon" `
        -Description "Serializes benchmark processes and controls processor performance." `
        -StartupType Automatic | Out-Null
} else {
    & sc.exe config $ServiceName binPath= ('"{0}" -daemon' -f $InstallPath) start= auto | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to update the $ServiceName service."
    }
}

Start-Service -Name $ServiceName
Write-Host "Installed and started the $ServiceName service."
