# myguo

A native macOS client for short dramas, built with Go.

[English](README.md) | [简体中文](README.zh-CN.md)

## Overview

myguo embeds a vanilla JavaScript frontend in a native WKWebView window, communicates with Go through a typed IPC bridge, and plays media through a local streaming server. Catalog and playback integration is provided by the OpenHongguo SDK.

The project is intended for personal technical study and local use. It is not affiliated with or endorsed by the upstream content provider.

## Features

| Area | Description |
| --- | --- |
| Native desktop UI | Hidden-inset title bar, sidebar navigation, dark theme, responsive grids, and inline SVG icons. |
| Discovery | Recommendations, search and suggestions, filters, rankings, new-release schedules, actor pages, and series details. |
| Account data | Profile, history, favorites, likes, and reservations from authenticated upstream endpoints. |
| Comments and danmaku | Episode comments, replies, timed danmaku playback, sending, and likes. |
| Playback | Episode lists, quality selection, progress saving, continuous playback, and keyboard controls. |
| Media pipeline | A local server proxies video, decrypts protected streams through FFmpeg, and supports HTTP range requests. |
| Local persistence | SQLite stores account credentials, device identity, profile, and user library data with WAL mode and automatic legacy-data import. |
| CGO-free build | The Go runtime and SQLite driver build with `CGO_ENABLED=0`. |

## Requirements

- macOS 12 or later
- Go 1.27 or later
- Bash, curl, unzip, and `shasum` for fetching the bundled FFmpeg binary
- FFmpeg for development playback; release builds bundle a static binary

## Quick Start

```bash
git clone https://github.com/zcyoung2000/hongguo-mygo.git
cd hongguo-mygo
go mod tidy
go run .
```

### Build an App Bundle

```bash
./scripts/fetch-ffmpeg.sh
CGO_ENABLED=0 go run github.com/egoist/mygo/cmd/mygo build -skip-dmg
```

The bundle is written to `dist/darwin-arm64/myguo.app` on Apple Silicon, or `dist/darwin-amd64/myguo.app` on Intel.

## Account Sign-In

myguo can run with an anonymous device profile. To sync personal data:

1. Open **Settings**.
2. Choose **Sign in**.
3. Paste the account Cookie.
4. Optionally add `x-tt-token` for a longer-lived session.

The app can also detect credentials from a compatible local installation and offer a one-click import. Treat Cookies and tokens like passwords: keep them private and never commit or share them.

## Data and Storage

Application data is stored under `~/Library/Application Support/myguo/`. SQLite stores account credentials, device identity, the profile, history, favorites, likes, and reservations; there is no app-owned `account.json`. Cached media and converted images are stored separately and can be cleared from Settings.

## Architecture

```text
WKWebView frontend
        |
        v
MyGo IPC bridge
        |
        v
myguo Go services --> OpenHongguo SDK and request helpers
        |
        +--> SQLite user data
        +--> MediaServer and FFmpeg
```

## Development

```bash
go mod tidy
go test ./...
CGO_ENABLED=0 go build -o /dev/null .
```

When changing the UI, keep list views in responsive card grids, use inline SVG rather than emoji or bitmap glyphs, and preserve native window drag behavior.

## License

This project is provided for personal technical study and local use. Drama content and metadata remain the property of their respective owners.

Review `THIRD_PARTY.md` before redistribution. Release builds bundle GPL-enabled FFmpeg, so applicable GPL obligations apply to those binaries.
