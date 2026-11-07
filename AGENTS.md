# AGENTS.md

This repository is a native macOS short-drama browsing client. Before starting development, check whether `main` contains commits missing from the current branch; merge `main` first when it does.

## Core rules

1. Keep the desktop runtime CGO-free; keep user-visible library data local and authentic.
2. Do not add account-bound workflows, private API adapters, community interaction surfaces, or rate-limit reset workflows.
3. Use inline SVG icons with `fill="currentColor"`; never use emoji in UI code or notifications.
4. Use responsive `.drama-grid` / `.drama-card`; every drama/poster image must use `handleImgError(this)`.
5. Integrate catalog, search, detail, episode, and playback behind the OpenHongguo SDK; protected media must flow through local `MediaServer`.
6. Keep SQLite at `~/Library/Application Support/myguo/userdata.db` in WAL mode; expose IPC through exported service methods called by `apiCall(method, ...args)`.
7. Delegate implementation tasks with `sc layout run tabs --provider codex --model zhipu/glm-5.3-flash` to a separate app-managed worktree branch, then review and merge it into the current target branch. Start a new child for unrelated tasks; send screenshots by stable absolute path; send follow-ups only while the child is idle. Keep the primary worktree on its target branch; give the child an explicit separate worktree path and permit branch changes only there.
8. UI/rendering/long-list tests must use stable labels, handles or test hooks after two similar failures; never retry fixed pixel coordinates or repeatedly micro-edit patches. Use at least a 30-second timeout, then report matched elements and the failure instead of looping.

## Verify

```bash
go mod tidy
go test ./...
CGO_ENABLED=0 go build -o /dev/null .
```
