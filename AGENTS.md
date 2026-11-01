# AGENTS.md - Agent & Developer Working Guidelines

This repository is a native macOS short-drama browsing client. Before starting development, check whether `main` contains commits missing from the current branch; merge `main` first when it does.

## Core rules

1. Keep the desktop runtime CGO-free: verify with `CGO_ENABLED=0 go build -o /dev/null .`.
2. Keep user-visible library data local and authentic. Do not create fake history, favorites, likes, or reservations.
3. Do not add account-bound workflows, private API adapters, community interaction surfaces, or rate-limit reset workflows.
4. Use inline SVG icons with `fill="currentColor"` in the UI. Do not add emoji to HTML, JavaScript, CSS, or notifications.
5. Use responsive `.drama-grid` / `.drama-card` layouts for collection views, including history.
6. All drama and poster images must use `handleImgError(this)`.

## Integration boundaries

- Catalog, search, detail, episode, and playback integration belongs behind the OpenHongguo SDK.
- Protected media must continue to flow through the local `MediaServer`; do not bypass the media pipeline in the WebView.
- SQLite is stored in `~/Library/Application Support/myguo/userdata.db` and uses WAL mode.
- The IPC contract is exposed through exported service methods and called by `apiCall(method, ...args)` in `frontend/app.js`.

## Verification

```bash
go mod tidy
go test ./...
CGO_ENABLED=0 go build -o /dev/null .
```
