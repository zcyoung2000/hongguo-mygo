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
assert.doesNotMatch(css, /\.feed-playlist-drawer\s*\{[^}]*width:\s*320px/);
assert.match(css, /\.feed-playlist-drawer\s+\.episodes-grid\s*\{[^}]*gap:\s*6px/);
assert.match(css, /\.feed-playlist-drawer\s+\.episodes-grid\.is-collapsed\s*\{[^}]*height:\s*calc\(3 \* 28px \+ 2 \* 6px\)/);
assert.match(css, /\.detail-episode-grid\.is-collapsed\s*\{[^}]*max-height:\s*calc\(3 \* 48px \+ 2 \* 10px\)/);
assert.match(css, /\.player-topbar\s*\{[^}]*padding:\s*max\(0px,\s*env\(safe-area-inset-top\)\)\s*24px\s*0\s*16px/);
assert.match(css, /\.player-back-btn\s*\{[^}]*min-height:\s*32px/s);

const app = read('frontend/app.js');
assert.match(app, /function createPlayerDrawerController\(/);
assert.match(app, /function createEpisodeCollapseController\(/);
assert.match(app, /if \(state\.player\.episodesExpanded\) \{\s*scrollActiveDrawerEpisode\('player-drawer-grid', num\);/s);
assert.match(app, /collapsePlayerDrawerEpisodes\(\);/);

const playerChrome = read('internal/uiapp/player_chrome.go');
assert.match(playerChrome, /playerDrawerMaxWidth = 248/);

[...read('frontend/player-chrome.js').matchAll(/<svg[^>]*>([\s\S]*?)<\/svg>/g)].forEach(([, svgBody]) => {
  assert.match(svgBody, /fill="currentColor"/, 'player chrome SVG paths must use currentColor');
});

console.log('player drawer, collapse, and navigation UI regression: ok');

{
  const app = read('frontend/app.js');
  assert.match(app, /for \(let episodeNum = 1; episodeNum <= total; episodeNum\+\+\)/);
  assert.doesNotMatch(app, /if \(chrome\.Episode > 12 && index == 11\)/);
  assert.doesNotMatch(app, /number = chrome\.Episode/);

  const chrome = read('internal/uiapp/player_chrome.go');
  assert.match(chrome, /ui\.GridView\(c, &a\.playerDrawerGrid, chrome\.TotalEpisodes/);
  assert.doesNotMatch(chrome, /min\(chrome\.TotalEpisodes,\s*12\)/);
  assert.doesNotMatch(chrome, /number = chrome\.Episode/);
}
