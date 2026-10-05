// Approved review binds this advisory to verified release bytes and a fresh regression.
const reviewedRelease = Object.freeze({
  modulePath: 'google.golang.org/grpc',
  version: 'v1.84.0',
  sum: 'h1:soMyaPJ8pAak5PIQ0DGBUir0XRo2fRoMqhNWMLlLxO0=',
  goModSum: 'h1:ljCht0DrxQrXBDRTZp52Qxh3Ffk8CdYm2sj4O2QN2C0=',
  replacement: false,
  transportSHA256: '6ff2da17e276ba22a782dffe467ec32b664895d12faccfbb5fc1ec40d59dcf6c',
});
const requiredRegression = Object.freeze({exitCode: 0, passed: 1, failed: 0, skipped: 0});

function matchesOwnFields(value, expected) {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    && Object.entries(expected).every(([key, required]) =>
      Object.hasOwn(value, key) && value[key] === required);
}

export function unresolvedAdvisories(ids, evidence) {
  const reviewed = matchesOwnFields(evidence, reviewedRelease)
    && Object.hasOwn(evidence, 'regression')
    && matchesOwnFields(evidence.regression, requiredRegression);
  return [...ids].filter(id => id !== 'GO-2026-6443' || !reviewed);
}
