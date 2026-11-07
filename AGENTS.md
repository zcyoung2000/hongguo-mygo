# AGENTS.md - Agent & Developer Working Guidelines

This repository is a native macOS short-drama browsing client. Before starting development, check whether `main` contains commits missing from the current branch; merge `main` first when it does.

## Core rules

1. Keep the desktop runtime CGO-free: verify with `CGO_ENABLED=0 go build -o /dev/null .`.
2. Keep user-visible library data local and authentic. Do not create fake history, favorites, likes, or reservations.
3. Do not add account-bound workflows, private API adapters, community interaction surfaces, or rate-limit reset workflows.
4. Use inline SVG icons with `fill="currentColor"` in the UI. Do not add emoji to HTML, JavaScript, CSS, or notifications.
5. Use responsive `.drama-grid` / `.drama-card` layouts for collection views, including history.
6. All drama and poster images must use `handleImgError(this)`.
7. Delegate implementation tasks to a Codex subagent with `sc layout run tabs --provider codex --model zhipu/glm-5.3-flash` unless the user explicitly chooses another model; use a separate app-managed worktree branch, then review and merge the result back to the current target branch.
8. Do not rely on `sc worktree create` for child-agent completion wakeups; its completed sessions currently lack parent association. Read or poll `sc agents list --worktree <path>` when using that path.
9. Send follow-ups to the same wakeable child only when they refine that child's current task; start a new `sc layout run tabs` child for every unrelated task. Attach user screenshots by copying them to a stable absolute path and giving that path to the child; verify the child is idle before sending, and do not rely on queue delivery.

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
