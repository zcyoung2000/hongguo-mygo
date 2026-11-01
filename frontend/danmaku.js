(() => {
  'use strict';

  const WINDOW_MS = 30000;
  const DURATION_MS = 9000;
  const FETCH_RUNWAY_MS = 3000;
  const MAX_FETCH_RATE = 4;
  const RETRY_MS = 3000;
  const KEEP_BEHIND_MS = 120000;
  const SEEK_LAUNCH_WINDOW_MS = 750;
  const SEEK_LAUNCH_MAX_ITEMS = 20;
  const LANE_GAP_PX = 18;

  class DanmakuController {
    constructor(options) {
      this.host = options.host;
      this.getVideo = options.getVideo || (() => null);
      this.getContext = options.getContext || (() => null);
      this.fetchPage = options.fetchPage;
      this.sendItem = options.sendItem || null;
      this.likeItem = options.likeItem || null;
      this.onError = options.onError || (() => {});

      this.enabled = localStorage.getItem('hongguo_danmaku_enabled') !== '0';
      this.opacity = clampNumber(parseFloat(localStorage.getItem('hongguo_danmaku_opacity')), 0.2, 1, 1);
      this.fontSize = clampNumber(parseFloat(localStorage.getItem('hongguo_danmaku_font_size')), 14, 40, 18);
      this.fontWeight = clampNumber(parseInt(localStorage.getItem('hongguo_danmaku_font_weight'), 10), 400, 900, 600);
      this.outline = localStorage.getItem('hongguo_danmaku_outline') !== '0';
      this.maxRows = clampNumber(parseInt(localStorage.getItem('hongguo_danmaku_rows'), 10), 0, 15, 0);

      this.layer = document.createElement('div');
      this.layer.className = 'danmaku-layer';
      this.host.appendChild(this.layer);
      this.resizeObserver = new ResizeObserver(() => this.clearActive());
      this.resizeObserver.observe(this.layer);

      this.items = [];
      this.seen = new Set();
      this.seenDisplay = new Set();
      this.ranges = [];
      this.active = [];
      this.loading = false;
      this.retryAt = 0;
      this.unavailable = false;
      this.requestToken = 0;
      this.contextKey = '';
      this.cursorMs = -1;
      this.lastTimeMs = -1;
      this.lastVideo = null;
      this.localSeq = 0;
      this.frame = 0;
      this.destroyed = false;
      this.tick = this.tick.bind(this);
      this.applySettings();
      this.schedule();
    }

    applySettings() {
      this.layer.style.opacity = String(this.opacity);
      this.layer.style.fontSize = `${this.fontSize}px`;
      this.layer.style.fontWeight = String(this.fontWeight);
      this.layer.classList.toggle('outline', this.outline);
      this.layer.classList.toggle('hidden', !this.enabled);
    }

    setEnabled(enabled) {
      this.enabled = Boolean(enabled);
      localStorage.setItem('hongguo_danmaku_enabled', this.enabled ? '1' : '0');
      if (!this.enabled) {
        this.requestToken++;
        this.loading = false;
        cancelAnimationFrame(this.frame);
        this.clearActive();
        this.layer.classList.add('hidden');
      } else {
        this.layer.classList.remove('hidden');
        this.cursorMs = -1;
        this.schedule();
      }
      this.applySettings();
    }

    setOpacity(value) {
      this.opacity = clampNumber(value, 0.2, 1, 1);
      localStorage.setItem('hongguo_danmaku_opacity', String(this.opacity));
      this.applySettings();
    }

    setFontSize(value) {
      this.fontSize = clampNumber(value, 14, 40, 18);
      localStorage.setItem('hongguo_danmaku_font_size', String(this.fontSize));
      this.clearActive();
      this.applySettings();
    }

    setFontWeight(value) {
      this.fontWeight = clampNumber(value, 400, 900, 600);
      localStorage.setItem('hongguo_danmaku_font_weight', String(this.fontWeight));
      this.applySettings();
    }

    setOutline(enabled) {
      this.outline = Boolean(enabled);
      localStorage.setItem('hongguo_danmaku_outline', this.outline ? '1' : '0');
      this.applySettings();
    }

    setRows(value) {
      this.maxRows = clampNumber(value, 0, 15, 0);
      localStorage.setItem('hongguo_danmaku_rows', String(this.maxRows));
      this.clearActive();
    }

    reset() {
      this.requestToken++;
      this.items = [];
      this.seen.clear();
      this.seenDisplay.clear();
      this.ranges = [];
      this.clearActive();
      this.loading = false;
      this.cursorMs = -1;
      this.lastTimeMs = -1;
      this.lastVideo = null;
      this.loading = false;
      this.retryAt = 0;
      this.unavailable = false;
    }

    destroy() {
      this.destroyed = true;
      cancelAnimationFrame(this.frame);
      this.resizeObserver.disconnect();
      this.reset();
      this.layer.remove();
    }

    schedule() {
      if (this.destroyed || !this.enabled) return;
      this.frame = requestAnimationFrame(this.tick);
    }

    tick() {
      const video = this.getVideo();
      const context = this.enabled ? this.getContext() : null;
      const key = context ? `${context.seriesId}:${context.episodeId}` : '';

      if (key !== this.contextKey) {
        this.contextKey = key;
        this.reset();
      }

      if (!video || !context) {
        this.clearActive();
        this.schedule();
        return;
      }
      if (video !== this.lastVideo) {
        this.lastVideo = video;
        this.cursorMs = -1;
        this.lastTimeMs = -1;
        this.clearActive();
      }

      const timeMs = Math.max(0, Math.floor((video.currentTime || 0) * 1000));
      if (this.lastTimeMs >= 0 && (timeMs < this.lastTimeMs - 800 || timeMs > this.lastTimeMs + 3000)) {
        this.clearActive();
        this.cursorMs = Math.max(0, timeMs - 400);
      }
      this.lastTimeMs = timeMs;

      if (this.enabled && Date.now() >= this.retryAt) {
        void this.fetchWindow(context, timeMs);
      }

      if (video.paused) {
        this.setActivePlaying(false);
      } else {
        this.launchUntil(context, timeMs);
        this.setActivePlaying(true);
      }
      this.pruneOld(timeMs);
      this.schedule();
    }

    async fetchWindow(context, timeMs) {
      const startMs = this.nextFetchStart(timeMs);
      if (startMs === null || this.loading || !this.fetchPage) return;

      const token = ++this.requestToken;
      this.loading = true;
      try {
        const page = await this.fetchPage(context.seriesId, context.episodeId, startMs, WINDOW_MS);
        if (token !== this.requestToken) return;
        this.addRange(startMs, Number(page?.nextMs) > startMs ? Number(page.nextMs) : startMs + WINDOW_MS);
        const known = new Set(this.items.map(item => item.id));
        for (const item of (page?.items || [])) {
          if (!item || !item.id || this.seen.has(item.id) || known.has(item.id)) continue;
          this.seen.add(item.id);
          this.items.push({
            id: item.id,
            userId: item.userId || '',
            text: item.text || '',
            timeMs: Number(item.timeMs) || 0,
            color: item.color || '#ffffff',
            likeCount: Number(item.likeCount) || 0,
            liked: Boolean(item.liked),
            showDigg: Boolean(item.showDigg),
            showAddOne: Boolean(item.showAddOne),
            local: false,
          });
        }
        this.items.sort((a, b) => a.timeMs - b.timeMs);
        this.unavailable = false;
      } catch (err) {
        if (token === this.requestToken) {
          this.unavailable = true;
          this.retryAt = Date.now() + RETRY_MS;
          this.onError(err);
        }
      } finally {
        if (token === this.requestToken) this.loading = false;
      }
    }

    nextFetchStart(timeMs) {
      const runwayMs = FETCH_RUNWAY_MS * Math.min(MAX_FETCH_RATE, Math.max(1, this.lastVideo?.playbackRate || 1));
      const direct = this.findRange(timeMs);
      if (direct) {
        return direct[1] - timeMs <= runwayMs ? direct[1] : null;
      }
      if (this.ranges.length === 0) return Math.max(0, timeMs - 500);
      const next = this.ranges.find(([start, end]) => end > timeMs);
      // If playback crossed a range while a request was still in flight, the
      // old runway can be gone by the next frame. Resume from playback instead
      // of permanently starving the stream behind the last range.
      if (!next) return Math.max(0, timeMs - 500);
      return next && next[0] - timeMs <= runwayMs ? next[0] : null;
    }

    findRange(timeMs) {
      return this.ranges.find(([start, end]) => timeMs >= start && timeMs < end) || null;
    }

    addRange(start, end) {
      if (end <= start) return;
      this.ranges.push([start, end]);
      this.ranges.sort((a, b) => a[0] - b[0]);
      const merged = [];
      for (const range of this.ranges) {
        const last = merged[merged.length - 1];
        if (last && range[0] <= last[1]) {
          last[1] = Math.max(last[1], range[1]);
        } else {
          merged.push([...range]);
        }
      }
      this.ranges = merged;
    }

    launchUntil(context, timeMs) {
      const previous = this.cursorMs < 0 ? -1 : this.cursorMs;
      this.cursorMs = timeMs;
      if (timeMs - previous > SEEK_LAUNCH_WINDOW_MS) return;
      this.items
        .filter(item => item.timeMs > previous && item.timeMs <= timeMs)
        .slice(0, SEEK_LAUNCH_MAX_ITEMS)
        .forEach(item => this.launch(item));
    }

    launch(item) {
      if (this.seenDisplay.has(item.id)) return;
      const containerWidth = this.layer.clientWidth || this.host.clientWidth || 640;
      const laneHeight = this.fontSize + 10;
      const element = document.createElement('div');
      element.className = 'danmaku-item';
      if (item.local) element.classList.add('own');
      if (item.liked) element.classList.add('liked');
      element.textContent = item.text;
      element.style.color = item.color || '#ffffff';
      element.style.transform = `translate3d(${containerWidth}px, 0, 0)`;
      this.layer.appendChild(element);

      let itemWidth = element.offsetWidth;
      if (!itemWidth) itemWidth = Math.min(containerWidth, this.fontSize * Array.from(item.text).length);
      const lane = this.selectLane(itemWidth, containerWidth);
      if (lane < 0) {
        element.remove();
        return;
      }

      element.style.transform = `translate3d(${containerWidth}px, ${lane * laneHeight}px, 0)`;
      const canLike = Boolean(this.likeItem && !item.local && item.showDigg !== false);
      const clickHandler = async event => {
        event.stopPropagation();
        if (!canLike) return;
        try {
          const liked = await this.likeItem(item, !item.liked);
          if (liked === undefined) return;
          item.liked = liked;
          item.likeCount = Math.max(0, item.likeCount + (liked ? 1 : -1));
          element.classList.toggle('liked', item.liked);
        } catch (err) {
          this.onError(err);
        }
      };
      element.addEventListener('click', clickHandler);
      element.addEventListener('keydown', event => {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        event.preventDefault();
        void clickHandler(event);
      });
      if (canLike) {
        element.setAttribute('role', 'button');
        element.setAttribute('aria-label', `点赞弹幕：${item.text}`);
        element.tabIndex = -1;
        element.title = '点击点赞';
      } else if (item.local) {
        element.title = '我的弹幕';
      }

      const animation = element.animate([
        { transform: `translate3d(${containerWidth}px, ${lane * laneHeight}px, 0)` },
        { transform: `translate3d(${-itemWidth}px, ${lane * laneHeight}px, 0)` },
      ], { duration: DURATION_MS, easing: 'linear', fill: 'forwards' });

      const active = {
        id: item.id,
        element,
        animation,
        lane,
        containerWidth,
        itemWidth,
        clickHandler,
      };
      this.active.push(active);
      this.seenDisplay.add(item.id);
      animation.finished.then(() => this.removeActive(active)).catch(() => {});
    }

    selectLane(itemWidth, containerWidth) {
      const height = this.fontSize + 10;
      const available = Math.max(1, Math.floor(this.layer.clientHeight / height));
      const rows = this.maxRows > 0 ? Math.min(this.maxRows, available) : available;
      for (let lane = 0; lane < rows; lane++) {
        const occupied = this.active.some(item => (
          item.lane === lane &&
          this.willCollide(item, itemWidth, containerWidth)
        ));
        if (!occupied) return lane;
      }
      return -1;
    }

    willCollide(previous, nextWidth, stageWidth) {
      const previousWidth = Number(previous.itemWidth) || previous.element.offsetWidth || nextWidth;
      const elapsed = Math.max(0, Number(previous.animation.currentTime) || 0);
      if (elapsed >= DURATION_MS) return false;

      // Check both ends of the linear distance between the two comments. The
      // first prevents an immediate overlap at the right edge; the second
      // prevents a longer/faster new comment from catching the old one later.
      const previousRight = (stageWidth + previousWidth) * (1 - elapsed / DURATION_MS);
      if (stageWidth - previousRight < LANE_GAP_PX) return true;

      const remaining = DURATION_MS - elapsed;
      const nextLeftAfterPreviousExits = stageWidth - (stageWidth + nextWidth) * remaining / DURATION_MS;
      return nextLeftAfterPreviousExits < LANE_GAP_PX;
    }

    removeActive(active) {
      const index = this.active.indexOf(active);
      if (index >= 0) this.active.splice(index, 1);
      active.element.removeEventListener('click', active.clickHandler);
      active.element.remove();
    }

    clearActive() {
      for (const active of [...this.active]) this.removeActive(active);
      this.seenDisplay = new Set();
    }

    setActivePlaying(playing) {
      for (const active of this.active) {
        if (playing) active.animation.play();
        else active.animation.pause();
      }
    }

    pruneOld(timeMs) {
      if (this.items.length === 0) return;
      const cutoff = timeMs - KEEP_BEHIND_MS;
      this.items = this.items.filter(item => item.local || item.timeMs >= cutoff);
    }

    discardLocal(item) {
      this.items = this.items.filter(candidate => candidate !== item);
      this.seenDisplay.delete(item.id);
      for (const active of [...this.active]) {
        if (active.id === item.id) this.removeActive(active);
      }
    }

    async send(text, timeMs) {
      text = String(text || '').trim();
      if (!text || !this.sendItem) return null;
      const context = this.getContext();
      if (!context) return null;
      const local = {
        id: `local-${Date.now()}-${++this.localSeq}`,
        userId: 'local',
        text,
        timeMs: Math.max(0, Math.floor(timeMs)),
        color: '#ffffff',
        likeCount: 0,
        liked: false,
        showDigg: false,
        showAddOne: false,
        local: true,
      };
      this.items.push(local);
      this.items.sort((a, b) => a.timeMs - b.timeMs);
      this.launch(local);
      let result;
      try {
        result = await this.sendItem(context.seriesId, context.episodeId, text, local.timeMs);
      } catch (err) {
        this.discardLocal(local);
        throw err;
      }
      if (result?.id && this.sendItem) {
        const localID = local.id;
        local.id = result.id;
        local.showDigg = true;
        local.local = false;
        local.userId = result.userId || local.userId;
        this.seen.add(local.id);
        this.seenDisplay.delete(localID);
        this.seenDisplay.add(local.id);
      }
      return result || local;
    }
  }

  function clampNumber(value, min, max, fallback) {
    if (!Number.isFinite(value)) return fallback;
    return Math.min(max, Math.max(min, value));
  }

  window.DanmakuController = DanmakuController;
})();
