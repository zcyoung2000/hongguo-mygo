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
  const chromeNode = {classList: {contains: () => false}, getBoundingClientRect: () => ({left: 350, width: 300})};
  context.document.getElementById = id => ['feed-cat-bar', 'feed-container', 'view-recommend'].includes(id) ? chromeNode : null;
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
        id, textContent: '', style: {}, value: '', dataset: {}, disabled: false,
        classList: {values: new Set(), add(...names) { names.forEach(name => this.values.add(name)); },
          remove(...names) { names.forEach(name => this.values.delete(name)); },
          toggle(name, force) { force ? this.values.add(name) : this.values.delete(name); },
          contains(name) { return this.values.has(name); }},
        handlers: {}, addEventListener(name, handler) { this.handlers[name] ||= []; this.handlers[name].push(handler); },
        querySelector() { return null; }, appendChild(child) { this.children ||= []; this.children.push(child); },
        focus() {}, scrollTop: 0, clientHeight: 96, offsetHeight: 28, offsetTop: 0, scrollHeight: 200, innerHTML: ''
      });
    }
    return nodes.get(id);
  };
  const video = {src: '', currentTime: 0, duration: 60, paused: false, playbackRate: 1, readyState: 4};
  const runtime = {
    console, setTimeout, clearTimeout, setImmediate,
    document: {getElementById: node, querySelectorAll: () => [], createElement: () => ({style: {}, dataset: {}, classList: {add() {}, remove() {}, toggle() {}, contains() { return false; }}, appendChild() {}})},
    window: {}, state: {currentView: 'recommend', feed: {
      index: 0, seriesId: '710000000000001', episodeNum: 2, episodeId: '710000000000102',
      totalEpisodes: 12, episodes: Array.from({length: 12}, (_, i) => ({id: `e${i + 1}`, number: i + 1})),
      quality: '720p', qualityOptions: [], playbackSpeed: 1, preloadMap: new Map(), playbackInfoLoads: new Map(),
      userPaused: false
    }},
    feedSlots: [{id: 0, video, ready: true, episodeNum: 2, quality: '720p'}], feedSlotCurr: 0,
    feedWarmupToken: 0, feedWarmupCenterKey: '', feedWarmupSession: null, feedPreloadRevision: 0,
    feedPlaybackGeneration: 0, getFeedVideo: () => video, __api: async (method, ...args) => {
      calls.push([method, ...args]);
      if (method === 'PlayEpisode') return {episodeId: args[2] === 'e4' ? 'e4' : 'e4', episodeNum: args[2] === 'e4' ? 4 : 4, quality: args[3], streamUrl: `local-${args[3]}`, durationSec: 60};
      return {};
    },
  };
  runtime.apiCall = runtime.__api;
  vm.createContext(runtime);
  vm.runInContext('var playFeedIndex = async (...args) => calls.push(["playFeedIndex", ...args]);', runtime);
  vm.runInContext('var calls=[];', runtime);
  return {runtime, calls, node, video};
}

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
