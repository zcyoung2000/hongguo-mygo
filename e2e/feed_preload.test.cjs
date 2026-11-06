const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const declarations = [...source.matchAll(/^(?:async )?function (\w+)\(/gm)];

function load(context, names) {
  for (const name of names) {
    const index = declarations.findIndex(match => match[1] === name);
    const start = declarations[index].index;
    const next = declarations[index + 1]?.index ?? source.length;
    const state = source.indexOf('\nconst state =', start);
    vm.runInContext(source.slice(start, state > start ? Math.min(state, next) : next), context);
  }
}

function policy() {
  const c = vm.createContext({});
  load(c, ['decideFeedPreload']);
  let previous;
  return sample => {
    previous = c.decideFeedPreload({
      now: 0, bufferSec: 30, rate: 1, complete: false, waiting: false,
      profile: 'strong', nextReady: false, secondReady: false, available: 10,
      ...sample
    }, previous);
    return previous;
  };
}

test('uses spare buffer progressively, with closer playable prefixes required first', () => {
  const decide = policy();
  assert.equal(decide({ now: 0 }).depth, 1);
  assert.equal(decide({ now: 1000 }).depth, 1);
  assert.equal(decide({ now: 3000 }).depth, 1);
  assert.equal(decide({ now: 4000, nextReady: true }).depth, 2);
  assert.equal(decide({ now: 5000, nextReady: true }).depth, 2);
  assert.equal(decide({ now: 6000, nextReady: true, secondReady: true }).depth, 2);
});

test('cold start reserves all media bandwidth for the first video', () => {
  const decide = policy();
  for (let now = 0; now <= 5000; now += 1000) {
    assert.equal(decide({ now, bufferSec: now / 1000 }).depth, 0);
  }
  assert.equal(decide({ now: 6000, bufferSec: 9 }).depth, 1);
  assert.equal(decide({ now: 7000, bufferSec: 10 }).depth, 1);
});

test('hysteresis preserves the next preload through small buffer fluctuations', () => {
  const decide = policy();
  decide({ now: 0, bufferSec: 9 });
  assert.equal(decide({ now: 1000, bufferSec: 9 }).depth, 1);
  for (const [i, bufferSec] of [7.8, 8.1, 7.5, 8.5, 6, 7].entries()) {
    assert.equal(decide({ now: 2000 + i * 1000, bufferSec }).depth, 1);
  }
});

test('starvation releases all speculative work immediately and recovery has a cooldown', () => {
  const decide = policy();
  decide({ now: 0 });
  decide({ now: 1000 });
  assert.equal(decide({ now: 3000, nextReady: true }).depth, 2);
  assert.equal(decide({ now: 4000, bufferSec: 20, waiting: true }).depth, 0);
  for (const now of [5000, 6000, 7000]) {
    assert.equal(decide({ now, bufferSec: 20, nextReady: true }).depth, 0);
  }
  assert.ok(decide({ now: 8000, bufferSec: 20, nextReady: true }).depth > 0);
});

test('a falling reserve contracts the window before the player starves', () => {
  const decide = policy();
  decide({ now: 0, bufferSec: 15 });
  decide({ now: 1000, bufferSec: 15 });
  assert.equal(decide({ now: 3000, bufferSec: 15, nextReady: true }).depth, 2);
  decide({ now: 4000, bufferSec: 13, nextReady: true });
  assert.equal(decide({ now: 5000, bufferSec: 11, nextReady: true }).depth, 1);
  assert.equal(decide({ now: 6000, bufferSec: 3, nextReady: true }).depth, 0);
});

test('2x playback needs twice as much media buffer to expand the preload window', () => {
  const normal = policy();
  const fast = policy();
  for (const now of [0, 1000, 3000, 5000]) {
    normal({ now, bufferSec: 14, nextReady: true });
    assert.equal(fast({ now, bufferSec: 14, rate: 2, nextReady: true }).depth, 1);
  }
  assert.equal(normal({ now: 6000, bufferSec: 14, nextReady: true }).depth, 2);
  const insufficient = policy();
  assert.equal(insufficient({ now: 0, bufferSec: 10, rate: 2 }).depth, 0);
});

test('a fully buffered short video can use idle bandwidth, within network and list caps', () => {
  for (const [profile, expected] of [['weak', 1], ['unknown', 2], ['medium', 2], ['strong', 2]]) {
    const decide = policy();
    for (const now of [0, 1000, 3000]) decide({ now, profile, complete: true, bufferSec: 2, nextReady: true, secondReady: true });
    assert.equal(decide({ now: 6000, profile, complete: true, bufferSec: 1, nextReady: true, secondReady: true }).depth, expected);
    assert.equal(decide({ now: 7000, profile, complete: true, available: 1 }).depth, 1);
    assert.equal(decide({ now: 8000, profile, complete: true, available: 0 }).depth, 0);
  }
});

test('a transient network downgrade cannot immediately reopen farther downloads', () => {
  const decide = policy();
  for (const now of [0, 1000, 3000, 6000]) decide({ now, nextReady: true, secondReady: true });
  assert.equal(decide({ now: 7000, profile: 'weak', nextReady: true, secondReady: true }).depth, 1);
  for (const now of [8000, 9000]) {
    assert.equal(decide({ now, nextReady: true, secondReady: true }).depth, 1);
  }
  assert.equal(decide({ now: 10000, nextReady: true, secondReady: true }).depth, 2);
});

function scheduler() {
  const timers = new Map();
  const calls = [];
  let timerID = 0;
  let now = 0;
  const current = { episodeId: 'current', itemIndex: 0, quality: '720p', video: {
    src: 'local-current', readyState: 4, videoWidth: 320, currentTime: 5, playbackRate: 1, ended: false
  } };
  const next = { itemIndex: -1 };
  const c = vm.createContext({
    console, Date, performance: { now: () => now },
    setTimeout: callback => { timers.set(++timerID, callback); return timerID; },
    clearTimeout: id => timers.delete(id),
    HTMLMediaElement: { HAVE_CURRENT_DATA: 2, HAVE_FUTURE_DATA: 3 },
    feedWarmupToken: 1, feedWarmupSession: null, feedWarmupCenterKey: 'center',
    feedPreloadRevision: 0, feedSlots: [current, next], feedSlotCurr: 0, feedSlotNext: 1,
    feedSwitchWaitDir: 0,
    state: { currentView: 'recommend', feed: { index: 0, quality: '720p', userPaused: false,
      items: [{ id: 'current' }, { id: 'next' }, { id: 'far' }] } },
    ensureFeedItemsAhead() {}, refreshFeedNetworkDiagnostics() {},
    effectiveFeedNetworkProfile: () => 'strong', feedVideoBufferAhead: () => 30,
    feedMediaDuration: () => 60,
    cachedFeedPlayback: i => i > 0 ? { episodeId: `ep-${i}`, quality: '540p', episodeNum: 1 } : null,
    prepareFeedSlot: (...args) => { calls.push(['prepare', ...args]); return Promise.resolve(); },
    clearFeedSlot: slot => { calls.push(['clear', slot]); slot.itemIndex = -1; },
    apiCall: (...args) => { calls.push(args); return Promise.resolve([]); }
  });
  load(c, ['decideFeedPreload', 'feedVideoHasFirstFrame', 'feedSlotIsFullyBuffered', 'updateFeedPreloadWindow', 'cancelScheduledFeedWarmup', 'wakeFeedWarmup', 'runFeedWarmup']);
  const session = { token: 1, centerIndex: 0, current, running: false, timer: null,
    lastDiagnostics: Infinity, policy: {}, statuses: new Map(), metadataTries: new Map() };
  c.feedWarmupSession = session;
  return { c, session, calls, timers, setNow: value => { now = value; } };
}

test('periodic controller warms real episode IDs and their selected quality without another play event', async () => {
  const { c, session, calls, setNow } = scheduler();
  await c.runFeedWarmup(session);
  assert.equal(calls[0][0], 'UpdateFeedPreloadWindow');
  assert.equal(calls[0].at(-1).length, 1);
  setNow(1000);
  await c.runFeedWarmup(session);
  const window = calls.filter(call => call[0] === 'UpdateFeedPreloadWindow').at(-1).at(-1);
  assert.equal(window.length, 1);
  assert.equal(window[0].episodeId, 'ep-1');
  assert.equal(window[0].quality, '540p');
  assert.ok(calls.some(call => call[0] === 'prepare'));
});

test('a cold first frame gets all bandwidth before any lookahead negotiation', async () => {
  const { c, session, calls } = scheduler();
  session.current.video.readyState = 1;
  session.current.video.videoWidth = 0;
  c.cachedFeedPlayback = () => null;
  let metadataCalls = 0;
  c.prefetchFeedPlaybackInfo = () => { metadataCalls++; return Promise.resolve(null); };
  await c.runFeedWarmup(session);
  assert.equal(metadataCalls, 0);
  assert.equal(calls.filter(call => call[0] === 'prepare').length, 0);
  assert.equal(calls.find(call => call[0] === 'UpdateFeedPreloadWindow').at(-1).length, 0);
  Object.assign(session.current.video, { readyState: 4, videoWidth: 320 });
  await c.runFeedWarmup(session);
  assert.equal(metadataCalls, 2, 'lookahead begins after the first frame is available');
});

test('pressure clears the offscreen decoder and sends an empty media window with current protected', async () => {
  const { c, session, calls, setNow } = scheduler();
  c.feedSlots[1].itemIndex = 1;
  session.policy = { depth: 2 };
  c.feedVideoBufferAhead = () => 2;
  setNow(1000);
  await c.runFeedWarmup(session);
  assert.equal(session.policy.depth, 0);
  assert.equal(calls[0][0], 'clear');
  assert.equal(calls[1][2], 'current');
  assert.equal(calls[1].at(-1).length, 0);
});

test('an explicit pending swipe keeps its target decoder despite pressure on the outgoing video', async () => {
  const { c, session, calls } = scheduler();
  c.feedSlots[1].itemIndex = 1;
  c.feedVideoBufferAhead = () => 2;
  c.feedSwitchWaitDir = 1;
  await c.runFeedWarmup(session);
  assert.equal(calls.filter(call => call[0] === 'clear').length, 0);
  assert.equal(calls.find(call => call[0] === 'UpdateFeedPreloadWindow').at(-1).length, 1);
});

test('a handoff readiness transition with buffered media does not cancel the next preload', async () => {
  const { c, session, calls } = scheduler();
  session.current.video.readyState = 2;
  session.current.mediaWaiting = true;
  session.policy = { depth: 1 };
  c.feedVideoBufferAhead = () => 8;
  c.feedSlots[1].itemIndex = 1;
  await c.runFeedWarmup(session);
  assert.equal(session.policy.depth, 1);
  assert.equal(calls.filter(call => call[0] === 'clear').length, 0);
  assert.equal(calls.find(call => call[0] === 'UpdateFeedPreloadWindow').at(-1).length, 1);
});

test('backpressure preserves an already cached adjacent decoder for reverse navigation', async () => {
  const { c, session, calls } = scheduler();
  Object.assign(c.feedSlots[1], { itemIndex: 1, completeCached: true, video: { src: 'local-cached' } });
  c.feedVideoBufferAhead = () => 2;
  await c.runFeedWarmup(session);
  assert.equal(session.policy.depth, 0);
  assert.equal(calls.filter(call => call[0] === 'clear').length, 0);
  assert.equal(calls.find(call => call[0] === 'UpdateFeedPreloadWindow').at(-1).length, 0,
    'keeping resident cached media must not admit speculative downloads');
});

test('a pending fragment decoder cannot acquire a duplicate HLS download under pressure', async () => {
  const { c, session, calls } = scheduler();
  Object.assign(c.feedSlots[1], { itemIndex: 1, ready: true, episodeId: 'ep-1',
    quality: '540p', progressiveMedia: {}, video: { src: 'blob:pending' } });
  c.feedVideoBufferAhead = () => 2;
  c.feedSwitchWaitDir = 1;
  await c.runFeedWarmup(session);
  assert.equal(calls.filter(call => call[0] === 'clear').length, 0);
  assert.equal(calls.find(call => call[0] === 'UpdateFeedPreloadWindow').at(-1).length, 0);
  assert.equal(calls.filter(call => call[0] === 'prepare').length, 0);
});

test('a slow old reply cannot prepare a decoder or rearm a timer after cancellation', async () => {
  const { c, session, timers, setNow, calls } = scheduler();
  let reply;
  c.apiCall = (...args) => {
    calls.push(args);
    return new Promise(resolve => { reply = resolve; });
  };
  session.policy = { stableSince: 0 };
  setNow(3000);
  const run = c.runFeedWarmup(session);
  c.cancelScheduledFeedWarmup({ releaseMedia: false });
  reply([{ episodeId: 'ep-1', quality: '540p', cachedSeconds: 10 }]);
  await run;
  assert.equal(calls.filter(call => call[0] === 'prepare').length, 0);
  assert.equal(timers.size, 0);
});

test('a waiting event does not overlap a running controller IPC call', async () => {
  const { c, session, calls } = scheduler();
  let reply;
  c.apiCall = (...args) => { calls.push(args); return new Promise(resolve => { reply = resolve; }); };
  const first = c.runFeedWarmup(session);
  assert.equal(session.pendingMedia.has('ep-1:540p'), true);
  c.wakeFeedWarmup(true);
  await c.runFeedWarmup(session);
  assert.equal(calls.length, 1);
  assert.equal(session.pending, true);
  reply([]);
  await first;
  assert.equal(session.pendingMedia, null);
});

test('while current is starved, lookahead negotiates metadata without starting media', async () => {
  const { c, session, calls } = scheduler();
  c.cachedFeedPlayback = () => null;
  c.feedVideoBufferAhead = () => 0;
  c.prefetchFeedPlaybackInfo = (...args) => { calls.push(['metadata', ...args]); return Promise.resolve(null); };
  await c.runFeedWarmup(session);
  assert.equal(calls.filter(call => call[0] === 'metadata').length, 2);
  assert.ok(calls.filter(call => call[0] === 'metadata').every(call => call[2] === false));
  assert.equal(calls.find(call => call[0] === 'UpdateFeedPreloadWindow').at(-1).length, 0);
  assert.equal(calls.filter(call => call[0] === 'prepare').length, 0);
});

test('pause releases speculative jobs, and leaving the feed also releases the old current', async () => {
  const { c, calls } = scheduler();
  c.state.feed.userPaused = true;
  c.cancelScheduledFeedWarmup();
  assert.equal(calls[0][2], 'current');
  assert.equal(calls[0].at(-1).length, 0);
  c.state.currentView = 'detail';
  c.cancelScheduledFeedWarmup();
  assert.equal(calls[1][2], '');
  assert.ok(calls[1][1] > calls[0][1]);
});
