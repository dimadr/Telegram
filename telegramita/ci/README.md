# ARM64 Release CI

Base: f32ffeec763f0bdbf4bec25b52d654f77bff733d, Telegram 13.0.2 / 7175.
Mieru 3.38.1 is rebuilt from the locked Go module and existing bridge source.
Toolchain: Ubuntu 24.04, Java Temurin 17.0.20, Go 1.26.2, Gradle 8.13,
AGP 8.13.2, Android SDK 36, build-tools 36.0.0, NDK 27.2.12479018,
CMake 3.22.1. Ninja is bundled with that Android CMake package.

Only pushes to `telegramita` and manual dispatch build this workflow. Checkout
uses the event's exact commit, including pinned submodules. Other branches are
not modified. There is no automatic GitHub Release creation.

Artifacts are deliberately unsigned. They cannot update the existing signed
Telegramita installation. Signing credentials remain on the owner's machine.
The CI-only Firebase build fixture is generated from the repository's public
upstream configuration. No local configuration or release key is uploaded.

Independent Gradle, Go, SDK and compiler caches use toolchain/dependency keys,
not app version or commit SHA. Restore prefixes retain the same toolchain. Native
objects are stored through ccache, which hashes source, headers and compiler;
Windows `.cxx` trees are not copied to Linux. Native cache keys change only when
native inputs change, allowing immutable Actions caches to retain new objects.
There is no cache deletion or standard clean step. Cache capacity remains subject
to GitHub's repository quota/eviction; ccache is capped at 2 GiB. Artifacts retain
14 days; they are independent of caches. Secret files are outside all cache paths.

Each command phase records Moscow timestamps, monotonic duration and cache state
in `ci-output/timing.jsonl`. GitHub job-step timestamps supply checkout, restore,
tool setup and artifact-upload durations. `build-report.json` records commit,
APK size/hash, source versions, ABI and verification status. Full logs are artifacts.

For cold/warm comparison dispatch the same `telegramita` commit again after its
first successful run (including cache-save post steps). Do not clear caches.
