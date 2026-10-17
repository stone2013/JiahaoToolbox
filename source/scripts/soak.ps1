[CmdletBinding()]
param([string]$Exe='', [ValidateRange(10,86400)][int]$Seconds=3600, [string]$Output='')
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
if(-not $Exe){$Exe=Join-Path $root 'build/JiahaoToolbox_v4.0.exe'}
$Exe=(Resolve-Path -LiteralPath $Exe).Path
if(-not $Output){$Output=Join-Path $root ('test-output/soak-'+(Get-Date -Format 'yyyyMMdd-HHmmss'))}
$Output=[IO.Path]::GetFullPath($Output)
New-Item -ItemType Directory -Force $Output | Out-Null
$profile=Join-Path $Output 'isolated-profile'
$report=Join-Path $Output 'backend-self-check.json'
if(Test-Path -LiteralPath $report){throw 'Output already has a report; choose a new directory. No test results will be overwritten.'}
$args=@('--self-check',($Seconds.ToString()+'s'),'--data-dir',('"'+$profile+'"'),'--report-file',('"'+$report+'"'))
$p=Start-Process -FilePath $Exe -ArgumentList $args -PassThru -RedirectStandardOutput (Join-Path $Output 'stdout.txt') -RedirectStandardError (Join-Path $Output 'stderr.txt')
$samples=[Collections.Generic.List[object]]::new();$clock=[Diagnostics.Stopwatch]::StartNew()
while(-not $p.HasExited){
 try {$p.Refresh();$samples.Add([pscustomobject]@{Seconds=$clock.Elapsed.TotalSeconds;WorkingSetBytes=$p.WorkingSet64;PrivateBytes=$p.PrivateMemorySize64;CPUSeconds=$p.TotalProcessorTime.TotalSeconds;Handles=$p.HandleCount})} catch {Write-Warning $_}
 Start-Sleep -Seconds 5
}
$p.WaitForExit();$p.Refresh();$samples | Export-Csv -NoTypeInformation -Encoding UTF8 (Join-Path $Output 'backend-process.csv')
@{ExitCode=$p.ExitCode;RequestedSeconds=$Seconds;ActualWallSeconds=$clock.Elapsed.TotalSeconds;ExecutableSHA256=(Get-FileHash -Algorithm SHA256 $Exe).Hash;Scope='Backend only; does not test WebView2, page switching, GPU driver correctness or child-process footprint'} | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $Output 'run-metadata.json')
if($p.ExitCode -ne 0 -or -not (Test-Path $report)){throw 'Self-check did not complete. Keep the output folder and inspect errors; do not mark PASS.'}
Write-Host "Self-check finished: $report. Read missing metrics and errors before judging results. Full GUI soak is separately required."
