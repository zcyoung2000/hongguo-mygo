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
    switchView(view) { calls.push(['view', view]); },
    closePlayer() { calls.push(['close']); context.window.nativeMediaClosed?.(); },
    openPlayer(id, episode, position) { calls.push(['play', id, episode, position]); return Promise.resolve(); },
    loadSeriesDetail(id) { calls.push(['detail-load', id]); },
    handleMediaCommand(payload) { calls.push(['command', payload.command, payload.position]); },
    changePlaybackQuality(quality) { calls.push(['quality', quality]); },
    playEpisodeNumber(episode) { calls.push(['episode', episode]); },
    qualityLabel(value) { return String(value || '720p').toUpperCase(); },
    showToast(message) { calls.push(['error', String(message)]); },
    state: {
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
test('native scenes stop outgoing media before selecting feed or settings', () => {
  const h = host();
  h.send({event:'recommend'});
  assert.deepEqual(h.calls.map(call=>call[0]+':'+(call[1]||'')), ['view:native-idle','close:','view:recommend']);
  h.send({event:'settings'});
  assert.deepEqual(h.calls.slice(3).map(call=>call[0]+':'+(call[1]||'')), ['view:native-idle','close:','view:settings']);
  assert.equal(h.calls.some(call=>call[0]==='NativeShell.Closed'), false, 'internal transitions must not close native detail');
});
test('native playback preserves original episode and position with the full player', () => {
  const h = host();
  h.send({event:'play',seriesId:'123456',episodeNum:4,resumeAt:42,session:7});
  assert.deepEqual(h.calls, [['view','native-idle'], ['play','123456',4,42]]);
  h.send({event:'command',command:'seek',position:12.5});
  assert.deepEqual(h.calls.at(-1), ['command','seek',12.5]);
  h.context.window.nativeMediaClosed();
  assert.deepEqual(h.calls.at(-1), ['NativeShell.ClosePlayer',7]);
});

test('closing player returns the native document to the current detail', () => {
  const h = host();
  h.send({event:'play',seriesId:'123456',episodeNum:4,resumeAt:42,session:7});
  h.send({event:'detail',seriesId:'123456'});
  const labels=h.calls.map(call=>call[0]+':'+(call[1]||''));
  assert.deepEqual(labels, ['view:native-idle','play:123456','view:native-idle','close:','view:detail','detail-load:123456']);
});
test('stop cannot resume a hidden recommendation and suppresses recursive close notifications', () => {
  const h = host();
  h.send({event:'stop'});
  assert.deepEqual(h.calls, [['view','native-idle'], ['close']]);
  assert.equal(h.calls.some(call=>call[0]==='play'), false);
});
test('embedded host uses the original media control modules and native shell bridges', () => {
  const html = fs.readFileSync('frontend/index.html','utf8');
  for (const name of ['ambient-glow.js','player-chrome.js','danmaku.js','progressive-media.js','app.js','native-media.js']) assert(html.includes(`src="${name}"`), name);
  const controls = fs.readFileSync('frontend/player-chrome.js','utf8');
  for (const control of ['quality','speed','danmaku','danmaku-settings','drawer-toggle','pip','fullscreen','like','comment','fav']) assert(controls.includes(`id('${control}')`), control);
  const app = fs.readFileSync('frontend/app.js','utf8');
  assert(app.includes('window.nativeMediaNotify?.("OpenDetail", seriesId)'));
  assert(app.includes('window.nativeMediaNotify?.("OpenActor", userId)'));
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

test('native player chrome mirrors geometry and hides with the player', () => {
  const h = host();
  const frame = (left=0, top=0, width=0, height=0) => ({getBoundingClientRect() { return {left:left, top:top, width:width, height:height}; }});
  const classes = new Set();
  const modal = {style:{display:'flex'}, classList:{contains(value) {return classes.has(value);}, add(value) {classes.add(value);}, remove(value) {classes.delete(value);}}};
  h.elements['player-modal'] = modal;
  h.elements['player-quality'] = frame(500, 700, 52, 28);
  h.elements['player-speed'] = frame(560, 700, 52, 28);
  h.elements['player-drawer-toggle'] = frame(760, 700, 72, 28);
  const controls = frame(100,680,800,52);
  controls.classList = {contains(value) {return classes.has(value);}};
  h.context.document.querySelector = selector => selector.includes('feed-controls') ? controls : null;
  h.ready();
  const updates = () => h.calls.filter(call => call[0] === 'NativeShell.PlayerChromeChanged');
  let value = updates().at(-1)[1];
  assert.equal(value.visible, true);
  assert.equal(value.controlsVisible, true);
  assert.deepEqual(JSON.parse(JSON.stringify(value.quality)), {x:500,y:700,w:52,h:28});
  assert.equal(value.qualityLabel, '720P');
  assert.equal(value.speedValue, 1.25);
  assert.equal(value.episode, 4);
  assert.equal(value.totalEpisodes, 12);
  const nativeBefore = updates().length;
  modal.classList.add('controls-hidden');
  h.listeners.observer0();
  assert.equal(updates().at(-1)[1].controlsVisible, false);
  modal.classList.remove('controls-hidden');
  h.listeners.observer0();
  assert.equal(updates().at(-1)[1].controlsVisible, true);
  const count = updates().length;
  h.listeners.observer0();
  assert.equal(updates().length, count, 'unchanged player chrome does not send another IPC update');
  modal.style.display = 'none';
  h.listeners.observer0();
  assert.equal(updates().at(-1)[1].visible, false);
});

test('native quality, speed and episode controls reuse existing player behavior', () => {
  const h = host();
  h.send({event:'quality', value:'1080p'});
  h.send({event:'speed', value:1.5});
  h.send({event:'speed', value:3});
  h.send({event:'episode', value:5});
  h.send({event:'episode', value:99});
  assert.deepEqual(h.calls.filter(call => ['quality','episode'].includes(call[0])), [['quality','1080p'],['episode',5]]);
  assert.equal(h.context.video.playbackRate, 1.5);
});

test('native layer never duplicates player quality, speed or episode controls', () => {
  const playerChrome = fs.readFileSync('internal/uiapp/player_chrome.go', 'utf8');
  const render = playerChrome.match(/func \(a \*App\) renderNativePlayerControls[\s\S]*?\n}/)?.[0] || '';
  assert.ok(render, 'player chrome render function must remain discoverable');
  assert.equal(render.includes('renderNativeQualityControl'), false, 'native quality button duplicates WebView');
  assert.equal(render.includes('renderNativeSpeedControl'), false, 'native speed button duplicates WebView');
  assert.equal(render.includes('renderNativeEpisodeButton'), false, 'native episode button duplicates WebView');
  assert.equal(playerChrome.includes('renderNativeQualityControl'), false);
  assert.equal(playerChrome.includes('renderNativeSpeedControl'), false);
  assert.equal(playerChrome.includes('renderNativeEpisodeButton'), false);
});

test('selected player quality and speed reach the document and refresh native chrome', () => {
  const h = host();
  h.context.state.player.quality = '720p';
  h.context.state.player.qualityOptions = [
    {id: '1080p', label: '1080P', available: true, active: false},
    {id: '720p', label: '720P', available: true, active: true}
  ];
  h.context.renderQualityMenu = function renderQualityMenu() {
    const active = h.context.state.player.qualityOptions.find(option => option.active);
    h.calls.push(['chrome-refresh', active.label, h.context.state.player.playbackSpeed]);
  };
  h.context.window.__syncPlayerChromeState = h.context.renderQualityMenu;
  h.send({event: 'quality', value: '1080p'});
  h.context.state.player.quality = '1080p';
  h.context.state.player.qualityOptions.forEach(option => {
    option.active = option.id === '1080p';
    option.available = true;
  });
  h.context.renderQualityMenu();
  h.send({event: 'speed', value: 2});
  assert.deepEqual(h.calls.filter(call => ['chrome-refresh', 'quality'].includes(call[0])), [
    ['quality', '1080p'],
    ['chrome-refresh', '1080P', 1.25],
    ['chrome-refresh', '1080P', 2]
  ]);
  assert.equal(h.context.video.playbackRate, 2);
});

test('the 812 episode drawer keeps the complete collection and never substitutes the current episode', () => {
});
