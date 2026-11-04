const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const declarations = [...source.matchAll(/^(?:async )?function (\w+)\(/gm)];

function harness() {
  const nodes = new Map();
  const element = id => {
    if (!nodes.has(id)) {
      const classes = new Set();
      nodes.set(id, {
        style: { display: 'none' }, scrollLeft: 0,
        classList: {
          add: (...items) => items.forEach(item => classes.add(item)),
          remove: (...items) => items.forEach(item => classes.delete(item)),
          contains: item => classes.has(item), toggle() {}
        },
        querySelector: () => element('body')
      });
    }
    return nodes.get(id);
  };
  element('player-modal').style.display = 'flex';
  const requests = [], saves = [], toasts = [], switches = [];
  const video = {
    src: '', duration: NaN, currentTime: 0, paused: true, playCount: 0,
    pause() { this.paused = true; },
    removeAttribute(name) { if (name === 'src') this.src = ''; },
    load() { this.duration = NaN; this.currentTime = 0; },
    play() { this.playCount++; this.paused = false; return Promise.resolve(); }
  };
  const c = vm.createContext({
    console, clearTimeout, video, window: {}, playerRequestToken: 0, playerPlaybackIdentity: null,
    document: { getElementById: element, querySelectorAll: () => [], documentElement: {}, body: element('body') },
    state: {
      currentView: 'detail', detail: { data: null },
      player: {
        seriesId: 'series', episodeNum: 1, episodeId: 'episode1', seriesTitle: 'Drama',
        seriesCover: '', totalEpisodes: 3, quality: '720p', playbackSpeed: 1,
        episodes: [1, 2, 3].map(number => ({ id: `episode${number}`, number }))
      }
    },
    commentState: { open: false }, danmakuControllers: {},
    updatePlayerCommentCount() {}, scrollActiveDrawerEpisode() {}, updatePlayerPlayBadge() {},
    resetPlaybackVideoFit() {}, updateQualityControls() {}, syncMainVideoOrientation() {},
    setPlaybackKeepAwake() {}, clearNowPlayingState() {}, restoreCommentOverlay() {},
    syncFeedScrollCapture() {}, pauseFeedPlayback() {}, clearCommentSceneLayout() {},
    fallbackQualityOptions: () => [], renderQualityMenu() {}, renderPlaylistDrawer() {},
    showToast: text => toasts.push(text), switchFeedDrama: value => switches.push(value),
    apiCall(method, ...args) {
      if (method === 'SaveProgress') { saves.push(args); return Promise.resolve(); }
      return new Promise((resolve, reject) => requests.push({ method, args, resolve, reject }));
    }
  });
  for (const name of [
    'clearPlayerVideo', 'openPlayer', 'playEpisodeNumber', 'persistProgress', 'closePlayer',
    'isPlayerRequestCurrent', 'playNextSeason', 'shouldCaptureFeedScroll', 'handleNativeFeedScroll',
    'clearMediaCache'
  ]) {
    const index = declarations.findIndex(match => match[1] === name);
    assert.ok(index >= 0, `missing ${name}`);
    vm.runInContext(source.slice(declarations[index].index, declarations[index + 1]?.index), c);
  }
  return { c, video, requests, saves, toasts, element, switches };
}

test('out-of-order episode replies cannot change the active source or progress identity', async () => {
  const { c, video, requests, saves } = harness();
  const older = c.playEpisodeNumber(2), newer = c.playEpisodeNumber(3);
  requests[1].resolve({ streamUrl: 'local/episode3' });
  assert.equal(await newer, true);
  requests[0].resolve({ streamUrl: 'local/episode2' });
  assert.equal(await older, false);
  assert.equal(video.src, 'local/episode3');
  video.currentTime = 42; video.duration = 100;
  c.persistProgress();
  assert.equal(saves.at(-1)[1], 'episode3');
  assert.equal(saves.at(-1)[6], 42);
});

test('closing invalidates a pending playback response and detached metadata callbacks', async () => {
  const { c, video, requests, element } = harness();
  const playing = c.playEpisodeNumber(1);
  requests[0].resolve({ streamUrl: 'local/episode1' });
  await playing;
  const detachedMetadata = video.onloadedmetadata;
  const pending = c.playEpisodeNumber(2);
  c.closePlayer();
  requests[1].resolve({ streamUrl: 'local/episode2' });
  assert.equal(await pending, false);
  detachedMetadata();
  assert.equal(video.src, '');
  assert.equal(video.playCount, 0);
  assert.equal(element('player-modal').style.display, 'none');
});

test('closing during detail loading cannot reopen the player or start an episode', async () => {
  const { c, requests, video } = harness();
  const opening = c.openPlayer('another-series');
  c.closePlayer();
  requests[0].resolve({ id: 'another-series', title: 'Other', episodes: [] });
  await opening;
  assert.deepEqual(requests.map(request => request.method), ['GetSeriesDetail']);
  assert.equal(video.src, '');
});

test('an old detail response cannot overwrite a newer series', async () => {
  const { c, requests } = harness();
  const older = c.openPlayer('old-series'), newer = c.openPlayer('new-series');
  requests[1].resolve({ title: 'New', episodes: [{ id: 'new-episode', number: 1 }] });
  await new Promise(resolve => setImmediate(resolve));
  requests[2].resolve({ streamUrl: 'local/new', episodeId: 'new-episode' });
  await newer;
  requests[0].resolve({ title: 'Old', episodes: [] });
  await older;
  assert.equal(c.state.player.seriesTitle, 'New');
  assert.equal(c.state.player.episodeId, 'new-episode');
});

test('switching persists the old source before changing episode identity', async () => {
  const { c, requests, video, saves } = harness();
  const first = c.playEpisodeNumber(1);
  requests[0].resolve({ streamUrl: 'local/episode1' });
  await first;
  video.duration = 100; video.currentTime = 25;
  const second = c.playEpisodeNumber(2);
  assert.equal(saves.at(-1)[1], 'episode1');
  assert.equal(saves.at(-1)[6], 25);
  video.duration = 100; video.currentTime = 99;
  c.persistProgress();
  assert.equal(saves.length, 1, 'pending source has no valid playback identity');
  requests[1].resolve({ streamUrl: 'local/episode2' });
  await second;
});

test('stale failures do not hide the current loader or display an error', async () => {
  const { c, requests, toasts } = harness();
  const old = c.playEpisodeNumber(1), current = c.playEpisodeNumber(2);
  requests[0].reject(new Error('old request failed'));
  await old;
  assert.equal(c.state.player.loading, true);
  assert.equal(toasts.length, 0);
  requests[1].resolve(null);
  assert.equal(await current, false);
  assert.equal(c.state.player.loading, false);
});

test('a growing source saves real progress using its SDK duration before EOF', async () => {
  const { c, requests, video, saves } = harness();
  const playing = c.playEpisodeNumber(1);
  requests[0].resolve({ streamUrl: 'local/growing', durationSec: 100 });
  await playing;
  video.duration = Infinity;
  video.currentTime = 20;
  c.persistProgress();
  assert.equal(saves[0][1], 'episode1');
  assert.equal(saves[0][6], 20);
  assert.equal(saves[0][7], 100);
});

test('closing before metadata arrives does not overwrite saved progress with zero', async () => {
  const { c, requests, saves } = harness();
  const playing = c.playEpisodeNumber(1);
  requests[0].resolve({ streamUrl: 'local/growing', durationSec: 100, lastPosition: 20 });
  await playing;
  c.closePlayer();
  assert.equal(saves.length, 0);
});

test('metadata arriving after a user pause does not restart autoplay', async () => {
  const { c, requests, video } = harness();
  const playing = c.playEpisodeNumber(1);
  requests[0].resolve({ streamUrl: 'local/growing', durationSec: 100 });
  await playing;
  c.state.player.userPaused = true;
  video.duration = 100;
  video.onloadedmetadata();
  assert.equal(video.playCount, 0);
  assert.equal(c.state.player.userPaused, true);
});

test('a late next-season lookup cannot reopen a closed player', async () => {
  const { c, requests } = harness();
  const next = c.playNextSeason('series');
  c.closePlayer();
  assert.equal(c.state.player.seasonSwitching, false);
  requests[0].resolve({ id: 'next-season' });
  assert.equal(await next, false);
  assert.equal(requests.length, 1);
});

test('episode controls cannot interrupt detail loading with the previous series metadata', async () => {
  const { c, requests } = harness();
  const opening = c.openPlayer('new-series');
  assert.equal(await c.playEpisodeNumber(2), false);
  assert.equal(requests.length, 1);
  requests[0].resolve({ title: 'New', episodes: [{ id: 'new-episode', number: 1 }] });
  await new Promise(resolve => setImmediate(resolve));
  requests[1].resolve({ streamUrl: 'local/new', episodeId: 'new-episode' });
  await opening;
  assert.equal(c.state.player.seriesTitle, 'New');
});

test('cache clearing invalidates renderer metadata and waits for speculative admission release', async () => {
  const { c, requests } = harness();
  c.feedPlaybackGeneration = 5;
  c.feedSlotCurr = 0;
  c.feedSlots = [{ ready: true, streamUrl: 'active' }, { ready: true, streamUrl: 'next' }];
  c.state.currentView = 'settings';
  c.state.feed = { index: 0, preloadMap: new Map([['old', {}]]), playbackInfoLoads: new Map([['old', {}]]) };
  c.cancelScheduledFeedWarmup = () => {};
  c.clearFeedSlot = slot => { slot.streamUrl = ''; };
  let released;
  c.updateFeedPreloadWindow = () => new Promise(resolve => { released = resolve; });
  const clearing = c.clearMediaCache();
  assert.equal(c.feedPlaybackGeneration, 6);
  assert.equal(c.state.feed.preloadMap.size, 0);
  assert.equal(c.state.feed.playbackInfoLoads.size, 0);
  assert.equal(c.feedSlots[0].streamUrl, '');
  assert.equal(c.feedSlots[1].streamUrl, '');
  assert.equal(requests.length, 0);
  released();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(requests[0].method, 'ClearCache');
  requests[0].resolve(123);
  assert.equal(await clearing, 123);
});

test('cache clearing keeps the active decoder and resumes warmup after invalidating adjacent sources', async () => {
  const { c, requests } = harness();
  c.feedPlaybackGeneration = 0; c.feedSlotCurr = 0;
  c.feedSlots = [{ ready: true, streamUrl: 'active' }, { ready: true, streamUrl: 'next' }];
  c.state.currentView = 'recommend';
  c.state.feed = { index: 4, preloadMap: new Map(), playbackInfoLoads: new Map() };
  c.cancelScheduledFeedWarmup = () => {};
  c.clearFeedSlot = slot => { slot.streamUrl = ''; };
  c.updateFeedPreloadWindow = current => { assert.equal(current, c.feedSlots[0]); return Promise.resolve(); };
  let warmupIndex;
  c.scheduleFeedWarmup = index => { warmupIndex = index; };
  const clearing = c.clearMediaCache();
  await new Promise(resolve => setImmediate(resolve));
  requests[0].resolve(20);
  await clearing;
  assert.equal(c.feedSlots[0].streamUrl, 'active');
  assert.equal(c.feedSlots[1].streamUrl, '');
  assert.equal(warmupIndex, 4);
});

test('native scroll events queued before comments or a modal opened cannot switch dramas', () => {
  const { c, element, switches } = harness();
  c.state.currentView = 'recommend';
  element('player-modal').style.display = 'none';
  assert.equal(c.shouldCaptureFeedScroll(), true);
  c.commentState.open = true;
  c.handleNativeFeedScroll({ direction: 'down' });
  assert.equal(c.shouldCaptureFeedScroll(), false);
  c.commentState.open = false;
  element('feed-playlist-drawer').classList.add('open');
  c.handleNativeFeedScroll({ direction: 'down' });
  element('feed-playlist-drawer').classList.remove('open');
  element('player-modal').style.display = 'flex';
  c.handleNativeFeedScroll({ direction: 'down' });
  assert.equal(switches.length, 0);
  element('player-modal').style.display = 'none';
  c.handleNativeFeedScroll({ direction: 'up' });
  assert.deepEqual(switches, [-1]);
});

test('native scroll capture changes serialize and converge after rapid comment toggles', async () => {
  const { c, element, requests } = harness();
  c.feedScrollCaptureSync = null;
  c.state.currentView = 'recommend';
  element('player-modal').style.display = 'none';
  const index = declarations.findIndex(match => match[1] === 'syncFeedScrollCapture');
  vm.runInContext(source.slice(declarations[index].index, declarations[index + 1].index), c);
  const syncing = c.syncFeedScrollCapture();
  c.commentState.open = true;
  c.syncFeedScrollCapture();
  assert.equal(requests.length, 1, 'updates must not race through IPC');
  requests[0].resolve();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(requests[1].args[0], false);
  c.commentState.open = false;
  c.syncFeedScrollCapture();
  assert.equal(requests.length, 2);
  requests[1].resolve();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(requests[2].args[0], true);
  requests[2].resolve();
  await syncing;
  assert.equal(c.feedScrollCaptureSync, null);
});
