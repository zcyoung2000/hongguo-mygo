const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, '../frontend/app.js'), 'utf8');
const functions = [...source.matchAll(/^(?:async )?function (\w+)\(/gm)];
function harness() {
  let next = 0;
  const timers = new Map(), written = [];
  const c = vm.createContext({ console, Date, performance,
    window: { addEventListener() {} }, document: { addEventListener() {} },
    feedDiagnosticSession: 'test-session', feedMetricQueue: [], feedMetricFlushTimer: null,
    feedMetricFlushInFlight: false, feedSlots: [], feedSlotCurr: 0,
    state: { feed: { index: 0, items: [{}], loading: false } },
    feedVideoBufferAhead: () => 0,
    setTimeout: fn => { timers.set(++next, fn); return next; }, clearTimeout: id => timers.delete(id),
    apiCall: async (method, rows) => { assert.equal(method, 'RecordFeedMetrics'); written.push(...rows); }
  });
  for (const name of ['recordFeedDiagnostic', 'enqueueFeedMetric', 'scheduleFeedMetricFlush', 'flushFeedMetrics']) {
    const i = functions.findIndex(match => match[1] === name);
    vm.runInContext(source.slice(functions[i].index, functions[i + 1]?.index), c);
  }
  async function drain() {
    for (let count = 0; timers.size && count < 20; count++) {
      const [id, fn] = timers.entries().next().value;
      timers.delete(id);
      await fn();
    }
    assert.equal(c.feedMetricQueue.length, 0);
  }
  return { c, written, drain };
}
test('navigation and phase diagnostics survive batching beyond one IPC batch', async () => {
  const { c, written, drain } = harness();
  for (let n = 1; n <= 70; n++) c.recordFeedDiagnostic('navigation-input', { inputId: n });
  await drain();
  assert.deepEqual(written.map(row => row.inputId), Array.from({ length: 70 }, (_, i) => i + 1));
  assert.ok(written.every(row => row.sessionId === 'test-session'));
});
test('a transient write failure retains diagnostics and retries without duplicating inputs', async () => {
  const { c, written, drain } = harness();
  let fail = true;
  c.apiCall = async (method, rows) => {
    if (fail) { fail = false; throw new Error('temporary IPC failure'); }
    written.push(...rows);
  };
  c.recordFeedDiagnostic('navigation-input', { inputId: 1 });
  await drain();
  assert.deepEqual(written.map(row => row.inputId), [1]);
});
test('diagnostics cannot break navigation when a media getter throws during teardown', () => {
  const { c } = harness();
  c.feedVideoBufferAhead = () => { throw new Error('detached media'); };
  assert.doesNotThrow(() => c.recordFeedDiagnostic('navigation-input', { inputId: 1 }));
});
