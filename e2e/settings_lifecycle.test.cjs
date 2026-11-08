const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');

function settingsHarness({ profile = {loggedIn: false, nickname: 'Guest', userId: 'anonymous'} } = {}) {
  const calls = [];
  const h_profile = profile;
  const nodes = new Map();
  const node = id => {
    if (!nodes.has(id)) {
      nodes.set(id, {
        id, style: {}, textContent: '', value: '', disabled: false,
        classList: {
          values: new Set(),
          add(...names) { names.forEach(name => this.values.add(name)); },
          remove(...names) { names.forEach(name => this.values.delete(name)); },
          toggle(name, force) { if (force === undefined) { if (this.values.has(name)) this.values.delete(name); else this.values.add(name); } else if (force) this.values.add(name); else this.values.delete(name); },
          contains(name) { return this.values.has(name); },
        },
        _timer: undefined,
        handlers: {}, querySelector(selector) { if (selector === 'span') return {textContent: ''}; return null; }, focus() {},
        addEventListener(name, handler) { this.handlers[name] = handler; },
      });
    }
    return nodes.get(id);
  };
  for (const id of [
    'settings-nickname', 'settings-status-indicator', 'settings-login-badge', 'settings-uid',
    'settings-avatar', 'btn-logout', 'btn-show-login', 'settings-cache-size', 'settings-device-id',
    'settings-import-banner', 'import-banner-desc', 'btn-settings-clear-cache', 'btn-settings-reset-device',
    'btn-refresh-profile', 'settings-login-panel', 'btn-cancel-login', 'btn-paste-clipboard',
    'login-cookie-input', 'login-token-input', 'btn-submit-login', 'btn-import-old-account', 'toast'
  ]) node(id);
  node('settings-login-panel').style.display = 'none';

  const runtime = {
    console, clearTimeout, setTimeout,
    performance: {now: () => Date.now()},
    document: {
      getElementById: node,
      querySelectorAll: () => [],
    },
    navigator: {clipboard: {readText: async () => 'sessionid=clipboard'}},
    window: {},
    nativeConfirm: async (message, detail, action) => {
      calls.push(['confirm', action, message]);
      return runtime.confirmResult !== false;
    },
    confirmResult: true,
    state: {currentView: 'settings', profile: null, feed: {channel: '推荐', preloadMap: new Map(), playbackInfoLoads: new Map(), playbackPositions: new Map(), retainedQualities: new Map()}},
    feedSlots: [{id: 0, ready: true, streamUrl: 'current'}],
    feedSlotCurr: 0,
    feedPlaybackGeneration: 0,
    feedPreloadRevision: 0,
    __api: (method, ...args) => {
      calls.push([method, ...args]);
      switch (method) {
        case 'GetProfile': return h_profile;
        case 'GetCacheSize': return runtime.cacheSizes.length ? runtime.cacheSizes.shift() : 1048576;
        case 'GetDeviceInfo': return {deviceId: 'device-e2e'};
        case 'CheckExistingAccount': return {exists: true, nickname: 'Old Account', userId: '710000000000001'};
        case 'ClearCache': return 2097152;
        default: return true;
      }
    },
    cacheSizes: [0],
  };
  vm.createContext(runtime);
  vm.runInContext('var apiCall = async (...args) => __api(...args);', runtime);
  vm.runInContext('var playFeedIndex = async () => {}; var scheduleFeedWarmup = async () => {}; var cancelScheduledFeedWarmup = () => {}; var feedWarmupToken = 0; var feedWarmupCenterKey = String(); var feedWarmupSession = null; var clearFeedSlot = slot => { slot.ready = false; slot.video = {src: slot.video?.src}; };', runtime);

  const load = name => {
    const start = source.search(new RegExp(`(?:async )?function ${name}[(]`));
    assert.ok(start >= 0, `missing ${name}`);
    let depth = 0, index = source.indexOf('{', start), end = index;
    for (; index < source.length; index++) {
      if (source[index] === '{') depth++;
      if (source[index] === '}') {
        depth--;
        if (depth === 0) { end = index + 1; break; }
      }
    }
    vm.runInContext(source.slice(start, end), runtime);
  };
  for (const name of [
    'revealToast', 'showToast', 'loadProfile', 'loadSettings', 'clearMediaCache',
    'initSettingsEvents', 'updateFeedPreloadWindow'
  ]) load(name);

  vm.runInContext('initSettingsEvents();', runtime);
  return {runtime, calls, node};
}

test('settings loads anonymous profile, storage, device and legacy import banner', async () => {
  const h = settingsHarness(); const {runtime, calls} = h;
  await runtime.loadSettings().catch(error => console.error('debug', error));
  assert.equal(h.node('settings-nickname').textContent, 'Guest');
  assert.equal(h.node('settings-cache-size').textContent, '0.0 MB');
  assert.equal(h.node('settings-device-id').textContent, 'device-e2e');
  assert.equal(h.node('settings-import-banner').style.display, 'flex');
  assert.match(h.node('import-banner-desc').textContent, /Old Account/);
  assert.equal(h.node('btn-logout').style.display, 'none');
  assert.equal(calls.filter(([method]) => method === 'CheckExistingAccount').length, 1);
});

test('logged-in settings hides legacy import and logout is visible', async () => {
  const h = settingsHarness({profile: {loggedIn: true, nickname: 'Henry', userId: '710000000000001'}}); const {runtime} = h;
  await runtime.loadSettings();
  assert.equal(h.node('settings-login-badge').textContent, '已登录官方账号');
  assert.equal(h.node('settings-import-banner').style.display, 'none');
  assert.equal(h.node('btn-logout').style.display, 'inline-flex');
});

test('clear cache refreshes remaining size and preserves the active decoder', async () => {
  const h = settingsHarness(); const {runtime, calls} = h;
  await h.node('btn-settings-clear-cache').handlers.click();
  assert.deepEqual(calls.filter(([method]) => ['ClearCache', 'GetCacheSize', 'UpdateFeedPreloadWindow'].includes(method)).map(([method]) => method),
    ['UpdateFeedPreloadWindow', 'ClearCache', 'GetCacheSize']);
  assert.equal(h.node('settings-cache-size').textContent, '0.0 MB');
  assert.match(h.node('toast').textContent, /已清理 2\.0 MB/);
  assert.equal(runtime.feedSlots[0].streamUrl, 'current');
});

test('reset device requires destructive confirmation and reloads settings after success', async () => {
  const h = settingsHarness(); const {runtime, calls} = h;
  await h.node('btn-settings-reset-device').handlers.click();
  assert.deepEqual(calls[0], ['confirm', '重置设备', '确认重置匿名设备指纹？']);
  assert.ok(calls.some(([method]) => method === 'ResetDevice'));
  assert.match(h.node('toast').textContent, /已成功重置匿名设备/);

  runtime.confirmResult = false;
  calls.length = 0;
  await h.node('btn-settings-reset-device').handlers.click();
  assert.equal(calls.some(([method]) => method === 'ResetDevice'), false);
});

test('login panel toggles, validates empty credentials and submits parsed JSON clipboard', async () => {
  const h = settingsHarness(); const {runtime, calls} = h;
  const panel = h.node('settings-login-panel');
  h.node('btn-show-login').handlers.click();
  assert.equal(panel.style.display, 'flex');
  h.node('btn-cancel-login').handlers.click();
  assert.equal(panel.style.display, 'none');

  h.node('btn-submit-login').handlers.click();
  assert.match(h.node('toast').textContent, /请输入有效的 Cookie/);
  assert.equal(calls.some(([method]) => method === 'LoginWithCookies'), false);

  h.node('btn-paste-clipboard').handlers.click();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(h.node('login-cookie-input').value, 'sessionid=clipboard');

  h.node('btn-submit-login').handlers.click();
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(calls.filter(([method]) => method === 'LoginWithCookies').at(-1), ['LoginWithCookies', 'sessionid=clipboard', '']);
});
