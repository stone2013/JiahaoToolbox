[CmdletBinding()]
param([string]$Output='')
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
$config=Get-Content -Raw (Join-Path $root 'version.json') | ConvertFrom-Json
$version=[string]$config.version
if($version -notmatch '^\d+\.\d+\.\d+$'){throw 'Invalid version.json SemVer.'}
if(-not $Output){$Output=Join-Path (Split-Path $root -Parent) "JiahaoToolbox_v$version"}
$out=[IO.Path]::GetFullPath($Output)
New-Item -ItemType Directory -Force $out | Out-Null
$stage=Join-Path $out '_build'
$resolvedStage=[IO.Path]::GetFullPath($stage)
if(-not $resolvedStage.StartsWith($out+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)){throw 'Unsafe build staging path.'}
New-Item -ItemType Directory -Force $stage | Out-Null
try {
  & (Join-Path $PSScriptRoot 'build.ps1') -RebuildFrontend -Output $stage
  if($LASTEXITCODE -ne 0){throw 'Application build failed.'}
  $exe=Join-Path $stage "JiahaoToolbox_v$version.exe"
  if(-not (Test-Path -LiteralPath $exe)){throw 'Built EXE missing.'}
  $iscc=(Get-Command iscc.exe -ErrorAction SilentlyContinue).Source
  if(-not $iscc){$candidates=@((Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 6\ISCC.exe'),(Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'),(Join-Path $env:ProgramFiles 'Inno Setup 6\ISCC.exe'));$iscc=$candidates|Where-Object{Test-Path $_}|Select-Object -First 1}
  if(-not $iscc){throw 'Inno Setup 6 ISCC.exe not found; install JRSoftware Inno Setup 6.'}
  $env:JHTB_VERSION=$version;$env:JHTB_EXE=$exe;$env:JHTB_OUTPUT=$stage
  & $iscc (Join-Path $root 'installer\JiahaoToolbox.iss')
  if($LASTEXITCODE -ne 0){throw 'Inno Setup compile failed.'}
  $portable=Join-Path $stage 'portable';New-Item -ItemType Directory -Force $portable|Out-Null
  Copy-Item -LiteralPath $exe -Destination (Join-Path $portable "JiahaoToolbox_v$version.exe")
  foreach($name in @('README.md','LICENSE','LICENSES.md')){Copy-Item -LiteralPath (Join-Path $root $name) -Destination $portable}
  Set-Content -LiteralPath (Join-Path $portable 'UNSIGNED.txt') -Encoding utf8 -Value 'Unsigned build. Verify files and source before running. No Authenticode certificate is configured.'
  $portableZip=Join-Path $stage "JiahaoToolbox_v${version}_Portable.zip"
  Compress-Archive -Path (Join-Path $portable '*') -DestinationPath $portableZip -CompressionLevel Optimal
  Copy-Item -LiteralPath $exe -Destination (Join-Path $out "JiahaoToolbox_v$version.exe") -Force
  Copy-Item -LiteralPath (Join-Path $stage "JiahaoToolbox_v${version}_Setup_x64.exe") -Destination $out -Force
  Copy-Item -LiteralPath $portableZip -Destination $out -Force
  foreach($name in @('README.md','CHANGELOG.md','RELEASE_GUIDE.md','TEST_REPORT.md','LICENSE','LICENSES.md')){if(Test-Path (Join-Path $root $name)){Copy-Item -LiteralPath (Join-Path $root $name) -Destination $out -Force}}
  $sourceDest=Join-Path $out 'source'
  robocopy $root $sourceDest /E /XD (Join-Path $root 'frontend\node_modules') (Join-Path $root 'test-output') (Join-Path $root 'tests\test-output') (Join-Path $root 'build') /XF '*.log' | Out-Null
  if($LASTEXITCODE -ge 8){throw 'Copying the complete source tree failed.'}
  $global:LASTEXITCODE=0
  $workflowDest=Join-Path $out '.github\workflows';New-Item -ItemType Directory -Force $workflowDest|Out-Null
  $workflow=Join-Path (Split-Path $root -Parent) '.github\workflows\release.yml'
  if(Test-Path -LiteralPath $workflow){Copy-Item -LiteralPath $workflow -Destination (Join-Path $workflowDest 'release.yml') -Force}
  Copy-Item -LiteralPath (Join-Path $root 'installer\JiahaoToolbox.iss') -Destination (Join-Path (New-Item -ItemType Directory -Force (Join-Path $out 'installer')).FullName 'JiahaoToolbox.iss') -Force
  Set-Content -LiteralPath (Join-Path $out 'SIGNING_STATUS.txt') -Encoding utf8 -Value 'UNSIGNED: no Authenticode code-signing certificate or trusted update-manifest key is configured. SHA-256 detects file corruption only; it does not independently authenticate the publisher.'
  $sumNames=@("JiahaoToolbox_v${version}.exe","JiahaoToolbox_v${version}_Setup_x64.exe","JiahaoToolbox_v${version}_Portable.zip")
  $lines=foreach($name in $sumNames){$hash=(Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $out $name)).Hash.ToLowerInvariant();"$hash  $name"}
  $lines|Set-Content -Encoding ascii (Join-Path $out 'SHA256SUMS.txt')
} finally {
  if(Test-Path -LiteralPath $stage){
    $check=[IO.Path]::GetFullPath($stage)
    if($check.StartsWith($out+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)){Remove-Item -LiteralPath $check -Recurse -Force}
  }
  Remove-Item Env:JHTB_VERSION -ErrorAction SilentlyContinue
  Remove-Item Env:JHTB_EXE -ErrorAction SilentlyContinue
  Remove-Item Env:JHTB_OUTPUT -ErrorAction SilentlyContinue
}
