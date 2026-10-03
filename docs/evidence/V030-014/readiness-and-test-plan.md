# V030-014 source read and test plan — preparation only

Status: **NOT RUN / implementation gated**. This file records sources and an
independent test oracle before any V030-014 behavior is written. It is not a RED
result, implementation claim, or product acceptance.

## Fixed starting point and ownership

- Independent branch/worktree: `task/V030-014-form-designer` at
  `9b89e8e928aedf30df235492fe89bc52021f6fe9` (actual PR26 head checked
  against GitHub and local Git on 2026-10-03). PR26, PR21 and main are untouched.
- V030-014 owns `apps/web/src/applications/forms/`, its reusable field renderer,
  own tests/task/evidence. Shell owner owns `Workspace.tsx`, `main.tsx`, and
  navigation; PR26 CI owner owns `TablePresetManager.tsx`. Expose a typed form
  module component and navigation/dirty interface for Shell to mount.
- No GSAP dependency exists in PR26 package.json. Official pinned GSAP skills
  `gsap-core`, `gsap-react`, `gsap-timeline`, and `gsap-performance` at
  `aed9cfd3277740755f6bfc1155c7aa645403b760` were read; dependency/version
  installation requires coordination with the lead.

Proposed Shell mount contract, pending V030-014 PLAN readback and not yet an
exported implementation:

```ts
type ApplicationStructureProps = {
  appId: string;
  onOpenForm: (viewId: string) => void;
  onDirtyChange: (dirty: boolean) => void;
};
type FormDesignerProps = {
  appId: string;
  viewId: string;
  onDirtyChange: (dirty: boolean) => void;
  onBack: () => void;
};
```

The Shell chooses route paths, tab state and navigation blocker. The module
loads the actual structure/definition, owns field/layout drafts, and reports
dirty state. Shell calls `onOpenForm` only for a persisted server view ID. No
fake form object or cross-module mutable store is required.

## Source ledger

| Source | Observed fact relevant here |
| --- | --- |
| [PRD v0.3.0](https://app.notion.com/p/3ed2f5a9e648811abf4ee555fb9695aa) | Read 2026-10-03 09:03:54 UTC. Iteration remains 待评审/未冻结. AP-FR-01–03, V030-013.1–7 and the corrected exact product contract were read. V030-014 PLAN had not yet appeared. |
| [ADR-009](https://app.notion.com/p/3ed2f5a9e64881258f3afbd0f83bca3f) | Read 2026-10-03 08:40:07 UTC. Still 拟议中; D02/D08 and explicit original Figma choice apply. |
| [Figma designer 327:3107](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=327-3107) | High-fidelity design context and screenshot obtained. Three columns: palette, form canvas, property panel. Original Home 358:18205 and Personnel 108:151 contexts/screenshots also obtained. |
| [Figma states 327:2104](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=327-2104) | Full page metadata inspected. High-fidelity context/screenshots obtained for preview 332:5028, preflight 332:5461, confirm 337:10258, settings 332:9733, unsaved 340:18187. Prototype wording that says 演示 does not establish API success. |
| [V030-013 proposal](https://github.com/Hubujiu/WeaveOS/blob/2980b5e89dd4e86af5b2a720346d8871384a4911/contracts/application-structure.proposal.md) | Original proposal read. PRD V030-013.6–7 supersedes its POST forms and layout node shapes and freezes the resulting product contract. Frontend consumption still waits for lead's V030-014 readback/release and backend revision. |
| [ADR-002](https://app.notion.com/p/3e52f5a9e648813da842e2ef15a09b3a) | Accepted; ordinary JSON envelope has `code/message/data/meta`, Session cookie/CSRF stays at BFF. |
| [React component library](https://github.com/Hubujiu/React-) | AGENTS, consumer/contract rules and 42-component approved manifest read. Product-specific designer composition may use project-native inputs; no component-source edits or fabricated general replacement. |

## RED-first acceptance matrix

Each case will be written as an executable assertion, run to a behavior RED,
then implemented to GREEN. A missing API or test server is a blocker, not RED.
Expected values below come from the PRD, corrected contract, and independent
decimal examples, never from observed implementation output.

| ID | Level | Given / action | Expected observation |
| --- | --- | --- | --- |
| DIR-1 | component + real API | Owner creates a root and child directory, renames and moves within one app | One `GET /structure` reread shows stable UUIDs, parent, position and incremented structure version; tree matches. |
| DIR-2 | component + real API | Move parent under its descendant, cross-app target, or stale structure version | Server rejects; tree keeps last confirmed state and unsaved input is retained. |
| DIR-3 | browser | Keyboard user focuses a tree node and invokes add, rename, and move | Complete operation without drag; focus returns to meaningful node or originating trigger; screen reader name/state remain available. |
| FORM-1 | real API | New form with `source:{kind:"new_table"}` | One POST atomically returns table+form; no orphan on failure; initial schema/view versions 0; first Save performs DDL. |
| FORM-2 | real API | Add second form with `source:{kind:"existing_table",tableId}` | Both form views reference the same stable table ID; no duplicate physical table. |
| FIELD-1 | component | Add each of text, multiline, number, money, date, datetime, single/multi select, boolean, member, department | Draft stores stable field UUID, field-specific configuration, label/required/default/presentation; no persisted write before Save. |
| FIELD-2 | component | Add id, createdBy, createdAt, updatedAt, recordVersion and group/divider/description | System fields stay read-only outside mutable fields; decoration creates layout nodes but no business field. |
| FIELD-3 | component | Drag or use keyboard to reorder, place in a group, change widths | Array order and `span` 1–12 produce a 12-column grid; omitted span normalizes to 12; divider/description span full row. Keyboard and pointer yield the same draft. |
| DEC-1 | component + API | Configure number/money and input `-1.25` with HALF_UP/HALF_EVEN at one decimal; `-149` FLOOR at hundreds | Wire and local draft use decimal strings, no float conversion; authoritative normalized results are `-1.3`, `-1.2`, `-200` respectively. |
| DEC-2 | component + API | Configure -18..18 places, scale 0..18 and precision 1..38; enter exponent, NaN, float, overflow after rounding | Invalid values show error and cannot be presented as saved; no silent truncation or precision loss. |
| OPT-1 | component + API | Rename stable option, select duplicate multi option, remove used option, multi→single with old multi value | IDs survive rename; duplicates normalize; used option requires explicit mapping and impact; multi→single blocks rather than choosing one. |
| DATE-1 | component + API | Date/datetime defaults and configurable minute/second/millisecond precision | Date uses exact YYYY-MM-DD; datetime wire includes explicit offset, server stores UTC; presentation timezone does not rewrite instant. |
| PREV-1 | component + real API | Edit draft then preview and exit | Preview shows draft only; it sends no definition PUT or DDL request; editor state survives return. |
| SAVE-1 | real API + browser | Save with fields/layout and both expected versions | Preflight produces inspectable plan; Save is the only commit action; confirmed response updates visible versions/state, no publish step. |
| SAVE-2 | real API + browser | Remove a valued field, then confirm current impact token | Impact counts/field IDs visible before action; exact current token accompanies one PUT; committed reread matches saved definition. |
| SAVE-3 | real API + browser | Enabled/in-flight dependency or required backfill/option mapping issue | Blocking reasons/dependencies visible, confirm cannot override; draft remains editable. |
| SAVE-4 | real API + browser | Another editor changes schema/view or data/dependency revision after preflight | 409/stale confirmation shown; fresh preflight required; unsaved input stays intact. |
| SAVE-5 | real API + browser | Response lost after COMMIT, operation lookup temporarily 404/unavailable, then committed replay | UI says result unconfirmed until original operation resolves; exact same key/body is retained, no false rollback/success/new key. |
| SAVE-6 | component + real API | Network error or 401/403/409 during create/save | Error or authorization state visible; no success toast; pending state eventually ends only when result is known or explicitly unconfirmed. |
| VIS-1 | three browsers + manual review | 1920×1080 original designer and every in-scope modal/loading/empty/error/unauthorized state | Real browser screenshots and comparison against original Figma; no Taste75:2/Precision styling. |
| MOT-1 | three browsers | Open/close modal from a trigger, quickly reverse, remove trigger, use reduced motion | Origin/focus restoration when source exists, fade fallback when absent; no stale overlay intercepting clicks; animation cleans on unmount. |
| NAV-1 | browser | Leave dirty designer via form tab, app tab, browser Back, refresh | Warning protects draft; cancel retains form/position; confirmed discard exits; Shell receives dirty state through explicit interface. |

For isolated component tests, a controlled HTTP boundary may return contract
fixtures. Final product E2E must use the actual BFF, isolated PostgreSQL and
restricted roles. Record/query/approval are independent follow-on modules and
are not counted as form-designer acceptance here.

## Intended execution and outstanding gate

After V030-014 PLAN readback and revised contract HEAD: create tests in the
owned test path; run a targeted Playwright/component command for genuine RED,
archive exact test source/command/exit/assertion, then implement in the owned
module. Run targeted GREEN, typecheck/build, three-browser real API E2E and
visual/keyboard/reduced-motion review; archive every actual UI/modal screenshot.
The backend contract/API availability, GSAP dependency/version coordination,
Shell mount and test environment will be verified before claiming completion.
