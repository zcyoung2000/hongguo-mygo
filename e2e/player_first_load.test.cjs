const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const hostSource = fs.readFileSync(path.join(__dirname, '../frontend/native-media.js'), 'utf8');
const appSource = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');

function loadFunction(context, name, source = appSource) {
  const match = source.match(new RegExp('(?:async )?function ' + name + '\\b'));
  assert.ok(match, `missing function ${name}`);
  const brace = source.indexOf('{', match.index + match[0].length + 2);
  let depth = 0;
  let end = -1;
  for (let index = brace; index < source.length; index++) {
    if (source[index] === '{') depth++;
    else if (source[index] === '}') {
      depth--;
      if (depth === 0) {
        end = index + 1;
        break;
      }
    }
  }
  assert.notEqual(end, -1, `unterminated function ${name}`);
  vm.runInContext(source.slice(match.index, end), context);
}

test('entering the native recommend stream immediately starts the first load', async () => {
  const calls = [];
  const listeners = {};
  const elements = {};
  const runtime = {
    window: {
      innerWidth: 1000,
      myguoNativeMedia: true,
      mygo: {call: async () => {}},
      addEventListener(name, callback) { listeners[name] = callback; }
    },
    document: {
      getElementById: id => elements[id] || null,
      addEventListener(name, callback) { listeners[name] = callback; },
      body: {classList: {add() {}}}
    },
    MutationObserver: class {
      constructor(callback) { listeners.observer = callback; }
      observe() {}
    },
    getComputedStyle: () => ({display: 'flex'}),
    localStorage: {getItem: () => null},
    console,
    state: {currentView: 'native-idle', feed: {channel: '', items: ['stale']}},
    switchView: view => {
      calls.push(['switchView', view]);
      context.state.currentView = view;
    },
    ensureRecommendColdStart: async channel => {
      calls.push(['ensureRecommendColdStart', channel]);
      assert.equal(context.state.currentView, 'recommend');
    },
    showToast: message => calls.push(['showToast', message]),
    enterSeriesPlayer: async () => calls.push(['enterSeriesPlayer'])
  };
  const context = vm.createContext(runtime);
  runtime.context = context;
  vm.runInContext(hostSource, context);
  listeners.DOMContentLoaded();
  calls.length = 0;
  context.window.nativeMediaEvent({event: 'recommend', channel: '真人剧'});
  await new Promise(resolve => setImmediate(resolve));

  assert.deepEqual(calls, [
    ['switchView', 'native-idle'],
    ['switchView', 'recommend'],
    ['ensureRecommendColdStart', '真人剧']
  ]);
});

function node(id) {
  const element = {
    id,
    classList: {
      values: new Set(),
      add(...names) { names.forEach(name => this.values.add(name)); },
      remove(...names) { names.forEach(name => this.values.delete(name)); },
      toggle(name, force) { force ? this.values.add(name) : this.values.delete(name); },
      contains(name) { return this.values.has(name); }
    },
    style: {},
    attributes: {},
    dataset: {},
    handlers: {},
    children: [],
    innerHTML: '',
    scrollTop: 0,
    clientHeight: 96,
    offsetHeight: 28,
    offsetTop: 0,
    appendChild(child) { this.children.push(child); },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    addEventListener(name, callback) { this.handlers[name] = callback; },
    setAttribute(name, value) { this.attributes[name] = String(value); }
  };
  return element;
}

function drawerHarness() {
  const nodes = new Map();
  const get = id => {
    if (!nodes.has(id)) nodes.set(id, node(id));
    return nodes.get(id);
  };
  const calls = [];
  const runtime = {
    document: {
      getElementById: get,
      addEventListener() {},
      querySelector: () => null,
      createElement: () => ({className: '', textContent: ''})
    },
    console,
    state: {
      currentView: 'recommend',
      feed: {
        channel: '',
        index: 0,
        seriesId: 'series-a',
        episodeNum: 1,
        totalEpisodes: 0,
        episodes: [],
        items: [{id: 'series-a'}]
      }
    },
    feedSlots: [{
      id: 0,
      video: {
        src: '', readyState: 0, ended: false, error: null, paused: true,
        addEventListener() {}
      },
      failed: true
    }],
    performance: {now: () => 0},
    setTimeout: () => 1,
    clearTimeout: () => {},
    Date,
    feedPlayToken: 0,
    feedPlayPromise: null,
    feedNeedsGestureUnmute: false,
    attachFeedGestureUnlock: () => {},
    waitForMygoBridge: async () => ({call: async () => {}}),
    showFeedHint: () => {},
    requestAnimationFrame: () => 0,
    cancelAnimationFrame: () => {},
    setInterval: () => 0,
    clearInterval: () => {},
    window: {location: {href: 'http://localhost/'}, addEventListener() {}},
    HTMLMediaElement: {HAVE_CURRENT_DATA: 2, HAVE_FUTURE_DATA: 3, HAVE_ENOUGH_DATA: 4},
    feedSlotCurr: 0,
    feedSlotForVideo: () => runtime.feedSlots[0],
    getFeedVideo: () => runtime.feedSlots[0].video,
    syncFeedScrollCapture: () => calls.push(['sync']),
    renderFeedPlaylistDrawer: () => calls.push(['render']),
    scrollActiveDrawerEpisode: () => calls.push(['scroll']),
    showToast: message => calls.push(['toast', message])
  };
  const context = vm.createContext(runtime);
  runtime.context = context;
  for (const name of [
    'renderEpisodeGrid', 'toggleFeedPlaylistDrawer', 'closeFeedPlaylistDrawer',
    'updateFeedPlayIndicator', 'feedPlaybackFailureState', 'clearFeedFailureHint', 'attachFeedLoadFailureRetry',
    'clearFeedSourceRetry', 'clearFeedFirstFrameWatchdog', 'scheduleFeedSourceRetry'
  ]) {
    loadFunction(context, name);
  }
  context.playerSpeedLabel = speed => `${Number(speed).toFixed(1)}x`;
  return {context, get, calls};
}

test('an empty episode drawer stays responsive and closes from every escape path', () => {
  const {context, get, calls} = drawerHarness();
  const drawer = get('feed-playlist-drawer');
  const toggle = get('feed-drawer-toggle');
  const close = get('feed-close-drawer');

  context.toggleFeedPlaylistDrawer({
    drawer,
    drawerToggle: toggle,
    drawerGrid: get('feed-drawer-grid'),
    drawerCount: get('feed-drawer-count')
  });
  drawer.addEventListener('click', event => {
    if (event.target === drawer) context.closeFeedPlaylistDrawer({drawer, drawerToggle: toggle});
  });
  assert.equal(drawer.classList.contains('open'), true);
  assert.equal(toggle.attributes['aria-expanded'], 'true');
  assert.equal(get('feed-drawer-count').textContent, '共 0 集');
  assert.equal(get('feed-drawer-grid').children.length, 1);
  assert.equal(get('feed-drawer-grid').children[0].className, 'drawer-empty-state');
  assert.match(get('feed-drawer-grid').children[0].textContent, /剧集信息/);
  assert.deepEqual(calls, [['sync'], ['scroll']]);

  context.closeFeedPlaylistDrawer({drawer, drawerToggle: toggle});
  assert.equal(drawer.classList.contains('open'), false);
  assert.equal(toggle.attributes['aria-expanded'], 'false');

  context.toggleFeedPlaylistDrawer({
    drawer,
    drawerToggle: toggle,
    drawerGrid: get('feed-drawer-grid'),
    drawerCount: get('feed-drawer-count')
  });
  drawer.handlers.click?.({target: drawer});
  assert.equal(drawer.classList.contains('open'), false);
  assert.equal(toggle.attributes['aria-expanded'], 'false');
});

test('a terminal first-source failure exposes and activates a visible retry entry', async () => {
  const {context, get, calls} = drawerHarness();
  loadFunction(context, 'prepareFeedSlot');
  loadFunction(context, 'feedSlotMatches');
  loadFunction(context, 'normalizeFeedMediaURL');
  const slot = context.feedSlots[0];
  const video = slot.video;
  slot.loadToken = 0;
  video.dataset = {};
    video.play = () => Promise.resolve();
    video.pause = () => {};
    video.removeAttribute = () => {};
    video.load = () => {};
    video.addEventListener = () => {};
  let attempts = 0;
  context.prefetchFeedPlaybackInfo = () => {
    attempts++;
    return Promise.reject(new Error('IPC unavailable'));
  };
  context.safeImageCSS = () => 'none';
  context.scheduleFeedFrameMetric = () => {};
  context.scheduleFeedAutoplayRetry = () => {};
  context.scheduleFeedWarmup = () => {};
  context.assessFeedMediaCapabilities = () => {};
  context.syncActiveFeedSlot = () => {};
  context.requestFeedDramaMetadata = () => {};
  context.ensureFeedItemsAhead = () => {};
  context.clearFeedFailureHint = () => {};
  context.showFeedSkeleton = () => {};
  context.recordFeedDiagnostic = () => {};
  context.reportFeedFailure = (failedSlot, text) => {
    failedSlot.failed = true;
    calls.push(['failure', text]);
  };
  context.ensureFeedSource = () => calls.push(['retry']);
  context.feedSlots = context.feedSlots;
  context.feedAssetKey = (seriesId, episodeNum, quality) => `${seriesId}:${episodeNum}:${quality}`;
  context.feedQualityForItem = () => '720p';
  context.recordFeedDiagnostic = () => {};
  context.clearFeedSlot = cleared => {
    cleared.loadToken++;
    cleared.ready = false;
    cleared.video.src = '';
    calls.push(['clear']);
  };

  await context.prepareFeedSlot(slot, 0, true, null, false, '720p', 1, '', 0);
  assert.equal(slot.failed, true);
  assert.equal(calls.find(call => call[0] === 'failure')?.[1], '获取播放流失败，正在重试');
  assert.equal(typeof slot.sourceRetryTimer, 'number');
  assert.equal(attempts, 1, 'first source failure must schedule a bounded retry');

  context.updateFeedPlayIndicator();
  const stage = get('feed-stage');
  assert.equal(stage.classList.contains('is-load-failed'), true);
  assert.equal(get('feed-loader').title, '加载失败，点击重试');
  const loader = get('feed-loader');
  const feedStage = get('feed-stage');
  const feedVideo = context.getFeedVideo();
  context.state.feed.consecutiveSourceFailures = 0;
  context.attachFeedLoadFailureRetry({loader, stage: feedStage});
  loader.handlers.click();
  assert.deepEqual(calls.filter(call => call[0] === 'retry'), [['retry']]);
});

test('repeated recommend cold starts reuse one in-flight reset', async () => {
  const calls = [];
  const listeners = {};
  const runtime = {
    console,
    feedRequestToken: 0,
    feedColdStartPromise: null,
    feedColdStartRequestToken: 0,
    state: {currentView: 'recommend', feed: {channel: '推荐', items: [], loading: false}},
    syncFeedScrollCapture: () => calls.push(['sync']),
    resumeFeedPlayback: () => calls.push(['resume']),
    loadFeed: (reset, channel) => {
      calls.push(['loadFeed', reset, channel]);
      return new Promise(resolve => { runtime.resolveColdStart = resolve; });
    }
  };
  const context = vm.createContext(runtime);
  for (const name of ['ensureRecommendColdStart']) loadFunction(context, name);
  const first = context.ensureRecommendColdStart('推荐');
  const second = context.ensureRecommendColdStart('推荐');
  assert.equal(first, second, 'same-channel duplicate events must reuse the in-flight reset');
  assert.deepEqual(calls.filter(call => call[0] === 'loadFeed'), [['loadFeed', true, '推荐']]);

  runtime.resolveColdStart();
  await first;
  assert.equal(runtime.feedColdStartPromise, null, 'finished cold start must release its owner');

  runtime.state.feed.items = [{id: 'series-a'}];
  await context.ensureRecommendColdStart('推荐');
  assert.deepEqual(calls.at(-1), ['resume'], 'existing same-channel content must resume without reset');
});

test('source failures retry at most three times before visible failure', async () => {
  const {context, get, calls} = drawerHarness();
  loadFunction(context, 'prepareFeedSlot');
  loadFunction(context, 'feedSlotMatches');
  loadFunction(context, 'normalizeFeedMediaURL');
  const slot = context.feedSlots[0];
  slot.loadToken = 0;
  slot.video.dataset = {};
  slot.video.play = () => Promise.resolve();
  slot.video.pause = () => {};
  slot.video.removeAttribute = () => {};
  slot.video.load = () => {};
  slot.video.addEventListener = () => {};
  let attempts = 0;
  const timers = new Map();
  let timerId = 0;
  Object.assign(context, {
    prefetchFeedPlaybackInfo: () => {
      attempts++;
      return Promise.resolve(null);
    },
    safeImageCSS: () => 'none',
    scheduleFeedFrameMetric: () => {},
    scheduleFeedAutoplayRetry: () => {},
    scheduleFeedWarmup: () => {},
    assessFeedMediaCapabilities: () => {},
    syncActiveFeedSlot: () => {},
    requestFeedDramaMetadata: () => {},
    ensureFeedItemsAhead: () => {},
    clearFeedFailureHint: () => {},
    showFeedSkeleton: () => {},
    recordFeedDiagnostic: () => {},
    reportFeedFailure: (failedSlot, text) => {
      failedSlot.failed = true;
      calls.push(['failure', text]);
    },
    ensureFeedSource: (failedSlot, itemIndex, attempt) => {
      calls.push(['retry', attempt]);
      return failedSlot.preparing = context.prepareFeedSlot(
        failedSlot, itemIndex, true, null, false, null, 1, '', attempt
      );
    },
    feedAssetKey: (seriesId, episodeNum, quality) => `${seriesId}:${episodeNum}:${quality}`,
    feedQualityForItem: () => '720p',
    clearFeedSlot: cleared => {
      cleared.loadToken = (cleared.loadToken || 0) + 1;
      cleared.ready = false;
      cleared.video.src = '';
    },
    setTimeout: callback => {
      const id = ++timerId;
      timers.set(id, callback);
      return id;
    },
    clearTimeout: id => timers.delete(id)
  });

  await context.prepareFeedSlot(slot, 0, true, null, false, '720p', 1, '', 0);
  for (let attempt = 1; attempt <= 3; attempt++) {
    const timer = [...timers.values()].at(-1);
    timer();
    await new Promise(resolve => setImmediate(resolve));
  }
  assert.equal(attempts, 4, 'one initial load plus three retries before terminal state');
  assert.equal(slot.failed, true);
  assert.equal(slot.preparing, null);
  assert.deepEqual(calls.filter(call => call[0] === 'failure').map(call => call[1]), [
    '获取播放流失败，正在重试',
    '获取播放流失败，正在重试',
    '获取播放流失败，正在重试',
    '获取播放流失败，正在重试',
    '视频加载失败，请点击重试'
  ]);
  context.updateFeedPlayIndicator();
  assert.equal(get('feed-stage').classList.contains('is-load-failed'), true);
});

test('a first-frame timeout walks the progressive HLS full fallback chain', () => {
  const {context, calls} = drawerHarness();
  loadFunction(context, 'normalizeFeedMediaURL');
  loadFunction(context, 'assignFeedMediaSource');
  loadFunction(context, 'scheduleFeedFirstFrameWatchdog');
  const slot = context.feedSlots[0];
  slot.loadToken = 7;
  slot.itemIndex = 0;
  slot.streamUrl = 'http://localhost/progressive';
  slot.progressiveStreamUrl = 'http://localhost/progressive';
  slot.hlsStreamUrl = 'http://localhost/hls';
  slot.fullStreamUrl = 'http://localhost/full';
  slot.usedProgressiveFallback = true;
  slot.video.error = null;
  const timers = [];
  let timerId = 0;
  Object.assign(context, {
    feedVideoHasFirstFrame: () => false,
    recordFeedDiagnostic: event => calls.push(['diagnostic', event]),
    beginFeedPlayback: () => calls.push(['play']),
    scheduleFeedAutoplayRetry: () => {},
    assignFeedMediaSource: (target, source, progressive) => calls.push(['source', source, progressive]),
    setTimeout: callback => {
      timers.push(callback);
      return ++timerId;
    }
  });

  context.scheduleFeedFirstFrameWatchdog(slot, 0, 7, 0);
  timers.at(-1)();
  assert.deepEqual(calls.filter(call => call[0] === 'diagnostic'), [['diagnostic', 'first-frame-timeout']]);
  assert.deepEqual(calls.filter(call => call[0] === 'source'), [
    ['source', 'http://localhost/hls', false]
  ]);
  assert.equal(slot.streamUrl, 'http://localhost/hls');
  assert.equal(slot.usedHLSFallback, true);

  timers.at(-1)();
  assert.deepEqual(calls.filter(call => call[0] === 'source'), [
    ['source', 'http://localhost/hls', false],
    ['source', 'http://localhost/full', false]
  ]);
  assert.equal(slot.usedFullFallback, true);
});
