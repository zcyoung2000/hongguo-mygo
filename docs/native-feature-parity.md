# Native migration feature parity

Baseline: `03a3d88`, the tip of `sc-superfluid-dewar-3975`, already an ancestor
of the current branch. `main` has no commits missing from this branch.
The comparison covers `frontend/index.html`, `app.js`, `player-chrome.js`,
`progressive-media.js`, `danmaku.js`, `ambient-glow.js`, and `internal/native`.

Browsing and collections use MyGo native views. The embedded document retains
the existing recommendation, full-player, and settings controllers. Source
negotiation, streaming, decryption, local library storage, and existing settings
operations continue through the same exported services and MediaServer.

| Area | Preserved behavior | Implementation and verification evidence |
| --- | --- | --- |
| Navigation | Recommend, discover, search, ranks, new releases, history, favorites, likes, reservations, settings; actor/detail entry; menu and notification navigation; sidebar resizing and persistence | `view.go`, `features.go`, `preferences.go`, native navigation handler in `main.go`; sidebar click, persisted-width, navigation-history and all-pages control tests in `internal/uiapp` |
| Recommend | Real channels, refresh, restored queue, vertical scroll/arrows, sound unlock, first-frame handoff, reversible navigation, adaptive preload and diagnostics; outgoing scenes stop hidden playback | Existing feed controller and media modules retained; `MediaHost.Recommend/Stop`; feed catalog, navigation, preload, feedback, playback and diagnostics JavaScript suites; native scene transition tests |
| Search/discover | Submit/clear, debounced suggestions and keyboard selection, dynamic genre filters, pagination/deduplication/retry, authentic poster fallback, actor-card entry | `service.go`, `features.go`, `view.go`, `artwork.go`; input, suggestion, filter/provider, async artwork and cursor-cycle regressions. Filter rows scroll separately to leave results accessible at the minimum window size |
| Rankings | Content and board tabs, ranks/promoted entries, metadata, pagination, actor links and explicit immediate playback | `feature_views.go`, `LoadRank`, `RankItemDTO.EpisodeCount`; ranking/actor and cursor-cycle tests. Pages deduplicate by content type and identity; changing boards discards obsolete results |
| New releases | Hot releases, calendar dates, upcoming/online state, reserve/cancel | `LoadReleases` uses the original distinct board/calendar services; calendar test checks date, upcoming state and reservation controls. Reservation changes propagate to discover/search/new-release snapshots |
| Detail | Explicit play, metadata/tags/stats/publish date, actors, original novel and tags, all episodes, seasons, adaptations, related shows, favorite/reservation actions, loading/retry and stale-response protection | `renderDetail`, `loadDetailAt`, `loadSupplement`, `RetryRelated`; detail supplement, related retry, stale navigation and mutation/read regressions. Opening a normal detail does not start hidden playback |
| Actor | Profile/avatar/tags/stats and paginated works | `renderActor`, `loadActor`; actor pagination and cursor-cycle tests. Only one request for the current actor runs at a time |
| History | Authentic records, completed/incomplete/search filters, progress/badges/time, delete/confirmed clear, original episode and position on resume | Native library views and existing store/services; last-episode completion, episode metadata/progress, removal and confirmation tests; end-to-end JavaScript controller regression verifies that the history position is passed as seek position, while the selected quality remains unchanged |
| Favorites/likes | Authentic records and counts, independent errors, removal and the correct liked episode | Separate library state/generations and per-episode removal; library provider/error and liked-episode removal tests. Mutations invalidate older reads; repeated pending actions cannot invert a local toggle twice |
| Reservations | Online/pending counts, date ordering, cancellation confirmation, online-show entry | `renderLibrary`, `visibleLibrary`, `reservationBefore`, existing reservation service; mutation/cancellation tests. One cancellation action is rendered per reservation card |
| Playback | Quality, speed, seek/buffer/time, volume/mute, playlist/current highlight, previous/next/next season, close/escape, fullscreen, PiP, orientation/fit and progress persistence | Original full player/control/media modules retained. Native host and player lifecycle suites cover scene routing, saved/explicit-zero/default resume positions, quality argument preservation and stale source callbacks. Native close callbacks carry a session so an old close cannot dismiss a newer player |
| Media interactions | Existing favorite/like actions, comments/replies, danmaku and persisted appearance controls, ambient glow | Original controller/modules and controls retained; `LibraryChanged` invalidates native cached reads. No additional upstream interaction API or account workflow introduced |
| Platform | Media keys/menu, Now Playing/artwork/rate, prevent display sleep, fullscreen/PiP events, notifications, confirmations and keyboard focus | Existing native implementations retained; scroll gestures and PiP now target the embedded Page; native callbacks route to the active scene. Sidebar/cards expose button roles and keyboard activation. Final live OS-interaction acceptance remains pending below |
| Settings | Existing profile/session/import actions, cache statistics/clear, device information/actions and about | `MediaHost.Settings` selects the original `loadSettings` scene; native scene routing tests. Existing settings code and APIs retained; no additional workflow added |
| Runtime | SDK/MediaServer integration, authentic SQLite/WAL data and CGO-free builds | Main service/store/media initialization retained; unit tests use isolated fixtures and never populate the user's library. Required Go checks, race tests, vet and JavaScript suites are recorded below |

User preference: no global Back/Forward buttons on any native page. Navigation
history remains internal. `TestNoGlobalBackForwardControlsOnAnyNativePage`
covers every native route, including actor and detail.

Grid row sizes account for metadata and actions. The headless MyGo rendering
test `TestNativeCollectionActionsFitTheMinimumWindow` checks actionable controls
at 1000 × 640 with the widest allowed sidebar. It also supports optional PNG
export with `MYGUO_NATIVE_SCREENSHOT_DIR`; these are test-fixture renders, not
screenshots of the running application or user-library records.

## Verification and remaining acceptance

Run from the repository root:

```sh
go mod tidy
go test ./...
go test -race ./...
go vet ./...
CGO_ENABLED=0 go build -o /dev/null .
node --test e2e/*.test.cjs
git diff --check
```

Latest results (2026-11-06): all required Go checks passed, including race
checks (270 test/subtest passes across 15 packages); all 127 JavaScript tests
passed. Vet, the CGO-disabled build and whitespace checks also passed. Six minimum-window fixture renders were exported and inspected; all collection
actions remain visible, including rank playback, history deletion, favorites
and likes removal, and the single reservation-cancellation action.

The legacy document-shell runner has been removed. The retained Node suite is
the JavaScript/controller regression suite; it does not replace native
navigation acceptance with a live rendered application.

Native page layouts and loading states are additionally audited through
`TestSkeletonsMatchOriginalPageShapes` and the optional
`MYGUO_NATIVE_SCREENSHOT_DIR=... go test ./internal/uiapp -run TestNativePageScreens`
fixture export. The exported states cover discover, rank, new releases, search,
history, favorites, likes, reservations, detail, actor and settings in both
loading and ready form (or the single settings form), with rank/list rows,
calendar cards, native section layouts, grid skeletons and the detail hero
skeleton checked as dark renders.

This session previously displayed the native recommendation and ranking pages.
A rebuilt application also starts and resolves real recommendation playback
through the local media service. However, desktop automation subsequently
returned `cgWindowNotFound` consistently, including for the running application.
The latest changes therefore have source, controller, race and headless native
rendering coverage; A live attempt to open `/Applications/myguo.app` beside the rebuilt bundle
exposed both CGWindows, but neither app activated reliably; the desktop
automation host then reported `cgWindowNotFound`, while `screencapture` showed
inactive/blank web views. Therefore final live side-by-side checks of every
route, media-key/menu behavior, fullscreen/PiP and settings interactions remain
unverified. The full
feature-parity goal must not be marked complete until that acceptance is done.
