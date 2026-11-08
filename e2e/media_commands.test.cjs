const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');

function commandHarness({detailOpen = false, currentView = 'recommend'} = {}) {
  const calls = [];
  const nodes = new Map();
  const node = id => {
    if (!nodes.has(id)) {
      nodes.set(id, {id, disabled: false, style: {}, classList: {contains: () => false}, click() { calls.push(['click', id]); }});
    }
    return nodes.get(id);
  };
  const feedVideo = {paused: false, currentTime: 10, duration: 60, play: async () => { feedVideo.paused = false; }, pause: () => { feedVideo.paused = true; }};
  const mainVideo = {paused: true, currentTime: 5, duration: 90};
  const context = {
    console, document: {getElementById: node},
    state: {currentView}, video: mainVideo, getFeedVideo: () => feedVideo,
    toggleFeedPlay() { calls.push(['toggleFeedPlay']); },
    togglePictureInPicture() { calls.push(['togglePictureInPicture']); },
    swipeFeed(direction) { calls.push(['swipeFeed', direction]); },
  };
  vm.createContext(context);
  const start = source.indexOf('function handleMediaCommand(');
  assert.ok(start >= 0, 'handleMediaCommand missing');
  const brace = source.indexOf('{', start);
  let depth = 0, end = -1;
  for (let index = brace; index < source.length; index++) {
    if (source[index] === '{') depth++;
    else if (source[index] === '}') {
      depth--;
      if (depth === 0) { end = index + 1; break; }
    }
  }
  vm.runInContext(source.slice(start, end), context);
  return {context, calls, node, feedVideo, mainVideo};
}

test('system media commands work in the unified feed without a stale detail player global', () => {
  const h = commandHarness();
  h.context.handleMediaCommand({command: 'toggle'});
  assert.deepEqual(h.calls, [['toggleFeedPlay']]);
  h.context.handleMediaCommand({command: 'pause'});
  assert.equal(h.feedVideo.paused, true);
  h.context.handleMediaCommand({command: 'next'});
  assert.deepEqual(h.calls.at(-1), ['swipeFeed', 'up']);
  h.context.handleMediaCommand({command: 'seek', position: 30});
  assert.equal(h.feedVideo.currentTime, 30);
  h.context.handleMediaCommand({command: 'pip'});
  assert.deepEqual(h.calls.at(-1), ['togglePictureInPicture']);
});
