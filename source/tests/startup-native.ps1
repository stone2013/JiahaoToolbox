[CmdletBinding()]
param([string]$Exe='',[int]$Count=10)
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
if(-not $Exe){$Exe=Join-Path $root '../release-v4.2/JiahaoToolbox_v4.2.exe'}
$Exe=(Resolve-Path $Exe).Path
$out=Join-Path $root 'test-output/animated'
New-Item -ItemType Directory -Force $out | Out-Null
$results=@()
function Launch-Measure([string]$Target,[string]$Profile,[string]$Label,[bool]$EarlyClose=$false){
 New-Item -ItemType Directory -Force $Profile | Out-Null
 $watch=[Diagnostics.Stopwatch]::StartNew()
 $p=Start-Process -FilePath $Target -ArgumentList @('--data-dir',('"'+$Profile+'"')) -WindowStyle Hidden -PassThru
 $ready=$false;$readyMS=$null;$uiMS=$null;$handle=0;$log=Join-Path $Profile 'logs/app.log'
 try{
  while($watch.ElapsedMilliseconds -lt 18000){
   $p.Refresh();if($p.HasExited){break}
   $handle=$p.MainWindowHandle
   if($EarlyClose -and $handle -ne 0){break}
   if(Test-Path $log){$text=Get-Content -Raw $log;if($text -match 'UI ready elapsed_ms=(\d+)'){$uiMS=[int]$Matches[1];if($Label -like 'baseline*' -or $text -match 'native startup: navigation and frontend handshake ready'){$ready=$true;$readyMS=$watch.ElapsedMilliseconds;break}}}
   Start-Sleep -Milliseconds 50
  }
  $privateBytes=$p.PrivateMemorySize64;$workingSetBytes=$p.WorkingSet64
  $responding=if(-not $p.HasExited){$p.Responding}else{$false}
  if(-not $p.HasExited){[void]$p.CloseMainWindow();if(-not $p.WaitForExit(8000)){throw 'graceful close timeout'}}
  if(-not $EarlyClose -and -not $ready){throw 'startup handshake did not complete'}
  $after=if(Test-Path $log){Get-Content -Raw $log}else{''}
  return [ordered]@{label=$Label;ready=$ready;processToObservedReadyMS=$readyMS;backendToUIReadyMS=$uiMS;responding=$responding;hadWindow=($handle -ne 0);earlyClose=$EarlyClose;exitCode=$p.ExitCode;workingSetBytes=$workingSetBytes;privateBytes=$privateBytes;joinedWorkers=$after.Contains('application stopped; sampler and inventory joined')}
 }finally{if(-not $p.HasExited){Stop-Process -Id $p.Id}}
}
$settings=@{theme='dark';intervalMS=1000;animations=$true;units='iec';disk='';interface='';width=1536;height=1000;maximized=$false}
$profile=Join-Path $out ('native-sequence-'+(Get-Date -Format 'yyyyMMddHHmmss'))
for($i=0;$i -lt $Count;$i++){
 New-Item -ItemType Directory -Force $profile | Out-Null
 if(Test-Path (Join-Path $profile 'logs/app.log')){Move-Item -LiteralPath (Join-Path $profile 'logs/app.log') -Destination (Join-Path $profile ('logs/sequence-'+$i+'.log'))}
 $settings.theme=if($i -eq 1){'light'}else{'dark'}
 $settings | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $profile 'settings.json')
 $results+=Launch-Measure $Exe $profile $(if($i -eq 0){'fresh-profile-dark'}elseif($i -eq 1){'warm-light'}else{'warm-dark-'+$i})
}
$results+=Launch-Measure $Exe (Join-Path $out ('early-close-'+(Get-Date -Format 'yyyyMMddHHmmss'))) 'close-during-WebView2-init' $true
$baseline=Join-Path $root '../JiahaoToolbox_v4.1.exe'
for($i=0;$i -lt 3;$i++){$results+=Launch-Measure $baseline (Join-Path $out ('baseline-'+(Get-Date -Format 'yyyyMMddHHmmss')+'-'+$i)) ('baseline-'+$i)}
$results | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $out 'native-startup-evidence.json')
($results | ForEach-Object {[pscustomobject]$_}) | Select-Object label,ready,backendToUIReadyMS,exitCode,joinedWorkers | Format-Table
