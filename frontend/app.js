// ==========================================================================
// Hongguo Short Drama - App Controller
// Pure native Go backend IPC via window.mygo.call
// ==========================================================================

const DEFAULT_POSTER = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyMDAiIGhlaWdodD0iMjgwIiB2aWV3Qm94PSIwIDAgMjAwIDI4MCI+PHJlY3Qgd2lkdGg9IjIwMCIgaGVpZ2h0PSIyODAiIGZpbGw9IiMxODE5MjAiLz48Y2lyY2xlIGN4PSIxMDAiIGN5PSIxMTAiIHI9IjI4IiBmaWxsPSIjMjUyNzMzIi8+PHBvbHlnb24gcG9pbnRzPSI5NCw5OCAxMTIsMTEwIDk0LDEyMiIgZmlsbD0iIzY2NiIvPjx0ZXh0IHg9IjEwMCIgeT0iMTY1IiBmaWxsPSIjNjY2IiBmb250LXNpemU9IjEyIiBmb250LWZhbWlseT0iLWFwcGxlLXN5c3RlbSxTaW5nSGVpU0MsU2Vnb2UgVUkiIHRleHQtYW5jaG9yPSJtaWRkbGUiPuaaguaXoOWBgeeJhzjwL3RleHQ+PC9zdmc+";
window.DEFAULT_POSTER = DEFAULT_POSTER;
window.handleImgError = function(img) {
  if (img) {
    img.onerror = null;
    img.src = DEFAULT_POSTER;
  }
};

function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function safeImageSrc(value) {
  const url = String(value || '');
  if (/^https:\/\//i.test(url) || /^http:\/\/127\.0\.0\.1:/i.test(url)) {
    return escapeHtml(url);
  }
  return escapeHtml(DEFAULT_POSTER);
}

function safeImageCSS(value) {
  const url = String(value || '');
  if (!/^https:\/\//i.test(url) && !/^http:\/\/127\.0\.0\.1:/i.test(url)) return 'none';
  const encoded = url.replace(/["'()\\\r\n]/g, char => encodeURIComponent(char));
  return `url("${encoded}")`;
}

const MACOS_SPINNER_SVG = `
  <svg class="macos-spinner" viewBox="0 0 32 32" width="32" height="32" fill="none" aria-hidden="true">
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="1.0" transform="rotate(0 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.93" transform="rotate(30 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.86" transform="rotate(60 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.8" transform="rotate(90 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.73" transform="rotate(120 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.66" transform="rotate(150 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.59" transform="rotate(180 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.52" transform="rotate(210 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.45" transform="rotate(240 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.39" transform="rotate(270 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.32" transform="rotate(300 16 16)" />
    <line x1="16" y1="3" x2="16" y2="8.5" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" opacity="0.25" transform="rotate(330 16 16)" />
  </svg>
`;

const LOADER_GRAPHIC = MACOS_SPINNER_SVG;

function renderDramaGridSkeleton(count = 12) {
  let html = '';
  for (let i = 0; i < count; i++) {
    html += `
      <div class="drama-card drama-card-skeleton" aria-hidden="true">
        <div class="card-poster-wrap skeleton-box">
          <div class="skeleton-badge"></div>
        </div>
        <div class="card-info">
          <div class="skeleton-line skeleton-title"></div>
          <div class="skeleton-line skeleton-heat"></div>
        </div>
      </div>
    `;
  }
  return html;
}

function renderFilterPanelSkeleton() {
  return `
    <div class="filter-panel-skeleton" aria-hidden="true">
      <div class="filter-row skeleton-row">
        <span class="filter-label skeleton-box"></span>
        <div class="filter-chips skeleton-chips">
          <div class="filter-chip-skeleton skeleton-box"></div>
          <div class="filter-chip-skeleton skeleton-box"></div>
          <div class="filter-chip-skeleton skeleton-box"></div>
        </div>
      </div>
      <div class="filter-row skeleton-row">
        <span class="filter-label skeleton-box"></span>
        <div class="filter-chips skeleton-chips">
          <div class="filter-chip-skeleton skeleton-box"></div>
          <div class="filter-chip-skeleton skeleton-box"></div>
          <div class="filter-chip-skeleton skeleton-box"></div>
          <div class="filter-chip-skeleton skeleton-box"></div>
          <div class="filter-chip-skeleton skeleton-box"></div>
        </div>
      </div>
      <div class="filter-row skeleton-row">
        <span class="filter-label skeleton-box"></span>
        <div class="filter-chips skeleton-chips">
          <div class="filter-chip-skeleton skeleton-box"></div>
          <div class="filter-chip-skeleton skeleton-box"></div>
          <div class="filter-chip-skeleton skeleton-box"></div>
          <div class="filter-chip-skeleton skeleton-box"></div>
        </div>
      </div>
    </div>
  `;
}

function renderRankListSkeleton(count = 8) {
  let html = '';
  for (let i = 1; i <= count; i++) {
    html += `
      <div class="rank-item-card rank-item-skeleton" aria-hidden="true">
        <div class="rank-position-skeleton skeleton-box"></div>
        <div class="rank-poster-skeleton skeleton-box"></div>
        <div class="rank-item-info">
          <div class="skeleton-line" style="width: 58%; height: 16px;"></div>
          <div class="skeleton-line" style="width: 32%; height: 12px;"></div>
        </div>
        <div class="rank-btn-skeleton skeleton-box"></div>
      </div>
    `;
  }
  return html;
}

function renderDetailSkeleton() {
  return `
    <div class="detail-hero detail-hero-skeleton">
      <div class="detail-poster-wrap skeleton-box"></div>
      <div class="detail-hero-body">
        <div class="skeleton-line detail-skeleton-title"></div>
        <div class="detail-stats-row">
          <div class="detail-stat-skeleton skeleton-box"></div>
          <div class="detail-stat-skeleton skeleton-box"></div>
          <div class="detail-stat-skeleton skeleton-box"></div>
          <div class="detail-stat-skeleton skeleton-box"></div>
        </div>
        <div class="detail-tags">
          <div class="detail-tag-skeleton skeleton-box"></div>
          <div class="detail-tag-skeleton skeleton-box"></div>
          <div class="detail-tag-skeleton skeleton-box"></div>
          <div class="detail-tag-skeleton skeleton-box"></div>
        </div>
        <div class="detail-skeleton-desc-lines">
          <div class="skeleton-line" style="width: 95%; height: 13px;"></div>
          <div class="skeleton-line" style="width: 88%; height: 13px;"></div>
          <div class="skeleton-line" style="width: 62%; height: 13px;"></div>
        </div>
        <div class="detail-actions">
          <div class="skeleton-box" style="width: 140px; height: 38px; border-radius: 19px;"></div>
          <div class="skeleton-box" style="width: 88px; height: 38px; border-radius: 19px;"></div>
        </div>
      </div>
    </div>

    <div class="detail-episodes-section detail-episodes-skeleton">
      <div class="detail-section-heading">
        <div class="skeleton-line" style="width: 50px; height: 18px;"></div>
        <div class="skeleton-line" style="width: 44px; height: 14px;"></div>
      </div>
      <div class="detail-episode-grid">
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
        <div class="ep-btn-skeleton skeleton-box"></div>
      </div>
    </div>

    <section class="detail-collection">
      <div class="detail-section-heading">
        <div class="skeleton-line" style="width: 80px; height: 18px;"></div>
      </div>
      <div class="drama-grid">
        ${renderDramaGridSkeleton(6)}
      </div>
    </section>
  `;
}

function renderCommentSkeleton(count = 4) {
  let html = '';
  for (let i = 0; i < count; i++) {
    html += `
      <div class="comment-item comment-item-skeleton" aria-hidden="true">
        <div class="comment-avatar skeleton-box" style="width:34px;height:34px;border-radius:50%;flex-shrink:0;"></div>
        <div style="flex:1;min-width:0;">
          <div style="display:flex;gap:10px;align-items:center;margin-bottom:8px;">
            <div class="skeleton-line" style="width:28%;height:13px;border-radius:3px;"></div>
            <div class="skeleton-line" style="width:16%;height:11px;border-radius:3px;"></div>
          </div>
          <div class="skeleton-line" style="width:92%;height:13px;margin-bottom:6px;border-radius:3px;"></div>
          <div class="skeleton-line" style="width:58%;height:13px;margin-bottom:8px;border-radius:3px;"></div>
          <div class="skeleton-line" style="width:36px;height:12px;border-radius:3px;"></div>
        </div>
      </div>
    `;
  }
  return html;
}

function showFeedSkeleton(show) {
  const skel = document.getElementById('feed-skeleton');
  if (!skel) return;
  if (show) {
    skel.style.display = 'block';
    skel.style.opacity = '1';
    skel.classList.add('is-active');
  } else {
    skel.style.opacity = '0';
    setTimeout(() => {
      if (!skel.classList.contains('is-active')) {
        skel.style.display = 'none';
      }
    }, 280);
    skel.classList.remove('is-active');
  }
}

function createLoader(text = '正在加载...') {
  return `
    <div class="video-loader" role="status" aria-label="${escapeHtml(text)}">
      <div class="macos-loader-hud">
        ${MACOS_SPINNER_SVG}
        ${text ? `<span class="macos-loader-text">${escapeHtml(text)}</span>` : ''}
      </div>
    </div>
  `;
}

// IPC responses must be ordinary mutable JS objects. A defensive clone keeps
// renderer state isolated from the bridge and avoids WebKit's read-only
// property errors when a bridge value is retained and later updated.
function cloneIPCValue(value) {
  if (!value || typeof value !== 'object') return value;
  if (typeof structuredClone === 'function') {
    try {
      return structuredClone(value);
    } catch (_) {
      // Fall through for non-JSON host objects.
    }
  }
  return JSON.parse(JSON.stringify(value));
}

let feedSlots = [];
let feedSlotCurr = 0;
let feedSlotNext = 1;
let feedSlotPrev = 2;
let feedActiveVideo = null;
let feedPlayToken = 0;
let feedPlayPromise = null;
let nativeScrollGestureSeen = false;
let feedWarmupToken = 0;
let feedWarmupTimers = [];
const feedSwitchMetrics = [];
const feedMetricQueue = [];
let feedMetricFlushTimer = null;
let feedMetricFlushInFlight = false;
let feedTimeSyncHandle = 0;
let feedTimeSyncHandleType = '';
let feedTimeSyncVideo = null;
let feedTimeSyncLastAt = 0;
let feedTimeSyncGeneration = 0;

function updateFeedPlayButtonState(video = null) {
  const feedVideo = video || getFeedVideo();
  const playBtn = document.getElementById('feed-play');
  const iconPlay = playBtn?.querySelector('.icon-play');
  const iconPause = playBtn?.querySelector('.icon-pause');
  const isPlaying = Boolean(feedVideo && !feedVideo.paused && !feedVideo.ended);
  if (iconPlay) iconPlay.style.display = isPlaying ? 'none' : 'block';
  if (iconPause) iconPause.style.display = isPlaying ? 'block' : 'none';
}

function stopFeedMediaTimeSync() {
  if (!feedTimeSyncHandle && !feedTimeSyncVideo) return;
  feedTimeSyncGeneration++;
  if (
    feedTimeSyncHandle &&
    feedTimeSyncHandleType === 'video-frame' &&
    typeof feedTimeSyncVideo?.cancelVideoFrameCallback === 'function'
  ) {
    feedTimeSyncVideo.cancelVideoFrameCallback(feedTimeSyncHandle);
  } else if (feedTimeSyncHandleType === 'animation-frame') {
    cancelAnimationFrame(feedTimeSyncHandle);
  }
  feedTimeSyncHandle = 0;
  feedTimeSyncHandleType = '';
  feedTimeSyncVideo = null;
}

function syncActiveFeedTimeline(video = null) {
  const feedVideo = video || getFeedVideo();
  updateFeedMediaTime(feedVideo);
  updatePlaybackBufferIndicator(
    feedVideo,
    document.getElementById('feed-progress-buffer'),
    feedMediaDuration(feedVideo)
  );
}

function scheduleFeedMediaTimeSync() {
  const feedVideo = getFeedVideo();
  if (!feedVideo) {
    stopFeedMediaTimeSync();
    return;
  }
  if (feedTimeSyncVideo === feedVideo && feedTimeSyncHandle) return;

  if (feedTimeSyncVideo !== feedVideo) {
    stopFeedMediaTimeSync();
    feedTimeSyncVideo = feedVideo;
    feedTimeSyncLastAt = performance.now();
  } else {
    feedTimeSyncHandle = 0;
    feedTimeSyncHandleType = '';
  }
  const generation = feedTimeSyncGeneration;

  const tick = timestamp => {
    // A generation also invalidates callbacks when a WebKit implementation
    // exposes rVFC but not cancelVideoFrameCallback.
    if (generation !== feedTimeSyncGeneration) return;
    feedTimeSyncHandle = 0;
    feedTimeSyncHandleType = '';
    if (feedTimeSyncVideo !== feedVideo || feedVideo !== getFeedVideo()) {
      feedTimeSyncVideo = null;
      return;
    }

    // Browsers emit timeupdate at implementation-defined intervals. Syncing
    // with presented video frames keeps the shared chrome tied to media time.
    if (timestamp - feedTimeSyncLastAt >= 200) {
      syncActiveFeedTimeline(feedVideo);
      feedTimeSyncLastAt = timestamp;
    }
    if (feedVideo.paused) {
      feedTimeSyncVideo = null;
      return;
    }
    scheduleFeedMediaTimeSync();
  };

  if (typeof feedVideo.requestVideoFrameCallback === 'function') {
    feedTimeSyncHandleType = 'video-frame';
    feedTimeSyncHandle = feedVideo.requestVideoFrameCallback(tick);
  } else {
    feedTimeSyncHandleType = 'animation-frame';
    feedTimeSyncHandle = requestAnimationFrame(tick);
  }
}

// The shared feed chrome is a projection of one active media element. Keep all
// activation paths synchronized through this single state transition.
function setActiveFeedSlot(slot, { force = false } = {}) {
  if (!slot) return;
  if (!force && feedSlots[feedSlotCurr] === slot && feedActiveVideo === slot.video) return;

  feedSlotCurr = slot.id;
  feedActiveVideo = slot.video;
  syncFeedMediaChrome();
}

function syncFeedMediaChrome() {
  const feedVideo = getFeedVideo();
  syncActiveFeedTimeline(feedVideo);
  updateFeedPlayIndicator();
  updateFeedPlayButtonState(feedVideo);
  scheduleFeedMediaTimeSync();
}

function initFeedVideoPool() {
  const ids = [0, 1, 2];
  feedSlots = ids.map(i => {
    const layer = document.getElementById(`feed-slide-${i}`);
    const video = document.getElementById(`feed-video-${i}`);
    const poster = document.getElementById(`feed-poster-${i}`);
    if (video) {
      video.addEventListener('loadedmetadata', () => syncFeedVideoOrientation(video));
      video.addEventListener('durationchange', () => {
        if (!Number.isFinite(video.duration) || video.duration <= 0) return;
        const slot = feedSlots.find(candidate => candidate.video === video);
        if (slot) slot.durationSec = video.duration;
        updateFeedMediaTime(video);
      });
      attachPlaybackVideoFit(video);
      ['enterpictureinpicture', 'leavepictureinpicture', 'webkitpresentationmodechanged'].forEach(evt => {
        video.addEventListener(evt, updatePiPButtons);
      });
    }
    const slot = {
      id: i,
      layer,
      video,
      poster,
      itemIndex: -1,
      seriesId: '',
      episodeNum: 1,
      episodeId: '',
      quality: '',
      streamUrl: '',
      durationSec: 0,
      fullStreamUrl: '',
      progressiveStreamUrl: '',
      hlsStreamUrl: '',
      usedFullFallback: false,
      usedProgressiveFallback: false,
      usedHLSFallback: false,
      bufferReady: false,
      preparedAt: 0,
      expiresAt: 0,
      totalEpisodes: 1,
      episodes: [],
      commentCount: 0,
      loaded: false,
      ready: false,
      loadToken: 0,
      preparing: null,
      firstFramePriming: null,
      isPriming: false,
      autoplayRetryTimer: null,
      sourceRetryTimer: null,
      failureHintTimer: null,
      metricPreparedAt: 0,
      metricFirstFrameAt: 0,
      metricFirstFrameSource: '',
      metricReadyAt: 0,
      metricRebufferCount: 0,
      metricRebufferMS: 0,
      metricRebufferStartedAt: 0,
      metricCacheHit: false,
      metricStreamMode: '',
      metricUsedFullFallback: false
    };
    attachFeedVideoMetrics(slot);
    attachFeedPosterVisibility(slot);
    scheduleFeedFrameMetric(slot);
    return slot;
  });

  feedSlotNext = 1;
  feedSlotPrev = 2;
  const incomingPoster = document.getElementById('feed-incoming-poster');
  if (incomingPoster) incomingPoster.style.backgroundImage = 'none';
  setActiveFeedSlot(feedSlots[0], { force: true });
}

function attachFeedVideoMetrics(slot) {
  if (!slot?.video) return;
  slot.video.addEventListener('loadeddata', () => {
    if (slot.metricPreparedAt && !slot.metricFirstFrameAt) {
      slot.metricFirstFrameAt = performance.now();
      slot.metricFirstFrameSource = 'loaded-data';
    }
  });
  ['canplay', 'canplaythrough', 'loadeddata', 'progress'].forEach(event => {
    slot.video.addEventListener(event, () => {
      if (slot.metricPreparedAt && !slot.metricReadyAt && feedVideoHasPlaybackBuffer(slot.video)) {
        slot.metricReadyAt = performance.now();
      }
    });
  });
  slot.video.addEventListener('waiting', () => {
    if (!slot.metricPreparedAt || slot.metricRebufferStartedAt) return;
    if (slot.isPriming) return;
    slot.metricRebufferStartedAt = performance.now();
  });
  slot.video.addEventListener('playing', () => {
    if (!slot.metricPreparedAt) return;
    if (!slot.metricFirstFrameAt) slot.metricFirstFrameAt = performance.now();
    if (slot.metricRebufferStartedAt) {
      slot.metricRebufferMS += Math.max(0, performance.now() - slot.metricRebufferStartedAt);
      slot.metricRebufferStartedAt = 0;
      slot.metricRebufferCount++;
    }
  });
  slot.video.addEventListener('pause', () => {
    if (slot.isPriming) return;
    if (!slot.metricRebufferStartedAt || slot.video.ended) return;
    slot.metricRebufferMS += Math.max(0, performance.now() - slot.metricRebufferStartedAt);
    slot.metricRebufferStartedAt = 0;
    slot.metricRebufferCount++;
  });
}

function feedVideoHasFirstFrame(video) {
  return Boolean(
    video?.src &&
    video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA &&
    video.videoWidth > 0
  );
}

function syncFeedPosterVisibility(slot) {
  if (!slot?.poster || !slot.video) return;
  slot.poster.classList.toggle('hidden', feedVideoHasFirstFrame(slot.video));
}

function attachFeedPosterVisibility(slot) {
  if (!slot?.video) return;
  ['loadeddata', 'canplay', 'canplaythrough', 'playing', 'progress'].forEach(event => {
    slot.video.addEventListener(event, () => syncFeedPosterVisibility(slot));
  });
  slot.video.addEventListener('error', () => syncFeedPosterVisibility(slot));
}

function hideFeedPosterImmediately(slot) {
  if (!slot?.poster) return;
  slot.poster.style.transition = 'none';
  slot.poster.classList.add('hidden');
  requestAnimationFrame(() => {
    if (slot.poster) slot.poster.style.transition = '';
  });
}

async function primeFeedSlotFirstFrame(slot) {
  if (!slot?.video?.src || feedVideoHasFirstFrame(slot.video)) return;
  const video = slot.video;
  const loadToken = slot.loadToken;
  slot.isPriming = true;
  slot.metricRebufferStartedAt = 0;
  video.muted = true;

  const firstFrame = new Promise(resolve => {
    const finish = () => {
      video.removeEventListener('loadeddata', finish);
      video.removeEventListener('playing', finish);
      resolve();
    };
    video.addEventListener('loadeddata', finish);
    video.addEventListener('playing', finish);
    setTimeout(finish, 1800);
  });

  try {
    await video.play();
    await firstFrame;
  } catch (_) {
    // Some WebKit versions defer loading offscreen videos until play is called;
    // failing here is safe because the cover remains the fallback.
  } finally {
    const isCurrent = feedSlots[feedSlotCurr] === slot;
    if (loadToken === slot.loadToken && (!isCurrent || state.feed.userPaused || state.currentView !== 'recommend')) {
      video.pause();
      if (!isCurrent && video.currentTime > 0 && Number.isFinite(video.currentTime)) video.currentTime = 0;
    }
    if (loadToken === slot.loadToken) {
      const finishPriming = () => {
        if (loadToken !== slot.loadToken) return;
        slot.isPriming = false;
        syncFeedPosterVisibility(slot);
        slot.firstFramePriming = null;
      };
      // WebKit can dispatch pause asynchronously; keep the priming guard active
      // through that turn so background first-frame work is not replay rebuffer.
      if (!isCurrent || state.feed.userPaused || state.currentView !== 'recommend') setTimeout(finishPriming, 0);
      else finishPriming();
    }
  }
}

function scheduleFeedFrameMetric(slot) {
  if (!slot?.video?.requestVideoFrameCallback || slot.video.dataset.frameMetricScheduled) return;
  slot.video.dataset.frameMetricScheduled = '1';
  slot.video.requestVideoFrameCallback(() => {
    delete slot.video.dataset.frameMetricScheduled;
    if (!slot.metricPreparedAt) return;
    slot.metricFirstFrameAt = performance.now();
    slot.metricFirstFrameSource = 'rendered-frame';
    if (!slot.metricReadyAt && feedVideoHasPlaybackBuffer(slot.video)) {
      slot.metricReadyAt = performance.now();
    }
    scheduleFeedFrameMetric(slot);
  });
}

function getFeedVideo() {
  return feedSlots[feedSlotCurr]?.video || feedActiveVideo || document.getElementById('feed-video-0');
}

function feedSlotForVideo(video) {
  return feedSlots.find(slot => slot.video === video) || null;
}

// HLS event playlists expose Infinity until ffmpeg writes ENDLIST. Backend
// metadata gives the real duration while the feed is still converting.
function feedMediaDuration(video) {
  if (Number.isFinite(video?.duration) && video.duration > 0) return video.duration;
  const slot = feedSlotForVideo(video);
  const metadataDuration = Number(slot?.durationSec || 0);
  return metadataDuration > 0 ? metadataDuration : 0;
}

function updateFeedMediaTime(video) {
  if (!video || video !== getFeedVideo()) return;
  const duration = feedMediaDuration(video);
  const position = Number.isFinite(video.currentTime) ? Math.max(0, video.currentTime) : 0;
  const progressCurrent = document.getElementById('feed-progress-current');
  const timeDisplay = document.getElementById('feed-time-display');
  if (progressCurrent) progressCurrent.style.width = `${duration ? (position / duration) * 100 : 0}%`;
  if (timeDisplay) timeDisplay.textContent = `${formatTime(position)} / ${formatTime(duration)}`;
}

function prepareFeedStartPosition(slot, dir) {
  if (!slot?.video || !Number.isFinite(slot.video.currentTime)) return;

  const duration = feedMediaDuration(slot.video);
  const storedPosition = Number(state.feed.playbackPositions.get(
    feedPlaybackKey(slot.seriesId, slot.episodeNum, slot.quality)
  ));
  const hasResumePosition = Number.isFinite(storedPosition) &&
    storedPosition > 3 &&
    (!duration || storedPosition < duration - 5);

  let intendedPosition = 0;
  if (hasResumePosition || dir < 0) {
    intendedPosition = hasResumePosition
      ? storedPosition
      : Math.max(0, Number(slot.video.currentTime) || 0);
  }
  if (duration > 0) intendedPosition = Math.min(intendedPosition, duration - 0.5);
  intendedPosition = Math.max(0, intendedPosition);

  if (Math.abs(slot.video.currentTime - intendedPosition) > 0.05) {
    slot.video.currentTime = intendedPosition;
  }
}

function feedAssetKey(seriesId, episodeNum, quality) {
  return `${seriesId || ''}:${episodeNum || 1}:${quality || state.feed.quality}`;
}

function feedPlaybackKey(seriesId, episodeNum, quality) {
  return `${seriesId || ''}:${episodeNum || 1}:${quality || state.feed.quality}`;
}

function rememberFeedPlaybackPosition(key, position) {
  state.feed.playbackPositions.delete(key);
  state.feed.playbackPositions.set(key, position);
  if (state.feed.playbackPositions.size <= 128) return;

  const oldestKey = state.feed.playbackPositions.keys().next().value;
  if (oldestKey !== undefined) state.feed.playbackPositions.delete(oldestKey);
}

function rememberFeedPlayback(key, playback) {
  state.feed.preloadMap.delete(key);
  state.feed.preloadMap.set(key, playback);
  if (state.feed.preloadMap.size <= 128) return;

  const oldestKey = state.feed.preloadMap.keys().next().value;
  if (oldestKey !== undefined) state.feed.preloadMap.delete(oldestKey);
}

function clearFeedSlot(slot) {
  if (!slot) return;
  slot.loadToken++;
  slot.preparing = null;
  slot.itemIndex = -1;
  slot.seriesId = '';
  slot.episodeNum = 1;
  slot.episodeId = '';
  slot.quality = '';
  slot.durationSec = 0;
  slot.streamUrl = '';
  slot.fullStreamUrl = '';
  slot.progressiveStreamUrl = '';
  slot.hlsStreamUrl = '';
  slot.usedFullFallback = false;
  slot.usedProgressiveFallback = false;
  slot.usedHLSFallback = false;
  slot.bufferReady = false;
  slot.preparedAt = 0;
  slot.expiresAt = 0;
  slot.totalEpisodes = 1;
  slot.episodes = [];
  slot.commentCount = 0;
  slot.loaded = false;
  slot.ready = false;
  slot.firstFramePriming = null;
  slot.isPriming = false;
  if (slot.autoplayRetryTimer) {
    clearTimeout(slot.autoplayRetryTimer);
    slot.autoplayRetryTimer = null;
  }
  if (slot.sourceRetryTimer) {
    clearTimeout(slot.sourceRetryTimer);
    slot.sourceRetryTimer = null;
  }
  if (slot.failureHintTimer) {
    clearTimeout(slot.failureHintTimer);
    slot.failureHintTimer = null;
  }
  slot.metricPreparedAt = 0;
  slot.metricFirstFrameAt = 0;
  slot.metricFirstFrameSource = '';
  slot.metricReadyAt = 0;
  slot.metricRebufferCount = 0;
  slot.metricRebufferMS = 0;
  slot.metricRebufferStartedAt = 0;
  slot.metricCacheHit = false;
  slot.metricStreamMode = '';
  slot.metricUsedFullFallback = false;

  if (!slot.video) return;
  slot.video.onloadeddata = null;
  slot.video.onloadedmetadata = null;
  slot.video.oncanplay = null;
  slot.video.oncanplaythrough = null;
  slot.video.onseeked = null;
  slot.video.onplaying = null;
  slot.video.onerror = null;
  slot.video.pause();
  slot.video.removeAttribute('src');
  slot.video.load();
  slot.video.dataset.seriesId = '';
  slot.video.dataset.episodeNum = '';
  resetPlaybackVideoFit(slot.video);
  delete slot.video.dataset.frameMetricScheduled;
  delete slot.video.dataset.temporaryMute;
  if (slot.poster) {
    slot.poster.style.backgroundImage = 'none';
    slot.poster.classList.remove('hidden');
  }
}

function feedSlotMatches(slot, itemIndex, seriesId, quality) {
  return Boolean(
    slot &&
    slot.ready &&
    slot.expiresAt > Date.now() &&
    slot.itemIndex === itemIndex &&
    slot.seriesId === seriesId &&
    slot.quality === quality &&
    slot.streamUrl &&
    slot.video?.src &&
    normalizeFeedMediaURL(slot.streamUrl) === normalizeFeedMediaURL(slot.video.src)
  );
}

function normalizeFeedMediaURL(value) {
  if (!value) return '';
  try {
    return new URL(value, window.location.href).href;
  } catch (_) {
    return String(value);
  }
}

function syncFeedVideoOrientation(video) {
  syncPlaybackVideoFit(video);
}

let videoFitObserver = null;
let videoFitFrame = 0;

function playbackVideoFitBox(video) {
  return video?.closest('.feed-slide-layer, .video-container') || video?.parentElement || null;
}

function syncPlaybackVideosFit() {
  document.querySelectorAll('.feed-video, #main-video').forEach(syncPlaybackVideoFit);
}

function schedulePlaybackVideoFit() {
  if (videoFitFrame) return;
  videoFitFrame = requestAnimationFrame(() => {
    videoFitFrame = 0;
    syncPlaybackVideosFit();
  });
}

function attachPlaybackVideoFit(video) {
  if (!video || video.dataset.videoFitAttached === 'true') return;
  video.dataset.videoFitAttached = 'true';
  video.addEventListener('resize', () => syncPlaybackVideoFit(video));
  if (!window.__videoFitResizeAttached) {
    window.__videoFitResizeAttached = true;
    window.addEventListener('resize', schedulePlaybackVideoFit);
  }

  const box = playbackVideoFitBox(video);
  if (!box || typeof ResizeObserver === 'undefined') return;
  if (!videoFitObserver) {
    videoFitObserver = new ResizeObserver(schedulePlaybackVideoFit);
  }
  videoFitObserver.observe(box);
}

function syncPlaybackVideoFit(video) {
  if (!video) return;
  const sourceWidth = video.videoWidth;
  const sourceHeight = video.videoHeight;
  const isPortrait = (sourceHeight || 16) >= (sourceWidth || 9);
  video.classList.toggle('portrait', isPortrait);
  video.classList.toggle('landscape', !isPortrait);

  const box = playbackVideoFitBox(video);
  if (!sourceWidth || !sourceHeight || !box?.clientWidth || !box?.clientHeight) {
    video.style.removeProperty('width');
    video.style.removeProperty('height');
    return;
  }

  // WKWebView compositing can lose object-fit scaling after source or stage
  // changes. Give the video element the exact fitted frame instead.
  const scale = Math.min(box.clientWidth / sourceWidth, box.clientHeight / sourceHeight);
  video.style.width = `${Math.max(1, Math.round(sourceWidth * scale))}px`;
  video.style.height = `${Math.max(1, Math.round(sourceHeight * scale))}px`;
}

function resetPlaybackVideoFit(video) {
  if (!video) return;
  video.style.removeProperty('width');
  video.style.removeProperty('height');
  video.style.removeProperty('aspectRatio');
}

function shouldCaptureFeedScroll() {
  const drawer = document.getElementById('feed-playlist-drawer');
  const modal = document.getElementById('player-modal');
  const drawerOpen = Boolean(drawer?.classList.contains('open'));
  const modalOpen = Boolean(modal && modal.style.display !== 'none');
  return state.currentView === 'recommend' && !drawerOpen && !modalOpen;
}

function syncFeedScrollCapture() {
  apiCall('SetFeedScrollCapture', shouldCaptureFeedScroll()).catch(() => {});
}

function resetFeedVideoPool() {
  feedPlayToken++;
  cancelPendingFeedSwitch();
  feedSlots.forEach((slot, index) => {
    clearFeedSlot(slot);
    if (slot.video) {
      slot.video.muted = true;
      slot.video.classList.toggle('is-active', index === 0);
      slot.video.classList.remove('is-loading');
    }

    if (slot.layer) {
      slot.layer.style.transition = 'none';
      slot.layer.style.transform = index === 0 ? 'translateY(0)' : (index === 1 ? 'translateY(100%)' : 'translateY(-100%)');
      slot.layer.style.zIndex = index === 0 ? '2' : '1';
      slot.layer.classList.toggle('is-active', index === 0);
    }
    if (slot.poster) {
      slot.poster.style.backgroundImage = 'none';
      slot.poster.classList.remove('hidden');
    }
  });

  feedSlotNext = 1;
  feedSlotPrev = 2;
  setActiveFeedSlot(feedSlots[0], { force: true });
}

function effectiveFeedNetworkProfile() {
  const diagnostics = state.feed.networkDiagnostics || {};
  let profile = diagnostics.networkProfile || 'unknown';
  const connection = navigator.connection || navigator.mozConnection || navigator.webkitConnection;
  if (connection) {
    if (connection.saveData || ['slow-2g', '2g'].includes(connection.effectiveType)) {
      profile = 'weak';
    } else if (connection.effectiveType === '3g') {
      profile = profile === 'strong' ? 'medium' : profile;
    }
  }
  return profile;
}

function feedWarmupLimits(profile = effectiveFeedNetworkProfile()) {
  if (profile === 'weak') return { next: 1, previous: 0 };
  if (profile === 'strong') return { next: 3, previous: 2 };
  return { next: 2, previous: 1 };
}

function definitionNumber(quality) {
  const match = String(quality || '').match(/\d+/);
  return match ? Number(match[0]) : 0;
}

function availableFeedQualityOptions() {
  return (state.feed.qualityOptions || []).filter(option => option.available !== false);
}

function feedDesiredQuality() {
  if (state.feed.userQuality && !state.feed.blockedQualities?.includes(state.feed.userQuality)) {
    return state.feed.userQuality;
  }
  const options = availableFeedQualityOptions();
  const usable = options.filter(option => !state.feed.blockedQualities?.includes(option.id));
  const candidates = usable.length ? usable : options;
  if (!candidates.length) return state.feed.baseQuality || '720p';

  const requested = state.feed.baseQuality || '720p';
  const profile = effectiveFeedNetworkProfile();
  // 720p is the automatic feed ceiling. 1080p roughly doubles upstream work and
  // makes rapid handoffs more fragile; users can still explicitly choose it.
  const maxDefinition = profile === 'weak' ? 540 : 720;
  let selected = candidates.reduce((lowest, option) => {
    const definition = definitionNumber(option.id) || option.height || 0;
    const lowestDefinition = definitionNumber(lowest.id) || lowest.height || 0;
    return definition < lowestDefinition ? option : lowest;
  }, candidates[0]);
  let score = Number.MAX_SAFE_INTEGER;
  candidates.forEach(option => {
    const definition = definitionNumber(option.id) || option.height || 0;
    if (definition > maxDefinition) return;
    const distance = Math.abs(definition - definitionNumber(requested));
    if (distance < score || (distance === score && definition > (definitionNumber(selected.id) || selected.height || 0))) {
      selected = option;
      score = distance;
    }
  });

  const bitrateBudget = (state.feed.networkDiagnostics?.networkBytesPerSec || 0) * 8 * 0.7;
  if (bitrateBudget > 0) {
    const affordable = candidates
      .filter(option => {
        const definition = definitionNumber(option.id) || option.height || 0;
        return definition <= maxDefinition && (!option.bitrate || option.bitrate <= bitrateBudget);
      })
      .sort((a, b) => (b.bitrate || definitionNumber(b.id) * 1000000) - (a.bitrate || definitionNumber(a.id) * 1000000));
    if (affordable.length) selected = affordable[0];
  }
  return selected.id || requested;
}

function mediaCodecString(option) {
  const codec = String(option.codec || '').toLowerCase();
  if (codec.includes('h265') || codec.includes('hevc')) return 'hvc1.1.6.L93.B0';
  if (codec.includes('h264') || codec.includes('avc')) return 'avc1.42E01E';
  if (/[a-z0-9]{2,}\.[a-z0-9.]+/i.test(codec)) return codec;
  return 'avc1.42E01E';
}

async function assessFeedMediaCapabilities() {
  if (!navigator.mediaCapabilities?.decodingInfo) return;
  const options = availableFeedQualityOptions().slice(0, 4);
  for (const option of options) {
    const key = `${option.id}:${option.codec || ''}:${option.width || 0}x${option.height || 0}`;
    if (state.feed.capabilityCache?.has(key)) continue;
    try {
      const result = await navigator.mediaCapabilities.decodingInfo({
        type: 'file',
        video: {
          contentType: `video/mp4; codecs="${mediaCodecString(option)}"`,
          width: option.width || 1280,
          height: option.height || 720,
          bitrate: option.bitrate || 1500000,
          framerate: 30
        }
      });
      if (!state.feed.capabilityCache) state.feed.capabilityCache = new Map();
      state.feed.capabilityCache.set(key, Boolean(result.supported && result.smooth));
      if (!result.supported || !result.smooth) {
        if (!state.feed.blockedQualities) state.feed.blockedQualities = [];
        if (!state.feed.blockedQualities.includes(option.id)) state.feed.blockedQualities.push(option.id);
      }
    } catch (_) {
      // Capability probing is best-effort; playback fallback remains available.
    }
  }
}

async function prepareFeedSlot(slot, itemIndex, autoPlay = false, resumeAt = null, primeFirstFrame = false) {
  if (!slot || !slot.video) return null;
  if (itemIndex < 0 || itemIndex >= state.feed.items.length) {
    clearFeedSlot(slot);
    return null;
  }

  const drama = state.feed.items[itemIndex];
  if (!drama) return null;
  const requestedQuality = feedDesiredQuality();
  const requestedKey = feedAssetKey(drama.id, 1, requestedQuality);

  if (feedSlotMatches(slot, itemIndex, drama.id, requestedQuality)) {
    const cachedPlayback = state.feed.preloadMap.get(requestedKey);
    const progressiveStartupURL = (
      autoPlay &&
      !cachedPlayback?.isFullCached &&
      !feedVideoHasFirstFrame(slot.video) &&
      (cachedPlayback?.progressiveStreamUrl || '')
    );
    if (
      progressiveStartupURL &&
      normalizeFeedMediaURL(slot.streamUrl) !== normalizeFeedMediaURL(progressiveStartupURL)
    ) {
      // The same race can occur on the first feed render or a quality switch:
      // metadata is warm, but a background HLS decoder still has no frame.
      slot.streamUrl = progressiveStartupURL;
      slot.progressiveStreamUrl = progressiveStartupURL;
      slot.video.src = progressiveStartupURL;
      slot.video.load();
      slot.usedHLSFallback = true;
      slot.usedProgressiveFallback = true;
      slot.metricStreamMode = 'progressive';
    }
    syncActiveFeedSlot(slot);
    state.feed.consecutiveSourceFailures = 0;
    if (primeFirstFrame && !feedVideoHasFirstFrame(slot.video) && !slot.firstFramePriming) {
      slot.firstFramePriming = primeFeedSlotFirstFrame(slot);
    }
    if (autoPlay && state.currentView === 'recommend' && !state.feed.userPaused) {
      beginFeedPlayback(slot.video, feedPlayToken);
    }
    return null;
  }

  clearFeedSlot(slot);
  const loadToken = slot.loadToken;
  slot.metricPreparedAt = performance.now();
  slot.itemIndex = itemIndex;
  slot.seriesId = drama.id;
  slot.episodeNum = 1;
  slot.quality = requestedQuality;
  slot.video.dataset.seriesId = String(drama.id);
  slot.video.dataset.episodeNum = '1';

  if (slot.poster) {
    slot.poster.style.backgroundImage = safeImageCSS(drama.coverUrl);
    slot.poster.classList.remove('hidden');
  }

  const loadPlayback = async () => {
    let pb = state.feed.preloadMap.get(requestedKey);
    if (!pb) {
      pb = await apiCall('PlayEpisode', drama.id, '', 1, requestedQuality, !autoPlay);
    }
    if (loadToken !== slot.loadToken) return;
    if (!pb || !pb.streamUrl) {
      if (feedSlots[feedSlotCurr] === slot) reportFeedFailure(slot, '获取播放流失败');
      return;
    }

    const selectedQuality = pb.quality || requestedQuality;
    rememberFeedPlayback(requestedKey, pb);
    rememberFeedPlayback(feedAssetKey(drama.id, 1, selectedQuality), pb);

    slot.episodeId = pb.episodeId || '';
    slot.episodeNum = pb.episodeNum || 1;
    slot.quality = selectedQuality;
    slot.durationSec = Number(pb.durationSec) || 0;
    slot.fullStreamUrl = pb.fullStreamUrl || pb.streamUrl;
    slot.progressiveStreamUrl = pb.progressiveStreamUrl || '';
    slot.hlsStreamUrl = pb.hlsStreamUrl || '';
    slot.streamUrl = pb.streamUrl;
    const progressiveStartup = Boolean(
      autoPlay &&
      !pb.isFullCached &&
      !feedVideoHasFirstFrame(slot.video) &&
      slot.progressiveStreamUrl
    );
    if (progressiveStartup) slot.streamUrl = slot.progressiveStreamUrl;
    if (!autoPlay && pb.isFullCached) slot.streamUrl = slot.fullStreamUrl;
    slot.usedHLSFallback = Boolean(pb.hlsStreamUrl && !progressiveStartup);
    slot.usedProgressiveFallback = progressiveStartup;
    slot.video.crossOrigin = 'anonymous';
    slot.video.src = slot.streamUrl;
    slot.video.preload = 'auto';
    state.feed.consecutiveSourceFailures = 0;
    scheduleFeedFrameMetric(slot);
    slot.video.playbackRate = state.feed.playbackSpeed || 1;
    slot.video.load();
    slot.ready = true;
    slot.metricCacheHit = Boolean(pb.isFullCached);
    slot.metricStreamMode = pb.isFullCached
      ? 'full-cache'
      : (progressiveStartup ? 'progressive' : (pb.hlsStreamUrl ? 'hls' : 'progressive'));
    slot.preparedAt = Date.now();
    slot.expiresAt = slot.preparedAt + 10 * 60 * 1000;
    syncActiveFeedSlot(slot, pb);
    assessFeedMediaCapabilities();
    if (!autoPlay && primeFirstFrame) {
      slot.firstFramePriming = primeFeedSlotFirstFrame(slot);
    }

    const storedPosition = state.feed.playbackPositions.get(feedPlaybackKey(drama.id, slot.episodeNum, selectedQuality));
    const restorePosition = Number.isFinite(resumeAt)
      ? resumeAt
      : (storedPosition ?? (autoPlay ? pb.lastPosition : null));
    const shouldRestore = restorePosition > 3 && (!slot.video.duration || restorePosition < slot.video.duration - 5);

    const startPlayback = () => {
      if (loadToken !== slot.loadToken) return;
      slot.loaded = true;
      clearFeedFailureHint(slot);
      if (slot.poster) slot.poster.classList.add('hidden');
      showFeedSkeleton(false);
      const loader = document.getElementById('feed-loader');
      if (loader && feedSlots[feedSlotCurr] === slot) loader.style.display = 'none';
      if (shouldRestore && Number.isFinite(slot.video.duration)) {
        slot.video.currentTime = Math.min(restorePosition, slot.video.duration - 0.5);
      }
      if (autoPlay && state.currentView === 'recommend' && !state.feed.userPaused && feedSlots[feedSlotCurr] === slot) {
        beginFeedPlayback(slot.video, feedPlayToken);
        scheduleFeedAutoplayRetry(slot);
      }
    };

    slot.video.onloadeddata = startPlayback;
    slot.video.onloadedmetadata = () => {
      if (loadToken !== slot.loadToken) return;
      slot.bufferReady = slot.video.readyState >= 3;
    };
    slot.video.oncanplay = () => {
      if (loadToken !== slot.loadToken) return;
      slot.bufferReady = slot.video.readyState >= 3;
    };
    slot.video.oncanplaythrough = () => {
      if (loadToken !== slot.loadToken) return;
      slot.bufferReady = true;
    };
    slot.video.onplaying = () => {
      if (loadToken !== slot.loadToken) return;
      slot.loaded = true;
      clearFeedFailureHint(slot);
      if (slot.poster) slot.poster.classList.add('hidden');
      showFeedSkeleton(false);
      const loader = document.getElementById('feed-loader');
      if (loader && feedSlots[feedSlotCurr] === slot) loader.style.display = 'none';
    };
    slot.video.onerror = event => {
      if (loadToken !== slot.loadToken) return;
      slot.ready = false;
      if (feedSlots[feedSlotCurr] !== slot) return;
      console.warn('feed video error:', drama.id, event);
      const loader = document.getElementById('feed-loader');
      if (loader) loader.style.display = 'none';
      const resumePosition = slot.video.currentTime || 0;
      const fallbackTo = source => {
        if (!source || slot.video.src === source) return false;
        slot.streamUrl = source;
        slot.video.src = source;
        slot.video.load();
        slot.video.onloadeddata = () => {
          if (loadToken !== slot.loadToken) return;
          slot.loaded = true;
          clearFeedFailureHint(slot);
          if (resumePosition > 0 && Number.isFinite(slot.video.duration)) {
            slot.video.currentTime = Math.min(resumePosition, slot.video.duration - 0.5);
          }
          if (autoPlay && state.currentView === 'recommend' && !state.feed.userPaused && feedSlots[feedSlotCurr] === slot) {
            beginFeedPlayback(slot.video, feedPlayToken);
            scheduleFeedAutoplayRetry(slot);
          }
        };
        scheduleFeedAutoplayRetry(slot);
        return true;
      };
      if (!slot.usedProgressiveFallback && fallbackTo(slot.progressiveStreamUrl)) {
        slot.usedProgressiveFallback = true;
        slot.metricStreamMode = 'progressive';
        return;
      }
      if (!slot.usedHLSFallback && fallbackTo(slot.hlsStreamUrl)) {
        slot.usedHLSFallback = true;
        slot.metricStreamMode = 'hls';
        return;
      }
      if (!slot.usedFullFallback && fallbackTo(slot.fullStreamUrl)) {
        slot.usedFullFallback = true;
        slot.metricUsedFullFallback = true;
        slot.metricStreamMode = 'full-fallback';
        return;
      }
      const nextIndex = slot.itemIndex + 1;
      const nextDrama = state.feed.items[nextIndex];
      const nextSlot = feedSlots.find(candidate => (
        candidate.itemIndex === nextIndex &&
        candidate.seriesId === nextDrama?.id
      ));
      if (feedVideoHasFirstFrame(nextSlot?.video)) {
        scheduleFeedRecoveryAdvance(slot, itemIndex);
        return;
      }
      reportFeedFailure(slot, '视频加载失败，请重试');
    };

    if (slot.video.readyState >= 1 && (!shouldRestore || slot.video.duration)) {
      startPlayback();
    }
  };

  const preparing = loadPlayback();
  slot.preparing = preparing;
  try {
    await preparing;
    return preparing;
  } catch (err) {
    if (loadToken === slot.loadToken && feedSlots[feedSlotCurr] === slot) {
      console.warn('prepareFeedSlot error:', drama.id, err);
      reportFeedFailure(slot, '播放失败，请重试');
    }
  } finally {
    if (loadToken === slot.loadToken) slot.preparing = null;
  }
}

function clearFeedFailureHint(slot) {
  if (!slot?.failureHintTimer) return;
  clearTimeout(slot.failureHintTimer);
  slot.failureHintTimer = null;
}

function reportFeedFailure(slot, text, delay = 3000) {
  if (!slot) return;
  clearFeedFailureHint(slot);
  const loadToken = slot.loadToken;
  slot.failureHintTimer = setTimeout(() => {
    slot.failureHintTimer = null;
    const video = slot.video;
    const stillFailing = (
      loadToken === slot.loadToken &&
      feedSlots[feedSlotCurr] === slot &&
      video &&
      (video.error || !video.src || (video.paused && video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA))
    );
    if (stillFailing) {
      showFeedHint(text, false, 2200);
    }
  }, delay);
}

function syncActiveFeedSlot(slot, playback = null) {
  // Preloading a background slot must never steal the active media chrome.
  if (!slot || feedSlots[feedSlotCurr] !== slot) return;
  state.feed.episodeId = slot.episodeId || state.feed.episodeId;
  state.feed.episodeNum = slot.episodeNum || 1;
  state.feed.quality = slot.quality || state.feed.quality;
  if (playback) {
    state.feed.isLiked = Boolean(playback.isLiked);
    const likeBtn = document.getElementById('feed-like');
    likeBtn?.classList.toggle('active', Boolean(playback.isLiked));
    if (Number(playback.commentCount) > 0) updateFeedCommentCount(playback.commentCount);
    updateFeedQualityControls(playback);
  }
  syncFeedMediaChrome();
}

function cancelScheduledFeedWarmup() {
  feedWarmupToken++;
  feedWarmupTimers.forEach(timer => clearTimeout(timer));
  feedWarmupTimers = [];
}

function feedSeriesIDs(items, centerIndex, nextCount, previousCount) {
  const nextIDs = [];
  const previousIDs = [];
  for (let i = 1; i <= nextCount; i++) {
    if (items[centerIndex + i]) nextIDs.push(items[centerIndex + i].id);
  }
  for (let i = 1; i <= previousCount; i++) {
    if (items[centerIndex - i]) previousIDs.push(items[centerIndex - i].id);
  }
  return { nextIDs, previousIDs };
}

function prefetchFeedPlaybackInfo(itemIndex, preferHLS = true) {
  const drama = state.feed.items[itemIndex];
  if (!drama) return null;
  const quality = feedDesiredQuality();
  const key = feedAssetKey(drama.id, 1, quality);
  if (state.feed.preloadMap.has(key)) return null;
  const existingLoad = state.feed.playbackInfoLoads.get(key);
  if (existingLoad) return existingLoad;

  const load = apiCall('PlayEpisode', drama.id, '', 1, quality, preferHLS)
    .then(playback => {
      if (playback?.streamUrl) rememberFeedPlayback(key, playback);
      return playback;
    })
    .catch(err => {
      console.warn('feed playback info prefetch failed:', drama.id, err);
      return null;
    })
    .finally(() => {
      if (state.feed.playbackInfoLoads.get(key) === load) {
        state.feed.playbackInfoLoads.delete(key);
      }
    });
  state.feed.playbackInfoLoads.set(key, load);
  return load;
}

function preloadFeedCovers(items, centerIndex, count = 3) {
  const limit = Math.max(0, Math.min(3, Number(count) || 0));
  for (let i = 1; i <= limit; i++) {
    const coverURL = items[centerIndex + i]?.coverUrl;
    if (!coverURL) continue;
    const image = new Image();
    image.decoding = 'async';
    image.src = coverURL;
  }
}

function recordFeedSwitchMetric(fromIndex, toIndex, bufferedAtSwitch, durationMs) {
  const diagnostics = state.feed.networkDiagnostics || {};
  const slot = feedSlots[feedSlotCurr];
  const switchStartedAt = performance.now() - durationMs;
  const metric = {
    type: 'switch',
    fromIndex,
    toIndex,
    bufferedAtSwitch,
    durationMs: Math.round(durationMs),
    timeToFirstFrameMs: slot?.metricFirstFrameAt && slot?.metricPreparedAt
      ? Math.max(0, Math.round(slot.metricFirstFrameAt - slot.metricPreparedAt))
      : 0,
    handoffFirstFrameMs: slot?.metricFirstFrameAt
      ? Math.max(0, Math.round(slot.metricFirstFrameAt - switchStartedAt))
      : 0,
    firstFrameSource: slot?.metricFirstFrameSource || '',
    switchReadyMs: slot?.metricReadyAt && slot?.metricPreparedAt
      ? Math.max(0, Math.round(slot.metricReadyAt - slot.metricPreparedAt))
      : 0,
    rebufferCount: slot?.metricRebufferCount || 0,
    rebufferMs: Math.round((slot?.metricRebufferMS || 0) + (slot?.metricRebufferStartedAt
      ? Math.max(0, performance.now() - slot.metricRebufferStartedAt)
      : 0)),
    networkProfile: diagnostics.networkProfile || effectiveFeedNetworkProfile(),
    networkBytesPerSec: diagnostics.networkBytesPerSec || 0,
    networkFailures: diagnostics.networkFailures || 0,
    prefetchCompleted: diagnostics.prefetchCompleted || 0,
    prefetchFailed: diagnostics.prefetchFailed || 0,
    prefetchCanceled: diagnostics.prefetchCanceled || 0,
    prefetchDropped: diagnostics.prefetchDropped || 0,
    hlsStarts: diagnostics.hlsStarts || 0,
    hlsHits: diagnostics.hlsHits || 0,
    streamMode: slot?.metricStreamMode || '',
    cacheHit: Boolean(slot?.metricCacheHit),
    fullFallback: Boolean(slot?.metricUsedFullFallback),
    quality: slot?.quality || state.feed.quality,
    at: new Date().toISOString()
  };
  feedSwitchMetrics.push(metric);
  if (feedSwitchMetrics.length > 100) feedSwitchMetrics.shift();
  feedMetricQueue.push(metric);
  if (feedMetricQueue.length > 32) feedMetricQueue.splice(0, feedMetricQueue.length - 32);
  scheduleFeedMetricFlush();
  console.debug('[feed-metric]', metric);
  setTimeout(() => recordFeedPlaybackMetric(metric.toIndex), 5000);
}

function recordFeedPlaybackMetric(itemIndex) {
  const slot = feedSlots[feedSlotCurr];
  if (!slot || slot.itemIndex !== itemIndex) return;
  const diagnostics = state.feed.networkDiagnostics || {};
  const activeRebufferMS = slot.metricRebufferStartedAt
    ? Math.max(0, performance.now() - slot.metricRebufferStartedAt)
    : 0;
  const metric = {
    type: 'playback-sample',
    toIndex: itemIndex,
    bufferedAtSwitch: feedVideoHasPlaybackBuffer(slot.video),
    durationMs: Math.round(performance.now() - (slot.metricFirstFrameAt || slot.metricPreparedAt)),
    timeToFirstFrameMs: slot.metricFirstFrameAt && slot.metricPreparedAt
      ? Math.max(0, Math.round(slot.metricFirstFrameAt - slot.metricPreparedAt))
      : 0,
    firstFrameSource: slot.metricFirstFrameSource || '',
    switchReadyMs: slot.metricReadyAt && slot.metricPreparedAt
      ? Math.max(0, Math.round(slot.metricReadyAt - slot.metricPreparedAt))
      : 0,
    rebufferCount: slot.metricRebufferCount || 0,
    rebufferMs: Math.round(slot.metricRebufferMS + activeRebufferMS),
    networkProfile: diagnostics.networkProfile || effectiveFeedNetworkProfile(),
    networkBytesPerSec: diagnostics.networkBytesPerSec || 0,
    networkFailures: diagnostics.networkFailures || 0,
    prefetchCompleted: diagnostics.prefetchCompleted || 0,
    prefetchFailed: diagnostics.prefetchFailed || 0,
    prefetchCanceled: diagnostics.prefetchCanceled || 0,
    prefetchDropped: diagnostics.prefetchDropped || 0,
    hlsStarts: diagnostics.hlsStarts || 0,
    hlsHits: diagnostics.hlsHits || 0,
    streamMode: slot.metricStreamMode || '',
    cacheHit: Boolean(slot.metricCacheHit),
    fullFallback: Boolean(slot.metricUsedFullFallback),
    quality: slot.quality || state.feed.quality,
    at: new Date().toISOString()
  };
  feedMetricQueue.push(metric);
  scheduleFeedMetricFlush();
}

function scheduleFeedMetricFlush() {
  if (feedMetricFlushTimer) return;
  feedMetricFlushTimer = setTimeout(async () => {
    feedMetricFlushTimer = null;
    await flushFeedMetrics();
  }, 2000);
}

async function flushFeedMetrics() {
  if (feedMetricFlushInFlight || !feedMetricQueue.length) return;
  feedMetricFlushInFlight = true;
  const metrics = feedMetricQueue.splice(0, 32);
  try {
    await apiCall('RecordFeedMetrics', metrics);
  } catch (err) {
    // Keep diagnostics on a bounded retry queue; a transient IPC failure should
    // not erase the only record of a rebuffer or failed handoff.
    feedMetricQueue.unshift(...metrics);
    if (feedMetricQueue.length > 128) feedMetricQueue.splice(0, feedMetricQueue.length - 128);
    console.debug('feed metric persistence unavailable:', err);
    scheduleFeedMetricFlush();
  } finally {
    feedMetricFlushInFlight = false;
  }
}

window.addEventListener('pagehide', () => {
  flushFeedMetrics();
});
window.addEventListener('beforeunload', () => {
  flushFeedMetrics();
});
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'hidden') flushFeedMetrics();
});

async function refreshFeedNetworkDiagnostics() {
  try {
    const diagnostics = await apiCall('GetMediaDiagnostics');
    if (diagnostics && diagnostics.networkProfile) state.feed.networkDiagnostics = diagnostics;
  } catch (_) {
    // Unknown network conditions use the safe medium window until data exists.
  }
}

function feedVideoBufferAhead(video) {
  if (!video?.src) return 0;
  const position = video.currentTime || 0;
  for (let i = 0; i < video.buffered.length; i++) {
    if (video.buffered.start(i) <= position && video.buffered.end(i) >= position) {
      return Math.max(0, video.buffered.end(i) - position);
    }
  }
  return 0;
}

async function scheduleFeedWarmup(centerIndex) {
  if (state.currentView !== 'recommend') return;
  const items = state.feed.items;
  const centerDrama = items?.[centerIndex];
  if (!items?.length || !centerDrama) return;
  ensureFeedItemsAhead();

  if (!items[centerIndex + 1]) {
    feedWarmupTimers.push(setTimeout(() => {
      if (state.currentView === 'recommend') scheduleFeedWarmup(centerIndex);
    }, 1500));
    return;
  }

  cancelScheduledFeedWarmup();
  const token = feedWarmupToken;
  await refreshFeedNetworkDiagnostics();
  if (token !== feedWarmupToken || state.currentView !== 'recommend') return;

  const profile = effectiveFeedNetworkProfile();
  const limits = feedWarmupLimits(profile);
  const requestWindow = (nextCount, previousCount) => {
    if (token !== feedWarmupToken) return;
    const { nextIDs, previousIDs } = feedSeriesIDs(items, centerIndex, nextCount, previousCount);
    apiCall('WarmupFeed', centerDrama.id, feedSlots[feedSlotCurr]?.episodeId || '', previousIDs, nextIDs, feedDesiredQuality(), profile)
      .catch(err => console.warn('feed warmup failed:', err));
  };

  requestWindow(1, 0);
  const prepareImmediateNext = async () => {
    const nextSlot = feedSlots[feedSlotNext];
    await Promise.resolve(nextSlot?.preparing || prepareFeedSlot(nextSlot, centerIndex + 1, false, null, true));
    if (token !== feedWarmupToken || state.currentView !== 'recommend') return;
    // Start i+2 only after i+1 has a source. The backend serializes HLS jobs,
    // so this cannot steal the immediate handoff's upstream bandwidth, but it
    // does ensure the following swipe has local segments much earlier.
    if (limits.next >= 2) prefetchFeedPlaybackInfo(centerIndex + 2);
  };
  prepareImmediateNext().catch(err => console.warn('feed next warmup failed:', err));
  preloadFeedCovers(items, centerIndex, profile === 'weak' ? 1 : (profile === 'medium' ? 2 : 3));
  finishFeedWarmup(centerIndex, token, profile, limits, requestWindow);
}

async function finishFeedWarmup(centerIndex, token, profile, limits, requestWindow) {
  const items = state.feed.items;
  const centerDrama = items?.[centerIndex];
  const nextDrama = items?.[centerIndex + 1];
  if (!centerDrama || !nextDrama) return;

  let nextReady = false;
  while (token === feedWarmupToken && state.currentView === 'recommend') {
    const nextSlot = feedSlots[feedSlotNext];
    const sourceMatches = Boolean(
      nextSlot &&
      nextSlot.itemIndex === centerIndex + 1 &&
      nextSlot.seriesId === nextDrama.id &&
      nextSlot.streamUrl &&
      nextSlot.video?.src
    );
    if (!sourceMatches) {
      await (nextSlot?.preparing || prepareFeedSlot(nextSlot, centerIndex + 1, false, null, true));
    }
    nextReady = await waitForFeedPlaybackBuffer(nextSlot.video, 5000);
    if (nextReady) break;

    // A failed source must be reloaded rather than waiting forever on a dead
    // decoder. prepareFeedSlot will clear and reuse the same slot on retry.
    if (nextSlot?.video?.error) {
      await prepareFeedSlot(nextSlot, centerIndex + 1, false, null, true);
    }
  }
  if (!nextReady || token !== feedWarmupToken) return;

  // Do not let i+2 or backward media downloads start until both the current
  // video and the unique highest-priority i+1 stream have a safe buffer.
  if (feedVideoBufferAhead(feedSlots[feedSlotCurr]?.video) < 4) {
    feedWarmupTimers.push(setTimeout(async () => {
      if (token !== feedWarmupToken) return;
      const activeVideo = feedSlots[feedSlotCurr]?.video;
      if (feedVideoBufferAhead(activeVideo) >= 4) {
        await finishFeedWarmupAfterNext(centerIndex, token, profile, limits, requestWindow);
      } else {
        await finishFeedWarmup(centerIndex, token, profile, limits, requestWindow);
      }
    }, 2000));
    return;
  }

  finishFeedWarmupAfterNext(centerIndex, token, profile, limits, requestWindow);
}

function finishFeedWarmupAfterNext(centerIndex, token, profile, limits, requestWindow) {
  const secondNextCount = Math.min(limits.next, 2);
  requestWindow(secondNextCount, Math.min(limits.previous, 1));
  if (secondNextCount >= 2) prefetchFeedPlaybackInfo(centerIndex + 2);
  // Keep i+3 metadata warm without starting another upstream HLS job; it can
  // use progressive startup if the user reaches it before i+2 finishes.
  if (limits.next >= 3) prefetchFeedPlaybackInfo(centerIndex + 3, false);
  if (limits.previous > 0) {
    prepareFeedSlot(feedSlots[feedSlotPrev], centerIndex - 1, false);
  }

  feedWarmupTimers.push(setTimeout(() => {
    if (token !== feedWarmupToken) return;
    if (profile !== 'strong' || feedVideoBufferAhead(feedSlots[feedSlotCurr]?.video) < 8) return;
    requestWindow(limits.next, limits.previous);
  }, 8000));
}

const state = {
  currentView: 'recommend',
  historyStack: ['recommend'],
  historyIndex: 0,
  reservationsCache: null,
  reservationsTab: 'online',
  favoritesView: {
    tab: 'favorites',
    favoritesList: [],
    likesList: [],
    loading: false
  },

  // Recommend vertical feed state (TikTok/Douyin full-bleed vertical player)
  feed: {
    items: [],
    index: 0,
    cursor: '',
    hasMore: true,
    channel: '',
    loading: false,
    pageRetryTimer: null,
    emptyFeedPages: 0,
    consecutiveSourceFailures: 0,
    lastSwitchTime: 0,
    seriesId: '',
    seriesTitle: '',
    seriesCover: '',
    episodeNum: 1,
    episodeId: '',
    totalEpisodes: 1,
    episodes: [],
    commentCount: 0,
    quality: '720p',
    baseQuality: '720p',
    userQuality: '',
    qualityOptions: [],
    networkDiagnostics: null,
    capabilityCache: new Map(),
    blockedQualities: [],
    isFavorite: false,
    isLiked: false,
    playbackSpeed: 1.0,
    userPaused: false,
    isClearScreen: false,
    playbackPositions: new Map(),
    preloadMap: new Map(),
    playbackInfoLoads: new Map()
  },

  // Actor view state
  actor: {
    userId: '',
    name: '',
    avatarUrl: '',
    cursor: '',
    hasMore: false,
    loading: false
  },

  // Drama detail state
  detail: {
    id: '',
    loading: false,
    requestToken: 0,
    episodesExpanded: false,
    episodesResizeObserver: null,
    relatedObserver: null,
    collectionObservers: {},
    collectionPromises: {},
    collectionData: {}
  },

  // Discover filters
  discover: {
    genre: 'short_play',
    selections: {},
    items: [],
    cursor: '',
    hasMore: false,
    loading: false,
    observer: null
  },

  // Rankings
  rank: {
    content: 'all',
    board: 'ranklist_hot_sc',
    contents: [],
    cursor: '',
    hasMore: false
  },

  // New releases
  newReleases: {
    tab: 'hot',
    date: '',
    dates: [],
    currentDate: ''
  },

  // History view state
  historyView: {
    filter: 'all',
    search: '',
    list: []
  },

  // Player state
  player: {
    seriesId: '',
    seriesTitle: '',
    seriesCover: '',
    episodeNum: 1,
    episodeId: '',
    totalEpisodes: 1,
    episodes: [],
    isFavorite: false,
    isLiked: false,
    commentCount: 0,
    quality: '720p',
    qualityOptions: [],
    playbackSpeed: 1.0,
    saveTimer: null,
    loading: false,
    seasonSwitching: false,
    userPaused: false
  }
};

// IPC Helper with auto-wait for window.mygo injection
async function apiCall(method, ...args) {
  try {
    const bridge = await waitForMygoBridge();
    if (!bridge?.call) throw new Error('MyGo IPC bridge is unavailable');
    const result = await bridge.call('HongguoService.' + method, ...args);
    return cloneIPCValue(result);
  } catch (err) {
    console.error(`API Call [${method}] error:`, err);
    throw err;
  }
}

// Official comment sheet state. It is shared because both player surfaces show
// the same episode dimension and should retain one consistent request token.
const commentState = {
  open: false,
  source: 'feed',
  loading: false,
  requestToken: 0,
  seriesId: '',
  episodeId: '',
  episodeNum: 1,
  title: '评论',
  items: [],
  total: 0,
  cursor: '',
  nextCursor: '',
  hasMore: false,
  error: '',
  replies: {},
  replyCursors: {},
  replyHasMore: {},
  replyLoading: {},
  replyErrors: {},
  replyOpen: {}
};
let commentScrollObserver = null;
let commentOverlayHome = null;
const danmakuControllers = {};
const COMMENT_MOTION_DURATION_MS = 260;
const COMMENT_MOTION_EASING = 'cubic-bezier(0.22, 1, 0.26, 1)';

async function waitForMygoBridge(timeoutMs = 5000) {
  const startedAt = performance.now();
  for (;;) {
    if (window.mygo?.on) return window.mygo;
    if (performance.now() - startedAt >= timeoutMs) return null;
    await new Promise(resolve => setTimeout(resolve, 25));
  }
}

// Toast notification
function showToast(msg) {
  const toast = document.getElementById('toast');
  toast.textContent = msg;
  toast.style.display = 'block';
  clearTimeout(toast._timer);
  toast._timer = setTimeout(() => {
    toast.style.display = 'none';
  }, 2200);
}

// ==========================================================================
// macOS Native Integration Helpers
// ==========================================================================

async function nativeConfirm(message, detail = '', confirmText = '确认', isDestructive = false) {
  try {
    const ok = await apiCall('ShowConfirmDialog', {
      title: '红果短剧',
      message: message,
      detail: detail,
      confirmText: confirmText,
      cancelText: '取消',
      destructive: isDestructive
    });
    return !!ok;
  } catch (err) {
    console.warn('Native confirm fallback:', err);
    return window.confirm(detail ? `${message}\n${detail}` : message);
  }
}

let nowPlayingDebounceTimer = null;
function updateNowPlayingState(opts = {}) {
  clearTimeout(nowPlayingDebounceTimer);
  nowPlayingDebounceTimer = setTimeout(() => {
    apiCall('UpdateNowPlaying', {
      title: opts.title || '',
      series: opts.series || '',
      episode: opts.episode || 1,
      duration: opts.duration || 0,
      position: opts.position || 0,
      playing: !!opts.playing,
      rate: opts.rate || 1.0,
      coverUrl: opts.coverUrl || ''
    }).catch(e => console.warn('UpdateNowPlaying failed:', e));
  }, 100);
}

function clearNowPlayingState() {
  clearTimeout(nowPlayingDebounceTimer);
  apiCall('ClearNowPlaying').catch(() => {});
}

function setPlaybackKeepAwake(active) {
  apiCall('SetPlaybackKeepAwake', !!active).catch(() => {});
}

function updatePiPButtons() {
  const playerModal = document.getElementById('player-modal');
  const isDetail = playerModal && playerModal.style.display !== 'none';
  const target = isDetail ? video : getFeedVideo();
  const inPiP = !!document.pictureInPictureElement || (target && target.webkitPresentationMode === 'picture-in-picture');
  const feedPipBtn = document.getElementById('feed-pip');
  const ctrlPipBtn = document.getElementById('player-pip');
  if (feedPipBtn) {
    feedPipBtn.classList.toggle('active', !!inPiP);
    feedPipBtn.title = inPiP ? '退出画中画 (P)' : '画中画 (P)';
  }
  if (ctrlPipBtn) {
    ctrlPipBtn.classList.toggle('active', !!inPiP);
    ctrlPipBtn.title = inPiP ? '退出画中画 (P)' : '画中画 (P)';
  }
}

function togglePictureInPicture() {
  const playerModal = document.getElementById('player-modal');
  const isDetail = playerModal && playerModal.style.display !== 'none';
  const targetVideo = isDetail ? video : getFeedVideo();

  if (!targetVideo) {
    showToast('未找到可播放视频');
    return;
  }

  const inStandardPiP = !!document.pictureInPictureElement;
  const inWebkitPiP = targetVideo.webkitPresentationMode === 'picture-in-picture';

  if (inStandardPiP || inWebkitPiP) {
    if (inWebkitPiP && typeof targetVideo.webkitSetPresentationMode === 'function') {
      try {
        targetVideo.webkitSetPresentationMode('inline');
        return;
      } catch (err) {
        console.warn('webkitSetPresentationMode(inline) failed:', err);
      }
    }
    if (inStandardPiP && typeof document.exitPictureInPicture === 'function') {
      document.exitPictureInPicture().catch(() => {});
      return;
    }
    apiCall('TogglePictureInPicture').catch(() => {});
    return;
  }

  if (!targetVideo.currentSrc && !targetVideo.src) {
    showToast('视频尚未加载');
    return;
  }

  // 1. WebKit presentation mode (Safari/WKWebView native API)
  if (typeof targetVideo.webkitSetPresentationMode === 'function') {
    try {
      targetVideo.webkitSetPresentationMode('picture-in-picture');
      return;
    } catch (err) {
      console.warn('webkitSetPresentationMode failed:', err);
    }
  }

  // 2. Standard W3C requestPictureInPicture
  if (typeof targetVideo.requestPictureInPicture === 'function') {
    targetVideo.requestPictureInPicture().catch((err) => {
      console.warn('requestPictureInPicture failed:', err);
      // 3. Fallback to MyGo backend toggle
      apiCall('TogglePictureInPicture').catch((backendErr) => {
        console.warn('Backend TogglePictureInPicture failed:', backendErr);
        showToast('画中画启动失败');
      });
    });
    return;
  }

  // 3. Fallback to MyGo backend toggle
  apiCall('TogglePictureInPicture').catch((err) => {
    console.warn('Backend TogglePictureInPicture failed:', err);
    showToast('当前环境不支持画中画');
  });
}

function initNativeBridgeEvents() {
  waitForMygoBridge().then(runtime => {
    if (!runtime?.on) return;

    // Handle macOS menu & media keys command events
    runtime.on('mediaCommand', (payload) => {
      const cmd = payload?.command;
      if (!cmd) return;

      const playerModal = document.getElementById('player-modal');
      const isDetailOpen = playerModal && playerModal.style.display !== 'none';

      if (cmd === 'toggle') {
        if (isDetailOpen) {
          const playPauseBtn = document.getElementById('player-play');
          if (playPauseBtn) playPauseBtn.click();
        } else if (state.currentView === 'recommend') {
          toggleFeedPlay();
        }
      } else if (cmd === 'play') {
        if (isDetailOpen) {
          if (video.paused) video.play().catch(e => console.log('play error:', e));
        } else if (state.currentView === 'recommend') {
          const feedVideo = getFeedVideo();
          if (feedVideo.paused) feedVideo.play().catch(e => console.log('play error:', e));
        }
      } else if (cmd === 'pause') {
        if (isDetailOpen) {
          if (!video.paused) video.pause();
        } else if (state.currentView === 'recommend') {
          const feedVideo = getFeedVideo();
          if (!feedVideo.paused) feedVideo.pause();
        }
      } else if (cmd === 'next') {
        if (isDetailOpen) {
          const nextBtn = document.getElementById('player-next-ep');
          if (nextBtn && !nextBtn.disabled) nextBtn.click();
        } else if (state.currentView === 'recommend') {
          swipeFeed('up');
        }
      } else if (cmd === 'prev') {
        if (isDetailOpen) {
          const prevBtn = document.getElementById('player-prev-ep');
          if (prevBtn && !prevBtn.disabled) prevBtn.click();
        } else if (state.currentView === 'recommend') {
          swipeFeed('down');
        }
      } else if (cmd === 'seek' && typeof payload.position === 'number') {
        if (isDetailOpen) {
          video.currentTime = payload.position;
        } else if (state.currentView === 'recommend') {
          const feedVideo = getFeedVideo();
          feedVideo.currentTime = payload.position;
        }
      } else if (cmd === 'pip') {
        togglePictureInPicture();
      }
    });

    // Handle macOS menu navigation events
    runtime.on('navigate', (payload) => {
      if (payload?.seriesId) {
        const playerModal = document.getElementById('player-modal');
        if (playerModal && playerModal.style.display !== 'none') {
          closePlayer();
        }
        navigateToDetail(payload.seriesId);
      } else if (payload?.view) {
        const playerModal = document.getElementById('player-modal');
        if (playerModal && playerModal.style.display !== 'none') {
          closePlayer();
        }
        navigateTo(payload.view);
      }
    });
    // Picture-in-Picture document events
    document.addEventListener('enterpictureinpicture', updatePiPButtons);
    document.addEventListener('leavepictureinpicture', updatePiPButtons);
  }).catch(() => {});
}

// ==========================================================================
// Initialization & Navigation
// ==========================================================================

document.addEventListener('DOMContentLoaded', () => {
  initNativeBridgeEvents();
  initNavigation();
  initSearch();
  mountPlayerChrome({
    prefix: 'feed',
    target: document.getElementById('feed-container'),
    drawerTitle: '选集播放'
  });
  mountPlayerChrome({
    prefix: 'player',
    target: document.getElementById('player-stage'),
    drawerTitle: '选集播放'
  });
  initFeedVideoPool();
  initFeedControls();
  initPlayerControls();
  initCommentSheet();
  initDanmaku();
  initDetailEvents();
  initDiscoverPagination();
  initSidebarResizer();
  initSettingsEvents();

  // 1. Immediately switch to Recommend view for instant render
  switchView('recommend');

  // 2. Load user profile in the background
  loadProfile();
});

function initNavigation() {
  const navItems = document.querySelectorAll('.nav-item');
  navItems.forEach(btn => {
    btn.addEventListener('click', () => {
      const view = btn.dataset.view;
      if (view) {
        navigateTo(view);
      }
    });
  });

  const backBtn = document.getElementById('btn-history-back');
  if (backBtn) {
    backBtn.addEventListener('click', navigateBack);
  }

  const fwdBtn = document.getElementById('btn-history-forward');
  if (fwdBtn) {
    fwdBtn.addEventListener('click', () => {
      if (state.historyIndex < state.historyStack.length - 1) {
        state.historyIndex++;
        const entry = state.historyStack[state.historyIndex];
        const view = historyEntryView(entry);
        if (view === 'detail') {
          state.detail.id = historyDetailId(entry);
        }
        switchView(view, false);
        updateNavButtons();
      }
    });
  }

  const backFromActorBtn = document.getElementById('btn-back-from-actor');
  if (backFromActorBtn) {
    backFromActorBtn.addEventListener('click', navigateBack);
  }

  const loadMoreActorBtn = document.getElementById('btn-actor-load-more');
  if (loadMoreActorBtn) {
    loadMoreActorBtn.addEventListener('click', () => {
      loadMoreActorWorks();
    });
  }

  // Global keydown listeners for Escape (close player)
  window.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      const playerModal = document.getElementById('player-modal');
      if (playerModal && playerModal.style.display !== 'none') {
        if (document.fullscreenElement) {
          document.exitFullscreen().catch(err => console.log(err));
          return;
        }
        closePlayer();
      }
    }
  });

  updateNavButtons();
}

function initDiscoverPagination() {
  document.getElementById('btn-discover-load-more')?.addEventListener('click', () => {
    loadMoreDiscover();
  });
}

function updateNavButtons() {
  const backBtn = document.getElementById('btn-history-back');
  const fwdBtn = document.getElementById('btn-history-forward');
  if (backBtn) {
    backBtn.disabled = state.historyIndex <= 0;
    backBtn.classList.toggle('disabled', state.historyIndex <= 0);
  }
  if (fwdBtn) {
    fwdBtn.disabled = state.historyIndex >= state.historyStack.length - 1;
    fwdBtn.classList.toggle('disabled', state.historyIndex >= state.historyStack.length - 1);
  }
}

function navigateTo(view) {
  if (view === state.currentView) return;
  state.historyStack = state.historyStack.slice(0, state.historyIndex + 1);
  state.historyStack.push(view);
  state.historyIndex++;
  switchView(view, false);
  updateNavButtons();
}
window.navigate = navigateTo;

function historyEntryView(entry) {
  return String(entry || '').split(':')[0] || 'recommend';
}

function historyDetailId(entry) {
  return String(entry || '').split(':')[1] || state.detail.id;
}

function navigateToDetail(seriesId) {
  if (!seriesId) return;
  const entry = `detail:${seriesId}`;
  if (state.currentView === 'detail' && state.historyStack[state.historyIndex] === entry) return;

  state.historyStack = state.historyStack.slice(0, state.historyIndex + 1);
  state.historyStack.push(entry);
  state.historyIndex++;
  state.detail.id = seriesId;
  switchView('detail', false);
  updateNavButtons();
}

function navigateBack() {
  const playerModal = document.getElementById('player-modal');
  if (playerModal && playerModal.style.display !== 'none') {
    closePlayer();
    return;
  }

  if (state.historyIndex > 0) {
    state.historyIndex--;
    const entry = state.historyStack[state.historyIndex];
    const view = historyEntryView(entry);
    if (view === 'detail') {
      state.detail.id = historyDetailId(entry);
    }
    switchView(view, false);
    updateNavButtons();
  }
}

let feedNextDramaTimer = null;

function pauseFeedPlayback() {
  if (window.ambientGlow) {
    window.ambientGlow.pause();
  }
  if (feedNextDramaTimer) {
    clearTimeout(feedNextDramaTimer);
    feedNextDramaTimer = null;
  }
  const feedVideo = typeof getFeedVideo === 'function' ? getFeedVideo() : document.getElementById('feed-video');
  if (feedVideo) {
    feedVideo.pause();
  }
  if (feedPlayPromise) {
    feedPlayPromise
      .then(() => {
        if (state.currentView !== 'recommend') {
          const v = typeof getFeedVideo === 'function' ? getFeedVideo() : document.getElementById('feed-video');
          if (v) v.pause();
        }
      })
      .catch(() => {});
  }
  const preloader = document.getElementById('feed-preloader-video');
  if (preloader) {
    preloader.pause();
    preloader.removeAttribute('src');
    preloader.load();
  }
  setPlaybackKeepAwake(false);
  clearNowPlayingState();
  persistFeedProgress();
  updateFeedPlayIndicator();
}

function resumeFeedPlayback() {
  if (state.currentView !== 'recommend') return;
  const modal = document.getElementById('player-modal');
  if (modal && modal.style.display !== 'none') return;
  if (state.feed.userPaused) {
    updateFeedPlayIndicator();
    return;
  }

  const feedVideo = typeof getFeedVideo === 'function' ? getFeedVideo() : document.getElementById('feed-video');
  if (feedVideo && feedVideo.src) {
    if (window.ambientGlow && !feedVideo.paused) {
      window.ambientGlow.attach(feedVideo).start();
    }
    feedPlayPromise = feedVideo.play();
    if (feedPlayPromise) {
      feedPlayPromise
        .then(() => {
          if (window.ambientGlow && state.currentView === 'recommend') {
            window.ambientGlow.attach(feedVideo).start();
          }
        })
        .catch(e => console.log('resume feed error:', e));
    }
  } else if (state.feed.items && state.feed.items.length > 0) {
    playFeedIndex(state.feed.index || 0).catch(err => console.error('resume playFeedIndex error:', err));
  }
  updateFeedPlayIndicator();
}

function switchView(viewName, pushHistory = true) {
  if (viewName === 'likes') {
    state.favoritesView.tab = 'likes';
    viewName = 'favorites';
  }

  if (viewName !== 'recommend') {
    if (window.ambientGlow) window.ambientGlow.stop();
    cancelScheduledFeedWarmup();
    cancelPendingFeedSwitch();
    pauseFeedPlayback();
  } else {
    resumeFeedPlayback();
  }

  state.currentView = viewName;
  syncFeedScrollCapture();

  // Update active sidebar item
  document.querySelectorAll('.nav-item').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.view === viewName);
  });

  // Update view pages
  document.querySelectorAll('.view-page').forEach(page => {
    page.classList.remove('active');
  });

  const page = document.getElementById(`view-${viewName}`);
  if (page) {
    page.classList.add('active');
  }

  // Update page title
  const titleMap = {
    'recommend': '推荐',
    'discover': '找剧',
    'rank': '排行榜',
    'new': '新剧',
    'history': '观看历史',
    'favorites': state.favoritesView.tab === 'likes' ? '我的点赞' : '我的收藏',
    'reservations': '我的预约',
    'search-results': '搜索结果',
    'settings': '系统设置',
    'detail': '短剧详情',
    'actor': '演员主页'
  };
  const pageTitle = document.getElementById('page-title');
  if (pageTitle) {
    if (viewName === 'actor' && state.actor.name) {
      pageTitle.textContent = `${state.actor.name} - 演员主页`;
    } else {
      pageTitle.textContent = titleMap[viewName] || 'myguo';
    }
  }

  // Load view content
  switch (viewName) {
    case 'recommend': loadRecommend(); break;
    case 'discover': loadDiscoverFilters(); break;
    case 'rank': loadRankings(); break;
    case 'new': loadNewReleases(); break;
    case 'history': loadHistory(); break;
    case 'favorites': loadFavoritesAndLikes(); break;
    case 'reservations': loadReservations(); break;
    case 'settings': loadSettings(); break;
    case 'detail': if (state.detail.id) loadSeriesDetail(state.detail.id); break;
    case 'actor': if (state.actor.userId) loadActorDetail(state.actor.userId); break;
  }
}

// ==========================================================================
// Drama Detail View (短剧详情)
// ==========================================================================

function openSeriesDetail(seriesId) {
  if (!seriesId) {
    showToast('短剧信息缺失');
    return;
  }
  navigateToDetail(seriesId);
}

function initDetailEvents() {
  const backBtn = document.getElementById('btn-back-from-detail');
  if (backBtn) backBtn.addEventListener('click', navigateBack);

  const playBtn = document.getElementById('btn-detail-play');
  if (playBtn) {
    playBtn.addEventListener('click', () => {
      const detail = state.detail.data;
      const isUpcoming = Boolean(detail?.upcoming) || detail?.episodeCount === 0 || detail?.status === 'upcoming';
      if (isUpcoming && detail) {
        toggleDetailReservation(detail, playBtn);
      } else {
        openPlayer(state.detail.id, 1);
      }
    });
  }

  const favoriteBtn = document.getElementById('btn-detail-favorite');
  if (favoriteBtn) {
    favoriteBtn.addEventListener('click', async () => {
      const detail = state.detail.data;
      if (!detail) return;
      try {
        const isNowFav = await apiCall('ToggleFavorite',
          detail.id,
          detail.title,
          detail.coverUrl,
          detail.description,
          detail.episodeCount
        );
        const updatedDetail = { ...detail, isFavorite: Boolean(isNowFav) };
        state.detail.data = updatedDetail;
        favoriteBtn.classList.toggle('active', updatedDetail.isFavorite);
        favoriteBtn.querySelector('span').textContent = updatedDetail.isFavorite ? '已收藏' : '收藏';
        showToast(updatedDetail.isFavorite ? '已加入收藏' : '已取消收藏');
        state.favoritesView.favoritesList = [];
      } catch (err) {
        showToast('收藏失败: ' + err);
      }
    });
  }
}

async function loadSeriesDetail(seriesId, force = false) {
  if (!seriesId) return;
  if (!force && state.detail.data && state.detail.data.id === seriesId) return;

  const token = ++state.detail.requestToken;
  state.detail.id = seriesId;
  state.detail.loading = true;
  state.detail.data = null;
  state.detail.episodesExpanded = false;
  clearDetailSupplements();

  const loader = document.getElementById('detail-loader');
  const content = document.getElementById('detail-content');
  const errorBox = document.getElementById('detail-error');
  if (loader) loader.style.display = 'grid';
  if (content) content.style.display = 'none';
  if (errorBox) errorBox.style.display = 'none';

  try {
    const detail = await apiCall('GetSeriesPage', seriesId);
    if (token !== state.detail.requestToken || !detail || !detail.id) {
      return;
    }

    state.detail.data = detail;
    renderSeriesDetail(detail);
    if (loader) loader.style.display = 'none';
    if (content) content.style.display = 'block';

    const viewport = document.querySelector('.view-viewport');
    if (viewport && state.currentView === 'detail') viewport.scrollTop = 0;
  } catch (err) {
    if (token !== state.detail.requestToken) return;
    if (loader) loader.style.display = 'none';
    if (errorBox) {
      errorBox.innerHTML = '';
      errorBox.appendChild(renderViewError({
        title: '加载短剧详情失败',
        message: err,
        onRetry: () => loadSeriesDetail(seriesId, true)
      }));
      errorBox.style.display = 'block';
    }
  } finally {
    if (token === state.detail.requestToken) state.detail.loading = false;
  }
}

function renderSeriesDetail(detail) {
  const poster = document.getElementById('detail-poster');
  const title = document.getElementById('detail-title');
  if (poster) {
    poster.src = detail.coverUrl || DEFAULT_POSTER;
    poster.alt = detail.title || '短剧封面';
  }
  if (title) title.textContent = detail.title || '短剧详情';

  const heat = document.getElementById('detail-heat');
  const plays = document.getElementById('detail-plays');
  const follows = document.getElementById('detail-follows');
  const status = document.getElementById('detail-status');
  if (heat) heat.textContent = detail.heat || '热播';
  if (plays) plays.textContent = `${formatCount(detail.playCount, '')}播放`;
  if (follows) follows.textContent = `${formatCount(detail.followCount, '')}人在追`;
  if (status) status.textContent = firstDisplayText(detail.statusText, `${detail.episodeCount || 0}集`);

  const tagsWrap = document.getElementById('detail-tags');
  if (tagsWrap) {
    tagsWrap.innerHTML = '';
    const tags = [];
    if (detail.category) tags.push(detail.category);
    (detail.tags || []).forEach(tag => tags.push(tag));
    if (detail.episodeCount > 0) tags.push(`全${detail.episodeCount}集`);
    Array.from(new Set(tags)).slice(0, 8).forEach(tag => {
      const span = document.createElement('span');
      span.className = 'detail-tag';
      span.textContent = tag;
      tagsWrap.appendChild(span);
    });
  }

  const desc = document.getElementById('detail-description');
  if (desc) desc.textContent = detail.description || '暂无剧情介绍';

  const playBtn = document.getElementById('btn-detail-play');
  const playText = document.querySelector('#btn-detail-play span');
  const isUpcoming = Boolean(detail.upcoming) || detail.episodeCount === 0 || detail.status === 'upcoming';
  if (playBtn && playText) {
    playBtn.classList.toggle('reserved', isUpcoming && Boolean(detail.reserved));
    playBtn.classList.toggle('btn-detail-reserve', isUpcoming);
    if (isUpcoming) {
      playText.textContent = detail.reserved ? '已预约' : '预约短剧';
    } else {
      playText.textContent = '播放第 1 集';
    }
  }
  const favoriteBtn = document.getElementById('btn-detail-favorite');
  if (favoriteBtn) {
    favoriteBtn.classList.toggle('active', Boolean(detail.isFavorite));
    const favoriteText = favoriteBtn.querySelector('span');
    if (favoriteText) favoriteText.textContent = detail.isFavorite ? '已收藏' : '收藏';
  }

  renderDetailActors(detail);
  renderDetailEpisodes(detail);
  renderDetailSupplementPlaceholders(detail);
  renderDetailRelatedPlaceholder(detail);
}

async function toggleDetailReservation(detail, button) {
  const playText = button.querySelector('span');
  const prevText = playText ? playText.textContent : '预约短剧';
  const prevReserved = Boolean(detail.reserved);
  button.disabled = true;
  if (playText) playText.textContent = prevReserved ? '取消中' : '预约中';
  try {
    const res = await apiCall(
      'ToggleReservation',
      detail.id,
      detail.title || '',
      detail.coverUrl || '',
      detail.heat || '',
      ''
    );
    detail.reserved = Boolean(res);
    button.classList.toggle('reserved', detail.reserved);
    if (playText) playText.textContent = detail.reserved ? '已预约' : '预约短剧';
    state.reservationsCache = null;
    showToast(detail.reserved ? `已预约《${detail.title}》` : `已取消预约《${detail.title}》`);
  } catch (err) {
    if (playText) playText.textContent = prevReserved ? '已预约' : '预约短剧';
    showToast(err);
  } finally {
    button.disabled = false;
    if (playText && (playText.textContent === '预约中' || playText.textContent === '取消中')) {
      playText.textContent = prevText;
    }
  }
}

function renderDetailActors(detail) {
  const section = document.getElementById('detail-actors-section');
  const list = document.getElementById('detail-actor-list');
  if (!section || !list) return;
  list.innerHTML = '';
  const actors = detail.actors || [];
  section.style.display = actors.length > 0 ? 'block' : 'none';
  if (actors.length === 0) return;
  actors.forEach(actor => {
    const item = document.createElement('div');
    item.className = 'detail-actor';
    item.innerHTML = `
      <div class="detail-actor-avatar">
        <svg viewBox="0 0 24 24"><path fill="currentColor" d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4s-4 1.79-4 4s1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/></svg>
      </div>
      <span>${escapeHtml(actor.name)}</span>
      ${actor.role ? `<small>${escapeHtml(actor.role)}</small>` : ''}
    `;
    list.appendChild(item);
  });
}

function renderDetailEpisodes(detail) {
  const grid = document.getElementById('detail-episode-grid');
  const count = document.getElementById('detail-episode-count');
  if (!grid) return;
  grid.innerHTML = '';

  const episodes = detail.episodes || [];
  const isUpcoming = Boolean(detail.upcoming) || detail.episodeCount === 0 || detail.status === 'upcoming';
  if (count) {
    count.textContent = isUpcoming ? '待上线' : `共 ${episodes.length || detail.episodeCount || 0} 集`;
  }

  if (episodes.length === 0) {
    grid.innerHTML = isUpcoming
      ? '<div class="detail-empty-episodes">该剧尚未上线，点击上方预约短剧</div>'
      : '<div class="detail-empty-episodes">暂无可播放剧集</div>';
    const existingToggle = document.getElementById('detail-episode-toggle');
    if (existingToggle) existingToggle.remove();
    return;
  }

  const expanded = Boolean(state.detail.episodesExpanded);
  grid.classList.toggle('is-collapsed', !expanded);

  const fragment = document.createDocumentFragment();
  episodes.forEach(ep => {
    const btn = document.createElement('button');
    btn.className = 'detail-episode';
    btn.title = ep.title || `第${ep.number}集`;
    btn.innerHTML = `
      <span class="episode-number">${ep.number}</span>
      <span class="episode-name">${escapeHtml(ep.title || `第${ep.number}集`)}</span>
      <span class="episode-meta">${ep.commentCount > 0 ? formatCount(ep.commentCount, '') + '评论' : '可播放'}</span>
    `;
    btn.onclick = () => openPlayer(detail.id, ep.number);
    fragment.appendChild(btn);
  });
  grid.appendChild(fragment);

  if (state.detail.episodesResizeObserver) {
    state.detail.episodesResizeObserver.disconnect();
    state.detail.episodesResizeObserver = null;
  }

  let toggleWrap = document.getElementById('detail-episode-toggle-wrap');
  let toggle = document.getElementById('detail-episode-toggle');
  if (!toggleWrap && grid) {
    toggleWrap = document.createElement('div');
    toggleWrap.id = 'detail-episode-toggle-wrap';
    toggleWrap.className = 'detail-episode-toggle-wrap';
    toggle = document.createElement('button');
    toggle.id = 'detail-episode-toggle';
    toggle.className = 'detail-episode-toggle';
    toggle.type = 'button';
    toggle.innerHTML = `
      <span class="detail-episode-toggle-text">展开全部</span>
      <svg class="detail-toggle-icon" viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
    `;
    toggleWrap.appendChild(toggle);
    grid.after(toggleWrap);
  }

  const headingToggle = document.getElementById('detail-heading-toggle');

  const updateToggleLabels = (isExp) => {
    if (toggle) {
      toggle.setAttribute('aria-expanded', String(isExp));
      const toggleText = toggle.querySelector('.detail-episode-toggle-text');
      if (toggleText) {
        toggleText.textContent = isExp ? '收起选集' : `展开全部 ${formatCount(episodes.length, '')} 集`;
      }
    }
    if (headingToggle) {
      headingToggle.setAttribute('aria-expanded', String(isExp));
      const headingText = headingToggle.querySelector('.detail-heading-toggle-text');
      if (headingText) {
        headingText.textContent = isExp ? '收起' : '展开全部';
      }
    }
  };

  const setEpisodesExpanded = (nextExp) => {
    state.detail.episodesExpanded = nextExp;
    grid.classList.toggle('is-collapsed', !nextExp);
    updateToggleLabels(nextExp);
    if (!nextExp) {
      const section = grid.closest('.detail-episodes-section') || grid;
      const rect = section.getBoundingClientRect();
      if (rect.top < 70) {
        section.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    }
  };

  if (toggle) {
    toggle.onclick = () => setEpisodesExpanded(!state.detail.episodesExpanded);
  }
  if (headingToggle) {
    headingToggle.onclick = () => setEpisodesExpanded(!state.detail.episodesExpanded);
  }

  updateToggleLabels(expanded);

  const syncToggle = () => {
    if (episodes.length === 0) {
      if (toggleWrap) toggleWrap.style.display = 'none';
      if (headingToggle) headingToggle.style.display = 'none';
      return;
    }
    const isExp = Boolean(state.detail.episodesExpanded);
    if (isExp) {
      if (toggleWrap) toggleWrap.style.display = 'flex';
      if (headingToggle) headingToggle.style.display = 'inline-flex';
      return;
    }
    // In collapsed mode: check if content overflows 2 rows
    let hasOverflow = false;
    if (grid.clientHeight > 0) {
      hasOverflow = grid.scrollHeight > grid.clientHeight + 1;
    } else {
      // Content container not yet rendered (e.g. during initial loading):
      // Estimate based on episode count. 2 rows holds at most 16 cards on wide displays.
      hasOverflow = episodes.length > 12;
    }
    if (toggleWrap) toggleWrap.style.display = hasOverflow ? 'flex' : 'none';
    if (headingToggle) headingToggle.style.display = hasOverflow ? 'inline-flex' : 'none';
  };

  syncToggle();
  if (typeof requestAnimationFrame !== 'undefined') {
    requestAnimationFrame(syncToggle);
  }
  if (typeof ResizeObserver !== 'undefined') {
    state.detail.episodesResizeObserver = new ResizeObserver(syncToggle);
    state.detail.episodesResizeObserver.observe(grid);
  }
}

function renderDetailCollection(sectionId, gridId, items, fallbackCover = '') {
  const section = document.getElementById(sectionId);
  const grid = document.getElementById(gridId);
  if (!section || !grid) return;
  grid.innerHTML = '';
  section.style.display = items.length > 0 ? 'block' : 'none';
  items.forEach(item => {
    if (!item || !item.id) return;
    grid.appendChild(createDramaCard(item, fallbackCover));
  });
}

function clearDetailSupplements() {
  Object.values(state.detail.collectionObservers || {}).forEach(observer => observer?.disconnect?.());
  state.detail.collectionObservers = {};
  state.detail.collectionPromises = {};
  state.detail.collectionData = {};
}

function renderDetailSupplementPlaceholders(detail) {
  prepareDetailSupplement(
    'detail-seasons-section',
    'detail-seasons-title',
    'detail-seasons-status',
    'detail-seasons-grid',
    detail?.id
  );
  observeDetailSupplement('seasons', 'detail-seasons-section', () => {
    ensureDetailSeasons(detail, state.detail.requestToken).catch(() => {});
  });

  const sourceTitle = detail?.source?.title?.trim();
  if (!sourceTitle) {
    const adaptedSection = document.getElementById('detail-adapted-section');
    if (adaptedSection) adaptedSection.style.display = 'none';
    return;
  }

  prepareDetailSupplement(
    'detail-adapted-section',
    'detail-adapted-title',
    'detail-adapted-status',
    'detail-adapted-grid',
    detail?.id,
    '同一剧本改编'
  );
  observeDetailSupplement('adaptations', 'detail-adapted-section', () => {
    ensureDetailAdaptations(detail, state.detail.requestToken).catch(() => {});
  });
}

function prepareDetailSupplement(sectionId, titleId, statusId, gridId, seriesId, title = '同系列短剧') {
  const section = document.getElementById(sectionId);
  if (!section) return;
  section.style.display = 'block';
  section.dataset.state = 'pending';
  section.dataset.seriesId = seriesId || '';
  const titleElem = document.getElementById(titleId);
  const status = document.getElementById(statusId);
  const grid = document.getElementById(gridId);
  if (titleElem) titleElem.textContent = title;
  if (status) status.textContent = '下滑加载';
  if (grid) grid.innerHTML = renderDramaGridSkeleton(6);
}

function observeDetailSupplement(kind, sectionId, load) {
  const section = document.getElementById(sectionId);
  const viewport = document.querySelector('.view-viewport');
  if (!section || !viewport || typeof IntersectionObserver === 'undefined') {
    load();
    return;
  }

  const observer = new IntersectionObserver((entries) => {
    if (!entries.some(entry => entry.isIntersecting)) return;
    observer.disconnect();
    delete state.detail.collectionObservers[kind];
    load();
  }, { root: viewport, rootMargin: '420px 0px' });

  state.detail.collectionObservers[kind] = observer;
  observer.observe(section);
}

function ensureDetailSeasons(detail, token) {
  const existing = state.detail.collectionPromises.seasons;
  if (existing && existing.seriesId === detail.id) return existing.promise;

  const promise = loadDetailSeasons(detail.id, token)
    .then(collection => {
      if (token === state.detail.requestToken && collection) {
        state.detail.collectionData.seasons = collection;
      }
      return collection;
    });
  state.detail.collectionPromises.seasons = { seriesId: detail.id, promise };
  return promise;
}

function ensureDetailAdaptations(detail, token) {
  const existing = state.detail.collectionPromises.adaptations;
  if (existing && existing.seriesId === detail.id) return existing.promise;

  const promise = ensureDetailSeasons(detail, token)
    .catch(() => null)
    .then(seasons => {
      if (token !== state.detail.requestToken) return null;
      const excludeIDs = [detail.id, ...((seasons?.items || []).map(item => item.id))]
        .filter(Boolean).join(',');
      return loadDetailAdaptations(detail, excludeIDs, token);
    })
    .then(collection => {
      if (token === state.detail.requestToken && collection) {
        state.detail.collectionData.adaptations = collection;
      }
      return collection;
    });
  state.detail.collectionPromises.adaptations = { seriesId: detail.id, promise };
  return promise;
}

async function loadDetailSeasons(seriesId, token) {
  const section = document.getElementById('detail-seasons-section');
  const status = document.getElementById('detail-seasons-status');
  const grid = document.getElementById('detail-seasons-grid');
  if (!section || !grid) return null;

  section.dataset.state = 'loading';
  if (status) status.textContent = '加载中';
  try {
    const collection = await apiCall('GetSeriesSeasons', seriesId);
    if (token !== state.detail.requestToken || seriesId !== state.detail.id) return null;
    renderLoadedDetailCollection('detail-seasons-section', 'detail-seasons-grid', collection, state.detail.data?.coverUrl || '');
    return collection;
  } catch (err) {
    if (token !== state.detail.requestToken || seriesId !== state.detail.id) return null;
    renderDetailSupplementError('seasons', '同系列短剧加载失败', () => {
      state.detail.collectionPromises.seasons = null;
      ensureDetailSeasons(state.detail.data, state.detail.requestToken);
    });
    throw err;
  }
}

async function loadDetailAdaptations(detail, excludeIDs, token) {
  const section = document.getElementById('detail-adapted-section');
  const status = document.getElementById('detail-adapted-status');
  const grid = document.getElementById('detail-adapted-grid');
  if (!section || !grid) return null;

  section.dataset.state = 'loading';
  if (status) status.textContent = '加载中';
  try {
    const collection = await apiCall(
      'GetSeriesAdaptations',
      detail.id,
      detail.source?.id || '',
      detail.source?.title || '',
      excludeIDs
    );
    if (token !== state.detail.requestToken || detail.id !== state.detail.id) return null;
    renderLoadedDetailCollection('detail-adapted-section', 'detail-adapted-grid', collection, detail.coverUrl || '');
    return collection;
  } catch (err) {
    if (token !== state.detail.requestToken || detail.id !== state.detail.id) return null;
    renderDetailSupplementError('adaptations', '改编短剧加载失败', () => {
      state.detail.collectionPromises.adaptations = null;
      ensureDetailAdaptations(state.detail.data, state.detail.requestToken);
    });
    throw err;
  }
}

function renderLoadedDetailCollection(sectionId, gridId, collection, fallbackCover) {
  const section = document.getElementById(sectionId);
  const kind = sectionId.replace('-section', '');
  const title = document.getElementById(`${kind}-title`);
  const status = document.getElementById(`${kind}-status`);
  const items = collection?.items || [];
  if (section && title && collection?.title) {
    title.textContent = collection.title;
  }
  if (section && status && items.length > 0) {
    section.dataset.state = 'loaded';
    status.textContent = `${items.length}部`;
  }
  renderDetailCollection(sectionId, gridId, items, fallbackCover);
}

function renderDetailSupplementError(kind, label, retry) {
  const section = document.getElementById(`detail-${kind}-section`);
  const status = document.getElementById(`detail-${kind}-status`);
  const grid = document.getElementById(`detail-${kind}-grid`);
  if (!section || !grid) return;
  section.dataset.state = 'error';
  if (status) status.textContent = '加载失败';
  grid.innerHTML = `
    <div class="detail-collection-error">
      <span>${label}</span>
      <button type="button" id="btn-detail-${kind}-retry">重试</button>
    </div>
  `;
  const retryBtn = document.getElementById(`btn-detail-${kind}-retry`);
  if (retryBtn) retryBtn.onclick = () => {
    section.dataset.state = 'loading';
    if (status) status.textContent = '加载中';
    grid.innerHTML = renderDramaGridSkeleton(6);
    retry();
  };
}

function renderDetailRelatedPlaceholder(detail) {
  const section = document.getElementById('detail-related-section');
  const status = document.getElementById('detail-related-status');
  const grid = document.getElementById('detail-related-grid');
  if (!section || !grid) return;

  if (state.detail.relatedObserver) {
    state.detail.relatedObserver.disconnect();
    state.detail.relatedObserver = null;
  }

  section.style.display = 'block';
  section.dataset.state = 'pending';
  section.dataset.seriesId = detail.id;
  if (status) status.textContent = '下滑加载';
  grid.innerHTML = `
    <div class="detail-inline-loader" aria-label="加载中">
      <svg class="loading-spinner" viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="10"></circle>
        <path d="M6.12 20.09A10 10 0 1 1 17.88 20.09"></path>
      </svg>
    </div>
  `;

  const viewport = document.querySelector('.view-viewport');
  if (!viewport || typeof IntersectionObserver === 'undefined') {
    loadDetailRelated(detail.id, state.detail.requestToken);
    return;
  }

  const observer = new IntersectionObserver((entries) => {
    if (!entries.some(entry => entry.isIntersecting)) return;
    observer.disconnect();
    state.detail.relatedObserver = null;
    loadDetailRelated(detail.id, state.detail.requestToken);
  }, { root: viewport, rootMargin: '360px 0px' });

  state.detail.relatedObserver = observer;
  observer.observe(section);
}

async function loadDetailRelated(seriesId, token) {
  const section = document.getElementById('detail-related-section');
  const status = document.getElementById('detail-related-status');
  const grid = document.getElementById('detail-related-grid');
  if (!section || !grid || section.dataset.state === 'loaded') return;
  section.dataset.state = 'loading';
  if (status) status.textContent = '加载中';

  try {
    const related = await apiCall('GetSeriesRelated', seriesId);
    if (token !== state.detail.requestToken || seriesId !== state.detail.id) return;

    const items = related?.items || [];
    if (items.length === 0) {
      section.style.display = 'none';
      return;
    }

    section.dataset.state = 'loaded';
    const titleElem = section.querySelector('.detail-section-title');
    if (titleElem && related?.title) titleElem.textContent = related.title;
    renderDetailCollection('detail-related-section', 'detail-related-grid', items, state.detail.data?.coverUrl || '');
    if (status) status.textContent = `${items.length}部`;
  } catch (err) {
    if (token !== state.detail.requestToken || seriesId !== state.detail.id) return;
    section.dataset.state = 'error';
    if (status) status.textContent = '加载失败';
    grid.innerHTML = `
      <div class="detail-collection-error">
        <span>相关推荐加载失败</span>
        <button type="button" id="btn-detail-related-retry">重试</button>
      </div>
    `;
    const retryBtn = document.getElementById('btn-detail-related-retry');
    if (retryBtn) retryBtn.onclick = () => loadDetailRelated(seriesId, token);
  }
}

function firstDisplayText(value, fallback) {
  return value && String(value).trim() ? value : fallback;
}

// ==========================================================================
// User Profile & Badges
// ==========================================================================

async function loadProfile() {
  try {
    const profile = await apiCall('GetProfile');
    state.profile = profile;
    if (profile) {
      const isLogin = Boolean(profile.loggedIn);
      const nickname = profile.nickname || (isLogin ? '已登录用户' : 'myguo用户');
      const avatarSrc = profile.avatar || '';

      // Settings page elements
      const setNick = document.getElementById('settings-nickname');
      if (setNick) setNick.textContent = nickname;

      const setAvatar = document.getElementById('settings-avatar');
      if (setAvatar && avatarSrc) setAvatar.src = avatarSrc;

      const setIndicator = document.getElementById('settings-status-indicator');
      if (setIndicator) setIndicator.classList.toggle('online', isLogin);

      const setBadge = document.getElementById('settings-login-badge');
      if (setBadge) {
        setBadge.textContent = isLogin ? '已登录官方账号' : '匿名设备访问';
        setBadge.classList.toggle('badge-success', isLogin);
      }

      const setUid = document.getElementById('settings-uid');
      if (setUid) setUid.textContent = profile.userId || (isLogin ? '已同步' : 'anonymous');

      // Buttons visibility
      const btnLogout = document.getElementById('btn-logout');
      const btnShowLogin = document.getElementById('btn-show-login');
      if (btnLogout) btnLogout.style.display = isLogin ? 'inline-flex' : 'none';
      if (btnShowLogin) {
        const textSpan = btnShowLogin.querySelector('span');
        if (textSpan) textSpan.textContent = isLogin ? '重新登录' : '立即登录';
      }
    }
  } catch (err) {
    console.error('loadProfile error:', err);
  }
}

// ==========================================================================
// Sidebar Resizer & Custom Width
// ==========================================================================

const SIDEBAR_STORAGE_KEY = 'hongguo_sidebar_width';
const SIDEBAR_MIN_WIDTH = 160;
const SIDEBAR_DEFAULT_WIDTH = 210;

function getSidebarMaxWidth() {
  return Math.max(SIDEBAR_MIN_WIDTH, Math.min(420, Math.floor(window.innerWidth * 0.45)));
}

function applySidebarWidth(width, persist = true) {
  const sidebar = document.getElementById('sidebar') || document.querySelector('.sidebar');
  if (!sidebar) return;

  const maxWidth = getSidebarMaxWidth();
  const clamped = Math.max(SIDEBAR_MIN_WIDTH, Math.min(maxWidth, Math.round(width)));

  sidebar.style.width = `${clamped}px`;
  document.documentElement.style.setProperty('--sidebar-width', `${clamped}px`);

  if (persist) {
    try {
      localStorage.setItem(SIDEBAR_STORAGE_KEY, clamped.toString());
    } catch (_) {}
  }
  return clamped;
}

function initSidebarResizer() {
  const sidebar = document.getElementById('sidebar') || document.querySelector('.sidebar');
  const resizer = document.getElementById('sidebar-resizer');
  if (!sidebar || !resizer) return;

  // Restore saved width
  let initialWidth = SIDEBAR_DEFAULT_WIDTH;
  try {
    const saved = localStorage.getItem(SIDEBAR_STORAGE_KEY);
    if (saved) {
      const parsed = parseInt(saved, 10);
      if (!isNaN(parsed) && parsed >= SIDEBAR_MIN_WIDTH) {
        initialWidth = parsed;
      }
    }
  } catch (_) {}
  applySidebarWidth(initialWidth, false);

  let isDragging = false;
  let startX = 0;
  let startWidth = 0;
  let pendingWidth = 0;
  let resizeFrame = 0;

  const paintResize = () => {
    resizeFrame = 0;
    applySidebarWidth(pendingWidth, false);
  };

  const onPointerMove = (e) => {
    if (!isDragging) return;
    e.preventDefault();
    pendingWidth = startWidth + (e.clientX - startX);
    if (!resizeFrame) resizeFrame = requestAnimationFrame(paintResize);
  };

  const onPointerUp = (e) => {
    if (!isDragging) return;
    isDragging = false;
    if (resizeFrame) {
      cancelAnimationFrame(resizeFrame);
      resizeFrame = 0;
    }
    applySidebarWidth(pendingWidth, true);
    try {
      resizer.releasePointerCapture(e.pointerId);
    } catch (_) {}
    window.removeEventListener('pointermove', onPointerMove);
    window.removeEventListener('pointerup', onPointerUp);
    window.removeEventListener('pointercancel', onPointerUp);
    document.body.classList.remove('is-resizing-sidebar');
  };

  const onPointerDown = (e) => {
    if (e.button !== 0) return; // Left click only
    e.preventDefault();
    isDragging = true;
    startX = e.clientX;
    startWidth = sidebar.getBoundingClientRect().width;
    pendingWidth = startWidth;

    try {
      resizer.setPointerCapture(e.pointerId);
    } catch (_) {}

    document.body.classList.add('is-resizing-sidebar');
    window.addEventListener('pointermove', onPointerMove);
    window.addEventListener('pointerup', onPointerUp);
    window.addEventListener('pointercancel', onPointerUp);
  };

  resizer.addEventListener('pointerdown', onPointerDown);

  // Double click resets to default width (210px)
  resizer.addEventListener('dblclick', (e) => {
    e.preventDefault();
    applySidebarWidth(SIDEBAR_DEFAULT_WIDTH, true);
  });

  // Clamp width when window is resized
  window.addEventListener('resize', () => {
    const current = sidebar.getBoundingClientRect().width;
    const maxW = getSidebarMaxWidth();
    if (current > maxW) {
      applySidebarWidth(maxW, true);
    }
  });
}

function initSidebarFooter() {
  // Deprecated: user profile box removed from sidebar
}

async function loadSettings() {
  await loadProfile();

  // 1. Fetch cache size
  try {
    const bytes = await apiCall('GetCacheSize');
    const mb = ((bytes || 0) / (1024 * 1024)).toFixed(1);
    const cacheEl = document.getElementById('settings-cache-size');
    if (cacheEl) cacheEl.textContent = `${mb} MB`;
  } catch (err) {
    console.error('GetCacheSize error:', err);
  }

  // 2. Fetch device info
  try {
    const devInfo = await apiCall('GetDeviceInfo');
    if (devInfo) {
      const devEl = document.getElementById('settings-device-id');
      if (devEl) devEl.textContent = devInfo.deviceId || '--';
    }
  } catch (err) {
    console.error('GetDeviceInfo error:', err);
  }

  // 3. Check for existing hgplayer account (only display when user is not logged in)
  try {
    const importBanner = document.getElementById('settings-import-banner');
    if (importBanner) {
      const isLogin = Boolean(state.profile && state.profile.loggedIn);
      if (isLogin) {
        // Once logged in or synced, remove/hide the import banner completely
        importBanner.style.display = 'none';
      } else {
        const oldAcc = await apiCall('CheckExistingAccount');
        if (oldAcc && oldAcc.exists) {
          importBanner.style.display = 'flex';
          const desc = document.getElementById('import-banner-desc');
          if (desc) {
            desc.textContent = `检测到本机旧版账号：${oldAcc.nickname || oldAcc.userId}，支持一键同步`;
          }
        } else {
          importBanner.style.display = 'none';
        }
      }
    }
  } catch (err) {
    console.error('CheckExistingAccount error:', err);
  }
}

function initSettingsEvents() {
  // Clear cache in settings
  const btnClear = document.getElementById('btn-settings-clear-cache');
  if (btnClear) {
    btnClear.addEventListener('click', async () => {
      try {
        const bytes = await apiCall('ClearCache');
        const mb = (bytes / (1024 * 1024)).toFixed(1);
        const cacheEl = document.getElementById('settings-cache-size');
        if (cacheEl) cacheEl.textContent = '0.0 MB';
        showToast(`已清理 ${mb} MB 本地媒体缓存`);
      } catch (err) {
        showToast('清理失败: ' + err);
      }
    });
  }

  // Reset device in settings
  const btnReset = document.getElementById('btn-settings-reset-device');
  if (btnReset) {
    btnReset.addEventListener('click', async () => {
      if (await nativeConfirm('确认重置匿名设备指纹？', '将重新注册设备标识并重置推荐算法。', '重置设备', true)) {
        try {
          await apiCall('ResetDevice');
          showToast('已成功重置匿名设备');
          await loadSettings();
        } catch (err) {
          showToast('重置失败: ' + err);
        }
      }
    });
  }

  // Refresh user profile
  const btnRefresh = document.getElementById('btn-refresh-profile');
  if (btnRefresh) {
    btnRefresh.addEventListener('click', async () => {
      try {
        btnRefresh.disabled = true;
        await apiCall('RefreshUserProfile');
        await loadProfile();
        showToast('个人资料已刷新');
      } catch (err) {
        showToast('刷新失败: ' + (err.message || err));
      } finally {
        btnRefresh.disabled = false;
      }
    });
  }

  // Logout button
  const btnLogout = document.getElementById('btn-logout');
  if (btnLogout) {
    btnLogout.addEventListener('click', async () => {
      if (await nativeConfirm('确定要退出当前登录账号吗？', '将清除登录凭据并切换为匿名访问。', '退出登录', true)) {
        try {
          await apiCall('Logout');
          showToast('已安全退出登录');
          await loadProfile();
          await loadSettings();
        } catch (err) {
          showToast('退出登录失败: ' + err);
        }
      }
    });
  }

  // Toggle login panel
  const loginPanel = document.getElementById('settings-login-panel');
  const btnShowLogin = document.getElementById('btn-show-login');
  const btnCancelLogin = document.getElementById('btn-cancel-login');

  if (btnShowLogin && loginPanel) {
    btnShowLogin.addEventListener('click', () => {
      const isVisible = loginPanel.style.display !== 'none';
      loginPanel.style.display = isVisible ? 'none' : 'flex';
      if (!isVisible) {
        document.getElementById('login-cookie-input')?.focus();
      }
    });
  }

  if (btnCancelLogin && loginPanel) {
    btnCancelLogin.addEventListener('click', () => {
      loginPanel.style.display = 'none';
    });
  }

  // Paste from clipboard helper
  const btnPaste = document.getElementById('btn-paste-clipboard');
  if (btnPaste) {
    btnPaste.addEventListener('click', async () => {
      try {
        const text = await navigator.clipboard.readText();
        if (!text) {
          showToast('剪贴板为空');
          return;
        }
        const trimmed = text.trim();
        // Check if user copied a JSON object
        if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
          try {
            const parsed = JSON.parse(trimmed);
            if (parsed.cookies) {
              document.getElementById('login-cookie-input').value = parsed.cookies;
              if (parsed.token) {
                document.getElementById('login-token-input').value = parsed.token;
              }
              showToast('已解析并自动填入剪贴板中的凭据');
              return;
            }
          } catch (e) {}
        }
        document.getElementById('login-cookie-input').value = trimmed;
        showToast('已填入剪贴板内容');
      } catch (err) {
        showToast('读取剪贴板受阻，请直接通过快捷键粘贴');
      }
    });
  }

  // Submit login
  const btnSubmit = document.getElementById('btn-submit-login');
  if (btnSubmit) {
    btnSubmit.addEventListener('click', async () => {
      const cookieInput = document.getElementById('login-cookie-input');
      const tokenInput = document.getElementById('login-token-input');
      const cookies = cookieInput ? cookieInput.value.trim() : '';
      const token = tokenInput ? tokenInput.value.trim() : '';

      if (!cookies) {
        showToast('请输入有效的 Cookie 凭据');
        return;
      }

      const origText = btnSubmit.innerHTML;
      btnSubmit.disabled = true;
      btnSubmit.innerHTML = '<span>验证登录中...</span>';

      try {
        const profile = await apiCall('LoginWithCookies', cookies, token);
        showToast(`登录成功！欢迎回来，${profile?.nickname || '用户'}`);
        if (loginPanel) loginPanel.style.display = 'none';
        const importBanner = document.getElementById('settings-import-banner');
        if (importBanner) importBanner.style.display = 'none';
        if (cookieInput) cookieInput.value = '';
        if (tokenInput) tokenInput.value = '';

        await loadProfile();
        await loadSettings();
      } catch (err) {
        showToast('登录验证失败: ' + (err.message || err));
      } finally {
        btnSubmit.disabled = false;
        btnSubmit.innerHTML = origText;
      }
    });
  }

  // Import old hgplayer account
  const btnImportOld = document.getElementById('btn-import-old-account');
  if (btnImportOld) {
    btnImportOld.addEventListener('click', async () => {
      btnImportOld.disabled = true;
      btnImportOld.textContent = '同步中...';
      try {
        const profile = await apiCall('ImportHgplayerAccount');
        showToast(`账号导入成功！当前用户：${profile?.nickname || '用户'}`);
        const importBanner = document.getElementById('settings-import-banner');
        if (importBanner) importBanner.style.display = 'none';
        await loadProfile();
        await loadSettings();
      } catch (err) {
        showToast('导入失败: ' + (err.message || err));
      } finally {
        btnImportOld.disabled = false;
        btnImportOld.textContent = '一键同步登录';
      }
    });
  }
}

// ==========================================================================
// 1. Recommend View (Vertical Swipe Video Feed - TikTok/Hongguo Style)
// ==========================================================================

let feedHintTimer = null;
let feedRequestToken = 0;
function showFeedHint(text, spinner = false, duration = 2200) {
  const toast = document.getElementById('feed-hint-toast');
  const txt = document.getElementById('feed-hint-text');
  const spin = document.getElementById('feed-hint-spinner');
  if (!toast || !txt) return;

  txt.textContent = text;
  if (spin) spin.style.display = spinner ? 'inline-block' : 'none';
  toast.style.display = 'flex';

  clearTimeout(feedHintTimer);
  feedHintTimer = setTimeout(() => {
    toast.style.display = 'none';
  }, duration);
}

async function loadRecommend() {
  syncFeedScrollCapture();
  if (state.feed.items.length === 0) {
    await loadFeed(true);
  } else {
    resumeFeedPlayback();
  }
}

async function loadFeed(reset = false, channel = '', forceRefresh = false) {
  if (!reset && state.feed.loading) return;
  const requestToken = ++feedRequestToken;
  state.feed.loading = true;
  if (state.feed.pageRetryTimer) {
    clearTimeout(state.feed.pageRetryTimer);
    state.feed.pageRetryTimer = null;
  }

  if (reset) {
    cancelScheduledFeedWarmup();
    resetFeedVideoPool();
    state.feed.items = [];
    state.feed.index = 0;
    state.feed.hasMore = true;
    state.feed.cursor = '';
    state.feed.preloadMap.clear();
    state.feed.playbackInfoLoads.clear();
    if (channel !== undefined) {
      state.feed.channel = channel;
    }
    if (forceRefresh) {
      state.feed.cursor = '';
    }

    showFeedSkeleton(true);
  }

  try {
    const queryCursor = forceRefresh ? 'refresh' : (reset ? '' : state.feed.cursor);
    const method = reset ? 'GetRecommendFeed' : 'GetRecommendFeedBatch';
    const res = await apiCall(method, state.feed.channel, queryCursor, reset ? 1 : 12);
    if (requestToken !== feedRequestToken) return;

    if (!res || !res.items || res.items.length === 0) {
      showFeedSkeleton(false);
      // A recommendation API can return an empty recoverable page. Only the
      // finite favorites channel should turn emptiness into a hard boundary.
      const canRetry = state.feed.channel !== '关注' && res?.hasMore !== false;
      state.feed.hasMore = canRetry;
      if (!reset) state.feed.emptyFeedPages++;
      if (reset) {
        if (!canRetry) showFeedHint('暂无关注内容', false, 3000);
        else showFeedHint('正在换个推荐流...', true, 2200);
      }
      return;
    }

    if (reset) {
      state.feed.items = res.items;
      state.feed.cursor = res.cursor || '';
      state.feed.hasMore = Boolean(res.hasMore);
      state.feed.emptyFeedPages = 0;
      if (state.currentView === 'recommend') {
        playFeedIndex(0).catch(err => console.error('playFeedIndex error:', err));
      } else {
        state.feed.index = 0;
      }
    } else {
      // Append non-duplicate items
      const existingIds = new Set(state.feed.items.map(it => it.id));
      const newItems = res.items.filter(it => !existingIds.has(it.id));
      state.feed.items.push(...newItems);
      state.feed.cursor = res.cursor || '';
      state.feed.hasMore = Boolean(res.hasMore);
      if (newItems.length > 0) state.feed.emptyFeedPages = 0;
      else state.feed.emptyFeedPages++;
    }
  } catch (err) {
    console.error('loadFeed error:', err);
    showFeedSkeleton(false);
    if (!reset) {
      state.feed.emptyFeedPages++;
    } else {
      showFeedHint('加载推荐失败: ' + err, false, 3000);
    }
  } finally {
    if (requestToken === feedRequestToken) {
      state.feed.loading = false;
      const loader = document.getElementById('feed-loader');
      if (loader) loader.style.display = 'none';
      ensureFeedItemsAhead();
    }
  }
}

function ensureFeedItemsAhead(minAhead = 8) {
  if (state.currentView !== 'recommend' || !state.feed.hasMore || state.feed.loading) return;
  const remaining = state.feed.items.length - (state.feed.index + 1);
  if (remaining >= minAhead) return;
  const loadPage = () => {
    state.feed.pageRetryTimer = null;
    if (state.currentView !== 'recommend' || !state.feed.hasMore || state.feed.loading) return;
    if (state.feed.items.length - (state.feed.index + 1) >= minAhead) return;
    loadFeed(false).catch(err => console.warn('prefetch recommend page failed:', err));
  };
  if (state.feed.emptyFeedPages > 0) {
    if (state.feed.pageRetryTimer) return;
    const delay = Math.min(6000, 750 * state.feed.emptyFeedPages);
    state.feed.pageRetryTimer = setTimeout(loadPage, delay);
    return;
  }
  loadPage();
}

function updateFeedDramaInfo(drama, episodeNum = 1) {
  if (!drama) return;
  const titleElem = document.getElementById('feed-info-title');
  if (titleElem) {
    titleElem.textContent = drama.title;
    titleElem.classList.add('feed-title-link');
    titleElem.onclick = () => openSeriesDetail(drama.id);
  }

  const descElem = document.getElementById('feed-info-desc');
  if (descElem) descElem.textContent = drama.description || drama.author || '精彩短剧，全集免费畅看';

  const rankText = document.getElementById('feed-rank-text');
  if (rankText) rankText.textContent = drama.heat || '官方推荐';
  updateFeedCommentCount(0);

  const epBadge = document.getElementById('feed-episode-badge');
  if (epBadge) epBadge.textContent = `第 ${episodeNum} 集`;

  const tagsContainer = document.getElementById('feed-info-tags');
  if (tagsContainer) {
    tagsContainer.innerHTML = '';
    const tags = Array.isArray(drama.tags) && drama.tags.length > 0 ? drama.tags : [drama.category || '短剧'];
    tags.forEach(tag => {
      if (tag) {
        const span = document.createElement('span');
        span.className = 'feed-info-tag';
        span.textContent = tag;
        tagsContainer.appendChild(span);
      }
    });
  }
}

function activateFeedDrama(drama, itemIndex, slot = null) {
  if (!drama) return;
  state.feed.index = itemIndex;
  state.feed.seriesId = drama.id;
  state.feed.seriesTitle = drama.title;
  state.feed.seriesCover = drama.coverUrl;
  state.feed.episodeNum = slot?.episodeNum || 1;
  state.feed.episodeId = slot?.episodeId || '';
  state.feed.totalEpisodes = slot?.totalEpisodes || 1;
  state.feed.episodes = slot?.episodes || [];
  state.feed.commentCount = slot?.commentCount || 0;
  updateFeedDramaInfo(drama, state.feed.episodeNum);
  updateFeedCommentCount(state.feed.commentCount);
}

function loadFeedDramaMetadata(drama) {
  if (!drama) return;
  apiCall('IsFavorite', drama.id).then(isFavorite => {
    if (state.feed.seriesId !== drama.id) return;
    state.feed.isFavorite = Boolean(isFavorite);
    document.getElementById('feed-fav')?.classList.toggle('active', Boolean(isFavorite));
  }).catch(() => {});

  apiCall('GetSeriesDetail', drama.id).then(detail => {
    if (!detail || detail.id !== state.feed.seriesId) return;
    const episodes = detail.episodes || [];
    const totalEpisodes = detail.episodeCount || episodes.length || 1;
    const currentSlot = feedSlots[feedSlotCurr];
    if (currentSlot?.seriesId === drama.id) {
      currentSlot.totalEpisodes = totalEpisodes;
      currentSlot.episodes = episodes;
    }
    if (state.feed.seriesId !== drama.id) return;

    state.feed.totalEpisodes = totalEpisodes;
    state.feed.episodes = episodes;
    const currentEpisode = episodes.find(episode => episode.number === state.feed.episodeNum);
    const commentCount = currentEpisode ? currentEpisode.commentCount : detail.commentCount || 0;
    state.feed.commentCount = Number(commentCount) || 0;
    if (currentSlot?.seriesId === drama.id) currentSlot.commentCount = state.feed.commentCount;
    updateFeedCommentCount(state.feed.commentCount);
    renderFeedPlaylistDrawer();
  }).catch(() => {});
}

async function playFeedIndex(idx, resumeAt = null) {
  if (idx < 0 || idx >= state.feed.items.length) return;
  const drama = state.feed.items[idx];
  if (!drama) return;

  const currentSlot = feedSlots[feedSlotCurr];
  activateFeedDrama(drama, idx, currentSlot?.itemIndex === idx ? currentSlot : null);
  loadFeedDramaMetadata(drama);

  if (state.currentView !== 'recommend') return;

  await prepareFeedSlot(currentSlot, idx, true, resumeAt);

  if (
    state.currentView === 'recommend' &&
    feedSlots[feedSlotCurr] === currentSlot &&
    currentSlot.itemIndex === idx &&
    !currentSlot.video?.src
  ) {
    setTimeout(() => {
      if (
        state.currentView === 'recommend' &&
        feedSlots[feedSlotCurr] === currentSlot &&
        currentSlot.itemIndex === idx &&
        !currentSlot.video?.src
      ) {
        scheduleFeedRecoveryAdvance(currentSlot, idx);
      }
    }, 600);
  }
  ensureFeedItemsAhead();
}

let feedNeedsGestureUnmute = false;

function attachFeedGestureUnlock() {
  document.addEventListener('pointerdown', handleFeedGestureUnlock, true);
  document.addEventListener('keydown', handleFeedGestureUnlock, true);
  document.addEventListener('wheel', handleFeedGestureUnlock, true);
}

function detachFeedGestureUnlock() {
  document.removeEventListener('pointerdown', handleFeedGestureUnlock, true);
  document.removeEventListener('keydown', handleFeedGestureUnlock, true);
  document.removeEventListener('wheel', handleFeedGestureUnlock, true);
}

function handleFeedGestureUnlock(event) {
  if (!feedNeedsGestureUnmute) return;
  if (event.type === 'keydown' && ![' ', 'Enter'].includes(event.key)) return;
  if (event.type === 'wheel') {
    if (!event.target?.closest?.('#feed-stage')) return;
    unlockFeedSound();
    return;
  }

  const target = event.target?.closest?.('#feed-stage, .feed-controls, #feed-play-indicator');
  if (!target) return;

  const feedVideo = getFeedVideo();
  const playControl = event.target?.closest?.('#feed-play, #feed-play-indicator');
  if (feedVideo && !feedVideo.paused && !playControl && target.closest('#feed-stage')) {
    event.preventDefault();
    event.stopPropagation();
  }
  unlockFeedSound();
}

function unlockFeedSound() {
  if (!feedNeedsGestureUnmute) return;
  const feedVideo = getFeedVideo();
  feedNeedsGestureUnmute = false;
  if (feedVideo?.dataset?.temporaryMute) delete feedVideo.dataset.temporaryMute;
  detachFeedGestureUnlock();
  if (!feedVideo) return;
  feedVideo.muted = false;
  feedVideo.volume = 1;
  if (feedVideo.paused && !state.feed.userPaused && state.currentView === 'recommend') {
    feedPlayPromise = feedVideo.play();
    feedPlayPromise?.catch(e => console.log('unmute feed play error:', e));
  }
  showFeedHint('声音已开启', false, 1200);
}

function restoreFeedFirstFrameSound(feedVideo) {
  if (!feedVideo || feedVideo.dataset.temporaryMute !== 'first-frame') return;
  delete feedVideo.dataset.temporaryMute;
  feedVideo.muted = false;
  feedVideo.volume = 1;
}

function feedVideoHasPlaybackBuffer(video) {
  if (!video?.src || video.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) return false;
  if (video.readyState >= HTMLMediaElement.HAVE_ENOUGH_DATA) return true;
  const position = video.currentTime || 0;
  for (let i = 0; i < video.buffered.length; i++) {
    if (video.buffered.start(i) <= position && video.buffered.end(i) >= position + 1.5) {
      return true;
    }
  }
  return false;
}

function playbackBufferedRange(video, duration) {
  const totalDuration = Number(duration);
  if (!video?.buffered?.length || !Number.isFinite(totalDuration) || totalDuration <= 0) return null;

  const position = Number.isFinite(video.currentTime) ? Math.max(0, video.currentTime) : 0;
  let activeRange = null;
  let upcomingRange = null;

  for (let i = 0; i < video.buffered.length; i++) {
    const start = Math.max(0, video.buffered.start(i));
    const end = Math.min(totalDuration, video.buffered.end(i));
    if (end <= start) continue;

    if (start <= position && end + 0.001 >= position) {
      activeRange = { start, end };
      break;
    }
    if (start > position && (!upcomingRange || start < upcomingRange.start)) {
      upcomingRange = { start, end };
    }
  }

  return activeRange || upcomingRange;
}

function updatePlaybackBufferIndicator(video, indicator, duration) {
  if (!indicator) return;
  const range = playbackBufferedRange(video, duration);
  if (!range) {
    indicator.style.left = '0%';
    indicator.style.width = '0%';
    return;
  }

  const left = (range.start / duration) * 100;
  const width = ((range.end - range.start) / duration) * 100;
  indicator.style.left = `${Math.max(0, Math.min(100, left))}%`;
  indicator.style.width = `${Math.max(0, Math.min(100 - left, width))}%`;
}

function waitForFeedPlaybackBuffer(video, timeoutMs = 2500) {
  return new Promise(resolve => {
    if (feedVideoHasPlaybackBuffer(video)) {
      resolve(true);
      return;
    }

    let settled = false;
    const finish = ready => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      ['canplay', 'canplaythrough', 'loadeddata', 'progress'].forEach(event => {
        video?.removeEventListener(event, checkReady);
      });
      resolve(ready);
    };
    const checkReady = () => {
      if (feedVideoHasPlaybackBuffer(video)) finish(true);
    };
    const timer = setTimeout(() => finish(false), timeoutMs);

    ['canplay', 'canplaythrough', 'loadeddata', 'progress'].forEach(event => {
      video?.addEventListener(event, checkReady);
    });
  });
}

function beginFeedPlayback(feedVideo, token) {
  if (!feedVideo || !feedVideo.src) return;
  if (feedVideo.dataset.temporaryMute !== 'gesture') {
    feedVideo.muted = true;
    feedVideo.dataset.temporaryMute = 'first-frame';
  }
  feedPlayPromise = feedVideo.play();
  if (!feedPlayPromise) return;

  feedPlayPromise.catch(error => {
    if (token !== feedPlayToken || error?.name !== 'NotAllowedError') return;

    feedNeedsGestureUnmute = true;
    feedVideo.muted = true;
    feedVideo.dataset.temporaryMute = 'gesture';
    feedPlayPromise = feedVideo.play();
    feedPlayPromise?.catch(e => console.log('muted feed autoplay error:', e));
    showFeedHint('点击画面开启声音', true, 4000);
    attachFeedGestureUnlock();
  });
}

function scheduleFeedRecoveryAdvance(slot, itemIndex) {
  const loadToken = slot.loadToken;
  const attemptAdvance = () => {
    if (loadToken !== slot.loadToken || state.currentView !== 'recommend') return;
    if (isFeedAnimating) {
      setTimeout(attemptAdvance, 120);
      return;
    }
    if (feedSlots[feedSlotCurr] !== slot || slot.itemIndex !== itemIndex) return;
    if (feedVideoHasFirstFrame(slot.video)) return;
    if (state.feed.consecutiveSourceFailures > 4) return;
    const nextIndex = itemIndex + 1;
    const nextDrama = state.feed.items[nextIndex];
    const nextSlot = feedSlots.find(candidate => (
      candidate.itemIndex === nextIndex &&
      candidate.seriesId === nextDrama?.id
    ));
    // Skipping is an escape hatch for an unavailable item, never a way to
    // cascade through several unprepared videos during an upstream outage.
    if (!feedVideoHasFirstFrame(nextSlot?.video)) return;
    state.feed.consecutiveSourceFailures++;
    state.feed.lastSwitchTime = 0;
    switchFeedDrama(1);
  };
  setTimeout(attemptAdvance, 800);
}

function scheduleFeedAutoplayRetry(slot, attempt = 0) {
  if (!slot?.video || state.feed.userPaused || state.currentView !== 'recommend') return;
  if (slot.autoplayRetryTimer) clearTimeout(slot.autoplayRetryTimer);

  const loadToken = slot.loadToken;
  slot.autoplayRetryTimer = setTimeout(() => {
    if (!slot || loadToken !== slot.loadToken) return;
    slot.autoplayRetryTimer = null;
    if (
      feedSlots[feedSlotCurr] !== slot ||
      state.currentView !== 'recommend' ||
      state.feed.userPaused ||
      !slot.video?.src ||
      !slot.video.paused
    ) {
      return;
    }

    // A prior first-frame priming pause can race with the handoff, and WebKit
    // occasionally drops loadeddata after an HLS-to-MP4 source transition.
    // Retry the muted autoplay until the browser reports a real failure.
    beginFeedPlayback(slot.video, feedPlayToken);
    if (attempt < 8) scheduleFeedAutoplayRetry(slot, attempt + 1);
  }, attempt === 0 ? 120 : 300);
}

function ensureFeedSource(slot, itemIndex, attempt = 0) {
  if (!slot || state.currentView !== 'recommend') return;
  if (feedSlots[feedSlotCurr] !== slot) return;
  const loadToken = slot.loadToken;
  prepareFeedSlot(slot, itemIndex, true).then(() => {
    if (loadToken !== slot.loadToken || slot.video?.src) return;
    if (feedSlots[feedSlotCurr] !== slot) return;
    state.feed.consecutiveSourceFailures++;
    if (attempt >= 2) {
      reportFeedFailure(slot, '获取播放流失败', 1200);
      return;
    }
    if (slot.sourceRetryTimer) clearTimeout(slot.sourceRetryTimer);
    slot.sourceRetryTimer = setTimeout(() => {
      slot.sourceRetryTimer = null;
      clearFeedFailureHint(slot);
      ensureFeedSource(slot, itemIndex, attempt + 1);
    }, 300 + attempt * 250);
  }).catch(() => {
    if (loadToken === slot.loadToken && feedSlots[feedSlotCurr] === slot && attempt < 2) {
      state.feed.consecutiveSourceFailures++;
      if (slot.sourceRetryTimer) clearTimeout(slot.sourceRetryTimer);
      slot.sourceRetryTimer = setTimeout(() => {
        slot.sourceRetryTimer = null;
        clearFeedFailureHint(slot);
        ensureFeedSource(slot, itemIndex, attempt + 1);
      }, 300 + attempt * 250);
    }
  });
}

let isFeedAnimating = false;
let feedSwitchWaitTimer = null;
let feedSwitchWaitDir = 0;
let feedSwitchWaitStartedAt = 0;
let feedAnimationToken = 0;

function cancelPendingFeedSwitch() {
  if (!feedSwitchWaitTimer) return;
  clearTimeout(feedSwitchWaitTimer);
  feedSwitchWaitTimer = null;
  feedSwitchWaitDir = 0;
  feedSwitchWaitStartedAt = 0;
}

function schedulePendingFeedSwitch(dir) {
  if (feedSwitchWaitTimer && feedSwitchWaitDir === dir) return;
  cancelPendingFeedSwitch();
  feedSwitchWaitDir = dir;
  feedSwitchWaitStartedAt = Date.now();
  feedSwitchWaitTimer = setTimeout(() => {
    feedSwitchWaitTimer = null;
    feedSwitchWaitDir = 0;
    if (state.currentView === 'recommend' && !isFeedAnimating) switchFeedDrama(dir);
  }, 400);
}

function prepareFeedTargetForSwitch(slot, itemIndex, quality) {
  const drama = state.feed.items[itemIndex];
  if (!slot || !drama) return;
  if (feedSlotMatches(slot, itemIndex, drama.id, quality)) {
    if (!feedVideoHasFirstFrame(slot.video) && !slot.firstFramePriming) {
      slot.firstFramePriming = primeFeedSlotFirstFrame(slot);
    }
    return;
  }
  if (slot.preparing) return;
  prepareFeedSlot(slot, itemIndex, false, null, true).catch(err => {
    console.warn('prepare feed target failed:', drama.id, err);
  });
}

function switchFeedDrama(dir) {
  if (state.currentView !== 'recommend') return;
  const now = Date.now();
  if (isFeedAnimating && now - state.feed.lastSwitchTime > 1000) {
    // The normal completion callback and watchdog above are defensive, but a
    // synchronous setup exception can still escape both. Self-heal here so one
    // bad swipe can never permanently disable the feed controls.
    console.warn('feed switch self-healing released a stuck animation lock');
    isFeedAnimating = false;
  }
  if (isFeedAnimating) return;
  if (now - state.feed.lastSwitchTime < 380) return;

  const targetIdx = state.feed.index + dir;
  if (targetIdx < 0) {
    showFeedHint('已经是第一部了', false, 1800);
    return;
  }

  if (targetIdx >= state.feed.items.length) {
    if (state.feed.hasMore) {
      const switchOrigin = state.feed.index;
      const finishBoundaryLoad = () => {
        if (state.feed.index === switchOrigin && state.feed.items[targetIdx]) {
          state.feed.lastSwitchTime = Date.now();
          switchFeedDrama(dir);
        }
      };
      if (state.feed.loading) {
        setTimeout(() => {
          if (state.feed.index === switchOrigin && state.currentView === 'recommend') switchFeedDrama(dir);
        }, 300);
        return;
      }
      loadFeed(false).then(finishBoundaryLoad).catch(finishBoundaryLoad);
    } else {
      showFeedHint('已经到底了，没有更多了', false, 2200);
    }
    return;
  }

  const targetDrama = state.feed.items[targetIdx];
  const oldCurrentSlot = feedSlots[feedSlotCurr];
  const targetSlot = dir > 0 ? feedSlots[feedSlotNext] : feedSlots[feedSlotPrev];
  const offscreenSlot = dir > 0 ? feedSlots[feedSlotPrev] : feedSlots[feedSlotNext];
  const desiredQuality = feedDesiredQuality();
  const requestedTargetQuality = dir > 0 ? desiredQuality : (targetSlot.quality || desiredQuality);
  const targetReadyBeforeMove = feedSlotMatches(targetSlot, targetIdx, targetDrama.id, requestedTargetQuality);
  const targetWaitMs = feedSwitchWaitStartedAt ? now - feedSwitchWaitStartedAt : 0;
  const targetStartupDeadline = targetSlot.progressiveStreamUrl ? 1200 : 2500;
  if ((!targetReadyBeforeMove || !feedVideoHasFirstFrame(targetSlot.video)) && targetWaitMs < targetStartupDeadline) {
    // Rapid swipes can outrun page pagination and HLS priming. Keep the active
    // video playing while the target decodes a real frame, then complete the
    // gesture without exposing a paused poster or a loading overlay.
    prepareFeedTargetForSwitch(targetSlot, targetIdx, requestedTargetQuality);
    schedulePendingFeedSwitch(dir);
    return;
  }

  state.feed.lastSwitchTime = now;
  isFeedAnimating = true;
  const animationToken = ++feedAnimationToken;
  setTimeout(() => {
    if (feedAnimationToken !== animationToken || !isFeedAnimating) return;
    // If synchronous switch setup throws before the normal completion timer is
    // registered, this watchdog still releases the carousel for the next swipe.
    console.warn('feed animation completion watchdog released a stuck switch');
    isFeedAnimating = false;
  }, 420);
  const switchStartedAt = performance.now();
  const fromIndex = state.feed.index;

  persistFeedProgress();
  cancelPendingFeedSwitch();

  if (dir > 0) {
    feedSlotPrev = oldCurrentSlot.id;
    feedSlotNext = offscreenSlot.id;
  } else {
    feedSlotNext = oldCurrentSlot.id;
    feedSlotPrev = offscreenSlot.id;
  }

  const targetQuality = dir > 0 ? desiredQuality : (targetSlot.quality || desiredQuality);
  const targetReady = feedSlotMatches(targetSlot, targetIdx, targetDrama.id, targetQuality);
  const targetBuffered = Boolean(targetReady && feedVideoHasPlaybackBuffer(targetSlot.video));
  activateFeedDrama(targetDrama, targetIdx, targetReady ? targetSlot : null);
  if (targetReady) prepareFeedStartPosition(targetSlot, dir);
  loadFeedDramaMetadata(targetDrama);
  setActiveFeedSlot(targetSlot);
  if (targetReady && (feedVideoHasFirstFrame(targetSlot.video) || feedVideoHasPlaybackBuffer(targetSlot.video))) {
    state.feed.consecutiveSourceFailures = 0;
  }
  if (targetReady && feedVideoHasFirstFrame(targetSlot.video)) {
    hideFeedPosterImmediately(targetSlot);
  }

  // Keep the prepared cover as the handoff visual. A source may not expose
  // buffered ranges until it becomes visible, so showing a loader here creates
  // a false loading state during an otherwise continuous swipe.
  ensureFeedItemsAhead();
  if (targetReady) {
    if (
      !feedVideoHasFirstFrame(targetSlot.video) &&
      targetSlot.progressiveStreamUrl &&
      targetSlot.video.src !== targetSlot.progressiveStreamUrl
    ) {
      // A background HLS preload can have a source but no decoded frame yet.
      // Progressive fMP4 starts faster for an immediate handoff; keep HLS for
      // slots that have already primed a real frame.
      targetSlot.streamUrl = targetSlot.progressiveStreamUrl;
      targetSlot.video.src = targetSlot.progressiveStreamUrl;
      targetSlot.video.load();
      targetSlot.usedHLSFallback = true;
      targetSlot.usedProgressiveFallback = true;
      targetSlot.metricStreamMode = 'progressive';
    }
    if (!state.feed.userPaused) {
      targetSlot.video.muted = false;
      targetSlot.video.volume = 1;
      beginFeedPlayback(targetSlot.video, feedPlayToken);
      scheduleFeedAutoplayRetry(targetSlot);
    }
  } else {
    ensureFeedSource(targetSlot, targetIdx);
    setTimeout(() => {
      if (
        feedSlots[feedSlotCurr] === targetSlot &&
        targetSlot.itemIndex === targetIdx &&
        !targetSlot.video?.src
      ) {
        scheduleFeedRecoveryAdvance(targetSlot, targetIdx);
      }
    }, 1200);
  }

  updateFeedDramaInfo(targetDrama, state.feed.episodeNum);
  const feedInfo = document.getElementById('feed-info');
  if (feedInfo) {
    feedInfo.style.transition = 'opacity 0.2s ease, transform 0.2s ease';
    feedInfo.style.opacity = '0.2';
    feedInfo.style.transform = `translateY(${dir > 0 ? '-10px' : '10px'})`;
    setTimeout(() => {
      feedInfo.style.opacity = '1';
      feedInfo.style.transform = 'translateY(0)';
    }, 180);
  }

  const duration = 320;
  const easing = 'cubic-bezier(0.2, 0.9, 0.3, 1)';
  oldCurrentSlot.layer.style.transition = `transform ${duration}ms ${easing}`;
  oldCurrentSlot.layer.style.transform = dir > 0 ? 'translateY(-100%)' : 'translateY(100%)';
  oldCurrentSlot.layer.style.zIndex = '1';

  targetSlot.layer.style.transition = `transform ${duration}ms ${easing}`;
  targetSlot.layer.style.transform = 'translateY(0)';
  targetSlot.layer.style.zIndex = '2';
  targetSlot.layer.classList.add('is-active');
  oldCurrentSlot.layer.classList.remove('is-active');

  setTimeout(() => {
    try {
      if (feedSlots[feedSlotCurr] !== oldCurrentSlot) oldCurrentSlot.video.pause();
      oldCurrentSlot.layer.style.transition = 'none';
      targetSlot.layer.style.transition = 'none';

      const newNextSlot = feedSlots[feedSlotNext];
      newNextSlot.layer.style.transition = 'none';
      newNextSlot.layer.style.transform = 'translateY(100%)';
      newNextSlot.layer.style.zIndex = '1';
      newNextSlot.layer.classList.remove('is-active');

      const newPreviousSlot = feedSlots[feedSlotPrev];
      newPreviousSlot.layer.style.transition = 'none';
      newPreviousSlot.layer.style.transform = 'translateY(-100%)';
      newPreviousSlot.layer.style.zIndex = '1';
      newPreviousSlot.layer.classList.remove('is-active');

      ensureFeedItemsAhead();
      recordFeedSwitchMetric(fromIndex, targetIdx, targetBuffered, performance.now() - switchStartedAt);
    } catch (err) {
      // A metrics or DOM cleanup error must never leave the carousel locked.
      console.error('feed switch cleanup failed:', err);
    } finally {
      isFeedAnimating = false;
    }
  }, duration);
}

async function enterSeriesPlayer(seriesId, episodeNum) {
  const modal = document.getElementById('player-modal');
  if (!seriesId || (modal && modal.style.display !== 'none')) return;

  // If feed (or another element) is currently in fullscreen mode, exit it
  // so the player modal can take over the viewport without being trapped.
  if (document.fullscreenElement && document.fullscreenElement !== document.getElementById('player-stage')) {
    try {
      await document.exitFullscreen();
    } catch (err) {
      console.warn('exitFullscreen failed in enterSeriesPlayer:', err);
    }
  }

  clearCommentSceneLayout();
  inheritFeedPlaybackSpeed();

  feedPlayToken++;
  cancelScheduledFeedWarmup();
  cancelPendingFeedSwitch();
  pauseFeedPlayback();
  state.feed.userPaused = true;

  const drawer = document.getElementById('feed-playlist-drawer');
  drawer?.classList.remove('open');

  openPlayer(seriesId, episodeNum).catch(err => {
    console.error('enterSeriesPlayer error:', err);
    showToast('打开播放器失败: ' + err);
  });
}

function clearCommentSceneLayout() {
  if (commentState.open) closeCommentSheet();

  // The comment sheet reserves viewport width through a body-level class.
  // Clear it before mounting the series player so a stale comment scene can
  // never shift the player stage or leave a blank column on the right.
  const overlay = document.getElementById('comment-sheet-overlay');
  if (overlay) overlay.style.display = 'none';
  restoreCommentOverlay();
  document.body.classList.remove('comments-open');
}

function inheritFeedPlaybackSpeed() {
  const speed = Number(state.feed.playbackSpeed) || 1;
  state.player.playbackSpeed = speed;

  const speedButton = document.getElementById('player-speed');
  const speedMenu = document.getElementById('player-speed-menu');
  if (speedButton) speedButton.textContent = `${speed}x`;
  if (!speedMenu) return;
  speedMenu.querySelectorAll('.speed-opt').forEach(option => {
    option.classList.toggle('active', Number(option.dataset.speed) === speed);
  });
}

function renderFeedPlaylistDrawer() {
  renderEpisodeGrid({
    grid: document.getElementById('feed-drawer-grid'),
    count: document.getElementById('feed-drawer-count'),
    total: state.feed.totalEpisodes || (state.feed.episodes ? state.feed.episodes.length : 1),
    current: state.feed.episodeNum,
    onSelect: episodeNum => enterSeriesPlayer(state.feed.seriesId, episodeNum)
  });
}

function renderEpisodeGrid({ grid, count, total, current, onSelect }) {
  if (!grid) return;
  grid.innerHTML = '';
  if (count) count.textContent = `共 ${total} 集`;

  for (let episodeNum = 1; episodeNum <= total; episodeNum++) {
    const button = document.createElement('button');
    button.className = `ep-btn ${episodeNum === current ? 'active' : ''}`;
    button.dataset.num = episodeNum;
    button.textContent = episodeNum;
    button.onclick = () => onSelect(episodeNum);
    grid.appendChild(button);
  }
}

function scrollActiveDrawerEpisode(gridId, episodeNum) {
  const grid = document.getElementById(gridId);
  if (!grid) return;
  const btn = grid.querySelector(`.ep-btn[data-num="${episodeNum}"]`);
  if (!btn) return;
  // Scroll strictly within the grid container vertically.
  // Never invoke element.scrollIntoView() which causes ancestor horizontal scrolling in WebKit.
  const targetTop = btn.offsetTop - grid.offsetTop;
  grid.scrollTop = Math.max(0, targetTop - (grid.clientHeight - btn.clientHeight) / 2);
}

function renderFeedQualityMenu() {
  const qualityBtn = document.getElementById('feed-quality');
  const qualityMenu = document.getElementById('feed-quality-menu');
  if (!qualityBtn || !qualityMenu) return;

  const active = state.feed.qualityOptions.find(opt => opt.active) || { label: qualityLabel(state.feed.quality) };
  qualityBtn.textContent = active.label;
  qualityMenu.innerHTML = '';
  state.feed.qualityOptions.forEach(opt => {
    const item = document.createElement('div');
    item.className = `quality-opt ${opt.active ? 'active' : ''} ${opt.available ? '' : 'disabled'}`;
    item.textContent = opt.label;
    if (opt.available) {
      item.addEventListener('click', () => {
        qualityMenu.style.display = 'none';
        qualityBtn.setAttribute('aria-expanded', 'false');
        changeFeedPlaybackQuality(opt.id);
      });
    }
    qualityMenu.appendChild(item);
  });
}

function updateFeedQualityControls(playback) {
  if (playback.quality) state.feed.quality = playback.quality;
  state.feed.qualityOptions = Array.isArray(playback.qualities) && playback.qualities.length > 0
    ? playback.qualities
    : fallbackQualityOptions(state.feed.quality);
  renderFeedQualityMenu();
}

async function changeFeedPlaybackQuality(quality) {
  if (!quality || quality === state.feed.quality) return;
  const feedVideo = getFeedVideo();
  const resumeAt = feedVideo.currentTime || 0;
  state.feed.baseQuality = quality;
  state.feed.userQuality = quality;
  state.feed.quality = quality;
  cancelScheduledFeedWarmup();
  state.feed.preloadMap.clear();
  state.feed.playbackInfoLoads.clear();
  await playFeedIndex(state.feed.index || 0, resumeAt);
}

function danmakuContext(source) {
  if (source === 'player') {
    const modal = document.getElementById('player-modal');
    if (!modal || modal.style.display === 'none' ||
        !state.player.seriesId || !state.player.episodeId) return null;
    return { seriesId: state.player.seriesId, episodeId: state.player.episodeId };
  }
  if (!state.feed.seriesId || !state.feed.episodeId) return null;
  return { seriesId: state.feed.seriesId, episodeId: state.feed.episodeId };
}

function initDanmaku() {
  if (!window.DanmakuController) return;
  const common = {
    fetchPage: (seriesId, episodeId, startMs, durationMs) =>
      apiCall('GetDanmaku', seriesId, episodeId, startMs, durationMs),
    sendItem: async (seriesId, episodeId, text, timeMs) => {
      try {
        return await apiCall('SendDanmaku', seriesId, episodeId, text, timeMs, '');
      } catch (err) {
        showToast(String(err.message || err));
        throw err;
      }
    },
    likeItem: async (item, liked) => {
      if (String(item.id || '').startsWith('local-')) return undefined;
      try {
        return await apiCall('LikeDanmaku', danmakuContext('feed')?.episodeId || state.player.episodeId, item.id, liked);
      } catch (err) {
        showToast(String(err.message || err));
        return undefined;
      }
    },
    onError: err => console.warn('danmaku:', err),
  };

  danmakuControllers.feed = new DanmakuController({
    ...common,
    host: document.getElementById('feed-stage'),
    getVideo: getFeedVideo,
    getContext: () => danmakuContext('feed'),
  });
  danmakuControllers.player = new DanmakuController({
    ...common,
    host: document.getElementById('video-container'),
    getVideo: () => (document.getElementById('player-modal')?.style.display !== 'none' ? video : null),
    getContext: () => danmakuContext('player'),
  });
  danmakuControllers.player.likeItem = async (item, liked) => {
    if (String(item.id || '').startsWith('local-')) return undefined;
    try {
      return await apiCall('LikeDanmaku', state.player.episodeId, item.id, liked);
    } catch (err) {
      showToast(String(err.message || err));
      return undefined;
    }
  };

  bindDanmakuControls('feed', danmakuControllers.feed, 'feed');
  bindDanmakuControls('player', danmakuControllers.player, 'player');

  document.addEventListener('keydown', event => {
    if (event.key !== 'b' && event.key !== 'B') return;
    const target = event.target;
    if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) return;
    const source = document.getElementById('player-modal')?.style.display !== 'none' ? 'player' : 'feed';
    const controller = danmakuControllers[source];
    if (controller) setDanmakuEnabled(source, controller, !controller.enabled);
  });
}

function setDanmakuEnabled(source, controller, enabled) {
  controller.setEnabled(enabled);
  const button = document.getElementById(`${source}-danmaku`);
  if (button) button.classList.toggle('active', controller.enabled);
  if (!enabled) {
    const composer = document.getElementById(`${source}-danmaku-composer`);
    if (composer) composer.classList.remove('open');
    closeDanmakuSettings(source);
  }
}

function closeDanmakuSettings(source) {
  const panel = document.getElementById(`${source}-danmaku-settings-panel`);
  const toggle = document.getElementById(`${source}-danmaku-settings`);
  if (panel) panel.classList.remove('open');
  if (toggle) toggle.setAttribute('aria-expanded', 'false');
}

function syncDanmakuSettings(source, controller) {
  const panel = document.getElementById(`${source}-danmaku-settings-panel`);
  if (!panel) return;
  const values = {
    'font-size': [controller.fontSize, `${Math.round(controller.fontSize)}px`],
    'font-weight': [controller.fontWeight, String(controller.fontWeight)],
    outline: [controller.outline, controller.outline ? '开启' : '关闭'],
    rows: [controller.maxRows, controller.maxRows ? `${controller.maxRows} 行` : '自动'],
  };
  for (const [name, [value, text]] of Object.entries(values)) {
    const input = panel.querySelector(`[data-danmaku-setting="${name}"]`);
    const output = panel.querySelector(`[data-danmaku-value="${name}"]`);
    if (input) input.value = String(value);
    if (output) output.textContent = text;
  }
}

function bindDanmakuControls(prefix, controller, source) {
  const button = document.getElementById(`${prefix}-danmaku`);
  const settingsToggle = document.getElementById(`${prefix}-danmaku-settings`);
  const settingsPanel = document.getElementById(`${prefix}-danmaku-settings-panel`);
  const settingsClose = document.getElementById(`${prefix}-danmaku-settings-close`);
  const composer = document.getElementById(`${prefix}-danmaku-composer`);
  const input = document.getElementById(`${prefix}-danmaku-input`);
  const send = document.getElementById(`${prefix}-danmaku-send`);
  if (button) {
    button.classList.toggle('active', controller.enabled);
    button.addEventListener('click', event => {
      event.stopPropagation();
      setDanmakuEnabled(source, controller, !controller.enabled);
    });
  }
  if (settingsToggle && settingsPanel) {
    settingsToggle.addEventListener('click', event => {
      event.stopPropagation();
      const open = !settingsPanel.classList.contains('open');
      settingsPanel.classList.toggle('open', open);
      settingsToggle.setAttribute('aria-expanded', String(open));
      if (open) syncDanmakuSettings(source, controller);
    });
  }
  if (settingsClose) {
    settingsClose.addEventListener('click', event => {
      event.stopPropagation();
      closeDanmakuSettings(source);
    });
  }
  if (settingsPanel) {
    settingsPanel.addEventListener('click', event => event.stopPropagation());
    settingsPanel.addEventListener('keydown', event => event.stopPropagation());
    settingsPanel.addEventListener('input', event => {
      const input = event.target;
      const name = input?.dataset?.danmakuSetting;
      if (name === 'font-size') controller.setFontSize(Number(input.value));
      if (name === 'font-weight') controller.setFontWeight(Number(input.value));
      if (name === 'rows') controller.setRows(Number(input.value));
      if (name) syncDanmakuSettings(source, controller);
    });
    settingsPanel.addEventListener('change', event => {
      const input = event.target;
      if (input?.dataset?.danmakuSetting === 'outline') controller.setOutline(input.checked);
      syncDanmakuSettings(source, controller);
    });
  }
  if (composer) composer.addEventListener('click', event => event.stopPropagation());
  if (input) {
    input.addEventListener('keydown', event => {
      event.stopPropagation();
      if (event.key === 'Enter') void sendDanmakuInput(source, controller, input);
    });
  }
  if (send) {
    send.addEventListener('click', event => {
      event.stopPropagation();
      void sendDanmakuInput(source, controller, input);
    });
  }
}

async function sendDanmakuInput(source, controller, input) {
  if (!controller || !input) return;
  const text = input.value.trim();
  if (!text) return;
  const playbackVideo = source === 'player' ? document.getElementById('main-video') : getFeedVideo();
  input.disabled = true;
  try {
    await controller.send(text, (playbackVideo?.currentTime || 0) * 1000);
    input.value = '';
    input.blur();
    const composer = document.getElementById(`${source}-danmaku-composer`);
    if (composer) composer.classList.remove('open');
  } catch (err) {
    console.warn('send danmaku failed:', err);
    showToast('发送弹幕失败: ' + (err?.message || err || '网络异常'));
  } finally {
    input.disabled = false;
  }
}

function updateFeedCommentCount(count) {
  state.feed.commentCount = Number(count) || 0;
  const label = document.getElementById('feed-comment-count');
  if (label) label.textContent = formatCount(state.feed.commentCount, '评论');
}

function updatePlayerCommentCount(count) {
  state.player.commentCount = Number(count) || 0;
  const label = document.getElementById('player-comment-count');
  if (label) label.textContent = formatCount(state.player.commentCount, '评论');
}

function commentRelativeTime(timestamp) {
  const value = Number(timestamp) || 0;
  if (!value) return '';
  const milliseconds = value > 100000000000 ? value : value * 1000;
  const seconds = Math.max(0, (Date.now() - milliseconds) / 1000);
  if (seconds < 60) return '刚刚';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}分钟前`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}小时前`;
  if (seconds < 86400 * 30) return `${Math.floor(seconds / 86400)}天前`;
  if (seconds < 86400 * 365) return `${Math.floor(seconds / (86400 * 30))}个月前`;
  return `${Math.floor(seconds / (86400 * 365))}年前`;
}

function initCommentSheet() {
  const overlay = document.getElementById('comment-sheet-overlay');
  const closeBtn = document.getElementById('btn-close-comment-sheet');
  const list = document.getElementById('comment-list');
  if (!overlay || !closeBtn || !list) return;

  commentOverlayHome = overlay.parentElement;
  closeBtn.addEventListener('click', closeCommentSheet);
  overlay.addEventListener('click', (event) => {
    if (event.target === overlay) closeCommentSheet();
  });
  if ('IntersectionObserver' in window) {
    commentScrollObserver = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        if (entry.target.dataset.commentSentinel === 'reply') {
          loadMoreReplies(entry.target.dataset.commentId);
        } else {
        loadMoreComments();
        }
      }
    }, { root: list, rootMargin: '160px' });
  } else {
    list.addEventListener('scroll', () => {
      const nearBottom = list.scrollTop + list.clientHeight >= list.scrollHeight - 160;
      if (nearBottom) loadMoreComments();
    });
  }

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && commentState.open) {
      event.preventDefault();
      event.stopPropagation();
      closeCommentSheet();
    }
  }, true);
}

function restoreCommentOverlay() {
  const overlay = document.getElementById('comment-sheet-overlay');
  if (overlay && commentOverlayHome) {
    commentOverlayHome.appendChild(overlay);
  }
}

function mountCommentOverlay(source) {
  const overlay = document.getElementById('comment-sheet-overlay');
  if (!overlay) return;

  const stage = document.getElementById('player-stage');
  const modal = document.getElementById('player-modal');
  modal?.classList.toggle('comments-open', source === 'player');
  const isFullscreenPlayer = source === 'player' && stage
    && (document.fullscreenElement === stage || stage.classList.contains('player-fullscreen'));
  if (isFullscreenPlayer) {
    stage.appendChild(overlay);
  } else {
    restoreCommentOverlay();
  }
}

function getCommentVideo(source) {
  return source === 'player'
    ? document.getElementById('main-video')
    : getFeedVideo();
}

function animateCommentVideo(source, fromRect) {
  const video = getCommentVideo(source);
  if (!video || !fromRect || !video.animate) return;
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

  const toRect = video.getBoundingClientRect();
  if (!toRect.width || !toRect.height) return;

  const deltaX = fromRect.left - toRect.left;
  const scale = fromRect.width / toRect.width;
  const deltaY = fromRect.top + fromRect.height / 2
    - (toRect.top + (toRect.height * scale) / 2);
  if (Math.abs(deltaX) < 1 && Math.abs(deltaY) < 1 && Math.abs(scale - 1) < 0.001) return;

  video.animate(
    [
      { transform: `translate(${deltaX}px, ${deltaY}px) scale(${scale})`, transformOrigin: '0 0' },
      { transform: 'translate(0, 0) scale(1, 1)', transformOrigin: '0 0' }
    ],
    {
      duration: COMMENT_MOTION_DURATION_MS,
      easing: COMMENT_MOTION_EASING,
      fill: 'backwards'
    }
  );
}

function openCommentSheet(options) {
  if (!options || !options.seriesId || !options.episodeId) {
    showToast('评论信息缺失');
    return;
  }
  const token = ++commentState.requestToken;
  Object.assign(commentState, {
    open: true,
    loading: true,
    requestToken: token,
    seriesId: String(options.seriesId),
    episodeId: String(options.episodeId),
    episodeNum: Number(options.episodeNum) || 1,
    title: options.title || '评论',
    items: [],
    total: Number(options.count) || 0,
    cursor: '',
    nextCursor: '',
    hasMore: false,
    error: '',
    replies: {},
    replyCursors: {},
    replyHasMore: {},
    replyLoading: {},
    replyErrors: {},
    replyOpen: {}
  });

  const overlay = document.getElementById('comment-sheet-overlay');
  const shouldAnimateVideo = !commentState.open;
  const fromVideoRect = shouldAnimateVideo
    ? getCommentVideo(options.source)?.getBoundingClientRect()
    : null;
  commentState.source = options.source || 'feed';
  mountCommentOverlay(options.source);
  if (overlay) overlay.style.display = 'flex';
  document.body.classList.add('comments-open');
  if (shouldAnimateVideo) animateCommentVideo(commentState.source, fromVideoRect);
  renderCommentSheet();
  apiCall('GetEpisodeComments', commentState.seriesId, commentState.episodeId, '')
    .then(page => {
      if (token !== commentState.requestToken) return;
      commentState.loading = false;
      applyCommentPage(page);
      renderCommentSheet();
    })
    .catch(err => {
      if (token !== commentState.requestToken) return;
      commentState.loading = false;
      commentState.error = String(err || '评论加载失败');
      renderCommentSheet();
    });
}

function applyCommentPage(page) {
  if (!page) return;
  const existing = new Set(commentState.items.map(item => item.id));
  page.items = page.items || [];
  page.items.forEach(item => {
    if (!existing.has(item.id)) {
      commentState.items.push(item);
      existing.add(item.id);
    }
  });
  commentState.total = Number(page.total) || commentState.items.length;
  commentState.cursor = page.cursor || commentState.cursor;
  commentState.nextCursor = page.nextCursor || '';
  commentState.hasMore = Boolean(page.hasMore);
  commentState.error = '';
  if (commentState.episodeNum === state.feed.episodeNum && commentState.seriesId === state.feed.seriesId) {
    updateFeedCommentCount(commentState.total);
  }
  if (commentState.episodeNum === state.player.episodeNum && commentState.seriesId === state.player.seriesId) {
    updatePlayerCommentCount(commentState.total);
  }
}

async function loadMoreComments() {
  if (!commentState.open || commentState.loading || !commentState.hasMore) return;
  const token = commentState.requestToken;
  commentState.loading = true;
  renderCommentSheet();
  try {
    const page = await apiCall('GetEpisodeComments', commentState.seriesId, commentState.episodeId, commentState.nextCursor);
    if (token !== commentState.requestToken) return;
    commentState.loading = false;
    applyCommentPage(page);
    renderCommentSheet();
  } catch (err) {
    if (token !== commentState.requestToken) return;
    commentState.loading = false;
    commentState.error = String(err || '评论加载失败');
    renderCommentSheet();
  }
}

function toggleCommentReplies(item) {
  const id = item.id;
  if (commentState.replyOpen[id]) {
    commentState.replyOpen[id] = false;
    renderCommentSheet();
    return;
  }
  commentState.replyOpen[id] = true;
  if (!commentState.replies[id] && !commentState.replyLoading[id]) {
    loadMoreReplies(id);
    return;
  }
  renderCommentSheet();
}

async function loadMoreReplies(commentId) {
  if (!commentState.open || !commentId || commentState.replyLoading[commentId]) return;
  if (!commentState.replyOpen[commentId] || commentState.replies[commentId] && !commentState.replyHasMore[commentId]) return;

  const token = commentState.requestToken;
  commentState.replyLoading[commentId] = true;
  commentState.replyErrors[commentId] = '';
  renderCommentSheet();
  try {
    const page = await apiCall(
      'GetEpisodeCommentReplies',
      commentState.seriesId,
      commentState.episodeId,
      commentId,
      commentState.replyCursors[commentId] || ''
    );
    if (token !== commentState.requestToken) return;

    const existing = new Set((commentState.replies[commentId] || []).map(reply => reply.id));
    commentState.replies[commentId] = commentState.replies[commentId] || [];
    page.items = page.items || [];
    page.items.forEach(reply => {
      if (!existing.has(reply.id)) {
        commentState.replies[commentId].push(reply);
        existing.add(reply.id);
      }
    });
    commentState.replyCursors[commentId] = page.nextCursor || commentState.replyCursors[commentId] || '';
    commentState.replyHasMore[commentId] = Boolean(page.hasMore);
    commentState.replyLoading[commentId] = false;
    commentState.replyErrors[commentId] = '';
    renderCommentSheet();
  } catch (err) {
    if (token !== commentState.requestToken) return;
    commentState.replyLoading[commentId] = false;
    commentState.replyErrors[commentId] = String(err || '回复加载失败');
    renderCommentSheet();
  }
}

function appendReplyElements(parent, replies) {
  replies.forEach(reply => {
    const replyRow = document.createElement('div');
    replyRow.className = 'comment-reply';

    const avatar = document.createElement('img');
    avatar.className = 'comment-reply-avatar';
    avatar.alt = reply.author || '回复用户';
    avatar.loading = 'lazy';
    avatar.onerror = () => {
      avatar.style.visibility = 'hidden';
    };
    if (reply.avatarUrl) avatar.src = reply.avatarUrl;

    const body = document.createElement('div');
    const meta = document.createElement('div');
    meta.className = 'comment-meta';
    const author = document.createElement('span');
    author.className = 'comment-author';
    author.textContent = reply.author || '红果用户';
    if (reply.replyTo) {
      const target = document.createElement('span');
      target.className = 'comment-reply-target';
      target.textContent = `回复 ${reply.replyTo}`;
      meta.append(author, target);
    } else {
      meta.append(author);
    }
    const time = document.createElement('span');
    time.className = 'comment-time';
    time.textContent = commentRelativeTime(reply.createdAt);
    meta.append(time);

    const content = document.createElement('div');
    content.className = 'comment-content';
    content.textContent = reply.content;

    const stats = document.createElement('div');
    stats.className = 'comment-stats';
    if (Number(reply.likes) > 0) {
      const likes = document.createElement('span');
      likes.textContent = `${formatCount(reply.likes, '0')} 赞`;
      stats.appendChild(likes);
    }

    body.append(meta, content);
    if (stats.children.length > 0) body.appendChild(stats);
    replyRow.append(avatar, body);
    parent.appendChild(replyRow);
  });
}

function appendReplyArea(body, row, item) {
  if (Number(item.replyCount) <= 0) return;

  const isOpen = Boolean(commentState.replyOpen[item.id]);
  const count = formatCount(item.replyCount, '0');
  const toggle = document.createElement('button');
  toggle.type = 'button';
  toggle.className = 'comment-replies-toggle';
  toggle.setAttribute('aria-expanded', String(isOpen));
  toggle.innerHTML = `
    <svg viewBox="0 0 24 24"><path fill="currentColor" d="M8.1 5.7L9.5 4.3l7.7 7.7-7.7 7.7-1.4-1.4 6.3-6.3z"/></svg>
    <span>${count} 条回复</span>
  `;
  toggle.addEventListener('click', (event) => {
    event.stopPropagation();
    toggleCommentReplies(item);
  });

  const replies = document.createElement('div');
  replies.className = 'comment-replies';
  replies.hidden = !isOpen;
  if (isOpen) {
    const loaded = commentState.replies[item.id] || [];
    if (loaded.length > 0) appendReplyElements(replies, loaded);

    if (commentState.replyErrors[item.id]) {
      const error = document.createElement('div');
      error.className = 'comment-reply-state comment-reply-error';
      error.textContent = commentState.replyErrors[item.id];
      replies.appendChild(error);
    } else if (commentState.replyLoading[item.id]) {
      const loading = document.createElement('div');
      loading.className = 'comment-reply-state';
      loading.textContent = '正在加载回复...';
      replies.appendChild(loading);
    } else if (loaded.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'comment-reply-state';
      empty.textContent = '暂无回复';
      replies.appendChild(empty);
    }

    if (commentState.replyHasMore[item.id] && !commentState.replyLoading[item.id]) {
      const sentinel = document.createElement('div');
      sentinel.className = 'comment-load-sentinel';
      sentinel.dataset.commentSentinel = 'reply';
      sentinel.dataset.commentId = item.id;
      replies.appendChild(sentinel);
      if (commentScrollObserver) commentScrollObserver.observe(sentinel);
    }
  }

  body.appendChild(toggle);
  row.appendChild(replies);
}

function renderCommentSheet() {
  const title = document.getElementById('comment-sheet-title');
  const subtitle = document.getElementById('comment-sheet-subtitle');
  const list = document.getElementById('comment-list');
  if (!title || !subtitle || !list) return;
  if (commentScrollObserver) commentScrollObserver.disconnect();

  const countText = commentState.total > 0 ? `共 ${formatCount(commentState.total, '0')} 条` : '';
  title.textContent = countText || '评论';
  subtitle.textContent = commentState.loading && commentState.items.length === 0
    ? '正在加载官方评论'
    : `${commentState.title || '短剧'} 第 ${commentState.episodeNum} 集`;

  list.innerHTML = '';
  if (commentState.items.length === 0) {
    const empty = document.createElement('div');
    empty.className = commentState.error ? 'comment-error' : 'comment-empty';
    empty.textContent = commentState.loading
      ? '正在加载评论...'
      : (commentState.error || '本集暂无评论');
    list.appendChild(empty);
    if (!commentState.loading && commentState.hasMore) {
      const sentinel = document.createElement('div');
      sentinel.className = 'comment-load-sentinel';
      list.appendChild(sentinel);
      if (commentScrollObserver) commentScrollObserver.observe(sentinel);
    }
    return;
  }

  commentState.items.forEach(item => {
    const row = document.createElement('div');
    row.className = 'comment-item';

    const avatar = document.createElement('img');
    avatar.className = 'comment-avatar';
    avatar.alt = item.author || '评论用户';
    avatar.loading = 'lazy';
    avatar.onerror = () => {
      avatar.style.visibility = 'hidden';
    };
    if (item.avatarUrl) avatar.src = item.avatarUrl;

    const body = document.createElement('div');
    const meta = document.createElement('div');
    meta.className = 'comment-meta';
    const author = document.createElement('span');
    author.className = 'comment-author';
    author.textContent = item.author || '红果用户';
    const time = document.createElement('span');
    time.className = 'comment-time';
    time.textContent = commentRelativeTime(item.createdAt);
    meta.append(author, time);

    const content = document.createElement('div');
    content.className = 'comment-content';
    content.textContent = item.content;

    const stats = document.createElement('div');
    stats.className = 'comment-stats';
    if (Number(item.likes) > 0) {
      const likes = document.createElement('span');
      likes.textContent = `${formatCount(item.likes, '0')} 赞`;
      stats.appendChild(likes);
    }

    body.append(meta, content);
    if (stats.children.length > 0) body.appendChild(stats);
    row.append(avatar, body);
    appendReplyArea(body, row, item);
    list.appendChild(row);
  });

  if (commentState.loading && commentState.items.length > 0) {
    const loading = document.createElement('div');
    loading.className = 'comment-load-state';
    loading.textContent = '正在加载更多...';
    list.appendChild(loading);
  } else if (commentState.hasMore) {
    const sentinel = document.createElement('div');
    sentinel.className = 'comment-load-sentinel';
    list.appendChild(sentinel);
    if (commentScrollObserver) commentScrollObserver.observe(sentinel);
  }
}

function closeCommentSheet() {
  const source = commentState.source;
  const playerModal = document.getElementById('player-modal');
  const fromVideoRect = getCommentVideo(source)?.getBoundingClientRect();
  const closeToken = commentState.requestToken + 1;
  commentState.open = false;
  commentState.requestToken++;
  const overlay = document.getElementById('comment-sheet-overlay');
  restoreCommentOverlay();
  document.body.classList.remove('comments-open');
  playerModal?.classList.remove('comments-open');
  animateCommentVideo(source, fromVideoRect);

  if (overlay) {
    if (!overlay.animate || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      overlay.style.display = 'none';
      return;
    }

    overlay.style.pointerEvents = 'none';
    overlay.animate(
      [{ opacity: 1 }, { opacity: 0 }],
      { duration: COMMENT_MOTION_DURATION_MS, easing: COMMENT_MOTION_EASING }
    );
    document.getElementById('comment-sheet')?.animate(
      [{ transform: 'translateX(0)', opacity: 1 }, { transform: 'translateX(28px)', opacity: 0 }],
      { duration: COMMENT_MOTION_DURATION_MS, easing: COMMENT_MOTION_EASING }
    );
    window.setTimeout(() => {
      overlay.style.pointerEvents = '';
      if (commentState.requestToken !== closeToken) return;
      overlay.style.display = 'none';
    }, COMMENT_MOTION_DURATION_MS);
  }
}

function formatCount(value, emptyText) {
  const n = Number(value) || 0;
  if (!n) return emptyText;
  if (n >= 100000000) return `${(n / 100000000).toFixed(1).replace(/\.0$/, '')}亿`;
  if (n >= 10000) return `${(n / 10000).toFixed(1).replace(/\.0$/, '')}万`;
  return String(n);
}

function persistFeedProgress() {
  const feedVideo = getFeedVideo();
  const duration = feedMediaDuration(feedVideo);
  if (!feedVideo || !state.feed.seriesId || !duration) return;
  apiCall('SaveProgress',
    state.feed.seriesId,
    state.feed.episodeId,
    state.feed.seriesTitle,
    state.feed.seriesCover,
    state.feed.episodeNum,
    state.feed.totalEpisodes,
    feedVideo.currentTime,
    duration
  ).catch(err => console.warn('save feed progress failed:', err));
}

function updateFeedPlayIndicator() {
  const centerPlay = document.getElementById('feed-play-indicator');
  const feedStage = document.getElementById('feed-stage');
  const loader = document.getElementById('feed-loader');
  const feedVideo = typeof getFeedVideo === 'function' ? getFeedVideo() : null;
  if (!centerPlay) return;

  const isFeedBuffering = (loader && loader.style.display !== 'none') ||
    (feedVideo && feedVideo.classList.contains('is-loading'));


  if (feedStage) {
    feedStage.classList.toggle('is-buffering', Boolean(isFeedBuffering));
  }

  // Keep the center control hidden while the active video is still buffering.
  if (
    state.feed.userPaused &&
    !isFeedBuffering &&
    feedVideo &&
    feedVideo.src &&
    feedVideo.readyState >= 2 &&
    feedVideo.paused
  ) {
    centerPlay.style.display = 'flex';
  } else {
    centerPlay.style.display = 'none';
  }
}

function initFeedControls() {
  const feedContainer = document.getElementById('feed-container');
  const feedStage = document.getElementById('feed-stage');
  const playBtn = document.getElementById('feed-play');
  const centerPlay = document.getElementById('feed-play-indicator');
  const prevEpBtn = document.getElementById('feed-prev-ep');
  const nextEpBtn = document.getElementById('feed-next-ep');
  const progressWrap = document.getElementById('feed-progress-wrap');
  const progressCurrent = document.getElementById('feed-progress-current');
  const progressBuffer = document.getElementById('feed-progress-buffer');
  const timeTooltip = document.getElementById('feed-time-tooltip');
  const speedBtn = document.getElementById('feed-speed');
  const speedMenu = document.getElementById('feed-speed-menu');
  const qualityBtn = document.getElementById('feed-quality');
  const qualityMenu = document.getElementById('feed-quality-menu');
  const drawerToggle = document.getElementById('feed-drawer-toggle');
  const drawer = document.getElementById('feed-playlist-drawer');
  const closeDrawerBtn = document.getElementById('feed-close-drawer');
  const fullscreenBtn = document.getElementById('feed-fullscreen');
  const pipBtn = document.getElementById('feed-pip');
  const likeBtn = document.getElementById('feed-like');
  const commentBtn = document.getElementById('feed-comment');
  const favBtn = document.getElementById('feed-fav');
  const catBar = document.getElementById('feed-cat-bar');
  const syncFeedBufferIndicator = () => {
    const feedVideo = getFeedVideo();
    updatePlaybackBufferIndicator(feedVideo, progressBuffer, feedMediaDuration(feedVideo));
  };

  if (pipBtn) {
    pipBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      togglePictureInPicture();
    });
  }

  const allVideos = (feedSlots && feedSlots.length > 0)
    ? feedSlots.map(s => s.video).filter(Boolean)
    : [document.getElementById('feed-video-0'), document.getElementById('feed-video-1'), document.getElementById('feed-video-2')].filter(Boolean);
  if (allVideos.length === 0) return;

  function toggleFeedPlay() {
    const feedVideo = getFeedVideo();
    if (!feedVideo || !feedVideo.src) {
      state.feed.userPaused = false;
      playFeedIndex(state.feed.index || 0).catch(err => console.error('resume playFeedIndex error:', err));
      return;
    }
    if (feedVideo.paused) {
      feedPlayPromise = feedVideo.play();
      if (feedPlayPromise) {
        feedPlayPromise.catch(e => console.log('play error:', e));
      }
      state.feed.userPaused = false;
    } else {
      feedVideo.pause();
      state.feed.userPaused = true;
      cancelScheduledFeedWarmup();
      cancelPendingFeedSwitch();
    }
    updateFeedPlayIndicator();
  }

  if (playBtn) playBtn.addEventListener('click', toggleFeedPlay);
  if (centerPlay) centerPlay.addEventListener('click', toggleFeedPlay);
  if (feedStage) {
    feedStage.addEventListener('click', (e) => {
      if (feedContainer && feedContainer.classList.contains('clearscreen')) {
        feedContainer.classList.remove('clearscreen');
        state.feed.isClearScreen = false;
        return;
      }
      if (e.target.closest?.('.feed-video') || e.target === feedStage) {
        toggleFeedPlay();
      }
    });
  }

  allVideos.forEach((video) => {
    video.addEventListener('play', () => {
      if (video !== getFeedVideo()) return;
      if (window.ambientGlow && state.currentView === 'recommend') {
        window.ambientGlow.attach(video).start();
      }
      scheduleFeedMediaTimeSync();
      updateFeedPlayButtonState(video);
      state.feed.userPaused = false;
      updateFeedPlayIndicator();
      setPlaybackKeepAwake(true);
      scheduleFeedWarmup(state.feed.index);
      const curItem = state.feed.items ? state.feed.items[state.feed.index] : null;
      if (curItem) {
        updateNowPlayingState({
          title: curItem.title || '红果短剧',
          series: curItem.title || '红果短剧',
          episode: state.feed.episodeNum || 1,
          duration: feedMediaDuration(video),
          position: video.currentTime || 0,
          playing: true,
          rate: 1.0,
          coverUrl: curItem.coverUrl || ''
        });
      }
    });

    video.addEventListener('pause', () => {
      if (video !== getFeedVideo()) return;
      if (window.ambientGlow) {
        window.ambientGlow.pause();
      }
      updateFeedPlayButtonState(video);
      updateFeedPlayIndicator();
      setPlaybackKeepAwake(false);
      const curItem = state.feed.items ? state.feed.items[state.feed.index] : null;
      if (curItem) {
        updateNowPlayingState({
          title: curItem.title || '红果短剧',
          series: curItem.title || '红果短剧',
          episode: state.feed.episodeNum || 1,
          duration: feedMediaDuration(video),
          position: video.currentTime || 0,
          playing: false,
          rate: 1.0,
          coverUrl: curItem.coverUrl || ''
        });
      }
      persistFeedProgress();
    });

    video.addEventListener('waiting', () => {
      if (video !== getFeedVideo()) return;
      if (window.ambientGlow) {
        window.ambientGlow.pause();
      }
      updateFeedPlayIndicator();
    });

    video.addEventListener('playing', () => {
      if (video !== getFeedVideo()) return;
      if (window.ambientGlow && state.currentView === 'recommend') {
        window.ambientGlow.attach(video).start();
      }
      restoreFeedFirstFrameSound(video);
      const loader = document.getElementById('feed-loader');
      if (loader) loader.style.display = 'none';
      const stage = document.getElementById('feed-stage');
      if (stage) stage.classList.remove('is-buffering');
      updateFeedPlayIndicator();
    });

    video.addEventListener('canplay', () => {
      if (video !== feedActiveVideo) return;
      updateFeedPlayIndicator();
    });

    video.addEventListener('loadeddata', () => {
      if (video !== feedActiveVideo) return;
      updateFeedPlayIndicator();
    });

    video.addEventListener('timeupdate', () => {
      if (video !== getFeedVideo()) return;
      rememberFeedPlaybackPosition(
        feedPlaybackKey(state.feed.seriesId, state.feed.episodeNum, state.feed.quality),
        video.currentTime
      );
      updateFeedMediaTime(video);
    });

    ['durationchange', 'loadeddata', 'canplay', 'canplaythrough', 'progress', 'seeking', 'seeked', 'timeupdate'].forEach(event => {
      video.addEventListener(event, () => {
        if (video !== getFeedVideo()) return;
        syncFeedBufferIndicator();
      });
    });

    ['emptied', 'loadstart'].forEach(event => {
      video.addEventListener(event, () => {
        if (video !== getFeedVideo()) return;
        updatePlaybackBufferIndicator(video, progressBuffer, 0);
      });
    });

    video.addEventListener('ended', () => {
      if (video !== getFeedVideo()) return;
      if (window.ambientGlow) {
        window.ambientGlow.stop();
      }
      if (state.currentView !== 'recommend') return;
      const playerModal = document.getElementById('player-modal');
      if (playerModal && playerModal.style.display !== 'none') return;
      persistFeedProgress();
      state.feed.playbackPositions.delete(
        feedPlaybackKey(state.feed.seriesId, state.feed.episodeNum, state.feed.quality)
      );
      const total = state.feed.totalEpisodes || (state.feed.episodes ? state.feed.episodes.length : 1);
      if (state.feed.episodeNum < total) {
        showFeedHint(`正在进入第 ${state.feed.episodeNum + 1} 集...`, true, 2200);
        const nextNum = state.feed.episodeNum + 1;
        enterSeriesPlayer(state.feed.seriesId, nextNum);
      } else {
        const seriesId = state.feed.seriesId;
        playNextSeason(seriesId, 'feed').then(started => {
          if (started || state.feed.seriesId !== seriesId) return;
          showFeedHint('本剧已播放完毕，即将进入下一部', false, 2500);
          if (feedNextDramaTimer) clearTimeout(feedNextDramaTimer);
          feedNextDramaTimer = setTimeout(() => {
            feedNextDramaTimer = null;
            if (state.currentView === 'recommend' && state.feed.seriesId === seriesId) {
              switchFeedDrama(1);
            }
          }, 1200);
        });
      }
    });
  });

  if (progressWrap) {
    progressWrap.addEventListener('click', (e) => {
      const rect = progressWrap.getBoundingClientRect();
      const pos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
      const feedVideo = getFeedVideo();
      const duration = feedMediaDuration(feedVideo);
      if (duration) {
        feedVideo.currentTime = pos * duration;
      }
    });

    progressWrap.addEventListener('mousemove', (e) => {
      const feedVideo = getFeedVideo();
      const duration = feedMediaDuration(feedVideo);
      if (!duration || !timeTooltip) return;
      const rect = progressWrap.getBoundingClientRect();
      const pos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
      const hoverSec = pos * duration;
      timeTooltip.textContent = formatTime(hoverSec);
      timeTooltip.style.left = `${pos * 100}%`;
      timeTooltip.style.display = 'block';
    });

    progressWrap.addEventListener('mouseleave', () => {
      if (timeTooltip) timeTooltip.style.display = 'none';
    });
  }

  if (prevEpBtn) {
    prevEpBtn.addEventListener('click', () => {
      if (state.feed.episodeNum > 1) {
        const prevNum = state.feed.episodeNum - 1;
        enterSeriesPlayer(state.feed.seriesId, prevNum);
      } else {
        showFeedHint('已经是第一集了', false, 1800);
      }
    });
  }

  if (nextEpBtn) {
    nextEpBtn.addEventListener('click', () => {
      const total = state.feed.totalEpisodes || (state.feed.episodes ? state.feed.episodes.length : 1);
      if (state.feed.episodeNum < total) {
        const nextNum = state.feed.episodeNum + 1;
        enterSeriesPlayer(state.feed.seriesId, nextNum);
      } else {
        showFeedHint('已经是最后一集了', false, 1800);
      }
    });
  }
  if (speedBtn && speedMenu) {
    speedBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      speedMenu.style.display = speedMenu.style.display === 'none' ? 'block' : 'none';
    });

    speedMenu.querySelectorAll('.speed-opt').forEach(opt => {
      opt.addEventListener('click', (e) => {
        e.stopPropagation();
        const spd = parseFloat(opt.dataset.speed);
        state.feed.playbackSpeed = spd;
        getFeedVideo().playbackRate = spd;
        speedBtn.textContent = spd + 'x';
        speedMenu.querySelectorAll('.speed-opt').forEach(o => o.classList.remove('active'));
        opt.classList.add('active');
        speedMenu.style.display = 'none';
      });
    });

    document.addEventListener('click', () => {
      if (speedMenu) speedMenu.style.display = 'none';
    });
  }

  function toggleFeedDrawer() {
    if (!drawer) return;
    const willOpen = !drawer.classList.contains('open');
    drawer.classList.toggle('open', willOpen);
    syncFeedScrollCapture();
    if (willOpen) {
      scrollActiveDrawerEpisode('feed-drawer-grid', state.feed.episodeNum);
    }
  }

  // Prevent any container accidental horizontal scrolling
  const viewPort = document.querySelector('.view-viewport');
  if (viewPort) {
    viewPort.scrollLeft = 0;
    viewPort.addEventListener('scroll', () => {
      if (viewPort.scrollLeft !== 0) viewPort.scrollLeft = 0;
    });
  }
  if (feedContainer) {
    feedContainer.scrollLeft = 0;
  }
  if (drawerToggle) drawerToggle.addEventListener('click', toggleFeedDrawer);
  if (closeDrawerBtn) closeDrawerBtn.addEventListener('click', () => {
    if (drawer) drawer.classList.remove('open');
    syncFeedScrollCapture();
  });

  if (fullscreenBtn) {
    fullscreenBtn.addEventListener('click', () => {
      if (!document.fullscreenElement) {
        if (feedContainer) feedContainer.requestFullscreen().catch(e => console.log(e));
      } else {
        document.exitFullscreen().catch(e => console.log(e));
      }
    });
  }

  if (commentBtn) {
    commentBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      openCommentSheet({
        source: 'feed',
        seriesId: state.feed.seriesId,
        episodeId: state.feed.episodeId,
        episodeNum: state.feed.episodeNum,
        title: state.feed.seriesTitle,
        count: state.feed.commentCount
      });
    });
  }

  if (likeBtn) {
    likeBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      if (!state.feed.seriesId) return;
      try {
        const isNowLiked = await apiCall('ToggleLike',
          state.feed.seriesId,
          state.feed.episodeId,
          state.feed.seriesTitle,
          state.feed.seriesCover,
          state.feed.episodeNum
        );
        state.feed.isLiked = isNowLiked;
        likeBtn.classList.toggle('active', isNowLiked);
        showToast(isNowLiked ? '已点赞' : '已取消点赞');
        state.favoritesView.likesList = [];
      } catch (err) {
        showToast('点赞失败: ' + err);
      }
    });
  }

  if (favBtn) {
    favBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      if (!state.feed.seriesId) return;
      try {
        const isNowFav = await apiCall('ToggleFavorite',
          state.feed.seriesId,
          state.feed.seriesTitle,
          state.feed.seriesCover,
          '',
          state.feed.totalEpisodes
        );
        state.feed.isFavorite = isNowFav;
        favBtn.classList.toggle('active', isNowFav);
        showToast(isNowFav ? '已加入收藏' : '已取消收藏');
        state.favoritesView.favoritesList = [];
      } catch (err) {
        showToast('收藏失败: ' + err);
      }
    });
  }

  if (catBar) {
    catBar.querySelectorAll('.cat-chip').forEach(chip => {
      chip.addEventListener('click', () => {
        catBar.querySelectorAll('.cat-chip').forEach(c => c.classList.remove('active'));
        chip.classList.add('active');
        const channel = chip.dataset.channel || '';
        loadFeed(true, channel);
      });
    });
  }

  const recommendView = document.getElementById('view-recommend');
  waitForMygoBridge().then(runtime => {
    if (!recommendView || !runtime?.on) return;
    runtime.on('scrollGesture', (payload) => {
      nativeScrollGestureSeen = true;
      const direction = String(payload?.direction || '').toLowerCase();
      if (direction === 'down') switchFeedDrama(1);
      if (direction === 'up') switchFeedDrama(-1);
    });
  }).catch(() => {});

  let wheelArmed = true;
  let wheelSum = 0;
  let wheelLastAt = 0;
  let wheelLastDirection = 0;
  let wheelLowCount = 0;
  let wheelLowEnergy = 0;

  if (recommendView) {
    // NSEvent is preferred. This velocity gate is only a WebKit fallback
    // because JS wheel events do not expose the native gesture phase.
    recommendView.addEventListener('wheel', (e) => {
      if (e.target?.closest?.('.feed-playlist-drawer, .episodes-grid, .speed-menu, .drawer')) return;
      e.preventDefault();
      if (nativeScrollGestureSeen) return;

      const deltaUnit = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? 100 : 1;
      const deltaY = e.deltaY * deltaUnit;
      if (Math.abs(deltaY) < 12) return;
      const direction = deltaY > 0 ? 1 : -1;
      const now = performance.now();
      const gap = now - wheelLastAt;
      const magnitude = Math.abs(deltaY);

      // A gesture is not re-armed by elapsed time alone. It is re-armed by a
      // physical direction change, an event-stream gap, or a short low-energy
      // tail. Momentum therefore belongs to the gesture that already fired.
      if (direction !== wheelLastDirection) {
        wheelArmed = true;
        wheelSum = deltaY;
        wheelLowCount = 0;
        wheelLowEnergy = 0;
      } else if (gap > 180) {
        wheelArmed = true;
        wheelSum = deltaY;
        wheelLowCount = 0;
        wheelLowEnergy = 0;
      } else if (!wheelArmed) {
        if (magnitude <= 1.5) {
          wheelLowCount += 1;
          wheelLowEnergy += magnitude;
          if (wheelLowCount >= 2 && wheelLowEnergy <= 3.5) {
            wheelArmed = true;
            wheelSum = 0;
          }
        } else {
          wheelLowCount = 0;
          wheelLowEnergy = 0;
        }
        wheelLastAt = now;
        wheelLastDirection = direction;
        return;
      } else {
        wheelSum += deltaY;
      }

      wheelLastAt = now;
      wheelLastDirection = direction;
      if (Math.abs(wheelSum) < 65) return;

      const dir = wheelSum > 0 ? 1 : -1;
      wheelSum = 0;
      wheelArmed = false;
      wheelLowCount = 0;
      wheelLowEnergy = 0;
      switchFeedDrama(dir);
    }, { passive: false });

  }

  let pointerStart = null;
  let pointerStartTime = 0;
  if (feedStage) {
    feedStage.addEventListener('pointerdown', (e) => {
      if (e.target.closest('button, .progress-bar-wrap, .cat-bar')) return;
      pointerStart = { x: e.clientX, y: e.clientY };
      pointerStartTime = Date.now();
    });

    feedStage.addEventListener('pointerup', (e) => {
      if (!pointerStart) return;
      const diffX = e.clientX - pointerStart.x;
      const diffY = e.clientY - pointerStart.y;
      const duration = Date.now() - pointerStartTime;
      pointerStart = null;

      const minVerticalDistance = Math.max(60, feedStage.clientHeight * 0.08);
      const isVertical = Math.abs(diffY) > Math.abs(diffX) * 1.5;
      if (Math.abs(diffY) >= minVerticalDistance && isVertical && duration < 800) {
        switchFeedDrama(diffY < 0 ? 1 : -1);
      }
    });
    feedStage.addEventListener('pointercancel', () => {
      pointerStart = null;
    });
  }

  document.addEventListener('keydown', (e) => {
    if (state.currentView !== 'recommend') return;
    const modal = document.getElementById('player-modal');
    if (modal && modal.style.display !== 'none') return;
    const targetTag = e.target.tagName;
    if (targetTag === 'INPUT' || targetTag === 'TEXTAREA' || e.target.isContentEditable) return;

    switch (e.code) {
      case 'ArrowDown':
        e.preventDefault();
        switchFeedDrama(1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        switchFeedDrama(-1);
        break;
      case 'Space':
        e.preventDefault();
        toggleFeedPlay();
        break;
      case 'ArrowLeft':
        e.preventDefault();
        const feedVideoLeft = getFeedVideo();
        if (feedVideoLeft.duration) {
          feedVideoLeft.currentTime = Math.max(0, feedVideoLeft.currentTime - 5);
          showFeedHint('-5s', false, 800);
        }
        break;
      case 'ArrowRight':
        e.preventDefault();
        const feedVideoRight = getFeedVideo();
        if (feedVideoRight.duration) {
          feedVideoRight.currentTime = Math.min(feedVideoRight.duration, feedVideoRight.currentTime + 5);
          showFeedHint('+5s', false, 800);
        }
        break;
      case 'KeyF':
        e.preventDefault();
        if (fullscreenBtn) fullscreenBtn.click();
        break;
      case 'KeyP':
        e.preventDefault();
        togglePictureInPicture();
        break;
    }
  });

  if (qualityBtn && qualityMenu) {
    qualityBtn.addEventListener('click', (event) => {
      event.stopPropagation();
      const isOpen = qualityMenu.style.display === 'none';
      qualityMenu.style.display = isOpen ? 'block' : 'none';
      qualityBtn.setAttribute('aria-expanded', String(isOpen));
    });

    document.addEventListener('click', (event) => {
      if (!qualityBtn.contains(event.target) && !qualityMenu.contains(event.target)) {
        qualityMenu.style.display = 'none';
        qualityBtn.setAttribute('aria-expanded', 'false');
      }
    });
  }

  setInterval(() => {
    const feedVideo = getFeedVideo();
    if (state.currentView === 'recommend' && feedVideo && !feedVideo.paused) {
      persistFeedProgress();
    }
  }, 5000);
}

function renderViewError({ title = '加载失败', message = '请检查网络或稍后重试', onRetry = null }) {
  const wrap = document.createElement('div');
  wrap.className = 'view-error-state';
  wrap.setAttribute('role', 'alert');
  wrap.style.cssText = 'grid-column: 1 / -1; width: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 48px 20px; text-align: center; color: var(--text-secondary);';

  const iconSvg = `<svg viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="currentColor" stroke-width="1.6" style="margin: 0 auto 12px; display: block; opacity: 0.65; color: var(--accent);"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`;
  const titleHtml = `<div style="font-size: 15px; font-weight: 600; color: var(--text-primary); margin-bottom: 6px;">${escapeHtml(title)}</div>`;
  const cleanMsg = typeof message === 'string' ? message : (message?.message || String(message || ''));
  const msgHtml = `<div style="font-size: 13px; color: var(--text-muted); max-width: 380px; line-height: 1.5; margin-bottom: 16px;">${escapeHtml(cleanMsg)}</div>`;

  wrap.innerHTML = `${iconSvg}${titleHtml}${msgHtml}`;
  if (typeof onRetry === 'function') {
    const retryBtn = document.createElement('button');
    retryBtn.type = 'button';
    retryBtn.className = 'btn-subtle';
    retryBtn.textContent = '点击重试';
    retryBtn.onclick = () => onRetry();
    wrap.appendChild(retryBtn);
  }
  return wrap;
}

function renderViewEmpty({ title = '暂无内容', description = '', actionText = '', onAction = null, icon = 'drama' }) {
  const wrap = document.createElement('div');
  wrap.className = 'view-empty-state';
  wrap.style.cssText = 'grid-column: 1 / -1; width: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 56px 20px; text-align: center; color: var(--text-secondary);';

  const svgIcons = {
    drama: '<svg viewBox="0 0 24 24" width="46" height="46" fill="none" stroke="currentColor" stroke-width="1.5" style="margin:0 auto 12px;display:block;opacity:0.45;"><rect x="2" y="4" width="20" height="16" rx="3"/><path d="M7 4v16M17 4v16M2 12h20M2 8h5M2 16h5M17 8h5M17 16h5"/></svg>',
    calendar: '<svg viewBox="0 0 24 24" width="46" height="46" fill="none" stroke="currentColor" stroke-width="1.5" style="margin:0 auto 12px;display:block;opacity:0.45;"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>',
    search: '<svg viewBox="0 0 24 24" width="46" height="46" fill="none" stroke="currentColor" stroke-width="1.5" style="margin:0 auto 12px;display:block;opacity:0.45;"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>',
    rank: '<svg viewBox="0 0 24 24" width="46" height="46" fill="none" stroke="currentColor" stroke-width="1.5" style="margin:0 auto 12px;display:block;opacity:0.45;"><path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/></svg>',
    history: '<svg viewBox="0 0 24 24" width="46" height="46" fill="none" stroke="currentColor" stroke-width="1.5" style="margin:0 auto 12px;display:block;opacity:0.45;"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>',
    favorite: '<svg viewBox="0 0 24 24" width="46" height="46" fill="none" stroke="currentColor" stroke-width="1.5" style="margin:0 auto 12px;display:block;opacity:0.45;"><path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"/></svg>',
    like: '<svg viewBox="0 0 24 24" width="46" height="46" fill="none" stroke="currentColor" stroke-width="1.5" style="margin:0 auto 12px;display:block;opacity:0.45;"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg>'
  };

  const iconSvg = svgIcons[icon] || svgIcons.drama;
  wrap.innerHTML = `
    ${iconSvg}
    <div style="font-size: 15px; font-weight: 500; color: var(--text-primary); margin-bottom: 6px;">${escapeHtml(title)}</div>
    ${description ? `<div style="font-size: 13px; color: var(--text-muted); margin-bottom: 16px;">${escapeHtml(description)}</div>` : ''}
  `;
  if (actionText && typeof onAction === 'function') {
    const actBtn = document.createElement('button');
    actBtn.type = 'button';
    actBtn.className = 'btn-subtle';
    actBtn.textContent = actionText;
    actBtn.onclick = () => onAction();
    wrap.appendChild(actBtn);
  }
  return wrap;
}

function createDramaCard(item, fallbackCover = '') {

  const card = document.createElement('div');
  card.className = 'drama-card';
  card.tabIndex = 0;
  card.setAttribute('role', 'button');
  card.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      card.click();
    }
  });
  const upcoming = Boolean(item.upcoming) || item.status === 'upcoming';
  const isSourceNovel = item.kind === 'source_novel';
  if (isSourceNovel) {
    card.classList.add('source-novel-card');
  } else if (upcoming) {
    card.classList.add('upcoming-card');
    card.onclick = () => openSeriesDetail(item.id || item.seriesId);
  } else if (typeof item.onClick === 'function') {
    card.onclick = item.onClick;
  } else if (item.episodeNum) {
    card.onclick = () => openPlayer(item.id || item.seriesId, item.episodeNum);
  } else {
    card.onclick = () => openSeriesDetail(item.id || item.seriesId);
  }

  let epBadgeText = '完结';
  if (upcoming) {
    epBadgeText = '待上线';
  } else if (item.episodeNum) {
    epBadgeText = `第${item.episodeNum}集`;
  } else if (item.episodeCount) {
    epBadgeText = `${item.episodeCount}集`;
  }

  const safeTitle = escapeHtml(item.title || '');
  const deleteBtnHtml = item.onDelete
    ? `<button class="card-badge-delete" title="${escapeHtml(item.deleteTitle || '删除')}">
        <svg viewBox="0 0 24 24" width="13" height="13" fill="currentColor"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>
      </button>`
    : '';

  const descText = (item.author || item.description || '').trim();
  const descHtml = descText ? `<div class="card-desc" title="${escapeHtml(descText)}">${escapeHtml(descText)}</div>` : '';

  card.innerHTML = `
    <div class="card-poster-wrap">
      <img class="card-poster" src="${safeImageSrc(item.coverUrl || fallbackCover)}" alt="${safeTitle}" loading="lazy" decoding="async" onerror="handleImgError(this)">
      ${item.category ? `<span class="card-badge-top">${escapeHtml(item.category)}</span>` : ''}
      <span class="card-badge-episodes">${escapeHtml(item.badgeText || epBadgeText)}</span>
      ${deleteBtnHtml}
      ${upcoming ? '<div class="card-reservation-overlay"></div>' : ''}
    </div>
    <div class="card-info">
      <div class="card-title" title="${safeTitle}">${safeTitle}</div>
      <div class="card-heat">${escapeHtml(item.heat || '热播中')}</div>
      ${descHtml}
    </div>
  `;

  if (upcoming) {
    const overlay = card.querySelector('.card-reservation-overlay');
    if (overlay) {
      overlay.innerHTML = `
        <button type="button" class="card-reserve-btn ${item.reserved ? 'reserved' : ''}">
          ${item.reserved ? '已预约' : '预约'}
        </button>
      `;
      overlay.querySelector('.card-reserve-btn').onclick = (event) => {
        event.stopPropagation();
        toggleCardReservation(item, event.currentTarget);
      };
    }
  }

  if (item.onDelete) {
    const delBtn = card.querySelector('.card-badge-delete');
    if (delBtn) {
      delBtn.onclick = (e) => {
        e.stopPropagation();
        item.onDelete();
      };
    }
  }

  return card;
}

async function toggleCardReservation(item, button) {
  const previousText = button.textContent;
  const previousReserved = Boolean(item.reserved);
  button.disabled = true;
  button.textContent = previousReserved ? '取消中' : '预约中';
  try {
    const seriesId = item.id || item.seriesId;
    const reserved = await apiCall(
      'ToggleReservation',
      seriesId,
      item.title || '',
      item.coverUrl || '',
      item.heat || '',
      ''
    );
    const nowReserved = Boolean(reserved);
    button.textContent = nowReserved ? '已预约' : '预约';
    button.classList.toggle('reserved', nowReserved);
    state.reservationsCache = null;
    showToast(nowReserved ? `已预约《${item.title}》` : `已取消预约《${item.title}》`);
  } catch (err) {
    button.textContent = previousReserved ? '已预约' : '预约';
    showToast(err);
  } finally {
    button.disabled = false;
    if (button.textContent === '预约中' || button.textContent === '取消中') {
      button.textContent = previousText;
    }
  }
}

// ==========================================================================
// 2. Discover View (找剧)
// ==========================================================================

async function loadDiscoverFilters() {
  const panel = document.getElementById('discover-filters');
  const alreadyRendered = state.discover.renderedGenre === state.discover.genre && panel && panel.children.length > 0;

  if (alreadyRendered) {
    // If filters for this genre are already mounted, only re-execute if grid is empty
    const grid = document.getElementById('discover-grid');
    if (!grid || grid.children.length === 0) {
      executeDiscover();
    }
    return;
  }

  const panelToken = ++discoverPanelToken;
  panel.innerHTML = renderFilterPanelSkeleton();
  panel.classList.add('discover-section-enter');

  // Parallelize filter panel fetch and initial discover fetch simultaneously!
  const filtersPromise = apiCall('GetFilterPanel', state.discover.genre);
  executeDiscover();

  try {
    const rows = await filtersPromise;
    if (panelToken !== discoverPanelToken) return;
    panel.innerHTML = '';
    state.discover.renderedGenre = state.discover.genre;

    // Genre Row
    const genreRow = document.createElement('div');
    genreRow.className = 'filter-row';
    genreRow.innerHTML = `
      <span class="filter-label">体裁</span>
      <div class="filter-chips">
        <button class="filter-chip ${state.discover.genre === 'short_play' ? 'active' : ''}" data-key="genre" data-val="short_play">真人剧</button>
        <button class="filter-chip ${state.discover.genre === 'comic_series' ? 'active' : ''}" data-key="genre" data-val="comic_series">漫剧</button>
        <button class="filter-chip ${state.discover.genre === 'ai_series' ? 'active' : ''}" data-key="genre" data-val="ai_series">AI剧</button>
      </div>
    `;
    panel.appendChild(genreRow);

    if (rows && rows.length > 0) {
      rows.forEach(r => {
        // The top genre selector is rendered separately because switching it
        // changes the set of rows returned by the filter-panel endpoint.
        if (!r.options || r.options.length === 0 || r.key === 'genre') return;

        const rowElem = document.createElement('div');
        rowElem.className = 'filter-row';
        const selectedValue = state.discover.selections[r.key] || '';
        let chipsHtml = `<button class="filter-chip ${selectedValue ? '' : 'active'}" data-key="${escapeHtml(r.key)}" data-val="all">全部</button>`;
        r.options.forEach(opt => {
          const active = selectedValue === opt.value ? ' active' : '';
          chipsHtml += `<button class="filter-chip${active}" data-key="${escapeHtml(r.key)}" data-val="${escapeHtml(opt.value)}">${escapeHtml(opt.name)}</button>`;
        });
        rowElem.innerHTML = `
          <span class="filter-label">${escapeHtml(r.name.replace(/^全部/, ''))}</span>
          <div class="filter-chips">${chipsHtml}</div>
        `;
        panel.appendChild(rowElem);
      });
    }

    // Bind chip clicks
    panel.querySelectorAll('.filter-chip').forEach(chip => {
      chip.addEventListener('click', () => {
        const row = chip.parentElement;
        row.querySelectorAll('.filter-chip').forEach(c => c.classList.remove('active'));
        chip.classList.add('active');

        const key = chip.dataset.key;
        const val = chip.dataset.val === 'all' ? '' : chip.dataset.val;
        if (key === 'genre') state.discover.genre = val;
        else if (val) state.discover.selections[key] = val;
        else delete state.discover.selections[key];

        state.discover.cursor = '';
        if (key === 'genre') {
          state.discover.selections = {};
          transitionDiscoverFilters();
          return;
        }
        executeDiscover();
      });
    });
  } catch (err) {
    if (panelToken !== discoverPanelToken) return;
    panel.innerHTML = '';
    panel.appendChild(renderViewError({
      title: '加载筛选面板失败',
      message: err,
      onRetry: () => loadDiscoverFilters()
    }));
  }
}

function disconnectDiscoverObserver() {
  if (state.discover.observer) {
    state.discover.observer.disconnect();
    state.discover.observer = null;
  }
}

function renderDiscoverLoadMore() {
  const wrap = document.getElementById('discover-load-more');
  const button = document.getElementById('btn-discover-load-more');
  if (!wrap || !button) return;

  wrap.hidden = !state.discover.hasMore;
  button.disabled = state.discover.loading;
  button.textContent = state.discover.loading ? '加载中...' : '加载更多';
}

function observeDiscoverLoadMore() {
  disconnectDiscoverObserver();
  const sentinel = document.getElementById('discover-load-more');
  const viewport = document.querySelector('.view-viewport');
  if (!sentinel || sentinel.hidden || typeof IntersectionObserver === 'undefined' || !viewport) return;

  state.discover.observer = new IntersectionObserver(entries => {
    if (entries.some(entry => entry.isIntersecting)) loadMoreDiscover();
  }, { root: viewport, rootMargin: '160px 0px' });
  state.discover.observer.observe(sentinel);
}

async function loadMoreDiscover() {
  if (state.discover.loading || !state.discover.hasMore) return;
  await executeDiscover(false);
}

function discoverVisibleCapacity(grid) {
  const viewport = grid.closest('.view-viewport');
  const columns = getComputedStyle(grid)
    .gridTemplateColumns
    .split(' ')
    .filter(Boolean).length || 1;
  const card = grid.querySelector('.drama-card');
  const style = getComputedStyle(grid);
  const rowHeight = card ? card.offsetHeight : Math.round((grid.clientWidth / columns) * 4 / 3);
  const rowGap = parseFloat(style.rowGap) || 0;

  if (!viewport) return columns;

  const viewportRect = viewport.getBoundingClientRect();
  const gridRect = grid.getBoundingClientRect();
  const visibleTop = Math.max(gridRect.top, viewportRect.top);
  const availableHeight = Math.max(0, viewportRect.bottom - visibleTop);
  const rows = Math.max(1, Math.ceil((availableHeight + rowGap) / (rowHeight + rowGap)));
  return columns * rows;
}

async function executeDiscover(reset = true) {
  const grid = document.getElementById('discover-grid');
  if (!reset && state.discover.loading) return;

  const requestToken = ++discoverRequestToken;
  if (reset) {
    disconnectDiscoverObserver();
    state.discover.items = [];
    state.discover.cursor = '';
    state.discover.hasMore = false;
    grid.innerHTML = renderDramaGridSkeleton(12);
    grid.classList.add('discover-section-enter');
  } else {
    state.discover.loading = true;
    renderDiscoverLoadMore();
  }

  try {
    let visibleTarget = 0;
    for (let page = 0; ; page += 1) {
      const res = await apiCall('GetDiscover', state.discover.genre, state.discover.selections, state.discover.cursor);
      if (requestToken !== discoverRequestToken) return;
      if (!res) break;

      const seen = new Set(state.discover.items.map(item => item.id));
      const fresh = (res.items || []).filter(item => item?.id && !seen.has(item.id));
      state.discover.items.push(...fresh);
      state.discover.cursor = res.cursor || '';
      state.discover.hasMore = Boolean(res.hasMore && state.discover.cursor);

      if (reset && page === 0) grid.innerHTML = '';
      if (fresh.length > 0) {
        const fragment = createDramaCardsFragment(fresh);
        grid.appendChild(fragment);
      }
      if (reset && page === 0) {
        visibleTarget = discoverVisibleCapacity(grid);
      }
      if (!state.discover.hasMore) break;
      if (reset && state.discover.items.length >= visibleTarget) break;
    }

    if (requestToken !== discoverRequestToken) return;
    if (state.discover.items.length === 0) {
      grid.innerHTML = '';
      grid.appendChild(renderViewEmpty({
        title: '暂无符合条件的短剧',
        description: '尝试切换其它体裁或重置筛选标签',
        actionText: '重置筛选',
        onAction: () => {
          state.discover.selections = {};
          transitionDiscoverFilters();
        }
      }));
    }
  } catch (err) {
    if (requestToken !== discoverRequestToken) return;
    const errStr = String(err || '');
    if (errStr.includes('content unavailable')) {
      grid.innerHTML = '';
      grid.appendChild(renderViewEmpty({
        title: '暂无符合条件的短剧',
        description: '该分类下内容正在上线筹备中'
      }));
      return;
    }
    if (state.discover.items.length > 0) {
      showToast('加载更多短剧失败');
    } else {
      grid.innerHTML = '';
      grid.appendChild(renderViewError({
        title: '短剧筛选加载失败',
        message: err,
        onRetry: () => executeDiscover(true)
      }));
    }
  } finally {
    if (requestToken === discoverRequestToken) {
      state.discover.loading = false;
      renderDiscoverLoadMore();
      observeDiscoverLoadMore();
    }
  }
}

function createDramaCardsFragment(items) {
  const fragment = document.createDocumentFragment();
  items.forEach(item => fragment.appendChild(createDramaCard(item)));
  return fragment;
}

let discoverPanelToken = 0;
let discoverRequestToken = 0;

function waitMs(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function transitionDiscoverFilters() {
  const panel = document.getElementById('discover-filters');
  const grid = document.getElementById('discover-grid');
  if (!panel || !grid) {
    loadDiscoverFilters();
    return;
  }

  const transitionToken = ++discoverPanelToken;
  panel.classList.remove('discover-section-enter');
  grid.classList.remove('discover-section-enter');
  panel.classList.add('discover-transition-leaving');
  grid.classList.add('discover-transition-leaving');

  await waitMs(140);
  if (transitionToken !== discoverPanelToken) return;

  panel.classList.remove('discover-transition-leaving');
  grid.classList.remove('discover-transition-leaving');
  panel.innerHTML = renderFilterPanelSkeleton();
  grid.innerHTML = renderDramaGridSkeleton(12);
  panel.classList.add('discover-section-enter');
  grid.classList.add('discover-section-enter');

  loadDiscoverFilters();
}

// ==========================================================================
// 3. Rankings View (排行榜)
// ==========================================================================

let rankRequestToken = 0;

function renderRankTabs(contents, activeContentId, activeBoardId) {
  const contentTabsElem = document.getElementById('rank-content-tabs');
  const boardTabsElem = document.getElementById('rank-board-tabs');
  if (!contentTabsElem || !boardTabsElem) return;

  contentTabsElem.innerHTML = '';
  contents.forEach(content => {
    const tab = document.createElement('button');
    tab.className = `rank-content-tab ${content.id === activeContentId ? 'active' : ''}`;
    tab.textContent = content.name;
    tab.onclick = () => {
      if (state.rank.content === content.id) return;
      state.rank.content = content.id;
      state.rank.board = content.boards && content.boards.length > 0 ? content.boards[0].id : '';
      renderRankTabs(state.rank.contents, state.rank.content, state.rank.board);
      loadRankings(true);
    };
    contentTabsElem.appendChild(tab);
  });

  boardTabsElem.innerHTML = '';
  const activeContent = contents.find(item => item.id === activeContentId) || contents[0];
  if (activeContent && activeContent.boards) {
    activeContent.boards.forEach(board => {
      const chip = document.createElement('button');
      chip.className = `rank-board-chip ${board.id === activeBoardId ? 'active' : ''}`;
      chip.textContent = board.name;
      chip.onclick = () => {
        if (state.rank.board === board.id) return;
        state.rank.board = board.id;
        renderRankTabs(state.rank.contents, state.rank.content, state.rank.board);
        loadRankings(true);
      };
      boardTabsElem.appendChild(chip);
    });
  }
}

function createRankItemCard(it) {
  const card = document.createElement('div');
  card.tabIndex = 0;
  card.setAttribute('role', 'button');
  card.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      card.click();
    }
  });
  let posClass = '';
  if (it.position === 1) posClass = 'top-1';
  else if (it.position === 2) posClass = 'top-2';
  else if (it.position === 3) posClass = 'top-3';
  const positionHtml = it.promoted
    ? '<span class="rank-position rank-promoted" title="平台推荐"></span>'
    : `<span class="rank-position ${posClass}">${it.position}</span>`;

  if (it.itemType === 'actor') {
    card.className = 'rank-item-card rank-actor-card';
    card.onclick = () => openActorDetail(it.userId, it.title || it.actor, it.avatarUrl || it.coverUrl);

    const detailsText = it.details && it.details.length > 0 ? it.details.join(' · ') : '';
    const introHtml = it.intro ? `<div class="rank-actor-intro" title="${escapeHtml(it.intro)}">${escapeHtml(it.intro)}</div>` : '';
    card.innerHTML = `
      ${positionHtml}
      <img class="rank-avatar" src="${safeImageSrc(it.avatarUrl || it.coverUrl)}" alt="${escapeHtml(it.title || it.actor || '')}" loading="lazy" decoding="async" onerror="handleImgError(this)">
      <div class="rank-item-info">
        <div class="rank-item-title">${escapeHtml(it.title || it.actor || '演员')}</div>
        <div class="rank-item-meta">
          <span class="rank-item-score" style="display:inline-flex;align-items:center;gap:3px;">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="currentColor"><path d="M19.48 12.35c-1.57-4.08-7.16-4.3-5.81-10.23c.1-.44-.34-.81-.74-.63c-3.15 1.4-8.08 6.47-5.93 12.87c.22.65-.3 1.34-.98 1.25c-.71-.09-1.29-.6-1.42-1.3c-.63-3.41.97-6.27.97-6.27c.18-.32-.15-.71-.5-.59c-2.32.79-4.97 3.52-4.97 7.55C1 20.19 5.03 24 10 24c5.52 0 10-4.48 10-10c0-.58-.06-1.14-.18-1.68c-.09-.43-.54-.62-.84-.32c-.52.52-1.12.97-1.78 1.35c-.44.25-.97-.04-.98-.55c-.03-1.02.57-2.18 1.26-3.45z"/></svg>
            ${escapeHtml(it.heat || '热门短剧演员')}
          </span>
          ${detailsText ? `<span class="rank-actor-details">${escapeHtml(detailsText)}</span>` : ''}
        </div>
        ${introHtml}
      </div>
      <button class="rank-actor-btn" title="查看演员主页">
        <span>查看主页</span>
        <svg viewBox="0 0 24 24" width="14" height="14"><path fill="currentColor" d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6-1.41-1.41z"/></svg>
      </button>
    `;
    return card;
  }

  card.className = 'rank-item-card';
  card.onclick = () => openSeriesDetail(it.seriesId);
  card.innerHTML = `
    ${positionHtml}
    <img class="rank-poster" src="${safeImageSrc(it.coverUrl)}" alt="${escapeHtml(it.title || '')}" loading="lazy" decoding="async" onerror="handleImgError(this)">
    <div class="rank-item-info">
      <div class="rank-item-title">${escapeHtml(it.title || '')}</div>
      <div class="rank-item-meta">
        <span>${escapeHtml(it.actor || (it.episodeCount ? `${it.episodeCount}集` : '短剧'))}</span>
        <span class="rank-item-score" style="display:inline-flex;align-items:center;gap:3px;">
          <svg viewBox="0 0 24 24" width="12" height="12" fill="currentColor"><path d="M19.48 12.35c-1.57-4.08-7.16-4.3-5.81-10.23c.1-.44-.34-.81-.74-.63c-3.15 1.4-8.08 6.47-5.93 12.87c.22.65-.3 1.34-.98 1.25c-.71-.09-1.29-.6-1.42-1.3c-.63-3.41.97-6.27.97-6.27c.18-.32-.15-.71-.5-.59c-2.32.79-4.97 3.52-4.97 7.55C1 20.19 5.03 24 10 24c5.52 0 10-4.48 10-10c0-.58-.06-1.14-.18-1.68c-.09-.43-.54-.62-.84-.32c-.52.52-1.12.97-1.78 1.35c-.44.25-.97-.04-.98-.55c-.03-1.02.57-2.18 1.26-3.45z"/></svg>
          ${escapeHtml(it.heat || it.score || '热榜推荐')}
        </span>
      </div>
    </div>
    <button class="rank-play-btn">立即播放</button>
  `;
  const playButton = card.querySelector('.rank-play-btn');
  if (playButton) {
    playButton.onclick = (event) => {
      event.stopPropagation();
      openPlayer(it.seriesId, 1);
    };
  }
  return card;
}

function appendRankLoadMore(listElem, loading = false) {
  if (!state.rank.cursor) return;
  const wrap = document.createElement('div');
  wrap.className = 'rank-load-more';
  wrap.innerHTML = loading
    ? createLoader('正在加载更多...')
    : '<button type="button" class="rank-more-btn">加载更多</button>';
  if (!loading) {
    wrap.querySelector('.rank-more-btn').onclick = () => loadRankings(false);
  }
  listElem.appendChild(wrap);
}

async function loadRankings(reset = true) {
  const listElem = document.getElementById('rank-items-list');
  if (!reset && !state.rank.cursor) return;

  const requestToken = ++rankRequestToken;
  const cursor = reset ? '' : state.rank.cursor;
  if (reset) {
    listElem.innerHTML = renderRankListSkeleton(8);
  } else {
    listElem.querySelector('.rank-load-more')?.remove();
    appendRankLoadMore(listElem, true);
  }

  try {
    const res = await apiCall('GetRankings', state.rank.content, state.rank.board, cursor);
    if (!res || requestToken !== rankRequestToken) return;

    state.rank.contents = res.contents || state.rank.contents;
    state.rank.cursor = res.cursor || '';
    state.rank.hasMore = Boolean(res.hasMore);
    if (reset) {
      renderRankTabs(state.rank.contents, res.content || state.rank.content, res.board || state.rank.board);
    }
    if (res.content) state.rank.content = res.content;
    if (res.board) state.rank.board = res.board;

    if (reset) listElem.innerHTML = '';
    listElem.querySelector('.rank-load-more')?.remove();
    if (reset && (!res.items || res.items.length === 0)) {
      listElem.innerHTML = '';
      listElem.appendChild(renderViewEmpty({
        title: '暂无上榜短剧',
        description: '该榜单数据更新中，请稍后再来看看',
        icon: 'rank'
      }));
      return;
    }

    const fragment = document.createDocumentFragment();
    (res.items || []).forEach(it => fragment.appendChild(createRankItemCard(it)));
    listElem.appendChild(fragment);
    appendRankLoadMore(listElem);
  } catch (err) {
    if (requestToken !== rankRequestToken) return;
    if (reset) {
      listElem.innerHTML = '';
      listElem.appendChild(renderViewError({
        title: '加载排行榜失败',
        message: err,
        onRetry: () => loadRankings(true)
      }));
    } else {
      showToast('加载更多榜单失败');
      appendRankLoadMore(listElem);
    }
  }
}

// ==========================================================================
// Actor Detail View Controller (演员主页)
// ==========================================================================

function openActorDetail(userId, name, avatarUrl) {
  if (!userId) {
    showToast('暂无该演员账号信息');
    return;
  }
  state.actor.userId = userId;
  state.actor.name = name || '演员';
  state.actor.avatarUrl = avatarUrl || '';
  state.actor.cursor = '';
  state.actor.hasMore = false;
  state.actor.loading = false;

  navigateTo('actor');
  loadActorDetail(userId);
}

async function loadActorDetail(userId) {
  const avatarElem = document.getElementById('actor-detail-avatar');
  const nameElem = document.getElementById('actor-detail-name');
  const tagsElem = document.getElementById('actor-detail-tags');
  const followersElem = document.getElementById('actor-stat-followers');
  const likesElem = document.getElementById('actor-stat-likes');
  const countElem = document.getElementById('actor-stat-works-count');
  const descElem = document.getElementById('actor-detail-desc');
  const grid = document.getElementById('actor-drama-grid');
  const loadMoreWrap = document.getElementById('actor-load-more-wrap');

  // Pre-fill immediate values from state
  if (state.actor.avatarUrl) avatarElem.src = state.actor.avatarUrl;
  if (state.actor.name) {
    nameElem.textContent = state.actor.name;
    const pageTitle = document.getElementById('page-title');
    if (pageTitle) pageTitle.textContent = `${state.actor.name} - 演员主页`;
    document.getElementById('actor-header-subtitle').textContent = `${state.actor.name}的短剧作品`;
  }
  tagsElem.innerHTML = '';
  descElem.textContent = '';
  followersElem.textContent = '--';
  likesElem.textContent = '--';
  countElem.textContent = '--';
  if (loadMoreWrap) loadMoreWrap.style.display = 'none';

  grid.innerHTML = renderDramaGridSkeleton(8);

  try {
    const profile = await apiCall('GetActorProfile', userId);
    if (!profile) {
      grid.innerHTML = '';
      grid.appendChild(renderViewError({
        title: '获取演员信息失败',
        onRetry: () => loadActorDetail(userId)
      }));
      return;
    }

    // Populate Hero
    avatarElem.src = profile.avatarUrl || state.actor.avatarUrl || DEFAULT_POSTER;
    nameElem.textContent = profile.name || state.actor.name;
    const pageTitle = document.getElementById('page-title');
    if (pageTitle) pageTitle.textContent = `${profile.name || state.actor.name} - 演员主页`;
    document.getElementById('actor-header-subtitle').textContent = `${profile.name || state.actor.name}的短剧作品`;

    tagsElem.innerHTML = '';
    if (profile.tags && profile.tags.length > 0) {
      profile.tags.forEach(t => {
        const chip = document.createElement('span');
        chip.className = 'actor-tag-chip';
        chip.textContent = t;
        tagsElem.appendChild(chip);
      });
    }

    followersElem.textContent = formatStatsNumber(profile.followers);
    likesElem.textContent = formatStatsNumber(profile.likes);
    countElem.textContent = profile.works ? profile.works.length : 0;
    descElem.textContent = profile.description || '暂无个人简介';

    // Populate Works Grid
    grid.innerHTML = '';
    if (!profile.works || profile.works.length === 0) {
      grid.appendChild(renderViewEmpty({
        title: '暂无参演短剧作品',
        description: '该演员暂无公开的短剧作品数据',
        actionText: '去推荐发现好剧',
        onAction: () => navigateTo('recommend')
      }));
    } else {
      profile.works.forEach(item => {
        grid.appendChild(createDramaCard(item));
      });
      document.getElementById('actor-works-sub').textContent = `共 ${profile.works.length} 部短剧作品`;
    }

    state.actor.cursor = profile.cursor || '';
    state.actor.hasMore = !!profile.hasMore;
    if (loadMoreWrap) loadMoreWrap.style.display = profile.hasMore ? 'flex' : 'none';
  } catch (err) {
    grid.innerHTML = '';
    grid.appendChild(renderViewError({
      title: '加载演员主页失败',
      message: err,
      onRetry: () => loadActorDetail(userId)
    }));
  }
}

async function loadMoreActorWorks() {
  if (!state.actor.hasMore || state.actor.loading || !state.actor.cursor) return;
  state.actor.loading = true;
  const btn = document.getElementById('btn-actor-load-more');
  const origText = btn.textContent;
  btn.textContent = '加载中...';

  try {
    const res = await apiCall('GetActorWorks', state.actor.userId, state.actor.cursor);
    if (res && res.works && res.works.length > 0) {
      const grid = document.getElementById('actor-drama-grid');
      res.works.forEach(item => {
        grid.appendChild(createDramaCard(item));
      });
      state.actor.cursor = res.cursor || '';
      state.actor.hasMore = !!res.hasMore;
      const countElem = document.getElementById('actor-stat-works-count');
      const curCount = grid.querySelectorAll('.drama-card').length;
      countElem.textContent = curCount;
      document.getElementById('actor-works-sub').textContent = `共 ${curCount} 部短剧作品`;
    } else {
      state.actor.hasMore = false;
    }
    const loadMoreWrap = document.getElementById('actor-load-more-wrap');
    if (loadMoreWrap) loadMoreWrap.style.display = state.actor.hasMore ? 'flex' : 'none';
  } catch (err) {
    showToast(`加载更多作品失败: ${err}`);
  } finally {
    btn.textContent = origText;
    state.actor.loading = false;
  }
}

function formatStatsNumber(n) {
  if (!n || n <= 0) return '0';
  if (n >= 100000000) {
    return (n / 100000000).toFixed(1).replace(/\.0$/, '') + '亿';
  }
  if (n >= 10000) {
    return (n / 10000).toFixed(1).replace(/\.0$/, '') + '万';
  }
  return n.toLocaleString();
}

// ==========================================================================
// 4. New Releases View (新剧)
// ==========================================================================

let newReleasesRequestToken = 0;

function renderNewCalendarTabs() {
  const datesElem = document.getElementById('new-calendar-dates');
  if (!datesElem) return;

  datesElem.innerHTML = '';
  state.newReleases.dates.forEach(date => {
    const tab = document.createElement('button');
    const activeDate = state.newReleases.date || state.newReleases.currentDate;
    tab.className = `calendar-tab ${date === activeDate ? 'active' : ''}`;

    const yr = date.substring(0, 4);
    const mo = date.substring(4, 6);
    const dy = date.substring(6, 8);
    const dateObj = new Date(`${yr}-${mo}-${dy}`);
    const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
    const weekday = weekdays[dateObj.getDay()];

    tab.innerHTML = `
      <span class="cal-weekday">${weekday}</span>
      <span class="cal-date">${mo}/${dy}</span>
    `;
    tab.onclick = () => {
      if (state.newReleases.date === date) return;
      state.newReleases.date = date;
      renderNewCalendarTabs();
      document.getElementById('new-drama-grid').innerHTML = renderDramaGridSkeleton(12);
      loadNewReleases();
    };
    datesElem.appendChild(tab);
  });
}

async function loadNewReleases() {
  const grid = document.getElementById('new-drama-grid');
  const datesElem = document.getElementById('new-calendar-dates');
  const requestToken = ++newReleasesRequestToken;
  if (!grid.children.length) {
    grid.innerHTML = renderDramaGridSkeleton(12);
  }

  try {
    const queryDate = state.newReleases.tab === 'calendar' ? state.newReleases.date : '';
    if (state.newReleases.tab !== 'calendar') {
      // The hot tab is the platform's all-content new-drama board, while the
      // calendar tab uses the subscribe-list-backed release schedule.
      const rank = await apiCall('GetRankings', 'all', 'ranklist_new_rank_sc', '');
      if (!rank || requestToken !== newReleasesRequestToken) return;
      datesElem.innerHTML = '';
      grid.innerHTML = '';
      const hotItems = (rank.items || [])
        .filter(item => item.itemType === 'drama' && item.seriesId)
        .map(item => ({
          seriesId: item.seriesId,
          title: item.title,
          coverUrl: item.coverUrl,
          episodeCount: item.episodeCount,
          online: true,
          heat: item.heat || '新剧热播',
          category: '新剧',
          reserved: false
        }));
      if (hotItems.length === 0) {
        grid.innerHTML = '';
        grid.appendChild(renderViewEmpty({
          title: '暂无热播新剧',
          description: '最新剧集正在转码上线中',
          icon: 'calendar'
        }));
        return;
      }

      const fragment = document.createDocumentFragment();
      hotItems.forEach(item => {
        fragment.appendChild(createNewReleaseCard(item));
      });
      grid.appendChild(fragment);
      return;
    }

    const res = await apiCall('GetNewReleases', queryDate);
    if (!res || requestToken !== newReleasesRequestToken) return;

    state.newReleases.dates = res.dates || [];
    state.newReleases.currentDate = res.currentDate || '';
    if (!state.newReleases.date) state.newReleases.date = state.newReleases.currentDate;
    renderNewCalendarTabs();

    grid.innerHTML = '';
    if (!res.items || res.items.length === 0) {
      grid.innerHTML = '';
      grid.appendChild(renderViewEmpty({
        title: '当日暂无新剧上线',
        description: '可选择其他日期查看上映排期，或前往推荐发现好剧',
        actionText: '去推荐发现好剧',
        onAction: () => navigateTo('recommend'),
        icon: 'calendar'
      }));
      return;
    }

    const fragment = document.createDocumentFragment();
    res.items.forEach(item => {
      fragment.appendChild(createNewReleaseCard(item));
    });
    grid.appendChild(fragment);
  } catch (err) {
    if (requestToken !== newReleasesRequestToken) return;
    grid.innerHTML = '';
    grid.appendChild(renderViewError({
      title: '加载新剧失败',
      message: err,
      onRetry: () => loadNewReleases()
    }));
  }
}

function createNewReleaseCard(item) {
  const statusBadge = item.online ? '已上线' : (item.note || '预计上线');
  return createDramaCard({
    id: item.seriesId,
    title: item.title,
    coverUrl: item.coverUrl,
    episodeCount: item.episodeCount,
    category: statusBadge,
    heat: item.heat || (item.online ? '新剧热播' : '预约中'),
    description: item.category,
    upcoming: !item.online,
    reserved: Boolean(item.reserved)
  });
}

// ==========================================================================
// ==========================================================================
// 5. History View (观看历史) - Modern Horizontal Row Layout
// ==========================================================================

async function loadHistory() {
  const grid = document.getElementById('history-grid');
  if (!state.historyView.list || state.historyView.list.length === 0) {
    grid.innerHTML = renderDramaGridSkeleton(8);
  } else {
    renderHistoryRows();
  }

  try {
    const list = await apiCall('GetHistory') || [];
    state.historyView.list = list;
    renderHistoryRows();
  } catch (err) {
    grid.innerHTML = '';
    grid.appendChild(renderViewError({
      title: '读取观看历史失败',
      message: err,
      onRetry: () => loadHistory()
    }));
  }
}

function renderHistoryRows() {
  const grid = document.getElementById('history-grid');
  let items = state.historyView.list || [];

  // Filter by unwatched / completed
  if (state.historyView.filter === 'unwatched') {
    items = items.filter(it => {
      const pct = (it.positionSec / (it.durationSec || 120));
      return pct < 0.95 && it.episodeNum < (it.totalEpisodes || 80);
    });
  } else if (state.historyView.filter === 'completed') {
    items = items.filter(it => {
      const pct = (it.positionSec / (it.durationSec || 120));
      return pct >= 0.95 || it.episodeNum >= (it.totalEpisodes || 80);
    });
  }

  // Filter by search keyword
  if (state.historyView.search) {
    const q = state.historyView.search.trim().toLowerCase();
    items = items.filter(it => (it.title || '').toLowerCase().includes(q));
  }

  grid.innerHTML = '';
  if (items.length === 0) {
    grid.appendChild(renderViewEmpty({
      title: '暂无匹配的观看记录',
      description: state.historyView.search ? '没有找到符合搜索条件的短剧' : '快去推荐发现好剧吧',
      actionText: '去推荐发现好剧',
      onAction: () => navigateTo('recommend'),
      icon: 'history'
    }));
    return;
  }

  const fragment = document.createDocumentFragment();
  items.forEach(it => {
    const card = document.createElement('div');
    card.className = 'drama-card history-card';
    card.tabIndex = 0;
    card.setAttribute('role', 'button');
    card.onclick = () => openSeriesDetail(it.seriesId);
    card.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        card.click();
      }
    });

    const dur = it.durationSec || 120;
    const pct = Math.min(100, Math.max(3, Math.round((it.positionSec / dur) * 100)));
    const isCompleted = pct >= 95 || (it.episodeNum >= (it.totalEpisodes || 80) && pct > 80);

    // Relative timestamp format
    let timeLabel = '';
    if (it.updatedAt > 0) {
      const dt = new Date(it.updatedAt * 1000);
      const now = new Date();
      const diffSec = Math.floor((now - dt) / 1000);
      if (diffSec < 60) {
        timeLabel = '刚刚';
      } else if (diffSec < 3600) {
        timeLabel = `${Math.floor(diffSec / 60)}分钟前`;
      } else if (diffSec < 86400) {
        timeLabel = `${Math.floor(diffSec / 3600)}小时前`;
      } else if (diffSec < 86400 * 2) {
        timeLabel = '昨天';
      } else if (diffSec < 86400 * 3) {
        timeLabel = '前天';
      } else {
        timeLabel = `${dt.getMonth() + 1}月${dt.getDate()}日`;
      }
    }

    const badgeText = isCompleted ? '已看完' : `看到第 ${it.episodeNum} 集`;
    const totalEpisodes = Number(it.totalEpisodes) || 0;

    card.innerHTML = `
      <div class="card-poster-wrap">
        <img class="card-poster" src="${safeImageSrc(it.coverUrl)}" alt="${escapeHtml(it.title || '')}" loading="lazy" decoding="async" onerror="handleImgError(this)">
        <span class="card-badge-top ${isCompleted ? 'badge-completed' : ''}">${badgeText}</span>
        <button class="card-badge-delete" title="删除记录">
          <svg viewBox="0 0 24 24" width="13" height="13" fill="currentColor"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>
        </button>
        <div class="card-progress-bar">
          <div class="card-progress-fill" style="width: ${pct}%;"></div>
        </div>
        <span class="card-badge-episodes">${totalEpisodes > 0 ? `共${totalEpisodes}集` : ''}</span>
      </div>
      <div class="card-info">
        <div class="card-title" title="${escapeHtml(it.title || '')}">${escapeHtml(it.title || '')}</div>
        <div class="card-history-meta">
          <span class="card-history-ep">${isCompleted ? '全剧看完' : `看到第${it.episodeNum}集`}</span>
          <span class="card-history-time">${timeLabel}</span>
        </div>
      </div>
    `;

    const delBtn = card.querySelector('.card-badge-delete');
    if (delBtn) {
      delBtn.onclick = async (e) => {
        e.stopPropagation();
        await apiCall('DeleteHistory', it.seriesId);
        state.historyView.list = state.historyView.list.filter(x => x.seriesId !== it.seriesId);
        renderHistoryRows();
        showToast('已删除记录');
      };
    }

    fragment.appendChild(card);
  });
  grid.appendChild(fragment);
}

// Sub-filter tabs for History (全部 / 未看完 / 已看完)
document.addEventListener('DOMContentLoaded', () => {
  const newMainTabs = document.getElementById('new-main-tabs');
  if (newMainTabs) {
    newMainTabs.addEventListener('click', (event) => {
      const btn = event.target.closest('button');
      if (!btn) return;
      const nextTab = btn.dataset.tab;
      if (!nextTab || nextTab === state.newReleases.tab) return;

      state.newReleases.tab = nextTab;
      newMainTabs.querySelectorAll('button').forEach(tab => {
        tab.classList.toggle('active', tab === btn);
      });
      loadNewReleases();
    });
  }

  const reservationTabs = document.getElementById('reservation-tabs');
  if (reservationTabs) {
    reservationTabs.addEventListener('click', (e) => {
      const btn = e.target.closest('button');
      if (!btn) return;
      state.reservationsTab = btn.getAttribute('data-tab') || 'online';
      renderReservationsList(state.reservationsCache || []);
    });
  }

  const favoritesTabs = document.getElementById('favorites-tabs');
  if (favoritesTabs) {
    favoritesTabs.addEventListener('click', (e) => {
      const btn = e.target.closest('button');
      if (!btn) return;
      const target = btn.getAttribute('data-tab') || 'favorites';
      switchFavoritesTab(target);
    });
  }

  const pills = document.getElementById('history-filter-pills');
  if (pills) {
    pills.addEventListener('click', (e) => {
      const btn = e.target.closest('button');
      if (!btn) return;
      pills.querySelectorAll('button').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      state.historyView.filter = btn.getAttribute('data-filter') || 'all';
      renderHistoryRows();
    });
  }

  const searchInput = document.getElementById('history-search-input');
  if (searchInput) {
    searchInput.addEventListener('input', (e) => {
      state.historyView.search = e.target.value;
      renderHistoryRows();
    });
  }
});

document.getElementById('btn-clear-all-history').addEventListener('click', async () => {
  if (await nativeConfirm('确认清空所有播放记录？', '清空后将无法恢复之前的播放进度。', '清空记录', true)) {
    await apiCall('ClearHistory');
    state.historyView.list = [];
    renderHistoryRows();
    showToast('已清空全部播放记录');
  }
});

// ==========================================================================
// 6. Favorites & Likes View (我的收藏与点赞 - 真实数据)
// ==========================================================================

async function loadFavoritesAndLikes() {
  const grid = document.getElementById('favorites-grid');
  if (!grid) return;

  const currentTab = state.favoritesView.tab || 'favorites';
  const hasCached = (currentTab === 'favorites' && state.favoritesView.favoritesList.length > 0) ||
                    (currentTab === 'likes' && state.favoritesView.likesList.length > 0);

  if (!hasCached && grid.children.length === 0) {
    grid.innerHTML = renderDramaGridSkeleton(8);
  }

  // Sync tab active styles and title
  document.querySelectorAll('#favorites-tabs .collection-tab').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.tab === currentTab);
  });
  const titleElem = document.getElementById('favorites-title-text');
  if (titleElem) {
    titleElem.textContent = currentTab === 'likes' ? '我的点赞' : '我的收藏';
  }

  try {
    const [favs, likes] = await Promise.all([
      apiCall('GetFavorites').catch(err => {
        console.error('Failed to get favorites:', err);
        return [];
      }),
      apiCall('GetLikes').catch(err => {
        console.error('Failed to get likes:', err);
        return [];
      })
    ]);

    state.favoritesView.favoritesList = favs || [];
    state.favoritesView.likesList = likes || [];

    updateFavoritesTabsCount();
    renderFavoritesOrLikes();
  } catch (err) {
    grid.innerHTML = '';
    grid.appendChild(renderViewError({
      title: '加载收藏与点赞失败',
      message: err,
      onRetry: () => loadFavoritesAndLikes()
    }));
  }
}

function updateFavoritesTabsCount() {
  const favCount = state.favoritesView.favoritesList ? state.favoritesView.favoritesList.length : 0;
  const likesCount = state.favoritesView.likesList ? state.favoritesView.likesList.length : 0;

  const tabFav = document.getElementById('tab-favorites-count');
  if (tabFav) tabFav.textContent = favCount;

  const tabLikes = document.getElementById('tab-likes-count');
  if (tabLikes) tabLikes.textContent = likesCount;

  const desc = document.getElementById('favorites-count-desc');
  if (desc) {
    desc.textContent = `共 ${favCount} 部收藏 · ${likesCount} 个点赞`;
  }
}

function switchFavoritesTab(tab) {
  if (tab !== 'favorites' && tab !== 'likes') tab = 'favorites';
  state.favoritesView.tab = tab;

  document.querySelectorAll('#favorites-tabs .collection-tab').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.tab === tab);
  });

  const titleElem = document.getElementById('favorites-title-text');
  if (titleElem) {
    titleElem.textContent = tab === 'likes' ? '我的点赞' : '我的收藏';
  }

  const pageTitle = document.getElementById('page-title');
  if (pageTitle && state.currentView === 'favorites') {
    pageTitle.textContent = tab === 'likes' ? '我的点赞' : '我的收藏';
  }

  renderFavoritesOrLikes();
}

function renderFavoritesOrLikes() {
  const grid = document.getElementById('favorites-grid');
  if (!grid) return;

  grid.innerHTML = '';
  const currentTab = state.favoritesView.tab || 'favorites';

  if (currentTab === 'favorites') {
    const list = state.favoritesView.favoritesList;
    if (!list || list.length === 0) {
      grid.appendChild(renderViewEmpty({
        title: '暂无收藏短剧',
        description: '在短剧详情或播放器中点击收藏，精彩不迷路',
        actionText: '去推荐发现好剧',
        onAction: () => navigateTo('recommend'),
        icon: 'favorite'
      }));
      return;
    }
    const fragment = document.createDocumentFragment();
    list.forEach(it => {
      fragment.appendChild(createDramaCard({
        id: it.seriesId,
        title: it.title,
        coverUrl: it.coverUrl,
        episodeCount: it.totalEpisodes,
        heat: '已收藏',
        description: it.description || '精彩短剧',
        category: '收藏',
        deleteTitle: '取消收藏',
        onDelete: async () => {
          try {
            await apiCall('ToggleFavorite', it.seriesId, it.title, it.coverUrl, it.description, it.totalEpisodes);
            state.favoritesView.favoritesList = state.favoritesView.favoritesList.filter(x => x.seriesId !== it.seriesId);
            updateFavoritesTabsCount();
            renderFavoritesOrLikes();
            showToast('已取消收藏');
          } catch (err) {
            showToast('操作失败: ' + err);
          }
        }
      }));
    });
    grid.appendChild(fragment);
  } else {
    const list = state.favoritesView.likesList;
    if (!list || list.length === 0) {
      grid.appendChild(renderViewEmpty({
        title: '暂无点赞记录',
        description: '在播放器中点击爱心即可点赞，记录心动瞬间',
        actionText: '去推荐发现好剧',
        onAction: () => navigateTo('recommend'),
        icon: 'like'
      }));
      return;
    }
    const fragment = document.createDocumentFragment();
    list.forEach(it => {
      fragment.appendChild(createDramaCard({
        id: it.seriesId,
        title: it.title,
        coverUrl: it.coverUrl,
        episodeNum: it.episodeNum,
        episodeCount: it.totalEpisodes,
        heat: it.episodeNum > 0 ? `已点赞 第${it.episodeNum}集` : '已点赞',
        description: it.description || (it.episodeNum > 0 ? `第${it.episodeNum}集 · 点赞记录` : '精彩短剧'),
        category: '点赞',
        deleteTitle: '取消点赞',
        onDelete: async () => {
          try {
            await apiCall('ToggleLike', it.seriesId, it.episodeId, it.title, it.coverUrl, it.episodeNum);
            state.favoritesView.likesList = state.favoritesView.likesList.filter(x => !(x.seriesId === it.seriesId && x.episodeNum === it.episodeNum));
            updateFavoritesTabsCount();
            renderFavoritesOrLikes();
            showToast('已取消点赞');
          } catch (err) {
            showToast('操作失败: ' + err);
          }
        }
      }));
    });
    grid.appendChild(fragment);
  }
}

async function loadFavorites() {
  state.favoritesView.tab = 'favorites';
  await loadFavoritesAndLikes();
}

async function loadLikes() {
  state.favoritesView.tab = 'likes';
  await loadFavoritesAndLikes();
}

// ==========================================================================
// 8. Reservations View (我的预约 - 当前账号真实预约数据)
// ==========================================================================

async function loadReservations() {
  const grid = document.getElementById('reservations-grid');
  if (!state.reservationsCache || state.reservationsCache.length === 0) {
    grid.innerHTML = renderDramaGridSkeleton(8);
  } else {
    renderReservationsList(state.reservationsCache);
  }

  try {
    const list = await apiCall('GetReservations') || [];
    state.reservationsCache = list;
    renderReservationsList(list);
  } catch (err) {
    if (!state.reservationsCache || state.reservationsCache.length === 0) {
      grid.innerHTML = '';
      grid.appendChild(renderViewError({
        title: '加载预约失败',
        message: err,
        onRetry: () => loadReservations()
      }));
    }
  }
}

function renderReservationsList(list) {
  const grid = document.getElementById('reservations-grid');
  const countDesc = document.getElementById('reservations-count-desc');
  const onlineList = list.filter(it => it.isOnline).sort((a, b) => getReservationDate(b) - getReservationDate(a));
  const pendingList = list.filter(it => !it.isOnline).sort((a, b) => {
    const ta = getReservationDate(a);
    const tb = getReservationDate(b);
    if (!ta && !tb) return 0;
    if (!ta) return 1;
    if (!tb) return -1;
    return ta - tb;
  });

  if (countDesc) {
    countDesc.textContent = `共 ${list.length} 部 · 已上线 ${onlineList.length} · 待上线 ${pendingList.length}`;
  }

  document.querySelectorAll('#reservation-tabs .reservation-tab').forEach(tab => {
    const key = tab.getAttribute('data-tab');
    const count = key === 'online' ? onlineList.length : pendingList.length;
    tab.innerHTML = `<span>${key === 'online' ? '已上线' : '待上线'}</span><em>${count}</em>`;
    tab.classList.toggle('active', key === state.reservationsTab);
  });

  grid.innerHTML = '';
  if (list.length === 0) {
    grid.innerHTML = '';
    grid.appendChild(renderViewEmpty({
      title: '暂无预约短剧',
      description: '可在新剧日历中预约感兴趣的新剧，上线时将自动同步',
      actionText: '去新剧日历预约',
      onAction: () => navigateTo('new'),
      icon: 'calendar'
    }));
    return;
  }

  const activeList = state.reservationsTab === 'pending' ? pendingList : onlineList;
  if (activeList.length === 0) {
    grid.innerHTML = '';
    grid.appendChild(renderViewEmpty({
      title: state.reservationsTab === 'pending' ? '暂无待上线短剧' : '暂无已上线短剧',
      description: state.reservationsTab === 'pending' ? '所有预约的短剧均已上线播放' : '预约的剧集正在准备上线',
      icon: 'calendar'
    }));
    return;
  }

  const fragment = document.createDocumentFragment();
  activeList.forEach(it => {
    const card = document.createElement('div');
    card.className = 'drama-card';
    card.tabIndex = 0;
    card.setAttribute('role', 'button');
    card.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        card.click();
      }
    });

    const isOnline = it.isOnline;
    if (isOnline) {
      card.onclick = () => openSeriesDetail(it.seriesId);
    } else {
      card.classList.add('upcoming-card');
    }
    const date = getReservationDate(it);
    const isScheduled = date && date.getTime() > Date.now();
    const dateText = isOnline
      ? (date ? formatReservationDate(date) : '已上线')
      : (isScheduled ? formatReservationDate(date) : '即将上线');
    const dateLabel = isOnline ? '上线日期' : (isScheduled ? '待上线日期' : '上线状态');
    const safeTitle = escapeHtml(it.title);

    card.innerHTML = `
      <div class="card-poster-wrap">
        <img class="card-poster" src="${safeImageSrc(it.coverUrl)}" alt="${safeTitle}" loading="lazy" decoding="async" onerror="handleImgError(this)">
        ${isOnline ? '' : '<span class="card-badge-top">待上线</span>'}
      </div>
      <div class="card-info">
        <div class="card-title reservation-title" title="${safeTitle}">${safeTitle}</div>
        <div class="card-heat reservation-date" title="${dateLabel}：${dateText}">
          <span>${dateLabel}</span><span>${dateText}</span>
        </div>
        <div class="card-desc reservation-actions">
          <button class="btn-cancel-res" title="取消预约">取消预约</button>
        </div>
      </div>
    `;

    card.querySelector('.btn-cancel-res').onclick = async (e) => {
      e.stopPropagation();
      const button = e.currentTarget;
      button.disabled = true;
      try {
        if (!await nativeConfirm(`确认取消预约《${it.title}》？`, '', '取消预约', false)) return;
        await apiCall('ToggleReservation', it.seriesId, it.title, '', '', '');
        state.reservationsCache = state.reservationsCache.filter(x => x.seriesId !== it.seriesId);
        renderReservationsList(state.reservationsCache);
        showToast(`已取消预约《${it.title}》`);
      } catch (err) {
        showToast(err);
      } finally {
        button.disabled = false;
      }
    };

    fragment.appendChild(card);
  });
  grid.appendChild(fragment);
}

function getReservationDate(item) {
  if (!item.publishAt) return null;
  const date = new Date(item.publishAt);
  return Number.isNaN(date.getTime()) ? null : date;
}

function formatReservationDate(date) {
  const now = new Date();
  const pad = n => String(n).padStart(2, '0');
  if (date.getFullYear() !== now.getFullYear()) {
    return `${date.getFullYear()}年`;
  }
  const base = `${date.getMonth() + 1}月${date.getDate()}日`;
  return date.getHours() === 0 && date.getMinutes() === 0 ? base : `${base} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

// ==========================================================================
// Search & Suggestions
// ==========================================================================

function initSearch() {
  const searchInput = document.getElementById('search-input');
  const clearBtn = document.getElementById('btn-clear-search');
  const suggsBox = document.getElementById('search-suggestions');

  let debounceTimer = null;
  searchInput.addEventListener('input', () => {
    const q = searchInput.value.trim();
    clearBtn.style.display = q ? 'block' : 'none';

    clearTimeout(debounceTimer);
    activeSuggsIndex = -1;
    if (!q) {
      suggsBox.style.display = 'none';
      return;
    }

    debounceTimer = setTimeout(async () => {
      const suggs = await apiCall('GetSuggestions', q);
      if (suggs && suggs.length > 0) {
        suggsBox.innerHTML = '';
        suggs.forEach(s => {
          const item = document.createElement('div');
          item.className = 'suggestion-item';
          item.textContent = s;
          item.onclick = () => {
            searchInput.value = s;
            suggsBox.style.display = 'none';
            executeSearch(s);
          };
          suggsBox.appendChild(item);
        });
        suggsBox.style.display = 'block';
      } else {
        suggsBox.style.display = 'none';
      }
    }, 200);
  });

  let activeSuggsIndex = -1;
  searchInput.addEventListener('keydown', (e) => {
    const items = suggsBox.querySelectorAll('.suggestion-item');
    if (suggsBox.style.display !== 'none' && items.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        activeSuggsIndex = (activeSuggsIndex + 1) % items.length;
        items.forEach((it, idx) => it.classList.toggle('active', idx === activeSuggsIndex));
        if (items[activeSuggsIndex]) searchInput.value = items[activeSuggsIndex].textContent;
        return;
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        activeSuggsIndex = (activeSuggsIndex - 1 + items.length) % items.length;
        items.forEach((it, idx) => it.classList.toggle('active', idx === activeSuggsIndex));
        if (items[activeSuggsIndex]) searchInput.value = items[activeSuggsIndex].textContent;
        return;
      } else if (e.key === 'Escape') {
        suggsBox.style.display = 'none';
        activeSuggsIndex = -1;
        return;
      }
    }
    if (e.key === 'Enter') {
      suggsBox.style.display = 'none';
      activeSuggsIndex = -1;
      executeSearch(searchInput.value.trim());
    }
  });

  clearBtn.addEventListener('click', () => {
    searchInput.value = '';
    clearBtn.style.display = 'none';
    suggsBox.style.display = 'none';
  });

  document.addEventListener('click', (e) => {
    if (!searchInput.contains(e.target) && !suggsBox.contains(e.target)) {
      suggsBox.style.display = 'none';
    }
  });
}

async function executeSearch(query) {
  if (!query) return;
  navigateTo('search-results');
  document.getElementById('search-query-header').textContent = `搜索: "${query}"`;
  const grid = document.getElementById('search-grid');
  grid.innerHTML = renderDramaGridSkeleton(12);

  try {
    const results = await apiCall('Search', query);
    grid.innerHTML = '';
    if (!results || results.length === 0) {
      grid.appendChild(renderViewEmpty({
        title: '未找到匹配短剧',
        description: `没有找到与 "${query}" 相关的短剧作品，换个关键词试试吧`,
        actionText: '去推荐发现好剧',
        onAction: () => navigateTo('recommend'),
        icon: 'search'
      }));
      return;
    }
    results.forEach(it => {
      grid.appendChild(createDramaCard(it));
    });
  } catch (err) {
    grid.innerHTML = '';
    grid.appendChild(renderViewError({
      title: '搜索短剧失败',
      message: err,
      onRetry: () => executeSearch(query)
    }));
  }
}

// ==========================================================================
// Embedded Video Player Controller (WebView Player)
// ==========================================================================

const video = document.getElementById('main-video');

function syncMainVideoOrientation() {
  syncPlaybackVideoFit(video);
}

function updatePlayerPlayBadge() {
  const centerBadge = document.getElementById('center-play-badge');
  const videoContainer = document.getElementById('video-container');
  const loader = document.getElementById('video-loader');
  if (!centerBadge) return;

  const isBuffering = state.player.loading || (loader && loader.style.display !== 'none');
  if (videoContainer) {
    videoContainer.classList.toggle('is-buffering', Boolean(isBuffering));
    videoContainer.classList.toggle('is-loading', Boolean(state.player.loading));
  }

  // Never show play button when video is buffering, loading, or not user-paused
  if (
    state.player.userPaused &&
    !isBuffering &&
    video &&
    video.src &&
    video.readyState >= 2 &&
    video.paused
  ) {
    centerBadge.style.display = 'flex';
  } else {
    centerBadge.style.display = 'none';
  }
}

function clearPlayerVideo() {
  state.player.loading = true;
  video.pause();
  video.onloadedmetadata = null;
  video.removeAttribute('src');
  video.load();
  resetPlaybackVideoFit(video);
  updatePlayerPlayBadge();
}

function initPlayerControls() {
  const modal = document.getElementById('player-modal');
  const stageWrap = document.getElementById('player-stage');
  const closeBtn = document.getElementById('btn-close-player');
  const playPauseBtn = document.getElementById('player-play');
  const iconPlay = playPauseBtn.querySelector('.icon-play');
  const iconPause = playPauseBtn.querySelector('.icon-pause');
  const prevBtn = document.getElementById('player-prev-ep');
  const nextBtn = document.getElementById('player-next-ep');
  const fullBtn = document.getElementById('player-fullscreen');
  const pipBtn = document.getElementById('player-pip');
  const drawerToggle = document.getElementById('player-drawer-toggle');
  const drawer = document.getElementById('player-playlist-drawer');
  const closeDrawerBtn = document.getElementById('player-close-drawer');
  const speedBtn = document.getElementById('player-speed');
  const speedMenu = document.getElementById('player-speed-menu');
  const qualityBtn = document.getElementById('player-quality');
  const qualityMenu = document.getElementById('player-quality-menu');
  const centerBadge = document.getElementById('center-play-badge');
  const progressBarWrap = document.getElementById('player-progress-wrap');
  const progressCurrent = document.getElementById('player-progress-current');
  const progressBuffer = document.getElementById('player-progress-buffer');
  const timeDisplay = document.getElementById('player-time-display');
  const timeTooltip = document.getElementById('player-time-tooltip');
  const syncPlayerBufferIndicator = () => {
    updatePlaybackBufferIndicator(video, progressBuffer, video.duration);
  };

  // Prevent any modal container accidental horizontal scrolling
  modal.scrollLeft = 0;
  modal.addEventListener('scroll', () => {
    if (modal.scrollLeft !== 0) modal.scrollLeft = 0;
  });
  if (stageWrap) {
    stageWrap.scrollLeft = 0;
    stageWrap.addEventListener('scroll', () => {
      if (stageWrap.scrollLeft !== 0) stageWrap.scrollLeft = 0;
    });
  }
  const playerBody = modal.querySelector('.player-body');
  if (playerBody) {
    playerBody.scrollLeft = 0;
    playerBody.addEventListener('scroll', () => {
      if (playerBody.scrollLeft !== 0) playerBody.scrollLeft = 0;
    });
  }

  let controlsHovered = false;
  const controlsHoverTargets = [
    document.querySelector('#player-stage .feed-rail'),
    document.querySelector('#player-stage .feed-controls')
  ].filter(Boolean);

  const controlsAreLocked = () => (
    video.paused ||
    state.player.loading ||
    document.body.classList.contains('comments-open') ||
    modal.classList.contains('comments-open') ||
    drawer.classList.contains('open') ||
    qualityMenu.style.display !== 'none' ||
    speedMenu.style.display !== 'none'
  );

  const clearControlsTimer = () => {
    clearTimeout(modal.__controlsTimer);
    modal.__controlsTimer = null;
  };

  const showPlayerControls = () => {
    clearControlsTimer();
    modal.classList.remove('controls-hidden');
  };

  const hidePlayerControls = () => {
    clearControlsTimer();
    if (controlsAreLocked()) return;
    modal.classList.add('controls-hidden');
  };

  const schedulePlayerControlsHide = (delay = 3000) => {
    clearControlsTimer();
    if (modal.style.display === 'none' || controlsAreLocked()) return;
    modal.__controlsTimer = setTimeout(() => {
      if (controlsHovered || controlsAreLocked()) return;
      hidePlayerControls();
    }, delay);
  };

  const wakePlayerControls = () => {
    showPlayerControls();
    schedulePlayerControlsHide();
  };

  modal.addEventListener('pointermove', wakePlayerControls);
  modal.addEventListener('pointerleave', schedulePlayerControlsHide);
  controlsHoverTargets.forEach((target) => {
    target.addEventListener('pointerenter', () => {
      controlsHovered = true;
      wakePlayerControls();
    });
    target.addEventListener('pointerleave', () => {
      controlsHovered = false;
      schedulePlayerControlsHide();
    });
  });

  // Close player
  closeBtn.addEventListener('click', closePlayer);

  // Play/Pause
  function togglePlay() {
    if (video.paused) {
      state.player.userPaused = false;
      video.play().catch(e => console.log('play error:', e));
    } else {
      state.player.userPaused = true;
      video.pause();
    }
    updatePlayerPlayBadge();
  }

  if (pipBtn) {
    pipBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      togglePictureInPicture();
    });
  }

  playPauseBtn.addEventListener('click', togglePlay);
  document.getElementById('video-container').addEventListener('click', (e) => {
    if (e.target === video || e.target.id === 'video-container') {
      // One tap on blank video toggles chrome; the play button and Space
      // remain the explicit playback controls.
      if (modal.classList.contains('controls-hidden') || controlsAreLocked()) {
        wakePlayerControls();
      } else {
        hidePlayerControls();
      }
    }
  });

  video.addEventListener('play', () => {
    if (window.ambientGlow) {
      window.ambientGlow.attach(video).start();
    }
    iconPlay.style.display = 'none';
    iconPause.style.display = 'block';
    state.player.userPaused = false;
    updatePlayerPlayBadge();
    wakePlayerControls();
    setPlaybackKeepAwake(true);
    const detail = state.player.seriesDetail || state.detail.data || {};
    updateNowPlayingState({
      title: detail.title || state.player.seriesTitle || '红果短剧',
      series: detail.title || state.player.seriesTitle || '红果短剧',
      episode: state.player.episodeNum || 1,
      duration: video.duration || 0,
      position: video.currentTime || 0,
      playing: true,
      rate: video.playbackRate || 1.0,
      coverUrl: detail.coverUrl || state.player.seriesCover || ''
    });
  });

  video.addEventListener('pause', () => {
    if (window.ambientGlow) {
      window.ambientGlow.pause();
    }
    iconPlay.style.display = 'block';
    iconPause.style.display = 'none';
    updatePlayerPlayBadge();
    setPlaybackKeepAwake(false);
    const detail = state.player.seriesDetail || state.detail.data || {};
    updateNowPlayingState({
      title: detail.title || state.player.seriesTitle || '红果短剧',
      series: detail.title || state.player.seriesTitle || '红果短剧',
      episode: state.player.episodeNum || 1,
      duration: video.duration || 0,
      position: video.currentTime || 0,
      playing: false,
      rate: video.playbackRate || 1.0,
      coverUrl: detail.coverUrl || state.player.seriesCover || ''
    });
    wakePlayerControls();
    persistProgress();
  });

  video.addEventListener('seeked', () => {
    const detail = state.player.seriesDetail || state.detail.data || {};
    updateNowPlayingState({
      title: detail.title || state.player.seriesTitle || '红果短剧',
      series: detail.title || state.player.seriesTitle || '红果短剧',
      episode: state.player.episodeNum || 1,
      duration: video.duration || 0,
      position: video.currentTime || 0,
      playing: !video.paused,
      rate: video.playbackRate || 1.0,
      coverUrl: detail.coverUrl || state.player.seriesCover || ''
    });
  });

  video.addEventListener('loadedmetadata', syncMainVideoOrientation);
  attachPlaybackVideoFit(video);

  video.addEventListener('waiting', () => {
    if (window.ambientGlow) {
      window.ambientGlow.pause();
    }
    document.getElementById('video-loader').style.display = 'grid';
    document.getElementById('video-container')?.classList.add('is-buffering');
    updatePlayerPlayBadge();
  });

  video.addEventListener('error', () => {
    if (!state.player.loading) return;
    const attempt = (state.player.loadRetries || 0) + 1;
    if (attempt > 2) {
      state.player.loading = false;
      state.player.loadRetries = 0;
      document.getElementById('video-loader').style.display = 'none';
      document.getElementById('video-container')?.classList.remove('is-buffering', 'is-loading');
      updatePlayerPlayBadge();
      showToast('播放失败，请稍后重试');
      return;
    }
    state.player.loadRetries = attempt;
    showToast(`网络波动，自动重试 ${attempt}/2`);
    setTimeout(() => {
      if (!state.player.loading) return;
      playEpisodeNumber(state.player.episodeNum, state.player.quality, video.currentTime || null, !state.player.userPaused);
    }, 800);
  });

  video.addEventListener('playing', () => {
    state.player.loading = false;
    state.player.loadRetries = 0;
    if (window.ambientGlow) {
      window.ambientGlow.attach(video).start();
    }
    document.getElementById('video-loader').style.display = 'none';
    document.getElementById('video-container')?.classList.remove('is-buffering', 'is-loading');
    updatePlayerPlayBadge();
  });

  video.addEventListener('canplay', updatePlayerPlayBadge);
  video.addEventListener('loadeddata', updatePlayerPlayBadge);

  // Time & progress update
  video.addEventListener('timeupdate', () => {
    if (video.duration) {
      const pct = (video.currentTime / video.duration) * 100;
      progressCurrent.style.width = pct + '%';
      timeDisplay.textContent = `${formatTime(video.currentTime)} / ${formatTime(video.duration)}`;
    }
  });

  ['durationchange', 'loadedmetadata', 'loadeddata', 'canplay', 'canplaythrough', 'progress', 'seeking', 'seeked', 'timeupdate'].forEach(event => {
    video.addEventListener(event, syncPlayerBufferIndicator);
  });

  ['emptied', 'loadstart'].forEach(event => {
    video.addEventListener(event, () => updatePlaybackBufferIndicator(video, progressBuffer, 0));
  });

  ['enterpictureinpicture', 'leavepictureinpicture', 'webkitpresentationmodechanged'].forEach(evt => {
    video.addEventListener(evt, updatePiPButtons);
  });

  // Seek on click
  progressBarWrap.addEventListener('click', (e) => {
    const rect = progressBarWrap.getBoundingClientRect();
    const pos = (e.clientX - rect.left) / rect.width;
    if (video.duration) {
      video.currentTime = pos * video.duration;
    }
  });

  progressBarWrap.addEventListener('mousemove', (e) => {
    if (!video.duration || !timeTooltip) return;
    const rect = progressBarWrap.getBoundingClientRect();
    const pos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    timeTooltip.textContent = formatTime(pos * video.duration);
    timeTooltip.style.left = `${pos * 100}%`;
    timeTooltip.style.display = 'block';
  });

  progressBarWrap.addEventListener('mouseleave', () => {
    if (timeTooltip) timeTooltip.style.display = 'none';
  });

  // Next / Prev episode
  nextBtn.addEventListener('click', () => {
    if (state.player.episodeNum < state.player.totalEpisodes) {
      playEpisodeNumber(state.player.episodeNum + 1);
    } else if (state.player.seriesId) {
      playNextSeason(state.player.seriesId, 'player').then(started => {
        if (!started && state.player.seriesId) showToast('已经是最后一集');
      });
    } else {
      showToast('已经是最后一集');
    }
  });

  prevBtn.addEventListener('click', () => {
    if (state.player.episodeNum > 1) {
      playEpisodeNumber(state.player.episodeNum - 1);
    } else {
      showToast('已经是第一集');
    }
  });

  // Autoplay next episode on video end
  video.addEventListener('ended', () => {
    if (window.ambientGlow) {
      window.ambientGlow.stop();
    }
    persistProgress();
    if (state.player.episodeNum < state.player.totalEpisodes) {
      showToast(`正在自动连播第 ${state.player.episodeNum + 1} 集...`);
      playEpisodeNumber(state.player.episodeNum + 1);
    } else {
      const seriesId = state.player.seriesId;
      playNextSeason(seriesId, 'player').then(started => {
        if (!started && state.player.seriesId === seriesId) showToast('全剧播放完毕');
      });
    }
  });

  // Speed selection
  speedBtn.addEventListener('click', () => {
    speedMenu.style.display = speedMenu.style.display === 'none' ? 'block' : 'none';
  });

  speedMenu.querySelectorAll('.speed-opt').forEach(opt => {
    opt.addEventListener('click', () => {
      const spd = parseFloat(opt.dataset.speed);
      state.player.playbackSpeed = spd;
      video.playbackRate = spd;
      speedBtn.textContent = spd + 'x';
      speedMenu.querySelectorAll('.speed-opt').forEach(o => o.classList.remove('active'));
      opt.classList.add('active');
      speedMenu.style.display = 'none';
      wakePlayerControls();
    });
  });

  // Quality selection
  if (qualityBtn && qualityMenu) {
    qualityBtn.addEventListener('click', () => {
      const isOpen = qualityMenu.style.display === 'none';
      qualityMenu.style.display = isOpen ? 'block' : 'none';
      qualityBtn.setAttribute('aria-expanded', String(isOpen));
    });

    document.addEventListener('click', (event) => {
      if (!qualityBtn.contains(event.target) && !qualityMenu.contains(event.target)) {
        qualityMenu.style.display = 'none';
        qualityBtn.setAttribute('aria-expanded', 'false');
        wakePlayerControls();
      }
    });
  }

  // Fullscreen contains only the video stage and its controls. The browser
  // hides sibling player chrome (topbar and playlist drawer) natively; the
  // class fallback covers WebViews where Element Fullscreen is unavailable.
  function togglePlayerFullscreen() {
    if (!stageWrap) return;
    if (document.fullscreenElement) {
      stageWrap.classList.remove('player-fullscreen');
      document.exitFullscreen().catch(err => console.log(err));
      return;
    }

    const enterPseudoFullscreen = () => stageWrap.classList.add('player-fullscreen');
    if (stageWrap.requestFullscreen) {
      stageWrap.requestFullscreen()
        .then(enterPseudoFullscreen)
        .catch(err => {
          console.log(err);
          enterPseudoFullscreen();
        });
    } else {
      enterPseudoFullscreen();
    }
  }

  document.addEventListener('fullscreenchange', () => {
    if (!document.fullscreenElement) {
      stageWrap.classList.remove('player-fullscreen');
    }
    wakePlayerControls();
  });

  fullBtn.addEventListener('click', togglePlayerFullscreen);

  if (closeDrawerBtn) {
    closeDrawerBtn.addEventListener('click', () => {
      drawer.classList.remove('open');
      drawerToggle.setAttribute('aria-expanded', 'false');
    });
  }

  // Toggle Drawer
  drawerToggle.addEventListener('click', () => {
    const willOpen = !drawer.classList.contains('open');
    drawer.classList.toggle('open', willOpen);
    drawerToggle.setAttribute('aria-expanded', String(willOpen));
    if (willOpen) {
      scrollActiveDrawerEpisode('player-drawer-grid', state.player.episodeNum);
    }
    wakePlayerControls();
  });

  // Favorite toggle
  const favBtn = document.getElementById('player-fav');
  const commentBtn = document.getElementById('player-comment');
  favBtn.addEventListener('click', async () => {
    try {
      const isNowFav = await apiCall('ToggleFavorite', state.player.seriesId, state.player.seriesTitle, state.player.seriesCover, '', state.player.totalEpisodes);
      state.player.isFavorite = isNowFav;
      favBtn.classList.toggle('active', isNowFav);
      showToast(isNowFav ? '已加入收藏' : '已取消收藏');
      state.favoritesView.favoritesList = [];
    } catch (err) {
      showToast('收藏失败: ' + err);
    }
  });

  // Like toggle
  const likeBtn = document.getElementById('player-like');
  likeBtn.addEventListener('click', async () => {
    try {
      const isNowLiked = await apiCall('ToggleLike', state.player.seriesId, state.player.episodeId, state.player.seriesTitle, state.player.seriesCover, state.player.episodeNum);
      state.player.isLiked = isNowLiked;
      likeBtn.classList.toggle('active', isNowLiked);
      showToast(isNowLiked ? '已点赞' : '已取消点赞');
      state.favoritesView.likesList = [];
    } catch (err) {
      showToast('点赞失败: ' + err);
    }
  });

  commentBtn.addEventListener('click', () => {
    openCommentSheet({
      source: 'player',
      seriesId: state.player.seriesId,
      episodeId: state.player.episodeId,
      episodeNum: state.player.episodeNum,
      title: state.player.seriesTitle,
      count: state.player.commentCount
    });
  });

  // Keyboard shortcuts
  document.addEventListener('keydown', (e) => {
    if (modal.style.display === 'none') return;
    const targetTag = e.target.tagName;
    if (targetTag === 'INPUT' || targetTag === 'TEXTAREA' || e.target.isContentEditable) return;

    switch (e.code) {
      case 'Space':
        e.preventDefault();
        togglePlay();
        break;
      case 'ArrowLeft':
        e.preventDefault();
        video.currentTime = Math.max(0, video.currentTime - 5);
        showToast('-5s');
        break;
      case 'ArrowRight':
        e.preventDefault();
        video.currentTime = Math.min(video.duration || 0, video.currentTime + 5);
        showToast('+5s');
        break;
      case 'ArrowUp':
        e.preventDefault();
        video.volume = Math.min(1, video.volume + 0.1);
        showToast(`音量: ${Math.round(video.volume * 100)}%`);
        break;
      case 'ArrowDown':
        e.preventDefault();
        video.volume = Math.max(0, video.volume - 0.1);
        showToast(`音量: ${Math.round(video.volume * 100)}%`);
        break;
      case 'KeyF':
        e.preventDefault();
        fullBtn.click();
        break;
      case 'KeyP':
        e.preventDefault();
        togglePictureInPicture();
        break;
      case 'Escape':
        e.preventDefault();
        if (drawer.classList.contains('open')) {
          drawer.classList.remove('open');
          drawerToggle.setAttribute('aria-expanded', 'false');
          return;
        }
        if (document.fullscreenElement) {
          document.exitFullscreen().catch(err => console.log(err));
          return;
        }
        closePlayer();
        break;
    }
  });

  // Auto-save progress every 5 seconds
  setInterval(() => {
    if (modal.style.display !== 'none' && !video.paused) {
      persistProgress();
    }
  }, 5000);
}

async function playNextSeason(seriesId, source = 'player') {
  if (!seriesId || state.player.seasonSwitching) return false;
  state.player.seasonSwitching = true;

  try {
    const next = await apiCall('GetNextSeason', seriesId);
    if (!next?.id || (source === 'player' && state.player.seriesId !== seriesId)) return false;

    const title = next.title || '下一季';
    if (source === 'feed') {
      showFeedHint(`正在自动播放下一季：${title}`, true, 2600);
      enterSeriesPlayer(next.id, 1);
    } else {
      showToast(`正在自动播放下一季：${title}`);
      await openPlayer(next.id, 1);
    }
    return true;
  } catch (err) {
    console.warn('load next season failed:', err);
    return false;
  } finally {
    state.player.seasonSwitching = false;
  }
}

async function openPlayer(seriesId, initialEpisodeNum = 1) {
  // Defensive guard: exit any active fullscreen outside the player stage
  if (document.fullscreenElement && document.fullscreenElement !== document.getElementById('player-stage')) {
    try {
      await document.exitFullscreen();
    } catch (err) {
      console.warn('exitFullscreen failed in openPlayer:', err);
    }
  }

  if (window.ambientGlow) {
    window.ambientGlow.stop();
  }

  // Pause recommend feed video if active
  pauseFeedPlayback();
  clearCommentSceneLayout();

  state.player.seriesId = seriesId;
  state.player.episodeNum = initialEpisodeNum;
  state.player.loading = true;
  state.player.seasonSwitching = false;
  state.player.userPaused = false;

  const modal = document.getElementById('player-modal');
  modal.style.display = 'flex';
  modal.classList.remove('controls-hidden');
  modal.scrollLeft = 0;
  const stage = document.getElementById('player-stage');
  if (stage) stage.scrollLeft = 0;
  const playerBody = modal.querySelector('.player-body');
  if (playerBody) playerBody.scrollLeft = 0;
  document.documentElement.scrollLeft = 0;
  document.body.scrollLeft = 0;
  clearTimeout(modal.__controlsTimer);
  modal.__controlsTimer = null;
  syncFeedScrollCapture();
  document.getElementById('video-loader').style.display = 'grid';
  document.getElementById('video-container')?.classList.add('is-loading', 'is-buffering');
  updatePlayerPlayBadge();

  clearPlayerVideo();

  try {
    // 1. Fetch series detail and episode list
    const detail = state.detail.data && state.detail.data.id === seriesId
      ? state.detail.data
      : await apiCall('GetSeriesDetail', seriesId);
    if (!detail) {
      showToast('获取短剧详情失败');
      closePlayer();
      return;
    }

    state.player.seriesTitle = detail.title;
    state.player.seriesCover = detail.coverUrl;
    state.player.totalEpisodes = detail.episodeCount || detail.episodes.length;
    state.player.episodes = detail.episodes || [];
    state.player.isFavorite = detail.isFavorite;
    state.player.qualityOptions = fallbackQualityOptions(state.player.quality);
    renderQualityMenu();

    document.getElementById('player-series-title').textContent = detail.title;
    document.getElementById('player-drawer-count').textContent = `共 ${state.player.totalEpisodes} 集`;
    document.getElementById('player-fav').classList.toggle('active', detail.isFavorite);
    updatePlayerCommentCount(0);

    // Render episode playlist drawer
    renderPlaylistDrawer();

    // 2. Play initial episode
    await playEpisodeNumber(initialEpisodeNum);
  } catch (err) {
    showToast('打开播放器失败: ' + err);
    closePlayer();
  }
}

function renderQualityMenu() {
  const qualityBtn = document.getElementById('player-quality');
  const qualityMenu = document.getElementById('player-quality-menu');
  if (!qualityBtn || !qualityMenu) return;

  const active = state.player.qualityOptions.find(opt => opt.active) || { label: qualityLabel(state.player.quality) };
  qualityBtn.textContent = active.label;
  qualityMenu.innerHTML = '';
  state.player.qualityOptions.forEach(opt => {
    const item = document.createElement('div');
    item.className = `quality-opt ${opt.active ? 'active' : ''} ${opt.available ? '' : 'disabled'}`;
    item.textContent = opt.label;
    if (opt.available) {
      item.addEventListener('click', () => {
        qualityMenu.style.display = 'none';
        qualityBtn.setAttribute('aria-expanded', 'false');
        changePlaybackQuality(opt.id);
      });
    }
    qualityMenu.appendChild(item);
  });
}

function updateQualityControls(playback) {
  if (playback.quality) state.player.quality = playback.quality;
  if (Array.isArray(playback.qualities) && playback.qualities.length > 0) {
    state.player.qualityOptions = playback.qualities;
  } else {
    state.player.qualityOptions = fallbackQualityOptions(state.player.quality);
  }
  renderQualityMenu();
}

function fallbackQualityOptions(selected) {
  return ['1080p', '720p', '540p', '480p', '360p'].map(quality => ({
    id: quality,
    label: qualityLabel(quality),
    available: quality === selected,
    active: quality === selected
  }));
}

function qualityLabel(quality) {
  return String(quality || '720p').toUpperCase();
}

async function changePlaybackQuality(quality) {
  if (!quality || quality === state.player.quality) return;

  const currentTime = video.currentTime || 0;
  const wasPlaying = !video.paused;
  const success = await playEpisodeNumber(state.player.episodeNum, quality, currentTime, wasPlaying);
  if (success) {
    const selected = state.player.qualityOptions.find(opt => opt.active) || { label: qualityLabel(state.player.quality) };
    showToast(`已切换到 ${selected.label}`);
  }
}

async function playEpisodeNumber(num, requestedQuality = state.player.quality, resumeAt = null, autoPlay = true) {
  state.player.episodeNum = num;
  state.player.loading = true;
  state.player.userPaused = !autoPlay;
  document.getElementById('player-episode-badge').textContent = `第 ${num} 集`;

  // Find episode ID
  let targetEp = state.player.episodes.find(e => e.number === num);
  const epId = targetEp ? targetEp.id : '';
  state.player.episodeId = epId;
  updatePlayerCommentCount(targetEp ? targetEp.commentCount : 0);

  // Highlight in drawer
  document.querySelectorAll('#player-drawer-grid .ep-btn').forEach(btn => {
    btn.classList.toggle('active', parseInt(btn.dataset.num) === num);
  });

  // Scroll active episode button into view safely within the grid only
  scrollActiveDrawerEpisode('player-drawer-grid', num);

  document.getElementById('video-loader').style.display = 'grid';
  document.getElementById('video-container')?.classList.add('is-loading', 'is-buffering');
  updatePlayerPlayBadge();

  // Drop the previous media immediately so episode switches cannot leave the
  // old decoded frame visible (or its stream connected) while the next one loads.
  clearPlayerVideo();

  try {
    const pb = await apiCall('PlayEpisode', state.player.seriesId, epId, num, requestedQuality, false);
    if (!pb || !pb.streamUrl) {
      showToast('获取播放地址失败');
      return false;
    }

    document.getElementById('player-like').classList.toggle('active', pb.isLiked);
    if (Number(pb.commentCount) > 0) {
      updatePlayerCommentCount(pb.commentCount);
    }
    updateQualityControls(pb);

    video.crossOrigin = 'anonymous';
    video.src = pb.streamUrl;
    video.load();

    // Auto-resume from last progress if available
    video.onloadedmetadata = () => {
      syncMainVideoOrientation();
      if (resumeAt !== null) {
        video.currentTime = Math.min(resumeAt, video.duration || resumeAt);
      } else if (pb.lastPosition > 3 && pb.lastPosition < (video.duration - 5)) {
        video.currentTime = pb.lastPosition;
        showToast(`已为您自动跳转到上次播放位置 ${formatTime(pb.lastPosition)}`);
      }
      video.playbackRate = state.player.playbackSpeed;
      if (autoPlay) {
        state.player.userPaused = false;
        video.play().catch(e => console.log('play error:', e));
      } else {
        state.player.loading = false;
        state.player.userPaused = true;
        document.getElementById('video-loader').style.display = 'none';
        document.getElementById('video-container')?.classList.remove('is-buffering', 'is-loading');
        updatePlayerPlayBadge();
      }
    };

    return true;
  } catch (err) {
    state.player.loading = false;
    document.getElementById('video-loader').style.display = 'none';
    document.getElementById('video-container')?.classList.remove('is-buffering', 'is-loading');
    updatePlayerPlayBadge();
    showToast('播放失败: ' + err);
    return false;
  }
}

function renderPlaylistDrawer() {
  renderEpisodeGrid({
    grid: document.getElementById('player-drawer-grid'),
    count: document.getElementById('player-drawer-count'),
    total: state.player.totalEpisodes || state.player.episodes.length || 1,
    current: state.player.episodeNum,
    onSelect: playEpisodeNumber
  });
}

function persistProgress() {
  if (!state.player.seriesId || !video.duration) return;
  apiCall('SaveProgress',
    state.player.seriesId,
    state.player.episodeId,
    state.player.seriesTitle,
    state.player.seriesCover,
    state.player.episodeNum,
    state.player.totalEpisodes,
    video.currentTime,
    video.duration
  );
}

function closePlayer() {
  if (window.ambientGlow) {
    window.ambientGlow.stop();
  }
  const playerModal = document.getElementById('player-modal');
  clearTimeout(playerModal?.__controlsTimer);
  if (playerModal) playerModal.__controlsTimer = null;
  setPlaybackKeepAwake(false);
  clearNowPlayingState();
  persistProgress();
  if (commentState.open) closeCommentSheet();
  // A stale scene class can leave the feed stage narrowed after returning.
  // Clear it unconditionally here because the whole player is going away.
  const commentOverlay = document.getElementById('comment-sheet-overlay');
  if (commentOverlay) commentOverlay.style.display = 'none';
  restoreCommentOverlay();
  document.body.classList.remove('comments-open');
  danmakuControllers.player?.reset();
  const playerComposer = document.getElementById('player-danmaku-composer');
  if (playerComposer) playerComposer.classList.remove('open');
  document.getElementById('player-modal')?.classList.remove('comments-open');
  state.player.loading = false;
  state.player.userPaused = false;
  video.pause();
  video.removeAttribute('src');
  video.load();
  resetPlaybackVideoFit(video);
  updatePlayerPlayBadge();
  document.getElementById('video-container')?.classList.remove('is-buffering', 'is-loading');
  document.getElementById('player-stage')?.classList.remove('player-fullscreen');
  document.getElementById('player-playlist-drawer')?.classList.remove('open');
  playerModal?.classList.remove('controls-hidden');
  document.getElementById('player-modal').style.display = 'none';
  if (playerModal) playerModal.scrollLeft = 0;
  const stage = document.getElementById('player-stage');
  if (stage) stage.scrollLeft = 0;
  const playerBody = playerModal?.querySelector('.player-body');
  if (playerBody) playerBody.scrollLeft = 0;
  document.documentElement.scrollLeft = 0;
  document.body.scrollLeft = 0;
  syncFeedScrollCapture();
  if (document.fullscreenElement) {
    document.exitFullscreen().catch(err => console.log(err));
  }
  // Resume recommend feed video if on recommend view
  if (state.currentView === 'recommend') {
    state.feed.userPaused = false;
    resumeFeedPlayback();
  }
  // Refresh history view if active
  if (state.currentView === 'history') {
    loadHistory();
  }
}

function formatTime(sec) {
  if (!Number.isFinite(Number(sec)) || Number(sec) <= 0) return '00:00';
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}
