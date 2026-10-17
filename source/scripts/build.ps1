[CmdletBinding()]
param(
    [string]$Output = '',
    [switch]$RebuildFrontend,
    [switch]$Animated,
    [switch]$AllowLegacyToolchain,
    [string]$WebView2LoaderPath = ''
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$versionConfig = Get-Content -Raw (Join-Path $root 'version.json') | ConvertFrom-Json
$appVersion = [string]$versionConfig.version
if ($appVersion -notmatch '^\d+\.\d+\.\d+$') { throw 'version.json must contain a stable MAJOR.MINOR.PATCH version.' }
if (-not $Output) { $Output = Join-Path $root 'build' }
$Output = [IO.Path]::GetFullPath($Output)
Push-Location $root
$oldGOOS=$env:GOOS; $oldGOARCH=$env:GOARCH; $oldCGO=$env:CGO_ENABLED
try {
    $version = (& go env GOVERSION).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Go is required to build, not to run the delivered EXE.' }
    if ($version -notmatch '^go(\d+)\.(\d+)') { throw "Cannot validate Go version: $version" }
    $minor = [int]$Matches[2]
    if ([int]$Matches[1] -eq 1 -and $minor -lt 26 -and -not $AllowLegacyToolchain) {
        throw 'Install a currently supported, fully patched Go release. -AllowLegacyToolchain is only for reproducing the legacy evaluation build; it is not a security approval.'
    }
    Write-Host "Toolchain: $version. Version floor is not a vulnerability scan; verify current support and latest security patches before release."
    if ($RebuildFrontend) {
        & node scripts/build-frontend.mjs
        if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
    }
    if (-not (Test-Path frontend/dist/assets/main.js)) { throw 'Embedded frontend missing. Run with -RebuildFrontend.' }
    # Resource generator must run on the build host, before setting the target OS.
    $env:GOOS = (& go env GOHOSTOS).Trim(); $env:GOARCH = (& go env GOHOSTARCH).Trim()
    & go run ./cmd/resourcegen
    if ($LASTEXITCODE -ne 0) { throw 'Resource generation failed.' }
    New-Item -ItemType Directory -Force $Output | Out-Null
    $env:GOOS='windows'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
    $stamp=(Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    $flags="-s -w -H=windowsgui -X jiahao-toolbox/internal/app.Version=$appVersion -X jiahao-toolbox/internal/app.BuildTime=$stamp"
    $exeName="JiahaoToolbox_v$appVersion.exe"
    $exe=Join-Path $Output $exeName
    & go build -trimpath -buildvcs=false -ldflags $flags -o $exe ./cmd/jiahao-toolbox
    if ($LASTEXITCODE -ne 0) { throw 'Windows x64 build failed.' }
    if ($WebView2LoaderPath) {
        $loader=(Resolve-Path $WebView2LoaderPath).Path
        if ([IO.Path]::GetFileName($loader) -ne 'WebView2Loader.dll') { throw 'Use the x64 WebView2Loader.dll from the official Microsoft WebView2 SDK.' }
        Write-Warning 'The script does not prove SDK origin, license approval, or ABI correctness. Verify the Microsoft SDK source and x64 architecture before redistribution.'
        Copy-Item -LiteralPath $loader -Destination (Join-Path $Output 'WebView2Loader.dll')
    }
    $docs=@('README.md','BUILD.md','CHANGELOG.md','TEST_REPORT.md','LICENSES.md','RELEASE_BLOCKERS.md','LICENSE','启动兼容模式.cmd')
    foreach($doc in $docs) { if(Test-Path -LiteralPath $doc){Copy-Item -LiteralPath $doc -Destination $Output -Force} }
    $hash=(Get-FileHash -Algorithm SHA256 -LiteralPath $exe).Hash.ToLowerInvariant()
    "$hash  $exeName" | Set-Content -Encoding ascii (Join-Path $Output 'SHA256SUMS.txt')
    Write-Host "Built $exe. Build success is NOT Windows runtime acceptance. Check TEST_REPORT executable hash before applying any previous validation results to this new build."
} finally {
    $env:GOOS=$oldGOOS; $env:GOARCH=$oldGOARCH; $env:CGO_ENABLED=$oldCGO
    Pop-Location
}
