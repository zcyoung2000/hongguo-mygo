(function (root) {
  'use strict';

  function boxes(data, start = 0, end = data.length) {
    const result = [];
    const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
    while (start + 8 <= end) {
      let size = view.getUint32(start), header = 8;
      if (size === 1) {
        if (start + 16 > end) break;
        size = view.getUint32(start + 8) * 4294967296 + view.getUint32(start + 12);
        header = 16;
      }
      if (!size) size = end - start;
      if (size < header || start + size > end) break;
      result.push({ type: String.fromCharCode(...data.subarray(start + 4, start + 8)), start: start + header, end: start + size });
      start += size;
    }
    return result;
  }

  function descriptor(data, start, end) {
    if (start >= end) return null;
    const tag = data[start++];
    let length = 0;
    for (let n = 0; n < 4 && start < end; n++) {
      const byte = data[start++];
      length = length * 128 + (byte & 127);
      if (!(byte & 128)) return start + length <= end ? { tag, start, end: start + length } : null;
    }
    return null;
  }

  function aacCodec(data, box) {
    const es = descriptor(data, box.start + 4, box.end);
    if (!es || es.tag !== 3) throw new Error('Missing audio configuration');
    let start = es.start + 3;
    const flags = data[es.start + 2];
    if (flags & 128) start += 2;
    if (flags & 64) start += 1 + data[start];
    if (flags & 32) start += 2;
    const decoder = descriptor(data, start, es.end);
    if (!decoder || decoder.tag !== 4 || data[decoder.start] !== 0x40) throw new Error('Unsupported audio codec');
    const config = descriptor(data, decoder.start + 13, decoder.end);
    if (!config || config.tag !== 5 || config.start >= config.end) throw new Error('Missing AAC configuration');
    let profile = data[config.start] >> 3;
    if (profile === 31) profile = 32 + ((data[config.start] & 7) << 3) + (data[config.start + 1] >> 5);
    return `mp4a.40.${profile}`;
  }

  function videoCodec(data, entry, config) {
    const p = config.start;
    const hex = n => n.toString(16).padStart(2, '0').toUpperCase();
    if (config.type === 'avcC' && config.end - p >= 4) {
      return `${entry.type}.${hex(data[p + 1])}${hex(data[p + 2])}${hex(data[p + 3])}`;
    }
    if (config.type !== 'hvcC' || config.end - p < 23) throw new Error('Unsupported video configuration');
    let compatibility = new DataView(data.buffer, data.byteOffset + p + 2, 4).getUint32(0), reversed = 0;
    for (let bit = 0; bit < 32; bit++) { reversed = (reversed * 2 + (compatibility & 1)) >>> 0; compatibility >>>= 1; }
    const space = ['', 'A', 'B', 'C'][data[p + 1] >> 6];
    const constraints = [...data.subarray(p + 6, p + 12)];
    while (constraints.length && !constraints.at(-1)) constraints.pop();
    return `${entry.type}.${space}${data[p + 1] & 31}.${reversed.toString(16).toUpperCase()}.${data[p + 1] & 32 ? 'H' : 'L'}${data[p + 12]}${constraints.length ? '.' + constraints.map(hex).join('.') : ''}`;
  }

  // Read the actual decoder configuration, including HEVC profile and AAC
  // object type. Guessed MIME profiles can make WebKit reject valid media.
  function mimeFromInit(data) {
    const moov = boxes(data).find(box => box.type === 'moov');
    if (!moov) return null;
    const codecs = [];
    const walk = (start, end) => {
      for (const box of boxes(data, start, end)) {
        if (['trak', 'mdia', 'minf', 'stbl'].includes(box.type)) walk(box.start, box.end);
        if (box.type !== 'stsd') continue;
        for (const entry of boxes(data, box.start + 8, box.end)) {
          if (['avc1', 'avc3', 'hvc1', 'hev1'].includes(entry.type)) {
            const config = boxes(data, entry.start + 78, entry.end).find(child => ['avcC', 'hvcC'].includes(child.type));
            if (!config) throw new Error('Missing video configuration');
            codecs.push(videoCodec(data, entry, config));
          } else if (entry.type === 'mp4a') {
            const config = boxes(data, entry.start + 28, entry.end).find(child => child.type === 'esds');
            if (!config) throw new Error('Missing audio configuration');
            codecs.push(aacCodec(data, config));
          } else throw new Error('Unsupported media track');
        }
      }
    };
    walk(moov.start, moov.end);
    if (!codecs.length) throw new Error('Missing media tracks');
    return `video/mp4; codecs="${codecs.join(',')}"`;
  }

  function waitFor(target, events, signal) {
    return new Promise((resolve, reject) => {
      const cleanup = () => {
        events.forEach(event => target.removeEventListener(event, finish));
        signal.removeEventListener('abort', abort);
      };
      const abort = () => { cleanup(); reject(new DOMException('Playback cancelled', 'AbortError')); };
      const finish = event => {
        cleanup();
        if (event.type === 'error') reject(new Error('Media append failed'));
        else resolve();
      };
      events.forEach(event => target.addEventListener(event, finish));
      signal.addEventListener('abort', abort, { once: true });
      if (signal.aborted) abort();
    });
  }

  class ProgressiveMediaSource {
    static isSupported() { return typeof root.MediaSource === 'function' && typeof root.fetch === 'function'; }
    static mimeFromInit(data) { return mimeFromInit(data); }

    constructor(video, url, options = {}) {
      this.video = video;
      this.url = url;
      this.options = options;
      this.abort = new AbortController();
      this.media = new root.MediaSource();
      this.objectURL = root.URL.createObjectURL(this.media);
      const opened = waitFor(this.media, ['sourceopen', 'error'], this.abort.signal);
      video.src = this.objectURL;
      video.load();
      this.done = this.run(opened).catch(error => {
        this.abort.abort();
        if (!this.disposed) options.onError?.(error);
      });
    }

    async append(buffer, data) {
      if (this.abort.signal.aborted) throw new DOMException('Playback cancelled', 'AbortError');
      // Keep the fetch reader bounded; the local server continues caching to
      // disk while the media element consumes its decoded buffer.
      while (buffer.buffered.length && buffer.buffered.end(buffer.buffered.length - 1) - this.video.currentTime > 60) {
        await waitFor(this.video, ['timeupdate', 'seeking', 'error'], this.abort.signal);
      }
      const updated = waitFor(buffer, ['updateend', 'error'], this.abort.signal);
      try { buffer.appendBuffer(data); }
      catch (error) { this.abort.abort(); await updated.catch(() => {}); throw error; }
      await updated;
    }

    async run(opened) {
      const responsePromise = root.fetch(this.url, { signal: this.abort.signal });
      // Observe both promises immediately, including cancellation before the
      // media element opens its SourceBuffer.
      const [response] = await Promise.all([responsePromise, opened]);
      if (!response.ok || !response.body) throw new Error('Progressive media is unavailable');
      const reader = response.body.getReader();
      this.reader = reader;
      let init = new Uint8Array(0), buffer;
      try {
        for (;;) {
          const { value, done } = await reader.read();
          if (this.abort.signal.aborted) return;
          if (done) break;
          let bytes = value;
          if (!buffer) {
            const joined = new Uint8Array(init.length + value.length);
            joined.set(init); joined.set(value, init.length); init = joined;
            const mime = mimeFromInit(init);
            if (!mime) {
              if (init.length > 2 * 1024 * 1024) throw new Error('Media initialization is too large');
              continue;
            }
            if (!root.MediaSource.isTypeSupported(mime)) throw new Error('Unsupported media codec');
            buffer = this.media.addSourceBuffer(mime);
            // Keep an unfinished download streaming. A finite VOD duration
            // makes WebKit wait for a much larger playback reserve.
            this.media.duration = Infinity;
            if (this.options.duration > 0 && this.media.setLiveSeekableRange) {
              this.media.setLiveSeekableRange(0, this.options.duration);
            }
            bytes = init;
            init = null;
          }
          await this.append(buffer, bytes);
        }
        if (!buffer) throw new Error('Media ended before initialization');
        if (this.media.readyState === 'open') this.media.endOfStream();
        this.options.onComplete?.();
      } finally { reader.releaseLock(); }
    }

    dispose() {
      if (this.disposed) return;
      this.disposed = true;
      this.abort.abort();
      this.reader?.cancel().catch(() => {});
      root.URL.revokeObjectURL(this.objectURL);
    }
  }

  root.ProgressiveMediaSource = ProgressiveMediaSource;
  if (typeof module !== 'undefined') module.exports = ProgressiveMediaSource;
})(typeof window === 'undefined' ? globalThis : window);
