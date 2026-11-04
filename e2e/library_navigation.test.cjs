const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const declarations = [...source.matchAll(/^(?:async )?function (\w+)\(/gm)];

function harness(view, names) {
  const nodes = new Map();
  class Element {
    constructor(id) {
      this.id = id; this.children = []; this.style = {}; this.dataset = {};
      this.listeners = {}; this.textContent = ''; this.disabled = false;
      const classes = new Set();
      this.classList = {
        add: (...items) => items.forEach(item => classes.add(item)),
        remove: (...items) => items.forEach(item => classes.delete(item)),
        contains: item => classes.has(item),
        toggle: (item, value) => {
          const active = value ?? !classes.has(item);
          if (active) classes.add(item); else classes.delete(item);
          return active;
        }
      };
    }
    set innerHTML(value) { this.html = value; this.children = []; }
    get innerHTML() { return this.html || ''; }
    appendChild(value) {
      if (value.fragment) this.children.push(...value.children);
      else this.children.push(value);
      return value;
    }
    setAttribute() {}
    addEventListener(name, callback) { this.listeners[name] = callback; }
    querySelector(selector) { return element(this.id + ':' + selector); }
    querySelectorAll(selector) {
      return selector === '.drama-card' ? this.children.filter(item => item.className?.includes('drama-card')) : [];
    }
    closest() { return null; }
  }
  const element = id => {
    if (!nodes.has(id)) nodes.set(id, new Element(id));
    return nodes.get(id);
  };
  const requests = [], toasts = [], renders = [];
  const c = vm.createContext({
    console, window: {},
    document: {
      getElementById: element, querySelector: () => null, querySelectorAll: () => [],
      addEventListener() {},
      createElement: () => new Element('created'),
      createDocumentFragment: () => Object.assign(new Element('fragment'), { fragment: true })
    },
    LIBRARY_REFRESH_INTERVAL_MS: 15000,
    state: {
      currentView: view, historyStack: [view], historyIndex: 0,
      feed: {}, player: {},
      detail: { id: 'A', requestToken: 0, data: { id: 'A', title: 'A', episodeCount: 20, isFavorite: false } },
      actor: { userId: 'A', name: 'A', avatarUrl: '', cursor: '', hasMore: false, loading: false },
      discover: { genre: 'short_play', selections: {}, items: [{ id: 'kept' }], cursor: 'page-1', hasMore: true, loading: false },
      favoritesView: {
        tab: 'favorites', favoritesList: [{ seriesId: 'A', title: 'Saved favorite' }],
        likesList: [{ seriesId: 'A', episodeId: 'episode', title: 'Saved like' }]
      },
      libraryRefreshAt: {},
      historyView: { list: [], filter: 'all', search: '' }
    },
    DEFAULT_POSTER: 'poster', discoverRequestToken: 0, discoverPanelToken: 0,
    navigateTo: next => { c.state.currentView = next; }, navigateBack() {},
    renderDramaGridSkeleton: () => '<loading>',
    renderViewError: options => Object.assign(new Element('error'), { options, type: 'error' }),
    renderViewEmpty: options => Object.assign(new Element('empty'), { options, type: 'empty' }),
    createDramaCard: item => Object.assign(new Element('card'), { item, className: 'drama-card' }),
    clearDetailSupplements() {}, renderSeriesDetail: detail => renders.push(detail),
    disconnectDiscoverObserver() {}, renderDiscoverLoadMore() {}, observeDiscoverLoadMore() {},
    discoverVisibleCapacity: () => 100,
    safeImageSrc: String, escapeHtml: String, formatStatsNumber: String,
    showToast: value => toasts.push(value),
    cancelScheduledFeedWarmup() {}, cancelPendingFeedSwitch() {}, pauseFeedPlayback() {},
    resumeFeedPlayback() {}, syncFeedScrollCapture() {}, loadRecommend() {},
    loadDiscoverFilters() {}, loadRankings() {}, loadNewReleases() {}, loadHistory() {},
    loadReservations() {}, loadSettings() {},
    apiCall: (method, ...args) => new Promise((resolve, reject) => requests.push({ method, args, resolve, reject }))
  });
  for (const name of new Set([
    'beginViewRequest', 'isViewRequestCurrent', 'invalidateViewRequests',
    'shouldRefreshLibraryData', 'markLibraryDataRefreshed', ...names
  ])) {
    const index = declarations.findIndex(match => match[1] === name);
    assert.ok(index >= 0, `missing function ${name}`);
    vm.runInContext(source.slice(declarations[index].index, declarations[index + 1]?.index), c);
  }
  return { c, requests, element, toasts, renders };
}

const navigation = ['navigateTo', 'switchView', 'updateNavButtons'];
const discovery = ['executeDiscover', 'loadMoreDiscover', 'createDramaCardsFragment'];
const collections = ['loadFavoritesAndLikes', 'renderFavoritesOrLikes', 'updateFavoritesTabsCount', 'switchFavoritesTab'];

test('late search success and failure cannot replace a newer query', async () => {
  const { c, requests, element } = harness('search-results', ['executeSearch']);
  const old = c.executeSearch('old'), current = c.executeSearch('current');
  requests[1].resolve([{ id: 'current-result' }]); await current;
  requests[0].resolve([{ id: 'old-result' }]); await old;
  assert.equal(element('search-query-header').textContent, '搜索: "current"');
  assert.equal(element('search-grid').children[0].item.id, 'current-result');
  const failed = c.executeSearch('fails'), latest = c.executeSearch('latest');
  requests[2].reject(new Error('old failure')); await failed;
  assert.equal(c.state.searchLoading, true);
  requests[3].resolve([{ id: 'latest-result' }]); await latest;
  assert.equal(element('search-grid').children[0].item.id, 'latest-result');
});

test('leaving and returning to search starts one replacement request', async () => {
  const { c, requests, element } = harness('recommend', [...navigation, 'executeSearch']);
  const pending = c.executeSearch('query');
  assert.equal(requests.length, 1, 'initial navigation must not double-fetch');
  c.switchView('recommend');
  requests[0].resolve([{ id: 'stale-result' }]); await pending;
  assert.equal(element('search-grid').children.length, 0);
  c.switchView('search-results');
  assert.equal(requests.length, 2);
  requests[1].resolve([{ id: 'restored-result' }]);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(element('search-grid').children[0].item.id, 'restored-result');
});

test('one discovery load-more action requests one page and deduplicates its cards', async () => {
  const { c, requests } = harness('discover', discovery);
  const pending = c.loadMoreDiscover();
  assert.equal(requests.length, 1);
  requests[0].resolve({ items: [{ id: 'new' }, { id: 'new' }, { id: 'kept' }], cursor: 'page-2', hasMore: true });
  await pending;
  assert.equal(requests.length, 1);
  assert.deepEqual(Array.from(c.state.discover.items, item => item.id), ['kept', 'new']);
  assert.equal(c.state.discover.hasMore, true);
});

test('initial discovery fill has a finite request budget', async () => {
  const { c } = harness('discover', discovery);
  let calls = 0;
  c.apiCall = async () => ({ items: [{ id: String(++calls) }], cursor: 'page-' + calls, hasMore: true });
  await c.executeDiscover(true);
  assert.equal(calls, 3);
  assert.equal(c.state.discover.loading, false);
});

for (const response of [
  { items: [{ id: 'new' }], cursor: 'page-1', hasMore: true },
  { items: [], cursor: 'page-2', hasMore: true },
  { items: [{ id: 'kept' }], cursor: 'page-2', hasMore: true }
]) {
  test('discovery stops automatic pagination without cursor or item progress: ' + JSON.stringify(response), async () => {
    const { c } = harness('discover', discovery);
    let calls = 0;
    c.apiCall = async () => { calls++; return response; };
    await c.loadMoreDiscover();
    await c.loadMoreDiscover();
    assert.equal(calls, 1);
    assert.equal(c.state.discover.hasMore, false);
  });
}

test('leaving discovery stops an in-flight fill and prevents follow-up requests', async () => {
  const { c, requests } = harness('discover', [...navigation, ...discovery]);
  const pending = c.executeDiscover(true);
  c.switchView('recommend');
  requests[0].resolve({ items: [{ id: 'late' }], cursor: 'next', hasMore: true });
  await pending;
  assert.equal(requests.length, 1);
  assert.equal(c.state.discover.items.length, 0);
  assert.equal(c.state.discover.loading, false);
});

test('discovery detects a cursor cycle across separate load-more actions', async () => {
  const { c, requests } = harness('discover', discovery);
  const first = c.loadMoreDiscover();
  requests[0].resolve({ items: [{ id: 'page-one' }], cursor: 'page-2', hasMore: true }); await first;
  const second = c.loadMoreDiscover();
  requests[1].resolve({ items: [{ id: 'page-two' }], cursor: 'page-1', hasMore: true }); await second;
  assert.equal(c.state.discover.hasMore, false);
  await c.loadMoreDiscover();
  assert.equal(requests.length, 2);
});

test('a previous discovery filter cannot replace the current filter results', async () => {
  const { c, requests } = harness('discover', discovery);
  c.discoverVisibleCapacity = () => 1;
  c.state.discover.selections = { category: 'old' };
  const old = c.executeDiscover(true);
  c.state.discover.selections.category = 'new';
  const current = c.executeDiscover(true);
  assert.equal(requests[0].args[1].category, 'old');
  requests[1].resolve({ items: [{ id: 'new-result' }], cursor: 'new-page', hasMore: true }); await current;
  requests[0].resolve({ items: [{ id: 'old-result' }], cursor: 'old-page', hasMore: true }); await old;
  assert.deepEqual(Array.from(c.state.discover.items, item => item.id), ['new-result']);
});

test('actor profile replies retain the current actor and its own cursor', async () => {
  const { c, requests, element } = harness('actor', ['loadActorDetail', 'loadMoreActorWorks']);
  const old = c.loadActorDetail('A');
  c.state.actor.userId = 'B'; c.state.actor.name = 'B';
  const current = c.loadActorDetail('B');
  requests[1].resolve({ name: 'B', works: [{ id: 'B-work' }], cursor: 'B-page', hasMore: true }); await current;
  requests[0].resolve({ name: 'A', works: [{ id: 'A-work' }], cursor: 'A-page', hasMore: true }); await old;
  assert.equal(element('actor-detail-name').textContent, 'B');
  const more = c.loadMoreActorWorks();
  assert.deepEqual(requests[2].args, ['B', 'B-page']);
  requests[2].resolve({ works: [{ id: 'B-next' }], cursor: '', hasMore: false }); await more;
  assert.deepEqual(element('actor-drama-grid').children.map(card => card.item.id), ['B-work', 'B-next']);
});

test('old actor pagination cannot append cards or clear the next actor loader', async () => {
  const { c, requests, element } = harness('actor', ['loadActorDetail', 'loadMoreActorWorks']);
  c.state.actor.cursor = 'A-page'; c.state.actor.hasMore = true;
  const more = c.loadMoreActorWorks();
  c.state.actor.userId = 'B'; c.state.actor.name = 'B';
  const profile = c.loadActorDetail('B');
  requests[0].resolve({ works: [{ id: 'A-next' }], cursor: 'A-next', hasMore: true }); await more;
  assert.equal(c.state.actor.loading, true);
  requests[1].resolve({ name: 'B', works: [{ id: 'B-work' }], cursor: 'B-page', hasMore: true }); await profile;
  assert.deepEqual(element('actor-drama-grid').children.map(card => card.item.id), ['B-work']);
  assert.equal(element('btn-actor-load-more').textContent, '加载更多作品');
  assert.equal(c.state.actor.loading, false);
});

test('opening an actor from another view fetches the profile once', async () => {
  const { c, requests } = harness('recommend', [...navigation, 'openActorDetail', 'loadActorDetail']);
  c.openActorDetail('B', 'B', '');
  assert.equal(requests.length, 1);
  requests[0].resolve({ name: 'B', works: [], cursor: '', hasMore: false });
  await new Promise(resolve => setImmediate(resolve));
});

test('a pending favorite action cannot overwrite another detail or run twice', async () => {
  const { c, requests, element } = harness('detail', ['initDetailEvents', 'applyFavoriteState']);
  c.initDetailEvents();
  const pending = element('btn-detail-favorite').listeners.click();
  await element('btn-detail-favorite').listeners.click();
  assert.equal(requests.length, 1);
  c.state.detail.id = 'B'; c.state.detail.data = { id: 'B', title: 'B', isFavorite: false };
  requests[0].resolve(true); await pending;
  assert.equal(c.state.detail.data.id, 'B');
  assert.equal(c.state.detail.data.isFavorite, false);
  assert.equal(element('btn-detail-favorite').disabled, false);
});

test('cached details revalidate and a read started before a favorite action cannot undo it', async () => {
  const { c, requests } = harness('detail', ['loadSeriesDetail', 'applyFavoriteState']);
  const pending = c.loadSeriesDetail('A');
  assert.equal(requests[0].method, 'GetSeriesPage');
  c.applyFavoriteState('A', true);
  requests[0].resolve({ id: 'A', title: 'Fresh metadata', isFavorite: false }); await pending;
  assert.equal(c.state.detail.data.title, 'Fresh metadata');
  assert.equal(c.state.detail.data.isFavorite, true);
});

test('like replies update only the episode that was actually mutated', () => {
  const { c, element } = harness('detail', ['applyLikeState']);
  c.state.feed = { seriesId: 'A', episodeId: 'new-episode', isLiked: false };
  c.state.player = { seriesId: 'A', episodeId: 'old-episode', isLiked: false };
  c.applyLikeState('A', 'old-episode', true);
  assert.equal(c.state.feed.isLiked, false);
  assert.equal(c.state.player.isLiked, true);
  assert.equal(element('player-like').classList.contains('active'), true);
});

test('collection failures preserve cached rows with independent error and retry states', async () => {
  const { c, requests, element } = harness('favorites', collections);
  const pending = c.loadFavoritesAndLikes();
  requests[0].reject(new Error('favorites unavailable'));
  requests[1].resolve([{ seriesId: 'new-like', episodeId: 'new-episode', title: 'New like' }]);
  await pending;
  assert.equal(c.state.favoritesView.favoritesList[0].seriesId, 'A');
  assert.equal(c.state.favoritesView.likesList[0].seriesId, 'new-like');
  assert.equal(element('favorites-grid').children[0].type, 'error');
  assert.equal(element('favorites-grid').children[1].item.id, 'A');
  c.switchFavoritesTab('likes');
  assert.equal(element('favorites-grid').children[0].item.id, 'new-like');
  assert.equal(c.state.favoritesView.errors.likes, null);
  c.switchFavoritesTab('favorites');
  const retry = element('favorites-grid').children[0].options.onRetry();
  requests[2].resolve([]); requests[3].resolve([]); await retry;
  assert.equal(c.state.favoritesView.favoritesList.length, 0);
  assert.equal(element('favorites-grid').children[0].options.title, '暂无收藏短剧');
});

test('collection failure without saved rows is an error rather than an empty collection', async () => {
  const { c, element } = harness('favorites', collections);
  c.state.favoritesView.favoritesList = []; c.state.favoritesView.likesList = [];
  c.apiCall = async () => { throw new Error('read failed'); };
  await c.loadFavoritesAndLikes();
  assert.equal(element('favorites-grid').children[0].options.title, '读取收藏失败');
  c.switchFavoritesTab('likes');
  assert.equal(element('favorites-grid').children[0].options.title, '读取点赞失败');
});

test('a collection read started before unfavorite cannot restore the removed card', async () => {
  const { c, requests } = harness('favorites', [...collections, 'applyFavoriteState']);
  const old = c.loadFavoritesAndLikes();
  c.applyFavoriteState('A', false);
  requests[0].resolve([{ seriesId: 'A', title: 'Old result' }]); requests[1].resolve([]); await old;
  assert.equal(c.state.favoritesView.favoritesList.length, 0);
  requests[2].resolve([]); requests[3].resolve([]);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(c.state.favoritesView.favoritesList.length, 0);
});

test('history filters and badges agree and require the last episode to be finished', () => {
  const { c, element } = harness('history', ['historyProgress', 'renderHistoryRows']);
  const cases = [
    ['first-episode', 1, 80, 100, 98, false],
    ['last-started', 80, 80, 100, 1, false],
    ['last-finished', 80, 80, 100, 95, true],
    ['unknown-count', 1, 0, 100, 100, false],
    ['unknown-duration', 80, 80, 0, 999, false],
    ['rounded-percent', 80, 80, 100, 94.6, false]
  ];
  c.state.historyView.list = cases.map(([title, episodeNum, totalEpisodes, durationSec, positionSec]) => ({
    seriesId: title, title, episodeNum, totalEpisodes, durationSec, positionSec, updatedAt: 100
  }));
  c.renderHistoryRows();
  element('history-grid').children.forEach((card, index) => {
    assert.equal(card.innerHTML.includes('全剧看完'), cases[index][5], cases[index][0]);
  });
  c.state.historyView.filter = 'completed'; c.renderHistoryRows();
  assert.equal(element('history-grid').children.length, 1);
  assert.ok(element('history-grid').children[0].innerHTML.includes('last-finished'));
  c.state.historyView.filter = 'unwatched'; c.renderHistoryRows();
  assert.equal(element('history-grid').children.length, 5);
  assert.equal(c.historyProgress({ durationSec: 0, positionSec: 999 }).percent, 0);
});

test('history view replaces cached rows with the refreshed cloud snapshot', async () => {
  const { c, requests } = harness('history', ['loadHistory', 'historyProgress', 'renderHistoryRows']);
  c.state.historyView.list = [{ seriesId: 'cached', title: 'Cached' }];
  const pending = c.loadHistory();
  assert.equal(requests[0].method, 'GetHistory');
  requests[0].resolve([{ seriesId: 'cached', title: 'Cached' }]);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(requests[1].method, 'RefreshHistory');
  requests[1].resolve([{ seriesId: 'fresh', title: 'Fresh' }]);
  await pending;
  assert.equal(c.state.historyView.list[0].seriesId, 'fresh');
  assert.equal(Number.isFinite(c.state.libraryRefreshAt.history), true);
});
