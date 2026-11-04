const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const script = fs.readFileSync(path.join(__dirname, '../frontend/progressive-media.js'), 'utf8');
const ProgressiveMediaSource = require('../frontend/progressive-media.js');

function box(type, ...parts) {
  const payload = Buffer.concat(parts.map(part => Buffer.from(part)));
  const header = Buffer.alloc(8);
  header.writeUInt32BE(payload.length + 8);
  header.write(type, 4);
  return Buffer.concat([header, payload]);
}

function init({ hevc = false, audio = true } = {}) {
  const config = hevc
    ? box('hvcC', [1, 1, 0x60, 0, 0, 0, 0xb0, 0, 0, 0, 0, 0, 93, ...Array(10).fill(0)])
    : box('avcC', [1, 100, 0, 31]);
  const track = entry => box('trak', box('mdia', box('minf', box('stbl', box('stsd', Buffer.alloc(8), entry)))));
  const video = track(box(hevc ? 'hvc1' : 'avc1', Buffer.alloc(78), config));
  // AAC-LC DecoderSpecificInfo (44.1 kHz, stereo), inside ES/DecoderConfig.
  const esds = box('esds', [0, 0, 0, 0, 3, 25, 0, 1, 0, 4, 17, 0x40, 0x15,
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 2, 0x12, 0x10, 6, 1, 2]);
  return box('moov', video, ...(audio ? [track(box('mp4a', Buffer.alloc(28), esds))] : []));
}

test('decoder MIME comes from AVC/HEVC and audio configuration rather than a guessed profile', () => {
  assert.equal(ProgressiveMediaSource.mimeFromInit(init()), 'video/mp4; codecs="avc1.64001F,mp4a.40.2"');
  assert.equal(ProgressiveMediaSource.mimeFromInit(init({ hevc: true })), 'video/mp4; codecs="hvc1.1.6.L93.B0,mp4a.40.2"');
  assert.equal(ProgressiveMediaSource.mimeFromInit(init({ audio: false })), 'video/mp4; codecs="avc1.64001F"');
  const bytes = init();
  for (let n = 0; n < bytes.length; n++) assert.equal(ProgressiveMediaSource.mimeFromInit(bytes.subarray(0, n)), null);
});

function harness({ supported = true, autoOpen = true, autoAppend = true, appendError = null } = {}) {
  const media = [], appended = [], revoked = [], errors = [];
  const chunks = [];
  let pending, completed = 0, cancelled = 0;
  const reader = {
    read() { return chunks.length ? Promise.resolve(chunks.shift()) : new Promise(resolve => { pending = resolve; }); },
    cancel() { cancelled++; pending?.({ done: true }); return Promise.resolve(); },
    releaseLock() {}
  };
  class BufferSource extends EventTarget {
    buffered = { length: 0 };
    appendBuffer(bytes) {
      if (appendError) throw appendError;
      appended.push(Buffer.from(bytes));
      if (autoAppend) queueMicrotask(() => this.dispatchEvent(new Event('updateend')));
    }
  }
  class Media extends EventTarget {
    static isTypeSupported() { return supported; }
    constructor() { super(); media.push(this); this.readyState = 'closed'; }
    addSourceBuffer(mime) { this.mime = mime; return this.buffer = new BufferSource(); }
    endOfStream() { this.readyState = 'ended'; this.ended = true; }
    setLiveSeekableRange(start, end) { this.seekable = [start, end]; }
  }
  const video = new EventTarget();
  video.currentTime = 0;
  video.load = () => {
    if (autoOpen) queueMicrotask(() => {
      const source = media.at(-1);
      source.readyState = 'open';
      source.dispatchEvent(new Event('sourceopen'));
    });
  };
  const window = {
    MediaSource: Media,
    URL: { createObjectURL: () => 'blob:test-media', revokeObjectURL: value => revoked.push(value) },
    fetch: () => Promise.resolve({ ok: true, body: { getReader: () => reader } })
  };
  const context = vm.createContext({ window, AbortController, DOMException });
  vm.runInContext(script, context);
  const controller = new window.ProgressiveMediaSource(video, 'http://localhost/video', {
    duration: 120, onError: error => errors.push(error), onComplete: () => { completed++; }
  });
  function push(value, done = false) {
    const result = { value, done };
    if (pending) { const resolve = pending; pending = null; resolve(result); }
    else chunks.push(result);
  }
  return { controller, video, media, appended, revoked, errors, push,
    get completed() { return completed; }, get cancelled() { return cancelled; } };
}
const flush = () => new Promise(resolve => setImmediate(resolve));

test('split initialization is appended before EOF and the unfinished media stays streaming', async () => {
  const h = harness();
  const bytes = init();
  h.push(bytes.subarray(0, 7));
  await flush();
  assert.equal(h.appended.length, 0);
  h.push(bytes.subarray(7));
  await flush();
  assert.deepEqual(h.appended, [bytes]);
  assert.equal(h.media[0].duration, Infinity);
  assert.equal(h.completed, 0);
  h.push(Uint8Array.from([1, 2, 3, 4]));
  await flush();
  assert.equal(h.appended.length, 2);
  h.push(undefined, true);
  await h.controller.done;
  assert.equal(h.completed, 1);
  assert.equal(h.media[0].ended, true);
  h.controller.dispose();
  assert.deepEqual(h.revoked, ['blob:test-media']);
});

test('switching away cancels an outstanding read and releases its object URL once', async () => {
  const h = harness();
  h.push(init());
  await flush();
  h.controller.dispose();
  h.controller.dispose();
  await h.controller.done;
  assert.equal(h.cancelled, 1);
  assert.equal(h.errors.length, 0);
  assert.equal(h.completed, 0);
  assert.deepEqual(h.revoked, ['blob:test-media']);
});

test('a large coalesced first chunk can contain a small valid initialization followed by media', async () => {
  const h = harness();
  const bytes = Buffer.concat([init(), box('mdat', Buffer.alloc(2 * 1024 * 1024))]);
  h.push(bytes);
  await flush();
  assert.deepEqual(h.appended, [bytes]);
  assert.equal(h.errors.length, 0);
  h.controller.dispose();
  await h.controller.done;
});

test('unrecognized initialization remains bounded even if no decoder configuration arrives', async () => {
  const h = harness();
  h.push(box('free', Buffer.alloc(2 * 1024 * 1024)));
  await h.controller.done;
  assert.equal(h.appended.length, 0);
  assert.equal(h.errors.length, 1);
  assert.match(h.errors[0].message, /initialization is too large/);
  h.controller.dispose();
});

test('cancellation before sourceopen or during an append cannot hang playback cleanup', async () => {
  for (const config of [{ autoOpen: false }, { autoAppend: false }]) {
    const h = harness(config);
    h.push(init());
    await flush();
    h.controller.dispose();
    await h.controller.done;
    assert.equal(h.errors.length, 0);
    assert.equal(h.completed, 0);
  }
});

test('unsupported codecs notify the native fallback instead of leaving a pending decoder', async () => {
  const h = harness({ supported: false });
  h.push(init());
  await h.controller.done;
  assert.equal(h.errors.length, 1);
  assert.equal(h.appended.length, 0);
  assert.equal(h.completed, 0);
  h.controller.dispose();
});

test('an ample buffer pauses appends and resumes when playback consumes the reserve', async () => {
  const h = harness();
  h.push(init());
  await flush();
  h.media[0].buffer.buffered = { length: 1, end: () => 70 };
  h.push(Uint8Array.from([1, 2, 3]));
  await flush();
  assert.equal(h.appended.length, 1);
  h.video.currentTime = 20;
  h.video.dispatchEvent(new Event('timeupdate'));
  await flush();
  assert.equal(h.appended.length, 2);
  h.controller.dispose();
  await h.controller.done;
});

test('an append failure releases listeners and reaches fallback exactly once', async () => {
  const h = harness({ appendError: new DOMException('Decoder quota', 'QuotaExceededError') });
  h.push(init());
  await h.controller.done;
  assert.equal(h.errors.length, 1);
  assert.equal(h.errors[0].name, 'QuotaExceededError');
  h.controller.dispose();
});
