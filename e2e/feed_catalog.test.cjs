const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const functions = [...source.matchAll(/^(?:async )?function (\w+)\(/gm)];

function harness(responses, restored = { items: [] }) {
  const calls = [], plays = [], errors = [], restores = [];
  const c = vm.createContext({
    console: { error: (...args) => errors.push(args), warn() {} }, performance,
    clearTimeout() {}, feedRequestToken: 0, feedPlaybackGeneration: 0,
    state: { currentView: 'recommend', feed: {
      items: [{ id: 'current' }], index: 0, cursor: 'expired', channel: '推荐',
      hasMore: true, loading: false, emptyFeedPages: 0,
      preloadMap: new Map(), playbackInfoLoads: new Map()
    } },
    apiCall: (...args) => {
      if (args[0] === 'RestoreRecommendFeed') {
        restores.push(args);
        return Promise.resolve(restored);
      }
      calls.push(args);
      assert.ok(responses.length, 'unexpected catalog request');
      return Promise.resolve(responses.shift());
    },
    playFeedIndex: index => { plays.push(index); return Promise.resolve(); },
    recordFeedDiagnostic() {}, cancelScheduledFeedWarmup() {}, resetFeedVideoPool() {},
    showFeedSkeleton() {}, showFeedHint() {}, updateFeedPlayIndicator() {}, ensureFeedItemsAhead() {}
  });
  const i = functions.findIndex(match => match[1] === 'loadFeed');
  assert.ok(i >= 0, 'missing loadFeed');
  vm.runInContext(source.slice(functions[i].index, functions[i + 1].index), c);
  return { c, calls, plays, errors, restores };
}

test('an empty filtered batch advances the cursor used for the next request', async () => {
  const h = harness([
    { items: [], cursor: 'after-repeats', hasMore: true },
    { items: [{ id: 'fresh' }], cursor: 'next', hasMore: true }
  ]);
  await h.c.loadFeed(false);
  assert.equal(h.c.state.feed.cursor, 'after-repeats');
  assert.equal(h.c.state.feed.hasMore, true);
  assert.equal(h.c.state.feed.emptyFeedPages, 1);
  assert.deepEqual(h.c.state.feed.items.map(item => item.id), ['current']);

  await h.c.loadFeed(false);
  assert.equal(h.calls[1][2], 'after-repeats');
  assert.deepEqual(h.c.state.feed.items.map(item => item.id), ['current', 'fresh']);
  assert.equal(h.c.state.feed.emptyFeedPages, 0);
  assert.deepEqual(h.plays, [], 'a buffer fill must not restart current playback');
  assert.deepEqual(h.errors, []);
});

test('an empty stream clears the obsolete cursor before the next retry', async () => {
  const h = harness([
    { items: [], cursor: '', hasMore: true },
    { items: [], cursor: 'new-stream', hasMore: true }
  ]);
  await h.c.loadFeed(false);
  assert.equal(h.c.state.feed.cursor, '');
  await h.c.loadFeed(false);
  assert.equal(h.calls[1][2], '');
  assert.deepEqual(h.errors, []);
});

test('cold start begins playback when a retry finds content after an empty page', async () => {
  const h = harness([
    { items: [], cursor: 'after-repeats', hasMore: true },
    { items: [{ id: 'fresh' }], cursor: 'next', hasMore: true }
  ]);
  await h.c.loadFeed(true, '推荐');
  assert.equal(h.c.state.feed.items.length, 0);
  assert.deepEqual(h.plays, []);
  await h.c.loadFeed(false);
  assert.equal(h.calls[1][2], 'after-repeats');
  assert.deepEqual(Array.from(h.c.state.feed.items, item => item.id), ['fresh']);
  assert.deepEqual(h.plays, [0]);
  assert.deepEqual(h.errors, []);
});

test('a stale empty response cannot overwrite a refreshed feed cursor', async () => {
  let resolveOld;
  const oldResponse = new Promise(resolve => { resolveOld = resolve; });
  const h = harness([
    oldResponse,
    { items: [{ id: 'fresh' }], cursor: 'new-stream', hasMore: true }
  ]);
  const oldRequest = h.c.loadFeed(false);
  await h.c.loadFeed(true, '推荐');
  resolveOld({ items: [], cursor: 'old-stream', hasMore: true });
  await oldRequest;
  assert.equal(h.c.state.feed.cursor, 'new-stream');
  assert.deepEqual(Array.from(h.c.state.feed.items, item => item.id), ['fresh']);
  assert.deepEqual(h.errors, []);
});

test('startup plays the restored queue before any recommendation request and appends background results', async () => {
  const h = harness([
    { items: [{ id: 'local-first' }, { id: 'new' }], cursor: 'next-page', hasMore: true }
  ], { items: [{ id: 'local-first' }, { id: 'local-next' }], cursor: 'saved-page', hasMore: true });
  await h.c.loadFeed(true, '推荐');
  assert.equal(h.restores.length, 1);
  assert.equal(h.calls.length, 0, 'startup must not wait for the recommendation network');
  assert.deepEqual(Array.from(h.c.state.feed.items, item => item.id), ['local-first', 'local-next']);
  assert.deepEqual(h.plays, [0]);
  await h.c.loadFeed(false);
  assert.equal(h.calls[0][2], 'saved-page');
  assert.deepEqual(Array.from(h.c.state.feed.items, item => item.id), ['local-first', 'local-next', 'new']);
  assert.deepEqual(h.plays, [0], 'background fill must preserve current playback');
  assert.deepEqual(h.errors, []);
});

test('empty or unavailable local queues fall back to a fresh recommendation request', async () => {
  for (const restored of [{ items: [] }, Promise.reject(new Error('local read failed'))]) {
    const h = harness([{ items: [{ id: 'remote' }], cursor: 'page-1', hasMore: true }], restored);
    await h.c.loadFeed(true, '推荐');
    assert.equal(h.calls.length, 1);
    assert.equal(h.calls[0][0], 'GetRecommendFeed');
    assert.deepEqual(h.plays, [0]);
    assert.deepEqual(h.errors, []);
  }
});

test('explicit refresh and favorites bypass the pending recommendation queue', async () => {
  for (const [channel, forceRefresh] of [['推荐', true], ['关注', false]]) {
    const h = harness([{ items: [{ id: 'new' }], cursor: '', hasMore: true }], { items: [{ id: 'old' }] });
    await h.c.loadFeed(true, channel, forceRefresh);
    assert.equal(h.restores.length, 0);
    assert.equal(h.calls.length, 1);
    assert.deepEqual(Array.from(h.c.state.feed.items, item => item.id), ['new']);
  }
});

test('a late queue restore cannot replace a newer channel or start a stale network fetch', async () => {
  let resolveRestore;
  const pending = new Promise(resolve => { resolveRestore = resolve; });
  const h = harness([{ items: [{ id: 'favorite' }], cursor: '', hasMore: false }], pending);
  const oldLoad = h.c.loadFeed(true, '推荐');
  await h.c.loadFeed(true, '关注');
  resolveRestore({ items: [{ id: 'stale' }], cursor: 'stale-page', hasMore: true });
  await oldLoad;
  assert.equal(h.calls.length, 1);
  assert.deepEqual(Array.from(h.c.state.feed.items, item => item.id), ['favorite']);
  assert.deepEqual(h.plays, [0]);
});
