const fs = require('fs');
const vm = require('vm');
const assert = require('assert');

const read = path => fs.readFileSync(path, 'utf8');

class ClassList {
  constructor(values = []) { this.values = new Set(values); }
  add(value) { this.values.add(value); }
  remove(value) { this.values.delete(value); }
  toggle(value, force) { if (force) this.values.add(value); else this.values.delete(value); }
  has(value) { return this.values.has(value); }
}

class Element {
  constructor(id) {
    this.id = id;
    this.classList = new ClassList();
    this.dataset = {};
    this.style = {};
    this.attributes = {};
    this.children = [];
    this.handlers = {};
    this.parent = null;
    this.offsetTop = 0;
    this.offsetHeight = 28;
    this.clientHeight = 96;
    this.scrollHeight = 400;
  }
  appendChild(child) {
    child.parent = this;
    this.children.push(child);
  }
  querySelector(selector) {
    if (selector === '[data-collapse-text]') return this.children.find(child => child.dataset.collapseText);
    if (selector === 'svg') return this.children.find(child => child.id === 'collapse-icon');
    return undefined;
  }
  setAttribute(name, value) { this.attributes[name] = value; }
  addEventListener(name, handler) { this.handlers[name] = handler; }
}

function createFixture() {
  const grid = new Element('player-drawer-grid');
  grid.querySelector = selector => {
    if (selector === '.ep-btn.active') {
      const active = new Element('active');
      active.classList.add('active');
      active.offsetTop = 102;
      grid.scrollTop = active.offsetTop - 45;
      return active;
    }
    return undefined;
  };
  const toggleWrap = new Element('player-drawer-collapse-wrap');
  const label = new Element('collapse-label');
  label.dataset.collapseText = true;
  const icon = new Element('collapse-icon');
  const toggle = new Element('player-drawer-collapse');
  toggle.children.push(label, icon);
  return { grid, toggleWrap, toggle, label, icon };
}

function loadController(fixture) {
  const source = read('frontend/app.js');
  const match = source.match(/function createEpisodeCollapseController\(([\s\S]*?)\n\}/);
  assert.ok(match, 'episode collapse controller function not found');
  const context = { ...fixture, createEpisodeCollapseController: null };
  context.window = context;
  vm.createContext(context);
  vm.runInContext(`${match[0]}\nresult = createEpisodeCollapseController({ grid, toggleWrap, toggle });`, context);
  return { controller: context.result, fixture };
}

{
  const { controller, fixture } = loadController(createFixture());
  const { grid, toggleWrap, toggle, label, icon } = fixture;
  assert.equal(controller.overflow, true, '3-row overflow should expose the toggle');
  assert.ok(grid.classList.has('is-collapsed'), 'episode list must default to collapsed');
  assert.equal(toggleWrap.style.display, 'flex');
  assert.equal(grid.scrollHeight, 400, 'three-row collapsed height should be enforced by CSS');

  toggle.handlers.click();
  assert.equal(controller.expanded, true);
  assert.ok(!grid.classList.has('is-collapsed'), 'toggle must expand the complete list');
  assert.equal(label.textContent, '收起选集');
  assert.equal(toggle.attributes['aria-expanded'], 'true');
  assert.equal(icon.style.transform, 'rotate(180deg)');
  assert.equal(grid.scrollTop, 68, 'expanded active row must be centered in view');

  toggle.handlers.click();
  assert.equal(controller.expanded, false);
  assert.ok(grid.classList.has('is-collapsed'), 'toggle must collapse again');
  assert.equal(label.textContent, '展开全部');
  assert.equal(toggle.attributes['aria-expanded'], 'false');
  assert.equal(grid.scrollTop, 0);
}

{
  const fixture = createFixture();
  fixture.grid.scrollHeight = 96;
  const { controller } = loadController(fixture);
  assert.equal(controller.overflow, false, 'three rows must not expose a toggle');
  assert.equal(fixture.toggleWrap.style.display, 'none');
}

const css = read('frontend/styles.css');
assert.match(css, /--player-drawer-width:\s*248px/);
assert.match(css, /--comment-sheet-width:\s*420px/);
assert.doesNotMatch(css, /\.feed-playlist-drawer\s*\{[^}]*width:\s*320px/);
assert.match(css, /\.feed-playlist-drawer\s+\.episodes-grid\s*\{[^}]*gap:\s*6px/);
assert.match(css, /\.feed-playlist-drawer\s+\.episodes-grid\.is-collapsed\s*\{[^}]*height:\s*calc\(3 \* 28px \+ 2 \* 6px\)/);

{
  const cssWithoutComments = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const selectorNeedsLayout = [
    'feed-controls', 'progress-bar-wrap', 'progress-bar-bg', 'progress-bar-buffer',
    'progress-bar-current', 'controls-row', 'controls-left', 'controls-right', 'ctrl-btn',
    'time-display', 'quality-selector', 'speed-selector', 'ctrl-text-btn', 'quality-menu',
    'speed-menu', 'speed-opt'
  ];
  for (const className of selectorNeedsLayout) {
    assert.match(
      cssWithoutComments,
      new RegExp(`(?:^|[\\s,}])\\.${className}\\s*[,{]|(?:^|[\\s,}])\\.${className}\\s*:`, 'm'),
      `player chrome class must retain a layout rule: .${className}`
    );
  }
  assert.match(cssWithoutComments, /\.feed-container\.feed-fullscreen\s*\{[^}]*position:\s*fixed/);
  assert.match(cssWithoutComments, /\.feed-controls\s*\{[^}]*left:\s*var\(--stage-chrome-inset/);
  assert.match(cssWithoutComments, /\.feed-controls\s*\{[^}]*bottom:\s*var\(--stage-chrome-inset/);
  assert.match(cssWithoutComments, /body\.comments-open\s+\.feed-stage\s*\{[^}]*inset:\s*0\s+var\(--comment-sheet-width\)/);
  assert.match(
    cssWithoutComments,
    /\.feed-container\.episode-drawer-open\s+\.feed-stage\s*\{[^}]*inset:\s*0\s+var\(--player-drawer-width\)/
  );
  assert.match(
    cssWithoutComments,
    /\.feed-container\.episode-drawer-open\s+\.feed-rail\s*\{[^}]*right:\s*calc\(var\(--player-drawer-width\)\s*\+/,
    'open episode drawer must reserve space for the vertical action rail'
  );
  assert.match(
    cssWithoutComments,
    /\.feed-container\.episode-drawer-open\s+\.feed-controls\s*\{[^}]*right:\s*calc\(var\(--player-drawer-width\)\s*\+/,
    'open episode drawer must reserve space for the bottom control bar'
  );
  assert.match(cssWithoutComments, /\.comment-sheet-overlay\s*\{[^}]*position:\s*fixed/);
  assert.match(cssWithoutComments, /\.comment-sheet\s*\{[^}]*display:\s*flex/);
  assert.match(cssWithoutComments, /\.comment-list\s*\{[^}]*overflow-y:\s*auto/);
}

{
  const cssWithoutComments = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const appSource = read('frontend/app.js');
  const renderStart = appSource.indexOf('function renderCommentSheet()');
  const renderEnd = appSource.indexOf('function closeCommentSheet()', renderStart);
  assert.ok(renderStart >= 0 && renderEnd > renderStart, 'comment renderer must exist');
  const renderSource = appSource.slice(renderStart, renderEnd);
  const replyStart = appSource.indexOf('function appendReplyElements()');
  const replySource = appSource.slice(replyStart, renderEnd);
  const dynamicCommentClasses = [...renderSource.matchAll(/className\s*=\s*['"`]([^'"`]+)['"`]/g)]
    .flatMap(match => match[1].trim().split(/\s+/))
    .filter(Boolean);
  dynamicCommentClasses.push(...[...replySource.matchAll(/className\s*=\s*['"`]([^'"`]+)['"`]/g)]
    .flatMap(match => match[1].trim().split(/\s+/))
    .filter(Boolean));
  for (const className of new Set(dynamicCommentClasses)) {
    assert.match(
      cssWithoutComments,
      new RegExp(`(?:^|[\\s,}])\\.${className}\\s*[,{]|(?:^|[\\s,}])\\.${className}\\s*:`, 'm'),
      `comment DOM class must retain a layout rule: .${className}`
    );
  }
}

const app = read('frontend/app.js');


[...read('frontend/player-chrome.js').matchAll(/<svg[^>]*>([\s\S]*?)<\/svg>/g)].forEach(([, svgBody]) => {
  assert.match(svgBody, /fill="currentColor"/, 'player chrome SVG paths must use currentColor');
});

console.log('player drawer, collapse, and navigation UI regression: ok');

{
  const app = read('frontend/app.js');
  assert.match(app, /for \(let episodeNum = 1; episodeNum <= total; episodeNum\+\+\)/);
  assert.doesNotMatch(app, /if \(chrome\.Episode > 12 && index == 11\)/);
  assert.doesNotMatch(app, /number = chrome\.Episode/);

}
