$checkoutRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$workspaceRoot = Split-Path -Parent $checkoutRoot
$env:JAVA_HOME = Join-Path $workspaceRoot '.toolchains\jdk17\jdk-17.0.20.1+1'
$env:ANDROID_HOME = Join-Path $workspaceRoot '.toolchains\android-sdk'
$env:ANDROID_SDK_ROOT = $env:ANDROID_HOME
$env:GRADLE_USER_HOME = Join-Path $workspaceRoot '.gradle-home'
$java = Join-Path $env:JAVA_HOME 'bin\java.exe'
$wrapper = Join-Path $checkoutRoot 'gradle\wrapper\gradle-wrapper.jar'
& $java -classpath $wrapper org.gradle.wrapper.GradleWrapperMain @args
