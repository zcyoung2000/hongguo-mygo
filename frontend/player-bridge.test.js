const fs = require('fs');
const vm = require('vm');
const assert = require('assert');
class Video {
  constructor() { this.currentTime = 0; this.duration = NaN; this.paused = true; this.ended = false; this.listeners = {}; }
  addEventListener(type, handler) { this.listeners[type] = handler; }
  removeAttribute() {}
  load() { this.currentTime = 0; this.duration = NaN; }
  setAttribute() {}
  play() { this.paused = false; this.listeners.play?.(); return Promise.resolve(); }
  pause() { this.paused = true; this.listeners.pause?.(); }
}
const video = new Video();
const callbacks = { progressValues: [], saved: [], nextValues: [], progress(p) { this.progressValues.push(p); }, saveProgress(p) { this.saved.push(p); }, next(p) { this.nextValues.push(p); } };
let timer;
const context = {
  document: { createElement: tag => tag === 'video' ? video : { setAttribute() {} }, body: { appendChild() {} } },
  nativePlayer: callbacks,
  addEventListener() {},
  setTimeout(fn) { timer = fn; return 1; },
  clearTimeout() { timer = null; },
};
context.window = context;
vm.createContext(context);
vm.runInContext(fs.readFileSync('frontend/native-player.js', 'utf8'), context);
const command = context.nativePlayerEvent;
command({ event: 'play', src: 'http://127.0.0.1/video1', seriesId: '123456', episodeId: '654321', session: 1, resumeAt: 7, autoplay: true });
assert.equal(video.currentTime, 0, 'resume must wait for metadata');
video.duration = 100; video.listeners.loadedmetadata();
assert.equal(video.currentTime, 7);
assert.equal(callbacks.progressValues.at(-1).session, 1);
command({ event: 'seek', position: 12.5 });
assert.equal(video.currentTime, 12.5);
video.listeners.timeupdate();
assert(timer);
command({ event: 'play', src: 'http://127.0.0.1/video2', seriesId: '123456', episodeId: '654322', session: 2, resumeAt: 0, autoplay: false });
assert.equal(timer, null, 'outgoing timer must be cancelled');
assert.equal(callbacks.saved.at(-1).episodeId, '654321');
assert.equal(callbacks.saved.at(-1).position, 12.5);
assert.equal(callbacks.saved.at(-1).session, 1);
assert.equal(video.paused, true);
video.duration = 120; video.listeners.loadedmetadata();
command({ event: 'resume' });
assert.equal(callbacks.progressValues.at(-1).playing, true);
video.currentTime = 42;
command({ event: 'pause' });
assert.equal(callbacks.progressValues.at(-1).playing, false);
assert.equal(callbacks.saved.at(-1).session, 2);
video.ended = true; video.listeners.ended();
assert.equal(callbacks.nextValues.at(-1).session, 2);
command({ event: 'stop' });
const count = callbacks.saved.length;
video.listeners.pause(); video.listeners.timeupdate();
assert.equal(callbacks.saved.length, count, 'events after stop must not save a detached session');
console.log('native player bridge script: ok');
