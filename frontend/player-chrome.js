// One source of truth for the chrome shared by the recommend feed and the
// series player. Controls are mounted directly into the consuming stage so
// both players use identical layout rules without wrapper flex anomalies.
function playerChromeId(prefix, name) {
  return `${prefix}-${name}`;
}

function playerControlIcons() {
  return {
    play: '<svg class="icon-play" viewBox="0 0 24 24"><path fill="currentColor" d="M8 5v14l11-7z"/></svg>',
    pause: '<svg class="icon-pause" viewBox="0 0 24 24" style="display:none;"><path fill="currentColor" d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>',
    prev: '<svg viewBox="0 0 24 24"><path fill="currentColor" d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/></svg>',
    next: '<svg viewBox="0 0 24 24"><path fill="currentColor" d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/></svg>',
    episodes: '<svg viewBox="0 0 24 24"><path fill="currentColor" d="M3 13h2v-2H3v2zm0 4h2v-2H3v2zm0-8h2V7H3v2zm4 4h14v-2H7v2zm0 4h14v-2H7v2zM7 7v2h14V7H7z"/></svg>',
    pip: '<svg viewBox="0 0 24 24"><path fill="currentColor" d="M19 7h-8v6h8V7zm2-4H3c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h18c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm0 16.01H3V4.99h18v14.02z"/></svg>',
    fullscreen: '<svg viewBox="0 0 24 24"><path fill="currentColor" d="M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z"/></svg>',
    danmaku: '<svg viewBox="0 0 24 24"><path fill="currentColor" d="M4 5h16a2 2 0 012 2v8a2 2 0 01-2 2H9.8l-3.4 2.8A1 1 0 014.8 19v-2H4a2 2 0 01-2-2V7a2 2 0 012-2zm0 2v8h16V7H4zm3 2h7v1.6H7V9zm0 3.4h10V14H7v-1.6z"/></svg>',
    settings: '<svg viewBox="0 0 24 24"><path fill="currentColor" d="M12 15.5A3.5 3.5 0 1112 8.5a3.5 3.5 0 010 7zm7.4-2.6l1.8 1.4c.2.1.2.4.1.6l-1.7 3c-.1.2-.3.3-.6.2l-2.1-.9c-.5.4-1 .7-1.6.9l-.3 2.3c0 .2-.2.4-.5.4h-3.4c-.2 0-.4-.2-.5-.4l-.3-2.3c-.6-.2-1.1-.5-1.6-.9l-2.1.9c-.2.1-.5 0-.6-.2l-1.7-3c-.1-.2-.1-.4.1-.6l1.8-1.4a7 7 0 010-1.8L4.5 9.7c-.2-.1-.2-.4-.1-.6l1.7-3c.1-.2.3-.3.6-.2l2.1.9c.5-.4 1-.7 1.6-.9l.3-2.3c0-.2.2-.4.5-.4h3.4c.2 0 .4.2.5.4l.3 2.3c.6.2 1.1.5 1.6.9l2.1-.9c.2-.1.5 0 .6.2l1.7 3c.1.2.1.4-.1.6l-1.8 1.4a7 7 0 010 1.8z"/></svg>',
    send: '<svg viewBox="0 0 24 24"><path fill="currentColor" d="M3 20.5v-17L22 12 3 20.5zm2-3.2L17.2 12 5 6.7v3.8L12 12l-7 1.5v3.8z"/></svg>',
    like: '<svg viewBox="0 0 24 24" width="28" height="28"><path fill="currentColor" d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg>',
    comment: '<svg viewBox="0 0 24 24" width="26" height="26"><path fill="currentColor" d="M12 3c5.1 0 9 3.36 9 7.5S17.1 18 12 18c-.86 0-1.7-.09-2.48-.27L5 20v-3.83C3.14 14.91 3 12.98 3 10.5C3 6.36 6.9 3 12 3z"/></svg>',
    favorite: '<svg viewBox="0 0 24 24" width="28" height="28"><path fill="currentColor" d="M12 17.27L18.18 21l-1.64-7.03L22 9.24l-7.19-.61L12 2L9.19 8.63L2 9.24l5.46 4.73L5.82 21z"/></svg>',
    close: '<svg viewBox="0 0 24 24"><path fill="currentColor" d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>'
  };
}

function mountPlayerChrome({ prefix, target, drawerTitle = '选集播放' }) {
  if (!target || target.dataset.chromeMounted === 'true') return null;
  const id = name => playerChromeId(prefix, name);
  const icons = playerControlIcons();
  const template = document.createElement('template');
  template.innerHTML = `
    <div class="feed-rail" id="${id('rail')}">
      <button class="rail-btn like" id="${id('like')}" title="点赞本集">
        <div class="rail-icon">${icons.like}</div>
        <span class="rail-label">点赞</span>
      </button>
      <button class="rail-btn" id="${id('comment')}" title="查看评论">
        <div class="rail-icon">${icons.comment}</div>
        <span class="rail-label" id="${id('comment-count')}">评论</span>
      </button>
      <button class="rail-btn fav" id="${id('fav')}" title="收藏短剧">
        <div class="rail-icon">${icons.favorite}</div>
        <span class="rail-label" id="${id('fav-label')}">收藏</span>
      </button>
    </div>

    <div class="feed-controls">
      <div class="progress-bar-wrap" id="${id('progress-wrap')}">
        <div class="progress-bar-bg">
          <div class="progress-bar-buffer" id="${id('progress-buffer')}"></div>
          <div class="progress-bar-current" id="${id('progress-current')}"></div>
        </div>
        <div class="progress-time-tooltip" id="${id('time-tooltip')}" style="display: none;">00:00</div>
      </div>

      <div class="controls-row">
        <div class="controls-left">
          <button class="ctrl-btn" id="${id('play')}" title="播放/暂停 (Space)">${icons.play}${icons.pause}</button>
          <button class="ctrl-btn" id="${id('prev-ep')}" title="上一集">${icons.prev}</button>
          <button class="ctrl-btn" id="${id('next-ep')}" title="下一集">${icons.next}</button>
          <span class="time-display" id="${id('time-display')}">00:00 / 00:00</span>
        </div>

        <div class="controls-right">
          <div class="quality-selector">
            <button class="ctrl-text-btn" id="${id('quality')}" title="清晰度" aria-haspopup="true" aria-expanded="false">720P</button>
            <div class="quality-menu" id="${id('quality-menu')}" style="display: none;"></div>
          </div>
          <div class="speed-selector">
            <button class="ctrl-text-btn" id="${id('speed')}">1.0x</button>
            <div class="speed-menu" id="${id('speed-menu')}" style="display: none;">
              <div class="speed-opt" data-speed="0.75">0.75x</div>
              <div class="speed-opt active" data-speed="1.0">1.0x</div>
              <div class="speed-opt" data-speed="1.25">1.25x</div>
              <div class="speed-opt" data-speed="1.5">1.5x</div>
              <div class="speed-opt" data-speed="2.0">2.0x</div>
            </div>
          </div>
          <button class="ctrl-btn danmaku-toggle" id="${id('danmaku')}" title="弹幕开关 (B)">${icons.danmaku}</button>
          <button class="ctrl-btn danmaku-settings-toggle" id="${id('danmaku-settings')}" title="弹幕设置" aria-controls="${id('danmaku-settings-panel')}" aria-expanded="false">${icons.settings}</button>
          <button class="ctrl-btn episodes-toggle" id="${id('drawer-toggle')}" title="选集" aria-controls="${id('playlist-drawer')}" aria-expanded="false">
            ${icons.episodes}<span>选集</span>
          </button>
          <button class="ctrl-btn" id="${id('pip')}" title="画中画 (P)">${icons.pip}</button>
          <button class="ctrl-btn" id="${id('fullscreen')}" title="全屏 (F)">${icons.fullscreen}</button>
        </div>
      </div>
    </div>

    <div class="danmaku-composer" id="${id('danmaku-composer')}">
      <input class="danmaku-input" id="${id('danmaku-input')}" maxlength="200" placeholder="发个弹幕见证当下" autocomplete="off">
      <button class="ctrl-text-btn danmaku-send" id="${id('danmaku-send')}">${icons.send}<span>发送</span></button>
    </div>

    <div class="danmaku-settings-panel" id="${id('danmaku-settings-panel')}" role="dialog" aria-label="弹幕设置">
      <div class="danmaku-settings-header">
        <span>弹幕设置</span>
        <button class="icon-btn" id="${id('danmaku-settings-close')}" title="关闭弹幕设置">${icons.close}</button>
      </div>
      <label class="danmaku-setting">
        <span class="danmaku-setting-label">字体大小</span>
        <input type="range" min="14" max="40" step="1" data-danmaku-setting="font-size">
        <span class="danmaku-setting-value" data-danmaku-value="font-size"></span>
      </label>
      <label class="danmaku-setting">
        <span class="danmaku-setting-label">字体粗细</span>
        <input type="range" min="400" max="900" step="100" data-danmaku-setting="font-weight">
        <span class="danmaku-setting-value" data-danmaku-value="font-weight"></span>
      </label>
      <label class="danmaku-setting">
        <span class="danmaku-setting-label">弹幕描边</span>
        <input type="checkbox" data-danmaku-setting="outline">
        <span class="danmaku-setting-value" data-danmaku-value="outline"></span>
      </label>
      <label class="danmaku-setting">
        <span class="danmaku-setting-label">显示行数</span>
        <input type="range" min="0" max="15" step="1" data-danmaku-setting="rows">
        <span class="danmaku-setting-value" data-danmaku-value="rows"></span>
      </label>
    </div>

    <aside class="feed-playlist-drawer" id="${id('playlist-drawer')}">
      <div class="drawer-header">
        <div class="drawer-header-left">
          <h4 id="${id('drawer-title')}">${drawerTitle}</h4>
          <span class="drawer-count" id="${id('drawer-count')}">共 0 集</span>
        </div>
        <button class="icon-btn" id="${id('close-drawer')}" title="关闭">${icons.close}</button>
      </div>
      <div class="episodes-grid" id="${id('drawer-grid')}"></div>
      <div class="drawer-collapse-wrap" id="${id('drawer-collapse-wrap')}" style="display: none;">
        <button class="drawer-collapse" id="${id('drawer-collapse')}" type="button" aria-expanded="false">
          <span data-collapse-text>展开全部</span>
          <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
        </button>
      </div>
    </aside>
  `;

  const fragment = template.content;
  const names = [
    'like', 'comment', 'comment-count', 'fav', 'fav-label', 'rail',
    'danmaku', 'danmaku-settings', 'danmaku-settings-panel', 'danmaku-settings-close',
    'danmaku-composer', 'danmaku-input', 'danmaku-send',
    'progress-wrap', 'progress-buffer', 'progress-current', 'time-tooltip',
    'play', 'prev-ep', 'next-ep', 'time-display', 'quality', 'quality-menu',
    'speed', 'speed-menu', 'drawer-toggle', 'pip', 'fullscreen',
    'playlist-drawer', 'drawer-title', 'drawer-count', 'drawer-grid', 'close-drawer',
    'drawer-collapse-wrap', 'drawer-collapse'
  ];
  const elements = Object.fromEntries(names.map(name => [name, fragment.querySelector(`#${CSS.escape(id(name))}`)]));
  target.appendChild(fragment);
  target.dataset.chromeMounted = 'true';
  return { root: target, elements, id };
}
