const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const appSource = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const hostSource = fs.readFileSync(path.join(__dirname, '../frontend/native-media.js'), 'utf8');

function loadFunction(context, name, source = appSource) {
  const match = source.match(new RegExp('(?:async )?function ' + name + '\\b'));
  assert.ok(match, `missing function ${name}`);
  const start = match.index;
  const brace = source.indexOf('{', match.index + match[0].length + 2);
  let depth = 0, end = -1;
  for (let index = brace; index < source.length; index++) {
    if (source[index] === '{') depth++;
    else if (source[index] === '}') {
      depth--;
      if (depth === 0) { end = index + 1; break; }
    }
  }
  assert.notEqual(end, -1, `unterminated function ${name}`);
  vm.runInContext(source.slice(start, end), context);
}

function initializeNativeHost(context, runtimeCalls = []) {
  const listeners = {};
  context.document.addEventListener = (name, handler) => { listeners[name] = handler; };
  context.document.body = {classList: {add() {}}};
  const chromeNode = {
    classList: {contains: () => false},
    getBoundingClientRect: () => ({left: 350, width: 300}),
    querySelector: () => null,
    querySelectorAll: () => [],
    addEventListener: () => {}
  };
  const prevGetElementById = context.document.getElementById?.bind(context.document);
  context.document.getElementById = id => ['feed-cat-bar', 'feed-container', 'view-recommend'].includes(id) ? chromeNode : (prevGetElementById?.(id) || null);
  context.window.addEventListener = (name, handler) => { listeners[name] = handler; };
  context.notify = async () => {};
  context.enterSeriesPlayer = async (...args) => { runtimeCalls.push(['enterSeriesPlayer', ...args]); };
  context.showToast = message => { runtimeCalls.push(['toast', String(message)]); };
  context.cancelScheduledFeedWarmup = () => {};  context.MutationObserver = class { constructor(callback) { listeners.observer = callback; } observe() {} };
  context.getComputedStyle = () => ({display: 'flex'});
  context.localStorage = {getItem: () => null};
  context.window.myguoNativeMedia = true;
  context.window.mygo = {call: async () => {}};
  context.notify = async () => {};
  context.enterSeriesPlayer = async (...args) => { runtimeCalls.push(['enterSeriesPlayer', ...args]); };
  context.showToast = message => { runtimeCalls.push(['toast', String(message)]); };
  context.cancelScheduledFeedWarmup = () => {};  vm.runInContext(hostSource, context);
  listeners.DOMContentLoaded?.();
  return context;
}

function playerHarness() {
  const calls = [];
  const nodes = new Map();
  const node = id => {
    if (!nodes.has(id)) {
      nodes.set(id, {
        id, textContent: '', style: {}, value: '', dataset: {}, disabled: false, attributes: {},
        classList: {values: new Set(), add(...names) { names.forEach(name => this.values.add(name)); },
          remove(...names) { names.forEach(name => this.values.delete(name)); },
          toggle(name, force) { force ? this.values.add(name) : this.values.delete(name); },
          contains(name) { return this.values.has(name); }},
        handlers: {}, addEventListener(name, handler) { this.handlers[name] ||= []; this.handlers[name].push(handler); },
        setAttribute(name, value) { this.attributes[name] = String(value); },
        getAttribute(name) { return this.attributes[name] || null; },
        querySelector() { return null; },
        querySelectorAll(selector) {
          if (this.id === 'feed-speed-menu' && selector === '.speed-opt') return this.speedOptions || [];
          return [];
        },
        appendChild(child) { this.children ||= []; this.children.push(child); },
        contains(node) { return node === this; },
        focus() {}, scrollTop: 0, clientHeight: 96, offsetHeight: 28, offsetTop: 0, scrollHeight: 200, innerHTML: ''
      });
    }
    return nodes.get(id);
  };
  const video = {
    src: '', currentTime: 0, duration: 60, paused: false, playbackRate: 1, readyState: 4, dataset: {},
    addEventListener() {}, removeAttribute() {}, load() {}, pause() {}
  };
  const runtime = {
    console, setTimeout, clearTimeout, setImmediate,
    document: {
      getElementById: node,
      querySelectorAll: () => [],
      querySelector: () => null,
      addEventListener(name, handler) { this.handlers ||= {}; this.handlers[name] = handler; },
      createElement: () => ({
        style: {}, dataset: {}, className: '', textContent: '',
        classList: {values: new Set(), add(...names) { names.forEach(name => this.values.add(name)); },
          remove(...names) { names.forEach(name => this.values.delete(name)); },
          toggle(name, force) { force ? this.values.add(name) : this.values.delete(name); },
          contains() { return false; }},
        handlers: {}, addEventListener(name, handler) { this.handlers[name] = handler; },
        appendChild() {}
      })
    },
    window: {addEventListener() {}}, state: {currentView: 'recommend', feed: {
      index: 0, seriesId: '710000000000001', episodeNum: 2, episodeId: '710000000000102',
      totalEpisodes: 12, episodes: Array.from({length: 12}, (_, i) => ({id: `e${i + 1}`, number: i + 1})),
      quality: '720p', qualityOptions: [], playbackSpeed: 1, preloadMap: new Map(), playbackInfoLoads: new Map(),
      userPaused: false
    }},
    requestAnimationFrame: callback => callback(0),
    feedSlots: [{id: 0, video, ready: true, episodeNum: 2, quality: '720p'}], feedSlotCurr: 0,
    feedWarmupToken: 0, feedWarmupCenterKey: '', feedWarmupSession: null, feedPreloadRevision: 0,
    feedPlaybackGeneration: 0, getFeedVideo: () => video, __api: async (method, ...args) => {
      calls.push([method, ...args]);
      if (method === 'PlayEpisode') return {episodeId: args[2] === 'e4' ? 'e4' : 'e4', episodeNum: args[2] === 'e4' ? 4 : 4, quality: args[3], streamUrl: `local-${args[3]}`, durationSec: 60};
      return {};
    },
    performance: {now: () => Date.now()},
    waitForMygoBridge: async () => ({call: async () => {}}),
    setInterval: () => 0,
    clearInterval: () => {},
    attachFeedLoadFailureRetry: () => {},
    toggleFeedPlaylistDrawer: () => {},
    scheduleFeedSourceRetry: () => {},
  };
  runtime.apiCall = runtime.__api;
  vm.createContext(runtime);
  vm.runInContext('var playFeedIndex = async (...args) => calls.push(["playFeedIndex", ...args]);', runtime);
  vm.runInContext('var calls=[];', runtime);
  return {runtime, calls, node, video};
}

test('episode selection updates the info badge immediately and after playback resolves', async () => {
  const h = playerHarness();
  loadFunction(h.runtime, 'activateFeedDrama');
  loadFunction(h.runtime, 'updateFeedDramaInfo');
  loadFunction(h.runtime, 'syncActiveFeedSlot');
  h.runtime.updateFeedCommentCount = () => {};
  const drama = {id: '710000000000001', title: '同步集数剧'};
  const slot = h.runtime.feedSlots[0];
  h.runtime.feedVideoHasFirstFrame = () => false;
  h.runtime.clearFeedSlot = () => {};
  h.runtime.feedAssetKey = (seriesId, episodeNum, quality) => `${seriesId}:${episodeNum}:${quality}`;
  h.runtime.feedQualityForItem = () => '720p';
  h.runtime.feedDesiredQuality = () => '720p';
  h.runtime.feedPlaybackKey = (seriesId, episodeNum, quality) => `${seriesId}:${episodeNum}:${quality}`;
  h.runtime.state.feed.playbackPositions = new Map();
  h.runtime.prefetchFeedPlaybackInfo = () => Promise.resolve({
    episodeId: 'e5', episodeNum: 5, quality: '720p', streamUrl: 'episode-5', durationSec: 68
  });
  h.runtime.rememberFeedQuality = () => {};
  h.runtime.rememberFeedPlayback = () => {};
  h.runtime.assignFeedMediaSource = () => {};
  h.runtime.scheduleFeedFrameMetric = () => {};
  h.runtime.scheduleFeedFirstFrameWatchdog = () => {};
  h.runtime.recordFeedDiagnostic = () => {};
  h.runtime.assessFeedMediaCapabilities = () => {};
  h.runtime.feedVideoBufferAhead = () => 0;
  h.runtime.showFeedSkeleton = () => {};
  h.runtime.beginFeedPlayback = () => {};
  h.runtime.scheduleFeedAutoplayRetry = () => {};
  h.runtime.scheduleFeedWarmup = () => {};
  h.runtime.reportFeedFailure = () => {};
  h.runtime.feedMediaDuration = () => 0;
  h.runtime.updateFeedPlayIndicator = () => {};
  h.runtime.feedPlayToken = 0;
  h.runtime.hasWarmFeedHLS = () => false;
  h.runtime.updateFeedQualityControls = () => {};
  h.runtime.syncFeedMediaChrome = () => {};
  loadFunction(h.runtime, 'normalizeFeedMediaURL');
  h.runtime.ensureFeedItemsAhead = () => {};
  h.runtime.requestFeedDramaMetadata = () => {};
  slot.itemIndex = 0;
  slot.seriesId = drama.id;
  slot.episodeId = 'e1';
  slot.episodeNum = 1;
  slot.quality = '720p';
  h.runtime.state.feed.items = [drama];

  h.runtime.activateFeedDrama(drama, 0, slot);
  assert.equal(h.node('feed-episode-badge').textContent, '第 1 集');

  loadFunction(h.runtime, 'feedSlotMatches');
  loadFunction(h.runtime, 'prepareFeedSlot');
  await h.runtime.prepareFeedSlot(slot, 0, true, null, false, '720p', 5, 'e5');
  assert.equal(h.runtime.state.feed.episodeNum, 5);
  assert.equal(h.node('feed-episode-badge').textContent, '第 5 集');
});

test('episode drawer close works for pointer and Escape interactions', () => {
  const h = playerHarness();
  loadFunction(h.runtime, 'playerSpeedLabel');
  for (const name of ['closeFeedPlaylistDrawer', 'toggleFeedPlaylistDrawer', 'initFeedControls']) {
    loadFunction(h.runtime, name);
  }
  h.runtime.toggleFeedPlaylistDrawer = (elements) => {
    elements.drawer.classList.add('open');
    elements.drawerToggle.setAttribute('aria-expanded', 'true');
    elements.drawerToggle.ariaExpanded = 'true';
  };
  h.runtime.closeFeedPlaylistDrawer = (elements) => {
    elements.drawer.classList.remove('open');
    elements.drawerToggle.setAttribute('aria-expanded', 'false');
    elements.drawerToggle.ariaExpanded = 'false';
  };
  h.runtime.syncFeedScrollCapture = () => {};
  loadFunction(h.runtime, 'shouldCaptureFeedScroll');
  h.runtime.document.addEventListener = (name, handler) => {
    h.runtime.document.handlers ||= {};
    h.runtime.document.handlers[name] ||= [];
    h.runtime.document.handlers[name].push(handler);
  };
  vm.runInContext('initFeedControls()', h.runtime);
  const drawer = h.node('feed-playlist-drawer');
  const toggle = h.node('feed-drawer-toggle');
  const close = h.node('feed-close-drawer');
  const documentHandlers = h.runtime.document.handlers;

  toggle.handlers.click[0]();
  assert.equal(drawer.classList.contains('open'), true);
  close.handlers.pointerup[0]({stopPropagation() {}});
  assert.equal(drawer.classList.contains('open'), false);

  toggle.handlers.click[0]();
  close.handlers.click[0]({stopPropagation() {}});
  assert.equal(drawer.classList.contains('open'), false);

  toggle.handlers.click[0]();
  const keydownHandler = documentHandlers.keydown.find(handler => handler.toString().includes("e.key === 'Escape'"));
  assert.equal(typeof keydownHandler, 'function');
  keydownHandler({target: h.node('feed-stage'), key: 'Escape', code: 'Escape', stopPropagation() {}, preventDefault() {}});
  assert.equal(drawer.classList.contains('open'), false);
});

test('native episode command replays the selected episode in bounds and ignores invalid episodes', async () => {
  const h = playerHarness();
  initializeNativeHost(h.runtime, h.runtime.calls);
  loadFunction(h.runtime, 'changeFeedPlaybackQuality');
  h.runtime.calls.length = 0;
  h.runtime.window.nativeMediaEvent({event: 'episode', value: 4});
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(JSON.parse(JSON.stringify(h.runtime.calls)), [['enterSeriesPlayer', '710000000000001', 4]]);
  h.runtime.window.nativeMediaEvent({event: 'episode', value: 99});
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(h.runtime.calls.length, 1);
});

test('native quality and speed commands change the shared feed player', async () => {
  const h = playerHarness();
  initializeNativeHost(h.runtime, h.runtime.calls);
  loadFunction(h.runtime, 'changeFeedPlaybackQuality');
  h.runtime.window.nativeMediaEvent({event: 'speed', value: 1.5});
  assert.equal(h.runtime.state.feed.playbackSpeed, 1.5);
  assert.equal(h.video.playbackRate, 1.5);
  h.runtime.window.nativeMediaEvent({event: 'speed', value: 3});
  assert.equal(h.runtime.state.feed.playbackSpeed, 1.5, 'out-of-range speed must be ignored');
  h.runtime.window.nativeMediaEvent({event: 'quality', value: '540p'});
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(JSON.parse(JSON.stringify(h.runtime.calls.find(([name]) => name === 'playFeedIndex'))), ['playFeedIndex', 0, 0]);
  assert.equal(h.runtime.state.feed.quality, '540p');
});

test('complete episode drawer renders every episode and marks only the current one', () => {
  const h = playerHarness();
  loadFunction(h.runtime, 'renderEpisodeGrid', appSource, true);
  vm.runInContext('renderEpisodeGrid({grid: document.getElementById("drawer"), count: document.getElementById("count"), total: state.feed.totalEpisodes, current: state.feed.episodeNum, onSelect: n => calls.push(["select", n])})', h.runtime);
  const grid = h.node('drawer');
  assert.equal(grid.children.length, 12);
  assert.equal(grid.children.filter(child => child.className.includes('active')).length, 1);
  assert.equal(grid.children[1].textContent, 2);
  grid.children[3].onclick();
  assert.deepEqual(JSON.parse(JSON.stringify(h.runtime.calls)), [['select', 4]]);
});

test('a prepared episode slot cannot satisfy a different selected episode', async () => {
  const h = playerHarness();
  loadFunction(h.runtime, 'prepareFeedSlot');
  loadFunction(h.runtime, 'feedSlotMatches');
  loadFunction(h.runtime, 'normalizeFeedMediaURL');
  h.runtime.safeImageCSS = () => 'none';
  h.runtime.updateFeedPlayIndicator = () => {};
  h.runtime.feedVideoHasFirstFrame = () => false;
  h.runtime.syncActiveFeedSlot = () => {};
  h.runtime.requestFeedDramaMetadata = () => {};
  h.runtime.clearFeedSlot = slot => {
    slot.loadToken++;
    slot.poster = {style: {}, classList: {remove() {}, add() {}}};
    h.runtime.calls.push(['clearFeedSlot', slot.episodeNum]);
  };
  h.runtime.prefetchFeedPlaybackInfo = () => Promise.reject(new Error('selection should reload'));
  h.runtime.reportFeedFailure = () => {};
  h.runtime.feedQualityForItem = () => '720p';
  h.runtime.feedAssetKey = (seriesId, episodeNum, quality) => `${seriesId}:${episodeNum}:${quality}`;
  const selectedSeriesId = '710000000000001';
  h.runtime.state.feed.items = [{id: selectedSeriesId, title: 'series', coverUrl: ''}];
  const slot = {
    id: 0, video: h.video, ready: true, expiresAt: Date.now() + 1000, itemIndex: 0,
    seriesId: '710000000000001', episodeNum: 2, quality: '720p', requestedQuality: '720p',
    streamUrl: 'episode-2', loadToken: 1
  };
  h.video.src = 'episode-2';
  h.runtime.feedSlots[0] = slot;

  assert.equal(h.runtime.feedSlotMatches(slot, 0, slot.seriesId, '720p', 4), false,
    'episode 4 must not reuse the prepared episode 2 slot');
  await h.runtime.prepareFeedSlot(slot, 0, false, null, false, '720p', 4);
  assert.deepEqual(JSON.parse(JSON.stringify(h.runtime.calls)), [['clearFeedSlot', 2]]);
});

test('local quality menu opens and selection changes playback quality', async () => {
  const h = playerHarness();
  initializeNativeHost(h.runtime, h.runtime.calls);
  loadFunction(h.runtime, 'playerSpeedLabel');
  loadFunction(h.runtime, 'initFeedControls');
  loadFunction(h.runtime, 'renderFeedQualityMenu');
  loadFunction(h.runtime, 'changeFeedPlaybackQuality');
  h.runtime.cancelScheduledFeedWarmup = () => {};
  h.runtime.state.feed.qualityOptions = [
    {id: '540p', label: '540P', available: true, active: false},
    {id: '720p', label: '720P', available: true, active: true}
  ];
  h.runtime.calls.length = 0;
  vm.runInContext('initFeedControls()', h.runtime);
  vm.runInContext('renderFeedQualityMenu()', h.runtime);
  const active = h.runtime.state.feed.qualityOptions.find(option => option.active);
  assert.equal(active.label, '720P', 'quality renderer must receive menu options');
  const qualityBtn = h.node('feed-quality');
  const qualityMenu = h.node('feed-quality-menu');
  qualityMenu.style.display = 'none';
  assert.equal(qualityBtn.textContent, '720P');
  assert.equal(qualityMenu.children.length, 2);

  qualityBtn.handlers.click[0]({stopPropagation() {}});
  assert.equal(qualityMenu.style.display, 'block');
  assert.equal(qualityBtn.attributes['aria-expanded'], 'true');

  const option = qualityMenu.children[0];
  assert.equal(typeof option.handlers.click, 'function', 'available quality option must be clickable');
  option.handlers.click({stopPropagation() {}});
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(qualityMenu.style.display, 'none');
  assert.equal(qualityBtn.attributes['aria-expanded'], 'false');
  assert.equal(h.runtime.state.feed.quality, '540p');
  assert.deepEqual(JSON.parse(JSON.stringify(h.runtime.calls.find(([name]) => name === 'playFeedIndex'))),
    ['playFeedIndex', 0, 0]);
});

test('speed indicator keeps the selected menu text across all fixed formats', () => {
  const h = playerHarness();
  loadFunction(h.runtime, 'playerSpeedLabel');
  h.runtime.state.feed.playbackSpeed = 1.5;
  h.runtime.speedOptions = [
    {
      dataset: {speed: '0.75'}, textContent: '0.75x',
      classList: {remove() {}, add() {}}
    },
    {
      dataset: {speed: '1.0'}, textContent: '1.0x',
      classList: {remove() {}, add() {}}
    },
    {
      dataset: {speed: '1.25'}, textContent: '1.25x',
      classList: {remove() {}, add() {}}
    },
    {
      dataset: {speed: '1.5'}, textContent: '1.5x',
      classList: {remove() {}, add() {}}
    },
    {
      dataset: {speed: '2.0'}, textContent: '2.0x',
      classList: {remove() {}, add() {}}
    }
  ];
  h.runtime.speedOptions.forEach(option => {
    option.addEventListener = (name, handler) => { option.handlers = {[name]: [handler]}; };
  });
  h.node('feed-speed-menu').speedOptions = h.runtime.speedOptions;
  h.runtime.speedOptions.forEach(option => {
    option.handlers = {click: [event => { event.stopPropagation(); option.onclick?.(event); }]};
  });
  loadFunction(h.runtime, 'initFeedControls');
  vm.runInContext('initFeedControls()', h.runtime);
  const speedBtn = h.node('feed-speed');
  assert.equal(speedBtn.textContent, '1.5x');

  h.runtime.speedOptions[4].handlers.click[0]({stopPropagation() {}});
  assert.equal(h.runtime.state.feed.playbackSpeed, 2);
  assert.equal(h.video.playbackRate, 2);
  assert.equal(speedBtn.textContent, '2.0x');

  h.runtime.speedOptions[1].handlers.click[0]({stopPropagation() {}});
  assert.equal(speedBtn.textContent, '1.0x');
  assert.equal(h.runtime.playerSpeedLabel(2.5), '2.5x');
});
