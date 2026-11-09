const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const appSource = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const CONTROL_INSET = 24;
const CONTROL_BUTTON_GAP = 14;
const RIGHT_CONTROL_WIDTHS = new Map([
  ['quality', 51],
  ['speed', 40],
  ['danmaku', 32],
  ['danmaku-settings', 32],
  ['drawer-toggle', 64],
  ['pip', 32],
  ['fullscreen', 32]
]);

function loadFunction(context, name) {
  const start = appSource.indexOf(`function ${name}(`);
  assert.ok(start >= 0, `missing function ${name}`);
  const end = appSource.indexOf('\n}', start) + 2;
  assert.ok(end > start, `unterminated function ${name}`);
  vm.runInContext(appSource.slice(start, end), context);
}

function element(id, parent = null) {
  return {
    id,
    parent,
    classList: {
      values: new Set(),
      add(...names) { names.forEach(name => this.values.add(name)); },
      remove(...names) { names.forEach(name => this.values.delete(name)); },
      toggle(name, force) { force ? this.values.add(name) : this.values.delete(name); },
      contains(name) { return this.values.has(name); }
    },
    attributes: {},
    style: {},
    children: [],
    closest(selector) { return selector === '.feed-container' ? parent : null; },
    setAttribute(name, value) { this.attributes[name] = String(value); },
    appendChild(child) { this.children.push(child); },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    addEventListener() {}
  };
}

function layoutFixture() {
  const container = element('feed-container');
  const stage = element('feed-stage', container);
  const rail = element('feed-rail', container);
  const controls = element('feed-controls', container);
  const drawer = element('feed-playlist-drawer', container);
  const stageBox = {left: 0, top: 0, width: 752, height: 600};
  const rectangles = new Map([
    [stage, stageBox],
    [rail, {left: 692, top: 400, width: 48, height: 160}],
    [controls, {left: 24, top: 524, width: 704, height: 52}],
    [drawer, {left: 752, top: 0, width: 248, height: 600}]
  ]);
  const context = {
    state: {
      currentView: 'recommend',
      feed: {seriesId: 'series-a', episodeNum: 1, totalEpisodes: 1, episodes: [{number: 1}]}
    },
    syncFeedScrollCapture() {},
    schedulePlaybackVideoFit() { layoutRequests++; },
    scrollActiveDrawerEpisode() {},
    renderEpisodeGrid({grid, count, total, current, onSelect}) {
      assert.equal(total, 1);
      assert.equal(current, 1);
      assert.equal(typeof onSelect, 'function');
      grid.textContent = count.textContent = 'rendered';
    }
  };
  let layoutRequests = 0;
  vm.createContext(context);
  loadFunction(context, 'toggleFeedPlaylistDrawer');
  loadFunction(context, 'closeFeedPlaylistDrawer');
  return {
    context,
    container,
    stage,
    rail,
    controls,
    drawer,
    rectangles,
    drawerBox() { return rectangles.get(drawer); },
    layoutRequests: () => layoutRequests,
    controlsRightEdge() {
      const controlsRect = rectangles.get(controls);
      return controlsRect.left + controlsRect.width;
    }
  };
}

function assertNoOverlap(left, right) {
  const normalizedLeft = {...left, right: left.left + left.width, bottom: left.top + left.height};
  const normalizedRight = {...right, right: right.left + right.width, bottom: right.top + right.height};
  assert.ok(
    normalizedLeft.right <= normalizedRight.left ||
      normalizedRight.right <= normalizedLeft.left ||
      normalizedLeft.bottom <= normalizedRight.top ||
      normalizedRight.bottom <= normalizedLeft.top,
    `rectangles overlap: ${JSON.stringify(normalizedLeft)} and ${JSON.stringify(normalizedRight)}`
  );
}

function controlButtonBoxes(panelWidth = 0, viewportWidth = 1000) {
  const controls = {
    left: CONTROL_INSET,
    top: 524,
    width: viewportWidth - CONTROL_INSET * 2 - panelWidth,
    height: 52
  };
  const buttons = [];
  let right = controls.left + controls.width - 24;
  for (const [name, width] of RIGHT_CONTROL_WIDTHS) {
    const left = right - width;
    buttons.push({name, left, top: 540, width, height: 32});
    right = left - CONTROL_BUTTON_GAP;
  }
  return {controls, buttons};
}

function assertControlsFit(buttonOwner, panel) {
  for (const button of buttonOwner.buttons) {
    assert.ok(button.left >= 0, `${button.name} is visible within the viewport`);
    assert.ok(
      button.left + button.width <= panel.left,
      `${button.name} must end before the panel: ${button.left + button.width} > ${panel.left}`
    );
    assertNoOverlap(button, panel);
  }
}

test('opening the episode drawer reserves a non-overlapping player column', () => {
  const layout = layoutFixture();
  const drawerBox = layout.drawerBox();
  const rail = layout.rectangles.get(layout.rail);
  const controls = layout.rectangles.get(layout.controls);
  const drawerControls = controlButtonBoxes(248);

  layout.context.toggleFeedPlaylistDrawer({
    drawer: layout.drawer,
    drawerToggle: element('feed-drawer-toggle', layout.container),
    drawerGrid: element('feed-drawer-grid', layout.drawer),
    drawerCount: element('feed-drawer-count', layout.drawer)
  });

  assert.equal(layout.drawer.classList.contains('open'), true);
  assert.equal(layout.container.classList.contains('episode-drawer-open'), true);
  assert.ok(rail.left + rail.width <= drawerBox.left, 'vertical action rail must end before the drawer');
  assert.ok(layout.controlsRightEdge() <= drawerBox.left, 'bottom controls must end before the drawer');
  assertNoOverlap(rail, drawerBox);
  assertNoOverlap(drawerControls.controls, drawerBox);
  assertControlsFit(drawerControls, drawerBox);
  assert.equal(layout.controlsRightEdge(), 728);
  assert.equal(layout.layoutRequests(), 1);

  const css = fs.readFileSync(path.join(__dirname, '../frontend/styles.css'), 'utf8')
    .replace(/\/\*[\s\S]*?\*\//g, '');
  assert.match(
    css,
    /\.feed-container\.episode-drawer-open\s+\.feed-controls\s*\{[^}]*right:\s*calc\(var\(--player-drawer-width\)\s*\+/,
    'open episode drawer must reserve space for the bottom control bar'
  );

  layout.context.closeFeedPlaylistDrawer({
    drawer: layout.drawer,
    drawerToggle: element('feed-drawer-toggle', layout.container)
  });

  assert.equal(layout.drawer.classList.contains('open'), false);
  assert.equal(layout.container.classList.contains('episode-drawer-open'), false);
  assert.equal(layout.layoutRequests(), 2);
});

test('opening comments keeps every right-side control visible and clickable', () => {
  const body = element('body');
  const overlay = element('comment-sheet-overlay');
  const commentSheet = element('comment-sheet');
  const nodes = new Map([
    ['comment-sheet-overlay', overlay],
    ['comment-sheet', commentSheet]
  ]);
  let fitRequests = 0;
  const context = {
    commentState: {open: false, requestToken: 0, source: 'feed'},
    document: {
      getElementById: id => nodes.get(id) || null,
      body
    },
    window: {matchMedia: () => ({matches: true})},
    showToast() {},
    syncFeedScrollCapture() {},
    schedulePlaybackVideoFit() { fitRequests++; },
    mountCommentOverlay() {},
    restoreCommentOverlay() {},
    renderCommentSheet() {},
    animateCommentVideo() {},
    getCommentVideo() { return null; },
    apiCall: async () => ({})
  };
  vm.createContext(context);
  loadFunction(context, 'openCommentSheet');
  loadFunction(context, 'closeCommentSheet');

  context.openCommentSheet({
    seriesId: 'series-a',
    episodeId: 'episode-a',
    episodeNum: 1,
    title: '短剧',
    count: 2
  });

  assert.equal(context.commentState.open, true);
  assert.equal(body.classList.contains('comments-open'), true);
  assert.equal(overlay.style.display, 'flex');
  const panel = {left: 580, top: 0, width: 420, height: 600};
  const commentsControls = controlButtonBoxes(panel.width);
  assertNoOverlap(commentsControls.controls, panel);
  assertControlsFit(commentsControls, panel);
  assert.equal(fitRequests, 1);

  context.closeCommentSheet();
  assert.equal(context.commentState.open, false);
  assert.equal(body.classList.contains('comments-open'), false);
  assert.equal(overlay.style.display, 'none');
  const restoredControls = controlButtonBoxes(0);
  assert.equal(restoredControls.controls.width, 952);
  assert.ok(restoredControls.buttons.at(-1).left + 32 > panel.left);
  assert.equal(fitRequests, 2);
});
