#!/usr/bin/env python3
"""Timed CI phases. Compiler output stays in artifact logs, not the console."""
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess as sp
import sys
import time
import zipfile

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'ci-output'
OUT.mkdir(exist_ok=True)

def stamp():
    return dt.datetime.now(dt.timezone(dt.timedelta(hours=3))).strftime('[%d.%m.%Y, %H:%M:%S UTC+3]')

def command(args, cwd=ROOT):
    sp.run(args, cwd=cwd, check=True)

def gradle(*args):
    command(['java', '-cp', 'gradle/wrapper/gradle-wrapper.jar', 'org.gradle.wrapper.GradleWrapperMain',
             *args, '--init-script', 'telegramita/gradle/telegramita.init.gradle', '--console=plain',
             '--max-workers=2', '-Dorg.gradle.jvmargs=-Xmx6g -XX:MaxMetaspaceSize=1g -Dfile.encoding=UTF-8',
             '--build-cache'])

def dependencies():
    config = ROOT.parent / '.build-config'
    config.mkdir(exist_ok=True)
    # Build-time compatibility fixture derived solely from already-public upstream data.
    google = json.loads((ROOT / 'TMessagesProj_App/google-services.json').read_text())
    for client in google['client']:
        client['client_info']['android_client_info']['package_name'] = 'org.telegramita.messenger'
    (config / 'google-services-telegramita.json').write_text(json.dumps(google))
    (ROOT / 'local.properties').write_text('sdk.dir=' + os.environ['ANDROID_HOME'] + '\n')
    sdkmanager = str(Path(os.environ['ANDROID_HOME']) / 'cmdline-tools/latest/bin/sdkmanager')
    command([sdkmanager, 'platforms;android-36', 'build-tools;36.0.0', 'ndk;27.2.12479018', 'cmake;3.22.1'])
    command(['go', 'mod', 'download'], ROOT / 'telegramita/native/mieru-android')
    command(['go', 'mod', 'verify'], ROOT / 'telegramita/native/mieru-android')
    gradle('--version')

def mieru():
    src = ROOT / 'telegramita/native/mieru-android'
    env = os.environ
    ndk = Path(env['ANDROID_HOME']) / 'ndk/27.2.12479018/toolchains/llvm/prebuilt/linux-x86_64/bin'
    env.update(CGO_ENABLED='1', GOOS='android', GOARCH='arm64', CC=str(ndk / 'aarch64-linux-android21-clang'),
               CGO_LDFLAGS='-Wl,-soname,libmieru.so -Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384')
    output = ROOT / 'TMessagesProj/jni/prebuild/arm64-v8a/libmieru.so'
    output.parent.mkdir(parents=True, exist_ok=True)
    command(['go', 'build', '-mod=readonly', '-trimpath', '-buildmode=c-shared', '-ldflags=-s -w', '-o', str(output), '.'], src)

def tests():
    env = os.environ.copy()
    env.update(CGO_ENABLED='0', GOOS='linux', GOARCH='amd64')
    src = ROOT / 'telegramita/native/mieru-android'
    sp.run(['go', 'test', '-v', 'mierucli.go', 'pair_unix.go', 'mierucli_test.go'], cwd=src, env=env, check=True)
    classes = ROOT / 'TMessagesProj/build/intermediates/javac/release/compileReleaseJavaWithJavac/classes'
    target = OUT / 'host-tests'
    target.mkdir(exist_ok=True)
    cp = str(classes) + ':' + os.environ['ANDROID_HOME'] + '/platforms/android-36/android.jar'
    command(['javac', '-encoding', 'UTF-8', '-cp', cp, '-d', str(target),
             'telegramita/tests/android/text/TextUtils.java', 'telegramita/tests/ProxyMigrationTest.java'])
    command(['java', '-cp', str(target) + ':' + cp, 'ProxyMigrationTest'])

def verify():
    sdk = Path(os.environ['ANDROID_HOME'])
    tools = sdk / 'build-tools/36.0.0'
    apk = ROOT / 'TMessagesProj_App/build/outputs/apk/afat/release/app-unsigned.apk'
    if not apk.exists():
        apk = ROOT / 'TMessagesProj_App/build/outputs/apk/afat/release/app.apk'
    data = sp.check_output([str(tools / 'aapt2'), 'dump', 'badging', str(apk)], text=True)
    for value in ["name='org.telegramita.messenger'", "versionName='13.0.2'", "versionCode='71759'", "application-label:'Telegramita'", "native-code: 'arm64-v8a'"]:
        assert value in data, value
    assert 'application-debuggable' not in data
    with zipfile.ZipFile(apk) as z:
        assert not any(n.startswith('META-INF/') and n.endswith(('.RSA', '.DSA', '.EC')) for n in z.namelist())
        assert {n.split('/')[1] for n in z.namelist() if n.startswith('lib/')} == {'arm64-v8a'}
        for name in ['libmieru.so', 'libtmessages.49.so']:
            (OUT / name).write_bytes(z.read('lib/arm64-v8a/' + name))
    meta = sp.check_output(['go', 'version', '-m', str(OUT / 'libmieru.so')], text=True)
    assert re.search(r'github.com/enfein/mieru/v3\s+v3\.38\.1\s+h1:UZ5zAJi0caSmaOniVy33pyY7ANxyf4PmPeLSuYZpDro=', meta)
    elf = sdk / 'ndk/27.2.12479018/toolchains/llvm/prebuilt/linux-x86_64/bin/llvm-readelf'
    symbols = sp.check_output([str(elf), '--dyn-syms', '--wide', str(OUT / 'libmieru.so')], text=True)
    for name in re.findall(r'^//export (\w+)', (ROOT / 'telegramita/native/mieru-android/main.go').read_text(), re.M):
        assert name in symbols, name
    symbols = sp.check_output([str(elf), '--dyn-syms', '--wide', str(OUT / 'libtmessages.49.so')], text=True)
    for name in re.findall(r'Java_org_telegram_messenger_MieruClient_\w+', (ROOT / 'TMessagesProj/jni/mieru/jni_mieru.c').read_text()):
        assert name in symbols, name
    assert 'libmieru.so' in sp.check_output([str(elf), '-d', str(OUT / 'libtmessages.49.so')], text=True)
    command([str(tools / 'zipalign'), '-c', '-P', '16', '4', str(apk)])
    filename = 'Telegramita-13.0.2-arm64-v8a-release-unsigned.apk'
    shutil.copy2(apk, OUT / filename)
    sha = hashlib.sha256(apk.read_bytes()).hexdigest()
    (OUT / 'SHA256SUMS').write_text(sha + '  ' + filename + '\n')
    report = dict(commit=os.environ.get('GITHUB_SHA'), telegram='13.0.2', versionCode=71759, mieru='3.38.1',
                  package='org.telegramita.messenger', signed=False, bytes=apk.stat().st_size, sha256=sha,
                  jni='PASS', abi='arm64-v8a', cache={k:v for k,v in os.environ.items() if k.startswith('CACHE_')})
    (OUT / 'build-report.json').write_text(json.dumps(report, indent=2))
    print(json.dumps(report))

PHASES = dict(dependencies=dependencies, mieru=mieru, java=lambda: gradle(':TMessagesProj:compileReleaseJavaWithJavac'),
              native=lambda: gradle(':TMessagesProj:externalNativeBuildRelease'),
              apk=lambda: gradle(':TMessagesProj_App:assembleAfatRelease'), tests=tests, verify=verify)
if __name__ == '__main__':
    phase = sys.argv[1]
    begin, tick, result = stamp(), time.monotonic(), 'FAIL'
    log = OUT / (phase + '.log')
    try:
        with log.open('w') as stream:
            saved_out, saved_err = os.dup(1), os.dup(2)
            try:
                sys.stdout.flush(); os.dup2(stream.fileno(), 1); os.dup2(stream.fileno(), 2)
                PHASES[phase]()
                result = 'PASS'
            finally:
                sys.stdout.flush(); os.dup2(saved_out, 1); os.dup2(saved_err, 2)
                os.close(saved_out); os.close(saved_err)
    finally:
        record = dict(phase=phase, start=begin, end=stamp(), seconds=round(time.monotonic()-tick, 3),
                      result=result, cache=os.environ.get('PHASE_CACHE', 'N/A'), retry=os.environ.get('RETRY_REASON', ''))
        with (OUT / 'timing.jsonl').open('a') as f: f.write(json.dumps(record)+'\n')
        print(json.dumps(record))
        if result != 'PASS': print('\n'.join(log.read_text(errors='replace').splitlines()[-55:]))
