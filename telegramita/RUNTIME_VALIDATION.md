# Telegramita runtime validation

Record the device model, Android version, test date, APK SHA-256, and certificate
SHA-256 for every release candidate. Keep diagnostic and release results separate.

## Theme diagnosis

Use `Telegramita-12.10.1-arm64-v8a-diagnostic.apk` first. Capture these log fields in
both system light and system dark mode:

- `selectedAutoNightType`
- `currentTheme` and `currentThemeKey`
- `currentThemeDark`
- `needSwitchToTheme`
- `windowBackgroundWhite`
- `windowBackgroundWhiteBlackText`
- the `LoginActivity.updateColors` event

Commands after each mode change and app restart:

```powershell
adb logcat -d -v time | findstr TelegramitaTheme
```

Attach one screenshot of the login screen in each mode. Do not propose or apply the
final theme patch until the log and screenshot agree on the cause.

## Release identity

- [ ] Package is `org.telegramita.messenger`
- [ ] Launcher label is `Telegramita`
- [ ] Launcher icon is the Telegramita T/tunnel mark
- [ ] App is non-debuggable
- [ ] Certificate SHA-256 is `1A:14:B6:DA:51:C9:EA:A1:F0:40:A8:47:1E:91:84:48:78:83:61:38:59:53:05:52:F5:4F:ED:8F:D3:13:95:FA`
- [ ] Only `arm64-v8a` native code is present
- [ ] `apksigner` passes
- [ ] `zipalign -P16` passes

## Runtime smoke tests

- [ ] Fresh install succeeds
- [ ] App launches without FATAL, ANR, or native crash
- [ ] Light theme is readable
- [ ] Dark theme is readable
- [ ] Mieru TCP connects and Telegram traffic works
- [ ] Mieru UDP connects and Telegram traffic works
- [ ] Mieru TCP+UDP connects and Telegram traffic works
- [ ] Reconnect works after network interruption
- [ ] Telegram media/messages continue to work through embedded Mieru
- [ ] Android system VPN is not active or required

## Play Protect control

The current device baseline is CLEAN (`No harmful apps found`). After installing each
new release candidate, run a normal Play Protect scan and record the verdict. Do not
disable Play Protect or add behavior-hiding workarounds. Investigate only if HARMFUL
becomes reproducible for a specific release APK.
