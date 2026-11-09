const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
function host() {
  const calls = [];
  const listeners = {nextName: 0};
  const elements = {};
  const context = {
    window: { innerWidth: 1000, addEventListener(name, callback) { listeners[name] = callback; }, myguoNativeMedia: true, mygo: { call(method, ...args) { calls.push([method, ...args]); return Promise.resolve(); } } },
    document: { getElementById(id) { return elements[id] || null; }, addEventListener(name, callback) { listeners[name] = callback; }, body: { classList: { add() {} } } },
    MutationObserver: class { constructor(callback) { listeners[`observer${listeners.nextName++}`] = callback; } observe() {} },
    getComputedStyle(node) { return {display: node.display || 'flex', visibility: node.visibility || 'visible'}; },
    localStorage: { getItem() { return null; } },
    switchView(view) {
      calls.push(['view', view]);
      context.state.currentView = view;
    },
    closePlayer() { calls.push(['close']); context.window.nativeMediaClosed?.(); },
    enterSeriesPlayer(id, episode) { calls.push(['play', id, episode]); return Promise.resolve(); },
    ensureRecommendColdStart(channel) {
      calls.push(['cold-start', channel]);
      if (channel && channel !== context.state.feed.channel) {
        context.state.feed.channel = channel;
        context.state.feed.items = [];
      }
      return Promise.resolve();
    },
    getFeedVideo() { return null; },
    loadSeriesDetail(id) { calls.push(['detail-load', id]); },
    handleMediaCommand(payload) { calls.push(['command', payload.command, payload.position]); },
    changePlaybackQuality(quality) { calls.push(['quality', quality]); },
    playEpisodeNumber(episode) { calls.push(['episode', episode]); },
    qualityLabel(value) { return String(value || '720p').toUpperCase(); },
    showToast(message) { calls.push(['error', String(message)]); },
    state: {
      currentView: 'native-idle',
      detail: {},
      feed: { channel: '', items: [] },
      player: { quality: '720p', qualityOptions: [], playbackSpeed: 1.25, episodeNum: 4, totalEpisodes: 12 }
    },
    video: { playbackRate: 1 },
    console,
  };
  vm.createContext(context);
  vm.runInContext(fs.readFileSync('frontend/native-media.js', 'utf8'), context);
  return { context, calls, elements, listeners, send: context.window.nativeMediaEvent, ready: () => listeners.DOMContentLoaded() };
}
test('native host announces readiness only after its scene handler is installed', () => {
  const h = host();
  assert.equal(typeof h.send, 'function');
  assert.equal(h.calls.length, 0);
  h.ready();
  assert.deepEqual(h.calls, [['NativeShell.MediaReady']]);
});

test('media host keeps runtime state and resume bridge required by playback', () => {
  const source = fs.readFileSync('frontend/app.js', 'utf8');
  for (const declaration of [
    'let feedNextDramaTimer = null',
    'let feedPlayPromise = null',
    'let feedPlaybackGeneration = 0',
    'function getFeedVideo()',
    "window.nativeMediaNotify?.('ResumeProgress', {",
    'episodeNum: state.feed.episodeNum,',
    'positionSec: feedVideo.currentTime,',
    'session,',
    'sequence: resumeProgressSequence'
  ]) {
    assert.ok(source.includes(declaration), `${declaration} must remain in the media host`);
  }
  const bridge = fs.readFileSync('frontend/native-media.js', 'utf8');
  assert.ok(bridge.includes('window.nativeMediaSession = () => Number(activeSession);'));
});

test('resume bridge advances sequence within a native playback session', async () => {
  const calls = [];
  const context = {
    window: {
      nativeMediaSession: () => 7,
      nativeMediaNotify: (method, payload) => calls.push({method, payload})
    },
    state: {feed: {seriesId: '123456', episodeNum: 4}},
    apiCall: async () => {},
    getFeedVideo: () => ({currentTime: 12.5}),
    feedMediaDuration: () => 90,
    console
  };
  vm.runInContext('let resumeProgressSession = null; let resumeProgressSequence = 0;', vm.createContext(context));
  vm.runInContext(fs.readFileSync('frontend/app.js', 'utf8').match(/function persistFeedProgress\(\) \{[\s\S]*?\n\}/)[0], vm.createContext(context));
  context.persistFeedProgress();
  context.persistFeedProgress();
  context.persistFeedProgress();
  assert.deepEqual(calls.map(call => call.payload.sequence), [1, 2, 3]);
  assert.ok(calls.every(call => call.payload.session === 7));
});
test('native scenes stop outgoing media before selecting feed or settings', () => {
  const h = host();
  h.send({event:'recommend'});
  assert.deepEqual(
    h.calls.filter(call => call[0] === 'view').map(call=>call[0]+':'+(call[1]||'')),
    ['view:native-idle','view:recommend']
  );
  h.send({event:'settings'});
  assert.equal(h.calls.at(-1)[0], 'view');
  assert.equal(h.calls.at(-1)[1], 'settings');
  assert.equal(h.calls.some(call=>call[0]==='NativeShell.Closed'), false, 'internal transitions must not close native detail');
});
test('settings remains a hosted scene and returns to a replaced feed', () => {
  const h = host();
  h.send({event:'settings'});
  assert.deepEqual(h.calls.map(call=>call[0]+':'+(call[1]||'')), ['view:native-idle','view:settings']);
  assert.equal(h.context.state.currentView, 'settings');
  h.send({event:'recommend', channel:'真人剧'});
  assert.deepEqual(
    h.calls.filter(call => call[0] === 'view').map(call=>call[0]+':'+(call[1]||'')),
    ['view:native-idle', 'view:settings', 'view:native-idle', 'view:recommend']
  );
  assert.equal(h.context.state.feed.channel, '真人剧');
  assert.equal(h.context.state.currentView, 'recommend');
});
test('native playback enters the shared recommend player', () => {
  const h = host();
  h.send({event:'play',seriesId:'123456',episodeNum:4,resumeAt:42,session:7});
  assert.deepEqual(h.calls, [['view','recommend'], ['play','123456',4]]);
  h.send({event:'command',command:'seek',position:12.5});
  assert.deepEqual(h.calls.at(-1), ['command','seek',12.5]);
  h.context.window.nativeMediaClosed();
  assert.deepEqual(h.calls.at(-1), ['NativeShell.ClosePlayer',7]);
});

test('stop cannot resume a hidden recommendation and suppresses recursive close notifications', () => {
  const h = host();
  h.send({event:'stop'});
  assert.deepEqual(h.calls, [['view','native-idle']]);
  assert.equal(h.calls.some(call=>call[0]==='play'), false);
});
test('embedded host uses the original media control modules and native shell bridges', () => {
  const html = fs.readFileSync('frontend/index.html','utf8');
  for (const name of ['ambient-glow.js','player-chrome.js','danmaku.js','progressive-media.js','app.js','native-media.js']) assert(html.includes(`src="${name}"`), name);
  const controls = fs.readFileSync('frontend/player-chrome.js','utf8');
  for (const control of ['quality','speed','danmaku','danmaku-settings','drawer-toggle','pip','fullscreen','like','comment','fav']) assert(controls.includes(`id('${control}')`), control);
  const app = fs.readFileSync('frontend/app.js','utf8');
  assert(app.includes('window.nativeMediaNotify?.("LibraryChanged")'));
});

test('native channel requests replace old feed items and survive a scene return', () => {
  const h = host();
  h.context.state.feed.items = ['old drama'];
  h.send({event:'recommend',channel:'漫剧'});
  assert.equal(h.context.state.feed.channel, '漫剧');
  assert.equal(h.context.state.feed.items.length, 0);
  h.context.state.feed.items = ['current drama'];
  h.send({event:'settings'});
  h.send({event:'recommend',channel:'漫剧'});
  assert.equal(h.context.state.feed.items[0], 'current drama', 'same-channel return should resume the current queue');
});
test('native channel visibility follows comments, clear screen and fullscreen without per-frame IPC', () => {
  const h = host();
  const classes = new Set();
  const node = () => ({classList:{contains(value) {return classes.has(value);}}, style:{display:'none', visibility:'visible'}});
  h.elements['feed-container'] = node();
  h.elements['view-recommend'] = node();
  h.elements['player-modal'] = node();
  h.elements['feed-cat-bar'] = {...node(), getBoundingClientRect() { return {left:350, width:300}; }};
  h.context.state.currentView = 'recommend';
  h.ready();
  const updates = () => h.calls.filter(call => call[0] === 'NativeShell.FeedChromeChanged');
  assert.equal(updates().at(-1)[1].visible, true);
  const count = updates().length;
  h.listeners.observer0();
  assert.equal(updates().length, count, 'unchanged chrome must not send another IPC update');
  h.elements['feed-cat-bar'].getBoundingClientRect = () => ({left:140,width:300,top:14});
  h.context.window.innerWidth = 900;
  h.listeners.observer0();
  assert.equal(updates().at(-1)[1].rightInset, 320);
  classes.add('clearscreen');
  h.listeners.observer0();
  assert.equal(updates().at(-1)[1].visible, false);
  classes.clear();
  h.listeners.observer0();
  assert.equal(updates().at(-1)[1].visible, true);
  h.context.document.fullscreenElement = {};
  h.listeners.fullscreenchange();
  assert.equal(updates().at(-1)[1].visible, false);
});

test('the 812 episode drawer keeps the complete collection and never substitutes the current episode', () => {
});
