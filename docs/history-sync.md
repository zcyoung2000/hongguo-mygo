# Watching history

History stays in `~/Library/Application Support/myguo/userdata.db` with SQLite WAL.
No production history is seeded or rewritten by this change.

## Classification

Both native and embedded history views require a known final episode and at least
95% of that episode's actual duration to label a series completed. Missing
metadata does not imply completion. An intermediate episode at 100% is unfinished.

OpenHongguo's `model.ContentShortSeries` is 1 (真人短剧), not a novel. The old JSON
migration, batch writer and query excluded that value, while the cloud merge even
queued its deletion. Those exclusions and automatic deletion are removed.

Cloud history's episode count is used when provided. A duration is reused only
for the same episode. For a final episode with missing duration, the existing SDK
episode provider supplies its duration; if absent, SDK ResolvePlayback supplies
DurationSeconds. This only reads metadata and does not play or bypass MediaServer.
If SDK lookup fails or duration is unavailable, completion stays unconfirmed.

## Existing bidirectional synchronization

- `SaveProgress` persists authentic playback progress, invalidates the history
  cache and queues an upload under the current identity. Background upload uses
  the existing history/progress reporting implementation and acknowledges only
  successful writes; failed writes remain queued for retry on subsequent saves
  or refreshes.
- `GetHistory` returns local rows promptly and revalidates in the background.
  `RefreshHistory` flushes pending mutations, paginates the cloud history and
  merges newer progress into SQLite. Native history loads local then refreshes;
  embedded history calls the same exported IPC methods using `apiCall`.
- Generation and identity checks discard stale reads. A recent local mutation
  prevents a lagging cloud snapshot from replacing local rows. Complete, settled
  snapshots can reflect deletions made elsewhere. HTTP failures, nonzero response
  codes and missing history payloads cannot become an authoritative empty list.
- Single-record deletion and clear-all are removed from native and embedded UI,
  application callbacks, library service and exported IPC. Store deletion helpers
  and legacy pending-operation handling remain internal compatibility machinery.

The pinned OpenHongguo SDK has no account-history list/upload methods. Existing
cloud history uses pre-existing signed requests in internal/library, with SDK
series/episode metadata. This change preserves that working architecture and adds
no private API adapter or account-binding flow. Moving all history reads/writes
behind SDK methods requires upstream SDK support; it is not simulated here.

Regression coverage uses temporary SQLite databases and stub HTTP/SDK metadata
providers. It verifies short-series retention, final-episode metadata, incomplete
and unknown progress, invalid cloud snapshots, IPC removal and native UI removal.
Authenticated live-cloud behavior is not asserted by these tests.
