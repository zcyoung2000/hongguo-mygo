const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const functions = [...source.matchAll(/^(?:async )?function (\w+)\(/gm)];
function load(c, name) {
  const i = functions.findIndex(match => match[1] === name);
  vm.runInContext(source.slice(functions[i].index, functions[i + 1]?.index), c);
}
function harness() {
  let now = 100, next = 0;
  const timers = new Map(), attributes = {}, classes = new Map(), traces = [];
  const video = new EventTarget();
  Object.assign(video, { src: 'http://localhost/video', readyState: 4, videoWidth: 320,
    currentTime: 0, paused: false, ended: false, dataset: {},
    buffered: { length: 1, start: () => 0, end: () => 30 } });
  const slot = { id: 0, itemIndex: 0, video, metricPreparedAt: 1, metricRebufferMS: 0, metricRebufferCount: 0 };
  const nodes = {
    'feed-play-indicator': { style: {} },
    'feed-loader': { style: { display: 'none' }, setAttribute: (name, value) => { attributes[name] = value; } },
    'feed-stage': { classList: { toggle: (name, value) => classes.set(name, value), remove: name => classes.delete(name) } }
  };
  const c = vm.createContext({ Event, video, window: {},
    performance: { now: () => now }, feedBufferingTimer: null, feedSwitchWaitReason: '',
    feedSlots: [slot], feedSlotCurr: 0, getFeedVideo: () => video, feedSlotForVideo: () => slot,
    HTMLMediaElement: { HAVE_FUTURE_DATA: 3, HAVE_ENOUGH_DATA: 4 },
    state: { currentView: 'recommend', feed: { index: 0, items: [{}], userPaused: false } },
    document: { getElementById: id => nodes[id] },
    setTimeout: (fn, delay) => { timers.set(++next, {fn, at: now + delay}); return next; },
    clearTimeout: id => timers.delete(id),
    recordFeedDiagnostic: event => traces.push(event), wakeFeedWarmup() {}, restoreFeedFirstFrameSound() {}
  });
  for (const name of ['feedVideoHasPlaybackBuffer', 'attachFeedVideoMetrics', 'updateFeedPlayIndicator']) load(c, name);
  c.attachFeedVideoMetrics(slot);
  const start = source.indexOf("    video.addEventListener('waiting',", source.indexOf('function initFeedControls()'));
  const end = source.indexOf("    video.addEventListener('canplay',", start);
  vm.runInContext(source.slice(start, end), c);
  function advance(ms) {
    now += ms;
    for (const [id, timer] of [...timers]) {
      if (timer.at <= now) { timers.delete(id); timer.fn(); }
    }
  }
  return { c, slot, video, nodes, attributes, classes, traces, advance };
}
test('a real waiting event shows delayed feedback, then playing clears it', () => {
  const h = harness();
  h.video.readyState = 1;
  h.video.dispatchEvent(new Event('waiting'));
  assert.equal(h.classes.get('is-buffering'), true);
  h.advance(199);
  assert.equal(h.nodes['feed-loader'].style.display, 'none');
  h.advance(1);
  assert.equal(h.nodes['feed-loader'].style.display, 'flex');
  assert.equal(h.attributes['aria-label'], '加载中');
  h.video.readyState = 4;
  h.video.dispatchEvent(new Event('playing'));
  assert.equal(h.nodes['feed-loader'].style.display, 'none');
  assert.equal(h.classes.get('is-buffering'), false);
  assert.ok(h.traces.includes('media-playing'));
});
test('preparing the next frame or catalog page does not cover a healthy playing video', () => {
  for (const reason of ['catalog-page', 'first-frame']) {
    const h = harness();
    h.c.feedSwitchWaitReason = reason;
    h.c.updateFeedPlayIndicator();
    h.advance(500);
    assert.equal(h.nodes['feed-loader'].style.display, 'none');
    assert.equal(h.classes.get('is-buffering'), false);
  }
});
test('short waits, a user pause, or leaving the feed cannot leave a spinner behind', () => {
  for (const action of ['playing', 'pause', 'leave']) {
    const h = harness();
    h.video.readyState = 1;
    h.video.dispatchEvent(new Event('waiting'));
    h.advance(100);
    if (action === 'playing') { h.video.readyState = 4; h.video.dispatchEvent(new Event('playing')); }
    if (action === 'pause') h.c.state.feed.userPaused = true;
    if (action === 'leave') h.c.state.currentView = 'history';
    h.c.updateFeedPlayIndicator();
    h.advance(1000);
    assert.equal(h.nodes['feed-loader'].style.display, 'none', action);
  }
});
