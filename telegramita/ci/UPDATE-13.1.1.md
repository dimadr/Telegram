# Verified baseline and update

Rollback baseline: branch `telegramita`, commit
`6c00c4740161dbe93d3e1086b9ba09ce5f50a222`, Telegram 13.0.2 (7175), Mieru 3.38.1.
Passing baseline runs: 41983994921 and 41986569550. The original branch stays fixed.
The owner has validated the baseline on POCO, Honor 200 and T-Mobile REVVL 7 Pro.

This branch merges only `dimadr/Telegram` upstream commit
`3de37c8cfb039452be7398909461da6288babf15` (Telegram 13.1.1 / 7237).
Mieru v3.39.0 resolves to `bcd14b88d0fb7e7707bb80e5d7f6c33675d26c06`;
the Go checksum is locked in go.sum. Its Client API matches the baseline API.
JNI/socketpair architecture and proxy migration code remain intact.

Upstream changes ANDROID_PLATFORM from 21 to 23; the Mieru compiler target follows
that change. Native cache keys now include the Gradle native arguments as actual
compiler inputs. Compatible toolchain restore prefixes and SDK caches remain.
GitHub scopes caches by branch: a new non-default branch cannot read arbitrary
caches from the non-default baseline branch. A cold new-branch run is therefore
possible even when keys are compatible; no cache deletion is performed.

The existing CI builds and verifies the exact new versions, using unsigned APKs.
It never uploads the signing key. Device networking and update installation are
owner tests; automated profile/URL/persistence tests do not claim live network PASS.
Any checked-in baseline libmieru.so is overwritten by the mandatory CI Mieru build;
APK verification checks the embedded 3.39.0 module checksum and JNI exports.
