(() => {
  const video = document.createElement('video');
  video.className = 'native-player-video';
  video.playsInline = true;
  video.controls = true;
  video.crossOrigin = 'anonymous';
  document.body.appendChild(video);
  const status = document.createElement('div');
  status.setAttribute('role', 'status');
  status.className = 'native-player-status';
  document.body.appendChild(status);

  let currentSeries = '';
  let currentEpisode = '';
  let session = 0;
  let saveTimer = null;
  let pendingSeek = null;
  const progressPayload = () => ({
    position: Number.isFinite(video.currentTime) ? video.currentTime : 0,
    duration: Number.isFinite(video.duration) ? video.duration : 0,
    playing: !video.paused && !video.ended,
    seriesId: currentSeries,
    episodeId: currentEpisode,
    session,
  });
  const saveProgress = () => {
    if (!currentSeries || !currentEpisode || !Number.isFinite(video.currentTime)) return;
    window.nativePlayer?.saveProgress(progressPayload());
  };
  const cancelSave = () => {
    if (saveTimer !== null) window.clearTimeout(saveTimer);
    saveTimer = null;
  };
  const reportProgress = () => {
    if (currentSeries && currentEpisode) window.nativePlayer?.progress(progressPayload());
  };
  const startPlaying = () => video.play().catch(error => {
    status.textContent = `无法开始播放：${error.message || error}`;
    reportProgress();
  });
  video.addEventListener('timeupdate', () => {
    reportProgress();
    if (saveTimer !== null || !currentEpisode) return;
    saveTimer = window.setTimeout(() => { saveTimer = null; saveProgress(); }, 5000);
  });
  video.addEventListener('play', reportProgress);
  video.addEventListener('pause', () => { reportProgress(); saveProgress(); });
  video.addEventListener('ended', () => {
    saveProgress();
    window.nativePlayer?.next(progressPayload());
  });
  video.addEventListener('loadedmetadata', () => {
    if (pendingSeek !== null) { video.currentTime = pendingSeek; pendingSeek = null; }
    reportProgress();
  });
  video.addEventListener('error', () => {
    if (currentEpisode) status.textContent = '视频加载失败，请重新选择剧集';
  });

  const play = (payload) => {
    if (!payload?.src) return;
    saveProgress();
    cancelSave();
    // Detach the outgoing session before load/pause events can arrive.
    currentSeries = ''; currentEpisode = '';
    video.pause();
    status.textContent = '';
    session = payload.session;
    pendingSeek = Math.max(0, Number(payload.resumeAt) || 0);
    video.poster = payload.poster || '';
    video.src = payload.src;
    video.load();
    currentSeries = payload.seriesId || '';
    currentEpisode = payload.episodeId || '';
    if (payload.autoplay !== false) startPlaying();
  };
  const commandHandlers = {
    resume: startPlaying,
    pause: () => video.pause(),
    stop: () => {
      saveProgress(); cancelSave();
      currentSeries = ''; currentEpisode = ''; session = 0; pendingSeek = null;
      video.pause(); video.removeAttribute('src'); video.load(); status.textContent = '';
    },
    seek: payload => {
      if (Number.isFinite(Number(payload?.position))) video.currentTime = Math.max(0, Number(payload.position));
    },
    error: payload => { status.textContent = payload.message || '播放失败'; },
  };
  window.nativePlayerEvent = (payload = {}) => {
    if (payload.event === 'play') return play(payload);
    commandHandlers[payload.event]?.(payload);
  };
  window.addEventListener('beforeunload', saveProgress);
})();
