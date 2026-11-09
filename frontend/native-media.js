// Uses the same feed/player controllers as the standalone frontend. The native
// shell owns browsing/navigation; source identity and media stay in one pipeline.
(() => {
  if (!window.myguoNativeMedia) return;
  document.addEventListener('DOMContentLoaded', () => {
    document.body.classList.add('native-media');
    observeFeedChrome();
    notify('MediaReady');
    const width = Number(localStorage.getItem('hongguo_sidebar_width'));
    if (width >= 160 && width <= 420) notify('ImportSidebarWidth', width);
  });
  let changingScene = false;
  let activeSession = 0;
  const notify = (method, ...args) => window.mygo.call(`NativeShell.${method}`, ...args)
    .catch(error => console.error('[native-shell]', method, error));
  window.nativeMediaSession = () => Number(activeSession);
  window.nativeMediaNotify = notify;
  const closeNativeMedia = () => {
    const session = Number(activeSession);
    if (!changingScene) notify('ClosePlayer', Number.isFinite(session) ? session : 0);
  };
  window.nativeMediaClosed = closeNativeMedia;
  function observeFeedChrome() {
    const bar = document.getElementById('feed-cat-bar');
    const feed = document.getElementById('feed-container');
    const page = document.getElementById('view-recommend');
    if (!bar || !feed || !page) return;
    let last = '';
    const sync = () => {
      const rect = bar.getBoundingClientRect();
      const visible = state.currentView === 'recommend'
        && !document.fullscreenElement
        && !feed.classList.contains('clearscreen')
        && !bar.classList.contains('clearscreen')
        && getComputedStyle(bar).display !== 'none';
      const rightInset = Math.max(0, Math.round(window.innerWidth - 2 * (rect.left + rect.width / 2)));
      const value = {visible, rightInset};
      const signature = JSON.stringify(value);
      if (signature === last) return;
      last = signature;
      notify('FeedChromeChanged', value);
    };
    const observer = new MutationObserver(sync);
    for (const node of [document.body, feed, page, bar]) {
      if (node) observer.observe(node, {attributes: true, attributeFilter: ['class', 'style']});
    }
    window.addEventListener('resize', sync);
    document.addEventListener('fullscreenchange', sync);
    sync();
  }
  window.nativeMediaEvent = payload => {
    try {
      switch (payload.event) {
        case 'stop':
          changingScene = true;
          // Change view first so closePlayer cannot resume the hidden feed.
          switchView('native-idle', false);
          changingScene = false;
          break;
        case 'recommend':
          changingScene = true;
          switchView('native-idle', false);
          changingScene = false;
          // Preserve the selected native channel across navigation and renderer
          // recovery. loadRecommend keeps using the existing feed loader.
          switchView('recommend', false);
          ensureRecommendColdStart(payload.channel).catch(error => {
            console.error('recommend feed cold start failed:', error);
            showToast('加载推荐失败，请重试');
          });
          break;
        case 'settings':
          changingScene = true;
          switchView('native-idle', false);
          changingScene = false;
          switchView('settings', false);
          break;
        case 'play':
          activeSession = payload.session || 0;
          switchView('recommend', false);
          enterSeriesPlayer(payload.seriesId, payload.episodeNum || 1)
            .then(() => {
              const resumeAt = Number(payload.resumeAt);
              if (Number.isFinite(resumeAt) && resumeAt > 0) {
                const feedVideo = getFeedVideo();
                if (feedVideo && Number.isFinite(feedVideo.duration)) feedVideo.currentTime = resumeAt;
              }
            })
            .catch(error => {
              console.error('[native-media] enterSeriesPlayer failed:', error);
              showToast(String(error));
              closeNativeMedia();
            });
          break;
        case 'detail':
          changingScene = true;
          switchView('native-idle', false);
          changingScene = false;
          switchView('detail', false);
          break;
        case 'command': handleMediaCommand(payload); break;
        case 'quality': applyPlayerControlPayload(payload); break;
        case 'speed': applyPlayerControlPayload(payload); break;
        case 'episode': applyPlayerControlPayload(payload); break;
      }
    } catch (error) {
      changingScene = false;
      console.error('[native-media]', error);
      showToast(String(error));
    }
  };
})();

function applyPlayerControlPayload(payload) {
  if (payload.event === 'command') {
    switch (payload.command) {
      case 'quality':
        if (typeof payload.value === 'string') changeFeedPlaybackQuality(payload.value);
        return;
      case 'speed': {
        const speed = Number(payload.value);
        if ([0.75, 1, 1.25, 1.5, 2].includes(speed)) {
          state.feed.playbackSpeed = speed;
          const feedVideo = getFeedVideo();
          if (feedVideo) feedVideo.playbackRate = speed;
        }
        return;
      }
      case 'episode': {
        const episode = Number(payload.value);
        if (Number.isInteger(episode) && episode >= 1 && episode <= state.feed.totalEpisodes) enterSeriesPlayer(state.feed.seriesId, episode);
        return;
      }
      default:
        handleMediaCommand(payload);
    }
    return;
  }
  if (payload.event === 'quality') changeFeedPlaybackQuality(payload.value);
  if (payload.event === 'speed') applyPlayerControlPayload({event: 'command', command: 'speed', value: payload.value});
  if (payload.event === 'episode') applyPlayerControlPayload({event: 'command', command: 'episode', value: payload.value});
}
