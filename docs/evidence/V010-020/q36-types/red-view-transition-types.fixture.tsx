// Compile-only approved stable API check. Not mounted in the application and
// not evidence of visual/animation acceptance. No canary imports or local shim.
import { ViewTransition } from 'react';
import type { ViewTransitionProps } from 'react';

const props: ViewTransitionProps = {
  name: 'q36-filter-panel',
  enter: 'q36-filter-enter',
  exit: 'q36-filter-exit',
  onEnter: (_instance, types) => { types.includes('filter-open'); },
};

export const stableViewTransitionTypeFixture = <ViewTransition {...props}><div /></ViewTransition>;
