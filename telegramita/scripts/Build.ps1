[CmdletBinding()]
param(
    [ValidateSet('Diagnostic', 'Release')]
    [string]$Configuration = 'Release'
)

$ErrorActionPreference = 'Stop'
$telegramitaRoot = Split-Path -Parent $PSScriptRoot
$checkoutRoot = Split-Path -Parent $telegramitaRoot
$workspaceRoot = Split-Path -Parent $checkoutRoot
$gradleHome = Join-Path $workspaceRoot '.gradle-home'
$javaHome = Join-Path $workspaceRoot '.toolchains\jdk17\jdk-17.0.20.1+1'
$androidSdk = Join-Path $workspaceRoot '.toolchains\android-sdk'
$initScript = Join-Path $telegramitaRoot 'gradle\telegramita.init.gradle'
$gradle = Join-Path $PSScriptRoot 'Gradle.ps1'

$env:JAVA_HOME = $javaHome
$env:ANDROID_HOME = $androidSdk
$env:ANDROID_SDK_ROOT = $androidSdk
$env:GRADLE_USER_HOME = $gradleHome

$isDiagnostic = $Configuration -eq 'Diagnostic'
$task = $isDiagnostic ? ':TMessagesProj_App:assembleAfatDebug' : ':TMessagesProj_App:assembleAfatRelease'
$variantDirectory = $isDiagnostic ? 'debug' : 'release'
$suffix = $isDiagnostic ? 'diagnostic' : 'release'

Push-Location $checkoutRoot
try {
    & $gradle $task --init-script $initScript --console=plain
    if ($LASTEXITCODE -ne 0) { throw "Gradle failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
}

$sourceApk = Join-Path $checkoutRoot "TMessagesProj_App\build\outputs\apk\afat\$variantDirectory\app.apk"
if (!(Test-Path -LiteralPath $sourceApk)) { throw "Expected APK not found: $sourceApk" }
$versionLine = Select-String -LiteralPath (Join-Path $checkoutRoot 'gradle.properties') -Pattern '^APP_VERSION_NAME=(.+)$'
$version = $versionLine.Matches[0].Groups[1].Value
$artifactDirectory = Join-Path $workspaceRoot 'artifacts'
New-Item -ItemType Directory -Force -Path $artifactDirectory | Out-Null
$artifact = Join-Path $artifactDirectory "Telegramita-$version-arm64-v8a-$suffix.apk"
Copy-Item -LiteralPath $sourceApk -Destination $artifact -Force

$expectedPackage = $isDiagnostic ? 'org.telegramita.messenger.debug' : 'org.telegramita.messenger'
$expectedLabel = $isDiagnostic ? 'Telegramita Debug' : 'Telegramita'
& (Join-Path $PSScriptRoot 'Verify-Apk.ps1') `
    -Apk $artifact `
    -ExpectedDebuggable $isDiagnostic `
    -ExpectedPackage $expectedPackage `
    -ExpectedLabel $expectedLabel
if ($LASTEXITCODE -ne 0) { throw 'APK verification failed' }

Write-Host "Telegramita APK: $artifact"
