const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const functions = [...source.matchAll(/^(?:async )?function (\w+)\(/gm)];
function load(c, name) {
  const i = functions.findIndex(match => match[1] === name);
  assert.ok(i >= 0, `missing ${name}`);
  vm.runInContext(source.slice(functions[i].index, functions[i + 1]?.index), c);
}
const flush = () => new Promise(resolve => setImmediate(resolve));

function harness() {
  let now = 10000, nextTimer = 0;
  const timers = new Map(), moves = [], hints = [], loads = [];
  class Clock extends Date { static now() { return now; } }
  const layer = () => ({ style: {}, classList: { add() {}, remove() {} } });
  const slots = [0, 1, -1].map((index, id) => ({
    id, itemIndex: index, seriesId: `series-${index}`, quality: '720p',
    ready: index >= 0, expiresAt: now + 60000, streamUrl: `http://localhost/video/${index}`,
    layer: layer(),
    video: { src: `http://localhost/video/${index}`, readyState: 4, videoWidth: 320,
      currentTime: 0, duration: 60, pause() {}, play() { return Promise.resolve(); } }
  }));
  const nodes = {
    'player-modal': { style: { display: 'none' } },
    'feed-playlist-drawer': { classList: { contains: () => false } }
  };
  const c = vm.createContext({
    console, URL, Date: Clock, performance: { now: () => now },
    window: { location: { href: 'http://localhost/' } },
    document: { getElementById: id => nodes[id] || null },
    setTimeout(fn, delay = 0) { timers.set(++nextTimer, { fn, at: now + delay }); return nextTimer; },
    clearTimeout: id => timers.delete(id),
    HTMLMediaElement: { HAVE_CURRENT_DATA: 2 },
    commentState: { open: false }, nativeFeedScrollReady: false,
    feedLastFallbackScrollAt: -Infinity, feedScrollCaptureSync: null,
    feedSlots: slots, feedSlotCurr: 0, feedSlotNext: 1, feedSlotPrev: 2, feedPlayToken: 1,
    state: { currentView: 'recommend', feed: {
      index: 0, lastSwitchTime: 0, items: [0, 1, 2, 3].map(i => ({ id: `series-${i}` })),
      hasMore: false, loading: false, userPaused: false
    } },
    feedDesiredQuality: () => '720p',
    showFeedHint: text => hints.push(text), persistFeedProgress() {},
    prepareFeedStartPosition() {}, loadFeedDramaMetadata() {}, hideFeedPosterImmediately() {},
    ensureFeedItemsAhead() {}, scheduleFeedWarmup() {}, scheduleFeedAutoplayRetry() {},
    updateFeedDramaInfo() {}, recordFeedSwitchMetric() {}, scheduleFeedRecoveryAdvance() {},
    feedVideoHasPlaybackBuffer: video => video.readyState >= 3,
    primeFeedSlotFirstFrame: () => Promise.resolve(),
    beginFeedPlayback: video => video.play(),
    activateFeedDrama(drama, index) { c.state.feed.index = index; moves.push(drama.id); },
    setActiveFeedSlot: slot => { c.feedSlotCurr = slot.id; },
    // Leave a cold decoder pending; navigation must still honor its deadline.
    prepareFeedSlot(slot, index) {
      loads.push(index);
      slot.preparing = new Promise(() => {});
      return slot.preparing;
    },
    ensureFeedSource: (slot, index) => loads.push(['foreground', index])
  });
  for (const name of ['feedVideoHasFirstFrame', 'normalizeFeedMediaURL', 'feedSlotMatches',
    'feedQualityForItem', 'shouldCaptureFeedScroll', 'handleNativeFeedScroll', 'syncFeedScrollCapture']) load(c, name);
  const controllerStart = source.indexOf('let isFeedAnimating = false;');
  const controllerEnd = source.indexOf('async function enterSeriesPlayer(', controllerStart);
  vm.runInContext(source.slice(controllerStart, controllerEnd), c);
  async function advance(ms) {
    const end = now + ms;
    for (let count = 0; ; count++) {
      assert.ok(count < 1000, 'navigation timers must not spin indefinitely');
      const next = [...timers].filter(([, timer]) => timer.at <= end)
        .sort((a, b) => a[1].at - b[1].at || a[0] - b[0])[0];
      if (!next) break;
      now = next[1].at;
      timers.delete(next[0]);
      next[1].fn();
      await flush();
    }
    now = end;
    await flush();
  }
  return { c, slots, moves, hints, loads, advance };
}

test('a decoder without a first frame cannot reset the navigation deadline forever', async () => {
  const { c, slots, moves, advance, loads } = harness();
  slots[1].ready = false;
  slots[1].video.readyState = 0;
  c.switchFeedDrama(1);
  await advance(349);
  assert.equal(c.state.feed.index, 0);
  await advance(1);
  assert.equal(c.state.feed.index, 1);
  assert.deepEqual(moves, ['series-1']);
  assert.equal(loads.filter(item => Array.isArray(item)).length, 1);
});

test('a first frame that arrives within the short grace period hands off immediately', async () => {
  const { c, slots, advance, moves } = harness();
  slots[1].video.readyState = 1;
  c.switchFeedDrama(1);
  await advance(100);
  slots[1].video.readyState = 3;
  await advance(50);
  assert.deepEqual(moves, ['series-1']);
});

test('animation retains one downward intent and coalesces key repeats', async () => {
  const { c, advance, moves } = harness();
  c.switchFeedDrama(1);
  await advance(100);
  for (let i = 0; i < 12; i++) c.switchFeedDrama(1);
  await advance(1100);
  assert.deepEqual(moves, ['series-1', 'series-2']);
});

test('the latest opposite direction replaces an intent queued during animation', async () => {
  const { c, advance, moves } = harness();
  c.switchFeedDrama(1);
  await advance(100);
  c.switchFeedDrama(1);
  c.switchFeedDrama(-1);
  await advance(700);
  assert.deepEqual(moves, ['series-1', 'series-0']);
});

test('a page-boundary load continues the original navigation without swallowing it', async () => {
  const { c, moves } = harness();
  c.state.feed.items.length = 1;
  c.state.feed.hasMore = true;
  let resolve;
  c.loadFeed = () => { c.state.feed.loading = true; return new Promise(done => { resolve = done; }); };
  c.switchFeedDrama(1);
  c.state.feed.items.push({ id: 'series-1' });
  c.state.feed.loading = false;
  resolve();
  await flush();
  assert.deepEqual(moves, ['series-1']);
});

test('an empty boundary page does not recursively fetch the catalog', async () => {
  const { c, advance, moves } = harness();
  c.state.feed.items.length = 1;
  c.state.feed.hasMore = true;
  let calls = 0;
  c.loadFeed = () => { calls++; return Promise.resolve(); };
  c.switchFeedDrama(1);
  await flush();
  await advance(2000);
  assert.equal(calls, 1);
  assert.equal(moves.length, 0);
});

test('a reversed or cancelled boundary request cannot navigate on a late page reply', async () => {
  const { c, advance, moves } = harness();
  c.state.feed.items.length = 1;
  c.state.feed.hasMore = true;
  let resolve;
  c.loadFeed = () => new Promise(done => { resolve = done; });
  c.switchFeedDrama(1);
  c.switchFeedDrama(-1);
  c.state.feed.items.push({ id: 'series-1' });
  resolve();
  await flush();
  await advance(2000);
  assert.equal(moves.length, 0);
});

test('leaving the feed cancels a waiting navigation and opening comments drops queued input', async () => {
  const { c, advance, slots, moves } = harness();
  slots[1].ready = false;
  c.switchFeedDrama(1);
  c.state.currentView = 'history';
  c.cancelPendingFeedSwitch();
  await advance(1000);
  assert.equal(moves.length, 0);
  c.state.currentView = 'recommend';
  slots[1].ready = true;
  c.switchFeedDrama(1);
  c.switchFeedDrama(1);
  c.commentState.open = true;
  await advance(1000);
  assert.deepEqual(moves, ['series-1']);
});

function inputs() {
  const h = harness();
  const { c } = h;
  const actions = [], handlers = {};
  c.switchFeedDrama = dir => actions.push(dir);
  c.recommendView = { addEventListener: (event, handler) => { handlers[event] = handler; } };
  const start = source.indexOf('  let wheelArmed = true;');
  const end = source.indexOf('  let pointerStart = null;', start);
  vm.runInContext(source.slice(start, end), c);
  c.document.addEventListener = (event, handler) => { handlers[event] = handler; };
  const keyStart = source.indexOf("  document.addEventListener('keydown', (e) => {", end);
  const keyEnd = source.indexOf('\n  if (qualityBtn && qualityMenu)', keyStart);
  vm.runInContext(source.slice(keyStart, keyEnd), c);
  function wheel(deltaY, deltaX = 0) {
    handlers.wheel({ deltaY, deltaX, deltaMode: 0, target: { closest: () => null }, preventDefault() {} });
  }
  return { ...h, actions, handlers, wheel };
}

test('DOM fallback accumulates a slow trackpad gesture and suppresses its inertial tail', async () => {
  const { wheel, advance, actions } = inputs();
  for (let i = 0; i < 60; i++) { wheel(4); await advance(10); }
  assert.deepEqual(actions, [1]);
  await advance(250);
  for (let i = 0; i < 20; i++) { wheel(4); await advance(10); }
  assert.deepEqual(actions, [1, 1]);
  wheel(100, 200);
  assert.equal(actions.length, 2, 'horizontal swipes must not navigate vertically');
});

test('native readiness disables the DOM fallback, and failure restores it', async () => {
  const { c, wheel, advance, actions } = inputs();
  c.apiCall = () => Promise.resolve(true);
  await c.syncFeedScrollCapture();
  wheel(100);
  assert.deepEqual(actions, []);
  c.handleNativeFeedScroll({ direction: 'down' });
  assert.deepEqual(actions, [1]);
  c.apiCall = () => Promise.reject(new Error('bridge unavailable'));
  await c.syncFeedScrollCapture();
  await advance(400);
  wheel(100);
  assert.deepEqual(actions, [1, 1]);
});

test('native startup and DOM fallback cannot double-navigate the same gesture', () => {
  const { c, wheel, actions } = inputs();
  wheel(100);
  c.handleNativeFeedScroll({ direction: 'down' });
  wheel(100);
  assert.deepEqual(actions, [1]);
});

test('down-arrow respects comments, drawers, and editable inputs', () => {
  const { c, handlers, actions } = inputs();
  const event = { code: 'ArrowDown', target: { tagName: 'DIV' }, preventDefault() {} };
  handlers.keydown(event);
  c.commentState.open = true;
  handlers.keydown(event);
  c.commentState.open = false;
  event.target.tagName = 'INPUT';
  handlers.keydown(event);
  event.target.tagName = 'DIV';
  c.document.getElementById('feed-playlist-drawer').classList.contains = () => true;
  handlers.keydown(event);
  assert.deepEqual(actions, [1]);
});
