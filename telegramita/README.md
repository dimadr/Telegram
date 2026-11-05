# Telegramita Android integration layer

Current base: `dimadr/Telegram` commit
`f32ffeec763f0bdbf4bec25b52d654f77bff733d` (13.0.2 / 7175).
Embedded Mieru: official `v3.38.1`, commit
`a2208ee7a38fb41d843a13ecdde858e67ef2f485`.
The original 12.10.1 checkout remains separate and unchanged.

This directory is the portable layer applied on top of Telegram Android upstream.
It deliberately keeps branding and build/release configuration outside upstream
source and resource trees.

## Product identity

- App name: `Telegramita`
- Release application ID: `org.telegramita.messenger`
- Diagnostic application ID: `org.telegramita.messenger.debug`
- Target ABI: `arm64-v8a`
- Release key default location: `../.release-secrets/telegramita-release.jks`
- Signing properties default location: `../.release-secrets/telegramita-keystore.properties`
- Google Services config: `../.build-config/google-services-telegramita.json`

The key and properties paths above are relative to the workspace containing the
Telegram checkout. They are intentionally outside Git. Back up both files securely;
losing the key makes it impossible to publish compatible upgrades.

The Google Services file is also external because a published fork should use its
own Firebase Android client. The current local compatibility file is sufficient for
building and transport validation, but replace it with the Telegramita Firebase
download before relying on production FCM delivery.

## Build

From the Telegram Android checkout:

```powershell
.\telegramita\scripts\Build-Mieru.ps1
.\telegramita\scripts\Build.ps1 -Configuration Release
.\telegramita\scripts\Verify-Release.ps1 -Apk ..\artifacts\Telegramita-13.0.2-arm64-v8a-release.apk
```

The PowerShell launcher invokes the upstream Gradle wrapper JAR (8.13).
The release retains the working 12.10.1 configuration with both code and
resource shrinking disabled, while remaining non-debuggable. Credentials and
the existing signing key remain external and are reused unchanged.
The Mieru module and dependencies are locked in `native/mieru-android/go.mod`
and `go.sum`; `go mod verify` validates downloaded module checksums.
The library uses the existing Client API, callback registry and socketpair bridge.

After release Java compilation, run `scripts/Test-ProxyMigration.ps1` to exercise
the actual compiled proxy model and codec on the host. The harness covers legacy
V3/V4 TCP, UDP and TCP+UDP records, range ports, active preferences, the new V5
round-trip and V2 SOCKS compatibility. Its minimal TextUtils shim is outside all
Android source sets and is not packaged into the APK.

The 13.0.2 proxy model is extended with Mieru. Persisted Mieru type remains `2`;
the upstream WEB proxy uses `3` in Telegramita. Legacy Telegramita proxy schemas
V2/V3/V4 are read and new entries are written as V5. Port ranges, credentials,
MTU and TCP/UDP/TCP+UDP are preserved in both the list and active preferences.

The release build fails closed if the external signing properties do not exist.
`Build.ps1` writes versioned APKs to the workspace `artifacts` directory and runs:

- package and app-label checks;
- debuggable-state check;
- ARM64-only native-code check;
- `apksigner verify --verbose --print-certs`;
- `zipalign -c -P 16 4`.

## Themes and runtime validation

The existing CRLF fix in `.attheme` parsing is retained because the selected
13.0.2 upstream base does not contain it. No login diagnostic logging is added.
Runtime installation, account login, reconnect and light/dark theme checks on
Honor and T-Mobile are performed by the owner. Build scripts do not use ADB.

## Portable patchset boundaries

- A. Mieru transport: native library, JNI bridge, and Mieru-owned source files.
- B. Telegram integration points: the small socket/connection callback patches.
- C. Proxy UI/config: Mieru proxy model, persistence, and selector UI patches.
- D. Branding: `telegramita/branding` only.
- E. Build/release: `telegramita/gradle/telegramita.init.gradle`.
- F. Signing/release: `telegramita/scripts`; secrets remain outside Git.

## Upstream update workflow

1. Fetch and check out the target Telegram upstream release in a clean update branch.
2. Rebuild the latest Mieru ARM64 library without changing the bridge contract.
3. Apply the Mieru transport and Telegram integration patches.
4. Copy/apply this `telegramita` directory unchanged.
5. Compile-check changed Java/resources and native tgnet/JNI, and run bridge tests.
6. Build `Release`; keep the same external release key and verify against the previous APK.
7. Run connectivity, reconnect, no-system-VPN, and Play Protect control checks.
8. Publish only the verified release APK and record its SHA-256 and certificate fingerprint.

## Launcher artwork provenance

The 1024px master was generated with the built-in image generation tool using a
`logo-brand` prompt for an original geometric T/tunnel mark, midnight navy background,
and teal-to-violet gradient. The prompt explicitly excludes Telegram's paper-plane
logo, words, watermarks, and device mockups. Density assets are deterministic resizes
of `branding/icon-master.png`.
