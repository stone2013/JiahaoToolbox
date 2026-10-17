[CmdletBinding()]
param([switch]$Race,[switch]$Frontend,[string]$Output='')
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
if(-not $Output){$Output=Join-Path $root ('test-output/'+(Get-Date -Format 'yyyyMMdd-HHmmss'))}
 $Output=[IO.Path]::GetFullPath($Output)
New-Item -ItemType Directory -Force $Output | Out-Null
Push-Location $root
try {
  & go version | Set-Content (Join-Path $Output 'go-version.txt')
  & go vet ./... 2>&1 | Tee-Object (Join-Path $Output 'go-vet.txt')
  if($LASTEXITCODE -ne 0){throw 'go vet failed'}
  $coverageArg='-coverprofile='+(Join-Path $Output 'coverage.out')
  & go test -count=1 -v $coverageArg ./... 2>&1 | Tee-Object (Join-Path $Output 'go-tests.txt')
  if($LASTEXITCODE -ne 0){throw 'go test failed'}
  if($Race){
    Write-Host 'Race tests require the Go-supported host C toolchain. They run only on the actual host.'
    & go test -race -count=1 ./... 2>&1 | Tee-Object (Join-Path $Output 'go-race.txt')
    if($LASTEXITCODE -ne 0){throw 'race tests failed'}
  }
  if($Frontend){
    & node scripts/build-frontend.mjs
    if($LASTEXITCODE -ne 0){throw 'frontend compilation failed'}
    & node --test tests/frontend.test.mjs tests/search.test.mjs tests/search-ui.test.mjs tests/startup.test.mjs tests/monitor-animation.test.mjs 2>&1 | Tee-Object (Join-Path $Output 'frontend-tests.txt')
    if($LASTEXITCODE -ne 0){throw 'frontend tests failed'}
  }
  Write-Host 'Automated host tests complete. Complete the manual Windows acceptance sheet separately.'
} finally {Pop-Location}

