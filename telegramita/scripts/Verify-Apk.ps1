[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Apk,

    [Parameter(Mandatory = $true)]
    [bool]$ExpectedDebuggable,

    [string]$ExpectedPackage = 'org.telegramita.messenger',
    [string]$ExpectedLabel = 'Telegramita'
)

$ErrorActionPreference = 'Stop'
$telegramitaRoot = Split-Path -Parent $PSScriptRoot
$checkoutRoot = Split-Path -Parent $telegramitaRoot
$workspaceRoot = Split-Path -Parent $checkoutRoot
$buildTools = Join-Path $workspaceRoot '.toolchains\android-sdk\build-tools\36.0.0'
$javaHome = Join-Path $workspaceRoot '.toolchains\jdk17\jdk-17.0.20.1+1'
$aapt2 = Join-Path $buildTools 'aapt2.exe'
$apksigner = Join-Path $buildTools 'apksigner.bat'
$zipalign = Join-Path $buildTools 'zipalign.exe'
$resolvedApk = (Resolve-Path -LiteralPath $Apk).Path
$env:JAVA_HOME = $javaHome

$badging = (& $aapt2 dump badging $resolvedApk) -join "`n"
if ($LASTEXITCODE -ne 0) { throw 'aapt2 dump badging failed' }

$packageMatch = [regex]::Match($badging, "package: name='([^']+)'")
if (!$packageMatch.Success -or $packageMatch.Groups[1].Value -ne $ExpectedPackage) {
    throw "Unexpected package: $($packageMatch.Groups[1].Value); expected $ExpectedPackage"
}
if ($badging -notmatch "application-label:'$([regex]::Escape($ExpectedLabel))'") {
    throw "Expected application label '$ExpectedLabel' was not found"
}
$isDebuggable = $badging -match '(?m)^application-debuggable$'
if ($isDebuggable -ne $ExpectedDebuggable) {
    throw "Unexpected debuggable state: $isDebuggable; expected $ExpectedDebuggable"
}
$nativeCode = [regex]::Match($badging, "native-code: ([^\r\n]+)")
if (!$nativeCode.Success -or $nativeCode.Groups[1].Value.Trim() -ne "'arm64-v8a'") {
    throw "Unexpected native ABIs: $($nativeCode.Groups[1].Value)"
}

$signerOutput = (& $apksigner verify --verbose --print-certs $resolvedApk 2>&1) -join "`n"
if ($LASTEXITCODE -ne 0) { throw "apksigner verification failed:`n$signerOutput" }
$certificateSha256 = [regex]::Match($signerOutput, 'Signer #1 certificate SHA-256 digest: ([0-9a-f]+)').Groups[1].Value
if ([string]::IsNullOrWhiteSpace($certificateSha256)) { throw 'Certificate SHA-256 was not reported by apksigner' }

$alignmentOutput = (& $zipalign -c -P 16 4 $resolvedApk 2>&1) -join "`n"
if ($LASTEXITCODE -ne 0) { throw "zipalign -P16 verification failed:`n$alignmentOutput" }

[pscustomobject]@{
    Apk = $resolvedApk
    Package = $packageMatch.Groups[1].Value
    Label = $ExpectedLabel
    Debuggable = $isDebuggable
    NativeAbis = $nativeCode.Groups[1].Value.Trim()
    CertificateSha256 = $certificateSha256.ToUpperInvariant()
    ApkSigner = 'PASS'
    ZipalignP16 = 'PASS'
}
