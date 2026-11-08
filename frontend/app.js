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
let nativeFeedScrollReady = false;
let feedLastFallbackScrollAt = -Infinity;
let feedScrollCaptureSync = null;
let feedWarmupToken = 0;
let feedWarmupSession = null;
let feedWarmupCenterKey = '';
let feedPreloadRevision = Date.now();
let feedPlaybackGeneration = 0;
let playerRequestToken = 0;
let playerPlaybackIdentity = null;
const feedSwitchMetrics = [];
const feedMetricQueue = [];
let feedMetricFlushTimer = null;
let feedMetricFlushInFlight = false;
let feedTimeSyncHandle = 0;
let feedTimeSyncHandleType = '';
let feedTimeSyncVideo = null;
let feedTimeSyncLastAt = 0;
let feedTimeSyncGeneration = 0;
let feedBufferingTimer = null;
let feedInputSequence = 0;
let feedLastRepeatAt = -Infinity;
const feedDiagnosticSession = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;

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
  requestFeedDramaMetadata(slot);
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
        if (slot) slot.durationSec = Math.max(slot.durationSec, video.duration);
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
      requestedQuality: '',
      streamUrl: '',
      durationSec: 0,
      metadataDurationSec: 0,
      completeCached: false,
      mediaWaiting: false,
      failed: false,
      metadataRequested: false,
      progressiveMedia: null,
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
      metricSourceAssignedAt: 0,
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
      recordFeedDiagnostic('media-data-ready', { toIndex: slot.itemIndex }, slot);
    }
  });
  ['canplay', 'canplaythrough', 'loadeddata', 'progress'].forEach(event => {
    slot.video.addEventListener(event, () => {
      if (slot.metricPreparedAt && !slot.metricReadyAt && feedVideoHasPlaybackBuffer(slot.video)) {
        slot.metricReadyAt = performance.now();
        recordFeedDiagnostic('media-buffer-ready', { toIndex: slot.itemIndex }, slot);
      }
    });
  });
  slot.video.addEventListener('waiting', () => {
    const wasWaiting = slot.mediaWaiting;
    slot.mediaWaiting = true;
    if (!wasWaiting && feedSlots[feedSlotCurr] === slot) recordFeedDiagnostic('media-waiting');
    if (!slot.metricPreparedAt || slot.metricRebufferStartedAt) return;
    if (slot.isPriming) return;
    slot.metricRebufferStartedAt = performance.now();
  });
  slot.video.addEventListener('playing', () => {
    if (slot.mediaWaiting && slot.metricPlayingAt && feedSlots[feedSlotCurr] === slot) {
      recordFeedDiagnostic('media-resumed');
    }
    slot.mediaWaiting = false;
    if (!slot.metricPreparedAt) return;
    if (!slot.metricFirstFrameAt) slot.metricFirstFrameAt = performance.now();
    if (!slot.metricPlayingAt) {
      slot.metricPlayingAt = performance.now();
      recordFeedDiagnostic('media-playing', { toIndex: slot.itemIndex }, slot);
    }
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

  let finish;
  let frameTimer;
  const firstFrame = new Promise(resolve => {
    finish = () => {
      clearTimeout(frameTimer);
      video.removeEventListener('loadeddata', finish);
      video.removeEventListener('playing', finish);
      resolve();
    };
    video.addEventListener('loadeddata', finish);
    video.addEventListener('playing', finish);
    frameTimer = setTimeout(finish, 900);
  });

  try {
    // WebKit may leave play() pending for an offscreen element. Bound frame
    // priming independently so promotion can always retry playback.
    Promise.resolve(video.play()).catch(finish);
    await firstFrame;
  } catch (_) {
    // Some WebKit versions defer loading offscreen videos until play is called;
    // failing here is safe because the cover remains the fallback.
  } finally {
    finish();
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
    if (!slot.metricFirstFrameAt && feedVideoHasFirstFrame(slot.video)) {
      slot.metricFirstFrameAt = performance.now();
      slot.metricFirstFrameSource = 'rendered-frame';
    }
    if (!slot.metricRenderedAt && feedVideoHasFirstFrame(slot.video)) {
      slot.metricRenderedAt = performance.now();
      recordFeedDiagnostic('media-rendered', { toIndex: slot.itemIndex }, slot);
    }
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
  const slot = feedSlotForVideo(video);
  const metadataDuration = Number(slot?.durationSec || 0);
  const mediaDuration = Number.isFinite(video?.duration) && video.duration > 0 ? video.duration : 0;
  // A growing HLS playlist can report only the duration published so far.
  // Keep the real episode duration as the buffer/progress denominator.
  return Math.max(metadataDuration, mediaDuration);
}

function feedSlotIsFullyBuffered(slot) {
  if (!slot?.video?.src) return false;
  if (slot.completeCached) return true;
  // A growing HLS playlist's finite duration is not the episode duration.
  const duration = Number(slot.metadataDurationSec) ||
    (slot.hlsStreamUrl ? 0 : feedMediaDuration(slot.video));
  const ranges = slot.video.buffered;
  if (!(duration > 0) || !ranges?.length || ranges.start(0) > 0.25) return false;
  let end = ranges.end(0);
  for (let i = 1; i < ranges.length; i++) {
    if (ranges.start(i) > end + 0.25) return false;
    end = ranges.end(i);
  }
  return end >= duration - 0.25;
}

function feedQualityForItem(itemIndex) {
  const desired = feedDesiredQuality();
  if (state.feed.userQuality) return desired;
  const retained = state.feed.retainedQualities?.get(state.feed.items[itemIndex]?.id);
  return retained && !state.feed.blockedQualities?.includes(retained) ? retained : desired;
}

function rememberFeedQuality(seriesId, quality) {
  if (!state.feed.retainedQualities) state.feed.retainedQualities = new Map();
  const qualities = state.feed.retainedQualities;
  qualities.delete(seriesId);
  qualities.set(seriesId, quality);
  if (qualities.size > 128) qualities.delete(qualities.keys().next().value);
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
  slot.progressiveMedia?.dispose();
  slot.progressiveMedia = null;
  slot.loadToken++;
  slot.preparing = null;
  slot.itemIndex = -1;
  slot.seriesId = '';
  slot.episodeNum = 1;
  slot.episodeId = '';
  slot.quality = '';
  slot.requestedQuality = '';
  slot.durationSec = 0;
  slot.metadataDurationSec = 0;
  slot.completeCached = false;
  slot.mediaWaiting = false;
  slot.failed = false;
  slot.metadataRequested = false;
  slot.queueConsumed = false;
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
  slot.metricSourceAssignedAt = 0;
  slot.metricFirstFrameAt = 0;
  slot.metricPlayingAt = 0;
  slot.metricRenderedAt = 0;
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
    (slot.expiresAt > Date.now() || feedVideoHasFirstFrame(slot.video)) &&
    slot.itemIndex === itemIndex &&
    slot.seriesId === seriesId &&
    (slot.quality === quality || slot.requestedQuality === quality) &&
    slot.streamUrl &&
    slot.video?.src &&
    !slot.video.error &&
    (normalizeFeedMediaURL(slot.streamUrl) === normalizeFeedMediaURL(slot.video.src) ||
      (slot.progressiveMedia?.url === slot.streamUrl && slot.progressiveMedia?.objectURL === slot.video.src))
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
  return video?.closest('.feed-slide-layer') || video?.parentElement || null;
}

function syncPlaybackVideosFit() {
  document.querySelectorAll('.feed-video').forEach(syncPlaybackVideoFit);
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
  return state.currentView === 'recommend' && !drawer?.classList.contains('open') && !commentState.open;
}

function syncFeedScrollCapture() {
  if (feedScrollCaptureSync) return feedScrollCaptureSync;
  const enabled = shouldCaptureFeedScroll();
  // Serialize updates so a slow "disable" cannot arrive after a later
  // "enable" and leave native scrolling permanently switched off.
  feedScrollCaptureSync = apiCall('SetFeedScrollCapture', enabled).then(ready => {
    nativeFeedScrollReady = ready === true;
  }).catch(() => { nativeFeedScrollReady = false; }).finally(() => {
    feedScrollCaptureSync = null;
    if (enabled !== shouldCaptureFeedScroll()) return syncFeedScrollCapture();
  });
  return feedScrollCaptureSync;
}

function handleNativeFeedScroll(payload) {
  const wasReady = nativeFeedScrollReady;
  nativeFeedScrollReady = true;
  if (!shouldCaptureFeedScroll()) return;
  // During native monitor startup the DOM fallback can receive the same
  // gesture first. Hand ownership over without navigating twice.
  if (!wasReady && Date.now() - feedLastFallbackScrollAt < 300) return;
  const direction = String(payload?.direction || '').toLowerCase();
  if (direction === 'down') requestFeedSwitch(1, 'native-scroll');
  if (direction === 'up') requestFeedSwitch(-1, 'native-scroll');
}

function resetFeedVideoPool() {
  feedPlayToken++;
  cancelPendingFeedSwitch();
  feedAnimationToken++;
  isFeedAnimating = false;
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

function assignFeedMediaSource(slot, source, progressive = false) {
  slot.progressiveMedia?.dispose();
  slot.progressiveMedia = null;
  slot.streamUrl = source;
  if (progressive && window.ProgressiveMediaSource?.isSupported()) {
    const token = slot.loadToken;
    slot.progressiveMedia = new window.ProgressiveMediaSource(slot.video, source, {
      duration: slot.metadataDurationSec,
      onError: error => {
        if (slot.loadToken === token) slot.video.onerror?.(error);
      },
      onComplete: () => {
        if (slot.loadToken === token) slot.completeCached = true;
      }
    });
  } else {
    slot.video.src = source;
    slot.video.load();
  }
}

function hasWarmFeedHLS(playback) {
  if (playback.isHlsCached) return true;
  const key = `${playback.episodeId}:${playback.quality}`;
  const status = feedWarmupSession?.statuses.get(key);
  return Boolean(status?.active || status?.complete || status?.cachedSeconds > 0 ||
    feedWarmupSession?.pendingMedia?.has(key));
}

async function prepareFeedSlot(slot, itemIndex, autoPlay = false, resumeAt = null, primeFirstFrame = false, quality = null, episodeNum = 1, episodeId = '') {
  if (!slot || !slot.video) return null;
  if (itemIndex < 0 || itemIndex >= state.feed.items.length) {
    clearFeedSlot(slot);
    return null;
  }

  const drama = state.feed.items[itemIndex];
  if (!drama) return null;
  const requestedQuality = quality || feedQualityForItem(itemIndex);
  const requestedKey = feedAssetKey(drama.id, episodeNum, requestedQuality);

  if (slot.preparing && slot.itemIndex === itemIndex && slot.seriesId === drama.id &&
      (slot.quality === requestedQuality || slot.requestedQuality === requestedQuality)) {
    const loadToken = slot.loadToken;
    await slot.preparing;
    if (loadToken !== slot.loadToken || !slot.ready) return null;
  }

  if (feedSlotMatches(slot, itemIndex, drama.id, requestedQuality)) {
    syncActiveFeedSlot(slot);
    requestFeedDramaMetadata(slot);
    state.feed.consecutiveSourceFailures = 0;
    if (primeFirstFrame && !feedVideoHasFirstFrame(slot.video) && !slot.firstFramePriming) {
      slot.firstFramePriming = primeFeedSlotFirstFrame(slot);
    }
    if (autoPlay && state.currentView === 'recommend' && !state.feed.userPaused &&
        feedSlots[feedSlotCurr] === slot && slot.video.paused !== false) {
      beginFeedPlayback(slot.video, feedPlayToken);
    }
    return null;
  }

  clearFeedSlot(slot);
  const loadToken = slot.loadToken;
  slot.metricPreparedAt = performance.now();
  slot.itemIndex = itemIndex;
  slot.seriesId = drama.id;
  slot.episodeNum = episodeNum;
  slot.quality = requestedQuality;
  slot.requestedQuality = requestedQuality;
  slot.video.dataset.seriesId = String(drama.id);
    slot.video.dataset.episodeNum = String(episodeNum);

  if (slot.poster) {
    slot.poster.style.backgroundImage = safeImageCSS(drama.coverUrl);
    slot.poster.classList.remove('hidden');
  }
  if (feedSlots[feedSlotCurr] === slot) updateFeedPlayIndicator();

  const loadPlayback = async () => {
    // Metadata prefetch and the visible slot share one load. Warm slots keep
    // their HLS rendition; a foreground cold start can consume MP4 fragments
    // before the next source keyframe closes a complete HLS segment.
    const pb = await prefetchFeedPlaybackInfo(itemIndex, true, requestedQuality, episodeNum, episodeId);
    if (loadToken !== slot.loadToken) return;
    if (!pb || !pb.streamUrl) {
      if (feedSlots[feedSlotCurr] === slot) reportFeedFailure(slot, '获取播放流失败');
      return;
    }

    const selectedQuality = pb.quality || requestedQuality;
    rememberFeedQuality(drama.id, selectedQuality);
    rememberFeedPlayback(requestedKey, pb);
    rememberFeedPlayback(feedAssetKey(drama.id, episodeNum, selectedQuality), pb);

    slot.episodeId = pb.episodeId || '';
    slot.episodeNum = pb.episodeNum || 1;
    slot.quality = selectedQuality;
    slot.durationSec = Number(pb.durationSec) || 0;
    slot.metadataDurationSec = slot.durationSec;
    slot.completeCached = Boolean(pb.isFullCached || pb.isHlsCached);
    slot.fullStreamUrl = pb.fullStreamUrl || pb.streamUrl;
    slot.progressiveStreamUrl = pb.progressiveStreamUrl || '';
    slot.hlsStreamUrl = pb.hlsStreamUrl || '';
    slot.streamUrl = pb.hlsStreamUrl || pb.streamUrl;
    if (pb.isFullCached) slot.streamUrl = slot.fullStreamUrl;
    const foreground = (state.currentView === 'recommend' && feedSlots[feedSlotCurr] === slot && state.feed.index === itemIndex) ||
      (feedSwitchWaitDir && itemIndex === state.feed.index + feedSwitchWaitDir);
    const progressive = foreground && !pb.isFullCached && !hasWarmFeedHLS(pb) && pb.progressiveStreamUrl &&
      window.ProgressiveMediaSource?.isSupported();
    if (progressive) slot.streamUrl = pb.progressiveStreamUrl;
    slot.usedHLSFallback = normalizeFeedMediaURL(slot.streamUrl) === normalizeFeedMediaURL(slot.hlsStreamUrl);
    slot.usedProgressiveFallback = normalizeFeedMediaURL(slot.streamUrl) === normalizeFeedMediaURL(slot.progressiveStreamUrl);
    slot.video.crossOrigin = 'anonymous';
    slot.video.preload = 'auto';
    state.feed.consecutiveSourceFailures = 0;
    scheduleFeedFrameMetric(slot);
    slot.video.playbackRate = state.feed.playbackSpeed || 1;
    slot.metricSourceAssignedAt = performance.now();
    assignFeedMediaSource(slot, slot.streamUrl, progressive);
    slot.ready = true;
    slot.metricCacheHit = slot.completeCached;
    slot.metricStreamMode = pb.isFullCached
      ? 'full-cache'
      : (slot.usedHLSFallback ? 'hls' : 'progressive');
    recordFeedDiagnostic('source-selected', { reason: foreground ? 'foreground' : 'preload',
      toIndex: itemIndex }, slot);
    slot.preparedAt = Date.now();
    slot.expiresAt = slot.preparedAt + 10 * 60 * 1000;
    syncActiveFeedSlot(slot, pb);
    assessFeedMediaCapabilities();
    const storedPosition = state.feed.playbackPositions.get(feedPlaybackKey(drama.id, slot.episodeNum, selectedQuality));
    const restorePosition = Number.isFinite(resumeAt)
      ? resumeAt
      : (storedPosition ?? (autoPlay ? pb.lastPosition : null));
    const shouldRestore = restorePosition > 3 && (!slot.video.duration || restorePosition < slot.video.duration - 5);

    const startPlayback = () => {
      if (loadToken !== slot.loadToken) return;
      slot.loaded = true;
      requestFeedDramaMetadata(slot);
      if (feedSlots[feedSlotCurr] === slot) ensureFeedItemsAhead();
      clearFeedFailureHint(slot);
      if (slot.poster) slot.poster.classList.add('hidden');
      showFeedSkeleton(false);
      if (feedSlots[feedSlotCurr] === slot) updateFeedPlayIndicator();
      const duration = feedMediaDuration(slot.video);
      if (shouldRestore && duration > 0) {
        slot.video.currentTime = Math.min(restorePosition, duration - 0.5);
      }
      if (state.currentView === 'recommend' && !state.feed.userPaused && feedSlots[feedSlotCurr] === slot) {
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
      requestFeedDramaMetadata(slot);
      if (feedSlots[feedSlotCurr] === slot) ensureFeedItemsAhead();
      clearFeedFailureHint(slot);
      if (slot.poster) slot.poster.classList.add('hidden');
      showFeedSkeleton(false);
      if (feedSlots[feedSlotCurr] === slot) updateFeedPlayIndicator();
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
        if (!source || slot.video.src === source || (slot.progressiveMedia && slot.streamUrl === source)) return false;
        slot.ready = true;
        assignFeedMediaSource(slot, source, source === slot.progressiveStreamUrl);
        slot.video.onloadeddata = () => {
          if (loadToken !== slot.loadToken) return;
          slot.loaded = true;
          clearFeedFailureHint(slot);
          if (resumePosition > 0 && Number.isFinite(slot.video.duration)) {
            slot.video.currentTime = Math.min(resumePosition, slot.video.duration - 0.5);
          }
          if (state.currentView === 'recommend' && !state.feed.userPaused && feedSlots[feedSlotCurr] === slot) {
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

    if (feedVideoHasFirstFrame(slot.video)) {
      startPlayback();
    }
    // WebKit may wait for play() before fetching/decoding. Waiting for
    // loadeddata before the first play() creates a cold-start deadlock.
    if (state.currentView === 'recommend' && !state.feed.userPaused && feedSlots[feedSlotCurr] === slot) {
      beginFeedPlayback(slot.video, feedPlayToken);
      scheduleFeedAutoplayRetry(slot);
      scheduleFeedWarmup(itemIndex);
    } else if (primeFirstFrame) {
      slot.firstFramePriming = primeFeedSlotFirstFrame(slot);
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
  if (slot) slot.failed = true;
  recordFeedDiagnostic('media-failure', { reason: 'source-unavailable' }, slot);
  updateFeedPlayIndicator();
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

function updateFeedPreloadWindow(slot, items) {
  feedPreloadRevision = Math.max(feedPreloadRevision + 1, Date.now());
  return apiCall('UpdateFeedPreloadWindow', feedPreloadRevision, slot?.episodeId || '', slot?.quality || state.feed.quality, items);
}

function cancelScheduledFeedWarmup({ releaseMedia = true } = {}) {
  feedWarmupToken++;
  feedWarmupCenterKey = '';
  if (feedWarmupSession?.timer) clearTimeout(feedWarmupSession.timer);
  feedWarmupSession = null;
  if (releaseMedia) {
    const slot = state.currentView === 'recommend' ? feedSlots[feedSlotCurr] : null;
    updateFeedPreloadWindow(slot, []).catch(err => console.debug('release feed preloads failed:', err));
  }
}

function prefetchFeedPlaybackInfo(itemIndex, preferHLS = true, quality = feedQualityForItem(itemIndex), episodeNum = 1, episodeId = '') {
  const drama = state.feed.items[itemIndex];
  if (!drama) return null;
  const key = feedAssetKey(drama.id, episodeNum, quality);
  const generation = feedPlaybackGeneration;
  const cached = state.feed.preloadMap.get(key);
  if (cached && cached.expiresAt > Date.now() && (!preferHLS || cached.isFullCached || cached.hlsStreamUrl)) return Promise.resolve(cached);
  const existingLoad = state.feed.playbackInfoLoads.get(key);
  if (existingLoad) {
    if (!preferHLS || existingLoad.preferHLS) return existingLoad;
    return existingLoad.then(playback => {
      if (generation !== feedPlaybackGeneration || state.feed.items[itemIndex]?.id !== drama.id) return null;
      if (preferHLS && playback && !playback.isFullCached && !playback.hlsStreamUrl) {
        return prefetchFeedPlaybackInfo(itemIndex, true, quality);
      }
      return playback;
    });
  }

  const requestedAt = performance.now();
  const load = apiCall('PlayEpisode', drama.id, episodeId, episodeNum, quality, preferHLS)
    .then(playback => {
      if (generation !== feedPlaybackGeneration) return null;
      if (playback?.streamUrl) {
        playback.expiresAt = Date.now() + 10 * 60 * 1000;
        rememberFeedPlayback(key, playback);
        rememberFeedPlayback(feedAssetKey(drama.id, episodeNum, playback.quality || quality), playback);
      }
      return playback;
    })
    .catch(err => {
      recordFeedDiagnostic('playback-info-failed', { toIndex: itemIndex,
        durationMs: Math.round(performance.now() - requestedAt), reason: 'request-failed' });
      console.warn('feed playback info prefetch failed:', drama.id, err);
      return null;
    })
    .finally(() => {
      if (state.feed.playbackInfoLoads.get(key) === load) {
        state.feed.playbackInfoLoads.delete(key);
      }
    });
  load.preferHLS = preferHLS;
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
  enqueueFeedMetric(metric);
  console.debug('[feed-metric]', metric);
  setTimeout(() => recordFeedPlaybackMetric(metric.toIndex), 5000);
}

function recordFeedPlaybackMetric(itemIndex, type = 'playback-sample') {
  const slot = feedSlots[feedSlotCurr];
  if (!slot || slot.itemIndex !== itemIndex) return;
  const diagnostics = state.feed.networkDiagnostics || {};
  const activeRebufferMS = slot.metricRebufferStartedAt
    ? Math.max(0, performance.now() - slot.metricRebufferStartedAt)
    : 0;
  const metric = {
    type,
    toIndex: itemIndex,
    bufferedAtSwitch: feedVideoHasPlaybackBuffer(slot.video),
    durationMs: Math.round(performance.now() - (slot.metricFirstFrameAt || slot.metricPreparedAt)),
    timeToFirstFrameMs: slot.metricFirstFrameAt && slot.metricPreparedAt
      ? Math.max(0, Math.round(slot.metricFirstFrameAt - slot.metricPreparedAt))
      : 0,
    playbackInfoMs: slot.metricSourceAssignedAt && slot.metricPreparedAt
      ? Math.max(0, Math.round(slot.metricSourceAssignedAt - slot.metricPreparedAt)) : 0,
    mediaStartupMs: slot.metricSourceAssignedAt && slot.metricFirstFrameAt
      ? Math.max(0, Math.round(slot.metricFirstFrameAt - slot.metricSourceAssignedAt)) : 0,
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
  enqueueFeedMetric(metric);
}

function recordFeedDiagnostic(event, details = {}, slot = feedSlots[feedSlotCurr]) {
  // Diagnostics must never interrupt playback or consume a navigation input.
  try {
    const video = slot?.video;
    const metric = {
      type: 'diagnostic', event, sessionId: feedDiagnosticSession,
      fromIndex: state.feed.index, toIndex: slot?.itemIndex ?? state.feed.index,
      readyState: video?.readyState || 0, bufferedSec: feedVideoBufferAhead(video),
      pageLoading: Boolean(state.feed.loading), itemCount: state.feed.items.length,
      streamMode: slot?.metricStreamMode || '', quality: slot?.quality || '',
      seriesId: slot?.seriesId || '', episodeId: slot?.episodeId || '',
      slotId: slot?.id ?? -1, loadToken: slot?.loadToken || 0,
      sincePageMs: Math.round(performance.now()),
      playingMs: slot?.metricPlayingAt ? Math.round(slot.metricPlayingAt - slot.metricPreparedAt) : 0,
      renderedMs: slot?.metricRenderedAt ? Math.round(slot.metricRenderedAt - slot.metricPreparedAt) : 0,
      playbackInfoMs: slot?.metricSourceAssignedAt ? Math.round(slot.metricSourceAssignedAt - slot.metricPreparedAt) : 0,
      timeToFirstFrameMs: slot?.metricFirstFrameAt ? Math.round(slot.metricFirstFrameAt - slot.metricPreparedAt) : 0,
      switchReadyMs: slot?.metricReadyAt ? Math.round(slot.metricReadyAt - slot.metricPreparedAt) : 0,
      ...details, at: new Date().toISOString()
    };
    enqueueFeedMetric(metric);
  } catch (_) { /* Best effort during media/DOM teardown. */ }
}

function enqueueFeedMetric(metric) {
  feedMetricQueue.push(metric);
  if (feedMetricQueue.length > 128) feedMetricQueue.splice(0, feedMetricQueue.length - 128);
  if (feedMetricQueue.length >= 24 && !feedMetricFlushInFlight) void flushFeedMetrics();
  else scheduleFeedMetricFlush();
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
    if (feedMetricQueue.length) scheduleFeedMetricFlush();
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
window.addEventListener('error', event => {
  recordFeedDiagnostic('frontend-error', { reason: `${event.error?.name || 'Error'}:line-${event.lineno || 0}` });
});
window.addEventListener('unhandledrejection', event => {
  recordFeedDiagnostic('frontend-rejection', { reason: String(event.reason?.name || 'Error').slice(0, 40) });
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
  const ranges = video?.buffered;
  if (!video?.src || !ranges) return 0;
  const position = video.currentTime || 0;
  for (let i = 0; i < ranges.length; i++) {
    if (ranges.start(i) <= position && ranges.end(i) >= position) {
      return Math.max(0, ranges.end(i) - position);
    }
  }
  return 0;
}

// All buffer thresholds use seconds of WALL time, including 1.5x/2x playback.
// Grow only after sustained headroom; shrink at lower watermarks to avoid
// repeatedly canceling/restarting downloads around a single threshold.
function decideFeedPreload(sample, previous = {}) {
  const rate = Math.max(0.25, Number(sample.rate) || 1);
  const reserve = Math.max(0, sample.bufferSec) / rate;
  const elapsed = (sample.now - (previous.at ?? sample.now)) / 1000;
  const slope = elapsed >= 0.25 && elapsed <= 3
    ? Math.max(-2, Math.min(4, (reserve - (previous.reserve ?? reserve)) / elapsed))
    : 0;
  const trend = elapsed > 3 ? 0 : (previous.trend || 0) * 0.7 + slope * 0.3;
  let depth = previous.depth || 0;
  let stableSince = previous.stableSince ?? null;
  let cooldownUntil = previous.cooldownUntil || 0;
  const limit = sample.profile === 'weak' ? 1 : 2;

  if (sample.waiting || (!sample.complete && reserve < 4)) {
    if (depth > 0 || sample.waiting) cooldownUntil = sample.now + 4000;
    return { depth: 0, at: sample.now, reserve, trend: 0, stableSince: null, cooldownUntil };
  }
  if (!sample.complete) {
    if (reserve < 10 || (trend < -0.35 && reserve < 14)) depth = Math.min(depth, 1);
    else if (reserve < 18) depth = Math.min(depth, 2);
  }
  const healthy = sample.complete || (reserve >= 6 && trend >= -0.2);
  stableSince = healthy ? (stableSince ?? sample.now) : null;
  const stableMS = stableSince === null ? 0 : sample.now - stableSince;
  if (sample.now >= cooldownUntil && healthy) {
    // Do not delay the immediate next once current has a safe prefix. Only
    // widening beyond it needs a sustained surplus across several samples.
    depth = Math.max(depth, 1);
    if (stableMS >= 2500 && (sample.complete || reserve >= 14) && sample.nextReady) depth = Math.max(depth, 2);
  }
  depth = Math.min(depth, limit);
  if (depth < (previous.depth || 0)) cooldownUntil = Math.max(cooldownUntil, sample.now + 3000);
  depth = Math.min(depth, Math.max(0, sample.available));
  return { depth, at: sample.now, reserve, trend, stableSince, cooldownUntil };
}

function cachedFeedPlayback(itemIndex) {
  const drama = state.feed.items[itemIndex];
  if (!drama) return null;
  const pb = state.feed.preloadMap.get(feedAssetKey(drama.id, 1, feedQualityForItem(itemIndex)));
  return pb?.expiresAt > Date.now() ? pb : null;
}

function wakeFeedWarmup(urgent = false) {
  const session = feedWarmupSession;
  if (!session || session.token !== feedWarmupToken) return;
  if (session.running) {
    session.pending = true;
    return;
  }
  if (session.timer) clearTimeout(session.timer);
  const delay = urgent ? 0 : Math.max(0, 750 - (performance.now() - session.lastRun));
  session.timer = setTimeout(() => runFeedWarmup(session), delay);
}

async function scheduleFeedWarmup(centerIndex) {
  if (state.currentView !== 'recommend' || state.feed.userPaused) return;
  const drama = state.feed.items[centerIndex];
  const current = feedSlots[feedSlotCurr];
  if (!drama || current?.itemIndex !== centerIndex || !current.episodeId) return;
  const centerKey = `${centerIndex}:${drama.id}:${current.episodeId}:${current.quality}`;
  if (feedWarmupCenterKey === centerKey) return;
  const old = feedWarmupSession;
  cancelScheduledFeedWarmup({ releaseMedia: false });
  feedWarmupCenterKey = centerKey;
  const inheritedDepth = old && centerIndex > old.centerIndex
    ? Math.max(0, old.policy.depth - (centerIndex - old.centerIndex)) : 0;
  const session = {
    token: feedWarmupToken, centerIndex, current, timer: null, running: false,
    pending: false, lastRun: -Infinity, lastDiagnostics: -Infinity,
    policy: { depth: inheritedDepth }, statuses: new Map(), metadataTries: new Map()
  };
  feedWarmupSession = session;
  preloadFeedCovers(state.feed.items, centerIndex, 3);
  await runFeedWarmup(session);
}

async function runFeedWarmup(session) {
  const valid = () => session === feedWarmupSession && session.token === feedWarmupToken &&
    state.currentView === 'recommend' && !state.feed.userPaused && state.feed.index === session.centerIndex;
  if (!valid() || session.running) return;
  session.running = true;
  session.pending = false;
  session.lastRun = performance.now();
  try {
    ensureFeedItemsAhead();
    const now = performance.now();
    if (now - session.lastDiagnostics >= 3000) {
      session.lastDiagnostics = now;
      refreshFeedNetworkDiagnostics();
    }
    // Negotiation is cheap and does not start a media download. Keep a small,
    // bounded metadata window ready even while current playback needs recovery.
    const firstFrameReady = feedVideoHasFirstFrame(session.current.video);
    for (let offset = 1; firstFrameReady && offset <= 2; offset++) {
      const index = session.centerIndex + offset;
      if (!state.feed.items[index] || cachedFeedPlayback(index)) continue;
      const lastTry = session.metadataTries.get(index) ?? -Infinity;
      if (now - lastTry < 5000) continue;
      session.metadataTries.set(index, now);
      Promise.resolve(prefetchFeedPlaybackInfo(index, false)).then(() => {
        if (valid()) wakeFeedWarmup();
      });
    }
    const video = session.current.video;
    const ahead = feedVideoBufferAhead(video);
    const ready = offset => {
      const pb = cachedFeedPlayback(session.centerIndex + offset);
      const status = pb && session.statuses.get(`${pb.episodeId}:${pb.quality}`);
      return Boolean(pb?.isFullCached || pb?.isHlsCached || status?.complete || status?.cachedSeconds >= 6);
    };
    session.policy = decideFeedPreload({
      now, bufferSec: firstFrameReady ? ahead : 0, rate: video.playbackRate, profile: effectiveFeedNetworkProfile(),
      complete: firstFrameReady && feedSlotIsFullyBuffered(session.current),
      // WebKit briefly lowers readyState during a handoff/seek even when a
      // playable prefix is buffered. Do not cancel and rebuild the next HLS
      // rendition just because its new foreground decoder is catching up.
      waiting: video.readyState < HTMLMediaElement.HAVE_FUTURE_DATA &&
        ahead / Math.max(0.25, Number(video.playbackRate) || 1) < 1.5 &&
        !video.ended && (video.currentTime > 0 || session.policy.depth > 0),
      nextReady: ready(1), secondReady: ready(2),
      available: state.feed.items.length - session.centerIndex - 1
    }, session.policy);

    // An explicit swipe is foreground demand, even before the animation has
    // promoted its target. Do not clear that decoder as speculative work.
    const depth = feedSwitchWaitDir > 0 ? Math.max(1, session.policy.depth) : session.policy.depth;
    const items = [];
    for (let offset = 1; offset <= depth; offset++) {
      const pb = cachedFeedPlayback(session.centerIndex + offset);
      if (!pb?.episodeId) break;
      const progressiveSlot = feedSlots.find(slot => slot.ready && slot.progressiveMedia &&
        slot.episodeId === pb.episodeId && slot.quality === pb.quality);
      if (progressiveSlot) {
        // Foreground MP4 fragments already own this asset. Do not admit a
        // second HLS download for the same pending swipe or cached decoder.
        if (feedVideoBufferAhead(progressiveSlot.video) < 6) break;
        continue;
      }
      items.push({ seriesId: state.feed.items[session.centerIndex + offset].id,
        episodeId: pb.episodeId, number: pb.episodeNum || 1, quality: pb.quality });
    }
    const nextSlot = feedSlots[feedSlotNext];
    if (!depth && nextSlot?.itemIndex === session.centerIndex + 1 && !feedSlotIsFullyBuffered(nextSlot)) {
      clearFeedSlot(nextSlot);
    }
    session.pendingMedia = new Set(items.map(item => `${item.episodeId}:${item.quality}`));
    const statuses = await updateFeedPreloadWindow(session.current, items);
    session.pendingMedia = null;
    if (!valid()) return;
    session.statuses = new Map((statuses || []).map(status => [`${status.episodeId}:${status.quality}`, status]));
    if (items.length && nextSlot && !nextSlot.preparing) {
      // Only i+1 needs a decoder. i+2 warms the SAME local HLS cache and
      // acquire a decoder when the three-element carousel rotates to them.
      prepareFeedSlot(nextSlot, session.centerIndex + 1, false, null, true)
        .catch(err => console.warn('feed next warmup failed:', err));
    }
  } catch (err) {
    console.warn('feed preload scheduling failed:', err);
  } finally {
    session.pendingMedia = null;
    session.running = false;
    if (valid()) {
      session.timer = setTimeout(() => runFeedWarmup(session), session.pending ? 250 : 1000);
    }
  }
}

const state = {
  currentView: 'recommend',

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
    hasStartedPlayback: false,
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
    detailReady: false,
    seasonSwitching: false,
    userPaused: false,
    episodesExpanded: false
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

// Keyboard actions stay immediate; pointer feedback may use subtle motion.
// Capture runs before control handlers, including synthetic clicks from keys.
document.addEventListener('keydown', () => {
  document.documentElement.dataset.motionInput = 'keyboard';
}, true);
document.addEventListener('pointerdown', () => {
  document.documentElement.dataset.motionInput = 'pointer';
}, true);

// Toasts remain mounted so a new message can reverse an in-progress exit.
function revealToast(toast, duration) {
  clearTimeout(toast._timer);
  toast.classList.add('is-visible');
  toast._timer = setTimeout(() => {
    toast.classList.remove('is-visible');
  }, duration);
}

// Toast notification
function showToast(msg) {
  const toast = document.getElementById('toast');
  toast.textContent = msg;
  revealToast(toast, 2200);
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
  const target = getFeedVideo();
  const inPiP = !!document.pictureInPictureElement || (target && target.webkitPresentationMode === 'picture-in-picture');
  const feedPipBtn = document.getElementById('feed-pip');
  if (feedPipBtn) {
    feedPipBtn.classList.toggle('active', !!inPiP);
    feedPipBtn.title = inPiP ? '退出画中画 (P)' : '画中画 (P)';
  }
}

function togglePictureInPicture() {
  const targetVideo = getFeedVideo();
  if (!targetVideo) {
    showToast('未找到可播放视频');
    return;
  }
  const inStandardPiP = !!document.pictureInPictureElement;
  const inWebkitPiP = targetVideo.webkitPresentationMode === 'picture-in-picture';
  if (inStandardPiP || inWebkitPiP) {
    if (inWebkitPiP && typeof targetVideo.webkitSetPresentationMode === 'function') {
      try { targetVideo.webkitSetPresentationMode('inline'); } catch (err) { console.warn(err); }
      return;
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
  if (typeof targetVideo.webkitSetPresentationMode === 'function') {
    try { targetVideo.webkitSetPresentationMode('picture-in-picture'); return; }
    catch (err) { console.warn(err); }
  }
  if (typeof targetVideo.requestPictureInPicture === 'function') {
    targetVideo.requestPictureInPicture().catch(err => {
      console.warn(err);
      apiCall('TogglePictureInPicture').catch(() => {});
    });
    return;
  }
  apiCall('TogglePictureInPicture').catch(() => {});
}

function handleMediaCommand(payload) {
  const command = payload?.command;
  if (!command) return;
  if (state.currentView !== 'recommend') return;
  const feedVideo = getFeedVideo();
  if (!feedVideo) return;

  if (command === 'toggle') {
    toggleFeedPlay();
    return;
  }
  if (command === 'play') {
    if (feedVideo.paused) feedVideo.play().catch(error => console.log('play error:', error));
    return;
  }
  if (command === 'pause') {
    if (!feedVideo.paused) feedVideo.pause();
    return;
  }
  if (command === 'next') {
    swipeFeed('up');
    return;
  }
  if (command === 'prev') {
    swipeFeed('down');
    return;
  }
  if (command === 'seek' && typeof payload.position === 'number') {
    feedVideo.currentTime = payload.position;
    return;
  }
  if (command === 'pip') {
    togglePictureInPicture();
  }
}

function initNativeBridgeEvents() {
  waitForMygoBridge().then(runtime => {
    if (!runtime?.on) return;

    // Handle macOS menu & media keys command events
    runtime.on('mediaCommand', handleMediaCommand);

    // Picture-in-Picture document events
    document.addEventListener('enterpictureinpicture', updatePiPButtons);
    document.addEventListener('leavepictureinpicture', updatePiPButtons);
  }).catch(() => {});
}

// ==========================================================================
// Initialization & Navigation
// ==========================================================================

document.addEventListener('DOMContentLoaded', () => {
  recordFeedDiagnostic('page-ready');
  initNativeBridgeEvents();
  mountPlayerChrome({
    prefix: 'feed',
    target: document.getElementById('feed-container'),
    drawerTitle: '选集播放'
  });
  initFeedVideoPool();
  initFeedControls();
  initCommentSheet();
  initDanmaku();
  initSettingsEvents();

  // 1. Immediately switch to Recommend view for instant render
  if (!window.myguoNativeMedia) {
    switchView('recommend');
    loadProfile();
  } else {
    state.currentView = 'native-idle';
    document.querySelectorAll('.view-page').forEach(page => page.classList.remove('active'));
  }
});

function beginViewRequest(view) {
  const tokens = state.viewRequestTokens || (state.viewRequestTokens = {});
  return tokens[view] = (tokens[view] || 0) + 1;
}

function isViewRequestCurrent(view, token) {
  return state.currentView === view && state.viewRequestTokens?.[view] === token;
}

function applyFavoriteState(seriesId, favorite) {
  window.nativeMediaNotify?.("LibraryChanged");
  const changes = state.favoriteChanges || (state.favoriteChanges = {});
  changes[seriesId] = { revision: (changes[seriesId]?.revision || 0) + 1, value: favorite };
  if (state.feed.seriesId === seriesId) {
    state.feed.isFavorite = favorite;
    document.getElementById('feed-fav')?.classList.toggle('active', favorite);
  }
  if (!favorite) {
  }
}

function applyLikeState(seriesId, episodeId, liked) {
  window.nativeMediaNotify?.("LibraryChanged");
  if (state.feed.seriesId === seriesId && state.feed.episodeId === episodeId) {
    state.feed.isLiked = liked;
    document.getElementById('feed-like')?.classList.toggle('active', liked);
  }
  if (!liked) {
  }
}

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

let feedNextDramaTimer = null;

function resumeFeedPlayback() {
  if (state.currentView !== 'recommend') return;
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

function switchView(viewName) {
  if (!['recommend', 'settings', 'native-idle'].includes(viewName)) return;
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
  document.querySelectorAll('.view-page').forEach(page => page.classList.remove('active'));
  const page = document.getElementById(`view-${viewName}`);
  if (page) page.classList.add('active');
  if (viewName === 'recommend') loadRecommend();
  if (viewName === 'settings') loadSettings();
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

async function clearMediaCache() {
  cancelScheduledFeedWarmup({ releaseMedia: false });
  feedPlaybackGeneration++;
  state.feed.preloadMap.clear();
  state.feed.retainedQualities?.clear();
  state.feed.playbackInfoLoads.clear();
  // Keep the visible decoder; discard adjacent sources that may reference a
  // cached file about to be removed. Late metadata loads have been invalidated.
  for (const slot of feedSlots) {
    if (state.currentView !== 'recommend' || slot !== feedSlots[feedSlotCurr] || !slot.ready) clearFeedSlot(slot);
  }
  try {
    const current = state.currentView === 'recommend' ? feedSlots[feedSlotCurr] : null;
    await updateFeedPreloadWindow(current, []);
    return await apiCall('ClearCache');
  } finally {
    if (state.currentView === 'recommend') {
      if (!feedSlots[feedSlotCurr]?.streamUrl) {
        playFeedIndex(state.feed.index).catch(err => console.warn('resume after cache clear failed:', err));
      } else {
        scheduleFeedWarmup(state.feed.index);
      }
    }
  }
}

function initSettingsEvents() {
  // Clear cache in settings
  const btnClear = document.getElementById('btn-settings-clear-cache');
  if (btnClear) {
    btnClear.addEventListener('click', async () => {
      try {
        const bytes = await clearMediaCache();
        const mb = (bytes / (1024 * 1024)).toFixed(1);
        const cacheEl = document.getElementById('settings-cache-size');
        const remaining = await apiCall('GetCacheSize');
        if (cacheEl) cacheEl.textContent = `${(remaining / (1024 * 1024)).toFixed(1)} MB`;
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

// ==========================================================================
// 1. Recommend View (Vertical Swipe Video Feed - TikTok/Hongguo Style)
// ==========================================================================

let feedRequestToken = 0;
function showFeedHint(text, spinner = false, duration = 2200) {
  const toast = document.getElementById('feed-hint-toast');
  const txt = document.getElementById('feed-hint-text');
  const spin = document.getElementById('feed-hint-spinner');
  if (!toast || !txt) return;

  txt.textContent = text;
  if (spin) spin.style.display = spinner ? 'inline-block' : 'none';
  revealToast(toast, duration);
}

async function loadRecommend() {
  syncFeedScrollCapture();
  if (state.feed.items.length === 0) {
    await loadFeed(true, state.feed.channel);
  } else {
    resumeFeedPlayback();
  }
}

async function loadFeed(reset = false, channel = '', forceRefresh = false) {
  if (!reset && state.feed.loading) return;
  const requestToken = ++feedRequestToken;
  const requestedAt = performance.now();
  recordFeedDiagnostic('catalog-request', { reason: reset ? 'initial' : 'fill' });
  state.feed.loading = true;
  if (state.feed.pageRetryTimer) {
    clearTimeout(state.feed.pageRetryTimer);
    state.feed.pageRetryTimer = null;
  }

  if (reset) {
    feedPlaybackGeneration++;
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
      const bar = typeof document !== 'undefined' ? document.getElementById('feed-cat-bar') : null;
      if (bar) {
        bar.querySelectorAll('.cat-chip').forEach(c => {
          c.classList.toggle('active', (c.dataset.channel || '') === channel);
        });
        if (typeof updateCatBarGlider === 'function') {
          updateCatBarGlider();
        }
      }
    }
    if (forceRefresh) {
      state.feed.cursor = '';
    }

    showFeedSkeleton(true);
  }

  try {
    const queryCursor = forceRefresh ? 'refresh' : (reset ? '' : state.feed.cursor);
    const method = reset ? 'GetRecommendFeed' : 'GetRecommendFeedBatch';
    let res = null;
    if (reset && !forceRefresh && state.feed.channel !== '关注') {
      try {
        res = await apiCall('RestoreRecommendFeed', state.feed.channel);
      } catch (err) {
        console.warn('restore recommend queue failed:', err);
      }
      if (requestToken !== feedRequestToken) return;
    }
    if (!res?.items?.length) {
      res = await apiCall(method, state.feed.channel, queryCursor, reset ? 1 : 12);
    }
    recordFeedDiagnostic('catalog-response', { durationMs: Math.round(performance.now() - requestedAt),
      itemCount: res?.items?.length || 0, reason: requestToken === feedRequestToken ? 'current' : 'stale' });
    if (requestToken !== feedRequestToken) return;

    // Even a fully filtered batch advances the stream. An empty cursor also
    // clears an expired stream so a retry can use the persisted continuation.
    if (res) state.feed.cursor = res.cursor || '';

    if (!res || !res.items || res.items.length === 0) {
      showFeedSkeleton(false);
      // A recommendation API can return an empty recoverable page. Only the
      // finite favorites channel should turn emptiness into a hard boundary.
      const canRetry = state.feed.channel !== '关注' && res?.hasMore !== false;
      state.feed.hasMore = canRetry;
      if (!reset) state.feed.emptyFeedPages++;
      if (reset) {
        if (!canRetry) showFeedHint('暂无关注内容', false, 3000);
      }
      return;
    }

    if (reset || state.feed.items.length === 0) {
      state.feed.items = res.items;
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
      state.feed.hasMore = Boolean(res.hasMore);
      if (newItems.length > 0) state.feed.emptyFeedPages = 0;
      else state.feed.emptyFeedPages++;
    }
  } catch (err) {
    recordFeedDiagnostic('catalog-failed', { durationMs: Math.round(performance.now() - requestedAt), reason: 'request-failed' });
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
      updateFeedPlayIndicator();
      ensureFeedItemsAhead();
    }
  }
}

function ensureFeedItemsAhead(minAhead = 8) {
  if (state.currentView !== 'recommend' || !state.feed.hasMore || state.feed.loading) return;
  if (state.feed.items.length && !feedVideoHasFirstFrame(getFeedVideo())) return;
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
    titleElem.classList.remove('feed-title-link');
    titleElem.onclick = null;
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

function requestFeedDramaMetadata(slot) {
  if (!slot || state.currentView !== 'recommend' ||
      feedSlots[feedSlotCurr] !== slot || !feedVideoHasFirstFrame(slot.video)) return;
  const drama = state.feed.items[slot.itemIndex];
  if (!drama || drama.id !== slot.seriesId) return;
  // A fetched or offscreen preloaded card is still pending. Remove it from
  // the durable queue only after it has actually appeared in the foreground.
  if (!slot.queueConsumed && state.feed.channel !== '关注') {
    slot.queueConsumed = true;
    const token = slot.loadToken;
    apiCall('ConsumeRecommendFeedItem', state.feed.channel, drama.id).catch(err => {
      if (slot.loadToken === token) slot.queueConsumed = false;
      console.warn('consume recommend queue item failed:', err);
    });
  }
  if (slot.metadataRequested) return;
  if (!state.feed.hasStartedPlayback) {
    state.feed.hasStartedPlayback = true;
    recordFeedPlaybackMetric(slot.itemIndex, 'startup');
  }
  slot.metadataRequested = true;
  const loadToken = slot.loadToken;
  Promise.resolve(loadFeedDramaMetadata(drama)).then(loaded => {
    if (!loaded && slot.loadToken === loadToken) slot.metadataRequested = false;
  });
}

function loadFeedDramaMetadata(drama) {
  if (!drama) return Promise.resolve(false);
  apiCall('IsFavorite', drama.id).then(isFavorite => {
    if (state.feed.seriesId !== drama.id) return;
    state.feed.isFavorite = Boolean(isFavorite);
    document.getElementById('feed-fav')?.classList.toggle('active', Boolean(isFavorite));
  }).catch(() => {});

  return apiCall('GetSeriesDetail', drama.id).then(detail => {
    if (!detail || detail.id !== state.feed.seriesId) return false;
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
    return true;
  }).catch(() => false);
}

async function playFeedIndex(idx, resumeAt = null, episodeNum = 1, episodeId = '') {
  if (idx < 0 || idx >= state.feed.items.length) return;
  const drama = state.feed.items[idx];
  if (!drama) return;

  const currentSlot = feedSlots[feedSlotCurr];
  activateFeedDrama(drama, idx, currentSlot?.itemIndex === idx ? currentSlot : null);

  if (state.currentView !== 'recommend') return;

  await prepareFeedSlot(currentSlot, idx, true, resumeAt, false, null, episodeNum, episodeId);

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
  const preparing = prepareFeedSlot(slot, itemIndex, true);
  // Preparing a new source advances loadToken synchronously. Capture the
  // resulting generation so an initial load failure can actually be retried.
  const loadToken = slot.loadToken;
  preparing.then(() => {
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
let feedSwitchWaitOrigin = -1;
let feedSwitchWaitReason = '';
let feedSwitchIntentGeneration = 0;
let feedQueuedSwitches = [];
let feedQueuedSwitchTimer = null;
let feedActiveSwitchRequest = null;
let feedAnimationToken = 0;

function cancelPendingFeedSwitch({ keepQueue = false, reason = 'cancelled' } = {}) {
  if (feedSwitchWaitDir) recordFeedDiagnostic('navigation-wait-end', {
    reason, waitingMs: Date.now() - feedSwitchWaitStartedAt, inputId: feedActiveSwitchRequest?.id || 0 });
  if (!keepQueue) {
    for (const queued of feedQueuedSwitches) recordFeedDiagnostic('navigation-cancelled', {
      reason, inputId: queued.request?.id || 0 });
  }
  if (feedSwitchWaitTimer) clearTimeout(feedSwitchWaitTimer);
  feedSwitchWaitTimer = null;
  feedSwitchWaitDir = 0;
  feedSwitchWaitStartedAt = 0;
  feedSwitchWaitOrigin = -1;
  feedSwitchWaitReason = '';
  feedActiveSwitchRequest = null;
  feedSwitchIntentGeneration++;
  if (!keepQueue) {
    if (feedQueuedSwitchTimer) clearTimeout(feedQueuedSwitchTimer);
    feedQueuedSwitchTimer = null;
    feedQueuedSwitches = [];
  }
  updateFeedPlayIndicator();
}

function queueFeedSwitch(dir, request = null) {
  const last = feedQueuedSwitches.at(-1);
  if (last && last.dir !== dir) {
    for (const queued of feedQueuedSwitches) recordFeedDiagnostic('navigation-cancelled', {
      reason: 'direction-changed', inputId: queued.request?.id || 0 });
    feedQueuedSwitches = [];
  }
  const discrete = request?.source === 'keyboard' && !request.repeat;
  if (discrete && feedQueuedSwitches.length >= 4) {
    recordFeedDiagnostic('navigation-ignored', { inputId: request.id, reason: 'queue-full', queueDepth: 4 });
    return;
  }
  if (discrete && feedQueuedSwitches.length < 4) feedQueuedSwitches.push({ dir, request });
  else if (!discrete && !feedQueuedSwitches.length) feedQueuedSwitches.push({ dir, request });
  recordFeedDiagnostic('navigation-queued', { inputId: request?.id || 0,
    queueDepth: feedQueuedSwitches.length, reason: discrete ? 'key-press' : 'coalesced' });
  drainFeedSwitchQueue();
}

function drainFeedSwitchQueue() {
  if (!feedQueuedSwitches.length || feedQueuedSwitchTimer || isFeedAnimating || feedSwitchWaitDir) return;
  const delay = Math.max(0, state.feed.lastSwitchTime + 320 - Date.now());
  feedQueuedSwitchTimer = setTimeout(() => {
    feedQueuedSwitchTimer = null;
    if (!shouldCaptureFeedScroll()) {
      cancelPendingFeedSwitch({ reason: 'scene-blocked' });
      return;
    }
    const queued = feedQueuedSwitches.shift();
    if (queued) switchFeedDrama(queued.dir, queued.request);
  }, delay);
}

function requestFeedSwitch(dir, source = 'gesture', repeat = false) {
  if (repeat && Date.now() - feedLastRepeatAt < 200) return;
  if (repeat) feedLastRepeatAt = Date.now();
  const request = { id: ++feedInputSequence, source, repeat, at: Date.now() };
  recordFeedDiagnostic('navigation-input', { inputId: request.id, inputSource: source, repeat,
    toIndex: state.feed.index + dir });
  switchFeedDrama(dir, request);
}

function schedulePendingFeedSwitch(dir, delay = 50, reason = 'first-frame') {
  if (feedSwitchWaitDir !== dir || feedSwitchWaitOrigin !== state.feed.index) {
    const request = feedActiveSwitchRequest;
    cancelPendingFeedSwitch({ keepQueue: true });
    feedActiveSwitchRequest = request;
    feedSwitchWaitDir = dir;
    feedSwitchWaitOrigin = state.feed.index;
    feedSwitchWaitStartedAt = Date.now();
  }
  if (feedSwitchWaitReason !== reason) {
    feedSwitchWaitReason = reason;
    recordFeedDiagnostic('navigation-wait', { reason, inputId: feedActiveSwitchRequest?.id || 0 });
    updateFeedPlayIndicator();
  }
  if (feedSwitchWaitTimer) return;
  const generation = feedSwitchIntentGeneration;
  feedSwitchWaitTimer = setTimeout(() => {
    feedSwitchWaitTimer = null;
    if (generation !== feedSwitchIntentGeneration) return;
    if (shouldCaptureFeedScroll() && state.feed.index === feedSwitchWaitOrigin) switchFeedDrama(dir);
    else cancelPendingFeedSwitch();
  }, delay);
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
  prepareFeedSlot(slot, itemIndex, false, null, true, quality).catch(err => {
    console.warn('prepare feed target failed:', drama.id, err);
  });
}

function switchFeedDrama(dir, request = null) {
  if (!shouldCaptureFeedScroll() || (dir !== 1 && dir !== -1)) {
    if (request) recordFeedDiagnostic('navigation-ignored', { reason: 'scene-blocked', inputId: request.id });
    return;
  }
  const now = Date.now();
  if (feedSwitchWaitDir && (feedSwitchWaitDir !== dir || feedSwitchWaitOrigin !== state.feed.index)) {
    cancelPendingFeedSwitch({ reason: 'direction-changed' });
  } else if (feedSwitchWaitDir && request) {
    queueFeedSwitch(dir, request);
    return;
  }
  if (isFeedAnimating && now - state.feed.lastSwitchTime > 1000) {
    // The normal completion callback and watchdog above are defensive, but a
    // synchronous setup exception can still escape both. Self-heal here so one
    // bad swipe can never permanently disable the feed controls.
    console.warn('feed switch self-healing released a stuck animation lock');
    isFeedAnimating = false;
  }
  if (isFeedAnimating || now - state.feed.lastSwitchTime < 320) {
    queueFeedSwitch(dir, request);
    return;
  }
  if (request) feedActiveSwitchRequest = request;

  const targetIdx = state.feed.index + dir;
  if (targetIdx < 0) {
    showFeedHint('已经是第一部了', false, 1800);
    return;
  }

  if (targetIdx >= state.feed.items.length) {
    if (state.feed.hasMore) {
      const switchOrigin = state.feed.index;
      schedulePendingFeedSwitch(dir, 150, 'catalog-page');
      const generation = feedSwitchIntentGeneration;
      if (now - feedSwitchWaitStartedAt > 10000) {
        cancelPendingFeedSwitch({ reason: 'catalog-timeout' });
        showFeedHint('加载较慢，请稍后重试', false, 1800);
        return;
      }
      const finishBoundaryLoad = () => {
        if (generation !== feedSwitchIntentGeneration || state.feed.index !== switchOrigin || !shouldCaptureFeedScroll()) return;
        if (state.feed.items[targetIdx]) {
          switchFeedDrama(dir);
        } else {
          cancelPendingFeedSwitch();
          showFeedHint('暂时没有更多视频，请稍后重试', false, 1800);
        }
      };
      if (state.feed.loading) {
        return;
      }
      loadFeed(false).then(finishBoundaryLoad).catch(finishBoundaryLoad);
    } else {
      cancelPendingFeedSwitch();
      showFeedHint('已经到底了，没有更多了', false, 2200);
    }
    return;
  }

  const targetDrama = state.feed.items[targetIdx];
  const oldCurrentSlot = feedSlots[feedSlotCurr];
  const targetSlot = dir > 0 ? feedSlots[feedSlotNext] : feedSlots[feedSlotPrev];
  const offscreenSlot = dir > 0 ? feedSlots[feedSlotPrev] : feedSlots[feedSlotNext];
  const requestedTargetQuality = feedQualityForItem(targetIdx);
  const targetReadyBeforeMove = feedSlotMatches(targetSlot, targetIdx, targetDrama.id, requestedTargetQuality);
  const targetWaitMs = feedSwitchWaitDir === dir && feedSwitchWaitOrigin === state.feed.index
    ? now - feedSwitchWaitStartedAt : 0;
  const targetStartupDeadline = 350;
  if ((!targetReadyBeforeMove || !feedVideoHasFirstFrame(targetSlot.video)) && targetWaitMs < targetStartupDeadline) {
    // Give an imminent first frame a short head start while current playback
    // continues. A stalled offscreen decoder must not block navigation.
    schedulePendingFeedSwitch(dir);
    prepareFeedTargetForSwitch(targetSlot, targetIdx, requestedTargetQuality);
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
    drainFeedSwitchQueue();
  }, 420);
  const switchStartedAt = performance.now();
  const fromIndex = state.feed.index;
  const completedRequest = feedActiveSwitchRequest;

  persistFeedProgress();
  cancelPendingFeedSwitch({ keepQueue: true, reason: 'committed' });

  if (dir > 0) {
    feedSlotPrev = oldCurrentSlot.id;
    feedSlotNext = offscreenSlot.id;
  } else {
    feedSlotNext = oldCurrentSlot.id;
    feedSlotPrev = offscreenSlot.id;
  }

  const targetReady = feedSlotMatches(targetSlot, targetIdx, targetDrama.id, requestedTargetQuality);
  const targetBuffered = Boolean(targetReady && feedVideoHasPlaybackBuffer(targetSlot.video));
  activateFeedDrama(targetDrama, targetIdx, targetReady ? targetSlot : null);
  if (targetReady) prepareFeedStartPosition(targetSlot, dir);
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
    if (!state.feed.userPaused) {
      targetSlot.video.muted = false;
      targetSlot.video.volume = 1;
      beginFeedPlayback(targetSlot.video, feedPlayToken);
      scheduleFeedAutoplayRetry(targetSlot);
      // A primed video may already be playing and emit no new play event.
      scheduleFeedWarmup(targetIdx);
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
    if (animationToken !== feedAnimationToken) return;
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
      recordFeedDiagnostic('navigation-complete', { inputId: completedRequest?.id || 0,
        inputSource: completedRequest?.source || 'automatic',
        durationMs: completedRequest ? Date.now() - completedRequest.at : 0, fromIndex, toIndex: targetIdx });
    } catch (err) {
      // A metrics or DOM cleanup error must never leave the carousel locked.
      console.error('feed switch cleanup failed:', err);
    } finally {
      isFeedAnimating = false;
      drainFeedSwitchQueue();
    }
  }, duration);
}

async function enterSeriesPlayer(seriesId, episodeNum) {
  if (!seriesId) return;
  clearCommentSceneLayout();
  feedPlayToken++;
  cancelScheduledFeedWarmup();
  cancelPendingFeedSwitch();
  pauseFeedPlayback();
  state.feed.userPaused = false;

  const drawer = document.getElementById('feed-playlist-drawer');
  drawer?.classList.remove('open');

  let index = state.feed.seriesId === seriesId ? state.feed.index : state.feed.items.findIndex(item => item.id === seriesId);
  let episodes = state.feed.seriesId === seriesId ? state.feed.episodes : [];
  if (index < 0) {
    const detail = await apiCall('GetSeriesDetail', seriesId);
    if (!detail) throw new Error('获取短剧详情失败');
    episodes = detail.episodes || [];
    index = Math.max(0, state.feed.index || 0);
    state.feed.items.splice(index, 0, {
      id: seriesId,
      title: detail.title || seriesId,
      coverUrl: detail.coverUrl || '',
      description: detail.description || '',
      heat: detail.heat || ''
    });
  }

  const targetEpisode = Math.max(1, Number(episodeNum) || 1);
  const episodeId = episodes.find(episode => episode.number === targetEpisode)?.id || '';
  state.feed.episodes = episodes;
  await playFeedIndex(index, null, targetEpisode, episodeId);
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

function createEpisodeCollapseController({ grid, toggleWrap, toggle, collapsedClass = 'is-collapsed', rows = 3, rowHeight = 28, gap = 6 }) {
  const collapsedHeight = rows * rowHeight + (rows - 1) * gap;
  let expanded = false;
  let overflow = false;

  const sync = () => {
    grid.classList.toggle(collapsedClass, !expanded);
    toggleWrap.style.display = overflow ? 'flex' : 'none';
    toggle.setAttribute('aria-expanded', String(expanded));
    const label = toggle.querySelector('[data-collapse-text]');
    if (label) label.textContent = expanded ? '收起选集' : '展开全部';
    const icon = toggle.querySelector('svg');
    if (icon) icon.style.transform = expanded ? 'rotate(180deg)' : '';
    if (!expanded) grid.scrollTop = 0;
  };

  const setExpanded = (nextExpanded) => {
    expanded = Boolean(nextExpanded);
    sync();
    if (expanded) {
      const active = grid.querySelector('.ep-btn.active');
      if (active) {
        const activeTop = active.offsetTop - grid.offsetTop;
        grid.scrollTop = Math.max(0, activeTop - (grid.clientHeight - active.offsetHeight) / 2);
      }
    }
  };

  const measure = () => {
    if (grid.clientHeight > 0 && !expanded) {
      overflow = grid.scrollHeight > collapsedHeight + 1;
    } else {
      overflow = grid.children.length > rows * 12;
    }
    sync();
  };

  toggle.addEventListener('click', () => setExpanded(!expanded));
  measure();

  return {
    expand: () => setExpanded(true),
    collapse: () => setExpanded(false),
    measure,
    get expanded() { return expanded; },
    get overflow() { return overflow; },
  };
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

function danmakuContext() {
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
        return await apiCall('LikeDanmaku', state.feed.episodeId, item.id, liked);
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
  bindDanmakuControls('feed', danmakuControllers.feed, 'feed');

  document.addEventListener('keydown', event => {
    if (event.key !== 'b' && event.key !== 'B') return;
    const target = event.target;
    if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) return;
    const controller = danmakuControllers.feed;
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
  const playbackVideo = getFeedVideo();
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

function mountCommentOverlay() {
  restoreCommentOverlay();
}
function getCommentVideo() {
  return getFeedVideo();
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
  syncFeedScrollCapture();

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
  const fromVideoRect = getCommentVideo(source)?.getBoundingClientRect();
  const closeToken = commentState.requestToken + 1;
  commentState.open = false;
  commentState.requestToken++;
  syncFeedScrollCapture();
  const overlay = document.getElementById('comment-sheet-overlay');
  restoreCommentOverlay();
  document.body.classList.remove('comments-open');
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

  const slot = feedSlotForVideo(feedVideo);
  const isFeedBuffering = state.currentView === 'recommend' && !state.feed.userPaused &&
    state.feed.items.length > 0 && feedVideo && !feedVideo.ended &&
    !feedVideo.error && !slot?.failed && (!feedVideo.src || feedVideo.readyState < 3 || slot?.mediaWaiting);

  if (!isFeedBuffering) {
    if (feedBufferingTimer) clearTimeout(feedBufferingTimer);
    feedBufferingTimer = null;
    if (loader) loader.style.display = 'none';
  } else if (loader && loader.style.display === 'none' && !feedBufferingTimer) {
    const index = state.feed.index;
    feedBufferingTimer = setTimeout(() => {
      feedBufferingTimer = null;
      if (state.currentView === 'recommend' && state.feed.index === index && !state.feed.userPaused) {
        loader.style.display = 'flex';
      }
      updateFeedPlayIndicator();
    }, 200);
  }
  loader?.setAttribute('aria-label', '加载中');
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
      wakeFeedWarmup(true);
      if (window.ambientGlow) {
        window.ambientGlow.pause();
      }
      updateFeedPlayIndicator();
    });

    video.addEventListener('playing', () => {
      if (video !== getFeedVideo()) return;
      wakeFeedWarmup();
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
          if (playerModal && playerModal.style.display !== 'none') return;
      persistFeedProgress();
      state.feed.playbackPositions.delete(
        feedPlaybackKey(state.feed.seriesId, state.feed.episodeNum, state.feed.quality)
      );
      const total = state.feed.totalEpisodes || (state.feed.episodes ? state.feed.episodes.length : 1);
      if (state.feed.episodeNum < total) {
        const nextNum = state.feed.episodeNum + 1;
        enterSeriesPlayer(state.feed.seriesId, nextNum);
      } else {
        const seriesId = state.feed.seriesId;
        playNextSeason(seriesId, 'feed').then(started => {
          if (started || state.feed.seriesId !== seriesId) return;
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
      const isFull = feedContainer?.classList.contains('feed-fullscreen');
      if (isFull) {
        feedContainer?.classList.remove('feed-fullscreen');
        if (document.fullscreenElement) {
          document.exitFullscreen?.().catch(() => {});
        }
      } else {
        feedContainer?.classList.add('feed-fullscreen');
      }
      schedulePlaybackVideoFit();
      try {
        if (window.mygo?.window?.toggleFullScreen) {
          window.mygo.window.toggleFullScreen();
        } else if (window.mygo?.currentWindow?.toggleFullScreen) {
          window.mygo.currentWindow.toggleFullScreen();
        } else if (window.nativeMediaNotify) {
          window.nativeMediaNotify('ToggleFullscreen');
        } else {
          window.mygo?.call?.('NativeShell.ToggleFullscreen');
        }
      } catch (e) {
        console.warn('toggleFullScreen failed:', e);
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
      if (!state.feed.seriesId || likeBtn.disabled) return;
      const { seriesId, episodeId, seriesTitle, seriesCover, episodeNum } = state.feed;
      likeBtn.disabled = true;
      try {
        const isNowLiked = await apiCall('ToggleLike',
          seriesId, episodeId, seriesTitle, seriesCover, episodeNum
        );
        applyLikeState(seriesId, episodeId, Boolean(isNowLiked));
        showToast(isNowLiked ? '已点赞' : '已取消点赞');
      } catch (err) {
        showToast('点赞失败: ' + err);
      } finally {
        likeBtn.disabled = false;
      }
    });
  }

  if (favBtn) {
    favBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      if (!state.feed.seriesId || favBtn.disabled) return;
      const { seriesId, seriesTitle, seriesCover, totalEpisodes } = state.feed;
      favBtn.disabled = true;
      try {
        const isNowFav = await apiCall('ToggleFavorite',
          seriesId, seriesTitle, seriesCover, '', totalEpisodes
        );
        applyFavoriteState(seriesId, Boolean(isNowFav));
        showToast(isNowFav ? '已加入收藏' : '已取消收藏');
      } catch (err) {
        showToast('收藏失败: ' + err);
      } finally {
        favBtn.disabled = false;
      }
    });
  }

  function updateCatBarGlider() {
    const bar = document.getElementById('feed-cat-bar');
    if (!bar) return;
    let glider = document.getElementById('feed-cat-glider');
    if (!glider) {
      glider = document.createElement('div');
      glider.id = 'feed-cat-glider';
      glider.className = 'cat-bar-glider';
      bar.prepend(glider);
    }
    const activeChip = bar.querySelector('.cat-chip.active');
    if (!activeChip) return;
    const left = activeChip.offsetLeft;
    const width = activeChip.offsetWidth;
    glider.style.transform = `translateX(${left}px)`;
    glider.style.width = `${width}px`;
  }
  window.updateCatBarGlider = updateCatBarGlider;
  window.addEventListener('resize', updateCatBarGlider);
  requestAnimationFrame(updateCatBarGlider);

  if (catBar) {
    catBar.querySelectorAll('.cat-chip').forEach(chip => {
      chip.addEventListener('click', () => {
        catBar.querySelectorAll('.cat-chip').forEach(c => c.classList.remove('active'));
        chip.classList.add('active');
        updateCatBarGlider();
        const channel = chip.dataset.channel || '';
        loadFeed(true, channel);
      });
    });
  }

  const recommendView = document.getElementById('view-recommend');
  waitForMygoBridge().then(runtime => {
    if (!recommendView || !runtime?.on) return;
    runtime.on('scrollGesture', handleNativeFeedScroll);
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
      if (!shouldCaptureFeedScroll() || e.target?.closest?.('.feed-playlist-drawer, .episodes-grid, .speed-menu, .drawer, #comment-sheet-overlay')) return;
      e.preventDefault();
      if (nativeFeedScrollReady) return;

      const deltaUnit = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? 100 : 1;
      const deltaY = e.deltaY * deltaUnit;
      if (!Number.isFinite(deltaY) || deltaY === 0 || Math.abs(e.deltaX || 0) > Math.abs(e.deltaY) * 1.5) return;
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
      feedLastFallbackScrollAt = Date.now();
      requestFeedSwitch(dir, 'wheel');
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
      if (!shouldCaptureFeedScroll()) return;

      const minVerticalDistance = Math.max(60, feedStage.clientHeight * 0.08);
      const isVertical = Math.abs(diffY) > Math.abs(diffX) * 1.5;
      if (Math.abs(diffY) >= minVerticalDistance && isVertical && duration < 800) {
        requestFeedSwitch(diffY < 0 ? 1 : -1, 'pointer');
      }
    });
    feedStage.addEventListener('pointercancel', () => {
      pointerStart = null;
    });
  }

  document.addEventListener('keydown', (e) => {
    if (state.currentView === 'recommend' && ['ArrowDown', 'ArrowUp'].includes(e.code) && !e.repeat) {
      recordFeedDiagnostic('key-received', { reason: e.target?.tagName || 'document', inputSource: 'keyboard' });
    }
  }, true);

  document.addEventListener('keydown', (e) => {
    const targetTag = e.target.tagName;
    const typing = targetTag === 'INPUT' || targetTag === 'TEXTAREA' || e.target.isContentEditable;
    if (!shouldCaptureFeedScroll() || typing) {
      if (state.currentView === 'recommend' && ['ArrowDown', 'ArrowUp'].includes(e.code) && !e.repeat) {
        recordFeedDiagnostic('navigation-ignored', { reason: typing ? 'editable-focus' : 'overlay-open', inputSource: 'keyboard' });
      }
      return;
    }

    switch (e.code) {
      case 'ArrowDown':
        e.preventDefault();
        requestFeedSwitch(1, 'keyboard', Boolean(e.repeat));
        break;
      case 'ArrowUp':
        e.preventDefault();
        requestFeedSwitch(-1, 'keyboard', Boolean(e.repeat));
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

async function playNextSeason(seriesId, source = 'feed') {
  if (!seriesId || state.feed.seasonSwitching) return false;
  state.feed.seasonSwitching = true;
  try {
    const next = await apiCall('GetNextSeason', seriesId);
    if (!next?.id || state.feed.seriesId !== seriesId) return false;
    await enterSeriesPlayer(next.id, 1);
    return true;
  } catch (err) {
    console.warn('load next season failed:', err);
    return false;
  } finally {
    state.feed.seasonSwitching = false;
  }
}

function formatTime(sec) {
  if (!Number.isFinite(Number(sec)) || Number(sec) <= 0) return '00:00';
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}
