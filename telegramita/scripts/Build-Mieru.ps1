[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$telegramitaRoot = Split-Path -Parent $PSScriptRoot
$checkoutRoot = Split-Path -Parent $telegramitaRoot
$workspaceRoot = Split-Path -Parent $checkoutRoot
$sourceRoot = Join-Path $telegramitaRoot 'native\mieru-android'
$ndk = Join-Path $workspaceRoot '.toolchains\android-sdk\ndk\27.2.12479018'
$output = Join-Path $checkoutRoot 'TMessagesProj\jni\prebuild\arm64-v8a\libmieru.so'
$env:GOCACHE = Join-Path $workspaceRoot '.gocache'
$env:CGO_ENABLED = '1'
$env:GOOS = 'android'
$env:GOARCH = 'arm64'
$env:CC = Join-Path $ndk 'toolchains\llvm\prebuilt\windows-x86_64\bin\aarch64-linux-android21-clang.cmd'
$env:CGO_LDFLAGS = '-Wl,-soname,libmieru.so -Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384'
if (!(Test-Path -LiteralPath $env:CC)) { throw 'Android ARM64 compiler is missing' }
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $output) | Out-Null
Push-Location $sourceRoot
try {
    & go build -mod=readonly -trimpath -buildmode=c-shared -ldflags '-s -w' -o $output .
    if ($LASTEXITCODE -ne 0) { throw 'Mieru ARM64 compilation failed' }
    $metadata = (& go version -m $output) -join "`n"
    if ($metadata -notmatch 'github.com/enfein/mieru/v3\s+v3\.38\.1') { throw 'Wrong Mieru version in compiled library' }
    if ($metadata -notmatch 'GOARCH=arm64' -or $metadata -notmatch 'GOOS=android') { throw 'Wrong Mieru target' }
    Write-Output $metadata
    Get-FileHash -Algorithm SHA256 -LiteralPath $output
} finally {
    Pop-Location
}
