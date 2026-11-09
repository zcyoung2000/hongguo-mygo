const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');

function loadFunction(context, name) {
  const match = source.match(new RegExp(`(?:async )?function ${name}\\b`));
  assert.ok(match, `missing ${name}`);
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
  assert.notEqual(end, -1, `unterminated ${name}`);
  vm.runInContext(source.slice(match.index, end), context);
}

function harness({episodeNum = 1, totalEpisodes = 2, items = [{id: 'series-a'}, {id: 'series-b'}]} = {}) {
  const timers = new Map();
  const calls = [];
  const video = {
    src: 'episode-a',
    currentTime: 0,
    duration: Infinity,
    ended: false,
    paused: false,
    error: null,
    dataset: {},
    handlers: {}
  };
  video.addEventListener = (name, handler) => {
    video.handlers[name] ||= [];
    video.handlers[name].push(handler);
  };
  const slot = {
    id: 0,
    video,
    itemIndex: 0,
    seriesId: 'series-a',
    episodeNum,
    quality: '720p',
    loadToken: 7,
    endedAdvanceKey: null
  };
  const context = vm.createContext({
    console,
    window: {},
    setTimeout(callback, delay = 0) {
      const id = timers.size + 1;
      timers.set(id, {callback, delay});
      return id;
    },
    clearTimeout(id) {
      timers.delete(id);
    },
    state: {
      currentView: 'recommend',
      feed: {
        index: 0,
        items,
        seriesId: 'series-a',
        episodeNum,
        totalEpisodes,
        quality: '720p',
        playbackOrigin: 'feed',
        playbackPositions: new Map()
      }
    },
    feedSlots: [slot],
    feedNextDramaTimer: null,
    feedSlotCurr: 0,
    getFeedVideo: () => video,
    feedSlotForVideo: () => slot,
    feedMediaDuration: () => 115,
    feedPlaybackKey: (seriesId, episodeNum, quality) => `${seriesId}:${episodeNum}:${quality}`,
    rememberFeedPlaybackPosition() {},
    updateFeedMediaTime() {},
    persistFeedProgress() {
      calls.push(['persist', episodeNum]);
    },
    enterSeriesPlayer(seriesId, nextEpisodeNum, origin) {
      calls.push(['enter', seriesId, nextEpisodeNum, origin]);
    },
    playNextSeason(seriesId, sourceName) {
      calls.push(['season', seriesId, sourceName]);
      return Promise.resolve(false);
    },
    switchFeedDrama(direction) {
      calls.push(['switch', direction]);
    }
  });
  for (const name of ['shouldAdvanceEndedFeedPlayback', 'advanceEndedFeedPlayback']) loadFunction(context, name);
  const timeUpdateStart = source.indexOf("video.addEventListener('timeupdate'", source.indexOf('function initFeedControls()'));
  const timeUpdateEnd = source.indexOf('    });', timeUpdateStart) + '    });'.length;
  context.hVideo = video;
  context.hSlot = slot;
  vm.runInContext(`var video = hVideo; var slot = hSlot;\n${source.slice(timeUpdateStart, timeUpdateEnd)}`, context);
  return {context, slot, video, calls, timers};
}

async function flush() {
  await new Promise(resolve => setImmediate(resolve));
}

test('an ended feed episode advances through the existing episode switch path', async () => {
  const h = harness();
  h.video.ended = true;
  h.context.advanceEndedFeedPlayback(h.slot);
  await flush();

  assert.deepEqual(h.calls, [['persist', 1], ['enter', 'series-a', 2, 'feed']]);
  assert.equal(h.context.state.feed.playbackPositions.size, 0);
});

test('the final feed episode advances to the next feed slot after no next season', async () => {
  const h = harness({episodeNum: 2});
  h.video.ended = true;
  h.context.advanceEndedFeedPlayback(h.slot);
  await flush();
  assert.deepEqual(h.calls, [['persist', 2], ['season', 'series-a', 'feed']]);

  const timer = [...h.timers.values()][0];
  assert.ok(timer, 'next-slot timer must be scheduled');
  timer.callback();
  assert.deepEqual(h.calls.at(-1), ['switch', 1]);
});

test('one ended playback identity can advance only once', async () => {
  const h = harness();
  h.video.ended = true;
  h.context.advanceEndedFeedPlayback(h.slot);
  h.context.advanceEndedFeedPlayback(h.slot);
  await flush();

  assert.deepEqual(h.calls, [['persist', 1], ['enter', 'series-a', 2, 'feed']]);
});

test('a metadata-duration boundary only synthesizes end while playback reaches it', () => {
  const h = harness();
  h.video.currentTime = 114.8;
  h.video.duration = Infinity;
  h.video.paused = true;
  h.video.handlers.timeupdate.forEach(handler => handler());
  assert.equal(h.video.ended, false);
  assert.deepEqual(h.calls, []);

  h.video.paused = false;
  h.video.handlers.timeupdate.forEach(handler => handler());
  assert.equal(h.video.ended, true);
  assert.deepEqual(h.calls, [['persist', 1], ['enter', 'series-a', 2, 'feed']]);
});

test('an old or inactive ended callback cannot advance current playback', async () => {
  const h = harness();
  h.context.state.currentView = 'history';
  h.context.advanceEndedFeedPlayback(h.slot);
  h.context.state.currentView = 'recommend';
  h.slot.endedAdvanceKey = null;
  h.context.feedSlotCurr = 1;
  h.context.advanceEndedFeedPlayback(h.slot);
  await flush();

  assert.deepEqual(h.calls, []);
});
