[CmdletBinding()]
param([string]$Exe='', [string]$Output='')
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
if(-not $Exe){$Exe=Join-Path $root 'build/JiahaoToolbox_v4.2.exe'}
$Exe=(Resolve-Path $Exe).Path
if(-not $Output){$Output=Join-Path $root ('test-output/acceptance-'+(Get-Date -Format 'yyyyMMdd-HHmmss'))}
New-Item -ItemType Directory -Force $Output | Out-Null
$envInfo=[ordered]@{Time=(Get-Date).ToUniversalTime().ToString('o');OS=[Environment]::OSVersion.VersionString;Is64Bit=[Environment]::Is64BitOperatingSystem;PowerShell=$PSVersionTable.PSVersion.ToString();ExecutableSHA256=(Get-FileHash -Algorithm SHA256 $Exe).Hash;Authenticode=(Get-AuthenticodeSignature $Exe).Status.ToString()}
$envInfo | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $Output 'machine.json')
Copy-Item (Join-Path $root 'docs/WINDOWS_ACCEPTANCE.md') (Join-Path $Output 'WINDOWS_ACCEPTANCE.md')
Write-Host 'Starting the actual Windows window with an isolated profile. No registry changes, network probes, tool launches or startup changes are performed automatically.'
$p=Start-Process -FilePath $Exe -ArgumentList @('--data-dir',('"'+(Join-Path $Output 'profile')+'"')) -PassThru
Write-Host "PID $($p.Id). Complete WINDOWS_ACCEPTANCE.md with observed evidence. This script does not assign PASS."
