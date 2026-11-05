[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Apk,
    [string]$PreviousApk,
    [string]$OutputDirectory
)
$ErrorActionPreference = 'Stop'
$checkoutRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$workspaceRoot = Split-Path -Parent $checkoutRoot
if (!$PreviousApk) { $PreviousApk = Join-Path $workspaceRoot 'artifacts\Telegramita-12.10.1-arm64-v8a-final.apk' }
if (!$OutputDirectory) { $OutputDirectory = Join-Path $workspaceRoot 'operations-13.0.2\verification' }
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$current = & (Join-Path $PSScriptRoot 'Verify-Apk.ps1') -Apk $Apk -ExpectedDebuggable $false
$previous = & (Join-Path $PSScriptRoot 'Verify-Apk.ps1') -Apk $PreviousApk -ExpectedDebuggable $false
if ($current.CertificateSha256 -ne $previous.CertificateSha256) { throw 'Release signing certificate changed' }
$aapt = Join-Path $workspaceRoot '.toolchains\android-sdk\build-tools\36.0.0\aapt2.exe'
$badging = (& $aapt dump badging $Apk) -join "`n"
if ($LASTEXITCODE -ne 0) { throw 'Version inspection failed' }
if ($badging -notmatch "versionCode='71759'" -or $badging -notmatch "versionName='13.0.2'") { throw 'Wrong Telegram version/code' }
$manifest = (& $aapt dump xmltree $Apk --file AndroidManifest.xml) -join "`n"
if ($LASTEXITCODE -ne 0) { throw 'Manifest inspection failed' }
if ($manifest -match 'android:debuggable[^\r\n]*=true|android:testOnly[^\r\n]*=true|android.permission.BIND_VPN_SERVICE') { throw 'Unexpected debug/test/VPN manifest option' }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::OpenRead((Resolve-Path -LiteralPath $Apk).Path)
$native = Join-Path $OutputDirectory 'libmieru-from-apk.so'
try {
    $entry = $zip.GetEntry('lib/arm64-v8a/libmieru.so')
    if (!$entry) { throw 'APK does not contain libmieru.so' }
    [IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $native, $true)
} finally {
    $zip.Dispose()
}
$metadata = (& go version -m $native) -join "`n"
if ($LASTEXITCODE -ne 0 -or $metadata -notmatch 'github.com/enfein/mieru/v3\s+v3\.38\.1\s+h1:UZ5zAJi0caSmaOniVy33pyY7ANxyf4PmPeLSuYZpDro=') {
    throw 'APK contains wrong or unverifiable Mieru source version'
}
if ($metadata -notmatch 'GOARCH=arm64' -or $metadata -notmatch 'GOOS=android') { throw 'Packaged Mieru target is wrong' }
$compiled = Join-Path $checkoutRoot 'TMessagesProj\jni\prebuild\arm64-v8a\libmieru.so'
$packagedHash = (Get-FileHash -Algorithm SHA256 $native).Hash
$compiledHash = (Get-FileHash -Algorithm SHA256 $compiled).Hash
$stripped = Join-Path $checkoutRoot 'TMessagesProj_App\build\intermediates\stripped_native_libs\afatRelease\stripAfatReleaseDebugSymbols\out\lib\arm64-v8a\libmieru.so'
if (Test-Path -LiteralPath $stripped) {
    if ((Get-FileHash -Algorithm SHA256 $stripped).Hash -ne $packagedHash) { throw 'APK native library differs from this build output' }
} elseif ($compiledHash -ne $packagedHash) {
    throw 'Cannot tie packaged library to this native build'
}
[pscustomobject]@{
    Apk = (Resolve-Path -LiteralPath $Apk).Path
    Bytes = (Get-Item -LiteralPath $Apk).Length
    Sha256 = (Get-FileHash -Algorithm SHA256 $Apk).Hash
    Telegram = '13.0.2'
    VersionCode = 71759
    Package = $current.Package
    Debuggable = $false
    CertificateSha256 = $current.CertificateSha256
    PreviousCertificateMatches = $true
    Mieru = '3.38.1'
    MieruSourceSha256 = $compiledHash
    MieruPackagedSha256 = $packagedHash
    ApkSigner = $current.ApkSigner
    ZipalignP16 = $current.ZipalignP16
}
