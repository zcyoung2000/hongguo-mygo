---
name: mygo-web
description: Web frontends of a MyGo app — how pages load (dev server, embedded dist, custom protocols), the trust model for who may call Go, bound services with channels and typed events, the generated TypeScript client and mygo-runtime, and plugins, the official ones and writing one. Use when a window shows a web page, the frontend must call Go or receive events from it, a page needs CORS-free HTTP, a WebSocket with headers or a local database, or a Go feature should reach pages as a plugin. Native UI windows are mygo-components and mygo-usage; desktop services are mygo-desktop.
---

# Web frontends

A window shows a web page in the system webview; the frontend calls Go
through **bound services**, streams values through **channels**, and
receives **typed events**; `mygo generate` writes the TypeScript client,
checked and documented on both sides. A window of native UI skips every
piece of this: its view calls Go directly.

Scope: pages and what serves them.

## Where the rest is

| Material | Reached for |
|---|---|
| `$MYGO/docs/bindings.md`, where `MYGO=$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)` | bound services, the types table, errors, events, channels, the generated client, who may call, `Page.Eval` |
| `$MYGO/docs/frontend.md` | how pages load, custom protocols, trust and untrusted pages, `mygo-runtime` exports, custom title bars, links, downloads |
| `$MYGO/docs/plugins.md` and `$MYGO/docs/plugins/` | the official plugins; writing one |
| `$MYGO/docs/i18n.md` | ICU catalogs in Go and in pages |

## Resolve the version, then read the section

Protocol at `../mygo-usage`: read the section you are about to use in the
copy `go list -m -f '{{.Dir}}'` names, prose included. Done when every
option you pass exists in that copy.

## Need → API, and the rule that bites

| Need | API | The rule that bites |
|---|---|---|
| Pages call Go | `mygo.Bind(v)`, `mygo.BindAs("Name", v)` | Every call runs on a goroutine of its own — guard shared state with a mutex; `Bind` panics at startup on a taken name or a type JSON cannot encode |
| Stream values during one call | a `*mygo.Channel[T]` parameter | One channel serves one call; it closes when the method returns; `Send` backpressures past 1 MiB; the page going away cancels the method's context |
| Push changes to pages | `mygo.NewEvent[T]("name")` + `Emit`/`Broadcast`/`EmitPage` | Events emitted before the page is ready deliver once it is; `Emit`'s error ends the loop feeding a closed window; names are unique and `mygo:` is reserved |
| Run code in the page | `Page.Eval` / `mygo.EvalAs[T]` | Statements run as an async function body — `return` produces the value; a throw is an `*mygo.EvalError` |
| Attach to the caller | `mygo.CallerWindow(ctx)`, `mygo.CallerPage(ctx)` | A leading `context.Context` cancels when the page navigates away or its window closes — pass it to slow work |
| Decide who may call | `PageOptions.TrustedOrigins`, `PageOptions.Untrusted` | Trust follows the page's URL as it navigates; frames never call; `"*"` hands Go to every site the page shows |
| Serve app content at a URL | `mygo.Protocol.Handle` | Handlers run on their own goroutines and stream; each scheme is its own origin with its own CORS; WebKitGTK plays no media from a custom scheme — serve it from a loopback HTTP server |
| Values across the boundary | Go `encoding/json/v2` rules | `[]byte` arrives base64, `time.Duration` in nanoseconds, ids past 2⁵³ carry `json:"id,string"`, named constants become unions, `MarshalJSON` becomes `unknown`; fields need `json` tags to keep their names |
| Report a Go error to the page | return the error | The promise rejects with a `CallError`; its message is the error's text and `err.method` names the call |
| Page without a build step | `window.mygo.call/on/channel` | Missing arguments arrive in Go as zero values |

## The generated client

`mygo generate` builds the app and runs it with `MYGO_GENERATE` set:
`main` runs up to `App.Run`. Work that must not happen then — migrating a
database, claiming a device — goes in `App.WhenReady`. Methods become
lower camel case and Go doc comments become TSDoc; `mygo dev` and
`mygo build` regenerate it, so it is rarely run by hand.

## Plugins

`mygo.Use(fetch.Plugin, …)` before `App.Run`, and the frontend imports the
matching `@mygo-plugins/*` package. Each plugin's `New` takes options — the
fetch and WebSocket plugins take an `http.Client` and an `Allow` function.

| Need | Plugin |
|---|---|
| HTTP from Go, any header, no CORS | `fetch` |
| WebSocket with handshake headers | `websocket` |
| A local database, without cgo | `sqlite` |
| Page-side file watches | `watch` |
| A Sparkle-style update window | `updater` |
| A terminal view for native UI | `terminal` |
| Liquid Glass surfaces | `glass` |
| Game loop, camera, gamepads | `game` |
| Loading indicators | `loading` |
| Another process drives the app — end-to-end tests, agents | `automation` (opt-in, left out of release builds); its `mygo-automation` CLI snapshots windows and acts by short ref — `go install github.com/egoist/mygo/plugins/automation/cmd/mygo-automation`, and `mygo-automation skills` prints the version-matched guide |

Writing one: a `mygo.Plugin{Name, Service, Setup}`, its service bound under
`plugin:<name>` outside the generated client, its JavaScript package calling
through `mygo-runtime` as a peer dependency.

## Verify

Build and test as `../mygo-usage` says. The generated client is checked by
compiling it: keep the frontend's typecheck in the loop. The tester cannot
answer for the browser side — a page's rendering, real navigation, custom
protocol behaviour, Linux media: drive those by hand or through the
automation plugin.

Done when: the frontend typecheck passes on the regenerated client, and
each of those browser-side paths is named in the handoff as unverified,
left for a human or the automation plugin to drive.
