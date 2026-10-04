# Root-authored visual oracle migration

2026-10-05 Beijing. User approved the new monochrome full-bleed design and frontend implementation. This supersedes old Q31/Q33 176px sidebar,56px top,L-material opacity and x16/y116 fixed menu assumptions.

Root personally updated personnel.component.spec.ts; implementation worker may not edit assertions. Source before migration is exact commit f1b0603f6aa77f84de6ea767068236d44bc87c37. The new contract predates implementation in V030-021 PRD/ADR and Root commit819aab8, not inferred from test failures.

Replacements assert the positive new white surface, pale background, x78/y0/right/bottom viewport geometry at desktop,190px sidebar/64px top and reachable compact navigation. Account moves into the far-left rail; its accessibility and menu visibility remain tested. Active tab indicator uses neutral selected gray instead of old black. Scrolling assertions follow visible controls and the actual table viewport rather than requiring a removed owner element to scroll. Dirty values,explicit discard,keyboard tabs,authorization,network DTOs,write counts,server outcomes,Arca table behaviors and immutable check SVG assertions remain.

No skips added. No runtime fixtures widened. Tests still require real rendered controls with mocked HTTP and do not prove real HTTPS/BFF integration. Original Root8 tests remain unchanged. New visual checks must be run, not assumed green. Expected frontend brand is kept WaveOS for P1; the design reference uses WeaveOS and final copy alignment is tracked separately.
