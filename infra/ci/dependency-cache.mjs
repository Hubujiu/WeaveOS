import {isAbsolute, join} from 'node:path';

// Only reproducible public dependencies cross job boundaries. No private
// environment, database, browser state, settings or test-result mount exists.
export function dependencyMounts(root, kind) {
  if (typeof root !== 'string' || !isAbsolute(root) || /[,\r\n]/.test(root)) {
    throw new Error('An unambiguous absolute dependency-cache path is required');
  }
  const layouts = {
    go: [['go-mod', '/go/pkg/mod'], ['go-build', '/root/.cache/go-build']],
    node: [['pnpm-store', '/repo/.work/pnpm-store']],
  };
  if (!Object.hasOwn(layouts, kind)) throw new Error('Unknown public dependency cache');
  return layouts[kind].flatMap(([name, destination]) =>
    ['--mount', `type=bind,src=${join(root, name)},dst=${destination}`]);
}
