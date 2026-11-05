$ErrorActionPreference = 'Stop'
$telegramitaRoot = Split-Path -Parent $PSScriptRoot
$checkoutRoot = Split-Path -Parent $telegramitaRoot
$workspaceRoot = Split-Path -Parent $checkoutRoot
$jdk = Join-Path $workspaceRoot '.toolchains\jdk17\jdk-17.0.20.1+1\bin'
$androidJar = Join-Path $workspaceRoot '.toolchains\android-sdk\platforms\android-36\android.jar'
$classes = Join-Path $checkoutRoot 'TMessagesProj\build\intermediates\javac\release\compileReleaseJavaWithJavac\classes'
$output = Join-Path $workspaceRoot 'operations-13.0.2\proxy-test-classes'
if (!(Test-Path -LiteralPath $classes)) { throw 'Compile release Java before testing migration' }
New-Item -ItemType Directory -Force -Path $output | Out-Null
& "$jdk\javac.exe" -encoding UTF-8 -cp "$classes;$androidJar" -d $output `
    "$telegramitaRoot\tests\android\text\TextUtils.java" "$telegramitaRoot\tests\ProxyMigrationTest.java"
if ($LASTEXITCODE -ne 0) { throw 'Proxy migration test compilation failed' }
& "$jdk\java.exe" -cp "$output;$classes;$androidJar" ProxyMigrationTest
if ($LASTEXITCODE -ne 0) { throw 'Proxy migration test failed' }
