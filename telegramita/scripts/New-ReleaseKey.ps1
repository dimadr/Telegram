[CmdletBinding()]
param(
    [string]$SecretDirectory
)

$ErrorActionPreference = 'Stop'
$telegramitaRoot = Split-Path -Parent $PSScriptRoot
$checkoutRoot = Split-Path -Parent $telegramitaRoot
$workspaceRoot = Split-Path -Parent $checkoutRoot
if ([string]::IsNullOrWhiteSpace($SecretDirectory)) {
    $SecretDirectory = Join-Path $workspaceRoot '.release-secrets'
}

$keyStorePath = Join-Path $SecretDirectory 'telegramita-release.jks'
$propertiesPath = Join-Path $SecretDirectory 'telegramita-keystore.properties'
$keytool = Join-Path $workspaceRoot '.toolchains\jdk17\jdk-17.0.20.1+1\bin\keytool.exe'

if ((Test-Path -LiteralPath $keyStorePath) -or (Test-Path -LiteralPath $propertiesPath)) {
    throw "Refusing to overwrite persistent Telegramita signing material in $SecretDirectory"
}
if (!(Test-Path -LiteralPath $keytool)) {
    throw "keytool not found: $keytool"
}

New-Item -ItemType Directory -Force -Path $SecretDirectory | Out-Null
$bytes = New-Object byte[] 24
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes)
$rng.Dispose()
$password = [Convert]::ToHexString($bytes)

& $keytool -genkeypair -v `
    -keystore $keyStorePath `
    -storepass $password `
    -alias telegramita `
    -keypass $password `
    -keyalg RSA `
    -keysize 4096 `
    -validity 10000 `
    -dname 'CN=Telegramita, OU=Android Release, O=Telegramita, C=RU' `
    -noprompt
if ($LASTEXITCODE -ne 0) {
    throw "keytool failed with exit code $LASTEXITCODE"
}

$properties = @(
    "storeFile=$($keyStorePath.Replace('\', '/'))"
    "storePassword=$password"
    'keyAlias=telegramita'
    "keyPassword=$password"
) -join "`r`n"
[System.IO.File]::WriteAllText($propertiesPath, $properties + "`r`n", [System.Text.UTF8Encoding]::new($false))

Write-Host "Created persistent signing key: $keyStorePath"
Write-Host "Created private Gradle properties: $propertiesPath"
Write-Host 'Back up both files securely. Never regenerate the key for later app updates.'
& $keytool -list -v -keystore $keyStorePath -storepass $password -alias telegramita |
    Select-String 'Alias name:|Creation date:|Valid from:|SHA1:|SHA256:'
