# YukiHub for Windows

The Windows desktop edition of YukiHub — a Galgame / visual novel library manager,
launcher and playtime tracker.

> **Status: early development.** This repository is a hard fork of LunaBox.
> De-branding, engineering hardening and product-layer rebuild are in progress.
> **There is no release build yet**, and the UI is still upstream's.
> See [docs/ROADMAP.md](docs/ROADMAP.md).

## What this is

YukiHub has two ends:

| End | Repository | License |
| --- | --- | --- |
| Android | https://github.com/xm486/YukiHub | GPL-3.0 |
| Windows desktop (this repository) | see repository URL | AGPL-3.0 |

The desktop edition is not a port of the Android app. Both ends share the same data
semantics and habits: game library, play sessions, metadata scraping, sync and backup.
Data can be imported and exported between the two ends.

## Capabilities

Inherited from the upstream baseline and usable today:

- Windows process detection and exit monitoring for automatic playtime tracking
- Locale Emulator launch (Japanese locale) and Magpie integration
- Metadata scraping (Bangumi, VNDB, Ymgal, Hikarinagi, Steam and more)
- Batch directory scan import, drag-and-drop import, and migration from
  Playnite / PotatoVN / Vnite / ReinaManager
- Save and database backup (local and cloud), multi-device sync
- System tray, launch at login, URL protocol, proxy, background mute
- CLI and MCP server
- NSIS installer and in-app incremental updates

Planned, ported from the Android end:

- Play session and sync contract compatible with the mobile app
  (see [migration design](docs/mobile-yukihub-migration.md), Chinese)
- AI play reports
- OCR and multi-engine translation workflow

Deferred:

- Offline 3D exhibition hall (implemented on Android; to be evaluated later)

Out of scope:

- Bundled Galgame engines and emulator launching (Windows runs native executables)

## Build from source

Requirements:

- Go (version in `go.mod`, currently 1.27.1)
- Node.js 24 and pnpm 9
- Wails v3 CLI, version **exactly matching** `github.com/wailsapp/wails/v3` in `go.mod`

```bash
cd frontend && pnpm install && cd ..
wails3 generate bindings -clean=true -ts
wails3 dev -config ./build/config.yml -port 9245
wails3 build
```

Before committing:

```bash
gofmt -l .
go vet ./...
go test ./... -count=1
```

## Fork notes

This repository is a hard fork of LunaBox v1.13.0. Branding in code has been replaced,
but the repository URL, third-party service credentials, code signing and update service
must be configured by YukiHub before any release. See [docs/fork-setup.md](docs/fork-setup.md)
(Chinese). Upstream attribution and the change log are in
[docs/upstream-lunabox.md](docs/upstream-lunabox.md).

## License

Licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.
See [LICENSE](LICENSE).

- This is a modified version based on LunaBox. It is **not** an official upstream release,
  and upstream provides no warranty or support.
- Upstream copyright belongs to the LunaBox contributors; upstream is also AGPL-3.0.
- See [NOTICE](NOTICE) and [docs/AGPL-COMPLIANCE.md](docs/AGPL-COMPLIANCE.md).
- Third-party licenses: [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md).

## Disclaimer

Use this project only to manage and launch games, applications or resources you have
the right to use. It does not provide game files, cracked resources, or any means to
bypass licensing.
