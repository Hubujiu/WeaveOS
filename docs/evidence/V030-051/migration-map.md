# V030-051 coverage migration map

Baseline: develop `3a33e710461388b27221a030d1e3ea46dfe038eb`.
Status: all 48 replacement engine instances completed (43 PASS / 5 documented native capability SKIP); representative observable-contract mutations failed as expected and the restored baseline passed. Retirement proceeds with this mapping; complete-entrypoint regression and final CI remain pending.

The retired fixture renders QueryFilterPanel. Production PersonnelAdmin renders TablePresetManager. Their motion state machines are independent, so passing the old tests is not evidence for the production controller. The accepted ADR-008 and frozen filter-manager contract define a finite OR-of-AND editor, not the former arbitrary recursive editor. Backend tree validation remains separately covered.

## q36-front-filter.component.spec.ts (6 identities)

1. Opens, initial keyboard focus, Escape return, empty root: new `keyboard entry gives the first control focus without manual refocusing` verifies production keyboard entry and automatic return. Existing `visibility-only save has independent searches and keeps at least one business field`, `visibility-only applies without a query and deletion of nonactive does not change the snapshot`, and `frozen text empty string stays a typed value and is distinct from zero conditions` retain current zero-condition semantics. The current manager is centered by contract; the obsolete beside-search layout is not reinstated.
2. Nested AND/OR, exact text and relation neq DTO: new `preserves exact text and relation inequality in the approved DTO` checks the complete production write body. Existing `save finite OR of AND blocks and typed comparison without applying` checks the current approved grouping. Arbitrary recursive UI shape is obsolete; neither test claims database NOT EXISTS execution.
3. Invalid empty nested group: the former fixture-only recursive group's remove-to-root interaction is obsolete. Existing `invalid empty intermediate row stays editable and cannot be saved` and `unsupported runtime tree cannot be silently flattened by edit or applied as a full query` protect the current editable intermediate state and lossless rejection. Shared Node tree validation remains intact.
4. Twenty leaves and recovered capacity: new `deleting a condition restores the shared AND OR twenty-leaf capacity` checks both add controls, removal without losing the surviving value, and re-exhaustion of shared capacity.
5. Three-level recursive UI and time/operator types: recursive add-group depth controls are obsolete under the finite editor contract. New `keeps typed operator menus and exact zoned microsecond input` checks exact text/time operator choices and the full zoned six-digit precision DTO. Backend depth validation remains in the unchanged Node/API tests.
6. Reduced motion rapid reversal and outside close: new `reduced-quick restores trigger pixels after Escape and outside close` verifies last close intent, automatic Escape focus, reopening, outside close, visible content, class cleanup and both retained late-frame observation windows. Existing real-manager operability test also remains.

## q36-front-motion.component.spec.ts (6 identities)

7. Native API open and close: new `uses native shared geometry at 300ms open and 220ms close` verifies actual production-native calls and real shared pseudo-element geometry durations.
8. Unsupported native fallback: new fallback pixel test verifies animations scoped to the production popup, close visibility and automatic trigger focus. The generic safe-edit test checks fallback editing too.
9. Quick native reversal: new quick pixel test verifies last close intent, automatic focus and transition class cleanup, as well as visual restoration and reopening.
10. Delayed native opening before fallback: new `delayed opening 0ms honors trusted Escape` gates the actual production opening update, verifies trusted input and awaits the released commit before asserting the last intent.
11. Delayed native opening after fallback: new `delayed opening 650ms honors trusted Escape` crosses the unchanged 600ms fallback and checks the same final intent after commit.
12. WebKit fallback safe editing: new `fallback safely edits finite AND OR groups including WebKit` runs every engine. WebKit uses its actual Apple fallback; others have native support explicitly disabled. Animation instrumentation is scoped to the production popup, native manager calls must remain zero, and values survive adding a second group. The old WebKit-only skip on non-WebKit is removed only when the old identity is retired.

## q36-front-trigger-visual.component.spec.ts (7 identities)

13–18. Native, fallback, reduced, quick, fallback-quick and reduced-quick: six matching production-manager tests retain full trigger PNG equality, visible icon/text, transition cleanup, Escape and outside closure, reopening, and both 1000ms late-frame windows. Native-only capability skips remain explicit on WebKit; fallback/reduced coverage runs there. Pixel preparation must compare the same input-modality state, without hiding product styles or relaxing equality.
19. 300ms/220ms shared geometry: consolidated into identity 7's production geometry test; this is a genuine overlapping contract, not a claim that the two old controllers are identical.

## Additional regression coverage and unchanged safeguards

Two controlled delayed-list tests cover the newly demonstrated disabled/inert initial-focus bug: focus reaches the first enabled control, and a newer user focus choice wins. Together with fast keyboard entry, all three are exercised on three engines.

The shared Table's four tests, QueryFilterState/validator Node tests, q36-b2 closing-generation and newer-interaction focus regressions, existing production manager tests, and real full-stack browser/video checks remain. The q36 fixture and old component implementation are not removed in this task because the Table tests still use the fixture.

Counts must be obtained from all three official collection entrypoints after final edits. Case counts alone do not establish coverage or runtime gains. No measured speedup is claimed before comparable measurements.
