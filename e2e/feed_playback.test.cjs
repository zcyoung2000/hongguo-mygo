// Scheduling regressions run without a browser; real demux/streaming is covered
// by internal/media's gated-upstream integration test.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const functions = [...source.matchAll(/^(?:async )?function (\w+)\(/gm)];

function harness() {
  const context = vm.createContext({
    console, URL, performance, setTimeout, clearTimeout,
    window: { location: { href: 'http://localhost/' } },
    document: { getElementById: () => null },
    HTMLMediaElement: { HAVE_CURRENT_DATA: 2, HAVE_FUTURE_DATA: 3, HAVE_ENOUGH_DATA: 4 },
    feedPlaybackGeneration: 0, feedPlayToken: 0, feedPlayPromise: null, feedWarmupSession: null,
    feedSlots: [], feedSlotCurr: 0, feedActiveVideo: null,
    state: {
      currentView: 'recommend',
      feed: {
        items: [{ id: 'series-a' }, { id: 'series-b' }],
        index: 0, quality: '720p', userPaused: false, playbackSpeed: 1,
        playbackPositions: new Map(), preloadMap: new Map(), playbackInfoLoads: new Map()
      }
    },
    feedDesiredQuality: () => '720p',
    syncActiveFeedSlot: () => {}, assessFeedMediaCapabilities: () => {},
    scheduleFeedFrameMetric: () => {}, scheduleFeedAutoplayRetry: () => {},
    scheduleFeedWarmup: () => {}, clearFeedFailureHint: () => {},
    ensureFeedItemsAhead() {}, loadFeedDramaMetadata: () => Promise.resolve(true),
    recordFeedPlaybackMetric() {},
    recordFeedDiagnostic() {}, updateFeedPlayIndicator() {},
    showFeedSkeleton: () => {}, reportFeedFailure: () => {},
    clearFeedSlot: slot => {
      slot.loadToken = (slot.loadToken || 0) + 1;
      slot.ready = false;
      slot.queueConsumed = false;
      slot.video.src = '';
      slot.preparing = null;
    }
  });
  for (const name of [
    'feedAssetKey', 'feedPlaybackKey', 'rememberFeedPlayback', 'normalizeFeedMediaURL',
    'feedSlotMatches', 'feedVideoHasFirstFrame', 'feedSlotForVideo', 'feedMediaDuration',
    'feedQualityForItem', 'rememberFeedQuality', 'feedSlotIsFullyBuffered',
    'primeFeedSlotFirstFrame', 'scheduleFeedFrameMetric',
    'requestFeedDramaMetadata', 'playFeedIndex',
    'prefetchFeedPlaybackInfo', 'assignFeedMediaSource', 'hasWarmFeedHLS', 'prepareFeedSlot', 'beginFeedPlayback',
    'ensureFeedSource',
    'playbackBufferedRange', 'updatePlaybackBufferIndicator'
  ]) {
    const index = functions.findIndex(match => match[1] === name);
    assert.ok(index >= 0, `missing ${name}`);
    vm.runInContext(source.slice(functions[index].index, functions[index + 1]?.index), context);
  }
  return context;
}

function playback({ hls = true } = {}) {
  return {
    episodeId: 'episode-a', episodeNum: 1, quality: '720p', durationSec: 60,
    isFullCached: false,
    streamUrl: hls ? 'http://localhost/hls/index.m3u8' : 'http://localhost/video?mode=progressive',
    hlsStreamUrl: hls ? 'http://localhost/hls/index.m3u8' : '',
    progressiveStreamUrl: 'http://localhost/video?mode=progressive',
    fullStreamUrl: 'http://localhost/video'
  };
}

function videoSlot() {
  const video = {
    src: '', dataset: {}, readyState: 0, videoWidth: 0, currentTime: 0, duration: NaN, paused: true,
    loadCount: 0, playCount: 0,
    load() { this.loadCount++; },
    play() { this.playCount++; this.paused = false; return Promise.resolve(); }
  };
  return { id: 0, video, loadToken: 0 };
}

test('metadata prefetch and foreground preparation share a single IPC load', async () => {
  const c = harness();
  let resolvePlayback;
  const calls = [];
  c.apiCall = (...args) => {
    calls.push(args);
    return new Promise(resolve => { resolvePlayback = resolve; });
  };
  const warmup = c.prefetchFeedPlaybackInfo(0);
  const slot = videoSlot();
  c.feedSlots = [slot];
  const prepare = c.prepareFeedSlot(slot, 0, true);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].at(-1), true, 'cold start must request the warmed HLS rendition');
  resolvePlayback(playback());
  await Promise.all([warmup, prepare]);
  assert.equal(slot.video.src, 'http://localhost/hls/index.m3u8');
  assert.ok(slot.video.playCount > 0, 'play must be requested before loadeddata exists');
  assert.equal(slot.video.readyState, 0, 'test must not simulate a completed download');
});

test('cold-start playback gets the first frame before requesting series decorations', async () => {
  const c = harness();
  const slot = videoSlot();
  c.feedSlots = [slot];
  const calls = [];
  c.activateFeedDrama = drama => { c.state.feed.seriesId = drama.id; };
  c.loadFeedDramaMetadata = drama => { calls.push(['detail', drama.id]); return Promise.resolve(true); };
  c.apiCall = (...args) => { calls.push(args); return Promise.resolve(playback()); };
  await c.playFeedIndex(0);
  assert.deepEqual(calls.map(call => call[0]), ['PlayEpisode']);
  Object.assign(slot.video, { readyState: 3, videoWidth: 320, duration: 60 });
  slot.video.onloadeddata();
  slot.video.onplaying();
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(calls.map(call => call[0]), ['PlayEpisode', 'ConsumeRecommendFeedItem', 'detail']);
});

test('foreground preparation exposes loading before playback metadata returns', async () => {
  const c = harness();
  let resolve, loadingUpdates = 0;
  c.apiCall = () => new Promise(done => { resolve = done; });
  c.updateFeedPlayIndicator = () => { loadingUpdates++; };
  const slot = videoSlot();
  c.feedSlots = [slot];
  const pending = c.prepareFeedSlot(slot, 0, true);
  assert.equal(slot.video.src, '');
  assert.ok(loadingUpdates > 0, 'a slow SDK reply must not leave the visible page without feedback');
  resolve(playback());
  await pending;
});

test('only a visible foreground frame consumes a pending recommendation', async () => {
  const c = harness();
  const current = videoSlot();
  const next = videoSlot();
  Object.assign(current, { itemIndex: 0, seriesId: 'series-a' });
  Object.assign(next, { itemIndex: 1, seriesId: 'series-b' });
  Object.assign(next.video, { src: 'http://localhost/next', readyState: 4, videoWidth: 320 });
  c.feedSlots = [current, next];
  const calls = [];
  c.apiCall = (...args) => { calls.push(args); return Promise.resolve(); };
  c.requestFeedDramaMetadata(next);
  c.requestFeedDramaMetadata(current);
  assert.equal(calls.length, 0, 'preloading and unresolved first frames must remain pending');
  Object.assign(current.video, { src: 'http://localhost/current', readyState: 4, videoWidth: 320 });
  c.requestFeedDramaMetadata(current);
  c.requestFeedDramaMetadata(current);
  assert.equal(calls.length, 1);
  assert.equal(calls[0][0], 'ConsumeRecommendFeedItem');
  assert.equal(calls[0][2], 'series-a');
});

test('the initial uncached video opens progressive fragments through MSE and reuses its blob decoder', async () => {
  const c = harness();
  const sources = [];
  c.window.ProgressiveMediaSource = class {
    static isSupported() { return true; }
    constructor(video, url, options) {
      this.url = url; this.objectURL = 'blob:cold-video'; this.options = options;
      video.src = this.objectURL; video.load(); sources.push(this);
    }
    dispose() { this.disposed = true; }
  };
  c.apiCall = () => Promise.resolve(playback());
  const slot = videoSlot();
  c.feedSlots = [slot];
  await c.prepareFeedSlot(slot, 0, true);
  assert.equal(sources.length, 1);
  assert.equal(sources[0].url, playback().progressiveStreamUrl);
  assert.equal(slot.metricStreamMode, 'progressive');
  const loads = slot.video.loadCount;
  await c.prepareFeedSlot(slot, 0, true);
  assert.equal(sources.length, 1);
  assert.equal(slot.video.loadCount, loads, 'blob ownership must count as a matching resident source');
  sources[0].options.onComplete();
  assert.equal(slot.completeCached, true);
  slot.video.onerror(new Error('unsupported configuration'));
  assert.equal(sources[0].disposed, true);
  assert.equal(slot.video.src, playback().hlsStreamUrl, 'a failed MSE source retains native HLS fallback');
});

test('completed or preloaded HLS keeps the existing rendition on the first play', async () => {
  const c = harness();
  c.window.ProgressiveMediaSource = class { static isSupported() { return true; } };
  c.apiCall = () => Promise.resolve({ ...playback(), isHlsCached: true });
  const slot = videoSlot();
  c.feedSlots = [slot];
  await c.prepareFeedSlot(slot, 0, true);
  assert.equal(slot.video.src, playback().hlsStreamUrl);
  assert.equal(slot.completeCached, true);
});

test('later cold videos and an explicit pending swipe can use fragments without waiting for HLS', async () => {
  for (const autoPlay of [true, false]) {
    const c = harness();
    c.state.feed.index = 0;
    c.state.feed.hasStartedPlayback = true;
    vm.runInContext('feedSwitchWaitDir = 1', c);
    c.window.ProgressiveMediaSource = class {
      static isSupported() { return true; }
      constructor(video, url) { this.url = url; this.objectURL = 'blob:next-cold'; video.src = this.objectURL; }
    };
    c.apiCall = () => Promise.resolve(playback());
    const slot = videoSlot();
    c.feedSlots = [slot];
    await c.prepareFeedSlot(slot, 1, autoPlay);
    assert.equal(slot.video.src, 'blob:next-cold');
    assert.equal(slot.progressiveMedia.url, playback().progressiveStreamUrl);
  }
});

test('active and in-flight HLS admission retain one transport instead of starting a duplicate MP4', async () => {
  for (const pending of [true, false]) {
    const c = harness();
    c.window.ProgressiveMediaSource = class {
      static isSupported() { return true; }
      constructor() { throw new Error('must reuse the admitted HLS download'); }
    };
    const key = 'episode-a:720p';
    c.feedWarmupSession = {
      statuses: new Map(pending ? [] : [[key, { active: true }]]),
      pendingMedia: new Set(pending ? [key] : [])
    };
    c.apiCall = () => Promise.resolve(playback());
    const slot = videoSlot();
    c.feedSlots = [slot];
    await c.prepareFeedSlot(slot, 0, true);
    assert.equal(slot.video.src, playback().hlsStreamUrl);
    assert.equal(slot.progressiveMedia, null);
  }
});

test('a streaming duration can still restore an authentic saved position', async () => {
  const c = harness();
  c.apiCall = () => Promise.resolve({ ...playback(), lastPosition: 20 });
  const slot = videoSlot();
  slot.video.duration = Infinity;
  c.feedSlots = [slot];
  await c.prepareFeedSlot(slot, 0, true);
  Object.assign(slot.video, { readyState: 3, videoWidth: 320 });
  slot.video.onloadeddata();
  assert.equal(slot.video.currentTime, 20);
});

test('later rendered frames cannot inflate the measured cold-start time', () => {
  const c = harness();
  let now = 200;
  c.performance = { now: () => now };
  c.feedVideoHasPlaybackBuffer = () => true;
  const callbacks = [];
  const slot = videoSlot();
  slot.metricPreparedAt = 100;
  Object.assign(slot.video, { src: 'blob:playing', readyState: 3, videoWidth: 320,
    requestVideoFrameCallback: callback => callbacks.push(callback) });
  c.scheduleFeedFrameMetric(slot);
  callbacks.shift()();
  assert.equal(slot.metricFirstFrameAt, 200);
  now = 1200;
  callbacks.shift()();
  assert.equal(slot.metricFirstFrameAt, 200);
});

test('activating a warm source with no first frame preserves its URL and decoder', async () => {
  const c = harness();
  const slot = videoSlot();
  Object.assign(slot, {
    itemIndex: 0, seriesId: 'series-a', quality: '720p', ready: true,
    streamUrl: 'http://localhost/hls/index.m3u8',
    progressiveStreamUrl: 'http://localhost/video?mode=progressive',
    expiresAt: Date.now() + 60_000
  });
  slot.video.src = slot.streamUrl;
  c.feedSlots = [slot];
  c.apiCall = () => { throw new Error('warm activation must not resolve again'); };
  await c.prepareFeedSlot(slot, 0, true);
  assert.equal(slot.video.loadCount, 0, 'must not discard buffered segments');
  assert.equal(slot.video.src, slot.streamUrl);
  assert.equal(slot.video.playCount, 1);
});

test('activation during preparation waits for the existing slot without resetting it', async () => {
  const c = harness();
  let resolvePlayback;
  let calls = 0;
  c.apiCall = () => {
    calls++;
    return new Promise(resolve => { resolvePlayback = resolve; });
  };
  const slot = videoSlot();
  c.feedSlots = [slot];
  const preload = c.prepareFeedSlot(slot, 0, false);
  const token = slot.loadToken;
  const activate = c.prepareFeedSlot(slot, 0, true);
  resolvePlayback(playback());
  await Promise.all([preload, activate]);
  assert.equal(calls, 1);
  assert.equal(slot.loadToken, token);
  assert.equal(slot.video.loadCount, 1);
  assert.equal(slot.video.playCount, 1);
});

test('a late preload reply promoted to the foreground still selects cold fragments and starts playback', async () => {
  const c = harness();
  let resolve;
  c.apiCall = () => new Promise(done => { resolve = done; });
  const slot = videoSlot();
  slot.id = 1;
  c.feedSlots = [videoSlot(), slot];
  c.window.ProgressiveMediaSource = class {
    static isSupported() { return true; }
    constructor(video, url) { this.url = url; this.objectURL = 'blob:promoted'; video.src = this.objectURL; }
  };
  c.primeFeedSlotFirstFrame = () => { throw new Error('the visible target must not remain background priming'); };
  const preparing = c.prepareFeedSlot(slot, 1, false, null, true);
  c.state.feed.index = 1;
  c.feedSlotCurr = 1;
  vm.runInContext('feedSwitchWaitDir = 0', c);
  resolve(playback());
  await preparing;
  assert.equal(slot.video.src, 'blob:promoted');
  assert.equal(slot.video.playCount, 1);
  assert.equal(slot.metricStreamMode, 'progressive');
});

test('a foreground waiter cannot restart the old slot after another swipe made it background', async () => {
  const c = harness();
  let resolve;
  c.apiCall = () => new Promise(done => { resolve = done; });
  const slot = videoSlot();
  slot.id = 1;
  c.feedSlots = [videoSlot(), slot];
  const preload = c.prepareFeedSlot(slot, 1, false);
  c.feedSlotCurr = 1;
  c.state.feed.index = 1;
  const activate = c.prepareFeedSlot(slot, 1, true);
  c.feedSlotCurr = 0;
  c.state.feed.index = 0;
  resolve(playback());
  await Promise.all([preload, activate]);
  assert.equal(slot.video.playCount, 0);
  assert.equal(slot.video.loadCount, 1, 'the late cache can remain resident without stealing playback');
});

test('metadata-only lookahead upgrades to HLS when admitted for media warmup', async () => {
  const c = harness();
  const calls = [];
  let resolveMetadata;
  c.apiCall = (...args) => {
    calls.push(args);
    if (!args.at(-1)) return new Promise(resolve => { resolveMetadata = resolve; });
    return Promise.resolve(playback());
  };
  const metadata = c.prefetchFeedPlaybackInfo(0, false);
  const media = c.prefetchFeedPlaybackInfo(0, true);
  resolveMetadata(playback({ hls: false }));
  await metadata;
  const result = await media;
  assert.deepEqual(calls.map(call => call.at(-1)), [false, true]);
  assert.equal(result.hlsStreamUrl, 'http://localhost/hls/index.m3u8');
});

test('lookahead with an HLS alternative promotes the same rendition without another IPC negotiation', async () => {
  const c = harness();
  let calls = 0;
  c.apiCall = () => {
    calls++;
    return Promise.resolve({ ...playback(), streamUrl: 'http://localhost/video?mode=progressive' });
  };
  await c.prefetchFeedPlaybackInfo(0, false);
  const slot = videoSlot();
  c.feedSlots = [slot];
  await c.prepareFeedSlot(slot, 0, true);
  assert.equal(calls, 1);
  assert.equal(slot.video.src, 'http://localhost/hls/index.m3u8',
    'the decoder must open the rendition already being warmed by the media window');
});

test('an unavailable HLS startup does not cause coalesced callers to retry forever', async () => {
  const c = harness();
  let calls = 0;
  c.apiCall = () => {
    calls++;
    return Promise.resolve(playback({ hls: false }));
  };
  const results = await Promise.all([
    c.prefetchFeedPlaybackInfo(0), c.prefetchFeedPlaybackInfo(0), c.prefetchFeedPlaybackInfo(0)
  ]);
  assert.equal(calls, 1);
  assert.ok(results.every(result => result.progressiveStreamUrl));
});

test('the upstream-selected quality can reuse its already warm decoder', async () => {
  const c = harness();
  const slot = videoSlot();
  c.feedSlots = [slot];
  c.apiCall = () => Promise.resolve({ ...playback(), quality: '540p' });
  await c.prepareFeedSlot(slot, 0, true);
  const loads = slot.video.loadCount;
  await c.prepareFeedSlot(slot, 0, true);
  assert.equal(slot.quality, '540p');
  assert.equal(slot.video.loadCount, loads);
});

test('a failed new source retries against the newly allocated slot generation', async () => {
  const c = harness();
  const timers = [];
  c.setTimeout = callback => { timers.push(callback); return timers.length; };
  let calls = 0;
  c.apiCall = () => { calls++; return Promise.resolve(null); };
  const slot = videoSlot();
  c.feedSlots = [slot];
  c.ensureFeedSource(slot, 0);
  // Drain IPC/preparation promises without running the scheduled retry yet.
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(calls, 1);
  assert.equal(timers.length, 1, 'the new load token must not suppress the retry');
  timers.shift()();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(calls, 2);
});

test('expired playback metadata reloads and a reset rejects stale cache writes', async () => {
  const c = harness();
  c.state.feed.preloadMap.set('series-a:1:720p', { ...playback(), expiresAt: Date.now() - 1 });
  let resolvePlayback;
  c.apiCall = () => new Promise(resolve => { resolvePlayback = resolve; });
  const load = c.prefetchFeedPlaybackInfo(0);
  c.feedPlaybackGeneration++;
  c.state.feed.preloadMap.clear();
  resolvePlayback(playback());
  await load;
  assert.equal(c.state.feed.preloadMap.size, 0);
});

test('revisiting a resident frame after metadata expiry preserves its decoder', async () => {
  const c = harness();
  const slot = videoSlot();
  Object.assign(slot, { itemIndex: 0, seriesId: 'series-a', quality: '720p',
    ready: true, queueConsumed: true, streamUrl: 'http://localhost/cached.m3u8', expiresAt: Date.now() - 1 });
  Object.assign(slot.video, { src: slot.streamUrl, readyState: 4, videoWidth: 320 });
  c.feedSlots = [slot];
  c.apiCall = () => { throw new Error('resident playback must not renegotiate'); };
  await c.prepareFeedSlot(slot, 0, true);
  assert.equal(slot.video.loadCount, 0);
  assert.equal(slot.video.playCount, 1);
});

test('a recycled slot reuses its remembered rendition after adaptive quality changes', async () => {
  const c = harness();
  const calls = [];
  c.apiCall = (...args) => { calls.push(args); return Promise.resolve(playback()); };
  const first = videoSlot();
  c.feedSlots = [first];
  await c.prepareFeedSlot(first, 0, true);
  c.feedDesiredQuality = () => '540p';
  const recycled = videoSlot();
  c.feedSlots = [recycled];
  await c.prepareFeedSlot(recycled, 0, true);
  assert.equal(calls.length, 1, 'returning to a warm rendition must not request another quality');
  assert.equal(recycled.quality, '720p');
  c.state.feed.userQuality = '540p';
  c.apiCall = (...args) => { calls.push(args); return Promise.resolve({ ...playback(), quality: '540p' }); };
  await c.prepareFeedSlot(recycled, 0, true);
  assert.equal(calls.length, 2, 'an explicit quality selection still takes effect');
  assert.equal(calls[1][4], '540p');
});

test('a growing HLS prefix is never mistaken for a completely buffered episode', () => {
  const c = harness();
  const slot = videoSlot();
  Object.assign(slot, { hlsStreamUrl: 'http://localhost/live.m3u8', durationSec: 6 });
  Object.assign(slot.video, { src: slot.hlsStreamUrl, duration: 6,
    buffered: { length: 1, start: () => 0, end: () => 6 } });
  c.feedSlots = [slot];
  assert.equal(c.feedSlotIsFullyBuffered(slot), false, 'unknown episode duration must not trust a prefix');
  slot.metadataDurationSec = 60;
  assert.equal(c.feedSlotIsFullyBuffered(slot), false);
  slot.video.buffered.end = () => 60;
  assert.equal(c.feedSlotIsFullyBuffered(slot), true);
  slot.video.buffered = { length: 2, start: i => i ? 10 : 0, end: i => i ? 60 : 6 };
  assert.equal(c.feedSlotIsFullyBuffered(slot), false, 'a gap is not a complete buffer');
});

test('offscreen play remaining pending cannot keep first-frame priming locked', async () => {
  const c = harness();
  const timers = new Map();
  let id = 0;
  c.setTimeout = (fn, delay) => { timers.set(++id, { fn, delay }); return id; };
  c.clearTimeout = id => timers.delete(id);
  c.syncFeedPosterVisibility = () => {};
  const slot = videoSlot();
  slot.video.src = 'http://localhost/pending.m3u8';
  const listeners = new Map();
  slot.video.addEventListener = (event, callback) => listeners.set(event, callback);
  slot.video.removeEventListener = event => listeners.delete(event);
  slot.video.play = () => new Promise(() => {});
  slot.video.pause = () => { slot.video.paused = true; };
  c.feedSlots = [videoSlot(), slot];
  const priming = c.primeFeedSlotFirstFrame(slot);
  let settled = false;
  priming.then(() => { settled = true; });
  const [timerId, timer] = [...timers][0];
  assert.equal(timer.delay, 900);
  timers.delete(timerId);
  timer.fn();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(settled, true, 'frame timeout must also bound the unresolved play promise');
  for (const { fn } of [...timers.values()]) fn();
  assert.equal(slot.isPriming, false);
  assert.equal(slot.video.paused, true);
  assert.equal(listeners.size, 0);
});

test('promotion during first-frame priming keeps foreground playback running', async () => {
  const c = harness();
  c.syncFeedPosterVisibility = () => {};
  const slot = videoSlot();
  const listeners = new Map();
  slot.video.src = 'http://localhost/priming.m3u8';
  slot.video.addEventListener = (event, callback) => listeners.set(event, callback);
  slot.video.removeEventListener = event => listeners.delete(event);
  let pauses = 0;
  slot.video.pause = () => { pauses++; };
  c.feedSlots = [videoSlot(), slot];
  const priming = c.primeFeedSlotFirstFrame(slot);
  c.feedSlotCurr = 1;
  slot.video.readyState = 3;
  slot.video.videoWidth = 320;
  listeners.get('loadeddata')();
  await priming;
  assert.equal(pauses, 0);
  assert.equal(slot.isPriming, false);
});

test('buffer bar grows against episode duration while the HLS playlist grows', () => {
  const c = harness();
  const slot = videoSlot();
  slot.durationSec = 60;
  c.feedSlots = [slot];
  const indicator = { style: {} };
  for (const bufferedSeconds of [2, 6, 12]) {
    slot.video.duration = bufferedSeconds;
    slot.video.buffered = { length: 1, start: () => 0, end: () => bufferedSeconds };
    const duration = c.feedMediaDuration(slot.video);
    assert.equal(duration, 60);
    c.updatePlaybackBufferIndicator(slot.video, indicator, duration);
    assert.ok(Math.abs(parseFloat(indicator.style.width) - bufferedSeconds / 60 * 100) < 0.001);
    assert.notEqual(indicator.style.width, '100%');
  }
});
