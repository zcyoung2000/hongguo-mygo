const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const functionNames = [...source.matchAll(/^(?:async )?function (\w+)\(/gm)].map(match => match[1]);

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
    requestFeedSwitch(direction, source) { calls.push(['requestFeedSwitch', direction, source]); },
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
  assert.deepEqual(h.calls.at(-1), ['requestFeedSwitch', 1, 'media-command']);
  h.context.handleMediaCommand({command: 'prev'});
  assert.deepEqual(h.calls.at(-1), ['requestFeedSwitch', -1, 'media-command']);
  h.context.handleMediaCommand({command: 'seek', position: 30});
  assert.equal(h.feedVideo.currentTime, 30);
  h.context.handleMediaCommand({command: 'pip'});
  assert.deepEqual(h.calls.at(-1), ['togglePictureInPicture']);
});

test('media navigation uses declared feed switch and ended handling stays active', () => {
  assert.ok(functionNames.includes('requestFeedSwitch'), 'requestFeedSwitch must be declared');
  assert.doesNotMatch(source, /\bswipeFeed\s*\(/, 'the removed swipeFeed path must stay removed');
  assert.doesNotMatch(source, /\bplayerModal\b/, 'ended handling must not read a stale modal global');
  const endedBody = source.slice(source.indexOf("video.addEventListener('ended'"), source.indexOf('if (progressWrap)', source.indexOf("video.addEventListener('ended'")));
  assert.match(endedBody, /state\.currentView !== 'recommend' \|\| feedSlots\[feedSlotCurr\] !== slot/);
});

test('quality fallback helpers are declared for the real command path', () => {
  for (const name of ['normalizeFeedQuality', 'qualityLabel', 'fallbackQualityOptions']) {
    assert.ok(functionNames.includes(name), `${name} must be declared`);
  }
});
