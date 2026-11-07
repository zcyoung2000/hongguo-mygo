// Uses the same feed/player controllers as the standalone frontend. The native
// shell owns browsing/navigation; source identity and media stay in one pipeline.
(() => {
  if (!window.myguoNativeMedia) return;
  document.addEventListener('DOMContentLoaded', () => {
    document.body.classList.add('native-media');
    observeFeedChrome();
    observePlayerChrome();
    notify('MediaReady');
    const width = Number(localStorage.getItem('hongguo_sidebar_width'));
    if (width >= 160 && width <= 420) notify('ImportSidebarWidth', width);
  });
  let changingScene = false;
  let activeSession = 0;
  const notify = (method, ...args) => window.mygo.call(`NativeShell.${method}`, ...args)
    .catch(error => console.error('[native-shell]', method, error));
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
    const player = document.getElementById('player-modal');
    if (!bar || !feed || !page) return;
    let last = '';
    const sync = () => {
      const rect = bar.getBoundingClientRect();
      const visible = state.currentView === 'recommend'
        && !document.fullscreenElement
        && (!player || player.style.display === 'none')
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
    for (const node of [document.body, feed, page, bar, player]) {
      if (node) observer.observe(node, {attributes: true, attributeFilter: ['class', 'style']});
    }
    window.addEventListener('resize', sync);
    document.addEventListener('fullscreenchange', sync);
    sync();
  }
  const frameOf = node => {
    if (!node) return {x: 0, y: 0, w: 0, h: 0};
    const rect = node.getBoundingClientRect();
    return {
      x: Math.max(0, Math.round(rect.left)),
      y: Math.max(0, Math.round(rect.top)),
      w: Math.round(rect.width),
      h: Math.round(rect.height)
    };
  };
  function observePlayerChrome() {
    const modal = document.getElementById('player-modal');
    if (!modal) return;
    let last = '';
    const hidden = () => ({
      visible: false, controlsVisible: false, quality: frameOf(), speed: frameOf(),
      drawer: frameOf(), qualityLabel: '', qualityOptions: [], speedValue: 1, episode: 0, totalEpisodes: 0
    });
    const sync = () => {
      const value = modal.style.display === 'none' ? hidden() : (() => {
        const quality = document.getElementById('player-quality');
        const speed = document.getElementById('player-speed');
        const drawerToggle = document.getElementById('player-drawer-toggle');
        const controls = document.querySelector('#player-stage .feed-controls');
        const activeQuality = state.player.qualityOptions.find(option => option.active)
          || {label: qualityLabel(state.player.quality)};
        return {
          visible: true,
          controlsVisible: Boolean(controls && controls.getBoundingClientRect().height > 0 && getComputedStyle(controls).visibility !== 'hidden' && !modal.classList.contains('controls-hidden')),
          quality: frameOf(quality),
          speed: frameOf(speed),
          drawer: frameOf(drawerToggle),
          qualityLabel: activeQuality.label || '720P',
          qualityOptions: state.player.qualityOptions || [],
          speedValue: Number(state.player.playbackSpeed) || 1,
          episode: Number(state.player.episodeNum) || 1,
          totalEpisodes: Number(state.player.totalEpisodes) || 0
        };
      })();
      const signature = JSON.stringify(value);
      if (signature === last) return;
      last = signature;
      notify('PlayerChromeChanged', value);
    };
    window.__syncPlayerChromeState = sync;
    const observer = new MutationObserver(sync);
    for (const node of [modal, document.body]) observer.observe(node, {attributes: true, attributeFilter: ['class', 'style']});
    window.addEventListener('resize', sync);
    sync();
  }
  window.nativeMediaEvent = payload => {
    try {
      switch (payload.event) {
        case 'stop':
          changingScene = true;
          // Change view first so closePlayer cannot resume the hidden feed.
          switchView('native-idle', false);
          closePlayer();
          changingScene = false;
          break;
        case 'recommend':
          changingScene = true;
          switchView('native-idle', false);
          closePlayer();
          changingScene = false;
          // Preserve the selected native channel across navigation and renderer
          // recovery. loadRecommend keeps using the existing feed loader.
          if (typeof payload.channel === 'string' && ['', '关注', '漫剧', '真人剧'].includes(payload.channel)
              && payload.channel !== state.feed.channel) {
            state.feed.channel = payload.channel;
            state.feed.items = [];
          }
          switchView('recommend', false);
          break;
        case 'settings':
          changingScene = true;
          switchView('native-idle', false);
          closePlayer();
          changingScene = false;
          switchView('settings', false);
          break;
        case 'play':
          activeSession = payload.session || 0;
          switchView('native-idle', false);
          openPlayer(payload.seriesId, payload.episodeNum || 1, payload.resumeAt ?? null)
            .catch(error => {
              console.error('[native-media] openPlayer failed:', error);
              showToast(String(error));
              closeNativeMedia();
            });
          break;
        case 'detail':
          changingScene = true;
          switchView('native-idle', false);
          closePlayer();
          changingScene = false;
          switchView('detail', false);
          if (payload.seriesId) { state.detail.id = payload.seriesId; loadSeriesDetail(payload.seriesId); }
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
        if (typeof payload.value === 'string') changePlaybackQuality(payload.value);
        return;
      case 'speed': {
        const speed = Number(payload.value);
        if ([0.75, 1, 1.25, 1.5, 2].includes(speed)) {
          state.player.playbackSpeed = speed;
          video.playbackRate = speed;
          window.__syncPlayerChromeState?.();
        }
        return;
      }
      case 'episode': {
        const episode = Number(payload.value);
        if (Number.isInteger(episode) && episode >= 1 && episode <= state.player.totalEpisodes) playEpisodeNumber(episode);
        return;
      }
      default:
        handleMediaCommand(payload);
    }
    return;
  }
  if (payload.event === 'quality') changePlaybackQuality(payload.value);
  if (payload.event === 'speed') applyPlayerControlPayload({event: 'command', command: 'speed', value: payload.value});
  if (payload.event === 'episode') applyPlayerControlPayload({event: 'command', command: 'episode', value: payload.value});
}
