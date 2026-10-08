function requireValid(condition, message) {
  if (!condition) throw new Error(`Invalid shard coverage: ${message}`);
}

function object(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function collect(report) {
  requireValid(object(report) && Array.isArray(report.suites), 'report suites are required');
  const cases = [];
  function walk(suites, ancestors) {
    for (const suite of suites) {
      requireValid(object(suite) && typeof suite.title === 'string', 'invalid suite title');
      const titles = [...ancestors, suite.title];
      const specs = suite.specs ?? [];
      const children = suite.suites ?? [];
      requireValid(Array.isArray(specs) && Array.isArray(children), 'invalid suite contents');
      for (const spec of specs) {
        requireValid(object(spec) && typeof spec.title === 'string' &&
          typeof spec.file === 'string' && spec.file.length > 0 &&
          Array.isArray(spec.tests) && spec.tests.length > 0, 'invalid spec');
        for (const test of spec.tests) {
          requireValid(object(test) && typeof test.projectName === 'string', 'invalid project');
          cases.push({
            identity: JSON.stringify([test.projectName, spec.file, [...titles, spec.title]]),
            test,
          });
        }
      }
      walk(children, titles);
    }
  }
  walk(report.suites, []);
  return cases;
}

// The candidate --list report is the sole source of expected identities.
export function verifyShards({baseline, reports, allowedSkips}) {
  requireValid(object(baseline) && Array.isArray(baseline.errors) && baseline.errors.length === 0,
    'baseline discovery errors or missing error summary');
  const expected = new Set();
  for (const {identity} of collect(baseline)) {
    requireValid(!expected.has(identity), `duplicate baseline case ${identity}`);
    expected.add(identity);
  }
  requireValid(expected.size > 0, 'empty baseline');
  requireValid(Array.isArray(allowedSkips), 'skip allowance must be an array');
  const permitted = new Set();
  for (const identity of allowedSkips) {
    requireValid(typeof identity === 'string' && expected.has(identity),
      `skip allowance is outside baseline: ${identity}`);
    permitted.add(identity);
  }
  requireValid(Array.isArray(reports) && reports.length > 0, 'empty shard reports');
  const seen = new Set();
  let passed = 0;
  let skipped = 0;
  for (const report of reports) {
    requireValid(object(report) && Array.isArray(report.errors) && report.errors.length === 0,
      'worker errors or missing error summary');
    const stats = report.stats;
    requireValid(object(stats), 'missing stats');
    for (const key of ['expected', 'unexpected', 'flaky', 'skipped']) {
      requireValid(Number.isSafeInteger(stats[key]) && stats[key] >= 0, `invalid stats.${key}`);
    }
    requireValid(stats.unexpected === 0 && stats.flaky === 0, 'failed or flaky summary');
    let shardPassed = 0;
    let shardSkipped = 0;
    for (const {identity, test} of collect(report)) {
      requireValid(expected.has(identity), `extra case ${identity}`);
      requireValid(!seen.has(identity), `duplicate actual case ${identity}`);
      seen.add(identity);
      requireValid(Array.isArray(test.results) && test.results.length === 1,
        `case must execute exactly once: ${identity}`);
      const result = test.results[0];
      requireValid(object(result) && result.retry === 0, `retry or invalid result: ${identity}`);
      requireValid(result.error == null &&
        (result.errors === undefined || (Array.isArray(result.errors) && result.errors.length === 0)),
      `result errors: ${identity}`);
      if (result.status === 'passed') {
        requireValid(test.status === 'expected' && test.expectedStatus === 'passed',
          `contradictory passed outcome: ${identity}`);
        shardPassed++;
      } else {
        requireValid(result.status === 'skipped' && permitted.has(identity),
          `failed, unexecuted or unapproved skip: ${identity}`);
        requireValid(test.status === 'skipped' &&
          (test.expectedStatus === 'passed' || test.expectedStatus === 'skipped'),
        `contradictory skipped outcome: ${identity}`);
        shardSkipped++;
      }
    }
    requireValid(stats.expected === shardPassed && stats.skipped === shardSkipped,
      'stats disagree with actual results');
    passed += shardPassed;
    skipped += shardSkipped;
  }
  const missing = [...expected].filter(identity => !seen.has(identity));
  requireValid(missing.length === 0, `missing cases: ${missing.join(', ')}`);
  return {total: expected.size, passed, skipped, identities: [...expected].sort()};
}
